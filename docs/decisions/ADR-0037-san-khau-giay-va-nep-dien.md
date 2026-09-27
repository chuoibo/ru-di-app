# ADR-0037 — Sân khấu giấy: cuốn sổ pop-up, Nếp diễn khoảnh khắc, Skia

- **Trạng thái:** 🟢 **ĐÃ CHẤP NHẬN** 2026-09-24 — Lead chốt bốn quyết định trong phiên 24/09 sau báo cáo QC
  `docs/claude/2026-09-24/qc-frontend-chieu-sau-va-cau-chuyen.md` (commit `9036001`).
- **Quyết định bởi:** Lead. **Hiện thực:** Claude, chiến dịch UI v3 trên nhánh `claude/practical-faraday-mswgmv`;
  kế hoạch chép ở `docs/architecture/04-ui-v3-san-khau-giay.md`.
- **Sửa:** ADR-0020 §2.4 (chuyển động), §2.5 (avatar), §3 (màu mới đi qua `guest.css`); luật DESIGN.md «Nếp Đứng
  Xa Tiền» (1286-1291), «Trong Trang / Trên Trang» (Elevation), quy ước «màu avatar theo tông màn» trong
  `ui/Avatar.tsx`.
- **Không đụng:** ADR-0035 (dock Nếp trong lề sổ), ba luật tiền, cam kết thương hiệu (wordmark, logo
  `#fc7b37 → #e75262`, ba tông mang nghĩa, «Rủ Đi thôi!»), «Một Tông Dẫn», «Mực Tĩnh trên Coral», «Một Face»,
  «Không Kicker», route/API/domain.

## 1. Bối cảnh

QC 24/09 đo trên thế giới seed thật (không phải fixture): sau đăng nhập mọi màn cùng một khuôn form — 22/41 màn
`TopBar` + `Heading`, `RudiButton` 218 lần, `StampButton` 2 lần; kho vẽ (24 tư thế Nếp, 10 cảnh, 8 sticker) chỉ
hiện khi màn rỗng; 7 màn không có một nét vẽ, dấu hay ảnh nào (trong đó Kèo mới, Nhóm mới, Mời); Nếp chỉ còn mép
giấy 10dp; chữ pháp lý đứng ở chỗ của câu chuyện; kết thúc một luồng là một con tem 26dp và 60% màn trống.

Gốc không nằm ở từng màn mà ở các luật đã ghi: Nếp chỉ ở trạng thái rỗng và cửa vào, không bao giờ cạnh tiền;
`celebrate` chỉ cho ba khoảnh khắc; avatar không mang màu người; chỉ `KhungAnh` có bóng; `Field` có viền là ô
nhập duy nhất; và mọi vòng review mỹ thuật chạy trên fixture («chưa phủ: các cửa live», DESIGN.md đợt 08/09).
Lead yêu cầu làm lại toàn bộ UI cho đẹp, giàu hình, sáng tạo, chuyển động tốt, ít chữ, ít ô, thật sự responsive
và đúng câu chuyện của Nếp. Charter cấm hợp thức hoá hậu nghiệm, nên ADR này mở **trước** khi màn đầu tiên đổi.

## 2. Quyết định

### D1. Hướng: «Sân khấu giấy» — cuốn sổ chuyến đi thành sách pop-up

Bìa vải indigo giữ nguyên; mở ra là trang giấy dày có nếp gấp giữa; mỗi màn là một **sân khấu** gồm nhiều
**tầng** giấy cắt dựng lên từ nếp gấp. Từ vựng dùng chung trong mã và tài liệu:

| Từ | Nghĩa |
|---|---|
| sân khấu | cảnh nhiều tầng của một màn |
| tầng | một lớp giấy cắt, có độ sâu `sau` và đường gập `nep` |
| đạo cụ | vật đứng trên sân khấu (bàn, đĩa, máy ảnh, vé…) |
| con rối | Nếp có khớp đinh ghim |
| hình nhân | người (avatar) đứng trên đế |
| tab kéo, nắp, đĩa xoay, cuống | cơ chế sách pop-up dùng làm tương tác |

Mỗi việc là một vật giấy: hội = bìa sổ, mời = phong bì, kèo = thiệp/vé, chia bill = hoá đơn nhiệt, kết quả =
cuống phiếu, đợt thu = trang sổ thu, sổ hai người = thư gấp ba, huy hiệu = tem, hồ sơ = hộ chiếu, kỷ niệm = ảnh
instax, story = polaroid. Ba tông là ba loại giấy màu (cam = giấy mời, teal = giấy tiền, tím = giấy can của AI).

### D2. Độ cao giấy thay «Trong Trang / Trên Trang»

| Mức | Là gì | Bóng |
|---|---|---|
| 0 | in trên trang: chữ, dấu, hàng, kẻ | không |
| 1 | dán lên trang: ảnh in, vé, hoá đơn, cuống | `cardShadow` như hôm nay |
| 2 | đứng dựng: tầng pop-up, hình nhân | bóng đổ lên trang, tỉ lệ với góc gập, bằng 0 khi nằm phẳng |
| 3 | đang cầm lên: vật dưới ngón tay, sheet | lớn và mềm hơn |

Một nguồn sáng duy nhất từ trên-trái; ở theme tối là đèn bàn. Bóng lệch cứng vẫn cấm. «Không Thẻ Lồng Thẻ» giữ;
hàng vẫn ở mức 0. Số đo nằm ở `tokens.json` → `sanKhau.cao`.

### D3. Ngân sách chuyển động

- `MOTION_MS` giữ đúng bốn bậc 100/200/300/550 (`tests/motion.test.mjs` ghim).
- Thêm ngân sách ghép `tokens.motion.sanKhau`: `batTang` 40ms so le mỗi tầng (tối đa 4 tầng so le),
  `batToiDa` ≤ 420ms (một cú bật dựng = `shared` + so le), `dien` ≤ 1400ms (một tiết mục của Nếp), `lat` = `shared`.
- Chuyển động **gắn với input** (gập theo cuộn, thị sai theo kéo, nghiêng máy) không có thời lượng và dừng khi tay
  dừng; không phải «ambient loop». `withRepeat(-1)` chỉ được trong `Skeleton`.
- Nghiêng máy: chỉ native, chỉ 3 sân khấu anh hùng (Welcome, thành phố ở Khám phá, sân khấu kèo), ≤ 3dp và 2°,
  lọc thông thấp, dừng khi màn mất focus hay app xuống nền, tắt khi Giảm chuyển động.
- Giảm chuyển động: bật dựng và tiết mục → khung cuối tĩnh; lật trang → cắt thẳng; gập theo cuộn → mờ dần, không 3D
  (WCAG 2.3.3); thị sai → tắt; haptic giữ.

### D4. Một lần mỗi sự kiện

`celebrate` nay phủ các cú dập dấu đã có cộng tám khoảnh khắc của D5. «Một lần» tính theo **khoá sự kiện** trong
sổ đăng ký cấp module của phiên app: mount lại không phát lại. Huy hiệu lưu danh sách đã thấy.

### D5. Nếp diễn khoảnh khắc (thay «Luật Nếp Đứng Xa Tiền»)

Nếp là **diễn viên trong trang** ở đúng tám khoảnh khắc, cộng trạng thái rỗng và cửa vào như cũ:

| # | Khoảnh khắc | Tiết mục | Mặt |
|---|---|---|---|
| M1 | khay tạo mở (lần đầu trong phiên) | `buoc-vao` | nhuong |
| M2 | chụp/chọn ảnh bill | `cam-may` | binh-than |
| M3 | ghi sổ xong | `dong-dau` | binh-than |
| M4 | tiền đã về (sau khi đọc lại) | `cui-cam-on` | nhuong |
| M5 | tạo kèo xong | `buoc-di` | hao-hung |
| M6 | sổ hai người mở | `keo-tab` rồi `nhay` | hao-hung |
| M7 | gửi tờ giấy | `gap-thu` | nhuong |
| M8 | mở huy hiệu mới | `nang-tem` | hao-hung |

Ranh giới («Luật Nếp Không Chạm Số»):

- Nếp diễn trong **vùng riêng chiếm chỗ trong bố cục**, không bao giờ là lớp nổi đè nội dung.
- Cách mọi con số tiền đang hiển thị ≥ 16dp và không chung hàng với nó; mắt và tay chỉ hướng vào đạo cụ của mình.
- Ở M2–M4 mặt chỉ `binh-than` hoặc `nhuong`; không biểu cảm về nợ, không câu «đòi».
- Không bao giờ ở trạng thái lỗi, từ chối, xung đột, hay tiền đang chờ/tranh cãi.
- «Một Nếp một lúc»: khi Nếp trong trang đang hiện, dock nhường chỗ (`useNhuongChoNep(true)`), không vẽ gì.
- ADR-0035 §2.4 giữ nguyên: trên màn tiền, dock chỉ là mép giấy trơn. `phieu.ts` giữ nguyên: tiền không bao giờ đi
  vào phiếu ngữ cảnh.
- Bản vẽ `trang` giữ khoá sha256; con rối được **dẫn xuất** từ nó, không vẽ lại.
- Nếp vẫn là lớp tháo được: mọi cảnh phải đọc được khi không có Nếp.

### D6. Mực người («Luật Mực Người»)

Tám màu mực mỗi scheme ở mục mới `tokens.json` → `mucNguoi`; mỗi màu ≥ 4,5:1 trên `paper`, `card` và `ground` của
scheme mình; hue (OKLCH) cách ≥ 30° so với `accent`, `split`, `ai` và `brand.coral/teal/violet`; khác nhau từng
cặp. Chỉ số là FNV-1a của person id, ổn định mọi nơi. Dùng cho vòng và chữ đầu của avatar, dấu của người (ghế,
chấm phiếu, tên trong chat); không bao giờ làm nền dưới chữ và không mang nghĩa. Ảnh đại diện lấy từ
`nguonAnhDaiDien` khi có (404 được nhớ).

### D7. Cơ chế sách pop-up là tương tác

Nắp lật, tab kéo, đĩa xoay, xé cuống, kéo thả vào ghế, lật trang. Mọi cử chỉ có **đường chạm tương đương** và
**đường trợ năng tương đương** (`accessibilityActions`, role `adjustable`). Control ≥ 48dp và nhìn thấy trước mọi
cử chỉ. «Luật Control Đứng Yên»: control không xoay 3D và không dịch quá 4dp khi animation đang chạy; chỉ tầng vẽ
và vật không mang chữ mới bật dựng.

### D8. «Luật Nói Rõ Trước Khi Bấm»

Phải còn nhìn thấy, cùng khung với control: câu đồng thuận lúc đồng ý (Lập sổ: hai cột Cho phép / Không kéo theo,
«im lặng không phải đồng ý»); hậu quả của việc không hoàn tác được (Phát đợt thu, `XacNhanViec`, xoá tài khoản);
câu «số này là gì / không phải gì» dưới mỗi sổ; câu riêng tư ngay chỗ thu thập dữ liệu; «Chưa mã hoá đầu cuối»;
mọi lỗi. Được gấp vào nắp (`NapGiay`) hoặc thành chú thích lề (`ChuThichLe`, vẫn là chữ nhìn thấy): cách dùng,
luật vai, mẹo, mô tả dài của các mức người đọc (một dòng tóm tắt vẫn hiện), trợ giúp định dạng.

### D9. Trợ năng canvas

Mọi chữ là RN `Text`. Mỗi canvas hoặc ẩn khỏi trình đọc màn hình, hoặc mang đúng một câu (như `Canh`, `KyHoa`).

### D10. Skia

Thêm `@shopify/react-native-skia` 2.6.2 (bản Expo SDK 57 khuyến nghị). Skia vẽ sân khấu, con rối và chất liệu;
`react-native-svg` giữ cho hàng danh sách, thumbnail, icon và là **renderer dự phòng**. Hình học vẫn là module
thuần `src/rudi/art/*.ts` dùng chung cho cả hai. Mọi import Skia nằm trong `src/rudi/ui/skia/**`. Trên web,
CanvasKit tải lười, SVG vẽ trước; không có WebGL thì không tải. `canvaskit.wasm` không vào Git (chép lúc build).
Dev client phải dựng lại một lần; dev client cũ rơi về SVG thay vì sập.

### D11. Không hạt bay

Confetti, hạt, lấp lánh, toast vẫn cấm. Một vật đơn lẻ bay một lần thì được (tờ lịch, cuống phiếu, lá thư).

### D12. Bằng chứng trên live

Review hình ảnh chạy trên thế giới seed thật (`npm run seed:rudi`), không chỉ fixture. Thêm cổng hiện diện: màn tạo
mới, màn tiền, màn sổ đôi phải render ít nhất một primitive sân khấu hoặc vật thể.

### D13. Kiến trúc token

Vai màu mới mà trang khách không vẽ nằm ở **mục riêng** của `tokens.json` (`sanKhau`, `mucNguoi`), không soi gương
vào `guest.css` (tiền lệ: `chatTheme`) và được đo bằng test node. `color.*` và danh sách 25 khoá `ORDER` của
`scripts/sinh_token_ui_v2.py` giữ nguyên; `guest.css` (tầng Python legacy) không đổi một byte.

### D14. Bàn đêm và dấu teal

Theme tối: nền vải, tầng vẽ `paper`, ánh đèn bàn, bóng sâu hơn. **Nền `paper` tối không phải mặt chữ** (đo trên
`#2e335c`: `accent` 4,12:1, `warn` 4,00, `inkFaint` 4,37, `lineStrong` 3,23): vật mang chữ dùng nền `card`; nơi bắt
buộc `paper` mang chữ (tờ `ToGiay`) chỉ dùng `ink`, `inkSoft`, `split`, `ai` và dấu `variant="ink"`. `StampButton`
thêm tông teal cho quyết định tiền (mực tĩnh trên `brand.teal` 5,33:1). Không có dấu tím (3,34:1; AI không bao giờ
«quyết»).

## 3. Hệ quả

- `packages/shared/tokens.json` thêm `sanKhau`, `mucNguoi`, `motion.sanKhau` (chỉ thêm).
- `src/rudi/theme.ts`, `src/rudi/motion.ts`, `src/rudi/ui/useMotion.ts` đọc các mục mới; module thuần
  `src/rudi/nguoi/muc-nguoi.ts` và `src/rudi/san-khau/ngan-sach.ts` giữ phần test được.
- Test mới: `token-san-khau`, `muc-nguoi`, `chu-tren-giay` (bắt đầu với danh sách nợ, co dần qua từng lát).
- DESIGN.md được documenter viết lại ở cuối chiến dịch (lát S7); trong lúc chuyển có ghi chú «đang chuyển».
- Flow Maestro sửa cùng lát khi câu chữ đổi; tên `takeScreenshot` giữ nguyên.
- Lead dựng lại dev client có Skia trước khi chạy bảng native.

## 4. Cái này KHÔNG chứng minh

- Sân khấu đẹp hơn không chứng minh sản phẩm đúng hướng (ADR-0006 vẫn đúng).
- Ảnh web (react-native-web + CanvasKit) không chứng minh Android/iOS; bảng native là cổng riêng của Lead.
- Test hình học chứng minh hình hợp lệ và tất định, không chứng minh hình đẹp; mắt người và đọc mù mới chấm được.

## 5. Phương án đã bác

| Phương án | Vì sao không |
|---|---|
| «Sổ Nếp sống» (đẩy sổ hiện tại lên, không pop-up) | Lead chọn hướng táo bạo hơn |
| «Zine hội bạn» | Lệch khỏi thế giới sổ indigo đã chốt; phải duyệt lại thương hiệu |
| Lottie / ảnh raster vẽ sẵn | Màu ngoài token (hỏng theme tối), phải ghim từng file, khó test |
| Giữ luật «Nếp đứng xa tiền» | Lead chốt Nếp diễn khoảnh khắc, kể cả khoảnh khắc tiền, với ranh giới D5 |
| Giao từng lát | Lead chọn làm hết rồi giao một lần; commit từng lát chỉ để dự phòng |
