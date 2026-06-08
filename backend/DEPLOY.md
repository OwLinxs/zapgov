# Deploy / Produção — checklist de segurança

## HTTPS (obrigatório)
A API não termina TLS sozinha — use um reverse proxy na frente. Exemplo com **Caddy**
(certificado Let's Encrypt automático):

```caddy
api.seu-dominio.gov.br {
    reverse_proxy localhost:8080
}
app.seu-dominio.gov.br {
    reverse_proxy localhost:3000
}
```

Ou **nginx** + certbot. Nunca expor a porta 8080 direto na internet.

## Variáveis obrigatórias em produção (.env)
```
JWT_SECRET=<string longa e aleatória>      # openssl rand -hex 32  — NÃO use o default
WHATSAPP_TOKEN=<token permanente System User>
WHATSAPP_APP_SECRET=<app secret>
WEBHOOK_VERIFY_TOKEN=<aleatório>
CORS_ORIGINS=https://app.seu-dominio.gov.br  # restringe ao domínio do front
DATABASE_URL=postgres://...?sslmode=require  # TLS no banco
```

## Checklist
- [ ] HTTPS no proxy (API + front)
- [ ] `JWT_SECRET` forte (não o default — o app avisa no log se estiver no default)
- [ ] Trocar a senha do admin no 1º acesso (forçado pelo sistema)
- [ ] `CORS_ORIGINS` apenas o domínio do front
- [ ] Postgres com `sslmode=require` e senha forte
- [ ] Webhook da Meta apontando para `https://api.../webhook`
- [ ] Backup automático do Postgres
- [ ] Worker e app rodando (docker compose up -d) com restart policy

## Proteções já no código
- Rate limit no `/auth/login` (10/min por IP) — anti-brute-force
- Senha com bcrypt; JWT HS256 (12h)
- RBAC por papel (admin/supervisor/atendente), back + front
- HMAC-SHA256 no webhook da Meta
- Queries parametrizadas (pgx) — sem SQL injection
