#!/usr/bin/env bash
set -euo pipefail
ROOT=/Users/liuwei/Downloads/WayMate/backend
cd "$ROOT"
set -a; source .env; set +a
HOST="${DEPLOY_HOST:-root@8.***.***.***}"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30 -o ConnectTimeout=20)
ssh_run(){ SSHPASS="${SSHPASS:-$SSH_CN_PASSWORD}" sshpass -e ssh "${SSH_OPTS[@]}" "$HOST" "$@"; }
scp_run(){ SSHPASS="${SSHPASS:-$SSH_CN_PASSWORD}" sshpass -e scp "${SSH_OPTS[@]}" "$@"; }
rsync_run(){ SSHPASS="${SSHPASS:-$SSH_CN_PASSWORD}" sshpass -e rsync -az -e "ssh ${SSH_OPTS[*]}" "$@"; }

echo "==> wait for ssh"
for i in $(seq 1 60); do
  if ssh_run 'echo up'; then break; fi
  sleep 10
done
ssh_run 'echo up'

# ensure docker mirror
ssh_run 'bash -s' <<'REMOTE'
set -euo pipefail
mkdir -p /etc/docker
cat >/etc/docker/daemon.json <<EOF
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://dockerproxy.net"
  ]
}
EOF
systemctl restart docker || true
sleep 2
docker --version
REMOTE

echo "==> sync"
rsync_run --exclude '.git' --exclude 'waymate_pg' --exclude '__pycache__' ./ "${HOST}:/opt/waymate/"
ssh_run 'mkdir -p /opt/waymate && chmod +x /opt/waymate/waymate-api || true'

echo "==> compose up"
ssh_run 'cd /opt/waymate && docker compose up -d --build'
ssh_run 'nginx -t && systemctl reload nginx'
sleep 5
ssh_run 'curl -sS --max-time 10 http://127.0.0.1:18080/healthz; echo; cd /opt/waymate && docker compose ps'
curl -sk --max-time 20 --resolve waymate.intentcomputing.cn:443:8.***.***.*** https://waymate.intentcomputing.cn/healthz || true
echo
