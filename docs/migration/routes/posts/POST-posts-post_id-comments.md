# POST /posts/{post_id}/comments

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

ADR-0022 §2.2: người đọc được một bài viết một bình luận dưới bài đó, nếu chủ tường cho phép. Người viết là actor đã xác thực; body chỉ có một trường.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: rỗng → 422 trước xác thực (`anonymous_empty_idempotency_key`); reuse → 422; replay (`services/api/app/api/idempotency.py:404-553`).
2. JSON hỏng → 422 `json_invalid`, trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 thắng lỗi path/body (`anonymous_non_uuid_post_blank_body`).
4. Path `post_id` và body validate cùng lúc, một mảng 422, path trước (`stranger_non_uuid_and_blank_body`). Bài không có + body rỗng → 422 (`stranger_unknown_post_blank_body`).
5. `_readable_post_or_404` (`services/api/app/api/service.py:2246-2270`) → 404 `post_not_found` (ma trận ở card `POST /posts/{post_id}/reactions`). Chạy trước chính sách: bài không đọc được luôn 404 bất kể policy (`stranger_on_only_me_policy_friends`) và bất kể role (`stranger_roles_empty_unreadable`).
6. Chính sách tường của **tác giả bài** (`service.py:2415-2419`; `services/api/app/domain/post_audience.py:94-123`): tác giả luôn được; `readers` → ai đọc được cũng được; `friends` → chỉ bạn đã chấp nhận (thành viên nhóm không phải bạn bị từ chối trên bài `group`, `mate_on_group_policy_friends`); `nobody` → chỉ tác giả. Chính sách của người bình luận không liên quan (`stranger_sets_own_policy_nobody`).
7. `_require_permission("comment_on_post", {"may_comment": allowed})` (`service.py:2420-2425`; `services/api/app/domain/permissions.py:444-447`). **Mọi** từ chối ở đây, kể cả thiếu role, bị đổi thành 403 `comments_closed` với cùng câu: `X-Actor-Roles: advancer` trên bài mở → `comments_closed`, không phải `role_not_permitted` (`stranger_roles_advancer_only`).

## Đầu vào

- Path `post_id` (UUID lax).
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` (vắng → vẫn parse JSON, `no_content_type_is_parsed_as_json` → 201; `text/plain` → 422).
- Body `PostCommentCreateRequest` (`services/api/app/api/schemas.py:1604-1607`), `extra="forbid"`: `body: StrictStr`, độ dài **ký tự** `1..2000`. Không trim: `"   "` được nhận và lưu nguyên (`body_whitespace_only` → 201); CHECK DB chỉ cấm chuỗi rỗng. Không có `author_id` (`extra_field_author_id` → 422).

## Đầu ra

- **201** `PostCommentResponse` (`schemas.py:1610-1620`), thứ tự khoá `id`, `post_id`, `author_id`, `author_display_name`, `body`, `created_at`.
- `body` trả lại **nguyên văn** (`body_needs_escaping`: `<b>&"Quận 9"</b>\ à 🙂` + xuống dòng; Python chỉ thoát `"`, `\` và ký tự điều khiển, giữ UTF-8 thô).
- `author_display_name` đọc từ `people` sau khi ghi (`services/api/app/api/repository.py:5636-5655`). Tài khoản đã xoá ở dev vẫn viết được và mang tên ẩn danh `Người dùng đã rời` (`services/api/app/domain/account_lifecycle.py:55`, `ghost_comments_after_deletion`).
- `created_at`: `_now()` của Python truyền tay (`service.py:2427`), ghi `YYYY-MM-DDTHH:MM:SS.ffffffZ`.
- Không float.
- Replay idempotency: cùng 201, cùng body (cùng `id`, không có dòng thứ hai), thêm `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `DELETE` trên `/posts/{id}/comments` → 405 `allow: GET` (chỉ route khớp đầu tiên, không liệt kê POST).

## Tác dụng phụ

- `post_comments`: INSERT `(id uuid4, post_id, author_id, body, created_at = _now())` (`repository.py:5647-5655`; model `services/api/app/db/models.py:2255-2287`, CHECK `post_comment_body_not_blank`, FK `posts.id` ON DELETE CASCADE).
- SELECT `posts`, `people` (tác giả bài lấy `wall_comment_policy`, `service.py:2415-2416`), `friend_requests`, `memberships`.
- Idempotency: chỉ lưu 201; 403/404/422 nhả key. `idem_comments_closed_not_stored` (403 khi policy `friends`) rồi, sau khi chủ tường mở lại `readers`, **cùng key cùng body** → 201 ghi thật (`idem_same_key_after_policy_opens`). Cùng key khác body → 422 reuse; key khác cùng body → bình luận thứ hai (`idem_second_distinct_key`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | như card `POST /posts/{post_id}/reactions` | `deps.py:147`, `:152-154` |
| 404 | `post_not_found` | `Post does not exist` | `service.py:2258`, `:2269` |
| 403 | `comments_closed` | `Chủ tường không cho bình luận bài này.` | `service.py:2422-2425` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework đã đo: `uuid_parsing`, `string_too_short` (`ctx {"min_length":1}`), `string_too_long` (`ctx {"max_length":2000}`), `string_type` (số, `null`), `missing` (`["body","body"]` hoặc `["body"]`), `extra_forbidden`, `model_attributes_type` (chuỗi top-level, `text/plain`), `json_invalid`.

## Mã Python

- Route: `services/api/app/api/routes/posts.py:180-195` (`post_comment`)
- Service: `services/api/app/api/service.py:2405-2429` (`post_comment`), `:588-596` (`_wire_post_comment`), `:2246-2270`, `:2150-2163`, `:475-504`
- Domain: `services/api/app/domain/post_audience.py:94-123` (`can_comment`), `:126-201`; `services/api/app/domain/permissions.py:444-447`
- Repository: `services/api/app/api/repository.py:5647-5655` (`create_post_comment`), `:5636-5645`, `:2525-2527` (`get_person`)
- Chính sách tường được đặt qua `PATCH /people/me` (`services/api/app/api/schemas.py:911-935`, `service.py:4079-4102`)

## Test đang phủ

- `services/api/tests/api/test_post_comments_reactions.py`: `test_a_reader_comments_and_the_list_runs_oldest_first` (139), `test_the_wall_owner_decides_who_may_comment_and_the_post_says_so` (176), `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` (260)
- `services/api/tests/domain/test_post_comment_policy.py`: `test_the_default_policy_is_one_of_the_three` (32), `test_commenting_is_reading_plus_the_wall_owners_policy` (48), `test_the_author_may_always_comment_on_their_own_post` (69), `test_an_unknown_policy_closes_the_composer` (81)
- `services/api/tests/postgres/test_post_social_postgres.py`: `test_an_unknown_kind_and_an_unknown_policy_are_refused_by_the_database` (65), `test_three_hearts_and_two_comments_read_three_and_two_over_http` (120)

## Kịch bản parity

`parity/scenarios/w2/posts/POST-posts-post_id-comments.yaml`, id `w2/posts/post-posts-post_id-comments` (89 bước):

- Thứ tự từ chối: `anonymous_unknown_post`, `anonymous_non_uuid_post_blank_body` (401), `anonymous_malformed_json`, `anonymous_empty_idempotency_key` (422 trước 401), `stranger_non_uuid_post`, `stranger_non_uuid_and_blank_body`, `stranger_unknown_post`, `stranger_unknown_post_blank_body` (422 trước 404), `stranger_unknown_post_roles_empty` (404).
- Ma trận đọc với policy `readers`: `author_on_only_me`, `stranger_on_only_me`, `friend_on_friends`, `stranger_on_friends`, `mate_on_group`, `friend_on_group`, `stranger_on_public`, `no_content_type_is_parsed_as_json`.
- Body: `body_needs_escaping`, `body_whitespace_only`, `body_at_max_length` (2000 ký tự), `body_over_max_length` (2001), `body_blank`, `body_number`, `body_null`, `missing_body_field`, `extra_field_author_id`, `empty_request_body`, `top_level_string`, `text_plain_content_type`.
- Role: `stranger_roles_advancer_only` (403 `comments_closed`), `stranger_roles_group_admin_only` (201), `stranger_roles_empty_unreadable` (404), `stranger_roles_unknown`.
- Policy `friends`: `author_sets_policy_friends`, `stranger_on_public_policy_friends` (403), `friend_on_public_policy_friends` (201), `mate_on_group_policy_friends` (403), `author_on_public_policy_friends` (201), `stranger_on_only_me_policy_friends` (404).
- Policy `nobody`: `author_sets_policy_nobody`, `friend_on_friends_policy_nobody` (403), `author_on_friends_policy_nobody` (201), `author_sets_policy_readers`, `stranger_sets_own_policy_nobody`.
- Chặn, rời nhóm, xoá tài khoản: `rival_blocks_author`, `rival_on_public_after_blocking` (404), `rival_on_group_after_blocking` (201), `author_on_blockers_public` (404), `mate_leaves_group`, `mate_on_group_after_leaving`, `stranger_on_departed_before`, `departed_deletes_account`, `stranger_on_departed_after`, `ghost_deletes_account`, `ghost_comments_after_deletion`.
- Idempotency: `idem_comments_closed_not_stored` + `idem_same_key_after_policy_opens`, `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_second_distinct_key`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`, `friend_lists_friends_comments`, `author_reads_public_counts` (đọc lại `comment_count`).
- Framework: `trailing_slash_redirects` (307), `delete_not_allowed` (405).

## Chưa phủ / lưu ý cho bản Go

- `prod` không có trong kịch bản này (lát posts core giữ kịch bản prod của nhóm). Ở prod, tài khoản đã xoá bị thu hồi phiên → 401 thay cho `ghost_comments_after_deletion`.
- Nhánh `author is None → policy "nobody"` (`service.py:2416`) không tới được qua HTTP: xoá tài khoản giữ dòng `people` (ẩn danh) và xoá luôn bài của người đó.
- Policy lạ trong DB (fail closed ở `post_audience.py:123`) bị CHECK trên `people.wall_comment_policy` chặn, không tạo được.
- 409 in-flight không tạo được (harness tuần tự).
- Độ dài là số **code point** (pydantic), không phải byte hay UTF-16: emoji ngoài BMP tính là 1. Go phải đếm `utf8.RuneCountInString`.
- Thoát JSON: giữ `<`, `>`, `&` và UTF-8 thô (`SetEscapeHTML(false)`), `\n` ghi là `\n`.
- `comments_closed` nuốt cả `role_not_permitted`; Go không được trả `permission_denied` ở đây.
