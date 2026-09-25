-- Retrieval index, lexical part (design 04 §3, ADR-0040 proposal). Version 1
-- of rag_schema_migrations. internal/rag is the only writer of every rag_*
-- table. No table here names a person: an index of places has no reason to.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE rag_index_versions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  corpus text NOT NULL CHECK (corpus IN ('place')),
  state text NOT NULL CHECK (state IN ('building','built','evaluated','active','retired','failed')),
  chunker text NOT NULL,
  embed_model text NOT NULL DEFAULT 'none',
  embed_dims integer NOT NULL DEFAULT 0,
  source_digest bytea,
  parent_id bigint REFERENCES rag_index_versions(id),
  eval jsonb,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  promoted_at timestamptz,
  retired_at timestamptz
);
CREATE UNIQUE INDEX rag_index_versions_one_active ON rag_index_versions (corpus) WHERE state = 'active';

CREATE TABLE rag_docs (
  version_id bigint NOT NULL REFERENCES rag_index_versions(id) ON DELETE CASCADE,
  doc_id text NOT NULL,
  destination_id text NOT NULL,
  category text NOT NULL,
  price_min_vnd bigint,
  price_max_vnd bigint,
  -- Minutes of the week the place is open; NULL is "hours unknown", which is
  -- not "closed".
  open_week int4multirange,
  -- Scanned deterministically from the row's own safe words (domain/tuvung).
  di_ung_nguon text[] NOT NULL DEFAULT '{}',
  an_kieng_nguon text[] NOT NULL DEFAULT '{}',
  khi_chat text[] NOT NULL DEFAULT '{}',
  name_fold text NOT NULL,
  lat double precision NOT NULL,
  lng double precision NOT NULL,
  license text,
  canonical_id text,
  PRIMARY KEY (version_id, doc_id)
);
CREATE INDEX rag_docs_loc ON rag_docs (version_id, destination_id, category);
CREATE INDEX rag_docs_name_trgm ON rag_docs USING gin (name_fold gin_trgm_ops);

-- search_text is xephang.ChuTimKiem(body): folded syllables, then each pair
-- of neighbours joined by '_'. The default text-search parser reads '_' as a
-- blank (to_tsvector('simple','ca_phe') is 'ca':1 'phe':2), which would turn
-- every pair back into its two syllables; so the vector drops the '_' and a
-- pair becomes one lexeme ('caphe'), the spelling the query side builds too.
CREATE TABLE rag_chunks (
  version_id bigint NOT NULL,
  chunk_id text NOT NULL,
  doc_id text NOT NULL,
  facet text NOT NULL CHECK (facet IN ('ho_so','danh_gia')),
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  search_text text NOT NULL,
  tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, replace(search_text, '_', ''))) STORED,
  content_hash bytea NOT NULL,
  PRIMARY KEY (version_id, chunk_id),
  FOREIGN KEY (version_id, doc_id) REFERENCES rag_docs(version_id, doc_id) ON DELETE CASCADE
);
CREATE INDEX rag_chunks_tsv ON rag_chunks USING gin (tsv);

-- A tombstone belongs to no version: every query anti-joins it, so rolling
-- back to an older version never brings a removed place back.
CREATE TABLE rag_tombstones (
  corpus text NOT NULL CHECK (corpus IN ('place')),
  doc_id text NOT NULL,
  reason text NOT NULL CHECK (reason IN ('unsafe','takedown','closed','source_deleted')),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (corpus, doc_id)
);

-- One row per retrieval an engine ran: ids, enums, counts and durations, no
-- words of the question and none of the evidence.
CREATE TABLE rag_query_log (
  at timestamptz NOT NULL DEFAULT clock_timestamp(),
  corpus text NOT NULL CHECK (corpus IN ('place')),
  y_dinh text NOT NULL CHECK (y_dinh IN ('find_places','plan','plan_help')),
  version_id bigint,
  n_loc integer NOT NULL CHECK (n_loc >= 0),
  n_ung_vien integer NOT NULL CHECK (n_ung_vien >= 0),
  vong smallint NOT NULL CHECK (vong BETWEEN 1 AND 2),
  cham_lai boolean NOT NULL,
  ket_qua text NOT NULL CHECK (ket_qua IN ('tra_loi','hoi_lai','khong_thoa','degraded')),
  ms integer NOT NULL CHECK (ms >= 0),
  co text[] NOT NULL DEFAULT '{}' CHECK (co <@ ARRAY['khong_dau','gio_chua_ro','gia_chua_ro','nhan_nhieu_diem_den']::text[])
);
CREATE INDEX rag_query_log_at ON rag_query_log (at);
