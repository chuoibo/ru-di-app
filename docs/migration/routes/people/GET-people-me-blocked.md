# GET /people/me/blocked

people · core · trạng thái trong bộ nhớ: không có

## Mục đích

Những người mà người gọi đang chặn, để gỡ. Không bao giờ là những người đang chặn người gọi: danh sách đó chính là điều `BLOCKED_IS_SILENT` giữ kín.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:144-155`, `services/api/app/api/service.py:4633-4647`), đo trên stack:

1. Router: đuôi `/` → 307 (`trailing_slash`); `POST` → 405 (`post_not_allowed`).
2. `get_actor` → 401, 422 `invalid_actor_id` / `invalid_actor_roles`.
3. `_require_permission("view_own_blocks", {"is_self": True})` (`service.py:4637`; `services/api/app/domain/permissions.py:223`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_lists_without_roles`, `owner_lists_as_group_admin`).
4. Không tra hàng `people` của người gọi: chưa đăng ký cũng 200 rỗng (`owner_lists_unregistered`).

## Đầu vào

Không path, không thân. Query và thân lạ bị bỏ qua (`owner_lists_with_query_and_json_body`).

## Đầu ra

**200** `{"blocked": [{"person_id", "display_name", "blocked_at"}, …]}` (`services/api/app/api/schemas.py:1004-1011`).

- Hàng: `friend_requests` có `state='blocked' AND decided_by_id = me`, xếp `decided_at DESC, id` (`services/api/app/api/repository.py:4720-4744`). Không phân biệt ai là requester.
- `person_id`: người còn lại của cạnh; `display_name`: tên hiện tại của người đó qua `_display_names` (`repository.py:2529-2548`, rơi về chuỗi id khi tên rỗng hoặc không có hàng); `blocked_at`: `decided_at`, rơi về `created_at`.
- Chặn lại một người đã chặn (200 không ghi) không đổi `decided_at` nên không đổi thứ tự (`owner_blocks_first_again`, `owner_lists_after_rename`).
- Người bị chặn đổi tên → tên mới (`first_renames`). Gỡ chặn → hàng biến mất (`owner_lists_after_lift`). Người bị chặn xoá tài khoản → cạnh bị xoá cùng họ, hàng biến mất (`owner_lists_after_erasure`).
- Người bị chặn luôn thấy danh sách của mình không chứa người chặn (`first_lists_nothing`, `watcher_lists_own`).

## Tác dụng phụ

Chỉ đọc `friend_requests`, `people`.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4637` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:144-155`
- Service: `services/api/app/api/service.py:4633-4647`
- Repository: `services/api/app/api/repository.py:4720-4744`, `:2529-2548`
- Quyền: `services/api/app/domain/permissions.py:223`

## Test đang phủ

- `services/api/tests/api/test_sessions_and_blocking.py`: `test_blocking_is_idempotent_and_only_the_blocker_lifts_it` (85)

## Kịch bản parity

`parity/scenarios/w10/people/GET-people-me-blocked.yaml`, id `w10/people/get-people-me-blocked` (32 bước, `dev`):

- Thứ tự: `anonymous_lists`, `actor_id_not_uuid`, `roles_unknown`, `owner_lists_without_roles`, `owner_lists_as_group_admin`, `owner_lists_unregistered`, `owner_lists_empty`.
- Ba cạnh khởi đầu khác nhau (không cạnh; bạn do người kia hỏi; lời mời của mình) và một người chặn ngược: `owner_lists_three` (mới nhất trước), `watcher_lists_own`, `first_lists_nothing`, `owner_lists_with_query_and_json_body`.
- `owner_blocks_first_again` + `first_renames` + `owner_lists_after_rename` (thứ tự giữ, tên mới), `owner_unblocks_second` + `owner_lists_after_lift`, `third_ends_account` + `owner_lists_after_erasure`.
- Framework: `trailing_slash`, `post_not_allowed` (405 `allow: GET`).

Đọc lại cũng có trong `POST-people-person_id-block.yaml` (`target_lists_own_blocks`, `owner_lists_own_blocks`, `owner_lists_blocks_after_variants`), `DELETE-people-person_id-block.yaml`, `DELETE-people-me-world.yaml` (`blocker_lists_blocks_after`, `leaver_lists_blocks_after`), crossreplay và concurrency của hai route chặn, `prod-auth.yaml` (`basic_scheme_lists_blocked`, `owner_lists_blocked`).

Corpus sinh: `generated/w10-422/get-people-me-blocked.yaml` (5 bước).

## Chưa phủ / lưu ý cho bản Go

- Thứ tự `decided_at DESC, id`: hai lần chặn cùng `decided_at` xếp theo `friend_requests.id` (uuid4 ngẫu nhiên); ba lần chặn đồng thời trong `concurrency/POST-…` có `decided_at` khác nhau nên không chạm tie.
- Tên rơi về chuỗi id khi `display_name` rỗng; không tới được qua HTTP vì tên tối thiểu 1 ký tự.

## Lỗi Python (chỉ báo, không sửa)

- Không có lỗi hành vi đo được. Ghi nhận: bức tường tới một người đã xoá tài khoản biến mất khỏi danh sách (cạnh bị xoá), nhưng chặn lại id đó tạo một hàng mới (xem thẻ POST).
