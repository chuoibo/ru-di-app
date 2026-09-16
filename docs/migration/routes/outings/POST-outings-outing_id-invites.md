# POST /outings/{outing_id}/invites

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đúc một lời mời. Ba `source`, hai cửa khác nhau:

- `link` — không nêu tên ai, là một **bí mật chuyển tay được**, đổi ở `POST /outing-invites/{token}/accept` lấy một membership `invited`;
- `group` / `friend` — nêu đích danh một người, và bí mật của nó là thứ người đó đổi ở `POST /sessions` lấy phiên đầu tiên (ADR-0014). Cửa link **từ chối** bí mật đích danh.

Server chỉ giữ **digest SHA-256**; token thô được trả về **đúng một lần**, ở đây. Kịch bản vì thế *bind* token, không bao giờ viết ra.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:156-168`, `services/api/app/api/service.py:3586-3654`):

1. Middleware idempotency: khoá rỗng → 422 trước 401 (`anonymous_empty_idempotency_key`).
2. Router: đuôi `/` → 307; `GET` → 405 (**không có route đọc danh sách lời mời**).
3. `get_actor` → 401 trước 422 của path (`anonymous_non_uuid_outing`).
4. Validation path + thân `OutingInviteCreateRequest`.
5. `repository.get_outing` → None → 404 `outing_not_found` **`Outing does not exist`** (`service.py:3593-3594`).
6. `_require_permission("invite_to_outing", …, {"is_group_member": …})` (`service.py:3595-3599`; `permissions.py:240-243`) → 403 (`stranger_invites`, `stranger_invites_themselves`, `owner_invites_as_guest_role`, `mate_mints_after_leaving`).
7. `_require_group_kind(outing.context_id)` (`service.py:3600`, `:1477-1491`) → context là pair → 409 `not_a_group` `Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.` (`owner_mints_a_link_on_the_pair_trip`, `owner_names_a_person_on_the_pair_trip`). Đứng **sau** quyền, nên người lạ vẫn nhận 403 trên một pair (`stranger_mints_on_the_pair_trip`).
8. `raw_token = secrets.token_urlsafe(32)`; `digest = sha256(token)` (`service.py:3606-3607`, `:411-412`). **Cả hai loại** đều mang bí mật.
9. Chỉ khi `source != "link"`:
   - `_require_registered_person(person_id)` (`service.py:1373-1387`) → 409 `person_not_registered` `Register this person with PUT /people/{person_id} first` — trước khi khoá ngoại `fk_outing_invites_person` kịp biến thành 500 (`owner_names_an_unregistered_person`, `owner_group_invites_an_unregistered_person`);
   - chỉ `source == "group"`: `_require_participants_are_members(context_id, [person_id])` (`service.py:6336-6371`) → 422 `participant_not_in_context` `Not members of this group: <id>` (`owner_group_invites_someone_off_the_roster`, `owner_group_invites_the_departed_member`). `source == "friend"` **cố ý không** kiểm danh sách: mời bạn bè là mời người ngoài nhóm;
   - `find_outing_invite_for_person(outing_id, person_id)` khác None → 409 `invite_already_exists` `Person is already invited to this outing` (`owner_invites_the_same_friend_again`, `mate_invites_the_same_friend_by_group`). Đây là kiểm «lịch sự» chạy đua với insert; bảo đảm thật là chỉ số một phần `uq_outing_invites_person`.

## Đầu vào

- Path `outing_id`: UUID.
- Thân `OutingInviteCreateRequest` (`schemas.py:717-727`), `extra="forbid"`: `source` ∈ {`group`,`friend`,`link`}; `person_id` UUID hoặc null; `model_validator(mode="after")`: `link` **không được** nêu người, `group`/`friend` **bắt buộc** nêu (`owner_link_names_a_person`, `owner_group_names_nobody`, `owner_friend_names_nobody`).

## Đầu ra

**201** `OutingInviteResponse` (`schemas.py:730-742`, dựng ở `service.py:966-990`), thứ tự khoá:

`id`, `outing_id`, `source`, `invited_person_id`, `invited_by_id`, `created_at`, `expires_at`, `revoked_at` (`null`), `invite_token`, `invite_path`.

- `invite_token`: token thô, `secrets.token_urlsafe(32)` → **43 ký tự base64url** (bộ chuẩn hoá parity đánh số là `<token43#n>`);
- `invite_path`: `f"/outing-invites/{raw_token}"` **chỉ khi** `source == "link"`; lời mời đích danh có token nhưng `invite_path: null`, vì nó được tiêu ở `/sessions` chứ không ở cửa link;
- `expires_at` = `now + OUTING_INVITE_TTL` = `now + 7 ngày` (`service.py:385`).

## Tác dụng phụ

Một hàng `outing_invites` (`repository.py:3364-3386`): `outing_id`, `source`, `invited_person_id` (null cho `link`), `invited_by_id` = actor, `token_digest` = 32 byte, `created_at` = `now`, `expires_at`. Cột `id` sinh phía Python (`default=uuid.uuid4`), `created_at` được gán nên **không** đọc lại server default. Ràng buộc: `link_carries_digest`, `link_names_nobody`, `acceptance_is_whole`, `expiry_after_creation`, và chỉ số một phần `uq_outing_invites_person` trên `(outing_id, invited_person_id) WHERE invited_person_id IS NOT NULL` (`db/models.py:1393-1474`).

## Idempotency

Không tự nhiên idempotent: hai lần mint `link` là hai hàng và hai bí mật. Khoá header lưu 201 và phát lại nguyên văn — nghĩa là **bí mật «chỉ trả một lần» được trả lại mỗi lần bấm khoá** (`crossreplay/POST-outings-outing_id-invites.yaml`: `python_mints_a_link` rồi `front_replays_python`). Bí mật của lần mint được *bind* **một lần**; thân của bản phát lại **không** được bind lần thứ hai, vì bộ chuẩn hoá từ chối gán hai tên cho một literal — và chính lời từ chối đó là bằng chứng hai bí mật là cùng một chuỗi. Trong bản ghi đã chuẩn hoá, thân phát lại in token là `<token:token_minted>`; nếu hai bên khác nhau nó sẽ in `<token43#n>` và khác biệt hiện ra ngay. 409 `invite_already_exists` nhả khoá.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | (validation) | `literal_error` cho `source`, `value_error` cho hai luật `person_id` | `main.py:319-351` |
| 404 | `outing_not_found` | `Outing does not exist` | `service.py:3593-3594` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 409 | `not_a_group` | `Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.` | `service.py:1487-1490` |
| 409 | `person_not_registered` | `Register this person with PUT /people/{person_id} first` | `service.py:1383-1386` |
| 422 | `participant_not_in_context` | `Not members of this group: <id>[, <id>…]` | `service.py:6362-6371` |
| 409 | `invite_already_exists` | `Person is already invited to this outing` | `service.py:3636-3640` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:156-168`
- Service: `services/api/app/api/service.py:3586-3654`, `:966-990` (`_wire_outing_invite`), `:1373-1387`, `:1477-1491`, `:6336-6371`, `:385`, `:411-412`
- Repository: `services/api/app/api/repository.py:3364-3386`, `:3388-3399`, `:2392-2404`
- Schema: `services/api/app/api/schemas.py:717-742`
- Quyền: `services/api/app/domain/permissions.py:240-243`
- Model DB: `services/api/app/db/models.py:1393-1474`

## Test đang phủ

- `services/api/tests/postgres/test_outings_postgres.py`: `test_a_member_invites_from_the_group_and_from_their_friends` (605), `test_an_invite_link_stores_only_a_digest_and_shows_the_token_once` (656), `test_redeeming_an_invite_link_grants_an_invited_membership_not_an_active_one` (710), `test_a_forged_or_reused_invite_link_is_refused` (768), `test_headcount_is_a_plan_and_not_a_door_policy` (806), `test_a_stranger_cannot_invite_anyone_to_a_trip_they_are_not_in` (835)
- `services/api/tests/postgres/test_outing_invite_source_must_match_roster.py`: cả 5 ca (125, 157, 184, 207, 226)
- `services/api/tests/postgres/test_outing_invite_lifetime_postgres.py`: 7 ca (184, 216, 266, 320, 370, 403, 448)
- `services/api/tests/postgres/test_session_bootstrap_postgres.py`: 11 ca qua `World.invite` (123), gồm `test_a_second_named_invitation_is_refused_which_is_why_rotation_exists` (321)
- `services/api/tests/postgres/test_outing_invite_escalation_postgres.py`: `test_a_forwarded_invite_link_cannot_promote_itself_to_active` (75)

## Kịch bản parity

`parity/scenarios/w7/outings/POST-outings-outing_id-invites.yaml`, id `w7/outings/post-outings-outing_id-invites` (53 bước, `dev`):

- Cửa: `anonymous_invites`, `anonymous_non_uuid_outing`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `owner_path_not_uuid`, `owner_unknown_outing`, `stranger_invites`, `stranger_invites_themselves`, `owner_invites_as_guest_role`.
- Thân: `owner_body_missing`, `owner_source_missing`, `owner_source_unknown`, `owner_link_names_a_person`, `owner_group_names_nobody`, `owner_friend_names_nobody`, `owner_person_id_not_uuid`, `owner_extra_field`.
- Người được nêu: `owner_names_an_unregistered_person`, `owner_group_invites_an_unregistered_person` (409 trước cả kiểm danh sách), `owner_group_invites_someone_off_the_roster` (422), `owner_group_invites_the_departed_member` (422), `owner_friend_invites_the_departed_member` (201 — `friend` không kiểm danh sách).
- Ghi: `owner_mints_a_link`, `owner_mints_a_second_link`, `owner_invites_a_friend_from_outside`, `mate_invites_a_member_by_group`.
- Một hàng đích danh cho một người: `owner_invites_the_same_friend_again`, `mate_invites_the_same_friend_by_group` (cả hai 409).
- Hai cửa: `friend_opens_the_named_secret_at_the_link_door` (404), `friend_opens_the_link` (200).
- Khoá: `owner_mints_with_key`, `owner_replays_key`, `owner_same_key_other_source`, `mate_same_key_own_scope`.
- Pair: `owner_mints_a_link_on_the_pair_trip` (409 `not_a_group`), `owner_names_a_person_on_the_pair_trip` (409 — **trước** kiểm người), `stranger_mints_on_the_pair_trip` (403).
- Framework: `trailing_slash`, `get_not_allowed`.

`crossreplay/POST-outings-outing_id-invites.yaml` (13 bước). `prod-auth.yaml`: `owner_mints_a_link`, `owner_names_the_stranger`, `stranger_mints_claiming_owner` (403).

Corpus sinh: route này **hoãn** (`OutingInviteCreateRequest` có `model_validator(mode="after")`).

## Chưa phủ / lưu ý cho bản Go

- **Không có kịch bản đồng thời cho route này.** Hình dạng đáng đo duy nhất — hai lời mời đích danh cho cùng một người cùng lúc — rơi vào `IntegrityError` mà `create_outing_invite` không bắt, nên kết quả là hỗn hợp `201/409/500` không xác định giữa hai stack; một tệp như thế sẽ đỏ vì đua chứ không vì lệch. Xem mục lỗi dưới.
- Không phủ `expires_at` đã qua: TTL là 7 ngày và kịch bản parity không có bước đồng hồ (không có cách nào tua thời gian qua HTTP).
- Không phủ đường `/sessions` đổi bí mật đích danh lấy phiên (route của sóng khác).
- `invite_path` chỉ có với `link`; `invite_token` có với **cả ba** loại. Đừng đơn giản hoá thành «đích danh thì không có token».
- Kiểm danh sách chỉ áp cho `source == "group"`. Đó là quyết định, không phải sót.

## Lỗi Python (chỉ báo, không sửa)

- `create_outing_invite` của repository **không bắt `IntegrityError`**: khi `uq_outing_invites_person` bắn (hai request đích danh chạy song song), route trả 500 chứ không 409. Kiểm «lịch sự» phía trước chỉ thu hẹp cửa sổ.
- Bí mật «chỉ trả về một lần» được trả lại đầy đủ cho mỗi lần phát lại theo `Idempotency-Key`, vì middleware lưu nguyên thân 201.
- 404 của route này (`Outing does not exist`) khác câu 404 của các route lời mời anh em (`Invite link is not valid`) và khác câu của cửa v2 (`Không tìm thấy chuyến đi.`).
