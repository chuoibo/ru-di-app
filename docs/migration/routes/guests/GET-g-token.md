# GET /g/{token}

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trang khách (spec mục 8.6). Người cầm link thấy **phong bì của chính mình** trong một phiên bản đợt thu: ai đã ghi, phần của ai, trong dịp nào, bao nhiêu, trả cho ai, và khoản đó đang ở đâu (đã báo chuyển, người nhận đã xác nhận, đang thắc mắc). Trang không bao giờ có số dư nhóm, lịch sử nhóm hay phần của người khác (`services/api/app/web/guest_view.py:1-15`). Câu trả lời là HTML render phía máy chủ bằng Jinja2, không có biến thể JSON. Trang này là cửa vào của các route còn lại: form `da-chuyen`, link `doi-so-tien?obligation_id=…` và link `khong-phai-toi`.

## Xác thực và quyền

Không có actor. Route không đọc `X-Actor-*`, `Authorization` hay cookie: token trong path **là** capability. `prod-auth.yaml` đo điều đó: phiên thật, bearer rác và header dev đều nhận cùng một trang.

Thứ tự (đọc từ mã, kịch bản đo):

1. Router Starlette, trước mọi mã của route:
   - phương thức khác `GET` → 405 `{"detail":"Method Not Allowed"}` kèm `allow: GET`. **HEAD cũng 405**, vì APIRoute của FastAPI không tự thêm HEAD; thân rỗng nhưng vẫn `content-length: 31` (`head_page`, `post_to_page`, `options_page`);
   - đuôi `/` → 307 với `location: http://<Host>/g/<token>` tuyệt đối, **trước** khi tra token nên token lạ cũng 307 (`page_trailing_slash`, `unknown_token_trailing_slash`);
   - `/g`, `/g/`, và `%2F` trong token (được giải mã thành `/` trước khi khớp route) → 404 `{"detail":"Not Found"}` (`prefix_alone`, `prefix_slash`, `token_with_encoded_slash`).
2. Validation path `Token` (`services/api/app/api/routes/guests.py:28-31`): `min_length=32`, `max_length=128`, `pattern=^[A-Za-z0-9_-]+$`, kiểm trên chuỗi đã giải mã (`%2E` là dấu chấm). Sai → 422 JSON không có `input` (`services/api/app/api/main.py:319-351`; `token_too_short`, `token_too_long`, `token_with_dot`, `token_with_encoded_dot`).
3. `ApiService.guest_view` (`services/api/app/api/service.py:6711-6723`) gọi `get_guest_envelope(sha256(token), now)` (`services/api/app/api/repository.py:6516-6722`). Không có hàng → `ApiProblem(404, "guest_link_not_found")`, và handler `main.py:313-314` đổi nó thành trang `guest_link_broken.html` (`routes/guests.py:34-50`; `unknown_token_43`, `unknown_token_min_length`, `unknown_token_max_length`).
4. `_require_permission("view_guest_envelope", _guest_actor(token), {"is_own_capability": True})` (`service.py:6715-6719`, `:427-441`; `services/api/app/domain/permissions.py:90`) luôn qua: actor tổng hợp `capability:<16 hex đầu của digest>` mang role `guest`.
5. `build_guest_view` (`guest_view.py:83-155`) ném `GuestViewError` → 409 (`service.py:6720-6723`). Không tới được qua HTTP: repository không bao giờ đưa khoá cấm hay trạng thái lạ.

## Đầu vào

- Path `token`: 32..128 ký tự `[A-Za-z0-9_-]`. Publish sinh 43 ký tự (`secrets.token_urlsafe(32)`).
- Query string bị bỏ qua (`guest_a_opens_with_query`).
- Không đọc header nào.

## Đầu ra

**200**, `content-type: text/html; charset=utf-8`, render `guest.html` với context đúng ba khoá `view`, `preview` (`NEUTRAL_PREVIEW`, `guest_view.py:158-161`) và `token` (`routes/guests.py:53-70`).

- Byte đầu là `<!doctype html>`: khối comment Jinja đứng sát `#}<!doctype` nên không để lại dòng nào. **Không có newline cuối**: Jinja mặc định `keep_trailing_newline=False` cắt newline cuối tệp mẫu, nên thân kết thúc bằng `</html>`.
- Khoảng trắng quanh thẻ `{% if %}`/`{% for %}` giữ nguyên (không `trim_blocks`, không `lstrip_blocks`). Ví dụ `<title>…</title>\n\n<meta property="og:title"`, vì một comment Jinja chiếm trọn một dòng.
- Autoescape của Starlette `Jinja2Templates` (markupsafe): `&` → `&amp;`, `<` → `&lt;`, `>` → `&gt;`, `"` → `&#34;`, `'` → `&#39;`. `+`, `=`, backtick, RTL mark (U+202E, U+200F) và emoji ra nguyên byte UTF-8. `guest_a_opens` ghim điều này: tên chủ đợt và nhãn bữa tối là chuỗi thù địch.
- Link không `active` (`guest.html:37-43`): một thẻ "Link này đã bị thu hồi / hết hạn / được thay bằng link mới" kèm tên người ghi, không có block nào (`guest_a_page_revoked`).
- Mỗi block (`guest.html:45-158`), nghĩa vụ xếp `ORDER BY recipient_id` (`repository.py:6545-6553`):
  - `data-obligation`, eyebrow "`<người ghi>` đã ghi", "Phần của **`<người được ghi tên>`** trong `<dịp>`", nút số tiền `data-copy="<int>"` và `amount_display` nhóm nghìn bằng dấu chấm (`guest_view.py:69-80`);
  - ghi chú đang thắc mắc khi `disputed` (`guest.html:68-74`);
  - câu hiện trạng: người nhận đã xác nhận, nếu không thì đã báo chuyển (`:84-93`); nhãn nút "Xem lại cách chuyển" hoặc "Đúng, xem cách chuyển", lớp `btn--quiet` khi đã xác nhận (`:103-110`);
  - link "Số tiền không đúng" chỉ khi `block.can_object`, tức nghĩa vụ đó còn dưới 3 lần phản đối (`guest_view.py:129-130`, `guest.html:117-120`); link "Tôi không phải …" luôn có;
  - mặt thứ hai: đã xác nhận, đã báo, form `POST /g/<token>/da-chuyen` khi `view.can_report_payment` (cả link còn dưới 3 lần báo), hoặc "Bạn đã báo nhiều lần rồi" (`guest.html:136-152`).
- `recorded_by_display_name`: tên người ghi nếu mọi khoản nguồn do cùng một người ghi, ngược lại chuỗi `Người tạo đợt` (`repository.py:6700-6706`; `GET-g-token-doi-so-tien.yaml` đo nhánh này). Người không có dòng `people` hiện `str(uuid)` (`repository.py:2529-2548`).
- `occasion_label`: mô tả các phiên bản khoản nguồn, bỏ trùng, xếp theo chuỗi, nối bằng `", "`; rỗng thì `đợt thu này` (`repository.py:6640-6646`).
- `receiver_confirmed` = trạng thái suy ra từ `receipt_confirmations` là `confirmed` hoặc `over_confirmed`; `already_reported` = link có `payment_reports` cho nghĩa vụ; `disputed` = có audit `guest_objection.wrong_amount` cho nghĩa vụ (`repository.py:6560-6698`).

**404** là `guest_link_broken.html`, `text/html; charset=utf-8`, 1251 byte cố định, không chứa token hay tên.

**Header**: mọi câu trả lời dưới `/g` (200, trang 404, 404/405/307 của framework, 422) mang đúng ba header, gán đè: `cache-control: no-store`, `referrer-policy: no-referrer`, `x-robots-tag: noindex, nofollow` (`services/api/app/api/guest_privacy.py:30-34`, `:74-108`). 500 do exception cũng mang ba header này (`guest_privacy.py:43-71`, `main.py:272`). 307 có `content-length: 0` và không có `content-type`. Không có `vary`, `etag` hay `set-cookie`.

## Tác dụng phụ

GET **ghi**. `get_guest_envelope` chạy `SELECT … FOR UPDATE` trên join `guest_links ⋈ collection_envelopes ⋈ collection_batch_versions ⋈ collection_batches` (`repository.py:6519-6533`). Không có `OF`, nên dòng của cả bốn bảng bị khoá tới commit. Trong transaction đó:

- `guest_links.first_opened_at = now` nếu đang NULL (`repository.py:6537-6538`). `guest_a_opens` ghi; `guest_a_opens_again` không ghi gì.
- `guest_links.status = 'expired'` nếu `status='active'` và `now >= expires_at` (`:6540-6543`; không phủ, xem dưới).

Commit xảy ra trước khi gửi thân (`services/api/app/api/unit_of_work.py:41-48`, cài ở `main.py:354`). Nhánh 404/409 là exception, nên `session.rollback()` (`services/api/app/api/deps.py:196-209`) và không có gì được ghi. Không có audit event. GET không đi qua middleware idempotency.

Hệ quả của khoá: mọi request khách trên cùng một đợt (kể cả link của người gửi khác, vì cùng hàng `collection_batches`) chạy nối tiếp nhau.

## Lỗi

| Status | Thân | Nguồn |
|---|---|---|
| 404 | trang `guest_link_broken.html`; mã `guest_link_not_found` và detail `Guest link does not exist` không bao giờ ra dây | `service.py:6712-6714`; `main.py:313-314`; `routes/guests.py:34-50` |
| 409 | `{"code":"<mã GuestViewError viết hoa>","detail":"Guest envelope is not renderable"}` (không tới được) | `service.py:6720-6723` |
| 422 | `{"detail":[{"type":…,"loc":["path","token"],"msg":…,"ctx":…}]}`, type là `string_too_short`, `string_too_long` hoặc `string_pattern_mismatch` | `routes/guests.py:28-31`; `main.py:319-351` |
| 405 | `{"detail":"Method Not Allowed"}`, `allow: GET`; HEAD thân rỗng | Starlette |
| 404 | `{"detail":"Not Found"}` | Starlette |
| 307 | thân rỗng, `location` tuyệt đối theo `Host` | Starlette `redirect_slashes` |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:25-70`
- Middleware, handler: `services/api/app/api/guest_privacy.py:30-108`; `services/api/app/api/main.py:252-272`, `:296-351`
- Service: `services/api/app/api/service.py:6711-6723`, `:427-441`
- Repository: `services/api/app/api/repository.py:6516-6722` (`get_guest_envelope`), `:2529-2548` (`_display_names`)
- View model: `services/api/app/web/guest_view.py:69-161`
- Mẫu: `services/api/app/web/templates/guest.html`, `guest_link_broken.html`

## Test đang phủ

- `services/api/tests/api/test_guests.py`: `test_guest_route_renders_only_closed_guest_view` (15), `test_raw_group_data_is_rejected_before_template_render` (31)
- `services/api/tests/api/test_guest_link_broken.py` (32, 56, 77, 92, 106)
- `services/api/tests/api/test_guest_privacy_headers.py` (110, 135, 150, 170, 186, 201)
- `services/api/tests/api/test_pr301_guest_boundary.py` (75, 95, 112, 139)
- `services/api/tests/web/test_guest_page.py` (view model và render, 77-334)
- `services/api/tests/postgres/test_repository_postgres.py`: `test_guest_is_not_told_the_money_arrived_before_it_did` (388), `test_guest_http_uses_name_derived_from_real_postgres_projection` (547)

Các test `tests/api/` chạy trên repository giả: không chứng minh khoá, `first_opened_at` hay rollback.

## Kịch bản parity

`parity/scenarios/w5/guests/GET-g-token.yaml`, id `w5/guests/get-g-token` (40 bước, `dev`):

- Dựng: ba người, tên chủ đợt chứa `<script>`, nháy kép, nháy đơn, `&`, `+`, `=`, backtick, U+202E, U+200F và emoji; nhóm; bữa tối 90000 chia ba; đợt; publish. Hai token gắn theo tên (`tok_a`, `tok_b`).
- Trạng thái: `guest_a_opens` (ghi `first_opened_at`), `guest_a_opens_again`, `guest_a_opens_with_query`, `guest_b_opens` (phong bì kia, không có gì của người A), `guest_a_reports_paid` → `guest_a_page_reported`, `owner_confirms_b_received` → `guest_b_page_confirmed`, `guest_b_objects_amount` → `guest_b_page_confirmed_disputed`, `guest_a_says_not_me` → `guest_a_page_revoked`, `guest_a_page_revoked_again`.
- Framework: `head_page`, `page_trailing_slash`, `post_to_page`, `options_page`, `prefix_alone`, `prefix_slash`.
- Hình dạng token: `unknown_token_43`, `unknown_token_min_length` (32), `unknown_token_max_length` (128), `unknown_token_trailing_slash`, `token_too_short` (31), `token_too_long` (129), `token_with_dot`, `token_with_encoded_dot`, `token_with_encoded_slash`.

Các tệp khác cũng mở trang này: `GET-g-token-doi-so-tien.yaml` (`guest_a_home` với hai nghĩa vụ và `Người tạo đợt`, `guest_a_home_disputed`, `guest_b_home`), `POST-g-token-khong-phai-toi.yaml` (`guest_a_opens_home`, `guest_a_home_revoked`), `POST-g-token-da-chuyen.yaml` (`guest_b_page_after_receipt`), `GET-g-token-khong-phai-toi.yaml` (`guest_b_home_revoked`), `POST-g-token-xin-cach-tinh.yaml` (`guest_a_home_after_evidence`), `validation-422.yaml` (`page_token_*`), `prod-auth.yaml` (`guest_a_opens_*`, bốn kiểu danh tính), `crossreplay/POST-g-token-da-chuyen.yaml` (`core_page_a`, `python_page_b`).

Corpus 422 sinh tự động: hoãn, `carries ['pattern']` (wave `w5` trong `scripts/render_parity_422_scenarios.py`). Các ca 422 viết tay nằm ở `validation-422.yaml`.

## Chưa phủ / lưu ý cho bản Go

- **Link hết hạn không phủ.** Định dạng kịch bản không có bước tua đồng hồ (`parity/internal/scenario/scenario.go` chỉ chứa request); publish từ chối hạn trong quá khứ và `CHECK expires_at > created_at` chặn tạo link đã hết hạn. Nhánh UPDATE `status='expired'` và câu "Link này đã hết hạn" chỉ đọc từ mã. Trạng thái `rotated` không route nào sinh ra.
- 409 `GuestViewError` không tới được.
- Escape phải khớp markupsafe từng byte. `html/template` của Go escape `+` thành `&#43;` và escape theo ngữ cảnh URL trong `href`; dùng nguyên dạng sẽ lệch ngay ở `guest_a_opens`.
- Không có newline cuối; whitespace của thẻ khối giữ nguyên; trang 404 là 1251 byte cố định.
- HEAD là 405 JSON, không phải GET bỏ thân. 307 đuôi `/` lấy `Host` của request, giữ query, và đi trước khi tra token.
- Khoá `FOR UPDATE` trên bốn bảng, ghi `first_opened_at` dù chỉ đọc, rollback ở mọi nhánh lỗi.
- Ba header riêng tư phải có trên cả 404/405/307/422/500 của framework dưới `/g`, và không có ngoài `/g` (`/goals` không phải `/g`, `guest_privacy.py:37-40`).

## Lỗi Python (chỉ báo, không sửa)

- HEAD trên một trang HTML là 405 JSON; công cụ xem trước link dùng HEAD sẽ thấy lỗi.
- Một GET chỉ để đọc lại khoá `FOR UPDATE` cả hàng `collection_batches`: mọi khách của một đợt, và publish của chủ đợt (khoá cùng hàng), phải chờ nhau.
