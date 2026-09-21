# POST /friends/lookup

friends · core · trạng thái trong bộ nhớ: `friend_lookup_limit`

## Mục đích

F03 "Thêm bạn": người gọi đưa một số di động mình đã có, máy chủ trả `person_id` và `display_name` của người giữ số đó, hoặc 404. Không bao giờ trả số điện thoại: số chỉ sống đủ lâu để thành một HMAC, không có cột nào lưu nó. Route vẫn là một "oracle" có-tài-khoản-hay-không, nên đòi danh tính và có limiter riêng.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-502`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) là dependency: 401 / 422 header chạy **trước** handler và **không** tốn limiter. Route không có model thân, nên JSON hỏng **không** thắng 401 (`anonymous_malformed_json` → 401), khác các route có model pydantic.
3. Limiter `_lookup_limiter(request).allow(_caller(request))` (`services/api/app/api/routes/friends.py:214-219`) → 429.
4. `await request.json()` (`friends.py:221-224`) → 422 `invalid_body`.
5. `phone` phải là chuỗi trong một object (`friends.py:226-228`) → 422 `phone_required`.
6. `canonical_mobile` (`services/api/app/api/person_identity.py:138-163`) → 422 `phone_not_mobile`. Chạy **trước** quyền: role rỗng + số sai → 422 (`roles_empty_not_a_mobile`).
7. `read_key()` (`friends.py:236-248`) → 503 `identity_key_missing`.
8. `find_person_by_phone_identity` (`services/api/app/api/service.py:7150-7170`): `account_identities(provider='phone', subject=hex(derive_phone_digest))` nếu có, không thì id dẫn xuất `derive_person_id`.
9. `_require_permission("find_person_by_phone", actor, {})` (`service.py:7182`; role `member`, không predicate, `services/api/app/domain/permissions.py:214`) → 403 `role_not_permitted` (`roles_empty`).
10. `get_person`; `None`, **hoặc** `deleted_at` khác null, **hoặc** `discoverable_by_phone = false` → cùng một 404 (`service.py:7183-7197`).

Không có `is_not_self`: tra số của chính mình được. Chặn không được xét: người bị chặn vẫn tra ra người đã chặn mình (`blocked_seeker_still_finds_holder`).

## Đầu vào

- Không path param, không query (số không bao giờ được đi qua path/query, `friends.py:15-17`).
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` bị bỏ qua.
- Thân đọc tay, **không có model** (`friends.py:1-36`): chỉ khoá `phone`, khoá thừa bị bỏ qua. Chuẩn hoá giống `POST /identity/person-id`: xoá `[\s.\-()]`, bỏ `+84`/`84`/`0`, phần còn lại khớp `^[35789]\d{8}$` (`person_identity.py:96`, `:101`).

## Đầu ra

- **200** (POST nhưng không 201) `PersonMatchResponse` (`services/api/app/api/schemas.py:2133-2143`), thứ tự khoá `person_id`, `display_name`. Không có trường số điện thoại nào, và không cột nào có thể cung cấp nó.
- `person_id` ở `dev` là id dẫn xuất (version 8) vì người giữ số được đăng ký bằng `PUT /people/{id}` với id từ `POST /identity/person-id`; người đăng nhập bằng OTP sau khi xoá tài khoản cũ có id `uuid4` qua `account_identities`.
- Không float, không datetime.
- Replay idempotency: cùng 200, thêm `idempotency-replayed: true`.
- Framework: 307 cho `/friends/lookup/`; 405 + `allow: POST` cho GET.

## Tác dụng phụ

- Chỉ đọc: `account_identities`, `people` (`services/api/app/api/repository.py:4387-4395`, `:2525-2527`).
- Có `Idempotency-Key` thì middleware ghi `idempotency_keys` và lưu 200; 404/422/403 nhả key (`idempotency.py:504-546`).
- **Limiter trong bộ nhớ** `friend_lookup_limit` trên `app.state` (`friends.py:76-88`): `FixedWindowLimit(30, 60.0)` (`friends.py:72-73`; lớp ở `services/api/app/api/routes/identity.py:64-94`). Cửa sổ cố định `int(time.monotonic()/60)`, đếm theo địa chỉ socket (`friends.py:91-93`), riêng với `person_id_limit`. Mọi request tới handler đều tốn một lượt, kể cả 404/422/403/500; request bị `get_actor` hoặc middleware idempotency trả lời thì không.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 429 | `rate_limited` | `Thử lại sau một phút. Máy chủ đang giới hạn số lần tìm bạn.` | `friends.py:214-219` |
| 422 | `invalid_body` | `Thân yêu cầu phải là JSON.` | `friends.py:221-224` |
| 422 | `phone_required` | `Thiếu trường phone, và phải là chuỗi.` | `friends.py:226-228` |
| 422 | `phone_not_mobile` | `Chưa đúng dạng số di động Việt Nam.` | `friends.py:230-234` |
| 503 | `identity_key_missing` | `Máy chủ chưa cấu hình khoá danh tính nên chưa tìm bạn được.` | `friends.py:236-248` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:7182`, `:502-504` |
| 404 | `person_not_found` | `Chưa có ai dùng số này trong Rủ Đi.` | `service.py:7195-7197` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |
| 500 | (không JSON) | `Internal Server Error` | xem card `POST /identity/person-id` |

Mọi câu từ chối là câu cố định, không chèn đầu vào. 503 của route này khác câu với 503 của `POST /identity/person-id` ("…chưa tìm bạn được" so với "…chưa đăng nhập được").

## Mã Python

- Route: `services/api/app/api/routes/friends.py:171-255` (`find_person_by_phone`), limiter `:72-93`
- Service: `services/api/app/api/service.py:7150-7200` (`find_person_by_phone_identity`, `find_person_by_person_id`)
- Danh tính: `services/api/app/api/person_identity.py:113-163`, `:170-180` (`derive_phone_digest`), `:195-213` (`derive_person_id`)
- Repository: `services/api/app/api/repository.py:4387-4395`, `:2525-2527`
- Cột: `services/api/app/db/models.py:991-993` (`discoverable_by_phone`), `:999-1001` (`deleted_at`); đổi qua `PATCH /people/me` (`services/api/app/api/schemas.py:921`)
- Xoá tài khoản đặt `discoverable_by_phone=false` và `deleted_at`: `services/api/app/domain/account_lifecycle.py:253-274`

## Test đang phủ

- `services/api/tests/api/test_friends_routes.py`: `test_lookup_finds_the_person_who_holds_that_number` (288), `test_lookup_answer_contains_no_telephone_number` (305), `test_lookup_refusal_does_not_echo_the_number_back` (327), `test_lookup_refusal_for_a_non_mobile_does_not_echo_it` (345), `test_lookup_of_an_unregistered_number_says_nothing_about_it` (356), `test_lookup_never_reveals_a_number_for_a_person_found_by_id` (369), `test_lookup_without_an_identity_key_refuses_rather_than_falling_back` (394), `test_lookup_is_rate_limited` (414)
- `services/api/tests/api/test_friends_lookup_phone_shapes.py`: `test_no_body_shape_makes_the_refusal_echo_the_number` (77, 11 hình dạng), `test_a_body_that_is_not_json_is_refused_without_a_traceback` (96), `test_no_response_header_carries_the_number` (106)

## Kịch bản parity

Ba file: một ở lượt chính, hai ở làn limiter (lý do ở "Chưa phủ").

`parity/scenarios/w2/friends/POST-friends-lookup.yaml`, id `w2/friends/post-friends-lookup` (15 bước, `dev`; tiêu 8 lượt `friend_lookup_limit`, không bước nào có `bind` trên route bị giới hạn, nằm trong corpus canary):

- Trước handler: `anonymous_valid_body`, `anonymous_malformed_json` (401 thắng JSON hỏng), `actor_id_not_uuid` (422), `empty_idempotency_key` (422 middleware).
- 404 cho số không ai dùng: `idem_refusal_unregistered_number`, `unregistered_other_spelling` (`+84`, ngoặc, chấm, gạch), `unregistered_actor_lookup` (actor chưa có dòng `people` vẫn hỏi được).
- 403 và thứ tự: `roles_empty_unregistered_number` (403 trước khi tìm người), `roles_empty_not_a_mobile` (422 trước 403).
- 422 của handler: `not_json` (`invalid_body`), `phone_as_object` (`phone_required`), `idem_same_key_after_refusal` (`phone_not_mobile`: key đã được nhả nên request khác dưới cùng key chạy thật, không phải `idempotency_key_reuse`).
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307).

`parity/scenarios/w2/limiter/POST-friends-lookup-holders.yaml`, id `w2/limiter/post-friends-lookup-holders` (28 bước, `dev`, `lane: limiter`; tiêu 10 lượt `friend_lookup_limit` và 2 lượt `person_id_limit`), chạy ở làn limiter có lane DB, không nằm trong corpus canary:

- Chuẩn bị hai người giữ số (dữ liệu mẫu): `mint_holder_id`, `mint_leaver_id` (`POST /identity/person-id`, bind `class: uuid`), `register_holder`, `register_leaver` + `leaver_deletes_account` (gửi dưới `X-Actor-ID` là id dẫn xuất).
- 200: `seeker_finds_holder`, `seeker_finds_holder_other_spelling`, `idem_same_key_after_refusal`, `blocked_seeker_still_finds_holder`.
- 404: `idem_refusal_unregistered_number`, `seeker_hidden_number` (giữa `holder_turns_lookup_off` và `holder_turns_lookup_on`), `seeker_deleted_number`.
- 403/422 và thứ tự: `roles_empty` (403 với số có người giữ), `roles_empty_not_a_mobile`, `not_json`, `phone_as_object`.
- Idempotency: `seeker_finds_holder`, `idem_replay`, `idem_reuse_other_spelling`, `idem_refusal_unregistered_number` + `idem_same_key_after_refusal` (200), `empty_idempotency_key`.
- Framework: `get_not_allowed`, `trailing_slash_redirects`.

`parity/scenarios/w2/limiter/POST-friends-lookup-limit.yaml`, id `w2/limiter/post-friends-lookup-limit` (37 bước, `dev`, `lane: limiter`): 401 và `X-Actor-ID` hỏng không tốn lượt, middleware idempotency không tốn lượt, 30 lượt tới handler (`invalid_body`, `phone_required` với object lồng và mảng, `phone_not_mobile`, 404), lượt 31 là 429 `rate_limited` kể cả khi thân hỏng, 401 vẫn đi trước khi đã hết lượt, `POST /identity/person-id` vẫn 200 (limiter riêng).

`parity/scenarios/w2/friends/prod-auth.yaml`: `anonymous_lookup` (401 `Missing bearer session`), `owner_lookup_unregistered` (404 qua bearer, tiêu 1 lượt).

## Chưa phủ / lưu ý cho bản Go

- **Làn limiter.** 429, limiter đứng trước thân và sau dependency xác thực, bộ đếm riêng với `person_id_limit` được so ở làn limiter (ADR-0029 §2.4). Lăn cửa sổ giữa chừng và xoá bảng khi quá 10 000 cặp chưa được so; nhiều replica và địa chỉ khách thật sau proxy không chứng minh được ở đây.
- **Vì sao nhánh có người giữ số ở làn limiter.** Tìm ra một người cần id dẫn xuất của họ, và cửa HTTP duy nhất là `POST /identity/person-id` (`person_id_limit`, 20/60 s). Id đó phải `bind`; canary lặp corpus chính mỗi chế độ hỏng, nên một 429 ở bước có `bind` sẽ làm canary dừng với `INFRA`. Ở làn limiter, file bắt đầu trong một cửa sổ mới ở mỗi phía.
- **Trạng thái sống qua các lần chạy.** Hai số trong kịch bản cố định, nên người giữ số tồn tại qua các lần chạy trên cùng stack: lần hai `register_holder` là 200 thay vì 201, `register_leaver` là 404 (tài khoản đã xoá không đăng ký lại được), `leaver_deletes_account` vẫn 204. Kịch bản bật lại `discoverable_by_phone` trước khi kết thúc để lần sau vẫn tra ra. Hai phía dùng chung lịch sử nên vẫn so được; harness không sinh được số điện thoại mới theo nonce.
- Nhánh `account_identities` (người đăng nhập OTP có id khác id dẫn xuất) chưa phủ: tạo nó cần luồng OTP, mà nhịp 60 s theo số điện thoại làm các lượt canary liên tiếp không tất định.
- 500 với chữ số toàn chiều rộng: cùng lỗi `canonical.encode("ascii")` như `POST /identity/person-id` (`person_identity.py:179`); bước đó chỉ nằm trong kịch bản identity để không tốn thêm lượt ở đây.
- 503 không tới được (stack luôn có khoá). 409 in-flight cần request đồng thời.
- Không kiểm chặn và không kiểm `is_not_self` là hành vi đo được; đổi thì mở ADR.
- `phone_required` cho object lồng, mảng chữ số, bool: bản Go không được dùng model có thông báo lỗi vọng lại đầu vào.
