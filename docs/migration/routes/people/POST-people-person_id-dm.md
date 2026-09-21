# POST /people/{person_id}/dm

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Mở, hoặc tìm lại, cuộc trò chuyện hai người của người gọi với một người bạn (ADR-0021 §2.5). Pair là một `contexts` có `kind='pair'`, `display_name` rỗng và `pair_key` duy nhất; tin nhắn, ảnh, dấu đọc, theme, sổ đôi dùng lại nguyên. 201 khi vừa tạo, 200 khi đã có; thân là một hàng của `GET /people/me/contexts`. Mọi từ chối là cùng một 404 với cùng một câu, trừ nhắn cho chính mình (422).

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:222-247`, `services/api/app/api/service.py:1493-1553`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): khoá rỗng 422 trước mọi thứ (`anonymous_empty_idempotency_key`); khoá đã xong phát lại 200 hoặc 201 như đã lưu; khoá cho người khác → 422 reuse (`owner_key_other_person`).
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 (trước 422 của path, `anonymous_non_uuid`); 422 `invalid_actor_id` / `invalid_actor_roles`.
4. Path `person_id`: UUID → 422 `uuid_parsing`. Thân không khai báo, bị bỏ qua (`owner_opens_friend_with_json_body`).
5. `person_id == actor.id` → 422 `self_direct_message` `Không thể nhắn riêng với chính mình.` (`service.py:1506-1509`), **trước** vai trò và trước khi tra người: chưa đăng ký hay không có vai trò cũng 422 (`owner_opens_self_unregistered`, `owner_opens_self_without_roles`).
6. `are_friends(actor, person)` (`services/api/app/api/repository.py:4077-4097`: có hàng `accepted` theo một trong hai chiều).
7. `_require_permission("open_direct_message", {"is_friend": …})` (`services/api/app/domain/permissions.py:347`); **mọi** 403 (thiếu vai trò, không phải bạn) đổi thành 404 `person_not_found` `Chưa thể nhắn riêng với người này.` (`service.py:1510-1516`; `owner_opens_friend_without_roles`, `owner_opens_friend_as_guest`, `owner_opens_stranger`, `owner_opens_pending`, `pending_opens_owner`, `owner_opens_unknown_id`, `owner_opens_unregistered_friend`).
8. `get_person(person)`; `direct.can_open(is_friend, other_exists = hàng có và deleted_at NULL)` (`services/api/app/domain/direct.py:66-77`) hoặc `_is_blocked_with` (`service.py:2138-2148`) → cùng 404 (`service.py:1517-1527`). Với dữ liệu tạo qua HTTP hai nhánh này không còn gì để chặn thêm: chặn đã đổi cạnh `accepted` thành `blocked`, xoá tài khoản đã xoá cạnh (`owner_opens_blocker_after_block`, `blocker_opens_owner_after_block`, `owner_opens_leaver_after_erasure`). Riêng dev: người **gọi** đã xoá tài khoản có thể kết bạn lại, và người kia mở DM với họ thì `other.deleted_at` chặn ở đây (`owner_opens_leaver_after_refriend`).
9. `pair_key = "<id nhỏ>:<id lớn>"` (so chuỗi, `services/api/app/domain/direct.py:46-55`, `friendship.py:117-130`), `get_pair_context(key)` (`repository.py:2618-2622`).
10. Chưa có → `create_pair_context` (`repository.py:2624-2672`) trong savepoint; `IntegrityError` trên `uq_contexts_pair_key` → `PAIR_EXISTS` → đọc lại; vẫn không có → 409 `pair_exists` `Cuộc trò chuyện vừa được mở ở nơi khác; thử lại.` (`service.py:1534-1547`, không tới được bằng kịch bản tuần tự hay đồng thời).
11. `list_person_context_summaries(actor)` (`repository.py:3648-3809`) rồi tìm hàng có id của pair; không có → 404 `context_not_found` `Context does not exist` (`service.py:1548-1552`). Tới được ở dev: người gọi đã xoá tài khoản (membership `left`) kết bạn lại và mở pair cũ (`leaver_opens_owner_after_refriend`).

## Đầu vào

- Path `person_id`: UUID. Không query, không thân.

## Đầu ra

**201** (vừa tạo) hoặc **200** (đã có), thân `ContextSummary` (`services/api/app/api/schemas.py:767-794`, dựng ở `service.py:443-473`), thứ tự khoá:

`id`, `display_name` (tên hiện tại của người kia; `Thành viên` khi không có người kia), `member_count` (membership `active`), `my_role` (`member`), `my_state` (`active`), `membership_id`, `joined_at`, `last_message` (null hoặc `{id, kind, preview, author_id, author_display_name, created_at}`), `unread_count`, `theme` (`mac-dinh`), `kind` (`pair`), `counterpart` (`{id, display_name}` hoặc null), `unavailable`.

`unavailable` luôn `false` ở route này: chỉ `GET /people/me/contexts` tính nó (`service.py:4395-4411`). Thân 201 và 200 giống nhau byte-cho-byte khi không có gì đổi (`owner_opens_friend`, `owner_opens_friend_again`); phía người kia thấy tên của người gọi (`friend_opens_owner`).

## Tác dụng phụ

Chỉ khi tạo, trong một savepoint: `contexts(display_name='', created_by_id=actor, kind='pair', pair_key, theme mặc định)` và hai `memberships(state='active', role='member', origin='named', invited_by_id=actor, joined_at=now)` cho hai người. Mở lại không ghi gì. Sau khi gỡ chặn và kết bạn lại, cùng pair cũ được trả 200 (`owner_opens_blocker_after_refriend`).

## Idempotency

Tự nhiên idempotent (200 khi đã có). Khoá header: 201 lưu và phát lại là 201 dù pair đã có (`crossreplay/POST-people-person_id-dm.yaml`). Ba lần mở đồng thời: một 201, hai 200 (`concurrency/POST-people-person_id-dm.yaml`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 422 | (validation) | `uuid_parsing` `["path","person_id"]` | `main.py:319-351` |
| 422 | `self_direct_message` | `Không thể nhắn riêng với chính mình.` | `service.py:1506-1509` |
| 404 | `person_not_found` | `Chưa thể nhắn riêng với người này.` | `service.py:1510-1527` |
| 404 | `context_not_found` | `Context does not exist` | `service.py:1548-1552` |
| 409 | `pair_exists` | `Cuộc trò chuyện vừa được mở ở nơi khác; thử lại.` (không đo được) | `service.py:1541-1547` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:222-247`
- Service: `services/api/app/api/service.py:1493-1553`, `:443-473` (`_context_summary`), `:2138-2148`
- Repository: `services/api/app/api/repository.py:2618-2672`, `:3648-3827`, `:4077-4097`
- Domain: `services/api/app/domain/direct.py`, `services/api/app/domain/friendship.py:117-130`
- Quyền: `services/api/app/domain/permissions.py:347`

## Test đang phủ

- `services/api/tests/api/test_direct_messages.py`: `test_two_friends_get_one_pair_whichever_of_them_opens_it` (62), `test_every_refusal_is_the_same_404_with_the_same_body` (88), `test_writing_to_oneself_is_a_422_not_a_404` (111), `test_the_pair_appears_in_both_conversation_lists_named_after_the_other` (120), `test_a_blocked_pair_says_so_on_the_row_for_both_people` (237)
- `services/api/tests/postgres/test_direct_message_postgres.py`: `test_the_database_keeps_one_pair_per_two_people` (67), `test_create_pair_context_writes_two_active_memberships_in_one_savepoint` (95), `test_two_friends_share_one_pair_over_http_and_outsiders_get_the_one_404` (133)

## Kịch bản parity

`parity/scenarios/w10/people/POST-people-person_id-dm.yaml`, id `w10/people/post-people-person_id-dm` (64 bước, `dev`):

- Thứ tự: `anonymous_opens`, `anonymous_non_uuid`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid`, `owner_opens_self_unregistered` (422 trước mọi tra cứu), `owner_opens_unregistered_friend` (404).
- Một 404 cho mọi lý do: `owner_opens_self`, `owner_opens_self_without_roles` (vẫn 422), `owner_opens_unknown_id`, `owner_opens_stranger`, `owner_opens_pending`, `pending_opens_owner`, `owner_opens_friend_without_roles`, `owner_opens_friend_as_guest`, `stranger_opens_pair_id_as_person`.
- Tạo và tìm lại: `owner_lists_before_pair`, `owner_opens_friend` (201, một `contexts` và hai `memberships` trong delta), `owner_opens_friend_again` (200 cùng thân), `friend_opens_owner` (200, tên của `owner`), `owner_opens_friend_with_json_body`, `owner_opens_friend_with_key`, `owner_replays_key`, `owner_key_other_person`, `owner_lists_with_pair`, `friend_lists_with_pair`, `friend_writes_in_pair`, `friend_renames_self`, `owner_opens_friend_after_message_and_rename` (`last_message`, `unread_count` 1, tên mới).
- Chặn: `blocker_opens_owner`, `blocker_blocks_owner`, `owner_opens_blocker_after_block`, `blocker_opens_owner_after_block` (404), `owner_lists_blocked_pair`, `blocker_unblocks_owner`, `owner_opens_blocker_after_unblock` (404), `owner_asks_blocker_again`, `blocker_accepts_again`, `owner_opens_blocker_after_refriend` (200, cùng pair cũ), `blocker_lists_after_refriend`.
- Xoá tài khoản: `owner_opens_leaver`, `leaver_ends_account`, `owner_opens_leaver_after_erasure` (404), `owner_lists_after_erasure`, `leaver_asks_owner_after_erasure` (201, dev), `owner_accepts_ended_account`, `leaver_opens_owner_after_refriend` (404 `context_not_found`), `owner_opens_leaver_after_refriend` (404 `person_not_found`).
- Framework: `trailing_slash`, `get_not_allowed` (405 `allow: POST`).

`crossreplay/POST-people-person_id-dm.yaml` (18 bước): 201 lưu qua Python phát lại là 201 qua cửa trước trong khi mở không khoá là 200; 404 người lạ nhả khoá.

`concurrency/POST-people-person_id-dm.yaml` (11 bước): `owner_opens_three_at_once` → hai 200 và một 201, một pair; `friend_opens_three_at_once` → ba 200; `owner_same_key_three_at_once` → ba 201, hai phát lại. Hai bước tạo pair có dòng RACY: `contexts.pair_key` ghép hai id persona theo thứ tự chuỗi, id persona đổi theo lượt lặp reference; candidate dùng cùng nonce với lượt đầu và khớp.

`prod-auth.yaml`: `anonymous_opens_dm`, `owner_opens_pair_with_key` (201), `owner_replays_pair_key`, `mate_same_key_own_session` (200: pair đã có, scope khác), `stranger_opens_mate_claiming_owner` (404), `owner_opens_pair_after` (401), `mate_opens_pair_after` (404).

Mở pair làm bước dựng ở `GET-people-me.yaml`, `GET-people-me-contexts.yaml`, `GET-people-person_id.yaml`, hai tệp chặn, `DELETE-people-me-world.yaml`, và các kịch bản W1/W8 dùng pair.

Corpus sinh: `generated/w10-422/post-people-person_id-dm.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- Nhắn cho chính mình là 422 **trước** vai trò và mọi truy vấn; mọi từ chối khác gộp vào một 404, kể cả thiếu vai trò.
- `pair_key` so chuỗi hai id dạng chuẩn viết thường, `<nhỏ>:<lớn>`; cùng pair cho cả hai chiều.
- Tạo pair: `display_name` rỗng, `kind='pair'`, hai membership `active`, `role='member'`, `origin='named'`, `invited_by_id` và `created_by_id` là người mở, `joined_at` cùng `now`.
- Thân lấy từ `list_person_context_summaries` của người gọi (cùng thứ tự khoá và cách tính với `GET /people/me/contexts`) nhưng `unavailable` không được tính (luôn false).
- Không đưa vào kịch bản hai người mở cùng một pair đồng thời (người thắng quyết `created_by_id`; mỗi bước đồng thời là một persona). Nhánh 409 `pair_exists` không tới được.

## Lỗi Python (chỉ báo, không sửa)

- `unavailable` trong thân route này luôn `false`, kể cả khi pair đã chặn (chỉ quan sát được khi mở lại pair sau khi kết bạn lại, nơi không còn chặn).
- `can_open(other_exists=…)` và `_is_blocked_with` không chặn thêm gì so với «là bạn»: chặn và xoá tài khoản đều đã gỡ cạnh `accepted` trước đó. Hai lớp trùng.
- Dev: người đã xoá tài khoản vẫn kết bạn được và mở pair cũ nhận 404 `context_not_found` `Context does not exist` (tiếng Anh, khác câu 404 của cửa này), trong khi người kia nhận câu chung.
