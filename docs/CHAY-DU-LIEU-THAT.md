# Chạy app Rủ Đi trên dữ liệu thật (Android emulator)

Ghi ngày 20/10/2026. `<IP-máy-dev>` là địa chỉ LAN của máy dev đang chạy stack vnlocal và Metro (hỏi chủ máy; không ghi vào repo). App chạy trên dữ liệu thật: ~18.600 quán, ~18.700 ảnh quán, 33 điểm đến.
Không còn nhóm/chuyến/người dùng mẫu. Bạn đăng ký tài khoản của chính mình và dùng như người dùng thật.

## Bạn nhận được
| Thứ | Ở đâu |
|---|---|
| `rudi-dev-client-x86_64.apk` (~140 MB, dev client cho emulator x86_64) | chủ máy chuyển; build `apps/mobile` (`expo run:android`) hoặc xin file cùng SHA256 |
| Máy chủ API | `http://<IP-máy-dev>:8124` (máy dev, cùng mạng LAN) |
| Máy chủ JS (Metro) | `http://<IP-máy-dev>:8081` — **máy dev phải đang bật Metro**, nếu app báo không tải được bundle thì nhắn chủ máy |

Cần: Android emulator **x86_64**, API 33 trở lên (máy dev dùng system image android-35), có mạng tới `<IP-máy-dev>`.
Emulator chạy trên máy KHÁC máy dev vẫn được nếu hai máy cùng mạng LAN (emulator NAT ra mạng thật).
Nếu tester ở ngoài LAN: không dùng được bản này — báo chủ máy để cấp đường Tailscale.

## Cài và mở app (3 lệnh)
```bash
adb install -r -d rudi-dev-client-x86_64.apk
adb shell am start -a android.intent.action.VIEW \
  -d "rudi://expo-development-client/?url=http%3A%2F%2F<IP-máy-dev>%3A8081"
```
Nếu màn «Development servers» hiện ra thay vì vào thẳng app: bấm **Enter URL manually**, nhập
`http://<IP-máy-dev>:8081`, bấm Connect. Lần đầu bundle tải ~1 phút, các lần sau nhanh.

Nếu emulator và Metro nằm CÙNG một máy: dùng `adb reverse tcp:8081 tcp:8081` và
`adb reverse tcp:8124 tcp:8124` cũng được, nhưng app đã trỏ API về `<IP-máy-dev>:8124` nên không bắt buộc.

## Đăng nhập — như người dùng thật
- Không có «bản trải nghiệm». Mở app là màn đăng nhập.
- **Đăng ký bằng email thật của bạn**: tên đăng nhập + email + mật khẩu → hệ thống gửi mã xác minh qua email thật
  (kiểm cả thư rác) → nhập mã. Hoặc **Đăng nhập Google** (tài khoản Google thật).
- Không có mã OTP cố định/debug. Không xin chủ máy mã.
- Mỗi tester dùng tài khoản riêng. Chat nhóm mã hoá cần **ít nhất 2 tài khoản trên 2 emulator/thiết bị**.

## Nên thử (dữ liệu thật)
1. **Khám phá**: lướt quán theo điểm đến; mở trang quán — ảnh, giờ mở cửa, giá, chỉ đường; lưu quán.
2. **Tìm quán**: gõ tự nhiên («quán cà phê yên tĩnh ở Đà Lạt», «phở sáng Quận 1»); thử gõ không dấu, thử dị ứng/ăn kiêng.
   Chú ý: kết quả sai/lạc đề, quán đã đóng cửa, ảnh trống/vỡ, giờ sai.
3. **Tạo nhóm → tạo chuyến → thêm điểm dừng từ quán thật**; mời người thứ hai; chat; chia tiền.
4. **Hồ sơ/Cài đặt**: đổi tên, đăng xuất, đăng nhập lại; thiết bị nhắn tin mã hoá.

## Lưu ý
- Tìm kiếm và Nếp (AI) gọi dịch vụ AI trả tiền: dùng bình thường, **đừng chạy vòng lặp/tự động hoá hàng loạt**.
- Mọi dữ liệu bạn tạo là dữ liệu thật trên máy chủ dùng chung; đừng nhập thông tin cá nhân thật của người khác.
- Dữ liệu quán cập nhật liên tục từ nguồn (giờ/giá có thể đổi giữa hai lần mở).

## Khi gặp lỗi, gửi kèm
- Giờ xảy ra (đến phút), màn hình, các bước, ảnh chụp màn hình.
- `adb logcat -d -t 300 > logcat.txt` ngay sau lỗi.
- Tên tài khoản bạn đã dùng (không gửi mật khẩu).
- Nếu «Unable to load script» / trắng màn hình: kiểm `curl http://<IP-máy-dev>:8081/status` phải trả `packager-status:running`.
