# ADR-0028 — Hành trình có xem trước, giờ ghim và điểm hẹn của hội

Đổi từ số 0027 khi gộp main để không trùng ADR sổ hai người đã được gộp trước.

- Trạng thái: triển khai theo kế hoạch được Lead yêu cầu ngày 2026-09-12;
  chờ review độc lập trước merge/phát hành.
- Thay thế ADR-0026 §2.2–2.4 về nguồn routing, phép tính và điểm hẹn.

## Quyết định

Lịch trình và Hành trình cùng đọc một danh sách chặng. Chặng giữ ID khi đổi
giờ hoặc thứ tự, nên check-in không biến mất. Ngày, thời lượng, giờ ghim và
điểm hẹn do hội chọn là dữ liệu kế hoạch. Chuyến một ngày cũ nhận ngày đã biết;
chuyến nhiều ngày chưa có ngày cho từng chặng phải được chia ngày trước khi
đánh giá lịch. Giờ cũ mặc định được ghim, thời lượng thiếu vẫn là thiếu.

Điểm hẹn được người dùng chọn và xác nhận trên bản đồ, chỉ thuộc chuyến đi và
chỉ thành viên hội đọc được. Không nhập vào danh mục công khai, không lấy GPS,
không thay đổi hợp đồng check-in. Không ghi tọa độ vào log hay gửi sang dịch
vụ routing công khai. Tên hội và nội dung chặng không rời backend sang routing.

Valhalla tự vận hành tính đường theo xe máy đô thị, ô tô hoặc đi bộ cho từng
ngày. Phiên bản engine và graph được ghim; phép so sánh dùng cùng nguồn và
cấu hình. Ma trận có hướng dùng để tìm ứng viên, sau đó tính tuyến của cả hai
phương án trước khi công bố tiết kiệm. Không dùng tỷ lệ chim bay suy ra km
đường bộ, không khẳng định tối ưu tuyệt đối hoặc có dữ liệu giao thông trực tiếp.
Thiếu tuyến không được biến thành ETA đường bộ. Có giới hạn và thử lại rõ ràng.

Xem trước không ghi kế hoạch. Áp dụng và hoàn tác là các lần ghi có revision
và idempotency, khóa hàng outing trong transaction. Bản sửa cũ bị từ chối khi
revision đã đổi. Endpoint timeline cũ chỉ sửa hợp đồng v1, vẫn tăng revision;
kế hoạch đã nâng cấp v2 yêu cầu client hỗ trợ hợp đồng mới. Điểm đã check-in
không bị planner tự chuyển giờ/thứ tự/địa điểm.

## Hình ảnh và tương tác

Giữ giấy–mực, typography và ký hoạ của Rủ Đi. Bản đồ là địa lý thật, số mốc và
chiều tuyến giúp hiểu ngày. Bảng thông tin đo kích thước thật để camera không
che điểm. So sánh hiện tại/gợi ý dùng cùng khung nhìn và liệt kê giờ thay đổi.
Nếp là lớp tháo được ở cửa vào/ngày trống. Chữ lớn, screen reader và Reduce
Motion là đường dùng chính thức.

## Kiểm chứng và phát hành

Test thuật toán đối kháng với ma trận có hướng; tuyến thực cho từng phương
tiện; PostgreSQL thật cho revision, ID/check-in, quyền và migration lên–xuống–lên.
Kiểm native sáng/tối, font lớn, màn rộng, mất mạng và compare/apply/undo. Báo cáo
tách source, fixture, live và giới hạn thiết bị. Feature flag tắt đề xuất vẫn
giữ lịch trình đã lưu; reviewer độc lập quyết định merge.
