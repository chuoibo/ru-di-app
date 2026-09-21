# GET /people/{person_id}/friend-requests

friends · core · trạng thái trong bộ nhớ: không có

## Mục đích

Hộp lời mời **đang chờ** của chính người gọi: đến (mặc định) hoặc đi. Lời mời đã được trả lời (chấp nhận, từ chối, chặn) biến khỏi danh sách; chấp nhận hay không chỉ hiện ở `GET /people/{person_id}/friends`.

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu danh tính → 401, kể cả khi path không phải UUID (`anonymous_non_uuid_person`).
2. Validate path `person_id` → 422 (`me_literal_me_path`: không có route `/people/me/friend-requests`, "me" bị parse như UUID).
3. `_require_permission("view_own_friends", actor, {"is_self": actor.id == person_id})` (`services/api/app/api/service.py:7117-7122`; role `member`, `services/api/app/domain/permissions.py:183`). Role trước predicate: role rỗng hoặc chỉ `group_admin` → `role_not_permitted`; đọc hộp của người khác, kể cả id không tồn tại → `is_self` (`stranger_reads_unknown_person`). Không bao giờ 404.

Không có kiểm tồn tại của `people`: một id chưa đăng ký đọc hộp của chính nó nhận 200 rỗng.

## Đầu vào

- Path `person_id: UUID` (lax; `stranger_unhyphenated_unknown` → 403 `is_self`, tức đã parse được).
- Query `direction: str = "incoming"` (`services/api/app/api/routes/friends.py:140-154`): route đổi mọi giá trị **khác đúng chữ `outgoing`** thành `incoming` (`friends.py:152-154`). `OUTGOING`, `both`, chuỗi rỗng đều là incoming. Tên tham số phân biệt hoa thường (`Direction=outgoing` bị bỏ qua). Lặp tham số thì FastAPI lấy **giá trị cuối** (`me_direction_repeated_last_is_incoming`, `me_direction_repeated_last_is_outgoing`).
- Query lạ bị bỏ qua. `Idempotency-Key` trên GET bị middleware bỏ qua (`services/api/app/api/idempotency.py:405-407`).

## Đầu ra

- **200** `FriendRequestListResponse` (`services/api/app/api/schemas.py:2119-2120`): `{"requests":[…]}`, mỗi phần tử là `FriendRequestResponse` (`schemas.py:2101-2116`) với thứ tự khoá `id`, `requester_id`, `addressee_id`, `other_person_id`, `other_display_name`, `state`, `created_at`, `decided_at`.
  - Chỉ dòng `state = 'pending'` ở phía đã chọn: incoming là `addressee_id = person_id`, outgoing là `requester_id = person_id` (`services/api/app/api/repository.py:7539-7563`). `state` luôn `pending`, `decided_at` luôn `null`.
  - Thứ tự `created_at DESC, id ASC` (`repository.py:7561`): mới nhất trước.
  - `other_person_id` là bên kia; `other_display_name` lấy từ `people` bằng một truy vấn gộp, không có tên thì rơi về chuỗi UUID (`repository.py:2529-2548`, `:7564-7581`). Tài khoản đã xoá mang tên `Người dùng đã rời`.
- Datetime `created_at`: giá trị `_now()` Python đã ghi lúc gửi, đọc lại từ `timestamptz`, ghi `…ffffffZ`.
- Không float. Không phân trang, không giới hạn số phần tử.
- Framework: 307 cho `/friend-requests/`; 405 + `allow: GET` cho POST và HEAD.

## Tác dụng phụ

Chỉ đọc (`friend_requests`, `people`). Không idempotency (GET). Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_self` | `service.py:502-504`, `:7120-7122` |

422 framework đã đo: `uuid_parsing` trên `["path","person_id"]` với `ctx.error` (`invalid character: found \`m\` at 1`, `invalid group length in group 4: expected 12, found 11`).

## Mã Python

- Route: `services/api/app/api/routes/friends.py:135-154`
- Service: `services/api/app/api/service.py:7117-7130`, `:810-821`
- Repository: `services/api/app/api/repository.py:7539-7581` (`list_friend_requests`), `:7379-7401` (`_friend_edge`), `:2529-2548` (`_display_names`)
- Quyền: `services/api/app/domain/permissions.py:183`

## Test đang phủ

- `services/api/tests/api/test_friends_routes.py`: `test_incoming_requests_are_listed_for_the_addressee` (245), `test_nobody_may_read_somebody_elses_inbox` (260), `test_a_pending_request_is_not_yet_a_friendship` (121)
- Không có ca Postgres nào gọi thẳng route này; `list_friend_requests` chỉ được phủ qua fake repository.

## Kịch bản parity

`parity/scenarios/w2/friends/GET-people-person_id-friend-requests.yaml`, id `w2/friends/get-people-person_id-friend-requests` (50 bước, `dev`):

- Thứ tự từ chối: `anonymous_unknown_person`, `anonymous_non_uuid_person` (401 trước 422).
- 403: `stranger_reads_my_inbox`, `stranger_reads_unknown_person`, `stranger_unhyphenated_unknown` (`is_self`), `me_roles_empty`, `me_roles_group_admin_only` (`role_not_permitted`).
- 422: `me_literal_me_path`, `stranger_non_uuid_person`, `stranger_short_uuid`.
- `direction`: `me_incoming_default`, `me_outgoing`, `me_direction_incoming_explicit`, `me_direction_uppercase`, `me_direction_unknown`, `me_direction_empty`, `me_direction_repeated_last_is_incoming`, `me_direction_repeated_last_is_outgoing`, `me_direction_name_is_case_sensitive`.
- Thứ tự và định hướng: ba lời mời đến tạo lần lượt (`me_incoming_default` mới nhất trước), `first_target_sees_me_incoming` (cùng dòng nhìn từ bên kia).
- Lời mời đã trả lời biến mất: `me_accepts_first`, `me_declines_second`, `me_blocks_third`, `first_target_declines_me` → `me_incoming_after_answers` (rỗng), `me_outgoing_after_decline`.
- Tài khoản đã xoá: `me_incoming_with_leaver` → `leaver_deletes_account` → `me_incoming_after_leaver_deleted` (dòng bị xoá) → `deleted_leaver_asks_me_again` → `me_incoming_names_ended_account` (`Người dùng đã rời`), `deleted_leaver_reads_own_outgoing`.
- Khác: `me_empty_inbox`, `me_with_idempotency_key` (bị bỏ qua), `trailing_slash_redirects` (307), `post_not_allowed` (405).

`parity/scenarios/w2/friends/prod-auth.yaml`: `owner_x_actor_id_header_ignored` (200 là owner dù `X-Actor-ID` là mate; `dev` sẽ 403), `mate_incoming`, `mate_reads_owner_requests` (403 `is_self`), `leaver_incoming`, `owner_outgoing_after_leaver_deleted`.

## Chưa phủ / lưu ý cho bản Go

- `direction` không bao giờ là lỗi: bản Go không được validate thành enum (422) và phải lấy giá trị **cuối** khi tham số lặp (`net/url` `Query().Get` lấy giá trị **đầu**).
- Không phân trang: một hộp rất lớn trả hết trong một lần.
- Tie-break `id ASC` chỉ lộ ra khi hai lời mời cùng micro giây; harness chạy tuần tự nên không tạo được.
- Dòng mà `other_person_id` không còn trong `people` sẽ hiện UUID dạng chuỗi làm tên; không tới được qua HTTP vì FK.
- Harness không viết hoa được id đã bind, nên "đọc hộp của chính mình bằng id chữ hoa vẫn 200" chỉ suy từ mã (so sánh `UUID`).
