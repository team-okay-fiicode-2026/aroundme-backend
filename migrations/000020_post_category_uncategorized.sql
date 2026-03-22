ALTER TABLE posts
    DROP CONSTRAINT IF EXISTS posts_category_check;

UPDATE posts
SET category = 'uncategorized';

ALTER TABLE posts
    ALTER COLUMN category SET DEFAULT 'uncategorized',
    ALTER COLUMN category SET NOT NULL;

ALTER TABLE posts
    ADD CONSTRAINT posts_category_check CHECK (category IN ('uncategorized', 'emergency', 'skill', 'item', 'community'));
