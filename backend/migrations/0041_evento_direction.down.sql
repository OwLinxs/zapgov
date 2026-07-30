DELETE FROM messages WHERE direction = 'evento';
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_direction_check;
ALTER TABLE messages ADD CONSTRAINT messages_direction_check
    CHECK (direction IN ('in','out','note'));
