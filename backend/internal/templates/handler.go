package templates

import "github.com/gofiber/fiber/v2"

// Handler expõe a sincronização de templates da WABA.
type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// Register: listar é aberto; criar é admin/supervisor.
func (h *Handler) Register(r fiber.Router, gestor fiber.Handler) {
	r.Get("/templates", h.list)
	r.Post("/templates", gestor, h.create)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req struct {
		Nome      string   `json:"nome"`
		Idioma    string   `json:"idioma"`
		Categoria string   `json:"categoria"`
		Corpo     string   `json:"corpo"`
		Header    string   `json:"header"`
		Footer    string   `json:"footer"`
		Botoes    []string `json:"botoes"`
		Exemplos  []string `json:"exemplos"`
	}
	if err := c.BodyParser(&req); err != nil || req.Nome == "" || req.Corpo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome e corpo são obrigatórios"})
	}
	if req.Idioma == "" {
		req.Idioma = "pt_BR"
	}
	if req.Categoria == "" {
		req.Categoria = "UTILITY"
	}
	if err := h.client.Create(c.Context(), CreateRequest{
		Nome: req.Nome, Idioma: req.Idioma, Categoria: req.Categoria,
		Corpo: req.Corpo, Header: req.Header, Footer: req.Footer, Botoes: req.Botoes, Exemplos: req.Exemplos,
	}); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (h *Handler) list(c *fiber.Ctx) error {
	tpls, err := h.client.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"templates": tpls})
}
