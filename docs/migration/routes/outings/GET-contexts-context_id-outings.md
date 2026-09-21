# GET /contexts/{context_id}/outings

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Mọi chuyến đi của một nhóm, kèm lịch trình của từng chuyến. Đây là đường đọc duy nhất cho một chuyến cụ thể: **không có `GET /outings/{outing_id}`**, nên client lấy một chuyến bằng cách đọc cả danh sách rồi lọc.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:96-106`, `services/api/app/api/service.py:2536-2550`):

1. Không có middleware idempotency (GET không nằm trong `WRITE_METHODS`).
2. Router: đuôi `/` → 307 (`trailing_slash`); `PATCH` → 405 (`patch_not_allowed`); `HEAD` được Starlette trả lời như GET không thân (`head_lists`).
3. `get_actor` → 401 trước 422 của path (`anonymous_non_uuid_context`); 422 `invalid_actor_id`, `invalid_actor_roles`.
4. Validation path `context_id`: UUID → 422 `uuid_parsing` (`owner_path_not_uuid`).
5. `_require_permission("view_outings", …, {"is_group_member": …})` (`service.py:2539-2543`; `services/api/app/domain/permissions.py:232-235`). Context **không được đọc**: id không tồn tại, nhóm của người khác, lời mời chưa nhận (`invited`), và người đã rời nhóm đều là 403 `is_group_member` (`owner_unknown_context`, `stranger_lists`, `invitee_lists_before_accepting`, `mate_lists_after_leaving`). Thiếu vai trò → 403 `role_not_permitted` (`owner_lists_as_guest_role`, `owner_lists_with_empty_roles`).

## Đầu vào

Path `context_id`: UUID. Không query, không thân.

## Đầu ra

**200** `OutingListResponse` (`services/api/app/api/schemas.py:635-637`): `{"context_id": …, "outings": [...]}`; `context_id` là **giá trị trong path**, không phải hàng đọc được — nên một nhóm rỗng và một id chưa tồn tại (nếu qua được quyền) nhìn giống nhau.

Mỗi phần tử là `OutingResponse` đúng như `POST` trả, gồm `stops` sắp theo `position` và `days` đọc từ `itinerary_days` (JSONB) được pydantic kiểm lại thành `ItineraryDay`. Thứ tự danh sách: `ORDER BY starts_on, id` (`repository.py:2872-2878`).

`at` của mỗi chặng là `_clock(minute_of_day)` — `f"{m//60:02d}:{m%60:02d}"`, không múi giờ (`service.py:421-424`). `meeting_point` có mặt khi và chỉ khi `meeting_lat` khác NULL (`service.py:886-895`).

## Tác dụng phụ

Không ghi gì. Mỗi chuyến tốn một truy vấn `outing_stops` riêng (`_outing_record`, `repository.py:2324-2331`): danh sách N chuyến là 1 + N truy vấn.

## Idempotency

Không áp dụng (GET). `tests/api/test_idempotency.py::test_read_requests_never_touch_the_store` (297) là ca gác chung.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","context_id"]` | `main.py:319-351` |
| 403 | `permission_denied` | `is_group_member` hoặc `role_not_permitted` | `service.py:499-504` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:96-106`
- Service: `services/api/app/api/service.py:2536-2550`, `:864-902`, `:421-424`
- Repository: `services/api/app/api/repository.py:2872-2878`, `:2324-2344`, `:2308-2322`
- Schema: `services/api/app/api/schemas.py:584-596`, `:619-637`
- Quyền: `services/api/app/domain/permissions.py:232-235`

## Test đang phủ

- `services/api/tests/postgres/test_outings_postgres.py`: `test_a_member_creates_an_outing_and_reads_it_back` (178), `test_a_stranger_can_neither_read_nor_create_outings` (353), `test_a_person_who_left_the_group_stops_seeing_its_outings` (383), `test_an_outing_of_another_group_never_appears_in_this_list` (409), `test_the_timeline_keeps_the_order_the_group_built_it_in` (442), `test_redeeming_an_invite_link_grants_an_invited_membership_not_an_active_one` (710), `test_a_stop_keeps_its_catalogue_place_and_a_reattach_keeps_its_checkins` (869)
- Không có ca `tests/api/` (fake repository).

## Kịch bản parity

`parity/scenarios/w7/outings/GET-contexts-context_id-outings.yaml`, id `w7/outings/get-contexts-context_id-outings` (39 bước, `dev`):

- Trước khi có ai: `anonymous_lists`, `anonymous_non_uuid_context`, `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid`, `owner_unknown_context`.
- Một 403 cho mọi lý do: `stranger_lists`, `invitee_lists_before_accepting`, `owner_lists_as_guest_role`, `owner_lists_with_empty_roles`, `owner_lists_other_group`, `mate_lists_after_leaving`.
- Nội dung: `owner_lists_empty`, ba chuyến nhập **lệch thứ tự** (`owner_creates_june_trip`, `owner_creates_march_trip`, `mate_creates_april_trip`) rồi `owner_lists_three` / `mate_lists_three` cho thấy `ORDER BY starts_on`; `owner_saves_timeline` → `owner_lists_with_stops`; `owner_checks_in` → `owner_lists_after_checkin` (chỉ `timeline_revision` đổi).
- Nhóm khác: `mate_creates_own_group`, `mate_creates_trip_in_own_group`, `mate_lists_own_group`, `mate_lists_shared_group_again`.
- Framework: `trailing_slash`, `patch_not_allowed`, `head_lists`.

Ba chuyến trong kịch bản có **ba `starts_on` khác nhau** là cố ý: khi `starts_on` bằng nhau, thứ tự rơi vào `id` ngẫu nhiên và hai stack sinh hai uuid khác nhau — một khác biệt giả.

`prod-auth.yaml`: `anonymous_lists`, `junk_bearer_lists`, `dev_headers_without_bearer`, `stranger_lists_claiming_context` (403 dù gửi `X-Actor-Contexts`), `owner_lists_trips`.

Corpus sinh: `generated/w7-422/get-contexts-context_id-outings.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- Không phủ danh sách dài (phân trang không tồn tại: route trả **mọi** chuyến của nhóm, không có `limit`).
- Không phủ hai chuyến cùng `starts_on` — xem lý do ở trên; bản Go phải giữ đúng `ORDER BY starts_on, id` để hoà vẫn quyết được.
- `context_id` trong thân lấy từ path: bản Go không được thay bằng giá trị đọc từ DB.

## Lỗi Python (chỉ báo, không sửa)

- N+1: mỗi chuyến một truy vấn `outing_stops` (`_outing_record`).
- Không có route đọc **một** chuyến, nên client phải tải cả nhóm để làm mới một chuyến sau mỗi lần lưu.
