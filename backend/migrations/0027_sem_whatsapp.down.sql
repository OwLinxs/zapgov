UPDATE contacts SET status = 'ativo' WHERE status = 'sem_whatsapp';
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_status_check;
ALTER TABLE contacts ADD CONSTRAINT contacts_status_check
    CHECK (status IN ('ativo','bloqueado'));
