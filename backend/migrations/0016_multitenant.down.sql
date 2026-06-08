DROP INDEX IF EXISTS uq_optouts_tenant_phone;
DROP INDEX IF EXISTS uq_flowstate_tenant_phone;
DROP INDEX IF EXISTS uq_conversations_tenant_phone;
DROP INDEX IF EXISTS uq_bot_config_tenant;
DROP INDEX IF EXISTS uq_org_settings_tenant;
DROP INDEX IF EXISTS uq_messages_tenant_waid;
DROP INDEX IF EXISTS uq_contacts_tenant_tel;
DROP INDEX IF EXISTS uq_users_tenant_email;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'audit_log','bot_config','bot_options','campaigns','contacts',
    'conversation_flow_state','conversation_labels','conversations','departments',
    'flows','labels','messages','opt_outs','org_settings','sessions','users','webhook_endpoints'
  ] LOOP
    EXECUTE format('DROP INDEX IF EXISTS idx_%I_tenant', t);
    EXECUTE format('ALTER TABLE %I DROP COLUMN IF EXISTS tenant_id', t);
  END LOOP;
END $$;

DROP INDEX IF EXISTS idx_tenants_phone;
DROP TABLE IF EXISTS tenants;
