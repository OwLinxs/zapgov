// Package chatbot faz o autoatendimento: menu de opções no inbound.
package chatbot

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Config struct {
	Enabled  bool   `json:"enabled"`
	Welcome  string `json:"welcome"`
	Fallback string `json:"fallback"`
}

type Option struct {
	ID           int64  `json:"id"`
	Ord          int    `json:"ord"`
	Chave        string `json:"chave"`
	Label        string `json:"label"`
	Resposta     string `json:"resposta"`
	DepartmentID *int64 `json:"department_id"`
	DeptNome     string `json:"department_nome"`
	Ativo        bool   `json:"ativo"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ensureConfig garante a linha de bot_config do tenant (criada sob demanda).
func (r *Repository) ensureConfig(ctx context.Context, tid int64) {
	_, _ = r.pool.Exec(ctx,
		`INSERT INTO bot_config (id, tenant_id) VALUES ($1, $1) ON CONFLICT DO NOTHING`, tid)
}

func (r *Repository) GetConfig(ctx context.Context) (Config, error) {
	tid := tenantctx.From(ctx)
	r.ensureConfig(ctx, tid)
	var c Config
	err := r.pool.QueryRow(ctx, `SELECT enabled, welcome, fallback FROM bot_config WHERE tenant_id = $1`, tid).
		Scan(&c.Enabled, &c.Welcome, &c.Fallback)
	return c, err
}

func (r *Repository) UpdateConfig(ctx context.Context, c Config) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE bot_config SET enabled = $1, welcome = $2, fallback = $3 WHERE tenant_id = $4`,
		c.Enabled, c.Welcome, c.Fallback, tenantctx.From(ctx))
	return err
}

func (r *Repository) ListOptions(ctx context.Context, onlyActive bool) ([]Option, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.ord, o.chave, o.label, o.resposta, o.department_id,
		       COALESCE(d.nome, ''), o.ativo
		FROM bot_options o
		LEFT JOIN departments d ON d.id = o.department_id
		WHERE o.tenant_id = $2 AND ($1 = false OR o.ativo = true)
		ORDER BY o.ord ASC, o.id ASC`, onlyActive, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Option, error) {
		var o Option
		err := row.Scan(&o.ID, &o.Ord, &o.Chave, &o.Label, &o.Resposta, &o.DepartmentID, &o.DeptNome, &o.Ativo)
		return o, err
	})
}

func (r *Repository) CreateOption(ctx context.Context, o Option) (*Option, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO bot_options (tenant_id, ord, chave, label, resposta, department_id)
		VALUES ($6, $1, $2, $3, $4, $5)
		RETURNING id`,
		o.Ord, o.Chave, o.Label, o.Resposta, o.DepartmentID, tenantctx.From(ctx),
	).Scan(&o.ID)
	if err != nil {
		return nil, err
	}
	o.Ativo = true
	return &o, nil
}

func (r *Repository) DeleteOption(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM bot_options WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}
