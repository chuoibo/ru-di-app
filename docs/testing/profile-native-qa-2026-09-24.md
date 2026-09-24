# Kiểm chứng hồ sơ và hành trình trên Android · 2026-09-24

Kiểm tra bằng bản APK debug build từ nhánh `feat/profile-story-routes` trên máy ảo Android 15 (1080 × 2400), qua cửa trước Go và PostgreSQL dùng một lần. Toàn bộ tài khoản, bài viết và ảnh trong phiên này là dữ liệu tổng hợp. Ảnh chụp được mở và xem trực tiếp, gồm cả giao diện sáng, tối và cỡ chữ 130%.

| Luồng đã thao tác | Kết quả quan sát |
| --- | --- |
| Hai tài khoản đăng nhập bằng OTP, xem hồ sơ của mình và bạn bè | Hồ sơ, huy hiệu và quyền xem bài tải theo người đang đăng nhập. Bài `only_me` không hiện ở tường bạn bè. |
| Chọn một hướng trong sổ hành trình, mở/đọc dấu mốc, xem màn đồng ý cho Nếp gợi ý | Các ngã rẽ và tiến độ lấy từ Go theo dữ liệu từng người; gợi ý fallback hiện được khi dịch vụ AI chưa cấu hình. |
| Thích bài, bình luận gốc, trả lời bình luận, thích bình luận, chia sẻ lại lên tường | Các tương tác được phản ánh trên bài; thông báo chia sẻ thành công có màu nhấn thay vì màu lỗi. |
| Chọn ảnh, nén, tải lên, đăng bài rồi mở ảnh và bảng bình luận | Ảnh tải từ ứng dụng Android thành công và mở được; bình luận viết trong bảng dưới ảnh hiện lên bài. |
| Chọn ảnh và đăng story ngay trong Android, mở dải story rồi xem story vừa đăng | Ảnh được tải lên và hiện trong trình xem story của chính người đăng. Lát cắt Go/PostgreSQL riêng kiểm tra bạn bè thấy story còn hạn, mở được ảnh và mất quyền xem sau 24 giờ. |
| Một người đăng bài qua API trong lúc người còn lại đang mở tường trên Android | Bài mới hiện trên màn đang mở sau khoảng 3–5 giây qua cập nhật realtime. |
| Đổi sang dark mode và cỡ chữ 130% | Profile, sổ hành trình và bảng bình luận ảnh vẫn đọc được; nút Nếp né vùng thao tác của hồ sơ. |

Ảnh chụp đã xem trực tiếp gồm profile ở cỡ chữ 130%, sổ hành trình nền tối, ảnh toàn màn hình kèm bình luận, bài có bình luận cha/con sau khi chia sẻ và story vừa đăng trong trình xem. Chúng được lưu ngoài repository của phiên QA; repo guard yêu cầu review và allowlist riêng cho mỗi ảnh nhị phân trước khi đưa vào Git.

Giới hạn của lần kiểm chứng: luồng realtime dùng một người trên Android và một người gửi qua API, chưa phải hai Android đồng thời. Story được đăng và xem bằng Android của tác giả; quyền xem từ tài khoản bạn bè được kiểm tra ở lát cắt Go/PostgreSQL, chưa mở bằng Android thứ hai trong lượt này vì máy ảo và stack dùng một lần dừng sau khi đăng xuất. Dịch vụ suy luận AI và dựng video chưa cấu hình trong stack tổng hợp nên chỉ kiểm chứng đường gợi ý fallback và quyền lợi lượt dựng MP4, chưa kiểm chứng chất lượng gợi ý AI hay file video đầu ra. Chưa chạy iOS, kiểm chứng crypto độc lập, kiểm thử tải hoặc Maestro. Những cổng đó vẫn là việc riêng trước khi phát hành.
