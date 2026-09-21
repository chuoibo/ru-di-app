# POST /contexts/{context_id}/notebook/proposals

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Xin một bậc của thang đồng ý (spec §6.1, ADR-0027 K4): `lap_so` (lập sổ), `bat_doi` (một đôi), `doc_chat` (cho Nếp đọc chat). Hỏi là đồng ý: route ghi lời đề nghị **và** đồng ý của chính người hỏi trong cùng giao dịch. Lời đề nghị `lap_so` đầu tiên của một cặp mở chu kỳ `pending`.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): `Idempotency-Key` rỗng hoặc dài hơn 255 → 422 `invalid_idempotency_key` (`owner_empty_header_key`); cùng scope và khoá, thân khác → 422 `idempotency_key_reuse` (`owner_header_key_other_body`); đã hoàn tất 2xx → phát lại (`owner_replays_header_key`); đang chạy quá 5 giây → 409 `idempotency_request_in_flight`. Scope dev là giá trị thô `X-Actor-ID` (`mate_uses_owner_header_key` chạy thật), prod là digest bearer.
2. JSON hỏng → 422 trước actor.
3. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 (`anonymous_proposes`), 422 `invalid_actor_id`/`invalid_actor_roles`.
4. Validation path và thân `PairProposalCreateRequest` (`services/api/app/api/schemas.py:2763-2764`) **trước khi tra context**: `stranger_unknown_purpose_in_pair` là 422 dù người gọi không ở pair.
5. `_pair_context_or_404` (`services/api/app/api/service.py:7215-7237`) → 404 `notebook_not_found` (`stranger_proposes_unknown`, `owner_proposes_in_group`, `stranger_proposes_in_pair`, `leaver_proposes_after_leaving`).
6. `_require_pair_permission("propose_pair_consent", {"is_group_member": True})` (`service.py:7356-7358`; `services/api/app/domain/permissions.py:526`): chỉ còn role → 403 `permission_denied` `role_not_permitted` (`owner_proposes_as_advancer`).
7. `_locked_notebook` (`service.py:7326-7341`): `SELECT … FOR UPDATE` hàng `pair_notebooks`; chưa có thì `INSERT` rồi khoá lại. Từ đây các lệnh ghi sổ của cùng pair xếp hàng.
8. Purpose khác `lap_so` khi `cycle_state != 'active'` (không có chu kỳ, chu kỳ `pending`, hoặc sau khi đóng) → 409 `consent_missing` `Cả hai cùng đồng ý lập sổ trước đã.` (`service.py:7362-7372`; `owner_proposes_bat_doi_first`, `owner_proposes_doc_chat_first`, `owner_proposes_bat_doi_while_pending`, `owner_proposes_bat_doi_after_close`). Kiểm **trước** khi mở chu kỳ.
9. Không có chu kỳ sống: ít hơn hai membership đang hoạt động → 409 `cycle_not_active` `Sổ này chưa đủ hai người.` (`service.py:7373-7375`; `owner_proposes_in_lone_pair` sau khi người kia xoá tài khoản). Ngược lại mở chu kỳ `pending` với hai người đó (`repository.py:7765-7788`).

## Đầu vào

- Path `context_id`.
- Thân JSON, `extra="forbid"`: `purpose: Literal["lap_so","bat_doi","doc_chat"]` bắt buộc. Hoa thường phân biệt (`owner_purpose_uppercase`); `null`, số, mảng, thiếu, trường lạ đều 422 (`owner_purpose_null`, `owner_purpose_number`, `owner_purpose_list`, `owner_purpose_missing`, `owner_extra_field`).
- Header `Idempotency-Key` tuỳ chọn.

## Đầu ra

**201** `PairProposalResponse` (`schemas.py:2734-2739`), JSON gọn, thứ tự khoá `id`, `purpose`, `expires_at`, `proposed_by_id`, `my_granted` (luôn `true`). `expires_at` = `now + 7 ngày` (`services/api/app/domain/pair_notebook.py:49`, `:72-74`), ISO-8601 UTC đuôi `Z`.

## Tác dụng phụ

Trong một giao dịch, cùng `now` (`service.py:7343-7396`):

- `pair_notebooks` (lần ghi đầu của cặp): `id` uuid4, `context_id`, `context_kind='pair'`, `created_at`.
- Khi chưa có chu kỳ sống: `pair_notebook_cycles` (`state='pending'`, `terms_version=1`, `opened_at`/`closed_at` NULL, `created_at`) và hai `pair_cycle_participants` (`created_at` = `now`).
- `pair_consent_proposals`: `cycle_id`, `purpose`, `proposed_by_id`, `terms_version=1` (`service.py:8033`), `completed_at` NULL, `created_at`, `expires_at`.
- `pair_consents`: `proposal_id`, `person_id` = người hỏi, `granted_at` = `now`, `revoked_at` NULL, `created_at` (`repository.py:7832-7861`).
- `idempotency_keys` khi có header và 201.
- Không trùng lặp: `lap_so` hỏi lại lúc `pending` hoặc `active` tạo **thêm** lời đề nghị trong cùng chu kỳ (`owner_proposes_lap_so_again`, `mate_proposes_lap_so`, `owner_proposes_lap_so_while_active`).
- Sau khi đóng sổ, `lap_so` mở **chu kỳ mới**; lời đề nghị của chu kỳ cũ không còn hiện (`owner_proposes_lap_so_new_cycle`, `owner_reads_new_cycle`).
- Mọi từ chối rollback, kể cả hàng `pair_notebooks` vừa chèn (`services/api/app/api/deps.py:196-209`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-494` |
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 409 | `consent_missing` | `Cả hai cùng đồng ý lập sổ trước đã.` | `service.py:7369-7372` |
| 409 | `cycle_not_active` | `Sổ này chưa đủ hai người.` | `service.py:7374` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:67-82`
- Service: `services/api/app/api/service.py:7343-7396`, `:7326-7341`, `:7215-7237`
- Repository: `services/api/app/api/repository.py:7748-7763` (`lock_pair_notebook`), `:7740-7746`, `:7765-7788`, `:7806-7826`, `:7832-7861`
- Domain: `services/api/app/domain/pair_notebook.py:49-74`
- Schema: `services/api/app/api/schemas.py:2533`, `:2734-2739`, `:2763-2764`
- Bảng: `services/api/app/db/models.py:2890-3104`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_one_yes_opens_nothing_and_the_other_side_can_see_whose_turn_it_is` (80), `test_a_higher_rung_is_refused_until_the_notebook_is_open` (130), `test_tier_two_does_not_carry_tier_three` (136), `test_a_closed_notebook_can_be_opened_again_as_a_new_agreement` (318)
- `services/api/tests/postgres/test_pair_notebook_postgres.py`: `test_mot_so_chi_co_mot_chu_ky_chua_dong` (144), `test_han_cua_loi_de_nghi_phai_sau_luc_tao` (290), `test_muc_dich_ngoai_tu_vung_bi_tu_choi` (309)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/POST-contexts-context_id-notebook-proposals.yaml` (48 bước, `dev`):

- Ngoài sổ và thứ tự: `anonymous_proposes`, `stranger_proposes_unknown`, `owner_proposes_in_group`, `stranger_proposes_in_pair`, `stranger_unknown_purpose_in_pair`, `owner_proposes_as_advancer`.
- Thang: `owner_proposes_bat_doi_first`, `owner_proposes_doc_chat_first`, `owner_proposes_lap_so`, `owner_proposes_lap_so_again`, `mate_proposes_lap_so`, `owner_proposes_bat_doi_while_pending`, `mate_grants_p1`, `owner_proposes_bat_doi`, `mate_proposes_doc_chat`, `owner_proposes_lap_so_while_active`, `owner_reads_offers`.
- Thân: `owner_purpose_uppercase`, `owner_purpose_missing`, `owner_purpose_null`, `owner_purpose_number`, `owner_extra_field`, `owner_purpose_list`.
- Header key: `owner_header_key`, `owner_replays_header_key`, `owner_header_key_other_body`, `owner_empty_header_key`, `mate_uses_owner_header_key`.
- Một người: `leaver_ends_account`, `owner_proposes_in_lone_pair`, `leaver_proposes_after_leaving`.
- Đóng rồi mở lại: `owner_previews_close`, `owner_closes`, `owner_proposes_bat_doi_after_close`, `owner_proposes_lap_so_new_cycle`, `owner_reads_new_cycle`; chặn: `mate_blocks_owner`, `owner_proposes_after_block`.

`parity/scenarios/w8/crossreplay/POST-contexts-context_id-notebook-proposals.yaml` (14 bước): 201 lưu qua Python được cửa trước phát lại; purpose khác cùng khoá 422; 409 `consent_missing` nhả khoá, nên sau khi sổ mở cùng khoá chạy thật.

`parity/scenarios/w8/concurrency/POST-contexts-context_id-notebook-proposals.yaml` (10 bước): ba `lap_so` cùng lúc trên sổ đã có hàng → ba 201, một chu kỳ; ba `bat_doi` cùng header key → một 201 và hai lần phát lại.

`pair_notebooks/prod-auth.yaml`: `junk_bearer_proposes`, `owner_proposes_with_header_key`, `owner_replays_header_key`, `mate_same_key_own_session`, `anonymous_with_owner_key`, `junk_bearer_with_owner_key`.

Corpus sinh tự động: `parity/scenarios/generated/w8-422/post-contexts-context_id-notebook-proposals.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Hàng `pair_notebooks` được tạo khi chưa có rồi khoá lại; **lần ghi đầu tiên** của hai request cùng lúc trên một pair chưa có hàng đụng `uq_pair_notebooks_context` (xem lỗi dưới). Kịch bản đồng thời tạo hàng trước bằng một lệnh thu hồi 204.
- 409 `consent_missing` đứng trước 409 `cycle_not_active`; cả hai sau 404 và 403.
- `expires_at` do Python tính, không có server default; CHECK `expires_at > created_at`.
- Hạn 7 ngày không đo được (không có bước đồng hồ).

## Lỗi Python (chỉ báo, không sửa)

- Hai request ghi đầu tiên cùng lúc vào một pair chưa từng có hàng sổ: cả hai thấy «chưa có», cùng `INSERT`, request thua nhận `IntegrityError` trên `uq_pair_notebooks_context` → 500. Phụ thuộc lịch chạy nên không có trong kịch bản.
- Không chống trùng: bấm «lập sổ» nhiều lần sinh nhiều lời đề nghị `lap_so` cùng lúc chờ trong một chu kỳ.
- Chặn không ngăn hỏi thêm bậc (`owner_proposes_after_block` là 201).
