#!/usr/bin/env bash
# Primeira instalação do ZapGov no servidor interno.
# Backend via Docker; front via Node/systemd. Rode na RAIZ do projeto.
#   sudo ./deploy/install.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# 1. Pré-checagens dos arquivos de segredo (NÃO versionados).
[ -f .env ] || { echo "ERRO: crie .env na raiz (DB_PASSWORD, APP_ORIGIN, API_URL). Veja deploy/README.md"; exit 1; }
[ -f backend/.env ] || { echo "ERRO: crie backend/.env (JWT_SECRET, WHATSAPP_*...). Veja deploy/README.md"; exit 1; }
# shellcheck disable=SC1091
source .env
: "${API_URL:?defina API_URL no .env (ex.: https://api.SEU-DOMINIO)}"
case "$API_URL" in
  https://*|http://*) ;;
  *) echo "ERRO: API_URL inválida ('$API_URL') — precisa começar com https:// (confira o ':')"; exit 1 ;;
esac

echo "==> Subindo backend (Docker) + migrações"
docker compose -f docker-compose.prod.yml up -d --build

echo "==> Build do front (Next.js) com NEXT_PUBLIC_API_URL=$API_URL"
cd web
npm ci
NEXT_PUBLIC_API_URL="$API_URL" npm run build
cd "$ROOT"

echo "==> Instalando serviço systemd do front"
sed "s#/opt/zapgov/web#$ROOT/web#" deploy/zapgov-web.service | sudo tee /etc/systemd/system/zapgov-web.service >/dev/null
sudo systemctl daemon-reload
sudo systemctl enable --now zapgov-web

echo "==> Pronto."
echo "    Backend:  docker compose -f docker-compose.prod.yml ps"
echo "    Front:    systemctl status zapgov-web"
