// Package surveys gerencia a pesquisa de satisfação pós-atendimento (nota 1-5
// enviada ao cidadão quando o atendente finaliza a conversa).
package surveys

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Create registra uma pesquisa pendente para a conversa (uma por vez —
// duplicada é ignorada pela UNIQUE parcial).
func (r *Repository) Create(ctx context.Context, convID int64, atendente, setor string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO surveys (tenant_id, conversation_id, atendente, setor)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`,
		tenantctx.From(ctx), convID, atendente, setor)
	return err
}

// Answer grava a nota da pesquisa pendente da conversa. Devolve true se havia
// pendência (resposta consumida).
func (r *Repository) Answer(ctx context.Context, convID int64, nota int) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE surveys SET nota = $3, respondida_em = now()
		WHERE tenant_id = $1 AND conversation_id = $2 AND nota IS NULL`,
		tenantctx.From(ctx), convID, nota)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// HasPending diz se a conversa tem pesquisa aguardando resposta.
func (r *Repository) HasPending(ctx context.Context, convID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM surveys WHERE tenant_id = $1 AND conversation_id = $2 AND nota IS NULL)`,
		tenantctx.From(ctx), convID).Scan(&ok)
	return ok, err
}

// Stats agrega satisfação (média geral, respondidas, enviadas) dos últimos 30 dias.
type Stats struct {
	Media      float64 `json:"media"`
	Respostas  int     `json:"respostas"`
	Enviadas   int     `json:"enviadas"`
}

func (r *Repository) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(avg(nota) FILTER (WHERE nota IS NOT NULL), 0),
		       count(*) FILTER (WHERE nota IS NOT NULL),
		       count(*)
		FROM surveys WHERE tenant_id = $1 AND enviada_em >= now() - interval '30 days'`,
		tenantctx.From(ctx)).Scan(&s.Media, &s.Respostas, &s.Enviadas)
	return s, err
}

var _ = pgx.ErrNoRows

// ParseNota extrai a nota 1-5 de uma resposta de pesquisa: id de botão
// "nota_N" ou texto contendo só o número. (0, false) se não for nota.
func ParseNota(buttonID, body string) (int, bool) {
	if strings.HasPrefix(buttonID, "nota_") {
		if n, err := strconv.Atoi(strings.TrimPrefix(buttonID, "nota_")); err == nil && n >= 1 && n <= 5 {
			return n, true
		}
	}
	t := strings.TrimSpace(body)
	// aceita "5", "5 estrelas", "⭐⭐⭐⭐⭐ Excelente" (título da lista)…
	if len(t) > 0 {
		if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= 5 {
			return n, true
		}
		if strings.Count(t, "⭐") >= 1 && strings.Count(t, "⭐") <= 5 {
			return strings.Count(t, "⭐"), true
		}
	}
	return 0, false
}
