# POST /votes/{vote_id}/ballots

votes · core · trạng thái trong bộ nhớ: không có

## Mục đích

F17: bỏ hoặc đổi lá phiếu duy nhất của người gọi trong một cuộc bỏ phiếu còn mở. Người bỏ phiếu luôn là actor; gửi lại (kể cả cùng lựa chọn) thay dòng cũ, không bao giờ thành hai phiếu.

## Xác thực và quyền

Thứ tự từ ngoài vào (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-502`).
2. JSON hỏng → 422 `json_invalid` trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): `dev` thiếu `X-Actor-ID` → 401 kể cả path không phải UUID (`anonymous_non_uuid_vote`); role lạ → 422 (`mate_roles_unknown`).
4. Path `vote_id` và body cùng validate, gom một mảng 422, path trước (`stranger_non_uuid_and_bad_body`). Chạy trước handler: body sai trên phiếu lạ là 422 chứ không 404 (`stranger_unknown_vote_bad_body`), body sai trên phiếu đã đóng là 422 chứ không 409 (`mate_closed_bad_body`).
5. Tìm phiếu (`services/api/app/api/service.py:2820-2822`) → 404 `vote_not_found`, **trước** kiểm role (`stranger_unknown_vote_roles_empty` → 404).
6. `_require_permission("cast_vote_ballot", ...)` (`service.py:2823-2827`; `services/api/app/domain/permissions.py:274-277`): thiếu `group_admin`/`member` → 403 `role_not_permitted` (`mate_roles_empty`, `mate_roles_guest`); không ACTIVE ở nhóm của phiếu → 403 `is_group_member`. Chạy trước kiểm lựa chọn và trạng thái đóng: người lạ gửi lựa chọn của phiếu khác → 403 (`stranger_real_vote_foreign_option`), người lạ trên phiếu đã đóng → 403 (`stranger_closed_vote`).
7. Phiếu đã đóng (`service.py:2828-2829`) → 409 `vote_closed`, **trước** kiểm lựa chọn (`mate_closed_foreign_option` → 409).
8. `option_id` không thuộc phiếu này (`service.py:2830-2835`) → 422 `unknown_option`: lựa chọn của phiếu khác, id lạ, id viết hoa không khớp.
9. Repository khoá dòng `votes` `FOR UPDATE` rồi kiểm lại ba điều trên (`services/api/app/api/repository.py:3044-3061`), ánh xạ về cùng ba mã (`service.py:2844-2855`). Nhánh này chỉ tới được khi có race (phiếu bị đóng giữa bước 7 và bước ghi).

## Đầu vào

- Path `vote_id`: UUID lax.
- Header `Idempotency-Key`, `Content-Type` (không có thì vẫn parse JSON: `no_content_type_is_parsed_as_json` → 200; `text/plain` → 422 `model_attributes_type`).
- Body `VoteBallotRequest` (`services/api/app/api/schemas.py:674-675`), `extra="forbid"`: đúng một trường `option_id: UUID` bắt buộc. Không có trường nào nói bỏ thay ai (`extra_field_voter_id` → 422 `extra_forbidden`). `null` hoặc số → `uuid_type`; chuỗi sai → `uuid_parsing`.
- So `option_id` với các lựa chọn bằng phép so `UUID` (`service.py:2830`), nên dạng chữ không quan trọng khi id có thật.

## Đầu ra

- **200** (POST nhưng không phải 201, cả khi tạo mới) `VoteBallotResponse` (`schemas.py:708-714`), thứ tự khoá `vote_id`, `option_id`, `voter_id`, `created_at`, `updated_at`, `replaced_previous_ballot`.
- Lần đầu: `created_at == updated_at` (cùng một `now`), `replaced_previous_ballot` false (`mate_first_ballot`).
- Lần sau, **kể cả cùng lựa chọn**: `created_at` giữ nguyên, `updated_at` là `now` mới, `replaced_previous_ballot` true (`mate_same_option_again`, `mate_changes_mind`).
- Datetime: `_now()` của Python (`service.py:2842`), wire `YYYY-MM-DDTHH:MM:SS.ffffffZ`. Không float.
- Replay idempotency: cùng 200 và cùng body, thêm `idempotency-replayed: true` (`idempotency.py:585-599`). Replay thắng mọi kiểm tra: lá phiếu đã lưu vẫn được trả lại sau khi phiếu đóng (`idem_replay_after_close` → 200), trong khi key mới lúc đó ra 409 (`idem_new_key_after_close`).
- Framework: 307 cho `/` cuối; GET → 405 với `allow: POST`.

## Tác dụng phụ

- SELECT `votes` (không khoá, `repository.py:3024-3026`), `vote_options`, `vote_ballots`, `memberships`; rồi trong `upsert_ballot` (`repository.py:3036-3085`): `SELECT votes ... FOR UPDATE`, SELECT `vote_options` khớp `(vote_id, id)`, `SELECT vote_ballots ... FOR UPDATE` theo `(vote_id, voter_id)`.
- Chưa có ballot → INSERT `vote_ballots` (`created_at = updated_at = now`). Đã có → UPDATE `option_id`, `updated_at`. Unique `uq_vote_ballots_one_per_person` (`services/api/app/db/models.py:1727-1731`).
- Ballot của người đã rời hoặc đã xoá tài khoản ở lại và vẫn được đếm; họ không đổi được nữa (`leaver_after_leaving`, `ghost_after_deletion` → 403).
- Commit trước response. Idempotency: chỉ lưu 2xx; 403/404/409/422 nhả key (`idempotency.py:504-546`); scope là actor (`owner_same_key_as_mate` → phiếu mới của chủ nhóm, không phải replay).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 404 | `vote_not_found` | `Vote does not exist` | `service.py:2821-2822`, `:2845-2846` |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |
| 409 | `vote_closed` | `Vote is closed` | `service.py:2828-2829`, `:2847-2848` |
| 422 | `unknown_option` | `Option does not belong to this vote` | `service.py:2830-2835`, `:2849-2854` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

- 422 framework đã đo: `json_invalid`, `uuid_parsing` (path và `option_id`), `uuid_type` (`null`, số), `missing` (`option_id`, cả body), `extra_forbidden`, `model_attributes_type` (mảng, `text/plain`).

## Mã Python

- Route: `services/api/app/api/routes/votes.py:72-83`
- Service: `services/api/app/api/service.py:2814-2863` (`cast_vote_ballot`)
- Repository: `services/api/app/api/repository.py:3036-3085` (`upsert_ballot`), `:3024-3026` (`get_vote`), `:2769-2782` (`is_member`)
- Domain: `services/api/app/domain/permissions.py:274-277`
- Model: `services/api/app/db/models.py:1720-1756`

## Test đang phủ

- `services/api/tests/postgres/test_votes_postgres.py`: `test_one_person_replaces_their_ballot_without_creating_a_second_row` (519), `test_changing_a_ballot_decrements_the_old_count_and_increments_the_new_count` (551), `test_closing_a_vote_blocks_ballots_without_changing_stored_ballot_count` (583), `test_a_stranger_can_neither_create_read_nor_ballot` (663), `test_a_former_member_loses_read_and_ballot_access` (688), `test_an_option_from_another_vote_is_rejected_as_unknown` (779)
- `services/api/tests/postgres/test_vote_ballots_postgres.py`: `test_database_refuses_a_second_ballot_from_the_same_voter` (214), `test_a_leaked_duplicate_makes_the_result_permanently_unreadable` (266), `test_two_people_voting_at_once_both_land_and_the_result_reads` (368), `test_one_person_racing_themselves_still_leaves_exactly_one_ballot` (407)

## Kịch bản parity

`parity/scenarios/w2/votes/POST-votes-vote_id-ballots.yaml`, id `w2/votes/post-votes-vote_id-ballots` (75 bước):

- Thứ tự từ chối: `anonymous_unknown_vote`, `anonymous_non_uuid_vote` (401), `anonymous_malformed_json` (422 trước 401), `stranger_non_uuid_vote`, `stranger_non_uuid_and_bad_body` (422), `stranger_unknown_vote` (404), `stranger_unknown_vote_bad_body` (422 trước 404), `stranger_unknown_vote_roles_empty` (404 trước 403), `stranger_real_vote`, `stranger_real_vote_foreign_option` (403 trước 422), `stranger_closed_vote` (403 trước 409), `mate_closed_foreign_option` (409 trước 422), `mate_closed_bad_body` (422 trước 409).
- Ma trận thành viên: `invitee_not_yet_active`, `leaver_after_leaving`, `ghost_after_deletion` (403); `owner_creator_votes`, `ghost_votes` (200).
- Role: `mate_roles_empty`, `mate_roles_guest` (403), `mate_roles_unknown` (422), `leaver_roles_group_admin_only` (200).
- Upsert: `mate_first_ballot`, `mate_same_option_again`, `mate_changes_mind`, `no_content_type_is_parsed_as_json`, cùng các lần đọc `owner_reads_tally`, `owner_reads_tally_after_departures`, `mate_reads_second_vote`.
- Lựa chọn lạ: `mate_option_from_other_vote`, `mate_unknown_option`, `mate_uppercase_option_id`, `stranger_own_vote_with_this_votes_option` (422).
- Đã đóng: `owner_closes_vote`, `mate_ballot_closed` (409).
- Validate: `option_id_not_uuid`, `option_id_missing`, `option_id_null`, `option_id_number`, `extra_field_voter_id`, `empty_body`, `top_level_array`, `text_plain_content_type`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_second_distinct_key` (chạy lại, `replaced_previous_ballot` true), `idem_refusal_not_stored` + `idem_same_key_after_refusal` (422 nhả key, cùng key khác body thành request mới), `owner_same_key_as_mate`, `idem_replay_after_close`, `idem_new_key_after_close`.
- `prod`: `parity/scenarios/w2/votes/prod-auth.yaml` (`anonymous_malformed_json`, `lowercase_scheme_ballot`, `mate_changes_ballot`, `idem_first`, `idem_replay`, `stranger_same_key`, `anonymous_same_key_with_owner_actor_header`, `mate_ballot_after_close`).

## Chưa phủ / lưu ý cho bản Go

- `prod` được so trên cặp stack `prod` mới dựng: 3 lần `parity run` (lane DB bật) đều `scenarios=5 steps=126 differences=0`, canary bắt đủ (xem `w2/votes/prod-auth`); ở `prod` scope idempotency là digest bearer.
- Nhánh race ở repository (`VOTE_NOT_FOUND`, `VOTE_CLOSED`, `UNKNOWN_OPTION` sau khi khoá) và 409 in-flight cần request đồng thời; harness chạy tuần tự.
- Harness không viết hoa được id đã bind, nên chưa chứng minh được việc id lựa chọn có thật viết hoa vẫn được nhận (Python so `UUID`, không so chuỗi). `mate_uppercase_option_id` chỉ là id lạ viết hoa.
- 200 cho cả tạo mới lẫn thay thế; bản Go không được đổi thành 201.
- Cùng lựa chọn gửi lại vẫn UPDATE `updated_at` và báo `replaced_previous_ballot` true; bản Go không được bỏ qua ghi "vì không đổi gì" (DB lane sẽ lệch).
- Thứ tự khoá `FOR UPDATE` (votes rồi ballot) là thứ giữ hai lần bỏ phiếu đồng thời của một người thành một dòng; unique index là lớp thứ hai. Bản Go phải khoá cùng thứ tự.
- Replay đứng trước mọi kiểm tra trạng thái: lá phiếu đã lưu vẫn được trả sau khi phiếu đóng.
