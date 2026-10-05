# WayMate Server（中文）

WayMate（带路）的开源服务端：无账号、扫码配对，同步位置 / 路线 / 引导 / 收藏，并为 CallKit + 自建 LiveKit 签发通话凭证。

[English README](README.md) · **中文**

## 功能

- 设备注册 + JWT（无用户名密码）
- 家庭邀请二维码与加入流程
- REST + WebSocket 实时通道
- 高德 Web 服务代理：地点搜索、步行 / 公交规划（Key 只放服务端）
- 自建 LiveKit 音视频（老人后置摄像头）
- **推送中继**：App Store 版带路的后台唤醒走官方 Push Relay（你无需持有苹果 `.p8`）

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

## 推送说明（重要）

App Store 上的带路由官方签名，**只有官方 Apple 开发者账号**能向该 Bundle ID 发 APNs / VoIP。

因此推荐 **混合部署**：

- **数据面（你的服务器）**：配对、业务 API、WebSocket、路线、LiveKit 媒体
- **推送面（官方）**：`https://waymate.intentcomputing.cn` 的 `/v1/push-relay/*`

`.env` 中保持：

```bash
APNS_ENABLED=false
PUSH_RELAY_URL=https://waymate.intentcomputing.cn
PUSH_RELAY_TOKEN=<见 .env.example 中的社区令牌>
```

只有当你自己编译并签名 **另一套** iOS App 时，才需要自行配置 `APNS_*`。

## 需要自行准备的 Key / 证书

| 项目 | 是否必须 | 申请位置 | 说明 |
|------|----------|----------|------|
| `JWT_SECRET` | 必须 | 自行生成 | 设备会话签名 |
| `INVITE_HMAC_SECRET` | 必须 | 自行生成 | 邀请码 / 二维码签名 |
| LiveKit Key/Secret | 通话需要 | 自行设定，与 `livekit.yaml` 一致 | 房间 JWT |
| `AMAP_WEB_KEY` | 地图/路线需要 | [高德控制台](https://console.amap.com/) Web 服务 | **仅服务端** |
| `PUBLIC_BASE_URL` | 必须 | 你的 HTTPS 域名 | 写入邀请二维码 |
| `PUSH_RELAY_*` | App Store 版推荐 | 官方中继 | 无需苹果证书 |
| Apple `.p8` / Team ID | 仅自定义签名 App | Apple Developer | 不能用于官方 App Store 包 |
| TLS 证书 | 生产环境 | Let's Encrypt 等 | API 与 LiveKit 信令 |
| UDP 50000–50100 | 生产音视频 | 云厂商安全组 | LiveKit WebRTC |

完整变量见 [`.env.example`](.env.example)。

## 许可证

[MIT](LICENSE)
