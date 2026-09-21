# POST /expenses

expenses · core · trạng thái trong bộ nhớ: không có

## Mục đích

Đề xuất một khoản chi: chạy allocator để cho người dùng xem mỗi người phải bỏ ra bao nhiêu, và tạo **danh tính** khoản chi (dòng `expenses`) để lần xác nhận sau gắn phiên bản vào. **Không ghi gì vào sổ**: không phiên bản, không phân bổ (`services/api/app/api/service.py:6317-6334`). Allocator là bản ADR-0004 (`services/api/app/domain/allocator.py:274-303`): số nguyên đồng, `Fraction` cho giá trị trung gian, một điểm làm tròn duy nhất (phần dư lớn nhất).

## Xác thực và quyền

Thứ tự (theo mã; stack tham chiếu là mốc):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key` (`services/api/app/api/idempotency.py:404-553`): rỗng hoặc dài hơn 255 → 422 trước mọi thứ khác (`owner_empty_idempotency_key`).
2. JSON hỏng / body sai → 422 framework.
3. **Không có `get_actor`**: route không khai báo actor (`services/api/app/api/routes/expenses.py:30-40`). Ẩn danh được 201 ở dev (`anonymous_even_split_remainder`) và ở prod không cần bearer (`anonymous_proposes_without_bearer`), bearer rác cũng 201 (`junk_bearer_proposes`). `X-Actor-*` không được đọc, nên `X-Actor-ID` sai định dạng không gây 422. Không có bảng quyền nào được hỏi.
4. `allocate(_allocator_input(proposal))` → `AllocationError` → 422 với `code` là mã miền **viết hoa nguyên văn**, detail `Expense cannot be allocated` (`service.py:6318-6321`). Chạy **trước** khi tra context: nhóm không tồn tại + phân bổ lỗi → 422 (`owner_unknown_context_bad_split`).
5. `create_expense(context_id)` (`services/api/app/api/repository.py:6022-6041`): vi phạm `fk_expenses_context_id` → `RepositoryConflict("EXPENSE_CONTEXT_NOT_FOUND")` → 404 `context_not_found` (`service.py:6322-6329`, `owner_unknown_context_valid_split`). IntegrityError khác bị ném lại (500, không tới được).

Không kiểm membership của `participants`, `paid_by_id`, `recorded_by_id` ở bước này: id bất kỳ (kể cả không phải người) được chia tiền trong đề xuất; kiểm tra thành viên chỉ có ở `POST /expenses/{id}/confirm`.

## Đầu vào

Body `ExpenseInput` (`services/api/app/api/schemas.py:97-110`), `extra="forbid"` ở mọi mức (`schemas.py:66-67`):

- `context_id: UUID`; `description: StrictStr | null` (mặc định `null`); `recorded_by_id: UUID`; `paid_by_id: UUID` (advancer của allocator, `service.py:714`);
- `verification_scope: "totals_only" | "items_reviewed"`;
- `occurred_at: datetime` **bắt buộc có offset** (`schemas.py:70-73`, `:110`) → thiếu offset 422 `value_error` "Value error, datetime must include a UTC offset" (`owner_occurred_at_without_offset`);
- `participants: list[UUID]` (UUID lax: chữ hoa được parse về cùng một id, `owner_duplicate_participant_two_spellings` → `DUPLICATE_PARTICIPANT`);
- `total_amount_vnd: MoneyVnd` = `int` strict **không có trần** (`schemas.py:26`): float `1000.0`, chuỗi, bool → 422 `int_type` (`owner_total_as_float`, `owner_total_as_string`, `owner_item_amount_as_bool`); số nguyên vượt int64 qua được pydantic và đến allocator (`owner_total_past_int64`, `owner_total_below_int64`, `owner_item_past_int64`);
- `items: list[ExpenseItemInput]` (`schemas.py:76-80`: `item_id: StrictStr`, `label: StrictStr | null = null`, `amount_vnd`, `shared_by: list[UUID]`); `surcharges: list[ExpenseSurchargeInput]` (`:83-87`: `surcharge_id`, `kind: StrictStr`, `amount_vnd`, `mode: "proportional" | "even"`); `discounts: list[ExpenseDiscountInput]` (`:90-94`: `discount_id`, `amount_vnd`, `scope: "global_proportional" | "item"`, `item_id: StrictStr | null = null`). Ba danh sách mặc định `[]` (`owner_minimal_body_defaults`).
- Nhiều lỗi pydantic cùng lúc được liệt kê hết (`owner_unknown_scope_and_extra_item_key`: literal sai, uuid sai, khoá thừa trong item).

Allocator (`allocator.py:54-162`, thứ tự `services/api/app/domain/contract.py:23-48`; trần `MAX_AMOUNT_VND = 10**12`, id tối đa 64 byte UTF-8, `contract.py:12-13`), các nhánh tới được qua HTTP:

| code | điều kiện | bước |
|---|---|---|
| `NO_PARTICIPANTS` | `participants` rỗng | `owner_no_participants`, `owner_unknown_context_bad_split` |
| `DUPLICATE_PARTICIPANT` | trùng sau khi parse UUID | `owner_duplicate_participant_two_spellings` |
| `INVALID_ENTITY_ID` | id rỗng, có khoảng trắng đầu/cuối, > 64 byte | `owner_item_id_padded`, `owner_item_id_over_64_bytes` (33 chữ `ư` = 66 byte) |
| `DUPLICATE_ENTITY_ID` | trùng id trong cùng loại | `owner_duplicate_item_id` |
| `NEGATIVE_AMOUNT` | số âm (tổng hoặc dòng) | `owner_negative_total`, `owner_total_below_int64`, `owner_negative_total_and_zero_item` (âm thắng 0) |
| `ZERO_AMOUNT` | dòng item/phụ phí/giảm giá bằng 0 (tổng 0 hợp lệ) | `owner_zero_item_amount` |
| `AMOUNT_TOO_LARGE` | > 10^12 | `owner_total_over_ceiling`, `owner_total_past_int64`, `owner_item_past_int64` |
| `INVALID_KIND` | `kind` rỗng hoặc > 32 byte | `owner_empty_surcharge_kind`, `owner_surcharge_kind_over_32_bytes` |
| `SCOPE_TARGET_MISMATCH` | `item` thiếu `item_id` / `global_proportional` có `item_id` | `owner_item_discount_without_target`, `owner_global_discount_with_target` |
| `EMPTY_SHARED_BY` / `DUPLICATE_SHARED_BY` | | `owner_empty_shared_by`, `owner_duplicate_shared_by` |
| `UNKNOWN_PARTICIPANT` | `shared_by` ngoài `participants` (thắng đối soát) | `owner_unknown_participant_and_mismatch` |
| `UNKNOWN_ITEM` | giảm giá item trỏ item không có | `owner_unknown_item` |
| `DISCOUNT_EXCEEDS_ITEM` / `DISCOUNT_EXCEEDS_BASE` | (thắng đối soát) | `owner_discount_exceeds_item_and_mismatch`, `owner_discount_exceeds_base` |
| `RECONCILIATION_MISMATCH` | Σ item + Σ phụ phí − Σ giảm giá ≠ tổng | `owner_reconciliation_mismatch` |

Không tới được: `INVALID_PARTICIPANT_ID` (UUID luôn thành chuỗi hợp lệ), `AMOUNT_NOT_INTEGER` (strict int), `INVALID_MODE`, `INVALID_SCOPE` (Literal).

## Đầu ra

- **201** `ExpenseProposalResponse` (`schemas.py:120-123`), thứ tự khoá `expense_id`, `proposal`, `allocation`.
  - `expense_id`: id của dòng `expenses` vừa tạo.
  - `proposal`: `ExpenseInput` đã validate được pydantic ghi lại, **không phải byte gửi lên**: mặc định được điền (`description: null`, ba danh sách `[]`), UUID về dạng thường có gạch, `occurred_at` ghi lại theo pydantic (offset giữ nguyên như `+07:00`, `+00:00` thành `Z`, phần lẻ giây đệm đủ 6 chữ số).
  - `allocation` (`schemas.py:113-117`, `service.py:718-728`), khoá `allocations`, `exact_shares`, `rounding_gainers`, `warnings`:
    - `allocations`: `{participant: int}` theo **thứ tự `participants`**; Σ = `total_amount_vnd`;
    - `exact_shares`: `{participant: "tử/mẫu"}` phân số tối giản, số nguyên vẫn ghi `"/1"` (vd. `"50000/1"`);
    - `rounding_gainers`: người được thêm 1 đồng, xếp theo (phần dư giảm dần, advancer trước khi hoà, byte của id) (`allocator.py:254-271`);
    - `warnings`: tập con đã sắp và khử trùng của `advancer_not_participant`, `proportional_fallback_to_even`, `zero_share_participants` (`allocator.py:292-302`; `zero_share_participants` chỉ khi tổng > 0).
  - Không float.
- Nhánh allocator dựng trong kịch bản (dữ liệu mẫu): chia đều có dư (`anonymous_even_split_remainder`), tối thiểu 1 đồng (`owner_minimal_body_defaults`), advancer ngoài danh sách (`owner_advancer_outside_participants`), tổng 0 (`owner_zero_total`), phần 0 (`owner_items_zero_share`), đủ năm tầng với phần mười ba (`owner_items_surcharges_discounts`), phụ phí tỉ lệ không có cơ sở (`owner_proportional_fallback_to_even`), 16 người 1 000 đồng (`owner_sixteen_participants`), phần dư khác cỡ (`owner_uneven_remainders`), đúng trần 10^12 (`owner_total_at_ceiling`).
- **Replay idempotency**: 201 + body đã lưu + `idempotency-replayed: true`; đổi thứ tự khoá/khoảng trắng JSON vẫn replay (`owner_key_replay_reordered`), đảo thứ tự **mảng** `participants` là request khác → 422 (`owner_key_reused_participants_reversed`).
- Framework: `GET /expenses` → 405 `allow: POST`; `POST /expenses/` → 307.

## Tác dụng phụ

- INSERT một dòng `expenses (id, context_id, created_at)` (`repository.py:6022-6026`; model `services/api/app/db/models.py:266-289`). Không khoá dòng. Không `audit_events`.
- Commit trước response (`services/api/app/api/unit_of_work.py:51`, `deps.py:196-209`); lỗi 404/422 rollback nên không có dòng nào.
- Idempotency: scope là digest bearer nếu có header `Authorization: Bearer …`, không thì `X-Actor-ID` thô, không thì chuỗi `anonymous` (`idempotency.py:327-335`, `:447-449`). Người gọi ẩn danh **dùng chung một scope** (`anonymous_key_first`, `anonymous_key_replay`; kịch bản đưa id nhóm của lượt chạy vào khoá), cùng khoá dưới scope của owner là request mới (`owner_uses_anonymous_key`). Chỉ 2xx được lưu; 404 và 422 nhả khoá (`owner_key_allocation_refused` + `owner_key_after_allocation_refusal`, `owner_key_unknown_context_refused` + `owner_key_after_unknown_context_refusal`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | mã `AllocationError` viết hoa (bảng trên) | `Expense cannot be allocated` | `service.py:6318-6321`; `allocator.py:54-226` |
| 404 | `context_not_found` | `Context does not exist` | `service.py:6324-6328`; `repository.py:6035-6040` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |
| 422 | (framework) | `{"detail":[…]}` không có `input`: `json_invalid`, `missing`, `int_type`, `string_type`, `uuid_parsing`, `literal_error`, `value_error`, `extra_forbidden` | `services/api/app/api/main.py:318-351` |

Lỗi của middleware có khoảng trắng sau `:` và `,` (`json.dumps` mặc định, `idempotency.py:602-618`); lỗi route thì gọn.

## Mã Python

- Route: `services/api/app/api/routes/expenses.py:30-40`
- Service: `services/api/app/api/service.py:6317-6334` (`propose_expense`), `:682-715` (`_allocator_input`), `:718-728` (`_wire_allocation`)
- Domain: `services/api/app/domain/allocator.py:42-303`; `services/api/app/domain/contract.py:12-13`, `:23-54`; `services/api/app/domain/money.py`
- Repository: `services/api/app/api/repository.py:6022-6041` (`create_expense`)
- Middleware: `services/api/app/api/idempotency.py:404-553`

## Test đang phủ

- `services/api/tests/api/test_expenses.py`: `test_proposal_calls_allocator_but_does_not_write_the_ledger` (12), `test_malformed_wire_money_never_reaches_domain_or_storage` (22) — repository giả
- `services/api/tests/api/test_money_wire_type_gate.py`: `test_a_bool_total_would_have_become_a_one_dong_bill` (200), `test_a_float_total_never_reaches_the_domain_or_storage` (217)
- `services/api/tests/api/test_idempotency.py` dùng route này làm mẫu: `test_the_same_key_replays_the_first_response_and_writes_once` (111), `test_the_same_key_with_a_different_payload_is_refused` (126), `test_a_rejected_request_releases_the_key_so_a_real_retry_can_run` (171), `test_the_key_is_scoped_to_the_actor` (185), `test_the_key_is_scoped_to_the_bearer_when_one_is_sent` (200), `test_a_reordered_json_object_is_the_same_request` (615), `test_array_order_still_changes_the_fingerprint` (645), …
- `services/api/tests/postgres/test_expense_context_fk_postgres.py`: `test_ledger_refuses_an_expense_for_a_group_that_does_not_exist` (141), `test_the_repository_turns_that_refusal_into_a_conflict_not_a_crash` (164), `test_naming_a_group_that_does_not_exist_is_refused_in_words` (238), `test_an_expense_for_a_real_group_is_still_written` (275)
- `services/api/tests/postgres/test_idempotency_postgres.py`: `test_posting_the_same_expense_twice_with_one_key_writes_one_row` (363), `test_reusing_a_key_with_another_payload_writes_nothing` (544)
- Miền: `services/api/tests/domain/test_allocator_golden.py` (41 vector), `test_allocator_properties.py`, `test_allocator_rejects_non_integer_amounts.py`

## Kịch bản parity

`parity/scenarios/w4/expenses/POST-expenses.yaml`, id `w4/expenses/post-expenses` (55 bước), lane DB bật:

- Không actor: `anonymous_even_split_remainder` (201).
- Phân bổ 201: `owner_minimal_body_defaults`, `owner_advancer_outside_participants`, `owner_zero_total`, `owner_items_zero_share`, `owner_items_surcharges_discounts`, `owner_proportional_fallback_to_even`, `owner_sixteen_participants`, `owner_uneven_remainders`, `owner_total_at_ceiling` (dòng `body_raw` mang `# repo-guard: allow=long-number reason=allocator-ceiling-probe`).
- Allocator 422: các bước trong bảng ở mục Đầu vào; ba bước vượt int64 mang `reason=int64-overflow-probe`.
- Context: `owner_unknown_context_valid_split` (404), `owner_unknown_context_bad_split` (422 thắng 404).
- Pydantic: `owner_occurred_at_without_offset`, `owner_total_as_float`, `owner_total_as_string`, `owner_item_amount_as_bool`, `owner_unknown_scope_and_extra_item_key`.
- Idempotency: `owner_key_first`, `owner_key_replay_reordered`, `owner_key_reused_participants_reversed`, `owner_key_allocation_refused` + `owner_key_after_allocation_refusal`, `owner_key_unknown_context_refused` + `owner_key_after_unknown_context_refusal`, `anonymous_key_first` + `anonymous_key_replay`, `owner_uses_anonymous_key`, `owner_empty_idempotency_key`.
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307). Chuẩn bị: `register_owner`, `owner_creates_group`.

`prod` (`w4/expenses/prod-auth`, 21 bước, chung với confirm): `anonymous_proposes_without_bearer`, `junk_bearer_proposes` (201 không cần phiên), `anonymous_key_first` + `anonymous_key_replay` (scope `anonymous`), khoá theo digest bearer (`owner_key_first`, `owner_key_replay`, `stranger_same_key`, `owner_same_key_bearer_scope`).

Replay chéo (`w4/crossreplay/post-expenses`, 17 bước): Python lưu → core replay, cả khi đổi thứ tự khoá; core từ chối khi đảo mảng `participants`; core lưu → Python replay/từ chối (thêm một trường mặc định viết tường minh là request khác); từ chối của phía này nhả khoá cho phía kia; khoá của scope ẩn danh replay chéo.

Đồng thời (`w4/concurrency/post-expenses`, 6 bước): `same_key_burst` (một dòng, các bản còn lại chờ rồi replay), `distinct_key_burst` (mỗi bản một dòng), `same_key_other_body_burst` (mọi bản 422), `anonymous_same_key_burst`.

Corpus 422 sinh: route bị hoãn trong wave `w4` của `scripts/render_parity_422_scenarios.py` với lý do `'function-after' is not probed` (validator múi giờ của `occurred_at`); các bước pydantic ở trên thay thế một phần.

## Chưa phủ / lưu ý cho bản Go

- **Số tiền không có trần ở biên**: JSON integer vượt int64 phải qua được bước parse và trả `AMOUNT_TOO_LARGE`/`NEGATIVE_AMOUNT` như Python, không phải 422 lỗi JSON (`owner_total_past_int64`, `owner_total_below_int64`, `owner_item_past_int64`). Đối soát `Σ item + Σ phụ phí − Σ giảm giá` chạy sau kiểm trần nên các tổng đó luôn nhỏ; bản Go vẫn nên dùng `big.Rat`/`big.Int` như ADR-0029 §2.5.
- Thứ tự mã lỗi là thứ tự của ADR-0004, duyệt phần tử theo **byte của id**, không theo thứ tự gửi.
- `proposal` là bản pydantic ghi lại, không phải echo byte: định dạng `occurred_at` (Z, offset, 6 chữ số lẻ), UUID thường, mặc định được điền.
- Route không có actor ở cả hai chế độ: bất kỳ ai biết id nhóm đều tạo được dòng `expenses` trong nhóm đó, và 404 so với 201/422 cho biết id nhóm có tồn tại. Parity giữ nguyên (xem lỗi nghi vấn).
- Scope idempotency `anonymous` dùng chung giữa mọi người gọi không danh tính; khoá cố định sẽ va giữa các lượt chạy, nên kịch bản gắn id nhóm vào khoá.
- 409 in-flight và tiến trình chết giữa chừng không phủ.
