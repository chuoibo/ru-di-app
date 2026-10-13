-- aiharness version 8: explicit random source searches share the existing
-- private metrics shape. Only the closed command enum widens.
SET LOCAL lock_timeout = '5s';
ALTER TABLE ai_turn_metrics
  DROP CONSTRAINT ai_turn_metrics_lenh_check,
  ADD CONSTRAINT ai_turn_metrics_lenh_check
    CHECK (lenh IN ('hoi','plan','chia_bill','random'));
