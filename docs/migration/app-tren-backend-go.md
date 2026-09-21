# App Rủ Đi chạy trên backend Go — đo trên máy ảo, có A/B với Python

Người gộp: Claude · 2026-09-20 · mốc `10e6b030`

## 1. Cấu hình

Stack compose thật (`MOBILE_CORE_CANDIDATE_ROUTES=ported make up`), core chạy **trong container** chứ không
phải binary dựng tay. Bằng chứng Go đang phục vụ lấy từ **dòng log khởi động của core**:

```
go_served=151  candidates=151  force_python=None  manifest_routes=156
```

**Đừng dùng `core routes --json` làm bằng chứng**: nó trả `151` ở cả cấu hình ép-Python, vì nó chỉ liệt kê
manifest. Tôi đã dùng nhầm nó một lần và phải tự đính chính.

App: dev client `com.lakiet.rudi` trên `emulator-5554`, bundle từ Metro của chính cây này
(`thiết bị đã nạp bundle của .../apps/mobile`), gọi `http://localhost:8099` qua `adb reverse`.

## 2. Lát cắt dọc của client: 11/11

`scripts/e2e_slice.sh` — chạy chính `apps/mobile/src/api.ts`, module app import, qua cửa trước Go, **chế độ auth
prod**: `11 pass, 0 fail`. Gồm ca «máy chủ prod từ chối `X-Actor-ID` giả khi không có phiên» và «mọi màn đọc dữ
liệu nhóm đều tự mang bearer».

## 3. Bảng Maestro trên máy ảo: 9/11 flow xanh

Chạy hết 11 flow. Các assert **tiền** đều qua: `3.840.000đ`, `2.560.000đ`, `919.583đ`, và assert
`1.106.250đ` **không** hiện — tức phép chia do Go tính, hiện đúng trên màn quyết toán.

Canary còn sống: «dấu vân sai → đỏ đúng ở bước assert dấu vân». Một bảng xanh mà không ca nào biết đỏ thì không
phân biệt được «đã gác» với «phép đo chết».

## 4. Hai flow đỏ — và A/B chứng minh không phải Go

| | backend **Go** | backend **Python** (`MOBILE_FORCE_PYTHON=all`) |
|---|---|---|
| flow chạy | 11 | 11 |
| flow đỏ | `08-hanh-trinh`, `07-trang-thai-song-qua-lan-tat-app` | `08-hanh-trinh`, `11-dang-xuat-khong-xoa-phien` |

- **`08-hanh-trinh` đỏ ở CẢ HAI.** App tự nói lý do trên màn: «APK đang chạy chưa gắn MapLibre. Timeline, mốc
  đánh số và tóm tắt vẫn dùng được. Rebuild dev client rồi bản đồ hiện.» Dev client trên máy cũ hơn cây; app
  xuống thang đúng cách nên câu chữ khác cái flow assert. Dữ liệu chuyến vẫn đọc từ Go bình thường.
- **Cái đỏ còn lại KHÁC flow giữa hai lượt** nhưng chết ở đúng một bước: `_vao-app-sach` → «Rủ Đi thôi!». Đó là
  bẫy khởi động chập chờn khi máy gánh compose + Metro + emulator, rơi ngẫu nhiên vào flow nào thì flow đó đỏ.

**Python không tốt hơn Go**: cùng 2 đỏ, một cái chung, một cái ngẫu nhiên.

## 5. Cái này KHÔNG chứng minh

- 151 route Go phục vụ ở đây là **candidate**; manifest vẫn `owner: python` ở cả 156 hàng. Đây **chưa** phải
  `LIVE-GO`.
- Bảng chạy trên **một** máy ảo, một cỡ màn, dữ liệu seed. Không nói gì về máy thật, mạng thật, tải thật.
- `08-hanh-trinh` chưa từng xanh trong lượt này ở **bất kỳ** backend nào, nên phần bản đồ hành trình **chưa được
  đo** — phải dựng lại dev client mới đo được.
- Không có khoá Gemini, nên các đường AI đi nhánh «không dùng được» ở cả hai bên.
