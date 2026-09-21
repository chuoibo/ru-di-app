# Kiểm tra triển khai chat: Assessment B

Method: dual-agent, Assessment B độc lập (`/root/chat_visual_b`). Không đọc báo cáo hay điểm của A; chỉ gửi kết quả detector sau khi root xác nhận A đã hoàn tất.

**Kết luận trong phạm vi web: REQUEST_CHANGES.** Đã chạy một lượt CLI trên bốn tệp, đăng nhập OTP thật bằng tài khoản tổng hợp trong browser riêng, inject `detect.js` thành công trên ba trạng thái, rồi xác nhận các điểm còn nghi ngờ ở 320 px. Có lỗi tương phản khi đổi chế độ sáng/tối trong phiên và lỗi điều hướng bàn phím ở sheet cài đặt. Không có kết luận native, E2EE, hiệu năng hay chất lượng đầu ra AI từ lượt này.

## Phạm vi và nguồn bằng chứng

- Target: `GroupChatLive.tsx`, `TheAi.tsx`, `SoHen.tsx` trong `apps/mobile/src/rudi/screens/chat/`, cùng `apps/mobile/src/rudi/screens/keo/CreateOutingLive.tsx`.
- Đã đọc `AGENTS.md`, `.claude/skills/impeccable/SKILL.md`, `reference/critique.md`; chạy `context.mjs` một lần; đối chiếu `packages/shared/tokens.json`.
- Source đã đổi trong lúc hoàn tất báo cáo: `GroupChatLive.tsx` và `SoHen.tsx` (SHA trước/sau ở `source-manifest.json`); tác giả đang sửa song song. Các kết luận dưới đây gắn với SHA export `web-final` đã ghi, chưa xác nhận bản sửa mới. Vị trí source là điểm điều tra của phiên đọc, cần đối chiếu lại khi sửa.
- HEAD khi ghi báo cáo: `ef1ee46d`, worktree đang có thay đổi. SHA-256 của bốn target nằm trong `source-manifest.json`; không coi HEAD là định danh đủ cho nội dung chưa commit.
- Dùng Puppeteer/Chrome headless vì không có browser tool trực tiếp. Context và browser cuối cùng đều do B tạo riêng. Không gắn nhãn `[Human]`: không có cửa sổ tương tác được trình bày cho người dùng.
- Server riêng `127.0.0.1:8189` phục vụ **nguyên trạng** `/tmp/rudi-chat-e2e.JpMlFu/web-final`, không export/build lại. API tổng hợp hiện hữu; đăng nhập qua UI, không chèn token vào app và không mock phản hồi.
- Lượt chính: 390×844; lượt xác nhận: 320×740. Khởi động tối được kiểm riêng với đổi scheme giữa phiên. Không giả lập bàn phím mềm native hay font scale hệ điều hành.
- Artifact ngoài checkout: `/tmp/rudi-chat-e2e.JpMlFu/design-b/`. Có ảnh thường, ảnh overlay, JSON detector, console, DOM, cây Chrome accessibility và script tái hiện. Đây là artifact runtime, không bảo đảm tồn tại vĩnh viễn.

## Detector

Lệnh duy nhất:

```sh
node .claude/skills/impeccable/scripts/detect.mjs --json \
  apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx \
  apps/mobile/src/rudi/screens/chat/TheAi.tsx \
  apps/mobile/src/rudi/screens/chat/SoHen.tsx \
  apps/mobile/src/rudi/screens/keo/CreateOutingLive.tsx
```

Node 22.23.2; exit **0**; JSON **`[]`**; stderr rỗng. Không có rule hay vị trí dòng CLI để liệt kê. Kết quả này chỉ nói bộ rule tĩnh không tìm được lỗi trong bốn tệp, không chứng minh giao diện đạt accessibility.

Preflight ghi DOM thành công: đổi `document.title`, append script thực thi, đọc lại cờ và khôi phục title. Server Impeccable riêng chạy ở 8471; mỗi trạng thái đều nạp `http://localhost:8471/detect.js`, chạy detector, chờ khoảng 2,8 giây rồi chụp overlay. Console thật nằm ở `detector-console.json`.

| Trạng thái | Console / nhóm phần tử | Rule và số lượng | Artifact ảnh |
|---|---:|---|---|
| Chat sáng, có thẻ lịch trình thật trong nhóm tổng hợp | 7 | `layout-transition` ×3; `text-occlusion` ×4 | `01-chat-light.png`, `01-chat-light-overlay.png` |
| Mở khay lời nhờ sau khi đổi scheme sang tối | 8 | `low-contrast` ×2; `layout-transition` ×3; `text-occlusion` ×3 | `02-plan-consent-dark.png`, `02-plan-consent-dark-overlay.png` |
| Tự viết tờ hẹn, đổi lại scheme sáng | 8 | `clipped-overflow-container` ×2; `low-contrast` ×3; `layout-transition` ×3 | `03-create-outing-light.png`, `03-create-outing-light-overlay.png` |

Tổng 23 nhóm cảnh báo, **không phải 23 lỗi sản phẩm độc lập**. Tệp `*-detector.json` giữ selector DOM, hình chữ nhật, rule và nội dung chi tiết của từng nhóm.

## Lỗi đã xác nhận

### B1 · P1 · Đổi scheme giữa phiên tạo giao diện pha màu, mất chữ

Tại 390×844, đăng nhập ở sáng → đổi `prefers-color-scheme` sang tối → mở Tờ hẹn: nền khay vẫn trắng nhưng nhãn “Bạn muốn rủ hội đi đâu?” và icon đóng dùng `#f4f1ea`. Detector đo **1,1:1** trên trắng; ảnh thường xác nhận nhãn/icon gần như biến mất. Sang tạo kèo rồi đổi về sáng, tiêu đề/back dùng cùng màu chữ sáng trên `#f7f3ec`, khoảng **1,0:1**.

Lượt xác nhận khởi động tối ở 320×740 cho chat và khay tối đồng nhất, đọc được (`05-chat-dark-320.png`, `06-plan-dark-320.png`). Sau đổi về sáng và chờ 3 giây, `matchMedia` báo sáng nhưng phần đã mount còn tối; mở Cài đặt nhóm xuất hiện nhãn tối trên panel tối và input sáng (`07-plan-after-light-switch-320.png`, `08-settings-sheet-320.png`). Đây là lỗi quan sát trên Expo web khi đổi scheme giữa phiên, chưa phải kết luận về Android/iOS. Chưa xác định tận gốc cơ chế đồng bộ theme.

Điểm điều tra: `apps/mobile/src/rudi/theme.ts:25`, `SoHen.tsx:88`, `CreateOutingLive.tsx:139`, các thành phần dùng chung. Cần đồng bộ palette của thành phần đang mount và vừa mở. Đóng lỗi bằng hai chuỗi sáng → tối và tối → sáng qua chat, khay, tạo kèo, cài đặt; chữ thường đạt 4,5:1, icon hữu ích đạt 3:1; kiểm riêng native.

### B2 · P1 · Sheet cài đặt không giữ focus trong phần đang mở

Tại 320×740, mở Cài đặt nhóm qua UI, **14 lần Tab liên tiếp đều đi vào chat phía sau**, gồm tờ hẹn, tin nhắn và lựa chọn bình chọn. Không có phần tử `role="dialog"`; cây accessibility vẫn công khai composer và các bình chọn nền. Escape không đóng sheet. Đã đo trước khi inject overlay nên không quy được cho công cụ detector.

Nguồn liên quan: `apps/mobile/src/rudi/ui/Sheet.tsx:110`–137 chỉ có `accessibilityViewIsModal`, nhãn và overlay; điều đó không tạo đủ modal semantics/focus management trên web. `CaiDatNhom.tsx:111` dùng sheet này. Hệ quả: người dùng bàn phím có thể thao tác nhầm chat bị che và phải đi qua lịch sử dài để tới cài đặt.

Cần đưa focus vào sheet khi mở, giới hạn Tab/Shift+Tab, cô lập nền khỏi keyboard/AX, đóng bằng Escape và trả focus về nút mở. Đóng lỗi bằng hành trình bàn phím thật cùng cây accessibility sau sửa; không suy từ web ra VoiceOver/TalkBack.

### B3 · P2 · Hàng màu bị cắt trên màn hẹp

Ảnh `08-settings-sheet-320.png` cho thấy lựa chọn màu cuối bị cắt ở mép phải. `CaiDatNhom.tsx:214`–215 đặt năm ô rộng 52 và bốn gap 10: tổng 300 trong vùng nội dung 288 px, không wrap. Cần wrap hoặc một vùng cuộn ngang có thể khám phá và thao tác bằng bàn phím. Đóng lỗi khi cả năm lựa chọn hiện đầy đủ hoặc cuộn tới được rõ ràng ở 320 px.

## Cảnh báo không nâng thành lỗi đã xác nhận

- Bảy `text-occlusion`: liên quan các hàng trong danh sách đảo chiều/cuộn, có hình chữ nhật chạy lên vùng header trong khi phần tương ứng bị viewport danh sách che. Ảnh thường không cho thấy các câu đó đang chồng lên nội dung người dùng đọc. Không cộng chúng thành bảy lỗi bố cục.
- Chín `layout-transition`: hai node kích thước 0 và `body` trong từng trạng thái có `transition: padding`. Đây là tín hiệu CSS của shell web; chưa đo được frame drop, không gọi là bằng chứng giật native.
- Hai `clipped-overflow-container` ở màn tạo kèo: wrapper toàn màn hình của navigator/scroll. Chưa thấy menu hay CTA bị cắt do hai wrapper này; không khuyến nghị bỏ overflow của navigator chỉ để làm detector xanh.
- Một `low-contrast` của nhãn “Tạo kèo” lấy nền giấy `#f7f3ec`, trong khi ảnh thường cho thấy nhãn trắng trên nút màu accent. Đây là cảnh báo chọn sai nền của detector; tách khỏi bốn cảnh báo chữ/icon mất tương phản đã nhìn thấy ở B1.

## Những gì lượt này chứng minh được

- Cả ba trạng thái thật đều có injection, console và overlay; phần chữ đồng ý chỉ gửi lời nhờ, không chia sẻ lịch sử, hiện trên UI. Có đường “Tự viết tờ hẹn”. Lượt này không gửi lời nhờ AI hay tạo kèo mới.
- Các button đang hiện mà DOM probe thu được ở ba trạng thái 390 px đều ít nhất 48 px cao; các input đo được ít nhất 48 px cao. Không mở rộng thành cam kết cho mọi điều khiển, mọi font scale hay native.
- Chat và khay khởi động tối ở 320 px hiển thị được. Không có tràn ngang toàn document trong các ảnh đã chụp; hàng màu trong sheet vẫn bị cắt nội bộ như B3.
- Label “Chưa mã hoá đầu cuối” hiện rõ trên chat. Đây chỉ là quan sát giao diện của stack tổng hợp, không xác nhận đạt yêu cầu Chat v2 E2EE hay cho phép phát hành plaintext.

## Giới hạn harness và dọn dẹp

Lượt đầu dùng endpoint browser chung gặp reset viewport khi reconnect và mất phiên runtime khi tải lại document; ảnh Team Đà Lạt demo đã bị loại, nằm riêng ở `discarded-reload-fixture/`. Có một lỗi wrapper đọc sai dạng trả về của `impeccableDetect`; detector thực tế không crash. Port 8178 sau đó không có listener; B chuyển sang server riêng của nguyên bản export cuối. Chi tiết nằm trong `harness-limitations.json`.

Lượt ba trạng thái thành công rồi script bị lỗi chọn node Back khi thử đi tiếp tới cài đặt; `browser-results.json` giữ nguyên lỗi đó. Vì vậy không tuyên bố toàn script xanh. Sheet được kiểm độc lập bằng lượt xác nhận riêng thành công, `confirmation-results.json` có `escapeClosed: false` và 14 giá trị `insideSheet: false`.

Đã đóng hai browser do B chạy ở lượt hợp lệ, dừng server tĩnh 8189 và server detector 8471, kiểm lại không còn listener của hai cổng. Không dừng server của root, không sửa source sản phẩm, không đọc/in credential. Báo cáo này không tự chấm Nielsen hay thay thế tổng hợp thiết kế của A.

Questions skipped: đây là Assessment B cung cấp bằng chứng cho root tổng hợp; không cần thêm lựa chọn của người dùng để hoàn tất lượt kiểm tra được giao.
