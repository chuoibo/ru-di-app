# Review độc lập: lời gọi AI, chốt hẹn và outbox

Reviewer: agent `chat_v2_perf`, không viết implementation `chatassist`,
`brain/client.go` hoặc `chatbus`. Ngày 2026-09-21.

**APPROVE trong phạm vi candidate Go legacy được kiểm dưới đây.** Đây không
phải phê duyệt release, giao thức MLS, native hoặc ngưỡng tải production.

## Phạm vi và kết quả

- Payload inference chỉ có lời gọi được chia sẻ và catalogue công khai. Không
  đọc transcript, danh sách thành viên, gu hoặc lịch sử chuyến đi. HTTP response
  của job không trả prompt hay digest; thành viên khác không đọc được job riêng.
- Idempotency giữ một job cho cùng logical ID và cùng input; input khác bị từ
  chối. Publication của AI card và terminal state nằm trong cùng transaction.
  Chốt hẹn ghi outing, stops và mapping nguồn cùng transaction, có thử đồng thời
  và rollback; không có nghiệp vụ tiền tự động do AI quyết định.
- Quyền được kiểm trước gọi capability, trước inference và trước publication.
  Person đã xóa, session bị thu hồi, membership đã rời không tiếp tục có quyền;
  rời rồi vào lại không khôi phục lượt chia sẻ cũ. Thứ tự khóa là person →
  session → membership → context; không giữ khóa DB qua inference.
- Worker có deadline, lease và số lần thử hữu hạn. Lease hết hạn nhưng chưa
  được worker khác nhận vẫn không được dispatch/publish. Cancel không thể thu
  hồi byte đã gửi ra inference; nó ngăn publication tiếp theo.
- Relay chỉ mang metadata. PostgreSQL là nguồn event; outbox checkpoint không
  phải bằng chứng đã giao tin. Broker lỗi giữ pending để thử lại; replica còn
  reconcile PostgreSQL. Khi broker lỗi chưa có cam kết đạt ngưỡng 800 ms.
- Các phát hiện review về deleted identity, deadline DB, lease expiry và probe
  capability trước authentication đã được tác giả sửa; regression tương ứng
  được chạy lại độc lập.

## Bằng chứng độc lập

Chạy PostgreSQL 16 dùng một lần, migration từ checkout hiện tại, Go `-race`,
`CORE_REQUIRE_POSTGRES_TESTS=1`, `GOMAXPROCS=2`, `nice -n 19`. Redis 7 chạy bằng
container riêng do test tạo. Không dùng shared database; không có SKIP.

Kết quả: **8 test AI và 2 test relay PASS**. Log ngoài Git:
`/tmp/chat-ai-bus-independent.log`; runner tạm:
`/tmp/chat-ai-bus-independent.sh`.

Hai regression mới được reviewer yêu cầu và đã tự chạy:
`TestUnauthenticatedCallsNeverReachInferenceCapability` và
`TestExpiredUnclaimedLeaseCannotDispatchOrPublish`.

Inference trong suite này là HTTP fixture; không dùng kết quả này để tuyên bố
provider thật hoặc chất lượng gợi ý đã đạt. Bằng chứng UI/provider của tác giả
là cổng riêng, không được ký thay trong review này. Cutover v2 sau này phải giữ
khóa context khi chuyển quyền writer; candidate hiện tại từ chối room đã v2.

SHA-256 của source tại lần kiểm:

```text
chatassist/handler.go c7306bb85a0af99bda2d6bc062285bbbc7c2bf20b25a0f38512e318ab46489e1
chatassist/worker.go fced775d38690b607999bd7b8011838ad807863133d179faf5ed582b5dd21225
chatassist/promotion.go ed8a3a959c8321359e2c8d58f33743156faf7b18d0b33a252db04affc1c1331e
chatassist/migrate.go a50dec12ed24c39e4d354c925416e93b35679a29c923b41b3a6fa3c4b9adcf47
chatassist/schema.sql c537bc1abfabd176dcfa4e11cfc7343ce4b3a3cf4e24e9f66ece896407e5ad63
chatassist/postgres_test.go 31be3c7902f22f29c07e98e6e3eb60bba74537cb6bc62852917918e8a1791243
brain/client.go 2270904230ca340f5894ed87fc7a5fa128635d588137a74d402993e60217ef09
chatbus/redis.go ff1dfae7a97c83977e5579eb382bced7a66114b61f5f2f33f3e8786eee63ad22
chatbus/redis_postgres_test.go 754102f97b17bc566c54b4f2eacbc0b47875aebdd43846ea661e996728251538
```
