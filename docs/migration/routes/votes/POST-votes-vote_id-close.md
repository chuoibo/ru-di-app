# POST /votes/{vote_id}/close

votes · core · trạng thái trong bộ nhớ: không có

## Mục đích

F17: người đã mở cuộc bỏ phiếu chốt nó. Sau khi đóng, không ai bỏ hay đổi phiếu được nữa và kết quả đếm đứng yên; phiếu vẫn đọc được. Không có mở lại.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-502`): key rỗng → 422 (`idem_key_empty`).
2. Route **không có tham số body**, nên body không bao giờ được giải mã: JSON hỏng không sinh 422, người vô danh gửi JSON hỏng vẫn nhận 401 (`anonymous_malformed_json_is_not_read`); chủ nhóm gửi JSON thừa hoặc `text/plain` vẫn đóng được (`owner_closes_with_ignored_json_body`, `owner_closes_with_text_body`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): `dev` thiếu `X-Actor-ID` → 401 kể cả path không phải UUID (`anonymous_non_uuid_vote`); role lạ → 422 (`owner_roles_unknown`).
4. Path `vote_id` → 422 `uuid_parsing` (`stranger_non_uuid_vote`).
5. Tìm phiếu (`services/api/app/api/service.py:2866-2868`) → 404 `vote_not_found`, trước kiểm role (`stranger_unknown_vote_roles_empty` → 404).
6. `_require_permission("close_vote", ...)` (`service.py:2869-2878`; bảng quyền `services/api/app/domain/permissions.py:278-281`, đánh giá theo thứ tự `:673-677`):
   - thiếu `group_admin`/`member` → 403 `role_not_permitted` (`owner_roles_empty`, `owner_roles_guest`);
   - không ACTIVE ở nhóm của phiếu → 403 `is_group_member`, được nêu **trước** `is_vote_creator` (`stranger_real_vote`, `invitee_not_yet_active`);
   - không phải người tạo (`record.created_by_id == actor.id`) → 403 `is_vote_creator` (`mate_not_creator`); claim `group_admin` không thay được (`mate_not_creator_group_admin_role`).
7. Đã đóng (`service.py:2879-2884`) → 409 `vote_already_closed`. Chạy **sau** quyền: người không phải người tạo gọi trên phiếu đã đóng nhận 403 chứ không 409 (`mate_closes_closed_vote`, `stranger_closes_closed_vote`).
8. Repository khoá `votes FOR UPDATE` và kiểm lại (`services/api/app/api/repository.py:3094-3100`), ánh xạ `VOTE_NOT_FOUND` → 404, `VOTE_ALREADY_CLOSED` → 409 (`service.py:2891-2900`); chỉ tới được khi có race.

Hệ quả: người tạo đã rời nhóm hoặc đã xoá tài khoản thì không đóng được (`leaver_closes_after_leaving`, `ghost_closes_after_deletion` → 403 `is_group_member`), và người khác cũng không (`owner_closes_leavers_vote` → 403 `is_vote_creator`). Phiếu đó mở mãi.

## Đầu vào

- Path `vote_id`: UUID lax.
- Header `Idempotency-Key` tuỳ chọn. Mọi body bị bỏ qua khi xử lý, nhưng vẫn đi vào fingerprint idempotency (`idem_reuse_different_body`).

## Đầu ra

- **200** `VoteResponse` (`services/api/app/api/schemas.py:686-700`), dựng bằng `_wire_vote` (`service.py:925-963`) trên bản ghi vừa đóng: `closed_at` là `now` của lần đóng, `is_closed` true, kết quả đếm tính tại lúc đóng, `my_option_id` của người tạo.
- `closed_at`: `_now()` của Python (`service.py:2889`), wire `YYYY-MM-DDTHH:MM:SS.ffffffZ`. `closed_by_id` được lưu nhưng không có trên wire.
- Replay idempotency: 200 đã lưu kèm `idempotency-replayed: true`, dù một lần đóng mới lúc đó sẽ là 409 (`idem_replay`).
- Framework: 307 cho `/` cuối; GET → 405 với `allow: POST`.

## Tác dụng phụ

- SELECT `votes`, `vote_options`, `vote_ballots`, `memberships`; `SELECT votes ... FOR UPDATE`; UPDATE `votes SET closed_at = now, closed_by_id = actor` (`repository.py:3087-3104`). CHECK `(closed_at IS NULL) = (closed_by_id IS NULL)` (`services/api/app/db/models.py:1653-1656`).
- Không xoá hay đổi ballot nào.
- Commit trước response. Idempotency: chỉ lưu 2xx; 403/409 nhả key (`idem_refusal_not_stored` → `idem_first_after_refusal`; `idem_second_distinct_key` và `idem_second_distinct_key_again` cùng ra 409 vì 409 không được lưu). Scope theo actor (`mate_same_key_as_owner` → 403 của chính người đó).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 404 | `vote_not_found` | `Vote does not exist` | `service.py:2867-2868`, `:2892-2893` |
| 403 | `permission_denied` | `role_not_permitted`, `is_group_member` hoặc `is_vote_creator` | `service.py:502-504` |
| 409 | `vote_already_closed` | `Vote is already closed` | `service.py:2879-2884`, `:2894-2899` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

- 422 framework: chỉ `uuid_parsing` trên `["path","vote_id"]`; không có lỗi body nào.

## Mã Python

- Route: `services/api/app/api/routes/votes.py:86-96`
- Service: `services/api/app/api/service.py:2865-2901` (`close_vote`), `:925-963` (`_wire_vote`)
- Repository: `services/api/app/api/repository.py:3087-3104` (`close_vote`), `:3024-3026` (`get_vote`), `:2769-2782` (`is_member`)
- Domain: `services/api/app/domain/permissions.py:278-281`, `:654-678`

## Test đang phủ

- `services/api/tests/postgres/test_votes_postgres.py`: `test_closing_a_vote_blocks_ballots_without_changing_stored_ballot_count` (583), `test_closing_an_already_closed_vote_is_rejected` (608), `test_a_member_who_did_not_create_the_vote_cannot_close_it` (622), `test_a_closed_vote_remains_readable_as_a_result` (639), `test_a_vote_lifecycle_never_changes_any_money_table` (873)

## Kịch bản parity

`parity/scenarios/w2/votes/POST-votes-vote_id-close.yaml`, id `w2/votes/post-votes-vote_id-close` (60 bước):

- Thứ tự từ chối: `anonymous_unknown_vote`, `anonymous_non_uuid_vote`, `anonymous_malformed_json_is_not_read` (401), `stranger_non_uuid_vote` (422), `stranger_unknown_vote`, `stranger_unknown_vote_roles_empty` (404 trước 403), `stranger_real_vote`, `invitee_not_yet_active` (403 `is_group_member`), `mate_not_creator`, `mate_not_creator_group_admin_role` (403 `is_vote_creator`), `owner_roles_empty`, `owner_roles_guest` (403 `role_not_permitted`), `owner_roles_unknown` (422).
- Đóng: `owner_closes` (200), `owner_closes_again` (409), `mate_closes_closed_vote`, `stranger_closes_closed_vote` (403 trước 409), `owner_reads_closed_vote`.
- Body bị bỏ qua: `owner_closes_with_ignored_json_body`, `owner_closes_with_text_body`; `owner_closes_with_group_admin_only`.
- Người tạo đi mất: `leaver_closes_after_leaving`, `owner_closes_leavers_vote`, `ghost_closes_after_deletion`, `owner_lists_votes` (phiếu của họ vẫn mở).
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).
- Idempotency: `idem_refusal_not_stored`, `idem_first_after_refusal`, `idem_replay`, `idem_no_key_after_close`, `idem_second_distinct_key`, `idem_second_distinct_key_again`, `idem_reuse_different_body`, `mate_same_key_as_owner`, `idem_key_empty`.
- `prod`: `parity/scenarios/w2/votes/prod-auth.yaml` (`basic_scheme`, `mate_not_creator_close`, `owner_closes`, `stranger_still_refused`).

## Chưa phủ / lưu ý cho bản Go

- `prod` được so trên cặp stack `prod` mới dựng: 3 lần `parity run` (lane DB bật) đều `scenarios=5 steps=126 differences=0`, canary bắt đủ (xem `w2/votes/prod-auth`).
- Nhánh race ở repository và 409 in-flight cần request đồng thời; harness chạy tuần tự.
- Handler Go không được đọc hay validate body: JSON hỏng, `text/plain`, trường thừa đều phải đi tiếp như không có body. Nhưng middleware idempotency vẫn băm body vào fingerprint.
- Thứ tự predicate trong 403 là thứ tự khai báo trong bảng quyền (`is_group_member` trước `is_vote_creator`); bản Go gộp hai điều kiện thì sẽ lệch `detail`.
- Phiếu của người tạo đã rời nhóm hoặc xoá tài khoản không đóng được nữa: là hành vi đo được, muốn đổi thì mở ADR.
