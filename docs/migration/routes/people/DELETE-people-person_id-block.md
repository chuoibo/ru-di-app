# DELETE /people/{person_id}/block

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Gỡ một lần chặn (ADR-0023 §2.3.3). Chỉ người đã chặn gỡ được; cạnh về `declined`, không bao giờ về `accepted`: tình bạn không tự quay lại.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:203-219`, `services/api/app/api/service.py:4608-4631`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`).
2. Router: đuôi `/` → 307 (`trailing_slash`); `PUT` → 405 (`put_not_allowed`).
3. `get_actor` → 401 (trước 422 của path, `anonymous_non_uuid`), 422 `invalid_actor_id` / `invalid_actor_roles`.
4. Path `person_id`: UUID → 422 `uuid_parsing`.
5. Đọc cạnh **trước** quyền: `get_friend_edge(actor, person)` (`services/api/app/api/repository.py:7403-7429`, cạnh không `declined` mới nhất) rồi `blocking.blocker_of` (`services/api/app/domain/blocking.py:45-55`): `is_blocker` đúng khi cạnh `blocked` và `decided_by_id` là người gọi (`service.py:4611-4620`). Câu SQL chạy cả với id không tồn tại và với chính mình.
6. `_require_permission("unblock_person", {"is_blocker": …})` (`service.py:4621`; `services/api/app/domain/permissions.py:222`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_unblocks_unknown_without_roles`, `owner_unblocks_without_roles`); không phải người chặn → 403 `permission_denied` `is_blocker`. Cùng một 403 cho: id bịa (`owner_unblocks_unknown_id`), chính mình (`owner_unblocks_self`), không có cạnh (`owner_unblocks_stranger_no_edge`), bạn bè `accepted` (`owner_unblocks_friend`), người bị chặn gỡ bức tường của người kia (`target_unblocks_owner`, `friend_unblocks_owner`), lần gỡ thứ hai (`owner_unblocks_target_again`).
7. `lift_block_edge` (`repository.py:4701-4718`): khoá cạnh sống `FOR UPDATE`; không có hoặc không `blocked` → `NOT_BLOCKED`; `decided_by_id` khác → `ONLY_BLOCKER_MAY_UNBLOCK`; cả hai thành 409 `<mã viết thường>` `Không gỡ chặn được người này.` (`service.py:4622-4630`). Tuần tự thì không tới được (bước 5 đã đọc đúng hàng đó); chỉ khi hai lần gỡ đua nhau.

## Đầu vào

- Path `person_id`: UUID. Không query, không thân.

## Đầu ra

**200** `{"person_id": "<uuid>", "state": "declined"}` (`BlockResponse`, `services/api/app/api/schemas.py:1014-1019`).

## Tác dụng phụ

- `friend_requests` của cạnh đó: `state='declined'`, `decided_by_id=actor`, `decided_at=now`; `requester_id`, `addressee_id`, `created_at` giữ.
- Vì `declined` không chiếm cặp, sau đó: `POST /friends/requests` của một trong hai là 201 mới (`target_asks_owner_after_lift`); chặn lại chèn một hàng `blocked` mới cạnh hàng `declined` (`owner_blocks_target_again_over_pending` đi qua lời mời vừa gửi).
- Gỡ chặn một người từng là bạn: không còn trong `GET /people/{id}/friends`; `POST /people/{id}/dm` 404; hàng pair `unavailable: false` trở lại nhưng `POST …/messages` vào pair vẫn theo `dm_allowed` (không chặn, không xoá → nhận tin) (`friend_lists_contexts_after_lift`, `friend_writes_in_pair_after_lift`); hồ sơ còn đọc được qua pair chung (`groupmate`).

## Idempotency

Không tự idempotent: lần hai là 403 `is_blocker`. Khoá header: 200 được lưu và phát lại, kể cả khi đã có bức tường mới (`crossreplay/DELETE-people-person_id-block.yaml`); từ chối nhả khoá.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 422 | (validation) | `uuid_parsing` `["path","person_id"]` | `main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_blocker` | `service.py:4621`, `:475-505` |
| 409 | `not_blocked` / `only_blocker_may_unblock` | `Không gỡ chặn được người này.` (chỉ khi đua) | `service.py:4622-4630` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:203-219`
- Service: `services/api/app/api/service.py:4608-4631`
- Repository: `services/api/app/api/repository.py:4701-4718` (`lift_block_edge`), `:4643-4655`, `:7403-7429`
- Domain: `services/api/app/domain/blocking.py:45-55`; `services/api/app/domain/friendship.py:243-256` (`unblock`, không được route gọi)
- Quyền: `services/api/app/domain/permissions.py:222`

## Test đang phủ

- `services/api/tests/api/test_sessions_and_blocking.py`: `test_blocking_is_idempotent_and_only_the_blocker_lifts_it` (85)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py`: `test_lifting_a_block_gives_the_wall_back_but_not_the_friendship` (160)

## Kịch bản parity

`parity/scenarios/w10/people/DELETE-people-person_id-block.yaml`, id `w10/people/delete-people-person_id-block` (41 bước, `dev`):

- Thứ tự: `anonymous_unblocks`, `anonymous_non_uuid`, `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid`, `owner_unblocks_unknown_id`, `owner_unblocks_self`, `owner_unblocks_unknown_without_roles`, `owner_unblocks_stranger_no_edge`, `owner_unblocks_friend`.
- Gỡ: `owner_blocks_target`, `owner_unblocks_without_roles` (`guest`), `target_unblocks_owner` (403), `owner_unblocks_target` (200 `declined`), `owner_unblocks_target_again` (403 `is_blocker`), `owner_lists_blocks_after_lift`, `target_asks_owner_after_lift` (201), `owner_blocks_target_again_over_pending`, `owner_unblocks_target_second_time`.
- Bức tường trên tình bạn: `owner_blocks_friend`, `friend_unblocks_owner` (403), `owner_unblocks_friend_after_block`, `owner_lists_friends_after_lift` (rỗng), `owner_opens_pair_after_lift` (404), `friend_lists_contexts_after_lift` (`unavailable: false`), `friend_writes_in_pair_after_lift` (201), `owner_reads_friend_profile_after_lift` (200 `groupmate`).
- Khoá: `owner_blocks_other`, `owner_unblocks_other_with_key`, `owner_replays_key`, `owner_key_other_person`, `owner_unblocks_other_without_key_after_replay` (403).
- Framework: `trailing_slash`, `put_not_allowed` (405 `allow: POST`).

`crossreplay/DELETE-people-person_id-block.yaml` (16 bước): 200 `declined` lưu qua Python phát lại qua cửa trước dù đã có bức tường mới, và ngược lại; 403 `is_blocker` nhả khoá.

`concurrency/DELETE-people-person_id-block.yaml` (5 bước): ba lần gỡ cùng khoá → một ghi, hai phát lại.

`prod-auth.yaml`: `anonymous_unblocks`, `stranger_unblocks_claiming_owner` (403: actor là stranger), `owner_unblocks_stranger`.

Corpus sinh: `generated/w10-422/delete-people-person_id-block.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- Đọc cạnh (`get_friend_edge`, cạnh không `declined` mới nhất) trước kiểm quyền; vai trò kiểm trước `is_blocker`.
- Gỡ không khôi phục tình bạn, nhưng cũng không đóng pair: pair nhận tin lại ngay.
- Không đưa vào làn đồng thời ba lần gỡ không khoá: người thua trả 409 `not_blocked` hoặc 403 `is_blocker` tuỳ thời điểm đọc, không tất định. Nhánh 409 vì vậy chưa phủ.

## Lỗi Python (chỉ báo, không sửa)

- Nhánh 409 `not_blocked`/`only_blocker_may_unblock` chỉ tới được khi đua, và khi đua câu trả lời cho cùng một thao tác là 409 hoặc 403 tuỳ lịch.
- Sau khi gỡ chặn, hai người không còn là bạn nhưng pair cũ vẫn nhận tin từ cả hai (`friend_writes_in_pair_after_lift` 201), trong khi mở pair (`POST /people/{id}/dm`) là 404: điều kiện «chỉ bạn bè» của ADR-0021 §2.5 chỉ gác cửa mở, không gác cửa viết.
