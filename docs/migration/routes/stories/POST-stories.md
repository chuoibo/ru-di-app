# POST /stories

stories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Đăng một story 24 giờ (ADR-0022 §2.3): một ảnh cá nhân **của chính người gọi** kèm chú thích tuỳ chọn, gửi cho bạn bè. Tác giả là actor, hạn là `created_at + 24h`, audience luôn là `friends`; không trường nào trong ba thứ đó do người gọi đặt.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: rỗng hoặc dài hơn 255 → 422 trước cả xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`); key đã dùng cho request khác → 422; đã có kết quả → replay (`services/api/app/api/idempotency.py:404-553`).
2. JSON hỏng → 422 `json_invalid`, **trước** xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu danh tính → 401, kể cả khi body sai (`anonymous_invalid_body` → 401, không 422). Ở `dev`: `X-Actor-ID` không phải UUID → 422 (`actor_id_not_uuid`), role lạ → 422 (`roles_unknown`). Ở `prod`: bearer, `X-Actor-*` bị bỏ qua (`prod-auth`).
4. Validate body `StoryCreateRequest` → 422 dạng `{"detail":[...]}`.
5. `_require_permission("create_story", actor, {"is_self": True})` (`services/api/app/api/service.py:1989`; bảng quyền `services/api/app/domain/permissions.py:466-469`, role `group_admin` hoặc `member`) → 403 `role_not_permitted`. Chạy **trước** khi đọc url: role rỗng gửi ảnh của người khác vẫn là `role_not_permitted` (`roles_empty_other_persons_photo`). Chỉ `group_admin` là đủ (`roles_group_admin_only_unknown_photo` → đi tiếp tới 404).
6. `parse_photo_url` (`services/api/app/domain/photo_ref.py:51-74`) → 422 `photo_url_invalid` (không tới được qua HTTP, xem Lỗi).
7. Chủ ảnh phải là actor (`service.py:1996-1999`) → 403 `permission_denied` với câu tiếng Việt. Kiểm **trước** sự tồn tại: photo id bịa của người khác → 403, không 404 (`author_other_persons_photo_unknown`); ảnh thật của người khác cũng 403 (`other_uses_authors_real_photo`).
8. `get_person_image(actor.id, photo_id)` với `purpose = 'personal'` (`service.py:2000-2001`; `services/api/app/api/repository.py:4560-4570`) → 404 `photo_not_found`. Người chưa đăng ký (`ghost_own_url_unknown_photo`) và người đã xoá tài khoản (`author_after_deleting_account`, ảnh bị xoá cùng tài khoản) cũng rơi vào đây.
9. `check_caption` (`service.py:2002-2008`) → 422 (không tới được qua HTTP).

## Đầu vào

- Không có path param. Router `stories` include ở `services/api/app/api/main.py:231`.
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type`. Không có `Content-Type` thì vẫn parse JSON (`no_content_type_is_parsed_as_json` → 201); `text/plain` → 422 `model_attributes_type`.
- Body `StoryCreateRequest` (`services/api/app/api/schemas.py:1350-1360`), `extra="forbid"` (`schemas.py:66-67`), thứ tự trường:
  1. `image_url: PersonPhotoUrl` — `StrictStr` khớp `\A/people/<uuid>/photos/<uuid>\z`, uuid chấp nhận hoa lẫn thường (`schemas.py:40-48`). Url ảnh nhóm, url tuyệt đối, `/` cuối → 422 `string_pattern_mismatch` (msg và `ctx.pattern` chép nguyên pattern).
  2. `caption: StrictStr (max_length=200) | None = None` — đếm **ký tự** (code point), không đếm byte: 200 chữ `ă` (400 byte) qua, 201 → 422 `string_too_long` với `ctx.max_length: 200`.
- Chuẩn hoá: `caption or None` (`service.py:2002`) nên `""` lưu thành `null`; chuỗi toàn khoảng trắng giữ nguyên (`author_whitespace_caption_kept`). `image_url` lưu dạng chuẩn `PhotoRef.url` (uuid thường, có gạch, `photo_ref.py:43-48`); photo id viết hoa vẫn tra đúng ảnh (`author_uppercase_photo_id_unknown` → 404 như id thường).
- Cùng một ảnh đăng được nhiều story (mọi bước 201 trong kịch bản dùng chung `photo`).

## Đầu ra

- **201** `StoryResponse` (`schemas.py:1363-1373`), thứ tự khoá `id`, `author_id`, `author_display_name`, `image_url`, `caption`, `audience`, `created_at`, `expires_at`, `seen` (`_wire_story`, `service.py:574-585`).
  - `author_display_name`: tên trong `people` qua `_display_names` (`repository.py:2529-2548`; rơi về chuỗi id nếu không có tên).
  - `audience`: luôn `"friends"`; `seen`: luôn `false` ở câu trả lời tạo mới (`repository.py:4986-4988`).
  - `created_at` = `_now()` của Python (`service.py:406-407`, `:2009`), **không** dùng `server_default now()` của cột; `expires_at` = đúng `created_at + 24h` (`services/api/app/domain/story_visibility.py:44`, `:71-73`), cùng phần micro giây.
  - Datetime ghi kiểu pydantic: `2026-09-14T20:03:52.925480Z` (UTC, hậu tố `Z`, 6 chữ số lẻ; pydantic bỏ phần lẻ khi micro giây bằng 0).
- Không có float.
- Chú thích được phản chiếu nguyên văn: Python chỉ thoát `"` và `\`, giữ `<`, `>`, `&` và chữ Việt dạng UTF-8 thô (`author_creates_with_caption`).
- Replay idempotency: cùng 201 và body, thêm `idempotency-replayed: true`; body JSON đổi thứ tự khoá và khoảng trắng vẫn là replay (`idem_replay_reordered_keys`).
- Framework: 307 cho `/stories/` (`location: http://<Host>/stories`); `PUT /stories` → 405 `{"detail":"Method Not Allowed"}` với **`allow: POST`** (Starlette lấy route khớp một phần đầu tiên, và `POST /stories` khai báo trước `GET /stories`).

## Tác dụng phụ

- INSERT một dòng `stories` (`repository.py:4964-4988`; bảng `services/api/app/db/models.py:2377-2432`): CHECK audience `friends`, `expires_at > created_at`, caption ≤ 200.
- SELECT `uploaded_images`, `people`.
- Idempotency: có key thì đặt chỗ trong `idempotency_keys`, chỉ lưu câu trả lời 2xx, nhả key khi 4xx (`idempotency.py:533-546`). Scope là chuỗi `X-Actor-ID` thô (`dev`) hoặc `bearer:<sha256>` (`prod`, `idempotency.py:327-335`, `:447-449`): cùng key từ người khác là key mới (`idem_same_key_other_actor_refused` và `friend_same_key_is_another_scope` trong file gap).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | `X-Actor-ID must be a UUID` / `X-Actor-Roles contains an unknown role` | `deps.py:144-154` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:1989`, `:502-504` |
| 403 | `permission_denied` | `Chỉ đăng được ảnh của chính mình.` | `service.py:1996-1999` |
| 404 | `photo_not_found` | `Photo does not exist` | `service.py:2000-2001` |
| 422 | `photo_url_invalid` | `image_url is not a photo of this product` | `service.py:1990-1995` (không tới được) |
| 422 | `caption_too_long` / `caption_not_text` | `Chú thích dài quá 200 ký tự.` | `service.py:2003-2008` (không tới được) |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

- Không tới được qua HTTP: `photo_url_invalid` (pattern của `PersonPhotoUrl` chặn mọi url mà `parse_photo_url` sẽ từ chối), `caption_too_long` và `caption_not_text` (pydantic đếm cùng đơn vị code point và chặn trước).
- 422 framework đã đo: `json_invalid`, `missing` (`["body","image_url"]`, hoặc `["body"]` khi body rỗng), `string_type`, `string_pattern_mismatch`, `string_too_long`, `extra_forbidden` (một lỗi cho mỗi trường thừa, theo thứ tự gửi), `model_attributes_type` (mảng top-level, `text/plain`). Không có `input` (`main.py:318-351`).
- Lỗi của middleware có khoảng trắng sau `:` và `,`; lỗi của route thì gọn.

## Mã Python

- Route: `services/api/app/api/routes/stories.py:40-57`
- Service: `services/api/app/api/service.py:1987-2018` (`create_story`), `:475-504` (`_require_permission`), `:574-585` (`_wire_story`)
- Domain: `services/api/app/domain/story_visibility.py:44-89`, `services/api/app/domain/photo_ref.py:51-74`, `services/api/app/domain/permissions.py:466-469`
- Repository: `services/api/app/api/repository.py:4560-4570` (`get_person_image`), `:4964-4988` (`create_story`), `:4949-4962` (`_story_record`)
- Ảnh cá nhân (bước chuẩn bị): `services/api/app/api/routes/photos.py:125-142`, `service.py:1909-1922`, `:1803-1830`

## Test đang phủ

- `services/api/tests/api/test_stories.py`: `test_only_ones_own_existing_photo_becomes_a_story` (102), `test_the_caption_is_at_most_two_hundred_characters` (117), `test_a_story_reaches_friends_and_nobody_else` (78)
- `services/api/tests/postgres/test_stories_postgres.py`: `test_the_checks_refuse_what_the_domain_refuses` (237), `test_a_story_reaches_a_friend_and_never_leaves_the_database_for_a_stranger` (65)
- `services/api/tests/domain/test_story_visibility.py`: `test_the_deadline_is_twenty_four_hours_after_writing` (20), `test_caption_length` (101), `test_a_naive_datetime_is_refused_everywhere` (25)

## Kịch bản parity

Hai file cho route này, tách theo lane (lý do ở mục Chưa phủ).

`parity/scenarios/w2/stories/POST-stories.yaml`, id `w2/stories/post-stories` (41 bước), được xác nhận với lane DB bật (`--reference-dsn`/`--candidate-dsn`), không có bước nào chèn hay xoá dòng `uploaded_images`:

- Thứ tự từ chối: `anonymous_valid_body`, `anonymous_invalid_body` (401 trước 422 body), `anonymous_malformed_json` (422 trước 401), `anonymous_empty_idempotency_key` (middleware trước 401), `actor_id_not_uuid`, `roles_unknown`, `roles_empty_caption_over_limit` (422 body trước 403 role).
- Role: `roles_empty_other_persons_photo`, `roles_guest_only` (403 `role_not_permitted`), `roles_group_admin_only_unknown_photo` (qua quyền, tới 404).
- Ảnh không có thật: `author_other_persons_photo_unknown` (403 trước 404), `author_own_url_unknown_photo`, `author_uppercase_photo_id_unknown`, `ghost_own_url_unknown_photo`, `caption_at_limit_reaches_photo_lookup` (200 ký tự qua model, tới 404), `no_content_type_is_parsed_as_json` (404, không phải 422), `author_after_deleting_account` (404).
- Validate: `caption_over_limit`, `missing_image_url`, `image_url_number`, `image_url_group_photo_shape`, `image_url_absolute`, `image_url_trailing_slash`, `caption_number`, `extra_fields_author_and_deadline`, `empty_object`, `empty_body`, `top_level_array`, `text_plain_content_type`.
- Framework: `trailing_slash_redirects` (307), `put_not_allowed` (405, `allow: POST`).
- Idempotency, chỉ các nhánh từ chối: `idem_key_too_long` (256), `idem_key_at_limit_refused` (255 là key hợp lệ, tới route và 404), `idem_refusal_not_stored` + `idem_same_key_other_body_after_refusal` (403, không phải `idempotency_key_reuse`) + `idem_same_body_after_two_refusals` (404 lại, không replay), `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal` (422 cũng không được lưu).
- Chuẩn bị: `register_author`, `register_other`, `author_deletes_account` (tác giả không có ảnh nên không chạm `uploaded_images`).

`parity/scenarios/w2-storage-key-gap/stories/POST-stories.yaml`, id `w2-storage-key-gap/stories/post-stories` (23 bước), được xác nhận **chỉ trên wire** (không DSN, vẫn có `--candidate-tap` và `--served-routes`):

- Đường vui (201): `author_creates_with_caption` (thoát JSON), `author_creates_without_caption`, `author_empty_caption_stored_as_null`, `author_null_caption`, `author_whitespace_caption_kept`, `author_caption_at_limit`, `no_content_type_is_parsed_as_json`, `author_group_admin_only_creates`; đọc lại ở `author_feed_after_writes`.
- Ảnh thật: `other_uses_authors_real_photo` (403), `author_after_deleting_account` (404, ảnh bị xoá cùng tài khoản).
- Idempotency: `idem_first`, `idem_replay`, `idem_replay_reordered_keys`, `idem_reuse_different_body`, `idem_second_distinct_key`, `idem_same_key_other_actor_refused` (scope khác, 403 không lưu), `idem_refusal_not_stored` + `idem_same_key_after_refusal` (201).
- Chuẩn bị: `register_author`, `register_other`, `upload_photo`, `author_deletes_account`.

`prod`:

- `parity/scenarios/w2/stories/prod-auth.yaml`, id `w2/stories/prod-auth` (23 bước, lane DB): `anonymous_create` (`Missing bearer session`), `anonymous_malformed_json` (422 trước 401), `author_roles_header_empty_ignored` và `author_roles_header_unknown_ignored` (dev sẽ 403/422, prod ra 404 vì role lấy từ roster), `author_other_persons_photo` (403), `author_refusal_with_key` + `author_same_key_other_body_after_refusal` (scope bearer, key được nhả), `author_create_after_sign_out` (401 `Session is not valid`).
- `parity/scenarios/w2-storage-key-gap/stories/prod-auth.yaml`, id `w2-storage-key-gap/stories/prod-auth` (16 bước): `author_creates_with_empty_roles_header` (201; dev sẽ 403), `author_creates_story` + `author_create_replay` (scope là bearer, `X-Actor-ID` gửi kèm không đổi scope), `friend_same_key_is_another_scope`.

## Chưa phủ / lưu ý cho bản Go

- **Khoảng trống `storage_key`, lý do corpus tách đôi**: mỗi lần tải ảnh cá nhân (`POST /people/me/photos`) ghi `uploaded_images.storage_key = secrets.token_hex(16)` (`services/api/app/media/storage.py:28-31`, gọi ở `services/api/app/api/service.py:1817`): 32 ký tự hex ngẫu nhiên, **không bao giờ lên wire** (không có trong `UploadedImageResponse`, `services/api/app/api/schemas.py:1334-1342`). Bộ chuẩn hoá của harness chỉ gắn placeholder cho UUID v4 viết thường, digest hex đúng 64 ký tự, timestamp và literal được đặt tên (persona, token đã bind) (`parity/internal/normalize/normalize.go:30-38`, `:64`), nên dòng này luôn khác nhau giữa hai stack, và kịch bản không có cách nào bind nó. Vì mọi story cần một ảnh thật: `w2/stories/*` không có bước nào chèn hay xoá dòng `uploaded_images` (không tải ảnh, không xoá tài khoản của người có ảnh) và được xác nhận với lane DB; `w2-storage-key-gap/stories/*` chứa mọi đường cần ảnh thật và được xác nhận **chỉ trên wire**; một lần chạy file gap có bật lane DB cho thấy khác biệt duy nhất là các dòng `uploaded_images` lệch nhau ở `storage_key` (giống hệt sau khi che cột đó). Cách đóng: một luật che hoặc bind cột `uploaded_images.storage_key` trong harness (phạm vi ADR-0029), **chưa làm ở đây**; khi có luật đó thì gộp hai file lại và chạy cả hai trên lane DB.
- `w2-storage-key-gap/stories/prod-auth` không chạy được bằng `parity run` khi thiếu DSN: persona `prod` cần database để seed phiên, và truyền DSN là bật lane DB (đo trên cặp stack `prod` mới dựng: đúng 1 khác biệt, dòng `uploaded_images` ở `upload_photo` chỉ khác `storage_key`; `parity: scenarios=1 steps=16 scenarios_diff=1 differences=1 database_lane=on`). Chỉ `parity canary` seed phiên mà không chụp DB.
- 409 in-flight chưa phủ (harness chạy tuần tự).
- Ảnh tải lên là PPM `P3` ASCII 1×1 (dữ liệu mẫu) gói trong multipart viết tay, vì `body_raw` chỉ mang được văn bản; JPEG/PNG thật không biểu diễn được.
- Người chưa đăng ký tải ảnh lên (FK `uploaded_images.owner_person_id`) chưa đo.
- Datetime: Python bỏ phần lẻ giây khi micro giây bằng 0; Go ghi cố định 6 chữ số sẽ lệch ở ca đó (normalizer giữ shape `f0`/`f6`), kịch bản không ép được đồng hồ vào ca này.
- `expires_at` phải cộng đúng 24h vào **cùng** instant với `created_at`, lấy từ đồng hồ ứng dụng, không lấy `now()` của Postgres.
- Thoát JSON: tắt `SetEscapeHTML` như card `POST /contexts/{context_id}/meet`.
- `allow: POST` cho `PUT /stories` là hành vi Starlette theo thứ tự khai báo route; bản Go liệt kê `GET, POST` sẽ lệch `put_not_allowed`.
- Hai nhánh 422 của service là mã chết; bản Go có thể bỏ nhưng phải giữ pattern `PersonPhotoUrl` và `max_length` đếm code point.
