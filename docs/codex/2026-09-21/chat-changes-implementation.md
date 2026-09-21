# Feed thay đổi chat legacy trong candidate Go

Ngày 2026-09-21. Đây là bản sửa hồi quy của chat legacy đang chuyển đổi, không phải
bật chat v2 plaintext. Không sửa manifest ownership production; không đổi backend
nghiệp vụ sang Python; không tự áp dụng migration lên database đang dùng.

## Hợp đồng đã triển khai

- `GET /contexts/{id}/changes?after=N&limit=100`: `context_id`, `changes`,
  `next_sequence`, `watermark`, `has_more`. Mỗi change có `sequence`,
  `type: message|vote`, `entity_id`, `revision`.
- Sequence liên tục theo **từng conversation**, chỉ tính transaction đã commit.
  Revision là sequence cuối cùng sửa đối tượng, không phải bộ đếm riêng của tin.
- `POST /contexts/{id}/changes/snapshot`: `{message_ids?, vote_ids?}`, tổng tối đa
  100 ID. Body `{}` lấy 50 tin gần nhất và các poll trong thẻ. Trả `messages`,
  `votes`, `watermark`; từng đối tượng có `revision`. Nội dung/reaction/quote dùng
  serializer hiện hữu; `mine` và `my_option_id` tính riêng theo người đọc.
- Snapshot dùng một transaction REPEATABLE READ, cùng watermark và projection.
  Messages/reactions/quotes/vote options/ballots được đọc theo batch, không một
  câu SQL cho từng ID. Không trả bản ghi của context khác. Tin xoá trả tombstone;
  poll bị xoá vật lý trả `{id,context_id,revision,deleted:true}`.
- WebSocket `/contexts/{id}/changes/stream?after=N`: header Bearer cho native;
  browser gửi frame `{type:authenticate,token}` trong 5 giây. Không nhận token
  trong URL. Server trả Page; client ACK `{type:ack,sequence:next_sequence}` sau
  khi áp dụng/lưu. Trang đầu rỗng cũng cần ACK. Deadline ACK 10 giây; tối đa
  1.000 kết nối, 64 chờ xác thực, 5 kết nối/tài khoản. Socket đóng khi rảnh phải
  giải phóng slot ngay. Origin được kiểm độc lập với CORS.
- Cursor của feed chỉ tiến theo trang change đã xử lý; không nhảy theo ACK gửi
  tin hoặc watermark của snapshot hydrate một số ID.

## Tính nguyên tử và quyền

SQL capture trigger bao phủ INSERT/UPDATE/DELETE của messages, reactions, votes,
options, ballots, kể cả poll/card do bot ghi trong transaction hiện hữu. Counter,
change và outbox commit hoặc rollback cùng nghiệp vụ. NOTIFY chỉ đánh thức;
reconcile PostgreSQL bù thông báo bị mất. Outbox không có cờ “mọi máy đã nhận”.

Candidate Go khóa head conversation trước khóa message/vote. Những transaction
Go tạo poll + card hoặc sửa nhiều tin cùng nhóm dùng cùng thứ tự này. Trigger
vẫn bắt được writer legacy nhưng không tự áp đặt thứ tự khóa lên mọi transaction
Python lịch sử: transaction legacy tuỳ ý sửa nhiều đối tượng có thể bị PostgreSQL
huỷ vì deadlock. Không dùng bằng chứng candidate để tuyên bố mọi writer cũ đã qua
cổng cutover.

Mỗi trang/snapshot kiểm session hiện hành, tài khoản, membership active và DM
block/deleted peer trong một `REPEATABLE READ, READ ONLY` snapshot mới. Không
cache quyền theo TTL. Snapshot đầu tiên là điểm tuyến tính hoá: revoke/erase
đã commit trước đó bị từ chối; revoke đồng thời có thể xong trước khi trả trang
cũ, nhưng trang đó không thấy tin hoặc thay đổi commit sau snapshot. Lần đọc
kế tiếp dùng quyền mới. Thời hạn session được kiểm bằng `clock_timestamp()`
ngay trước commit đọc, không dùng `now()` cố định ở đầu transaction.

Bản đầu giữ `FOR SHARE` tới cuối đọc. Review bổ sung thấy thứ tự đọc DM
membership của peer → person của peer có thể tạo chu kỳ với xoá tài khoản
person → membership. Bản snapshot chỉ đọc bỏ chu kỳ này và không tạo WAL
khoá hàng cho từng người nhận. Hai regression PostgreSQL mới giữ reader tại
ranh giới đó, commit xoá peer rồi thêm tin tương lai, kiểm reader cũ chỉ thấy
tin cũ / reader mới từ chối; ca thứ hai giữ snapshot qua hạn session và
buộc từ chối trước handoff. Điều kiện này dành cho compatibility legacy,
không thay hợp đồng MLS/epoch/native.

Khác biệt sửa lỗi với legacy: AddReaction chỉ nuốt unique conflict đúng
`uq_message_reactions_one_per_kind`; CHECK/FK/outbox lỗi phải trả lỗi, không được
báo thành công như một reaction lặp. Test chèn CHECK lỗi vào outbox đã bắt được
nhánh nuốt toàn bộ SQLSTATE 23 trước khi sửa.

Candidate chặn `/plan`, mention và `/chia-bill` tự đọc lịch sử trong route gửi tin,
trả `explicit_invocation_required`. Các entry point AI cũ được handler chatassist
chặn; giao diện dùng invocation mới có đồng ý chia sẻ prompt cụ thể.

## Chạy và bằng chứng

Bật bằng `MOBILE_CHAT_CHANGES_CANDIDATE=1`, auth prod và toàn bộ Go candidate
messages/votes/outings. Thiếu route hoặc migration thì core từ chối khởi động.
`core migrate-chat-candidate` áp dụng Go/SQL rõ ràng; serve không tự chạy DDL.
Migration có checksum; downgrade chỉ bỏ metadata/trigger feed, giữ messages/votes.

Stack tổng hợp riêng: `bash scripts/chat_e2e_stack.sh up`; script áp dụng migration
candidate trước khi core phục vụ, PostgreSQL bật durability. Có `restart-core`
để dựng lại binary trong cùng stack, và `down` để dừng đúng container đã tạo.
Thông tin kết nối, token và media tổng hợp nằm dưới `/tmp`, không vào Git.

Các ca PostgreSQL/race trực tiếp bao gồm reaction/delete/poll/close, projection
riêng người đọc, rollback, replay reaction, migration lên/xuống/lên, người ngoài,
ID khác context, leave/session revoke, 20 writer đồng thời, head chưa commit,
transaction nhiều tin ngược thứ tự + bot poll/card, WebSocket Origin qua CORS,
first-auth/reconnect, socket rảnh đóng không rò slot, DM block/deleted peer và
outbox lỗi không commit nghiệp vụ. Lệnh tái chạy:

```sh
scripts/go_postgres_tier.sh -- ./internal/db ./internal/chatlegacychange -run TestPostgres -race
```

Tier trên tắt fsync để kiểm semantics; **không** dùng kết quả của nó làm bằng
chứng latency/durability. Kiểm tải và E2E browser/native là các cổng riêng. Root
kiểm lại với người dùng tổng hợp và review độc lập trước khi chốt checkpoint.
