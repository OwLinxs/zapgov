# ZapGov — Implantação (servidor interno + proxy/DMZ)

Arquitetura na prefeitura:

```
[Meta / cidadãos]
        │  HTTPS (443)
        ▼
  Proxy na DMZ (nginx)  ──► TLS + roteamento
        │  HTTP interno
        ▼
  Servidor interno
    ├─ web   :3000  (painel Next.js — serviço systemd, FORA do Docker)
    └─ Docker Compose:
        ├─ app   :8080  (API Go + /webhook)
        ├─ worker        (fila de disparos)
        ├─ postgres      (volume pgdata)
        └─ redis
```

- **app :8080** e **web :3000** NÃO são públicos — só o proxy da DMZ os alcança
  (o firewall institucional bloqueia o acesso direto da internet).
- O proxy publica dois hosts:
  - `app.SEU-DOMINIO` → painel
  - `api.SEU-DOMINIO` → API e **webhook** (`https://api.SEU-DOMINIO/webhook`)

---

## Requisitos de rede (pra alinhar/configurar)

1. **Entrada (inbound)** — a Meta precisa alcançar `https://api.SEU-DOMINIO/webhook`
   pela internet → o proxy da DMZ deve estar publicado em 443. (Opcional, mais
   seguro: liberar inbound 443 só das faixas de IP da Meta.)
2. **Saída (outbound)** — o servidor interno precisa alcançar
   `graph.facebook.com:443`. Se houver proxy de saída/whitelist, liberar
   `graph.facebook.com` (e `*.facebook.com`/`*.fbcdn.net` p/ mídia).
3. **DNS** — `app.SEU-DOMINIO` e `api.SEU-DOMINIO` apontando pro IP público do proxy.
4. **TLS** — certificado p/ os dois hosts (Let's Encrypt ou certificado institucional).

---

## Passos no SERVIDOR INTERNO

### A) Backend (Docker)
1. Instalar Docker + Compose plugin.
2. Copiar o projeto pro servidor (ex.: `/opt/zapgov`).
3. Criar **`backend/.env`** (segredos do app) — ver template abaixo.
4. Criar **`.env`** na raiz (variáveis do compose):
   ```
   DB_PASSWORD=<senha forte do banco>
   APP_ORIGIN=https://app.SEU-DOMINIO
   API_URL=https://api.SEU-DOMINIO
   ```
   Com Nginx na mesma máquina, o backend usa `127.0.0.1` por padrão. Somente se o proxy estiver em outra máquina, adicione `API_BIND_IP=<IP privado do servidor>` ao `.env`.
5. Subir:
   ```bash
   docker compose -f docker-compose.prod.yml up -d --build
   ```
   (as migrações rodam sozinhas no serviço `migrate`.)
6. Conferir: `docker compose -f docker-compose.prod.yml ps` e `... logs -f app`.

### B) Front (Node + systemd, fora do Docker)
1. Instalar **Node 20** (ex.: nodesource).
2. Na pasta `web/`, build com a URL pública da API (assada no bundle):
   ```bash
   cd /opt/zapgov/web
   npm ci
   NEXT_PUBLIC_API_URL=https://api.SEU-DOMINIO npm run build
   ```
3. Instalar o serviço:
   ```bash
   sudo cp /opt/zapgov/deploy/zapgov-web.service /etc/systemd/system/
   # ajuste WorkingDirectory (/opt/zapgov/web) e User no arquivo, se preciso
   sudo systemctl daemon-reload
   sudo systemctl enable --now zapgov-web
   sudo systemctl status zapgov-web    # deve estar active, ouvindo :3000
   ```
> Ao atualizar o front: `git pull && cd web && npm ci && NEXT_PUBLIC_API_URL=... npm run build && sudo systemctl restart zapgov-web`.

### Template `backend/.env`
```
# Banco (host/senha são injetados pelo compose; não precisa DATABASE_URL aqui)
JWT_SECRET=<openssl rand -hex 32>
META_CREDENTIALS_KEY=<outro openssl rand -hex 32; manter backup>
SEED_ADMIN_PASSWORD=<senha inicial de 12 a 72 bytes, para instalação nova>
APP_ENV=production
LOG_LEVEL=info

# WhatsApp / Meta (do tenant da plataforma; demais órgãos no Super Admin)
WHATSAPP_PHONE_NUMBER_ID=
WHATSAPP_WABA_ID=
WHATSAPP_TOKEN=
WHATSAPP_APP_SECRET=
WHATSAPP_API_VERSION=v21.0
WEBHOOK_VERIFY_TOKEN=<string aleatória>

# Opcional
SENTRY_DSN=
```
> `CORS_ORIGINS` é definido pelo compose (= `APP_ORIGIN`). Não precisa aqui.

---

## Passos no PROXY (DMZ)

1. Instalar nginx.
2. Copiar `deploy/nginx-dmz.conf` e trocar: `INTERNAL_IP`, `app.SEU-DOMINIO`,
   `api.SEU-DOMINIO`, e os caminhos do certificado.
3. Instalar o certificado TLS (Let's Encrypt: `certbot --nginx`).
4. `nginx -t && systemctl reload nginx`.

---

## Configurar a Meta (uma vez só)

App Dashboard → WhatsApp → Configuration → Webhook:
- **Callback URL:** `https://api.SEU-DOMINIO/webhook`
- **Verify token:** o mesmo `WEBHOOK_VERIFY_TOKEN` do `backend/.env`
- Assinar o campo **messages**.

---

## Operação

- **Logs backend:** `docker compose -f docker-compose.prod.yml logs -f app worker`
- **Logs front:** `journalctl -u zapgov-web -f`
- **Atualizar backend:** `git pull && docker compose -f docker-compose.prod.yml up -d --build`
- **Atualizar front:** `git pull && cd web && npm ci && NEXT_PUBLIC_API_URL=https://api.SEU-DOMINIO npm run build && sudo systemctl restart zapgov-web`
- **Backup do banco (cron diário):**
  ```bash
  docker compose -f docker-compose.prod.yml exec -T postgres \
    pg_dump -U zapgov zapgov | gzip > /backup/zapgov-$(date +%F).sql.gz
  ```
- **Restaurar:**
  ```bash
  gunzip -c /backup/zapgov-AAAA-MM-DD.sql.gz | \
    docker compose -f docker-compose.prod.yml exec -T postgres psql -U zapgov zapgov
  ```

## Checklist de segurança (produção)
- [ ] `JWT_SECRET` forte (não o default de dev).
- [ ] `DB_PASSWORD` forte.
- [ ] Portas 8080/3000 só acessíveis pelo proxy (firewall).
- [ ] TLS válido nos dois hosts.
- [ ] Backup diário testado (restauração).

## Atualização de instalações existentes

Leia [UPGRADE.md](UPGRADE.md) antes de implantar esta versão. O administrador
inicial usa `admin@zapgov.local` e a senha configurada em `SEED_ADMIN_PASSWORD`;
não há senha padrão. Redis usa volume persistente e AOF.
