-- Módulo de chamados do T.I. (helpdesk). Domínio próprio, reaproveita
-- auth/notificação/SLA do ZapGov. Link opcional à conversa de origem.
CREATE TABLE IF NOT EXISTS tickets (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id         BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    numero            INT NOT NULL,            -- sequencial por tenant (#123) p/ humano
    titulo            TEXT NOT NULL,
    descricao         TEXT NOT NULL DEFAULT '',
    categoria         TEXT NOT NULL DEFAULT '',
    prioridade        TEXT NOT NULL DEFAULT 'media',  -- baixa|media|alta|critica
    status            TEXT NOT NULL DEFAULT 'aberto',  -- aberto|em_andamento|aguardando|resolvido|cancelado
    solicitante_nome  TEXT NOT NULL DEFAULT '',
    solicitante_fone  TEXT NOT NULL DEFAULT '',
    secretaria        TEXT NOT NULL DEFAULT '',
    local             TEXT NOT NULL DEFAULT '',
    tecnico           TEXT NOT NULL DEFAULT '',
    conversation_id   BIGINT REFERENCES conversations(id),
    criado_em         TIMESTAMPTZ NOT NULL DEFAULT now(),
    atribuido_em      TIMESTAMPTZ,
    resolvido_em      TIMESTAMPTZ,
    sla_venc          TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tickets_numero ON tickets(tenant_id, numero);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(tenant_id, status, prioridade);

-- Timeline: comentários e andamentos do chamado.
CREATE TABLE IF NOT EXISTS ticket_comments (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    ticket_id  BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    autor      TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL,
    interno    BOOLEAN NOT NULL DEFAULT false,  -- true = só técnicos veem
    criado_em  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ticket_comments ON ticket_comments(ticket_id, criado_em);

-- Config do módulo: SLA por prioridade (horas) e categorias.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ti_sla_horas JSONB
    NOT NULL DEFAULT '{"critica":4,"alta":8,"media":24,"baixa":72}';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ti_categorias TEXT[]
    NOT NULL DEFAULT ARRAY['Hardware','Rede','Sistema','Impressora','E-mail','Outros'];
