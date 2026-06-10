DROP TABLE IF EXISTS reminders;
ALTER TABLE org_settings DROP COLUMN IF EXISTS lembrete_hora;
ALTER TABLE org_settings DROP COLUMN IF EXISTS lembrete_idioma;
ALTER TABLE org_settings DROP COLUMN IF EXISTS lembrete_template;
ALTER TABLE org_settings DROP COLUMN IF EXISTS lembrete_ativo;
