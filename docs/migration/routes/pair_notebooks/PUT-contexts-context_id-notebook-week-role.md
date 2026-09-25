# PUT /contexts/{context_id}/notebook/week-role

pair_notebooks · core · trạng thái trong bộ nhớ: không có · sinh ra đã là Go (ADR-0034 §2.4, 2026-09-25)

## Mục đích

«Anh lo / Em lo / Hôm nay mình share» cho tuần này trong sổ «Một đôi». Không chọn thì «Người lo» được
suy mỗi lần đọc sổ (`pair_notebook.nguoi_lo_suy`: ai gửi tờ trước ×2, ai đề nghị sửa ×1, trong chu kỳ
này; hoà → người lập sổ) và không lưu. Chọn chỉ quyết tuần đó đọc là lượt của ai, không cấp quyền gì.
Không có giới tính ở bất kỳ đâu.

## Xác thực và quyền

1. Idempotency, JSON hỏng, `get_actor` như các PUT khác của sổ.
2. Validation thân `PairWeekRoleRequest` (`extra="forbid"`): `lo` ∈ {`toi`, `nguoi_kia`, `ca_hai`}, khác → 422.
3. `_pair_context_or_404` → 404 `notebook_not_found` (người lạ, nhóm, context không có).
4. `_require_pair_permission("set_pair_week_role", {"is_group_member": True})` → 403 khi không có vai `member`.
5. `_locked_notebook`; chưa có chu kỳ, chu kỳ chưa `active`, hoặc chưa là «Một đôi» (bat_doi cả hai trên
   một đề nghị) → 409 `consent_missing` `Hai bạn bật «Một đôi» trước đã.`

## Đầu vào

Path `context_id` (UUID). Thân `{"lo": "toi" | "nguoi_kia" | "ca_hai"}`.

## Đầu ra

**200** `PairWeekRoleResponse`: `tuan` (thứ Hai của tuần, `pair_paper.tuan_cua`), `nguoi_lo` (danh sách
id; hai id khi share), `cach` = `chon`, `diem` (`[{person_id, score}]` theo thứ tự người tham gia).
`GET …/notebook` mang cùng khối ở `week_role` (null ngoài «Một đôi» đang mở; `cach` = `suy` khi chưa chọn).

## Tác dụng phụ

`set_pair_rhythm`: một hàng `pair_cycle_rhythms(cycle_id, tuan)` — chèn, hoặc sửa đúng cột đổi
(`nguoi_lo_id` NULL = share, `chon_boi_id`, `updated_at`). Rồi đọc lại tờ và lựa chọn để trả lời.
Xoá tài khoản: bảng GIỮ như mọi hàng của chu kỳ (`account_lifecycle` ERASURE keep).

## Lỗi

| Status | code | detail | Nguồn |
|---|---|---|---|
| 422 | (validation) | `lo` ngoài ba giá trị | `schemas.py` `PairWeekRoleRequest` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `_pair_roster_or_404` |
| 403 | `permission_denied` | `role_not_permitted` | `_require_pair_permission` |
| 409 | `consent_missing` | `Hai bạn bật «Một đôi» trước đã.` | `set_pair_week_role` |

## Mã Python

`services/api/app/api/routes/pair_notebooks.py` `set_pair_week_role`; `ApiService.set_pair_week_role`,
`_week_role`, `_paper_signals`; domain `pair_notebook.nguoi_lo_suy`, `vai_tuan`.

## Test đang phủ

- `tests/domain/test_pair_notebook.py` (5 ca người lo), `tests/api/test_pair_notebook.py` (3 ca route),
  quét người lạ `test_every_pair_route_refuses_a_stranger` (20 cửa).
- Golden Go↔Python: `pairnotebook/testdata/python_pair_notebook*.json` (ca `lo:*`, fuzz 200),
  `pairsteps/testdata/python_pair_steps*.json` (`set_pair_week_role` 7 ca đặt tên + cửa + fuzz;
  `pair_notebook` `role_*`).

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/chia-gu.yaml` (bước người lo: đọc suy luận, chọn, share).

## Chưa phủ / lưu ý cho bản Go

Go là bản đầu tiên phục vụ route này; Python là oracle cùng lúc (ADR-0029 §2.9).

## Lỗi Python (chỉ báo, không sửa)

Không có.

## Đổi 2026-09-25 (lượt 2) — gậy luân phiên, hạn mức tuần, «Một đôi» trên hồ sơ, 4 sticker đôi (ADR-0034 §2.4–2.5)

Diff này: (1) `vai_tuan` nhận `mo_loi_truoc` — người lo quen đã mở lời (tờ đầu tiên gửi trong tuần, `nguoi_mo_loi`) hai tuần liền thì tuần này sang người kia, `cach` = `luot`; tín hiệu tờ thêm `tuan`, `sent_at`. (2) `draft_pair_paper` từ chối 409 `paper_week_quota` khi người gọi đã phác `TO_MOI_NGUOI_MOI_TUAN` (3, cùng số với `packages/shared/nep-nhip.json`) tờ trong tuần. (3) `get_person_profile` trả `relation` = `couple` khi hai người là một «Một đôi» (`same_couple`, hỏi SAU cửa quyền, không phải oracle cho người lạ). (4) Từ vựng sticker thêm `hen-nhe`, `nho-nhau`, `ve-toi-chua`, `om-cai`. Golden pair_notebook (ca baton, 10 shard), pair_steps (`quota_*`, `role_baton_*`), people_steps (`couple_*`), stickers; Go 0 lệch.

- `PUT /contexts/{context_id}/notebook/week-role`: Câu trả lời có thể là `cach` = `luot` khi chưa chọn — ở đây luôn là `chon` vì vừa chọn; hàm dùng chung đổi chữ ký.
