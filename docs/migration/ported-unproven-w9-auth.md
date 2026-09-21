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

## Kịch bản (thêm 2026-09-19)

Bảy hàng đã có thẻ route và kịch bản, nên chúng đã **đo được** — trước đó không:

- `docs/migration/routes/sessions/` và `docs/migration/routes/auth/`: bảy thẻ.
- `parity/scenarios/w9/sessions/`: `POST-sessions` (23 bước), `GET-sessions` (22),
  `DELETE-sessions-session_id` (21), `prod-sessions` (18, `prod`).
- `parity/scenarios/w9/limiter/`: ba route `/auth/*` — `POST-auth-otp-request` (11),
  `POST-auth-otp-verify` (33), `POST-auth-google` (11). **Bắt buộc ở làn limiter**: cửa sổ địa
  chỉ bị tiêu trước khi thân được đọc, nên mọi request tốn một suất và một file ở làn chính sẽ
  tiêu sạch cửa sổ dưới chân mọi file khác.
- `parity/scenarios/generated/w9-422/`: ba file (`post-sessions` 45 bước, `get-sessions` 5,
  `delete-sessions-session_id` 18), sinh trong ảnh ghim `mobile-parity-api:7bf58e3d`.
  `DELETE /sessions/current` và ba route `/auth/*` nằm trong `excluded` của sóng `w9`, mỗi cái
  kèm lý do.

Đã chạy trên hai stack cô lập tại `a305b25f`: **10 kịch bản, 207 bước, 0 khác biệt**
(dev 134 + limiter 55 + prod 18), `unserved=0`.

Lượt này làm lộ một lỗi hạ tầng: `MOBILE_OTP_DEBUG_CODE` chưa bao giờ được truyền cho core Go,
nên Go sinh mã OTP ngẫu nhiên còn reference sinh `000000`. Đã vá trong
`scripts/parity_stacks.sh`; chi tiết ở `docs/archive/claude/2026-09-19/cong-a305b25f-racy-va-diff.md`.

Chưa chạy trọn `gate.sh parity` xanh trên SHA sạch **sau** khi thêm các kịch bản này, nên nhãn
vẫn là `PORTED-UNPROVEN`.
