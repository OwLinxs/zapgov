-- Dados coletados pelo bot (variáveis) entregues ao atendente no handoff.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS dados JSONB NOT NULL DEFAULT '{}';
