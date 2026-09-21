# Review độc lập: snapshot quyền của feed legacy

Reviewer: agent `chat_v2_perf`, không viết `chatlegacychange`.
Ngày 2026-09-21. **APPROVE phần đổi transaction đọc sang REPEATABLE READ READ
ONLY và kiểm tra expiry tại handoff**, trong phạm vi candidate legacy; không
phê duyệt release hoặc SLO tải.

ACL, session, person, membership, peer của DM, trạng thái block, watermark và
mọi projection message/reaction/quote/vote đều đọc trong một snapshot mới.
Không tái dùng grant giữa các lần đọc. Revoke đồng thời có thể commit trước
khi trang cũ trả về; trang cũ không được chứa dữ liệu commit sau snapshot.
Lần đọc mới phải từ chối quyền đã mất. Đây là thay đổi điểm tuyến tính hóa
được ghi rõ, không phải giữ nguyên lời hứa revoke phải chờ reader commit.

`commitRead` dùng đồng hồ PostgreSQL để từ chối session đã hết hạn trong lúc
đọc, áp dụng cho cả Changes và Snapshot. Handler chỉ serialize kết quả sau
khi kiểm tra error; lỗi handoff không phát payload. Writer vẫn dùng khóa và
transaction riêng. Không còn đường khóa membership của peer trước person
của peer gây chu kỳ với account erasure.

Reviewer chạy độc lập toàn bộ package trên PostgreSQL 16 dùng một lần,
migration từ checkout hiện tại, Go `-race`, `CORE_REQUIRE_POSTGRES_TESTS=1`,
`GOMAXPROCS=2`, `nice -n 19`: PASS, không SKIP. Log ngoài Git:
`/tmp/chat-feed-snapshot-independent.log`; runner:
`/tmp/chat-feed-snapshot-independent.sh`.

Đã tự chạy hai canary mới: pause reader trước đọc peer, commit peer erasure
và message mới, kiểm trang cũ không thấy message tương lai và trang mới bị
từ chối; pause qua thời điểm session hết hạn, kiểm handoff bị từ chối.
Các test reaction/delete/poll, ACL, hai handler và outbox rollback cũng PASS.

```text
chatlegacychange/store.go c2afce55ed0c683d6adef49c04cd40a52efa3567e6217cf2ae97a10a514221e5
chatlegacychange/snapshot_postgres_test.go 3840d71aaaa632bebae7b01455ec3ceda34596eed36f46e94826c96263e8449f
```
