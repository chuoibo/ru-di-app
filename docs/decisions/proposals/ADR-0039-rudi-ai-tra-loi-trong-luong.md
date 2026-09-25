# ADR-0039 — Rủ Đi AI trả lời trong luồng: tin @ là tin thường, câu trả lời là một tin trả lời

- Ngày: 2026-09-25.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0039 khác thì
  văn bản này nhận số trống kế tiếp, như ADR-0036 từng đổi số.
- Quyết định sản phẩm: người dùng chốt trong phiên lập kế hoạch 2026-09-25. Nội dung: cơ chế kiểu Meta
  AI trong Messenger, chip xem trước gọn, cả phòng thấy chữ chạy ở lane cũ, chỉ người gọi thấy ở E2EE
  v2. Kế hoạch đã duyệt; ADR này chờ chữ ký Lead.
- Thiết kế chi tiết: `docs/claude/2026-09-25/thiet-ke-ai/03-rudi-ai-trong-luong.md` (commit gốc `f251db7`).
- Cùng đợt: ADR-0037 engine, ADR-0038 hàng đợi và stream, ADR-0040 RAG, ADR-0041 Nếp, ADR-0042 eval.
  Văn bản này chỉ quyết hình dạng của bot nhóm trong luồng chat.
- Sửa ADR-0036 §2.1, §2.5, §2.8 và §4. Thêm một ngoại lệ Go-only cho ADR-0021 §2.2.2. Chi tiết ở mục
  5; **không sửa bản lịch sử** của ADR nào.
- Không đổi bởi văn bản này:
  - ba luật tiền;
  - ADR-0036 §2.2, §2.4, §2.6, §2.9 (§2.3 do ADR-0040 sửa);
  - ADR-0031 §7: văn bản này hiện thực nó cho nhóm, không thay nó.

## 1. Bối cảnh

- Ở `f251db7`, gõ `@Rủ Đi`, `/plan` hay `/chia-bill` thì tin **không được gửi**: `goiMoHinh` chặn lại
  và mở khay (`GroupChatLive.tsx:85-87`, `:358-366`). Kết quả là một thẻ `ai_card` không tác giả,
  **không trả lời vào tin nào** (`chatassist/worker.go:261`). Người khác trong phòng không thấy ai nhờ
  gì, và AI không đứng trong luồng như một thành viên.
- Người dùng muốn đúng cơ chế Meta AI:
  - tin `@Rủ Đi` là tin thường;
  - AI trả lời vào chính tin đó;
  - trả lời tiếp vào tin AI là hỏi tiếp;
  - chữ chạy trực tiếp;
  - có ý định «hỏi chuyện thường», không chỉ `plan` và `chia_bill`.
- ADR-0036 §2.5 đòi khối xem trước **trên nút gửi**, và «Chỉ gửi lời nhờ» luôn có mặt. Khối «Mình đang
  thấy» hiện nay (`SoHen.tsx:224`, `:248`) sống trong khay. Khay biến mất thì khối phải thành một chip
  gọn, mà vẫn giữ đúng lời hứa.
- ADR-0021 §2.2.2 chỉ cho trả lời `text|image|sticker`. Luật đó là bản port có oracle Python
  (`domain/messageedit/messageedit.go:10`, `:42-53`). Hỏi tiếp bằng cách trả lời vào tin AI cần một
  ngoại lệ, và ngoại lệ không được làm lệch oracle.
- Lane cũ chưa E2EE; phòng v2 chỉ có ở chat-lab. Máy chủ hôm nay từ chối phòng v2 bằng 409
  (`handler.go:163-176`).

## 2. Quyết định

1. **Tin @ là tin thường; lời gọi vẫn tường minh.** Client gửi tin qua hàng gửi sẵn có, rồi gọi
   `POST /contexts/{c}/ai-invocations` với `trigger_message_id`. Máy chủ **không bao giờ** suy ra lời gọi
   từ chữ; `actOnMessageIntent` giữ hành vi (`routes/messages_wai.go:536-548`).
   - Tin tag phải của chính người gọi, `kind='text'`, chưa xoá, ≤24 giờ, cùng phòng.
   - Máy chủ đọc `id, author_id, kind, created_at, deleted_at`, không bao giờ đọc `body`.
2. **Ngữ cảnh** là lời nhờ trong tin tag, cộng N tin gần (mặc định 20), cộng tối đa 6 lượt của chuỗi
   hỏi tiếp, trong trần 40 lượt hiện có. Mỗi id vẫn qua `thuocPhong`.
3. **Chip xem trước gọn thay khối «Mình đang thấy», giữ nghĩa ADR-0036 §2.5.** Chip nằm ngay trên nút
   gửi, dạng «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ».
   - `n` đọc từ chính gói sắp gửi.
   - «Xem» liệt kê đúng gói đó.
   - «Chỉ gửi lời nhờ» là một chạm và không gửi lượt nào, kể cả chuỗi hỏi tiếp.
4. **Câu trả lời là tin `ai_card` kind `tra_loi`, `reply_to_id` = tin tag.** Dạng thẻ theo hợp đồng
   chung: `{ban, tac_gia:"rudi-ai", invocation_id, lenh, doc{so_tin, chi_loi_nho}, phan[≤3:
   text|places|itinerary|expense_draft], outing_id?}`.
   - Tác giả DB là NULL; danh tính nằm trong thẻ, vì chỉ cách đó sống qua E2EE.
   - `doc.so_tin` là số máy chủ đã kiểm, không phải lời client khai.
   - Kiểm bằng `GroundReply` mới, chỉ ở Go. `GroundCard` giữ nguyên cho oracle.
   - `POST /messages` không tạo được thẻ này (422 `card_ungrounded`).
5. **Trả lời vào tin AI là hỏi tiếp, chỉ khi chip đang bật và hiện.** Chip tự bật khi người dùng trả
   lời vào `tra_loi` và có nút «Không hỏi Rủ Đi». Không chip thì là tin thường, không lời gọi nào.
   **Ngoại lệ Go-only với ADR-0021 §2.2.2:** `laTraLoiAi` chạy trước `CheckReplyTarget` và chỉ nhận
   `ai_card` tác giả NULL có `card.kind='tra_loi'`. `CheckReplyTarget` và golden Python không sửa. Hàng
   `tra_loi` chỉ do worker Go sinh ra, nên parity không chạm được nhánh này.
6. **Lane cũ: cả phòng thấy chữ chạy.**
   - Người gọi nhận SSE `…/ai-invocations/{id}/events`. Người xem khác nhận frame `ai` trên WS
     `chatlegacychange` sẵn có, bật bằng opt-in.
   - Sự kiện là enum đóng của `aistream` (ADR-0038).
   - Cửa sổ output guard 48 rune là nơi **duy nhất** sinh `delta`. Không byte nào tới phòng trước khi
     guard quét nó, và **không có «rút lại»**.
   - Frame phòng không mang mã guard.
7. **E2EE v2 (chat-lab): chỉ người gọi thấy chữ chạy.** Phòng chỉ nhận metadata «Rủ Đi AI đang trả
   lời…». Máy người gọi dựng thẻ, mã hoá ở epoch hiện hành rồi đăng; người khác đối chiếu
   `result_digest`.
   - `lane` do **máy chủ tự suy** từ `chat_v2_conversations`; client không khai.
   - Writer stream không bao giờ ghi khoá phòng cho job lane v2.
8. **Một tin tag, một câu trả lời.** Có unique index trên `trigger_message_id` cho `queued|running|
   succeeded`. Tin tag bị xoá thì một trigger Go huỷ job và xoá chữ. Hạn phòng: 3 lời gọi đang chạy,
   30 mỗi giờ. Hạn 8/phút/người giữ nguyên. Số lời gọi model mỗi lượt theo `MaxModelCallsPerTurn`
   (ADR-0037).
9. **AI nhóm không chạm tiền.**
   - `chia_bill` giữ nháp ở cột `result`. Phần `expense_draft` trên thẻ chỉ là con trỏ: không số tiền,
     không id người.
   - «Ghi khoản này» mở luồng xác nhận chi tiêu sẵn có, đã điền trước.
   - Dấu «Đã ghi vào sổ» **suy từ dòng chi tiêu thật**: máy chủ kiểm phòng, người ghi, thời điểm.
     Không có nút «đánh dấu xong».
10. **Nhóm không đọc trí nhớ Nếp.** `recall_memory`, `remember_fact`, `forget_fact`, `set_reminder`
    chỉ tới được từ gốc `scope=me`. Toolset nhóm do `quyen.golden.json` quy định.
11. **Câu chữ trên màn đi cùng commit với hành vi** (mẫu ADR-0036 §5). Không bản build nào được đổi hành
    vi trước khi câu hứa đổi theo. Cùng commit phải có:
    - câu của chip, sheet «Xem», hàng «đang đọc», câu thất bại cho người xem;
    - mọi mã từ chối mới trong `LOI_GOI_AI`;
    - sửa `apps/mobile/docs/chat-ui-contract.md:26-29`, `:150-152` và `.impeccable/surfaces/chat.md:43`;
    - mục DESIGN.md «Trả lời của Rủ Đi AI trong luồng».

## 3. Hệ quả

- `chat_ai_invocations` thêm `trigger_message_id`, `lane` (giữ `DEFAULT 'legacy'` để deploy cuốn chiếu
  không hỏng) và `so_tin_doc`, trong một migration mới của chuỗi `chatassist`. Số version cấp theo
  thứ tự lên main. Cột v2 đi migration riêng ở lát 20.
- Có thêm một trigger Go trên bảng `messages` do Python sở hữu, cùng tiền lệ
  `chatlegacychange/schema.sql:52`. Nó chỉ ghi `chat_ai_invocations`.
- Parity không so được nhánh `tra_loi`; đó là ngoại lệ có tên ở mục 2.5. Parity thêm hai ca giữ phần
  còn lại: tin `@Rủ Đi` là chữ thường, không intent; trả lời vào thẻ bình chọn bị từ chối.
- Người xem thấy chữ chậm hơn model khoảng 48 rune. Đó là giá của việc không byte nào tới cả phòng
  trước khi guard thấy nó.
- Chữ chạy là tạm. Tin đã đăng là nguồn sự thật và tới qua change feed. Mất Redis thì mất chữ chạy,
  không mất tin (ADR-0031 §2).
- Mục tiêu latency theo hợp đồng chung: trạng thái đầu p95 ≤300 ms; token chữ đầu p50 ≤2.5 s, p95
  ≤5 s; `plan` p95 ≤8 s.
- Mọi route mới vào `ownership/routes.json` loại `GO-ONLY` trong cùng commit với handler.
- Đồ thị đọc của `chatassist` thêm ba câu không đọc `body`: tin tag, chuỗi trả lời, `card` của lượt AI.
  Cổng đọc xuyên gói (lát 5) phải thấy cả ba.

## 4. Cái này KHÔNG cho phép

- Không cho máy chủ suy ra lời gọi AI từ chữ tin nhắn, hay đọc `body` để lấy ngữ cảnh.
- Không cho trả lời vào tin AI thành lời gọi khi chip không hiện.
- Không cho bỏ «Chỉ gửi lời nhờ», và không cho đếm số tin trên chip từ màn thay vì từ gói.
- Không cho client khai `lane`. Không cho ghi khoá phòng cho job v2, hay đẩy chữ rõ vào phòng v2.
- Không cho nhả byte nào trước output guard, và không hứa «rút lại».
- Không cho tạo `tra_loi` qua `POST /messages`. Không cho sửa `CheckReplyTarget` hay golden Python của nó.
- Không cho đưa số tiền, id người trả hay `shared_by` vào thẻ. Không cho dấu «đã ghi» do client tự báo.
  Không cho AI ghi sổ.
- Không cho bot nhóm gọi tool trí nhớ Nếp.
- Không cho vẽ mặt Nếp lên thẻ nhóm (ADR-0036 §2.6 giữ nguyên).
- Không cho đổi hành vi trước câu chữ trên màn.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

| Điều khoản | Hiện nói | Sau ADR này |
|---|---|---|
| ADR-0036 §2.1 | AI chỉ chạy khi được gọi tường minh qua hàng đợi | Giữ. Tin `@Rủ Đi` là tin thường **đi kèm** một lời gọi tường minh do client gửi; trả lời vào tin AI chỉ là lời gọi khi chip đang bật và hiện |
| ADR-0036 §2.5 | Khối «Mình đang thấy» trên nút gửi, mở ra thành danh sách | Được là chip gọn, nếu chip nêu số tin đọc từ gói, «Xem» liệt kê đúng gói, và «Chỉ gửi lời nhờ» là một chạm |
| ADR-0036 §2.8, vế nhóm | Một thẻ trong phòng, có provenance số tin đã đọc | Một tin `tra_loi` trả lời vào tin tag, có `doc{so_tin, chi_loi_nho}`; stream theo mục 2.6 và 2.7 |
| ADR-0036 §4 «Không cho AI tự nói khi không ai gọi» | — | Giữ; nói rõ trả lời vào tin AI không có chip thì không phải lời gọi |
| ADR-0036 §4 «Không cho đưa nội dung tin nhắn vào endpoint mà không có khối xem trước ở trên nút gửi» | Khối xem trước | Chip ở mục 2.3 là khối xem trước hợp lệ |
| ADR-0021 §2.2.2 | Chỉ trả lời được `text|image|sticker` | Giữ cho mọi đường có oracle; ngoại lệ Go-only duy nhất: tin `ai_card` tác giả NULL, `card.kind='tra_loi'` |
| `chat-ui-contract.md:26-29` | «Lời nhờ AI chỉ gửi nội dung trong ô…» | Lời nhờ trong tin `@Rủ Đi` và đúng gói hiện trên chip; sửa cùng commit |
| `.impeccable/surfaces/chat.md:43` | «AI chỉ nhận đúng lời nhờ trong ô» | «…lời nhờ trong tin @Rủ Đi và đúng gói hiện trên chip trên nút gửi»; sửa cùng commit |

Không đổi bởi văn bản này: ADR-0031 §7 (kết quả niêm tới người gọi, client đăng ở epoch hiện hành:
mục 2.7 là hiện thực của nó); ADR-0036 §2.6 (không mặt Nếp), §2.9 (AI không chạm tiền), §3.

## 6. Cổng nghiệm thu

- **Lát 7:**
  - `scripts/go_postgres_tier.sh` (skip là đỏ): tin tag sai chủ, sai phòng, đã xoá → 422; hai lời gọi
    đua trên một tin tag → một 202, một 409; trả lời vào `tra_loi` → 201, vào bình chọn → 422.
  - Test node của chip và gói; Maestro `49-rudi-ai-trong-luong.yaml`; **mở từng ảnh chụp ra nhìn**.
  - Canary: bỏ chip thì flow 49 đỏ.
- **Lát 9:** eval nhóm lõi ≥14/16, ổn định qua 5 lần, với số lời gọi thật đã được Lead duyệt.
- **Lát 12:** job lane v2 để lại 0 byte ở khoá phòng; câu vi phạm guard để lại 0 frame `ai`.
- **Lát 14:** `ghi` với chi tiêu phòng khác hoặc do người khác ghi → từ chối.
- **Mỗi lát:** chạy lại trong cây sạch đúng SHA; ít nhất hai đột biến tự nghĩ, kiểm tương đương trước,
  đỏ đúng bước dự đoán; số đo ghi thẳng vào commit message.

## 7. Sau review phản biện lát 7 (2026-09-25): điều kiện vào main, giá đã biết, khoảng hở

Review phản biện lát 7 (verdict `REQUEST_CHANGES`, 10 phát hiện) được xử lý ở commit sửa theo review
trên nhánh `claude/peaceful-hopper-32kwjs`. Ba điều dưới đây không sửa được bằng mã ở lát này, nên
ghi thẳng vào văn bản sẽ được ký.

### 7.1 Điều kiện vào main

- **`main` không được nhận lát 7** (`5af8655`, `603515f` và commit sửa theo review) **trước khi Lead ký
  ADR-0039.** Lát này đổi hành vi của hai điều khoản đã ký bằng một văn bản còn ở `proposals/`:
  - ngoại lệ Go-only `laTraLoiAi` (`routes/messages_wai.go`) sửa **ADR-0021 §2.2.2**, vốn chỉ cho trả
    lời vào `text|image|sticker`;
  - chip trên nút gửi và tin `@Rủ Đi` là tin thường thay **khay của ADR-0036** (§2.1, §2.5, §2.8, §4,
    xem bảng mục 5).
- Cùng điều kiện: Maestro `49-rudi-ai-trong-luong.yaml` (mục 6) phải được viết — file này **chưa có**
  trong cây — và chạy trên máy thật, và ảnh chụp (sáng, tối, Reduce Motion) phải được mở ra nhìn. Máy
  làm lát này không có emulator; flow 30 (hai nhánh AI) và máy kiểm sau flow 40 đã sửa theo luồng mới
  nhưng cũng **chưa chạy trên máy**.

### 7.2 Thứ tự khoá của publish

- Publish lấy **head của feed phòng trước, rồi KEY SHARE tin tag, rồi job**, đúng thứ tự mọi lệnh ghi
  chat Go lấy qua `chatlegacychange.BeforeWrite` (head trước, rồi `FOR UPDATE` trên tin). Thứ tự trước
  (tin tag trước head) khoá chéo với xoá tin và thả cảm xúc: đo được `40P01` ở cả hai đường.
- Ghi chú khoá chéo **bên trong** `schema_luong.sql` (migration `chatassist` v4) vẫn nói thứ tự cũ.
  File đó bị ghim checksum; sửa một chữ trong comment là mọi database đã cài v4 báo «checksum mismatch».
  Ghi chú đúng nằm ở `giuTrigger` (`chatassist/luong.go`) và cạnh chỗ nhúng file (`migrate.go`).

### 7.3 Giá cuốn chiếu cho người dùng app cũ (chấp nhận, không chặn được theo phiên bản)

- Capability `ai.mention` chỉ bảo vệ **app của người gọi**: app cũ không thấy nó thì không gửi
  `trigger_message_id` và nhận đúng thẻ cũ. Nhưng câu trả lời `tra_loi` là một tin trong phòng, và
  **thành viên khác** còn dùng app trước lát 7 thấy nó là «Một thẻ bản này chưa hiển thị được.»
  (`TheAi.tsx:181` của bản cũ), còn danh sách cuộc trò chuyện ghi «[Rủ Đi AI]». Họ không đọc được chữ
  nào của câu trả lời cho tới khi cập nhật app.
- Không chặn được theo phiên bản tối thiểu: `chat-capabilities` trả lời theo request của người gọi,
  không mang phiên bản app nào, và máy chủ không giữ sổ phiên bản app của từng thành viên. Gate
  `mention` theo phiên bản của người gọi cũng không cứu người đọc. Văn bản này chấp nhận đó là giá của
  đợt cuốn chiếu; muốn bỏ giá này thì cần một cơ chế khai phiên bản theo thành viên, là việc riêng.

### 7.4 Khoảng hở đã biết: tin @ có thể không được trả lời mà không có dấu hiệu nào

- Nếu người gọi rời màn chat trong lúc tin `@Rủ Đi` còn đang gửi, lời gọi AI bị bỏ: `hoiAiVeTin` không
  chạy, cặp chờ trong `capChoTin` ở lại bộ nhớ rồi mất. Tin nằm trong luồng, không có câu trả lời, và
  không có hàng «Rủ Đi AI chưa nhận lời nhờ» nào nói điều đó. App bị tắt giữa cặp cũng vậy.
- Lối thoát theo thiết kế (§4.1 bước 5 của thiết kế 03) là mục «Nhờ Rủ Đi AI trả lời tin này» trong
  `MenuTin`, **mở sheet chip trước** rồi mới gọi. Mục đó **chưa làm**: nó cần một sheet xem trước mới
  (ADR-0036 §2.5: không gói nào rời máy khi chưa hiện trước), và UI mới phải qua cổng ảnh chụp mà máy
  làm lát này không chạy được. Cho tới khi mục đó có, khoảng hở này là một mục mở của lát 7.
