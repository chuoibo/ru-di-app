# Issues mới trên main: audit UI/UX app mobile RuDi, phần sau pipeline

- Cây đo: main `461eabf`, bản web export production trên stack cục bộ thứ hai (Postgres 16, API Python, cửa
  trước Go, cổng khác stack của audit gốc), dữ liệu seed tổng hợp. Chromium 141 headless, giả lập di động như
  audit gốc (xem `report.md` §A).
- File này chỉ ghi issue **mới**, đánh số tiếp audit gốc (UI-001…UI-122 ở
  `docs/claude/2026-09-27/mobile-ui-audit/issues.md`). Kết quả đo lại các issue cũ ở `retest.md`.
- MODE = AUDIT_ONLY: không issue nào được sửa. «Trạng thái sửa» của mọi issue là *chưa sửa*; «Retest» là
  *không áp dụng*.
- Phân loại, mức và phương pháp như audit gốc: BUG · UX ISSUE · VISUAL POLISH; P0–P3; RUNTIME-WEB, STATIC,
  HYPOTHESIS.

## Tóm tắt theo mức

| Mức | Issue |
|---|---|
| P2 | UI-123 |

---

## F00 Vỏ toàn cục

### UI-123 · Web: chuyển tab rồi bấm Back thì rời khỏi app, vì tab đầu nay là Cộng đồng mà app vẫn mở ở Khám phá

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (điều hướng, web) · **P2** |
| Feature / Screen / Layer | F00 · mọi màn tab · L05 thanh tab (`app/(tabs)/_layout.tsx`, `ui/RudiTabBar.tsx`) |
| Nền tảng, cấu hình | web, C1, có phiên và không phiên. Cơ chế không phụ thuộc bề rộng (thanh tab và rail cùng một navigator), nhưng chỉ C1 được đo. Native: Android với `backBehavior` «firstRoute» thì Back hệ thống đưa về tab đầu (Cộng đồng) thay vì tab trước, không thoát app; iOS không có Back (STATIC, chưa đo) |
| Điều kiện | App mở ở Khám phá: đó vẫn là đích sau đăng nhập trên main (`TC-F00-DT-dalat-0`: tới `/explore`). Hoặc mở thẳng bất kỳ tab nào không phải Cộng đồng |
| Tái hiện | 1. Mở app, tới Khám phá. 2. Chạm tab «Lên plan». 3. Bấm Back của trình duyệt (trên Android Chrome là cử chỉ back hệ thống) |
| Expected | Back về tab vừa rời (Khám phá), như ở bản `7ea1a7c`; ít nhất là không rời app |
| Actual | Chuyển tab không thêm mục lịch sử nào: có phiên `/explore#3` → Lên plan `/plan#3`; không phiên `/explore#2` → `/plan#2` (số sau «#» là `history.length`). Back rời app, tới trang trình duyệt mở trước đó (trong harness là trang gắn phiên `/favicon.ico`, hoặc `about:blank`). Chuỗi Khám phá → Lên plan → Tin nhắn cũng không thêm mục nào. Chỉ khi bắt đầu ở tab đầu thì có một mục: Cộng đồng `#3` → Khám phá `#4` → Lên plan `#4`, và Back về Cộng đồng chứ không về Khám phá. Đối chứng bản `7ea1a7c` bằng cùng phép đo: `/explore#3` → Lên plan `#4` → Tin nhắn `#4`, Back về `/explore` |
| Evidence | Không có ảnh: lỗi nằm ở lịch sử trình duyệt, không nằm trên màn hình. Hàng `TC-M-UI-123` (sổ của main); phép đo `kich-ban/tham-do-lich-su-tab.mjs` chạy trên cả hai bản (`report.md` §E) |
| Source | Commit `0ec19fa` thêm `Tabs.Screen name="community"` đứng đầu `app/(tabs)/_layout.tsx`. Bộ định tuyến (bản fork `useLinking` trong `expo-router/build/fork`) chỉ `history.push` khi lịch sử của navigator đang focus dài thêm (`historyDelta > 0`); bằng nhau thì `history.replace`. Tabs mặc định `backBehavior: "firstRoute"`, nên lịch sử tab chỉ giữ [tab đầu, tab hiện tại]: đi giữa hai tab không phải tab đầu thì độ dài không đổi, nên là replace. Ở `7ea1a7c`, tab đầu là Khám phá, trùng với đích sau đăng nhập, nên lần chuyển tab đầu tiên luôn là push |
| Hậu quả | Trên web, và Android Chrome, sau một lần chuyển tab, Back đóng app thay vì về tab trước. Thứ đang soạn dở trên màn đó mất, người dùng phải mở lại app. Đường tái hiện của UI-117 cũng vì vậy mà đổi: Back rời app trước khi tới màn bị khoá (`retest.md`, `TC-R-UI-117-A`/`-B`) |
| Đề xuất sửa | Chọn theo ý đồ sản phẩm: `backBehavior: "history"` cho Tabs, để mỗi lần chuyển tab là một mục lịch sử; hoặc đưa đích sau đăng nhập về tab đầu; hoặc để Khám phá đứng đầu như trước. Sau khi sửa, kiểm lại các flow Maestro có dùng Back |
| Tiêu chí gỡ | Mở app ở Khám phá, chạm «Lên plan», Back: về Khám phá (ít nhất là một màn của app). `tham-do-lich-su-tab.mjs` trên bản sửa cho `history.length` tăng khi chuyển tab |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

---

## Quan sát chưa thành issue (chờ audit feature mới)

Những điều thấy trong lúc retest, thuộc phần main mới đổi. Chưa đủ căn cứ để gọi là lỗi, vì cần đối chiếu ý đồ
thiết kế của đúng feature đó. Mỗi điều được giao cho task audit tương ứng, không tính vào số issue.

| # | Quan sát | Căn cứ | Giao cho |
|---|---|---|---|
| Q1 | Cặp bạn bè (chưa «Một đôi») vừa lập sổ: thân màn sang «Hai người cũng thành một hội», khoảnh khắc bìa sổ M6 không còn diễn | `KhongGianGiay.tsx`: M6 (`vuaMoSo`) chỉ nằm trong nhánh `giay-trong`, nhánh này chỉ tới được khi `batDoi`; runtime: `TC-R-UI-084-B` C1/C9 không thấy khung M6 nào | #26 hai lớp chat |
| Q2 | Máy chủ vẫn cho phác và gửi tờ giấy ở cặp chưa «Một đôi»; chỉ UI ẩn nút | `pairsteps/papers.go` `DraftPaper` không kiểm `CanBatDoi` (đọc mã, chưa gọi API trên cặp như vậy) | #26 hai lớp chat |
| Q3 | Màn bài `/posts/[id]` không còn lối xoá bình luận nào, cho cả người viết lẫn tác giả bài; nhấn giữ bình luận cũng không mở gì. API `DELETE /posts/{id}/comments/{id}` vẫn còn | `TC-R-UI-096` (đổi); `BaiChiTietScreen.tsx` hàng bình luận chỉ có «Thích», «Trả lời» | #21 tường v2 |
| Q4 | Album kèo ghi «đã chia» bằng tổng phân bổ của mọi khoản chi trong nhóm có ngày rơi vào khoảng ngày của kèo, không theo kèo: «Kèo album retest» vừa tạo ghi «đã chia 13.705.678đ» của khoản chi lượt F04. Hai kèo trùng ngày sẽ cùng ghi một khoản | `repo/recap.go`: nối `expenses` theo `on_date BETWEEN outings.starts_on AND outings.ends_on`; bảng `expenses` không có cột kèo. Đọc mã Go, chưa đối chiếu oracle Python | #15 nhật ký chuyến |
| Q5 | Tab Cộng đồng khi chưa đăng nhập có nút «Đăng nhập», trong khi bốn tab demo kia và hai route demo không có (tính vào `TC-R-UI-082-F11`) | `TC-R-UI-082-F11` | #14 Cộng đồng |
