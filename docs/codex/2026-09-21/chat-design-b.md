# Assessment B — detector và browser thật

Người kiểm: `/root/chat_live_stack`. Kiểm độc lập với Assessment A; không đọc
báo cáo hoặc findings A. Phạm vi: bản web Expo của
`apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx`, `http://127.0.0.1:8177`,
API thật `http://127.0.0.1:57648`, 21 thành viên nhóm tổng hợp. Không phải native
Android/iOS, không phải review mã hoá.

## Cách kiểm

Đọc skill Impeccable v4.1.2, playbook critique, chạy context một lần trong
assessment. Tạo browser/tab mới bằng Chromium 148/Puppeteer vì phiên công cụ
không có browser-native tool. Đăng nhập qua giao diện số điện thoại → OTP thật,
đi tab Tin nhắn → Phòng kiểm thử đồng thời; không gắn bearer giả vào frontend.
User tổng hợp index 20 có session riêng. Không đọc tab Assessment A.

CLI thực thi:

```sh
node .claude/skills/impeccable/scripts/detect.mjs --json \
  apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx
```

Kết quả `[]`, exit 0: **0 finding CLI trên đúng file này**. Không suy rộng sang
toàn ứng dụng hoặc coi đây là chứng minh UI tốt, accessibility đạt hay mượt.

Preflight DOM mutation thành công: đổi document title sang `[Human] Chat
assessment B`, chèn script inline, đọc lại sentinel. Live server riêng chạy
port 8400, `detect.js` được inject thành công. Browser console ghi
`[impeccable] 3 anti-patterns found`; screenshot có overlay vàng. Browser này
headless và đã đóng: không tuyên bố người dùng đang có một tab overlay mở.

## Bằng chứng thu được

- [Chat trước overlay](chat-design-b/b-chat-before-overlay.png).
- [Chat có overlay](chat-design-b/b-chat-overlay.png).
- [Khay sticker có overlay](chat-design-b/b-sticker-overlay.png).
- [JSON CLI](chat-design-b/detector-b.json).
- [DOM, kích thước controls và log browser](chat-design-b/assessment-b-browser.json).

Viewport 430×932, document scrollWidth 430 ở các trạng thái đã thu: không tràn
ngang toàn trang. Nút quay lại, gửi sticker, gửi ảnh, gửi tin đều 48×48 CSS px;
thành viên nhóm/cài đặt cao 48px. Tám ô sticker khoảng 90×122px, nằm trọn trong
viewport này, nhãn không bị cắt ở cỡ chữ browser mặc định. Các số đo này là CSS
px trên web, không phải dp/SP native hoặc kiểm font-scale Android.

Nhãn `Chưa mã hoá đầu cuối` hiện rõ dưới header. Chat giữ nền giấy ấm, bubble
màu đất, composer và khay sticker đọc được trong ảnh đã kiểm. Đã mở khay bằng
nút thật; chưa gửi sticker trong assessment này để tránh thay đổi corpus của
bài đo đồng bộ chính. DOM còn chứa nội dung screen trước đó do navigation giữ
mounted screen; `document.body.innerText` không phải chứng minh tất cả nội dung
đó đang nhìn thấy. Ảnh là căn cứ nhìn thấy.

## Finding và giới hạn detector

Browser overlay ghi ba lần **layout property animation**, trong đó banner ghi
`transition: padding`. Đây là tín hiệu animation có thể gây layout; chưa có
trace chứng minh frame drop nên không nâng thành kết luận lag. CLI không bắt
finding này trong component TSX: tín hiệu browser có thể phát sinh từ style
runtime hoặc component phụ thuộc. Chưa đủ dữ liệu để gắn nhãn false positive.

Vòng xác nhận thu selector chi tiết bị lỗi trong **harness assessment**:
`impeccableDetect()` trả object đã serialize mặc định, trong khi extractor
nhầm sang `{el, findings}` raw rồi đọc `el.tagName`. Lỗi này không phải lỗi app.
Không có JSON selector đáng tin để đính kèm, nên báo cáo chỉ giữ rule label,
count và overlay đã thực sự quan sát. Chưa hoàn tất viewport 360/slash sheet
trong B; không dùng chúng làm bằng chứng. Assessment này không đủ để chấm
accessibility, reduced motion, reader order hoặc performance/native readiness.

Live server đã stop, xác nhận port 8400 không trả lời; helper cảnh báo thiếu
`.impeccable/live/config.json` khi tìm cấu hình gỡ tag. Script được chèn vào DOM
tab tạm đã đóng, không sửa source HTML của ứng dụng. Stack API/web được giữ
chạy theo yêu cầu bài E2E chính, không coi đó là server chỉ dành cho critique.

Questions skipped: đây là evidence-only Assessment B; parent tổng hợp cùng
Assessment A và bài E2E, không phải một critique độc lập gửi người dùng.
