CREATE TABLE managed_accounts (
 person_id uuid PRIMARY KEY REFERENCES people(id),
 username text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9._]{3,32}$'),
 email_digest bytea UNIQUE CHECK (email_digest IS NULL OR octet_length(email_digest)=32),
 email_cipher bytea,
 password_hash text,
 google_issuer text,
 google_subject text,
 discoverable boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (google_issuer,google_subject),
 CHECK ((email_digest IS NULL) = (email_cipher IS NULL)),
 CHECK ((google_issuer IS NULL) = (google_subject IS NULL)),
 CHECK (password_hash IS NULL OR email_digest IS NOT NULL),
 CHECK (password_hash IS NOT NULL OR google_subject IS NOT NULL)
);
ALTER TABLE account_sessions ADD COLUMN reauthenticated_at timestamptz;
-- Preserve historical provenance; new local sessions have no invitation.
DO $$ DECLARE c text; BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='account_sessions'::regclass AND contype='c'
  AND pg_get_constraintdef(oid) LIKE '%issued_via%IN%' LOOP
  EXECUTE format('ALTER TABLE account_sessions DROP CONSTRAINT %I',c);
 END LOOP;
 -- Alembic renders IN as = ANY on some PostgreSQL versions.
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='account_sessions'::regclass AND contype='c'
  AND pg_get_constraintdef(oid) LIKE '%issued_via%ANY%' LOOP
  EXECUTE format('ALTER TABLE account_sessions DROP CONSTRAINT %I',c);
 END LOOP;
END $$;
ALTER TABLE account_sessions ADD CONSTRAINT managed_session_via CHECK (issued_via IN ('invite','otp','google','genesis','password'));
CREATE TABLE account_challenges (
 id uuid PRIMARY KEY,
 kind text NOT NULL CHECK(kind IN ('register','reset','email','google','google_register')),
 subject_digest bytea NOT NULL CHECK(octet_length(subject_digest)=32),
 binding_digest bytea NOT NULL CHECK(octet_length(binding_digest)=32),
 code_digest bytea,
 payload_cipher bytea NOT NULL,
 person_id uuid REFERENCES people(id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 consumed_at timestamptz,
 CHECK(expires_at > created_at)
);
CREATE INDEX account_challenges_subject ON account_challenges(kind,subject_digest,created_at DESC);
CREATE INDEX account_challenges_cleanup ON account_challenges(expires_at);
CREATE TABLE account_mail_outbox (
 id uuid PRIMARY KEY,
 challenge_id uuid NOT NULL REFERENCES account_challenges(id) ON DELETE CASCADE,
 payload_cipher bytea NOT NULL,
 expires_at timestamptz NOT NULL,
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 attempts integer NOT NULL DEFAULT 0,
 lease_id uuid,
 lease_until timestamptz,
 done_at timestamptz
);
CREATE INDEX account_mail_due ON account_mail_outbox(next_attempt_at) WHERE done_at IS NULL;
-- Prelaunch retirement: historical people/ledger stay; legacy credentials stop working.
UPDATE account_sessions SET revoked_at=clock_timestamp()
 WHERE revoked_at IS NULL AND NOT EXISTS(SELECT 1 FROM managed_accounts m WHERE m.person_id=account_sessions.person_id);
DELETE FROM account_identities WHERE provider='phone';
CREATE FUNCTION erase_managed_account_credentials() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
  UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=NEW.id AND revoked_at IS NULL;
  DELETE FROM account_challenges WHERE person_id=NEW.id;
  DELETE FROM managed_accounts WHERE person_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER erase_managed_account_credentials AFTER UPDATE OF deleted_at ON people
 FOR EACH ROW EXECUTE FUNCTION erase_managed_account_credentials();
-- Legacy social readers use this column as an addressability flag. It contains
-- no phone identity; managed_accounts is its only writer for new accounts.
CREATE FUNCTION sync_managed_account_discovery() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE people SET discoverable_by_phone=NEW.discoverable WHERE id=NEW.person_id;
 RETURN NEW;
END $$;
CREATE TRIGGER sync_managed_account_discovery AFTER INSERT OR UPDATE OF discoverable ON managed_accounts
 FOR EACH ROW EXECUTE FUNCTION sync_managed_account_discovery();
CREATE FUNCTION guard_managed_account_discovery() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE managed_discovery boolean;
BEGIN
 SELECT discoverable INTO managed_discovery FROM managed_accounts WHERE person_id=NEW.id;
 IF FOUND THEN NEW.discoverable_by_phone=managed_discovery; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_managed_account_discovery BEFORE UPDATE OF discoverable_by_phone ON people
 FOR EACH ROW EXECUTE FUNCTION guard_managed_account_discovery();
