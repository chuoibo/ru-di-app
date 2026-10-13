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

Checkpoint `229497dd` đã chạy clean9: typecheck pass, bước guide fail do map/fingerprint sinh từ source chưa cập nhật; bảy bước sau chưa chạy. Sinh lại đúng bằng `tools/rut-huong-dan.mjs`: 62 routes, fingerprint `5c1123cf5893`, chỉ thêm nhãn nav cho đường quán nguồn. Giữ raw fail và graph 229 riêng (1120 source/43 harness/2240 raw comparisons, 0 mismatch), không gọi đó là clean9 pass. Checkpoint kế tiếp chạy lại trên SHA mới.

Checkpoint `84255c18`: năm bước đầu pass, Node 1911/1915 (fail 4, skip/cancel/todo 0), ba bước cuối chưa chạy. Ba failure là guard cũ chưa biết lệnh thứ năm và nhánh request-only; một failure là hai nút chưa có lý do disabled. Sửa rõ nhánh mới, giữ các assertion quyền/cặp/context; thêm copy lý do vào hai nút, không bỏ test hoặc nới regex thành catch-all. Graph 842: 1120 source/43 harness, 2240 raw comparisons, 0 mismatch; giữ làm packet lịch sử, không dùng cho source mới.

Chạy lại 9 bước ở cây sạch đúng SHA; cập nhật graph/source receipt; kiểm native light/dark, 100/200%, IME, hai game/motion/reduced motion, offline/retry/reload/hai thành viên và tìm quán với backend QC mới. Backend QC đang giữ runtime cũ; source mới chưa được coi đã deploy. Sau ảnh cuối cần fresh finish reviewer rồi documenter của Impeccable. Chưa merge main/deploy production; strict32, iOS/hardware/TalkBack/crypto và audit FE A–S toàn app còn mở.

Đợt native b8 trước đã planned OFF. Lần clear poll cuối thất bại do root chạy UIAutomator capture khi driver còn chạy; crash buffer là UIAutomator, không tự gọi app OOM. Giữ evidence đó; các thao tác native sau phải chờ mỗi action/capture hoàn tất trước thao tác tiếp.


## Lượt tiếp 14/10: nguồn catalogue và producer tách worker

Checkpoint `1ad95c9abe90f47b610cc5cc3989984ffa618389` đã qua đủ **9/9 bước** tại cây CHECK sạch: Node **1915/1915**, 250 file, fail/skip/cancel/todo 0; Go guide **98 pass**, 1 package, fail/skip 0. Root đối chiếu lại 2240 file thực/Git OID, 18 raw stream, receipt từng bước và ELF. Peak scope toàn lượt khoảng 1,15GiB, host available thấp nhất 18,04GiB; số byte chính xác giữ trong receipt riêng. Bước fixup thiếu telemetry cgroup; không suy diễn null thành zero OOM. Canonical packet riêng `native-audit-1ad95c9a` giữ nguyên, chưa có ảnh native của checkpoint này.

Runtime preflight phát hiện ELF của clean9 không có VCS stamp. Go 1.26.8 đang dùng chỉ nhận `.git` directory, không nhận file `.git` của linked worktree; bật `-buildvcs=true` ở CHECK vẫn ra cùng ELF. Root dựng clone local riêng có `.git` directory, đúng commit/tree, so lại 1120 file với graph và build hữu hạn. ELF bổ sung có `vcs.revision=1ad95c9a…`, `vcs.modified=false`, SHA256 `8eb167d44efee4c54ca0a49217dc888d1c0d339031b11feee3ed581dbc12dc5f`; giữ nguyên ELF/receipt clean9 gốc. Inspection đạt; execute sau đó từ chối **trước signal** vì Python venv không có pidfd API. Core cũ PID/start ticks vẫn giữ nguyên. Helper r3 tách riêng pidfd TERM qua system Python đã được đọc và review source, chưa execute. Không restart Docker, không dừng tài nguyên audit được bảo vệ.

Sửa frontend sau review: `placeId` dùng khóa text opaque của catalogue, không bắt UUID; giữ nguyên NFD/khoảng trắng. Thêm giới hạn và mã hóa route nội bộ, lọc nguồn không hỗ trợ trước render. Shared parser chỉ giữ nguyên ID, vẫn trim tên hiển thị. Review phát hiện Expo Router giải mã percent escape hai lần; hợp đồng Random hiện từ chối `%HH` và dot segment, không tuyên bố đã sửa router dùng chung. ADR-0058 ghi rõ giới hạn này. Không đổi asset hoặc bố cục.

Sửa Go sau review: producer `serve` tách worker nhận lệnh Random bằng provider và port catalogue chỉ đọc, không dựng full execution engine/embedding hay tự chạy job. Giữ nhánh inproc và worker như trước. Test HTTP thật có Gemini, AGY-only không có Gemini, keyless; producer phải giữ invocation queued/attempts 0, không lease/result/card/metrics hoặc model call. Capability biểu thị nhận hàng đợi, **không chứng minh worker sống hay đã tìm được quán**. Cấu hình QC hiện chưa có worker và thiếu dependency embedding của full worker; positive AI delivery vẫn **OPEN**, chưa đổi provider hay nạp key để che lỗi.

Kiểm root cho các sửa tiếp: typecheck pass; test compile pass; Node focused **27/27**, fail/skip/cancel/todo 0; unit Go hai package pass; PostgreSQL **21 pass**, 3 package pass, fail/skip 0. PG lượt r1 fail 11/pass 8 vì fixture cmd/core thiếu clone uploaded_images và còn fallback public. Sửa fixture search_path chỉ schema riêng, thêm clone dependency đó; không đổi migration/Installed production. Sau lượt fail, root so snapshot metadata public với trước: mọi version/checksum của tám family, table OID và hai constraint khớp hoàn toàn. Đây là bằng chứng metadata, không phải audit mọi dữ liệu DB. Log r1 và r2 đều được giữ.

Các sửa sau `1ad95c9a` cần checkpoint mới và clean9 mới; không dùng pass của parent thay thế. Emulator/Metro/watch vẫn OFF trong lượt build/test này. Native độ đẹp, chuyển động, gửi/replay, font/IME và Impeccable finish review/documenter của Random chưa hoàn tất; A–S/strict32/main và các cổng native độc lập vẫn OPEN.

Checkpoint `317bb116`: năm bước đầu clean9 pass; Node **1918/1919**, một failure của guard hiển thị ID nhận nhầm ternary đọc khóa catalogue; fail 1, skip/cancel/todo 0, ba bước sau chưa chạy. Tách hàm đọc khóa opaque khỏi hàm xử lý chữ hiển thị, giữ nguyên guard và regression bảo toàn ID; không thêm ngoại lệ scanner. Graph 317 riêng đo đủ 1120 source/43 harness, 2240 raw comparisons, 0 mismatch; không dùng làm graph/pass của checkpoint sau. Native vẫn OFF.

## Native b646 và đợt sửa bố cục chat

Checkpoint `b6462736` qua **9/9** trên CHECK sạch: Node **1919/1919**, 250 file; Go guide **98 pass**, một package; fail/skip/cancel/todo 0. Root so lại 2240 file/OID và 18 raw stream. ELF có VCS đúng b646 được dựng riêng từ clone local sạch như đã mô tả; không thay receipt ELF gốc của clean9.

QC local đã áp dụng migration chat/metrics v8 và chạy core mới. Lượt launcher đầu không ghi được owner receipt dù core mới sống; giữ nguyên failure, không suy ra nguyên nhân hoặc equality environment cũ. Root và reviewer đọc helper adoption, kiểm PID/start ticks/ELF/cổng/health/config chọn lọc, metadata v8/OID, legacy và dependencies, rồi chỉ cập nhật receipt đúng core đang chạy. Không chạy lại migration hoặc restart Docker. Full worker AGY-only vẫn thiếu embedding; tìm quán AI thực tế **OPEN**.

Root đã mở Android emulator ở 320×640, dark, chữ 100/200%, xác nhận marker b646 và actor fixture. Nhập hai lựa chọn tổng hợp Pho/Bun bằng driver giới hạn dữ liệu; đua vịt gửi thành công qua MLS, hiện kết quả Bun trong editor và thẻ chat. Vịt là cutout ImageGen thật và thấy rõ khi thu nhỏ. Chưa có ảnh in-flight, chưa kiểm vòng quay, replay/reload/hai thành viên, tám lựa chọn, reduced motion hoặc light; không dùng lượt này nghiệm thu toàn tính năng.

Ảnh cho thấy ba lỗi: phần tìm quán và chú thích chiếm chỗ; lịch sử inverted còn lộ trên editor ở cỡ chữ lớn; tên/avatar tin nhắn lệch thị giác đúng phản ánh tiếp của chủ sản phẩm. Đợt sửa kế tiếp gộp tìm quán thành mục mở khi cần, bỏ chú thích tự nhập và đường kẻ lặp, đặt CTA trong chân màn, kéo đầu nội dung khi lượt được xác nhận. Chat riêng bỏ tên trùng header; nhóm dùng tên dịu và căn theo chữ trong bong bóng. Plain text/pending giữ target tối thiểu 48dp và avatar đi theo chiều cao dòng hiện tại. Boundary mounted ẩn cả lịch sử khi editor mở; không unmount crypto/list hooks. Asset/round/queue/Rust ABI không đổi trong đợt bố cục này.

Trước sửa, planned stop native có **803 mẫu**, PID app ổn định, không đổi PID, không safety stop hoặc sampled scope OOM/OOM-kill. Host available thấp nhất **15,68GiB**, guest đã boot thấp nhất **680,26MiB**, PSS app lớn nhất **750,29MiB**. Đây là đợt khoảng 13 phút, không chứng minh không còn OOM ở mọi flow. Root đợi từng action/input/capture kết thúc trước bước tiếp.

Typecheck/compile và focused regression **51/51**, fail/skip/cancel/todo 0, của đợt bố cục đã pass tại cây làm việc; clean9 và ảnh sau sửa phải đo lại tại checkpoint mới. Native finish review/documenter, A–S, strict32, main và các cổng native độc lập vẫn **OPEN**.
