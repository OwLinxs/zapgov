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

// Group é um grupo de templates (ex.: "Secretaria de Saúde").
type Group struct {
	ID   int64  `json:"id"`
	Nome string `json:"nome"`
}

// ListGroups devolve os grupos do tenant.
func (r *Repository) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, nome FROM template_groups WHERE tenant_id = $1 ORDER BY nome`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Nome); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// CreateGroup cria um grupo.
func (r *Repository) CreateGroup(ctx context.Context, nome string) (*Group, error) {
	var g Group
	err := r.pool.QueryRow(ctx,
		`INSERT INTO template_groups (tenant_id, nome) VALUES ($1, $2) RETURNING id, nome`,
		tenantctx.From(ctx), nome).Scan(&g.ID, &g.Nome)
	return &g, err
}

// DeleteGroup remove o grupo (os templates ficam "sem grupo" via CASCADE dos itens).
func (r *Repository) DeleteGroup(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM template_groups WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// SetGroup define o grupo de um template (groupID 0 = remove do grupo).
func (r *Repository) SetGroup(ctx context.Context, template string, groupID int64) error {
	tid := tenantctx.From(ctx)
	if groupID == 0 {
		_, err := r.pool.Exec(ctx,
			`DELETE FROM template_group_items WHERE tenant_id = $1 AND template = $2`, tid, template)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO template_group_items (tenant_id, template, group_id) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, template) DO UPDATE SET group_id = EXCLUDED.group_id`,
		tid, template, groupID)
	return err
}

// AllGroups devolve o mapa template -> nome do grupo.
func (r *Repository) AllGroups(ctx context.Context) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.template, g.nome FROM template_group_items i
		JOIN template_groups g ON g.id = i.group_id
		WHERE i.tenant_id = $1`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var t, g string
		if err := rows.Scan(&t, &g); err != nil {
			return nil, err
		}
		out[t] = g
	}
	return out, rows.Err()
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
