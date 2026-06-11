-- Grupos de templates (ex.: "Secretaria de Saúde"): organização visual,
-- independente da atribuição por departamento (que controla quem usa).
CREATE TABLE IF NOT EXISTS template_groups (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    nome      TEXT NOT NULL,
    UNIQUE (tenant_id, nome)
);

-- Cada template pertence a (no máximo) um grupo.
CREATE TABLE IF NOT EXISTS template_group_items (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    template  TEXT NOT NULL,
    group_id  BIGINT NOT NULL REFERENCES template_groups(id) ON DELETE CASCADE,
    UNIQUE (tenant_id, template)
);
