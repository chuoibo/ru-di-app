# PUT /contexts/{context_id}/notebook/constraints/{kind}

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Một trong hai dòng một người viết về chính mình trong vùng chung của sổ (spec §6.4, ADR-0027 K7): `khong_an_duoc` («Không ăn được») hoặc `dung` («Đừng»). Người kia đọc được, không sửa được. Mỗi lần ghi tăng `version` của dòng.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): nội dung khác cùng khoá → 422 `idempotency_key_reuse` (`owner_header_key_other_content`); 422 validation nhả khoá (`crossreplay/PUT-…` `python_blank_refused` rồi `front_same_key_valid`).
2. JSON hỏng → 422 (`owner_body_broken_json`).
3. `get_actor` → 401 (`anonymous_puts`), 422.
4. Validation path `context_id` và thân `PairConstraintPutRequest` (`services/api/app/api/schemas.py:2767-2783`) — **trước kind và context**: `stranger_unknown_kind_blank_body` là 422 dù kind lạ và context lạ.
5. Kind ngoài `CONSTRAINT_KINDS` (so chính xác trên path đã giải mã) → 404 `constraint_kind_unknown` `Không có ô này.` (`services/api/app/api/service.py:7504-7505`; `stranger_unknown_kind_unknown_context`, `owner_kind_uppercase`, `owner_kind_padded`). `khong%5Fan%5Fduoc` là `khong_an_duoc` (`owner_kind_encoded`).
6. `_pair_context_or_404` → 404 `notebook_not_found` (`stranger_puts_in_pair`, `owner_puts_in_group`).
7. `_require_pair_permission("edit_pair_constraint", {"is_self": True})` (`service.py:7507`; `services/api/app/domain/permissions.py:596`) → 403 `role_not_permitted` (`owner_puts_as_advancer`).
8. `_locked_notebook`; không có chu kỳ sống → 409 `cycle_not_active` `Sổ chưa mở.` (`service.py:7510-7511`; `owner_puts_on_fresh_pair`, `owner_puts_after_close`). Chu kỳ `pending` **được** ghi (`owner_puts_while_pending`).

## Đầu vào

- Path `context_id` (UUID), `kind` (chuỗi).
- Thân JSON, `extra="forbid"`: `content: StrictStr` 1..200 ký tự, rồi `field_validator` cắt khoảng trắng hai đầu (`str.strip()`, gồm tab, xuống dòng, NBSP) và từ chối chuỗi rỗng sau khi cắt:
  - bound 1..200 kiểm trên chuỗi **thô**, trước khi cắt: 199 ký tự bọc 3 dấu cách mỗi bên (205) → 422 `string_too_long` (`owner_content_raw_over_bound`); 200 → 200 (`owner_content_200`); 201 → 422 (`owner_content_201`); `""` → 422 `string_too_short` (`owner_content_empty`);
  - `"   "`, `"\n\t "`, `"\u00a0\u00a0"` → 422 `value_error` `Value error, content must not be blank` (`owner_content_blank`, `owner_content_tabs_newlines`, `owner_content_nbsp_only`);
  - số, `null`, thiếu, trường lạ, mảng → 422 (`owner_content_number`, `owner_content_null`, `owner_content_missing`, `owner_content_extra_field`, `owner_body_array`);
  - độ dài tính theo code point: 200 emoji → 200 (`owner_content_emoji_200`);
  - `\u200b` một mình không phải khoảng trắng với `str.strip()` → lưu (`owner_content_zero_width_only`); HTML và `\u202e` lưu nguyên văn (`owner_content_html`, `owner_content_rtl_override`).
- Header `Idempotency-Key` tuỳ chọn.

## Đầu ra

**200** `PairConstraintResponse` (`schemas.py:2727-2731`), thứ tự khoá `owner_id`, `kind`, `content` (đã cắt), `version` (số nguyên). Không có trường thời gian.

## Tác dụng phụ

Dưới khoá hàng sổ, `repository.py:7926-7957`:

- Chưa có hàng `(cycle_id, owner_id=actor, kind)` → chèn `content`, `version=1`, `updated_at` = `now`.
- Đã có → `content` mới, `version += 1`, `updated_at` = `now` (`owner_puts_again`, `owner_puts_padded`). Ghi lại cùng nội dung vẫn tăng version.
- Hàng thuộc chu kỳ: sau khi đóng sổ và mở chu kỳ mới, dòng cũ không còn hiện (`GET …/notebook` chỉ đọc chu kỳ sống).
- CHECK `length(content) <= 200` và `kind IN (...)` (`services/api/app/db/models.py:3439-3470`).
- NUL (`\u0000`) qua được pydantic nhưng không ghi được vào cột `text` (psycopg từ chối) → **500** `text/plain; charset=utf-8` `Internal Server Error`, rollback (`owner_content_nul`). Surrogate đơn lẻ (`\ud800`) bị pydantic từ chối trước: 422 `string_unicode` `Input should be a valid string, unable to parse raw data as a unicode string` (`owner_content_lone_surrogate`).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["body","content"]` | `main.py:318-351` |
| 404 | `constraint_kind_unknown` | `Không có ô này.` | `service.py:7505` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `cycle_not_active` | `Sổ chưa mở.` | `service.py:7511` |
| 500 | — | `Internal Server Error` (`text/plain`) | `services/api/app/api/guest_privacy.py:65` |
| 422 | idempotency | như các POST | `idempotency.py` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:118-133`
- Service: `services/api/app/api/service.py:7496-7524`
- Repository: `services/api/app/api/repository.py:7926-7957`
- Schema: `services/api/app/api/schemas.py:2727-2731`, `:2767-2783`
- Bảng: `services/api/app/db/models.py:3439-3470`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_both_may_read_the_two_lines_and_only_their_owner_writes_one` (185), `test_an_unknown_constraint_kind_is_not_a_door` (231), `test_a_blank_constraint_is_not_a_constraint` (242)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/PUT-contexts-context_id-notebook-constraints-kind.yaml` (49 bước, `dev`):

- Thứ tự: `anonymous_puts`, `stranger_unknown_kind_unknown_context`, `stranger_unknown_kind_blank_body`, `owner_puts_on_fresh_pair`, `stranger_puts_in_pair`, `owner_puts_in_group`, `owner_puts_as_advancer`.
- Chu kỳ: `owner_puts_while_pending`, `owner_puts_again`, `owner_puts_dung`, `mate_puts_khong_an_duoc`, `owner_puts_padded`, `mate_reads_constraints`, `owner_puts_after_close`.
- Kind trên path: `owner_kind_uppercase`, `owner_kind_padded`, `owner_kind_encoded`.
- Thân: `owner_content_200`, `owner_content_201`, `owner_content_raw_over_bound`, `owner_content_blank`, `owner_content_empty`, `owner_content_tabs_newlines`, `owner_content_nbsp_only`, `owner_content_number`, `owner_content_null`, `owner_content_missing`, `owner_content_extra_field`, `owner_body_array`, `owner_body_broken_json`.
- Chữ độc: `owner_content_zero_width_only`, `owner_content_html`, `owner_content_rtl_override`, `owner_content_emoji_200`, `owner_content_nul`, `owner_content_lone_surrogate`.
- Header key: `owner_header_key`, `owner_replays_header_key`, `owner_header_key_other_content`.

`crossreplay/PUT-…-constraints-kind.yaml` (15 bước). `concurrency/PUT-…-constraints-kind.yaml` (9 bước): ba lần ghi cùng lúc → version 1, 2, 3 (đọc sau khoá, không mất lần ghi); ba lần cùng header key → một lần ghi, hai lần phát lại.

`pair_notebooks/prod-auth.yaml`: `owner_puts_with_advancer_header`, `mate_puts`, `junk_bearer_puts`, `anonymous_puts_unknown_kind`, `stranger_puts`.

Corpus sinh tự động: hoãn, `path kind is str` (sau đó thân còn `field_validator`).

## Chưa phủ / lưu ý cho bản Go

- Thứ tự: JSON → actor → validation (path + thân) → kind → pair → role → chu kỳ.
- Bound độ dài trên chuỗi thô, rồi cắt theo định nghĩa khoảng trắng của Python `str.strip()` (Unicode White_Space cộng các ký tự điều khiển \x1c-\x1f), không phải của Go `strings.TrimSpace` (Go không cắt \x1c-\x1f). Thông điệp `Value error, content must not be blank`.
- Độ dài theo code point, không theo byte hay UTF-16.
- 500 cho NUL là hành vi tham chiếu (cùng status, `text/plain; charset=utf-8`, thân `Internal Server Error`); surrogate đơn lẻ trong chuỗi JSON là 422 `string_unicode` với `loc` của trường.

## Lỗi Python (chỉ báo, không sửa)

- NUL trong `content` là 500 thay vì 422.
- Bound 200 tính trước khi cắt: một dòng hợp lệ có khoảng trắng thừa bị từ chối.
- Ghi lại cùng nội dung vẫn tăng `version`.
- Ghi được ràng buộc khi chu kỳ mới `pending` (chỉ một người đồng ý lập sổ), trái với câu «Sổ chưa mở.» mà nhánh không có chu kỳ trả.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `PUT /contexts/{context_id}/notebook/constraints/{kind}`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `PUT /contexts/{context_id}/notebook/constraints/{kind}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
