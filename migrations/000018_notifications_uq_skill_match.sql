ALTER TABLE notifications
  ADD CONSTRAINT uq_notifications_user_post
  UNIQUE (user_id, entity_id)
  WHERE type = 'skill_match';
