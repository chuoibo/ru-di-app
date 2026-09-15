# POST /papers/{paper_id}/done

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

«Đã đi rồi» (spec §3.1): một trong hai người ghi rằng buổi đi đã diễn ra, kèm tên người ghi. Không bao giờ suy từ ngày; chỉ được ghi từ ngày của buổi đi trở đi, theo đồng hồ máy chủ và giờ Việt Nam (§3.3 luật 6).

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 200 lưu và phát lại (`owner_replays_header_key`); thân `{}` cùng khoá → 422 (`crossreplay/…done.yaml` `python_same_key_with_body`).
2. `get_actor` → 401 (`anonymous_records`), 422. Không thân.
3. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_records_unknown`), 404 `notebook_not_found` (`stranger_records_real_paper`), 403 role (`owner_records_as_advancer`), 404 cho nháp của người khác (`mate_records_owner_draft`).
4. `_require_pair_permission("record_pair_outing_done", {"is_group_member": True})` (`service.py:7994-7996`; `services/api/app/domain/permissions.py:590`).
5. `_co_the_ghi_da_di` (`service.py:8227-8237`): `hieu_luc == "chot"` **và** ngày hôm nay ở `Asia/Ho_Chi_Minh` ≥ `content.ngay` của phiên bản hiện hành. Sai vì bất kỳ lý do nào → 409 `paper_wrong_state` `Chưa tới ngày đi.` (`service.py:7998-7999`): nháp (`owner_records_draft`), tờ đã gửi (`owner_records_sent`), kế hoạch ngày tương lai (`owner_records_future_plan`), đã ghi rồi (`owner_records_again`, `mate_records_after_owner`).
6. `chuyen(paper, "da_di", nguoi_ghi=True)` → `da_di`.

## Đầu vào

- Path `paper_id`. Không thân.

## Đầu ra

**200** `PaperCommandResponse` `{id, state:"da_di", version, outing_id:<uuid>}` — `outing_id` lấy từ link của tờ.

## Tác dụng phụ

- `pair_papers`: `state='da_di'`, `done_recorded_by_id` = actor, `done_recorded_at` = `now` (`repository.py:8155-8173`; CHECK `paper_done_has_recorder`). `GET /papers/{id}` sau đó có `co_the_ghi_da_di: false` (`owner_reads_recorded`).
- Không kiểm chu kỳ: sau khi đóng sổ, tờ `chot` vẫn ghi được `da_di` và nhận dòng giữ lại (`mate_records_after_close`, `owner_keeps_after_close`).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_wrong_state` | `Chưa tới ngày đi.` | `service.py:7999` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:200-213`
- Service: `services/api/app/api/service.py:7984-8004`, `:8193-8204`, `:8227-8237`
- Domain: `services/api/app/domain/pair_paper.py:59`, `:225-302`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_the_day_has_to_have_come_before_anybody_says_they_went` (446), `test_a_plan_that_stands_outlives_the_week` (387), `test_a_closed_notebook_refuses_the_whole_sheet` (610)
- `services/api/tests/postgres/test_pair_papers_postgres.py`: `test_da_di_phai_co_nguoi_ghi_nhan` (429)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-done.yaml` (42 bước, `dev`): `anonymous_records`, `stranger_records_unknown`, `owner_records_draft`, `mate_records_owner_draft`, `owner_records_sent`, `owner_records_future_plan` (ngày `2031-12-31`), `stranger_records_real_paper`, `owner_records_as_advancer`, `mate_reads_future_plan`, `mate_records_past_plan` (ngày `2026-09-01`), `owner_records_again`, `owner_reads_recorded`, `owner_records_with_header_key`, `owner_replays_header_key`, `mate_records_after_owner`, `owner_previews_close`, `owner_closes`, `mate_records_after_close`, `owner_keeps_after_close`.

`crossreplay/POST-papers-paper_id-done.yaml` (14 bước). `concurrency/POST-papers-paper_id-done.yaml` (11 bước): chỉ ba lần cùng header key.

`pair_papers/prod-auth.yaml`: `junk_bearer_records_done`, `mate_records_done`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-papers-paper_id-done.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Biên «đúng ngày hôm nay» không phủ bằng ngày cố định (phải là ngày chạy); kịch bản chỉ dùng một ngày đã qua và một ngày xa.
- So sánh ngày theo giờ Việt Nam của `now`; `content.ngay` không đọc được → 409 cùng câu.
- Đọc `hieu_luc` (tờ `chot` không hết hạn).

## Lỗi Python (chỉ báo, không sửa)

- «Chưa tới ngày đi.» cho mọi lý do: nháp, tờ chưa chốt, tờ đã ghi `da_di` hay đã `da_giu` đều nghe là chưa tới ngày.
- Sổ đã đóng vẫn nhận `da_di` (trái với «đóng sổ là đóng»).
- Đọc cũ dưới khoá (xem thẻ `POST /papers/{id}/send`): hai lần ghi cùng lúc không khoá idempotency có thể cùng 200 và ghi đè `done_recorded_by_id`.
