# POST /papers/{paper_id}/send

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trao tờ nháp cho người kia. Bấm gửi là đồng ý với cái mình gửi (spec §3.4): route đóng dấu phiên bản, ghi `dong_y` của người gửi thành một hàng, và chuyển tờ sang `da_gui`. Thân ghim phiên bản người gửi đang nhìn.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 200 lưu và phát lại (`owner_replays_header_key`); phiên bản khác cùng khoá → 422 (`owner_header_key_other_version`); 409 nhả khoá (`crossreplay/POST-papers-paper_id-send.yaml` `python_stale_refused` rồi `front_sends_same_key`).
2. JSON hỏng → 422. `get_actor` → 401 (`anonymous_sends`), 422.
3. Validation `paper_id` và thân `PaperSendRequest` (`services/api/app/api/schemas.py:2556`, `:2683-2684`): `version` strict int ≥ 1 → 422 cho `0`, `"1"`, `1.0`, `true`, thiếu (`owner_version_zero`, `owner_version_string`, `owner_version_float`, `owner_version_bool`, `owner_version_missing`).
4. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_sends_unknown`), 404 `notebook_not_found` (`stranger_sends_real_paper`), 403 `role_not_permitted` (`owner_sends_as_advancer`), 404 `paper_not_found` cho nháp của người khác (`mate_sends_owner_draft`); rồi `SELECT … FOR UPDATE` hàng `pair_papers`.
5. `_require_pair_permission("send_pair_paper", …)` (`service.py:7738-7745`; `services/api/app/domain/permissions.py:558-561`), theo thứ tự:
   - `is_draft_owner` → 404 `paper_not_found` `Không có tờ giấy này.` (`mate_sends_sent`; cả người viết phiên bản 2: `mate_sends_own_v2`);
   - `version_current` → 409 `paper_version_stale` `Tờ giấy đã sang phiên bản mới.` (`owner_sends_version_2`, `owner_sends_again_version_2` — đi trước kiểm trạng thái).
6. `pair_paper.chuyen(paper, "gui")` (`services/api/app/domain/pair_paper.py:239-302`; `service.py:7701-7705`):
   - tờ mở đã quá hạn tuần → 409 `paper_expired` `Tuần này hết rồi. Tuần sau mình rủ lại nhé.`;
   - `hieu_luc != "nhap"` → 409 `paper_wrong_state` `Tờ giấy không ở trạng thái làm được việc này.` (`owner_sends_again`, `owner_sends_skipped`, `owner_sends_v2`).

## Đầu vào

- Path `paper_id`. Thân `{"version": <int ≥ 1>}`, `extra="forbid"`.

## Đầu ra

**200** `PaperCommandResponse` `{id, state:"da_gui", version, outing_id:null}` (`schemas.py:2663-2675`). Không có nội dung.

## Tác dụng phụ

Cùng `now`, dưới khoá hàng tờ (`service.py:7747-7759`):

- `pair_paper_versions (paper_id, current_version)`: `sent_at` = `now`, `sent_by` = actor, chỉ khi chưa gửi (`repository.py:8140-8153`).
- `pair_paper_responses`: `paper_id`, `version`, `person_id` = actor, `kind='dong_y'`, `created_at` (`repository.py:8190-8224`). Trigger `pair_paper_response_is_participant` đòi người trả lời có membership `left_at IS NULL` (`c4f27a90d1e3_them_so_hai_nguoi_va_to_giay.py:688-722`).
- `pair_papers.state = 'da_gui'`.
- Tờ tạm gửi được như tờ trong chu kỳ (mọi tờ trong kịch bản là tờ tạm).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["body","version"]` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052-8053` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_version_stale` | `Tờ giấy đã sang phiên bản mới.` | `service.py:8066-8070` |
| 409 | `paper_wrong_state` | `Tờ giấy không ở trạng thái làm được việc này.` | `service.py:8087` |
| 409 | `paper_expired` | `Tuần này hết rồi. Tuần sau mình rủ lại nhé.` | `service.py:8085` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:118-131`
- Service: `services/api/app/api/service.py:7728-7759`, `:7685-7705`, `:8051-8104`
- Repository: `services/api/app/api/repository.py:8088-8090`, `:8140-8173`, `:8190-8224`
- Domain: `services/api/app/domain/pair_paper.py:225-302`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_sending_is_agreeing` (144), `test_sending_a_version_that_has_moved_on_is_refused` (322), `test_a_sent_sheet_is_no_longer_a_draft` (331), `test_only_the_owner_sends_their_own_draft` (633)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_phien_ban_da_gui_khong_sua_duoc` (136), `test_mot_nguoi_mot_dong_y_tren_mot_phien_ban` (324)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-send.yaml` (34 bước, `dev`): `anonymous_sends`, `stranger_sends_unknown`, `mate_sends_owner_draft`, `stranger_sends_real_paper`, `owner_sends_as_advancer`, `owner_sends_version_2`, `owner_version_zero`, `owner_version_string`, `owner_version_float`, `owner_version_bool`, `owner_version_missing`, `owner_sends`, `mate_reads_sent`, `owner_sends_again`, `owner_sends_again_version_2`, `mate_sends_sent`, `owner_sends_skipped`, `owner_sends_with_header_key`, `owner_replays_header_key`, `owner_header_key_other_version`, `mate_counter_proposes`, `owner_sends_v2`, `mate_sends_own_v2`, `owner_reads_third`.

`crossreplay/POST-papers-paper_id-send.yaml` (11 bước). `concurrency/POST-papers-paper_id-send.yaml` (8 bước): chỉ ba lần cùng header key → một lần gửi, hai lần phát lại.

`pair_papers/prod-auth.yaml`: `junk_bearer_sends`, `owner_sends_with_guest_roles_header`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-papers-paper_id-send.yaml`.

## Chưa phủ / lưu ý cho bản Go

- `paper_expired` không phủ (không có bước đồng hồ).
- Kiểm chủ nháp (404) trước phiên bản (409) trước trạng thái (409).
- Khoá hàng tờ phải được lấy **trước** khi đọc trạng thái và phiên bản dùng cho quyết định (xem lỗi dưới); Go không được sao chép cách đọc cũ.
- Burst không khoá idempotency bị bỏ khỏi kịch bản vì kết quả Python phụ thuộc lịch chạy.

## Lỗi Python (chỉ báo, không sửa)

- Đọc cũ dưới khoá: `_locked_paper` đọc tờ qua `session.get` rồi mới `session.get(…, with_for_update=True)`. SQLAlchemy 2.0.36 phát `SELECT … FOR UPDATE` nhưng **không làm mới** đối tượng đã có trong identity map (thiếu `populate_existing`), nên `state`, `current_version` và phiên bản dùng cho quyết định có thể là giá trị trước khi request kia commit. Hai lần gửi cùng lúc: request sau thấy `nhap`, cố `UPDATE` phiên bản đã gửi và bị trigger `pair_paper_versions_immutable` từ chối → 500 (thay vì 409) khi nó đọc trước lúc request đầu commit.
- Người viết phiên bản 2 (đề nghị sửa) bị trả 404 khi «gửi» tờ mình đọc được.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /papers/{paper_id}/send`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.
