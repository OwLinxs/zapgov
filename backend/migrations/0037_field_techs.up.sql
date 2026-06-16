-- Técnicos de campo: contatos de WhatsApp (sem login) que recebem ordens de
-- serviço encaminhadas a partir dos chamados.
CREATE TABLE IF NOT EXISTS field_techs (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    nome      TEXT NOT NULL,
    telefone  TEXT NOT NULL,
    ativo     BOOLEAN NOT NULL DEFAULT true,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_field_techs_tenant ON field_techs(tenant_id, ativo);
