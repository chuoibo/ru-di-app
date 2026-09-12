# Phản biện spec «Nếp truyền giấy» — 12/09/2026

Method: dual-agent (A: /root/design_review_a · B: /root/evidence_review_b), cộng lượt đọc toàn bộ và tái tính màu độc lập của Codex root. A hoàn tất trước khi root đọc findings B.

**VERDICT: REQUEST_CHANGES đối với việc chuyển spec này sang kế hoạch triển khai. Giữ hướng Nếp truyền giấy; chưa duyệt chất lượng UI native hoặc độ stunning.** Đây là review thiết kế của Claude, không phải tự duyệt phần hiện thực của Codex. Phần cải thiện bên dưới là đề xuất để tác giả phản biện; chưa được Lead hoặc tác giả chấp nhận.

## 1. Phạm vi và nguồn

- Đã đọc **đủ 1667 dòng, mục 0–24** tại `9c0d6f98a32361d220b3f5cb0e5c8cc8d94242a8`, nhánh `claude/p0-w-ui8-spec-mode-hai-nguoi`. Mọi số dòng trong báo cáo chỉ về revision này, không về bản sửa sau.
- [Spec gốc, permalink](https://github.com/chuoibo/ru-di-app/blob/9c0d6f98a32361d220b3f5cb0e5c8cc8d94242a8/docs/superpowers/specs/2026-09-12-mode-hai-nguoi-nep-truyen-giay-design.md). SHA256 nội dung: `9d2eb0a08cda1f07a3dc805d32b2de82b2f40c2f8b32682ff13aa10e7858821c`.
- Đối chiếu cùng revision: `packages/shared/tokens.json`, `apps/mobile/src/rudi/art/nep.ts`, `adaptive.ts`, `motion.ts`, `ui/useMotion.ts`, `screens/chat/GroupChatLive.tsx`, `screens/chat/CaiDatNhom.tsx`, `screens/Create.tsx`, layout các tab, `services/api/app/db/models.py`, route check-in và ADR-0020.
- Đã xem concept Nếp `linhvat/nep-concept-01-selected.png` và hai ảnh fixture native cũ `docs/codex/2026-09-11/native-evidence/{20-explore-light2,21-chat-light2}.png`. Đây là tham chiếu nhận diện/độ dày nội dung ở chữ lớn; **không phải hình mode hai người**, cũng không phải ảnh mới tại commit spec. Báo cáo 11/09 gắn ảnh với `5061b610`, Android 15 và fixture tổng hợp.
- Không có ảnh hoặc video feature mới trong spec: chỉ có bảng, ASCII và mô tả. Không chạy native pilot, không chạy nghiên cứu người thật, không mở gate ADR-0006. `protocol_version` của snapshot v1 không đổi.
- Checkout người dùng vẫn ở `claude/p0-w-hanh-trinh @ 714f469b`. Review được chuẩn bị trong worktree riêng; không đổi branch của checkout đó hoặc sửa nguồn sản phẩm.
- Lead cho phép tạo PR trong phiên này: [draft PR #607](https://github.com/chuoibo/ru-di-app/pull/607). **Tại thời điểm viết, chưa có phản hồi của tác giả.** Các luận điểm bảo vệ thiết kế ở mục 9 là cách reviewer diễn giải lập luận mạnh nhất của spec, không phải lời tác giả vừa trả lời.

## 2. Nhận định chính

**Feature có lý do để tồn tại trong Rủ Đi và có thể trở thành một nhánh trải nghiệm lớn. Nhưng spec hiện kể chuyện hấp dẫn hơn những màn hình mà nó thực sự định nghĩa.**

Ý mạnh nhất là **một vật có đời sống**: mảnh giấy được trao, được mở, được giữ, rồi quay lại ở một thời điểm khác. Nếp có một công việc hợp với cơ thể giấy của nó. Đây là chất liệu tốt để xây bản sắc lâu dài; không cần một hệ màu hẹn hò riêng.

Điểm yếu nhất là biến trải nghiệm ấy thành **chat cộng ba thẻ tác vụ**, rồi đặt mảnh giấy và ký ức ở tận đợt 3. Người dùng có thể dùng hàng tháng mà chủ yếu thấy một lịch trình và câu «Còn đúng không?». Tên feature hứa một vật được truyền tay; lát đầu lại giao một sổ ghi chép về người kia.

Tôi đề nghị lấy câu này làm trục thử nghiệm mới:

> **Một lời rủ. Một điều để nhớ.**
>
> Ở hội bạn, Nếp chừa một chỗ. Ở hai người, Nếp giữ một điều.

Đây là copy đề xuất, chưa phải thay thế đã duyệt. Nó giữ cả sự chủ động của hai người lẫn vai trò hỗ trợ của Nếp. Câu «Không ai rủ ai nữa» ở dòng 143 đang xoá chính hành động mà tên Rủ Đi và phần thưởng «được rủ một lần» ở dòng 297 muốn nuôi.

### Ba điểm nên giữ

1. **Sai rẻ, nghỉ được:** một cú đổi, không phạt, không streak, một người đủ quyền rời. Những điều này phù hợp một sản phẩm giúp giảm công sức.
2. **Nếp biểu đạt bằng việc làm:** mang/giữ/trao giấy; không suy diễn tình cảm, không chấm quan hệ. Anatomy hiện có và lớp mascot tháo được là nền tốt.
3. **Cùng hệ giấy–mực, khác công việc:** tái sử dụng context và sản phẩm hiện có, không ép người dùng sống trong hai app. Một tờ giấy từ lời rủ thành ký ức là khả năng đặc trưng nhất.

## 3. Năm ưu tiên cải thiện

### U1 · P1 — Chốt ai mở lời, ai đồng ý, và Nếp nhận phần việc nào

**Bằng chứng:** dòng 263–281 cho cùng một kèo xuất hiện trên hai máy, không ai là tác giả. Dòng 295–301 cho một người thấy riêng trước, sửa rồi gửi, người nhận thấy lời rủ từ người yêu. Dòng 181–183 có vai cố định, gậy tuần luân phiên, cộng thêm chủ của ba chặng ở 341–347.

**Hậu quả:** designer không biết vẽ trạng thái nào trước; người dùng không biết «Ừ» đang đồng ý một gợi ý của máy hay nhận lời người kia. Người vốn lo nhiều còn mặc định nhận vai «Người lo», trong một feature định giải việc chia lại sự chủ động.

**Đề nghị:** một lượt chỉ có hai việc hiện ra bằng câu hành động: **«Tuần này bạn mở lời»** và **«Bạn thêm một điều»**. Nếp chuẩn bị bản nháp; người gửi xem và chủ động gửi; người nhận đồng ý hoặc đề nghị sửa. Khi Nếp tự đề nghị, tờ giấy đứng rõ dưới tên Nếp. Không cần phô công nghệ, nhưng không được giả một người đã gửi khi họ chưa làm.

Không bắt người mở lời phải sửa đúng một thứ để được gửi. Một nháp đã vừa ý có thể gửi nguyên; công sức thật nằm ở lựa chọn gửi. Tránh biến «sửa một thứ» thành thao tác diễn để nhận công trạng.

**Điều kiện khép finding:** có một timeline theo giờ, hai cột người dùng, xác định riêng tư/chung và người gửi ở từng bước; cùng một bộ state dùng cho §4, §9, §20 và §22. Mỗi phản ứng gắn với **phiên bản kèo**. Sửa kèo không mang theo chấp thuận cũ. Chỉ sinh một outing sau điều kiện chốt được định nghĩa; im lặng không tự là đồng ý.

### U2 · P2 — Lát đầu cần trả đủ lời hứa của feature

**Bằng chứng:** giả thuyết quan trọng nhất là app quyết thay ở dòng 28; routine được gọi là sản phẩm ở 117. Nhưng 1a là chín bề mặt về sổ riêng/cửa vào (1497–1505), còn mảnh giấy, trao tặng và giấy quay lại tới đợt 3 (823–829).

**Hậu quả dự đoán:** người dùng mới gặp thủ tục ghép đôi và bảy mục chưa điền trước khi nhận giá trị cảm xúc. Lát 1a có thể chứng minh nhu cầu ghi nhớ, nhưng không chứng minh cơ chế quyết định hoặc sức cuốn hút của truyền giấy.

**Đề nghị:** lát pilot khép một vòng nhỏ **mở lời → nhận lời → cùng đi → giữ một điều**. Lời rủ đã là một mảnh giấy; sau buổi đi, mỗi người có thể thêm một dòng vào vật đó. Chưa cần xây toàn hệ thư hẹn năm sau, bảy mục sổ, mô hình học gu hay bản đồ mới.

Nếu vẫn chọn sổ riêng trước, ghi rõ đó là **thử nghiệm giá trị cá nhân và cửa mời**, với tiêu chí riêng. Đừng gọi kết quả ấy là kiểm chứng giả thuyết kèo tự tới. Phạm vi roadmap cần Lead chốt, đây là đề xuất ưu tiên chứ không phải đổi scope đã được duyệt.

### U3 · P2 — Một tờ đang mở, để cuộc chat còn chỗ thở

**Bằng chứng:** §20.1 xếp kèo, ôn, giấy trước chat; §22 chỉ cộng chiều ngang. Ba thẻ chứa ít nhất sáu hành động chính/secondary, chưa tính soạn tin và điều hướng. Ở chữ lớn, riêng ba nút kèo đã cần ít nhất 144dp chiều cao trước khoảng cách. Source chat dùng `FlatList inverted`, có composer bám bàn phím.

**Hậu quả dự đoán:** mở cuộc chat để nói với người kia nhưng gặp một bảng việc. «Ở đầu chat» cũng chưa xác định là vùng ghim ngoài list hay phần cuộn; hai cách cho hành vi hoàn toàn khác. Chưa có bản native để gọi đây là lỗi đã tái hiện.

**Đề nghị hình dạng:** ngay dưới tên người/sổ là một hàng **«Tờ giấy của hai mình»** dẫn vào không gian giấy của đúng context. Chat giữ nhịp trò chuyện. Trong không gian đó, **một tờ mở theo việc đang diễn ra**, các vật khác là hàng đóng có nhãn ngắn. Sổ riêng có cửa vào rõ **«Chỉ mình»**, không chen câu ôn riêng giữa nội dung chung.

Một phương án ít đổi điều hướng hơn vẫn hợp lệ: một khe trong chat, thu gọn mặc định, chạm mới mở tờ giấy. Tác giả cần so sánh hai phương án bằng frame có đủ nội dung thật của giao diện, không chỉ tờ thẻ rời.

**Ưu tiên theo tình huống:** việc cần quyết trước buổi đi; giấy do người kia vừa gửi hoặc đã tới lúc mở; kỷ niệm; ôn riêng chỉ khi người dùng mở sổ riêng. Không dùng thứ tự cứng `kèo > ôn > mảnh giấy` để luôn đặt món quà dưới câu kiểm tra dữ liệu.

**Điều kiện kiểm:** frame 360×800 là kích thước proposal để so bố cục, kèm frame theo viewport native thực sau insets; sáng/tối, font 1.0/2.0, bàn phím mở, tin mới đến, đọc tin cũ. Composer và đường quay về luôn rõ; mở thẻ không nhảy vị trí chat. Màn rộng có thể dùng hai vùng nếu content box cho phép; không kéo giãn một lá thư ngang toàn tablet.

### U4 · P2 — Đừng bắt cuộc sống phục vụ hình học của giấy

**Bằng chứng:** đôi lâu năm thường «ăn gì đó rồi về» (109), nhưng mặc định chia đúng ba chặng (335–351) để trùng ba phần giấy. «Một lần lạ» bị gọi là bắt buộc/còn nợ (325–331) trong khi §8 cấm tạo tội lỗi.

**Đề nghị:** default **một chỗ chính, phần đi tiếp tùy chọn**. Buổi dài có thể ba chặng khi thời gian, ngân sách, di chuyển và mong muốn hai người cho phép. Hai nếp gấp là cấu trúc lá thư, không phải ràng buộc số địa điểm.

Núm độ mới vẫn hữu ích, nhưng ràng buộc thứ tự nên là: điều không thể làm → khung giờ/ngân sách/khu vực người dùng khai → một việc cả hai chấp nhận → mức mới. Đừng để «trả lượt đã thiếu» đẩy một trải nghiệm mà người kia không muốn.

Copy nên chuyển từ «còn nợ một lần lạ» sang **«Tháng này thử một điều mới?»**. Có lựa chọn nghỉ/hẹn tuần khác, không sinh khoản nợ cảm xúc. Những mệnh đề về nam giới hoặc mọi đôi lâu năm ở §0/§1 cần đổi thành giả thuyết về **người thường lo/người ít mở lời**, tránh lấy một kiểu quan hệ làm chuẩn cho tất cả.

### U5 · P1 — Kỹ thuật đo được phải hỗ trợ mỹ thuật, không thay thế nó

**Bằng chứng tính lại trên token cùng revision:**

| Cặp màu được spec chọn | Kết quả | Spec yêu cầu |
|---|---:|---:|
| Tối: `inkFaint` trên `paper` | **4.372764:1** | ≥4.5 |
| Tối: `paper` trên nền `#1c1f36` do spec ghi | **1.343130:1** | ≥1.9 |
| Tối: `ink` trên `paper` | 10.673323:1 | ≥7 |
| Tối: `inkSoft` trên `paper`, phương án sửa | **6.859643:1** | ≥4.5 |

Đây là phép tính màu đặc sRGB, **không phải đo ảnh native mới**. Nền `#1c1f36` là literal spec sử dụng. Con số 1.06 ở dòng 1555 đang lấy nhầm điều kiện: báo cáo PR #603 dùng nó cho `card`, còn thẻ mới chọn `paper`.

**Sửa cụ thể:** dùng `inkSoft` cho lý do trên giấy tối; giữ token toàn app. Với tách lớp, xác định mẫu vật liệu rồi chọn ngưỡng có căn cứ. Nếu vẫn bắt buộc 1.9 thì phải đổi thiết kế surface trong phạm vi mode và ghi ngoại lệ; không thể chỉ thêm một viền rồi coi tỷ lệ fill đã tăng. 1.9 là ngưỡng mỹ thuật do spec tự đặt, không phải ngưỡng WCAG cho mọi thẻ.

Sàn chữ thường 4.5:1 và việc không làm tròn kết quả thiếu thành đủ được đối chiếu với [W3C SC 1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html). Thực thi còn cần kiểm các cặp foreground/background thật sự dùng; snapshot màu không chứng minh khả năng đọc cả màn.

«Đúng một coral trên toàn màn» (1593–1605) không khả thi khi vẫn giữ chat có bubble coral, sticker và ảnh do người dùng gửi. Nó còn tranh dấu với góc Nếp và dấu giấy chưa mở. Đổi thành **một điểm nhấn hành động dẫn trong vùng thiết kế mới**; scope rõ phần art tự vẽ, không đếm nội dung người dùng. Trạng thái mở giấy luôn có chữ ngắn như **«Mở tối nay, 20:00»**, không chỉ dựa vào tam giác. [W3C SC 1.4.1](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html) yêu cầu thông tin không chỉ truyền qua màu.

Vết gấp sáng chỉ là hairline trên thẻ bo 20 rất dễ đọc như separator của bảng. Cần nhìn chỗ mép giấy gặp nếp, một mặt gấp có hướng và bóng tiếp xúc có chủ đích. Không thêm hàng loạt texture để cứu nghĩa hình. Thử bản vật liệu ở cỡ thật và đọc mù trước khi chốt công thức.

«Hơi cũ mềm» không có một số đơn giản không có nghĩa là không kiểm được. Có thể đối chiếu mẫu tham chiếu về độ mềm mép, độ tương phản và độ tiết chế. Stddev dải 3px chỉ đo sự biến thiên pixel; văn bản, noise hoặc lệch căn cũng làm nó tăng. Muốn dùng làm guard, phải chỉ rõ ROI, mật độ pixel, mask chữ, nền so sánh và ý nghĩa phép đo. Không dùng số đó để ký «đã stunning».

## 4. Câu chuyện Nếp cần một bản canon ngắn, đúng hình đã có

§1.2 dòng 50 mô tả Nếp không mặt/không mắt. §19 và source cùng revision có mắt, một mày, miệng, tay và chân. Giữ anatomy hiện có; viết lại lời mở đầu. Mô tả đề nghị:

> Nếp là một mảnh giấy biết giữ chỗ và giữ lời. Ở hội bạn, nó kéo ghế, giữ bản đồ, chừa một khoảng cho người tới sau. Trong sổ hai người, nó thu mình bên mép giấy, mang điều một người muốn gửi và lui ra khi người kia mở. Nếp không biết hai người yêu nhau đến đâu. Nó biết điều nào đã được gửi, điều nào còn riêng, và việc nào hai người đã chọn làm cùng nhau.

### Ba quy tắc diễn xuất

1. **Mỗi lần xuất hiện có một việc và có kết thúc.** Đưa giấy xong rút tay; giữ mép giấy khi đang soạn; lật trang rồi nhường sân cho nội dung. Không đứng cười cạnh mọi thông báo.
2. **Tình cảm nằm ở nội dung của hai người.** Nếp không nói «người ấy chắc sẽ cảm động», không giả viết câu thương thay họ. Copy cần phân biệt lời hệ thống, nháp của Nếp, lời do người dùng gửi.
3. **Vật còn ý nghĩa khi bỏ mascot.** Giấy đã gửi, chưa mở, riêng tư, bị huỷ đều đọc được qua tên/ngày/trạng thái. Ở chữ lớn hoặc khi có lỗi/xung đột, bỏ nhân vật vẫn giữ trọn thông tin.

### Tạo hình cần nâng ở đâu

Giữ tỉ lệ và nét Nếp hiện có, thử biến thể `manh` bằng silhouette ngắn/vuông hơn và động tác truyền tay. Không đặt toàn bộ khả năng nhận ra biến thể lên hai đường nếp và một biểu cảm rất nhỏ: bản gọn `chiTiet:false` hiện bỏ chi tiết mày/nếp. 48dp cần một bản hình riêng đọc được, không phải bản 96 thu nhỏ.

Làm trước **ba tư thế** đủ kể vòng: mang tới, giữ chờ, mở/nhường. Bảng cần đặt cạnh Nếp nhóm ở 96 và 48, sáng/tối, rồi xem không nhãn. Người soát phải nhận ra cùng một nhân vật nhưng công việc khác. Đây là đề nghị kiểm hình, chưa có concept mới hoặc native specimen để kết luận đã đạt.

Chất cảm xúc trưởng thành nên đến từ hai vật có khoảng cách rồi chạm nhau: hai mép giấy, hai nét mực do hai người chọn, một khoảng trắng được điền. Không cần ép màu hồng, trái tim hoặc làm mắt Nếp lớn hơn. Các biểu tượng hiện có vẫn dùng cho thao tác; đồ vật có ý nghĩa dùng cho cảnh.

## 5. Storyboard đề nghị để feature có mở đầu, cao trào và thứ ở lại

Đây là storyboard bằng nội dung/bố cục để tác giả vẽ thành frame, **chưa phải mockup hình hay native proof**. Các ví dụ đều tổng hợp; không dùng thông tin của cặp thật.

| Khoảnh khắc | Người dùng nhìn và làm gì | Hình/motion mang câu chuyện |
|---|---|---|
| **Mở một chỗ riêng** | «Một chỗ cho những lời rủ của hai mình». Thấy rõ ai được mời, phần chung/phần riêng. Nút «Gửi lời mời» | Một tờ giấy đủ lớn để đọc, một nửa đang gấp. Nếp ở mép; câu consent là chữ thường đọc được, không nằm trong hoạt cảnh |
| **Người kia nhận** | «Cùng mở sổ hai người?»; «Đồng ý» và «Để sau» bình đẳng về khả năng tìm thấy. Không ép công khai trạng thái quan hệ | Nửa gấp thứ hai hoàn tất **sau** máy chủ ghi đồng ý. Người gửi xem sự kiện khi quay lại cũng hiểu đủ; không cần hai người online cùng lúc |
| **Có một lời rủ** | Tiêu đề «Tối thứ Bảy, mình đi một chút». Một chỗ chính, giờ, khung chi phí nếu biết; một dòng để người gửi thêm. Thấy nguồn Nếp nếu là nháp | Một ký họa có liên hệ với hoạt động và đường mực dẫn từ lời rủ tới chi tiết. Khi có ảnh địa điểm, dùng nguồn được phép; không lấy ảnh bất kỳ để giả quán |
| **Hai người góp một điều** | Một người gửi lời rủ; người kia chọn «Mình đi» hoặc «Đổi một chút». Nhận xét/sửa không bị diễn thành từ chối tình cảm | Hai dấu đóng góp nhỏ cùng hiện trên một tờ. Đây là dấu chấp thuận kèo, không phải read receipt của câu hỏi riêng |
| **Đi xong, giữ một điều** | «Có điều gì muốn giữ từ tối nay?»; một dòng hoặc bỏ qua. Không cần ảnh, không cần chấm điểm quan hệ | Cùng tờ giấy bỏ nút ra quyết định và nhận một dòng ngày tháng. Một nét/chi tiết đã có được giữ lại; tờ giấy đã sống qua buổi đi |
| **Gặp lại** | Khi người dùng mở ký ức hoặc tới mốc được chọn, xem điều thực sự từng gửi. Có «Cất đi/Không nhắc lại» | Vật cũ trở lại cùng hình dáng; dấu thời gian và câu của hai người làm phần cảm xúc. Không bịa wear theo thời gian hoặc suy diễn tâm trạng |

### Bố cục một tờ lời rủ

Ở điện thoại, dùng một vùng chính rộng theo content box. Bricolage làm một tiêu đề ngắn; body hệ thống giữ dấu Việt và font scale. Phần trên có một cảnh nhỏ gắn với việc sẽ làm, phần giữa là một lời rủ và thông tin thiết yếu, phần dưới là thao tác. Các hàng địa điểm không cần mỗi hàng một card lồng thêm.

```text
←  Sổ hai người                         Sổ riêng ›

Bản nháp · chỉ bạn thấy

Tối thứ Bảy,
mình đi một chút.

       [một ký họa của buổi đi, có khoảng thở]

18:30  Một chỗ ăn tối
       [thông tin có nguồn, chưa biết thì nói chưa biết]

Đi tiếp nếu còn hứng                    Thêm ›

Một điều muốn nhắn                      Viết ›

             Gửi lời rủ
```

Đây là frame phía **soạn/gửi**. Frame phía **nhận** dùng «Mình đi / Đổi một chút», không lẫn nút sửa nháp với nút chấp thuận. Tác giả cần vẽ riêng hai phía và trạng thái chưa được gửi.

Để đẹp ở dùng hàng ngày, nên dành hoạt cảnh đáng nhớ cho lần ghép sổ và lần nhận giấy; mở lại cùng vật không chiếu lễ khai mạc lần nữa. Fold chỉ chạy sau trạng thái thật, không chặn nút, không tranh gesture Back hoặc cuộn. Reduce Motion theo contract hiện có là trạng thái tĩnh/đổi ngay; fade có duration khác 0 cần được ghi thành ngoại lệ nếu chọn, vì nó chưa phải hành vi mặc định của `motion.ts`.

## 6. Những mâu thuẫn contract phải khép trước kế hoạch triển khai

Các mục C1–C6 là vấn đề spec, consent hoặc khả năng kiểm chứng; không phải yêu cầu designer theo sở thích cá nhân của reviewer. Những lựa chọn như tagline, thêm bề mặt riêng hoặc pilot nhỏ là đề xuất ở mục 3–5.

| ID | Bằng chứng tại spec gốc | Hậu quả và tiêu chí gỡ |
|---|---|---|
| **C1 · P1 — Kèo chưa có một máy trạng thái chung** | 263–301; 674–684; 1360–1361. Hai người cùng thấy và một người thấy riêng trước chưa có timeline; chỉ nói «bốn trạng thái» ở 1507 mà chưa định nghĩa đủ | Phải có actor/phiên bản/điều kiện chốt. Đổi, nghỉ, một người chưa trả lời, hai phản hồi đồng thời, retry, offline, hết hạn và sau khi đóng sổ phải cho một kết quả xác định. Không tự chốt từ im lặng hoặc gắn chấp thuận bản cũ sang bản mới |
| **C2 · P1 — Quyền đọc riêng tư tự mâu thuẫn** | 721–725 đòi gu cá nhân chủ động chia; 1132–1135 cho cold start dùng gu toàn cục. 747–749 đòi người kia biết và sổ đối xứng; 1209–1215 cho dùng sổ ngay lúc chưa đồng ý | Phải quyết rõ solo chỉ là ghi tay riêng hay được Nếp đọc chat/đề nghị trang. Gợi ý chung không đọc nguồn riêng chưa consent. Bảng nguồn → ai thấy → được dùng cho gợi ý nào → khi nào thu hồi; bao gồm ảnh/media, cache, push và switch context. Tóm tắt lại dữ liệu riêng vẫn có thể lộ nó |
| **C3 · P1 — Đóng rồi mở lại có hồi sinh giấy không?** | 236, 753, 867 cấm giấy chưa tới lúc mở hồi phục; 1095–1097 cho dữ liệu đôi ngủ rồi tỉnh lại | Dùng một bảng vòng đời duy nhất, phân biệt giấy đã mở, chưa tới lúc, đã tới lúc nhưng chưa đọc, sổ riêng, túi riêng. Chu kỳ nối lại phải có identity riêng để không hồi sinh quyền đọc/lịch gửi cũ. Tác giả phải chọn nghĩa «chưa mở» và nêu ai còn đọc được sau đóng. Đường đọc ảnh trực tiếp phải tuân cùng luật |
| **C4 · P1 — Bộ số và dấu trạng thái không thể cùng đúng** | 1555–1568, 1587–1605, 1614–1645; §19 đòi coral trên Nếp trong khi §22 chỉ cho coral trên thẻ | Sửa cặp màu, scope accent, cách biểu thị giấy chưa tới lúc, và reduced-motion contract. Chạy lại phép tính kèm specimen trong bối cảnh chat; không thay số ngưỡng chỉ để có xanh. Phần nghĩa hình phải có đọc mù |
| **C5 · P1 — Nguồn ghi nhận bị nói thành sự thật ngoài đời** | 272 «chưa từng đi», 579 «không bao giờ sai», 559 nói thiếu catalogue không chặn; 397 dùng một check-in để mở «lần tới hai người đi cùng» | Đổi thành «chưa có trong sổ này», có nguồn/mốc và sửa được. Catalogue thiếu món/giờ/giá không thể bảo đảm lọc ràng buộc chỉ từ một ghi chú. Phân biệt biết phù hợp/biết không phù hợp/chưa biết. Check-in hiện chỉ ghi một actor đã bấm, không chứng minh hai người cạnh nhau; chốt điều kiện mở bằng thao tác rõ, không hứa cảm biến |
| **C6 · P1 — Roadmap/đo lường đang kiểm sai lời hứa** | 792–793 nói đợt 1 không cần sổ nhưng 1218–1224 đưa sổ vào đầu; 851 kill «twin» khi Ừ thấp, dù 28 nói cần kiểm muốn tự chọn hay không; 849/853 diễn nghỉ tăng thành Nếp ồn | Hợp nhất roadmap, phụ thuộc và metric theo từng lát. Xác định mẫu số, đã thấy đề nghị hay chưa, cả hai đồng ý chưa, thực sự đi chưa, và lý do nghỉ tùy chọn. Thử tám đề nghị không tự chứng minh mô hình hiểu gu; chưa có kế hoạch/mẫu đủ để kết luận thống kê. Không dùng tỉ lệ điền bảy mục để tối ưu thu thêm dữ liệu riêng |

### Bảng trạng thái gợi ý cho tác giả hoàn thiện

| Tình huống | Hành vi đề xuất | Điều không được nhập nhằng |
|---|---|---|
| Lời mời sổ còn chờ | Cho rút lời mời; phần ghi riêng nếu được chọn phải độc lập khỏi consent chung | Không đọc sổ, gu hoặc chat của người kia bằng quyền chưa được cấp |
| Nháp kèo riêng | Chỉ chủ lượt thấy, chọn gửi hoặc bỏ | Đã phác không bằng đã gửi |
| Đề nghị v1 gửi chung | Mỗi người phản ứng trên v1 | Đã nhận dữ liệu không bằng đã đọc hay đồng ý |
| Một người đề nghị sửa | Tạo/đề nghị v2, hiển thị thay đổi | Consent v1 không chốt v2 |
| Cả hai đồng ý cùng bản | Chốt đúng một outing, retry không nhân đôi | UI/animation theo kết quả server |
| Nghỉ tuần này | Tắt đề nghị theo tuần; việc nhắn/gửi giấy do người dùng chủ động vẫn có thể tiếp tục theo setting rõ | «Nếp im» không đồng nghĩa giấu món quà người kia tự gửi |
| Đóng sổ | Một người đủ quyền thực hiện, nhãn rõ, xem hậu quả trước xác nhận | Đừng dùng «mở giấy» làm tên cho hành động phá hiệu lực thư |
| Hai người nối lại | Consent mới, chu kỳ mới | Không khôi phục giấy/lịch cũ trái luật đã chốt |

Bảng này là đề nghị contract, chưa phải API thiết kế sẵn. Persistence/auth cần ADR và ca PostgreSQL thật khi hiện thực. Không chạy những suite ấy trong một review chỉ thêm tài liệu để giả đã chứng minh sản phẩm.

### Những điểm bổ sung tác giả cần sửa gọn

- §17 nói màn chính là danh sách sổ; shell hiện có bốn tab, Create hiện liệt kê hành động, không phải loại sổ. Cần route map thật: mục nào thêm, mục nào giữ; không khẳng định «không tốn gì» chỉ vì cùng `context_id`.
- §17.3 cấm mọi nhánh theo loại sổ ngoài một module không phải chứng minh quyền truy cập. Tập trung capability và presentation policy được; authorization vẫn cần check phía server tại biên đọc/ghi. Không đẩy kiểm consent vào config UI để qua phép đếm chuỗi.
- Một `guest_link` hiện gắn với `collection_envelopes`, không phải link tổng quát của couple. Digest/one-time token presentation không đồng nghĩa URL chỉ đọc được một lần. Đường khách là scope capability mới cần định nghĩa, không gọi là tái dùng miễn phí.
- §11/§14 nói chi tiêu chung hợp lý hơn chia bill: có thể là default hợp sản phẩm, nhưng không được làm người đã có nghĩa vụ mất đường đọc sổ cái. Giữ cửa truy cập lịch sử và nói rõ tổng tháng tính theo ngày nào, những phần nào; không gắn Nếp vào màn tiền.
- «Người nhà» chưa có hành vi nhưng xuất hiện như lựa chọn ngang hàng đã sẵn dùng. Nêu rõ chỉ phân loại hay có chức năng, hoặc hoãn nó khỏi cửa vào đầu tiên.
- §16.1 hướng node mới không có `text` để giữ cổng XML: giới hạn luật này vào art trang trí; nhãn trạng thái/chức năng phải đọc được. Khi feature hợp lệ đổi UI, cập nhật test đúng phạm vi thay vì giấu thông tin khỏi cây accessibility.
- §21 thiếu đường đóng sổ, huỷ/hết hạn lời mời, xem giấy đã mở, quản lý nhắc/nguồn, lỗi gửi/lưu, không tìm được kèo. Bảng 19 bề mặt chưa thể là inventory hoàn chỉnh. Không bắt buộc mỗi trạng thái thành màn mới, nhưng phải có nơi xử lý.
- Đường «chỉ một đôi» có thể là cắt scope V1; đừng dùng câu phán xét người dùng ở 865–866 để làm lý do kỹ thuật. Cũng không cần lưu một giới tính hoặc buộc tên «Sổ về anh/em» để có hai người.
- Dọn mô tả ba phần dọc còn sót ở 806; luật Nếp im lặng so với nói khi được gọi ở 1269; câu «hơi cũ mềm» còn ở 127; tham chiếu sai tới mục 9 cho số vật liệu. **Sửa chỗ cũ, không nối thêm mục 25 để giải thích mục 24.**

## 7. Đo thành công và mức stunning bằng đúng loại bằng chứng

### Phần công dụng

Đề nghị đo chuỗi sự kiện: đã được đề nghị → thực sự thấy → một người gửi → người kia phản hồi → cả hai chốt cùng bản → buổi đi được ghi nhận → giữ một điều → chủ động quay lại. Phân biệt lời rủ do Nếp đề nghị với lời do người dùng khởi xướng. Không dùng nội dung ghi chú riêng hoặc danh mục sổ được điền làm payload analytics.

Chỉ số ưu tiên để thử giả thuyết: công sức ra quyết định giảm không; lời rủ có được nhận là một hành động quan tâm thật không; hai người có tự muốn quay lại không. Đây là câu hỏi nghiên cứu cần phương pháp/consent được duyệt. Tỉ lệ «Ừ không Đổi» chỉ là một chỉ báo phụ: nó cũng có thể tăng khi đề nghị đều là chỗ quen dễ đồng ý.

«Nghỉ» có thể là sản phẩm đang tôn trọng một tuần bận, không tự là thất bại. Cho lý do tùy chọn như bận/không hợp/hẹn khác; không bắt người dùng giải thích đời sống riêng để được nghỉ. Các giả thuyết về trách nhiệm, giới, thói quen, nhớ và cảm xúc ở §0/§6 chưa có bằng chứng; không dùng giọng chắc chắn của văn để thay cho thử nghiệm.

### Phần hình ảnh

| Cần chứng minh | Bằng chứng tối thiểu đề nghị | Chưa được thay thế bằng |
|---|---|---|
| Cùng Nếp, khác công việc | Bảng Nếp nhóm/đôi 96 và 48; đọc không nhãn; sau đó xem cạnh nội dung | Đếm path hợp lệ hoặc có thêm pose |
| Mảnh giấy thực sự đọc thành một vật | Frame toàn viewport, mép/nếp/bóng cùng ánh sáng; so sáng/tối | Hai đường ngang và một tam giác |
| Người mới biết ai đọc được | Chỉ vào riêng/chung, gửi/mở/đóng mà không đọc spec | Nhãn trong Cài đặt hoặc `content-desc` đơn độc |
| Có một khoảnh khắc đáng nhớ | Storyboard từ lời rủ thành vật được giữ, nội dung do người dùng tạo | Nhiều texture hoặc animation dài |
| Chat vẫn dễ dùng | Native 360dp/chữ lớn/IME/tin mới/đọc lịch sử, dữ liệu tổng hợp | Frame thẻ rời không có app chrome |
| Motion đúng và không cản | Clip một vòng normal/reduced, semantic state đúng, gesture Back sống | Test duration thuần hoặc screenshot tĩnh |

Các tiêu chí về đọc hình/hành vi là proposal cho vòng kiểm kế tiếp. Không mặc nhiên tiến hành thử người thật khi ADR-0006 còn gác. Có thể so mù trong review thiết kế trước; điều đó vẫn không thành bằng chứng người dùng thật.

## 8. Điểm heuristic và giới hạn

Điểm dưới đây là nhận định về **độ hoàn thiện contract**, được tổng hợp từ A và kiểm B. Không so trend với điểm native các tuần trước, không dùng làm phần trăm hoàn thành feature.

| Heuristic | /4 | Điểm còn thiếu |
|---|---:|---|
| Hiện trạng hệ thống | 2 | Timeline hai máy, gửi/chốt/đổi chưa khép |
| Khớp đời thực | 2 | Giấy có nghĩa; ba chặng/vai cố định quá cứng |
| Kiểm soát và tự do | 3 | Nghỉ, tắt, tự rời tốt; hậu quả/undo chưa rõ |
| Nhất quán | 1 | Nếp, coral, mở/đóng, scope và khôi phục mâu thuẫn |
| Ngừa lỗi | 2 | Ý định consent tốt; biên nguồn và version còn thiếu |
| Nhận biết thay ghi nhớ | 2 | Nhiều tên vật, phạm vi đọc và hẹn mở chưa hiện đủ |
| Linh hoạt và hiệu quả | 3 | Đổi nhanh; thiếu đường cho buổi đi rất ngắn |
| Thẩm mỹ và tối giản | 2 | Có thế giới riêng; main screen vẫn dồn tác vụ |
| Phục hồi lỗi | 1 | Thiếu lỗi gửi, retry, offline, stale/thu hồi |
| Trợ giúp đúng lúc | 2 | Copy ngắn tốt; chưa rõ hậu quả các hành động nhạy cảm |
| **Tổng cho contract** | **20/40** | **Không phải điểm độ đẹp đã render** |

Tải nhận thức: bốn rủi ro rõ ở một tâm điểm, một quyết định tại một lúc, số lựa chọn và tiết lộ từng phần. Số «ba thẻ» không chứng minh màn nhẹ. §18.4 đếm «Sửa» là một cú bấm chỉ đếm mở trình sửa, chưa đếm hoàn thành việc; không nên dùng bảng đó như bằng chứng tổng công sức.

Ba persona cần kiểm: người luôn lo đang muốn được chia việc; đôi chỉ có 45 phút và ngân sách hạn chế; người mới dùng chữ lớn cần biết riêng/chung trước khi viết. Với cả ba, nhịp bắt đầu phải đem lại một kết quả nhỏ có nghĩa trước khi đòi họ học từ vựng sổ/gậy/trang/túi.

## 9. Phản biện gửi tác giả

**Chưa có cuộc trao đổi hai chiều tại thời điểm viết.** Cột trái là reviewer rút ra lập luận mạnh nhất của tài liệu, không phải phát ngôn mới của tác giả. Xin tác giả trả lời từng D bằng «giữ/sửa», lý do và vị trí spec đã hợp nhất; với khác biệt về UI, trả bằng frame cùng kích thước.

| ID | Lập luận có thể bảo vệ thiết kế gốc | Phản biện và yêu cầu trả lời |
|---|---|---|
| **D1** | App chọn để không ai chịu trách nhiệm; gậy để người kia được chủ động | Tôi đồng ý hai giá trị đều đáng thử. Xin vẽ timeline hai máy, chỉ rõ lúc nào là nháp riêng, lời ai gửi, và khi nào Nếp tự đề nghị. Hai lời hứa hiện cùng chiếm một trạng thái |
| **D2** | Sổ solo phải đi trước để người khởi xướng không bị treo | Đồng ý cần solo value. Vì sao phải chín bề mặt/bảy mục trước vòng rủ–đi–giữ? Có thể chỉ một ghi chú tay trong lúc chờ không? Nếu giữ 1a, xin tách metric ghi nhớ khỏi giả thuyết kèo |
| **D3** | Chat có sẵn, thêm ba thẻ ít tốn hơn một màn | Tận dụng code không tự làm trải nghiệm nhẹ. Xin hai frame 360dp/font 2.0/IME: ba thẻ và một khe thu gọn. Bao nhiêu hội thoại còn đọc được, tin mới tới có nhảy không? |
| **D4** | Ba chặng bảo đảm công bằng, cũng chính là ba phần giấy | Công bằng không bắt buộc ba địa điểm. Xin chứng minh luồng chỉ ăn 45 phút vẫn tốt; tách số chặng khỏi số nếp. Người dùng có được cả hai cùng chọn một chỗ không? |
| **D5** | Một coral và số stddev làm chất lượng kiểm được | Các số đang tự mâu thuẫn và không đo nghĩa hình. Xin sửa cặp màu/ROI/scope. Đưa specimen và đọc mù; giữ số như guard kỹ thuật, không giấy chứng nhận đẹp |
| **D6** | Không biên nhận làm người nhận còn cảm giác được nhớ tới | Giữ việc không báo đã xem câu hỏi riêng. Nhưng ai gửi lời rủ phải trung thực, và consent kèo phải thấy được. Xin phân biệt read receipt, trạng thái giao dữ liệu và chấp thuận lời rủ |
| **D7** | Sổ riêng hợp lý vì nó là quan sát và cả hai đối xứng | Trước consent chưa có đối xứng; cold start lại dùng gu toàn cục. Xin một bảng dữ liệu cho pending/active/closed/reopened và quy tắc ghi tay/Nếp đọc/gợi ý chung |
| **D8** | Tên «Nếp truyền giấy» đủ mạnh để kéo người dùng | Đợt 1 đã có truyền nửa giấy onboarding và lời rủ qua gậy, nhưng chưa có vòng gửi–mở–giữ mảnh giấy ở §5. Xin một vòng ba bề mặt chứng minh lời rủ thành kỷ niệm, hoặc lý do kiểm chứng được để trì hoãn vòng đó tới đợt 3 |

### Gói sửa đề nghị cho bản tiếp theo

1. Hợp nhất §0/§2/§4 thành một lời hứa và một state machine; chọn vai theo lượt hoặc giải thích được vai cố định.
2. Hợp nhất §3/§7/§9/§10/§17/§18 thành một bảng consent/nguồn/vòng đời. Bỏ các đường cold-start và mở lại mâu thuẫn.
3. Sửa §20–§23 bằng layout toàn màn, phép tính màu thật, accent có scope, semantic trạng thái và inventory đủ đường thoát/lỗi.
4. Vẽ storyboard nhỏ của vòng rủ–đi–giữ, bảng Nếp 96/48. Chọn một phương án cùng Lead trước khi nhân 19 bề mặt.
5. Viết lại §12–§13 theo lát đã chọn, việc gì đo được/không đo được, và chỗ phải có ADR. Không thêm chương để vá chương cũ.

Lệnh Impeccable phù hợp theo thứ tự: `clarify` cho contract/copy, `distill` cho scope và vai, `shape`/`layout` cho bề mặt, `animate` cho vật xuyên trạng thái, `audit` cho màu/a11y, `polish` sau khi vòng đã đúng. Đây là đường sửa để planner sử dụng, không phải các lượt UI đã chạy trong review này.

## 10. Kiểm chứng đã thực hiện

- Đọc toàn bộ spec tại SHA cố định; kiểm các nguồn nêu ở mục 1. Đã xem ba ảnh tham chiếu cũ, giữ nguyên phạm vi bằng chứng.
- Hai assessment độc lập; [Assessment B](mode-hai-nguoi/assessment-b.md) ghi source và giới hạn chi tiết.
- Detector chạy thật đúng target: exit 0, [kết quả `[]`](mode-hai-nguoi/detector.json). Markdown trực tiếp đi qua regex fallback; không có engine kiểm câu chuyện, ASCII UI hoặc native. Không có browser target feature để inspect; không tạo overlay hoặc server giả.
- Root chạy [script tái tính màu](mode-hai-nguoi/kiem_mau_spec.py), lưu [kết quả](mode-hai-nguoi/ket-qua-mau.json). Tái lập từ gốc repo: `python3 docs/codex/2026-09-12/mode-hai-nguoi/kiem_mau_spec.py`. Script đọc token qua Git ở SHA gốc, không đọc cấu hình riêng hoặc dữ liệu người dùng.
- Đây là thay đổi tài liệu và artifact kiểm số. Chưa hiện thực UI/backend; không dùng test sản phẩm không liên quan để tuyên bố feature đã đúng. Repo guard và kiểm whitespace được chạy trước commit, kết quả giao hàng ghi trên PR.

**Quyết định cần tác giả phản hồi:** sửa C1–C6 để spec có một nghĩa; tranh luận D1–D8 để chọn cấu trúc trải nghiệm. Hướng giấy–mực/Nếp đủ tốt để tiếp tục phát triển, còn độ stunning cần được chứng minh bằng vòng trải nghiệm có hình và native sau khi contract được chốt.
