# Ảnh kiểm thử chat legacy trên Android

**Bằng chứng lịch sử trước UI mới.** Packet thuộc source `ad350d41`, không
kiểm chứng bản “Sổ hẹn của hội” export `entry-5c477e5372d98f5ce4adba7628905d24.js`.
Cổng native của UI mới vẫn còn mở.

Năm ảnh nguyên bản từ `adb screencap`, emulator riêng Android 15/x86_64,
development APK dựng lại tại máy bằng JDK 21. Account, tên và tin đều do
HTTP fixture tổng hợp tạo; không phải backend Go thật hoặc điện thoại thật.
Digest, kích thước và PNG metadata được ghi trong [manifest](manifest.json).
Không có EXIF/text metadata; parent và reviewer độc lập đã xem từng ảnh.

| Trạng thái | Ảnh |
|---|---|
| Bàn phím và danh sách slash | [31](31-rebuilt-keyboard-slash.png) |
| Tin dài thất bại, bản nháp kế tiếp còn nguyên | [32](32-rebuilt-long-failed.png) |
| Lỗi tổng hợp trong dark, font 1.3 trước retry | [32b](32b-rebuilt-dark-failure.png) |
| Sau retry: bubble đã gửi, bản nháp mới vẫn còn | [33](33-rebuilt-retry-success.png) |
| Dark/font 1.3, header, tin nhiều dòng và composer | [34](34-rebuilt-dark-font13.png) |

Reviewer Impeccable trả **ship chỉ cho UI legacy trong các trạng thái này**.
Không suy ra MLS/E2EE, hoạt động bot thật, reduced motion, hiệu suất, ổn định
native hoặc production readiness. Một cold launch đạt trên APK mới không
xoá finding crash ở APK cũ khi chưa có kiểm chứng rộng hơn.

Lần chụp 33/34 đầu không đúng trạng thái (fixture dừng; theme đổi làm rời
chat); đã loại khỏi packet. Các ảnh tại đây là bản chụp lại đã được xác minh.
Xem [báo cáo đầy đủ](../../../../apps/mobile/docs/chat-native-evidence-2026-09-21.md)
và [hướng dẫn development build](../../../../apps/mobile/docs/chat-development-build.md).
