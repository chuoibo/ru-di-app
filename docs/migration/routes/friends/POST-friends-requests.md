# POST /friends/requests

friends · core · trạng thái trong bộ nhớ: không có

## Mục đích

F03: người gọi hỏi kết bạn một người khác. 201 nghĩa là **đã hỏi**, không bao giờ nghĩa là đã là bạn: dòng mới ở trạng thái `pending` và không cấp quyền gì cho tới khi người được hỏi trả lời qua `POST /friends/requests/{request_id}/respond`. Người hỏi là actor, không bao giờ là một trường trong thân.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: rỗng hoặc dài hơn 255 → 422; key đã dùng cho request khác → 422; đã có câu trả lời → replay (`services/api/app/api/idempotency.py:404-502`).
2. JSON hỏng → 422 `json_invalid`, **trước** xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu danh tính → 401, kể cả khi `addressee_id` sai dạng (`anonymous_bad_addressee` → 401, không phải 422). `dev`: `X-Actor-ID` không phải UUID → 422 `invalid_actor_id`; role lạ → 422 `invalid_actor_roles`. `prod`: bearer (`deps.py:93-107`, `services/api/app/api/service.py:4452-4473`), bỏ qua `X-Actor-*`.
4. Validate thân `FriendRequestCreate` → 422 dạng `{"detail":[...]}`.
5. `_require_permission("send_friend_request", actor, {"is_not_self": actor.id != addressee_id})` (`service.py:7012-7017`): bảng quyền đòi role `member` và predicate `is_not_self` (`services/api/app/domain/permissions.py:169`). Role được xét **trước** predicate (`permissions.py:673-677`): role rỗng gửi cho chính mình → `role_not_permitted` (`roles_empty_self`); chỉ `group_admin` → `role_not_permitted` (`roles_group_admin_only`). So sánh là giữa hai `UUID` nên chữ hoa/không gạch nối vẫn là "chính mình".
6. `get_person(addressee_id) is None` → 404 (`service.py:7018-7019`). Chạy **sau** quyền: role rỗng + người không tồn tại → 403 (`roles_empty_unknown_person`).
7. `get_friend_edge(actor, addressee)` (`services/api/app/api/repository.py:7403-7428`) lấy dòng **sống** mới nhất của cặp không thứ tự (bỏ qua `declined`), rồi domain `open_request` (`services/api/app/domain/friendship.py:138-167`): còn dòng `pending`, `accepted` hoặc `blocked` theo **bất kỳ chiều nào** → `REQUEST_NOT_OPEN` → 409 (`service.py:7021-7029`, `:7202-7206`). Một mã cho cả ba trạng thái (`BLOCKED_IS_SILENT`, `friendship.py:94-100`).
8. INSERT (`repository.py:7446-7469`); `IntegrityError` → `RepositoryConflict("FRIEND_EDGE_EXISTS")` → 409 cùng mã, cùng câu (`service.py:7031-7041`).

Nhánh 422 `self_edge` của domain (`service.py:7207-7208`, `friendship.py:126-129`) **không tới được** qua HTTP: `is_not_self` chặn trước ở bước 5.

## Đầu vào

- Không path param, không query.
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` (`text/plain` → 422 `model_attributes_type`).
- Thân `FriendRequestCreate` (`services/api/app/api/schemas.py:2084-2092`), `extra="forbid"` (`schemas.py:66-67`): đúng một trường `addressee_id: UUID`, bắt buộc. UUID "lax" của pydantic: chấp nhận chữ hoa và dạng không gạch nối (`asker_unknown_person_unhyphenated`, `asker_unknown_person_uppercase` đều tới được bước 6). `requester_id` trong thân → 422 `extra_forbidden`.

## Đầu ra

- **201** `FriendRequestResponse` (`schemas.py:2101-2116`, dựng ở `service.py:810-821`), thứ tự khoá: `id`, `requester_id`, `addressee_id`, `other_person_id`, `other_display_name`, `state`, `created_at`, `decided_at`.
  - `state` luôn `"pending"`; `decided_at` luôn `null`.
  - `other_person_id` là người **không phải** người đọc, tức addressee (`repository.py:7379-7401`). `other_display_name` đọc từ `people.display_name`; không có tên thì rơi về chuỗi UUID (`repository.py:2529-2548`). Người đã xoá tài khoản mang tên `Người dùng đã rời` (`services/api/app/domain/account_lifecycle.py:55`).
  - UUID luôn viết thường có gạch, dù thân gửi dạng nào.
- Datetime: `created_at` là `_now()` của Python (`datetime.now(UTC)`, `service.py:406-407`) truyền tay vào INSERT (`repository.py:7453-7458`); `server_default now()` của cột (`services/api/app/db/models.py:2187-2189`) không được dùng. Pydantic ghi `YYYY-MM-DDTHH:MM:SS.ffffffZ` (6 chữ số lẻ, `Z`); micro giây bằng 0 thì bỏ phần lẻ.
- Không có float.
- Replay idempotency: cùng 201 và cùng body, thêm `idempotency-replayed: true`; thân gửi lại với khoá đổi thứ tự/khoảng trắng vẫn replay (`idem_replay_reordered_spaces`) vì fingerprint chuẩn hoá JSON.
- Framework: 307 cho `/friends/requests/` (`location: http://<Host>/friends/requests`); 405 + `allow: POST` cho GET.

## Tác dụng phụ

- `friend_requests`: INSERT một dòng (`id` là `uuid4` phía Python, `state='pending'`, `decided_by_id` và `decided_at` null). Ràng buộc: CHECK `no_self_friendship`, CHECK `decided_state_matches_timestamp`, unique index một phần `uq_friend_edge_live` trên `least/greatest(requester_id, addressee_id)` với `state IN ('pending','accepted','blocked')`, FK `requester_id`/`addressee_id` → `people` (`models.py:2128-2175`).
- Đọc `people` (addressee, tên), `friend_requests` (dòng sống của cặp).
- Idempotency: có key thì đặt chỗ trong `idempotency_keys`, lưu 201, nhả key khi 4xx (`idempotency.py:504-546`). Scope là digest bearer, hoặc `X-Actor-ID` thô, hoặc `anonymous`.
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `PUT /people/me/interests` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_not_self` | `service.py:502-504`, `:7013-7017` |
| 404 | `person_not_found` | `Chưa có ai mang danh tính này.` | `service.py:7018-7019` |
| 409 | `request_not_open` | `Chưa gửi được lời mời này.` | `service.py:7202-7206` (domain), `:7035-7041` (INSERT) |
| 422 | `self_edge` | `Không tự kết bạn với chính mình được.` — không tới được | `service.py:7207-7208` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

422 framework đã đo: `json_invalid` (`loc: ["body", <byte>]`), `missing` (`["body","addressee_id"]`; `["body"]` khi thân rỗng), `uuid_type` (`null`, số), `uuid_parsing` (chuỗi sai, có `ctx.error`), `extra_forbidden`, `model_attributes_type` (mảng top-level, `text/plain`). Không có khoá `input` (`services/api/app/api/main.py:319-351`).

## Mã Python

- Route: `services/api/app/api/routes/friends.py:96-113` (`send_friend_request`)
- Service: `services/api/app/api/service.py:7002-7042`, `:7202-7211` (`_friend_refusal`), `:810-821` (`_wire_friend_edge`)
- Domain: `services/api/app/domain/friendship.py:100` (`BLOCKED_IS_SILENT`), `:114` (`_LIVE`), `:117-130` (`pair_key`), `:138-167` (`open_request`); `services/api/app/domain/permissions.py:169`
- Repository: `services/api/app/api/repository.py:2525-2527` (`get_person`), `:7403-7428` (`_edge_between`, `get_friend_edge`), `:7446-7469` (`open_friend_request`), `:7379-7401` (`_friend_edge`), `:2529-2548` (`_display_names`)
- Model: `services/api/app/db/models.py:2128-2192`

## Test đang phủ

- `services/api/tests/api/test_friends_routes.py`: `test_nobody_may_friend_themselves` (164), `test_asking_an_unknown_person_is_404` (170), `test_a_second_request_to_the_same_person_is_refused` (179), `test_blocking_refuses_a_later_request_the_same_way_a_duplicate_does` (189), `test_being_declined_once_does_not_bar_asking_again` (214)
- `services/api/tests/postgres/test_friend_requests_postgres.py`: `test_two_people_asking_each_other_at_once_produce_one_edge` (59), `test_the_index_also_refuses_a_duplicate_in_the_same_direction` (85), `test_a_declined_edge_frees_the_pair_for_a_new_request` (99), `test_a_blocked_edge_keeps_holding_the_pair` (127), `test_the_database_refuses_a_self_friendship` (146), `test_consent_is_required_over_real_http_against_this_database` (256)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py::test_a_friend_request_from_a_blocked_person_reads_like_a_duplicate` (118)
- `services/api/tests/domain/test_friendship.py`: 87, 93, 102, 108, 114, 176

## Kịch bản parity

`parity/scenarios/w2/friends/POST-friends-requests.yaml`, id `w2/friends/post-friends-requests` (52 bước, `dev`):

- Thứ tự từ chối: `anonymous_valid_body`, `anonymous_bad_addressee` (401 trước 422 thân), `anonymous_malformed_json` (422 trước 401), `actor_id_not_uuid`.
- Quyền: `asker_asks_self` (403 `is_not_self`), `roles_empty_self` (role trước predicate), `roles_empty_unknown_person` (403 trước 404), `roles_group_admin_only`, `roles_unknown` (422).
- 404: `asker_unknown_person`, `asker_unknown_person_unhyphenated`, `asker_unknown_person_uppercase`.
- Validate: `missing_addressee`, `addressee_null`, `addressee_number`, `addressee_not_uuid`, `extra_requester_id`, `top_level_array`, `empty_body`, `text_plain_content_type`.
- Vòng đời cặp: `asker_asks_target` (201), `asker_asks_target_again` và `target_asks_asker_back` (409 cả hai chiều), `target_declines` → `asker_asks_again_after_decline` (201), `target_accepts_second` → `asker_asks_friend` (409), `asker_asks_other` (201).
- Chặn hai chiều: `blocker_blocks_other` → `blocked_asks_blocker`, `blocker_asks_blocked` (409 cùng câu).
- Tài khoản đã xoá và actor chưa đăng ký: `leaver_deletes_account` → `asker_asks_deleted_person` (201), `deleted_person_asks_blocker` (201), `unregistered_actor_asks` (409).
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307).
- Idempotency: `idem_first`, `idem_replay`, `idem_replay_reordered_spaces`, `idem_reuse_different_body`, `idem_refusal_not_stored` (409) + `idem_same_key_after_refusal` (201), `idem_same_key_other_actor`, `idem_empty_key`.

`parity/scenarios/w2/friends/prod-auth.yaml`, id `w2/friends/prod-auth`: `anonymous_send` (`Missing bearer session`), `anonymous_malformed_json`, `junk_bearer_send` (`Session is not valid`), `actor_headers_ignored_send`, `owner_asks_mate`, `owner_empty_roles_header_ignored` (201 ở `prod`, còn `dev` là 403), `idem_owner_asks_leaver` + `idem_owner_replay` (scope bearer) + `idem_same_key_actor_header_scope` (không replay: `X-Actor-ID` là scope khác, rồi 401), `other_sends_after_sign_out` (401 sau `DELETE /sessions/current`).

## Chưa phủ / lưu ý cho bản Go

- **BUG (chỉ báo cáo):** gửi lời mời tới một tài khoản **đã xoá** vẫn 201 (`asker_asks_deleted_person`), dòng mang tên `Người dùng đã rời`. `get_person` trả cả dòng có `deleted_at` (`repository.py:2525-2527`) và `send_friend_request` không kiểm cờ đó, trong khi `register_person` (`service.py:1295-1304`) và `POST /friends/lookup` (`service.py:7188-7197`) đều coi tài khoản đã xoá là không tồn tại. Bản Go giữ nguyên cho tới khi có ADR.
- **Hành vi lạ (chỉ báo cáo):** actor `dev` chưa có dòng `people` nhận 409 `request_not_open` (`unregistered_actor_asks`): INSERT vi phạm FK `fk_friend_requests_requester`, và `open_friend_request` gộp **mọi** `IntegrityError` thành `FRIEND_EDGE_EXISTS` (`repository.py:7460-7468`). Bản Go nếu phân biệt lỗi FK với lỗi unique sẽ lệch bước này. Ở `prod` không tới được (phiên luôn có dòng `people`).
- Ở `dev`, tài khoản đã xoá vẫn gửi được (`deleted_person_asks_blocker`) vì `get_actor` không kiểm `people`; ở `prod` phiên đã bị thu hồi khi xoá.
- Harness không viết hoa hay bỏ gạch nối được một giá trị bind, nên "gửi chính id của mình ở dạng chữ hoa vẫn là 403 `is_not_self`" chỉ đọc từ mã (so sánh `UUID`), chưa đo.
- Đua hai người cùng hỏi nhau (nhánh INSERT → 409) và 409 in-flight cần request đồng thời; harness chạy tuần tự.
- Thứ tự `get_friend_edge` khi một cặp có nhiều dòng sống là không thể (index unique); khi có nhiều dòng `declined` thì chúng bị bỏ qua.
- DB lane so dòng `friend_requests` và `idempotency_keys`; `created_at` được đặt từ đồng hồ Python chứ không từ Postgres, bản Go phải ghi giá trị do ứng dụng cấp (hoặc chứng minh thứ tự hạng thời gian không đổi).
