-- Eventos de sistema no chat (resolver/assumir/transferir) usam direction
-- 'evento'. A constraint da 0023 só permitia in/out/note → inserts falhavam.
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_direction_check;
ALTER TABLE messages ADD CONSTRAINT messages_direction_check
    CHECK (direction IN ('in','out','note','evento'));
