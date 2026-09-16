# PUT /outings/{outing_id}/timeline

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Lưu kế hoạch **v1**: một danh sách chặng theo giờ đồng hồ, giữ đúng thứ tự người dùng xếp. Request **không mang id chặng**, nên danh tính chặng phải đọc từ *nội dung* chặng — bộ ba `(phút trong ngày, nhãn, tên địa điểm)`. Chặng nói y như cũ thì giữ nguyên hàng, và giữ luôn các lượt «đã tới» treo trên hàng đó; chặng gõ lại là chặng mới. Khi chuyến đã đi qua cửa v2 (`PUT …/itinerary`), cửa này đóng vĩnh viễn.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:109-120`, `services/api/app/api/service.py:3356-3397`):

1. Middleware idempotency: PUT là write method, nên khoá rỗng → 422 trước 401 (`anonymous_empty_idempotency_key`); route **không** khai header, nên không có khoá là bình thường.
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 trước 422 của path (`anonymous_non_uuid_outing`).
4. Validation path + thân `OutingTimelineRequest` cùng một lần (`owner_path_not_uuid_with_bad_body` trả cả hai mục).
5. `repository.get_outing(outing_id)` → 404 `outing_not_found` **`Outing does not exist`** (tiếng Anh; cửa v2 nói tiếng Việt cho cùng tình huống) (`service.py:3363-3364`; `owner_unknown_outing`).
6. `_require_permission("edit_outing_timeline", …, {"is_group_member": is_member(record.context_id, actor.id)})` (`service.py:3364-3368`; `permissions.py:236-239`) → 403 (`stranger_saves`, `owner_saves_as_guest_role`).
7. Vòng qua **mọi** chặng: `place_id` khác None mà `place_row` trả None → 422 `stop_place_unknown` `Chặng nêu một địa điểm không có trong danh mục.` (`service.py:3369-3377`; `owner_unknown_place`, `owner_unknown_place_on_second_stop` — chặng thứ hai cũng chặn, chưa ghi gì). Kiểm này **sau** 404 và 403 (`owner_unknown_outing_with_bad_place` vẫn là 404).
8. `repository.replace_outing_stops` (`repository.py:3106-3205`) dưới `SELECT … FOR UPDATE` hàng `outings`:
   - hàng biến mất → `OUTING_NOT_FOUND` → 404 `Không tìm thấy chuyến đi.` (không tới được tuần tự);
   - `expected_revision` khác `timeline_revision` → `TIMELINE_REVISION_CONFLICT` → 409 `timeline_conflict` **`Lịch trình đã được sửa. Tải bản mới trước khi lưu.`** (`owner_saves_stale_revision`);
   - `itinerary_version == 2` → `ITINERARY_UPGRADE_REQUIRED` → 409 `itinerary_upgrade_required` `Mở bản app mới để sửa lịch trình này.` (`owner_saves_timeline_after_upgrade`).

`expected_revision` **không bắt buộc**: bỏ trống thì không kiểm gì và bản lưu luôn thắng (`mate_drops_to_one_stop`, `owner_saves_empty_plan`).

## Đầu vào

- Path `outing_id`: UUID.
- Thân `OutingTimelineRequest` (`services/api/app/api/schemas.py:509-512`), `extra="forbid"`:
  - `expected_revision`: int strict ≥ 0 **hoặc null** (mặc định null);
  - `stops`: list `OutingStopInput` tối đa 50 (`schemas.py:482-507`): `at` khớp `^([01][0-9]|2[0-3]):[0-5][0-9]$` (`owner_clock_out_of_pattern`, `owner_clock_single_digit_hour`, `owner_clock_with_seconds`), `label` 1..200 đã cắt khoảng trắng và không rỗng (`owner_label_blank`), `place_name` ≤ 200 hoặc null (cắt khoảng trắng; chuỗi trắng thành **null**), `place_id` 1..80 hoặc null (`owner_place_id_empty`).

## Đầu ra

**200** `OutingResponse` — cùng hình dạng `POST /contexts/{id}/outings`, với `stops` đã sắp lại theo `position` và `timeline_revision` đã tăng 1.

## Tác dụng phụ

Một đơn vị công việc trên `outing_stops`, theo đúng trình tự SQLAlchemy đã ghim (`repository.py:3106-3205`):

1. `SELECT … FOR UPDATE` hàng `outings` (`populate_existing`).
2. `SELECT` mọi `outing_stops` của chuyến `ORDER BY position`.
3. Ghép danh tính: gom hàng cũ vào `unclaimed[(minute_of_day, label, place_name)]`, rồi với từng chặng của request lấy ra một hàng khớp (`kept`) hoặc xếp vào `added`.
4. `DELETE` mọi hàng không được nhận, rồi `flush`. **Xoá hàng chặng kéo theo các `outing_stop_checkins` của nó.**
5. Đưa mọi hàng sống sót lên vùng đỗ `parking = max(max(position cũ), len(stops)-1) + 1` rồi `flush` — bắt buộc, vì `uq_outing_stops_position` không deferrable và ORM ra một UPDATE mỗi hàng.
6. Hạ từng hàng về `position` mới và gán lại `place_id` (khoá danh mục là **thứ duy nhất** được đổi trên hàng giữ lại), rồi `flush`.
7. `INSERT` các chặng mới, `day = starts_on` nếu chuyến một ngày, ngược lại `NULL`; `flush`.
8. `outing.timeline_revision += 1`.

Thứ tự UPDATE là thứ tự **nạp trong session** (theo `position` cũ), cha (`outings`) trước con (`outing_stops`). Hình dạng INSERT đổi theo việc có phải đọc lại server default hay không: hàng `outing_stops` mới không gán `id` ở đây nên `id` sinh phía Python (`default=uuid.uuid4`), còn `time_locked`/`itinerary_version`-kiểu cột có default thì có RETURNING.

## Idempotency

Không tự nhiên idempotent khi không có `expected_revision` (mỗi lần gọi tăng revision, kể cả lưu y hệt — `owner_saves_the_same_plan_again`). Khoá header lưu 200 và phát lại **qua middleware** (route này không có nhánh `itinerary_authorized_replay`, nên phát lại **không** kiểm lại quyền): `owner_saves_with_key`, `owner_replays_key`, `owner_same_key_other_body`.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 422 | (validation) | `string_pattern_mismatch`, `string_too_short`, `value_error`, `int_type`… | `main.py:319-351` |
| 404 | `outing_not_found` | `Outing does not exist` | `service.py:3363-3364` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 422 | `stop_place_unknown` | `Chặng nêu một địa điểm không có trong danh mục.` | `service.py:3372-3377` |
| 409 | `timeline_conflict` | `Lịch trình đã được sửa. Tải bản mới trước khi lưu.` | `service.py:3402-3406` |
| 409 | `itinerary_upgrade_required` | `Mở bản app mới để sửa lịch trình này.` | `service.py:3407-3411` |
| 404 | `outing_not_found` | `Không tìm thấy chuyến đi.` (không đo được) | `service.py:3417` |
| 422 | `stop_not_found` | `Chặng không thuộc chuyến đi này.` (không tới được từ cửa này) | `service.py:3412-3416` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:109-120`
- Service: `services/api/app/api/service.py:3356-3397`, `:3400-3420` (`_itinerary_conflict`), `:414-424`
- Repository: `services/api/app/api/repository.py:3106-3205`
- Schema: `services/api/app/api/schemas.py:482-512`
- Quyền: `services/api/app/domain/permissions.py:236-239`
- Model DB: `services/api/app/db/models.py:1273-1324` (`uq_outing_stops_position`, `minute_in_day`, `position_not_negative`, `label_not_blank`, `meeting_point_valid`)

## Test đang phủ

- `services/api/tests/postgres/test_outings_postgres.py`: `test_the_timeline_keeps_the_order_the_group_built_it_in` (442), `test_replacing_the_timeline_leaves_no_stops_from_the_previous_plan` (486), `test_a_malformed_clock_time_is_refused` (533), `test_a_stranger_cannot_rewrite_a_groups_timeline` (572), `test_a_stop_keeps_its_catalogue_place_and_a_reattach_keeps_its_checkins` (869)
- `services/api/tests/postgres/test_stop_checkins_postgres.py`: `test_adding_a_stop_keeps_the_checkins_of_the_stops_that_did_not_change` (434), `test_reordering_the_timeline_carries_each_checkin_with_its_stop` (462), `test_a_stop_dropped_from_the_new_plan_takes_only_its_own_checkins` (485), `test_editing_a_stop_drops_the_checkins_of_the_stop_it_replaced` (507), `test_a_checkin_from_another_group_never_appears` (306)
- `services/api/tests/postgres/test_itinerary_postgres.py`: `test_save_reorder_identity_undo_conflict_and_real_replay` (94) — nhánh 409 `itinerary_upgrade_required`

## Kịch bản parity

`parity/scenarios/w7/outings/PUT-outings-outing_id-timeline.yaml`, id `w7/outings/put-outings-outing_id-timeline` (57 bước, `dev`):

- Cửa: `anonymous_saves`, `anonymous_non_uuid_outing`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `owner_path_not_uuid_with_bad_body`, `owner_unknown_outing`, `owner_unknown_outing_with_bad_place`, `stranger_saves`, `owner_saves_as_guest_role`, `owner_unknown_place`, `owner_unknown_place_on_second_stop`.
- Thân: `owner_body_missing`, `owner_stops_missing`, `owner_stops_not_a_list`, `owner_clock_out_of_pattern`, `owner_clock_single_digit_hour`, `owner_clock_with_seconds`, `owner_label_blank`, `owner_place_id_empty`, `owner_expected_revision_negative`, `owner_expected_revision_string`, `owner_extra_field`.
- Bốn hình dạng của đơn vị công việc: `owner_reorders_keeping_every_stop` (hoán vị giữ đủ ba chặng), `owner_saves_the_same_plan_again` (không đổi gì), `owner_adds_a_stop` (thêm), `mate_drops_to_one_stop` và `owner_saves_empty_plan` (bớt).
- Danh tính chặng: `owner_checks_in_at_coffee` + `mate_checks_in_at_coffee` rồi `owner_reads_checkins_after_reorder` (giữ), `owner_attaches_a_place_to_a_kept_stop` + `owner_reads_checkins_after_attaching_place` (gắn địa điểm không làm mất «đã tới»), `owner_retypes_the_coffee_stop` + `owner_reads_checkins_after_retyping` (gõ lại là chặng khác, «đã tới» đi theo hàng cũ), `owner_checks_in_at_dropped_stop`.
- Revision: `owner_saves_stale_revision` (409), một chuyến một ngày `owner_saves_one_day_plan` (chặng mới được đóng dấu `day`).
- Khoá: `owner_saves_with_key`, `owner_replays_key`, `owner_same_key_other_body`.
- v2: `owner_upgrades_to_itinerary`, rồi `owner_saves_timeline_after_upgrade` (không nêu revision → 409 `itinerary_upgrade_required`) và `owner_saves_timeline_after_upgrade_with_revision` (nêu revision sai → 409 `timeline_conflict`): trong `replace_outing_stops` kiểm revision đứng **trước** kiểm `itinerary_version == 2`.
- Framework: `trailing_slash`, `get_not_allowed`.

Corpus sinh: route này **hoãn** (`OutingStopInput.at` mang `pattern`).

## Chưa phủ / lưu ý cho bản Go

- 404 `Không tìm thấy chuyến đi.` và 422 `stop_not_found` của `_itinerary_conflict` **không tới được** từ cửa này (chuyến đã được đọc ngay trước đó; `replace_outing_stops` không bao giờ ném `STOP_NOT_FOUND`). Bản Go vẫn phải giữ bảng ánh xạ, nhưng không có kịch bản nào chứng minh hai nhánh đó.
- Không phủ 51 chặng (chạm `max_length`), cũng không phủ hai chặng **giống hệt nhau** trong một request (khi đó `unclaimed` phát một hàng cho mỗi bản sao — hành vi có thật, chưa có bước).
- Thứ tự UPDATE trong bước «đỗ rồi hạ» là hợp đồng với oracle repository, không phải với wire; làn database chỉ thấy trạng thái sau bước.
- `place_name` chuỗi trắng bị biến thành `null` **trước** khi tham gia khoá danh tính — nên `"  "` và `null` là cùng một chặng.

## Lỗi Python (chỉ báo, không sửa)

- Cùng một tình huống «không có chuyến này» trả **`Outing does not exist`** ở cửa v1 và **`Không tìm thấy chuyến đi.`** ở cửa v2; và cùng một xung đột revision trả `…trước khi lưu.` ở v1, `…trước khi tiếp tục.` ở v2.
- N+1: `place_row` được gọi một lần cho mỗi chặng có `place_id`.
- `_minute_of_day` (`service.py:414-419`) không bắt lỗi: nó chỉ chạy được vì `pattern` của trường đã chặn trước. Nới `pattern` ra là route trả 500.
