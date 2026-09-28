# ADR-0040 — Cộng đồng chia sẻ cuộc đi

Trạng thái: chấp nhận theo kế hoạch người dùng duyệt ngày 27/09/2026.

Cộng đồng là tab riêng, feed kiểu Threads, cùng bài/like/comment với tường cá
nhân. Công khai mới nghĩa là gửi cộng đồng và phải qua kiểm duyệt. Bạn bè là
quan hệ kết bạn hiện tại; theo dõi và tag không cấp quyền. Bài cũ không tự vào
cộng đồng. Chỉ mình tôi và Nhóm vẫn tồn tại. Nhật ký nháp luôn riêng.

Go/SQL sở hữu API, worker, quyền, media và xuất bản. Python chỉ inference.
Nội dung công khai liên quan cuộc đi, ăn uống, địa điểm, văn hóa, du lịch;
bình luận được kiểm an toàn và spam theo ngữ cảnh. Mất AI không có nghĩa được
duyệt. Quản trị viên được cấp riêng, mọi quyết định có audit và phiên bản.
Không gửi dữ liệu thật ra dịch vụ ngoài; inference cộng đồng dùng endpoint
nội bộ do người vận hành cấu hình, không tự dùng Gemini của luồng khác.

Một bài lưu ở `posts`; nội dung đang xét nằm trong phiên bản riêng. Bài mới
chờ duyệt có audience vật lý `only_me`; phiên bản đã duyệt mới được công khai.
Ảnh/video cộng đồng có đường đọc kiểm quyền riêng, không mở URL nhóm gốc.
Sửa bài tạo revision; worker phải đối chiếu revision trước khi quyết định.

Realtime dùng HTTP ghi và WebSocket nhận tín hiệu sau commit. PostgreSQL giữ
outbox/sự kiện; Redis chỉ truyền tín hiệu đánh thức. Client bỏ trùng, tải lại
theo quyền hiện tại, phục hồi sau reconnect và không tự đẩy vị trí cuộn.
Thu hồi phiên/quyền phải áp dụng cả kết nối đang mở. Không dùng stream chat.

Cá nhân hóa hỏi trước, chỉ dùng sự kiện cộng đồng. Có tắt, xóa lịch sử và
không quan tâm. Dữ liệu thô tối đa 90 ngày. Nếp chỉ chạy khi được gọi, xác nhận
đúng nguồn; trả nháp, không tự đăng. Không đọc chat/nhật ký riêng để gợi ý.

Frontend dùng Impeccable, kế thừa DESIGN.md. Chuyển động có Reduce Motion;
ảnh chụp native và đo frame trên thiết bị thật là bằng chứng riêng.

Mốc kiểm thử: 10.000 socket, 1.000 feed read/s, 200 mutation/s; p95 feed
500 ms, mutation 700 ms, realtime sau commit 2 s. Burst 2x, soak 24 giờ.
Đây là điều kiện nghiệm thu, không phải năng lực đã được chứng minh.

Gates: contract, PostgreSQL thật, E2E đa tài khoản/đa instance, native, media
revocation, moderation, race, tải, clean-tree SHA, canary và hai mutant.
Tiến độ và bằng chứng nằm ở docs/testing/cong-dong.md.
