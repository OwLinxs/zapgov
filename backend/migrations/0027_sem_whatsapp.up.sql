-- Contato sem WhatsApp (erro 131026 da Meta ao enviar): novo status.
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_status_check;
ALTER TABLE contacts ADD CONSTRAINT contacts_status_check
    CHECK (status IN ('ativo','bloqueado','sem_whatsapp'));
