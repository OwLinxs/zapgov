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
	// o pacote messaging ao chatbot.
	OnInbound func(ctx context.Context, from, name, body string)

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
	if to == "" || body == "" {
		return nil, fmt.Errorf("destinatário e corpo são obrigatórios")
	}

	// LGPD: bloqueia envio a quem pediu opt-out.
	if out, _ := s.repo.ConversationOptedOut(ctx, to); out {
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

	res, err := s.providerFor(ctx).Send(ctx, OutgoingMessage{To: to, Body: body})
	if err != nil {
		return nil, fmt.Errorf("envio falhou: %w", err)
	}

	convID, err := s.repo.EnsureConversation(ctx, to, "")
	if err != nil {
		return nil, fmt.Errorf("garantir conversa: %w", err)
	}

	msg, err := s.repo.InsertOutbound(ctx, convID, res.WAMessageID, body)
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
	if out, _ := s.repo.IsOptedOut(ctx, to); out {
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
		return nil, fmt.Errorf("garantir conversa: %w", err)
	}

	body := preview
	if body == "" {
		body = "[modelo: " + name + "]"
	}
	msg, err := s.repo.InsertOutbound(ctx, convID, res.WAMessageID, body)
	if err != nil {
		return nil, fmt.Errorf("persistir mensagem: %w", err)
	}
	s.emit(ctx, convID, "message")
	return msg, nil
}

// SendMenu envia um menu interativo (botões/lista). Usado pelo bot.
func (s *Service) SendMenu(ctx context.Context, to, text string, options []MenuOption) (*Message, error) {
	if out, _ := s.repo.ConversationOptedOut(ctx, to); out {
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
	msg, err := s.repo.InsertOutbound(ctx, convID, res.WAMessageID, body)
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
	if out, _ := s.repo.ConversationOptedOut(ctx, to); out {
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
	if out, _ := s.repo.ConversationOptedOut(ctx, to); out {
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
			log.Printf("webhook: garantir conversa de %s: %v", in.From, err)
			continue
		}
		inserted, err := s.repo.InsertInbound(ctx, convID, in.WAMessageID, in.Body, in.Timestamp,
			MediaInfo{Type: in.MediaType, ID: in.MediaID, Mime: in.MediaMime, Name: in.MediaName})
		if err != nil {
			log.Printf("webhook: inserir inbound %s: %v", in.WAMessageID, err)
			continue
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
		// Resposta de pesquisa de satisfação: grava a nota, agradece e devolve a
		// conversa a resolvida — não reabre fila nem dispara bot.
		if s.OnSurveyAnswer != nil && s.OnSurveyAnswer(ctx, convID, in.ButtonID, in.Body, in.From) {
			_ = s.repo.Resolve(ctx, convID)
			s.emit(ctx, convID, "message")
			continue
		}

		s.emitInbound(ctx, convID, nome, preview)

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
			s.OnInbound(ctx, in.From, in.Name, in.Body)
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
		if err := s.repo.UpdateStatus(ctx, st.WAMessageID, st.Status, errText); err != nil {
			log.Printf("webhook: atualizar status %s: %v", st.WAMessageID, err)
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
func (s *Service) ListConversations(ctx context.Context, limit int, setor, status, q string, soAtendimento bool) ([]Conversation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListConversations(ctx, limit, setor, status, q, soAtendimento)
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
	return s.repo.Claim(ctx, id, atendente)
}

// ClaimIfQueued: assume a conversa apenas se estiver na fila.
func (s *Service) ClaimIfQueued(ctx context.Context, id int64, atendente string) error {
	return s.repo.ClaimIfQueued(ctx, id, atendente)
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
	return nil
}

// ContatoDaConversa devolve telefone e nome do contato de uma conversa.
func (s *Service) ContatoDaConversa(ctx context.Context, convID int64) (string, string, error) {
	return s.repo.ConversationContact(ctx, convID)
}

// ConversationMessages devolve as mensagens de uma conversa e a marca como lida.
func (s *Service) ConversationMessages(ctx context.Context, convID int64, limit int) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	msgs, err := s.repo.ListMessagesByConversation(ctx, convID, limit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkRead(ctx, convID); err != nil {
		return msgs, nil // leitura é best-effort; não falha o GET
	}
	return msgs, nil
}
