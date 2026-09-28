-- The embedding cache of PUBLIC content (places, the app manual): a rebuilt
-- or re-versioned index re-embeds only what changed. The key is
-- sha256(model, dims, prompt_version, the NFC prompt with its task prefix),
-- so a change of any of the four is a miss, never a stale vector.
--
-- Private content never enters: the kho CHECK admits the two public stores
-- only, so a memory vector cannot be written here even by hand, and a
-- «forget» has nothing to find in this table.
CREATE TABLE IF NOT EXISTS nhung_cache (
  khoa           bytea PRIMARY KEY CHECK (length(khoa) = 32),
  kho            text NOT NULL CHECK (kho IN ('places', 'manual')),
  model          text NOT NULL,
  dims           integer NOT NULL CHECK (dims > 0),
  prompt_version text NOT NULL,
  vec            bytea NOT NULL,
  tao_luc        timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT nhung_cache_vec_dims CHECK (length(vec) = 4 * dims)
);
