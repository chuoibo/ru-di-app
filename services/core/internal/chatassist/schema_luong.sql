-- Version 4: the group AI answers inside the thread (ADR-0039, proposed).
--
-- The `@Rủ Đi …` words are an ordinary message now, posted by the person who
-- asks, and the invocation names that message. The answer is published as a
-- reply to it, so this column is the whole of "which question is this the
-- answer to" -- the server still never reads what the message says.

ALTER TABLE chat_ai_invocations
    ADD COLUMN trigger_message_id uuid,
    -- Which transport the room is on: derived by the server from
    -- chat_v2_conversations inside `authority`, never read from a request.
    -- The default is deliberate, not an oversight to drop later: a replica
    -- from before this version INSERTs without the column during a rolling
    -- deploy, and every room such a replica can serve is a legacy room, because
    -- it refuses a v2 room with 409 before it inserts anything.
    ADD COLUMN lane text NOT NULL DEFAULT 'legacy' CHECK (lane IN ('legacy','v2')),
    -- How many shared turns the server confirmed are messages of this room
    -- (thuocPhong). The published reply says «đọc N tin» from this column,
    -- never from the caller's own count. 40 is the bundle ceiling (maxLuot).
    ADD COLUMN so_tin_doc integer CHECK (so_tin_doc BETWEEN 0 AND 40),
    -- Same room by construction: the pair is the one `messages` itself
    -- keeps unique for its reply key (uq_messages_id_context), so a trigger
    -- from another room is refused by the database as well as by the handler.
    -- A hard delete forgets only the link; context_id is never nulled.
    ADD CONSTRAINT chat_ai_trigger_room FOREIGN KEY (trigger_message_id, context_id)
        REFERENCES messages(id, context_id) ON DELETE SET NULL (trigger_message_id),
    -- A personal (Nếp) job has no room, so it has no message in one.
    ADD CONSTRAINT chat_ai_trigger_scope CHECK (trigger_message_id IS NULL OR scope = 'group');

-- One message, one answer. Two presses with two logical ids on the same
-- `@Rủ Đi` message would otherwise publish two replies to it. A failed or
-- cancelled job does not hold the message: asking again after a failure is
-- how the person recovers.
CREATE UNIQUE INDEX chat_ai_one_answer_per_trigger ON chat_ai_invocations(trigger_message_id)
    WHERE trigger_message_id IS NOT NULL AND status IN ('queued','running','succeeded');

-- The room limit (three jobs in flight per room) counts exactly these rows.
CREATE INDEX chat_ai_room_active ON chat_ai_invocations(context_id, created_at)
    WHERE status IN ('queued','running');

-- Taking back the `@Rủ Đi` message takes back the question: the job stops and
-- its shared words go, the same shape as chat_ai_membership_revoked. A job
-- that already answered keeps its answer; there is nothing left to scrub.
--
-- Deadlock note, because this is a second writer on a table Python owns
-- (precedent: chatlegacychange/schema.sql). A deletion locks the message
-- FOR UPDATE after taking the room's feed head (chatlegacychange BeforeWrite),
-- then this trigger updates the job. publish() takes the same order: the feed
-- head first, then a KEY SHARE lock on the message, then the job, so the two
-- serialise on the feed head and never hold each other's next lock.
CREATE FUNCTION chat_ai_trigger_deleted() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE chat_ai_invocations SET status='cancelled',code='trigger_deleted',
        prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp()
    WHERE trigger_message_id=NEW.id AND status IN ('queued','running','failed');
    RETURN NEW;
END $$;
CREATE TRIGGER chat_ai_trigger_deleted AFTER UPDATE OF kind ON messages
    FOR EACH ROW WHEN (NEW.kind = 'deleted' AND OLD.kind <> 'deleted')
    EXECUTE FUNCTION chat_ai_trigger_deleted();
