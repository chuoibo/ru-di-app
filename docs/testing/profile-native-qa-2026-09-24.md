# Kiểm chứng hồ sơ và hành trình trên Android · 2026-09-24–25

Kiểm tra bằng bản APK debug build từ nhánh `feat/profile-story-routes` trên máy ảo Android 15 (1080 × 2400), qua cửa trước Go và PostgreSQL dùng một lần. Toàn bộ tài khoản, bài viết và ảnh trong phiên này là dữ liệu tổng hợp. Ảnh chụp được mở và xem trực tiếp, gồm cả giao diện sáng, tối và cỡ chữ 130%.

| Luồng đã thao tác | Kết quả quan sát |
| --- | --- |
| Hai tài khoản đăng nhập bằng OTP, xem hồ sơ của mình và bạn bè | Hồ sơ, huy hiệu và quyền xem bài tải theo người đang đăng nhập. Bài `only_me` không hiện ở tường bạn bè. |
| Chọn một hướng trong sổ hành trình, mở/đọc dấu mốc, xem màn đồng ý cho Nếp gợi ý | Các ngã rẽ và tiến độ lấy từ Go theo dữ liệu từng người; gợi ý fallback hiện được khi dịch vụ AI chưa cấu hình. |
| Thích bài, bình luận gốc, trả lời bình luận, thích bình luận, chia sẻ lại lên tường | Các tương tác được phản ánh trên bài; thông báo chia sẻ thành công có màu nhấn thay vì màu lỗi. |
| Chọn ảnh, nén, tải lên, đăng bài rồi mở ảnh và bảng bình luận | Ảnh tải từ ứng dụng Android thành công và mở được; bình luận viết trong bảng dưới ảnh hiện lên bài. |
| Chọn ảnh và đăng story ngay trong Android, mở dải story rồi xem story vừa đăng | Ảnh được tải lên và hiện trong trình xem story của chính người đăng. Lát cắt Go/PostgreSQL riêng kiểm tra story mất quyền xem sau 24 giờ. |
| Một người đăng bài qua API trong lúc người còn lại đang mở tường trên Android | Bài mới hiện trên màn đang mở sau khoảng 3–5 giây qua cập nhật realtime. |
| Hai Android đăng nhập hai tài khoản, B đứng trên tường A rồi A đăng bài mức Bạn bè trong ứng dụng | Bài xuất hiện trên B sau 6 giây, không cần kéo làm mới. B thích và bình luận; tường đang mở trên A tự đổi từ `0 thích · 0 bình luận` thành `1 thích · 1 bình luận`. |
| A đăng story bằng ảnh chọn trong Android, B mở dải story và xem bằng Android thứ hai | B thấy vòng Minh Anh sau khi chuyển lại tab Tin nhắn, vòng chưa xem có màu nhấn. B mở được đúng ảnh A đã tải lên; trình xem có hai thanh tiến độ cho hai story còn hạn. PostgreSQL ghi story mới với tác giả A. |
| Đổi sang dark mode và cỡ chữ 130% | Profile, sổ hành trình và bảng bình luận ảnh vẫn đọc được; nút Nếp né vùng thao tác của hồ sơ. |

Ảnh chụp đã xem trực tiếp gồm profile ở cỡ chữ 130%, sổ hành trình nền tối, ảnh toàn màn hình kèm bình luận, bài có bình luận cha/con sau khi chia sẻ, story vừa đăng trong trình xem của A và B, vòng story chưa xem ở máy B và tường A trước/sau thao tác từ Android B. Chúng được lưu ngoài repository của phiên QA; repo guard yêu cầu review và allowlist riêng cho mỗi ảnh nhị phân trước khi đưa vào Git.

Giới hạn của lần kiểm chứng: lượt đăng story đầu sau khi khôi phục stack chỉ ghi ảnh tải lên, không ghi story; lặp lại cùng thao tác thì story được tạo và B xem được. Chưa tái hiện được lần đầu để quy nguyên nhân, nên độ ổn định của thao tác đăng story cần được theo dõi thêm. Việc khôi phục stack sinh khóa danh tính mới làm một lệnh OTP QA tạo tài khoản khác và bị từ chối khi dùng ảnh của A; lệnh API kiểm tra sau đó dùng phiên thử gắn đúng tác giả, rồi thao tác Android A/B xác nhận luồng thật. Lượt hai Android chứng minh cập nhật bài mới, lượt thích và số bình luận trên tường đang mở; chưa đo tải nhiều người hoặc mọi kiểu mất kết nối/kết nối lại. Dịch vụ suy luận AI và dựng video chưa cấu hình trong stack tổng hợp nên chỉ kiểm chứng đường gợi ý fallback và quyền lợi lượt dựng MP4, chưa kiểm chứng chất lượng gợi ý AI hay file video đầu ra. Chưa chạy iOS, kiểm chứng crypto độc lập, kiểm thử tải hoặc Maestro. Những cổng đó vẫn là việc riêng trước khi phát hành.
