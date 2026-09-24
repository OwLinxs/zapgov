# ZapGov

Plataforma de atendimento público via WhatsApp Business (Meta Cloud API), com
órgãos independentes, atendimento humano, chatbot visual, campanhas, lembretes,
relatórios, auditoria e base de conhecimento.

O módulo de chamados está desativado: não há menu, endpoints de tickets/técnicos
ou integração com o inbox. Código e tabelas históricos foram preservados para
uma eventual retomada. Ações antigas `abrir_chamado` encaminham para atendimento
humano, preservando as variáveis coletadas.

## Arquitetura

- `../web`: Next.js 14, React 18 e TypeScript; consome a API por HTTP/JWT.
- `cmd/server`: API Fiber, webhook, processamento da inbox persistente,
  agendador de campanhas/lembretes e SSE.
- `cmd/worker`: tarefas Asynq de campanhas; lê destinatários do PostgreSQL.
- `internal/*`: módulos com handlers, serviços e repositórios pgx.
- PostgreSQL 16: dados e estado durável de entrega.
- Redis 7: fila Asynq e distribuição de eventos entre processos.
- `migrations`: SQL versionado (golang-migrate); executar antes dos binários.

O isolamento é por `tenant_id` em tabelas compartilhadas, com filtros na aplicação.
Não usa schema por órgão nem políticas PostgreSQL RLS. As rotas de mensageria
aplicam também a visibilidade por setor/grupo; atendente sem setor não recebe
acesso irrestrito quando o roteamento por setor está habilitado.

## Mensagens e entrega

1. O webhook separa os eventos por número, resolve o órgão e valida o HMAC do
   corpo original. Números desconhecidos são recusados.
2. O lote validado é persistido atomicamente em `webhook_inbox`. Só então recebe
   HTTP 200. Falha de persistência retorna 503.
3. A API processa a inbox com locks e tentativas adicionais. A mensagem recebida
   e a atualização da conversa são atômicas. `wa_message_id` impede duplicatas.
4. Campanha e destinatários são persistidos na mesma transação. O agendador
   recupera tarefas pendentes mesmo após indisponibilidade/perda de fila Redis.
5. Cada destinatário é reservado atomicamente antes do envio. Opt-out fica
   `skipped`. HTTP 429 explícito permite retry; recusa 4xx fica `failed`.
6. Timeout, 5xx, interrupção ou falha de persistência após chamar a Meta podem
   representar envio aceito. Esses itens ficam `uncertain`, visíveis no painel
   como **Conferir envio**, sem reenvio automático.

Não há garantia de entrega exatamente uma vez entre PostgreSQL e uma API externa.
Confira o histórico/Meta antes de reenviar um item incerto. O histórico recebido
é durável; efeitos externos do chatbot não são uma transação única com esse
histórico. Falhas de resposta automática exigem acompanhamento operacional.

SSE usa Redis Pub/Sub e revalida sessão e acesso ao setor. Polling do frontend
recupera eventos perdidos durante desconexões. Pub/Sub não é armazenamento.

## Autenticação e segredos

- bcrypt; JWT HS256 com expiração de 12h.
- Cada requisição verifica usuário/órgão ativos e permissões atuais no banco.
- Troca/reset de senha incrementa `session_version`, revogando tokens antigos.
- Troca obrigatória permite apenas `/auth/me` e `/auth/password` até concluir.
- Primeiro administrador: `SEED_ADMIN_PASSWORD` explícita, 12–72 bytes; login
  `admin@zapgov.local`. Não existe senha inicial fixa nem senha escrita em log.
- Produção exige `JWT_SECRET` não padrão, com pelo menos 32 bytes, e
  `META_CREDENTIALS_KEY` com 64 caracteres hexadecimais (`openssl rand -hex 32`).
- Tokens/segredos Meta cadastrados no banco são cifrados com AES-256-GCM,
  vinculados ao órgão/campo. Valores antigos são cifrados na inicialização
  quando a chave está configurada. Guarde a chave fora do banco e com backup.
- Somente o órgão 1 pode usar o provider do ambiente. Outros órgãos sem
  credenciais próprias falham de forma explícita; não usam o número da plataforma.

## Desenvolvimento

Veja [ambiente local](../deploy/DEV.md). Na raiz do projeto:

```sh
cp backend/.env.dev.example backend/.env.dev
# Configure SEED_ADMIN_PASSWORD e, para cadastrar credenciais, META_CREDENTIALS_KEY.
make dev-up
make api
# Em outros terminais:
make worker
make web
```

A infraestrutura local usa PostgreSQL 5433 e Redis 6380. Use um aplicativo/número
Meta de teste. Não aponte o ambiente local para recursos de produção.

## Validação

```sh
# Em backend/
go test ./...
go vet ./...
# Integração: banco EXCLUSIVO com nome iniciado por zapgov_test.
TEST_DATABASE_URL='postgres://.../zapgov_test?sslmode=disable' go test ./...
# Em web/
npm ci
npx tsc --noEmit --incremental false
npm run build
```

Sem `TEST_DATABASE_URL`, testes de integração são pulados explicitamente. Com a
variável, cada teste cria um schema isolado, aplica todas as migrações e remove
apenas esse schema ao terminar. Nunca utilize credenciais de produção.

Os testes cobrem limites de setor por HTTP, sessão/revogação, criptografia,
concorrência da reserva de destinatários, recuperação da fila e inbox do webhook.
A pipeline em `.github/workflows/ci.yml` executa esses testes com PostgreSQL isolado.

## Operação

Leia [implantação](../deploy/README.md) e [atualização desta versão](../deploy/UPGRADE.md).
O frontend é um serviço Node/systemd; o backend, worker, banco e Redis rodam no
Compose. TLS é terminado no nginx da DMZ. Os dados de PostgreSQL e Redis têm
volumes persistentes no Compose de produção.

`/health` verifica PostgreSQL; não substitui a supervisão do worker, Redis ou da
idade dos itens pendentes. A auditoria registra mutações e falhas autenticadas;
erros de gravação aparecem no log. Não é um journal transacional de cada efeito.

## Integração do bot com SIGE-TI

O editor oferece o bloco **Chamar API**, com variáveis no corpo, retorno mapeado e saídas de sucesso/erro. Configuração e exemplo de fluxo em [deploy/SIGE-TI.md](../deploy/SIGE-TI.md).
