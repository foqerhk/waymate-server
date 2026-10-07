# Deploy: China (CN) vs International (INTL)

Same Go binary and Compose layout; **different host, `.env`, maps provider, and nginx site**.

For AI entry points see [`../AGENTS.md`](../AGENTS.md). Do **not** commit real `.env`, `.p8`, or SSH passwords.

## Overview

| | CN | INTL |
| --- | --- | --- |
| Public API | `https://waymate.intentcomputing.cn` | `https://waymate.intentcomputing.net` |
| Maps | `MAPS_PROVIDER=amap` | `MAPS_PROVIDER=google` |
| Local env file (not in git) | `.env` | `.env.intl` → copied to remote `.env` |
| Helper scripts | `scripts/deploy.sh`, `scripts/finish-deploy-prebuilt.sh` | `scripts/deploy-intl.sh` |
| Sample nginx | `deploy/nginx-waymate.conf` | `deploy/nginx-waymate-net.conf` |
| Privacy HTML (official site) | Chinese | English |

Generic self-host template (any region): `scripts/deploy.example.sh` + `deploy/nginx.example.conf`.

## Prebuilt binary rule (both regions)

The [`Dockerfile`](../Dockerfile) only **COPY**s a prebuilt `waymate-api`. Remote `docker compose up --build` alone will **not** compile new Go code and often serves a **cached** image.

```bash
# On a Mac / CI with Go installed:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o waymate-api ./cmd/server/

# Sync binary to the region host (paths/credentials are local-only):
rsync -az waymate-api "${DEPLOY_HOST}:/opt/waymate/waymate-api"

ssh "${DEPLOY_HOST}" 'cd /opt/waymate && docker compose build --no-cache api && docker compose up -d api'

curl -sk "https://<public-host>/healthz"
```

Optional finish helper used by operators: `scripts/finish-deploy-prebuilt.sh`.

## CN notes

- Typical VPS: Aliyun CN; remote dir `/opt/waymate`; API container name `waymate-api-1`.
- LiveKit: public WSS on same host/domain family; open UDP media ports (see README).
- Official CN API also terminates push-relay for community self-hosters (`/v1/push-relay/*`) when `PUSH_RELAY_AUTH_TOKEN` is set on that host only.

## INTL notes

- Mac → INTL SSH is often blocked; `deploy-intl.sh` defaults to **ProxyJump via the CN host** (`USE_CN_JUMP=1`).
- Use `.env.intl` / `livekit.yaml.intl` locally; script uploads them as remote `.env` / `livekit.yaml`.
- Cloudflare DNS helper: `scripts/cloudflare_dns.py` (needs operator tokens locally).

## App clients vs regions

| Client | Default API habit |
| --- | --- |
| iOS App Store build | Storefront / settings may choose `.cn` vs `.net`; **Mainland China App Store availability is off** until 备案 |
| Android `cn` flavor | Points at CN API + OEM push |
| Android `intl` flavor | Points at INTL API + FCM |

Signing for the **official** iOS binary is documented in [`AGENTS.md`](../AGENTS.md) §4 (Team `U5SLTWD6AH`, Distribution + “WayMate AppStore” profile). Self-hosters of **only the API** do not need Apple certificates if they use the official push relay.

## Checklist before calling a deploy “done”

1. [ ] New `waymate-api` binary built for `linux/amd64` and synced  
2. [ ] `docker compose build --no-cache api && up -d api` on the **correct** region host  
3. [ ] `MAPS_PROVIDER` matches the region  
4. [ ] `PUBLIC_BASE_URL` / `LIVEKIT_PUBLIC_URL` use that region’s HTTPS/WSS  
5. [ ] Official hosts: APNs secrets present on disk under `secrets/` (gitignored)  
6. [ ] Self-host: `APNS_ENABLED=false` + `PUSH_RELAY_*`  
7. [ ] `healthz` OK from the public URL  
