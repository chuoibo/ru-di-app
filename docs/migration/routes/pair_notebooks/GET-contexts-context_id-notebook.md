# GET /contexts/{context_id}/notebook

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Sổ hai người nhìn từ một trong hai người (ADR-0027 K1, K4): chu kỳ đang sống, hai người của nó, mình đã bật công tắc nào, người kia đã bật công tắc nào (chỉ boolean), các lời đề nghị còn chờ, hai ô ràng buộc của cả hai, và id tờ giấy đang mở mà người này được biết. Route chỉ đọc; không tạo hàng sổ.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Router: đuôi `/` → 307 `location` tuyệt đối (`owner_reads_trailing_slash`); `POST` → 405 `allow: GET` (`owner_posts_to_notebook`). Query lạ bị bỏ qua (`owner_reads_with_query`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`):
   - dev: thiếu `X-Actor-ID` → 401 `authentication_required` `Missing X-Actor-ID` (`anonymous_reads_unknown`); không phải UUID → 422 `invalid_actor_id` (`stranger_bad_actor_id`); role lạ → 422 `invalid_actor_roles`.
   - prod: không có bearer → 401 `authentication_required` `Missing bearer session`; bearer rác → 401 `Session is not valid`; `X-Actor-*` bị bỏ qua (`pair_notebooks/prod-auth.yaml`).
3. Validation path `context_id` (UUID lax) → 422.
4. `_pair_context_or_404` (`services/api/app/api/service.py:7215-7237`): context không có, `kind != 'pair'`, hoặc actor không có membership `active` và `left_at IS NULL` (`repository.py:2769-2782`) → **cùng một** 404 `notebook_not_found` `Không có sổ này.` (`stranger_reads_unknown`, `owner_reads_group`, `stranger_reads_pair`, `third_reads_pair` — bạn của owner nhưng không ở pair).
5. `_require_pair_permission("view_pair_notebook", {"is_group_member": True})` (`service.py:7254`, `:8095-8104`; `services/api/app/domain/permissions.py:525`): chỉ còn kiểm role `member` → 403 `permission_denied` `role_not_permitted` (`owner_reads_as_advancer`). Người lạ gửi cùng header vẫn 404 vì bước 4 đi trước (`stranger_reads_pair_as_advancer`). Ở prod mọi phiên mang `member` (`repository.py:3629`).
6. Không kiểm chặn: sau khi một người chặn người kia, cả hai vẫn đọc 200 (`owner_reads_after_block`, `mate_reads_after_block`).

## Đầu vào

- Path `context_id`. Không query, không thân.

## Đầu ra

**200** `PairNotebookResponse` (`services/api/app/api/schemas.py:2742-2760`), JSON gọn, thứ tự khoá:

1. `context_id` — lặp lại id trong path.
2. `cycle_state` — `null` khi chưa có hàng sổ hoặc không còn chu kỳ chưa đóng (`_live_cycle`, `repository.py:7715-7730`); ngược lại `pending` / `active`. **Không bao giờ `closed`**: chu kỳ đã đóng không phải chu kỳ sống (sau khi đóng và hỏi lại, `mate_reads_reopened` chỉ thấy chu kỳ `pending` mới, không còn đồng ý hay ràng buộc cũ).
3. `participants` — chu kỳ sống thì là `pair_cycle_participants` xếp `created_at, person_id` (`repository.py:7667-7674`); không có chu kỳ thì là membership đang hoạt động xếp `created_at, id` (`repository.py:2749-2767`, `service.py:7239-7247`). Hai người của một chu kỳ được ghi cùng `now`, nên thứ tự thực tế là thứ tự **person_id**. Không có chu kỳ sống thì hai membership của pair cũng cùng `created_at` (tạo trong một giao dịch), nên thứ tự rơi vào `memberships.id` là uuid4 ngẫu nhiên: **Python không tất định** ở nhánh này (đo được: `owner_reads_closed` khác nhau giữa hai stack Python ở lượt chạy đầu), và kịch bản không đọc 200 một sổ chưa có hoặc đã hết chu kỳ sống.
4. `my_consents` — luôn ba phần tử theo `CONSENT_PURPOSES` (`lap_so`, `bat_doi`, `doc_chat`), mỗi phần tử `{purpose, granted}`.
5. `their_consents_granted` — object ba khoá cùng thứ tự, boolean; không lộ thời điểm người kia bấm.
6. `pending_proposals` — lời đề nghị của chu kỳ sống còn `dang_cho` (chưa `completed_at`, chưa tới `expires_at`), xếp `created_at, id`; mỗi phần tử `{id, purpose, expires_at, proposed_by_id, my_granted}`. `my_granted` là «mình đang giữ đồng ý **sống** cho purpose này ở bất kỳ lời đề nghị nào của chu kỳ», không phải cho riêng lời đề nghị đó (`service.py:7260-7282`).
7. `constraints` — hàng `pair_shared_constraints` của chu kỳ sống, xếp `owner_id, kind` (`repository.py:7683-7696`), mỗi phần tử `{owner_id, kind, content, version}`; cả hai người đọc cả hai dòng.
8. `nep_gui_ho` — luôn `false` (lát 1 không có cửa bật).
9. `open_paper_id` — tờ đầu tiên (theo `created_at DESC, id`) có `hieu_luc` thuộc `OPEN_STATES`, **bỏ qua nháp của người khác** (`service.py:7312-7324`; `owner_reads_own_draft` thấy id, `mate_reads_without_draft` thấy `null`, `mate_reads_sent_paper` thấy id).

Datetime `expires_at`: ISO-8601 UTC đuôi `Z`, 6 chữ số thập phân (trừ khi micro giây bằng 0). Hạn = lúc đề nghị + 7 ngày (`services/api/app/domain/pair_notebook.py:49`, `:72-74`).

## Tác dụng phụ

Không ghi. Không khoá. Mọi lần đọc dùng `now` của request để áp hạn lời đề nghị (`service.py:406`), nên một lời đề nghị quá 7 ngày biến khỏi `pending_proposals` và đồng ý đi kèm nó thôi tính (`pair_notebook.py:77-91`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143`; `service.py` `actor_for_session_token` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:144-147` |
| 422 | (validation) | `{"detail":[…]}` không có `input` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504`, `:8095-8104` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: GET` | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:52-64`
- Service: `services/api/app/api/service.py:7249-7310` (`pair_notebook`), `:7215-7237`, `:7239-7247`, `:7312-7324`, `:8141-8152`
- Repository: `services/api/app/api/repository.py:7732-7738` (`get_pair_notebook`), `:7657-7713`, `:7623-7643`, `:7715-7730`, `:8092-8098`
- Domain: `services/api/app/domain/pair_notebook.py:77-116` (`_live`, `granted_by`), `:220-233` (`dang_cho`); `services/api/app/domain/pair_paper.py:150-164` (`hieu_luc`)
- Schema: `services/api/app/api/schemas.py:2722-2760`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_a_fresh_pair_has_a_notebook_nobody_has_opened` (60), `test_one_yes_opens_nothing_and_the_other_side_can_see_whose_turn_it_is` (80), `test_an_offer_nobody_answered_in_time_stops_being_an_offer` (117), `test_both_may_read_the_two_lines_and_only_their_owner_writes_one` (185), `test_everybody_outside_the_pair_gets_the_same_four_oh_four` (330)
- `services/api/tests/api/test_pair_papers.py`: `test_every_pair_route_refuses_a_stranger` (660)
- `services/api/tests/postgres/test_pair_notebook_postgres.py` (ràng buộc bảng, không qua route)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/GET-contexts-context_id-notebook.yaml`, id `w8/pair_notebooks/get-contexts-context_id-notebook` (44 bước, `dev`):

- Ngoài sổ: `anonymous_reads_unknown`, `stranger_bad_actor_id`, `stranger_reads_unknown`, `owner_reads_group`, `stranger_reads_pair`, `third_reads_pair`, `stranger_reads_pair_as_advancer`, `owner_reads_as_advancer`.
- Framework: `owner_reads_trailing_slash`, `owner_reads_with_query`, `owner_posts_to_notebook`.
- Vòng đời: `owner_reads_one_yes`, `mate_reads_one_yes`, `owner_reads_open`, `mate_reads_constraints_and_offer`, `owner_reads_own_draft`, `mate_reads_without_draft`, `mate_reads_sent_paper`, `owner_reads_couple`, `owner_reads_after_block`, `mate_reads_after_block`, `owner_reopens`, `mate_reads_reopened`.

`pair_notebooks/prod-auth.yaml` (39 bước): `anonymous_reads`, `dev_headers_without_bearer`, `owner_reads_with_guest_roles_header`, `stranger_reads_claiming_owner`, `mate_reads_open`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/get-contexts-context_id-notebook.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Sổ không có chu kỳ sống (pair chưa từng hỏi, hoặc vừa đóng) không được đọc 200 trong kịch bản vì thứ tự `participants` ngẫu nhiên ở Python. Mọi thứ tự Go chọn đều là một câu trả lời Python có thể cho; xếp theo `person_id` khớp với nhánh có chu kỳ.
- Lời đề nghị hết hạn (7 ngày) và `consent_proposal_expired` theo đồng hồ không phủ được: harness không có bước đồng hồ.
- Thứ tự `participants` và `constraints` dựa vào person_id (hai hàng cùng `created_at`); Go phải đọc cùng `ORDER BY` chứ không dựa vào thứ tự chèn.
- `my_granted` trong `pending_proposals` là «đã đồng ý purpose này», không phải «đã đồng ý lời đề nghị này»: hai lời đề nghị cùng purpose hiện cùng giá trị.
- `their_consents_granted` là object JSON: thứ tự khoá phải là `lap_so`, `bat_doi`, `doc_chat`.
- `open_paper_id` áp `hieu_luc` (hạn tuần) lúc đọc; một tờ quá hạn không còn là tờ mở dù cột `state` vẫn là trạng thái mở.
- 404 đi trước 403 role; chặn không ảnh hưởng.

## Lỗi Python (chỉ báo, không sửa)

- `participants` của sổ không có chu kỳ sống xếp theo `memberships.created_at, id`; hai membership của pair luôn cùng `created_at`, nên thứ tự phụ thuộc uuid4 ngẫu nhiên của membership và khác nhau giữa hai pair hay hai stack.
- Chặn (ADR-0023) không đóng cửa sổ: người đã chặn và người bị chặn vẫn đọc và ghi sổ của nhau. Chỉ tin nhắn kiểm `_require_pair_is_alive`.
- Người kia đã xoá tài khoản vẫn nằm trong `participants` của chu kỳ sống (danh sách chụp lúc mở chu kỳ).

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Thân trả lời thêm trường cuối `granted_purposes: list["lap_so"|"bat_doi"|"doc_chat"]` (thứ tự thang): bậc mà CẢ HAI đã đồng ý trên CÙNG MỘT lời đề nghị. `my_consents` và `their_consents_granted` giữ nghĩa cũ (mỗi người đã trả lời chưa). Client chỉ sáng một bậc theo `granted_purposes` (`apps/mobile/src/rudi/to-giay/so-doi-map.ts caHaiDongY`). Go `pairsteps.ReadNotebook` + `routes.wireNotebook`, Python `pair_notebook` + `PairNotebookResponse`.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `GET /contexts/{context_id}/notebook`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `GET /contexts/{context_id}/notebook`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `GET /contexts/{context_id}/notebook`: `my_consents`/`their_consents_granted` có thêm `chia_gu`; trường mới `taste`: null ngoài «Một đôi», trong «Một đôi» là `{mine_shared, theirs_shared, theirs, common}` — gu người kia chỉ khi HỌ bật, gu chung chỉ khi CẢ HAI bật; chỉ khi đó mới đọc `interests_by_person`.

## Đổi 2026-09-25 — Nếp dùng gu của người đã bật `chia_gu` khi phác tờ (ADR-0034 §2.2)

Diff này tách `_pair_context_or_404` thành `_pair_roster_or_404` (cùng ba lệnh đọc, cùng thứ tự, cùng câu trả lời; chỉ giữ thêm tên hiển thị của hàng thành viên — Go `pairRosterOr404`, `Member.DisplayName`) và thêm vào `draft_pair_paper` bước gu: `_gu_cho_nep` + hàm thuần `pair_paper.gu_cho_nep` / `loai_theo_gu` / `lam_giau_theo_gu` (Go `pairpaper.GuChoNep` / `LoaiTheoGu` / `LamGiauTheoGu`). Golden: `python_pair_paper*.json` (ca `gu_cho_nep`, `lam_giau_theo_gu`, fuzz riêng), `python_pair_steps.json` (7 ca `taste_*` của draft); Go replay 0 lệch.

- `GET /contexts/{context_id}/notebook`: Chỉ đi qua `_pair_roster_or_404` (cùng lệnh đọc, cùng câu trả lời) hoặc bị cổng nối theo tên hàm kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — «Người lo» của tuần trong sổ đôi (ADR-0034 §2.3–2.4)

Diff này thêm `week_role` vào `PairNotebookResponse` (`_week_role`, Go `pairsteps.weekRole`): chỉ trong «Một đôi» đang mở; lựa chọn của tuần (`get_pair_rhythm`, bảng mới `pair_cycle_rhythms`, migration `f4a8d2c6b1e9`) nếu có, không thì suy bằng hàm thuần `pair_notebook.nguoi_lo_suy` (gửi tờ trước ×2, đề nghị sửa ×1, trong chu kỳ này; hoà → người lập sổ) và `vai_tuan`. Không có giới tính. `pair_notebook` giờ đọc tờ một lần cho cả tờ mở lẫn người lo (`_open_paper_id(papers=…)`). Route mới `PUT …/notebook/week-role` (Go phục vụ, evidence riêng). Golden `python_pair_notebook*.json` (ca `lo:*`), `python_pair_steps*.json` (`role_*`, `set_pair_week_role`), permissions; repo oracle Postgres Go có 4 ca week-role.

- `GET /contexts/{context_id}/notebook`: Trường mới `week_role` (null ngoài «Một đôi» đang mở; `{tuan, nguoi_lo, cach: suy|chon, diem}`); thêm lệnh đọc `get_pair_rhythm` sau `list_pair_papers`, trước `interests_by_person`.
