# Review độc lập: batch ciphertext và snapshot quyền

Reviewer: agent `chat_changes`, không viết implementation `chatv2`/`chatv2http`.
Ngày 2026-09-21. **APPROVE trong phạm vi code transport thử nghiệm được chốt dưới
đây**; không phải APPROVE release, MLS, native hoặc SLO tải 30 phút/burst/soak.

## Những điểm đã kiểm

- Batch REPEATABLE READ READ ONLY lấy session/person/device/membership incarnation,
  first sequence, epoch/ready, high-watermark và event trong cùng snapshot mới.
  Không tái dùng quyền của batch trước. Session hết hạn được kiểm lại bằng đồng
  hồ PostgreSQL trước bàn giao dữ liệu. Không gộp projection theo người đọc.
- Điểm tuyến tính hoá của **batch đọc** chuyển sang snapshot đầu tiên, thay cho
  yêu cầu revoke phải chờ đọc commit. Revoke đồng thời có thể commit trước khi
  trang cũ được trả, nhưng snapshot cũ không thấy event commit sau revoke. Test
  dừng reader trước event query, commit revoke + event mới, rồi chứng minh reader
  chỉ thấy event trước đó và lần đọc mới bị từ chối. Đổi nghĩa này phải ghi rõ
  trong ADR/tracker; write-side và REST read-side cũ vẫn giữ hợp đồng khóa.
- CTE appendEvent cập nhật sequence → INSERT event → INSERT outbox trong cùng
  transaction. Data-modifying CTE outbox chạy dù không được SELECT trực tiếp;
  NOTIFY chỉ phát lúc commit. Error decode/constraint làm transaction rollback.
- Dispatcher gộp theo conversation/cursor, không dùng high-watermark làm cache
  quyền. Wake và reconcile vẫn kích hoạt xác thực; ACK khi đã bắt kịp không tạo
  lại lượt đọc rỗng ngay. Mỗi encoding có reference count; cancellation và slow
  writer giải phóng phần được giữ, không giữ khóa DB trong khi chờ socket.
- Giới hạn 64 MiB áp dụng cho **frame đã encode**, không phải tổng RSS. Ngoài ra
  còn decoded window tối đa 2 MiB/call, 8 dispatcher slots và bootstrap riêng.
  Việc này chưa chứng minh giới hạn bộ nhớ dưới mọi dạng lịch sử/phân bố tải.

## Bằng chứng chạy lại

Reviewer chạy độc lập `bash scripts/chat_v2_postgres.sh` trên PostgreSQL dùng một
lần, migration từ checkout hiện tại, durability bật, Go `-race`.
Kết quả: 40 test cấp cao PASS, không SKIP. Log lần chốt nằm ngoài Git tại
`/tmp/chat-v2-independent-mvcc-cte.log`.

Bao gồm session snapshot/revocation/expiry, history boundary, sai session/device,
leave-rejoin không hồi sinh membership cũ, byte-bound, reconnect, hai process,
writer restart, committed retry, slow consumer và quyền trên socket đang rảnh.
Load generator, 30 phút, 300 tin/s và native không nằm trong verdict này.

SHA-256 tại thời điểm review:

```text
chatv2/batch.go       3cf42a65fa0cce57a84b75cafdaf5159c06c7411cd0dfd76d7c74cf744cbe2d6
chatv2/store.go       d1d86cc1364ab94e7feaf9106142e1b518954589d4d3130a7779f1c364631799
chatv2http/dispatch.go 440308cc53309a3079541332829f22f90f1338810d22fdcbe0f450f1a92a64a6
chatv2http/handler.go 1b05d1d4a7278a193c08e7b18a9b76dbe638b13dc82c9b7b63724937be36aab8
```
