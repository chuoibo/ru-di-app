# GET /contexts/{context_id}/unbatched-expenses

batches · core · **chỉ Go** (`python: absent`) · trạng thái trong bộ nhớ: không có

## Mục đích

Đếm số khoản chi của nhóm mà một đợt thu mới sẽ gom, để màn quyết toán chỉ mời «Tạo đợt thu từ sổ» khi máy chủ sẽ không
từ chối nó bằng `409 no_unbatched_allocations` (QA UI-058, đợt nâng cấp UI/UX 01/10/2026, batch B4).

- Trước đây nút hiện khi còn ai nợ ai. Danh sách chuyển tính từ số dư, gồm cả khoản đã vào đợt. Chạm vào thì bị từ chối,
  số đợt vẫn 1 → 1.
- Chỉ đọc, chỉ một con số đếm, không đọc số tiền.
- Không đổi luật tiền nào. Phép chọn giống hệt `moneysteps.FreezeBatch`: bản mới nhất của mỗi khoản có phân bổ đã xác
  nhận, và không dòng thu được nào (không phải phần của người trả, lớn hơn 0) đã làm nguồn cho một nghĩa vụ.
- Đọc không khoá dòng. Lối tạo đợt dùng `FOR UPDATE`; một màn chỉ cần biết có hay không.

Vì sao chỉ Go: `GET /contexts/{context_id}/batches` còn Python làm oracle (`python: live`). Thêm trường vào đó thì phải
thêm nghiệp vụ mới vào Python, trái quy tắc ADR-0031. Route mới thuộc về Go.

## Xác thực và quyền

Như `GET /contexts/{context_id}/batches`:
- `get_actor`, rồi path `context_id` (UUID);
- rồi `view_collection_board` (`is_group_member`) **trước mọi truy vấn**: nhóm không tồn tại và nhóm không phải của mình
  đều bị từ chối như nhau.

## Đầu ra

`200` `{"context_id": "…", "unbatched_expense_count": N}`. Thứ tự khoá cố định; `N` là số nguyên ≥ 0.

## Bằng chứng

- Mã: `services/core/internal/repo/balances.go` (`CountUnbatchedExpenses`), `services/core/internal/routes/batches.go`
  (`countUnbatchedExpenses`).
- PostgreSQL thật: `services/core/internal/repo/unbatched_postgres_test.go`, chạy qua `scripts/go_postgres_tier.sh`.
  - Khoản sửa hai bản còn nợ thì được đếm.
  - Khoản đã làm nguồn cho nghĩa vụ không được đếm, bản không có phân bổ cũng không, nhóm khác cũng không.
  - Sau khi khoản cuối vào đợt: 0.
- Người dùng: `apps/mobile/src/rudi/screens/Bill.tsx` đọc số này cho nút «Tạo đợt thu từ sổ».
