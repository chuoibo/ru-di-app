# PATCH /papers/{paper_id}/draft

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Viết lại nháp của chính mình khi nó còn là nháp: ngày, một hoặc hai chặng, lý do. Ghi đè phiên bản 1 tại chỗ; phiên bản đã gửi thì bất biến (trigger), sửa sau khi gửi là việc của «đề nghị sửa».

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 200 lưu và phát lại; khoá đã lưu vẫn phát lại `nhap` sau khi tờ đã gửi (`owner_replays_key_after_send`); nội dung khác cùng khoá → 422 (`owner_header_key_other_body`).
2. JSON hỏng → 422 (`owner_body_broken_json`). Content-type không phải JSON → 422 `model_attributes_type` tại `["body"]` (`owner_body_without_content_type`).
3. `get_actor` → 401 (`anonymous_edits_unknown`), 422.
4. Validation path và thân `PaperDraftEditRequest` → 422, **trước khi tra tờ** (`owner_edits_sent_with_bad_body` là 422 trên tờ đã gửi).
5. `_locked_paper` (`services/api/app/api/service.py:7685-7699`) = `_readable_paper_or_404` rồi `SELECT … FOR UPDATE`:
   - không có → 404 `paper_not_found` (`stranger_edits_unknown`);
   - ngoài pair → 404 `notebook_not_found` (`stranger_edits_real_paper`);
   - role → 403 `role_not_permitted` (`owner_edits_as_advancer`);
   - nháp của người khác → 404 `paper_not_found` (`mate_edits_owner_draft`).
6. `_require_pair_permission("edit_pair_draft", {"is_draft_owner": …})` (`service.py:7711-7715`; `services/api/app/domain/permissions.py:555`): không phải chủ nháp → 404 `paper_not_found` `Không có tờ giấy này.` (ánh xạ `service.py:8053`; `mate_edits_sent`).
7. `hieu_luc != "nhap"` → 409 `paper_wrong_state` `Tờ này đã gửi, sửa thì gửi bản mới.` (`service.py:7717-7720`; `owner_edits_sent`, và cả tờ đã nghỉ tuần: `owner_edits_skipped`).

## Đầu vào

Thân JSON `PaperDraftEditRequest` (`services/api/app/api/schemas.py:2678-2680`), mọi model `extra="forbid"`:

- `content: PaperContentInput` bắt buộc (`schemas.py:2571-2577`):
  - `ngay: date` — pydantic lax: `YYYY-MM-DD`; chuỗi datetime đúng nửa đêm (`2026-09-01T00:00:00`) được nhận, có giờ khác 0 thì 422; số nguyên được hiểu là Unix timestamp (`0` là 1970-01-01, được nhận); `01/09/2026`, `2026-02-30`, `2026-9-1`, `null`, thiếu → 422 (`owner_day_*`). Không kiểm ngày quá khứ.
  - `chang: list[PaperStopInput]` 1..2 phần tử (`owner_no_stops`, `owner_three_stops`, `owner_stops_missing`).
  - mỗi chặng (`schemas.py:2559-2569`): `gio` StrictStr khớp `^([01][0-9]|2[0-3]):[0-5][0-9]$` (`24:00`, `7:30`, `19:00:00`, `"19:00\n"` → 422 `string_pattern_mismatch`; số → 422 `string_type`), `viec` StrictStr 1..200, `place_id: UUID | null` (chuỗi rỗng và chuỗi không phải UUID → 422), `can_kiem: StrictBool = true` (`"true"`, `1` → 422). Trường lạ ở chặng, ở content, ở gốc → 422.
- `ly_do: StrictStr ≤ 200 | null` (mặc định `null`); số → 422; 201 → 422. Service cắt và biến chuỗi trắng thành `null` (`owner_blank_ly_do`, `owner_null_ly_do`).
- `viec` **không** được cắt, không kiểm trắng; HTML, `\u202e`, 200 emoji được lưu nguyên (`owner_task_html`, `owner_task_rtl_override`, `owner_task_emoji_200`, `owner_reads_hostile`).

## Đầu ra

**200** `PaperCommandResponse` `{id, state:"nhap", version:1, outing_id:null}` (`service.py:7726`, `:8314-8320`). Không có nội dung.

## Tác dụng phụ

- Dưới khoá hàng `pair_papers`: `pair_paper_versions (paper_id, 1)` được `UPDATE` `content` = `{ngay: "YYYY-MM-DD", chang:[{gio, viec, place_id: chuỗi hoặc null, can_kiem}]}` (`service.py:8323-8337`) và `ly_do` (`repository.py:8100-8110`). `nguon`, `author_type`, `created_at` không đổi. Không có cột thời gian nào đổi.
- Trigger `pair_paper_versions_immutable` từ chối UPDATE khi `sent_at` đã có (`services/api/app/db/migrations/versions/c4f27a90d1e3_them_so_hai_nguoi_va_to_giay.py:560-585`); kiểm 409 ở service đi trước nên route không chạm tới nó.
- NUL trong `viec` (JSONB) hoặc trong `ly_do` (text) → **500** `text/plain; charset=utf-8` `Internal Server Error`, rollback (`owner_task_nul`, `owner_ly_do_nul`; `owner_reads_after_refusals` thấy nội dung trước đó). Surrogate đơn lẻ → 422 `string_unicode` (`owner_task_lone_surrogate`).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` như `["body","content","chang",0,"gio"]` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052-8053` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_wrong_state` | `Tờ này đã gửi, sửa thì gửi bản mới.` | `service.py:7718-7720` |
| 500 | — | `Internal Server Error` | `services/api/app/api/guest_privacy.py:65` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:102-115`
- Service: `services/api/app/api/service.py:7707-7726`, `:7663-7699`, `:8323-8337`
- Repository: `services/api/app/api/repository.py:8100-8110`, `:8088-8090`
- Schema: `services/api/app/api/schemas.py:2556-2577`, `:2678-2680`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_only_the_owner_edits_the_draft` (136), `test_a_sent_sheet_is_no_longer_a_draft` (331), `test_the_hour_has_to_be_an_hour` (529), `test_three_stops_are_not_a_sheet` (541), `test_a_request_cannot_set_what_the_server_decides` (563)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_phien_ban_da_gui_khong_sua_duoc` (136), `test_ban_nhap_chua_gui_thi_sua_duoc` (168)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/PATCH-papers-paper_id-draft.yaml` (71 bước, `dev`):

- Thứ tự: `anonymous_edits_unknown`, `stranger_edits_unknown`, `mate_edits_owner_draft`, `stranger_edits_real_paper`, `owner_edits_as_advancer`.
- Ghi: `owner_edits_one_stop`, `owner_edits_two_stops`, `owner_reads_two_stops`, `owner_blank_ly_do`, `owner_null_ly_do`, `owner_reads_blank_ly_do`.
- Thân 422 (route bị bộ sinh hoãn): `owner_hour_24`, `owner_hour_one_digit`, `owner_hour_with_seconds`, `owner_hour_number`, `owner_hour_trailing_newline`, `owner_task_empty`, `owner_task_201`, `owner_task_missing`, `owner_no_stops`, `owner_three_stops`, `owner_stops_missing`, `owner_day_missing`, `owner_day_null`, `owner_day_slashes`, `owner_day_impossible`, `owner_day_short_parts`, `owner_day_midnight_datetime`, `owner_day_datetime_with_time`, `owner_day_number`, `owner_place_not_uuid`, `owner_place_empty`, `owner_check_as_string`, `owner_check_as_number`, `owner_extra_stop_field`, `owner_extra_content_field`, `owner_extra_top_field`, `owner_ly_do_201`, `owner_ly_do_number`, `owner_content_null`, `owner_body_empty_object`, `owner_body_broken_json`, `owner_body_without_content_type`.
- Chữ độc: `owner_task_html`, `owner_task_rtl_override`, `owner_task_emoji_200`, `owner_reads_hostile`, `owner_task_nul`, `owner_ly_do_nul`, `owner_task_lone_surrogate`, `owner_reads_after_refusals`.
- Header key: `owner_header_key`, `owner_replays_header_key`, `owner_header_key_other_body`, `owner_replays_key_after_send`.
- Sau gửi: `owner_edits_sent`, `mate_edits_sent`, `owner_edits_sent_with_bad_body`; tờ nghỉ tuần: `owner_edits_skipped`.

`crossreplay/PATCH-papers-paper_id-draft.yaml` (13 bước). `concurrency/PATCH-papers-paper_id-draft.yaml` (9 bước): ba lần sửa giống nhau cùng lúc → ba 200; ba lần cùng header key → một lần sửa, hai lần phát lại.

`pair_papers/prod-auth.yaml`: `owner_dates`, `junk_bearer_edits`.

Corpus sinh tự động: hoãn, `core schema 'date' is not probed`.

## Chưa phủ / lưu ý cho bản Go

- Parse `date` phải chép pydantic-core (lax): chấp nhận Unix timestamp số nguyên và datetime đúng nửa đêm; thông điệp lỗi (`date_from_datetime_inexact`, `date_from_datetime_parsing`, …) nguyên văn.
- Regex `gio` chạy trong pydantic-core (Rust `regex`), không phải Python `re`.
- `viec` không cắt; `ly_do` cắt rồi rỗng → `null`.
- Kiểm chủ nháp trả 404 (không 403) và đi sau `may_view_paper`: người kia trên tờ đã gửi (đọc được) vẫn nhận 404.
- 500 cho NUL là hành vi tham chiếu; surrogate đơn lẻ là 422 `string_unicode`.

## Lỗi Python (chỉ báo, không sửa)

- NUL trong `viec`/`ly_do` là 500 thay vì 422.
- Người kia sửa một tờ đã gửi mà họ đọc được nhận 404 `Không có tờ giấy này.` — tờ vẫn hiện trên màn của họ.
- Nhận ngày quá khứ, và số nguyên như `0` thành 1970-01-01.
- Khoá idempotency đã lưu phát lại `state: "nhap"` sau khi tờ đã gửi.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `PATCH /papers/{paper_id}/draft`: Route này nhận `place_id` dạng slug (trước 422); UUID không còn chuẩn hoá chữ. Đọc lại tờ: như các route đọc.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `PATCH /papers/{paper_id}/draft`: Route này đọc/ghi tờ qua `_readable_paper_or_404`: với tờ chưa từng gửi ở trạng thái đóng, người không phải chủ nay nhận 404 `paper_not_found` (trước: đọc được, hoặc lỗi trạng thái). Tờ đã gửi: byte không đổi.
