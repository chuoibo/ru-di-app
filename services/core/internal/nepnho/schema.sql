-- Nếp's long-term memory ledger, version 1 (design 05 §6, ADR-0043 draft).
-- internal/nepnho is the only writer of every nep_* table.
--
-- The words of a fact never live here: they live in the mem0 sidecar's
-- Milvus collection (services/ai-infer). Postgres holds the consent, one
-- receipt per fact (ids, kinds, times), the typed app events of the last 30
-- days, hashed tombstones and the deletion ledger, and it is the source of
-- truth for what exists: a fact the sidecar returns without a live receipt
-- here is never shown to anyone.

-- Consent. Off by default; on only with the version of the disclosure text
-- the person agreed to (ADR-0041 §2.5). su_kien_at paces event batches.
CREATE TABLE nep_cai_dat (
  person_id uuid PRIMARY KEY REFERENCES people(id),
  nho boolean NOT NULL DEFAULT false,
  cong_bo_ban smallint CHECK (cong_bo_ban > 0),
  cong_bo_at timestamptz,
  su_kien_at timestamptz,
  cap_nhat_at timestamptz NOT NULL DEFAULT now(),
  CHECK (NOT nho OR (cong_bo_ban IS NOT NULL AND cong_bo_at IS NOT NULL))
);

-- One receipt per fact held in the sidecar: id = the mem0 memory id. No text.
-- dang_xoa_at hides the fact the moment a forget is asked; deleted_at is the
-- receipt that the rows were counted gone in Milvus.
CREATE TABLE nep_su_that (
  id uuid PRIMARY KEY,
  person_id uuid NOT NULL REFERENCES people(id),
  loai text NOT NULL CHECK (loai IN ('thich_danh_muc','thich_dia_diem','ne_dia_diem','diem_den_quen','khung_gio_hay_di','phuong_tien','thoi_luong_chang','nhip_len_keo','dieu_da_dan')),
  nguon text NOT NULL CHECK (nguon IN ('hanh_vi','noi_ro','hoi_dap')),
  tu_luc timestamptz NOT NULL,
  den_luc timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  dang_xoa_at timestamptz,
  deleted_at timestamptz,
  CHECK (den_luc IS NULL OR den_luc > tu_luc),
  CHECK (deleted_at IS NULL OR dang_xoa_at IS NOT NULL)
);
CREATE INDEX nep_su_that_nguoi ON nep_su_that(person_id) WHERE deleted_at IS NULL;

-- Tombstones: HMAC-SHA256 (server key) of a forgotten fact's normalised
-- words, so the same fact is not written again. Kept 365 days.
CREATE TABLE nep_quen (
  person_id uuid NOT NULL REFERENCES people(id),
  khoa_bam bytea NOT NULL CHECK (octet_length(khoa_bam) = 32),
  den timestamptz NOT NULL,
  PRIMARY KEY (person_id, khoa_bam)
);

-- Closed, typed app events (POST /me/nep/su-kien): a kind and typed ids or
-- enums, never words. Kept 30 days.
CREATE TABLE nep_su_kien (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  person_id uuid NOT NULL REFERENCES people(id),
  loai text NOT NULL CHECK (loai IN ('mo_dia_diem','luu_dia_diem','bo_luu','them_chang','chon_phuong_tien','chon_thoi_luong','loc_danh_muc','chon_diem_den','tao_keo','check_in')),
  luc timestamptz NOT NULL,
  dia_diem_id text CHECK (dia_diem_id ~ '^[A-Za-z0-9_-]{1,64}$'),
  diem_den_id text CHECK (diem_den_id ~ '^[A-Za-z0-9_-]{1,64}$'),
  danh_muc text CHECK (danh_muc ~ '^[a-z][a-z0-9_]{0,31}$'),
  phuong_tien text CHECK (phuong_tien IN ('motorbike','car','walk')),
  thoi_luong_phut smallint CHECK (thoi_luong_phut BETWEEN 5 AND 1440),
  ghi_at timestamptz NOT NULL DEFAULT now(),
  CHECK (
    (loai IN ('mo_dia_diem','luu_dia_diem','bo_luu','them_chang','check_in')
      AND dia_diem_id IS NOT NULL AND diem_den_id IS NULL AND danh_muc IS NULL AND phuong_tien IS NULL AND thoi_luong_phut IS NULL)
    OR (loai = 'chon_diem_den'
      AND diem_den_id IS NOT NULL AND dia_diem_id IS NULL AND danh_muc IS NULL AND phuong_tien IS NULL AND thoi_luong_phut IS NULL)
    OR (loai = 'loc_danh_muc'
      AND danh_muc IS NOT NULL AND dia_diem_id IS NULL AND diem_den_id IS NULL AND phuong_tien IS NULL AND thoi_luong_phut IS NULL)
    OR (loai = 'chon_phuong_tien'
      AND phuong_tien IS NOT NULL AND dia_diem_id IS NULL AND diem_den_id IS NULL AND danh_muc IS NULL AND thoi_luong_phut IS NULL)
    OR (loai = 'chon_thoi_luong'
      AND thoi_luong_phut IS NOT NULL AND dia_diem_id IS NULL AND diem_den_id IS NULL AND danh_muc IS NULL AND phuong_tien IS NULL)
    OR (loai = 'tao_keo'
      AND dia_diem_id IS NULL AND diem_den_id IS NULL AND danh_muc IS NULL AND phuong_tien IS NULL AND thoi_luong_phut IS NULL)
  )
);
CREATE INDEX nep_su_kien_nguoi ON nep_su_kien(person_id, luc);

-- The deletion ledger and its receipts. One row per forget (one fact, all
-- facts, the account). A row is the saga's state: 'cho' until the sidecar
-- deleted, answered remaining 0, and Go's own listing no longer held what was
-- deleted; then 'xong' with the receipt (da_xoa_at, so_da_xoa: ids and counts
-- only). loi is the last failure, a closed code; lan_thu counts attempts.
--
-- When an account's deletion completes, every row of that person here loses
-- its person_id and keeps nguoi_bam instead (HMAC-SHA256 of the person id
-- under the server key): the receipt stays provable by whoever holds the key
-- and the id, and no nep_* row names the deleted person any more.
CREATE TABLE nep_xoa (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  person_id uuid REFERENCES people(id),
  nguoi_bam bytea CHECK (octet_length(nguoi_bam) = 32),
  pham_vi text NOT NULL CHECK (pham_vi IN ('mot','tat_ca','tai_khoan')),
  su_that_id uuid,
  buoc text NOT NULL DEFAULT 'cho' CHECK (buoc IN ('cho','xong')),
  lan_thu integer NOT NULL DEFAULT 0 CHECK (lan_thu >= 0),
  loi text CHECK (loi IN ('dich_vu_vang','dich_vu_loi','con_hang')),
  so_da_xoa integer CHECK (so_da_xoa >= 0),
  tao_at timestamptz NOT NULL DEFAULT now(),
  chay_luc timestamptz NOT NULL DEFAULT now(),
  da_xoa_at timestamptz,
  CHECK ((pham_vi = 'mot') = (su_that_id IS NOT NULL)),
  CHECK (buoc = 'cho' OR (da_xoa_at IS NOT NULL AND so_da_xoa IS NOT NULL)),
  CHECK (person_id IS NOT NULL OR (buoc = 'xong' AND nguoi_bam IS NOT NULL))
);
-- One open deletion per person and scope (and fact): asking twice while the
-- first still runs is the same request.
CREATE UNIQUE INDEX nep_xoa_dang_mo ON nep_xoa(person_id, pham_vi, COALESCE(su_that_id::text, '')) WHERE buoc = 'cho';
CREATE INDEX nep_xoa_den_luot ON nep_xoa(chay_luc) WHERE buoc = 'cho';

-- Every attempt of an open deletion is a message on the `memory` lane of the
-- job outbox (internal/jobs), due at chay_luc: the row is inserted with
-- attempt 0, and each failure raises lan_thu and pushes chay_luc out, which
-- enqueues the next attempt in the same transaction. The worker's consumer
-- runs it (nepnho.Kho.XuLyTin); the periodic pass (nepnho.Kho.DinhKyXoa) is
-- the safety net while the broker is down. Only the id reaches the broker.
CREATE FUNCTION nep_xoa_enqueue() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.buoc = 'cho' AND (TG_OP = 'INSERT' OR NEW.lan_thu <> OLD.lan_thu) THEN
    PERFORM jobs_them('memory', NEW.id, NEW.lan_thu, NEW.chay_luc, NULL);
  END IF;
  RETURN NULL;
END
$$;
CREATE TRIGGER nep_xoa_enqueue AFTER INSERT OR UPDATE OF lan_thu ON nep_xoa
  FOR EACH ROW EXECUTE FUNCTION nep_xoa_enqueue();

-- Account deletion (design 05 §6 «Xoá»): the moment people.deleted_at is set,
-- by the Go or the Python erasure path alike, every nep_* row that holds a
-- preference goes in the same transaction, every receipt is hidden, and one
-- 'tai_khoan' deletion is queued for the sidecar and Milvus. The Go pass
-- (the memory lane, nepnho.Kho.XuLyTin) retries it until the sidecar and Go's
-- own listing count zero rows for the person.
CREATE FUNCTION nep_xoa_nguoi() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM nep_su_kien WHERE person_id = NEW.id;
  DELETE FROM nep_quen WHERE person_id = NEW.id;
  DELETE FROM nep_cai_dat WHERE person_id = NEW.id;
  UPDATE nep_su_that SET dang_xoa_at = COALESCE(dang_xoa_at, now())
   WHERE person_id = NEW.id AND deleted_at IS NULL;
  INSERT INTO nep_xoa(person_id, pham_vi) VALUES (NEW.id, 'tai_khoan') ON CONFLICT DO NOTHING;
  RETURN NULL;
END
$$;
CREATE TRIGGER nep_xoa_nguoi AFTER UPDATE OF deleted_at ON people FOR EACH ROW
  WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
  EXECUTE FUNCTION nep_xoa_nguoi();
