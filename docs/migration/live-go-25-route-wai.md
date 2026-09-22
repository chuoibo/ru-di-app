# 25 route WAI chuyển hẳn sang Go (LIVE-GO)

Ngày 2026-09-22. Thực hiện: Claude, theo uỷ quyền ADR-0016/ADR-0029 và quy tắc
toàn repo ADR-0031.

Đợt này khép lại nhóm `PORTED-UNPROVEN`: 13 route thuần backend và 12 route
AI/lai. Sau khi lật, **151/156 route thuộc về Go**; 5 route còn lại là của
FastAPI (`/openapi.json`, `/docs`, `/docs/oauth2-redirect`, `/redoc`,
`MOUNT /static`).

## Vì sao 25 hàng này KHÔNG lật cùng 126 hàng hôm qua

Vì chúng chưa có kịch bản parity nào. Đo trước khi viết: trong 371 kịch bản
có sẵn, **12/13 route thuần backend có đúng 0 kịch bản** khai đích danh.

Phép đo ấy cũng trả về một con số không ai đi tìm, và nó nói về đợt trước:
**0/126 route đã LIVE-GO thiếu kịch bản**. Tức đợt lật hôm qua không phải lật
mù. Máy đo đầu tiên của tôi hỏng — nó trả 0 cho cả `POST /expenses`, route
chắc chắn đã chứng minh — nên con số này chỉ đáng tin sau khi dựng ca đối chứng.

Việc giữ 25 hàng lại hoá ra là quyết định đúng: viết kịch bản cho chúng tìm ra
**ba lệch Python↔Go thật**, và cả ba đều nằm trong chính 25 route ấy.

## Ba lệch tìm được, và vì sao không thứ gì ngoài parity thấy chúng

### 1. `Null{}` không phải nil — bẫy nil-CÓ-KIỂU của Go

Không có khoá Gemini thì brain trả `{"card": null}`. JSON `null` giải mã thành
`pyjson.Null{}` — một **struct** — nên `card == nil` là **false**.

| | |
|---|---|
| Python | `if raw is None: return _silent("unavailable")` — dừng ngay |
| Go | `if card == nil { ... }` không bắt được, rơi xuống `Ground()` → `"ungrounded"` |

Comment sẵn có trong `value.go` góp phần dựng bẫy: «A nil Value is treated as
Null by the encoders» đúng lúc **ghi**, nhưng dễ đọc ngược thành lúc **đọc**.

Lane chat sửa lỗi này trước tôi (`db52a39d`) và tìm được **năm** chốt, trong
khi tôi chỉ thấy **ba**. Hai chốt tôi bỏ sót: `picksVal` (cuốn phim) và
`stops`. Bản của họ ở `main`, tôi lấy bản ấy và bỏ bản trùng của mình.

### 2 và 3. Thứ tự khoá jsonb — `group_fit` và `reviews`

```
Python: {"min_people":2,"max_people":8,"relation":"Bạn thân, cặp đôi"}
Go:     {"relation":"Bạn thân, cặp đôi","max_people":8,"min_people":2}
```

**Độ dài thân bằng nhau từng byte: 5694 với 5694**, lệch từ byte 489. Với
`reviews` là 1051 với 1051.

Cột là `jsonb`, và jsonb **không giữ thứ tự ghi vào** — Postgres sắp khoá theo
độ dài rồi theo byte, nên `relation` (8 ký tự) ra trước `max_people`/
`min_people` (10). Python không bao giờ thấy thứ tự đó vì hàng đi qua
`class GroupFit(BaseModel)` ở `routes/places.py:117`, mà pydantic ghi theo thứ
tự khai báo trường. Go dội thẳng cột ra wire.

Đo thẳng mới ra, sau hai lần tôi đoán sai:

```
DB (jsonb):     {"relation": "Bạn thân", "max_people": 6, "min_people": 2}
Python phục vụ: {"min_people": 2, "max_people": 6, "relation": "Bạn thân"}
```

**Cùng JSON, cùng độ dài, khác wire.** Mọi bộ test so JSON sẽ xanh mãi mãi.

### Tập rủi ro đã đóng

Điều kiện để sinh lỗi này cần **hai** vế cùng lúc:

1. Go dội nguyên `json.RawMessage` ra wire, **và**
2. phía Python đi qua một `BaseModel` **lồng** có trường khai sẵn

Nếu Python khai `dict` trơn thì pydantic cũng dội nguyên xi — hai bên cùng trả
thứ tự jsonb và Go dội theo là **đúng**.

`models.py` có đúng 7 cột jsonb chứa khoá. Đã quét cả 7:

| Cột | Vế (a) | Vế (b) | Kết luận |
|---|---|---|---|
| `group_fit` | có | có | **đã sửa** |
| `reviews` | có | có | **đã sửa** |
| `card` (Message) | có | **không** — `dict` trơn | đúng |
| `content`, `nguon` (PairPaperVersion) | **không** — dựng tay | có | đúng |
| `notify_prefs`, `event_data` | — | — | không ra wire |

Đó cũng là lý do W8 `pair_papers` qua parity sạch từ trước: `wirePaperVersion`
dựng từng trường theo đúng thứ tự model khai. Nó đúng **một cách tình cờ**, nên
đã thêm comment cảnh báo tại chỗ: rút gọn nó thành dội thẳng sẽ mở lại đúng lỗ
này, ngắn hơn, cùng JSON, khác wire.

## Bằng chứng

13 kịch bản trong `parity/scenarios/wai/`, phủ **25/25** route (và 126/126
route đã LIVE-GO cũng có kịch bản).

Cổng parity đầy đủ, bốn pha, mỗi pha một cặp stack cô lập:

| Pha | Kết quả |
|---|---|
| dev main + probe | **352 kịch bản · 10.777 bước · 0 khác biệt** · tap `answered_in_core=10211`, `served_routes=151`, `unserved=0` · probe 22 ca, 10 lệch đều là ngoại lệ ADR-0029 §2.4, `unexpected=0 stale=0` |
| dev canary | `identity equal, every exercised damage caught` |
| dev limiter | 9 · 209 · 0 |
| prod main + canary | 23 · 604 · 0 · `every exercised damage caught` |

Cộng: `make e2e` 11/11, `go build`/`go vet`/`gofmt` sạch, repo guard đạt.

Hai lần chạy hỏng dọc đường, cả hai là hạ tầng chứ không phải lệch, và cả hai
được phân loại đúng ngay lúc đọc:

- Ba pha chết vì `auth.docker.io` timeout khi BuildKit nạp frontend. Kiểm lại
  ngay sau đó: HTTP 200, 1,25 s — mạng chập nhất thời.
- Pha canary chết vì **stack reference biến mất giữa lượt** (`connection
  refused`). Bằng chứng nó không phải lỗ hổng bộ so: canary của pha prod chạy
  trọn ở cùng lượt và kết luận `every exercised damage caught`. Chạy lại sau
  khi tắt máy ảo Android (RAM khả dụng 8,8 → 12 GiB) thì xanh.

Nhãn ba mức trong script — `0` xanh, `1` đỏ thật, `2` không dựng nổi stack —
là thứ giữ cho hai sự cố ấy không bị đọc thành lệch. Gộp chúng vào «đỏ» là cách
đi sửa một lỗi không tồn tại.

## Cái này KHÔNG chứng minh

Ghi ở đầu mỗi file kịch bản, để một bảng toàn xanh không ngụ ý quá phần nó đo:

- **Nhánh mô hình thật trả lời.** Stack parity không có khoá Gemini nên mọi
  đường AI rơi vào nhánh không-có-mô-hình. Nhánh ấy tất định hai phía nên so
  được từng byte; nhánh có mô hình thật cần khoá thật và một câu trả lời vốn
  không tất định.
- **Khả năng dò mặt.** Harness sinh ảnh từ seed, tức nhiễu màu không có mặt
  người. Bộ dò không thấy gì ở **cả hai** phía, nên sự khớp nhau ấy nói về câu
  trả lời rỗng.
- **Thân ảnh địa điểm.** Hàng trong `place_photos` chỉ đến từ trình nhập OSM,
  nên `/places/{id}/photos/{photo_id}` chỉ đi được tới 404 và 422. Bytes,
  `content-type` và header `public, max-age=86400` chưa được parity chứng minh.
- **Nhánh 429 của các route AI.** Cần cửa sổ limiter mới cho từng kịch bản,
  tức làn `lane: limiter`, không phải làn chính.
- **Tiêu chí phụ theo `id` của `/destinations`.** Cần hai điểm đến trùng toạ
  độ, mà seed không có và không route nào tạo điểm đến. Canh bằng test Go thay
  vì parity.

## Còn lại gì

1. **5 route framework.** Cần viết lệnh con `core openapi`, nhúng 3 file
   `/static` vào binary, và gỡ điều kiện `class == "framework"` trong
   `manifest.go` vốn đang cấm mọi route framework về Go.
2. **Chuyển nguồn manifest từ Python sang Go.** `check_route_ownership.py`
   hiện gọi `render_route_manifest.py --check`, tức dựng manifest **từ chính
   app Python**. Chưa làm bước này thì không xoá được dòng Python nào.
3. **Xoá Python** — khoảng 48.800 dòng trên tổng 50.421 của `services/api/app`,
   giữ lại ~1.600 dòng AI. Rồi tới 509 file / 129.513 dòng test Python, phần
   đắt nhất và dễ tự lừa nhất.
