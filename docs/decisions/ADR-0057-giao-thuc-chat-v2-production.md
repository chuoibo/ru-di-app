# ADR-0057 — Giao thức chat v2 E2EE bản production: thiết bị, KeyPackage, commit, đa thiết bị, media, push

- Ngày: 2026-10-05. Phụ lục của ADR-0031 (không sửa ADR-0031).
- Trạng thái: **Đề xuất** — chủ sản phẩm chốt 05/10 «E2EE gộp vào đợt này, làm dứt điểm, có push; TestFlight chỉ sau
  khi E2EE xong». Bật production vẫn cần review crypto độc lập APPROVE (ADR-0031 §4); văn bản này không thay chữ ký đó.
- Trước đợt này (`173f1d43`): Rust core MLS một thiết bị một nhóm, ops Text/Reaction/Delete/Vote, 21 canary; FFI 7
  symbol; Go `chatv2` chỉ chạy trong `chat-lab`; không module native, không push.
- Đã dựng trong nhánh `audit-0510-g3-e2ee` (xem mục 10): §1–§3, §4.4, §5 (trừ `receipt`/`typing`), §7 phía server,
  §8. **Chưa làm**: §4.2 chuyển lịch sử, §4.3 mã khôi phục, §1.3(b)(c) mã an toàn và xác nhận thiết bị mới, §6 AI
  trong phòng v2, push phía app, iOS chưa biên dịch.

## 1. Danh tính và thiết bị

1. **Một danh tính dài hạn mỗi thiết bị** (không mỗi nhóm): cặp khoá Ed25519 ký envelope (đã có) + credential MLS
   `BasicCredential(identity = device_id)` với khoá ký MLS riêng. Một `Client` Rust giữ nhiều nhóm dưới cùng danh tính.
2. **Ghi danh** (`POST /v2/chat/devices`): bearer của phiên đăng nhập + khoá công khai (transport, MLS) + chữ ký của
   khoá transport trên `"RUDI-CHAT-DEVICE\0v1\0" ‖ len‖person ‖ len‖device ‖ mls_pub` (chứng minh giữ khoá
   transport; khoá MLS được chứng minh bằng chữ ký trên mỗi KeyPackage). Idempotent với đúng bộ khoá cũ. Server lưu, không bao giờ thấy khoá bí mật.
   Tối đa 5 thiết bị còn hiệu lực mỗi tài khoản.
3. **Mô hình tin cậy, nói thẳng**: server là bên chứng thực "thiết bị X thuộc người P". Server độc hại có thể ghi danh
   thiết bị ma. Bù lại: (a) mọi thành viên thấy danh sách thiết bị của nhau và mỗi lần một thiết bị mới vào nhóm là một
   sự kiện hiển thị trong phòng; (b) "mã an toàn" (fingerprint 60 chữ số trên tập khoá thiết bị) so được ngoài kênh;
   (c) thiết bị đã tin cậy của cùng người phải xác nhận thiết bị mới (QR/mã 6 chữ) trước khi nó nhận lịch sử.
   Key transparency đầy đủ ngoài phạm vi đợt này — ghi rõ ở trang bảo mật.
4. **Thu hồi** (`DELETE /v2/chat/devices/{id}`, đăng xuất, xoá tài khoản): thiết bị bị đánh dấu `revoked_at`, mọi phòng
   chứa nó chuyển `ready=false` (rào thu hồi): không gửi tin thường được cho tới khi một commit gỡ leaf đó được chấp nhận.
   Xoá tài khoản xoá luôn hàng thiết bị, KeyPackage, mailbox Welcome và token push (`nepnho/dangky.go` hết `Chua`).

## 2. KeyPackage và Welcome

1. Mỗi thiết bị giữ 8 KeyPackage dùng-một-lần đã đăng (trần 20 chưa claim trên server, 64 đang lưu hành trong Rust);
   `POST /v2/chat/key-packages` bổ sung khi còn < 3. Không có last-resort.
2. `POST …/claim` (thành viên đã ở trong phòng, cho thiết bị roster đang chờ) **giữ chỗ** một KP cho người claim:
   không bao giờ cấp lại cho ai khác, commit thêm thiết bị đó tiêu nó, quá 1 giờ không dùng thì **đốt** (vá rà soát bảo
   mật 05/10: claim lặp đi lặp lại để rút cạn KP của người khác không còn tác dụng).
3. **Welcome** đi vào hộp thư theo thiết bị (`chat_v2_welcomes`), ciphertext mờ với server, xoá khi thiết bị ACK.

## 3. Sự kiện, epoch và commit

1. Ba loại sự kiện trong log của phòng: `application` (tin), `commit`, `welcome_ref`. Tất cả nằm trong cùng một
   sequence tăng dần của phòng (đã có `last_sequence`).
2. **Rào epoch**: một commit mang `epoch_from`; server nhận khi và chỉ khi `conversations.epoch = epoch_from`
   (CAS trong tx), rồi `epoch := epoch_from + 1`. Mỗi epoch đúng một commit thắng; bên thua nhận 409 `epoch_moved` kèm
   sequence để bắt kịp, rồi tự dựng lại commit. Tin `application` mang epoch hiện hành; tin ở epoch cũ bị 409.
   Go epoch = MLS epoch + 1 (giữ nguyên quy ước hiện có).
3. **Roster do server chứng thực**: tập thành viên của phòng = membership đang hoạt động của nhóm × thiết bị còn hiệu
   lực, kèm `incarnation` (tăng mỗi lần đổi). Server ghi `roster_digest` của epoch mới vào sự kiện commit; client chỉ
   nhận commit khi tập leaf sau commit khớp roster server công bố cho incarnation đó (`receive(envelope,
   verified_next_roster)` đã có). Commit thêm/gỡ người không có trong roster server → client từ chối, phòng không tiến.
4. `ready=true` chỉ khi tập leaf của epoch hiện hành khớp roster hiện hành; mọi đổi membership/thiết bị/block hạ `ready`
   (trigger đã có) và thiết bị trực tuyến đầu tiên thấy chênh lệch sẽ đề xuất commit sửa. **Gửi tin và đánh dấu cần
   `ready`; đọc thì không** — thiết bị phải đọc được chính commit đưa roster về khớp. Quyền đọc vẫn đòi membership còn
   hiệu lực, thiết bị chưa thu hồi, tài khoản còn, không bị chặn, và `first_sequence`.
5. **Server là thẩm quyền về roster**: commit chỉ được **thêm** thiết bị server đang chờ (thành viên đang hoạt động ×
   thiết bị còn hiệu lực đã ghi danh MLS) và chỉ được **gỡ** thiết bị server **không còn** chờ (rời nhóm, bị thu hồi,
   tài khoản xoá). Không ai đẩy được thiết bị của một thành viên hợp lệ ra. Thiết bị được thêm đọc từ sau commit thêm
   nó (`first_sequence`), nhận Welcome kèm roster để kiểm.
6. Khởi tạo phòng: thiết bị đầu tiên của thành viên đầu tiên trực tuyến `create_group` rồi commit thêm mọi thiết bị
   trong roster; cho tới khi `ready`, UI nói "đang thiết lập mã hoá" và không gửi được.

## 4. Đa thiết bị, khôi phục, lịch sử

1. ≤ 5 thiết bị; mỗi thiết bị là một leaf. Thành viên mới chỉ đọc từ lúc vào (ADR-0031 §5).
2. **Chuyển lịch sử** thiết bị → thiết bị của cùng người: gói lịch sử đã giải mã, mã hoá bằng khoá thoả thuận qua QR
   (X25519 + HKDF), gửi qua mailbox như Welcome. Server chỉ thấy blob mờ.
3. **Sao lưu tuỳ chọn bằng mã khôi phục**: mã 24 ký tự (Crockford base32, 120 bit), Argon2id → khoá bọc, snapshot lịch
   sử (không phải ratchet) được mã hoá và tải lên dạng blob mờ. Khôi phục tạo **danh tính thiết bị mới** (không hồi sinh
   ratchet cũ) và đọc lại lịch sử từ snapshot.
4. Trạng thái cục bộ: `seal_local_state` sau mỗi thay đổi, trước khi trả lời JS; khoá bọc 32 byte giữ dưới khoá
   Android Keystore / trong iOS Keychain (`AfterFirstUnlockThisDeviceOnly`), loại khỏi backup (`noBackupFilesDir`,
   `isExcludedFromBackup`); neo `{current, next}` chống rollback; ghi file tạm + fsync + rename; một luồng tuần tự.
5. **Sổ phòng trên máy** (`RoomLog`, Kotlin + Swift, cùng định dạng): MLS xoá khoá sau mỗi lần dùng, nên tin đã giải
   mã chỉ còn nếu được ghi lại. Log nối đuôi các khung AES-GCM dưới khoá HMAC-SHA256(khoá bọc, "rudi-chat-room-log-v1"),
   AAD `người|thiết bị|phòng|chỉ số khung`, fsync mỗi khung, khung rách cuối bị bỏ, nén mỗi 256 khung. Một khung chứa
   các bản ghi **và** con trỏ đi qua chúng: không gì tiến con trỏ mà không ghi những gì nó đi qua.
6. **Phát lại sau crash**: app có thể chết giữa lúc Rust đã niêm trạng thái sau `receive` và lúc sổ phòng được ghi.
   Mỗi phòng trong Rust giữ ≤ 128 kết quả gần nhất theo digest(envelope ‖ roster được kiểm); đúng đầu vào đó lặp lại thì
   trả kết quả cũ thay vì lỗi "khoá đã dùng" (phòng sẽ kẹt mãi). App gọi `settle_received` sau khi sổ ghi xong; sau đó
   phát lại bị MLS từ chối như cũ. Roster nằm trong digest: commit cũ kèm roster khác không được trả từ sổ.
7. Envelope ứng dụng không mở được (giả, hỏng) bị bỏ qua và **đếm, hiện ra** trong phòng — một máy xấu không làm kẹt
   phòng của mọi người. Commit không kiểm được vẫn dừng phòng (fail closed).
8. Tin của chính thiết bị được ghi vào sổ **trước** khi gửi (`cho`), xác nhận bằng bản sao trên làn (`da-gui`), lỗi
   thì thành hàng "chưa gửi được" có Thử lại/Bỏ; mở lại app thì tự gửi lại cái chưa xác nhận (cùng logical id). Bỏ trùng
   theo (người gửi, logical id) — logical id do người gửi chọn và lộ trên envelope.

## 5. Nội dung

1. Ops v2: `text`, `reaction`, `delete`, `vote`, **`reply`**, **`edit`** (chỉ tác giả, giữ lịch sử sửa cục bộ),
   **`image`**, **`sticker`**, **`voice`**, **`receipt`** (đã đọc, theo watermark), **`typing`** (không lưu log).
2. **Media**: khoá XChaCha20-Poly1305 ngẫu nhiên mỗi tệp, AAD = media id ‖ mime, mã hoá trên máy sau khi nén lại (không
   EXIF); ciphertext tải lên `PUT /v2/chat/media/{phòng}/{media}` (trần 300 tệp/ngày/thiết bị, 2 GiB, khoá advisory
   theo thiết bị); `{media_id, key, sha256, mime, kích thước}` nằm trong tin MLS. Server không bao giờ có khoá.
3. Outbox: GC khi ACK; tombstone cho xoá; trần 128 giữ nguyên.

## 6. AI trong phòng v2

Theo quyết định chủ sản phẩm 05/10 (phương án b — AI được đọc chat): trong phòng v2 server không có khoá, nên AI chỉ
nhận văn bản **client gửi lên** khi người dùng gọi (12 tin gần nhất, đủ nội dung, `Tên: lời nói`). Kết quả AI được
client niêm phong lại thành tin MLS trong epoch hiện hành. Điều này thay §7 ADR-0031 về grant từng tác giả cho phòng
v2; ghi nhận ở đây vì nó đổi hợp đồng của E2EE.

## 7. Push

1. `expo-notifications` (FCM trên Android, APNs trên iOS). Bảng Go `push_devices` (module `internal/push`, một writer):
   một hàng mỗi lần cài (`installation_id`), gắn **phiên đăng nhập** (`session_id`); token không đổi chủ giữa hai lần
   cài khác nhau (409). Phiên bị thu hồi (đăng xuất) → thiết bị push ngừng (trigger); xoá tài khoản → xoá thiết bị và
   hàng đợi. `PUT/DELETE /push/devices/{installation_id}`. `MOBILE_PUSH_MODE=log|expo`, giá trị lạ → từ chối khởi
   động.
2. **Push không mang nội dung**: tiêu đề "Rủ Đi", thân cố định "Có tin nhắn mới", data
   `{"t":"chat","c":<conversation_id>,"s":<sequence>}`; không tên người, không tên phòng, không chữ nào của tin. Worker
   đọc log `chat_v2_events` qua con trỏ riêng (chỉ đọc bảng của chatv2), gộp một lần đánh thức mỗi người mỗi phòng.
   Giải mã trong nền để hiện xem trước là bước sau (iOS Notification Service Extension) — chưa làm.
3. Gửi qua outbox Go (một writer), có retry và xoá token chết theo phản hồi của FCM/APNs.

## 8. Phiên bản và cutover

1. `protocol` trong AAD (đã có) + server từ chối client dưới `min_protocol`; app cũ nhận 426 và màn "cần cập nhật".
2. Phòng chuyển v2 khi mọi thành viên có ≥ 1 thiết bị đã ghi danh; từ đó route legacy ghi/xoá/react/mark trả
   409 `conversation_is_e2ee` (fail-closed); lịch sử cũ chỉ đọc, nhãn "chưa mã hoá đầu cuối". Guard dùng **chính router
   của cửa trước** để quyết route và `ParsePythonUUID` để đọc id phòng (không có bộ phân tích riêng), **không xác thực
   người gọi** (mọi lời ghi legacy vào phòng v2 đều bị từ chối, kể cả qua `X-Actor-ID` ở dev), đứng **kể cả khi tắt cờ
   làn v2**, và `core serve` từ chối khởi động nếu có bảng chat v2 mà schema lệch hoặc không kiểm được (vá rà soát bảo
   mật 05/10). Đánh đổi chấp nhận: ai biết id phòng thì biết phòng đó đã E2EE.
3. Không bao giờ hạ về plaintext. Rollback = tắt cờ gửi v2 mới; dữ liệu v2 giữ nguyên, không xoá.

## 9. Cổng

ADR-0031 giữ nguyên. Thêm: `cargo … --locked` ở mọi cổng; ABI Android strict cả local, mỗi `.so` phải có SONAME (thiếu
nó thì shim JNI ghi đường dẫn của máy dựng làm DT_NEEDED và điện thoại không nạp được — chỉ emulator bắt được); ABI iOS
trên runner macOS (EAS); test FFI; drill Rust↔Go trên PostgreSQL (`scripts/chat_drill.sh`); E2E native
(`.maestro/49-chat-e2ee.yaml` với thiết bị tổng hợp `tests/drill/ban-doi-tac.mjs`), cần thêm 3 tài khoản đa thiết bị.

## 10. Bằng chứng đã có (05/10)

- Rust: 33 canary (gồm phát lại sau khôi phục, commit phát lại kèm roster khác); clippy sạch; FFI test qua C ABI.
- Go: vòng đời thiết bị/KP/commit/Welcome/media/push trên PostgreSQL thật; drill ba thiết bị qua HTTP thật.
- Engine app với MLS thật (drill): ba thiết bị, Welcome, đua epoch, gỡ người; app chết sau `receive`; envelope rác;
  mất mạng rồi Thử lại; viết xong thì sập → gửi đúng một lần.
- Emulator Android (x86_64, bản debug): đăng nhập → phòng hai người lên "Mã hoá đầu cuối" → gửi → thiết bị kia giải mã
  và trả lời → máy giải mã câu trả lời → tắt mở app, lịch sử còn, đúng thứ tự và giờ.
