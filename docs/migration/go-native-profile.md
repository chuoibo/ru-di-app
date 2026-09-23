# Các route hồ sơ mới do Go sở hữu

Ngày 2026-09-23, tính năng hồ sơ mở các route mới cho hành trình thành tựu,
tường nhà v2 và video dùng lượt thưởng. Đây là API nghiệp vụ mới nên Go/SQL là
writer duy nhất theo ADR-0031. Python không có route đối ứng: `python: absent`,
`state: GO-NATIVE` trong `services/core/ownership/routes.json` nói rõ điều đó.

Các route này đi qua Go extension mux trước bộ định tuyến legacy. Bảng ownership
vẫn là nguồn sự thật: `core routes --json` phải liệt kê mọi ID extension; bộ
kiểm tra ownership so danh sách đó với manifest. Go-only route không được ép
về Python bằng ID hoặc group. Chế độ `all` chỉ trả các route có bản Python
đối ứng về Python, còn route Go-only vẫn chạy tại Go.

`achievementv1` ghi thành tựu, lựa chọn nhánh và nguồn lượt video. `socialv2`
ghi lượt thích, bình luận, đăng lại và thông báo thay đổi. `profilemedia` ghi
đặt chỗ, trạng thái và kết quả video; khi job thất bại lượt được trả. Cả ba
schema được cài bằng lệnh migration Go tường minh, không chạy DDL khi server
nhận request. Kho Python cũ vẫn phục vụ các route legacy làm oracle đối chiếu.

Chứng cứ bắt buộc trước khi bật production: test PostgreSQL thật về quyền xem
ba tài khoản, hai thiết bị đua nhận huy hiệu/dùng lượt, thất bại media hoàn
lượt, đồng bộ tường nhà sau reconnect, cùng route ownership và canary theo
`AGENTS.md`. Kiểm tra native và file MP4 thật là cổng riêng.
