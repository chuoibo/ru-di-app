# POST /posts/{post_id}/reactions

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

ADR-0022 §2.2: người đọc được một bài để lại **một** cảm xúc thuộc một trong sáu loại. Bấm lần hai cùng loại vẫn là một dòng và vẫn trả 200. Câu trả lời là tổng đếm lại từ các dòng sau khi ghi, không phải số được cộng dồn.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: rỗng hoặc dài hơn 255 → 422 trước cả xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`); key đã dùng cho request khác → 422; đã có kết quả → replay (`services/api/app/api/idempotency.py:404-553`).
2. JSON hỏng → 422 `json_invalid`, **trước** xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): dev thiếu `X-Actor-ID` → 401; không phải UUID → 422 `invalid_actor_id`; role lạ → 422 `invalid_actor_roles`. 401 thắng lỗi path (`anonymous_non_uuid_post`).
4. Path `post_id` **và** body được validate cùng lúc, gom vào **một** mảng 422, lỗi path đứng trước (`stranger_non_uuid_and_bad_kind`). Vì chạy trước handler, bài không tồn tại + `kind` sai → 422 chứ không 404 (`stranger_unknown_post_bad_kind`).
5. `_readable_post_or_404` (`services/api/app/api/service.py:2246-2270`): không có bài → 404; có bài nhưng `post_audience.visible_to` từ chối → **cùng** 404. Hai sự thật đọc từ bảng lúc gọi: `is_friend` (cạnh `accepted`, `service.py:2127-2136`) và `is_group_member` (membership ACTIVE, `left_at` null, `services/api/app/api/repository.py:2769-2782`); thêm `is_blocked` (cạnh `blocked` bất kể chiều, `service.py:2138-2148`). `X-Actor-Contexts` không được tin (`stranger_claims_group_context` → 404).
6. `_require_permission("react_to_post", {"may_read_post": True})` (`service.py:2388`; bảng quyền `services/api/app/domain/permissions.py:440-443`, role `group_admin` hoặc `member`). Predicate luôn đúng nên chỉ còn lý do `role_not_permitted`. Chạy **sau** bước 5: role rỗng trên bài không đọc được → 404 (`stranger_unknown_post_roles_empty`, `stranger_roles_empty_unreadable`), trên bài đọc được → 403 (`stranger_roles_advancer_only`). Chỉ `group_admin` là đủ (`stranger_roles_group_admin_only` → 200).

Ma trận đọc (`services/api/app/domain/post_audience.py:126-201`): tác giả luôn đọc được bài của mình, kể cả `only_me`; `public` → mọi người; `friends` → chỉ bạn đã chấp nhận (lời mời đang chờ không tính, `pending_on_friends`); `group` → chỉ thành viên ACTIVE của đúng nhóm đó (được mời chưa nhận và đã rời đều 404); `friends` và `group` rời nhau (`mate_on_friends`, `friend_on_group`). Chặn (theo bất kỳ chiều nào) ẩn bài `public`/`friends` cả hai chiều, nhưng bài `group` trong nhóm cả hai còn ở vẫn đọc được (`rival_on_group_after_blocking` → 200).

## Đầu vào

- Path `post_id` (UUID lax: viết hoa, bỏ gạch, có ngoặc nhọn `%7B…%7D` đều được chấp nhận và đi tới handler → 404; thiếu một ký tự → 422 `uuid_parsing`).
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` (không có vẫn parse JSON → 200 `no_content_type_is_parsed_as_json`; `text/plain` → 422 `model_attributes_type`).
- Body `PostReactionRequest` (`services/api/app/api/schemas.py:1592-1593`), `extra="forbid"` (`schemas.py:66-67`): `kind: Literal["heart","haha","like","wow","sad","fire"]` (`schemas.py:1584`), bắt buộc, phân biệt hoa thường. Số hoặc `null` cũng ra `literal_error` (không phải `string_type`).

## Đầu ra

- **200** (POST nhưng không phải 201) `PostReactionsResponse` (`schemas.py:1596-1601`), thứ tự khoá `post_id`, `reactions`, `my_reactions`.
  - `reactions[]`: `{"kind","count"}` cho mỗi loại có ít nhất một dòng, **sắp theo tên loại** (`sorted(per_kind)`, `service.py:2372-2377`), không theo thứ tự thêm: `fire, haha, heart, like, sad, wow`. Loại không có dòng không xuất hiện (không có `count: 0`).
  - `my_reactions[]`: các loại của **chính người gọi** trên bài này, sắp theo tên (`service.py:2378`).
  - Đếm bằng ba truy vấn nhóm (`repository.py:5550-5583`). Dòng của người đã chặn, của người mất quyền đọc vẫn được đếm (`author_sees_rivals_like_still_counted` ở kịch bản DELETE).
- Không float, không datetime trong câu trả lời.
- Replay idempotency: cùng 200 và cùng body đã lưu, thêm `idempotency-replayed: true`; body chuẩn hoá JSON (khoảng trắng) vẫn replay (`idem_replay_reordered_spaces`).
- Framework: 307 cho `/` cuối (`location: http://<Host>/posts/<id>/reactions`); 405 + `allow: POST` cho GET và PUT.

## Tác dụng phụ

- `post_reactions`: INSERT `(id uuid4, post_id, person_id, kind, created_at = _now())` trong savepoint (`repository.py:5585-5618`). Bấm lại cùng loại: INSERT vi phạm `uq_post_reactions_one_per_kind` → bắt `IntegrityError`, đọc dòng cũ, không ghi gì (`services/api/app/db/models.py:2224-2252`). Ràng buộc khác: CHECK `post_reaction_kind_known`, FK `posts.id` ON DELETE CASCADE, FK `people.id`.
- Commit trước khi gửi response (như mọi route qua `get_repository`, `deps.py:196-209`).
- Idempotency: có key thì middleware đặt chỗ trong `idempotency_keys`, chỉ lưu câu trả lời 2xx; 401/404/422 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`). Scope là `X-Actor-ID` thô (`idem_same_key_other_actor` → ghi thật, không reuse). Fingerprint gồm path, nên cùng key trên bài khác → 422 reuse (`idem_reuse_different_post`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:147` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:152-154` |
| 404 | `post_not_found` | `Post does not exist` (không tồn tại **và** không được đọc) | `service.py:2258`, `:2269` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:502-504` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework đã đo (không có `input`, `services/api/app/api/main.py:318-351`): `uuid_parsing` (`loc ["path","post_id"]`), `literal_error` (`ctx.expected` = `'heart', 'haha', 'like', 'wow', 'sad' or 'fire'`), `missing` (`["body","kind"]`, hoặc `["body"]` khi body rỗng), `extra_forbidden`, `model_attributes_type` (mảng top-level, `text/plain`), `json_invalid`. Lỗi middleware có khoảng trắng sau `:` và `,`; lỗi route thì gọn.

## Mã Python

- Route: `services/api/app/api/routes/posts.py:127-141` (`react_to_post`)
- Service: `services/api/app/api/service.py:2381-2392` (`react_to_post`), `:2367-2379` (`_post_reactions_response`), `:2246-2270` (`_readable_post_or_404`), `:2150-2163` (`_post_facts`), `:2127-2148`, `:556-562` (`_post_dict`), `:475-504` (`_require_permission`)
- Domain: `services/api/app/domain/post_audience.py:126-156` (`visible_to`), `:165-201` (`can_read`); `services/api/app/domain/blocking.py:40-42`, `:58-64`
- Repository: `services/api/app/api/repository.py:5522-5524` (`get_post`), `:5550-5583` (`post_social_counts`), `:5585-5618` (`add_post_reaction`), `:7424-7428` (`get_friend_edge`), `:2769-2782` (`is_member`)
- Model: `services/api/app/db/models.py:2215-2252`

## Test đang phủ

- `services/api/tests/api/test_post_comments_reactions.py`: `test_a_reader_reacts_once_per_kind_and_the_post_recounts_from_the_rows` (83), `test_an_unknown_kind_is_refused_at_the_wire` (124), `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` (260)
- `services/api/tests/postgres/test_post_social_postgres.py`: `test_one_person_reacts_once_per_kind` (39), `test_an_unknown_kind_and_an_unknown_policy_are_refused_by_the_database` (65), `test_reactions_and_comments_die_with_their_post` (87), `test_three_hearts_and_two_comments_read_three_and_two_over_http` (120), `test_the_same_tap_twice_over_http_is_one_row_and_answers_200_both_times` (191)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py::test_a_block_hides_the_wall_both_ways_and_the_rows_never_leave_the_database` (42) cho luật chặn chung

## Kịch bản parity

`parity/scenarios/w2/posts/POST-posts-post_id-reactions.yaml`, id `w2/posts/post-posts-post_id-reactions` (103 bước):

- Thứ tự từ chối: `anonymous_unknown_post`, `anonymous_non_uuid_post` (401 trước 422 path), `anonymous_malformed_json` (422 trước 401), `anonymous_empty_idempotency_key` (422 middleware trước 401), `anonymous_with_key_not_stored`, `stranger_non_uuid_post`, `stranger_non_uuid_and_bad_kind` (hai lỗi gom một), `stranger_unknown_post_bad_kind` (422 trước 404), `stranger_unknown_post_roles_empty` (404 trước 403).
- UUID lax: `stranger_uppercase_unknown_post`, `stranger_unhyphenated_unknown_post`, `stranger_braced_unknown_post`, `stranger_short_uuid`, `stranger_unknown_post`.
- Ma trận đọc: `author_heart_only_me`, `friend_on_only_me`, `friend_on_friends`, `stranger_on_friends`, `pending_on_friends`, `mate_on_group`, `mate_on_friends`, `friend_on_group`, `invitee_not_yet_active_on_group`, `stranger_claims_group_context`, `stranger_on_only_me`, `mate_on_group_after_leaving`.
- Đếm: `stranger_like_public`, `friend_heart_public`, `stranger_heart_public`, `stranger_heart_public_again` (lần hai cùng loại), `author_every_kind_haha`/`_sad`/`_fire` (thứ tự theo tên), `no_content_type_is_parsed_as_json`, `author_reads_public_after` (đọc lại qua `GET /posts/{id}`).
- Role: `stranger_roles_advancer_only` (403), `stranger_roles_group_admin_only` (200), `stranger_roles_empty_unreadable` (404), `stranger_roles_unknown`, `actor_id_not_uuid`.
- Validate: `kind_unknown`, `kind_wrong_case`, `kind_number`, `kind_null`, `missing_kind`, `extra_field_person_id`, `empty_body`, `top_level_array`, `text_plain_content_type`.
- Chặn: `rival_blocks_author`, `author_blocks_shunned`, `rival_on_public_after_blocking`, `author_on_blockers_public`, `rival_on_group_after_blocking`, `shunned_on_public`, `rival_on_own_public`.
- Tài khoản đã xoá: `stranger_on_departed_before`, `departed_deletes_account`, `stranger_on_departed_after` (bài bị xoá cùng tài khoản → 404); `ghost_heart_public_before`, `ghost_deletes_account`, `ghost_heart_public_after` (dev vẫn tin header → 200, dòng cũ đã bị xoá).
- Idempotency: `idem_first`, `idem_replay`, `idem_replay_reordered_spaces`, `idem_reuse_different_body`, `idem_reuse_different_post`, `idem_same_key_other_actor`, `idem_second_distinct_key`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed`, `put_not_allowed` (405).

## Chưa phủ / lưu ý cho bản Go

- `prod` không nằm trong kịch bản này (kịch bản prod của nhóm posts do lát posts core viết). Trong prod, tài khoản đã xoá bị thu hồi phiên nên `ghost_heart_public_after` sẽ là 401; ở dev header vẫn được tin và người đã xoá vẫn ghi được cảm xúc.
- 409 in-flight và cuộc đua hai lần bấm đồng thời (nhánh `IntegrityError` khi hai transaction cùng chèn) không tạo được vì harness chạy tuần tự. Lần bấm lại tuần tự **có** đi qua nhánh `IntegrityError` (không có kiểm tra trước), nên bản Go phải cho ra đúng 200 không đổi, không 409/500.
- Harness so HTTP và delta DB. Việc INSERT nằm trong savepoint (lần bấm lại không để lại dòng) được delta DB nhìn thấy; thời điểm `created_at` chỉ so theo thứ hạng.
- Thứ tự `reactions[]` là thứ tự byte của tên loại (`sorted`), không phải thứ tự khai báo trong `Literal` hay thứ tự SQL.
- `literal_error` cho `kind` số/`null`: Go phải phát cùng `type`, `msg` và `ctx.expected` (chuỗi có dấu nháy đơn và chữ `or`).
- 403 `role_not_permitted` chỉ tới được khi header role thiếu `member` và `group_admin`; ở prod mọi người đăng nhập đều có `member` (`repository.py:3629`), nên nhánh này chỉ còn ở dev.
- Không có nhánh 404 nào nói "bài có tồn tại": bản Go không được đổi thành 403 cho bài không đọc được.
