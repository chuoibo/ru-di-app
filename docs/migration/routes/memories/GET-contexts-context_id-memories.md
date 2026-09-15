# GET /contexts/{context_id}/memories

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tường kỷ niệm của nhóm, mới nhất trước, phân trang keyset. Ảnh và check-in chung một dòng thời gian; `kind` và `place_id` chỉ **thu hẹp** trong nhóm sau khi quyền đã được quyết (`services/api/app/api/routes/memories.py:83-106`). Mỗi dòng mang tổng tim, tổng bình luận và "tôi đã thả tim chưa", đếm lại mỗi lần đọc.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path và lỗi query (`anonymous_non_uuid_context`, `anonymous_bad_limit`).
2. Path `context_id` + query (`limit`, `kind`, `place_id`, `before`) validate cùng lúc → một mảng 422 (`stranger_non_uuid_and_bad_query`). Lỗi query thắng 403 (`stranger_unknown_context_bad_limit`).
3. `_require_permission("view_group_memories", {"is_group_member": …})` (`services/api/app/api/service.py:4839-4843`; `services/api/app/domain/permissions.py:427-430`): 403 cho nhóm không tồn tại / thật, người được mời, người đã rời, header `X-Actor-Contexts` tự khai (`stranger_unknown_context`, `stranger_real_context`, `invitee_reads`, `leaver_reads_after_leaving`, `stranger_claims_context_header`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200.
4. `before` được giải mã **sau** quyền (`service.py:4844-4847`): người lạ gửi cursor hỏng nhận 403 (`stranger_unknown_context_bad_cursor`), thành viên nhận 422 `invalid_cursor`.

## Đầu vào

- Path `context_id` (UUID lax).
- Query (`memories.py:92-95`):
  - `limit: int`, 1..100, mặc định 50: `0` → `greater_than_equal`, `101` → `less_than_equal`, `x` và `1.5` → `int_parsing`. Khoá lặp lấy giá trị cuối (`limit_repeated_last_wins`: `limit=0&limit=2` → 200).
  - `kind: Literal["photo","checkin"] | None`: sai → `literal_error` với `ctx.expected` = `'photo' or 'checkin'` (`kind_video`, `kind_uppercase`, `kind_empty` với `kind=`).
  - `place_id: str | None`, tối đa 200 (`place_201_letters` → `string_too_long`). Khớp chính xác; id không có → 200 rỗng (`place_unknown`).
  - `before: str | None`, không giới hạn độ dài. Giải mã bởi `decode_cursor` (`services/api/app/api/cursors.py:22-46`): thêm `=` cho đủ bội 4, `b64decode(altchars="-_", validate=True)`, UTF-8, tách ở `|` đầu tiên, `datetime.fromisoformat`, `uuid.UUID`; timestamp không múi giờ coi là UTC (`before_naive_timestamp` → 200). Chuỗi đã có `=` vẫn nhận (`before_padded` → 200). Chuỗi rỗng (`before_empty`), ký tự ngoài bảng base64 (`before_not_base64`), thiếu `|` (`before_no_separator`), uuid hỏng (`before_bad_uuid`), ngày hỏng (`before_bad_date`) → 422 `invalid_cursor`.
- Query lạ bị bỏ qua, kể cả `viewer_id` (`undeclared_query_ignored`).

## Đầu ra

- **200** `MemoryListResponse` (`services/api/app/api/schemas.py:1482-1486`), thứ tự khoá `context_id`, `memories`, `next_cursor`, `has_more`.
  - `context_id`: UUID path dạng chuẩn.
  - `memories[]`: `MemoryResponse` (`schemas.py:1449-1479`, `service.py:832-848`), `ORDER BY created_at DESC, id DESC`, lọc `(created_at, id) < before` bằng so sánh tuple (`services/api/app/api/repository.py:5167-5205`).
  - `reaction_count`, `comment_count`: hai truy vấn `GROUP BY` riêng trên trang; `viewer_has_reacted`: truy vấn thứ ba với `person_id = actor` (`repository.py:5264-5305`).
  - `has_more`: đọc `limit + 1` dòng (`repository.py:5188`).
  - `next_cursor`: `cursor` của dòng cuối trang khi trang **không rỗng**, kể cả trang cuối có `has_more: false`; `null` khi trang rỗng (`service.py:4865`; `owner_empty_wall`, `before_far_past`, `place_and_kind`).
  - `cursor`: base64url không padding của `created_at.isoformat()|id` (`cursors.py:15-19`).
- `lat`/`lng` float cho check-in, `null` cho ảnh; datetime pydantic UTC `Z`.
- Framework: 307 cho `/` cuối; `PUT` → 405 `allow: POST`.

## Tác dụng phụ

- Chỉ đọc: `memberships`, `memories` (index `ix_memories_context_feed`), `memory_reactions` ×2, `memory_comments`. Không khoá, không ghi, không idempotency.
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:4839-4843`, `:504` |
| 422 | `invalid_cursor` | `Memory cursor is invalid` | `service.py:4846-4847` |

422 framework: `uuid_parsing`, `greater_than_equal` (`ctx.ge`), `less_than_equal` (`ctx.le`), `int_parsing`, `literal_error`, `string_too_long`.

## Mã Python

- Route: `services/api/app/api/routes/memories.py:83-106`
- Service: `services/api/app/api/service.py:4833-4867` (`list_context_memories`), `:832-848`
- Codec: `services/api/app/api/cursors.py:15-46`
- Domain: `services/api/app/domain/permissions.py:427-430`
- Repository: `services/api/app/api/repository.py:5167-5205` (`list_memories`), `:5264-5305` (`_memory_social_counts`), `:2769-2782`
- Schema: `services/api/app/api/schemas.py:1440-1486`

## Test đang phủ

- `services/api/tests/postgres/test_group_memories_postgres.py`: `test_a_member_posts_a_memory_and_reads_it_back` (146), `test_a_stranger_can_neither_read_nor_post_group_memories` (185), `test_a_person_who_left_the_group_stops_seeing_its_memories` (222), `test_a_memory_belonging_to_another_group_never_appears_in_this_feed` (258), `test_the_wall_is_newest_first_and_pages_backwards` (332)
- `services/api/tests/postgres/test_group_checkins_postgres.py::test_photos_and_checkins_share_one_wall_and_can_be_narrowed` (336)
- `services/api/tests/postgres/test_memory_reactions_postgres.py`: `test_the_feed_carries_both_totals` (503), `test_hearts_and_comments_do_not_multiply_each_other` (519), `test_viewer_has_reacted_is_a_fact_about_the_reader` (545), `test_a_fresh_memory_reports_zero_and_not_null` (562)

## Kịch bản parity

`parity/scenarios/w3/memories/GET-contexts-context_id-memories.yaml`, id `w3/memories/get-contexts-context_id-memories` (66 bước), lane DB bật. Cursor cho `before` được dựng tay (base64url của chuỗi cố định), không bind từ câu trả lời:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `anonymous_bad_limit` (401 trước 422 query), `stranger_non_uuid_context`, `stranger_unknown_context_bad_limit` (422 trước 403), `stranger_unknown_context_bad_cursor` (403 trước 422 cursor), `stranger_non_uuid_and_bad_query`.
- 403: `stranger_unknown_context`, `stranger_real_context`, `stranger_claims_context_header`, `invitee_reads`, `leaver_reads_after_leaving`, `mate_roles_empty`.
- Đọc: `owner_empty_wall`, `owner_reads_wall`, `mate_reads_wall` (`viewer_has_reacted`), `mate_roles_group_admin_only`.
- `limit`: `limit_one`, `limit_hundred`, `limit_zero`, `limit_101`, `limit_text`, `limit_float`, `limit_repeated_last_wins`.
- `kind`/`place_id`: `kind_photo`, `kind_checkin`, `kind_video`, `kind_uppercase`, `kind_empty`, `place_filter`, `place_unknown`, `place_201_letters`, `place_and_kind`.
- `before`: `before_far_future`, `before_far_future_limit_one`, `before_far_past`, `before_naive_timestamp`, `before_padded`, `before_no_separator`, `before_bad_uuid`, `before_bad_date`, `before_not_base64`, `before_empty`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects`, `put_not_allowed`.
- Chuẩn bị: năm `register_*`, hai nhóm, mời/nhận, `mate_posts_first_photo`, `owner_checks_in`, `mate_posts_photo_at_place`, `owner_posts_second_photo`, `owner_posts_in_other_group`, `mate_hearts_second_photo`, `owner_comments_second_photo`, `leaver_leaves`.

`prod` (`w3/memories/prod-auth`): `junk_bearer_wall` (401), `stranger_reads_wall_claiming_owner` (403), `mate_reads_wall` (200), `owner_after_revoke` (401).

Chạy với làn DB bật; `cursor` và `next_cursor` trong câu trả lời được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` (ADR-0029 §2.4). Cursor dựng tay trong query là một phần của request, không phải giá trị server sinh. Corpus 422 sinh tự động hoãn: `query kind: no candidate value is accepted`.

## Chưa phủ / lưu ý cho bản Go

- `cursor` và `next_cursor` được bind theo cấu trúc (card `POST /contexts/{id}/memories`): `next_cursor` phải trỏ đúng dòng cuối trang. Phân trang bằng cursor lấy từ câu trả lời (bind rồi đưa vào `before`) chưa có bước nào dùng; các bước `before_*` dùng cursor dựng tay.
- `decode_cursor` chấp nhận timestamp bất kỳ `fromisoformat` đọc được (có hay không múi giờ, có phần lẻ hay không) và padding thừa; bản Go phải nhận đúng tập đó và từ chối đúng phần còn lại. `b64decode(validate=True)` từ chối ký tự ngoài bảng sau khi đổi `-_`.
- So sánh `(created_at, id) < (before_ts, before_id)` là so tuple của Postgres (`uuid` so theo byte).
- Hai dòng cùng `created_at` (không xảy ra với đồng hồ Python tuần tự) sẽ sắp theo `id DESC`.
