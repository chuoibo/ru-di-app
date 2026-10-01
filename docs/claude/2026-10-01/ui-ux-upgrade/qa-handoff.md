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
- **Android** chưa đo: APK debug cũ hơn `expo-video` nên cần build native mới.
- **Lên plan** có hai lối mở cùng một khay: nút «Tạo mới» ở đầu màn và con dấu. Để B3 quyết có bỏ nút đầu màn hay không.
