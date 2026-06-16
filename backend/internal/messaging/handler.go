package messaging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Handler expõe as rotas HTTP de mensageria. Só traduz HTTP ↔ Service.
type Handler struct {
	svc *Service
	// SignatureEnabled, se definido, diz se a assinatura do atendente deve ser
	// anexada às mensagens enviadas manualmente (config por tenant).
	SignatureEnabled func(ctx context.Context) bool
	// RouteBySetor, se definido e true, restringe o atendente às conversas do
	// próprio setor (config por tenant).
	RouteBySetor func(ctx context.Context) bool
	// TagAttendant, se definido, aplica uma etiqueta com o nome do atendente ao
	// assumir a conversa (config por tenant; no-op se desligado).
	TagAttendant func(ctx context.Context, convID int64, nome string)
	// TemplateAllowed, se definido, diz se um template está liberado para o setor
	// (trava de template por departamento). nil = sem trava.
	TemplateAllowed func(ctx context.Context, template, setor string) bool
	// EnsureContact, se definido, garante o telefone no cadastro de contatos
	// (auto-cadastro ao iniciar conversa com número avulso).
	EnsureContact func(ctx context.Context, nome, telefone string)
	// OnDateInTemplate, se definido, é chamado quando uma variável do template
	// enviado contém uma data futura (DD/MM/AAAA) — agenda lembrete da véspera.
	OnDateInTemplate func(ctx context.Context, phone, nome string, data time.Time)
}

// dataBR reconhece DD/MM/AAAA dentro de um texto.
var dataBR = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)

// acharData devolve a primeira data futura encontrada nas variáveis.
func acharData(params []string) (time.Time, bool) {
	for _, p := range params {
		m := dataBR.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		t, err := time.Parse("02/01/2006", m[0])
		if err != nil {
			continue
		}
		return t, true
	}
	return time.Time{}, false
}

func (h *Handler) routeBySetor(c *fiber.Ctx) bool {
	return h.RouteBySetor != nil && h.RouteBySetor(c.Context())
}

func (h *Handler) tag(c *fiber.Ctx, convID int64) {
	if h.TagAttendant != nil {
		h.TagAttendant(c.Context(), convID, operador(c))
	}
}

// setorEfetivo decide o filtro de setor da listagem: com o roteamento por setor
// ligado, o atendente é FORÇADO ao próprio setor (ignora o filtro pedido);
// atendente sem setor definido vê tudo (não trava acesso); admin/supervisor
// usam o filtro que pediram. Função pura — testável sem HTTP.
func setorEfetivo(papel, setorUsuario, setorPedido string, rotear bool) string {
	if papel == "atendente" && rotear && setorUsuario != "" {
		return setorUsuario
	}
	return setorPedido
}

// herdaSetor grava o setor do atendente na conversa (se ela ainda não tiver).
// Sem isso, conversa iniciada por template nasce sem setor e some da lista de
// quem a criou quando o roteamento por setor está ligado.
func (h *Handler) herdaSetor(c *fiber.Ctx, convID int64) {
	if setor, _ := c.Locals("setor").(string); setor != "" {
		_ = h.svc.SetSetorIfEmpty(c.Context(), convID, setor)
	}
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// assina anexa "— *Nome*" ao corpo se a assinatura estiver ativa para o tenant.
func (h *Handler) assina(c *fiber.Ctx, body string) string {
	if body == "" || h.SignatureEnabled == nil || !h.SignatureEnabled(c.Context()) {
		return body
	}
	return "*" + operador(c) + "*\n" + body
}

// Register registra as rotas autenticadas de mensageria.
// (Auth/JWT entra numa fatia posterior; aqui as rotas estão abertas no dev.)
func (h *Handler) Register(r fiber.Router) {
	r.Post("/messages/send", h.sendMessage)
	r.Post("/messages/template", h.sendTemplate)
	r.Post("/messages/media", h.sendMedia)
	r.Post("/messages/upload", h.uploadMedia)
	r.Get("/media/:id", h.getMedia)
	r.Get("/messages", h.listMessages)
	r.Get("/conversations", h.listConversations)
	r.Get("/conversations/:id/messages", h.conversationMessages)
	r.Get("/conversations/:id/sessions", h.conversationSessions)
	r.Get("/sessions/:id/messages", h.sessionMessages)
	r.Put("/conversations/:id/claim", h.claim)
	r.Put("/conversations/:id/transfer", h.transfer)
	r.Post("/conversations/:id/note", h.addNote)
	r.Post("/conversations/:id/react", h.react)
	r.Put("/conversations/:id/resolve", h.resolve)
	r.Put("/conversations/:id/optout", h.optOut)
	r.Put("/conversations/:id/optin", h.optIn)
	r.Put("/conversations/:id/labels", h.setLabels)
	r.Post("/conversations/resolve-bulk", h.resolveBulk)
}

func (h *Handler) resolveBulk(c *fiber.Ctx) error {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	n, err := h.svc.ResolveBulk(c.Context(), body.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"resolvidas": n})
}

func (h *Handler) setLabels(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		LabelIDs []int64 `json:"label_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.svc.SetLabels(c.Context(), int64(id), body.LabelIDs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) optOut(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.svc.OptOutConversation(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) optIn(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.svc.OptInConversation(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

type sendRequest struct {
	To      string `json:"to"`
	Body    string `json:"body"`
	ReplyTo string `json:"reply_to"` // wa_message_id citado (responder mensagem)
}

func (h *Handler) sendMessage(c *fiber.Ctx) error {
	var req sendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}

	msg, err := h.svc.SendTextReply(c.Context(), req.To, h.assina(c, req.Body), req.ReplyTo)
	if errors.Is(err, ErrWindowClosed) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error(), "window_closed": true})
	}
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	// Respondeu uma conversa da fila → assume automaticamente.
	_ = h.svc.ClaimIfQueued(c.Context(), msg.ConversationID, operador(c))
	h.tag(c, msg.ConversationID)
	h.herdaSetor(c, msg.ConversationID)
	return c.Status(fiber.StatusCreated).JSON(msg)
}

// operador devolve o nome do atendente autenticado (para auto-atribuição).
func operador(c *fiber.Ctx) string {
	if n, _ := c.Locals("nome").(string); n != "" {
		return n
	}
	return "Atendente"
}

type templateRequest struct {
	To       string   `json:"to"`
	Nome     string   `json:"nome"` // nome do destinatário (auto-cadastro de avulso)
	Template string   `json:"template"`
	Language string   `json:"language"`
	Params   []string `json:"params"`
	Preview  string   `json:"preview"`
}

func (h *Handler) sendTemplate(c *fiber.Ctx) error {
	var req templateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	// Trava por departamento: atendente só dispara template liberado pro setor dele.
	if papel, _ := c.Locals("papel").(string); papel == "atendente" && h.TemplateAllowed != nil {
		setor, _ := c.Locals("setor").(string)
		if !h.TemplateAllowed(c.Context(), req.Template, setor) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "template não liberado para o seu departamento"})
		}
	}
	msg, err := h.svc.SendTemplate(c.Context(), req.To, req.Template, req.Language, req.Params, req.Preview)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	// Conversa iniciada por nós (operador) → já entra em atendimento com quem iniciou.
	// Assim, a resposta do cidadão NÃO dispara o bot (já está com humano).
	_ = h.svc.Claim(c.Context(), msg.ConversationID, operador(c))
	h.tag(c, msg.ConversationID)
	// Template inicia um NOVO atendimento → a conversa vai pro setor de quem
	// iniciou, mesmo que tivesse setor de um atendimento antigo (senão some da
	// lista do atendente com o roteamento por setor ligado).
	if setor, _ := c.Locals("setor").(string); setor != "" {
		_ = h.svc.SetSetor(c.Context(), msg.ConversationID, setor)
	}
	// Auto-cadastro: garante o destinatário nos contatos e o nome na conversa.
	_ = h.svc.SetContactNameIfEmpty(c.Context(), msg.ConversationID, req.Nome)
	if h.EnsureContact != nil {
		h.EnsureContact(c.Context(), req.Nome, CanonPhone(req.To))
	}
	// Lembrete automático: variável com data futura (DD/MM/AAAA) agenda a véspera.
	if h.OnDateInTemplate != nil {
		if data, ok := acharData(req.Params); ok {
			h.OnDateInTemplate(c.Context(), CanonPhone(req.To), req.Nome, data)
		}
	}
	return c.Status(fiber.StatusCreated).JSON(msg)
}

type mediaRequest struct {
	To      string `json:"to"`
	Kind    string `json:"kind"` // image|document|audio|video
	Link    string `json:"link"`
	Caption string `json:"caption"`
	Name    string `json:"name"`
}

func (h *Handler) sendMedia(c *fiber.Ctx) error {
	var req mediaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if req.Kind == "" {
		req.Kind = "image"
	}
	msg, err := h.svc.SendMedia(c.Context(), req.To, req.Kind, req.Link, h.assina(c, req.Caption), req.Name)
	if errors.Is(err, ErrWindowClosed) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error(), "window_closed": true})
	}
	if errors.Is(err, ErrOptedOut) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	_ = h.svc.ClaimIfQueued(c.Context(), msg.ConversationID, operador(c))
	h.tag(c, msg.ConversationID)
	h.herdaSetor(c, msg.ConversationID)
	return c.Status(fiber.StatusCreated).JSON(msg)
}

// uploadMedia recebe um arquivo (multipart "file"), sobe pra Meta e envia.
func (h *Handler) uploadMedia(c *fiber.Ctx) error {
	to := c.FormValue("to")
	caption := c.FormValue("caption")
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "arquivo é obrigatório (campo 'file')"})
	}
	f, err := fh.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	mime := fh.Header.Get("Content-Type")

	// Áudio gravado no navegador: converte SEMPRE pra AAC/M4A mono (audio/mp4),
	// o formato de áudio mais aceito pela Meta. Navegadores gravam webm/opus ou
	// mp4/opus, recusados com 131053.
	if strings.HasPrefix(mime, "audio/") {
		if conv, err := toM4A(c.Context(), data); err == nil {
			data, mime = conv, "audio/mp4"
			if i := strings.LastIndexByte(fh.Filename, '.'); i > 0 {
				fh.Filename = fh.Filename[:i]
			}
			fh.Filename += ".m4a"
		} else {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "conversão de áudio falhou: " + err.Error()})
		}
	}
	kind := mediaKind(mime)

	msg, err := h.svc.SendMediaUpload(c.Context(), to, kind, data, fh.Filename, mime, h.assina(c, caption))
	if errors.Is(err, ErrWindowClosed) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error(), "window_closed": true})
	}
	if errors.Is(err, ErrOptedOut) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	_ = h.svc.ClaimIfQueued(c.Context(), msg.ConversationID, operador(c))
	h.tag(c, msg.ConversationID)
	h.herdaSetor(c, msg.ConversationID)
	return c.Status(fiber.StatusCreated).JSON(msg)
}

// toM4A converte áudio (webm/mp4/ogg/etc) para AAC em M4A mono — formato
// amplamente aceito pela Meta. O ffmpeg detecta o container de entrada sozinho.
func toM4A(ctx context.Context, in []byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "zg-audio-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "in.bin")
	dst := filepath.Join(dir, "out.m4a")
	if err := os.WriteFile(src, in, 0o600); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", src,
		"-vn", "-c:a", "aac", "-b:a", "64k", "-ar", "44100", "-ac", "1",
		"-movflags", "+faststart", "-y", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %v (%s)", err, lastLine(out))
	}
	return os.ReadFile(dst)
}

func lastLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

// mediaKind mapeia o mime para o tipo aceito pela Meta (image/audio/video/document).
func mediaKind(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	default:
		return "document"
	}
}

// getMedia baixa a mídia da Meta e devolve os bytes (proxy autenticado).
func (h *Handler) getMedia(c *fiber.Ctx) error {
	data, mime, err := h.svc.DownloadMedia(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	if mime != "" {
		c.Set("Content-Type", mime)
	}
	c.Set("Cache-Control", "private, max-age=86400")
	return c.Send(data)
}

func (h *Handler) listMessages(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	msgs, err := h.svc.ListRecent(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"messages": msgs})
}

func (h *Handler) listConversations(c *fiber.Ctx) error {
	setor := c.Query("setor")
	if setor == "Todos" {
		setor = ""
	}
	// Atendente não enxerga conversas em fase de bot — só fila/atendimento/resolvidas.
	papel, _ := c.Locals("papel").(string)
	soAtendimento := papel == "atendente"
	userSetor, _ := c.Locals("setor").(string)
	setor = setorEfetivo(papel, userSetor, setor, h.routeBySetor(c))
	convs, err := h.svc.ListConversations(c.Context(), c.QueryInt("limit", 100), setor, c.Query("status"), c.Query("q"), soAtendimento)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"conversations": convs})
}

func (h *Handler) conversationSessions(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	sess, err := h.svc.ListSessions(c.Context(), int64(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"sessions": sess})
}

func (h *Handler) sessionMessages(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	msgs, err := h.svc.SessionMessages(c.Context(), int64(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"messages": msgs})
}

func (h *Handler) claim(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Atendente string `json:"atendente"`
	}
	_ = c.BodyParser(&body)
	if err := h.svc.Claim(c.Context(), int64(id), body.Atendente); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.tag(c, int64(id))
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) react(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		WAMessageID string `json:"wa_message_id"`
		Emoji       string `json:"emoji"` // vazio remove a reação
	}
	if err := c.BodyParser(&body); err != nil || body.WAMessageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "wa_message_id é obrigatório"})
	}
	if err := h.svc.React(c.Context(), int64(id), body.WAMessageID, body.Emoji); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) addNote(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := c.BodyParser(&body); err != nil || body.Body == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "texto da nota é obrigatório"})
	}
	msg, err := h.svc.AddNote(c.Context(), int64(id), operador(c), body.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(msg)
}

func (h *Handler) transfer(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Setor string `json:"setor"`
	}
	if err := c.BodyParser(&body); err != nil || body.Setor == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "setor é obrigatório"})
	}
	if err := h.svc.Transfer(c.Context(), int64(id), body.Setor); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) resolve(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.svc.Resolve(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) conversationMessages(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	// read=0 não marca como lida (atualização em segundo plano sem foco).
	marcarLida := c.QueryInt("read", 1) != 0
	msgs, err := h.svc.ConversationMessages(c.Context(), int64(id), c.QueryInt("limit", 200), marcarLida)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"messages": msgs})
}
