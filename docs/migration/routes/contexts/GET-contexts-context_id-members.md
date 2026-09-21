# GET /contexts/{context_id}/members

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Danh sách hiện tại của một nhóm: mọi dòng `memberships` chưa đóng (`invited` và `active`), cũ nhất trước, kèm tên đọc từ `people` lúc gọi. Chỉ thành viên ACTIVE đọc được; người đã rời mất quyền ngay (`services/api/app/api/service.py:1726-1743`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_context`); role lạ → 422 thắng lỗi path (`roles_unknown_non_uuid_context`).
2. Path `context_id: UUID` → 422 `uuid_parsing` (`stranger_non_uuid_context`, `stranger_short_uuid`); `urn:uuid:` được chấp nhận (`stranger_urn_unknown_context` → 403).
3. `_require_permission("view_context_members", {"is_group_member": …})` (`service.py:1732-1736`; `services/api/app/domain/permissions.py:311-314`). Không tồn tại và có thật cùng 403 `is_group_member` (`stranger_unknown_context`, `stranger_real_context`). `X-Actor-Contexts` không được tin (`stranger_claims_context_header`). Người được mời chưa nhận (`invitee_lists`), người đã rời (`leaver_lists_after_leaving`), tài khoản đã xoá (`ghost_lists_after_deletion`) → 403. Role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200 (`mate_roles_group_admin_only`).

## Đầu vào

- Path `context_id` (UUID lax).
- Không query, không body: query lạ và body hỏng bị bỏ qua (`undeclared_query_ignored`, `undeclared_malformed_body_ignored` → 200).

## Đầu ra

- **200** `MembershipListResponse` (`services/api/app/api/schemas.py:1142-1144`): `{"context_id", "members"}`.
  - `context_id` là UUID của path ở dạng chuẩn (chữ thường, có gạch), không phải chuỗi đã gửi.
  - `members[]`: `MembershipResponse` (`schemas.py:1116-1139`), thứ tự khoá `id`, `context_id`, `person_id`, `display_name`, `state`, `role`, `invited_by_id`, `joined_at`, `left_at`, `created_at`.
  - Chỉ dòng `left_at IS NULL`, sắp `ORDER BY created_at, id` (`services/api/app/api/repository.py:2749-2767`). Người rời rồi được mời lại có dòng `invited` mới ở cuối (`owner_lists_after_reinvite`); dòng cũ đã rời không xuất hiện (`owner_lists_after_leave`). Tài khoản đã xoá biến mất vì xoá tài khoản đóng dòng (`owner_lists_after_deletion`; `repository.py:4840-4852`).
  - `display_name` đọc bằng **một** câu SELECT cho cả danh sách (`repository.py:2529-2548`), nên đổi tên có hiệu lực ngay (`owner_lists_after_rename`). Fallback `str(person_id)` khi tên rỗng không tới được (`people.display_name` NOT NULL, `PUT /people` không cho rỗng).
- Datetime pydantic UTC `Z`: `created_at` (DB `now()`), `joined_at`/`left_at` (đồng hồ Python) hoặc `null`.
- Framework: 307 cho `/` cuối (`location: http://<Host>/contexts/<id>/members`); `PUT` → 405 `allow: POST`.

## Tác dụng phụ

- Chỉ đọc: `memberships` (`is_member`), `memberships` + `people`. Không khoá, không ghi, không idempotency (GET).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 422 | (framework) `uuid_parsing` | `Input should be a valid UUID, invalid character: found `k` at 1` | FastAPI |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:1732-1736`, `:504` |

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:105-115`
- Service: `services/api/app/api/service.py:1726-1743` (`list_context_members`), `services/api/app/api/service.py:795-807` (`_wire_membership`)
- Domain: `services/api/app/domain/permissions.py:311-314`
- Repository: `services/api/app/api/repository.py:2749-2767`, `:2769-2782`, `:2529-2548`, `:2240-2264`

## Test đang phủ

- `services/api/tests/postgres/test_member_display_name_postgres.py`: `test_the_roster_names_every_member_it_returns` (98), `test_two_members_who_registered_different_names_stay_apart` (124), `test_the_members_route_puts_the_name_on_the_wire` (179), `test_a_stranger_cannot_read_a_groups_roster` (244), `test_someone_who_left_stops_reading_the_roster` (259)

## Kịch bản parity

`parity/scenarios/w3/contexts/GET-contexts-context_id-members.yaml`, id `w3/contexts/get-contexts-context_id-members` (43 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `roles_unknown_non_uuid_context`, `stranger_non_uuid_context`, `stranger_short_uuid`.
- 403: `stranger_unknown_context`, `stranger_urn_unknown_context`, `stranger_real_context`, `stranger_claims_context_header`, `invitee_lists`, `mate_roles_empty`, `leaver_lists_after_leaving`, `ghost_lists_after_deletion`.
- Đường vui: `owner_lists_alone`, `owner_lists_everyone`, `mate_lists`, `mate_roles_group_admin_only`, `owner_lists_after_leave`, `owner_lists_after_reinvite`, `owner_lists_after_deletion`, `owner_lists_after_rename`.
- Framework và đầu vào thừa: `undeclared_query_ignored`, `undeclared_malformed_body_ignored`, `trailing_slash_redirects`, `put_not_allowed`.
- Chuẩn bị: sáu `register_*`, `owner_creates_group`, mời/nhận `mate`, `leaver`, `ghost`, mời `invitee`, `leaver_leaves`, `owner_reinvites_leaver`, `ghost_deletes_account`, `mate_renames_self`.

`prod` (`w3/contexts/prod-auth`): `junk_bearer_members` (401), `owner_lists_members` (200).

## Chưa phủ / lưu ý cho bản Go

- **Thứ tự của một pair không tất định**: `create_pair_context` chèn hai dòng trong một transaction nên `created_at` bằng nhau, và `ORDER BY created_at, id` rơi vào uuid4 ngẫu nhiên. Hai stack sẽ trả thứ tự khác nhau, nên kịch bản không liệt kê thành viên của pair. Bản Go phải giữ đúng `ORDER BY created_at, id` (so sánh uuid của Postgres) để trùng Python trên cùng dữ liệu.
- Nhóm thường: dòng admin của người tạo và mỗi lời mời nằm ở các transaction khác nhau, nên thứ tự tất định.
- Fallback tên bằng id không tới được qua HTTP.
