# GET /contexts/{context_id}

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Đổi một id nhóm lấy tên, theme và loại của nhóm, chỉ cho thành viên ACTIVE. Id nhóm đi trong link chia sẻ, nên membership được quyết **trước** khi đọc dòng: người lạ nhận cùng một 403 dù id có thật hay không (`services/api/app/api/service.py:1706-1724`). Một pair được gọi bằng tên người kia, đọc từ roster lúc gọi.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_context`); `X-Actor-Contexts` không phải UUID → 422 `invalid_actor_contexts` (`contexts_header_not_uuid`).
2. Path `context_id: UUID` → 422 (`stranger_non_uuid_context`); dạng `%7B…%7D` được chấp nhận (`stranger_braced_unknown_context` → 403).
3. `_require_permission("view_context_members", {"is_group_member": …})` (`service.py:1716-1720`; `services/api/app/domain/permissions.py:311-314`): 403 `is_group_member` cho id không tồn tại (`stranger_unknown_context`), id thật (`stranger_real_context`), pair của người khác (`stranger_reads_pair`), người được mời (`invitee_reads`), người đã rời (`leaver_reads_after_leaving`), người đã xoá tài khoản (`mate_reads_pair_after_deletion`). `X-Actor-Contexts` không được tin (`stranger_claims_context_header`). Role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200 (`mate_roles_group_admin_only`).
4. `get_context` trả None → 404 `context_not_found` (`service.py:1721-1723`): không tới được qua HTTP với Postgres (thành viên ACTIVE nghĩa là dòng tồn tại); chỉ repository giả trong test chạm tới.

## Đầu vào

- Path `context_id` (UUID lax).
- Không query, không body (`undeclared_query_ignored` → 200).

## Đầu ra

- **200** `ContextResponse` (`services/api/app/api/schemas.py:450-457`), thứ tự khoá `id`, `display_name`, `created_by_id`, `created_at`, `theme`, `kind`, `counterpart`.
- Nhóm: `display_name` như đã lưu, `kind: "group"`, `counterpart: null`; `theme` phản ánh `PATCH` gần nhất (`mate_reads_after_theme`).
- Pair (`service.py:1389-1419`): `list_members` (dòng chưa đóng) → `counterpart_of` chọn người không phải actor khi còn đúng một người khác (`services/api/app/domain/direct.py:80-86`):
  - còn người kia → `display_name` = tên người kia đọc lúc gọi, `counterpart: {"id","display_name"}` (`owner_reads_pair`, `mate_reads_pair`, `owner_reads_pair_after_rename`);
  - người kia đã xoá tài khoản (dòng của họ bị đóng) → `display_name: "Thành viên"` (`ANONYMOUS_COUNTERPART`, `direct.py:29`), `counterpart: null` (`owner_reads_pair_after_mate_deleted`).
- `created_at`: DB `now()` lúc tạo; pydantic UTC `Z`.
- Framework: 307 cho `/` cuối (`location: http://<Host>/contexts/<id>`); `POST`, `DELETE` → 405 `allow: PATCH`.

## Tác dụng phụ

- Chỉ đọc: `memberships` (`is_member`), `contexts`, với pair thêm `memberships` + `people`. Không khoá, không ghi, không idempotency.
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:1716-1720`, `:504` |
| 404 | `context_not_found` | `Context does not exist` (không tới được) | `service.py:1723` |

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:131-141`
- Service: `services/api/app/api/service.py:1706-1724` (`get_context`), `:1389-1419` (`_context_response`)
- Domain: `services/api/app/domain/permissions.py:311-314`; `services/api/app/domain/direct.py:29`, `:58-103`
- Repository: `services/api/app/api/repository.py:2596-2600`, `:2749-2782`
- Model: `services/api/app/db/models.py:1054-1117`

## Test đang phủ

- `services/api/tests/api/test_context_read.py`: `test_a_member_can_read_the_group_name_from_its_id_alone` (36), `test_a_stranger_holding_the_id_is_refused_the_name` (50), `test_a_stranger_is_refused_before_the_row_is_read` (66), `test_a_member_of_a_group_with_no_row_gets_404_not_500` (81, chỉ repository giả)
- `services/api/tests/api/test_direct_messages.py::test_reading_the_pair_as_a_context_names_it_after_the_other_person` (144)
- `services/api/tests/api/test_context_settings.py::test_a_new_group_starts_on_the_default_theme` (38)

## Kịch bản parity

`parity/scenarios/w3/contexts/GET-contexts-context_id.yaml`, id `w3/contexts/get-contexts-context_id` (44 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `contexts_header_not_uuid`, `stranger_non_uuid_context`.
- 403: `stranger_braced_unknown_context`, `stranger_unknown_context`, `stranger_real_context`, `stranger_claims_context_header`, `invitee_reads`, `mate_roles_empty`, `leaver_reads_after_leaving`, `stranger_reads_pair`, `mate_reads_pair_after_deletion`.
- Nhóm: `owner_reads`, `mate_reads`, `mate_roles_group_admin_only`, `mate_sets_theme` + `mate_reads_after_theme`, `owner_reads_group_after_mate_deleted`.
- Pair: `owner_asks_mate_to_be_friends`, `mate_accepts_friendship`, `owner_opens_pair_with_mate`, `owner_reads_pair`, `mate_reads_pair`, `mate_renames_self` + `owner_reads_pair_after_rename`, `mate_deletes_account` + `owner_reads_pair_after_mate_deleted`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects` (307), `post_not_allowed`, `delete_not_allowed` (405).

`prod` (`w3/contexts/prod-auth`): `actor_headers_ignored_read_context` (401), `owner_lowercase_scheme_reads` (200), `stranger_reads_claiming_owner` (403), `mate_reads_after_leaving` (403), `owner_revokes_session` + `owner_after_revoke` (401).

## Chưa phủ / lưu ý cho bản Go

- Tên pair được suy ra mỗi lần đọc, không lưu; `contexts.display_name` của pair là chuỗi rỗng. Bản Go không được trả chuỗi rỗng đó.
- `counterpart_of` chỉ trả người kia khi roster chưa đóng còn **đúng** hai người; mọi trường hợp khác (người kia rời hay xoá tài khoản) cho `"Thành viên"` và `counterpart: null`.
- Nhánh 404 không tới được qua HTTP; bản Go phải giữ membership trước lookup để không thành oracle cho id nhóm.
