ALTER TABLE messages DROP COLUMN IF EXISTS autor;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_direction_check;
ALTER TABLE messages ADD CONSTRAINT messages_direction_check CHECK (direction IN ('in','out'));
