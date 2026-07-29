package tenants

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	repo *Repository
	// seedAdmin cria o admin inicial de um tenant recém-provisionado.
	seedAdmin func(ctx context.Context, tenantID int64, email, senha string)
}

func NewHandler(repo *Repository, seedAdmin func(ctx context.Context, tenantID int64, email, senha string)) *Handler {
	return &Handler{repo: repo, seedAdmin: seedAdmin}
}

// RegisterPublic monta a rota aberta usada pela tela de login para listar os
// painéis disponíveis (sem autenticação, sem dados sensíveis).
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Get("/tenants/options", h.options)
}

func (h *Handler) options(c *fiber.Ctx) error {
	opts, err := h.repo.PublicOptions(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"options": opts})
}

// Register monta as rotas de Super Admin (cross-tenant). O guard de plataforma
// é aplicado pelo chamador (main).
func (h *Handler) Register(r fiber.Router, platformAdmin fiber.Handler) {
	r.Get("/admin/tenants", platformAdmin, h.list)
	r.Post("/admin/tenants", platformAdmin, h.create)
	r.Put("/admin/tenants/:id/plano", platformAdmin, h.updatePlano)
	r.Put("/admin/tenants/:id/modulos", platformAdmin, h.setModulos)
	r.Put("/admin/tenants/:id/meta", platformAdmin, h.setCreds)
	r.Post("/admin/tenants/:id/suspend", platformAdmin, h.suspend)
	r.Post("/admin/tenants/:id/activate", platformAdmin, h.activate)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"tenants": items})
}

func (h *Handler) create(c *fiber.Ctx) error {
	var body struct {
		Nome       string `json:"nome"`
		Subdominio string `json:"subdominio"`
		Plano      string `json:"plano"`
		AdminEmail string `json:"admin_email"`
		AdminSenha string `json:"admin_senha"`
	}
	if err := c.BodyParser(&body); err != nil || body.Nome == "" || body.Subdominio == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome e subdomínio são obrigatórios"})
	}
	t, err := h.repo.Create(c.Context(), body.Nome, body.Subdominio, body.Plano)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	// Seed do admin inicial do novo tenant.
	if h.seedAdmin != nil {
		h.seedAdmin(c.Context(), t.ID, body.AdminEmail, body.AdminSenha)
	}
	return c.Status(fiber.StatusCreated).JSON(t)
}

func (h *Handler) updatePlano(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Plano  string `json:"plano"`
		Limite int    `json:"limite_mensal"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.UpdatePlano(c.Context(), int64(id), body.Plano, body.Limite); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) setModulos(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Modulos map[string]bool `json:"modulos"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetModulos(c.Context(), int64(id), body.Modulos); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) setCreds(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var m MetaCreds
	if err := c.BodyParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetCreds(c.Context(), int64(id), m); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) suspend(c *fiber.Ctx) error { return h.setActive(c, false) }
func (h *Handler) activate(c *fiber.Ctx) error { return h.setActive(c, true) }

func (h *Handler) setActive(c *fiber.Ctx, ativo bool) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if id == 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "o tenant da plataforma não pode ser suspenso"})
	}
	if err := h.repo.SetActive(c.Context(), int64(id), ativo); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
