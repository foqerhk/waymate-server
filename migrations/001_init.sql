-- WayMate production schema: device identity + QR family pairing (no user accounts)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS devices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    installation_id TEXT NOT NULL UNIQUE,
    role            TEXT NOT NULL CHECK (role IN ('child', 'elder')),
    display_name    TEXT NOT NULL DEFAULT '',
    platform        TEXT NOT NULL DEFAULT 'ios',
    apns_token      TEXT,
    apns_sandbox    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS families (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL DEFAULT 'My Family',
    created_by_device_id UUID NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS family_members (
    family_id       UUID NOT NULL REFERENCES families(id) ON DELETE CASCADE,
    device_id       UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('child', 'elder')),
    display_name    TEXT NOT NULL DEFAULT '',
    relationship_key TEXT,
    avatar_symbol   TEXT NOT NULL DEFAULT 'person.crop.circle.fill',
    font_scale      DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (family_id, device_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS family_members_one_family_per_device
    ON family_members(device_id);

CREATE TABLE IF NOT EXISTS invite_codes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id       UUID NOT NULL REFERENCES families(id) ON DELETE CASCADE,
    code            TEXT NOT NULL UNIQUE,
    created_by      UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    expires_at      TIMESTAMPTZ NOT NULL,
    max_uses        INT NOT NULL DEFAULT 20,
    use_count       INT NOT NULL DEFAULT 0,
    revoked         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS invite_codes_family_idx ON invite_codes(family_id);

CREATE TABLE IF NOT EXISTS elder_locations (
    elder_device_id UUID PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    latitude        DOUBLE PRECISION NOT NULL,
    longitude       DOUBLE PRECISION NOT NULL,
    accuracy        DOUBLE PRECISION NOT NULL DEFAULT 0,
    heading         DOUBLE PRECISION,
    speed           DOUBLE PRECISION,
    is_background   BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS active_trips (
    elder_device_id     UUID PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    created_by_device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    route_json          JSONB NOT NULL,
    guidance_json       JSONB,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS favorites (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id       UUID NOT NULL REFERENCES families(id) ON DELETE CASCADE,
    elder_device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    payload_json    JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS favorites_elder_idx ON favorites(elder_device_id);
