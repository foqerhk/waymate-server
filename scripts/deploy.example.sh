#!/usr/bin/env bash
# Example production sync. Fill DEPLOY_* in .env — no hard-coded hosts.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

: "${DEPLOY_HOST:?Set DEPLOY_HOST, e.g. root@203.0.113.10}"
: "${DEPLOY_REMOTE_DIR:=/opt/waymate}"

SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30)

echo "==> Sync to ${DEPLOY_HOST}:${DEPLOY_REMOTE_DIR}"
rsync -az -e "ssh ${SSH_OPTS[*]}" \
  --exclude '.git' \
  --exclude 'waymate_pg' \
  --exclude '.env' \
  --exclude 'secrets' \
  ./ "${DEPLOY_HOST}:${DEPLOY_REMOTE_DIR}/"

echo "==> Reminder: copy .env + secrets + livekit.yaml on the server, then:"
echo "    ssh ${DEPLOY_HOST} 'cd ${DEPLOY_REMOTE_DIR} && docker compose up -d --build'"
echo "Install TLS with your preferred tool and adapt deploy/nginx.example.conf."
