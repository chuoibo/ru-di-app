# Assessment B — detector và đối chiếu bằng chứng nguồn

Target cố định: `docs/superpowers/specs/2026-09-12-mode-hai-nguoi-nep-truyen-giay-design.md` tại **9c0d6f98**. Snapshot `/tmp/rudi-mode-hai-nguoi-audit-9c0d6f98/spec.md` khớp byte với `git show`; 1667 dòng; SHA256 `9d2eb0a08cda1f07a3dc805d32b2de82b2f40c2f8b32682ff13aa10e7858821c`.

Đánh giá độc lập; chưa đọc Assessment A. Không sửa source, không dựng hay khởi động app, không tạo native evidence. HEAD workspace khi đối chiếu là `714f469bb0d3da895a149254ec7748311b22ed49`, khác revision doc. Toàn bộ trích nguồn bên dưới dùng `git show 9c0d6f98:...`. Diff giữa revision này và HEAD cho `nep.ts`, tokens, tab layout, motion, chat không cho thay đổi; phần khác trong `src/rudi` có thay đổi bản đồ native nên không nhập nhằng capability bản đồ hai revision.

## 1. Detector thật, phạm vi rất hẹp

Đã chạy lệnh bắt buộc:

`node .claude/skills/impeccable/scripts/detect.mjs --json /tmp/rudi-mode-hai-nguoi-audit-9c0d6f98/spec.md`

Exit **0**, stdout **`[]`**, findings count **0**, rule names **không có**, file locations **không có**, false positives **không có findings để phân loại**. JSON lưu tại `/tmp/rudi-mode-hai-nguoi-audit-9c0d6f98/detector.json`.

Coverage quan trọng: `.md` không nằm trong `SCANNABLE_EXTENSIONS` của `scripts/detector/node/file-system.mjs:26–30`, nhưng vì truyền **một file trực tiếp**, `cli/main.mjs:108–112` vẫn đọc nội dung và đưa vào `detectText` (regex fallback). Vì vậy không nói sai rằng tool bỏ qua file hoàn toàn; nó quét văn bản bằng các quy tắc hướng HTML/CSS/JSX, **không có engine hiểu đặc tả Markdown, ASCII wireframe, thiết kế React Native hay câu chuyện**. `[]` không chứng minh bất cứ màn native nào sạch.

Browser steps **skipped có chủ đích**: target là đặc tả chưa triển khai, không có URL render feature, screenshot hay prototype feature. Render Markdown trong browser chỉ tạo hình tài liệu, không cho bằng chứng UI của ứng dụng. Không có browser console findings, không inject, không overlay, không mở server. Đây là dual-agent review một spec; giới hạn là **design/spec evidence**, không phải native acceptance.

## 2. Bằng chứng hình của feature chưa tồn tại trong spec

Quét toàn 1667 dòng: **0 Markdown image**, **0 link .png/.jpg/.jpeg/.svg/.webp/.mp4**. Có ASCII wireframe và bảng tạo hình, không có storyboard có hình, concept `manh`, screenshot hay clip nghi thức. Mục 19 viện dẫn concept sheet Nếp 08/09 để giữ nhân vật cũ; không phải hình cho feature mới.

Vì vậy có thể kết luận story có/không mạch lạc và constraint có/không khả thi; **chưa thể kết luận stunning, phân biệt được hai loại sổ ở 48dp, đọc được nếp gấp, hoặc motion đẹp**. Thêm tiêu chí gate vật liệu không thay thế art-direction proof.

## 3. Những claim nguồn xác nhận được

- Anatomy Nếp **ở mục 19.1–19.5 đúng với source**: `apps/mobile/src/rudi/art/nep.ts:1–18` mô tả tờ giấy rộng, hai ve áo, góc coral, mắt chấm, một mày, miệng nghiêng, tay chân; `:66` có sáu `BIEU_CAM`; tư thế map đủ `nghieng/nhin/bieuCam/dang`. Source xác nhận lớp tháo được, không đứng cạnh tiền/lỗi/xung đột. `gap: "manh"`/`giu-kin`/sáu pose mới là đề xuất, **chưa có trong nguồn tại revision**.
- `packages/shared/tokens.json` xác nhận space 6/10/16/24/36/48, radius 10/14/20, label/body/title/h1 14/17/20/28. `adaptive.ts:137–138` xác nhận `chuLon >= 1.28`.
- `apps/mobile/app/(tabs)/_layout.tsx:18–21` đúng bốn tab Khám phá / Lên plan / Tin nhắn / Cá nhân; Create là action, rail khi rộng. Ý tưởng thêm loại quan hệ trong cùng context thay vì một navigation thứ hai phù hợp shell hiện tại.
- `CaiDatNhom.tsx:22–24,52` hiện đã biết pair; pair ẩn rename/member/leave, giữ theme. Có chỗ để thêm Loại sổ nhưng không phải UI đã có.
- `KyHoa.tsx:19–24,34–35` xác nhận hình art có `accessibilityLabel` và không mang chữ. Điều này **không tự làm nhãn, buttons, error/recovery của thẻ mới accessible**, cũng không chứng minh screen reader announce đúng lúc khi mở thư.
- `useMotion.ts:34–41` đọc Reduce Motion sống; `motion.ts:60–67` đưa navigator về none và mọi bậc trừ instant về duration 0. Capability nền có; nghi thức mới chưa có.

## 4. Findings cụ thể phải xử lý trước handoff thiết kế

### B1 — P1: Hợp đồng màu §23 không thể đạt với token bị đóng băng

Spec 23.1 (1614–1619) cấm thêm/sửa token; bảng 23.3 (1640–1645) yêu cầu nền giấy trên nền tối >=1.9:1, chữ `inkFaint` trên `paper` >=4.5:1. Tính độc lập bằng công thức sRGB linearized contrast trên **token cùng revision**:

| Cặp | Sáng | Tối |
|---|---:|---:|
| paperShade / paper | 1.3241 | 1.3966 |
| line / paper | 1.3241 | 1.1146 |
| ink / paper | 15.7924 | 10.6733 |
| inkFaint / paper | 5.1306 | **4.3728** |
| paper / ground | 1.1061 | **1.4472** |

`dark.paper = #2e335c` trên **màu nền mà spec tự đưa `#1c1f36`** chỉ **1.3431:1**, không phải >=1.9. Đây là phép tính trên literal trong spec, **không phải đo ảnh native mới**. Thậm chí trên token ground tối hơn cũng chỉ 1.4472. Claim §22.1 dòng 1552 `paper` chỉ 1.06:1 cũng không khớp token revision này; dấu hiệu số cũ còn sót.

Consequence: người triển khai không thể vừa giữ đúng token, đúng surface, vừa pass chính gate đề ra. Viền line hairline cũng chỉ 1.1146 so với paper; dùng nó không sửa được foreground text 4.3728.

Fix: đổi chữ lý do trên giấy đêm sang semantic foreground hiện có mạnh hơn, xác định rõ surface nào được phép và đo lại cơ sở cho threshold tách thẻ; nếu 1.9 là ý đồ bắt buộc thì mở ngoại lệ layer token có phạm vi thay vì yêu cầu bất khả thi. Đừng tùy tiện hạ gate để lấy pass: kèm native specimen để chứng minh lựa chọn mới tách lớp và đọc được.

### B2 — P1: «Một coral trên toàn màn» mâu thuẫn với chat được giữ nguyên

Spec 1593–1608 siết **đúng một coral trên toàn màn**, của thẻ kèo > ôn > mảnh giấy, yêu cầu đếm trên screenshot. Nhưng màn chính §20.1 chính là **chat + ba thẻ**. Source `GroupChatLive.tsx:175–177,438–441,469,529,646` vẫn render sender bubble, quote border, reactions với theme; `tokens.chatTheme.mac-dinh.dark.bubble = #fb693e`, và chat chứa sticker/ảnh người dùng có thể mang coral. `CaiDatNhom.tsx` cho đổi theme. Không thể giữ chat và simultaneously đảm bảo ảnh màn có đúng một coral. Bản thân §19.5 đòi mỗi Nếp vẫn một lớp coral, §19.7 nói góc Nếp là thứ ấm duy nhất; §22.4 lại chuyển quyền coral cho thẻ và giấu góc Nếp: cần quy tắc cuối cùng duy nhất.

Consequence: designer có thể ép sửa chat hoặc vô hiệu hóa nét riêng của Nếp chỉ để qua bộ đếm. Counting pixel không có khái niệm ownership/layer cho ảnh người dùng.

Fix: «một **điểm nhấn hành động chủ đạo trong vùng tính năng**», loại chat, ảnh người dùng, navigation và utility semantic colors khỏi phép đếm; vẫn quyết rõ góc Nếp khi đã có card accent. Validate hierarchy bằng ảnh hai theme chat representative và chat thật synthetic, không dùng screen-wide cardinality như chứng cứ stunning.

### B3 — P1: Nếp ở mở bài và Nếp ở spec anatomy là hai nhân vật

Spec 1.2 dòng 50: «không có mặt, không có mắt, không nói chuyện thành tiếng»; mục 19 dòng 1247–1250 mô tả mắt, mày, miệng, tay chân chính xác với source `nep.ts`. Mục 19.1 còn viết hội bạn «nói khi được gọi», trong khi §16.3 và §17.3 yêu cầu giữ Nếp ở hội im lặng. Nếu «nói» là copy của AI khác với nhân vật, doc chưa phân vai đủ để người dựng màn hiểu giống nhau.

Consequence: story mở đầu đi từ tiền đề sai, có thể dẫn designer đẩy icon/fold trừu tượng ở entry nhưng vẫn dùng mascot nhân hình ở thẻ. Đây là mâu thuẫn tài liệu, không phải yêu cầu thay mascot hiện có.

Fix: viết lại một canonical character paragraph: Nếp có hình nhân hoá, biểu đạt bằng hành động, **không đóng vai người phát ngôn cảm xúc**; phân biệt rõ lời hệ thống, lời do người kia chủ động gửi, và gợi ý của máy. Xoá mô tả 1.2 cũ và lập bảng hành vi của Nếp theo surface dùng chung cho §1/§17/§19.

### B4 — P1/P2: Ba thẻ «ở đầu chat» chưa xác định hành vi trong layout chat thật

Spec §20.1/§22 chỉ cộng **chiều ngang 360dp**. Source `GroupChatLive.tsx:613–621` là **FlatList inverted**: header list nằm ở cuối mới nhất dưới nội dung chat, khác nghĩa «đầu» trong ASCII. Màn hiện có topbar + pills, composer bám keyboard, pending/notice rows; không phải tờ giấy trống 360dp. Spec không xác định ba thẻ fixed ngoài list hay scroll cùng list, khi keyboard mở còn bao nhiêu chat, collapse thế nào, unread/new message scroll ra sao. `chuLon` đổi hai CTA thành hai hàng cộng rest 48 nghĩa riêng vùng actions của kèo đã tối thiểu 144dp trước gaps; cộng ba chặng, lý do, thẻ ôn và thư thì screen chat bị ăn mạnh.

Consequence: có thể code «đúng spec» mà mở chat toàn việc phải làm, không nhìn người kia đang nói gì; một nếp gấp đẹp không cứu được nhịp hội thoại.

Fix: khóa priority slot và quy tắc collapse (một thẻ mở chính, việc còn lại hàng gọn/door), chỉ rõ behavior tại newest/older scroll, incoming message và keyboard; vẽ frame **360 × chiều cao thực** ở 1.0/2.0, cho biết vùng hội thoại còn lại. Không gọi đây là bug native đã tái hiện: nó là lỗ spec được source layout làm lộ.

### B5 — P2: Reduce Motion ghi hai contract khác nhau

Spec §20.4 dòng 1439–1442 vừa nói dùng đường hiện tại `stackAnimation none`, vừa nói khi giảm chuyển động thì **chuyển mờ**. Source `motion.ts:60–67` cho none/duration=0; `useMotion` buộc Reanimated Always. Fade có duration khác 0 sẽ là ngoại lệ mới, không tự được kế thừa từ đường cũ.

Fix: định nghĩa reduced motion bằng trạng thái tĩnh đã gấp + text/status semantic và announce; nếu muốn fade ngắn thì quyết exception rõ, lấy khả năng OS/API và gate native làm bằng chứng. Không xem «opacity» đồng nghĩa «không chuyển động» trong hợp đồng hiện tại.

### B6 — P2: Affordance thư khóa bị rút còn màu và hình, thiếu semantics cho người mới

§22.3 dòng 1584–1588 gọi tam giác coral «toàn bộ affordance», không cần chữ khóa; nhưng §20.1 vẫn có copy «có một mảnh giấy, mở tối nay». Cần khóa luật content: nhãn thời gian và trạng thái phải tồn tại; góc gấp chỉ củng cố. Coral trên thẻ còn có nghĩa ưu tiên hành động §22.4, không thể cùng lúc ngầm bảo «chưa được làm». Không có ảnh để chứng minh người mới hiểu góc gấp nghĩa nào.

Fix: giữ một dòng «Mở tối nay, 20:00» / «Đã tới lúc mở» kèm accessibilityState/label; phân biệt hình dạng + copy giữa riêng, niêm, đã mở và lỗi đọc. Kiểm closed/open/expired-or-revoked cùng nhau. Không thêm nhiều explanatory copy: một nhãn đúng nghĩa thay ba khái niệm ẩn.

## 5. Đề nghị proof trước khi gọi UI đã tốt

Một storyboard **có hình** gồm entry → nửa giấy gửi đi → người kia nhận → chờ → một kèo → thư mở lại (có frame khi trống/nghỉ/không đồng thuận), tất cả cùng character canon, typography và vật liệu; một frame cạnh chat incumbent đủ nội dung để chứng minh hòa vào app. Đừng triển khai 19 bề mặt từ ASCII trước khi so art proof. Native pilot nhỏ chỉ sau khi Lead/ADR cho phép, kiểm 360dp sáng/tối, chữ lớn và reduced motion thật. Gate pixel/art anatomy chỉ là guardrail; người đọc phải hiểu «Nếp đang giữ gì, cho ai, và khi nào được mở» trong frame không có đoạn giải thích bên ngoài.

Các số contrast và contradiction nguồn trên có thể đưa trực tiếp vào debate với planner; không cần giả định user behavior hoặc nghiên cứu bên ngoài để xác nhận chúng.
