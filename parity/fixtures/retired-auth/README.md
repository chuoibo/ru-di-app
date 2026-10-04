# Oracle đăng nhập đã nghỉ

Các scenario này là bằng chứng lịch sử của danh tính số điện thoại, Google cũ
và lời mời đổi phiên. ADR-0055 gỡ các cửa ấy; không còn nạp chúng từ
`parity/scenarios/`. Giữ nguyên yêu cầu cũ để đối chiếu, không nhận chúng làm
nghiệm thu hiện tại. Tài khoản mới là Go-only, đo qua PostgreSQL/Redis thật,
client HTTP, Android và web; không thêm writer Python để làm parity xanh.

Các scenario phiên đọc/xoá trong chế độ prod vẫn nằm trong suite đang chạy.
