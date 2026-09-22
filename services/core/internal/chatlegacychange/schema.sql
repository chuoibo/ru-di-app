CREATE TABLE chat_legacy_change_heads (
 context_id uuid PRIMARY KEY REFERENCES contexts(id) ON DELETE CASCADE,
 sequence bigint NOT NULL DEFAULT 0 CHECK (sequence >= 0)
);
CREATE TABLE chat_legacy_changes (
 context_id uuid NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
 sequence bigint NOT NULL CHECK(sequence > 0),
 entity_type text NOT NULL CHECK(entity_type IN ('message','vote')),
 entity_id uuid NOT NULL,
 revision bigint NOT NULL CHECK(revision = sequence),
 deleted boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(context_id,sequence)
);
CREATE INDEX chat_legacy_changes_entity ON chat_legacy_changes(context_id,entity_type,entity_id,sequence DESC);
-- The log is also the durable outbox. Every subscriber resumes independently;
-- publication is only a wake hint, never a delivery acknowledgement.
CREATE TABLE chat_legacy_change_outbox (
 context_id uuid NOT NULL,
 sequence bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(context_id,sequence),
 FOREIGN KEY(context_id,sequence) REFERENCES chat_legacy_changes(context_id,sequence) ON DELETE CASCADE
);
CREATE FUNCTION chat_legacy_capture_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE row_data jsonb; room uuid; entity uuid; entity_kind text; seq bigint;
BEGIN
 IF TG_OP = 'UPDATE' AND NEW IS NOT DISTINCT FROM OLD THEN RETURN NULL; END IF;
 IF TG_OP = 'DELETE' THEN row_data := to_jsonb(OLD); ELSE row_data := to_jsonb(NEW); END IF;
 IF TG_TABLE_NAME = 'messages' THEN
  room := (row_data->>'context_id')::uuid; entity := (row_data->>'id')::uuid; entity_kind := 'message';
 ELSIF TG_TABLE_NAME = 'message_reactions' THEN
  entity := (row_data->>'message_id')::uuid; entity_kind := 'message';
  SELECT context_id INTO room FROM messages WHERE id=entity;
 ELSIF TG_TABLE_NAME = 'votes' THEN
  room := (row_data->>'context_id')::uuid; entity := (row_data->>'id')::uuid; entity_kind := 'vote';
 ELSE
  entity := (row_data->>'vote_id')::uuid; entity_kind := 'vote';
  SELECT context_id INTO room FROM votes WHERE id=entity;
 END IF;
 -- Cascaded child deletion is represented by the parent's tombstone.
 IF room IS NULL OR NOT EXISTS (SELECT 1 FROM contexts WHERE id=room) THEN RETURN NULL; END IF;
 INSERT INTO chat_legacy_change_heads(context_id,sequence) VALUES(room,1)
 ON CONFLICT(context_id) DO UPDATE SET sequence=chat_legacy_change_heads.sequence+1
 RETURNING sequence INTO seq;
 INSERT INTO chat_legacy_changes(context_id,sequence,entity_type,entity_id,revision,deleted)
 VALUES(room,seq,entity_kind,entity,seq,TG_OP='DELETE' AND TG_TABLE_NAME IN ('messages','votes'));
 INSERT INTO chat_legacy_change_outbox(context_id,sequence) VALUES(room,seq);
 PERFORM pg_notify('chat_legacy_changes',room::text);
 RETURN NULL;
END $$;
CREATE TRIGGER chat_legacy_message AFTER INSERT OR UPDATE OR DELETE ON messages FOR EACH ROW EXECUTE FUNCTION chat_legacy_capture_change();
CREATE TRIGGER chat_legacy_reaction AFTER INSERT OR UPDATE OR DELETE ON message_reactions FOR EACH ROW EXECUTE FUNCTION chat_legacy_capture_change();
CREATE TRIGGER chat_legacy_vote AFTER INSERT OR UPDATE OR DELETE ON votes FOR EACH ROW EXECUTE FUNCTION chat_legacy_capture_change();
CREATE TRIGGER chat_legacy_vote_option AFTER INSERT OR UPDATE OR DELETE ON vote_options FOR EACH ROW EXECUTE FUNCTION chat_legacy_capture_change();
CREATE TRIGGER chat_legacy_ballot AFTER INSERT OR UPDATE OR DELETE ON vote_ballots FOR EACH ROW EXECUTE FUNCTION chat_legacy_capture_change();
