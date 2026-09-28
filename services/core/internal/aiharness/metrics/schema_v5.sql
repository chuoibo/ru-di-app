-- aiharness version 5: the group assistant on the Go engine (slice 9). Its
-- split draft is a path of its own, 'nhap_chia_bill': one structured
-- reading of the shared messages, a fixed template around it, nothing
-- written. Still a closed list: no free text can be stored.
ALTER TABLE ai_turn_metrics
  DROP CONSTRAINT IF EXISTS ai_turn_metrics_duong_check,
  ADD CONSTRAINT ai_turn_metrics_duong_check
    CHECK (duong IN ('','tu_choi_tien','hoi_lai','thang','nhanh','tac_tu','truy_hoi','nhap_chia_bill'));
