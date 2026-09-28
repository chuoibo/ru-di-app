-- aiharness version 1: one row per turn of the AI engine, enums and numbers
-- only (ADR-0037 §2.8). No column can hold words: every text column is held
-- to a closed list or a fixed shape by a CHECK, and there is no json, jsonb,
-- bytea or free varchar. The row goes with its invocation, and with every
-- invocation of a deleted account, through the foreign key; the sweeper
-- deletes rows older than thirty days.
CREATE TABLE ai_turn_metrics (
  invocation_id uuid NOT NULL REFERENCES chat_ai_invocations(id) ON DELETE CASCADE,
  lan_thu smallint NOT NULL CHECK (lan_thu BETWEEN 1 AND 100),
  bot text NOT NULL CHECK (bot IN ('nep','nhom')),
  lenh text NOT NULL CHECK (lenh IN ('hoi','plan','chia_bill')),
  guard text NOT NULL CHECK (guard IN ('proceed','restricted','refused')),
  out_guard text NOT NULL CHECK (out_guard IN ('none','chan')),
  ket_thuc text NOT NULL CHECK (ket_thuc IN ('xong','that_bai')),
  code text CHECK (code IN ('provider_unavailable','invalid_ai_result','nep_lui_man_tien','nep_khong_cham_tien','ai_tra_loi_bi_chan','ai_het_ngan_sach')),
  loi_mo_hinh text NOT NULL CHECK (loi_mo_hinh IN ('none','timeout','429','5xx','safety','bad_response','khac')),
  prompt_version char(12) NOT NULL CHECK (prompt_version ~ '^[0-9a-f]{12}$'),
  buoc smallint NOT NULL CHECK (buoc >= 0),
  so_goi_model smallint NOT NULL CHECK (so_goi_model >= 0),
  so_cong_cu smallint NOT NULL CHECK (so_cong_cu >= 0),
  tokens_in integer NOT NULL CHECK (tokens_in >= 0),
  tokens_out integer NOT NULL CHECK (tokens_out >= 0),
  tokens_cached integer NOT NULL CHECK (tokens_cached >= 0),
  tokens_thoughts integer NOT NULL CHECK (tokens_thoughts >= 0),
  luot_bo smallint NOT NULL CHECK (luot_bo >= 0),
  phieu_bo smallint NOT NULL CHECK (phieu_bo >= 0),
  ngay_mo_ho smallint NOT NULL CHECK (ngay_mo_ho >= 0),
  ky_tu_an integer NOT NULL CHECK (ky_tu_an >= 0),
  khong_dau boolean NOT NULL,
  ms_trang_thai_dau integer NOT NULL CHECK (ms_trang_thai_dau >= 0),
  ms_tien_xu_ly integer NOT NULL CHECK (ms_tien_xu_ly >= 0),
  ms_mo_hinh integer NOT NULL CHECK (ms_mo_hinh >= 0),
  ms_tong integer NOT NULL CHECK (ms_tong >= 0),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (invocation_id, lan_thu)
);
CREATE INDEX ai_turn_metrics_created_at ON ai_turn_metrics (created_at);
