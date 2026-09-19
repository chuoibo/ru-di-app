# POST /auth/otp/verify

auth · core · trạng thái trong bộ nhớ: **có** — `app.state.otp_verify_limit`, cửa sổ cố định 30 lần / 60 giây theo địa chỉ người gọi

## Mục đích

Tiêu một mã lấy một phiên. **Mọi lý do thử thách đã chết đều là một 404** (`service.py:3908`).

Bốn kết cục `not_found`, `consumed`, `expired`, `burned_already` cố ý không phân biệt được với người gọi API (`domain/otp.py:8-12`), vì nói cho kẻ đoán biết rằng một mã **từng** có thật đáng giá với họ hơn là với người chỉ cần xin mã mới. Chúng phân biệt trong domain để test nhìn thấy.

## Xác thực và quyền

Không có actor. Thứ tự (`routes/auth.py:149-165`, `service.py:3905-3997`):

1. Middleware idempotency: `POST` ∈ `WRITE_METHODS`, phạm vi `anonymous`.
2. Router: đuôi `/` → 307; `GET` → 405.
3. **Limiter địa chỉ, TRƯỚC khi đọc thân** (`routes/auth.py:153-154`): `VERIFY_LIMIT = 30` / 60 giây (`routes/auth.py:33`, `:35`).
4. `_json_object` → 422 `invalid_body`.
5. `challenge_id` parse tay (`routes/auth.py:156-162`): không phải chuỗi, hoặc không phải UUID → 422 `challenge_id_invalid`, detail `Thiếu hoặc sai challenge_id.` Parse tay chứ không dùng pydantic vì cùng lý do với route bên cạnh: thông báo lỗi của FastAPI dội lại giá trị.
6. `_otp_phone` → 422 `phone_required` / `phone_not_mobile` / 503 `identity_key_missing`.
7. `code` phải là chuỗi không rỗng sau `strip()` → 422 `code_required` (`service.py:3910-3911`).
8. Tra thử thách; **thử thách phát cho số khác, với người gọi này, là không có thử thách** — so `phone_digest` bằng `hmac.compare_digest` rồi đặt `challenge = None` (`service.py:3915-3920`).
9. `plan_verify` quyết kết cục; bốn kết cục chết → 404 `otp_challenge_not_found`; `burned` → 429 `otp_too_many_attempts`; `wrong_code` → 422 `otp_code_invalid` kèm số lần còn lại.

## Đầu vào

Thân JSON: `challenge_id` (chuỗi UUID), `phone` (chuỗi), `code` (chuỗi). Không có model pydantic; `openapi_extra` ở `routes/auth.py:51-68` chỉ là tài liệu.

## Đầu ra

**201** `SessionResponse` (`schemas.py:1047-1070`), `issued_via` = `otp`.

- Số này thuộc về ai là việc của `account_identities`, **không** phải của id dẫn xuất (ADR-0023 §2.2.2, `service.py:3961-3965`). Dẫn xuất chỉ quyết định một tài khoản **mới** nhận id nào.
- Một số có tài khoản cũ đã xoá **quay lại như một người mới** (`service.py:3972-3975`): hồi sinh hàng đã ẩn danh sẽ trao cho người giữ số kế tiếp nhóm và sổ của người khác.
- `is_new_person` là `True` khi cửa này vừa tạo hàng `people`.
- Hai lượt verify đua nhau trên một số hoàn toàn mới: `RepositoryConflict` bị nuốt và `is_new` quay về `False` (`service.py:3980-3982`).

## Tác dụng phụ

`record_otp_attempt` trên mọi lượt đi tới đó (`service.py:3943-3948`) — số lần thử nằm **trong hàng**, không trong bộ nhớ. Khi mở: có thể `create_person`, luôn `upsert_account_identity`, và một hàng `account_sessions` với `issued_via='otp'` và `issued_from_invite_id` NULL (ràng buộc `invite_matches_via`, `db/models.py:2828-2830`).

## Idempotency

`POST` ∈ `WRITE_METHODS`. Không luỹ đẳng theo miền: mã đã tiêu là mã chết, lần hai là 404.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 429 | `rate_limited` | `Thử lại sau một phút.` | `routes/auth.py:154` |
| 422 | `invalid_body` | | `routes/auth.py:113`, `:115-117` |
| 422 | `challenge_id_invalid` | `Thiếu hoặc sai challenge_id.` | `routes/auth.py:162` |
| 422 | `phone_required` / `phone_not_mobile` | | `service.py:3828-3835` |
| 422 | `code_required` | `Thiếu mã xác minh.` | `service.py:3911` |
| 404 | `otp_challenge_not_found` | `Mã không còn hiệu lực. Hãy yêu cầu mã mới.` | `service.py:3937-3941` |
| 429 | `otp_too_many_attempts` | `Sai mã quá nhiều lần. Hãy yêu cầu mã mới.` | `service.py:3950-3953` |
| 422 | `otp_code_invalid` | `Mã chưa đúng. Còn {n} lần thử.` | `service.py:3957-3959` |
| 503 | `identity_key_missing` | | `service.py:3839-3843` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/auth.py:142-165`
- Service: `services/api/app/api/service.py:3905-3997`
- Domain: `services/api/app/domain/otp.py` (`plan_verify`, hằng số `:22-27`: `max_attempts` 5, `code_ttl_seconds` 300)
- Repository: `repository.py:3863` (`get_otp_challenge`), `:3867` (`record_otp_attempt`), `:4387` (`get_account_identity`), `:4397` (`upsert_account_identity`), `:3536` (`create_account_session`)

## Test đang phủ

- `services/api/tests/api/test_auth_otp.py`
- `services/api/tests/postgres/test_auth_otp_postgres.py`: `test_a_new_number_becomes_a_person_a_session_and_one_identity_row` (93), `test_a_friend_named_this_number_first_so_the_login_lands_in_that_row` (109), `test_wrong_guesses_are_counted_in_the_row_not_in_memory` (144), `test_a_number_that_deleted_and_came_back_is_findable_by_phone_again` (220).

## Kịch bản parity

`parity/scenarios/w9/limiter/POST-auth-otp-verify.yaml`, id `w9/limiter/post-auth-otp-verify` (33 bước, `dev`, **`lane: limiter`**). Lý do phải ở làn limiter giống hệt route anh em, và ở đây còn gắt hơn: **không có `get_actor` đứng trước để từ chối cái gì miễn phí**, nên mọi request đều tiêu.

Hai bước đầu gọi `POST /auth/otp/request` — route khác, cửa sổ khác, nên **tiêu 0 của 30**. Sau đó ba mươi request đi tới handler và một cái thứ ba mươi mốt bị cửa sổ chặn.

- Hình dạng thân và trường: `spend_01_body_broken_json` … `spend_10_code_not_string`.
- Thử thách không phải của người gọi: `spend_11_unknown_challenge`, `spend_12_challenge_of_another_number`.
- Đốt một thử thách: `spend_13_wrong_code_1` … `spend_17_wrong_code_5` (trần `max_attempts` là 5), rồi `spend_18_burned_challenge` cho thấy mã **đúng** trên thử thách đã cháy vẫn là một 404.
- Cửa mở rồi chết: `spend_19_right_code_opens_a_session`, `spend_20_spent_code_replayed`.
- `spend_21` … `spend_30` lặp năm hình dạng từ chối để tiêu nốt cửa sổ — đúng lối `parity/scenarios/w2/limiter/POST-friends-lookup-limit.yaml` đã dùng.
- `thirty_first_is_rate_limited`.

Corpus sinh: **không có** — cùng lý do `excluded` như route anh em.

## Lỗi hạ tầng tìm ra khi viết kịch bản này

`MOBILE_OTP_DEBUG_CODE` được truyền cho container Python nhưng **không** cho core Go (dòng `-e` của container Python có, chuỗi gán env của core thì không). Ở stack candidate, `POST /auth/otp/request` do **Go** phục vụ, nên Go sinh mã ngẫu nhiên trong khi reference sinh `000000`; bước verify bằng mã đúng vì thế ra 201 ở reference và 422 `otp_code_invalid` ở candidate.

Không phải lỗi logic của Go — Go có đọc biến đó (`services/core/internal/sms/sms.go:26`, giải ở `:150-168`). Đúng cùng họ với `MOBILE_INTERNAL_TOKEN` mà `8c13e1c3` vừa vá: một biến tới được Python mà không tới core. Đã sửa: một khai báo duy nhất `otp_debug_code` (`scripts/parity_stacks.sh:114-118`) dùng cho cả container Python (`:175`) lẫn core (`:209-210`), để hai phía không lệch lại lần nữa.

Lỗi này **vô hình cho tới khi có kịch bản**: không kịch bản nào chạm hai route OTP, nên bốn commit qua không ai thấy. Đây đúng là điều bàn giao đã nói — trạng thái đo bằng kịch bản, không đo bằng mã.

## Chưa phủ / lưu ý cho bản Go

- **Thử thách hết hạn** (`expires_at <= now`, TTL 300 giây) không có bước: kịch bản phải kết thúc trong một cửa sổ 60 giây nên không chờ được. Chỉ test Python phủ. `burned_already` thì có phủ, qua `spend_18`.
- `otp_code_invalid` mang **số lần còn lại** trong nguyên văn detail; bản Go phải đếm giống hệt, kể cả thứ tự tăng `attempts` so với lúc quyết kết cục.
- `hmac.compare_digest` trên digest đã được domain Go chép đúng CPython: hằng thời gian theo **nội dung**, **không** hằng thời gian theo **độ dài** (so `len` trước). Không đổi — xem `docs/migration/ported-unproven-w9-auth.md`.
- Bản Go **không** chép `PlanVerify`/`compareDigest` vào handler; biên hết hạn và trần lần thử nằm trong `otp`.
- Nhánh đua hai verify trên một số mới (`RepositoryConflict` → `is_new=False`) không có bước: kịch bản làn limiter **không được** dùng `concurrent` (`parity/cmd/parity/main.go:270-273`), vì reference lặp lại một kịch bản burst và mỗi lượt lặp lại tiêu cửa sổ một lần nữa.
