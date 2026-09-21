# Cửa Google native

Nút Google chỉ hiện trên Android/iOS có cấu hình build hợp lệ. Web và build
chưa cấu hình không có nút giả. Apple nằm ngoài phạm vi.

- `EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID`: OAuth client loại Web, dùng làm audience
  của ID token trên cả hai nền tảng.
- `EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID`: OAuth client iOS đúng bundle ID; cần thêm
  biến này trên iOS. `app.config.ts` sinh URL scheme trả về tương ứng.
- Android cần OAuth client Android đúng package `com.lakiet.rudi` và chữ ký
  của build được kiểm tra; không dùng chữ ký debug làm bằng chứng cho release.
- Máy chủ cấu hình `MOBILE_GOOGLE_CLIENT_IDS` với audience được chấp nhận.

Đây là ID public, không phải client secret. Giá trị thật vẫn đặt ngoài Git.
Sau đổi cấu hình native, prebuild/build/cài lại dev client; Metro reload không
đủ để thêm URL scheme hay module native. Không đặt email làm khóa ghép tài khoản.

Luồng: SDK chọn tài khoản → ID token → `/auth/google` → lưu phiên ứng dụng →
preferences khi là người mới, màn chính khi đã có tài khoản. Hủy chọn không
gọi API và không hiển thị lỗi. Lỗi có thể thử lại hoặc quay về OTP.

Đã kiểm logic gate/cancellation/exchange bằng ca tổng hợp. Chưa xác nhận
đăng nhập với OAuth thật, chữ ký phát hành hoặc thiết bị iOS.
