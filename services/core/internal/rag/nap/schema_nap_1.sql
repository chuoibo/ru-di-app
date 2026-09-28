-- Vector index ingestion (internal/rag/nap; research sdlc-production §C,
-- design 04 §3 and §6). Version 1 of rag_nap_schema_migrations. This package
-- is the only writer of every table here. No table names a person: an index
-- of places and of the app manual has no reason to.

-- One row per physical Milvus collection. The collection's name derives
-- from the id, so no two versions can ever share one, and the pattern never
-- equals an alias (rd_places, rd_manual, rd_*_shadow): the alias__vN scheme
-- of internal/vectordb, whose schema every collection has.
CREATE TABLE rag_vector_versions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  corpus text NOT NULL CHECK (corpus IN ('place','manual')),
  state text NOT NULL CHECK (state IN ('building','built','evaluated','active','retired','failed')),
  milvus_collection text GENERATED ALWAYS AS (CASE corpus WHEN 'place' THEN 'rd_places__v' ELSE 'rd_manual__v' END || id::text) STORED,
  dense_model text NOT NULL CHECK (char_length(dense_model) BETWEEN 1 AND 64),
  dense_dims integer NOT NULL CHECK (dense_dims > 0),
  sparse_mode text NOT NULL CHECK (sparse_mode IN ('bm25','milco')),
  sparse_rev text NOT NULL CHECK (char_length(sparse_rev) BETWEEN 1 AND 200),
  chunker text NOT NULL CHECK (char_length(chunker) BETWEEN 1 AND 32),
  config_fingerprint text NOT NULL CHECK (config_fingerprint ~ '^[0-9a-f]{12}$'),
  prompt_version text CHECK (prompt_version ~ '^[0-9a-f]{12}$'),
  source_digest bytea,
  docs integer,
  chunks integer,
  changed_docs integer,
  parent_id bigint REFERENCES rag_vector_versions(id),
  eval jsonb,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  promoted_at timestamptz,
  retired_at timestamptz,
  dropped_at timestamptz
);
CREATE UNIQUE INDEX rag_vector_versions_one_active ON rag_vector_versions (corpus) WHERE state = 'active';

-- Change capture (S0). One row per changed document, however many times it
-- changed: noticed_at keeps the first change not yet indexed (freshness lag
-- is now - min(noticed_at)), lan counts changes so an indexer deletes a row
-- only if no change arrived while it worked.
CREATE TABLE rag_dirty (
  corpus text NOT NULL CHECK (corpus IN ('place')),
  doc_id text NOT NULL,
  noticed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  lan bigint NOT NULL DEFAULT 1,
  xoa boolean NOT NULL DEFAULT false,
  PRIMARY KEY (corpus, doc_id)
);

-- The change feed: every write to places marks the document dirty and wakes
-- the indexer through the outbox lane 'rag' (one message per corpus per
-- minute: the message says «look», the rows say what). The fixed ref names
-- the corpus, not a document. The trigger needs the outbox's version 2.
CREATE FUNCTION rag_nap_danh_dau() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND OLD.id <> NEW.id) THEN
    INSERT INTO rag_dirty(corpus, doc_id, xoa) VALUES ('place', OLD.id, true)
    ON CONFLICT (corpus, doc_id) DO UPDATE SET lan = rag_dirty.lan + 1, xoa = true;
  END IF;
  IF TG_OP <> 'DELETE' THEN
    INSERT INTO rag_dirty(corpus, doc_id, xoa) VALUES ('place', NEW.id, false)
    ON CONFLICT (corpus, doc_id) DO UPDATE SET lan = rag_dirty.lan + 1, xoa = false;
  END IF;
  PERFORM jobs_them('rag', 'ab5a1dfe-cafe-4bad-8ace-feedbeefcafe'::uuid,
                    floor(extract(epoch FROM clock_timestamp()) / 60)::bigint, clock_timestamp(), NULL);
  RETURN NULL;
END $$;

CREATE TRIGGER rag_nap_places_dirty AFTER INSERT OR UPDATE OR DELETE ON places
  FOR EACH ROW EXECUTE FUNCTION rag_nap_danh_dau();

-- S5 enrichment: one row per place and extractor, keyed by the source hash
-- the model saw. output holds closed ids, a confidence enum and at most five
-- short dish names; never a place's own text. Deleting a place deletes its
-- enrichment.
CREATE TABLE place_enrichments (
  place_id text NOT NULL REFERENCES places(id) ON DELETE CASCADE,
  extractor text NOT NULL CHECK (extractor IN ('place-enrich')),
  source_hash bytea NOT NULL CHECK (octet_length(source_hash) = 32),
  model text NOT NULL CHECK (char_length(model) BETWEEN 1 AND 64),
  prompt_version text NOT NULL CHECK (prompt_version ~ '^[0-9a-f]{12}$'),
  output jsonb NOT NULL,
  chen_lenh boolean NOT NULL,
  tin_cay text NOT NULL CHECK (tin_cay IN ('cao','vua','thap')),
  can_duyet boolean NOT NULL,
  review text NOT NULL CHECK (review IN ('auto','reviewed','rejected')),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  reviewed_at timestamptz,
  PRIMARY KEY (place_id, extractor)
);
CREATE INDEX place_enrichments_queue ON place_enrichments (created_at) WHERE can_duyet AND review = 'auto';

-- S8 caches: a vector per content hash, model, dims and task. Only a hash
-- missing here costs a provider call.
CREATE TABLE rag_embedding_cache (
  content_hash text NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
  model text NOT NULL,
  dims integer NOT NULL CHECK (dims > 0),
  task text NOT NULL,
  vec bytea NOT NULL CHECK (octet_length(vec) = 4 * dims),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (content_hash, model, dims, task)
);
CREATE TABLE rag_sparse_cache (
  content_hash text NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
  model text NOT NULL,
  rev text NOT NULL,
  prune_k integer NOT NULL CHECK (prune_k > 0),
  idx integer[] NOT NULL,
  val real[] NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (content_hash, model, rev, prune_k)
);

-- Dead letters of the batch stages: which document failed at which stage,
-- how often. No text, no payload: the payload rebuilds from places by id.
CREATE TABLE rag_ingest_dlq (
  corpus text NOT NULL CHECK (corpus IN ('place','manual')),
  doc_id text NOT NULL,
  chang text NOT NULL CHECK (chang IN ('hop_dong','lam_giau','nhung','thua','ghi','xoa')),
  ma_loi text NOT NULL CHECK (ma_loi IN ('het_gio','nha_cung_cap','cau_truc','het_tran','milvus','khac')),
  so_lan integer NOT NULL DEFAULT 1 CHECK (so_lan > 0),
  lan_cuoi timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (corpus, doc_id, chang)
);
