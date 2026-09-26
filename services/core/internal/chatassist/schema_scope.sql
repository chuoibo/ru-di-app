-- Version 3: the personal scope, and context the caller hands over.
--
-- One table, not two. A second table would mean a second claim(), a second
-- expiry sweep and a second publish() -- which is the two parallel paths this
-- change exists to remove.

ALTER TABLE chat_ai_invocations
    ADD COLUMN scope text NOT NULL DEFAULT 'group',
    ADD COLUMN boi_canh jsonb,
    ADD COLUMN result jsonb;

-- The default existed only to fill the rows already here. Leaving it in place
-- would let an INSERT that forgets `scope` pass as a group job.
ALTER TABLE chat_ai_invocations ALTER COLUMN scope DROP DEFAULT;

-- A personal invocation belongs to nobody's room.
ALTER TABLE chat_ai_invocations
    ALTER COLUMN context_id DROP NOT NULL,
    ALTER COLUMN membership_id DROP NOT NULL;

-- Deliberately NOT `IF EXISTS`. This is the name PostgreSQL gives a column
-- CHECK (`<table>_<column>_check`), and it is deterministic -- but if it ever
-- is not, `IF EXISTS` would swallow the miss, the replacement below would
-- still install, and `command = 'plan'` would quietly stay enforced until
-- chia_bill returned 500 in production. Failing here, loudly, at migration
-- time, is the whole point.
ALTER TABLE chat_ai_invocations DROP CONSTRAINT chat_ai_invocations_command_check;

ALTER TABLE chat_ai_invocations
    ADD CONSTRAINT chat_ai_scope_shape CHECK (
        (scope = 'group' AND context_id IS NOT NULL AND membership_id IS NOT NULL)
     OR (scope = 'me'    AND context_id IS NULL     AND membership_id IS NULL)),

    -- Closes the 6-cell scope x command matrix down to the 3 legal pairs, so
    -- the command whitelist needs no constraint of its own.
    ADD CONSTRAINT chat_ai_command_scope CHECK (
        (scope = 'group' AND command IN ('plan','chia_bill'))
     OR (scope = 'me'    AND command = 'hoi')),

    ADD CONSTRAINT chat_ai_boi_canh_object CHECK (
        boi_canh IS NULL OR jsonb_typeof(boi_canh) = 'object'),

    -- The load-bearing one. "Context lives exactly as long as the prompt" is
    -- a property no reviewer can verify by eye: miss one `SET prompt=NULL` and
    -- someone else's plaintext stays behind without a sound. This turns that
    -- miss into an error at the offending statement.
    ADD CONSTRAINT chat_ai_context_needs_prompt CHECK (
        boi_canh IS NULL OR prompt IS NOT NULL),

    ADD CONSTRAINT chat_ai_publication CHECK (
        scope = 'group' OR message_id IS NULL);

-- UNIQUE (context_id, person_id, logical_id) stops enforcing the moment
-- context_id is NULL: PostgreSQL treats every NULL as distinct. Without this
-- partial index a personal invocation has no idempotency at all, and the
-- advisory lock does not cover it -- that only serialises, it does not dedupe.
CREATE UNIQUE INDEX chat_ai_me_logical
    ON chat_ai_invocations(person_id, logical_id) WHERE context_id IS NULL;

-- Same trigger, now scrubbing the handed-over context too. REPLACE keeps the
-- trigger binding from version 1 intact, so there is no drop-and-recreate
-- window. Personal rows are immune without a special case: membership_id is
-- NULL for them and `NULL = OLD.id` is never true.
CREATE OR REPLACE FUNCTION chat_ai_membership_revoked() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state = 'active' AND NEW.state <> 'active' THEN
        UPDATE chat_ai_invocations SET status='cancelled',code='sharing_revoked',
            prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp()
        WHERE membership_id=OLD.id AND status IN ('queued','running','failed');
    END IF;
    RETURN NEW;
END $$;
