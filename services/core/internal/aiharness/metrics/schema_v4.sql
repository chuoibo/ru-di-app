-- aiharness version 4: a nhay_cam turn leaves no trace a row can be read
-- for. Version 3 stored its label as '' but kept the router's other
-- columns, and '' otherwise means «no router result», so
-- nhan_guard = '' AND huong <> '' held exactly for nhay_cam turns (and the
-- direct path the label forces, duong = 'thang' with another huong, told it
-- too). The engine now records such a turn as the clean turn it cannot be
-- told from: nhan_guard 'sach', and on the direct path the canonical small
-- talk labels (huong 'tra_loi_thang', y_dinh 'smalltalk', so_y_dinh 1),
-- which a clean small-talk turn records on the same path. Rows an earlier
-- binary wrote are rewritten the same way, and a CHECK keeps the table from
-- holding router columns without a label again. Earlier versions are left
-- as they were applied; this file runs again safely.
UPDATE ai_turn_metrics
   SET nhan_guard = 'sach',
       huong = CASE WHEN tien = 'none' THEN 'tra_loi_thang' ELSE huong END,
       y_dinh = CASE WHEN tien = 'none' THEN 'smalltalk' ELSE y_dinh END,
       so_y_dinh = CASE WHEN tien = 'none' THEN 1 ELSE so_y_dinh END
 WHERE nhan_guard = '' AND (huong <> '' OR tien <> '' OR y_dinh <> '' OR so_y_dinh <> 0);
ALTER TABLE ai_turn_metrics
  DROP CONSTRAINT IF EXISTS ai_turn_metrics_nhan_nhay_cam_check,
  ADD CONSTRAINT ai_turn_metrics_nhan_nhay_cam_check
    CHECK (nhan_guard <> '' OR (huong = '' AND tien = '' AND y_dinh = '' AND so_y_dinh = 0));
