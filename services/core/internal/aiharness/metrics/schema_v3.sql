-- aiharness version 3: the router's nhay_cam label is never stored. A row
-- names its invocation, and the invocation names the person
-- (chat_ai_invocations.person_id), so an inferred self-harm or abuse label
-- in this table would be a sensitive trait linked to one person for the
-- 30 days a row lives. The engine records it as '' (no label); any row an
-- earlier binary wrote with it is cleared, and the CHECK no longer admits
-- it. Version 2 is left as it was applied.
UPDATE ai_turn_metrics SET nhan_guard = '' WHERE nhan_guard = 'nhay_cam';
ALTER TABLE ai_turn_metrics
  DROP CONSTRAINT ai_turn_metrics_nhan_guard_check,
  ADD CONSTRAINT ai_turn_metrics_nhan_guard_check CHECK (nhan_guard IN ('','sach','chen_lenh','ngoai_pham_vi'));
