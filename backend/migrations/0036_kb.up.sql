-- Base de conhecimento do T.I.: artigos pesquisáveis (problema/solução) que os
-- técnicos registram a partir dos atendimentos.
CREATE TABLE IF NOT EXISTS kb_articles (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id     BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    titulo        TEXT NOT NULL,
    problema      TEXT NOT NULL DEFAULT '',
    solucao       TEXT NOT NULL DEFAULT '',
    categoria     TEXT NOT NULL DEFAULT '',
    tags          TEXT[] NOT NULL DEFAULT '{}',
    autor         TEXT NOT NULL DEFAULT '',
    ticket_id     BIGINT REFERENCES tickets(id),
    status        TEXT NOT NULL DEFAULT 'publicado', -- publicado | arquivado
    views         INT NOT NULL DEFAULT 0,
    uteis         INT NOT NULL DEFAULT 0,
    criado_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- busca full-text em português (título + problema + solução + tags)
    busca tsvector GENERATED ALWAYS AS (
        to_tsvector('portuguese',
            coalesce(titulo,'')||' '||coalesce(problema,'')||' '||
            coalesce(solucao,'')||' '||array_to_string(tags,' '))
    ) STORED
);
CREATE INDEX IF NOT EXISTS idx_kb_busca ON kb_articles USING GIN (busca);
CREATE INDEX IF NOT EXISTS idx_kb_tenant ON kb_articles(tenant_id, status, atualizado_em DESC);
