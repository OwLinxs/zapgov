-- Restrição de templates por departamento.
-- Cada linha libera um template para um setor. Template sem nenhuma linha =
-- usável só por admin/supervisor (atendente não vê).
CREATE TABLE IF NOT EXISTS template_setores (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    template  TEXT NOT NULL,
    setor     TEXT NOT NULL,
    UNIQUE (tenant_id, template, setor)
);
CREATE INDEX IF NOT EXISTS idx_template_setores_tenant ON template_setores(tenant_id, template);
