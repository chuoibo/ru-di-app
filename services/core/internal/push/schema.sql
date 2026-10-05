-- ADR-0024/0041 §11/0057 §7: content-free push. One writer: package push.
-- A device is one app installation of one person under one sign-in session;
-- its Expo token never changes owner between two installations.
CREATE TABLE IF NOT EXISTS push_devices (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id),
 session_id uuid NOT NULL REFERENCES account_sessions(id),
 installation_id uuid NOT NULL UNIQUE,
 platform text NOT NULL CHECK (platform IN ('android','ios')),
 expo_push_token text NOT NULL UNIQUE CHECK (char_length(expo_push_token) BETWEEN 10 AND 200),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 last_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 revoked_at timestamptz
);
CREATE INDEX IF NOT EXISTS push_devices_person_live ON push_devices(person_id) WHERE revoked_at IS NULL;

-- What to wake whom for: one pending row per person per conversation (the
-- latest sequence wins); sent rows are kept a day for idempotency, then purged.
CREATE TABLE IF NOT EXISTS push_outbox (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id),
 conversation_id uuid NOT NULL,
 sequence bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 sent_at timestamptz,
 attempts integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS push_outbox_pending ON push_outbox(person_id, conversation_id) WHERE sent_at IS NULL;

-- How far the worker has read each conversation's chat v2 log.
CREATE TABLE IF NOT EXISTS push_chat_cursor (
 conversation_id uuid PRIMARY KEY,
 sequence bigint NOT NULL DEFAULT 0
);

-- A revoked session, a deleted account: their devices stop receiving pushes.
CREATE OR REPLACE FUNCTION push_revoke_on_session() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.revoked_at IS NOT NULL AND OLD.revoked_at IS NULL THEN
  UPDATE push_devices SET revoked_at = clock_timestamp() WHERE session_id = NEW.id AND revoked_at IS NULL;
 END IF;
 RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS push_session_revoked ON account_sessions;
CREATE TRIGGER push_session_revoked AFTER UPDATE ON account_sessions
 FOR EACH ROW EXECUTE FUNCTION push_revoke_on_session();
CREATE OR REPLACE FUNCTION push_erase_on_account_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
  DELETE FROM push_outbox WHERE person_id = NEW.id;
  DELETE FROM push_devices WHERE person_id = NEW.id;
 END IF;
 RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS push_account_deleted ON people;
CREATE TRIGGER push_account_deleted AFTER UPDATE ON people
 FOR EACH ROW EXECUTE FUNCTION push_erase_on_account_deletion();
