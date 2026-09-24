// Package reminders agenda e envia lembretes automáticos (véspera de
// consulta/cirurgia) detectados pela variável de data nos templates enviados.
package reminders

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Reminder struct {
	ID         int64     `json:"id"`
	TenantID   int64     `json:"-"`
	Phone      string    `json:"phone"`
	Nome       string    `json:"nome"`
	DataEvento time.Time `json:"data_evento"`
	EnviarEm   time.Time `json:"enviar_em"`
	Status     string    `json:"status"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Create agenda um lembrete (ignora duplicado pendente do mesmo paciente/data).
func (r *Repository) Create(ctx context.Context, phone, nome string, dataEvento time.Time, enviarEm time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reminders (tenant_id, phone, nome, data_evento, enviar_em)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		tenantctx.From(ctx), phone, nome, dataEvento, enviarEm)
	return err
}

// Due devolve os lembretes pendentes que já venceram (todos os tenants —
// o agendador injeta o tenant de cada um no envio).
func (r *Repository) Due(ctx context.Context, limit int) ([]Reminder, error) {
	_, err := r.pool.Exec(ctx, `UPDATE reminders SET status='revisao',updated_at=now() WHERE status='enviando' AND updated_at<now()-interval '5 minutes'`)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH due AS (SELECT id FROM reminders WHERE status='pendente' AND enviar_em<=now()
 ORDER BY enviar_em FOR UPDATE SKIP LOCKED LIMIT $1)
 UPDATE reminders r SET status='enviando',updated_at=now() FROM due WHERE r.id=due.id
 RETURNING r.id,r.tenant_id,r.phone,r.nome,r.data_evento,r.enviar_em,r.status`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

func scan(row pgx.CollectableRow) (Reminder, error) {
	var x Reminder
	err := row.Scan(&x.ID, &x.TenantID, &x.Phone, &x.Nome, &x.DataEvento, &x.EnviarEm, &x.Status)
	return x, err
}

// MarkStatus atualiza o resultado do envio.
func (r *Repository) MarkStatus(ctx context.Context, id int64, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE reminders SET status = $2,updated_at=now() WHERE id = $1 AND tenant_id=$3`, id, status, tenantctx.From(ctx))
	return err
}

// ListPending lista os lembretes futuros do tenant (para gestão na config).
func (r *Repository) ListPending(ctx context.Context) ([]Reminder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, phone, nome, data_evento, enviar_em, status
		FROM reminders
		WHERE tenant_id = $1 AND status IN ('pendente','enviando','revisao','falhou')
		ORDER BY enviar_em LIMIT 200`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

// Cancel cancela um lembrete pendente.
func (r *Repository) Cancel(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reminders SET status = 'cancelado' WHERE id = $1 AND tenant_id = $2 AND status = 'pendente'`,
		id, tenantctx.From(ctx))
	return err
}

// ---- HTTP ----

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

func (h *Handler) Register(r fiber.Router) {
	r.Get("/reminders", h.list)
	r.Delete("/reminders/:id", h.cancel)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.ListPending(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if items == nil {
		items = []Reminder{}
	}
	return c.JSON(fiber.Map{"reminders": items})
}

func (h *Handler) cancel(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	if err := h.repo.Cancel(c.Context(), int64(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
