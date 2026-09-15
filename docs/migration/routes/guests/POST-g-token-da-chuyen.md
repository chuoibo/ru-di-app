# POST /g/{token}/da-chuyen

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Người gửi báo "tôi đã chuyển". Route ghi một `payment_reports` và một audit event, và **không** đổi trạng thái nghĩa vụ: chỉ `confirm-receipt` của người nhận làm việc đó (spec mục 8.6). Client nhận 201 JSON; form HTML trên trang khách (Accept có `text/html`) nhận 303 về lại trang (post-redirect-get).

## Xác thực và quyền

Không có actor; token là capability. Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`), vì mọi POST đều qua nó, kể cả dưới `/g`:
   - `Idempotency-Key` rỗng hoặc dài hơn 255 → 422 `invalid_idempotency_key` (`guest_c_empty_header_key`);
   - scope = digest của bearer nếu có `Authorization: Bearer`, không thì giá trị thô của `X-Actor-ID`, không thì `anonymous` (`guest_c_header_key_with_actor_header`; `prod-auth.yaml`);
   - cùng scope, cùng khoá, cùng request đã hoàn tất 2xx → phát lại (`guest_c_replays_header_key`); khác thân → 422 `idempotency_key_reuse` (`guest_c_header_key_other_body`); request trước chưa xong → 409 `idempotency_request_in_flight`.
2. Router: đuôi `/` → 307 tuyệt đối; `GET`/`HEAD` → 405 `allow: POST`.
3. Validation: token (32..128, `[A-Za-z0-9_-]`) và form model `PaymentReportRequest` (`services/api/app/api/schemas.py:1927-1929`). Lỗi path và lỗi thân nằm chung một danh sách 422.
4. `get_payment_report_target` (`services/api/app/api/repository.py:6724-6757`) khoá `guest_links ⋈ collection_envelopes` `FOR UPDATE`. Không có token, **hoặc** nghĩa vụ không thuộc đúng người gửi và phiên bản đợt của phong bì → 404 `guest_obligation_not_found` (`services/api/app/api/service.py:6922-6928`). Nhánh này đi trước kiểm quyền, nên nghĩa vụ ngoài link vẫn 404 khi hạn mức đã hết (`guest_a_obligation_of_link_b`).
   - **Token lạ ra JSON 404, không phải trang link hỏng**: handler ở `main.py:313-314` chỉ đổi mã `guest_link_not_found` (`unknown_token_report`, `unknown_token_report_from_browser`).
5. `_require_permission("report_payment", guest, …)` (`service.py:6929-6937`; `services/api/app/domain/permissions.py:91-98`) kiểm các vị từ theo thứ tự khai báo: `active_capability` (link `active` và chưa tới `expires_at`) → 403 `active_capability`; rồi `report_budget_available` (`reports_used < 3`, đếm mọi báo của **cả link**) → 403 `report_budget_available` (`guest_a_fourth_report`).
6. `save_payment_report` (`repository.py:6759-6808`): nếu `idempotency_key` trong thân đã có hàng mà khác nghĩa vụ, khác link hoặc khác số tiền → `RepositoryConflict` → 409 `idempotency_key_reused` (`service.py:6938-6947`; `guest_a_reuses_body_key_of_b`).
7. Thành công: `"text/html" in accept` → 303, ngược lại 201 (`routes/guests.py:91-94`).

## Đầu vào

- Path `token`.
- Thân form, model `PaymentReportRequest` với `extra="forbid"` (`schemas.py:66-67`):
  - `obligation_id: UUID` bắt buộc, UUID lax của pydantic (`urn:uuid:…` được nhận);
  - `idempotency_key: UUID | None = None`.
- Cách đọc form (FastAPI 0.115.6, Starlette 0.41.3):
  - `application/x-www-form-urlencoded`, kể cả có `; charset=utf-8`, và `multipart/form-data` đều được đọc (`report_form_with_charset`, `report_multipart`);
  - content-type khác (`application/json`, `text/plain`) hoặc không có → form rỗng → 422 `missing` (`report_json_body`, `report_text_plain`, `report_no_content_type`, `report_empty_body`);
  - trường lặp → **giá trị cuối** (`report_repeated_field_last_wins`);
  - giá trị rỗng **không** tính là thiếu: `obligation_id=` và `idempotency_key=` đều ra 422 `uuid_parsing` với msg `Input should be a valid UUID, invalid length: expected length 32 for simple format, found 0` (`report_empty_value`, `report_empty_idempotency_key`);
  - trường lạ → 422 `extra_forbidden`; tên phân biệt hoa thường (`Obligation_id` là `missing` + `extra_forbidden`, `missing` đứng trước) (`report_extra_field`, `report_field_name_case`, `report_multipart_extra_field`).
- Header `Accept`: so **chuỗi con, phân biệt hoa thường**. `text/html` và `application/xhtml+xml,text/html;q=0.9,*/*;q=0.8` → 303; `TEXT/HTML` → 201 JSON (`guest_a_accept_uppercase`).
- Header `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **201** `PaymentReportResponse` (`schemas.py:1932-1938`), JSON gọn, thứ tự khoá `payment_report_id`, `obligation_id`, `amount_vnd`, `obligation_status`.
  - `amount_vnd` luôn là **toàn bộ** số tiền của nghĩa vụ; khách không gửi số tiền.
  - `obligation_status` suy từ receipts: `outstanding`, hoặc `confirmed` khi người nhận đã xác nhận đủ (`guest_b_reports_after_receipt`).
- **303**: `location: /g/<token>` tương đối, `content-length: 0`, không có `content-type`; báo cáo **vẫn được ghi** (`guest_a_reports_from_browser`, `guest_b_accept_list_with_html`).
- Mọi từ chối là JSON kể cả khi Accept có `text/html` (`guest_a_fourth_report_from_browser`, `unknown_token_report_from_browser`).
- Phát lại từ middleware: 201, header `content-length`, `idempotency-replayed: true`, `content-type: application/json` (`idempotency.py:585-599`). Thân là byte đã lưu.
- Từ chối của middleware: thân `json.dumps` **có khoảng trắng** (`{"code": "…", "detail": "…"}`), header `content-type` đứng trước `content-length` (`idempotency.py:602-620`). Từ chối của route (`JSONResponse`) là JSON gọn.
- Luôn kèm ba header riêng tư `cache-control: no-store`, `referrer-policy: no-referrer`, `x-robots-tag: noindex, nofollow`.

## Tác dụng phụ

- Khoá dòng `guest_links` và `collection_envelopes` `FOR UPDATE` tới commit (commit trước khi gửi thân, `services/api/app/api/unit_of_work.py:41-48`).
- Báo mới (`repository.py:6780-6800`), cùng một `now`:
  - `payment_reports`: `id` uuid4, `obligation_id`, `guest_link_id`, `reported_by_id` NULL, `amount_vnd` của nghĩa vụ, `idempotency_key` = khoá trong thân hoặc uuid4 mới, `reported_at`;
  - `audit_events`: `actor_id` NULL, `event_type` `payment_reported`, `aggregate_type` `collection_obligation`, `aggregate_id` = nghĩa vụ, `request_id` = idempotency key của báo, `event_data` `{"payment_report_id": "<id>"}`, `occurred_at`.
- Khoá trong thân đã có, cùng đích → không ghi gì, trả hàng cũ với tổng receipt hiện tại (`guest_b_repeats_body_key`, `guest_b_body_key_urn_spelling`).
- `idempotency_keys` chỉ khi có header và câu trả lời 2xx: một hàng (`scope`, `idempotency_key`, `request_fingerprint`, `response_status` 201, `response_body`, `response_media_type` `application/json`). 303 và mọi từ chối **không** được lưu, khoá được nhả (`guest_c_header_key_from_browser_again` ghi báo thứ hai).
- **Không** chạm `guest_links.first_opened_at`; chỉ các route đọc trang ghi cột này.
- Mọi từ chối không ghi gì (rollback, `services/api/app/api/deps.py:196-209`).
- Bảng thu hiển thị `payment_reported_at` = `MIN(reported_at)` của nghĩa vụ; trạng thái nghĩa vụ không đổi.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:404-553` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:404-553` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["path","token"]` hoặc `["body","<trường>"]` | `main.py:319-351` |
| 404 | `guest_obligation_not_found` | `Obligation is outside this link` | `service.py:6926-6928` |
| 403 | `permission_denied` | `active_capability` / `report_budget_available` (`role_not_permitted`, `is_own_capability` không tới được) | `service.py:6929-6937`, `:504` |
| 409 | `idempotency_key_reused` | `Payment report conflicted` | `service.py:6944-6947`; `repository.py:6777` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:73-94`
- Service: `services/api/app/api/service.py:6918-6958` (`report_payment`), `:427-441` (`_guest_actor`)
- Repository: `services/api/app/api/repository.py:6724-6757` (`get_payment_report_target`), `:6759-6808` (`save_payment_report`)
- Schema: `services/api/app/api/schemas.py:1927-1938`
- Quyền: `services/api/app/domain/permissions.py:91-98`; hạn mức `services/api/app/api/limits.py:21`
- Middleware: `services/api/app/api/idempotency.py:404-620`

## Test đang phủ

- `services/api/tests/api/test_guests.py`: `test_sender_self_report_is_an_event_and_never_closes_obligation` (42), `test_browser_payment_form_uses_post_redirect_get` (56), `test_guest_link_cannot_report_an_obligation_outside_its_scope` (71)
- `services/api/tests/api/test_payment_report_board.py` (51, 67, 84, 97, 137, 166)
- `services/api/tests/postgres/test_repository_postgres.py`: `test_guest_is_not_told_the_money_arrived_before_it_did` (388), `test_the_guest_pressing_the_button_changes_the_advancers_next_refresh` (911)
- `services/api/tests/api/test_idempotency.py` (middleware chung), `test_guest_privacy_headers.py`

## Kịch bản parity

`parity/scenarios/w5/guests/POST-g-token-da-chuyen.yaml`, id `w5/guests/post-g-token-da-chuyen` (39 bước, `dev`; bữa tối 120000 chia bốn, ba link một nghĩa vụ):

- Khoá trong thân: `guest_b_reports_with_body_key`, `guest_b_repeats_body_key`, `guest_b_body_key_urn_spelling`, `guest_a_reuses_body_key_of_b` (409).
- Accept và hạn mức: `guest_a_reports_paid`, `guest_a_reports_from_browser` (303), `guest_a_accept_uppercase` (201), `guest_a_fourth_report` và `guest_a_fourth_report_from_browser` (403 JSON), `guest_a_obligation_of_link_b` (404 trước hạn mức), `guest_b_accept_list_with_html` (303), `guest_b_unknown_obligation`.
- Sau xác nhận: `owner_confirms_b_received`, `guest_b_reports_after_receipt` (201 `confirmed`), `guest_b_page_after_receipt`.
- Token lạ: `unknown_token_report`, `unknown_token_report_from_browser`.
- Header key: `guest_c_reports_with_header_key`, `guest_c_replays_header_key`, `guest_c_header_key_other_body`, `guest_c_header_key_from_browser`, `guest_c_header_key_from_browser_again`, `guest_c_header_key_with_actor_header` (scope khác, 403 hạn mức), `guest_c_empty_header_key`.

`validation-422.yaml` (`report_*`, 15 bước về form cho route này, cộng `report_token_*`). `prod-auth.yaml`: khoá của phiên chủ đợt phát lại cho chủ đợt, chạy lại cho phiên khác, cho khách không bearer và cho bearer rác (`owner_reports_for_a_with_key` … `junk_bearer_sends_owner_key`).

`parity/scenarios/w5/crossreplay/POST-g-token-da-chuyen.yaml` (27 bước): 201 lưu dưới header key ở một phía được phía kia phát lại, thân khác bị 422; khoá trong thân đọc chéo và 409 trên link khác; 303 không được lưu và 422 nhả khoá, nên phía kia chạy lại.

`parity/scenarios/w5/concurrency/POST-g-token-da-chuyen.yaml` (29 bước, mỗi người gửi một đợt): `report_twice_at_once` (2×201, hai hàng), `report_same_body_key_at_once` (2×201 cùng id, một hàng), `report_four_at_once` (3×201 + 403), `report_same_header_key_at_once` (201 + phát lại), `browser_reports_at_once` (2×303). Khoá dòng làm các kết quả tất định; ba lượt reference không có bước RACY.

Corpus 422 sinh tự động: hoãn, `carries ['pattern']`; bộ sinh cũng từ chối thân form (`form bodies are not probed`).

## Chưa phủ / lưu ý cho bản Go

- Link hết hạn (403 `active_capability` do `expires_at`) không phủ được: không có bước đồng hồ. Link bị thu hồi thì phủ (`POST-g-token-khong-phai-toi.yaml`).
- 409 `idempotency_request_in_flight` không có kịch bản: chỉ tới được khi request đầu chưa xong quá thời gian chờ, tức là lịch chạy.
- Bản Go phải đọc form đúng như FastAPI: giá trị cuối thắng, chuỗi rỗng không phải thiếu, multipart được nhận, content-type lạ thành form rỗng, `missing` liệt kê trước `extra_forbidden`, `loc` là `["body", "<tên>"]`.
- Thông điệp `uuid_parsing` đến từ pydantic-core (`invalid length: expected length 32 for simple format, found 0`, ``invalid character: found `k` at 1``); bản Go phải chép từng byte, và nhận cùng các cách viết UUID lax.
- Accept là chuỗi con phân biệt hoa thường. 303 dùng `location` tương đối và `content-length: 0`.
- Hạn mức 3 tính trên cả link; tra nghĩa vụ đi trước quyền; `active_capability` đi trước `report_budget_available`.
- Hai tầng idempotency độc lập: header (lưu cả câu trả lời, scope `anonymous` dùng chung cho mọi khách) và khoá trong thân (unique trên toàn bảng `payment_reports`).

## Lỗi Python (chỉ báo, không sửa)

- Token lạ trả JSON 404 tiếng Anh `guest_obligation_not_found` thay vì trang link hỏng. Khách bấm "Tôi đã chuyển" từ một link bị cắt đuôi sẽ thấy máy đọc. `test_every_route_that_can_refuse_an_unknown_token_answers_with_the_page` không chạm route này.
- Từ chối trên nhánh trình duyệt vẫn là JSON (403, 404, 409, 422).
- Accept so chuỗi con phân biệt hoa thường: `TEXT/HTML` ra JSON, còn một giá trị như `text/htmlx` lại ra 303.
- 303 không được middleware lưu, nên `Idempotency-Key` vô tác dụng trên nhánh trình duyệt: mỗi lần thử lại ghi thêm một báo và tiêu hạn mức.
- Scope `anonymous` dùng chung cho mọi khách: hai khách tình cờ dùng cùng một header key trên hai link khác nhau thì người sau bị 422 `idempotency_key_reuse`. Ở dev, `X-Actor-ID` đổi scope trên một route vốn không có actor.
- Vẫn nhận báo sau khi người nhận đã xác nhận đủ (201 `confirmed`). Có thể là chủ ý; chỉ ghi nhận.
