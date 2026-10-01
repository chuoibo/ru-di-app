# mobile-ui-audit

Harness bằng chứng cho audit UI/UX app mobile ngày 27/09/2026
(`docs/claude/2026-09-27/mobile-ui-audit/`) và phần sau pipeline của nó trên main
(`docs/claude/2026-09-29/mobile-ui-audit-main/`: retest từng issue, issue mới từ UI-123). Không phải mã sản
phẩm, app không import.

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
# Phần sau pipeline, trên main: stack thứ hai và AUDIT_OUT riêng, tài liệu ở docs/claude/2026-09-29/mobile-ui-audit-main
node kich-ban/retest-main.mjs [--chi r-f00,r-f01,r-f02,r-f03,r-f06,r-f07,r-f08,r-f09,r-f11,r-e,r-moi]
                                    # đo lại từng issue: một hàng TC-R-UI-xxx (hoặc một hàng mỗi phần, -A/-B); issue mới là TC-M-…
                                    # chạy riêng một phần của một mục: --chi r-f08:094 (dữ liệu của mục vẫn được dựng, có chốt)
node kich-ban/retest-main.mjs --chi r-p3-f00,r-p3-f01,r-p3-f02,r-p3-f03,r-p3-f04,r-p3-f05,r-p3-f06,r-p3-f07,r-p3-f09,r-p3-e
                                    # P3 (checkpoint retest 2); P3 của F08 nằm trong r-f08: --chi r-f08:098,…,r-f08:106
                                    # UI-092 đo thêm ngày xa: --chi r-p3-f07:092b (dời bản phác của chat-8 rồi trả lại)
AUDIT_BASE=http://127.0.0.1:8091 node kich-ban/retest-main.mjs --chi r-f10:113   # bảng dev: cần server dev của main có fixture
AUDIT_BASE=http://127.0.0.1:8091 node kich-ban/f10-bang-dev.mjs --chi nghieng,long-nut,renderer   # P3 của F10 trên server dev
node kich-ban/retest-phan-xu.mjs     # phán quyết bằng mắt; hàng kịch bản gốc thành hàng retest; rút hàng lệch; hàng NOT_TESTED giữ chỗ
node kich-ban/retest-ghep.mjs        # ảnh ghép retest theo feature, gắn vào mọi hàng có khung trong ảnh
# Thứ tự chốt một checkpoint retest: retest-phan-xu → retest-ghep → chot-anh → tong-hop → retest-bang → kiem-tai-lieu
node kich-ban/tham-do-lich-su-tab.mjs   # chỉ đọc: history.length qua một chuỗi chuyển tab, rồi Back (UI-123); CHUOI=…, PERSONA=…
# Feature mới #26, hai lớp chat hai người (checkpoint N26). Phần: api, ban, moi, doi, soan, chuyen, gu, ru, q2, loi, nhay, lab-e1
node kich-ban/n26-hai-lop-chat.mjs --chi ban:C1,doi:C9   # từng cấu hình; chuyen:doi, gu:C1 và q2 GHI lên stack (xem report §A)
AUDIT_BASE=http://127.0.0.1:8091 node kich-ban/n26-hai-lop-chat.mjs --chi lab   # trang lab /dev/hai-lop-chat: server dev có fixture
node kich-ban/n26-phan-xu.mjs        # phân xử bằng mắt, gắn issue, hàng native, rút hàng giữ chỗ TC-N-26-…; chạy lại không thêm dòng
node kich-ban/n26-ghep.mjs           # ảnh ghép của N26, gắn vào hàng
# Thứ tự chốt checkpoint N26: n26-phan-xu → n26-ghep → chot-anh → tong-hop → kiem-tai-lieu
# Feature mới #14, Cộng đồng (checkpoint N14). Phần, theo thứ tự: api, rong, khong-phien, tu-kiem-ly-do, dang, duyet,
# sau-duyet, bang, ws, chi-tiet, binh-luan, phu, loi, tuong. api và rong chỉ đo được TRƯỚC khi duyet duyệt bài đầu tiên.
# Trước duyet: cấp chat-15 vào community_moderators bằng SQL (cách người vận hành cấp, docs/testing/cong-dong.md).
AUDIT_CORE_LOG=/tmp/rudi-stack2/core.log node kich-ban/n14-cong-dong.mjs --chi api,rong   # AUDIT_CORE_LOG: đếm dòng 23502 (UI-132)
node kich-ban/n14-cong-dong.mjs --chi bang:C2,bang:theo-doi,ws:chi-tiet,chi-tiet:sua-doc   # từng cấu hình hay từng phần con
                                    # dang, duyet, binh-luan, bang:an, chi-tiet:C1 GHI lên stack (report §A); ws đi qua relay Node
node kich-ban/n14-phan-xu.mjs        # phân xử bằng mắt, gắn issue, hàng native và video BLOCKED, rút TC-N-14-…; chạy lại không thêm dòng
node kich-ban/n14-ghep.mjs           # ảnh ghép của N14, gắn vào hàng
# Thứ tự chốt checkpoint N14: n14-phan-xu → n14-ghep → chot-anh → tong-hop → kiem-tai-lieu
# Feature mới #15, sổ chuyến đi / Nếp v3 (checkpoint N15). Phần, theo thứ tự: api, vao, khep-truoc, khep, nguon, dung-ai,
# dung-tay, sua, luu, doc, cong-khai, khoanh-khac-xoa, khong-phien, loi, q4, hep. khep KHÉP KÈO THẬT, không hoàn tác được:
# khep-truoc phải chạy trước nó. vao:keo là màn kèo (chỉ có nghĩa trước khep), vao:<persona>:<cấu hình> là một kệ.
node kich-ban/n15-nhat-ky.mjs --chi api,vao,khep-truoc   # trước khi khép
node kich-ban/n15-nhat-ky.mjs --chi hep,vao:chat-0:C2   # chỉ đọc: sửa sổ ở C2/C3, kệ có sổ
                                    # khep, luu, cong-khai, khoanh-khac-xoa, q4 GHI lên stack (report §A); dung-ai cần không có khoá AI
node kich-ban/n15-phan-xu.mjs        # phân xử bằng mắt, gắn issue, hàng native và Nếp thật BLOCKED, rút TC-N-15-…; chạy lại không thêm dòng
node kich-ban/n15-ghep.mjs           # ảnh ghép của N15, gắn vào hàng
# Thứ tự chốt checkpoint N15: n15-phan-xu → n15-ghep → chot-anh → tong-hop → kiem-tai-lieu
node kich-ban/n15-nhat-ky.mjs --chi q4:hero   # chỉ đọc, sau khi «Kèo album retest» xong: recap và hero quyết toán (UI-149)
# Feature mới #21, hồ sơ kể chuyện (checkpoint N21). Đọc trước mọi lần ghi: api, hanh-trinh, ca-nhan, moi-mo, nep, loi.
# Ghi, sau 0 giờ của một ngày Việt Nam mới (ngày kể đếm theo ngày VN) và theo thứ tự: bai-anh, ket, trung-bay, xem-nguoi,
# binh-luan, anh-toan-man, dang-lai, trang. Mỗi phần ghi kiểm trước khi ghi. ket nhận kết thật, không nhận lại được.
node kich-ban/n21-ho-so.mjs --chi api,hanh-trinh,ca-nhan,moi-mo,nep,loi   # chỉ đọc
node kich-ban/n21-ho-so.mjs --chi hep,khong-phien   # chỉ đọc, lúc nào cũng được: hep:C2, hep:tablet, hep:tu-vo, hep:la
node kich-ban/n21-phan-xu.mjs        # phân xử bằng mắt, gắn issue, hàng native, Nếp thật và MP4 BLOCKED, rút TC-N-21-…; chạy lại không thêm dòng
node kich-ban/n21-ghep.mjs           # ảnh ghép của N21, gắn vào hàng
# Thứ tự chốt checkpoint N21: n21-phan-xu (và n15-phan-xu cho hàng hero) → n21-ghep → chot-anh → tong-hop → kiem-tai-lieu
# Feature mới #22, Rủ Đi AI trong chat (checkpoint N22). Stack không có khoá AI: phòng «sẵn sàng» dựng bằng cách viết lại
# đúng một phản hồi (chat-capabilities) ở trình duyệt; mọi lời gọi khác tới máy chủ thật.
node kich-ban/n22-ai-chat.mjs --chi api,chan,nhac,san-sang,doi,xem-ghim,loi-goi,nep,lab-prod   # bản export E1, không AUDIT_BASE
# nhac:gui và san-sang:gui gửi tin thật vào nhóm chat-test (mỗi tin kiểm trước khi gửi); chan chặn rồi bỏ chặn chat-21;
# api gọi ai-invocations (máy chủ không có khoá nên không lưu gì). Các phần còn lại chỉ đọc hoặc chỉ gõ.
AUDIT_BASE=http://127.0.0.1:8091 node kich-ban/n22-ai-chat.mjs --chi lab,lab-nhom,doi-lab   # server dev E2, fixture bật
node kich-ban/n22-phan-xu.mjs        # phân xử bằng mắt, gắn issue, hàng native và AI thật BLOCKED, rút TC-N-22-…; chạy lại không thêm dòng
node kich-ban/n22-ghep.mjs           # 3 ảnh ghép của N22 (cao 660 cho vừa ngân sách), gắn vào hàng
# Thứ tự chốt checkpoint N22: n22-ghep → n22-phan-xu → n22-ghep → chot-anh → tong-hop → ghim-ma-tran → kiem-tai-lieu
node ghim-ma-tran.mjs <docs main>   # ghim coverage-matrix.md theo digest (luật aggregate-base64-fragments); chạy lại sau mỗi tong-hop
node retest-bang.mjs <docs gốc> <docs main>   # sinh retest.md từ issues.md gốc và sổ retest
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

Bài học của phần retest trên main (29/09):
- Tên chip và nút mang ký tự icon (vùng riêng U+E000–U+F8FF): mọi bộ tìm mới phải bỏ chúng trước khi so tên. Quên
  điều đó, `r-f10` không chạm được chip «Không ảnh» và ghi một hàng sai cách đo (đã rút, đo lại).
- `pkill -f` và `pgrep -f` khớp luôn dòng lệnh của chính shell đang chạy chúng: viết mẫu dạng `[e]xpo start`.
- Không đặt tên biến trùng hàm đã import (`keo` là hàm kéo; đặt tên kèo là `keoAlbum`).
- Chụm hai ngón phóng cả trang trên web: chụp ảnh bằng chứng trước khi chụm.
- Trên main, chuyển tab không thêm mục lịch sử nào (UI-123). Kịch bản dùng Back sau khi chuyển tab sẽ rời app;
  `trangMoi` mở trang gắn phiên (`/favicon.ico`) trước, nên Back về đó cũng là rời app.
- Kịch bản gốc chạy lại trên bản UI mới có thể lệch từng bước: đọc từng hàng trước khi tin, và rút hàng lệch kèm lý do
  (bốn hàng F06).

Bài học của checkpoint retest 2 (P3, 29–30/09):
- expo-image bọc `<img>` trong một lớp cao 0px. Khung ảnh là khối đầu tiên phía trên `<img>` có chiều cao, không phải
  phần tử cha trực tiếp (UI-102, UI-105 đo ra 0 rồi phải rút).
- Màn stack cũ vẫn mount bên dưới màn mới, và có nút cùng tên (chat dưới form tạo kèo đều có «Tạo kèo»). Khi hai màn
  cùng có tên nút, bộ tìm chỉ nhận nút mà tâm của nó trúng chính nó (`elementFromPoint`), như `nutTrung` của `r-p3-e`.
  Form là một route push: chờ URL và ô nhập hiện rồi mới gõ.
- Một PASS có thể chỉ đúng vào ngày chạy: dải ngày của sheet sửa tờ bắt đầu từ hôm nay (UI-092). Đổi ngày của dữ liệu
  (và trả lại) để đo một ngày xa, rồi mới kết luận.
- Muốn thử chữ dài mà stack không có, chèn nó vào phản hồi trình duyệt nhận (`page.route` + `route.fetch`), đừng ghi DB;
  ghi rõ trong hàng rằng dữ liệu là chèn (UI-099).
- Hàng giữ chỗ NOT_TESTED không được chặn hàng đo thật (`daDo` trong `retest-phan-xu.mjs`). Issue đo theo phần
  (`TC-R-UI-027-M5`, …) thì rút hàng giữ chỗ một hàng của nó, nếu không ma trận giữ nó mãi.
- Hàng tự động có thể xanh vì đếm nhầm: 54 ký tự của UI-009 là nhãn năm tab. Hàng nào dựa vào số chữ hay số phần tử thì
  mở ảnh ra xem trước khi tin.
- Id viết cứng của stack gốc (`f08-ky-niem.mjs`) phải thay bằng tra cứu trước khi chạy trên stack khác.

Bài học của checkpoint N26 (hai lớp chat, 30/09):
- Ô soạn là textbox, không phải nút: bộ tìm nút không thấy nó, chữ gõ đi vào khoảng không. Tìm theo nhãn
  (`[aria-label="Ô soạn tin"]`) rồi chạm vào tâm (`oSoan`).
- Đọc chữ của một hàng thì bỏ glyph icon (vùng private-use) như khi so tên nút: mũi tên cuối hàng là một ký tự.
- Mỗi phần đo có ID riêng khi cấu hình trùng: sổ giữ hàng cuối theo `tc|nền tảng|cấu hình`, nên hai phần (mạng nhanh và
  trễ, đám bạn và cặp đôi) cùng ID thì phần sau đè phần trước.
- Chuỗi rỗng khớp mọi chữ: đọc tên thứ cần tìm từ API (tên quán), và dừng nếu nó rỗng. Một PASS sai đã ra vì thế.
- Chèn lỗi mạng thì ghi lại từng lần đọc kèm mã, để chứng minh lỗi rơi đúng chỗ; gỡ bằng `goHet` (`unrouteAll`), và kiểm
  URL sau mỗi bước điều hướng.
- Trạng thái đã qua thời điểm của nó (đồng ý chia gu trước mốc 29/09) dựng bằng cách sửa đúng một trường của phản hồi
  (`page.route` + `route.fetch`), rồi để lệnh thật chạy tiếp; ghi rõ trong hàng.
- Trang lab điều khiển giảm chuyển động bằng prop của nó (chip «Reduce Motion»), không đọc cài đặt hệ thống: C9 không
  đo được gì ở đó.
- Khoảnh khắc chỉ diễn khi trạng thái đổi ngay trên màn (M6): đo bằng hai persona và lấy mẫu rAF từ trước cú chạm đồng ý.
- Script thăm dò dùng `khoiDong` phải gọi `mt.dong()` (hay `process.exit`): server web của harness giữ tiến trình sống.

Bài học của checkpoint N14 (Cộng đồng, 30/09):
- Trạng thái chỉ có một lần (cộng đồng chưa có bài duyệt) phải đo trước mọi lần ghi, và kịch bản phải xếp phần theo thứ tự
  đó (`api`, `rong` trước `duyet`). Đo lại UI-132 cần một stack mới.
- Lý do của nút tắt nằm ở phần tử liền sau nút (vỏ `lyDo` của `RudiButton`, có glyph thông tin), không ở phần tử cha: cha
  có thể là cả mục, và đọc cha đã cho một PASS sai. Dùng `LY_DO_SRC` chung, và chạy `--chi tu-kiem-ly-do` (canary «Đăng
  story», identity «Gửi lên cộng đồng») trước khi tin nó.
- Nhãn trên màn có thể viết hoa chữ đầu hay không tuỳ chỗ («Theo dõi tác giả», «Bỏ theo dõi tác giả»): so khớp không phân
  biệt hoa thường, và bỏ glyph icon trước khi so (bộ lấy mẫu rAF cũng vậy).
- Căn giữa đúng phần tử sẽ chạm, không căn giữa cả thẻ: thẻ có ảnh cao hơn cửa sổ ở 320 và 390×460, và điểm chạm rơi
  dưới phần đầu màn.
- Bộ đọc thân bài chọn chữ dài nhất không phải dải trạng thái hay dòng chú thích; chữ dài đầu tiên có thể là dải «Đang chờ
  duyệt».
- Dữ liệu đổi giữa hai lượt đo: sau khi sửa B1, B1 rời bảng tin của tác giả. Lượt đo lại phải tới bài bằng đường không
  phụ thuộc lượt trước («Bài của tôi»), hoặc đo trên bài khác.
- Stack không cho origin của trang mở WebSocket (403, không có `MOBILE_CORS_ALLOW_ORIGINS`): `routeWebSocket` của
  Playwright chuyển socket qua một `WebSocket` của Node không gửi Origin. Khung là của máy chủ; relay còn cho phép đóng
  socket phía máy chủ để đo lúc nối lại. Bản ghi khung phải `trim()`: khung mang dấu xuống dòng ở cuối.
- Khung của máy chủ mang mốc giờ mili giây (`occurred_at_ms`, 13 chữ số): chép nguyên vào ghi chú thì repo guard chặn ma
  trận (luật long-number). Che nó như che id bài; nó không phải số đo.
- Luật `aggregate-base64-fragments` của repo guard cộng mọi token có cả chữ hoa lẫn chữ thường trên toàn file allowlist,
  kể cả đường dẫn ảnh `…/evidence/EV-…`: tới ghim N14 thì tổng vượt 16 KiB, và chỉ quét range hay tree thấy (staged chỉ
  cộng dòng thêm). Theo lựa chọn của người giao việc, `chot-anh.mjs` viết mỗi ghim mới với `reason` đứng trước `path` và
  mở đầu bằng annotation hẹp của luật đó; ghim cũ giữ nguyên. Chạy `repo_guard.py range` và `tree` trước khi push, đừng
  chỉ `staged`.
- Lần đọc đầu tiên của màn xảy ra ngay lúc mở trang: đọc nó từ nhật ký HTTP của cả phiên trang (`suKien.log.http`), không
  từ bộ ghi bắt đầu sau đó.
- Đừng đặt tên biến trùng hàm đã import: một biến `chup` trong một khối của kịch bản N14 sẽ che hàm `chup` của
  `thu-vien/chup.mjs` trong khối đó. Đã đổi thành `daChup` trước khi chạy; cùng họ với biến `keo` ở checkpoint retest 1.

Bài học của checkpoint N15 (sổ chuyến đi, 30/09):
- Một việc không hoàn tác được (khép cuộc đi) chia kịch bản làm hai: mọi phép đo cần trạng thái «chưa khép» nằm ở phần chạy
  trước (`khep-truoc`), và phần nhiều cấu hình không được vô tình chạy lại nó (`vao:keo` tách khỏi các phần kệ).
- So hai giá trị trước và sau một thao tác chỉ chứng minh được gì khi hai giá trị khác nhau từ đầu: hai trang cùng tên
  «2026-09-29» làm phép so đổi chỗ trang thành PASS rỗng. Đặt tên riêng cho từng trang trước khi đổi chỗ.
- Đọc định danh (ở đây `cover_id`) từ phản hồi của chủ dữ liệu, không từ phản hồi của người đang bị kiểm quyền: sau khi
  thu hồi, phản hồi của người ngoài là 404 và không mang định danh nào.
- Trước khi đánh số một issue mới, tìm trong issue gốc theo hành vi, không theo màn: «link → đăng nhập → Khám phá» đã là
  UI-121 (link chat), dù lần này là link sổ. Tương tự, ô chữ cao 44 là UI-001, không phải «đạt».
- Đánh số lại sau khi đã dựng ảnh ghép thì phải dựng lại và ghim lại mọi ảnh mang nhãn số cũ trước khi commit.
- Hai ngày ISO chỉ cách nhau một dấu cách trông như một số di động với luật `vn-phone` của repo guard. Đặt mỗi ngày trong
  «» khi chép chữ của trang vào ghi chú, như `ghi` của `n15-nhat-ky.mjs` đang làm; và đừng chép nguyên cặp ngày đó vào tài
  liệu hay comment khi giải thích (chính lời giải thích đã bị chặn một lần).
- Khung đỏ của ảnh chú thích (`-ct`) có thể đè lên chữ đầu dòng («0 check-in» đọc thành «check-in»). Ảnh commit cho một
  con số thì dùng bản không chú thích.

Bài học của checkpoint N21 (hồ sơ kể chuyện, 30/09–01/10):
- Luật PASS tự động phải phủ mọi vế của expected. Hàng `TC-N21-CA-NHAN` nêu ba vế (số trên thẻ, nơi tới, tên lối vào) mà
  luật chỉ xét thẻ có mặt; hai hàng PASS thiếu vế đã phải rút. Viết expected xong thì đọc lại luật theo từng vế.
- Một mẫu neo cuối (`…$`) không được thử trên chuỗi ghép từ hai nguồn (tên trợ năng nối với chữ hiện): chuỗi kết thúc bằng
  nguồn sau. Thử riêng từng chuỗi.
- Nhãn có thể ở tên trợ năng chứ không ở chữ hiện: trong thanh đầu, `DemoBadge` vẽ «Demo» nhưng tên là «Dữ liệu demo». Đọc
  nhãn theo `aria-label` trước khi kết luận «không có nhãn».
- Khoảnh khắc chỉ diễn một lần phải đọc ngay sau thao tác gây ra nó, không đọc sau khi tải lại (`TC-N21-M8`).
- Phần tử cha của một nút là thẻ; leo thêm vài tầng là ra cả danh sách (`TC-N21-TUONG-ANH-CD` PASS giả). Tìm khối của một bình
  luận thì đi từ chữ của nó lên, không đi từ một nút thích lên: lời đáp lồng trong khối của bình luận gốc.
- Detector hình (tràn, cắt) không thấy từ bị vỡ giữa chừng: chữ vẫn nằm trong khung. `hep:tu-vo` đo bằng Range cho từng từ;
  dùng khi cột chữ hẹp (khối có ảnh hai bên).
- Một thao tác ghi có thể chạm dữ liệu của chính persona đang đo (thích nhầm bình luận của chính mình). Đếm lại trạng thái
  máy chủ ngay sau mỗi lần ghi, và trả lại bằng API khi lệch.

Bài học của checkpoint N22 (Rủ Đi AI trong chat, 01/10):
- `fullPage` không chụp được nửa dưới của màn react-native-web: trang không cuộn, chỉ một view bên trong cuộn. Muốn thấy
  phần dưới thì cuộn chính view đó (tìm tổ tiên có `overflow-y` auto/scroll) rồi chụp (`lab-nhom`, sự cố 46). Nhãn «cả
  trang» trên một ảnh như vậy là sai.
- Expected phải khớp dữ liệu đang có. Đọc số tin của luồng qua API trước khi đòi chip «Kèm N tin»: chat đôi trống thì chip
  đúng là «Hai bạn chưa có tin nào» (sự cố 45). Thiếu dữ liệu thì đo trên trang lab, không ghi thêm tin để có dữ liệu.
- Khi máy chủ thiếu một năng lực (khoá AI), dựng trạng thái bằng cách viết lại đúng một phản hồi ở trình duyệt; mọi lời gọi
  khác vẫn tới máy chủ thật, nên yêu cầu app gửi đi và lời từ chối của máy chủ là thật. Ghi rõ phản hồi nào đã viết lại.
- Hit-test bỏ qua lớp inert, giống cú chạm: chạm vào chỗ một nút bị che vẫn trúng nút đó. Muốn biết lớp nào vẽ trên cùng
  thì nhìn ảnh, kèm `z-index` tính được (UI-166).
- Đọc danh sách ellipsis trong file số đo của mỗi ảnh (`metrics/*.json`, `doDac.ellipsis`): nhãn bị «…» không tính là cắt
  chữ, nên hàng tự động vẫn PASS (nhãn dải ghim ở 320, UI-023).
- `ghi.mjs` chặn method lạ: hàng gọi API ghi RUNTIME-WEB như các checkpoint trước.
- `pkill -f "<mẫu>"` nằm trong một lệnh ghép thì khớp luôn dòng lệnh của chính shell và giết nó (exit 144). Chạy `pkill` một
  mình, rồi kiểm bằng `ps aux | grep "[e]xpo start"`.
- Guard `staged` chỉ xét dòng thêm, còn `range` và `tree` xét cả file. Một file sinh máy lớn dần (ma trận) có thể qua `staged`
  rồi bị `range` chặn sau commit (sự cố 47). Sau mỗi `tong-hop` của thư mục main, chạy `ghim-ma-tran.mjs`; trước khi commit,
  `python3 scripts/repo_guard.py tree HEAD` chỉ đọc allowlist của commit đã có, nên kiểm sau commit và sửa bằng amend khi
  chưa push.

## Những điều harness không đo được

- Cỡ chữ hệ thống: react-native-web cố định `fontScale` 1.0.
- Safe area: `index.html` không có `viewport-fit=cover`.
- Bàn phím ảo.
- Mọi thứ native.
- Độ mượt: Chromium headless vẽ bằng SwiftShader.
  - Số liệu từ `thu-vien/chuyen-dong.mjs` chỉ nói về hình dạng của chuyển động (đi đâu, dừng ở đâu,
    có bản sao không), không nói FPS.
  - Cử chỉ mang timestamp ảo (`thu-vien/cu-chi.mjs`) vì độ trễ CDP ở đây là 30–125 ms mỗi sự kiện.
