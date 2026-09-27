# ADR-0039 — Khép cuộc đi, giữ lại một câu chuyện

**Trạng thái:** Chấp nhận theo quyết định của leader trong phiên 2026-09-27.
**Bổ sung:** ADR-0022, ADR-0027, ADR-0031, ADR-0036.

## Quyết định sản phẩm

- Quan hệ chỉ còn Hội bạn (từ hai người) và Cặp đôi có đồng thuận. `pair`
  là định danh lưu trữ/hội thoại, không tự khẳng định tình cảm. Sổ bạn bè cũ
  hiển thị như hội hai người; không đổi khóa, consent hay mở lịch sử cho người mới.
- Cả hai quan hệ dùng Khoảnh khắc cho buổi ngắn, Sổ chuyến đi cho du lịch.
  Nhiều ngày gợi ý chuyến đi, cùng ngày gợi ý khoảnh khắc; người dùng xác nhận.
- Người tổ chức khép cuộc đi; cặp đôi cho cả hai, hội cho chủ hội thay khi
  người tạo đã rời hội. Đây là trạng thái cuộc đi, không phải hoàn tất đợt thu.
- Mỗi thành viên tự giữ một diary trên tường mình. Mặc định chỉ mình tôi;
  công khai là hành động riêng sau xem trước. Không đăng lên tường người khác.
- Mọi ảnh nhóm mà người đó được đọc có thể đưa vào diary, kể cả bản công khai.
  Không xin phép từng tác giả; chủ diary gỡ hoặc thay ảnh khi được yêu cầu.
  Công khai diary không mở URL ảnh gốc của nhóm. Thu hồi hiển thị áp dụng cả byte ảnh.

## AI và quyền truy cập

Người dùng bấm Dựng sổ, xem đúng gói ảnh/ngữ cảnh sắp gửi, loại mục không muốn
chia sẻ rồi xác nhận. Không gửi chat là lựa chọn đầy đủ. Chat E2EE chỉ giải mã
trên thiết bị; không cấp khóa, quyền đọc chat hay quyền tự đọc lịch sử cho AI.
Nguồn là dữ liệu, không phải chỉ dẫn. AI không quyết định quyền, không viết sổ cái,
không tự công khai. Kết quả là schema trang giới hạn có tham chiếu nguồn, không là mã.

Go/SQL sở hữu toàn bộ vòng đời, lưu trữ, tác vụ, kiểm quyền và xuất bản.
Python chỉ thêm inference diary tại seam brain hiện có. Bản dựng tự động là
nháp riêng, được sửa trước khi lưu; dựng lại không ghi đè bản đã sửa.
Nội dung gửi AI có hạn lưu, được xóa khỏi tác vụ sau khi xử lý hoặc hết hạn.
Khi tài khoản bị xóa, migration Go xóa cả sổ, mọi phiên bản và tác vụ AI của
người đó; không giữ lại nội dung riêng trong các bảng phụ.

## Thiết kế và chuyển đổi

Giữ thế giới giấy, mực và nếp gấp. Giọng ấm, có duyên, gọn; nút gọi đúng hành động.
Diary có bìa, trang ảnh/ghi chú xen kẽ; khoảnh khắc là một trang ngắn, tồn tại lâu dài.
Motion có điểm nhấn lúc mở/khép/lưu, tôn trọng Reduce Motion. Nếp vẫn đứng xa tiền.
Luồng và đường dẫn cũ tiếp tục đọc được; kho tờ giấy lịch sử không bị xóa.

## Bằng chứng cần có

Contract, PostgreSQL thật, retry và race; quyền đọc metadata/ảnh trước và sau
công khai/thu hồi; dữ liệu cũ; AI không bịa nguồn; E2E và ảnh native sáng/tối,
chữ lớn, Reduce Motion; gate clean-tree, canary, hai mutant trước khi lên main.
ADR xác lập hợp đồng, không tự chứng minh những cổng đó đã chạy.
