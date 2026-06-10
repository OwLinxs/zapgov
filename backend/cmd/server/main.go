package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strconv"
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
	"zapgov/backend/internal/labels"
	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/messaging/whatsapp"
	"zapgov/backend/internal/metrics"
	"zapgov/backend/internal/queue"
	"zapgov/backend/internal/quickreplies"
	"zapgov/backend/internal/realtime"
	"zapgov/backend/internal/settings"
	"zapgov/backend/internal/templates"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/tenants"
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
	tenants.NewHandler(tenantsRepo, authSvc.EnsureSeedAdmin).Register(api, auth.RequirePlatformAdmin())

	msgHandler := messaging.NewHandler(svc)
	msgHandler.Register(api)
	contactsRepo := contacts.NewRepository(pool)
	contacts.NewHandler(contactsRepo).Register(api)
	// Auto-cadastro: número avulso vira contato ao iniciar conversa.
	msgHandler.EnsureContact = func(ctx context.Context, nome, telefone string) {
		_ = contactsRepo.Ensure(ctx, nome, telefone)
	}
	metrics.NewHandler(metrics.NewRepository(pool)).Register(api)

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
	templates.NewHandler(templatesClient, templatesRepo).Register(api, opsOnly)
	// Trava de template por departamento (atendente).
	msgHandler.TemplateAllowed = func(ctx context.Context, template, setor string) bool {
		ok, _ := templatesRepo.Allowed(ctx, template, setor)
		return ok
	}

	queueClient := queue.NewClient(cfg.RedisAddr)
	defer queueClient.Close()
	campaignSvc := campaigns.NewService(campaigns.NewRepository(pool), queueClient, templatesClient)
	campaigns.NewHandler(campaignSvc).Register(api, opsOnly)

	departments.NewHandler(departments.NewRepository(pool)).Register(api, adminOnly)
	labelsRepo := labels.NewRepository(pool)
	labels.NewHandler(labelsRepo).Register(api, opsOnly) // criar/remover: admin/supervisor
	quickreplies.NewHandler(quickreplies.NewRepository(pool)).Register(api, opsOnly)

	chatbot.NewHandler(chatbot.NewService(chatbot.NewRepository(pool), svc)).Register(api, adminOnly)

	// Motor de fluxo (grafo de blocos). É ele que processa o inbound agora.
	flowRepo := flow.NewRepository(pool)
	flowSvc := flow.NewService(flowRepo, svc)
	svc.OnInbound = flowSvc.HandleInbound
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
		now := time.Now().In(loc)

		// Feriado / fim de semana (toggle de ausência).
		if a.Ativo {
			md, ymd := now.Format("01-02"), now.Format("2006-01-02")
			for _, f := range a.Feriados {
				if (f == md || f == ymd) && a.Feriado != "" {
					return a.Feriado, true
				}
			}
			if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday {
				if a.FimSemana != "" {
					return a.FimSemana, true
				}
			}
		}

		// Horário comercial: fora do expediente em dia útil configurado.
		if a.HorarioAtivo && a.HorarioMsg != "" {
			wd := strconv.Itoa(int(now.Weekday()))
			if strings.Contains(","+a.HorarioDias+",", ","+wd+",") {
				cur := now.Format("15:04")
				if cur < a.HorarioInicio || cur >= a.HorarioFim {
					return a.HorarioMsg, true
				}
			}
		}
		return "", false
	}

	// Mensagem de encerramento: ao finalizar uma conversa, manda um aviso ao
	// cidadão (se ativo na config e a janela de 24h ainda estiver aberta).
	svc.OnResolved = func(ctx context.Context, convID int64) {
		enc, err := settingsRepo.GetEncerramento(ctx)
		if err != nil || !enc.Ativo || enc.Msg == "" {
			return
		}
		phone, name, err := svc.ContatoDaConversa(ctx, convID)
		if err != nil {
			return
		}
		texto := strings.ReplaceAll(enc.Msg, "{{nome}}", firstName(name))
		if _, err := svc.SendText(ctx, phone, texto); err != nil {
			log.Printf("encerramento: falha ao enviar para conv %d: %v", convID, err)
		}
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
