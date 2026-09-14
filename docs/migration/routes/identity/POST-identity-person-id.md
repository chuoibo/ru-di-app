# POST /identity/person-id

identity · core · trạng thái trong bộ nhớ: `person_id_limit`

## Mục đích

Đổi một số di động Việt Nam thành `person_id` mà máy chủ dùng cho người đó: HMAC-SHA256 dưới khoá `MOBILE_PERSON_ID_KEY`, cắt 16 byte, ép version 8 và variant RFC 9562. Route không đọc và không ghi bảng nghiệp vụ nào; số điện thoại chỉ tồn tại trong một lần tính HMAC. Đây là cửa lấy id trước khi đăng nhập, nên không có danh tính và vì thế có limiter.

## Xác thực và quyền

- **Không có dependency nào**, kể cả `get_actor` và `get_repository` (`services/api/app/api/routes/identity.py:157`). Không 401, không 403 ở cả `dev` lẫn `prod`. `X-Actor-*` rác không được đọc (`not_json_junk_actor_headers` vẫn là lỗi thân, không phải 422 của header).
- Chế độ xác thực chỉ đổi **scope idempotency**: có `Authorization: Bearer x` thì scope là `bearer:<sha256(x)>` (`services/api/app/api/idempotency.py:327-335`), không thì là chuỗi `X-Actor-ID` thô, không nữa thì `anonymous` (`idempotency.py:447-449`). Bearer rác vẫn là một scope, route không kiểm phiên.

Thứ tự trong handler (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: rỗng hoặc dài hơn 255 → 422 trước routing (`idempotency.py:432-439`); key đã dùng cho request khác → 422; đã có câu trả lời → replay (`idempotency.py:466-502`). Ba nhánh này **không** tốn limiter.
2. Limiter `_limiter(request).allow(_caller(request))` (`identity.py:158-163`) → 429. Tốn một lượt cho **mọi** request tới handler, kể cả request bị từ chối sau đó.
3. `await request.json()` (`identity.py:165-170`): thân rỗng, JSON cắt cụt, byte không phải UTF-8 → 422 `invalid_body`. Không kiểm `Content-Type`: `text/plain` vẫn được parse.
4. `phone` phải là chuỗi trong một object (`identity.py:172-176`) → 422 `phone_required` (mảng ở top level, số, `null`, object lồng, thiếu trường).
5. `canonical_mobile` (`services/api/app/api/person_identity.py:138-163`) → 422 `phone_not_mobile`.
6. `read_key()` (`person_identity.py:113-135`) → 503 `identity_key_missing` nếu khoá thiếu hoặc ngắn hơn 32 ký tự.
7. `derive_person_id` (`person_identity.py:195-213`) → 200.

## Đầu vào

- Không path param, không query.
- Header: `Idempotency-Key` (1..255) tuỳ chọn. `Content-Type` bị bỏ qua.
- Thân đọc tay, **không có model pydantic** (`identity.py:1-37` giải thích: 422 của FastAPI vọng lại `input`). Chỉ đọc khoá `phone`; khoá thừa bị bỏ qua; khoá trùng thì `json.loads` lấy giá trị sau.
- Chuẩn hoá (`person_identity.py:96`, `:101`, `:138-163`): xoá mọi ký tự khớp `[\s.\-()]` (khoảng trắng Unicode, tab, chấm, gạch, ngoặc) ở bất kỳ đâu; bỏ tiền tố `+84`, hoặc `84`, hoặc `0`; phần còn lại phải khớp `^[35789]\d{8}$`. Cùng một số viết `0…`, `+84…`, `84…`, có ngoặc, chấm, gạch, tab đều ra cùng id (`mint_with_key` và `idem_same_key_other_spelling`). Chữ cái không bị xoá: chuỗi có chữ là `phone_not_mobile`.

## Đầu ra

- **200** (POST nhưng không 201) `PersonIdResponse` (`services/api/app/api/schemas.py:352-361`): đúng một khoá `person_id`. Không vọng lại số, kể cả dạng đã chuẩn hoá.
- `person_id` là UUID **version 8** chữ thường (`person_identity.py:209-213`), không phải v4: bộ chuẩn hoá của harness để nguyên literal, hai phía giống nhau vì dùng chung khoá trong một lần dựng stack.
- Không float, không datetime.
- Replay: cùng 200 và cùng body, thêm `idempotency-replayed: true` (`idempotency.py:585-599`).
- Framework: 307 cho `/identity/person-id/` (`location: http://<Host>/identity/person-id`); 405 + `allow: POST` cho GET và HEAD (HEAD không body).

## Tác dụng phụ

- Không đọc, không ghi bảng nghiệp vụ nào; không mở session DB.
- Có `Idempotency-Key` thì middleware đặt chỗ và lưu câu trả lời 200 vào `idempotency_keys`; 4xx/5xx thì nhả key (`idempotency.py:504-546`), nên cùng key gửi lại một request đúng sẽ chạy thật (`idem_refusal_landline` → `idem_same_key_other_spelling`).
- **Limiter trong bộ nhớ** `person_id_limit` trên `app.state` (`identity.py:97-110`): `FixedWindowLimit(20, 60.0)` (`identity.py:55-56`, `:64-94`). Cửa sổ cố định `int(time.monotonic() / 60)`, đếm theo `request.client.host` (địa chỉ socket, không đọc `X-Forwarded-For`, `identity.py:113-123`). Quá 10 000 cặp (caller, cửa sổ) thì xoá sạch bảng (`identity.py:61`, `:87-88`). Theo tiến trình: hai worker uvicorn cho gấp đôi, restart là quên. Limiter này **tách riêng** với `friend_lookup_limit` của `POST /friends/lookup`, và `POST /friends/lookup` không tiêu nó. Trong `scenarios/w2` chỉ file này gọi route; kịch bản `w2-limiter-bound/friends/POST-friends-lookup.yaml` gọi thêm hai lần để lấy id (bước có `bind`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |
| 429 | `rate_limited` | `Thử lại sau một phút. Máy chủ đang giới hạn số lần tra danh tính.` | `identity.py:158-163` |
| 422 | `invalid_body` | `Thân yêu cầu phải là JSON.` | `identity.py:165-170` |
| 422 | `phone_required` | `Thiếu trường phone, và phải là chuỗi.` | `identity.py:172-176` |
| 422 | `phone_not_mobile` | `Chưa đúng dạng số di động Việt Nam.` | `identity.py:178-186` |
| 503 | `identity_key_missing` | `Máy chủ chưa cấu hình khoá danh tính nên chưa đăng nhập được.` | `identity.py:188-199` |
| 500 | (không JSON) | `Internal Server Error` (`text/plain; charset=utf-8`) | xem "Chưa phủ" |

- Lỗi của middleware ghi bằng `json.dumps` mặc định nên **có khoảng trắng** sau `:` và `,` (`idempotency.py:602-618`); lỗi của route thì gọn (`services/api/app/api/main.py:297-316`).
- Không có 422 dạng `{"detail":[...]}` nào: route không có model.

## Mã Python

- Route: `services/api/app/api/routes/identity.py:126-201` (`mint_person_id`); limiter `:55-123`
- Chuẩn hoá và dẫn xuất: `services/api/app/api/person_identity.py:75-163` (`KEY_ENV_VAR`, `MIN_KEY_LENGTH`, `DOMAIN`, `_MOBILE`, `_SEPARATORS`, `read_key`, `canonical_mobile`), `:195-213` (`derive_person_id`)
- Schema: `services/api/app/api/schemas.py:352-361`
- Middleware: `services/api/app/api/idempotency.py:404-553`
- Đăng ký router: `services/api/app/api/main.py:222`

## Test đang phủ

- `services/api/tests/api/test_identity_route.py`: `test_a_number_mints_a_person_id` (50), `test_the_same_number_mints_the_same_id_twice` (59), `test_the_response_carries_no_trace_of_the_number` (65), `test_a_refused_number_is_not_repeated_back` (76), `test_a_number_sent_as_a_json_number_is_not_echoed` (84), `test_a_body_that_is_not_an_object_is_refused_without_echo` (103), `test_no_key_answers_503_and_never_mints` (110), `test_a_short_key_is_treated_as_no_key` (125), `test_the_key_never_appears_in_any_answer` (131), `test_the_route_is_rate_limited` (138), `test_the_limit_is_per_application_not_per_process` (154), `test_the_window_moves_on` (173)
- `services/api/tests/api/test_person_identity.py`: `test_one_number_however_spelled_reaches_one_id` (151), `test_two_numbers_never_reach_one_id` (170), `test_derived_ids_are_well_formed_custom_uuids` (177), `test_what_is_not_a_vietnamese_mobile_is_refused` (200), `test_no_key_refuses_rather_than_falling_back` (213), `test_a_short_key_is_refused` (225), `test_the_key_never_appears_in_the_refusal` (236), `test_env_example_declares_the_name_the_code_reads` (246), cùng ba ca brute-force 86-136

## Kịch bản parity

`parity/scenarios/w2/identity/POST-identity-person-id.yaml`, id `w2/identity/post-identity-person-id` (13 bước, `dev`, tiêu 6 lượt `person_id_limit` mỗi lần chạy):

- Đường vui và chuẩn hoá: `mint_with_key`, `idem_same_key_other_spelling` (`+84`, ngoặc, chấm, gạch ra cùng id).
- Từ chối của handler: `not_json_junk_actor_headers` (`invalid_body`, header actor rác không được đọc), `phone_as_number` (`phone_required`), `idem_refusal_landline` (`phone_not_mobile`, đầu số 2), `fullwidth_digits_crash` (500).
- Idempotency: `mint_with_key`, `mint_replay`, `mint_key_reuse_other_spelling` (cùng số, khác byte → reuse 422), `empty_idempotency_key` (persona khác), `idem_refusal_landline` + `idem_same_key_other_spelling` (từ chối không lưu), `idem_key_too_long` (256).
- Framework: `get_not_allowed`, `head_not_allowed` (405), `trailing_slash_redirects` (307).

`parity/scenarios/w2/identity/prod-auth.yaml`, id `w2/identity/prod-auth` (5 bước, `prod`, tiêu 1 lượt): `owner_mint_with_key`, `owner_replay` (scope là digest bearer), `owner_key_reuse_other_body`, `junk_bearer_empty_key`, `get_not_allowed`.

Số điện thoại trong kịch bản là dữ liệu mẫu, tách bằng escape JSON `\t` để không có dãy chữ số nào vào repo.

## Chưa phủ / lưu ý cho bản Go

- **Lane limiter tách riêng.** Kịch bản tiêu 6 lượt mỗi lần chạy, không bước nào có `bind`. Canary chạy mọi file `w2` một lần cho mỗi chế độ hỏng trên cùng tiến trình (khoảng 19 s mỗi lượt, cửa sổ cố định theo `monotonic`), nên các lượt sau có thể gặp 429: chúng chỉ thêm khác biệt vào các chế độ hỏng, còn chế độ `identity` chạy đầu tiên trong một cửa sổ mới (chờ hơn 60 s sau các lần `parity run`). Một bước có `bind` trên route này thì khác: 429 ở phía tham chiếu làm canary dừng với `INFRA`, nên kịch bản cần id dẫn xuất nằm ở `w2-limiter-bound`. 429, lăn cửa sổ, xoá bảng khi quá 10 000 cặp, và "đếm theo địa chỉ socket" chưa được so. Bản Go phải giữ: 20 lượt mỗi 60 s, cửa sổ cố định (không trượt), lượt bị từ chối **không** tăng bộ đếm, request tới handler rồi bị 422 **có** tăng, request bị middleware idempotency trả lời thì **không**.
- **BUG (chỉ báo cáo):** `\d` của `re` trong Python khớp chữ số Unicode, nên `03` + tám chữ số toàn chiều rộng qua được `_MOBILE`, rồi `canonical.encode("ascii")` (`person_identity.py:209`) ném `UnicodeEncodeError` và Starlette trả 500 thô (`fullwidth_digits_crash`). Bước này giữ nguyên hành vi đo được; bản Go muốn trả `phone_not_mobile` thì phải mở ADR và đổi kịch bản cùng lúc. Chữ số Unicode khác (Ả Rập, Devanagari) cùng lỗi.
- 503 `identity_key_missing` không tới được: stack parity luôn đặt khoá 44 ký tự.
- 409 in-flight cần hai request đồng thời cùng key.
- `json.loads` của Python chấp nhận `NaN`, `Infinity`, số rất lớn, BOM UTF-8 và khoá trùng (lấy giá trị sau); bộ giải mã Go mặc định từ chối hoặc xử lý khác. Chưa có bước nào cho các hình dạng này.
- `request.json()` không nhìn `Content-Type`; Go không được đòi `application/json`.
- Id là version 8: normalizer của harness không thay nó, nên một lệch về chữ hoa/thường hoặc gạch nối sẽ hiện ra ngay.
- Tách tiền tố theo thứ tự `+84`, `84`, `0` sau khi đã xoá dấu phân cách: `084…` chỉ bị bỏ `0`, phần `84…` còn lại dài 11 chữ số nên không khớp `_MOBILE` → `phone_not_mobile`. Bản Go phải tách đúng một lần theo cùng thứ tự.
