# 带路容量说明（官方节点）

数字为工程估算，实时短板以 `/status` 为准。

## 现网

| 区域 | 主机 | 地图 | 说明 |
|------|------|------|------|
| 国区 `.cn` | 阿里云 `8.***.***.***` | 高德 Web（个人开发者） | LiveKit UDP `50000–50100` |
| 外区 `.net` | `208.***.***.***` | Google Maps Platform | 同一二进制，`MAPS_PROVIDER=google` |

## 并发通话（单 LiveKit）

| 环境变量 | 默认 | 含义 |
|----------|------|------|
| `CAPACITY_MAX_VOICE_CALLS` | 30 | 同时语音通话数 |
| `CAPACITY_MAX_VIDEO_CALLS` | 8 | 同时视频（老人后置约 720p） |
| `CAPACITY_VIDEO_WEIGHT` | 3 | 负载 ≈ 语音 + 视频×权重 |
| `PEAK_CALL_FRACTION_PCT` | 2 | 假设高峰 2% 家庭在通话 |

`families_by_calls ≈ 语音上限 × 100 / 高峰百分比`

## 高德配额（个人开发者基线）

按 **每个 Key**：

| 服务 | 默认月配额 | 环境变量 |
|------|------------|----------|
| 关键字搜索 | **5000** | `AMAP_SEARCH_MONTHLY_QUOTA` |
| 步行+公交 LBS | **150000** | `AMAP_LBS_MONTHLY_QUOTA` |

多 Key：`AMAP_WEB_KEYS=key1,key2,...`（兼容 `AMAP_WEB_KEY`）。库内按 Key 指纹记账；**状态页只展示总和**。

`families_by_maps ≈ (搜索月配额合计 / 30) / MAPS_CALLS_PER_FAMILY_DAY`（默认日均 20 次）。

推荐家庭数 = `min(地图侧, 通话侧)`。控制台用量仍以高德后台为准；面板为自计量成功调用。

## 如何服务更多人

1. 地图：加 Key、买流量包、调高 env 配额或升企业认证  
2. 通话：升配带宽/CPU、提高 `CAPACITY_MAX_*`、多 LiveKit 节点  
3. API/库：API 水平扩展、托管 Postgres  
4. 先看 `/status` 的 `bottleneck` 再扩容  

## 入口

- 官网 `/` · 状态 `/status` · JSON `/v1/public/status?range=7d`
