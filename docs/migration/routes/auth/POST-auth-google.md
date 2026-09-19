# POST /auth/google

auth · core · trạng thái trong bộ nhớ: **có** — `app.state.google_login_limit`, cửa sổ cố định 10 lần / 60 giây theo địa chỉ người gọi

## Mục đích

Một ID token của Google đổi lấy một phiên (ADR-0016). **Địa chỉ e-mail không bao giờ tới được đây** (`service.py:4002`).

Thứ tự là toàn bộ lập luận an ninh của route (`service.py:4004-4009`): một host không có client id **từ chối trước khi đọc token** (503); một token mà verifier không bảo lãnh là **một** 401 dù lý do là gì; và `sub` được tra trong `account_identities` chứ không ở đâu khác — một `sub` lần đầu là một người **mới**, kể cả khi đã có người cùng e-mail, vì `GoogleClaims` không mang e-mail để làm lựa chọn đó.

## Xác thực và quyền

Không có actor. Thứ tự (`routes/auth.py:175-185`, `service.py:3999-4030`):

1. Middleware idempotency: `POST` ∈ `WRITE_METHODS`, phạm vi `anonymous`.
2. Router: đuôi `/` → 307; `GET` → 405.
3. **Limiter địa chỉ, TRƯỚC khi đọc thân** (`routes/auth.py:180-181`): `GOOGLE_LIMIT = 10` / 60 giây (`routes/auth.py:34`, `:35`).
4. `_json_object` → 422 `invalid_body`.
5. `verifier is None` → **503 `google_not_configured`, trước khi `id_token` được nhìn tới** (`service.py:4011-4016`). `get_google_verifier` đọc `app.state.google_verifier`, vắng thì `None` (`deps.py:312-314`).
6. `id_token` không phải chuỗi, hoặc rỗng sau `strip()` → 422 `id_token_required`.
7. `verifier.verify` ném `GoogleTokenInvalid` → **một** 401 `google_token_invalid` cho mọi lý do.

## Đầu vào

Thân JSON, một trường `id_token` (chuỗi). Không có model pydantic; `openapi_extra` ở `routes/auth.py:69-82` chỉ là tài liệu.

## Đầu ra

**201** `SessionResponse` (`schemas.py:1047-1070`), `issued_via` = `google`, `issued_from_invite_id` NULL.

## Tác dụng phụ

`sub` chưa từng thấy: `uuid.uuid4()` cho người mới, một hàng `people`, một hàng `account_identities` provider `google`. `sub` đã thấy: làm mới ràng buộc, **không** đụng người. Luôn một hàng `account_sessions`.

## Idempotency

`POST` ∈ `WRITE_METHODS`. Đăng nhập hai lần trên cùng `sub` tạo hai phiên và giữ nguyên một người.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 429 | `rate_limited` | `Thử lại sau một phút.` | `routes/auth.py:181` |
| 422 | `invalid_body` | | `routes/auth.py:113`, `:115-117` |
| 503 | `google_not_configured` | `Máy chủ chưa cấu hình đăng nhập Google.` | `service.py:4011-4016` |
| 422 | `id_token_required` | `Thiếu id_token.` | `service.py:4018` |
| 401 | `google_token_invalid` | `Google không xác nhận lượt đăng nhập này. Thử lại.` | `service.py:4022-4026` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/auth.py:168-185`
- Service: `services/api/app/api/service.py:3999-4030`
- Verifier: `services/api/app/api/google_identity.py`; seam ở `services/api/app/api/deps.py:312-314`
- Repository: `repository.py:4387` (`get_account_identity`), `:4397` (`upsert_account_identity`), `:3536` (`create_account_session`)

## Test đang phủ

- `services/api/tests/api/test_auth_google.py`
- `services/api/tests/postgres/test_auth_google_postgres.py`: `test_a_first_login_writes_one_person_one_binding_one_google_session` (92), `test_a_second_login_on_the_same_sub_refreshes_the_binding_not_the_person` (117), `test_the_database_refuses_binding_one_sub_to_a_second_person` (144), `test_a_phone_person_and_a_google_person_stay_two_people` (167).

## Kịch bản parity

`parity/scenarios/w9/limiter/POST-auth-google.yaml`, id `w9/limiter/post-auth-google` (11 bước, `dev`, **`lane: limiter`**). Lý do làn limiter giống hai route anh em.

**Route này chứng minh được ít hơn hai route kia, và phải nói thẳng ra.** Stack parity không mang client id Google nào, nên `get_google_verifier` trả `None` và service từ chối bằng 503 `google_not_configured` **trước khi** nhìn token. Cái được chứng minh ở đây đúng bằng ba thứ, theo thứ tự: cửa sổ địa chỉ trước, parse JSON sau, host chưa cấu hình thứ ba — và token không bao giờ được đọc. Mười bước tiêu cửa sổ đi qua đủ hình dạng thân để cho thấy thứ tự đó không đổi; bước thứ mười một là 429.

Corpus sinh: **không có** — cùng lý do `excluded` như hai route anh em.

## Chưa phủ / lưu ý cho bản Go

- **Không chứng minh được trên host này**: `id_token_required`, `google_token_invalid`, và toàn bộ nhánh sau verifier — `sub` lần đầu thành người mới, `sub` cũ làm mới ràng buộc mà không đụng người, một người phone và một người google vẫn là hai người. Cần một verifier đã cấu hình. **Một lượt xanh ở đây không được đọc là bằng chứng cho những nhánh đó**; chúng chỉ có test Python phủ.
- Bản Go giữ đúng thứ tự: verifier `nil` → 503 trước khi đọc token (`docs/migration/ported-unproven-w9-auth.md`). Đọc token trước rồi mới kiểm cấu hình sẽ đổi 503 thành 422 ở bước `spend_04_id_token_missing` và kịch bản bắt đúng chỗ đó.
- Bản Go phải giữ **một** 401 cho mọi lý do verifier từ chối. Tách theo lý do là một kênh phụ cho kẻ đoán.
- Thân OTP/Google được parse tay (IR `null`) để không dội lại số điện thoại hay token; bản Go giữ nguyên tính chất đó.
