# PUT /people/me/saved-places/{place_id}

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đánh dấu một địa điểm có trong danh mục. 201 lần đầu, 200 khi đã đánh dấu, cùng thân.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:108-127`, `services/api/app/api/service.py:4256-4260`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): khoá rỗng 422 (`anonymous_empty_idempotency_key`); khoá đã xong phát lại 201 (`owner_replays_key`); chỗ khác cùng khoá → 422 reuse (`owner_key_other_place`).
2. Router: path đã giải mã phần trăm trước khi khớp: `p%2Dtiem…` là `p-tiem…` (`owner_saves_percent_encoded_hyphens`); `%2F` tạo hai đoạn và không khớp route nào → 404 `{"detail":"Not Found"}` (`owner_place_encoded_slash`); đuôi `/` → 307 (`trailing_slash`); đoạn rỗng `/people/me/saved-places/` → 307 về route GET (`empty_place_segment`); `POST` → 405.
3. `get_actor` → 401 (trước 404 của chỗ lạ, `anonymous_unknown_place`); 422 `invalid_actor_id` / `invalid_actor_roles`.
4. `place_id: str` bất kỳ, không validation. Thân không khai báo, bị bỏ qua (`owner_saves_with_json_body`).
5. `_require_permission("manage_saved_places", {"is_self": True})` (`service.py:4257`; `services/api/app/domain/permissions.py:202`): thiếu `member` → 403 `permission_denied` `role_not_permitted`, trước khi tra chỗ (`owner_unknown_place_without_roles`).
6. `_known_place` (`service.py:4270-4279`): `place_row` (`service.py:1241-1247` → `get_place`, `services/api/app/api/repository.py:4190-4192`) không có → 404 `place_not_found` `Không có địa điểm này trong danh mục.` So khớp chính xác: viết hoa, có khoảng trắng, tiếng Việt đều 404 (`owner_unknown_place`, `owner_place_uppercase`, `owner_place_padded`, `owner_place_vietnamese`).
7. `save_place` (`repository.py:4273-4297`): SELECT hàng `(person_id, place_id)`; có → 200 với hàng cũ; không → INSERT `saved_places(person_id, place_id, created_at=now)` trong savepoint → 201. `IntegrityError` → đọc lại hàng thắng → 200. Người gọi chưa có hàng `people` (dev): INSERT vi phạm khoá ngoại, cũng là `IntegrityError`, đọc lại không có gì → `assert` → 500 (`ghost_saves_unregistered`).

## Đầu vào

- Path `place_id`: khoá danh mục (`places.id`), chuỗi, phân biệt hoa thường, sau giải mã phần trăm.
- Không query, không thân.

## Đầu ra

**201** hoặc **200** `SavedPlaceSummary` (`services/api/app/api/schemas.py:951-955`), thứ tự khoá: `place_id`, `name`, `category`, `saved_at`. `name`, `category` từ danh mục; `saved_at` = `created_at` của hàng (lần đầu), không đổi khi lưu lại.

## Tác dụng phụ

INSERT `saved_places` một lần cho mỗi `(person, place)` (`uq_saved_places_person_place`, `services/api/app/db/models.py:2704`). Lưu lại không ghi. Bỏ lưu rồi lưu lại là hàng mới với `saved_at` mới (`owner_saves_first_after_unsave`).

## Idempotency

Tự nhiên idempotent (200 lần sau). Khoá header: 201 được lưu và phát lại là 201 dù hàng đã có (`crossreplay/PUT-people-me-saved-places-place_id.yaml`). Ba lần lưu đồng thời: một 201, hai 200 cùng `saved_at` (`concurrency/…`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4257` |
| 404 | `place_not_found` | `Không có địa điểm này trong danh mục.` | `service.py:4270-4279` |
| 404 | — | `{"detail":"Not Found"}` (`%2F`) | Starlette |
| 500 | — | `Internal Server Error` (người gọi chưa đăng ký) | `repository.py:4288-4295` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:108-127`
- Service: `services/api/app/api/service.py:4256-4260`, `:4270-4288`, `:1241-1247`
- Repository: `services/api/app/api/repository.py:4273-4297`, `:4190-4192`
- Quyền: `services/api/app/domain/permissions.py:202`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_saved_places_are_idempotent_and_named_from_the_catalogue` (191), `test_a_key_the_catalogue_does_not_know_is_refused_on_put_and_delete` (224)
- `services/api/tests/postgres/test_profile_postgres.py`: `test_one_bookmark_per_person_and_place_in_the_database_and_over_http` (262)

## Kịch bản parity

`parity/scenarios/w10/people/PUT-people-me-saved-places-place_id.yaml`, id `w10/people/put-people-me-saved-places-place_id` (31 bước, `dev`):

- Thứ tự: `anonymous_saves`, `anonymous_unknown_place` (401 trước 404), `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `owner_unknown_place_without_roles` (403 trước 404), `ghost_saves_unregistered` (500), `ghost_unknown_place_unregistered` (404).
- Khoá danh mục: `owner_unknown_place`, `owner_place_uppercase`, `owner_place_padded`, `owner_place_encoded_slash` (404 Starlette), `owner_place_vietnamese`.
- Lưu: `owner_saves_first` (201), `owner_saves_first_again`, `owner_saves_percent_encoded_hyphens`, `owner_saves_with_json_body` (200), `owner_saves_second_as_group_admin_and_member` (201).
- Khoá: `owner_saves_third_with_key` (201), `owner_replays_key`, `owner_key_other_place`, `owner_saves_third_without_key` (200).
- `other_saves_same_place` (201), `owner_lists`, `owner_unsaves_then_saves_again`, `owner_saves_first_after_unsave` (201, `saved_at` mới).
- Framework: `trailing_slash`, `empty_place_segment` (307 về `/people/me/saved-places`), `post_not_allowed` (405 `allow: PUT`).

`crossreplay/PUT-people-me-saved-places-place_id.yaml` (11 bước), `concurrency/PUT-people-me-saved-places-place_id.yaml` (4 bước: ba lần lưu cùng lúc → hai 200 và một 201 cùng `saved_at`; ba lần cùng khoá → ba 201, một chèn).

`prod-auth.yaml`: `junk_bearer_unknown_place` (401 trước 404), `owner_saves_place`, `owner_saves_after` (401).

## Chưa phủ / lưu ý cho bản Go

- Tra danh mục theo khoá chính xác, sau giải mã phần trăm của router; `%2F` không bao giờ tới handler.
- Vai trò trước danh mục; danh mục trước hàng `people` (không tra).
- Người gọi chưa có hàng `people` là 500 (`AssertionError`), transaction rollback.
- Corpus sinh: hoãn, `path place_id is str`; hình dạng khoá đã phủ viết tay ở trên.

## Lỗi Python (chỉ báo, không sửa)

- `IntegrityError` của khoá ngoại `person_id` bị đọc như một lần đua, đọc lại không thấy gì rồi `assert` → 500 thay vì 404/409 (`repository.py:4288-4295`). Chỉ tới được khi người gọi không có hàng `people` (dev).
