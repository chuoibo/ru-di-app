-- Version 2 of job_schema_migrations: the lane 'rag', the vector index's
-- change-capture wake-up (internal/rag/nap). The trigger on places enqueues
-- at most one message per corpus per minute; the message is an id and a
-- number like every other lane's, the dirty rows themselves stay in Postgres.
ALTER TABLE job_outbox DROP CONSTRAINT job_outbox_queue_check;
ALTER TABLE job_outbox ADD CONSTRAINT job_outbox_queue_check CHECK (queue IN ('ai.group','ai.nep','memory','notify','rag'));
