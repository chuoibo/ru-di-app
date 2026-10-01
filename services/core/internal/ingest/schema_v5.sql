-- Freshness of the feed as RuDi reads it (the SLO the rag-indexer serves on
-- /slo and `core rag v-status` prints): per source, when the last
-- successful round ended, where its cursor stands and how far the feed's
-- newest row is past it. Written by the sync daemon after each round; ingest
-- is the only writer. Counts and instants only.
CREATE TABLE ingest_do_tre (
 source text PRIMARY KEY CHECK (source ~ '^vnlocal\.'),
 vong_at timestamptz NOT NULL,
 con_tro timestamptz,
 nguon_moi_nhat timestamptz,
 tre_giay double precision NOT NULL CHECK (tre_giay >= 0)
);
