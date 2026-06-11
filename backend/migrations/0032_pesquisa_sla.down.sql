ALTER TABLE conversations DROP COLUMN IF EXISTS queued_at;
DROP TABLE IF EXISTS surveys;
ALTER TABLE org_settings DROP COLUMN IF EXISTS sla_minutos;
ALTER TABLE org_settings DROP COLUMN IF EXISTS pesquisa_obrigado;
ALTER TABLE org_settings DROP COLUMN IF EXISTS pesquisa_msg;
ALTER TABLE org_settings DROP COLUMN IF EXISTS pesquisa_ativo;
