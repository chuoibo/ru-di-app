# ADR-0038 — Hàng đợi Go trên RabbitMQ, chữ chạy qua Redis Streams; Postgres vẫn là sự thật

- Ngày: 2026-09-25.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0038 thì
  lấy số trống kế tiếp và sửa mọi chỗ dẫn chiếu trong cùng commit.
- Quyết định sản phẩm: người dùng chốt trong phiên 2026-09-25. Hàng đợi là Go + RabbitMQ + Redis,
  không Celery. Câu trả lời chữ stream qua SSE. Phòng lane cũ: cả phòng thấy chữ chạy. Phòng
  E2EE v2: chỉ người gọi thấy.
- Thiết kế chi tiết: `docs/claude/2026-09-25/thiet-ke-ai/02-hang-doi-va-streaming.md`. Hợp đồng
  chung, thắng khi lệch: `docs/architecture/03-ai-engine-hop-dong.md`.
- Sửa ADR-0031 §2 và ADR-0036 §3 (xem mục 5). **Không sửa văn bản** của các ADR đó.
- Trạng thái triển khai: lát 4 đã vào main (`d596621`: `core work`, heartbeat, `ClaimByID`, sweep
  riêng); hàng đợi, outbox, Redis và SSE chưa có code, chưa có bằng chứng.

## 1. Bối cảnh

Engine AI (`services/core/internal/chatassist`) chạy worker **ngay trong process `serve`**: hai
goroutine, nhịp 250 ms, nhận việc bằng `FOR UPDATE SKIP LOCKED` với lease 75 s
(`worker.go:31-50`, `:77`). Client biết kết quả bằng cách poll mỗi 2 s. Mọi route của engine chịu
timeout 8 s (`handler.go:83`), nên không route nào giữ được một stream. Repo chưa có SSE nào và
chưa có RabbitMQ. Redis có trong `go.mod` nhưng chỉ chat-lab dùng.

Người dùng muốn ngữ nghĩa kiểu ChatGPT: gửi, nhận task id, làm việc khác, rồi nhận kết quả hoặc
stream theo id. Trước chữ đầu tiên phải có animation «đang nghĩ». ADR-0036 §3 từng viết «cách chữa
rẻ là nâng số worker, không phải tách hàng đợi». Câu đó đúng với một bot gọi model một lần. Nó hết
đúng khi hai bot, mỗi lượt vài lời gọi model, cùng tranh một pool 15 kết nối DB (`db.go:32-34`)
với API.

Ràng buộc không đổi: Postgres là nguồn sự thật. ADR-0031 §2 viết «mất Redis không được làm mất
event đã ACK». Python chỉ làm AI (ADR-0031 §1, CLAUDE.md).

## 2. Quyết định

1. **Việc là hàng trong Postgres; outbox không mang nội dung.** Mỗi job là một hàng trong bảng việc
   của gói sở hữu. Với AI, đó vẫn là **một bảng** `chat_ai_invocations`. Mỗi lần job chuyển sang
   `queued` (tạo, `/retry`, thử lại tự động, nhả khi tắt), trigger tăng `enqueue_seq` đơn điệu và
   chèn `(queue, ref_id, enqueue_seq, available_at, expires_at)` vào `job_outbox` trong cùng
   transaction. Bảng này không có cột payload.
2. **RabbitMQ chỉ để giao việc nhanh, không giữ sự thật.** Relay Go publish với `mandatory` và
   publisher confirm, và chỉ đặt `published_at` sau confirm. Có quorum queue theo hàng: `ai.group`,
   `ai.nep`, `memory`, `notify`. Mỗi hàng có DLQ và `x-delivery-limit=5`. Ưu tiên có được nhờ tách
   hàng, không nhờ message priority. Thân tin chỉ là `{v, ref, seq}`. Nạp dữ liệu là lệnh CLI,
   không phải một hàng.
3. **Poller dự phòng luôn chạy.** `claimNext` (`FOR UPDATE SKIP LOCKED`) cứu tin lạc và lease hết
   hạn mỗi 2 s. Khi AMQP mất hoặc không cấu hình, nó chạy mỗi 250 ms. Mất RabbitMQ không mất job
   nào đã nhận 202, chỉ chậm hơn.
4. **Worker là một process Go riêng: `core work`.** Nó nhân bản ngang. Mỗi hàng có consumer riêng,
   có heartbeat 5 s và lease 30 s. Lease chỉ hạ từ 75 s khi mọi process đã có heartbeat. Worker có
   pool DB riêng, hai kết nối dành riêng cho heartbeat và cho `first_token_at`/`model_calls`, và một
   semaphore cho tool DB. `serve` chỉ chạy
   worker trong process khi `MOBILE_INPROC_WORKER=1`. Các sweep giới hạn plaintext chạy ở **cả
   hai** process, để trần 15 phút vẫn giữ khi worker co về 0.
5. **Thử lại tự động chỉ trước nội dung đầu tiên.** Điều kiện: lỗi tạm thời, chưa có `phan` hay
   `delta` nào rời worker (`first_token_at IS NULL`), `attempts<3`, còn trong 30 s đầu. Job đã phát
   nội dung mà worker chết thì kết thúc `worker_interrupted`, không chạy lại từ đầu. Mỗi job có
   **một** trần lời gọi model (`MaxModelCallsPerTurn`), đếm nguyên tử trên hàng job qua mọi lần
   thử, kể cả retry do `aiharness/llm` tự làm (retry riêng của genai bị tắt, ADR-0037 §2.5).
6. **Redis có thêm một vai: stream chữ tạm thời (Redis Streams).** Chỉ worker ghi, qua gói
   `aistream`. Mọi key có TTL không quá cửa sổ chia sẻ. Redis không persistence (`--save ""`,
   không AOF). Mất Redis chỉ mất chữ đang chạy. Kết quả cuối vẫn commit trong Postgres, và client
   rơi về polling.
7. **Hai đường tới người xem.** Người gọi và Nếp dùng SSE:
   `GET /contexts/{c}/ai-invocations/{id}/events` và `GET /me/nep/ai-invocations/{id}/events`.
   Hai route này miễn timeout 8 s, không giữ kết nối DB khi stream, nối lại bằng `Last-Event-ID`
   hoặc `?after=`. Người xem khác trong phòng lane cũ nhận **frame `ai` trên WS
   `chatlegacychange` sẵn có**. Frame này phải opt-in trong khung xác thực, để client cũ không bị
   vỡ. Không mở SSE thứ hai cho phòng.
8. **Một enum sự kiện đóng.** `hello`, `trang_thai{cau}`, `phan{kind,json}`, `delta{p,text}`,
   `lam_lai`, `xong{message_id|text,chips,nguon}`, `that_bai{code}`, `huy`, `thu_hoi`, `ket_noi_lai`,
   cộng `: ping`. **Không có «rút lại».** Output guard, với cửa sổ 48 ký tự, là nơi duy nhất sinh
   `delta`. Frame phòng không bao giờ mang mã guard chi tiết.
9. **Phòng E2EE v2 không bao giờ nhận chữ từ máy chủ.** `lane` do máy chủ suy từ
   `chat_v2_conversations`. Writer không bao giờ ghi key phòng cho job v2. WS phòng v2 không bao giờ
   bơm frame `ai`.
10. **Hạn mức.** Trần 8 lượt/phút/người đếm trong Postgres, giữ nguyên, và là nguồn có thẩm quyền.
    Redis thêm hai thứ, cả hai fail-open: limiter theo **từng lời gọi model**, và giới hạn mở SSE.
11. **Không nội dung trên hạ tầng.** Broker, outbox, DLQ và log chỉ mang id, enum, số đếm, độ dài
    byte, thời gian.
12. **Migration.** Số version cấp theo thứ tự vào main, không đặt trước. Gói `jobs` có bảng version
    riêng theo mẫu `chatassist/migrate.go`. `serve` và `work` từ chối chạy khi version thấp hơn mức
    cần.
13. **Celery vẫn bị loại.** Python không tiêu thụ hàng nào và không chạy việc nền nghiệp vụ.

## 3. Hệ quả

- Hai dịch vụ mới cần host production: RabbitMQ và Redis. Repo chưa mô tả host nào; Lead phải
  quyết. Đến lúc đó `MOBILE_AMQP_URL` rỗng nghĩa là chỉ poll, `MOBILE_REDIS_URL` rỗng nghĩa là không
  stream. Mọi thứ vẫn chạy, chỉ chậm hơn.
- Thêm một tầng test, `scripts/go_broker_tier.sh`, theo mẫu `go_postgres_tier.sh`: **skip là đỏ**,
  có sentinel riêng, có stage `go-broker` trong `scripts/gate.sh`.
- Trigger vào hàng được chọn vì nó phủ mọi đường sang `queued`, không phải để lách cổng đọc. Cổng
  đọc xuyên gói phải **khai rõ** `job_outbox` trong allowlist của gốc Nếp và gốc nhóm.
- Client WS mới phải rẽ frame `ai` trước nhánh áp trang. Client cũ không opt-in thì không thấy gì
  khác.
- Chữ chạy chậm hơn model khoảng 48 ký tự. Đổi lại, không byte vi phạm nào tới được cả phòng.
- Worker chết sau nội dung đầu tiên là lỗi thấy được: chữ đã phát, rồi `that_bai`. Chạy lại lặng
  lẽ sẽ là một lần «rút lại» trá hình.
- Bản chữ trong Redis sống không quá cửa sổ mà `result` vốn đã sống. ADR-0036 §2.8 và §4 không
  bị nới.

## 4. Cái này KHÔNG cho phép

- Không đưa prompt, gói ngữ cảnh hay chữ trả lời lên RabbitMQ, vào `job_outbox`, DLQ hay log.
- Không coi Redis hay RabbitMQ là nguồn sự thật. Không quyết định nghiệp vụ bằng trạng thái đọc
  từ Redis.
- Không bật persistence (RDB/AOF) cho Redis đang giữ key stream.
- Không ghi key phòng, không đẩy chữ, không đẩy mã guard tới phòng E2EE v2.
- Không «rút lại» chữ đã phát. Không `lam_lai` sau khi đã có nội dung. Không tự thử lại sau nội
  dung đầu tiên.
- Không mở kênh stream thứ hai cho người xem trong phòng.
- Không giữ kết nối DB trong suốt một stream.
- Không Celery, RQ hay worker Python cho việc nghiệp vụ. Không thêm hàng `ingest`.
- Không để một đường Go nào tự chèn `job_outbox` ngoài hàm của gói `jobs`.

## 5. Điều khoản ADR cũ bị thay hoặc sửa (văn bản gốc giữ nguyên)

| ADR | Điều khoản | Nay |
|---|---|---|
| ADR-0031 §2 | «Redis chỉ fanout, presence và rate limiting» | **Sửa.** Thêm vai thứ tư: stream chữ tạm thời, TTL ≤ cửa sổ chia sẻ, không persistence. «Mất Redis không được làm mất event đã ACK» giữ nguyên, và mở rộng: mất RabbitMQ không được làm mất job đã nhận 202. «API/worker riêng» nay có hình dạng cụ thể là `core work` |
| ADR-0036 §3, gạch cuối | «Engine có hai worker… Cách chữa rẻ là nâng số worker, không phải tách hàng đợi» | **Thay.** Worker là process `core work` riêng, nhân bản ngang. Mỗi hàng (`ai.group`, `ai.nep`) có consumer và concurrency riêng, **trên cùng một bảng** (giữ lập luận «một bảng» của `schema_scope.sql`) |
| ADR-0024 §5, hàng «Celery/RQ + Redis» | lý do «không có hạ tầng trong compose» | **Kết luận giữ nguyên, lý do đổi.** Sau ADR này compose có Redis, nhưng Celery vẫn bị loại, vì Python chỉ làm AI (ADR-0031 §1). Worker `push_pending` của ADR-0024 §2.2 sẽ thành hàng `notify` bằng Go khi ADR-0041 port push |

## 6. Cách kiểm chứng

- **Đơn vị**: bộ mã SSE, parser nối lại, codec thân AMQP đúng `{v, ref, seq}`. Cổng AST: chỉ guard
  gọi `Sink.Delta`, chỉ `aistream` gọi `XADD`, không log nào nhận chữ.
- **Postgres**: trigger vào hàng đúng bốn đường, không vào khi heartbeat, huỷ hay xong; 100 × 2
  `claimByID` đồng thời có đúng một người thắng; `model_calls` không vượt trần.
- **Broker**: đầu-cuối 50 `delta` đúng thứ tự. `docker pause` RabbitMQ hay Redis không mất job.
  Tin độc vào DLQ. SIGTERM giữa job vẫn đăng đúng một lần.
- **Canary**:
  1. Relay đặt `published_at` trước confirm. Chạy với **poller tắt** thì đỏ ở «50/50 xong».
  2. Job v2 để lại 0 byte trong key phòng.
  3. Câu vi phạm guard để lại 0 byte trong Redis.
- **Đột biến** (có đoán trước chỗ đỏ):
  - trigger BEFORE tăng seq ở mọi UPDATE: đỏ ở «heartbeat không tạo dòng outbox»;
  - nối lại bao gồm `after`: đỏ ở «không lặp sự kiện đầu»;
  - `release` giữ seq cũ: đỏ ở «job nhả được nhận lại ≤1 s»;
  - nhận lại lease không kiểm `first_token_at`: đỏ ở «không phát lại chữ».
- **Số đo** (executor stub, 0 lời gọi model thật, ghi vào commit message):
  - trễ hàng p95: ≤50 ms qua broker, ≤300 ms khi poll;
  - sự kiện trạng thái đầu p95: ≤300 ms;
  - phòng 100 người nghe × 20 lời gọi: trễ p95 ≤200 ms;
  - mất hoặc trùng: 0.
