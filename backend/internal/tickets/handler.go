package tickets

import (
	"github.com/gofiber/fiber/v2"
)

// Handler expõe as rotas HTTP dos chamados. Só traduz HTTP ↔ Service.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register monta as rotas. Todas exigem login (api já é autenticado); abrir e
// operar chamado é de qualquer usuário interno — o front limita o menu ao T.I.
func (h *Handler) Register(r fiber.Router) {
	r.Get("/tickets", h.list)
	r.Get("/tickets/metrics", h.metrics)
	r.Post("/tickets", h.create)
	r.Get("/tickets/:id", h.get)
	r.Put("/tickets/:id/status", h.setStatus)
	r.Put("/tickets/:id/assign", h.assign)
	r.Post("/tickets/:id/comment", h.comment)
}

// operador devolve o nome do usuário logado (autor de comentário/atribuição).
func operador(c *fiber.Ctx) string {
	if n, _ := c.Locals("nome").(string); n != "" {
		return n
	}
	return "T.I."
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.svc.List(c.Context(), c.Query("status"), c.Query("prioridade"), c.Query("categoria"), c.Query("tecnico"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"tickets": items})
}

func (h *Handler) metrics(c *fiber.Ctx) error {
	m, err := h.svc.Metrics(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(m)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in struct {
		Titulo          string `json:"titulo"`
		Descricao       string `json:"descricao"`
		Categoria       string `json:"categoria"`
		Prioridade      string `json:"prioridade"`
		SolicitanteNome string `json:"solicitante_nome"`
		SolicitanteFone string `json:"solicitante_fone"`
		Secretaria      string `json:"secretaria"`
		Local           string `json:"local"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	t, err := h.svc.Create(c.Context(), CreateInput{
		Titulo: in.Titulo, Descricao: in.Descricao, Categoria: in.Categoria,
		Prioridade: in.Prioridade, SolicitanteNome: in.SolicitanteNome,
		SolicitanteFone: in.SolicitanteFone, Secretaria: in.Secretaria, Local: in.Local,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(t)
}

func (h *Handler) get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	t, err := h.svc.Get(c.Context(), int64(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "chamado não encontrado"})
	}
	comments, _ := h.svc.Comments(c.Context(), int64(id))
	return c.JSON(fiber.Map{"ticket": t, "comments": comments})
}

func (h *Handler) setStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&in); err != nil || in.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "status é obrigatório"})
	}
	t, err := h.svc.SetStatus(c.Context(), int64(id), in.Status)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(t)
}

func (h *Handler) assign(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var in struct {
		Tecnico string `json:"tecnico"`
	}
	_ = c.BodyParser(&in)
	// Sem técnico no corpo: assume o próprio usuário logado.
	if in.Tecnico == "" {
		in.Tecnico = operador(c)
	}
	t, err := h.svc.Assign(c.Context(), int64(id), in.Tecnico)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(t)
}

func (h *Handler) comment(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var in struct {
		Body    string `json:"body"`
		Interno bool   `json:"interno"`
	}
	if err := c.BodyParser(&in); err != nil || in.Body == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "texto é obrigatório"})
	}
	cm, err := h.svc.AddComment(c.Context(), int64(id), operador(c), in.Body, in.Interno)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(cm)
}
