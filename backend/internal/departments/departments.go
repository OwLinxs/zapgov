// Package departments faz o CRUD de departamentos (setores) do tenant.
package departments

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Department struct {
	ID       int64     `json:"id"`
	Nome     string    `json:"nome"`
	Cor      string    `json:"cor"`
	Ativo    bool      `json:"ativo"`
	CriadoEm time.Time `json:"criado_em"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func scan(row pgx.CollectableRow) (Department, error) {
	var d Department
	err := row.Scan(&d.ID, &d.Nome, &d.Cor, &d.Ativo, &d.CriadoEm)
	return d, err
}

func (r *Repository) List(ctx context.Context) ([]Department, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nome, cor, ativo, criado_em FROM departments WHERE tenant_id = $1 ORDER BY nome ASC`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Create(ctx context.Context, nome, cor string) (*Department, error) {
	rows, err := r.pool.Query(ctx, `
		INSERT INTO departments (tenant_id, nome, cor) VALUES ($3, $1, COALESCE(NULLIF($2,''),'oklch(0.6 0.02 264)'))
		RETURNING id, nome, cor, ativo, criado_em`, nome, cor, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	d, err := pgx.CollectOneRow(rows, scan)
	return &d, err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM departments WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// ---- HTTP ----

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: listar é aberto (atendente usa no filtro do inbox); criar/remover é admin.
func (h *Handler) Register(r fiber.Router, admin fiber.Handler) {
	r.Get("/departments", h.list)
	r.Post("/departments", admin, h.create)
	r.Delete("/departments/:id", admin, h.del)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"departments": items})
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in struct {
		Nome string `json:"nome"`
		Cor  string `json:"cor"`
	}
	if err := c.BodyParser(&in); err != nil || in.Nome == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome é obrigatório"})
	}
	d, err := h.repo.Create(c.Context(), in.Nome, in.Cor)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(d)
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
