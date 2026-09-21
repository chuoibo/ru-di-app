# 32 hàng `PORTED-UNPROVEN` (WAI, W9 HTTP, healthz)

Review 2026-09-17 (`REQUEST_CHANGES`, `431003ef`): nhãn `PORTED` trên các hàng này
hứa một lượt `gate.sh parity` đầy đủ trên SHA sạch, bằng chứng mà 108 hàng PORTED
trước đó đều có. 36 hàng mới chỉ có handler + unit test trên cây bẩn.

Trạng thái `PORTED-UNPROVEN` (ADR-0029 §2.3) giữ chúng là candidate
(`MOBILE_CORE_CANDIDATE_ROUTES=ported`) nhưng không nói dối trên manifest.

Gỡ: một lượt `gate.sh --strict parity` đầy đủ trên SHA sạch, cộng canary, probe
và hai đột biến của người gộp, rồi lật từng hàng sang `PORTED`.

**11 hàng outings đã rời danh sách này 2026-09-20** (36 → 32): lượt cổng tại
`46e7627d` `ĐẠT` trọn vẹn — 383 EQUAL, 0 DIFF, 0 INFRA; canary dev và prod đều
`identity equal, every exercised damage caught`; probe `stale=0`; hai đột biến
của người gộp đỏ đúng chỗ với đối chứng xanh. Bằng chứng:
`docs/migration/ported-w7-outings.md`.

32 hàng còn lại — WAI 24, W9 HTTP 7, healthz 1 — **chưa có kịch bản parity nào**:
không có thư mục `wai` trong `parity/scenarios/`, và W9 mới có kịch bản từ
`6abc9867` nhưng chưa qua một lượt cổng nào. Viết mã port xong **không** làm route
tiến trạng thái; trạng thái đo bằng kịch bản chứ không đo bằng mã.

Năm hàng framework (`GET /openapi.json`, `/docs`, `/docs/oauth2-redirect`,
`/redoc`, `MOUNT /static`) không nằm đây: chúng là `DEFERRED`, xem
`docs/migration/deferred-framework.md`.
