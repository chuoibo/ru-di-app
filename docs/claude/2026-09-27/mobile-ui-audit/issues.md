# Issues: audit UI/UX app mobile RuDi

- Cây đo: `7ea1a7c`. Bản chạy: web export production (`expo export --platform web`) trên stack cục bộ
  (Postgres 16, API Python, cửa trước Go), dữ liệu seed tổng hợp. Chromium 141 headless, giả lập di
  động (`isMobile`, `hasTouch`, DPR 2), chữ thân Roboto (xem `report.md` §A).
- MODE = AUDIT_ONLY: không issue nào được sửa trong đợt này. «Trạng thái sửa» của mọi issue là
  *chưa sửa*; «Retest» là *không áp dụng*.
- Phân loại: BUG (sai, tái hiện được) · UX ISSUE (khó hiểu/khó thao tác, chưa chắc là lỗi kỹ thuật) ·
  VISUAL POLISH (đề xuất thẩm mỹ, không phải lỗi khách quan).
- Mức: P0 mất dữ liệu / app không dùng được diện rộng · P1 chặn flow chính, action quan trọng không
  làm được, overlay không thoát được, điều hướng hỏng · P2 lỗi layout/interaction rõ, che/mất nội dung,
  vẫn còn đường đi tiếp · P3 polish, nhất quán, UX nhẹ.
- Phương pháp: RUNTIME-WEB là đã chạy và đo trên bản web; STATIC là đọc mã; HYPOTHESIS là nghi vấn
  chưa kiểm chứng. Mọi issue dưới đây đã được tái hiện runtime trên web, trừ chỗ ghi khác.
- Native Android/iOS: không chạy được trong container (xem `report.md` §A); cột «Native» nói điều đọc
  mã cho phép suy ra, không phải bằng chứng.

## Tóm tắt theo mức

| Mức | Issue |
|---|---|
| P1 | UI-005 |
| P2 | UI-002, UI-003, UI-004, UI-006, UI-011, UI-016, UI-018, UI-019, UI-021, UI-022, UI-023, UI-024 |
| P3 | UI-001, UI-007, UI-008, UI-009, UI-010, UI-012, UI-013, UI-014, UI-015, UI-017, UI-020, UI-025, UI-026, UI-027, UI-028, UI-029, UI-030, UI-031 |

---

## F00 Vỏ toàn cục

### UI-005 · Đóng khay «Tạo mới» bằng Back trình duyệt để lại `aria-hidden` trên cả màn Khám phá

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (accessibility) · **P1** cho người dùng trình đọc màn hình trên web |
| Feature / Screen / Layer | F00 · `/explore` + `/create` · L01 khay tạo (`src/rudi/ui/Sheet.tsx`) |
| Nền tảng, cấu hình | web, C1 (390×844). Native: không bị (nhánh web của `Sheet`), suy từ mã |
| Điều kiện ban đầu | Đăng nhập Minh Anh (Team Đà Lạt), đang ở Khám phá |
| Tái hiện | 1. Chạm «Tạo mới». 2. Khi khay mở, bấm Back của trình duyệt (trên Android Chrome: cử chỉ back hệ thống). 3. Đọc cây truy cập |
| Expected | Khay đóng, trang trở lại như trước khi mở: không phần tử nào còn `inert` hay `aria-hidden` |
| Actual | Khay đóng và chạm vẫn hoạt động. Nhưng `div[data-testid="explore-screen"]` 390×844, **chứa cả thanh tab**, giữ `aria-hidden="true"`. Trình đọc màn hình không còn thấy nội dung Khám phá lẫn thanh tab cho tới khi tải lại trang. Đóng bằng X, nền, Esc hay kéo thì sạch (0 phần tử sót) |
| Evidence | Số đo DOM (runtime, lặp lại 3 lần): phần tử `div` tổ tiên của `explore-screen`, 390×844, `inert: false`, `aria-hidden: "true"`, chứa cả `role="tab"`. Ảnh đánh dấu phần tử ấy ngay sau khi đóng bằng Back: ![aria-hidden sót](evidence/EV-F00-KT-back-aria-C1.jpg) |
| Source | `src/rudi/ui/Sheet.tsx:75-121` (lưu `aria-hidden` cũ của các nhánh anh em, khôi phục khi cleanup) |
| Root cause (giả thuyết mạnh) | Khi mở `/create`, navigator đã đặt `aria-hidden="true"` cho màn bên dưới. `Sheet` ghi nhận giá trị ấy làm «giá trị cũ». Đóng bằng X: `Sheet` khôi phục trước, rồi route pop và navigator gỡ `aria-hidden`, nên sạch. Đóng bằng Back: route pop trước và navigator gỡ `aria-hidden`, rồi cleanup của `Sheet` chạy sau và **khôi phục lại** `"true"` đã lưu |
| Hậu quả | Với người dùng trình đọc màn hình trên web, sau một thao tác back phổ biến, toàn bộ màn và thanh tab câm |
| Đề xuất sửa | Khi cleanup, chỉ khôi phục thuộc tính nếu giá trị hiện tại vẫn là giá trị `Sheet` đã tự đặt; hoặc bỏ việc tự đặt `aria-hidden` cho nhánh đã được navigator ẩn |
| Tiêu chí gỡ | Mở khay rồi Back: 0 phần tử `inert`/`aria-hidden` phủ ≥25% màn; thêm test web cho đường Back |

### UI-002 · Người đã đăng nhập mở link lạ thì bị đưa về bìa Welcome, rồi bị đòi đăng nhập lại

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (điều hướng) · **P2** |
| Feature / Screen | F00 · `[...legacy]` → `/welcome` → `/login` |
| Nền tảng, cấu hình | web, C1. Native: route catch-all giống hệt, suy ra cùng hành vi (STATIC) |
| Điều kiện | Đăng nhập Minh Anh (cookie phiên còn hạn) |
| Tái hiện | 1. Mở một đường dẫn không tồn tại (ví dụ `/khong-co-trang-nay`). 2. Bấm «Rủ Đi thôi!» |
| Expected | Người đã có phiên được đưa về tab của mình (hoặc thấy trang «không tìm thấy» có lối về), không bị coi như khách |
| Actual | Bước 1 hiện bìa Welcome như khi chưa đăng nhập. Bước 2 dẫn tới màn Đăng nhập đòi nhập số điện thoại, dù phiên vẫn còn |
| Evidence | ![welcome khi đã đăng nhập](evidence/EV-F00-DT-dalat-0-la.jpg) ![login sau CTA](evidence/EV-F00-DT-la-cta-C1.jpg) |
| Source | `app/[...legacy].tsx` (luôn `Redirect` `/welcome`), `src/rudi/screens/Welcome.tsx:109-116` (CTA luôn `push("/login")`) |
| Hậu quả | Một link cũ hay gõ nhầm làm người dùng tưởng đã bị đăng xuất; họ phải OTP lại (tốn lượt OTP) |
| Đề xuất | Catch-all đọc phiên như `/` (`duong-vao.ts`), hoặc Welcome/Login chuyển thẳng về tab khi đã có phiên |
| Tiêu chí gỡ | URL lạ khi có phiên thì dẫn về tab (hoặc trang lỗi có lối về); flow Maestro 00/91 vẫn xanh |

### UI-003 · Trên web, `accessibilityState` không tới DOM: thanh tab không báo tab đang chọn (và các chỗ chỉ dùng `accessibilityState`)

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (accessibility, web) · **P2** |
| Feature / Screen / Layer | F00 · mọi màn tab · L05 `RudiTabBar`; hệ thống |
| Nền tảng, cấu hình | web, C1, C2, C3, C6, C7. Native: `accessibilityState` là API chuẩn của RN nên không bị (STATIC) |
| Tái hiện | Mở bất kỳ tab nào, đọc thuộc tính ARIA của 4 phần tử `role="tab"` |
| Expected | Tab đang chọn có `aria-selected="true"`, và các tab nằm trong một `role="tablist"` |
| Actual | Cả 4 tab đều không có `aria-selected`, và không có `tablist`. Nhìn bằng mắt vẫn phân biệt được tab đang chọn (màu, icon đặc, dải washi), nhưng trình đọc màn hình thì không |
| Evidence | Số đo runtime ở 5 cấu hình: `chon: null` ở mọi tab. Mã `react-native-web` 0.21 (`dist/modules/createDOMProps`) nhận `aria-selected`/`accessibilitySelected` mà **không** đọc object `accessibilityState`. Quét tĩnh: 35 chỗ dùng `accessibilityState`, trong đó 20 chỗ không truyền kèm thuộc tính `aria-*` tương ứng (danh sách ở `report.md` §C). Đối chứng runtime cho thấy chỗ nào có truyền kèm `aria-*` thì đạt: chip gu ở Sở thích (`role=checkbox`, `aria-checked`) và thẻ mức chi (`role=radio`, `aria-checked`). Vì vậy mỗi dòng trong danh sách 20 cần xác nhận runtime; đã xác nhận: thanh tab |
| Hậu quả | Trên web, người dùng trình đọc màn hình không biết tab nào, ngày nào, chip gu nào, màu nào đang được chọn, và mục nào đang mở/gập |
| Đề xuất | Truyền thêm prop `aria-selected`/`aria-checked`/`aria-expanded`/`aria-busy` (RNW đọc được; `HangChang` và `RosterPicker` đã làm vậy), hoặc gom lại trong một helper ở kit; thêm `role="tablist"` cho thanh tab |
| Tiêu chí gỡ | Quét DOM: mỗi control có trạng thái đều mang thuộc tính ARIA tương ứng; tab đang chọn có `aria-selected=true` |

### UI-004 · Rail (≥600dp, web): vạch chỉ báo tab đang chọn nằm lệch khỏi tab

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (layout/trạng thái hiển thị) · **P2** |
| Feature / Screen / Layer | F00 · mọi màn tab ở cửa sổ ≥600dp · L05 `RudiTabBar` (rail) |
| Nền tảng, cấu hình | web, biên 600, C6 768×1024, biên 839, biên 840, C7 1024×1366. Native: có thể không bị (Yoga tôn trọng `height: 72`), chưa đo (BLOCKED) |
| Tái hiện | Mở `/messages` hoặc `/profile` ở cửa sổ rộng ≥600dp |
| Expected | Vạch cam dọc nằm cạnh tab đang chọn |
| Actual | Mỗi ô rail chỉ cao 48dp nhưng vạch dịch 72dp/ô. Ở Tin nhắn: tab y156–204, vạch y216–288 (cạnh «Cá nhân»). Ở Cá nhân: tab y204–252, vạch y288–360 (dưới mọi tab). Chỉ Khám phá trùng một phần |
| Evidence | ![vạch chỉ báo lệch](evidence/EV-F00-RAIL-chibao-cat.jpg) |
| Source | `src/rudi/ui/RudiTabBar.tsx:54-58` (dịch `column * 72`), `:164-165` (`railItem: { flex: 0, height: 72 }`) |
| Root cause | react-native-web biên dịch `flex: 0` thành `flex-basis: 0%`, nên trong cột flex `height: 72` bị bỏ và ô chỉ còn `minHeight: 48` |
| Hậu quả | Tín hiệu «đang ở đâu» nói sai, có lúc chỉ vào tab khác. Màu chữ vẫn đúng nên người dùng còn tự định hướng được |
| Đề xuất | Tính vị trí vạch từ layout đo được (`onLayout`) thay vì hằng 72, hoặc đặt `flexBasis: 72`/`minHeight: 72` cho ô rail |
| Tiêu chí gỡ | Ở mọi biên 600/768/839/840/1024 và cả 4 tab, tâm vạch nằm trong khoảng dọc của tab đang chọn |

### UI-006 · Chạm «+» hai lần nhanh thì chạm thứ hai kích hoạt một việc trong khay đang mở

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (interaction) · **P2** |
| Feature / Screen / Layer | F00 · tab bar → `/create` · L01 |
| Nền tảng, cấu hình | web, C1 (chạm cảm ứng giả lập). Native: bố cục giống nên có thể xảy ra, chưa đo |
| Tái hiện | Ở Khám phá, chạm «Tạo mới» hai lần cách nhau 60 ms |
| Expected | Mở đúng một khay; không có hành động nào khác |
| Actual | Chạm thứ hai rơi vào thẻ của khay đang trượt lên, nên app đi thẳng tới `/hai-nguoi/chon-nguoi` («Rủ một người đi chơi»). Với người có DM, thẻ này được xếp lên đầu |
| Evidence | Dòng thời gian URL từ 0 ms: `/hai-nguoi/chon-nguoi`, 0 dialog · ![sau chạm đúp](evidence/EV-F00-KT-cham-dup-C1.jpg) |
| Source | `src/rudi/screens/Create.tsx` (thẻ nhận chạm ngay khi mount), `ui/Sheet.tsx` |
| Hậu quả | Một cú chạm vội (hoặc chạm lại khi tưởng chưa ăn) mở nhầm một luồng |
| Đề xuất | Bỏ qua chạm vào nội dung khay trong pha mở (khoảng 250 ms), hoặc chỉ bật `pointerEvents` khi lò xo đã gần tới đích |
| Tiêu chí gỡ | Chạm đúp «+» cách 60 ms: đúng 1 dialog, URL vẫn `/create` |

### UI-011 · Nút «Vẽ» trong bảng Nếp không làm gì trên web

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (web) · **P2** |
| Feature / Screen / Layer | F00 · bảng Nếp (`src/rudi/nep/NepBang.tsx`) · L03 |
| Nền tảng, cấu hình | web, C1. Native: `Alert.alert` hiện hộp thoại hệ thống (STATIC) |
| Tái hiện | Mở bảng Nếp (chạm mép phải, chạm Nếp), gõ «một con mèo đội nón lá», bấm «Vẽ» |
| Expected | Hỏi xác nhận «Nhờ Nếp vẽ?» rồi vẽ, hoặc nói vì sao chưa vẽ được (ví dụ chưa cấu hình AI) |
| Actual | Không có gì: 0 dialog mới, 0 alert trình duyệt, 0 request, không dòng trạng thái |
| Evidence | ![bấm Vẽ không phản hồi](evidence/EV-F00-NEP-VE-C1.jpg) |
| Source | `src/rudi/nep/NepBang.tsx` (`Alert.alert("Nhờ Nếp vẽ?", …)`); `react-native-web/dist/exports/Alert/index.js`: `static alert() {}` |
| Hậu quả | Tính năng vẽ chết hẳn trên web, không một lời báo. Ngoài ra DESIGN.md cấm hộp thoại hệ thống |
| Đề xuất | Xác nhận bằng một dòng hoặc sheet trong bảng (đúng tinh thần DESIGN), không dùng `Alert` |
| Tiêu chí gỡ | Web: bấm «Vẽ» luôn cho một phản hồi nhìn thấy được |

### UI-007 · Khay tạo cao vượt trần 82% ở cửa sổ nhỏ và thấp

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (lệch spec) · **P3** |
| Feature / Screen / Layer | F00 · `/create` · L01; `ui/Sheet.tsx` (dùng chung) |
| Nền tảng, cấu hình | web, C2 320×640: 589/640 = 92%; C8 390×460: 441/460 = 96% (chỉ còn 19px nền để chạm ra ngoài). C1 75%, C3 80%: đạt |
| Expected | DESIGN.md: sheet cao tối đa 82% màn |
| Actual | `ScrollView` bên trong được giới hạn 82%, nhưng panel còn cộng hàng tay cầm 48dp + margin + padding đáy, nên tổng vượt trần |
| Evidence | ![C8](evidence/EV-F00-KT-mo-C8.jpg) ![C2](evidence/EV-F00-KT-mo-C2.jpg) |
| Source | `src/rudi/ui/Sheet.tsx:53` (`tran = 0.82 × cao cửa sổ` cho ScrollView), `:207-217` |
| Hậu quả | Ở cửa sổ thấp (split-screen, hoặc khi bàn phím thu nhỏ cửa sổ) gần như không còn nền để chạm ra ngoài; mất ngữ cảnh màn dưới |
| Đề xuất | Trừ phần đầu panel khỏi `tran` (giới hạn cả panel, không chỉ vùng cuộn) |
| Tiêu chí gỡ | Chiều cao panel ≤82% ở C2 và C8 |

### UI-013 · Sheet dùng chung: cuối pha đóng, panel còn lộ rồi biến mất đột ngột

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (motion) · **P3** |
| Feature / Screen / Layer | F00 · mọi sheet cao hơn 480dp · `ui/Sheet.tsx` |
| Nền tảng, cấu hình | web, C1 (khay tạo cao 637). C9 (giảm chuyển động): không bị, cắt thẳng đúng |
| Tái hiện | Mở khay tạo, bấm X, quay khung hình |
| Expected | Panel trượt khỏi mép dưới rồi mới gỡ |
| Actual | Panel dịch cố định 480dp: khung cuối trước khi gỡ, panel ở y=667, còn lộ 177px, độ mờ 1. Khung sau thì biến mất (bật mất). Khi mở, khung đầu cũng đã lộ 157px đỉnh panel |
| Evidence | ![khung đóng](evidence/EV-F00-MO04-dong-khay-C1.jpg) · lấy mẫu mỗi rAF: `y 565 → 632 → 667 → (mất)` |
| Source | `src/rudi/ui/Sheet.tsx:167-170` (`translateY: (1 - progress) * 480 + keo`) |
| Hậu quả | Cú bật mất nhìn thấy được mỗi lần đóng sheet cao; trông như giật |
| Đề xuất | Dịch theo chiều cao đo được của panel (hoặc theo chiều cao cửa sổ), hoặc mờ panel ở phần cuối pha đóng |
| Tiêu chí gỡ | Khung cuối trước khi gỡ: đỉnh panel ≥ chiều cao cửa sổ, hoặc độ mờ ≈0 |

### UI-008 · Trong khay tạo, điểm dừng phím Tab đầu tiên là khối Nếp không tên

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (accessibility, web) · **P3** |
| Feature / Screen / Layer | F00 · `/create` · L01 + L04 (`ui/NepDien.tsx`) |
| Nền tảng, cấu hình | web, C1 (bàn phím) |
| Tái hiện | Mở khay, nhấn Tab |
| Expected | Mọi điểm dừng focus có tên và vai trò |
| Actual | Điểm dừng đầu: `div` 112×112, `tabindex=0`, không role, không tên (Nếp M1). Sau đó mới đến 5 thẻ và «Đóng bảng». Focus ban đầu khi mở vào «Đóng bảng», và vòng Tab giữ trong khay: đạt |
| Evidence | danh sách phần tử nhận focus (runtime) trong `report.md` §C; ![khay](evidence/EV-F00-KT-mo-C1.jpg) |
| Source | `src/rudi/ui/NepDien.tsx:52` (`Pressable accessible={false}` vẫn nhận `tabindex=0` trên web) |
| Đề xuất | `focusable={false}` (hoặc `tabIndex={-1}`) trên web khi không có việc để bấm |
| Tiêu chí gỡ | Không điểm dừng Tab nào không có tên |

### UI-009 · Khôi phục phiên chậm thì vùng nội dung trống, không có chỉ báo tải

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE · **P3** |
| Feature / Screen / Layer | F00 · mọi route bọc `phienDaDoc` (tab, chat, tiền) · L35 |
| Nền tảng, cấu hình | web, C1, `/sessions/web/resume` bị làm trễ 5 s bằng chặn mạng (giả lập mạng chậm) |
| Tái hiện | Mở lạnh `/plan` khi có cookie phiên, mạng chậm |
| Expected | Có khung xương hoặc chỉ báo đang tải |
| Actual | Suốt thời gian chờ: nền trống, chỉ có thanh tab và mép Nếp; 0 progressbar. Các tab khác cũng trống |
| Evidence | ![1,8 s](evidence/EV-F00-RESUME-CHAM-1800ms-C1.jpg) |
| Source | `app/(tabs)/plan.tsx` và các route khác: `if (!phienDaDoc) return null;` |
| Đề xuất | Hiện skeleton của màn (kit `Skeleton`) thay cho `null` |
| Tiêu chí gỡ | Trễ resume 5 s: có progressbar hoặc skeleton trong ≤300 ms |

### UI-010 · Link tới `/create` mở lạnh trên web không mở khay

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (web) · **P3** |
| Feature / Screen / Layer | F00 · `app/create.tsx` · L01 |
| Nền tảng, cấu hình | web, C1, có và không có phiên. Native: chưa đo (BLOCKED) |
| Tái hiện | Mở thẳng URL `/create` |
| Expected | Theo chú thích trong mã: đặt Khám phá bên dưới rồi mở lại khay trên đó |
| Actual | Chỉ `replaceState /explore` được ghi vào history; không có push `/create`; 0 dialog trong 3 s |
| Evidence | ![mở lạnh /create](evidence/EV-F00-CREATE-LANH-C1.jpg) |
| Source | `app/create.tsx:19-24` |
| Root cause (giả thuyết) | `router.replace` làm route unmount ngay trên web; cleanup `clearTimeout(t)` huỷ cú push hẹn 0 ms |
| Đề xuất | Push trong callback sau khi replace xong (ví dụ trên màn đích), hoặc không huỷ timer khi unmount do chính replace |
| Tiêu chí gỡ | Mở lạnh `/create`: có 1 dialog trên Khám phá |

### UI-012 · Chip gợi ý trong bảng Nếp chỉ cao 36dp

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (vùng bấm) · **P3** |
| Feature / Screen / Layer | F00 · bảng Nếp · L03 |
| Nền tảng, cấu hình | web, C1 |
| Expected | DESIGN.md: chip 48dp, mọi đích bấm ≥48×48dp |
| Actual | 3 chip («Quanh đây có gì hay?», …) cao 36dp, rộng 150–184dp |
| Evidence | ![bảng Nếp](evidence/EV-F00-NEP-BANG-C1.jpg) |
| Source | `src/rudi/nep/NepBang.tsx` (style `goiY`) |
| Đề xuất | `minHeight: 48` cho chip gợi ý (dùng `Chip` của kit) |
| Tiêu chí gỡ | Chip ≥48dp |

### UI-014 · Ở màn hẹp, mép Nếp đè lên chữ của hàng chip tràn mép

| Trường | Nội dung |
|---|---|
| Category / Severity | VISUAL POLISH (lệch luật ADR-0035 «không che chữ») · **P3** |
| Feature / Screen / Layer | F00 · Khám phá · L02 |
| Nền tảng, cấu hình | web, C2 320×640 (ở C1 và C3 mép rơi vào vùng thẻ, không đè chữ) |
| Actual | Mép Nếp x310–320 nằm trên chữ chip «Vui chơi». Hàng chip cuộn ngang nên chữ vốn đã bị mép màn cắt; ảnh hưởng thực tế nhỏ |
| Evidence | ![mép đè chip](evidence/EV-F00-NEP-CHE-CHIP-C2.jpg) |
| Source | `src/rudi/nep/dock-vi-tri.ts` (`yTuTyLe`: vị trí dọc theo tỷ lệ chiều cao) |
| Đề xuất | Tránh dải y của hàng cuộn ngang tràn mép, hoặc để hàng chip chừa lề phải 16dp |
| Tiêu chí gỡ | `nep.chuBiChe` rỗng ở C2 trên Khám phá |

## F01 Vào cửa

### UI-001 · Ô nhập một dòng `ONhapMuc` cao 44dp, dưới ngưỡng 48dp của DESIGN.md

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (vùng bấm, lệch spec hệ thống) · **P3** |
| Feature / Screen / Layer | F01 và mọi màn dùng `ONhapMuc` một dòng · Login (ô số điện thoại), Lời mời (ô mã) |
| Nền tảng, cấu hình | web, C1, C2, C3 (đo runtime). Native: cùng `minHeight: 44` (STATIC) |
| Expected | DESIGN.md §Mục tiêu chạm: «Mọi node bấm được ≥48×48dp, kể cả `TextInput`» |
| Actual | Ô số điện thoại 358×44; ô mã lời mời 196×44 |
| Evidence | ![ô nhập 44](evidence/EV-F01-O-NHAP-44-C1.jpg) |
| Source | `src/rudi/ui/ONhapMuc.tsx:60` (`minHeight: 44`) |
| Đề xuất | `minHeight: 48` (vẫn không hộp, dòng kẻ giữ nguyên) |
| Tiêu chí gỡ | Mọi `input` một dòng ≥48dp cao |

### UI-016 · Welcome trên web: chấm trang và mốc trên đường đứng yên ở trang 1 khi vuốt

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (web) · **P2** |
| Feature / Screen / Layer | F01 · `/welcome` · L30 pager (`src/rudi/screens/Welcome.tsx`) |
| Nền tảng, cấu hình | web, C1. Native: `onMomentumScrollEnd` có phát, nên suy ra không bị (STATIC) |
| Tái hiện | Mở `/welcome`, vuốt trái trên đoạn chữ |
| Expected | Chấm trang, mốc sáng trên đường vẽ, và nhãn truy cập «Trang x trên 4» theo trang đang hiện |
| Actual | Nội dung sang trang 3 («Chia bill từng đồng») nhưng chấm và mốc vẫn ở trang 1; `aria-label` luôn «Trang 1 trên 4». Hệ quả: «Tìm hiểu thêm» ở trang cuối đưa sang trang 2 thay vì trang 1 |
| Evidence | ![trang 3, chấm ở trang 1](evidence/EV-F01-WEL-trang2-C1.jpg) · `scrollLeft` 0 → 780 → 1170, nhãn không đổi |
| Source | `Welcome.tsx:186-192` (`onMomentumScrollEnd={onScroll}` là nơi duy nhất cập nhật `page`); `react-native-web/dist/exports/ScrollView/ScrollViewBase.js` không phát sự kiện momentum nào |
| Hậu quả | Màn đầu tiên của bản web nói sai vị trí; người dùng trình đọc màn hình nghe «Trang 1» ở mọi trang |
| Đề xuất | Cập nhật `page` trong `onScroll` (có `scrollEventThrottle`) theo `Math.round(x / pageWidth)` |
| Tiêu chí gỡ | Sau mỗi lần vuốt, chấm, mốc và nhãn khớp `scrollLeft / pageWidth` |

### UI-019 · Mã lời mời sai được báo thành «Cập nhật app rồi thử lại»

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (thông điệp lỗi sai chuyện) · **P2** |
| Feature / Screen | F01 · `/moi` (`screens/LoiMoi.tsx`), `src/api.ts` `thongDiepNguoiDoc` |
| Nền tảng, cấu hình | web, C1. Native: cùng hàm dịch lỗi (STATIC) |
| Tái hiện | Mở `/moi`, dán `khong-phai-ma-that`, bấm «Nhận lời mời» |
| Expected | Câu nói đúng chuyện: mã không đúng hoặc đã hết hạn, hỏi lại người mời |
| Actual | «Phần này chưa mở được trên bản app này. Cập nhật app rồi thử lại.» Máy chủ trả `404 {"code":"invite_not_found","detail":"Invite link is not valid"}` |
| Evidence | ![mã sai](evidence/EV-F01-MOI-sai-C1.jpg) |
| Source | `src/api.ts:300-313`: `detail` chỉ được dùng khi là tiếng Việt; 404 luôn được coi là «app và máy chủ lệch phiên bản»; `code` bị bỏ qua |
| Hậu quả | Người gõ nhầm hoặc cầm link hết hạn đi cập nhật app vô ích. Có thể lặp ở mọi 404 «không tìm thấy» thật (HYPOTHESIS, sẽ ghi khi gặp) |
| Đề xuất | Dịch theo `code` (`invite_not_found` → «Mã lời mời không đúng hoặc đã hết hạn…»); chỉ dùng câu «cập nhật app» cho 404 không kèm code |
| Tiêu chí gỡ | Mã sai hiện câu về mã, không nhắc cập nhật app |

### UI-017 · Welcome trên web: vuốt nhanh nhảy qua một trang

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (web) · **P3** |
| Feature / Screen / Layer | F01 · `/welcome` · L30 |
| Nền tảng, cấu hình | web, C1 (cảm ứng giả lập, timestamp đúng) |
| Actual | Vuốt 250dp trong 900 ms hoặc 200dp trong 500 ms: sang 1 trang. Vuốt 280dp trong 260 ms (khoảng 1077 px/s): nhảy 2 trang, bỏ qua trang 2 |
| Source | `pagingEnabled` trên web thành `scroll-snap-type: x mandatory` với `scroll-snap-stop: normal` |
| Đề xuất | `scroll-snap-stop: always` cho từng trang trên web (hoặc tự dừng ở trang kế trong `onScroll`) |
| Tiêu chí gỡ | Vuốt nhanh cũng chỉ sang một trang |

### UI-018 · Nút «Quay lại» không làm gì khi màn được mở thẳng bằng link

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (điều hướng) · **P2**. Checkpoint 2 ghi P3; nâng ở checkpoint 3 vì phạm vi không còn là một màn: đo lại ở `/places/[id]`, và nút back của `TopBar` trong kit cũng gọi `router.back()` không kiểm `canGoBack()` |
| Feature / Screen | F01 · `/login` (`ui/CoverBand.tsx`: `onBack === true ? router.back()`). F02 · `/places/[id]` (`TopBar`, `src/rudi/ui.tsx:235`). 37 file màn dùng `TopBar` với `back` mặc định; các màn còn lại đo ở feature của chúng |
| Nền tảng, cấu hình | web, C1 |
| Tái hiện | Mở thẳng `/login` (không có lịch sử), chạm «Quay lại» |
| Expected | Đưa về màn hợp lý (Welcome), hoặc không vẽ nút khi không có nơi để về |
| Actual | Đứng yên ở `/login`, không phản hồi |
| Evidence | ![sau khi chạm Quay lại](evidence/EV-F01-LOGIN-back-lanh-C1.jpg) · F02: mở thẳng `/places/p-tiem-nuong-xom-lao`, chạm «Quay lại», URL vẫn là `/places/p-tiem-nuong-xom-lao` sau 900 ms (`TC-F02-CHI-TIET-BACK-LANH`). Màn chi tiết không có thanh tab, nên trong app không còn lối ra |
| Source | `src/rudi/ui/CoverBand.tsx:45`; `src/rudi/ui.tsx:235` (`TopBar`); chỉ `app/create.tsx` kiểm `canGoBack()` |
| Đề xuất | `router.canGoBack() ? router.back() : router.replace(<màn cha>)` trong nút back của kit |
| Tiêu chí gỡ | Mở lạnh rồi chạm «Quay lại» luôn đi tới một màn |

### UI-020 · Welcome: cụm chấm trang không đọc được, pager không nhận focus bàn phím

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (accessibility, web) · **P3** |
| Feature / Screen / Layer | F01 · `/welcome` · L30 |
| Nền tảng, cấu hình | web, C1 và C3 (axe WCAG 2 A/AA) |
| Actual | axe: `aria-prohibited-attr` (cụm chấm mang `aria-label` trên `div` không có role, nên không được đọc); `scrollable-region-focusable` (vùng pager cuộn ngang không nhận focus: người dùng bàn phím chỉ xem trang 2–4 được qua «Tìm hiểu thêm») |
| Source | `Welcome.tsx:201` (`View` chấm có `accessibilityLabel`, không role); `ScrollView` pager |
| Đề xuất | Đặt role cho cụm chấm (`progressbar` hoặc `text` có `aria-live`), hoặc gắn nhãn vào pager; `tabIndex=0` cho vùng cuộn trên web |
| Tiêu chí gỡ | axe 0 vi phạm trên `/welcome` |

## F02 Khám phá

### UI-021 · Hàng địa điểm: dòng giá bị cắt, mất giá trên và «mỗi người»

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (mất nội dung) · **P2** |
| Feature / Screen / Layer | F02 · `/explore` · hàng `HangDiaDiem` và cặp so sánh đầu danh sách |
| Nền tảng, cấu hình | web, C1–C6 (đo DOM); C7 đạt. Native: cùng component, `numberOfLines={1}` cũng cắt (STATIC) |
| Điều kiện ban đầu | Minh Anh, Đà Lạt, 10 nơi đều có giờ mở cửa |
| Tái hiện | Mở Khám phá, đọc dòng «điểm · km · giá» của mỗi hàng |
| Expected | Khoảng giá đọc trọn, ví dụ «200.000đ – 250.000đ mỗi người». Chính mã ghi dải giá có dòng riêng để không bị cắt (QA 23/09) |
| Actual | Dòng bị ellipsis, người xem chỉ thấy «4.8 (64) · 3.9 km · 250.000đ …». Số dòng chứa giá bị cắt: C1 9/10 (thiếu 35–127px) · C2 10/10 (68–162px) · C3 10/10 (28–142px) · C4 10/10 (13–134px) · C5 8/10 (22–107px) · C6 8/10 (48–119px) · C7 0/10 |
| Evidence | ![dòng giá bị cắt C1–C5](evidence/EV-F02-CAT-META-ghep.jpg) Số đo: `scrollWidth − clientWidth` của phần tử một dòng, `kich-ban/f02-kham-pha.mjs --chi cat-chu`, hàng `TC-F02-META` |
| Source | `src/rudi/kham-pha/dia-diem.ts:541-565` (`chiTietNgan` trả [điểm, km, giá, giờ mở]); `src/rudi/screens/explore/HangDiaDiem.tsx:206-207`, `:236-242`, `:313-321` (coi phần tử **cuối** là dải giá, cho nó dòng riêng; phần còn lại ghép thành một dòng `numberOfLines={1}`) |
| Root cause | Quán có giờ mở thì giá không còn là phần tử cuối. Giá rơi vào dòng ghép một dòng; dòng riêng lại thuộc về giờ mở cửa. Bản sửa QA 23/09 mất tác dụng với mọi quán có giờ mở |
| Hậu quả | Giá, thông tin chọn quán quan trọng nhất, bị ẩn ở mọi điện thoại |
| Đề xuất sửa | Tìm dải giá theo loại (icon `wallet-outline`), không theo vị trí; hoặc cho giá một dòng riêng, giờ mở một dòng khác |
| Tiêu chí gỡ | 0/10 dòng chứa giá bị cắt ở C1–C6, đo lại bằng `cat-chu` |

### UI-022 · «Chỉ đường» trên web không làm gì và không báo gì

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG · **P2** |
| Feature / Screen / Layer | F02 · `/places/[id]` · L33 link ngoài |
| Nền tảng, cấu hình | web, C1 (Chromium 141, Linux) |
| Tái hiện | Mở chi tiết Tiệm Nướng Xóm Lào. Chạm «Chỉ đường», hoặc chạm dòng địa chỉ («Mở địa chỉ trên bản đồ») |
| Expected | Mở bản đồ ở tab mới, hoặc một câu nói vì sao không mở được |
| Actual | Một lời gọi `window.open("geo:11.9404,108.4383?q=…", "_blank", "noopener")`, trả `null`. Không có trang mới trong 3 giây, URL không đổi, không có câu nào hiện. Nhánh `catch` («Máy này chưa có ứng dụng bản đồ để chỉ đường.») không bao giờ chạy trên web, vì `Linking.openURL` của react-native-web không ném |
| Evidence | Số đo runtime: bọc `window.open` rồi lắng nghe trang mới, ở hàng `TC-F02-CHI-TIET-CHI-DUONG` và `TC-L33-VONGDOI`. Ảnh sau khi chạm trông y như trước khi chạm, nên không commit |
| Source | `src/rudi/kham-pha/dia-diem.ts:582` (URL `geo:`); `src/rudi/screens/explore/PlaceDetailLive.tsx:154-160`, `:314`, `:321` (cả hai lối cùng gọi `chiDuong`) |
| Hậu quả | Trên web, và trên trình duyệt không có ứng dụng nhận `geo:`, cả hai lối chỉ đường là nút chết |
| iOS | UNVERIFIED HYPOTHESIS: iOS không có scheme `geo:`, nên `Linking.openURL` sẽ từ chối, và máy luôn nói «chưa có ứng dụng bản đồ» dù có Apple Maps. Chưa chạy được vì không có macOS |
| Đề xuất sửa | Chọn URL theo nền tảng (`Platform.select`): web dùng URL https của một dịch vụ bản đồ, iOS dùng `maps:` hoặc `https://maps.apple.com/?ll=`, Android giữ `geo:`. Kiểm `Linking.canOpenURL` trước khi mở |
| Tiêu chí gỡ | Web: chạm «Chỉ đường» mở trang mới hoặc hiện câu. iOS: đo trên máy |

### UI-023 · Nhãn «Lưu địa điểm» ở chân trang chi tiết bị cắt trên điện thoại

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (layout, mất nội dung) · **P2** |
| Feature / Screen | F02 · `/places/[id]` · chân trang hai nút |
| Nền tảng, cấu hình | web. Bị cắt: C1 (thiếu 5px), C2 (34px), C3 (17px), C4 (11px). Đọc trọn: C5, C6, C7 |
| Tái hiện | Mở chi tiết bất kỳ quán nào, nhìn chân trang |
| Expected | Nhãn nút đọc trọn |
| Actual | «Lưu địa đi…» ở 390, «Lưu đ…» ở 320. Ở 320 không còn đoán được nút làm gì nếu không nhìn icon |
| Evidence | ![nhãn Lưu theo bề rộng](evidence/EV-F02-CAT-LUU-ghep.jpg) (hàng `TC-F02-NHAN-LUU`) |
| Source | `PlaceDetailLive.tsx:166-183` và `:483-497`: nút trái `flex: 1`, nút phải `flex: 1.4`, chia theo tỉ lệ chứ không theo nhãn; nhãn `RudiButton` kẹp một dòng (`src/rudi/ui.tsx:509`) |
| Hậu quả | Mất chữ trên một hành động chính của màn, ở mọi điện thoại dưới 430dp |
| Đề xuất sửa | Để nhãn quyết định bề rộng nút trái; hoặc rút nhãn thành «Lưu»; hoặc xếp dọc hai nút dưới khoảng 400dp |
| Tiêu chí gỡ | Nhãn đọc trọn ở C2 |

### UI-024 · Nút ✦ «Hỏi Rủ Đi AI» không hỏi; danh sách báo «0 kết quả» trước khi có câu hỏi

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE · **P2** |
| Feature / Screen | F02 · `/explore` · ô tìm và nút ✦ |
| Nền tảng, cấu hình | web, C1. Máy chủ cục bộ không có khoá AI |
| Tái hiện | 1. Chạm ✦. 2. Muốn hỏi thật: chạm vào ô rồi Enter |
| Expected | ✦ đặt câu mẫu và sẵn sàng gửi (gửi luôn, hoặc focus vào ô). Danh sách không báo thất bại khi chưa hỏi gì. AI không trả lời được thì nói rõ AI chưa sẵn sàng |
| Actual | Bước 1: ô nhận câu «quán nướng cho 6 người, 200k mỗi người», focus ở lại nút ✦. Danh sách lọc theo tên với cả câu, lập tức hiện «0 kết quả», «Chưa thấy nơi phù hợp / Thử từ khóa khác, hoặc bỏ bớt bộ lọc…» và nút «Xóa lọc». Bước 2: `POST /places/search` trả 200, `source: "none"`, 0 nơi. Thẻ «Rủ Đi AI chưa đủ chắc để xếp hạng cho câu này. Thử nói rõ số người, ngân sách hoặc khu vực.» chồng lên cùng trạng thái rỗng, dù câu mẫu đã có số người và ngân sách. Kèm theo: placeholder «…hoặc hỏi Rủ Đi AI» bị cắt thành «…hoặc hỏi Rủ …» ở 390 (đọc trọn ở 430) |
| Evidence | ![sau khi chạm ✦](evidence/EV-F02-AI-MAU-C1.jpg) ![sau khi gửi](evidence/EV-F02-AI-HOI-C1.jpg) |
| Source | `ExploreLive.tsx:261` (`onPress={() => setQuery(CAU_MAU)}`); `:191-196` (`locTheoTen` đòi mọi từ trong ô khớp tên); `:348-356` (một kiểu trạng thái rỗng); `src/rudi/kham-pha/dia-diem.ts:596-597` (câu cho `khong-tra-loi`); `src/screens/kham-pha/tim-kiem.ts` (`source: "none"` nghĩa là mô hình không trả lời, hoặc câu trả lời bị từ chối vì không bám danh mục) |
| Hậu quả | Lối vào AI nổi bật nhất trông như hỏng ngay lần chạm đầu, và lời khuyên tự mâu thuẫn với câu mẫu của chính app |
| Ghi chú môi trường | Bước 2 đo trên máy chủ không có AI; với AI thật có thể ra kết quả. Bước 1 không phụ thuộc AI |
| Đề xuất sửa | ✦ đặt câu rồi gọi `hoi()`, hoặc ít nhất focus vào ô. Khi ô chứa câu hỏi chưa gửi thì chưa lọc theo tên. Với `source: "none"`, nói «Rủ Đi AI chưa trả lời được lúc này», không bảo người dùng sửa câu |
| Tiêu chí gỡ | Chạm ✦ không bao giờ hiện «0 kết quả» trước khi có câu trả lời |

### UI-025 · Lần đầu mở Khám phá, danh sách hiện rồi bị sân khấu đẩy xuống 149dp

| Trường | Nội dung |
|---|---|
| Category / Severity | VISUAL POLISH (layout shift) · **P3** |
| Feature / Screen | F02 · `/explore` · sân khấu thành phố |
| Nền tảng, cấu hình | web, C1 và C9 |
| Tái hiện | Từ Cá nhân chạm tab Khám phá lần đầu trong phiên |
| Expected | Phần đầu giữ chỗ cho sân khấu, dòng «10 nơi ở Đà Lạt» vẽ một lần ở chỗ cuối cùng |
| Actual | Lấy mẫu mỗi khung hình: dòng «10 nơi ở Đà Lạt» vẽ ở y=257, rồi nhảy xuống y=406. Nhảy sau 95–104 ms ở C1 và 28–37 ms ở C9; cả danh sách dịch 149dp. Ảnh ghép: khung 332 ms danh sách sát ô tìm, khung 474 ms mới có chỗ trống cho sân khấu |
| Evidence | ![khung hình mở tab](evidence/EV-F02-MO12-bat-C1.jpg) (hàng `TC-F02-NHAY`) |
| Source | `ExploreLive.tsx:239-242` và `:401`: khung `sanThanhPho` không có chiều cao; `SanKhau` chỉ vẽ sau khi `onLayout` cho ra `rongSan` |
| Hậu quả | Nội dung nhảy dưới ngón tay đúng lúc vừa hiện. Với người bật giảm chuyển động, đây là chuyển động họ không muốn |
| Đề xuất sửa | Cho `sanThanhPho` một `aspectRatio` (khung.w/khung.h) để giữ chỗ trước khi đo |
| Tiêu chí gỡ | `TC-F02-NHAY` nhảy 0dp ở C1 và C9 |

### UI-026 · Bỏ lọc hoặc xoá tìm làm sân khấu dựng lại từ đầu mỗi lần

| Trường | Nội dung |
|---|---|
| Category / Severity | VISUAL POLISH · **P3** |
| Feature / Screen | F02 · `/explore` · MO12 |
| Nền tảng, cấu hình | web, C1 |
| Tái hiện | Chạm chip «Cafe», rồi chạm lại để bỏ lọc |
| Expected | Theo chú thích trong `ExploreLive.tsx:236` («It stands up once per city»), sân khấu quay lại ở tư thế đứng |
| Actual | Sân khấu mount lại và chạy lại cú bật dựng từ phẳng tới đứng mỗi lần bỏ lọc (khoảng 1 giây trong môi trường này; SwiftShader không đại diện cho thời lượng) |
| Evidence | ![bỏ lọc ở C1](evidence/EV-F02-MO12-bo-loc-C1.jpg) |
| Source | `ExploreLive.tsx:239` (render có điều kiện `!dangLoc && query === ""`); `src/rudi/ui/SanKhau.tsx:55-66` (bật dựng một lần mỗi lần **mount**) |
| Đề xuất sửa | Giữ `SanKhau` mount và gập bằng `gap` (API đã có), thay vì unmount |
| Tiêu chí gỡ | Bỏ lọc không phát lại cú bật dựng |

### UI-027 · Giảm chuyển động: sân khấu trống một lúc khi mount lại

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (hiển thị) · **P3** |
| Feature / Screen | F02 · `/explore` · `KhungSkia` (chung cho mọi tranh Skia) |
| Nền tảng, cấu hình | web, C9 (`prefers-reduced-motion: reduce`). C1 không thấy trống: ở đó SVG được giữ trong 400 ms của cú fade |
| Tái hiện | Như UI-026, ở C9 |
| Expected | Chú thích `KhungSkia` hứa không bao giờ trống: SVG ở lại tới khi canvas Skia đã vẽ |
| Actual | Dòng thời gian `data-renderer` trên cùng đồng hồ với screencast: 71 ms SVG và Skia cùng mount, Skia opacity 0; 145 ms SVG bị gỡ, Skia opacity 1. Khung 176 ms vùng sân khấu trống; khung 609 ms tranh Skia mới hiện. Trống khoảng 430–460 ms |
| Evidence | ![bỏ lọc ở C9](evidence/EV-F02-MO12-bo-loc-C9.jpg) |
| Source | `src/rudi/ui/KhungSkia.tsx:65-93` (`HienSauKhiVe` đợi đúng hai `requestAnimationFrame`, rồi fade; giảm chuyển động nên thời lượng 0), `:130-134` (gỡ SVG khi fade xong) |
| Root cause | Hai rAF không phải tín hiệu «canvas đã vẽ»; dựng surface CanvasKit mất lâu hơn |
| Giới hạn | Độ dài khoảng trống phụ thuộc GPU, SwiftShader chậm hơn máy thật. Cơ chế gỡ SVG trước khi canvas có khung đầu thì không phụ thuộc máy |
| Đề xuất sửa | Chỉ gỡ SVG sau khung vẽ thật đầu tiên của canvas |
| Tiêu chí gỡ | Ở C9 không khung nào có vùng sân khấu trống |

### UI-028 · Thành phố chưa có quán: trạng thái rỗng khuyên bỏ một bộ lọc không tồn tại

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE · **P3** |
| Feature / Screen | F02 · `/explore` sau `/destinations` |
| Nền tảng, cấu hình | web, C1 |
| Tái hiện | «Đổi điểm đến», chọn Hội An (0 nơi trong danh mục) |
| Expected | Nói rõ Hội An chưa có địa điểm, lối ra là đổi điểm đến |
| Actual | «0 nơi ở Hội An», «Chưa thấy nơi phù hợp», «Thử từ khóa khác, hoặc bỏ bớt bộ lọc để thấy lại cả danh mục.» và nút «Xóa lọc», trong khi không có bộ lọc hay từ khoá nào |
| Evidence | ![Hội An rỗng](evidence/EV-F02-HOI-AN-C1.jpg) |
| Source | `ExploreLive.tsx:348-356`: một `EmptyState` cho mọi danh sách rỗng, không xét `dangLoc` |
| Đề xuất sửa | Nhánh riêng khi `!dangLoc`: «Hội An chưa có địa điểm nào», kèm «Đổi điểm đến» |
| Tiêu chí gỡ | Thành phố rỗng không hiện «Xóa lọc» |

### UI-029 · Khám phá gặp lỗi máy chủ nhưng bảo người dùng kiểm tra mạng

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE · **P3** |
| Feature / Screen / Layer | F02 · `/explore` · L35 trạng thái lỗi |
| Nền tảng, cấu hình | web, C1; `/places` trả 503 (chặn ở trình duyệt) |
| Actual | Tiêu đề «Chưa đọc được danh mục», thân là câu mặc định của kit: «Kiểm tra mạng rồi thử lại. Những gì bạn đã nhập vẫn còn nguyên.» Màn này không có gì để nhập. «Thử lại» tải lại được «10 nơi ở Đà Lạt» (đạt) |
| Evidence | ![503](evidence/EV-F02-LOI-503-C1.jpg) |
| Source | `ExploreLive.tsx:271` không truyền `body`, dù `trang.loi` đã có câu đúng nguyên nhân (`loiRaChu` → `thongDiepNguoiDoc`; 5xx là «Rủ Đi đang gặp sự cố…»). Đếm tĩnh toàn `src/`: 24 chỗ dùng `ErrorState` khác đều truyền `body`, đây là chỗ duy nhất |
| Đề xuất sửa | `body={trang.loi}` |
| Tiêu chí gỡ | 503 hiện câu về máy chủ; mất mạng hiện câu về mạng |

### UI-030 · Mất mạng rồi quay lại tab: danh sách đã tải bị thay bằng màn lỗi

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE · **P3** |
| Feature / Screen / Layer | F02 · `/explore` · L35 |
| Nền tảng, cấu hình | web, C1 (`context.setOffline(true)`) |
| Tái hiện | Mở Khám phá (10 nơi), ngắt mạng, chạm tab «Lên plan», rồi quay lại «Khám phá» |
| Expected | Nội dung đã tải vẫn xem được, kèm một câu báo đang offline |
| Actual | Hiện «Chưa đọc được danh mục» và «Thử lại»; 10 nơi đã tải biến mất (tên thành phố và sân khấu vẫn còn) |
| Evidence | ![offline](evidence/EV-F02-OFFLINE-C1.jpg) |
| Source | `ExploreLive.tsx:159-163` (`useFocusEffect` gọi `nap()` mỗi lần focus), `:154-156` (lỗi thì `setTrang({ pha: "hong" })`, bất kể đã có dữ liệu) |
| Đề xuất sửa | Khi đã có dữ liệu, lỗi lúc nạp lại chỉ hiện một dòng báo và giữ danh sách |
| Tiêu chí gỡ | Mất mạng rồi đổi tab không làm mất danh sách |

### UI-031 · Lưới Điểm đến luôn 2 cột, kể cả ở expanded

| Trường | Nội dung |
|---|---|
| Category / Severity | VISUAL POLISH · **P3** |
| Feature / Screen | F02 · `/destinations` |
| Nền tảng, cấu hình | web; C6 (768) đạt, C7 (1024) không đạt |
| Expected | DESIGN.md: expanded 3 cột; `gridFor(…, maxColumns = 3)` cho hàng thẻ |
| Actual | Ở 1024 vẫn 2 cột, mỗi thẻ khoảng 450px; một màn chỉ thấy 8/15 thành phố |
| Evidence | ![Điểm đến C6, C7](evidence/EV-F02.S02-rong-BASE-ghep.jpg) |
| Source | `src/rudi/screens/explore/DiemDenScreen.tsx:120` (`Math.floor((rongLuoi - KHE) / 2)`) |
| Đề xuất sửa | Dùng `gridFor` như các lưới thẻ khác |
| Tiêu chí gỡ | C7 hiện 3 cột |

## F09 Hồ sơ · Cài đặt

### UI-015 · Màn Cài đặt hiện dữ liệu giữ chỗ («Bạn», avatar «B», công tắc tắt) rồi mới đổi

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE · **P3** |
| Feature / Screen | F09 · `/settings` (`cai-dat/CaiDatScreen.tsx`) |
| Nền tảng, cấu hình | web, C1, stack cục bộ (mạng nhanh) |
| Tái hiện | Từ Cá nhân chạm «Cài đặt», quay khung hình |
| Actual | Ở 130 ms: avatar «B», tên «Bạn», công tắc «Tìm theo số điện thoại» tắt. Từ 162–197 ms: «Minh Anh», công tắc lật sang bật. Mạng chậm thì trạng thái giả kéo dài hơn |
| Evidence | ![khung mở Cài đặt](evidence/EV-F00-MO01-push-C1.jpg) |
| Hậu quả | Nháy nội dung sai; công tắc tự lật trông như thiết lập vừa bị đổi. Chạm vào công tắc trong lúc còn hiện trạng thái giả có thể ghi ngược ý người dùng (HYPOTHESIS, chưa đo) |
| Đề xuất | Skeleton hoặc ẩn control tới khi có dữ liệu; công tắc `disabled` trong lúc tải |
| Tiêu chí gỡ | Không khung nào hiện tên/giá trị giữ chỗ |
