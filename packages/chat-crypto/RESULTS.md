# Bàn giao spike MLS — 2026-09-21

**Đã có thư viện Rust chạy thật và canary trên Android emulator. Chưa có E2EE
tích hợp trong app, chưa qua review crypto độc lập, chưa được enable production.**
Phạm vi và hợp đồng chi tiết ở [README](README.md).

## Kết quả trên source cuối

| Phép kiểm | Kết quả | Phạm vi chứng minh |
|---|---|---|
| Rust host, `cargo test --locked` | 21/21 PASS, 0 ignored | 4 ca attacker có khoá thật và 17 ca lifecycle OpenMLS |
| `cargo fmt --check` | PASS | Định dạng source |
| `cargo clippy --all-targets -- -D warnings` | PASS | Lint, không thay review crypto |
| Go wire interoperability | PASS | Preimage byte-for-byte với `types.go` đang checkout, Ed25519 verify và reject sửa epoch |
| Android ARM64 release | PASS | Link static/shared library và executable thật với NDK clang API26; ELF AArch64 |
| Android x86_64/Bionic, emulator API35 | 21/21 PASS, 0 ignored | Hai executable Rust chạy trên runtime Android; không phải APK/Expo/native UI |
| Dependency feature inspection | PASS | Không bật OpenMLS test-utils, debug secret hoặc draft feature |
| 197 Cargo registry archives | SHA256 khớp Cargo.lock | Tính nhất quán archive tải từ crates.io; không phải advisory audit |
| Repo guard, checkpoint | Root đã review/pin hai false positive `vn-phone` trong Cargo.lock | Chỉ đúng path + digest + rule; không thay crypto audit |

Canary bao gồm: ba peer gửi text/reaction/delete/vote; peer thứ tư join; retry cùng
logical ID giữ nguyên ciphertext; đổi payload bị conflict; tin đảo thứ tự và
replay; rekey chờ ACK; bỏ epoch cũ; xoá thiết bị online/offline; commit/Welcome
bị từ chối không tiêu state hợp lệ; thay khoá/roster; sửa outer metadata dù ký
lại bằng khoá thiết bị thật; giả MLS sender; TLS bị cắt/thêm bytes; payload sai
version; local restart giữ replay/outbox/pending commit; checkpoint cũ/sai khoá/
sai device/tamper; recovery tạo identity mới; account vượt 5 device; outbox đầy
từ chối gửi mới mà giữ retry cũ. Không có mock MLS hoặc plaintext fallback.

Host canary cuối mất 0,16 + 0,48 giây; Android release canary cuối 0,01 + 0,09
giây. Đây là thời gian một lượt suite với build profile khác nhau, **không phải
benchmark latency, smoothness, high traffic hoặc bằng chứng SLO**.

## Định danh source và môi trường

Fingerprint source/code/test/build scripts (không gồm README/RESULTS):

```text
a2b56e299a3a60e8e2d62c98bf2e44b4c8a3ab5d4ec0714565babd4978221d7f
```

Tạo lại bằng:

```bash
cd packages/chat-crypto
sha256sum Cargo.toml Cargo.lock rust-toolchain.toml src/*.rs tests/*.rs examples/*.rs scripts/*.sh | sha256sum
```

Cargo.lock SHA256:

```text
2f92268375658bdf5dd53cde29536fee58affce0f6983ce8b3175c725cacd4dd
```

Hai cảnh báo lockfile nằm trong checksum công khai của `autocfg 1.5.1` và
`regex-syntax 0.8.11`. Không sửa lockfile hoặc nới scanner để làm xanh. Root đã
kiểm độc lập 197 registry archive SHA256 khớp lock, rồi pin duy nhất `vn-phone`
theo digest trên, theo `docs/security/repo-guard.md`. Phép kiểm này chỉ xác nhận
false positive của lockfile, không phải review tính đúng đắn crypto.

Môi trường: Rust 1.98.1, OpenMLS 0.9.0, các provider 0.6.0; Linux x86_64 host;
<!-- repo-guard: allow=long-number reason=android-ndk-version -->
Android NDK 27.1.12297006, clang API26; emulator đang có `emulator-5554`, ABI
x86_64, API35, ADB port5038. Chỉ tạo thư mục mktemp riêng dưới
`/data/local/tmp/rudi-chat-crypto.*`, push/chạy hai executable rồi trap xoá.
Không cài APK, thay UI, tạo emulator hoặc restart ADB server. Không kiểm iPhone,
Android ARM thật, iOS build, JNI, Expo hoặc khoá OS.

## Tiếp tục trong session hiện tại

Toolchain cài riêng ngoài repo, không sửa toolchain hệ thống:

```bash
export CARGO_HOME=/tmp/rudi-chat-rust.t5HYgz/cargo
export RUSTUP_HOME=/tmp/rudi-chat-rust.t5HYgz/rustup
export CARGO_TARGET_DIR=/tmp/rudi-chat-rust.t5HYgz/target
export CARGO_BUILD_JOBS=2
export CARGO_BIN=/tmp/rudi-chat-rust.t5HYgz/cargo/bin/cargo
export RUST_TOOLCHAIN_ARG=+stable
# repo-guard: allow=long-number reason=android-ndk-sdk-path
export ANDROID_NDK_ROOT=/home/lakiet/Android/Sdk/ndk/27.1.12297006
export ADB_BIN=/home/lakiet/Android/Sdk/platform-tools/adb
export CHAT_ADB_PORT=5038
export CHAT_ADB_SERIAL=emulator-5554
nice -n 19 "$CARGO_BIN" +stable test --locked --manifest-path packages/chat-crypto/Cargo.toml
nice -n 19 "$CARGO_BIN" +stable clippy --locked --manifest-path packages/chat-crypto/Cargo.toml --all-targets -- -D warnings
bash packages/chat-crypto/scripts/check_go_interop.sh
bash packages/chat-crypto/scripts/check_android.sh
bash packages/chat-crypto/scripts/check_android_emulator.sh
```

Alias `stable` trong thư mục tạm này hiện chính là 1.98.1; script mặc định dùng
pin `+1.98.1` khi không truyền biến override. Các target ARM64 và x86_64 Android
đã cài. Giữ `CARGO_BUILD_JOBS=2` và `nice -n 19` khi kiểm tra cùng test tải Go.
Session cuối `56170` đã exit0; không còn cargo/adb task của spike chạy nền.

Raw log ngoài worktree:

- `/tmp/rudi-chat-crypto-tests.log`
- `/tmp/rudi-chat-crypto-clippy.log`
- `/tmp/rudi-chat-crypto-interop.log`
- `/tmp/rudi-chat-crypto-android.log`
- `/tmp/rudi-chat-crypto-android-runtime.log`
- `/tmp/rudi-chat-crypto-features.log`

Không commit trong shared tree; thay đổi spike chỉ ở `packages/chat-crypto`.
Không đưa artifact binary, checkpoint, khoá hoặc dữ liệu người thật vào repo.

## Những việc còn thiếu

1. Review độc lập source/crypto và dependency advisory audit; review rồi pin
   Cargo.lock nếu đồng ý. Tác giả spike không tự đóng cổng review.
2. Enrollment/xác minh identity, roster authority, liên kết membership incarnation,
   epoch/ready/revoke barrier với Go. Go wire test chưa phải roundtrip dịch vụ có
   enrollment/rekey thật; khoá riêng không được đi vào Go.
3. Kho native có transaction và antirollback, Keychain/Keystore, process exclusivity,
   backup exclusion, persist state + outbox + ACK nguyên tử; outbox GC, commit
   races, reconnect qua nhiều epoch. Anchor demo không tự tạo bảo đảm của OS.
4. JNI/Swift/Expo development build, iPhone/Android thật, background lifecycle,
   memory/crash/recovery tests. Khôi phục backup phải device identity mới; không
   import ratchet checkpoint cũ.
5. 500 leaf thực, media/voice key protocol, reducer E2EE, sealed AI output, thiết bị
   bị mất, kiểm malicious server, native performance và review crypto độc lập
   trước enable production.

Các mục này không được thay bằng canary xanh của spike hoặc số tải Go transport.
