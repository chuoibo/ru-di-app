-- Version 5: the job queue (slice 10; docs/claude/2026-09-25/thiet-ke-ai/02
-- §3.2 and §4, ADR-0038 proposed). Needs job_outbox and jobs_them from
-- internal/jobs, which `core migrate-chat` installs first.
--
-- Every DEFAULT stays. During a rolling deploy a replica from before this
-- version INSERTs without these columns (handler.go, nep.go), and each default
-- is what such a row must hold: due now, never enqueued yet (the trigger below
-- numbers it anyway), no content out, no model call spent.
ALTER TABLE chat_ai_invocations
    -- Not claimable before this instant: a retry after a transient provider
    -- failure waits out its backoff here, in the row, not in a timer.
    ADD COLUMN available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    -- How many times the row has entered 'queued'. A broker message names
    -- (id, enqueue_seq); a message from an earlier entry claims nothing.
    ADD COLUMN enqueue_seq bigint NOT NULL DEFAULT 0 CHECK (enqueue_seq >= 0),
    -- When the first content (a part or a delta) left the worker. Once set,
    -- the job is never run again from the start: a second worker would write
    -- the answer again over what the reader already saw.
    ADD COLUMN first_token_at timestamptz,
    -- Model calls spent across every attempt, each one taken in this row
    -- before the call goes out (aiharness/llm MaxModelCallsPerTurn is the cap;
    -- the worker's UPDATE carries it, so it is written in one place).
    ADD COLUMN model_calls integer NOT NULL DEFAULT 0 CHECK (model_calls >= 0);

-- The sequence belongs to the database: every UPDATE passes through here, and
-- only a row entering 'queued' moves it. A statement that sets enqueue_seq
-- itself is overruled, so no Go path can reuse or skip a number.
--
-- Entering 'queued' is: a new question (INSERT), /retry (failed -> queued),
-- retryLater and release (running -> queued). A heartbeat, a cancel and a
-- finished job never enter it, so they never enqueue.
CREATE FUNCTION chat_ai_enqueue_seq() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        NEW.enqueue_seq := CASE WHEN NEW.status = 'queued' THEN 1 ELSE 0 END;
    ELSIF NEW.status = 'queued' AND OLD.status <> 'queued' THEN
        NEW.enqueue_seq := OLD.enqueue_seq + 1;
    ELSE
        NEW.enqueue_seq := OLD.enqueue_seq;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_ai_enqueue_seq BEFORE INSERT OR UPDATE ON chat_ai_invocations
    FOR EACH ROW EXECUTE FUNCTION chat_ai_enqueue_seq();

-- The outbox row commits with the transaction that made the job due. It
-- follows the sequence, so the decision lives in one place (the trigger
-- above): a new number is a new entry into the queue, and only then is a
-- message owed. The queue comes from the scope; the message expires with the
-- sharing window, after which no worker may run the job anyway.
--
-- Not a way around the read gate: aigate declares job_outbox for the Nếp and
-- the group roots, and a PostgreSQL test reads pg_trigger to hold these two
-- triggers to being the only way into the table.
CREATE FUNCTION chat_ai_enqueue() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.enqueue_seq > 0 AND (TG_OP = 'INSERT' OR NEW.enqueue_seq <> OLD.enqueue_seq) THEN
        PERFORM jobs_them(CASE NEW.scope WHEN 'me' THEN 'ai.nep' ELSE 'ai.group' END,
            NEW.id, NEW.enqueue_seq, NEW.available_at, NEW.share_expires_at);
    END IF;
    RETURN NULL;
END $$;
CREATE TRIGGER chat_ai_enqueue AFTER INSERT OR UPDATE ON chat_ai_invocations
    FOR EACH ROW EXECUTE FUNCTION chat_ai_enqueue();

-- The poller's safety net asks for due jobs by available_at.
CREATE INDEX chat_ai_jobs_due ON chat_ai_invocations(available_at)
    WHERE status IN ('queued','running');
