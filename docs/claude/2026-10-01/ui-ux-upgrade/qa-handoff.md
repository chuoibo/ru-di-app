# Bàn giao QA retest

QA chạy lại độc lập; mọi trạng thái trong tài liệu này là **tự kiểm của người sửa**, không phải QA PASS.

## Cách dựng lại môi trường

Giống `docs/claude/2026-09-29/mobile-ui-audit-main/report.md` §A, với các khác biệt:
- Bản web export trỏ vào cửa trước Go của stack thử, phục vụ ở **cổng cố định** để `MOBILE_CORS_ALLOW_ORIGINS` liệt
  kê được origin (khi biến này trống, core nhận mọi loopback cho HTTP nhưng từ chối WebSocket, như QA đã gặp ở N14).
- Harness: `tests/qa/mobile-ui-audit/` của PR #663, chạy với `AUDIT_BASE=<url bản export>`.

Khác biệt của lượt tự kiểm này, để QA tái lập đúng:

- Harness là bản sao **ngoài repo**, không sửa phép đo. Chỉ thêm hai thứ:
  - một kịch bản nhỏ làm ấm phiên `dalat-0` (`kiem-ux/am-phien.mjs`);
  - việc chép file phiên đó vào từng thư mục kết quả trước mỗi kịch bản.
- Lý do: máy chủ cho **5 mã OTP mỗi số trong 15 phút** (`domain/otp/otp.go:43`). Mỗi thư mục kết quả giữ bộ phiên riêng,
  nên chạy nhiều kịch bản liền nhau sau khi xoá phiên cũ sẽ chạm 429 ngay lần đăng nhập đầu.
- Dữ liệu: stack được xoá trắng và dựng lại (`seed:rudi`, `chat_e2e_seed.mjs`, `seed-bien-the.mjs` thường và `--chat`,
  `f06 --chi tao-nhom,moi,loi-moi,duoc-moi`).
  - «Trước» đo trên bản web của `main` `d95edb4`.
  - «Sau» đo trên bản web của cây B1, cùng stack, cùng dữ liệu.
  - Lượt «trước» của các kịch bản ngoài `retest-main` chạy **sau** lượt «sau», trên cùng dữ liệu, ghi rõ ở từng mục.

## Danh sách retest theo batch

### B1 · P1, quyền riêng tư, lối vào và lối ra

Bảng tóm tắt (C1 = 390×844 sáng, web). «Trước» = `main` `d95edb4`; «sau» = cây B1. Ảnh ghép ở `evidence/`.

| Issue | Hàng harness | Trước | Sau | Tự kiểm của người sửa | Ảnh |
|---|---|---|---|---|---|
| UI-005 P1 | `TC-R-UI-005` | FAIL | PASS | đạt: vùng khoá ≥25% màn sau Back 1 → 0 | `EV-B1-UI-005-117-C1.jpg` |
| UI-022 | `TC-R-UI-022` | FAIL | PASS | đạt: mở `https://www.google.com/maps/search/?api=1&query=…` ở trang mới | — (lỗi không có hình) |
| UI-038 (phần Sheet) | `TC-L08-DONG-back` | FAIL | PASS | đạt: ở lại kèo, lưới chạm đổi 12 → 0, focus về «Thêm chặng». Khay trong màn chat (L20, L21) chưa sửa: B2/B6 | — |
| UI-049 P1 | `TC-L32-VONGDOI` | FAIL | FAIL | đạt theo chữ của tiêu chí, ghi chú 1 | `EV-B1-UI-049-C1.jpg` |
| UI-082 P1 | `TC-R-UI-082-SO` | FAIL | PASS | đạt: link sổ thật → `/login?tiep=…`, 0 lệnh ghi | `EV-B1-UI-082-C1.jpg` |
| UI-082 P1 | `TC-R-UI-082-F11` | FAIL | FAIL | đạt trừ Cộng đồng, ghi chú 2 | `EV-B1-UI-082-C1.jpg`, `EV-B1-CUA-DEMO.jpg` |
| UI-087 | `TC-L23-CAI-DAT-DAY` | FAIL | PASS | đạt: quay lại thấy tờ giấy, 1 → 0 hộp thoại | `EV-B1-UI-087-C1.jpg` |
| UI-117 | `TC-R-UI-117-A` | BLOCKED | FAIL | đạt, ghi chú 3 | `EV-B1-UI-005-117-C1.jpg` |
| UI-117 | `TC-R-UI-117-B` | FAIL | FAIL | đạt, ghi chú 3 | như trên |
| UI-120 P1 | `TC-R-UI-120` | FAIL | PASS | đạt: hai phía «Sổ này đã dừng», máy chủ từ chối `409 direct_message_unavailable` | `EV-B1-UI-120-C1.jpg` |
| UI-121 | `TC-R-UI-121` | FAIL | PASS | đạt: sau mã về đúng `/groups/[id]/chat` | `EV-B1-UI-121-C1.jpg` |
| UI-123 | `TC-M-UI-123` | FAIL | PASS | đạt: Back đi qua các tab đã ghé, cả có và không phiên | — (lịch sử trình duyệt) |
| UI-137 | `TC-N14-KHONG-PHIEN` | PASS | FAIL | đạt, ghi chú 4 | — |
| UI-137 | `TC-N14-KHONG-PHIEN-SAU` | FAIL | FAIL | đạt, ghi chú 4 | `EV-B1-UI-137-C1.jpg` |
| UI-136 | (N14 `bang`, cần dữ liệu Cộng đồng) | — | chưa đo | test đơn vị `chia-se-web.test.mjs`, ghi chú 5 | — |
| UI-163 | (không có hàng UI) | — | — | test đơn vị `chip-boi-canh.test.mjs` (mặc định 20, trần 40) | — |
| UI-131 | — | — | — | không sửa máy chủ: `adr-de-xuat/UI-131-…` | — |

Ghi chú cho từng mục, theo thứ tự trong bảng:

1. **UI-049.** Đo trên đợt thu vừa phát của nhóm 20 người (19 phong bì).
   - (a) Trình duyệt không có Web Share: hàng ghi «Đã chép link. Dán vào tin nhắn gửi Chat Test 05.», nút thành «Gửi lại
     cho …».
   - (b) Có Web Share: hàng ghi «Đã mở khay chia sẻ cho Chat Test 05.».
   - (c) Đóng khay: không câu nào.
   - Harness vẫn ghi FAIL vì biểu thức của nó chỉ nhận ra câu lỗi cũ («Không kết nối được…», «chưa rõ đã gửi», «Share is
     not supported») làm bằng chứng cho (a). Câu mới nói đúng việc đã xảy ra (đã chép link).
   - «Trước» không đo lại được trên máy này: phong bì link chỉ nằm trên máy đã phát đợt, nên bản `main` mở cùng đợt thấy 0
     nút. Dùng hàng `TC-L32-VONGDOI` và ảnh `EV-F04-CHIA-SE-KHONG-CO-C1` của QA. Mã `DotThuLive` không đổi từ `461eabf`.
2. **UI-082 F11.**
   - Năm tab và hai màn stack demo đều có nhãn demo và một lối «Đăng nhập».
   - Còn lại đúng một chỗ: Cộng đồng không có nhãn «Dữ liệu demo». Chưa đăng nhập, Cộng đồng **không hiện dữ liệu mẫu**,
     chỉ lời mời đăng nhập, nên gắn nhãn demo là nói sai.
   - Màn stack demo: TopBar có **một** «cửa demo» (bình thí nghiệm + «Demo» + biểu tượng đăng nhập; tên trợ năng «Dữ liệu
     demo. Đăng nhập»), thay cho nhãn và nút rời nhau. Dưới 360dp chỉ còn hai biểu tượng.
   - Lý do: bản đầu của B1 đặt nhãn và nút cạnh nhau, đẩy tiêu đề sổ demo xuống 6px ở 390dp (đo bằng detector Impeccable
     trên trang đã render, rồi đo lại bằng `scrollWidth`). Bản sửa, đo trên đúng bản build B1:
     - tiêu đề không còn bị cắt ở 390 và 320dp;
     - tiêu đề được xuống hai dòng;
     - hai bên TopBar chỉ bằng nhau khi tiêu đề vẫn vừa.
3. **UI-117.** Đã sửa:
   - Back khi sheet mở đóng sheet và **ở lại** `/plan`. Trên `main` cùng phép đo Back rời app (`about:blank`).
   - Sau Back: 0 hộp thoại; tab «Khám phá» không inert; chạm tab «Khám phá» tới `/explore`.

   Hai việc harness còn đếm:
   - **«Vùng inert ≥25%» = 1–2.** Đó là màn tab **không hoạt động** (Khám phá, Cộng đồng) mà navigator tự gắn
     `aria-hidden="true"`. Vùng này có cả **trước khi mở sheet**, và cả trên `main`. Lấy mẫu 96 điểm trên màn: 0 điểm rơi vào
     vùng đó (`kiem-ux/probe117c.mjs`).
   - **117-A chạm tim / thẻ «không đổi».** Kịch bản giả định Back đưa về Khám phá (hành vi cũ). Nay Back ở lại `/plan`, nên
     nó chạm phần tử của tab ẩn.

   Sau Back lần hai về `/explore`: tim lưu được, chạm thẻ tới `/places/banh-can-le` (`kiem-ux/probe117b.mjs`).
4. **UI-137.**
   - Bốn route Cộng đồng mở lạnh không phiên: trước thì đứng ở «Đang mở câu chuyện…» hoặc trang không lối ra. Sau thì tới cửa
     đăng nhập với `?tiep=` của chính route đó; đăng nhập xong quay về đó (cùng cơ chế UI-121).
   - Harness ghi FAIL vì:
     - biểu thức `/\/(login|welcome)$/` không nhận đuôi `?tiep=`;
     - nó tìm một nút «Đăng nhập» **trên** cửa đăng nhập (trang đó là cửa, nút của nó là «Gửi mã»).
   - `TC-N14-KHONG-PHIEN` đổi PASS → FAIL chỉ vì nút «Đăng nhập» của Cộng đồng nay mang `?tiep=%2Fcommunity`.
5. **UI-136.**
   - Bản web chia sẻ link `https` của chính trang (`origin` + `/community/posts/<id>`) qua trường `url` của Web Share; không
     có Web Share thì chép vào clipboard và nút nói «Đã chép link».
   - Trên stack loopback origin là `http://127.0.0.1`, nên hàng `TC-N14-BANG-CHIA-SE-LINK` (đòi `^https://`) chỉ đo được
     trên bản deploy có https.
   - Bản native vẫn `rudi://`: đề xuất hạ tầng ở `adr-de-xuat/UI-136-…`.
   - Chưa chạy hàng N14 `bang` vì cần dữ liệu Cộng đồng đã duyệt (`--chi dang,duyet,bang` cùng vai kiểm duyệt).

**Cổng đã chạy cho B1** (cây sạch = `main` `d95edb4` + đúng 65 file mã của B1, không có gì của B2):
- `npm run typecheck` 0 lỗi; `npm test` 1393/1393 (gồm `build:check`).
- `go test ./...` 157 gói ok. `scripts/go_postgres_tier.sh`: 3283 ca PASS, sentinel có mặt.
- `scripts/check_route_ownership.py`: 227 hàng, 220 Go phục vụ.
- `repo_guard tree HEAD` sạch. pytest `tests/test_repo_guard.py` 39/39 (ảnh có git).
- pytest toàn bộ: 133 fail + 54 error, **đúng cùng tập** với `main` (môi trường container thiếu git/make/ruff), không có hàng mới.
- Parity: xem commit message.

Detector Impeccable trên trang đã render (390×844): `/plan`, `/groups/cap-demo/to-giay`, `/login?tiep=…`.
- **Sửa:** `text-overflow` (tiêu đề sổ demo, ghi chú 2).
- **Dương tính giả, không sửa:**
  - `low-contrast`:
    - logo «Rủ Đi» trên ô cam;
    - chữ ở phần giấy của trang đăng nhập bị tính trên nền bìa navy phía sau;
    - chữ trắng trên ảnh hero có gradient tối;
    - chữ trắng trên nút gradient `accent` (5.5:1).
  - `buried-raster`, `clipped-overflow-container`, `layout-transition`: cách react-native-web dựng `Image` và
    `overflow: hidden`.
- **Giữ theo DESIGN.md:** `undersized-ui-text` 10px của `DemoBadge` thụ động. Cửa demo bấm được thì dùng 12sp.

Rủi ro còn lại của B1:
- Chưa đo trên Android emulator (AVD đang chạy thuộc phiên khác). Ba chỗ chạm native:
  - Back hệ thống đi qua các tab đã ghé;
  - sheet đóng khi màn mất focus;
  - `Share.share` có `url` riêng trên iOS.
- Chưa có iOS.

### B2 · Primitive dùng chung

Đo trên bản web của cây B2, cùng stack và cùng dữ liệu với B1.
- «Trước» của các hàng `retest-main` phần chính là bản B1 (`a6341c19`). Các phần `r-p3-*` (B1 không đổi) lấy hàng `main`
  `d95edb4`.
- Các kịch bản F02, F05, N22 chạy lần lượt trên bản B1 rồi bản B2.

| Issue | Hàng harness | Trước | Sau | Tự kiểm | Ảnh |
|---|---|---|---|---|---|
| UI-003 | `TC-R-UI-003`, `TC-F05-BINH-CHON-PHIEU` | FAIL, FAIL | PASS, PASS | đạt: tab có `aria-selected` trong `tablist`; phiếu bầu `aria-checked` | — |
| UI-004 | `TC-R-UI-004` | FAIL | FAIL | đạt, ghi chú 1 | `EV-B2-UI-004-RAIL.jpg` |
| UI-006 | `TC-R-UI-006` | FAIL | FAIL | đạt, ghi chú 2 | — |
| UI-007 | `TC-R-UI-007` | FAIL (`d95edb4`) | PASS | khay 82% ở C2, C8 | — |
| UI-010 | `TC-R-UI-010` | FAIL (`d95edb4`) | PASS | `/create` mở lạnh: 1 hộp thoại trên Khám phá | — |
| UI-013 | `TC-R-UI-013` | FAIL (`d95edb4`) | PASS | khung cuối: đỉnh 868 > 844, độ mờ 0 | — |
| UI-018 | `TC-R-UI-018` | FAIL | PASS | «Quay lại» mở lạnh về tab của route | — |
| UI-019 | `TC-R-UI-019` | FAIL | PASS | mã mời sai: câu về mã, không «cập nhật app» | — |
| UI-029 | `TC-F02-LOI-503` (câu) | câu chung | câu về máy chủ | đạt | `EV-B2-UI-029.jpg` |
| UI-038 (khay) | `TC-L20-VONGDOI` | FAIL | PASS | Back khi khay mở ở lại chat | — |
| UI-039 (B3) | `TC-R-UI-039` C6 | FAIL (`d95edb4`) | PASS | chạm đúp «Thêm chặng» = 1 sheet (nhờ Sheet v2) | — |
| UI-040 | `TC-L08-KICH-THUOC` C8, `TC-N22-XEM-GHIM` | 93–96% | 82% | đạt | `EV-B2-UI-166-040.jpg` |
| UI-053 | — | — | — | test `trang-thai-tro-nang.test.mjs`; Space ở ghế, danh sách người, phiếu, ngân sách | — |
| UI-066 | `TC-L21-VONGDOI` (Esc) | khay còn | khay đóng | đạt, ghi chú 4 | — |
| UI-070 | `TC-F05-GHIM-TREN-NEN` | PASS | PASS | lớp sheet `zIndex 10` | — |
| UI-072 | `TC-R-UI-072` | FAIL (QA `461eabf`) | PASS | 409: «Người này đã ở trong nhóm hoặc đã được mời rồi.» | — |
| UI-088 | `TC-R-UI-088` | FAIL (QA `461eabf`) | PASS | chạm đúp ⚙: còn 1 sheet | — |
| UI-089 | `TC-R-UI-089` | FAIL (QA `461eabf`) | PASS | axe `aria-prohibited-attr` 0 | — |
| UI-091 | `TC-R-UI-091` | FAIL (QA `461eabf`) | FAIL | đạt: nút «Lưu điều cần tránh» chỉ hiện khi có thay đổi, đúng đề xuất của QA; harness đòi thấy nút | — |
| UI-093 | `TC-R-UI-093` | FAIL (QA `461eabf`) | PASS | tờ 592px, sheet 640px ở C6, C7 | — |
| UI-099 | `TC-R-UI-099` | FAIL | PASS | chip tràn 0 ở C1, C2 | — |
| UI-112 | `TC-R-UI-112` | FAIL (`d95edb4`) | PASS | focus vào tiêu đề ở 5/5 màn | — |
| UI-166 | `TC-N22-XEM-GHIM` | FAIL | FAIL | đạt, ghi chú 3 | `EV-B2-UI-166-040.jpg` |
| UI-001 | — | — | — | `ONhapMuc` 48 (test `o-nhap-muc`); các nút 44 của từng màn ở batch của màn đó | — |

Ghi chú B2:

1. **UI-004 rail.** Harness tìm vạch chỉ báo là anh em của phần tử tab. Từ B2 các tab nằm trong một `tablist` riêng, nên
   harness ghi «vạch không thấy». Đo bằng `kiem-ux/vach-rail.mjs`: tâm vạch so với khoảng dọc của tab đang chọn, 5 tab ×
   C6, C7.
   - Trước bản sửa: 2/10 (tab rail chỉ cao 48 trên web vì `flex: 0` thành `flex-basis: 0`).
   - Sau: 10/10.
   - Ảnh `EV-B2-UI-004-RAIL.jpg`.
2. **UI-006.** Hai chạm cách 60 ms: 1 hộp thoại, đạt. Harness so URL bằng đúng `/create`, nhưng nay khay mang ngữ cảnh tab
   (`/create?tu=plan`). Trên Lên plan, ô dưới ngón tay ở 60 ms là «Đóng», nhưng sheet không nhận chạm trong 250 ms đầu.
3. **UI-166, UI-040 (tấm «Xem» trong chat có dải ghim).**
   - Tấm cao đúng 82% ở C1, C2, C8.
   - Lớp sheet `zIndex 10` nên dải ghim nằm dưới nền mờ.
   - Chạm vào chỗ «Đóng bảng» trúng «Đóng bảng»; chạm dòng đầu trúng tấm.
   - Hàng `TC-N22-XEM-GHIM` vẫn FAIL vì quy tắc của nó là FAIL khi dải có `z-index 1` và tấm khác `1`. Nghĩa là mọi lớp sheet
     cao hơn đều bị chấm FAIL, trái với chính đề xuất sửa của QA («cho lớp sheet zIndex cao hơn»).
   - Ảnh `EV-B2-UI-166-040.jpg`.
4. **Hàng `TC-L21-VONGDOI`.** Kịch bản bấm Esc trước. Esc nay đóng khay (UI-066 đạt), nên Back sau đó không còn khay nào để
   đóng và rời chat là đúng. Back khi khay đang mở được đo ở khay công cụ dùng cùng hook (`TC-L20-VONGDOI` FAIL → PASS).
5. **Tay cầm sheet** ra khỏi cây truy cập (UI-089), nên harness không tìm được nó bằng nhãn «Tay cầm» để kéo.
   - Bản B2 cho nó `testID="tay-cam"`.
   - Bản sao harness của tôi tìm theo cả hai (`thu-vien/lop-phu.mjs`).
   - Kéo ngắn bật về, kéo dài và vuốt nhanh đều đóng (`TC-L08-DONG-*` 7/7 PASS).
   - QA nên cập nhật bộ định vị như vậy.
6. **UI-099, UI-018, UI-019, UI-003:** hàng harness FAIL → PASS. UI-029: câu 503 nay là «Rủ Đi đang gặp sự cố…»
   (`EV-B2-UI-029.jpg`).

Cổng B2 (cây sạch = `main` + đúng các file của B2): xem commit message.

Rủi ro còn lại của B2:
- **Sheet v2** thay hành vi của 39 chỗ gọi: trần cả panel, chặn chạm 250 ms, trượt theo chiều cao. Harness đo F03, F05, F07,
  N22 và khay Tạo mới; các sheet khác chưa được đo riêng.
- **Android: đã đo sau commit B2.** APK debug build mới từ `e182ce64`; JS phục vụ từ worktree sạch đúng `fb6e352b`; AVD
  `rudi-diary-review`, 1080×2400, density 420, chưa đăng nhập. Kết quả:
  - Thanh tab 5 tab + con dấu cột giữa vẽ đúng, vạch ở tab đang chọn; cửa demo «Dữ liệu demo ⇥» trên ảnh hero.
  - Con dấu mở khay: «Tạo cuộc hẹn» đầu khay khi mở từ Lên plan, «Viết bài» cuối; panel dừng ở khoảng 82%.
  - Back hệ thống đóng khay; kéo tay cầm xuống cũng đóng khay.
  - Back hệ thống đi qua các tab đã ghé (Lên plan → Khám phá → Tin nhắn, Back về Khám phá), không thoát app.
- **Hai lỗi nhỏ thấy được trên Android, sửa ở B3:**
  - nhãn «Tạo» thấp hơn nhãn các tab vài px;
  - khi khay mở, đầu màn Lên plan phía sau hiện cửa «Demo ⇥» vì `TopBar` đọc segment toàn cục (`/create`) thay vì
    navigator của chính màn.
- **Lên plan** có hai lối mở cùng một khay: nút «Tạo mới» ở đầu màn và con dấu. Để B3 quyết có bỏ nút đầu màn hay không.
  B3 quyết giữ (change-log B3).
- **Con dấu «Tạo mới» (01/10, sau B3):** chủ sản phẩm đánh giá thiết kế con dấu tròn ở thanh tab chưa đạt và đang vẽ mockup
  thay thế.
  - Sau đó (01/10) chủ sản phẩm nhận luôn phần thanh tab dưới cùng: **chuyển giao, ra khỏi danh sách việc của đợt này**.
  - Đừng retest phần hình của thanh tab theo bản B2.
  Hành vi (mọi tab có lối Tạo mới, khay theo tab, `/create` mở lạnh) không đổi.

### B3 · Pilot: Kèo · Lên plan · Hành trình (F03 + E2)

Đo trên bản web của cây B3, cùng stack và dữ liệu với B2.
- **«Trước»:** bản B2 `e182ce64`, phục vụ ở cổng của harness. Chạy kịch bản `f03-keo.mjs` đầy đủ và `retest-main.mjs --chi
  r-f03,r-p3-f03,r-e,r-p3-e` (`out/b3-truoc`).
- **«Sau»:** bản cuối của B3 (`out/b3-sau2`), cùng `f03-keo.mjs` đầy đủ và `r-f03,r-p3-f03`. Các hàng E2 (UI-118, UI-119,
  UI-121) lấy từ lượt trên bản B3 trước đợt sửa cuối (`out/b3-sau`): đợt sửa cuối chỉ đổi trang ngày trên bản đồ, không chạm
  mã của chúng.
- **Probe riêng** (`kiem-ux/b3-probe.mjs`, `b3-probe-043.mjs`), cho những gì harness không đo được trên giao diện hiện tại:
  - nền mờ của sheet phủ đầu màn;
  - nút «Xếp»;
  - `aria-expanded` của nút gập;
  - dòng nguồn bản đồ;
  - tên điều khiển MapLibre;
  - Esc trên popup cụm.

| Issue | Hàng harness / probe | Trước | Sau | Tự kiểm | Ảnh |
|---|---|---|---|---|---|
| UI-032 | `TC-R-UI-032`; probe UI-032 | FAIL | PASS | 3 ngày đều nêu «3 chặng của kèo chưa xếp ngày: Cà phê sáng, Ăn trưa, Tối nướng.»; «Xếp cả 3 chặng vào ngày này» → mốc hiện, «Lưu cho cả hội» hiện, cả hội chưa thấy | `EV-B3-UI-032-C1.jpg` |
| UI-033 | `TC-R-UI-033` C1–C4, C8; `TC-F03-VE-LICH-TRINH` | FAIL (C8 thấy 40%) | PASS (C8 100%) | câu giải thích trong màn ở cả 5 cấu hình (probe) | `EV-B3-UI-033-C8.jpg` |
| UI-034 | `TC-R-UI-034`; `TC-F03-TAO-NGAN-SACH-TRONG` | FAIL, FAIL | PASS, PASS | câu «Chọn một mức hoặc gõ số tiền mỗi người.» ở y 676, ngay dưới ô | `EV-B3-UI-034-C1.jpg` |
| UI-035 | `TC-R-UI-035`; `TC-F03-DEMO-TIMELINE` | FAIL, FAIL | PASS, PASS | có phiên → `/outings/[id]` | — |
| UI-036 | `TC-R-UI-036`; `TC-F03-SAP-XEP-PHIM` | FAIL, FAIL | PASS, PASS | tay nắm `slider` `tabindex 0`; mũi tên xuống dời chặng | — |
| UI-037 | `TC-F03-NEP-LICH-TRINH` | FAIL | PASS | Android: dải Nếp ở Lịch trình (trước: vắng) | `EV-B3-UI-037-AND.jpg` |
| UI-039 | `TC-R-UI-039` C1, C6 | PASS (B2) | PASS | nút chỉ mở | — |
| UI-041 | probe UI-041 | — | đạt | điểm ở giữa «Quay lại» rơi vào lớp sheet; đầu màn `inert`; Android nền mờ phủ đầu màn | `EV-B3-UI-041.jpg` |
| UI-042 | `TC-R-UI-042` C1, C3 | FAIL (allowed-attr ×3, required-attr ×3) | PASS (0 critical) | — | — |
| UI-043 | `TC-F03-BAN-DO-NHAN`; probe UI-043 | FAIL | FAIL; đạt | ghi chú 1 | `EV-B3-UI-046-043-C1.jpg` |
| UI-044 | `TC-F03-META` C1, C2, C4, C5 | FAIL (2/3 dòng cắt) | PASS | — | `EV-B3-UI-044-045-C2.jpg` |
| UI-045 | `TC-F03-COT-CHANG` C1, C2 | FAIL (118px, 48px) | PASS (222px, 152px) | — | `EV-B3-UI-044-045-C2.jpg` |
| UI-046 | `TC-F03-CHON-THIEU` | FAIL (skeleton) | PASS | câu và «Mở Khám phá» | `EV-B3-UI-046-043-C1.jpg` |
| UI-047 | `TC-F03-DAU-MAN` C6, C7 | PASS (B2) | PASS | lệch 8px = lề của nút | — |
| UI-118 | `TC-R-UI-118` | FAIL | PASS | tên kèo và lối mở trong chat của người tạo và của `moi-61` | `EV-B3-UI-118-C1.jpg` |
| UI-119 | `TC-R-UI-119` | FAIL | FAIL | ghi chú 2 | — |

Ghi chú B3:

1. **UI-043.** Harness tìm `.maplibregl-ctrl button`. Bản đồ đã bỏ la bàn từ trước, và nút nguồn của MapLibre là một
   `<summary>`, nên harness thấy «không có» nút nào và chấm FAIL.
   - Probe đọc mọi điều khiển: «Bản đồ hành trình» (canvas), «Hiện hoặc ẩn nguồn bản đồ» (summary).
   - Thu nhỏ cho hai chặng gộp thành cụm «2 điểm gần nhau: 1, 2», chạm mở popup có nút «Đóng danh sách điểm gần nhau».
   - Esc: còn 0 popup, focus về đúng nút cụm.
2. **UI-119.** Phần UI đã làm:
   - sau «Tôi đã tới» có «Thêm khoảnh khắc ở đây», mở màn thả khoảnh khắc gắn sẵn nhóm và quán;
   - album nói «N check-in ở M chỗ»;
   - tường trống phân biệt «Tôi đã tới» với check-in.

   Tiêu chí của QA đòi album đếm lượt tới chặng, hoặc tường có dấu của lần tới đó. Đó là đổi luật máy chủ, nên đã viết
   thành ADR đề xuất (`adr-de-xuat/UI-119-da-toi-la-check-in.md`) và chờ chủ sản phẩm. Hàng sẽ FAIL tới khi có quyết định.
   Harness còn tìm chữ «chỗ đã tới», mà album nay cố ý không dùng nữa.
3. **`TC-F03-BAN-DO-CHANG-NHIEU-NGAY`** vẫn FAIL vì chỉ đếm mốc trên bản đồ. Thiết kế B3 không đoán ngày cho chặng: nó nêu
   tên chúng và để người dùng xếp. Hàng retest `TC-R-UI-032` chấm theo đúng tiêu chí của QA («hoặc có dòng nói rõ các
   chặng chưa xếp ngày») và PASS.
4. **`TC-L13-GAP`** tìm nút gập bằng chữ «Các chặng trong ngày», thiết kế cũ. Nút nay là «Thu gọn trang ngày» / «Mở trang
   ngày», `aria-expanded` true → false (probe).
5. **`TC-MO-M5` C9** (Nếp đổi ảnh một lần ở 538 ms khi giảm chuyển động) thuộc UI-027, batch B5.
6. **`TC-L11-GIU`** BLOCKED: CDP không sinh `contextmenu` từ cú giữ; cần Chrome Android thật.
7. **Dòng nguồn bản đồ** (không phải issue QA; hồi quy của chính B3, probe bắt được):
   - Trên bản B3 trước đợt sửa cuối, ở ngày trống, trang ngày che dòng «© OpenStreetMap · OpenFreeMap».
   - Bản cuối: nguồn thấy ở kèo nhiều ngày C1, kèo một ngày C1 và kèo trống C1.
   - Ở C8 ngày trống, bản đồ ẩn hẳn (0×0), nên không có dải bản đồ nào thiếu nguồn.
8. **Critique (Flow D)**, hai subagent cô lập mỗi lần: **24/40 → 26/40**. Archive ở
   `.impeccable/critique/*luong-keo-len-plan-hanh-trinh.md`, chỉ trên máy này (thư mục bị gitignore). Phần detector chạy trên snapshot DOM. Snapshot mất stylesheet
   của react-native-web (chèn qua CSSOM, không có trong text của thẻ `<style>`), nên phần lớn finding là do cách chụp. Đã
   dựng lại stylesheet để phân loại, và đã sửa script chụp cho các lượt sau.

9. **Finish review** (`impeccable-finish-reviewer`, ngữ cảnh mới):
   - Lượt đầu: `fix`, 8 điểm. Đã sửa một đợt, dựng lại và chụp lại.
   - Verdict pass: 6 resolved, 2 partial, 1 hồi quy.
   - Ba điểm còn lại được chỉnh một đợt cuối và kiểm bằng ảnh của người sửa, **không** chấm lại bằng reviewer
     (change-log B3, «Verdict pass»).
   - Popup cụm điểm trong bản cuối là mẩu giấy có nút đóng 48×48. Focus vào chặng đầu; Esc đóng và trả focus về cụm.

Cổng B3 (cây = `main` + đúng các file của B3): xem commit message.

Rủi ro còn lại của B3:
- **`KheLop`** mới chỉ dùng cho hai sheet của bản đồ. Ô nhập trong khe được cập nhật cùng commit:
  - `khe-lop.test.mjs` đo ở mức React;
  - `rudi-hanh-trinh-web.test.mjs` gõ vào «Giờ xuất phát» thật trên Chrome.

  Chưa đo gõ nhanh trên bàn phím Android.
- **Bản đồ web không còn sàn 220dp.** Ở cửa sổ thấp có mốc, bản đồ cao đúng phần chỗ còn lại (ở C8 khoảng 90dp), và lề
  khớp camera co tối đa một phần tư mỗi cạnh. Chưa có ảnh C8 của ngày có mốc.
- **Dải kèo sắp tới trong chat** đọc `GET /contexts/{id}/outings` mỗi lần chat được focus: thêm một lượt đọc mỗi lần mở
  chat.
- **Android:** đã xem trên emulator ở chế độ demo:
  - sheet sửa trang ngày phủ đầu màn;
  - dải Nếp ở Lịch trình;
  - nhãn «Tạo» thẳng hàng;
  - đầu màn sau khay không còn cửa demo.

  Chưa xem bản live (cần đăng nhập trên emulator): form tạo kèo, dải kèo trong chat, gợi ý ghim điểm.
- **Còn mở từ critique, chưa làm trong pilot:**
  - lịch trình kèo nhiều ngày chưa chia theo ngày;
  - tiền đứng trước lịch trình (B4/B11);
  - «Tôi đã tới» trước ngày đi (luật, hỏi chủ sản phẩm);
  - sheet sửa chặng ~20 điều khiển;
  - từ vựng chặng/điểm/hoạt động (B11);
  - tiêu đề kèo 28/34 dòng cao 1.21× ở 320.

### B4 · Tiền: chia bill · quyết toán · đợt thu · tài chính (F04 + E3)

Đo trên bản web, cùng stack riêng. **Mỗi bản đo trên một thế giới vừa dựng lại** (`reseed.sh`), cùng thứ tự kịch bản.
Lượt baseline đầu (`out/b4-truoc`) chạy trên trạng thái B3 để lại, nên PHAT và L32 hỏng vì trạng thái, không vì mã.
- **«Trước»:** bản main `bc8dbdfb` (B3), `f04-tien.mjs` đầy đủ và `retest-main.mjs --chi r-p3-f04` (`out/b4-truoc-sach`).
- **«Sau»:** bản cuối của B4 (`out/b4-cuoi3`, bản `b4h`), cùng hai kịch bản, cộng probe `kiem-ux/b4-probe.mjs` và
  `kiem-ux/b4-chup-rong.mjs`.
- Các bản giữa chừng:
  - `out/b4-sau`: ba chỗ yếu về hình (tên ghế, cuống bị ép, hàng đợt thu cao);
  - `out/b4-cuoi2`: sau finish review;
  - `out/b4-cuoi2-anh`: lượt `--chi anh` trên `b4g`.

  Mỗi bản sửa một đợt rồi mới đo bản kế.

- **Tổng:**
  - `f04-tien.mjs`: **39 PASS / 20 FAIL → 52 PASS / 7 FAIL** (59 hàng, 0 BLOCKED).
  - `r-p3-f04`: 0/2 → 1/2.
  - Bảy hàng còn FAIL đều có lý do ở ghi chú: bộ định vị cũ, đo không cuộn, verdict viết cứng, hoặc thuộc batch khác.
- **«Sau» là bản `b4h`.** Bản này có thêm đợt chỉnh sau finish review và verdict pass. Lượt `out/b4-cuoi3` đo trên thế
  giới vừa dựng lại.

| Issue | Hàng harness / probe | Trước | Sau | Tự kiểm của người sửa | Ảnh |
|---|---|---|---|---|---|
| UI-048 | `TC-F04-MON-DAI` C2; probe `B4-BAN-DAI` | FAIL (B3 «12.345.678đ» còn 68px) | PASS | thẻ món không cắt số ở C1, C2, C6; quyết toán demo «1.106.250đ» đủ ở 320 | `EV-B4-UI-050-048.jpg` |
| UI-050 | `TC-F04-BAN-20` C1, C2, C6; probe `B4-BAN-DAI` | FAIL (10/17/6 ghế nhầm; chạm «07» đổi «08») | FAIL theo kịch bản (6/12/2, mọi chỗ là `null`) · probe đạt | ghi chú 3; probe cuộn từng ghế: 0/20 nhầm ở cả 3 cấu hình, hộp chạm nhỏ nhất 69dp, 0 cặp hộp chồng | `EV-B4-UI-050-048.jpg`, `EV-B4-AND.jpg` |
| UI-051 | `TC-F04-CHAN-TEN`, `CHAN-NGUOI`, `BILL-503` | FAIL ×3 (câu ở y −267, −275, −597) | PASS ×3 | câu chặn ở footer ngay trên nút; dòng có lỗi mở ra, ô cần sửa có câu và con trỏ | `EV-B4-UI-052-056.jpg` |
| UI-052 | `TC-F04-LUI-VE-BUOC1`, `BACK-TRINH-DUYET`, `TAI-LAI` | FAIL ×3 | FAIL (ghi chú 4), PASS, PASS | tải lại hay Back rồi Forward: về đúng bước Gán món; về bước 1: tờ «Bill đang gõ» là nút tiếp tục | `EV-B4-UI-052-056.jpg` |
| UI-054 | `TC-F04-QT-20` C1, C2 | FAIL (25, 32 cặp nhãn đè) | PASS | 20 người: một câu thay sơ đồ; 8 người ở 320: tên gọi | `EV-B4-UI-054-061.jpg` |
| UI-055 | `TC-F04-NEP-M2-KHUNG` C1–C3 | FAIL (ra ngoài 43/113/73px) | PASS | — | `EV-B4-UI-055.jpg` |
| UI-056 | `TC-F04-ANH-DOC` | FAIL | PASS | câu sau 268 ms, chỉ tới «Nhập tay», và «Nhập tay» là nút chính | `EV-B4-UI-052-056.jpg` |
| UI-057 | `TC-F04-NEP-MEP` | FAIL (mép là nút «chạm để kéo ra», chạm không làm gì) | PASS | mép không role, không nhãn, không Tab stop | — |
| UI-058 | `TC-F04-DOT-RONG`; probe `B4-DOT-RONG` | FAIL (nút mời, máy chủ từ chối) | FAIL viết cứng · probe đạt | ghi chú 2: không nút, câu nói vì sao, route đếm 0 | `EV-B4-UI-058.jpg` |
| UI-059 | — (không có hàng harness) | ảnh: «Chat Test …», mất «(trả)» | ảnh: tên đủ, «ĐÃ TRẢ» dòng dưới | trang «Đã ghi» | `EV-B4-UI-059.jpg` |
| UI-060 | `TC-R-UI-060` | FAIL | FAIL theo bộ định vị (ghi chú 1) | mục cũ đã bỏ; dưới trang sổ chỉ còn chú thích nguồn số và «Xem quyết toán» | — |
| UI-061 | `TC-R-UI-061` C2 | FAIL (9 dòng) | PASS (đầu sổ 1 + 1 + 1 dòng) | — | `EV-B4-UI-054-061.jpg` |
| Bước «Kết quả» | probe `B4-KET-QUA` | — | đạt | «Cộng 20 phần 13.705.678đ … 18 phần được làm tròn lên 1đ»; con dấu người trả hạ đúng hàng | `EV-B4-KET-QUA.jpg` |
| Đợt thu | probe `B4-DOT-THU` | 19 nút đầy chiều ngang, 19 dấu «CHƯA CHUYỂN» | đạt | 1 mục «Chuyển cho …», 0 dấu «chưa chuyển», 0 mũi tên, không mắt chữ; 18 nút «Đã về» 94px | `EV-B4-DOT-THU.jpg` |
| Bill 0đ (ngoài QA) | Android | «lỗi của app» | đạt | chặn ở bước xem lại, ô tiền có câu và con trỏ | `EV-B4-AND.jpg` |

Ghi chú B4:

1. **Bộ định vị đổi theo nhãn mới** (bản sao harness ngoài repo, không sửa ý của hàng):
   - Nút hàng ở đợt thu nay là «Đã về» / «Gửi» / «Gửi lại». Tên đầy đủ («Tiền đã về từ X», «Gửi cho X») nằm ở
     `aria-label`. `f04-tien.mjs` tìm theo chữ hiển thị, nên bản sao được sửa để tìm theo tên truy cập (`aria-label`, rồi
     mới tới chữ), 7 chỗ, ghi chú tại chỗ.
   - `TC-R-UI-060` tìm tiêu đề «Chi theo nhóm», nay là «Ai nợ ai» nên báo «không thấy mục». Tiêu chí của QA là «tiêu đề
     mục khớp nội dung bên dưới»: bên dưới là «Người khác đang nợ bạn …đ» và lối «Xem quyết toán».
2. **`TC-F04-DOT-RONG` có `status: "FAIL"` viết cứng.** Kịch bản ghi lại điều QA thấy, không kiểm nút có hay không.
   Trong log, `nut` vắng (không tìm thấy nút «Tạo đợt thu từ sổ»), số đợt 1 → 1, không câu từ chối. Probe `B4-DOT-RONG`
   đọc thẳng: nút không có, câu «Mọi khoản đã ghi đều đã vào một đợt thu ở trên: chưa có gì mới để gom.», route đếm 0.
3. **`TC-F04-BAN-20`** đo bằng `elementFromPoint` không cuộn. Bàn dài cao hơn khung, nên ghế dưới mép màn trả `null` và
   bị đếm là «trúng ghế khác». Chạm thật vào ghế bị đếm sai đổi **đúng** ghế đó. Probe `B4-BAN-DAI` cuộn từng ghế vào khung
   rồi mới đo.
4. **`TC-F04-LUI-VE-BUOC1`**: khi có bill đang gõ, bước 1 không còn «Nhập tay» trần mà là tờ «Bill đang gõ» (chạm để tiếp
   tục), «Chọn ảnh bill khác», «Bắt đầu bill mới» (hỏi trước khi bỏ). Kịch bản chạm «Nhập tay» nên không tìm thấy nút.
   Tiêu chí «bill đã gõ còn nguyên, hoặc có câu hỏi trước khi bỏ» đạt theo thiết kế.
5. **Back của trình duyệt** vẫn rời luồng (về `/plan`), không lùi một bước trong luồng. Bill không mất (Forward hay mở lại
   đều về đúng bước), nên không cần hỏi. Đưa từng bước vào lịch sử trình duyệt là việc của điều hướng (B10/B11).
6. **`TC-MO13-M3` C9** (Nếp đổi ảnh khi giảm chuyển động) thuộc UI-027, batch B5. **`TC-L32-VONGDOI`**: UI-049, xem ghi
   chú 1 của B1.
7. **Tầng PostgreSQL của Go.**
   - Lượt đầu đỏ ở một test không liên quan: `nepnho.TestQuenKhiConHangThiThuLai:420`. `t.Fatal` trong lúc còn giữ
     transaction mở, nên `pool.Close()` của cleanup chờ mãi, và test treo 21 phút. Cùng lúc máy đang chạy harness, export
     và reseed.
   - Chạy lại gói đó: PASS (0.02 s). Chạy lại trọn tầng khi máy rảnh: **3319 PASS, sentinel có mặt, exit 0**.
   - Test chập chờn này, và việc nó treo thay vì đỏ, là việc mở ngoài B4.
8. **Android** (emulator, dev client nối stack riêng, đăng nhập bằng OTP của stack riêng):
   - đã xem chặn món 0đ tại dòng, bàn dài 20 người và dải phiếu «Kết quả»;
   - chưa xem trang đợt thu trên Android (cần tạo và phát đợt trên máy), bàn tròn, và chế độ tối.
9. **Bộ định vị ngoài F04 cũng đổi theo:**
   - tiêu đề quyết toán nay là «Quyết toán» (`f11-demo.mjs` liệt kê «Quyết toán chuyến đi» trong regex chữ cần đo);
   - bảng đợt thu không còn «A → B» và «chưa chuyển»;
   - 6 flow Maestro trong repo đã sửa cùng commit.
10. **Thiết kế (luật của chủ sản phẩm 01/10)** được làm thành ba vòng:
    - tự đánh giá màn tiền;
    - finish review ngữ cảnh mới: `fix`, 8 điểm;
    - verdict pass: 6 resolved, 1 partial, 1 unresolved và hồi quy. Hai điểm cuối đã chỉnh, kiểm bằng test hình học
      (`hinh-tien`, cả hộp đĩa), ảnh và probe, **không** chấm lại bằng reviewer.

    Chi tiết ở `change-log.md` mục B4.
11. **Detector** (`impeccable detect`) trên 7 file màn đổi: 0 finding. Đây là quét nguồn; StyleSheet của RN phần lớn nằm
    ngoài tầm luật, nên chỉ là bằng chứng yếu. Không quét được URL vì màn cần phiên.
12. **Còn mở:**
    - tài chính theo từng nhóm (cần chủ sản phẩm chọn ngữ nghĩa, route Go đọc tổng hợp);
    - Back của trình duyệt lùi trong luồng chia bill (B10/B11);
    - test `nepnho` chập chờn và treo;
    - câu hỏi truy hồi sổ tay «bo fieu o dau v» nằm sát mép top 5.

### B5 · Khám phá (F02 + F10)

Đo trên bản web, stack riêng, thế giới dựng lại trước mỗi lượt chính.
- **«Trước»:** main `33daaa38` (B4), `f02-kham-pha.mjs` đầy đủ và `retest-main --chi r-f02,r-f09` (`out/b5-truoc`),
  cộng `r-p3-f02` (`out/b5-truoc-p3`).
- **«Sau»:** bản B5 (`out/b5-sau`), cùng các kịch bản. Các lượt kiểm lại có mục tiêu trên bản cuối:
  - `b5-sau-them`: `r-p3-f02`, F02 `cat-chu`, `r-f10`, F10 đầy đủ;
  - `b5-b`: sân khấu, F03 `tao`, F04 `c9`;
  - `b5-d`: dòng giá.
- **F10 (bảng dev)** cần server dev có `EXPO_PUBLIC_RUDI_FIXTURE=1`, không chạy trên bản export. «Sau» chạy trên server
  dev của cây B5 (cổng 8171). «Trước» lấy theo retest của QA trên main: các file F10 đo (`HangDiaDiem`, `CanhGap`,
  `KhungSkia`) không đổi từ bản đó tới `33daaa38`.

| Issue | Hàng harness | Trước | Sau | Tự kiểm của người sửa | Ảnh |
|---|---|---|---|---|---|
| UI-021 | `TC-R-UI-021` C1–C6; `TC-F02-META` C1–C7 | FAIL (8/8 dòng cắt ở C1–C4) | PASS (0/8 ở cả 7 cấu hình) | giá tìm theo loại, dòng riêng; giờ mở không gãy giữa khung giờ | `EV-B5-UI-021-023.jpg` |
| UI-023 | `TC-R-UI-023`; `TC-F02-NHAN-LUU` C1–C7 | FAIL (thiếu 10–39px) | PASS («Lưu» 27px, thiếu 0) | demo: hai nút xếp theo nhãn, xuống dòng khi không vừa | `EV-B5-UI-021-023.jpg` |
| UI-024 | `TC-R-UI-024`; `TC-F02-AI-MAU` | FAIL («0 kết quả») | PASS (con trỏ vào ô, danh sách 8 nơi) | `source none` nói «chưa trả lời được câu này», không đoán lỗi ở câu | `EV-B5-UI-024.jpg` |
| UI-025 | `TC-F02-NHAY` C1, C9 | FAIL (nhảy 149dp) | PASS (0dp) | khung mang tỉ lệ của bức vẽ từ lần dựng đầu | — |
| UI-026 | `TC-MO12-BO-LOC` C1 (ảnh khung) | sân khấu gỡ rồi dựng lại từ phẳng ~550 ms | gập/mở theo chiều cao ~250 ms, không mount lại | C9: hai khung, đứng sẵn | `EV-B5-UI-026.jpg` |
| UI-027 | `TC-MO-M5` C9; `TC-MO13-M3` C9; bảng dev «Chạy lại» C9 | FAIL (Nếp 2 ảnh; khung trống 603/704 ms) | PASS (1 ảnh, `vẽ: svg`); bảng dev 8/8 khung có sân khấu | giảm chuyển động: chỉ SVG | `EV-B5-UI-113-027.jpg` |
| UI-028 | ảnh `EV-F02-HOI-AN` | «Xóa lọc» khi không có lọc | «Hội An chưa có địa điểm nào» + «Đổi điểm đến» | dòng gu không in trên danh sách rỗng | `EV-B5-UI-028-030.jpg` |
| UI-029 | ảnh `EV-F02-LOI-503`, `EV-F02-OFFLINE` | — | — | đã đạt từ B2: 503 và mất mạng hai câu khác nhau | — |
| UI-030 | ảnh `EV-F02-OFFLINE` | danh sách bị thay bằng màn lỗi | danh sách còn, một câu «Chưa cập nhật được danh mục: …» | — | `EV-B5-UI-028-030.jpg` |
| UI-031 | `TC-R-UI-031` C6, C7 | FAIL (2 cột) | PASS (3 cột) | `cotDiemDen` | `EV-B5-UI-031.jpg` |
| UI-113 | `TC-R-UI-113` C1, C4, C2; `TC-F10-SO-SANH-TIM` | FAIL (QA) | PASS (tim 48px thấy trọn ở 320) | — | `EV-B5-UI-113-027.jpg` |
| UI-114 | `TC-F10-TIM-LONG` | FAIL (QA) | PASS (0 chỗ lồng ở cặp so sánh) | lỗi axe còn lại là `div[aria-label="Giờ chặng"]` của bàn xoay giờ, issue khác (B9) | — |
| UI-115 | `TC-F10-KEO-DOC-TREN-TRANH` C1, C9 | FAIL (QA, cuộn 0 → 0) | PASS (cuộn 0 → 175) | `touchAction` theo hướng kéo | — |

Ghi chú B5:

1. **Bộ định vị:** `f02-kham-pha.mjs` tìm «Lưu địa điểm|Đã lưu». Nút nay ghi «Lưu», tên truy cập «Lưu địa điểm này»; bản
   sao harness được thêm «Lưu» vào regex, như `retest-main` của QA đã có sẵn. Lượt `b5-sau` thiếu 7 hàng `NHAN-LUU` vì
   chưa sửa regex; lượt `b5-sau-them` và `b5-d` đo đủ.
2. **Không làm lại hình Khám phá:** chủ sản phẩm có spec làm lại đầu tab Khám phá đang chờ duyệt (`claude/thanh-tab-5-cot`).
   Batch này không đụng hàng tiêu đề, tiêu đề mục kết quả hay kiểu thẻ đầu. Khi spec được code, phải gộp với các thay
   đổi trong `ExploreLive.tsx`/`HangDiaDiem.tsx` của B5.
3. **F10, các hàng FAIL không thuộc B5:** `ALBUM-XEM`, `XEM-ANH` (ảnh 390×0 trong trình xem, UI-094, B9),
   `BAN-XOAY-LONG` (B9), `TAM-STICKER` C2 (ellipsis).
4. **Còn mở:** hàng `TC-MO12-BAT`, `BO-LOC` của QA là hàng đọc ảnh («cần đọc ảnh ghép»); tôi đã đọc ảnh và ghi ở bảng.
   `TC-R-UI-107` thuộc B9.
5. **Android:** chưa xem Khám phá trên emulator ở batch này.
