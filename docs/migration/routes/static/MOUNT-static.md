# Thẻ route — `MOUNT /static`

Đo ngày 2026-09-22 trên ảnh Python ghim `mobile-parity-api` (id `680a21678fab`),
là stack reference của cổng parity. Mọi con số dưới đây là **đo**, không đọc từ
nguồn Starlette.

Trước hôm nay `/static` chưa từng có thẻ, và **chưa kịch bản parity nào tải một
file thật** — corpus chỉ chạm nhánh 404. Nên `etag`, `last-modified`,
`accept-ranges`, 304 và 206 chưa có hợp đồng nào cả.

## Vì sao route này không phải rác framework

`services/core/internal/web/guest/templates/guest.html:30` — **trang khách do Go
phục vụ** — trỏ `<link rel="stylesheet" href="/static/guest.css">`, và dòng 162
trỏ `/static/guest.js`. Bốn template khách khác cũng vậy.

Tức hôm nay: **HTML của Go, CSS của Python.** Xoá Python mà chưa chuyển `/static`
thì trang khách mất toàn bộ style và script. Đây là rủi ro đang sống, không phải
dọn dẹp cho đẹp.

`MOUNT /static` phục vụ đúng ba file: `guest.css` (10.184 B), `guest.js`
(2.224 B), `design_system.css` (11.198 B).

## Hợp đồng đo được

| Yêu cầu | Trả lời |
|---|---|
| `GET /static/guest.css` | 200 · `content-type: text/css; charset=utf-8` · `content-length` · `accept-ranges: bytes` · `etag` · `last-modified` |
| `GET /static/guest.js` | 200 · `content-type: text/javascript; charset=utf-8` |
| `HEAD` | 200, cùng bộ header, không thân |
| `If-None-Match` khớp | **304, chỉ có `etag`** — không kèm `last-modified` |
| `If-Modified-Since` khớp | 304, cũng chỉ có `etag` |
| `If-None-Match` sai | 200 đầy đủ |
| `Range: bytes=0-9` | 206 · `content-range: bytes 0-9/10184` · `content-length: 10` |
| file không có | 404 · `application/json` · `{"detail":"Not Found"}` |
| `/static/` (thư mục) | 404, y hệt |
| `/static` (trần) | **307** · `location` tuyệt đối dựng từ `Host` |
| `POST /static/guest.css` | 405 · `{"detail":"Method Not Allowed"}` · **không có header `Allow`** |
| `..%2f..%2fmain.py` | 404 |
| `Range: bytes=-10` (đuôi) | 206 · `content-range: bytes 10174-10183/10184` <!-- repo-guard: allow=long-number reason=http-byte-range --> |
| `Range: bytes=99999-100000` <!-- repo-guard: allow=long-number reason=http-byte-range --> | **416** · `content-range: */10184` · `content-length: 0` · `content-type: text/plain; charset=utf-8` |
| `Range: bytes=0-1,5-6` | 206 · `content-range: multipart/byteranges; boundary=<32 hex ngẫu nhiên>` |
| `If-Range` khớp etag + `Range` | 206 như thường |
| `If-Range` cũ + `Range` | **200 đầy đủ**, bỏ qua `Range` |
| `HEAD` file không có | 404, cùng thân JSON |

Bốn dòng in đậm là chỗ dễ port sai nhất:

- **304 không mang `last-modified`.** Một bản port "trả lại đủ header cho tử tế"
  sẽ thêm nó vào và đỏ.
- **405 không có `Allow`.** Khác với 405 của route API, vốn *có* `Allow`. Đây là
  Starlette `StaticFiles` chứ không phải router chính, và hai chỗ đó không cùng
  luật. Kế hoạch W0 ghi "405 carries `Allow`" — đúng cho route API, **sai cho
  mount này**.
- **416 trả `text/plain` và thân RỖNG.** Không phải JSON như 404/405 cùng mount.
  `http.Error` của Go viết một câu vào thân — muốn khớp thì phải tự viết.
- **Multi-range sinh `boundary` ngẫu nhiên mỗi lần trả lời.** Hai lần gọi cùng
  một stack đã khác nhau, nên đây là giá trị parity **không so byte được** —
  phải bind như placeholder, hoặc để ngoài corpus và ghi rõ.

Go `http.ServeContent` của thư viện chuẩn làm đúng phần lớn: `Range`, `If-Range`,
`If-None-Match`, 304 chỉ-có-etag, suffix range, multipart. Ba chỗ phải tự viết
là 416 thân rỗng, thân JSON của 404/405, và công thức etag.

## `etag` KHÔNG dẫn xuất từ nội dung — và điều đó đo được

Starlette dựng etag từ **mtime và kích thước**, không từ byte:

```
etag = md5(f"{st_mtime}-{st_size}")
```

Đối chiếu trong ảnh: `md5("<mtime>-<size>")` với mtime `…020,898309` và size
`10184` cho ra `643656…3ec` = đúng etag
được phục vụ. Còn `md5(nội dung)` = `397aab…f7e`, **không liên quan gì**.

Phép thử quyết định — hai bản dựng khác nhau của cùng nội dung:

| Ảnh | mtime | `etag` phục vụ | md5(nội dung) |
|---|---|---|---|
| `680a21678fab` | 21/09 14:47:00,898309 | `643656382d0b4df1c31428b69735d3ec` | `397aab04ee043894e776a8a6a2605f7e` |
| `f061d2b53ef1` | 21/09 17:46:18,253247 | `a31f7470416e1f039dddfaf64f1362c5` | `397aab04ee043894e776a8a6a2605f7e` |

**Nội dung giống hệt nhau, etag khác nhau.** Ba file trong một ảnh dùng chung
một mtime, tức mtime là dấu vết của lớp `COPY` lúc dựng ảnh, không phải của file.

Hệ quả cần nói rõ, vì nó đổi bản chất câu hỏi:

1. **Không có hợp đồng nào để phá.** Giá trị etag hiện tại đã đổi sau mỗi lần
   dựng ảnh, kể cả khi không ai sửa một byte CSS. Không client nào phụ thuộc
   được vào một giá trị cụ thể.
2. **Đây là một lỗi cache đang sống, hướng vô hại.** Cùng nội dung mà validator
   đổi → trình duyệt tải lại CSS không đổi sau mỗi lần deploy. Chiều ngược lại
   (nội dung đổi mà etag giữ) **không xảy ra**: đổi nội dung thì đổi lớp ảnh,
   đổi lớp thì đổi mtime.
3. Parity từ trước tới nay mù với chuyện này vì reference và candidate **dùng
   chung một ảnh**, nên mtime bằng nhau một cách tình cờ.

## Cái bẫy chờ sẵn cho bản port Go

CPython **không** tính `st_mtime` bằng `ns / 1e9`:

```
st_mtime_ns   19 chữ số
st_mtime      phần lẻ .898309     ← sec + nsec*1e-9
ns/1e9        phần lẻ .8983088    ← LỆCH, dài thêm một chữ số
```

Số nanosecond ấy vượt quá 53 bit nguyên chính xác của float64, nên đổi
nguyên số sang float trước rồi mới chia là mất chính xác theo kiểu khác.

Go phải viết `float64(sec) + float64(nsec)*1e-9`, rồi in bằng
`pyjson.FloatRepr` (đã có, đã có oracle đối chiếu Python). Viết
`float64(t.UnixNano())/1e9` — cách tự nhiên nhất — cho ra **etag khác**, và
**không bộ test so JSON nào bắt được**, vì etag là header.

## Cái này chưa chứng minh

- Nội dung ba file có đúng không: thẻ này đo **cơ chế phục vụ**, không đọc CSS.
- Trình duyệt thật có dùng lại cache không — mới chỉ đo `curl`.
- `boundary` của multi-range là ngẫu nhiên hai phía, nên nhánh đó chứng minh
  được *hình dạng* chứ không bao giờ chứng minh được *byte*.
- Hành vi khi hai file khác `content-type` cùng lúc, và khi tên file có ký tự
  không ASCII: chưa đo.

## Bằng chứng để lật sang Go (LIVE-GO)

Cổng đầy đủ `scripts/gate.sh parity` chạy trên **cây SẠCH tại `336700ac`**
(`git status` rỗng), bốn pha, mỗi pha một cặp stack cô lập. Kết quả
`ĐẠT 1 · HỎNG 0 · BỎ QUA 0`, mất 2 giờ 20 phút:

| Pha | Kết quả |
|---|---|
| dev main + làn DB + làn ảnh | 353 kịch bản · 10797 bước · **0 khác biệt** · tap `answered_in_core=10232`, `served_routes=151`, `unserved=0` |
| canary | `identity equal, every exercised damage caught` |
| probe | 22 ca · 10 lệch · **10 đều là ngoại lệ đã khai** · `unexpected=0 stale=0` |
| limiter | 9 · 209 · 0 |
| prod main + canary | 23 · 604 · 0 · `every exercised damage caught` |

Hai ngoại lệ ADR-0029 §2.4 đều **kích hoạt**, tức không cái nào đã hết lý do:

```
RESPONSE-204-CONTENT-LENGTH = 28   (w0/replay-204)
STATIC-VALIDATOR-VALUE      = 12   (static/get-static-files)
```

12 khớp chính xác số bước mang validator: 12 bước 200/206/304, 8 bước còn lại
(416, 404, 405, dựng người) không mang etag nên không có gì để nhận.

`main` đã nhích sang `fdad3129` trong lúc chạy, nhưng hai commit ấy chỉ chạm
`apps/mobile/**` và `scripts/mobile_native.sh` — **0 file** trong
`services/core/` hay `parity/`, nên số đo trên `336700ac` còn đúng cho cây
backend hiện tại.

### Đột biến tự nghĩ, đều đỏ đúng chỗ

| Đột biến | Kết quả |
|---|---|
| 304 kèm thêm `last-modified` | 2 ca đỏ — «go sent last-modified, python sent none» |
| 405 kèm header `Allow` | 1 ca đỏ — «header allow: python "", go "GET, HEAD"» |
| etag không bám nội dung | 1 ca đỏ — «does not track the content» |
| khôi phục mặt nạ `<hex32#n>` | in ra `python "<hex32#1>", go "<hex32#1>"` |
| thêm handler không có hàng manifest | «152 of 153 handlers are in the manifest» |

### Cái này vẫn KHÔNG chứng minh

- **Nội dung ba file.** Đo cơ chế phục vụ, không đọc CSS. Ca đối chiếu bản nhúng
  với cây Python chỉ nói hai bản GIỐNG NHAU, không nói bản nào đúng.
- **Trình duyệt thật dùng lại cache.** Mới đo bằng `curl` và bằng parity.
- **`boundary` của multi-range.** Ngẫu nhiên hai phía; Go trả nguyên file.
- **Etag của route khác.** Toàn corpus chạy với etag KHÔNG che và 0 khác biệt,
  nhưng điều đó nói mọi bước ĐÃ CHẠY không lệch — không nói route nào đó không
  thể lệch ở một nhánh chưa có kịch bản.
