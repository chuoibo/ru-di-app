# Đề xuất ADR: «đã chia» của một kèo tính theo kèo, không theo ngày (QA UI-149)

- Trạng thái: **đề xuất**, chưa triển khai. Viết 01/10/2026 trong đợt nâng cấp UI/UX từ audit PR #663.
- Người quyết: chủ sản phẩm. Chạm luật tiền 3 («số dư tính lại được từ sổ»), nên đợt này không sửa mã.

## Vấn đề

Khoản chi gắn vào kèo bằng ngày: `on_date BETWEEN outings.starts_on AND outings.ends_on`, trong cùng nhóm
(Go `services/core/internal/repo/recap.go:127`, oracle Python `services/api/app/api/repository.py:3012`). Bảng
`expenses` không có cột kèo.

Hai kèo trùng ngày thì một khoản chi được đếm ở cả hai. QA đo được:

- Một khoản chi 13.705.678đ hiện «đã chia 13.705.678đ» ở cả hai kèo trên kệ album.
- Sau khi cả hai kèo xong, hero quyết toán của nhóm ghi **27.411.356đ** cho một sổ chỉ có 13.705.678đ (`TC-N15-Q4-HERO`).

App (`src/rudi/doc-live.ts:126 tongTuRecap`) cộng tổng các kèo đã xong, nên con số sai lên đến màn tiền.

Parity vẫn xanh vì Go và Python cộng trùng giống nhau. Python là oracle nên bản sửa phải đi qua cả hai, cùng kịch bản
parity.

## Phương án

| Phương án | Hệ quả |
|---|---|
| A. Khoản chi mang `outing_id` (nullable). Khoản chi ghi từ màn của một kèo thì mang kèo đó. Hàng cũ chỉ được gán khi đúng **một** kèo phủ ngày đó; còn lại là «chưa thuộc kèo nào» | Mỗi khoản chi thuộc đúng một kèo hoặc không kèo nào. Cần migration, sửa Go + Python oracle + goldens + parity, và một câu cho mục «chưa thuộc kèo nào» |
| B. Giữ nối theo ngày nhưng chọn **một** kèo cho mỗi khoản (kèo ngắn nhất phủ ngày đó; hoà thì kèo tạo trước) | Không đổi schema. Vẫn là đoán: một bữa tối trong chuyến 3 ngày có thể bị gán vào kèo cà phê cùng tối |
| C. Hero quyết toán của nhóm đọc tổng **từ sổ** (Σ khoản chi của nhóm), không cộng tổng các kèo | Sửa ngay số sai trên màn tiền, độc lập với A hay B. Đổi cách một con số tiền được hiện, nên vẫn cần chủ sản phẩm duyệt |

## Khuyến nghị

**C trước, rồi A.**

- C làm con số trên màn tiền khớp sổ, đúng luật 3.
- A làm «đã chia» của từng kèo đúng nghĩa.

B không được khuyến nghị: nó thay một cách đếm sai bằng một cách đoán.

Đợt này chỉ ghi đề xuất. B4 không đụng allocator, sổ cái, hay cách tính tổng.

## Tiêu chí xong (nếu chọn A + C)

- Hai kèo trùng ngày, một khoản chi ghi từ kèo thứ nhất: kèo thứ hai «đã chia 0đ».
- Σ «đã chia» các kèo ≤ tổng sổ của nhóm.
- Hero quyết toán = Σ khoản chi của nhóm, tính lại từ sổ.
- Có ca parity cho hai kèo trùng ngày.
