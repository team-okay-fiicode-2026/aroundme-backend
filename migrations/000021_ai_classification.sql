-- Backfill existing categories before tightening the constraint
UPDATE posts SET category = 'request' WHERE category = 'skill';
UPDATE posts SET category = 'event' WHERE category = 'community';

-- Update category constraint to the new taxonomy
ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_category_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_category_check
    CHECK (category IN ('uncategorized', 'emergency', 'request', 'offer', 'item', 'event'));

DROP INDEX IF EXISTS posts_category_status_created_at_idx;
CREATE INDEX posts_category_status_created_at_idx
    ON posts (category, status, created_at DESC);

-- AI classification audit columns
ALTER TABLE posts
    ADD COLUMN IF NOT EXISTS ai_post_type TEXT
        CHECK (ai_post_type IN ('emergency', 'request', 'offer', 'item', 'event')),
    ADD COLUMN IF NOT EXISTS ai_urgency TEXT
        CHECK (ai_urgency IN ('critical', 'high', 'normal')),
    ADD COLUMN IF NOT EXISTS ai_confidence REAL,
    ADD COLUMN IF NOT EXISTS ai_rationale TEXT,
    ADD COLUMN IF NOT EXISTS ai_classified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS ai_classification_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (ai_classification_status IN ('pending', 'classified', 'failed')),
    ADD COLUMN IF NOT EXISTS override_post_type TEXT
        CHECK (override_post_type IN ('emergency', 'request', 'offer', 'item', 'event')),
    ADD COLUMN IF NOT EXISTS override_urgency TEXT
        CHECK (override_urgency IN ('critical', 'high', 'normal')),
    ADD COLUMN IF NOT EXISTS overridden_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS overridden_by UUID REFERENCES users(id);

CREATE INDEX IF NOT EXISTS posts_classification_pending_idx
    ON posts (created_at ASC)
    WHERE ai_classification_status = 'pending';
