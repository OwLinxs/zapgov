ALTER TABLE campaign_recipients DROP COLUMN delivery_state, DROP COLUMN preview, DROP COLUMN updated_at, DROP COLUMN last_error, DROP COLUMN wa_message_id;
ALTER TABLE campaigns DROP COLUMN puladas, DROP COLUMN revisao;
UPDATE campaigns SET status='enviando' WHERE status='revisao';
UPDATE reminders SET status='falhou' WHERE status IN ('enviando','revisao');
ALTER TABLE reminders DROP CONSTRAINT reminders_status_check;
ALTER TABLE reminders ADD CONSTRAINT reminders_status_check CHECK (status IN ('pendente','enviado','falhou','cancelado'));
ALTER TABLE reminders DROP COLUMN updated_at;
