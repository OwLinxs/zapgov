-- Novo módulo gateável: "chatbot" (editor de fluxo + config do bot).
-- Liga em todos os tenants existentes (todos já tinham o editor) sem mexer nos
-- demais módulos — jsonb '||' faz merge da chave.
UPDATE tenants SET modulos = modulos || '{"chatbot": true}'::jsonb;
