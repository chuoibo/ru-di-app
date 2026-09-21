# W9 auth + sessions — 7 route, bằng chứng để rời `PORTED-UNPROVEN`

Người gộp: Claude · ADR-0029 §2.3 + ADR-0030

## 1. Lượt cổng

Kịch bản W9 (`6abc9867`: 7 kịch bản tay + 3 corpus 422 sinh trong ảnh ghim) chạy trong lượt
`gate.sh --strict parity` `ĐẠT` tại `46e7627d` — lượt xanh trọn vẹn đã lật 11 route outings. Trong đó:

| kịch bản | bước | kết quả |
|---|---|---|
| `w9/sessions/post-sessions` | 23 | EQUAL |
| `w9/sessions/get-sessions` | 22 | EQUAL |
| `w9/sessions/delete-sessions-session_id` | 21 | EQUAL |
| `w9/sessions/prod-sessions` (auth **prod**) | 18 | EQUAL |
| `w9/limiter/post-auth-otp-request` | 11 | EQUAL |
| `w9/limiter/post-auth-otp-verify` | 33 | EQUAL |
| `w9/limiter/post-auth-google` | 11 | EQUAL |
| `generated/w9-422/post-sessions` | 45 | EQUAL |
| `generated/w9-422/delete-sessions-session_id` | 18 | EQUAL |
| `generated/w9-422/get-sessions` | 5 | EQUAL |

Cả ba tầng của lượt ấy đều xanh: canary dev và prod `identity equal, every exercised damage caught`,
probe `stale=0`, `unserved=0`. Số đầy đủ ở `docs/migration/ported-w7-outings.md` §4.

## 2. Hai đột biến của người gộp — và một sai lầm của tôi phải ghi ra

Đặc tả: `services/core/tools/w9-http-nguoi-gop/dot-bien.json`.

| đột biến | chạy bằng | kết quả | đối chứng |
|---|---|---|---|
| `current` của `GET /sessions` luôn `false` | `--auth prod` + DSN | **1 DIFF, 10 khác biệt** | 1 EQUAL, 0 |
| số lần thử OTP lấy nhầm trường liền kề | `--lane limiter` | **1 DIFF, 5 khác biệt** | 3 EQUAL, 0 |

Chữ ký của cái thứ hai đi thẳng ra câu chữ người dùng đọc: reference «Còn **4** lần thử», candidate «Còn
**1** lần thử».

**Lần chạy đầu cả hai đột biến đều «sống sót», và đó là lỗi của tôi chứ không phải bằng chứng.** Tôi chạy chúng
trên làn chính ở chế độ dev, nơi **không kịch bản nào với tới được chúng**: `current` chỉ có nghĩa khi có bearer
thật (chế độ prod), còn số lần thử OTP chỉ lộ ở làn limiter. Một đột biến sống sót **chưa nói gì** cho tới khi
chứng minh được kịch bản chạm tới nó — luật này tôi đã tự đặt từ sóng W7 và vẫn quên.

## 3. Những gì KHÔNG được chứng minh

- 7 route này vẫn `owner: python`. Chưa route nào được Go phục vụ trong cấu hình thật.
- SMS đi qua `LogSender`, **không** qua cổng SMS thật; Google đi qua verifier cấu hình bằng client id của
  harness, **không** qua Google thật. Cái được chứng minh là **ánh xạ** của Go, không phải hành vi của nhà cung
  cấp.
- `MOBILE_OTP_DEBUG_CODE` cố định ở cả hai stack. Đường sinh mã ngẫu nhiên thật không nằm trong tầm đo.
