// Package labels gerencia o catálogo de etiquetas (cor + nome) do tenant.
package labels

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Label struct {
	ID   int64  `json:"id"`
	Nome string `json:"nome"`
	Cor  string `json:"cor"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) List(ctx context.Context) ([]Label, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nome, cor FROM labels WHERE tenant_id = $1 ORDER BY nome`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Label, error) {
		var l Label
		err := row.Scan(&l.ID, &l.Nome, &l.Cor)
		return l, err
	})
}

func (r *Repository) Create(ctx context.Context, nome, cor string) (*Label, error) {
	var l Label
	err := r.pool.QueryRow(ctx, `
		INSERT INTO labels (tenant_id, nome, cor) VALUES ($3, $1, COALESCE(NULLIF($2,''),'oklch(0.62 0.155 256)'))
		RETURNING id, nome, cor`, nome, cor, tenantctx.From(ctx)).Scan(&l.ID, &l.Nome, &l.Cor)
	return &l, err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM labels WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// EnsureByName devolve o id de uma etiqueta pelo nome, criando se não existir.
// Usado na auto-etiquetagem por atendente.
func (r *Repository) EnsureByName(ctx context.Context, nome, cor string) (int64, error) {
	tid := tenantctx.From(ctx)
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM labels WHERE tenant_id = $1 AND nome = $2`, tid, nome).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO labels (tenant_id, nome, cor) VALUES ($1, $2, COALESCE(NULLIF($3,''),'oklch(0.62 0.155 256)'))
		RETURNING id`, tid, nome, cor).Scan(&id)
	return id, err
}

// ---- HTTP ----

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: listar é aberto (qualquer atendente usa); criar/remover é admin/supervisor.
func (h *Handler) Register(r fiber.Router, gestor fiber.Handler) {
	r.Get("/labels", h.list)
	r.Post("/labels", gestor, h.create)
	r.Delete("/labels/:id", gestor, h.del)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"labels": items})
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in struct {
		Nome string `json:"nome"`
		Cor  string `json:"cor"`
	}
	if err := c.BodyParser(&in); err != nil || in.Nome == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome é obrigatório"})
	}
	l, err := h.repo.Create(c.Context(), in.Nome, in.Cor)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(l)
}

func (h *Handler) del(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.repo.Delete(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
