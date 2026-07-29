-- ============================================================
-- Módulos por tenant: liga/desliga funcionalidades por painel.
-- Chave ausente/false = módulo escondido no menu E bloqueado na API.
-- Módulos gated hoje: chamados, base, disparo. O resto é sempre visível.
-- ============================================================
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS modulos jsonb NOT NULL DEFAULT '{}'::jsonb;

-- Seed dos tenants já existentes (só onde ainda está vazio):
--  - tenant 1 (Prefeitura/Saúde): mantém disparo; SEM módulos de T.I.
--  - demais (T.I.): chamados + base + disparo ligados.
UPDATE tenants SET modulos = '{"disparo": true}'::jsonb
 WHERE id = 1 AND modulos = '{}'::jsonb;
UPDATE tenants SET modulos = '{"chamados": true, "base": true, "disparo": true}'::jsonb
 WHERE id <> 1 AND modulos = '{}'::jsonb;
