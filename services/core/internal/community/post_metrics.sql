-- These derived counters have one writer: canonical reaction/comment triggers.
-- The original rows remain the source of truth and allow complete rebuilding.
CREATE TABLE community_post_metrics (
 post_id uuid PRIMARY KEY REFERENCES community_posts(post_id) ON DELETE CASCADE,
 likes bigint NOT NULL DEFAULT 0 CHECK(likes >= 0),
 comments bigint NOT NULL DEFAULT 0 CHECK(comments >= 0)
);
INSERT INTO community_post_metrics(post_id,likes,comments)
 SELECT c.post_id,(SELECT count(*) FROM post_reactions WHERE post_id=c.post_id AND kind='heart'),
 (SELECT count(*) FROM post_comments WHERE post_id=c.post_id) FROM community_posts c;
CREATE FUNCTION community_initialize_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO community_post_metrics(post_id,likes,comments)
 SELECT NEW.post_id,(SELECT count(*) FROM post_reactions WHERE post_id=NEW.post_id AND kind='heart'),
 (SELECT count(*) FROM post_comments WHERE post_id=NEW.post_id);
 RETURN NEW;
END $$;
CREATE TRIGGER community_metrics_created AFTER INSERT ON community_posts FOR EACH ROW EXECUTE FUNCTION community_initialize_metrics();
CREATE OR REPLACE FUNCTION community_social_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE pid uuid; delta integer;
BEGIN
 IF TG_OP='DELETE' THEN pid:=OLD.post_id; delta:=-1; ELSE pid:=NEW.post_id; delta:=1; END IF;
 IF EXISTS(SELECT 1 FROM community_posts WHERE post_id=pid) THEN
  PERFORM community_emit(pid,'post.changed');
  IF TG_TABLE_NAME='post_reactions' THEN
   IF (TG_OP='INSERT' AND NEW.kind='heart') OR (TG_OP='DELETE' AND OLD.kind='heart') THEN
    UPDATE community_post_metrics SET likes=likes+delta WHERE post_id=pid;
   END IF;
  ELSE
   UPDATE community_post_metrics SET comments=comments+delta WHERE post_id=pid;
  END IF;
 END IF;
 RETURN COALESCE(NEW,OLD);
END $$;
