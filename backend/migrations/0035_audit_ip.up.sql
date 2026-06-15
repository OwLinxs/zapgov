-- Auditoria: registra o IP de origem da ação (rastreio de segurança).
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS ip TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(tenant_id, user_id, criado_em);
