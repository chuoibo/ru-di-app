# GET /sessions

sessions · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Tài khoản này đang đăng nhập ở đâu (ADR-0023 §2.5). Danh sách là **của chính người gọi và của không ai khác**.

Header `Authorization` được đọc **lần nữa** ngay cạnh actor, vì một lý do: màn hình phải nói được hàng nào là «phiên này», mà actor mang một *người*, không mang một *phiên* (`routes/sessions.py:67-72`).

Không có nhãn thiết bị, không IP, không user agent — bảng không lưu thứ nào trong đó, và bịa «iPhone của Minh» từ một header không ai xác minh là một câu sản phẩm không đứng sau được (`schemas.py:982-989`).

## Xác thực và quyền

Thứ tự (`routes/sessions.py:57-75`, `service.py:4489-4517`):

1. Không có middleware idempotency (`GET` không nằm trong `WRITE_METHODS`, `idempotency.py:76`).
2. Router: đuôi `/` → 307; `PATCH` → 405; `HEAD` được Starlette trả lời như GET không thân.
3. `get_actor` → 401 (`deps.py:102-106` dev, `:143` prod); 422 `invalid_actor_id`, `invalid_actor_roles` (`deps.py:147`, `:152-154`).
4. `_require_permission("manage_own_sessions", actor, {"is_self": True})` (`service.py:4499`; `domain/permissions.py:219`): cần vai trò `member` **và** `is_self`. Vai trò rỗng hoặc chỉ `guest` → 403 `role_not_permitted`.

Không có bậc 404: một tài khoản không có phiên nào trả về danh sách rỗng.

## Đầu vào

Không path, không query, không thân. Một header tuỳ chọn: `Authorization`.

## Đầu ra

**200** `SessionListResponse` (`schemas.py:1000-1001`): `{"sessions": [...]}`.

Mỗi phần tử là `SessionSummary` (`schemas.py:982-998`), đúng thứ tự khoá: `id`, `issued_via`, `created_at`, `expires_at`, `current`.

- Hàng nào có mặt: `list_account_sessions` (`repository.py:4604-4619`) lấy phiên **chưa thu hồi và chưa hết hạn** — `revoked_at IS NULL AND expires_at > now`. `now` là đồng hồ của service, không phải của database, nên một test đứng được hai phía của mốc hết hạn.
- Thứ tự: `ORDER BY created_at DESC, id` (`repository.py:4617`).
- `current` **được tính từ token của chính request này**, không phải đọc từ kho (`service.py:4500-4504`): digest của bearer được tra ra một hàng, và `current` là `row.id == current_id`. Ở chế độ `dev` không có bearer nên **mọi hàng trả về `false`** — đó là câu trả lời đúng, không phải tính năng thiếu: không phiên nào đang được dùng (`service.py:4494-4496`).
- Một bearer còn sống nhưng của người khác cũng cho `current` toàn `false`, vì hàng đó không nằm trong danh sách của người gọi.

## Tác dụng phụ

Không ghi gì. Hai truy vấn khi có bearer (danh sách, rồi tra digest), một khi không có.

## Idempotency

Không áp dụng (GET).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `deps.py:143` (dev), `:102`/`:106` (prod) |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:147`, `:152-154` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:502-504`, `permissions.py:219` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/sessions.py:57-75`
- Service: `services/api/app/api/service.py:4489-4517`
- Repository: `repository.py:4604-4619` (`list_account_sessions`), `:3562` (`get_account_session_by_digest`)
- Schema: `schemas.py:982-998`, `:1000-1001`
- Quyền: `services/api/app/domain/permissions.py:219`
- Bearer: `services/api/app/api/deps.py:93-107` (`bearer_token`)

## Test đang phủ

- `services/api/tests/api/test_sessions_and_blocking.py`: `test_the_session_list_is_only_ones_own_and_hides_dead_rows` (44).
- `services/api/tests/postgres/test_session_bootstrap_postgres.py`: `test_signing_out_kills_the_session` (527).

## Kịch bản parity

`parity/scenarios/w9/sessions/GET-sessions.yaml`, id `w9/sessions/get-sessions` (22 bước, `dev`):

- Trước khi có ai: `anonymous_lists`, `actor_id_not_uuid`, `roles_unknown`, `owner_lists_with_empty_roles`, `owner_lists_as_guest_role`, `owner_lists_empty`.
- Fixture rồi một phiên qua cửa lời mời: `owner_names_self`, `owner_names_friend`, `owner_creates_group`, `owner_creates_trip`, `owner_invites_friend_by_name`, `friend_redeems_invitation`.
- Nội dung: `friend_lists_one` (một hàng, `issued_via` `invite`, `current` false vì dev không gửi bearer), `owner_lists_still_empty` (danh sách là của riêng người gọi).
- Hàng thứ hai: `owner_creates_second_trip`, `owner_invites_friend_to_second_trip`, `friend_redeems_second_invitation`, `friend_lists_two`. Phải là **chuyến thứ hai**: `uq_outing_invites_person` duy nhất trên `(outing_id, invited_person_id)` (`db/models.py:1424-1430`), nên lời mời đích danh thứ hai trên cùng chuyến là 409 và không mang token để bind.
- `friend_lists_with_junk_bearer`: ở dev, danh tính vẫn đến từ `X-Actor-ID`; một bearer rác không được đổi người hỏi.
- Framework: `trailing_slash`, `patch_not_allowed`, `head_lists`.

`parity/scenarios/w9/sessions/prod-sessions.yaml` (18 bước, `prod`) phủ phần chỉ nói được ở prod: `owner_lists_own_session`, `other_lists_own_session`, `owner_lists_with_others_bearer`, và mọi hình dạng header bị từ chối.

Corpus sinh: `generated/w9-422/get-sessions.yaml`.

## Chưa phủ / lưu ý cho bản Go

- **Phiên hết hạn** (`expires_at <= now`) không có bước: kịch bản không lùi được đồng hồ. Chỉ `test_the_session_list_is_only_ones_own_and_hides_dead_rows` phủ nhánh này.
- `ORDER BY created_at DESC, id`: hai phiên tạo trong **cùng một micro-giây** rơi về `id`, mà `id` là uuid ngẫu nhiên khác nhau ở hai stack — một khác biệt giả. Kịch bản đặt hai lần đổi lời mời cách nhau nguyên một vòng HTTP nên không chạm mốc đó; bản Go vẫn phải giữ đúng cặp khoá sắp xếp để trường hợp hoà còn quyết được. Cùng một cái bẫy đã ghi ở card `GET /contexts/{context_id}/outings` cho `starts_on`.
- Bản Go phải tính `current` từ token của **request hiện tại**, không được lưu cờ đó vào hàng.
