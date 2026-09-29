package gateway

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

// Sender é o que o gateway usa para enviar no WhatsApp (messaging.Service).
type Sender interface {
	SendText(ctx context.Context, to, body string) (*messaging.Message, error)
	SendTemplate(ctx context.Context, to, name, lang string, params []string, preview string) (*messaging.Message, error)
	SendMedia(ctx context.Context, to, kind, link, caption, name string) (*messaging.Message, error)
}

// relayCloser encerra o relay de um telefone (satisfeito por *Repository).
type relayCloser interface {
	Clear(ctx context.Context, phone string) error
}

// Handler expõe as rotas /v1 que o SIGE chama (máquina-a-máquina, sem JWT).
type Handler struct {
	repo     relayCloser
	sender   Sender
	tenantID int64
	apiKey   string // GATEWAY_INBOUND_API_KEY
}

func NewHandler(repo relayCloser, sender Sender, tenantID int64, apiKey string) *Handler {
	return &Handler{repo: repo, sender: sender, tenantID: tenantID, apiKey: apiKey}
}

// Register monta /v1/* no app (fora do JWT), atrás da validação de X-API-Key.
func (h *Handler) Register(app fiber.Router) {
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
		msg, err = h.sender.SendMedia(c.Context(), in.Telefone, kindFromMime(in.MidiaMime), in.MidiaURL, in.Texto, "")
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
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"erro": "envio_falhou"})
		}
	}
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
