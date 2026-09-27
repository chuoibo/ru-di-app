# Audit UI/UX app mobile RuDi: báo cáo

- Ngày: 27/09/2026. Cây đo: `7ea1a7c` (trùng `origin/main` lúc bắt đầu). Nhánh ghi: `claude/busy-cray-vfmt4r`.
- MODE = **AUDIT_ONLY**: không sửa mã app; chỉ thêm tài liệu, ảnh bằng chứng và harness đo.
- protocol_version: không áp dụng (không đụng giao thức v1 hay trang khách).
- Verdict: không có (chưa có reviewer thật; đây là báo cáo phát hiện).
- Trạng thái: **đang làm, checkpoint 5** (xong F00–F04). Mục «Checkpoint» ở cuối là nguồn sự thật về
  phần đã và chưa đo.

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
| Gián đoạn | Máy khởi động lại hai lần: giữa F02, và giữa F04 lúc phiên chờ hạn mức. Mỗi lần Postgres, API và cửa Go được dựng lại trên đúng thư mục dữ liệu cũ, không seed lại; phiên đã lưu vẫn dùng được |
| Dữ liệu biến thể (F03) | `tests/qa/mobile-ui-audit/seed-bien-the.mjs` tạo qua API của app hai kèo giả: tên 96 ký tự với 12 chặng dài (một nhãn 57 ký tự liền), và kèo 2 ngày không chặng. Ca nào ghi vào hai kèo này thì đặt lại chặng sau đó. Việc dọn dẹp chỉ chạy trên DB cục bộ: xoá kèo «Kèo thử…» do ca tạo kèo sinh ra, xoá check-in của kèo biến thể. Không route nào xoá được hai thứ này, và đã kiểm trước: không bảng hay tin chat nào tham chiếu tới chúng |
| Ghi tiền (F04) | Sổ tiền là append-only (trigger chặn sửa, xoá nghĩa vụ), nên mọi lần ghi ở lại trên DB cục bộ. Team Đà Lạt **không bị ghi sổ**: 1 khoản chi, 1 đợt, 0 biên nhận như lúc seed; chỉ có thêm 17 bill nháp (`POST /bills` ở bước 2 → 3, không route nào liệt kê, UI không hiện). Mọi lần ghi đi vào nhóm chat-test 20 người: 3 khoản chi (13.705.678đ và hai khoản «Trà đá» 20.000đ), 1 đợt đã phát (19 link, lưu trong localStorage của context test rồi mất khi đóng), 3 biên nhận «Tiền đã về». Ghi tiền không đăng tin chat (đã đọc mã Go), nên ảnh F05 không bị ảnh hưởng |

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

## B. Coverage thực tế (checkpoint 5)

Đếm lấy từ `coverage-matrix.md` (sinh máy). Mọi hàng BLOCKED và NOT_TESTED đều được đếm.

| Phạm vi | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| Tất cả (538 hàng) | 171 | 126 | 181 | 56 | 4 |
| Web | 171 | 126 | 61 | 56 | 4 |
| Android native | 0 | 0 | 60 | 0 | 0 |
| iOS native | 0 | 0 | 60 | 0 | 0 |
| Method RUNTIME-WEB | 171 | 124 | 1 | 56 | 3 |
| Method STATIC | 0 | 2 | 180 | 0 | 1 |

Theo feature đã đo:

| Feature | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| F00 Vỏ toàn cục | 33 | 17 | 15 | 4 | 1 |
| F01 Vào cửa | 18 | 7 | 15 | 0 | 0 |
| F02 Khám phá | 24 | 26 | 12 | 1 | 2 |
| F03 Plan · Kèo · Hành trình | 56 | 39 | 25 | 0 | 0 |
| F04 Tiền | 40 | 36 | 15 | 0 | 1 |

Một hàng BLOCKED trên web là thật sự không chạy được trong giả lập: giữ ngón tay trên bản đồ (`TC-L11-GIU`). Giả lập
cảm ứng CDP không sinh `contextmenu` từ cú giữ như Chrome Android thật.

Hai hàng STATIC có kết quả FAIL ở F04 là phép tính trên chính hàm bố cục của app (`viTriGhe`, `soDoChuyen` từ
`dist-test`), cùng vị từ với test của repo, cho số người vượt phạm vi test. Chúng đỡ cho hàng runtime cùng issue,
không thay thế nó.

Hàng rút: `coverage-matrix.md` có mục «Hàng đã rút» cho 14 test case có phán quyết sinh từ lỗi của harness.
Sổ gốc giữ nguyên các dòng đó; ma trận chỉ bỏ chúng khỏi bảng và ghi lý do.

Inventory:
- 12 feature (F00–F11), 60 screen, 36 lớp UI, 25 motion.
- 60 BLOCKED trên web là 60 hàng «cỡ chữ 1.3/2.0», mỗi screen một hàng.

## C. Issues

61 issue sau checkpoint 5, chi tiết và ảnh ở `issues.md`.

| Mức | BUG | UX ISSUE | VISUAL POLISH |
|---|---|---|---|
| P1 | UI-005, UI-049 | | |
| P2 | UI-003, UI-004, UI-006, UI-011, UI-016, UI-022, UI-032, UI-035, UI-036, UI-048 | UI-002, UI-018, UI-019, UI-021, UI-023, UI-024, UI-033, UI-034, UI-050, UI-051, UI-052 | |
| P3 | UI-010, UI-013, UI-027, UI-042, UI-046, UI-053 | UI-001, UI-007, UI-008, UI-009, UI-012, UI-015, UI-017, UI-020, UI-028, UI-029, UI-030, UI-037, UI-038, UI-039, UI-041, UI-043, UI-044, UI-056, UI-057, UI-058, UI-059, UI-060 | UI-014, UI-025, UI-026, UI-031, UI-040, UI-045, UI-047, UI-054, UI-055, UI-061 |

Đổi mức: UI-018 từ P3 lên P2 ở checkpoint 3. Nút back của `TopBar` trong kit (37 file màn) cũng không kiểm
`canGoBack()`, và F02 đo lại được ở `/places/[id]`, một màn không có thanh tab nên mở thẳng bằng link thì
trong app không còn lối ra.

Điểm cần đọc trước:
- **UI-005 (P1, web + trình đọc màn hình):** đóng khay «Tạo mới» bằng Back trình duyệt để lại
  `aria-hidden="true"` trên cả màn và thanh tab.
- **UI-049 (P1, web, đợt thu):** «Gửi cho <tên>» không bao giờ gửi được link trên web. Trình duyệt không có Web
  Share thì không có lối nào khác, mà link chỉ có một bản trên máy đã phát. Có Web Share thì link đi được nhưng
  màn vẫn báo «Kiểm tra mạng» ở ngoài khung nhìn và không ghi đã mở khay.
- **UI-048 (P2, chia bill):** số tiền của món bị ellipsis còn «12.3…» ở mọi bề rộng điện thoại khi tên món dài.
- **UI-050 (P2, chia bill):** bàn gán món chồng ghế từ 9 người; ở nhóm 20 người chạm một ghế đổi ghế bên cạnh.
- **UI-051, UI-052 (P2, chia bill):** lý do không đi tiếp và lỗi máy chủ hiện ở đầu trang ngoài màn; lùi về bước 1,
  Back trình duyệt hay tải lại đều mất bill đang gõ.
- **UI-003 (P2, hệ thống):** `accessibilityState` không tới DOM trên web ở 35 chỗ, nên trạng thái
  chọn/đánh dấu/mở gập vô hình với công nghệ hỗ trợ.
- **UI-021 (P2, Khám phá):** giá bị cắt ở 8–10 trên 10 hàng, ở mọi bề rộng điện thoại và cả 768. Nguyên nhân
  là thứ tự dữ liệu: bản sửa QA 23/09 cho «phần tử cuối» dòng riêng, nhưng phần tử cuối nay là giờ mở cửa.
- **UI-022 (P2, web):** «Chỉ đường» và dòng địa chỉ là nút chết trên web (`geo:` qua `window.open`), không
  có câu báo. Trên iOS nghi vấn luôn báo «chưa có ứng dụng bản đồ» (chưa đo).
- **UI-024 (P2):** nút ✦ «Hỏi Rủ Đi AI» chỉ thả câu mẫu; danh sách lập tức báo «0 kết quả» trước khi có câu hỏi.
- **UI-032 (P2, Kèo):** với kèo nhiều ngày, chế độ Bản đồ không hiện chặng nào. Chặng thêm từ Lịch trình
  hay «Thêm vào kèo» không có ngày, và màn chỉ vẽ chặng có ngày. Kèo 1 ngày thì đạt.
- **UI-034 (P2, tạo kèo):** ô ngân sách hiện placeholder «250000» như đã điền; bỏ trống thì «Tạo kèo» từ
  chối bằng một câu nằm ngoài màn.
- **UI-035 (P2):** `/trips/[id]/timeline` hiện lịch trình demo cho người đã đăng nhập, thiếu lớp chặn B5 mà
  hai route anh em có.
- **UI-036 (P2, web):** đổi thứ tự chặng chỉ làm được bằng kéo; tay nắm không nhận focus.
- **UI-003 sửa đề xuất:** checkpoint 3 nêu `HangChang` làm ví dụ đúng, nhưng nó đặt `aria-selected` trên
  `button`, là thuộc tính không hợp lệ (UI-042). Đề xuất đã sửa theo role.

Danh sách quét tĩnh cho UI-003: 20/35 chỗ dùng `accessibilityState` mà không truyền kèm `aria-*` tương ứng.
Mỗi dòng cần xác nhận runtime: `Onboarding.tsx:252` hoá ra dương tính giả (role radio đã có `aria-checked`).
- `ui.tsx:523` (busy)
- `ui/ReorderList.tsx:81` (disabled)
- `ui/CoverButton.tsx:35` (busy)
- `ui/RudiTabBar.tsx:84` (selected, **đã xác nhận runtime**)
- `hanh-trinh/ManHinhHanhTrinh.tsx:175` (expanded) và `:265` (selected)
- `screens/Group.tsx:266` (expanded)
- `screens/Bill.tsx:250` (expanded)
- `screens/chia-bill/ChiaBillLive.tsx:391` (busy), `:467` và `:578` (expanded, **đã xác nhận runtime** ở F04)
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

Ở F03:
- «Tạo kèo không đi đâu»: lượt đầu harness không điền ngân sách. Chính điều này lộ ra bẫy UI-034, được đo
  thành ca riêng.
- «Câu lỗi nhãn chặng không có»: câu thật là «Đặt tên cho chặng, ví dụ Ăn tối.», regex của harness đoán sai.
- «Đóng sheet Gắn quán để sót lớp»: lưới chạm «trước» được chụp trước khi cuộn chặng vào tầm nhìn.
- «Lịch tháng không khép»: phép đếm ô ngày tính cả hai lá ngày.
- «Nhãn chuỗi liền bị che»: tín hiệu che là hàng đang cuộn dưới đầu màn cố định; tiêu chí quá chặt.
- «Kéo nghiêng / giữ ngón tay trên bản đồ không làm gì»: giả lập cảm ứng không sinh `contextmenu`; ghi BLOCKED.
- Giả định sai trong seed: thay lịch trình không đổi id chặng, nên check-in không tự mất. Đã đo lại và sửa.

Ở F04:
- «Nếp M2 nằm trọn trong màn»: `scrollIntoView` của harness cuộn ngang cả khung `overflow: hidden` (tiêu đề bị đẩy
  tới x −27 ở C1, −97 ở C2), nên lượt đầu đo ra PASS giả. Không ngón tay nào cuộn ngang được khung đó. Sửa: mọi
  lần cuộn của harness trả lại cuộn ngang của khung không cuộn được (`thu-vien/cu-chi.mjs` `tamCua` và
  `f04-tien.mjs`); `tu-kiem` vẫn 16/16. Đo lại: FAIL (UI-055). Các phát hiện cũ dựa trên vị trí ngang (UI-004
  vạch rail, UI-047 đầu màn tablet) đã có nguyên nhân trong mã và ảnh không bị lệch, nên không bị ảnh hưởng.
- «Đọc ảnh bill không ra câu nào»: lượt đầu chụp sau 4 s khi nút còn quay. Chạy lại chờ tới câu: máy chủ trả 503
  sau 91 ms, câu hiện sau 286 ms.
- `fullPage` không chụp được phần cuộn: trang cuộn trong ScrollView của react-native-web, không phải document. Ảnh
  số tiền bị cắt được chụp bằng cách đánh dấu phần tử rồi cuộn tới nó.
- «M3 và M4 nhảy tư thế ở C9»: tiêu chí «một ảnh» quá chặt. Xem ảnh đầu và cuối thì cùng tư thế; lần đổi pixel duy
  nhất là SVG nhường Skia. Phán quyết bằng mắt: đạt.

Mỗi trường hợp đã sửa trong harness, và giữ ghi chú ở đây để người đọc biết đã được loại trừ.

## D. Thay đổi

Không sửa file nào trong `apps/`, `services/`, `packages/`. Thêm:
- `docs/claude/2026-09-27/mobile-ui-audit/`: tài liệu và ảnh (86 ảnh, 11,35 MiB, ngân sách 20 MiB);
- `tests/qa/mobile-ui-audit/`: harness. Thư viện dùng chung ở `thu-vien/` (trước là `lib/`, xem sự cố 2);
  `kich-ban/f02-phan-xu.mjs`, `f03-phan-xu.mjs`, `f04-phan-xu.mjs` ghi các phán quyết bằng mắt kèm ảnh đã xem;
  `seed-bien-the.mjs` tạo dữ liệu biến thể qua API;
- các mục ghim ảnh trong `.repo-guard-allowlist.json`.

## E. Verification

| Lệnh | Kết quả |
|---|---|
| `npm ci` (apps/mobile) | xong |
| `npm run typecheck` | 0 lỗi |
| `npm test` (gồm `build:check`), `CHROME_BIN` trỏ Chromium | 1220 test: 1219 pass, **1 fail có sẵn** trước mọi thay đổi |
| `node tu-kiem.mjs --dot-bien` (harness) | 16/16 xanh; đột biến M1 (bỏ ngưỡng tràn ngang) và M2 (coi mọi nền trong suốt) đỏ **đúng dòng dự đoán**. Chạy lại sau khi đổi `lib/` thành `thu-vien/`: vẫn 16/16, M1 và M2 đỏ đúng chỗ |
| `node tu-kiem.mjs` sau khi vá `tamCua` (F04) | 16/16 xanh |
| Chặn file bị `.gitignore` bỏ qua (bước mới của script commit) | canary: một file trong thư mục `lib/` giả bị liệt kê; identity: 0 file bị bỏ qua ngoài `node_modules/` |

Test fail có sẵn:
- `tests/rudi-hanh-trinh-web.test.mjs:76`: «timed out waiting for Lịch trình trên /plan».
- Test này mở bản build-check không backend, trỏ tới host `.invalid`.
- Đã đo ở F03. Kết luận: không tái hiện như lỗi màn.
  - Chạy lại riêng test: vẫn đỏ, hết 25 s.
  - Cùng bản build-check, cùng Chromium, mở bằng Playwright: `/plan` (demo khi chưa đăng nhập) hiện
    «Lịch trình» sau 660 ms với cờ `--disable-gpu` như test, và 803 ms với SwiftShader.
  - Request `.invalid` hỏng ngay (ERR_NAME_NOT_RESOLVED ở 553 ms).
  - Bản E1 khi chặn hết API cũng hiện sau 654 ms.
  - Nguyên nhân nằm trong harness `chrome-cdp` của test, chưa khoanh. Giả thuyết cũ (UI-009) không đứng vững
    cho ca này.

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
- Chia sẻ trên web (UI-049) đo bằng `navigator.share` giả lập hai kiểu (xong, người dùng đóng); khay chia sẻ thật
  của Android và iOS chưa chạy. Đọc bill từ ảnh chỉ đo nhánh máy chủ chưa có khoá AI.

## Checkpoint

- **Đã xong:** F00 (vỏ toàn cục), F01 (vào cửa), F02 (Khám phá), F03 (Plan · Kèo · Hành trình), F04 (Tiền).
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
  - F03:
    - baseline 8 màn ở C1–C3, danh sách/form/kèo ở C4–C7, kèo 12 chặng và kèo rỗng;
    - Nếp ở màn kèo; công tắc Lịch trình/Bản đồ và việc giữ chế độ;
    - Bản đồ: ngày trống ở 7 cấu hình, mốc theo ngày ở kèo 1 ngày và 3 ngày;
    - sheet «Chặng mới»: 7 cách đóng, focus, chạm đúp, nhãn rỗng, gửi thật, C8/C2/C6;
    - gắn quán, kéo đổi thứ tự (C1, C9) và lối bàn phím, check-in;
    - tạo kèo: thiếu tên, bẫy ngân sách, chip, lịch tháng, tạo thật và M5 (C1, C9), C8;
    - thêm quán vào kèo, route thiếu tham số, 3 route demo, 503, offline, back lạnh;
    - các lớp L09–L15: sheet sửa ngày, điểm hẹn, popup cụm, trang ngày gập, lá lịch, mặt quay giờ;
    - đầu màn ở tablet.
  - F04:
    - baseline 5 màn tiền ở C1–C3 và C4–C7, màn gán món demo khi chưa đăng nhập;
    - chia bill 5 bước với món tên dài và tiền 8 chữ số ở C1–C5, tablet C6/C7, C8;
    - lý do chặn ở bước 2 và 3, lỗi 503 khi tạo bill, lùi trong luồng, Back trình duyệt, tải lại;
    - bàn gán món: chạm, kéo, phím, nhóm 20 người ở C1/C2/C6, tính bố cục 2–20 người;
    - ảnh bill: huỷ bộ chọn, xem trước, máy chủ không đọc được, Nếp M2 ở C1–C3/C6;
    - ghi sổ chạm đúp, Nếp M3 và dấu; quyết toán 20 người; đợt thu: mở, hỏi hai bước, phát, chia sẻ web ba kiểu,
      tiền về và Nếp M4; «Tạo đợt thu» khi không còn khoản ngoài đợt;
    - 503 và thử lại ở quyết toán, đợt thu, tài chính; tài chính mất mạng; back lạnh 4 màn;
    - mép Nếp ở màn tiền, khoảng cách Nếp tới số tiền khi cuộn; MO10, MO13 (M3/M4), MO14, MO15 ở C1/C9.
- **Tiếp theo:** F05 Chat → F06 Nhóm và người → F07 Sổ đôi → F08 Kỷ niệm → F09 Hồ sơ/Cài đặt →
  F10 bảng QA dev → F11 demo → E1–E6.
- **Còn NOT_TESTED:**
  - mọi hàng của F05–F11 trong `coverage-matrix.md`;
  - `TC-F02-CHI-TIET-RU`, nút «Rủ <tên> tới đây», cần persona có sổ đôi, chuyển F07;
  - 4 hàng F00 đã ghi ở checkpoint trước.
