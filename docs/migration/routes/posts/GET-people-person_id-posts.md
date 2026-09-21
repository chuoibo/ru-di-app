# GET /people/{person_id}/posts

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tường của một người: bài của người đó mà người gọi được đọc, mới nhất trước. Luôn 200 với một danh sách (rỗng cho người không tồn tại, người đã xoá tài khoản hay người không chia sẻ gì), để route không thành danh bạ tài khoản.

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`) chạy **trước** validate path và query: người không danh tính gửi id sai dạng kèm `limit=0` vẫn nhận 401 (`anonymous_non_uuid_bad_limit`).
2. Path `person_id` và query `limit` validate cùng lúc; lỗi của cả hai gom **một** mảng 422, path trước (`stranger_non_uuid_and_bad_limit`).
3. **Không có** `_require_permission` và không kiểm người tồn tại: role rỗng vẫn 200 (`author_wall_as_stranger_roles_empty`), id không có ai vẫn 200 (`stranger_unknown_person`).
4. Lọc: SQL `Post.author_id == person_id` AND `_readable_by(reader)` (`services/api/app/api/repository.py:5537-5546`, `:5423-5497`), rồi `post_audience.visible_to` từng dòng (`services/api/app/api/service.py:2165-2195`). Quy tắc giống card `GET /posts`: tác giả đọc hết; bạn (`accepted`) đọc `friends` + `public`; thành viên `ACTIVE` đọc `group` + `public`; người lạ chỉ `public`; chặn hai chiều giấu `public` và `friends` nhưng giữ `group` của nhóm chung; `X-Actor-Contexts` không được đọc.

## Đầu vào

- Path `person_id`: UUID lax (chữ hoa, không gạch nối đều nhận). `/people/me/posts` không có route riêng nên `me` bị parse như id → 422 `uuid_parsing` (`stranger_me_is_not_an_id`).
- Query `limit`: `int`, `ge=1`, `le=100`, mặc định 50 (`services/api/app/api/routes/posts.py:88-106`). `1.0` được nhận (`stranger_limit_float_zero_fraction` → 200); `2.5` → `int_parsing`.
- Không đọc body; GET không qua idempotency.

## Đầu ra

- **200** `PersonPostListResponse` (`services/api/app/api/schemas.py:1693-1703`), thứ tự khoá `person_id`, `posts`.
  - `person_id`: id đã parse, viết lại chữ thường có gạch nối (`stranger_uppercase_unknown_person`, `stranger_unhyphenated_unknown_person`).
  - `posts[]`: `PostResponse` (`schemas.py:1663-1686`), cùng thứ tự khoá, cùng cách tính `reactions` / `my_reactions` / `comment_count` / `can_comment` như `GET /posts` (`_wire_posts`, `service.py:2197-2244`).
  - Thứ tự `created_at DESC, id DESC`, `LIMIT`. Không có tổng, không cursor.
- `created_at`: ISO 8601 UTC `Z`, 6 chữ số. Không float.
- Framework: 307 cho `/people/{id}/posts/` (`location` giữ đúng id gửi lên); 405 + `allow: GET` cho POST.

## Tác dụng phụ

Chỉ đọc: `posts`, `friend_requests`, `memberships`, `people`, `post_reactions`, `post_comments`. Không idempotency, không limiter. Mọi bài trả về có `author_id == person_id`, nên câu trả lời chỉ phụ thuộc bài của người đó, không phụ thuộc bài `public` của người khác trên stack.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143`, `:102-106` |
| 401 | `authentication_required` | `Session is not valid` (prod) | `services/api/app/api/service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `PUT /people/me/interests` | `deps.py:144-163` |
| 422 | (framework) | `uuid_parsing` `["path","person_id"]`; `greater_than_equal` / `less_than_equal` / `int_parsing` `["query","limit"]` | `routes/posts.py:94-98` |

Route không ném `ApiProblem` nào.

## Mã Python

- Route: `services/api/app/api/routes/posts.py:88-106` (`list_person_posts`)
- Service: `services/api/app/api/service.py:2492-2511` (`list_person_posts`), `:2165-2244`
- Repository: `services/api/app/api/repository.py:5537-5546` (`list_person_posts_visible_to`), `:5423-5497` (`_readable_by`), `:5550-5583`
- Domain: `services/api/app/domain/post_audience.py:94-201`

## Test đang phủ

- `services/api/tests/api/test_posts_audience.py`: `test_a_persons_wall_is_filtered_for_the_reader` (317), `test_an_only_me_post_is_absent_from_a_stranger_wall_body_entirely` (353), `test_a_post_in_a_group_the_reader_left_is_gone_from_the_wall` (373)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py`: `test_a_block_hides_the_wall_both_ways_and_the_rows_never_leave_the_database` (42), `test_lifting_a_block_gives_the_wall_back_but_not_the_friendship` (160)
- `services/api/tests/api/test_sessions_and_blocking.py`: `test_a_block_hides_the_wall_both_ways_but_leaves_the_shared_group` (123)

## Kịch bản parity

`parity/scenarios/w2/posts/GET-people-person_id-posts.yaml`, id `w2/posts/get-people-person_id-posts` (58 bước):

- Thứ tự: `anonymous_wall`, `anonymous_non_uuid_bad_limit` (401 trước 422).
- Path và query: `stranger_non_uuid_person`, `stranger_me_is_not_an_id`, `stranger_non_uuid_and_bad_limit` (hai lỗi một mảng), `stranger_limit_zero`, `stranger_limit_101`, `stranger_limit_float_zero_fraction` (200), `stranger_limit_float`, `roles_unknown`.
- Tường rỗng: `stranger_unknown_person`, `stranger_uppercase_unknown_person`, `stranger_unhyphenated_unknown_person`, `stranger_empty_wall_before_posts`.
- Ma trận người đọc trên tường tác giả (bốn audience): `author_wall_as_author`, `author_wall_as_author_limit_1`, `author_wall_as_author_limit_100`, `author_wall_as_friend`, `author_wall_as_mate`, `author_wall_as_stranger`, `author_wall_as_stranger_with_contexts_header`, `author_wall_as_stranger_roles_empty`, `mate_wall_as_author`.
- Chặn hai chiều: `blocker_blocks_author`, `author_wall_as_blocker_after_block` (chỉ còn bài nhóm), `blocker_wall_as_author_after_block` (rỗng), `blocker_wall_as_stranger`, `blocker_wall_as_self`.
- Rời nhóm: `mate_leaves_group`, `author_wall_as_mate_after_leaving`.
- Người đã xoá tài khoản: `gone_posts_public`, `gone_wall_as_stranger_before_deletion`, `gone_deletes_account`, `gone_wall_as_stranger_after_deletion`, `gone_wall_as_self_after_deletion` (200 rỗng).
- Framework: `trailing_slash_redirects` (307), `post_not_allowed` (405 `allow: GET`).

`parity/scenarios/w2/posts/prod-auth.yaml` (auth `prod`): `anonymous_wall` (401 `Missing bearer session`), `reader_wall_of_owner`, `owner_wall_of_owner`, `reader_wall_of_leaver_after_deletion`.

## Chưa phủ / lưu ý cho bản Go

- Không được trả 404 cho người không tồn tại hay 403 cho người không chia sẻ gì: cả hai là 200 rỗng, cùng byte ngoại trừ `person_id`.
- `person_id` phản chiếu dạng đã parse (chữ thường, có gạch nối), không phải chuỗi trong path.
- Parse `limit` lax của pydantic (`1.0` hợp lệ, `2.5` không) phải được tái tạo; `strconv.Atoi` của Go sẽ từ chối `1.0`.
- Lời mời kết bạn đang chờ, cạnh `declined` sau khi gỡ chặn: xem card `GET /posts/{post_id}` (cùng quy tắc, phủ ở đó).
- `prod` không đổi quy tắc đọc; chỉ đổi xác thực.
