-- Responder mensagem (citação) + ligação mensagem↔campanha (contadores de
-- entregues/lidas por disparo via webhook de status).
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to    TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS campaign_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_messages_campaign ON messages(campaign_id) WHERE campaign_id IS NOT NULL;
