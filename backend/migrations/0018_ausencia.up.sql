-- Auto-resposta de ausência (fim de semana / feriado).
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ausencia_ativo       BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ausencia_fim_semana  TEXT NOT NULL DEFAULT 'Olá! Nosso atendimento não funciona aos finais de semana. Retornaremos no próximo dia útil. 🙂';
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ausencia_feriado     TEXT NOT NULL DEFAULT 'Olá! Hoje é feriado e não há atendimento. Retornaremos no próximo dia útil. 🙂';
-- Datas de feriado: 'YYYY-MM-DD' (data específica) ou 'MM-DD' (recorrente todo ano).
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ausencia_feriados    TEXT[] NOT NULL DEFAULT '{}';

-- Throttle: quando a auto-resposta de ausência foi enviada por última vez.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS last_ausencia_at TIMESTAMPTZ;
