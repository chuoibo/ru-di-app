# Trạng thái rỗng của Cộng đồng vẽ bằng cảnh của app — kết quả

- Nhánh `claude/cong-dong-trang-rong`, base `f043bea4`. Nguồn: ghi chú trần của finish review đợt «Cộng đồng năm
  chữ-tab» (`docs/claude/2026-10-03/cong-dong-chu-tab/ket-qua.md`, «ký hoạ cho trạng thái rỗng để đợt vẽ lại cả
  bốn»), cộng việc nhỏ để lại «bỏ lưu lỗi thì bài về đầu danh sách».
- Cách làm: native trong phiên, impeccable-pipeline (Extension, code-led; tham chiếu = DESIGN.md và mười cảnh có
  sẵn, không có comp). Finish reviewer và documenter chạy bằng agent general-purpose đọc đúng file hướng dẫn của
  chúng (plugin 4.5.0 không đăng ký hai agent đó trong phiên).
- Verdict finish review (ngữ cảnh mới): `fix` (6 mục) → `fix` (R1–R3, hồi quy do chính mẻ sửa) → **`ship`**.
  Mục 1 (DESIGN.md) qua documenter.

## Đã làm

| Phần | Commit |
|---|---|
| Năm danh sách rỗng thành `EmptyState` inline trên gutter 16 (trước: Ionicons 42 trên `h1`, những ô rỗng cuối cùng ngoài `EmptyState`); hai cảnh mới `chua-luu-bai`, `chua-co-bai`; chữ, lối đi tiếp, thẻ xin cá nhân hoá, tiêu đề danh sách, cỡ cảnh; `traVeCho`; nút `EmptyState` gọn trên mọi nền tảng; tìm kiếm Cộng đồng có cảnh; test quét `trang-rong-co-hinh` | `63e40909` |
| DESIGN.md + surface brief (documenter), ảnh bằng chứng, hàng đợi, file này | commit tài liệu |

Bản đồ:

| Danh sách rỗng | Cảnh | Tiêu đề · lối đi tiếp |
|---|---|---|
| Dành cho bạn, Thịnh hành | `chua-co-ky-niem` (dây phơi, hai kẹp trống) | «Một ngày đáng kể» · «Viết bài» |
| Đang theo dõi | `chua-co-ban` (hai ghế, một chỗ trống) | «Chưa theo dõi ai» · «Xem bài thịnh hành» |
| Đã lưu | **`chua-luu-bai`** (hộp mở nắp, lòng trống, dải đánh dấu coral; Nếp `gap-lai`, mặt `binh-than`) | «Chưa lưu bài nào» · «Xem bài thịnh hành» |
| Bài của tôi | **`chua-co-bai`** (bảng tin khu phố hai cột có mái, một ghim coral; Nếp `dua-giay`) | «Chưa kể chuyện nào» · «Viết bài» |
| Bài đã ẩn | không cảnh (danh sách quản trị) | «Chưa ẩn bài nào» |
| Tìm trong Cộng đồng không thấy | `tim-khong-ra` (như Khám phá), lui khi bàn phím mở | giữ nguyên |

## Số đo

- Cây làm việc ở `63e40909`: npm test 1503/1504 (`MOBILE_REQUIRE_WEB_A11Y=1`), ca đỏ là
  `rudi-hanh-trinh-web` (chập chờn đã biết, load 12.7), chạy lại xanh; tsc sạch.
- Cổng sạch đúng SHA: xem mục «Cổng sạch» dưới.
- 15 đột biến tự nghĩ, mỗi cái đỏ đúng test đã đoán, identity xanh: `traVeCho` luôn về đầu; cảnh cho «Bài đã ẩn»;
  «Đang theo dõi» mời viết bài; bảng tin dùng lại `voi-len` (trùng pose); hộp chỉ hai lớp (không đứng một mình);
  bỏ cảnh khỏi tường nhóm (test quét bắt, gọi đúng tên); xin cá nhân hoá khi rỗng; ngưỡng cảnh đảo hai lần
  (168/144, rồi chỉ theo chiều cao); tiêu đề danh sách trên danh sách rỗng và trên khung xương; cảnh tìm kiếm khi
  bàn phím mở. Canary: test mới chạy trên `CommunityScreen.tsx` của main đỏ 6/6.
- Web (stack QA loopback, persona `dalat-0`, cả năm tab rỗng): cạnh trái trạng thái rỗng x 16 = ô tìm; tablet 1024
  x 300 = cột. Nút thấp nhất kết thúc / mép thanh tab: 320×640 cảnh 120 → 552 / 567 (trước sửa: 571 và 598, nút
  «Kể khoảnh khắc đầu tiên» trùm cột con dấu x 128–192); 360×640 cảnh 168 → 518 / 567; 375×667 cảnh 168 → 518 / 594.
  «Xem bài thịnh hành» từ «Đã lưu» → tab «Thịnh hành» `aria-selected`.
- Android (AVD riêng chỉ-đọc cổng 5610, dev client debug APK của cây gốc, Metro 8096 của nhánh): 411×914 ở 1.0/1.3
  và 360×640 dp (`wm size 1080x1920`, density 480) ở 1.0/1.3; mọi nút gọn và nằm trên thanh tab; 360×640 1.3 cảnh
  120. Tìm kiếm (mở bằng deep link `rudi://community/search?q=…`): uiautomator thấy cảnh khi bàn phím đóng, không
  thấy khi mở, câu gợi ý có ở cả hai. Hàng hai nút (`rudi://places/khong-co-noi-nay` → «Chưa mở được địa điểm»):
  «Thử lại» và «Về Khám phá» một hàng ở cả bốn tổ hợp kích thước × cỡ chữ.

Ảnh: `evidence/EV-CDR-WEB.jpg`, `evidence/EV-CDR-ANDROID.jpg` (ghim sha256). Ảnh đủ của ba vòng ở
`.impeccable/review/cdr*/` (gitignore).

## Quyết định thay chủ sản phẩm

- **Nút của mọi `EmptyState` gọn theo chữ trên máy thật** (`full={false}`). Android giãn nút hết cột vì `width:
  100%` Yoga tính theo cột, web tính theo hàng co; DESIGN.md ghi «một hành động `RudiButton compact`» và mọi ảnh
  đã review là bản web. Chạm khoảng 35 màn trên Android/iOS. **Đính chính commit `63e40909`**: dòng «không mount nào
  dùng secondary» (chép theo lời reviewer) sai — năm chỗ có cửa phụ (Nhóm «Tôi có lời mời», Khám phá «Xóa lọc», ba
  `ErrorState` ở `PlaceDetailLive`, `OutingLive`, `PickOutingLive`). Ở đó Android trước chồng hai nút giãn hết cột,
  nay một hàng như web (luật «Hai nút cùng một hàng» của DESIGN.md); đã chụp `PlaceDetailLive` «Thử lại» + «Về Khám
  phá» ở 411×914 và 360×640, cỡ 1.0 và 1.3: một hàng, không cắt. Bốn chỗ còn lại cùng component, chưa chụp. Nếu sai:
  bỏ `full={false}` ở `EmptyState.tsx`.
- «Bài đã ẩn» không có cảnh: danh sách quản trị, mở từ menu cài đặt, như «Bạn chưa chặn ai».
- «Đang theo dõi»: «Câu chuyện bắt đầu từ một người» → «Chưa theo dõi ai» (hai dòng ở 320, và cùng dạng với ba tab
  của người đọc); lối «Kể khoảnh khắc đầu tiên» → «Xem bài thịnh hành» (viết bài của mình không làm đầy danh sách
  theo dõi).
- Bảng tin rỗng mời «Viết bài» thay «Kể khoảnh khắc đầu tiên»: cùng tên với khay Tạo và «Bài của tôi», và nút ngắn
  tránh cột con dấu ở 320.
- Thẻ xin cá nhân hoá chờ tới khi «Dành cho bạn» có bài: trên bảng tin rỗng nó là lời mời coral thứ hai và không đổi
  được gì.
- Cảnh 168, chỉ 120 khi cửa sổ < 700 dp **và** chật (đầu hai hàng, rộng < 360, hoặc chữ > 1.15).

## Còn mở

- **Hai màn rỗng chưa có cảnh** (nợ, không phải quyết định; `CHUA_VE` trong `tests/trang-rong-co-hinh.test.mjs`):
  «Chưa có bạn nào để rủ» (`hai-nguoi/ChonNguoi.tsx`), «Chưa có địa điểm để thêm» (`keo/PickOutingLive.tsx`).
- Tìm trong Cộng đồng ở 360×640 cỡ 1.3, bàn phím mở: tiêu đề trích từ khoá dài ba dòng nên câu gợi ý chỉ hiện dòng
  đầu (có từ trước đợt này; cảnh đã lui). Hướng: tiêu đề ngắn khi từ khoá dài, hoặc cuộn theo bàn phím.
- 360×640 cỡ 1.3: hàng «Địa điểm | Cộng đồng» cuộn ngang nên mép trái hiện «a điểm» (hành vi cuộn có chủ ý của
  `DauKhamPha`; chưa xét là lỗi).
- Bỏ lưu lỗi trên «Đã lưu» chỉ kiểm bằng hàm thuần và đọc mã; không giả lập được lỗi mạng trên stack.
- iOS chưa chụp (máy Linux). Việc từ trước vẫn mở: lỗi xoá tài khoản chờ bản phiên B8. «Điều mình muốn giữ» ở lại
  menu cài đặt bảng tin: chủ sản phẩm xác nhận 03/10.

## Cổng sạch

Cây sạch đúng SHA `63e40909` (worktree tạm, `npm ci`, gỡ sau khi chạy):

- `gate.sh` guard · guard-range · screens · go-vet · go-test · eval-kich-ban · shared · mobile: **ĐẠT 8/8**; mobile
  **1504/1504**.
- Tầng FastAPI trong `rudi-ux-pytest-git:dev` (`--strict` contract · client-routes · server-routes · cors · ownership ·
  python-touch): **ĐẠT 6/6**; 389 lời gọi đều gửi X-Actor-ID.
- pytest gốc: nhánh 19 đỏ / 841 xanh / 52 bỏ qua; main `f043bea4` 19 đỏ / 840 xanh / 53 bỏ qua; danh sách ca đỏ
  **giống hệt main** (môi trường).
- `go_postgres_tier.sh`: **3329 PASS · 0 SKIP · 1 FAIL** `TestXoaTaiKhoanKhongConHangNaoCuaNguoi` — đúng ca đỏ có sẵn
  trên main, chờ bản sửa của phiên B8 (ghi chép đợt chữ-tab).
- Commit tài liệu sau đó (DESIGN.md, surface brief, ảnh, file này) cộng hai sửa chú thích (tên test «mọi cảnh»,
  mép bảng x 78): chạy lại guard · guard-range · mobile trên cây sạch của SHA cuối, số ghi trong commit message.
