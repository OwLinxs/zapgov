-- Horário comercial: auto-resposta fora do expediente em dia útil.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS horario_ativo  BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS horario_inicio TEXT NOT NULL DEFAULT '08:00';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS horario_fim    TEXT NOT NULL DEFAULT '18:00';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS horario_dias   TEXT NOT NULL DEFAULT '1,2,3,4,5'; -- 0=Dom..6=Sáb
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS horario_msg    TEXT NOT NULL DEFAULT 'Estamos fora do horário de atendimento. Retornaremos no próximo horário comercial. 🙂';
