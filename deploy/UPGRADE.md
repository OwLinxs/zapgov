# Atualização: autorização, entrega durável e retirada dos chamados

As alterações deste diretório não implantam nada automaticamente por si mesmas.
Faça a atualização em uma janela controlada e confira o backup antes de migrar.

## Antes de subir esta versão

1. Em `backend/.env`, configure `META_CREDENTIALS_KEY` com o resultado de
   `openssl rand -hex 32`. Use a MESMA chave na API e no worker. Guarde-a com
   segurança: o backup do banco, sozinho, não recupera credenciais cifradas.
2. Confira `JWT_SECRET` (não padrão e pelo menos 32 bytes). O Compose fixa
   `APP_ENV=production`, que valida essas configurações ao iniciar.
3. Com o proxy na mesma máquina, não é necessário configurar `API_BIND_IP`: o
   padrão é `127.0.0.1`. Para proxy em outra máquina, informe o IP privado do
   servidor nessa variável na `.env` da raiz.
4. Instalação nova: configure `SEED_ADMIN_PASSWORD` (12–72 bytes) para criar
   `admin@zapgov.local`. Instalações com usuários existentes não recriam o seed.
5. Pare novos disparos e deixe campanhas em andamento terminarem. Pare API e
   worker antigos antes de aplicar as migrações novas, para evitar que continuem
   operando com regras de entrega anteriores.
6. Faça backup do PostgreSQL. Não exclua volumes nem limpe o Redis para atualizar.

## Mudanças de dados e compatibilidade

- 0042/0043 são migrações locais preexistentes: continuam preservadas.
- 0044 adiciona a versão da sessão. Todos os tokens anteriores serão recusados;
  os usuários precisam entrar novamente.
- 0045 cria `webhook_inbox` para persistir eventos antes do HTTP 200.
- 0046 adiciona estados por destinatário, contadores de descadastrados/revisão
  e reserva de lembretes. Campanhas antigas ainda em envio ficam para revisão,
  sem replay automático. Campanhas agendadas mantêm os destinatários pendentes.
- Tarefas antigas sem `recipient_id` não são reenviadas pelo worker novo. Confira
  campanhas antigas em revisão antes de qualquer novo disparo.
- As tabelas de chamados e os dados existentes não são removidos. A rota do painel
  retorna 404, endpoints de chamados/técnicos não são registrados e o chatbot
  antigo encaminha o cidadão para atendimento humano.
- Atendentes sem setor deixam de enxergar todas as conversas quando a restrição
  por setor estiver ligada. Revise esses cadastros.

Credenciais existentes da Meta são cifradas na primeira inicialização com a chave
configurada. Não troque a chave sem um procedimento de recifragem. Para rollback
para uma versão anterior à criptografia, restaure o backup anterior em uma janela
controlada e reconcilie mensagens recebidas após o backup; não basta trocar o binário.

## Conferência depois da atualização

- API `/health`, frontend e worker ativos; logs sem erros de configuração.
- Login, troca obrigatória, bloqueio de usuário e separação por setor.
- Mensagem no número de teste; confira persistência, inbox e resposta.
- Campanha de teste e histórico: enviadas, descadastrados e itens para revisão.
- Lembretes em `revisao` devem ser conferidos, não reenviados automaticamente.

Consultas operacionais (somente leitura):

```sql
SELECT tenant_id, count(*), min(created_at)
FROM webhook_inbox WHERE processed_at IS NULL GROUP BY tenant_id;
SELECT tenant_id, campaign_id, phone, delivery_state, wa_message_id, last_error
FROM campaign_recipients WHERE delivery_state = 'uncertain' ORDER BY id;
SELECT tenant_id, id, phone, status FROM reminders WHERE status = 'revisao';
```

O histórico da inbox contém dados pessoais. Defina a política de retenção e
arquivamento conforme a operação; esta versão não apaga automaticamente eventos.
O sistema não assume garantia de envio exatamente uma vez. Há tratamento explícito
de resultados incertos para evitar reenvios cegos.

## Rollback e limites

O script de atualização faz backup e verifica saúde, mas não restaura o banco
automaticamente: isso poderia descartar atendimentos novos. Rollback de schema e
restauração precisam considerar as mensagens recebidas após a atualização.
Firewall, TLS, retenção de dados e backups externos continuam sendo configuração
da infraestrutura. O armazenamento de JWT no frontend continua em localStorage;
esta alteração não migra a autenticação para cookies HttpOnly.
