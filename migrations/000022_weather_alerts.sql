ALTER TABLE posts
    ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'user',
    ADD COLUMN IF NOT EXISTS visibility_priority INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

ALTER TABLE posts
    DROP CONSTRAINT IF EXISTS posts_origin_check;

ALTER TABLE posts
    ADD CONSTRAINT posts_origin_check
    CHECK (origin IN ('user', 'weather_alert'));

CREATE INDEX IF NOT EXISTS posts_origin_status_priority_created_at_idx
    ON posts (origin, status, visibility_priority DESC, created_at DESC);

INSERT INTO users (email, name)
VALUES ('alerts@system.aroundme.local', 'AroundMe Alerts')
ON CONFLICT (email) DO UPDATE
SET name = EXCLUDED.name,
    updated_at = NOW();

CREATE TABLE IF NOT EXISTS weather_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    event TEXT NOT NULL,
    headline TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    instruction TEXT NOT NULL DEFAULT '',
    area_desc TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL,
    provider_severity TEXT NOT NULL DEFAULT '',
    urgency TEXT NOT NULL DEFAULT '',
    certainty TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    geometry JSONB,
    center_latitude DOUBLE PRECISION NOT NULL,
    center_longitude DOUBLE PRECISION NOT NULL,
    radius_km DOUBLE PRECISION NOT NULL DEFAULT 0,
    post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT weather_alerts_status_check CHECK (status IN ('active', 'expired')),
    CONSTRAINT weather_alerts_severity_check CHECK (severity IN ('critical', 'high', 'moderate', 'low'))
);

CREATE UNIQUE INDEX IF NOT EXISTS weather_alerts_provider_external_id_idx
    ON weather_alerts (provider, external_id);

CREATE UNIQUE INDEX IF NOT EXISTS weather_alerts_post_id_idx
    ON weather_alerts (post_id)
    WHERE post_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS weather_alerts_provider_status_updated_at_idx
    ON weather_alerts (provider, status, updated_at DESC);
