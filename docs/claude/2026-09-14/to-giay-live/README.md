# Phase 4 «Nếp truyền giấy» — một vòng trọn vẹn trên máy thật

Bảy ảnh dưới đây là **một lượt Maestro duy nhất**, chạy hết từ đầu đến cuối trên máy ảo
Android với API thật (`e2e_slice.sh --keep`) và **hai tài khoản thật** đăng nhập bằng OTP.
Không fixture, không cửa dev: bản dựng này tắt `EXPO_PUBLIC_RUDI_FIXTURE`.

Flow: `apps/mobile/.maestro/47-to-giay-hai-nguoi.yaml` · lượt xanh, `rc=0`, 7/7 ảnh.

| Ảnh | Ai đang cầm máy | Màn |
|---|---|---|
| `47a-chua-co-so.png` | C | Không gian giấy khi chưa có sổ |
| `47b-bac-lap-so.png` | C | Tờ đồng ý bậc «Lập sổ hai người» |
| `47c-ho-de-nghi.png` | D | **Phía người NHẬN lời đề nghị** |
| `47d-so-da-mo.png` | D | Sổ đã mở, màn mời viết tờ đầu |
| `47e-ban-phac.png` | D | Bản phác máy chủ dựng |
| `47f-nhan-duoc.png` | C | Tờ vừa nhận, nút «Ừ» |
| `47g-da-chot.png` | C | Đã chốt |

## Hai lỗi chỉ máy thật tìm ra

**1. Người NHẬN lời đề nghị không có nút nào để đồng ý.** `47c` là ảnh của bản đã sửa. Trước
đó màn ấy hiện «Đã đề nghị. Chờ người ấy đồng ý» — nói về chính người đang đọc — và không có
gì để bấm. Bậc thang đồng ý đứng im ở đó, nên **không tờ giấy nào từng tới được**.

Vì sao không tầng nào khác thấy: trên bản trải nghiệm có một nút dev «(Bản trải nghiệm) Người
kia đồng ý», và nó chỉ tồn tại ở cây fixture. Mọi vòng đọc mù của Phase 2 nhìn thấy một đường
đi trọn vẹn. `dangCho` chỉ hỏi «có lời đề nghị đang chờ không», không hỏi «của ai» — máy chủ
vẫn luôn trả `proposed_by_id`, không ai đọc nó.

**2. Ngày ISO in thẳng ra màn.** Wire mang `2026-09-19` (phải thế: máy chủ so ngày ấy với hôm
nay để quyết «đã tới ngày đi chưa»), fixture Phase 2 thì ghi sẵn «Thứ Bảy 20/09». Trên máy,
tờ giấy hiện `2026-09-19` giữa thân, và câu trạng thái hiện «Đã chốt. Hẹn 2026-09-19.» Hai
chỗ, cùng một nguyên nhân. `to-giay/ngay.ts` đổi nó thành «Thứ Bảy 19/09»; bảng chữ nằm trong
mã chứ không gọi `toLocaleDateString`, vì locale của máy sẽ cho hai người đang nhìn cùng một
buổi tối đọc ra hai thứ tiếng.

## Ba cái bẫy của lượt đo, ghi lại vì chúng tốn thời gian thật

- **Maestro không nghe `ANDROID_ADB_SERVER_PORT`.** Emulator chạy trên server 5038, Maestro
  mở server riêng ở 5037 và thấy **không máy nào**, rồi treo 25 phút mà không chạy một bước.
  Log của nó dừng ở dòng «System Info». Chữa: cho cả hai dùng cổng mặc định.
- **`case` chọn flow bỏ qua file mới trong im lặng.** Flow 47 không có trong danh sách nên hai
  lượt bảng (80 phút) in «đã chạy 23 flow», không dòng đỏ nào, và tính năng chưa từng được lái.
  `tests/test_maestro_flows_are_all_reachable.py` đóng lỗ ấy.
- **Bảng đầy đủ dừng ở flow 45** nếu thiếu `MOBILE_DATABASE_URL`, trước khi tới flow mới.

## Còn nợ

Ba phép đo trong lát (spec 20.5) và pixel-diff màn hội bạn chưa chạy. Đường hội bạn không đổi
byte nào theo cấu tạo — hàng ghim chỉ mount khi `laPair`, và flow 30 (chat nhóm thật) xanh
trong cùng lượt bảng — nhưng đó là lập luận, không phải phép đo, và nó được ghi ở đây như thế.
