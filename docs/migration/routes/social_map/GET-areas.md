# GET /areas

social_map · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trả danh sách cố định các khu vực (quận / thành phố) mà `POST /contexts/{context_id}/meet` chấp nhận, kèm tâm toạ độ dùng để đo mọi khoảng cách. Tồn tại để màn chọn điểm xuất phát không phải chép tay id.

## Xác thực và quyền

- Không có dependency nào, kể cả `get_repository` (`services/api/app/api/routes/social_map.py:41-58`). Không xác thực ở cả `prod` lẫn `dev`, không mở session DB.
- `X-Actor-*` rác bị bỏ qua (`junk_actor_headers_ignored` → 200).
- Không có kiểm quyền, nên không có câu hỏi 403 hay 404.

## Đầu vào

Không path, query, header hay body nào được đọc. Query bị bỏ qua (`query_string_ignored`). Chỉ nhận `GET`; `HEAD` và `POST` → 405.

## Đầu ra

- 200: mảng JSON ở top level (không bọc object) các `AreaSummary` (`services/api/app/api/schemas.py:2250-2256`), thứ tự khoá `id`, `label`, `lat`, `lng`.
- Thứ tự phần tử là thứ tự khai báo trong `AREAS` (`services/api/app/places/areas.py:75-97`), **không** sort theo id: `da-lat`, `hcm-quan-1`, `hcm-quan-3`, `hcm-quan-4`, `hcm-phu-nhuan`, `hcm-quan-7`, `hcm-binh-thanh`, `hcm-thu-duc`.
- Float: `lat`, `lng`. Python ghi bằng `repr` ngắn nhất: literal nguồn `10.7840` ra `10.784`, `106.6800` ra `106.68`, `10.8500` ra `10.85`.
- Không có datetime. Nhãn tiếng Việt đi ra UTF-8 thô.
- Framework: 307 cho `/areas/` (`location: http://<Host>/areas`); 405 `{"detail":"Method Not Allowed"}` + `allow: GET` (HEAD thì không body).
- Có `Origin` loopback thì thêm `access-control-allow-origin` (`services/api/app/api/main.py:283`).

## Tác dụng phụ

Chỉ đọc, không chạm DB. Không idempotency (GET). Không limiter.

## Lỗi

Route không ném `ApiProblem` nào. Chỉ có 405 và 307 của framework.

## Mã Python

- Route: `services/api/app/api/routes/social_map.py:41-58` (`list_areas`, không đi qua `ApiService`)
- Dữ liệu: `services/api/app/places/areas.py:75-97` (`AREAS`), `:190-203` (`area_summary`)

## Test đang phủ

- `services/api/tests/api/test_areas.py`: `test_every_known_area_is_offered` (18), `test_the_ids_offered_are_ids_meet_accepts` (33), `test_the_catalogue_carries_no_group_and_no_person` (55), `test_no_actor_header_is_still_answered` (64)
- `services/api/tests/places/test_areas.py` (25-156): id duy nhất, toạ độ hợp lệ, khớp với catalogue seed

## Kịch bản parity

`parity/scenarios/w1/social_map/GET-areas.yaml`, id `w1/social_map/get-areas`:

- `anonymous`, `with_actor`: nhánh 200 duy nhất.
- `junk_actor_headers_ignored`, `query_string_ignored`: đầu vào bị bỏ qua.
- `cors_simple_get`: header CORS.
- `trailing_slash_redirects`: 307.
- `head_not_allowed`, `post_not_allowed`: 405 + `allow`.

## Chưa phủ / lưu ý cho bản Go

- Chạy `auth_mode: dev`. Harness đã có persona phiên `prod` (`w0/prod-sessions`), nhưng route không xác thực nên không đổi theo mode.
- Float: `strconv.FormatFloat(v, 'f', -1, 64)` cho kết quả giống `repr` với các giá trị hiện có. Nếu sau này có toạ độ nguyên (ví dụ `106.0`), Python ghi `106.0` còn Go ghi `106`. Canary `float-lost-point` chỉ bắt được nếu có giá trị như vậy.
- Top-level là mảng: đừng bọc thành `{"areas": [...]}`.
- Danh sách này và bộ kiểm `find_area` của `meet` phải cùng một nguồn; bản Go nên dùng chung một bảng cho cả hai route.
