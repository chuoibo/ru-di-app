# GET /people/{person_id}/avatar

photos · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trả byte của ảnh đại diện mới nhất của một người, cho chính người đó và cho người đang cùng một nhóm đang hoạt động với họ. Không có id ảnh trong path: `url` avatar luôn là `/people/{person_id}/avatar`.

## Xác thực và quyền

Thứ tự đo được:

1. Router: đuôi `/` → 307, `location` tuyệt đối (`trailing_slash_redirects`); `HEAD`, `PATCH` → 405 với `allow: POST` (không phải `GET`: route `POST` cùng path được đăng ký trước, `head_not_allowed`, `patch_not_allowed`).
2. `get_actor`: 401 đi trước 422 của path (`anonymous_me_path`); 422 `invalid_actor_roles` (`roles_unknown`).
3. Path `person_id` UUID: `/people/me/avatar` → 422 `uuid_parsing` (`stranger_me_path`).
4. `_require_permission("view_person_avatar", …)` (`services/api/app/api/service.py:1886-1897`; `services/api/app/domain/permissions.py:151-154`):
   - vai trò (`group_admin`/`member`) → 403 `role_not_permitted`, kể cả khi đọc avatar của chính mình (`owner_roles_empty_reads_own`, `owner_roles_guest_reads_own`, `mate_roles_guest_reads_owner`);
   - `shares_a_group_with_subject` (`services/api/app/api/repository.py:2784-2808`): người đọc là chính chủ, **hoặc** hai người cùng có membership `state = ACTIVE` trong một context. Chỉ xét `state`, không xét `left_at` (rời nhóm đặt cả hai). Sai → 403 `shares_a_group_with_subject`.
   - Quyền **đi trước** tra avatar: người ngoài nhận 403 dù người kia chưa có avatar hay không tồn tại (`stranger_reads_owner_without_avatar`, `stranger_reads_unknown_person`).
   - Lời mời đang chờ không tính, theo cả hai chiều (`mate_reads_owner_while_invited`, `invitee_reads_owner`, `owner_reads_invitee`); đã rời không tính, theo cả hai chiều (`leaver_reads_owner_after_leaving`, `owner_reads_leaver_after_leaving`); `X-Actor-Contexts` không mở được gì (`stranger_with_contexts_header`).
5. `get_latest_avatar(person_id)` (`repository.py:4545-4558`): `owner_person_id = person_id AND purpose = 'avatar'`, `ORDER BY created_at DESC, id DESC LIMIT 1`; không có → 404 `avatar_not_found`.
   - Ảnh cá nhân và ảnh nhóm **không** phải avatar (`mate_reads_owner_after_personal_upload` vẫn trả avatar cũ, `owner_reads_mate_with_only_personal`, `owner_reads_mate_with_group_photo` → 404).
6. Tệp mất hoặc rỗng → cùng 404 `avatar_not_found` (`service.py:652-679`).

## Đầu vào

- Path `person_id`: UUID lax.
- Không query, không body. `If-None-Match` bị bỏ qua (`owner_reads_with_if_none_match` 200).

## Đầu ra

- **200**, thân là byte tệp đã lưu của hàng avatar mới nhất; header `content-type` (`image/jpeg`/`image/png` từ hàng), `content-length`, `cache-control: private, max-age=300`; không `etag`, `last-modified`, `accept-ranges` (`services/api/app/api/routes/photos.py:113-121`).
- Thay avatar thì `GET` trả ảnh mới ngay (`mate_reads_replaced` PNG sau JPEG). URL không đổi, còn `max-age=300`, nên client có thể giữ ảnh cũ tới 5 phút.

## Tác dụng phụ

Chỉ đọc `memberships`, `uploaded_images` và tệp. Không idempotency, không limiter.

Xoá tài khoản: hàng và tệp avatar bị xoá, membership chuyển `LEFT`; bạn cùng nhóm nhận 403 (không còn chung nhóm), chính chủ (dev) nhận 404 (`mate_reads_owner_after_deletion`, `owner_reads_own_after_deletion`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `services/api/app/api/deps.py:110-164` |
| 422 | `invalid_actor_roles` (và `invalid_actor_id`, `invalid_actor_contexts`) | `X-Actor-Roles contains an unknown role` | `deps.py:144-163` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","person_id"]` | `services/api/app/api/main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` / `shares_a_group_with_subject` | `service.py:475-504` |
| 404 | `avatar_not_found` | `Avatar does not exist` | `service.py:1898-1906` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/photos.py:104-121` (`read_person_avatar`)
- Service: `services/api/app/api/service.py:1886-1907`, `:652-679`
- Repository: `services/api/app/api/repository.py:2784-2808` (`shares_active_context`), `:4545-4558` (`get_latest_avatar`)
- Quyền: `services/api/app/domain/permissions.py:151-154`

## Test đang phủ

- `services/api/tests/postgres/test_photo_upload_postgres.py`: `test_a_person_sets_their_own_avatar_and_a_groupmate_can_see_it` (432), `test_a_stranger_cannot_read_an_avatar` (489)
- `services/api/tests/api/test_person_photos_gate.py`: `test_a_personal_photo_is_not_an_avatar` (132)
- `services/api/tests/api/test_photo_bytes_present_but_empty.py` (215-240)

## Kịch bản parity

`parity/scenarios/w6/photos/GET-people-person_id-avatar.yaml`, id `w6/photos/get-people-person_id-avatar` (50 bước, `dev`):

- Thứ tự: `anonymous_read`, `anonymous_me_path`, `roles_unknown`, `stranger_me_path`.
- Trước khi có avatar và nhóm: `owner_reads_own_before_any` (404), `owner_roles_empty_reads_own` (403), `stranger_reads_owner_without_avatar`, `stranger_reads_unknown_person` (403).
- Có avatar, chưa có nhóm: `owner_sets_avatar`, `owner_reads_own`, `owner_roles_guest_reads_own`, `stranger_reads_owner`.
- Nhóm: `mate_reads_owner_while_invited` (403), `mate_accepts`, `mate_reads_owner`, `mate_roles_group_admin_reads_owner`, `mate_roles_guest_reads_owner`, `owner_reads_mate_without_avatar` (404), `invitee_reads_owner`, `owner_reads_invitee`, `leaver_reads_owner_while_member`, `stranger_with_contexts_header`.
- Thay và loại ảnh: `owner_replaces_with_png_alpha`, `mate_reads_replaced`, `owner_uploads_personal_photo`, `mate_reads_owner_after_personal_upload`, `mate_uploads_only_personal`, `owner_reads_mate_with_only_personal`, `mate_uploads_group_photo`, `owner_reads_mate_with_group_photo`.
- Rời và xoá: `leaver_leaves`, `leaver_reads_owner_after_leaving`, `owner_reads_leaver_after_leaving`, `owner_reads_with_if_none_match`, `owner_deletes_account`, `mate_reads_owner_after_deletion`, `owner_reads_own_after_deletion`.
- Framework: `trailing_slash_redirects`, `head_not_allowed`, `patch_not_allowed`.

Cũng đọc avatar: `w6/photos/post-people-person_id-avatar` (sau mỗi lần thay và sau từ chối), `w6/crossreplay/post-people-person_id-avatar`, `w6/concurrency/post-people-person_id-avatar`, `w6/photos/prod-auth` (`mate_reads_owners_avatar`, `stranger_reads_owners_avatar`, `junk_bearer_avatar_read`).

## Chưa phủ / lưu ý cho bản Go

- `shares_active_context` xét `state = ACTIVE` mà không xét `left_at`, khác `is_member` của route ảnh nhóm; mọi đường rời nhóm hiện đặt cả hai cột nên kịch bản không phân biệt được. Bản Go nên chép đúng câu điều kiện.
- Thứ tự `created_at DESC, id DESC` khi hai avatar khác nhau có cùng `created_at` không có kịch bản.
- `405` ghi `allow: POST` cho path này: router Go phải lấy danh sách phương thức theo cách Starlette lấy (route khớp path đầu tiên), không gộp mọi route cùng path.
- Tệp mất/rỗng → 404 `avatar_not_found` chưa có kịch bản.
- Kho tệp dùng chung: xem thẻ `POST /contexts/{context_id}/photos`.

## Lỗi Python (chỉ báo, không sửa)

- `HEAD`/`PATCH` trả 405 với `allow: POST`, tức nói sai rằng path này không đọc được bằng `GET`.
- `max-age=300` trên một URL không đổi khi thay avatar: người khác thấy ảnh cũ tới 5 phút.
- Chính chủ có vai trò rỗng không đọc được avatar của mình (403 `role_not_permitted`); nhất quán với bảng quyền, chỉ ghi nhận.
