# POST /contexts/{context_id}/notebook/close

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Bước hai của «Đóng sổ là đóng» (spec §7.6, ADR-0027 §8). Mang `revision` vừa đọc ở preview; nếu sổ đã đổi thì từ chối. Không xoá gì: nháp thành `bo`, tờ đang chờ thành `huy`, kế hoạch đã chốt giữ nguyên, hàng cặp đôi bị xoá, chu kỳ sống thành `closed`.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 204 lưu và phát lại (`owner_replays_close`); revision khác cùng khoá → 422 (`owner_header_key_other_revision`); 409 stale nhả khoá (`crossreplay/POST-…-close.yaml` `front_stale_refused`).
2. JSON hỏng → 422.
3. `get_actor` → 401 (`anonymous_closes`), 422.
4. Validation path và thân `CloseNotebookRequest` (`services/api/app/api/schemas.py:2797-2798`) → 422.
5. `_pair_context_or_404` → 404 `notebook_not_found` (`stranger_closes_unknown`, `stranger_closes_pair`, `owner_closes_group`).
6. `_require_pair_permission("close_pair_notebook", {"is_group_member": True})` (`services/api/app/api/service.py:7568-7570`; `services/api/app/domain/permissions.py:604`) → 403 `role_not_permitted` (`owner_closes_as_advancer`). Cả hai người đều đóng được (`mate_closes`).
7. `_locked_notebook` (`service.py:7572`): khoá, tạo hàng sổ nếu chưa có.
8. Tính lại preview dưới khoá; `revision` khác → 409 `notebook_revision_stale` `Sổ vừa đổi. Xem lại rồi đóng.` (`service.py:7573-7578`; `owner_closes_junk_revision`, `owner_revision_64`, `owner_revision_uppercase`, `owner_closes_stale`, `mate_closes_again`).

## Đầu vào

- Path `context_id`.
- Thân JSON, `extra="forbid"`: `revision: StrictStr` 1..64 ký tự. `""` → 422 `string_too_short`; 65 → 422 `string_too_long`; số, `null`, thiếu, trường lạ, không thân → 422 (`owner_revision_empty`, `owner_revision_65`, `owner_revision_number`, `owner_revision_null`, `owner_revision_missing`, `owner_revision_extra_field`, `owner_closes_without_body`). So sánh phân biệt hoa thường.

## Đầu ra

**204**, thân rỗng.

## Tác dụng phụ

Cùng giao dịch, cùng `now` (`service.py:7579-7583`):

- Pair chưa có hàng sổ: `pair_notebooks` được chèn (`owner_closes_fresh` với revision rỗng `e3b0c44298fc1c14`).
- `close_open_pair_papers` (`repository.py:8260-8288`): khoá `FOR UPDATE` mọi tờ của context có **cột** `state` thuộc `nhap, da_gui, da_xem, de_nghi_sua, dong_y`, rồi `nhap → bo`, còn lại `→ huy`. Áp cho cả tờ tạm (không có chu kỳ). Tờ `chot`, `da_di` và mọi trạng thái khép không đổi (`owner_reads_plan`). Không đổi `current_version`, không ghi phản hồi.
- Có chu kỳ sống: xoá `active_couple_members` của hai participant (`repository.py:7916-7920`), rồi chu kỳ `state='closed'`, `closed_at` = `now` (`repository.py:7798-7804`). Lời đề nghị chờ không bị đánh dấu; chúng chỉ không còn thuộc chu kỳ sống.
- Không có chu kỳ sống: chỉ các tờ bị đổi (`owner_closes_closed_notebook`).
- Chu kỳ `pending` cũng đóng được (`owner_reopens`, `owner_closes_pending_cycle`).
- Ràng buộc và đồng ý của chu kỳ cũ giữ nguyên trong bảng; chu kỳ mới không kế thừa.
- `idempotency_keys` khi có header và 204. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["body","revision"]` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `notebook_revision_stale` | `Sổ vừa đổi. Xem lại rồi đóng.` | `service.py:7574-7578` |
| 422 | idempotency | như các POST | `idempotency.py` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:166-181`
- Service: `services/api/app/api/service.py:7557-7583`, `:7539-7545`
- Repository: `services/api/app/api/repository.py:8260-8288`, `:7798-7804`, `:7916-7920`
- Domain: `services/api/app/domain/pair_notebook.py:169-217`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_closing_refuses_a_revision_that_has_gone_stale` (280), `test_closing_is_closing` (294), `test_a_closed_notebook_can_be_opened_again_as_a_new_agreement` (318)
- `services/api/tests/api/test_pair_papers.py`: `test_a_closed_notebook_refuses_the_whole_sheet` (610)
- `services/api/tests/postgres/test_pair_papers_races_postgres.py`: `test_dong_so_thang_thi_lan_chot_bi_tu_choi` (327), `test_xem_truoc_cu_khong_dong_duoc_cuon_so_vua_doi` (367)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/POST-contexts-context_id-notebook-close.yaml` (55 bước, `dev`):

- Sổ chưa mở: `owner_closes_fresh`, `owner_closes_fresh_again`.
- Thứ tự: `anonymous_closes`, `stranger_closes_unknown`, `stranger_closes_pair`, `owner_closes_group`, `owner_closes_as_advancer`, `owner_closes_junk_revision`.
- Thân: `owner_revision_empty`, `owner_revision_65`, `owner_revision_64`, `owner_revision_number`, `owner_revision_null`, `owner_revision_missing`, `owner_revision_extra_field`, `owner_revision_uppercase`, `owner_closes_without_body`.
- Đóng thật (một đôi, một kế hoạch chốt, nháp của mate, lời đề nghị chờ): `owner_previews`, `mate_puts_constraint` và `mate_edits_draft` (revision không đổi, `owner_previews_after_edits`), `mate_proposes_lap_so`, `owner_closes_stale`, `owner_previews_again`, `mate_closes`, `owner_lists_papers`, `mate_lists_papers`, `owner_reads_plan`, `mate_closes_again`.
- Sổ đã đóng: `owner_previews_closed`, `owner_closes_closed_notebook`, `owner_replays_close`, `owner_header_key_other_revision`, `owner_proposes_bat_doi_after_close`.
- Chu kỳ `pending`: `owner_reopens`, `mate_reads_reopened`, `owner_previews_reopened`, `owner_closes_pending_cycle`.

`crossreplay/POST-…-close.yaml` (17 bước). `concurrency/POST-…-close.yaml` (12 bước): ba lần đóng cùng một revision → một 204, hai 409 `notebook_revision_stale` (tính lại dưới khoá); ba lần cùng header key trên sổ đã đóng → một 204, hai lần phát lại.

`pair_notebooks/prod-auth.yaml`: `stranger_closes`, `junk_bearer_closes`, `owner_closes`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-contexts-context_id-notebook-close.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Sổ ngay sau khi đóng không được đọc bằng `GET …/notebook`: không còn chu kỳ sống nên thứ tự `participants` ngẫu nhiên ở Python (thẻ GET). Kịch bản chỉ đọc sau khi hỏi lại `lap_so`.
- Revision so với preview tính lại **dưới khoá hàng sổ**; Go phải khoá trước khi đọc tờ và lời đề nghị.
- `close_open_pair_papers` lọc theo cột `state`, còn revision dùng `hieu_luc`: một tờ mở đã quá hạn tuần vẫn bị đổi thành `bo`/`huy`. Không phủ (không có bước đồng hồ).
- Lệnh đóng trên pair chưa từng có hàng sổ chèn hàng rồi mới so revision; chuỗi rỗng `e3b0c44298fc1c14` là revision hợp lệ duy nhất cho sổ trống.

## Lỗi Python (chỉ báo, không sửa)

- Sau khi đóng, tờ `chot`/`da_di` vẫn nhận `done` và `keeps` (thẻ `POST /papers/{id}/done`), và `draft` sinh tờ tạm mới ngay (thẻ `POST …/papers/draft`): «đóng sổ» không chặn tờ.
- Đóng không đánh dấu lời đề nghị chờ là đã huỷ; `so_de_nghi_huy` là con số, không phải một lần ghi.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /contexts/{context_id}/notebook/close`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /contexts/{context_id}/notebook/close`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `POST /contexts/{context_id}/notebook/close`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
