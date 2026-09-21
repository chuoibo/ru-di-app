# Xác nhận bản sửa Assessment B

Assessment B độc lập: `/root/chat_visual_b`. Không đọc báo cáo A hay A-confirm; kết quả được giữ riêng đến khi root xác nhận A đã chốt.

**APPROVE có giới hạn: đóng B1, B2, B3 của báo cáo B trên Expo web đã kiểm.** Không mở rộng thành phê duyệt thiết kế toàn sản phẩm, native, E2EE hay phát hành.

## Định danh và cách kiểm

- URL của root: `http://127.0.0.1:8178`; dùng Chrome headless mới và browser context mới, đăng nhập tài khoản tổng hợp qua OTP UI. Không dùng browser cũ hoặc chèn token.
- Script entry được đọc lại từ DOM: `entry-5c477e5372d98f5ce4adba7628905d24.js`, khớp export `web-reviewfix` được giao. SHA-256 đầy đủ của asset và bảy source liên quan nằm trong `manifest.json`; source không đổi từ lúc chụp manifest đến lúc hoàn tất.
- Viewport 320×740, touch/mobile emulation, reduced motion. Đổi `prefers-color-scheme` thật qua Chrome: sáng → tối trên khay đang mở; từ tối → sáng trên màn tạo kèo. Đo màu thực tế và tỷ lệ tương phản, không chỉ nhìn ảnh.
- Artifact: `/tmp/rudi-chat-e2e.JpMlFu/design-b-confirm/`. Có `results.json`, ảnh trước overlay, ảnh overlay, console, JSON rule, mẫu màu, cây accessibility của sheet, script và manifest. Đây là artifact runtime ngoài checkout.
- Không có cửa sổ người dùng trực tiếp: **không nhãn `[Human]`**. Bằng chứng là browser automation và ảnh headless, không phải ảnh Android/iOS.

## Đóng lỗi

| Lỗi B | Kiểm lại thực tế | Kết luận |
|---|---|---|
| **B1 · Scheme pha màu** | Header chat, nhãn lời nhờ và icon đóng đều đổi sang palette tối; tiêu đề, nhãn tên và icon Back của tạo kèo đổi lại palette sáng. Màu được so bằng RGB cụ thể với palette mong đợi; các mẫu đồng thời vượt ngưỡng tương phản. | **Đóng trong phạm vi web đã kiểm.** |
| **B2 · Sheet để lọt focus/nền AX, không Escape** | Mở sheet đưa focus tới “Đóng bảng”; 14/14 Tab và 3/3 Shift+Tab nằm trong dialog. `aria-modal=true`; composer nền có ancestor `inert` và `aria-hidden=true`. Cây Chrome accessibility không còn công khai composer, tin nhắn hay lựa chọn bình chọn nền. Escape đóng dialog, bỏ inert nền và trả focus về “Cài đặt nhóm”. | **Đóng.** |
| **B3 · Cắt lựa chọn màu tại 320 px** | Đủ năm radio 52×52 nằm trọn trong panel và viewport. Bốn ô hàng đầu, ô thứ năm xuống hàng tại `(16, 523)`; không còn cắt mép phải. | **Đóng.** |

### Số đo cho B1

Các mẫu tính tương phản từ màu chữ và nền thực sau khi resolve ancestor trong DOM, có xử lý alpha. Đo riêng nhãn/icon từng bị mất chữ; không dùng số của detector khi detector chọn sai nền nút gradient.

| Mẫu | Chữ/icon | Nền | Tỷ lệ |
|---|---|---|---:|
| Header chat sáng | `#1f2230` | `#f7f3ec` | **14,28:1** |
| Nhãn lời nhờ và icon đóng, sáng | `#1f2230` | `#ffffff` | **15,79:1** |
| Header chat sau đổi tối | `#f4f1ea` | `#151830` | **15,45:1** |
| Nhãn lời nhờ và icon đóng sau đổi tối | `#f4f1ea` | `#1f2340` | **13,56:1** |
| Tiêu đề Kèo mới, nhãn tên và Back sau đổi lại sáng | `#1f2230` | `#f7f3ec` | **14,28:1** |

Ngưỡng kiểm: 4,5:1 cho chữ, 3:1 cho icon. Cả kiểm đúng palette lẫn kiểm tương phản đều đạt. Ảnh đối chiếu: `02-tray-light.png`, `03-tray-after-dark.png`, `04-outing-after-light.png`.

## Detector và kiểm soát nhiễu

Tái sử dụng lượt CLI ban đầu cho bốn target cũ, không quét lại để tạo thêm số xanh. Chạy **một lượt bổ sung** trên phần dùng chung mới sửa:

```sh
node .claude/skills/impeccable/scripts/detect.mjs --json \
  apps/mobile/src/rudi/ui/GiaoDienProvider.tsx \
  apps/mobile/src/rudi/theme.ts \
  apps/mobile/src/rudi/ui/Sheet.tsx \
  apps/mobile/src/rudi/screens/chat/CaiDatNhom.tsx
```

Node 22.23.2; exit **0**, JSON `[]`, stderr rỗng. Không có rule/vị trí CLI phát sinh. Không coi lượt cũ là quét mới các thay đổi ngoài phạm vi B.

Preflight mutation đổi title/append script thành công. Server detector riêng tại 8472; `detect.js` được inject thật và chạy trên ba trạng thái, chờ khoảng 2,8 giây rồi lưu ảnh overlay. Console được giữ nguyên trong `detector-console.json`:

| Trạng thái | Nhóm cảnh báo / overlay | Rule |
|---|---:|---|
| Khay sáng | 8 / 8 | `text-overflow` ×1, `low-contrast` ×1, `layout-transition` ×3, `text-occlusion` ×3 |
| Cùng khay sau đổi tối | 12 / 12 | `text-overflow` ×1, `low-contrast` ×5, `layout-transition` ×3, `text-occlusion` ×3 |
| Tạo kèo sau đổi lại sáng | 6 / 6 | `clipped-overflow-container` ×2, `low-contrast` ×1, `layout-transition` ×3 |

Không gọi 26 nhóm trên là 26 lỗi thật:

- Hai `text-overflow` là tên nhóm dài đang rút gọn bằng ellipsis tại 320 px; không phải chữ chồng.
- Bốn cảnh báo dark `low-contrast` đo icon radio ngoài vùng cuộn là 4,1:1 nhưng áp ngưỡng chữ 4,5:1. Icon hữu ích dùng ngưỡng 3:1; các cảnh báo đó không tái hiện B1.
- Hai cảnh báo ở nút gửi AI đang disabled và một cảnh báo nút Tạo kèo lấy nền container thay vì mặt nút. Ảnh thường cho thấy mặt nút khác nền container. Các mẫu B1 được đo độc lập ở bảng trên.
- `layout-transition`, `text-occlusion` của danh sách cuộn/đảo chiều và hai wrapper `clipped-overflow-container` vẫn là tín hiệu cấu trúc web đã phân biệt trong báo cáo B đầu; lượt này không có phép đo frame time hay bằng chứng popover bị cắt để nâng thành lỗi mới.

Không dùng detector để suy ra mọi thành phần đều đạt WCAG; chỉ đóng các mẫu và hành vi B đã kiểm trực tiếp.

## Giới hạn và kết thúc

Harness đầu dùng nhãn hành động cũ “Tự viết tờ hẹn”, trong khi bản sửa đã đổi thành “Tự tạo kèo”, nên dừng sau ba kiểm màu đầu. Giữ nguyên `phase1-results.json`; tiếp tục phần còn lại trong Chrome/context mới bằng nhãn hiện tại, không lặp lại detector của hai trạng thái đã có. Kết quả tổng hợp ghi rõ `instrumentationNote`, không che lỗi harness hoặc đổi assertion để cho qua.

Tổng cộng 12 assertion ghi trong `results.json` đều đạt; trong đó có hai lần xác nhận cùng entry asset. Không có page error trong các đoạn được theo dõi. Không gửi tin, tạo kèo, đổi màu nhóm hay sửa source sản phẩm trong lượt xác nhận này.

Đã đóng các browser riêng, dừng detector 8472 và kiểm không còn listener. Server 8178 của root giữ nguyên. Chưa kiểm font scale native, bàn phím mềm, TalkBack/VoiceOver, các dialog lồng nhau hay toàn bộ biến thể theme; cần cổng riêng nếu phạm vi phát hành yêu cầu.

Questions skipped: phạm vi xác nhận B1–B3 đã rõ; không cần thêm lựa chọn của người dùng.
