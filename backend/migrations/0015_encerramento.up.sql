-- Mensagem automática de encerramento ao finalizar um atendimento.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS encerramento_ativo BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS encerramento_msg   TEXT NOT NULL DEFAULT 'Obrigado pelo contato, {{nome}}! Seu atendimento foi encerrado. Se precisar, é só chamar novamente. 👋';
