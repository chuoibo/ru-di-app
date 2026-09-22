# Chat native 21-09: bằng chứng có giới hạn

**Chưa đạt cổng native production.** Source/hook đã kiểm; màn chat mới đã hiện trên
Android emulator qua dev client mới dựng lại và server fixture riêng. Không phải E2E với
backend PostgreSQL/Go, không phải bằng chứng E2EE, voice, iOS hoặc hiệu suất.

## Môi trường và nguồn

- Checkout `/home/lakiet/mobile`; source thuộc checkpoint mobile trên nhánh
  `codex/p0-w28-chat-go-e2ee`, commit `ad350d41`. Lượt build/chụp bắt đầu
  tại `c0a50319`; autosquash metadata sau đó đổi ID commit, giữ nguyên byte source
  mobile. Ref `refs/codex/chat-before-guard-pin` giữ provenance trước rewrite.
  Metro `8117` ghi `Starting project at /home/lakiet/mobile/apps/mobile`, lần
  cuối `Android Bundled ... (2403 modules)`; `EXPO_NO_DOTENV=1`.
- API fixture `127.0.0.1:20177`, toàn account/tên/tin do harness tạo. Đăng nhập
  OTP qua UI thật tới fixture; đây không kiểm chứng SMS hoặc auth backend thật.
  Không gọi API đang chạy ở `8099`.
- `emulator-5554`, ADB `5038`, có lane Maestro khác đang dùng. Đã dừng thao tác
  sau khi phát hiện, không dừng process của lane đó. Bằng chứng chat dưới đây
  thuộc **emulator-5560**, AVD `rudi-qa3` mở `-read-only -no-snapshot-save` riêng.
- Emulator Android 15, x86_64, 1080×2400. Lượt đầu dùng APK 12-09; lượt bổ
  sung dùng APK **dựng lại ngày 21-09**, package `com.lakiet.rudi`, version
  1.0.0/code 1. Lượt mới gồm light/font 1.0 và dark/font 1.3; APK chỉ có ABI
  x86_64, chưa dùng cho điện thoại ARM hoặc iPhone.
- Build với JDK Ubuntu 21.0.12 có compiler, SHA256 package khớp metadata apt,
  runtime cùng phiên bản copy độc lập ngoài repo; không đổi Java hệ thống.
  `JAVA_HOME` và `ANDROID_HOME` chỉ cấp cho lệnh build, không prebuild/reset cây.
  `:app:assembleDebug -PreactNativeArchitectures=x86_64` thành công trong 30 giây,
  539 task/37 thực thi. Log `gradle-rebuild-jdk.log`, APK và digest ở manifest
  trong thư mục bằng chứng; fingerprint package/app cũ không được dùng thay build.
- Ảnh/XML/log/harness gốc đặt ngoài repo: `/tmp/rudi-chat-native-20260921/`.
  Năm ảnh đã kiểm và manifest được lưu lâu dài trong
  [packet ảnh native](../../../docs/codex/2026-09-21/chat-native/README.md).
  `fixture-server.mjs` là harness HTTP trả dữ liệu giả, không phải backend mới.
  Bản tái chạy lưu tại `apps/mobile/tools/chat-native-fixture.mjs`; chạy bằng
  `CHAT_NATIVE_EVIDENCE_DIR=/tmp/rudi-chat-native-probe node apps/mobile/tools/chat-native-fixture.mjs`.
  Tool bind localhost:20177; không dùng tài khoản/nội dung thật. Bản trong `/tmp`
  cùng manifest giữ nguyên script chính xác của lượt chụp; bản repo có cấu hình
  output để tái chạy và các sửa fixture được ghi rõ ở cuối tài liệu.

## Đã quan sát trực tiếp trên emulator-5560

| Ca | Quan sát | Artifact |
|---|---|---|
| Mở chat đã sửa | Header `QA Chat 21-09`, nhãn chưa E2EE, bubble gom tác giả, composer hiện | `22-chat-warm.png`, `.xml` |
| Tin tới lúc đọc lịch sử | Scroll lên, fixture thêm một tin; poll đã nhận tin nhưng read-mark vẫn `qa-050`; nút về tin mới xuất hiện | `23-reading-history.png`, `.xml`, `fixture-events.jsonl` |
| Quay về cuối | Bấm nút, read-mark tiến tới `qa-51` | `fixture-events.jsonl` |
| Send thất bại khi đang soạn tin kế | Fixture giữ request 7 giây rồi trả 503; hàng `first-native-message` giữ lỗi/Thử lại/Bỏ, composer vẫn giữ `newer-native-draft` | `24-send-failed-draft.png`, `.xml` |

## Bổ sung trên APK mới dựng

| Ca | Quan sát | Artifact |
|---|---|---|
| Keyboard + slash | Bàn phím mở; khay lệnh có scroll, composer và tin cuối cùng cùng hiện | `31-rebuilt-keyboard-slash.png`, `.xml` |
| Tin dài gặp lỗi mạng | Nội dung, lỗi, Thử lại/Bỏ và bản nháp kế tiếp đều còn trên màn | `32-rebuilt-long-failed.png`, `.xml` |
| Tin dài lỗi 503 ở dark/font 1.3 | Fixture trả lỗi có kiểm soát; dòng lỗi và hai nút hiện; giữ `keep-new-draft` | `32b-rebuilt-dark-failure.png`, `.xml` |
| Retry thành công | Gửi lại trả tin server; bubble chuyển sang màu của người gửi, mất hàng lỗi/nút retry; vẫn giữ `keep-new-draft` | `33-rebuilt-retry-success.png`, `.xml`, `fixture-retry-events.jsonl` |
| Dark/font 1.3 | Header nhóm, các bubble nhiều dòng và composer hiện | `34-rebuilt-dark-font13.png`, `.xml` |

XML của lượt retry được kiểm: không còn nút Thử lại; có nội dung tin dài đã gửi;
composer vẫn mang đúng `keep-new-draft`. Đây là native UI + HTTP fixture;
không suy ra idempotency backend thật hoặc E2EE đã đạt.

Hai ảnh chụp nhầm trạng thái được giữ với hậu tố `-invalid`: retry khi fixture
đã chết vẫn lỗi, và đổi font/theme đưa app về Explore. Không dùng chúng như
bằng chứng thành công. Sau đó khởi động lại fixture riêng (log pha hai), mở lại
route chat, kiểm response và trạng thái thật rồi mới chụp ảnh thay thế ở bảng.
Log fixture không ghi bearer hoặc dữ liệu thật. Chưa đo FPS/ANR, tải nhiều
người, TalkBack, font 2.0, tablet hoặc iPhone.

## Runtime còn phải theo dõi

APK 12-09 từng SIGSEGV trên cả emulator dùng chung và emulator riêng, thread
`mqt_v_js`, stack `MountingCoordinator::pullTransaction` / `FabricUIManagerBinding`
trong `libreactnative.so`; warm reconnect mở được chat. Log
`first-launch-native-crash.log`, `isolated-cold-launch-crash.log` giữ nguyên.

APK mới dựng có một lượt cold-launch đã render được trước loạt ảnh bổ sung.
Hai probe khác chỉ thấy process còn sống sau sáu giây, không chứng minh đã render.
Không kết
luận đã sửa crash hoặc xác định nguyên nhân: chưa phân lập root cause; những
lượt khởi động hữu hạn không chứng minh độ ổn định production. Thiếu compiler
đã được giải quyết bằng JDK cục bộ; lỗi build cũ được giữ trong
`gradle-rebuild.log`, `gradle-rebuild-sdk.log`, không còn là blocker build hiện tại.

## Kiểm source

TypeScript và web export đã qua. Bộ hook/queue/transport mục tiêu 32/32 qua;
3 regression mới chạy **ViewabilityHelper của React Native đã cài**, dùng config
production `CHAT_VIEWABILITY`, qua: tin 1200px trong viewport 500px, tin ngắn,
mép 1px và dwell 600ms. Negative control với item threshold cũ tái hiện tin dài
không bao giờ được đánh dấu đọc. Đây là bằng chứng thuật toán/callback; native
read-mark quan sát ở bảng trên bổ sung một tình huống, không thay bộ E2E.

Review source độc lập đã chạy lại typecheck và 50 test chat (gồm ba ca helper
mới), APPROVE đúng phạm vi source/hook. Reviewer Impeccable độc lập trả **ship
chỉ cho UI legacy trong năm trạng thái đã chụp**: keyboard/slash, tin dài lỗi
mạng, lỗi 503 dark/font 1.3, retry thành công giữ nháp, chat dark/font 1.3.
Verdict này không xác nhận toàn app, MLS/E2EE, bot thật, reduced motion,
hiệu suất, độ ổn định native hoặc production readiness.

Harness lặp lại tại `tools/chat-native-fixture.mjs`. Bản lưu trong Git đã sửa
cursor của `/control/new` bằng cùng ID tin (harness tạm ban đầu tăng nhầm một
lần), dùng UUID tổng hợp có chữ hex và sửa member count về hai. Log/ảnh cũ vẫn giữ nguyên nguồn, không dùng
sửa fixture để diễn giải lại kết quả đã chụp. Chạy fixture chỉ trên loopback,
ghi artifact vào thư mục tạm ngoài repo bằng `CHAT_NATIVE_EVIDENCE_DIR`.

Đã dừng emulator riêng `5560`, Metro `8117` và fixture `20177`; xác nhận hai
port không còn listener. Emulator dùng chung `5554`, API `8099` và Metro
`8095` không bị dừng. JDK, APK, log và ảnh chụp sai trạng thái vẫn ở ngoài repo.
