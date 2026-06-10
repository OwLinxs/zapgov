-- Notas internas: mensagens que NÃO vão pro cidadão (direction='note').
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_direction_check;
ALTER TABLE messages ADD CONSTRAINT messages_direction_check CHECK (direction IN ('in','out','note'));
-- Quem escreveu a nota (para exibir no histórico).
ALTER TABLE messages ADD COLUMN IF NOT EXISTS autor TEXT NOT NULL DEFAULT '';
