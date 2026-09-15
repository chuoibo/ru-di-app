# GET /contexts/{context_id}/memories/{memory_id}/comments

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Cuộc trò chuyện dưới một dòng tường kỷ niệm, cũ nhất trước, sau cùng cổng `view_group_memories` với tường (`services/api/app/api/routes/memories.py:220-239`). Không phân trang.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_both`); role lạ → 422 thắng lỗi path (`roles_unknown_non_uuid_both`).
2. Hai path → một mảng 422 (`stranger_non_uuid_both`).
3. `_memory_of_member(…, "view_group_memories")` (`services/api/app/api/service.py:5116-5145`; `services/api/app/domain/permissions.py:427-430`): 403 trước lookup (`stranger_unknown_both`, `stranger_real_memory`, `stranger_claims_context_header`, `invitee_reads`, `leaver_reads_after_leaving`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200.
4. Memory không thuộc nhóm của path → 404 `memory_not_found` (`mate_unknown_memory`, `mate_other_groups_memory_via_this_context`).

## Đầu vào

- Path `context_id`, `memory_id` (UUID lax).
- Không query, không body (`undeclared_query_ignored` với `limit=1` vẫn trả đủ).

## Đầu ra

- **200** `MemoryCommentListResponse` (`services/api/app/api/schemas.py:1578-1580`), thứ tự khoá `memory_id`, `comments`.
  - `memory_id`: UUID path dạng chuẩn.
  - `comments[]`: `MemoryCommentResponse` (`schemas.py:1560-1575`), `ORDER BY created_at, id` (`services/api/app/api/repository.py:5388-5397`); chỉ bình luận của memory này (`owner_reads_second_photo`). Memory chưa có bình luận → `[]` (`owner_no_comments_yet`).
  - `display_name` đọc từ `people` **mỗi bình luận một lần** lúc gọi (`service.py:5244-5256`): đổi tên có hiệu lực ngay (`owner_reads_after_rename`); người viết đã rời vẫn giữ tên (`owner_reads_after_leaver_left`); người viết đã xoá tài khoản → `"Người dùng đã rời"` (`owner_reads_after_author_deleted`).
  - `created_at`: đồng hồ Python lúc viết; pydantic UTC `Z`.
- Framework: 307 cho `/` cuối; `DELETE` → 405 `allow: POST`.

## Tác dụng phụ

- Chỉ đọc: `memberships`, `memories` + hai truy vấn đếm, `memory_comments`, `people` × số bình luận. Không ghi, không idempotency, không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5137-5141`, `:504` |
| 404 | `memory_not_found` | `Memory does not exist` | `service.py:5142-5144` |

## Mã Python

- Route: `services/api/app/api/routes/memories.py:220-239`
- Service: `services/api/app/api/service.py:5232-5242` (`list_memory_comments`), `:5244-5256`, `:5116-5145`
- Domain: `services/api/app/domain/permissions.py:427-430`; `services/api/app/domain/account_lifecycle.py:55`
- Repository: `services/api/app/api/repository.py:5388-5397`, `:5307-5327`, `:2525-2527`

## Test đang phủ

- `services/api/tests/postgres/test_memory_reactions_postgres.py`: `test_an_outsider_cannot_read_the_comments` (290), `test_the_comments_come_back_oldest_first` (572), `test_a_comment_from_another_group_never_appears` (656), `test_a_group_comment_never_reaches_the_guest_page` (681)

## Kịch bản parity

`parity/scenarios/w3/memories/GET-contexts-context_id-memories-memory_id-comments.yaml`, id `w3/memories/get-contexts-context_id-memories-memory_id-comments` (45 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown`, `anonymous_non_uuid_both`, `roles_unknown_non_uuid_both`, `stranger_non_uuid_both`.
- 403: `stranger_unknown_both`, `stranger_real_memory`, `stranger_claims_context_header`, `invitee_reads`, `mate_roles_empty`, `leaver_reads_after_leaving`.
- 404: `mate_unknown_memory`, `mate_other_groups_memory_via_this_context`.
- Đọc: `owner_no_comments_yet`, `mate_reads`, `owner_reads_second_photo`, `mate_roles_group_admin_only`, `owner_reads_after_leaver_left`, `owner_reads_after_rename`, `owner_reads_after_author_deleted`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects`, `delete_not_allowed`.
- Chuẩn bị: năm `register_*`, hai nhóm, mời/nhận, `owner_posts_photo`, `owner_posts_second_photo`, `owner_posts_in_other_group`, năm bình luận, `leaver_leaves`, `mate_renames_self`, `mate_deletes_account`.

`prod` (`w3/memories/prod-auth`): `owner_reads_comments` (200).

Chạy với làn DB bật; `cursor` của ba bước chuẩn bị đăng ảnh được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` như ở card `POST /contexts/{id}/memories` (ADR-0029 §2.4). Corpus 422 sinh tự động: `parity/scenarios/generated/w3-422/get-contexts-context_id-memories-memory_id-comments.yaml` (32 bước).

## Chưa phủ / lưu ý cho bản Go

- Hai bình luận cùng `created_at` sẽ sắp theo `id` (uuid4); đồng hồ Python tuần tự không tạo ra trường hợp này.
- Tra tên từng bình luận (N+1) không thấy qua HTTP; bản Go có thể gộp nếu giữ nguyên kết quả, kể cả `display_name: null` khi thiếu dòng `people`.
