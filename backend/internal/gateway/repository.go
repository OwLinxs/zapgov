// Package gateway faz a ponte bidirecional entre o WhatsApp (Meta) e o sistema
// de chamados SIGE: bot cru abre/retoma chamado e, em relay, repassa as mensagens
// nos dois sentidos. O SIGE é dono do chamado e do chat. Ver deploy/GATEWAY.md.
package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

// Passos da sessão do gateway.
const (
	StepMenu     = "menu"
	StepAbrindo  = "abrindo"
	StepListando = "listando"
	StepRelay    = "relay"
)

// Session é o estado da conversa no gateway.
type Session struct {
	Step       string
	TicketID   int64
	Referencia string
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Load devolve a sessão do telefone (Step "menu" quando não há).
func (r *Repository) Load(ctx context.Context, phone string) Session {
	var s Session
	var tid *int64
	err := r.pool.QueryRow(ctx,
		`SELECT step, ticket_id, referencia FROM gateway_sessions WHERE tenant_id = $2 AND telefone = $1`,
		messaging.CanonPhone(phone), tenantctx.From(ctx)).Scan(&s.Step, &tid, &s.Referencia)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return Session{Step: StepMenu}
	}
	if tid != nil {
		s.TicketID = *tid
	}
	if s.Step == "" {
		s.Step = StepMenu
	}
	return s
}

// Save grava a sessão (upsert por telefone).
func (r *Repository) Save(ctx context.Context, phone string, s Session) error {
	var tid *int64
	if s.TicketID > 0 {
		tid = &s.TicketID
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO gateway_sessions (tenant_id, telefone, step, ticket_id, referencia, atualizado_em)
		VALUES ($5, $1, $2, $3, $4, now())
		ON CONFLICT (tenant_id, telefone)
		DO UPDATE SET step = $2, ticket_id = $3, referencia = $4, atualizado_em = now()`,
		messaging.CanonPhone(phone), s.Step, tid, s.Referencia, tenantctx.From(ctx))
	return err
}

// Clear volta o telefone ao início (menu).
func (r *Repository) Clear(ctx context.Context, phone string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM gateway_sessions WHERE tenant_id = $2 AND telefone = $1`,
		messaging.CanonPhone(phone), tenantctx.From(ctx))
	return err
}

// WindowExpiry lê o fim da janela de 24h da conversa (nil se não há prazo).
func (r *Repository) WindowExpiry(ctx context.Context, phone string) *time.Time {
	var expires *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT window_expires_at FROM conversations WHERE tenant_id = $1 AND contact_phone = $2`,
		tenantctx.From(ctx), messaging.CanonPhone(phone)).Scan(&expires)
	if err != nil {
		return nil
	}
	return expires
}
