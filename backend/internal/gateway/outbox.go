package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
)

// maxTentativas antes de desistir de um evento. Com backoff de 30s·2^n (teto
// 2^7 = 64 min) dá ~4h de tentativas.
const maxTentativas = 10

// Falha descreve um evento que não foi entregue, para avisar a pessoa.
type Falha struct {
	TenantID int64
	Telefone string
	Final    bool // true = desistiu (não vai mais tentar)
}

// Outbox é a fila durável de eventos ZapGov -> SIGE (tabela gateway_outbox).
type Outbox struct{ pool *pgxpool.Pool }

func NewOutbox(pool *pgxpool.Pool) *Outbox { return &Outbox{pool: pool} }

// Enqueue grava o evento para entrega. dedup (wa_message_id) evita repasse duplo
// quando a Meta reentrega o mesmo webhook.
func (o *Outbox) Enqueue(ctx context.Context, phone, dedup string, ev map[string]any) error {
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if dedup == "" {
		dedup = fmt.Sprintf("sem-id-%d", time.Now().UnixNano())
	}
	_, err = o.pool.Exec(ctx, `
		INSERT INTO gateway_outbox (tenant_id, telefone, dedup, payload)
		VALUES ($1, $2, $3, $4) ON CONFLICT (tenant_id, dedup) DO NOTHING`,
		tenantctx.From(ctx), messaging.CanonPhone(phone), dedup, raw)
	return err
}

// ProcessOne entrega o próximo evento elegível. Só o pendente mais antigo de cada
// telefone é elegível: se ele falha e entra em backoff, os seguintes esperam
// (o chat chega ao técnico na ordem em que a pessoa escreveu).
//
// Devolve found=false quando não há nada a fazer. aviso != nil quando a pessoa
// deve ser avisada (primeira falha do telefone na última hora, ou desistência).
func (o *Outbox) ProcessOne(ctx context.Context, deliver func(context.Context, []byte) error) (found bool, aviso *Falha, err error) {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return false, nil, err
	}
	defer tx.Rollback(ctx)

	var id, tid int64
	var phone string
	var attempts int
	var body []byte
	err = tx.QueryRow(ctx, `
		SELECT g.id, g.tenant_id, g.telefone, g.attempts, g.payload
		FROM gateway_outbox g
		WHERE g.sent_at IS NULL AND g.failed_at IS NULL AND g.next_attempt_at <= now()
		  AND NOT EXISTS (
		      SELECT 1 FROM gateway_outbox o
		      WHERE o.tenant_id = g.tenant_id AND o.telefone = g.telefone
		        AND o.id < g.id AND o.sent_at IS NULL AND o.failed_at IS NULL)
		ORDER BY g.next_attempt_at, g.id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&id, &tid, &phone, &attempts, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	ctx = tenantctx.With(ctx, tid)
	derr := deliver(ctx, body)
	if derr == nil {
		if _, err := tx.Exec(ctx, `UPDATE gateway_outbox SET sent_at = now(), attempts = attempts + 1, last_error = '' WHERE id = $1`, id); err != nil {
			return false, nil, err
		}
		return true, nil, tx.Commit(ctx)
	}

	final := attempts+1 >= maxTentativas
	if final {
		_, err = tx.Exec(ctx, `UPDATE gateway_outbox SET attempts = attempts + 1, last_error = $2, failed_at = now(), avisado = true WHERE id = $1`, id, derr.Error())
		if err != nil {
			return false, nil, err
		}
		aviso = &Falha{TenantID: tid, Telefone: phone, Final: true}
	} else {
		// Avisa a pessoa uma vez por hora no máximo (não a cada tentativa/mensagem).
		var jaAvisado bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM gateway_outbox
			               WHERE tenant_id = $1 AND telefone = $2 AND avisado
			                 AND created_at > now() - interval '1 hour')`, tid, phone).Scan(&jaAvisado); err != nil {
			return false, nil, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE gateway_outbox
			SET attempts = attempts + 1, last_error = $2, avisado = avisado OR $3,
			    next_attempt_at = now() + interval '30 seconds' * power(2, LEAST(attempts, 7))
			WHERE id = $1`, id, derr.Error(), !jaAvisado)
		if err != nil {
			return false, nil, err
		}
		if !jaAvisado {
			aviso = &Falha{TenantID: tid, Telefone: phone}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, nil, err
	}
	return true, aviso, derr
}

// Purge apaga eventos entregues/desistidos há mais de 30 dias.
func (o *Outbox) Purge(ctx context.Context) (int64, error) {
	tag, err := o.pool.Exec(ctx, `
		DELETE FROM gateway_outbox
		WHERE (sent_at IS NOT NULL AND sent_at < now() - interval '30 days')
		   OR (failed_at IS NOT NULL AND failed_at < now() - interval '30 days')`)
	return tag.RowsAffected(), err
}

// Run drena a fila a cada segundo até ctx acabar. avisar recebe o contexto já
// com o tenant do evento.
func (o *Outbox) Run(ctx context.Context, deliver func(context.Context, []byte) error, avisar func(context.Context, Falha)) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	ultimaLimpeza := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if time.Since(ultimaLimpeza) > time.Hour {
			ultimaLimpeza = time.Now()
			if n, err := o.Purge(ctx); err != nil {
				slog.Error("gateway outbox: limpeza", "err", err)
			} else if n > 0 {
				slog.Info("gateway outbox: eventos antigos apagados", "n", n)
			}
		}
		for n := 0; n < 50; n++ {
			opctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			found, aviso, err := o.ProcessOne(opctx, deliver)
			cancel()
			if aviso != nil && avisar != nil {
				avisar(tenantctx.With(ctx, aviso.TenantID), *aviso)
			}
			if err != nil {
				slog.Warn("gateway outbox: entrega ao SIGE falhou", "err", err)
			}
			if !found {
				break
			}
			if err != nil {
				// SIGE fora: não insiste nos outros nesta rodada (o backoff cuida).
				break
			}
		}
	}
}
