<!-- Bản chép kế hoạch đã được Lead duyệt ngày 2026-09-24 (ADR-0037). Nguồn: phiên Claude, kế hoạch t-i-mu-n-th-nh-t-mighty-magpie. Theo tiền lệ ADR-0020 (bản chép 03-ui-v2): bản này là hợp đồng của chiến dịch; chỗ nào bản ship lệch thì ghi lại ở nhật ký docs/claude/, không sửa hậu nghiệm ở đây. -->

# Kế hoạch UI v3 «Sân khấu giấy»: Rủ Đi thành cuốn sổ pop-up, Nếp là con rối giấy dẫn chuyện

## Context

- **Vì sao làm:** QC 24/09 (commit `9036001`, `docs/claude/2026-09-24/qc-frontend-chieu-sau-va-cau-chuyen.md`) xác nhận
  phản hồi của Lead. Sau đăng nhập, mọi màn đều cùng một khuôn form:
  - 22/41 màn dùng `TopBar` + `Heading`; `RudiButton` xuất hiện 218 lần, `StampButton` chỉ 2 lần.
  - Kho vẽ (24 tư thế Nếp, 10 cảnh, 8 sticker) chỉ hiện ở trạng thái rỗng; Nếp chỉ còn một mép giấy 10dp.
  - Chữ pháp lý chiếm chỗ của câu chuyện; không có khoảnh khắc nào; người không có mặt (avatar chữ cái cùng màu).
- **Gốc rễ nằm ở luật cũ, không ở từng màn làm ẩu.** Những luật giữ UI trơn:
  - Nếp chỉ ở trạng thái rỗng và «đứng xa tiền» (DESIGN.md 1266-1291).
  - `celebrate` chỉ cho 3 khoảnh khắc; không animation nền (ADR-0020 §2.4).
  - Avatar «màu theo tông màn, không theo người».
  - Chỉ `KhungAnh` được có bóng.
  - `Field` (ô có viền) là ô nhập duy nhất.
  - Review mỹ thuật chỉ chạy trên fixture, «chưa phủ các cửa live».
- **Lead chốt ngày 24/09:**
  1. Hướng **«Sân khấu giấy pop-up»**.
  2. **Nếp diễn khoảnh khắc** (mở lại luật cũ bằng ADR mới).
  3. **Thêm Skia** (`@shopify/react-native-skia` 2.6.2, bản Expo SDK 57 khuyến nghị).
  4. **Làm hết rồi giao một lần.**
- **Kết quả mong muốn:**
  - Mọi màn live và mọi pop-up thành sân khấu giấy cắt nhiều lớp: có chiều sâu, chuyển động mang nghĩa, ít chữ, ít ô.
  - Nếp là con rối giấy dẫn chuyện.
  - Chạy tốt trên phone, tablet và web.
  - Không đổi một dòng logic tiền hay API.

---

## 1. Thế giới «Sân khấu giấy»

- **Cuốn sổ pop-up của hội.** Bìa vải indigo giữ nguyên. Mở ra là trang giấy dày có nếp gấp giữa. Mỗi màn là
  một **sân khấu** gồm nhiều tầng giấy cắt, dựng lên từ nếp gấp đó.
- **Vật thể thay form.** Mỗi việc là một vật giấy:

  | Việc | Vật giấy |
  |---|---|
  | Hội | bìa sổ |
  | Mời | phong bì |
  | Kèo | thiệp / vé |
  | Chia bill | hoá đơn nhiệt |
  | Kết quả chia | cuống phiếu xé |
  | Đợt thu | trang sổ thu |
  | Sổ hai người | thư gấp ba |
  | Huy hiệu | tem |
  | Hồ sơ | hộ chiếu |
  | Kỷ niệm | ảnh instax |
  | Story | polaroid |

- **Ba tông là ba loại giấy màu:** cam là giấy mời, teal là giấy tiền, tím là giấy can của AI.
- **Người là hình nhân giấy.** Avatar có màu mực riêng từng người, đứng trên đế trong các cảnh.
- **Nếp là con rối giấy có đinh ghim ở khớp.** Nếp kéo tab, dập dấu, gấp thư, cầm máy ảnh.
- **Cơ chế sách pop-up dùng làm tương tác:**
  - tầng bật dựng;
  - lật trang;
  - tab kéo;
  - nắp lật (chữ phụ nằm dưới nắp);
  - đĩa xoay (chọn giờ, mức chi);
  - xé cuống;
  - kéo thả vào ghế.

  Mọi cử chỉ đều có cách chạm tương đương và thao tác trợ năng tương đương.
- **Độ cao giấy thay «chỉ KhungAnh có bóng»:**

  | Mức | Là gì | Bóng |
  |---|---|---|
  | 0 | in trên trang: chữ, dấu, hàng | không |
  | 1 | dán lên trang | `cardShadow` |
  | 2 | đứng dựng | bóng đổ theo góc gập |
  | 3 | đang cầm lên | bóng lớn, mềm |

  Một nguồn sáng duy nhất từ trên-trái. Ở theme tối là đèn bàn.
- **Mười cử động chữ ký:**
  1. Sân khấu bật lên khi mở trang (so le theo độ sâu, tổng ≤420ms).
  2. Cảnh gập phẳng theo cuộn, tiêu đề thu vào thanh.
  3. Thị sai theo cuộn/kéo; tuỳ chọn nghiêng máy trên native ở 3 cảnh.
  4. Mực tự vẽ cho đường kèo và chữ ký.
  5. Dấu lớn rơi xuống, có mực loang (shader).
  6. Lật trang giữa các bước.
  7. Thư gấp ba rồi bay đi.
  8. Hoá đơn xé thành cuống phiếu.
  9. Bàn ăn pop-up: kéo đĩa tới ghế.
  10. Bìa sổ mở ra (tái dùng cử động của Welcome).
- **Nếp diễn đúng 8 khoảnh khắc**, mỗi sự kiện một lần, chạm để bỏ qua:

  | # | Khoảnh khắc | Tiết mục |
  |---|---|---|
  | M1 | khay tạo mở | `buoc-vao` |
  | M2 | chụp bill | `cam-may` |
  | M3 | ghi sổ xong | `dong-dau` |
  | M4 | tiền đã về | `cui-cam-on` |
  | M5 | tạo kèo | `buoc-di` |
  | M6 | sổ hai người mở | `keo-tab` + `nhay` |
  | M7 | gửi tờ giấy | `gap-thu` |
  | M8 | huy hiệu mới | `nang-tem` |

- **Luật cho Nếp:**
  - Đứng trong vùng riêng của bố cục, không bao giờ nổi đè lên nội dung.
  - Cách mọi con số tiền ≥16dp; tay và mắt chỉ hướng vào đạo cụ của mình.
  - Ở M2–M4 mặt chỉ được `binh-than` hoặc `nhuong`.
  - Không bao giờ ở màn lỗi, màn từ chối, màn xung đột.
  - Khi Nếp trong trang đang hiện, dock nhường chỗ.
  - Dock theo ADR-0035 giữ nguyên.
- **Vẫn cấm:** confetti và hạt bay, toast, modal lỗi, hero metric, thẻ lồng thẻ, bóng lệch cứng, animation tự
  chạy vô hạn. Chỉ được một vật đơn lẻ bay một lần (tờ lịch, cuống phiếu, lá thư).

---

## 2. Luật không đổi (mọi lát)

- **Hành vi giữ nguyên:** route, lời gọi API, body, khoá idempotency/attempt (`attemptFor(...)`, `lam(...)`),
  validator (`kiemTraTaoBuoiDi`, `ngayVeISO`, `loiHoaDon`, `loiGanMon`), máy chủ tính mọi số tiền, nhánh
  fixture/live, và các phiếu `useNepNguCanh`.
  - Ngoại lệ có tên: B1, B2, B3, B4, B5, B7, B8 trong QC, đều sửa phía client và khoanh vùng.
- **Chuỗi Maestro:** mọi chữ, accessibilityLabel và testID mà `.maestro/*.yaml` bấm hoặc kiểm đều giữ nguyên.
  - Không đổi tên `takeScreenshot`.
  - Đổi câu chữ nào thì sửa flow trong cùng commit.
- **Chuyển động chỉ để trang trí:**
  - Chữ có mặt từ khung hình đầu.
  - Không khoá input.
  - «Luật Control Đứng Yên»: control không xoay 3D, không dịch quá 4dp khi đang animate.
- **Mọi chữ là RN `Text`.** Canvas chỉ trang trí, hoặc mang đúng một câu accessibilityLabel.
- **Màu và hình:**
  - Màu chỉ đi `packages/shared/tokens.json` → `src/rudi/theme.ts`.
  - Hình chỉ đi `src/rudi/art/*.ts` (tuyệt đối `M/L/C/Z`, số dựng lúc chạy).
  - Không có dãy ≥9 chữ số, kể cả trong SkSL.
  - Không dùng chuỗi phần trăm hiển thị.
- **Theme tối, nền `paper` không phải mặt chữ (đã đo):**
  - `accent` 4,12:1, `warn` 4,00, `inkFaint` 4,37, `lineStrong` 3,23 trên `#2e335c`.
  - Hiện đang ship sai ở hai chỗ: chữ «cần kiểm» trong `ChiaBillLive`, dấu cam trên `ToLoiRu`.
  - Vật thể mang chữ phải dùng nền `card`. Tờ `ToGiay` chỉ dùng `ink`/`inkSoft`/`split`/`ai` và dấu `variant="ink"`.
- **Phải còn nhìn thấy, không gấp dưới nắp:**
  - câu đồng thuận lúc đồng ý (Lập sổ);
  - hậu quả của việc không hoàn tác được (Phát đợt thu, `XacNhanViec`, xoá tài khoản);
  - câu «số này là gì / không phải gì» dưới mỗi sổ;
  - câu riêng tư ngay chỗ thu thập dữ liệu;
  - «Chưa mã hoá đầu cuối»;
  - mọi lỗi.

  Phần giải thích cách dùng, luật vai, mẹo thì được đưa vào `NapGiay` (nắp lật) hoặc `ChuThichLe` (chú thích lề).
- **Hình dạng mã đang bị test ghim, giữ nguyên:**
  - JSX `<LoaiSo>` / `onLuu` trong `KhongGianGiay` (`so-doi-dong-y`);
  - `ChonNguoi` / `Conversations` (`chon-nguoi-that`);
  - biểu thức trong allowlist `mac-dinh-am-tham-id` (`CreateOutingLive`, `NepBang`);
  - cách viết `borderColor` của `RudiButton`/`Field`/`Chip`/`CoverButton`/`Card` và phụ đề `Heading` (`test_contrast_floor.py`).
- **Chỗ đặt file mới:** thành phần mới vào `src/rudi/ui/**` hoặc `src/rudi/art/**` (`check_screens_reachable.py`).
  Tên file không chứa bill/receipt/export.

---

## 3. S0.1: ADR-0037 và hệ thiết kế

- **`docs/decisions/ADR-0037-san-khau-giay-va-nep-dien.md`** (Chấp nhận, Lead 24/09).
  - Sửa: ADR-0020 §2.4 (chuyển động) và §2.5 (avatar); luật DESIGN «Nếp Đứng Xa Tiền» và «Trong/Trên Trang».
  - Không đụng: ADR-0035, ba luật tiền, cam kết thương hiệu, «Một Tông Dẫn», «Một Face».
  - Các quyết định:

    | Mã | Nội dung |
    |---|---|
    | D1 | Hướng và bộ từ vựng (sân khấu, tầng, đạo cụ, con rối, tab kéo, nắp, đĩa xoay, cuống, hình nhân) |
    | D2 | Độ cao giấy 0–3 |
    | D3 | Ngân sách chuyển động |
    | D4 | Một lần mỗi sự kiện, tính theo khoá sự kiện mỗi phiên |
    | D5 | Nếp diễn 8 khoảnh khắc và ranh giới (mục 1) |
    | D6 | Mực người |
    | D7 | Cử chỉ pop-up luôn có tương đương chạm và trợ năng |
    | D8 | «Luật Nói Rõ Trước Khi Bấm» (mục 2) |
    | D9 | Trợ năng canvas |
    | D10 | Skia vẽ sân khấu/con rối/chất liệu; SVG giữ cho hàng danh sách và làm dự phòng |
    | D11 | Không hạt bay |
    | D12 | Bằng chứng trên live, không trên fixture |
    | D13 | Token mới đặt ở mục riêng, không đổi `guest.css` (Python legacy) |
    | D14 | «Bàn đêm» và dấu teal (mực tĩnh trên `brand.teal` 5,33:1; không có dấu tím, 3,34:1) |

  - Chi tiết D3:
    - `MOTION_MS` giữ đúng 4 bậc (`motion.test` ghim).
    - Thêm ngân sách ghép `tokens.motion.sanKhau = { batTang: 40, batToiDa: 420, dien: 1400, lat: 300 }`.
    - Chuyển động gắn với cuộn/kéo/cảm biến không phải vòng lặp và dừng khi tay dừng.
    - `withRepeat(-1)` chỉ được trong `Skeleton`.
    - Cảm biến nghiêng: chỉ native, 3 cảnh, ≤3dp / 2°, tắt khi không focus hoặc khi Giảm chuyển động.
    - Khi Giảm chuyển động:

      | Chuyển động | Thay bằng |
      |---|---|
      | Bật dựng, tiết mục | Khung cuối tĩnh |
      | Lật trang | Cắt thẳng |
      | Gập theo cuộn | Mờ dần, không 3D |
      | Thị sai | Tắt |

  - Chi tiết D6:
    - Mục `mucNguoi.{light,dark}`, 8 màu.
    - Mỗi màu đạt ≥4,5:1 trên `paper`/`card`/`ground` của scheme mình.
    - Hue cách ≥30° so với các tông ngữ nghĩa và thương hiệu.
    - Chọn màu bằng băm FNV-1a của person id.
- **Chép kế hoạch** vào `docs/architecture/04-ui-v3-san-khau-giay.md` (giống tiền lệ UI v2), và cập nhật hợp đồng
  hướng đi v3 trong comment `apps/mobile/app/_layout.tsx`.
- **Token (chỉ thêm):**
  - `sanKhau.{light,dark}.{bong, anhDen, gayGiay}`
  - `sanKhau.cao.{1,2,3}.{dy, blur, alpha}`
  - `mucNguoi.{light,dark}`
  - `motion.sanKhau`

  Giữ nguyên `ORDER` 25 khoá của `scripts/sinh_token_ui_v2.py`, `guest.css` và `test_shared_tokens.py`. Script
  sinh thêm bảng DESIGN cho mực người và bóng sân khấu.
- **`theme.ts`:** thêm `mauSanKhau(dark)`, `mucNguoi(dark)`, `bongCao(level, dark)`.
- **`motion.ts`:** thêm `NGAN_SACH_SAN_KHAU` và `batToiDa()`; giữ nguyên `celebrateOnce`.
- **`useMotion`:** trả thêm ngân sách mới.
- **DESIGN.md:**
  - S0 thêm ghi chú «đang chuyển».
  - S7 viết lại các mục Overview, Colors, Elevation, Shapes, Components, Lớp vẽ, Inputs, Navigation, Signatures,
    Chuyển động, Câu chữ, Do/Don't, «Cổng phải xanh».

---

## 4. Kiến trúc kỹ thuật

### S0.2: Skia

- **Cài đặt:**
  - `npx expo install @shopify/react-native-skia` (ra bản 2.6.2).
  - Kiểm `npm ls` chỉ có một bản reanimated/worklets/react.
  - Ghim lại sha256 của `apps/mobile/package-lock.json` trong `.repo-guard-allowlist.json`.
  - Không cần config plugin. Lead phải dựng lại dev client.
- **Ranh giới module:** mọi import Skia chỉ nằm trong `src/rudi/ui/skia/**`. Gate `skia-ranh-gioi.test.mjs` giữ luật này.

  | File | Vai trò |
  |---|---|
  | `nap-skia.ts` (web) | Dò WebGL; có thì `LoadSkiaWeb({locateFile: f => "/"+f})` nhớ một lần; trạng thái `chua\|dang-nap\|san-sang\|loi` |
  | `nap-skia.native.ts` | Dò module native trước khi `require`; dev client cũ thì rơi về SVG, không sập |
  | `VeSkia.tsx` | `LopVe[]` → `Path`, cache LRU 500 cho `Skia.Path.MakeFromSVGString`, cùng bảng `mauLop()` |
  | `SanKhauSkia`, `NepRoiSkia`, `DauLonSkia`, `BanTronSkia`, `chat-lieu.ts` | Renderer |
  | `sksl.ts` | Chuỗi shader, thuần |

- **Lớp bọc công khai** (`ui/SanKhau.tsx`, `ui/NepDien.tsx`, `ui/DauLon.tsx`):
  - Bản `.native.tsx` import Skia trực tiếp.
  - Bản web dùng `React.lazy` sau khi CanvasKit sẵn sàng, `Suspense` với SVG làm fallback, và error boundary rơi về SVG.
  - Theo tiền lệ `BanDo.tsx` / `BanDo.native.tsx`.
  - Gắn `data-renderer="skia|svg"` để lấy bằng chứng.
  - Công tắc QA `EXPO_PUBLIC_QA_TAT_SKIA=1` ép dùng SVG.
- **Web:**
  - `tools/chep-canvaskit.mjs` chép `canvaskit.wasm` vào `public/`, kiểm sha256 (chạy ở `postinstall`, `build:check`, `web`).
  - `public/canvaskit.wasm` vào `.gitignore` (tệp >2MiB, repo guard sẽ chặn).
  - Thêm `preload` trong `public/index.html`.
  - Không chặn khung hình đầu: SVG vẽ trước, rồi chuyển mờ sang Skia trong 200ms, không bao giờ giữa lúc đang animate.
  - Thêm MIME `.wasm` vào `tests/chrome-cdp.mjs`.
  - **Spike go/no-go ở S0.2:** web export đóng gói được, CanvasKit vẽ được trong Chrome headless (cờ SwiftShader),
    Group 3D render được, `expo export --platform all` xanh.
  - Nếu web không được thì web chỉ chạy SVG với gập 2D, và ghi rõ là suy giảm.
- **Reanimated → Skia:**
  - Shared/derived value đưa thẳng vào prop.
  - Tầng pop-up: `<Group origin={vec(cx, nep)} transform={[{perspective: 700}, {rotateX: θ}]}>`.
  - Dự phòng: gập trực giao `scaleY = cos θ` (cũng là cách bản SVG chạy).
  - Mực tự vẽ: `<Path start={0} end={sv}>`.
- **Chất liệu:**
  - Hạt giấy trong canvas là `ImageShader` của các ô PNG đã đo (`giayTrang`, `vaiBia`).
  - Chỉ một hiệu ứng SkSL: `mucLoang` (mực loang ở viền dấu và nét chữ ký), hạt giống cố định, hằng số ngắn.
  - Ánh sáng: `RadialGradient` từ nguồn sáng cam của cảnh, hoặc đèn bàn ở theme tối.
  - Tầng gập đổi màu theo góc bằng `interpolateColors(θ)`.
  - Bóng: bóng chiếu dẹt kèm `BlurMask` ≤10; không dùng `Shadow` filter trên tầng đang animate.
- **Ngân sách hiệu năng:**
  - ≤3 canvas sống mỗi màn; 0 canvas trong hàng danh sách (hàng dùng `VeLop` SVG).
  - `createPicture` cho tầng tĩnh; ≤12 derived value và ≤1 runtime effect mỗi canvas.
  - Web: canvas tĩnh huỷ WebGL context; màn không focus vẽ SVG (giới hạn 16 context mỗi trang).
  - Mục tiêu native: jank ≤5%, p90 ≤16ms (Lead đo bằng `dumpsys gfxinfo`).
- **Test node:** chỉ module thuần vào `tsconfig.test.json`, không import RN/reanimated/Skia.

### S0.3: Máy sân khấu pop-up

- **`src/rudi/art/san-khau.ts` (thuần):**
  - `TangSanKhau {id, sau 0-3, nep, dung, cao, lop: LopVe[]}`
  - `SanKhau {id, khung, nen, tang[], nguonSang, moTa}`
  - Builder dùng lại kho hiện có:
    - `sanKhauTuCanh(id)` tách 10 cảnh của `canh.ts` thành sàn / đạo cụ đứng / Nếp;
    - `sanKhauKyHoa(loai, tags, {gon})` chia ký hoạ thành 3 độ sâu, đúng một nguồn sáng cam;
    - `sanKhauBanLamViec()`, `sanKhauBan()`.
- **`src/rudi/art/giay.ts` (thuần):** `rangCua` (răng cưa), `lotLo` (mép tem), `khuyetVe` (khuyết vé),
  `duongXe` (đường xé), `napPhongBi` (nắp phong bì), `goGap`. Tất cả tất định theo bề rộng.
- **`src/rudi/san-khau/dong-hoc.ts` (thuần, worklet):**
  - `gocBatTang(mo, i, n)`, `gocGapCuon(y, h)`, `lechThiSai(v, sau)`, `bongTheoGoc(θ, cao)`, `huongLat`.
  - `NHIP_DAU {lao 130, cham 60}`.
  - `kichThuocSanKhau(layout, fontScale)`: cao 160–220 trên phone, 220–260 medium; trang trái khi expanded; dạng
    `gon` khi `chuLon`; ẩn khi chiều cao `short`.
- **Thành phần:**
  - `ui/SanKhau.tsx` (fallback `ui/art/SanKhauSvg.tsx`): bật dựng một lần mỗi lần mount; tham số `thiSai: cuon|keo|nghieng`.
  - `ui/CanhGap.tsx`: sân khấu làm đầu màn, gập phẳng theo cuộn; thanh tiêu đề gọn gắn lên đầu; nút «Quay lại»
    luôn đứng yên. `RudiScreen` thêm slot `canh?` (chỉ thêm) và cấp `cuonY` qua context.
  - `ui/useThiSai.ts`: thị sai theo cuộn / pan ±6° (về chỗ bằng `spring.settle`) / `useAnimatedSensor` (chỉ native).
  - `ui/KeoTab.tsx`: tab kéo; chạm cũng làm được; có accessibilityActions.

### S0.4: Nếp con rối giấy

- **Tách hàm, không xê dịch một pixel.** Tách ruột `hinhNep` thành `thanNep`, `matNep`, `chanNep`, `tayNep` rồi ghép
  lại theo đúng thứ tự. Test sha256 bản `trang` phải giữ xanh. Tách `khongCatNepGap`/`laDaiGap` ra `tests/_nep-gap.mjs`.
- **`src/rudi/art/nep-roi.ts`:**
  - Bộ phận: thân + mặt (7 biểu cảm), cánh tay trên/dưới, bàn tay, đùi, cẳng, bàn chân. Mỗi bộ phận có trục xoay.
  - Chấm đinh ghim ở vai và hông.
  - `KhopNep {nghieng, nhin, bieuCam, vai/khuyu/hong/goi × gần/xa, dx, dy}`.
  - `ik2` đổi toạ độ tay có sẵn thành góc khuỷu, nên mỗi `PoseNep` thành tư thế nghỉ `TU_THE_ROI[pose]`.
  - `tuTheRoi()` trả `LopVe[]` (cho test và cho khung cuối SVG); `maTranBoPhan()` trả ma trận cho Group lồng của Skia.
- **`src/rudi/art/nep-dien.ts`:** `TIET_MUC` gồm 9 tiết mục ≤1400ms:
  `buoc-vao`, `keo-tab`, `cam-may`, `dong-dau`, `cui-cam-on`, `buoc-di`, `gap-thu`, `nhay`, `nang-tem`.
  Mỗi tiết mục có khung khoá, nhịp haptic, vật cầm. `khopTai(t)` là worklet.
- **Runtime:**
  - `src/rudi/khoanh-khac.ts`: sổ đăng ký cấp module, trần 200. `nenDien({khoa, reduced, hopLe, coLoi})`.
  - `ui/useKhoanhKhac.ts`: một đồng hồ chung cho sân khấu, con rối và dấu; chạm để bỏ qua; haptic qua `runOnJS`;
    chú thích dùng `accessibilityLiveRegion`.
  - `ui/NepDien.tsx`: giữ chỗ trong bố cục (128 / 112 / 88 / 0 dp theo cỡ); gọi `useNhuongChoNep(true)`; tắt khi có công tắc QA.
  - `ui/useNhipDau.ts`: tách 3 nhịp của `Stamp` ra dùng chung cho `Stamp`, `DauLon` và con rối; xoá `DauDaGhi` bản lệch
    trong `ChiaBillLive`.

### S0.5: Primitives dùng chung

| File | Vai trò | Module thuần + test |
|---|---|---|
| `ui/LatTrang.tsx` | Khung bước: trang mới xoay quanh gáy 300ms; TopBar/Stepper nằm ngoài | `dong-hoc.ts` |
| `ui/DauLon.tsx` | Dấu lớn (viền Skia + `mucLoang` + nhãn RN) | `NHIP_DAU` |
| `ui/ONhapMuc.tsx` | Ô không viền, gạch mực 1dp `lineStrong`, 2dp `accent` khi focus, `warn` khi lỗi; `outlineStyle:none` trên web (sửa B7) | render test + dòng mới trong `interactive_boundaries()` |
| `ui/CauRu.tsx` | Câu điền chỗ trống: chữ tĩnh là `Text` riêng, mỗi ô ≥48dp và tự xuống dòng | `ui/cau-ru.ts` |
| `ui/ChonNgayLich.tsx` | Lịch xé (`to-lich`) hoặc gõ tay kèm nút lịch (`dong`); luôn trả `dd/mm/yyyy` về `ngayVeISO` | `ui/lich/lich-thang.ts` |
| `ui/BanXoay.tsx` | Đĩa xoay chọn giờ (nấc 15 phút) / mức chi; role `adjustable`; chạm tâm để gõ | `ui/ban-xoay.ts` |
| `ui/NapGiay.tsx`, `ui/ChuThichLe.tsx` | Nắp lật cho chữ phụ; chú thích lề (vẫn là chữ nhìn thấy) | contrast row |
| `ui/Avatar.tsx` (thêm prop, không đổi API cũ) | `personId` → vòng + chữ màu mực người; ảnh qua `nguonAnhDaiDien` với cache 404; biến thể `dung` (hình nhân) | `nguoi/muc-nguoi.ts`, `nguoi/anh-dai-dien-cache.ts` |
| `ui/StampButton.tsx` | Thêm `tone: "accent"\|"split"` (dấu teal cho quyết định tiền) | test tương phản 5,33 |
| `ui/TheVe`, `ui/CuongPhieu`, `ui/HoaDonGiay`, `ui/PhongBi`, `ui/Tem` | Vé, cuống, hoá đơn nhiệt (font hệ thống + số tabular), phong bì, tem; nền `card`, viền `lineStrong` | `art/giay.ts` |
| `ui/SoMoHaiTrang.tsx` | Trải hai trang ở ≥840dp; `HaiCot` dùng lại | `adaptive.ts` |
| `ui/Sheet.tsx` | Mép trên đục lỗ, vân giấy, tuỳ chọn `dauTrang`; cử chỉ và bẫy focus giữ nguyên | — |

`app/dev/ui-lab.tsx` thêm các mục: sân khấu, 15 thành phố, con rối (nút cho từng tiết mục, bật/tắt `trang`/`manh`,
Giảm chuyển động, Skia/SVG), mọi primitive, chạy thử M1–M8, 8 mực người sáng/tối.

---

## 5. Đặc tả màn (logic, handler, testID, nhãn giữ nguyên)

### S1: Tiền

- **`ChiaBillLive`: `LatTrang` giữa các bước.** Stepper thành tab washi nhưng giữ «Bước n/5» và dòng lý do khoá.
  - **`bat-dau`:**
    - Bàn nhìn từ trên xuống, một `HoaDonGiay` trống. Tờ hoá đơn chính là nút, nhãn «Chọn ảnh bill».
    - «Nhập tay» thành một dòng bút chì (giữ nguyên chữ).
    - Nếp đứng tĩnh ở tư thế `cam-may`.
    - Phụ đề còn một dòng; «Cách chia» vào `NapGiay`.
  - **`xem-anh`:**
    - Ảnh là bản in `KhungAnh` đặt trên bàn.
    - «Chưa gửi gì cho tới khi bạn bấm dùng.» thành `ChuThichLe`, vẫn nhìn thấy.
    - M2 chạy.
  - **`xem-lai`:**
    - `HoaDonGiay` nền `card` (sửa lỗi `warn` trên `paper` ở theme tối).
    - Sửa tại dòng bằng `ONhapMuc` («Ô tên món N», «Ô số lượng món N», «Ô tiền món N»). Hoá đơn ≤3 dòng luôn mở sẵn.
    - «Thêm món» thành dải xé «+ dòng».
    - `cauTongMon` và `cauNguonBill` nằm ở đầu tờ.
  - **`gan-mon`: `BanGanMon` (signature).**
    - Bàn pop-up nghiêng khoảng 35°. Người là hình nhân quanh bàn; món đang chọn là tấm thẻ gấp dựng giữa bàn.
    - Kéo thẻ tới ghế, hoặc chạm ghế («Ghế {tên}»), đều gọi `toggle(assignment, line.id, id)`, kèm haptic và một chấm mực.
    - Ghế là control nên không xoay 3D.
    - Bên dưới giữ `RosterPicker` («{tên} · {món}») và chip «Cả nhóm», «Bỏ hết», «Như món trên», «Cả hai».
    - Dấu «CHIA ĐỀU» khi `everyoneShares`.
    - Màn rộng: `AiCoGi` ở trang phải.
  - **`ket-qua`:**
    - Mỗi người là một `CuongPhieu` (mực người, tên, `Money` teal). «Phần của bạn» là cuống nổi lên đầu tiên.
    - Giữ nhãn «Đã trả bill», «Người trả {tên}», «Ô tên khoản chi».
    - Các câu về ghi sổ và làm tròn thành `ChuThichLe`.
    - «Ghi vào sổ» là `StampButton tone="split"`.
  - **`da-ghi`:**
    - Hoá đơn gập vào trang sổ, `DauLon` «ĐÃ GHI SỔ» rơi xuống cùng M3, cuống phiếu xoè ra.
    - Giữ nguyên chữ «Đã ghi: …», «Xem quyết toán», «Về Tin nhắn».
- **`DotThuLive`:**
  - `CanhGap` trang sổ thu với dải washi tiến độ (đếm từ máy chủ), luôn đi kèm câu «{n}/{m} lượt chuyển đã về».
  - Hàng là dòng sổ có ô dấu (`Stamp dong` như cũ); người mang mực riêng.
  - M4 chạy ở đầu trang, không bao giờ ở hàng.
  - Câu hậu quả của Phát đợt thu hiện đủ; nút Phát là dấu teal.
  - Link gửi là `PhongBi` nhỏ.
- **`QuyetToanLive` (trong `Bill.tsx`):**
  - Sơ đồ mũi tên mực giữa các hình nhân, tự vẽ một lần, **không có số** (số chỉ nằm ở danh sách bên dưới).
  - Mọi chữ đang bị flow kiểm giữ nguyên.
  - Không đụng `ReceiptReviewScreen` của fixture (chuỗi bị ghim).
- **`TaiChinhLive` (trong `Profile.tsx`):**
  - Trang sổ cái.
  - Thêm dòng thời gian `movements`: máy chủ đã trả dữ liệu này mà màn chưa bao giờ hiện; dùng helper có sẵn trong
    `tai-chinh.ts` (`tienCoDau`, `ngayNgan`, `moTaGiaoDich`, `ghiChuGioiHan`).
  - Câu tuyên bố vẫn hiện.

### S2: Sổ hai người

- **Chưa có sổ:**
  - Một cuốn sổ nhỏ đóng, dán hai nhãn tên, Nếp `manh` ở tư thế `gap-lai`.
  - «Đề nghị lập sổ» là dấu cam.
- **Tuần chưa có tờ:**
  - Hình `ThuGapBa` lớn, đang gấp.
  - Nếp `dua-giay` hiện **cả lần đầu** (sửa `KhongGianGiay.tsx:134-143, 176`).
  - Tờ thư là nút «Mở tờ giấy mới»; nút «Rủ đi chơi» vẫn riêng.
- **Tờ đang mở (`ToLoiRu`):**
  - `ToGiay` gấp ba thật, có `VetGap`.
  - Phần Nếp phác vẽ bút chì đứt, kèm chú thích lề «Nếp phác, bạn sửa»; chỗ người sửa thì thành mực.
  - «Gửi cho người ấy» chạy M7 (gấp ba, bay đi).
  - «Ừ, hẹn…» giữ nguyên.
- **Lập sổ / Bật một đôi (`DongYBac`):**
  - Giao kèo viết tay: hai cột Cho phép / Không kéo theo hiện đủ, và hai dòng chữ ký.
  - Chữ ký người đề nghị tự vẽ; dòng còn lại để đứt «chờ {tên} ký».
  - Giữ «Lập sổ hai người», «Đề nghị lập sổ», «Đồng ý», «Để sau».
- **M6 khi sổ mở:** chữ ký thứ hai tự vẽ, dấu «SỔ ĐÃ MỞ», bìa sổ mở 3D, Nếp `keo-tab` rồi `nhay`.
- **`DeNghiSua`:** sửa ngay trên tờ, gồm:
  - ngày: dãy lá lịch;
  - giờ: `BanXoay`;
  - chỗ: thẻ `KyHoa` mini;
  - «Vì sao đổi»: ô ghi chú lề.

  testID `de-nghi-sua-*` giữ nguyên.
- **`RangBuoc`, `LoaiSo`, `DongSo`, `GiuMotDieu`, `XacNhanViec`:** chỉ đổi mép `Sheet`; JSX bị ghim không đụng; câu hậu quả hiện đủ.
- **`ChonNguoi`:** bạn bè hiện thành hình nhân mực.
- **B1 (chỉ sửa phía client):**
  1. `nenXinTo` trả «thoi» khi sổ chưa hoạt động (không sinh tờ mồ côi nữa).
  2. Thêm `sauKhiXinTo(code)`. Khi nhận `paper_wrong_state` mà không thấy tờ nào mở, màn hiện «Tuần này đã có một tờ
     đang mở ở phía người ấy. Khi tờ được gửi, nó tới đây.», cùng Nếp `up-xuong`, ẩn «Rủ đi chơi», và có nút «Làm mới».
  3. `SoDoiApi` thêm `coLuot` (fixture true, live false) để live không còn nói «Tuần này bạn mở lời».
  4. Việc sửa phía Go/Python được ghi vào `docs/team/hang-doi.md`.

### S3: Cửa tạo mới

- **Khay «+» (`Create.tsx`):**
  - Bàn làm việc pop-up, lưới 2×n gồm năm vật ký hoạ: tờ lịch, hoá đơn, ảnh in, polaroid, thư gấp.
  - Mỗi ô đều bấm được cả ô; tiêu đề là RN text và giữ nguyên («Chia hóa đơn» là chữ flow bấm).
  - Giữ thứ tự `coCap`. M1 chạy.
- **`CreateOutingLive`: tấm thiệp là cả màn.**
  - `TheVe` có washi mang tên hội. Nội dung là một `CauRu`:
    «Rủ {nhóm} đi [Ô tên kèo] từ [lá lịch] tới [lá lịch], [−/+ Ô số người] người, mỗi người [4 phong bì + Ô ngân sách một người]».
  - Ngày mặc định hôm nay; sheet lịch vẫn có ô gõ «Ô ngày đi» / «Ô ngày về».
  - Phần xem trước là tấm vé.
  - «Tạo kèo» là dấu cam. Thân hàm `tao()` giữ nguyên từng byte, rồi `router.replace(/outings/{id}?vua=tao)` để M5 chạy.
- **`groups/New`: bìa sổ mới.**
  - `CoverBand` vải; tên nhóm là nhãn dán (`ONhapMuc` «Ô tên nhóm»).
  - «Mở nhóm» là dấu cam; thành công thì bìa mở 3D.
  - Câu «Chỉ hai người?…» thành `ChuThichLe`.
- **`groups/Invite`: phong bì.**
  - Dòng địa chỉ là `ONhapMuc` («Ô số điện thoại người được mời», «Ô tên người được mời»).
  - Câu riêng tư vẫn hiện.
  - «Gửi lời mời» là dấu; phong bì niêm lại rồi bay đi.
- **`friends/AddFriend`:** danh thiếp; người tìm thấy hiện thành hình nhân.
- **`LoiMoi`:** phong bì mở ra.
- **Chặng mới (trong `OutingLive`):**
  - Giờ bằng `BanXoay`, «Ô giờ chặng» giữ làm đường gõ.
  - «Ô tên chặng» giữ nguyên.
  - Thẻ quán `KyHoa` mini thay chip.
- **Soạn bình chọn (`SoHen.tsx`):** soạn thẳng trên tấm giấy nhớ; nhãn cũ giữ nguyên.
- **`ToHenChungKhay`:**
  - Giấy kẻ dòng, chặng viết bút chì.
  - `ChonNgayLich kieu="dong"` vẫn gõ được (flow 48 gõ `03/10/2026`).

### S4: Khám phá và Lên plan

- **`ExploreLive`:**
  - `CanhGap` sân khấu thành phố theo `destination.id`, lấy từ `art/thanh-pho.ts`: 15 thành phố cộng một bưu thiếp chung.
    Motif lấy từ chính blurb của máy chủ (đồi thông, phố đèn lồng, vịnh đá vôi…). Câu mô tả bắt đầu «Ký hoạ …».
  - Có thị sai và nghiêng.
  - Thẻ dẫn đầu là bản in `KhungAnh`: ảnh có ghi công nếu có, không thì `KyHoa` lớn (sửa lỗi bản live không bao giờ hiện KyHoa).
  - Mỗi hàng có ảnh thu nhỏ hoặc `KyHoa` thay glyph trùng lặp.
  - Khung ảnh mới phải đăng ký vào `rudi-anh-ghi-cong.test.mjs`.
- **`DiemDenScreen`:** bưu thiếp 2 cột (SVG tĩnh); thành phố đang ở có dấu bưu điện «ĐANG Ở».
- **`PlaceDetailLive`:** `KyHoa` tách 3 độ sâu, hoặc bản in ảnh; «Rủ hội tới đây» là dấu cam.
- **`PlanLive`:**
  - Mỗi kèo là một `TheVe` (tên, ngày, chồng hình nhân, `RouteLine` mini tự vẽ, dấu nhịp).
  - «Đã qua» là chồng cuống vé đã xé.
- **`OutingLive`:**
  - `CanhGap` sân khấu đường kèo tự vẽ, có thị sai; M5 chạy.
  - «Chia bill buổi này» thành nút hình hoá đơn xé.
  - «Tôi đã tới» và mọi chữ đang bị flow kiểm giữ nguyên.
- **`hanh-trinh/*`:** chỉ thêm khung giấy bản đồ xé.
- **B5:** `/votes/[id]`, `/trips/[id]/itinerary`, `/ai-match` chuyển hướng về màn live khi có phiên; flow fixture
  (không có phiên) không bị ảnh hưởng.

### S5: Chat, bảng Nếp, người

- **Avatar mực người + ảnh** ở mọi nơi: thành viên, bạn bè, Rủ ai, Tin nhắn, chat, kết quả chia, đợt thu, quyết toán, hồ sơ, tường.
- **`Conversations`:** mỗi nhóm là gáy sổ tô theo `chatTheme`, kèm chồng avatar; nhắn riêng là phong bì.
- **`GroupChatLive`:**
  - B2: thanh ghim có nền đặc, nội dung bắt đầu dưới thanh.
  - «Chưa mã hoá đầu cuối» là `ChuThichLe` có khoá.
  - Tên người gửi mang mực người.
- **Khay công cụ:** bốn đồ vật ký hoạ; nhãn giữ nguyên.
- **`ThePoll`:** lựa chọn là giấy nhớ, phiếu là dấu vân tay mực; nhãn giữ nguyên.
- **`NepBang`:**
  - Nếp 96dp, tư thế theo màn (`cam-ban-do`, `gop-y`, `goi-loi`, `dua-giay`).
  - «Mình đang thấy» thành bong bóng lời Nếp.
  - testID giữ nguyên.
- **`Members`:**
  - Vai là con dấu.
  - «Đặt làm quản trị» vào hàng mở rộng (giữ nhãn).
  - Đoạn luật 4 dòng vào `NapGiay`.
- **`Friends`:** sửa B8 (nút lồng nút).
- **`HoSoNguoiScreen`:**
  - Đầu trang kiểu hộ chiếu.
  - B3: thêm «Kết bạn», gọi `guiLoiMoi` sẵn có (`POST /friends/requests`).
- **`CaiDatNhom`:** xem trước màu bong bóng; tách «Rời nhóm» ra xa.

### S6: Kỷ niệm và Cá nhân

- **`GroupWallLive`:** ảnh in nghiêng ±1–2° (tất định theo id), washi ở góc, tim là dấu tay mực.
- **`ShareMomentLive`:**
  - Khung instax trống.
  - Chú thích viết trên lề trắng («Ô câu chú thích»).
  - Câu EXIF vẫn hiện.
  - «Thả khoảnh khắc» là dấu.
- **`DangBaiScreen`:**
  - 4 mức người đọc thành 4 phong bì (giữ role `radio`, mỗi cái còn một dòng tóm tắt).
  - Đoạn dài vào `NapGiay`.
- **`DangStoryScreen`, `BaiChiTietScreen`:** polaroid kèm đồng hồ cát; bỏ nợ `Card` (cập nhật test cùng commit).
- **`AlbumLive`:** B4, tách «chưa có kèo» khỏi «kèo chưa tới ngày».
- **`AchievementsLive`:**
  - Huy hiệu là tem có sticker Nếp; tem khoá là tem viền.
  - M8 chạy, danh sách đã thấy lưu bằng `kho.ts`.
  - «Cách tính» vào nắp.
- **Hồ sơ (`HoSoSong` + `Profile.tsx`):**
  - Hộ chiếu (`CoverBand` nhỏ, avatar ảnh, dấu «Tham gia…», số liệu thật).
  - Sửa ngay trên hộ chiếu; nhãn ô giữ nguyên.
- **Sở thích (`Onboarding`):**
  - Bảng sticker `GuGlyph` 56dp, chọn là dán.
  - Ba mức chi là ba phong bì dày mỏng.
- **Cài đặt:** giữ trơn có chủ ý, chỉ đổi mép `Sheet`. Không đặt Nếp ở màn xoá tài khoản.

### S7: Hoàn thiện

- **Vào cửa:** Welcome mở bìa thành lật trang 3D quanh gáy, đường kèo tự vẽ; OTP thành vé đục lỗ («Ô nhập mã»
  giữ nguyên); Login dùng `ONhapMuc`.
- **Chạy lại toàn app:**
  - lượt «bàn đêm» cho theme tối;
  - trải hai trang ở 768/1280;
  - cỡ chữ;
  - Giảm chuyển động;
  - hiệu năng;
  - trợ năng;
  - quét lại B7/B8.
- **Documenter:** viết DESIGN.md v3, sinh `.impeccable/design.json` bằng script.
- **Finish review** do reviewer context mới làm, cộng đọc mù trên ảnh cắt không nhãn.
- **Nhật ký:** `docs/claude/<ngày>/san-khau-giay/README.md`.
- **Hàng đợi** (`docs/team/hang-doi.md`): B1 phía máy chủ, bằng chứng native, iOS.

---

## 6. Thứ tự làm và cách giao

| Lát | Nội dung | Cỡ tương đối |
|---|---|---|
| S0.1 | ADR, token, theme, motion, gate `chu-tren-giay` | 3% |
| S0.2 | Skia + spike go/no-go | 4% |
| S0.3 | Máy sân khấu | 6% |
| S0.4 | Nếp con rối + khoảnh khắc | 7% |
| S0.5 | Primitives + ui-lab + các gate | 4% |
| S1 | Tiền | 14% |
| S2 | Sổ hai người + B1 | 10% |
| S3 | Cửa tạo mới | 13% |
| S4a / S4b | Khám phá/plan + 5 thành phố / 10 thành phố còn lại | 15% |
| S5 | Chat, Nếp, người | 9% |
| S6 | Kỷ niệm, cá nhân | 9% |
| S7 | Hoàn thiện | 6% |

- Mỗi lát con là một commit, push lên `claude/practical-faraday-mswgmv` để dự phòng (container là tạm thời).
- Commit message viết tiếng Việt, ghi *cái gì đổi, vì sao*, kèm số đo của cổng.
- Đầu mỗi lát, merge `origin/main` vào nhánh (merge, không rebase), vì các lane khác vẫn sửa sổ hai người và chat.
- **Giao một lần khi xong S7:**
  - báo cáo cuối;
  - trang ảnh trước/sau (artifact);
  - danh sách việc native Lead phải chạy.

  Chỉ mở PR nếu Lead yêu cầu.

---

## 7. Kiểm chứng

### Chạy được trong container, mỗi lát

```bash
cd apps/mobile && npm test && npx tsc --noEmit && node tools/chep-canvaskit.mjs --kiem
python3 -m pytest services/api/tests/web -q
python3 -m pytest tests/test_repo_guard.py tests/test_screens_reachable_gate.py tests/test_server_routes_called_gate.py tests/test_bang_doi_chieu_mockup.py tests/test_maestro_flows_are_all_reachable.py -q
python3 scripts/check_screens_reachable.py && python3 scripts/check_server_routes_called.py
python3 scripts/repo_guard.py tree HEAD && python3 scripts/repo_guard.py range origin/main HEAD
scripts/gate.sh guard guard-range screens server-routes shared mobile
```

### Test mới

Mỗi test có canary và ít nhất 2 đột biến tự nghĩ, kiểm tương đương trước; mỗi đột biến phải đỏ đúng bước đã dự đoán.

- `san-khau`, `nep-roi`:
  - ranh giới;
  - tất định;
  - tư thế nghỉ khớp `trang`;
  - FK (toạ độ tính trực tiếp) khớp ma trận;
  - không có mực đè nếp gấp cam ở mọi khung của mọi tiết mục;
  - mặt ở khoảnh khắc tiền.
- `khoanh-khac`, `muc-nguoi`, `token-san-khau`, `chu-tren-giay`, `lich-thang`, `ban-xoay`, `ban-tron`, `giay-vat-the`,
  `thanh-pho` (đọc id từ `destinations_vn.py`), `khong-vong-lap-vo-han`, `skia-ranh-gioi`.
- `chuoi-maestro-con-song`: mọi chuỗi trong `.maestro` phải còn tồn tại trong mã hoặc nằm trong danh sách dữ liệu seed.
- `suc-song-man-tao`: màn tạo mới, màn tiền, màn sổ đôi phải render ít nhất một primitive sân khấu hoặc vật thể.
  Bắt đầu bằng danh sách nợ, co dần qua từng lát.
- `test_contrast_floor.py` thêm dòng cho `ONhapMuc`, tab `NapGiay`, lá `ChonNgayLich`, viền `BanXoay`, và mực trên teal.

### Bằng chứng hình ảnh trên stack live seed (cách của QC §9)

- Chuẩn bị: Postgres cục bộ, API cổng 58098, Go core cổng 58099, `npm run seed:rudi`, web export phục vụ tĩnh.
- Runner mới `apps/mobile/tools/xem-san-khau.mjs`:
  - puppeteer-core với Chromium `/opt/pw-browsers`, cờ SwiftShader;
  - đăng nhập OTP 000000, đi bằng `pushState` + `popstate`;
  - ma trận 360/412/768/1280 × sáng/tối × reduced-motion.
- Runner **đỏ** nếu `data-renderer` không phải `skia`. Canary: bật `EXPO_PUBLIC_QA_TAT_SKIA=1` thì runner phải báo `svg`.
- Dùng lại `doChe` của `tools/xem-dock-nep.mjs` để chứng minh:
  - `NepDien` không che chữ;
  - `NepDien` cách `Money` ≥16dp;
  - dock nhường chỗ khi `NepDien` đang hiện.
- Quay khung hình M1–M8 và các cú pop-up (ffmpeg có ở `/opt/pw-browsers/ffmpeg-1011`) để kiểm thứ tự nhịp và kiểm
  Giảm chuyển động.
- **Mở ảnh ra nhìn.** Đọc mù bằng subagent context mới. Ảnh để ở `/tmp`, không commit.

### Việc native của Lead (container không có Android SDK/KVM)

- Dựng lại dev client có Skia (`npx expo prebuild --clean && npx expo run:android`), cài lại.
- `scripts/mobile_native.sh --otp` chạy các flow 01, 12, 22, 24–30, 32, 33, 35, 36, 38, 39, 41–43, 47, 48.
  Canary 09 phải vẫn đỏ.
- Đếm khung chuyển động cho: khay tạo, lật trang bill, M3, pop-up.
- Chụp ảnh: sáng 1.0, tối 1.3, cỡ chữ 2.0, tablet.
- Smoke riêng cho crash Skia (cả dev client cũ phải rơi về SVG).
- Đo `dumpsys gfxinfo` khi cuộn Khám phá và khi kéo trên bàn chia bill.

---

## 8. Rủi ro chính

| Rủi ro | Giảm thiểu |
|---|---|
| CanvasKit ≈2,9MB gzip làm chậm web | SVG vẽ trước rồi chuyển mờ; preload; không có WebGL thì không tải |
| Giới hạn 16 WebGL context, trong khi stack giữ màn cũ | ≤3 canvas mỗi màn; canvas tĩnh huỷ context; màn không focus vẽ SVG |
| Skia làm Android sập, hoặc dev client cũ | Dò module trước; error boundary; tránh filter khi đang animate; smoke native của Lead |
| Hiệu năng máy yếu | Picture cache, LRU path, `BlurMask`, cảm biến 32ms và tắt khi không focus |
| Trợ năng canvas | Mọi chữ là RN; một câu cho mỗi canvas; mọi cử chỉ có đường chạm và đường trợ năng |
| Flow Maestro trôi | Gate kiểm kê chuỗi; sửa flow cùng commit; không đổi tên ảnh |
| Hồi quy tiền | Chỉ đổi trình bày; diff thân `chay`/`lam`/`tao` phải bằng 0; flow 28/29 native |
| Xung đột với lane khác trên `main` | Merge `origin/main` đầu mỗi lát; tách phần trình bày khỏi logic |
| Phạm vi rất lớn | S4 chia 4a/4b; mục kéo dài (màu bìa nhóm, M9, nghiêng máy ở hơn 3 cảnh) mặc định tắt |

## 9. Ngoài phạm vi

- Trang khách web `/g/{token}`: Python là oracle và parity so byte HTML, nên không đổi.
- Sửa B1 phía máy chủ.
- iOS.
- Chạy flow Maestro trên máy thật (việc của Lead).
- Các màn phụ thuộc khoá AI (B6) chỉ đo lại khi có khoá.
