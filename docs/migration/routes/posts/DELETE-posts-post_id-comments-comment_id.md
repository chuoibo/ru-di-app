# DELETE /posts/{post_id}/comments/{comment_id}

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

ADR-0022 §2.2: gỡ một bình luận. Chỉ người viết bình luận hoặc tác giả bài được gỡ; không có vai kiểm duyệt. Bình luận của bài khác gọi dưới id bài này là "không có ở đây" (404), không phải gợi ý.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (DELETE là phương thức ghi, `services/api/app/api/idempotency.py:76`): key rỗng → 422 trước xác thực (`anonymous_empty_idempotency_key`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 thắng lỗi path (`anonymous_non_uuid_ids`).
3. Path `post_id` và `comment_id` (UUID lax) validate cùng lúc, một mảng 422, `post_id` trước (`stranger_non_uuid_both`).
4. `_readable_post_or_404` (`services/api/app/api/service.py:2246-2270`) → 404 `post_not_found`. Chạy trước tìm bình luận: bài không đọc được + id bình luận có thật → `post_not_found` (`stranger_unreadable_post_real_comment`).
5. Bình luận không có, hoặc thuộc bài khác → 404 `comment_not_found` (`service.py:2468-2470`; `author_unknown_comment`, `author_comment_of_other_post`). Chạy **trước** kiểm quyền: thiếu role + bình luận không có → 404 (`stranger_roles_advancer_unknown_comment`).
6. `_require_permission("delete_post_comment", {"may_delete_comment": …})` (`service.py:2471-2481`; `services/api/app/domain/permissions.py:448-451`; `services/api/app/domain/post_audience.py:159-162`): thiếu `member`/`group_admin` → 403 `role_not_permitted` (xét trước predicate, `stranger_roles_advancer_own_comment`); không phải người viết bình luận cũng không phải tác giả bài → 403 `may_delete_comment`.

Hệ quả: người viết bình luận mất quyền đọc bài (rời nhóm, chặn tác giả) thì **không gỡ được** bình luận của mình (`mate_deletes_own_after_leaving`, `rival_deletes_own_after_blocking` → 404), còn tác giả bài vẫn gỡ được (`author_deletes_former_mates_comment`, `author_deletes_blockers_comment` → 204).

## Đầu vào

- Path `post_id`, `comment_id` (UUID lax; viết hoa được nhận, `stranger_uppercase_unknown_comment` → 404 `comment_not_found`).
- Header: `Idempotency-Key` tuỳ chọn. Body không được đọc (`stranger_deletes_own_with_ignored_body` → 204) nhưng vẫn vào fingerprint idempotency.
- Không query.

## Đầu ra

- **204**, body rỗng, không `content-type` (`services/api/app/api/routes/posts.py:198-212` trả `Response(status_code=204)`).
- Gỡ lại lần hai → 404 `comment_not_found` (`stranger_deletes_own_again`): không idempotent ở tầng nghiệp vụ, chỉ idempotent qua key.
- Replay idempotency: 204, header `content-length: 0` và `idempotency-replayed: true`, không `content-type` (`idempotency.py:585-599`).
- Framework: 307 cho `/` cuối; GET → 405 `allow: DELETE`.

## Tác dụng phụ

- `post_comments`: `session.get` rồi DELETE dòng (`services/api/app/api/repository.py:5657-5667`). Không soft delete. Bình luận ngừng xuất hiện ở `GET /posts/{id}/comments` và `comment_count` (`author_lists_after_deletes`, `author_lists_at_end`).
- Idempotency: lưu 204; 403/404/422 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_forbidden_not_stored` + `idem_same_key_after_forbidden`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`). Cùng key trên bình luận khác → 422 reuse (`idem_reuse_different_comment`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | như card `POST /posts/{post_id}/reactions` | `deps.py:147`, `:152-154` |
| 404 | `post_not_found` | `Post does not exist` | `service.py:2258`, `:2269` |
| 404 | `comment_not_found` | `Comment does not exist` | `service.py:2470` |
| 403 | `permission_denied` | `role_not_permitted` hoặc `may_delete_comment` | `service.py:502-504` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework đã đo: `uuid_parsing` với `loc ["path","post_id"]` và `["path","comment_id"]` (`msg` ghi ký tự và vị trí lỗi đầu tiên, ví dụ `invalid character: found `u` at 2`).

## Mã Python

- Route: `services/api/app/api/routes/posts.py:198-212` (`delete_post_comment`)
- Service: `services/api/app/api/service.py:2462-2482` (`delete_post_comment`), `:2246-2270`, `:475-504`
- Domain: `services/api/app/domain/post_audience.py:159-162` (`can_delete_comment`), `services/api/app/domain/permissions.py:448-451`
- Repository: `services/api/app/api/repository.py:5657-5659` (`get_post_comment`), `:5661-5667` (`delete_post_comment`)

## Test đang phủ

- `services/api/tests/api/test_post_comments_reactions.py`: `test_a_comment_is_deleted_by_its_author_or_the_posts_author_and_nobody_else` (221), `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` (260)
- `services/api/tests/domain/test_post_comment_policy.py::test_a_comment_is_deleted_by_its_author_or_by_the_posts_author_only` (91)
- `services/api/tests/postgres/test_post_social_postgres.py::test_reactions_and_comments_die_with_their_post` (87)

## Kịch bản parity

`parity/scenarios/w2/posts/DELETE-posts-post_id-comments-comment_id.yaml`, id `w2/posts/delete-posts-post_id-comments-comment_id` (65 bước):

- Thứ tự từ chối: `anonymous_unknown_ids`, `anonymous_non_uuid_ids` (401), `anonymous_empty_idempotency_key` (422 middleware), `stranger_non_uuid_post`, `stranger_non_uuid_comment`, `stranger_non_uuid_both`, `stranger_unknown_post`, `stranger_unknown_post_roles_empty`, `stranger_unreadable_post_real_comment` (`post_not_found` trước `comment_not_found`), `author_unknown_comment`, `author_comment_of_other_post`, `stranger_uppercase_unknown_comment`, `stranger_roles_advancer_unknown_comment` (404 trước 403).
- Quyền: `friend_deletes_strangers_comment`, `stranger_deletes_authors_comment` (403 `may_delete_comment`), `stranger_roles_advancer_own_comment` (403 `role_not_permitted`), `stranger_roles_unknown`, `actor_id_not_uuid`.
- Đường vui: `stranger_deletes_own_with_ignored_body`, `stranger_deletes_own_again` (404), `author_deletes_friends_comment_on_own_post`, `author_lists_after_deletes`, `author_deletes_own_comment`, `author_lists_at_end`.
- Mất quyền đọc: `mate_leaves_group`, `mate_deletes_own_after_leaving` (404), `author_deletes_former_mates_comment` (204), `rival_blocks_author`, `rival_deletes_own_after_blocking` (404), `author_deletes_blockers_comment` (204).
- Idempotency: `idem_first`, `idem_replay` (204 replay), `idem_reuse_different_comment`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_forbidden_not_stored` + `idem_same_key_after_forbidden`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).

## Chưa phủ / lưu ý cho bản Go

- `prod` không có trong kịch bản này (lát posts core giữ kịch bản prod của nhóm).
- Replay 204: Python gửi `content-length: 0`; `net/http` của Go không gửi. Đây đúng là ngoại lệ đã chấp nhận `RESPONSE-204-CONTENT-LENGTH` (ADR-0029 §2.4, `parity/internal/compare/compare.go`), nên `idem_replay` sẽ được so theo luật đó khi Go phục vụ route này.
- Hai lệnh gỡ đồng thời cùng một bình luận (dòng biến mất giữa `get_post_comment` và `delete_post_comment`, `repository.py:5661-5664` trả False im lặng) và 409 in-flight không tạo được (harness tuần tự).
- Bình luận của tài khoản đã xoá không còn để gỡ (`erase_person` xoá chúng, `repository.py:4802-4805`).
- Người viết mất quyền đọc không gỡ được bình luận của chính mình: hành vi hiện tại, ghi lại để bản Go không đổi lén; muốn đổi thì mở ADR.
