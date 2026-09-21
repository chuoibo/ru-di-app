# Mô phỏng nhiều người qua realtime thật — 2026-09-21

**Đã bật hai tiến trình Go chat-lab, PostgreSQL 16 riêng, 200 tài khoản tổng hợp,
1.000 WebSocket thật. Có chạy gửi/nhận đồng thời. Tính toàn vẹn đạt trong lượt
60 giây; độ trễ chưa đạt ADR-0031. Chưa thể gọi chat production-ready.**

Đây là test transport Go v2, chưa nối vào app Expo. Envelope được ký Ed25519,
nhưng payload là bytes tổng hợp, **không phải bằng chứng MLS/E2EE**. Không phải
1.000 người thật hay 1.000 điện thoại. UI/voice/sticker/bot/native là các cổng
khác; báo cáo này không thay thế chúng.

## Lượt đo chính

Lệnh có thể chạy lại từ root, tự tạo và xoá database/container riêng:

```bash
bash scripts/chat_mass_realtime.sh -duration 60s -people 200 -devices 5 -rate 100
```

Cấu hình: hai nhóm, mỗi nhóm 100 tài khoản × 5 thiết bị; mỗi thiết bị có key
riêng; bearer session thật qua bảng `account_sessions`. Hai tiến trình server
có pool 15 connections mỗi tiến trình, HTTP/WS loopback, schema legacy migrate
thật trước schema chat v2. Không thay `fsync`/durability hoặc bỏ authorization.
Dữ liệu và server đều tổng hợp, không dùng stack/app/database chia sẻ.

| Chỉ số | Kết quả |
|---|---:|
| WebSocket mở cùng lúc | 1.000 |
| Thời gian tạo tải | 60 giây |
| HTTP gửi tin mới | 5.996, xấp xỉ 99,93 request/s |
| HTTP 201 / lỗi gửi / queue drop | 5.996 / 0 / 0 |
| Retry cùng logical ID trả 200 | 60/60 |
| Event / outbox / dedup trong PostgreSQL | 5.996 / 5.996 / 5.996 |
| Delivery kỳ vọng / nhận được | 2.998.000 / 2.998.000 |
| Thiếu / trùng / sai sequence / sai envelope | 0 / 0 / 0 / 0 |
| Ngắt có chủ đích rồi resume cursor | 100/100 |
| Rớt socket ngoài dự kiến / lỗi dial | 0 / 0 |
| Delivery p50 / p95 / p99 / max | 1.609 / 2.427 / 3.023 / 3.713 ms |
| HTTP receipt p50 / p95 / p99 | 1.251 / 1.856 / 2.324 ms |
| Thời gian gồm chờ HTTP cuối | 61,30 giây |

Mỗi message phải đến đủ 500 thiết bị của đúng nhóm. Receiver kiểm bytes và
metadata envelope, actor, conversation, sequence tăng liên tục, logical ID ↔
sequence đồng nhất giữa clients. Harness kiểm page rồi ACK để tiếp tục quan sát;
bất kỳ sai khác nào làm cổng đỏ dù lượt đo vẫn tiếp tục. Cursor ở RAM của
harness, **chưa phải kho crypto bền vững của điện thoại**.
Reconnect dùng cursor đã nhận, không bắt đầu lại từ đầu. Sau drain, đối chiếu
cursor từng thiết bị với sequence bền vững và tổng event/outbox/dedup.

Delivery latency tính từ trước HTTP request gửi envelope tới lúc client nhận
và kiểm envelope. Có cả thời gian reconnect/catch-up chủ đích. Lượt đo không
chèn network RTT/loss bằng `tc`; cùng host không mô phỏng mạng di động.

**Không đạt latency:** ADR-0031 yêu cầu delivery p95 ≤800 ms, p99 ≤2 giây.
Harness trả exit code 1 dù integrity/capacity đúng vì cả hai phân vị vượt ngưỡng.
Không chạy tiếp soak 30 phút/24 giờ hoặc burst 300 tin/s rồi gọi là đạt khi
baseline latency đã đỏ.

Bằng chứng chính: [JSON](chat-mass-realtime/1000-connections-100rps.json),
[progress](chat-mass-realtime/1000-connections-100rps-progress.log),
[source hashes và môi trường](chat-mass-realtime/1000-connections-100rps-provenance.txt),
[time -v](chat-mass-realtime/1000-connections-100rps-resources.txt).
Các số thực trong JSON lưu Git được làm tròn ba chữ số sau dấu thập phân;
các bộ đếm và phân vị mili giây giữ nguyên. Bản stdout gốc ở thư mục `/tmp`
của từng lượt đo, đường dẫn có trong provenance.

## Lỗi tìm thấy và sửa

`Store.Events` trước đây dùng cùng khóa `FOR UPDATE` trên conversation như
`Send`. Tất cả recipients tranh khóa ghi dù chỉ đọc catch-up. Trong lượt tải
đầu, `pg_stat_activity` thực tế có 20–23 backend đợi tuple lock và hai backend
đợi transaction ID. Đã đổi riêng catch-up sang `FOR SHARE`; gửi/mark vẫn dùng
`FOR UPDATE`. Khóa trên person/device/membership/incarnation và thứ tự khóa
không đổi. Shared conversation lock vẫn giữ epoch/ready/last_sequence ổn định
và chặn invalidation/writer đến khi read transaction kết thúc.

Regression PostgreSQL mới
`TestCatchupReadersShareConversationWithoutWeakeningRevocation` giữ read
transaction thứ nhất mở, chứng minh catch-up thứ hai chạy được, quan sát
revocation thực sự đợi lock qua `pg_stat_activity`, rồi xác nhận thu hồi và
đóng epoch sau commit. Mutant đổi catch-up lại sang exclusive gây đỏ với
`parallel catch-up blocked: context deadline exceeded`; không sửa test để xanh.

Đây là sửa tranh khóa đọc thật, **chưa khép cổng hiệu năng**. Sau sửa, sampling
chuyển từ tuple wait sang WALWrite/WALSync; có thể còn chi phí transaction và
row locks trên từng recipient. Đây là bằng chứng hướng điều tra, chưa phải
phân rã đầy đủ nguyên nhân p95. Cần profile và thử fanout/batching hoặc pool
riêng có kiểm authority; không được bỏ permission checks để giảm latency.

## Hiệu chỉnh phép đo và các lượt phụ

Hai lượt đầu dùng 32 HTTP workers, dẫn tới chính load generator chờ khi request
chậm. Vì vậy **không dùng mức 14–21 tin/s của hai lượt này để kết luận trần
throughput server**. JSON được giữ để không che lịch sử fail:

- [Trước sửa, generator bị giới hạn](chat-mass-realtime/initial-generator-limited.json):
  1.830 HTTP 201, 15 HTTP 503, 1.832 event bền vững. Hai event có outcome HTTP
  mơ hồ khi timeout; đều được fanout. 4.168 `send_errors` của schema JSON cũ
  gồm 4.153 queue drops và 15 lỗi HTTP, không phải 4.168 request tới server.
- [Sau shared lock, cùng generator cũ](chat-mass-realtime/shared-lock-generator-limited.json):
  2.040 HTTP 201, 31 HTTP 503, 3.929 queue drops. Không dùng như phép so sánh
  benchmark sạch vì host dùng chung và phân phối message theo từng nhóm.
- [1.000 connections, 10 tin/s](chat-mass-realtime/1000-connections-10rps-one-active-group.json):
  300/300 tin, 150.000 delivery, không lỗi/mất/trùng; p95 917 ms nên vẫn đỏ
  latency. Lượt phụ này chỉ một nhóm có tin do thứ tự sender của generator cũ.
- [Smoke 20 connections](chat-mass-realtime/smoke-20-connections.json):
  100 tin, 2.000 delivery, p95 66 ms, hai reconnect, không lỗi/mất/trùng.

Lượt chính đã sửa generator: 512 workers để bao phủ deadline server 5 giây ở
100 request/s, sender xen kẽ hai nhóm liên tục. Cả `offered`, `scheduled`,
HTTP statuses và queue drops đều được xuất; không lấy số cấu hình thay cho
số request thực sự chạy. Tick hụt bốn request được ghi rõ, không làm tròn thành
6.000. Cổng đòi đạt ít nhất 99% rate cấu hình và không queue drop.

## Tài nguyên và giới hạn bằng chứng

Máy Linux 16 logical CPUs, RAM khoảng 20 GiB, có emulator và các tác vụ khác
cùng host. Browser/legacy tests khác cũng có thể dùng CPU/IO. Đây là phép đo
local có tải nền, không phải benchmark máy dedicated hay production sizing.

Peak RSS lấy mẫu: server thứ nhất 106.544 KiB, server thứ hai 101.876 KiB,
harness 85.924 KiB. Đây là per-process RSS ở các lần lấy mẫu, không phải peak
RAM toàn stack; PostgreSQL/Docker/kernel không nằm trong ba con số đó.
`time -v` có giới hạn riêng của đo process/children, không thay phép đo container.

Native Android/iOS, MLS/rekey/recovery, media/voice mã hoá, bot consent/plan,
push nền, mạng yếu, full 30 phút/24 giờ và release gate vẫn chưa được lượt này
chứng minh. Chỉ có bytes tổng hợp trong artifacts; token/private key được tạo
trong RAM và database test tạm, không ghi vào báo cáo hoặc Git.

## Các ca lỗi chạy riêng trên PostgreSQL/tiến trình thật

`bash scripts/chat_v2_postgres.sh` chạy lại sau sửa, race detector bật, không
có test skip. [Log](chat-mass-realtime/postgres-race-tests.log).

Ca mới `TestPostgresSlowConsumerIsolationResumeAndLiveRevocation` đạt trong
12,49 giây: một recipient không ACK, recipient khác vẫn nhận tin thứ hai;
server ngắt slow consumer sau thời hạn ACK; reconnect từ cursor 1 chỉ nhận
sequence 2. Cùng ca đó thu hồi bearer session trong database khi socket đang
im lặng, xác nhận socket bị đóng và gửi tiếp bị 401. Đây là hai process thật
và DB thật; không dùng memory store. Ca kill/restart/receipt replay đã có cũng
được chạy lại thành công.

[Mutant độc quyền khóa đọc](chat-mass-realtime/exclusive-reader-mutant.log)
đã đỏ đúng regression mới; binary sản phẩm không bị sửa để ép test đạt.
