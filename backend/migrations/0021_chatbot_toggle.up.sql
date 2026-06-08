-- Liga/desliga o chatbot. Desligado: a mensagem do cidadão vai direto pra fila,
-- sem menu/auto-resposta.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS chatbot_ativo BOOLEAN NOT NULL DEFAULT true;
