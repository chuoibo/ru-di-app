# GET /contexts/{context_id}/heatmap

social_map · core · trạng thái trong bộ nhớ: không có

## Mục đích

F44: nhóm hay tụ ở khu vực nào, trả bằng quận và số lần, không kèm người, không kèm thời điểm. Toạ độ trả về là tâm khu vực, không phải toạ độ quán.

## Xác thực và quyền

Giống `GET /contexts/{context_id}/map`, khác tên action:

1. `get_actor` (`services/api/app/api/deps.py:110-164`), chạy trước validate path.
2. Path `context_id: UUID` → 422.
3. `is_member` (`services/api/app/api/repository.py:2769-2782`) rồi `_require_permission("view_group_heatmap")` (`services/api/app/api/service.py:5048-5052`). Role `group_admin` hoặc `member` (`services/api/app/domain/permissions.py:399-402`): thiếu → 403 `role_not_permitted`; không phải thành viên đang hoạt động → 403 `is_group_member`.
- Nhóm không tồn tại → 403, không 404.

## Đầu vào

Path `context_id` (UUID lax). Không query, không body.

## Đầu ra

- 200 `GroupHeatmapResponse` (`services/api/app/api/schemas.py:2221-2235`), thứ tự khoá: `context_id`, `areas`, `resolved_checkins`, `unknown_area_count`, `scanned_checkins`, `truncated`.
- `areas[]` (`HeatmapArea`, `schemas.py:2212-2218`): `id`, `label`, `lat`, `lng`, `visit_count`, `share_percent`.
  - Mỗi check-in được gán khu vực gần nhất trong bán kính 25 km bằng haversine (`services/api/app/places/areas.py:103`, `:129-161`). Duyệt `AREAS` theo id tăng dần với so sánh `<` chặt, nên khi hoà khoảng cách thì id nhỏ nhất thắng.
  - Xếp `(-visit_count, id)` (`services/api/app/places/social_map.py:149-151`).
  - `share_percent = visit_count * 100 // resolved_total` (`social_map.py:152-158`), chia sàn nên tổng thường **nhỏ hơn 100** (kịch bản: 42+14+14+14+14 = 98; 33+33+11+11+11 = 99). Nhánh `0` khi `resolved_total == 0` không tới được, vì khi đó `areas` rỗng.
  - `lat`, `lng` là tâm khu vực lấy từ `AREAS` (`areas.py:75-97`), ví dụ `10.799`, `106.68`.
- `resolved_checkins`: tổng `visit_count` (`service.py:5058`). `unknown_area_count`: số check-in ngoài mọi bán kính (`social_map.py:162-170`).
- `scanned_checkins`, `truncated`: như card map (`service.py:4943-4986`, trang 100, trần 500).
- Float: `lat`, `lng`. Không có datetime.
- Framework: 307 cho `/` cuối; 405 cho method khác.

## Tác dụng phụ

Chỉ đọc (`memories` theo trang, `memberships`). Không idempotency, không limiter.

## Lỗi

| Status | code | detail | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |
| 422 | (framework) | `uuid_parsing` trên `["path","context_id"]` | FastAPI |

## Mã Python

- Route: `services/api/app/api/routes/social_map.py:80-97`
- Service: `services/api/app/api/service.py:5043-5062` (`get_group_heatmap`), `:4943-4986` (`_scan_checkins`)
- Places: `services/api/app/places/social_map.py:116-170` (`_resolve`, `heatmap_rows`, `unknown_area_count`), `services/api/app/places/areas.py:75-203`
- Repository: `services/api/app/api/repository.py:5167-5205`, `:2769-2782`

## Test đang phủ

- `services/api/tests/postgres/test_social_map_postgres.py`: `test_an_outsider_cannot_read_the_heatmap_and_a_member_can` (188), `test_another_groups_visits_never_appear_on_this_heatmap` (289), `test_the_heatmap_answer_carries_no_person_and_no_time` (331), `test_the_heatmap_reports_a_centroid_not_the_venue` (356), cùng ba ca chung cho cả ba route ở dòng 579, 609, 634
- `services/api/tests/places/test_social_map.py` (129-170): gom theo quận, phần trăm, lịch sử rỗng, check-in ngoài mọi quận, thứ tự ổn định khi hoà
- `services/api/tests/places/test_areas.py` (25-156): haversine, bán kính, mọi place seed đều thuộc một khu vực

## Kịch bản parity

`parity/scenarios/w1/social_map/GET-contexts-context_id-heatmap.yaml`, id `w1/social_map/get-heatmap` (35 bước):

- Xác thực, dạng id, quyền: như card map (`anonymous_*`, `stranger_*`, `owner_roles_empty`, `mate_invited_not_yet_active`, `mate_after_leaving`).
- Dữ liệu: `owner_empty_heatmap` (mọi số 0, `truncated` false); `owner_heatmap_one_row` (100%); `owner_heatmap_seven` (5 khu vực, 42/14, tổng 98, hoà `visit_count` 1 xếp theo id); `owner_heatmap_tie_on_count` và `mate_heatmap_tie_on_count` (hoà 3–3 giữa `da-lat` và `hcm-quan-4`, 33/11).
- Framework: `trailing_slash_redirects`.

## Chưa phủ / lưu ý cho bản Go

- `prod` chưa phủ.
- Harness: `cursor` trong câu trả lời check-in dựng dữ liệu là base64(`created_at|id`), không chuẩn hoá được; kịch bản bind nó với `class: token`.
- `unknown_area_count > 0`: mọi place seed đều nằm trong 25 km của một khu vực, và check-in chỉ nhận `place_id` từ catalogue (`service.py:4791-4795`), nên không tạo được qua HTTP với dữ liệu seed.
- `truncated`: cần hơn 500 check-in, harness chưa có `repeat`.
- Haversine: `math.asin`, `math.sqrt`, `math.radians` của Python và `math` của Go cho cùng bit trên x86-64 với các phép này, nhưng biên 25 km so bằng `<` chặt. Một điểm cách đúng 25 km phải cho cùng kết quả; đừng đổi sang `<=`.
- `share_percent` là chia sàn số nguyên, **không** làm tròn.
- `lat`/`lng` là tâm khu vực, không phải toạ độ snapshot của check-in.
