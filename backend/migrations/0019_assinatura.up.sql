-- Assinatura do atendente no fim de cada mensagem enviada manualmente.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS assinatura_ativo BOOLEAN NOT NULL DEFAULT false;
