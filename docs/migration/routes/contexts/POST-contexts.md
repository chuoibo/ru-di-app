# POST /contexts

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tạo một nhóm. Người tạo trở thành thành viên ACTIVE với vai trò `admin` ngay trong cùng transaction: nhóm không bao giờ được sinh ra mà không có ai quản trị (`services/api/app/api/service.py:1569-1575`). Tên nhóm lưu **đúng như gửi**, không cắt khoảng trắng (khác `PATCH`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-553`): rỗng hoặc dài hơn 255 → 422 trước cả xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 `json_invalid`, **trước** xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): dev thiếu `X-Actor-ID` → 401; không phải UUID → 422 `invalid_actor_id` (`actor_id_not_uuid`); role lạ → 422 `invalid_actor_roles` (`roles_unknown`); `X-Actor-Contexts` không phải UUID → 422 `invalid_actor_contexts` (`contexts_header_not_uuid`). 401 thắng lỗi body: body `{}` không header vẫn 401 (`anonymous_invalid_body`).
4. Body `ContextCreateRequest` → 422 (`unregistered_invalid_body`: người chưa đăng ký gửi tên rỗng nhận 422, không phải 409).
5. `_require_permission("create_context", {})` (`service.py:1565`; bảng `services/api/app/domain/permissions.py:282`, role `group_admin` hoặc `member`, không predicate). Role rỗng → 403 `role_not_permitted` (`owner_roles_empty`), chỉ `advancer` → 403 (`owner_roles_advancer_only`), chỉ `group_admin` → 201 (`owner_roles_group_admin_only`). Chạy **trước** kiểm tra đăng ký: `unregistered_roles_empty` → 403.
6. `_require_registered_person(actor.id)` (`service.py:1373-1387`): không có dòng `people` → 409 `person_not_registered` (`unregistered_creates`). Chỉ hỏi dòng có tồn tại (`repository.py:2525-2527`), không nhìn `deleted_at`: ở dev, tài khoản đã xoá vẫn tạo được nhóm (`ghost_creates_after_deletion` → 201).
7. `accept_membership` trả None → 409 `creator_membership_missing` (`service.py:1576-1581`) — không tới được qua HTTP (dòng vừa chèn trong cùng transaction).

## Đầu vào

- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` (không có vẫn parse JSON → 201 `no_content_type`; `text/plain` → 422 `model_attributes_type`, `text_plain_content_type`).
- Body `ContextCreateRequest` (`services/api/app/api/schemas.py:406-407`), `extra="forbid"` (`schemas.py:66-67`): `display_name: StrictStr`, 1..200 **code point** (`name_200_letters` với 200 chữ `ạ` 3 byte → 201; `name_201_letters` → 422 `string_too_long`). Số → `string_type` (`name_number`), null → `string_type` (`name_null`), thiếu → `missing` (`missing_name`), `theme`/`kind` → `extra_forbidden` (`extra_field_theme`, `extra_field_kind`), mảng → `model_attributes_type` (`top_level_array`).
- Tên chỉ có khoảng trắng được chấp nhận (`name_only_spaces` → 201); khoảng trắng đầu/cuối được giữ (`name_with_spaces_kept`). Tên không duy nhất (`owner_creates_same_name_again` → 201).

## Đầu ra

- **201** `ContextResponse` (`schemas.py:450-457`), thứ tự khoá `id`, `display_name`, `created_by_id`, `created_at`, `theme`, `kind`, `counterpart`.
  - `id`: uuid4 do SQLAlchemy sinh (`models.py:1100-1102`).
  - `display_name`: nguyên văn body.
  - `created_by_id`: actor.
  - `created_at`: `contexts.created_at` = `now()` của Postgres (**giờ bắt đầu transaction**, `models.py:1115-1117`), không phải đồng hồ Python; pydantic UTC `Z`, 6 chữ số lẻ.
  - `theme: "mac-dinh"` (server default), `kind: "group"`, `counterpart: null`.
- Không float.
- Replay idempotency: 201 + body đã lưu + `idempotency-replayed: true`; body chỉ khác khoảng trắng JSON vẫn replay (`idem_replay_reordered_spaces`).
- Framework: 307 cho `/contexts/` (`location: http://<Host>/contexts`); `GET`, `PUT` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `contexts (id, display_name, created_by_id)`; `theme`, `kind`, `created_at` từ server default (`repository.py:2576-2583`).
- INSERT `memberships` trong savepoint: `role = admin`, `state = invited`, `origin = named`, `invited_by_id = actor`, `created_at = now()` (`repository.py:2672-2701`). Rồi `SELECT … FOR UPDATE` cùng dòng và UPDATE `state = active`, `joined_at = _now()` Python (`repository.py:2709-2725`, `service.py:1575`). Hệ quả thấy được ở làn DB: `joined_at` (đồng hồ Python) muộn hơn `created_at` (đồng hồ DB, đầu transaction).
- SELECT `people` hai lần (đăng ký; tên cho bản ghi membership, `repository.py:2529-2548`).
- Commit trước response (`services/api/app/api/unit_of_work.py:51`, `deps.py:196-209`).
- Idempotency: key → `idempotency_keys`, chỉ lưu 2xx; 409/422 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`). Scope là `X-Actor-ID` thô (`idem_same_key_other_actor` → ghi thật một nhóm nữa).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:147` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:152-154` |
| 422 | `invalid_actor_contexts` | `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:159-163` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:1565`, `:504` |
| 409 | `person_not_registered` | `Register this person with PUT /people/{person_id} first` | `service.py:1383-1387` |
| 409 | `creator_membership_missing` | `Creator membership disappeared during context creation` (không tới được) | `service.py:1577-1581` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework (không có `input`, `services/api/app/api/main.py:318-351`): `string_too_short` / `string_too_long` (`ctx.min_length` / `ctx.max_length`), `string_type`, `missing`, `extra_forbidden`, `model_attributes_type`, `json_invalid`. Lỗi của middleware có khoảng trắng sau `:` và `,`; lỗi route thì gọn.

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:33-44`
- Service: `services/api/app/api/service.py:1562-1582` (`create_context`), `:1373-1387`, `:1389-1419` (`_context_response`), `:475-504` (`_require_permission`)
- Domain: `services/api/app/domain/permissions.py:282`, `:654-678`
- Repository: `services/api/app/api/repository.py:2576-2583`, `:2672-2701`, `:2709-2725`, `:2525-2548`
- Model: `services/api/app/db/models.py:1054-1117` (`Context`), `:1120-1209` (`Membership`)

## Test đang phủ

- `services/api/tests/postgres/test_person_identity_postgres.py`: `test_registering_a_name_then_opening_a_group_works_over_http` (76), `test_opening_a_group_without_an_identity_is_refused_not_a_crash` (119)
- `services/api/tests/postgres/test_membership_role_postgres.py::test_whoever_creates_a_group_administers_it` (74)
- `services/api/tests/postgres/test_idempotency_postgres.py::test_the_app_replays_the_group_the_seed_created_instead_of_being_refused` (711)
- `services/api/tests/api/test_context_settings.py::test_a_new_group_starts_on_the_default_theme` (38)

## Kịch bản parity

`parity/scenarios/w3/contexts/POST-contexts.yaml`, id `w3/contexts/post-contexts` (50 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_creates`, `anonymous_malformed_json` (422 trước 401), `anonymous_empty_idempotency_key` (422 middleware trước 401), `anonymous_invalid_body` (401 trước 422 body), `actor_id_not_uuid`, `roles_unknown`, `contexts_header_not_uuid`, `unregistered_creates` (409), `unregistered_roles_empty` (403 trước 409), `unregistered_invalid_body` (422 trước 409).
- Đường vui: `owner_creates`, `owner_reads_members` (admin ACTIVE), `owner_reads_context`, `owner_creates_same_name_again`, `name_with_spaces_kept`, `name_only_spaces`, `name_200_letters`, `no_content_type`.
- Role: `owner_roles_empty`, `owner_roles_advancer_only` (403), `owner_roles_group_admin_only` (201).
- Validate: `name_empty`, `name_201_letters`, `name_number`, `name_null`, `missing_name`, `extra_field_theme`, `extra_field_kind`, `top_level_array`, `text_plain_content_type`.
- Tài khoản đã xoá: `ghost_creates_before_deletion`, `ghost_deletes_account`, `ghost_creates_after_deletion` (dev → 201).
- Idempotency: `idem_first`, `idem_replay`, `idem_replay_reordered_spaces`, `idem_reuse_different_body`, `idem_same_key_other_actor`, `idem_refusal_not_stored` + `register_late` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed`, `put_not_allowed` (405).
- Chuẩn bị: `register_owner`, `register_mate`, `register_ghost`.

`prod`: `parity/scenarios/w3/contexts/prod-auth.yaml`, id `w3/contexts/prod-auth`: `anonymous_create_missing_bearer`, `anonymous_malformed_json`, `anonymous_empty_idempotency_key`, `owner_creates_group_empty_roles_header` (header role rỗng bị bỏ qua → 201), `stranger_deletes_account` + `stranger_after_deletion` (401).

## Chưa phủ / lưu ý cho bản Go

- Hai đồng hồ: `contexts.created_at` và `memberships.created_at` là `now()` của Postgres (giờ bắt đầu transaction), `memberships.joined_at` là `datetime.now(UTC)` của Python sau đó. Làn DB so thứ hạng thời điểm, nên bản Go dùng một giá trị cho cả ba sẽ lệch.
- Độ dài tên đếm theo code point (pydantic), không theo byte hay UTF-16.
- Tên chỉ có khoảng trắng được lưu ở đây, trong khi `PATCH` từ chối; parity giữ nguyên cả hai (xem báo cáo lỗi nghi vấn).
- Ở prod, tài khoản đã xoá bị thu hồi phiên nên chỉ còn 401; nhánh `ghost_creates_after_deletion` → 201 chỉ có ở dev.
- 409 `creator_membership_missing`, 409 in-flight và hai request đồng thời cùng key không phủ được tuần tự.
