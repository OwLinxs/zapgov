-- Destinatários da campanha persistidos: necessário p/ disparo agendado
-- (enfileirar depois) e personalizado (variáveis por destinatário).
CREATE TABLE IF NOT EXISTS campaign_recipients (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id),
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    phone       TEXT NOT NULL,
    nome        TEXT NOT NULL DEFAULT '',
    params      JSONB NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_camp_recip_campaign ON campaign_recipients(campaign_id);

-- Campanha cancelada (agendada que não vai mais sair).
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_status_check;

-- Disparo personalizado (variáveis por destinatário) — habilita lembretes.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS personalizado BOOLEAN NOT NULL DEFAULT false;
