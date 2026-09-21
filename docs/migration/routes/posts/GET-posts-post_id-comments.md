# GET /posts/{post_id}/comments

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

ADR-0022 §2.2: đọc bình luận dưới một bài mà người gọi được đọc, cũ nhất trước, từng trang theo keyset `(created_at, id)`; `after` đi tiếp về phía mới hơn.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. Không qua idempotency: GET không phải phương thức ghi (`services/api/app/api/idempotency.py:76`, `:405-407`), header `Idempotency-Key` bị bỏ qua kể cả khi rỗng (`get_with_empty_idempotency_key` → 200).
2. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 thắng lỗi path và query (`anonymous_non_uuid_post`, `anonymous_limit_zero`).
3. Path `post_id` và query `limit` validate cùng lúc, một mảng 422, path trước (`stranger_non_uuid_and_limit_zero`). Chạy trước handler: bài không có + `limit=0` → 422 (`stranger_unknown_post_limit_zero`).
4. `_readable_post_or_404` (`services/api/app/api/service.py:2246-2270`) → 404 `post_not_found` (không có hoặc không được đọc; ma trận ở card `POST /posts/{post_id}/reactions`).
5. Giải mã `after` **sau** bước 4 (`service.py:2440-2447`): bài không có + cursor hỏng → 404 (`stranger_unknown_post_bad_cursor`); bài đọc được + cursor hỏng → 422 `invalid_cursor`.

**Không có kiểm quyền theo role**: `list_post_comments` không gọi `_require_permission`, nên `X-Actor-Roles: ''` vẫn 200 (`stranger_roles_empty_readable`), khác bốn route ghi cùng nhóm.

## Đầu vào

- Path `post_id` (UUID lax).
- Query `limit: int`, `1..100`, mặc định 50 (`services/api/app/api/routes/posts.py:161-177`). `+2` được nhận là 2 (`list_limit_plus_sign`); `1.5`, `abc`, rỗng → `int_parsing`; lặp lại → giá trị **cuối** thắng (`list_limit_repeated_last_wins`).
- Query `after: str | None`: cursor mờ `base64url(created_at.isoformat() + "|" + id)` bỏ `=` (`services/api/app/api/cursors.py:15-19`). Giải mã (`cursors.py:22-46`): thêm padding, `b64decode(validate=True, altchars="-_")`, UTF-8, tách ở `|` đầu tiên, `datetime.fromisoformat`, `uuid.UUID`; giờ không múi → coi là UTC. `after=` rỗng vẫn là "có gửi" → 422. Cursor tự chế hợp lệ được nhận (`after_naive_far_past` → mọi bình luận; `after_far_future` → trang rỗng). Cursor không gắn với bài: dùng cursor của bài A trên bài B chỉ là một vị trí thời gian (`after_foreign_cursor`).
- Query lạ bị bỏ qua (`list_unknown_query_ignored`).

## Đầu ra

- **200** `PostCommentListResponse` (`services/api/app/api/schemas.py:1623-1629`), thứ tự khoá `post_id`, `comments`, `next_cursor`, `has_more`.
- `comments[]`: `PostCommentResponse` (`schemas.py:1610-1620`) khoá `id`, `post_id`, `author_id`, `author_display_name`, `body`, `created_at`. Sắp `created_at, id` tăng dần (`services/api/app/api/repository.py:5669-5700`).
- Phân trang: đọc `limit + 1` dòng; `has_more = len(rows) > limit`; `next_cursor` = cursor của dòng **cuối trang** khi `has_more`, ngược lại `null` (`service.py:2448-2460`). Còn đúng `limit` dòng → `has_more: false`, không có trang rỗng đuôi (`list_limit_equals_count`).
- `author_display_name` đọc **lúc gọi** (`repository.py:2529-2548`): đổi tên thì danh sách đổi theo (`list_after_rename_and_ghost`).
- Datetime `created_at`: giá trị `_now()` của Python lúc ghi (`service.py:2427`), đọc lại từ `timestamptz`, pydantic ghi `YYYY-MM-DDTHH:MM:SS.ffffffZ` (bỏ phần lẻ khi micro giây bằng 0). Trong cursor thì là `isoformat()` với `+00:00`.
- Không float.
- Framework: 307 cho `/` cuối, **giữ query** (`location: http://<Host>/posts/<id>/comments?limit=1`); `HEAD` và `PUT` → 405 `allow: GET` (chỉ liệt kê phương thức của route khớp đầu tiên, không có `POST` dù route POST cùng path tồn tại); HEAD không body.

## Tác dụng phụ

Chỉ đọc: `posts`, `people`, `friend_requests`, `memberships`, `post_comments`. Không idempotency, không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | như card `POST /posts/{post_id}/reactions` | `deps.py:147`, `:152-154` |
| 404 | `post_not_found` | `Post does not exist` | `service.py:2258`, `:2269` |
| 422 | `invalid_cursor` | `Comment cursor is invalid` | `service.py:2442-2447` |

422 framework đã đo: `uuid_parsing` (`["path","post_id"]`), `greater_than_equal` (`ctx {"ge":1}`), `less_than_equal` (`ctx {"le":100}`), `int_parsing` (`["query","limit"]`, không có `ctx`).

## Mã Python

- Route: `services/api/app/api/routes/posts.py:161-177` (`list_post_comments`)
- Service: `services/api/app/api/service.py:2431-2460` (`list_post_comments`), `:588-596` (`_wire_post_comment`), `:2246-2270`
- Cursor: `services/api/app/api/cursors.py:11-46`
- Repository: `services/api/app/api/repository.py:5669-5700` (`list_post_comments`), `:2529-2548` (`_display_names`)
- Model: `services/api/app/db/models.py:2255-2287` (index `ix_post_comments_post (post_id, created_at, id)`)

## Test đang phủ

- `services/api/tests/api/test_post_comments_reactions.py`: `test_a_reader_comments_and_the_list_runs_oldest_first` (139), `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` (260)
- `services/api/tests/postgres/test_post_social_postgres.py`: `test_reactions_and_comments_die_with_their_post` (87), `test_three_hearts_and_two_comments_read_three_and_two_over_http` (120)

## Kịch bản parity

`parity/scenarios/w2/posts/GET-posts-post_id-comments.yaml`, id `w2/posts/get-posts-post_id-comments` (90 bước):

- Thứ tự từ chối: `anonymous_unknown_post`, `anonymous_non_uuid_post`, `anonymous_limit_zero` (401), `stranger_non_uuid_post`, `stranger_non_uuid_and_limit_zero` (path + query gom một), `stranger_unknown_post`, `stranger_unknown_post_limit_zero` (422 trước 404), `stranger_unknown_post_bad_cursor` (404 trước `invalid_cursor`).
- Trang: `author_empty_list`, `list_default`, `list_limit_one` (bind `cursor_one`), `list_after_one_limit_one` (bind `cursor_two`), `list_after_two_limit_two` (trang cuối), `list_limit_three` (bind `cursor_three`), `list_after_three`, `list_limit_equals_count`, `list_limit_two_repeats_cursor_two`, `list_limit_max`.
- Query: `list_limit_over_max`, `list_limit_zero`, `list_limit_negative`, `list_limit_not_int`, `list_limit_float`, `list_limit_empty`, `list_limit_repeated_last_wins`, `list_limit_plus_sign`, `list_unknown_query_ignored`.
- Cursor: `after_malformed`, `after_empty`, `after_without_separator`, `after_bad_uuid_inside`, `after_bad_timestamp_inside`, `after_naive_far_past`, `after_far_future`, `after_foreign_cursor`.
- Ma trận đọc: `stranger_on_only_me`, `stranger_on_friends`, `friend_on_friends`, `friend_on_group`, `mate_on_group`, `author_on_only_me`, `rival_on_public_after_blocking` (404), `rival_on_group_after_blocking` (200), `mate_on_group_after_leaving`, `stranger_on_departed_before`/`_after`.
- Không role: `stranger_roles_empty_readable`, `stranger_roles_unknown`, `actor_id_not_uuid`.
- Tên và xoá tài khoản: `friend_renames`, `ghost_comments_public`, `list_after_rename_and_ghost`, `ghost_deletes_account`, `list_after_ghost_deleted` (bình luận của người xoá tài khoản biến mất), `ghost_reads_after_deletion` (dev vẫn đọc được).
- Framework: `get_with_empty_idempotency_key`, `trailing_slash_redirects` (307 giữ query), `head_not_allowed`, `put_not_allowed` (405).

## Chưa phủ / lưu ý cho bản Go

- `prod` không có trong kịch bản này (lát posts core giữ kịch bản prod của nhóm).
- Cursor: harness bind `next_cursor` như token nên chỉ so **vị trí** xuất hiện, không so byte của cursor giữa hai stack. Bản Go phải tự khớp `created_at.isoformat()` của Python: `+00:00` (không phải `Z`), bỏ `.ffffff` khi micro giây bằng 0, rồi base64url bỏ `=`. Cần test vector riêng.
- Bộ giải mã chấp nhận cả bảng chữ base64 chuẩn (`+`, `/`) vì `altchars` chỉ dịch thêm `-`/`_`, và chấp nhận timestamp có múi khác UTC. Chưa có bước đo cho hai điều này.
- Hai bình luận cùng `created_at` đến micro giây sẽ xếp theo `id` ngẫu nhiên; kịch bản ghi từng bình luận ở request riêng nên không gặp.
- 405 cho `/comments` chỉ ghi `allow: GET` dù có route POST cùng path: Starlette dừng ở route khớp một phần đầu tiên. Go phải tái tạo đúng, không gộp `GET, POST`.
- 307 giữ nguyên query string trong `location`.
- Không kiểm role ở route đọc: bản Go không được thêm `_require_permission` ở đây.
