-- Vector ingestion, version 3 of rag_nap_schema_migrations: MILCO is gone
-- (rd.v4, owner 2026-09-29). The sparse leg is Milvus's BM25 over the row's
-- text, computed by Milvus; nothing caches a client-side sparse vector any
-- more, so the MILCO cache goes with it.
DROP TABLE rag_sparse_cache;
