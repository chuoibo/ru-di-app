# POST /papers/{paper_id}/versions/{version}/responses

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Hai điều người nhận có thể nói (spec §3.3, ADR-0027 K3): «Ừ» (`dong_y`) hoặc «đề nghị sửa» (`de_nghi_sua`). Khi người thứ hai đồng ý cùng một phiên bản, tờ thành `chot` và đúng một buổi đi (outing) được tạo **trong cùng request**. Đề nghị sửa là một phiên bản mới do chính người đề nghị gửi và đồng ý.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 200 lưu và phát lại (`mate_replays_header_key`); thân khác cùng khoá → 422 (`mate_header_key_other_body`).
2. JSON hỏng → 422. `get_actor` → 401 (`anonymous_responds`), 422.
3. Validation path (`version: int` lax) và thân `PaperResponseRequest` (`services/api/app/api/schemas.py:2691-2707`), union phân biệt theo `kind`:
   - `discriminator` **không** có tác dụng khi FastAPI validate thân này: lỗi là của union thường, liệt kê lỗi của **từng model** theo thứ tự `PaperAgreeRequest`, `PaperReviseRequest`, với tên model trong `loc`. `kind` thiếu → `missing` tại `["body","PaperAgreeRequest","kind"]`, `["body","PaperReviseRequest","kind"]`, `["body","PaperReviseRequest","content"]`; `kind` lạ → `literal_error` `Input should be 'dong_y'` rồi `Input should be 'de_nghi_sua'` và `missing` `content` (`mate_kind_missing`, `mate_kind_unknown`, `mate_kind_uppercase`, `mate_kind_number`);
   - `dong_y` kèm `content` → 422 `extra_forbidden` (`mate_agree_with_content`); `de_nghi_sua` thiếu `content` → 422 (`mate_counter_without_content`); `content` theo `PaperContentInput` như PATCH nháp (`mate_counter_three_stops`, `mate_counter_ly_do_201`);
   - thân `null`/mảng → 422 (`mate_body_null`, `mate_body_array`); `mate_version_word` → 422 `int_parsing`; `mate_version_decimal` (`9.0`) và `mate_version_leading_zero` (`09`) parse thành 9 và nhận 404 ở bước 5.
4. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_responds_unknown`), 404 `notebook_not_found` (`stranger_agrees_real_paper`), 403 `role_not_permitted` của `view_pair_paper` (`mate_agrees_as_advancer`), 404 cho nháp của người khác (`mate_agrees_owner_draft`).
5. Phiên bản không có → 404 `paper_not_found` `Không có phiên bản này.` (`service.py:7786-7787`; `mate_agrees_version_9`).
6. `_require_pair_permission("respond_pair_paper", …)` (`service.py:7788-7796`; `services/api/app/domain/permissions.py:575-578`), theo thứ tự (role đã qua ở bước 4): `version_current` → 409 `paper_version_stale` (`owner_agrees_version_2`, `mate_agrees_stale_v1`); `is_not_version_sender` → 409 `paper_self_response` (`owner_agrees_own_sent`, `mate_agrees_own_v2`, `mate_counter_own_v2`, `owner_agrees_own_v3`).
7. Nhánh `dong_y` (`service.py:7801-7837`):
   - ghi hàng `dong_y`; nếu người này đã đồng ý phiên bản đó (`paper_already_agreed`) → **200** với trạng thái hiện tại (`hieu_luc`) và `outing_id` hiện có, không ghi gì (`mate_agrees_v3_again` → `chot`, `mate_agrees_after_done` → `da_di`, `mate_agrees_after_keep` → `da_giu`);
   - ngược lại `chuyen(paper, "dong_y", du_dong_y=…)`: tờ mở quá hạn → 409 `paper_expired`; `hieu_luc` ngoài `da_gui, da_xem, dong_y` → 409 `paper_wrong_state` (hàng vừa ghi rollback; `owner_agrees_own_draft`, `owner_agrees_withdrawn`, `mate_agrees_cancelled`).
8. Nhánh `de_nghi_sua` (`service.py:7898-7946`): `chuyen(paper, "de_nghi_sua")`: quá hạn → 409 `paper_expired`; `chot`/`da_di` → 409 `paper_frozen` `Hai bạn chốt rồi, không sửa nữa.` (`mate_counter_frozen`, `mate_counter_after_done`); trạng thái khác ngoài `da_gui, da_xem, dong_y` → 409 `paper_wrong_state` (`owner_counter_own_draft`, `mate_counter_after_keep`, `mate_counter_cancelled`).

## Đầu vào

- Path `paper_id`, `version`.
- Thân `{"kind":"dong_y"}` hoặc `{"kind":"de_nghi_sua","content":{…},"ly_do":…}`; mọi model `extra="forbid"`.

## Đầu ra

**200** `PaperCommandResponse` (`schemas.py:2663-2675`), thứ tự `id`, `state`, `version`, `outing_id`:

- `dong_y` thứ hai → `{state:"chot", version:<đã đồng ý>, outing_id:<uuid>}` (`mate_agrees_v3`).
- `dong_y` chưa đủ hai người → `{state:"dong_y", …}` (chỉ tới được với tờ Nếp gửi; lát 1 không có).
- `de_nghi_sua` → `{state:"da_gui", version:<cũ + 1>, outing_id:null}` (`mate_counter_proposes`, `owner_counter_v2`).

## Tác dụng phụ

Cùng `now`, dưới khoá hàng tờ:

- `dong_y` (`service.py:7805-7837`):
  - `pair_paper_responses (paper_id, version, person_id, kind='dong_y', created_at)`; unique một phần `uq_pair_responses_agree_per_person` (`services/api/app/db/models.py:3326-3368`); trigger người trả lời còn trong cuộc trò chuyện (`c4f27a90d1e3_them_so_hai_nguoi_va_to_giay.py:688-722`).
  - Đủ hai người (`services/api/app/domain/pair_paper.py:167-178`) → `_chot` (`service.py:7839-7896`): đã có link → dùng lại; ngược lại dựng `OutingCreateRequest` (tiêu đề `Tờ lời rủ DD/MM` theo `content.ngay` của phiên bản đó, `starts_on = ends_on = ngay`, `headcount = 2`, `budget_per_person_vnd = 0`) → `outings (context_id, created_by_id = người đồng ý cuối, title, starts_on, ends_on, headcount, budget_per_person_vnd, created_at)` (`repository.py:2842-2866`) → `pair_paper_outings (paper_id, version, outing_id, linked_at)` (`repository.py:8226-8239`; trigger hoãn `pair_paper_outing_link_valid`, migration `:595-636`) → `pair_papers.state='chot'`.
- `de_nghi_sua` (`service.py:7914-7946`): `pair_paper_responses` `de_nghi_sua` cho phiên bản cũ; `pair_paper_versions (version+1)` với `content` chuẩn hoá, `ly_do` cắt/`null` (`owner_reads_hostile_counter`), `nguon = {"scope":"chung","dung":["nguoi"],"luc": now.isoformat()}`, `author_type='human'`, `sent_at = now`, `sent_by = actor`; `pair_paper_responses` `dong_y` cho phiên bản mới; `pair_papers.state='da_gui'`, `current_version = version+1`.
- NUL trong `viec` của đề nghị sửa → 500 (`mate_counter_task_nul`).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` / `Không có phiên bản này.` | `service.py:7668`, `:7787` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_version_stale` | `Tờ giấy đã sang phiên bản mới.` | `service.py:8066-8070` |
| 409 | `paper_self_response` | `Đây là tờ bạn gửi, chờ người kia trả lời.` | `service.py:8071-8075` |
| 409 | `paper_frozen` | `Hai bạn chốt rồi, không sửa nữa.` | `service.py:8086` |
| 409 | `paper_wrong_state` | `Tờ giấy không ở trạng thái làm được việc này.` | `service.py:8087` |
| 409 | `paper_expired` | `Tuần này hết rồi. Tuần sau mình rủ lại nhé.` | `service.py:8085` |
| 409 | `paper_outing_exists` | `Tờ này đã có buổi đi rồi.` (không tới được qua HTTP) | `service.py:7890-7895` |
| 500 | — | `Internal Server Error` | `services/api/app/api/guest_privacy.py:65` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:151-166`
- Service: `services/api/app/api/service.py:7780-7946`
- Repository: `services/api/app/api/repository.py:8190-8243`, `:8112-8138`, `:2842-2866`
- Domain: `services/api/app/domain/pair_paper.py:167-178`, `:225-302`
- Schema: `services/api/app/api/schemas.py:460-479` (`OutingCreateRequest`), `:2691-2707`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_the_sender_cannot_answer_their_own_version` (204), `test_two_yeses_make_one_outing_in_the_same_request` (212), `test_a_second_yes_is_the_same_yes_and_never_a_second_outing` (231), `test_a_counter_proposal_is_a_new_version_its_author_has_agreed_to` (248), `test_a_yes_aimed_at_a_version_that_has_been_replaced_is_refused` (278), `test_a_frozen_plan_takes_no_counter_proposal` (309), `test_a_yes_carrying_a_counter_proposal_is_refused` (580), `test_a_version_that_does_not_exist_is_not_a_version` (591)
- `services/api/tests/postgres/test_pair_papers_races_postgres.py`: `test_hai_lan_dong_y_cung_luc_chi_sinh_mot_keo` (245), `test_de_nghi_sua_thang_thi_lan_dong_y_cu_nhan_stale` (283), `test_chot_hai_lan_tren_cung_mot_to_khong_sinh_keo_thu_hai` (440)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_mot_to_chi_sinh_mot_outing` (216), `test_link_outing_*` (243-309), `test_nguoi_da_roi_cuoc_tro_chuyen_cung_khong_tra_loi_duoc` (380)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-versions-version-responses.yaml` (67 bước, `dev`):

- Nháp: `owner_agrees_own_draft`, `mate_agrees_owner_draft`, `owner_counter_own_draft`.
- Thứ tự: `anonymous_responds`, `stranger_responds_unknown`, `owner_agrees_own_sent`, `owner_agrees_version_2`, `mate_agrees_version_9`, `stranger_agrees_real_paper`, `mate_agrees_as_advancer`.
- Thân 422 (route bị bộ sinh hoãn): `mate_kind_missing`, `mate_kind_unknown`, `mate_kind_number`, `mate_agree_with_content`, `mate_counter_without_content`, `mate_counter_three_stops`, `mate_counter_ly_do_201`, `mate_body_null`, `mate_body_array`, `mate_kind_uppercase`, `mate_version_word`, `mate_version_decimal`, `mate_version_leading_zero`.
- Ba phiên bản: `mate_counter_proposes`, `mate_agrees_stale_v1`, `mate_agrees_own_v2`, `mate_counter_own_v2`, `owner_counter_v2`, `mate_agrees_v3`, `owner_reads_plan`, `mate_agrees_v3_again`, `owner_agrees_own_v3`, `mate_counter_frozen`, `owner_records_done`, `mate_agrees_after_done`, `mate_counter_after_done`, `owner_keeps`, `mate_agrees_after_keep`, `mate_counter_after_keep`.
- Header key: `mate_agrees_with_header_key`, `mate_replays_header_key`, `mate_header_key_other_body`.
- Chữ độc: `mate_counter_task_nul`, `mate_counter_hostile`, `owner_reads_hostile_counter`; tờ rút và tờ huỷ: `mate_withdraws_v2`, `owner_agrees_withdrawn`, `mate_agrees_cancelled`, `mate_counter_cancelled`.

`crossreplay/…responses.yaml` (16 bước): `dong_y` lưu qua Python được cửa trước phát lại, một outing; `de_nghi_sua` cùng khoá 422; đề nghị sửa lưu ở cửa trước được Python phát lại, hai phiên bản.

`concurrency/…responses.yaml` (9 bước): chỉ ba lần `dong_y` cùng header key → một `chot`, hai lần phát lại.

`pair_papers/prod-auth.yaml`: `junk_bearer_responds`, `mate_counter_proposes`, `owner_agrees_v2`.

Corpus sinh tự động: hoãn, `path version is int`.

## Chưa phủ / lưu ý cho bản Go

- 422 của thân là lỗi union thường, không phải tagged union: Go phải liệt kê lỗi của cả hai model, đúng thứ tự, tên model trong `loc`, dù schema khai `discriminator`.
- Outing sinh trong cùng giao dịch với link và trạng thái; trigger link hoãn tới commit.
- `paper_already_agreed` là đường phát lại: 200, không phải 409, và trả `hieu_luc` hiện tại.
- `paper_expired`, `paper_outing_exists` và `state:"dong_y"` không phủ.

## Lỗi Python (chỉ báo, không sửa)

- Đọc cũ dưới khoá (xem thẻ `POST /papers/{id}/send`): hai «ừ» cùng lúc không khoá idempotency — request sau có thể thấy `da_gui` cũ và trả `{"state":"da_gui","outing_id":…}` thay vì `chot`; hai đề nghị sửa cùng lúc có thể chèn trùng khoá `(paper_id, version+1)` → 500 thay vì 409 `paper_version_stale`. Phụ thuộc lịch chạy nên không có trong kịch bản.
- Nhánh `dong_y` coi «đã đồng ý phiên bản này» là phát lại **trước** khi xét trạng thái: sau `da_di`/`da_giu` một lần «ừ» nữa vẫn 200 với trạng thái hiện tại (`mate_agrees_after_done`, `mate_agrees_after_keep`), còn «đề nghị sửa» cùng lúc là 409.
- `Field(discriminator="kind")` không được dùng khi validate: thân sai trả lỗi của cả hai model thay vì một lỗi tag.
- NUL trong nội dung đề nghị sửa là 500.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /papers/{paper_id}/versions/{version}/responses`: Route này đổi hành vi thật: «ừ» thứ hai tạo kèo có tên theo chỗ và có chặng; đề nghị sửa nhận `place_id` dạng slug (trước 422). Id dạng UUID vẫn nhận nguyên chữ, nhưng không còn chuẩn hoá (`{…}`/`urn:uuid:` giữ nguyên chữ thay vì về dạng chuẩn).

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /papers/{paper_id}/versions/{version}/responses`: Route này đọc/ghi tờ qua `_readable_paper_or_404`: với tờ chưa từng gửi ở trạng thái đóng, người không phải chủ nay nhận 404 `paper_not_found` (trước: đọc được, hoặc lỗi trạng thái). Tờ đã gửi: byte không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `POST /papers/{paper_id}/versions/{version}/responses`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — Nếp dùng gu của người đã bật `chia_gu` khi phác tờ (ADR-0034 §2.2)

Diff này tách `_pair_context_or_404` thành `_pair_roster_or_404` (cùng ba lệnh đọc, cùng thứ tự, cùng câu trả lời; chỉ giữ thêm tên hiển thị của hàng thành viên — Go `pairRosterOr404`, `Member.DisplayName`) và thêm vào `draft_pair_paper` bước gu: `_gu_cho_nep` + hàm thuần `pair_paper.gu_cho_nep` / `loai_theo_gu` / `lam_giau_theo_gu` (Go `pairpaper.GuChoNep` / `LoaiTheoGu` / `LamGiauTheoGu`). Golden: `python_pair_paper*.json` (ca `gu_cho_nep`, `lam_giau_theo_gu`, fuzz riêng), `python_pair_steps.json` (7 ca `taste_*` của draft); Go replay 0 lệch.

- `POST /papers/{paper_id}/versions/{version}/responses`: Chỉ đi qua `_pair_roster_or_404` (cùng lệnh đọc, cùng câu trả lời) hoặc bị cổng nối theo tên hàm kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — «Người lo» của tuần trong sổ đôi (ADR-0034 §2.3–2.4)

Diff này thêm `week_role` vào `PairNotebookResponse` (`_week_role`, Go `pairsteps.weekRole`): chỉ trong «Một đôi» đang mở; lựa chọn của tuần (`get_pair_rhythm`, bảng mới `pair_cycle_rhythms`, migration `f4a8d2c6b1e9`) nếu có, không thì suy bằng hàm thuần `pair_notebook.nguoi_lo_suy` (gửi tờ trước ×2, đề nghị sửa ×1, trong chu kỳ này; hoà → người lập sổ) và `vai_tuan`. Không có giới tính. `pair_notebook` giờ đọc tờ một lần cho cả tờ mở lẫn người lo (`_open_paper_id(papers=…)`). Route mới `PUT …/notebook/week-role` (Go phục vụ, evidence riêng). Golden `python_pair_notebook*.json` (ca `lo:*`), `python_pair_steps*.json` (`role_*`, `set_pair_week_role`), permissions; repo oracle Postgres Go có 4 ca week-role.

- `POST /papers/{paper_id}/versions/{version}/responses`: Chỉ bị cổng nối theo tên hàm (`_open_paper_id`, `pair_notebook`, repository) kéo vào — hành vi không đổi.
