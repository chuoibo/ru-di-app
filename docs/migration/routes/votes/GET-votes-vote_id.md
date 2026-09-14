# GET /votes/{vote_id}

votes · core · trạng thái trong bộ nhớ: không có

## Mục đích

F17: một cuộc bỏ phiếu với kết quả đếm hiện tại (hoặc kết quả chốt khi đã đóng) và lựa chọn của chính người đọc. Không bao giờ nói ai bỏ cho lựa chọn nào.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`): `dev` thiếu `X-Actor-ID` → 401 kể cả khi path không phải UUID (`anonymous_non_uuid_vote`); role lạ → 422 (`owner_roles_unknown`). `prod` đọc bearer.
2. Path `vote_id` → 422 `uuid_parsing` (`stranger_non_uuid_vote`, `stranger_short_uuid`).
3. Tìm phiếu (`services/api/app/api/service.py:2804-2806`) → 404 `vote_not_found`. Bước này chạy **trước** kiểm quyền: id lạ là 404 cho cả người lạ lẫn người không có role (`stranger_unknown_vote`, `owner_roles_empty_unknown_vote`), còn phiếu có thật của nhóm khác là 403 (`stranger_real_vote`). Route vì vậy cho người đã đăng nhập biết một id phiếu có tồn tại hay không.
4. `_require_permission("view_votes", actor, {"is_group_member": is_member(record.context_id, actor.id)})` (`service.py:2807-2811`; `services/api/app/domain/permissions.py:270-273`): thiếu `group_admin`/`member` → 403 `role_not_permitted`; không có membership ACTIVE ở nhóm của phiếu (`services/api/app/api/repository.py:2769-2782`) → 403 `is_group_member`.

## Đầu vào

- Path `vote_id`: UUID lax (`stranger_uppercase_unknown_vote` → 404, tức là parse được).
- Query và body không được đọc (`query_string_ignored`).

## Đầu ra

- **200** `VoteResponse` (`services/api/app/api/schemas.py:686-700`), thứ tự khoá như card `POST /contexts/{context_id}/votes`, dựng ở `_wire_vote` (`service.py:925-963`).
- Kết quả đếm (`services/api/app/domain/vote.py:20-63`), đã đo từng trạng thái:
  - chưa ai bỏ: mọi `ballot_count` 0, `total_ballots` 0, `leading_option_ids` `[]`, `is_tie` false, `decided_option_id` null (`owner_reads_fresh`);
  - một người dẫn: `leading_option_ids` một phần tử, `decided_option_id` bằng nó (`owner_reads_single_leader`);
  - hoà hai và hoà ba: `leading_option_ids` theo thứ tự `position`, `is_tie` true, `decided_option_id` null (`owner_reads_two_way_tie`, `owner_reads_three_way_tie`);
  - hết hoà (`owner_reads_tie_broken`), đổi phiếu làm đổi người dẫn (`mate_reads_after_change`).
- `my_option_id` là lựa chọn của người đọc (`service.py:935-938`), khác nhau giữa `mate_reads_own_choice` và bước đọc của chủ nhóm.
- Ballot của người đã rời hoặc đã xoá tài khoản vẫn được đếm (`owner_reads_after_leaver_left`, `owner_reads_after_ghost_deleted`; `services/api/app/domain/account_lifecycle.py:22-24`).
- Phiếu đã đóng vẫn đọc được: `closed_at` khác null, `is_closed` true (`mate_reads_closed`); người nhận lời mời **sau** khi phiếu đóng cũng đọc được (`invitee_reads_closed`).
- `outing_id` là id chuyến đi khi phiếu gắn chuyến đi; `place_name` null hoặc chuỗi đã strip.
- Datetime `YYYY-MM-DDTHH:MM:SS.ffffffZ`. Không float.
- Framework: 307 cho `/` cuối (`location: http://<Host>/votes/<id>`); POST và DELETE → 405 với `allow: GET`.

## Tác dụng phụ

- Chỉ đọc: `votes` theo khoá chính (`repository.py:3024-3026`), `vote_options` `ORDER BY position`, `vote_ballots` `ORDER BY created_at, id` (`repository.py:2367-2389`), `memberships`.
- Không idempotency (GET), không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 404 | `vote_not_found` | `Vote does not exist` | `service.py:2805-2806` |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |

- 422 framework: `uuid_parsing` trên `["path","vote_id"]`.
- 500 lý thuyết: `VoteError` từ `tally` khi dữ liệu hỏng, không tới được qua HTTP (unique `uq_vote_ballots_one_per_person`, `services/api/app/db/models.py:1727-1731`).

## Mã Python

- Route: `services/api/app/api/routes/votes.py:59-69`
- Service: `services/api/app/api/service.py:2803-2812` (`get_vote_results`), `:925-963` (`_wire_vote`)
- Repository: `services/api/app/api/repository.py:3024-3026` (`get_vote`), `:2367-2389` (`_vote_record`), `:2769-2782` (`is_member`)
- Domain: `services/api/app/domain/vote.py:20-63`, `services/api/app/domain/permissions.py:270-273`

## Test đang phủ

- `services/api/tests/postgres/test_votes_postgres.py`: `test_a_missing_vote_is_reported_as_not_found` (389), `test_seven_members_produce_the_spec_four_two_one_result` (401), `test_a_two_two_tie_decides_nothing_and_orders_both_leaders_by_position` (446), `test_a_three_way_tie_decides_nothing_and_lists_all_leaders` (476), `test_an_unanswered_vote_has_zero_counts_and_no_leader` (500), `test_a_closed_vote_remains_readable_as_a_result` (639), `test_a_stranger_can_neither_create_read_nor_ballot` (663), `test_a_former_member_loses_read_and_ballot_access` (688), `test_each_member_reads_only_their_own_selected_option_id` (795), `test_vote_results_never_expose_the_identity_of_a_voter` (820)
- `services/api/tests/domain/test_vote_tally.py` (37-193)

## Kịch bản parity

`parity/scenarios/w2/votes/GET-votes-vote_id.yaml`, id `w2/votes/get-votes-vote_id` (59 bước):

- Thứ tự từ chối: `anonymous_unknown_vote`, `anonymous_non_uuid_vote` (401), `stranger_non_uuid_vote`, `stranger_short_uuid` (422), `stranger_unknown_vote`, `stranger_uppercase_unknown_vote`, `owner_roles_empty_unknown_vote` (404 trước 403), `stranger_real_vote` (403).
- Role: `owner_roles_empty_real_vote`, `owner_roles_guest` (403), `owner_roles_group_admin_only` (200), `owner_roles_unknown` (422).
- Kết quả đếm: `owner_reads_fresh`, `owner_reads_single_leader`, `mate_reads_own_choice`, `owner_reads_two_way_tie`, `owner_reads_three_way_tie`, `owner_reads_tie_broken`, `mate_reads_after_change`.
- Ma trận thành viên: `invitee_not_yet_active`, `leaver_after_leaving`, `ghost_after_deletion` (403); `owner_reads_after_leaver_left`, `owner_reads_after_ghost_deleted`.
- Đã đóng: `mate_reads_closed`, `invitee_reads_closed`.
- Nhóm khác: `stranger_reads_own_vote` (200), `owner_reads_other_groups_vote` (403).
- Framework: `query_string_ignored`, `trailing_slash_redirects` (307), `post_not_allowed`, `delete_not_allowed` (405 `allow: GET`).
- `prod`: `parity/scenarios/w2/votes/prod-auth.yaml` (`junk_bearer`, `stranger_unknown_vote`, `stranger_real_vote`, `owner_reads_closed`, `owner_after_revoke`).

## Chưa phủ / lưu ý cho bản Go

- `prod` được so trên cặp stack `prod` mới dựng: 3 lần `parity run` (lane DB bật) đều `scenarios=5 steps=126 differences=0`, canary bắt đủ (xem `w2/votes/prod-auth`).
- 404 trước 403 là hành vi đo được: bản Go không được "sửa" thành 403 cho id lạ mà không mở ADR.
- Kết quả hoà nhiều hơn ba lựa chọn và phiếu 20 lựa chọn có ballot chưa được đọc qua route này; quy tắc là một hàm thuần có test domain riêng.
- Harness không đổi được dạng chữ của id đã bind, nên chưa so `id` trong body khi path viết hoa id có thật (Python ghi dạng chuẩn chữ thường).
- `leading_option_ids` theo `position`, không theo thứ tự ballot; bản Go dùng map thì phải xếp lại.
