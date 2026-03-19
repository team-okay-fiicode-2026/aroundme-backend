-- Add trust score to users.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS trust_score INTEGER NOT NULL DEFAULT 0;

-- Track who endorsed whom; one endorsement per (from, to) pair.
CREATE TABLE IF NOT EXISTS trust_endorsements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note         TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT trust_endorsements_pair_key UNIQUE (from_user_id, to_user_id)
);

CREATE INDEX IF NOT EXISTS trust_endorsements_to_user_id_idx ON trust_endorsements (to_user_id);
