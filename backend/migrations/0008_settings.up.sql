-- Configurações da organização (single-tenant por enquanto) + webhooks + API key.

CREATE TABLE IF NOT EXISTS org_settings (
    id            INT PRIMARY KEY DEFAULT 1,
    nome          TEXT NOT NULL DEFAULT 'Minha Organização',
    uf            TEXT NOT NULL DEFAULT 'SP',
    subdominio    TEXT NOT NULL DEFAULT '',
    fuso          TEXT NOT NULL DEFAULT 'America/Sao_Paulo',
    plano         TEXT NOT NULL DEFAULT 'Gov Pro',
    limite_mensal INT  NOT NULL DEFAULT 250000,
    api_key       TEXT NOT NULL DEFAULT '',
    CONSTRAINT org_single CHECK (id = 1)
);
INSERT INTO org_settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url       TEXT NOT NULL,
    eventos   TEXT[] NOT NULL DEFAULT '{}',
    ativo     BOOLEAN NOT NULL DEFAULT true,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
