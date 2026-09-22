# ADR-0034 — Một đường gọi AI, và người gọi là người trao ngữ cảnh

- Ngày: 2026-09-22.
- Quyết định sản phẩm: leader chốt trong phiên lập kế hoạch và yêu cầu triển khai.
- Trạng thái triển khai: đang thực hiện; chưa có bằng chứng production, chưa có cổng chất lượng đã đóng.
- Thay ADR-0031 §7 (lời gọi và trích đoạn) và bổ sung ADR-0033 §4 (Nếp đọc gì).
  Không sửa bản lịch sử của hai ADR đó.

## 1. Bối cảnh

Hôm nay cây có **ba** bề mặt AI, và chúng không cùng một hình dạng:

| Bề mặt | Đường | Ngữ cảnh nhận được |
|---|---|---|
| Companion v1 | `POST /contexts/{id}/ai-turn`, cộng tự kích hoạt trong `POST /messages` | máy chủ tự đọc 40 tin gần nhất, roster, gu nhóm, ngân sách |
| `chatassist` v2 | `POST /contexts/{id}/ai-invocations` | đúng câu người gõ; `members` rỗng; catalogue 40 hàng lấy theo `ORDER BY id` |
| Nếp (ADR-0033) | dock trên mọi màn | phiếu ngữ cảnh khoá đóng; **chưa có não chữ** |

v1 trả lời tốt vì nó đọc hội thoại. v2 trả lời kém vì nó bị bịt mắt. Nhưng v1 không sống
qua cutover chat v2: khi máy chủ không giữ khoá, nó không còn gì để đọc. Hai đường cùng tồn
tại là hai bộ luật riêng tư, hai bộ giới hạn, hai chỗ phải sửa.

ADR-0031 §7 đã chọn hướng "mặc định lời gọi, trích đoạn thêm cần grant từng tác giả". Vế
sau chưa bao giờ được xây, nên thực tế là v2 chỉ có vế đầu, và nó dở.

Cùng lúc, `chat-ui-contract.md` đang hứa với người dùng: *"Lịch sử chat không được chia sẻ."*

## 2. Quyết định

1. **Một đường duy nhất.** AI chỉ chạy khi được gọi tường minh, qua hàng đợi lời gọi. Mọi
   đường tự kích hoạt bị xoá, không để lại cờ: route `ai-turn` ở cả Go lẫn Python, nhánh
   companion và nhánh `chia_bill` trong `POST /messages`, và logic nhịp (cooldown, hạn lượt
   trong cửa sổ tin) vốn chỉ có nghĩa khi AI tự nói.

2. **Người gọi trao ngữ cảnh; máy chủ không đọc hội thoại.** Client gom đoạn chat nó đang
   hiển thị và gửi kèm lời gọi. Đây là chỗ thay ADR-0031 §7: bỏ cơ chế "grant từng tác giả"
   chưa từng được xây, thay bằng một cơ chế đơn giản hơn và **lộ ít hơn v1**, vì v1 để máy
   chủ chọn gửi gì, kể cả tin người gọi chưa từng mở.

3. **Máy chủ chỉ đắp thêm thứ nó vốn sở hữu và chưa bao giờ mã hoá**: roster, gu nhóm, ngân
   sách, lịch sử chuyến, và catalogue địa điểm đã qua `promptsafety`. Không bao giờ đọc bảng
   tin nhắn để lấy ngữ cảnh. Một cổng quét nguồn giữ điều này, vì nó là thứ làm cả đường sống
   sót qua cutover.

4. **Máy chủ không kiểm client gửi đúng nguyên văn.** Client đã giữ sẵn các byte đó và có thể
   dán thẳng vào ô lời nhờ; một máy kiểm ở đây không chặn được gì mà vẫn phải bảo trì. Máy chủ
   kiểm đúng ba thứ nó kiểm được và có nghĩa: id tin thuộc đúng phòng này, tác giả khai đúng,
   và gói nằm trong giới hạn. Ba thứ đó mua được: chặn rửa tin của phòng khác vào đây, chặn
   gán sai người nói, chặn đầu vào vô hạn. Không mua được: không chặn một thành viên bịa nội
   dung mà chính họ vốn có thể tự gõ. Viết ra để không ai xây nhầm.

5. **Người gọi phải thấy trước thứ sắp gửi.** Khối "Mình đang thấy" nằm ngay trên nút gửi,
   đếm số tin sẽ đi kèm, và mở ra được thành danh sách **dựng từ chính gói sắp gửi**, không
   dựng lại từ màn. Và **"Chỉ gửi lời nhờ" luôn có mặt**: hợp đồng hiện tại đã hứa không chia
   sẻ lịch sử, nên phải còn một đường về đúng lời hứa đó. Đơn phương rút lại một lời hứa
   riêng tư đã nói ra là việc không được làm, kể cả khi khối xem trước đã rất rõ.

6. **Nếp và AI trong nhóm dùng chung một engine, nhưng là hai vai, hai tên, hai khuôn mặt.**
   Trong nhóm nó là "Rủ Đi AI"; hình Nếp không được vẽ lên thẻ trả lời của nhóm.
   `ban-tinh.ts` giữ nguyên: sổ loại `hoi` cho Nếp không việc gì, Nếp vẫn im trong nhóm.

7. **Bổ sung ADR-0033 §4.** Nếp được nhận **các lượt hỏi đáp của chính phiên đang mở**, gửi
   lại từ máy mỗi lượt. Đây không phải nới whitelist phiếu: whitelist quy định thứ một **màn**
   được khai thay mặt người dùng; nó chưa bao giờ quy định thứ **chính người dùng** gõ ra. Hai
   xuất xứ khác nhau, hai trường khác nhau. Nếp vẫn **không** đọc hội thoại nhóm, không đọc gu,
   không đọc lịch sử. Phiên giữ trên máy, không xuống đĩa, không lên máy chủ.

8. **Kết quả có hai hình dạng.** Nhóm: một thẻ trong phòng, mang provenance nói rõ nó đã đọc
   bao nhiêu tin. Cá nhân: trả kín về đúng người gọi, không phòng nào được ghi. Hình dạng thứ
   hai chính là `sealed result` mà ADR-0031 §7 đòi cho nhóm **sau** cutover, nên xây nó bây
   giờ là xây trước thứ chat v2 sẽ cần.

9. **AI không chạm tiền.** `chia_bill` ra một thẻ chữ cộng số liệu có cấu trúc để người xác
   nhận, không bao giờ tự ghi sổ. Ba luật tiền không đổi. Nếp vẫn phải câm ở màn tiền, lỗi và
   xung đột theo ADR-0033 §2.2, và lệnh câm đó áp cho **cả** trả lời bằng chữ lẫn vẽ ảnh.

10. **Ngoại lệ có tên cho Python.** Được phép xoá route legacy `take_companion_turn` và nhánh
    tự kích hoạt trong `service.py`, dù CLAUDE.md giới hạn sửa Python legacy. Lý do: đây là
    orchestration, không phải bước gọi model, và để lại thì cửa trước vẫn proxy tới nó. Phần
    Python **duy nhất** được thêm là một action brain cho Nếp: dựng prompt và gọi model.

## 3. Hệ quả

- `share_scope` trong `chat-capabilities` đang là **nhãn, không phải cổng**: không dòng nào
  đọc nó. Nó trở thành cổng thật. Máy chủ còn khai `invocation_only` thì client **không đính
  gói nào**. Điều này cho một đường rollout sạch: máy chủ cũ nhận thân cũ, test cũ vẫn xanh,
  không cần cờ riêng.
- **Idempotency phải tính cả gói bối cảnh, ở CẢ hai phía.** Hôm nay client khoá lần thử chỉ
  theo lời nhờ, và máy chủ băm `command + prompt`. Cùng câu hỏi với đoạn chat mới sẽ trùng
  digest, và máy chủ trả lại **kết quả cũ** kèm mã 200. Người dùng tưởng AI vừa đọc tin mới.
  Đây là hỏng **im lặng**, nên nó là hệ quả phải ghi ra chứ không phải chi tiết thi công.
- Hợp đồng `POST /contexts/{id}/messages` mất `companion` và `expense_card`; `intent` và
  `intent_error` **thu hẹp** chứ không biến mất, vì nhánh `/vote` ở lại (xem 3b).
- Câu "Các lệnh AI đang có là dấu vết legacy, không xác lập quyền đọc chat cho AI ở v2"
  trong `chat-ui-contract.md` **giữ nguyên, không sửa**. Nó vẫn đúng và đang là cách phát
  biểu gọn nhất về lõi của quyết định này: AI không được trao **quyền đọc**; client được trao
  một **hành động gửi**.
- Ba route của engine hiện không có trong manifest quyền sở hữu và chỉ sống sau một cờ môi
  trường. Chúng phải vào manifest và ra khỏi cờ **trước** khi xoá v1: xoá trước thì production
  mất sạch AI, và không cổng nào bắt được vì không ca nào chạy với cờ tắt rồi hỏi lại.
- Engine có hai worker. Thêm Nếp làm người gọi thứ ba, mà Nếp nổi trên mọi màn, nhân nhu cầu
  lên trên một nguồn cung không đổi. Cách chữa rẻ là nâng số worker, không phải tách hàng đợi.

## 3b. Ba điều đo được, ghi ra vì chúng đổi hình dạng việc

- **`/vote` KHÔNG phải đường AI và phải sống.** Vỏ RuDi đang ship tạo bình chọn bằng cách
  gửi chuỗi `/vote ...` qua `POST /messages` (`SoHen.tsx:129` → `GroupChatLive.tsx:873`).
  Nhánh đó không gọi model và không tiêu hạn mức nào. Xoá nguyên hàm xử lý lệnh trong tin
  nhắn là làm hỏng bình chọn trên máy người dùng, và **không cổng nào bắt**: không kịch bản
  parity nào gửi lệnh gạch chéo. Chỉ ba nhánh AI bị xoá; nhánh vote ở lại và nên được thêm
  vào kịch bản parity trước khi đụng vào.

- **Xoá Python phải đi cùng commit với xoá Go và sửa manifest.** Cửa trước Go proxy mọi
  request không khớp route Go sang Python, nên bỏ bản Go trước là làm đường cũ sống lại
  nguyên vẹn dưới quyền Python, với mọi cổng vẫn xanh. Bỏ bản Python trước thì cổng quyền
  sở hữu đỏ vì Go còn handler mà manifest hết hàng. Không có thứ tự nào khác xanh. Điều này
  cũng là lý do CLAUDE.md được bổ sung một ngoại lệ có tên.

- **Đừng xoá một cổng chất lượng dưới danh nghĩa dọn dẹp.** Trần lượt gọi đổi từ 30 mỗi
  phút xuống 8, và mã từ chối đổi tên; nhưng mã mới chưa có câu tiếng Việt nào. Cổng giữ bất
  biến "mọi mã từ chối của máy chủ đều có câu người đọc" lại nằm đúng trong tập file sẽ xoá.
  Bất biến phải được chuyển sang chỗ mới **trước**, không phải sau.

## 4. Cái này KHÔNG cho phép

- Không cho máy chủ đọc bảng tin nhắn để lấy ngữ cảnh cho AI, ở bất kỳ đường nào.
- Không cho AI tự nói khi không ai gọi.
- Không cho Nếp đọc hội thoại nhóm, gu, hay lịch sử. Mục 7 chỉ mở đúng các lượt hỏi đáp của
  chính phiên đang mở.
- Không cho giữ phiên Nếp ở máy chủ hay xuống đĩa thiết bị.
- Không cho AI ghi vào sổ tiền, tạo nghĩa vụ, hay chốt kèo. Người xác nhận mọi hành động thật.
- Không cho vẽ hình Nếp lên thẻ trả lời trong nhóm.
- Không cho bỏ lựa chọn "Chỉ gửi lời nhờ".
- Không cho đưa nội dung tin nhắn vào endpoint mà không có khối xem trước ở trên nút gửi.
