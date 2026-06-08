-- Roteamento de atendimento.
-- roteamento_setor: atendente só vê conversas do próprio setor.
-- etiqueta_atendente: ao assumir, aplica automaticamente uma etiqueta com o nome do atendente.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS roteamento_setor   BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS etiqueta_atendente BOOLEAN NOT NULL DEFAULT false;
