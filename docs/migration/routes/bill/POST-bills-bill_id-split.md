# POST /bills/{bill_id}/split

bills · core · trạng thái trong bộ nhớ: không có

(Thư mục là `bill/` chứ không phải `bills/`: repo guard chặn thành phần đường dẫn `bills`, `scripts/repo_guard.py:99-114`.)

## Mục đích

Chiếu một bản nháp hoá đơn lên đầu vào allocator ADR-0004 và trả bản xem trước phần của từng người, chia giữa **roster ACTIVE** của nhóm, không phải giữa những người có share. Không ghi gì (ngoài hàng idempotency khi có khoá). `for_ledger: true` là cổng: chỉ qua khi mọi share đã được người xác nhận; ghi sổ thật vẫn là `POST /expenses/{expense_id}/confirm` (`services/api/app/api/routes/bills.py:95-106`, `services/api/app/api/service.py:6200-6315`).

## Xác thực và quyền

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`), xem card `POST /bills`. Route là POST nên preview 200 được lưu và phát lại.
2. Giải mã JSON → 422 trước actor (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401 (`anonymous_unknown_bill`), 422 `invalid_actor_id` (`actor_id_not_uuid`).
4. Path + body (pydantic) → 422 (`stranger_non_uuid_bill`, `owner_invalid_split_body`).
5. `_bill_for_actor` (`service.py:5919-5928`): 404 `bill_not_found` trước quyền (`stranger_unknown_bill`); 403 `confirm_expense_proposal` / `is_group_member` (`stranger_real_bill`, `invitee_splits`, `mate_splits_after_leaving`) hoặc `role_not_permitted` (`mate_roles_empty`).
6. Roster một lần (`service.py:6226-6243`, `services/api/app/api/repository.py:2749-2767`, lọc `left_at IS NULL`): `participant_ids` = hàng `state == active`, `excluded_member_ids` = hàng còn lại (người được mời chưa nhận), cả hai sắp theo byte UUID.
7. Phép chiếu `allocator_input_from_bill` (`services/api/app/domain/bill.py:59-134`) → `BillError` → 422 `Bill cannot be projected` (`service.py:6294-6295`): `BILL_HAS_NO_ITEMS` (`bill.py:67-71`, `owner_splits_bill_without_lines`), `ITEM_HAS_NO_ASSIGNEE` (`bill.py:73-77`, `owner_splits_unassigned_for_ledger` — trước cổng `for_ledger`). `INVALID_SHARE_SOURCE` (`bill.py:79-84`) không tới được (enum DB).
8. Cổng: `for_ledger` và còn share gợi ý → 422 `bill_assignments_not_confirmed` (`service.py:6297-6302`, `owner_for_ledger_with_suggestions`).
9. `allocate` (`services/api/app/domain/allocator.py:274-303`) → `AllocationError` → 422 với mã miền viết hoa, detail `Bill cannot be allocated` (`service.py:6304-6307`), theo thứ tự `services/api/app/domain/contract.py:23-48`.

## Đầu vào

- Path `bill_id` (UUID lax).
- Body `BillSplitRequest` (`services/api/app/api/schemas.py:205-207`), bắt buộc là một object (FastAPI coi body là bắt buộc): `for_ledger: bool` strict, mặc định `false`; `paid_by_id: UUID | null`, mặc định `null`; `extra="forbid"`.
- `paid_by_id` **không** được kiểm với roster: người lạ vẫn là advancer (`mate_splits_paid_by_stranger`).
- Đầu vào allocator do phép chiếu dựng (`service.py:6246-6293`): `participants` = roster ACTIVE theo byte; mỗi dòng thành `item_id = item_key`, `amount_vnd = line_total_vnd`, `shared_by` = người có share (theo thứ tự share của `_bill_record`); phụ phí/giảm giá chuyển nguyên (`discount.target_item_key` thành `item_id`); `advancer_id = str(paid_by_id)` hoặc `None`.
- Tổng (`bill.py:98-112`): `printed_total_vnd` nếu có, không thì **tổng dòng + tổng phụ phí − tổng giảm giá** bằng số nguyên Python.

## Đầu ra

- **200** `BillSplitResponse` (`schemas.py:256-290`), thứ tự khoá: `allocation`, `assignment_state`, `suggested_item_keys`, `total_amount_vnd`, `participant_ids`, `excluded_member_ids`.
  - `allocation` (`AllocationProposal`, `schemas.py:113-117`; `_wire_allocation` `service.py:718-728`): `allocations` (UUID → số nguyên đồng, thứ tự khoá = roster theo byte, người không ăn gì có `0`), `exact_shares` (cùng thứ tự, chuỗi `"tử/mẫu"` của `Fraction` đã rút gọn, số nguyên vẫn có `"/1"`; `allocator.py:300`), `rounding_gainers` (theo phần dư giảm dần, rồi advancer, rồi byte của id; `allocator.py:254-271`), `warnings` (sắp chữ cái, bỏ trùng; `advancer_not_participant` `:292-294`, `zero_share_participants` `:295-296`, `proportional_fallback_to_even` `:233-238`).
  - `assignment_state`: `"ai_suggested"` nếu còn bất kỳ share gợi ý nào, không thì `"confirmed"` (`bill.py:130-132`) — khác cách `_wire_bill` tính.
  - `suggested_item_keys`: theo byte UTF-8 (`bill.py:86-93`).
  - `total_amount_vnd`: tổng của phép chiếu (`service.py:6312`); luôn ≤ `10**12` khi 200 vì allocator từ chối trên trần.
  - `participant_ids` / `excluded_member_ids`: UUID theo byte; validator của schema đòi `participant_ids` trùng tập khoá `allocations` và không giao `excluded_member_ids` (`schemas.py:280-290`), luôn đúng theo cách dựng.
- Giá trị theo kịch bản (mô tả, không đo): người được mời vào nhóm sau đó có mặt trong `participant_ids` với phần 0 (`owner_previews_after_invitee_joined`); người đã rời không còn trong roster nên share của họ làm allocator trả `UNKNOWN_PARTICIPANT` (`owner_splits_after_mate_left`).
- Framework: `GET /bills/{id}/split` → 405 `allow: POST` (`get_not_allowed`).

## Tác dụng phụ

- Không ghi, không khoá: `session.get(Bill)` và các SELECT của `_bill_record` (`repository.py:5895-5897`, `:2444-2523`), `list_members` (một câu cho roster, một câu cho tên, `repository.py:2749-2767`), `is_member`.
- Có header `Idempotency-Key` và 2xx: hàng `idempotency_keys` giữ nguyên preview; phát lại trả preview cũ dù share hay roster đã đổi.
- Delta DB rỗng khi không có khoá.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` · prod: `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /bills` | `deps.py:144-163` |
| 422 | (không có `code`) | `{"detail":[…]}` của pydantic, bỏ `input` | `services/api/app/api/main.py:318-351` |
| 422 / 409 | `invalid_idempotency_key` / `idempotency_key_reuse` / `idempotency_request_in_flight` | xem card `POST /bills` | `idempotency.py:432-493` |
| 404 | `bill_not_found` | `Bill does not exist` | `service.py:5920-5922` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5923-5927`, `:504` |
| 422 | `BILL_HAS_NO_ITEMS` / `ITEM_HAS_NO_ASSIGNEE` | `Bill cannot be projected` | `bill.py:67-77`; `service.py:6294-6295` |
| 422 | `bill_assignments_not_confirmed` | `Bill assignments must be confirmed before ledger use` | `service.py:6297-6302` |
| 422 | `INVALID_ENTITY_ID`, `NEGATIVE_AMOUNT`, `AMOUNT_TOO_LARGE`, `INVALID_KIND`, `UNKNOWN_PARTICIPANT`, `UNKNOWN_ITEM`, `DISCOUNT_EXCEEDS_ITEM`, `DISCOUNT_EXCEEDS_BASE`, `RECONCILIATION_MISMATCH` | `Bill cannot be allocated` | `allocator.py:54-141`, `:144-162`, `:170-226`; `service.py:6304-6307` |

Mã allocator không tới được từ bill đã lưu: `NO_PARTICIPANTS` (người gọi luôn là thành viên ACTIVE), `INVALID_PARTICIPANT_ID`, `DUPLICATE_PARTICIPANT`, `DUPLICATE_ENTITY_ID` và `DUPLICATE_SHARED_BY` (ràng buộc UNIQUE), `AMOUNT_NOT_INTEGER`, `ZERO_AMOUNT` (CHECK `> 0`), `INVALID_MODE`/`INVALID_SCOPE`/`SCOPE_TARGET_MISMATCH` (enum và CHECK), `EMPTY_SHARED_BY` (phép chiếu từ chối trước).

## Mã Python

- Route: `services/api/app/api/routes/bills.py:95-106`
- Service: `services/api/app/api/service.py:6200-6315` (`split_bill`), `:5919-5928`, `:718-728` (`_wire_allocation`)
- Domain: `services/api/app/domain/bill.py:59-134` (`allocator_input_from_bill`); `services/api/app/domain/allocator.py:274-303` (`allocate`), `:54-162`, `:170-271`; `services/api/app/domain/contract.py:12` (`MAX_AMOUNT_VND = 10**12`), `:23-48`
- Repository: `services/api/app/api/repository.py:5895-5897`, `:2444-2523`, `:2749-2767`
- Schema: `services/api/app/api/schemas.py:113-117`, `:205-207`, `:256-290`

## Test đang phủ

- `services/api/tests/api/test_bills.py`: `test_split_calls_the_frozen_allocator` (238), `test_split_charges_each_person_for_the_dish_they_ate` (263), `test_the_split_sums_to_the_printed_total` (275), `test_a_preview_still_reports_which_lines_are_only_guesses` (290), `test_a_bill_whose_lines_miss_the_printed_total_is_refused` (300), `test_a_bill_with_no_lines_is_not_quietly_split_evenly` (316), `test_a_suggested_assignment_may_not_be_taken_to_the_ledger` (328), `test_one_unconfirmed_line_is_enough_to_hold_the_whole_bill_back` (343), `test_a_confirmed_bill_passes_the_gate` (362), `test_an_outsider_cannot_split_another_groups_bill` (417)
- `services/api/tests/api/test_split_declares_its_participants.py`: 87, 111, 132, 158, 179; `test_split_does_not_invent_participants.py`: 67, 86; `test_bills_surcharges.py`: 125, 147, 157, 217, 230; `test_bill_items_total_matches_lines.py::test_the_split_screen_and_the_bill_screen_report_the_same_meal` (180)
- Postgres thật: `tests/postgres/test_split_declares_its_participants_postgres.py` (169, 180, 194), `test_bill_surcharges_postgres.py` (217), `test_bill_self_claim_postgres.py` (343, 377, 402)
- Miền: `services/api/tests/domain/test_bill_projection.py` (52–409), golden allocator `tests/domain/golden/*.json`
- Không có test cho tổng phép chiếu vượt int64 hay `item_key` có khoảng trắng.

## Kịch bản parity

`parity/scenarios/w4/bill/POST-bills-bill_id-split.yaml`, id `w4/bill/post-bills-bill_id-split` (63 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_bill`, `anonymous_malformed_json`, `actor_id_not_uuid`, `stranger_non_uuid_bill`, `stranger_unknown_bill`.
- 403: `stranger_real_bill`, `invitee_splits`, `mate_roles_empty`, `mate_splits_after_leaving`.
- Xem trước và cổng: `owner_previews_suggested_bill` (người được mời trong `excluded_member_ids`), `owner_for_ledger_with_suggestions` (422), `owner_confirms_pho_and_nem` + `owner_confirms_tra`, `owner_splits_for_ledger` (200), `mate_splits_paid_by_stranger` (cảnh báo `advancer_not_participant`), `third_splits_paid_by_null`, `owner_splits_odd_bill_paid_by_mate` (dư 2 đồng, advancer thắng hoà).
- 422 phép chiếu: `owner_splits_bill_without_lines`, `owner_splits_unassigned_for_ledger`.
- 422 allocator: `owner_splits_printed_mismatch` (`RECONCILIATION_MISMATCH`), `owner_splits_discount_target_missing` (`UNKNOWN_ITEM`), `owner_splits_item_discount_over_line` (`DISCOUNT_EXCEEDS_ITEM`, tổng in 0), `owner_splits_global_discount_over_base` (`DISCOUNT_EXCEEDS_BASE`, tổng in 0), `owner_splits_negative_total` (`NEGATIVE_AMOUNT`, không tổng in), `owner_splits_empty_surcharge_kind` (`INVALID_KIND`), `owner_splits_padded_item_key` (`INVALID_ENTITY_ID`), `owner_splits_over_ceiling` (`AMOUNT_TOO_LARGE`), `owner_splits_total_past_int64` (dòng `2**62` và `2**62 − 1`, phụ phí `2**62`, không tổng in: tổng vượt int64 → `AMOUNT_TOO_LARGE`), `owner_splits_after_mate_left` (`UNKNOWN_PARTICIPANT`). Mỗi ca có bước `owner_creates_bill_*` tương ứng.
- Roster đổi: `invitee_accepts` + `owner_previews_after_invitee_joined`, `mate_leaves`.
- Pydantic: `owner_invalid_split_body`.
- Idempotency: `owner_key_first`, `owner_key_replay_spaced` (`{ }` = `{}`), `owner_key_reused_explicit_default` (`{"for_ledger":false}` là body khác), `owner_key_refused_gate` + `owner_key_after_refusal`.
- Framework: `get_not_allowed` (405).

`prod` (`w4/bill/prod-auth`): `basic_scheme_splits` (401), `stranger_splits` (403), `owner_splits_for_ledger` (200), `idem_first` + `idem_replay` + `stranger_same_key` (khoá theo digest bearer).

Replay chéo `parity/scenarios/w4/crossreplay/POST-bills-bill_id-split.yaml` (`w4/crossreplay/post-bills-bill_id-split`, 21 bước): preview được phát lại nguyên như lúc tính sau khi share đổi; `{"paid_by_id":null}` khác `{}`; 422 cổng và 403 nhả khoá.

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/post-bills-bill_id-split.yaml` (60 bước).

## Chưa phủ / lưu ý cho bản Go

- **Tổng vượt int64**: khi không có tổng in, bản Go phải cộng dòng + phụ phí − giảm giá bằng số không tràn. Cộng `int64` sẽ quay âm và allocator trả `NEGATIVE_AMOUNT` (nhóm cấu trúc kiểm âm trước trần) thay vì `AMOUNT_TOO_LARGE` (`owner_splits_total_past_int64`). Mọi phép so với `MAX_AMOUNT_VND` cũng phải an toàn tràn.
- `paid_by_id` không bị kiểm roster; cảnh báo và thứ tự nhận đồng dư phụ thuộc id tuỳ ý người gọi gửi.
- Phát lại idempotency trả preview cũ; bản Go phải lưu và phát lại byte đã gửi, không tính lại.
- `exact_shares` giữ `"/1"`; thứ tự khoá `allocations`/`exact_shares` theo roster byte; `warnings` sắp.
- `allocator_input_from_bill` truyền `shared_by` theo thứ tự share của `_bill_record` (theo `participant_id`), không theo thứ tự gán.
- `assignment_state` của split khác của `_wire_bill` với món không share (ở đây là lỗi chiếu).
- Roster đọc không khoá; một lần rời nhóm song song có thể đổi kết quả; harness chạy tuần tự.
- Khoảng trống harness: không bước nào ở đây ghi nhiều share của cùng một người trong một lần (xem card `PUT …/assignments`); kịch bản gán từng phần qua hai bước.
