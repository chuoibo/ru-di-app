# GET /papers/{paper_id}

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Một tờ giấy với mọi thứ đã viết trên nó, nhìn từ người đang đọc: mọi phiên bản, câu trả lời của chính mình, người kia đã «ừ» chưa, người nhận đã xem lúc nào (chỉ người gửi thấy), buổi đi sinh ra, và các dòng giữ lại. Quyền kiểm lúc đọc, không lúc lệnh trả lời (ADR-0027 §3).

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Router: đuôi `/` → 307 (`owner_reads_trailing_slash`).
2. `get_actor` → 401 (`anonymous_reads_unknown`), 422.
3. Validation `paper_id` (UUID lax; chữ hoa được nhận, `owner_reads_uppercase_id`) → 422.
4. `_readable_paper_or_404` (`services/api/app/api/service.py:7663-7679`):
   - tờ không có → 404 `paper_not_found` `Không có tờ giấy này.` (`stranger_reads_unknown`);
   - `_pair_context_or_404(paper.context_id)` → 404 `notebook_not_found` `Không có sổ này.` cho người ngoài pair (`stranger_reads_real_paper`, `stranger_reads_real_paper_as_advancer`);
   - `_require_pair_permission("view_pair_paper", {"may_view_paper": state != "nhap" or draft_owner == actor})` (`services/api/app/domain/permissions.py:554`): role trước → 403 `role_not_permitted` (`mate_reads_owner_draft_as_advancer`); rồi `may_view_paper` → 404 `paper_not_found` `Không có tờ giấy này.` (`service.py:8052`; `mate_reads_owner_draft`). `may_view_paper` đọc **cột** `state`, không `hieu_luc`.

## Đầu vào

- Path `paper_id`. Không query, không thân.

## Đầu ra

**200** `PaperResponse` (`services/api/app/api/schemas.py:2617-2630`; `service.py:8287-8311`), thứ tự khoá:

1. `id`
2. `state` — `hieu_luc`.
3. `version` — `current_version`.
4. `author_type` — của phiên bản hiện hành.
5. `sent_by` — người gửi phiên bản hiện hành hoặc `null` (nháp).
6. `tuan` — `YYYY-MM-DD`.
7. `expires_at` — `…T17:00:00Z`.
8. `outing_id` — từ `pair_paper_outings`, hoặc `null`.
9. `co_the_ghi_da_di` — `hieu_luc == "chot"` và ngày hôm nay theo giờ Việt Nam ≥ `content.ngay` (`service.py:8227-8237`; `mate_reads_plan` là `true` với ngày đã qua).
10. `versions` — theo `version` tăng dần; mỗi phần tử `PaperVersionResponse` (`schemas.py:2592-2608`; `service.py:8240-8284`):
    `version`, `content` (`{ngay, chang:[{gio, viec, place_id, can_kiem}]}`, `place_id` là UUID hoặc `null`), `ly_do`, `author_type`, `sent_at`, `sent_by`, `my_response` (câu trả lời **cuối** của người đọc cho phiên bản đó, theo `created_at, id`: `dong_y` / `de_nghi_sua` / `null`), `their_agreed` (có `dong_y` của người khác cho phiên bản đó), `viewed_by_recipient_at` (chỉ khi người đọc là người gửi phiên bản đó: `seen_at` của lần xem đầu tiên của người khác; ngược lại `null`).
11. `keeps` — theo `created_at, id`; mỗi phần tử `{id, line, created_at}`.

Datetime (`expires_at`, `sent_at`, `viewed_by_recipient_at`, `created_at`): ISO-8601 UTC đuôi `Z`.

Ví dụ trong kịch bản: sau khi gửi, người gửi có `my_response: "dong_y"` ở v1 (gửi là đồng ý) và người nhận có `their_agreed: true` (`owner_reads_sent`, `mate_reads_sent`); sau khi mate xem, `owner_reads_seen` có `viewed_by_recipient_at`, `mate_reads_seen` không. Sau đề nghị sửa, mate có `my_response: "de_nghi_sua"` ở v1 và `dong_y` ở v2 (`mate_reads_two_versions`).

## Tác dụng phụ

Không ghi, không khoá. Một tờ đã khép (`bo` sau khi đóng sổ) không còn là `nhap` nên **người kia đọc được** (`mate_reads_dropped_draft`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_wrong_state` | `Tờ giấy này không đọc được.` (JSON phiên bản hỏng) | `service.py:8188-8190` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:91-99`
- Service: `services/api/app/api/service.py:7663-7683`, `:8165-8190`, `:8227-8311`
- Repository: `services/api/app/api/repository.py:8084-8086`, `:7969-8041`
- Schema: `services/api/app/api/schemas.py:2580-2630`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_a_draft_is_not_a_sent_sheet` (121), `test_sending_is_agreeing` (144), `test_the_view_mark_is_the_senders_to_see` (168), `test_a_counter_proposal_is_a_new_version_its_author_has_agreed_to` (248), `test_a_command_answer_never_carries_content` (516), `test_every_pair_route_refuses_a_stranger` (660)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/GET-papers-paper_id.yaml` (39 bước, `dev`):

- Thứ tự: `anonymous_reads_unknown`, `stranger_reads_unknown`, `stranger_reads_real_paper`, `stranger_reads_real_paper_as_advancer`, `mate_reads_owner_draft`, `mate_reads_owner_draft_as_advancer`, `owner_reads_uppercase_id`, `owner_reads_trailing_slash`.
- Vòng đời: `owner_reads_draft`, `owner_reads_edited` (hai chặng, `place_id`, `can_kiem: false`, `ly_do`), `owner_reads_sent`, `mate_reads_sent`, `owner_reads_seen`, `mate_reads_seen`, `owner_reads_two_versions`, `mate_reads_two_versions`, `mate_reads_plan`, `owner_reads_kept`.
- Sau khi đóng: `mate_reads_kept_after_close`, `owner_reads_dropped_draft`, `mate_reads_dropped_draft`.

`pair_papers/prod-auth.yaml`: `dev_headers_without_bearer`, `mate_reads_owner_draft`, `stranger_reads_claiming_owner`, `stranger_reads_claiming_owner_header`, `junk_bearer_reads`, `mate_reads_kept`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/get-papers-paper_id.yaml`.

## Chưa phủ / lưu ý cho bản Go

- 409 cho JSON phiên bản không đọc được và `het_han` không phủ (không có cửa HTTP ghi hàng hỏng, không có bước đồng hồ).
- `co_the_ghi_da_di` dùng ngày giờ Việt Nam của `now`; ngày tương lai trong kịch bản là `2031-12-31`, ngày quá khứ là `2026-09-01`.
- `content` được dựng lại qua `PaperContent`: `place_id` rỗng hoặc thiếu → `null`, `can_kiem` thiếu → `true`.
- `my_response` lấy phần tử cuối theo thứ tự của repository; `viewed_by_recipient_at` lấy lần xem đầu tiên (`version, seen_at`) của người khác.

## Lỗi Python (chỉ báo, không sửa)

- Oracle tồn tại: id tờ không có → `paper_not_found`, id tờ có thật nhưng không thuộc pair của người gọi → `notebook_not_found`. Người lạ dò id biết id nào là một tờ giấy thật (ngược với lý do route sổ trả 404 thay cho 403).
- Role kiểm trước `may_view_paper`: ở dev, người kia gửi `X-Actor-Roles` thiếu `member` nhận 403 thay vì 404 và biết nháp tồn tại. Ở prod mọi phiên có `member` nên không lộ.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `GET /papers/{paper_id}`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.
