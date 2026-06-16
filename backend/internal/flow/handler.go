package flow

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: ver o fluxo é aberto; editar (salvar grafo) é admin.
func (h *Handler) Register(r fiber.Router, admin fiber.Handler) {
	r.Get("/flows", h.list)
	r.Get("/flow", h.active) // fluxo ativo (para o canvas)
	r.Post("/flows", admin, h.create)
	r.Put("/flows/:id", admin, h.saveGraph)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var body struct {
		Nome string `json:"nome"`
	}
	_ = c.BodyParser(&body)
	f, err := h.repo.Create(c.Context(), body.Nome)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(f)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"flows": items})
}

func (h *Handler) active(c *fiber.Ctx) error {
	fl, err := h.repo.ActiveFlow(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if fl == nil {
		return c.JSON(fiber.Map{"flow": nil})
	}
	return c.JSON(fiber.Map{"flow": fl})
}

func (h *Handler) saveGraph(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Graph json.RawMessage `json:"graph"`
	}
	if err := c.BodyParser(&body); err != nil || len(body.Graph) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "graph é obrigatório"})
	}
	// valida o grafo antes de salvar
	if _, err := Parse(body.Graph); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "grafo inválido: " + err.Error()})
	}
	if err := h.repo.SaveGraph(c.Context(), int64(id), body.Graph); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
