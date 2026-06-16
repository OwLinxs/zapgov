package kb

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: leitura é aberta (qualquer usuário interno); criar/editar/arquivar é
// para gestão (admin/supervisor) via o middleware passado.
func (h *Handler) Register(r fiber.Router, gestor fiber.Handler) {
	r.Get("/kb", h.list)
	r.Get("/kb/categorias", h.categorias)
	r.Get("/kb/:id", h.get)
	r.Post("/kb/:id/util", h.vota)
	r.Post("/kb", gestor, h.create)
	r.Put("/kb/:id", gestor, h.update)
	r.Delete("/kb/:id", gestor, h.arquiva)
}

func operador(c *fiber.Ctx) string {
	if n, _ := c.Locals("nome").(string); n != "" {
		return n
	}
	return ""
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context(), c.Query("q"), c.Query("categoria"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"articles": items})
}

func (h *Handler) categorias(c *fiber.Ctx) error {
	cats, err := h.repo.Categorias(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"categorias": cats})
}

func (h *Handler) get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	a, err := h.repo.Get(c.Context(), int64(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "artigo não encontrado"})
	}
	return c.JSON(a)
}

type articleInput struct {
	Titulo    string   `json:"titulo"`
	Problema  string   `json:"problema"`
	Solucao   string   `json:"solucao"`
	Categoria string   `json:"categoria"`
	Tags      []string `json:"tags"`
	TicketID  *int64   `json:"ticket_id"`
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in articleInput
	if err := c.BodyParser(&in); err != nil || in.Titulo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "título é obrigatório"})
	}
	a, err := h.repo.Create(c.Context(), Article{
		Titulo: in.Titulo, Problema: in.Problema, Solucao: in.Solucao,
		Categoria: in.Categoria, Tags: in.Tags, TicketID: in.TicketID, Autor: operador(c),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(a)
}

func (h *Handler) update(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var in articleInput
	if err := c.BodyParser(&in); err != nil || in.Titulo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "título é obrigatório"})
	}
	a, err := h.repo.Update(c.Context(), int64(id), Article{
		Titulo: in.Titulo, Problema: in.Problema, Solucao: in.Solucao,
		Categoria: in.Categoria, Tags: in.Tags,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(a)
}

func (h *Handler) arquiva(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.repo.Arquiva(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) vota(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.repo.Vota(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
