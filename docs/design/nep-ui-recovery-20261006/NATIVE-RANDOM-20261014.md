# Nếp chọn giúp — checkpoint trước native, 14/10/2026

Tiếp nối audit FE toàn app. Chủ sản phẩm yêu cầu ImageGen mockup trước khi code Impeccable pipeline, hai mode vòng quay/đua vịt trong hội bạn và `/random [yêu cầu]`. Độ đẹp là cổng riêng. Chỉ kiểm UI trong Android emulator; chưa nghiệm thu tính năng hay toàn app.

## Hướng và asset

- Tham chiếu UI: `docs/design/nep-ui-implementation/redesign-20261013/random/random-reference-v1.png`. Chọn vòng quay trung tâm, làn ao bằng nhau, request/mode/editor gọn và thẻ kết quả trong chat. Đây là critique reference, không approved comp/ảnh native.
- Theo yêu cầu tiếp về entity, tạo character sheet `duck-character-v2.png`, rồi ba cutout ImageGen thật `apps/mobile/assets/rudi/random/duck-{idle,racing,winner}-v2.png`. Vịt chỉ là runner; Nếp canonical giữ nguyên. Study v1 được giữ ở thư mục thiết kế, không ship.
- Sáu PNG có exact prompt và provenance bên cạnh; embed-prompt scan 6/6, không thiếu. Pixel RGBA bản copy khớp bản gốc sau khi nhúng metadata. Ba sprite 1254×1254 RGBA, alpha 0–255. Không dùng Python sửa raster.
- Palette giấy cho tám sector nằm trong `tokens.choiceGame`, qua theme; số và legend giữ danh tính độc lập màu. Mực/giấy đo từ token có contrast thấp nhất 8,887:1; đây là tính toán token, chưa thay kiểm native.
- Brief sáu block: `.impeccable/surfaces/apps-mobile-src-rudi-screens-chat-randompicker-tsx.md`; extension Operate/code-led, không rebrand. Detector HTML/CSS **SKIP** cho native, không gọi clean.

## Hành vi đã viết

ADR-0058 giữ lệnh random riêng trong queue Go, search grounded places tối đa năm, không itinerary/poll/expense. Catalogue chưa có menu; UI nói tìm quán, hiện nguồn thật, không món mẫu hoặc hứa xác minh giá. Verifier/budget fallback không cung cấp ứng viên. Bare `/random` mở editor; tool cùng editor. Lời nhờ trong tool chỉ gửi chính request cho AI.

Editor cho 2–8 lựa chọn, sửa tên bỏ attribution quán, hai mode cùng snapshot. Seed từ native Expo UUID v4, rejection sampling uint32; không fallback Math.random. Gửi MLS cùng round logical id trước mô phỏng; retry giữ round/seed/id. Grammar canonical đóng schema, bind id với logical_send_id, chỉ parse sau MLS. Round không sửa được; xóa theo quyền tác giả. Không sửa ai_card/receipt hoặc Rust ABI. Đây là lượt do người tổ chức tạo, không chứng thực server chống người tổ chức gian lận.

Kết quả lưu thành tin của người tổ chức. Mở quán nguồn, xem lựa chọn, lượt mới và Tạo kèo có đường đi thật; Tạo kèo chỉ điền form (title/place), vẫn cần xác nhận. Pending không lộ JSON kỹ thuật hay giả đã gửi. Owner generation chặn kết quả publish/search cũ qua A→B→A. Cùng mounted history nhường chỗ khi editor mở, composer ẩn; input dùng native focus reveal.

## Kiểm trước checkpoint

- TypeScript toàn app: exit 0, 8,521 giây; compiler peak khoảng 1,08 GiB. Lượt đầu heap 768MiB quá thấp bị V8 abort; giữ log, không phải app/emulator OOM. Native vẫn OFF trong các lượt build/test.
- Test compile: exit 0; targeted Node **75/75**, fail/skip/cancel/todo 0. Gồm grammar/replay/immutability/alias, native entropy/rejection, command/capability và hồi quy poll/AI.
- Go: 30 package companion/aiharness/chatassist có dòng `ok`; lượt gọi đầu overall lỗi do root chỉ sai path `./tools/aigate`. Chạy đúng `./internal/aigate` riêng exit 0. Không gọi lượt sai path là toàn cổng xanh.
- PostgreSQL QC thật, schema riêng: **6/6**, 2 package pass, fail/skip 0. Lượt đầu metrics fixture fail Installed sau migrate; sửa test search_path chỉ schema riêng, không fallback public/cached namespace. Giữ Installed production nguyên. Đọc lại public: metrics/chatassist vẫn version 1–7, không migrate public trong test.
- Review độc lập source/artwork: năm finding đã resolved ở source (readonly replay, sending state, nguồn mở được, AI copy dài, palette). Không có native beauty/SHIP verdict.
- Repo guard staged: 65 file scan trước tài liệu này, pass. Clean-SHA 9 bước, canary/identity/mutants, native và finish review/documenter **chưa chạy cho checkpoint này**.

Raw logs riêng dưới `~/.local/share/rudi-nep-ui/runtime-private-merged-20261006/memory-bounded-checks/random-v1-*`. Giữ cả các lượt fail (heap, ESM fixup chưa chạy, source locator cũ, sai path Go, fixture metrics). Không lấy các lượt đó làm pass hoặc xóa để làm sạch lịch sử.

## Còn mở

Chạy lại 9 bước ở cây sạch đúng SHA; cập nhật graph/source receipt; kiểm native light/dark, 100/200%, IME, hai game/motion/reduced motion, offline/retry/reload/hai thành viên và tìm quán với backend QC mới. Backend QC đang giữ runtime cũ; source mới chưa được coi đã deploy. Sau ảnh cuối cần fresh finish reviewer rồi documenter của Impeccable. Chưa merge main/deploy production; strict32, iOS/hardware/TalkBack/crypto và audit FE A–S toàn app còn mở.

Đợt native b8 trước đã planned OFF. Lần clear poll cuối thất bại do root chạy UIAutomator capture khi driver còn chạy; crash buffer là UIAutomator, không tự gọi app OOM. Giữ evidence đó; các thao tác native sau phải chờ mỗi action/capture hoàn tất trước thao tác tiếp.
