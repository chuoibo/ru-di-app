# POST /contexts/{context_id}/notebook/close/preview

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Bước một của «Đóng sổ» (spec §7.6, ADR-0027 §8): đếm ba số phận khác nhau của các tờ và số lời đề nghị sẽ huỷ, kèm `revision` mà lệnh đóng phải mang lại. Là POST vì nó sinh ra con số người dùng đồng ý, nhưng **không ghi gì** và **không khoá**.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): POST có header key được lưu và **phát lại revision cũ** dù sổ đã đổi (`owner_replays_stale_preview`); lệnh đóng mang revision đó bị 409 (`owner_closes_with_replayed_revision`).
2. Router: `GET` → 405 `allow: POST` (`owner_gets_preview`).
3. `get_actor` → 401 (`anonymous_previews`), 422. Thân gửi kèm bị bỏ qua (`owner_previews_with_body`).
4. `_pair_context_or_404` (`services/api/app/api/service.py:7215-7237`) → 404 `notebook_not_found` (`stranger_previews_unknown`, `stranger_previews_pair`, `owner_previews_group`).
5. `_require_pair_permission("preview_close_pair_notebook", {"is_group_member": True})` (`service.py:7552-7554`; `services/api/app/domain/permissions.py:600-603`) → 403 `role_not_permitted` (`owner_previews_as_advancer`).

## Đầu vào

- Path `context_id`. Không thân.

## Đầu ra

**200** `ClosePreviewResponse` (`services/api/app/api/schemas.py:2786-2794`), JSON gọn, thứ tự khoá:

- `revision`: 16 ký tự hex thường = `sha256("\n".join(sorted(material)))[:16]` (`services/api/app/domain/pair_notebook.py:169-178`), với `material` gồm:
  - `"<paper_id>:<hieu_luc>"` cho **mọi** tờ của context, mọi trạng thái kể cả đã khép, nháp của người kia, tờ tạm (`pair_notebook.py:197-208`);
  - `"dn:<proposal_id>"` cho mỗi lời đề nghị còn `dang_cho` của chu kỳ sống (`pair_notebook.py:209-210`).
  - Sổ không có tờ và không có lời đề nghị chờ: `e3b0c44298fc1c14` (digest của chuỗi rỗng; `owner_previews_fresh`, `owner_previews_open_empty`).
- `so_nhap_bo`: số tờ `hieu_luc == "nhap"` (của **cả hai** người; `mate_previews_owner_draft` là 1).
- `so_to_huy`: số tờ `da_gui`, `da_xem`, `de_nghi_sua`, `dong_y` (`owner_previews_sent`, `owner_previews_seen`).
- `so_to_khoa`: số tờ `chot`, `da_di` (`owner_previews_plan`, `owner_previews_done`); `da_giu` không đếm (`owner_previews_kept`).
- `so_de_nghi_huy`: số lời đề nghị chờ (`owner_previews_pending_offer`).

Revision không đổi khi sửa nội dung nháp hay ràng buộc (`owner_previews_after_edit`); đổi khi một tờ đổi trạng thái hoặc có lời đề nghị mới. Sau khi đóng, tờ `chot`/`da_di` **vẫn được đếm** là «sẽ khoá» (`owner_previews_closed`).

## Tác dụng phụ

Không ghi, không khoá; hàng `pair_notebooks` không được tạo (`service.py:7539-7545`, `:7547-7555`). Riêng header key: `idempotency_keys` lưu 200 kèm revision.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: POST` | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:151-163`
- Service: `services/api/app/api/service.py:7539-7555`, `:8107-8115`, `:8154-8163`
- Domain: `services/api/app/domain/pair_notebook.py:169-217` (`_revision`, `xem_truoc_dong_so`), `:220-233`; `services/api/app/domain/pair_paper.py:86-90`, `:150-164`
- Repository: `services/api/app/api/repository.py:7732-7738`, `:8092-8098`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_the_preview_counts_three_different_fates` (256), `test_closing_refuses_a_revision_that_has_gone_stale` (280)
- `services/api/tests/domain/test_pair_notebook.py`
- `services/api/tests/postgres/test_pair_papers_races_postgres.py`: `test_xem_truoc_cu_khong_dong_duoc_cuon_so_vua_doi` (367)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/POST-contexts-context_id-notebook-close-preview.yaml` (50 bước, `dev`):

- Thứ tự: `anonymous_previews`, `stranger_previews_unknown`, `stranger_previews_pair`, `owner_previews_group`, `owner_previews_as_advancer`, `owner_previews_with_body`, `owner_gets_preview`.
- Sổ rỗng: `owner_previews_fresh`, `mate_previews_fresh`, `owner_previews_open_empty`.
- Từng số phận: `owner_previews_pending_offer`, `mate_previews_pending_offer`, `mate_previews_owner_draft`, `owner_previews_after_edit`, `owner_previews_sent`, `owner_previews_seen`, `owner_previews_plan`, `owner_previews_done`, `owner_previews_kept`, `owner_previews_skipped`.
- Header key phát lại revision cũ: `owner_previews_with_key`, `owner_replays_stale_preview`, `owner_previews_current`, `owner_closes_with_replayed_revision`, `owner_closes`.
- Sau khi đóng: `owner_previews_closed`, `mate_previews_closed`.

Mọi revision không rỗng được bind `class: token`: nó băm id sinh ngẫu nhiên nên khác nhau giữa hai stack; revision rỗng `e3b0c44298fc1c14` giữ nguyên văn.

`crossreplay/POST-…-close-preview.yaml` (15 bước): revision lưu qua Python được cửa trước phát lại sau khi sổ đổi; cùng khoá kèm thân `{}` 422; revision phát lại bị 409 khi đóng. `concurrency/POST-…-close.yaml`: ba preview cùng lúc.

`pair_notebooks/prod-auth.yaml`: `stranger_previews`, `junk_bearer_previews`, `owner_previews`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-contexts-context_id-notebook-close-preview.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Revision phải là **cùng** digest: sort chuỗi theo byte UTF-8 (Python so sánh code point, cùng thứ tự với byte UTF-8 cho các chuỗi ASCII này), nối `\n`, SHA-256, 16 hex đầu. Id là UUID dạng chữ thường có gạch. Parity chỉ so tên đã bind, nên sai digest ở Go chỉ lộ khi đóng bằng revision do Python phát (crossreplay) hoặc qua test golden.
- `hieu_luc` áp hạn tuần lúc đọc: tờ mở quá hạn vào material dưới dạng `het_han`.
- Hết hạn không phủ (không có bước đồng hồ).

## Lỗi Python (chỉ báo, không sửa)

- Preview đếm nháp của người kia (`so_nhap_bo`) và đổi revision khi người kia tạo nháp: màn «Đóng sổ» cho biết người kia đang có một tờ nháp mà `GET …/papers` và `open_paper_id` giấu (ADR-0027 §3.3 luật 1).
- Sau khi sổ đã đóng, preview vẫn báo tờ `chot`/`da_di` là «sẽ khoá».
- POST có header key được lưu và phát lại revision cũ, dẫn người dùng tới 409 ở bước đóng.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /contexts/{context_id}/notebook/close/preview`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /contexts/{context_id}/notebook/close/preview`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
