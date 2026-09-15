# POST /contexts/{context_id}/photos

photos · core · trạng thái trong bộ nhớ: không có

## Mục đích

Thành viên đang hoạt động của nhóm tải một ảnh nhóm lên. Ảnh được giải mã và dựng lại điểm ảnh bằng Pillow trước khi lưu, nên không metadata nào của điện thoại (GPS, máy chụp, giờ chụp, ICC, tEXt, EXIF) qua được ranh giới lưu trữ. Câu trả lời đưa `url` mà tường kỷ niệm (`POST /contexts/{id}/memories`) và bài đăng nhóm (`POST /posts`, audience `group` của đúng nhóm đó) nhận.

Thẻ này giữ phần chung của cả ba route upload: bộ lọc ảnh (`app/media/images.py`), cách đọc multipart, và cách lưu tệp (`app/media/storage.py`). Hai thẻ upload còn lại chỉ ghi phần khác.

## Xác thực và quyền

Thứ tự đo được trên stack, từ ngoài vào trong:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`), chỉ khi có header `Idempotency-Key`:
   - khoá rỗng hoặc dài hơn 255 → 422 `invalid_idempotency_key` (`owner_idem_empty_key`);
   - middleware **đọc hết thân** vào bộ nhớ và băm nguyên byte (thân multipart không được chuẩn hoá như JSON), nên cùng ảnh mà khác tên tệp là request khác → 422 `idempotency_key_reuse` (`owner_idem_same_image_other_filename`);
   - cùng khoá, cùng thân, đã xong 2xx → phát lại 201 với `idempotency-replayed: true` (`owner_idem_replay`); 415 không được lưu và nhả khoá (`owner_idem_refusal_not_stored`, `owner_idem_same_key_after_refusal`);
   - scope là digest của bearer, không thì giá trị thô của `X-Actor-ID`.
2. Router Starlette: đuôi `/` → 307, `location` tuyệt đối `http://<Host>/contexts/<id>/photos` (`trailing_slash_redirects`); `GET`, `PUT` → 405 `allow: POST`.
3. FastAPI đọc form (`request.form()`) **trước mọi dependency**, nên lỗi multipart đi trước 401:
   - `multipart/form-data` không có boundary → 400 `{"detail":"Missing boundary in multipart."}`, kể cả người không danh tính (`anonymous_multipart_without_boundary`);
   - thân dùng boundary khác header → 400 `{"detail":"There was an error parsing the body"}` (`owner_body_uses_another_boundary`);
   - phần thiếu `name` trong Content-Disposition → 400 `{"detail":"The Content-Disposition header field \"name\" must be provided."}` (`owner_part_without_name`);
   - thân thiếu dấu kết `--boundary--` → không có phần nào → 422 `missing` (`owner_body_without_closing_delimiter`).
4. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod); 422 `invalid_actor_roles` (`roles_unknown`). 401 đi trước 422 của path (`anonymous_non_uuid_context`).
5. Validation: path `context_id` (UUID lax) và trường `file` (`File()`) nằm chung một danh sách 422, lỗi path trước (`stranger_non_uuid_context_no_file`).
6. Route đọc tối đa `MAX_UPLOAD_BYTES + 1` byte của phần `file` (`services/api/app/api/routes/photos.py:30-40`).
7. `_require_permission("post_group_memory", …)` (`services/api/app/api/service.py:1832-1838`; `services/api/app/domain/permissions.py:423-426`): vai trò trước (`group_admin` hoặc `member`) → 403 `role_not_permitted`; rồi `is_group_member` = membership `ACTIVE` và `left_at IS NULL` của đúng `context_id` (`services/api/app/api/repository.py:2769-2782`) → 403 `is_group_member`.
   - Quyền **đi trước** bộ lọc ảnh: người ngoài gửi rác hay thân quá 10 MiB vẫn nhận 403 (`stranger_unknown_context_garbage`, `stranger_unknown_context_over_cap`).
   - Nhóm không tồn tại cũng là 403 `is_group_member`, không phải 404 (`stranger_unknown_context`).
   - Người được mời chưa nhận, người đã rời, người lạ có `X-Actor-Contexts` chứa nhóm: đều 403 (`invitee_uploads_before_accepting`, `leaver_uploads_after_leaving`, `stranger_with_contexts_header`). Chỉ `group_admin` trong header vẫn tải được (`mate_roles_group_admin_only`).
8. `sanitize_image` (`services/api/app/media/images.py:30-86`), xem mục Bộ lọc ảnh.
9. Ghi tệp, rồi ghi hàng (`service.py:1803-1830`).

## Đầu vào

- Path `context_id`: UUID lax của pydantic.
- Thân `multipart/form-data`, một trường bắt buộc `file` (đọc bởi FastAPI 0.115.6, Starlette 0.41.3, python-multipart 0.0.20):
  - thiếu trường, sai tên (`photo`), khác hoa thường (`FILE`), không có phần nào, content-type không phải multipart (`application/json`, không có header) → 422 `[{"type":"missing","loc":["body","file"],"msg":"Field required"}]` (`owner_wrong_field_name`, `owner_uppercase_field_name`, `owner_no_parts`, `owner_json_body`, `owner_no_content_type`);
  - `file` là giá trị chữ chứ không phải tệp (phần không có `filename`, kể cả giá trị rỗng, hoặc `application/x-www-form-urlencoded` `file=…`) → 422 `[{"type":"value_error","loc":["body","file"],"msg":"Value error, Expected UploadFile, received: <class 'str'>","ctx":{"error":{}}}]` (`owner_text_value_named_file`, `owner_empty_text_value_named_file`, `owner_urlencoded_file_value`);
  - hai phần `file` → **phần cuối** thắng (`owner_two_files_garbage_first` 201, `owner_two_files_garbage_last` 415);
  - trường khác bị bỏ qua (`owner_extra_field_beside_file`);
  - `filename` và `Content-Type` của phần không được đọc: PNG khai là `image/jpeg` vẫn lưu `image/png`, phần không có Content-Type và `filename=""` vẫn là tệp (`owner_png_declared_as_jpeg`, `owner_no_part_content_type`, `owner_empty_filename`).
- Header `Idempotency-Key` tuỳ chọn.

## Bộ lọc ảnh (chung cho ba route upload)

`sanitize_image` (`images.py:30-86`), Pillow 12.2.0 trong ảnh ghim:

1. `len(raw) > 10485760` → 413 `image_too_large`. Đúng 10485760 byte vẫn được giải mã; đệm số 0 sau EOI của JPEG hay sau IEND của PNG không làm hỏng ảnh (`owner_jpeg_padded_to_cap`, `owner_png_padded_to_cap` 201; `owner_one_byte_over_cap`, `owner_far_over_cap` 413).
2. `Image.open` (thử mọi plugin Pillow có, không theo tên tệp hay content-type). Cảnh báo `DecompressionBombWarning` bị nâng thành lỗi.
3. `width * height > 50000000` → 413 `image_dimensions_too_large`. Ba ngưỡng đều ra **cùng** 413 với cùng thông điệp: chặn của route (> 50000000, `owner_png_claims_one_row_over_pixel_cap` 10001×5000), cảnh báo của Pillow (> 89478485, `owner_png_claims_past_pillow_warning` 10000×10000), lỗi của Pillow (> gấp đôi 89478485, `owner_png_claims_past_pillow_error` 20000×10000; `owner_jpeg_claims_huge`, `owner_gif_claims_huge` 65535×65535). Đúng 50000000 điểm ảnh qua được chặn rồi hỏng khi giải mã dữ liệu thiếu → 415 (`owner_png_claims_exactly_pixel_cap`, `owner_bmp_claims_exactly_pixel_cap`).
4. `load()`, `ImageOps.exif_transpose`: Orientation 2..8 lật/xoay, 5..8 đổi chiều rộng và cao (`owner_jpeg_orientation_5` … `_8`: 64×48 thành 48×64); 1 và 9 không đổi (`owner_jpeg_orientation_1`, `_9`). EXIF được đọc từ APP1 của JPEG, chunk `eXIf` của PNG, chunk `EXIF` của WebP (`owner_png_orientation_6`, `owner_webp_alpha_orientation_8`).
5. Có alpha khi band có `A`, hoặc mode `P` có `transparency` → `RGBA`, nếu không → `RGB`; `Image.frombytes` dựng lại điểm ảnh.
6. Mọi lỗi khác (không nhận ra định dạng, tệp rỗng, cắt cụt, rác) → 415 `not_an_image` (`owner_garbage`, `owner_empty_file`, `owner_jpeg_truncated`, `owner_png_truncated`, `owner_jpeg_head_then_zeros`, `owner_webp_truncated`).
7. Mã hoá: `RGBA` → PNG (tham số mặc định của Pillow), `RGB` → JPEG `quality=88, optimize=True`.

Bảng đo (đầu vào sinh bởi harness; đầu ra là `content_type` của câu trả lời):

| Đầu vào | Mode Pillow | Đầu ra | Bước |
|---|---|---|---|
| JPEG màu, JPEG xám, JPEG 1×1 | RGB / L | `image/jpeg` | `owner_jpeg`, `owner_jpeg_gray`, `owner_jpeg_1x1` |
| PNG RGB, PNG xám, PNG bảng màu | RGB / L / P | `image/jpeg` | `mate_uploads_png`, `owner_png_gray`, `owner_png_palette` |
| PNG RGBA (kể cả 1×1 trong suốt hoàn toàn) | RGBA | `image/png` | `owner_png_rgba`, `owner_png_1x1_transparent` |
| PNG bảng màu có tRNS | P + transparency | `image/png` | `owner_png_palette_transparent` |
| GIF, GIF có màu trong suốt | P / P + transparency | `image/jpeg` / `image/png` | `owner_gif`, `owner_gif_transparent` |
| WebP lossless, có và không alpha | RGB / RGBA | `image/jpeg` / `image/png` | `owner_webp`, `owner_webp_alpha` |
| BMP 24 bit | RGB | `image/jpeg` | `owner_bmp` |
| PPM (P3), PGM (P2), PBM (P1) dạng chữ | RGB / L / 1 | `image/jpeg` | `owner_ppm_ascii`, `owner_pgm_ascii`, `owner_pbm_ascii` |

| code | status | detail (nguyên văn) |
|---|---|---|
| `image_too_large` | 413 | `Image exceeds the 10485760-byte upload limit.` |
| `image_dimensions_too_large` | 413 | `Image exceeds the 50000000-pixel limit.` |
| `not_an_image` | **415** (không phải 422) | `The uploaded bytes could not be decoded as a complete image.` |

Ánh xạ mã sang status ở `service.py:507-513`. Không có nhánh 422 nào của bộ lọc.

## Đầu ra

- **201** `UploadedImageResponse` (`services/api/app/api/schemas.py:1334-1342`, dựng ở `service.py:637-649`), JSON gọn, thứ tự khoá `id`, `context_id`, `url`, `content_type`, `byte_size`, `width`, `height`, `created_at`:
  - `url` = `/contexts/{context_id}/photos/{id}`, hai UUID viết thường có gạch nối (in từ UUID đã parse, không phải chuỗi path);
  - `context_id` = nhóm trong path;
  - `content_type` `image/jpeg` hoặc `image/png`;
  - `byte_size` = độ dài byte **sau khi Pillow nén lại**, không phải độ dài tệp gửi lên (JPEG 64×48 gửi lên được đệm tới 10 MiB vẫn ra `byte_size` 1600);
  - `width`, `height` sau transpose;
  - `created_at` ISO 8601 UTC `Z`, 6 chữ số.
- Phát lại từ middleware: 201, `idempotency-replayed: true`, thân là byte đã lưu.
- Từ chối của route (`ApiProblem`) là JSON gọn `{"code":…,"detail":…}`; từ chối của middleware idempotency là `json.dumps` có khoảng trắng.
- 400 lỗi multipart và 405/404 framework là `{"detail":"…"}`.

## Tác dụng phụ

- **Tệp trước, hàng sau** (`service.py:1817-1830`): `PhotoStorage.write` rồi `create_uploaded_image` (`repository.py:4504-4532`).
- Hàng `uploaded_images` (`services/api/app/db/models.py:1477-1556`):
  - `id` uuid4, `storage_key` = `secrets.token_hex(16)` (32 hex thường, unique; làn DB so dạng `<hex32#n>`);
  - `context_id` = nhóm, `owner_person_id` NULL, `uploaded_by_id` = actor, `purpose` `group`;
  - `content_type`, `byte_size`, `width`, `height` như câu trả lời; `created_at` = đồng hồ của service.
  - Ràng buộc: đúng một chủ (`context_id` hoặc `owner_person_id`), `content_type IN ('image/jpeg','image/png')`, kích thước dương, `purpose` trong ba giá trị và `purpose = 'group'` ⇔ có `context_id`.
- `idempotency_keys` một hàng khi có header key và câu trả lời 2xx.
- Mọi từ chối không ghi tệp nào và không ghi hàng nào (403, 413, 415 đều trước `write`).
- Xoá tài khoản người tải **không** xoá ảnh nhóm: `erase_person` chỉ xoá hàng có `owner_person_id` của họ (`repository.py:4770-4886`).

## Lưu trữ tệp (chung cho sáu route)

- Gốc: `MOBILE_MEDIA_ROOT`, mặc định `~/.local/share/rudi/media`, được `resolve()` (`services/api/app/media/storage.py:16-25`). Ảnh Docker tạo sẵn `/var/lib/rudi/media` thuộc `app` (uid 10001) để mount volume.
- Đường dẫn: `<root>/<key[0:2]>/<key[2:4]>/<key>`; key phải khớp `[0-9a-f]{32}`, không thì `ValueError` (`storage.py:74-79`).
- Ghi (`storage.py:38-59`): `mkdir(parents=True)`, `NamedTemporaryFile` tên `.<key>.<ngẫu nhiên>.tmp` trong cùng thư mục, `fsync`, `os.replace`; tệp tạm bị xoá nếu lỗi. Tệp mang quyền của `NamedTemporaryFile`, tức **0600**, thư mục theo umask của tiến trình. Số đo trên stack nằm trong mục «Chưa phủ / lưu ý cho bản Go».
- Đọc: `read_bytes`. Tệp mất (`FileNotFoundError`) hoặc rỗng → 404 của route đọc (`service.py:652-679`); lỗi đọc khác (quyền) → 500.
- Xoá: chỉ khi xoá tài khoản (`service.py:4528-4568`), sau khi hàng đã mất; lỗi unlink chỉ ghi log.
- Stack parity (`scripts/parity_stacks.sh:117-124`): mỗi container API chạy với `MOBILE_MEDIA_ROOT=/tmp/parity-media` **trong container**, không có volume. Reference và candidate có kho tệp riêng, mất khi `down`. `core` (Go) chạy trên host, không thấy `/tmp` của container.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 400 | — | `Missing boundary in multipart.` / `There was an error parsing the body` / `The Content-Disposition header field "name" must be provided.` | Starlette `request.form()` |
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `deps.py:110-164`; `service.py:4453-4474` |
| 422 | `invalid_actor_roles` (và `invalid_actor_id`, `invalid_actor_contexts`) | `X-Actor-Roles contains an unknown role` | `deps.py:144-163` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:404-553` |
| 422 | (validation) | `uuid_parsing` ở `["path","context_id"]`; `missing` hoặc `value_error` ở `["body","file"]` | `services/api/app/api/main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:475-504` |
| 413 | `image_too_large` | `Image exceeds the 10485760-byte upload limit.` | `images.py:33-37` |
| 413 | `image_dimensions_too_large` | `Image exceeds the 50000000-pixel limit.` | `images.py:44-48`, `:62-66` |
| 415 | `not_an_image` | `The uploaded bytes could not be decoded as a complete image.` | `images.py:67-71` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/photos.py:44-61` (`upload_context_photo`), `:30-40` (`_read_upload`)
- Service: `services/api/app/api/service.py:1832-1848`, `:1803-1830` (`_store_uploaded_image`), `:637-649`, `:507-513`, `:475-504`
- Repository: `services/api/app/api/repository.py:2769-2782` (`is_member`), `:4504-4532` (`create_uploaded_image`)
- Media: `services/api/app/media/images.py:1-86`, `services/api/app/media/storage.py:1-79`
- Quyền: `services/api/app/domain/permissions.py:423-426`, `:654-678`
- Mô hình: `services/api/app/db/models.py:1477-1556`; schema `services/api/app/api/schemas.py:1334-1342`

## Test đang phủ

- `services/api/tests/postgres/test_photo_upload_postgres.py`: `test_gps_is_gone_from_the_bytes_a_member_reads_back` (159), `test_the_file_on_disk_is_stripped_too_not_only_the_response` (190), `test_the_stored_filename_cannot_be_derived_from_the_group_or_the_uploader` (216), `test_a_person_who_is_not_an_active_member_can_neither_upload_nor_read` (242), `test_a_shell_script_wearing_an_image_content_type_is_refused` (333), `test_a_body_over_the_cap_is_refused` (361), `test_a_member_uploads_and_the_photo_becomes_a_memory_on_the_wall` (384)
- `services/api/tests/media/test_image_sanitizer.py` (58-198): GPS, máy chụp, orientation, tEXt, alpha, rác, JPEG đầu thật thân rác, thân rỗng, trần 10 MiB
- `services/api/tests/media/test_photo_storage.py` (23-104): gốc ngoài repo, key ngẫu nhiên, đường dẫn không chứa id, key dạng đường dẫn bị từ chối

## Kịch bản parity

Tệp ảnh không nằm trong git: bước dùng `body_parts` và harness sinh byte từ tham số (`parity/internal/scenario/parts.go`, `parity/internal/runner/imagegen.go`).

`parity/scenarios/w6/photos/POST-contexts-context_id-photos.yaml`, id `w6/photos/post-contexts-context_id-photos` (105 bước, `dev`):

- Thứ tự: `anonymous_upload`, `anonymous_non_uuid_context`, `anonymous_multipart_without_boundary` (400 trước 401), `anonymous_json_body`, `roles_unknown`, `stranger_non_uuid_context`, `stranger_non_uuid_context_no_file`, `stranger_unknown_context`, `stranger_unknown_context_garbage`, `stranger_unknown_context_over_cap`, `stranger_roles_empty_unknown_context`.
- Thang quyền: `stranger_uploads`, `stranger_with_contexts_header`, `invitee_uploads_before_accepting`, `leaver_uploads_while_member` (201) rồi `leaver_uploads_after_leaving` (403), `mate_roles_guest`, `mate_roles_empty`, `mate_roles_group_admin_only`, `mate_uploads_png`.
- Định dạng: `owner_jpeg` … `owner_pbm_ascii` (bảng trên); orientation `owner_jpeg_orientation_1` … `_9`, `owner_png_orientation_6`, `owner_webp_alpha_orientation_8`; tên và content-type của phần: `owner_png_declared_as_jpeg`, `owner_no_part_content_type`, `owner_empty_filename`.
- Từ chối tệp: `owner_garbage`, `owner_empty_file`, `owner_jpeg_truncated`, `owner_png_truncated`, `owner_jpeg_head_then_zeros`, `owner_webp_truncated`; trần byte `owner_jpeg_padded_to_cap`, `owner_png_padded_to_cap`, `owner_one_byte_over_cap`, `owner_far_over_cap`; trần điểm ảnh `owner_png_claims_*`, `owner_jpeg_claims_huge`, `owner_gif_claims_huge`, `owner_bmp_claims_*`.
- Phong bì: `owner_no_content_type`, `owner_json_body`, `owner_urlencoded_file_value`, `owner_no_parts`, `owner_wrong_field_name`, `owner_uppercase_field_name`, `owner_text_value_named_file`, `owner_empty_text_value_named_file`, `owner_two_files_garbage_first`, `owner_two_files_garbage_last`, `owner_extra_field_beside_file`, `owner_body_uses_another_boundary`, `owner_part_without_name`, `owner_body_without_closing_delimiter`.
- Idempotency: `owner_idem_first`, `owner_idem_replay`, `owner_idem_same_image_other_filename`, `owner_idem_refusal_not_stored`, `owner_idem_same_key_after_refusal`, `owner_idem_empty_key`.
- `url` dùng tiếp: `owner_hangs_photo_on_wall` (`POST /contexts/{id}/memories`), `owner_posts_photo_to_group` (`POST /posts`, audience `group`, ảnh đã xoay).
- Framework: `trailing_slash_redirects`, `get_collection_not_allowed`, `put_collection_not_allowed`.

`parity/scenarios/w6/crossreplay/POST-contexts-context_id-photos.yaml` (13 bước): 201 lưu dưới header key qua Python được cửa trước phát lại và ngược lại; ảnh khác hoặc tên tệp khác dưới cùng khoá là 422; 415 nhả khoá; ảnh lưu qua một phía được đọc bằng `GET` qua phía kia (`front_reads_python_upload`, `python_reads_front_upload`).

`parity/scenarios/w6/concurrency/POST-contexts-context_id-photos.yaml` (6 bước): bốn bản cùng lúc là bốn hàng (`upload_four_at_once`, `upload_four_named_at_once`), ba bản dưới một khoá là một 201 và hai phát lại, một hàng (`upload_same_key_at_once`), ba bản vượt trần điểm ảnh là ba 413 (`refuse_three_at_once`). Ba lượt reference không có bước RACY.

`parity/scenarios/w6/photos/prod-auth.yaml`: xem thẻ `GET /people/{person_id}/photos/{photo_id}`.

## Chưa phủ / lưu ý cho bản Go

- **Kho tệp dùng chung là điều kiện để bật route.** Khi Go phục vụ một route ảnh còn Python phục vụ route khác, `core` phải đọc tệp Python ghi và ngược lại (các bước `via: python` trong `w6/crossreplay` đòi đúng điều đó). Stack parity hiện không làm được: tệp nằm trong `/tmp` của container API, `core` chạy trên host. Script stack cần:
  - mount một thư mục host (ví dụ `$work/<role>-media`) vào container API tại `MOBILE_MEDIA_ROOT`, và truyền cùng thư mục đó cho `core` qua `MOBILE_MEDIA_ROOT`;
  - khớp quyền: Python chạy uid 10001 và ghi tệp 0600, nên `core` (uid của host) không đọc được tệp Python ghi, và Python không ghi được vào thư mục `core` tạo nếu quyền khác. Cần chạy hai bên cùng uid (`docker run --user`) hoặc đổi chế độ quyền, và ghi quyết định đó vào ADR-0029.
  - Chưa đổi script ở sóng này.
- Bản Go phải ghi đúng bố cục `<root>/<k0k1>/<k2k3>/<key>`, key 32 hex thường từ 16 byte ngẫu nhiên, ghi qua tệp tạm cùng thư mục + fsync + rename, quyền 0600, và **ghi tệp trước khi insert hàng**.
- `byte_size` là độ dài đầu ra của Pillow. ADR-0029 §2.8 cho JPEG lệch byte (SSIM), nhưng thân JSON và hàng DB mang `byte_size` phải bằng nhau; mâu thuẫn này đã ghi trong `docs/codex/QUEUE.md` và chờ số đo nén lại giống từng byte.
- Pillow nhận nhiều định dạng hơn năm định dạng đã phủ (AVIF, BLP, BMP, BUFR, CUR, DCX, DDS, DIB, EPS, FITS, FLI, FTEX, GBR, GIF, GRIB, HDF5, ICNS, ICO, IM, IMT, IPTC, JPEG, JPEG2000, MCIDAS, MPEG, MSP, PCD, PCX, PIXAR, PNG, PPM, PSD, QOI, SGI, SPIDER, SUN, TGA, TIFF, WEBP, WMF, XBM, XPM, XVTHUMB theo `Image.OPEN` trong ảnh ghim). Danh sách định dạng Go không giải mã được là câu hỏi mở 3 của ADR-0029 §8.
- Chưa phủ: WebP lossy (VP8) và có chunk ALPH, WebP/GIF động (khung đầu), JPEG progressive/CMYK/có ICC, PNG 16 bit/LA/APNG/tEXt, TIFF có orientation, orientation chỉ trong XMP, `Image.open` của EPS khi không có ghostscript. Harness sinh được JPEG, PNG, GIF, WebP lossless, BMP; phần còn lại cần mở rộng bộ sinh.
- Chưa phủ: 409 `idempotency_request_in_flight` (lịch chạy), lỗi ghi tệp (đĩa đầy, quyền) → 500 không có hàng, thân chunked (harness luôn gửi `Content-Length`).
- Thứ tự Go phải giữ: lỗi multipart 400 → 401 → 422 (path trước body) → 403 → 413/415. Trần 10 MiB chỉ được xét **sau** quyền; thân lớn hơn vẫn phải được nhận hết.
- Hai phần `file`: phần cuối thắng. Giá trị chữ, kể cả rỗng, là `value_error` chứ không phải `missing`.

## Lỗi Python (chỉ báo, không sửa)

- Trần 10 MiB không giới hạn thân request: Starlette/python-multipart đọc và spool **cả** phần tệp trước khi `_read_upload` cắt, và middleware idempotency đọc cả thân vào bộ nhớ khi có header key. Thân 16 MB (`owner_far_over_cap`) được nhận hết rồi mới 413; người ngoài nhóm cũng gửi được thân lớn tuỳ ý (403 sau khi đã đọc xong). Chú thích «Retain only enough bytes» dễ đọc thành có giới hạn.
- `not_an_image` trả 415 và `image_dimensions_too_large` trả 413 (Payload Too Large cho một ảnh nhỏ khai kích thước lớn); client phải phân biệt hai 413 theo `code`.
- Tệp được ghi trước khi insert: nếu insert lỗi, tệp mồ côi ở lại kho (đo ở thẻ `POST /people/me/photos`, `ghost_uploads`).
- Nhóm không tồn tại trả 403 `is_group_member` thay vì 404; nhất quán với các route nhóm khác, chỉ ghi nhận.
