-- Nested replies on post comments
ALTER TABLE post_comments ADD COLUMN parent_id UUID REFERENCES post_comments(id) ON DELETE CASCADE;
CREATE INDEX idx_post_comments_parent ON post_comments(parent_id) WHERE parent_id IS NOT NULL;

-- Upvotes on comments
CREATE TABLE post_comment_reactions (
    comment_id UUID NOT NULL REFERENCES post_comments(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (comment_id, user_id)
);
CREATE INDEX idx_comment_reactions_comment ON post_comment_reactions(comment_id);

ALTER TABLE post_comments ADD COLUMN reaction_count INT NOT NULL DEFAULT 0;
