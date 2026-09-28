package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"zapgov/backend/internal/tenantctx"
)

type Envelope struct {
	TenantID int64
	Body     []byte
}
type Inbox struct{ pool *pgxpool.Pool }

func NewInbox(pool *pgxpool.Pool) *Inbox { return &Inbox{pool: pool} }

// Accept persists the entire verified batch before acknowledging the webhook.
func (i *Inbox) Accept(ctx context.Context, batch []Envelope) error {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, event := range batch {
		sum := sha256.Sum256(event.Body)
		if _, err := tx.Exec(ctx, `INSERT INTO webhook_inbox(tenant_id,digest,payload) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, event.TenantID, hex.EncodeToString(sum[:]), event.Body); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (i *Inbox) ProcessOne(ctx context.Context, process func(context.Context, []byte) error) (bool, error) {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id, tid int64
	var body []byte
	err = tx.QueryRow(ctx, `SELECT id,tenant_id,payload FROM webhook_inbox WHERE processed_at IS NULL AND next_attempt_at<=now() ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &tid, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Serialize a tenant's bot state even with several API replicas.
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(73401,$1::int)`, tid).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	processErr := process(tenantctx.With(ctx, tid), body)
	if processErr != nil {
		_, err = tx.Exec(ctx, `UPDATE webhook_inbox SET attempts=attempts+1, last_error=$2, next_attempt_at=now()+interval '30 seconds' * power(2, LEAST(attempts,7)) WHERE id=$1`, id, processErr.Error())
	} else {
		_, err = tx.Exec(ctx, `UPDATE webhook_inbox SET processed_at=now(), attempts=attempts+1, last_error='' WHERE id=$1`, id)
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, processErr
}

func (i *Inbox) Run(ctx context.Context, process func(context.Context, []byte) error) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		for n := 0; n < 50; n++ {
			opctx, cancel := context.WithTimeout(ctx, 90*time.Second)
			found, err := i.ProcessOne(opctx, process)
			cancel()
			if err != nil {
				slog.Error("webhook inbox", "err", err)
				// found=true means the failure was durably rescheduled.
				// Continue draining so a bad status cannot block other events.
				if !found {
					break
				}
			}
			if !found {
				break
			}
		}
	}
}

// splitPayload retains the Meta envelope while separating phone numbers.
func splitPayload(body []byte) ([][]byte, error) {
	var root struct {
		Object string `json:"object"`
		Entry  []struct {
			ID      string            `json:"id"`
			Changes []json.RawMessage `json:"changes"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	var out [][]byte
	for _, entry := range root.Entry {
		for _, change := range entry.Changes {
			raw, err := json.Marshal(map[string]any{"object": root.Object, "entry": []any{map[string]any{"id": entry.ID, "changes": []json.RawMessage{change}}}})
			if err != nil {
				return nil, err
			}
			if phoneNumberIDFromBody(raw) == "" {
				return nil, fmt.Errorf("evento sem phone_number_id")
			}
			out = append(out, raw)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("webhook sem eventos")
	}
	return out, nil
}
