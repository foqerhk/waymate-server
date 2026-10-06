# Capacity planning for WayMate official hosts
# Numbers are engineering estimates for guidance — live shortfalls appear on /status.

## Current deployments

| Region | Host | Typical VM | Maps | Notes |
|--------|------|------------|------|-------|
| CN `.cn` | Aliyun `8.***.***.***` | ~2–4 vCPU class | Amap Web (personal) | LiveKit UDP `50000–50100` |
| Intl `.net` | `208.***.***.***` | shared VPS | Google Maps Platform | Same binary, `MAPS_PROVIDER=google` |

## Concurrent calls (single LiveKit node)

Defaults in env (override per host):

| Knob | Default | Meaning |
|------|---------|---------|
| `CAPACITY_MAX_VOICE_CALLS` | 30 | Simultaneous voice sessions (2-party) |
| `CAPACITY_MAX_VIDEO_CALLS` | 8 | Simultaneous video (elder rear cam ~720p) |
| `CAPACITY_VIDEO_WEIGHT` | 3 | Load units ≈ voice + video×weight |
| `PEAK_CALL_FRACTION_PCT` | 2 | Assume 2% of families on a call at peak |

Rough family capacity from calls:

`families_by_calls ≈ CAPACITY_MAX_VOICE_CALLS × 100 / PEAK_CALL_FRACTION_PCT`

Example: 30 voice slots → ~1500 families at 2% peak concurrency (optimistic; CPU/bandwidth may hit earlier).

## Amap quotas (personal developer baseline)

Per **key** (configure more keys later via `AMAP_WEB_KEYS`):

| Service group | Default monthly / key | Env |
|---------------|----------------------|-----|
| Keyword search | **5,000** | `AMAP_SEARCH_MONTHLY_QUOTA` |
| LBS walk + transit | **150,000** | `AMAP_LBS_MONTHLY_QUOTA` |

Metering is **per key fingerprint** in Postgres; the public `/status` page shows **totals only** (`keys × per-key quota` vs summed usage).

Family estimate from maps (search usually binds first):

`families_by_maps ≈ (searchQuotaMonth / 30) / MAPS_CALLS_PER_FAMILY_DAY`

with `MAPS_CALLS_PER_FAMILY_DAY` default **20**.

**Recommended families** on the status page = `min(families_by_maps, families_by_calls)`.

Official console usage remains authoritative; our counters are self-metered successful API calls.

## Google (intl)

Set `GOOGLE_MAPS_API_KEYS` and `GOOGLE_SEARCH_MONTHLY_QUOTA` / `GOOGLE_LBS_MONTHLY_QUOTA` from your Cloud quotas. Same per-key metering, aggregated UI.

## How to serve more users

1. **Maps**: add more Amap/Google keys, buy traffic packs, raise env quotas, or move to enterprise tier.
2. **Calls**: larger ECS / more bandwidth; raise `CAPACITY_MAX_*`; add LiveKit nodes and split rooms.
3. **API/DB**: horizontal API replicas behind nginx; managed Postgres when write load grows.
4. **Watch** `/status` bottleneck field (`maps` vs `calls`) before buying capacity.

## Public endpoints

- Marketing: `https://waymate.intentcomputing.cn/` (and `.net`)
- Status: `https://waymate.intentcomputing.cn/status`
- JSON: `GET /v1/public/status?range=7d|30d`
