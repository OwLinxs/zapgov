# Gateway WhatsApp ↔ SIGE (relay bidirecional)

Especificação do lado **ZapGov** da integração com o sistema de chamados (SIGE).
O ZapGov é um **gateway de WhatsApp**: cuida de enviar/receber, mídia, janela de
24h e status de entrega. O **SIGE** é dono do chamado e do chat — o técnico
conversa com a pessoa dentro da tela do chamado. As mensagens vão e voltam por API.

> Status: **Fase 1 implementada** (texto ponta a ponta) no pacote
> `backend/internal/gateway`, mais a **fila com retry** do `mensagem.recebida`
> (Fase 2, parte) e a **saída do relay** (palavra-chave + inatividade). O lado
> SIGE já foi implementado segundo este contrato. **Mídia (Fase 3)** implementada
> nos dois sentidos. Pendente: `mensagem.status`.

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
relay + "sair" | "#sair" | "menu"      → menu   (a pessoa sai; o chamado segue no SIGE)
relay parado > GATEWAY_RELAY_TTL_H     → menu   (na próxima mensagem da pessoa)
```

**Saída do relay.** Sem isso a pessoa ficaria presa se o SIGE nunca chamasse
`/v1/relay/encerrar`. A palavra-chave só vale como mensagem inteira ("vou sair
da sala" continua no relay). "Movimento" = mensagem da pessoa repassada ou
resposta do técnico via `POST /v1/mensagens`; ambas renovam `atualizado_em`. A
mensagem que encontra o relay expirado **não** é repassada: a pessoa recebe o
menu com a orientação de usar *Retomar chamado*. O SIGE não é notificado da
saída (não há evento no contrato; a próxima `mensagem.recebida` após um
*Retomar* reabre o vínculo normalmente).

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
GATEWAY_RELAY_TTL_H=72              # horas sem movimento até o relay expirar (0 = nunca)
GATEWAY_PUBLIC_URL=https://api.zapgov.exemplo.gov.br  # URL pública desta API, alcançável pelo SIGE (monta midia_url)
GATEWAY_MEDIA_TTL_MIN=30            # validade (min) da URL assinada de mídia recebida
```

Sem `GATEWAY_PUBLIC_URL` o gateway funciona, mas mídias recebidas chegam ao SIGE
só com `tipo`/`texto`/`midia_mime` e `midia_url: null` (log de aviso na subida).

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
// mídia: midia_url TEM de estar no mesmo esquema+host de SIGE_BASE_URL
{ "telefone": "5546999990000", "tipo": "midia", "texto": "legenda (opcional)",
  "midia_url": "https://sige.../arquivo.pdf", "midia_mime": "application/pdf",
  "midia_nome": "laudo.pdf" }   // midia_mime e midia_nome opcionais
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
- `422 { "erro": "opt_out" }` — a pessoa pediu para não receber mensagens.
- `422 { "erro": "midia_invalida", "detalhe": "..." }` — `midia_url` fora do SIGE
  ou arquivo acima do limite da Meta (imagem 5 MB, áudio/vídeo 16 MB, documento 100 MB).
- `502 { "erro": "midia_indisponivel" }` — não conseguiu baixar do SIGE (pode tentar de novo).
- `502 { "erro": "envio_falhou" }` — a Meta recusou o envio.

### POST /v1/relay/encerrar
Tira o telefone do modo relay (chamado encerrado no SIGE). Próxima mensagem da
pessoa volta ao menu do bot.

```json
{ "telefone": "5546999990000" }
```
Resposta: `200 { "ok": true }` (idempotente; ok mesmo se já não estava em relay).

### GET /v1/midia/{token}
Serve a mídia recebida da pessoa. É a URL colocada em `mensagem.recebida.midia_url`.
**Não** exige `X-API-Key`: a autorização é o próprio token (HMAC, expira em
`GATEWAY_MEDIA_TTL_MIN`). `404` para token inválido/expirado. O SIGE deve
**baixar e guardar** o arquivo ao receber o evento — depois do prazo a URL morre
(e a Meta apaga a mídia em ~30 dias).

## Webhooks que o ZapGov ENVIA (o SIGE recebe)

Destino: `POST {SIGE_BASE_URL}/integracao/zapgov/eventos`.
Cabeçalhos:
- `X-API-Key: <SIGE_API_KEY>`
- `X-Signature: sha256=<hex(HMAC_SHA256(SIGE_HMAC_SECRET, corpo_cru))>`

Entrega confiável do `mensagem.recebida`: o inbound grava em `gateway_outbox`
(migration 0048) e um loop no processo da API entrega a cada 1s:

- **Ordem por telefone**: só o evento pendente mais antigo de cada telefone é
  elegível; se ele falha, os seguintes esperam.
- **Retry**: backoff `30s · 2^n` (teto 64 min), até 10 tentativas (~4h). Depois
  marca `failed_at` e desiste.
- **Aviso à pessoa**: na 1ª falha (no máximo 1 aviso/hora por telefone) recebe
  "sistema de chamados instável, será entregue, não precisa reenviar"; na
  desistência, "não conseguimos entregar, reenvie mais tarde". Se nem gravar na
  fila der (banco fora), pede para reenviar na hora.
- **Dedup**: `UNIQUE (tenant_id, wa_message_id)` na fila; o SIGE também deduplica
  por `wa_message_id`.
- Eventos entregues/desistidos há mais de 30 dias são apagados.
- Diagnóstico: `SELECT telefone, attempts, last_error, next_attempt_at FROM
  gateway_outbox WHERE sent_at IS NULL AND failed_at IS NULL;`

`chamado.abrir` continua síncrono (precisa do `ticket_id` na hora). Em falha a
pessoa é orientada a reenviar e a `referencia` é reutilizada (idempotente).

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

## Mídia (Fase 3) ✅

- **Recebida** (pessoa → técnico): a fila guarda só a referência (media id da
  Meta). A URL assinada é gerada **na hora da entrega** ao SIGE, então não vence
  durante os retries. O token leva tenant, media id, mime, nome e expiração,
  assinado com chave derivada de `SIGE_HMAC_SECRET`. Ao servir, o ZapGov busca a
  mídia na Meta e devolve com `nosniff` + CSP `sandbox`; tipos fora de
  imagem/áudio/vídeo/PDF vão como `application/octet-stream` + `attachment`
  (um `.html` enviado pela pessoa nunca é renderizado no domínio do ZapGov).
- **Enviada** (técnico → pessoa): `POST /v1/mensagens tipo=midia` → o ZapGov
  baixa a `midia_url` (só do host de `SIGE_BASE_URL`, mandando `X-API-Key:
  SIGE_API_KEY`, sem seguir redirect, timeout 60s, limite por tipo), sobe na Meta
  e envia por media id. A URL do SIGE não precisa ser pública. Tipo do envio sai
  do mime (`midia_mime` > `Content-Type` > detecção); nome sai de `midia_nome` >
  `Content-Disposition` > fim do caminho da URL.

## Janela de 24h

- Todo webhook leva `janela_24h { aberta, expira_em }` (de `window_expires_at` da conversa).
- `POST /v1/mensagens` texto/mídia fora da janela → `422 janela_fechada`. O SIGE
  então usa `tipo=template` (template de retomada pré-aprovado na Meta).

## Segurança

- Entrada (`/v1/*`): `X-API-Key` == `GATEWAY_INBOUND_API_KEY`.
- Saída (webhooks): `X-API-Key` = `SIGE_API_KEY` + assinatura `X-Signature` HMAC-SHA256.
- Sem seguir redirects nas chamadas ao SIGE. Timeout curto (15s) e limite de corpo (1 MiB).
- `GET /v1/chamados` e `POST /v1/mensagens` só do tenant do gateway.

## REMOVIDO (substituído por este gateway)

O bloco de fluxo "Chamar API" (abertura one-way, sem chat de volta) foi removido:
- `backend/internal/flow/api.go`, `api_test.go`, `credentials.go`, `credentials_test.go`
- `WindowInfo` e o handle do bloco `api` em `flow/engine.go`/`graph.go`/`repository.go`
- `deploy/SIGE-TI.md`
- Nó "Chamar API" no editor: `web/components/flow/FlowCanvas.tsx`, `nodes.tsx`

Sobra inofensiva: `tenants.SealIntegrationSecret`/`OpenIntegrationSecret` (não
mais usadas; mantidas por serem genéricas). Nós `api` em fluxos antigos passam a
ser ignorados pelo motor (caem no `default`).

## Fases de construção (ZapGov)

1. **Texto ponta a ponta**: `POST /v1/mensagens` (texto) + `GATEWAY_INBOUND_API_KEY`;
   estado de relay; bot menu Abrir/Retomar; `GET /v1/chamados`; webhooks
   `chamado.abrir` e `mensagem.recebida` (texto); confirmação. → chat texto funciona.
2. **Confiabilidade**: fila/retry do `mensagem.recebida` ✅ (`gateway_outbox`);
   saída do relay ✅; `mensagem.status` — pendente.
3. **Mídia** ✅: recebida (URL assinada) + enviada (download do SIGE + upload à Meta).
4. **Janela/template**: `422 janela_fechada` + `POST /v1/relay/encerrar`.

## Interop a confirmar com o SIGE (evita bug de integração)

1. **Formato do X-Signature**: `sha256=<hex>` do corpo cru? (confirmar hex vs base64 e o prefixo).
2. **Formato do telefone**: E.164 sem `+` (`5546999990000`) nos dois lados?
3. **Nomes de template** de retomada e a ordem dos `params`.
4. **`GET /v1/chamados`**: inclui fechados (para reabrir) — confirmado. Limite/paginação se > 10.
5. **Idempotência de `chamado.abrir`**: mesma `referencia` devolve o mesmo `ticket_id` — confirmado.
6. **Arquivos do SIGE no mesmo host de `SIGE_BASE_URL`**: se o SIGE guardar anexos
   em outro domínio (S3/CDN), o envio de mídia dá `422 midia_invalida` — aí é
   preciso liberar esse host explicitamente.
7. **SIGE baixa a mídia recebida na hora** (a `midia_url` expira em `GATEWAY_MEDIA_TTL_MIN`).
```
