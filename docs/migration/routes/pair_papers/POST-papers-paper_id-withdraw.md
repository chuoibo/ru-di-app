# POST /papers/{paper_id}/withdraw

pair_papers · core · trạng thái trong bộ nhớ: không có

## Mục đích

Người gửi rút lại tờ khi người kia chưa mở và chưa trả lời (ADR-0027 §3). Sau một lần mở thì đã có điều để nói, và lối ra là một phiên bản mới chứ không phải biến mất.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 200 lưu và phát lại (`owner_replays_header_key`); phiên bản khác cùng khoá → 422 (`owner_header_key_other_version`); 409 nhả khoá (`crossreplay/…withdraw.yaml` `python_not_withdrawable`, `front_same_key_again`).
2. JSON hỏng → 422. `get_actor` → 401 (`anonymous_withdraws`), 422.
3. Validation `PaperWithdrawRequest` (`services/api/app/api/schemas.py:2687-2688`): `version` strict int ≥ 1 (`owner_version_string`, `owner_version_missing`, `owner_version_zero`).
4. `_locked_paper` (`services/api/app/api/service.py:7685-7699`): 404 `paper_not_found` (`stranger_withdraws_unknown`), 404 `notebook_not_found` (`stranger_withdraws_real_paper`), 403 role (`owner_withdraws_as_advancer`), 404 cho nháp của người khác (`mate_withdraws_owner_draft`).
5. `co_the_rut` (`services/api/app/domain/pair_paper.py:181-222`) tính từ hàng đã khoá: cột `state == 'da_gui'`; phiên bản hiện hành `author_type='human'` và `sent_by == actor`; không có view và không có phản hồi của người khác cho phiên bản hiện hành.
6. `_require_pair_permission("withdraw_pair_paper", {"is_group_member": True, "paper_unseen_unanswered": co_the})` (`service.py:7961-7965`; `services/api/app/domain/permissions.py:582-585`): role; rồi `co_the` sai → 409 `paper_not_withdrawable` `Người kia đã mở tờ này rồi, không rút lại được.` **cho mọi lý do**: nháp (`owner_withdraws_draft`), người nhận (`mate_withdraws_sent`, `mate_withdraws_sent_stale`), đã xem (`owner_withdraws_seen`, `mate_withdraws_seen_counter`), không phải người gửi phiên bản hiện hành (`owner_withdraws_countered`), đã rút (`owner_withdraws_again`), đã chốt (`mate_withdraws_plan`).
7. `version != current_version` → 409 `paper_version_stale` (`service.py:7966-7970`; `owner_withdraws_stale`, `mate_withdraws_counter_stale`) — chỉ sau khi rút được.
8. `chuyen(paper, "rut", co_the_rut=True)`: tờ quá hạn → 409 `paper_expired`.

## Đầu vào

- Path `paper_id`. Thân `{"version": <int ≥ 1>}`.

## Đầu ra

**200** `PaperCommandResponse` `{id, state:"rut", version, outing_id:null}`.

## Tác dụng phụ

- `pair_papers.state='rut'` (`repository.py:8155-8173`). Phiên bản, phản hồi, view giữ nguyên. Người kia vẫn đọc được tờ `rut` (`mate_reads_withdrawn`).
- Người viết đề nghị sửa rút được phiên bản của mình khi người kia chưa xem (`mate_withdraws_counter`, `owner_reads_withdrawn_counter`).
- `idempotency_keys` khi có header và 200. Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `paper_not_found` | `Không có tờ giấy này.` | `service.py:7668`, `:8052` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `paper_not_withdrawable` | `Người kia đã mở tờ này rồi, không rút lại được.` | `service.py:8061-8065` |
| 409 | `paper_version_stale` | `Tờ giấy đã sang phiên bản mới.` | `service.py:7967-7969` |
| 409 | `paper_expired` | `Tuần này hết rồi. Tuần sau mình rủ lại nhé.` | `service.py:8085` |

## Mã Python

- Route: `services/api/app/api/routes/pair_papers.py:169-182`
- Service: `services/api/app/api/service.py:7948-7972`, `:8117-8139`
- Domain: `services/api/app/domain/pair_paper.py:181-222`, `:225-302`

## Test đang phủ

- `services/api/tests/api/test_pair_papers.py`: `test_withdrawal_works_until_they_have_looked` (341), `test_withdrawal_stops_the_moment_they_have_looked` (351), `test_only_the_sender_withdraws` (364)

## Kịch bản parity

`parity/scenarios/w8/pair_papers/POST-papers-paper_id-withdraw.yaml` (46 bước, `dev`; mọi tờ là tờ tạm): `anonymous_withdraws`, `stranger_withdraws_unknown`, `owner_withdraws_draft`, `mate_withdraws_owner_draft`, `stranger_withdraws_real_paper`, `owner_withdraws_as_advancer`, `mate_withdraws_sent`, `mate_withdraws_sent_stale`, `owner_withdraws_stale`, `owner_version_string`, `owner_version_missing`, `owner_version_zero`, `owner_withdraws`, `owner_withdraws_again`, `mate_reads_withdrawn`, `owner_withdraws_seen`, `owner_withdraws_countered`, `mate_withdraws_counter_stale`, `mate_withdraws_counter`, `owner_reads_withdrawn_counter`, `mate_withdraws_seen_counter`, `mate_withdraws_plan`, `owner_withdraws_with_header_key`, `owner_replays_header_key`, `owner_header_key_other_version`.

`crossreplay/POST-papers-paper_id-withdraw.yaml` (15 bước). `concurrency/POST-papers-paper_id-withdraw.yaml` (9 bước): chỉ ba lần cùng header key.

`pair_papers/prod-auth.yaml`: `junk_bearer_withdraws`, `stranger_withdraws`, `owner_withdraws`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-papers-paper_id-withdraw.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Nhánh «người kia đã trả lời mà chưa xem» của `co_the_rut` không tách được qua HTTP: `dong_y` của người nhận chốt tờ, `de_nghi_sua` đổi phiên bản hiện hành.
- `paper_expired` không phủ. Tờ Nếp gửi (`author_type='nep'`) không có cửa tạo.
- `paper_not_withdrawable` đi **trước** `paper_version_stale`.

## Lỗi Python (chỉ báo, không sửa)

- Một câu cho mọi lý do: người nhận, chủ nháp chưa gửi, tờ đã chốt hay đã rút đều nghe «Người kia đã mở tờ này rồi».
- Đọc cũ dưới khoá (xem thẻ `POST /papers/{id}/send`): hai lần rút cùng lúc không khoá idempotency có thể cùng 200 thay vì 200 + 409.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /papers/{paper_id}/withdraw`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /papers/{paper_id}/withdraw`: Route này đọc/ghi tờ qua `_readable_paper_or_404`: với tờ chưa từng gửi ở trạng thái đóng, người không phải chủ nay nhận 404 `paper_not_found` (trước: đọc được, hoặc lỗi trạng thái). Tờ đã gửi: byte không đổi.
