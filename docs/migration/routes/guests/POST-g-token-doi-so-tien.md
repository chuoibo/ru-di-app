# POST /g/{token}/doi-so-tien

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Người gửi thắc mắc số tiền của một nghĩa vụ (spec mục 8.2). Route lưu một audit event `guest_objection.wrong_amount` kèm lý do trong danh sách đóng. Không có cột trạng thái nào đổi: trang khách và bảng thu **suy ra** `disputed` từ event này. Thành công luôn là 303 về trang chính.

## Xác thực và quyền

Không có actor; token là capability. Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`): 303 **không bao giờ được lưu**, nên cùng khoá gửi lại sẽ ghi event thứ hai (`crossreplay`, `core_objects_same_key`); mọi từ chối nhả khoá.
2. Router: đuôi `/` → 307 tuyệt đối (`objection_trailing_slash`); `GET` là route trang; phương thức khác → 405 `allow: GET` (chỉ route đầu tiên khớp path, không có `POST`).
3. Validation: token, và hai tham số `Form()` thường (`services/api/app/api/routes/guests.py:171-181`). Thiếu → 422 `missing`, liệt kê theo thứ tự khai báo `obligation_id` rồi `reason`.
4. `uuid.UUID(obligation_id)` chạy **trong handler, trước service** (`routes/guests.py:178-180`). `ValueError` không được bắt → 500 `Internal Server Error` `text/plain; charset=utf-8`, `content-length: 21`, có ba header riêng tư (`services/api/app/api/guest_privacy.py:43-71`). uvicorn đóng kết nối TCP ngay sau câu trả lời này. 500 thắng cả token lạ lẫn link đã thu hồi (`guest_b_obligation_not_uuid`, `guest_b_obligation_empty`, `unknown_token_not_uuid`, `guest_b_objects_not_uuid_after_revoke`).
5. `record_objection(token, "wrong_amount", uuid, reason)` (`services/api/app/api/service.py:6834-6916`):
   1. `kind` hợp lệ (nhánh `unknown_objection` không tới được);
   2. `reason` ngoài danh sách đóng → 422 `unknown_reason`, **trước khi tra link**: token lạ với lý do sai là 422, không phải trang 404 (`unknown_token_unknown_reason`, `guest_b_unknown_reason_after_revoke`);
   3. `_objection_envelope` → trang link hỏng 404 (`unknown_token_objection`);
   4. link không `active` → 409 `link_not_active`, **trước** kiểm phạm vi (`guest_b_objects_other_link_after_revoke`);
   5. nghĩa vụ không có trong phong bì → 404 `unknown_obligation` (`guest_b_obligation_of_link_a`, `guest_b_unknown_obligation`);
   6. hạn mức **theo nghĩa vụ**: số `wrong_amount` của nghĩa vụ `>= 3` → 429 `objection_rate_limited` (`guest_a_fourth_objection`).
6. `save_guest_objection` → 303.

## Đầu vào

- Path `token`.
- Form, hai tham số `Form()` thường (không phải model, **không** `extra="forbid"`):
  - `obligation_id: str`, parse bằng `uuid.UUID` nên nhận cả dạng không gạch, chữ hoa, `{…}` và `urn:uuid:…`, rồi lưu dạng chuẩn (`guest_b_obligation_urn`);
  - `reason: str`, một trong `amount_too_high`, `did_not_join`, `already_paid`, `split_wrong`, `other` (`services/api/app/web/objection_view.py:57-63`).
- Cách đọc: urlencoded và multipart; content-type khác hoặc không có → cả hai `missing` (`objection_json_body`, `objection_text_plain`). Giá trị rỗng là `""`, không phải thiếu: `reason=` → 422 `unknown_reason` (`guest_b_reason_empty`), `obligation_id=` → 500. Lặp → giá trị cuối (`guest_b_reason_repeated_last_unknown`, `guest_b_reason_repeated_last_known_extra_field`). Trường lạ bị bỏ qua.
- `Accept` bị bỏ qua (`guest_a_objects_asking_for_json` vẫn 303).

## Đầu ra

- **303** `location: /g/<token>` tương đối, `content-length: 0`, không có `content-type` (`routes/guests.py:181`).
- Từ chối là JSON gọn `{"code","detail"}`, trừ trang 404 và 500 text/plain.
- Ba header riêng tư trên mọi câu trả lời.

## Tác dụng phụ

Khoá `FOR UPDATE` bốn bảng từ khi nạp phong bì tới commit. Ở 303:

- `guest_links.first_opened_at = now` nếu NULL;
- INSERT `audit_events`: `actor_id` NULL, `event_type` `guest_objection.wrong_amount`, `aggregate_type` `guest_link`, `aggregate_id` = id link, `request_id` NULL, `event_data` `{"kind":"wrong_amount","obligation_id":"<uuid chuẩn>","reason":"<lý do>"}`, `occurred_at` (`services/api/app/api/repository.py:7056-7068`).

Đọc lại:

- trang chính `disputed` và mất link "Số tiền không đúng" sau 3 lần (`guest_a_home_disputed`);
- bảng thu `disputed: true`, `disputed_reason` = lý do **đầu tiên** theo `occurred_at, id` (`owner_board_disputed`);
- không tiêu hạn mức báo chuyển hay xin cách tính (`guest_a_reports_after_quota`, `guest_a_asks_evidence_after_quota`).

Mọi từ chối rollback, không ghi gì. Không có `idempotency_keys` (303 không được lưu).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | (validation) | `{"detail":[…]}` | `main.py:319-351` |
| 500 | text/plain | `Internal Server Error` | `routes/guests.py:178-180`; `guest_privacy.py:43-71` |
| 422 | `unknown_reason` | `Unknown objection reason` | `service.py:6845-6851` |
| 422 | `unknown_objection` | `Unknown objection kind` (không tới được) | `service.py:6843-6844` |
| 404 | trang `guest_link_broken.html` | — | `service.py:6726-6728`; `main.py:313-314` |
| 409 | `link_not_active` | `This link is no longer open` | `service.py:6859-6861` |
| 404 | `unknown_obligation` | `No such obligation on this link` | `service.py:6872-6876` |
| 429 | `objection_rate_limited` | `Too many objections on this obligation` | `service.py:6902-6908` |
| 422 / 409 | idempotency | xem card `POST-g-token-da-chuyen` | `idempotency.py:404-553` |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:171-181`
- Service: `services/api/app/api/service.py:6834-6916`, `:6725-6732`
- Repository: `services/api/app/api/repository.py:7041-7077`; đọc lại ở `:6560-6698` và bảng thu `list_batch_obligations`
- Danh sách lý do: `services/api/app/web/objection_view.py:57-63`; hạn mức `services/api/app/api/limits.py:26`, `:34`

## Test đang phủ

- `services/api/tests/api/test_guest_objections.py`: `TestWrongAmount.test_submitting_records_the_reason` (139), `test_an_unknown_reason_is_refused` (152); `TestTheLinkIsTheOnlyAuthority` (215); `TestQuota.test_objecting_more_than_the_limit_is_refused` (244); `TestADisputeStopsExactlyOneObligation` (323, 348); `TestAReceiptCannotCloseAnArgument` (512, 541); `TestObjectingOnALinkWithTwoDebts.test_objecting_from_the_second_card_does_not_flag_the_first` (602); `TestAClosedLinkIsClosedInBothDirections` (643)
- `services/api/tests/postgres/test_repository_postgres.py::test_an_objection_disputes_an_outstanding_obligation_in_postgres` (746)

## Kịch bản parity

`parity/scenarios/w5/guests/POST-g-token-doi-so-tien.yaml`, id `w5/guests/post-g-token-doi-so-tien` (41 bước, `dev`):

- thành công và đọc lại: `guest_a_objects_amount`, `guest_a_home_disputed`, `owner_board_disputed`, `guest_a_objects_other_reason`, `guest_a_objects_asking_for_json`, `guest_a_fourth_objection` (429), `guest_a_asks_evidence_after_quota`, `guest_a_reports_after_quota`;
- lý do: `guest_b_reason_hostile` (`<script>`, `&`, nháy mã hoá phần trăm), `guest_b_reason_rtl_emoji`, `guest_b_reason_empty`, `guest_b_reason_missing`, `guest_b_reason_repeated_last_unknown`, `guest_b_reason_repeated_last_known_extra_field`;
- `obligation_id`: `guest_b_obligation_of_link_a`, `guest_b_unknown_obligation`, `guest_b_obligation_urn`, `guest_b_obligation_not_uuid` (500), `guest_b_obligation_empty` (500), `guest_b_obligation_missing`;
- token lạ: `unknown_token_objection`, `unknown_token_unknown_reason`, `unknown_token_not_uuid`;
- sau thu hồi: `guest_b_says_not_me`, `guest_b_objects_after_revoke`, `guest_b_objects_other_link_after_revoke`, `guest_b_objects_not_uuid_after_revoke`, `guest_b_unknown_reason_after_revoke`;
- `objection_trailing_slash`.

`validation-422.yaml`: `objection_token_*`, `objection_json_body`, `objection_text_plain`, `objection_multipart_missing_reason`. `prod-auth.yaml`: `third_objects_on_a`.

`parity/scenarios/w5/concurrency/POST-g-token-doi-so-tien.yaml` (21 bước): `objection_twice_at_once` (2×303, hai event), `objection_twice_more_at_once` (303 + 429), `objection_four_at_once` (3×303 + 429), `evidence_twice_at_once` (2×303). Khoá dòng làm các kết quả tất định.

`parity/scenarios/w5/crossreplay/POST-g-token-khong-phai-toi.yaml`: `python_objects_with_key` / `core_objects_same_key` (303 không lưu), `core_refused_objection_with_key` / `python_objection_same_key_valid` (422 nhả khoá).

Corpus 422 sinh tự động: hoãn, `carries ['pattern']`; bộ sinh cũng từ chối thân form.

## Chưa phủ / lưu ý cho bản Go

- Thứ tự phải giữ: validation → parse UUID (500) → lý do (422) → tra link (404 trang) → trạng thái link (409) → phạm vi (404) → hạn mức (429).
- Bản parity cần một 500 text/plain có ba header riêng tư ở đúng các ca không phải UUID. Proxy đứng trước uvicorn phải tắt keep-alive phía upstream, vì uvicorn đóng kết nối sau 500 do exception mà không gửi `Connection: close`.
- Tham số `Form()` thường: trường lạ bị bỏ qua, chuỗi rỗng không phải thiếu, giá trị cuối thắng.
- Đếm hạn mức phải sau khoá dòng; `event_data` lưu uuid dạng chuẩn, không phải giá trị thô.
- Link hết hạn không phủ (không có bước đồng hồ).

## Lỗi Python (chỉ báo, không sửa)

- `obligation_id` không phải UUID, hoặc rỗng, là 500 (kèm đóng kết nối) thay vì 422, và đi trước cả 404 lẫn 409.
- Lý do được kiểm trước link: token lạ với lý do sai ra 422 thay vì trang link hỏng.
- `reason=` rỗng là `unknown_reason` chứ không phải `missing`.
- Luôn 303 kể cả khi client xin JSON; từ chối luôn là JSON kể cả với trình duyệt.
- `Idempotency-Key` vô tác dụng: 303 không được lưu, nên thử lại ghi thêm event và tiêu hạn mức.
