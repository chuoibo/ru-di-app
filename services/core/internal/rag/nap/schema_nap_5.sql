-- Freshness, as the SLO reads it. A row the indexer deferred (waiting for
-- its vector, or for the online budget) already holds its current
-- attributes in the index: cho_tu is when it began to wait, and a new mark
-- on it starts its lag afresh, since what is stale dates from that mark. A
-- row deferred after a failure (a dead-letter row exists) keeps its lag: it
-- is the staleness the SLO is for.
ALTER TABLE rag_dirty ADD COLUMN cho_tu timestamptz;

CREATE OR REPLACE FUNCTION rag_danh_dau(p_corpus text, p_ids text[], p_xoa boolean, p_uu_tien smallint) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO rag_dirty AS d (corpus, doc_id, xoa, uu_tien)
  SELECT p_corpus, s.id, COALESCE(p_xoa, false), p_uu_tien
  FROM (SELECT DISTINCT unnest(p_ids) AS id) s WHERE s.id IS NOT NULL
  ON CONFLICT (corpus, doc_id) DO UPDATE SET
    lan = d.lan + 1,
    xoa = COALESCE(p_xoa, d.xoa),
    cho_den = NULL,
    noticed_at = CASE
      WHEN d.uu_tien < 0 AND EXCLUDED.uu_tien >= 0 THEN clock_timestamp()
      WHEN d.cho_den > clock_timestamp() AND NOT EXISTS (
        SELECT 1 FROM rag_ingest_dlq q WHERE q.corpus = d.corpus AND q.doc_id = d.doc_id) THEN clock_timestamp()
      ELSE d.noticed_at END,
    uu_tien = GREATEST(d.uu_tien, EXCLUDED.uu_tien);
  IF FOUND THEN
    PERFORM pg_notify('rag_dirty', p_corpus);
  END IF;
END $$;
