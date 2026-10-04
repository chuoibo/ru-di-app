-- Account deletion forgets who mentioned whom (found after B7 by the full
-- PostgreSQL tier: nepnho's person-column gate saw community_notifications.
-- actor_id, added in notification_source.sql, with no erasure answer). The
-- people row is kept with deleted_at, so the foreign key's ON DELETE SET NULL
-- never fires; the erasure trigger sets it instead. The function is the one
-- schema.sql created, with that one line added.
CREATE OR REPLACE FUNCTION community_person_erasure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
  DELETE FROM community_comment_drafts WHERE author_id=NEW.id;
  DELETE FROM community_posts WHERE post_id IN(SELECT id FROM posts WHERE author_id=NEW.id);
  DELETE FROM community_media WHERE owner_id=NEW.id;
  DELETE FROM community_preferences WHERE person_id=NEW.id;
  DELETE FROM community_keeps WHERE person_id=NEW.id;
  DELETE FROM community_feeds WHERE person_id=NEW.id;
  DELETE FROM community_interactions WHERE person_id=NEW.id;
  DELETE FROM community_follows WHERE person_id=NEW.id OR (kind='person' AND target=NEW.id::text);
  DELETE FROM community_feedback WHERE person_id=NEW.id;
  DELETE FROM community_notifications WHERE person_id=NEW.id;
  DELETE FROM community_idempotency WHERE person_id=NEW.id;
  DELETE FROM community_limits WHERE person_id=NEW.id;
  DELETE FROM community_moderators WHERE person_id=NEW.id;
  UPDATE community_audit SET actor_id=NULL WHERE actor_id=NEW.id;
  -- Notifications other people received keep their row; the deleted
  -- person stops being named as who mentioned them.
  UPDATE community_notifications SET actor_id=NULL WHERE actor_id=NEW.id;
  PERFORM community_emit(NULL,'access.changed');
 END IF;
 RETURN NEW;
END $$;
