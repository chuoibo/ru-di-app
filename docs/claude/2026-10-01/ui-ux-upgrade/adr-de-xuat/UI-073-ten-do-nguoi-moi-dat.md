# Đề xuất ADR: tài khoản vào bằng lời mời là «người mới», và tên người mời đặt chưa phải tên công khai (QA UI-073)

- Trạng thái: **đề xuất**, chưa triển khai. Viết 01/10/2026 trong đợt nâng cấp UI/UX từ audit PR #663.
- Người quyết: chủ sản phẩm. Thuộc nhóm blocker «quyền riêng tư / đồng thuận».

## Vấn đề

Lời mời tạo trước một hàng `people` kèm tên do người mời đặt (hồ sơ tạm, có chủ đích). Khi chính chủ đăng nhập lần đầu,
máy chủ coi họ «không mới» vì hàng đó đã có:

- Python `app/api/service.py:3977` (`is_new = person is None`).
- Go `internal/domain/authsteps/otpdoor.go:255`.

Hệ quả QA đo được:

- Người được mời vào thẳng Tin nhắn, bỏ qua Sở thích, không thấy và không sửa được tên người khác đặt cho mình.
- Sau «Đồng ý», cả nhóm thấy họ dưới tên đó, có thể là một biệt danh: «Bạn thân từ hồi cấp ba của mình, người hay trễ
  hẹn nhất hội», «Khôi bạn đi dạo».
- Ở mốc đo, một người lạ tra số cũng thấy tên đó. `main` đã đổi luật tra số ở `dd75752`; phần này cần đo lại.

## Phương án

| Phương án | Hệ quả |
|---|---|
| A. `is_new_person` = lần đăng nhập **đầu tiên** của danh tính (chưa từng có phiên), không phải «chưa có hàng người» | Người được mời đi qua Sở thích như mọi người mới. Sửa Go + Python oracle + goldens + parity của `/auth/otp/verify` |
| B. Thêm cờ «tên do chính chủ xác nhận». App coi người chưa xác nhận tên là mới, và Sở thích hiện ô tên điền sẵn: «Nhóm đang gọi bạn là …» | Cần một cột và một route đọc cờ. Tách được «gu» với «tên» |
| C. Tên chưa được chính chủ xác nhận không hiện cho người ngoài nhóm đã mời (tra số, gợi ý kết bạn) | Khép phần lộ ra ngoài, kể cả khi A/B chưa xong |

## Khuyến nghị

- **A + C ở máy chủ.**
- **App** (B8 của đợt này): khi app biết đây là người mới, Sở thích có ô tên điền sẵn tên người mời đặt, ghi rõ
  «Nhóm đang gọi bạn là …», sửa được trước khi vào nhóm.

B chỉ cần khi chủ sản phẩm muốn tách «đã nói gu» khỏi «đã xác nhận tên».

## Tiêu chí xong (theo QA)

- Số được mời đăng nhập lần đầu: vào Sở thích, ô tên điền sẵn và sửa được.
- Tra số: không lộ tên người khác đặt khi chính chủ chưa xác nhận.

## Phần UI đã làm (B8, 02/10/2026), không đổi luật máy chủ

- App đoán phía mình: phiên `is_new_person = false` nhưng nhóm mặc định đang ở `invited` (chưa vào nhóm nào) thì coi là
  người mới. Người đó đi qua Sở thích với `?moi=1` (`duong-vao.ts`, `manSauDangNhap`).
- Sở thích hiện ô «Bạn tên gì?» điền sẵn tên nhóm đang gọi, kèm câu «Nhóm mời bạn đang gọi bạn là «…». Sửa nếu bạn muốn
  được gọi khác.». Giữ nguyên thì không ghi gì; sửa thì `PUT /people/{id}` như ô tên của người mới.
- Giới hạn của cách đoán: người từng có nhóm, rời hết nhóm, rồi được mời lại cũng đi qua Sở thích một lần (bỏ qua được),
  và ô tên điền sẵn tên của chính họ. Phương án A hoặc B ở trên mới phân biệt đúng hai trường hợp.
- Phần tra số (phương án C) không đổi: vẫn là quyết định của chủ sản phẩm.
