# POST /g/{token}/khong-phai-toi

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Người đọc bấm "Tôi không phải `<tên>`". Route lưu một audit event `guest_objection.not_me` và **thu hồi link** ngay, để link ngừng hiển thị số tiền (spec mục 8.2: khoản nợ vẫn còn, chỉ link chết). Rồi nó render trang xác nhận từ tên đã đọc **trước khi** thu hồi (`services/api/app/api/routes/guests.py:122-145`). Trả 200 HTML, không redirect.

## Xác thực và quyền

Không có actor; token là capability. Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`): khoá rỗng hoặc dài → 422; 200 được **lưu và phát lại** (`crossreplay`, `core_replays_python_not_me_key`); thân khác dưới cùng khoá, kể cả một form `x=1`, → 422 `idempotency_key_reuse`. Scope `anonymous`, `X-Actor-ID` thô, hoặc digest bearer.
2. Router: đuôi `/` → 307 tuyệt đối (`not_me_trailing_slash`); `GET` là route trang; phương thức khác → 405 `allow: GET`. Starlette chỉ nêu phương thức của route đầu tiên khớp path, dù path này còn nhận `POST`.
3. Validation token → 422. Route **không đọc thân**: thân bất kỳ bị bỏ qua (`guest_a_says_not_me_with_form`, `unknown_token_not_me_with_form`).
4. `not_me_view(token)` (`routes/guests.py:133`; `services/api/app/api/service.py:6734-6738`): token lạ → trang `guest_link_broken.html` 404 (`unknown_token_not_me`); link không `active` → 409 `LINK_NOT_ACTIVE` (`guest_a_says_not_me_again`).
5. `record_objection(token, "not_me", None, None)` (`service.py:6834-6916`):
   - `reason` là None nên không kiểm danh sách;
   - nạp phong bì lần hai trong cùng transaction (khoá đã giữ); nhánh `link_not_active` ở đây không tới được sau bước 4;
   - không có `obligation_id` nên không kiểm phạm vi;
   - hạn mức: `not_me` là loại tiêu hạn mức. `block` tìm theo `str(None)` nên luôn None, và lấy `objections_used` **của cả link** (mọi `not_me` + `wrong_amount`). `>= 3` → 429 `objection_rate_limited` `Too many objections on this obligation` (`service.py:6878-6908`; `GET-g-token-khong-phai-toi.yaml`, `guest_a_not_me_after_quota`).
6. `save_guest_objection` (`services/api/app/api/repository.py:7041-7077`) rồi render.

## Đầu vào

- Path `token`. Không thân, không header ngoài `Idempotency-Key`.

## Đầu ra

- **200** `text/html; charset=utf-8`, `guest_not_me.html` với `view` dựng tay: `claimed_person_display_name`, `recorded_by_display_name` đọc ở bước 4, `already_reported: True`, `can_object: False` (`routes/guests.py:134-145`). Mẫu đi nhánh "Đã ghi nhận" (`services/api/app/web/templates/guest_not_me.html:24-38`): "Bạn đã báo rằng đây không phải link của mình.", và hai lần tên người ghi.
- Phát lại: 200 cùng byte, `content-length`, `idempotency-replayed: true`, `content-type: text/html; charset=utf-8`.
- Ba header riêng tư trên mọi câu trả lời.

## Tác dụng phụ

Trong một transaction, khoá `guest_links ⋈ collection_envelopes ⋈ collection_batch_versions ⋈ collection_batches` `FOR UPDATE` từ bước 4:

- `guest_links.first_opened_at = now₁` nếu NULL;
- INSERT `audit_events`: `actor_id` NULL, `event_type` `guest_objection.not_me`, `aggregate_type` `guest_link`, `aggregate_id` = id link, `request_id` NULL, `event_data` `{"kind":"not_me","obligation_id":null,"reason":null}` (jsonb, Postgres tự xếp khoá), `occurred_at = now₂`;
- UPDATE `guest_links`: `status='revoked'`, `revoked_at = now₂` (`repository.py:7070-7077`). `now₂` là `_now()` gọi riêng lúc lưu, nên khác `now₁` vài micro giây;
- `idempotency_keys` khi có header: `response_status` 200, thân HTML, `response_media_type` `text/html; charset=utf-8`.

Sau đó (đo trong `POST-g-token-khong-phai-toi.yaml`):

- trang chính 200 với thẻ thu hồi;
- báo chuyển 403 `active_capability`, cả khi Accept có `text/html`;
- thắc mắc và xin cách tính 409 `link_not_active`, **trước** kiểm phạm vi nên cả nghĩa vụ của link khác cũng 409;
- trang sai số tiền 409 `no_open_obligation` (không có id) hoặc `LINK_NOT_ACTIVE` (có id);
- bảng thu của chủ đợt không đổi: "không phải tôi" không làm nghĩa vụ `disputed`.

Mọi từ chối rollback, không ghi gì.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 404 | trang `guest_link_broken.html` | — | `service.py:6726-6728`; `main.py:313-314` |
| 409 | `LINK_NOT_ACTIVE` | `Objection page is not renderable` | `service.py:6736-6738` |
| 429 | `objection_rate_limited` | `Too many objections on this obligation` | `service.py:6904-6908` |
| 409 | `link_not_active` | `This link is no longer open` (không tới được qua route này) | `service.py:6859-6861` |
| 422 | validation token; `invalid_idempotency_key`, `idempotency_key_reuse` | xem card `POST-g-token-da-chuyen` | `main.py:319-351`; `idempotency.py:404-553` |
| 409 | `idempotency_request_in_flight` | xem card `POST-g-token-da-chuyen` | `idempotency.py:404-553` |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:122-145`
- Service: `services/api/app/api/service.py:6725-6738`, `:6834-6916`
- Repository: `services/api/app/api/repository.py:6516-6722`, `:7041-7077`
- Hạn mức: `services/api/app/api/limits.py:26`, `:34`, `:38`
- Mẫu: `services/api/app/web/templates/guest_not_me.html`

## Test đang phủ

- `services/api/tests/api/test_guest_objections.py`: `TestNotMe.test_reporting_revokes_the_link` (60), `test_the_obligation_survives_a_revoked_link` (65); `TestAClosedLinkIsClosedInBothDirections.test_posting_to_a_revoked_link_is_refused` (643); `TestQuota` (244)
- `services/api/tests/api/test_guest_link_broken.py` (56)

Repository giả; không chứng minh khoá, `revoked_at` hay rollback.

## Kịch bản parity

`parity/scenarios/w5/guests/POST-g-token-khong-phai-toi.yaml`, id `w5/guests/post-g-token-khong-phai-toi` (32 bước, `dev`):

- `guest_a_opens_home`, `guest_a_says_not_me` (audit + thu hồi), `guest_a_says_not_me_again` (409), `guest_a_says_not_me_with_form` (409, thân bị bỏ qua);
- sau thu hồi: `guest_a_home_revoked`, `guest_a_reports_after_revoke`, `guest_a_reports_after_revoke_from_browser`, `guest_a_objects_after_revoke`, `guest_a_objects_other_link_after_revoke`, `guest_a_asks_evidence_after_revoke`, `guest_a_wrong_amount_default_after_revoke`, `guest_a_wrong_amount_after_revoke`, `owner_board_after_not_me`;
- hai lần thắc mắc vẫn cho "không phải tôi": `guest_b_objects_amount_1`, `guest_b_objects_amount_2`, `guest_b_says_not_me_after_two`, `guest_b_objects_after_not_me`;
- `unknown_token_not_me`, `unknown_token_not_me_with_form`, `not_me_trailing_slash`.

Nhánh 429: `GET-g-token-khong-phai-toi.yaml` (`guest_a_not_me_after_quota`). `prod-auth.yaml`: `mate_says_not_me_for_b` (phiên của một thành viên không đổi gì), `anonymous_not_me_b_again`.

`parity/scenarios/w5/crossreplay/POST-g-token-khong-phai-toi.yaml` (29 bước): trang 200 lưu ở một phía, phía kia phát lại; form dưới cùng khoá 422; không khoá thì 409; 303 của thắc mắc và xin cách tính không được lưu nên phía kia ghi event thứ hai; 422 nhả khoá.

`parity/scenarios/w5/concurrency/POST-g-token-khong-phai-toi.yaml` (18 bước): `not_me_twice_at_once` → 200 + 409 `LINK_NOT_ACTIVE`, một audit event; `not_me_same_header_key_at_once` → 200 + 200 phát lại.

Corpus 422 sinh tự động: hoãn, `carries ['pattern']`.

## Chưa phủ / lưu ý cho bản Go

- Thứ tự: tên phải được đọc **trước** khi thu hồi, và việc nạp đầu tiên giữ khoá dòng tới commit. Không có khoá thì hai lần bấm cùng lúc sẽ ghi hai event.
- Hạn mức của `not_me` là của cả link; detail vẫn nói "on this obligation".
- 200 HTML là câu trả lời duy nhất được middleware lưu trong nhóm route phản đối.
- 405 trên path có cả GET lẫn POST mang `allow: GET`, không phải `GET, POST`; bản Go liệt kê đủ phương thức sẽ lệch.
- Link hết hạn không phủ (không có bước đồng hồ).

## Lỗi Python (chỉ báo, không sửa)

- Ba lần thắc mắc số tiền của một nghĩa vụ chặn "Tôi không phải X" bằng 429, với detail nói về "this obligation" dù "không phải tôi" không gắn nghĩa vụ nào.
- POST trả 200 HTML thay vì 303: tải lại trang trong trình duyệt gửi lại POST và nhận JSON 409 `LINK_NOT_ACTIVE`.
- Mỗi request nạp phong bì hai lần (`not_me_view`, rồi `record_objection`), và `save_guest_objection` đọc lại link lần ba không khoá (`repository.py:7050-7054`), im lặng bỏ qua nếu không thấy.
