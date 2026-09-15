# DELETE /contexts/{context_id}/members/{person_id}

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Rời một nhóm. Chỉ chính người đó rời được: không có đường nào để admin loại người khác. Dòng `memberships` được đóng (`state = left`, `left_at`), không bị xoá; sổ tiền vẫn giữ người đã rời.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (DELETE là write method, `services/api/app/api/idempotency.py:76`, `:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_both`).
3. Hai path `context_id`, `person_id: UUID` → một mảng 422, `context_id` trước (`stranger_non_uuid_both`, `stranger_non_uuid_person`).
4. `_require_permission("leave_context", {"is_group_member": …, "is_self": actor.id == person_id})` (`services/api/app/api/service.py:1691-1698`; `services/api/app/domain/permissions.py:307-310`). Thứ tự `role` → `is_group_member` → `is_self` (`permissions.py:673-677`):
   - người lạ, nhóm không tồn tại hay có thật → 403 `is_group_member` (`stranger_unknown_context_self`, `stranger_real_context_self`, `stranger_removes_mate`);
   - role rỗng → 403 `role_not_permitted` (`stranger_roles_empty_unknown_context`, `mate_roles_empty_leaves`); chỉ `group_admin` là đủ (`mate_roles_group_admin_only_leaves` → 204);
   - admin xoá người khác → 403 `is_self` (`owner_removes_mate`); thành viên xoá một id bất kỳ → 403 `is_self` (`mate_removes_unknown_person`);
   - người được mời chưa nhận → 403 `is_group_member` (`invitee_declines_by_leaving`): không từ chối lời mời được qua route này;
   - đã rời → 403 (`mate_leaves_again`); tài khoản đã xoá → 403 (`ghost_leaves_after_deletion`).
5. `_require_group_kind` (`service.py:1477-1491`): pair → 409 `not_a_group` (`second_leaves_pair`); người kia của pair xoá mình → 403 `is_self` trước (`mate_removes_second_from_pair`).
6. `leave_context` trả None → 404 `membership_not_found` (`service.py:1700-1704`): không tới được tuần tự, vì bước 4 đã chứng minh có dòng ACTIVE.

## Đầu vào

- Path `context_id`, `person_id` (UUID lax).
- Không body: body bị bỏ qua (`mate_leaves_with_ignored_body` với `{"person_id": <owner>}` → 204 cho chính mate).
- Header: `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **204**, thân rỗng, không `content-type` (`services/api/app/api/routes/contexts.py:101-102`).
- Replay idempotency: 204 + `idempotency-replayed: true` + `content-length: 0` do middleware. Qua `core`, `content-length` biến mất: đó là ngoại lệ `RESPONSE-204-CONTENT-LENGTH` đã duyệt (ADR-0029 §2.4), harness đếm là `accepted`, không phải khác biệt (`idem_replay`).
- Framework: 307 cho `/` cuối (`location: http://<Host>/contexts/<id>/members/<person_id>`); `GET` → 405 `allow: DELETE`.

## Tác dụng phụ

- `SELECT memberships … FOR UPDATE` (ACTIVE, `left_at IS NULL`), rồi UPDATE `state = left`, `left_at = _now()` Python (`services/api/app/api/repository.py:2727-2747`). CHECK `left_state_matches_timestamp` buộc hai cột đi cùng nhau (`services/api/app/db/models.py:1149-1157`).
- Không xoá gì: phân bổ và nghĩa vụ của người rời vẫn nằm trong sổ (`GET /contexts/{id}/balances` vẫn tính họ).
- Người rời cuối cùng có vai trò admin vẫn rời được (`owner_leaves` → 204); sau đó không ai trong nhóm có `group_admin` theo roster, và lời mời mới bị 403 `role_not_permitted` (`second_invites_after_admin_left`).
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 409 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`). Fingerprint gồm path (`idem_reuse_other_path` → 422).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` / `is_self` | `service.py:1691-1698`, `:504` |
| 409 | `not_a_group` | `Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.` | `service.py:1487-1491` |
| 404 | `membership_not_found` | `Active membership does not exist` (không tới được) | `service.py:1702-1704` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

Lưu ý hai chuỗi 404 khác nhau trong cùng nhóm route: accept dùng `Membership does not exist`, leave dùng `Active membership does not exist`.

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:90-102`
- Service: `services/api/app/api/service.py:1688-1704` (`leave_context`), `:1477-1491`, `:475-504`
- Domain: `services/api/app/domain/permissions.py:307-310`, `:654-678`
- Repository: `services/api/app/api/repository.py:2727-2747`, `:2769-2782`
- Model: `services/api/app/db/models.py:1120-1209`

## Test đang phủ

- `services/api/tests/postgres/test_membership_role_postgres.py::test_leaving_and_rejoining_starts_from_a_plain_membership` (222)
- `services/api/tests/postgres/test_member_display_name_postgres.py::test_someone_who_left_stops_reading_the_roster` (259)
- `services/api/tests/postgres/test_group_memories_postgres.py::test_a_person_who_left_the_group_stops_seeing_its_memories` (222)
- `services/api/tests/api/test_person_contexts.py::test_lists_active_and_invited_groups_but_not_the_one_left` (57)
- `services/api/tests/api/test_direct_messages.py::test_roster_doors_are_closed_on_a_pair_but_the_theme_still_changes` (169)

## Kịch bản parity

`parity/scenarios/w3/contexts/DELETE-contexts-context_id-members-person_id.yaml`, id `w3/contexts/delete-contexts-context_id-members-person_id` (54 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown`, `anonymous_non_uuid_both`, `anonymous_empty_idempotency_key`, `stranger_non_uuid_both` (hai lỗi gom một), `stranger_non_uuid_person`.
- 403: `stranger_unknown_context_self`, `stranger_roles_empty_unknown_context`, `stranger_real_context_self`, `stranger_removes_mate`, `owner_removes_mate` (`is_self`), `invitee_declines_by_leaving`, `mate_roles_empty_leaves`, `mate_removes_unknown_person`, `mate_leaves_again`, `ghost_leaves_after_deletion` (sau `ghost_deletes_account`).
- Đường vui: `mate_leaves_with_ignored_body`, `mate_reads_members_after_leaving` (403), `owner_reads_members_after_mate_left`, `owner_reinvites_mate` + `mate_accepts_again`, `mate_roles_group_admin_only_leaves`, `owner_leaves` (admin cuối), `second_invites_after_admin_left` (403), `second_reads_members`.
- Pair: `second_asks_mate_to_be_friends`, `mate_accepts_friendship`, `second_opens_pair_with_mate`, `second_leaves_pair` (409), `mate_removes_second_from_pair` (403).
- Idempotency: `second_as_group_admin_invites_mate` + `mate_accepts_third_time`, `idem_first`, `idem_replay` (204 replay), `idem_reuse_other_path`, `idem_refusal_not_stored` (409) + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).

`prod` (`w3/contexts/prod-auth`): `mate_leaves` (204), `mate_reads_after_leaving` (403).

## Chưa phủ / lưu ý cho bản Go

- `left_at` là đồng hồ Python; CHECK trong DB buộc `state` và `left_at` đổi cùng một UPDATE.
- Bản Go qua `net/http` không gửi `Content-Length` cho 204; ngoại lệ đã duyệt chỉ chấp nhận đúng cặp (reference `0`, candidate không có header).
- 404 `Active membership does not exist` chỉ tới được khi hai lần rời chạy đồng thời; chưa phủ.
- Không có kiểm tra "còn admin nào không" trước khi admin cuối rời: parity giữ nguyên, xem báo cáo lỗi nghi vấn.
