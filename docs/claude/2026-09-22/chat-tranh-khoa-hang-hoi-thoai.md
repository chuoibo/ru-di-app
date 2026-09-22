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

## 5. Baseline 30 phút: ĐẠT, và vẫn còn hai sự cố

Chạy trên đúng SHA đã commit, 100/s, 30 phút, 1.000 socket.

| | đo được | ngưỡng ADR-0031 |
|---|---|---|
| `passed` | **true** | |
| `adr0031_latency_passed` | **true** | |
| `integrity_and_capacity_passed` | **true** | |
| delivery p50 | 39 ms | — |
| delivery p95 | **106 ms** | ≤ 800 ms |
| delivery p99 | **1.441 ms** | ≤ 2.000 ms |
| delivery max | 3.773 ms | — |
| send p95 · p99 | 38 ms · 1.382 ms | — |
| nhịp đạt | 99,998/s | mời 100/s |
| deliveries | **90.000.000 / 90.000.000** | |
| thiếu · trùng · hụt · hỏng | **0 · 0 · 0 · 0** | |
| `503` · rớt hàng đợi · rớt bất ngờ | **0 · 0 · 0** | |
| `http_status` | `{201: 180.000, 200: 1.800}` | |
| replay | 1.800 đạt, 0 lỗi | |

So với lượt 30 phút trước khi sửa: p99 **2.323 → 1.441 ms**, max **7.991 →
3.773 ms**, và **381 lần `503` → 0**.

**Nhưng cổng đạt không có nghĩa là hết chuyện.** Bảng theo phút, đọc từ
`pg-locks.txt` bằng `phan-tich-khoa.py`, cho thấy đúng **hai** cửa sổ sự cố
trong ba mươi phút — và giữa chúng là im lặng tuyệt đối:

| phút | lượt chờ | khoá tuple | kẻ giữ nhiều nhất |
|---|---|---|---|
| 07:21–07:30 | 0–2 | **0** | `IO/WALSync` ở `commit`, 1–2 mẫu |
| **07:31–07:33** | **61 · 68 · 30** | **20 · 19 · 11** | `Lock/transactionid` ở CTE, `LWLock/WALWrite` ở `commit` **×17–20** |
| 07:34–07:44 | 0–5 | **0** | 1–2 mẫu |
| **07:45–07:47** | **36 · 185 · 79** | **10 · 52 · 29** | `Lock/transactionid` ×53, `LWLock/WALWrite` ở `commit` **×48** |
| 07:48–07:50 | 0–1 | **0** | 1 mẫu |

Hai điều đọc được ngay:

1. **Vùng tới hạn cũ đã biến mất thật.** Cả lượt chỉ còn **8 mẫu** bắt được kẻ
   giữ ở `idle in transaction` — trước khi sửa con số ấy là **115** chỉ riêng ở
   câu `FOR UPDATE`, cộng 98 ở câu kiểm trùng. Không còn ai giữ khoá hàng để
   chờ vòng mạng.
2. **Cái còn lại là `commit`.** `LWLock/WALWrite` khi kẻ giữ đang `commit` nhảy
   từ **1–2 mẫu mỗi phút** lúc im lặng lên **48 mẫu** trong phút tệ nhất. Cộng
   `IO/WALSync` là 210/368 mẫu kẻ giữ của cả lượt.

Hai cửa sổ cách nhau **14 phút**. Đó là hình dạng của một thứ chạy theo chu kỳ
làm chậm ghi WAL, chứ không phải suy giảm dần theo tải — và khi ghi WAL chậm
lại, cái fsync nằm **trong** khoá hàng kéo cả nhóm xếp hàng theo.

### Nghi can, và vì sao lần này chưa kết luận

Checkpoint khớp cả chu kỳ lẫn chữ ký. Nhưng bản ghi trước đã một lần loại
checkpoint bằng lý do "nó chạy suốt lượt" — đúng về **sự có mặt**, không nói gì
về **cường độ**: một checkpoint muộn có nhiều buffer bẩn hơn một checkpoint sớm.

Lượt này **không** lấy mẫu checkpoint nên tôi không kết luận. Thay vì đoán, bộ
lấy mẫu đã được nối thêm hai câu — `pg_stat_bgwriter` (số checkpoint theo giờ
và theo yêu cầu, `checkpoint_write_time`, `checkpoint_sync_time`,
`buffers_checkpoint`) và `pg_stat_wal` (`wal_bytes`, `wal_sync`,
`wal_sync_time`) — đã chạy thử trên PostgreSQL 16.15 thật để chắc cột tồn tại,
vì cả khối sampler nuốt stderr và một câu sai cú pháp sẽ im lặng thành "không
có dữ liệu". Lượt sau trả lời được câu hỏi này bằng số.

### Đòn bẩy tiếp theo, nếu cần

Nếu xác nhận là fsync, cách sửa **không** phải vặn nút database mà là **chia
một fsync cho nhiều tin**: gom vài lần gửi của cùng hội thoại vào một giao dịch,
tăng dãy một lần cho cả cụm. Đúng thứ bản ghi trước gọi là "gom nhiều tin vào
một lần tăng". Đó là thay đổi thiết kế, cần đo riêng.

**Ngưỡng ADR-0031 giữ nguyên.** Cổng đạt với biên rộng ở p95 (106/800) và biên
hẹp hơn ở p99 (1.441/2.000); ghi ở đây để lượt sau biết p99 mỏng ở chỗ nào.

## 6. Còn nợ

`Mark` vẫn giữ `FOR UPDATE` suốt năm vòng mạng. Harness hiện không đánh vào
biên nhận đọc nên chưa có số; đừng sửa khi chưa đo được.

Soak 24 giờ chưa chạy.

**Một lượt đo đã hỏng và bị bỏ:** trong lúc baseline này chạy tôi có chạy
`pytest` và hai cổng hợp đồng trên **cùng máy**, đúng cửa sổ 07:31. Ban đầu tôi
đã quy sự cố ấy cho chính mình. Sự cố thứ hai lúc 07:45 xảy ra khi máy hoàn
toàn rảnh, cùng một chữ ký — nên câu đúng là **không quy trách nhiệm được cho
cửa sổ đầu**, chứ không phải "do tôi". Nhật ký việc đã chạy nằm ở
`/tmp/rudi-chat-load-locks/nhieu-host.txt`. Bài học giữ lại: máy đo phải im
lặng, và khi nó không im lặng thì nói ra chứ đừng suy.
