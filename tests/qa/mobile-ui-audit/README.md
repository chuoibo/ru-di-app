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
node kich-ban/f03-keo.mjs [--chi …]  # F03 Plan · Kèo · Hành trình (đọc dữ liệu biến thể của seed-bien-the.mjs)
node kich-ban/f03-phan-xu.mjs        # phán quyết bằng mắt của F03
node kich-ban/f04-tien.mjs [--chi buoc,chan,lui,ban,ban-20,anh,aria,m2,ghi,c9,qt,dot-rong,dot,chia-se,tien-ve,loi,lanh,nep,c8,tablet,demo,mo15,lat]
                                    # F04 Tiền: Team Đà Lạt chỉ đọc sổ; mọi lần ghi sổ vào nhóm chat-test (append-only)
node kich-ban/f04-phan-xu.mjs        # phán quyết bằng mắt của F04
node seed-bien-the.mjs --chat        # 40 tin tổng hợp cho luồng chat cũ của nhóm chat-test (bỏ qua nếu đã đủ)
node kich-ban/f05-chat.mjs [--chi bong-dai,soan,gui,offline,anh,sticker,menu,menu-dong,bao-cao,cong-cu,cai-dat,tin-moi,bo-cuc,phieu,rong,ghim-phu,thong-bao,ghim,lanh,loi,c8,dm,votes]
                                    # F05 Chat: Team Đà Lạt chỉ đọc; mọi lần ghi vào nhóm chat-test
node kich-ban/f05-phan-xu.mjs        # phán quyết bằng mắt của F05
node kich-ban/f06-nhom-nguoi.mjs [--chi tao-nhom,moi,loi-moi,moi-lai,duoc-moi,thanh-vien,tu-bo-quan-tri,ban-be,them-ban,them-ban-app,ho-so,chan,chan-chat,vung-bam,lanh,loi,c8,tablet]
                                    # F06 Nhóm · Người: ghi vào moi-51…53 và chat-20/21; Team Đà Lạt chỉ đọc
node kich-ban/f06-phan-xu.mjs        # phán quyết bằng mắt của F06
node tong-hop.mjs <docs-dir>        # coverage-matrix.md (+ CSV và đếm ngoài git)
node chot-anh.mjs <docs-dir> <danh-sach.json>   # chép ảnh được chọn, ghim sha256 vào allowlist
```

Thư viện dùng chung nằm ở `thu-vien/`, không phải `lib/`: `.gitignore` gốc bỏ qua mọi thư mục `lib/`
(mẫu Python), và hai checkpoint đầu đã push thiếu cả thư viện vì thế.

Mọi lần cuộn của harness chỉ được cuộn dọc: `scrollIntoView` cũng cuộn ngang khung `overflow: hidden`, việc ngón tay
không làm được (F04 đo ra một PASS giả vì thế). `tamCua` và các kịch bản trả lại cuộn ngang sau mỗi lần cuộn.

Danh sách tin của chat là FlatList đảo ngược: thứ tự DOM ngược thứ tự trên màn, và danh sách tự nhảy về cuối khi
nạp thêm. Kịch bản F05 tìm bong bóng theo nhãn và vị trí trên màn, không theo thứ tự DOM, và chạm vào tin mới gửi
(nằm sẵn ở cuối) thay vì cuộn tới tin cũ.

Mở một màn bằng `pushState` + `popstate` (hàm `moQua`/`diToi`) để lại router chỉ một màn: nút trong app gọi
`router.back()` sẽ không có chỗ về. Ca nào đo nút «Quay lại»/«Về …» thì phải tới màn bằng nút của chính app. Tên nút
có ký tự icon (vùng Private Use): hàm tìm nút bỏ các ký tự này trước khi so tên. Câu trên màn chỉ tính phần tử đang hiện:
stack giữ các màn trước ở trạng thái ẩn.

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
