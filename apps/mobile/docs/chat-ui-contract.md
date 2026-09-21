---
name: Rủ Đi · giao diện chat legacy
description: Hợp đồng bề mặt chat đã kiểm tra ngày 2026-09-21, giữ hệ giấy và mực hiện hữu.
---

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
