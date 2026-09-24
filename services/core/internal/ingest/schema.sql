-- Landing zone and identity tables for the external place catalogue feed.
--
-- These tables are isolated: nothing here alters a table Alembic owns. The
-- columns this feed needs on `places` and `place_photos` arrive through an
-- Alembic revision instead, because `internal/db/db.go` states the rule --
-- Alembic owns the schema, and a second writer issuing DDL on the same table
-- is how two migration systems silently disagree.
--
-- The feed is an LLM-backed pipeline: names are model-authored, its own row key
-- is sha256(name|province), and its schema moves with prompt versions. Every
-- shape below exists so that instability upstream cannot corrupt data here.

-- One delivery. `dot_seq` increments so a missing delivery is detectable, and
-- `dot_truoc` chains them; neither is a foreign key, because a broken chain
-- must be reported rather than refused -- refusing it would block the very
-- recovery load that repairs the gap.
CREATE TABLE ingest_batch (
 id text PRIMARY KEY,
 source text NOT NULL,
 schema_version text NOT NULL,
 dot_seq integer NOT NULL UNIQUE CHECK(dot_seq > 0),
 dot_truoc text,
 kieu_dot text NOT NULL CHECK(kieu_dot IN ('toan_bo', 'tang_dan')),
 file_name text NOT NULL,
 file_sha256 text NOT NULL CHECK(file_sha256 ~ '^[0-9a-f]{64}$'),
 file_bytes bigint NOT NULL CHECK(file_bytes > 0),
 rows_declared integer NOT NULL CHECK(rows_declared >= 0),
 rows_landed integer NOT NULL DEFAULT 0 CHECK(rows_landed >= 0),
 updated_at_min timestamptz,
 updated_at_max timestamptz,
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 applied_at timestamptz
);

-- The payload exactly as delivered, before anything interprets it.
--
-- This is the same invariant the ledger has: a projection must be recomputable
-- from the record. The mapping rules for a model-authored source will be wrong
-- the first time, and keeping the bytes means fixing the rule and replaying,
-- rather than asking the other side to export again.
CREATE TABLE ingest_place_raw (
 batch_id text NOT NULL REFERENCES ingest_batch(id),
 line_no integer NOT NULL CHECK(line_no > 0),
 payload jsonb NOT NULL,
 payload_sha text NOT NULL CHECK(payload_sha ~ '^[0-9a-f]{64}$'),
 source_key text NOT NULL CHECK(length(btrim(source_key)) > 0),
 source_updated_at timestamptz,
 PRIMARY KEY(batch_id, line_no)
);
CREATE INDEX ingest_place_raw_source_key ON ingest_place_raw (source_key);

-- A row that did not make it, and why. A dropped row that leaves no trace is
-- indistinguishable from a row the feed never sent.
CREATE TABLE ingest_reject (
 batch_id text NOT NULL REFERENCES ingest_batch(id),
 line_no integer NOT NULL,
 reason text NOT NULL CHECK(reason ~ '^[a-z][a-z0-9_]*$'),
 detail text,
 source_key text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(batch_id, line_no, reason)
);
CREATE INDEX ingest_reject_reason ON ingest_reject (reason);

-- Which platform posts a place was built from. This, not the upstream row key,
-- is what identity is anchored on: post ids are issued by the platform and do
-- not move, while the upstream key is a hash of a name a model chose.
--
-- Many-to-many on purpose. One post legitimately describes several places (a
-- "five best places" video), and the same place is described by many posts.
CREATE TABLE place_source_post (
 platform text NOT NULL CHECK(platform IN ('tiktok', 'threads')),
 post_id text NOT NULL CHECK(length(btrim(post_id)) > 0),
 place_id text NOT NULL REFERENCES places(id),
 source_url text,
 first_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(platform, post_id, place_id)
);
CREATE INDEX place_source_post_place ON place_source_post (place_id);

-- A human correction, held beside the ingested row rather than written into it.
--
-- Editing the ingested row directly would work until the next delivery quietly
-- reverted it. Keeping corrections separate means a reload never destroys human
-- work, and it stays visible who changed what and why.
CREATE TABLE place_override (
 place_id text NOT NULL REFERENCES places(id),
 field text NOT NULL CHECK(field ~ '^[a-z][a-z0-9_]*$'),
 value jsonb NOT NULL,
 actor text NOT NULL CHECK(length(btrim(actor)) > 0),
 reason text NOT NULL CHECK(length(btrim(reason)) >= 8),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(place_id, field)
);

-- The 34 provinces of the 2025 reorganisation, carried over from the same seed
-- file the feed normalises against. Sharing one code table is the point: two
-- independently sourced lists that disagree on a single code produce an empty
-- province nobody notices.
--
-- `places.province_code` deliberately carries NO foreign key to this table.
-- Alembic runs before this migration, so the constraint could not be created
-- there, and adding it from here would mean this file issuing DDL against a
-- table Alembic owns. The ingest worker rejects a row whose province code is
-- not present here, which is the same guarantee enforced where it can be.
CREATE TABLE admin_province (
 code smallint PRIMARY KEY CHECK(code > 0),
 name text NOT NULL CHECK(length(btrim(name)) > 0),
 name_norm text NOT NULL CHECK(length(btrim(name_norm)) > 0),
 division_type text NOT NULL,
 codename text NOT NULL
);

-- Objects whose row is gone but whose bytes are not yet deleted.
--
-- On a filesystem, swallowing an unlink error leaves rubbish. Against an object
-- store the same swallowed error leaves a picture somebody asked to have
-- deleted, which is an unmet obligation rather than untidiness. The row is
-- written before the delete is attempted and removed once it succeeds, so a
-- failure is a queue entry rather than a silence.
CREATE TABLE pending_object_deletes (
 storage_key text PRIMARY KEY CHECK(storage_key ~ '^[0-9a-f]{32}$'),
 reason text NOT NULL CHECK(reason ~ '^[a-z][a-z0-9_]*$'),
 requested_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts >= 0),
 last_error text
);
CREATE INDEX pending_object_deletes_requested ON pending_object_deletes (requested_at);
