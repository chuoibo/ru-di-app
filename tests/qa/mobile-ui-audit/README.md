# mobile-ui-audit

Harness bằng chứng cho audit UI/UX app mobile ngày 27/09/2026
(`docs/claude/2026-09-27/mobile-ui-audit/`). Không phải mã sản phẩm, app không import.

## Cần có

- Một bản web export của `apps/mobile`:
  `EXPO_PUBLIC_API_URL=http://127.0.0.1:<cổng core> npx expo export --platform web --output-dir <thư mục>`.
- Stack cục bộ (Postgres, API Python, cửa trước Go) và dữ liệu seed:
  - `npm run seed:rudi` (Team Đà Lạt);
  - `scripts/chat_e2e_seed.mjs` (22 người, nhóm 20 thành viên).
  - Công thức ở `docs/claude/2026-09-24/qc-frontend-chieu-sau-va-cau-chuyen.md` §9 và `report.md` §A.
- Chromium: `timTrinhDuyet()` (`tests/qa/tim-trinh-duyet.mjs`) tìm qua `CHROME_BIN` hoặc
  `PUPPETEER_EXECUTABLE_PATH`.
- `playwright-core` ghim đúng bản có Chromium đã cài, nên không tải trình duyệt.
- `npm ci` trong thư mục này.

## Biến môi trường

| Biến | Nghĩa |
|---|---|
| `AUDIT_WEB` | thư mục bản export (được phục vụ bằng `serve()` của `apps/mobile/tests/chrome-cdp.mjs`) |
| `AUDIT_BASE` | hoặc URL của server có sẵn (dev server cho `/dev/*`) |
| `AUDIT_API` | API mà bản export trỏ tới (mặc định `http://127.0.0.1:58099`) |
| `AUDIT_OUT` | thư mục đầu ra, **ngoài repo**: ảnh PNG/JPEG, số đo JSON, sổ `results.jsonl`, phiên |
| `AUDIT_CHAT_SESSIONS` | file `sessions.json` do chat seed ghi |
| `AUDIT_FONTCONFIG` | tuỳ chọn: file fontconfig đưa Roboto vào font stack (xem `report.md` §A) |

## Chạy

```bash
npm run tu-kiem                     # detector: canary đỏ, identity xanh (+ --dot-bien: 2 đột biến)
node khung-ma-tran.mjs              # hàng khởi tạo của ma trận (NOT_TESTED / BLOCKED)
node kich-ban/khoi-dong.mjs         # khói: build nạp được, renderer, đăng nhập bằng cookie
node kich-ban/baseline.mjs --persona dalat-0 --configs C1,C2,C3 --man F02.S01=/explore
node kich-ban/f00-vo.mjs [--chi dinh-tuyen,tab,khay-tao,create-lanh,resume-cham,nep,chuyen-canh]
node kich-ban/f01-vao-cua.mjs       # Welcome, Đăng nhập, OTP (AUDIT_MOI=1 cho tài khoản mới), lời mời
node kich-ban/f02-kham-pha.mjs [--chi loc,tim,ai,tim-luu,cuoi,loi,diem-den,chi-tiet,bat-san,cat-chu]
node kich-ban/f02-phan-xu.mjs       # phán quyết bằng mắt của F02, ghi kèm ảnh đã xem (chạy sau f02-kham-pha)
node tong-hop.mjs <docs-dir>        # coverage-matrix.md (+ CSV và đếm ngoài git)
node chot-anh.mjs <docs-dir> <danh-sach.json>   # chép ảnh được chọn, ghim sha256 vào allowlist
```

Thư viện dùng chung nằm ở `thu-vien/`, không phải `lib/`: `.gitignore` gốc bỏ qua mọi thư mục `lib/`
(mẫu Python), và hai checkpoint đầu đã push thiếu cả thư viện vì thế.

Sổ `results.jsonl` chỉ được ghi thêm. Hàng sinh từ lỗi của harness được rút bằng `soGhi(out).rut(tc, lyDo)`:
dòng gốc ở lại trong sổ, ma trận bỏ nó khỏi bảng và liệt kê trong mục «Hàng đã rút» kèm lý do.

## Những điều harness không đo được

- Cỡ chữ hệ thống: react-native-web cố định `fontScale` 1.0.
- Safe area: `index.html` không có `viewport-fit=cover`.
- Bàn phím ảo.
- Mọi thứ native.
- Độ mượt: Chromium headless vẽ bằng SwiftShader.
  - Số liệu từ `thu-vien/chuyen-dong.mjs` chỉ nói về hình dạng của chuyển động (đi đâu, dừng ở đâu,
    có bản sao không), không nói FPS.
  - Cử chỉ mang timestamp ảo (`thu-vien/cu-chi.mjs`) vì độ trễ CDP ở đây là 30–125 ms mỗi sự kiện.
