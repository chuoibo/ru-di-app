# Chat native 21-09: bằng chứng có giới hạn

**Chưa đạt cổng native production.** Source/hook đã kiểm; màn chat mới đã hiện trên
Android emulator qua dev client cũ và server fixture riêng. Không phải E2E với
backend PostgreSQL/Go, không phải bằng chứng E2EE, voice, iOS hoặc hiệu suất.

## Môi trường và nguồn

- Checkout `/home/lakiet/mobile`; source thuộc checkpoint mobile trên nhánh
  `codex/p0-w28-chat-go-e2ee` (xem lịch sử Git của tài liệu này).
  Metro `8117` ghi `Starting project at /home/lakiet/mobile/apps/mobile`, lần
  cuối `Android Bundled ... (2403 modules)`; `EXPO_NO_DOTENV=1`.
- API fixture `127.0.0.1:20177`, toàn account/tên/tin do harness tạo. Đăng nhập
  OTP qua UI thật tới fixture; đây không kiểm chứng SMS hoặc auth backend thật.
  Không gọi API đang chạy ở `8099`.
- `emulator-5554`, ADB `5038`, có lane Maestro khác đang dùng. Đã dừng thao tác
  sau khi phát hiện, không dừng process của lane đó. Bằng chứng chat dưới đây
  thuộc **emulator-5560**, AVD `rudi-qa3` mở `-read-only -no-snapshot-save` riêng.
- Emulator Android 15, x86_64, 1080×2400, font scale 1.0, light. Cài APK local
  `android/app/build/outputs/apk/debug/app-debug.apk`, artifact
  ngày 12-09; package `com.lakiet.rudi`, version 1.0.0/code 1. Hash
  package.json/app.json khớp fingerprint local cũ, **không đủ chứng minh** native
  binary khớp lockfile và toàn bộ generated native hiện tại.
- Ảnh/XML/log/harness đặt ngoài repo: `/tmp/rudi-chat-native-20260921/`.
  `fixture-server.mjs` là harness HTTP trả dữ liệu giả, không phải backend mới.
  Bản tái chạy lưu tại `apps/mobile/tools/chat-native-fixture.mjs`; chạy bằng
  `CHAT_NATIVE_EVIDENCE_DIR=/tmp/rudi-chat-native-probe node apps/mobile/tools/chat-native-fixture.mjs`.
  Tool bind localhost:20177; không dùng tài khoản/nội dung thật. Bản trong `/tmp`
  cùng manifest giữ nguyên script chính xác của lượt chụp; bản repo chỉ thay
  cấu hình thư mục output để tái chạy, giữ response/fixture như lượt này.

## Đã quan sát trực tiếp trên emulator-5560

| Ca | Quan sát | Artifact |
|---|---|---|
| Mở chat đã sửa | Header `QA Chat 21-09`, nhãn chưa E2EE, bubble gom tác giả, composer hiện | `22-chat-warm.png`, `.xml` |
| Tin tới lúc đọc lịch sử | Scroll lên, fixture thêm một tin; poll đã nhận tin nhưng read-mark vẫn `qa-050`; nút về tin mới xuất hiện | `23-reading-history.png`, `.xml`, `fixture-events.jsonl` |
| Quay về cuối | Bấm nút, read-mark tiến tới `qa-51` | `fixture-events.jsonl` |
| Send thất bại khi đang soạn tin kế | Fixture giữ request 7 giây rồi trả 503; hàng `first-native-message` giữ lỗi/Thử lại/Bỏ, composer vẫn giữ `newer-native-draft` | `24-send-failed-draft.png`, `.xml` |

Log fixture không ghi bearer hoặc dữ liệu thật. Chưa chạy đủ ca long-text lỗi,
retry thành công, keyboard + slash, dark/font lớn trên **APK mới dựng**; review
native tổng thể vẫn cần bổ sung các trạng thái đó. Chưa đo FPS/ANR, tải nhiều
người, TalkBack, tablet hoặc iPhone.

## Hai blocker riêng

1. **Crash native cold launch:** thấy SIGSEGV trên cả emulator dùng chung lẫn
   emulator riêng, thread `mqt_v_js`, stack `MountingCoordinator::pullTransaction`
   / `FabricUIManagerBinding` trong `libreactnative.so`. Warm reconnect mở được
   chat. Chưa xác định nguồn gốc hoặc phân lập baseline/source mới; không kết
   luận lỗi do UI chat, và không bỏ qua crash vì một lượt warm render thành công.
   Log: `first-launch-native-crash.log`, `isolated-cold-launch-crash.log`.
2. **Chưa dựng APK hiện tại:** Gradle lần đầu thiếu `ANDROID_HOME`; lần sau đã
   trỏ SDK thật nhưng JRE 21 không có `javac`, dừng ở capability `JAVA_COMPILER`.
   Log: `gradle-rebuild.log`, `gradle-rebuild-sdk.log`. Cần JDK 21 đầy đủ, build
   lại, cài riêng vào emulator/thiết bị thử nghiệm rồi kiểm cold launch và các
   trạng thái ở trên. Không dùng test xanh để gỡ blocker này.

## Kiểm source

TypeScript và web export đã qua. Bộ hook/queue/transport mục tiêu 32/32 qua;
3 regression mới chạy **ViewabilityHelper của React Native đã cài**, dùng config
production `CHAT_VIEWABILITY`, qua: tin 1200px trong viewport 500px, tin ngắn,
mép 1px và dwell 600ms. Negative control với item threshold cũ tái hiện tin dài
không bao giờ được đánh dấu đọc. Đây là bằng chứng thuật toán/callback; native
read-mark quan sát ở bảng trên bổ sung một tình huống, không thay bộ E2E.

Review source độc lập đã chạy lại typecheck và 50 test chat (gồm ba ca helper
mới), APPROVE đúng phạm vi source/hook. Review visual độc lập yêu cầu chụp bổ
sung keyboard + slash, long-text lỗi/retry và dark/font lớn trên APK mới;
chưa có APPROVE cho toàn bộ UI native hoặc độ mượt.

Harness lặp lại tại `tools/chat-native-fixture.mjs`. Bản lưu trong Git đã sửa
cursor của `/control/new` bằng cùng ID tin (harness tạm ban đầu tăng nhầm một
lần) và sửa member count về hai. Log/ảnh cũ vẫn giữ nguyên nguồn, không dùng
sửa fixture để diễn giải lại kết quả đã chụp. Chạy fixture chỉ trên loopback,
ghi artifact vào thư mục tạm ngoài repo bằng `CHAT_NATIVE_EVIDENCE_DIR`.
