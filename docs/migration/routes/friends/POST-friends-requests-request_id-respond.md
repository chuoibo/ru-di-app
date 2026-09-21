# POST /friends/requests/{request_id}/respond

friends · core · trạng thái trong bộ nhớ: không có

## Mục đích

F04: trả lời một lời mời kết bạn bằng `accept`, `decline` hoặc `block`. Chỉ người được hỏi mới được chấp nhận hoặc từ chối; chặn là câu trả lời duy nhất mà **cả hai bên** được đưa ra, kể cả từ một tình bạn đã `accepted`. Tình bạn chỉ tồn tại khi dòng ở `accepted`.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-502`).
2. JSON hỏng → 422 `json_invalid`, trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 thắng cả path không phải UUID (`anonymous_non_uuid_request`). `dev`: `X-Actor-Contexts` sai → 422 `invalid_actor_contexts` (`actor_contexts_invalid`).
4. Path `request_id` **và** thân được validate cùng lúc: lỗi của cả hai gom vào **một** mảng 422, lỗi path trước (`stranger_non_uuid_and_bad_decision`).
5. `get_friend_request(request_id, actor.id)` (`services/api/app/api/repository.py:7430-7444`): không có dòng, **hoặc** người gọi không phải một trong hai bên → `None` → 404 (`services/api/app/api/service.py:7054-7056`). Bước này chạy **trước** quyền: người không có role hỏi id lạ vẫn nhận 404 (`roles_empty_unknown_request`), người lạ hỏi id thật cũng 404 (`stranger_real_request`).
6. `_require_permission("respond_to_friend_request", actor, {"is_invitee": ...})` (`service.py:7058-7070`; role `member`, `services/api/app/domain/permissions.py:180`). Với `block`, `is_invitee` = "là một trong hai bên"; với `accept`/`decline`, `is_invitee` = "là người được hỏi". Role thiếu → `role_not_permitted` (`roles_empty_real_request`); người hỏi tự chấp nhận hoặc từ chối → `is_invitee` (`asker_accepts_own_request`, `asker_declines_own_request`).
7. Domain `decide` (`services/api/app/domain/friendship.py:170-214`) trên dòng đã đọc: `block` khi đã `blocked` → 409 `already_blocked`; `accept`/`decline` khi không `pending` → 409 `not_pending` (`service.py:7072-7083`, `:7211`).
8. `decide_friend_request` (`repository.py:7471-7537`): `SELECT … FOR UPDATE`, chạy lại `decide` trên dòng mới nhất (mã domain → 409 cùng mã), rồi UPDATE `state`, `decided_at`, `decided_by_id`. `IntegrityError` của `uq_friend_edge_live` → `FRIEND_EDGE_EXISTS` → 409 `request_not_open` với câu **khác** câu của `POST /friends/requests` (`service.py:7101-7111`). Tới được không cần đồng thời: chặn một dòng `declined` cũ trong khi cặp đang có một dòng `pending` mới (`idem_refusal_block_stale_declined`).

Không tới được qua HTTP: `ONLY_ADDRESSEE_MAY_ANSWER` và `NOT_A_PARTY` của domain (403 với detail là mã viết thường, `service.py:7209-7210`) vì bước 5 và 6 chặn trước; `record is None` sau khi khoá (`service.py:7113-7114`) cần dòng bị xoá giữa hai câu lệnh.

## Đầu vào

- Path `request_id: UUID` (lax: chữ hoa, không gạch nối đều được, `stranger_unhyphenated_unknown_request`).
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type`.
- Thân `FriendRequestDecision` (`services/api/app/api/schemas.py:2095-2098`), `extra="forbid"`: `decision: Literal["accept", "decline", "block"]`, bắt buộc, phân biệt hoa thường (`decision_uppercase` → `literal_error`).

## Đầu ra

- **200** `FriendRequestResponse` (`schemas.py:2101-2116`), thứ tự khoá `id`, `requester_id`, `addressee_id`, `other_person_id`, `other_display_name`, `state`, `created_at`, `decided_at`.
  - `other_person_id`/`other_display_name` định hướng theo **người trả lời** (`repository.py:7537`, `:7379-7401`): người được hỏi chấp nhận thì thấy người hỏi; người hỏi chặn thì thấy người được hỏi (`asker_blocks_friendship`).
  - `state` là trạng thái mới: `accepted`, `declined` hoặc `blocked`.
  - `created_at` giữ nguyên; `decided_at` là `_now()` Python (`service.py:7090`, `:406-407`), ghi đè mỗi lần quyết định (chặn sau khi chấp nhận đổi `decided_at`). Định dạng `…ffffffZ`.
- Không float.
- Replay: cùng 200 và cùng body **đã lưu**, dù dòng đã đổi trạng thái sau đó; thêm `idempotency-replayed: true`.
- Framework: 307 cho `/respond/` (`location` giữ id); 405 + `allow: POST` cho GET.

## Tác dụng phụ

- `friend_requests`: một UPDATE (`state`, `decided_at`, `decided_by_id`) dưới khoá dòng. Không INSERT dòng mới, kể cả khi chặn.
- CHECK `decided_state_matches_timestamp` và index `uq_friend_edge_live` (`services/api/app/db/models.py:2137-2158`) có hiệu lực trên UPDATE.
- Idempotency: có key thì lưu 200, nhả key khi 4xx (`idempotency.py:504-546`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 404 | `friend_request_not_found` | `Không có lời mời này.` | `service.py:7055-7056`, `:7113-7114` |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_invitee` | `service.py:502-504` |
| 409 | `not_pending` | `Lời mời không ở trạng thái đó.` | `service.py:7211` |
| 409 | `already_blocked` | `Lời mời không ở trạng thái đó.` | `service.py:7211` |
| 409 | `request_not_open` | `Chưa trả lời được lời mời này.` | `service.py:7101-7111` |
| 403 | `permission_denied` | `only_addressee_may_answer` / `not_a_party` — không tới được | `service.py:7209-7210` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

422 framework đã đo: `uuid_parsing` (path, có `ctx.error`), `literal_error` (`ctx.expected` là `'accept', 'decline' or 'block'`), `missing` (`["body","decision"]`; `["body"]` khi thân rỗng), `extra_forbidden`, `json_invalid`.

## Mã Python

- Route: `services/api/app/api/routes/friends.py:116-132`
- Service: `services/api/app/api/service.py:7044-7115` (`respond_to_friend_request`), `:7202-7211` (`_friend_refusal`), `:810-821`
- Domain: `services/api/app/domain/friendship.py:86-91` (`Decision`), `:170-214` (`decide`); `services/api/app/domain/permissions.py:180`
- Repository: `services/api/app/api/repository.py:7430-7444` (`get_friend_request`), `:7471-7537` (`decide_friend_request`)
- Model: `services/api/app/db/models.py:2128-2192`

## Test đang phủ

- `services/api/tests/api/test_friends_routes.py`: `test_requester_cannot_accept_their_own_request_over_http` (70), `test_addressee_accepting_is_what_creates_the_friendship` (95), `test_declining_does_not_create_a_friendship` (134), `test_a_stranger_cannot_answer_a_request_between_two_other_people` (150), `test_either_party_may_block_an_accepted_friendship` (225)
- `services/api/tests/postgres/test_friend_decide_conflict_http_postgres.py`: `test_blocking_a_stale_declined_notice_answers_409_not_500` (100), `test_the_live_edge_is_untouched_after_that_refusal` (159)
- `services/api/tests/postgres/test_friend_consent_races_postgres.py`: `test_a_block_survives_an_accept_that_was_decided_on_a_stale_read` (89), `test_blocking_a_stale_declined_request_is_refused_not_a_crash` (179)
- `services/api/tests/postgres/test_friend_requests_postgres.py`: `test_an_answered_row_must_carry_a_decision_time` (161), `test_the_accepting_party_is_recorded_on_the_row` (220), `test_a_stranger_reading_a_request_by_id_gets_nothing` (241)
- `services/api/tests/domain/test_friendship.py`: 36, 49, 56, 63, 134, 144, 150, 157

## Kịch bản parity

`parity/scenarios/w2/friends/POST-friends-requests-request_id-respond.yaml`, id `w2/friends/post-friends-requests-request_id-respond` (46 bước, `dev`):

- Thứ tự từ chối: `anonymous_unknown_request`, `anonymous_non_uuid_request` (401), `anonymous_malformed_json` (422 trước 401), `stranger_non_uuid_request`, `stranger_non_uuid_and_bad_decision` (hai lỗi một mảng), `actor_contexts_invalid`.
- 404: `stranger_unknown_request`, `stranger_real_request` (không phải một bên), `stranger_unhyphenated_unknown_request`, `roles_empty_unknown_request` (404 trước 403), `request_of_deleted_person_is_gone` (dòng bị xoá cùng tài khoản).
- 403: `asker_accepts_own_request`, `asker_declines_own_request` (`is_invitee`), `roles_empty_real_request` (`role_not_permitted`).
- Validate: `decision_uppercase`, `decision_missing`, `decision_extra_field`, `empty_body`.
- Chuyển trạng thái: `idem_answerer_accepts` (200 accepted), `accept_again_not_pending`, `decline_after_accept_not_pending` (409), `asker_blocks_friendship` (người hỏi chặn từ accepted), `answerer_blocks_again` (409 `already_blocked`), `answerer_accepts_blocked` (409 `not_pending`), `asker_blocks_own_pending` (200), `asker_unblocks_leaver` → `leaver_accepts_after_unblock` (409: gỡ chặn trả về `declined`).
- Dòng `declined` cũ: `second_answerer_declines` → `second_asker_asks_again` → `idem_refusal_block_stale_declined` (409 `request_not_open`, câu "Chưa trả lời được…") → `accept_declined_not_pending`.
- Idempotency: `idem_answerer_accepts`, `idem_replay`, `idem_reuse_different_decision`, `idem_refusal_block_stale_declined` (409 không lưu) + `idem_same_key_after_refusal` (cùng key, request khác → chạy thật).
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307).

`parity/scenarios/w2/friends/prod-auth.yaml`: `actor_headers_ignored_respond` (401 dù có `X-Actor-ID`), `owner_accepts_own_request` (403 `is_invitee` qua bearer), `mate_accepts` (200).

## Chưa phủ / lưu ý cho bản Go

- Hai câu khác nhau cho cùng mã `request_not_open`: `Chưa gửi được lời mời này.` ở `POST /friends/requests` và `Chưa trả lời được lời mời này.` ở route này. Hai câu 409 `not_pending` và `already_blocked` giống hệt nhau.
- Nhánh đua (đọc cũ rồi khoá thấy trạng thái khác, `repository.py:7493-7521`) cho cùng mã với nhánh đọc; harness chạy tuần tự nên chỉ đo được nhánh đọc. Bản Go phải khoá dòng và kiểm lại trên dòng đã khoá, không ghi đè mù.
- 403 `only_addressee_may_answer`/`not_a_party` không tới được; nếu bản Go đảo thứ tự quyền và domain thì chúng sẽ lộ ra.
- Harness không viết hoa được id đã bind, nên "path chữ hoa trỏ tới một lời mời thật" chưa đo (id lạ viết không gạch nối thì đã đo: 404).
- Ở `dev`, `get_actor` không kiểm `people`: tài khoản đã xoá vẫn gọi được route, nhưng mọi dòng của nó đã bị `erase_person` xoá (`repository.py:4829-4835`) nên luôn 404.
- 409 in-flight cần hai request đồng thời.
