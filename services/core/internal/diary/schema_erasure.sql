CREATE FUNCTION diary_erase_for_account() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 DELETE FROM outing_diary_jobs WHERE owner_id=NEW.id;
 DELETE FROM outing_diaries WHERE owner_id=NEW.id;
 RETURN NEW;
END;
$$;
CREATE TRIGGER diary_erase_for_account
 AFTER UPDATE OF deleted_at ON people
 FOR EACH ROW WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
 EXECUTE FUNCTION diary_erase_for_account();

-- Apply the same retention rule to accounts erased before this migration.
DELETE FROM outing_diary_jobs WHERE owner_id IN (SELECT id FROM people WHERE deleted_at IS NOT NULL);
DELETE FROM outing_diaries WHERE owner_id IN (SELECT id FROM people WHERE deleted_at IS NOT NULL);
