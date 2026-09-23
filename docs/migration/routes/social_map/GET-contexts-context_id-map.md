# GET /contexts/{context_id}/map

social_map · core · trạng thái trong bộ nhớ: không có

## Mục đích

F43: bản đồ của nhóm gồm những nơi nhóm đã check-in (có đếm), những nơi catalogue gắn cờ `hot`, tối đa 8 gợi ý xếp theo khẩu vị của các thành viên đang hoạt động, và lớp `saved` được khai là chưa có.

## Xác thực và quyền

Thứ tự:

1. `get_actor` (`services/api/app/api/deps.py:110-164`): `prod` Bearer (401 `Missing bearer session` / `Session is not valid`); `dev` thiếu `X-Actor-ID` → 401, header sai → 422. Chạy trước validate path (ẩn danh + id không phải UUID → 401).
2. Validate path `context_id: UUID` → 422.
3. `is_member` (active và `left_at IS NULL`, `services/api/app/api/repository.py:2769-2782`) rồi `_require_permission("view_social_map")` (`services/api/app/api/service.py:4996-5000`). Role `group_admin` hoặc `member` (`services/api/app/domain/permissions.py:395-398`): thiếu → 403 `role_not_permitted`; không phải thành viên → 403 `is_group_member`.
- Nhóm không tồn tại → 403 `is_group_member` (không 404). Người được mời chưa nhận, và người đã rời, đều 403.

## Đầu vào

- Path `context_id`: UUID lax (chữ hoa, 32 hex, `{}`, `urn:uuid:` đều nhận).
- Query bị bỏ qua; không body.

## Đầu ra

- 200 `SocialMapResponse` (`services/api/app/api/schemas.py:2194-2209`), thứ tự khoá: `context_id`, `visited`, `trending`, `recommended`, `unavailable`, `scanned_checkins`, `truncated`.
- `visited[]` (`VisitedPlace`, `schemas.py:2167-2179`): `place_id`, `place_name`, `lat`, `lng`, `visit_count`.
  - Gộp theo `place_id` (`services/api/app/places/social_map.py:56-90`). `place_name`, `lat`, `lng` lấy từ **check-in đầu tiên gặp**, tức check-in **mới nhất** vì quét theo `created_at DESC, id DESC` (`repository.py:5186`). Đây là snapshot trên dòng `memories`, không phải catalogue hiện tại.
  - Xếp `(-visit_count, place_id)` (`social_map.py:87-90`).
- `trending[]` (`MapPlace`, `schemas.py:2156-2164`): `place_id`, `place_name`, `lat`, `lng`, `rating`, `rating_count`. Mọi place có `flag == "hot"`, xếp theo id (`social_map.py:93-113`). Không phụ thuộc nhóm, và **không** loại nơi đã đến.
- `recommended[]` (`MapPlace`): place chưa có trong `visited`, xếp `(-(score_place(place, group_taste)[0] or 0), id)`, lấy 8 (`service.py:5003-5011`, `:5019-5029`, `:402`).
  - `group_taste` (`service.py:1165-1197`) cộng câu trả lời `person_interests` và `people.budget_band` của thành viên **đang hoạt động** (`repository.py:4042-4062`, `:4063`; `services/api/app/places/taste.py:199`).
  - Nhóm cặp đôi chưa đồng ý thì dùng `UNKNOWN` (`service.py:1185-1186`). Không ai trả lời thì mọi điểm là `None`, thứ tự rơi về id.
  - `score_place` ở `services/api/app/places/scoring.py:167`.
- `unavailable[]` (`UnavailableLayer`, `schemas.py:2182-2191`): luôn đúng một phần tử `{"layer":"saved","reason":"Chưa có chỗ lưu địa điểm yêu thích, nên lớp này chưa có gì để hiện."}` (`service.py:5030-5038`).
- `scanned_checkins`: số dòng đã quét; `truncated`: `true` khi dừng ở trần 500 mà vẫn còn trang (`service.py:4943-4986`, trang 100 ở `:397`, trần ở `:401`).
- Float: `lat`, `lng` (cả ba lớp), `rating` (`trending`, `recommended`). Python `repr` (`4.5`, `10.7702`).
- Không có datetime.
- Framework: 307 cho `/` cuối; 405 + `allow: GET` cho POST.

## Tác dụng phụ

Chỉ đọc: SELECT `memories` theo trang, `places`, `memberships`, `person_interests`, `people`. Không idempotency, không limiter. `place_rows()` được nhớ trong **một** instance `ApiService` (`service.py:1211-1213`), mà mỗi request tạo instance mới, nên không phải trạng thái trong bộ nhớ.

## Lỗi

| Status | code | detail | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |
| 422 | (framework) | `uuid_parsing` trên `["path","context_id"]` | FastAPI |

## Mã Python

- Route: `services/api/app/api/routes/social_map.py:61-77`
- Service: `services/api/app/api/service.py:4988-5041` (`get_social_map`), `:4943-4986` (`_scan_checkins`), `:1165-1197` (`group_taste`), `:1199-1213` (`place_rows`)
- Repository: `services/api/app/api/repository.py:5167-5205`, `:4173-4188`, `:2769-2782`, `:4042-4062`
- Places: `services/api/app/places/social_map.py:56-113`, `services/api/app/places/scoring.py:167`, `services/api/app/places/taste.py:199`

## Test đang phủ

- `services/api/tests/postgres/test_social_map_postgres.py`: `test_an_outsider_cannot_read_the_map_and_a_member_can` (169), `test_an_invited_member_is_not_yet_a_member` (221), `test_someone_who_left_stops_reading_the_map` (243), `test_another_groups_visits_never_appear_on_this_map` (266), `test_the_map_answer_names_nobody` (309), `test_a_group_with_no_history_gets_an_empty_map_not_an_error` (372), `test_the_saved_layer_is_declared_missing_rather_than_empty` (390), `test_recommended_excludes_places_the_group_has_already_been` (407), `test_all_three_refuse_a_non_member_by_naming_membership_not_a_role` (579), `test_a_member_who_did_not_create_the_group_reads_all_three` (609), `test_a_member_sending_no_role_is_refused_for_the_other_reason` (634)
- `services/api/tests/places/test_social_map.py` (60-120): gộp, thứ tự, photo không phải lượt đến, không lộ tác giả hay thời gian, `hot`
- `services/api/tests/places/test_scoring.py`: điểm gợi ý

## Kịch bản parity

`parity/scenarios/w1/social_map/GET-contexts-context_id-map.yaml`, id `w1/social_map/get-map` (34 bước):

- Xác thực và dạng id: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401); `stranger_non_uuid_context`, `stranger_short_uuid` (422); `stranger_unknown_context` và bốn dạng UUID lạ (403).
- Quyền: `owner_roles_empty`, `stranger_real_group`, `mate_invited_not_yet_active`, `mate_after_leaving` (403).
- Dữ liệu: `owner_empty_map` (visited rỗng, recommended theo id); `owner_map_visited_no_tastes` (visited đếm 2/1/1/1, nơi `hot` đã đến vẫn ở trending nhưng rời recommended); `owner_map_with_tastes` và `mate_map_with_tastes` (recommended đổi thứ tự sau khi hai người khai khẩu vị); `owner_map_after_mate_left` (khẩu vị của người đã rời không còn được cộng).
- Framework: `trailing_slash_redirects` (307), `post_not_allowed` (405).

## Chưa phủ / lưu ý cho bản Go

- `prod` chưa phủ.
- Harness: `cursor` trong câu trả lời check-in dựng dữ liệu là base64(`created_at|id`), không chuẩn hoá được; kịch bản bind nó với `class: token`.
- `truncated: true` cần hơn 500 check-in; harness chưa có `repeat`. Nhánh `scanned_checkins = 500` cũng vậy.
- Nhóm cặp đôi (`kind = pair`) với đồng ý đọc chat còn hay đã tắt: chưa có kịch bản.
- Place không có `rating` (dữ liệu import OSM): `MapPlace.rating: float` bắt buộc, nên Python sẽ ném lỗi validate khi dựng response và trả 500. Seed không có trường hợp này.
- Snapshot tên và toạ độ lấy từ check-in mới nhất: nếu bản Go gộp bằng `GROUP BY` thì phải chọn đúng dòng mới nhất theo `(created_at DESC, id DESC)`.
- `score_place` dùng `Fraction`, hoà điểm phá bằng `place_id`; bản Go phải dùng số hữu tỉ hoặc số nguyên, không float.
- Float `lat`/`lng`/`rating` phải giữ `repr` ngắn nhất và `.0` cho giá trị nguyên.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm) (với `live-go-25-route-wai.md`: áp cho các route AI/gợi ý đọc gu nhóm của pair; các route khác trong tệp chỉ bị chạm theo tên). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).
