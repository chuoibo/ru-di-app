# GET /contexts/{context_id}/papers

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Danh sách mọi tờ giấy người này được thấy trong một pair, mới nhất trước, đã áp hạn tuần. Mỗi dòng mang đúng hai mẩu mà dòng khép trên màn cần (ngày, chặng đầu, dòng giữ lại đầu tiên) để màn không phải tải từng tờ.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Router: đuôi `/` → 307 (`owner_lists_trailing_slash`); query lạ bỏ qua (`owner_lists_with_query`).
2. `get_actor` → 401 (`anonymous_lists`), 422.
3. Validation `context_id` → 422.
4. `_pair_context_or_404` (`services/api/app/api/service.py:7215-7237`) → 404 `notebook_not_found` (`stranger_lists_unknown`, `stranger_lists_pair`, `owner_lists_group`).
5. `_require_pair_permission("view_pair_notebook", {"is_group_member": True})` (`service.py:7590`; `services/api/app/domain/permissions.py:525`) → 403 `role_not_permitted` (`owner_lists_as_advancer`).

## Đầu vào

- Path `context_id`. Không query, không thân.

## Đầu ra

**200** `PaperListResponse` (`services/api/app/api/schemas.py:2659-2660`): `{"papers":[…]}`. Pair chưa có tờ → `{"papers":[]}` (`owner_lists_fresh`).

Thứ tự `created_at DESC, id` (`repository.py:8092-8098`). Bỏ tờ có `hieu_luc == "nhap"` mà `draft_owner_id != actor` (`service.py:7596-7597`; `mate_lists_without_draft`, `owner_lists_all` so với `mate_lists_all`). Mỗi phần tử `PaperSummary` (`schemas.py:2638-2656`), thứ tự khoá:

1. `id`
2. `state` — `hieu_luc` (quá hạn tuần và còn mở → `het_han`).
3. `version` — `current_version`.
4. `tuan` — ngày thứ Hai của tuần theo `Asia/Ho_Chi_Minh`, dạng `YYYY-MM-DD`.
5. `ngay` — `content.ngay` của phiên bản hiện hành, dạng `YYYY-MM-DD`, hoặc `null` nếu không đọc được (`service.py:8193-8204`).
6. `expires_at` — nửa đêm cuối Chủ nhật giờ Việt Nam, viết UTC đuôi `Z` (`services/api/app/domain/pair_paper.py:118-133`), tức `…T17:00:00Z` của Chủ nhật.
7. `chang_dau` — `{gio, viec}` của chặng đầu phiên bản hiện hành, hoặc `null` (`service.py:8206-8224`).
8. `dong_giu_dau` — `line` của dòng giữ lại sớm nhất (`created_at, id`), hoặc `null` (`owner_lists_kept`).

## Tác dụng phụ

Không ghi, không khoá. Không tạo hàng sổ.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:55-68`
- Service: `services/api/app/api/service.py:7585-7609`, `:8193-8224`
- Repository: `services/api/app/api/repository.py:8092-8098`, `:7969-8041`
- Domain: `services/api/app/domain/pair_paper.py:150-164`
- Schema: `services/api/app/api/schemas.py:2633-2660`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_a_draft_is_not_a_sent_sheet` (121), `test_the_list_carries_the_one_line_a_closed_row_shows` (709), `test_a_sheet_the_list_cannot_read_does_not_take_the_list_down` (743), `test_every_pair_route_refuses_a_stranger` (660)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/GET-contexts-context_id-papers.yaml` (35 bước, `dev`; mọi tờ là tờ tạm, sổ không bao giờ mở):

- Thứ tự: `anonymous_lists`, `stranger_lists_unknown`, `stranger_lists_pair`, `owner_lists_group`, `owner_lists_as_advancer`.
- Hiển thị: `owner_lists_fresh`, `owner_lists_own_draft`, `mate_lists_without_draft`, `owner_lists_edited` (hai chặng, ngày đã sửa), `mate_lists_sent`, `owner_lists_kept` (dòng giữ lại đầu là của mate), `owner_lists_all`, `mate_lists_all` (bốn tờ: `da_giu`, `nghi_tuan`, `rut`, nháp của mate chỉ mate thấy).
- Framework: `owner_lists_with_query`, `owner_lists_trailing_slash`.

`pair_papers/prod-auth.yaml`: `anonymous_lists`, `junk_bearer_lists`, `mate_lists`, `owner_lists_end`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/get-contexts-context_id-papers.yaml`.

## Chưa phủ / lưu ý cho bản Go

- `het_han` và `ngay: null` / `chang_dau: null` (hàng JSON hỏng) không phủ: không có bước đồng hồ, không có cửa HTTP ghi nội dung hỏng.
- `tuan` và `ngay` là chuỗi ngày, không phải timestamp; `expires_at` luôn là `…T17:00:00Z` (không phần thập phân).
- Thứ tự `created_at DESC, id ASC`: hai tờ cùng `created_at` hiếm nhưng khi xảy ra, `id` tăng dần.

## Lỗi Python (chỉ báo, không sửa)

- Không thấy lỗi riêng của route. (Nháp giấu ở đây nhưng lộ qua `POST …/papers/draft` 409 và preview đóng sổ, xem các thẻ đó.)

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `GET /contexts/{context_id}/papers`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `GET /contexts/{context_id}/papers`: Route này lọc danh sách: tờ chưa từng gửi không còn hiện cho người không phải chủ, ở mọi trạng thái.
