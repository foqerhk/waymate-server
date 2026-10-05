# WayMate Server

Open-source backend for [WayMate](https://github.com/foqerhk/waymate-server) (带路): a no-account family navigation companion. Elders and family members pair with a QR code; the API syncs location, routes, guidance, favorites, and CallKit/LiveKit media tokens.

**English** · [中文说明](README.zh-CN.md)

## Features

- Device registration with JWT (no usernames / passwords)
- Family invite QR + join flow
- REST + WebSocket realtime (location, routes, call signaling)
- Place search and walking / transit routing proxied through [Amap Web Service](https://lbs.amap.com/)
- Self-hosted [LiveKit](https://livekit.io/) for voice / video (elder rear camera)
- Optional Apple Push Notification service (APNs) / VoIP push

## Quick start

Requirements: Docker + Docker Compose, and the keys listed below.

```bash
git clone https://github.com/foqerhk/waymate-server.git
cd waymate-server
cp .env.example .env
cp livekit.yaml.example livekit.yaml
# Edit .env and livekit.yaml — set secrets and AMAP_WEB_KEY
docker compose up --build -d
curl -s http://127.0.0.1:18080/healthz
```

Point the WayMate iOS app **Settings → Server** URL at your API origin (LAN HTTP for debug, HTTPS in production).

## Keys and certificates you must prepare

| Item | Required? | Where to get it | Notes |
|------|-----------|-----------------|-------|
| `JWT_SECRET` | **Yes** | Generate yourself (`openssl rand -hex 32`) | Signs device session tokens |
| `INVITE_HMAC_SECRET` | **Yes** | Generate yourself | Signs family invite QR payloads |
| `LIVEKIT_API_KEY` / `LIVEKIT_API_SECRET` | **Yes** (for calls) | Choose any pair; must match `livekit.yaml` → `keys` | Used to mint LiveKit room JWTs |
| `AMAP_WEB_KEY` | **Yes** (for maps / routes) | [Amap console](https://console.amap.com/) → Web service key | Keep **server-side only**. Enable place search + walking + transit. Whitelist your server egress IP if the console requires it |
| `PUBLIC_BASE_URL` | **Yes** | Your public HTTPS origin | Embedded in invite QR links |
| Apple Developer Team ID | For push / VoIP | [Apple Developer](https://developer.apple.com/) | `APNS_TEAM_ID` |
| APNs Auth Key (`.p8`) | For push / VoIP | Certificates, Identifiers & Profiles → Keys | Enable Apple Push Notifications; download once; store under `secrets/` (gitignored) |
| `APNS_KEY_ID` | For push / VoIP | Same Keys page | 10-character Key ID |
| App Bundle ID | For push / VoIP | Your iOS app id | Must match the app that registers the device / VoIP token (`APNS_BUNDLE_ID`) |
| Push & VoIP capabilities | For calling | Xcode + Apple Developer App ID | Enable Push Notifications and Voice over IP; create a VoIP Services certificate if you still use certificate-based VoIP (token auth via `.p8` is preferred) |
| TLS certificate | Production HTTPS / WSS | Let's Encrypt, Cloudflare, etc. | Terminate TLS in nginx / Caddy in front of the API and LiveKit signaling |
| UDP `50000–50100` + TCP `7881` | Production media | Cloud firewall / security group | Required for LiveKit WebRTC |

See [`.env.example`](.env.example) for every variable.

### Minimal local setup (no push)

You can run pairing, location sync, and Amap routing with only:

1. Strong `JWT_SECRET` + `INVITE_HMAC_SECRET`
2. `AMAP_WEB_KEY`
3. Matching LiveKit key/secret if you test calls

Leave `APNS_ENABLED=false` until the Apple key is ready.

## Production checklist

1. Put the API behind HTTPS and LiveKit signaling behind WSS (sample nginx: [`deploy/nginx.example.conf`](deploy/nginx.example.conf)).
2. Set `PUBLIC_BASE_URL` and `LIVEKIT_PUBLIC_URL` to that public host.
3. Open UDP `50000–50100` (and TCP `7881`) toward the LiveKit container / host.
4. If LiveKit cannot detect your public IP, set `rtc.node_ip` in `livekit.yaml`.
5. Enable APNs when ready (`APNS_ENABLED=true`, mount `secrets/*.p8`).
6. Back up the Postgres volume (`waymate_pg`).

## API overview

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/v1/devices/register` | Create/restore device + JWT |
| `GET` | `/v1/session` | Family snapshot |
| `POST` | `/v1/families` | Family creates invite |
| `POST` | `/v1/families/join` | Elder joins via QR / code |
| `POST` | `/v1/places/search` | Amap place search proxy |
| `POST` | `/v1/routes/plan` | Walking / transit plan proxy |
| `GET` | `/v1/ws` | Realtime WebSocket |
| `POST` | `/v1/calls/...` | Start / answer / end + LiveKit tokens |

## Project layout

```text
cmd/server/          HTTP entrypoint
internal/            API, DB, Amap, LiveKit tokens, APNs, WebSocket hub
migrations/          Postgres SQL
deploy/              Reverse-proxy examples
livekit.yaml.example LiveKit SFU sample config
docker-compose.yml   Postgres + API + LiveKit
```

## Security notes

- Never commit `.env`, `secrets/`, or `livekit.yaml`.
- Never embed Amap or Apple keys in the iOS client.
- Rotate JWT / invite / LiveKit secrets if they ever leak.
- Report vulnerabilities privately (see [SECURITY.md](SECURITY.md)).

## License

[MIT](LICENSE)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
