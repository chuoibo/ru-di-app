# Kiểm chứng transport chat v2

ADR-0031. Đây là cổng transport thử nghiệm, **chưa phải chat E2EE production**.
Chữ ký envelope chứng minh credential ký bytes; không chứng minh bytes là MLS.
Không có route tự cấp thiết bị hoặc tự bật `ready`. Chỉ test fixture tổng hợp
được provision trạng thái này; native enrollment/rekey còn phải triển khai.

## Những gì hiện thực

- PostgreSQL cấp sequence theo conversation, dedup logical send và ghi event +
  outbox cùng transaction. NOTIFY chỉ đánh thức; đọc lại event vẫn là nguồn thật.
- Thiết bị bị thu hồi/thành viên rời không được replay/catch-up. Thay đổi roster
  vô hiệu hoá epoch; không nhận tin mới tới khi rekey đã được kiểm chứng.
- HTTP gửi/đọc event, cập nhật mark; WebSocket phân trang có ACK và cursor resume.
  Client chỉ ACK sau khi lưu bền vững ciphertext/trạng thái crypto. ACK ở
  transport không tự coi là đã đọc; mark đọc là thao tác riêng.
- Tái kiểm session cả khi socket im lặng; một page đang bay, thời hạn ACK,
  giới hạn connection và Origin. Không có token trên URL hoặc log nội dung.
- Nhiều replica nhận pg_notify; reconcile mỗi giây khép lỗ mất thông báo.
  Redis fanout và load 1.000 connections vẫn là lát tiếp theo, chưa được chứng minh.

## Wire thử nghiệm

Các route chỉ trên `chat-lab`, không có trên public `core`:

- `POST /v2/chat/{conversation}/events`: envelope gồm `conversation_id`,
  `device_id`, `logical_send_id`, `protocol`, `epoch`, `ciphertext`, `signature`.
  Bytes JSON dùng base64; protocol `rudi-chat-v2-mls`. 201 lần đầu, 200 replay.
- `GET /v2/chat/{conversation}/events?device_id=...&after=...&limit=...`:
  `events`, `next_sequence`, `has_more`; limit tối đa 100 và tổng ngân sách
  page 512 KiB (kể cả base64 và dự phòng metadata). Tin vượt ngân sách sang
  page tiếp theo; cursor không nhảy qua tin chưa trả.
- `PUT /v2/chat/{conversation}/marks`: `device_id`, `kind`, `sequence`.
- `GET /v2/chat/{conversation}/stream?device_id=...&after=...`:
  WebSocket Bearer, subprotocol `rudi-chat-v2-mls`; frame `ready`, rồi `events`
  chứa `page`. Client trả `{"type":"ack","sequence":...}` đúng cuối page.
  Chậm quá 10 giây hoặc ACK sai thì ngắt, reconnect bằng cursor đã lưu.

## Cách chạy an toàn

Cổng độc lập tự dựng image từ checkout, tạo PostgreSQL 16 tạm trên loopback,
migrate baseline, chạy race detector và xoá container sau khi kết thúc:

```bash
bash scripts/chat_v2_postgres.sh
```

Script từ chối kết quả thiếu test sentinel hoặc có test bị bỏ qua. Không cần
EAS, dữ liệu app hoặc database dùng chung. CI hiện có cũng chạy các package
này qua `scripts/go_postgres_tier.sh` trong job Go; chưa có kết quả CI của nhánh.

Ngày 2026-09-21 đã chạy thành công script này trên checkout triển khai:
image dựng từ source hiện tại, migration legacy tới head, race tests đạt và
không có ca skip. Review độc lập transport/lab nhận APPROVE phạm vi nền này.
Ca nhiều replica chỉ dùng hai HTTP handler trong cùng process/pool; restart
chỉ tạo handler mới, chưa phải kill tiến trình. Reconcile cũng đang bật nên
không dùng ca đó để tuyên bố riêng kênh LISTEN đã được kiểm độc lập.

Test đơn vị và transport socket thật:

```bash
cd services/core
go test -race ./internal/chatv2 ./internal/chatv2http ./cmd/chat-lab
```

Postgres cần database test riêng đã qua Alembic legacy. Không trỏ vào `mobile`
hoặc stack dùng chung. Đặt `CORE_TEST_DATABASE_URL` ngoài Git và chạy:

```bash
cd services/core
CORE_REQUIRE_POSTGRES_TESTS=1 go test -race -tags postgres -count=1 ./internal/chatv2 ./internal/chatv2http
```

`chat-lab` bắt buộc `RUDI_CHAT_LAB=1`, `RUDI_CHAT_LAB_DATABASE_URL` có tên DB kết
thúc `_test`/`_lab`, listen IP loopback. `go run ./cmd/chat-lab migrate` chạy SQL
additive đã ghim checksum; `go run ./cmd/chat-lab serve` phục vụ lab. Không mở
cổng Internet hoặc dùng cho người thật. Migration không tự chạy khi serve.

## Chưa chứng minh

MLS trên Android/iOS, private-key storage, enrollment đáng tin cậy, rekey,
backup/restore, native E2E, ảnh/voice mã hoá, AI consent/job, Redis worker,
push thật, tải/soak, và người dùng thực chưa được các test này chứng minh.
