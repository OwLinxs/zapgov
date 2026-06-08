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

// receive processa eventos. SEMPRE responde 200 para a Meta — erros internos são
// logados; a idempotência (dedup por wa_message_id) cobre reentregas.
func (h *Handler) receive(c *fiber.Ctx) error {
	body := c.Body()

	// Roteia o evento para o tenant dono do número (default 1) ANTES de validar
	// a assinatura — o HMAC usa o app_secret daquele tenant.
	tid := int64(1)
	if h.resolveTenant != nil {
		if got := h.resolveTenant(phoneNumberIDFromBody(body)); got > 0 {
			tid = got
		}
	}
	ctxT := tenantctx.With(c.Context(), tid)

	// Provider do tenant (credenciais próprias) ou o padrão (.env, tenant 1).
	prov := h.provider
	if h.resolveProvider != nil {
		if p := h.resolveProvider(ctxT); p != nil {
			prov = p
		}
	}
	if !prov.VerifySignature(body, c.Get("X-Hub-Signature-256")) {
		log.Printf("webhook: assinatura HMAC inválida (tenant %d) — descartando", tid)
		return c.SendStatus(fiber.StatusForbidden)
	}

	if err := h.svc.ProcessWebhook(ctxT, body); err != nil {
		log.Printf("webhook: erro ao processar (tenant %d): %v", tid, err)
	}
	return c.SendStatus(fiber.StatusOK)
}
