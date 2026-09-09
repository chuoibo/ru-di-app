# Cổng motion hữu hạn — audit native 09/09 §5

Audit nói đúng: source có nền tảng motion tốt (`useMotion.ts` đọc Reduce Motion, `ReduceMotion.System`,
viewer phân trang) nhưng **chưa có số đo** nào để ký «mượt» hay «không jank». Cổng này chỉ làm việc audit
đề nghị ở bước 1–2: **đo** trên build release, lặp lượt Reduce Motion. Không thêm animation (bước 3 của
audit là đề xuất, quyết định sau khi có số).

## Cái gì được đo, bằng gì

- **Build**: `assembleRelease` x86_64, Hermes, bundle nhúng với env fixture (`EXPO_PUBLIC_RUDI_FIXTURE=1`,
  API trỏ cổng không phục vụ) — không Metro, không dev client. Cùng `applicationId` nên đè dev client trên máy
  ảo; sau khi đo cài lại `app-debug.apk`.
- **Máy**: `emulator-5554`, Android 15, 1080×2400 @420dpi, host WSL2 20 GB RAM. **Máy ảo không phải điện
  thoại**: số ở đây là **đường cơ sở hồi quy của rig này**, không phải phán quyết «mượt 60/120 Hz».
- **Chuỗi thao tác** (`apps/mobile/.maestro-motion/`): `m1` đổi bốn tab ×5 · `m2` cuộn Khám phá xuống/lên ×4
  · `m3` mở/đóng sheet «Tạo mới» ×5 · `m4` vào chi tiết địa điểm rồi Back ×5. Mỗi chuỗi: `force-stop` → mở
  qua flow vào app → `dumpsys gfxinfo <pkg> reset` → chạy chuỗi → `dumpsys gfxinfo <pkg>` → lấy tổng khung,
  % janky, p50/p90/p95/p99, số khung UI chậm / vẽ chậm / đóng băng (`do-motion.sh`).
- **Reduce Motion**: cùng bốn chuỗi với ba `*_animation_scale` = 0 (đúng thứ «Remove animations» của Android
  đặt, và đúng cờ `AccessibilityInfo.isReduceMotionEnabled` đọc), rồi trả về 1.

## Kết quả

### Tầng 1 — dev client + bundle dev, fixture (đo xong)

Cửa fixture đòi `__DEV__ && EXPO_PUBLIC_RUDI_FIXTURE=1` **theo thiết kế** (`cua-fixture.ts`: bản store không bao giờ
có cửa ấy), nên build release **không vào được** bản trải nghiệm — lượt release đầu đỏ ở màn «Chào bạn» nhập
số. Tầng này đo trên dev client với bundle dev từ Metro: JS chưa minify, có dev check, nên là **giới hạn trên**
của jank — release ít nhất tốt bằng.

Lượt thường (`animator_duration_scale` = 1):

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | slow UI | slow draw | missed vsync | maestro |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 804 | 12.69% | 26ms | 36ms | 61ms | 150ms | 54 | 87 | 16 | rc=0 |
| m2-cuon-kham-pha | 1482 | 6.34% | 16ms | 32ms | 36ms | 117ms | 30 | 78 | 17 | rc=0 |
| m3-sheet-tao | 933 | 11.79% | 26ms | 36ms | 53ms | 150ms | 32 | 97 | 16 | rc=0 |
| m4-back-chi-tiet | 812 | 12.81% | 28ms | 36ms | 65ms | 150ms | 47 | 87 | 18 | rc=0 |

Lượt Reduce Motion (ba `*_animation_scale` = 0):

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | slow UI | slow draw | missed vsync | maestro |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 226 | 43.36% | 30ms | 89ms | 129ms | 150ms | 44 | 83 | 17 | rc=0 |
| m2-cuon-kham-pha | 900 | 8.78% | 16ms | 30ms | 38ms | 150ms | 28 | 70 | 15 | rc=0 |
| m3-sheet-tao | 315 | 24.76% | 19ms | 65ms | 117ms | 150ms | 28 | 60 | 12 | rc=0 |
| m4-back-chi-tiet | 454 | 17.84% | 28ms | 40ms | 77ms | 150ms | 37 | 68 | 10 | rc=0 |

Đọc số: **% janky không so được giữa hai lượt** — tắt animation làm số khung vẽ giảm 2–4 lần (đổi tab 804 →
226) vì các khung chuyển cảnh không còn, phần còn lại là khung mount/layout nặng do JS, nên tỉ lệ tăng dù số
khung UI chậm tuyệt đối tương đương (54 → 44). p99 = 150ms ở mọi hàng là **trần bucket** của histogram gfxinfo,
không phải giá trị đo. Mọi chuỗi `rc=0`: bốn chuỗi đã thật sự chạy, trạng thái cuối chụp ở `dev-client/*/motion-m*-cuoi.png`.

### Tầng 2 — release + stack live (đăng nhập OTP)

**Đã thử hai lần, chưa có số hợp lệ — dừng ở một quyết định không phải của tôi.**

1. Release + env fixture: đỏ ở màn «Chào bạn» — cửa fixture đòi `__DEV__` theo thiết kế
   (`release-thu/release-fixture-khong-co-cua.png`). Đúng, không sửa.
2. Release + stack live (`e2e_slice --keep`, cổng 46903, `adb reverse`, `healthz` xanh từ host): bấm «Gửi mã»
   → app báo **«Không nối được máy chủ. Kiểm tra mạng rồi thử lại.»** (`release-thu/release-live-chan-o-gui-ma.png`
   — tiện thể là câu F43 mới chạy trên bản release). Nguyên nhân: bản release **chặn HTTP cleartext** (Android
   9+); chỉ `android/app/src/debug/AndroidManifest.xml` có `usesCleartextTraffic="true"`, manifest main không.
   Stack loopback là `http://`, nên release không nối được. Bảng của lượt này (`bang-khong-hop-le-rc1.md`) mọi
   chuỗi `rc=1` — **không dùng**: ~130 khung chỉ là mở app + bước đăng nhập.

Muốn đo tầng 2 cần một trong hai: (a) bật `usesCleartextTraffic` **tạm, cục bộ, không commit** cho đúng bản đo
— thay đổi tư thế bảo mật của bản release dù chỉ trên máy đo, bộ phân loại của phiên này chặn, và tôi không vòng
qua; (b) đưa stack đo sau một HTTPS local (mkcert + reverse proxy) — tốn thêm một lượt dựng. Đây là **quyết
định của team**, không phải chỗ tôi tự làm. Chuỗi đo đã sẵn (`.maestro-motion-live/`, `do-motion.sh` với
`FLOWS_DIR`/`OTP_PHONE`), chạy được ngay khi có một trong hai.

## Cái này KHÔNG chứng minh

- Cảm giác chạm trên điện thoại thật; 120 Hz; iOS; haptic.
- Khung hình **giữa** chuyển cảnh stack khi Reduce Motion bật: chưa chụp được xác định — chỉ có trạng thái
  cuối mỗi chuỗi (ảnh `motion-m*-cuoi.png`) và số khung; `_layout.tsx` khai `slide_from_right`/`fade` cho
  native-stack, hệ điều hành xử lý scale 0 cho Fragment transition — kết luận ấy vẫn là đọc mã, đúng như
  audit cảnh báo.
- Live send/error/retry, TalkBack, tablet.

## Chạy lại

```bash
cd apps/mobile/android && ANDROID_HOME=~/Android/Sdk EXPO_PUBLIC_RUDI_FIXTURE=1 \
  EXPO_PUBLIC_API_URL=http://127.0.0.1:18999 ./gradlew :app:assembleRelease -PreactNativeArchitectures=x86_64
ANDROID_ADB_SERVER_PORT=5038 adb -s emulator-5554 install -r app/build/outputs/apk/release/app-release.apk
docs/claude/2026-09-10/motion/do-motion.sh <ra>            # lượt thường
docs/claude/2026-09-10/motion/do-motion.sh <ra-reduce> reduce
ANDROID_ADB_SERVER_PORT=5038 adb -s emulator-5554 install -r app/build/outputs/apk/debug/app-debug.apk  # trả dev client
```
