// Package webhook recebe os callbacks da Meta (verificação e eventos).
package webhook

import (
	"context"
	"encoding/json"
	"log"

	"github.com/gofiber/fiber/v2"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

type Handler struct {
	svc         *messaging.Service
	provider    messaging.MessageProvider
	verifyToken string
	Accept      func(context.Context, []Envelope) error
	// resolveTenant mapeia o número da Meta (phone_number_id) -> tenant_id.
	// 0/desconhecido cai no tenant 1.
	resolveTenant func(phoneNumberID string) int64
	// resolveProvider devolve o provider (credenciais) do tenant do context,
	// usado para validar o HMAC com o app_secret correto.
	resolveProvider func(ctx context.Context) messaging.MessageProvider
}

func NewHandler(svc *messaging.Service, provider messaging.MessageProvider, verifyToken string) *Handler {
	return &Handler{svc: svc, provider: provider, verifyToken: verifyToken}
}

// SetTenantResolver liga o roteamento de entrada por número.
func (h *Handler) SetTenantResolver(fn func(phoneNumberID string) int64) {
	h.resolveTenant = fn
}

// SetProviderResolver liga a resolução do provider por tenant (HMAC).
func (h *Handler) SetProviderResolver(fn func(ctx context.Context) messaging.MessageProvider) {
	h.resolveProvider = fn
}

// phoneNumberIDFromBody extrai metadata.phone_number_id do payload da Meta
// (entry[].changes[].value.metadata.phone_number_id) para rotear ao tenant.
func phoneNumberIDFromBody(body []byte) string {
	var p struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						PhoneNumberID string `json:"phone_number_id"`
					} `json:"metadata"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &p) != nil {
		return ""
	}
	for _, e := range p.Entry {
		for _, ch := range e.Changes {
			if id := ch.Value.Metadata.PhoneNumberID; id != "" {
				return id
			}
		}
	}
	return ""
}

// Register monta as rotas do webhook (SEM auth JWT — a Meta chama diretamente).
func (h *Handler) Register(r fiber.Router) {
	r.Get("/webhook", h.verify)
	r.Post("/webhook", h.receive)
}

// verify responde ao handshake de assinatura da Meta:
// GET /webhook?hub.mode=subscribe&hub.verify_token=...&hub.challenge=...
func (h *Handler) verify(c *fiber.Ctx) error {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == h.verifyToken && h.verifyToken != "" {
		return c.SendString(challenge)
	}
	return c.SendStatus(fiber.StatusForbidden)
}

// receive only acknowledges events after durable storage.
func (h *Handler) receive(c *fiber.Ctx) error {
	body := c.Body()
	parts, err := splitPayload(body)
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	batch := make([]Envelope, 0, len(parts))
	for _, part := range parts {
		tid := int64(0)
		if h.resolveTenant != nil {
			tid = h.resolveTenant(phoneNumberIDFromBody(part))
		}
		if tid <= 0 {
			return c.SendStatus(fiber.StatusForbidden)
		}
		prov := h.provider
		if h.resolveProvider != nil {
			prov = h.resolveProvider(tenantctx.With(c.Context(), tid))
		}
		if prov == nil || !prov.VerifySignature(body, c.Get("X-Hub-Signature-256")) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		batch = append(batch, Envelope{TenantID: tid, Body: part})
	}
	if h.Accept == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if err := h.Accept(c.Context(), batch); err != nil {
		log.Printf("webhook: persistir lote: %v", err)
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return c.SendStatus(fiber.StatusOK)
}
