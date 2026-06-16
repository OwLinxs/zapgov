package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"zapgov/backend/internal/obs"

	"zapgov/backend/internal/audit"
	"zapgov/backend/internal/auth"
	"zapgov/backend/internal/campaigns"
	"zapgov/backend/internal/chatbot"
	"zapgov/backend/internal/config"
	"zapgov/backend/internal/contacts"
	"zapgov/backend/internal/database"
	"zapgov/backend/internal/departments"
	"zapgov/backend/internal/flow"
	"zapgov/backend/internal/kb"
	"zapgov/backend/internal/labels"
	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/messaging/whatsapp"
	"zapgov/backend/internal/metrics"
	"zapgov/backend/internal/queue"
	"zapgov/backend/internal/quickreplies"
	"zapgov/backend/internal/realtime"
	"zapgov/backend/internal/reminders"
	"zapgov/backend/internal/settings"
	"zapgov/backend/internal/surveys"
	"zapgov/backend/internal/templates"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/tenants"
	"zapgov/backend/internal/tickets"
	"zapgov/backend/internal/webhook"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Observabilidade: log estruturado (slog JSON) + Sentry (se SENTRY_DSN).
	flush := obs.Setup(cfg.LogLevel, cfg.SentryDSN, cfg.Env)
	defer flush()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	// Provider real da Meta. Quando o multi-tenant entrar, isto passa a ser
	// instanciado por request com as credenciais do tenant.
	provider := whatsapp.New(
		cfg.WhatsAppPhoneNumberID,
		cfg.WhatsAppToken,
		cfg.WhatsAppAppSecret,
		cfg.WhatsAppAPIVersion,
	)

	repo := messaging.NewRepository(pool)
	svc := messaging.NewService(repo, provider)

	// Eventos em tempo real (SSE) para o inbox.
	hub := realtime.NewHub()
	svc.OnEvent = func(tenantID, convID int64, kind, name, preview string) {
		hub.Publish(realtime.Event{TenantID: tenantID, Type: kind, ConversationID: convID, Name: name, Preview: preview})
	}

	app := fiber.New(fiber.Config{AppName: "ZapGov API"})

	// Aceita o prefixo /api e o remove internamente. Permite servir API e front
	// no MESMO domínio atrás de um proxy simples (o proxy só encaminha /api ->
	// backend, sem precisar reescrever o caminho). /webhook segue na raiz.
	app.Use(func(c *fiber.Ctx) error {
		p := c.Path()
		if p == "/api" || p == "/api/" {
			c.Path("/")
			return c.RestartRouting()
		}
		if strings.HasPrefix(p, "/api/") {
			c.Path(p[len("/api"):])
			return c.RestartRouting()
		}
		return c.Next()
	})

	app.Use(obs.Recover())
	app.Use(obs.RequestLogger())
	// CORS para o front Next (dev em :3000). Restringir em produção.
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins(),
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	// Auth: serviço + seed do admin padrão na primeira subida.
	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo, cfg.JWTSecret)
	authSvc.EnsureSeedAdmin(ctx, 1, "", "") // admin do tenant padrão
	authH := auth.NewHandler(authSvc, authRepo)

	// Tenants (escopo compartilhado): resolve subdomínio/número -> tenant.
	tenantsRepo := tenants.NewRepository(pool)
	authH.SetTenantResolver(func(c *fiber.Ctx, sub string) int64 {
		return tenantsRepo.IDBySubdomain(c.Context(), sub)
	})

	if cfg.JWTSecret == "dev-secret-troque-em-producao" {
		log.Printf("⚠️  ATENÇÃO: JWT_SECRET está com o valor padrão de DEV. Defina JWT_SECRET em produção!")
	}

	// Rate limit no login: trava tentativas de força bruta (10/min por IP).
	loginLimiter := limiter.New(limiter.Config{
		Max:          10,
		Expiration:   time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "muitas tentativas — aguarde um minuto"})
		},
	})
	app.Use("/auth/login", loginLimiter)

	// Middlewares de papel reutilizados.
	adminOnly := auth.RequireRole("admin")
	opsOnly := auth.RequireRole("admin", "supervisor")

	// ---------- Rotas PÚBLICAS (sem JWT) ----------
	app.Get("/health", func(c *fiber.Ctx) error {
		if err := pool.Ping(c.Context()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "degraded"})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})
	authH.RegisterPublic(app) // POST /auth/login
	// Painéis disponíveis no login — PÚBLICA, registrada ANTES do grupo com auth
	// (senão o middleware global a protegeria e o seletor não carregaria).
	tenantsHandler := tenants.NewHandler(tenantsRepo, authSvc.EnsureSeedAdmin)
	tenantsHandler.RegisterPublic(app) // GET /tenants/options
	// Resolver de provider por tenant (credenciais Meta próprias, fallback .env).
	provResolver := tenantsRepo.ProviderResolver(provider)
	svc.ProviderResolver = provResolver
	whHandler := webhook.NewHandler(svc, provider, cfg.WebhookVerifyToken)
	whHandler.SetTenantResolver(func(phoneNumberID string) int64 {
		return tenantsRepo.IDByPhoneNumberID(context.Background(), phoneNumberID)
	})
	whHandler.SetProviderResolver(provResolver)
	whHandler.Register(app) // Meta chama direto
	hub.Register(app, authSvc.TenantFromToken) // GET /events (SSE por tenant, auth por ?token=)

	// ---------- Rotas PROTEGIDAS (exigem JWT) ----------
	api := app.Group("", authSvc.Required())

	// Trilha de auditoria: registra toda mutação (não-GET) bem-sucedida.
	auditRepo := audit.NewRepository(pool)
	api.Use(audit.Middleware(auditRepo))
	audit.NewHandler(auditRepo).Register(api, opsOnly) // GET /audit: admin/supervisor

	authH.RegisterProtected(api) // /auth/me, /users (admin)

	// Super Admin (cross-tenant): só o admin da plataforma (tenant 1).
	tenantsHandler.Register(api, auth.RequirePlatformAdmin())

	msgHandler := messaging.NewHandler(svc)
	msgHandler.Register(api)
	contactsRepo := contacts.NewRepository(pool)
	contacts.NewHandler(contactsRepo).Register(api)
	// Auto-cadastro: número avulso vira contato ao iniciar conversa.
	msgHandler.EnsureContact = func(ctx context.Context, nome, telefone string) {
		_ = contactsRepo.Ensure(ctx, nome, telefone)
	}
	// Meta reportou 131026 (número sem WhatsApp) → marca o contato.
	svc.OnUndeliverable = func(ctx context.Context, phone string) {
		_ = contactsRepo.MarkSemWhatsApp(ctx, phone)
	}

	// Lembretes automáticos: data futura nas variáveis do template agenda a véspera.
	remindersRepo := reminders.NewRepository(pool)
	reminders.NewHandler(remindersRepo).Register(api)
	metricsH := metrics.NewHandler(metrics.NewRepository(pool))
	metricsH.Register(api)
	metricsH.RegisterReports(api, opsOnly)

	templatesClient := templates.NewClient(cfg.WhatsAppWABAID, cfg.WhatsAppToken, cfg.WhatsAppAPIVersion)
	// Templates por tenant: usa a WABA/token do órgão (fallback .env no tenant 1).
	templatesClient.Resolve = func(ctx context.Context) (string, string, string) {
		cr, err := tenantsRepo.Creds(ctx, tenantctx.From(ctx))
		if err != nil {
			return "", "", ""
		}
		return cr.WABAID, cr.Token, cr.APIVersion
	}
	templatesRepo := templates.NewRepository(pool)
	templates.NewHandler(templatesClient, templatesRepo).Register(api, opsOnly, adminOnly)
	// Trava de template por departamento (atendente).
	msgHandler.TemplateAllowed = func(ctx context.Context, template, setor string) bool {
		ok, _ := templatesRepo.Allowed(ctx, template, setor)
		return ok
	}

	queueClient := queue.NewClient(cfg.RedisAddr)
	defer queueClient.Close()
	campRepo := campaigns.NewRepository(pool)
	campaignSvc := campaigns.NewService(campRepo, queueClient, templatesClient)
	campaignSvc.SemWhatsApp = contactsRepo.SemWhatsApp // pula números sem WhatsApp
	// Contadores do disparo: o webhook de status (delivered/read/failed) de
	// mensagens de campanha atualiza entregues/lidas/falhas da campanha.
	svc.OnCampaignStatus = func(ctx context.Context, campID int64, oldSt, newSt string) {
		if err := campRepo.ApplyStatus(ctx, campID, oldSt, newSt); err != nil {
			slog.Error("campanha: atualizar contadores", "campanha", campID, "err", err)
		}
	}
	// Lembrete de véspera (personalizado): delega pro mesmo motor do envio
	// individual. Closure pois msgHandler.OnDateInTemplate é definido mais abaixo.
	campaignSvc.OnDateInTemplate = func(ctx context.Context, phone, nome string, data time.Time) {
		if msgHandler.OnDateInTemplate != nil {
			msgHandler.OnDateInTemplate(ctx, phone, nome, data)
		}
	}

	// Agendador de disparos: a cada minuto despacha campanhas agendadas vencidas.
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			if n, err := campaignSvc.DispatchDue(context.Background(), 10); err != nil {
				slog.Error("disparo agendado: erro", "err", err)
			} else if n > 0 {
				slog.Info("disparos agendados despachados", "n", n)
			}
		}
	}()
	campaigns.NewHandler(campaignSvc).Register(api, opsOnly)

	departments.NewHandler(departments.NewRepository(pool)).Register(api, adminOnly)
	labelsRepo := labels.NewRepository(pool)
	labels.NewHandler(labelsRepo).Register(api, opsOnly) // criar/remover: admin/supervisor
	quickreplies.NewHandler(quickreplies.NewRepository(pool)).Register(api, opsOnly)

	chatbot.NewHandler(chatbot.NewService(chatbot.NewRepository(pool), svc)).Register(api, adminOnly)

	// Módulo de chamados do T.I. (helpdesk). Reaproveita o envio de template do
	// messaging.Service p/ notificar o solicitante por WhatsApp.
	ticketsSvc := tickets.NewService(tickets.NewRepository(pool))
	// notificarChamado avisa o solicitante: tenta texto livre (grátis, dentro da
	// janela de 24h — o solicitante normalmente acabou de interagir); se a janela
	// fechou, cai para o template aprovado. Sem janela aberta E sem template
	// aprovado, registra aviso (não quebra o fluxo do chamado).
	notificarChamado := func(ctx context.Context, fone, texto, template string, params []string) {
		if fone == "" {
			return
		}
		if _, err := svc.SendText(ctx, fone, texto); err == nil {
			return
		} else if errors.Is(err, messaging.ErrOptedOut) {
			return // descadastrado (LGPD): não insiste
		}
		// Janela fechada (ou texto recusado): usa o template aprovado.
		if _, err := svc.SendTemplate(ctx, fone, template, "pt_BR", params, texto); err != nil {
			slog.Warn("chamado: falha ao notificar (texto e template)", "template", template, "err", err)
		}
	}
	ticketsSvc.OnAssigned = func(ctx context.Context, t *tickets.Ticket) {
		texto := fmt.Sprintf("Seu chamado #%d está sendo atendido por %s.", t.Numero, t.Tecnico)
		notificarChamado(ctx, t.SolicitanteFone, texto, "chamado_atribuido",
			[]string{fmt.Sprintf("%d", t.Numero), t.Tecnico})
	}
	ticketsSvc.OnResolved = func(ctx context.Context, t *tickets.Ticket) {
		texto := fmt.Sprintf("Chamado #%d resolvido. Obrigado!", t.Numero)
		notificarChamado(ctx, t.SolicitanteFone, texto, "chamado_resolvido",
			[]string{fmt.Sprintf("%d", t.Numero)})
	}
	// Despacho: envia a ordem de serviço no WhatsApp do técnico de campo (texto
	// livre na janela; senão template 'despacho_chamado').
	ticketsSvc.OnDispatch = func(ctx context.Context, fone, texto string, t *tickets.Ticket) {
		notificarChamado(ctx, fone, texto, "despacho_chamado",
			[]string{fmt.Sprintf("%d", t.Numero), firstNonEmpty(t.Local, t.Secretaria, "—"), t.Titulo})
	}
	tickets.NewHandler(ticketsSvc).Register(api, opsOnly)
	// Base de conhecimento (T.I.): leitura aberta, gestão p/ admin/supervisor.
	kb.NewHandler(kb.NewRepository(pool)).Register(api, opsOnly)
	// Atendimento pelo chat: quando a conversa é assumida/resolvida/transferida,
	// espelha o status no chamado ligado (kanban reflete o que acontece no inbox).
	svc.OnConversationStatus = func(ctx context.Context, convID int64, status, atendente string) {
		ts := map[string]string{"em_atendimento": "em_andamento", "resolvido": "resolvido", "aguardando": "aguardando"}[status]
		if ts == "" {
			return
		}
		if err := ticketsSvc.SyncFromConversation(ctx, convID, ts, atendente); err != nil {
			slog.Warn("chamado: sincronizar do chat", "conv", convID, "err", err)
		}
	}

	// Motor de fluxo (grafo de blocos). É ele que processa o inbound agora.
	flowRepo := flow.NewRepository(pool)
	flowSvc := flow.NewService(flowRepo, svc)
	svc.OnInbound = flowSvc.HandleInbound
	// Bloco "abrir_chamado" do fluxo cria um chamado com as variáveis coletadas.
	// Roda no ctx do tenant (T.I.) → grava no tenant certo automaticamente.
	flowSvc.OnOpenTicket = func(ctx context.Context, from, nome string, vars map[string]string) (int, error) {
		titulo := firstNonEmpty(vars["titulo"], vars["assunto"], vars["descricao"], "Chamado via WhatsApp")
		// Liga o chamado à conversa de origem (técnico abre o WhatsApp que o gerou).
		var convID *int64
		if id, _ := svc.ConversationIDByPhone(ctx, from); id > 0 {
			convID = &id
		}
		t, err := ticketsSvc.Create(ctx, tickets.CreateInput{
			Titulo:          titulo,
			Descricao:       vars["descricao"],
			Categoria:       vars["categoria"],
			Prioridade:      vars["prioridade"], // default 'media' no service
			Secretaria:      vars["secretaria"],
			Local:           vars["local"],
			SolicitanteNome: nome,
			SolicitanteFone: from, // já canônico no HandleInbound
			ConversationID:  convID,
		})
		if err != nil {
			return 0, err
		}
		return t.Numero, nil
	}
	flow.NewHandler(flowRepo).Register(api, adminOnly)

	settingsRepo := settings.NewRepository(pool)
	// Assinatura do atendente no fim das mensagens (config por tenant).
	msgHandler.SignatureEnabled = func(ctx context.Context) bool {
		on, _ := settingsRepo.GetAssinatura(ctx)
		return on
	}
	// Roteamento por setor: atendente só vê conversas do próprio setor.
	msgHandler.RouteBySetor = func(ctx context.Context) bool {
		rt, _ := settingsRepo.GetRoteamento(ctx)
		return rt.PorSetor
	}
	// Chatbot on/off: desligado, inbound vai direto pra fila.
	svc.ChatbotEnabled = func(ctx context.Context) bool {
		on, err := settingsRepo.GetChatbot(ctx)
		if err != nil {
			return true // em erro, mantém o bot (comportamento padrão)
		}
		return on
	}

	// Agenda o lembrete: véspera da data detectada, na hora configurada (fuso da org).
	msgHandler.OnDateInTemplate = func(ctx context.Context, phone, nome string, data time.Time) {
		l, err := settingsRepo.GetLembrete(ctx)
		if err != nil || !l.Ativo || l.Template == "" {
			return
		}
		loc, e := time.LoadLocation(l.Fuso)
		if e != nil || loc == nil {
			loc = time.UTC
		}
		hh, mm := 9, 0
		if t, err := time.Parse("15:04", l.Hora); err == nil {
			hh, mm = t.Hour(), t.Minute()
		}
		vespera := time.Date(data.Year(), data.Month(), data.Day(), hh, mm, 0, 0, loc).AddDate(0, 0, -1)
		now := time.Now().In(loc)
		if !data.After(now) {
			return // evento já passou/é hoje: sem lembrete
		}
		if vespera.Before(now) {
			vespera = now.Add(2 * time.Minute) // véspera já passou (evento amanhã): manda já
		}
		_ = remindersRepo.Create(ctx, phone, nome, data, vespera)
	}

	// Agendador: a cada minuto envia os lembretes vencidos (template da config).
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			due, err := remindersRepo.Due(context.Background(), 50)
			if err != nil {
				continue
			}
			for _, rm := range due {
				ctx := tenantctx.With(context.Background(), rm.TenantID)
				l, err := settingsRepo.GetLembrete(ctx)
				if err != nil || !l.Ativo || l.Template == "" {
					_ = remindersRepo.MarkStatus(ctx, rm.ID, "cancelado")
					continue
				}
				params := []string{firstName(rm.Nome), rm.DataEvento.Format("02/01/2006")}
				preview := "[lembrete automático: " + l.Template + " — " + rm.DataEvento.Format("02/01/2006") + "]"
				if _, err := svc.SendTemplate(ctx, rm.Phone, l.Template, l.Idioma, params, preview); err != nil {
					slog.Error("lembrete: envio falhou", "id", rm.ID, "err", err)
					_ = remindersRepo.MarkStatus(ctx, rm.ID, "falhou")
					continue
				}
				_ = remindersRepo.MarkStatus(ctx, rm.ID, "enviado")
			}
		}
	}()
	// Auto-etiqueta: ao assumir, aplica uma etiqueta com o nome do atendente.
	msgHandler.TagAttendant = func(ctx context.Context, convID int64, nome string) {
		if nome == "" {
			return
		}
		rt, _ := settingsRepo.GetRoteamento(ctx)
		if !rt.PorEtiqueta {
			return
		}
		if id, err := labelsRepo.EnsureByName(ctx, nome, ""); err == nil {
			_ = repo.AddLabel(ctx, convID, id)
		}
	}
	settings.NewHandler(settingsRepo, settings.MetaInfo{
		PhoneNumberID: cfg.WhatsAppPhoneNumberID,
		WABAID:        cfg.WhatsAppWABAID,
		APIVersion:    cfg.WhatsAppAPIVersion,
		Connected:     cfg.WhatsAppToken != "",
	}).Register(api, adminOnly)

	// Auto-resposta de ausência: avalia se agora é feriado/fim de semana (no fuso
	// da org) e devolve a mensagem configurada.
	svc.AbsenceMessage = func(ctx context.Context) (string, bool) {
		a, err := settingsRepo.GetAusencia(ctx)
		if err != nil || (!a.Ativo && !a.HorarioAtivo) {
			return "", false
		}
		loc, e := time.LoadLocation(a.Fuso)
		if e != nil || loc == nil {
			loc = time.UTC
		}
		return settings.MensagemAusencia(a, time.Now().In(loc))
	}

	// Mensagem de encerramento: ao finalizar uma conversa, manda um aviso ao
	// cidadão (se ativo na config e a janela de 24h ainda estiver aberta).
	surveysRepo := surveys.NewRepository(pool)

	svc.OnResolved = func(ctx context.Context, convID int64) {
		phone, name, errC := svc.ContatoDaConversa(ctx, convID)

		// Reinicia o chatbot: limpa o estado do fluxo do contato, para que a
		// PRÓXIMA mensagem dele recomece o bot do início (não continue de onde
		// parou no atendimento anterior).
		if errC == nil {
			_ = flowRepo.ClearState(ctx, messaging.CanonPhone(phone))
		}

		// Mensagem de encerramento (se ativa).
		if enc, err := settingsRepo.GetEncerramento(ctx); err == nil && enc.Ativo && enc.Msg != "" && errC == nil {
			texto := strings.ReplaceAll(enc.Msg, "{{nome}}", firstName(name))
			if _, err := svc.SendText(ctx, phone, texto); err != nil {
				log.Printf("encerramento: falha ao enviar para conv %d: %v", convID, err)
			}
		}

		// Pesquisa de satisfação (se ativa): lista interativa 1-5.
		if p, err := settingsRepo.GetPesquisa(ctx); err == nil && p.Ativo && p.Msg != "" && errC == nil {
			setor, atendente, _ := repo.SetorAtendente(ctx, convID)
			opts := []messaging.MenuOption{
				{Key: "nota_5", Label: "⭐⭐⭐⭐⭐ Excelente"},
				{Key: "nota_4", Label: "⭐⭐⭐⭐ Bom"},
				{Key: "nota_3", Label: "⭐⭐⭐ Regular"},
				{Key: "nota_2", Label: "⭐⭐ Ruim"},
				{Key: "nota_1", Label: "⭐ Muito ruim"},
			}
			if _, err := svc.SendMenu(ctx, phone, p.Msg, opts); err != nil {
				log.Printf("pesquisa: falha ao enviar para conv %d: %v", convID, err)
				return
			}
			_ = surveysRepo.Create(ctx, convID, atendente, setor)
		}
	}

	// Captura da resposta da pesquisa: nota válida + pendência → consome.
	svc.OnSurveyAnswer = func(ctx context.Context, convID int64, buttonID, body, from string) bool {
		nota, ok := surveys.ParseNota(buttonID, body)
		if !ok {
			return false
		}
		answered, err := surveysRepo.Answer(ctx, convID, nota)
		if err != nil || !answered {
			return false // sem pesquisa pendente: segue fluxo normal
		}
		if p, err := settingsRepo.GetPesquisa(ctx); err == nil && p.Obrigado != "" {
			_, _ = svc.SendText(ctx, from, p.Obrigado)
		}
		return true
	}

	// Job periódico: finaliza conversas cuja janela de 24h expirou.
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			if n, err := svc.AutoResolveExpired(context.Background()); err != nil {
				slog.Error("auto-resolve: erro", "err", err)
			} else if n > 0 {
				slog.Info("conversas finalizadas por janela expirada", "n", n)
			}
		}
	}()

	log.Printf("ZapGov API ouvindo na porta %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// firstName devolve só o primeiro nome (para personalizar o {{nome}}).
func firstName(nome string) string {
	nome = strings.TrimSpace(nome)
	if i := strings.IndexByte(nome, ' '); i > 0 {
		return nome[:i]
	}
	return nome
}

func corsOrigins() string {
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		return v
	}
	return "http://localhost:3000,http://localhost:3001"
}

// firstNonEmpty devolve o primeiro argumento não-vazio (após trim).
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
