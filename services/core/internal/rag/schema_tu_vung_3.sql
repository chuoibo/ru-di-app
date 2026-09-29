-- Retrieval index, version 3 of rag_schema_migrations: a place the web says
-- is permanently closed leaves search.
--
-- 'web_closed' is written and lifted by one module only, rudi-ingest's
-- web-facts pass (internal/ingest/facts.go), from vnlocal's web-facts@2
-- `con_hoat_dong = 'dong_vinh_vien'` while the fact is unexpired. It is not
-- 'closed': that one is a person's word and only a person lifts it, while
-- this one comes back off when a later web check no longer confirms the
-- closure. The `places` row is kept either way.
ALTER TABLE rag_tombstones DROP CONSTRAINT rag_tombstones_reason_check;
ALTER TABLE rag_tombstones ADD CONSTRAINT rag_tombstones_reason_check
  CHECK (reason IN ('unsafe','takedown','closed','source_deleted','web_closed'));
