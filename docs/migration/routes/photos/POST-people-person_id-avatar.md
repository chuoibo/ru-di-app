# POST /people/{person_id}/avatar

photos · core · trạng thái trong bộ nhớ: không có

## Mục đích

Đặt ảnh đại diện của chính mình. Mỗi lần gọi thêm một hàng `uploaded_images` `purpose = 'avatar'`; hàng mới nhất là ảnh đại diện mà `GET /people/{person_id}/avatar` trả. Ảnh cũ không bị xoá.

Bộ lọc ảnh, cách đọc multipart và cách lưu tệp giống hệt `POST /contexts/{context_id}/photos`; thẻ đó giữ bảng định dạng, thông điệp và bố cục kho tệp.

## Xác thực và quyền

1. Middleware idempotency, router, đọc form, `get_actor`, validation: như thẻ `POST /contexts/{context_id}/photos` (400 multipart trước 401; 401 trước 422; lỗi path và body chung một danh sách, `owner_non_uuid_without_file`).
2. Path `person_id` là UUID: `/people/me/avatar` là 422 `uuid_parsing` (`owner_me_path`), còn người không danh tính vẫn 401 (`anonymous_me_path`).
3. `_require_permission("set_own_avatar", actor, {"is_self": actor.id == person_id})` (`services/api/app/api/service.py:1869-1875`; `services/api/app/domain/permissions.py:147-150`):
   - vai trò trước (`group_admin`/`member`) → 403 `role_not_permitted`, kể cả khi path là người khác (`owner_roles_guest_self`, `owner_roles_guest_other`);
   - rồi `is_self` → 403 `is_self` cho người khác và cho id không tồn tại (`owner_for_other_person`, `owner_for_unknown_person`);
   - quyền đi trước bộ lọc: rác gửi cho người khác là 403 (`owner_for_other_person_garbage`).
   - Không kiểm hàng `people`: người chưa đăng ký qua được quyền rồi hỏng ở insert (xem Lỗi Python).
4. `sanitize_image`, ghi tệp, ghi hàng (`service.py:1803-1830`).

## Đầu vào

- Path `person_id`: UUID lax.
- Thân `multipart/form-data`, trường `file` (xem thẻ ảnh nhóm): thiếu, sai tên, JSON → 422 `missing` (`owner_no_parts`, `owner_wrong_field_name`, `owner_json_body`); giá trị chữ → 422 `value_error` (`owner_text_value_named_file`); hai phần `file` → phần cuối (`owner_two_files` lưu PNG 12×12 của phần thứ hai).

## Đầu ra

- **201** `UploadedImageResponse`, thứ tự khoá `id`, `context_id`, `url`, `content_type`, `byte_size`, `width`, `height`, `created_at`:
  - `url` = `/people/{person_id}/avatar` (UUID viết thường có gạch nối), **không** chứa id hàng;
  - `context_id` = `null`;
  - `content_type`, `byte_size` (độ dài sau khi nén lại), `width`, `height` như ảnh nhóm (`owner_sets_rotated_jpeg` 64×40 orientation 8 → 40×64; `owner_sets_ppm_ascii` 1×1).
- Phát lại idempotency: 201 `idempotency-replayed: true` (`owner_idem_replay`).

## Tác dụng phụ

- Tệp trước, hàng sau. Hàng: `context_id` NULL, `owner_person_id` = `person_id` (= actor), `uploaded_by_id` = actor, `purpose` `avatar`, `storage_key` 32 hex.
- Hàng và tệp avatar cũ ở lại (`owner_sets_jpeg`, `owner_replaces_with_png_alpha`, … mỗi bước một hàng); `GET` đọc hàng mới nhất theo `created_at DESC, id DESC`.
- Từ chối (403/413/415/422) không ghi gì: `owner_reads_avatar_after_refusals` vẫn trả ảnh PPM đặt trước đó.
- `idempotency_keys` một hàng khi có header key và 201; scope là `X-Actor-ID` thô, nên cùng khoá của người khác là request mới (`other_idem_same_key_own_avatar`).
- Xoá tài khoản xoá mọi hàng và tệp avatar của người đó (`gone_reads_avatar_after_deletion` 404).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 400 | — | lỗi multipart của Starlette (xem thẻ ảnh nhóm) | Starlette |
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `services/api/app/api/deps.py:110-164` |
| 422 | `invalid_actor_roles`, `invalid_idempotency_key`, `idempotency_key_reuse` | như thẻ ảnh nhóm | `deps.py`, `services/api/app/api/idempotency.py` |
| 422 | (validation) | `uuid_parsing` ở `["path","person_id"]`; `missing`/`value_error` ở `["body","file"]` | `services/api/app/api/main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_self` | `service.py:475-504` |
| 413 | `image_too_large` / `image_dimensions_too_large` | xem thẻ ảnh nhóm | `services/api/app/media/images.py` |
| 415 | `not_an_image` | `The uploaded bytes could not be decoded as a complete image.` | `images.py:67-71` |
| 500 | — | `Internal Server Error` (`text/plain; charset=utf-8`), khi actor không có hàng `people` | FK `fk_uploaded_images_owner` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` (`PUT`, `DELETE`) | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/photos.py:84-101` (`set_person_avatar`)
- Service: `services/api/app/api/service.py:1869-1884`, `:1803-1830`, `:637-649`
- Repository: `services/api/app/api/repository.py:4504-4532`, `:4545-4558` (`get_latest_avatar`), `:4770-4886` (`erase_person`)
- Quyền: `services/api/app/domain/permissions.py:147-150`

## Test đang phủ

- `services/api/tests/postgres/test_photo_upload_postgres.py`: `test_a_person_sets_their_own_avatar_and_a_groupmate_can_see_it` (432), `test_nobody_may_set_somebody_elses_avatar` (470)
- `services/api/tests/api/test_person_photos_gate.py`: `test_a_personal_photo_is_not_an_avatar` (132)

## Kịch bản parity

`parity/scenarios/w6/photos/POST-people-person_id-avatar.yaml`, id `w6/photos/post-people-person_id-avatar` (47 bước, `dev`):

- Thứ tự: `anonymous_upload`, `anonymous_me_path`, `roles_unknown`, `owner_me_path`, `owner_non_uuid_without_file`, `owner_for_other_person`, `owner_for_other_person_garbage`, `owner_for_unknown_person`, `owner_roles_guest_self`, `owner_roles_guest_other`, `owner_roles_group_admin_only`.
- Đặt và thay: `owner_sets_jpeg`, `owner_reads_jpeg_avatar`, `owner_replaces_with_png_alpha`, `owner_reads_replaced_avatar`, `owner_sets_rotated_jpeg`, `owner_sets_webp`, `owner_sets_ppm_ascii`, `owner_reads_ppm_avatar`.
- Từ chối tệp: `owner_garbage`, `owner_empty_file`, `owner_gif_truncated`, `owner_over_cap`, `owner_over_pixel_cap`, `owner_reads_avatar_after_refusals`.
- Phong bì: `owner_no_parts`, `owner_wrong_field_name`, `owner_text_value_named_file`, `owner_two_files`, `owner_reads_after_two_files`, `owner_json_body`.
- Idempotency: `owner_idem_first`, `owner_idem_replay`, `owner_idem_other_image`, `other_idem_same_key_own_avatar`.
- Không có hàng `people`: `ghost_sets_own_avatar` (500), `ghost_reads_own_avatar` (404).
- Xoá tài khoản: `gone_sets_avatar`, `gone_deletes_account`, `gone_reads_avatar_after_deletion` (404), `gone_sets_avatar_after_deletion` (dev: 201).
- Framework: `trailing_slash_redirects`, `put_not_allowed`, `delete_not_allowed`.

`parity/scenarios/w6/crossreplay/POST-people-person_id-avatar.yaml` (8 bước): khoá lưu qua Python được cửa trước phát lại và ngược lại, ảnh khác dưới cùng khoá 422, và `GET` qua mỗi phía đọc đúng avatar mà request đã lưu đặt.

`parity/scenarios/w6/concurrency/POST-people-person_id-avatar.yaml` (5 bước): bốn bản cùng lúc là bốn hàng và `GET` sau đó trả cùng byte (bốn tệp cùng một ảnh); ba bản dưới một khoá là một hàng.

Prod: `w6/photos/prod-auth` (`owner_sets_avatar`, `owner_sets_mates_avatar` 403 `is_self`).

## Chưa phủ / lưu ý cho bản Go

- Hai avatar khác nhau có cùng `created_at`: `GET` chọn `id` lớn hơn. Kịch bản đồng thời dùng cùng một ảnh nên không thấy được thứ tự này.
- Bản Go phải để `set_own_avatar` xét vai trò trước `is_self`, và cả hai trước bộ lọc ảnh.
- 500 khi actor không có hàng `people` (chỉ tới được ở `dev`; ở `prod` phiên của người không tồn tại là 401). Thân 500 là của Starlette; uvicorn đóng kết nối sau câu trả lời này.
- Kho tệp dùng chung với Python và quyền tệp: xem thẻ `POST /contexts/{context_id}/photos`.

## Lỗi Python (chỉ báo, không sửa)

- Actor không có hàng `people` (dev, `ghost_sets_own_avatar`): quyền qua, tệp **đã ghi**, insert hỏng khoá ngoại → 500 `text/plain` và tệp mồ côi ở lại kho. Đo trên stack reference: 342 tệp cho 336 hàng sau ba lượt chạy, đúng bằng số bước upload của người chưa đăng ký.
- Thay avatar không xoá ảnh cũ: mọi khuôn mặt từng đặt vẫn nằm trong DB và trên đĩa tới khi xoá tài khoản.
- `dev`: tài khoản đã xoá vẫn đặt được avatar mới (`gone_sets_avatar_after_deletion` 201), vì hàng `people` chỉ bị ẩn danh. `prod` chặn bằng phiên.
- `405` trên path này (`PUT`, `DELETE`, và `HEAD`/`PATCH` ở thẻ GET) ghi `allow: POST` dù `GET` cũng tồn tại: Starlette lấy phương thức của route khớp path đầu tiên.
