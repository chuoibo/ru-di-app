# Thiết kế 02 — Hàng đợi bất đồng bộ và streaming (Rủ Đi AI + Nếp)

- Ngày: 2026-09-25. Commit gốc: `f251db7`. Nhánh: `claude/peaceful-hopper-32kwjs`.
- protocol_version: không áp dụng (không phải lượt thí nghiệm). Verdict: không có reviewer người.
- Trạng thái: **thiết kế đã được người dùng duyệt, chờ Lead ký ADR**. Đề xuất đi kèm:
  `docs/decisions/proposals/ADR-0038-hang-doi-rabbitmq-redis-va-stream.md`.
- Thắng khi lệch: `docs/architecture/03-ai-engine-hop-dong.md`. Tài liệu này chép từ bản thiết kế
  `infra` và hai phản biện (tmp, sẽ mất). Lát 4 đã vào main (`d596621`: `core work`, heartbeat,
  `ClaimByID`, sweep riêng); hàng đợi, outbox, Redis và SSE chưa có code; mọi số dưới đây là mục
  tiêu, chưa phải số đo.

Nhãn **(sửa theo phản biện)** đánh dấu chỗ đổi so với bản gốc để áp một phát hiện phản biện
(PB1 = «ràng buộc, riêng tư, bảo mật, đúng đắn»; PB2 = «khả thi, trình tự, đầy đủ»). Nhãn
**(tự phát hiện)** đánh dấu chỗ lệch tìm ra khi đối chiếu lại code lúc chép vào repo.

| Phát hiện | Nội dung ngắn | Áp ở |
|---|---|---|
| PB1 #3, #4, #5 | phòng không nhận mã guard; guard trước mọi byte; lane v2 cưỡng chế ở writer | §3.6, §5.1, §5.3 |
| PB1 #6, PB2 #10 | một trần lời gọi model mỗi job, đếm cả retry; limiter theo lời gọi | §4 bước 5, §6 |
| PB1 #7, PB2 #1, #3 | số migration theo thứ tự vào main; một enum sự kiện; phòng đi qua WS sẵn có | §3, §5 |
| PB1 #17, PB2 #11 | pool DB riêng cho worker, semaphore tool, kết nối dành cho heartbeat | §6 |
| PB1 #18, PB2 #16 | `enqueue_seq` đơn điệu thay `generation`; giữ default khi rollout | §3.2, §4 bước 9 |
| PB1 #19, #20, #26 | `retry:` là trường thân SSE; khai `job_outbox` trong cổng; sweep chạy cả ở `serve` | §5.2, §3.2, §4 bước 6 |
| PB2 #13, #14, #17 | task Nếp sống khi đóng bảng; registry tác vụ định kỳ; route vào manifest | §5.4, §4 bước 10, §5.2 |

## 0. Sự thật đã kiểm trong code (HEAD `f251db7`)

- **Worker hôm nay**: `Handler.Run` chạy 2 goroutine, nhịp 250 ms (`chatassist/worker.go:31-50`).
  Mỗi nhịp `claim` chạy 3 sweep (`worker.go:61,67,71`) rồi nhận một job bằng `FOR UPDATE SKIP LOCKED`,
  lease 75 s, `attempts<3` (`worker.go:77`). Job có trần 70 s (`worker.go:89`), gọi brain 60 s
  (`worker.go:124`). `publish` là một transaction đăng-một-lần (`worker.go:235-270`). Worker chạy
  ngay trong `serve` (`cmd/core/main.go:214-218`).
- **Timeout 8 s** trên mọi route `chatassist` (`chatassist/handler.go:83`). Route tính năng đi qua
  CORS rồi thẳng vào handler, không qua dispatch và `idem` (`main.go:220-237`), nên SSE không bị
  đệm. `cors.simpleWriter` có `Unwrap` và `Flush` (`httpapi/mw/cors/cors.go:212-218`). Danh sách
  header CORS khoá theo golden Starlette, không có `last-event-id` (`cors.go:27-28`).
- **Server công khai không có `WriteTimeout`** (`main.go:269-276`); khi dừng, `stopChat()` chạy
  **trước** `public.Shutdown` 25 s (`main.go:301-305`). **Pool DB 15** mỗi process (`db/db.go:32-34`).
- **`chatassist.Matches` khớp mọi đường dưới `ai-invocations`**, kể cả `/me/nep/...`
  (`handler.go:72-78`); `ownership/routes.json` chưa có hàng nào của `chatassist`. Phòng v2 bị từ
  chối ở cửa tạo: 409 `encrypted_invocation_required` (`handler.go:163-176`).
- **Hạn 8 lượt/phút/người** đếm trên `chat_ai_invocations` dưới advisory lock
  (`handler.go:413-421`, `nep.go:307-313`). Cửa sổ chia sẻ 15 phút (`handler.go:422`, `nep.go:315`).
- **Migration `chatassist`**: chuỗi version có checksum, mỗi version một file
  (`chatassist/migrate.go:41-111`). `serve` chỉ kiểm bảng tồn tại (`main.go:166-180`).
- **Mẫu dùng lại**: relay outbox `chatbus.Flush` (crash giữa publish và commit → trùng, không mất;
  `chatbus/redis.go:102-161`), regex namespace (`redis.go:30`). WS `chatlegacychange`: 1000 slot,
  reconcile 1 s (`chatlegacychange/handler.go:45`), tối đa 5 kết nối mỗi người (`handler.go:294`),
  vòng trang gọi lại `Store.Changes` (có `authorize`) mỗi nhịp (`handler.go:284-291`).
  `authorize` không kiểm v2 (`chatlegacychange/store.go:50-118`).
- **WS: khung xác thực chỉ có `type` và `token`** (`chatlegacychange/handler.go:229-232`), đọc bằng
  `wsjson.Read`, tức `json.Unmarshal`, nên trường lạ bị bỏ qua. `query()` từ chối mọi tham số
  ngoài `after`/`limit` (`handler.go:94-97`). Frame trang không có trường `type` (`store.go:28-35`).
- **Client WS cũ coi mọi tin là một trang**: sai dạng thì đóng socket, và tin tới lúc đang `busy`
  cũng đóng socket (`apps/mobile/src/rudi/chat/useChatChanges.ts:55-66`).
- **Redis đã có trong `go.mod`** (`go-redis/v9 v9.17.3`), chỉ chat-lab dùng
  (`cmd/chat-lab/main.go:101-102`). Chưa có RabbitMQ, chưa có SSE. `amqp091-go` v1.15.0 khai
  `go 1.20`, dùng được cả trước khi nâng Go 1.25 (lát 3).
- **Mobile**: gói `expo` 57.0.17 cài `expo/fetch` làm `fetch` toàn cục trừ khi đặt
  `EXPO_PUBLIC_USE_RN_FETCH` (`node_modules/expo/src/winter/runtime.native.ts:42-52`, ngoài Git);
  repo không đặt biến đó. Polling hôm nay: 2000 ms (`useChatAi.ts:39-41`), `nhipHoiNepMs`
  (`nep/hoi.ts:125`). Đóng bảng Nếp là xoá câu đang chờ (`nep/useNepHoi.ts:58-68`).

## 1. Thành phần

| Mới/sửa | Đường dẫn | Vai |
|---|---|---|
| MỚI | `services/core/internal/jobs/` | outbox, relay AMQP có confirm, consumer theo hàng, poller dự phòng, registry tác vụ định kỳ, `Migrate` + `job_schema_migrations` |
| MỚI | `services/core/internal/aistream/` | enum sự kiện, `Writer` (writer Redis duy nhất), hub đánh thức, bộ mã SSE, limiter Redis |
| SỬA | `chatassist/worker.go` | `claimNext` (SQL hôm nay), `claimByID`, `process`, `heartbeat`, `release`, `retryLater`; sweep sang ticker riêng |
| SỬA | `chatassist/handler.go`, `nep.go` | hai route `/events`; bỏ timeout 8 s cho `/events`; `ai.stream` trong `chat-capabilities` |
| SỬA | `chatlegacychange/handler.go`, `store.go` | frame `ai` opt-in trên WS sẵn có; `Store.Changes` trả thêm `lane` |
| SỬA | `cmd/core/main.go` | lệnh `core work`; `MOBILE_INPROC_WORKER`; kiểm version schema; `jobs.Migrate` trước `chatassist.Migrate` |
| MỚI | `apps/mobile/src/rudi/ai/sse.ts`, `useAiStream.ts` | đọc SSE trên `expo/fetch`, nối lại, rơi về polling |
| SỬA | `rudi/chat/useChatChanges.ts`, `rudi/nep/NepProvider.tsx` | rẽ frame `ai` trước nhánh `busy`; task Nếp do provider giữ |

## 2. Hai chữ dễ nhầm: «hàng» và «lane»

**Hàng (queue)** là `ai.group`, `ai.nep`, `memory`, `notify`, suy từ `scope` trong trigger. **Lane**
là `legacy` hay `v2`: cột `lane` do lát 7 thêm, **máy chủ tự suy** từ `chat_v2_conversations`, mặc
định `'legacy'`; lane quyết định stream đi đâu, không quyết định hàng. Bản gốc gọi cột outbox là
`lane`; ở đây đổi thành `queue` để không ai đọc nhầm thành lane E2EE (tự phát hiện).

## 3. Lược đồ và hợp đồng dây

Số version **không đặt trước**: cấp theo thứ tự lên main. `serve` và `work` từ chối chạy khi
`max(version)` của `chat_ai_schema_migrations` < N hoặc của `job_schema_migrations` < M
(sửa theo phản biện). Hôm nay `main.go:166-180` chỉ kiểm bảng tồn tại.

### 3.1 Gói `jobs`, version đầu (bảng version riêng, mẫu `chatassist/migrate.go`)

```sql
CREATE TABLE job_outbox (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  queue text NOT NULL CHECK (queue IN ('ai.group','ai.nep','memory','notify')),
  ref_id uuid NOT NULL,             -- id hàng trong bảng việc sở hữu
  enqueue_seq bigint NOT NULL,      -- lần vào hàng thứ mấy của hàng việc đó
  available_at timestamptz NOT NULL,
  expires_at timestamptz,           -- quá hạn mà chưa publish thì xoá
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (queue, ref_id, enqueue_seq));
CREATE INDEX job_outbox_den_han ON job_outbox(available_at, id) WHERE published_at IS NULL;
-- jobs_them(queue, ref, seq, available_at, expires_at): INSERT … ON CONFLICT DO NOTHING
-- rồi pg_notify('job_outbox',''). Là đường chèn DUY NHẤT vào job_outbox.
```

Bảng **không có cột payload**: thứ duy nhất tới được broker là id. Không cột `person_id`, nên test
liệt kê cột `person_id` của trigger xoá trên `people.deleted_at` (lát 15) thấy nó rỗng, đúng thiết
kế. Tin hàng `memory` chỉ mang id: fact đã «quên» (xoá cứng, lát 15) thì tin cũ gặp 0 hàng và bị
Ack. `queue` bỏ `ingest`: nạp dữ liệu là lệnh CLI, không phải hàng (sửa theo phản biện).

### 3.2 `chatassist`, version kế tiếp lúc vào main (lát 10)

```sql
ALTER TABLE chat_ai_invocations
  ADD COLUMN available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  ADD COLUMN enqueue_seq bigint NOT NULL DEFAULT 0,
  ADD COLUMN first_token_at timestamptz,          -- nội dung đầu tiên (phan|delta) rời worker
  ADD COLUMN model_calls integer NOT NULL DEFAULT 0 CHECK (model_calls >= 0);
```

- **Giữ mọi `DEFAULT`**, không `DROP DEFAULT`: replica cũ INSERT theo `handler.go:422` vẫn chạy
  trong lúc rollout (sửa theo phản biện).
- Trigger `BEFORE INSERT OR UPDATE OF status`: khi `NEW.status='queued'` và (INSERT hoặc
  `OLD.status<>'queued'`) thì tăng `NEW.enqueue_seq`. Trigger `AFTER` cùng điều kiện gọi
  `jobs_them(CASE NEW.scope WHEN 'me' THEN 'ai.nep' ELSE 'ai.group' END, NEW.id, NEW.enqueue_seq,
  NEW.available_at, NEW.share_expires_at)`. Phủ đủ: tạo, `/retry` (`handler.go:511`), `retryLater`,
  `release`. Heartbeat, huỷ, xong không đổi sang `queued` nên không chạm outbox.
- `enqueue_seq` đơn điệu thay cho `generation = attempts`. Bản gốc có lỗi: `release` đặt
  `attempts-1`, trùng khoá cũ, `ON CONFLICT DO NOTHING` nuốt mất lần vào hàng, job chờ poller ≥5 s
  (sửa theo phản biện).
- **Trigger không phải để lách cổng đọc.** Lý do chọn trigger là mọi đường vào `queued` đều được
  phủ mà không đường Go nào phải nhớ. Cổng đọc xuyên gói (lát 5) **khai rõ** `job_outbox` trong
  allowlist của gốc Nếp và gốc nhóm, và một test Postgres đọc `pg_trigger` để chắc hai trigger này
  là đường ghi duy nhất (sửa theo phản biện).
- Hàng `queued` từ trước migration không có dòng outbox; poller (§4 bước 6) nhận chúng sau ≤7 s.

### 3.3 Thông điệp AMQP (≤128 byte)

Thân `{"v":1,"ref":"<uuid>","seq":N}`, giải mã với `DisallowUnknownFields`; `MessageId =
"<queue>:<ref>:<seq>"`, persistent. **Không đặt `Expiration` từng tin** như bản gốc: tin hết hạn bị
dead-letter làm DLQ đầy rác, trong khi hạn đã do `claimByID` (`share_expires_at>now()`) và relay
(xoá dòng quá `expires_at`) lo (tự phát hiện).

### 3.4 Topology RabbitMQ (`jobs.Declare`, idempotent mỗi lần nối)

Exchange direct bền `rudi.jobs`; quorum queue `ai.group`, `ai.nep`, `memory`, `notify`, mỗi cái
`x-delivery-limit=5`, DLX `rudi.dlx` → `<queue>.dlq` (quorum, TTL 7 ngày, chỉ id). **Ưu tiên bằng
tách hàng, không dùng message priority**: mỗi hàng một channel, `Qos(prefetch = concurrency)`, mặc
định mỗi process `ai.nep` 4, `ai.group` 4, `memory` 2, `notify` 8; bản gốc 8+8 cho hai hàng AI là
quá sức pool 15 (sửa theo phản biện). `memory`/`notify` chỉ có queue ở lát 10; consumer do lát 15
(`nep_trich`) và 17 (push) mang theo, theo hợp đồng ở §4 bước 11.

### 3.5 Key Redis (namespace `rudi:{ns}:`, kiểm bằng regex của `chatbus`)

| Key | Kiểu | Người đọc | Giới hạn |
|---|---|---|---|
| `ai:inv:{id}` | Stream, trường `e`, `j` | người gọi (Nếp; nhóm lane v2) | `MAXLEN ~1024`; `EXPIREAT share_expires_at`; sau sự kiện kết thúc `EXPIRE min(120 s, còn lại)` |
| `ai:room:{context}` | Stream, thêm trường `inv` | phòng lane `legacy` | `MAXLEN ~2048`; mỗi lần ghi `XTRIM MINID ~ <now−15 phút>` và `EXPIRE 900` |
| `ai:wake` | Pub/Sub, payload là hậu tố key | hub mỗi process | chỉ metadata, ≤128 byte |
| `rl:model:{model}` | GCRA (Lua) | limiter lời gọi model | TTL theo chu kỳ |
| `rl:sse:{băm người}` | `INCR` + `EXPIRE 60` | giới hạn mở SSE | không để id người thô trong Redis |

`XTRIM MINID` giữ mọi entry của key phòng sống không quá cửa sổ chia sẻ 15 phút, kể cả khi phòng
liên tục có lời gọi mới làm mới TTL của key (tự phát hiện). Redis chạy
`--save "" --appendonly no --maxmemory 128mb --maxmemory-policy volatile-ttl`: chữ stream không
bao giờ xuống đĩa; bị evict thì client rơi về polling.

### 3.6 Sự kiện stream: enum đóng của hợp đồng (gói `aistream`)

| Sự kiện | Dữ liệu | Ai nhận | Thay cho (bản gốc) |
|---|---|---|---|
| `hello` | `{nhip_ms}` | SSE | `hello` |
| `trang_thai` | `{cau}` | SSE + phòng | `thinking` |
| `phan` | `{kind, json}` (thẻ đã grounding) | SSE + phòng | — |
| `delta` | `{p, text}` | SSE + phòng | `delta` |
| `lam_lai` | `{}` (chỉ khi chưa có nội dung) | SSE + phòng | `reset` |
| `xong` | nhóm `{message_id}`; Nếp `{text, chips, nguon}` | SSE; phòng chỉ `message_id` | `done` |
| `that_bai` | `{code}` | SSE; phòng chỉ mã chung | `failed` |
| `huy` | `{}` | SSE + phòng | `cancelled` |
| `thu_hoi` | `{}` rồi đóng | SSE | `revoked` |
| `ket_noi_lai` | `{sau_ms}` | SSE | `reconnect` |

Nhịp tim là dòng `: ping`. Không có `retract` hay `snapshot` (sửa theo phản biện). `cau` là enum
của engine (thiết kế 01); hạ tầng thêm đúng một giá trị, `dang_xep_hang`, do handler SSE dựng từ
Postgres (§5.2). Frame phòng không bao giờ mang mã guard: mọi mã guard gộp về `ai_tu_choi`, câu
khủng hoảng chỉ tới người gọi (sửa theo phản biện). Mã mới cần câu tiếng Việt trong
`cau-chu-goi-ai.test.mjs` cùng commit.

## 4. Trình tự thực thi

1. **Tạo** (handler không đổi). INSERT và dòng outbox do trigger commit cùng transaction, kèm
   `pg_notify`. Client nhận 202 `{id}`: đó là task id.
2. **Relay** (`jobs.Relay`, mỗi process worker một cái; nhiều relay an toàn nhờ SKIP LOCKED): thức
   theo `LISTEN job_outbox` hoặc nhịp 250 ms; `SELECT … WHERE published_at IS NULL AND
   available_at<=now() ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 256`; xoá dòng quá `expires_at`;
   publish `mandatory` + confirm (chờ ≤2 s, `basic.return` là thất bại); đặt `published_at` **sau**
   confirm rồi commit. Crash giữa hai bước → trùng, không mất (như `chatbus`).
3. **Consumer**: tin sai dạng → `Reject(false)` → DLQ. `claimByID(ref, seq)` là cùng câu UPDATE với
   `claimNext`, ứng viên chọn bởi `id=$1 AND enqueue_seq=$2 AND available_at<=now()`. 0 hàng →
   `Ack` (trùng, đã xong, đã huỷ, tin của lần vào hàng trước, chưa tới hạn). Lỗi DB →
   **dừng tiêu thụ**, về chế độ poll tới khi DB khoẻ; tin đang cầm được **giữ lại** (không
   `Nack(requeue)` — sửa ở vòng 2 lát 10, xem §8) rồi chạy lại khi DB trả lời. `Ack` chỉ sau khi
   transaction kết thúc job commit (`publish`, `nepXong`, `finishFailure`, `retryLater`, `release`).
4. **Heartbeat** mỗi 5 s trên kết nối dành riêng:
   `UPDATE … SET lease_until=now()+interval '30 s' WHERE id AND lease_id AND status='running'`.
   0 hàng → huỷ context job, writer phát `huy`. Huỷ tay hay trigger rút quyền thành viên
   (`schema.sql:38-49`) vì thế dừng worker trong ≤5 s.
5. **Thử lại tự động** qua `retryLater`: `status='queued'`, `available_at=now()+backoff` (1 s rồi
   4 s, ±20 %), trigger đưa vào hàng, stream phát `lam_lai`. Chỉ khi **đủ cả**: mã tạm thời
   (`provider_unavailable`, 429/5xx sau khi `aiharness/llm` đã tự thử — genai tắt retry riêng,
   thiết kế 01 §5 —, limiter model từ chối);
   `first_token_at IS NULL` (chưa có `phan` hay `delta` nào rời worker); `attempts<3`;
   `model_calls < MaxModelCallsPerTurn`; `now()+backoff < created_at+30 s`. Mọi lỗi khác là cuối
   cùng. Route `/retry` giữ nguyên và đặt lại `first_token_at`, `model_calls`: đó là lượt mới do
   chính người gọi bấm; `attempts<3` vẫn chặn tổng.
6. **Lưới Postgres**: `claimNext` luôn chạy, mỗi 2 s với lọc `available_at<now()-5s` (cứu tin lạc,
   lease hết hạn); mỗi 250 ms chỉ với `available_at<=now()` khi AMQP mất kết nối hoặc không cấu
   hình. Ba sweep hôm nay (`worker.go:61,67,71`) sang ticker 5 s, chạy ở **cả `serve` lẫn `work`**,
   để trần 15 phút plaintext (`docs/architecture/02-chat-go-e2ee.md:207`) vẫn giữ khi worker co về 0
   (sửa theo phản biện). UPDATE idempotent nên chạy trùng vô hại; mỗi lượt lấy
   `pg_try_advisory_xact_lock` để khỏi dồn.
7. **Lease sau nội dung đầu tiên.** Nhận lại job `running` hết lease chỉ khi `first_token_at IS NULL`;
   đã có nội dung mà worker chết thì sweep đặt `failed/worker_interrupted`, chữ đã phát giữ nguyên,
   thêm `that_bai`. Không thế thì worker thứ hai viết lại từ đầu: «rút lại» trá hình (tự phát hiện).
8. **Lease 75 s → 30 s** chỉ khi mọi process đã chạy heartbeat (`MOBILE_AI_LEASE_SECONDS`).
9. **Tắt êm `core work`**: SIGTERM → huỷ consumer. Job chưa có nội dung được `release`
   (`status='queued'`, `attempts=attempts-1`, xoá lease, `available_at=now()`); `enqueue_seq` mới nên
   worker khác nhận ngay qua broker (sửa theo phản biện). Job đang stream có tối đa 60 s để xong;
   compose `stop_grace_period: 75s`.
10. **Tác vụ định kỳ**: `jobs.DinhKy{Ten, Nhip, Chay}`, mỗi lượt giữ
    `pg_try_advisory_xact_lock(hashtextextended('jobs:'||ten,0))` và chạy **trong chính transaction
    đó** (vòng 2 lát 10: khoá phiên trên một kết nối rồi làm việc trên kết nối thứ hai làm pool nhỏ
    tự bỏ đói, xem §8), chạy trong `core work`. Registry không
    sở hữu logic; gói sở hữu tự đăng ký: dọn outbox, sweep `chatassist`, xoá 30 ngày
    (`ai_turn_metrics`, `rag_query_log`, `chat_ai_tin_hieu`), củng cố trí nhớ hằng đêm, nhắc 15 phút,
    indexer RAG (sửa theo phản biện).
11. **Hợp đồng cho bảng việc mới** (`nep_trich`, push): cột `status, attempts, lease_id, lease_until,
    available_at, enqueue_seq`; trigger BEFORE/AFTER gọi `jobs_them`; một `claimByID`; bảng được
    khai trong allowlist cổng đọc.

## 5. Streaming

### 5.1 Writer (`aistream.Writer`, trong worker; writer Redis duy nhất)

- **Chọn key ở đúng một hàm** `khoa(job)`: `scope='me'` → `ai:inv:{id}`; nhóm lane `legacy` →
  `ai:room:{context}`; nhóm lane `v2` → `ai:inv:{id}`. **Không bao giờ** `XADD` key phòng cho job v2
  (sửa theo phản biện). Lane đọc từ cột lúc claim; `prepare` gọi lại `authority`
  (`handler.go:163-176`) nên phòng vừa chuyển v2 làm job hỏng trước khi có byte nào.
- **Chỉ output guard sinh `delta`.** Engine nối `Sink` qua `guard.CuaSo` (cửa sổ 48 ký tự): chữ chỉ
  rời cửa sổ khi đã lùi quá 48 rune sau đầu model, hoặc khi model xong và phần còn lại đã qua guard.
  Câu vi phạm không bao giờ rời cửa sổ; stream dừng và kết thúc bằng `that_bai` hay câu từ chối
  của engine. Không có sự kiện rút lại (sửa theo phản biện). Cổng AST: ngoài test, chỉ
  `aiharness/guard` được gọi `Sink.Delta`.
- Trước lần ghi nội dung đầu tiên, writer chạy đồng bộ `UPDATE … SET first_token_at=clock_timestamp()
  WHERE id AND lease_id AND first_token_at IS NULL`. `trang_thai` phát lúc claim; `xong` phát
  **sau** khi transaction đăng commit. `Delta` cài `slog.LogValuer`, log ra `"[n bytes]"`.
- Gom `delta`: xả mỗi 60 ms hoặc 256 byte; `XADD` + `PUBLISH ai:wake` chung pipeline, timeout
  300 ms; hai lỗi liên tiếp → no-op cho job đó, job vẫn xong trong Postgres. Khi engine chưa stream
  (lát 6, 9), cả câu trả lời qua cửa sổ thành một `delta` cuối, nên hạ tầng ship được độc lập.

### 5.2 SSE cho người gọi và Nếp

- Route `GET /contexts/{context}/ai-invocations/{id}/events` và
  `GET /me/nep/ai-invocations/{id}/events`. `Matches` đã khớp. `ServeHTTP` bỏ timeout 8 s cho hậu tố
  `/events`. Hai hàng `GO-ONLY` vào `ownership/routes.json` cùng commit (sửa theo phản biện).
- **Auth trong một transaction ngắn rồi commit** (`begin` / `beginNep`, cộng kiểm job thuộc đúng
  người, đúng membership). Stream không giữ kết nối DB nào. Gốc `nepEvents` vào allowlist gốc Nếp
  của cổng đọc xuyên gói lát 5 và chỉ chạm các bảng của gốc đó (`chat_ai_invocations`,
  `account_sessions`, `people`, `job_outbox`).
- Header: `Content-Type: text/event-stream; charset=utf-8`, `X-Accel-Buffering: no`. Thân mở bằng
  dòng `retry: 2000`. Bản gốc ghi `retry` như một response header, sai: đó là trường trong thân SSE
  (sửa theo phản biện). Mỗi lần ghi qua `http.NewResponseController` với hạn ghi 10 s.
- Mở đầu: `hello`; nếu Redis chưa có gì cho job và hàng Postgres còn `queued`/`running` thì phát
  ngay `trang_thai{cau:"dang_xep_hang"}` **không có dòng `id:`**, nên sự kiện trạng thái đầu không
  phụ thuộc hàng đợi. Nối lại: `Last-Event-ID` (native) hoặc `?after=` (web, vì CORS khoá header),
  đọc loại trừ `XRANGE key (after +`.
- Đọc: một subscription `ai:wake` mỗi process nuôi hub. Mỗi người nghe chạy `XRANGE … COUNT 128`
  không chặn khi được đánh thức hoặc mỗi 1 s. Pool Redis 32; không có kết nối Redis riêng cho từng
  client. Hơn 64 entry tồn thì gộp `delta` trước khi ghi.
- Nhóm lane `legacy`: người gọi đọc key phòng lọc theo `inv`, nên mỗi `delta` chỉ ghi một lần.
  Thiếu key mà hàng Postgres đã kết thúc: dựng `xong`/`that_bai` từ hàng rồi đóng. Kiểm quyền lại
  mỗi 10 s trong transaction ngắn; hỏng → `thu_hoi` rồi đóng.
- Tối đa 180 s rồi `ket_noi_lai`. Khi dừng process, vòng stream nghe `chatCtx`: `stopChat()` chạy
  trước `Shutdown` (`main.go:301-305`), nên mọi stream kịp phát `ket_noi_lai` và đóng. Không cần
  `RegisterOnShutdown` như bản gốc (tự phát hiện).
- Sức chứa: 5 mỗi người, 2000 mỗi process, vượt thì 503 `stream_capacity`. Redis hỏng → 503
  `stream_unavailable` kèm `Retry-After`. Giới hạn mở 30 lần/phút/người (Redis, fail-open).
- `chat-capabilities` thêm `ai.stream: "room" | "requester" | "none"`, một tên trường thay cho
  `stream.available` và `ai.stream` của hai bản gốc (sửa theo phản biện). Thiếu trường = `none`,
  hợp với kiểu `ChatCapabilities` hiện có (`rudi/chat/ai-invocations.ts:5-21`).

### 5.3 Người xem khác trong phòng: frame `ai` trên WS sẵn có

Không mở SSE thứ hai: mỗi thành viên đã giữ một WS có auth, ack và nối lại (sửa theo phản biện).

- **Opt-in trong khung xác thực**: `{"type":"authenticate","token":"…","ai":true}`; server cũ bỏ qua
  trường lạ. Phải opt-in vì client cũ đóng socket khi gặp tin không phải trang
  (`useChatChanges.ts:55-66`), gửi `ai` cho nó là vòng đóng/mở vô tận. Không dùng `?ai=1` vì
  `query()` của server cũ từ chối tham số lạ (tự phát hiện). Kết nối xác thực bằng header không
  nhận frame `ai`.
- Frame: `{"type":"ai","inv":"…","tin":"<trigger_message_id>","so_tin":n,"id":"<redis id>","e":"…","d":{…}}`.
  `e` ∈ {`trang_thai`, `phan`, `delta`, `lam_lai`, `xong`, `that_bai`, `huy`}. Frame trang không có
  `type` nên phân biệt an toàn.
- **Không ack**: client không gửi gì cho frame `ai`; goroutine đọc ack của server giữ nguyên.
  Một goroutine bơm mỗi kết nối đã opt-in, ghi thẳng lên kết nối (`coder/websocket` v1.8.15 cho
  phép gọi `Write` đồng thời, `conn.go:30`, ngoài Git). Frame `ai` không đổi `after`.
- **Người vào muộn**: đọc key phòng từ đầu (entry ≤15 phút), mỗi `inv` chưa kết thúc gộp thành
  `trang_thai` mới nhất, các `phan` và một `delta` mỗi phần, rồi chạy tiếp.
- **Thu hồi**: vòng trang gọi `Store.Changes` (có `authorize`) mỗi 1 s; hỏng là đóng kết nối, bơm
  chết theo, thành viên bị rút mất chữ trong ~1 s. **v2**: `Store.Changes` trả thêm `lane`, tính
  trong cùng snapshot từ `chat_v2_conversations`; lane `v2` thì không bao giờ khởi động bơm (sửa
  theo phản biện). Dùng chung slot (1000, 5 mỗi người); hạn ghi 10 s, trễ là đóng kết nối.

### 5.4 Mobile

- `sse.ts` (~120 dòng): `fetch` với `AbortController` riêng, không dùng timeout 15 s của `send()`;
  `body.getReader()` qua `TextDecoder({stream:true})` nên ký tự UTF-8 bị cắt đôi vẫn đúng; phân tích
  đúng spec (`data` nhiều dòng, chú thích). Nối lại `min(15 s, 500·2^n)` + jitter như
  `useChatChanges.ts:74`. 401/403/404 thì dừng; 503 hoặc 3 lần hỏng thì về polling hôm nay. Vào nền
  thì huỷ; trở lại thì nối từ id cuối.
- **Nếp**: task và stream do `NepProvider` giữ trong RAM, không do bảng. Đóng bảng không giết câu
  đang chờ; xong thì báo bằng tờ thứ hai (`xong-viec`), và tờ đó ẩn ở màn tiền (sửa theo phản biện).
- **Phòng**: `useChatChanges` rẽ `type:"ai"` **trước** nhánh `busy` sang kho `useRoomAi`; bong bóng
  thành thẻ thật khi feed mang `message_id` tới; 30 s không có sự kiện thì bỏ. Phải chứng minh trên
  máy thật: flow Maestro trên emulator Android, mở ảnh chụp ra nhìn.

## 6. Giới hạn, ngân sách và latency

| Mục | Chốt |
|---|---|
| Hạn người | 8 lượt/phút/người, chung hai bot, trong Postgres (`handler.go:413-421`), không đổi, sống qua mất Redis |
| Trần lời gọi model | `MaxModelCallsPerTurn` (thiết kế 01). Đếm **mỗi job, qua mọi lần thử**: trước mỗi lời gọi, `UPDATE … SET model_calls=model_calls+1 WHERE id AND lease_id AND model_calls<$tran RETURNING`. 0 hàng thì không gọi. Chính xác cả khi worker chết (sửa theo phản biện) |
| Limiter theo lời gọi | Redis GCRA `rl:model:{model}`, `MOBILE_MODEL_RPM`, trong `llm/gemini.go`. Job giữ trước số lời gọi dự kiến lúc claim. Bị từ chối trước nội dung → `retryLater`; sau nội dung → chờ ≤2 s. Redis hỏng → fail-open (sửa theo phản biện) |
| Hạn phòng | 3 job chạy, 30/giờ (lát 7), đếm trong Postgres |
| Pool DB | `core work` có pool riêng `MOBILE_WORKER_DB_CONNS` (mặc định 10), cộng 2 kết nối chỉ cho heartbeat và `first_token_at`/`model_calls`. Semaphore tool DB ≤ pool/2. Tổng `serve` 15 + các worker + `api` dưới 100 (lý do của `db.go:32-34`) (sửa theo phản biện) |
| Khoá Gemini | Chỉ nạp ở process chạy worker; `serve` với `MOBILE_INPROC_WORKER=0` không cần nó (sửa theo phản biện) |
| SSE | 5/người, 2000/process, 180 s mỗi kết nối, 30 lần mở/phút/người |

| Latency (hợp đồng) | Mục tiêu | Phần của hạ tầng |
|---|---|---|
| Sự kiện trạng thái đầu (animation) | p95 ≤300 ms, đo từ lúc client nhận 202 | SSE: `trang_thai` dựng từ Postgres ngay sau auth. Phòng: cần broker khoẻ |
| Token chữ đầu | p50 ≤2.5 s, p95 ≤5 s | hàng ≤50 ms p95 (broker), ≤300 ms (poll); writer → client ≤150 ms p95; cửa sổ 48 ký tự nằm trong số này |
| Plan tổng | p95 ≤8 s | — |

## 7. Hỏng hóc

| Hỏng | Hành vi |
|---|---|
| RabbitMQ chết | Outbox dồn, không mất gì; worker về poll 250 ms; broker sống lại thì xả, trùng bị `claimByID` bỏ |
| Redis chết | Job vẫn xong trong Postgres. SSE trả 503 `stream_unavailable`, client poll. Writer no-op. `ai.stream="none"` |
| Worker chết trước nội dung | Không ack → broker giao lại; lease chặn nhận đôi; poller nhận lại trong ≤30 s |
| Worker chết sau nội dung | `failed/worker_interrupted`; chữ đã phát giữ nguyên + `that_bai`; nhóm bấm thử lại, Nếp hỏi lại (Nếp không có `/retry`) |
| Tin độc | 5 lần giao → `.dlq` → một dòng log cảnh báo chỉ id |
| Postgres chết | Consumer dừng; tin chờ trong queue |
| Thành viên bị rút | Trigger huỷ job; heartbeat dừng worker ≤5 s; SSE `thu_hoi` ≤10 s; WS đóng ~1 s |
| Guard chặn | Byte vi phạm không rời cửa sổ; stream kết thúc; phòng chỉ thấy `ai_tu_choi` |
| Log | Chỉ id, loại sự kiện, số đếm, độ dài byte, thời gian; không prompt, gói, chữ trả lời |

## 8. Triển khai

- `docker-compose.yml` thêm `rabbitmq` (4.x alpine, ghim digest lúc thi công, không publish cổng,
  `rabbitmq-diagnostics -q ping`), `redis` (alpine, ghim digest, cờ ở §3.5, `redis-cli ping`) và
  `worker` (`<<: *core-image`, `command: ["work"]`, chờ `migrate-chat`/`rabbitmq`/`redis` khoẻ,
  liveness `core healthcheck`, `--scale worker=N`). `core` có `MOBILE_REDIS_URL` và
  `MOBILE_INPROC_WORKER=0`. Vẫn **một mạng** (`docker-compose.yml:38-44`).
- Biến (`.env.example`): `MOBILE_AMQP_URL` (rỗng = chỉ poll), `MOBILE_REDIS_URL` (rỗng = không
  stream), `MOBILE_REDIS_NAMESPACE`, `MOBILE_INPROC_WORKER` (mặc định 1 tới khi deploy có worker),
  `MOBILE_WORKER_QUEUES`, `MOBILE_WORKER_CONCURRENCY_<QUEUE>`, `MOBILE_WORKER_DB_CONNS`,
  `MOBILE_AI_LEASE_SECONDS`, `MOBILE_MODEL_RPM`.
- `migrateChat` chạy `jobs.Migrate` trước `chatassist.Migrate` (`main.go:455-457`). `core work` từ
  chối khởi động khi thiếu schema hoặc version thấp, cùng phép kiểm với `serve`.
- CI: `scripts/go_broker_tier.sh` theo mẫu `go_postgres_tier.sh`. Container loopback PG + RabbitMQ
  + Redis, alembic, `core migrate-chat`, `go test -tags broker`, `CORE_REQUIRE_BROKER_TESTS=1`.
  Sentinel `TestBrokerTierReachesRabbitAndRedis`; **mọi SKIP là đỏ**. Thêm stage `go-broker` vào
  `STAGES` (`scripts/gate.sh:75`), một job `test.yml`, sửa `tests/test_gate_covers_every_inline_step.py`.

### 8.1 Vòng sửa 2 của lát 10: vận hành và rollout (review phản biện của `849664a`)

Lệch khỏi bản thiết kế ở trên, có chủ ý, kèm lý do:

- **Pool của `core work`.** Mỗi lượt định kỳ chạy trong transaction giữ khoá của nó (một kết nối);
  LISTEN của relay là kết nối riêng ngoài pool, mở bằng cấu hình của pool, đóng khi relay dừng.
  `MOBILE_WORKER_DB_CONNS` có sàn = `MOBILE_AI_WORKERS` + số tác vụ định kỳ (4) + 1 lượt xả của
  relay khi có broker (mặc định 7); thấp hơn thì từ chối khởi động và nói con số; trống thì
  `max(10, sàn)`. Ngoài pool: 2 kết nối heartbeat/bộ đếm, 1 kết nối LISTEN.
- **Tạm dừng vì DB không trả tin về hàng.** Đo trên RabbitMQ 3.12: `Nack(requeue)`, đóng channel,
  rớt kết nối khi tin chưa Ack đều cộng 1 vào `x-delivery-count`, tin bị dead-letter ở lần trả
  thứ 6 (`x-delivery-limit=5`, 6 lần giao). Trả tin về hàng mỗi lần tạm dừng thì năm lần DB kẹt
  khoá ngắn đẩy một tin lành vào DLQ. Nay consumer huỷ đăng ký, giữ tin đang cầm và tin đã được
  giao sẵn (chưa Ack, trên channel đó), chờ DB, chạy lại chúng rồi đăng ký lại. Chỉ khi process
  dừng giữa lúc tạm dừng thì channel đóng và tin về hàng, tính một lần. Chưa đo trên 4.x (ở 4.x
  nghĩa của `x-delivery-count` đổi). DB chết quá `consumer_timeout` của broker (mặc định 30 phút)
  thì broker đóng channel, tin về hàng tính một lần, Ket nối lại.
- **DLQ (§7).** Tin bị consumer đưa vào DLQ — thân không giải mã được, handler trả `ErrMalformed`,
  hoặc lần trả vượt giới hạn (đọc `x-delivery-count` broker đóng lên tin) — để lại đúng một dòng
  `job message dead-lettered` với `queue` và `id`. Id chỉ được lặp lại khi có đúng dạng relay viết
  (`<hàng>:<uuid>:<seq>`); tin không do relay phát thì `id` rỗng. Tin bị dead-letter vì process
  chết đúng ở lần giao cuối thì không ai ở lại để ghi dòng đó.
- **Ghi cuối hỏng vì DB.** Job đã claim mà ghi kết thúc hỏng (DB, không phải job) thì về hàng ngay
  (`queued`, `enqueue_seq` mới, lượt thử **đã tiêu**: lỗi lặp lại trên chính job vẫn bị chặn bởi
  `attempts<3`, kể cả đường brain mà `model_calls` không đếm). Job hết lượt thử hoặc đã có nội dung
  thì lease kết thúc ngay và sweep đặt `worker_interrupted` ở lượt kế. DB chết hẳn thì câu trả về
  cũng hỏng, job chờ hết lease như trước.
- **Phiên bản 5** dùng `DEFAULT now()` (ổn định, PostgreSQL ghi một lần vào catalogue, không ghi lại
  bảng dưới ACCESS EXCLUSIVE) và `SET LOCAL lock_timeout='5s'`: chờ khoá quá 5 s thì `migrate-chat`
  hỏng, chạy lại, thay vì xếp hàng sau một transaction dài và chặn mọi câu lệnh xếp sau nó. Đổi
  checksum của phiên bản 5 chỉ đụng database nháp trên nhánh này (chưa database chung nào cài).

Vòng sửa 3 (review phản biện của `3b1e819`):

- **Kết nối LISTEN của relay chết** (Postgres khởi động lại, failover, proxy cắt): `Run` trả lỗi ngay,
  Ket nghe lại trên kết nối mới sau 250 ms (nhân đôi tới 5 s, về 250 ms sau một lượt chạy ≥10 s)
  **trên cùng kết nối broker**. Không quay số lại: quay số lại trả mọi tin consumer đang giữ về hàng,
  mỗi tin tính thêm một lần giao, đúng điều vòng 2 đã bỏ. Channel của relay đóng thì vẫn quay số lại.
- **Tạm dừng lùi dần.** Trước mỗi lần chạy lại tin đang giữ, consumer chờ 250 ms, nhân đôi, tối đa
  30 s; đợt tạm dừng kết thúc khi mọi tin giữ chạy xong, đợt sau lại từ 250 ms. Một dòng
  `job consumer paused` mỗi đợt. DB trả ping nhưng từ chối mọi claim thì consumer thử vài lần mỗi
  phút thay vì ~1 000 lần/giây. Channel đóng giữa lúc tạm dừng (`consumer_timeout`, rớt kết nối) thì
  consumer trả lỗi và Ket nối lại, như gạch đầu dòng trên đã hứa.
- **Heartbeat dừng trước khi trả job**: một lần gia hạn đang kẹt cùng lỗi với câu ghi cuối không còn
  đè lên lease vừa kết thúc. Dừng chờ nhiều nhất lần gia hạn đang bay (≤2 s); không lần nào bắt đầu
  sau khi đã gọi dừng. Còn lại: một lần gia hạn đã hết giờ phía client (pgx đóng kết nối) vẫn có thể
  nằm chờ khoá phía server và chạy khi khoá nhả; nó xếp hàng trước câu trả job nên thực tế chạy
  trước, nhưng chưa có test chặn thứ tự đó.

Còn phải biết khi vận hành:

- **Nối lại broker chờ job dài nhất.** Kết nối broker rớt thì Ket chờ mọi job consumer đã bắt đầu
  xong (≤70 s, hạn của một job) rồi mới quay số lại. Trong lúc đó consumer đã tách (`Song()=false`)
  nên poller chạy nhịp 250 ms: job mới vẫn được nhận, chỉ với trễ của poll (§9: p95 ~220 ms).
- **Rollout từ `84e3c31`.** Replica còn chạy bản trước lát 10 (a) claim không xét `available_at`,
  `first_token_at`, `enqueue_seq`: nó bỏ qua backoff của `retryLater` (job thử lại sớm hơn 1 s/4 s)
  và — khi lát 11 đặt `first_token_at` — có thể nhận lại job đã có nội dung; (b) xử lý `/retry`
  không đặt lại `model_calls`: job được thử lại có thể chạm trần 8 lời gọi sớm và kết thúc
  `ai_het_ngan_sach`. Không mất job, không trả lời đôi (lease và trạng thái vẫn chặn). Cách làm: đưa
  mọi `serve` và `core work` lên cùng một lần, không để hai bản cùng chạy lâu; lát 11 không được
  bật `first_token_at` khi còn replica trước lát 10.

## 9. Cổng, test, canary, đột biến

**Đơn vị (`go test ./...`)**: bộ mã SSE (chèn xuống dòng, `data` nhiều dòng); parser
`Last-Event-ID`/`after`; gom `delta` và che log; golden topology; codec tin; backoff; `ServeHTTP`
không đặt deadline cho `/events`; `khoa(job)` trả key phòng cho đúng một tổ hợp. Cổng mới
`khong_noi_dung_len_broker_test.go`: `job_outbox` không có cột chữ; thân AMQP đúng `{v, ref, seq}`;
quét AST không lời gọi log nào trong `aistream`/`jobs` nhận `text`/`delta`/`chunk`/`payload`; ngoài
test chỉ `aiharness/guard` gọi `Sink.Delta` và chỉ `aistream` gọi `XADD`.

**`-tags postgres`**: trigger vào hàng khi tạo, `/retry`, `retryLater`, `release`, không khi
heartbeat, huỷ, xong; 100 × 2 `claimByID` đồng thời → đúng một người thắng; publish hỏng giữ
`published_at` NULL; poll tôn trọng `available_at`; huỷ bị phát hiện ≤6 s; `model_calls` không vượt
trần dưới 8 luồng tranh nhau; hai trigger là đường ghi duy nhất vào `job_outbox` (đọc `pg_trigger`);
các bộ `ProcessOne` cũ vẫn xanh.

**`-tags broker`** (executor stub tất định, 0 lời gọi model thật, ADR-0034 §2.6):
- Đầu-cuối: tạo → outbox → relay → consumer → 50 `delta`; client SSE qua CORS thật nhận đúng thứ
  tự; `xong.message_id` = hàng `messages`. Nối lại giữa chừng trả đúng phần đuôi.
- Phòng: 3 thành viên nhận frame `ai`; client không opt-in nhận 0 frame; người ngoài 403; thành
  viên bị rút mất frame ≤2 s. `docker pause` RabbitMQ trong 50 job → đủ 50; `docker pause` Redis →
  job xong, SSE 503. Tin độc vào DLQ. SIGTERM giữa job vẫn đăng đúng một lần. TTL ≤
  `share_expires_at`, và ≤120 s sau sự kiện kết thúc.

**Canary** (đỏ ở chỗ đã dự đoán; bản gốc giữ nguyên là identity, xanh):
1. Relay đặt `published_at` trước confirm. Kịch bản: tạm unbind `ai.nep` (publish `mandatory` bị
   trả về), 50 job, bind lại, **poller tắt**. Đỏ ở bước «50/50 xong». Bản gốc để poller bật, và
   poller sẽ cứu job, che mất canary (tự phát hiện).
2. Job lane `v2`: sau cả lượt, `XRANGE ai:room:{context}` rỗng. Canary: `khoa()` bỏ nhánh v2 → đỏ.
3. Stub model phát một số điện thoại giữa câu: quét mọi key Redis thấy 0 byte của câu đó. Canary:
   writer nhận chữ thẳng từ model, bỏ cửa sổ → đỏ.

**Đột biến** (kiểm tương đương trước; mỗi lát chọn ít nhất hai):
- M1: trigger **BEFORE** tăng `enqueue_seq` ở mọi UPDATE → đỏ ở «heartbeat không tạo dòng outbox».
  Chỉ bỏ điều kiện ở trigger AFTER là **tương đương**, vì `UNIQUE` nuốt dòng trùng; không dùng.
- M2: nối lại với `after` bao gồm → đỏ ở «không lặp sự kiện đầu». M3: bỏ `EXPIREAT` → đỏ ở ca TTL.
  M4: bỏ miễn timeout 8 s → đỏ trên stub stream 12 s.
- M5: `release` giữ `enqueue_seq` cũ → đỏ ở «job nhả khi SIGTERM được nhận lại ≤1 s, poller tắt».
- M6: nhận lại lease hết hạn mà không kiểm `first_token_at IS NULL` → đỏ ở «worker chết sau nội
  dung: không phát lại chữ, có `that_bai{worker_interrupted}`».
- M7: bơm frame `ai` không cần opt-in → đỏ ở «client không opt-in nhận 0 frame».

**Số đo cho commit message** (stub; tải bằng cờ mới `-mode ai-stream` của `cmd/chat-load`):

| Chỉ số | Mục tiêu |
|---|---|
| Trễ hàng p95, có broker / poll | ≤50 ms / ≤300 ms |
| Sự kiện trạng thái đầu p95 (202 → client) | ≤300 ms |
| Hạ tầng tới `delta` đầu p95 (writer → client) | ≤150 ms |
| Phòng 100 người nghe × 20 lời gọi đồng thời, trễ p95 | ≤200 ms |
| Tin mất / trùng tới client | 0 |

TTFT với Gemini thật chỉ đo khi Lead duyệt số lời gọi. Mobile: `tests/sse-doc.test.mjs` (cắt giữa
dòng, cắt giữa ký tự UTF-8, nối lại, 503 → polling) và flow Maestro có ảnh chụp đã mở ra xem.

## 10. Lát (theo số của kế hoạch)

| Lát | Phần của mảng này |
|---|---|
| 0 | Tài liệu này + đề xuất ADR-0038 |
| 4 | `claimNext`/`claimByID`/`process`, heartbeat 5 s, lease 30 s có điều kiện, `core work`, `MOBILE_INPROC_WORKER`, pool riêng + semaphore, sweep ở cả hai process. Tầng: postgres |
| 5 | Cổng đọc xuyên gói khai `job_outbox` cho gốc Nếp và gốc nhóm |
| 10 | Gói `jobs`, migration §3.1–3.2, relay, consumer, DLQ, `retryLater`, poller, registry định kỳ, limiter theo lời gọi, `model_calls`; compose `rabbitmq`, `worker`; tầng broker + job CI |
| 11 | `aistream`, Redis, hai route SSE + manifest, `sse.ts`, task Nếp trong `NepProvider`; compose `redis` |
| 12 | Frame `ai` opt-in trên WS, `lane` trong `Store.Changes`, `useRoomAi`, số đo fan-out |
| 15, 17 | Consumer `memory` (`nep_trich`) và `notify` (push) theo hợp đồng ở §4 bước 11 |
| 18 | Tầng T4: latency qua HTTP/SSE |
| 20 | Lane v2: chỉ `ai:inv`, phòng nhận chỉ metadata qua kênh chat v2 |

## 11. Quyết định còn mở

1. **Host production** cho RabbitMQ và Redis: managed hay tự chạy; TLS; ACL. Repo chưa mô tả host nào.
2. **Hợp đồng chưa ghi hai trường phong bì của frame phòng**: `tin` (neo hàng «đang trả lời» vào
   tin tag) và `so_tin` (cho «đang đọc {n} tin…»). `trang_thai{cau}` không có chỗ cho `n`. Đề xuất
   giữ chúng ở phong bì WS, ghi vào `03-ai-engine-hop-dong.md` khi Lead ký.
3. **Mục tiêu 300 ms cho người xem trong phòng khi broker chết**: poll 250 ms cộng claim có thể vượt.
   Đề xuất chế độ suy giảm ≤600 ms, ghi rõ là suy giảm.
4. **Kênh metadata cho phòng v2** (lát 20): `chatv2http` chưa có sự kiện tạm nào (kiểu typing).
   **Giá trị `MOBILE_MODEL_RPM`**: cần hạn mức thật của khoá Gemini từ Lead.
5. **Dùng chung Redis với chat-lab** (`RUDI_CHAT_REDIS_*`) hay tách: đề xuất chung, khác namespace.
