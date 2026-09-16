# POST /outings/{outing_id}/invites/{invite_id}/rotate

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đặt một bí mật mới lên một lời mời **đích danh** — đường quay lại cho thành viên mất máy. Không phải lời mời thứ hai: chỉ số một phần `uq_outing_invites_person` chỉ cho một hàng đích danh mỗi người mỗi chuyến, nên cách vào lại là thay bí mật trên hàng đã nêu tên họ. Người được nêu **không đổi**, và đó là thứ giữ cho route này không thành cách trao tài khoản của người khác cho bên thứ ba.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:185-202`, `services/api/app/api/service.py:4676-4722`):

1. Middleware idempotency: khoá rỗng → 422 trước 401.
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 trước 422 của path.
4. Validation hai path param.
5. **Tồn tại đứng trước quyền** (`service.py:4692-4695`): `outing is None or invite is None or invite.outing_id != outing_id` → 404 `invite_not_found` `Invite link is not valid`.
6. `_require_permission("invite_to_outing", …)` — **cùng quyền với cửa mint**, không phải `revoke_outing_invite` (`service.py:4697-4701`; `permissions.py:240-243`) → 403 (`stranger_rotates_a_real_invite`, `friend_rotates_their_own_invitation`, `owner_rotates_as_guest_role`, `mate_rotates_after_leaving`).
7. `invite.invited_person_id is None` → 409 `invite_not_named` `Only a named invitation can be rotated` (`service.py:4702-4707`; `owner_rotates_the_link`).
8. `repository.rotate_outing_invite_digest` (`repository.py:3461-3493`) dưới `FOR UPDATE`: `OUTING_INVITE_NOT_FOUND`, `OUTING_INVITE_NOT_NAMED`, `OUTING_INVITE_NOT_REDEEMABLE` (hàng đã rút) — service biến **mọi** `RepositoryConflict` thành **404** (`service.py:4718-4721`). Nên xoay một lời mời **đã bị rút** trả 404 `Invite link is not valid`, không phải 409 (`owner_rotates_the_revoked_invitation`, `mate_rotates_their_own_revoked_invitation`).
9. `accepted_at` **cố ý không bị xoá** (docstring `repository.py:3468-3475`): xoay thay bí mật, không tua lại lịch sử, và không trao cho cửa link một lần đổi thứ hai của hàng đã tiêu.

## Đầu vào

Path `outing_id`, `invite_id`: UUID. Không query, **không thân**.

## Đầu ra

**200** `OutingInviteResponse` với `invite_token` **mới** và `expires_at` = `now + 7 ngày` mới. `invite_path` vẫn `null` vì `source != "link"`. `id`, `outing_id`, `source`, `invited_person_id`, `invited_by_id`, `created_at`, `revoked_at` giữ nguyên.

## Tác dụng phụ

`UPDATE outing_invites SET token_digest = …, expires_at = …` cho hàng đó, dưới khoá hàng. Bí mật cũ **không thể trình lại** ở bất kỳ cửa nào. Không đụng bảng nào khác. `idempotency_keys` khi có header và 2xx.

## Idempotency

Không tự nhiên idempotent: mỗi lượt xoay là một bí mật mới (`owner_rotates_the_named_invitation`, `owner_rotates_the_named_invitation_again`, `mate_rotates_the_named_invitation` — ba token khác nhau). Khoá header lưu 200 và phát lại **cùng token đã trả**, nghĩa là một bí mật mới được trao lại mỗi lần bấm khoá (`owner_rotates_with_key`, `owner_replays_key`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 422 | (validation) | `uuid_parsing` trên một hoặc cả hai path param | `main.py:319-351` |
| 404 | `invite_not_found` | `Invite link is not valid` | `service.py:4694-4695`, `:4718-4721` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 409 | `invite_not_named` | `Only a named invitation can be rotated` | `service.py:4703-4707` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:185-202`
- Service: `services/api/app/api/service.py:4676-4722`, `:966-990`, `:385`, `:411-412`
- Repository: `services/api/app/api/repository.py:3401-3403`, `:3461-3493`
- Quyền: `services/api/app/domain/permissions.py:240-243`

## Test đang phủ

- `services/api/tests/postgres/test_session_bootstrap_postgres.py`: `test_rotation_kills_the_old_secret_and_signs_the_same_person_back_in` (334)
- **Một ca, một tệp.** Không có ca nào cho 403, 404, 409 `invite_not_named`, hay hàng đã rút. Đây là route mỏng nhất của sóng về mặt test Python.

## Kịch bản parity

`parity/scenarios/w7/outings/POST-outings-outing_id-invites-invite_id-rotate.yaml`, id `w7/outings/post-outings-outing_id-invites-invite_id-rotate` (44 bước, `dev`):

- Cửa: `anonymous_rotates`, `anonymous_non_uuid_outing`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `owner_invite_path_not_uuid`, `owner_unknown_everything`.
- Tồn tại trước quyền: `owner_rotates_an_unknown_invite`, `owner_rotates_a_real_invite_on_an_unknown_trip`, `owner_rotates_an_invite_of_another_trip` (404); rồi `stranger_rotates_a_real_invite`, `friend_rotates_their_own_invitation`, `owner_rotates_as_guest_role` (403).
- Loại: `owner_rotates_the_link` (409 `invite_not_named`).
- Xoay: `owner_rotates_the_named_invitation`, `owner_rotates_the_named_invitation_again`, `mate_rotates_the_named_invitation` — mỗi lượt một token mới, *bind*, không viết ra.
- Cửa link vẫn từ chối bí mật đích danh: `friend_opens_the_first_secret_at_the_link_door`, `friend_opens_the_newest_secret_at_the_link_door`, `friend_opens_the_replayed_secret_at_the_link_door`.
- Vì sao xoay tồn tại: `owner_names_the_same_friend_again` (409 `invite_already_exists`).
- Hàng đã rút: `owner_revokes_the_mates_invitation`, `owner_rotates_the_revoked_invitation`, `mate_rotates_their_own_revoked_invitation` (cả hai 404, **không** 409).
- Khoá: `owner_rotates_with_key`, `owner_replays_key`, `owner_same_key_other_invite`.
- Rời nhóm: `mate_leaves_group`, `mate_rotates_after_leaving`, `owner_rotates_the_other_trips_invitation`.
- Framework: `trailing_slash`, `get_not_allowed`.

`prod-auth.yaml`: `anonymous_rotates`, `stranger_rotates_their_own_invitation` (403), `owner_rotates_the_named_invitation`.

Corpus sinh: `generated/w7-422/post-outings-outing_id-invites-invite_id-rotate.yaml` (32 bước).

## Chưa phủ / lưu ý cho bản Go

- **Không phủ** việc bí mật mới thật sự đổi được phiên ở `POST /sessions`, và bí mật cũ thì không: đó là route của sóng khác. Ở đây chỉ chứng minh được token **đổi giá trị** và cửa link vẫn từ chối cả hai.
- Không phủ xoay một lời mời đích danh **đã được tiêu** (`accepted_at` khác NULL): đường duy nhất đặt `accepted_at` cho hàng đích danh là `/sessions`.
- Không có tệp `concurrency`: hai lượt xoay cùng lúc đều thành công với hai token khác nhau, và bước đồng thời không bind được nên không đối chiếu được token.
- `OUTING_INVITE_NOT_NAMED` của repository **không bao giờ** tới được: service đã chặn ở bước 7. Giữ nhánh trong bản Go, nhưng không có kịch bản nào cho nó.

## Lỗi Python (chỉ báo, không sửa)

- `rotate_outing_invite_secret` biến **mọi** `RepositoryConflict` thành 404, nên một lượt xoay hỏng vì lý do khác — hàng đã rút — được báo là «lời mời không tồn tại». Người dùng mất máy sẽ đọc thành «link của tôi biến mất».
- Bí mật mới được trả lại nguyên vẹn cho mỗi lần phát lại theo `Idempotency-Key`.
