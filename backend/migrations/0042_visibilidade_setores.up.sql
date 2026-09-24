-- Grupos de visibilidade de setores (por tenant): setores no mesmo grupo se
-- enxergam. Formato: array de grupos, cada grupo = array de nomes de setor.
-- Ex.: [["Agendamento","Saúde Mental","MAC","Recepção"]]
-- Setor fora de qualquer grupo = isolado (atendente vê só o próprio).
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS visibilidade_setores jsonb NOT NULL DEFAULT '[]'::jsonb;
