-- Respostas rápidas (canned): atalho + texto pronto para o atendente.
CREATE TABLE IF NOT EXISTS quick_replies (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    atalho    TEXT NOT NULL,
    texto     TEXT NOT NULL,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_quick_replies_tenant ON quick_replies(tenant_id);
