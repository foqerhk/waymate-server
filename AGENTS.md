# waymate-server — AI / Agent 必读

公开仓：https://github.com/foqerhk/waymate-server  
客户端仓：https://github.com/foqerhk/WayMate（**不要**把本仓密钥写进客户端仓）

本机含 SSH / `.p8` / 真实 IP 的操作手册：各维护者私有副本（例如本机 `WayMate/backend` 旁的 `docs/AGENT-LOCAL*` 或 skill `waymate-device-deploy`）。**禁止**把密码、私钥、完整 `.env` 提交到本公开库。

---

## 1) Git（硬性）

| 项 | 要求 |
| --- | --- |
| Author / Committer | `foqerhk <foqerhk@gmail.com>` |
| 禁止 | `Co-authored-by: Cursor`、任何 Cursor / cursoragent 痕迹 |
| 禁止 | 改全局 `git config` |

提交后检查：`git log -1 --format='%an <%ae>%n%b'`。若被注入 trailer，用 `git commit-tree` 重写后再按授权 force-with-lease。

---

## 2) 国区 / 外区（同一套代码，两套部署）

| | **国区 CN** | **外区 INTL** |
| --- | --- | --- |
| 公网 API | `https://waymate.intentcomputing.cn` | `https://waymate.intentcomputing.net`（及 `.net` 站点） |
| `MAPS_PROVIDER` | `amap` + `AMAP_WEB_KEY(S)` | `google` + `GOOGLE_MAPS_API_KEY(S)` |
| 隐私页语言 | 简体中文 | English |
| 典型脚本 | `scripts/deploy.sh` / `finish-deploy-prebuilt.sh` | `scripts/deploy-intl.sh`（Mac→外区常 **经 CN 跳板**） |
| 远端目录 | `/opt/waymate` | `/opt/waymate`（外区机） |
| Compose 容器 | `waymate-api-1` | 同名约定 |
| Android 客户端 flavor | `cn`（OEM 推送） | `intl`（FCM） |
| iOS 客户端默认 | 指向 CN；设置里可按商店/网络切 `.net` | 外区商店流量走 INTL |

环境文件（**不进 Git**）：国区用 `.env`；外区用 `.env.intl`（部署时拷到远端 `.env`）。示例变量见 [`.env.example`](.env.example)。

更细的 nginx / DNS / TLS：`deploy/`、`docs/DEPLOY-REGIONS.md`。

---

## 3) 生产部署铁律（官方机也一样）

Dockerfile **只 COPY 预编译** `waymate-api`，服务器上 `docker compose up --build` **不会**重新 `go build`。

正确顺序：

1. 本机：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o waymate-api ./cmd/server/`
2. `rsync` 二进制到 `${DEPLOY_HOST}:/opt/waymate/waymate-api`
3. 远端：`docker compose build --no-cache api && docker compose up -d api`
4. `curl` healthz；必要时 `grep -a` 抽查新字符串是否在容器内 `/proc/1/exe`

细节与检查清单：[`docs/DEPLOY-REGIONS.md`](docs/DEPLOY-REGIONS.md)。

---

## 4) App 上架 / 签名 / 推送（官方 vs 自托管）

### 官方 App Store 客户端（`foqerhk/WayMate`）

| 项 | 值 |
| --- | --- |
| Bundle ID | `cn.intentcomputing.waymate` |
| Apple Team | **个人号** `U5SLTWD6AH`（Developer ID / Distribution：`wei liu`） |
| **禁止** | 三鹰公司号 `BGC93C2SX5` 签官方包（TCC / 推送会对不上） |
| Debug 真机 | Automatic + Development |
| App Store IPA | **Apple Distribution** + profile 名 **「WayMate AppStore」**；Release entitlements `aps-environment=production` |
| ASC App ID | `6820131465`（外区先上；**国区可用性先关**，等备案） |
| Xcode | `/Applications/Xcode.app`（27.0）；**不要**用 KoKo 的 27.1 beta |

官方生产 API（CN / INTL）上应配置 **本机 APNs**（`APNS_ENABLED=true` + 服务器 `secrets/*.p8`，不进 Git），直接推官方 Bundle。

### 社区自托管 API + 官方 App Store 包

自托管者**没有**官方 Team 的 `.p8`，不能直连 APNs。模式：

- 数据面：自有服务器  
- 推送面：官方 Push Relay（`PUSH_RELAY_URL` + `PUSH_RELAY_TOKEN`，见 README）  
- `APNS_ENABLED=false`

### 自己重新签名 / 换 Bundle 的 Fork

用你自己的 Team + `.p8`，`APNS_ENABLED=true`，不要走官方 relay（或令牌无效）。

---

## 5) 推送矩阵（服务端）

| 客户端 | 国区 | 外区 |
| --- | --- | --- |
| iOS 官方包 | APNs / VoIP（官方服）或 Relay（自托管服） | 同左 |
| Android `cn` | 华为 / 小米 / OPPO / vivo / 荣耀等 OEM（凭证可空=stub） | — |
| Android `intl` | — | FCM HTTP v1 |
| Harmony | Push Kit（常复用华为凭证） | 按部署 |

OEM / FCM 变量见 `.env.example`；空凭证不得导致进程崩溃。

---

## 6) 与客户端仓分工

| 改动 | 仓库 |
| --- | --- |
| API / migration / nginx / 部署脚本 | **本仓** waymate-server |
| iOS / Android / Harmony UI | WayMate 客户端仓 |
| 本机双真机安装、ASC 上传密钥路径 | 客户端 `AGENTS.md` + 本机 AGENT-LOCAL / skill |

改完服务端协议后：先按 §3 部署对应区域，再让客户端联调。
