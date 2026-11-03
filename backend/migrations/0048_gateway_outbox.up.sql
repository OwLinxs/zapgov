-- Fila de eventos ZapGov -> SIGE (mensagem.recebida). O inbound grava aqui e um
-- loop entrega com retry/backoff; a mensagem não se perde se o SIGE cair.
-- Entrega em ordem por telefone (só o evento pendente mais antigo de cada
-- telefone é elegível). Ver deploy/GATEWAY.md.
CREATE TABLE IF NOT EXISTS gateway_outbox (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    telefone        TEXT   NOT NULL,                 -- número canônico da pessoa
    dedup           TEXT   NOT NULL,                 -- wa_message_id (evita repasse duplo)
    payload         JSONB  NOT NULL,                 -- corpo do evento enviado ao SIGE
    attempts        INT    NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ,                     -- entregue (2xx do SIGE)
    failed_at       TIMESTAMPTZ,                     -- desistiu após o máximo de tentativas
    avisado         BOOLEAN NOT NULL DEFAULT false,  -- pessoa já foi avisada da demora
    last_error      TEXT   NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, dedup)
);
CREATE INDEX IF NOT EXISTS idx_gateway_outbox_pending
    ON gateway_outbox (next_attempt_at, id) WHERE sent_at IS NULL AND failed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_gateway_outbox_phone
    ON gateway_outbox (tenant_id, telefone, id);
