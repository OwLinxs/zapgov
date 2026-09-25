#!/usr/bin/env bash
# Atualização do ZapGov (puxa do Git, faz backup, sobe back, rebuild front).
# Rode na RAIZ do projeto:  ./deploy/update.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck disable=SC1091
source .env
: "${API_URL:?defina API_URL no .env}"
case "$API_URL" in
  https://*|http://*) ;;
  *) echo "ERRO: API_URL inválida ('$API_URL') — precisa começar com https:// (confira o ':')"; exit 1 ;;
esac

BACKUP_DIR="${BACKUP_DIR:-/backup}"
mkdir -p "$BACKUP_DIR"

echo "==> Backup do banco antes de atualizar"
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U zapgov zapgov | gzip > "$BACKUP_DIR/zapgov-$(date +%F_%H%M).sql.gz"

echo "==> git pull"
# Com sudo, o git rodaria como root (sem chave SSH) — puxa como o usuário real.
if [ -n "${SUDO_USER:-}" ]; then
  sudo -u "$SUDO_USER" git pull --ff-only
else
  git pull --ff-only
fi

echo "==> Validando configuração e compilando backend"
docker compose -f docker-compose.prod.yml config --quiet
docker compose -f docker-compose.prod.yml build app worker

echo "==> Backend: migrações + reinício"
docker compose -f docker-compose.prod.yml up -d

echo "==> Front: build + restart"
cd web
# Build como o usuário real (root deixaria .next com dono errado → EACCES depois).
if [ -n "${SUDO_USER:-}" ]; then
  chown -R "$SUDO_USER" .
  sudo -u "$SUDO_USER" npm ci
  sudo -u "$SUDO_USER" env NEXT_PUBLIC_API_URL="$API_URL" npm run build
else
  npm ci
  NEXT_PUBLIC_API_URL="$API_URL" npm run build
fi
cd "$ROOT"
sudo systemctl restart zapgov-web

echo "==> Conferindo saúde da API e serviço do painel"
API_HEALTH="http://${API_BIND_IP:-127.0.0.1}:8080/health"
healthy=false
for attempt in {1..30}; do
  if curl --noproxy "*" --fail --silent --max-time 3 "$API_HEALTH" >/dev/null; then healthy=true; break; fi
  sleep 2
done
if [ "$healthy" != true ]; then
  echo "ERRO: API não respondeu em $API_HEALTH. Preserve o backup."
  curl --noproxy "*" --fail --show-error --max-time 3 "$API_HEALTH" || true
  docker compose -f docker-compose.prod.yml ps -a
  echo "Confira os logs: docker compose -f docker-compose.prod.yml logs --tail=100 app worker migrate"
  exit 1
fi
sudo systemctl is-active --quiet zapgov-web
curl --fail --silent --max-time 10 http://127.0.0.1:3000/login >/dev/null

echo "==> Atualizado. Versão: $(git describe --tags --always)"
