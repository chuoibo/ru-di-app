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
npm run tu-kiem                     # detector và hàm làm tròn số: canary đỏ, identity xanh (+ --dot-bien: 4 đột biến)
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
node kich-ban/f07-so-doi.mjs [--chi chon-nguoi,lap-so,ru,cai-dat,vong-doi,ru-toi-day,m6-c9,m6-khung,c8,lanh,loi,tablet]
                                    # F07 Sổ hai người: ghi vào chat-0/1 và các cặp chat-2…13 lập qua API; Team Đà Lạt không đụng
node kich-ban/f07-phan-xu.mjs        # phán quyết bằng mắt của F07
node kich-ban/f08-ky-niem.mjs [--chi du-lieu,tha,chon-huy,nhap,checkin,tuong,album,xem-anh,story,story-dong,bai,thanh-tich,tem-hep,khong-phien,lanh,loi,c8,voi-toi,tablet]
                                    # F08 Kỷ niệm · Media: ghi vào nhóm chat-test và chat-0/1; Team Đà Lạt chỉ đọc
node kich-ban/f08-phan-xu.mjs        # phán quyết bằng mắt của F08
node kich-ban/f09-ho-so.mjs [--chi du-lieu,ho-so,tab-back,da-luu,sua,cai-dat,chan-trang,loi-cai-dat,phien,da-chan,lanh,khong-phien,loi,c8,tablet,xoa]
                                    # F09 Hồ sơ · Cài đặt: ghi vào moi-54 và chat-20 (bỏ chặn), chat-0 chỉ đọc; `xoa` XOÁ HẲN moi-55, chạy một lần
node kich-ban/f09-phan-xu.mjs        # phán quyết bằng mắt của F09
node kich-ban/f01-phan-xu.mjs        # gắn lại bằng chứng cho baseline F00/F01 (checkpoint 10, sự cố 3)
AUDIT_BASE=http://127.0.0.1:8081 node kich-ban/f10-bang-dev.mjs [--chi lab-base,san-base,nghieng,gap,album,kham-pha,so-sanh,long-nut,chang,sticker,hang-cho,o-tim,reorder,xem-anh,renderer]
                                    # F10 bảng dev; cần server dev: EXPO_PUBLIC_RUDI_FIXTURE=1 EXPO_OFFLINE=1 CI=1 npx expo start --web
node kich-ban/f10-bang-dev.mjs --chi prod   # không đặt AUDIT_BASE: bản export production phải chuyển /dev/* về /welcome
node kich-ban/f10-phan-xu.mjs        # phán quyết bằng mắt của F10, kèm sửa trùng ID TC-MO12-KEO với F02
node kich-ban/f11-demo.mjs [--chi tab,route,co-phien,l28,l28-thoat,cat-chu,thoat]
                                    # F11 chế độ demo trên bản export, chưa đăng nhập; co-phien dùng phiên chat-0, chỉ đọc
node kich-ban/f11-phan-xu.mjs        # phán quyết bằng mắt của F11; `lat` lật hàng tự động mà ảnh bác bỏ, chạy lại thì bỏ qua
node kich-ban/e-luong.mjs [--chi e1-vao,e1-nhom,e1-moi,e1-tin,e1-dem,e2-tao,e2-chang,e2-album,e3,e3-sau,e5-ban,e4,e5-chan,e5-chan-so,e5-ghep,e6-thoat,e6-khong-phien,e6-co-phien]
                                    # E1–E6 bằng nút của app; moi-56/57/58 vào cửa qua UI; theo đúng thứ tự trên
node kich-ban/e-phan-xu.mjs          # phán quyết bằng mắt của E, gắn issue, gắn ảnh ghép cho từng hàng, 12 hàng native
node tong-hop.mjs <docs-dir>        # coverage-matrix.md (+ CSV và đếm ngoài git)
node kiem-tai-lieu.mjs <docs-dir> [--canary]
                                    # ghim ảnh, link ảnh, bảng issue theo mức/loại; --canary đòi 5 canary đỏ
node chot-anh.mjs <docs-dir> <danh-sach.json>   # chép ảnh được chọn, ghim sha256 vào allowlist
node so-sanh-so.mjs <sổ chính> <sổ chạy lại>    # so một lượt chạy lại (vd. trong worktree sạch) với sổ chính, theo hàng tự động cuối và đầu
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

Chuyển động nhanh hơn chu kỳ chụp ảnh thì đo bằng giá trị chính phần tử mang trong từng khung rAF, bắt đầu TRƯỚC thao tác,
không đo bằng ảnh chụp định kỳ: ở F07 ảnh chụp cách 150 ms có mẫu đầu ở 620 ms, lúc bìa sổ đã lật xong, nên chỉ đếm được
Nếp diễn và SVG nhường Skia. Khi đọc transform, chọn đúng phần tử: bìa sổ có hai mặt cùng `rotateY` (mặt trong là
180 − 108·m). Khoảnh khắc chỉ diễn một lần cho mỗi sự kiện, nên mỗi lượt đo M6 cần một cặp chưa lập sổ; cặp mới lập qua
API (`lapCap` trong `f07-so-doi.mjs`). Một khoảng dài không có khung rAF nghĩa là luồng chính bận: trình tự đọc được,
thời lượng thì không.

Ảnh của expo-image trên web là `<img alt=…>`, không có `aria-label`: tìm ảnh theo `alt`. Ảnh in trên tường nghiêng nên
hộp bao lớn hơn khung; đo khung bằng `offsetWidth/offsetHeight`. Cử chỉ trong trình xem ảnh phải chạm vào giữa khung lật
trang, không vào «ảnh cuối trong DOM» (thường là ảnh kế bên, ngoài khung). Một cú chụm mà ảnh không nhận thì trình duyệt
phóng **cả trang**: sau mọi cú chụm, đọc `visualViewport.scale`, và đừng chạm tiếp theo toạ độ bố cục (F08 đã đọc nhầm
«Đóng không đóng» vì thế). Đo vòng đời của một lớp trên bản chưa bị cử chỉ nào làm lệch.

Dữ liệu mặc định có thể làm ca vô nghĩa: bài mới đăng ở mức «Chỉ mình tôi», nên ca của người đọc cần một bài «Bạn bè».
Dòng ngay sau một nút không mặc nhiên là lý do của nút: kiểm cả vai trò của nó (F08 lấy nhầm nút «Báo cáo bài này»).
`innerText` có thể mang xuống dòng và ký tự icon; chuẩn hoá khoảng trắng và bỏ vùng Private Use trước khi ghi vào sổ.

Đổi tab bằng thanh tab không phải lúc nào cũng thêm mục lịch sử: từ tab đầu (Khám phá), lần đổi đầu tiên thêm một mục,
các lần sau thay mục; mở thẳng một tab khác rồi đổi tab thì không thêm gì, và Back rời hẳn app (F09 đo nhầm vì thế). Ca
nào bấm Back phải ghi `history.length` và nói rõ đường vào. Các tab đã ghé vẫn gắn trên trang: mọi truy vấn DOM phải
giới hạn trong `data-testid` của màn đang đo (F09 đếm nhầm ô tìm của Khám phá). `SkeletonRow` dùng trần không mang role;
chỉ `SkeletonGroup` có role progressbar «Đang tải», nên «đang chờ» phải đo bằng việc không có hàng, trạng thái rỗng hay
câu lỗi. Tràn và chữ bị cắt thì đọc số của bộ đo harness (`chup()` trả `tomTat.tranNgang` và `chuBiCat`, có canary),
đừng tự đếm mép phần tử: các lớp ẩn cũng có mép. Kiểm một nút có với tới được không thì kéo từng cú nhỏ về phía nút và
đọc sau mỗi cú; kéo tới cuối rồi mới đọc thì nút ở đầu trang đã trôi mất.

ID test case phải duy nhất giữa các feature: sổ khoá hàng theo `tc|nền tảng|cấu hình`, nên hàng của feature sau dùng
lại ID của feature trước sẽ đè hàng đó (F10 từng đè hàng N/A của F02 vì dùng chung `TC-MO12-KEO`). Hàng mới mang tiền tố
`TC-Fxx-` của feature đang đo. Đừng chạy hai kịch bản có trình duyệt cùng lúc khi ca có chờ hoạt ảnh: SwiftShader chia
CPU và hoạt ảnh vượt ngưỡng chờ. Lọc tín hiệu của bộ đo theo vị trí (hộp của vùng hay của hộp thoại), không theo chữ:
cùng một chữ có thể nằm dưới lớp nền. Bộ đo vùng bấm quét cả tài liệu, nên một nút ngoài màn lặp lại ở mọi cửa sổ:
đọc toạ độ trước khi kết luận. Placeholder của ô nhập là chữ vẽ đè lên ô (F44), không phải thuộc tính `placeholder`.

`elementFromPoint` và cú chạm theo toạ độ đều bỏ qua phần tử `inert`: hit-test coi chúng như `pointer-events: none`. Khi
màn đang hiện bị inert, cả hai rơi xuống lớp nằm dưới, nên đừng dùng hit-test để kết luận một lớp «thấy được»: nhìn ảnh
(F11 từng báo «sheet hiện lại» trong khi ảnh vẫn là Khám phá). Ngược lại, hit-test là cách đúng để biết cú chạm sẽ rơi
vào đâu. Màn tab không bị gỡ khi rời tab: màn cũ vẫn có bố cục bên dưới, và lớp của nó vẫn có kích thước, độ mờ khác 0.

Bộ đo xếp chữ kết bằng «…» vào `ellipsis`, tách khỏi `chuBiCat`. Tiêu chí «không cắt» chỉ đọc `chuBiCat` sẽ PASS trong
khi số tiền hay nhãn nút đang bị «…» (F11: «1.106.25…»). Mỗi màn đọc cả danh sách `ellipsis`, và tách chữ cắt có chủ
đích (mô tả một dòng) khỏi số tiền, tiêu đề và nhãn nút.

Hành trình E1–E6 chạy theo thứ tự: e1 lập nhóm cho mọi hành trình sau, e5-ban kết bạn cho e4, e5-chan chặn sau e4.
Mỗi phần có ghi dữ liệu đọc trạng thái trên máy chủ trước và dừng nếu việc đã làm, nên chạy lại không ghi lần hai.
Những điều E1–E6 đã dạy:
- Lối của trạng thái rỗng biến mất khi có dữ liệu («Rủ hội một buổi» chỉ có khi chat chưa có tin): thử cả lối thường.
- Màn stack (chat, kèo) không có thanh tab: muốn sang tab khác thì quay về một màn tab trước.
- Nhãn của bản demo khác bản sống («Cần trả» và «Còn phải trả»); đọc ô tiền theo DOM (`oTien`), không theo thứ tự
  `innerText`, vì bố cục hai cột đảo thứ tự.
- `chuTrang` chỉ giữ 400 ký tự đầu; câu nằm sâu trong sheet phải đọc trên cả thân trang.
- Độ trễ tin nhắn đo từ lúc bên gửi bắt đầu soạn, với bên đọc đã chờ sẵn.
- Đăng xuất thu hồi cả bearer harness đã lưu cho persona đó; gặp 401 thì xoá bản lưu ở `phien/`.
- Hai ảnh có thể trùng từng byte khi cùng màn, cùng dữ liệu: Chromium dựng tất định. Mở ra xem trước khi nghi chép nhầm.
- Xem ảnh vẫn tìm ra điều không hàng tự động nào đo (thanh đầu «1 thành viên», UI-122): đo lại có hẹn giờ trước khi ghi.

Số trong chữ của sổ được in ra ma trận qua `thu-vien/lam-tron.mjs`: chỉ số một dấu chấm có từ 9 chữ số trở lên được làm
tròn (đúng phạm vi luật long-number của guard); mọi số khác in nguyên văn. Bản đầu làm tròn mọi số có dấu chấm và in
tiền kiểu Việt sai («75.000đ» thành «75đ», sự cố 4 trong report). `tu-kiem` giữ bốn hàng và hai đột biến cho hàm này.

Chạy `kiem-tai-lieu.mjs` sau `tong-hop.mjs` và trước mỗi commit. Nó không đếm `evidence-manifest.md`: file đó liệt kê
mọi ảnh trong thư mục, nên đếm nó thì «ảnh không ai dùng» không bao giờ đỏ (sự cố 3 trong report). Ảnh chỉ được tính
là có dùng khi một issue, report, hoặc một hàng phán quyết trong ma trận dẫn tới nó; cột `evidence` của hàng phải ghi
đúng tên ảnh đã commit.

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
