# POST /papers/{paper_id}/skip

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

«Tuần này nghỉ», ai trong hai người cũng nói được. Nháp chưa ai nhận thì bỏ cho tuần đó (`nghi_tuan`); tờ đã gửi thì huỷ (`huy`) vì người kia đã thấy một điều và cần một kết thúc (spec §3.3).

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 200 lưu và phát lại (`owner_replays_header_key`); cùng khoá dưới actor khác chạy thật (`mate_uses_owner_header_key` → 409); thân `{}` cùng khoá → 422 (`crossreplay/…skip.yaml`).
2. `get_actor` → 401 (`anonymous_skips`), 422. Thân bị bỏ qua (`mate_skips_sent_with_body`).
3. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_skips_unknown`), 404 `notebook_not_found` (`stranger_skips_real_paper`), 403 role (`owner_skips_as_advancer`), 404 cho nháp của người khác (`mate_skips_owner_draft`).
4. `_require_pair_permission("skip_pair_week", {"is_group_member": True})` (`service.py:7978`; `services/api/app/domain/permissions.py:589`): chỉ còn role.
5. `chuyen(paper, "nghi_tuan")` (`services/api/app/domain/pair_paper.py:239-302`): tờ mở quá hạn → 409 `paper_expired`; `hieu_luc` ngoài `OPEN_STATES` → 409 `paper_wrong_state` `Tờ giấy không ở trạng thái làm được việc này.` (`owner_skips_again`, `owner_skips_plan`, `mate_skips_withdrawn`, `mate_uses_owner_header_key`).

## Đầu vào

- Path `paper_id`. Không thân.

## Đầu ra

**200** `PaperCommandResponse` `{id, state, version, outing_id:null}`, `state` = `nghi_tuan` cho nháp (`owner_skips_draft`), `huy` cho tờ `da_gui` hoặc `da_xem` (`mate_skips_sent_with_body`, `owner_skips_seen`).

## Tác dụng phụ

- `pair_papers.state` = `nghi_tuan` hoặc `huy` (`repository.py:8155-8173`). Không đổi phiên bản, phản hồi, view.
- Tờ `nghi_tuan` không còn là `nhap` nên người kia đọc được nháp đã bỏ (`mate_reads_skipped_draft`).
- Tuần không bị «dùng hết»: xin tờ mới trong cùng tuần được (thẻ `POST …/papers/draft`).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_wrong_state` | `Tờ giấy không ở trạng thái làm được việc này.` | `service.py:8087` |
| 409 | `paper_expired` | `Tuần này hết rồi. Tuần sau mình rủ lại nhé.` | `service.py:8085` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:185-197`
- Service: `services/api/app/api/service.py:7974-7982`
- Domain: `services/api/app/domain/pair_paper.py:86`, `:225-302`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_either_of_them_may_call_the_week_off` (397), `test_a_closed_notebook_refuses_the_whole_sheet` (610)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-skip.yaml` (34 bước, `dev`; mọi tờ là tờ tạm): `anonymous_skips`, `stranger_skips_unknown`, `mate_skips_owner_draft`, `stranger_skips_real_paper`, `owner_skips_as_advancer`, `owner_skips_draft`, `owner_skips_again`, `mate_reads_skipped_draft`, `mate_skips_sent_with_body`, `owner_skips_seen`, `owner_skips_plan`, `owner_skips_with_header_key`, `owner_replays_header_key`, `mate_uses_owner_header_key`, `mate_skips_withdrawn`, `owner_lists_end`.

`crossreplay/POST-papers-paper_id-skip.yaml` (10 bước). `concurrency/POST-papers-paper_id-skip.yaml` (9 bước): chỉ ba lần cùng header key.

`pair_papers/prod-auth.yaml`: `junk_bearer_skips`, `owner_skips_mates_draft`, `mate_skips`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-papers-paper_id-skip.yaml`.

## Chưa phủ / lưu ý cho bản Go

- `paper_expired` không phủ. `de_nghi_sua` và `dong_y` là trạng thái mở nhưng không lưu được qua cửa HTTP của lát 1.
- Người kia chỉ nghỉ được tờ đã gửi; nháp của người khác là 404, không phải 409.

## Lỗi Python (chỉ báo, không sửa)

- Đọc cũ dưới khoá (xem thẻ `POST /papers/{id}/send`): hai lần nghỉ cùng lúc không khoá idempotency có thể cùng 200 thay vì 200 + 409.
- Người kia đọc được nháp đã `nghi_tuan` (nội dung người kia chưa bao giờ được gửi).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /papers/{paper_id}/skip`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.
