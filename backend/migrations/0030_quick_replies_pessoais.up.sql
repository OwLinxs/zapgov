-- Respostas rápidas pessoais: user_id NULL = global do órgão; preenchido = só do atendente.
ALTER TABLE quick_replies ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
