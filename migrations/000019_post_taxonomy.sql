ALTER TABLE posts
    ADD COLUMN category TEXT;

UPDATE posts
SET
    category = 'uncategorized'
WHERE category IS NULL;

ALTER TABLE posts
    ALTER COLUMN category SET DEFAULT 'uncategorized',
    ALTER COLUMN category SET NOT NULL;

ALTER TABLE posts
    ADD CONSTRAINT posts_category_check CHECK (category IN ('uncategorized', 'emergency', 'skill', 'item', 'community'));

CREATE INDEX IF NOT EXISTS posts_category_status_created_at_idx
    ON posts (category, status, created_at DESC);
