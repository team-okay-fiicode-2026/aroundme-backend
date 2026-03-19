ALTER TABLE posts ADD COLUMN IF NOT EXISTS ai_tagged_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS posts_ai_tagging_idx
    ON posts (created_at ASC)
    WHERE ai_tagged_at IS NULL;
