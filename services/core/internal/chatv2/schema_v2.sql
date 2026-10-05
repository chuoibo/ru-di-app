-- ADR-0057: device enrollment, key packages, the Welcome mailbox and commits.
-- The server stores public keys and opaque MLS bytes only; it never holds a
-- private key or a plaintext.

-- The MLS signature key beside the transport key (signing_key), and the
-- device's proof that it holds the transport key for this person and id.
ALTER TABLE chat_v2_devices
 ADD COLUMN mls_signature_key bytea CHECK(octet_length(mls_signature_key) = 32),
 ADD COLUMN enrollment_proof bytea CHECK(octet_length(enrollment_proof) = 64),
 ADD COLUMN label text CHECK(char_length(label) <= 60);
CREATE UNIQUE INDEX chat_v2_devices_mls_key ON chat_v2_devices(mls_signature_key) WHERE mls_signature_key IS NOT NULL;
CREATE UNIQUE INDEX chat_v2_devices_transport_key ON chat_v2_devices(signing_key) WHERE mls_signature_key IS NOT NULL;
CREATE INDEX chat_v2_devices_person_live ON chat_v2_devices(person_id) WHERE revoked_at IS NULL;

-- One-time MLS key packages a device publishes for others to add it.
CREATE TABLE chat_v2_key_packages (
 id uuid PRIMARY KEY,
 device_id uuid NOT NULL REFERENCES chat_v2_devices(id) ON DELETE CASCADE,
 key_package bytea NOT NULL CHECK(octet_length(key_package) BETWEEN 1 AND 65536),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 -- A package handed to one adding device is that device's for good: it is
 -- never handed to anyone else (RFC 9420: a key package is used once), a
 -- repeated claim by the same device returns it, and the commit that adds the
 -- target consumes it. One adding device holds at most one package per target,
 -- so claiming without adding cannot drain the target.
 claimed_by uuid REFERENCES chat_v2_devices(id),
 claimed_at timestamptz,
 CHECK ((claimed_by IS NULL) = (claimed_at IS NULL))
);
CREATE INDEX chat_v2_key_packages_device ON chat_v2_key_packages(device_id, created_at);

-- A Welcome for one added device, kept until that device acknowledges it.
CREATE TABLE chat_v2_welcomes (
 id uuid PRIMARY KEY,
 device_id uuid NOT NULL REFERENCES chat_v2_devices(id) ON DELETE CASCADE,
 context_id uuid NOT NULL REFERENCES chat_v2_conversations(context_id),
 sequence bigint NOT NULL,
 welcome bytea NOT NULL CHECK(octet_length(welcome) BETWEEN 1 AND 262144),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(context_id, sequence) REFERENCES chat_v2_events(context_id, sequence)
);
CREATE INDEX chat_v2_welcomes_device ON chat_v2_welcomes(device_id, created_at);

-- A commit is an event of the conversation's log like an envelope.
ALTER TABLE chat_v2_events DROP CONSTRAINT chat_v2_events_kind_check;
ALTER TABLE chat_v2_events ADD CONSTRAINT chat_v2_events_kind_check CHECK (kind IN ('envelope','mark','commit'));

-- A revoked device leaves nothing to claim and nothing to read.
CREATE FUNCTION chat_v2_revoke_device_material() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.revoked_at IS NOT NULL AND OLD.revoked_at IS NULL THEN
  DELETE FROM chat_v2_key_packages WHERE device_id = NEW.id OR claimed_by = NEW.id;
  DELETE FROM chat_v2_welcomes WHERE device_id = NEW.id;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER chat_v2_device_revoked AFTER UPDATE ON chat_v2_devices
 FOR EACH ROW EXECUTE FUNCTION chat_v2_revoke_device_material();

-- Account deletion revokes every device of the person, which in turn erases
-- their key packages and pending Welcomes (above) and clears `ready` of each
-- of their rooms (chat_v2_invalidate_device). Device rows stay: the room
-- logs' foreign keys point at them, and they hold public keys only.
CREATE FUNCTION chat_v2_revoke_on_account_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
  UPDATE chat_v2_devices SET revoked_at = clock_timestamp()
   WHERE person_id = NEW.id AND revoked_at IS NULL;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER chat_v2_account_deleted AFTER UPDATE ON people
 FOR EACH ROW EXECUTE FUNCTION chat_v2_revoke_on_account_deletion();
