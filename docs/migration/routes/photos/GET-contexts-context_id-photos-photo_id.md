# GET /contexts/{context_id}/photos/{photo_id}

photos · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trả nguyên byte của một ảnh nhóm (tệp bộ lọc đã ghi lúc upload) cho thành viên đang hoạt động của đúng nhóm trong path. Đây là `url` mà `POST /contexts/{context_id}/photos` trả về và tường kỷ niệm, bài đăng nhóm hiển thị.

## Xác thực và quyền

Thứ tự đo được:

1. Router: đuôi `/` → 307, `location` tuyệt đối (`trailing_slash_redirects`); `HEAD`, `POST`, `DELETE` → 405 `allow: GET` (`head_not_allowed` có `content-length: 31` và thân rỗng, `post_not_allowed`, `delete_not_allowed`). FastAPI không tự thêm `HEAD` cho route `GET`.
2. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 đi trước 422 của path (`anonymous_non_uuid_both`); 422 `invalid_actor_roles` (`roles_unknown`).
3. Path `context_id`, `photo_id`: UUID lax, lỗi cả hai nằm chung một danh sách, `context_id` trước (`stranger_non_uuid_both`).
4. `_require_permission("view_group_memories", …)` (`services/api/app/api/service.py:1850-1856`; `services/api/app/domain/permissions.py:427-430`): vai trò (`group_admin`/`member`) → 403 `role_not_permitted` (`mate_roles_guest`, `mate_roles_empty`); `is_group_member` (membership `ACTIVE`, `left_at IS NULL` của `context_id` trong path, `services/api/app/api/repository.py:2769-2782`) → 403 `is_group_member`.
   - Quyền **đi trước** tra ảnh: người ngoài nhận 403 cho id không tồn tại, nhóm không tồn tại, và cả ảnh của chính họ đặt dưới nhóm họ không thuộc (`stranger_unknown_both`, `stranger_reads_unknown_photo_in_real_group`, `other_own_photo_under_first_group`).
   - Người được mời chưa nhận, người lạ có `X-Actor-Contexts`, người đã rời (kể cả với ảnh do chính họ tải lên), người đã xoá tài khoản: 403 (`invitee_reads`, `stranger_with_contexts_header`, `leaver_reads_after_leaving`, `leaver_reads_own_photo_after_leaving`, `mate_reads_after_deleting_account`).
5. `get_context_image(context_id, photo_id)` (`repository.py:4534-4543`) lọc theo **cả hai** cột → không có → 404 `photo_not_found`. Ảnh của nhóm khác dưới path nhóm này, avatar hay ảnh cá nhân dưới path nhóm đều 404, kể cả với người thuộc cả hai nhóm (`owner_other_groups_photo_under_own_group`, `both_other_groups_photo_under_first_group`, `both_first_groups_photo_under_other_group`, `owner_avatar_under_group_path`, `owner_personal_photo_under_group_path`).
6. `_stored_image_bytes` (`service.py:652-679`): tệp mất hoặc rỗng → cùng 404 `photo_not_found`.

## Đầu vào

- Path `context_id`, `photo_id`: UUID lax của pydantic. Viết hoa và không gạch nối được parse và đi tới tra cứu (`owner_uppercase_unknown_photo`, `owner_unhyphenated_unknown_photo` → 404); thiếu một ký tự → 422 `Input should be a valid UUID, invalid group length in group 4: expected 12, found 11` (`owner_short_photo_id`).
- Không query, không body. `Range`, `If-None-Match`, `If-Modified-Since`, `Accept` bị bỏ qua: luôn 200 với cả tệp (`mate_reads_with_range`, `mate_reads_with_if_none_match`, `mate_reads_with_accept_json`).

## Đầu ra

- **200**, thân là byte tệp đã lưu, so từng byte trong kịch bản. Header (`services/api/app/api/routes/photos.py:73-81`):
  - `content-type`: `image/jpeg` hoặc `image/png`, lấy từ cột `content_type` của hàng, không có `charset`, không đoán từ byte;
  - `content-length`;
  - `cache-control: private, max-age=300`;
  - **không** có `etag`, `last-modified`, `accept-ranges`.
- Người đọc khác nhau nhận cùng byte (`owner_reads_jpeg`, `mate_reads_owners_jpeg`, `leaver_reads_while_member`).
- Từ chối: JSON gọn `{"code":…,"detail":…}`.

## Tác dụng phụ

Chỉ đọc `memberships`, `uploaded_images` và tệp. Không idempotency (GET), không limiter.

Ảnh nhóm sống lâu hơn người tải: người tải rời nhóm hay xoá tài khoản, ảnh vẫn đọc được với thành viên còn lại (`owner_reads_leavers_photo`, `owner_reads_deleted_mates_photo`), vì `erase_person` chỉ xoá hàng có `owner_person_id`.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `deps.py:110-164` |
| 422 | `invalid_actor_roles` (và `invalid_actor_id`, `invalid_actor_contexts`) | `X-Actor-Roles contains an unknown role` | `deps.py:144-163` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","context_id"]` / `["path","photo_id"]` | `services/api/app/api/main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:475-504` |
| 404 | `photo_not_found` | `Photo does not exist` | `service.py:1860-1866`, `:652-679` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: GET` | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/photos.py:64-81` (`read_context_photo`), `:27` (`_PRIVATE_CACHE_HEADERS`)
- Service: `services/api/app/api/service.py:1850-1867`, `:652-679` (`_stored_image_bytes`), `:475-504`
- Repository: `services/api/app/api/repository.py:2769-2782` (`is_member`), `:4534-4543` (`get_context_image`)
- Media: `services/api/app/media/storage.py:61-62`, `:74-79`
- Quyền: `services/api/app/domain/permissions.py:427-430`

## Test đang phủ

- `services/api/tests/postgres/test_photo_upload_postgres.py`: `test_gps_is_gone_from_the_bytes_a_member_reads_back` (159), `test_a_person_who_is_not_an_active_member_can_neither_upload_nor_read` (242), `test_a_member_of_another_group_cannot_read_this_groups_photo` (276), `test_an_unknown_photo_id_inside_a_group_the_actor_belongs_to_is_404` (312)
- `services/api/tests/api/test_photo_bytes_present_but_empty.py`: `test_the_covered_routes_are_the_ones_that_actually_serve_bytes` (215), `test_h_a_healthy_photo_still_arrives` (228), `test_d_a_zero_byte_file_answers_exactly_what_a_missing_file_answers` (240)

## Kịch bản parity

`parity/scenarios/w6/photos/GET-contexts-context_id-photos-photo_id.yaml`, id `w6/photos/get-contexts-context_id-photos-photo_id` (70 bước, `dev`):

- Thứ tự: `anonymous_read`, `anonymous_non_uuid_both`, `roles_unknown`, `stranger_non_uuid_photo`, `stranger_non_uuid_both`, `stranger_unknown_both`, `stranger_roles_empty_unknown_both`.
- Chuẩn bị: nhóm của `owner` với `mate`, `invitee` (chưa nhận), `leaver`, `both`; nhóm của `other` có `both`. Ảnh JPEG, PNG alpha, JPEG xoay orientation 6 của `mate`, WebP của `leaver`, ảnh ở nhóm khác, một avatar và một ảnh cá nhân.
- Người đọc: `owner_reads_jpeg`, `owner_reads_png`, `mate_reads_owners_jpeg`, `owner_reads_mates_rotated`, `leaver_reads_while_member`, `both_reads_in_first_group`, `mate_roles_group_admin_only`, `mate_reads_with_if_none_match`, `mate_reads_with_range`, `mate_reads_with_accept_json`.
- Thang quyền: `invitee_reads`, `stranger_reads`, `stranger_with_contexts_header`, `stranger_reads_unknown_photo_in_real_group`, `mate_roles_guest`, `mate_roles_empty`.
- Tra theo (nhóm, id): `owner_unknown_photo`, `owner_uppercase_unknown_photo`, `owner_unhyphenated_unknown_photo`, `owner_short_photo_id`, `owner_other_groups_photo_under_own_group`, `both_other_groups_photo_under_first_group`, `both_reads_other_groups_photo`, `both_first_groups_photo_under_other_group`, `other_own_photo_under_first_group`, `owner_avatar_under_group_path`, `owner_personal_photo_under_group_path`.
- Rời và xoá: `leaver_leaves`, `leaver_reads_after_leaving`, `leaver_reads_own_photo_after_leaving`, `owner_reads_leavers_photo`, `mate_deletes_account`, `owner_reads_deleted_mates_photo`, `mate_reads_after_deleting_account`.
- Framework: `trailing_slash_redirects`, `head_not_allowed`, `post_not_allowed`, `delete_not_allowed`.

Đọc chéo: `w6/crossreplay/post-contexts-context_id-photos` đọc qua cửa trước ảnh lưu qua Python và ngược lại. Prod: `w6/photos/prod-auth` (`mate_reads_group_photo`, `stranger_reads_group_photo`, `anonymous_reads_group_photo`, `mate_reads_group_photo_after_sign_out`, `owner_still_reads_group_photo`).

## Chưa phủ / lưu ý cho bản Go

- Tệp mất hoặc rỗng (404 `photo_not_found`) và tệp không đọc được (500) không có kịch bản: harness không xoá được tệp trong container. Đã có test API ở `test_photo_bytes_present_but_empty.py`.
- Khi Go phục vụ route này mà upload vẫn ở Python (hoặc ngược lại), `core` phải đọc cùng kho tệp với Python; xem mục lưu trữ và lưu ý ở thẻ `POST /contexts/{context_id}/photos` (mount chung cùng đường dẫn, cùng uid của host, tệp 0600).
- Byte trả về phải là tệp nguyên vẹn, không mã hoá lại: ADR-0029 §2.8 đòi GET giống từng byte.
- Header phải khớp đúng ba dòng `content-type`, `content-length`, `cache-control: private, max-age=300`. `http.ServeContent`/`http.FileServer` của Go tự thêm `accept-ranges`, `last-modified`, xử lý `Range` và `If-Modified-Since`; không dùng được.
- `HEAD` phải là 405 `allow: GET` với thân rỗng.
- 403 trước 404: không được tra ảnh trước khi xét quyền.

## Lỗi Python (chỉ báo, không sửa)

- `HEAD` trả 405 dù `GET` hợp lệ; client hay proxy dò bằng `HEAD` sẽ thấy lỗi.
- Không có `etag`/`last-modified` nên `max-age=300` hết hạn là tải lại cả tệp; `Range` bị bỏ qua.
- Nhóm không tồn tại là 403 `is_group_member`, không phân biệt với nhóm có thật mà người gọi không thuộc; nhất quán với các route nhóm khác, chỉ ghi nhận.
