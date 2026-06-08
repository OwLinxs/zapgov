-- Força troca de senha no primeiro acesso (ex: admin seed com senha padrão).
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;
