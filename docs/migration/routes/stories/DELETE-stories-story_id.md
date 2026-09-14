# DELETE /stories/{story_id}

stories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tác giả gỡ story của mình trước hạn 24h. Không ai khác gỡ được: bạn đang xem được story nhận 403, người không xem được nhận cùng 404 như mọi route có `{story_id}`.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: dài hơn 255 → 422 trước xác thực (`anonymous_key_too_long`) (`services/api/app/api/idempotency.py:432-439`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_story`).
3. Path `story_id: UUID` (lax) → 422 `uuid_parsing`.
4. `_viewable_story_or_404` (`services/api/app/api/service.py:1968-1985`): không tồn tại, không được xem (không phải bạn, lời mời đang chờ, bị chặn theo chiều nào, tài khoản người gọi đã xoá) **hoặc thiếu role** → 404 `story_not_found`. Tác giả với role rỗng hay chỉ `guest` cũng nhận 404 cho story của chính mình (`author_roles_empty`, `author_roles_guest_only`), vì `view_story` đòi `group_admin` hoặc `member` (`services/api/app/domain/permissions.py:470-473`).
5. `_require_permission("delete_own_story", …, {"is_author": record.author_id == actor.id})` (`service.py:2077-2079`; `permissions.py:474-477`) → 403 `permission_denied`, detail `is_author`. Chỉ tới được khi người gọi xem được story mà không phải tác giả, tức là bạn (`friend_deletes_friends_story`, `new_friend_deletes_friends_story`).

Hệ quả thứ tự: cùng một người bạn đổi từ 403 sang 404 khi chặn tác giả (`friend_deletes_after_block`) hoặc xoá tài khoản (`new_friend_after_account_deleted`); tác giả vẫn xoá được khi đang bị chặn (`author_deletes_while_blocked`).

## Đầu vào

- Path `story_id` (UUID lax, như card `POST /stories/{story_id}/seen`).
- Không body; body JSON gửi kèm bị bỏ qua (`author_group_admin_only_with_ignored_body` → 204).
- Header: `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **204** không body, không `content-type`, không `content-length` (`services/api/app/api/routes/stories.py:98-99`).
- Replay idempotency: 204 với `content-length: 0` và `idempotency-replayed: true` (`idempotency.py:585-599`); lần đầu thì không có `content-length`. Đây đúng là ca ADR-0029 §2.4 `RESPONSE-204-CONTENT-LENGTH` chấp nhận cho Go (không gửi `content-length` trên 204).
- Xoá lần hai → 404 (`author_deletes_story_one_again`).
- Framework: 307 cho `/stories/<id>/` (`location: http://<Host>/stories/<id>`); `GET` và `PATCH` → 405 `allow: DELETE`.

## Tác dụng phụ

- DELETE một dòng `stories` qua ORM (`services/api/app/api/repository.py:5046-5050`); `story_views` của story đó đi theo bằng `ON DELETE CASCADE` (`services/api/app/db/models.py:2443-2447`), thấy ở `author_deletes_story_one` trong lần chạy file gap có bật lane DB (bạn đã xem trước đó ở `friend_sees_story_one`).
- Không xoá ảnh trong `uploaded_images`: ảnh vẫn dùng được cho story khác.
- SELECT `stories`, `people`, `friend_requests`.
- Idempotency: có key thì lưu 204 vào `idempotency_keys`; fingerprint gồm path nên cùng key cho story khác là reuse (`idem_reuse_other_story`); 403/404 không được lưu. Scope theo actor: key `w2-del-c` của bạn (403, `idem_refusal_not_stored`) và của tác giả là hai key khác nhau.
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | xem card `POST /stories` | `deps.py:144-154` |
| 422 | (framework) `uuid_parsing` | như card `POST /stories/{story_id}/seen` | FastAPI |
| 404 | `story_not_found` | `Story does not exist` | `service.py:1973-1974`, `:1981-1984` |
| 403 | `permission_denied` | `is_author` | `service.py:2077-2079`, `:502-504` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

## Mã Python

- Route: `services/api/app/api/routes/stories.py:86-99`
- Service: `services/api/app/api/service.py:2073-2080` (`delete_story`), `:1968-1985` (`_viewable_story_or_404`)
- Domain: `services/api/app/domain/story_visibility.py:92-117`, `services/api/app/domain/permissions.py:470-477`
- Repository: `services/api/app/api/repository.py:4990-4997` (`get_story`), `:5046-5050` (`delete_story`)
- Middleware: `services/api/app/api/idempotency.py:404-553`, `:585-599`

## Test đang phủ

- `services/api/tests/api/test_stories.py::test_only_the_author_takes_a_story_down` (161)
- `services/api/tests/postgres/test_stories_postgres.py`: `test_seen_is_one_row_per_look_and_goes_with_the_story` (106), `test_the_sweep_removes_only_what_nothing_points_at` (258)

## Kịch bản parity

Hai file cho route này, tách theo lane (lý do ở mục Chưa phủ).

`parity/scenarios/w2/stories/DELETE-stories-story_id.yaml`, id `w2/stories/delete-stories-story_id` (21 bước), được xác nhận với lane DB bật (`--reference-dsn`/`--candidate-dsn`), không có bước nào chèn hay xoá dòng `uploaded_images`; không có story thật:

- Thứ tự từ chối: `anonymous_unknown_story`, `anonymous_non_uuid_story` (401 trước 422), `anonymous_key_too_long` (middleware trước 401), `actor_id_not_uuid`, `roles_unknown`.
- Path và 404: `caller_non_uuid_story`, `caller_short_uuid` (422), `caller_uppercase_uuid`, `caller_urn_uuid`, `caller_unknown_story`, `caller_roles_empty_unknown_story`, `caller_body_ignored_unknown_story`, `caller_after_account_deleted`.
- Framework: `get_not_allowed`, `patch_not_allowed` (405 `allow: DELETE`), `trailing_slash_redirects` (307).
- Idempotency, chỉ các nhánh từ chối: `idem_empty_key`, `idem_refusal_not_stored` + `idem_same_key_other_story_after_refusal` (404, không phải reuse).
- Chuẩn bị: `register_caller`, `caller_deletes_account`.

`parity/scenarios/w2/stories-photo/DELETE-stories-story_id.yaml`, id `w2/stories-photo/delete-stories-story_id` (36 bước, lane DB):

- 404 trước 403: `stranger_real_story`, `friend_pending_request`, `friend_roles_empty`, `author_roles_empty`, `author_roles_guest_only`, `friend_deletes_after_block`, `new_friend_after_account_deleted`, `author_deletes_story_one_again`.
- 403: `friend_deletes_friends_story`, `new_friend_deletes_friends_story`.
- 204: `author_deletes_story_one` (CASCADE `story_views`), `author_group_admin_only_with_ignored_body`, `author_deletes_while_blocked`; đọc lại ở `friend_feed_after_delete`.
- Idempotency: `idem_first`, `idem_replay` (204 + `content-length: 0`), `idem_reuse_other_story`, `idem_refusal_not_stored` (403 của bạn), `idem_refusal_unknown_story_not_stored` (404 của tác giả) + `idem_same_key_after_refusal`.
- Chuẩn bị: `register_*`, `upload_photo`, `author_creates_story_*`, `friend_asks_author`, `author_accepts`, `friend_sees_story_one`, `friend_blocks_author`, `stranger_asks_author`, `author_accepts_stranger`, `new_friend_deletes_account`.

`prod`:

- `parity/scenarios/w2/stories/prod-auth.yaml`, id `w2/stories/prod-auth` (lane DB): `actor_headers_ignored_delete` (401 dù có `X-Actor-ID`), `author_delete_unknown_story` (404), `author_non_uuid_delete_after_sign_out` (401 trước 422 path).
- `parity/scenarios/w2/stories-photo/prod-auth.yaml`, id `w2/stories-photo/prod-auth`: `friend_deletes_authors_story` (403), `author_deletes_story` (204).

## Chưa phủ / lưu ý cho bản Go

- Tác giả xoá story **đã hết hạn** của mình (vẫn tới được vì `can_view` cho tác giả qua trước kiểm hạn): không dời được đồng hồ qua HTTP.
- **`storage_key`, lý do corpus tách đôi**: mỗi lần tải ảnh cá nhân (`POST /people/me/photos`) ghi `uploaded_images.storage_key = secrets.token_hex(16)` (`services/api/app/media/storage.py:28-31`, gọi ở `services/api/app/api/service.py:1817`): 32 ký tự hex ngẫu nhiên, **không bao giờ lên wire** (không có trong `UploadedImageResponse`, `services/api/app/api/schemas.py:1334-1342`). Harness bind chuỗi đúng 32 hex thường theo lần xuất hiện thành `<hex32#n>` (ADR-0029 §2.4), nên mọi đường cần ảnh thật (`w2/stories-photo/*`, kể cả `prod-auth`) chạy với lane DB; khoá dùng lại, viết hoa hay độ dài khác vẫn đỏ. `w2/stories/*` giữ các đường không cần ảnh.
- Replay 204: Python gửi `content-length: 0`, Go thì không; harness đã chấp nhận riêng cặp này (ADR-0029 §2.4, `w0/replay-204`). Bản Go không được thêm `content-type` vào 204.
- Hai người cùng xoá một lúc và 409 in-flight chưa phủ.
- Role check nằm trong cổng 404, trước `is_author`: bản Go tách thành 403 sẽ lệch `author_roles_empty`.
