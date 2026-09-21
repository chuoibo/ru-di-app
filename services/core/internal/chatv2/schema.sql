CREATE TABLE chat_v2_devices (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id),
 signing_key bytea NOT NULL CHECK(octet_length(signing_key) = 32),
 revoked_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE chat_v2_conversations (
 context_id uuid PRIMARY KEY REFERENCES contexts(id),
 epoch bigint NOT NULL CHECK(epoch > 0),
 ready boolean NOT NULL DEFAULT false,
 last_sequence bigint NOT NULL DEFAULT 0 CHECK(last_sequence >= 0)
);
-- An approved enrollment/rekey flow must bind an exact membership incarnation.
-- There is deliberately no public provisioning API in this foundation.
CREATE TABLE chat_v2_members (
 context_id uuid NOT NULL REFERENCES chat_v2_conversations(context_id),
 device_id uuid NOT NULL REFERENCES chat_v2_devices(id),
 membership_id uuid NOT NULL REFERENCES memberships(id),
 first_sequence bigint NOT NULL CHECK(first_sequence > 0),
 PRIMARY KEY(context_id, device_id)
);
CREATE TABLE chat_v2_events (
 context_id uuid NOT NULL REFERENCES chat_v2_conversations(context_id),
 sequence bigint NOT NULL CHECK(sequence > 0),
 kind text NOT NULL CHECK(kind IN ('envelope','mark')),
 actor_id uuid NOT NULL REFERENCES people(id),
 body jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(context_id,sequence)
);
CREATE TABLE chat_v2_sends (
 context_id uuid NOT NULL,
 device_id uuid NOT NULL REFERENCES chat_v2_devices(id),
 logical_send_id uuid NOT NULL,
 digest bytea NOT NULL CHECK(octet_length(digest)=32),
 sequence bigint NOT NULL,
 PRIMARY KEY(context_id,device_id,logical_send_id),
 FOREIGN KEY(context_id,sequence) REFERENCES chat_v2_events(context_id,sequence)
);
CREATE TABLE chat_v2_marks (
 context_id uuid NOT NULL,
 device_id uuid NOT NULL,
 delivered bigint NOT NULL DEFAULT 0 CHECK(delivered >= 0),
 read_sequence bigint NOT NULL DEFAULT 0 CHECK(read_sequence >= 0 AND read_sequence <= delivered),
 PRIMARY KEY(context_id,device_id),
 FOREIGN KEY(context_id,device_id) REFERENCES chat_v2_members(context_id,device_id)
);
CREATE TABLE chat_v2_outbox (
 context_id uuid NOT NULL,
 sequence bigint NOT NULL,
 published_at timestamptz,
 PRIMARY KEY(context_id,sequence),
 FOREIGN KEY(context_id,sequence) REFERENCES chat_v2_events(context_id,sequence)
);
CREATE INDEX chat_v2_outbox_pending ON chat_v2_outbox(context_id,sequence) WHERE published_at IS NULL;

-- Legacy membership/device changes invalidate the epoch before they commit.
-- A stale roster must never continue sending on the old encryption epoch.
CREATE FUNCTION chat_v2_invalidate_membership() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP = 'INSERT' THEN
  UPDATE chat_v2_conversations SET ready=false WHERE context_id=NEW.context_id;
 ELSIF TG_OP = 'DELETE' THEN
  UPDATE chat_v2_conversations SET ready=false WHERE context_id=OLD.context_id;
 ELSE
  IF NEW.state IS DISTINCT FROM OLD.state OR NEW.left_at IS DISTINCT FROM OLD.left_at
    OR NEW.person_id IS DISTINCT FROM OLD.person_id OR NEW.context_id IS DISTINCT FROM OLD.context_id THEN
   UPDATE chat_v2_conversations SET ready=false WHERE context_id IN (OLD.context_id,NEW.context_id);
  END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER chat_v2_membership_change AFTER INSERT OR UPDATE OR DELETE ON memberships
 FOR EACH ROW EXECUTE FUNCTION chat_v2_invalidate_membership();
CREATE FUNCTION chat_v2_invalidate_device() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.revoked_at IS DISTINCT FROM OLD.revoked_at OR NEW.signing_key IS DISTINCT FROM OLD.signing_key
    OR NEW.person_id IS DISTINCT FROM OLD.person_id THEN
  UPDATE chat_v2_conversations SET ready=false WHERE context_id IN
   (SELECT context_id FROM chat_v2_members WHERE device_id=OLD.id);
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER chat_v2_device_change AFTER UPDATE ON chat_v2_devices
 FOR EACH ROW EXECUTE FUNCTION chat_v2_invalidate_device();
CREATE FUNCTION chat_v2_invalidate_block() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.state = 'blocked' THEN
  UPDATE chat_v2_conversations cv SET ready=false FROM contexts c
   WHERE c.id=cv.context_id AND c.kind='pair'
   AND EXISTS (SELECT 1 FROM memberships m WHERE m.context_id=c.id AND m.person_id=NEW.requester_id)
   AND EXISTS (SELECT 1 FROM memberships m WHERE m.context_id=c.id AND m.person_id=NEW.addressee_id);
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER chat_v2_block_change AFTER INSERT OR UPDATE ON friend_requests
 FOR EACH ROW EXECUTE FUNCTION chat_v2_invalidate_block();
CREATE FUNCTION chat_v2_invalidate_account() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
  UPDATE chat_v2_conversations SET ready=false WHERE context_id IN
   (SELECT context_id FROM memberships WHERE person_id=OLD.id);
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER chat_v2_account_change AFTER UPDATE ON people
 FOR EACH ROW EXECUTE FUNCTION chat_v2_invalidate_account();
