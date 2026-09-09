# Audit native Rủ Đi — main 92d0f241, ngày 09/09/2026

## Kết luận cho leader

**REQUEST_CHANGES cho nghiệm thu toàn bộ trải nghiệm native. Đồng ý giữ hướng mỹ thuật hiện tại.** Hai kết luận này không mâu thuẫn: app đã có diện mạo và câu chuyện riêng rõ hơn, nhưng một số tình huống sử dụng thật chưa đạt chất lượng tương ứng với hình vẽ.

Không đề nghị quay lại thiết kế từ đầu, bỏ Nếp, hoặc vẽ thêm hàng loạt cảnh. Tám sticker đã thống nhất ngôn ngữ, các màn có bố cục khác nhau theo nhiệm vụ, và câu chuyện “chừa một chỗ cho nhau” đã xuất hiện qua hành động. Việc tiếp theo là sửa đường phục hồi lỗi, kiểm soát phản hồi muộn trong chat, sửa chữ lớn, rồi tinh chỉnh một vài nghĩa hình còn yếu. **Chưa đủ bằng chứng để nói animation mượt hoặc toàn app đã được kiểm chứng native.**

Đánh giá này dùng Impeccable của repository: audit native làm khung kỹ thuật; critique bổ sung nhận xét về câu chuyện, hình và tính riêng. Hai nhánh đánh giá độc lập đã được khởi chạy. Nhánh A hoàn tất; nhánh B trả kết quả source/test sơ bộ nhưng dừng vì giới hạn sử dụng. Reviewer chính kiểm tra lại các phát hiện được đưa vào đây; không gọi đây là hai bản nghiệm thu độc lập hoàn chỉnh.

## 1. Nguồn, phạm vi và giới hạn

- Checkout: `/home/lakiet/mobile`; HEAD và remote-tracking `origin/main` cùng `92d0f241f06fa873d719d9bd8c84c1918949fcf0`. Đã fetch và đối chiếu ở phần đầu đợt audit; lần kiểm tra cuối vẫn cùng SHA, không phải lời cam kết remote sẽ không có commit mới sau đó.
- Đọc ba trả lời mới của team: [delta 584–585](../../claude/2026-09-09/tra-loi-review-delta-584-585.md), [tám sticker](../../claude/2026-09-09/tra-loi-tam-sticker.md), [cảnh mới](../../claude/2026-09-09/tra-loi-canh-moi-im-lang.md), cùng source hiện tại và DESIGN.md.
- Native: Android emulator `emulator-5554`, 1080×2400 px, density 420, khoảng 411×914 dp; dev client có sẵn, không build lại APK release. JS bundle có fingerprint `codex-sep09-92d0f241` ngay trên [ảnh Welcome](native-audit-evidence/01-welcome.png).
- Chạy dữ liệu tổng hợp với `EXPO_PUBLIC_RUDI_FIXTURE=1`. API audit trỏ cổng loopback không phục vụ để kiểm tra lỗi kết nối; không đăng nhập tài khoản thật hay ghi dữ liệu backend.
- Có **28 cặp PNG/XML; 24 cặp hợp lệ cho mục tiêu chụp**. Bốn cặp 11–14 bị chuyển về Welcome sau đổi font: giữ để truy vết, **loại khỏi bằng chứng khay sticker**. Không suy từ tên file ra nội dung. Không có ảnh số 15.
- [Manifest](native-audit-evidence/manifest.json) ghi SHA-256 và đánh dấu bốn cặp loại; kiểm tra cuối: 56 file khớp hash, các link trong báo cáo đều có đích local. Hash không thay thế bằng chứng xuất xứ, và chưa phải pin vào allowlist để commit.
- Khay ở font 1.0 light, 1.3 dark và 2.0 dark; Explore và create sheet có kiểm tra bổ sung 2.0 dark. Đây **không phải** ma trận mọi màn × hai theme × ba cỡ chữ.
- Khi tiếp tục lượt đo motion cuối, ADB 5038 không trả lời cả trong và ngoài sandbox; helper `/tmp` trước đó không còn. Không có video mới hay phép đo frame time sạch để bàn giao. Các ảnh lưu trong repo vẫn đọc được. Không dùng thống kê gfxinfo tích lũy lẫn nhiều thao tác trước đó làm benchmark.
- Cuối lượt chụp đã đưa font về 1.0 và theme light. Ở lượt tiếp tục, không còn thấy process Metro/emulator audit qua kiểm tra process; hai ADB client bị treo do reviewer vừa mở đã được dừng. Không xác nhận lại được trạng thái thiết bị hoặc reverse port khi ADB mất kết nối.
- Không kiểm tra mới trên iOS, máy vật lý, tablet/foldable, multi-window, TalkBack/VoiceOver hoặc bàn phím trong chat live. Không kiểm chứng send/error/retry qua API OTP đang chạy. Bảng queue dùng component thật với state tổng hợp, không thay thế bằng chứng transport.
- Không sử dụng ảnh team ghi rõ thuộc phiên Google thật. Bảng hình team được ghi là **art sheet**, không gắn nhãn native capture mới.

## 2. Đẹp, sinh động, có câu chuyện và có chất riêng chưa?

### Điều đã đạt — nên giữ

**Bản sắc:** nền giấy, mực indigo, nếp gấp coral, dấu đóng và nét vẽ có quan hệ với nhau. Nếp không còn giống linh vật đứng cạnh một bộ UI bất kỳ. Đây là nhận xét về tính nhất quán và đặc trưng trong sản phẩm, không phải xác nhận “độc nhất trên thị trường”.

**Câu chuyện:** chừa ghế, nhìn qua khung ảnh, đưa lời mời, treo kỷ niệm là những động từ của việc đi cùng nhau. Hình có lý do xuất hiện, không chỉ để lấp khoảng trắng. Giữ quyết định không đưa Nếp cười vào trạng thái lỗi.

**Đa dạng bố cục:** Welcome là trang bìa; Sở thích là lưới lựa chọn; Explore có điểm dẫn, cặp so sánh và hàng địa điểm; lịch trình có trục thời gian; chia bill dùng cấu trúc sổ; Album để ảnh dẫn. Không đúng nếu kết luận toàn app vẫn chỉ lặp một kiểu card. Những màn tiền và thiết lập không cần nhiều linh vật hơn: sự yên tĩnh giúp đọc và tin số liệu.

**Tám sticker:** [khay 64dp light](native-audit-evidence/10-stickers-light.png) và [khay chữ lớn dark](native-audit-evidence/18-tray-font2-dark.png) cho thấy một hệ nét/gấp giấy thống nhất, có hành động khác nhau. “Đi thôi!” có sải chân, “OK, chốt!” có động tác đặt dấu, “Tuyệt vời” có sức bật; không còn bảy sticker cũ đi cùng một sticker mới lạc hệ.

Riêng **“Chờ tí”: reviewer đọc được ý chờ/giữ chỗ** từ ghế và đồng hồ, nhất là ở bubble. Đủ để giữ hướng đã chọn. Đây là đánh giá chuyên môn, chưa phải kết quả người dùng nhìn không nhãn ở kích thước thật.

### Điều chưa đạt tới mức “xong toàn bộ”

1. **Một số hình vẫn cần nhãn để chốt nghĩa.** “Kẹt xe” hiện dễ đọc thành “đi xe”; “Trả tiền nè” có thể đọc thành “đưa tờ vé”. Không cần thêm tiểu tiết nhỏ: sửa silhouette/tư thế để thấy bị chặn hoặc đang trao phần của mình. Không vẽ ngân hàng, QR hay tín hiệu xác nhận giao dịch.
2. **Một cặp cảnh mới còn gần cùng hành động.** `chua-co-keo` và `chua-co-tin-nhan` đều dùng `ghi-lai`, chỉ đổi đạo cụ/biểu cảm/placement ([source](../../../apps/mobile/src/rudi/art/canh.ts), dòng 204 và 221; [sheet mới của team](../../claude/2026-09-09/may-nep/05-muoi-canh-sau.png)). Dùng lại pose không tự nó là lỗi. Nhưng cặp này chưa làm rõ “bắt đầu một kế hoạch” khác “mở lời với người khác” ở đâu. Chỉ cần tách một cảnh bằng quan hệ tay–đạo cụ–người được mời; không yêu cầu mỗi cảnh phải có một bộ giải phẫu riêng.
3. **Cảnh lỗi vẫn có tín hiệu dễ nhầm.** Vòng coral rời dưới tờ giấy rách ở ảnh 22 trông giống một loading/retry ring đứng yên. Đây là nhận xét thị giác, không phải kết quả khảo sát. Nên gắn dấu vào vết rách/đường bị đứt hoặc bỏ vòng; giữ trạng thái “đã lỗi, có thể thử lại” rõ hơn trạng thái “đang tải”.
4. **Chất riêng chưa được chứng minh ở chuyển động nối các bước.** Source có nền tảng motion tốt, nhưng chưa đủ để nói hành trình từ rủ → chốt → chia → giữ kỷ niệm đã có nhịp riêng trên thiết bị.

Không nên tăng số cảnh hoặc làm mọi vật nhún liên tục để giải quyết bốn điểm này. Độ sống nằm ở nghĩa của hành động và phản hồi đúng lúc, không ở số lượng animation.

## 3. Phát hiện cần giao team

### F41 — P1: Nút “Thử lại” trong hàng sticker lỗi tràn ra ngoài màn

- **Bằng chứng:** [PNG 16](native-audit-evidence/16-failed-sticker-dark.png), [XML 16](native-audit-evidence/16-failed-sticker-dark.xml). Hàng lỗi có thể thử lại chỉ còn mép phải nút ở mép trái màn; chữ “Thử lại” biến mất. “Bỏ” chiếm phần còn lại. Đây là native của `HangChoGui` trong bảng tổng hợp.
- **Vị trí/nguyên nhân:** `GroupChatLive.tsx:123–125` đặt hai `RudiButton compact` trong một hàng. `RudiButton` mặc định `full=true` (`ui.tsx:416`), `buttonFull.width="100%"` (`:1055`), lại không co (`:1054`). Hai nút full-width cộng gap không thể vừa hàng.
- **Tác hại:** đúng lúc gửi lỗi, người dùng không nhìn thấy hành động phục hồi. Không nói nút hoàn toàn bất khả dụng: XML vẫn có label và một phần vùng chạm còn trên màn.
- **Sửa:** kích thước theo nội dung hoặc chia chiều rộng hàng hợp lệ; cho phép chuyển dọc khi font lớn. Đừng thu chữ hoặc giấu “Thử lại” để vừa.
- **Đóng khi:** chụp component này và chat live ở 1.0/1.3/2.0, light/dark; cả hai nút và nhãn đều trong khung; retry vẫn dùng Attempt cũ; lỗi vĩnh viễn chỉ có “Bỏ”. Lệnh phù hợp: `/impeccable adapt`, sau đó `/impeccable harden`.

### F42 — P1: Phản hồi gửi muộn có thể nhập tin nhóm A vào state nhóm B

- **Mức bằng chứng:** source và probe bộ lập lịch hook; **chưa tái hiện bằng React renderer/native/API thật**.
- **Vị trí:** `useTinNhan.ts:105–109` chỉ reset queue khi đổi context/person. `chay():148–154` vẫn nhận kết quả gửi cũ, ghép vào `tinRef.current`, rồi gọi `napMoi` đóng trên context cũ. Các hàm tải cũng cần kiểm tra response có còn thuộc phiên/context hiện hành không.
- **Tái hiện:** [probe-late-context.mjs](native-audit-evidence/probe-late-context.mjs) transpile hook và các helper thật, dùng transport tổng hợp và scheduler nhỏ. Tải A → gửi nhưng giữ response → đổi cùng instance sang B và tải B → resolve gửi A. Trước: `[B-existing]`; sau: `[B-existing, A-late-send, A-existing]`; lượt đọc: `[A, B, A]`.
- **Tác hại:** nội dung đang hiển thị có thể sai nhóm. Đây không phải bằng chứng backend gửi dữ liệu trái quyền hay rò rỉ chéo tài khoản đã quan sát được; là lỗi tính đúng đắn của state frontend với response muộn.
- **Sửa:** gắn generation/context/person vào tác vụ; bỏ mọi completion cũ khi context/session đổi hoặc unmount. Chặn cả thành công và thất bại, load/poll/send/read-mark; không chỉ reset mảng queue. Giữ nguyên Attempt cho retry trong đúng cuộc hội thoại.
- **Đóng khi:** test tích hợp React với deferred transport cho A→B, đổi người dùng, unmount, response load/poll muộn và send failure muộn; rồi tái hiện native synthetic. Không còn tin A hoặc thông báo lỗi của A trong B. Lệnh phù hợp: `/impeccable harden`.

### F43 — P1: Trạng thái mất kết nối vẫn nói với lập trình viên

- **Bằng chứng native thật:** [PNG 22](native-audit-evidence/22-network-error.png), [XML 22](native-audit-evidence/22-network-error.xml): màn Đi đâu? in URL loopback và “Máy chủ có đang chạy không?”. Đây là lỗi kết nối được chủ động tạo trong audit, không phải incident production.
- **Vị trí:** `api.ts:271`; nhánh 404 ở `:276` còn hướng dẫn kiểm tra bản API và địa chỉ máy chủ. `DiemDenScreen` hiển thị thông điệp này.
- **Tác hại:** app đang dùng giọng thân thiện về việc đi chơi, đến lúc hỏng lại yêu cầu người dùng vận hành server. URL không giúp họ chọn bước tiếp theo. Không có điều kiện chỉ-dev tại nhánh formatter này.
- **Sửa:** thông điệp có hành động, ví dụ “Chưa kết nối được. Kiểm tra mạng rồi thử lại.”; giữ chi tiết kỹ thuật ở log/kênh hỗ trợ. Không thay những lời từ chối nghiệp vụ vốn đúng bằng một câu chung, và không hứa “chưa ghi gì” khi chưa biết server đã ghi hay chưa.
- **Đóng khi:** test formatter và native no-network/404/5xx; UI không in hostname/port/API-version instructions; Retry thể hiện đang xử lý và trạng thái cuối. Lệnh: `/impeccable clarify`, `/impeccable harden`.

### F44 — P2: Placeholder tìm kiếm bị cắt ở font 2.0

- **Bằng chứng:** [PNG 20](native-audit-evidence/20-explore-font2-dark.png). “Tìm quán, món…” xuống dòng và bị cắt ở đáy ô; tab bar bên dưới đã tăng chiều cao đúng.
- **Vị trí:** `SearchField` → `Field`/native TextInput (`ui.tsx:574`, style `:1062–1066`) và hàng tìm kiếm Explore. Không khẳng định có `height` cứng trong source: style dùng minHeight, nhưng kết quả native vẫn không đủ chỗ cho placeholder khi bố trí này nhận font 2.0.
- **Tác hại:** người cần chữ lớn nhận một ô nhập trông hỏng. Đã là nợ cũ có tên; lần này chưa đóng.
- **Sửa:** xác định rõ một dòng tìm kiếm, đo chiều cao theo font và kiểm tra placeholder native; hoặc dùng nhãn ngắn mà vẫn giữ mô tả trợ năng đầy đủ. Đừng tắt font scaling toàn ô.
- **Đóng khi:** 1.0/1.3/2.0, cả placeholder và chữ nhập, màn hẹp, hai theme; caret và nội dung đều đọc được. Lệnh: `/impeccable adapt`.

### F45 — P2: Một số nghĩa hình chưa tự đứng được

Gộp có chủ đích ba điểm thị giác ở §2: cặp cảnh `ghi-lai`, hai sticker “Kẹt xe”/“Trả tiền nè”, vòng lỗi tách rời. Không coi đây là bốn blocker riêng hoặc ép vẽ lại cả hệ.

- **Tác hại:** người mới hiểu nhờ nhãn nhiều hơn nhờ hình; “một hình thay cho một câu” mới đạt một phần.
- **Đóng khi:** một vòng sửa nhỏ với A/B ở 64/120dp cho hai sticker, và cảnh trong màn thật; thử bỏ nhãn với vài người chưa đọc brief, ghi lại từ họ thực sự nói. Đây là kiểm tra định tính, không bịa tỷ lệ nhận biết hoặc dùng AI tự đoán làm user test.
- **Hướng:** giữ khung Nếp, sửa hành động/quan hệ, không tăng chi tiết nhỏ. Lệnh: `/impeccable critique`, `/impeccable clarify`.

### F46 — P3: DESIGN.md còn mô tả trạng thái đã bị thay thế

`DESIGN.md:1083` nói đã bỏ `nang-bong` nhưng `:1140` vẫn liệt kê pose đó trong phần cảnh; `:1690–1694` còn nói cảnh chưa có màn gọi và khay có bảy sticker cũ. Những đoạn lịch sử chưa được đánh dấu rõ khiến người tiếp theo có thể khôi phục quyết định cũ.

Đánh dấu lịch sử theo SHA/ngày hoặc cập nhật phần hiện hành, không xóa bằng chứng cũ. Đóng khi tài liệu phân biệt “đã từng” và “đang dùng”, khớp consumer hiện tại. Lệnh: `/impeccable document`.

**Tổng phát hiện trong báo cáo:** 0 P0, 3 P1, 2 P2, 1 P3. F42 là phát hiện source/probe có điều kiện tái hiện, không gắn nhãn native-confirmed.

## 4. Đối chiếu các mục team đã sửa

| Mục | Đánh giá lần này | Giới hạn |
|---|---|---|
| F21 ảnh Bình chọn | Đúng quyết định: ba ô vẽ cùng kích thước, không còn một ảnh kéo mắt | Puppy Farm vẫn là tay cầm game; nợ nghĩa hình chưa đóng |
| F31 cổng ghi công | 12/12 probe của team đạt; các lối đã nêu trong yêu cầu cũ được chặn | Không gọi AST gate là chứng minh mọi chương trình đều in credit; không mở lại F31 chỉ vì có thể nghĩ ra cú pháp mới |
| F32 queue/Attempt | Hướng lưu Attempt và phân biệt lỗi retry/permanent đúng; test helper đạt | F41/F42 và live transport còn mở, nên chưa nghiệm thu chat |
| Tám sticker | Duyệt hướng hình thống nhất, không còn khay pha hai hệ | Nhận biết không nhãn và live send vẫn chưa được user/native E2E chứng minh |
| Khay chữ lớn | Native 3 cột ở 1.3, 2 cột ở 2.0; tám nhãn đọc được trên cấu hình đã chụp | Không suy thành mọi sheet/mọi thiết bị đều đạt |
| Cảnh mới | Có tình huống và quan hệ nhân vật–đạo cụ; error không gắn Nếp | Cặp hành động còn giống, vòng lỗi còn mơ hồ; sheet không phải bằng chứng mọi consumer |
| Tab bar | Label wrap và chiều cao tăng thấy rõ ở ảnh 20 | Không bao gồm TalkBack/IME hoặc tablet |

Nợ cũ vẫn giữ tên, không nhận vơ đã sửa: ảnh Album/timeline demo chưa ghi công tương ứng, bare Photo thiếu chữ lỗi, nút disabled coral nhạt, dấu phân cách credit bị lẻ. Ảnh 25/26 là fixture; không dùng để đóng các case AlbumLive hoặc lịch trình live. Disabled mờ là vấn đề chất lượng/khả năng hiểu, không tự động tuyên bố vi phạm chuẩn tương phản của control đang hoạt động.

## 5. Animation: kết luận được gì, chưa kết luận được gì?

Source có điểm tốt: `useMotion.ts` đọc Reduce Motion ban đầu, nghe thay đổi, timing/spring dùng `ReduceMotion.System`; PhotoViewer chọn `none` khi giảm chuyển động, dùng FlatList phân trang và giới hạn số ảnh render. Đây là nền tảng hợp lý.

Root stack vẫn khai báo slide ngang/dọc ở `_layout.tsx:155,169,173,177`. **Không kết luận chỉ từ đó rằng vi phạm Reduce Motion:** native stack/OS có thể xử lý thiết lập hệ thống. Cần kiểm tra trực tiếp cả stack và các primitive.

Không có lượt benchmark release/profile hoặc video mới hoàn tất. Do đó chưa thể ký “mượt 60/120Hz”, “không jank”, “haptic đã tốt”, hoặc “motion mới mẻ xuyên suốt”. Test motion xanh chỉ chứng minh logic đã test, không chứng minh frame pacing hay cảm giác chạm.

Đề nghị một gate hữu hạn, không mở dự án animation mới:

1. Trên build release/profile và thiết bị có tên/cấu hình, warm-up rồi đo riêng chuỗi tab/scroll, sheet open/close, album swipe/pinch, gửi–lỗi–retry, back trong lúc đang chuyển. Ghi frame-time distribution, slow/frozen frames và thao tác tương ứng; tách lượt ghi video khỏi lượt đo nếu recording ảnh hưởng kết quả.
2. Lặp chuỗi ngắn khi Reduce Motion bật, kiểm tra không còn chuyển động lớn ngoài mong đợi, không mất trạng thái hoặc affordance.
3. Chỉ sau khi đường thao tác ổn, chọn tối đa một chuyển động mang nghĩa Rủ Đi: chẳng hạn xác nhận chốt kế hoạch bằng dấu giấy ở đúng kết quả thành công. Đây là đề xuất, chưa phải tính năng đã thấy trên máy. Không dùng animation để ngụ ý backend đã ghi khi chưa có phản hồi.

## 6. Điểm kiểm tra — không biến thiếu bằng chứng thành điểm xanh

### Native technical health

| Chiều | Điểm tạm /4 | Căn cứ |
|---|---:|---|
| Accessibility | 2 | Khay/tab chữ lớn cải thiện; search còn cắt, retry mất nhãn thị giác; chưa kiểm tra screen reader |
| Performance | Chưa chấm | Thiếu lượt đo motion sạch và release/device evidence |
| Appearance/theming | 3 | Nền giấy/mực, sticker light/dark tốt ở các mẫu; nợ disabled/media còn |
| Platform conformance | 3 | Đọc như app native, có sheet/back/insets trên mẫu; chưa xác nhận IME/predictive Back/iOS |
| Adaptivity | 2 | Khay/tab đáp ứng text scale; chưa chứng minh tablet/windowing, app còn portrait lock |

**10/16 ở bốn chiều đã chấm tạm; không có tổng /20 hợp lệ.** Không gán rating “Good/Excellent” cho toàn native bằng cách tự điền điểm performance. Không coi portrait lock hoặc predictive Back tắt là lỗi trải nghiệm đã quan sát; đó là phạm vi cần quyết định/kiểm chứng.

### Critique UX — Operate, đánh giá chuyên môn có giới hạn

| Heuristic | /4 | Ghi chú |
|---|---:|---|
| Hiển thị trạng thái | 2 | Có pending/error nhưng recovery chưa trọn |
| Ngôn ngữ đời thực | 3 | Câu chuyện rủ bạn tốt; error copy phá giọng |
| Quyền kiểm soát | 2 | Có Bỏ/Retry; Retry bị tràn |
| Nhất quán | 3 | Tám sticker đồng hệ, tài liệu còn drift |
| Phòng lỗi | 2 | Attempt đúng hướng; response cũ chưa cô lập |
| Nhận biết hơn ghi nhớ | 3 | Hình/nhãn hữu ích; hai sticker còn mơ hồ |
| Hiệu quả thao tác | 3 | Hàng, timeline, picker theo nhiệm vụ; chưa duyệt mọi live flow |
| Tối giản | 3 | Đã phân cấp hình/chữ; không cần thêm cảnh mọi nơi |
| Nhận biết và phục hồi lỗi | 1 | F41/F43 chạm trực tiếp bước phục hồi |
| Hướng dẫn/trợ giúp | 2 | Có nhãn và gợi ý; hướng dẫn server không phù hợp người dùng |
| **Tổng** | **24/40** | **Điểm UX trên phạm vi đã đọc/chụp; không phải điểm đẹp của bộ tranh** |

Hành trình cảm xúc hiện tại: trang bìa gây chú ý → chọn gu và lên kế hoạch rõ ràng → Nếp tạo cảm giác có bạn đồng hành → khi lỗi, lời kỹ thuật và nút tràn làm đứt mạch. Người mới chịu ảnh hưởng nhất ở F43/F45; người cần chữ lớn ở F41/F44; người chuyển nhanh giữa nhóm ở F42. Tám lựa chọn có nhãn không phải tám thứ phải ghi nhớ, nên không báo lỗi “quá tải vì có tám sticker”.

## 7. Bản đồ bằng chứng và kiểm tra

| PNG/XML trong `native-audit-evidence/` | Nội dung | Loại bằng chứng |
|---|---|---|
| 01–03 | Welcome, login, sở thích | Native dev/demo; không xác minh đăng nhập thật |
| 04–08 | Explore, timeline, chat, profile, tài chính | Native fixture |
| 09–10 | Create sheet, khay sticker light | Native fixture/component board |
| 11–14 | Welcome sau recreation, tên file ghi nhầm sticker | **Loại khỏi mục tiêu kiểm tra sticker** |
| 16 | Pending/retry/permanent failure sticker | Native component board, state tổng hợp |
| 17–19 | Khay dark 1.3/2.0, kiểm tra cuối khay | Native component board; 19 gần trùng 18 |
| 20–21 | Explore và create sheet dark 2.0 | Native fixture |
| 22 | Lỗi kết nối Đi đâu? | Native màn thật, API cố ý không phục vụ |
| 23–29 | Bill review, gán món, settlement, Album, vote, tạo cuộc đi, settings | Native fixture; không phải lưu/ghi backend thật |

Kiểm tra chạy lại lúc hoàn tất báo cáo:

- `npm run typecheck`: đạt.
- Compile `tsconfig.test.json`, `node tools/fixup-esm.mjs`, rồi sáu file test `rudi-anh-ghi-cong`, `rudi-chat-sticker`, `rudi-chat-tin-song`, `rudi-chat-hang-cho`, `adaptive`, `motion`: **6/6 file test đạt**, theo output runner. Không báo đây là chạy lại 755/3433 test của team.
- `node docs/claude/2026-09-09/kiem-cong-ghi-cong.mjs`: 12/12 ca đạt, không whole-file bypass.
- `node docs/codex/2026-09-09/native-audit-evidence/probe-late-context.mjs`: tái hiện F42. Exit 0 của probe nghĩa là **tái hiện lỗi lịch sử**, không phải app đã pass. Script cố ý assert lỗi còn hiện diện; khi sửa đúng cần chuyển thành regression test trong harness React.
- Detector đã chạy ở nhánh critique trước đó, trả `[]`; không lấy kết quả regex/TSX làm nghiệm thu native.
- Không chạy lại toàn backend suite, CI hay repo guard staged vì không stage/commit. Không sửa product source, không push/merge. Báo cáo, probe và chứng cứ mới chỉ ở local; chưa pin/commit lên GitHub.

## 8. Thứ tự làm tiếp và tiêu chí mở cổng

1. **P1 — `/impeccable harden` + `/impeccable adapt`:** F42 và F41 trước. Chốt state isolation và hàng retry trong cùng lượt kiểm tra live synthetic; giữ Attempt/reply semantics hiện tại.
2. **P1/P2 — `/impeccable clarify` + `/impeccable adapt`:** F43 và F44, không đổi hàng loạt copy nghiệp vụ hay tắt scaling.
3. **P2 — `/impeccable critique`:** một pilot nhỏ cho F45. Giữ bộ tám và hệ cảnh; chỉ chỉnh các nghĩa yếu. Đồng thời xử lý nợ glyph farm, media demo và disabled theo scope đã có.
4. **Gate còn thiếu — `/impeccable optimize` + `/impeccable audit`:** motion thật, Reduce Motion, IME và các live failure cases. Ghi rõ thiết bị/state chưa quét; không dùng screenshot board thay cả flow.
5. **P3 — `/impeccable document`**, rồi **`/impeccable polish`** là bước cuối, sau khi hành vi đúng.

**Được tiếp tục triển khai trong hệ hình này; chưa được gọi là “full native đã đẹp, mượt và hoàn tất”.** Không cần chờ vẽ lại toàn bộ app. Cổng tiếp theo nên là các bằng chứng đóng F41–F44, một quyết định có kiểm tra cho F45, và motion/live coverage còn thiếu; không phải số lượng cảnh mới hay số test xanh tăng thêm.

Người dùng có thể chọn chạy từng bước, cả nhóm hoặc đổi thứ tự. Audit này không tự cấp quyền sửa frontend, thay đổi PR hoặc merge.
