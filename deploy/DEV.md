# Ambiente de Desenvolvimento (local)

Objetivo: testar mudanças **sem tocar na produção** da prefeitura. A produção é
sagrada — nunca se edita código nem banco direto nela. O fluxo é:

```
feature/x  →  develop (testa local)  →  main (deploy em produção)
```

## As 3 camadas que ficam separadas

| Camada | Produção | Desenvolvimento |
|---|---|---|
| Código | branch `main` | branch `develop` / `feature/*` |
| Banco / Redis | VM da prefeitura | Docker local (`zapgov_dev`, portas 5433/6380) |
| WhatsApp (Meta) | número oficial | número de **teste** de um app Meta separado |

> ⚠️ **Nunca** aponte o webhook do app Meta de produção para a sua máquina —
> isso desvia as mensagens reais dos cidadãos. Use um **app Meta separado** para
> dev, com o próprio número de teste e o próprio webhook.

## Pré-requisitos
- Docker + Docker Compose
- Go 1.25, Node 20
- (Opcional) `ngrok` ou `cloudflared` para expor o webhook de dev à Meta

## Passo a passo

### 1. Configurar o `.env.dev`
```bash
cp backend/.env.dev.example backend/.env.dev
# edite backend/.env.dev com as credenciais do número de TESTE da Meta
```

### 2. Subir a infra (Postgres + Redis isolados, já migra)
```bash
make dev-up
```

### 3. Rodar a aplicação (3 terminais)
```bash
make api      # backend  → http://localhost:8080
make worker   # fila de disparos (Asynq)
make web      # painel   → http://localhost:3000
```

Hot reload: o front recarrega sozinho; no backend, Ctrl-C e `make api` de novo
(go run recompila em segundos).

### 4. Primeiro acesso
O seed cria o admin inicial (mesma lógica da prod). Veja `deploy/README.md` para
as credenciais padrão e a troca de senha no primeiro login.

### 5. Receber webhooks da Meta no dev (opcional)
Só necessário para testar **mensagens recebidas** (inbound). Para só enviar, não
precisa.
```bash
ngrok http 8080
# cadastre a URL https://xxxx.ngrok.app/webhook no app Meta de DEV
# (Verify Token = WEBHOOK_VERIFY_TOKEN do .env.dev)
```

## Comandos úteis
```bash
make dev-down    # para a infra (mantém dados)
make dev-reset   # ZERA o banco de dev e remigra
make test        # testes do backend
```

## Fluxo de release para produção
```bash
git checkout develop && git pull
# ...desenvolve, testa local...
git checkout main && git merge develop
git push
# na VM da prefeitura:
cd /opt/zapgov/zapgov && sudo bash deploy/update.sh
```

## Por que a infra em Docker e o app fora?
Mesma escolha da produção (front fora do Docker) levada ao dev: Postgres/Redis em
container dão isolamento e descarte fácil (`dev-reset`); backend/front locais dão
recompilação instantânea e debug direto, sem rebuild de imagem a cada mudança.
