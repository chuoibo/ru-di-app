# POST /contexts/{context_id}/notebook/proposals/{proposal_id}/grant

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Tiếng «ừ» thứ hai cho một lời đề nghị. Khi cả hai người của chu kỳ đang giữ đồng ý sống cho purpose đó, lời đề nghị được đánh dấu hoàn tất và điều nó mở ra xảy ra trong cùng giao dịch: `lap_so` → chu kỳ `active`; `bat_doi` → hai hàng `active_couple_members` (K2: một người một đôi); `doc_chat` → không ghi gì thêm.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`) như mọi POST. Thân rỗng và thân `{}` có fingerprint khác nhau (`crossreplay/…grant.yaml` `python_same_key_with_body` → 422 `idempotency_key_reuse`).
2. `get_actor` → 401/422 (`anonymous_grants`). Validation path (`context_id`, `proposal_id` UUID lax) → 422. Route không có thân; thân gửi kèm bị bỏ qua.
3. `_pair_context_or_404` (`services/api/app/api/service.py:7215-7237`) → 404 `notebook_not_found` (`stranger_grants`, `mate_grants_in_group`).
4. `_locked_notebook` (`service.py:7326-7341`): khoá, tạo hàng sổ nếu chưa có. **Không kiểm role trước bước này.**
5. Lời đề nghị không có, sổ chưa có chu kỳ sống, hoặc lời đề nghị thuộc chu kỳ khác (pair khác, hay chu kỳ đã đóng) → 404 `consent_proposal_not_found` `Không có lời đề nghị này.` (`service.py:7406-7413`; `mate_grants_unknown_on_fresh_pair`, `mate_grants_unknown_proposal`, `mate_grants_other_pairs_offer`, `mate_grants_offer_of_closed_cycle`).
6. `_require_pair_permission("grant_pair_consent", …)` (`service.py:7415-7431`; `services/api/app/domain/permissions.py:533-536`), vị từ theo thứ tự khai báo:
   - role `member` → 403 `permission_denied` `role_not_permitted` (`mate_grants_as_advancer`);
   - `is_invitee` = actor thuộc participants của chu kỳ **và** không phải người hỏi → 403 `permission_denied` `is_invitee` (không có ánh xạ sang câu tiếng Việt; `owner_grants_own_offer`, `owner_grants_own_revoked_offer`);
   - `proposal_in_force` = `dang_cho` (chưa hoàn tất, chưa hết hạn) → 409 `consent_proposal_expired` `Lời đề nghị này đã hết hạn.` (`service.py:8051-8081`; `mate_grants_lap_so_again`, `mate_grants_bat_doi_b_again`).
7. `bat_doi` hoàn tất mà một trong hai người đang là một đôi ở chu kỳ khác → 409 `couple_slot_taken` `Một trong hai người đang là một đôi ở sổ khác.` (`service.py:7442-7455`; `repository.py:7903-7914`; `owner_grants_second_couple`). Toàn bộ giao dịch rollback, kể cả đồng ý vừa ghi.

## Đầu vào

- Path `context_id`, `proposal_id`. Không thân.

## Đầu ra

**200** `PairProposalResponse` (`services/api/app/api/schemas.py:2734-2739`): `id`, `purpose`, `expires_at`, `proposed_by_id` (người hỏi, không phải người đồng ý), `my_granted` (luôn `true`). Cùng hình dạng với 201 của lời đề nghị.

## Tác dụng phụ

Cùng `now`, dưới khoá hàng sổ (`service.py:7432-7455`):

- `pair_consents` của người đồng ý (`repository.py:7832-7861`): chưa có → chèn `granted_at` = `now`; đã có mà `granted_at` NULL → điền và xoá `revoked_at`; **đã có và đã đồng ý (kể cả đã thu hồi) → không đổi gì**.
- Đọc lại sổ và tính `granted_purposes` trên participants (`services/api/app/domain/pair_notebook.py:118-135`). Khi purpose đủ hai người:
  - `pair_consent_proposals.completed_at` = `now` (`repository.py:7863-7870`);
  - `lap_so`: chu kỳ `state='active'`, `opened_at` = `opened_at` cũ hoặc `now` (`repository.py:7790-7796`; `mate_grants_older_offer` giữ `opened_at` lần đầu);
  - `bat_doi`: `active_couple_members (person_id, cycle_id, since)` cho từng participant; hàng đã có cùng chu kỳ thì bỏ qua. Trigger hoãn `active_couple_needs_two_consents` kiểm lúc commit (`services/api/app/db/migrations/versions/c4f27a90d1e3_them_so_hai_nguoi_va_to_giay.py:645-680`).
- Người hỏi đã thu hồi đồng ý của mình → purpose không đủ hai người: đồng ý mới được ghi, lời đề nghị **không** hoàn tất, trả 200 như thường (`mate_grants_revoked_offer`, `owner_reads_half_couple`).
- `idempotency_keys` khi có header và 200. Phát lại đi trước service: một khoá đã lưu vẫn trả 200 cho lời đề nghị đã hoàn tất, trong khi cùng lệnh với khoá khác là 409 (`mate_replays_header_key`, `mate_same_offer_other_key`).
- Mọi từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 404 | `consent_proposal_not_found` | `Không có lời đề nghị này.` | `service.py:7411-7413` |
| 403 | `permission_denied` | `role_not_permitted` / `is_invitee` | `service.py:504`, `permissions.py:533-536` |
| 409 | `consent_proposal_expired` | `Lời đề nghị này đã hết hạn.` | `service.py:8056-8060` |
| 409 | `couple_slot_taken` | `Một trong hai người đang là một đôi ở sổ khác.` | `service.py:7450-7454` |
| 422/409 | idempotency | như `POST …/notebook/proposals` | `idempotency.py` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:85-98`
- Service: `services/api/app/api/service.py:7398-7461`, `:8051-8104`
- Repository: `services/api/app/api/repository.py:7828-7830`, `:7832-7870`, `:7790-7796`, `:7903-7914`
- Domain: `services/api/app/domain/pair_notebook.py:77-135`, `:220-233`; `services/api/app/domain/permissions.py:533-536`, `:654-679`
- Trigger: `c4f27a90d1e3_them_so_hai_nguoi_va_to_giay.py:645-680`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_the_second_yes_opens_the_notebook` (98), `test_the_asker_cannot_answer_their_own_offer` (106), `test_an_offer_nobody_answered_in_time_stops_being_an_offer` (117), `test_a_couple_needs_both_and_one_person_belongs_to_one` (143)
- `services/api/tests/postgres/test_pair_notebook_postgres.py`: `test_mot_nguoi_mot_dong_y_tren_mot_loi_de_nghi` (164), `test_mot_doi_can_dong_y_cua_ca_hai_nguoi` (199), `test_mot_nguoi_khong_the_o_hai_so_doi` (255)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/POST-contexts-context_id-notebook-proposals-proposal_id-grant.yaml` (50 bước, `dev`; ba người: owner–mate và owner–third là hai pair):

- Thứ tự: `anonymous_grants`, `mate_grants_unknown_on_fresh_pair`, `owner_grants_own_offer`, `stranger_grants`, `mate_grants_in_group`, `mate_grants_unknown_proposal`, `mate_grants_as_advancer`.
- `lap_so`: `mate_grants_lap_so`, `mate_grants_lap_so_again`, `mate_grants_older_offer`.
- Đồng ý đã thu hồi: `owner_revokes_bat_doi`, `mate_grants_revoked_offer`, `owner_reads_half_couple`, `owner_grants_own_revoked_offer`.
- Một đôi: `mate_grants_bat_doi_b`, `mate_grants_bat_doi_b_again`, `owner_grants_second_couple` (409), `mate_grants_other_pairs_offer` (404), `owner_leaves_couple_with_mate`, `owner_grants_second_couple_after_release` (200).
- `doc_chat`: `owner_grants_doc_chat`. Chu kỳ cũ: `mate_grants_offer_of_closed_cycle`, `mate_grants_lap_so_new`.
- Header key: `mate_grants_with_header_key`, `mate_replays_header_key`, `mate_same_offer_other_key`, `owner_reads_end`.

`crossreplay/…grant.yaml` (14 bước): 200 lưu ở cửa trước được Python phát lại; cùng khoá kèm thân `{}` là 422; 403 `is_invitee` nhả khoá cho người kia dùng.

`concurrency/…grant.yaml` (9 bước): ba lần đồng ý cùng lúc → một 200 hoàn tất, hai 409 `consent_proposal_expired`; ba lần cùng header key → một 200, hai lần phát lại.

`pair_notebooks/prod-auth.yaml`: `stranger_grants`, `owner_grants_own_offer`, `mate_grants`, `junk_bearer_grants`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-contexts-context_id-notebook-proposals-proposal_id-grant.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Hết hạn 7 ngày (409 `consent_proposal_expired` do đồng hồ) không phủ; nhánh «đã hoàn tất» cho cùng mã và câu thì phủ.
- 404 lời đề nghị đi trước 403 role: người có membership nhưng thiếu role vẫn biết một id có thuộc chu kỳ sống hay không.
- `detail` của 403 `is_invitee` là tên vị từ, không phải câu cho người đọc.
- `participants` của chu kỳ là nguồn `is_invitee`, không phải membership hiện tại.
- Thứ tự ghi hai hàng cặp đôi theo thứ tự participants; lỗi `couple_slot_taken` rollback tất cả, nên kết quả không phụ thuộc thứ tự đó.
- Trigger T3 (hoãn) phải còn đúng: nếu Go ghi hàng cặp đôi trước đồng ý, commit vẫn phải thành công.

## Lỗi Python (chỉ báo, không sửa)

- Lời đề nghị đã hoàn tất trả `consent_proposal_expired` «Lời đề nghị này đã hết hạn.»: bấm «đồng ý» lần hai bị báo hết hạn dù vừa thành công.
- `grant_consent` không khôi phục đồng ý đã thu hồi (`granted_at` còn thì bỏ qua): người hỏi đã thu hồi không có cách «đồng ý lại» lời đề nghị của mình, cũng không thể đồng ý nó (403 `is_invitee`); lời đề nghị treo tới khi hết hạn.
- `_locked_notebook` chạy trước kiểm role, nên một request 403 vẫn xếp hàng khoá sổ (rollback, không để lại hàng).

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

«Cả hai đồng ý» (`pair_notebook.granted_purposes`, quyết định hoàn tất lời đề nghị, mở chu kỳ, ghi `active_couple_members`) tính theo CÙNG MỘT lời đề nghị, không theo purpose; một lời đề nghị đã hoàn tất không còn hết hạn theo cửa sổ 7 ngày (`_live`/`live`). Với dữ liệu một-đề-nghị-mỗi-bậc (mọi kịch bản hiện có) byte trả lời không đổi.

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /contexts/{context_id}/notebook/proposals/{proposal_id}/grant`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /contexts/{context_id}/notebook/proposals/{proposal_id}/grant`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `POST /contexts/{context_id}/notebook/proposals/{proposal_id}/grant`: Một lời đề nghị `chia_gu` đã hoàn tất lúc tạo nên `proposal_in_force` sai: người kia không «đồng ý hộ» được gu của ai. Không đổi mã.

## Đổi 2026-09-25 — Nếp dùng gu của người đã bật `chia_gu` khi phác tờ (ADR-0034 §2.2)

Diff này tách `_pair_context_or_404` thành `_pair_roster_or_404` (cùng ba lệnh đọc, cùng thứ tự, cùng câu trả lời; chỉ giữ thêm tên hiển thị của hàng thành viên — Go `pairRosterOr404`, `Member.DisplayName`) và thêm vào `draft_pair_paper` bước gu: `_gu_cho_nep` + hàm thuần `pair_paper.gu_cho_nep` / `loai_theo_gu` / `lam_giau_theo_gu` (Go `pairpaper.GuChoNep` / `LoaiTheoGu` / `LamGiauTheoGu`). Golden: `python_pair_paper*.json` (ca `gu_cho_nep`, `lam_giau_theo_gu`, fuzz riêng), `python_pair_steps.json` (7 ca `taste_*` của draft); Go replay 0 lệch.

- `POST /contexts/{context_id}/notebook/proposals/{proposal_id}/grant`: Chỉ đi qua `_pair_roster_or_404` (cùng lệnh đọc, cùng câu trả lời) hoặc bị cổng nối theo tên hàm kéo vào — hành vi không đổi.
