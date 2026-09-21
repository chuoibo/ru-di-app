# POST /outings/{outing_id}/itinerary/preview

outings · core · trạng thái trong bộ nhớ: **`itinerary_limiter`** (`app/api/main.py:151-156`)

## Mục đích

Tính thử một ngày của bản nháp mà **không lưu gì**: giờ đến từng chặng, quãng đường, và — khi có máy định tuyến — một thứ tự đi khác để so. Trả về `status` ∈ {`incomplete`, `unavailable`, `ready`} cùng một danh sách `issues` nói rõ còn thiếu gì.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:42-53`, `services/api/app/api/service.py:3496-3504`):

1. Middleware idempotency **nhường đường trước khi đọc header**: `POST /outings/*/itinerary/preview` được cho đi thẳng (`idempotency.py:418-424`). Hệ quả đo được: `idempotency-key: ''` **không** là 422 ở route này (`owner_previews_with_empty_key`), và cùng một khoá gửi hai lần không phát lại gì (`owner_previews_with_key`, `owner_previews_with_the_same_key_again`). Bình luận trong mã nói lý do: bản xem trước luôn phải giải lại quyền và dữ liệu hiện tại.
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 (`anonymous_previews`), 422 `invalid_actor_id`.
4. Validation path + thân `ItineraryPreviewRequest`. **Trước** khi handler chạy, nên một thân sai **không** tiêu một suất của cửa sổ nhịp (`owner_day_missing`, `owner_day_not_a_date`, `owner_include_suggestion_string`, `owner_body_missing`, `owner_extra_field`).
5. `limiter.check(actor.id)` — dòng **đầu tiên** của handler (`routes/outings.py:49`), trước cả khi đọc hàng chuyến. 30 lượt mỗi 60 giây mỗi actor; vượt → 429 `itinerary_rate_limited` `Đã tính nhiều tuyến liên tiếp. Chờ một phút rồi thử lại.` Cửa sổ **không** bị kéo dài bởi chính lần từ chối (`search_rate_limit.py:210-235`).
6. `response.headers["Cache-Control"] = "no-store"` trên mọi câu trả lời.
7. `_itinerary_draft` — **cùng hàm, cùng thứ tự, cùng câu chữ** với `PUT …/itinerary`: 404 `Không tìm thấy chuyến đi.` → 403 → 409 `timeline_conflict` → `itinerary_day_invalid` (ngày của `days`) → `itinerary_anchor_invalid` → `stop_not_found` → `itinerary_day_invalid` (ngày của chặng) → `stop_place_unknown`.
8. Rồi mới kiểm `record.starts_on <= request.day <= record.ends_on` → 422 `itinerary_day_invalid` `Ngày nằm ngoài chuyến đi.` (`service.py:3502-3503`; `owner_previews_a_day_outside_the_trip`). Thứ tự này quan trọng: ngày của **chặng** sai được báo trước ngày **được xem trước** sai.
9. `preview_itinerary(draft)` (`services/api/app/journey/preview.py:47-208`).

## Đầu vào

- Path `outing_id`: UUID.
- Thân `ItineraryPreviewRequest` = `ItineraryRequest` + `day` (`date`, bắt buộc) + `include_suggestion` (bool strict, mặc định `false`) (`schemas.py:579-581`). Header `idempotency-key` **không** được khai và không có tác dụng.

## Đầu ra

**200**, một `dict` thuần (route **không** khai `response_model`), thứ tự khoá do `preview.py:51-63`:

`revision` (= `expected_revision` của request), `day` (chuỗi `YYYY-MM-DD` như request gửi), `status`, `source` `{engine: "valhalla", graph_version, traffic: "none"}`, `current`, `suggestion`, `savings`, `issues`.

`issues` là danh sách `{code, message, stop_id?}` (`app/domain/journey.py`, hàm `issue`). Các nhánh và điều kiện:

| `code` | Khi nào | Sau đó |
|---|---|---|
| `invalid_stops` | > `MAX_STOPS` chặng hoặc id trùng | trả ngay, `status: incomplete` — **không tới được**, `max_length=50` và `model_validator` chặn trước |
| `missing_day_settings` | `days` không có mục nào cho `day`, hoặc `transport_mode` lạ | trả ngay |
| `empty_day` | không chặng nào mang `day` đó | trả ngay |
| `unassigned_day` | một chặng bất kỳ có `day` null | **không** trả ngay, đi tiếp |
| `invalid_schedule` | `minute()` hoặc `duration_minutes` hỏng | trả ngay — **không tới được**, `pattern` và biên của thân chặn trước |
| `missing_location` | chặng của ngày đó không có toạ độ | trả ngay |
| `anchor_mismatch` | neo có thật nhưng không nằm đúng đầu/cuối | **không** trả ngay |
| `routing_unavailable` | `configured_provider()` là None, hoặc `_CAPACITY` hết chỗ, hoặc provider ném | `status: unavailable` |

`graph_version` là `None` khi không có provider, nên `source.graph_version` là `null` trên stack parity.

## Tác dụng phụ

Không ghi gì (`test_preview_is_private_read_only_and_resolves_catalogue` kiểm đúng điều này). Đọc: hàng `outings`, `outing_stops`, `outing_stop_checkins` của chuyến, và một `get_place` **mỗi chặng có `place_id`**. Không có lệnh gửi ra ngoài trên stack parity vì `MOBILE_VALHALLA_URL` không được đặt.

## Idempotency

Không có. Middleware nhường route này trước khi đọc header (xem trên).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` | `deps.py:102-106`, `:143` |
| 422 | (validation) | thân và path | `main.py:319-351` |
| 429 | `itinerary_rate_limited` | `Đã tính nhiều tuyến liên tiếp. Chờ một phút rồi thử lại.` | `main.py:151-156`, `search_rate_limit.py:210-228` |
| 404 | `outing_not_found` | `Không tìm thấy chuyến đi.` | `service.py:3427` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 409 | `timeline_conflict` | `Lịch trình đã được sửa. Tải bản mới trước khi tiếp tục.` | `service.py:3440-3445` |
| 422 | `itinerary_day_invalid` | `Ngày nằm ngoài chuyến đi.` / `Chọn cấu hình cho ngày của chặng.` | `service.py:3450`, `:3476`, `:3503` |
| 422 | `itinerary_anchor_invalid` | `Điểm đầu/cuối phải thuộc ngày này.` | `service.py:3459` |
| 422 | `stop_not_found` | `Chặng không thuộc chuyến đi này.` | `service.py:3469` |
| 422 | `stop_place_unknown` | `Địa điểm không còn trong danh mục.` | `service.py:3481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:38-53`
- Service: `services/api/app/api/service.py:3496-3504`, `:3435-3494`
- Máy tính: `services/api/app/journey/preview.py:21-208`, `services/api/app/journey/routing.py:181-189` (`configured_provider`), `services/api/app/domain/journey.py`
- Nhịp: `services/api/app/api/main.py:151-156`, `services/api/app/api/search_rate_limit.py:210-235`

## Test đang phủ

- `services/api/tests/postgres/test_itinerary_postgres.py`: `test_preview_is_private_read_only_and_resolves_catalogue` (156)
- `services/api/tests/api/test_idempotency.py`: `test_itinerary_preview_never_caches_private_geometry` (288); `test_every_write_route_consults_the_store` (257) **bỏ qua** route này một cách tường minh (dòng 273)
- `services/api/tests/journey/test_preview.py`: 18 ca gọi thẳng `preview_itinerary`, không qua HTTP (79, 91, 98, 110, 119, 125, 134, 146, 154, 160, 169, 179, 187, 198, 206, 217, 227, 235)
- **Không có ca nào phủ `get_itinerary_limiter`** ở bất kỳ tầng nào.

## Kịch bản parity

`parity/scenarios/w7/outings/POST-outings-outing_id-itinerary-preview.yaml`, id `w7/outings/post-outings-outing_id-itinerary-preview` (43 bước, `dev`):

- Cửa: `anonymous_previews`, `anonymous_non_uuid_outing`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `owner_unknown_outing`, `stranger_previews`, `owner_previews_as_guest_role`.
- Thân: `owner_day_missing`, `owner_day_not_a_date`, `owner_include_suggestion_string`, `owner_body_missing`, `owner_extra_field`.
- Thứ tự `_itinerary_draft`: `owner_stale_revision`, `owner_days_day_outside_the_trip`, `owner_anchor_outside_its_day`, `owner_stop_of_another_trip`, `owner_place_not_in_catalogue`, rồi `owner_previews_a_day_outside_the_trip` (kiểm `day` đứng **sau**).
- Từng nhánh `issues`: `owner_previews_a_day_with_no_settings`, `owner_previews_an_empty_day`, `owner_previews_with_a_stop_that_has_no_day`, `owner_previews_a_stop_with_no_location`, `owner_previews_with_a_misplaced_anchor`.
- Không còn gì sai: `owner_previews_a_clean_day`, `owner_previews_asking_for_a_suggestion` — cả hai ra `status: unavailable` vì stack parity không cấu hình Valhalla.
- Khoá không có tác dụng: `owner_previews_with_empty_key`, `owner_previews_with_key`, `owner_previews_with_the_same_key_again`.
- Một lượt «đã tới» làm bản nháp cũ: `mate_previews_before_arriving`, `mate_checks_in_at_cafe`, `owner_previews_with_the_revision_from_before` (409), `owner_previews_after_the_arrival` (`checked_in` đã bật).
- Framework: `trailing_slash`, `get_not_allowed`.

Tệp này giữ **20 lượt gọi của `owner`**, dưới trần 30, để bản chạy chính và canary (chạy lại cả bộ) không bao giờ đụng cửa sổ.

`parity/scenarios/w7/limiter/POST-outings-outing_id-itinerary-preview.yaml`, `lane: limiter` (35 bước): 30 lượt 404 rồi lượt thứ 31 là 429; một lượt của `mate` cho thấy cửa sổ đếm theo actor; `owner_bad_body_after_the_ceiling` cho thấy thân sai vẫn là 422 (chưa tới limiter) và `anonymous_preview_after_the_ceiling` vẫn là 401. Làn này **không** nằm trong bản chạy chính hay canary.

`prod-auth.yaml`: `anonymous_previews_with_broken_json`, `owner_previews`.

Corpus sinh: route này **hoãn** (`ItineraryPreviewRequest` có `model_validator(mode="after")`).

## Chưa phủ / lưu ý cho bản Go

- **Nửa có định tuyến thật không được phủ.** Stack parity không đặt `MOBILE_VALHALLA_URL`/`MOBILE_ROUTING_GRAPH_VERSION`, nên `configured_provider()` luôn trả None và mọi bản nháp không lỗi dừng ở `unavailable`. `_route`, `schedule`, `suggest_order`, `savings`, `feasible`, `segments`, `late_fixed_stop`, `routing_busy` **chưa có bằng chứng parity nào** — chúng chỉ có golden vi sai ở tầng domain (`internal/domain/itinerary`, `internal/domain/valhalla`).
- `invalid_stops` và `invalid_schedule` không tới được qua HTTP; giữ lại trong bản Go vì `preview_itinerary` là hàm xuất, nhưng không có kịch bản nào chứng minh.
- `routing_busy` (`_CAPACITY` là `BoundedSemaphore(4)` **mức tiến trình**) không đo được: cần 5 bản xem trước chồng nhau và một provider chậm.
- 429 chỉ được chứng minh trong làn limiter, và làn đó phải chạy trọn trong **một** cửa sổ 60 giây cho cả hai stack.

## Lỗi Python (chỉ báo, không sửa)

- `preview_itinerary` báo `status: "unavailable"` ngay cạnh một `current` đã định tuyến đầy đủ khi chỉ ma trận hoặc bước tra ứng viên hỏng — wire tự mâu thuẫn.
- Một ngày định tuyến trót lọt vẫn bị `feasible: false` nếu `issues` của **cả** bản xem trước không rỗng, kể cả khi issue thuộc ngày khác hoặc chỉ là `anchor_mismatch`.
- `suggest_order` chấm đường thiếu bằng `10**9`, đúng con số mà `_number` từ chối khi **vượt** — nên một quãng đúng `10**9` giây được nhận rồi chấm như «không có đường».
- `trip["status"] != 0` nhận cả `False`, vì `bool` là `int`: câu trả lời Valhalla có `status: false` được coi là thành công.
- Cửa nhịp nằm **trước** cả kiểm chuyến tồn tại, nên một người lạ có thể làm cạn cửa sổ của chính họ trên một id không có thật — vô hại, nhưng cũng có nghĩa 429 không nói gì về quyền.
