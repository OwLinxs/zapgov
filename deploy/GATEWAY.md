# Gateway WhatsApp ↔ SIGE (relay bidirecional)

Especificação do lado **ZapGov** da integração com o sistema de chamados (SIGE).
O ZapGov é um **gateway de WhatsApp**: cuida de enviar/receber, mídia, janela de
24h e status de entrega. O **SIGE** é dono do chamado e do chat — o técnico
conversa com a pessoa dentro da tela do chamado. As mensagens vão e voltam por API.

> Status: **desenho** (nenhum código escrito ainda). O lado SIGE já foi
> implementado segundo o contrato em `deploy/GATEWAY.md`/`SIGE-TI.md` anteriores.
> Esta spec é o par no ZapGov. Ao implementar, atualizar esta seção.

## Visão geral

```
Pessoa → WhatsApp → [ZapGov: bot cru] → "Abrir" ou "Retomar"
  Abrir   → coleta mínima → POST chamado.abrir → SIGE cria → ticket_id → RELAY
  Retomar → GET /v1/chamados?telefone → lista → escolhe → confirma → RELAY

Em RELAY(ticket_id):
  pessoa manda no WhatsApp → ZapGov → webhook mensagem.recebida → chat do chamado
  técnico digita no chat    → SIGE  → POST /v1/mensagens        → WhatsApp da pessoa
```

## Máquina de estado da conversa (ZapGov)

Uma conversa (telefone, dentro de um tenant) tem um passo do gateway:

- `menu` — mostrou/mostra o menu Abrir/Retomar.
- `abrindo` — pediu a descrição do problema, aguardando o texto.
- `listando` — mostrou a lista de chamados (retomar), aguardando a escolha.
- `relay(ticket_id)` — vinculada a um chamado; tudo é repassado ao SIGE.

Transições:

```
(1ª msg / sem relay)              → menu
menu + "Abrir"                    → abrindo
abrindo + <texto do problema>     → POST chamado.abrir → relay(novo)   [+ confirmação]
menu + "Retomar"                  → GET /v1/chamados
      lista vazia                 → abrindo  (avisa que vai abrir novo)
      lista com itens             → listando
listando + escolhe #N             → relay(N)  [+ confirmação; reabre se fechado]
relay + (SIGE POST /v1/relay/encerrar) → menu   (volta ao início)
```

Reabertura: chamado fechado é listável e escolhível. A **primeira `mensagem.recebida`**
num chamado fechado faz o SIGE reabrir automaticamente (comportamento já implementado
no SIGE) — o ZapGov não precisa de chamada extra para reabrir.

## Configuração (.env do ZapGov)

Um SIGE por instância (o T.I.). Correlação com o `.env` do SIGE entre parênteses.

```
GATEWAY_ENABLED=true
GATEWAY_TENANT_ID=2                 # tenant cujo número usa o gateway (T.I.)
GATEWAY_INBOUND_API_KEY=<segredo>   # SIGE manda no X-API-Key ao chamar /v1/*  (= ZAPGOV_API_KEY no SIGE)
SIGE_BASE_URL=https://sige.exemplo.gov.br   # ZapGov chama aqui               (= ZAPGOV_BASE_URL aponta pra cá; este aponta pro SIGE)
SIGE_API_KEY=<segredo>              # X-API-Key que o ZapGov envia ao SIGE     (= INTEGRACAO_API_KEY no SIGE)
SIGE_HMAC_SECRET=<segredo>          # assina os webhooks ZapGov→SIGE           (= INTEGRACAO_HMAC_SECRET no SIGE)
GATEWAY_MEDIA_TTL_MIN=30            # validade (min) da URL assinada de mídia recebida
```

> Decisão registrada: config por **.env** (um órgão). Se outro órgão precisar,
> migrar para config por tenant no banco (criptografada, padrão de `tenants/secrets.go`).

## Endpoints que o ZapGov EXPÕE (o SIGE chama)

Autenticação: header `X-API-Key: <GATEWAY_INBOUND_API_KEY>`. 401 se não bater.
Prefixo `/v1` fora do middleware de JWT (é integração máquina-a-máquina).

### POST /v1/mensagens
Envia uma mensagem no WhatsApp da pessoa.

```jsonc
// texto
{ "telefone": "5546999990000", "tipo": "texto", "texto": "Olá, sou o técnico..." }
// mídia (Fase 3)
{ "telefone": "5546999990000", "tipo": "midia",
  "midia_url": "https://sige.../arquivo.pdf", "midia_mime": "application/pdf" }
// template (janela fechada / retomada)
{ "telefone": "5546999990000", "tipo": "template",
  "template": "retomada_atendimento", "params": ["João", "OS-2026-0123"] }
```

Respostas:
- `200 { "wa_message_id": "wamid.XXX" }` — enfileirado/enviado à Meta.
- `401` — chave inválida.
- `422 { "erro": "janela_fechada" }` — texto/mídia fora da janela de 24h. O SIGE
  deve reenviar como `template`.
- `422 { "erro": "dados_invalidos" }` — payload inválido.

### POST /v1/relay/encerrar
Tira o telefone do modo relay (chamado encerrado no SIGE). Próxima mensagem da
pessoa volta ao menu do bot.

```json
{ "telefone": "5546999990000" }
```
Resposta: `200 { "ok": true }` (idempotente; ok mesmo se já não estava em relay).

### GET /v1/midia/{token}  (Fase 3)
Serve a mídia recebida da pessoa, atrás de token assinado e expirável
(`GATEWAY_MEDIA_TTL_MIN`). É a URL colocada em `mensagem.recebida.midia_url`.

## Webhooks que o ZapGov ENVIA (o SIGE recebe)

Destino: `POST {SIGE_BASE_URL}/integracao/zapgov/eventos`.
Cabeçalhos:
- `X-API-Key: <SIGE_API_KEY>`
- `X-Signature: sha256=<hex(HMAC_SHA256(SIGE_HMAC_SECRET, corpo_cru))>`

Entrega confiável: reusar o padrão de `webhook_inbox` (fila + retry com backoff).
O SIGE faz dedup por `wa_message_id`.

### evento: chamado.abrir
```json
{
  "evento": "chamado.abrir",
  "telefone": "5546999990000",
  "nome": "João da Silva",
  "referencia": "wa-<uuid>",
  "dados": { "descricao": "impressora não liga" },
  "janela_24h": { "aberta": true, "expira_em": "2026-09-29T12:00:00Z" }
}
```
Resposta esperada: `200 { "ticket_id": 123, "numero": "OS-2026-0123" }`.
`referencia` é estável por abertura (idempotência: reenvio devolve o mesmo ticket).

### evento: mensagem.recebida
```json
{
  "evento": "mensagem.recebida",
  "ticket_id": 123,
  "telefone": "5546999990000",
  "wa_message_id": "wamid.XXX",
  "tipo": "texto",              // texto|imagem|audio|documento|video
  "texto": "cheguei na sala",   // texto ou legenda da mídia
  "midia_url": null,            // preenchido quando mídia (URL assinada, Fase 3)
  "midia_mime": null,
  "timestamp": "2026-09-29T12:00:00Z",
  "janela_24h": { "aberta": true, "expira_em": "2026-09-29T12:00:00Z" }
}
```

### evento: mensagem.status
```json
{ "evento": "mensagem.status", "wa_message_id": "wamid.YYY",
  "status": "enviado", "erro": "" }   // enviado|entregue|lido|falhou
```
`wa_message_id` casa com o devolvido em `POST /v1/mensagens`.

## Endpoints que o ZapGov CONSOME (o SIGE expõe)

### GET /v1/chamados?telefone=<E164>&limite=10
Cabeçalho: `X-API-Key: <SIGE_API_KEY>`.
```json
{ "chamados": [
  {"id":123,"numero":"OS-2026-0123","resumo":"Impressora não liga",
   "status":"aberto","fechado":false,"atualizado_em":"2026-09-29T12:00:00Z"},
  {"id":118,"numero":"OS-2026-0118","resumo":"Sem internet",
   "status":"resolvido","fechado":true,"atualizado_em":"2026-09-20T09:00:00Z"}
] }
```
O ZapGov transforma em lista interativa do WhatsApp (até 10 linhas). Rótulo por
linha: `#numero · resumo` (resumo cortado ~24 chars). Ordena abertos primeiro.

## Roteamento do inbound (onde plugar no ZapGov)

Hoje `svc.OnInbound = flowSvc.HandleInbound` (motor de fluxo). Para o tenant do
gateway (`GATEWAY_TENANT_ID`), o inbound passa a ser decidido pelo gateway:

```
inbound(tenant == GATEWAY_TENANT_ID) →
   relay ativo p/ o telefone? → repassa como mensagem.recebida (não roda bot)
   senão                       → máquina de estado do menu (abrir/retomar)
outros tenants → fluxo normal (flowSvc)
```

Implementação sugerida: pacote `gateway` com um handler próprio (espelha o padrão
de `techbot`), um dispatcher que escolhe gateway vs fluxo por tenant, e reuso de
`messaging.Service` para enviar (texto/mídia/template) e da infra de mídia do inbox.

## Estado (migração nova)

```sql
-- Sessão do gateway por telefone: passo do menu + chamado em relay.
CREATE TABLE gateway_sessions (
    tenant_id  BIGINT NOT NULL,
    telefone   TEXT   NOT NULL,
    step       TEXT   NOT NULL DEFAULT 'menu',  -- menu|abrindo|listando|relay
    ticket_id  BIGINT,                          -- chamado em relay
    referencia TEXT   NOT NULL DEFAULT '',       -- wa-<uuid> da abertura em curso
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, telefone)
);
```

## Mídia (Fase 3)

- **Recebida** (pessoa → técnico): o ZapGov já baixa a mídia da Meta no inbox.
  O gateway gera uma **URL assinada e expirável** (`GET /v1/midia/{token}`) e a
  coloca em `mensagem.recebida.midia_url`. O SIGE baixa/rearmazena se quiser manter.
- **Enviada** (técnico → pessoa): `POST /v1/mensagens tipo=midia` com `midia_url`
  do lado do SIGE → o ZapGov baixa dessa URL, sobe pra Meta e envia.

## Janela de 24h

- Todo webhook leva `janela_24h { aberta, expira_em }` (de `window_expires_at` da conversa).
- `POST /v1/mensagens` texto/mídia fora da janela → `422 janela_fechada`. O SIGE
  então usa `tipo=template` (template de retomada pré-aprovado na Meta).

## Segurança

- Entrada (`/v1/*`): `X-API-Key` == `GATEWAY_INBOUND_API_KEY`.
- Saída (webhooks): `X-API-Key` = `SIGE_API_KEY` + assinatura `X-Signature` HMAC-SHA256.
- Sem seguir redirects nas chamadas ao SIGE. Timeout curto (15s) e limite de corpo (1 MiB).
- `GET /v1/chamados` e `POST /v1/mensagens` só do tenant do gateway.

## A REMOVER (substituído por este gateway)

O bloco de fluxo "Chamar API" (abertura one-way, sem chat de volta) sai:
- `backend/internal/flow/api.go`, `api_test.go`, `credentials.go`, `credentials_test.go`
- WIP de `janela_24h` em `flow/engine.go` e `flow/repository.go` (WindowInfo)
- `deploy/SIGE-TI.md`
- Nó "Chamar API" no editor: `web/components/flow/FlowCanvas.tsx`, `nodes.tsx`
- Restos de config `SIGE_TI_*` (se houver)
- `graph.go`: struct/handle do bloco `api`

## Fases de construção (ZapGov)

1. **Texto ponta a ponta**: `POST /v1/mensagens` (texto) + `GATEWAY_INBOUND_API_KEY`;
   estado de relay; bot menu Abrir/Retomar; `GET /v1/chamados`; webhooks
   `chamado.abrir` e `mensagem.recebida` (texto); confirmação. → chat texto funciona.
2. **Confiabilidade**: fila/retry dos webhooks (padrão `webhook_inbox`); `mensagem.status`.
3. **Mídia**: recebida (URL assinada) + enviada (upload à Meta).
4. **Janela/template**: `422 janela_fechada` + `POST /v1/relay/encerrar`.

## Interop a confirmar com o SIGE (evita bug de integração)

1. **Formato do X-Signature**: `sha256=<hex>` do corpo cru? (confirmar hex vs base64 e o prefixo).
2. **Formato do telefone**: E.164 sem `+` (`5546999990000`) nos dois lados?
3. **Nomes de template** de retomada e a ordem dos `params`.
4. **`GET /v1/chamados`**: inclui fechados (para reabrir) — confirmado. Limite/paginação se > 10.
5. **Idempotência de `chamado.abrir`**: mesma `referencia` devolve o mesmo `ticket_id` — confirmado.
```
