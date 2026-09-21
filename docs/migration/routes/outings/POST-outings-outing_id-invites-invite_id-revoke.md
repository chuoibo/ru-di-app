# POST /outings/{outing_id}/invites/{invite_id}/revoke

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Rút một lời mời lại. Từ lúc `revoked_at` được đặt, cả cửa link lẫn cửa `/sessions` từ chối bí mật của hàng đó. Rút một lời mời **đã được dùng** là 409: thu hồi không viết lại lịch sử.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:171-182`, `services/api/app/api/service.py:4724-4765`):

1. Middleware idempotency: khoá rỗng → 422 trước 401.
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 trước 422 của path.
4. Validation **hai** path param; sai cả hai thì `detail` có hai mục (`owner_both_paths_not_uuid`).
5. **Tồn tại đứng trước quyền** (`service.py:4730-4733`): đọc `get_outing(outing_id)` và `get_outing_invite(invite_id)`, rồi `outing is None or invite is None or invite.outing_id != outing_id` → 404 `invite_not_found` `Invite link is not valid`. Ba tình huống, một câu: chuyến lạ, lời mời lạ, lời mời của chuyến khác (`owner_unknown_everything`, `owner_revokes_an_unknown_invite`, `owner_revokes_a_real_invite_on_an_unknown_trip`, `owner_revokes_an_invite_of_another_trip`). Người lạ cũng nhận đúng 404 đó khi id không khớp (`stranger_revokes_an_unknown_invite`).
6. `_require_permission("revoke_outing_invite", …, {"is_group_member": is_member(outing.context_id, actor.id)})` (`service.py:4735-4739`; `permissions.py:258-261`) → 403 cho người lạ trên một lời mời có thật (`stranger_revokes_a_real_invite`), cho **chính người được mời** (`friend_revokes_their_own_invitation`), và cho vai trò sai (`owner_revokes_as_guest_role`). Bất kỳ thành viên `active` nào cũng rút được, không chỉ người đã mời (`mate_revokes_the_named_invitation`).
7. `invite.accepted_at is not None` → 409 `invite_already_accepted` `Invite link was already used` (`service.py:4740-4745`; `owner_revokes_the_spent_link`).
8. `repository.revoke_outing_invite` (`repository.py:3439-3459`) dưới `FOR UPDATE`: `OUTING_INVITE_NOT_FOUND` → 404, `OUTING_INVITE_ALREADY_ACCEPTED` → 409, và **`revoked_at` đã có thì trả hàng nguyên trạng, không ghi gì** (`owner_revokes_the_link_again`, `mate_revokes_the_link_again`).

## Đầu vào

Path `outing_id`, `invite_id`: UUID. Không query, **không thân**.

## Đầu ra

**200** `OutingInviteResponse` với `revoked_at` đã đặt và **`invite_token: null`, `invite_path: null`** — route gọi `_wire_outing_invite(revoked, None)` (`service.py:4765`). Bí mật không bao giờ quay lại wire sau lần mint.

## Tác dụng phụ

`UPDATE outing_invites SET revoked_at = now` cho hàng đó, dưới khoá hàng — **và chỉ khi `revoked_at` còn NULL**. Lần rút thứ hai đọc rồi trả về, không ra lệnh ghi nào. Không đụng `memberships`: người đã đổi link trước khi bị rút vẫn giữ membership `invited`. `idempotency_keys` khi có header và 2xx.

## Idempotency

Tự nhiên idempotent (lần hai vẫn 200, cùng thân). Khoá header cũng lưu và phát lại (`owner_revokes_with_key`, `owner_replays_key`, `owner_same_key_other_invite` → 422 reuse).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","outing_id"]` và/hoặc `["path","invite_id"]` | `main.py:319-351` |
| 404 | `invite_not_found` | `Invite link is not valid` | `service.py:4732-4733`, `:4759-4763` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 409 | `invite_already_accepted` | `Invite link was already used` | `service.py:4741-4745`, `:4752-4757` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:171-182`
- Service: `services/api/app/api/service.py:4724-4765`, `:966-990`
- Repository: `services/api/app/api/repository.py:3401-3403`, `:3439-3459`
- Quyền: `services/api/app/domain/permissions.py:258-261`

## Test đang phủ

- `services/api/tests/postgres/test_outing_invite_lifetime_postgres.py`: `test_a_revoked_link_stops_working_immediately` (403), `test_only_someone_inside_the_group_can_take_a_link_back` (448)
- **Hai ca, một tệp.** Không có ca nào cho 409 `invite_already_accepted`, cho lời mời của chuyến khác, hay cho lần rút thứ hai.

## Kịch bản parity

`parity/scenarios/w7/outings/POST-outings-outing_id-invites-invite_id-revoke.yaml`, id `w7/outings/post-outings-outing_id-invites-invite_id-revoke` (44 bước, `dev`):

- Cửa: `anonymous_revokes`, `anonymous_non_uuid_outing`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `owner_both_paths_not_uuid`, `owner_invite_path_not_uuid`, `owner_unknown_everything`.
- Tồn tại trước quyền: `owner_revokes_an_unknown_invite`, `owner_revokes_a_real_invite_on_an_unknown_trip`, `owner_revokes_an_invite_of_another_trip`, `stranger_revokes_an_unknown_invite` (404), rồi `stranger_revokes_a_real_invite`, `friend_revokes_their_own_invitation`, `owner_revokes_as_guest_role` (403).
- Rút: `owner_revokes_the_link`, `stranger_opens_the_revoked_link` (404), `owner_revokes_the_link_again`, `mate_revokes_the_link_again` (200, không ghi gì).
- Đã dùng: `stranger_opens_the_second_link`, `owner_revokes_the_spent_link` (409).
- Đích danh: `mate_revokes_the_named_invitation`, `owner_names_the_same_friend_again` (rút rồi **vẫn** 409 `invite_already_exists` — hàng còn đó).
- Khoá: `owner_mints_a_link_for_the_key_case`, `owner_revokes_with_key`, `owner_replays_key`, `owner_same_key_other_invite`.
- Rời nhóm: `mate_leaves_group`, `mate_revokes_after_leaving`, `owner_revokes_the_other_trips_link`.
- Framework: `trailing_slash`, `get_not_allowed`.

`prod-auth.yaml`: `stranger_revokes_their_own_invitation` (403), `owner_revokes_the_named_invitation`, `owner_revokes_the_spent_link` (409).

Corpus sinh: `generated/w7-422/post-outings-outing_id-invites-invite_id-revoke.yaml` (32 bước).

## Chưa phủ / lưu ý cho bản Go

- Hai nhánh `RepositoryConflict` (`OUTING_INVITE_NOT_FOUND`, `OUTING_INVITE_ALREADY_ACCEPTED`) chỉ tới được khi có đua giữa lần đọc và lần khoá hàng; bộ kịch bản không dựng được.
- Không có tệp `concurrency` cho route này: hai lần rút cùng lúc đều kết thúc 200 với cùng một thân, nên bước đồng thời không chứng minh thêm gì so với hai bước tuần tự.
- Thứ tự **404 trước 403** là hợp đồng: đừng đảo thành «403 cho mọi thứ» trong bản Go.
- Rút **không** xoá hàng và không gỡ chỉ số một phần, nên sau khi rút vẫn không mời lại đích danh được — đường quay lại là `/rotate`.

## Lỗi Python (chỉ báo, không sửa)

- Rút một lời mời đích danh rồi mời lại người đó vẫn là 409 `invite_already_exists`, còn xoay khoá hàng đã rút thì lại là 404 `invite_not_found`: cùng một hàng, hai route nói hai kiểu về nó.
