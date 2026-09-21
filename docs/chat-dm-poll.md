# Kiểm DM hai chiều và poll nhiều người — 21/09/2026

**DM hai chiều qua UI: đạt trong lượt đã kiểm. Poll cập nhật giữa hai người:
REQUEST_CHANGES — đã tái hiện số phiếu cũ trên tab người quan sát.**

## Phạm vi và danh tính

- Chrome 148, Expo web tại `127.0.0.1:8177`, API thật tại `127.0.0.1:57648`.
  Dữ liệu và tài khoản đều synthetic; không có mock response/interception.
- Dùng browser đang giữ từ phép kiểm 20 người, không reload và không thay
  bundle. Kết quả thuộc bundle cũ trước lượt sửa cursor đang diễn ra của
  parent, không tự nhận là kết quả bản sau sửa.
- Không suy thứ tự tab thành thứ tự người dùng. Script đọc Authorization của
  request phát sinh, dùng nó gọi `GET /people/me`, rồi so person ID với danh
  sách synthetic trong tệp ngoài worktree. Token chỉ ở memory, không log và
  không ghi vào output. Token login UI mới khác token fixture ban đầu.
- User index 2 = `Chat Test 03`, tab position 9; user index 3 = `Chat Test 04`,
  tab position 8. Hai tab đầu đang dùng cho feature test được giữ nguyên.
- Backend xác nhận hai người cùng là thành viên của một pair hiện hữu và
  counterpart là nhau. Không cần tạo friendship/DM giả hoặc sửa membership.
- Screenshot DM giữ viewport 800×600 của tab đã có; poll là capture trực tiếp
  element. Đây là kiểm UI web và API thật, không phải native/crypto/E2EE.

## DM: gửi và nhận theo hai chiều

Đi qua UI `Quay lại → danh sách hội thoại → tên người kia`, không dùng
`page.goto` hoặc bơm session vào storage.

| Bước | Kết quả |
|---|---|
| Cả hai tab mở đúng pair do server xác nhận | Đạt |
| User 03 gửi câu hỏi từ composer | POST messages 201 |
| User 04 thấy câu hỏi trong DOM hiện hành | Đạt |
| User 04 trả lời từ composer | POST messages 201 |
| User 03 thấy câu trả lời | Đạt |
| Đọc lại lịch sử qua API bằng user 03 | HTTP 200, có cả hai body |
| User index 4 không thuộc pair: danh sách context | Không có pair này |
| Cùng user index 4 gọi GET messages của pair | HTTP 403 |

Đã mở và nhìn trực tiếp cả hai screenshot: đúng tên counterpart, cùng hai
nội dung, phía gửi/nhận đổi bên đúng, composer và nhãn “Chưa mã hoá đầu cuối”
đều còn hiện. Dòng “Tờ giấy của hai mình” không che hội thoại trong viewport.
Không dùng một negative case HTTP 403 để kết luận toàn bộ authorization đã
được kiểm; chưa thử mọi vai trò, thay đổi thành viên, xoá/block hay race.

## P1 — Poll giữ kết quả cũ sau khi người khác bỏ phiếu

Poll đang có thật: `QA design A: Toi nay an gi?`, tạo từ UI trong Assessment A.

1. User 03 chọn Bún bò bằng radio trong thẻ. Tab này hiện Bún bò **2**, Phở
   **0**, tổng **2**; có chữ “của bạn”.
2. User 04 chọn Phở bằng radio trong cùng poll; POST ballots trả **200**.
3. Tab user 04 hiện Bún bò **2**, Phở **1**, tổng **3**.
4. Đợi thêm **8.5 giây** sau lượt cập nhật của người bỏ phiếu. Tab user 03 vẫn
   hiện Bún bò **2**, Phở **0**, tổng **2**, nguyên như trước.
5. `GET` poll thật bằng quyền user 03 trả **200** và tổng **3**, tương ứng
   Bún bò **2**, Phở **1**. Server và quyền đọc đã biết kết quả mới; UI observer
   chưa cập nhật.

Đã nhìn trực tiếp hai capture thẻ; đây không phải suy luận từ một node ngoài
viewport hoặc từ khác biệt “của bạn” giữa hai người. Số phiếu và tổng thực sự
khác nhau. Source `apps/mobile/src/rudi/screens/chat/TheAi.tsx`, `ThePoll`,
chỉ gọi `nap()` lúc effect mount và sau lượt bỏ phiếu của chính client; không
có cơ chế refresh theo thay đổi của người khác trong nhánh đã đọc. Đây là
nguyên nhân phù hợp với bằng chứng; phép kiểm không chứng minh mọi con đường
đổi poll đều có cùng lỗi.

**Hậu quả:** hai thành viên đang xem cùng quyết định của hội lại thấy tổng và
phân bố khác nhau, mà không có nhãn kết quả cũ hay nút cập nhật.

**Đề nghị sửa:** có cơ chế đồng bộ poll đang được xem theo thay đổi server,
hoặc refresh định kỳ có giới hạn và dừng đúng lifecycle; không tăng tần suất
vô hạn cho mọi thẻ ngoài màn hình. Khi fetch lỗi, giữ kết quả đã biết nhưng
nói rõ chưa cập nhật, không biến nó thành số 0.

**Đóng finding khi:** hai browser giữ cùng poll, A bỏ phiếu/B đổi lựa chọn,
observer tự thấy tổng/phân bố đúng trong giới hạn đã công bố mà không cần
rời màn, reload hoặc tự bỏ phiếu; thêm ca ngắt mạng/khôi phục. Không đổi đáp
án test thành số cũ để làm xanh.

## Bằng chứng và cách chạy

Script: [chat-live-dm-poll.mjs](../tools/chat-live-dm-poll.mjs).

```sh
node tools/chat-live-dm-poll.mjs \
  /tmp/rudi-chat-e2e.Nllrhk/sessions.json \
  /tmp/rudi-chat-browser-run2/browser-endpoint \
  /tmp/rudi-chat-dm-poll
```

Đối số thứ tư `dm` cho phép chỉ chạy phần DM và giữ lại kết quả poll đã ghi;
được dùng trong phiên để không thay đổi lại hai lá phiếu sau khi tái hiện.
Script là diagnostic của session synthetic này, không phải benchmark tổng
quát. Nó phụ thuộc hai tài khoản đã đăng nhập, poll đã tồn tại và pair thật.

Tệp ngoài worktree:

- `/tmp/rudi-chat-dm-poll/results.json`: danh tính tab theo index, response
  status, ba snapshot tally, đối chiếu server và kết quả DM; không có token.
- `01-poll-observer-after-peer.png`: observer vẫn tổng 2.
- `02-poll-voter-after-peer.png`: voter đã tổng 3.
- `03-dm-user03-bidirectional.png`, `04-dm-user04-bidirectional.png`: DM từ hai
  góc nhìn sau khi gửi nhận hai chiều.

Lượt đầu navigation gặp selector trỏ vào nút Back của màn cũ đang hidden.
Đó là lỗi harness; script đã đổi sang tìm control có bounding box visible
và click thật. Không sửa DOM để cưỡng ép click, không gán lỗi đó cho product.

Đã chạy `node --check` và hoàn tất diagnostic trên session thật. Sau kiểm,
hai tab được đưa lại nhóm chung; browser của parent giữ nguyên hoạt động.
Không sửa product source, không commit.
