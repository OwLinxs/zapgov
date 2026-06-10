-- Lembretes automáticos: detecta data (DD/MM/AAAA) nas variáveis do template
-- enviado e agenda um lembrete pro dia anterior.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS lembrete_ativo    BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS lembrete_template TEXT NOT NULL DEFAULT '';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS lembrete_idioma   TEXT NOT NULL DEFAULT 'pt_BR';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS lembrete_hora     TEXT NOT NULL DEFAULT '09:00';

CREATE TABLE IF NOT EXISTS reminders (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    phone       TEXT NOT NULL,
    nome        TEXT NOT NULL DEFAULT '',
    data_evento DATE NOT NULL,            -- data da consulta/cirurgia
    enviar_em   TIMESTAMPTZ NOT NULL,     -- véspera, na hora configurada
    status      TEXT NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente','enviado','falhou','cancelado')),
    criado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Evita lembrete duplicado pro mesmo paciente/data.
CREATE UNIQUE INDEX IF NOT EXISTS uq_reminders_tenant_phone_data
    ON reminders(tenant_id, phone, data_evento) WHERE status = 'pendente';
CREATE INDEX IF NOT EXISTS idx_reminders_due ON reminders(status, enviar_em);
