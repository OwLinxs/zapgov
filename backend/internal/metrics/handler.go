package metrics

import "github.com/gofiber/fiber/v2"

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

func (h *Handler) Register(r fiber.Router) {
	r.Get("/metrics/kpis", h.kpis)
	r.Get("/metrics/volume", h.volume)
	r.Get("/metrics/atendentes", h.atendentes)
	r.Get("/metrics/satisfacao", h.satisfacao)
	r.Get("/activity", h.activity)
}

// RegisterReports monta os relatórios CSV (admin/supervisor).
func (h *Handler) RegisterReports(r fiber.Router, gestor fiber.Handler) {
	r.Get("/reports/atendentes.csv", gestor, h.reportAtendentes)
	r.Get("/reports/setores.csv", gestor, h.reportSetores)
	r.Get("/reports/diario.csv", gestor, h.reportDiario)
	r.Get("/reports/satisfacao.csv", gestor, h.reportSatisfacao)
}

func (h *Handler) satisfacao(c *fiber.Ctx) error {
	s, err := h.repo.Satisfacao(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(s)
}

func (h *Handler) atendentes(c *fiber.Ctx) error {
	items, err := h.repo.Atendentes(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"atendentes": items})
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
