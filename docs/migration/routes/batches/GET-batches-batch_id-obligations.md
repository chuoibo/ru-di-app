# GET /batches/{batch_id}/obligations

batches · core · trạng thái trong bộ nhớ: không có

## Mục đích

Bảng thu của một đợt: ai nợ ai bao nhiêu, người nhận đã xác nhận tới đâu, có ai phản đối con số không, người nợ đã báo chuyển chưa. Ba sự thật được báo cạnh nhau và không gộp (`services/api/app/api/schemas.py:1994-2012`). Trạng thái nghĩa vụ **suy ra mỗi lần đọc** từ các lần xác nhận nhận tiền, không đọc cột lưu (luật tiền số 3).

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_batch`); `X-Actor-ID` không phải UUID → 422 (`actor_id_not_uuid`).
2. Path `batch_id: UUID` → 422 (`stranger_non_uuid_batch`).
3. `repository.list_batch_obligations` trả `None` → 404 `unknown_batch` **trước kiểm quyền** (`services/api/app/api/service.py:6762-6764`; `stranger_unknown_batch`): ai có phiên cũng biết id đợt có tồn tại.
4. `_require_permission("view_collection_board", {"is_group_member": is_member(board.context_id, actor)})` (`service.py:6772-6776`; `services/api/app/domain/permissions.py:65`): 403 `is_group_member` cho người lạ (`stranger_real_batch`), người được mời chưa nhận (`invitee_reads`), người đã rời (`mate_after_leaving`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header`); role rỗng → `role_not_permitted` (`mate_roles_empty`). Người nợ đọc được cả dòng của người khác (`mate_reads_board`).

## Đầu vào

- Path `batch_id` (UUID lax). Không query (`undeclared_query_ignored` → 200), không body.

## Đầu ra

- **200** `BatchObligationsResponse` (`schemas.py:2029-2041`), thứ tự khoá `batch_id`, `obligations`, `disputed_count`, `payment_reported_count`.
  - `batch_id` là giá trị path đã parse (UUID chữ thường có gạch).
  - `obligations[]`: `BatchObligationView` (`schemas.py:2014-2026`) theo thứ tự `obligation_id`, `sender_id`, `recipient_id`, `amount_vnd`, `obligation_status`, `disputed`, `disputed_reason`, `payment_reported_at`.
  - Chỉ nghĩa vụ của phiên bản đợt mới nhất (`services/api/app/api/repository.py:6823-6837`), `ORDER BY sender_id` **duy nhất**: hai nghĩa vụ cùng người nợ không có thứ tự do câu truy vấn quy định (`owner_board_same_sender_twice`).
  - `obligation_status` = `obligation_status(amount_vnd, receipts)` (`services/api/app/domain/ledger.py:163-216`) trên mọi `receipt_confirmations` của nghĩa vụ, xếp `confirmed_at, id` (`repository.py:7032-7039`), cộng bằng số nguyên Python: `outstanding` (chưa nhận), `partially_confirmed` (<), `confirmed` (=), `over_confirmed` (>). Không lọc `confirmed_by_id` (khác `GET /contexts/{id}/balances`).
  - `disputed`, `disputed_reason`: có sự kiện `guest_objection.wrong_amount` trên link khách của phiên bản này; lý do đầu tiên theo `occurred_at, id` thắng (`repository.py:6840-6871`).
  - `payment_reported_at`: `MIN(payment_reports.reported_at)` của nghĩa vụ (`repository.py:6873-6891`), `null` khi chưa ai báo; datetime từ `timestamptz`.
  - `disputed_count`, `payment_reported_count`: số dòng tương ứng (`service.py:6794-6797`).
- Giá trị trong kịch bản (dữ liệu mẫu, suy từ mã): đợt A có hai nghĩa vụ về owner 25 001 và 25 000 (`owner_board_outstanding`: cả hai `outstanding`); nhận 1 000 trên dòng đầu → `partially_confirmed` (`owner_board_partial`); nhận 30 000 trên dòng sau → `over_confirmed` (`third_board_partial_and_over`); đợt C nhận đúng 5 000 → `confirmed` (`owner_board_confirmed`); đợt D một lần nhận `2**63 - 1` → `over_confirmed` (`owner_board_int64_receipt`). `disputed` luôn `false`, `payment_reported_at` luôn `null` trong kịch bản.
- Framework: 307 cho `/` cuối (`trailing_slash_redirects`); `POST` → 405 (`post_not_allowed`).

## Tác dụng phụ

- Không ghi, không khoá (`session.get` và `SELECT` thường, `repository.py:6820-6891`); không idempotency (GET).
- Truy vấn: một `SELECT` receipts cho **mỗi** nghĩa vụ (`repository.py:6906`), một cho link, một cho sự kiện phản đối, một cho báo chuyển.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /batches` | `deps.py:144-163` |
| 422 | (validation path) | `{"detail":[…]}` không có `input` | `services/api/app/api/main.py:318-351` |
| 404 | `unknown_batch` | `No such batch` | `service.py:6763-6764` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:6772-6776`, `:504` |
| 409/500 | `LedgerError` từ `obligation_status` | không có handler: số tiền DB luôn nguyên dương nên không tới được | `ledger.py:206-208` |

## Mã Python

- Route: `services/api/app/api/routes/batches.py:60-75`
- Service: `services/api/app/api/service.py:6748-6798` (`list_batch_obligations`)
- Domain: `services/api/app/domain/ledger.py:143-216`; `services/api/app/domain/permissions.py:65`
- Repository: `services/api/app/api/repository.py:6812-6914`, `:7032-7039`, `:2769-2782`
- Schema: `services/api/app/api/schemas.py:1994-2041`

## Test đang phủ

- `services/api/tests/api/test_guest_objections.py`: `test_the_objected_obligation_becomes_disputed` (323), `test_every_other_obligation_is_untouched` (348), `test_an_unknown_batch_is_404_not_an_empty_board` (383), `test_evidence_request_survives_an_exhausted_objection_quota` (398), `test_a_member_of_the_group_can_read_it` (442), `test_somebody_from_another_group_cannot` (449), `test_an_actor_with_no_context_at_all_cannot` (464)
- `services/api/tests/api/test_payment_report_board.py`: `test_a_board_with_no_report_says_so_rather_than_staying_silent` (51), `test_the_next_read_of_the_board_carries_the_guest_claim` (67), `test_the_claim_does_not_move_the_payment_status` (84), `test_one_persons_claim_does_not_appear_on_another_persons_row` (97), `test_confirming_receipt_does_not_erase_the_claim` (137), `test_saying_it_twice_keeps_the_time_they_first_said_it` (166)
- `services/api/tests/postgres/test_repository_postgres.py`: `test_a_paid_obligation_can_still_be_disputed` (704), `test_an_objection_disputes_an_outstanding_obligation_in_postgres` (746), `test_asking_for_the_calculation_never_makes_an_obligation_disputed` (794), `test_the_board_reads_back_the_senders_claim_from_postgres` (827), `test_a_second_claim_does_not_move_the_time_the_first_one_was_made` (882), `test_the_guest_pressing_the_button_changes_the_advancers_next_refresh` (911), `test_an_unreported_obligation_carries_no_claim_in_postgres` (987)
- `services/api/tests/api/test_cors.py` (CORS trên route này)

## Kịch bản parity

`parity/scenarios/w4/batches/GET-batches-batch_id-obligations.yaml`, id `w4/batches/get-batches-batch_id-obligations` (50 bước), lane DB bật:

- Thứ tự: `anonymous_unknown_batch`, `anonymous_non_uuid_batch` (401), `actor_id_not_uuid`, `stranger_non_uuid_batch` (422), `stranger_unknown_batch` (404).
- 403: `stranger_real_batch`, `stranger_claims_context_header`, `invitee_reads`, `mate_roles_empty`, `mate_after_leaving`.
- 200 và trạng thái: `owner_board_outstanding`, `mate_reads_board`, `owner_confirms_part_of_first` + `owner_board_partial`, `owner_over_confirms_second` + `third_board_partial_and_over`, `mate_confirms_exact` + `owner_board_confirmed`, `mate_confirms_int64_max` + `owner_board_int64_receipt` (một lần nhận duy nhất trên nghĩa vụ đó).
- Thứ tự hoà: `owner_batches_one_sender_two_recipients` + `owner_board_same_sender_twice`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects`, `post_not_allowed`.

`parity/scenarios/w4/batches/prod-auth.yaml`: `junk_bearer_board` (401), `mate_reads_board` (200), `stranger_reads_board` (403), `owner_after_revoke` (401 sau khi thu hồi phiên).

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/get-batches-batch_id-obligations.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- `disputed`, `disputed_reason`, `payment_reported_at`, `payment_reported_count` > 0 cần link khách, mà link khách chỉ có sau `publish` thành công — giá trị token ngẫu nhiên harness chưa bind được (xem card publish). Bản Go phải giữ: lý do đầu tiên theo `occurred_at, id`, `MIN` thời điểm báo, và đếm phản đối ở mọi trạng thái thanh toán.
- `ORDER BY sender_id` không phá hoà: Python trả thứ tự Postgres chọn (thực tế là thứ tự chèn, trùng `(sender, recipient)` theo chuỗi). Bản Go nên dùng đúng câu `ORDER BY` đó; thêm khoá phụ sẽ vẫn khớp kịch bản nhưng là thay đổi hành vi không được hứa.
- Tổng nhận của một nghĩa vụ vượt int64 không có trong kịch bản: hai lần nhận lớn trên một nghĩa vụ làm view `collection_obligation_progress` ép `::bigint` lỗi (SQLSTATE 22003, `20260827_0001_initial_api_schema.py:621-646`) và làn DB dừng. Bản Go phải cộng tổng nhận không tràn (`big.Int` hoặc `numeric`) vì Python cộng không giới hạn.
- 404 trước 403 là oracle tồn tại; giữ nguyên thứ tự.
- Luôn chọn phiên bản đợt mới nhất; hiện chỉ có phiên bản 1 (không route nào tạo phiên bản 2).
