# Bàn giao PR nháp: tài khoản tự quản, email thật và Google

Ngày 2026-10-04. Người nhận: engineer làm trọn lát cắt, theo ADR-0032.
Đây là bàn giao công việc còn mở theo yêu cầu chủ sản phẩm, **không phải
phán quyết sẵn sàng production và không cho phép bỏ cổng để merge**.

## 1. Đọc trước và tiếp tục từ đâu

1. Đọc `AGENTS.md`, `CLAUDE.md`, ADR-0055 và
   `docs/testing/tai-khoan-tu-quan.md`.
2. Đọc bàn giao riêng trên chính máy này:
   `/home/lakiet/.local/share/rudi-auth/HANDOFF-PRIVATE.md`.
   File đó chứa tên tài khoản thật, địa chỉ nghiệm thu, thông tin client OAuth,
   đường dẫn ENV, screenshot và backup. Không copy file đó vào Git hoặc PR.
3. Nhánh tính năng: `codex/tai-khoan-tu-quan-20261004`.
   Checkout làm việc: `/home/lakiet/.local/share/rudi-auth/worktree`.
   Checkout dựng/kiểm cuối: `/home/lakiet/.local/share/rudi-auth/release4`.
   **Tên release4 là tên thư mục; HEAD của nó đang là candidate5.**
4. Mã sản phẩm cuối đã dựng và triển khai nghiệm thu:
   `ec0736f7ede7e3fa12669bc372210479dede6326`.
   Commit bàn giao sau nó chỉ bổ sung tài liệu/template cấu hình.
5. `origin/main` lúc bàn giao: `d3f747307a8e318bd92015bb2fb0cb862bf2f6ae`.
   Repo gốc `/home/lakiet/project/mobile/ru-di-app` còn rất nhiều WIP của
   phiên khác, HEAD `2b6c9360f40219a20c727cb03585443859160146`.
   Không stash/reset/checkout/merge vào cây gốc và không đụng WIP đó.

Nếu nhận task ở máy khác, lấy code từ PR là đủ để đọc/sửa/test tổng hợp;
credential và bằng chứng thật không đi cùng Git. Cần kênh secret riêng để
vận hành bản nghiệm thu. Không thể tái tạo khóa hoặc mật khẩu từ tài liệu này.

## 2. Yêu cầu sản phẩm đã chốt trong phiên

Đầu bài cũ là OTP email thay SMS, bỏ lời mời đích danh và dọn demo. Chủ sản
phẩm mở rộng thành tài khoản tự quản bằng username/mật khẩu, xác minh email
đăng ký và khôi phục, hoặc Google; UUID ngẫu nhiên bất biến làm định danh.
Tra bạn/mời nhóm dùng username. Không lấy số điện thoại/email làm UUID.
Không tự gộp tài khoản Google và local theo email. Giữ lời mời vào nhóm và
buổi đi; chỉ bỏ lời mời đổi lấy phiên đăng nhập.

Chủ sản phẩm yêu cầu tối thiểu **8 ký tự**, đã sửa cả Go và giao diện từ
15 thành 8. Giữ trần 128, Unicode/NFC, không trim mật khẩu, blocklist và
Argon2id. ADR-0055 ghi rõ đây **không đạt mốc 15 ký tự single-factor của NIST
SP 800-63B-4**; OTP đăng ký không phải MFA cho mỗi lần đăng nhập. Không âm
thầm đổi lại 15 hoặc tuyên bố đã có MFA.

Người dùng cho phép cấu hình Google Cloud, Brevo, HTTPS Funnel và dùng hai
hộp thư thật được chỉ định. Chưa cho phép mua domain hay dịch vụ trả phí.
Đã yêu cầu mở PR để biết tiến độ và bàn giao; không merge tự động.

## 3. Đã triển khai trong code

### Auth Go và persistence

- Module mới `services/core/internal/accountauth/` sở hữu nghiệp vụ auth,
  migration SQL, mail outbox và worker; không thêm backend nghiệp vụ Python.
- `managed_accounts` gắn `person_id` UUID ngẫu nhiên, username unique,
  email mã hóa + HMAC tra cứu, password hash hoặc `(issuer, sub)` Google.
  Không lưu email thô cho tra cứu và không liên kết theo email.
- Argon2id có salt riêng, tham số phiên bản; giới hạn hash đồng thời,
  body JSON 16 KiB, decoder chặn field lạ/JSON mơ hồ và context request 10s.
- Email challenge sáu số, TTL 300s, tối đa năm lần sai, resend 60s,
  giới hạn theo người nhận; secret binding phía client, DB giữ digest.
  Outbox mã hóa, claim/lease/retry, bỏ mail quá hạn và xóa payload đã gửi.
- Redis limiter chia sẻ giữa replica, yêu cầu `noeviction`, đóng auth bằng
  503 khi lỗi; không fallback limiter trong RAM. Chỉ tin proxy CIDR được cấu
  hình và một IP do proxy đã làm sạch cung cấp.
- Google dùng `google.golang.org/api/idtoken`: kiểm chữ ký, hạn, issuer,
  audience và nonce một lần; không nhận token client tự khai là hợp lệ.
- Thay password/email, link/unlink Google cần reauth trong năm phút.
  Không gỡ cách đăng nhập cuối. Reset/thay password thu hồi phiên theo ca
  kiểm chứng; logout-all và account erasure cũng thu hồi phiên.
- Web cookie HttpOnly/Secure/SameSite=Strict, bearer chỉ trong bộ nhớ.
  Native lưu phiên bằng SecureStore. Auth mới tránh kho idempotency chứa
  request/response, tránh lưu password/OTP/token vào generic replay store.
- UUID hiện hữu của profile/nhóm/sổ cái được giữ. Discovery mới đồng bộ
  cột legacy để reader cũ không mở rộng quyền; module có một writer.

18 route của handler mới nằm tại `accountauth/handler.go`:

```text
POST   /auth/register
POST   /auth/register/verify
POST   /auth/login
POST   /auth/password/reset/request
POST   /auth/password/reset/confirm
POST   /auth/google/challenge
POST   /auth/google
POST   /auth/google/register
GET    /people/me/account
POST   /people/me/account/reauth
PUT    /people/me/account/password
POST   /people/me/account/email
POST   /people/me/account/email/verify
POST   /people/me/account/google
DELETE /people/me/account/google
PUT    /people/me/account/discovery
DELETE /sessions/all
POST   /friends/lookup
```

Đọc manifest `services/core/ownership/routes.json` và contract tương ứng;
không lấy danh sách trên làm thay thế nguồn sở hữu route. Phiên web và các
consumer domain còn dùng route hiện hữu, không phải tất cả đều mới.

### Migration và retirement

Thứ tự operator: Alembic hiện hữu → `core migrate-profile` →
`core migrate-accounts` → `core serve`. Migration Go có advisory lock,
transaction và digest version 1; startup kiểm checksum. Không sửa `schema.sql`
đã cài tại chỗ để bổ sung schema: thêm migration có phiên bản phù hợp.

Migration giữ person và ledger, thu hồi phiên không thuộc managed account,
xóa credential phone. Không có chuyển tài khoản điện thoại thành tài khoản
local. Đường `/auth/otp/*`, `/identity/person-id`, bootstrap `POST /sessions`
và lời mời đăng nhập `/moi` bị retirement. Không bật lại chúng để rollback.

Ngoại lệ Python: sửa runtime legacy để ngừng đăng ký cửa cũ và vá tương
thích/bảo mật; oracle/test lịch sử vẫn giữ. Manifest, Go retirement và Python
runtime cùng thay đổi theo ADR-0036. Không xóa oracle để gọi là port xong.

### Frontend, native và demo

- Form đăng ký, xác minh mail, login username, Google chọn username, reset,
  trang bảo mật, reauth/link/unlink/discovery/logout-all và khôi phục phiên.
- Web Google Identity Services popup; Android Expo module Kotlin dùng
  Credential Manager. Android dùng **web/server audience**, còn Google Cloud
  có client Android riêng khóa theo package/certificate release.
- Bỏ phone lookup, SMS OTP, lời mời đổi lấy phiên và dữ liệu demo khỏi đường
  sản phẩm. Câu mời nhóm cuối đã sửa thành đăng nhập đúng tài khoản được mời.
- Script seed và Maestro legacy chuyển vào fixture test lịch sử cô lập,
  không được dùng để gọi là nghiệm thu dữ liệu thật. Không xóa bằng chứng
  CI chỉ vì nó còn chữ demo/phone trong corpus lịch sử.
- Thêm flow native account mới, web E2E account/session, phục hồi test bản
  đồ web có và không WebGL; fixture đều tổng hợp.
- Hướng dẫn Nếp đã sinh lại: hash `eda560b4faaf`, 61 route. Chạy generator
  trong **apps/mobile**, không phải `tools/` ở root repo.

## 4. Bằng chứng máy: phải phân biệt SHA

Log và ảnh gốc nằm ngoài worktree, dưới
`/home/lakiet/.local/share/rudi-auth/`. Không upload nguyên log có dữ liệu thật.

| Source | Đã đo | Kết quả / giới hạn |
|---|---|---|
| `ec0736f7` | Auth unit + `go vet` package accountauth | PASS; gồm 11 ca biên độ dài/password policy |
| `ec0736f7` | PG thật auth chọn lọc + DB sentinel | 7 ca PASS, sentinel có mặt, 0 SKIP; không phải toàn tầng Go PG |
| `ec0736f7` | Identity/canary/hai mutant | Identity xanh; canary sai mật khẩu đỏ đúng bước; bỏ nonce và bỏ reset revocation đều đỏ đúng test hành vi |
| `ec0736f7` | `npm test` mobile | 1496 PASS, 0 FAIL, 0 SKIP; không thay thế toàn chặng mobile/native |
| `ec0736f7` | Generator Nếp | PASS hash `eda560b4faaf` |
| `ec0736f7` | APK release ký thật | BUILD SUCCESSFUL 11m17s, 997 tasks; apksigner xác minh v2, đúng certificate/package |
| `ec0736f7` | Cutover web/core/api | Healthy; HTTPS login 200, `/api/healthz` 200, catalog 200, asset thiếu 404; chưa theo dõi một giờ ở SHA này |
| `8b44dd51` | Tải auth hai replica | 100 workers, 20 login hợp lệ/s, 15 phút, 18000 HTTP 201, 0 lỗi; p95 32.687001 ms; giữ rate limit và assert 429 |
| `8b44dd51` | Theo dõi một giờ | 121 mẫu / 3600.03s, 0 lỗi HTTP ngoài dự kiến, 0 lỗi mail worker được ghi nhận, p95 khoảng 16 ms |
| `f7622b8a` | Native account tổng hợp | Hai lượt Maestro PASS + canary fingerprint; đã mở cả 8 ảnh login sai/đúng/resume/logout để xem |
| `f7622b8a` | Parity legacy | 345 scenarios / 10618 steps / 0 differences; prod auth legacy 21 scenarios / 572 steps / 0 differences; chặng parity PASS |
| `f7622b8a` | PG Python + QA | 702 + 51 PASS |
| `f7622b8a` | Go PG / media / broker | 3351 / 43 / 180 PASS; sentinel có mặt |
| `f7622b8a` | Toàn `gate.sh --strict` | **CHƯA ĐẠT**: Go Milvus không healthy sau 180s; AI Milvus treo hơn hai giờ, operator dừng; các chặng sau chưa chạy trong lượt này |

Backend auth source `8b44dd51` và `ec0736f7` giống nhau; khác câu giao diện
mời nhóm và hướng dẫn sinh máy. Điều đó giải thích tính liên quan của tải,
**không** biến kết quả nguồn trước thành whole-gate tại HEAD PR.

Lượt focused cuối có lỗi operator ở lệnh generator: gọi
`node tools/rut-huong-dan.mjs --check` từ root, nên wrapper thoát lỗi module
not found sau khi các test đã PASS. Đã gọi lại đúng từ apps/mobile và PASS;
giữ nguyên log lỗi, sửa helper riêng cho lượt sau. Không gọi wrapper cũ là
exit 0, không có lỗi generator sản phẩm ở kiểm đúng đường dẫn.

Các mutant ở source cuối nằm trong `/tmp/rudi-account-mutants.IU0e0L`;
source/harness SHA được ghi cùng bộ. Dự đoán trước: nonce sai phải bị từ chối
trong `TestPostgresGoogleNonceNoEmailMergeAndSafeLink`; session cũ sau reset
phải thất bại trong `TestPostgresAccountProofResetAndSessionRevocation`.
Mutant không tương đương vì hai witness đó đổi kết quả HTTP, không phải
compile failure hay SKIP. Bộ nguồn trước cũng có mutant PASS nhưng không
dùng nó thay nguồn cuối.

## 5. Bằng chứng tài khoản và dữ liệu thật

Đã tạo **3 managed account thật qua UI**, gồm 1 local và 2 Google.
Đếm cuối: 0 phone identity, 1 đăng ký đã xác minh mail, 1 mail outbox hoàn
tất, 0 mail chờ. **Chưa có phiên đăng nhập password thành công** tại lúc bàn
giao. Người dùng nói đã đăng nhập Google trong Chrome; không coi đó là login
password của app. Không biết và không lưu password local.

Đã kiểm bằng thao tác UI thật:

- Google admin và Google người dùng thứ hai đăng nhập, chọn username,
  tạo UUID, tải trang bảo mật sau reload và logout-all.
- Gửi lời mời kết bạn giữa hai người, người còn lại nhận/chấp nhận, danh sách
  bạn hiển thị đúng. Không giả token hoặc seed person để gọi là bằng chứng.
- Tắt discovery ở người dùng thứ hai, tài khoản admin lookup username bị
  từ chối tìm thấy. Cài đặt này **vẫn đang tắt**; cần trả về lựa chọn ban đầu
  khi tiếp tục nghiệm thu, hoặc giữ theo chủ ý người dùng.
- Tạo một nhóm nghiệm thu bằng admin, hiện chỉ có một thành viên.
  Chưa mời người thứ hai và chưa chứng minh quyền outsider bằng dữ liệu thật.
- Email Google thứ hai đã thuộc account local: app tạo person Google riêng,
  không tự gộp và không lấy email recovery đó. Đây là hành vi đúng đã quan
  sát, không sửa bằng UPDATE DB hay gộp person.

Tên/email/UUID nhóm và ảnh thật nằm trong bàn giao riêng. Không có bằng
chứng 100 tài khoản người thật: **100 client ở bài tải là tổng hợp**, không
phải 100 người/email thật. Catalog thật giữ lại, nhưng không dùng số hàng
catalog để tuyên bố đã nghiệm thu 100 luồng người dùng.

## 6. Hạ tầng đã vận hành và cấu hình ENV

Stack nghiệm thu riêng `rudi-auth-prelaunch`, compose private
`/home/lakiet/.local/share/rudi-auth/prelaunch/compose.json`.
Core Go + API FastAPI legacy + ingest + Caddy + PostgreSQL 16 + Redis riêng.
DB/Redis không publish port; web loopback 12330 được HTTPS Funnel chuyển tiếp.
Không đụng stack dùng chung `rudi-vnlocal-*`, các stack UX của phiên khác,
hay emulator do người/agent khác mở.

Chuyển DB nghiệm thu từ máy remote Tailscale sang PG riêng cùng Docker
network sau reboot/latency. Dump mã hóa attempt3 restore thành công:
159 public table counts khớp, ba fingerprint identity/session khớp chính xác.
Backup trước migration cũng đã restore vào DB riêng; không sửa ledger để dọn.

Các file thật `~/.config/rudi/`, mode 0600:

| File / biến | Mục đích / điều kiện |
|---|---|
| `accounts.env` | Cấu hình auth Go; compose nạp sau các ENV chung |
| `MOBILE_ACCOUNT_AUTH_ENABLED=1`, `MOBILE_AUTH_MODE=prod` | Bật auth mới, yêu cầu migration; không dùng 0 làm rollback |
| `MOBILE_ACCOUNT_ENCRYPTION_KEY` | Khóa AES-GCM 32 byte base64, mã hóa email/challenge/outbox |
| `MOBILE_ACCOUNT_LOOKUP_KEY` | Khóa HMAC 32 byte base64 khác khóa trên; tra email/rate key/binding digest |
| `MOBILE_AUTH_REDIS_URL` | Redis riêng có password, AOF, noeviction; URL là secret |
| `MOBILE_EMAIL_SMTP_HOST/PORT/USER/PASSWORD`, `MOBILE_EMAIL_FROM` | Brevo SMTP TLS, sender đã xác minh; password là SMTP key, không phải password Google |
| `MOBILE_GOOGLE_CLIENT_IDS` | Web/server ID-token audience; client ID công khai, không cần OAuth client secret để verify ID token |
| `MOBILE_CORS_ALLOW_ORIGINS` | Frontend HTTPS được cho phép; không thay cho Authorized JavaScript origins trên Google Cloud |
| `MOBILE_AUTH_TRUSTED_PROXY_CIDRS` | Đang để trống; không tin header IP tự khai; cần đo proxy trước mở rộng tải nhiều người |
| `prelaunch-local-database.env` | `MOBILE_DATABASE_URL` role app riêng; nạp cuối để thắng URL DB remote |
| `prelaunch-database.env` | URL DB remote cũ, giữ cho provenance/backup; không dùng làm URL live cuối |
| `prelaunch-postgres-admin-password` / `prelaunch-postgres-app-password` | Secret files admin/app PG riêng; role app không superuser/createdb/createrole |
| `auth-redis-password` / `brevo-smtp-password` | Secret gốc riêng; không đưa vào Dockerfile/build args/JS |
| `stack.env` / `vnlocal.env` | Internal token, hạ tầng/cổng/catalog/S3 hiện hữu; không sửa quyền hoặc key dùng chung |
| `ai.env` / `ai-infer.env` | Cấu hình AI hiện hữu và secret liên quan; không phải credential auth user |
| `android-release.keystore` / `android-release-password` | Khóa ký APK release; alias/certificate có trong bàn giao riêng |
| `auth-backup-key` | Khóa backup GPG; cất cùng máy chưa chứng minh DR ngoài site |

Đã tạo profile tiện tiếp tục, hoàn toàn ngoài repo:

```text
/home/lakiet/.local/share/rudi-auth/session-env/core.env
/home/lakiet/.local/share/rudi-auth/session-env/mobile.env
/home/lakiet/.local/share/rudi-auth/session-env/toolchain.env
/home/lakiet/.local/share/rudi-auth/session-env/manifest.json
```

`core.env` là snapshot hợp nhất các ENV backend theo thứ tự compose và
override; compose live vẫn đọc các file gốc, không tự đồng bộ snapshot khi
sửa file gốc. `mobile.env` chỉ có `EXPO_PUBLIC_API_URL` và
`EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID`. `toolchain.env` là shell exports các
đường toolchain đã dùng; không chứa credential backend. Không source core.env
vào build frontend. Template `.env.example` chỉ chứa tên biến/placeholder.

Google Cloud admin đã bật 2FA; Web origin và Android release client đã tạo
theo các phê duyệt cụ thể. OAuth app vẫn **Testing**, chưa published/verified.
Client JSON người dùng tải xuống không đưa vào repo; web/native hiện verify
ID token qua web audience, không dùng client secret của JSON đó. Brevo Free
300 mail/ngày; sender Gmail không thay thế domain SPF/DKIM/DMARC production.

Proxy Caddy private đã xử lý Path cookie khi strip `/api`, CSP/GIS popup,
HTML no-cache, giữ hai entry JS của các bản trước khi cutover, asset thiếu
trả 404 thay vì SPA HTML. File và compose backup nằm cạnh bản live. Phải
recreate web sau đổi export directory để mount nhận thư mục mới.

## 7. APK cuối, toolchain và bằng chứng UI

Artifact riêng:
`/home/lakiet/.local/share/rudi-auth/artifacts/rudi-prelaunch-release5.apk`.
SHA-256:
`5768de23d008da4320f616be8014e6a2ad98943ad98de01a486b0c9986d51f47`.
Package `com.lakiet.rudi`, versionName 1.0.0, versionCode 1, targetSdk 36.
SHA-1 certificate:
`63:13:A5:8F:25:83:82:B7:E2:5B:D0:4C:08:3B:5F:FD:87:18:5D:01`.
APK chưa được nghiệm thu Google thật trên thiết bị release và chưa phát hành
Play Store; versionCode 1 không phải kế hoạch phát hành hoàn chỉnh.

Toolchain thực tế: Go 1.26.8, Node 22.23.2, Python 3.12 trong venv riêng,
<!-- repo-guard: allow=long-number reason=public-android-ndk-version -->
JDK 17.0.20.1+1, Android SDK 36/NDK 27.1.12297006, Expo/React Native +
TypeScript theo package-lock, Kotlin Credential Manager, Maestro và Rust
toolchain cho crypto theo lockfile. Shell `python3` mặc định máy là 3.14;
source toolchain profile trước gate để chọn venv 3.12. Python operator
ngoài repo chỉ điều phối build/backup, không phải backend nghiệp vụ.

Giới hạn build riêng: Gradle max workers 1, heap 1536 MiB, ActiveProcessorCount
2, Ninja compile pool 2/link pool 1; giữ mọi ABI. Không đổi gate/ABI để giảm
độ phủ. Init signing đọc file secret, không hard-code password.

Ảnh native tổng hợp đã mở xem: `/tmp/rudi-native-account.ZbdpgR/`, hai lap,
flow 50, bốn trạng thái mỗi lap. Ảnh web thật đã mở xem và gửi trong phiên
cho chủ sản phẩm nằm tại thư mục `evidence/` private. Không upload ảnh có
tên/email thật lên PR. Agent sau mở ảnh local khi đánh giá UI; thiếu ảnh
ở Git không được diễn giải thành đã có independent review.

## 8. Việc còn thiếu để hoàn tất, theo thứ tự thực hiện

### P0 — whole-gate sạch đúng HEAD và Milvus

1. Ổ đĩa lúc chẩn đoán dùng 96%, còn khoảng 26 GiB. Container AI Milvus
   tạm log `logstore local disk above hard watermark, write backpressure
   active`, retry liên tục. Đây là quan sát trực tiếp; chưa chứng minh nó
   cũng là nguyên nhân duy nhất của lỗi Go Milvus readiness trước đó.
2. Lượt source3 đã dừng, container `ai-infer-milvus-779287` đã gỡ; giữ
   `gate-release3-resumed.log`, status interrupted và hai log private
   `evidence/ai-milvus-interrupted-*.log`. Không nhận là gate kết thúc xanh.
3. Giải phóng **chỉ cache build/test của task này đã xác định**, hoặc cấp
   storage test đủ dung lượng; giữ secrets, backup, artifacts và evidence.
   Không xóa Docker volume/ảnh/database dùng chung bằng prune tổng quát.
4. Đo lại `scripts/go_milvus_tier.sh` và
   `scripts/ai_infer_tier.sh --milvus` trên Milvus tạm pinned có auth.
   Giữ log startup khi lỗi. Không bỏ auth, tăng timeout vô căn cứ, skip
   sentinel hoặc patch dependency để tạo pass.
5. Tạo checkout sạch đúng HEAD PR/merge result rồi chạy `make gate` hoặc
   `scripts/gate.sh --strict` đủ 32 chặng hiện tại. Lượt source3 dừng trước
   e2e/chat-e2e/crypto; source5 chưa chạy whole-gate.
6. Chạy identity/canary/hai mutant cùng HEAD/harness; ghi số thật vào commit
   kết quả. Commit sửa code làm mất giá trị nhận whole-gate tại SHA trước.

AVD của task là `rudi-auth-acceptance`, serial emulator-5612, đã dừng để
nhường RAM. Emulator khác còn chạy là của phiên khác, không dùng/kill nó.
Lệnh khởi động wrapper chính thức có trong bàn giao riêng. Không replay
qemu argv đã chụp ở file JSON: có thể thiếu môi trường launcher.

### P0 — nghiệm thu auth thật còn thiếu

1. Local account: login sai/đúng bằng password người dùng đã đặt, reload,
   logout, reset với email thật và chứng minh mọi phiên cũ bị thu hồi.
2. Đổi email/password sau reauth, link/unlink Google, không gỡ cách cuối,
   link account đã thuộc người khác bị từ chối; kiểm UUID/nhóm/ledger giữ
   nguyên. Không UPDATE account để lách việc local và Google cùng email
   thuộc hai person. Bộ test tổng hợp có phủ nhưng nghiệm thu thật chưa có.
3. B không đọc/ghi dữ liệu riêng A; friend/block/discovery hai chiều;
   mời người thứ hai vào nhóm, nhận lời mời, outsider trước/sau khi tham gia;
   buổi đi qua consumer thật. Nhóm hiện chỉ một người, friendship đã accepted.
4. Chrome công cụ bị `ERR_BLOCKED_BY_CLIENT` ở API danh mục/Explore, trong
   khi cùng HTTPS API từ terminal trả JSON 200. Chưa có trả lời người dùng
   cho kiểm trực tiếp API danh mục. Kiểm policy/extension hợp lệ trên máy;
   không đổi tên endpoint để vượt chặn và không dùng CDP/đọc profile để lách.
5. Native Android **APK release cuối** Google thật + lifecycle SecureStore,
   mạng di động/thiết bị thật, signing đúng client; iOS Google/native riêng.
   Maestro debug tổng hợp không chứng minh Google release.
6. Chuẩn bị thêm trường hợp dữ liệu thật có consent rõ nếu mục tiêu là 100
   trường hợp/người thật; không gửi hàng loạt mail hay tự thu thập PII để
   đạt số. Không dùng tải 100 client tổng hợp thay thế bằng chứng này.

Công cụ computer use bắt buộc người dùng tự nhập/xác nhận/submit credential
mới khi đặt hoặc đổi password. Agent không biết password local; không hỏi
password/OTP trong chat, không đọc OTP từ DB, không giả token. Quyền điều
khiển toàn máy đã cho không bỏ yêu cầu handoff này. Đây là phần không thể
cam kết agent sau hoàn tất mà không có thao tác credential của người dùng.

### P0/P1 — vận hành auth và tiêu chí production

- Sau bản cuối, theo dõi một giờ đúng source triển khai và outbox/SMTP,
  kiểm restart riêng và outage PostgreSQL. Redis outage/recovery thật đã
  kiểm; PG restart/failover cuối chưa đủ bằng chứng. Test PG transaction
  không thay cho service outage/recovery.
- Mở rộng limiter/proxy: CIDR đang trống nên người dùng qua proxy có thể
  chia sẻ giới hạn IP. Kiểm X-Forwarded-For sanitization của chuỗi HTTPS
  proxy thật trước chọn CIDR. Không whitelist toàn Internet.
- Hạ tầng ổn định có storage headroom, monitoring/alerts, backup có retention,
  restore drill ngoài máy/site và quản lý khóa tách biệt. Backup GPG đã kiểm
  decrypt/restore tại chỗ; chưa có geographic/offsite DR và chưa đo RPO/RTO.
- Domain gửi + SPF/DKIM/DMARC, SMTP capacity/bounce/abuse, Google OAuth
  production origin/consent/publication/verification phù hợp; Funnel trên
  máy cá nhân và Brevo Free là nghiệm thu, chưa thay hosting production.
- Đánh giá độc lập bảo mật và dependency, CSRF/session/rate/abuse/log retention,
  tải đăng ký/email/hash thực tế ngoài phép đo login hiện tại; không gọi
  Argon2 + unit test là toàn bộ security review.
- Chat nhóm thật đang hiện nhãn «Chưa mã hoá đầu cuối» và có composer legacy.
  **Không gửi nội dung thật để test auth.** Đọc
  `docs/architecture/02-chat-go-e2ee.md`; chat v2/E2EE/native crypto/kiểm
  chứng độc lập/tải là chương trình cổng riêng. Auth mới không chứng minh
  hoặc khôi phục khóa chat. Chưa sửa đường chat legacy thành v2 trong task này.

## 9. Lệnh tiếp tục và nguyên tắc kết thúc

```bash
source /home/lakiet/.local/share/rudi-auth/session-env/toolchain.env
cd /home/lakiet/.local/share/rudi-auth/worktree
git status --short
git rev-parse HEAD
python3 scripts/repo_guard.py staged
python3 scripts/repo_guard.py tree HEAD
python3 scripts/repo_guard.py range origin/main HEAD
```

Tạo checkout sạch riêng đúng SHA để gate, cài dependency đã pin, source
toolchain; không lấy kết quả cây engineer dirty. Full gate cần AVD riêng
đã boot, Docker và dependency CI, không dùng stack có dữ liệu thật làm DB test.

```bash
bash scripts/gate.sh --strict
bash scripts/account_auth_mutants.sh
(cd apps/mobile && node tools/rut-huong-dan.mjs --check)
```

Live prelaunch đã migrate nên không migrate lại bằng password lộ trên argv.
Vận hành bằng compose private đã có ENV và secret files. Không chạy
`docker compose config` hoặc in env nguyên bản vào log/PR. Build frontend
nạp riêng mobile.env; build APK theo helper private đã pin source, nếu SHA
thay phải cập nhật assert và dựng lại. Không giao APK release4 cũ.

Giữ PR draft đến khi P0 và các cổng bắt buộc hoàn tất. Cập nhật tiến độ bằng
commit/tài liệu mới theo quy ước nhật ký; không sửa hồ sơ frozen protocol v1.
Mọi thay đổi phải giữ các luật tiền, writer và oracle. Có lỗi gate ngoài auth
thì ghi rõ và xử lý, không xóa test hoặc dùng green auth để nhận toàn sản phẩm.
