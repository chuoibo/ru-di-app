# Spike MLS native của chat Rủ Đi

**Thử nghiệm có mã chạy được, chưa bật trong app hoặc production.** Crate này dùng
OpenMLS 0.9.0 thật; không giả ciphertext bằng random bytes và không tự viết primitive
mã hoá. Chỉ thêm Rust vào `packages/chat-crypto`, không đổi writer Go/Python hoặc
frontend. Quyết định nền: [ADR-0031](../../docs/decisions/ADR-0031-chat-realtime-e2ee-et-regles-go.md).

## Phạm vi đã có

- Tạo nhóm, cấp KeyPackage, thêm hai thiết bị, join bằng Welcome; thêm thành viên
  tiếp theo, xoá thiết bị, tự rekey qua commit MLS. Commit do mình tạo chỉ merge
  sau `acknowledge_commit` khớp chính xác envelope đã gửi.
- Ciphersuite `MLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519`; chỉ nhận MLS
  PrivateMessage. Không có chế độ plaintext. Không bật `test-utils`,
  `content-debug`, `crypto-debug` hoặc các feature draft của OpenMLS.
- Mã hoá operation phiên bản 1: text, reaction, delete, vote. Payload có kiểu,
  giới hạn kích thước, từ chối trường/type/version lạ. Đây là codec; quyền xoá
  tin của tác giả, hội tụ reaction/poll và reducer client còn là tầng riêng.
- Kiểm roster đã được bên gọi xác minh: account/device, khoá ký MLS, khoá ký
  transport; kiểm KeyPackage và từng leaf. BasicCredential tự nó không chứng minh
  danh tính account. Không cho thay khoá của một device hiện hữu trong rekey.
- Chống replay bằng ratchet MLS, cho phép tối đa 32 thế hệ đến đảo thứ tự trong
  epoch hiện tại. Không giữ secret epoch cũ (`max_past_epochs(0)`). Tin trì hoãn
  qua rekey bị từ chối; bài toán phân phối commit và drain trước rekey chưa nối.
- Outbox trong state giữ đúng ciphertext theo logical ID; retry không mã hoá lại,
  đổi payload với cùng ID bị từ chối. Pending commit sống qua local restart.
- Ràng buộc conversation/device/logical ID/protocol/epoch trong MLS authenticated
  data và chữ ký Ed25519 outer. Hai khoá ký MLS/transport tách riêng.
- Snapshot bộ nhớ giao dịch nhận: commit/Welcome bị từ chối vì roster, identity
  hoặc context không làm mất ratchet/key package của lần nhận hợp lệ tiếp theo.
  Đây là cơ chế prototype; không thay thế transaction của kho thiết bị thật.

## Wire với Go v2

`Envelope` dùng đúng tên JSON và base64 `[]byte` của
`services/core/internal/chatv2/types.go`. Chữ ký dùng preimage length-prefix của
`SigningBytes`, không ký JSON. `check_go_interop.sh` tạo ciphertext MLS thật, rồi
biên dịch **file Go đang checkout** cùng driver ngoài repo để so byte preimage,
verify chữ ký và từ chối sửa epoch. Không sửa source Go để chạy phép đối chiếu.

MLS bắt đầu epoch 0, Go yêu cầu epoch >= 1: spike ánh xạ `Go epoch = MLS epoch + 1`.
Commit gửi dưới epoch trước đó; application tiếp theo dùng epoch mới. **Chưa có
enrollment/rekey API phối hợp ánh xạ này với epoch, membership incarnation,
`ready` và revoke barrier trên Go.** Không được gửi thẳng fixture vào production
rồi coi một chữ ký đúng là E2EE tích hợp xong. Không có private key đi sang Go.

## Local restart khác khôi phục backup

`seal_local_state` mã hoá toàn bộ MemoryStorage, khoá transport và outbox bằng
XChaCha20-Poly1305 với nonce ngẫu nhiên. Bên gọi cung cấp wrapping key riêng cho
thiết bị; crate không ghi file, không log secret và không xuất private key qua API.
Buffer serialized plaintext và bản sao key được zeroize; không hứa đã chứng minh
toàn bộ allocator/upstream/OS không bao giờ giữ bản sao secret.

`resume_local_state` chỉ dùng để mở lại process trên **cùng thiết bị**, với
`LocalAnchor` hiện hành lấy độc lập từ kho thiết bị, gồm device, generation và
digest blob. Blob cũ với anchor mới, sai khoá, sai thiết bị hoặc bị sửa đều bị
từ chối. Anchor được tính lại từ blob do người nhập đưa vào không phải bằng chứng
chống rollback. Hai bản process cùng mở một identity cũng chưa bị platform khoá.

**Chưa có Keychain/Keystore, kho transaction chống rollback, khoá độc quyền process,
backup exclusion, hoặc atomic persist state + outbox + ACK.** App phải bảo đảm
những điều này trước mọi network I/O/ACK. Checkpoint API không phải tính năng
backup; bản sao checkpoint và wrapping key không được đưa vào cloud/OS backup.

`recover_as_new_device` tạo device ID và hai cặp khoá mới, từ chối ID cũ và không
nhận checkpoint/ratchet cũ. Thiết bị recovery chưa vào nhóm và không giải được tin
cũ; cần enrollment mới rồi re-add. Import lịch sử riêng có consent từ thiết bị
tin cậy hoặc backup recovery-code chưa triển khai.

## Giới hạn để review

- Một Client chỉ giữ một group; API Rust thuần, chưa có ABI/JNI/Swift/Expo bridge.
  Không expose symbol C để gọi tuỳ ý và không bỏ `forbid(unsafe_code)` để giả native
  integration. Expo Go không nạp module native tuỳ ý này.
- Tối đa 128 logical sends còn giữ trong outbox, kể cả commit đã ACK; đầy thì
  `Capacity`. Chưa có garbage collection/durable idempotency tombstone. Checkpoint
  tối đa 8 MiB; KeyPackage generation của client chưa join bị giới hạn. Roster
  enforce 100 account, 5 device/account, 500 leaf, nhưng canary mới chạy 2–4 peer,
  **chưa đo nhóm 500 leaf**.
- Snapshot giao dịch nhận sao chép memory state: thuận tiện cho spike an toàn
  khi reject, chưa đo latency/RAM trên điện thoại. Storage hiện dùng provider
  MemoryStorage của OpenMLS; chưa phải kho native bền vững.
- Không tự quyết quyền admin thêm/xoá: `verified_next_roster` là assertion từ tầng
  enrollment/authorization đáng tin của caller. Enrollment, transparency,
  cross-signing, mất khoá, revocation offline phối hợp server và race commit còn
  cần thiết kế/kiểm chứng độc lập. Peer bị xoá không giải epoch sau trong canary;
  không thể hứa thu hồi plaintext họ đã có.
- Chưa có attachment/voice key protocol, encrypted local search/history, UI reducer,
  push, background wake, hay sealed AI invocation. Không có tuyên bố crypto audit,
  500-leaf load, iOS build, thiết bị Android/iPhone thật hoặc E2EE end-to-end app.

## Chạy lại

Toolchain được pin ở `rust-toolchain.toml`; dependency graph pin trong Cargo.lock.
Tất cả artifact phải nằm ngoài worktree. Ví dụ với Rust đã cài riêng:

```bash
export CARGO_TARGET_DIR=/tmp/rudi-chat-crypto-target
export CARGO_BUILD_JOBS=2
nice -n 19 cargo +1.98.1 test --locked --manifest-path packages/chat-crypto/Cargo.toml
nice -n 19 cargo +1.98.1 clippy --locked --manifest-path packages/chat-crypto/Cargo.toml --all-targets -- -D warnings
bash packages/chat-crypto/scripts/check_go_interop.sh
rustup target add aarch64-linux-android --toolchain 1.98.1
export ANDROID_NDK_ROOT=/path/to/android/ndk
bash packages/chat-crypto/scripts/check_android.sh
```

Android script cross-compile ARM64 bằng NDK clang API 26, cả static/shared library
và executable canary để linker thật phải giải các dependency. `readelf` xác minh
AArch64 ELF. **Cross-compile không phải chạy canary trên Android.** Linux host
canary không thay thế iOS/native UI hoặc review crypto độc lập.

Khi đã được phép dùng một emulator x86_64 đang có, có thể chạy binary Rust trực
tiếp trên Bionic bằng `check_android_emulator.sh`. Script chỉ tạo một thư mục
riêng dưới `/data/local/tmp/rudi-chat-crypto.*`, push hai test executable, chạy
canary tuần tự rồi xoá đúng thư mục đó. Không cài APK, không điều khiển UI, không
restart server ADB. Cần target `x86_64-linux-android` cùng các biến
`CHAT_ADB_PORT`, `CHAT_ADB_SERIAL`, `ADB_BIN`, `ANDROID_NDK_ROOT`, `CARGO_TARGET_DIR`.
Điều này chứng minh Rust chạy trên runtime Android emulator; chưa phải app native
hay điện thoại thật.

## Nguồn chính thức

- [OpenMLS 0.9.0 và MSRV 1.91](https://book.openmls.tech/releases/0.9.0.html).
- [OpenMLS API, ciphersuites, native target chỉ build chứ không test](https://latest.openmls.tech/doc/openmls/index.html).
- [Application messages và xoá sender key sau encrypt](https://book.openmls.tech/user_manual/application_messages.html).
- [MLS RFC 9420](https://www.rfc-editor.org/rfc/rfc9420).

Khoá phiên bản và chứng cứ build không phải xác nhận không có lỗ hổng trong
dependency. Release vẫn cần review crypto độc lập, dependency advisory audit,
native integration và các cổng ADR-0031.
