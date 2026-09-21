# GET /contexts/{context_id}/widget

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

F38: bức ảnh mới nhất của một nhóm cho widget màn hình chính, hoặc `null`. Là tường kỷ niệm với `limit=1, kind="photo"` qua đúng repository và đúng quyền của tường (`services/api/app/api/service.py:4869-4934`). Check-in không bao giờ làm widget trống.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_context`); `X-Actor-ID` không phải UUID → 422 (`actor_id_not_uuid`).
2. Path `context_id: UUID` → 422 (`stranger_non_uuid_context`).
3. `_require_permission("view_group_memories", {"is_group_member": …})` (`service.py:4900-4904`; `services/api/app/domain/permissions.py:427-430`): cùng một 403 `is_group_member` cho nhóm không tồn tại (`stranger_unknown_context`), nhóm trống (`stranger_empty_group_widget`), nhóm có ảnh (`stranger_real_context`); người được mời (`invitee_widget`), người đã rời (`leaver_widget_after_leaving`), header tự khai (`stranger_claims_context_header`) → 403. Role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200.

## Đầu vào

- Path `context_id` (UUID lax).
- Không query, không body (`undeclared_query_ignored` với `kind=checkin` vẫn trả ảnh).

## Đầu ra

- **200** `WidgetResponse` (`services/api/app/api/schemas.py:1513-1529`), thứ tự khoá `context_id`, `photo`.
  - Không có ảnh (nhóm trống hoặc chỉ có check-in) → `photo: null` (`owner_empty_group_widget`, `owner_widget_only_checkins`).
  - Có ảnh → `WidgetPhotoResponse` (`schemas.py:1489-1510`), thứ tự khoá `memory_id`, `image_url`, `caption`, `author_id`, `author_name`, `created_at`. Không `cursor`, không bộ đếm, không toạ độ.
  - Ảnh mới nhất theo `created_at DESC, id DESC`; check-in mới hơn không thay nó (`mate_widget_after_newer_checkin`); ảnh mới hơn thì thay (`mate_widget_newest_photo`).
  - `author_name` đọc từ `people` lúc gọi (`owner_widget_reads_name_now`); tác giả đã rời vẫn giữ tên (`mate_widget_first_photo` là ảnh của người sau đó rời); tác giả đã xoá tài khoản → `"Người dùng đã rời"` (`ANONYMOUS_DISPLAY_NAME`, `services/api/app/domain/account_lifecycle.py:55`; `owner_widget_after_author_deleted`). Fallback `"Thành viên nhóm"` (`service.py:829`, `:4932`) không tới được (FK).
  - `created_at`: đồng hồ Python lúc đăng; pydantic UTC `Z`.
- Framework: 307 cho `/` cuối; `POST` → 405 `allow: GET`.

## Tác dụng phụ

- Chỉ đọc: `memberships`, `memories` (`list_memories(limit=1, kind="photo")`, `services/api/app/api/repository.py:5167-5205`), hai truy vấn đếm (`repository.py:5264-5305`, không có `viewer_id` nên không có truy vấn thứ ba; kết quả bị bỏ), `people`. Không ghi, không idempotency, không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:4900-4904`, `:504` |

## Mã Python

- Route: `services/api/app/api/routes/memories.py:109-133`
- Service: `services/api/app/api/service.py:4869-4934` (`read_context_widget`), `:829` (`_SOMEBODY`)
- Domain: `services/api/app/domain/permissions.py:427-430`; `services/api/app/domain/account_lifecycle.py:55`
- Repository: `services/api/app/api/repository.py:5167-5205`, `:5264-5305`, `:2525-2527`

## Test đang phủ

- `services/api/tests/api/test_widget_leak.py`: `test_a_member_sees_the_groups_newest_photograph` (135), `test_a_second_member_sees_a_photograph_they_did_not_take` (153), `test_the_widget_shows_the_newest_of_several` (167), `test_a_newer_checkin_does_not_blank_the_widget` (186), `test_a_stranger_gets_no_records_and_no_photo_url` (209), `test_a_stranger_claiming_every_role_still_gets_no_records` (230), `test_a_guest_gets_no_records` (251), `test_a_member_of_one_group_gets_no_records_from_another` (269), `test_a_stranger_cannot_tell_an_empty_group_from_a_full_one` (288), `test_a_group_with_no_photographs_answers_two_hundred_and_null` (313), `test_a_group_holding_only_checkins_answers_null` (323), `test_the_empty_body_carries_nothing_about_the_group` (335), `test_the_widget_route_takes_no_identity_field` (369)
- `services/api/tests/postgres/test_widget_privacy_postgres.py`: 206, 229, 261, `test_the_author_keeps_their_name_after_they_leave` (279), 314, 334, 358, 382, 402, `test_the_refusal_is_the_same_whether_the_group_exists` (438), 467, 488

## Kịch bản parity

`parity/scenarios/w3/memories/GET-contexts-context_id-widget.yaml`, id `w3/memories/get-contexts-context_id-widget` (42 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `actor_id_not_uuid`, `stranger_non_uuid_context`.
- 403: `stranger_unknown_context`, `stranger_empty_group_widget`, `stranger_real_context`, `stranger_claims_context_header`, `invitee_widget`, `mate_roles_empty`, `leaver_widget_after_leaving`.
- `null`: `owner_empty_group_widget`, `owner_widget_only_checkins`.
- Ảnh: `mate_widget_first_photo`, `mate_widget_after_newer_checkin`, `mate_widget_newest_photo`, `mate_roles_group_admin_only`, `owner_widget_reads_name_now`, `owner_widget_after_author_deleted`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects`, `post_not_allowed`.
- Chuẩn bị: năm `register_*`, `owner_creates_group`, `owner_creates_empty_group`, mời/nhận, `mate_checks_in`, `leaver_posts_photo`, `owner_checks_in_later`, `owner_posts_photo_no_caption`, `leaver_leaves`, `mate_posts_newest_photo`, `mate_renames_self`, `mate_deletes_account`.

`prod` (`w3/memories/prod-auth`): `basic_scheme_widget` (401), `owner_widget` (200), `mate_after_deletion` (401).

Chạy với làn DB bật; `cursor` của các bước chuẩn bị đăng ảnh/check-in được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` như ở card `POST /contexts/{id}/memories` (ADR-0029 §2.4). Corpus 422 sinh tự động: `parity/scenarios/generated/w3-422/get-contexts-context_id-widget.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- Câu trả lời widget không có `cursor`; cursor chỉ xuất hiện ở các bước chuẩn bị.
- Bản Go phải lọc `kind = photo` trong truy vấn, không lấy dòng mới nhất rồi bỏ nếu là check-in (nếu không, check-in mới nhất sẽ cho `null`).
- Truy vấn đếm thừa của `list_memories` không thấy được qua HTTP.
