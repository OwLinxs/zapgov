package templates

import "github.com/gofiber/fiber/v2"

// Handler expõe a sincronização de templates da WABA.
type Handler struct {
	client *Client
	repo   *Repository
}

func NewHandler(client *Client, repo *Repository) *Handler {
	return &Handler{client: client, repo: repo}
}

// Register: listar é filtrado por setor; criar é admin/supervisor; atribuir
// departamentos é SÓ admin (decisão de política).
func (h *Handler) Register(r fiber.Router, gestor, admin fiber.Handler) {
	r.Get("/templates", h.list)
	r.Post("/templates", gestor, h.create)
	r.Put("/templates/:name/setores", admin, h.setSetores)
}

// setSetores define quais departamentos podem usar um template.
func (h *Handler) setSetores(c *fiber.Ctx) error {
	name := c.Params("name")
	var body struct {
		Setores []string `json:"setores"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetSetores(c.Context(), name, body.Setores); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
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

	// Anota cada template com os setores liberados.
	mapa, _ := h.repo.AllSetores(c.Context())
	for i := range tpls {
		tpls[i].Setores = mapa[tpls[i].Name]
		if tpls[i].Setores == nil {
			tpls[i].Setores = []string{}
		}
	}

	papel, _ := c.Locals("papel").(string)
	setor, _ := c.Locals("setor").(string)
	return c.JSON(fiber.Map{"templates": VisibleFor(tpls, papel, setor)})
}

// VisibleFor aplica a regra de visibilidade de templates: atendente só vê os
// liberados pro setor dele (sem atribuição = invisível; sem setor = nenhum);
// admin/supervisor veem todos. Função pura — testável sem HTTP.
func VisibleFor(tpls []Template, papel, setor string) []Template {
	if papel != "atendente" {
		return tpls
	}
	out := make([]Template, 0, len(tpls))
	if setor == "" {
		return out
	}
	for _, t := range tpls {
		if contains(t.Setores, setor) {
			out = append(out, t)
		}
	}
	return out
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
