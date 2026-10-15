---
version: 1
slug: "apps-mobile-src-rudi-screens-chat-sohen-tsx"
primary_target: "apps/mobile/src/rudi/screens/chat/SoHen.tsx"
related_targets: ["apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx","apps/mobile/src/rudi/screens/chat/usePollFocusReveal.ts"]
---

## Direction contract

THESIS: Bình chọn là một editor trong cuộc chat. Trường đang viết phải nhận diện tích có ích khi bàn phím mở; một heading, một vùng cuộn và một hành động đủ rõ, không giữ một mẩu lịch sử phía trên để đổi lấy ô nhập bị cắt.

OWN-WORLD: Operate, code-led refinement của giấy/mực/coral/navy hiện hữu. poll-ime-reference-v1 là tham chiếu phản biện, không phải comp được duyệt. Bỏ boxed fields, gradient và tỷ lệ màn cao của ảnh gen; không thêm raster trang trí hay rebrand.

STORY: Mở bình chọn, nhập câu hỏi và các lựa chọn, đọc lỗi tại ô rồi gửi hoặc đóng. Giữ cùng controller, draft, refs, selection, validators và writer. Focus/IME/font không tự gửi hay tạo attempt; lịch sử được trả về đúng chỗ đọc sau khi editor khép.

FIRST VIEWPORT: 320×640/100/noIME hiện cả ba vùng nhập ban đầu, không dải chỉ hé avatar/bubble bị cắt. Khi poll mở, cùng mounted history nhường cột cho editor; native viewport của reader giữ chiều cao dương đã đo, callbacks ẩn không đổi chỗ đọc. Body khi IME mở chứa trọn input focus≥48dp, label đi cùng khi đủ chỗ. 100→200→100 giữ focus và đưa chính ô đó vào viewport native sau reflow. Header danh tính/bảo mật và một send target≥48dp vẫn đọc được; helper/lỗi tổng quát cuộn cùng form.

FORM: Seed reference: incumbent code-led refinement, no seed roll. Giữ ONhapMuc ruled lines và tokens. Không lấy window/keyboard bottom thay viewport của body. Async measurement phải từ chối target/viewport/generation cũ; manual drag không bị kéo lại. Thu lịch sử bằng layout trên cùng instance, bảo vệ reader anchor/read-mark/load khi viewport ẩn. Ngoài IME giữ natural height, không giant empty form.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
