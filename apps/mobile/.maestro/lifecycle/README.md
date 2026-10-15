# Kiểm vòng đời native có fixture sẵn

`map-ready.yaml` chạy app Android đã đăng nhập trong stack QC riêng, với
`KEO_ID`, `CTX_ID`, `TIEU_DE`, `CHANG` của kèo/chặng danh mục tổng hợp.
Ba vòng tắt/mở → chọn chặng → bản đồ → pan/khớp → timeline kiểm lời gọi
projection trước khi MapLibre native sẵn sàng. Flow không ghi API hoặc reset
phiên. Giữ file ngoài glob `*.yaml` ở gốc để bảng sản phẩm không tự chạy flow
thiếu fixture/biến. Chỉ receipt native và ảnh đã mở mới chứng minh lượt chạy;
parse/typecheck không chứng minh không crash.

`map-cluster.yaml` dùng hai chặng cùng tọa độ, thêm biến `FIRST_STOP_ID`,
`CHON_2` (nhãn lựa chọn thứ hai đầy đủ) và `CHANG_THU_HAI`. Flow chọn chặng
thứ hai và Back khỏi bảng lựa chọn; không ghi API. Screenshot phải cho thấy
cả hai lựa chọn trên native, không chỉ tìm chuỗi trong cây trợ năng.
