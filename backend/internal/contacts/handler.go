package contacts

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r fiber.Router) {
	r.Get("/contacts", h.list)
	r.Post("/contacts", h.create)
	r.Post("/contacts/import", h.importCSV)
	r.Get("/contacts/:id", h.get)
}

func (h *Handler) list(c *fiber.Ctx) error {
	setor := c.Query("setor")
	if setor == "Todos" {
		setor = ""
	}
	items, err := h.repo.List(c.Context(), setor, c.Query("q"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"contacts": items})
}

func (h *Handler) get(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	ct, err := h.repo.Get(c.Context(), int64(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "contato não encontrado"})
	}
	return c.JSON(ct)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in Contact
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	in.Telefone = digits(in.Telefone)
	if in.Nome == "" || in.Telefone == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome e telefone são obrigatórios"})
	}
	if in.Setor == "" {
		in.Setor = "Geral"
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	ct, err := h.repo.Create(c.Context(), in)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ct)
}

// importCSV recebe um arquivo CSV (campo "file" multipart) ou o corpo bruto e
// cadastra os contatos em massa. Aceita cabeçalho (nome,telefone,cpf,setor,tags)
// ou a ordem fixa: nome, telefone, cpf, setor, tags. Tags separadas por ; ou |.
func (h *Handler) importCSV(c *fiber.Ctx) error {
	raw, err := csvBytes(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	records, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV inválido: " + err.Error()})
	}
	if len(records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV vazio"})
	}

	// Mapeia colunas. Se a 1ª linha tem cabeçalho conhecido, usa-o.
	idx := map[string]int{"nome": 0, "telefone": 1, "cpf": 2, "setor": 3, "tags": 4}
	start := 0
	if hasHeader(records[0]) {
		idx = map[string]int{}
		for i, h := range records[0] {
			idx[strings.ToLower(strings.TrimSpace(h))] = i
		}
		start = 1
	}
	col := func(rec []string, name string) string {
		if i, ok := idx[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	var items []ImportRow
	var invalidos int
	for _, rec := range records[start:] {
		nome := col(rec, "nome")
		tel := digits(col(rec, "telefone"))
		if nome == "" || tel == "" {
			invalidos++
			continue
		}
		var tags []string
		if t := col(rec, "tags"); t != "" {
			for _, p := range strings.FieldsFunc(t, func(r rune) bool { return r == ';' || r == '|' }) {
				if p = strings.TrimSpace(p); p != "" {
					tags = append(tags, p)
				}
			}
		}
		items = append(items, ImportRow{Nome: nome, Telefone: tel, CPF: col(rec, "cpf"), Setor: col(rec, "setor"), Tags: tags})
	}

	inseridos, pulados, err := h.repo.ImportBulk(c.Context(), items)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"inseridos": inseridos, "pulados": pulados, "invalidos": invalidos})
}

// csvBytes extrai o conteúdo do CSV: arquivo multipart "file" ou corpo bruto.
func csvBytes(c *fiber.Ctx) ([]byte, error) {
	if fh, err := c.FormFile("file"); err == nil {
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	if b := c.Body(); len(b) > 0 {
		return b, nil
	}
	return nil, fiber.NewError(fiber.StatusBadRequest, "envie um arquivo CSV no campo 'file'")
}

// hasHeader detecta se a primeira linha é cabeçalho (contém 'nome' ou 'telefone').
func hasHeader(rec []string) bool {
	for _, h := range rec {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "nome", "telefone", "cpf", "setor", "tags":
			return true
		}
	}
	return false
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
