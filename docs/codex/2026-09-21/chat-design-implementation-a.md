# Đánh giá thiết kế chat: Reviewer A

Phương pháp: Assessment A độc lập của `/root/chat_visual_a`, thuộc lượt hai reviewer do `/root` điều phối. Chưa đọc detector, Assessment B hoặc báo cáo review khác khi chốt điểm. Đây là phần A, không phải báo cáo tổng hợp Impeccable.

**Kết luận: REQUEST_CHANGES, 28/40.** Mức Good theo rubric Impeccable; chưa đạt mục tiêu 40/40, chưa đủ để gọi trải nghiệm “Sổ hẹn của hội” hoàn chỉnh hoặc xuất sắc.

## Phạm vi và nguồn chứng cứ

- Target: `apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx`, `SoHen.tsx`, `TheAi.tsx`; kiểm tra đường đi sang tạo kèo và lịch trình.
- Đã đọc `AGENTS.md`, `.claude/skills/impeccable/SKILL.md`, `reference/critique.md` và nguồn theme. Đã chạy context helper một lần; target tên cũ `apps/mobile/src/screens/ChatScreen.tsx` không tồn tại, nên tự xác định target đúng từ cây nguồn và đọc trực tiếp, không chạy lại helper.
- Checkout ở HEAD `ef1ee46d9943b62d31d8083cbca850d6aa5f862d`, có thay đổi chưa commit của tác giả. Hash nguồn lúc kết thúc và hash ảnh ở `manifest.json` bên dưới; HEAD không đại diện toàn bộ bản đang xem.
- Tự tạo browser context và tab mới, đăng nhập qua OTP bằng tài khoản tổng hợp số 19 (index 18), không dùng tab của tác giả. Dữ liệu nhóm 20 tài khoản tổng hợp, API Go/PostgreSQL thật `127.0.0.1:45801`, web `127.0.0.1:8178`; không chặn hay giả response API.
- Ảnh chuẩn mobile từ số 05 trở đi có khung 430×932. Ảnh 01–04 là lượt đầu 800×600, không dùng làm bằng chứng mobile. Ảnh 20–23 vẫn là sáng do lựa chọn theme chưa thực thi; ảnh 24–27 mới xác nhận tối qua Cài đặt → Tối. Việc này ghi rõ trong manifest.
- Web ban đầu không có listener; tác giả khởi động lại trước khi reviewer thao tác. Reviewer không khởi động server riêng.
- Bằng chứng ngoài repo: `/tmp/rudi-chat-e2e.JpMlFu/design-a/`; ảnh `.png`, nội dung màn `.txt`, script thao tác `review.mjs`, dấu vết kiểm tra `manifest.json`. Không đưa token vào báo cáo hoặc ảnh.
- Không chứng minh Android/iOS, bàn phím native, TalkBack/VoiceOver, chuyển động trên thiết bị, tải production, crypto hay E2EE. Màn đang hiện rõ “Chưa mã hoá đầu cuối”. Điểm dưới đây chỉ đánh giá UX của bề mặt legacy/candidate đã thao tác.

## Bản sắc và cảm giác tổng thể

Bề mặt đã có tiếng nói của Rủ Đi: nền giấy ấm, màu mực rõ, coral dành cho hành động, lời “hội mình”, góc gấp trên tờ ghim và nét chì nối chặng. Khay sticker mang Nếp có hình dáng riêng; không phải chỉ thay màu một ứng dụng chat phổ thông. Tuy vậy, ở nhóm đông, phần lớn khung nhìn vẫn là các thẻ bo góc lớn, các ô lựa chọn lại nằm trong thẻ, và tờ ghim lặp tiêu đề của thẻ bên dưới. Nếp chưa nối các bước cùng chọn, sửa và chốt bằng một thay đổi thị giác có ý nghĩa; từ bình chọn sang tờ hẹn vẫn là hai công cụ tách nhau.

Thiết kế dễ đọc, đủ sạch để tiếp tục triển khai. Nó chưa tạo được cảm giác cùng viết một tờ hẹn xuyên suốt. Điểm cần ưu tiên là bảo toàn việc đang viết và làm liền mạch câu chuyện, không thêm linh vật hay lời giải thích vào mọi chỗ.

## Mười nguyên tắc, mỗi mục tối đa 4

| # | Nguyên tắc | Điểm | Căn cứ |
|---|---|---:|---|
| 1 | Hiển thị trạng thái | 3 | Phiếu của bạn, số phiếu, đã đóng và đã thành kèo hiện rõ. Lỗi bình chọn bị lẫn sang trạng thái khác. |
| 2 | Khớp ngôn ngữ đời thực | 3 | “Hội mình”, “Tờ hẹn”, “Cùng chọn” tự nhiên; “Tự viết tờ hẹn” lại vào “Kèo mới”, ngày nhập dạng ISO. |
| 3 | Quyền kiểm soát, đường thoát | 2 | Có đóng khay, bỏ trả lời, tiếp tục bình chọn; Escape không đóng menu trên web, rời phòng làm mất bản nháp form. |
| 4 | Nhất quán và chuẩn | 3 | Sáng/tối nhất quán, cùng hệ nút và màu; tờ ghim góc gấp nhưng nội dung chính quay lại thẻ bo tròn chung. |
| 5 | Phòng ngừa lỗi | 3 | Chặn poll thiếu nội dung, chốt poll phải xác nhận, AI có phạm vi chia sẻ trước nút gửi. Nháp chưa được bảo vệ khi điều hướng. |
| 6 | Nhận biết thay vì ghi nhớ | 3 | Khay có bốn icon kèm chữ, tờ ghim mở thẳng lịch trình. Cơ chế nối bình chọn với tờ hẹn chưa lộ ra cho người mới. |
| 7 | Linh hoạt và hiệu quả | 3 | Có khay công cụ và đường gõ `/`, reply và đổi màu nhóm. Nút gửi poll nằm dưới vùng cuộn nhỏ làm tăng thao tác. |
| 8 | Thẩm mỹ và tối giản | 3 | Màu, độ thoáng và chữ nhất quán; thẻ poll/tờ hẹn khá cao, chiếm phần lớn khung chat, tờ ghim và thẻ lặp ý. |
| 9 | Nhận biết và hồi phục lỗi | 2 | Lỗi tiếng Việt cụ thể và giữ các ô đã nhập; lỗi cũ không tự hết sau sửa/chuyển khay, vị trí lỗi nằm dưới vùng cuộn. |
| 10 | Trợ giúp đúng lúc | 3 | Consent AI, xác nhận chốt poll và nhắc xem lại lịch trình đúng ngữ cảnh. Thiếu chỉ dẫn ngắn về quan hệ giữa cùng chọn và tờ hẹn. |
| | **Tổng** | **28/40** | **Good; chưa đạt 40/40. Cả 10 nguyên tắc áp dụng.** |

## Những điểm đã làm tốt

1. **Xin phép AI gắn với hành động.** Lời nhờ có ô riêng; câu “Chỉ lời nhờ trong ô này…” đặt ngay trước nút gửi, có lựa chọn tự viết. Ảnh `08-mobile-ai-consent.png`, `26-ai-dark-confirmed.png`. Reviewer không gửi thêm yêu cầu AI; đã thấy thẻ do phiên khác tạo đến trong chính phòng mình.
2. **Bình chọn có hồi đáp dễ hiểu.** Reviewer tự tạo poll “Review A: chiều nay mình ghé đâu?”, chọn “Cà phê ven hồ”, thấy dấu chọn và “1 phiếu · của bạn”, mở xác nhận chốt rồi chọn tiếp tục. Ảnh `10-poll-published.png`, `11-poll-vote.png`, `12-poll-close-confirm.png`.
3. **Tờ ghim là lối tắt có ích.** Sau khi tờ hẹn đã thành kèo xuất hiện, bấm tờ ghim mở đúng lịch trình có hai chặng, không rơi vào form tạo mới. Sáng và tối giữ thứ bậc màu ổn. Ảnh `21-chat-dark.png` thực tế là sáng, `25-chat-dark-confirmed.png` là tối, `27-pinned-plan-open.png` là lịch trình thật được mở.

## Năm vấn đề ưu tiên

### A1 · P1 · Bản nháp bình chọn mất khi rời phòng

**Tái hiện:** mở + → Bình chọn; điền câu hỏi “Review A draft chưa gửi” và lựa chọn “Bờ hồ”; đóng và mở khay lại vẫn giữ nội dung. Sau đó quay về danh sách chat, mở lại cùng nhóm, mở Bình chọn: câu hỏi và lựa chọn trở về rỗng, không hỏi bỏ nháp. Kết quả hai bước được ghi trong `manifest.json`, ảnh sau quay lại `28-draft-after-route-return.png`.

**Hệ quả:** người đang soạn nhiều lựa chọn để hỏi hội phải viết lại chỉ vì đi xem nội dung nơi khác. Điều này trái với hình ảnh cuốn sổ giữ lời hẹn. `SoHen.tsx:39` giữ form bằng state cục bộ; composer có đường lưu nháp riêng nhưng form này không có.

**Sửa:** lưu nháp theo tài khoản + nhóm + loại form trong tầng nháp phù hợp; phục hồi sau điều hướng và chỉ xoá khi gửi thành công hoặc người dùng chủ động bỏ. Nếu chưa có lưu, phải có cảnh báo mất nội dung khi thoát. Cần đối chiếu cả poll và lời nhờ AI; reviewer chỉ tái hiện mất poll nên chưa kết luận thực nghiệm cho AI.

**Đóng finding:** thao tác trên giữ nguyên câu hỏi và mọi lựa chọn; gửi thành công xoá nháp đúng nhóm. Lệnh phù hợp: `/impeccable harden`.

### A2 · P2 · Lỗi poll xuất hiện sai khay, và nút gửi khó thấy

**Tái hiện:** gửi poll rỗng → đóng khay → mở +. Menu Ảnh/Sticker/Bình chọn/Tờ hẹn vẫn hiện “Thêm câu hỏi và ít nhất hai lựa chọn.” Chuyển sang Tờ hẹn vẫn giữ lỗi trong nội dung. Điền poll hợp lệ cũng vẫn nhìn thấy lỗi cũ trước khi gửi lại. Ảnh `07-mobile-tools.png`, `08-mobile-ai-consent.txt`, `09-poll-filled-scroll.png`. Form ban đầu chỉ thấy đến “Thêm lựa chọn”; phải cuộn trong khay để thấy “Gửi bình chọn” và lỗi (`05-mobile-poll-error.png`).

**Hệ quả:** lỗi ở sai hành động làm người dùng tưởng Tờ hẹn/AI yêu cầu hai lựa chọn; lỗi dưới scroll không chỉ thẳng ô cần sửa. Tại `SoHen.tsx:42`, `:98`, validation dùng chung các panel; vùng cuộn bị giới hạn 310 ở `:115`.

**Sửa:** gắn validation với panel/field phát sinh, cập nhật khi input hợp lệ, không mang lỗi sang công cụ khác. Giữ CTA và trạng thái lỗi nhìn được ở chân khay, để phần trường nhập cuộn. Lệnh phù hợp: `/impeccable harden`, `/impeccable layout`.

### A3 · P2 · Nhánh tự viết chưa có nhịp tờ nháp → cùng sửa → chốt

**Tái hiện:** + → Tờ hẹn → Tự viết tờ hẹn đưa thẳng sang `/outings/new`, tiêu đề “Kèo mới”, yêu cầu tên/ngày/ngân sách, CTA “Tạo kèo”. Không có bước chia sẻ tờ nháp vào chat ở đường này. Ảnh `23-manual-plan-dark.png` thực tế là theme sáng.

**Hệ quả:** người muốn mở lời trước phải lập kèo ngay; khái niệm “tờ hẹn” và “kèo” nhập vào nhau. Poll riêng không thể hiện nó vừa cung cấp lựa chọn cho tờ hẹn nào. Đây là khoảng cách với brief “mở lời → cùng chọn → sửa tờ hẹn → chốt chuyến đi”, không phải kết luận backend không có khả năng lưu draft.

**Sửa:** cho nhánh thủ công tạo một tờ nháp có thể gửi/sửa trước khi chốt; hoặc trong phạm vi hiện tại đổi nhãn chính xác thành “Tự tạo kèo” và ghi rõ nhánh nháp chung chưa có. Ngày nên dùng cách chọn quen thuộc, giữ dạng lưu nội bộ khỏi làm gánh nặng cho người nhập. Lệnh phù hợp: `/impeccable shape`, `/impeccable clarify`.

### A4 · P2 · Menu tin nhắn thiếu đường Escape trên web

**Tái hiện:** nhấn giữ tin do reviewer vừa gửi để mở menu, nhấn Escape; menu vẫn mở. Sau đó chọn Trả lời và Bỏ trả lời được. Ảnh `13-message-menu.png`, `14-menu-escape.png`, `15-message-reply.png`; `manifest.json` ghi Escape không đóng. `ui/Sheet.tsx` có BackHandler Android nhưng không có đường Escape cho web.

**Hệ quả:** người dùng bàn phím không có thao tác thoát quen thuộc; lớp phủ có nút đóng bằng scrim nhưng không thấy nút × trong menu. Đây là finding web, chưa suy diễn thành lỗi nút Back Android.

**Sửa:** Escape đóng sheet đang hoạt động, đưa focus về tin gốc; thêm đường đóng nhìn được nếu cần cho người dùng bàn phím. Lệnh phù hợp: `/impeccable adapt`.

### A5 · P2 · Tờ hẹn mới có bản sắc ở viền, chưa dẫn câu chuyện bằng cấu trúc

**Chứng cứ:** `11-poll-vote.png`, `25-chat-dark-confirmed.png`, `26-ai-dark-confirmed.png`. Hai lựa chọn poll cùng viền lớn, vùng chữ ký/nút chốt làm thẻ cao; tờ AI hai chặng cộng diễn giải, nhắc nháp, CTA và chữ ký chiếm gần nửa màn. Tờ ghim lặp tiêu đề. Lúc mở công cụ, lịch trình bị cắt còn phần đuôi và CTA phía trên, form mới ở dưới, composer chat vẫn hiện.

**Hệ quả:** hội đọc ít lời trò chuyện hơn, trong khi tính liên tục từ bình chọn sang tờ hẹn chưa rõ. Có vật liệu giấy và nét chì, nhưng chưa đạt mức một trải nghiệm có thể nhận ra bằng cấu trúc ngay khi che logo.

**Sửa:** bản tóm tắt hẹn gọn cho cuộc trò chuyện, mở chi tiết khi cần sửa; tờ ghim ưu tiên trạng thái hoặc bước tiếp theo thay vì lặp toàn tiêu đề. Dùng trạng thái giấy đã chọn/đang sửa/đã chốt để kể chuyện. Giữ Nếp ở các mốc nhẹ, không tăng mật độ mascot hay văn giải thích. Lệnh phù hợp: `/impeccable distill`, `/impeccable polish`.

## Tải nhận thức và hành trình cảm xúc

- Khay chính có đúng bốn công cụ, có icon và chữ; đạt progressive disclosure. Consent AI chỉ có hai đường đi, dễ hiểu.
- Menu tin có sáu phản ứng cộng ba hành động trong trường hợp tin của mình. Vượt bốn lựa chọn, nhưng sáu biểu cảm quen thuộc được nhóm riêng nên mức tải vừa. Trên Chrome kiểm thử, các emoji hiện đơn sắc và “Cháy” là ô thiếu glyph; chưa biết có xảy ra trên thiết bị đích, không dùng lỗi font môi trường để hạ verdict native.
- Điểm vui là Nếp trong khay sticker và dấu đã chọn trong poll. Điểm yên tâm là xác nhận chốt có câu hậu quả và nút tiếp tục. Thung lũng cảm xúc rõ nhất là mất nội dung đang viết và từ “Tự viết tờ hẹn” sang biểu mẫu ngày/ngân sách.
- Trạng thái kết thúc “Đã thành kèo · mở lịch trình” có giá trị vì thao tác mở đúng kèo thật. Chưa thấy điểm kết thúc thị giác biến tờ nháp thành một điều được cả hội cùng giữ; không gán bằng chứng hiểu sản phẩm của người dùng thật cho một reviewer.

## Ba góc nhìn người dùng

- **Jordan, người lần đầu:** tìm poll dễ qua icon có chữ; khó biết poll và tờ hẹn liên quan nhau thế nào. “Tự viết tờ hẹn” khiến họ bất ngờ trước form tạo kèo.
- **Casey, dùng một tay và bị ngắt quãng:** composer ở thấp hợp lý, nhưng CTA poll cần cuộn riêng; đi khỏi nhóm làm mất nháp chưa gửi.
- **Sam, dùng bàn phím/hỗ trợ tiếp cận:** các nút và trường đã có tên, reply có đường bỏ; Escape thất bại trên sheet. Chưa chạy screen reader nên không kết luận toàn bộ điều hướng trợ năng đạt hoặc hỏng.

## Ghi chú nhỏ và ranh giới chưa kiểm

- Tờ ghim đã thành kèo vẫn mang accessibilityLabel “Mở tờ hẹn để sửa”, trong khi phần nhìn thấy ghi “Đã thành kèo · mở lịch trình”; nên đổi nhãn theo trạng thái để người nghe nhận cùng thông tin.
- Dòng “Tin nhắn thoại chưa được bật.” chiếm chỗ trong menu dù menu không có nút thoại; có thể bỏ cho đến khi người dùng chủ động tìm tính năng.
- Không chấm empty state của nhóm mới: phòng đang có nhiều tin. Không tuyên bố đã độc lập tạo/gửi AI hoặc tạo kèo bằng tài khoản reviewer; chỉ kiểm consent, thẻ đến và mở kèo từ tờ ghim. Lỗi reload sang fixture do root báo được loại khỏi điểm A vì reviewer chưa tự tái hiện.
- Không sửa product source, không commit. Tab riêng giữ nguyên để có thể đối chiếu; server và browser do root sở hữu.

## Câu hỏi đưa cho người tổng hợp

1. Với nhánh thủ công, giữ phạm vi nhỏ bằng nhãn “Tự tạo kèo”, hay triển khai đầy đủ tờ nháp chung trước khi chốt?
2. Với nhóm đông, ưu tiên thẻ hẹn thu gọn theo trạng thái, hay giữ nội dung đầy đủ và thu tờ ghim thành một dòng bước tiếp theo?

Điểm A đã chốt trước khi nhận kết quả B. Chỉ chấm lại phần đã sửa khi có bản chạy mới cùng bằng chứng; không cộng điểm chỉ vì tác giả sửa source hoặc test xanh.
