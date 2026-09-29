-- Sessão do gateway WhatsApp<->SIGE por telefone: passo do menu do bot cru
-- (menu/abrindo/listando) ou conversa em relay (vinculada a um chamado do SIGE).
CREATE TABLE IF NOT EXISTS gateway_sessions (
    tenant_id     BIGINT NOT NULL,
    telefone      TEXT   NOT NULL,               -- número canônico da pessoa
    step          TEXT   NOT NULL DEFAULT 'menu', -- menu | abrindo | listando | relay
    ticket_id     BIGINT,                         -- chamado do SIGE em relay
    referencia    TEXT   NOT NULL DEFAULT '',      -- wa-<uuid> da abertura em curso (idempotência)
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, telefone)
);
