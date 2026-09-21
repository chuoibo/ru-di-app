# GET /outings/{outing_id}/checkins

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Ai đã tới đâu trong một chuyến, cũ nhất trước. Một danh sách phẳng cho **cả** chuyến, không nhóm theo chặng: client tự gom theo `stop_id`.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:143-153`, `services/api/app/api/service.py:3557-3574`):

1. Không có middleware idempotency (GET).
2. Router: đuôi `/` → 307; `DELETE` → 405.
3. `get_actor` → 401 trước 422 của path (`anonymous_non_uuid_outing`).
4. Validation path `outing_id`.
5. `repository.get_outing(outing_id)` → None → 404 `outing_not_found` **`Outing does not exist`** (`service.py:3561-3562`; `owner_unknown_outing`).
6. `_require_permission("view_stop_checkins", …, {"is_group_member": …})` (`service.py:3563-3567`; `permissions.py:252-255`) → 403: người lạ, người mới nhận link (`invited`), người đã rời nhóm, và người thiếu vai trò (`stranger_reads`, `invitee_reads`, `mate_reads_after_leaving`, `owner_reads_as_guest_role`, `owner_reads_with_empty_roles`, `owner_reads_the_other_groups_list`).

Đây là thứ tự **404 trước 403**: một người lạ vẫn biết được chuyến có tồn tại hay không.

## Đầu vào

Path `outing_id`: UUID. Không query, không thân.

## Đầu ra

**200** `OutingCheckinListResponse` (`schemas.py:614-616`): `{"outing_id": <từ path>, "checkins": [...]}`. Mỗi phần tử là `StopCheckinResponse` như route ghi trả về.

Thứ tự: `JOIN outing_stops ON id = stop_id WHERE outing_stops.outing_id = … ORDER BY created_at, id` (`repository.py:3344-3353`) — **mọi chặng của chuyến gộp làm một**, không phải theo `position`.

`display_name` là tên **hiện tại** của người đó, đọc lại mỗi lần (`mate_renames_self` → `owner_reads_after_rename`).

## Tác dụng phụ

Không ghi gì. Một truy vấn cho danh sách, cộng **một `get_person` cho mỗi hàng** (`service.py:3576-3584`).

## Idempotency

Không áp dụng (GET).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","outing_id"]` | `main.py:319-351` |
| 404 | `outing_not_found` | `Outing does not exist` | `service.py:3561-3562` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:143-153`
- Service: `services/api/app/api/service.py:3557-3574`, `:3576-3584`
- Repository: `services/api/app/api/repository.py:3344-3353`
- Schema: `services/api/app/api/schemas.py:597-616`
- Quyền: `services/api/app/domain/permissions.py:252-255`

## Test đang phủ

- `services/api/tests/postgres/test_stop_checkins_postgres.py` (helper `_read_checkins` 89): `test_the_same_person_cannot_check_in_twice` (147), `test_one_person_may_check_in_to_each_of_several_stops` (195), `test_two_members_may_check_in_to_the_same_stop` (209), `test_an_outsider_cannot_read_who_arrived` (238), `test_an_invited_member_cannot_read_who_arrived` (269), `test_a_checkin_from_another_group_never_appears` (306), `test_no_response_field_carries_a_location` (362), `test_a_body_offering_coordinates_changes_nothing` (393), `test_adding_a_stop_keeps_the_checkins_of_the_stops_that_did_not_change` (434), `test_reordering_the_timeline_carries_each_checkin_with_its_stop` (462), `test_a_stop_dropped_from_the_new_plan_takes_only_its_own_checkins` (485), `test_editing_a_stop_drops_the_checkins_of_the_stop_it_replaced` (507)
- `services/api/tests/postgres/test_itinerary_postgres.py`: `test_save_reorder_identity_undo_conflict_and_real_replay` (94)

## Kịch bản parity

`parity/scenarios/w7/outings/GET-outings-outing_id-checkins.yaml`, id `w7/outings/get-outings-outing_id-checkins` (44 bước, `dev`):

- Cửa: `anonymous_reads`, `anonymous_non_uuid_outing`, `actor_id_not_uuid`, `owner_path_not_uuid`, `owner_unknown_outing`, `stranger_reads`, `invitee_reads`, `owner_reads_as_guest_role`, `owner_reads_with_empty_roles`.
- Nội dung: `owner_reads_empty_list`, `owner_reads_after_timeline`, bốn lượt tới xen kẽ hai người hai chặng rồi `owner_reads_four_rows` / `mate_reads_four_rows` (cùng một thân cho hai người đọc).
- Tên đọc lại: `mate_renames_self`, `owner_reads_after_rename`.
- Chặng bị bỏ: `owner_drops_the_coffee_stop`, `owner_reads_after_the_drop`.
- Nhóm khác: `stranger_creates_own_group` … `owner_reads_the_other_groups_list` (403), `stranger_reads_own_list`.
- Rời nhóm: `mate_leaves_group`, `mate_reads_after_leaving`, `owner_reads_after_mate_left` (hàng của người đã rời **vẫn còn** và vẫn mang tên họ).
- Framework: `trailing_slash`, `delete_not_allowed`.

`prod-auth.yaml`: `basic_scheme_reads_checkins`, `mate_reads_checkins`, `stranger_reads_checkins`, `stranger_reads_checkins_after_redeeming` (403 — link mới chỉ cho `invited`).

Corpus sinh: `generated/w7-422/get-outings-outing_id-checkins.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- `display_name: null` (hàng người không còn) **không được phủ**: đường duy nhất tới đó là xoá tài khoản, mà `DELETE /people/me` kéo theo cả membership và có thể cả hàng «đã tới» — dựng nó ở đây sẽ đo một route của sóng khác chứ không phải route này.
- Không phủ danh sách dài: không có `limit`, không có phân trang.
- Thứ tự là `(created_at, id)` trên **toàn chuyến**, không phải theo `position` của chặng; bản Go phải giữ đúng thế.

## Lỗi Python (chỉ báo, không sửa)

- 404 đứng **trước** 403: một người ngoài nhóm phân biệt được «chuyến này có thật» với «không có chuyến nào như thế». Các route lời mời của cùng sóng này làm ngược lại (gộp mọi thứ vào một 404).
- N+1: một `get_person` cho mỗi lượt «đã tới».
