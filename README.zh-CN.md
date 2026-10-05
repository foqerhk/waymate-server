# WayMate Server（中文）

WayMate（带路）的开源服务端：无账号、扫码配对，同步位置 / 路线 / 引导 / 收藏，并为 CallKit + 自建 LiveKit 签发通话凭证。

[English README](README.md) · **中文**

## 功能

- 设备注册 + JWT（无用户名密码）
- 家庭邀请二维码与加入流程
- REST + WebSocket 实时通道
- 高德 Web 服务代理：地点搜索、步行 / 公交规划（Key 只放服务端）
- 自建 LiveKit 音视频（老人后置摄像头）
- 可选 APNs / VoIP 推送

## 快速开始

需要 Docker Compose，以及下方列出的密钥。

```bash
git clone https://github.com/foqerhk/waymate-server.git
cd waymate-server
cp .env.example .env
cp livekit.yaml.example livekit.yaml
# 编辑 .env 与 livekit.yaml
docker compose up --build -d
curl -s http://127.0.0.1:18080/healthz
```

在 iOS App **设置 → 服务器** 填入你的 API 地址。

## 需要自行准备的 Key / 证书

| 项目 | 是否必须 | 申请位置 | 说明 |
|------|----------|----------|------|
| `JWT_SECRET` | 必须 | 自行生成 | 设备会话签名 |
| `INVITE_HMAC_SECRET` | 必须 | 自行生成 | 邀请码 / 二维码签名 |
| LiveKit Key/Secret | 通话需要 | 自行设定，与 `livekit.yaml` 一致 | 房间 JWT |
| `AMAP_WEB_KEY` | 地图/路线需要 | [高德控制台](https://console.amap.com/) Web 服务 | **仅服务端**；开通搜索与路径规划；按控制台要求配置服务器出口 IP 白名单 |
| `PUBLIC_BASE_URL` | 必须 | 你的 HTTPS 域名 | 写入邀请二维码 |
| Apple Team ID + `.p8` Key | 推送/通话通知 | [Apple Developer](https://developer.apple.com/) | 开启 Push；Key 下载一次后放入 `secrets/` |
| Bundle ID + Push / VoIP 能力 | 通话推送 | Xcode / Developer App ID | 与 App 一致 |
| TLS 证书 | 生产环境 | Let's Encrypt 等 | API 与 LiveKit 信令走 HTTPS/WSS |
| UDP 50000–50100 | 生产音视频 | 云厂商安全组 | LiveKit WebRTC 媒体面 |

完整变量见 [`.env.example`](.env.example)。本地联调可不启 APNs（`APNS_ENABLED=false`）。

## 生产建议

1. 用 Nginx/Caddy 终止 TLS（示例：[`deploy/nginx.example.conf`](deploy/nginx.example.conf)）
2. `PUBLIC_BASE_URL` / `LIVEKIT_PUBLIC_URL` 指向公网域名
3. 放开 LiveKit UDP 端口；必要时在 `livekit.yaml` 设置 `rtc.node_ip`
4. 备份 Postgres 数据卷

## 许可证

[MIT](LICENSE)
