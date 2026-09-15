# PATCH /contexts/{context_id}

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

ADR-0021 §2.4: mọi thành viên ACTIVE đổi tên nhóm hoặc chọn bộ màu chat (`theme`). Cập nhật từng phần: trường nào có giá trị thì đổi trường đó. Một cặp (`kind = pair`, ADR-0021 §2.5) không có tên riêng nên không đổi tên được, nhưng vẫn đổi theme được.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (PATCH là write method, `services/api/app/api/idempotency.py:76`): key rỗng hoặc dài hơn 255 → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 `json_invalid` trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401 (`anonymous_unknown_context`). 401 thắng lỗi path (`anonymous_non_uuid_context`) và thắng cả after-validator: body `{}` không header vẫn 401 (`anonymous_empty_update`).
4. Path `context_id` và body `ContextUpdateRequest` validate cùng lúc, gom **một** mảng 422, path trước (`stranger_non_uuid_and_empty_update`: `uuid_parsing` rồi `value_error`). Vì chạy trước handler, người lạ gửi `{}` vào nhóm thật nhận 422 chứ không 403 (`stranger_empty_update_real_context`).
5. `_require_permission("edit_context", {"is_group_member": is_member(context_id, actor)})` (`services/api/app/api/service.py:1594-1598`; bảng `services/api/app/domain/permissions.py:337-340`, role `group_admin` hoặc `member`). Role trước predicate (`permissions.py:673-677`): role rỗng → 403 `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` là đủ (`mate_roles_group_admin_only` → 200). `is_member` đọc bảng: `state = active` và `left_at IS NULL` (`services/api/app/api/repository.py:2769-2782`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header` → 403). Membership được hỏi **trước** khi đọc dòng context: id không tồn tại và id thật cùng một 403 `is_group_member` (`stranger_unknown_context`, `stranger_uppercase_unknown_context`, `stranger_real_context`). Người được mời chưa nhận (`invitee_renames`) và người đã rời (`leaver_renames`) đều 403.
6. Có `display_name` → `_require_group_kind` (`service.py:1477-1491`): context là pair → 409 `not_a_group` (`owner_renames_pair`), kể cả khi body có thêm theme (`owner_renames_and_themes_pair`, không ghi gì). Chỉ theme trên pair → 200 (`owner_themes_pair`, `mate_themes_pair`). Người lạ trên pair → 403 trước 409 (`stranger_renames_pair`, `stranger_themes_pair`).
7. Theme ngoài danh sách → 422 `theme_unknown` (`service.py:1604-1609`): không tới được qua HTTP, `Literal` đã chặn ở bước 4.
8. `update_context` trả None → 404 `context_not_found` (`service.py:1611-1612`): không tới được, thành viên ACTIVE nghĩa là dòng context tồn tại (FK `fk_memberships_context`).

## Đầu vào

- Path `context_id` (UUID lax: chữ hoa được chấp nhận và đi tiếp tới 403).
- Header: `Idempotency-Key` tuỳ chọn.
- Body `ContextUpdateRequest` (`services/api/app/api/schemas.py:416-433`), `extra="forbid"`:
  - `display_name`: `StrictStr` 1..200 code point, nullable, mặc định null (`schemas.py:422-424`).
  - `theme`: `ChatTheme = Literal["mac-dinh","hoang-hon","bien-dem","rung-thong","ruc-ro"]` (`schemas.py:413`), nullable (`schemas.py:425`). Sai hoa thường → `literal_error` (`theme_wrong_case`); số → `literal_error` (`theme_number`).
  - After-validator `_something_to_change` (`schemas.py:427-433`): cả hai null → 422 `value_error` `"Value error, cần ít nhất một trường để sửa"` (`empty_update`, `both_null`); `display_name` chỉ khoảng trắng → `"Value error, tên nhóm không được rỗng"` (`blank_name`). Cả hai có `loc: ["body"]` và `ctx: {"error": {}}`.
  - Một lỗi trường làm after-validator không chạy: `blank_name_plus_extra_field` chỉ có `extra_forbidden` ở `["body","kind"]`.
  - Một trường null, trường kia có giá trị: trường null bị bỏ qua (`owner_theme_with_null_name`, `owner_name_with_null_theme`).

## Đầu ra

- **200** `ContextResponse` (`schemas.py:450-457`), thứ tự khoá `id`, `display_name`, `created_by_id`, `created_at`, `theme`, `kind`, `counterpart`.
  - `display_name` được `.strip()` trước khi lưu và trả (`service.py:1603`; `mate_renames`).
  - Nhóm: `kind: "group"`, `counterpart: null`. Pair: `display_name` là tên người kia đọc từ `people` lúc gọi và `counterpart: {"id","display_name"}` (`service.py:1389-1419`).
  - `created_at` là giá trị DB lúc tạo, không đổi; pydantic UTC `Z`.
- Không float.
- Replay idempotency: 200 + body đã lưu + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối (`location: http://<Host>/contexts/<id>`); `PUT`, `DELETE`, `POST` → 405 `allow: PATCH` (Starlette chỉ báo route khớp một phần đầu tiên, dù cùng path còn có GET).

## Tác dụng phụ

- UPDATE `contexts.display_name` và/hoặc `contexts.theme` (`services/api/app/api/repository.py:2602-2616`); CHECK `context_theme_known` là lớp hai của danh sách theme (`services/api/app/db/models.py:1068-1072`). Gửi lại đúng giá trị cũ vẫn là 200 và không có delta DB (`owner_same_theme_again`).
- SELECT `memberships` (`is_member`), `contexts` (kiểm pair và cập nhật), với pair thêm `memberships` + `people` (người kia).
- 409 xảy ra trước UPDATE: không ghi gì.
- Commit trước response.
- Idempotency: key → `idempotency_keys`, chỉ lưu 2xx; 409/422 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`). Fingerprint gồm path và body: cùng key với body khác (`idem_reuse_different_body`) hoặc sang context khác (`idem_reuse_other_context`) → 422 reuse.
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:1594-1598`, `:504` |
| 409 | `not_a_group` | `Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.` | `service.py:1487-1491` |
| 422 | `theme_unknown` | `Bộ màu này không có trong bộ của Rủ Đi.` (không tới được) | `service.py:1606-1608` |
| 404 | `context_not_found` | `Context does not exist` (không tới được) | `service.py:1612` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework (không `input`): `uuid_parsing`, `value_error` (after-validator, chuỗi tiếng Việt có tiền tố `Value error, `), `string_too_short`, `string_too_long`, `string_type`, `literal_error` (`ctx.expected` = `'mac-dinh', 'hoang-hon', 'bien-dem', 'rung-thong' or 'ruc-ro'`), `extra_forbidden`, `model_attributes_type`, `json_invalid`.

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:47-59`
- Service: `services/api/app/api/service.py:1584-1613` (`update_context`), `:1477-1491` (`_require_group_kind`), `:1389-1419` (`_context_response`), `:475-504`
- Domain: `services/api/app/domain/permissions.py:337-340`, `:654-678`; `services/api/app/domain/chat_theme.py:14-19`; `services/api/app/domain/direct.py:58-103`
- Repository: `services/api/app/api/repository.py:2602-2616`, `:2596-2600`, `:2749-2782`
- Schema: `services/api/app/api/schemas.py:410-457`

## Test đang phủ

- `services/api/tests/api/test_context_settings.py`: `test_a_member_can_choose_a_theme_and_the_group_remembers_it` (45), `test_a_member_can_rename_the_group` (55), `test_both_at_once_is_one_update` (63), `test_an_empty_update_is_refused` (73), `test_a_theme_outside_the_five_is_refused_at_the_boundary` (79), `test_a_blank_name_is_refused` (87), `test_a_field_this_model_does_not_name_is_refused` (93), `test_a_stranger_holding_the_id_is_refused_before_the_row_is_read` (100), `test_the_wire_enum_and_the_domain_tuple_are_the_same_list` (115)
- `services/api/tests/api/test_direct_messages.py::test_roster_doors_are_closed_on_a_pair_but_the_theme_still_changes` (169)

## Kịch bản parity

`parity/scenarios/w3/contexts/PATCH-contexts-context_id.yaml`, id `w3/contexts/patch-contexts-context_id` (68 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401 trước 422 path), `anonymous_malformed_json` (422 trước 401), `anonymous_empty_update` (401 trước after-validator), `anonymous_empty_idempotency_key`, `stranger_non_uuid_context`, `stranger_empty_update_real_context` (422 trước 403), `stranger_non_uuid_and_empty_update` (hai lỗi gom một).
- 403: `stranger_unknown_context`, `stranger_real_context`, `stranger_claims_context_header`, `stranger_uppercase_unknown_context`, `invitee_renames`, `leaver_renames`, `mate_roles_empty`.
- Đường vui: `mate_renames` (strip), `mate_sets_theme`, `owner_both_at_once`, `owner_theme_with_null_name`, `owner_name_with_null_theme`, `owner_same_theme_again`, `mate_roles_group_admin_only`, `owner_reads_after`.
- Validate: `empty_update`, `both_null`, `blank_name`, `empty_name`, `name_201_letters`, `name_number`, `theme_unknown`, `theme_wrong_case`, `theme_number`, `extra_field_kind`, `blank_name_plus_extra_field`, `top_level_array`.
- Pair: `owner_asks_mate_to_be_friends`, `mate_accepts_friendship`, `owner_opens_pair_with_mate`, `owner_renames_pair` (409), `owner_renames_and_themes_pair` (409), `owner_themes_pair`, `mate_themes_pair` (200), `stranger_themes_pair`, `stranger_renames_pair` (403).
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_reuse_other_context`, `idem_refusal_not_stored` (409) + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects` (307), `put_not_allowed`, `delete_not_allowed` (405).
- Chuẩn bị: năm `register_*`, `owner_creates_group`, `owner_invites_mate` + `mate_accepts`, `owner_invites_invitee`, `owner_invites_leaver` + `leaver_accepts` + `leaver_leaves`.

`prod` (`w3/contexts/prod-auth`): `mate_renames`, `idem_first`, `idem_replay`, `stranger_same_key` (scope là digest bearer: request của người lạ chạy thật và bị 403).

## Chưa phủ / lưu ý cho bản Go

- `.strip()` của Python bỏ mọi ký tự `str.isspace()`, gồm cả `\x1c`..`\x1f` và `\x85`; `strings.TrimSpace` của Go thì không bỏ `\x1c`..`\x1f`. Cả kiểm "tên rỗng" lẫn giá trị lưu đều dùng strip này. Chưa có bước nào gửi các ký tự đó.
- Thông điệp after-validator là tiếng Việt, có tiền tố `Value error, ` và `ctx.error` là `{}`; bản Go phải phát đúng như vậy, và không được chạy kiểm này khi đã có lỗi trường.
- `404 context_not_found` và `422 theme_unknown` không tới được qua HTTP; giữ để khớp nếu thứ tự kiểm tra thay đổi.
- Header `allow` của 405 là `PATCH` (route khớp đầu tiên), không phải `GET, PATCH`.
- 409 in-flight và hai lần PATCH đồng thời chưa phủ.
