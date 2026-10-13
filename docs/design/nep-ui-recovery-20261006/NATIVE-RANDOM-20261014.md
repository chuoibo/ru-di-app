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

## Checkpoint e5eb: review native có finding cần sửa

`e5eb6c4c` clean9 lượt đầu: năm bước đầu pass, Node **1917/1919**, fail 2 do hai test gọi TypeScript con vượt heap 768MiB của harness; ba bước sau chưa chạy. Native OFF, không phải app OOM. Giữ raw logs; không bỏ test hoặc đổi assertion. Runner private r3 chỉ tăng bước Node subset lên heap 1536MiB/cgroup 2560MiB, vẫn serial và cùng toàn bộ inventory. Chạy lại đủ **9/9**, Node **1919/1919**, Go guide **98 pass**, fail/skip/cancel/todo 0. Root so lại 2240 file/OID và 18 stream. Fixup/Go guide thiếu telemetry cgroup; không gọi null là zero.

Native e5eb dùng đúng JS mới, API b646 còn sống; helper r5 chứng minh `services/core` tree và root Go workspace giống nhau, manifest ghi đúng API SHA/ELF/PID, không đổi nhãn thành SHA UI. Root mở marker và actor mới. Ảnh dark phone 320/390 và expanded 768, chữ 100/200, IME đã mở thật: tên lặp trong chat riêng bỏ, avatar/bong bóng Aaaa cùng tâm ở 100/200, editor không hé history/composer, hai input/caret thấy được với IME. Nhóm QC hiện chỉ thấy ảnh của chính actor, chưa chứng minh peer caption hoặc avatar cạnh tin nhiều dòng. Ảnh đặt tên sai trạng thái được giữ riêng, không tính credit.

Fresh Impeccable finish reviewer trả **fix**, đủ năm section persistence/fidelity/ceiling/material_fixes/keep. Finding: wheel đánh số chiếm viewport trước khi thấy tên lựa chọn, font 200 chỉ còn một phần graphic; tên thắng ở sau hình. Đợt sửa tiếp đưa input trước preview, cửa sổ thấp/chữ lớn mở preview khi cần, tên thắng trước hình khi hoàn tất. Cập nhật direction contract theo finding thật, giữ world/assets/font scaling/48dp. Đây là verdict tại phạm vi batch, không phải Random hoặc toàn app SHIP.

Root tìm thêm payload Random còn nằm trong full preview/accessibility của inbox dù bị cắt trên màn. Sửa format tại metadata tin local đã được so-tin chấp nhận sau replay MLS; không decode raw body thêm, không đổi acceptance/logical ID, round hoặc sổ mã hoá. Thêm bốn ca hành vi: hai mode, replay/own ACK/merge inbox, alias/snapshot sai/quote/edit không promotion, xoá và các loại tin cũ. Kiểm native full Text của preview tại checkpoint kế tiếp còn mở.

Planned stop e5eb: **1043 mẫu**, một PID app ổn định, không safety stop hoặc sampled scope OOM/OOM-kill. Host thấp nhất **15,16GiB**, guest đã boot **626,60MiB**, PSS app lớn nhất **729,98MiB**. Motion in-flight, light, các case đủ dài, replay/hai thành viên và positive AI chưa đủ proof; không suy từ ảnh settled hoặc unit sang các cổng đó.

Ba finding chủ động ở scope kế tiếp: decoded v2 poll/random có thể nhập vào run text; ảnh chưa cap theo cột hẹp; caption ảnh lỗi của tin mình có thể trắng trên nền trắng trong light. Chưa sửa hoặc nghiệm thu các finding này trong batch hierarchy/inbox. A–S/strict32/main và native gates vẫn **OPEN**.

Kiểm trước checkpoint hierarchy/inbox: typecheck và compile pass; focused **57/57**, fail/skip/cancel/todo 0. Lượt focused đầu pass 55/fail 2 vì structuredClone của test bỏ prototype null của map reactions, không phải formatter sửa dữ liệu. Đổi snapshot test sang replay độc lập cùng fixture, giữ full deepEqual/body/identity/seed assertion; log fail được giữ. Source reviewer độc lập không tìm blocker cụ thể ở trust path. Clean9 và native verdict pass sau sửa vẫn phải đo tại SHA kế tiếp.
# Tiếp tục ngày 14/10 — mẫu đua vịt và lịch chung

Chủ sản phẩm bác chất lượng đua vịt cũ và chỉ mẫu Online Stopwatch Duck Race. Root đã xem game thật qua browser tham khảo; không dùng bản web app để audit. ImageGen sinh mockup mới, nền sông và bốn vịt alpha có phụ kiện; root mở từng output và reviewer độc lập phản biện mockup. Chuyển sang sân sông liên tục, đích caro, số đi với vịt, nhãn ngoài hình, preview gập trên màn thấp và kết quả gọn. Brief Impeccable đã cập nhật trước UI code, craft-floor đã đọc; detector HTML/CSS **SKIP native**, không phải clean. Prompt/provenance nhúng 9/9 raster được scan ở thư mục asset Random và hướng mới, 0 thiếu; decoded RGBA giữ nguyên.

ADR-0059 bổ sung snapshot v2/lịch chung, không sửa grammar v1 hoặc protocol frozen. Mốc bắt đầu từ event được lưu, clock đọc DB có ACL room/device/session; client đo RTT hữu hạn, monotonic elapsed, resume/latejoin/fail-closed. Poll 3 giây chưa bảo đảm mọi thành viên thấy đủ countdown. Không frame broadcast, server key, plaintext hoặc tuyên bố công bằng. Chỉ scope client cập nhật trên QC local, chưa bật production; client cũ không được tuyên bố tương thích v2.

Kiểm source ban đầu: TypeScript pass; focused Node **36/36**, fail/skip/cancel/todo 0; HTTP clock unit pass; PostgreSQL thật trên schema tổng hợp tách biệt **20/20 test/subtest**, fail/skip 0; route ownership **225 rows / 218 Go / 73 Go-only**. Giữ các lần đỏ: focused r1 do root gọi Node fixup thiếu PATH, r2 do fixture thay cả Performance làm mất React User Timing; r3 chỉ mock `performance.now`, giữ React thật. PG r1 đỏ hai fixture expiry vi phạm `expires_at>created_at`; r2 dùng created_at cũ và expiry đã qua, không sửa constraint/production. Chưa nhận các số này là exact-SHA clean gate hoặc native.

Hai đường lộ kết quả đã sửa source: projection live vào AI chỉ có nhãn chung; inbox/quote/copy không có trailer/seed/winner. Renderer khởi tạo progress 0, remount theo round id và giữ dưới đích khi chưa được reveal trong khoảng bất định. Chưa có motion/native proof cho các sửa này.

Checkpoint trước `6f70bfcc`: clean gate **9/9**, Node **1923/1923**, Go guide **98**, 0 fail/skip. Native 6f chạy PID 2997 ổn định: **988 mẫu**, host available thấp nhất **13,20 GiB**, guest **657,29 MiB**, app PSS cao nhất **741,17 MiB**, PID đổi **0**, sampled OOM/kill **0**; đã dừng emulator/Metro/watch có kế hoạch trước code, infra giữ nguyên. Root mở ảnh peer Aaaa100/200, inbox readable wheel, draft duck choices trước preview100/200; không gán ảnh chỉ có card thành bằng chứng peer. Chưa có native 6f result/390/tablet hoàn chỉnh hoặc finish verdict sau sửa density. User yêu cầu visual/live mới nên Random vẫn **OPEN**.

Clean checkpoint `a084eaac`: typecheck/guide/export/compile/fixup pass; Node **1931/1932**, fail **1**, skip/cancel/todo **0**. Ca `rudi-khong-hex` bắt bốn màu artwork viết tay trong RandomGame; gate dừng trước ba bước cuối, không seal canonical/native. Sửa đưa palette sân vào `choiceGame.river` trong shared tokens, giữ nguyên kiểm tra; targeted màu **3/3**, typecheck pass. Đồng thời clock animation đi đúng tốc độ raw tới 1; chỉ clamp pose nhìn thấy ở 0,99 trước reveal, tránh nội suy chậm rồi nhảy mỗi 100ms. Chưa đo motion native.

Tiếp theo: exact checkpoint clean checks sau sửa, core QC hỗ trợ clock, native capture/motion/IME/theme/font cho sân mới, hai actor cùng lúc để đo lịch chung, finish reviewer/documenter. Không main merge/push, không SHIP toàn FE; AI positive worker, toàn bộ scene map và ba finding chat trước vẫn mở.

## Checkpoint d91: clean xanh, native bắt lỗi timestamp

`d91d5659` qua **9/9** trên CHECK sạch: Node **1932/1932**, 252 file; Go guide **98**, một package, fail/skip/cancel/todo **0**. Root đo lại 2280 file/OID, 18 raw stream và chín receipt. Peak scope khoảng **1,20 GiB**, host available thấp nhất **18,34 GiB**; số byte chính xác giữ trong receipt riêng. Fixup/Go guide thiếu telemetry, giữ null. Đây vẫn là gate compile/source, không phải native hoặc beauty pass.

Core QC đã đổi sang ELF đúng d91, VCS stamp sạch, SHA256 `c7aca4b5d8932e2b4e314224f89717560dd8718b2a21347a38935d64b7fd9d33`. Clone local có `.git` directory được so 1705 core Git OID và 1140 hash source FE. Lượt đổi binary giữ toàn bộ environment trừ hai khóa owner của chính process; đối chiếu environment thực tế đạt. Không chạy migration/worker, metadata schema giữ nguyên, legacy và các dependency không restart. Đây là QC local, không phải production.

Root mở Android đúng marker/actor d91, dark 320×640, font 100%; nhập bốn lựa chọn tổng hợp Pho/Bun/Com/Pizza, IME thấy field đang gõ. Asset sông/vịt mới hiện đúng sau tải; preview mở ra nhưng nằm dưới fold, phải swipe thêm mới thấy sân. Đây là finding bố cục cần sửa, không phải preview đạt.

Video native 22 giây của lượt v2 chỉ hiện **“Chưa đồng bộ được giờ”**, vịt đứng ở vạch xuất phát. Lượt đã được lưu vào chat; không có motion, kết quả hoặc live hai thành viên đạt. Probe PGX thật chỉ đọc metadata xác nhận event được serialize dạng `2026-10-14T04:47:43.458684+07:00`, còn clock endpoint là UTC `Z`. Parser đang dùng UTC-Z-only cho cả event nên từ chối offset hợp lệ. Sửa tiếp phải giữ nguyên timestamp trong sổ sealed và chỉ đổi phép đọc thành epoch khi tính lịch; clock endpoint vẫn có hợp đồng UTC riêng.

Planned stop d91: **1348 mẫu**, PID app **3007** xuyên suốt, **0** đổi PID/safety stop/sampled scope OOM hoặc OOM-kill. Host available thấp nhất **16,67 GiB**, guest đã boot **692,07 MiB**, app PSS cao nhất **792,39 MiB**. Stop r8 từ chối trước mutation do cwd monitor thực tế là `/home/lakiet`; r9 chỉ đổi đúng expected cwd này, giữ kiểm PID/start ticks/owner/script/scope, rồi dừng riêng emulator/Metro/watch. Infra tiếp tục được xác minh, tài nguyên audit được bảo vệ giữ nguyên. Không suy từ một đợt này rằng toàn app hết OOM.

Finding chủ động còn mở: khối thống kê hồ sơ vẫn thô; lề back chat cần rà lại; preview cần tự đưa sân vào vùng nhìn khi được yêu cầu; sân compact có thể bị unmount đúng lúc kết thúc theo source, chưa có native proof. Toàn bộ gate native/live/beauty/finish, positive AI và A–S/strict32/main vẫn **OPEN**.

Đợt sửa tiếp tách parser event RFC3339 khỏi clock UTC; kiểm offset/ngày lịch và giữ nguyên bytes timestamp sealed. Clock renewal chuyển sang 20 giây, giữ anchor cùng reader còn hạn trong lúc probe, hủy ở hạn gốc nếu renewal lỗi; blur/background đổi thế hệ vẫn vô hiệu hóa response cũ. Preview cuộn theo layout mới đúng một lần khi được mở rõ ràng, hủy khi kéo tay/IME/tạo lượt và tôn trọng reduced motion. Thẻ chat giữ tư thế về đích cho lượt vừa xem, lịch sử cũ vẫn gọn; nội suy native được memo hóa thay vì tạo graph mới mỗi 100ms. Đây là sửa source, chưa phải native AFTER.

Root chạy bounded typecheck/compile/fixup pass; focused năm file **38/38**, fail/skip/cancel/todo **0**. Test mới đi qua pending → ACK → replay → projection với `Z`, `+07:00` và offset âm cùng instant, giữ deadline và timestamp gốc; React lifecycle thật kiểm renewal giữa cuộc đua, lỗi renewal đến hạn gốc và completion sau cleanup. Peak typecheck khoảng **1,09 GiB**, không safety stop; telemetry bước fixup vẫn null. Exact-SHA clean9, ảnh/video AFTER, hai actor và finish reviewer chưa được tính từ các kiểm này.
