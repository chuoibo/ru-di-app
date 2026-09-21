# POST /contexts/{context_id}/papers/draft

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Xin sổ một tờ giấy mới, điền sẵn một khung để sửa (ADR-0027 §6: một lệnh, không phải GET có tác dụng phụ). Tờ thuộc tuần hiện tại, là nháp của người xin, và là «tờ tạm» khi pair chưa có chu kỳ sống (lời rủ đầu tiên, §14.1).

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 201 lưu và phát lại cùng `id` (`owner_replays_header_key`), kể cả sau khi tờ đó đã khép (`crossreplay/…draft.yaml` `front_replays_after_skip`); thân `{}` cùng khoá → 422 (`owner_header_key_with_body`); cùng khoá dưới actor khác chạy thật (`mate_uses_owner_header_key`).
2. `get_actor` → 401 (`anonymous_drafts`), 422. Không thân được đọc (`owner_drafts_with_ignored_body` là 201 dù thân có `state`, `cycle_id`, `content`).
3. `_pair_context_or_404` → 404 `notebook_not_found` (`stranger_drafts_unknown`, `stranger_drafts_pair`, `owner_drafts_group`).
4. `_locked_notebook` (`services/api/app/api/service.py:7619`): khoá, tạo hàng sổ nếu chưa có.
5. `_require_pair_permission("draft_pair_paper", …)` (`service.py:7620-7630`; `services/api/app/domain/permissions.py:547-550`), theo thứ tự:
   - role → 403 `role_not_permitted` (`owner_drafts_as_advancer`);
   - `cycle_active_or_temporary` = chu kỳ sống `active` **hoặc không có chu kỳ sống** → 409 `cycle_not_active` `Sổ chưa mở. Cả hai cùng đồng ý lập sổ trước đã.` khi chu kỳ `pending` (`owner_drafts_in_pending_notebook`).
6. Một tờ mở mỗi pair: bất kỳ tờ nào của context có `hieu_luc` thuộc `OPEN_STATES`, **của ai cũng vậy** → 409 `paper_wrong_state` `Đang có một tờ mở. Xong tờ này đã.` (`service.py:7631-7640`; `owner_drafts_again`, `mate_drafts_while_owner_draft_open`, `owner_drafts_while_temporary_open`).

## Đầu vào

- Path `context_id`. Không thân.

## Đầu ra

**201** `PaperCommandResponse` (`services/api/app/api/schemas.py:2663-2675`), thứ tự khoá `id`, `state` (`nhap`), `version` (1), `outing_id` (`null`). Không có nội dung; đọc qua `GET /papers/{id}`.

## Tác dụng phụ

Cùng `now`, dưới khoá hàng sổ (`service.py:7641-7661`; `repository.py:8043-8082`):

- `pair_notebooks` nếu chưa có.
- `pair_papers`: `context_id`, `context_kind='pair'`, `cycle_id` = chu kỳ sống hoặc NULL, `is_temporary = (cycle_id IS NULL)`, `draft_owner_id` = actor, `state='nhap'`, `current_version=1`, `tuan` = thứ Hai tuần này theo `Asia/Ho_Chi_Minh` (`services/api/app/domain/pair_paper.py:107-115`), `expires_at` = nửa đêm cuối Chủ nhật đó viết UTC (`pair_paper.py:118-133`), `created_at`, `done_recorded_*` NULL.
- `pair_paper_versions` v1: `content` = `{"ngay": <thứ Bảy tuần này, hoặc hôm nay nếu thứ Bảy đã qua>, "chang": [{"gio": "18:30", "viec": "Ăn tối", "place_id": null, "can_kiem": true}]}` (`service.py:8039`, `pair_paper.py:136-147`, `:305-352`); `ly_do` NULL; `nguon` = `{"scope": "chung", "dung": ["routine"] (+ "rang_buoc" nếu chu kỳ sống có ít nhất một dòng ràng buộc, của bất kỳ ai), "luc": now.isoformat()}` (`luc` dạng `+00:00`); `author_type='human'`; `sent_at`/`sent_by` NULL.
- Không kiểm «một tờ mỗi tuần»: nghỉ tuần rồi xin tiếp trong cùng tuần là 201.
- Sau khi đóng sổ, chu kỳ đã đóng không phải chu kỳ sống, nên tờ mới lại là tờ tạm (`owner_drafts_after_close`, `owner_reads_after_close`).
- `idempotency_keys` khi có header và 201. Từ chối rollback, kể cả hàng sổ.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `cycle_not_active` | `Sổ chưa mở. Cả hai cùng đồng ý lập sổ trước đã.` | `service.py:8076-8080` |
| 409 | `paper_wrong_state` | `Đang có một tờ mở. Xong tờ này đã.` | `service.py:7635-7639` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:71-88`
- Service: `services/api/app/api/service.py:7611-7661`, `:8039`, `:8051-8104`
- Repository: `services/api/app/api/repository.py:8043-8082`, `:8092-8098`
- Domain: `services/api/app/domain/pair_paper.py:107-147`, `:305-352`
- Bảng: `services/api/app/db/models.py:3136-3296` (`uq_pair_papers_open_per_context`, CHECK `paper_temporary_has_no_cycle`)

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_a_fresh_sheet_arrives_pre_filled_and_unpromised` (95), `test_one_open_sheet_at_a_time` (413), `test_the_very_first_invitation_happens_before_any_notebook` (421), `test_a_notebook_half_open_takes_no_sheet` (435)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_mot_cuoc_tro_chuyen_chi_mot_to_dang_mo` (187), `test_to_tam_thoi_khong_co_chu_ky_va_nguoc_lai` (448)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-contexts-context_id-papers-draft.yaml` (36 bước, `dev`):

- Thứ tự: `anonymous_drafts`, `stranger_drafts_unknown`, `stranger_drafts_pair`, `owner_drafts_group`, `owner_drafts_as_advancer`.
- Tờ tạm: `owner_drafts_temporary`, `owner_reads_temporary`, `owner_drafts_again`, `mate_drafts_while_owner_draft_open`.
- Chu kỳ: `owner_drafts_in_pending_notebook`, `owner_drafts_while_temporary_open`, `owner_skips_temporary`, `owner_puts_constraint`, `owner_drafts_with_ignored_body`, `owner_reads_prefilled`, `mate_drafts`, `owner_reads_mates_draft` (404).
- Header key: `owner_drafts_with_header_key`, `owner_replays_header_key`, `mate_uses_owner_header_key`, `owner_header_key_with_body`.
- Sau khi đóng: `owner_previews_close`, `owner_closes`, `owner_drafts_after_close`, `owner_reads_after_close`, `owner_lists_end`.

`crossreplay/POST-…-papers-draft.yaml` (13 bước). `concurrency/POST-…-papers-draft.yaml` (11 bước): ba lần xin cùng lúc trên sổ đã có hàng → một 201, hai 409 (tờ được đọc sau khoá); ba lần cùng header key → một tờ, một 201, hai lần phát lại.

`pair_papers/prod-auth.yaml`: `junk_bearer_drafts`, `owner_drafts_with_header_key`, `owner_replays_header_key`, `mate_same_key_own_session`, `mate_drafts`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-contexts-context_id-papers-draft.yaml`.

## Chưa phủ / lưu ý cho bản Go

- `ngay` mặc định, `tuan` và `expires_at` phụ thuộc ngày chạy theo giờ Việt Nam; hai stack cùng chạy một ngày nên so được, nhưng Go phải tính theo `Asia/Ho_Chi_Minh`, không theo đồng hồ máy.
- `nguon.luc` là `datetime.isoformat()` của Python (`+00:00`, sáu chữ số thập phân khi micro giây khác 0) nằm trong JSONB.
- Kiểm quyền và kiểm tờ mở đọc sau khoá hàng sổ; hàng sổ phải có trước (xem lỗi dưới).
- `het_han` của tờ cũ giải phóng chỗ cho tờ mới, nhưng tờ đó vẫn giữ `state` mở trong cột, còn `uq_pair_papers_open_per_context` đếm theo cột: xin tờ mới khi tờ cũ đã quá hạn nhưng chưa ai khép → vi phạm unique. Không phủ (không có bước đồng hồ).

## Lỗi Python (chỉ báo, không sửa)

- 409 `Đang có một tờ mở.` cho người **không được thấy** nháp của người kia: lộ đúng điều `GET …/papers` và `open_paper_id` giấu.
- Sau khi đóng sổ, lệnh xin tờ vẫn thành công như lời rủ đầu tiên (`cycle_active_or_temporary` coi «chu kỳ đã đóng» là «chưa có chu kỳ»), trái với chú thích «stops a sheet appearing in a notebook that … has been closed».
- Tờ mở đã quá hạn (`het_han` khi đọc, `state` mở trong cột) chặn không được bằng kiểm service nhưng vẫn giữ unique index: lệnh xin tờ tuần sau sẽ nhận `IntegrityError` → 500 cho tới khi ai đó khép tờ cũ. Suy từ mã, không đo được.
- Hai request ghi đầu tiên cùng lúc trên pair chưa có hàng sổ → 500 (xem thẻ `POST …/notebook/proposals`).
