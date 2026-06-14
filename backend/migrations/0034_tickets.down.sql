ALTER TABLE org_settings DROP COLUMN IF EXISTS ti_categorias;
ALTER TABLE org_settings DROP COLUMN IF EXISTS ti_sla_horas;
DROP TABLE IF EXISTS ticket_comments;
DROP TABLE IF EXISTS tickets;
