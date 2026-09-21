# POST /expenses/{expense_id}/confirm

expenses · core · trạng thái trong bộ nhớ: không có

## Mục đích

Xác nhận khoản chi vào sổ: ghi **một phiên bản bất biến** (`expense_versions`) cùng các dòng con và các dòng `confirmed_allocations` đúng bằng phân bổ người dùng đã xem. Xác nhận lại cùng một khoản chi tạo phiên bản `n+1`, không bao giờ ghi đè (`services/api/app/api/service.py:6373-6470`, `services/api/app/api/repository.py:6051-6169`). Phân bổ được tính lại ở server và phải **bằng** `expected_allocations` gửi lên; lệch là 409, không lặng lẽ ghi số khác.

## Xác thực và quyền

Thứ tự (theo mã; stack tham chiếu là mốc):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-553`).
2. JSON hỏng → 422 `json_invalid`, **trước** actor (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu → 401 (`anonymous_unknown_expense`; prod `anonymous_confirms`, `basic_scheme_confirms`, `junk_bearer_confirms`, header `X-Actor-*` bị bỏ qua `actor_headers_ignored_confirm`); `invalid_actor_id` (`actor_id_not_uuid`); `invalid_actor_roles` (`roles_unknown`).
4. Path `expense_id: UUID` và body → 422 gộp (`stranger_non_uuid_expense`).
5. `get_expense` khoá `expenses` `FOR UPDATE` (`repository.py:6043-6049`); không có → 404 `expense_not_found` (`service.py:6379-6381`, `stranger_unknown_expense`). **Trước quyền**.
6. `identity.context_id != proposal.context_id` → 409 `expense_context_mismatch` (`service.py:6382-6387`, `stranger_wrong_context`). **Trước quyền**: người ngoài biết được khoản chi có tồn tại và thuộc context nào (so bằng đoán).
7. `_require_permission("confirm_expense_proposal", {"is_group_member": is_member(identity.context_id, actor)})` (`service.py:6389-6397`; `services/api/app/domain/permissions.py:49`, role `member`). 403 `is_group_member`: người lạ (`stranger_right_context`), `X-Actor-Contexts` không được tin (`stranger_claims_context_header`), người được mời chưa nhận (`invitee_confirms`), người đã rời (`third_confirms_after_leaving`); prod `stranger_confirms_claiming_context`. Role rỗng → `role_not_permitted` (`mate_roles_empty`, `mate_key_refused_403`).
8. `_require_participants_are_members` trên `participants` + `paid_by_id` + `recorded_by_id` (`service.py:6336-6371`, `:6418-6425`): tập membership `state = active` từ `list_members` (`repository.py:2749-2767`); ai ngoài tập → 422 `participant_not_in_context`, detail `Not members of this group: ` + các id **khử trùng, sắp theo byte UUID**, nối `", "` (`owner_names_non_members`: id không ai, người được mời, người lạ lặp hai lần; `owner_paid_by_stranger`; `owner_recorded_by_stranger`; `owner_names_the_leaver`). Allocator chưa chạy.
9. `acknowledge_as_advancer: true` → `_require_permission("acknowledge_advancer_role", {"is_named_advancer": actor.id == paid_by_id})` (`service.py:6426-6433`; `permissions.py:53-56`, role `advancer`): không có role → `role_not_permitted` (`owner_acknowledges_without_advancer_role`), không phải người trả → `is_named_advancer` (`mate_acknowledges_owners_payment`, dev và prod; `expected_allocations` sai trong bước này cho thấy kiểm tra chạy trước allocator). Ở prod mọi phiên có role `advancer` (`repository.py:3629`).
10. `allocate` → 422 mã miền viết hoa, detail `Expense cannot be allocated` (`service.py:6435-6439`; `owner_empty_participants` → `NO_PARTICIPANTS`, `owner_allocation_refused` → `RECONCILIATION_MISMATCH`). Bảng mã đầy đủ: card `POST /expenses`.
11. `wire.allocations != expected_allocations` (so dict, không xét thứ tự) → 409 `proposal_changed` (`service.py:6440-6446`; `owner_expected_differs`, `owner_expected_extra_person` — thêm một người 0 đồng cũng lệch, `mate_key_refused_409`).
12. `save_expense_confirmation` ném `RepositoryConflict("EXPENSE_NOT_FOUND")` → 409 `expense_not_found` (chữ thường), detail `Expense confirmation conflicted` (`service.py:6448-6462`) — không tới được vì dòng đã khoá ở bước 5.

## Đầu vào

- Path `expense_id` (UUID lax).
- Body `ExpenseConfirmationRequest` (`services/api/app/api/schemas.py:126-129`), `extra="forbid"`:
  - `proposal: ExpenseInput` — cùng model với `POST /expenses` (card đó), gồm `occurred_at` bắt buộc offset và tiền strict int không trần;
  - `expected_allocations: dict[UUID, MoneyVnd]` — khoá UUID lax, giá trị strict int;
  - `acknowledge_as_advancer: StrictBool = false`.
  - `owner_expected_float` gộp ba lỗi pydantic: giá trị `100000.0`, khoá không phải UUID, `"true"` cho bool.
- `proposal` không phải là bản đã đề xuất: server không đọc lại đề xuất cũ, chỉ dùng body này; `POST /expenses` chỉ tạo danh tính.

## Đầu ra

- **201** `ExpenseConfirmationResponse` (`schemas.py:132-138`), thứ tự khoá `expense_id`, `expense_version_id`, `version_number`, `total_amount_vnd`, `payer_acknowledgement`, `allocations` (`service.py:6463-6470`).
  - `version_number`: `MAX(version_number) + 1` dưới khoá (`owner_confirms_pending` 1, `owner_confirms_edit_acknowledged` 2, `mate_confirms_version_three` 3).
  - `payer_acknowledgement`: `"acknowledged"` khi cờ bật và qua kiểm tra, còn lại `"pending"`.
  - `total_amount_vnd`: số trong `proposal`.
  - `allocations`: **chính `expected_allocations` của request, theo thứ tự khoá trong request** (UUID ghi dạng thường), không phải thứ tự `participants` (`owner_confirms_pending` gửi mate trước owner).
  - Không float, không datetime.
- Nhánh dựng trong kịch bản (dữ liệu mẫu): chỉnh sửa 200 000 → 240 000 → 250 001 (đồng lẻ về advancer); khoản lẩu 426 000 với item `lau` 300 000 (owner + mate), `bia` 90 000 (third), phụ phí VAT tỉ lệ 42 000, `service` đều 30 000, `shipping` đều 15 000, giảm giá item 30 000 và toàn bộ 21 000, kỳ vọng owner 157 875 / mate 157 875 / third 110 250 (`owner_confirms_hotpot_items`); phần 0 đồng (`mate_confirms_zero_share`); người trả không nằm trong người chia tự xác nhận (`third_confirms_advancer_outside`).
- **Replay idempotency**: 201 + body đã lưu + `idempotency-replayed: true`, không ghi phiên bản nữa (`owner_key_replay` với khoá JSON đảo thứ tự và khoảng trắng); body khác → 422 (`owner_key_reused_other_body`).
- Framework: `GET` → 405 `allow: POST`.

## Tác dụng phụ

- Khoá `expenses` `FOR UPDATE` hai lần trong cùng transaction (`repository.py:6043-6049`, `:6065-6069`), giữ tới commit trước response. Hai xác nhận song song cùng một khoản chi xếp hàng, nên số phiên bản không trùng (`uq_expense_versions_expense_version` không bị chạm).
- Đọc: `is_member` (`repository.py:2769-2782`), `list_members` + tên (`:2749-2767`), `SELECT MAX(version_number)`.
- INSERT (`repository.py:6071-6164`), mọi bảng dưới đây **chỉ thêm** (trigger `reject_*_mutation`, `services/api/app/db/migrations/versions/20260827_0001_initial_api_schema.py:666-688`, `:763-778`):
  - `expense_versions`: `version_number`, `previous_version_number` (NULL ở bản 1, CHECK `version_chain`), `description`, `recorded_by_id`, `paid_by_id`, `payer_acknowledgement`, `verification_scope`, `occurred_at`, `created_at = now` Python, và roll-up từ `component_rollups` (`services/api/app/domain/expense.py:8-52`): `subtotal` = Σ item (chia đều không item thì = tổng), `vat_amount_vnd` = phụ phí có `kind.casefold() == "vat"`, `shipping_amount_vnd` = `"shipping"`, `fee_amount_vnd` = mọi `kind` khác, `discount_amount_vnd` = Σ giảm giá; CHECK `total_components_match` (`services/api/app/db/models.py:320-324`).
  - `expense_items` (`item_key` = `item_id`, `label`, `amount_vnd`), `expense_item_shares` (một dòng mỗi người trong `shared_by`), `expense_surcharges`, `expense_discounts` (`target_item_id` = id dòng `expense_items` vừa tạo, hoặc NULL).
  - `confirmed_allocations`: một dòng mỗi khoá của `expected_allocations`, **sắp theo byte UUID**, kể cả 0 đồng, `confirmed_by_id = actor`, `confirmed_at = now` (`repository.py:6139-6150`).
  - `audit_events`: `expense_confirmed`, `aggregate_type = "expense"`, `event_data = {expense_version_id, version_number, allocator_warnings}` (`repository.py:6151-6164`).
- Commit trước response (`services/api/app/api/unit_of_work.py:51`); mọi 4xx rollback, không dòng nào.
- Idempotency: chỉ 2xx được lưu; 403/409/422 nhả khoá (`mate_key_refused_403` + `mate_key_after_403`, `mate_key_refused_409` + `mate_key_after_409`). Scope: digest bearer, rồi `X-Actor-ID`, rồi `anonymous` (`idempotency.py:447-449`); ở prod người khác dùng cùng khoá thì chạy thật và bị từ chối theo quyền của họ (`stranger_same_key`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 404 | `expense_not_found` | `Expense does not exist` | `service.py:6379-6381` |
| 409 | `expense_context_mismatch` | `Proposal context does not match the expense identity` | `service.py:6382-6387` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` / `is_named_advancer` | `service.py:6389-6397`, `:6426-6433`, `:504` |
| 422 | `participant_not_in_context` | `Not members of this group: <uuid>, <uuid>` | `service.py:6362-6371` |
| 422 | mã `AllocationError` viết hoa | `Expense cannot be allocated` | `service.py:6435-6439` |
| 409 | `proposal_changed` | `Confirmed allocations differ from the reviewed proposal` | `service.py:6441-6446` |
| 409 | `expense_not_found` (từ repository) | `Expense confirmation conflicted` (không tới được) | `service.py:6459-6462`; `repository.py:6068-6069` |
| 422 / 409 | `invalid_idempotency_key`, `idempotency_key_reuse`, `idempotency_request_in_flight` | xem card `POST /expenses` | `idempotency.py:432-493` |
| 422 | (framework) | `{"detail":[…]}` không có `input` | `services/api/app/api/main.py:318-351` |

## Mã Python

- Route: `services/api/app/api/routes/expenses.py:43-55`
- Service: `services/api/app/api/service.py:6373-6470` (`confirm_expense`), `:6336-6371` (`_require_participants_are_members`), `:682-715`, `:718-728`, `:475-504`
- Domain: `services/api/app/domain/allocator.py:274-303`; `services/api/app/domain/expense.py:8-52`; `services/api/app/domain/permissions.py:49`, `:53-56`, `:654-678`
- Repository: `services/api/app/api/repository.py:6043-6049`, `:6051-6169` (`save_expense_confirmation`), `:2749-2782`
- Model: `services/api/app/db/models.py:266-531` (`Expense`, `ExpenseVersion`, `ExpenseItem`, `ExpenseItemShare`, `ExpenseSurcharge`, `ExpenseDiscount`, `ConfirmedAllocation`)

## Test đang phủ

- `services/api/tests/api/test_expenses.py`: `test_confirm_writes_version_and_allocations_and_calls_central_permissions` (30), `test_confirm_rejects_unreviewed_allocation_change` (66), `test_confirm_permission_is_not_an_inline_role_check` (86) — repository giả
- `services/api/tests/api/test_expense_participants_must_be_members.py`: `test_confirm_refuses_a_participant_the_group_does_not_contain` (50), `test_confirm_names_every_stranger_at_once_not_just_the_first` (59), `test_confirm_still_accepts_an_expense_whose_participants_are_all_members` (76)
- `services/api/tests/postgres/test_expense_participant_membership_postgres.py`: `test_two_active_members_are_billable` (148), `test_an_invited_person_who_never_accepted_cannot_be_billed` (159), `test_a_person_who_left_the_group_cannot_be_billed` (174), `test_a_member_of_a_different_group_cannot_be_billed` (190), `test_nothing_is_written_when_a_participant_is_refused` (267)
- `services/api/tests/postgres/test_idempotency_postgres.py`: `test_confirming_the_same_expense_twice_with_one_key_writes_one_version` (590), `test_a_second_confirmation_under_its_own_key_still_writes_its_own_version` (622)
- `services/api/tests/postgres/test_repository_postgres.py::test_repository_lifecycle_reaches_confirmed_receipt` (333)
- Miền: `services/api/tests/domain/test_expense.py` (`test_pure_even_split_projects_total_to_subtotal` 8, `test_itemized_rollups_keep_vat_shipping_and_catch_all_fee_separate` 26), allocator golden

## Kịch bản parity

`parity/scenarios/w4/expenses/POST-expenses-expense_id-confirm.yaml`, id `w4/expenses/post-expenses-expense_id-confirm` (53 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_expense` (401), `anonymous_malformed_json` (422 trước 401), `actor_id_not_uuid`, `roles_unknown`, `stranger_non_uuid_expense`, `stranger_unknown_expense` (404), `stranger_wrong_context` (409 trước 403).
- 403: `stranger_right_context`, `stranger_claims_context_header`, `invitee_confirms`, `mate_roles_empty`, `mate_acknowledges_owners_payment`, `owner_acknowledges_without_advancer_role`, `third_confirms_after_leaving`.
- 422 thành viên: `owner_names_non_members`, `owner_paid_by_stranger`, `owner_recorded_by_stranger`, `owner_names_the_leaver`.
- 422 allocator / pydantic: `owner_empty_participants`, `owner_allocation_refused`, `owner_expected_float`.
- 409: `owner_expected_differs`, `owner_expected_extra_person`.
- Phiên bản: `owner_confirms_pending`, `owner_confirms_edit_acknowledged`, `mate_confirms_version_three`; dòng con: `owner_proposes_hotpot` + `owner_confirms_hotpot_items`; phần 0: `mate_proposes_coffee` + `mate_confirms_zero_share`; advancer ngoài: `third_proposes_hotel` + `third_confirms_advancer_outside`.
- Idempotency: `owner_key_first`, `owner_key_replay`, `owner_key_reused_other_body`, `mate_key_refused_403` + `mate_key_after_403`, `mate_key_refused_409` + `mate_key_after_409`.
- Người rời: `third_leaves`, `third_confirms_after_leaving`, `owner_names_the_leaver`. Framework: `get_not_allowed` (405).
- Chuẩn bị: `register_*`, `owner_creates_group`, mời/nhận mate và third, `owner_invites_invitee`, `owner_proposes_dinner`.

`prod` (`w4/expenses/prod-auth`, 21 bước): `anonymous_confirms`, `basic_scheme_confirms`, `junk_bearer_confirms`, `actor_headers_ignored_confirm` (401), `stranger_confirms_claiming_context` (403), `mate_acknowledges_owners_payment` (403 `is_named_advancer`), `owner_confirms_acknowledged_empty_roles_header` (201, header role bị bỏ qua), `mate_confirms_version_two` (201), khoá theo digest bearer (`owner_key_first`, `owner_key_replay`, `stranger_same_key`), `owner_revokes_session` + `owner_confirms_after_revoke` (401).

Replay chéo (`w4/crossreplay/post-expenses-expense_id-confirm`, 19 bước): phiên bản xen kẽ giữa hai bản cài đặt; khoá Python lưu được core replay (cả khi đổi thứ tự khoá JSON) và ngược lại; body khác bị từ chối ở phía kia; 409 `proposal_changed` hay 403 của phía này nhả khoá cho phía kia; `owner_same_key_other_scope`.

Đồng thời (`w4/concurrency/post-expenses-expense_id-confirm`, 9 bước): `same_key_burst` (3 bản, ghi phiên bản 1 một lần, còn lại replay), `same_key_other_body_burst` (2 bản, 422 `idempotency_key_reuse`), `mate_key_after_burst` (2 bản dưới khoá của mate, phiên bản 2 ghi một lần, bản kia replay).

Corpus 422 sinh: route bị hoãn trong wave `w4` với lý do `'function-after' is not probed` (validator múi giờ của `proposal.occurred_at`).

## Chưa phủ / lưu ý cho bản Go

- **Đánh số phiên bản dưới khoá khi hai xác nhận không khoá cùng chạy**: không phủ. Hai phiên bản của một khoản chi ghi trong một loạt cho các dòng `confirmed_allocations` giống hệt sau khi che id, và với khoá khác nhau thì bản nào nhận số 1 là lịch chạy; làn DB của harness không so được. Bản Go phải khoá `expenses` `FOR UPDATE` **trước** mọi kiểm tra như Python, rồi đọc `MAX(version_number)` dưới khoá.
- **Một người chia hai item trong cùng một lần xác nhận**: không phủ. Hai dòng `expense_item_shares` của cùng người trong một bước chỉ khác nhau ở `expense_item_id` mới sinh, harness đánh số khác nhau giữa hai stack (vì vậy khoản lẩu cho mỗi người đúng một item).
- 404 và 409 `expense_context_mismatch` chạy trước kiểm tra quyền: giữ nguyên thứ tự này (xem lỗi nghi vấn).
- `allocations` trả về theo thứ tự khoá của request, không theo `participants`; `confirmed_allocations` ghi theo byte UUID. Bản Go dùng `map` sẽ làm mất thứ tự trả về.
- Roll-up `fee/vat/shipping` phân loại theo `kind.casefold()`; `"VAT"` rơi vào `vat_amount_vnd`.
- `created_at` của phiên bản, `confirmed_at` và `audit_events.occurred_at` cùng một giá trị `now` Python; `expenses.created_at` là `now()` của Postgres ở lần đề xuất.
- Tiền vượt int64 trong `expected_allocations` hoặc `proposal` bị allocator/so sánh từ chối trước khi ghi (`AMOUNT_TOO_LARGE` hoặc `proposal_changed`); không phủ riêng ở route này.
- 409 in-flight và 409 `expense_not_found` từ repository không phủ.
