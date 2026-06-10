// Package quickreplies gerencia respostas rápidas (canned) do tenant.
package quickreplies

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type QuickReply struct {
	ID     int64  `json:"id"`
	Atalho string `json:"atalho"`
	Texto  string `json:"texto"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) List(ctx context.Context) ([]QuickReply, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, atalho, texto FROM quick_replies WHERE tenant_id = $1 ORDER BY atalho`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (QuickReply, error) {
		var q QuickReply
		err := row.Scan(&q.ID, &q.Atalho, &q.Texto)
		return q, err
	})
}

func (r *Repository) Create(ctx context.Context, atalho, texto string) (*QuickReply, error) {
	var q QuickReply
	err := r.pool.QueryRow(ctx,
		`INSERT INTO quick_replies (tenant_id, atalho, texto) VALUES ($1, $2, $3) RETURNING id, atalho, texto`,
		tenantctx.From(ctx), atalho, texto).Scan(&q.ID, &q.Atalho, &q.Texto)
	return &q, err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM quick_replies WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// ---- HTTP ----

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: listar é aberto (atendente usa); criar/remover é admin/supervisor.
func (h *Handler) Register(r fiber.Router, gestor fiber.Handler) {
	r.Get("/quick-replies", h.list)
	r.Post("/quick-replies", gestor, h.create)
	r.Delete("/quick-replies/:id", gestor, h.del)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"quick_replies": items})
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in struct {
		Atalho string `json:"atalho"`
		Texto  string `json:"texto"`
	}
	if err := c.BodyParser(&in); err != nil || in.Texto == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "texto é obrigatório"})
	}
	q, err := h.repo.Create(c.Context(), in.Atalho, in.Texto)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(q)
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
