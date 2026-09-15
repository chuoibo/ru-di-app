# POST /g/{token}/xin-cach-tinh

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Người gửi xin xem phần tính của mình cho một nghĩa vụ (spec mục 10.5). Chỉ hỏi thôi: route lưu một audit event `guest_objection.evidence_request`, không tiêu hạn mức phản đối, không làm nghĩa vụ `disputed`. Người ghi từ chối chia sẻ cũng không làm người hỏi thành người sai (`services/api/app/api/routes/guests.py:184-202`). Thành công là 303 tới trang sai số tiền của đúng nghĩa vụ đó.

## Xác thực và quyền

Không có actor; token là capability. Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`): 303 không được lưu, cùng khoá gửi lại sẽ ghi event thứ hai (`crossreplay`, `core_asks_evidence_same_key`).
2. Router: không có route GET trên path này, nên `GET` và `HEAD` → 405 `allow: POST` (`get_evidence`, `head_evidence`); đuôi `/` → 307 tuyệt đối (`evidence_trailing_slash`).
3. Validation: token, và một tham số `Form()` thường `obligation_id: str`. Thiếu → 422 `missing` (`guest_b_evidence_missing`, `guest_b_evidence_json_body`, `evidence_no_content_type`, `evidence_text_plain`).
4. `uuid.UUID(obligation_id)` trong handler (`routes/guests.py:196-198`): không phải UUID hoặc rỗng → 500 `Internal Server Error` text/plain có ba header riêng tư; uvicorn đóng kết nối (`guest_b_evidence_not_uuid`, `guest_b_evidence_empty`, `unknown_token_evidence_not_uuid`).
5. `record_objection(token, "evidence_request", uuid, None)` (`services/api/app/api/service.py:6834-6916`): token lạ → trang link hỏng 404 (`unknown_token_evidence`); link không `active` → 409 `link_not_active` (`guest_b_evidence_after_revoke`); nghĩa vụ ngoài phong bì → 404 `unknown_obligation` (`guest_b_evidence_obligation_of_a`, `guest_b_evidence_unknown_obligation`). **Không có hạn mức**: loại này không nằm trong `QUOTA_CONSUMING_OBJECTIONS` (`services/api/app/api/limits.py:34-38`).
6. `save_guest_objection` → 303.

## Đầu vào

- Path `token`.
- Form: `obligation_id: str` qua `uuid.UUID` (không gạch, chữ hoa, `{…}`, `urn:uuid:…` đều được nhận). Urlencoded và multipart (`guest_b_evidence_multipart`); trường lạ bị bỏ qua (`guest_b_evidence_extra_reason`); lặp → giá trị cuối (`guest_b_evidence_repeated`).
- `Accept` bị bỏ qua (`guest_a_asks_with_accept_json`).

## Đầu ra

- **303** với `location: /g/<token>/doi-so-tien?obligation_id=<giá trị form như đã gửi>`, `content-length: 0`, không có `content-type` (`routes/guests.py:199-202`). Starlette `RedirectResponse` bọc URL qua `quote(url, safe=":/%#?=@[]!$&'()*+,;")`: `urn:uuid:` giữ nguyên, `{`/`}` bị mã hoá phần trăm.
  - **Giá trị thô, không phải dạng chuẩn**: `obligation_id=urn:uuid:<id>` redirect tới `?obligation_id=urn:uuid:<id>`, và trang đó so chuỗi nên trả 409 `UNKNOWN_OBLIGATION` (`guest_a_asks_urn_spelling` → `guest_a_follows_urn_redirect`).
- Theo redirect với dạng chuẩn: trang sai số tiền hiện "Yêu cầu của bạn đã được lưu lại…" và không còn form xin cách tính (`guest_a_follows_redirect`).
- Từ chối là JSON gọn, trừ trang 404 và 500 text/plain. Ba header riêng tư luôn có.

## Tác dụng phụ

Khoá `FOR UPDATE` bốn bảng từ khi nạp phong bì tới commit. Ở 303:

- `guest_links.first_opened_at = now` nếu NULL;
- INSERT `audit_events`: `actor_id` NULL, `event_type` `guest_objection.evidence_request`, `aggregate_type` `guest_link`, `aggregate_id` = id link, `request_id` NULL, `event_data` `{"kind":"evidence_request","obligation_id":"<uuid chuẩn>","reason":null}`, `occurred_at` (`services/api/app/api/repository.py:7056-7068`).

**Không khử trùng**: mỗi lần bấm là một event mới (`guest_a_asks_again`; burst hai bản cùng lúc ghi hai event). Đọc lại chỉ ở `evidence_requested` của trang sai số tiền, theo nghĩa vụ (`repository.py:6571-6583`). Trang chính và bảng thu không đổi (`guest_a_home_after_evidence`, `owner_board_after_evidence`). Không có `idempotency_keys` (303 không được lưu). Mọi từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | (validation) | `{"detail":[…]}` | `main.py:319-351` |
| 500 | text/plain | `Internal Server Error` | `routes/guests.py:196-198`; `guest_privacy.py:43-71` |
| 404 | trang `guest_link_broken.html` | — | `service.py:6726-6728`; `main.py:313-314` |
| 409 | `link_not_active` | `This link is no longer open` | `service.py:6859-6861` |
| 404 | `unknown_obligation` | `No such obligation on this link` | `service.py:6872-6876` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` | Starlette |
| 422 / 409 | idempotency | xem card `POST-g-token-da-chuyen` | `idempotency.py:404-553` |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:184-202`
- Service: `services/api/app/api/service.py:6834-6916`
- Repository: `services/api/app/api/repository.py:7041-7077`, đọc lại `:6571-6583`
- View model đọc lại: `services/api/app/web/objection_view.py:128-129`

## Test đang phủ

- `services/api/tests/api/test_guest_objections.py`: `TestEvidenceRequest.test_asking_is_all_it_does` (195), `TestNoPageClaimsSomeoneWasTold.test_evidence_request_does_not_claim_anyone_was_asked` (181), `TestTheLinkIsTheOnlyAuthority.test_asking_for_evidence_is_scoped_the_same_way` (230), `TestQuota.test_asking_how_a_number_was_reached_does_not_spend_the_quota` (265), `TestAskingHowANumberWasReachedIsNotAnObjection` (398)

Repository giả.

## Kịch bản parity

`parity/scenarios/w5/guests/POST-g-token-xin-cach-tinh.yaml`, id `w5/guests/post-g-token-xin-cach-tinh` (36 bước, `dev`):

- `guest_a_asks_evidence`, `guest_a_follows_redirect`, `guest_a_asks_again`, `guest_a_asks_urn_spelling`, `guest_a_follows_urn_redirect` (409), `guest_a_home_after_evidence`, `owner_board_after_evidence`, `guest_a_asks_with_accept_json`;
- `guest_b_evidence_obligation_of_a`, `guest_b_evidence_unknown_obligation`, `guest_b_evidence_not_uuid`, `guest_b_evidence_empty`, `guest_b_evidence_missing`, `guest_b_evidence_extra_reason`, `guest_b_evidence_repeated`, `guest_b_evidence_json_body`, `guest_b_evidence_multipart`;
- `unknown_token_evidence`, `unknown_token_evidence_not_uuid`, `get_evidence`, `head_evidence`, `guest_b_says_not_me`, `guest_b_evidence_after_revoke`, `evidence_trailing_slash`.

Cũng gọi route này: `GET-g-token-doi-so-tien.yaml` (`guest_a_asks_evidence_second`), `POST-g-token-doi-so-tien.yaml` (`guest_a_asks_evidence_after_quota`), `POST-g-token-khong-phai-toi.yaml` (`guest_a_asks_evidence_after_revoke`), `validation-422.yaml` (`evidence_token_*`, `evidence_no_content_type`, `evidence_text_plain`), `crossreplay/POST-g-token-khong-phai-toi.yaml`, `concurrency/POST-g-token-doi-so-tien.yaml` (`evidence_twice_at_once`).

Corpus 422 sinh tự động: hoãn, `carries ['pattern']`; bộ sinh cũng từ chối thân form.

## Chưa phủ / lưu ý cho bản Go

- `location` phải lặp lại giá trị form thô qua đúng phép `quote` của Starlette, không phải uuid đã parse. Kể cả khi điều đó dẫn tới một trang 409.
- Không có hạn mức và không khử trùng.
- 500 cho giá trị không phải UUID, như `doi-so-tien`.
- Link hết hạn không phủ (không có bước đồng hồ).

## Lỗi Python (chỉ báo, không sửa)

- Redirect lặp lại giá trị thô trong khi event lưu dạng chuẩn. Mọi cách viết không chuẩn mà `uuid.UUID` nhận (chữ hoa, không gạch, `{…}`, `urn:uuid:`) dẫn tới một trang 409 `UNKNOWN_OBLIGATION`.
- `obligation_id` không phải UUID là 500 thay vì 422.
- Không khử trùng: bấm lại hoặc tải lại ghi thêm event, và `Idempotency-Key` không chặn được vì 303 không được lưu.
