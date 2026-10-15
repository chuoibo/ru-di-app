---
version: 1
slug: "apps-mobile-src-rudi-screens-chat-khaysticker-tsx"
primary_target: "apps/mobile/src/rudi/screens/chat/KhaySticker.tsx"
related_targets: ["apps/mobile/src/rudi/screens/chat/CaiDatNhom.tsx","apps/mobile/src/rudi/screens/chat/SoHen.tsx"]
---

## Direction contract

THESIS: Ba khay con phục vụ một việc rõ ràng: chọn sticker, soạn bình chọn, chỉnh màu chat. Nhãn phải đọc đủ, phần nhập nhận diện tích chính, các lựa chọn ngang phải đều nhau; không dùng artwork che lỗi bố cục.

OWN-WORLD: Giữ hệ giấy/mực/coral và sticker vector hiện hữu. Operate, code-led trên app Android hiện hữu; không rebrand hay concept-seed. Mockup chat-child-sheets-reference-v1 là tham chiếu phản biện, không phải comp đã duyệt. Không cần raster sản phẩm mới.

STORY: Người dùng nhận ra hành động từ tiêu đề, đọc hoặc sửa nội dung, rồi đóng hoặc thực hiện đúng hành động cũ. Giữ nháp, tên a11y, từ vựng sticker, attempts, validation, writer MLS và rời nhóm. Không remount controller/input/Pressable theo cỡ chữ, không thêm native observer khi đã có owner.

FIRST VIEWPORT: Ở320×640, nội dung sticker288dp dùng2 cột tại100/200;4 cột chỉ khi measured width trừ đủ gap đáp ứng artwork và native-text budget, unknown dùng1 cột. Caption không line cap/chiều cao cố định. Poll100/IME đóng cho thấy câu hỏi và cả hai vùng lựa chọn; cap riêng60% cửa sổ thay25%, content-led và shrinkable, minHeight0 để nhường KAV;200/IME mở cuộn fields, giữ close/CTA và vùng focus có ích. Settings5 ô một hàng, trừ4×gap8 trước chia, target≥48dp cả hai chiều, cùng chiều cao và selection không đổi geometry; Text/preview/caption reflow ngay100→200→100 không reopen.

FORM: Seed reference: incumbent code-led refinement, no seed roll. Giữ vector/tokens/primitive hiện hữu. Bỏ border/padding của giấy lồng poll, giữ ONhapMuc ruled fields, input semantics và validation-on-press. Dùng NativeFontMetrics owner có sẵn cho raw text leaf; không observer hay remount controller/input/Pressable. Sticker tính gap10 và budget tối thiểu96dp/tile cùng widest-word budget theo native metrics; reserve hai line-pitch làm minimum, cho chữ tự tăng thêm. Swatch có tick slot bằng nhau và selected ring không tham gia layout; hàng hẹp cuộn ngang giữ48dp. Không lấy disabled-empty, thiếu Aa, phone shell, gradient, boxed fields, poll handle hoặc fullheight settings của board làm yêu cầu.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
