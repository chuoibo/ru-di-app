-- aiharness version 2: the router path's columns (obs/dinhtuyen.go). What
-- the model decided, as closed labels, what the engine did with it, the
-- tools that ran, whether the grader's corrective round ran, and the
-- verifier's verdict. Still no free text: every text column is held by a
-- CHECK to a closed list, and the one array holds only registry tool names.
-- Rows written before this version keep the defaults (no router result).
ALTER TABLE ai_turn_metrics
  ADD COLUMN nhan_guard text NOT NULL DEFAULT '' CHECK (nhan_guard IN ('','sach','chen_lenh','ngoai_pham_vi','nhay_cam')),
  ADD COLUMN y_dinh text NOT NULL DEFAULT '' CHECK (y_dinh IN ('','find_places','smalltalk','app_help','explain_screen','plan_help','remember','forget','what_you_remember','plan','hoi','chia_bill_draft')),
  ADD COLUMN so_y_dinh smallint NOT NULL DEFAULT 0 CHECK (so_y_dinh >= 0),
  ADD COLUMN tien text NOT NULL DEFAULT '' CHECK (tien IN ('','none','split_draft','money_action')),
  ADD COLUMN huong text NOT NULL DEFAULT '' CHECK (huong IN ('','tra_loi_thang','truy_hoi_mot_buoc','tac_tu','hoi_lai')),
  ADD COLUMN duong text NOT NULL DEFAULT '' CHECK (duong IN ('','tu_choi_tien','hoi_lai','thang','nhanh','tac_tu','truy_hoi')),
  ADD COLUMN cong_cu text[] NOT NULL DEFAULT '{}' CHECK (cong_cu <@ ARRAY['search_places','get_place','list_destinations','nearest_area','group_snapshot','list_group_outings','search_app_manual','explain_screen','propose_places','propose_itinerary','draft_poll','suggest_screen','my_upcoming_outings','recall_memory','remember_fact','forget_fact','what_you_remember','set_reminder']::text[]),
  ADD COLUMN vong_sua smallint NOT NULL DEFAULT 0 CHECK (vong_sua >= 0),
  ADD COLUMN ket_kiem text NOT NULL DEFAULT 'khong_chay' CHECK (ket_kiem IN ('khong_chay','dat','khong_dat','hong')),
  ADD COLUMN sinh_lai boolean NOT NULL DEFAULT false,
  ADD COLUMN so_xep_lai smallint NOT NULL DEFAULT 0 CHECK (so_xep_lai >= 0);
