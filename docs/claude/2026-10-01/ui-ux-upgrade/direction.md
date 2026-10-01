# Hướng thiết kế: nâng cấp UI/UX app mobile từ audit QA PR #663

- Ngày: 01/10/2026. Cây gốc: `main` `d95edb4`. Đầu vào: PR #663 (`claude/busy-cray-vfmt4r`, head `3bd112e5`):
  - audit gốc `docs/claude/2026-09-27/mobile-ui-audit/` (122 issue);
  - retest + feature mới `docs/claude/2026-09-29/mobile-ui-audit-main/` (45 issue mới).
- Vai: thiết kế và triển khai. Không phải QA: mọi trạng thái ở đây là tự kiểm, QA retest độc lập ở `qa-handoff.md`.
- Hệ thiết kế hiện hành: `DESIGN.md` v3 «Sân khấu giấy» (ADR-0037/0038) và `PRODUCT.md`. Đợt này **tinh chỉnh**, không
  thay thế thế giới đó. Mọi thay đổi hệ được ghi lại vào DESIGN.md ở cuối đợt.

## Người dùng đang làm gì, app nên cho cảm giác gì

Rủ Đi là một cuốn sổ chuyến đi cả hội cùng viết: **Khám phá → thống nhất kế hoạch → cùng đi → chia tiền → giữ kỷ
niệm**. Người tổ chức mở app với một tay, thường đang đứng dậy ra về; người được rủ hay mở link. App phải:

- **Gợi hứng** ở Khám phá (sân khấu thành phố, bưu thiếp), nhưng không bắt chờ.
- **Rõ ràng, dễ quyết** ở Lên plan và các bước đồng ý: một quyết định mỗi màn.
- **Nhanh và to** khi đang đi: một hành động chính, chữ đọc được dưới đèn quán.
- **Chính xác, đáng tin** ở màn tiền: mỗi số nói nó là gì; không trang trí quanh số; tiền không bao giờ bị cắt.
- **Giàu cảm xúc hơn** ở kỷ niệm, sổ chuyến đi, hồ sơ kể chuyện (ảnh in, lật trang, con dấu).
- **Quen tay** ở chat: chuẩn nhắn tin mà người dùng đã thuộc (người yêu cầu chọn Messenger làm tham chiếu); giấy chỉ là
  chất liệu, không cản việc gõ.

## Giữ nguyên (bản sắc đã có)

- Giấy, mực, coral; Bricolage Grotesque cho tiêu đề và số; body system.
- Ba tông mang nghĩa: cam = lời rủ/hành động chính, teal = tiền, tím = AI. Một tông dẫn mỗi màn.
- Con dấu = trạng thái đã đúng; nét chì đứt = nháp; đường mực liền = kế hoạch; ảnh in có xuất xứ.
- Hàng nằm trên trang, ngăn bằng kẻ tóc; không thẻ lồng thẻ.
- Nếp là bạn đồng hành, không chạm số, không đứng cạnh lỗi.
- Bốn bậc motion 100/200/300/550 ms; Reduce Motion cắt thẳng tới khung cuối.
- Đích bấm 48dp; body 17/24; caption 13/18.
- **Không toast, không modal lỗi** (DESIGN.md). Mọi phản hồi là chữ tại chỗ.

## Chỗ hệ chưa có ngữ pháp (những gì QA thật sự phơi ra)

Phần lớn 167 issue không phải lỗi rời: chúng là chỗ kit chưa có câu trả lời, nên mỗi màn tự xoay xở theo một kiểu.

| # | Thiếu | Hậu quả QA đo được | Câu trả lời của đợt này |
|---|---|---|---|
| 1 | Phản hồi tại chỗ ngón tay vừa chạm | Câu lỗi ở y −2855, −1140, 1124…; ~50 câu `warn` tự viết, 10 chỗ `role=alert` màu `accent` | Primitive **`CauTaiCho`**: một câu `warn` neo ngay dưới control vừa thất bại hoặc trong chân CTA dính đáy, `aria-live`, tự vào khung nhìn, biến mất khi thao tác lại thành công. Luật: lỗi tải không bao giờ thay dữ liệu đã có |
| 2 | Câu lỗi theo nguyên nhân thật | 404 → «Cập nhật app», 409 → «Lần bấm trước chưa chạy xong», 4xx → «lỗi của app», 503 = mất mạng | Đọc `code` trước mã HTTP; câu nói hỏng gì, vì sao (khi biết), làm gì tiếp. Lỗi vĩnh viễn không mời «Thử lại» |
| 3 | Vòng đời trọn của lớp phủ | Back trình duyệt rời màn hoặc để lại `inert`; chạm đúp mở rồi đóng; khép không mờ; 92–96% ở cửa sổ thấp; trải 768–1024 ở tablet; nằm dưới dải ghim | **Sheet v2**: một hợp đồng mở/đóng cho mọi sheet và khay |
| 4 | Đường lui chắc chắn | «Quay lại» chết khi mở bằng link (48 file TopBar); đổi tab thay lịch sử; đăng nhập xong không về link; rời màn mất nháp | `luiVe(router, cha)` một nguồn; tab giữ lịch sử; `?tiep=`; focus vào tiêu đề màn; hỏi trước khi bỏ nháp |
| 5 | Chữ xếp lại thay vì bị cắt | Giá, tiền «12.3…», «(trả)», nhãn nút, tên trên bìa | Tiền và giá không bao giờ ellipsis; meta xuống dòng theo cụm «·»; tên nhường trước; nút xếp chồng khi hẹp |
| 6 | Cột nội dung ở tablet | Danh sách, sheet, tờ giấy trải 720–1024dp; header lệch cột | Cột đọc của `RudiScreen` theo `adaptive.ts`; sheet có bề rộng tối đa |
| 7 | Web ngang native ở chỗ chạm nhiều | Ô soạn chat, link dài, pager, viewer ảnh, chia sẻ, chỉ đường, `Alert` | Shim web một chỗ: xác nhận, chia sẻ-hoặc-chép, URL bản đồ, pager theo `onScroll`, ô soạn tự cao |
| 8 | Trạng thái cho công nghệ hỗ trợ | `accessibilityState` không tới DOM ở ~45 chỗ | `trangThaiA11y(role, state)` sinh `aria-*`; `tablist`; Space cho ô chọn |
| 9 | Kiểu nút phá huỷ | «Xoá cuốn sổ» là nút cam đặc; «Bỏ bản phác» giống «Tuần này nghỉ» | Tông `warn` cho `RudiButton` (outline/ghost); nút tắt bắt buộc có lý do (ADR-0038) |
| 10 | Lối vào «Tạo mới» khi có 5 tab | FAB biến mất; khay chỉ mở được từ Lên plan | **Con dấu Tạo mới** trên mọi tab (dưới đây) |

## Con dấu «Tạo mới» trên mọi tab

Yêu cầu của người dùng (01/10): mọi tab đều có «+», đẹp, linh hoạt, thông minh, responsive.

- **Vật:** một con dấu coral tròn (ngôn ngữ `StampButton`), glyph «+», do `RudiTabBar` vẽ như lớp phủ chứ không chiếm cột.
  Năm tab giữ nguyên bề rộng.
  - Điện thoại: góc phải, vắt qua mép trên thanh tab, như con dấu vừa đóng lên mép trang.
  - Rail (≥600dp): đầu rail.
- **Khay mọc ra từ con dấu:** gốc biến hình tại con dấu; «+» xoay thành «×» trong pha mở; Reduce Motion cắt thẳng.
- **Theo ngữ cảnh, không giấu gì:** thứ tự khay đổi theo tab đang đứng; hàng đầu ghi «Hợp với chỗ bạn đang đứng».

  | Tab | Lên đầu khay |
  |---|---|
  | Cộng đồng | «Viết bài» (thay nút soạn vuông ở header: còn một lối) |
  | Khám phá | «Tạo kèo» |
  | Lên plan | «Tạo kèo», «Chia hoá đơn» |
  | Tin nhắn | «Tạo nhóm», «Đăng story» |
  | Cá nhân | «Đăng kỷ niệm», «Đăng story» |

- **Biết nhường:** cuộn xuống thì thu nhỏ và tụt nửa vào thanh tab, dừng hoặc cuộn lên thì hiện lại; ẩn khi bàn phím mở
  hay có sheet; không đè dock Nếp (dock ở giữa mép phải); nội dung có đệm đáy đủ cho hàng cuối.
- **Trợ năng:** nhãn «Tạo mới», `aria-haspopup=dialog`, `aria-expanded`; focus trả về con dấu khi khay đóng.
- **Spike trước khi chốt:** (a) con dấu vắt mép thanh; (b) cột giữa nhô lên của thanh 6 cột. Chọn bằng ảnh ở
  320/390/768/1024 và Android; ghi lựa chọn vào đây.

## Motion: mỗi chuyển động phải nói được một điều

- Sheet: vào bằng lò xo; ra bằng mờ và trượt `standard` 200 ms tới hết; nền không nhận chạm trong pha mở; Back và Esc
  đóng; chạm đúp không mở rồi đóng.
- Khay Tạo mới: mọc ra từ con dấu.
- Viewer ảnh: mờ vào và mờ ra cân xứng.
- Sân khấu Khám phá: giữ chỗ trước để danh sách không nhảy; không dựng lại khi bỏ lọc.
- Reduce Motion: khung cuối ngay, không bao giờ một khung trống.
- Không thêm độ trễ giả; không chuyển động nào tranh với cuộn, bàn phím hay cử chỉ hệ thống.

## Mỗi chặng một sắc thái, cùng một app

| Chặng | Mode | Được phép |
|---|---|---|
| Khám phá | Experience nhẹ | sân khấu, bưu thiếp, một lần bật dựng |
| Lên plan, kèo | Operate | vé, lá lịch; một quyết định mỗi màn |
| Trong chuyến | Operate | chữ to, một hành động, con dấu «ĐÃ TỚI» |
| Tiền | Operate nghiêm | sổ kẻ dòng, số không animate trước domain state, mỗi số một câu nói nó là gì |
| Chat | Operate quen tay | bong bóng chuẩn, giấy chỉ ở nền |
| Kỷ niệm, sổ chuyến đi, hồ sơ | Experience | ảnh in nghiêng, lật trang, con dấu, Nếp diễn đúng khoảnh khắc |

## Không làm

- Không thêm toast, modal lỗi, confetti, gradient, glassmorphism, bóng dày, thẻ lồng thẻ.
- Không đổi token màu, font, bán kính nếu không có lý do đo được.
- Không đổi luật tiền, allocator, sổ cái (UI-149 chỉ là đề xuất ADR).
- Không đổi kiến trúc điều hướng ngoài phạm vi trên; không thêm feature ngoài report.
