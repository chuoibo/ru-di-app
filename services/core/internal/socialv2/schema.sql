ALTER TABLE post_comments ADD COLUMN parent_id uuid REFERENCES post_comments(id) ON DELETE CASCADE;
ALTER TABLE post_comments ADD CONSTRAINT social_post_comment_not_self CHECK (parent_id IS NULL OR parent_id <> id);
CREATE INDEX social_post_comments_parent ON post_comments(parent_id,created_at,id) WHERE parent_id IS NOT NULL;

CREATE FUNCTION social_validate_comment_parent() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p_post uuid; p_parent uuid;
BEGIN
 IF NEW.parent_id IS NULL THEN RETURN NEW; END IF;
 SELECT post_id,parent_id INTO p_post,p_parent FROM post_comments WHERE id=NEW.parent_id;
 IF p_post IS NULL OR p_post<>NEW.post_id OR p_parent IS NOT NULL THEN
  RAISE EXCEPTION 'invalid social comment parent' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER social_comment_parent BEFORE INSERT OR UPDATE OF parent_id,post_id ON post_comments
 FOR EACH ROW EXECUTE FUNCTION social_validate_comment_parent();

CREATE TABLE social_comment_likes (
 comment_id uuid NOT NULL REFERENCES post_comments(id) ON DELETE CASCADE,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(comment_id,person_id)
);
CREATE TABLE social_reposts (
 post_id uuid PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
 origin_post_id uuid REFERENCES posts(id) ON DELETE SET NULL,
 CHECK (post_id IS DISTINCT FROM origin_post_id)
);
CREATE INDEX social_reposts_origin ON social_reposts(origin_post_id);

CREATE TABLE social_mutation_keys (
 actor_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 request_key text NOT NULL CHECK(length(request_key)>0 AND length(request_key)<=128),
 kind text NOT NULL CHECK(kind IN ('comment','repost')),
 request_hash bytea NOT NULL,
 result_id uuid,
 PRIMARY KEY(actor_id,request_key)
);

CREATE TABLE social_wall_changes (
 sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
 wall_owner_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('post','comment','like','repost')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX social_wall_changes_wall_seq ON social_wall_changes(wall_owner_id,sequence);

CREATE FUNCTION social_capture_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE item uuid; wall_owner uuid; category text;
BEGIN
 IF TG_TABLE_NAME='posts' THEN
  IF TG_OP='DELETE' THEN RETURN NULL; END IF;
  item:=NEW.id; wall_owner:=NEW.author_id; category:='post';
 ELSIF TG_TABLE_NAME='post_comments' THEN
  item:=CASE WHEN TG_OP='DELETE' THEN OLD.post_id ELSE NEW.post_id END; category:='comment';
 ELSIF TG_TABLE_NAME='post_reactions' THEN
  item:=CASE WHEN TG_OP='DELETE' THEN OLD.post_id ELSE NEW.post_id END; category:='like';
 ELSIF TG_TABLE_NAME='social_comment_likes' THEN
  SELECT post_id INTO item FROM post_comments WHERE id=CASE WHEN TG_OP='DELETE' THEN OLD.comment_id ELSE NEW.comment_id END;
  category:='like';
 ELSE
  item:=CASE WHEN TG_OP='DELETE' THEN OLD.post_id ELSE NEW.post_id END; category:='repost';
 END IF;
 IF item IS NULL THEN RETURN NULL; END IF;
 IF wall_owner IS NULL THEN SELECT author_id INTO wall_owner FROM posts WHERE id=item; END IF;
 IF wall_owner IS NULL THEN RETURN NULL; END IF;
 INSERT INTO social_wall_changes(wall_owner_id,post_id,kind) VALUES(wall_owner,item,category);
 PERFORM pg_notify('social_wall_changes',wall_owner::text);
 RETURN NULL;
END $$;
CREATE TRIGGER social_post_change AFTER INSERT OR UPDATE ON posts FOR EACH ROW EXECUTE FUNCTION social_capture_change();
CREATE TRIGGER social_comment_change AFTER INSERT OR UPDATE OR DELETE ON post_comments FOR EACH ROW EXECUTE FUNCTION social_capture_change();
CREATE TRIGGER social_reaction_change AFTER INSERT OR DELETE ON post_reactions FOR EACH ROW EXECUTE FUNCTION social_capture_change();
CREATE TRIGGER social_comment_like_change AFTER INSERT OR DELETE ON social_comment_likes FOR EACH ROW EXECUTE FUNCTION social_capture_change();
CREATE TRIGGER social_repost_change AFTER INSERT OR DELETE ON social_reposts FOR EACH ROW EXECUTE FUNCTION social_capture_change();
