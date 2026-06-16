// Package kb implementa a base de conhecimento (artigos problema/solução que os
// técnicos registram a partir dos atendimentos). Busca full-text em português.
package kb

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Article struct {
	ID           int64     `json:"id"`
	Titulo       string    `json:"titulo"`
	Problema     string    `json:"problema"`
	Solucao      string    `json:"solucao"`
	Categoria    string    `json:"categoria"`
	Tags         []string  `json:"tags"`
	Autor        string    `json:"autor"`
	TicketID     *int64    `json:"ticket_id"`
	Status       string    `json:"status"`
	Views        int       `json:"views"`
	Uteis        int       `json:"uteis"`
	CriadoEm     time.Time `json:"criado_em"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const cols = `id, titulo, problema, solucao, categoria, tags, autor, ticket_id, status, views, uteis, criado_em, atualizado_em`

func scan(row pgx.CollectableRow) (Article, error) {
	var a Article
	err := row.Scan(&a.ID, &a.Titulo, &a.Problema, &a.Solucao, &a.Categoria, &a.Tags,
		&a.Autor, &a.TicketID, &a.Status, &a.Views, &a.Uteis, &a.CriadoEm, &a.AtualizadoEm)
	return a, err
}

// List busca artigos publicados. Com q usa full-text (ranqueado por relevância);
// sem q, lista por mais recentes. Filtro opcional por categoria.
func (r *Repository) List(ctx context.Context, q, categoria string) ([]Article, error) {
	var rows pgx.Rows
	var err error
	if q != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT `+cols+` FROM kb_articles
			WHERE tenant_id = $1 AND status = 'publicado'
			  AND ($3 = '' OR categoria = $3)
			  AND busca @@ plainto_tsquery('portuguese', $2)
			ORDER BY ts_rank(busca, plainto_tsquery('portuguese', $2)) DESC
			LIMIT 100`,
			tenantctx.From(ctx), q, categoria)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT `+cols+` FROM kb_articles
			WHERE tenant_id = $1 AND status = 'publicado'
			  AND ($2 = '' OR categoria = $2)
			ORDER BY atualizado_em DESC LIMIT 100`,
			tenantctx.From(ctx), categoria)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, scan)
	if out == nil {
		out = []Article{}
	}
	return out, err
}

// Get devolve um artigo e incrementa o contador de visualizações.
func (r *Repository) Get(ctx context.Context, id int64) (*Article, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE kb_articles SET views = views + 1
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+cols, id, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	a, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) Create(ctx context.Context, a Article) (*Article, error) {
	if a.Tags == nil {
		a.Tags = []string{}
	}
	rows, err := r.pool.Query(ctx, `
		INSERT INTO kb_articles (tenant_id, titulo, problema, solucao, categoria, tags, autor, ticket_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+cols,
		tenantctx.From(ctx), a.Titulo, a.Problema, a.Solucao, a.Categoria, a.Tags, a.Autor, a.TicketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectOneRow(rows, scan)
	return &out, err
}

func (r *Repository) Update(ctx context.Context, id int64, a Article) (*Article, error) {
	if a.Tags == nil {
		a.Tags = []string{}
	}
	rows, err := r.pool.Query(ctx, `
		UPDATE kb_articles SET titulo = $3, problema = $4, solucao = $5, categoria = $6,
			tags = $7, atualizado_em = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+cols,
		id, tenantctx.From(ctx), a.Titulo, a.Problema, a.Solucao, a.Categoria, a.Tags)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectOneRow(rows, scan)
	return &out, err
}

// Arquiva tira o artigo da base (não apaga — preserva histórico).
func (r *Repository) Arquiva(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE kb_articles SET status = 'arquivado' WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// Vota incrementa o contador "isto ajudou".
func (r *Repository) Vota(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE kb_articles SET uteis = uteis + 1 WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// Categorias devolve as categorias distintas em uso (para o filtro).
func (r *Repository) Categorias(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT categoria FROM kb_articles
		WHERE tenant_id = $1 AND status = 'publicado' AND categoria <> ''
		ORDER BY categoria`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
