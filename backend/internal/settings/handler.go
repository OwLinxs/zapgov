package settings

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"zapgov/backend/internal/auth"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/tenants"
)

// MetaInfo expõe apenas metadados e indicadores de presença dos segredos Meta.
type MetaInfo struct {
	PhoneNumberID         string `json:"phone_number_id"`
	WABAID                string `json:"waba_id"`
	APIVersion            string `json:"api_version"`
	Connected             bool   `json:"connected"`
	TokenConfigured       bool   `json:"token_configured"`
	AppSecretConfigured   bool   `json:"app_secret_configured"`
	VerifyTokenConfigured bool   `json:"webhook_verify_token_configured"`
	Source                string `json:"source"`
}

type Handler struct {
	repo     *Repository
	meta     MetaInfo
	metaRepo *tenants.Repository
}

func NewHandler(repo *Repository, meta MetaInfo, metaRepo *tenants.Repository) *Handler {
	return &Handler{repo: repo, meta: meta, metaRepo: metaRepo}
}

// Register: leitura aberta a qualquer logado; escrita é admin.
func (h *Handler) Register(r fiber.Router, admin fiber.Handler) {
	r.Get("/settings/org", h.getOrg)
	r.Put("/settings/org", admin, h.putOrg)
	r.Get("/settings/encerramento", h.getEncerramento)
	r.Put("/settings/encerramento", admin, h.putEncerramento)
	r.Get("/settings/ausencia", h.getAusencia)
	r.Put("/settings/ausencia", admin, h.putAusencia)
	r.Get("/settings/assinatura", h.getAssinatura)
	r.Put("/settings/assinatura", admin, h.putAssinatura)
	r.Get("/settings/roteamento", h.getRoteamento)
	r.Put("/settings/roteamento", admin, h.putRoteamento)
	r.Get("/settings/visibilidade", h.getVisibilidade)
	r.Put("/settings/visibilidade", admin, h.putVisibilidade)
	r.Get("/settings/chatbot", h.getChatbot)
	r.Put("/settings/chatbot", admin, h.putChatbot)
	r.Get("/settings/lembrete", h.getLembrete)
	r.Put("/settings/lembrete", admin, h.putLembrete)
	r.Get("/settings/pesquisa", h.getPesquisa)
	r.Put("/settings/pesquisa", admin, h.putPesquisa)
	r.Get("/settings/meta", h.getMeta)
	r.Put("/settings/meta", auth.RequireRole("admin"), h.putMeta)
	r.Get("/settings/billing", h.getBilling)
	r.Get("/settings/apikey", admin, h.getApiKey)
	r.Post("/settings/apikey/regen", admin, h.regenApiKey)
	r.Get("/settings/webhooks", h.listWebhooks)
	r.Post("/settings/webhooks", admin, h.createWebhook)
	r.Delete("/settings/webhooks/:id", admin, h.deleteWebhook)
}

func (h *Handler) getOrg(c *fiber.Ctx) error {
	o, err := h.repo.GetOrg(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(o)
}

func (h *Handler) putOrg(c *fiber.Ctx) error {
	var b struct {
		Nome string `json:"nome"`
		UF   string `json:"uf"`
		Fuso string `json:"fuso"`
	}
	if err := c.BodyParser(&b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.UpdateOrg(c.Context(), b.Nome, b.UF, b.Fuso); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getEncerramento(c *fiber.Ctx) error {
	e, err := h.repo.GetEncerramento(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(e)
}

func (h *Handler) putEncerramento(c *fiber.Ctx) error {
	var e Encerramento
	if err := c.BodyParser(&e); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetEncerramento(c.Context(), e); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getAusencia(c *fiber.Ctx) error {
	a, err := h.repo.GetAusencia(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(a)
}

func (h *Handler) putAusencia(c *fiber.Ctx) error {
	var a Ausencia
	if err := c.BodyParser(&a); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetAusencia(c.Context(), a); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getAssinatura(c *fiber.Ctx) error {
	on, err := h.repo.GetAssinatura(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ativo": on})
}

func (h *Handler) putAssinatura(c *fiber.Ctx) error {
	var b struct {
		Ativo bool `json:"ativo"`
	}
	if err := c.BodyParser(&b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetAssinatura(c.Context(), b.Ativo); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getRoteamento(c *fiber.Ctx) error {
	rt, err := h.repo.GetRoteamento(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(rt)
}

func (h *Handler) putRoteamento(c *fiber.Ctx) error {
	var rt Roteamento
	if err := c.BodyParser(&rt); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetRoteamento(c.Context(), rt); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getVisibilidade(c *fiber.Ctx) error {
	g, err := h.repo.GetVisibilidade(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"grupos": g})
}

func (h *Handler) putVisibilidade(c *fiber.Ctx) error {
	var body struct {
		Grupos [][]string `json:"grupos"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetVisibilidade(c.Context(), body.Grupos); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getChatbot(c *fiber.Ctx) error {
	on, err := h.repo.GetChatbot(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ativo": on})
}

func (h *Handler) putChatbot(c *fiber.Ctx) error {
	var b struct {
		Ativo bool `json:"ativo"`
	}
	if err := c.BodyParser(&b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetChatbot(c.Context(), b.Ativo); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getLembrete(c *fiber.Ctx) error {
	l, err := h.repo.GetLembrete(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(l)
}

func (h *Handler) putLembrete(c *fiber.Ctx) error {
	var l Lembrete
	if err := c.BodyParser(&l); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetLembrete(c.Context(), l); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getPesquisa(c *fiber.Ctx) error {
	p, err := h.repo.GetPesquisa(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(p)
}

func (h *Handler) putPesquisa(c *fiber.Ctx) error {
	var p Pesquisa
	if err := c.BodyParser(&p); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetPesquisa(c.Context(), p); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getMeta(c *fiber.Ctx) error {
	visible := func(s string) string {
		if c.Locals("papel") == "admin" {
			return s
		}
		return mask(s)
	}
	m, err := h.metaRepo.Creds(c.Context(), tenantctx.From(c.Context()))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "não foi possível consultar as credenciais Meta"})
	}
	if m.PhoneNumberID != "" || m.Token != "" {
		return c.JSON(MetaInfo{
			PhoneNumberID: visible(m.PhoneNumberID), WABAID: visible(m.WABAID), APIVersion: m.APIVersion,
			Connected:       m.PhoneNumberID != "" && m.Token != "",
			TokenConfigured: m.Token != "", AppSecretConfigured: m.AppSecret != "",
			VerifyTokenConfigured: m.WebhookVerifyToken != "", Source: "painel",
		})
	}
	if tenantctx.From(c.Context()) != 1 {
		return c.JSON(MetaInfo{APIVersion: "v21.0"})
	}
	return c.JSON(MetaInfo{
		PhoneNumberID:   visible(h.meta.PhoneNumberID),
		WABAID:          visible(h.meta.WABAID),
		APIVersion:      h.meta.APIVersion,
		Connected:       h.meta.PhoneNumberID != "" && h.meta.Connected,
		TokenConfigured: h.meta.Connected, Source: "ambiente",
	})
}

func (h *Handler) putMeta(c *fiber.Ctx) error {
	var m tenants.MetaCreds
	if err := c.BodyParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	tid := tenantctx.From(c.Context())
	current, err := h.metaRepo.Creds(c.Context(), tid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "não foi possível consultar as credenciais Meta"})
	}
	m.PhoneNumberID = strings.TrimSpace(m.PhoneNumberID)
	m.WABAID = strings.TrimSpace(m.WABAID)
	m.APIVersion = strings.TrimSpace(m.APIVersion)
	if m.Token == "" {
		m.Token = current.Token
	}
	if m.AppSecret == "" {
		m.AppSecret = current.AppSecret
	}
	if m.WebhookVerifyToken == "" {
		m.WebhookVerifyToken = current.WebhookVerifyToken
	}
	if m.APIVersion == "" {
		m.APIVersion = current.APIVersion
	}
	if m.PhoneNumberID == "" || m.WABAID == "" || m.Token == "" || m.AppSecret == "" || m.WebhookVerifyToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "informe Phone Number ID, WABA ID, token, App Secret e Verify Token"})
	}
	if err := h.metaRepo.SetCreds(c.Context(), tid, m); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "não foi possível salvar as credenciais Meta"})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getBilling(c *fiber.Ctx) error {
	b, err := h.repo.BillingUsage(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(b)
}

func (h *Handler) getApiKey(c *fiber.Ctx) error {
	key, err := h.repo.EnsureApiKey(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"api_key": key})
}

func (h *Handler) regenApiKey(c *fiber.Ctx) error {
	key, err := h.repo.RegenApiKey(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"api_key": key})
}

func (h *Handler) listWebhooks(c *fiber.Ctx) error {
	items, err := h.repo.ListWebhooks(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"webhooks": items})
}

func (h *Handler) createWebhook(c *fiber.Ctx) error {
	var b struct {
		URL     string   `json:"url"`
		Eventos []string `json:"eventos"`
	}
	if err := c.BodyParser(&b); err != nil || b.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url é obrigatória"})
	}
	if b.Eventos == nil {
		b.Eventos = []string{}
	}
	w, err := h.repo.CreateWebhook(c.Context(), b.URL, b.Eventos)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(w)
}

func (h *Handler) deleteWebhook(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.repo.DeleteWebhook(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// mask mostra só os últimos 4 caracteres (•••• 4521).
func mask(s string) string {
	if len(s) <= 4 {
		return s
	}
	return "•••• " + s[len(s)-4:]
}
