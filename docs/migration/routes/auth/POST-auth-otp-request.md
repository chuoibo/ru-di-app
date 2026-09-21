# POST /auth/otp/request

auth · core · trạng thái trong bộ nhớ: **có** — `app.state.otp_request_limit`, cửa sổ cố định 10 lần / 60 giây theo địa chỉ người gọi

## Mục đích

Phát một mã cho một số. Số điện thoại **không bao giờ được lưu và không bao giờ được ghi log** — kho chỉ giữ `derive_phone_digest(canonical, key)` (`service.py:3856`).

Thân được parse tay, cùng lý do `routes/identity.py` tự parse: lỗi validation của FastAPI **dội lại giá trị vi phạm**, mà giá trị ở đây là một số điện thoại (`routes/auth.py:1-7`). Không có số ví dụ nào xuất hiện trong file Python đó; repo guard từ chối dãy chữ số có hình dạng như vậy.

## Xác thực và quyền

Không có actor — danh tính được cấp ở đây. Thứ tự (`routes/auth.py:128-139`):

1. Middleware idempotency: `POST` ∈ `WRITE_METHODS` (`idempotency.py:76`), phạm vi `anonymous`.
2. Router: đuôi `/` → 307; `GET` → 405.
3. **Limiter địa chỉ, TRƯỚC khi đọc thân** (`routes/auth.py:134-135`): `_limit(request, "otp_request_limit", REQUEST_LIMIT)` với `REQUEST_LIMIT = 10` và `WINDOW_SECONDS = 60.0` (`routes/auth.py:32`, `:35`). Khoá là `request.client.host` hoặc `"unknown"` (`routes/auth.py:105-106`). Vượt → 429 `rate_limited`, detail `Thử lại sau một phút.` **Mọi** request tiêu một suất, kể cả những request sau đó sẽ là 422.
4. `_json_object` (`routes/auth.py:109-118`): thân không phải JSON, hoặc JSON không phải object → 422 `invalid_body`.
5. `_otp_phone` (`service.py:3825-3844`): không phải chuỗi → 422 `phone_required`; `canonical_mobile` trả None → 422 `phone_not_mobile`; thiếu khoá danh tính → 503 `identity_key_missing`.
6. Trần theo **số** (khác trần theo địa chỉ): `plan_request` (`domain/otp.py:37-67`) trên các thử thách gần đây của cùng digest → 429 `otp_resend_too_soon` hoặc 429 `otp_too_many_requests`.
7. Gửi SMS; `SmsDeliveryError` → thử thách bị đánh dấu `consumed` rồi 503 `sms_unavailable` (`service.py:3886-3898`).

Hai trần là hai thứ khác nhau và trả hai code khác nhau: `rate_limited` là cửa sổ địa chỉ áp **trước** khi đọc thân, `otp_resend_too_soon` là thời gian chờ theo số áp **sau** khi đọc thân.

## Đầu vào

Thân JSON, một trường `phone` (chuỗi). Không có model pydantic — `openapi_extra` chỉ mô tả cho tài liệu (`routes/auth.py:37-50`), việc kiểm là thủ công.

## Đầu ra

**202** `OtpRequestResponse`: `challenge_id`, `expires_in_seconds` (`code_ttl_seconds` = 300), `resend_after_seconds` (`resend_cooldown_seconds` = 60) — `service.py:3899-3903`, hằng số ở `domain/otp.py:22-27`.

Mã **không** nằm trong câu trả lời. Ở stack parity, `MOBILE_OTP_DEBUG_CODE=000000` (`scripts/parity_stacks.sh:171`) nên mọi thử thách mang cùng một mã, và nhánh so sánh lúc verify là **một** nhánh chứ không phải hai (`service.py:3851-3853`).

## Tác dụng phụ

Một hàng `otp_challenges` (`repository.py:3829`), giữ `phone_digest` và `code_digest`, không giữ số và không giữ mã. Khi gửi SMS hỏng: thêm một `record_otp_attempt(attempts=0, consumed=True)` (`repository.py:3867`) để thử thách chết ngay.

## Idempotency

`POST` ∈ `WRITE_METHODS`. Không có luỹ đẳng theo miền: hai request cùng khoá phát lại câu trả lời đã lưu, hai request khác khoá gặp trần theo số.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 429 | `rate_limited` | `Thử lại sau một phút.` | `routes/auth.py:135` |
| 422 | `invalid_body` | `Thân yêu cầu phải là JSON.` / `Thân yêu cầu phải là một đối tượng JSON.` | `routes/auth.py:113`, `:115-117` |
| 422 | `phone_required` | `Thiếu trường phone, và phải là chuỗi.` | `service.py:3828-3830` |
| 422 | `phone_not_mobile` | `Chưa đúng dạng số di động Việt Nam.` | `service.py:3833-3835` |
| 429 | `otp_resend_too_soon` | `Mã vừa được gửi. Gửi lại sau {n} giây.` | `service.py:3865-3869` |
| 429 | `otp_too_many_requests` | `Số này đã nhận quá nhiều mã. Thử lại sau {n} giây.` | `service.py:3870-3874` |
| 503 | `identity_key_missing` | | `service.py:3839-3843` |
| 503 | `sms_unavailable` | `Chưa gửi được tin nhắn lúc này, thử lại sau.` | `service.py:3896-3898` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/auth.py:121-139`; limiter `:97-102`; khoá người gọi `:105-106`; parse thân `:109-118`
- Service: `services/api/app/api/service.py:3846-3903`; chuẩn hoá số `:3825-3844`
- Domain: `services/api/app/domain/otp.py:37-67` (`plan_request`), `:22-27` (hằng số)
- Repository: `repository.py:3829` (`create_otp_challenge`), `:3850` (`recent_otp_challenges`), `:3867` (`record_otp_attempt`)

## Test đang phủ

- `services/api/tests/api/test_auth_otp.py`
- `services/api/tests/postgres/test_auth_otp_postgres.py`: `test_a_failed_delivery_leaves_a_consumed_challenge_behind` (208).

## Kịch bản parity

`parity/scenarios/w9/limiter/POST-auth-otp-request.yaml`, id `w9/limiter/post-auth-otp-request` (11 bước, `dev`, **`lane: limiter`**).

**Vì sao phải ở làn limiter, không phải lựa chọn phong cách.** Cửa sổ địa chỉ bị tiêu **trước** khi thân được đọc, nên mọi request tới route này tốn một trong mười suất — 422 cũng tốn. Bucket là state của ứng dụng, khoá theo địa chỉ người gọi, và mọi kịch bản trong một lượt đều đến từ cùng một host. Một file ở làn chính sẽ tiêu sạch cửa sổ dưới chân mọi file khác. Làn limiter khởi động kịch bản ngay sau một mốc cửa sổ và chạy cả hai stack bên trong nó, nên mỗi bên gặp limiter với con số đếm bằng không (`parity/internal/limiterlane`).

Đúng mười một request: mười đi lọt tới handler, dù sau đó trả gì, và một cái thứ mười một bị chính limiter chặn. Hai con 429 là hai code khác nhau, cố ý.

Corpus sinh: **không có**. `POST /auth/otp/request` nằm trong `excluded` của wave `w9` ở `scripts/render_parity_422_scenarios.py`, lý do: corpus 422 sinh ra 18–56 bước cho mỗi route, mà mỗi bước ở đây tiêu một suất của cửa sổ mười — corpus sẽ biến thành một bức tường 429 (vẫn `EQUAL` ở cả hai bên, nên **xanh mà vô nghĩa**) và tiêu luôn bucket của các kịch bản khác. Làn limiter phủ thay.

## Lỗi hạ tầng tìm ra khi viết kịch bản này

`MOBILE_OTP_DEBUG_CODE` được truyền cho container Python nhưng **không** cho core Go (dòng `-e` của container Python có, chuỗi gán env của core thì không). Ở stack candidate, `POST /auth/otp/request` do **Go** phục vụ, nên Go sinh mã ngẫu nhiên trong khi reference sinh `000000`; bước verify bằng mã đúng vì thế ra 201 ở reference và 422 `otp_code_invalid` ở candidate.

Không phải lỗi logic của Go — Go có đọc biến đó (`services/core/internal/sms/sms.go:26`, giải ở `:150-168`). Đúng cùng họ với `MOBILE_INTERNAL_TOKEN` mà `8c13e1c3` vừa vá: một biến tới được Python mà không tới core. Đã sửa: một khai báo duy nhất `otp_debug_code` (`scripts/parity_stacks.sh:114-118`) dùng cho cả container Python (`:175`) lẫn core (`:209-210`), để hai phía không lệch lại lần nữa.

Lỗi này **vô hình cho tới khi có kịch bản**: không kịch bản nào chạm hai route OTP, nên bốn commit qua không ai thấy. Đây đúng là điều bàn giao đã nói — trạng thái đo bằng kịch bản, không đo bằng mã.

## Chưa phủ / lưu ý cho bản Go

- **`retry_after_seconds` phụ thuộc thời gian.** `math.ceil(resend_cooldown - elapsed)` (`domain/otp.py:53-58`) nằm trong nguyên văn detail. Hai stack chỉ đồng ý khi cả hai đi từ bước tạo tới bước gửi lại trong **dưới một giây**; vượt qua mốc giây sẽ ra hai con số và một khác biệt thật trên dây. Bước `same_number_too_soon` chấp nhận rủi ro đó có chủ ý, vì nhánh này là hành vi thật phải chứng minh. Đây là cùng họ với cái bẫy `starts_on` hoà ở card `GET /contexts/{context_id}/outings`.
- **`otp_too_many_requests` chưa có bước**: cần 5 thử thách cho cùng một số trong cửa sổ 900 giây (`max_challenges_per_window`), mà cooldown 60 giây chặn không cho tạo cái thứ hai trong cùng một kịch bản. Nhánh này chỉ có test Python phủ.
- **`sms_unavailable` chưa có bước**: stack parity dùng log sender, không hỏng được.
- `identity_key_missing` không phủ: `MOBILE_PERSON_ID_KEY` luôn có trên cả hai stack (`scripts/parity_stacks.sh:169`).
- Bản Go phải tiêu limiter **trước** khi parse thân. Đổi thứ tự sẽ biến bước thứ mười một từ 429 `rate_limited` thành 422, và kịch bản bắt đúng chỗ đó.
