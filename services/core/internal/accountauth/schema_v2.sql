-- Version 2 (ADR-0055 review): version 1 is installed on the prelaunch stack
-- and is never edited in place.

-- Erasure writes discoverable_by_phone=false and deleted_at in one UPDATE; the
-- guard used to copy the managed value back before the AFTER trigger removed
-- the account, leaving an erased person addressable on paper.
CREATE OR REPLACE FUNCTION guard_managed_account_discovery() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE managed_discovery boolean;
BEGIN
 IF NEW.deleted_at IS NULL THEN
  SELECT discoverable INTO managed_discovery FROM managed_accounts WHERE person_id=NEW.id;
  IF FOUND THEN NEW.discoverable_by_phone=managed_discovery; END IF;
 END IF;
 RETURN NEW;
END $$;
UPDATE people SET discoverable_by_phone=false WHERE deleted_at IS NOT NULL AND discoverable_by_phone;

-- A username is fixed for its owner and is not handed to the next person
-- after an erasure, who could otherwise pass for them. Only a digest stays.
CREATE TABLE retired_usernames (
 digest bytea PRIMARY KEY CHECK (octet_length(digest)=32),
 retired_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE FUNCTION refuse_retired_username() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM retired_usernames WHERE digest=sha256(convert_to(NEW.username,'UTF8'))) THEN
  RAISE unique_violation USING MESSAGE='username retired';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER refuse_retired_username BEFORE INSERT OR UPDATE OF username ON managed_accounts
 FOR EACH ROW EXECUTE FUNCTION refuse_retired_username();
CREATE OR REPLACE FUNCTION erase_managed_account_credentials() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
  UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=NEW.id AND revoked_at IS NULL;
  DELETE FROM account_challenges WHERE person_id=NEW.id;
  INSERT INTO retired_usernames(digest)
   SELECT sha256(convert_to(username,'UTF8')) FROM managed_accounts WHERE person_id=NEW.id
   ON CONFLICT DO NOTHING;
  DELETE FROM managed_accounts WHERE person_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;

-- The phone doors are retired; their challenges held an HMAC of a phone
-- number, low entropy enough to guess if the key ever leaked.
DO $$ BEGIN
 IF to_regclass('otp_challenges') IS NOT NULL THEN
  EXECUTE 'DELETE FROM otp_challenges';
 END IF;
END $$;
