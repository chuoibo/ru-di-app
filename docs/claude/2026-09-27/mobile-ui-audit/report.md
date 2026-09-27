# Audit UI/UX app mobile RuDi: báo cáo

- Ngày: 27/09/2026. Cây đo: `7ea1a7c` (trùng `origin/main` lúc bắt đầu). Nhánh ghi: `claude/busy-cray-vfmt4r`.
- MODE = **AUDIT_ONLY**: không sửa mã app; chỉ thêm tài liệu, ảnh bằng chứng và harness đo.
- protocol_version: không áp dụng (không đụng giao thức v1 hay trang khách).
- Verdict: không có (chưa có reviewer thật; đây là báo cáo phát hiện).
- Trạng thái: **đang làm, checkpoint 3** (xong F00, F01, F02). Mục «Checkpoint» ở cuối là nguồn sự thật
  về phần đã và chưa đo.

Tài liệu đi kèm:
- `inventory.md`: danh mục feature → screen → lớp UI → motion.
- `coverage-matrix.md`: mọi hàng test, sinh từ sổ kết quả, có đếm.
- `issues.md`: từng phát hiện với bước tái hiện và ảnh.
- `evidence-manifest.md`: danh sách ảnh đã commit.
- Harness: `tests/qa/mobile-ui-audit/` (README ở đó).

## A. Phạm vi và môi trường

### Đã chạy thật

| Thứ | Giá trị |
|---|---|
| Nền tảng runtime | **web** (react-native-web), bản **production export** (`expo export --platform web`, không lớp phủ dev) |
| Trình duyệt | Chromium 141 headless, giả lập di động (`isMobile`, `hasTouch`), DPR 2, locale `vi-VN`, giờ `Asia/Ho_Chi_Minh` |
| Vẽ | WebGL qua SwiftShader; mọi ảnh ghi `data-renderer`: sân khấu vẽ bằng **Skia** (không phải SVG dự phòng) |
| Chữ | Chữ thân Roboto qua một file fontconfig riêng, vì mặc định máy chỉ có DejaVu Sans (rộng hơn Roboto/SF). Tiêu đề là Bricolage của app |
| Backend | Postgres 16 cục bộ, API Python, cửa trước Go (`core serve`, 152/159 route phục vụ bằng Go), `MOBILE_AUTH_MODE` vắng nên là `prod`, OTP debug |
| Dữ liệu | `seed:rudi` (Team Đà Lạt: 8 người, 13 tin, 1 kèo 3 ngày, bill 1.280.000đ, 1 đợt thu, 5 kỷ niệm) + chat seed (22 người tổng hợp, nhóm 20 thành viên, 2 DM). Toàn bộ là dữ liệu giả; số điện thoại bị che trên mọi ảnh |
| Phiên | OTP qua API một lần cho mỗi persona, rồi gắn vào trang bằng `POST /sessions/web` (cookie HttpOnly, đúng đường app tự dùng sau khi tải lại). Luồng OTP qua UI được audit riêng ở F01 |
| Gián đoạn | Máy khởi động lại giữa F02. Postgres, API và cửa Go được dựng lại trên đúng thư mục dữ liệu cũ, không seed lại; phiên đã lưu vẫn dùng được. Mọi hàng F02 ghi sau đó chạy trên cùng dữ liệu |

### Ma trận cấu hình

Kích thước là dp (CSS px). Tất cả DPR 2, cảm ứng.

| ID | Viewport | Theme | Vai trò |
|---|---|---|---|
| C1 | 390×844 | sáng | điện thoại chuẩn |
| C2 | 320×640 | sáng | hẹp nhất, dò tràn |
| C3 | 360×800 | tối | Android phổ biến + tối |
| C4 | 375×667 | sáng | iPhone SE 2/3 |
| C5 | 430×932 | sáng | máy lớn |
| C6 | 768×1024 | sáng | tablet medium (rail) |
| C7 | 1024×1366 | tối | tablet expanded (hai pane) |
| C8 | 390×460 | sáng | cửa sổ thấp: proxy cho IME/split-screen, **không** phải bằng chứng về bàn phím |
| C9 | 390×844 | sáng | `prefers-reduced-motion: reduce` |
| B599/B600/B839/B840 | 599–840 | sáng | biên size class của `adaptive.ts` |

Cách lấy mẫu:
- Mọi screen đo baseline ở C1, C2, C3.
- C4, C5: màn danh sách, chi tiết, form.
- C6, C7: tab gốc, cặp danh sách/chi tiết, sheet.
- C8: form, sheet có ô nhập, chat.
- C9: motion inventory.
- Flow rủi ro cao (tiền, tạo kèo, chat, vào cửa, sổ đôi) chạy đủ C1–C9.
- Đây là **lấy mẫu có chủ đích**, không phải mọi tổ hợp.

### Không chạy được, và vì sao

| Hạng mục | Trạng thái | Lý do đo được |
|---|---|---|
| Android native | BLOCKED | Không có `/dev/kvm`, CPU không có vmx/svm, không có Android SDK (proxy từ chối `dl.google.com`) |
| iOS native | BLOCKED | Không có macOS / iOS Simulator |
| Cỡ chữ hệ thống 1.3 / 2.0 | BLOCKED (runtime) | react-native-web cố định `fontScale` = 1.0; chỉ rà mã các nhánh `fontScale`/`chuLon` |
| Bàn phím ảo che nội dung | BLOCKED (runtime) | Chromium headless không có bàn phím ảo; C8 chỉ là proxy cửa sổ thấp; rà mã `KeyboardAvoidingView` |
| Safe area, notch, home indicator | N/A trên web; BLOCKED native | `index.html` không có `viewport-fit=cover`, nên inset luôn 0 |
| Nền bản đồ | BLOCKED | Proxy từ chối `tiles.openfreemap.org`; vẫn đo marker, đường, control, sheet |
| AI thật | BLOCKED | Không có khoá; chỉ đo trạng thái «chưa cấu hình» |
| Trình đọc màn hình thật, haptics, BackHandler, vuốt back iOS | BLOCKED | Chỉ đo cây ARIA trên web |
| Độ mượt / FPS | Không đo | SwiftShader headless không đại diện cho máy; motion chỉ kết luận về hình dạng (đi đâu, dừng đâu, có bị ngắt, giảm chuyển động) |

## B. Coverage thực tế (checkpoint 3)

Đếm lấy từ `coverage-matrix.md` (sinh máy). Mọi hàng BLOCKED và NOT_TESTED đều được đếm.

| Phạm vi | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| Tất cả (387 hàng) | 75 | 51 | 180 | 78 | 3 |
| Web | 75 | 51 | 60 | 78 | 3 |
| Android native | 0 | 0 | 60 | 0 | 0 |
| iOS native | 0 | 0 | 60 | 0 | 0 |
| Method RUNTIME-WEB | 75 | 51 | 0 | 78 | 3 |
| Method STATIC | 0 | 0 | 180 | 0 | 0 |

Theo feature đã đo:

| Feature | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| F00 Vỏ toàn cục | 33 | 17 | 15 | 4 | 1 |
| F01 Vào cửa | 18 | 7 | 15 | 0 | 0 |
| F02 Khám phá | 24 | 26 | 12 | 1 | 2 |

Hàng rút: `coverage-matrix.md` có mục «Hàng đã rút» cho 7 test case có phán quyết sinh từ lỗi của harness.
Sổ gốc giữ nguyên các dòng đó; ma trận chỉ bỏ chúng khỏi bảng và ghi lý do.

Inventory:
- 12 feature (F00–F11), 60 screen, 36 lớp UI, 25 motion.
- 60 BLOCKED trên web là 60 hàng «cỡ chữ 1.3/2.0», mỗi screen một hàng.

## C. Issues

31 issue sau checkpoint 3, chi tiết và ảnh ở `issues.md`.

| Mức | BUG | UX ISSUE | VISUAL POLISH |
|---|---|---|---|
| P1 | UI-005 | | |
| P2 | UI-003, UI-004, UI-006, UI-011, UI-016, UI-022 | UI-002, UI-018, UI-019, UI-021, UI-023, UI-024 | |
| P3 | UI-010, UI-013, UI-027 | UI-001, UI-007, UI-008, UI-009, UI-012, UI-015, UI-017, UI-020, UI-028, UI-029, UI-030 | UI-014, UI-025, UI-026, UI-031 |

Đổi mức: UI-018 từ P3 lên P2 ở checkpoint 3. Nút back của `TopBar` trong kit (37 file màn) cũng không kiểm
`canGoBack()`, và F02 đo lại được ở `/places/[id]`, một màn không có thanh tab nên mở thẳng bằng link thì
trong app không còn lối ra.

Điểm cần đọc trước:
- **UI-005 (P1, web + trình đọc màn hình):** đóng khay «Tạo mới» bằng Back trình duyệt để lại
  `aria-hidden="true"` trên cả màn và thanh tab.
- **UI-003 (P2, hệ thống):** `accessibilityState` không tới DOM trên web ở 35 chỗ, nên trạng thái
  chọn/đánh dấu/mở gập vô hình với công nghệ hỗ trợ.
- **UI-021 (P2, Khám phá):** giá bị cắt ở 8–10 trên 10 hàng, ở mọi bề rộng điện thoại và cả 768. Nguyên nhân
  là thứ tự dữ liệu: bản sửa QA 23/09 cho «phần tử cuối» dòng riêng, nhưng phần tử cuối nay là giờ mở cửa.
- **UI-022 (P2, web):** «Chỉ đường» và dòng địa chỉ là nút chết trên web (`geo:` qua `window.open`), không
  có câu báo. Trên iOS nghi vấn luôn báo «chưa có ứng dụng bản đồ» (chưa đo).
- **UI-024 (P2):** nút ✦ «Hỏi Rủ Đi AI» chỉ thả câu mẫu; danh sách lập tức báo «0 kết quả» trước khi có câu hỏi.

Danh sách quét tĩnh cho UI-003: 20/35 chỗ dùng `accessibilityState` mà không truyền kèm `aria-*` tương ứng.
Mỗi dòng cần xác nhận runtime: `Onboarding.tsx:252` hoá ra dương tính giả (role radio đã có `aria-checked`).
- `ui.tsx:523` (busy)
- `ui/ReorderList.tsx:81` (disabled)
- `ui/CoverButton.tsx:35` (busy)
- `ui/RudiTabBar.tsx:84` (selected, **đã xác nhận runtime**)
- `hanh-trinh/ManHinhHanhTrinh.tsx:175` (expanded) và `:265` (selected)
- `screens/Group.tsx:266` (expanded)
- `screens/Bill.tsx:250` (expanded)
- `screens/chia-bill/ChiaBillLive.tsx:391` (busy), `:467` và `:578` (expanded)
- `screens/hai-nguoi/ChonNguoi.tsx:108` (busy)
- `screens/chat/TheAi.tsx:290` (checked)
- `screens/chat/CaiDatNhom.tsx:154` (selected, checked)
- `screens/chat/SoHen.tsx:235` (expanded)
- `screens/keo/CreateOutingLive.tsx:235` (selected)
- `screens/tuong/BaiChiTietScreen.tsx:222` (selected)
- `screens/explore/DiemDenScreen.tsx:125` (selected)
- `screens/nguoi/DangBaiScreen.tsx:170` (selected)

Phát hiện bị loại vì là lỗi của harness, không phải của app:
- «fling không đóng sheet»: CDP giao sự kiện chậm 30–125 ms nên cú vuốt chỉ còn khoảng 200 px/s.
  Với timestamp đúng thì fling đóng ở 1250 px/s và ở lại ở 825 px/s, đúng ngưỡng 900.
- «bảng Nếp không mở»: chạm vào phần dock nằm ngoài màn.
- «chạm Cài đặt mở khay tạo»: chưa cuộn hàng vào tầm nhìn.

Ở F02, thêm các trường hợp sau (mỗi cái có hàng rút kèm lý do trong `coverage-matrix.md`):
- «503 không có nút Thử lại»: nút có, nhưng tên nằm ở chữ chứ không ở `aria-label`. Còn mẫu chặn
  `/places?` thì trượt, vì máy chưa chọn thành phố gọi `/places` không có query.
- «Rủ một người không có»: tên nút thật là «Rủ <tên> tới đây», và nút chỉ hiện khi có sổ đôi đang mở.
- «AI không trả lời gì»: Enter rơi vào nút ✦ vừa được chạm, nên câu chưa hề được gửi. Chạy lại tách hai ca.
- «Kéo nghiêng sân khấu không động»: Khám phá không nối nghiêng; đổi thành đo cú bật dựng.
- Ảnh ghép có khung đỏ cũ: bản chú thích của lượt trước còn sót khi lượt mới không khoanh gì. `chup` nay xoá
  bản cũ.

Mỗi trường hợp đã sửa trong harness, và giữ ghi chú ở đây để người đọc biết đã được loại trừ.

## D. Thay đổi

Không sửa file nào trong `apps/`, `services/`, `packages/`. Thêm:
- `docs/claude/2026-09-27/mobile-ui-audit/`: tài liệu và ảnh (51 ảnh, 7,52 MiB, ngân sách 20 MiB);
- `tests/qa/mobile-ui-audit/`: harness. Thư viện dùng chung ở `thu-vien/` (trước là `lib/`, xem sự cố 2);
  `kich-ban/f02-phan-xu.mjs` ghi các phán quyết bằng mắt kèm ảnh đã xem;
- các mục ghim ảnh trong `.repo-guard-allowlist.json`.

## E. Verification

| Lệnh | Kết quả |
|---|---|
| `npm ci` (apps/mobile) | xong |
| `npm run typecheck` | 0 lỗi |
| `npm test` (gồm `build:check`), `CHROME_BIN` trỏ Chromium | 1220 test: 1219 pass, **1 fail có sẵn** trước mọi thay đổi |
| `node tu-kiem.mjs --dot-bien` (harness) | 16/16 xanh; đột biến M1 (bỏ ngưỡng tràn ngang) và M2 (coi mọi nền trong suốt) đỏ **đúng dòng dự đoán**. Chạy lại sau khi đổi `lib/` thành `thu-vien/`: vẫn 16/16, M1 và M2 đỏ đúng chỗ |
| Chặn file bị `.gitignore` bỏ qua (bước mới của script commit) | canary: một file trong thư mục `lib/` giả bị liệt kê; identity: 0 file bị bỏ qua ngoài `node_modules/` |

Test fail có sẵn:
- `tests/rudi-hanh-trinh-web.test.mjs:76`: «timed out waiting for Lịch trình trên /plan».
- Test này mở bản build-check không backend, trỏ tới host `.invalid`.
- Nghi vấn: màn trả `null` trong lúc chờ khôi phục phiên (xem UI-009). Sẽ đo ở F03.

### Sự cố quy trình

1. **Checkpoint 2, luật long-number.** `coverage-matrix.md` chứa một toạ độ chưa làm tròn: mười chữ số liền,
   có dấu chấm thập phân sau ba chữ số đầu. Đó đúng là dạng repo guard chặn (con số không chép lại ở đây vì
   cùng lý do). Lệnh commit nối `| tail -1` nên mã thoát của guard bị nuốt, và commit đã được push.
   - Sửa: làm tròn mọi số lẻ trước khi ghi markdown.
   - Commit đã được amend thành `48a0a04` rồi force-push có lease trên nhánh của đợt này.
   - Từ đó mọi commit đi qua một script dừng ở bước guard đầu tiên bị đỏ.
2. **Checkpoint 1–2, harness thiếu thư viện.** `.gitignore` gốc có dòng `lib/` (mẫu Python). Vì vậy 13 file
   `tests/qa/mobile-ui-audit/lib/*.mjs` chưa từng vào git: hai commit `6cac592` và `48a0a04` có kịch bản import
   `../lib/…` không tồn tại trên máy khác.
   - Test `test_moi_nhap_noi_bo_deu_ton_tai` xanh ở máy này, vì nó kiểm file trên đĩa.
   - Đo trên worktree sạch: ở `48a0a04` test đỏ, «48 import khong link duoc»; ở commit sửa, file test
     xanh 43/43.
   - CI chưa chạy, vì nhánh không có PR.
   - Phát hiện ở checkpoint 3. Sửa bằng cách đổi thư mục thành `thu-vien/`, không đụng `.gitignore` gốc.
   - Script commit thêm bước dừng khi có file audit bị ignore.
   - Hai commit cũ giữ nguyên trong lịch sử.

## F. Giới hạn và rủi ro còn lại

- Không có bằng chứng native nào (A).
- Mọi nhận xét về cỡ chữ lớn, bàn phím và safe area là đọc mã.
- Web khác native ở ba điểm đã thấy trong đợt này:
  - chuyển cảnh stack trên web là cắt thẳng;
  - `Alert.alert` là hàm rỗng;
  - `flex: 0` đổi nghĩa.
- Vì vậy nhận xét gắn nhãn web không tự chuyển sang native.
- Không có nền bản đồ, không có AI thật (UI-024 đo trên máy chủ không có khoá AI).
- Thời lượng motion không kết luận (SwiftShader). UI-027 nêu cơ chế; độ dài khoảng trống trên máy thật có thể
  ngắn hơn.

## Checkpoint

- **Đã xong:** F00 (vỏ toàn cục), F01 (vào cửa), F02 (Khám phá).
  - F00:
    - định tuyến theo phiên, URL lạ;
    - thanh tab và rail ở 9 cấu hình;
    - khay tạo: 7 cách đóng, chạm đúp, ngắt, focus, kích thước ở 6 cấu hình, `/create` lạnh;
    - dock và bảng Nếp;
    - khôi phục phiên chậm;
    - MO01/MO02/MO04 kèm giảm chuyển động.
  - F01:
    - Welcome: pager, lật bìa C1/C9, C8;
    - Đăng nhập: back lạnh, số sai, 503, C8;
    - OTP thật cho 2 tài khoản mới: sai mã, đúng mã, Back;
    - Sở thích người mới: chưa đủ, đủ, lưu, ARIA;
    - Lời mời sai.
  - F02:
    - baseline Khám phá C1–C7, Điểm đến C1–C3/C6/C7, chi tiết quán C1–C7, kèm tên 76 ký tự và tên liền;
    - lọc, tìm, tim lưu, cuối danh sách;
    - 503 và thử lại, mất mạng, thành phố rỗng;
    - AI hai bước (chạm ✦, gửi);
    - «Thêm vào kèo», «Chỉ đường», back lạnh;
    - chữ bị cắt đo bằng DOM ở C1–C7;
    - MO12: bật dựng C1/C9, bỏ lọc C1/C9, nhảy bố cục theo từng khung hình.
- **Tiếp theo:** F03 Plan/Kèo/Hành trình → F04 Tiền → F05 Chat → F06 Nhóm và người → F07 Sổ đôi →
  F08 Kỷ niệm → F09 Hồ sơ/Cài đặt → F10 bảng QA dev → F11 demo → E1–E6.
- **Còn NOT_TESTED:**
  - mọi hàng của F03–F11 trong `coverage-matrix.md`;
  - `TC-F02-CHI-TIET-RU`, nút «Rủ <tên> tới đây», cần persona có sổ đôi, chuyển F07;
  - 4 hàng F00 đã ghi ở checkpoint trước.
