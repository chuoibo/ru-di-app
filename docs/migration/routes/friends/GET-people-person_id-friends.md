# GET /people/{person_id}/friends

friends · core · trạng thái trong bộ nhớ: không có

## Mục đích

Danh sách bạn của chính người gọi, đọc lại từ các dòng `friend_requests` ở trạng thái `accepted` theo cả hai chiều. Không có bảng `friends` riêng; `pending`, `declined`, `blocked` đều không phải tình bạn.

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu danh tính → 401, kể cả path không phải UUID.
2. Validate path `person_id` → 422.
3. `_require_permission("view_own_friends", actor, {"is_self": actor.id == person_id})` (`services/api/app/api/service.py:7132-7135`; role `member`, `services/api/app/domain/permissions.py:183`). Bạn bè cũng không đọc được danh sách của nhau (`friend_reads_my_friends` → 403 `is_self`). Role `guest` → `role_not_permitted`.

Không kiểm tồn tại: một id chưa có dòng `people` đọc chính nó nhận 200 rỗng (`unknown_person_reads_own_friends`).

## Đầu vào

- Path `person_id: UUID` (lax).
- Không có query nào được đọc; `direction`, `limit` bị bỏ qua (`me_query_string_ignored`).

## Đầu ra

- **200** `FriendListResponse` (`services/api/app/api/schemas.py:2129-2130`): `{"friends":[…]}`, mỗi phần tử `FriendSummary` (`schemas.py:2123-2126`) với thứ tự khoá `person_id`, `display_name`, `friends_since`.
  - Dòng `state = 'accepted'` mà người gọi là requester **hoặc** addressee (`services/api/app/api/repository.py:7583-7601`).
  - Thứ tự `decided_at DESC, id ASC` (`repository.py:7599`): tình bạn được chấp nhận gần nhất đứng đầu, **không** theo thời điểm gửi (`me_three_friends`).
  - `display_name` từ `people` (gộp một truy vấn, rơi về chuỗi UUID nếu thiếu, `repository.py:2529-2548`).
  - `friends_since` = `decided_at or created_at` (`service.py:7144`); CHECK `decided_state_matches_timestamp` (`services/api/app/db/models.py:2137-2140`) làm `decided_at` luôn có ở dòng `accepted`.
- Datetime: `decided_at` là `_now()` Python lúc chấp nhận, ghi `…ffffffZ`.
- Không float, không phân trang.
- Framework: 307 cho `/friends/`; 405 + `allow: GET` cho POST.

## Tác dụng phụ

Chỉ đọc (`friend_requests`, `people`). Không idempotency. Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_self` | `service.py:502-504`, `:7133-7135` |

422 framework: `uuid_parsing` trên `["path","person_id"]`.

## Mã Python

- Route: `services/api/app/api/routes/friends.py:157-168`
- Service: `services/api/app/api/service.py:7132-7148`
- Repository: `services/api/app/api/repository.py:7583-7619` (`list_friends`), `:2529-2548`
- Xoá tài khoản xoá mọi dòng của người đó: `repository.py:4829-4835`
- Gỡ chặn trả dòng về `declined`: `services/api/app/api/service.py:4608-4631`, `services/api/app/domain/friendship.py:243-256`

## Test đang phủ

- `services/api/tests/api/test_friends_routes.py`: `test_addressee_accepting_is_what_creates_the_friendship` (95), `test_a_pending_request_is_not_yet_a_friendship` (121), `test_declining_does_not_create_a_friendship` (134), `test_either_party_may_block_an_accepted_friendship` (225), `test_nobody_may_read_somebody_elses_friend_list` (273)
- `services/api/tests/postgres/test_friend_requests_postgres.py::test_friendship_is_read_back_from_the_accepted_row` (199)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py::test_lifting_a_block_gives_the_wall_back_but_not_the_friendship` (160)

## Kịch bản parity

`parity/scenarios/w2/friends/GET-people-person_id-friends.yaml`, id `w2/friends/get-people-person_id-friends` (37 bước, `dev`):

- Thứ tự từ chối: `anonymous_unknown_person`, `anonymous_non_uuid_person` (401).
- 403: `stranger_reads_my_friends`, `friend_reads_my_friends` (`is_self`), `me_roles_guest_only` (`role_not_permitted`); 422: `me_roles_unknown`, `stranger_non_uuid_person`, `me_literal_me_path`.
- Rỗng: `me_no_friends`, `unknown_person_reads_own_friends`, `me_friends_while_all_pending` (pending không phải bạn).
- Thứ tự theo `decided_at`: `second_friend_accepts`, `me_accepts_first`, `me_accepts_leaver` → `me_three_friends`; `second_friend_sees_me` (chiều ngược lại), `me_query_string_ignored`.
- Chặn và gỡ chặn: `first_friend_blocks_me` → `me_friends_after_block`, `blocker_friends_after_block` → `first_friend_lifts_block` → `me_friends_after_unblock` (không trở lại là bạn).
- Tài khoản đã xoá: `leaver_deletes_account` → `me_friends_after_leaver_deleted`, `deleted_leaver_reads_own_friends` (200 rỗng ở `dev`).
- Framework: `trailing_slash_redirects` (307), `post_not_allowed` (405).

`parity/scenarios/w2/friends/prod-auth.yaml`: `basic_scheme_list` (401 `Missing bearer session`), `lowercase_scheme_list` (200, scheme không phân biệt hoa thường), `owner_friends`, `mate_reads_owner_friends` (403), `leaver_lists_after_deleting` và `other_lists_after_sign_out` (401 `Session is not valid`).

## Chưa phủ / lưu ý cho bản Go

- Sắp theo `decided_at`, không phải `created_at`; tie-break `id ASC` chỉ lộ khi hai quyết định cùng micro giây, harness không tạo được.
- `friends_since` rơi về `created_at` là nhánh chết khi CHECK còn đó; bản Go có thể giữ nó để an toàn nhưng không được đổi thứ tự sắp.
- Ở `dev`, tài khoản đã xoá vẫn đọc được (200 rỗng); ở `prod` là 401 vì phiên đã bị thu hồi khi xoá.
- Harness không viết hoa được id đã bind; "đọc chính mình bằng id chữ hoa" chỉ suy từ mã.
