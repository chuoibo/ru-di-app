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
| P2 | UI-002, UI-003, UI-004, UI-006, UI-011 |
| P3 | UI-001, UI-007, UI-008, UI-009, UI-010, UI-012, UI-013, UI-014, UI-015 |

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

### UI-003 · Trên web, `accessibilityState` không tới DOM: thanh tab không báo tab đang chọn (và 34 chỗ khác)

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (accessibility, web) · **P2** |
| Feature / Screen / Layer | F00 · mọi màn tab · L05 `RudiTabBar`; hệ thống |
| Nền tảng, cấu hình | web, C1, C2, C3, C6, C7. Native: `accessibilityState` là API chuẩn của RN nên không bị (STATIC) |
| Tái hiện | Mở bất kỳ tab nào, đọc thuộc tính ARIA của 4 phần tử `role="tab"` |
| Expected | Tab đang chọn có `aria-selected="true"`, và các tab nằm trong một `role="tablist"` |
| Actual | Cả 4 tab đều không có `aria-selected`, và không có `tablist`. Nhìn bằng mắt vẫn phân biệt được tab đang chọn (màu, icon đặc, dải washi), nhưng trình đọc màn hình thì không |
| Evidence | Số đo runtime ở 5 cấu hình: `chon: null` ở mọi tab. Mã `react-native-web` 0.21 (`dist/modules/createDOMProps`) nhận `aria-selected`/`accessibilitySelected` mà **không** đọc object `accessibilityState`. Trong app có 35 chỗ dùng `accessibilityState` (selected, checked, expanded, busy, disabled), ví dụ `ui/RudiTabBar.tsx:84`, `ui/ChonNgayLich.tsx`, `screens/Onboarding.tsx`, `chat/CaiDatNhom.tsx` |
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
