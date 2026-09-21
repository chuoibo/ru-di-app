# GET /posts

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Bảng tin: một trang những bài người gọi được đọc, mới nhất trước. Người đọc luôn là actor; không có tham số `viewer_id`, không có cursor để đọc trang tiếp.

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`): `dev` thiếu `X-Actor-ID` → 401, role lạ → 422 `invalid_actor_roles`; `prod` đọc bearer, bỏ qua `X-Actor-*` (401 `Missing bearer session` / `Session is not valid`). Dependency chạy **trước** validate query: người không danh tính gửi `limit=0` nhận 401 (`anonymous_limit_zero`).
2. Validate query `limit` → 422.
3. **Không có** `_require_permission`: role rỗng hay chỉ `guest` vẫn 200 (`author_roles_empty_feed`).
4. Lọc hai lớp cho mỗi dòng:
   - SQL `SqlAlchemyApiRepository._readable_by` (`services/api/app/api/repository.py:5423-5497`) chỉ lấy dòng thoả một trong: là tác giả; `public` và không có cạnh `blocked` giữa hai người (hai chiều); `friends`, không bị chặn, và có cạnh `accepted` (hai chiều); `group` có `context_id` và người đọc có membership `ACTIVE`, `left_at IS NULL`.
   - Domain `post_audience.visible_to` (`services/api/app/domain/post_audience.py:126-156`) chạy lại trên từng dòng trong `_readable_posts` (`services/api/app/api/service.py:2165-2195`), với `is_friend` từ `get_friend_edge` state `accepted` (`service.py:2127-2136`), `is_group_member` từ `is_member` và `is_blocked` từ cạnh `blocked` (`service.py:2138-2148`).
   - Hệ quả: `only_me` chỉ tác giả; lời mời kết bạn đang chờ không phải bạn; chặn (ai chặn ai cũng vậy) giấu `public` và `friends` cả hai chiều nhưng bài `group` trong nhóm chung vẫn đọc được; rời nhóm thì mất bài `group` của người khác, bài của chính mình vẫn còn (vế tác giả); `X-Actor-Contexts` không được đọc.

## Đầu vào

- Query `limit`: `int`, `ge=1`, `le=100`, mặc định 50 (`services/api/app/api/routes/posts.py:72-85`). Pydantic parse lax: `1.0` được nhận là 1 (đo ở `GET /people/{person_id}/posts`), `1.5`, `abc`, chuỗi rỗng → `int_parsing`; `0`, `-1` → `greater_than_equal`; `101` → `less_than_equal`. Lặp tham số (`?limit=1&limit=0`) thì **giá trị cuối** thắng (`stranger_limit_repeated` → 422).
- Tham số lạ bị bỏ qua (`stranger_viewer_id_ignored`).
- Không đọc body. `Idempotency-Key` bị bỏ qua vì GET không thuộc `WRITE_METHODS` (`services/api/app/api/idempotency.py:76`, `:405-407`), kể cả key rỗng (`stranger_feed_empty_idempotency_key_ignored` → 200).

## Đầu ra

- **200** `PostListResponse` (`services/api/app/api/schemas.py:1689-1690`): `{"posts":[...]}`, không có `next_cursor`, `has_more` hay tổng.
- Thứ tự `created_at DESC, id DESC` (`repository.py:5529-5535`), rồi `LIMIT`.
- Mỗi phần tử là `PostResponse` (`schemas.py:1663-1686`), thứ tự khoá `id`, `author_id`, `audience`, `context_id`, `body`, `image_url`, `created_at`, `author_display_name`, `reactions`, `my_reactions`, `comment_count`, `can_comment`; dựng ở `_wire_posts` (`service.py:2197-2244`):
  - `reactions`: `[{"kind","count"}]` xếp theo `kind` (chuỗi); `my_reactions`: các kind của **người đọc**, xếp tăng; `comment_count`: số comment. Ba truy vấn gom nhóm trên cả trang (`repository.py:5550-5583`).
  - `can_comment`: `post_audience.can_comment` (`post_audience.py:94-123`) cho người đọc này theo `wall_comment_policy` của tác giả (`readers` / `friends` / `nobody`); tác giả luôn `true`; không còn dòng tác giả → policy `nobody`.
  - `author_display_name`: tên hiện tại của tác giả.
- `created_at`: ISO 8601 UTC `Z`, 6 chữ số (giá trị `_now()` lúc tạo). Không có float.
- Framework: 307 cho `/posts/` (`location: http://<Host>/posts`); 405 + `allow: POST` cho `DELETE /posts` (route POST khai trước).

## Tác dụng phụ

Chỉ đọc: `posts`, `friend_requests`, `memberships`, `people`, `post_reactions`, `post_comments`. Không idempotency, không limiter. Truy vấn theo từng tác giả và từng nhóm khác nhau trên trang (`get_friend_edge`, `is_member`, `get_person`), không cache giữa request.

**Trạng thái toàn cục**: bài `public` của mọi người (mọi kịch bản, mọi lần chạy trước trên cùng stack) đều nằm trong bảng tin của một persona mới. Hai stack so sánh được chỉ khi cùng nhận một chuỗi request; đó là lý do các bước dựa vào riêng kịch bản dùng `limit` không lớn hơn số bài kịch bản tạo cho người đọc đó.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143`, `:102-106` |
| 401 | `authentication_required` | `Session is not valid` (prod) | `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `PUT /people/me/interests` | `deps.py:144-163` |
| 422 | (framework) | `greater_than_equal` / `less_than_equal` / `int_parsing`, `loc: ["query","limit"]` | `routes/posts.py:76` |

Route không ném `ApiProblem` nào khác.

## Mã Python

- Route: `services/api/app/api/routes/posts.py:72-85` (`list_posts`)
- Service: `services/api/app/api/service.py:2484-2490` (`list_posts`), `:2165-2195` (`_readable_posts`), `:2197-2244` (`_wire_posts`), `:2127-2148` (`_is_friend`, `_is_blocked_with`), `:1465-1475` (`_friend_edge_dict`)
- Repository: `services/api/app/api/repository.py:5423-5497` (`_readable_by`), `:5526-5535` (`list_posts_visible_to`), `:5550-5583` (`post_social_counts`), `:2769-2782` (`is_member`)
- Domain: `services/api/app/domain/post_audience.py:94-201`, `services/api/app/domain/blocking.py:40-64`

## Test đang phủ

- `services/api/tests/api/test_posts_audience.py`: `test_only_me_reaches_the_author_and_nobody_else` (154), `test_friends_reaches_friends_only` (228), `test_unfriending_takes_the_post_back` (239), `test_a_pending_request_is_not_friendship` (253), `test_group_reaches_the_group_only` (261), `test_leaving_the_group_takes_the_post_back` (273), `test_public_reaches_everybody` (281), `test_each_reader_sees_exactly_their_own_slice` (292)
- `services/api/tests/postgres/test_posts_postgres.py`: `test_the_query_returns_exactly_the_reader_slice` (197), `test_only_me_never_leaves_the_database` (213), `test_a_pending_friend_request_does_not_open_the_friends_audience` (231), `test_a_departed_member_stops_reading_the_group` (244), `test_a_group_post_does_not_reach_another_group` (254)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py`: `test_a_block_hides_the_wall_both_ways_and_the_rows_never_leave_the_database` (42)
- `services/api/tests/api/test_sessions_and_blocking.py`: `test_a_block_hides_the_wall_both_ways_but_leaves_the_shared_group` (123)
- `services/api/tests/postgres/test_post_social_postgres.py`: `test_three_hearts_and_two_comments_read_three_and_two_over_http` (120)

## Kịch bản parity

`parity/scenarios/w2/posts/GET-posts.yaml`, id `w2/posts/get-posts` (56 bước). Kịch bản tạo 11 bài đánh số trong body (1-10 trước khi đọc, 11 của người sắp xoá tài khoản).

- Thứ tự: `anonymous_feed`, `anonymous_limit_zero` (401 trước 422).
- `limit`: `stranger_limit_zero`, `stranger_limit_101`, `stranger_limit_negative`, `stranger_limit_not_int`, `stranger_limit_float`, `stranger_limit_empty`, `stranger_limit_repeated`; `roles_unknown` (422 actor).
- Ma trận người đọc (limit đúng bằng số bài kịch bản cho người đó): `author_feed_limit_8`, `author_feed_limit_1`, `author_roles_empty_feed`, `friend_feed_limit_5`, `mate_feed_limit_4`, `stranger_feed_limit_3_with_contexts_header`, `stranger_viewer_id_ignored`, `stranger_feed_empty_idempotency_key_ignored`.
- Chặn hai chiều: `blocker_blocks_author`, `author_feed_after_block_limit_6`, `blocker_feed_after_block_limit_4`.
- Rời nhóm: `mate_leaves_group`, `mate_feed_after_leaving_limit_3`.
- Người đọc đã xoá tài khoản (`dev` vẫn tin header): `leaver_posts_public`, `leaver_feed_before_deletion_limit_3`, `leaver_deletes_account`, `leaver_feed_after_deletion_limit_2` (bài của chính họ đã bị xoá).
- Đọc trạng thái toàn cục (có chủ ý): `stranger_feed_default_limit_reads_global_public`, `stranger_feed_limit_100_reads_global_public`.
- Framework: `trailing_slash_redirects` (307), `delete_not_allowed` (405 `allow: POST`).

`parity/scenarios/w2/posts/prod-auth.yaml` (auth `prod`): `anonymous_feed`, `junk_bearer_feed` (401 hai câu), `owner_feed_limit_5`, `reader_feed_limit_2`, `leaver_feed_after_deletion` (401 sau khi xoá tài khoản).

## Chưa phủ / lưu ý cho bản Go

- Hai lớp lọc phải cho cùng một tập; bản Go có thể chỉ giữ một lớp nếu nó đúng bằng cả hai, nhưng khi SQL lấy `LIMIT n` rồi domain loại bớt thì trang sẽ ngắn hơn `n`. Với dữ liệu thật hai lớp trùng nhau nên harness không tạo được ca trang ngắn.
- Chặn không giấu bài `group` trong nhóm chung (`blocker_feed_after_block_limit_4` vẫn có bài nhóm của tác giả).
- Không có cursor: đọc quá 100 bài là không thể; nếu bản Go thêm cursor thì phải mở ADR.
- Hai bài cùng `created_at` xếp theo `id DESC` (uuid ngẫu nhiên); harness không tạo được hai bài cùng micro giây.
- Các bước `*_reads_global_public` phụ thuộc lịch sử của stack: chỉ bằng nhau khi hai stack nhận cùng chuỗi request (chạy `parity run` hay canary đều thoả). Probe tay chỉ gửi một phía sẽ làm chúng lệch.
- Bài có tác giả đã xoá tài khoản không còn trong DB (bị `erase_person` xoá), nên nhánh `author_display_name: ""` / policy `nobody` khi thiếu dòng tác giả không tới được.
- `prod`: role không ảnh hưởng (route không kiểm quyền); khác biệt duy nhất là cách xác thực.
