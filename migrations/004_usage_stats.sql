-- Usage metering for public /status panel (per maps API key fingerprint).

CREATE TABLE IF NOT EXISTS maps_usage_daily (
    day        DATE NOT NULL,
    provider   TEXT NOT NULL,
    key_fp     TEXT NOT NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('search', 'lbs')),
    count      INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (day, provider, key_fp, kind)
);

CREATE INDEX IF NOT EXISTS maps_usage_daily_month_idx
    ON maps_usage_daily (provider, kind, day);

CREATE TABLE IF NOT EXISTS capacity_samples (
    ts           TIMESTAMPTZ PRIMARY KEY DEFAULT now(),
    active_voice INT NOT NULL DEFAULT 0,
    active_video INT NOT NULL DEFAULT 0,
    ws_approx    INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS capacity_samples_ts_idx ON capacity_samples (ts DESC);

CREATE TABLE IF NOT EXISTS maps_quota_state (
    provider      TEXT PRIMARY KEY,
    exhausted_at  TIMESTAMPTZ,
    last_infocode TEXT NOT NULL DEFAULT ''
);
