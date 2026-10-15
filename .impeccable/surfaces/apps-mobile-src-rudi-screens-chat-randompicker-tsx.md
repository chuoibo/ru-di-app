---
version: 1
slug: "apps-mobile-src-rudi-screens-chat-randompicker-tsx"
primary_target: "apps/mobile/src/rudi/screens/chat/RandomPicker.tsx"
related_targets: ["apps/mobile/src/rudi/screens/chat/RandomGame.tsx","apps/mobile/src/rudi/screens/chat/ChatRandomCard.tsx","apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx","apps/mobile/src/rudi/screens/chat/SoHen.tsx","apps/mobile/src/rudi/screens/chat/usePollFocusReveal.ts","apps/mobile/app/outings/new.tsx","apps/mobile/src/rudi/screens/keo/CreateOutingLive.tsx"]
---

## Direction contract

THESIS: Nếp giúp hội bạn tìm lựa chọn có nguồn rồi cùng chơi một lượt chọn ngẫu nhiên. Vòng quay hoặc ao đua là nội dung chính; kết quả trở lại chat thành một vật lưu được.

OWN-WORLD: Extension Operate/code-led của giấy, mực, Bricolage và coral/navy hiện hữu. duck-race-reference-v2 là critique reference, không phải approved comp. Nền sông và bốn vịt có phụ kiện là raster thực; số, nhãn, đồng hồ, đích và nút là UI native. Nếp giữ nhân dạng riêng; không sao chép artwork trang tham khảo.

STORY: Tool hoặc /random mở cùng editor. Tìm quán có nguồn hoặc tự nhập món; sửa danh sách, chọn mode, khóa lượt, gửi MLS rồi mô phỏng. Retry/replay giữ seed; lượt mới là thao tác riêng. Thiếu nguồn không sinh ứng viên mẫu.

FIRST VIEWPORT: Gutter 16dp, heading/close gọn, mode rồi danh sách sửa; tìm quán và preview mở khi cần. Khi đua: pha/đồng hồ nhỏ, một mặt sông rộng hết vùng nội dung, 2–8 vịt đánh số, tên đầy đủ ngoài hình. Khi xong: tên thắng trước sân thu gọn và hành động thật. 320dp/200% cho tên xuống dòng, không cố nhét hình; CTA chân màn ẩn khi IME mở; không lịch sử/composer phía sau.

FORM: Seed reference: inherited code-led extension, user-pinned continuous river race, no seed roll. Bỏ hàng tiến trình; đường chạy ẩn, số đi cùng vịt, vạch đích caro. Nhịp tăng/giảm và vượt nhau tái hiện từ seed, không chọn lại kết quả. V1 xem lại cục bộ; v2 chỉ diễn đúng pha đồng hồ đã đồng bộ, reduced motion giữ cùng hạn lộ kết quả. Không badge người đang xem, đồng thuận hay chứng nhận công bằng.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Ghi chú triển khai

Bấm mở preview phải đưa sân vừa mở vào vùng đọc theo layout và chiều cao scroll thực; sân cao hơn viewport giữ đầu sân để người dùng cuộn tiếp. Chỉ cuộn một lần từ thao tác mở rõ ràng, tôn trọng reduced motion; hủy khi kéo tay, mở IME hoặc tạo lượt. Tick live không kéo vị trí đọc. Native d91 đã thấy sân chỉ lộ một khe sau thao tác mở ở 320×640/font1; sửa source chưa phải bằng chứng native AFTER.

Thẻ đang mount ở pha đếm ngược hoặc đua giữ sân thu gọn qua lúc về đích của cùng lượt; lịch sử mở muộn vẫn là tóm tắt, có thể mở chi tiết. Interpolation node được giữ qua các tick cùng lượt. Cần native AFTER để kiểm tư thế thắng, vị trí đọc và độ mượt.

Sân sông (`duong-dua.ts`) tính pitch dọc hữu hạn cho cả full stage và compact stage: ở 4 lựa chọn, full stage cao 216dp (pitch ~43dp) và compact stage cao 176dp (pitch ~34,7dp); ở 8 lựa chọn, full stage cao 306dp (pitch ~36,9dp, sprite 36dp) và compact stage cao 256dp (pitch ~31,1dp, sprite 28dp); khi > 6 làn dùng badge số thu gọn 18×18dp để không chồng lấp ở xuất phát hoặc vạch đích caro. `RandomGame.tsx` preload nền sông và các sprite vịt đang dùng bằng `Image.prefetch` với timeout hữu hạn 450ms trước khi lộ lớp nghệ thuật của sân, tránh hiện nền phẳng hoặc thiếu vịt ở nhịp đếm ngược đầu tiên mà không đổi lịch chung v2. `ChatRandomCard.tsx` bọc sân đua trong `stageWrap` có đệm ngang 16dp khớp trục lề trái của trạng thái `Đang đua · Xs` và ẩn dòng tóm tắt lặp khi sân thu gọn đang hiển thị trực tiếp. Đã kiểm chứng native AFTER trên Android emulator ở 320×640, 390×844 và 768×1024 (tối/sáng, chữ 100%/200%, 4 và 8 lựa chọn, IME mở).

