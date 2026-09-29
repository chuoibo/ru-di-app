-- Web facts from the feed (vnlocal `place_web_facts`, web-facts@1 and @2):
-- hours per weekday, whether the place still trades, spend per person, a
-- priced menu, activities. Answer to RuDi request PR #3
-- (vnlocal HANDOFF-RUDI-GIO-GIA-TRA-LOI.md).
--
-- One row per catalogue place, keyed by the id the catalogue mints for the
-- feed's place (ingest.PlaceID). No foreign key to `places`: the two feeds
-- move on separate cursors, so a place's facts can land before the place
-- does, and the derivation (ApplyFacts) joins them whenever both exist.
--
-- What is NOT copied, on purpose: `bang_chung` (quoted web passages),
-- `truy_van` (the search queries), `bo` (what the checker dropped), `nguon`
-- (URLs), `model`. They are the feed's audit trail, not facts about the
-- place, and they stay on the feed's side.
--
-- `gio_osm`, `gia_min_vnd`, `gia_max_vnd` and `gia_uoc` are derived at
-- landing (facts.go: GioOSM, GiaMoiNguoi) and copied onto `places` only while
-- `het_han_at` is in the future: an expired fact reads as unknown, never as
-- the last value seen.
CREATE TABLE place_facts (
 place_id text PRIMARY KEY CHECK (place_id ~ '^vnl-'),
 source_ref text NOT NULL UNIQUE,
 schema_version text NOT NULL,
 trang_thai text NOT NULL,
 con_hoat_dong text NOT NULL
   CHECK (con_hoat_dong IN ('con_hoat_dong','tam_dong','dong_vinh_vien','khong_ro')),
 -- Hours as fed: [{thu, mo, dong, qua_dem}]; [] is unknown, never closed.
 gio_mo_cua jsonb NOT NULL CHECK (jsonb_typeof(gio_mo_cua) = 'array'),
 gio_ghi_chu text,
 -- The same hours as an OSM opening_hours string domain/giomo reads; NULL
 -- when the feed has none or they do not read cleanly.
 gio_osm text,
 gia_nguoi_min_vnd bigint CHECK (gia_nguoi_min_vnd >= 0),
 gia_nguoi_max_vnd bigint CHECK (gia_nguoi_max_vnd >= 0),
 gia_nguoi_co_so text,
 gia_nguoi_ghi_chu text,
 gia_feed_min_vnd bigint CHECK (gia_feed_min_vnd >= 0),
 gia_feed_max_vnd bigint CHECK (gia_feed_max_vnd >= 0),
 gia_don_vi text,
 gia_ghi_chu text,
 -- Spend per person, one visit (facts.go GiaMoiNguoi). gia_uoc: estimated
 -- from dish prices (`uoc_tu_mon`), to be shown as «khoảng … (ước)».
 gia_min_vnd bigint CHECK (gia_min_vnd >= 0),
 gia_max_vnd bigint CHECK (gia_max_vnd >= 0),
 gia_uoc boolean NOT NULL DEFAULT false,
 CHECK (gia_max_vnd IS NULL OR gia_min_vnd IS NULL OR gia_max_vnd >= gia_min_vnd),
 -- [{mon, gia_vnd, ghi_chu}], at most 20 priced dishes; [] = no priced menu.
 menu jsonb NOT NULL CHECK (jsonb_typeof(menu) = 'array'),
 -- [{ten, mo_ta, lich, gia_vnd, gia_don_vi, can_dat_truoc, thoi_luong_phut}].
 hoat_dong jsonb NOT NULL CHECK (jsonb_typeof(hoat_dong) = 'array'),
 can_dat_truoc text,
 trong_nha_ngoai_troi text,
 thoi_luong_phut_min integer,
 thoi_luong_phut_max integer,
 -- The business's own public contact points (phone, page), as fed.
 lien_he text[] NOT NULL DEFAULT '{}',
 checked_at timestamptz NOT NULL,
 het_han_at timestamptz NOT NULL,
 synced_at timestamptz NOT NULL,
 landed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX place_facts_het_han ON place_facts (het_han_at);
CREATE INDEX place_facts_dong ON place_facts (place_id) WHERE con_hoat_dong = 'dong_vinh_vien';

-- The web-facts pull keeps its cursor in the same table as the place pull,
-- under its own source name. It lands no ingest_batch (the facts are an
-- upsert, not a delivery to replay), so its batch_id is NULL.
ALTER TABLE ingest_cursor ALTER COLUMN batch_id DROP NOT NULL;
