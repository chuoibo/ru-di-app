-- The transactional outbox between job tables and RabbitMQ (ADR-0038 proposal,
-- docs/claude/2026-09-25/thiet-ke-ai/02 §3.1). No payload column: the only
-- thing that ever reaches the broker is an id, and no person column, so the
-- erasure trigger has nothing to find here by design.
CREATE TABLE job_outbox (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  queue text NOT NULL CHECK (queue IN ('ai.group','ai.nep','memory','notify')),
  ref_id uuid NOT NULL,
  enqueue_seq bigint NOT NULL CHECK (enqueue_seq >= 0),
  available_at timestamptz NOT NULL,
  expires_at timestamptz,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (queue, ref_id, enqueue_seq)
);
CREATE INDEX job_outbox_due ON job_outbox (available_at, id) WHERE published_at IS NULL;

-- The only way into job_outbox. A job table's trigger calls it in the same
-- transaction that makes the job due, so a job and its message commit together.
CREATE FUNCTION jobs_them(q text, ref uuid, seq bigint, due timestamptz, expires timestamptz)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO job_outbox(queue, ref_id, enqueue_seq, available_at, expires_at)
  VALUES (q, ref, seq, due, expires)
  ON CONFLICT (queue, ref_id, enqueue_seq) DO NOTHING;
  PERFORM pg_notify('job_outbox', '');
END $$;
