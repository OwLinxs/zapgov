package messaging

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"zapgov/backend/internal/tenantctx"
)

// ErrWindowClosed: texto livre só dentro da janela de 24h (regra da Meta).
var ErrWindowClosed = errors.New("janela de 24h fechada — inicie a conversa com um template aprovado")

// ErrOptedOut: contato pediu para não receber mais mensagens (LGPD).
var ErrOptedOut = errors.New("contato descadastrado (opt-out) — não pode receber mensagens")

const optOutMsg = "Você foi removido da nossa lista e não receberá mais mensagens. Para voltar a receber, responda VOLTAR."

// palavras que disparam o descadastro automático.
func isStopWord(up string) bool {
	switch up {
	case "SAIR", "PARAR", "CANCELAR", "STOP", "DESCADASTRAR", "REMOVER":
		return true
	}
	return false
}

// Service contém a regra de negócio de mensageria, separada do handler (HTTP)
// e do repository (banco).
type Service struct {
	repo     *Repository
	provider MessageProvider

	// OnInbound, se definido, é chamado para cada mensagem recebida NOVA
	// (não reentregue). Usado pelo chatbot para autoatendimento, sem acoplar
	// o pacote messaging ao chatbot. buttonID é a chave do botão/lista tocado
	// (vazio em texto livre) — identificador confiável, não trunca como o título.
	OnInbound func(ctx context.Context, from, name, body, buttonID string)

	// OnEvent, se definido, publica eventos em tempo real (SSE) para o front.
	// tenantID isola a entrega; name/preview só em mensagem recebida.
	OnEvent func(tenantID, convID int64, kind, name, preview string)

	// OnResolved, se definido, é chamado ao finalizar uma conversa (Resolve
	// individual). Usado para disparar a mensagem de encerramento.
	OnResolved func(ctx context.Context, convID int64)

	// ProviderResolver, se definido, devolve o provider (credenciais Meta) do
	// tenant do context. nil => usa o provider padrão (env, tenant 1).
	ProviderResolver func(ctx context.Context) MessageProvider

	// AbsenceMessage, se definido, devolve (msg, true) quando o momento atual é
	// de ausência (fim de semana/feriado) com auto-resposta ativa. Usado para
	// avisar o cidadão fora do horário de atendimento.
	AbsenceMessage func(ctx context.Context) (string, bool)

	// ChatbotEnabled, se definido e false, desliga o bot: a mensagem do cidadão
	// vai direto pra fila (aguardando), sem menu/auto-resposta.
	ChatbotEnabled func(ctx context.Context) bool

	// OnUndeliverable, se definido, é chamado quando a Meta reporta que o número
	// NÃO tem WhatsApp (erro 131026). Usado p/ marcar o contato.
	OnUndeliverable func(ctx context.Context, phone string)

	// OnSurveyAnswer, se definido, tenta consumir a mensagem como resposta de
	// pesquisa de satisfação. true = consumida (a conversa volta a resolvida e
	// nem bot nem fila são acionados).
	OnSurveyAnswer func(ctx context.Context, convID int64, buttonID, body, from string) bool

	// OnCampaignStatus, se definido, é chamado quando o status de uma mensagem
	// de CAMPANHA muda (webhook) — atualiza os contadores do disparo
	// (entregues/lidas/falhas) sem acoplar messaging ao pacote campaigns.
	OnCampaignStatus func(ctx context.Context, campaignID int64, oldStatus, newStatus string)

	// OnConversationStatus, se definido, é chamado quando uma conversa muda de
	// situação (assumida/resolvida/transferida). Usado para espelhar o status
	// no chamado (ticket) ligado àquela conversa, sem acoplar messaging a tickets.
	OnConversationStatus func(ctx context.Context, convID int64, status, atendente string)

	// OnButtonAction, se definido, trata o toque num botão de ação (ButtonID).
	// true = consumido (não roda o bot). Ex.: técnico finaliza chamado por botão.
	OnButtonAction func(ctx context.Context, from, buttonID string) bool

	// Gateway, se definido e devolve true, consumiu a mensagem: é o gateway
	// WhatsApp<->SIGE (relay/menu do bot cru). Roda antes dos gates de fila/chatbot,
	// só para o tenant do gateway (a própria função decide e devolve false fora dele).
	Gateway func(ctx context.Context, in IncomingMessage) bool

	// IsBotExempt, se definido e true para o remetente, não dispara o chatbot/fila
	// para as mensagens dele (ex.: técnico de campo só abre a janela de 24h para
	// receber ordens). Ações por botão continuam funcionando.
	IsBotExempt func(ctx context.Context, from string) bool

	// TechBot, se definido, processa as mensagens de um remetente isento
	// (IsBotExempt) — o "modo técnico" do helpdesk: o técnico de campo lista e
	// age nos chamados por menu/botões. Roda no lugar do bot do cidadão.
	TechBot func(ctx context.Context, from, name, body, buttonID string)
}

// providerFor escolhe o provider do tenant atual (multi-tenant) ou o padrão.
func (s *Service) providerFor(ctx context.Context) MessageProvider {
	if s.ProviderResolver != nil {
		if p := s.ProviderResolver(ctx); p != nil {
			return p
		}
	}
	return s.provider
}

func (s *Service) emit(ctx context.Context, convID int64, kind string) {
	if s.OnEvent != nil {
		s.OnEvent(tenantctx.From(ctx), convID, kind, "", "")
	}
}

// emitInbound notifica uma mensagem recebida do cidadão (com nome + prévia).
func (s *Service) emitInbound(ctx context.Context, convID int64, name, preview string) {
	if s.OnEvent != nil {
		s.OnEvent(tenantctx.From(ctx), convID, "message", name, preview)
	}
}

func NewService(repo *Repository, provider MessageProvider) *Service {
	return &Service{repo: repo, provider: provider}
}

// SendText envia uma mensagem de texto única (síncrono): chama o provider e,
// se aceito pela Meta, persiste com o wa_message_id retornado.
func (s *Service) SendText(ctx context.Context, to, body string) (*Message, error) {
	return s.SendTextReply(ctx, to, body, "")
}

// SendTextReply envia texto citando uma mensagem anterior (replyTo =
// wa_message_id alvo; vazio = sem citação).
func (s *Service) SendTextReply(ctx context.Context, to, body, replyTo string) (*Message, error) {
	if to == "" || body == "" {
		return nil, fmt.Errorf("destinatário e corpo são obrigatórios")
	}

	// LGPD: bloqueia envio a quem pediu opt-out.
	if out, err := s.repo.ConversationOptedOut(ctx, to); err != nil {
		return nil, fmt.Errorf("verificar opt-out: %w", err)
	} else if out {
		return nil, ErrOptedOut
	}

	// Pré-checagem da janela de 24h: evita a recusa da Meta e orienta o uso de template.
	open, err := s.repo.WindowOpen(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("verificar janela: %w", err)
	}
	if !open {
		return nil, ErrWindowClosed
	}

	res, err := s.providerFor(ctx).Send(ctx, OutgoingMessage{To: to, Body: body, ReplyTo: replyTo})
	if err != nil {
		return nil, fmt.Errorf("envio falhou: %w", err)
	}

	convID, err := s.repo.EnsureConversation(ctx, to, "")
	if err != nil {
		return nil, fmt.Errorf("garantir conversa: %w", err)
	}

	msg, err := s.repo.InsertOutbound(ctx, convID, res.WAMessageID, body, replyTo)
	if err != nil {
		return nil, fmt.Errorf("persistir mensagem: %w", err)
	}
	s.emit(ctx, convID, "message")
	return msg, nil
}

// SendTemplate inicia uma conversa com um template aprovado (válido fora da
// janela de 24h e em disparos). Persiste a mensagem com um corpo legível para o histórico.
func (s *Service) SendTemplate(ctx context.Context, to, name, lang string, params []string, preview string) (*Message, error) {
	if to == "" || name == "" {
		return nil, fmt.Errorf("destinatário e nome do template são obrigatórios")
	}

	// LGPD: não dispara para quem está na lista de opt-out.
	if out, err := s.repo.IsOptedOut(ctx, to); err != nil {
		return nil, fmt.Errorf("verificar opt-out: %w", err)
	} else if out {
		return nil, ErrOptedOut
	}

	res, err := s.providerFor(ctx).SendTemplate(ctx, TemplateMessage{
		To:           to,
		Name:         name,
		LanguageCode: lang,
		BodyParams:   params,
	})
	if err != nil {
		return nil, fmt.Errorf("envio de template falhou: %w", err)
	}

	convID, err := s.repo.EnsureConversation(ctx, to, "")
	if err != nil {
		return &Message{WAMessageID: res.WAMessageID}, fmt.Errorf("garantir conversa após envio: %w", err)
	}

	body := preview
	if body == "" {
		body = "[modelo: " + name + "]"
	}
	msg, err := s.repo.InsertOutbound(ctx, convID, res.WAMessageID, body, "")
	if err != nil {
		return &Message{WAMessageID: res.WAMessageID}, fmt.Errorf("persistir mensagem após envio: %w", err)
	}
	s.emit(ctx, convID, "message")
	return msg, nil
}

// SendMenu envia um menu interativo (botões/lista). Usado pelo bot.
func (s *Service) SendMenu(ctx context.Context, to, text string, options []MenuOption) (*Message, error) {
	if out, err := s.repo.ConversationOptedOut(ctx, to); err != nil {
		return nil, fmt.Errorf("verificar opt-out: %w", err)
	} else if out {
		return nil, ErrOptedOut
	}
	if open, _ := s.repo.WindowOpen(ctx, to); !open {
		return nil, ErrWindowClosed
	}
	res, err := s.providerFor(ctx).SendMenu(ctx, to, text, options)
	if err != nil {
		return nil, fmt.Errorf("envio do menu falhou: %w", err)
	}
	convID, err := s.repo.EnsureConversation(ctx, to, "")
	if err != nil {
		return nil, err
	}
	body := text
	for _, o := range options {
		body += "\n" + o.Key + " - " + o.Label
	}
	msg, err := s.repo.InsertOutbound(ctx, convID, res.WAMessageID, body, "")
	if err == nil {
		s.emit(ctx, convID, "message")
	}
	return msg, err
}

// SendMedia envia uma mídia por link (imagem/documento/áudio/vídeo).
func (s *Service) SendMedia(ctx context.Context, to, kind, link, caption, name string) (*Message, error) {
	if to == "" || link == "" {
		return nil, fmt.Errorf("destinatário e link da mídia são obrigatórios")
	}
	if out, err := s.repo.ConversationOptedOut(ctx, to); err != nil {
		return nil, fmt.Errorf("verificar opt-out: %w", err)
	} else if out {
		return nil, ErrOptedOut
	}
	if open, _ := s.repo.WindowOpen(ctx, to); !open {
		return nil, ErrWindowClosed
	}
	res, err := s.providerFor(ctx).SendMedia(ctx, OutgoingMedia{To: to, Kind: kind, Link: link, Caption: caption, Name: name})
	if err != nil {
		return nil, fmt.Errorf("envio de mídia falhou: %w", err)
	}
	convID, err := s.repo.EnsureConversation(ctx, to, "")
	if err != nil {
		return nil, err
	}
	return s.repo.InsertOutboundMedia(ctx, convID, res.WAMessageID, caption, kind, name, "")
}

// SendMediaUpload sobe um arquivo pra Meta e envia por media_id (upload direto,
// sem precisar de URL pública). Persiste no histórico.
func (s *Service) SendMediaUpload(ctx context.Context, to, kind string, data []byte, filename, mime, caption string) (*Message, error) {
	if to == "" || len(data) == 0 {
		return nil, fmt.Errorf("destinatário e arquivo são obrigatórios")
	}
	if out, err := s.repo.ConversationOptedOut(ctx, to); err != nil {
		return nil, fmt.Errorf("verificar opt-out: %w", err)
	} else if out {
		return nil, ErrOptedOut
	}
	if open, _ := s.repo.WindowOpen(ctx, to); !open {
		return nil, ErrWindowClosed
	}
	prov := s.providerFor(ctx)
	id, err := prov.UploadMedia(ctx, data, filename, mime)
	if err != nil {
		return nil, fmt.Errorf("upload da mídia falhou: %w", err)
	}
	res, err := prov.SendMedia(ctx, OutgoingMedia{To: to, Kind: kind, ID: id, Caption: caption, Name: filename})
	if err != nil {
		return nil, fmt.Errorf("envio de mídia falhou: %w", err)
	}
	convID, err := s.repo.EnsureConversation(ctx, to, "")
	if err != nil {
		return nil, err
	}
	m, err := s.repo.InsertOutboundMedia(ctx, convID, res.WAMessageID, caption, kind, filename, id)
	if err == nil {
		s.emit(ctx, convID, "message")
	}
	return m, err
}

// DownloadMedia baixa os bytes de uma mídia (proxy para exibir no front).
func (s *Service) DownloadMedia(ctx context.Context, mediaID string) ([]byte, string, error) {
	return s.providerFor(ctx).DownloadMedia(ctx, mediaID)
}

// ProcessWebhook persiste mensagens recebidas e atualizações de status.
// Erros são logados mas não propagados de forma a impedir o HTTP 200 — a Meta
// reentrega em caso de não-200, e a idempotência cobre o reprocessamento.
func (s *Service) ProcessWebhook(ctx context.Context, body []byte) error {
	incoming, statuses, reactions, err := s.provider.ParseWebhook(body)
	if err != nil {
		return err
	}

	// Reações do cidadão: atualiza a mensagem alvo e avisa o front.
	for _, rx := range reactions {
		if convID, err := s.repo.SetReaction(ctx, rx.TargetWAMessageID, rx.Emoji); err == nil && convID > 0 {
			s.emit(ctx, convID, "message")
		}
	}

	for _, in := range incoming {
		convID, err := s.repo.EnsureConversation(ctx, in.From, in.Name)
		if err != nil {
			return fmt.Errorf("garantir conversa: %w", err)
		}
		inserted, err := s.repo.InsertInbound(ctx, convID, in.WAMessageID, in.Body, in.Timestamp,
			MediaInfo{Type: in.MediaType, ID: in.MediaID, Mime: in.MediaMime, Name: in.MediaName}, in.ReplyTo)
		if err != nil {
			return fmt.Errorf("inserir inbound: %w", err)
		}
		if !inserted {
			continue // reentrega da Meta
		}

		// LGPD: descadastro/recadastro por palavra-chave.
		up := strings.ToUpper(strings.TrimSpace(in.Body))
		if isStopWord(up) {
			_ = s.repo.OptOut(ctx, in.From, "solicitado pelo cidadão")
			_, _ = s.providerFor(ctx).Send(ctx, OutgoingMessage{To: in.From, Body: optOutMsg})
			continue // não roda o bot
		}
		if up == "VOLTAR" || up == "INICIAR" {
			_ = s.repo.OptIn(ctx, in.From)
		}

		// Empurra para o front em tempo real (SSE) com dados para notificação.
		preview := in.Body
		if preview == "" && in.MediaType != "" {
			preview = "[" + in.MediaType + "]"
		}
		nome := in.Name
		if nome == "" {
			nome = in.From
		}
		// Ação por botão (ex.: técnico de campo confirma finalização de um chamado).
		// O ButtonID identifica a ação/alvo; consome sem rodar o bot.
		if s.OnButtonAction != nil && in.ButtonID != "" && s.OnButtonAction(ctx, in.From, in.ButtonID) {
			continue
		}

		// Número isento (técnico de campo): não roda o bot do cidadão nem entra na
		// fila. Se o modo técnico está ligado, a mensagem navega os chamados
		// (menu/botões); senão só abre a janela de 24h para receber ordens.
		if s.IsBotExempt != nil && s.IsBotExempt(ctx, in.From) {
			if s.TechBot != nil {
				s.TechBot(ctx, in.From, nome, in.Body, in.ButtonID)
			}
			continue
		}

		// Resposta de pesquisa de satisfação: grava a nota, agradece e devolve a
		// conversa a resolvida — não reabre fila nem dispara bot.
		if s.OnSurveyAnswer != nil && s.OnSurveyAnswer(ctx, convID, in.ButtonID, in.Body, in.From) {
			_ = s.repo.Resolve(ctx, convID)
			s.emit(ctx, convID, "message")
			continue
		}

		s.emitInbound(ctx, convID, nome, preview)

		// Gateway WhatsApp<->SIGE: no tenant do gateway, o relay/menu consome a
		// mensagem (não roda fila/chatbot deste ZapGov).
		if s.Gateway != nil && s.Gateway(ctx, in) {
			continue
		}

		// Auto-resposta de ausência (feriado/fim de semana), no máximo 1x por
		// período (throttle no banco). Não interrompe o fluxo normal.
		if s.AbsenceMessage != nil {
			if msg, off := s.AbsenceMessage(ctx); off && msg != "" {
				if due, _ := s.repo.MarkAbsenceIfDue(ctx, convID); due {
					_, _ = s.SendText(ctx, in.From, msg)
				}
			}
		}

		// Já está na fila ou em atendimento humano → o bot NÃO responde
		// (senão reiniciaria o menu a cada mensagem). Atendente assume.
		if st, _ := s.repo.ConversationStatus(ctx, convID); st == "aguardando" || st == "em_atendimento" {
			continue
		}

		// Chatbot desligado → manda direto pra fila, sem menu/auto-resposta.
		if s.ChatbotEnabled != nil && !s.ChatbotEnabled(ctx) {
			if err := s.repo.Enqueue(ctx, convID); err == nil {
				s.emit(ctx, 0, "conversation")
			}
			continue
		}

		// Caso contrário (fase de bot), roda o fluxo.
		if s.OnInbound != nil {
			s.OnInbound(ctx, in.From, in.Name, in.Body, in.ButtonID)
		}
	}

	for _, st := range statuses {
		errText := ""
		if st.Status == "failed" {
			// Falha sem código: a Meta não detalhou (comum quando o destinatário
			// bloqueou o número). Loga o payload bruto p/ diagnóstico.
			if st.ErrorCode == 0 {
				log.Printf("webhook: failed SEM código (wamid=%s, dest=%s) payload=%s",
					st.WAMessageID, st.RecipientID, truncate(string(body), 600))
				errText = "Falha no envio — sem detalhe da Meta (possível bloqueio pelo destinatário)"
			} else {
				errText = friendlyError(st.ErrorCode, st.ErrorTitle)
			}
			// 131026 = destinatário não tem WhatsApp → marca o contato.
			if st.ErrorCode == 131026 && s.OnUndeliverable != nil && st.RecipientID != "" {
				s.OnUndeliverable(ctx, CanonPhone(st.RecipientID))
			}
		}
		campID, oldStatus, err := s.repo.UpdateStatus(ctx, st.WAMessageID, st.Status, errText)
		if err != nil {
			return fmt.Errorf("atualizar status: %w", err)
		} else if campID > 0 && oldStatus != st.Status && s.OnCampaignStatus != nil {
			// Mensagem de campanha mudou de status → atualiza os contadores do disparo.
			s.OnCampaignStatus(ctx, campID, oldStatus, st.Status)
		}
	}
	if len(statuses) > 0 {
		s.emit(ctx, 0, "status")
	}
	return nil
}

// truncate corta uma string para log (payloads grandes).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// friendlyError traduz os códigos de erro mais comuns da Meta.
func friendlyError(code int, title string) string {
	switch code {
	case 131026:
		return "Número sem WhatsApp (não cadastrado)"
	case 131047:
		return "Janela de 24h expirada — use um template"
	case 131048, 131056:
		return "Limite de envio atingido (spam) — tente mais tarde"
	case 132000, 132001:
		return "Problema no template (não existe ou parâmetros errados)"
	case 131021:
		return "Não é possível enviar para o próprio número"
	}
	if title != "" {
		return fmt.Sprintf("Falha no envio (%d: %s)", code, title)
	}
	return "Falha no envio"
}

// ListRecent devolve as mensagens mais recentes.
func (s *Service) ListRecent(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListMessages(ctx, limit)
}

// ListConversations devolve as conversas para a lista do inbox (filtro opcional).
// soAtendimento=true esconde conversas em fase de bot (uso do papel atendente).
func (s *Service) ListConversations(ctx context.Context, limit int, setores []string, status, q string, soAtendimento bool) ([]Conversation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListConversations(ctx, limit, setores, status, q, soAtendimento)
}

// ListSessions devolve as sessões (atendimentos) de uma conversa.
func (s *Service) ListSessions(ctx context.Context, convID int64) ([]Session, error) {
	return s.repo.ListSessions(ctx, convID)
}

// SessionMessages devolve as mensagens de uma sessão específica.
func (s *Service) SessionMessages(ctx context.Context, sessionID int64) ([]Message, error) {
	return s.repo.ListSessionMessages(ctx, sessionID)
}

// OptOutConversation / OptInConversation: descadastro manual pelo atendente.
func (s *Service) OptOutConversation(ctx context.Context, convID int64) error {
	phone, err := s.repo.PhoneByID(ctx, convID)
	if err != nil {
		return err
	}
	return s.repo.OptOut(ctx, phone, "manual pelo atendente")
}

func (s *Service) OptInConversation(ctx context.Context, convID int64) error {
	phone, err := s.repo.PhoneByID(ctx, convID)
	if err != nil {
		return err
	}
	return s.repo.OptIn(ctx, phone)
}

// SetLabels aplica as etiquetas escolhidas a uma conversa.
func (s *Service) SetLabels(ctx context.Context, convID int64, labelIDs []int64) error {
	return s.repo.SetLabels(ctx, convID, labelIDs)
}

// React envia uma reação (emoji) a uma mensagem do cidadão e persiste.
// Emoji vazio remove a reação.
func (s *Service) React(ctx context.Context, convID int64, waMessageID, emoji string) error {
	phone, err := s.repo.PhoneByID(ctx, convID)
	if err != nil {
		return err
	}
	if err := s.providerFor(ctx).SendReaction(ctx, phone, waMessageID, emoji); err != nil {
		return fmt.Errorf("reagir falhou: %w", err)
	}
	if _, err := s.repo.SetReaction(ctx, waMessageID, emoji); err != nil {
		return err
	}
	s.emit(ctx, convID, "message")
	return nil
}

// AddNote grava uma nota interna (não vai pro cidadão) e emite evento p/ o front.
func (s *Service) AddNote(ctx context.Context, convID int64, autor, body string) (*Message, error) {
	m, err := s.repo.AddNote(ctx, convID, autor, body)
	if err == nil {
		s.emit(ctx, convID, "message")
	}
	return m, err
}

// SetAuthor marca quem (atendente) enviou uma mensagem — exibido no painel.
func (s *Service) SetAuthor(ctx context.Context, id int64, autor string) error {
	return s.repo.SetAuthor(ctx, id, autor)
}

// ConversationAttendant devolve o atendente atual da conversa ("" se ninguém).
func (s *Service) ConversationAttendant(ctx context.Context, convID int64) string {
	_, atendente, _ := s.repo.SetorAtendente(ctx, convID)
	return atendente
}

// OwnsConversation confirma que a conversa é do tenant do contexto. Usar antes
// de qualquer escrita por id vindo da URL (barreira anti-IDOR cross-tenant).
func (s *Service) OwnsConversation(ctx context.Context, convID int64) bool {
	ok, err := s.repo.ConversationBelongs(ctx, convID)
	return err == nil && ok
}

// AddEvent registra um evento de sistema na conversa (resolvido/assumiu/
// transferido) — some no painel da equipe, nunca é enviado ao cidadão.
func (s *Service) AddEvent(ctx context.Context, convID int64, texto string) {
	if _, err := s.repo.InsertEvent(ctx, convID, texto); err != nil {
		log.Printf("evento: falha ao gravar na conv %d: %v", convID, err)
		return
	}
	s.emit(ctx, convID, "message")
}

// SetSetorIfEmpty define o setor da conversa se ainda não tiver (conversa
// iniciada pelo atendente herda o setor dele).
func (s *Service) SetSetorIfEmpty(ctx context.Context, convID int64, setor string) error {
	return s.repo.SetSetorIfEmpty(ctx, convID, setor)
}

// SetSetor força o setor da conversa (template iniciado pelo atendente).
func (s *Service) SetSetor(ctx context.Context, convID int64, setor string) error {
	if err := s.repo.SetSetor(ctx, convID, setor); err != nil {
		return err
	}
	s.emit(ctx, 0, "conversation")
	return nil
}

// SetContactNameIfEmpty grava o nome do contato na conversa (avulso).
func (s *Service) SetContactNameIfEmpty(ctx context.Context, convID int64, nome string) error {
	return s.repo.SetContactNameIfEmpty(ctx, convID, nome)
}

// Transfer move a conversa para outro setor (volta à fila do novo setor).
func (s *Service) Transfer(ctx context.Context, convID int64, setor string) error {
	if err := s.repo.Transfer(ctx, convID, setor); err != nil {
		return err
	}
	s.emit(ctx, 0, "conversation")
	s.notifyConvStatus(ctx, convID, "aguardando", "")
	return nil
}

// AutoResolveExpired finaliza conversas com janela de 24h expirada. Emite evento
// se algo mudou (para o inbox atualizar).
func (s *Service) AutoResolveExpired(ctx context.Context) (int, error) {
	n, err := s.repo.AutoResolveExpired(ctx)
	if err == nil && n > 0 {
		s.emit(ctx, 0, "conversation")
	}
	return n, err
}

// ResolveBulk finaliza várias conversas de uma vez.
func (s *Service) ResolveBulk(ctx context.Context, ids []int64) (int, error) {
	n, err := s.repo.ResolveBulk(ctx, ids)
	if err == nil && n > 0 {
		s.emit(ctx, 0, "conversation")
	}
	return n, err
}

// Claim: atendente assume a conversa.
func (s *Service) Claim(ctx context.Context, id int64, atendente string) error {
	if err := s.repo.Claim(ctx, id, atendente); err != nil {
		return err
	}
	s.notifyConvStatus(ctx, id, "em_atendimento", atendente)
	return nil
}

// ClaimIfQueued: assume a conversa apenas se estiver na fila.
func (s *Service) ClaimIfQueued(ctx context.Context, id int64, atendente string) error {
	if err := s.repo.ClaimIfQueued(ctx, id, atendente); err != nil {
		return err
	}
	s.notifyConvStatus(ctx, id, "em_atendimento", atendente)
	return nil
}

// notifyConvStatus espelha a mudança de situação da conversa (p/ o chamado).
func (s *Service) notifyConvStatus(ctx context.Context, convID int64, status, atendente string) {
	if s.OnConversationStatus != nil {
		s.OnConversationStatus(ctx, convID, status, atendente)
	}
}

// Resolve: encerra o atendimento.
func (s *Service) Resolve(ctx context.Context, id int64) error {
	if err := s.repo.Resolve(ctx, id); err != nil {
		return err
	}
	// Dispara a mensagem de encerramento (se configurada e janela aberta).
	if s.OnResolved != nil {
		s.OnResolved(ctx, id)
	}
	// Espelha no chamado ligado (resolvido). O aviso ao solicitante já vem do
	// encerramento da conversa — a sincronização aqui é silenciosa (só status).
	s.notifyConvStatus(ctx, id, "resolvido", "")
	return nil
}

// ContatoDaConversa devolve telefone e nome do contato de uma conversa.
func (s *Service) ContatoDaConversa(ctx context.Context, convID int64) (string, string, error) {
	return s.repo.ConversationContact(ctx, convID)
}

// ConversationIDByPhone devolve o id da conversa de um telefone (0 se não há).
func (s *Service) ConversationIDByPhone(ctx context.Context, phone string) (int64, error) {
	return s.repo.IDByPhone(ctx, phone)
}

// ConversationMessages devolve as mensagens de uma conversa. marcarLida=false
// (ex.: atualização em segundo plano com a janela sem foco) mantém o contador de
// não-lidas — assim o atendente não "perde" a mensagem por deixar o chat aberto.
func (s *Service) ConversationMessages(ctx context.Context, convID int64, limit int, marcarLida bool) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	msgs, err := s.repo.ListMessagesByConversation(ctx, convID, limit)
	if err != nil {
		return nil, err
	}
	if marcarLida {
		_ = s.repo.MarkRead(ctx, convID) // leitura é best-effort; não falha o GET
	}
	return msgs, nil
}
