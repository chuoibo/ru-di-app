-- Where the pull from the feed's own database has reached.
--
-- The feed writes in transactions of about two hundred rows that share one
-- `synced_at`, so a cursor on `synced_at` alone skips the rest of a batch cut
-- across a page boundary -- measured upstream at 14% of rows lost, silently.
-- The cursor is the pair, matching the source index `(synced_at, place_id)`.
--
-- Kept in the same transaction as the rows it covers: either the rows landed
-- and the cursor moved, or neither happened.
CREATE TABLE ingest_cursor (
 source text PRIMARY KEY CHECK(source ~ '^[a-z][a-z0-9_.]*$'),
 synced_at timestamptz NOT NULL,
 place_id text NOT NULL,
 batch_id text NOT NULL REFERENCES ingest_batch(id),
 moved_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- When a batch's photographs were imported. Separate from `applied_at` because
-- photos come from another system (object storage) and can fail on their own:
-- a batch whose photo pass broke half way stays NULL here and is retried on
-- the next round, instead of being forgotten because its rows already applied.
ALTER TABLE ingest_batch ADD COLUMN photos_at timestamptz;
