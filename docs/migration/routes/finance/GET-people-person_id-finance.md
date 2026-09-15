# GET /people/{person_id}/finance

finance · core · trạng thái trong bộ nhớ: không có

## Mục đích

Màn tài chính cá nhân: một người đã tiêu bao nhiêu (phần của **chính họ**), còn nợ bao nhiêu, người khác còn nợ họ bao nhiêu, và 20 lần nhận tiền đã được xác nhận gần nhất theo cả hai chiều. **Chỉ chính người đó đọc được**, không có ngoại lệ cho admin nhóm. Mọi con số tính lại trên từng request từ sổ (luật tiền số 3, `services/api/app/api/routes/finance.py:1-10`, `services/api/app/api/service.py:1333-1357`); không có cột số dư. Người chưa từng chia gì nhận số 0, không phải 404.

## Xác thực và quyền

Thứ tự (theo mã; stack tham chiếu là mốc):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_reads`, `anonymous_non_uuid_person`); `X-Actor-ID` không phải UUID → 422 `invalid_actor_id` (`actor_id_not_uuid`).
2. Path `person_id: UUID` → 422 (`newcomer_non_uuid_person`).
3. `actor.id != person_id` → 403 `not_your_finances` (`service.py:1349-1354`), **trước mọi câu đọc**: id của không ai (`newcomer_reads_unknown_person`), người khác trong cùng nhóm, kể cả admin (`owner_reads_mate`), người lạ (`stranger_reads_owner`). Không đi qua bảng quyền `permissions.py`: không có role nào được kiểm, `X-Actor-Roles` rỗng vẫn 200 (`owner_roles_empty_reads_self`); role lạ vẫn 422 ở bước 1.
4. Không kiểm membership, không kiểm người có đăng ký: người chưa có dòng `people` đọc chính mình → 200 (`newcomer_reads_self_unregistered`).

## Đầu vào

- Path `person_id` (UUID lax; so với `actor.id` sau khi parse, nên chữ hoa hay thiếu gạch vẫn là cùng người).
- Không query, không body (`undeclared_query_ignored` → 200).

## Đầu ra

- **200** `PersonFinanceResponse` (`services/api/app/api/schemas.py:383-403`), dựng lại từng trường trong route (`finance.py:47-70`), thứ tự khoá `person_id`, `display_name`, `spend_vnd`, `settled_vnd`, `outstanding_vnd`, `receivable_vnd`, `expense_count`, `group_count`, `movements`.
  - `display_name`: `people.display_name` (`session.get(Person)`, không lọc `deleted_at`); `null` khi không có dòng `people` (`repository.py:7091`, `:7263`).
  - `spend_vnd`: tổng `confirmed_allocations.amount_vnd` của người đó trên **phiên bản mới nhất** mỗi khoản chi, mọi nhóm (`repository.py:7098-7139`). Gồm cả phần của khoản mình tự trả.
  - `expense_count`: số khoản chi khác nhau có dòng phân bổ của người đó ở phiên bản mới nhất, kể cả phần 0 đồng (`repository.py:7140-7145`).
  - `group_count`: số membership `state = active` của người đó, mọi loại context (`repository.py:7146-7154`). Rời nhóm đặt `state = left` (`repository.py:2744-2745`) nên giảm (`mate_after_leaving`).
  - `outstanding_vnd = max(0, owed − paid)` (`repository.py:7172-7199`): `owed` = phần phân bổ (phiên bản mới nhất) trên khoản **người khác trả**, dù đã vào đợt thu hay chưa; `paid` = tổng `receipt_confirmations.amount_vnd` trên mọi nghĩa vụ mà người đó là **sender**. Báo đã chuyển (`payment_reports`) không tính.
  - `settled_vnd = spend − outstanding` (`repository.py:7267`), luôn ≥ 0.
  - `receivable_vnd = max(0, advanced − collected)` (`repository.py:7216-7259`): `advanced` = phân bổ (mới nhất) của **người khác** trên khoản người đó trả; `collected` = tổng biên nhận trên nghĩa vụ mà người đó là **recipient**. Không nằm trong đẳng thức `settled + outstanding = spend`.
  - `movements[]`: `FinanceMovementView` (`schemas.py:364-380`), khoá `obligation_id`, `direction`, `amount_vnd`, `counterparty_id`, `counterparty_name`, `context_id`, `context_name`, `occasion`, `occurred_at` (`repository.py:7275-7345`):
    - một phần tử cho mỗi dòng `receipt_confirmations` trên nghĩa vụ mà người đó là sender (`direction: "out"`) hoặc recipient (`"in"`), sắp `confirmed_at DESC, receipt_confirmations.id`, **LIMIT 20** (`service.py:1331`, `:1355-1357`);
    - `amount_vnd` là số của biên nhận (không phải của nghĩa vụ), có thể tới `2**63 - 1`;
    - `counterparty_name`: `people.display_name` của phía kia hoặc `null`;
    - `context_name`: `contexts.display_name` qua OUTER JOIN;
    - `occasion` (`repository.py:7347-7375`): mô tả của các phiên bản khoản chi nguồn (qua `collection_obligation_sources`), sắp `occurred_at`, bỏ mô tả rỗng/null, khử trùng lặp giữ thứ tự; một mô tả → nguyên văn; nhiều → `"<mô tả đầu> +<n-1>"`; không có → `null`;
    - `occurred_at` = `receipt_confirmations.confirmed_at` đọc từ DB: `timestamptz` UTC, pydantic ghi `Z`, 6 chữ số lẻ (bỏ phần lẻ khi micro giây bằng 0).
  - Mọi số tiền là số nguyên đồng (`MoneyVnd` strict int). Các tổng SQL (`SUM(bigint)` ra `numeric`) được ép `int` Python, nên `paid`/`collected` vượt int64 vẫn đúng; kết quả trả ra đã kẹp nên nhỏ.
- Giá trị kịch bản dựng để rơi vào (dữ liệu mẫu, **suy từ mã, chưa đo**): bữa tối 200 000 sửa thành 240 000 (chỉ bản 2 tính), taxi 60 000 do mate trả, món vặt 60 000 không mô tả, bữa sáng và bữa trưa 20 000, mọi khoản chia đôi owner/mate, mỗi khoản một đợt thu.
  - `owner_after_expenses`: owner spend 200 000, outstanding 30 000, settled 170 000, receivable 170 000, 5 khoản, 1 nhóm, không movement; `mate_after_expenses`: spend 200 000, outstanding 170 000, settled 30 000, receivable 30 000.
  - `owner_after_first_receipt`: biên nhận 30 000 trên nghĩa vụ món vặt → movement `in`, `occasion: null`.
  - `mate_over_confirms_taxi` (40 000 trên nghĩa vụ 30 000): owner outstanding kẹp về 0, mate receivable kẹp về 0.
  - 21 biên nhận 1 000 trên nghĩa vụ bữa tối: owner có 23 movement, trả 20 (`owner_after_twenty_one_receipts`), `occasion` `"Bữa tối (dữ liệu mẫu)"`.
  - Hai biên nhận `2**63 - 1` trên hai nghĩa vụ mate → owner (`owner_after_int64_receipts`, `mate_after_int64_receipts`): `collected` của owner và `paid` của mate vượt int64, receivable/outstanding kẹp về 0; hai movement đầu mang `amount_vnd` = `2**63 - 1`.
- Framework: 307 cho `/` cuối; `POST` → 405 `allow: GET`.

## Tác dụng phụ

- Không ghi dòng nào; không idempotency (GET); không limiter; không khoá.
- Đọc: `people` (1), bảy câu tổng hợp (`spend`, `expense_count`, `group_count`, `owed`, `paid`, `advanced`, `collected`), một câu movements, rồi **mỗi movement hai câu nữa** (`session.get(Person)` cho phía kia, `_obligation_occasion`) — N+1, tối đa 40 câu thêm.
- Không lọc theo nhóm, theo đợt thu hay theo trạng thái đợt thu (đợt `frozen` chưa công bố vẫn tính).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 422 | (framework) | `{"detail":[…]}` không có `input`, `uuid_parsing` cho path | `services/api/app/api/main.py:318-351` |
| 403 | `not_your_finances` | `A finance summary is readable only by the person it describes` | `service.py:1349-1354` |

## Mã Python

- Route: `services/api/app/api/routes/finance.py:32-70`
- Service: `services/api/app/api/service.py:1331-1357` (`person_finance_summary`)
- Repository: `services/api/app/api/repository.py:7079-7273` (`person_finance_summary`), `:7275-7345` (`_finance_movements`), `:7347-7375` (`_obligation_occasion`); bản ghi `FinanceMovement` `:989`, `PersonFinanceSummary` `:1019`
- Schema: `services/api/app/api/schemas.py:364-403`

## Test đang phủ

- `services/api/tests/api/test_finance.py`: `test_a_person_reads_their_own_finances` (54), `test_nobody_reads_somebody_elses_finances` (72), `test_a_person_who_has_split_nothing_reads_zero_rather_than_404` (95), `test_the_direction_of_money_survives_the_response` (116), `test_the_route_never_recomputes_what_the_ledger_answered` (138) — repository giả
- `services/api/tests/postgres/test_person_finance_postgres.py`: `test_a_person_with_no_ledger_rows_reads_as_zero_not_as_missing` (211), `test_confirming_a_split_makes_the_share_owed_immediately` (227), `test_confirming_more_than_was_owed_never_makes_the_debt_negative` (325), `test_confirming_more_than_was_owed_never_makes_the_receivable_negative` (438), `test_correcting_an_expense_does_not_double_what_the_payer_is_owed` (451), `test_every_money_figure_arrives_as_a_python_int` (465), `test_a_confirmed_receipt_clears_the_debt_and_appears_as_an_outgoing_movement` (496), `test_the_same_settlement_is_an_incoming_movement_for_the_recipient` (525), `test_correcting_an_expense_does_not_count_both_versions` (540), `test_the_two_figures_under_the_total_always_add_back_up_to_it` (564), …
- `services/api/tests/api/test_money_response_type_gate.py::test_a_float_spend_is_refused_by_the_real_response_model` (186)

## Kịch bản parity

`parity/scenarios/w4/finance/GET-people-person_id-finance.yaml`, id `w4/finance/get-people-person_id-finance` (79 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_reads`, `anonymous_non_uuid_person` (401), `actor_id_not_uuid`, `newcomer_non_uuid_person` (422), `newcomer_reads_unknown_person` (403).
- 403 người khác: `owner_reads_mate`, `stranger_reads_owner`.
- Số 0: `newcomer_reads_self_unregistered` (`display_name: null`), `owner_reads_self_empty`, `third_after_expenses`, `third_after_int64_receipts`; không kiểm role: `owner_roles_empty_reads_self`.
- Sổ: `owner_proposes_dinner` + `owner_confirms_dinner` + `owner_confirms_dinner_edit` (phiên bản 2), `mate_proposes_taxi` + `mate_confirms_taxi`, `owner_proposes_snack` + `owner_confirms_snack` (không mô tả), `owner_proposes_breakfast` + `owner_confirms_breakfast`, `owner_proposes_lunch` + `owner_confirms_lunch`, `third_proposes_unconfirmed` (đề xuất không tính); `owner_after_expenses`, `mate_after_expenses`.
- Đợt thu (không công bố): `owner_batches_dinner`, `mate_batches_taxi`, `owner_batches_snack`, `owner_batches_breakfast`, `owner_batches_lunch`, `mate_after_batches`.
- Biên nhận: `owner_confirms_snack_receipt` + `owner_after_first_receipt` + `mate_after_first_receipt`; `mate_over_confirms_taxi` + `owner_after_over_receipt` + `mate_after_over_receipt`; `owner_confirms_dinner_receipt_01` … `owner_confirms_dinner_receipt_21` + `owner_after_twenty_one_receipts` + `mate_after_twenty_one_receipts`. `idempotency_key` trong body là tiền tố 23 ký tự của id nhóm do server sinh (`kp`, bind bằng regex) cộng đuôi hex cố định, vì khoá này duy nhất toàn bảng.
- Tổng vượt int64: `owner_confirms_breakfast_int64_max`, `owner_confirms_lunch_int64_max` (hai dòng `body_raw` mang `# repo-guard: allow=long-number reason=int64-receipt-sum-probe`), `owner_after_int64_receipts`, `mate_after_int64_receipts`.
- Người rời: `mate_leaves`, `mate_after_leaving`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects` (307), `post_not_allowed` (405).

`prod` (`w4/finance/prod-auth`, 18 bước): `anonymous_missing_bearer`, `basic_scheme`, `junk_bearer` (401), `owner_reads_self_with_mate_actor_header` (200: header bị bỏ qua), `owner_reads_mate_claiming_mate` (403), `stranger_reads_self` (200, tên do harness seed), `stranger_non_uuid_person` (422), `owner_confirms_receipt` + `mate_reads_self` (movement `out`), `owner_lowercase_scheme_reads_self`, `owner_revokes_session` + `owner_after_revoke` (401).

Corpus 422 sinh: `parity/scenarios/generated/w4-422/get-people-person_id-finance.yaml`, id `generated/w4-422/get-people-person_id-finance` (18 bước: biến thể UUID của path, input không khai báo, thứ tự actor trước path).

## Chưa phủ / lưu ý cho bản Go

- **Tổng vượt int64**: `paid` và `collected` là tổng biên nhận không có trần của nhiều nghĩa vụ; bản Go phải đọc `SUM` dạng `numeric` (hoặc cộng `big.Int`) rồi kẹp, không được tràn trước khi trừ. `owner_after_int64_receipts` đỏ nếu dùng `int64`.
- Hai biên nhận `2**63 - 1` trên **cùng một** nghĩa vụ không có trong kịch bản: view `collection_obligation_progress` ép tổng mỗi nghĩa vụ về `bigint` và làn DB của harness dừng `INFRA`.
- `outstanding` gộp mọi chủ nợ: trả dư cho người này che khoản còn nợ người khác (`paid` không theo cặp). Tương tự `receivable` gộp mọi con nợ. Parity giữ nguyên.
- `owed`/`advanced` đọc phiên bản mới nhất, còn `paid`/`collected` đọc biên nhận trên nghĩa vụ đã đóng băng từ phiên bản cũ hơn nếu khoản chi bị sửa sau khi vào đợt thu; hai vế có thể lệch nhau. Chưa phủ.
- Nhánh `occasion` `"<mô tả đầu> +N"` **chưa phủ**: nó cần một nghĩa vụ gộp từ hai phiên bản khoản chi, và `POST /batches` trả `source_expense_version_ids` sắp theo chuỗi uuid4 ngẫu nhiên (`services/api/app/domain/ledger.py:135-137`), nên thứ tự danh sách khác nhau giữa hai stack so với cách harness đánh số `<uuid#n>`. Cần mở rộng harness trước khi phủ.
- Làn DB của harness: các dòng mới cùng bảng trong một bước mà giống nhau sau khi che uuid4 và thời điểm được xếp theo id ngẫu nhiên; kịch bản chỉ tạo mỗi đợt thu một nguồn, và các dòng phân bổ khác nhau ở `participant_id` (uuid v5 của persona, không bị che).
- Thứ tự movement: `confirmed_at DESC, id ASC`; hoà `confirmed_at` rơi vào uuid ngẫu nhiên (chỉ xảy ra khi biên nhận đồng thời) — không phủ.
- `context_name: null` không tới được: chú thích ở `repository.py:7301-7307` nói `collection_batches.context_id` không có FK, nhưng model đã có `fk_collection_batches_context_id` (`services/api/app/db/models.py:544`) và `contexts.display_name` là NOT NULL. `counterparty_name: null` cần một nghĩa vụ với người không có dòng `people`, không dựng được qua HTTP hiện tại.
- Nhánh biên nhận gắn `payment_report_id` và báo đã chuyển của khách cần link khách (token ngẫu nhiên không bind được) — không phủ; mã nói báo cáo không làm đổi số nào.
- Không pagination: người có hơn 20 biên nhận không xem được bản cũ hơn.
