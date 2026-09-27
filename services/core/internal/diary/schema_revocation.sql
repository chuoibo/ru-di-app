-- Consent belongs to this continuous membership, not a future rejoin.
CREATE FUNCTION outing_diary_revoke_sharing() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (OLD.state='active' AND NEW.state<>'active') OR (OLD.left_at IS NULL AND NEW.left_at IS NOT NULL) THEN
  UPDATE outing_diary_jobs j SET source=NULL,result=NULL,status='failed',code='sharing_revoked',lease_id=NULL,lease_until=NULL
  FROM outings o WHERE o.id=j.outing_id AND o.context_id=NEW.context_id AND j.owner_id=NEW.person_id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER outing_diary_membership_revoked AFTER UPDATE OF state,left_at ON memberships
 FOR EACH ROW EXECUTE FUNCTION outing_diary_revoke_sharing();
