# Audit UI/UX app mobile RuDi: báo cáo

- Ngày: 27/09/2026. Cây đo: `7ea1a7c` (trùng `origin/main` lúc bắt đầu). Nhánh ghi: `claude/busy-cray-vfmt4r`.
- MODE = **AUDIT_ONLY**: không sửa mã app; chỉ thêm tài liệu, ảnh bằng chứng và harness đo.
- protocol_version: không áp dụng (không đụng giao thức v1 hay trang khách).
- Verdict: không có (chưa có reviewer thật; đây là báo cáo phát hiện).
- Trạng thái: **đang làm, checkpoint 1** (xong F00). Mục «Checkpoint» ở cuối là nguồn sự thật về
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

## B. Coverage thực tế (checkpoint 1)

Đếm lấy từ `coverage-matrix.md` (sinh máy). Mọi hàng BLOCKED và NOT_TESTED đều được đếm.

| Phạm vi | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| Tất cả (323 hàng) | 33 | 18 | 180 | 91 | 1 |
| Web | 33 | 18 | 60 | 91 | 1 |
| Android native | 0 | 0 | 60 | 0 | 0 |
| iOS native | 0 | 0 | 60 | 0 | 0 |
| Method RUNTIME-WEB | 33 | 18 | 0 | 91 | 1 |
| Method STATIC | 0 | 0 | 180 | 0 | 0 |

Inventory:
- 12 feature (F00–F11), 60 screen, 36 lớp UI, 25 motion.
- 60 BLOCKED trên web là 60 hàng «cỡ chữ 1.3/2.0», mỗi screen một hàng.

## C. Issues

15 issue sau checkpoint 1, chi tiết và ảnh ở `issues.md`.

| Mức | BUG | UX ISSUE | VISUAL POLISH |
|---|---|---|---|
| P1 | UI-005 | | |
| P2 | UI-003, UI-004, UI-006, UI-011 | UI-002 | |
| P3 | UI-010, UI-013 | UI-001, UI-007, UI-008, UI-009, UI-012, UI-015 | UI-014 |

Điểm cần đọc trước:
- **UI-005 (P1, web + trình đọc màn hình):** đóng khay «Tạo mới» bằng Back trình duyệt để lại
  `aria-hidden="true"` trên cả màn và thanh tab.
- **UI-003 (P2, hệ thống):** `accessibilityState` không tới DOM trên web ở 35 chỗ, nên trạng thái
  chọn/đánh dấu/mở gập vô hình với công nghệ hỗ trợ.

Phát hiện bị loại vì là lỗi của harness, không phải của app:
- «fling không đóng sheet»: CDP giao sự kiện chậm 30–125 ms nên cú vuốt chỉ còn khoảng 200 px/s.
  Với timestamp đúng thì fling đóng ở 1250 px/s và ở lại ở 825 px/s, đúng ngưỡng 900.
- «bảng Nếp không mở»: chạm vào phần dock nằm ngoài màn.
- «chạm Cài đặt mở khay tạo»: chưa cuộn hàng vào tầm nhìn.

Mỗi trường hợp đã sửa trong harness, và giữ ghi chú ở đây để người đọc biết đã được loại trừ.

## D. Thay đổi

Không sửa file nào trong `apps/`, `services/`, `packages/`. Thêm:
- `docs/claude/2026-09-27/mobile-ui-audit/`: tài liệu và ảnh;
- `tests/qa/mobile-ui-audit/`: harness;
- các mục ghim ảnh trong `.repo-guard-allowlist.json`.

## E. Verification

| Lệnh | Kết quả |
|---|---|
| `npm ci` (apps/mobile) | xong |
| `npm run typecheck` | 0 lỗi |
| `npm test` (gồm `build:check`), `CHROME_BIN` trỏ Chromium | 1220 test: 1219 pass, **1 fail có sẵn** trước mọi thay đổi |
| `node tu-kiem.mjs --dot-bien` (harness) | 16/16 xanh; đột biến M1 (bỏ ngưỡng tràn ngang) và M2 (coi mọi nền trong suốt) đỏ **đúng dòng dự đoán** |

Test fail có sẵn:
- `tests/rudi-hanh-trinh-web.test.mjs:76`: «timed out waiting for Lịch trình trên /plan».
- Test này mở bản build-check không backend, trỏ tới host `.invalid`.
- Nghi vấn: màn trả `null` trong lúc chờ khôi phục phiên (xem UI-009). Sẽ đo ở F03.

## F. Giới hạn và rủi ro còn lại

- Không có bằng chứng native nào (A).
- Mọi nhận xét về cỡ chữ lớn, bàn phím và safe area là đọc mã.
- Web khác native ở ba điểm đã thấy trong đợt này:
  - chuyển cảnh stack trên web là cắt thẳng;
  - `Alert.alert` là hàm rỗng;
  - `flex: 0` đổi nghĩa.
- Vì vậy nhận xét gắn nhãn web không tự chuyển sang native.
- Không có nền bản đồ, không có AI thật.

## Checkpoint

- **Đã xong:** F00 (vỏ toàn cục):
  - định tuyến theo phiên, URL lạ;
  - thanh tab và rail ở 9 cấu hình;
  - khay tạo: 7 cách đóng, chạm đúp, ngắt, focus, kích thước ở 6 cấu hình, `/create` lạnh;
  - dock và bảng Nếp;
  - khôi phục phiên chậm;
  - MO01/MO02/MO04 kèm giảm chuyển động.
- **Đã chụp baseline nhưng chưa ghi kết luận:** Welcome, Login, Lời mời (F01).
- **Tiếp theo:** F01 Vào cửa → F02 Khám phá → F03 Plan/Kèo/Hành trình → F04 Tiền → F05 Chat → F06 Nhóm và người →
  F07 Sổ đôi → F08 Kỷ niệm → F09 Hồ sơ/Cài đặt → F10 bảng QA dev → F11 demo → E1–E6.
- **Còn NOT_TESTED:** mọi hàng của F01–F11 trong `coverage-matrix.md`.
