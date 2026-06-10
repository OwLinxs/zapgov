package templates

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

// Repository guarda a liberação de templates por departamento (setor).
type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// AllSetores devolve, para o tenant atual, o mapa template -> setores liberados.
func (r *Repository) AllSetores(ctx context.Context) (map[string][]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT template, setor FROM template_setores WHERE tenant_id = $1 ORDER BY template, setor`,
		tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var t, s string
		if err := rows.Scan(&t, &s); err != nil {
			return nil, err
		}
		out[t] = append(out[t], s)
	}
	return out, rows.Err()
}

// SetSetores substitui os setores liberados de um template.
func (r *Repository) SetSetores(ctx context.Context, template string, setores []string) error {
	tid := tenantctx.From(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM template_setores WHERE tenant_id = $1 AND template = $2`, tid, template); err != nil {
		return err
	}
	for _, s := range setores {
		if s == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO template_setores (tenant_id, template, setor) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			tid, template, s); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Allowed diz se um template está liberado para um setor.
func (r *Repository) Allowed(ctx context.Context, template, setor string) (bool, error) {
	if setor == "" {
		return false, nil
	}
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM template_setores WHERE tenant_id = $1 AND template = $2 AND setor = $3)`,
		tenantctx.From(ctx), template, setor).Scan(&ok)
	return ok, err
}
