-- Contatos e campanhas (disparos). Ainda single-tenant.

CREATE TABLE IF NOT EXISTS contacts (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome            TEXT NOT NULL,
    telefone        TEXT NOT NULL UNIQUE,
    cpf             TEXT NOT NULL DEFAULT '',
    setor           TEXT NOT NULL DEFAULT 'Geral',
    tags            TEXT[] NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'ativo' CHECK (status IN ('ativo','bloqueado')),
    mensagens       INT  NOT NULL DEFAULT 0,
    ultimo_contato  TIMESTAMPTZ,
    criado_em       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contacts_setor ON contacts(setor);

CREATE TABLE IF NOT EXISTS campaigns (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    template      TEXT NOT NULL,
    language      TEXT NOT NULL DEFAULT 'pt_BR',
    setor         TEXT NOT NULL DEFAULT 'Geral',
    total         INT  NOT NULL DEFAULT 0,
    enviadas      INT  NOT NULL DEFAULT 0,
    entregues     INT  NOT NULL DEFAULT 0,
    lidas         INT  NOT NULL DEFAULT 0,
    falhas        INT  NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'enviando', -- enviando|enviado|agendado|falhou
    operador      TEXT NOT NULL DEFAULT '',
    scheduled_at  TIMESTAMPTZ,
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_campaigns_criado ON campaigns(criado_em DESC);

-- Liga mensagens a uma campanha (para futuras métricas de entrega/leitura por disparo).
ALTER TABLE messages ADD COLUMN IF NOT EXISTS campaign_id BIGINT REFERENCES campaigns(id) ON DELETE SET NULL;
