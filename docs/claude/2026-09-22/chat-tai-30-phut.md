# Tải chat 30 phút: đạt toàn vẹn, **trượt độ trễ**, và lần đầu có nguyên nhân

**Ngày 22-09-2026.** Lượt baseline theo ADR-0031 §"Cổng nghiệm thu": 1.000 kết
nối, 100 tin/giây, 30 phút. Kết quả là **ĐỎ**, và tài liệu này ghi đỏ ở đâu.

Artifact ngoài repo: `/tmp/rudi-chat-load-base30/` (`result.json`,
`progress.log`, `pg-samples.txt`, `host-before.txt`, `provenance.txt`).
`/tmp` không bền qua reboot; các con số cần giữ đã chép vào đây.

## Phán quyết

| | |
|---|---|
| `passed` | **false** |
| `adr0031_latency_passed` | **false** |
| `integrity_and_capacity_passed` | **false** |

## Toàn vẹn: sạch tuyệt đối

| | |
|---|---|
| deliveries | **89.819.000 / 89.819.000** |
| thiếu · trùng · hụt dãy · hỏng envelope | **0 · 0 · 0 · 0** |
| dial errors · queue drops · rớt bất ngờ | **0 · 0 · 0** |
| replay | 1.797 đạt, **0 lỗi** |
| kết nối đỉnh | 1.000 |

Gần chín mươi triệu lượt giao, không mất một cái nào. Phần này không có gì để bàn.

## Độ trễ: trượt p99

| | đo được | ngưỡng ADR-0031 |
|---|---|---|
| delivery p50 | 38 ms | — |
| delivery p95 | **128 ms** | ≤ 800 ms ✅ |
| delivery p99 | **2.323 ms** | ≤ 2.000 ms ❌ |
| delivery max | 7.991 ms | — |
| send p99 | 2.638 ms | — |

p95 dưới ngưỡng gần **sáu lần**. p99 vượt **16%**. Trượt là trượt.

## Sức chứa: 381 lần bị từ chối

`http_status: {201: 179.619, 200: 1.797, 503: 381}`

**0,212%** số lần gửi nhận `503`. Đây là **hàng đợi nhận tải từ chối trước khi
nhận**, không phải tin đã ACK bị mất — nên toàn vẹn vẫn sạch. Cơ chế admission
làm đúng việc của nó: chịu tải không nổi thì từ chối sớm chứ không đổ.

## Hỏng ở đâu, và không phải vì cái mình tưởng

`progress.log` cho thấy **hai sự cố rời rạc**, không phải suy giảm dần:

| mẫu | ~phút | p99 | max | send 503 |
|---|---|---|---|---|
| 0–40 | 0–13 | 60–66 ms | ~100 ms | 0 |
| 50 | ~16 | 257 ms | 1.411 ms | 0 |
| 90 | ~24 | 1.272 ms | 3.299 ms | 0 |
| **157–158** | **~26** | 1.229 → 1.913 ms | **7.991 ms** | **360 cùng lúc** |
| 165 | ~27 | **2.471 ms** | 7.991 ms | 381 |

Mười ba phút đầu hoàn toàn sạch. Rồi hai cú, cú thứ hai làm 360 lần gửi bị từ
chối trong một khoảnh khắc.

### Không phải checkpoint

Bàn giao trước đoán "WALSync/checkpoint/pool contention có dấu vết".
`pg-samples.txt` lấy mẫu mỗi 60 giây cho thấy **checkpoint chạy suốt lượt**, kể
cả trong mười ba phút sạch đầu tiên. Nó không phân biệt được lúc tốt với lúc xấu.

### Là tranh khoá hàng ở đường ghi

Đúng cửa sổ sự cố, `pg_stat_activity` đổi hẳn hình dạng:

```
tick=26 11:24:48  waits=[]                                    active=1   idle=30
tick=27 11:25:50  waits=[Lock/transactionid=4, Lock/tuple=1]  active=12  idle=14
tick=28 11:26:51  waits=[Lock/transactionid=3, Lock/tuple=1]  active=7   idle=8
tick=29 11:27:52  waits=[IO/WALSync=1]                        active=1   idle=30
```

`Lock/transactionid` là **đợi một transaction khác commit**; `Lock/tuple` là
**đợi khoá một hàng cụ thể**. Số phiên `active` nhảy từ 1 lên 12 rồi về 1.

Đó là dấu của **hàng đợi trên cùng một hàng dữ liệu**. Ở đường ghi chat v2, mỗi
lần gửi phải tăng bộ đếm dãy của **chính hội thoại đó** — nên mọi tin trong một
nhóm đều xếp hàng sau nhau trên đúng một hàng. Lượt này có 1.000 socket chia
cho hai nhóm.

## Đã làm xong ba việc dưới đây

Xem `chat-tranh-khoa-hang-hoi-thoai.md` cùng thư mục: giả thuyết bộ đếm đã được
xác nhận bằng `pg_locks` có tên quan hệ, vùng tới hạn đã được thu lại, và lượt
burst 300/s đã đo lại — `passed: true`, p95 2.309 ms → 90 ms, ngưỡng giữ nguyên.
Baseline 30 phút vẫn phải chạy lại trên bản sửa.

## Việc tiếp theo, theo thứ tự

1. **Xác nhận giả thuyết bộ đếm** bằng `pg_locks` gắn tên quan hệ trong lúc sự
   cố, chứ không chỉ đếm theo `mode`. Supervisor đã lấy mẫu; cần thêm cột.
2. Nếu đúng, giảm tranh chấp ở chỗ đó — gom nhiều tin vào một lần tăng, hoặc
   tách dãy theo shard trong hội thoại — rồi **đo lại**. Không hạ ngưỡng.
3. Chạy lại 30 phút sau khi sửa, rồi mới tới burst và soak 24h.

## Lượt ngắn không thay được lượt dài

Cùng harness, cùng máy, cùng cấu hình, 60 giây trước đó: **p95 57 ms, p99 68 ms,
0 lỗi, `passed: true`**. Lượt 30 phút mới thấy sự cố ở phút 26. Một lượt ngắn
xanh không nói gì về lượt dài.

## Bối cảnh máy

Máy đã được dọn trước khi đo: cổng parity của chính lượt này bị dừng và tám
container parity bị gỡ, load average xuống 0,88 lúc bắt đầu. Vẫn còn một máy ảo
Android (`qemu`, ~32% một nhân) chạy suốt vì cổng native cần nó — ghi ở đây để
lượt sau biết, không dùng làm lý do.

Bàn giao trước ghi p95 **2.427 ms** ở cùng cấu hình và tự chú thích rằng máy lúc
đó còn qemu/Python/ffmpeg/rustc cùng chạy. p95 lượt này là **128 ms**. Chênh
lệch đó gần như chắc chắn là **máy**, không phải mã — nhưng nó không cứu được
p99, và p99 mới là chỗ trượt.

## Burst 300/s: cũng đỏ, cùng một chữ ký

Lượt riêng, không chạy chồng lên baseline, 60 giây.

| | đo được | ngưỡng |
|---|---|---|
| delivery p50 | 353 ms | — |
| delivery p95 | **3.613 ms** | ≤ 800 ms ❌ |
| delivery p99 | **4.142 ms** | ≤ 2.000 ms ❌ |
| nhịp đạt được | 279,9/s | mời 300/s |
| `503` | 132 (0,74%) | — |
| deliveries | **8.938.500 / 8.938.500** | |
| thiếu · trùng · hụt · hỏng | **0 · 0 · 0 · 0** | |

Lại đúng hình dạng cũ: **toàn vẹn sạch, độ trễ và sức chứa trượt**. Gấp ba nhịp
thì hàng đợi tệ hơn gấp ba — khớp với giả thuyết xếp hàng trên bộ đếm dãy của
hội thoại, không khớp với giả thuyết I/O.

So với lượt burst ghi trong bàn giao (p95 **4.983 ms**, **3.661** lỗi): lượt này
p95 **3.613 ms**, **132** lỗi. Tốt hơn nhiều, vẫn đỏ. Không được đọc "tốt hơn"
thành "đạt".

## Phạm vi

`scope` do chính harness khai: *synthetic signed opaque envelopes, real
HTTP/WebSocket/PostgreSQL, **NOT MLS or mobile E2E***. Lượt này **không** chứng
minh gì về MLS, về ứng dụng di động thật, hay về nhiều replica.
