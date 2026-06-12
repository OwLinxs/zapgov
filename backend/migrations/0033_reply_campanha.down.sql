DROP INDEX IF EXISTS idx_messages_campaign;
ALTER TABLE messages DROP COLUMN IF EXISTS campaign_id;
ALTER TABLE messages DROP COLUMN IF EXISTS reply_to;
