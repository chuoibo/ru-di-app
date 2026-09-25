-- Retrieval index, version 2 of rag_schema_migrations: pairs keep a marker.
--
-- Version 1 dropped the '_' that joins a pair of neighbouring syllables, so
-- the pair «tho_ai» became the lexeme 'thoai' -- the same lexeme as the
-- syllable «thoại» (measured: to_tsvector('simple','tho ai tho_ai thoai')
-- gave 'thoai':3,4). The pair is now joined with 'ǂ' (U+01C2), a letter to
-- the default parser, so it stays inside one token and matches nothing but
-- the same pair (measured: to_tsvector('simple','thoǂai thoai') gives
-- 'thoai':2 'thoǂai':1, in a C and a C.UTF-8 database alike). No folded
-- syllable holds it: promptsafety.Fold never produces it from Vietnamese or
-- English text. The query side (tsQuery) joins pairs the same way.
DROP INDEX rag_chunks_tsv;
ALTER TABLE rag_chunks DROP COLUMN tsv;
ALTER TABLE rag_chunks ADD COLUMN tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, replace(search_text, '_', 'ǂ'))) STORED;
CREATE INDEX rag_chunks_tsv ON rag_chunks USING gin (tsv);
