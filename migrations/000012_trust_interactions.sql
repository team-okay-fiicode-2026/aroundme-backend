CREATE TABLE IF NOT EXISTS trust_interactions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind              TEXT NOT NULL CHECK (kind IN ('lend', 'help')),
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'cancelled')),
    source_post_id    UUID NULL REFERENCES posts(id) ON DELETE SET NULL,
    note              TEXT,
    feedback_note     TEXT,
    positive_feedback BOOLEAN,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ,
    feedback_at       TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT trust_interactions_distinct_users CHECK (provider_user_id <> recipient_user_id)
);

CREATE INDEX IF NOT EXISTS trust_interactions_provider_idx ON trust_interactions (provider_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS trust_interactions_recipient_idx ON trust_interactions (recipient_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS trust_interactions_completed_positive_idx
    ON trust_interactions (provider_user_id, positive_feedback)
    WHERE status = 'completed';

CREATE UNIQUE INDEX IF NOT EXISTS trust_interactions_pending_pair_kind_idx
    ON trust_interactions (provider_user_id, recipient_user_id, kind)
    WHERE status = 'pending';

UPDATE users
SET trust_score = 0
WHERE trust_score <> 0;
