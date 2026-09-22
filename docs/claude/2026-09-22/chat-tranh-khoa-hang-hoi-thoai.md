# Tranh khoá trên một hàng: từ đỏ sang xanh, và bằng chứng cho cả hai chiều

**Ngày 22-09-2026.** Tiếp việc bỏ dở trong `chat-tai-30-phut.md`, mục
"Việc tiếp theo": *xác nhận giả thuyết bộ đếm bằng `pg_locks` gắn tên quan hệ,
giảm tranh chấp, rồi đo lại. Không hạ ngưỡng.* Cả ba bước đều đã làm.

## 1. Xác nhận: đúng là hàng đợi trên hàng hội thoại

Lượt trước đếm chờ theo `mode`, mà `Lock/transactionid` thì hàng nào cũng
giống hàng nào. Nên trước hết phải thêm cột.

`scripts/chat_mass_realtime.sh` có thêm bộ lấy mẫu **tuỳ chọn**
(`RUDI_CHAT_LOAD_LOCK_SAMPLE=1`, nhịp `RUDI_CHAT_LOAD_LOCK_TICK`, mặc định
0,25 s): ba câu hỏi mỗi nhịp — nhịp tim `active/idletx/waiting`, các khoá
**chưa được cấp** kèm `relname/page/tuple`, và **truy vấn của kẻ đang giữ**
qua `pg_blocking_pids`. Tuỳ chọn vì nó tốn một vòng psql mỗi nhịp.

Lượt burst 300/s, 60 giây, 1.000 socket, 2 nhóm — 1.532 mẫu:

```
    254   WAIT transactionid ShareLock rel=-
    235   WAIT tuple AccessExclusiveLock rel=chat_v2_conversations page=… tuple=…
```

**Mọi khoá tuple đang chờ đều nằm trên `chat_v2_conversations`.** Không có
quan hệ thứ hai nào xuất hiện. Giả thuyết thành bằng chứng.

Còn quan trọng hơn là *kẻ giữ khoá đang làm gì*:

| Kẻ giữ đang ở đâu | mẫu |
|---|---|
| `SELECT … FROM chat_v2_conversations … FOR UPDATE` (kể cả `idle in transaction`) | **423** |
| `SELECT digest,sequence FROM chat_v2_sends …` | 98 |
| CTE tăng dãy | 127 |
| `commit` (WALSync/WALWrite) | 87 |

115 mẫu bắt được kẻ giữ ở trạng thái **`idle in transaction / Client/ClientRead`
ngay tại câu `FOR UPDATE`**: nó đã chiếm khoá hàng rồi **trả quyền về cho tiến
trình Go** và ngồi chờ vòng mạng tiếp theo. Đó là câu trả lời đầy đủ.

Vùng tới hạn cũ là: `FOR UPDATE` → **vòng mạng** → kiểm trùng → **vòng mạng** →
CTE → **vòng mạng** → `commit` + fsync. Ba vòng mạng và một fsync, tất cả nằm
trong một khoá hàng mà **mọi tin nhắn của cả nhóm** đều phải đi qua. Không phải
I/O, không phải checkpoint.

253 mẫu cho thấy kẻ giữ khoá *bản thân nó cũng đang chờ* ở câu `FOR UPDATE` của
chính nó: đây là **đoàn xe khoá**, không phải một điểm nghẽn đơn lẻ.

## 2. Sửa: dời điều kiện vào trong câu lệnh, không giữ khoá để bảo vệ nó

`chatv2.send` không còn khoá hàng hội thoại lúc xác thực. Ba thay đổi:

1. **`convLock`** thay cho cờ `exclusive bool`: `lockNone | lockShare |
   lockUpdate`. Đường gửi dùng `lockNone`. Đường đọc `Events()` giữ `lockShare`;
   `mark()` giữ `lockUpdate`. (`EventsBatch` — đường phát sóng cho 1.000 socket
   — vốn đã là `REPEATABLE READ, READ ONLY` không khoá, không phải sửa.)
2. **Câu tăng dãy mang theo điều kiện**: `… WHERE context_id=$1 AND ready AND
   epoch=$8`. PostgreSQL đánh giá lại `WHERE` trên **phiên bản hàng nó vừa
   khoá**, nên "đọc epoch → so → ghi" vẫn nguyên tử — chỉ là gói trong một câu
   thay vì trải qua ba vòng mạng. Không khớp thì `inserted` rỗng, và
   `appendGuardError` đọc lại hàng để trả **đúng** `ErrEpoch` hay `ErrNotReady`.
3. **Đua trùng biên nhận**: bỏ khoá thì hai bản của cùng một `logical_send_id`
   có thể cùng trượt qua bước kiểm trùng. Khoá chính `chat_v2_sends_pkey` vẫn
   quyết, và kẻ thua **chạy lại đúng một lần**; lượt hai tìm thấy biên nhận và
   đi đường replay — đường này so digest, nên vẫn phân biệt được retry thật với
   một tin khác dùng lại id.

Đây **không** phải nới lỏng kiểm tra. Bảo đảm chuyển chỗ, không biến mất.

## 3. Đo lại: cùng máy, cùng tham số, cùng harness

Burst 300/s · 60 s · 1.000 socket · 200 người · 5 thiết bị. Máy ảo Android vẫn
chạy suốt ở cả hai lượt — cố ý, để hai cột so được với nhau.

| | TRƯỚC | SAU | ngưỡng ADR-0031 |
|---|---|---|---|
| `passed` | **false** | **true** | |
| delivery p50 | 611 ms | **50 ms** | — |
| delivery p95 | 2.309 ms | **90 ms** | ≤ 800 ms |
| delivery p99 | 2.594 ms | **131 ms** | ≤ 2.000 ms |
| delivery max | 2.742 ms | **276 ms** | — |
| send p95 | 2.281 ms | **39 ms** | — |
| send p99 | 2.564 ms | **74 ms** | — |
| nhịp đạt | 292,8/s | **299,8/s** | mời 300/s |
| chờ khoá tuple (mẫu) | **235** | **17** | — |

Toàn vẹn **không đổi và vẫn sạch** ở cả hai lượt: 9.000.000/9.000.000 lượt
giao, 0 thiếu · 0 trùng · 0 hụt dãy · 0 hỏng envelope · 0 lỗi gửi · 0 rớt hàng
đợi · 180 replay đạt, 0 lỗi. `http_status` y hệt: `{201: 18.000, 200: 180}`.
Nhanh hơn mà không đánh đổi gì — vì chỗ sửa là thời gian **giữ** khoá, không
phải thứ khoá bảo vệ.

Phân bố kẻ giữ khoá sau khi sửa nói đúng điều đã dự đoán: câu `FOR UPDATE` và
câu kiểm trùng **biến mất hoàn toàn** khỏi danh sách; chỉ còn CTE tăng dãy và
`commit`. Vùng tới hạn giờ là **một câu lệnh cộng một fsync**.

## 4. Ca đỏ được khi mất bảo vệ

Bỏ khoá mà chỉ *nói* rằng điều kiện trong câu lệnh thay thế được thì không tính.
Ba ca mới trong `services/core/internal/chatv2/store_test.go`:

- `TestASupersededEpochCannotStillAppend` — rekey commit **xen vào giữa** xác
  thực và ghi; phải là `ErrEpoch` và **không** để lại sự kiện nào.
- `TestARosterInvalidationCannotStillAppend` — trigger đặt `ready=false` xen
  vào giữa; phải là `ErrNotReady`.
- `TestTheDuplicateReceiptRaceIsRecognisedAsItself` — giữ giao dịch thứ nhất mở
  cho tới khi giao dịch thứ hai đã qua bước kiểm trùng, rồi commit, để khoá
  chính thật sự nổ; đòi `isDuplicateReceipt` nhận ra nó.

Đã chạy đột biến, không chỉ chạy ca:

| Đột biến | Kết quả |
|---|---|
| `AND ready AND epoch=$8` → `AND (ready OR true) AND (epoch=$8 OR true)` | hai ca đầu **ĐỎ**, thông điệp `want ErrEpoch, got <nil>` |
| `chat_v2_sends_pkey` → `chat_v2_events_pkey` | ca thứ ba **ĐỎ** |
| `"23505"` → `"23503"` | ca thứ ba **ĐỎ** |
| tắt hẳn vòng chạy lại | ca thứ ba **ĐỎ** |

Ghi lại một bước sai của chính tôi vì nó là bài học lặp được: đột biến **đầu
tiên** tôi thử là **xoá** `AND … epoch=$8`, và ca đỏ với `expected 7 arguments,
got 8`. Ca đỏ **vì sai số tham số**, không phải vì mất bảo vệ — nó sẽ đỏ với bất
kỳ sửa đổi nào ở đó. Một đột biến chỉ có giá trị khi nó thay **hành vi** mà giữ
nguyên **hình dạng**.

Và một ca tôi đã suýt tính là bằng chứng: bản đầu của ca thứ ba chạy hai
goroutine cùng `Send`. Nó **xanh cả khi đã tắt vòng chạy lại** (3/3) — hai
goroutine chỉ thay phiên nhau, đua không bao giờ xảy ra. Đã thay bằng ca ép
đúng thứ tự xen kẽ.

## 5. Còn nợ

Lượt burst xanh **không** đóng cổng ADR-0031. Bản ghi cũ tự nói: *"lượt ngắn
không thay được lượt dài"* — sự cố lượt 30 phút rơi vào phút 26. Baseline
30 phút @100/s phải chạy lại trên chính bản sửa này; kết quả ghi tiếp ở đây.

`Mark` vẫn giữ `FOR UPDATE` suốt năm vòng mạng. Harness hiện không đánh vào
biên nhận đọc nên chưa có số; đừng sửa khi chưa đo được.
