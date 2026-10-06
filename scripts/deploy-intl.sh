#!/usr/bin/env bash
# Deploy WayMate backend to intl VPS (208.***.***.***) + Cloudflare DNS + TLS.
# Usage: from backend/: ./scripts/deploy-intl.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

REGISTRAR_ENV="${REGISTRAR_ENV:-/Users/liuwei/Downloads/runeverything-registrar/.env}"
if [[ -f "$REGISTRAR_ENV" ]]; then
  # shellcheck disable=SC1090
  set -a; source "$REGISTRAR_ENV"; set +a
fi
if [[ -f .env.intl ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env.intl
  set +a
elif [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

HOST_USER="${SSH_INTL_USER:-ubuntu}"
HOST_IP="${SSH_INTL_HOST:-208.***.***.***}"
HOST_PORT="${SSH_INTL_PORT:-34417}"
HOST="${HOST_USER}@${HOST_IP}"
ORIGIN_IP="${DEPLOY_ORIGIN_IP:-${HOST_IP}}"
API_HOST="${DEPLOY_API_HOST:-waymate.intentcomputing.net}"
REMOTE_DIR="${DEPLOY_REMOTE_DIR:-/opt/waymate}"
ZONE="${RE_INTL_ZONE:-intentcomputing.net}"
RR="waymate"
SSHPASS_VALUE="${SSH_INTL_PASSWORD:-${SSHPASS:-}}"
SSH_CN_HOST_VAL="${SSH_CN_HOST:-8.***.***.***}"
SSH_CN_USER_VAL="${SSH_CN_USER:-root}"
SSH_CN_PASS_VAL="${SSH_CN_PASSWORD:-${SSHPASS_CN:-}}"
# Mac → intl often blocked at kex; tunnel via CN Aliyun by default.
USE_CN_JUMP="${USE_CN_JUMP:-1}"

SSH_WRAP="$(mktemp /tmp/waymate-intl-ssh.XXXXXX)"
cleanup_ssh_wrap() { rm -f "$SSH_WRAP"; }
trap cleanup_ssh_wrap EXIT

{
  echo '#!/usr/bin/env bash'
  echo 'set -euo pipefail'
  echo "export SSHPASS=$(printf %q "$SSHPASS_VALUE")"
  if [[ "$USE_CN_JUMP" == "1" ]]; then
    echo "exec sshpass -e ssh -p $(printf %q "$HOST_PORT") \\"
    echo "  -o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30 -o ConnectTimeout=25 \\"
    echo "  -o PreferredAuthentications=password -o PubkeyAuthentication=no \\"
    echo "  -o ProxyCommand=$(printf %q "sshpass -p ${SSH_CN_PASS_VAL} ssh -o StrictHostKeyChecking=accept-new -W %h:%p ${SSH_CN_USER_VAL}@${SSH_CN_HOST_VAL}") \\"
    echo '  "$@"'
  else
    echo "exec sshpass -e ssh -p $(printf %q "$HOST_PORT") \\"
    echo "  -o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30 -o ConnectTimeout=25 \\"
    echo "  -o PreferredAuthentications=password -o PubkeyAuthentication=no \\"
    echo '  "$@"'
  fi
} >"$SSH_WRAP"
chmod +x "$SSH_WRAP"

ssh_run() { "$SSH_WRAP" "$HOST" "$@"; }
scp_run() {
  # scp uses same proxy via -S / -o; use ssh wrap for remote shell copy via tar instead when needed
  local scp_cmd=(scp -P "$HOST_PORT" -o StrictHostKeyChecking=accept-new -o PreferredAuthentications=password -o PubkeyAuthentication=no)
  if [[ "$USE_CN_JUMP" == "1" ]]; then
    scp_cmd+=(-o "ProxyCommand=sshpass -p ${SSH_CN_PASS_VAL} ssh -o StrictHostKeyChecking=accept-new -W %h:%p ${SSH_CN_USER_VAL}@${SSH_CN_HOST_VAL}")
  fi
  SSHPASS="$SSHPASS_VALUE" sshpass -e "${scp_cmd[@]}" "$@"
}
rsync_run() { rsync -az -e "$SSH_WRAP" "$@"; }

echo "==> Cloudflare DNS upsert ${RR}.${ZONE} -> ${ORIGIN_IP} (DNS-only)"
export RE_CF_API_TOKEN CF_API_TOKEN="${RE_CF_API_TOKEN:-${CF_API_TOKEN:-}}"
CF_PROXIED=0 CF_TTL=120 python3 scripts/cloudflare_dns.py "$ZONE" "$RR" "$ORIGIN_IP"

echo "==> Wait for SSH ${HOST}:${HOST_PORT}"
ok=0
for i in $(seq 1 30); do
  if ssh_run 'echo up'; then ok=1; break; fi
  sleep 5
done
[[ "$ok" == "1" ]] || { echo "SSH failed to ${HOST}:${HOST_PORT}"; exit 1; }

echo "==> Ensure remote dir ${REMOTE_DIR}"
ssh_run "sudo mkdir -p ${REMOTE_DIR}/deploy ${REMOTE_DIR}/scripts ${REMOTE_DIR}/secrets /var/lib/waymate && sudo chown -R ${HOST_USER}:${HOST_USER} ${REMOTE_DIR} /var/lib/waymate"

echo "==> Sync backend (uses .env.intl on remote as .env)"
rsync_run \
  --exclude '.git' \
  --exclude 'waymate_pg' \
  --exclude '__pycache__' \
  --exclude '.env' \
  ./ "${HOST}:${REMOTE_DIR}/"

if [[ -f .env.intl ]]; then
  scp_run .env.intl "${HOST}:${REMOTE_DIR}/.env"
elif [[ -f .env ]]; then
  echo "WARN: no .env.intl — copying .env (verify PUBLIC_BASE_URL / LIVEKIT_PUBLIC_URL)"
  scp_run .env "${HOST}:${REMOTE_DIR}/.env"
fi

if [[ -f livekit.yaml.intl ]]; then
  scp_run livekit.yaml.intl "${HOST}:${REMOTE_DIR}/livekit.yaml"
fi

# Copy APNs key from CN layout if present locally
if [[ -f secrets/AuthKey_A3289S3NS2.p8 ]]; then
  scp_run secrets/AuthKey_A3289S3NS2.p8 "${HOST}:${REMOTE_DIR}/secrets/AuthKey_A3289S3NS2.p8"
fi

echo "==> Ensure docker + nginx + certbot"
ssh_run 'bash -s' <<'REMOTE'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
SUDO="sudo"
if ! $SUDO -n true 2>/dev/null; then
  # passwordless sudo expected on this host; fall back to plain sudo
  true
fi
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | $SUDO sh
  $SUDO usermod -aG docker "$USER" || true
fi
# docker compose plugin
if ! docker compose version >/dev/null 2>&1; then
  $SUDO apt-get update -y
  $SUDO apt-get install -y docker-compose-plugin || true
fi
if ! command -v nginx >/dev/null 2>&1; then
  $SUDO apt-get update -y
  $SUDO apt-get install -y nginx
fi
if ! command -v certbot >/dev/null 2>&1; then
  $SUDO apt-get update -y
  $SUDO apt-get install -y certbot python3-certbot-nginx
fi
# websocket map once
MAP_FILE=/etc/nginx/conf.d/00-websocket-map.conf
if [[ ! -f "$MAP_FILE" ]]; then
  $SUDO tee "$MAP_FILE" >/dev/null <<'EOF'
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}
EOF
fi
REMOTE

echo "==> Install nginx site"
scp_run deploy/nginx-waymate-net.conf "${HOST}:/tmp/waymate-nginx.conf"
ssh_run "sudo cp /tmp/waymate-nginx.conf /etc/nginx/sites-available/waymate && sudo ln -sfn /etc/nginx/sites-available/waymate /etc/nginx/sites-enabled/waymate && sudo nginx -t && sudo systemctl reload nginx"

echo "==> docker compose up"
# New docker group may need sg/newgrp; use sudo docker for first boot
ssh_run "cd ${REMOTE_DIR} && (docker compose version >/dev/null 2>&1 && docker compose up -d --build || sudo docker compose up -d --build)"

echo "==> Issue TLS via certbot nginx if missing"
ssh_run "if [[ ! -d /etc/letsencrypt/live/${API_HOST} ]]; then sudo certbot --nginx -d ${API_HOST} --non-interactive --agree-tos -m admin@intentcomputing.net --redirect; fi"
# Always re-apply our site config (certbot may rewrite the vhost)
scp_run deploy/nginx-waymate-net.conf "${HOST}:/tmp/waymate-nginx.conf"
ssh_run "sudo cp /tmp/waymate-nginx.conf /etc/nginx/sites-available/waymate && sudo ln -sfn /etc/nginx/sites-available/waymate /etc/nginx/sites-enabled/waymate && sudo nginx -t && sudo systemctl reload nginx"

echo "==> Health checks"
sleep 5
ssh_run "curl -sS --max-time 10 http://127.0.0.1:18080/healthz || true"
echo
curl -sS --max-time 20 "https://${API_HOST}/healthz" || true
echo
echo "Deployed: https://${API_HOST}"
echo "LiveKit public: wss://${API_HOST}"
