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
