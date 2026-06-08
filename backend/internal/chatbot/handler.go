package chatbot

import "github.com/gofiber/fiber/v2"

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register: ver é aberto; configurar o bot simples é admin.
func (h *Handler) Register(r fiber.Router, admin fiber.Handler) {
	r.Get("/bot", h.get)
	r.Put("/bot", admin, h.updateConfig)
	r.Post("/bot/options", admin, h.addOption)
	r.Delete("/bot/options/:id", admin, h.delOption)
}

func (h *Handler) get(c *fiber.Ctx) error {
	cfg, err := h.svc.Config(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	opts, err := h.svc.Options(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"config": cfg, "options": opts})
}

func (h *Handler) updateConfig(c *fiber.Ctx) error {
	var cfg Config
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.svc.SaveConfig(c.Context(), cfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) addOption(c *fiber.Ctx) error {
	var o Option
	if err := c.BodyParser(&o); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if o.Chave == "" || o.Label == "" || o.Resposta == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "chave, label e resposta são obrigatórios"})
	}
	out, err := h.svc.AddOption(c.Context(), o)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) delOption(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.svc.RemoveOption(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
