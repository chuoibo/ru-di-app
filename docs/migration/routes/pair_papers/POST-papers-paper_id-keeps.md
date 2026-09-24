# POST /papers/{paper_id}/keeps

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Giữ lại một dòng sau buổi đi (spec §7.4). Dòng đầu tiên biến `da_di` thành `da_giu`, kết thúc tốt của tuần; mỗi dòng sau đó cũng được giữ.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 201 lưu và phát lại (`owner_replays_header_key`); dòng khác cùng khoá → 422 (`owner_header_key_other_line`); 422 validation nhả khoá (`crossreplay/…keeps.yaml` `front_blank_refused`, `python_same_key_valid`).
2. JSON hỏng → 422. `get_actor` → 401 (`anonymous_keeps`), 422.
3. Validation `PaperKeepRequest` (`services/api/app/api/schemas.py:2710-2719`) **trước khi tra tờ** (`stranger_keeps_unknown_blank`).
4. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_keeps_unknown`), 404 `notebook_not_found` (`stranger_keeps_real_paper`), 403 role (`owner_keeps_as_advancer`), 404 cho nháp của người khác (`mate_keeps_owner_draft`).
5. `_require_pair_permission("keep_pair_paper_line", {"is_group_member": True})` (`service.py:8011-8013`; `services/api/app/domain/permissions.py:591`).
6. `chuyen(paper, "giu")` (`services/api/app/domain/pair_paper.py:225-302`): `hieu_luc` ngoài `da_di, da_giu` → 409 `paper_wrong_state` `Tờ giấy không ở trạng thái làm được việc này.` (`owner_keeps_draft`, `owner_keeps_plan`, `owner_keeps_skipped`).

## Đầu vào

- Path `paper_id`.
- Thân JSON, `extra="forbid"`: `line: StrictStr` 1..200 trên chuỗi thô, rồi `field_validator` `str.strip()` và từ chối rỗng (`Value error, line must not be blank`):
  - `""` → `string_too_short`; `"   "`, `"\n\n\t"`, `"\u00a0"` → `value_error` (`owner_line_empty`, `owner_line_blank`, `owner_line_newlines`, `owner_line_nbsp`);
  - 201 → `string_too_long`; 199 ký tự bọc hai dấu cách mỗi bên (203) → `string_too_long` (`owner_line_201`, `owner_line_raw_over_bound`);
  - số, `null`, thiếu, trường lạ → 422 (`owner_line_number`, `owner_line_null`, `owner_line_missing`, `owner_line_extra_field`).

## Đầu ra

**201** `PaperKeepResponse` (`schemas.py:2611-2614`), thứ tự `id`, `line` (đã cắt), `created_at` (ISO-8601 UTC đuôi `Z`).

## Tác dụng phụ

- `pair_paper_keeps (id uuid4, paper_id, person_id = actor, line, created_at)` (`repository.py:8245-8258`); CHECK `line ~ '[^[:space:]]'` (`services/api/app/db/models.py:3405-3437`).
- `pair_papers.state='da_giu'` (lần đầu đổi, các lần sau ghi lại cùng giá trị).
- Dòng chỉ gồm `\u200b` qua được cả `str.strip()` lẫn CHECK và được lưu (`owner_keeps_zero_width`); HTML, `\u202e`, 200 emoji lưu nguyên văn (`owner_keeps_html`, `owner_keeps_rtl_override`, `owner_keeps_emoji_200`).
- NUL → **500** `text/plain; charset=utf-8` `Internal Server Error`, rollback (`owner_keeps_nul`); surrogate đơn lẻ → 422 `string_unicode` (`owner_keeps_lone_surrogate`).
- `GET …/papers` lấy dòng sớm nhất làm `dong_giu_dau` (`mate_lists_first_line`).
- Không kiểm chu kỳ: sổ đã đóng vẫn nhận dòng (`POST-papers-paper_id-done.yaml` `owner_keeps_after_close`).
- `idempotency_keys` khi có header và 201. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["body","line"]` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_wrong_state` | `Tờ giấy không ở trạng thái làm được việc này.` | `service.py:8087` |
| 500 | — | `Internal Server Error` | `services/api/app/api/guest_privacy.py:65` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:216-230`
- Service: `services/api/app/api/service.py:8006-8021`
- Repository: `services/api/app/api/repository.py:8245-8258`, `:8155-8173`
- Schema: `services/api/app/api/schemas.py:2611-2614`, `:2710-2719`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_a_line_kept_closes_the_sheet` (465), `test_a_line_of_spaces_is_not_a_line` (494), `test_keeping_a_line_needs_an_evening_that_happened` (506)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_dong_giu_lai_khong_duoc_rong` (405)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-keeps.yaml` (44 bước, `dev`; tờ tạm):

- Thứ tự và trạng thái: `anonymous_keeps`, `stranger_keeps_unknown`, `stranger_keeps_unknown_blank`, `owner_keeps_draft`, `mate_keeps_owner_draft`, `owner_keeps_plan`, `stranger_keeps_real_paper`, `owner_keeps_as_advancer`, `owner_keeps_skipped`.
- Thân 422 (route bị bộ sinh hoãn): `owner_line_empty`, `owner_line_blank`, `owner_line_newlines`, `owner_line_nbsp`, `owner_line_201`, `owner_line_raw_over_bound`, `owner_line_number`, `owner_line_null`, `owner_line_missing`, `owner_line_extra_field`.
- Ghi: `mate_keeps_padded`, `owner_keeps_200`, `owner_keeps_zero_width`, `owner_keeps_html`, `owner_keeps_rtl_override`, `owner_keeps_emoji_200`, `owner_keeps_nul`, `owner_keeps_lone_surrogate`, `owner_reads_keeps`, `mate_lists_first_line`.
- Header key: `owner_keeps_with_header_key`, `owner_replays_header_key`, `owner_header_key_other_line`.

`crossreplay/POST-papers-paper_id-keeps.yaml` (17 bước). `concurrency/POST-papers-paper_id-keeps.yaml` (13 bước): ba dòng cùng lúc → ba 201, ba hàng; ba lần cùng header key → một hàng, hai lần phát lại.

`pair_papers/prod-auth.yaml`: `anonymous_keeps_blank` (401 `Missing bearer session` trước validation thân), `junk_bearer_keeps`, `owner_keeps`.

Corpus sinh tự động: hoãn, `core schema 'function-after' is not probed`.

## Chưa phủ / lưu ý cho bản Go

- Cắt theo `str.strip()` của Python, bound trên chuỗi thô, độ dài theo code point; CHECK Postgres là lớp hai.
- 500 cho NUL là hành vi tham chiếu; surrogate đơn lẻ là 422 `string_unicode`.

## Lỗi Python (chỉ báo, không sửa)

- NUL trong `line` là 500 thay vì 422.
- Dòng chỉ có ký tự vô hình (`\u200b`) được giữ như một kỷ niệm, trái với lý do CHECK tồn tại.
- Sổ đã đóng vẫn nhận dòng giữ lại.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /papers/{paper_id}/keeps`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /papers/{paper_id}/keeps`: Route này đọc/ghi tờ qua `_readable_paper_or_404`: với tờ chưa từng gửi ở trạng thái đóng, người không phải chủ nay nhận 404 `paper_not_found` (trước: đọc được, hoặc lỗi trạng thái). Tờ đã gửi: byte không đổi.
