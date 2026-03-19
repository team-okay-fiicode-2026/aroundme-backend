-- Add a stable canonical pair key to direct conversations so that
-- GetOrCreateDirectConversation can use INSERT ... ON CONFLICT to eliminate
-- the SELECT-then-INSERT race condition when two requests arrive simultaneously.
--
-- The value is LEAST(user_a, user_b) || ':' || GREATEST(user_a, user_b) so
-- it is always the same regardless of which user initiates the conversation.

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS direct_pair TEXT;

-- Backfill existing direct conversations.
UPDATE conversations c
SET direct_pair = (
    SELECT LEAST(cp1.user_id, cp2.user_id) || ':' || GREATEST(cp1.user_id, cp2.user_id)
    FROM conversation_participants cp1
    JOIN conversation_participants cp2
      ON cp2.conversation_id = cp1.conversation_id
     AND cp2.user_id > cp1.user_id
    WHERE cp1.conversation_id = c.id
    LIMIT 1
)
WHERE c.kind = 'direct';

CREATE UNIQUE INDEX IF NOT EXISTS conversations_direct_pair_idx
    ON conversations (direct_pair)
    WHERE direct_pair IS NOT NULL;
