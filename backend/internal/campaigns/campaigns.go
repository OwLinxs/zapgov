// Package campaigns registra e lista disparos (histórico).
package campaigns

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Campaign struct {
	ID          int64      `json:"id"`
	Template    string     `json:"template"`
	Language    string     `json:"language"`
	Setor       string     `json:"setor"`
	Total       int        `json:"total"`
	Enviadas    int        `json:"enviadas"`
	Entregues   int        `json:"entregues"`
	Lidas       int        `json:"lidas"`
	Falhas      int        `json:"falhas"`
	Status      string     `json:"status"`
	Operador    string     `json:"operador"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	CriadoEm    time.Time  `json:"criado_em"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const cols = `id, template, language, setor, total, enviadas, entregues, lidas, falhas, status, operador, scheduled_at, criado_em`

func scan(row pgx.CollectableRow) (Campaign, error) {
	var c Campaign
	err := row.Scan(&c.ID, &c.Template, &c.Language, &c.Setor, &c.Total,
		&c.Enviadas, &c.Entregues, &c.Lidas, &c.Falhas, &c.Status, &c.Operador,
		&c.ScheduledAt, &c.CriadoEm)
	return c, err
}

func (r *Repository) Create(ctx context.Context, c Campaign) (*Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		INSERT INTO campaigns (tenant_id, template, language, setor, total, status, operador, scheduled_at)
		VALUES ($8,$1,$2,$3,$4,$5,$6,$7)
		RETURNING `+cols,
		c.Template, c.Language, c.Setor, c.Total, c.Status, c.Operador, c.ScheduledAt, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectOneRow(rows, scan)
	return &out, err
}

func (r *Repository) UpdateStats(ctx context.Context, id int64, enviadas, entregues, falhas int, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET enviadas=$2, entregues=$3, falhas=$4, status=$5 WHERE id=$1`,
		id, enviadas, entregues, falhas, status,
	)
	return err
}

// IncEnviadas/IncFalhas: contadores atômicos atualizados pelo worker da fila.
func (r *Repository) IncEnviadas(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE campaigns SET enviadas = enviadas + 1 WHERE id = $1`, id)
	return err
}

func (r *Repository) IncFalhas(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE campaigns SET falhas = falhas + 1 WHERE id = $1`, id)
	return err
}

func (r *Repository) List(ctx context.Context, setor, status string) ([]Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+` FROM campaigns
		WHERE tenant_id = $3 AND ($1 = '' OR setor = $1) AND ($2 = '' OR status = $2)
		ORDER BY criado_em DESC`,
		setor, status, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM campaigns WHERE id=$1 AND tenant_id=$2`, id, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
