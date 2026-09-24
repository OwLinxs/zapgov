package campaigns

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"zapgov/backend/internal/queue"
	"zapgov/backend/internal/tenantctx"
)

// CreateDelivery records the campaign and its complete outbox atomically.
func (r *Repository) CreateDelivery(ctx context.Context, c Campaign, recs []Recipient, body string) (*Campaign, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO campaigns(tenant_id,template,language,setor,total,status,operador,personalizado,scheduled_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,criado_em`, tenantctx.From(ctx), c.Template, c.Language, c.Setor, c.Total, c.Status, c.Operador, c.Personalizado, c.ScheduledAt).Scan(&c.ID, &c.CriadoEm)
	if err != nil {
		return nil, err
	}
	for _, rec := range recs {
		params := rec.Params
		if params == nil {
			params = []string{}
		}
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		preview := render(body, params)
		if preview == "" {
			preview = "[modelo: " + c.Template + "]"
		}
		_, err = tx.Exec(ctx, `INSERT INTO campaign_recipients(tenant_id,campaign_id,phone,nome,params,preview) VALUES($1,$2,$3,$4,$5,$6)`, tenantctx.From(ctx), c.ID, rec.Phone, rec.Nome, raw, preview)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

// DispatchPending is recoverable: Redis may be unavailable or lose a queued task.
// The recipient's database state is the authority, not the Redis task lifetime.
func (r *Repository) DispatchPending(ctx context.Context, enq Enqueuer, limit int) (int, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE campaigns SET status='enviando' WHERE status='agendado' AND scheduled_at<=now()`); err != nil {
		return 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT r.id,r.tenant_id,r.campaign_id,r.phone,c.template,c.language,r.params,r.preview
 FROM campaign_recipients r JOIN campaigns c ON c.id=r.campaign_id JOIN tenants t ON t.id=r.tenant_id
 WHERE t.ativo AND c.status='enviando' AND (r.delivery_state='pending' OR (r.delivery_state='queued' AND r.updated_at<now()-interval '5 minutes'))
 ORDER BY r.id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (queue.CampaignSendPayload, error) {
		var p queue.CampaignSendPayload
		var raw []byte
		err := row.Scan(&p.RecipientID, &p.TenantID, &p.CampaignID, &p.To, &p.Template, &p.Language, &raw, &p.Preview)
		if err == nil {
			err = json.Unmarshal(raw, &p.Params)
		}
		return p, err
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range tasks {
		if err := enq.EnqueueCampaignSend(p); err != nil {
			return n, err
		}
		_, err := r.pool.Exec(ctx, `UPDATE campaign_recipients SET delivery_state='queued',updated_at=now() WHERE id=$1 AND delivery_state IN ('pending','queued')`, p.RecipientID)
		if err != nil {
			return n, err
		}
		n++
	}
	// A crash after starting a provider call is ambiguous. Never resend automatically.
	stale, err := r.pool.Query(ctx, `SELECT id,tenant_id,campaign_id FROM campaign_recipients WHERE delivery_state='sending' AND updated_at<now()-interval '5 minutes' LIMIT $1`, limit)
	if err != nil {
		return n, err
	}
	type item struct{ id, tid, cid int64 }
	items, err := pgx.CollectRows(stale, func(row pgx.CollectableRow) (item, error) {
		var x item
		e := row.Scan(&x.id, &x.tid, &x.cid)
		return x, e
	})
	if err != nil {
		return n, err
	}
	for _, x := range items {
		if err := r.FinishDelivery(tenantctx.With(ctx, x.tid), x.id, x.cid, "uncertain", "", 0, "Processo interrompido durante envio; conferir antes de reenviar."); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (r *Repository) ClaimDelivery(ctx context.Context, id, campID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE campaign_recipients r SET delivery_state='sending',updated_at=now()
 FROM campaigns c,tenants t WHERE r.id=$1 AND r.campaign_id=$2 AND r.tenant_id=$3 AND c.id=r.campaign_id
 AND t.id=r.tenant_id AND t.ativo AND c.status='enviando' AND r.delivery_state IN ('pending','queued')`, id, campID, tenantctx.From(ctx))
	return err == nil && tag.RowsAffected() == 1, err
}

func (r *Repository) RetryDelivery(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE campaign_recipients SET delivery_state='pending',updated_at=now() WHERE id=$1 AND tenant_id=$2 AND delivery_state='sending'`, id, tenantctx.From(ctx))
	return err
}

// FinishDelivery serializes completion and counter updates for a campaign.
func (r *Repository) FinishDelivery(ctx context.Context, id, campID int64, state, waID string, msgID int64, detail string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists int64
	if err := tx.QueryRow(ctx, `SELECT id FROM campaigns WHERE id=$1 AND tenant_id=$2 FOR UPDATE`, campID, tenantctx.From(ctx)).Scan(&exists); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE campaign_recipients SET delivery_state=$3,wa_message_id=$4,last_error=$5,updated_at=now() WHERE id=$1 AND tenant_id=$2 AND campaign_id=$6 AND delivery_state='sending'`, id, tenantctx.From(ctx), state, waID, detail, campID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	if msgID > 0 {
		if _, err := tx.Exec(ctx, `UPDATE messages SET campaign_id=$2 WHERE id=$1 AND tenant_id=$3`, msgID, campID, tenantctx.From(ctx)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE campaigns c SET
 enviadas=(SELECT count(*) FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state='sent'),
 puladas=(SELECT count(*) FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state='skipped'),
 revisao=(SELECT count(*) FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state='uncertain'),
 falhas=(SELECT count(*) FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state='failed')+(SELECT count(*) FROM messages WHERE campaign_id=c.id AND status='failed'),
 entregues=(SELECT count(*) FROM messages WHERE campaign_id=c.id AND status IN ('delivered','read')),
 lidas=(SELECT count(*) FROM messages WHERE campaign_id=c.id AND status='read'),
 status=CASE WHEN EXISTS(SELECT 1 FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state IN ('pending','queued','sending')) THEN 'enviando'
 WHEN EXISTS(SELECT 1 FROM campaign_recipients WHERE campaign_id=c.id AND delivery_state='uncertain') THEN 'revisao' ELSE 'enviado' END
 WHERE c.id=$1 AND c.tenant_id=$2`, campID, tenantctx.From(ctx))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeliveryPayload loads trusted recipient data rather than trusting a queue payload.
func (r *Repository) DeliveryPayload(ctx context.Context, id, campID int64) (queue.CampaignSendPayload, error) {
	var p queue.CampaignSendPayload
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT r.id,r.tenant_id,r.campaign_id,r.phone,c.template,c.language,r.params,r.preview FROM campaign_recipients r JOIN campaigns c ON c.id=r.campaign_id WHERE r.id=$1 AND r.campaign_id=$2 AND r.tenant_id=$3`, id, campID, tenantctx.From(ctx)).Scan(&p.RecipientID, &p.TenantID, &p.CampaignID, &p.To, &p.Template, &p.Language, &raw, &p.Preview)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	if err == nil {
		err = json.Unmarshal(raw, &p.Params)
	}
	return p, err
}
