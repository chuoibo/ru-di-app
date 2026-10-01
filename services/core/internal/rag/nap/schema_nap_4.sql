-- Event-driven indexing. The indexer listens on channel rag_dirty instead of
-- waking once a minute: every mark notifies it, and Postgres folds the
-- identical notifications of one transaction into one.
--
-- cho_den defers a row: a backoff after a failure, or the wait for a vector
-- the batch door will bring (cho_nhung, kept until the row is indexed so a
-- later mark never pays online for a vector already ordered). uu_tien orders
-- the queue: 1 a change of visibility (a tombstone, a review), 0 a change of
-- the source, -1 a background re-check (after a build); a row raised from -1
-- restarts its noticed_at, since the lag a source change waits starts then.
ALTER TABLE rag_dirty
  ADD COLUMN cho_den timestamptz,
  ADD COLUMN cho_nhung boolean NOT NULL DEFAULT false,
  ADD COLUMN uu_tien smallint NOT NULL DEFAULT 0 CHECK (uu_tien BETWEEN -1 AND 1);
CREATE INDEX rag_dirty_hang ON rag_dirty (corpus, uu_tien DESC, noticed_at, doc_id);

-- rag_danh_dau marks documents dirty and notifies the indexer: the one way
-- in, for triggers and for the pipeline's own marks. p_xoa NULL keeps a
-- row's flag.
CREATE FUNCTION rag_danh_dau(p_corpus text, p_ids text[], p_xoa boolean, p_uu_tien smallint) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO rag_dirty AS d (corpus, doc_id, xoa, uu_tien)
  SELECT p_corpus, s.id, COALESCE(p_xoa, false), p_uu_tien
  FROM (SELECT DISTINCT unnest(p_ids) AS id) s WHERE s.id IS NOT NULL
  ON CONFLICT (corpus, doc_id) DO UPDATE SET
    lan = d.lan + 1,
    xoa = COALESCE(p_xoa, d.xoa),
    cho_den = NULL,
    noticed_at = CASE WHEN d.uu_tien < 0 AND EXCLUDED.uu_tien >= 0 THEN clock_timestamp() ELSE d.noticed_at END,
    uu_tien = GREATEST(d.uu_tien, EXCLUDED.uu_tien);
  IF FOUND THEN
    PERFORM pg_notify('rag_dirty', p_corpus);
  END IF;
END $$;

-- places: an UPDATE that changes nothing marks nothing (a feed that
-- rewrites identical rows no longer wakes the indexer for them).
CREATE OR REPLACE FUNCTION rag_nap_danh_dau() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' OR (TG_OP = 'UPDATE' AND OLD.id <> NEW.id) THEN
    PERFORM rag_danh_dau('place', ARRAY[OLD.id], true, 0::smallint);
  END IF;
  IF TG_OP <> 'DELETE' THEN
    PERFORM rag_danh_dau('place', ARRAY[NEW.id], false, 0::smallint);
  END IF;
  PERFORM jobs_them('rag', 'ab5a1dfe-cafe-4bad-8ace-feedbeefcafe'::uuid,
                    floor(extract(epoch FROM clock_timestamp()) / 60)::bigint, clock_timestamp(), NULL);
  RETURN NULL;
END $$;
DROP TRIGGER rag_nap_places_dirty ON places;
CREATE TRIGGER rag_nap_places_ghi AFTER INSERT OR DELETE ON places
  FOR EACH ROW EXECUTE FUNCTION rag_nap_danh_dau();
CREATE TRIGGER rag_nap_places_doi AFTER UPDATE ON places
  FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*) EXECUTE FUNCTION rag_nap_danh_dau();

-- A tombstone placed, changed or lifted, and an enrichment written,
-- reviewed or removed, change what a place's row may hold: every writer of
-- those tables marks the place, without each call site having to. The
-- indexer's own writes (inside its pass, on the documents it is indexing)
-- set rag.chi_muc and mark nothing: the pass handles them.
CREATE FUNCTION rag_nap_danh_dau_phu() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  muc smallint := 1;
BEGIN
  IF current_setting('rag.chi_muc', true) = 'on' THEN
    RETURN NULL;
  END IF;
  IF TG_TABLE_NAME = 'rag_tombstones' THEN
    IF TG_OP <> 'INSERT' AND OLD.corpus <> 'place' THEN
      RETURN NULL;
    END IF;
    IF TG_OP <> 'DELETE' AND NEW.corpus <> 'place' THEN
      RETURN NULL;
    END IF;
    IF TG_OP <> 'INSERT' THEN
      PERFORM rag_danh_dau('place', ARRAY[OLD.doc_id], NULL, muc);
    END IF;
    IF TG_OP <> 'DELETE' THEN
      PERFORM rag_danh_dau('place', ARRAY[NEW.doc_id], NULL, muc);
    END IF;
    RETURN NULL;
  END IF;
  -- place_enrichments: a review decides whether the enrichment counts at
  -- all (urgent); a new enrichment is a source change.
  IF TG_OP = 'UPDATE' AND OLD.review IS NOT DISTINCT FROM NEW.review THEN
    muc := 0;
  ELSIF TG_OP = 'INSERT' THEN
    muc := 0;
  END IF;
  IF TG_OP <> 'INSERT' THEN
    PERFORM rag_danh_dau('place', ARRAY[OLD.place_id], NULL, muc);
  END IF;
  IF TG_OP = 'UPDATE' AND NEW.place_id <> OLD.place_id THEN
    PERFORM rag_danh_dau('place', ARRAY[NEW.place_id], NULL, muc);
  ELSIF TG_OP = 'INSERT' THEN
    PERFORM rag_danh_dau('place', ARRAY[NEW.place_id], NULL, muc);
  END IF;
  RETURN NULL;
END $$;
CREATE TRIGGER rag_nap_bia_ghi AFTER INSERT OR DELETE ON rag_tombstones
  FOR EACH ROW EXECUTE FUNCTION rag_nap_danh_dau_phu();
CREATE TRIGGER rag_nap_bia_doi AFTER UPDATE ON rag_tombstones
  FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*) EXECUTE FUNCTION rag_nap_danh_dau_phu();
CREATE TRIGGER rag_nap_lam_giau_ghi AFTER INSERT OR DELETE ON place_enrichments
  FOR EACH ROW EXECUTE FUNCTION rag_nap_danh_dau_phu();
CREATE TRIGGER rag_nap_lam_giau_doi AFTER UPDATE ON place_enrichments
  FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*) EXECUTE FUNCTION rag_nap_danh_dau_phu();

-- The places a build's dedupe (S6) left out of its collection, and the place
-- each one merged into: the indexer never writes them back into that
-- version (it cannot dedupe a single place; the next build does).
CREATE TABLE rag_trung (
  phien_ban bigint NOT NULL REFERENCES rag_vector_versions(id) ON DELETE CASCADE,
  doc_id text NOT NULL,
  giu text NOT NULL,
  PRIMARY KEY (phien_ban, doc_id)
);

-- Until now a tombstone never marked its place: a live collection may still
-- hold a place hidden since it was built. Check every one once.
SELECT rag_danh_dau('place', ARRAY(SELECT doc_id FROM rag_tombstones WHERE corpus = 'place'), NULL, 0::smallint);
