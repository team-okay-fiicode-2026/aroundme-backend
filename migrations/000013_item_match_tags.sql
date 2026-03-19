ALTER TABLE user_items
    ADD COLUMN IF NOT EXISTS match_tags TEXT[] NOT NULL DEFAULT '{}';

UPDATE user_items
SET match_tags = CASE category
    WHEN 'tools' THEN ARRAY['tools', 'repair']::TEXT[]
    WHEN 'clothing' THEN ARRAY['clothing']::TEXT[]
    WHEN 'books' THEN ARRAY['books']::TEXT[]
    WHEN 'sports' THEN ARRAY['sports']::TEXT[]
    WHEN 'kitchen' THEN ARRAY['food']::TEXT[]
    WHEN 'home' THEN ARRAY['shelter']::TEXT[]
    WHEN 'garden' THEN ARRAY['garden', 'cleanup']::TEXT[]
    ELSE ARRAY[]::TEXT[]
END
WHERE COALESCE(array_length(match_tags, 1), 0) = 0;
