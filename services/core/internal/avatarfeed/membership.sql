-- Who may see whose avatar changes when someone enters or leaves an active
-- membership. The hint names the context and the person; delivery tells every
-- active member of that context, and the person, to ask for versions again,
-- so an answer cached as "you may not see this" is never left standing.
CREATE FUNCTION avatar_feed_membership() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE was_active boolean := false; is_active boolean := false; room uuid; person uuid;
BEGIN
 IF TG_OP <> 'INSERT' THEN was_active := OLD.state = 'active'; END IF;
 IF TG_OP <> 'DELETE' THEN is_active := NEW.state = 'active'; END IF;
 IF was_active = is_active AND (TG_OP <> 'UPDATE' OR (NEW.context_id = OLD.context_id AND NEW.person_id = OLD.person_id)) THEN
  RETURN NULL;
 END IF;
 IF TG_OP = 'DELETE' THEN room := OLD.context_id; person := OLD.person_id;
 ELSE room := NEW.context_id; person := NEW.person_id; END IF;
 PERFORM pg_notify('avatar_changes', 'm:' || room::text || ':' || person::text);
 IF TG_OP = 'UPDATE' AND (NEW.context_id <> OLD.context_id OR NEW.person_id <> OLD.person_id) THEN
  PERFORM pg_notify('avatar_changes', 'm:' || OLD.context_id::text || ':' || OLD.person_id::text);
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER avatar_feed_membership AFTER INSERT OR UPDATE OR DELETE ON memberships
 FOR EACH ROW EXECUTE FUNCTION avatar_feed_membership();
