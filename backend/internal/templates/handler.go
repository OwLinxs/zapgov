package templates

import (
	"regexp"

	"github.com/gofiber/fiber/v2"
)

// templateIDRe valida o id do template da Meta (numérico) antes de compor a URL.
var templateIDRe = regexp.MustCompile(`^\d+$`)

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
	r.Post("/templates/edit", gestor, h.edit) // editar componentes (volta p/ aprovação)
	r.Put("/templates/:name/setores", admin, h.setSetores)
	// Grupos de templates (organização por secretaria): admin gerencia.
	r.Get("/template-groups", h.listGroups)
	r.Post("/template-groups", admin, h.createGroup)
	r.Delete("/template-groups/:id", admin, h.deleteGroup)
	r.Put("/templates/:name/grupo", admin, h.setGroup)
}

func (h *Handler) listGroups(c *fiber.Ctx) error {
	gs, err := h.repo.ListGroups(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"groups": gs})
}

func (h *Handler) createGroup(c *fiber.Ctx) error {
	var in struct {
		Nome string `json:"nome"`
	}
	if err := c.BodyParser(&in); err != nil || in.Nome == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome é obrigatório"})
	}
	g, err := h.repo.CreateGroup(c.Context(), in.Nome)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(g)
}

func (h *Handler) deleteGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.repo.DeleteGroup(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) setGroup(c *fiber.Ctx) error {
	var in struct {
		GroupID int64 `json:"group_id"` // 0 remove do grupo
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.repo.SetGroup(c.Context(), c.Params("name"), in.GroupID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
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

func (h *Handler) edit(c *fiber.Ctx) error {
	var req struct {
		ID       string   `json:"id"`
		Corpo    string   `json:"corpo"`
		Header   string   `json:"header"`
		Footer   string   `json:"footer"`
		Botoes   []string `json:"botoes"`
		Exemplos []string `json:"exemplos"`
	}
	if err := c.BodyParser(&req); err != nil || req.ID == "" || req.Corpo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id e corpo são obrigatórios"})
	}
	// Anti-IDOR/confused-deputy: o id vai direto na URL da Graph API. Exige que
	// seja numérico E que pertença a um template DESTE tenant (List é tenant-scoped
	// pelas credenciais do contexto) — senão daria pra editar objeto de outra WABA
	// alcançável pelo token de fallback.
	if !templateIDRe.MatchString(req.ID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id de template inválido"})
	}
	tpls, err := h.client.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	achou := false
	for _, t := range tpls {
		if t.ID == req.ID {
			achou = true
			break
		}
	}
	if !achou {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "template não pertence a este órgão"})
	}
	if err := h.client.Edit(c.Context(), EditRequest{
		ID: req.ID, Corpo: req.Corpo, Header: req.Header, Footer: req.Footer, Botoes: req.Botoes, Exemplos: req.Exemplos,
	}); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) list(c *fiber.Ctx) error {
	tpls, err := h.client.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	// Anota cada template com os setores liberados e o grupo.
	mapa, _ := h.repo.AllSetores(c.Context())
	grupos, _ := h.repo.AllGroups(c.Context())
	for i := range tpls {
		tpls[i].Setores = mapa[tpls[i].Name]
		if tpls[i].Setores == nil {
			tpls[i].Setores = []string{}
		}
		tpls[i].Grupo = grupos[tpls[i].Name]
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
