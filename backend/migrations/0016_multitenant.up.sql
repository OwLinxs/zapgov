-- ============================================================
-- Multi-tenant (modelo row-level: tenant_id em cada tabela).
-- Tenant 1 = organização padrão (dados existentes migram p/ ela).
-- ============================================================

-- 1. Tabela de tenants (órgãos) com credenciais Meta próprias.
CREATE TABLE IF NOT EXISTS tenants (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome                 TEXT NOT NULL,
    subdominio           TEXT NOT NULL UNIQUE,
    plano                TEXT NOT NULL DEFAULT 'Gov Pro',
    limite_mensal        INT  NOT NULL DEFAULT 250000,
    ativo                BOOLEAN NOT NULL DEFAULT true,
    -- Credenciais Meta (por tenant). Vazias => cai no fallback do .env (tenant 1).
    wa_phone_number_id   TEXT NOT NULL DEFAULT '',
    wa_waba_id           TEXT NOT NULL DEFAULT '',
    wa_token             TEXT NOT NULL DEFAULT '',
    wa_app_secret        TEXT NOT NULL DEFAULT '',
    wa_api_version       TEXT NOT NULL DEFAULT 'v21.0',
    webhook_verify_token TEXT NOT NULL DEFAULT '',
    criado_em            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Garante o tenant 1 (padrão) com id fixo = 1.
INSERT INTO tenants (nome, subdominio)
SELECT 'Organização Padrão', 'default'
WHERE NOT EXISTS (SELECT 1 FROM tenants);

-- Lookup rápido: número da Meta -> tenant (roteamento de webhook de entrada).
CREATE INDEX IF NOT EXISTS idx_tenants_phone ON tenants(wa_phone_number_id);

-- 2. tenant_id em todas as tabelas, default 1 (backfill automático dos dados atuais).
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'audit_log','bot_config','bot_options','campaigns','contacts',
    'conversation_flow_state','conversation_labels','conversations','departments',
    'flows','labels','messages','opt_outs','org_settings','sessions','users','webhook_endpoints'
  ] LOOP
    EXECUTE format(
      'ALTER TABLE %I ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES tenants(id)', t);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_tenant ON %I(tenant_id)', t, t);
  END LOOP;
END $$;

-- 3. UNIQUEs viram compostas (mesmo telefone/email/wa_id pode existir em tenants distintos).
ALTER TABLE users    DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_telefone_key;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_wa_message_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_tenant_email      ON users(tenant_id, email);
CREATE UNIQUE INDEX IF NOT EXISTS uq_contacts_tenant_tel     ON contacts(tenant_id, telefone);
CREATE UNIQUE INDEX IF NOT EXISTS uq_messages_tenant_waid    ON messages(tenant_id, wa_message_id);

-- 4. org_settings / bot_config deixam de ser singleton (uma linha por tenant).
ALTER TABLE org_settings DROP CONSTRAINT IF EXISTS org_single;
CREATE UNIQUE INDEX IF NOT EXISTS uq_org_settings_tenant ON org_settings(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_bot_config_tenant ON bot_config(tenant_id);

-- 5. Chaves naturais por telefone viram compostas com o tenant.
ALTER TABLE conversations          DROP CONSTRAINT IF EXISTS conversations_contact_phone_key;
ALTER TABLE conversation_flow_state DROP CONSTRAINT IF EXISTS conversation_flow_state_pkey;
ALTER TABLE opt_outs               DROP CONSTRAINT IF EXISTS opt_outs_pkey;

CREATE UNIQUE INDEX IF NOT EXISTS uq_conversations_tenant_phone ON conversations(tenant_id, contact_phone);
CREATE UNIQUE INDEX IF NOT EXISTS uq_flowstate_tenant_phone     ON conversation_flow_state(tenant_id, phone);
CREATE UNIQUE INDEX IF NOT EXISTS uq_optouts_tenant_phone       ON opt_outs(tenant_id, phone);
