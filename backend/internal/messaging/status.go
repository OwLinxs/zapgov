package messaging

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"zapgov/backend/internal/tenantctx"
)

func nextStatus(old, incoming string) string {
	rank := map[string]int{"pending": 0, "sent": 1, "failed": 2, "delivered": 3, "read": 4}
	n, ok := rank[incoming]
	if !ok || n <= rank[old] {
		return old
	}
	return incoming
}

// UpdateStatus commits message state and campaign counters together.
func (r *Repository) UpdateStatus(ctx context.Context, waID, status, errText string) (int64, string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(ctx)
	var id, campID int64
	var old string
	err = tx.QueryRow(ctx, `SELECT id,COALESCE(campaign_id,0),status FROM messages WHERE wa_message_id=$1 AND tenant_id=$2 AND direction='out' FOR UPDATE`, waID, tenantctx.From(ctx)).Scan(&id, &campID, &old)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", err
	} // inbox retries status arriving before local persistence
	if err != nil {
		return 0, "", err
	}
	status = nextStatus(old, status)
	if status == old {
		return 0, old, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE messages SET status=$2,error=CASE WHEN $2='failed' THEN $3 ELSE '' END,updated_at=now() WHERE id=$1`, id, status, errText); err != nil {
		return 0, "", err
	}
	if campID > 0 {
		// Lock serializes the aggregate with recipient completion and other statuses.
		if _, err := tx.Exec(ctx, `SELECT id FROM campaigns WHERE id=$1 FOR UPDATE`, campID); err != nil {
			return 0, "", err
		}
		_, err = tx.Exec(ctx, `UPDATE campaigns c SET
   entregues=(SELECT count(*) FROM messages WHERE campaign_id=c.id AND status IN ('delivered','read')),
   lidas=(SELECT count(*) FROM messages WHERE campaign_id=c.id AND status='read'),
   falhas=(SELECT count(*) FROM messages WHERE campaign_id=c.id AND status='failed')+(SELECT count(*) FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state='failed')
   WHERE c.id=$1 AND c.tenant_id=$2`, campID, tenantctx.From(ctx))
		if err != nil {
			return 0, "", err
		}
	}
	return 0, old, tx.Commit(ctx) // counters already applied: no best-effort callback
}
