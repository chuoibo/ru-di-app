# PUT /outings/{outing_id}/itinerary

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Lưu kế hoạch **v2**: chặng có id, có ngày, có thời lượng, có điểm hẹn toạ độ; ngày có phương tiện, giờ xuất phát và điểm đầu/cuối. Khác cửa v1 ở chỗ request **mang id chặng**, nên danh tính do client quyết: id thật là «giữ hàng này», tiền tố `tmp-` là «tạo hàng mới», id vắng mặt là «xoá». Lưu xong `itinerary_version` thành 2 và cửa v1 đóng lại vĩnh viễn.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:56-78`, `services/api/app/api/service.py:3506-3525` → `:3435-3494`):

1. Middleware idempotency: khoá rỗng → 422 `invalid_idempotency_key` trước mọi thứ (`anonymous_empty_key`).
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 (`anonymous_saves`, `anonymous_without_key`).
4. Validation: header `idempotency_key` được **khai là tham số bắt buộc** `Header(min_length=1, max_length=255)` (`routes/outings.py:64`), nên **thiếu** header là 422 của FastAPI với `loc` `["header","idempotency-key"]`, còn **rỗng** là 422 của middleware — hai câu trả lời khác nhau cho hai tình huống gần giống nhau (`anonymous_without_key` vs `anonymous_empty_key`). Cùng lượt này validate path và thân `ItineraryRequest`.
5. Nhánh phát lại: nếu middleware đã đặt `scope["itinerary_authorized_replay"]` thì route gọi `authorize_outing_itinerary` **rồi mới** trả thân đã lưu (`routes/outings.py:69-77`). Thân v2 chứa điểm hẹn riêng tư, nên một bản phát lại vẫn phải chứng minh phiên và tư cách thành viên **lúc này**: người đã rời nhóm nhận 403 thay cho 200 đã lưu (`mate_replays_key_after_leaving`).
6. `authorize_outing_itinerary` (`service.py:3422-3433`): `get_outing` → 404 `outing_not_found` **`Không tìm thấy chuyến đi.`** (tiếng Việt; cửa v1 nói tiếng Anh) → `_require_permission("edit_outing_timeline", …)` → 403.
7. `_itinerary_draft` (`service.py:3435-3494`), đúng thứ tự này:
   - `request.expected_revision != record.timeline_revision` → 409 `timeline_conflict` **`Lịch trình đã được sửa. Tải bản mới trước khi tiếp tục.`**;
   - mỗi `days[i].day` phải nằm trong `[starts_on, ends_on]` → 422 `itinerary_day_invalid` `Ngày nằm ngoài chuyến đi.`;
   - `start_stop_id` / `end_stop_id` (nếu **truthy**) phải nằm trong tập id của chặng thuộc đúng ngày đó → 422 `itinerary_anchor_invalid` `Điểm đầu/cuối phải thuộc ngày này.`;
   - `list_outing_checkins(outing_id)` được đọc để đánh dấu `checked_in`;
   - mỗi chặng: id không bắt đầu `tmp-` và không thuộc `record.stops` → 422 `stop_not_found` `Chặng không thuộc chuyến đi này.`; `stop.day` khác None mà ngoài chuyến **hoặc** không có trong `days` → 422 `itinerary_day_invalid` `Chọn cấu hình cho ngày của chặng.`; `place_id` không có trong danh mục → 422 `stop_place_unknown` `Địa điểm không còn trong danh mục.`
8. `repository.replace_outing_itinerary` (`repository.py:3207-3284`) dưới `FOR UPDATE`: `OUTING_NOT_FOUND` → 404, `TIMELINE_REVISION_CONFLICT` → 409, `STOP_NOT_FOUND` → 422 (hai nhánh sau chỉ tới được khi có đua).

## Đầu vào

- Path `outing_id`: UUID. Header `idempotency-key` **bắt buộc**, 1..255.
- Thân `ItineraryRequest` (`services/api/app/api/schemas.py:565-577`), `extra="forbid"`:
  - `expected_revision`: int strict ≥ 0, **bắt buộc**;
  - `stops`: ≤ 50 `ItineraryStopInput` (`schemas.py:532-554`) = `OutingStopInput` + `id` 1..80 (UUID chuẩn hoá, hoặc `tmp-` + ít nhất một ký tự — `"tmp-"` trần bị từ chối), `day` `date|null`, `duration_minutes` int 0..1440 hoặc null, `time_locked` bool strict mặc định `true`, `meeting_point` `{lat, lng, label}` hoặc null; `model_validator`: `place_id` và `meeting_point` không được cùng có;
  - `days`: ≤ 50 `ItineraryDay` (`schemas.py:556-563`) = `day` date, `transport_mode` ∈ {`motorbike`,`car`,`walk`}, `start_at` khớp `ClockTime`, `start_stop_id`/`end_stop_id` ≤ 80 hoặc null (**không có `min_length`**), `return_to_start` bool mặc định false;
  - `model_validator` của request: id chặng phải duy nhất, `day` phải duy nhất.

## Đầu ra

**200** `OutingResponse`, với `itinerary_version` = 2, `timeline_revision` đã tăng 1, `stops` theo `position` mới và `days` là `itinerary_days` đã lưu (đã thay id `tmp-` ở hai neo bằng id thật). Header `Cache-Control: no-store` trên **mọi** câu trả lời của route (`routes/outings.py:67`), và bản phát lại thêm `Idempotency-Replayed: true`.

## Tác dụng phụ

Một đơn vị công việc (`repository.py:3207-3284`), trình tự:

1. `SELECT … FOR UPDATE` `outings`.
2. `SELECT` mọi `outing_stops` của chuyến (không `ORDER BY`), đưa vào `existing` theo id.
3. `requested_ids` = id không phải `tmp-`; phải là tập con của `existing`, nếu không `STOP_NOT_FOUND`.
4. `DELETE` mọi hàng không được nêu, rồi `flush` — **kéo theo `outing_stop_checkins` của chúng**.
5. Đưa mọi hàng sống sót lên vùng đỗ `parking = max(position cũ ∪ {len(stops)}) + 1` rồi `flush` (vòng này duyệt một **set**, nên thứ tự đỗ không xác định; mọi hàng đều được gán lại `position` ngay sau đó).
6. Với từng chặng theo thứ tự request: hàng cũ được cập nhật tại chỗ, `tmp-` sinh hàng mới với `id=uuid.uuid4()` gán **phía Python** (nên INSERT không cần RETURNING cho `id`) và được ghi vào `new_ids`; mọi cột nội dung được gán lại, kể cả về `None`.
7. `outing.itinerary_days` = `days` đã thay `start_stop_id`/`end_stop_id` qua `new_ids`; `itinerary_version = 2`; `timeline_revision += 1`; `flush`.

UPDATE ra theo thứ tự nạp trong session, cha trước con. Ràng buộc `meeting_point_valid` đòi ba cột điểm hẹn cùng có hoặc cùng không, và `place_id IS NULL` khi có điểm hẹn — cùng điều mà `model_validator` đã chặn ở tầng thân.

## Idempotency

Khoá **bắt buộc**. Bản phát lại không do middleware trả thẳng: middleware đặt `scope["itinerary_authorized_replay"]` và cho request đi tiếp, route xác thực lại rồi trả thân đã lưu (`idempotency.py:495-499`). Hệ quả đo được: phát lại sau khi rời nhóm là 403 (`mate_replays_key_after_leaving`), phát lại bởi người khác là một khoá **mới** trong phạm vi của họ (`stranger_replays_mates_key` → 403 vì không phải thành viên, `owner_same_key_own_scope` → 200 thật).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | (validation) | thiếu header → `loc` `["header","idempotency-key"]` | `main.py:319-351` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:474-480` |
| 404 | `outing_not_found` | `Không tìm thấy chuyến đi.` | `service.py:3426-3427` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 409 | `timeline_conflict` | `Lịch trình đã được sửa. Tải bản mới trước khi tiếp tục.` | `service.py:3440-3445` |
| 422 | `itinerary_day_invalid` | `Ngày nằm ngoài chuyến đi.` | `service.py:3450-3452` |
| 422 | `itinerary_anchor_invalid` | `Điểm đầu/cuối phải thuộc ngày này.` | `service.py:3458-3462` |
| 422 | `stop_not_found` | `Chặng không thuộc chuyến đi này.` | `service.py:3469-3471` |
| 422 | `itinerary_day_invalid` | `Chọn cấu hình cho ngày của chặng.` | `service.py:3476-3478` |
| 422 | `stop_place_unknown` | `Địa điểm không còn trong danh mục.` | `service.py:3481-3483` |
| 409 | `timeline_conflict` | `Lịch trình đã được sửa. Tải bản mới trước khi lưu.` (từ repository, chỉ khi đua) | `service.py:3402-3406` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:56-78`
- Service: `services/api/app/api/service.py:3506-3525`, `:3435-3494`, `:3422-3433`, `:3400-3420`
- Repository: `services/api/app/api/repository.py:3207-3284`
- Middleware: `services/api/app/api/idempotency.py:404-499`
- Schema: `services/api/app/api/schemas.py:517-577`
- Model DB: `services/api/app/db/models.py:1273-1324`

## Test đang phủ

- `services/api/tests/postgres/test_itinerary_postgres.py`: `test_save_reorder_identity_undo_conflict_and_real_replay` (94), `test_saved_pin_response_cannot_replay_after_leaving_group` (188), `test_invalid_drafts_cannot_write` (227, 7 tham số: `foreign_id`, `duplicate_id`, `unknown_place`, `outside_day`, `both_locations`, `nan_pin`, `foreign_anchor`)
- `services/api/tests/postgres/test_itinerary_migration_race.py`: `test_two_connections_cannot_overwrite_same_revision` (120) — ở tầng repository
- Meta: `tests/api/test_idempotency.py::test_every_write_route_consults_the_store` (257)

## Kịch bản parity

`parity/scenarios/w7/outings/PUT-outings-outing_id-itinerary.yaml`, id `w7/outings/put-outings-outing_id-itinerary` (70 bước, `dev`):

- Header: `anonymous_saves`, `anonymous_without_key`, `anonymous_empty_key`.
- Cửa: `anonymous_non_uuid_outing`, `actor_id_not_uuid`, `owner_unknown_outing`, `stranger_saves`, `owner_saves_as_guest_role`.
- Thân: `owner_body_missing`, `owner_revision_missing`, `owner_revision_string`, `owner_revision_negative`, `owner_duplicate_stop_ids`, `owner_duplicate_days`, `owner_stop_id_not_uuid_nor_tmp`, `owner_stop_id_bare_tmp_prefix`, `owner_place_and_meeting_point_together`, `owner_meeting_point_out_of_range`, `owner_meeting_point_label_blank`, `owner_meeting_point_lat_as_string`, `owner_duration_over_a_day`, `owner_duration_float`, `owner_time_locked_string`, `owner_transport_mode_unknown`, `owner_start_at_out_of_pattern`, `owner_day_not_a_real_date`, `owner_extra_field`.
- Thứ tự của `_itinerary_draft`: `owner_stale_revision`, `owner_day_outside_the_trip`, `owner_anchor_outside_its_day`, `owner_end_anchor_outside_its_day`, `owner_stop_id_of_another_trip`, `owner_stop_day_not_configured`, `owner_stop_day_outside_the_trip`, `owner_place_not_in_catalogue`.
- **Bốn hình dạng của đơn vị công việc**: `owner_permutes_keeping_every_stop` (giữ đủ ba, đảo thứ tự, neo đổi theo), `owner_drops_two_stops` (bớt, «đã tới» của hàng bị bỏ đi theo), `owner_adds_a_stop` (thêm một `tmp-` giữa hai hàng cũ), `owner_saves_the_same_plan_again` (không đổi gì ngoài revision).
- Nhận diện: `owner_saves_first_itinerary` (ba `tmp-` thành id thật, neo được viết lại), `owner_checks_in_at_cafe` + `owner_reads_checkins_after_permutation` / `_after_adding` / `_after_dropping`, `owner_checks_in_at_dropped_stop` (404), `owner_names_a_dropped_stop_again` (422 `stop_not_found`).
- Một lượt «đã tới» làm bản nháp đang mở thành cũ: `owner_saves_with_the_revision_from_before_the_arrival` (409).
- Biên: `owner_saves_with_empty_anchors` (neo `""` lặng lẽ thành «không neo»), `owner_saves_a_stop_without_a_day`, `owner_saves_an_empty_plan`.
- Phát lại: `mate_saves_with_key`, `mate_replays_key`, `mate_same_key_other_body`, `owner_same_key_own_scope`, `mate_leaves_group`, `mate_replays_key_after_leaving` (403), `stranger_replays_mates_key`.
- Framework: `trailing_slash`, `get_not_allowed`.

`crossreplay/PUT-outings-outing_id-itinerary.yaml` (16 bước): 200 lưu ở cửa sau phát lại ở cửa trước và ngược lại; 409 revision cũ nhả khoá; sau khi rời nhóm cả hai cửa đều từ chối bản phát lại.

`concurrency/PUT-outings-outing_id-itinerary.yaml` (10 bước): ba lần lưu cùng một revision, ba khoá khác nhau → một 200 hai 409; ba lần dưới **một** khoá → một lần lưu hai lần phát lại.

`prod-auth.yaml`: `anonymous_saves_itinerary_without_key`, `owner_saves_itinerary_with_key`, `owner_replays_itinerary_key`, `mate_same_itinerary_key_own_session`.

Corpus sinh: route này **hoãn** (khai header `idempotency-key`; và thân có `model_validator(mode="after")` phía sau).

## Chưa phủ / lưu ý cho bản Go

- Không phủ 51 chặng / 51 ngày (`max_length`), `duration_minutes` = 0 và = 1440 (đúng biên), `lat` = ±90 / `lng` = ±180 (đúng biên).
- Hai nhánh của `_itinerary_conflict` chỉ tới được khi có đua: 404 `Không tìm thấy chuyến đi.` từ repository, và 422 `stop_not_found` từ repository. Bước `concurrent` chỉ chứng minh nhánh 409.
- Không phủ hai người lưu **cùng lúc trên hai chuyến khác nhau** (mỗi bước đồng thời là một persona, một đường dẫn).
- `_itinerary_draft` được dùng chung với route xem trước; đổi nó là đổi cả hai cửa.
- Thân của bước 6 gán **mọi** cột, kể cả về `None`: một chặng giữ lại mà request bỏ `duration_minutes` thì cột đó thành NULL, không phải giữ giá trị cũ.

## Lỗi Python (chỉ báo, không sửa)

- `start_stop_id` / `end_stop_id` có `max_length` mà **không có `min_length`**, và cả hai chỗ kiểm chỉ xét tính chân trị (`if anchor and …`, `if settings.get(key) and …`). Client gửi `""` thì neo không được kiểm, và `""` được lưu thẳng vào `itinerary_days`.
- Thiếu header `idempotency-key` là 422 hình dạng «trường thiếu», không phải `invalid_idempotency_key`: hai đường vào cùng một ý nghĩa, hai hình dạng lỗi.
- `_wire_outing` 500 được: một hàng chặng có `meeting_lat` mà thiếu `meeting_lng` làm `MeetingPoint` ném `ValidationError` thay vì render chặng không kèm điểm hẹn. Ràng buộc DB chặn được hàng như thế hôm nay, nên đây là nợ tiềm ẩn.
- Vòng «đỗ» duyệt một `set` các UUID, nên thứ tự UPDATE trung gian không xác định giữa hai tiến trình; chỉ trạng thái cuối bước là xác định.
