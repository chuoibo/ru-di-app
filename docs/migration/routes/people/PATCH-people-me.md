# PATCH /people/me

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Sửa một phần hồ sơ của chính người gọi: tên hiển thị, giới thiệu, thành phố, ai được bình luận trên tường (ADR-0022 §2.2) và có cho tra theo số điện thoại không (ADR-0023 §2.5). Chuỗi rỗng hoặc toàn khoảng trắng ở `bio`/`city` là cách duy nhất để xoá câu đã viết.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:78-93`, `services/api/app/api/service.py:4079-4101`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): thân JSON được chuẩn hoá trước khi băm, nên cùng giá trị khác khoảng trắng vẫn là phát lại (`owner_replays_key_respaced_json`); giá trị khác → 422 reuse; từ chối nhả khoá (`owner_refused_with_key_two`, `owner_valid_with_key_two`).
2. Router: đuôi `/` → 307.
3. FastAPI giải mã JSON trước dependency → 422 `json_invalid` kể cả ẩn danh (`anonymous_malformed_json`).
4. `get_actor` → 401 trước lỗi model (`anonymous_patches`, `anonymous_empty_body_object`); 422 `invalid_actor_id` / `invalid_actor_roles`.
5. Model `ProfileUpdateRequest` (`services/api/app/api/schemas.py:911-935`), trước vai trò và trước hàng (`owner_empty_object_without_roles`):
   - trường: `display_name: StrictStr` 1..200, `bio: StrictStr` ≤ 500, `city: StrictStr` ≤ 120, `wall_comment_policy: Literal["readers","friends","nobody"]`, `discoverable_by_phone: StrictBool`, tất cả nullable, `extra=forbid`. Độ dài đếm **trước** strip (`owner_bio_500_before_strip` là 200). Lỗi trường gộp một danh sách (`owner_several_errors`) và khi có lỗi trường thì validator sau không chạy.
   - `model_validator(mode="after")`: mọi trường null → 422 `value_error` `Value error, cần ít nhất một trường để sửa` (`owner_empty_object`, `owner_all_null`); `display_name` toàn khoảng trắng → `Value error, tên hiển thị không được rỗng` (`owner_name_blank`). `loc` là `["body"]`.
   - Thân thiếu → `missing` `["body"]`; mảng hoặc `text/plain` → `model_attributes_type`; không `content-type` thì đọc như JSON (`owner_no_content_type` 200).
6. `_require_permission("edit_own_profile", {"is_self": True})` (`service.py:4084`; `services/api/app/domain/permissions.py:191`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_valid_without_roles`, `owner_as_group_admin_only`).
7. `update_person_profile` (`services/api/app/api/repository.py:3915-3926`, `FOR UPDATE`); không có hàng → 404 `person_not_found` `Chưa có hồ sơ cho tài khoản này.` (`service.py:4096-4100`; `owner_valid_unregistered`). Không đọc `deleted_at`: dev cho người đã xoá tài khoản đổi tên và mở lại hai cài đặt (`leaver_renames_after_erasure`).

## Đầu vào

Thân JSON, ít nhất một trường khác null:

| Trường | Kiểu | Ghi |
|---|---|---|
| `display_name` | chuỗi 1..200 | strip rồi lưu; toàn khoảng trắng → 422 |
| `bio` | chuỗi ≤ 500 | strip; rỗng sau strip → NULL |
| `city` | chuỗi ≤ 120 | strip; rỗng sau strip → NULL |
| `wall_comment_policy` | `readers` / `friends` / `nobody` | phân biệt hoa thường |
| `discoverable_by_phone` | bool | `"false"`, `0` → 422 |

`null` nghĩa là «không đổi», không phải «xoá» (`owner_sets_bio_only` giữ `city`).

## Đầu ra

**200** `ProfileResponse`, cùng hình dạng và thứ tự khoá với `GET /people/me` (xem thẻ đó), đọc sau khi ghi.

## Tác dụng phụ

UPDATE `people` chỉ các cột có trong thân (setattr từng khoá); ghi lại cùng giá trị vẫn là một UPDATE (không cột nào đổi thì SQLAlchemy không phát câu lệnh). Không audit event.

## Idempotency

Tự nhiên idempotent với cùng thân. Khoá header: 200 lưu và phát lại nguyên văn kể cả khi hàng đã đổi sau đó (`crossreplay/PATCH-people-me.yaml`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 422 | (validation) | `json_invalid`, `missing`, `string_type`, `string_too_short`, `string_too_long`, `literal_error`, `bool_type`, `extra_forbidden`, `model_attributes_type`, `value_error` | `main.py:319-351`; `schemas.py:922-935` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4084` |
| 404 | `person_not_found` | `Chưa có hồ sơ cho tài khoản này.` | `service.py:4096-4100` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:78-93`
- Service: `services/api/app/api/service.py:4079-4101`, `:4167-4189`
- Repository: `services/api/app/api/repository.py:3915-3926`
- Schema: `services/api/app/api/schemas.py:911-935`
- Quyền: `services/api/app/domain/permissions.py:191`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_patch_touches_only_the_fields_sent_and_clears_with_empty` (126), `test_patch_refuses_nothing_unknown_fields_and_a_blank_name` (151)
- `services/api/tests/postgres/test_profile_postgres.py`: `test_a_patch_persists_and_the_public_view_follows_the_relation` (231)

## Kịch bản parity

`parity/scenarios/w10/people/PATCH-people-me.yaml`, id `w10/people/patch-people-me` (46 bước, `dev`):

- Thứ tự: `anonymous_patches` (401), `anonymous_malformed_json` (422 `json_invalid`), `anonymous_empty_body_object` (401), `actor_id_not_uuid`, `roles_unknown`, `owner_empty_object_without_roles` (422 trước 403), `owner_valid_without_roles` (403), `owner_valid_unregistered` (404), `owner_body_missing`, `owner_body_array`.
- Model: `owner_empty_object`, `owner_all_null`, `owner_name_blank`, `owner_name_empty`, `owner_name_number`, `owner_name_too_long`, `owner_bio_too_long`, `owner_city_too_long`, `owner_policy_unknown`, `owner_policy_uppercase`, `owner_discoverable_string`, `owner_discoverable_zero`, `owner_unknown_field`, `owner_several_errors` (sáu lỗi theo thứ tự trường), `owner_text_plain`.
- Ghi: `owner_sets_everything` (strip tên, giới thiệu, thành phố; `<b>` giữ nguyên), `owner_sets_bio_only` (`city: null` giữ thành phố), `owner_clears_bio_with_blank_and_city_with_empty`, `owner_bio_500_before_strip` (200, lưu 498 ký tự), `owner_policy_readers_discoverable_true`, `owner_same_values_again` (200, không có delta DB), `owner_as_group_admin_only` (403), `owner_no_content_type` (200).
- Khoá: `owner_patches_with_key`, `owner_patches_other_city`, `owner_replays_key` (thành phố cũ), `owner_replays_key_respaced_json`, `owner_key_other_body`, `owner_refused_with_key_two`, `owner_valid_with_key_two`, `owner_reads_profile`.
- Sau xoá tài khoản (dev): `leaver_renames_after_erasure` (200).
- `trailing_slash`.

`crossreplay/PATCH-people-me.yaml` (11 bước), `concurrency/PATCH-people-me.yaml` (4 bước: ba patch giống nhau cùng lúc → ba 200 giống nhau, một cập nhật trong delta; ba lần cùng khoá → một ghi, hai phát lại).

`prod-auth.yaml`: `anonymous_bad_body_patch` (401 trước model), `owner_writes_profile` (`X-Actor-Roles: ''` bị bỏ qua), `owner_patch_with_key`, `mate_patch_same_key_own_session` (chạy lại trong scope của mate), `anonymous_patch_same_key` (401, scope `anonymous`), `owner_patches_after` (401), `owner_patch_key_replay_after` (200 phát lại sau khi phiên chết).

Corpus sinh: hoãn, `'function-after' is not probed`; 422 viết tay ở trên.

## Chưa phủ / lưu ý cho bản Go

- Validator sau (`mode="after"`) chạy chỉ khi mọi trường hợp lệ, `loc` là `["body"]`, `msg` có tiền tố `Value error, ` và `ctx.error` là `{}`.
- Độ dài đếm code point trước strip; strip là `str.strip()` của Python (khoảng trắng Unicode).
- `null` và vắng mặt như nhau; chuỗi rỗng ở `bio`/`city` là xoá.
- Chưa phủ: NUL trong `bio`/`city` (xem 500 của `PUT /people/{id}`), khoảng trắng Unicode ngoài ASCII khi strip.

## Lỗi Python (chỉ báo, không sửa)

- Giới hạn độ dài kiểm trên chuỗi chưa strip: một `bio` 499 ký tự với khoảng trắng hai đầu vượt 500 bị từ chối dù thứ được lưu ngắn hơn.
- Dev: người đã xoá tài khoản đổi được tên và mở lại `discoverable_by_phone`, `wall_comment_policy` trên hàng ẩn danh (`leaver_renames_after_erasure`).
- `PATCH` từ chối tên toàn khoảng trắng nhưng `PUT /people/{id}` chấp nhận và lưu nguyên.
