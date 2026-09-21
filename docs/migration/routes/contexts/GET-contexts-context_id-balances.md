# GET /contexts/{context_id}/balances

contexts · core · trạng thái trong bộ nhớ: không có

## Mục đích

Số dư ròng của từng người trong nhóm và một đề xuất chuyển tiền để thanh toán hết, **tính lại mỗi lần gọi** từ sổ: phiên bản mới nhất đã xác nhận của từng khoản chi và các lần người nhận xác nhận đã nhận tiền. Không đọc cột số dư nào (luật tiền số 3). Đề xuất chuyển tiền là bản nháp cần đồng thuận, không phải nghĩa vụ (`services/api/app/domain/ledger.py:219-232`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_context`); `X-Actor-ID` không phải UUID → 422 (`actor_id_not_uuid`).
2. Path `context_id: UUID` → 422 (`stranger_non_uuid_context`).
3. `_require_permission("view_context_members", {"is_group_member": …})` (`services/api/app/api/service.py:1750-1754`; `services/api/app/domain/permissions.py:311-314`): 403 `is_group_member` cho nhóm không tồn tại (`stranger_unknown_context`), nhóm thật (`stranger_real_context`), người được mời (`invitee_reads`), người đã rời (`mate_after_leaving`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header`). Role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200 (`owner_roles_group_admin_only`).
4. Miền tính toán ném `LedgerError` → 409 với `code` là mã miền **viết hoa nguyên văn** (vd. `BALANCES_DO_NOT_NET_TO_ZERO`) và detail `Confirmed ledger events cannot be balanced` (`service.py:1779-1782`). Không tới được qua HTTP: route ghi sổ đã chặn số âm, float và tổng lệch trước khi ghi.

## Đầu vào

- Path `context_id` (UUID lax).
- Không query, không body (`undeclared_query_ignored` → 200).

## Đầu ra

- **200** `ContextBalancesResponse` (`services/api/app/api/schemas.py:1160-1166`), thứ tự khoá `balances`, `transfers`, `proven_minimal`, `transfer_count`.
  - `balances[]`: `{"person_id","net_vnd"}` (`schemas.py:1147-1149`), sắp theo **byte của UUID** (`service.py:1787-1790`), không theo chuỗi. Dương là nhóm nợ người đó. Người có ròng 0 không xuất hiện (`ledger.py:256-258`).
  - `transfers[]`: `{"sender_id","recipient_id","amount_vnd"}` (`schemas.py:1152-1157`); trường `kind: "offset_proposal_draft"` của miền bị bỏ khi đưa lên wire. Thứ tự do `settlement_plan` (`ledger.py:352-395`): chia nhóm con tổng 0 nhiều nhất (quy hoạch động bitmask, `ledger.py:298-349`) khi có ≤ 15 người khác 0; trong mỗi nhóm con ghép tham lam con nợ và chủ nợ theo số tiền giảm dần rồi theo id dạng chuỗi (`ledger.py:261-295`). Trên 15 người: tham lam toàn cục, `proven_minimal: false`.
  - `proven_minimal`: bool; `transfer_count`: int ≥ 0.
  - Mọi số tiền là số nguyên đồng (`MoneyVnd` strict int, `schemas.py:26-27`). Không float, không datetime.
- Nhóm trống: `{"balances":[],"transfers":[],"proven_minimal":true,"transfer_count":0}` (`owner_empty_group`).
- Giá trị đo được trong kịch bản (dữ liệu mẫu):
  - bữa tối 200 000 do owner trả, chia owner + mate → owner +100 000, mate −100 000, một chuyển (`owner_after_dinner`);
  - thêm taxi 90 000 do mate trả, chia ba → owner +70 000, mate −40 000, third −30 000, hai chuyển về owner (`owner_after_taxi`);
  - thêm khách sạn 60 000 do third trả hộ owner + mate (third không nằm trong người chia) → owner +40 000, mate −70 000, third +30 000 (`third_after_hotel`);
  - xác nhận nhận 40 000 trên nghĩa vụ mate → owner: cặp còn 60 000 → owner 0 (biến mất), mate −30 000, third +30 000 (`owner_after_partial_receipt`);
  - xác nhận thêm 70 000 (tổng 110 000 > 100 000): cặp bị bỏ hẳn (`remaining <= 0`, không âm) → owner −60 000, mate +30 000, third +30 000 (`owner_after_over_receipt`).
  - hai khoản chi mới do owner trả (bữa sáng, bữa trưa, mỗi khoản mate nợ owner 10 000), mỗi khoản một đợt thu, rồi owner xác nhận **một** lần nhận `2**63 - 1` đồng trên **mỗi** nghĩa vụ: cả hai 201 `over_confirmed`. Tổng đã nhận của cặp mate → owner là 110 000 + 2·(2**63 − 1), vượt int64; cặp bị bỏ nên số dư không đổi, và owner, mate, third đọc cùng một câu trả lời (`owner_after_int64_receipts`, `mate_after_int64_receipts`, `third_after_int64_receipts`);
  - rồi mate tạo đợt thu cho taxi và xác nhận nhận 10 000 trên nghĩa vụ đầu tiên của đợt (một cặp khác, người nhận là mate): cặp đó còn 20 000 và số dư đổi theo (`owner_after_normal_receipt_other_pair`; lần đo thử bằng replay ra owner −50 000, mate +20 000, third +30 000, vì nghĩa vụ đầu tiên là owner → mate).
- Framework: 307 cho `/` cuối; `POST` → 405 `allow: GET`.

## Tác dụng phụ

- **Khoá dòng trong một GET**: `load_batch_inputs(context_id, None)` chọn phiên bản mới nhất của mỗi khoản chi với `FOR UPDATE OF expense_versions` (`services/api/app/api/repository.py:6204`), rồi với từng phiên bản `SELECT confirmed_allocations … ORDER BY participant_id FOR UPDATE` (`repository.py:6220`). Khoá giữ tới commit (commit trước response, `services/api/app/api/unit_of_work.py:51`). Một lần xác nhận khoản chi hay tạo đợt thu chạy song song trên cùng dòng sẽ chờ.
- `load_confirmed_receipts` (`repository.py:6270-6300`): tổng `receipt_confirmations.amount_vnd` theo `(sender, recipient)` của nghĩa vụ, chỉ tính khi `confirmed_by_id = recipient_id`, qua `collection_batch_versions` → `collection_batches.context_id`.
- Tổng vượt int64 tới được qua HTTP: `ReceiptConfirmationRequest.amount_vnd` là `PositiveMoneyVnd` không có trần (`services/api/app/api/schemas.py:27`, `:1979-1982`), cột `receipt_confirmations.amount_vnd` là `BIGINT` với CHECK `amount_vnd > 0` (`services/api/app/db/models.py:780`, `:796`). `SUM(bigint)` của Postgres trả `numeric`, nên `load_confirmed_receipts` không tràn và Python cộng/trừ bằng số nguyên không giới hạn (`ledger.py:249-255`).
- View `collection_obligation_progress` ép tổng đã nhận **của một nghĩa vụ** về `::bigint` (`services/api/app/db/migrations/versions/20260827_0001_initial_api_schema.py:621-646`). Khi một nghĩa vụ nhận quá `2**63 - 1`, mọi SELECT trên view báo `bigint out of range` (SQLSTATE 22003); không route nào đọc view này, nhưng làn DB của harness đọc mọi relation và dừng `INFRA`. Vì thế kịch bản chia hai lần nhận `2**63 - 1` ra hai nghĩa vụ của cùng một cặp.
- Phiên bản chưa xác nhận không có dòng `confirmed_allocations` nên không vào sổ (`owner_after_proposal_only`); đã vào đợt thu vẫn được tính (`owner_after_batch` không đổi).
- Người đã rời vẫn nằm trong số dư (`owner_after_mate_left`).
- Không ghi dòng nào (delta DB rỗng); không idempotency (GET); không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:1750-1754`, `:504` |
| 409 | mã `LedgerError` (vd. `SELF_OBLIGATION`, `NEGATIVE_AMOUNT`, `BALANCES_DO_NOT_NET_TO_ZERO`) | `Confirmed ledger events cannot be balanced` (không tới được) | `service.py:1779-1782`; `ledger.py:36-103`, `:219-258`, `:352-370` |

## Mã Python

- Route: `services/api/app/api/routes/contexts.py:118-128`
- Service: `services/api/app/api/service.py:1745-1801` (`get_context_balances`)
- Domain: `services/api/app/domain/ledger.py:63-103` (`obligations_from_allocations`), `:106-140` (`merge_obligations`), `:219-258` (`group_balances`), `:261-295`, `:298-349`, `:352-395` (`settlement_plan`); `services/api/app/domain/permissions.py:311-314`
- Repository: `services/api/app/api/repository.py:6171-6268` (`load_batch_inputs`), `:6270-6300` (`load_confirmed_receipts`), `:2769-2782`

## Test đang phủ

- `services/api/tests/api/test_context_balances.py`: `test_multiple_expenses_produce_stable_net_balances_and_minimal_plan` (43), `test_non_member_cannot_read_balances_even_with_forged_context_header` (78), `test_recipient_confirmed_receipt_reduces_balance` (92), `test_zero_net_group_has_no_balances_or_transfer_proposals` (126) — repository giả, không chạm SQL hay khoá
- Miền: `services/api/tests/domain/test_ledger.py`, `test_settlement_minimal.py`, `test_settlement_properties.py`; `services/api/tests/test_one_money_check.py`
- `services/api/tests/test_money_api_boundary_is_integer.py::test_every_money_field_refuses_a_fractional_value` (343)

## Kịch bản parity

`parity/scenarios/w3/contexts/GET-contexts-context_id-balances.yaml`, id `w3/contexts/get-contexts-context_id-balances` (59 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `actor_id_not_uuid`, `stranger_non_uuid_context`.
- 403: `stranger_unknown_context`, `stranger_real_context`, `invitee_reads`, `stranger_claims_context_header`, `mate_roles_empty`, `mate_after_leaving`.
- Sổ: `owner_empty_group`, `propose_dinner` + `owner_after_proposal_only`, `confirm_dinner` + `owner_after_dinner` + `mate_after_dinner`, `propose_taxi` + `confirm_taxi` + `owner_after_taxi`, `propose_hotel` + `confirm_hotel` + `third_after_hotel`.
- Xác nhận nhận tiền: `owner_creates_batch` + `owner_after_batch`, `owner_confirms_partial_receipt` + `owner_after_partial_receipt`, `owner_confirms_over_receipt` + `owner_after_over_receipt`. Khoá `idempotency_key` của hai lần xác nhận mượn id khoản chi do server sinh trong chính lượt chạy, vì khoá này duy nhất toàn bảng và một chuỗi cố định sẽ va ở lượt thứ hai.
- Tổng vượt int64: `propose_breakfast` + `confirm_breakfast`, `propose_lunch` + `confirm_lunch`, `owner_creates_breakfast_batch`, `owner_creates_lunch_batch`, `owner_confirms_int64_max_receipt`, `owner_confirms_int64_max_receipt_again` (khoá `idempotency_key` mượn id hai khoản chi mới; dòng `body_raw` mang `# repo-guard: allow=long-number reason=int64-receipt-sum-probe`), `owner_after_int64_receipts`, `mate_after_int64_receipts`, `third_after_int64_receipts`; rồi `mate_creates_taxi_batch`, `mate_confirms_taxi_receipt`, `owner_after_normal_receipt_other_pair`.
- Người rời: `mate_leaves`, `owner_after_mate_left`; role: `owner_roles_group_admin_only`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects` (307), `post_not_allowed` (405).

`prod` (`w3/contexts/prod-auth`): `basic_scheme_balances` (401), `propose_dinner` + `confirm_dinner` + `mate_reads_balances` (200).

## Chưa phủ / lưu ý cho bản Go

- Khoá `FOR UPDATE` trên một route đọc: harness chạy tuần tự nên không thấy việc chờ; bản Go bỏ khoá sẽ vẫn xanh parity nhưng đổi hành vi đồng thời với xác nhận khoản chi và tạo đợt thu.
- Thứ tự `balances` là byte của UUID (trùng thứ tự `uuid` của Postgres), còn phá hoà trong `transfers` là so chuỗi id; hai thứ tự khác nhau có chủ ý trong Python.
- Sửa khoản chi (phiên bản mới) chưa phủ: chỉ phiên bản có `version_number` lớn nhất được tính.
- Nhánh trên 15 người (`proven_minimal: false`) và 409 `LedgerError` chưa phủ qua HTTP; test miền phủ.
- Nghĩa vụ ngược chiều giữa hai người không được bù trừ ở mức cặp (`merge_obligations`), chỉ ròng ở mức người; kịch bản có cặp owner↔mate hai chiều qua bữa tối và taxi.
- **Tổng nhận vượt int64**: bản Go phải cộng tổng đã nhận của một cặp (và `total - receipts`) bằng số không tràn (vd. `numeric` từ SQL rồi `big.Int`, hoặc so sánh trước khi cộng), và cho ra đúng số dư Python; `int64` sẽ tràn ở `owner_after_int64_receipts`. Số dư trả ra vẫn nhỏ vì cặp bị bỏ khi `remaining <= 0`.
- Hai lần nhận `2**63 - 1` trên **cùng một** nghĩa vụ không có trong kịch bản: Python trả 201 cả hai, nhưng sau đó view `collection_obligation_progress` không đọc được và làn DB dừng `INFRA`. Chưa có test Python nào cho tổng vượt int64.
