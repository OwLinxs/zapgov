ALTER TABLE conversations DROP COLUMN IF EXISTS last_ausencia_at;
ALTER TABLE org_settings DROP COLUMN IF EXISTS ausencia_feriados;
ALTER TABLE org_settings DROP COLUMN IF EXISTS ausencia_feriado;
ALTER TABLE org_settings DROP COLUMN IF EXISTS ausencia_fim_semana;
ALTER TABLE org_settings DROP COLUMN IF EXISTS ausencia_ativo;
