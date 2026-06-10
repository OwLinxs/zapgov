-- Reações com emoji (WhatsApp): emoji atual aplicado a uma mensagem.
-- Última reação vence; vazio = sem reação/removida.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reaction TEXT NOT NULL DEFAULT '';
