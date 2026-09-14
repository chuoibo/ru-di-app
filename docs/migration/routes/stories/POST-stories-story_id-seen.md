# POST /stories/{story_id}/seen

stories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Ghi «tôi đã xem story này» cho người gọi. Lần xem đầu tiên là lần được ghi; mọi lần sau trả lại `seen_at` của lần đầu. Đây là thứ làm `seen` và `all_seen` trên `GET /stories` đổi và làm tác giả tụt xuống trên thanh.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-553`): key rỗng → 422 trước xác thực (`anonymous_empty_idempotency_key`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401. Là dependency nên thắng lỗi path: người ẩn danh gửi id không phải UUID nhận 401 (`anonymous_non_uuid_story`); header role lạ cùng path sai ra 422 `invalid_actor_roles`, không phải `uuid_parsing` (`roles_unknown_non_uuid_story`).
3. Path `story_id: UUID` → 422 `uuid_parsing`. Parse lax: chữ hoa, không gạch, `{...}`, `urn:uuid:` đều hợp lệ và đi tiếp tới 404 (`reader_uppercase_uuid`, `reader_unhyphenated_uuid`, `reader_braced_uuid`, `reader_urn_uuid`).
4. `_viewable_story_or_404` (`services/api/app/api/service.py:1968-1985`):
   - `get_story` không thấy → 404 `story_not_found`.
   - `story_visibility.can_view` (`services/api/app/domain/story_visibility.py:92-117`) với `is_friend`, `is_blocked` đọc lúc gọi (`service.py:1962-1966`, `:2127-2148`) và `now = _now()`.
   - `_require_permission("view_story", …, {"may_view_story": …})` (`permissions.py:470-473`, role `group_admin` hoặc `member`). **Mọi** từ chối ở đây, kể cả `role_not_permitted`, đổi thành cùng một 404 (`service.py:1981-1984`): bạn thật với role rỗng nhận 404, không 403 (`friend_roles_empty`, `friend_roles_guest_only`).
5. Không có kiểm tra nào khác: tác giả tự đánh dấu story của mình được (`author_sees_own_story` → 200, ghi một dòng `story_views` cho tác giả).

Không phân biệt được «không tồn tại», «không phải bạn», «lời mời đang chờ», «bị chặn», «đã gỡ chặn», «story đã bị xoá», «tài khoản đã xoá»: tất cả là cùng một câu 404.

## Đầu vào

- Path `story_id` (UUID lax).
- Không body: body gửi kèm bị bỏ qua, kể cả `seen_at` hay `viewer_id` (`friend_look_with_ignored_body` → 200 với `seen_at` của lần đầu), và kể cả JSON hỏng, vì route không khai báo body nên FastAPI không giải mã (`reader_malformed_body_ignored` → 404, không phải `json_invalid`).
- Header: `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **200** (POST nhưng không phải 201) `StorySeenResponse` (`services/api/app/api/schemas.py:1395-1397`), thứ tự khoá `story_id`, `seen_at`.
- `story_id`: id dạng chuẩn của dòng (`record.id`), không phải chuỗi trong path.
- `seen_at`: giá trị **đọc lại từ DB** sau `INSERT … ON CONFLICT DO NOTHING` (`services/api/app/api/repository.py:5025-5044`), nên lần hai trả đúng instant lần một (`friend_second_look`). Giá trị gốc là `_now()` của Python (`service.py:2070`, `:406-407`). Datetime kiểu pydantic UTC `Z`, 6 chữ số lẻ.
- Replay idempotency: cùng 200 và body, thêm `idempotency-replayed: true`. Câu trả lời đã lưu vẫn replay **sau khi bị chặn** (`idem_replay_after_block`), dù route lúc đó sẽ trả 404.
- Framework: 307 cho `/seen/` (`location: http://<Host>/stories/<id>/seen`); `GET` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `story_views (story_id, viewer_id, seen_at)` nếu chưa có; khoá chính `pk_story_views` là luật (`services/api/app/db/models.py:2435-2453`). Lần sau không ghi gì.
- SELECT `stories`, `people`, `friend_requests` (hai lần).
- Dòng `story_views` bị xoá theo story (CASCADE) và khi người xem xoá tài khoản (`repository.py:4812-4815`); **không** bị xoá khi chặn hay gỡ kết bạn, nên kết bạn lại thì `seen` cũ quay về (xem `GET /stories`).
- Idempotency: có key thì ghi `idempotency_keys` (chỉ lưu 2xx). Fingerprint gồm path, nên cùng key cho story khác là reuse (`idem_reuse_other_story`).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | xem card `POST /stories` | `deps.py:144-154` |
| 422 | (framework) `uuid_parsing` | `Input should be a valid UUID, invalid character: found `k` at 1` / `…, invalid group length in group 4: expected 12, found 11` | FastAPI |
| 404 | `story_not_found` | `Story does not exist` | `service.py:1973-1974`, `:1981-1984` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

422 path có `loc: ["path","story_id"]` và `ctx.error` lặp lại phần sau dấu phẩy của `msg`.

## Mã Python

- Route: `services/api/app/api/routes/stories.py:71-83`
- Service: `services/api/app/api/service.py:2068-2071` (`mark_story_seen`), `:1962-1985` (`_story_facts`, `_viewable_story_or_404`), `:2127-2148`
- Domain: `services/api/app/domain/story_visibility.py:92-117`, `services/api/app/domain/permissions.py:470-473`
- Repository: `services/api/app/api/repository.py:4990-4997` (`get_story`), `:5025-5044` (`mark_story_seen`)

## Test đang phủ

- `services/api/tests/api/test_stories.py::test_seen_is_per_reader_and_the_first_look_counts` (130)
- `services/api/tests/postgres/test_stories_postgres.py::test_seen_is_one_row_per_look_and_goes_with_the_story` (106)

## Kịch bản parity

Hai file cho route này, tách theo lane (lý do ở mục Chưa phủ).

`parity/scenarios/w2/stories/POST-stories-story_id-seen.yaml`, id `w2/stories/post-stories-story_id-seen` (25 bước), được xác nhận với lane DB bật (`--reference-dsn`/`--candidate-dsn`), không có bước nào chèn hay xoá dòng `uploaded_images`; không có story thật:

- Thứ tự từ chối: `anonymous_unknown_story`, `anonymous_non_uuid_story` (401 trước 422 path), `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown_non_uuid_story` (422 `invalid_actor_roles` thắng 422 path).
- Path và 404: `reader_non_uuid_story`, `reader_short_uuid` (422), `reader_uppercase_uuid`, `reader_unhyphenated_uuid`, `reader_braced_uuid`, `reader_urn_uuid`, `reader_unknown_story`, `unregistered_unknown_story`, `reader_roles_empty_unknown_story`, `reader_malformed_body_ignored` (JSON hỏng không được đọc, vẫn 404), `reader_after_account_deleted`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).
- Idempotency, chỉ các nhánh từ chối: `idem_key_too_long`, `idem_refusal_not_stored` + `idem_same_key_other_story_after_refusal` (404, không phải reuse), `idem_path_422_not_stored` + `idem_same_key_after_path_422`.
- Chuẩn bị: `register_reader`, `reader_deletes_account`.

`parity/scenarios/w2/stories-photo/POST-stories-story_id-seen.yaml`, id `w2/stories-photo/post-stories-story_id-seen` (37 bước, lane DB):

- Ma trận 404: `stranger_real_story`, `friend_pending_request`, `friend_roles_empty`, `friend_roles_guest_only`, `friend_after_block`, `friend_after_unblock_declined`, `author_sees_deleted_story`, `friend_after_account_deleted`.
- Đường vui: `friend_first_look`, `friend_second_look` (cùng `seen_at`), `friend_look_with_ignored_body`, `friend_roles_group_admin_only`, `author_sees_own_story`, `author_still_sees_own_while_blocking`, `friend_refriended_looks_at_story_two`; đọc lại ở `friend_feed_shows_seen`, `author_feed_shows_own_seen`.
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_other_story`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_replay_after_block`.

`prod`:

- `parity/scenarios/w2/stories/prod-auth.yaml`, id `w2/stories/prod-auth` (lane DB): `basic_scheme_seen` (401), `anonymous_non_uuid_seen` (401 trước 422), `author_actor_id_header_junk_ignored` (dev sẽ 422, prod ra 404).
- `parity/scenarios/w2/stories-photo/prod-auth.yaml`, id `w2/stories-photo/prod-auth`: `friend_seen_not_friend_yet` (404), `friend_sees_story` + `friend_seen_replay`.

## Chưa phủ / lưu ý cho bản Go

- Story của **bạn** đã hết hạn → 404; story hết hạn của **chính mình** vẫn 200: không dời được đồng hồ qua HTTP, chỉ test Python phủ.
- **`storage_key`, lý do corpus tách đôi**: mỗi lần tải ảnh cá nhân (`POST /people/me/photos`) ghi `uploaded_images.storage_key = secrets.token_hex(16)` (`services/api/app/media/storage.py:28-31`, gọi ở `services/api/app/api/service.py:1817`): 32 ký tự hex ngẫu nhiên, **không bao giờ lên wire** (không có trong `UploadedImageResponse`, `services/api/app/api/schemas.py:1334-1342`). Harness bind chuỗi đúng 32 hex thường theo lần xuất hiện thành `<hex32#n>` (ADR-0029 §2.4), nên mọi đường cần ảnh thật (`w2/stories-photo/*`, kể cả `prod-auth`) chạy với lane DB; khoá dùng lại, viết hoa hay độ dài khác vẫn đỏ. `w2/stories/*` giữ các đường không cần ảnh.
- Hai người cùng xem một lúc (race trên `ON CONFLICT`) và 409 in-flight chưa phủ.
- Bản Go phải đọc lại `seen_at` từ DB (hoặc `RETURNING` sau `ON CONFLICT DO UPDATE` không đổi giá trị), không trả `now` của request hiện tại.
- Role check nằm **trong** cổng 404: tách nó ra thành 403 sẽ lệch `friend_roles_empty` và biến route thành oracle cho sự tồn tại của story.
- Replay sau khi bị chặn là hành vi của middleware chung, không riêng route này.
