# POST /memberships/{membership_id}/accept

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Chuyển một dòng `memberships` từ `invited` sang `active`. Ai được bấm phụ thuộc vào nguồn gốc lời mời (`origin`): lời mời đích danh (`named`) do chính người được mời nhận; yêu cầu vào nhóm qua link (`link`, sinh bởi `POST /outing-invites/{token}/accept`) phải được một thành viên ACTIVE **khác** duyệt (`services/api/app/api/service.py:1646-1686`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_membership`); header role lạ → 422 `invalid_actor_roles` thắng lỗi path (`roles_unknown_non_uuid_membership`).
3. Path `membership_id: UUID` → 422 `uuid_parsing` (`stranger_non_uuid_membership`, `stranger_short_uuid`); chữ hoa đi tiếp (`stranger_uppercase_unknown_membership`).
4. `get_membership` (`services/api/app/api/repository.py:2703-2707`) không thấy → 404 `membership_not_found` **trước mọi kiểm tra quyền** (`service.py:1655-1657`): người lạ, kể cả role rỗng, nhận 404 (`stranger_unknown_membership`, `stranger_unknown_membership_roles_empty`).
5. Theo `origin`:
   - `link` → `_require_permission("approve_link_join_request", {is_group_member, is_not_self})` (`service.py:1659-1669`; `services/api/app/domain/permissions.py:303-306`). Người gửi yêu cầu tự nhận → 403 `is_group_member` (dòng của họ là INVITED, `joiner_accepts_own_link_request`); người lạ và người chỉ được mời cũng 403 `is_group_member` (`stranger_approves_link_request`, `invitee_approves_link_request`); role rỗng → `role_not_permitted` (`mate_approves_roles_empty`). `is_not_self` không tới được (index `uq_memberships_open_per_person` không cho một người vừa ACTIVE vừa INVITED trong cùng nhóm; ghi chú ở `permissions.py:291-302`).
   - còn lại (`named`) → `_require_permission("accept_context_membership", {"is_invitee": membership.person_id == actor.id})` (`service.py:1670-1675`; `permissions.py:287-290`). Người khác, kể cả admin, → 403 `is_invitee` (`stranger_accepts_mates_invite`, `owner_accepts_mates_invite`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` là đủ (`mate_roles_group_admin_only` → 200).
6. `_require_group_kind(membership.context_id)` (`service.py:1477-1491`): dòng thuộc pair → 409 `not_a_group` (`owner_accepts_pair_membership`); người khác trên dòng đó vẫn 403 `is_invitee` trước (`mate_accepts_owners_pair_membership`).
7. `accept_membership` (`repository.py:2709-2725`): `SELECT … FOR UPDATE` với `populate_existing`; `state` khác `invited` → 409 `membership_not_invited`: đã ACTIVE (`mate_accepts_again`, `owner_accepts_own_active_membership`, `owner_approves_again`), đã rời (`leaver_accepts_left_membership`), bị đóng khi xoá tài khoản (`ghost_accepts_after_deletion`). Trả None → 404 (`service.py:1684-1685`) chỉ khi dòng biến mất giữa hai lần đọc.

## Đầu vào

- Path `membership_id` (UUID lax).
- Không body: body bị bỏ qua, kể cả JSON hỏng (`stranger_malformed_body_ignored` → 404) hay `{"state":"left"}` (`leaver_accepts_with_ignored_body` → 200).
- Header: `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **200** (không phải 201) `MembershipResponse` (`services/api/app/api/schemas.py:1116-1139`), thứ tự khoá `id`, `context_id`, `person_id`, `display_name`, `state`, `role`, `invited_by_id`, `joined_at`, `left_at`, `created_at`.
  - `state: "active"`, `joined_at` = `_now()` của Python lúc duyệt (`service.py:1679`), `left_at: null`.
  - `role` và `invited_by_id` giữ nguyên dòng cũ; yêu cầu qua link có `invited_by_id` là người tạo link (`mate_approves_link_request`).
  - `display_name` là tên người **được nhận vào**, không phải người duyệt.
- Replay: 200 + body đã lưu + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `GET` → 405 `allow: POST`.

## Tác dụng phụ

- `SELECT memberships … FOR UPDATE`, rồi UPDATE `state = active`, `joined_at = now` (`repository.py:2709-2725`).
- SELECT trước đó: `memberships` (tra dòng), `memberships` (`is_member` cho nhánh link), `contexts` (kiểm pair), `people` (tên).
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 404 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`). Fingerprint gồm path (`idem_reuse_other_membership` → 422).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 404 | `membership_not_found` | `Membership does not exist` | `service.py:1657`, `:1685` |
| 403 | `permission_denied` | `role_not_permitted` / `is_invitee` / `is_group_member` | `service.py:1659-1675`, `:504` |
| 409 | `not_a_group` | `Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.` | `service.py:1487-1491` |
| 409 | `membership_not_invited` | `Membership acceptance conflicted` | `service.py:1680-1683`; `repository.py:2720-2721` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 path: `uuid_parsing` với `loc ["path","membership_id"]`.

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:77-87`
- Service: `services/api/app/api/service.py:1646-1686` (`accept_context_membership`), `:1477-1491`, `:475-504`
- Domain: `services/api/app/domain/permissions.py:287-306`, `:654-678`
- Repository: `services/api/app/api/repository.py:2703-2725`, `:2769-2782`, `:4464-4502` (`ensure_invited_membership`, nơi sinh dòng `origin = link`), `:4840-4852` (xoá tài khoản đóng mọi dòng mở)
- Model: `services/api/app/db/models.py:1120-1209`

## Test đang phủ

- `services/api/tests/postgres/test_membership_origin_postgres.py`: `test_an_active_member_can_approve_a_link_request_and_that_opens_the_group` (129), `test_the_requester_cannot_approve_themselves_whatever_roles_they_claim` (183), `test_a_named_invitee_still_accepts_their_own_invitation` (221)
- `services/api/tests/postgres/test_outings_postgres.py::test_redeeming_an_invite_link_grants_an_invited_membership_not_an_active_one` (710)
- `services/api/tests/postgres/test_membership_role_postgres.py::test_an_invited_member_joins_as_a_plain_member` (90)

## Kịch bản parity

`parity/scenarios/w3/contexts/POST-memberships-membership_id-accept.yaml`, id `w3/contexts/post-memberships-membership_id-accept` (58 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_membership`, `anonymous_non_uuid_membership`, `anonymous_empty_idempotency_key`, `roles_unknown_non_uuid_membership`, `stranger_non_uuid_membership`, `stranger_short_uuid`, `stranger_unknown_membership`, `stranger_uppercase_unknown_membership`, `stranger_unknown_membership_roles_empty` (404 trước role), `stranger_malformed_body_ignored`.
- Lời mời đích danh: `owner_invites_mate`, `stranger_accepts_mates_invite`, `owner_accepts_mates_invite` (403 `is_invitee`), `mate_roles_empty` (403), `mate_roles_group_admin_only` (200), `mate_accepts_again`, `owner_accepts_own_active_membership` (409, id lấy ở `owner_reads_own_membership`).
- Đã rời / đã xoá: `owner_invites_leaver`, `leaver_accepts_with_ignored_body`, `leaver_leaves`, `leaver_accepts_left_membership` (409); `owner_invites_ghost`, `ghost_deletes_account`, `ghost_accepts_after_deletion` (409).
- Link: `owner_plans_outing`, `owner_creates_link` (bind `token`), `joiner_redeems_link`, `joiner_accepts_own_link_request`, `stranger_approves_link_request`, `invitee_approves_link_request` (403 `is_group_member`), `mate_approves_roles_empty` (403), `mate_approves_link_request` (200), `owner_approves_again` (409), `joiner_reads_members`.
- Pair: `owner_asks_mate_to_be_friends`, `mate_accepts_friendship`, `owner_opens_pair_with_mate` (bind `membership_id` của owner), `owner_accepts_pair_membership` (409), `mate_accepts_owners_pair_membership` (403).
- Idempotency: `owner_invites_stranger`, `idem_first`, `idem_replay`, `idem_reuse_other_membership`, `owner_invites_invitee` + `idem_refusal_not_stored` (404) + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).

`prod` (`w3/contexts/prod-auth`): `anonymous_non_uuid_accept` (401), `stranger_accepts_mates_invite` (403), `mate_accepts` (200).

## Chưa phủ / lưu ý cho bản Go

- 404 đứng **trước** kiểm tra quyền: route cho biết một membership id có tồn tại hay không với bất kỳ ai có phiên. Parity giữ nguyên thứ tự này; đảo lại sẽ lệch `stranger_*`.
- `FOR UPDATE` với `populate_existing`: dòng được đọc lại sau khi khoá. Hai lần duyệt đồng thời (một 200, một 409) chưa phủ.
- `joined_at` là đồng hồ Python, `created_at` là `now()` của DB lúc mời.
- Không có đường từ chối lời mời: người được mời không tự rời được (xem card `DELETE /contexts/{id}/members/{person_id}`).
- Nhánh 404 thứ hai và `is_not_self` không tới được qua HTTP.
