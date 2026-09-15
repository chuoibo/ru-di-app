# GET /contexts/{context_id}/budget

budget · core · trạng thái trong bộ nhớ: không có

## Mục đích

Nhận thức ngân sách của nhóm (F34): mỗi người đã tiêu bao nhiêu trên các chuyến **đang đi**, và một mức giá ứng viên (tuỳ chọn) so với **trung bình mỗi người** của các chuyến **đã kết thúc**. Mọi con số tính lại trên từng request từ sổ: phiên bản mới nhất đã xác nhận của từng khoản chi, gán vào chuyến theo **ngày lịch Việt Nam** của `occurred_at` (luật tiền số 3, `services/api/app/api/service.py:2595-2633`). Không đọc cột tổng nào; tái dùng đúng câu đọc của tường kỷ niệm (`group_recap`) để F30/F32/F34 cùng một cách hiểu sổ.

## Xác thực và quyền

Thứ tự (theo mã; stack tham chiếu là mốc):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) giải trước tham số path/query: ẩn danh → 401 kể cả khi path hoặc query hỏng (`anonymous_unknown_context`, `anonymous_non_uuid_context`, `anonymous_bad_candidate`); `X-Actor-ID` không phải UUID → 422 `invalid_actor_id` (`actor_id_not_uuid`).
2. Path `context_id: UUID` và query `candidate_per_person_vnd` → 422 gộp một lần (`stranger_non_uuid_context`, `stranger_non_uuid_context_bad_candidate`). Query hỏng thắng 403: người lạ + nhóm không tồn tại + query `x` → 422 (`stranger_unknown_context_bad_candidate`).
3. `_require_permission("view_group_budget", {"is_group_member": is_member(context_id, actor)})` (`service.py:2604-2608`; bảng `services/api/app/domain/permissions.py:413-416`: role `group_admin` hoặc `member`, predicate `is_group_member`; `is_member` đòi `state = active` và `left_at IS NULL`, `services/api/app/api/repository.py:2769-2782`). 403 `is_group_member`: nhóm không tồn tại (`stranger_unknown_context`), nhóm thật (`stranger_real_context`), người được mời chưa nhận (`invitee_reads`), người đã rời (`mate_after_leaving`), `X-Actor-Contexts` không được tin (`stranger_claims_context_header`). Role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 200 (`mate_roles_group_admin_only`).
4. Miền (`services/api/app/domain/budget.py`) có `BudgetError("INVALID_BUDGET_INPUT")` (`budget.py:27-36`) nhưng service **không bắt**: nếu nổ sẽ thành 500. Không tới được qua HTTP (repository chỉ trả số nguyên không âm, `headcount > 0` do CHECK `outings`).

## Đầu vào

- Path `context_id` (UUID lax).
- Query `candidate_per_person_vnd` tuỳ chọn (`services/api/app/api/routes/budget.py:19-35`, `:51-54`): `BeforeValidator(_parse_candidate_money)` đổi chuỗi **toàn chữ số ASCII** (`str.isascii() and str.isdigit()`) thành `int`, rồi `int` strict `ge=0`. Hệ quả:
  - `007` → 7 (`owner_candidate_leading_zeros`); `0` hợp lệ (`owner_candidate_zero`).
  - `-1`, `+1`, `1.5`, `1e2`, chuỗi rỗng, chữ số full-width `１２` → 422 `int_type` "Input should be a valid integer" (`owner_candidate_negative`, `owner_candidate_plus_sign`, `owner_candidate_empty`, `owner_candidate_exponent`, `owner_candidate_fullwidth_digits`). `-1` không tới được `ge` vì không phải chữ số, nên lỗi là `int_type`, không phải `greater_than_equal`.
  - **Không có trần**: `10^17` (18 chữ số) và `2^64` (20 chữ số) được nhận (`owner_candidate_scaled_past_int64`, `owner_candidate_past_int64`).
  - Key lặp: Starlette đưa **giá trị cuối** (`owner_candidate_repeated_last_accepted` → 200, `owner_candidate_repeated_last_refused` → 422).
- Query không khai báo bị bỏ qua (`owner_undeclared_query_ignored`). Không body.

## Đầu ra

- **200** `GroupBudgetResponse` (`services/api/app/api/schemas.py:1240-1258`), thứ tự khoá `context_id`, `outing_count`, `active_member_count`, `avg_per_person_vnd`, `in_progress`, `comparison`.
  - `outing_count`: số chuyến đã bắt đầu và **đã kết thúc** (`ends_on < today`).
  - `active_member_count`: số membership `state = active` trong `list_members` (lọc `left_at IS NULL`, `repository.py:2749-2767`; `service.py:2628-2630`). Người được mời không tính (`owner_after_history`); người rời làm giảm (`owner_after_mate_left`).
  - `avg_per_person_vnd`: `Σ split_total // Σ headcount` của chuyến đã kết thúc, **chia sàn** (`budget.py:102-115`, `:135`). `null` khi không có chuyến kết thúc (`owner_empty_group`); `0` (không phải `null`) khi có chuyến kết thúc mà chưa có tiền (`owner_trips_without_money`). Mẫu số là `headcount` khai báo của chuyến, không phải số người tham gia khoản chi.
  - `in_progress[]`: `BudgetOutingView` (`schemas.py:1215-1231`), khoá `outing_id`, `title`, `headcount`, `budget_per_person_vnd`, `spent_per_person_vnd`, `remaining_per_person_vnd`, `over_budget`. `spent = split_total // headcount` (0 nếu headcount 0), `remaining = budget − spent` (có thể âm), `over_budget = remaining < 0` (`budget.py:117-133`). `title` là tiêu đề đã strip (khi tạo, `schemas.py:467-473`, và lại ở `budget.py:50-59`). Thứ tự: `group_recap` sắp `ends_on DESC, id` (`repository.py:2895-2899`) — hai chuyến cùng `ends_on` rơi vào id ngẫu nhiên, nên kịch bản cho mỗi chuyến một ngày kết thúc riêng.
  - `comparison`: `null` khi không có ứng viên **hoặc** không có trung bình (`owner_empty_group_with_candidate`). Có cả hai → `BudgetComparison` (`schemas.py:1234-1237`) khoá `candidate_per_person_vnd`, `delta_vnd` (= ứng viên − trung bình, có thể âm), `verdict`: `nhu-thuong` khi `|delta| · 100 <= trung bình · 10` (biên **bao gồm**), còn lại `re-hon` nếu `delta < 0`, `cao-hon` nếu dương (`budget.py:69-82`).
  - Mọi số tiền là số nguyên đồng, không float, không datetime. Validator response (`schemas.py:1224-1231`, `:1248-1258`) kiểm lại `remaining`, `over_budget`, `delta`; lệch sẽ là 500 (không tới được).
- Các giá trị kịch bản được dựng để rơi vào (dữ liệu mẫu, suy từ mã, không phải số đo):
  - hai chuyến kết thúc 2020: Đà Lạt 01-01..01-03 (2 người) nhận bữa tối 300 001 và bữa khuya 10 000 lúc `2020-01-03T16:30:00Z` (23:30 giờ Việt Nam, vẫn ngày 03); khoản `2020-01-03T17:30:00Z` là 00:30 ngày 04 giờ Việt Nam nên **không** thuộc chuyến; bảo tàng 02-10..02-11 (3 người) nhận 90 000 → trung bình `400001 // 5 = 80000`;
  - biên phán quyết quanh 80 000: 88 000 và 72 000 → `nhu-thuong`, 88 001 → `cao-hon`, 71 999 → `re-hon` (`owner_candidate_upper_boundary`, `owner_candidate_above_boundary`, `owner_candidate_lower_boundary`, `owner_candidate_below_boundary`), 80 000 → delta 0 (`mate_candidate_equal_to_average`);
  - hai chuyến đang đi (kết thúc 2099-12-31 và 2098-12-31) và một chuyến chưa bắt đầu (2099-01-01), chuyến chưa bắt đầu không xuất hiện; khách sạn sửa từ 250 000 lên 260 000 chỉ tính bản 2 (`owner_after_hotel_first_version` so với `owner_after_history`); đề xuất chưa xác nhận và khoản chi của nhóm khác không tính;
  - trung bình 0: ứng viên 0 → `nhu-thuong`, ứng viên 1 → `cao-hon` (`owner_zero_average_candidate_zero`, `owner_zero_average_candidate_one`).
- Framework: 307 cho `/` cuối; `POST` → 405 `allow: GET`.

## Tác dụng phụ

- Không ghi dòng nào; không idempotency (GET); không limiter.
- Đọc: `is_member`; `group_recap(context_id, today)` (`repository.py:2880-2989`) gồm ba câu: các `outings` có `starts_on <= today`; tổng `confirmed_allocations.amount_vnd` của phiên bản mới nhất mỗi khoản chi trong nhóm, nối `outings` theo `timezone('Asia/Ho_Chi_Minh', occurred_at)::date BETWEEN starts_on AND ends_on` (`repository.py:120-125`, `:2908-2960`; `SUM` ra `numeric`, ép `int`); đếm `memories` theo ngày (không dùng ở đây); rồi `list_members` (một câu roster + một câu tên, `repository.py:2749-2767`).
- `today` là ngày của đồng hồ server đổi sang `Asia/Ho_Chi_Minh` (`service.py:2609`), không phải ngày của Postgres.
- Một khoản chi được gán vào **mọi** chuyến có khoảng ngày chứa nó; hai chuyến chồng nhau cùng tính một khoản (`owner_after_history`: chuyến 2020-01-01..2098-12-31 nhận mọi khoản trong kịch bản).
- Không khoá `FOR UPDATE` (khác `GET /contexts/{id}/balances`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 422 | (framework) | `{"detail":[…]}` không có `input`: `uuid_parsing` (path), `int_type` (query), gộp khi cả hai hỏng | `services/api/app/api/main.py:318-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:2604-2608`, `:504` |
| 500 | — | `BudgetError INVALID_BUDGET_INPUT` không được bắt (không tới được) | `budget.py:35-43`, `service.py:2616-2632` |

## Mã Python

- Route: `services/api/app/api/routes/budget.py:19-62`
- Service: `services/api/app/api/service.py:2595-2633` (`group_budget`), `:475-504` (`_require_permission`)
- Domain: `services/api/app/domain/budget.py:23-24`, `:39-66`, `:69-82` (`_comparison`), `:85-147` (`build_group_budget`); `services/api/app/domain/permissions.py:413-416`, `:654-678`
- Repository: `services/api/app/api/repository.py:120-125`, `:2749-2767`, `:2769-2782`, `:2880-2989` (`group_recap`)
- Schema: `services/api/app/api/schemas.py:1215-1258`

## Test đang phủ

- `services/api/tests/api/test_budget.py`: `test_group_budget_route_declares_no_request_body` (66), `test_group_budget_returns_history_live_ledger_spend_and_comparison` (74), `test_group_budget_without_history_refuses_to_invent_a_comparison` (129), `test_group_budget_without_candidate_omits_comparison` (149), `test_group_budget_requires_active_membership_before_reading_ledger` (169), `test_group_budget_rejects_an_invalid_candidate_query` (192), `test_budget_outing_schema_ties_remaining_to_over_budget` (200), `test_group_budget_response_money_fields_are_strict` (285) — repository giả
- `services/api/tests/postgres/test_group_budget_postgres.py::test_group_budget_reloads_the_latest_ledger_version_on_each_request` (31)
- Câu đọc chung: `services/api/tests/postgres/test_group_recap_postgres.py` (`test_a_supper_after_midnight_stays_on_its_vietnamese_day` 337, `test_correcting_a_bill_does_not_double_the_trip_total` 401, `test_the_last_day_of_a_trip_still_counts_as_being_on_it` 533, `test_a_trip_still_ahead_is_not_a_memory_yet` 656, `test_another_groups_dinner_never_lands_on_this_wall` 682, …)
- Miền: `services/api/tests/domain/test_budget.py` (`test_budget_uses_floor_division_for_every_per_person_figure` 84, `test_budget_comparison_has_an_inclusive_ten_percent_band` 126, `test_budget_zero_baseline_compares_without_dividing` 207, `test_budget_rejects_non_integer_or_negative_candidates` 254, …)
- `services/api/tests/test_money_api_boundary_is_integer.py::test_every_money_query_or_path_parameter_refuses_a_fractional_value` (381)

## Kịch bản parity

`parity/scenarios/w4/budget/GET-contexts-context_id-budget.yaml`, id `w4/budget/get-contexts-context_id-budget` (70 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `anonymous_bad_candidate` (401 thắng 422), `actor_id_not_uuid`, `stranger_non_uuid_context`, `stranger_non_uuid_context_bad_candidate`, `stranger_unknown_context`, `stranger_unknown_context_bad_candidate` (422 thắng 403).
- 403 và role: `stranger_real_context`, `invitee_reads`, `stranger_claims_context_header`, `mate_roles_empty`, `mate_after_leaving`; `mate_roles_group_admin_only` (200).
- Nhóm trống: `owner_empty_group`, `owner_empty_group_with_candidate`.
- Chuyến: `owner_creates_finished_trip` (tiêu đề có khoảng trắng hai đầu), `owner_creates_second_finished_trip`, `owner_creates_live_trip`, `owner_creates_long_live_trip`, `owner_creates_future_trip`; `owner_trips_without_money`, `owner_zero_average_candidate_zero`, `owner_zero_average_candidate_one`.
- Sổ: `owner_proposes_dinner` + `owner_confirms_dinner`, `owner_proposes_late_supper` + `owner_confirms_late_supper`, `owner_proposes_after_midnight` + `owner_confirms_after_midnight`, `mate_proposes_museum` + `mate_confirms_museum`, `owner_proposes_hotel` + `owner_confirms_hotel` + `owner_after_hotel_first_version` + `owner_confirms_hotel_edit`, `owner_proposes_unconfirmed`, `owner_creates_other_group` + `owner_proposes_other_group` + `owner_confirms_other_group`, `owner_after_history`.
- So sánh: `mate_candidate_equal_to_average`, bốn bước biên, `owner_candidate_zero`, `owner_candidate_leading_zeros`, `owner_candidate_scaled_past_int64`, `owner_candidate_past_int64` (hai dòng `path` mang `# repo-guard: allow=long-number reason=int64-overflow-probe`).
- Query hỏng: `owner_candidate_negative`, `owner_candidate_plus_sign`, `owner_candidate_empty`, `owner_candidate_exponent`, `owner_candidate_fullwidth_digits`, `owner_candidate_repeated_last_accepted`, `owner_candidate_repeated_last_refused`, `owner_undeclared_query_ignored`.
- Người rời: `mate_leaves`, `mate_after_leaving`, `owner_after_mate_left`.
- Framework: `trailing_slash_redirects` (307), `post_not_allowed` (405).
- Chuẩn bị: `register_*`, `owner_creates_group`, `owner_invites_mate`, `mate_accepts`, `owner_invites_invitee`.

`prod` (`w4/budget/prod-auth`, 18 bước): `anonymous_missing_bearer`, `junk_bearer`, `basic_scheme` (401), `actor_headers_ignored` (401), `owner_reads_empty_roles_header` (200: header role bị bỏ qua), `owner_non_uuid_context` (422), `mate_reads_with_candidate` (200, trung bình 50 000, ứng viên 60 000), `owner_lowercase_scheme_reads`, `stranger_reads_claiming_owner` (403), `stranger_bad_candidate` (422), `mate_leaves` + `mate_after_leaving` (403).

Corpus 422 sinh: route bị hoãn trong `scripts/render_parity_422_scenarios.py` (wave `w4`) với lý do `'function-after' is not probed` — `BeforeValidator` của query cho ra schema `function-after` mà bộ sinh chưa dò; các probe query ở trên thay cho nó.

## Chưa phủ / lưu ý cho bản Go

- **Số không có trần**: ứng viên là số nguyên tuỳ ý. `delta_vnd` vượt int64 (`owner_candidate_past_int64`) và `|delta| · 100` vượt int64 dù `delta` còn vừa (`owner_candidate_scaled_past_int64`, 10^17 · 100 = 10^19). Bản Go phải parse query thành `big.Int` (hoặc so sánh không tràn) và ghi `candidate_per_person_vnd`, `delta_vnd` ra JSON đúng từng chữ số. Parse bằng `strconv.ParseInt` sẽ 422 sai ở chỗ Python trả 200.
- Quy tắc chữ số là `isascii() and isdigit()` của Python: `007` hợp lệ, dấu `+`, khoảng trắng, full-width không hợp lệ, và lỗi là `int_type` chứ không phải `greater_than_equal` cho số âm.
- `today` theo đồng hồ server ở `Asia/Ho_Chi_Minh`; ngày của khoản chi theo `timezone('Asia/Ho_Chi_Minh', occurred_at)` trong SQL. Kịch bản dùng ngày 2020 và 2098/2099 để không phụ thuộc hôm nay; nhánh chuyến kết thúc **đúng hôm nay** (`ends_on = today` vẫn là đang đi) và nửa đêm giờ Việt Nam của `today` không phủ được bằng kịch bản tĩnh.
- Thứ tự `in_progress` hoà trên `ends_on` rơi vào `outings.id` (uuid4) — không so được giữa hai stack, kịch bản tránh.
- Chia sàn trên số không âm; bản Go dùng phép chia nguyên có dấu cũng ra cùng kết quả vì mọi toán hạng không âm, nhưng `remaining` và `delta` có thể âm.
- `BudgetError` không được bắt ở service: bản Go nên giữ 500 thay vì dịch sang 4xx nếu tái hiện kiểm tra miền.
- Người rời vẫn giữ phần chia trong sổ; chỉ `active_member_count` đổi.
