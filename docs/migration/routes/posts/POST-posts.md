# POST /posts

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

F39/F42: người gọi viết một bài dưới tên chính mình, gửi tới một trong bốn audience `only_me`, `friends`, `group`, `public`. Body không có trường tác giả, không có danh sách người nhận; `context_id` chỉ có nghĩa khi audience là `group`; `image_url` là ảnh cá nhân của chính người viết hoặc ảnh của đúng nhóm được gửi tới.

## Xác thực và quyền

Thứ tự từ ngoài vào (đo trên stack tham chiếu):

1. `IdempotencyMiddleware`, chỉ khi có header `Idempotency-Key` (POST nằm trong `WRITE_METHODS`, `services/api/app/api/idempotency.py:76`, `:404-439`): rỗng hoặc dài hơn 255 → 422 `invalid_idempotency_key` trước routing và xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 `json_invalid`, **trước** xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`):
   - `dev`: thiếu `X-Actor-ID` → 401; không phải UUID → 422 `invalid_actor_id`; role lạ → 422 `invalid_actor_roles`.
   - `prod`: `Authorization: Bearer` (`deps.py:93-107`, `:131-140`), bỏ qua `X-Actor-*`; thiếu hoặc sai scheme → 401 `Missing bearer session`; phiên không tồn tại, hết hạn, bị thu hồi hoặc người đã xoá tài khoản → 401 `Session is not valid` (`services/api/app/api/service.py:4462-4468`, `services/api/app/api/repository.py:3614-3621`).
4. Validate body `PostCreateRequest`. Dependency giải trước body, nên người không danh tính gửi JSON hợp lệ nhưng sai trường vẫn nhận **401**, không 422 (`anonymous_bad_fields_valid_json`).
5. `_require_permission("create_post", actor, {})` (`service.py:2275`; `services/api/app/domain/permissions.py:478`: role `group_admin` hoặc `member`, không predicate) → 403 `role_not_permitted`. Chạy **trước** kiểm cặp audience/context: không role và `{"audience":"group"}` thiếu nhóm vẫn là 403 (`roles_empty_bad_pairing`). Ở `dev`, header `X-Actor-Roles: group_admin` một mình cũng đủ (`roles_group_admin_only` → 201).
6. `post_audience.check_writable` (`service.py:2276-2279`; `services/api/app/domain/post_audience.py:71-87`) → 422 `group_audience_needs_context` (group mà `context_id` vắng hoặc `null`) hoặc `context_not_addressable` (audience khác group mà có `context_id`). Chạy **trước** mọi lần đọc roster: `public` kèm id nhóm không tồn tại → 422, không 403 (`author_public_names_unknown_group`).
7. Chỉ khi audience là `group`: `_require_permission("address_post_to_group", ..., {"is_group_member": repository.is_member(context_id, actor.id)})` (`service.py:2281-2293`; `permissions.py:487-490`; `repository.py:2769-2782`, chỉ membership `ACTIVE` và `left_at IS NULL`) → 403 `is_group_member`. Nhóm không tồn tại, người lạ, người mới được mời (INVITED) đều là cùng 403. `X-Actor-Contexts` không được đọc (`stranger_group_with_contexts_header`).
8. Chỉ khi có `image_url` (`service.py:2295-2297`, `_post_photo_url` `:2311-2348`):
   - Ảnh nhóm `/contexts/{id}/photos/{id}`: audience khác `group`, hoặc id nhóm trong url khác `context_id` → 422 `photo_not_addressable`; sau đó kiểm `address_post_to_group` lần nữa cho nhóm trong url rồi trả url, **không kiểm ảnh có tồn tại** (`image_url_group_photo_same_group_uppercase` → 201).
   - Ảnh cá nhân `/people/{id}/photos/{id}`: id chủ ảnh khác người gọi → 403 `permission_denied` với detail là câu tiếng Việt (không phải tên predicate); ảnh không có trong `uploaded_images` của người gọi (`repository.get_person_image`) → 404 `photo_not_found`.
9. INSERT. Ở `dev`, người gọi chưa có dòng `people` làm vỡ khoá ngoại `fk_posts_author` → **500** `Internal Server Error` dạng `text/plain` (`ghost_unregistered_author_500`).

Route không có 404 cho chính bài. Người đã xoá tài khoản: `dev` vẫn đăng được (dòng `people` ẩn danh vẫn thoả khoá ngoại, `leaver_posts_after_deletion` → 201 với `author_display_name` = `Người dùng đã rời`); `prod` bị 401 vì phiên đã thu hồi (`prod-auth` · `leaver_posts_after_deletion`).

## Đầu vào

- Không path param, không query.
- Header: `Idempotency-Key` tuỳ chọn (1..255); `Content-Type` (`text/plain` → 422 `model_attributes_type`; không có header thì FastAPI vẫn parse JSON, `author_no_content_type` → 201).
- Body `PostCreateRequest` (`services/api/app/api/schemas.py:1632-1660`), `extra="forbid"` (`schemas.py:66-67`), thứ tự trường:
  1. `body`: `StrictStr`, 1..5000 **ký tự** (code point: 5000 chữ `ạ` → 201, 5001 → 422 `string_too_long`). Một dấu cách là hợp lệ (CHECK `body <> ''`, `services/api/app/db/models.py:2318`). Số → 422 `string_type`.
  2. `audience`: `Literal["only_me","friends","group","public"]`, phân biệt hoa thường (`Public` → 422 `literal_error`).
  3. `context_id`: `UUID | None = None`, UUID lax; `null` tường minh bằng vắng mặt.
  4. `image_url`: `PostPhotoUrl | None` (`schemas.py:51-58`): regex hai dạng url, hex hoa hay thường đều khớp; sai dạng → 422 `string_pattern_mismatch` (msg và `ctx.pattern` chép nguyên regex).
- Không có `author_id`: gửi lên → 422 `extra_forbidden`.

## Đầu ra

- **201** `PostResponse` (`schemas.py:1663-1686`), thứ tự khoá: `id`, `author_id`, `audience`, `context_id`, `body`, `image_url`, `created_at`, `author_display_name`, `reactions`, `my_reactions`, `comment_count`, `can_comment` (dựng ở `_wire_posts`, `service.py:2197-2244`, gọi từ `:2309`).
  - Bài mới: `reactions: []`, `my_reactions: []`, `comment_count: 0`, `can_comment: true` (tác giả luôn được bình luận bài mình, `post_audience.py:117-118`, bất kể `wall_comment_policy`).
  - `context_id`: chuỗi UUID chữ thường hoặc `null`. `image_url`: `PhotoRef.url` (`services/api/app/domain/photo_ref.py:44-48`), tức id đã parse lại thành chữ thường (`AAAAAAAA-…` gửi lên thành `aaaaaaaa-…`).
  - `author_display_name`: `people.display_name` hiện tại, `""` nếu không còn dòng người.
  - `created_at`: `_now()` của Python (`service.py:406-407`, truyền ở `:2307`; cột có `server_default now()` ở `models.py:2372-2374` nhưng không được dùng). Wire: ISO 8601 UTC hậu tố `Z`, 6 chữ số micro giây.
- Không có float.
- JSON gọn (không khoảng trắng), UTF-8 thô: tiếng Việt và emoji không thoát; `<`, `>`, `&` không thoát; chỉ `"` và `\` được thoát (`author_public_escaping`).
- Replay idempotency: cùng 201 và cùng body, header `content-length`, `idempotency-replayed: true`, `content-type` theo thứ tự đó (`idempotency.py:585-599`).
- 500 (`ghost_unregistered_author_500`): body `Internal Server Error`, `content-type: text/plain; charset=utf-8`.
- Framework: 307 cho `/posts/` (`location: http://<Host>/posts`); 405 `{"detail":"Method Not Allowed"}` + `allow: POST` cho `PUT` (và `DELETE`): Starlette trả `allow` của route **đầu tiên** khớp path, là route POST vì nó được khai trước GET.

## Tác dụng phụ

- INSERT `posts` (`repository.py:5499-5520`), id `uuid4` sinh ở Python. Đọc `memberships` (audience group), `uploaded_images` (ảnh cá nhân), `people` (tên, policy), `post_reactions` và `post_comments` (đếm, luôn rỗng với bài mới, `repository.py:5550-5583`).
- Commit trước khi gửi response (`deps.py:196-209`).
- Idempotency: có key thì đặt chỗ trong `idempotency_keys`, chỉ lưu câu trả lời 2xx; câu trả lời 4xx được nhả key để dùng lại (`idempotency.py:504-546`; `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_forbidden_not_stored` + `idem_same_key_after_forbidden`). Scope: digest bearer, nếu không có thì chuỗi `X-Actor-ID` thô, nếu không thì `anonymous` (`idempotency.py:447-449`); cùng key của người khác là scope khác (`idem_same_key_other_actor` → 201 bài mới). Fingerprint là JSON đã chuẩn hoá: đổi thứ tự khoá và khoảng trắng vẫn replay (`idem_replay_reordered_keys`).
- Không có limiter.
- Bài `public` lập tức nằm trong `GET /posts` của **mọi** người (xem card `GET /posts`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143`, `:102-106` |
| 401 | `authentication_required` | `Session is not valid` (prod) | `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:147` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:152-154` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:502-504` |
| 422 | `group_audience_needs_context` | `A post shared with a group must name the group` | `service.py:520-524`, `:2279` |
| 422 | `context_not_addressable` | `Only a group post may name a group` | như trên |
| 403 | `permission_denied` | `is_group_member` | `service.py:2285-2293` |
| 422 | `photo_not_addressable` | `Ảnh của nhóm chỉ đăng được cho chính nhóm đó.` | `service.py:2326-2331` |
| 403 | `permission_denied` | `Chỉ đăng được ảnh của chính mình.` | `service.py:2342-2345` |
| 404 | `photo_not_found` | `Photo does not exist` | `service.py:2346-2347` |
| 500 | (không JSON) | `Internal Server Error` | khoá ngoại `fk_posts_author`, `models.py:2353-2357` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

- Không tới được qua HTTP: `unknown_audience` (`post_audience.py:80-81`, schema `Literal` chặn trước); `photo_url_invalid` (`service.py:2319-2324`, regex `PostPhotoUrl` chặn trước với 422 `string_pattern_mismatch`); 403 `is_group_member` lần hai trong `_post_photo_url` (`service.py:2332-2340`) vì id nhóm trong url phải bằng `context_id` đã được kiểm ở bước 7.
- 422 framework đã đo: `json_invalid`, `missing` (`["body"]` khi body rỗng; `["body","body"]` và `["body","audience"]` gom một mảng cho `{}`), `string_too_short`, `string_too_long`, `string_type`, `literal_error` (có `ctx.expected`), `uuid_parsing` (`["body","context_id"]`), `string_pattern_mismatch`, `extra_forbidden`, `model_attributes_type` (mảng top-level hoặc `text/plain`). Không có `input` (`services/api/app/api/main.py:318-351`). Lỗi của middleware có khoảng trắng sau `:` và `,` (`idempotency.py:602-618`); lỗi route thì gọn (`main.py:296-316`).

## Mã Python

- Route: `services/api/app/api/routes/posts.py:50-69` (`create_post`)
- Service: `services/api/app/api/service.py:2272-2309` (`create_post`), `:2311-2348` (`_post_photo_url`), `:2197-2244` (`_wire_posts`), `:520-524` (`_AUDIENCE_DETAIL`), `:475-504` (`_require_permission`)
- Domain: `services/api/app/domain/post_audience.py:44` (`AUDIENCES`), `:71-87` (`check_writable`), `:94-123` (`can_comment`); `services/api/app/domain/photo_ref.py:37-74`; `services/api/app/domain/permissions.py:478`, `:487-490`
- Repository: `services/api/app/api/repository.py:5499-5520` (`create_post`), `:2769-2782` (`is_member`), `:4560` (`get_person_image`), `:5550-5583` (`post_social_counts`)
- Schema: `services/api/app/api/schemas.py:1632-1686`, `:51-58`
- Model: `services/api/app/db/models.py:2290-2374` (`Post`, CHECK `body_not_blank`, `audience_matches_target`)
- Middleware: `services/api/app/api/idempotency.py:404-618`

## Test đang phủ

- `services/api/tests/api/test_posts_audience.py`: `test_the_author_is_the_actor_and_cannot_be_named` (76), `test_a_created_post_is_attributed_to_the_caller` (90), `test_a_group_post_must_name_a_group` (95), `test_a_non_group_post_may_not_name_a_group` (104), `test_an_unknown_audience_is_refused` (114), `test_posting_to_a_group_one_is_not_in_is_refused` (123), `test_the_membership_header_does_not_grant_membership` (133)
- `services/api/tests/postgres/test_posts_postgres.py`: `test_the_database_refuses_a_group_post_with_no_group` (274), `test_the_database_refuses_a_non_group_post_that_names_a_group` (295), `test_the_database_refuses_an_empty_body` (316), `test_the_membership_header_buys_nothing_over_http` (338), `test_the_written_post_is_attributed_to_the_caller` (457)
- `services/api/tests/postgres/test_person_photo_gate_postgres.py`: `test_the_gate_follows_the_post_and_closes_when_the_post_is_gone` (68), `test_the_checks_refuse_a_purpose_that_does_not_match_its_owner` (147)
- `services/api/tests/domain/test_post_audience.py` (246-264): `check_writable`

## Kịch bản parity

`parity/scenarios/w2/posts/POST-posts.yaml`, id `w2/posts/post-posts` (69 bước):

- Thứ tự từ chối: `anonymous_valid_body` (401), `anonymous_bad_fields_valid_json` (401 trước 422 body), `anonymous_malformed_json` (422 trước 401), `anonymous_empty_idempotency_key` (422 middleware trước 401), `roles_empty_bad_pairing` (403 role trước 422 cặp), `author_public_names_unknown_group` (422 cặp trước roster), `stranger_group_photo_unknown_group` (403 roster trước ảnh).
- Đường vui: `author_only_me`, `author_friends` (`context_id` và `image_url` null tường minh), `author_public_escaping`, `author_body_single_space`, `author_body_at_max_length`, `author_no_content_type`, `author_group`, `mate_posts_to_group`, `stranger_only_me`, `roles_group_admin_only`.
- Cặp audience/context: `author_group_no_context`, `author_group_null_context`, `author_only_me_names_group`.
- Roster: `mate_invited_posts_to_group` (INVITED → 403), `mate_accepts`, `stranger_posts_to_group`, `stranger_group_with_contexts_header`, `stranger_unknown_group`.
- Danh tính và role: `roles_empty`, `roles_guest_only`, `roles_unknown`, `actor_id_not_uuid`, `ghost_unregistered_author_500`, `leaver_deletes_account` + `leaver_posts_after_deletion`.
- Validate body: `author_body_over_max_length`, `body_empty_string`, `body_number`, `audience_missing`, `audience_unknown`, `context_id_not_uuid`, `extra_author_id_and_empty_body` (hai lỗi một mảng), `empty_object`, `empty_body`, `top_level_array`, `text_plain_content_type`.
- Ảnh: `image_url_bad_shape`, `image_url_other_persons_photo` (403 câu tiếng Việt), `image_url_own_photo_missing`, `image_url_own_photo_uppercase_missing` (404), `image_url_group_photo_on_public`, `image_url_group_photo_of_another_group` (422), `image_url_group_photo_same_group_uppercase` (201, url chữ thường).
- Framework: `trailing_slash_redirects` (307), `put_not_allowed` (405 `allow: POST`).
- Idempotency: `idem_first`, `idem_replay`, `idem_replay_reordered_keys`, `idem_reuse_different_body`, `idem_same_key_other_actor`, `idem_second_distinct_key`, `idem_refusal_not_stored` + `idem_same_key_after_refusal` (422 không lưu), `idem_forbidden_not_stored` + `idem_same_key_after_forbidden` (403 không lưu), `idem_key_too_long`.

`parity/scenarios/w2/posts/prod-auth.yaml`, id `w2/posts/prod-auth` (auth `prod`): `anonymous_create_missing_bearer`, `anonymous_malformed_json`, `junk_bearer_create`, `basic_scheme_create`, `actor_headers_ignored_create` (401), `owner_empty_roles_header_ignored` (201: role lấy từ roster, header rỗng không lấy mất), `owner_lowercase_scheme_create` (`bearer` viết thường → 201), `owner_posts_public`, `owner_posts_friends`, `owner_posts_group`, `reader_posts_to_group_with_contexts_header` (403), `idem_owner_first`, `idem_owner_replay`, `idem_reader_same_key_same_body` (scope là digest bearer), `idem_owner_replay_with_actor_header` (bearer thắng `X-Actor-ID`), `leaver_posts_public`, `leaver_deletes_account`, `leaver_posts_after_deletion` (401 `Session is not valid`).

## Chưa phủ / lưu ý cho bản Go

- **Ảnh đã tải thật**: nhánh 201 với ảnh cá nhân có thật cần `POST /people/me/photos` multipart chở byte ảnh; `body_raw` của harness là chuỗi YAML UTF-8 nên không chở được ảnh nhị phân. Chưa phủ. Ảnh nhóm thì 201 tới được mà không cần ảnh (xem lỗi nghi ngờ bên dưới).
- 409 in-flight cần hai request đồng thời cùng key; harness chạy tuần tự.
- **Hành vi trông như lỗi (chỉ báo, không sửa)**: (1) ảnh nhóm không được kiểm tồn tại, bài `group` có thể trỏ tới ảnh không có; (2) người gọi chưa đăng ký ở `dev` nhận 500 thay vì một 4xx; (3) ở `dev`, tài khoản đã xoá vẫn đăng bài được với tên ẩn danh.
- 403 ảnh cá nhân của người khác dùng `code: permission_denied` nhưng detail là câu người đọc, khác mọi 403 `permission_denied` khác (detail là tên predicate). Go phải giữ đúng câu.
- Thứ tự kiểm: role → cặp audience/context → roster → ảnh. Đảo thứ tự sẽ lệch ở `roles_empty_bad_pairing`, `author_public_names_unknown_group`, `stranger_group_photo_unknown_group`.
- `created_at`: pydantic 2.x bỏ phần thập phân khi micro giây bằng 0 (`…:00Z`). Bản Go luôn viết 6 chữ số sẽ lệch shape (`f0` vs `f6`) với xác suất một phần triệu mỗi bài; harness không ép được ca này.
- `image_url` và `context_id` trả về dạng chữ thường đã parse, không phải chuỗi người gửi.
- `allow` của 405 là `POST` (route khai đầu tiên cho `/posts`), không phải `GET, POST`.
- Hai mức JSON: lỗi middleware có khoảng trắng, lỗi route và body 201 thì gọn.
- 500 là `text/plain` của Starlette; nếu bản Go kiểm người gọi tồn tại trước INSERT thì phải mở ADR, không đổi lặng lẽ.
