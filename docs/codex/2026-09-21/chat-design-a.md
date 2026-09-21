# Chat live — Assessment A của Impeccable, 21/09/2026

**Kết luận A: giữ hướng giấy–mực–Nếp; REQUEST_CHANGES cho mục tiêu trải nghiệm chat → bot → plan hoàn chỉnh. 24/40 ở bề mặt live đã kiểm.**

Assessment A do agent `/root/chat_visual_review` thực hiện độc lập, chưa nhận
kết quả detector của Assessment B. Đây là đánh giá thiết kế trên Expo web nối
API thật; verdict `ship` của vòng native fixture trước không áp dụng cho báo
cáo này. Không sửa product source, không chạy detector.

## Phạm vi và bằng chứng

- Source chính: `apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx`; đọc thêm
  `TheAi.tsx`, `MenuTin.tsx`, `DESIGN.md`, `PRODUCT.md` và rubric
  `.claude/skills/impeccable/reference/critique.md` Assessment A.
- Trình duyệt Chrome 148 riêng, tab mới; app tại `http://127.0.0.1:8177`, API
  `http://127.0.0.1:57648`. Login bằng UI số điện thoại/OTP của tài khoản thử
  nghiệm, chuyển tab Tin nhắn và mở nhóm bằng điều khiển app.
- Mọi tên, số điện thoại và nội dung là synthetic. Không đưa credentials vào
  tài liệu hoặc worktree. Nhóm có 21 membership; một membership riêng cho
  review, 20 browser tải đồng thời là phép kiểm riêng của parent.
- Capture `03`, `04` là web 800×600; `05` trở đi là web 412×915, không phải
  ảnh native, không có IME thật. Tệp nằm ngoài worktree tại
  `/tmp/rudi-design-a-evidence/`.
- Thực hiện: mở cảnh chat rỗng và slash; gửi `/plan`; tạo `/vote`, bấm bỏ phiếu
  thật; long-press tin, mở/bỏ reply, mở sticker; thử Enter/Space trên tin có
  focus; nhận tin từ tài khoản thứ hai qua API thật; tiếp tục giữ tab để xem
  luồng gửi đồng thời do parent chạy từ 20 browser thật.
- Không xác nhận E2EE, crypto, AI sinh lịch trình thành công, Android/iOS,
  screen reader hay hiệu năng thiết bị. Không suy từ web sang native.

## Design specificity — giữ được bản sắc, chưa đạt lời hứa chat → cùng đi

Nền giấy ngà, mực đậm, đỏ gạch và Nếp gọi lời ở cảnh rỗng tạo thành một hệ
nhất quán. Bộ sticker dùng các hành động quen thuộc của hội: rủ đi, hỏi ăn,
chờ nhau, chốt hẹn. Đây là phần riêng của Rủ Đi, không chỉ thay màu một UI kit.
Trong luồng đã có tin, khung bubble/composer vẫn là hình thái messenger thông
thường; điều đó giúp dễ học và tự nó không phải lỗi. Phần có thể làm chat
thuộc riêng sản phẩm là biến lời bàn thành quyết định và lịch trình chung.
Lượt `/plan` live hiện dừng ở thông báo chưa nối được mô hình, nên chưa có
bằng chứng trải nghiệm này hoàn thành hoặc đạt “state of the art”.

Nếp hợp vai ở cửa mở lời và sticker do con người chọn gửi. Không cần đưa Nếp
vào lỗi AI, nhãn bảo mật hay sổ tiền để tăng nhận diện. Poll đã đưa quyết định
vào cuộc nói chuyện, nhưng cú pháp lệnh còn nổi rõ như công cụ dành cho người
biết trước cách sử dụng.

## Nielsen — điểm trên các trạng thái đã thao tác

| # | Heuristic | Điểm /4 | Căn cứ |
|---|---|---:|---|
| 1 | Nhìn thấy trạng thái hệ thống | 2 | Vote phản hồi số và “của bạn”; trạng thái AI thất bại biến mất sau lần gửi tiếp. |
| 2 | Khớp ngôn ngữ đời thường | 3 | Câu chữ “hội”, “Trả lời”, “Chờ tí” tự nhiên; `/plan`, `A | B` vẫn là cú pháp. |
| 3 | Kiểm soát và thoát | 3 | Sheet đóng được, reply bỏ được, có xác nhận xoá trong source; không có phục hồi trực tiếp cho lỗi AI đã gặp. |
| 4 | Nhất quán và tiêu chuẩn | 3 | Palette, sheet, chữ ký poll thống nhất; thông báo AI dùng nhãn đầu khối khác với tờ AI ký chân. |
| 5 | Phòng lỗi | 2 | Gửi rỗng bị khoá; lệnh AI vẫn được quảng bá khi server không có model. |
| 6 | Nhận biết thay vì nhớ | 2 | Slash có gợi ý; reply/reaction/copy chỉ lộ sau long-press, cách chuyển chat thành plan chưa lộ. |
| 7 | Linh hoạt và hiệu quả | 2 | Có slash và reply; Enter/Space không mở được menu tin trên web. |
| 8 | Thẩm mỹ và tối giản | 3 | Luồng đọc rõ, Nếp có chỗ đúng; raw command lặp phần poll vừa tạo. |
| 9 | Nhận biết và phục hồi lỗi | 2 | Lỗi model nói rõ chưa nối được, nhưng không có bước phục hồi và không giữ cùng yêu cầu gốc. |
| 10 | Hướng dẫn đúng lúc | 2 | Slash cung cấp mẫu; chưa giúp người mới hiểu quyền đọc của AI hoặc bước sau thẻ lịch trình. |
| | **Tổng** | **24/40** | **Acceptable theo rubric; cần sửa các khoảng đứt của luồng chính.** |

Các heuristic đều áp dụng cho bề mặt thao tác này. Điểm là đánh giá của A,
không phải kết quả đo người dùng hoặc chứng nhận accessibility.

## Luồng nhiều người sau lượt gửi đồng thời

Capture `14-live-many-senders.png` được chụp từ chính tab A sau lượt bulk của
parent. Đã nhìn trực tiếp chuỗi người gửi khác nhau, tin reconnect, lệnh plan
và sticker Nếp. Header/composer đứng yên; mỗi chuỗi có tên, avatar chân chuỗi
và timestamp đọc được; không thấy bubble đè nhau tại viewport này. Sticker
Đi thôi làm rõ lời rủ mà không phủ lên nội dung người khác.

Mật độ vẫn đọc được ở 412×915 nhưng tên + bubble + avatar/time tốn khoảng
100–145 px mỗi lượt người nói với nội dung test hai đến ba dòng. Đây chưa
phải lý do cắt tên hoặc bỏ accessibility; nên giữ khả năng gom chuỗi cùng
người và xem lại bằng copy ngắn đời thường trước khi thêm vật trang trí.
Không dùng ảnh này để xác nhận nhận đủ mọi tin, tốc độ realtime hay không
mất tin: parent đang kiểm riêng khoảng chênh DOM/traffic, A không kết luận
nguyên nhân từ ảnh.

## Ba điểm nên giữ

1. Poll cập nhật từ 0 lên 1 phiếu sau thao tác thật, có dấu chọn và dòng
   “của bạn”; không bắt người dùng suy trạng thái chỉ bằng màu.
2. Menu tin và reply có phân lớp rõ; bỏ reply đưa người dùng về composer
   ngay, không mở wizard hay thêm giải thích dài.
3. Nếp trong cảnh rỗng và tám sticker kể các động từ của một cuộc đi chung.
   Khi trò chuyện có nội dung, người dùng và nội dung tiếp quản mặt chính.

## Vấn đề ưu tiên

### A1 — P1: `/plan` hứa một tác vụ không thực hiện được trên server đang chạy

- **Đã thấy:** `05-slash-phone.png` chào “Rủ Đi AI phác lịch trình”; gửi lệnh
  thật nhận HTTP 201 nhưng `06-plan-result.png` hiện “Rủ Đi AI chưa nối được
  mô hình trên máy chủ này.” Không có plan để sửa hoặc chốt.
- **Hậu quả:** người dùng chỉ phát hiện giới hạn sau khi nghĩ nội dung và gửi;
  câu chuyện mở lời → cùng quyết → cùng đi dừng ngay sau lời gọi.
- **Sửa:** cho UI biết capability hiện tại trước khi gửi; nếu chưa sẵn sàng,
  ghi trạng thái ngay trên lựa chọn và đưa đường lập kế hoạch thủ công thật.
  Khi mở capability, kiểm trọn lượt tạo → xem → sửa → chốt bằng API/model thật.
- **Đóng khi:** lời hứa và tác vụ thực hiện khớp nhau ở cả server có/không AI.
  Đây là finding của cấu hình đang kiểm, không kết luận mọi môi trường đều
  thiếu model. Lệnh phù hợp: `/impeccable clarify`, `/impeccable harden`.

### A2 — P1 trên web: thao tác tin nhắn không có lối bàn phím hoạt động

- **Đã thấy:** long-press 800 ms mở menu ở `09-message-menu.png`. Cùng bubble
  là `DIV tabindex=0`; Enter và giữ Space 800 ms đều không mở menu, ảnh
  `12-keyboard-message.png`. Source `GroupChatLive.tsx:525` có long-press và
  native accessibility action, không có đường kích hoạt web tương đương.
- **Hậu quả:** người dùng chỉ dùng bàn phím không mở được reply/copy/delete
  qua tin đang focus, mặc dù tin có điểm dừng Tab.
- **Sửa:** cung cấp thao tác mở menu có tên và dùng được bằng Enter/Space,
  hoặc nút tuỳ chọn hiện khi focus; kiểm focus vào/ra sheet và Escape.
- **Đóng khi:** cùng tác vụ làm được từ keyboard mà không cần chuột. Không
  suy diễn lỗi này sang TalkBack/VoiceOver chưa thử. Lệnh: `/impeccable harden`.

### A3 — P2: lời giải thích AI thất bại biến mất khi gửi tin kế tiếp

- **Đã thấy:** lỗi có trong `06-plan-result.png`; sau gửi `/vote`, ảnh
  `08-vote-selected.png` còn lệnh `/plan` nhưng không còn lý do thất bại.
  `GroupChatLive.tsx:290` xoá `thongBao` khi gửi, `:301` chỉ giữ một thông báo.
- **Hậu quả:** đọc lại lịch sử chỉ thấy yêu cầu bị bỏ qua; việc gửi tiếp một
  câu không liên quan làm mất trạng thái của việc trước.
- **Sửa:** gắn kết quả/tình trạng với yêu cầu tương ứng và giữ trong phiên;
  nếu cần lưu bền phải đi qua contract phù hợp, không tự biến lỗi local thành
  một tin nhắn backend. Recovery phải bám capability hiện tại.
- **Đóng khi:** gửi tin thường hoặc tạo vote không làm mất lời giải thích cho
  lệnh đã thất bại. Lệnh: `/impeccable harden`, `/impeccable clarify`.

### A4 — P2: đường mời cả hội quyết định vẫn bắt người mới học cú pháp

- **Đã thấy:** slash hiện `/vote` cùng mẫu `câu hỏi? A | B`; khi tạo thành công,
  raw command và poll cùng hiện với nội dung lặp trong `08-vote-selected.png`.
- **Hậu quả:** người đang muốn hỏi “ăn gì” phải chuyển sang tư duy lệnh và dấu
  phân cách. Viewport của hội vừa chứa câu lệnh kỹ thuật vừa chứa kết quả.
- **Sửa:** giữ slash cho người quen, thêm lối “Bình chọn” mở các ô câu hỏi/
  lựa chọn có cấu trúc; hiển thị lời dẫn ngắn hoặc liên kết với poll thay cho
  lặp nguyên cú pháp. Không xoá nội dung gốc hay sửa lịch sử chỉ vì thẩm mỹ.
- **Đóng khi:** người mới tạo được poll từ nhãn tác vụ mà không biết `|`, và
  người đọc nhìn thấy một quyết định chính. Lệnh: `/impeccable clarify`,
  `/impeccable distill`.

## Cognitive load

Mức **vừa** trên tác vụ bot/poll: thất bại ở nhận biết bước tiếp theo và
working memory khi cần nhớ cú pháp/đường sang plan. Grouping, hierarchy, single
focus và progressive disclosure của chat cơ bản nhìn chung tốt. Không tính
mọi control trên toàn màn thành một quyết định.

Menu tin có sáu reaction, rồi ba tác vụ reply/copy/delete. Khay sticker có tám
lựa chọn, chia hai hàng bốn, mỗi ô có hình và chữ. Hai điểm vượt bốn lựa chọn
được ghi nhận, nhưng đều là lựa chọn nhận diện trực quan; chưa có bằng chứng
buộc phải cắt bộ sticker. Khay `/` chỉ hiện ba lệnh vì `@Rủ Đi` thuộc cách gọi
khác; không gán thành bốn lệnh đang nhìn thấy.

## Hành trình cảm xúc và persona

Cảnh rỗng có Nếp gọi lời tạo lối vào nhẹ → gửi một ý tưởng là bước chủ động
của con người → poll có phản hồi cụ thể là điểm có cảm giác tiến triển → lỗi
model và mất lời giải thích khi gửi tiếp tạo thung lũng cảm xúc. Điểm kết của
lượt `/plan` hiện là giới hạn máy chủ, chưa phải kế hoạch cả hội cùng sửa.

- **Jordan, người mới:** thấy mẫu slash nhưng không biết cần cú pháp nào để
  chốt một lời rủ; nhãn tạo lịch trình không báo trước capability thiếu.
- **Sam, dùng bàn phím:** focus tới được bubble nhưng không mở được tác vụ;
  đây là đường chặn cụ thể đã thử trên web, chưa phải audit screen reader.
- **Casey, dùng một tay:** composer và sheet ở đáy thuận thao tác; poll dài
  chiếm phần lớn vùng hội thoại, raw command phía trên làm phải đọc lại.

Không có mục `Design Context` trong `CLAUDE.md`, nên không dựng persona riêng
và gán cho người dùng thật.

## Quan sát nhỏ và điều chưa chứng minh

- Web composer khi focus có viền đen vuông bên trong khung bo tròn; nên dùng
  focus ring từ hệ màu, nhưng vẫn phải giữ tín hiệu focus rõ.
- Chrome headless hiện reaction cuối thành ô vuông trong menu. Đây có thể là
  font môi trường; cần xác nhận trên browser ship trước khi gán lỗi product.
- Nhãn “Chưa mã hoá đầu cuối” nói đúng trạng thái legacy; nó không giải thích
  phạm vi dữ liệu AI được đọc. `/chia-bill` nói đọc tin gần đây, nhưng phiên
  này chưa thực nghiệm consent/processing nên không kết luận về backend quyền.
- `TheAi.tsx:90–113` vẽ itinerary và nói nhóm sửa được trước khi chốt, nhưng
  không có action chuyển tiếp trong nhánh này. Đó là khoảng thiếu được thấy
  trong source; AI live chưa trả itinerary nên chưa ghi thành finding thao
  tác thẻ plan. Phải kiểm lại khi A1 có đường thành công.
- Chưa kiểm dark, zoom 200%, IME native, thiết bị vật lý hay chuyện người dùng
  thật hiểu bộ sticker. Không kế thừa bằng chứng native fixture vào đây.

## Câu hỏi cho bước tổng hợp

1. Với server chưa có AI, ưu tiên **lập plan thủ công ngay trong ngữ cảnh** hay
   **hiện capability chưa sẵn sàng trên lệnh**?
2. Với người mới, ưu tiên **tạo poll bằng form ngắn** hay **giữ slash và thêm
   ví dụ điền sẵn**?
3. Điểm kết mong muốn của chat là **một poll cả hội đã chọn** hay **một bản
   plan có đường sửa/chốt rõ**? Trả lời này quyết định điểm cần làm nổi trong
   luồng, không quyết định đổi toàn bộ nhận diện.

Đây là đầu vào Assessment A cho parent tổng hợp cùng Assessment B, không tự
mở vòng hỏi người dùng hoặc tự ghi verdict merge.
