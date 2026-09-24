package techbot

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Load devolve o passo atual e o chamado em foco do técnico (vazios se não há
// sessão). Nunca falha ruidosamente: em erro devolve estado neutro.
func (r *Repository) Load(ctx context.Context, phone string) (step string, ticketID int64) {
	var tid *int64
	err := r.pool.QueryRow(ctx,
		`SELECT step, ticket_id FROM techbot_sessions WHERE tenant_id = $2 AND telefone = $1`,
		messaging.CanonPhone(phone), tenantctx.From(ctx)).Scan(&step, &tid)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return "", 0
	}
	if tid != nil {
		ticketID = *tid
	}
	return step, ticketID
}

// Save grava o passo e o chamado em foco (upsert por técnico).
func (r *Repository) Save(ctx context.Context, phone, step string, ticketID int64) error {
	var tid *int64
	if ticketID > 0 {
		tid = &ticketID
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO techbot_sessions (tenant_id, telefone, step, ticket_id, updated_at)
		VALUES ($4, $1, $2, $3, now())
		ON CONFLICT (tenant_id, telefone) DO UPDATE SET step = $2, ticket_id = $3, updated_at = now()`,
		messaging.CanonPhone(phone), step, tid, tenantctx.From(ctx))
	return err
}

// Clear apaga a sessão do técnico (volta ao ponto de partida).
func (r *Repository) Clear(ctx context.Context, phone string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM techbot_sessions WHERE tenant_id = $2 AND telefone = $1`,
		messaging.CanonPhone(phone), tenantctx.From(ctx))
	return err
}
