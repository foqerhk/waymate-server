CREATE TABLE IF NOT EXISTS calls (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id         UUID NOT NULL REFERENCES families(id) ON DELETE CASCADE,
    caller_device_id  UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    callee_device_id  UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    media_type        TEXT NOT NULL CHECK (media_type IN ('voice', 'video')),
    channel_name      TEXT NOT NULL,
    status            TEXT NOT NULL CHECK (status IN ('ringing', 'active', 'ended', 'missed')) DEFAULT 'ringing',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at       TIMESTAMPTZ,
    ended_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS calls_callee_idx ON calls(callee_device_id, status);
CREATE INDEX IF NOT EXISTS calls_family_idx ON calls(family_id);

ALTER TABLE devices ADD COLUMN IF NOT EXISTS voip_token TEXT;
