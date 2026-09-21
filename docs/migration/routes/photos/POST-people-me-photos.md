# POST /people/me/photos

photos · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tải một ảnh của chính mình để đăng bài hoặc story (ADR-0022 §2.1). Chủ ảnh là phiên, không phải id trong path: path là `me` cố định. Ảnh chưa ai khác đọc được cho tới khi một bài hoặc story đọc được có `image_url` trỏ tới nó.

Bộ lọc ảnh, cách đọc multipart và cách lưu tệp giống hệt `POST /contexts/{context_id}/photos`; thẻ đó giữ bảng định dạng, thông điệp và bố cục kho tệp.

## Xác thực và quyền

1. Middleware idempotency, router, đọc form, `get_actor`, validation: như thẻ ảnh nhóm (400 multipart trước 401, `anonymous_multipart_without_boundary`; 422 `invalid_actor_roles`, `roles_unknown`).
2. `_require_permission("upload_personal_photo", actor, {"is_self": True})` (`services/api/app/api/service.py:1909-1912`; `services/api/app/domain/permissions.py:455-458`): chỉ còn vai trò → 403 `role_not_permitted` (`author_roles_guest`); quyền đi trước bộ lọc (`author_roles_empty_garbage` 403, không phải 415).
3. `sanitize_image`, ghi tệp, ghi hàng.

Không kiểm hàng `people` (xem Lỗi Python).

## Đầu vào

- Không có tham số path hay query.
- Thân `multipart/form-data`, trường `file`: thiếu phần hay sai tên → 422 `missing` (`author_no_parts`, `author_wrong_field_name`); giá trị chữ hay `application/x-www-form-urlencoded` → 422 `value_error` `Expected UploadFile` (`author_text_value_named_file`, `author_urlencoded`); hai phần `file` → phần cuối (`author_two_files` lưu JPEG 12×12).

## Đầu ra

- **201** `UploadedImageResponse`, thứ tự khoá `id`, `context_id`, `url`, `content_type`, `byte_size`, `width`, `height`, `created_at`:
  - `url` = `/people/{actor_id}/photos/{id}` (`person_photo_url`, `services/api/app/domain/photo_ref.py:77-78`), hai UUID viết thường có gạch nối;
  - `context_id` = `null`;
  - `content_type`/`byte_size`/`width`/`height` như ảnh nhóm: PNG alpha và GIF trong suốt → `image/png` (`author_png_alpha`, `author_gif_transparent`); WebP orientation 6 64×48 → JPEG 48×64 (`author_webp_orientation_6`); PGM chữ 2×2, WebP 1×1 (`author_pgm_ascii`, `author_webp_1x1`).
- Phát lại idempotency: 201 `idempotency-replayed: true` (`author_idem_replay`).

## Tác dụng phụ

- Tệp trước, hàng sau (`service.py:1803-1830`). Hàng: `context_id` NULL, `owner_person_id` = `uploaded_by_id` = actor, `purpose` `personal`, `storage_key` 32 hex.
- `idempotency_keys` một hàng khi có header key và 201; scope `X-Actor-ID` thô, nên bạn dùng cùng khoá là request mới (`friend_idem_same_key` 201).
- Từ chối không ghi gì.
- Xoá tài khoản xoá mọi ảnh cá nhân (hàng và tệp) cùng bài và story của người đó.

## `url` được dùng ở đâu

- `POST /posts` nhận `image_url` dạng `/people/{id}/photos/{id}` chỉ khi id người là tác giả và hàng `purpose = 'personal'` tồn tại (`service.py:2340-2348`): bài `only_me` của chủ 201 (`author_posts_only_me_with_photo`), người khác đăng ảnh của chủ → 403 `permission_denied` `Chỉ đăng được ảnh của chính mình.` (`friend_posts_authors_photo`).
- `POST /stories` cùng quy tắc (`service.py:1987-2003`, `author_story_with_photo` 201).
- `GET /people/{person_id}/photos/{photo_id}` đọc ảnh; thẻ đó giữ quy tắc ai đọc được.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 400 | — | lỗi multipart của Starlette (xem thẻ ảnh nhóm) | Starlette |
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `services/api/app/api/deps.py:110-164` |
| 422 | `invalid_actor_roles`, `invalid_idempotency_key`, `idempotency_key_reuse`, validation `["body","file"]` | như thẻ ảnh nhóm | `deps.py`, `services/api/app/api/idempotency.py`, `services/api/app/api/main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:475-504` |
| 413 | `image_too_large` / `image_dimensions_too_large` | xem thẻ ảnh nhóm | `services/api/app/media/images.py` |
| 415 | `not_an_image` | `The uploaded bytes could not be decoded as a complete image.` | `images.py:67-71` |
| 500 | — | `Internal Server Error` (`text/plain; charset=utf-8`), khi actor không có hàng `people` | FK `fk_uploaded_images_owner` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` (`GET /people/me/photos`) | Starlette |
| 404 | — | `{"detail":"Not Found"}` cho `POST /people/{id}/photos` (không có route) | Starlette |
| 307 | — | `location: http://<Host>/people/me/photos` | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/photos.py:126-142` (`upload_personal_photo`)
- Service: `services/api/app/api/service.py:1909-1922`, `:1803-1830`, `:637-649`
- Domain: `services/api/app/domain/photo_ref.py:77-82`
- Repository: `services/api/app/api/repository.py:4504-4532`, `:4560-4570` (`get_person_image`, dùng bởi posts/stories)
- Quyền: `services/api/app/domain/permissions.py:455-458`

## Test đang phủ

- `services/api/tests/api/test_person_photos_gate.py`: `test_the_owner_reads_their_own_photo_and_nobody_else_does_until_a_post_shows_it` (104), `test_a_post_may_show_ones_own_photo_but_not_somebody_elses` (151), `test_the_url_is_stored_in_canonical_form` (206), `test_memories_and_chat_still_refuse_the_personal_shape` (221)
- `services/api/tests/api/test_stories.py` (ảnh của chính mình cho story)

## Kịch bản parity

`parity/scenarios/w6/photos/POST-people-me-photos.yaml`, id `w6/photos/post-people-me-photos` (39 bước, `dev`):

- Thứ tự: `anonymous_upload`, `anonymous_multipart_without_boundary`, `roles_unknown`, `author_roles_guest`, `author_roles_empty_garbage`, `author_roles_group_admin_only`.
- Định dạng: `author_jpeg`, `author_png_alpha`, `author_png_orientation_3`, `author_webp_orientation_6`, `author_gif_transparent`, `author_webp_1x1`, `author_pgm_ascii`.
- Từ chối tệp: `author_garbage`, `author_empty_file`, `author_bmp_truncated`, `author_over_cap`, `author_over_pixel_cap`.
- Phong bì: `author_no_parts`, `author_wrong_field_name`, `author_text_value_named_file`, `author_two_files`, `author_urlencoded`.
- Idempotency: `author_idem_first`, `author_idem_replay`, `author_idem_other_image`, `friend_idem_same_key`.
- `url` dùng tiếp: `author_posts_only_me_with_photo`, `author_story_with_photo`, `friend_posts_authors_photo`, `author_reads_own_photo`.
- Không có hàng `people`: `ghost_uploads` (500). Sau xoá tài khoản: `author_deletes_account`, `author_uploads_after_deletion` (dev: 201).
- Framework: `get_not_allowed`, `trailing_slash_redirects`, `person_id_in_path_not_found`.

`parity/scenarios/w6/crossreplay/POST-people-me-photos.yaml` (11 bước): khoá lưu qua Python được cửa trước phát lại (cùng `id` và `url`) và ngược lại; ảnh lưu qua một phía được chủ đọc qua phía kia; bài công khai tạo qua Python với `url` do cửa trước trả mở ảnh cho người lạ qua cả hai phía.

`parity/scenarios/w6/concurrency/POST-people-me-photos.yaml` (4 bước): ba bản cùng lúc là ba hàng, ba bản dưới một khoá là một hàng, hai bản rác là hai 415.

Cũng dùng route này: `parity/scenarios/w2/stories-photo/*` (PPM chữ), `w6/photos/prod-auth` (`owner_uploads_personal`, phát lại theo scope bearer).

## Chưa phủ / lưu ý cho bản Go

- `url` phải được in từ UUID của actor và của hàng mới, viết thường có gạch nối: `POST /posts` và `POST /stories` so `image_url` đã chuẩn hoá, còn `GET` ảnh cá nhân so bằng chuỗi (`posts.image_url = url`).
- 500 khi actor không có hàng `people` chỉ tới được ở `dev`.
- Kho tệp dùng chung và quyền tệp: xem thẻ `POST /contexts/{context_id}/photos`.
- Thân lớn, định dạng chưa phủ, `byte_size`: xem thẻ ảnh nhóm.

## Lỗi Python (chỉ báo, không sửa)

- Actor không có hàng `people` (dev, `ghost_uploads`): tệp được ghi, insert hỏng khoá ngoại → 500 và tệp mồ côi. Đo trên stack reference: 342 tệp, 336 hàng `uploaded_images` sau ba lượt chạy; sáu tệp thừa đúng bằng ba lượt × hai bước upload của người chưa đăng ký (`ghost_uploads` và `ghost_sets_own_avatar`).
- `dev`: tài khoản đã xoá vẫn tải được ảnh (`author_uploads_after_deletion` 201) và ảnh đó thuộc một người đã xoá.
- Thân request không có trần (xem thẻ ảnh nhóm).
