-- Pesquisa de satisfação pós-atendimento + SLA da fila.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS pesquisa_ativo    BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS pesquisa_msg      TEXT NOT NULL DEFAULT 'Seu atendimento foi finalizado. De 1 a 5, como você avalia o atendimento?';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS pesquisa_obrigado TEXT NOT NULL DEFAULT 'Obrigado pela sua avaliação! 🙏';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS sla_minutos       INT NOT NULL DEFAULT 15;

CREATE TABLE IF NOT EXISTS surveys (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    atendente       TEXT NOT NULL DEFAULT '',
    setor           TEXT NOT NULL DEFAULT '',
    enviada_em      TIMESTAMPTZ NOT NULL DEFAULT now(),
    nota            INT,                  -- 1-5; NULL = sem resposta
    respondida_em   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_surveys_tenant ON surveys(tenant_id, enviada_em);
-- No máximo UMA pesquisa pendente por conversa.
CREATE UNIQUE INDEX IF NOT EXISTS uq_surveys_pendente ON surveys(tenant_id, conversation_id) WHERE nota IS NULL;

-- SLA: quando a conversa entrou na fila (aguardando).
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS queued_at TIMESTAMPTZ;
