# GET /contexts/{context_id}/recap

recap · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tường kỷ niệm của nhóm: các chuyến đã kết thúc (mới nhất trước) và, tách riêng, chuyến đang đi. Mỗi chuyến kèm tổng chi tính lại từ sổ theo ngày lịch Việt Nam, số khoản chi và số kỷ niệm.

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`): `prod` Bearer (401 `Missing bearer session` / `Session is not valid`); `dev` thiếu `X-Actor-ID` → 401, header sai → 422. Chạy trước validate path.
2. Path `context_id: UUID` → 422 `uuid_parsing`.
3. `is_member` (active, `left_at IS NULL`, `services/api/app/api/repository.py:2769-2782`) rồi `_require_permission("view_group_memories")` (`services/api/app/api/service.py:2568-2572`). Dùng chung quyền với tường ảnh; role `group_admin` hoặc `member` (`services/api/app/domain/permissions.py:427-430`). Thiếu role → 403 `role_not_permitted`; không phải thành viên đang hoạt động → 403 `is_group_member`.
- Nhóm không tồn tại → 403 `is_group_member`, không 404. Người được mời chưa nhận, và người đã rời, đều 403.

## Đầu vào

Path `context_id` (UUID lax: chữ hoa, 32 hex, `{}`, `urn:uuid:`). Query bị bỏ qua (`owner_query_ignored`). Không body.

## Đầu ra

- 200 `GroupRecapResponse` (`services/api/app/api/schemas.py:1193-1212`), thứ tự khoá: `context_id`, `outings`, `in_progress`, `split_total_vnd`.
- `outings[]` và `in_progress[]` cùng dạng `RecapOutingResponse` (`schemas.py:1169-1190`; `service.py:904-922`), thứ tự khoá: `outing_id`, `title`, `starts_on`, `ends_on`, `headcount`, `stops`, `split_total_vnd`, `expense_count`, `memory_count`.
  - `stops[]`: `OutingStopResponse` (`schemas.py:584-594`) lấy qua `_wire_outing` (`service.py:864`). Chuyến vừa tạo có `[]`.
- Chọn chuyến: `starts_on <= today`, **`ORDER BY ends_on DESC, id`** (`repository.py:2895-2900`). `in_progress = ends_on >= today` (`repository.py:2983`). Service tách hai danh sách mà giữ nguyên thứ tự (`service.py:2578-2586`). Chuyến chưa bắt đầu không xuất hiện ở đâu.
- `today = _now().astimezone(ZoneInfo("Asia/Ho_Chi_Minh")).date()`: đồng hồ Python của server (`service.py:406-407`, `:2576`; zone ở `repository.py:120`).
- Tiền (`repository.py:2904-2960`):
  - Chỉ **phiên bản mới nhất** của mỗi khoản chi (`MAX(version_number)`) có `confirmed_allocations`. Khoản mới đề xuất chưa xác nhận không được đếm.
  - Ngày của khoản chi là `timezone('Asia/Ho_Chi_Minh', occurred_at)::date`, tính trong DB (`repository.py:123-125`). Vì vậy `2021-04-30T17:30:00Z` thuộc ngày `2021-05-01`.
  - Chuyến nhận mọi khoản có ngày `BETWEEN starts_on AND ends_on`, gồm cả hai đầu. Các chuyến chồng ngày đều nhận cùng một khoản.
  - `split_total_vnd = int(COALESCE(SUM(amount_vnd), 0))`; `expense_count = COUNT(DISTINCT expense_id)`.
- `memory_count` (`repository.py:2961-2979`): số `memories` của nhóm (**cả ảnh lẫn check-in**) có ngày Việt Nam của `created_at` nằm trong chuyến. `created_at` của check-in là `_now()` Python lúc ghi.
- `split_total_vnd` ở gốc = tổng của `outings` thôi, không cộng `in_progress` (`service.py:2587-2593`).
- `starts_on`, `ends_on` là chuỗi ngày `YYYY-MM-DD`. Không có float, không có datetime.
- Framework: 307 cho `/` cuối; 405 cho method khác.

## Tác dụng phụ

Chỉ đọc: SELECT `outings`, `expense_versions`, `confirmed_allocations`, `expenses`, `memories`, `memberships`. Không idempotency, không limiter.

## Lỗi

| Status | code | detail | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |
| 422 | (framework) | `uuid_parsing` trên `["path","context_id"]` | FastAPI |

## Mã Python

- Route: `services/api/app/api/routes/recap.py:40-54`
- Service: `services/api/app/api/service.py:2552-2593` (`group_recap`), `:904-922` (`_wire_recap_outing`), `:864` (`_wire_outing`)
- Repository: `services/api/app/api/repository.py:2880-2989` (`group_recap`), `:123-125` (`_wall_clock_date`), `:2769-2782` (`is_member`)
- Domain: `services/api/app/domain/permissions.py:427-430`

## Test đang phủ

- `services/api/tests/postgres/test_group_recap_postgres.py`: `test_a_finished_trip_reports_where_it_went_and_what_it_cost` (235), `test_every_recap_money_figure_arrives_as_a_python_int` (283), `test_a_supper_after_midnight_stays_on_its_vietnamese_day` (337), `test_spending_after_everyone_went_home_is_not_claimed_by_the_trip` (372), `test_correcting_a_bill_does_not_double_the_trip_total` (401), `test_a_trip_the_group_is_still_on_reports_what_it_has_cost_so_far` (440), `test_a_finished_trip_reads_exactly_as_it_did_before_in_progress_existed` (488), `test_the_last_day_of_a_trip_still_counts_as_being_on_it` (533), `test_a_dinner_before_the_live_trip_began_is_not_charged_to_it` (561), `test_only_an_active_member_sees_what_the_live_trip_has_cost` (599), `test_a_trip_still_ahead_is_not_a_memory_yet` (656), `test_another_groups_dinner_never_lands_on_this_wall` (682), `test_photos_posted_during_the_trip_are_counted_on_it` (717), `test_a_non_member_cannot_read_the_wall` (746), `test_a_member_who_left_can_no_longer_read_the_wall` (764), `test_an_invited_member_cannot_read_the_wall_yet` (813)
- `services/api/tests/postgres/test_group_intelligence_postgres.py::test_album_photo_count_equals_the_recap_memory_count` (774)
- Fake repository của route khác cũng cài `group_recap` (`services/api/tests/api/test_budget.py:52`, `test_trip_reel.py:196`, `test_companion_rate_limit.py:203`), nhưng không đi qua route này.

## Kịch bản parity

`parity/scenarios/w1/recap/GET-contexts-context_id-recap.yaml`, id `w1/recap/get-recap` (44 bước):

- Xác thực và dạng id: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401); `stranger_non_uuid_context`, `stranger_short_uuid` (422); `stranger_unknown_context` và bốn dạng UUID lạ (403).
- Quyền: `owner_roles_empty`, `stranger_real_group`, `mate_invited_not_yet_active`, `mate_after_leaving` (403).
- Rỗng: `owner_empty_recap`, `owner_query_ignored`, `owner_recap_future_trip_only` (chuyến tương lai không hiện).
- Thứ tự và phân loại: `owner_recap_trips_no_money` (hai chuyến đã xong xếp `ends_on` giảm dần, một chuyến đang đi, mọi số 0).
- Tiền: `owner_recap_with_money` (bữa ăn khuya 17:30Z tính vào ngày hôm sau; khoản trước mọi chuyến không được đếm; khoản chưa xác nhận không được đếm; chuyến đang đi chồng ngày nhận ba khoản; tổng gốc chỉ cộng chuyến đã xong), `owner_recap_after_correction` (phiên bản 2 thay phiên bản 1, không cộng dồn).
- Kỷ niệm: `checkin_today` rồi `owner_recap_final` và `mate_recap_final` (`memory_count` = 1 ở chuyến đang đi).
- Framework: `trailing_slash_redirects`.

## Chưa phủ / lưu ý cho bản Go

- `prod` chưa phủ.
- Harness: `cursor` của bước `checkin_today` là base64(`created_at|id`), không chuẩn hoá được; kịch bản bind nó với `class: token`.
- **Hoà `ends_on`** được phá bằng `outings.id`, là UUID ngẫu nhiên, nên hai stack có thể xếp khác nhau. Kịch bản cố ý tránh hoà. Bản Go phải dùng cùng `ORDER BY ends_on DESC, id` (so sánh UUID theo byte của PostgreSQL).
- Chuyến có `stops` (lịch trình), ảnh (memory kind `photo`) trong `memory_count`: chưa có kịch bản.
- Ranh giới nửa đêm giờ Việt Nam: `today` lấy từ đồng hồ Python, còn ngày của khoản chi và kỷ niệm tính trong PostgreSQL. Bản Go phải quy đổi cùng múi `Asia/Ho_Chi_Minh` ở cả hai chỗ. Kịch bản dùng ngày 2020/2021/2099 nên không chạm biên.
- Tiền là số nguyên: `SUM` của PostgreSQL trả `numeric`; Python ép `int` (`repository.py:2941-2944`). Go phải quét vào `int64`, tuyệt đối không ra `520000.0`.
- Nhóm lớn: không có phân trang, không có giới hạn số chuyến.
