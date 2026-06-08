-- Setor (departamento) do atendente. Vazio = todos os setores (sem restrição).
ALTER TABLE users ADD COLUMN IF NOT EXISTS setor TEXT NOT NULL DEFAULT '';
