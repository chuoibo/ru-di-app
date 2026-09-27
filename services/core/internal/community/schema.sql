CREATE TABLE community_moderators (
 person_id uuid PRIMARY KEY REFERENCES people(id) ON DELETE CASCADE
);
CREATE TABLE community_posts (
 post_id uuid PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 published_revision integer,
 requested_audience text NOT NULL CHECK(requested_audience IN ('only_me','friends','group','public')),
 status text NOT NULL CHECK(status IN ('private','pending','approved','rejected','review')),
 reason text NOT NULL DEFAULT '',
 topics text[] NOT NULL DEFAULT '{}',
 mentions uuid[] NOT NULL DEFAULT '{}',
 diary_document jsonb,
 diary_kind text,
 diary_id uuid,
 diary_revision integer,
 deleted_at timestamptz,
 published_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX community_feed ON community_posts(published_at DESC,post_id DESC) WHERE status='approved' AND deleted_at IS NULL;
CREATE INDEX community_topics ON community_posts USING gin(topics);
CREATE TABLE community_revisions (
 post_id uuid NOT NULL REFERENCES community_posts(post_id) ON DELETE CASCADE,
 revision integer NOT NULL,
 body text NOT NULL,
 topics text[] NOT NULL,
 mentions uuid[] NOT NULL,
 media_ids uuid[] NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(post_id,revision)
);
CREATE TABLE community_media (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 storage_key text NOT NULL,
 content_type text NOT NULL,
 byte_size bigint NOT NULL,
 width integer NOT NULL DEFAULT 0,
 height integer NOT NULL DEFAULT 0,
 duration_ms integer NOT NULL DEFAULT 0,
 state text NOT NULL CHECK(state IN ('ready','processing','failed')),
 lease_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE community_comment_meta (
 comment_id uuid PRIMARY KEY REFERENCES post_comments(id) ON DELETE CASCADE,
 parent_id uuid REFERENCES post_comments(id) ON DELETE SET NULL,
 mentions uuid[] NOT NULL DEFAULT '{}',
 media_id uuid REFERENCES community_media(id) ON DELETE SET NULL
);
CREATE TABLE community_comment_drafts (
 id uuid PRIMARY KEY,
 post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 author_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 body text NOT NULL,
 parent_id uuid,
 mentions uuid[] NOT NULL DEFAULT '{}',
 media_id uuid REFERENCES community_media(id) ON DELETE SET NULL,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected','review')),
 reason text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE community_jobs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 post_id uuid REFERENCES community_posts(post_id) ON DELETE CASCADE,
 revision integer,
 comment_id uuid REFERENCES community_comment_drafts(id) ON DELETE CASCADE,
 attempts integer NOT NULL DEFAULT 0,
 lease_until timestamptz,
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 done boolean NOT NULL DEFAULT false,
 CHECK((post_id IS NOT NULL) <> (comment_id IS NOT NULL))
);
CREATE INDEX community_jobs_ready ON community_jobs(available_at) WHERE NOT done;
CREATE TABLE community_follows (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('person','topic')),
 target text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(person_id,kind,target)
);
CREATE TABLE community_preferences (
 person_id uuid PRIMARY KEY REFERENCES people(id) ON DELETE CASCADE,
 personalized boolean NOT NULL DEFAULT false,
 asked boolean NOT NULL DEFAULT false
);
CREATE TABLE community_keeps (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 post_id uuid REFERENCES posts(id) ON DELETE SET NULL,
 body text NOT NULL CHECK(length(body) BETWEEN 1 AND 5000),
 ai_generated boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE community_feeds (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 mode text NOT NULL,
 post_ids uuid[] NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL DEFAULT clock_timestamp()+interval '30 minutes'
);
CREATE TABLE community_feedback (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('saved','hidden')),
 PRIMARY KEY(person_id,post_id,kind)
);
CREATE TABLE community_interactions (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('impression','view','skip','complete')),
 dwell_ms integer NOT NULL CHECK(dwell_ms BETWEEN 0 AND 180000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX community_interactions_person ON community_interactions(person_id,created_at DESC);
CREATE INDEX community_interactions_post ON community_interactions(post_id,created_at DESC);
CREATE TABLE community_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 post_id uuid,
 revision integer,
 actor_id uuid REFERENCES people(id) ON DELETE SET NULL,
 action text NOT NULL,
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE community_notifications (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 kind text NOT NULL,
 read_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX community_notifications_person ON community_notifications(person_id,created_at DESC);
CREATE TABLE community_idempotency (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 logical_id uuid NOT NULL,
 digest text NOT NULL,
 resource_id uuid NOT NULL,
 PRIMARY KEY(person_id,logical_id)
);
CREATE TABLE community_limits (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 minute timestamptz NOT NULL,
 count integer NOT NULL,
 PRIMARY KEY(person_id,minute)
);
-- Commit-ordered IDs: the transaction lock prevents late commits behind cursors.
CREATE TABLE community_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 post_id uuid,
 kind text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 relayed_at timestamptz
);
CREATE INDEX community_events_relay ON community_events(id) WHERE relayed_at IS NULL;
CREATE TABLE community_media_gc(storage_key text PRIMARY KEY);
CREATE OR REPLACE FUNCTION community_queue_media_gc() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO community_media_gc VALUES(OLD.storage_key) ON CONFLICT DO NOTHING;
 RETURN OLD;
END $$;
CREATE TRIGGER community_media_gc BEFORE DELETE ON community_media FOR EACH ROW EXECUTE FUNCTION community_queue_media_gc();
-- Legacy comment endpoints must not bypass moderation on community public posts.
CREATE OR REPLACE FUNCTION community_require_comment_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM community_posts c JOIN posts p ON p.id=c.post_id WHERE p.id=NEW.post_id AND p.audience='public')
 AND NOT EXISTS(SELECT 1 FROM community_comment_drafts d WHERE d.id=NEW.id AND d.post_id=NEW.post_id AND d.author_id=NEW.author_id AND d.body=NEW.body AND d.status='approved') THEN
  RAISE EXCEPTION 'community_comment_requires_review' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER community_comment_review BEFORE INSERT ON post_comments FOR EACH ROW EXECUTE FUNCTION community_require_comment_review();
CREATE OR REPLACE FUNCTION community_emit(pid uuid, event_kind text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(73413801);
 INSERT INTO community_events(post_id,kind) VALUES(pid,event_kind);
END $$;
CREATE OR REPLACE FUNCTION community_social_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE pid uuid;
BEGIN
 IF TG_OP='DELETE' THEN pid:=OLD.post_id; ELSE pid:=NEW.post_id; END IF;
 IF EXISTS(SELECT 1 FROM community_posts WHERE post_id=pid) THEN
  PERFORM community_emit(pid,'post.changed');
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER community_reactions_changed AFTER INSERT OR DELETE ON post_reactions FOR EACH ROW EXECUTE FUNCTION community_social_changed();
CREATE TRIGGER community_comments_changed AFTER INSERT OR DELETE ON post_comments FOR EACH ROW EXECUTE FUNCTION community_social_changed();
CREATE OR REPLACE FUNCTION community_access_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM community_emit(NULL,'access.changed');
 RETURN NULL;
END $$;
CREATE TRIGGER community_friend_access AFTER INSERT OR UPDATE OR DELETE ON friend_requests FOR EACH STATEMENT EXECUTE FUNCTION community_access_changed();
CREATE TRIGGER community_session_access AFTER UPDATE OR DELETE ON account_sessions FOR EACH STATEMENT EXECUTE FUNCTION community_access_changed();
CREATE TRIGGER community_member_access AFTER UPDATE OR DELETE ON memberships FOR EACH STATEMENT EXECUTE FUNCTION community_access_changed();
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
  PERFORM community_emit(NULL,'access.changed');
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER community_erase AFTER UPDATE OF deleted_at ON people FOR EACH ROW EXECUTE FUNCTION community_person_erasure();
CREATE OR REPLACE FUNCTION community_diary_revoked() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE did uuid;
BEGIN
 IF TG_OP='DELETE' THEN did:=OLD.id; ELSE did:=NEW.id; END IF;
 UPDATE posts SET audience='only_me',context_id=NULL WHERE id IN(SELECT post_id FROM community_posts WHERE diary_id=did);
 UPDATE community_posts SET status='private',requested_audience='only_me',published_at=NULL WHERE diary_id=did;
 PERFORM community_emit(NULL,'access.changed');
 RETURN NULL;
END $$;
CREATE TRIGGER community_diary_revoked AFTER UPDATE OF revision,audience OR DELETE ON outing_diaries FOR EACH ROW EXECUTE FUNCTION community_diary_revoked();

-- Legacy writers cannot change a moderated edition behind the Go module.
CREATE OR REPLACE FUNCTION community_guard_edition() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM community_posts WHERE post_id=OLD.id)
 AND current_setting('rudi.community_writer',true) IS DISTINCT FROM 'on'
 AND pg_trigger_depth()=1 THEN
  RAISE EXCEPTION 'community_endpoint_required' USING ERRCODE='check_violation';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER community_guard_edition BEFORE UPDATE OF body,audience,context_id,image_url ON posts FOR EACH ROW EXECUTE FUNCTION community_guard_edition();
