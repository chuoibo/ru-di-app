---
name: Rủ Đi · Sổ hẹn của hội
description: Hợp đồng chat Operate trên candidate Go legacy, cập nhật ngày 2026-09-21.
---

# Sổ hẹn của hội

Hội thoại là nơi mở lời; bình chọn giúp cùng chọn; tờ hẹn nháp được người dùng
xem lại trước khi tạo kèo; lịch trình đã tạo được mọi người sửa trên cùng một
revision của server. Giấy–mực và Nếp theo hệ nhận diện hiện hữu. Không đổi
thế giới của toàn app để trang trí riêng màn chat.

## Cấu trúc hiện tại

- Header gọn gồm tên hội, thành viên và cài đặt; nhãn legacy chưa mã hoá đầu
  cuối luôn hiện. Hội thoại không được tự chuyển sang plaintext khi v2 lỗi.
- Một tờ hẹn gấp góc ở đầu cuộc trò chuyện dẫn tới poll đang mở hoặc thẻ
  itinerary gần nhất. `outing_id` do server ghi biến trạng thái nháp thành
  “Đã thành kèo”; bấm mở đúng lịch trình đã tạo.
- Ô soạn giữ bản nháp, hàng chờ và vị trí đọc. Dấu cộng mở ảnh, sticker,
  bình chọn và tờ hẹn. Poll có biểu mẫu; slash là lối tắt vào cùng thao tác.
- Nếp chỉ hiện ở mở lời và sticker do người dùng chọn. Không dùng mascot
  cho lỗi, quyền riêng tư, xung đột hay tiền.
- Bubble và sticker mở menu bằng chạm, Enter/Space hoặc long press. Poll
  có radio có trạng thái, số phiếu từ server và xác nhận đóng cho người tạo.
- Lời nhờ AI chỉ gửi nội dung trong ô có thông báo chia sẻ rõ. Danh sách
  invocation và lỗi lấy từ server; gửi tin mới không làm mất lỗi trước đó.
  Khi provider chưa sẵn sàng, “Tự tạo kèo” dẫn sang form tạo kèo thủ công.
  Nhánh này chưa có tờ nháp chung trước khi chốt.

- Nháp form bình chọn và lời nhờ AI giữ trong bộ nhớ theo tài khoản/nhóm;
  trở lại phòng khôi phục đủ trường, có nút bỏ nháp riêng. Đăng xuất/đổi tài
  khoản xoá bộ nhớ này; khởi động lại app không khôi phục plaintext từ đĩa.
- Lỗi poll nằm cùng nút gửi ở chân khay; các trường dài cuộn phía trên.
  Poll đã đóng thu thành tóm tắt, mở các phiếu khi cần; thẻ đã thành kèo
  mở lịch trình hiện hành thay vì giữ một bản dài dễ nhầm là còn cập nhật.
- Sheet có Escape/Back, focus vào dialog, Tab trong sheet, trả focus và
  cô lập nền trên web. Palette hệ thống lấy một snapshot chung cho toàn cây.
- Route chat thật khi hết phiên phải về đăng nhập. Chỉ ID fixture tường minh
  mới mở demo; không dùng demo thay cho lỗi khôi phục phiên.

## Đồng bộ và ranh giới kiểm chứng

`useChatChanges` dùng snapshot ban đầu và change feed có sequence liên tục.
Hydration hoàn thành rồi mới ACK; watermark của hydration không thay cursor
của feed. Revision loại phản hồi cũ, tombstone làm sạch cả trích dẫn. Một
reaction vào tin chưa tải không được làm nhảy qua các trang lịch sử còn thiếu.
ACK reaction không có revision không đè snapshot mới hơn. Poll nhận cùng
luồng snapshot với tin nhắn.

Lịch sử vẫn có đường đọc legacy dự phòng. Cổng v2 MLS, voice thật, iOS và
Android dùng backend thật, smoothness trên máy và đánh giá 40/40 vẫn là các
cổng riêng. Không suy ra từ TypeScript, detector hoặc ảnh web.

Đợt chạy ứng dụng web đầu tiên trên PostgreSQL/Go thật có 20 phiên OTP riêng,
400/400 quan sát DOM, không trùng; ảnh, sticker, reply, reaction, xoá và retry
mất mạng qua thao tác thật. Bộ ảnh native dưới đây thuộc bản trước nâng cấp,
không phải bằng chứng native cho Sổ hẹn của hội. Báo cáo đầy đủ và SHA do
checkpoint tích hợp ghi lại sau review độc lập.

## Bằng chứng và hợp đồng bản trước

Phần dưới được giữ để đối chiếu lịch sử; các mô tả header ba dòng, slash
điền thẳng composer và AI gọi qua POST message đã được thay bằng cấu trúc trên.

# Hợp đồng giao diện chat legacy

## Overview

**Creative North Star: "Nhật ký chuyến đi sau giờ làm"**

Chat ở chế độ **Operate**: đọc hội thoại, soạn và xử lý tin chưa gửi được.
Giữ thế giới giấy–mực của [DESIGN.md](../../../DESIGN.md), giọng Việt ngắn
và yêu cầu đọc được của [PRODUCT.md](../../../PRODUCT.md). Không thay nhận
diện hoặc mở rộng phạm vi sản phẩm. Với backend, quyền riêng tư và AI,
[lộ trình chat Go/E2EE](../../../docs/architecture/02-chat-go-e2ee.md) cùng AGENTS.md hiện
hành ưu tiên hơn mô tả lịch sử trong PRODUCT.md.

Bản ghi dựa trên [GroupChatLive.tsx](../src/rudi/screens/chat/GroupChatLive.tsx)
và năm ảnh Android native dùng dữ liệu tổng hợp đã được reviewer cho `ship`
**chỉ trong phạm vi các trạng thái UI legacy này**:

| Ảnh trong bộ `rudi-chat-native-20260921` | Điều nhìn thấy |
| --- | --- |
| `31-rebuilt-keyboard-slash.png` | Bàn phím mở, khay slash cuộn, ô soạn và nút gửi còn hiện |
| `32-rebuilt-long-failed.png` | Tin dài xuống dòng, lỗi và hai hành động, bản nháp kế tiếp ở ô soạn |
| `32b-rebuilt-dark-failure.png` | Cùng cấu trúc lỗi ở giao diện tối |
| `33-rebuilt-retry-success.png` | Tin sau thử lại mang màu đã gửi, bản nháp kế tiếp còn nguyên |
| `34-rebuilt-dark-font13.png` | Chuỗi cùng người gửi và ô soạn ở giao diện tối, font scale 1.3 |

Ảnh không chứng minh API production, E2EE, độ ổn định native, TalkBack, iOS
hay mọi cỡ chữ. Hành vi readmark dưới đây được ghi từ source, không suy ra từ
ảnh tĩnh. Nhãn **“Chưa mã hoá đầu cuối”** mô tả đúng bề mặt legacy đang chụp;
không phải sự chấp thuận cho chat v2 gửi plaintext hoặc kho cũ tiếp tục ghi.

## Colors

Giá trị chuẩn vẫn ở [tokens.json](../../../packages/shared/tokens.json),
được [theme.ts](../src/rudi/theme.ts) đọc theo scheme và
[mau-chat.ts](../src/rudi/mau-chat.ts) chọn theo theme nhóm. Không sao chép
giá trị token vào hợp đồng này.

- `ground` là nền; `card` và `line` phân biệt bong bóng nhận, ô soạn, khay lệnh.
- `ink`, `inkSoft`, `inkFaint` tách nội dung, tên/ngữ cảnh và thời gian.
- `chatTheme.*.bubble/bubbleInk` đi thành cặp cho tin đã gửi của mình;
  `accent` đánh dấu hành động, `warn` đi cùng câu lỗi bằng chữ.
- Dòng đang gửi/thất bại dùng nền trung tính; không giả màu của tin đã gửi.

## Typography

Giữ `typography.body` cho nội dung và ô soạn, `label` cho hành động/lệnh,
`caption` cho tên, thời gian và lỗi. Đây là chữ hệ thống từ theme hiện hữu;
không đưa bậc display hoặc câu quảng bá vào hội thoại. Tin dài xuống dòng;
nhãn hành động không bị ép thành nút toàn chiều rộng trong cùng một hàng.

## Layout

Header cố định gồm tên, thành viên/cài đặt và nhãn bảo mật. Danh sách đảo
chiều đặt tin mới ở đáy; khi đang đọc lịch sử, giữ vị trí và cho nút
“Tin mới nhất”. Ô soạn ở đáy, dùng safe-area và KeyboardAvoidingView;
khoảng đáy thay đổi khi bàn phím mở. Khay slash có chiều cao giới hạn và cuộn
riêng. Source nằm ở `GroupChatLiveScreen`, `FlatList` và `styles`.

## Elevation & Depth

Bong bóng, khay lệnh và ô soạn phân lớp bằng nền cùng viền mảnh. Không thêm
bóng trang trí vào các khối này. Độ sâu và nhịp bấm của nút dùng kit hiện hữu.

## Shapes

Giữ bong bóng và ô soạn bo mềm trong `styles`, cùng `space`/`radius` của
theme khi component đã dùng chúng. Kích thước cục bộ của màn không trở thành
token chung mới. Hai nút xử lý lỗi ôm nhãn, căn phải và được phép xuống hàng.

## Components

**Luật Chuỗi Cùng Người.** `cungNguoi` chỉ nối hai tin người dùng liền kề,
cùng tác giả và cách nhau tối đa năm phút; thẻ AI và vạch ngày ngắt chuỗi.
Tên ở đầu chuỗi nhận, avatar ở cuối; thời gian ở cuối hoặc cạnh phản ứng.
Phần chừa avatar giữ thẳng cột chữ.

**Luật Bản Nháp Độc Lập.** `HangChoGui` giữ nội dung, trích dẫn và trạng thái
của từng lần gửi. Lỗi nằm ngay dưới tin; “Thử lại” chỉ hiện nếu còn thử được,
“Bỏ” loại dòng chờ. Theo [useTinNhan.ts](../src/rudi/chat/useTinNhan.ts) và
[hang-cho.ts](../src/rudi/chat/hang-cho.ts), thử lại dùng cùng attempt key;
ô soạn mới không bị thay bằng nội dung lỗi. Đây là hợp đồng client, không
thay bằng chứng chống ghi trùng ở server.

Ô soạn cho nhập nhiều dòng, có giới hạn chiều cao; nút gửi bị vô hiệu khi
không có chữ. Gõ tiền tố `/` hoặc `@` mở các lệnh phù hợp trong `LENH`;
chọn gợi ý điền vào ô soạn, chưa tự gửi. Các lệnh AI đang có là dấu vết
legacy, không xác lập quyền đọc chat cho AI ở v2.

**Luật Thấy Mới Đánh Dấu.** `baoTinHienThi` chỉ đưa ID tin mà FlatList báo
viewable vào `danhDauHienThi`. [viewability.ts](../src/rudi/chat/viewability.ts)
giữ thời gian tối thiểu 600 ms: dòng nhỏ hiện trọn được tính; dòng cao phải
phủ ít nhất 60% viewport. Hook chỉ ghi khi màn có focus và app active,
chờ server xác nhận mới tiến mốc đã ghi, giữ mục tiêu để thử lại khi lỗi và
chặn kết quả của thế hệ hội thoại cũ. Việc tải tin về không tự chứng minh đã đọc.

## Do's and Don'ts

- Giữ Nếp trong thế giới hiện hữu; không thêm mascot hệ thống vào dòng lỗi,
  thử lại hoặc thông báo bảo mật. Sticker do người gửi chọn vẫn theo quy tắc riêng.
- Giữ câu lỗi, trạng thái chờ và nhãn bảo mật bằng chữ; màu không thay nội dung.
- Không chuẩn hoá plaintext, quyền AI đọc lịch sử hoặc trạng thái gửi được của
  legacy thành thiết kế production. Không mở rộng verdict từ năm ảnh sang E2EE,
  hiệu năng, accessibility đầy đủ hay toàn bộ chat.
