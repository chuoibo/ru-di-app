# POST /outing-invites/{token}/accept

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đổi một **link chuyển tay được** lấy chỗ đứng trong nhóm. Đổi xong người đó là `invited`, **không** phải `active`: link tự nhận diện người cầm nó là *người xin vào*, không phải người duyệt, nên một thành viên `active` khác vẫn phải đồng ý trước khi dữ liệu nhóm hiện ra.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:204-214`, `services/api/app/api/service.py:3656-3729`):

1. Middleware idempotency: khoá rỗng → 422 trước 401 (`anonymous_empty_idempotency_key`).
2. Router: `token` là **`str` thuần** (không pattern, không độ dài), nên token rác là 404 *của route* chứ không 422; token rỗng đổi đường dẫn thành `/outing-invites//accept`, không khớp route nào → 404 của Starlette với thân khác hẳn (`holder_accepts_an_empty_token`). Đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 (`anonymous_accepts_junk`); 422 `invalid_actor_id`, `invalid_actor_roles`. **Phải có danh tính**: token nói *lời mời nào*, phiên nói *ai đang đổi*.
4. **Không có kiểm quyền nào.** Theo thiết kế — token chính là lời khai — và ghi ra đây để người đọc sau không tưởng là sót.
5. `get_outing_invite_by_digest(sha256(token))` → None → 404 `invite_not_found` `Invite link is not valid` (`service.py:3666-3667`).
6. `invite.invited_person_id is not None` → **404 cùng câu** (`service.py:3668-3676`): bí mật đích danh là chứng của cửa kia; tiêu nó ở đây sẽ tiêu mất hàng mà phiên của ai đó sắp lấy từ, và tiêu hộ bất kỳ người nào đang cầm token. Trả lời «không hợp lệ» chứ không «nhầm cửa», vì token không được phép khai nó tìm thấy gì.
7. `invite.accepted_at is not None` → 409 `invite_already_accepted` `Invite link was already used` (`service.py:3677-3682`).
8. `invite.revoked_at is not None or invite.expires_at <= now` → 404 cùng câu (`service.py:3684-3685`).
9. `get_outing(invite.outing_id)` → None → 404 cùng câu (`service.py:3687-3691`): giữ ranh giới năng lực ngay cả khi toàn vẹn tham chiếu vỡ.
10. `repository.accept_outing_invite` (`repository.py:3415-3437`) dưới `FOR UPDATE`: `OUTING_INVITE_ALREADY_ACCEPTED` → 409; `OUTING_INVITE_NOT_FOUND` / `OUTING_INVITE_NOT_REDEEMABLE` → 404.
11. `ensure_invited_membership` (`repository.py:4464-4502`): có sẵn hàng `left_at IS NULL` thì **trả về nguyên trạng**; chưa có thì tạo `state='invited'`, `role='member'`, `origin='link'`, `invited_by_id` = người đã mint, `joined_at=None`, `created_at=now`.

## Đầu vào

Path `token`: `str`. Không query, **không thân**.

## Đầu ra

**200** `OutingInviteAcceptResponse` (`schemas.py:745-750`), thứ tự khoá: `invite_id`, `outing_id`, `context_id`, `membership_id`, `membership_state` ∈ {`invited`,`active`}.

Cố ý **không** có tên nhóm và tên chuyến: người vừa đổi link chưa phải thành viên.

## Tác dụng phụ

Trong một giao dịch: `outing_invites.accepted_at = now`, `accepted_by_id = actor` (dưới khoá hàng), rồi `memberships` **có thể** thêm một hàng `invited`. Người đã `active` sẵn nhận lại đúng membership cũ và **không** có hàng mới (`mate_opens_a_link_as_an_active_member` → `membership_state: "active"`). Không đụng `outings`, không đụng `people`. `idempotency_keys` khi có header và 2xx; 409/404 nhả khoá.

## Idempotency

Một link một lần: lần thứ hai là 409 `invite_already_accepted`, kể cả bởi chính người đã đổi (`holder_opens_the_link_again`) hay bởi người khác (`second_opens_the_spent_link`). Khoá header lưu 200 và phát lại (`friend_opens_the_keyed_link`, `friend_replays_key`). Ba lần đồng thời: một 200 và hai 409 (`concurrency/POST-outing-invites-token-accept.yaml`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 404 | `invite_not_found` | `Invite link is not valid` | `service.py:3666-3667`, `:3676`, `:3685`, `:3691`, `:3708-3712` |
| 409 | `invite_already_accepted` | `Invite link was already used` | `service.py:3678-3682`, `:3699-3704` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 404 | (Starlette) | `{"detail":"Not Found"}` cho `/outing-invites//accept` | Starlette |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:204-214`
- Service: `services/api/app/api/service.py:3656-3729`, `:411-412` (`token_digest`)
- Repository: `services/api/app/api/repository.py:3405-3413`, `:3415-3437`, `:4464-4502`
- Schema: `services/api/app/api/schemas.py:745-750`
- Model DB: `services/api/app/db/models.py:1393-1474`

## Test đang phủ

- `services/api/tests/postgres/test_outings_postgres.py`: `test_redeeming_an_invite_link_grants_an_invited_membership_not_an_active_one` (710), `test_a_forged_or_reused_invite_link_is_refused` (768)
- `services/api/tests/postgres/test_outing_invite_lifetime_postgres.py`: `test_an_outstanding_link_stops_working_once_it_expires` (216), `test_a_link_still_inside_its_window_redeems` (266), `test_a_revoked_link_stops_working_immediately` (403), `test_only_someone_inside_the_group_can_take_a_link_back` (448), và hai ca tầng repository (320, 370)
- `services/api/tests/postgres/test_outing_invite_escalation_postgres.py`: `test_a_forwarded_invite_link_cannot_promote_itself_to_active` (75)
- `services/api/tests/postgres/test_membership_origin_postgres.py`: 3 ca (129, 183, 260)
- `services/api/tests/postgres/test_session_bootstrap_postgres.py`: `test_a_named_secret_is_refused_at_the_link_door` (292)
- `services/api/tests/api/test_outing_invite_expiry_service.py`: `test_the_service_refuses_an_expired_link_without_asking_the_repository` (123), `test_the_service_still_redeems_a_link_inside_its_window` (151) — **ca duy nhất của sóng chạy trên repository giả**

## Kịch bản parity

`parity/scenarios/w7/outings/POST-outing-invites-token-accept.yaml`, id `w7/outings/post-outing-invites-token-accept` (49 bước, `dev`):

- Token không phải uuid nên không có 422 của path: `anonymous_accepts_junk`, `holder_accepts_junk`, `holder_accepts_a_token_shaped_string`, `holder_accepts_with_url_escapes`, `holder_accepts_an_empty_token` (404 của Starlette, thân khác).
- Cửa: `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`.
- Một 404 cho mọi lý do: `friend_opens_their_own_named_secret`, `holder_opens_the_named_secret`, `owner_revokes_the_third_link` → `holder_opens_the_revoked_link`.
- Đổi và cái trần của nó: `holder_reads_own_contexts_before`, `holder_opens_the_link`, `holder_reads_own_contexts_after`, `holder_lists_the_trips` (403), `holder_checks_in` (403), `holder_reads_the_checkins` (403), `holder_mints_a_link_of_their_own` (403), `holder_approves_themselves` (403) — `invited` không mở được cửa nào của nhóm.
- Một lần: `holder_opens_the_link_again` (409), `second_opens_the_spent_link` (409), `second_opens_the_second_link` (200).
- Người đã ở trong nhóm: `mate_opens_a_link_as_an_active_member` (200, `membership_state: active`, `membership_id` là hàng cũ), `mate_lists_contexts_after`.
- Khoá: `friend_opens_the_keyed_link`, `friend_replays_key`, `friend_same_key_other_token`, `second_same_key_own_scope`.
- Framework: `trailing_slash`, `get_not_allowed`.

`concurrency/POST-outing-invites-token-accept.yaml` (12 bước): ba lần đổi một link cùng lúc; ba lần dưới một khoá; ba lần bởi một thành viên `active`.

`prod-auth.yaml`: `anonymous_accepts_a_token` (401 — vẫn cần phiên), `stranger_opens_the_link`, `stranger_opens_the_link_again` (409), `stranger_reads_checkins_after_redeeming` (403).

Corpus sinh: route này **hoãn** (`token` trong path là `str`, không phải uuid).

## Chưa phủ / lưu ý cho bản Go

- **Hết hạn không được phủ.** `OUTING_INVITE_TTL` là 7 ngày và kịch bản parity chỉ nói HTTP: không có bước đồng hồ, không có bước SQL. Nhánh `expires_at <= now` ở service (`service.py:3684`) và nhánh `OUTING_INVITE_NOT_REDEEMABLE` ở repository (`repository.py:3428`) chỉ được chạm qua **thu hồi**, vốn đi vào cùng một câu lệnh `if` nhưng qua vế kia. Bằng chứng cho vế hết hạn nằm ở `tests/postgres/test_outing_invite_lifetime_postgres.py` và ở golden vi sai của domain, không nằm trong parity.
- Không phủ `get_outing(invite.outing_id)` trả None (toàn vẹn tham chiếu vỡ): không có đường HTTP nào xoá chuyến.
- Không phủ hai **người khác nhau** đổi cùng một link cùng lúc (bước đồng thời là một persona).
- Route **không kiểm quyền**; đừng «sửa» điều đó trong bản Go.
- `/outing-invites//accept` là 404 của framework với thân `{"detail":"Not Found"}`, khác hẳn thân `ErrorResponse` của route. Bản Go phải để router trả lời, không phải handler.

## Lỗi Python (chỉ báo, không sửa)

- Hai lý do rất khác nhau — «link này đã bị rút» và «link này đã hết hạn» — dùng chung một câu với «token này không tồn tại». Cố ý theo ranh giới năng lực, nhưng nó cũng có nghĩa người dùng không bao giờ biết vì sao.
- `ensure_invited_membership` trả về membership `active` sẵn có, nên route đóng dấu `accepted_at` lên một hàng mà **không ai** được thêm vào nhóm: link bị tiêu, không đổi lại gì. Người đúc link không có cách nào biết.
