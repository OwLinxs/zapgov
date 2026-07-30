-- Módulo gateável: "fila" (painel Fila ao Vivo). Gate é no front (a rota de
-- dados é compartilhada com a caixa de entrada). Liga nos tenants existentes.
UPDATE tenants SET modulos = modulos || '{"fila": true}'::jsonb;
