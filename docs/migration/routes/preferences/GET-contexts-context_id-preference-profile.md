# GET /contexts/{context_id}/preference-profile

preferences · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tính lại khẩu vị ngầm của một nhóm (F31) từ check-in của nhóm và tổng chi đọc từ sổ, ngay trên request hỏi. Không có bảng hồ sơ nào được lưu.

## Xác thực và quyền

Thứ tự:

1. `get_actor` (`services/api/app/api/deps.py:110-164`): `prod` cần Bearer (thiếu → 401 `Missing bearer session`, phiên hỏng → 401 `Session is not valid`, `services/api/app/api/service.py:4465`, `:4468`); `dev` thiếu `X-Actor-ID` → 401, id / role / contexts sai → 422. Dependency chạy **trước** khi validate path, nên ẩn danh gửi `context_id` không phải UUID vẫn nhận 401 (`anonymous_non_uuid_context`).
2. Validate path `context_id: UUID` → 422 `uuid_parsing`.
3. `is_member(context_id, actor.id)` luôn chạy trước (`service.py:2935-2939`): có dòng `memberships` với `state = active` **và** `left_at IS NULL` (`services/api/app/api/repository.py:2769-2782`).
4. `_require_permission("view_group_preference_profile", ...)`: role phải có `group_admin` hoặc `member` (`services/api/app/domain/permissions.py:364-367`), rồi predicate `is_group_member` (`permissions.py:673-677`).
   - Thiếu cả hai role → 403 detail `role_not_permitted`, kể cả khi là thành viên (`owner_roles_empty`).
   - Có role nhưng không phải thành viên đang hoạt động → 403 detail `is_group_member`.
   - Header `X-Actor-Roles: group_admin` một mình qua được kiểm role (`owner_roles_group_admin_only` → 200). Role trong `dev` là lời tự khai.
   - `X-Actor-Contexts` được parse nhưng không được đọc: khai nhóm trong header không cho quyền gì (`stranger_claims_group_in_header` → 403).
- Nhóm không tồn tại trả **403 `is_group_member`**, không phải 404. Thành viên mới được mời (`invited`) và người đã rời (`left_at` khác null) cũng 403.

## Đầu vào

- Path: `context_id`, UUID theo pydantic lax. Nhận chữ hoa, 32 hex không gạch, dạng `{...}` (gửi percent-encoded `%7B`/`%7D`), `urn:uuid:...`; thiếu một ký tự → 422 (kịch bản `stranger_*_uuid`).
- Query bị bỏ qua. Không body, không header riêng.

## Đầu ra

- 200 `PreferenceProfileResponse` (`services/api/app/api/schemas.py:2348-2371`), thứ tự khoá: `context_id`, `has_profile`, `reason`, `sections`, `checkin_count`, `outing_count`, `split_total_vnd`, `avg_per_person_vnd`.
  - `context_id`: UUID đã parse, luôn in chữ thường có gạch dù path gửi dạng khác.
  - `has_profile = bool(sections)`; `reason` là `"ok"` hoặc `"no_behaviour"` (`service.py:2989-2990`).
  - `sections[]` (`schemas.py:2336-2345`): `section` (`food` trước, `activity` sau, bỏ section rỗng: `services/api/app/domain/preferences.py:62`, `:147-150`), `taste_count` (số khẩu vị khác nhau, trước khi cắt), `tastes[]` tối đa 6 (`preferences.py:67`, `:169`).
  - `tastes[]` (`schemas.py:2315-2333`): `label`, `checkin_count`, `score`. Xếp theo `checkin_count` giảm dần rồi `label` tăng dần theo **code point** Python (`preferences.py:154`): `Local` đứng trước `Lào`.
  - `checkin_count`: số check-in *được đếm* (thuộc section đã biết và có nhãn), không phải tổng check-in (`preferences.py:137-144`).
  - `outing_count`: số chuyến đã bắt đầu, gồm cả chuyến **đang đi** (`service.py:2966-2973`).
  - `split_total_vnd`: cộng `split_total_vnd` của từng chuyến. Chuyến chồng ngày nhau thì **cùng một khoản chi bị cộng hai lần** (kịch bản: một khoản 200000 ra 400000).
  - `avg_per_person_vnd = total // Σheadcount`, hoặc `null` khi không có chuyến (`preferences.py:174-187`). Chia sàn số nguyên.
- Float: `score = ((count*200 + top) // (2*top)) / 100` (`preferences.py:167`): làm tròn nửa lên trên số nguyên, rồi chia float. Ra `1.0`, `0.67`, `0.5`, `0.33`.
- Không có datetime. Ngày "hôm nay" là `_now()` của Python (`service.py:406-407`) quy về `Asia/Ho_Chi_Minh` (`service.py:2966`, `repository.py:120`).
- Nguồn dữ liệu:
  - Catalogue đầy đủ `place_rows()` (`service.py:1199-1213`), `ORDER BY id` (`repository.py:4173-4188`).
  - Check-in: `list_memories(kind="checkin", limit=PROFILE_HISTORY_LIMIT)` với giới hạn = 100 (`service.py:370`, `:375`, `:2955-2964`), mới nhất trước theo `(created_at DESC, id DESC)` (`repository.py:5186`). Check-in có `place_id` đã rời catalogue bị bỏ.
  - Chuyến: `repository.group_recap` (`repository.py:2880-2989`), xem card `GET /contexts/{context_id}/recap`.
- Framework: 307 cho `/` cuối; 405 cho method khác.

## Tác dụng phụ

Chỉ đọc (một transaction chỉ có SELECT, commit trước khi trả, `services/api/app/api/unit_of_work.py:41-48`). Không idempotency (GET). Không limiter. Không ghi log nội dung (`service.py:2930-2932`).

## Lỗi

| Status | code | detail | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | `X-Actor-ID must be a UUID` / `X-Actor-Roles contains an unknown role` / `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:502-504` |
| 403 | `permission_denied` | `is_group_member` | `service.py:502-504` |
| 422 | (framework) | `{"detail":[{"type":"uuid_parsing","loc":["path","context_id"],"msg":"Input should be a valid UUID, <lỗi pydantic-core>","ctx":{"error":"..."}}]}` | FastAPI |

`PreferenceError` của domain (`preferences.py:74-88`, `:176-180`) không tới được qua HTTP: dữ liệu do repository dựng. Nếu nó xảy ra thì là 500.

## Mã Python

- Route: `services/api/app/api/routes/preferences.py:84-98`
- Service: `services/api/app/api/service.py:2909-2996` (`preference_profile`), `:475-504` (`_require_permission`)
- Repository: `services/api/app/api/repository.py:2769-2782` (`is_member`), `:5167-5205` (`list_memories`), `:2880-2989` (`group_recap`), `:4173-4188` (`list_places`)
- Domain: `services/api/app/domain/preferences.py:53-188`, `services/api/app/domain/permissions.py:364-367`

## Test đang phủ

- `services/api/tests/postgres/test_group_intelligence_postgres.py`: `test_a_member_reads_the_profile_of_their_own_group` (257), `test_a_stranger_is_refused` (277), `test_a_membership_row_that_is_not_active_is_not_membership` (294), `test_a_new_checkin_moves_the_profile_with_no_refresh_of_any_kind` (318), `test_two_reads_of_unchanged_rows_agree` (352), `test_a_group_that_has_been_nowhere_says_so` (376), `test_another_groups_checkins_never_reach_this_profile` (395)
- `services/api/tests/postgres/test_pr301_cross_group_leak_postgres.py`: `test_the_profile_of_another_group_is_refused_and_empty` (120), `test_the_profile_of_a_reader_in_both_groups_stays_on_the_path_group` (304), `test_naming_the_group_in_a_header_grants_nothing` (349)
- `services/api/tests/domain/test_preferences.py` (33-184): điểm, làm tròn nửa lên, cắt 6, hoà theo nhãn, chia sàn

## Kịch bản parity

`parity/scenarios/w1/preferences/GET-contexts-context_id-preference-profile.yaml`, id `w1/preferences/get-preference-profile` (41 bước):

- Xác thực: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401 trước 422).
- Dạng id: `stranger_non_uuid_context`, `stranger_short_uuid` (422); `stranger_unknown_context`, `stranger_uppercase_uuid`, `stranger_unhyphenated_uuid`, `stranger_braced_uuid`, `stranger_urn_uuid` (403, nghĩa là đã parse được).
- Quyền: `owner_roles_empty` (`role_not_permitted`), `owner_roles_group_admin_only` (200), `owner_bad_contexts_header` (422), `stranger_real_group`, `stranger_claims_group_in_header`, `mate_invited_not_yet_active`, `mate_after_leaving` (403 `is_group_member`).
- Dữ liệu: `owner_empty_profile` (`no_behaviour`, `avg` null), `owner_profile_food_only` (1.0 / 0.5, thứ tự `Local` < `Lào`), `owner_profile_full` và `mate_profile_full` (activity 10 khẩu vị cắt còn 6, 0.67 / 0.33, 2 chuyến gồm chuyến đang đi, chuyến tương lai bị loại, khoản chi bị đếm hai lần, `avg` 57142).
- Framework: `trailing_slash_redirects`.

## Chưa phủ / lưu ý cho bản Go

- `prod` chưa phủ; harness chưa có persona `prod`.
- Harness: bước dựng `POST /contexts/{context_id}/checkins` trả `cursor` = base64(`created_at|id`). Bộ chuẩn hoá không nhìn vào trong được nên hai stack luôn khác; kịch bản bind nó với `class: token`. Lượt chạy đầu đỏ ở đúng các bước này vì thiếu bind, không phải vì proxy.
- Không phủ được với catalogue seed: category ngoài `SECTION_OF_CATEGORY` (mọi category seed đều đã ánh xạ), `kinds` rỗng, nhãn dài hơn 40 ký tự (`preferences.py:71`, `:106-110`), place đã rời catalogue.
- Giới hạn 100 check-in **không được báo** trên wire (không có `truncated`). Muốn phủ cần hơn 100 bước check-in; harness chưa có `repeat`.
- Ranh giới nửa đêm giờ Việt Nam giữa hai stack có thể làm một chuyến đổi trạng thái; kịch bản dùng ngày 2020/2099 để tránh.
- Float: Go `strconv.FormatFloat(x,'f',-1,64)` ghi `1` chứ không phải `1.0`. Cần tự thêm `.0` cho giá trị nguyên (canary `float-lost-point` bắt đúng chỗ này). Phép `/100` trên float64 cho cùng bit ở hai bên.
- Sort nhãn theo byte UTF-8 của Go trùng với code point của Python; **không** dùng collation.
- `split_total_vnd` cộng chồng các chuyến trùng ngày là hành vi hiện tại, không phải hợp đồng mong muốn; đổi thì mở ADR.
- Thông điệp `uuid_parsing` là chữ của pydantic-core và phải chép nguyên văn cho mọi dạng lỗi (ký tự lạ, độ dài nhóm, độ dài tổng).

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm) (với `live-go-25-route-wai.md`: áp cho các route AI/gợi ý đọc gu nhóm của pair; các route khác trong tệp chỉ bị chạm theo tên). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `GET /contexts/{context_id}/preference-profile`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `GET /contexts/{context_id}/preference-profile`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `GET /contexts/{context_id}/preference-profile`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
