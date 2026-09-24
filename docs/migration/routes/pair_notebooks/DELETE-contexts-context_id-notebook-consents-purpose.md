# DELETE /contexts/{context_id}/notebook/consents/{purpose}

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Rút lại đồng ý của chính mình cho một purpose trong chu kỳ sống. Có hiệu lực ở lần đọc sau vì mọi lần đọc đều hỏi lại (không có cache). Rút `bat_doi` chấm dứt «một đôi» cho cả hai; rút `doc_chat` bỏ các nháp Nếp viết từ chat mà chưa gửi.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`) vì DELETE là lệnh ghi. 204 được lưu và phát lại với `content-length: 0`, `idempotency-replayed: true`, không `content-type` (`owner_replays_header_key`); purpose khác cùng khoá → 422 `idempotency_key_reuse` (`owner_header_key_other_purpose`).
2. Router: path đã giải mã phần trăm trước khi khớp. `lap%5Fso` là `lap_so` (`owner_purpose_encoded_underscore`); `lap%2Fso` thành hai đoạn và không khớp route nào → 404 `{"detail":"Not Found"}` (`owner_purpose_encoded_slash`).
3. `get_actor` → 401 (`anonymous_revokes_unknown_purpose`), 422.
4. Validation `context_id` → 422. `purpose` là `str` bất kỳ.
5. **Purpose ngoài thang, trước mọi kiểm context**: không thuộc `CONSENT_PURPOSES` (phân biệt hoa thường, không cắt khoảng trắng) → 404 `consent_purpose_unknown` `Không có mục đích này.` (`services/api/app/api/service.py:7468-7469`; `stranger_unknown_purpose_unknown_context`, `owner_purpose_uppercase`, `owner_purpose_padded`, `owner_purpose_long`, `owner_purpose_vietnamese`).
6. `_pair_context_or_404` → 404 `notebook_not_found` (`stranger_known_purpose_unknown_context`, `stranger_revokes_in_pair`, `owner_revokes_in_group`).
7. `_require_pair_permission("revoke_pair_consent", {"is_self": True})` (`service.py:7471`; `services/api/app/domain/permissions.py:541`): chỉ còn role → 403 `role_not_permitted` (`owner_revokes_as_advancer`).
8. `_locked_notebook` (`service.py:7326-7341`); không có chu kỳ sống → 204 không làm gì (`owner_revokes_on_fresh_pair`, `owner_revokes_after_close`).

## Đầu vào

- Path `context_id` (UUID), `purpose` (chuỗi, so khớp chính xác `lap_so` / `bat_doi` / `doc_chat`). Không thân.

## Đầu ra

**204**, thân rỗng, không `content-type`. Mọi nhánh thành công đều 204, kể cả không có gì để rút (`mate_revokes_doc_chat_again`, `owner_revokes_never_granted`).

## Tác dụng phụ

- Pair chưa có hàng sổ: **`pair_notebooks` được chèn và commit** dù không có gì được rút (`_locked_notebook`; `owner_revokes_on_fresh_pair`).
- Có chu kỳ sống (`service.py:7474-7485`), cùng `now`:
  - `pair_consents.revoked_at` = `now` cho **mọi** hàng của người gọi, purpose đó, chu kỳ đó, đang `granted_at IS NOT NULL AND revoked_at IS NULL`, khoá `FOR UPDATE OF pair_consents` (`repository.py:7872-7901`). Lời đề nghị không đổi (`completed_at` giữ nguyên).
  - `bat_doi`: xoá `active_couple_members` của **cả hai** participant (`repository.py:7916-7920`; `mate_revokes_bat_doi`), kể cả khi người gọi không có đồng ý sống.
  - `doc_chat`: mọi tờ `nhap` của context có phiên bản 1 `author_type='nep'` → `state='bo'` (`service.py:7486-7494`). Lát 1 không có cửa sinh tờ Nếp, nên nhánh này không ghi gì trong kịch bản.
  - `lap_so`: chu kỳ **vẫn `active`**; sổ vẫn nhận tờ mới (`owner_revokes_active_lap_so`, `mate_reads_after_lap_so_revoke`, `owner_drafts_after_lap_so_revoke` là 201).
- Rút `lap_so` lúc chu kỳ `pending`: đồng ý của người hỏi bị rút, người kia đồng ý sau đó không làm chu kỳ `active` (`owner_revokes_pending_lap_so`, `mate_grants_revoked_lap_so`, `owner_reads_still_pending`).
- Từ chối (404/403/401/422) rollback, kể cả hàng sổ.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}`, `loc` `["path","context_id"]` | `main.py:318-351` |
| 404 | `consent_purpose_unknown` | `Không có mục đích này.` | `service.py:7469` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 404 | — | `{"detail":"Not Found"}` (path không khớp) | Starlette |
| 422 | `idempotency_key_reuse` / `invalid_idempotency_key` | như các POST | `idempotency.py` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:101-115`
- Service: `services/api/app/api/service.py:7463-7494`
- Repository: `services/api/app/api/repository.py:7872-7901` (`revoke_consents`), `:7916-7920` (`clear_couple_member`), `:8155-8173` (`set_paper_state`)
- Domain: `services/api/app/domain/pair_notebook.py:58`; `services/api/app/domain/permissions.py:541`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_taking_back_one_half_of_a_couple_ends_it_for_both` (155), `test_one_cannot_revoke_the_other_persons_grant` (169), `test_an_unknown_purpose_is_not_a_door` (177)
- `services/api/tests/api/test_companion_pair_consent.py` (rút `doc_chat` và nháp Nếp)
- `services/api/tests/postgres/test_pair_notebook_postgres.py`: `test_thu_hoi_ma_chua_tung_dong_y_bi_tu_choi` (181), `test_dong_y_bi_thu_hoi_thi_khong_con_la_mot_doi` (230)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/DELETE-contexts-context_id-notebook-consents-purpose.yaml` (44 bước, `dev`):

- Thứ tự: `anonymous_revokes_unknown_purpose`, `stranger_unknown_purpose_unknown_context`, `stranger_known_purpose_unknown_context`, `stranger_revokes_in_pair`, `owner_revokes_in_group`, `owner_revokes_as_advancer`.
- Sổ chưa có: `owner_revokes_on_fresh_pair` (hàng sổ mới hiện ở làn DB; không đọc lại bằng GET vì thứ tự `participants` ngẫu nhiên khi chưa có chu kỳ).
- Cách viết path: `owner_purpose_uppercase`, `owner_purpose_padded`, `owner_purpose_encoded_underscore`, `owner_purpose_encoded_slash`, `owner_purpose_long`, `owner_purpose_vietnamese`.
- Chu kỳ: `owner_revokes_pending_lap_so`, `mate_grants_revoked_lap_so`, `owner_reads_still_pending`, `mate_revokes_doc_chat`, `owner_reads_after_chat_revoke`, `mate_revokes_doc_chat_again`, `owner_revokes_never_granted`, `mate_revokes_bat_doi`, `owner_reads_after_couple_revoke`, `owner_revokes_active_lap_so`, `mate_reads_after_lap_so_revoke`, `owner_drafts_after_lap_so_revoke`.
- Header key: `owner_revokes_with_header_key`, `owner_replays_header_key`, `owner_header_key_other_purpose`. Sau đóng: `owner_revokes_after_close`.

`crossreplay/DELETE-…-consents-purpose.yaml` (16 bước): 204 lưu qua Python được cửa trước phát lại; purpose khác cùng khoá 422; 404 `consent_purpose_unknown` nhả khoá.

`concurrency/DELETE-…-consents-purpose.yaml` (10 bước): ba lần rút `bat_doi` cùng lúc → ba 204, hai hàng cặp đôi biến mất một lần.

`pair_notebooks/prod-auth.yaml`: `anonymous_unknown_purpose` (401 trước purpose), `owner_revokes_doc_chat`, `stranger_revokes`, `junk_bearer_revokes`.

Corpus sinh tự động: hoãn, `path purpose is str`.

## Chưa phủ / lưu ý cho bản Go

- Kiểm purpose đi **trước** context và quyền, nhưng **sau** actor (401 trước 404 purpose).
- So khớp purpose trên path đã giải mã phần trăm, phân biệt hoa thường; `%2F` không bao giờ tới handler.
- Nháp Nếp bị bỏ khi rút `doc_chat` không phủ được bằng HTTP (không có cửa tạo tờ `author_type='nep'`).
- 204 được middleware lưu như mọi 2xx.

## Lỗi Python (chỉ báo, không sửa)

- Một DELETE trên pair chưa từng mở sổ để lại một hàng `pair_notebooks` đã commit.
- Rút `lap_so` không đóng, không tạm dừng chu kỳ: sổ vẫn `active`, vẫn nhận tờ và ràng buộc. `granted_purposes` không còn `lap_so` nhưng không ai đọc nó cho việc gì.
- Rút `lap_so` lúc `pending` làm lời đề nghị đó không bao giờ hoàn tất được (người hỏi không đồng ý lại được, xem thẻ grant); phải hỏi lại.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `DELETE /contexts/{context_id}/notebook/consents/{purpose}`: Route này đọc nội dung tờ qua `_noi_dung_wire` (hoặc chỉ bị chạm theo tên hàm): `place_id` đã lưu trả nguyên chữ. Mọi hàng ghi trước 23/09 đều là UUID dạng chuẩn (`model_dump` của `uuid.UUID`), nên byte trả lời không đổi với dữ liệu cũ; hàng mang `place_id` không phải UUID trước đây là 409 `paper_wrong_state`, nay đọc được.
