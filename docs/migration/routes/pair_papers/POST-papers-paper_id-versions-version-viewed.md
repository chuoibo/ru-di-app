# POST /papers/{paper_id}/versions/{version}/viewed

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

«Đã xem» (spec §3.3 luật 4, §7.5): ghi lần mở đầu tiên của người nhận cho một phiên bản. Chỉ người gửi phiên bản đó thấy mốc này (`viewed_by_recipient_at`). Xem phiên bản hiện hành của tờ `da_gui` chuyển tờ sang `da_xem`, điều làm người gửi mất quyền rút.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 204 lưu và phát lại (`owner_replays_header_key`); phiên bản khác trên path cùng khoá → 422 (fingerprint gồm path; `owner_header_key_other_version`); 409 nhả khoá (`crossreplay/…viewed.yaml` `python_owner_self_refused`).
2. `get_actor` → 401 (`anonymous_views`), 422.
3. Validation path: `paper_id` UUID, `version: int` lax của pydantic. Kịch bản thử các cách viết số 9 (không có phiên bản 9; `mate_version_*`): `09`, `%2B9` (`+9`), `-9`, `%209` (` 9`), `9.0` và `9_0` (thành 90) đều parse được và tới bước 5 → 404; `9e0`, `chin` và chữ số toàn chiều rộng `%EF%BC%99` → 422 `int_parsing` `Input should be a valid integer, unable to parse string as an integer`.
4. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_views_unknown`), 404 `notebook_not_found` (`stranger_views_real_paper`), 403 `role_not_permitted` của `view_pair_paper` (`mate_views_as_advancer`), 404 `paper_not_found` cho nháp của người khác (`mate_views_owner_draft`).
5. Phiên bản không có → 404 `paper_not_found` `Không có phiên bản này.` (`service.py:7767-7768`; `mate_views_version_9`, `mate_views_version_0`).
6. `_require_pair_permission("view_pair_paper_as_recipient", {"is_not_version_sender": row.sent_by != actor})` (`service.py:7769-7773`; `services/api/app/domain/permissions.py:565-568`): role đã qua ở bước 4; người gửi phiên bản đó → 409 `paper_self_response` `Đây là tờ bạn gửi, chờ người kia trả lời.` (`owner_views_own_sent`, `mate_views_own_v2`, `owner_views_own_v1`).
7. Nháp chưa gửi có `sent_by` NULL, nên chủ nháp qua được bước 6 (`owner_views_own_draft` là 204).

## Đầu vào

- Path `paper_id`, `version`. Không thân.

## Đầu ra

**204**, thân rỗng.

## Tác dụng phụ

Dưới khoá hàng tờ (`service.py:7774-7778`):

- `pair_paper_views (paper_id, version, person_id)`: chèn `seen_at` = `now` nếu chưa có; có rồi thì giữ lần đầu (`repository.py:8175-8188`; `mate_views_again`, `owner_reads_first_look_kept`).
- Nếu `version == current_version` **và** cột `state == 'da_gui'`: `chuyen(paper, "xem")` → `state='da_xem'`. Tờ mở quá hạn → 409 `paper_expired` (view vừa chèn bị rollback). Phiên bản cũ, tờ `da_xem`, tờ đã khép: chỉ ghi view (`mate_views_old_v1`, `owner_views_cancelled` trên tờ `huy`).
- `idempotency_keys` khi có header và 204. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["path","version"]` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` / `Không có phiên bản này.` | `service.py:7668`, `:7768`, `:8052` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_self_response` | `Đây là tờ bạn gửi, chờ người kia trả lời.` | `service.py:8071-8075` |
| 409 | `paper_expired` | `Tuần này hết rồi. Tuần sau mình rủ lại nhé.` | `service.py:8085` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:134-148`
- Service: `services/api/app/api/service.py:7761-7778`
- Repository: `services/api/app/api/repository.py:8175-8188`, `:8155-8173`
- Domain: `services/api/app/domain/pair_paper.py:225-302`
- Bảng: `services/api/app/db/models.py:3298-3324`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_the_view_mark_is_the_senders_to_see` (168), `test_the_sender_cannot_mark_their_own_sheet_seen` (196), `test_withdrawal_stops_the_moment_they_have_looked` (351), `test_a_version_that_does_not_exist_is_not_a_version` (591)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_moc_xem_la_mot_hang_moi_nguoi_moi_phien_ban` (415)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-versions-version-viewed.yaml` (42 bước, `dev`): `anonymous_views`, `stranger_views_unknown`, `owner_views_own_draft`, `mate_views_owner_draft`, `stranger_views_real_paper`, `owner_views_own_sent`, `mate_views_as_advancer`, `mate_views_version_9`, `mate_views_version_0`, `mate_views`, `owner_reads_seen`, `mate_views_again`, `owner_reads_first_look_kept`, chín bước `mate_version_*`, `mate_counter_proposes`, `mate_views_own_v2`, `owner_views_v2`, `owner_views_own_v1`, `mate_views_old_v1`, `mate_reads_two_versions`, `owner_views_v2_with_header_key`, `owner_replays_header_key`, `owner_header_key_other_version`, `mate_skips`, `owner_views_cancelled`.

`crossreplay/…viewed.yaml` (12 bước). `concurrency/…viewed.yaml` (10 bước): ba lần xem cùng lúc → ba 204, một hàng view, một lần sang `da_xem`; ba lần cùng header key → một 204, hai lần phát lại.

`pair_papers/prod-auth.yaml`: `junk_bearer_views`, `mate_views`, `owner_views_v2`.

Corpus sinh tự động: hoãn, `path version is int`.

## Chưa phủ / lưu ý cho bản Go

- Parse `version` phải chép pydantic-core lax int từ chuỗi: nhận khoảng trắng hai đầu, dấu `+`/`-`, số 0 đứng đầu, gạch dưới giữa chữ số (`9_0` = 90) và phần thập phân bằng 0 (`9.0` = 9); từ chối số mũ và chữ số ngoài ASCII; thông điệp `int_parsing` nguyên văn.
- Role (403) kiểm trong `_locked_paper`, trước khi tra phiên bản (404); `is_not_version_sender` (409) sau cùng.
- Chuyển `da_xem` dựa vào cột `state`, không `hieu_luc`.
- `paper_expired` không phủ.

## Lỗi Python (chỉ báo, không sửa)

- Chủ nháp «xem» được nháp chưa gửi của mình (204, ghi một hàng view): `is_not_version_sender` coi `sent_by` NULL là «người khác gửi».
- Xem một tờ đã khép vẫn ghi hàng view.
- Đọc cũ dưới khoá (xem thẻ `POST /papers/{id}/send`): kết quả HTTP của route này không đổi, nhưng lệnh `UPDATE state` có thể chạy lại với giá trị cũ.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /papers/{paper_id}/versions/{version}/viewed`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.
