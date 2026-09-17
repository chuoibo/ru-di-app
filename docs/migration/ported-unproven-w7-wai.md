# 36 hàng `PORTED-UNPROVEN` (W7 HTTP, WAI, healthz)

Review 2026-09-17 (`REQUEST_CHANGES`, `431003ef`): nhãn `PORTED` trên các hàng này
hứa một lượt `gate.sh parity` đầy đủ trên SHA sạch, bằng chứng mà 108 hàng PORTED
trước đó đều có. 36 hàng mới chỉ có handler + unit test trên cây bẩn.

Trạng thái `PORTED-UNPROVEN` (ADR-0029 §2.3) giữ chúng là candidate
(`MOBILE_CORE_CANDIDATE_ROUTES=ported`) nhưng không nói dối trên manifest.

Gỡ: sau T1 (W9 + stub) rebase `--3way`, chạy `gate.sh parity` một lượt (T5),
rồi lật từng hàng sang `PORTED`.

Nửa routed của `POST /outings/{outing_id}/itinerary/preview` (`_route`,
`schedule`, `suggest_order`, `savings`, `feasible`, `segments`,
`late_fixed_stop`) vẫn chưa chứng minh cho tới stub Valhalla.

Năm hàng framework (`GET /openapi.json`, `/docs`, `/docs/oauth2-redirect`,
`/redoc`, `MOUNT /static`) không nằm đây: chúng là `DEFERRED`, xem
`docs/migration/deferred-framework.md`.
