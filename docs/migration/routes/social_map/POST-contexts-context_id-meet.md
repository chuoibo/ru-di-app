# POST /contexts/{context_id}/meet

social_map · core · trạng thái trong bộ nhớ: không có

## Mục đích

F45: nhận một danh sách khu vực xuất phát không gắn tên người, trả 5 điểm hẹn công bằng nhất (tối thiểu hoá quãng đường dài nhất) cùng các phép tính đứng sau. Route không đọc lịch sử nào của nhóm và không ghi bảng nghiệp vụ nào.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` nếu có `Idempotency-Key`: key rỗng hoặc dài hơn 255 → 422; key đã dùng cho request khác → 422; đã có kết quả → replay (`services/api/app/api/idempotency.py:404-502`).
2. JSON hỏng → 422 `json_invalid`, **trước** xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`).
4. Path `context_id` **và** body được validate cùng lúc: lỗi của cả hai gom vào **một** mảng 422, lỗi path đứng trước (`stranger_non_uuid_and_bad_body`). Vì bước này chạy trước handler, người lạ gửi trường thừa nhận **422 chứ không 403** (`stranger_unknown_context_extra_field`).
5. `is_member` + `_require_permission("view_meeting_point")` (`services/api/app/api/service.py:5082-5086`; role `group_admin` hoặc `member`, predicate `is_group_member` ở `services/api/app/domain/permissions.py:407-410`) → 403. Bước này chạy **trước** kiểm số lượng và id khu vực (`stranger_real_group_unknown_area` → 403).
6. Số khu vực phải trong `[2, 12]` (`service.py:5087-5093`; `services/api/app/places/meeting.py:63`, `:67`) → 422 `invalid_origin_count`. Chạy trước kiểm id: `["sao-hoa"]` → `invalid_origin_count`.
7. Từng id theo thứ tự gửi, id lạ **đầu tiên** → 422 `unknown_area` (`service.py:5095-5104`).

Nhóm không tồn tại → 403 `is_group_member`, không 404.

## Đầu vào

- Path `context_id` (UUID lax).
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` (`text/plain` → 422 `model_attributes_type`).
- Body `MeetingPointRequest` (`services/api/app/api/schemas.py:2238-2247`), `extra="forbid"`: `from_areas: list[StrictStr]`, bắt buộc. Schema không giới hạn độ dài (giới hạn nằm ở service). Được trùng id. So khớp id chính xác theo byte với `AREAS` (`services/api/app/places/areas.py:119-126`): không trim, không casefold.

## Đầu ra

- **200** (POST nhưng không phải 201) `MeetingPointResponse` (`schemas.py:2290-2309`), thứ tự khoá `context_id`, `origins`, `candidates`, `two_origin_inversion`.
- `origins[]` (`AreaSummary`, `schemas.py:2250-2256`: `id`, `label`, `lat`, `lng`): lặp lại đúng thứ tự gửi, **giữ bản trùng** (`service.py:5111`).
- `candidates[]` tối đa 5 (`service.py:403`, `:5106-5108`), mỗi phần tử `MeetingCandidate` (`schemas.py:2279-2288`): `place_id`, `place_name`, `category`, `address`, `lat`, `lng`, `fairness`, `travel`.
  - `fairness` (`schemas.py:2265-2276`): `worst_km`, `total_km`, `spread_km`.
  - `travel[]` (`MeetingLeg`, `schemas.py:2259-2262`, kế thừa `AreaSummary`): `id`, `label`, `lat`, `lng`, `km`, một phần tử cho mỗi origin kể cả bản trùng.
  - Tính trên **mọi** place của catalogue (`place_rows()`, `service.py:1199-1213`), không lọc theo nhóm hay thành phố.
  - Xếp theo giá trị **chưa làm tròn** `(worst, sum(legs), place_id)` (`meeting.py:132-136`).
  - Số trên wire là `round(x, 2)` của Python (`meeting.py:121-131`).
- `two_origin_inversion`: `len(origins) == 2`, tính cả bản trùng (`service.py:5113`).
- Float: `origins[].lat/lng`, `candidates[].lat/lng`, `fairness.*`, `travel[].lat/lng/km`. Không có datetime.
- Replay idempotency: cùng 200 và cùng body, thêm `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; 405 + `allow: POST` cho GET.

## Tác dụng phụ

- Không ghi bảng nghiệp vụ nào; SELECT `memberships`, `places`.
- **Có ghi** khi có `Idempotency-Key`: middleware không phân biệt route chỉ đọc, nên đặt chỗ và lưu câu trả lời 200 vào `idempotency_keys` (`idempotency.py:504-546`). Docstring route nói "Not idempotency-keyed, because it writes nothing" (`services/api/app/api/routes/social_map.py:117-121`), nhưng hành vi đo được thì có replay (`idem_replay`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:144-163` | |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |
| 422 | `invalid_origin_count` | `Cần từ 2 đến 12 khu vực xuất phát để tìm điểm hẹn.` | `service.py:5087-5093` |
| 422 | `unknown_area` | `Không có khu vực nào tên {area_id}.` (chèn **nguyên văn** id người gửi) | `service.py:5098-5103` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `PUT /people/me/interests` | `idempotency.py:432-480` |
| 409 | `idempotency_request_in_flight` | như trên | `idempotency.py:481-493` |

422 framework đã đo: `list_type`, `string_type` (một lỗi cho **mỗi** phần tử sai, `loc` có chỉ số), `missing` (`["body","from_areas"]` hoặc `["body"]` khi body rỗng), `extra_forbidden`, `model_attributes_type`, `json_invalid`.

## Mã Python

- Route: `services/api/app/api/routes/social_map.py:100-123`
- Service: `services/api/app/api/service.py:5064-5114` (`get_meeting_point`)
- Places: `services/api/app/places/meeting.py:63-139` (`rank_meeting_points`), `services/api/app/places/areas.py:119-203` (`find_area`, `haversine_km`, `area_summary`)
- Repository: `services/api/app/api/repository.py:2769-2782`, `:4173-4188`

## Test đang phủ

- `services/api/tests/postgres/test_social_map_postgres.py`: `test_an_outsider_cannot_ask_for_a_meeting_point_and_a_member_can` (205), `test_one_area_is_refused_because_it_is_not_a_meeting` (425), `test_an_unknown_area_is_refused_rather_than_silently_dropped` (439), `test_the_two_origin_inversion_is_disclosed` (459), `test_the_meeting_answer_names_no_member` (486), `test_the_meeting_request_has_no_field_for_naming_a_person` (509), cùng ba ca chung ở dòng 579, 609, 634
- `services/api/tests/api/test_areas.py::test_the_ids_offered_are_ids_meet_accepts` (33)
- `services/api/tests/places/test_meeting.py` (46-177): minimax, tie-break theo tổng, bản trùng, giới hạn, không lộ người

## Kịch bản parity

`parity/scenarios/w1/social_map/POST-contexts-context_id-meet.yaml`, id `w1/social_map/post-meet` (48 bước):

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401), `anonymous_malformed_json` (422 trước 401), `stranger_non_uuid_context`, `stranger_unknown_context_extra_field` (422 trước 403), `stranger_non_uuid_and_bad_body` (hai lỗi gom một), `stranger_unknown_context`, `stranger_real_group_unknown_area` (403 trước 422).
- Đường vui: `owner_two_origins` (inversion true), `owner_three_with_repeat` (trùng id), `owner_cross_city` (km lớn), `owner_every_area`, `owner_twelve_origins_max`, `mate_two_origins`.
- Biên số lượng: `owner_thirteen_origins`, `owner_one_origin`, `owner_zero_origins`, `owner_one_unknown_area_counts_first`.
- Id lạ: `owner_unknown_area`, `owner_first_unknown_is_named`, `owner_unknown_area_needs_escaping` (`<b>&"Quận 9"</b>\` được chèn vào detail), `owner_area_leading_space`, `owner_area_uppercase`.
- Validate: `from_areas_not_a_list`, `from_areas_numbers`, `from_areas_null`, `missing_from_areas`, `extra_field_members`, `empty_body`, `text_plain_content_type`.
- Quyền: `owner_roles_empty`, `mate_invited_not_yet_active`, `mate_after_leaving`.
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_second_distinct_key`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`.
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307).

## Chưa phủ / lưu ý cho bản Go

- `prod` và 409 in-flight chưa phủ (harness chưa có persona `prod`, chạy tuần tự).
- **Thoát JSON**: Python chỉ thoát `"` và `\`; `encoding/json` của Go mặc định thoát `<`, `>`, `&` thành `<`… Phải tắt bằng `SetEscapeHTML(false)`, nếu không thì `owner_unknown_area_needs_escaping` sẽ lệch.
- **Làm tròn**: `round(x, 2)` của Python làm tròn đúng theo giá trị thập phân của số nhị phân (half-even trên giá trị chính xác). `math.Round(x*100)/100` của Go khác ở các ca biên. Nên dùng `strconv.FormatFloat(x, 'f', 2, 64)` rồi parse lại, và kiểm bằng vector.
- Sau khi làm tròn, float vẫn được ghi bằng `repr` ngắn nhất: `10.0` giữ `.0`, `3.1` không thành `3.10`.
- Xếp theo giá trị chưa làm tròn; hai place làm tròn ra cùng số vẫn có thứ tự xác định bằng tổng rồi `place_id`.
- `unknown_area` phản chiếu đầu vào người dùng vào detail; bản Go phải giữ nguyên chuỗi (kể cả khoảng trắng đầu và chữ hoa), không được làm sạch.
- Idempotency trên route chỉ đọc là hành vi đo được. Bản Go có bỏ middleware cho route này thì `idem_replay` lệch (thiếu `idempotency-replayed`); muốn đổi thì mở ADR.
- Catalogue lớn (OSM import) sẽ đổi tập candidate; kịch bản chỉ dùng 12 place seed.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm) (với `live-go-25-route-wai.md`: áp cho các route AI/gợi ý đọc gu nhóm của pair; các route khác trong tệp chỉ bị chạm theo tên). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /contexts/{context_id}/meet`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /contexts/{context_id}/meet`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `POST /contexts/{context_id}/meet`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — Nếp dùng gu của người đã bật `chia_gu` khi phác tờ (ADR-0034 §2.2)

Diff này tách `_pair_context_or_404` thành `_pair_roster_or_404` (cùng ba lệnh đọc, cùng thứ tự, cùng câu trả lời; chỉ giữ thêm tên hiển thị của hàng thành viên — Go `pairRosterOr404`, `Member.DisplayName`) và thêm vào `draft_pair_paper` bước gu: `_gu_cho_nep` + hàm thuần `pair_paper.gu_cho_nep` / `loai_theo_gu` / `lam_giau_theo_gu` (Go `pairpaper.GuChoNep` / `LoaiTheoGu` / `LamGiauTheoGu`). Golden: `python_pair_paper*.json` (ca `gu_cho_nep`, `lam_giau_theo_gu`, fuzz riêng), `python_pair_steps.json` (7 ca `taste_*` của draft); Go replay 0 lệch.

- `POST /contexts/{context_id}/meet`: Chỉ đi qua `_pair_roster_or_404` (cùng lệnh đọc, cùng câu trả lời) hoặc bị cổng nối theo tên hàm kéo vào — hành vi không đổi.
