# PUT /people/{person_id}

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Gắn một tên cho một id người mà người gọi đã giữ (id của người tham gia được client tạo trước khi ai gõ tên). Tạo mới: thành viên nào cũng được; lặp cùng tên: trả lời như cũ; đổi tên: chỉ chính người đó. Tài khoản đã kết thúc không được đặt tên lại (ADR-0023 §2.2.3).

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:269-298`, `services/api/app/api/service.py:1282-1325`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): khoá rỗng 422 trước mọi thứ (`anonymous_empty_idempotency_key`); phát lại 201 đã lưu (`owner_replays_key`); tên khác cùng khoá → 422 reuse (`owner_key_other_name`).
2. Router: `PUT /people/me` khớp route này (không có `PUT` cho literal `me`) → 422 `uuid_parsing` (`owner_put_me_literal`); đuôi `/` → 307; `POST`, `PATCH` trên id → 405.
3. FastAPI giải mã JSON trước dependency: 422 `json_invalid` kể cả ẩn danh (`anonymous_malformed_json`).
4. `get_actor` → 401 trước lỗi path và lỗi thân (`anonymous_register`, `anonymous_bad_body_fields`); 422 `invalid_actor_id`, `invalid_actor_roles` (`member,khong-co` cũng lạ), `invalid_actor_contexts`.
5. Path và thân cùng một lượt validation (lỗi gộp một danh sách): `person_id` UUID; `PersonRegistrationRequest` (`services/api/app/api/schemas.py:333-343`): `display_name: StrictStr`, 1..200 **code point**, không strip, `extra=forbid` (`owner_body_missing`, `owner_name_empty`, `owner_name_number`, `owner_name_null`, `owner_extra_field`, `owner_name_201_code_points`).
6. `get_person(person_id)` (`services/api/app/api/repository.py:2525-2527`), chưa kiểm quyền:
   - có và `deleted_at` có → 404 `person_not_found` `Chưa có ai dùng số này trong Rủ Đi.` (`service.py:1296-1306`), với mọi người gọi, mọi tên, kể cả không vai trò (`leaver_reclaims_old_name`, `owner_names_leaver_anonymous_name`, `stranger_without_roles_names_leaver`);
   - không có → `_require_permission("register_person_identity", {})` (`services/domain/permissions.py:136`): cần `member` hoặc `group_admin`, thiếu → 403 `permission_denied` `role_not_permitted` (`owner_roles_guest_new`, `owner_roles_empty_new`); rồi `create_person` → **201**; `IntegrityError` (hai máy cùng tạo) → 409 `person_already_exists` `Person identity conflicted` (`service.py:1307-1314`, chỉ khi đua);
   - có, cùng `display_name` (so chính xác) → **200** không kiểm quyền (`service.py:1315-1319`; `owner_retry_same_name`, `stranger_retries_owner_name_without_roles`);
   - có, tên khác → `_require_permission("rename_person_identity", {"is_self": …})` (`permissions.py:141-144`): thiếu vai trò → 403 `role_not_permitted` (`owner_renames_self_as_guest`), người khác → 403 `permission_denied` `is_self` (`stranger_renames_owner`, `owner_renames_friend_after`); rồi `rename_person` (`repository.py:2566-2574`, khoá hàng) → **200**; hàng biến mất giữa chừng → 404 `person_not_found` `Person disappeared during rename` (không đo được).

## Đầu vào

- Path `person_id`: UUID.
- Thân JSON `{"display_name": "<1..200 code point>"}`. Khoảng trắng được giữ nguyên: `"   "` là một tên hợp lệ (`blank_renames_to_spaces`). NUL (`U+0000`) qua được validation và làm Postgres từ chối → 500 (`blank_renames_nul`).

## Đầu ra

**201** (vừa tạo) hoặc **200** (đã có, dù đổi tên hay không), thân `PersonResponse` (`schemas.py:346-349`, dựng ở `people.py:294-298`), thứ tự khoá: `id`, `display_name`, `created_at`. `id` in lại dạng chuẩn.

## Tác dụng phụ

- Tạo: INSERT `people(id=person_id, display_name)` trong savepoint; các cột khác theo mặc định của bảng (`wall_comment_policy='readers'`, `discoverable_by_phone=true`, `created_at=now()` của DB).
- Đổi tên: UPDATE `people.display_name` dưới `FOR UPDATE`. Lặp cùng tên: không ghi.
- Tên mới hiện ngay ở mọi nơi đọc tên lúc đọc (pair, danh sách chặn, trang khách, roster).

## Idempotency

Tự nhiên idempotent với cùng tên. Khoá header: 201 lưu và phát lại là 201 (`crossreplay/PUT-people-person_id.yaml`). Ba lần lặp cùng tên hoặc ba lần tự đổi sang một tên mới đồng thời: ba 200 (`concurrency/PUT-people-person_id.yaml`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | | `deps.py:144-164` |
| 422 | (validation) | `json_invalid`, `uuid_parsing`, `missing`, `string_type`, `string_too_short`, `string_too_long`, `extra_forbidden` | `main.py:319-351` |
| 404 | `person_not_found` | `Chưa có ai dùng số này trong Rủ Đi.` | `service.py:1296-1306` |
| 403 | `permission_denied` | `role_not_permitted` / `is_self` | `service.py:1308`, `:1320-1322` |
| 409 | `person_already_exists` | `Person identity conflicted` (chỉ khi đua) | `service.py:1309-1314` |
| 500 | — | `Internal Server Error` (NUL trong tên) | Postgres |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:269-298`
- Service: `services/api/app/api/service.py:1282-1325`
- Repository: `services/api/app/api/repository.py:2525-2574`
- Quyền: `services/api/app/domain/permissions.py:136-144`

## Test đang phủ

- `services/api/tests/postgres/test_delete_account_postgres.py`: `test_an_ended_id_cannot_be_claimed_or_renamed_by_anybody` (234)
- Tầng fake dùng route này làm bước dựng trong gần như mọi tệp `services/api/tests/api/`.

## Kịch bản parity

`parity/scenarios/w10/people/PUT-people-person_id.yaml`, id `w10/people/put-people-person_id` (45 bước, `dev`):

- Thứ tự dây: `anonymous_register` (401), `anonymous_malformed_json` (422 `json_invalid` trước 401), `anonymous_bad_body_fields` (401 trước lỗi path và thân), `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `contexts_not_uuid`.
- Path và thân: `owner_path_not_uuid`, `owner_put_me_literal`, `owner_body_missing`, `owner_name_empty`, `owner_name_number`, `owner_name_null`, `owner_extra_field`, `owner_text_plain`, `owner_body_array`, `owner_name_201_code_points`.
- Tạo: `owner_roles_guest_new`, `owner_roles_empty_new` (403), `owner_registers_with_key` (201), `owner_replays_key`, `owner_key_other_name`.
- Lặp và đổi tên: `owner_retry_same_name` (200), `stranger_retries_owner_name_without_roles` (200), `stranger_renames_owner` (403 `is_self`), `owner_renames_self_as_guest` (403 `role_not_permitted`), `owner_renames_self`, `owner_renames_self_as_group_admin` (200).
- Đặt tên người khác: `owner_names_friend` (201), `friend_renames_self` (200), `owner_renames_friend_after` (403).
- Nội dung tên: `blank_registers_200_code_points` (201), `blank_renames_to_spaces` (200, lưu `"   "`), `blank_renames_escaped_unicode` (thoát JSON được giải mã, trả UTF-8 thật), `blank_renames_nul` (500), `blank_reads_own_profile_after_nul` (tên trước đó còn nguyên).
- Tài khoản đã kết thúc: `leaver_registers`, `leaver_ends_account`, `leaver_reclaims_old_name`, `owner_names_leaver_anonymous_name`, `stranger_without_roles_names_leaver` (404).
- Framework: `trailing_slash` (307), `post_not_allowed`, `patch_on_id_not_allowed` (405 `allow: GET`), `owner_reads_own_profile`.

`crossreplay/PUT-people-person_id.yaml` (9 bước): 201 lưu qua Python phát lại qua cửa trước; 200 đổi tên lưu qua cửa trước phát lại qua Python; 403 `is_self` nhả khoá cho một đăng ký 201 của chính người đó.

`concurrency/PUT-people-person_id.yaml` (6 bước): ba lần lặp cùng tên → ba 200 không ghi; ba lần tự đổi sang một tên → ba 200, một cập nhật; ba lần đăng ký mới cùng khoá → một chèn, hai phát lại 201.

`prod-auth.yaml`: `anonymous_registers` (401), `owner_renames_self` (hàng do harness gieo), `stranger_renames_owner_claiming_owner` (403 `is_self`), `owner_names_a_new_id` (201 với id là một uuid context vừa bind), `owner_reclaims_name_after` (401), `mate_names_owner_after` (404).

Corpus sinh: **loại trừ** (`excluded`): bước hợp lệ của bộ sinh đăng ký id giữ chỗ dùng chung `aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee` thành một người, làm mọi bước «id không tồn tại» chạy sau nó (kể cả của sóng khác) đổi nhánh. Đo được ở lượt đầu: `crossreplay/POST-people-person_id-block` `front_unknown_person_refused` thành 200. 422 viết tay ở trên.

## Chưa phủ / lưu ý cho bản Go

- Kiểm tài khoản đã kết thúc **trước** vai trò; lặp cùng tên **trước** quyền đổi tên; vai trò chỉ được hỏi khi tạo mới hoặc đổi tên.
- So tên bằng so chuỗi chính xác, không strip, không chuẩn hoá Unicode.
- Độ dài đếm code point (200 `ệ` qua, 201 không).
- NUL: Postgres từ chối → 500 `Internal Server Error` text/plain; transaction rollback.
- Không đưa vào làn đồng thời ba lần đăng ký mới không khoá: người thua trả 409 `person_already_exists` hoặc 200 tuỳ việc lần đọc của nó thấy commit của người thắng.
- Chưa phủ: 404 `Person disappeared during rename` (không có cửa xoá hàng `people`), id viết hoa của người có thật.

## Lỗi Python (chỉ báo, không sửa)

- Tên không được strip và toàn khoảng trắng được lưu (`PATCH /people/me` thì từ chối).
- NUL trong tên là 500 thay vì 422.
- Lặp cùng tên trả 200 kèm `id`, `display_name`, `created_at` cho bất kỳ ai, không cần vai trò: một oracle xác nhận id nào đang mang tên nào.
- Câu 404 cho tài khoản đã kết thúc nói về số điện thoại («Chưa có ai dùng số này trong Rủ Đi.») trên một route không nhận số.
