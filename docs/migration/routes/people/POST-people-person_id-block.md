# POST /people/{person_id}/block

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Chặn một người (ADR-0023 §2.3). Chặn không có bảng riêng: nó là trạng thái `blocked` của cạnh sống duy nhất giữa hai người trong `friend_requests`, với `decided_by_id` là người chặn. Chặn lần hai là cùng bức tường, trả 200 như lần đầu.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:184-200`, `services/api/app/api/service.py:4569-4606`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): khoá rỗng 422; khoá đã xong phát lại 200; khoá dùng cho người khác → 422 `idempotency_key_reuse` (`owner_key_other_person`).
2. Router: đuôi `/` → 307 (`trailing_slash`); `GET` → 405 (`get_not_allowed`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 trước 422 của path (`anonymous_non_uuid`); 422 `invalid_actor_id`, `invalid_actor_roles`.
4. Path `person_id`: UUID lax → 422 `uuid_parsing` (`owner_path_not_uuid`). Thân không khai báo, bị bỏ qua kể cả JSON (`owner_blocks_with_json_body`).
5. `_require_permission("block_person", {"is_not_self": actor.id != person_id})` (`service.py:4572-4574`; `services/api/app/domain/permissions.py:221`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_blocks_self_without_roles`, `owner_blocks_target_without_roles`, `owner_blocks_unknown_as_guest`); chặn chính mình → 403 `permission_denied` `is_not_self`, **trước** khi tra người, kể cả khi người gọi chưa đăng ký (`owner_blocks_self_unregistered`, `owner_blocks_self`).
6. `get_person(person_id)` không có → 404 `person_not_found` `Chưa có ai mang danh tính này.` (`service.py:4575-4576`; `owner_blocks_unknown_id`). Người đã xoá tài khoản vẫn là một hàng: không bị từ chối (`owner_blocks_ended_account` 200).
7. `get_friend_edge(actor, person)` (`services/api/app/api/repository.py:7424-7429`, `_edge_between` `:7403-7422`: cạnh không phải `declined`, mới nhất theo `created_at`), rồi `friendship.open_block` (`services/api/app/domain/friendship.py:217-240`):
   - không có cạnh, hoặc cạnh `declined` → cạnh mới;
   - `pending` (ai hỏi cũng được) hoặc `accepted` → `decide(BLOCK)`;
   - `blocked` (do ai chặn cũng vậy) → `ALREADY_BLOCKED` → **200** `{"state":"blocked"}` không ghi gì (`service.py:4593-4595`).
   Các mã khác của domain qua `_friend_refusal` (`service.py:7203-7212`) không tới được từ route này (`NOT_A_PARTY` cần cạnh không chứa người gọi).
8. `open_block_edge` (`repository.py:4657-4699`): khoá cạnh sống `FOR UPDATE`; có → `state='blocked'`, `decided_by_id=actor`, `decided_at=now`; không có → INSERT `friend_requests(id=uuid4, requester_id=actor, addressee_id=person, state='blocked', decided_by_id=actor, created_at=now, decided_at=now)` trong savepoint. `IntegrityError` (`uq_friend_edge_live`, `services/api/app/db/models.py:2153`) → `EDGE_EXISTS` → nuốt, vẫn 200 (`service.py:4596-4605`).

## Đầu vào

- Path `person_id`: UUID (viết hoa, không gạch nối, `urn:uuid:`, ngoặc nhọn đều được Pydantic chấp nhận; xem corpus sinh).
- Không query, không thân.

## Đầu ra

**200** `{"person_id": "<uuid>", "state": "blocked"}` (`BlockResponse`, `services/api/app/api/schemas.py:1014-1019`), thứ tự khoá như trên, `person_id` in lại dạng chuẩn. Giống hệt nhau cho lần đầu, lần lặp, và khi người bị chặn «chặn lại» người đã chặn mình.

## Tác dụng phụ

- Một hàng `friend_requests` được cập nhật hoặc chèn như bước 8. Cạnh `declined` cũ ở lại, cạnh `blocked` mới nằm cạnh nó (`owner_blocks_after_decline`).
- Chặn từ `accepted` chấm dứt tình bạn (cạnh sống duy nhất nay là `blocked`); chặn từ lời mời của chính mình hoặc lời mời gửi tới mình đều đổi đúng hàng đó, `requester_id`/`addressee_id` giữ nguyên.
- Không đụng nhóm, membership, pair, bài, story.

Hệ quả ở route khác (đo trong `POST-people-person_id-block.yaml`, cùng câu chữ ở `services/api/app/domain/blocking.py`):

| Route | Sau khi chặn |
|---|---|
| `GET /people/{id}/friends` | người kia biến mất khỏi danh sách của cả hai |
| `POST /friends/requests` (cả hai chiều) | 409 `request_not_open` `Chưa gửi được lời mời này.`, cùng byte với «đã có lời mời» |
| `GET /posts`, `GET /people/{id}/posts` | bài `public`/`friends` của người kia ẩn hai chiều; bài `group` trong nhóm chung vẫn thấy |
| `GET /posts/{id}`, `GET/POST /posts/{id}/comments` | bài `public` của người kia 404; bài `group` 200 |
| `GET /stories`, ảnh của story | story ẩn hai chiều; ảnh cá nhân chỉ hiện qua story → 404 |
| `GET /people/{id}` | không còn `friend`; còn nhóm hoặc pair chung thì vẫn 200 `groupmate` |
| `POST /people/{id}/dm` | 404 `person_not_found` `Chưa thể nhắn riêng với người này.` cả hai chiều |
| `GET /people/me/contexts` | hàng pair có `unavailable: true` cho cả hai |
| `POST /contexts/{pair}/messages` | 409 `direct_message_unavailable` `Cuộc trò chuyện này không còn nhận tin.` |
| `POST /contexts/{group}/messages` | 201: nhóm chung không bị chạm |
| `GET /people/me/blocked` | chỉ người chặn thấy hàng; người bị chặn thấy danh sách rỗng |

## Idempotency

Tự nhiên idempotent (lặp là 200 không ghi). Khoá header: 200 được lưu và phát lại kể cả sau khi đã gỡ chặn (`crossreplay/POST-people-person_id-block.yaml`). Từ chối (403/404) nhả khoá.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143`; `service.py:4452-4474` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | như các route khác | `deps.py:144-155` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","person_id"]` | `main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_not_self` | `service.py:4572-4574` |
| 404 | `person_not_found` | `Chưa có ai mang danh tính này.` | `service.py:4575-4576` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:184-200`
- Service: `services/api/app/api/service.py:4569-4606`, `:7203-7212` (`_friend_refusal`), `:2138-2148` (`_is_blocked_with`)
- Repository: `services/api/app/api/repository.py:4657-4699` (`open_block_edge`), `:4643-4655` (`_friend_edge_row`), `:7403-7429` (`_edge_between`, `get_friend_edge`)
- Domain: `services/api/app/domain/friendship.py:170-240` (`decide`, `open_block`), `services/api/app/domain/blocking.py`
- Quyền: `services/api/app/domain/permissions.py:221`

## Test đang phủ

- `services/api/tests/api/test_sessions_and_blocking.py`: `test_blocking_is_idempotent_and_only_the_blocker_lifts_it` (85), `test_blocking_yourself_and_a_stranger_are_refused_differently` (115), `test_a_block_hides_the_wall_both_ways_but_leaves_the_shared_group` (123)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py`: `test_a_block_hides_the_wall_both_ways_and_the_rows_never_leave_the_database` (42), `test_a_block_hides_a_story_both_ways` (84), `test_a_friend_request_from_a_blocked_person_reads_like_a_duplicate` (118), `test_lifting_a_block_gives_the_wall_back_but_not_the_friendship` (160)
- `services/api/tests/api/test_direct_messages.py`: `test_a_blocked_pair_says_so_on_the_row_for_both_people` (237)

## Kịch bản parity

`parity/scenarios/w10/people/POST-people-person_id-block.yaml`, id `w10/people/post-people-person_id-block` (76 bước, `dev`):

- Thứ tự: `anonymous_blocks`, `anonymous_non_uuid` (401 trước 422), `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid`, `owner_blocks_self_unregistered` (403 `is_not_self` trước khi tra người), `owner_blocks_self_without_roles` (403 `role_not_permitted`), `owner_blocks_unknown_id` (404), `owner_blocks_unknown_as_guest` (403), `owner_blocks_self`, `owner_blocks_target_without_roles` (`advancer` thôi).
- Dựng: bạn `accepted`, nhóm chung, pair, ảnh cá nhân và story của `target`, bài `public`/`friends`/`group` của `target`, bài `public` của `owner` với bình luận của `target`; đọc trước `owner_reads_feed_before`, `owner_reads_stories_before`, `owner_lists_friends_before`.
- Chặn: `owner_blocks_target` (một cập nhật `friend_requests`), `owner_blocks_target_again` (không delta), `owner_blocks_target_with_key`, `owner_replays_key`, `owner_key_other_person`, `owner_blocks_with_json_body`, `target_blocks_owner_back` (200 `blocked`, không ghi), `target_lists_own_blocks` (rỗng), `owner_lists_own_blocks`.
- Hệ quả: `owner_lists_friends_after`, `target_lists_friends_after`, `target_asks_owner_again` và `owner_asks_target_again` (409 `request_not_open`), `owner_reads_feed_after`, `target_reads_feed_after`, `owner_reads_target_wall_after` (chỉ bài `group`), `target_reads_owner_post_after`, `target_reads_owner_post_comments_after`, `target_comments_after` (404 `post_not_found`), `owner_reads_target_group_post_after` (200), `owner_reads_target_public_post_after` (404), `owner_reads_stories_after` (`{"authors":[]}`), `owner_reads_target_story_photo_after` (404 `photo_not_found`), `owner_reads_target_profile_after` (200 `groupmate`), `target_opens_pair_after` (404), `target_lists_contexts_after` (pair `unavailable: true`), `target_writes_in_pair_after` (409), `target_writes_in_group_after` (201).
- Cạnh ban đầu khác: `owner_blocks_own_pending`, `owner_blocks_incoming_pending` (cập nhật đúng hàng lời mời), `owner_blocks_after_decline` (chèn hàng mới), `bystander_blocks_owner` + `owner_blocks_bystander_back` (200, không ghi, không lên danh sách của `owner`), `owner_lists_blocks_after_variants`.
- `owner_blocks_ended_account` (200, chèn cạnh tới người đã xoá). Framework: `trailing_slash`, `get_not_allowed` (405 `allow: POST`).

`crossreplay/POST-people-person_id-block.yaml` (13 bước): 200 lưu qua Python phát lại qua cửa trước dù đã gỡ chặn, và ngược lại; 404 người lạ nhả khoá.

`concurrency/POST-people-person_id-block.yaml` (10 bước): ba lần chặn người lạ cùng lúc → ba 200, một hàng chèn; ba lần chặn bạn cùng lúc → ba 200, một cập nhật; ba lần cùng khoá → một ghi, hai phát lại.

`prod-auth.yaml`: `junk_bearer_blocks`, `owner_blocks_stranger`, `stranger_unblocks_claiming_owner`, `owner_blocks_after` (401), `mate_blocks_owner_after` (200 lên id đã xoá).

Chặn làm bước dựng ở `GET-people-me-blocked.yaml`, `GET-people-me-contexts.yaml`, `GET-people-person_id.yaml`, `POST-people-person_id-dm.yaml`, `DELETE-people-person_id-block.yaml`, `DELETE-people-me-world.yaml`.

Corpus sinh: `generated/w10-422/post-people-person_id-block.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- Kiểm `is_not_self` (và vai trò) trước khi tra người: chặn chính mình là 403 kể cả khi người gọi chưa có hàng `people`.
- Cạnh đã `blocked` theo **bất kỳ** chiều nào trả 200 `blocked` mà không ghi; `decided_by_id` giữ người chặn đầu tiên.
- Chèn dùng `uuid4` mới cho `friend_requests.id`; `requester_id` là người chặn; cạnh `declined` cũ không đổi.
- `IntegrityError` khi chèn (hai lần chặn cùng cặp đồng thời) bị nuốt thành 200; bản Go phải phân biệt vi phạm `uq_friend_edge_live` với vi phạm khoá ngoại (xem Lỗi Python) theo đúng cách Python đang làm: cả hai đều nuốt.
- Chưa phủ: hai người chặn nhau đồng thời (người thắng quyết `decided_by_id`, không tất định), người gọi chưa đăng ký chặn một người có thật (khoá ngoại `requester_id` hỏng bị nuốt thành 200, xem Lỗi Python).

## Lỗi Python (chỉ báo, không sửa)

- Người bị chặn «chặn lại» nhận 200 `{"state":"blocked"}` nhưng không có gì được ghi: danh sách chặn của họ vẫn rỗng và họ không gỡ được bức tường mà câu trả lời nói họ vừa dựng.
- Chặn một tài khoản đã xoá là 200 và chèn cạnh mới tới hàng ẩn danh.
- `open_block_edge` biến **mọi** `IntegrityError` thành `EDGE_EXISTS` và service nuốt nó: một người gọi dev chưa có hàng `people` chặn một người có thật nhận 200 dù chèn thất bại vì khoá ngoại.
- Chặn không ẩn hồ sơ khi hai người còn nhóm hoặc pair chung (`groupmate`), và pair sau khi chặn vẫn là «nhóm chung».
