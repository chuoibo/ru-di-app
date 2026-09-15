# POST /contexts/{context_id}/members

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Quản trị viên của nhóm mời đích danh một người đã đăng ký. Kết quả là một dòng `memberships` ở trạng thái `invited`; người đó tự nhận qua `POST /memberships/{id}/accept`. Rời nhóm rồi quay lại là một dòng mới, không hồi sinh dòng cũ.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 `json_invalid` trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401; thắng lỗi path (`anonymous_non_uuid_context`) và lỗi body (`anonymous_invalid_body`).
4. Path `context_id` + body `MembershipInviteRequest` → một mảng 422, path trước (`stranger_non_uuid_and_bad_body`). 422 thắng 403 (`stranger_unknown_context_bad_body`).
5. `_require_permission("invite_context_member", {"is_group_member": …}, extra_roles=_group_admin_role(...))` (`services/api/app/api/service.py:1621-1635`; bảng `services/api/app/domain/permissions.py:283-286`: role **chỉ** `group_admin`, predicate `is_group_member`). Role của lần gọi là role header ∪ `{group_admin}` nếu người gọi là `admin` ACTIVE của **chính nhóm này** (`service.py:1359-1371`, `repository.py:2808-2821`). Role kiểm trước predicate (`permissions.py:673-677`), nên:
   - thành viên thường → 403 `role_not_permitted` (`mate_invites_invitee`);
   - người lạ với role mặc định → 403 `role_not_permitted`, không phải `is_group_member` (`stranger_unknown_context`);
   - người lạ tự khai `group_admin` → 403 `is_group_member` (`stranger_as_group_admin_unknown_context`, `stranger_as_group_admin_real_context`); người được mời (`invitee_as_group_admin_invites_leaver`) và người đã rời (`leaver_as_group_admin_invites_stranger`) cũng vậy;
   - **dev**: thành viên ACTIVE tự khai `group_admin` qua header mời được (`mate_as_group_admin_invites_invitee` → 201);
   - admin theo roster vẫn mời được khi header role rỗng (`owner_roles_empty_invites_leaver` → 201).
6. `_require_group_kind` (`service.py:1477-1491`): pair → 409 `not_a_group`. Dòng membership của pair có `role = member`, nên không tự khai `group_admin` thì người trong pair nhận 403 trước (`owner_invites_into_pair`); có header thì 409 (`owner_as_group_admin_invites_into_pair`), và 409 này thắng 409 người chưa đăng ký (`owner_as_group_admin_invites_unregistered_into_pair`).
7. `_require_registered_person(request.person_id)` (`service.py:1373-1387`) → 409 `person_not_registered` (`owner_invites_unregistered`, `owner_invites_uppercase_unregistered`). Chỉ kiểm dòng `people` tồn tại: tài khoản đã xoá (dòng đã ẩn danh) vẫn được mời (`owner_invites_deleted_person` → 201).
8. `add_member` vi phạm `uq_memberships_open_per_person` → 409 `membership_already_open` (`service.py:1637-1643`): đang được mời (`owner_invites_mate_again`), đang ACTIVE (`owner_invites_mate_while_active`), tự mời mình (`owner_invites_self`).

## Đầu vào

- Path `context_id` (UUID lax).
- Header: `Idempotency-Key` tuỳ chọn.
- Body `MembershipInviteRequest` (`services/api/app/api/schemas.py:1108-1109`), `extra="forbid"`: `person_id: UUID` bắt buộc, lax (chữ hoa được chấp nhận). Thiếu → `missing`, null → `uuid_type`, số → `uuid_type` (`person_id_number`), `role` → `extra_forbidden` (`extra_field_role`), mảng → `model_attributes_type`.

## Đầu ra

- **201** `MembershipResponse` (`schemas.py:1116-1139`), thứ tự khoá `id`, `context_id`, `person_id`, `display_name`, `state`, `role`, `invited_by_id`, `joined_at`, `left_at`, `created_at`.
  - `state: "invited"`, `role: "member"`, `invited_by_id`: actor, `joined_at: null`, `left_at: null`.
  - `display_name` đọc từ `people` của người được mời (`repository.py:2240-2264`, `:2529-2548`); fallback `str(person_id)` không tới được (FK + `NOT NULL`).
  - `created_at`: `now()` của Postgres (server default, `services/api/app/db/models.py:1207-1209`).
- Replay: 201 + body đã lưu + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `PUT` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `memberships (context_id, person_id, state = invited, role = member, origin = named, invited_by_id)` trong savepoint (`services/api/app/api/repository.py:2672-2701`). `IntegrityError` trên `uq_memberships_open_per_person` (partial unique `left_at IS NULL`, `models.py:1137-1143`) → 409, savepoint rollback; mọi `IntegrityError` khác ném tiếp (500).
- SELECT trước quyết định quyền: `memberships` (`is_member`), `memberships.role` (`membership_role`) — cả hai chạy kể cả với người lạ.
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 409 nhả key (`idem_refusal_not_stored` → `register_late` → `idem_same_key_after_refusal` 201). Fingerprint gồm body và path (`idem_reuse_different_body`, `idem_reuse_other_context` → 422).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:1621-1635`, `:504` |
| 409 | `not_a_group` | `Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.` | `service.py:1487-1491` |
| 409 | `person_not_registered` | `Register this person with PUT /people/{person_id} first` | `service.py:1383-1387` |
| 409 | `membership_already_open` | `Membership invitation conflicted` | `service.py:1641-1643`; `repository.py:2692-2700` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

`code` của 409 conflict là `RepositoryConflict.code.lower()` (`MEMBERSHIP_ALREADY_OPEN` → `membership_already_open`). 422 framework: `uuid_parsing` (path và body), `uuid_type`, `missing`, `extra_forbidden`, `model_attributes_type`, `json_invalid`.

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:62-74`
- Service: `services/api/app/api/service.py:1615-1644` (`invite_context_member`), `:1359-1371` (`_group_admin_role`), `:1373-1387`, `:1477-1491`
- Domain: `services/api/app/domain/permissions.py:283-286`, `:654-678`
- Repository: `services/api/app/api/repository.py:2672-2701`, `:2769-2782`, `:2808-2821`, `:2525-2548`
- Model: `services/api/app/db/models.py:1120-1209`

## Test đang phủ

- `services/api/tests/postgres/test_person_identity_postgres.py::test_inviting_somebody_who_was_never_named_is_refused_not_a_crash` (141)
- `services/api/tests/postgres/test_membership_role_postgres.py`: `test_an_invited_member_joins_as_a_plain_member` (90), `test_an_admin_of_one_group_is_not_an_admin_of_another` (196), `test_leaving_and_rejoining_starts_from_a_plain_membership` (222)
- `services/api/tests/postgres/test_member_display_name_postgres.py::test_an_invited_member_is_named_in_the_answer_that_invites_them` (160)
- `services/api/tests/api/test_direct_messages.py::test_roster_doors_are_closed_on_a_pair_but_the_theme_still_changes` (169)

## Kịch bản parity

`parity/scenarios/w3/contexts/POST-contexts-context_id-members.yaml`, id `w3/contexts/post-contexts-context_id-members` (57 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `anonymous_malformed_json`, `anonymous_empty_idempotency_key`, `anonymous_invalid_body`, `stranger_non_uuid_context`, `stranger_unknown_context_bad_body`, `stranger_non_uuid_and_bad_body`.
- Role trước membership: `stranger_unknown_context` (`role_not_permitted`), `stranger_as_group_admin_unknown_context`, `stranger_as_group_admin_real_context` (`is_group_member`), `mate_invites_invitee`, `mate_as_group_admin_invites_invitee` (201 dev), `invitee_as_group_admin_invites_leaver`, `owner_roles_empty_invites_leaver` (201), `leaver_as_group_admin_invites_stranger`.
- 409: `owner_invites_unregistered`, `owner_invites_uppercase_unregistered`, `owner_invites_mate_again`, `owner_invites_mate_while_active`, `owner_invites_self`.
- Đường vui: `owner_invites_mate`, `owner_reinvites_leaver` (dòng mới), `owner_reads_members`, `owner_invites_deleted_person` (sau `ghost_deletes_account`).
- Validate: `person_id_missing`, `person_id_null`, `person_id_number`, `extra_field_role`, `top_level_array`.
- Pair: `owner_asks_mate_to_be_friends`, `mate_accepts_friendship`, `owner_opens_pair_with_mate`, `owner_invites_into_pair` (403), `owner_as_group_admin_invites_into_pair`, `owner_as_group_admin_invites_unregistered_into_pair` (409 `not_a_group`).
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_reuse_other_context`, `idem_refusal_not_stored` + `register_late` + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects`, `put_not_allowed`.

`prod` (`w3/contexts/prod-auth`): `owner_invites_mate` (admin theo roster), `mate_invites_stranger_with_group_admin_header` (prod bỏ qua header → 403 `role_not_permitted`).

## Chưa phủ / lưu ý cho bản Go

- Trong prod, `group_admin` chỉ đến từ roster (`repository.py:3629` không cấp nó); nhánh "thành viên thường tự khai `group_admin`" chỉ có ở dev. Bản Go ở dev phải hợp role header với role suy từ roster rồi mới kiểm, đúng thứ tự role → predicate.
- `_group_admin_role` và `is_member` là hai SELECT chạy trước khi quyết định, kể cả khi kết quả là 403; không thấy được qua harness nhưng là một phần của hình dạng truy vấn.
- 409 conflict là do index trả lời (không kiểm trước); hai lời mời đồng thời cho cùng người chưa phủ.
- Mời được một tài khoản đã xoá (dòng `people` ẩn danh vẫn còn): parity giữ hành vi này, xem báo cáo.
- 409 in-flight chưa phủ.
