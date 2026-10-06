#!/usr/bin/env bash
# Deploy WayMate backend to Aliyun ECS + upsert DNS + TLS + nginx.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

HOST="${DEPLOY_HOST:-root@8.***.***.***}"
ORIGIN_IP="${DEPLOY_ORIGIN_IP:-8.***.***.***}"
API_HOST="${DEPLOY_API_HOST:-waymate.intentcomputing.cn}"
REMOTE_DIR="${DEPLOY_REMOTE_DIR:-/opt/waymate}"
ZONE="${RE_CN_DOMAIN:-intentcomputing.cn}"
RR="waymate"
REGISTRAR_ROOT="${REGISTRAR_ROOT:-/Users/liuwei/Downloads/runeverything-registrar}"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30)

ssh_run() {
  if command -v sshpass >/dev/null 2>&1 && [[ -n "${SSHPASS:-${SSH_CN_PASSWORD:-}}" ]]; then
    SSHPASS="${SSHPASS:-$SSH_CN_PASSWORD}" sshpass -e ssh "${SSH_OPTS[@]}" "$HOST" "$@"
  else
    ssh "${SSH_OPTS[@]}" "$HOST" "$@"
  fi
}

scp_run() {
  if command -v sshpass >/dev/null 2>&1 && [[ -n "${SSHPASS:-${SSH_CN_PASSWORD:-}}" ]]; then
    SSHPASS="${SSHPASS:-$SSH_CN_PASSWORD}" sshpass -e scp "${SSH_OPTS[@]}" "$@"
  else
    scp "${SSH_OPTS[@]}" "$@"
  fi
}

rsync_run() {
  if command -v sshpass >/dev/null 2>&1 && [[ -n "${SSHPASS:-${SSH_CN_PASSWORD:-}}" ]]; then
    SSHPASS="${SSHPASS:-$SSH_CN_PASSWORD}" sshpass -e rsync -az -e "ssh ${SSH_OPTS[*]}" "$@"
  else
    rsync -az -e "ssh ${SSH_OPTS[*]}" "$@"
  fi
}

echo "==> DNS upsert ${RR}.${ZONE} -> ${ORIGIN_IP}"
export RE_ALIYUN_ACCESS_KEY_ID ALIYUN_ACCESS_KEY_ID
export RE_ALIYUN_ACCESS_KEY_SECRET ALIYUN_ACCESS_KEY_SECRET
# Prefer registrar script; fall back to RE_ / ALIYUN_ env names
if [[ -f "$REGISTRAR_ROOT/scripts/aliyun_dns.py" ]]; then
  python3 "$REGISTRAR_ROOT/scripts/aliyun_dns.py" upsert "$ZONE" "$RR" "$ORIGIN_IP"
else
  python3 - <<'PY'
import os, sys
sys.path.insert(0, os.environ.get("REGISTRAR_ROOT", ""))
raise SystemExit("missing aliyun_dns.py; set REGISTRAR_ROOT")
PY
fi

echo "==> Ensure remote dir ${REMOTE_DIR}"
ssh_run "mkdir -p ${REMOTE_DIR}/deploy ${REMOTE_DIR}/scripts /var/lib/waymate"

echo "==> Sync backend sources"
rsync_run \
  --exclude '.git' \
  --exclude 'waymate_pg' \
  --exclude '__pycache__' \
  ./ "${HOST}:${REMOTE_DIR}/"

echo "==> Ensure Aliyun DNS hooks available for certbot"
ssh_run "test -x /opt/runeverything-registrar/scripts/certbot-aliyun-auth.sh"

echo "==> Issue / renew TLS for ${API_HOST} (DNS-01)"
ssh_run "bash /opt/runeverything-registrar/scripts/issue-cert-dns01.sh /opt/runeverything-registrar ${API_HOST}"

echo "==> Install nginx site"
# Ensure websocket map exists once in http context
ssh_run 'bash -s' <<'REMOTE'
set -euo pipefail
MAP_FILE=/etc/nginx/conf.d/00-websocket-map.conf
if [[ ! -f "$MAP_FILE" ]]; then
  cat >"$MAP_FILE" <<'EOF'
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}
EOF
fi
REMOTE

scp_run deploy/nginx-waymate.conf "${HOST}:/etc/nginx/sites-available/waymate"
ssh_run "ln -sfn /etc/nginx/sites-available/waymate /etc/nginx/sites-enabled/waymate"

echo "==> Open RTC UDP ports if firewalld/ufw present (best-effort)"
ssh_run 'bash -s' <<'REMOTE'
set -euo pipefail
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q active; then
  ufw allow 50000:50100/udp || true
  ufw allow 443/tcp || true
fi
if command -v firewall-cmd >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port=50000-50100/udp || true
  firewall-cmd --reload || true
fi
# Aliyun security group is outside the VM — document if media fails.
REMOTE

echo "==> docker compose up"
ssh_run "cd ${REMOTE_DIR} && docker compose pull || true"
ssh_run "cd ${REMOTE_DIR} && docker compose up -d --build"

echo "==> nginx test + reload"
ssh_run "nginx -t && systemctl reload nginx"

echo "==> Health checks"
sleep 3
ssh_run "curl -sS --max-time 10 http://127.0.0.1:18080/healthz || true"
echo
curl -sS --max-time 20 "https://${API_HOST}/healthz" || true
echo
echo "Deployed: https://${API_HOST}"
echo "LiveKit public: wss://${API_HOST}"
