ALTER TABLE campaign_recipients ADD COLUMN delivery_state TEXT NOT NULL DEFAULT 'pending'
 CHECK (delivery_state IN ('pending','queued','sending','sent','failed','skipped','uncertain'));
ALTER TABLE campaign_recipients ADD COLUMN preview TEXT NOT NULL DEFAULT '';
ALTER TABLE campaign_recipients ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE campaign_recipients ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE campaign_recipients ADD COLUMN wa_message_id TEXT NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN puladas INT NOT NULL DEFAULT 0;
ALTER TABLE campaigns ADD COLUMN revisao INT NOT NULL DEFAULT 0;
-- Never replay recipients from a campaign already dispatched by the old worker.
UPDATE campaign_recipients r SET delivery_state = CASE WHEN c.status='enviado' THEN 'sent' ELSE 'uncertain' END,
 last_error='Campanha anterior ao controle de entrega; conferir histórico antes de reenviar.'
 FROM campaigns c WHERE c.id=r.campaign_id AND c.status <> 'agendado';
UPDATE campaigns c SET revisao=(SELECT count(*) FROM campaign_recipients r WHERE r.campaign_id=c.id AND r.delivery_state='uncertain'), status='revisao'
 WHERE c.status='enviando';
CREATE INDEX idx_campaign_delivery_pending ON campaign_recipients(delivery_state,updated_at);
ALTER TABLE reminders DROP CONSTRAINT reminders_status_check;
ALTER TABLE reminders ADD CONSTRAINT reminders_status_check CHECK (status IN ('pendente','enviando','enviado','falhou','cancelado','revisao'));
ALTER TABLE reminders ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
