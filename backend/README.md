# ZapGov — Backend (fatia de mensageria)

Primeira fatia vertical: **enviar e receber mensagens** via WhatsApp Business
(Meta Cloud API), ponta a ponta. Prova o caminho de maior risco antes de
construir multi-tenant, fila e o resto.

> Escopo intencionalmente enxuto. Adiados (entram depois, sem retrabalhar o provider):
> multi-tenant por schema, Auth/JWT/RBAC, Asynq/fila, contacts/templates/campaigns/metrics.

## Arquitetura
```
cmd/server/main.go            → wiring
internal/config/              → carrega .env
internal/database/            → pool pgx v5
internal/messaging/
  provider.go                 → interface MessageProvider (canal plugável)
  whatsapp/whatsapp.go        → implementação Meta Cloud API (Send + ParseWebhook + HMAC)
  repository.go               → queries pgx (conversations, messages)
  service.go                  → regra de negócio
  handler.go                  → rotas HTTP
internal/webhook/handler.go   → callbacks da Meta (verify + receive)
migrations/                   → SQL (golang-migrate)
```
Separação handler (HTTP) ↔ service (negócio) ↔ repository (banco). `MessageProvider`
é interface pequena: trocar/somar canal (SMS/Email) não toca o service.

## Endpoints
```
POST /messages/send   { "to": "5516998123344", "body": "Olá" }  → envia (síncrono)
GET  /messages        → histórico recente (conferência)
GET  /webhook         → handshake de verificação da Meta
POST /webhook         → eventos (HMAC-SHA256, idempotente, sempre 200)
GET  /health          → healthcheck
```

## Rodar

### 1. Tudo via Docker (recomendado)
```bash
cp .env.example .env      # preencha as credenciais da Meta
docker compose up --build # sobe postgres + aplica migrations + sobe a API
```

### 2. Local
```bash
cp .env.example .env
docker compose up -d postgres
make migrate-up           # requer golang-migrate instalado
make run
```

## Testar de verdade (precisa de credencial Meta)
1. Preencha no `.env`: `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_TOKEN`,
   `WHATSAPP_APP_SECRET`, `WEBHOOK_VERIFY_TOKEN`.
2. Exponha o webhook publicamente (dev): `ngrok http 8080`.
3. No painel da Meta → WhatsApp → Configuration → Webhook:
   - Callback URL: `https://<ngrok>/webhook`
   - Verify token: o mesmo `WEBHOOK_VERIFY_TOKEN`
   - Assine o campo `messages`.
4. Enviar:
   ```bash
   curl -X POST localhost:8080/messages/send \
     -H 'Content-Type: application/json' \
     -d '{"to":"5516998123344","body":"Teste ZapGov"}'
   ```
5. Responda pelo WhatsApp → o inbound chega em `POST /webhook` → confira em `GET /messages`.

## Comportamento do webhook
- Valida `X-Hub-Signature-256` (HMAC-SHA256 do corpo cru com o App Secret).
- `messages` (inbound) → cria/atualiza conversa, reabre janela de 24h.
- `statuses` (sent→delivered→read) → atualiza a mensagem por `wa_message_id`.
- Idempotente: `wa_message_id` é UNIQUE; reentregas da Meta não duplicam.
- Sempre responde 200 em erro interno de processamento (a idempotência cobre o reprocesso).
