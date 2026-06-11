#!/usr/bin/env bash
# Atualização do ZapGov (puxa do Git, faz backup, sobe back, rebuild front).
# Rode na RAIZ do projeto:  ./deploy/update.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck disable=SC1091
source .env
: "${API_URL:?defina API_URL no .env}"

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

echo "==> Backend: rebuild + migrações"
docker compose -f docker-compose.prod.yml up -d --build

echo "==> Front: build + restart"
cd web
npm ci
NEXT_PUBLIC_API_URL="$API_URL" npm run build
cd "$ROOT"
sudo systemctl restart zapgov-web

echo "==> Atualizado. Versão: $(git describe --tags --always)"
