# Bàn giao chat: tiếp tục từ checkpoint, không làm lại từ đầu

**Cập nhật ngày 21-09-2026, khoảng 23:30 giờ Việt Nam.** Tài liệu được tạo theo
yêu cầu trực tiếp của Lead khi phiên làm việc sắp hết quota. Đây là trạng thái
công việc thực tế, không phải tuyên bố hoàn thành. Mục cuối ghi các cập nhật sau
mốc này; kiểm lại Git, process và PR trước khi tiếp tục vì chúng có thể đã đổi.

## 1. Mục tiêu, quyền đã có và giới hạn

Lead yêu cầu triển khai chat nhóm/cá nhân ở mức production: giao diện có câu
chuyện Nếp, mục tiêu 40/40 qua kiểm độc lập; đồng bộ reaction/xoá/poll; nhiều
người gửi đồng thời; bot tạo plan thật; Go backend hiệu quả; E2E bằng thao tác
người dùng và tải thật. Lead đã yêu cầu **bắt đầu tạo PR dần** và tài liệu này.
Tiếp tục triển khai, tạo checkpoint, push và mở PR trong phạm vi đó. **Chưa có
lệnh merge**; cần reviewer độc lập APPROVE theo AGENTS.md trước khi merge.

- Repo chính: `/home/lakiet/mobile`. Không reset/clean hay checkout đổi nhánh
  trong cây này: có rất nhiều file chưa tracked thuộc các công việc khác.
- Go cho backend nghiệp vụ; Python chỉ AI. Bản vá Python bảo mật legacy là
  ngoại lệ được ghi rõ. Không thêm domain, realtime hoặc migration Python mới.
- Chat v2 bắt buộc E2EE, không fallback plaintext. Legacy vẫn là compatibility;
  phải gắn nhãn, không gọi legacy là v2. AI chỉ nhận prompt được chia sẻ rõ ràng.
- Mỗi module chỉ một writer. Không đổi ownership production để làm xanh test.
- Docs/commit tiếng Việt; comment/docstring code tiếng Anh. Staging chính xác
  từng đường dẫn, chạy repo guard, không `git add .`, không miễn scanner rộng.
- Không đưa `.env`, token, transcript thật, session fixture hoặc toàn bộ Docker
  inspect vào repo/PR. Chỉ dữ liệu synthetic được dùng trong các lượt dưới đây.
- Lead có iPhone và Android; đang dùng **Expo Go, chưa setup EAS**. Chưa có
  runner Mac/iOS đã xác minh. Không hỏi lại những thông tin đã biết này.
- UI dùng skill local `.claude/skills/impeccable` đã đọc trong phiên trước;
  A/B phải tự chạy browser/context riêng, không đọc chéo trước kết luận.

## 2. Bản đồ Git và PR

| Vị trí/nhánh | Trạng thái tại bàn giao |
|---|---|
| `/home/lakiet/mobile`, `codex/p0-w28-chat-go-e2ee` | HEAD `e1501346`; backend candidate đã commit; perf/UI/Rust còn một phần chưa commit tại đây |
| `origin/main` | Đã fetch tới `63959c1d`; 14 commit mới từ ancestor `534c0fd1` |
| `/tmp/rudi-chat-pr-security`, `codex/chat-01-security` | Tạo từ main mới; cherry-pick thành công hai commit thành `38087902`, `6e325b00`; kiểm tích hợp/push/PR đang làm |
| `/tmp/rudi-chat-pr-ui`, `codex/chat-04-so-hen` | Checkpoint `c63a9471846791c1443eaf6629af148d91453573`, sạch, parent `63959c1d`; chưa push; phải đặt lên PR backend trước khi mở PR phụ thuộc |

`main` mới đã dọn nhiều tài liệu/ảnh, di chuyển probes, thu allowlist từ 453 còn
71 entry, và PR #623 đã chuyển 126 route sang LIVE-GO. **Không chép nguyên cây
cũ hoặc allowlist cũ lên main**, không hoàn tác cleanup/ownership. Những câu
trong tài liệu cũ nói production vẫn toàn Python có thể đã lỗi thời; kiểm
`services/core/ownership/routes.json` trước khi sửa lời mô tả.

Các checkpoint cũ trên nhánh gốc, theo thứ tự phụ thuộc:

1. `173f2391`: AGENTS/CLAUDE, ADR-0031, kiến trúc Go/E2EE.
2. `04311e20`: reauth idempotency replay, read watermark UPSERT Go + vá Python.
3. `f894604b`: Go v2 store, transport, lab tách biệt.
4. `63be9003`: retry/reconnect qua tiến trình thật.
5. `ad350d41`: bản nháp, read marker theo vùng nhìn thấy.
6. `08e346dc`: packet native cũ và giới hạn.
7. `35798a31`: load 1.000 kết nối và giảm tranh khoá đọc.
8. `bea1f7e5`: cursor nhận tách khỏi ACK gửi khi nhiều người nhắn.
9. `ef1ee46d`: 20-user E2E và các cổng còn đỏ.
10. `e1501346`: feed legacy, AI queue/promotion, snapshot/revocation fix;
    29 file, 3.633 dòng thêm. Đã qua staged guard + commit hook.

Chuỗi PR dự định (chưa có nghĩa mọi PR đã mở):

1. **chat-01-security → main**: quy tắc Go, replay authorization, read watermark.
2. **chat-02-realtime → chat-01-security**: toàn bộ final Go v2 store/transport,
   admission, dispatcher, outbox relay, load harness, erasure lock order.
3. **chat-03-changes-ai → chat-02-realtime**: `e1501346` + prerequisite stack/seed;
   đồng bộ reaction/delete/poll, AI consent/job/retry/promotion.
4. **chat-04-so-hen → chat-03-changes-ai**: frontend cuối, tests, A/B, ảnh.
5. **chat-05-crypto → nhánh có Go v2**: OpenMLS spike riêng, draft, chưa bật app.

Các PR phụ thuộc nên ghi rõ base và điều kiện đổi về main. Chọn full final file
snapshots theo phạm vi thay vì cherry-pick mù các commit cũ xen UI/backend. Không
đưa cùng thay đổi vào hai PR. Dùng `gh pr create --draft --body-file` với file
body tạm ngoài repo; cập nhật link thật ở cuối tài liệu. Chưa tự ký APPROVE.

## 3. Kết quả hiện tại: chứng minh gì, chưa chứng minh gì

| Hạng mục | Bằng chứng đạt | Chưa đạt/chưa làm |
|---|---|---|
| UI | A độc lập **32/40** trên bundle cuối; B đóng ba lỗi P1, 12 assertion; TS + 66 chat test; recovery UI 10/10 | Chưa 40/40, chưa native UI cuối, chưa motion/frame-time thiết bị thật |
| 20 browser users | 400/400 tin duy nhất; DOM p95 670 ms, p99 717 ms; 6/6 image/sticker/reply/reaction/delete/offline-retry; poll 4/4; DM hai chiều và outsider 403 | Lượt 20 người chạy trước vòng chỉnh UI cuối; không gọi đây là native/E2EE |
| AI plan thật | Provider thật, job durable, peer thấy card; review và tạo outing 201; peer mở outing; root chạy 5/5 | Harness 3-user tranh tạo cùng card mới chỉ syntax-check, chưa chạy; sealed AI v2 chưa làm |
| Legacy feed/security | 14 test legacy có PostgreSQL thật và race, độc lập APPROVE; 8 AI + 2 Redis relay độc lập PASS | Compatibility plaintext, chưa thay MLS |
| Go v2 transport | 46 test PostgreSQL thật/race, không SKIP; bytes/retry/ACK/revoke/session/dispatch/relay có canary | Burst 300/s còn FAIL; 30 phút cuối gián đoạn; chưa soak 24h |
| Rust MLS | OpenMLS thật, 21 host canary; Go preimage/signature interoperability; ARM64 build; Android emulator Bionic 21 canary | Chưa Expo bridge/Keystore, enrollment/rekey API, iOS, điện thoại thật, crypto audit độc lập |

**Không được báo production-ready, UI 40/40 hoặc E2EE end-to-end app đã xong.**

## 4. Backend đã làm và đường dẫn cần tiếp tục

### 4.1 Go v2 perf/transport — còn chưa commit ở cây gốc

Các file task, không gom file ngoài danh sách bằng wildcard toàn repo:

- `services/core/internal/chatv2/`: `store.go`, `store_test.go`, `batch.go`,
  `admission.go`, `admission_test.go`, `admission_postgres_test.go`.
- `services/core/internal/chatv2http/`: `handler.go`, `postgres_test.go`,
  `process_postgres_test.go`, `dispatch.go`, `dispatch_test.go`,
  `batch_postgres_test.go`.
- `services/core/internal/chatbus/redis.go`, `redis_postgres_test.go`;
  `internal/chatv2diag/profile.go`.
- `services/core/cmd/chat-load/main.go`, `main_test.go`;
  `cmd/chat-lab/main.go`; `services/core/go.mod`, `go.sum`.
- `services/core/internal/repo/erasure.go`, `erasure_lock_postgres_test.go`,
  `people_repo_oracle_postgres_test.go`.
- `docs/codex/2026-09-21/chat-v2-batch-independent-review.md`;
  `docs/architecture/02-chat-go-e2ee.md`;
  `docs/decisions/ADR-0031-chat-realtime-e2ee-et-regles-go.md`.

Các file foundation đã tracked ở checkpoint cũ cũng phải có trong PR02;
liệt kê bằng `git diff --name-only 534c0fd1 -- services/core scripts docs` và
lọc đúng feature. Đừng chỉ chép delta dirty vì base main chưa có foundation.

Cơ chế hiện có:

- Dispatcher theo hội thoại/replica; gom 20 ms, 8 slot DB; read window 2 MiB;
  frame immutable dùng chung tối đa 64 MiB; ACK 512 KiB/10 giây; cô lập peer chậm.
- Fresh REPEATABLE READ READ ONLY: session/person/device/member incarnation,
  block/epoch/HWM/events cùng snapshot; không cache ACL TTL. Kiểm expiry bằng
  clock DB trước trả kết quả. Send/readmark vẫn writer lock.
- Admission queue trước lấy DB connection: 1.024 room, 2.048 request toàn cục,
  256 mỗi room; pool 15 dùng 7 writer active, 2 writer/room (pool nhỏ thì 1).
- `SendSession`/`MarkSession` kiểm bearer sau queue trong transaction; thứ tự
  person → session → device → membership → incarnation → pair → conversation.
- Counter/event/outbox/dedup một data-modifying CTE. Bỏ NOTIFY trong writer.
- Relay mặc định PostgreSQL: 256 row SKIP LOCKED, gom hint rồi checkpoint cùng
  transaction. Redis tuỳ chọn: chỉ conversation UUID + seq, publish rồi mới
  checkpoint; broker lỗi giữ pending. Reconciliation 1 giây vẫn còn.
- `go-redis/v9` pin 9.17.3 vì module Go 1.23; bản mới nhất đòi toolchain cao hơn.
- Profiling loopback opt-in; pool/CPU/mutex/block counters. Không bật mặc định.

Nghiên cứu đã đối chiếu primary source: PostgreSQL 16 `async.c` cho thấy đường
NOTIFY serialize bằng global lock; Slack mô tả fanout theo conversation. Link
trong ADR, không suy luận rằng chỉ đổi bus là tự đạt realtime SLA.

Review sớm trong `chat-v2-batch-independent-review.md` chỉ bao phủ digest/40 test
tại lúc đó, **không tự mở rộng APPROVE sang session/admission/relay cuối**. Có
46-test rerun cuối nhưng chưa có full final integration review của PR02.

### 4.2 Legacy changes + AI — đã commit `e1501346`

Xem `chat-changes-implementation.md`, `chat-ai-independent-review.md`,
`chat-legacy-snapshot-independent-review.md` cùng thư mục.

- `internal/chatlegacychange`: SQL trigger/counter/change/outbox; GET changes,
  websocket auth/ACK, snapshot batched tối đa 100 ID, mặc định 50 tin. Projection
  revision chống ACK cũ ghi đè thay đổi mới; reaction cũ không chèn lịch sử giả.
- Root sửa read lock deadlock với erase: fresh RR READ ONLY thay FOR SHARE;
  ACL/projection/watermark cùng snapshot. Revoke trước snapshot thì từ chối;
  revoke đồng thời có thể kết thúc trước page cũ, nhưng page cũ không thấy event
  tương lai; lần đọc tiếp từ chối. Expiry dùng DB clock lúc commit read.
- Hai test deterministic pause peer-person query, erase person→membership,
  thêm tin tương lai, đọc snapshot cũ, rồi reauth bị từ chối; test expiry giữa
  snapshot/handoff. Không quay lại read lock gây cycle.
- `cmd/core`: candidate migration explicit, routes phải Go, auth prod; không
  tự bật candidate trong manifest production.
- `internal/chatassist`: capability, submit/get/retry/cancel; logical idempotency;
  8 job/người/phút; 2 worker, lease 75 giây, hạn job 15 phút, tối đa 3 lần;
  SKIP LOCKED; timeout job/model/DB 70/60/5 giây.
- Reauth trước capability, inference và publish; revoked/deleted/left member
  bị chặn; trigger cancel/purge; exact membership incarnation. Chỉ prompt đã
  chia sẻ + catalogue công khai tối đa 40 place; không history/roster/taste.
- Publish result + terminal job state cùng TX, lease fencing; purge prompt khi
  complete/cancel/expire. Old implicit AI endpoints/history bị chặn.
- Promotion atomic: source card unique/context advisory lock; outing/stops/
  binding/source-card một TX; 201 tạo, 200 retry cùng input, 409 input khác;
  peer feed nhận outing ID/revision.
- Python chỉ thêm internal AI capability, có internal token; 9 test AI và Ruff
  pass. Không đưa inference key vào Go response, log hay tài liệu.

Cần kiểm thêm queue fairness/index/hot polling, retry crash và các mutation
traffic mix. Tính năng AI hiện là compatibility; chưa sealed v2 invocation,
multi-author grants hoặc encrypted result distribution.

## 5. UI cuối và công việc thiết kế còn mở

Agent `/root/chat_frontend` đang chép đúng task vào worktree UI riêng. Bao gồm
26 tracked file thay đổi từ ancestor và 13 file mới, cả draft/read-cursor fix
cũ; không chỉ delta sau `e1501346`.

Các điểm chính: `GroupChatLive.tsx`, `SoHen.tsx`, `TheAi.tsx`,
`CreateOutingLive.tsx`, `CaiDatNhom.tsx`; hooks `useChatChanges`/`useChatAi`;
`thay-doi.ts`, `ai-invocations.ts`, `ban-nhap-cong-cu.ts`, `chat-route.ts`;
`tin-song.ts`, `useTinNhan.ts`; theme/provider/Sheet và session logout cleanup.

- Giấy–mực/Nếp, composer có công cụ image/sticker/poll/plan; voice chưa hỗ trợ
  thì không giả nút hoạt động. Legacy label rõ.
- Draft RAM theo account/context/form; giữ qua đóng panel/đổi route; xoá khi
  logout/account switch; submit thành công mới clear. Validation + footer CTA.
- Poll đóng và plan promoted thu gọn; pin mở outing thật; CTA manual tên thật
  “Tự tạo kèo”, không gọi đó là bản nháp shared khi chưa có backend shared draft.
- Theme một external store với matchMedia subscription; khắc phục palette mixed.
- Web Sheet focus trap, aria-modal, inert/aria-hidden nền, Escape + restore focus;
  năm ô màu 52×52 wrap đúng viewport 320. Native accessibility chưa kiểm.

Bundle final đã A/B nhìn độc lập:
`entry-5c477e5372d98f5ce4adba7628905d24.js`.

A-confirm: `chat-design-implementation-a-confirm.md`, **32/40**; đóng mất draft,
footer validation, label manual, pin, thu gọn poll. Còn P2: poll → bản nháp
chung → chốt chưa liền mạch; discard draft chưa Undo; P3 form nhỏ còn cuộn và
composer nền còn hiện. A không APPROVE tuyên bố 40/40.

B-confirm: `chat-design-implementation-b-confirm.md`, APPROVE riêng B1–B3;
palette thay đổi sống, contrast các chỗ lỗi đạt 13,56–15,79:1; 14 Tab và 3
Shift+Tab ở dialog; AX ẩn nền; Escape/restore focus; cả năm màu nằm trong 320 px.
12 assertion bao gồm hai check bundle. Detector warning được phân loại, không
coi mọi React Native web warning là lỗi thật hoặc dùng detector làm điểm đẹp.

Muốn đạt 40/40: thiết kế và triển khai nốt flow shared draft có semantics rõ,
Undo nếu cần, sự liên tục giữa poll/plan/outing; kiểm tương tác thật lại hai
reviewer độc lập, rồi native font scale/screen reader/motion. Không tăng điểm
bằng tự đánh giá hoặc chỉ sửa copy dài hơn. Không cho Nếp xen vào lỗi/tiền.

## 6. Crypto Rust: spike chạy thật, chưa tích hợp app

Owner `/root/chat_changes`, source `packages/chat-crypto`; README mô tả đúng
giới hạn, agent đang ghi `RESULTS.md`. OpenMLS 0.9.0/provider 0.6, Rust 1.98.1.

- Create/join/Welcome/add/remove/rekey; ACK commit riêng; private MLS messages;
  text/reaction/delete/vote typed operations; outer Ed25519 đúng Go signing bytes.
- Verify roster/identity/key package; hai signing key MLS/transport tách biệt;
  reject replay/tamper/outsider; retry giữ ciphertext; transport epoch = MLS + 1.
- XChaCha20-Poly1305 checkpoint với anchor độc lập từ OS bắt buộc để chống
  rollback; recovery identity mới, không import ratchet cũ. Chưa có OS storage.
- 21 host canary và 21 test executable chạy trên Android x86_64 Bionic API35;
  cross-build/link ARM64 bằng NDK27.1 API26. Chỉ push vào thư mục mktemp riêng
  dưới `/data/local/tmp`, chạy rồi xoá. Không đụng app/UI/emulator người khác.
- Chưa JNI/Swift/Expo dev build, Android Keystore/iOS Keychain, atomic
  state+outbox+ACK, process exclusivity, backup exclusion, trustworthy Go
  enrollment/rekey endpoints, 500-leaf load, attachment key protocol hoặc audit.
- Expo Go không nạp module Rust tuỳ ý này. Cần dev build khi tới native gate;
  chưa có EAS/Mac thì ghi gate thiếu, không thay bằng web unit tests.

Toolchain/task artifacts ngoài repo:

```bash
export CARGO_HOME=/tmp/rudi-chat-rust.t5HYgz/cargo
export RUSTUP_HOME=/tmp/rudi-chat-rust.t5HYgz/rustup
export CARGO_TARGET_DIR=/tmp/rudi-chat-rust.t5HYgz/target
export CARGO_BUILD_JOBS=2
cd /home/lakiet/mobile/packages/chat-crypto
/tmp/rudi-chat-rust.t5HYgz/cargo/bin/cargo +stable test --all-targets
```

Xem README trước chạy `scripts/check_go_interop.sh`, `check_android*.sh`;
<!-- repo-guard: allow=long-number reason=android-ndk-sdk-path -->
NDK `/home/lakiet/Android/Sdk/ndk/27.1.12297006`; ADB server 5038 có emulator
`emulator-5554`. Không restart ADB server, reboot emulator hay đổi APK của việc khác.

**Guard còn pending**: Cargo.lock hai checksum bị bắt nhầm số điện thoại;
agent đã đối chiếu 197 archive SHA256. Cần reviewer khác kiểm rồi mới thêm
allowlist đúng path + digest + duy nhất rule `vn-phone` theo repo policy.
Digest đang báo: `2f92268375658bdf5dd53cde29536fee58affce0f6983ce8b3175c725cacd4dd`.
Không tự miễn cả thư mục/lockfile mọi rule. Chưa checkpoint Rust trước review này.

## 7. Tải: lần cuối đã gián đoạn, cần chạy lại có giám sát bền vững

Tại 23:28 đã kiểm `ps`: PID 356548, 356653, 356668 đều không tồn tại; session
39596 cũng không còn. `/tmp/rudi-chat-mass.d8q7Sv6R/result.json` rỗng, progress
dừng khoảng 120 giây tại 11.979 sends, khoảng 5,984 triệu deliveries; p95 518 ms,
p99 1.077 ms, 0 send/socket error tại mẫu cuối. **Không có final verdict**, không
đủ 30 phút. Nguyên nhân dừng chưa xác minh; có tiền lệ process thuộc exec/agent
bị SIGTERM khi lifetime kết thúc. Không được kết luận chắc nguyên nhân này.

Giữ nguyên artifact và `/tmp/chat-v2-final-30m.log`. Đây là 200 người × 5 thiết
bị = 1.000 websocket, hai group 100 người/500 thiết bị, hai Go replica, PostgreSQL
fsync ON; signed opaque synthetic envelopes, **không phải payload MLS mobile**.

Lịch sử tải cần giữ cả lượt đỏ:

| Artifact | Kết quả và giới hạn |
|---|---|
| `/tmp/rudi-chat-mass.eE6Q1FMS` | Burst 300/s: 14.399 DB events, 7.199.500 deliveries đủ; 14.339 HTTP success, 3.661 error; p95 4.983 ms, p99 5.066 ms; FAIL, dù không loss/dup/gap |
| `/tmp/rudi-chat-mass.eaMTuvtw` | Burst sau đó vẫn FAIL; 3.039 DB messages, 1.519.500 deliveries; p95 khoảng 5.158 ms. Host có qemu/Python/ffmpeg/rustc khác đang tải; không gọi clean-host |
| `/tmp/rudi-chat-mass.6jrvu4z7` | Lượt 30 phút cũ dừng khi đã fail sau 8–9 phút; 26 send error, p95 1.360 ms, p99 3.776 ms; WALSync/checkpoint/pool contention có dấu vết |
| Lượt ngắn 60 giây | 2.997.500 deliveries; p95 89 ms, p99 179 ms. Chỉ chứng minh lượt ngắn, không thay soak |
| `/tmp/rudi-chat-mass.d8q7Sv6R` | Lượt cuối interrupted khoảng 120 giây, result rỗng; không tính PASS |

Chạy tiếp: trước hết commit/freeze source và lưu digest binary/source/hardware,
đảm bảo runner sống độc lập với agent (container/service được giám sát), rồi:

```bash
cd /home/lakiet/mobile
GOMAXPROCS=2 scripts/chat_mass_realtime.sh -duration 30m -rate 100 -people 200 -devices 5
# Chạy burst ở lượt riêng sau baseline, không chạy chồng.
GOMAXPROCS=2 scripts/chat_mass_realtime.sh -duration 60s -rate 300 -people 200 -devices 5
```

Lệnh trên là entrypoint harness, **chưa tự cung cấp supervisor bền vững**.
Harness tạo PG riêng, fsync mặc định ON, giữ artifact ngoài repo. Trước lỗi đầu
tiên lưu pg_stat_activity/locks/waits, pool/queue counters, CPU pressure/RSS và
source digests; không in connection strings có credential thật. Cho lượt full
duration chạy hết nếu còn kiểm được, đừng xoá log đỏ. Không giết qemu/ffmpeg/
Python hay container không thuộc task để làm đẹp số. Đo thêm replica restart,
broker failure, slow readers, reconnect, ACK/read-receipt mix, fair queue và
1.000 người × 1 thiết bị. 24h soak chỉ sau các lỗi hiện hữu được giải quyết.

## 8. Stack synthetic đang sống và E2E cần chạy tiếp

- Runtime state: `/tmp/rudi-chat-e2e.JpMlFu/`.
- `connection.json`, `sessions.json` chứa synthetic credentials: đọc bằng code,
  không cat, không đưa vào PR. 22 user fixture; OTP synthetic của stack là 000000.
- Go API `http://127.0.0.1:45801`; web `http://127.0.0.1:8178`.
- Web nằm trong container task `chat-e2e-web-review`, host network/user1000,
  serve `/tmp/rudi-chat-e2e.JpMlFu/web-reviewfix` bằng Go SPA server.
  Bundle final 5c477… nêu trên. Container bền hơn exec background trước đó.
- Core binary đang chạy có AI/promotion nhưng **trước fix RR read-only cuối**.
  Muốn E2E final backend phải restart riêng core từ source đã kiểm:

```bash
scripts/chat_e2e_stack.sh restart-core /tmp/rudi-chat-e2e.JpMlFu/connection.json
```

Script giữ env runtime ngoài repo rồi xoá tempfile. Không in Docker env:
Python AI container đang có provider key thực được cấp từ cấu hình đã có;
không ghi lại key hay tự đưa key vào repo. Chỉ prompt synthetic được gửi.
Nếu stack đã mất, `scripts/chat_e2e_stack.sh` và `scripts/chat_e2e_seed.mjs` là
entrypoint dựng mới; đọc script trước, không trỏ vào database production.

Root đã chạy provider thật qua UI: `/tmp/rudi-chat-e2e.JpMlFu/ai-ui.cjs`,
`ai-ui.json` 5/5, các ảnh `ai-consent.png`, `ai-result.png`, `ai-plan-review.png`,
`ai-created-outing.png`, `ai-peer-promoted.png`. Model khoảng 10,4 giây, reviewed
time được sửa từ range sang HH:mm, outing tạo thành công, peer nhận card.

**Việc chưa chạy quan trọng:** harness mới committed cùng UI:
`apps/mobile/tools/chat-live-plan.mjs`. Ba context độc lập, login OTP thật,
submit AI, hai người cùng promotion source card, yêu cầu 200+201 cùng outing,
người thứ ba mở đúng outing. Đã syntax-check, chưa execution. Lệnh:

```bash
cd /home/lakiet/mobile/apps/mobile
export PATH=/home/lakiet/.nvm/versions/node/v22.23.2/bin:$PATH
CHAT_E2E_SESSIONS=/tmp/rudi-chat-e2e.JpMlFu/sessions.json \
CHAT_E2E_OUTPUT=/tmp/rudi-chat-plan-final \
CHAT_E2E_WEB=http://127.0.0.1:8178 \
node tools/chat-live-plan.mjs
```

Cần tạo OUTPUT ngoài worktree nếu script yêu cầu; chỉ chạy khi đã kiểm stack
và bundle. Selector source card dùng `data-testid=chat-message-ID`. OTP limit
có retry đợi 61 giây; đừng xoá auth/rate-limit để test xanh. Web auth ở RAM;
page reload mất auth, không nhầm demo Team Đà Lạt với context synthetic live.

Các artifact khác: `ui-final/`, `dm-final/`, `recovery-reviewfix/recovery.json`
(10/10), `design-a-confirm/`, `design-b-confirm/`, `frontend-tests.tap`,
`frontend-api-tests.tap` trong cùng state dir. Browser của root/A/B đã đóng.
Chỉ stop container task nếu cần; không dùng `docker prune` hoặc `pkill chrome`.

## 9. Lệnh kiểm và bằng chứng đã giữ

System Node18 không phù hợp ESM test này; dùng Node22 ở đường dẫn trên.

```bash
cd /home/lakiet/mobile/apps/mobile
export PATH=/home/lakiet/.nvm/versions/node/v22.23.2/bin:$PATH
npm run typecheck
npx tsc -p tsconfig.test.json
node tools/fixup-esm.mjs
node --test tests/rudi-chat*.test.mjs
```

Đã có TS+66 chat test trên final UI và fresh UI worktree; 46 API/auth test ở
lượt trước. UI evidence không thay thế test native.

```bash
cd /home/lakiet/mobile
GOMAXPROCS=2 scripts/chat_v2_postgres.sh
# Các package ngoài script cần PG thật + migration; dùng tier này với sentinel.
GOMAXPROCS=2 scripts/go_postgres_tier.sh -- -race \
  ./internal/testdb ./internal/chatlegacychange ./internal/chatassist ./internal/chatbus
```

**Chú ý:** `go_postgres_tier.sh` là correctness tier có fsync/synchronous_commit
OFF để nhanh, không dùng số latency tier này làm tải production. Mọi test phải
có `CORE_REQUIRE_POSTGRES_TESTS=1`, `CORE_TEST_DATABASE_URL`; không SKIP. Script
chat_v2 có sentinel riêng. Khi đổi `-run` phải giữ sentinel của tier đang dùng.

Log ngoài repo (đọc result/PASS/FAIL; không giả rằng file tồn tại mãi):

- `/tmp/chat-v2-two-writer-confirm-pg.log`: 46 test v2/race, không SKIP.
- `/tmp/chat-erasure-perf-tests.log`: erase lock-order oracle.
- `/tmp/chat-ai-bus-independent.log`: 8 AI + 2 relay độc lập PASS.
- `/tmp/chat-feed-snapshot-independent.log`: 14 legacy độc lập PASS.
- `/tmp/chat-feed-snapshot-gate.log`: root full 22 ca liên quan; pre-fix
  `/tmp/chat-feed-root-independent.log` có 20 ca, không nhầm hai snapshot.
- `/tmp/chat-candidate-unit-final.log`: core/httpapi/chatassist/feed/brain/routes.
- `/tmp/chat-ai-bus-pg-final.log` có lần đỏ khi pull Redis quá timeout;
  `/tmp/chat-ai-bus-pg-confirm.log` chạy lại sau pull đạt 16; giữ cả hai.
- Rust: `/tmp/rudi-chat-crypto-{tests,clippy,interop,android,android-runtime}.log`.

`/tmp` là artifact tạm, **có thể mất khi reboot**. Trước đổi máy/cleanup hãy đưa
report tổng hợp, digest và selected synthetic screenshots đã guard vào PR hoặc
artifact store được phép; không copy sessions/key/runtime env vào repo. Tài liệu
handoff và checkpoint Git là phần bền vững, log tạm không được coi là đã backup.

## 10. Thứ tự tiếp tục đề nghị

1. Đọc AGENTS.md, tài liệu này và cập nhật cuối; kiểm `git status` theo path,
   `git log -5`, `gh pr list` từng head, không quét mọi branch hàng trăm trang.
2. Hoàn tất PR01 từ `/tmp/rudi-chat-pr-security`: sửa docs ownership lỗi thời,
   chạy targeted tests trên main mới + guard; push và mở draft. Giữ link tại đây.
3. Thu checkpoint frontend/Rust từ agent; ghi commit/digest. Commit perf bằng
   path cụ thể, bảo toàn root dirty UI/unrelated assets. Chưa có final perf review.
4. Dựng PR02/03 theo chuỗi, preserve cleanup/manifest main. Test trên **cây PR**,
   không chỉ root dirty. Cần baseline/contract/PG thật trước production ownership.
5. Restart core synthetic với RR fix; chạy harness 3-user plan, cập nhật UI PR
   và evidence. Đừng báo harness vừa viết là E2E đã pass.
6. Đặt UI commit lên PR03, thêm ảnh và A/B report, mở draft ghi rõ32/40.
7. Hoàn tất guard lock Rust sau kiểm độc lập, mở crypto spike draft riêng;
   thiết kế enrollment/epoch/atomic-native-storage trước Expo bridge production.
8. Chạy tải 30 phút bằng runner bền vững; điều tra 300/s bằng profiling/waits,
   fix nguyên nhân, rồi burst/mixed-load/chaos/soak. Không hạ ngưỡng để xanh.
9. Cải thiện flow UI tới mục tiêu qua A/B độc lập; native Android/iOS và crypto
   audit là cổng riêng. Ghi rõ thiết bị thật/runner nào chưa truy cập được.
10. Agent quay lại chỉ cần đọc delta từ checkpoint mới, so digest và review các
    commit tiếp nối; không replay mọi test/log cũ hoặc ghi đè sửa của agent sau.

## 11. Trạng thái agent và cập nhật sau mốc đầu

- `chat_frontend`: đã commit `c63a9471846791c1443eaf6629af148d91453573`, 58 file;
  fresh TS + 66/66 test, guard/hook pass, worktree sạch. Allowlist chỉ thêm 10
  digest (71 → 81). Chưa push/PR; vẫn cần đặt trên PR03. Handoff riêng:
  `docs/codex/2026-09-21/chat-ui-implementation/README.md` ở worktree UI.
- `chat_visual_a`: hoàn tất A-confirm32/40, browser đóng.
- `chat_visual_b`: hoàn tất B-confirm scoped APPROVE12assertion; browser/detector
  8472 đóng, web8178 không đổi; không còn pending.
- `chat_changes`: Rust source đóng băng, chuỗi fmt/clippy/interop/ARM64/Android
  cuối session56170 exit0; `RESULTS.md` đã có. Fingerprint source
  `a2b56e299a3a60e8e2d62c98bf2e44b4c8a3ab5d4ec0714565babd4978221d7f`.
  Không còn task nền; chờ review/guard lockfile, chưa commit.
- `chat_v2_perf`: đã bàn giao; session tải39596 không còn; không coi agent idle
  nghĩa là load vẫn chạy. Nên kiểm process/container theo ID trước thao tác.

**Cập nhật tiếp theo phải ghi ngày, checkout, SHA, PR URL, tests đã chạy và việc
còn mở. Không sửa lượt FAIL/INTERRUPTED thành PASS; thêm lượt mới với provenance.**
