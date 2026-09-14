# GET /interests

preferences · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trả bộ từ vựng khẩu vị đóng (8 tag) và 3 khoảng ngân sách mà `PUT /people/me/interests` chấp nhận. Route công khai, vì màn cá nhân hoá được vẽ trước khi có phiên.

## Xác thực và quyền

- Không có `get_actor`. Route chỉ khai `Depends(get_repository)` (`services/api/app/api/routes/preferences.py:58-63`), nên cả `prod` lẫn `dev` đều trả 200 cho người không danh tính.
- `X-Actor-*` rác không bị kiểm (kịch bản `junk_actor_headers_ignored` → 200). Mọi route có `get_actor` thì trả 422 cho cùng header đó (`services/api/app/api/deps.py:144-163`).
- Không có kiểm quyền, nên không có câu hỏi 403 hay 404.

## Đầu vào

- Không đọc path, query hay header nào. Query string bị bỏ qua (`query_string_ignored`), `Accept` bị bỏ qua (`accept_html_still_json`).
- Không body. Chỉ nhận `GET`; `HEAD` và `POST` → 405, vì `APIRoute` của FastAPI không tự thêm `HEAD`.

## Đầu ra

- 200 `InterestVocabularyResponse` (`services/api/app/api/schemas.py:872-881`), thứ tự khoá: `interests`, `budget_bands`.
  - `interests[]`: `id`, `label` (`schemas.py:850-854`), đúng thứ tự tuple `INTEREST_TAGS` (`services/api/app/domain/interests.py:68-77`): `an-uong`, `cafe`, `nightlife`, `mon-local`, `outdoor`, `shopping`, `karaoke`, `game`.
  - `budget_bands[]`: `id`, `label`, `min_vnd`, `max_vnd` (`schemas.py:857-869`), thứ tự `BUDGET_BANDS` (`domain/interests.py:106-110`). Cả ba khoảng hiện đều có `max_vnd` là số nguyên; schema cho phép `null`.
- Không có float, không có datetime. Nhãn có chữ ngoài ASCII (`Ăn uống`, `Dưới 100K`, dấu U+2013 trong nhãn `vua-phai`) đi ra UTF-8 thô: Starlette `JSONResponse` dùng `ensure_ascii=False` và dấu phân cách gọn `,` `:`.
- 307 cho `/interests/`, header `location: http://<Host>/interests`, không body, không `content-type` (redirect_slashes của Starlette).
- 405 `{"detail":"Method Not Allowed"}` kèm `allow: GET`; trả lời `HEAD` thì không có body.
- Có `Origin` loopback thì thêm `access-control-allow-origin` (`services/api/app/api/cors.py:28`, cài ngoài cùng ở `services/api/app/api/main.py:283`).

## Tác dụng phụ

Chỉ đọc. Không chạy query nào, nhưng `get_repository` vẫn mở rồi đóng một session (`deps.py:196-209`). Không tham gia idempotency: `IdempotencyMiddleware` bỏ qua GET (`services/api/app/api/idempotency.py:76`, `:405-407`). Không có limiter.

## Lỗi

Route không ném `ApiProblem` nào. Chỉ có trả lời của framework: 405 và 307 như trên.

## Mã Python

- Route: `services/api/app/api/routes/preferences.py:58-63`
- Service: `services/api/app/api/service.py:4106-4127` (`interest_vocabulary`, không gọi repository)
- Domain: `services/api/app/domain/interests.py:68-77` (`INTEREST_TAGS`), `:106-110` (`BUDGET_BANDS`)

## Test đang phủ

- `services/api/tests/api/test_interests.py::test_the_vocabulary_is_public` (dòng 34), `::test_budget_bands_travel_as_two_integers` (50)
- `services/api/tests/postgres/test_interests_postgres.py::test_the_vocabulary_route_needs_no_session` (208)
- `services/api/tests/domain/test_interests.py` (dòng 24-121): id duy nhất, thứ tự, khoảng ngân sách không chồng nhau

## Kịch bản parity

`parity/scenarios/w1/preferences/GET-interests.yaml`, id `w1/preferences/get-interests`:

- `anonymous`, `with_actor`: nhánh 200 duy nhất, không và có danh tính.
- `junk_actor_headers_ignored`: chứng minh route không có `get_actor`.
- `query_string_ignored`, `accept_html_still_json`: đầu vào bị bỏ qua.
- `cors_simple_get`: header CORS trên câu trả lời 200.
- `trailing_slash_redirects`: 307 + `location`.
- `head_not_allowed`, `post_not_allowed`: 405 + `allow`.

## Chưa phủ / lưu ý cho bản Go

- Kịch bản chạy `auth_mode: dev`; harness chưa dựng được persona `prod`. Route không xác thực, nên khác biệt ở `prod` chỉ có thể đến từ middleware.
- Không phủ: DB không kết nối được. Python vẫn mở session, nên có thể 500 khi pool cạn; một bản Go trả hằng số sẽ không 500. Nếu chọn vậy thì ghi lại là khác biệt có chủ ý.
- Thứ tự phần tử là dữ liệu, không được sort lại. `max_vnd` phải giữ được `null` (không `omitempty`).
- Không thoát chữ tiếng Việt thành `\uXXXX`. `encoding/json` của Go giữ UTF-8 nhưng mặc định thoát `<`, `>`, `&` (không có trong dữ liệu này, nhưng xem `POST /contexts/{context_id}/meet`).
