#!/usr/bin/env bash
# Auto-deploy por polling: se a branch remota mudou, roda o update.sh.
# Chamado pelo systemd timer (ou cron). Só tráfego de saída pro Git.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Evita rodar duas vezes ao mesmo tempo.
exec 9>/tmp/zapgov-autoupdate.lock
flock -n 9 || { echo "$(date '+%F %T') já em execução, pulando"; exit 0; }

BRANCH="${DEPLOY_BRANCH:-main}"
git fetch origin "$BRANCH" --quiet

LOCAL="$(git rev-parse HEAD)"
REMOTE="$(git rev-parse "origin/$BRANCH")"

if [ "$LOCAL" = "$REMOTE" ]; then
  exit 0 # nada novo
fi

echo "$(date '+%F %T') novo commit em $BRANCH ($REMOTE) — atualizando"
"$ROOT/deploy/update.sh"
echo "$(date '+%F %T') deploy concluído"
