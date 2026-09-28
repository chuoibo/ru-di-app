# Surface brief · Chat nhóm ("Sổ hẹn của hội")

<!-- impeccable:surface-brief 1 -->

**Route:** `apps/mobile/app/groups/[id]/chat.tsx` → `src/rudi/screens/chat/GroupChatLive.tsx`
**Mode:** Operate. Người dùng vào đây để **làm xong một việc** — chốt buổi đi chơi —
không phải để được thuyết phục hay để ngắm. Quen tay thắng biểu cảm.
**Thế giới:** thừa kế `.impeccable/decision/ui-v2-direction.json`
("Nhật ký chuyến đi sau giờ làm"). Đây là **extension**, không mở vòng bản sắc mới:
không màu mới, không chữ mới, không bo góc mới.

## Việc thật trên màn này

Một hội 4–10 người đang cãi nhau chọn chỗ trong chat. Màn này phải đưa họ từ
**«mười ý kiến»** tới **«một buổi đã chốt»** mà không ai phải chép tay sang chỗ khác.

Chuỗi đó có bốn chặng, và cả bốn phải ở trong cùng một luồng:

1. **Cùng chọn** — một bình chọn trong chat.
2. **Cùng sửa** — lựa chọn đã chốt mở ra **một tờ hẹn chung** cả hội viết được.
3. **Chốt** — chính tờ đó thành kèo, không phải một bản sao.
4. **Chia tiền** — việc của màn khác, không kéo vào đây.

Chặng 2 là chặng mới. Trước nó, bình chọn chốt xong là hết đường: nhánh thủ công
tạo kèo thẳng từ form riêng của một người.

## Câu người dùng phải trả lời được trong ba giây

- «Hội đang chốt cái gì?» → tờ ghim gập góc trên đầu luồng.
- «Tờ nào đang nhận lựa chọn vừa rồi?» → một bình chọn chỉ nuôi **một** tờ đang mở.
- «Tôi sửa được không, hay chỉ đọc?» → «Cả hội sửa được tờ này · bản N» + «Sửa cùng hội».
- «Có ai vừa sửa không?» → số bản đổi, và người ghi sau bị báo bản mình cũ.

## Ranh giới

- **Nếp chỉ xuất hiện ở trạng thái rỗng mở đầu và ở sticker người dùng tự chọn.**
  Không bao giờ ở lỗi, quyền riêng tư, xung đột hay tiền.
- **Nhãn «Chưa mã hoá đầu cuối» luôn hiện.** Chat không được im lặng tụt về
  plaintext khi v2 hỏng.
- **Thẻ AI và tờ hẹn chung phải phân biệt được bằng mắt.** Thẻ AI ký bằng tia
  lấp lánh tông `ai`; tờ hẹn chung ký bằng biểu tượng người, tông `accent`. Một
  tờ của hội mượn dấu của mô hình là nói sai ai viết nó.
- **AI chỉ nhận đúng lời nhờ trong tin @Rủ Đi và đúng gói hiện trên chip trên
  nút gửi**; chip nói số tin đọc từ chính gói, «Xem» liệt kê đúng gói, «Chỉ
  gửi lời nhờ» là một chạm. Câu trả lời trả lời vào tin tag, ký ở chân bằng tia
  lấp lánh tông `ai`, không mặt Nếp.
- Ngày người đọc thấy là **ngày/tháng/năm**; ISO chỉ sống trên dây.

## Cổng phải xanh trước khi đổi hệ

- `rudi-khong-hex` — nợ đang **bằng 0**. Màu mới phải đi qua
  `packages/shared/tokens.json` → `theme.ts`, rồi qua cả hai cổng tương phản.
- `rudi-mau-chat` — ink trên bong bóng ≥ 4.5:1 ở cả hai chế độ.
- `test_contrast_floor.py` — biên nào nhận diện một **control** thì nợ 3:1.
- `mac-dinh-am-tham-id` — không giá trị hiển thị nào được lấy id thô làm mặc định.

## Những gì màn này KHÔNG chứng minh

Ảnh web không thay được native. Font scale, TalkBack, bàn phím thật và độ mượt
60 Hz là **cổng riêng**. Bố cục khay (C3 trong báo cáo reviewer) cố ý chưa đụng
tới: reviewer nói rõ phải kiểm cùng bàn phím native trước khi đổi.
