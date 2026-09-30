-- What vnlocal's AI passes say about each place, pulled from the feed:
-- the ten categories (vnlocal `place_danh_muc`, danh-muc@1, RuDi request
-- PR #4) and the search attributes (vnlocal `place_lam_giau`, lam-giau@1,
-- RuDi request PR #5). Owner decision 2026-09-29: vnlocal runs both for
-- every new place and every place whose input changes; RuDi only reads.
--
-- One row per catalogue place, keyed by ingest.PlaceID of the feed's id, no
-- foreign key to `places` (the feeds move on separate cursors). ingest is
-- the only writer. Not copied: `ly_do` (the model's prose about why) and
-- `nguon_sha256` (vnlocal's own input hash, not comparable with anything
-- RuDi computes).
--
-- The codes are the closed lists of domain/tuvung, checked in Go before a
-- row is written (facts: danhMucReject, lamGiauReject) and here as a
-- backstop.
CREATE TABLE place_danh_muc (
 place_id text PRIMARY KEY CHECK (place_id ~ '^vnl-'),
 source_ref text NOT NULL UNIQUE,
 danh_muc text[] NOT NULL CHECK (cardinality(danh_muc) BETWEEN 1 AND 10)
   CHECK (danh_muc <@ ARRAY['quan_an','an_vat','cafe','bar_nhau','cho_am_thuc','thien_nhien',
                           'van_hoa','vui_choi','trai_nghiem','luu_tru','khong_ro'])
   CHECK (NOT ('khong_ro' = ANY(danh_muc)) OR cardinality(danh_muc) = 1),
 model text,
 prompt_version text,
 schema_version text NOT NULL,
 checked_at timestamptz NOT NULL,
 synced_at timestamptz NOT NULL,
 landed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- The search attributes as vnlocal's model answered them. rag/nap reads a
-- row through the same checks as an answer of its own enrichment
-- (DocTraLoi), and uses it for a place instead of place_enrichments: vnlocal
-- only writes a row for a place RuDi had none for, or whose text changed
-- since. An allergen-free claim from here is never certain until a person
-- reviews it, like any automatic enrichment.
CREATE TABLE place_lam_giau (
 place_id text PRIMARY KEY CHECK (place_id ~ '^vnl-'),
 source_ref text NOT NULL UNIQUE,
 di_ung text[] NOT NULL,
 an_kieng text[] NOT NULL,
 khi_chat text[] NOT NULL CHECK (cardinality(khi_chat) <= 4),
 mon_chinh text[] NOT NULL CHECK (cardinality(mon_chinh) <= 5),
 chen_lenh boolean NOT NULL,
 tin_cay text NOT NULL CHECK (tin_cay IN ('cao','vua','thap')),
 model text,
 prompt_version text,
 schema_version text NOT NULL,
 checked_at timestamptz NOT NULL,
 synced_at timestamptz NOT NULL,
 landed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
