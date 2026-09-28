-- Batch embedding jobs (ADR-0049 §2.7): one row per Gemini Batch API job the
-- ingest submitted for documents whose vectors the cache did not hold. A run
-- that finds a job still running for the same model, dims and task polls it
-- instead of submitting (and paying for) the same documents again. Counts and
-- states only: no text.
CREATE TABLE rag_embed_batches (
  job text PRIMARY KEY CHECK (job ~ '^batches/[A-Za-z0-9_-]{1,128}$'),
  model text NOT NULL,
  dims integer NOT NULL CHECK (dims > 0),
  task text NOT NULL,
  so_dong integer NOT NULL CHECK (so_dong > 0),
  trang_thai text NOT NULL CHECK (trang_thai IN ('dang_chay','xong','hong')),
  so_ghi integer CHECK (so_ghi >= 0),
  loi_dong integer CHECK (loi_dong >= 0),
  tao_at timestamptz NOT NULL DEFAULT now(),
  xong_at timestamptz,
  CHECK ((trang_thai = 'dang_chay') = (xong_at IS NULL))
);
CREATE UNIQUE INDEX rag_embed_batches_mot_dang_chay ON rag_embed_batches(model, dims, task)
  WHERE trang_thai = 'dang_chay';
