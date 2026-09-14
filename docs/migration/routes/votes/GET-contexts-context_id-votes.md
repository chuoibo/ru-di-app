# GET /contexts/{context_id}/votes

votes · core · trạng thái trong bộ nhớ: không có

## Mục đích

F17: mọi cuộc bỏ phiếu của một nhóm, cũ trước mới sau, mỗi phiếu kèm kết quả đếm và lựa chọn của **chính người đọc** (`my_option_id`). Không phân trang, không lọc mở/đóng.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`): `dev` thiếu `X-Actor-ID` → 401 kể cả khi path không phải UUID (`anonymous_non_uuid_context`); role lạ → 422 `invalid_actor_roles`. `prod` đọc bearer (`deps.py:131-140`).
2. Path `context_id` → 422 `uuid_parsing` (`stranger_non_uuid_context`, `stranger_short_uuid`).
3. `_require_permission("view_votes", actor, {"is_group_member": ...})` (`services/api/app/api/service.py:2790-2794`; `services/api/app/domain/permissions.py:270-273`): thiếu `group_admin`/`member` → 403 `role_not_permitted`; không có membership ACTIVE (`services/api/app/api/repository.py:2769-2782`) → 403 `is_group_member`.
   - Nhóm không tồn tại → 403 `is_group_member`, không 404 (`stranger_unknown_context`).
   - `X-Actor-Contexts` được parse (sai dạng → 422 `invalid_actor_contexts`) nhưng không bao giờ được hỏi: người lạ khai nhóm trong header vẫn 403 (`stranger_claims_the_group_in_header`), chủ nhóm khai nhóm khác vẫn 200 (`owner_contexts_header_ignored`).
4. GET không đi qua `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:76`, `:405-407`): `Idempotency-Key` rỗng bị bỏ qua (`get_ignores_idempotency_key` → 200).

## Đầu vào

- Path `context_id`: UUID lax (chữ hoa, không gạch, `%7B...%7D`, `urn:uuid:` đều parse; `stranger_uppercase_uuid`, `stranger_unhyphenated_uuid`, `stranger_braced_uuid`, `stranger_urn_uuid` → 403).
- Không đọc query: `?limit=1&after=x` bị bỏ qua (`query_string_ignored`). Không đọc body.

## Đầu ra

- **200** `VoteListResponse` (`services/api/app/api/schemas.py:703-705`), thứ tự khoá `context_id`, `votes`. Nhóm chưa có phiếu → `{"context_id":"...","votes":[]}` (`owner_lists_empty`).
- `context_id` là UUID của path đã parse, ghi lại dạng chuẩn chữ thường có gạch.
- `votes[]` là `VoteResponse` (`schemas.py:686-700`), cùng thứ tự khoá và cùng cách dựng như card `POST /contexts/{context_id}/votes` (`_wire_vote`, `service.py:925-963`).
- Thứ tự phiếu: `ORDER BY created_at, id` (`repository.py:3028-3034`); phiếu đóng và mở lẫn nhau. `options` theo `position`; ballot đọc `ORDER BY created_at, id` (`repository.py:2367-2389`).
- Kết quả đếm (`services/api/app/domain/vote.py:20-63`): `ballot_count` mỗi lựa chọn; `total_ballots`; `leading_option_ids` là các lựa chọn có số phiếu cao nhất **theo thứ tự position**, rỗng khi chưa ai bỏ; `is_tie` khi có hơn một người dẫn; `decided_option_id` chỉ khác null khi đúng một người dẫn.
- `my_option_id`: lựa chọn trong ballot của người đọc, null nếu chưa bỏ (`service.py:935-938`). Không có danh sách người bỏ phiếu.
- Ballot của người đã rời nhóm hoặc đã xoá tài khoản **vẫn được đếm**; phiếu do người đã rời tạo vẫn nằm trong danh sách với `created_by_id` cũ (`owner_lists_after_leaver_left`, `mate_lists_after_ghost_deleted`; chính sách "keep" ở `services/api/app/domain/account_lifecycle.py:22-24`, `:123-125`).
- Datetime: `created_at`, `closed_at` dạng `YYYY-MM-DDTHH:MM:SS.ffffffZ`. Không float.
- Framework: 307 cho `/` cuối; **HEAD → 405** không body và PUT → 405, cả hai với `allow: POST` (route POST cùng path đứng trước trong router).

## Tác dụng phụ

- Chỉ đọc: `memberships`, `votes`, và cho **mỗi** phiếu thêm hai SELECT `vote_options`, `vote_ballots` (N+1, `repository.py:2367-2377`).
- Không idempotency (GET), không limiter, không ghi bảng nào.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |

- 422 framework: `uuid_parsing` trên `["path","context_id"]` (`msg` và `ctx.error` nêu ký tự hoặc nhóm sai).
- Không có 404.
- 500 lý thuyết: `tally` ném `VoteError` khi dữ liệu hỏng (hai ballot cùng người, `vote.py:35-43`); unique `uq_vote_ballots_one_per_person` (`services/api/app/db/models.py:1727-1731`) khiến điều đó không xảy ra qua HTTP. `services/api/tests/postgres/test_vote_ballots_postgres.py:266` cho thấy khi xảy ra thì kết quả không đọc được nữa.

## Mã Python

- Route: `services/api/app/api/routes/votes.py:46-56`
- Service: `services/api/app/api/service.py:2787-2801` (`list_context_votes`), `:925-963` (`_wire_vote`)
- Repository: `services/api/app/api/repository.py:3028-3034` (`list_votes`), `:2367-2389` (`_vote_record`), `:2769-2782` (`is_member`)
- Domain: `services/api/app/domain/vote.py:20-63`, `services/api/app/domain/permissions.py:270-273`

## Test đang phủ

- `services/api/tests/postgres/test_votes_postgres.py`: `test_a_vote_from_another_group_never_appears_in_this_contexts_list` (712), `test_an_outsider_cannot_list_a_groups_votes` (733), `test_someone_who_left_stops_listing_the_groups_votes` (756)
- `services/api/tests/domain/test_vote_tally.py` (37-193)

## Kịch bản parity

`parity/scenarios/w2/votes/GET-contexts-context_id-votes.yaml`, id `w2/votes/get-contexts-context_id-votes` (60 bước):

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401), `stranger_non_uuid_context`, `stranger_short_uuid` (422), `stranger_unknown_context`, `stranger_uppercase_uuid`, `stranger_unhyphenated_uuid`, `stranger_braced_uuid`, `stranger_urn_uuid` (403).
- Nội dung: `owner_lists_empty`, `owner_lists` (ba phiếu: đóng có ballot, gắn chuyến đi có `place_name`, do người khác tạo), `mate_lists` và `ghost_lists_before_deletion` (`my_option_id` khác nhau theo người đọc), `invitee_lists_after_accepting` (nhận lời muộn vẫn đọc phiếu cũ).
- Cô lập nhóm: `stranger_lists_own_group`, `owner_lists_other_group` (403), `stranger_real_group`, `stranger_claims_the_group_in_header`, `owner_contexts_header_ignored`.
- Ma trận thành viên: `invitee_not_yet_active`, `leaver_after_leaving`, `ghost_after_deletion` (403); `owner_lists_after_leaver_left`, `mate_lists_after_ghost_deleted` (dữ liệu của người đi vẫn ở lại).
- Role: `owner_roles_group_admin_only`, `owner_roles_member_only` (200), `owner_roles_empty`, `owner_roles_guest` (403), `owner_roles_unknown` (422).
- Framework: `query_string_ignored`, `get_ignores_idempotency_key`, `trailing_slash_redirects` (307), `head_not_allowed`, `put_not_allowed` (405 `allow: POST`).
- `prod`: `parity/scenarios/w2/votes/prod-auth.yaml` (`anonymous_non_uuid_list`, `actor_headers_ignored`, `stranger_lists_claiming_owner_id`, `mate_lists`, `mate_after_deletion`).

## Chưa phủ / lưu ý cho bản Go

- `prod` được so trên cặp stack `prod` mới dựng: 3 lần `parity run` (lane DB bật) đều `scenarios=5 steps=126 differences=0`, canary bắt đủ (xem `w2/votes/prod-auth`); ở `prod` `role_not_permitted` không tới được.
- Harness không đổi được dạng chữ của id đã bind, nên chưa so việc `context_id` trong body 200 được chuẩn hoá về chữ thường khi path viết hoa. Bản Go phải ghi UUID dạng chuẩn, không lặp lại chuỗi path.
- Hai phiếu cùng `created_at` xếp theo `id` (ngẫu nhiên); harness không tạo được.
- `leading_option_ids` theo `position`, không theo id hay số phiếu; `decided_option_id` null khi hoà hoặc khi chưa có phiếu.
- HEAD/PUT trả `allow: POST` dù route là GET: hành vi của router Starlette, bản Go phải tái tạo hoặc mở ADR.
- N+1 query: bản Go có gom query thì thứ tự ballot vẫn phải là `created_at, id` để `my_option_id` và tally giữ nguyên (tally không phụ thuộc thứ tự, nhưng lỗi dữ liệu hỏng thì có).
