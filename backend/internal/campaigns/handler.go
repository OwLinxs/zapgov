package campaigns

import (
	"bytes"
	"encoding/csv"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register: ver histórico é aberto; disparar é admin/supervisor.
func (h *Handler) Register(r fiber.Router, ops fiber.Handler) {
	r.Get("/campaigns", h.list)
	r.Get("/campaigns/export.csv", h.exportCSV)
	r.Post("/campaigns", ops, h.create)
	r.Get("/campaigns/:id", h.get)
}

// exportCSV devolve o histórico de disparos (com os mesmos filtros do list)
// como arquivo CSV para download.
func (h *Handler) exportCSV(c *fiber.Ctx) error {
	setor := c.Query("setor")
	if setor == "Todos" {
		setor = ""
	}
	status := c.Query("status")
	if status == "Todos" {
		status = ""
	}
	items, err := h.svc.List(c.Context(), setor, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // BOM p/ Excel abrir UTF-8 com acentos
	w := csv.NewWriter(&buf)
	w.Write([]string{"ID", "Template", "Setor", "Operador", "Total", "Enviadas", "Entregues", "Lidas", "Falhas", "Status", "Criado em"})
	for _, ca := range items {
		w.Write([]string{
			strconv.FormatInt(ca.ID, 10),
			ca.Template, ca.Setor, ca.Operador,
			strconv.Itoa(ca.Total), strconv.Itoa(ca.Enviadas), strconv.Itoa(ca.Entregues),
			strconv.Itoa(ca.Lidas), strconv.Itoa(ca.Falhas), ca.Status,
			ca.CriadoEm.Format("2006-01-02 15:04"),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="disparos.csv"`)
	return c.Send(buf.Bytes())
}

func (h *Handler) list(c *fiber.Ctx) error {
	setor := c.Query("setor")
	if setor == "Todos" {
		setor = ""
	}
	status := c.Query("status")
	if status == "Todos" {
		status = ""
	}
	items, err := h.svc.List(c.Context(), setor, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"campaigns": items})
}

func (h *Handler) get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	camp, err := h.svc.Get(c.Context(), int64(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "campanha não encontrada"})
	}
	return c.JSON(camp)
}

type createReq struct {
	Template string   `json:"template"`
	Language string   `json:"language"`
	Params   []string `json:"params"`
	Setor    string   `json:"setor"`
	Operador string   `json:"operador"`
	Preview  string   `json:"preview"`
	Phones   []string `json:"phones"`
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	camp, err := h.svc.Create(c.Context(), CreateInput{
		Template: req.Template,
		Language: req.Language,
		Params:   req.Params,
		Setor:    req.Setor,
		Operador: req.Operador,
		Preview:  req.Preview,
		Phones:   req.Phones,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(camp)
}
