-- A new avatar is a wake hint for everyone who may see it. The hint carries
-- only the owner's id; who may see it and which picture is current are read
-- from the tables at delivery time, so a missed NOTIFY loses nothing that a
-- reconnect's resync does not repair.
CREATE FUNCTION avatar_feed_notify() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_notify('avatar_changes', NEW.owner_person_id::text);
 RETURN NULL;
END $$;
CREATE TRIGGER avatar_feed_capture AFTER INSERT ON uploaded_images
 FOR EACH ROW WHEN (NEW.purpose = 'avatar' AND NEW.owner_person_id IS NOT NULL)
 EXECUTE FUNCTION avatar_feed_notify();
