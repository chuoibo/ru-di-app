# DELETE /contexts/{context_id}/notebook/constraints/{kind}

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Xoá dòng ràng buộc của chính mình (`khong_an_duoc` hoặc `dung`) trong chu kỳ sống. «Làm rỗng» một ràng buộc là việc của route này, không phải PUT với chuỗi rỗng.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 204 được lưu; phát lại đi trước service (`owner_replays_header_key` phát lại 204 dù dòng viết lại ở giữa vẫn còn).
2. `get_actor` → 401 (`anonymous_deletes`), 422.
3. Validation `context_id` → 422.
4. Kind ngoài `CONSTRAINT_KINDS` → 404 `constraint_kind_unknown` `Không có ô này.` **trước context** (`services/api/app/api/service.py:7529-7530`; `stranger_unknown_kind`, `owner_kind_uppercase`).
5. `_pair_context_or_404` → 404 `notebook_not_found` (`stranger_deletes_in_pair`, `owner_deletes_in_group`).
6. `_require_pair_permission("edit_pair_constraint", {"is_self": True})` (`service.py:7532`) → 403 `role_not_permitted` (`owner_deletes_as_advancer`).
7. `_locked_notebook`; không có chu kỳ sống → 204 không làm gì (`owner_deletes_on_fresh_pair`, `owner_deletes_after_close`).

## Đầu vào

- Path `context_id`, `kind`. Không thân.

## Đầu ra

**204**, thân rỗng. Không phân biệt «đã xoá» với «không có gì để xoá» (`owner_deletes_missing`).

## Tác dụng phụ

- Pair chưa có hàng sổ: `pair_notebooks` được chèn và commit (`owner_deletes_on_fresh_pair`).
- Có chu kỳ: `DELETE pair_shared_constraints WHERE (cycle_id, owner_id=actor, kind)` (`repository.py:7959-7967`). Chỉ dòng của người gọi; dòng cùng kind của người kia còn nguyên (`mate_deletes_khong_an_duoc`, `owner_reads_after_mate_delete`).
- Không có lịch sử: PUT sau khi xoá bắt đầu lại `version=1` (`owner_puts_after_delete`, `mate_reads_version_restart`).
- Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `constraint_kind_unknown` | `Không có ô này.` | `service.py:7530` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:136-148`
- Service: `services/api/app/api/service.py:7526-7537`
- Repository: `services/api/app/api/repository.py:7959-7967`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_both_may_read_the_two_lines_and_only_their_owner_writes_one` (185), `test_an_unknown_constraint_kind_is_not_a_door` (231)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/DELETE-contexts-context_id-notebook-constraints-kind.yaml` (31 bước, `dev`): `anonymous_deletes`, `stranger_unknown_kind`, `owner_deletes_on_fresh_pair`, `stranger_deletes_in_pair`, `owner_deletes_in_group`, `owner_deletes_as_advancer`, `owner_kind_uppercase`, `owner_deletes_missing`, `mate_deletes_khong_an_duoc`, `owner_reads_after_mate_delete`, `owner_deletes_khong_an_duoc`, `owner_puts_after_delete`, `mate_reads_version_restart`, `owner_header_key`, `owner_puts_between`, `owner_replays_header_key`, `owner_reads_after_replay`, `owner_deletes_after_close`.

`crossreplay/DELETE-…-constraints-kind.yaml` (13 bước): 204 lưu ở cửa trước được Python phát lại mà không xoá dòng viết lại; kind khác cùng khoá 422.

`concurrency/DELETE-…-constraints-kind.yaml` (9 bước): ba lần xoá cùng lúc → ba 204.

`pair_notebooks/prod-auth.yaml`: `mate_deletes`, `junk_bearer_deletes_constraint`.

Corpus sinh tự động: hoãn, `path kind is str`.

## Chưa phủ / lưu ý cho bản Go

- Kind trước context, sau actor. 204 cho mọi nhánh thành công, kể cả khi không có dòng hoặc không có chu kỳ.
- Phát lại một DELETE đã lưu không chạm DB.

## Lỗi Python (chỉ báo, không sửa)

- Một DELETE trên pair chưa từng mở sổ để lại một hàng `pair_notebooks` đã commit.
- `version` bắt đầu lại từ 1 sau khi xoá: client giữ `version` cũ để phát hiện màn cũ sẽ nhầm dòng mới với dòng đã biết.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `DELETE /contexts/{context_id}/notebook/constraints/{kind}`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `DELETE /contexts/{context_id}/notebook/constraints/{kind}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `DELETE /contexts/{context_id}/notebook/constraints/{kind}`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — Nếp dùng gu của người đã bật `chia_gu` khi phác tờ (ADR-0034 §2.2)

Diff này tách `_pair_context_or_404` thành `_pair_roster_or_404` (cùng ba lệnh đọc, cùng thứ tự, cùng câu trả lời; chỉ giữ thêm tên hiển thị của hàng thành viên — Go `pairRosterOr404`, `Member.DisplayName`) và thêm vào `draft_pair_paper` bước gu: `_gu_cho_nep` + hàm thuần `pair_paper.gu_cho_nep` / `loai_theo_gu` / `lam_giau_theo_gu` (Go `pairpaper.GuChoNep` / `LoaiTheoGu` / `LamGiauTheoGu`). Golden: `python_pair_paper*.json` (ca `gu_cho_nep`, `lam_giau_theo_gu`, fuzz riêng), `python_pair_steps.json` (7 ca `taste_*` của draft); Go replay 0 lệch.

- `DELETE /contexts/{context_id}/notebook/constraints/{kind}`: Chỉ đi qua `_pair_roster_or_404` (cùng lệnh đọc, cùng câu trả lời) hoặc bị cổng nối theo tên hàm kéo vào — hành vi không đổi.
