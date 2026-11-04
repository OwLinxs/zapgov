package gateway

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

// Sender é o que o gateway usa para enviar no WhatsApp (messaging.Service).
type Sender interface {
	SendText(ctx context.Context, to, body string) (*messaging.Message, error)
	SendTemplate(ctx context.Context, to, name, lang string, params []string, preview string) (*messaging.Message, error)
	SendMediaUpload(ctx context.Context, to, kind string, data []byte, filename, mime, caption string) (*messaging.Message, error)
	DownloadMedia(ctx context.Context, mediaID string) ([]byte, string, error)
}

// baixador busca no SIGE o arquivo de POST /v1/mensagens tipo=midia (*Baixador).
type baixador interface {
	Baixar(ctx context.Context, rawURL, mime, nome string) (*Arquivo, error)
}

// relayCloser encerra o relay de um telefone e marca movimento do técnico
// (satisfeito por *Repository).
type relayCloser interface {
	Clear(ctx context.Context, phone string) error
	Touch(ctx context.Context, phone string) error
}

// Handler expõe as rotas /v1 que o SIGE chama (máquina-a-máquina, sem JWT).
type Handler struct {
	repo      relayCloser
	sender    Sender
	tenantID  int64
	apiKey    string // GATEWAY_INBOUND_API_KEY
	assinador *Assinador
	baixador  baixador
}

func NewHandler(repo relayCloser, sender Sender, tenantID int64, apiKey string, assinador *Assinador, baixador baixador) *Handler {
	return &Handler{repo: repo, sender: sender, tenantID: tenantID, apiKey: apiKey, assinador: assinador, baixador: baixador}
}

// Register monta /v1/* no app (fora do JWT), atrás da validação de X-API-Key.
// GET /v1/midia/:token é registrado antes do grupo: a autorização é o próprio
// token assinado (o SIGE pode baixar ou exibir sem mandar a chave).
func (h *Handler) Register(app fiber.Router) {
	app.Get("/v1/midia/:token", h.midia)
	g := app.Group("/v1", h.auth)
	g.Post("/mensagens", h.enviar)
	g.Post("/relay/encerrar", h.encerrar)
}

// auth valida o X-API-Key e injeta o tenant do gateway no contexto.
func (h *Handler) auth(c *fiber.Ctx) error {
	got := []byte(c.Get("X-API-Key"))
	want := []byte(h.apiKey)
	if len(got) == 0 || len(want) == 0 || subtle.ConstantTimeCompare(got, want) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "chave_invalida"})
	}
	c.Locals(tenantctx.Key, h.tenantID) // repos/serviços leem via c.Context()
	return c.Next()
}

type enviarReq struct {
	Telefone  string   `json:"telefone"`
	Tipo      string   `json:"tipo"` // texto | midia | template
	Texto     string   `json:"texto"`
	MidiaURL  string   `json:"midia_url"`
	MidiaMime string   `json:"midia_mime"`
	MidiaNome string   `json:"midia_nome"` // opcional: nome do arquivo (documento)
	Template  string   `json:"template"`
	Params    []string `json:"params"`
}

func (h *Handler) enviar(c *fiber.Ctx) error {
	var in enviarReq
	if err := c.BodyParser(&in); err != nil || strings.TrimSpace(in.Telefone) == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "dados_invalidos"})
	}
	var msg *messaging.Message
	var err error
	switch in.Tipo {
	case "texto":
		if strings.TrimSpace(in.Texto) == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "dados_invalidos"})
		}
		msg, err = h.sender.SendText(c.Context(), in.Telefone, in.Texto)
	case "template":
		if strings.TrimSpace(in.Template) == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "dados_invalidos"})
		}
		msg, err = h.sender.SendTemplate(c.Context(), in.Telefone, in.Template, "pt_BR", in.Params, in.Texto)
	case "midia":
		if strings.TrimSpace(in.MidiaURL) == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "dados_invalidos"})
		}
		arq, berr := h.baixador.Baixar(c.Context(), in.MidiaURL, in.MidiaMime, in.MidiaNome)
		switch {
		case errors.Is(berr, errMidiaOrigem), errors.Is(berr, errMidiaGrande):
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "midia_invalida", "detalhe": berr.Error()})
		case berr != nil:
			slog.Warn("gateway: baixar mídia do SIGE", "telefone", in.Telefone, "err", berr)
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"erro": "midia_indisponivel"})
		}
		msg, err = h.sender.SendMediaUpload(c.Context(), in.Telefone, kindFromMime(arq.Mime), arq.Dados, arq.Nome, arq.Mime, in.Texto)
	default:
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "dados_invalidos"})
	}
	if err != nil {
		switch {
		case errors.Is(err, messaging.ErrWindowClosed):
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "janela_fechada"})
		case errors.Is(err, messaging.ErrOptedOut):
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "opt_out"})
		default:
			slog.Warn("gateway: envio ao WhatsApp falhou", "telefone", in.Telefone, "tipo", in.Tipo, "err", err)
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"erro": "envio_falhou"})
		}
	}
	_ = h.repo.Touch(c.Context(), in.Telefone) // técnico respondeu: relay segue vivo
	return c.JSON(fiber.Map{"wa_message_id": msg.WAMessageID})
}

func (h *Handler) encerrar(c *fiber.Ctx) error {
	var in struct {
		Telefone string `json:"telefone"`
	}
	if err := c.BodyParser(&in); err != nil || strings.TrimSpace(in.Telefone) == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"erro": "dados_invalidos"})
	}
	_ = h.repo.Clear(c.Context(), in.Telefone)
	return c.JSON(fiber.Map{"ok": true})
}

// midia serve a mídia recebida da pessoa (URL assinada em mensagem.recebida).
func (h *Handler) midia(c *fiber.Ctx) error {
	ref, err := h.assinador.Validar(c.Params("token"))
	if err != nil || ref.TenantID != h.tenantID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "nao_encontrada"})
	}
	ctx := tenantctx.With(c.Context(), ref.TenantID)
	data, mt, err := h.sender.DownloadMedia(ctx, ref.MediaID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"erro": "midia_indisponivel"})
	}
	if mt == "" {
		mt = ref.Mime
	}
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Set("Cache-Control", "private, no-store")
	nome := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(ref.Nome)
	if mimeSeguroInline(mt) {
		c.Set("Content-Type", mt)
		if nome != "" {
			c.Set("Content-Disposition", `inline; filename="`+nome+`"`)
		}
	} else {
		// html/svg/etc. nunca são renderizados no domínio do ZapGov.
		c.Set("Content-Type", "application/octet-stream")
		if nome == "" {
			nome = "arquivo"
		}
		c.Set("Content-Disposition", `attachment; filename="`+nome+`"`)
	}
	return c.Send(data)
}

func kindFromMime(m string) string {
	switch {
	case strings.HasPrefix(m, "image/"):
		return "image"
	case strings.HasPrefix(m, "audio/"):
		return "audio"
	case strings.HasPrefix(m, "video/"):
		return "video"
	default:
		return "document"
	}
}
