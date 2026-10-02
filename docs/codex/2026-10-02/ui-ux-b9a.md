# B9a · Trang cuối của cuộc đi

Ngày 02/10/2026 · gốc `95755a71` · nhánh `codex/ui-ux-b9a`.
Tiếp nối session Claude `c97c4813-340b-485b-a30b-32b6828ad498` sau B7; B8 đang dở
ở worktree chính được giữ nguyên. Không sửa bảng tiến độ hay report của Claude.

## Brief và hướng thiết kế

Người vừa trở về muốn giữ chuyện của cuộc đi. Trang cuối cần nói đúng ai được khép,
khi nào khép được và bản viết đã lưu chưa. Operate cho quyết định/quyền; Experience
cho bìa và trang sổ. Giữ thế giới giấy, mực, coral; không thêm token hay dependency.

- THESIS: trang cuối phản ánh trạng thái thật; một quyết định ở mỗi bước.
- OWN-WORLD: trang giấy, nét kẻ mảnh, chữ Bricolage, con dấu chỉ dành cho hành động khả thi.
- STORY: nhận ra cuộc hẹn → khép khi được phép → tự chọn chất liệu → xem/sửa → chọn người xem → lưu.
- FIRST VIEWPORT: đầu màn ổn định; lời dẫn theo trạng thái; tên cuộc đi cùng ngày bắt đầu/ngày về; quyết định hoặc lối quay về.
- FORM: kế thừa seed `c8e88116` của UI v3 trong DESIGN.md; code-led, không có comp mới; tinh chỉnh trong hệ hiện có, không thay thế nhận diện; quyền xem là đoạn cuối có kẻ mảnh, hành động lưu và lỗi ở chân màn, không card lồng card.
- FINISH: xem bản render, detector và reviewer ngữ cảnh mới; ghi giới hạn thiết bị, không tự nhận QA chấp nhận.

## Thay đổi

| ID | Nhóm | Hành vi | Trạng thái |
|---|---|---|---|
| UI-150 | BUG_FIX, UX_IMPROVEMENT | Lỗi lưu cạnh nút, giữ bản đang soạn; lỗi đọc và lỗi thao tác tách riêng | READY_FOR_QA |
| UI-151 | BUG_FIX | GET `can_end` xét ngày Việt Nam giống POST; không đổi shape, schema hoặc quyền | READY_FOR_QA |
| UI-152 | UX_IMPROVEMENT | Không có bộ chọn bấm được khi chưa được khép; câu chờ và lối về cuộc hẹn | READY_FOR_QA |
| UI-154 | UX_IMPROVEMENT | Heading của trang tự xếp là ngày đọc được; ngày nguồn vẫn ISO, heading tự đặt không bị viết lại | READY_FOR_QA |
| B9A-D01 | VISUAL_UPGRADE | Thay hình ba chặng trang trí bằng ngày thật, hierarchy theo trạng thái | READY_FOR_QA |
| B9A-D02 | UX_IMPROVEMENT, VISUAL_UPGRADE | Quyền xem sau phần xem/sửa; hành động lưu và lỗi giữ trong chân màn kể cả Android; header cùng cột form | READY_FOR_QA |
| B9A-D03 | BUG_FIX | Khoá request đồng bộ khi bấm nhanh; không remount toàn màn khi đổi chế độ sửa | READY_FOR_QA |

## Retest

- Kèo tương lai: không CTA khép, không bộ chọn, không thử lại vô ích; API GET false,
  POST của người tổ chức vẫn 409 `outing_not_started`, thành viên vẫn 403.
- Kèo hôm nay/đã qua: quyền người tổ chức, cặp đôi đồng thuận và quản trị thay thế không đổi.
- Cuộn cuối bản dài, lưu trả 503: câu lỗi ở vùng lưu, lời/ảnh còn nguyên; thử lại thành công.
- Lưu bị xung đột revision: không gửi lại bản cũ bằng nút «Thử lại»; lối đối chiếu vẫn có.
- Tự xếp ảnh ngày 29/09: ô tên trang, màn đọc và bài Cộng đồng cùng «Ngày 29/09/2026».
- Mở/đóng sheet, mở sửa rồi xem lại, bàn phím, chữ lớn, Reduced Motion và 320dp.

B9a không giải quyết bảo toàn nháp khi rời màn, viewer ảnh hoặc phần còn lại của B9.
## Kiểm chứng thực tế

- Android native, dev client Expo SDK57 trên AVD `rudi-diary-review`, `emulator-5600`;
  tái dùng máy ảo đang rảnh, không dựng VM thứ hai. Bundle từ worktree B9a, API Go và
  PostgreSQL loopback riêng; baseline dựng từ worktree sạch `95755a71`, không từ B8 dở.
- Phone 411dp/chữ100%/sáng: chờ cuộc đi tương lai, tự xếp trang, lỗi503 cạnh lưu,
  mở sửa và bàn phím, thử lại thành công. Phone 320dp/chữ130%/tối/animator scale0:
  lời và ngày reflow, lỗi và cả hai hành động còn thấy; flow `_diary-save-failure.yaml`
  đã chạy đủ trên cả hai kích thước. Tablet 768dp/chữ100%/sáng: cột form và header.
  Hai kích thước phụ là override display trên cùng AVD, không phải phần cứng/tablet riêng.
- Fault proxy chỉ dùng trong harness ngoài repo. Double tap trong request trễ1300ms
  tạo đúng **1** PUT. Bản đã sửa có marker tổng hợp còn nguyên sau503; retry gửi
  **cùng document/audience/revision**. PostgreSQL xác nhận revision1, private,
  document đúng bản gửi. Không gọi AI trong flow này.
- Mobile: typecheck, web export và toàn bộ **1459 test PASS, 0 fail, 0 skip**.
  PostgreSQL thật: **13 test PASS**, có sentinel, không skip, gồm GET tương lai/
  hôm nay/đã qua, member, couple, admin, diary và sources. Go unit/vet đạt.
- Harness Go cùng mã test: identity xanh; canary `outingStarted` luôn true đỏ ở
  `TestOutingStartedUsesVietnamCalendar`; mutant UTC thay ngày Việt Nam đỏ ở ranh
  17:00Z; mutant trả heading ISO đỏ ở `TestManualPageHeadingIsReadableWithoutChangingSource`.
  Cả hai mutant không tương đương: có input phản ví dụ nêu trong test. Mã được trả
  nguyên lại sau mỗi phép thử. Cổng ở SHA sạch được ghi riêng bên dưới khi hoàn tất.
- Impeccable: static TSX detector không có finding chính (`[]`), đây là regex,
  **không phải** contrast đo trên native. Reviewer ngữ cảnh mới đã mở sáu ảnh,
  không yêu cầu sửa visual; disposition ban đầu `fix` vì DESIGN.md còn mô tả
  remount/vị trí lưu cũ. Documenter sửa riêng đoạn diary; verdict cuối **`ship`**
  cho fix tài liệu đó, không phải QA độc lập hay phê duyệt release.

## Evidence

Ảnh PNG gốc từ `adb exec-out screencap`, không sửa nội dung. Toàn bộ tiêu đề,
caption và ảnh sọc là dữ liệu tổng hợp. Hai ảnh trước/sau cùng phone, theme, dữ
liệu và trạng thái request; UI-150 có vị trí cuộn khác vì hành động mới nằm cố định
ở chân màn, không dùng sự khác biệt này làm phép so pixel.

| Kiểm tra | Trước | Sau |
|---|---|---|
| UI-151/152 · cuộc đi tương lai | [Trước](evidence/native-before-future.png) | [Sau](evidence/native-after-future.png) |
| UI-150 · lưu503 | [Trước: lỗi ngoài vùng nút](evidence/native-before-save-error.png) | [Sau: lỗi và nút chân màn](evidence/native-after-save-error.png) |
| 320dp, chữ130%, tối | — | [Trang cuối](evidence/native-after-320-font130-dark.png), [Lưu503](evidence/native-after-save-320-font130-dark.png) |
| Tablet768dp | — | [Cột form](evidence/native-after-tablet.png) |
| Bàn phím | — | [Ô tên sổ và nút lưu](evidence/native-after-keyboard-phone.png) |
| Lưu thành công | — | [Bản riêng tư](evidence/native-after-success.png) |

## Giới hạn và bàn giao

- Chưa QA_ACCEPTED. QA retest các ca ở trên, nhất là xung đột revision, sheet đóng/
  mở nhanh, tải ảnh lỗi, Back khi đang lưu và danh sách24 trang. Những ca này chưa
  được xác nhận bằng runtime B9a; không lấy unit test thay bằng chứng thao tác.
- Không có iOS/device thật, release APK, đo FPS/frame time hoặc TalkBack audit.
  APK dev client được tái dùng; chưa có fingerprint native để chứng minh APK khớp
  toàn bộ manifest. Screenshot không chứng minh độ mượt. Web được chạy ban đầu,
  native là evidence cuối của thay đổi này; không suy ra coverage mọi viewport web.
- `make gate` toàn repo đã được gọi, nhưng chưa thể coi là xanh: chặng native chung
  thiếu JAVA_HOME trong invocation đó và fingerprint APK; các flow toàn app không
  là bằng chứng B9a. Các flow B9a riêng dùng JDK cấu hình đúng đã chạy đạt. Báo cáo
  cổng chọn lọc/SHA sạch và phần còn lại được ghi ở mục cuối; không xóa test/assertion.
- Không sửa báo cáo QA gốc, docs/claude, nguồn B8, business rule/API shape hoặc token.
  Chỉ cập nhật đoạn diary trong DESIGN.md cho hành vi đã triển khai. Nhánh riêng
  cần được tích hợp sau khi B8 được tiếp tục; chưa đặt lên main đang có thay đổi dở.

