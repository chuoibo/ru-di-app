# GET /posts/{post_id}

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Đọc một bài. Bài không tồn tại và bài tồn tại nhưng người gọi không được đọc trả **cùng một 404**, để route không thành máy dò id bài trong nhóm hay tường riêng của người khác.

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`) chạy trước validate path: người không danh tính gửi id sai dạng nhận 401 (`anonymous_non_uuid_post`).
2. Path `post_id` (UUID lax) → 422 `uuid_parsing`.
3. `_readable_post_or_404` (`services/api/app/api/service.py:2246-2270`):
   - `repository.get_post` (`services/api/app/api/repository.py:5522-5524`) → không có → 404 `post_not_found`.
   - `_post_facts` (`service.py:2150-2163`): `is_friend` (cạnh `accepted`, `:2127-2136`), `is_group_member` (membership `ACTIVE`, `left_at IS NULL`, `repository.py:2769-2782`), cộng `is_blocked` (`service.py:2138-2148`).
   - `post_audience.visible_to` (`services/api/app/domain/post_audience.py:126-156`, trên `can_read` `:165-201`) sai → **cùng** 404 `post_not_found`.
4. **Không có** `_require_permission`: role rỗng hoặc `guest` vẫn đọc được (`author_roles_empty_reads_own`, `stranger_roles_guest_reads_public`).

Quy tắc đọc: tác giả luôn đọc; `public` ai cũng đọc trừ khi có chặn giữa hai người (ai chặn ai cũng vậy); `friends` chỉ bạn `accepted` và không bị chặn (lời mời đang chờ hay cạnh `declined` sau khi gỡ chặn đều không phải bạn); `group` chỉ thành viên `ACTIVE` của đúng nhóm đó, **kể cả** khi hai người chặn nhau; `only_me` chỉ tác giả. `X-Actor-Contexts` và `X-Actor-ID` của người khác không mở được gì.

## Đầu vào

- Path `post_id`: UUID lax của pydantic. Chữ hoa, không gạch nối, có ngoặc nhọn (`%7B…%7D`) đều parse được và đi tiếp tới 404 (`stranger_uppercase_unknown_post`, `stranger_unhyphenated_uuid`, `stranger_braced_uuid`); thiếu ký tự → 422 với `msg` nêu nhóm sai (`stranger_short_uuid`).
- Không query, không body. GET không qua idempotency.

## Đầu ra

- **200** `PostResponse` (`services/api/app/api/schemas.py:1663-1686`), thứ tự khoá `id`, `author_id`, `audience`, `context_id`, `body`, `image_url`, `created_at`, `author_display_name`, `reactions`, `my_reactions`, `comment_count`, `can_comment` (`_wire_posts`, `service.py:2197-2244`, gọi từ `:2360-2363`):
  - `reactions`: đếm theo kind, xếp theo tên kind (`fire` trước `heart`); `my_reactions`: kind của **người đọc**, xếp tăng (tác giả thấy `["fire","heart"]`, bạn thấy `["heart"]` trên cùng bài, `author_reads_public_with_counts` / `friend_reads_public_with_counts`); `comment_count`.
  - `can_comment` theo `wall_comment_policy` của tác giả cho **người đọc này** (`post_audience.py:94-123`): `readers` → mọi người đọc được; `friends` → chỉ bạn (thành viên nhóm nhận `false`, `mate_reads_group_policy_friends`); `nobody` → chỉ tác giả (`friend_reads_public_policy_nobody` → `false`, `author_reads_own_policy_nobody` → `true`).
- 404: `{"code":"post_not_found","detail":"Post does not exist"}` cho cả không tồn tại lẫn không được đọc.
- `created_at`: ISO 8601 UTC `Z`, 6 chữ số. Không float.
- Framework: 307 cho `/posts/{id}/` (`location` là `http://<Host>/posts/{id}`); 405 + `allow: GET` cho `DELETE` và `POST` lên `/posts/{id}`.

## Tác dụng phụ

Chỉ đọc: `posts`, `friend_requests`, `memberships`, `people`, `post_reactions`, `post_comments`. Không idempotency, không limiter.

Xoá tài khoản tác giả (`erase_person`, `repository.py:4770-4835`) xoá luôn bài, reaction và comment của họ: id bài cũ trả 404 cho mọi người kể cả chính tác giả (`stranger_reads_gone_after_deletion`, `gone_reads_own_post_after_deletion`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143`, `:102-106` |
| 401 | `authentication_required` | `Session is not valid` (prod) | `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `PUT /people/me/interests` | `deps.py:144-163` |
| 422 | (framework) | `uuid_parsing`, `loc: ["path","post_id"]` | `services/api/app/api/routes/posts.py:111` |
| 404 | `post_not_found` | `Post does not exist` | `service.py:2257-2258`, `:2263-2269` |

## Mã Python

- Route: `services/api/app/api/routes/posts.py:109-117` (`read_post`)
- Service: `services/api/app/api/service.py:2350-2363` (`read_post`), `:2246-2270` (`_readable_post_or_404`), `:2150-2163` (`_post_facts`), `:2127-2148`, `:2197-2244`
- Repository: `services/api/app/api/repository.py:5522-5524` (`get_post`), `:7424` (`get_friend_edge`), `:2769-2782` (`is_member`), `:5550-5583` (`post_social_counts`), `:4770-4835` (`erase_person`)
- Domain: `services/api/app/domain/post_audience.py:94-201`, `services/api/app/domain/blocking.py:40-64`, `services/api/app/domain/friendship.py:217-256` (chặn và gỡ chặn)

## Test đang phủ

- `services/api/tests/api/test_posts_audience.py`: `test_only_me_is_not_readable_by_id_by_anyone_else` (163), `test_reading_by_id_refuses_every_audience_the_reader_is_outside_of` (176)
- `services/api/tests/postgres/test_posts_postgres.py`: `test_reading_someone_elses_only_me_post_by_id_is_404` (376), `test_reading_by_id_refuses_every_audience_the_reader_is_outside_of` (410)
- `services/api/tests/api/test_post_comments_reactions.py`: `test_a_reader_reacts_once_per_kind_and_the_post_recounts_from_the_rows` (83), `test_the_wall_owner_decides_who_may_comment_and_the_post_says_so` (176), `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` (260)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py`: `test_lifting_a_block_gives_the_wall_back_but_not_the_friendship` (160)
- `services/api/tests/domain/test_post_audience.py` (36-238): ma trận `can_read`

## Kịch bản parity

`parity/scenarios/w2/posts/GET-posts-post_id.yaml`, id `w2/posts/get-posts-post_id` (94 bước):

- Thứ tự và path: `anonymous_unknown_post`, `anonymous_non_uuid_post` (401 trước 422), `stranger_non_uuid_post`, `stranger_short_uuid` (422), `stranger_braced_uuid`, `stranger_unhyphenated_uuid`, `stranger_uppercase_unknown_post`, `stranger_unknown_post` (404), `roles_unknown` (422).
- Chuẩn bị: kết bạn (`author_asks_friend`, `friend_accepts`, `blocker_asks_author`, `author_accepts_blocker`), lời mời treo (`requester_asks_author`), nhóm ba thành viên, bốn bài của tác giả (`author_posts_only_me` … `author_posts_public`), bài công khai của `blocker`, `blocked`, `gone`; reaction và comment (`friend_reacts_heart`, `author_reacts_heart`, `author_reacts_fire`, `friend_comments`).
- Ma trận: tác giả `author_reads_*` (4 × 200, `author_roles_empty_reads_own`); bạn `friend_reads_*`; thành viên nhóm `mate_reads_*` (nhóm không phải bạn); người lạ `stranger_reads_*`, `stranger_reads_group_with_contexts_header`, `stranger_roles_guest_reads_public`; lời mời treo `requester_reads_friends`, `requester_reads_public`.
- Chặn hai chiều: `blocker_reads_friends_before_block`, `blocker_blocks_author`, `author_blocks_blocked`, rồi `blocker_reads_friends_after_block`, `blocker_reads_public_after_block` (404), `blocker_reads_group_after_block` (200 nhóm chung), `blocker_reads_own_public`, `author_reads_blocker_public` (404), `blocked_reads_public` (404), `blocked_reads_group` (200), `author_reads_blocked_public` (404), `mate_reads_blocked_public` (200: chặn chỉ giữa hai người).
- Gỡ chặn: `blocker_lifts_block`, `blocker_reads_public_after_unblock` (200), `blocker_reads_friends_after_unblock` (404: cạnh `declined`).
- `can_comment`: `author_sets_policy_friends`, `mate_reads_group_policy_friends`, `friend_reads_friends_policy_friends`, `author_sets_policy_nobody`, `friend_reads_public_policy_nobody`, `author_reads_own_policy_nobody`.
- Rời nhóm: `mate_leaves_group`, `mate_reads_group_after_leaving` (404).
- Xoá tài khoản: người đọc `leaver_reads_public_before_deletion`, `leaver_deletes_account`, `leaver_reads_public_after_deletion` (`dev` vẫn 200); tác giả `stranger_reads_gone_before_deletion`, `gone_deletes_account`, `stranger_reads_gone_after_deletion`, `gone_reads_own_post_after_deletion` (404).
- Framework: `trailing_slash_redirects` (307), `delete_not_allowed`, `post_to_post_id_not_allowed` (405 `allow: GET`).

`parity/scenarios/w2/posts/prod-auth.yaml` (auth `prod`): `anonymous_read_post` (401 `Missing bearer session`), `junk_bearer_read_post` (401 `Session is not valid`), `reader_reads_public`, `reader_reads_friends_not_yet`, `reader_reads_group_with_actor_headers` (header dev bị bỏ qua → 404), `reader_asks_owner`, `owner_accepts_reader`, `reader_reads_friends_as_friend`, `reader_reads_leaver_post`, `reader_reads_leaver_post_after_deletion` (404), `reader_signs_out`, `reader_reads_public_after_sign_out` (401), `owner_still_reads_public`.

## Chưa phủ / lưu ý cho bản Go

- Thân 404 phải giống hệt nhau cho "không tồn tại" và "không được đọc"; tách thành 403 là mở lại máy dò mà route cố tình đóng.
- Id có ngoặc nhọn và id không gạch nối được chấp nhận vì pydantic parse UUID lax; `uuid.Parse` của Go cũng nhận ngoặc nhọn và dạng 32 hex, nhưng thông điệp 422 (`invalid group length in group 4: expected 12, found 11`, `invalid character: found \`k\` at 1`) là của pydantic-core và phải chép nguyên.
- Kịch bản không đọc được bài bằng id viết hoa của một bài **có thật**: template của harness không đổi hoa thường giá trị đã bind. Chỉ phủ id viết hoa của bài không tồn tại.
- Không còn dòng tác giả trong `people` (`author_display_name: ""`, policy `nobody`): không tới được, vì xoá tài khoản xoá luôn bài và `people` chỉ bị ẩn danh chứ không bị xoá.
- `is_blocked` bỏ qua khi người đọc là tác giả (`service.py:2146-2147`).
- `prod`: người đọc đã đăng xuất hoặc đã xoá tài khoản nhận 401 thay vì đọc tiếp như `dev`.
