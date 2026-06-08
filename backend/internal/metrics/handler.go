package metrics

import "github.com/gofiber/fiber/v2"

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

func (h *Handler) Register(r fiber.Router) {
	r.Get("/metrics/kpis", h.kpis)
	r.Get("/metrics/volume", h.volume)
	r.Get("/activity", h.activity)
}

func (h *Handler) kpis(c *fiber.Ctx) error {
	k, err := h.repo.KPIs(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(k)
}

func (h *Handler) volume(c *fiber.Ctx) error {
	rng := c.Query("range", "hora")
	pts, err := h.repo.Volume(c.Context(), rng)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"pontos": pts})
}

func (h *Handler) activity(c *fiber.Ctx) error {
	items, err := h.repo.Atividade(c.Context(), c.QueryInt("limit", 8))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"atividade": items})
}
