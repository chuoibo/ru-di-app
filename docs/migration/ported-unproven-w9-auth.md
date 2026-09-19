# 7 hàng W9 `PORTED-UNPROVEN` (sessions + auth)

Bảy hàng `POST/GET /sessions`, `DELETE /sessions/current`, `DELETE /sessions/{session_id}`,
`POST /auth/otp/request`, `POST /auth/otp/verify`, `POST /auth/google` có handler Go
mỏng gọi `authsteps` (domain `8ebc2334`) trên `repo.Repository` (repo `ca38f0e0`).
`owner` vẫn `python`. Nhãn không phải `PORTED` và không phải `LIVE-GO`.

Gỡ: `gate.sh parity` trên SHA sạch (T5, người gộp), rồi lật từng hàng sang `PORTED`.

Handler không chép `PlanVerify` / `compareDigest`. Biên hết hạn và trần lần thử
nằm trong `otp` (`expires_at <= now`, `attempts >= MaxAttempts`).

`hmac.compare_digest` trên digest **đã** được domain chép đúng CPython: constant-time
theo nội dung, **không** constant-time theo độ dài (so `len` trước). Không đổi.

Limiter địa chỉ chạy trước thân JSON. `DELETE /sessions/current` không gọi `get_actor`.
Verifier Google `nil` → 503 `google_not_configured` trước khi đọc token. Body OTP/Google
parse tay (IR `null`) để không echo số điện thoại. `POST /sessions` đi pydantic
`invite_token` `min_length=1, max_length=512`.

Không chạy `gate.sh parity` trên SHA này.
