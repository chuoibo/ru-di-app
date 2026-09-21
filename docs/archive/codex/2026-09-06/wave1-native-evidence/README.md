# Bằng chứng Android — checkpoint nền UI, 06/09/2026

Ảnh từ APK debug Expo SDK 57/RN 0.86.3, AVD `rudi-qa3`, serial
`emulator-5560`, Android 15/API 35. Không phải ảnh browser hay mockup.
Dữ liệu và tên hiển thị là fixture demo của app; ảnh stock đã có trong repo,
không phải người tham gia hoặc ảnh thật của địa điểm được ghi trên card demo.

| Ảnh | Cấu hình và phạm vi |
|---|---|
| [Gán món phone](phone-assignment.png) | 1080×2400, density 420, font 1,0×, sáng; profile QA sạch user 11. Sau fix caption teal; 2 cột roster, một món mở. |
| [Tạo phone](phone-create-dark.png) | 1080×2400, density 420, font 1,0×, tối; ba hành động. |
| [Gán món tablet](tablet-assignment-dark.png) | 1800×2400, density 320, font 1,3×, tối; 3 cột roster. Ảnh này trước fix màu caption tổng bill. |
| [Tạo tablet](tablet-create-dark.png) | 1800×2400, density 320, font 1,3×, tối; sheet giới hạn 560dp, không kéo giãn toàn màn. |
| [Grid Khám phá](tablet-explore.png) | 1800×2400, density 320, font 1,3×, sáng; đã cuộn tới grid, 2 cột trong phần còn lại sau rail. Chỉ chứng minh bố cục, không chứng minh mapping ảnh–địa điểm. |
| [Viewer đổi chiều rộng](tablet-viewer-resize.png) | Font 1,3×, sáng; mở ảnh 1, vuốt ảnh 2, chạm đúp rồi đổi chiều rộng 1600→1800 px. Ảnh 2, số trang và caption vẫn khớp; sau đó vuốt về ảnh 1 được. |

App có khóa portrait; ca viewer kiểm tra đổi kích thước cửa sổ bằng `wm size`,
không gọi đó là kiểm tra landscape. Không có bằng chứng pinch hai ngón từ bộ
ảnh này: phép giới hạn offset được test bằng hàm thuần và đọc worklet, chưa
phải đo gesture đa điểm thật. Không đo FPS/haptic trên phần cứng thật.

Ảnh dùng cùng JS đang triển khai nhưng khác thời điểm trong lượt kiểm tra.
Fingerprint đầu là `codex-ui-wave1-c29c54f`, cuối là
`codex-ui-wave1-2fb215f`: đây là mốc nền cộng diff local, không phải build
đã phát hành. Không suy ra iOS, OAuth thật, timeline ghi API hoặc album riêng
tư live từ fixture và probe này.

Lệnh hỗ trợ lưu tại `apps/mobile/tools/native-wave-proof.mjs`; trước ghi ảnh
phải tìm đúng text/label trong hierarchy. Main agent đã mở xem từng PNG;
reviewer Impeccable xem ảnh trong `.impeccable/review/wave1/`, các file ở đây
được copy nguyên byte để bàn giao. Ảnh sai màn/tải chưa xong không đưa vào đây.
