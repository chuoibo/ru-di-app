# POST /contexts/{context_id}/memories/{memory_id}/comments

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

F41: viết một bình luận dưới một dòng của tường kỷ niệm, dưới tên chính mình. `author_id` là actor, không phải trường của body. Thân bình luận là dữ liệu riêng của nhóm: không vào log, không lặp lại trong lỗi (`services/api/app/api/service.py:5209-5230`, `services/api/app/api/main.py:318-351`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path và body (`anonymous_non_uuid_both`, `anonymous_invalid_body`).
4. Hai path + body `MemoryCommentCreateRequest` → một mảng 422: `context_id`, `memory_id`, rồi body (`stranger_non_uuid_both_bad_body`); thắng 403 (`stranger_unknown_bad_body`).
5. `_memory_of_member(…, "post_group_memory")` (`service.py:5116-5145`; `services/api/app/domain/permissions.py:423-426`): 403 trước lookup (`stranger_unknown_both`, `stranger_real_memory`, `invitee_comments`, `leaver_comments_after_leaving`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 201 (`owner_roles_group_admin_only`).
6. Memory không thuộc nhóm của path → 404 `memory_not_found` (`mate_unknown_memory`, `mate_other_groups_memory_via_this_context`).

## Đầu vào

- Path `context_id`, `memory_id` (UUID lax).
- Header: `Idempotency-Key` tuỳ chọn; `text/plain` → 422 (`text_plain_content_type`).
- Body `MemoryCommentCreateRequest` (`services/api/app/api/schemas.py:1550-1557`), `extra="forbid"`: `body: StrictStr` 1..2000 code point. Rỗng → `string_too_short` (`body_empty`), số/null → `string_type` (`body_number`, `body_null`), thiếu → `missing` (`body_missing`), `author_id` → `extra_forbidden` (`extra_field_author_id`).
- Không cắt khoảng trắng: chỉ dấu cách vẫn được lưu (`mate_comment_spaces_only` → 201); xuống dòng giữ nguyên (`mate_comment_with_newline`). Cùng nội dung lần hai là dòng mới (`mate_comments_same_text_again`).

## Đầu ra

- **201** `MemoryCommentResponse` (`schemas.py:1560-1575`), thứ tự khoá `id`, `memory_id`, `author_id`, `display_name`, `body`, `created_at`.
  - `display_name`: tên người viết đọc từ `people` sau khi ghi (`service.py:5244-5256`); `null` nếu không có dòng `people` (không tới được, FK).
  - `created_at`: `_now()` Python (`service.py:5228`), pydantic UTC `Z`.
- Check-in cũng nhận bình luận (`mate_comments_on_checkin`).
- Replay: 201 + body đã lưu + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `PUT` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `memory_comments (id, memory_id, author_id, body, created_at)` (`services/api/app/api/repository.py:5373-5386`); CHECK `body_not_blank` (`body <> ''`) và FK `memories.id` ON DELETE CASCADE (`services/api/app/db/models.py:1924-1965`).
- SELECT `memberships`, `memories` + hai truy vấn đếm, `people`.
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 404 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`); body khác → 422 reuse (`idem_reuse_different_body`). `idempotency_keys.response_body` lưu cả thân bình luận.
- 422 validation không có `input`, nên thân bình luận không bị lặp lại trong lỗi (`main.py:318-351`).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5137-5141`, `:504` |
| 404 | `memory_not_found` | `Memory does not exist` | `service.py:5142-5144` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework: `uuid_parsing`, `string_too_short`, `string_too_long`, `string_type`, `missing`, `extra_forbidden`, `model_attributes_type`, `json_invalid`.

## Mã Python

- Route: `services/api/app/api/routes/memories.py:200-217`
- Service: `services/api/app/api/service.py:5209-5230` (`post_memory_comment`), `:5244-5256` (`_wire_memory_comment`), `:5116-5145`
- Domain: `services/api/app/domain/permissions.py:423-426`
- Repository: `services/api/app/api/repository.py:5373-5386`, `:5307-5327`, `:2525-2527`
- Model: `services/api/app/db/models.py:1924-1965`

## Test đang phủ

- `services/api/tests/postgres/test_memory_reactions_postgres.py`: `test_a_comment_cannot_name_its_own_author` (193), `test_a_comment_is_written_under_the_callers_name` (210), `test_a_blank_comment_is_refused` (240), `test_the_check_constraint_itself_refuses_a_blank_body` (250), `test_an_outsider_cannot_comment` (280), `test_the_comment_body_is_not_quoted_back_in_any_refusal` (760), `test_the_comment_body_never_reaches_the_logs` (782), `test_deleting_a_memory_takes_its_hearts_and_comments` (804)
- `services/api/tests/api/test_validation_error_redaction.py` (thân 5800 ký tự không được lặp lại trong 422)

## Kịch bản parity

`parity/scenarios/w3/memories/POST-contexts-context_id-memories-memory_id-comments.yaml`, id `w3/memories/post-contexts-context_id-memories-memory_id-comments` (54 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown`, `anonymous_non_uuid_both`, `anonymous_malformed_json`, `anonymous_empty_idempotency_key`, `anonymous_invalid_body`, `stranger_non_uuid_both_bad_body` (ba lỗi gom một), `stranger_unknown_bad_body` (422 trước 403).
- 403: `stranger_unknown_both`, `stranger_real_memory`, `invitee_comments`, `mate_roles_empty`, `leaver_comments_after_leaving`.
- 404: `mate_unknown_memory`, `mate_other_groups_memory_via_this_context`.
- Đường vui: `mate_comments`, `owner_comments`, `mate_comments_same_text_again`, `mate_comments_on_checkin`, `mate_comment_spaces_only`, `mate_comment_with_newline`, `leaver_comments_before_leaving`, `owner_roles_group_admin_only`, `owner_reads_comments`.
- Validate: `body_empty`, `body_number`, `body_null`, `body_missing`, `extra_field_author_id`, `top_level_array`, `text_plain_content_type`.
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_refusal_not_stored` (404) + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects`, `put_not_allowed`.

`prod` (`w3/memories/prod-auth`): `anonymous_malformed_json_comment` (422), `mate_comments` (201), `idem_first`, `idem_replay`, `stranger_same_key` (403).

Chạy với làn DB bật; `cursor` ở các bước chuẩn bị (`owner_posts_photo`, `owner_checks_in`, `owner_posts_in_other_group`) được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` như ở card `POST /contexts/{id}/memories` (ADR-0029 §2.4). Corpus 422 sinh tự động: `parity/scenarios/generated/w3-422/post-contexts-context_id-memories-memory_id-comments.yaml`, id `generated/w3-422/post-contexts-context_id-memories-memory_id-comments` (76 bước; `MAX_STEPS` của bộ sinh đã nâng lên 80).

## Chưa phủ / lưu ý cho bản Go

- Độ dài 2000 đếm theo code point; bản Go không được đếm byte hay đơn vị UTF-16. Corpus sinh tự động dò các biên độ dài bằng ký tự 3 byte, emoji 4 byte và surrogate escape (các bước `body_len_*`).
- Không strip: bản Go không được thêm kiểm "chỉ khoảng trắng".
- Bản Go không được ghi thân bình luận vào log hay lỗi.
- 409 in-flight chưa phủ.
