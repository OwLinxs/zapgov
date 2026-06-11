// Package quickreplies gerencia respostas rápidas (canned): globais do órgão
// (gestão cria) e pessoais de cada atendente.
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
	Minha  bool   `json:"minha"` // pessoal do usuário logado (editável por ele)
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// List devolve as globais + as pessoais do usuário.
func (r *Repository) List(ctx context.Context, uid int64) ([]QuickReply, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, atalho, texto, (user_id IS NOT NULL) AS minha
		FROM quick_replies
		WHERE tenant_id = $1 AND (user_id IS NULL OR user_id = $2)
		ORDER BY minha DESC, atalho`,
		tenantctx.From(ctx), uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (QuickReply, error) {
		var q QuickReply
		err := row.Scan(&q.ID, &q.Atalho, &q.Texto, &q.Minha)
		return q, err
	})
}

// Create insere: pessoal (uid != nil) ou global (uid == nil).
func (r *Repository) Create(ctx context.Context, uid *int64, atalho, texto string) (*QuickReply, error) {
	var q QuickReply
	err := r.pool.QueryRow(ctx, `
		INSERT INTO quick_replies (tenant_id, user_id, atalho, texto)
		VALUES ($1, $2, $3, $4) RETURNING id, atalho, texto, (user_id IS NOT NULL)`,
		tenantctx.From(ctx), uid, atalho, texto).Scan(&q.ID, &q.Atalho, &q.Texto, &q.Minha)
	return &q, err
}

// Delete remove. Atendente só apaga as próprias; gestão apaga qualquer uma do tenant.
func (r *Repository) Delete(ctx context.Context, id, uid int64, gestor bool) error {
	if gestor {
		_, err := r.pool.Exec(ctx, `DELETE FROM quick_replies WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
		return err
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM quick_replies WHERE id = $1 AND tenant_id = $2 AND user_id = $3`,
		id, tenantctx.From(ctx), uid)
	return err
}

// ---- HTTP ----

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: todos os papéis listam e criam (atendente cria pessoal; gestão cria
// global). Apagar respeita o dono.
func (h *Handler) Register(r fiber.Router, _ fiber.Handler) {
	r.Get("/quick-replies", h.list)
	r.Post("/quick-replies", h.create)
	r.Delete("/quick-replies/:id", h.del)
}

func uidPapel(c *fiber.Ctx) (int64, string) {
	uid, _ := c.Locals("uid").(int64)
	papel, _ := c.Locals("papel").(string)
	return uid, papel
}

func (h *Handler) list(c *fiber.Ctx) error {
	uid, _ := uidPapel(c)
	items, err := h.repo.List(c.Context(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"quick_replies": items})
}

func (h *Handler) create(c *fiber.Ctx) error {
	var in struct {
		Atalho  string `json:"atalho"`
		Texto   string `json:"texto"`
		Pessoal bool   `json:"pessoal"` // gestão pode optar por criar pessoal também
	}
	if err := c.BodyParser(&in); err != nil || in.Texto == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "texto é obrigatório"})
	}
	uid, papel := uidPapel(c)
	// Atendente SEMPRE cria pessoal; gestão cria global (ou pessoal se pedir).
	var owner *int64
	if papel == "atendente" || in.Pessoal {
		owner = &uid
	}
	q, err := h.repo.Create(c.Context(), owner, in.Atalho, in.Texto)
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
	uid, papel := uidPapel(c)
	gestor := papel == "admin" || papel == "supervisor"
	if err := h.repo.Delete(c.Context(), int64(id), uid, gestor); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
