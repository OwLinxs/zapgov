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
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Função wrapper IMMUTABLE: to_tsvector('portuguese', ...) por nome de config é
-- tratado como não-imutável pelo Postgres (não serve em índice/coluna gerada).
-- Marcar a função como IMMUTABLE resolve (a config 'portuguese' é fixa na prática).
CREATE OR REPLACE FUNCTION kb_busca_tsv(_titulo text, _problema text, _solucao text, _tags text[])
RETURNS tsvector LANGUAGE sql IMMUTABLE AS $$
    SELECT to_tsvector('portuguese',
        coalesce(_titulo,'')||' '||coalesce(_problema,'')||' '||
        coalesce(_solucao,'')||' '||array_to_string(_tags,' '));
$$;

CREATE INDEX IF NOT EXISTS idx_kb_busca ON kb_articles
    USING GIN (kb_busca_tsv(titulo, problema, solucao, tags));
CREATE INDEX IF NOT EXISTS idx_kb_tenant ON kb_articles(tenant_id, status, atualizado_em DESC);
