// Package audit registra uma trilha de auditoria de todas as ações que alteram
// dados (mutações), para compliance/LGPD. Captura via middleware, sem tocar handlers.
package audit

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Entry struct {
	ID       int64     `json:"id"`
	TenantID int64     `json:"-"`
	UserID   *int64    `json:"user_id"`
	UserNome string    `json:"user_nome"`
	Papel    string    `json:"papel"`
	Metodo   string    `json:"metodo"`
	Rota     string    `json:"rota"`
	Status   int       `json:"status"`
	CriadoEm time.Time `json:"criado_em"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Write(ctx context.Context, e Entry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, user_id, user_nome, papel, metodo, rota, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.TenantID, e.UserID, e.UserNome, e.Papel, e.Metodo, e.Rota, e.Status)
	return err
}

func (r *Repository) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, user_nome, papel, metodo, rota, status, criado_em
		FROM audit_log WHERE tenant_id = $1 ORDER BY criado_em DESC LIMIT $2`,
		tenantctx.From(ctx), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Entry, error) {
		var e Entry
		err := row.Scan(&e.ID, &e.UserID, &e.UserNome, &e.Papel, &e.Metodo, &e.Rota, &e.Status, &e.CriadoEm)
		return e, err
	})
}

// Middleware registra cada requisição que ALTERA dados (não-GET) com sucesso.
func Middleware(repo *Repository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		m := c.Method()
		if m == fiber.MethodGet || m == fiber.MethodOptions || m == fiber.MethodHead {
			return err
		}
		status := c.Response().StatusCode()
		if status >= 400 {
			return err // não audita tentativa que falhou
		}

		uid, _ := c.Locals("uid").(int64)
		nome, _ := c.Locals("nome").(string)
		papel, _ := c.Locals("papel").(string)
		tid, _ := c.Locals(tenantctx.Key).(int64)
		var uidPtr *int64
		if uid != 0 {
			uidPtr = &uid
		}
		// Escrita best-effort em background (não atrasa a resposta).
		entry := Entry{TenantID: tid, UserID: uidPtr, UserNome: nome, Papel: papel, Metodo: m, Rota: c.Path(), Status: status}
		go func() {
			_ = repo.Write(context.Background(), entry)
		}()
		return err
	}
}

// Handler expõe a consulta da trilha (somente leitura).
type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register: a consulta é restrita (admin/supervisor) via o middleware passado.
func (h *Handler) Register(r fiber.Router, gestor fiber.Handler) {
	r.Get("/audit", gestor, h.list)
}

func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.repo.List(c.Context(), c.QueryInt("limit", 200))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"audit": items})
}
