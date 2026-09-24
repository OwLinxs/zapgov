-- Estado da sessão do "modo técnico" no WhatsApp: cada técnico de campo navega
-- os chamados por menu/botões. Guarda o passo atual e o chamado em foco.
CREATE TABLE IF NOT EXISTS techbot_sessions (
    tenant_id  BIGINT NOT NULL,
    telefone   TEXT   NOT NULL,          -- número canônico do técnico
    step       TEXT   NOT NULL DEFAULT '', -- '' (menu) | note | search
    ticket_id  BIGINT,                    -- chamado em foco (detalhe/comentário)
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, telefone)
);
