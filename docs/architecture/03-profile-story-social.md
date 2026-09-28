# Hồ sơ là cuốn sổ có nhiều ngã rẽ

Ngày 2026-09-23. Hồ sơ kể chuyện bằng dấu mốc đã có trong sản phẩm: check-in,
`destination_id`, ảnh, bài kể và chuyến đi chung. Người dùng chọn hướng khám phá;
máy chủ Go tính tiến độ và quyết định trao huy hiệu từ dữ liệu PostgreSQL. Nếp
chỉ gợi ý và viết một câu dẫn sau khi người dùng xem trước dữ liệu sẽ gửi và
đồng ý cho **lần gọi đó**. Không gửi ảnh, bài viết, bạn đồng hành, vị trí chi
tiết hoặc chat cho Nếp.

## Những lối đi

| Lối | Kết đầu tiên | Dấu mốc để nhận |
|---|---|---|
| Dấu chân | Một ngày nhiều ngã | Hai nơi khác nhau trong một chuyến |
| Dấu chân | Bản đồ mở | Hai `destination_id` khác nhau trong hai chuyến có điểm đến |
| Kỷ niệm | Những tấm ảnh còn đây | Ảnh kỷ niệm ở ba ngày khác nhau |
| Kỷ niệm | Chuyện mình kể | Bài kể đủ dài ở ba ngày khác nhau |
| Đồng hành | Hẹn rồi lại hẹn | Cùng một người check-in ở hai chuyến |
| Đồng hành | Đủ mặt hôm nay | Hai chuyến có bạn cùng đi, một chuyến ít nhất ba người |

Bốn huy hiệu mở đầu ghi nhận check-in, ảnh, bài kể và lần đi cùng đầu tiên. Ba
tuyến trên mở sáu kết khác nhau. Kết đã nhận luôn ở lại sổ, kể cả khi đổi hướng
hoặc chơi lại một nhánh.

**Thứ tự chọn có ý nghĩa.** Dấu chân → Kỷ niệm mở cơ hội “Bản đồ thành trang”
khi đã có kết ở cả hai tuyến và một ảnh ở nơi từng check-in. Kỷ niệm → Đồng
hành mở “Kỷ niệm chung” khi có hai kết tương ứng và ảnh trong nhóm có chuyến đi
chung. Bốn chiều chuyển còn lại tạo cảnh dẫn và mục tiêu tiếp theo khác nhau;
chúng không tự trao kết. Khi đạt một kết của cả ba tuyến và một trong hai ngã rẽ
trên, người dùng có thể nhận kết “Hành trình của mình”. Vì vậy hai người có
cùng số check-in vẫn có thể nhìn thấy trang tiếp theo khác nhau do lựa chọn
trước đó.

## Huy hiệu mang lại điều gì

Huy hiệu đã đạt có thể chọn tối đa ba chiếc để trưng bày trên hồ sơ. Người xem
khác chỉ nhận ba huy hiệu chủ hồ sơ chọn và chỉ khi quan hệ cho phép xem; tiến
độ, nhánh chưa mở và lượt video là riêng tư.

Kết đầu tiên ở mỗi tuyến Dấu chân, Kỷ niệm, Đồng hành và Ngã rẽ cấp một lượt
dựng MP4 cho Nếp. Kết cuối cấp thêm một lượt, tối đa năm nguồn cấp. Một lần
dựng chỉ dùng ảnh do chính người đó đã tạo bằng Nếp; Go giữ sổ cấp lượt, đặt
chỗ nguyên tử và hoàn lượt khi job hỏng. Video hoàn thành ở thư viện riêng của
chủ sở hữu. Tên các mẫu sáng tạo trên thẻ huy hiệu hiện là định hướng thiết kế;
giao diện ghi “sắp dùng được” cho tới khi bộ dựng mẫu tương ứng được nối vào
worker. Không quảng cáo mẫu chưa nối như quyền đã dùng được.

## Tường và cuộc trò chuyện quanh ảnh

Tường có phân trang theo cursor, phản ứng like chung với bài cũ, bình luận cha
và một tầng trả lời, like cho từng bình luận, đăng lại với quyền xem bài gốc.
Nhấn vào ảnh mở ảnh toàn màn hình cùng khay bình luận; soạn lời đáp ở đó quay
về đúng chuỗi của bài. Khay chia sẻ cho phép đăng lại lên tường ở mức người xem
đã chọn. Gửi vào chat chờ chat v2 mã hoá đầu cuối, không gửi văn bản rõ qua
đường chat legacy.

Người đang xem tường nhận sự kiện từ PostgreSQL qua long poll tối đa 20 giây;
`LISTEN` đánh thức sớm, còn đọc lại bảng sự kiện bền vững sau reconnect.
Client làm mới nền khi có sự kiện và đối soát mỗi lần poll hết hạn, giữ nội
dung hiện tại khi mạng tạm lỗi. ACL vẫn được kiểm trên từng lần đọc và trên
trích đoạn bài gốc của bài đăng lại.

## Ảnh review với dữ liệu giả

Bộ [13 huy hiệu ở cỡ 96/48 px](../assets/profile-badges/contact-sheet.png)
được xem trên nền sáng và tối. Ba ảnh chụp trình duyệt ở bề rộng 390 px ghi lại
[hồ sơ và tường](../assets/profile-badges/review/profile-390.png),
[bản đồ ngã rẽ](../assets/profile-badges/review/achievement-390.png) và
[ảnh cùng bình luận cha/con](../assets/profile-badges/review/photo-comments-390.png).
Tài khoản, bài viết và bình luận trong ảnh đều là dữ liệu tổng hợp để review.

## Ranh giới triển khai

`services/core/ownership/routes.json` chỉ rõ Go sở hữu các route mới.
`achievementv1`, `socialv2`, `profilemedia` là ba writer Go/SQL độc lập; Python
chỉ làm suy luận Nếp. Migration Go chạy tường minh sau Alembic. Kho Python cũ
vẫn phục vụ route legacy và làm oracle cho parity. Kiểm tra thiết bị native,
tải nhiều người dùng và chất lượng file MP4 từ media proxy thật là các cổng
riêng trước khi bật production.
