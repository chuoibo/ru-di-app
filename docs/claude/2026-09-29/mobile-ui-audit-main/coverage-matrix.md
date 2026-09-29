# Coverage matrix: audit UI/UX app mobile RuDi

Sinh bởi `tests/qa/mobile-ui-audit/tong-hop.mjs` từ sổ `results.jsonl`; không sửa tay. Mỗi hàng là một
(test case × nền tảng × nhóm cấu hình). Cấu hình C1–C9 ở `report.md` §A. Method: RUNTIME-WEB là đã chạy
trên bản web trong Chromium; STATIC là chỉ đọc mã; HYPOTHESIS là nghi vấn chưa kiểm chứng. PASS chỉ có
ở hàng RUNTIME-WEB.

## Đếm

| Phạm vi | Đếm |
|---|---|
| Tất cả (173 hàng) | PASS 20 · FAIL 68 · BLOCKED 1 · NOT_TESTED 84 · NOT_APPLICABLE 0 |
| Web (Chromium) | PASS 20 · FAIL 68 · BLOCKED 1 · NOT_TESTED 84 · NOT_APPLICABLE 0 |
| Android native | PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0 |
| iOS native | PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0 |
| Method RUNTIME-WEB | PASS 20 · FAIL 68 · BLOCKED 1 · NOT_TESTED 84 · NOT_APPLICABLE 0 |
| Method STATIC | PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0 |
| Method HYPOTHESIS | PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0 |

## E1

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-122 | E1 | - | - | - | chưa đo lại trên main | web | Cùng các bước: thanh đầu đổi sang số mới trong vài giây sau tin đầu của người vừa vào, không cần rời chat | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-122 |

## E2

Đếm: PASS 0 · FAIL 1 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-118 | E2 | - | - | - | chưa đo lại trên main | web | Tạo kèo từ chat rồi Back: chat có tên kèo và lối mở nó, ở máy của người tạo và của người khác trong nhóm | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-118 |
| TC-R-UI-119 | E2 | E2 | - | nhóm chat-test, kèo «Kèo album retest» có chặng «Lưng Chừng Cafe»: có; check-in trên máy chủ: 1 | «Tôi đã tới» ở chặng; mở album và tường của nhóm | web C1 | sau «Tôi đã tới» ở một chặng: album của kèo ghi ít nhất «1 chỗ đã tới», hoặc tường có dấu của lần tới đó | FAIL (còn: thêm «Lưng Chừng Cafe» từ Khám phá: thẻ có, «Thêm vào kèo» có, «Thêm vào» có; «Tôi đã tới»: chạm; album ghi «0 chỗ đã tới»; tường có tên quán: không) | RUNTIME-WEB | EV-R-UI-119-C1 (ngoài git) [EV-R-F10-F11-E-ghep](evidence/EV-R-F10-F11-E-ghep.jpg) | UI-119 |

## E5

Đếm: PASS 0 · FAIL 1 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-120 | E5 | E5 | - | cặp đôi chat-8/chat-9 (lập sổ 200, «Một đôi» 200, là cặp đôi: có); chat-8 chặn chat-9 (200) | chat-9: Tin nhắn → chat đôi → link tờ giấy (chat không có dải) → «Rủ đi chơi» → «Gửi cho người ấy»; chat-8 mở tờ giấy | web C1 | A chặn B: POST /papers/{id}/send của B bị từ chối (409); màn B nói không gửi được; sổ của A không nhận tờ mới và không mời «Rủ đi chơi» | FAIL (còn: đã chặn trên máy chủ: có; chat mở ở /groups/[id]/chat, tờ giấy /groups/[id]/to-giay; «Rủ đi chơi» chạm được, «Gửi cho người ấy» chạm được; câu ở B: «· Đổi 18:30 Ăn tối Thứ Bảy 03/10 ĐÃ GỬI Đã gửi, chờ trả lời.»; tờ phía B: da_gui; tờ phía A: da_gui; A thấy «Rủ đi chơi»: không, chữ «chặn»: không; bỏ chặn 200, hết chặn: có) | RUNTIME-WEB | [EV-R-UI-120-B-C1](evidence/EV-R-UI-120-B-C1.jpg) [EV-R-UI-120-A-C1](evidence/EV-R-UI-120-A-C1.jpg) [EV-R-E5-ghep](evidence/EV-R-E5-ghep.jpg) | UI-120 |

## E6

Đếm: PASS 0 · FAIL 1 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-121 | E6 | E6 | - | không phiên, mở link chat nhóm chat-test | từ cửa vào, đăng nhập chat-3 bằng OTP qua UI | web C1 | không phiên, mở link chat nhóm, đăng nhập: tới đúng /groups/<id>/chat | FAIL (còn: link mở ở /login, qua /login; sau mã tới /explore) | RUNTIME-WEB | EV-R-UI-121-C1 (ngoài git) | UI-121 |

## F00

Đếm: PASS 3 · FAIL 8 · BLOCKED 0 · NOT_TESTED 8 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F00-DT-chat-20 | F00 | F00.S01 | - | người chưa có nhóm | mở / | web C1 | đi tới /messages | PASS (tới /messages) | RUNTIME-WEB | EV-F00-DT-chat-20-goc (ngoài git) |  |
| TC-F00-DT-dalat-0 | F00 | F00.S01 | - | người có nhóm đang hoạt động | mở / | web C1 | đi tới /explore | PASS (tới /explore) | RUNTIME-WEB | EV-F00-DT-dalat-0-goc (ngoài git) |  |
| TC-F00-LA-DANG-NHAP | F00 | F00.S02 | - | URL lạ khi đã đăng nhập | mở /khong-co-trang-nay | web C1 | không lạc vào màn chào/đăng nhập khi đã có phiên | FAIL (tới /welcome) | RUNTIME-WEB | EV-F00-DT-dalat-0-la (ngoài git) |  |
| TC-F00-TAB-DOI | F00 | F00.S03 | L05 | đã đăng nhập (dalat-0), năm tab | chạm lần lượt từng tab | web C1 | URL và nội dung đổi theo tab được chạm; tab đang chọn có dấu hiệu nhìn thấy (màu + icon đặc + dải washi) | PASS (phân xử bằng mắt: URL đổi đúng theo từng tab (số đo tự động); trên ảnh ghép C1, C2, C3, 599 và 600, tab đang chọn «Khám phá» có màu nhấn, icon đặc và dải washi. Hàng tự động đỏ vì đọc aria-selected, thiếu trên web: UI-003 (TC-R-UI-003)) | RUNTIME-WEB | [EV-F00-TAB-ghep-a](evidence/EV-F00-TAB-ghep-a.jpg) |  |
| TC-M-UI-123 | F00 | F00.S03 | L05 | có phiên (dalat-0) và không phiên; app mở ở Khám phá | chạm tab «Lên plan», rồi Back trình duyệt; đối chứng: bắt đầu ở Cộng đồng, qua Khám phá rồi Lên plan | web C1 | Back sau khi chuyển tab về tab trước (hoặc ít nhất về một màn của app), không rời app | FAIL (có phiên: /explore#3, Lên plan → /plan#3; Back → /favicon.ico. Không phiên: /explore#2, Lên plan → /plan#2; Back → about:blank. Bắt đầu ở tab đầu: /community#3, Khám phá → /explore#4, Lên plan → /plan#4; Back → /community. Số sau «#» là history.length) | RUNTIME-WEB |  | UI-123 |
| TC-R-UI-002 | F00 | F00.S02 | - | đã đăng nhập (dalat-0) | mở /khong-co-trang-nay | web C1 | URL lạ khi có phiên thì dẫn về tab (hoặc trang lỗi có lối về) | FAIL (còn: tới /welcome; chữ đầu trang «Từ lời rủ đến trang kỷ niệm     Một lời rủ. Nhiều ngày đáng nhớ. Hội bạn hay») | RUNTIME-WEB | EV-R-UI-002-C1 (ngoài git) | UI-002 |
| TC-R-UI-003 | F00 | F00.S03 | L05 | đã đăng nhập, ở Khám phá | đọc ARIA của các phần tử role=tab | web C1 | tab đang chọn có aria-selected=true | FAIL (còn: 5 tab (Cộng đồng=null, Khám phá=null, Lên plan=null, Tin nhắn=null, Cá nhân=null)) | RUNTIME-WEB |  | UI-003 |
| TC-R-UI-004 | F00 | F00.S03 | L05 | rail ở cửa sổ rộng | mở từng tab ở C6 và C7, đo vạch chỉ báo | web C6,C7 | tâm vạch nằm trong khoảng dọc của tab đang chọn, ở mọi tab | FAIL (còn: 4/10 đúng; lệch: C6 Lên plan tab 108–156 vạch 144–216; C6 Tin nhắn tab 156–204 vạch 216–288; C6 Cá nhân tab 204–252 vạch 288–360; C7 Lên plan tab 108–156 vạch 144–216; C7 Tin nhắn tab 156–204 vạch 216–288; C7 Cá nhân tab 204–252 vạch 288–360) | RUNTIME-WEB | EV-R-UI-004-C6 (ngoài git) [EV-R-F00-ghep](evidence/EV-R-F00-ghep.jpg) | UI-004 |
| TC-R-UI-005 | F00 | F00.S04 | L01 | khay tạo mở từ «Tạo mới» của Lên plan | Back trình duyệt khi khay mở, rồi đọc cây truy cập | web C1 | 0 phần tử inert/aria-hidden phủ ≥25% màn sau Back | FAIL (còn: thanh tab không còn nút «Tạo mới» (5 tab); khay mở từ Lên plan: có; Back làm, tới /plan; vùng khoá ≥25% màn: trước 0, sau 1) | RUNTIME-WEB | [EV-R-UI-005-C1](evidence/EV-R-UI-005-C1.jpg) [EV-R-F00-ghep](evidence/EV-R-F00-ghep.jpg) | UI-005 |
| TC-R-UI-006 | F00 | F00.S04 | L01 | Lên plan, khay đóng | chạm «Tạo mới» hai lần cách 60 ms | web C1 | đúng 1 dialog, URL vẫn /create | FAIL (đổi: nút «+» của thanh tab không còn, chạm nút «Tạo mới» của Lên plan; 60 ms sau chạm đầu, dưới ngón là «Đóng»; sau hai chạm 0 dialog, URL /plan) | RUNTIME-WEB | EV-R-UI-006-C1 (ngoài git) [EV-R-F00-ghep](evidence/EV-R-F00-ghep.jpg) | UI-006 |
| TC-R-UI-007 | F00 | - | - | - | chưa đo lại trên main | web | Chiều cao panel ≤82% ở C2 và C8 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-007 |
| TC-R-UI-008 | F00 | - | - | - | chưa đo lại trên main | web | Không điểm dừng Tab nào không có tên | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-008 |
| TC-R-UI-009 | F00 | - | - | - | chưa đo lại trên main | web | Trễ resume 5 s: có progressbar hoặc skeleton trong ≤300 ms | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-009 |
| TC-R-UI-010 | F00 | - | - | - | chưa đo lại trên main | web | Mở lạnh `/create`: có 1 dialog trên Khám phá | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-010 |
| TC-R-UI-011 | F00 | F00.S05 | L03 | bảng Nếp, đã gõ mô tả | bấm «Vẽ» | web C1 | web: bấm «Vẽ» luôn cho một phản hồi nhìn thấy được | FAIL (còn: nút «Vẽ» có; dialog mới 0; alert 0; request 0; chữ bảng đổi: không) | RUNTIME-WEB | EV-R-UI-011-C1 (ngoài git) [EV-R-F00-ghep](evidence/EV-R-F00-ghep.jpg) | UI-011 |
| TC-R-UI-012 | F00 | - | - | - | chưa đo lại trên main | web | Chip ≥48dp | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-012 |
| TC-R-UI-013 | F00 | - | - | - | chưa đo lại trên main | web | Khung cuối trước khi gỡ: đỉnh panel ≥ chiều cao cửa sổ, hoặc độ mờ ≈0 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-013 |
| TC-R-UI-014 | F00 | - | - | - | chưa đo lại trên main | web | `nep.chuBiChe` rỗng ở C2 trên Khám phá | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-014 |
| TC-R-UI-112 | F00 | - | - | - | chưa đo lại trên main | web | Sau khi mở mỗi màn trên, `document.activeElement` nằm trong màn mới | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-112 |

## F01

Đếm: PASS 0 · FAIL 3 · BLOCKED 0 · NOT_TESTED 3 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-001 | F01 | - | - | - | chưa đo lại trên main | web | Mọi `input` một dòng ≥48dp cao | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-001 |
| TC-R-UI-016 | F01 | F01.S01 | L30 | Welcome trang 1 | vuốt trái trên đoạn chữ, đọc chấm trang sau mỗi lần | web C1 | sau mỗi lần vuốt, chấm, mốc và nhãn khớp scrollLeft / pageWidth | FAIL (còn: Trang 1 trên 4 @0/390 → Trang 1 trên 4 @780/390 → Trang 1 trên 4 @1170/390 → Trang 1 trên 4 @1170/390) | RUNTIME-WEB | EV-R-UI-016-C1 (ngoài git) [EV-R-F01-ghep](evidence/EV-R-F01-ghep.jpg) | UI-016 |
| TC-R-UI-017 | F01 | - | - | - | chưa đo lại trên main | web | Vuốt nhanh cũng chỉ sang một trang | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-017 |
| TC-R-UI-018 | F01 | F01.S02 | - | màn mở thẳng bằng link, không có lịch sử | chạm «Quay lại» | web C1 | mở lạnh rồi chạm «Quay lại» luôn đi tới một màn | FAIL (còn: /login: /login → /login; /places/p-lung-chung-cafe: /places/p-lung-chung-cafe → /places/p-lung-chung-cafe; /outings/[id]: /outings/[id] → /outings/[id]) | RUNTIME-WEB | EV-R-UI-018-C1 (ngoài git) [EV-R-F01-ghep](evidence/EV-R-F01-ghep.jpg) | UI-018 |
| TC-R-UI-019 | F01 | F01.S04 | - | mã lời mời sai | dán «khong-phai-ma-that», «Nhận lời mời» | web C1 | mã sai hiện câu về mã, không nhắc cập nhật app | FAIL (còn:  Mã lời mời Nhận lời mời Phần này chưa mở được trên bản app này. \|  Cập nhật app rồi thử lại. \|   Chưa có lời mời? Nhờ một người trong nhóm gửi cho bạn.) | RUNTIME-WEB | EV-R-UI-019-C1 (ngoài git) [EV-R-F01-ghep](evidence/EV-R-F01-ghep.jpg) | UI-019 |
| TC-R-UI-020 | F01 | - | - | - | chưa đo lại trên main | web | axe 0 vi phạm trên `/welcome` | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-020 |

## F02

Đếm: PASS 0 · FAIL 5 · BLOCKED 0 · NOT_TESTED 8 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-021 | F02 | F02.S01 | - | Khám phá Đà Lạt, có phiên | đọc dòng «điểm · km · giá» của mỗi hàng | web C1–C6 | 0 dòng chứa giá bị cắt ở C1–C6 | FAIL (còn: C1 7/8 bị cắt, C2 8/8 bị cắt, C3 8/8 bị cắt, C4 8/8 bị cắt, C5 7/8 bị cắt, C6 6/8 bị cắt) | RUNTIME-WEB | EV-R-UI-021-C1 (ngoài git) [EV-R-F02-ghep](evidence/EV-R-F02-ghep.jpg) | UI-021 |
| TC-R-UI-022 | F02 | F02.S03 | L33 | chi tiết Tiệm Nướng Xóm Lào | chạm «Chỉ đường» | web C1 | web: chạm «Chỉ đường» mở trang mới hoặc hiện câu | FAIL (còn: chi tiết ở /places/p-tiem-nuong-xom-lao; window.open geo:11.9404,108.4383?q=… → null; trang mới không; câu: không) | RUNTIME-WEB | EV-R-UI-022-C1 (ngoài git) [EV-R-F02-ghep](evidence/EV-R-F02-ghep.jpg) | UI-022 |
| TC-R-UI-023 | F02 | F02.S03 | - | chi tiết Tiệm Nướng Xóm Lào | đọc nhãn nút ở chân trang | web C1,C2 | nhãn «Lưu địa điểm» đọc trọn ở C2 | FAIL (còn: C1 «Lưu địa điểm» rộng 78 thiếu 5; C2 «Lưu địa điểm» rộng 49 thiếu 34) | RUNTIME-WEB | EV-R-UI-023-C2 (ngoài git) [EV-R-F02-ghep](evidence/EV-R-F02-ghep.jpg) | UI-023 |
| TC-R-UI-024 | F02 | F02.S01 | - | Khám phá, máy chủ không có khoá AI | chạm ✦ «Hỏi Rủ Đi AI» | web C1 | chạm ✦ không hiện «0 kết quả» trước khi có câu trả lời | FAIL (còn: ô «quán nướng cho 6 người, 200k mỗi người», focus vào ô: không; tiêu đề danh sách «0 kết quả»; «0 kết quả/Chưa thấy nơi phù hợp»: có) | RUNTIME-WEB | EV-R-UI-024-C1 (ngoài git) [EV-R-F02-ghep](evidence/EV-R-F02-ghep.jpg) | UI-024 |
| TC-R-UI-025 | F02 | - | - | - | chưa đo lại trên main | web | `TC-F02-NHAY` nhảy 0dp ở C1 và C9 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-025 |
| TC-R-UI-026 | F02 | - | - | - | chưa đo lại trên main | web | Bỏ lọc không phát lại cú bật dựng | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-026 |
| TC-R-UI-027 | F02 | - | - | - | chưa đo lại trên main | web | Ở C9 không khung nào có vùng sân khấu trống; Nếp M5 ở C9 chỉ có một ảnh từ lúc hiện | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-027 |
| TC-R-UI-028 | F02 | - | - | - | chưa đo lại trên main | web | Thành phố rỗng không hiện «Xóa lọc» | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-028 |
| TC-R-UI-029 | F02 | - | - | - | chưa đo lại trên main | web | 503 hiện câu về máy chủ; mất mạng hiện câu về mạng | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-029 |
| TC-R-UI-030 | F02 | - | - | - | chưa đo lại trên main | web | Mất mạng rồi đổi tab không làm mất danh sách | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-030 |
| TC-R-UI-031 | F02 | - | - | - | chưa đo lại trên main | web | C7 hiện 3 cột | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-031 |
| TC-R-UI-113 | F02 | F10.S01 | - | bảng dev /dev/ui-lab, mục «Khám phá · renderer live», chip «Không ảnh»; «Still Cafe» mang dấu | đo hai nút «Lưu …» của cặp so sánh | web C1,C4,C2 | ở 320dp, cả hai nút «Lưu …» của cặp so sánh không ảnh nằm trọn trong ô và trong màn | FAIL (còn: C1 «Lưu Lẩu gà lá é» 48px ở 139–187, thấy 48px, «Lưu Still Cafe» 48px ở 329–377, thấy 48px; C4 «Lưu Lẩu gà lá é» 48px ở 132–180, thấy 48px, «Lưu Still Cafe» 48px ở 321–369, thấy 48px; C2 «Lưu Lẩu gà lá é» 48px ở 104–152, thấy 48px, «Lưu Still Cafe» 48px ở 294–342, thấy 26px) | RUNTIME-WEB | EV-R-UI-113-C2 (ngoài git) [EV-R-F10-F11-E-ghep](evidence/EV-R-F10-F11-E-ghep.jpg) | UI-113 |
| TC-R-UI-114 | F02 | - | - | - | chưa đo lại trên main | web | axe không còn `nested-interactive` ở cặp so sánh có ảnh; tim vẫn nằm trên góc ảnh và vẫn lưu được | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-114 |

## F03

Đếm: PASS 1 · FAIL 4 · BLOCKED 0 · NOT_TESTED 11 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-032 | F03 | F03.S04 | - | kèo 3 ngày, 3 chặng gắn quán, chặng không có ngày | mở kèo, chọn «Bản đồ», lần lượt từng ngày | web C1 | tổng mốc qua các ngày = số chặng gắn quán, hoặc có dòng nói rõ các chặng chưa xếp ngày | FAIL (còn: Ngày 1: 0 mốc, «chưa có điểm nào»; Ngày 2: 0 mốc, «chưa có điểm nào»; Ngày 3: 0 mốc, «chưa có điểm nào»; câu về chặng chưa xếp ngày: không) | RUNTIME-WEB | EV-R-UI-032-C1 (ngoài git) [EV-R-F03-ghep](evidence/EV-R-F03-ghep.jpg) | UI-032 |
| TC-R-UI-033 | F03 | F03.S04 | L13 | kèo 2 ngày chưa có chặng (dựng qua API trong nhóm chat-test) | chọn «Bản đồ» | web C1,C2,C3,C4,C8 | «Về Lịch trình» thấy trọn không cần cuộn ở C1–C4 và C8 | PASS (phân xử bằng mắt: đổi: «Về Lịch trình» nay ghim thấy trọn ở cả năm cấu hình, tiêu chí đạt; trên ảnh C1, dòng thứ hai của lời giải thích nằm dưới nút, bị cắt (số đo tự động: hết: C1 100% thấy, C2 100% thấy, C3 100% thấy, C4 100% thấy, C8 100% thấy)) | RUNTIME-WEB | EV-R-UI-033-C1 (ngoài git) [EV-R-F03-ghep](evidence/EV-R-F03-ghep.jpg) | UI-033 |
| TC-R-UI-034 | F03 | F03.S02 | - | form tạo kèo, tên đã gõ, ngân sách bỏ trống | chạm «Tạo kèo» | web C1 | bỏ trống ngân sách rồi «Tạo kèo»: câu lỗi thấy được ngay (hoặc kèo được tạo) | FAIL (phân xử bằng mắt: còn: ô ngân sách trống nhưng trên ảnh vẫn hiện «250000» (chữ gợi ý vẽ bằng một Text đè lên ô, không phải thuộc tính placeholder, nên số đo tự động đọc ra rỗng); chạm «Tạo kèo» không tạo kèo, câu lỗi nằm ngoài khung nhìn (số đo tự động: còn: ô ngân sách «Ô ngân sách một người», placeholder «», giá trị «»; sau chạm ở /outings/new?contextId=[id]; kèo trên máy chủ 1 → 1; câu liên quan: «mỗi người khoảng» y 482, «Kèo retest ngân sách trống» y 749, «Ngân sách mỗi người là số tiền Việt Nam, viết bằng chữ số.» y 854 (ngoài màn))) | RUNTIME-WEB | EV-R-UI-034-C1 (ngoài git) [EV-R-F03-ghep](evidence/EV-R-F03-ghep.jpg) | UI-034 |
| TC-R-UI-035 | F03 | F03 | - | có phiên (dalat-0) | mở /trips/<id kèo>/timeline | web C1 | có phiên: /trips/<id>/timeline về /outings/<id> | FAIL (còn: tới /trips/[id]/timeline; dữ liệu demo trên màn: có) | RUNTIME-WEB | EV-R-UI-035-C1 (ngoài git) [EV-R-F03-ghep](evidence/EV-R-F03-ghep.jpg) | UI-035 |
| TC-R-UI-036 | F03 | F03.S03 | - | kèo 3 chặng, thứ tự Cà phê sáng → Ăn trưa → Tối nướng | focus tay nắm «Thứ tự Cà phê sáng», nhấn mũi tên xuống | web C1 | chỉ bằng bàn phím đổi được thứ tự một chặng | FAIL (còn: tay nắm role slider, tabindex -, nhận focus: không; sau mũi tên xuống: Cà phê sáng → Ăn trưa → Tối nướng) | RUNTIME-WEB | EV-R-UI-036-C1 (ngoài git) [EV-R-F03-ghep](evidence/EV-R-F03-ghep.jpg) | UI-036 |
| TC-R-UI-037 | F03 | - | - | - | chưa đo lại trên main | web | Kèo ở Lịch trình có mép Nếp; Bản đồ thì không | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-037 |
| TC-R-UI-038 | F03 | - | - | - | chưa đo lại trên main | web | Back khi sheet mở: sheet đóng, URL giữ nguyên | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-038 |
| TC-R-UI-039 | F03 | - | - | - | chưa đo lại trên main | web | Chạm đúp: 1 sheet | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-039 |
| TC-R-UI-040 | F03 | - | - | - | chưa đo lại trên main | web | C8: sheet ≤ 82% | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-040 |
| TC-R-UI-041 | F03 | - | - | - | chưa đo lại trên main | web | Khi sheet mở, đầu màn bị làm mờ và chạm vào thì đóng sheet | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-041 |
| TC-R-UI-042 | F03 | - | - | - | chưa đo lại trên main | web | axe 0 vi phạm critical trên màn kèo | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-042 |
| TC-R-UI-043 | F03 | - | - | - | chưa đo lại trên main | web | Không còn nhãn tiếng Anh; Esc đóng popup | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-043 |
| TC-R-UI-044 | F03 | - | - | - | chưa đo lại trên main | web | C2: không dòng nào mất số chặng | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-044 |
| TC-R-UI-045 | F03 | - | - | - | chưa đo lại trên main | web | C2: cột tên ≥ 120px | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-045 |
| TC-R-UI-046 | F03 | - | - | - | chưa đo lại trên main | web | Mở `/outings/chon`: một câu và một lối ra trong ≤ 1 s | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-046 |
| TC-R-UI-047 | F03 | - | - | - | chưa đo lại trên main | web | C6/C7: nút Quay lại thẳng mép trái cột nội dung | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-047 |

## F04

Đếm: PASS 9 · FAIL 20 · BLOCKED 0 · NOT_TESTED 9 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F04-BACK-TRINH-DUYET | F04 | F04.S01 | - | bước 3, bill đã gõ | Back của trình duyệt, rồi Forward | web C1 | Back lùi một bước trong luồng như nút của màn (hoặc hỏi trước khi bỏ bill); Forward không mất bill | FAIL (trước /smart-split/moi/review; sau Back /plan; sau Forward /smart-split/moi/review, Bước 1 trên 5: Bill) | RUNTIME-WEB | EV-F04-BACK-C1 (ngoài git) |  |
| TC-F04-BAN-20 | F04 | F04.S01 | - | bước 3, nhóm 20 người | chạm vào giữa hình nhân của từng ghế (elementFromPoint), rồi chạm thật một ghế bị che | web C1 | chạm vào hình nhân nào thì đúng ghế đó; tên đọc được | FAIL (10/20 ghế: chạm vào hình nhân trúng ghế khác; chạm thật «Ghế Chat Test 07» đổi Ghế Chat Test 08) | RUNTIME-WEB | EV-F04-BAN-20-C1 (ngoài git) |  |
| TC-F04-BAN-20 | F04 | F04.S01 | - | bước 3, nhóm 20 người | chạm vào giữa hình nhân của từng ghế (elementFromPoint), rồi chạm thật một ghế bị che | web C2 | chạm vào hình nhân nào thì đúng ghế đó; tên đọc được | FAIL (17/20 ghế: chạm vào hình nhân trúng ghế khác; chạm thật «Ghế Chat Test 07» đổi Ghế Chat Test 08) | RUNTIME-WEB | EV-F04-BAN-20-C2 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) |  |
| TC-F04-BAN-20 | F04 | F04.S01 | - | bước 3, nhóm 20 người | chạm vào giữa hình nhân của từng ghế (elementFromPoint), rồi chạm thật một ghế bị che | web C6 | chạm vào hình nhân nào thì đúng ghế đó; tên đọc được | FAIL (6/20 ghế: chạm vào hình nhân trúng ghế khác; chạm thật «Ghế Chat Test 07» đổi Ghế Chat Test 08) | RUNTIME-WEB | EV-F04-BAN-20-C6 (ngoài git) |  |
| TC-F04-BILL-503 | F04 | F04.S01 | - | bước 2, máy chủ trả 503 khi tạo bill | chạm «Tiếp» ở cuối trang, rồi chạm lại khi máy chủ ổn | web C1 | ở lại bước 2, bill còn nguyên; câu lỗi thấy được và không đổ cho mạng; lần sau đi tiếp được | FAIL (Bước 2 trên 5: Xem lại; câu: «Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một ch» y -578 (ngoài màn); lần sau: Bước 3 trên 5: Gán món) | RUNTIME-WEB | EV-F04-HOA-DON-503-C1 (ngoài git) |  |
| TC-F04-CHAN-NGUOI | F04 | F04.S01 | - | bước 3, một món bỏ hết người | chạm «Xem kết quả» | web C1 | ở lại bước 3, câu lý do thấy được cạnh chỗ bấm | FAIL (ở Bước 3 trên 5: Gán món; câu: «Món "Lẩu gà lá é nồi lớn cho cả nhóm hai mươi người, thêm nấm và rau r» y -256 ngoài màn; «Món "Lẩu gà lá é nồi lớn cho cả nhóm hai mươi người, thêm nấm và rau r» y -184 ngoài màn) | RUNTIME-WEB | EV-F04-CHAN-NGUOI-C1 (ngoài git) |  |
| TC-F04-CHAN-TEN | F04 | F04.S01 | - | bước 2, 6 món, món thứ 5 chưa có tên, đang ở cuối trang | chạm «Tiếp: ai dùng món nào?» | web C1 | ở lại bước 2 và lý do hiện ở chỗ người dùng đang nhìn (DESIGN: lỗi là một câu cạnh chỗ bấm) | FAIL (ở Bước 2 trên 5: Xem lại; nút không mờ (aria-disabled không có); câu lý do có 2 chỗ, y -248–-212 ngoài màn, y -194–-146 ngoài màn; cửa sổ cao 844) | RUNTIME-WEB | EV-F04-CHAN-TEN-C1 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) |  |
| TC-F04-GHI-CHAM-DUP | F04 | F04.S01 | - | bước 4, 20 cuống, tổng 13.705.678đ | chạm «Ghi vào sổ» hai lần cách 70 ms | web C1 | đúng một khoản chi được ghi; sang bước 5 | PASS (khoản chi của nhóm: 0 → 1) | RUNTIME-WEB | EV-F04-KET-QUA-20-C1 (ngoài git) EV-F04-DA-GHI-C1 (ngoài git) |  |
| TC-F04-LUI-TRONG | F04 | F04.S01 | - | bước 4, bill 3 món đã gõ | chạm «Quay lại» của đầu màn hai lần | web C1 | lùi 4 → 3 → 2, giữ nguyên các món và số tiền | PASS (Bước 4  → Bước 3  → Bước 2 ; tổng 13.705.678đ còn) | RUNTIME-WEB |  |  |
| TC-F04-LUI-VE-BUOC1 | F04 | F04.S01 | - | bước 2, bill 3 món đã gõ | «Quay lại» về bước 1, rồi chạm «Nhập tay» (bước 1 không có lối nào khác về bill) | web C1 | bill đã gõ còn nguyên, hoặc có câu hỏi trước khi bỏ | FAIL (bước 1 có: Quay lại, Chọn ảnh bill, Nhập tay, Cách chia; sau «Nhập tay»: Ô tên món 1=, Ô số lượng món 1=1, Ô tiền món 1=) | RUNTIME-WEB | EV-F04-LUI-MAT-C1 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) |  |
| TC-F04-MON-DAI | F04 | F04.S01 | - | 3 món: tên 70 ký tự, một từ 41 ký tự liền, 12.345.678đ | nhập tay ở bước 2, sang bước 3 và 4 | web C1 | không tràn ngang; không số tiền nào bị cắt hay ellipsis; tổng đúng | FAIL (tràn 0px; số tiền bị cắt 2: B2 12.345.678đ còn 41px; B2 400.000đ còn 42px; tới Bước 4 trên 5: Kết quả; 8 ghế) | RUNTIME-WEB | EV-F04-BUOC2-C1 (ngoài git) EV-F04-BUOC3-C1 (ngoài git) EV-F04-BUOC4-C1 (ngoài git) EV-F04-TIEN-CAT-B2-C1 (ngoài git) |  |
| TC-F04-MON-DAI | F04 | F04.S01 | - | 3 món: tên 70 ký tự, một từ 41 ký tự liền, 12.345.678đ | nhập tay ở bước 2, sang bước 3 và 4 | web C2 | không tràn ngang; không số tiền nào bị cắt hay ellipsis; tổng đúng | FAIL (tràn 0px; số tiền bị cắt 3: B2 12.345.678đ còn 30px; B2 400.000đ còn 31px; B3 12.345.678đ còn 68px; tới Bước 4 trên 5: Kết quả; 8 ghế) | RUNTIME-WEB | EV-F04-BUOC2-C2 (ngoài git) EV-F04-BUOC3-C2 (ngoài git) EV-F04-BUOC4-C2 (ngoài git) EV-F04-TIEN-CAT-B2-C2 (ngoài git) EV-F04-TIEN-CAT-B3-C2 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) |  |
| TC-F04-MON-DAI | F04 | F04.S01 | - | 3 món: tên 70 ký tự, một từ 41 ký tự liền, 12.345.678đ | nhập tay ở bước 2, sang bước 3 và 4 | web C3 | không tràn ngang; không số tiền nào bị cắt hay ellipsis; tổng đúng | FAIL (tràn 0px; số tiền bị cắt 2: B2 12.345.678đ còn 37px; B2 400.000đ còn 37px; tới Bước 4 trên 5: Kết quả; 8 ghế) | RUNTIME-WEB | EV-F04-BUOC2-C3 (ngoài git) EV-F04-BUOC3-C3 (ngoài git) EV-F04-BUOC4-C3 (ngoài git) EV-F04-TIEN-CAT-B2-C3 (ngoài git) |  |
| TC-F04-MON-DAI | F04 | F04.S01 | - | 3 món: tên 70 ký tự, một từ 41 ký tự liền, 12.345.678đ | nhập tay ở bước 2, sang bước 3 và 4 | web C4 | không tràn ngang; không số tiền nào bị cắt hay ellipsis; tổng đúng | FAIL (tràn 0px; số tiền bị cắt 2: B2 12.345.678đ còn 39px; B2 400.000đ còn 39px; tới Bước 4 trên 5: Kết quả; 8 ghế) | RUNTIME-WEB | EV-F04-BUOC2-C4 (ngoài git) EV-F04-BUOC3-C4 (ngoài git) EV-F04-BUOC4-C4 (ngoài git) EV-F04-TIEN-CAT-B2-C4 (ngoài git) |  |
| TC-F04-MON-DAI | F04 | F04.S01 | - | 3 món: tên 70 ký tự, một từ 41 ký tự liền, 12.345.678đ | nhập tay ở bước 2, sang bước 3 và 4 | web C5 | không tràn ngang; không số tiền nào bị cắt hay ellipsis; tổng đúng | FAIL (tràn 0px; số tiền bị cắt 2: B2 12.345.678đ còn 47px; B2 400.000đ còn 48px; tới Bước 4 trên 5: Kết quả; 8 ghế) | RUNTIME-WEB | EV-F04-BUOC2-C5 (ngoài git) EV-F04-BUOC3-C5 (ngoài git) EV-F04-BUOC4-C5 (ngoài git) EV-F04-TIEN-CAT-B2-C5 (ngoài git) |  |
| TC-F04-NEP-M3 | F04 | F04.S01 | - | bước 5 «Đã ghi sổ» | sau khi ghi | web C1 | Nếp M3 và dấu «Đã ghi sổ» cùng nhịp; Nếp cách số tiền ≥16dp | PASS (Nếp gần tiền nhất: 160dp tới 685.284đ; khung: N0.2 N0.2 N1 N1 N1 N1 N1 N1 N1 N1 N1 N1) | RUNTIME-WEB | EV-F04-DA-GHI-C1 (ngoài git) |  |
| TC-F04-PHAT-HAI-BUOC | F04 | F04.S04 | - | đợt mới, chưa phát | «Phát đợt thu», rồi «Thôi, chưa phát» | web C1 | lần chạm đầu chỉ mở câu hỏi nói rõ không hoàn lại; «Thôi» đóng lại, chưa phát | PASS (hộp hỏi hiện; sau «Thôi»: hộp đóng, nút «Phát đợt thu» còn) | RUNTIME-WEB | EV-F04-PHAT-HOI-C1 (ngoài git) |  |
| TC-F04-PHAT | F04 | F04.S04 | - | đợt của nhóm 20 người | «Phát đợt thu» → «Phát, không hoàn lại» | web C1 | đã phát; phong bì có một link cho mỗi người nợ | PASS (đường /batches/8a0a11f7-0c6e-4794-8395-88e2be0c9985; 19 nút «Gửi cho …») | RUNTIME-WEB | EV-F04-DOT-MOI-C1 (ngoài git) EV-F04-DA-PHAT-C1 (ngoài git) |  |
| TC-F04-TAI-LAI | F04 | F04.S01 | - | bước 3 (máy chủ đã giữ bản nháp bill) | tải lại trang | web C1 | trở lại được bill đang làm, hoặc được báo là bill đã bỏ | FAIL (sau khi tải lại: Bước 1 trên 5: Bill, không có câu nào nhắc bill vừa làm) | RUNTIME-WEB |  |  |
| TC-F04.S03-503 | F04 | F04.S03 | - | máy chủ trả 503 | mở /settlements/[id], rồi «Thử lại» khi máy chủ đã ổn | web C1 | câu tiếng Việt nói đúng là máy chủ lỗi, không nói mạng; «Thử lại» tải lại được | PASS (màn: « Quyết toán chuyến đi Chưa đọc được sổ Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút. Thử lại»; thử lại được) | RUNTIME-WEB | EV-TC-F04.S03-503-C1 (ngoài git) |  |
| TC-F04.S04-503 | F04 | F04.S04 | - | máy chủ trả 503 | mở /batches/[id], rồi «Thử lại» khi máy chủ đã ổn | web C1 | câu tiếng Việt nói đúng là máy chủ lỗi, không nói mạng; «Thử lại» tải lại được | PASS (màn: « Đợt thu Chưa đọc được bảng thu Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút. Thử lại»; thử lại được) | RUNTIME-WEB | EV-TC-F04.S04-503-C1 (ngoài git) |  |
| TC-F04.S05-503 | F04 | F04.S05 | - | máy chủ trả 503 | mở /finance, rồi «Thử lại» khi máy chủ đã ổn | web C1 | câu tiếng Việt nói đúng là máy chủ lỗi, không nói mạng; «Thử lại» tải lại được | PASS (màn: « Tài chính của tôi Chưa đọc được sổ Rủ Đi đang gặp sự cố, chưa đọc được sổ. Thử lại»; thử lại được) | RUNTIME-WEB | EV-TC-F04.S05-503-C1 (ngoài git) |  |
| TC-F04.S05-OFFLINE | F04 | F04.S05 | - | mất mạng | mở Tài chính từ trong app | web C1 | câu nói không kết nối được, có «Thử lại» | PASS (màn: « Tài chính của tôi Chưa đọc được sổ Không kết nối được Rủ Đi. Thử lại») | RUNTIME-WEB | EV-F04.S05-OFFLINE-C1 (ngoài git) |  |
| TC-L32-VONGDOI | F04 | F04.S04 | L32 | đợt đã phát, phong bì link trên máy này | «Gửi cho Chat Test 09» trên web: (a) trình duyệt không có navigator.share, (b) có và chia sẻ xong, (c) người dùng đóng khay | web C1 | (a) có lối khác (chép link) hoặc câu nói đúng là trình duyệt không chia sẻ được; (b) hàng ghi «Đã mở khay chia sẻ»; (c) không báo lỗi | FAIL ((a) «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.» y -2855 (ngoài màn); (b) «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.» y -2855 (ngoài màn); (c) «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.» y -2855 (ngoài màn)) | RUNTIME-WEB | [EV-F04-CHIA-SE-KHONG-CO-C1](evidence/EV-F04-CHIA-SE-KHONG-CO-C1.jpg) EV-F04-CHIA-SE-DUOC-C1 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) |  |
| TC-R-UI-048 | F04 | F04.S01 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F04-MON-DAI C1, TC-F04-MON-DAI C2, TC-F04-MON-DAI C3, TC-F04-MON-DAI C4, TC-F04-MON-DAI C5 | web C1,C2,C3,C4,C5 | bill trên, bước 2 ở C1–C5 và bước 3 ở C2: 0 phần tử số tiền có scrollWidth > clientWidth; quyết toán demo ở C2 cũng vậy | FAIL (còn: C1: số tiền bị cắt 2: B2 12.345.678đ còn 41px; C2: số tiền bị cắt 3: B2 12.345.678đ còn 30px; C3: số tiền bị cắt 2: B2 12.345.678đ còn 37px; C4: số tiền bị cắt 2: B2 12.345.678đ còn 39px; C5: số tiền bị cắt 2: B2 12.345.678đ còn 47px; quyết toán demo ở C2 chưa đo lại) | RUNTIME-WEB | EV-F04-BUOC2-C1 (ngoài git) EV-F04-BUOC3-C1 (ngoài git) EV-F04-BUOC4-C1 (ngoài git) EV-F04-TIEN-CAT-B2-C1 (ngoài git) EV-F04-BUOC2-C2 (ngoài git) EV-F04-BUOC3-C2 (ngoài git) EV-F04-BUOC4-C2 (ngoài git) EV-F04-TIEN-CAT-B2-C2 (ngoài git) EV-F04-TIEN-CAT-B3-C2 (ngoài git) EV-F04-BUOC2-C3 (ngoài git) EV-F04-BUOC3-C3 (ngoài git) EV-F04-BUOC4-C3 (ngoài git) EV-F04-TIEN-CAT-B2-C3 (ngoài git) EV-F04-BUOC2-C4 (ngoài git) EV-F04-BUOC3-C4 (ngoài git) EV-F04-BUOC4-C4 (ngoài git) EV-F04-TIEN-CAT-B2-C4 (ngoài git) EV-F04-BUOC2-C5 (ngoài git) EV-F04-BUOC3-C5 (ngoài git) EV-F04-BUOC4-C5 (ngoài git) EV-F04-TIEN-CAT-B2-C5 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) | UI-048 |
| TC-R-UI-049 | F04 | F04.S04 | L32 | đo bằng kịch bản gốc chạy lại trên main | TC-L32-VONGDOI C1 | web C1 | ba trường hợp: (a) chép được link, (b) hàng ghi «Đã mở khay chia sẻ», (c) không có câu lỗi | FAIL (còn: (a) «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.» y -2855 (ngoài màn); (b) «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.» y -2855 (ngoài màn); (c) «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử lại.» y -2855 (ngoài màn)) | RUNTIME-WEB | [EV-F04-CHIA-SE-KHONG-CO-C1](evidence/EV-F04-CHIA-SE-KHONG-CO-C1.jpg) EV-F04-CHIA-SE-DUOC-C1 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) | UI-049 |
| TC-R-UI-050 | F04 | F04.S01 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F04-BAN-20 C1, TC-F04-BAN-20 C2, TC-F04-BAN-20 C6 | web C1,C2,C6 | chạm tâm mỗi hình nhân trúng đúng ghế, nhóm tới 20 người ở 288–700dp | FAIL (còn: C1: 10/20 ghế: chạm vào hình nhân trúng ghế khác; chạm thật «Ghế Chat Test 07» đổi Ghế Chat Test 08; C2: 17/20 ghế: chạm vào hình nhân trúng ghế khác; chạm thật «Ghế Chat Test 07» đổi Ghế Chat Test 08; C6: 6/20 ghế: chạm vào hình nhân trúng ghế khác; chạm thật «Ghế Chat Test 07» đổi Ghế Chat Test 08) | RUNTIME-WEB | EV-F04-BAN-20-C1 (ngoài git) EV-F04-BAN-20-C2 (ngoài git) EV-F04-BAN-20-C6 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) | UI-050 |
| TC-R-UI-051 | F04 | F04.S01 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F04-CHAN-TEN C1, TC-F04-CHAN-NGUOI C1, TC-F04-BILL-503 C1 | web C1 | các trường hợp đã đo: câu nằm trong khung nhìn ngay sau khi chạm | FAIL (còn: CHAN-TEN: ở Bước 2 trên 5: Xem lại; nút không mờ (aria-disabled không có); câu lý do có 2 chỗ, y -248–-212 ngoài màn, y -194–-146 ngoài màn; cửa sổ cao 844; CHAN-NGUOI: ở Bước 3 trên 5: Gán món; câu: «Món "Lẩu gà lá é nồi lớn cho cả nhóm hai mươi người, thêm nấm và rau r» y -256 ngoài màn; «Món "Lẩu gà lá é nồi lớn ch; BILL-503: Bước 2 trên 5: Xem lại; câu: «Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một ch» y -578 (ngoài màn); lần sau:; trường hợp 4 (ảnh bill máy chủ không đọc được) chưa đo lại) | RUNTIME-WEB | EV-F04-CHAN-TEN-C1 (ngoài git) EV-F04-CHAN-NGUOI-C1 (ngoài git) EV-F04-HOA-DON-503-C1 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) | UI-051 |
| TC-R-UI-052 | F04 | F04.S01 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F04-LUI-VE-BUOC1 C1, TC-F04-BACK-TRINH-DUYET C1, TC-F04-TAI-LAI C1 | web C1 | về bước 1, Back rồi Forward, tải lại: không mất món nào mà không hỏi | FAIL (còn: LUI-VE-BUOC1: bước 1 có: Quay lại, Chọn ảnh bill, Nhập tay, Cách chia; sau «Nhập tay»: Ô tên món 1=, Ô số lượng món 1=1, Ô tiền món 1=; BACK-TRINH-DUYET: trước /smart-split/moi/review; sau Back /plan; sau Forward /smart-split/moi/review, Bước 1 trên 5: Bill; TAI-LAI: sau khi tải lại: Bước 1 trên 5: Bill, không có câu nào nhắc bill vừa làm) | RUNTIME-WEB | EV-F04-LUI-MAT-C1 (ngoài git) EV-F04-BACK-C1 (ngoài git) [EV-R-F04-ghep](evidence/EV-R-F04-ghep.jpg) | UI-052 |
| TC-R-UI-053 | F04 | - | - | - | chưa đo lại trên main | web | Space đổi `aria-checked` ở ghế và ô danh sách | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-053 |
| TC-R-UI-054 | F04 | - | - | - | chưa đo lại trên main | web | Nhóm 10 người ở 288–398: 0 cặp nhãn đè, 0 nhãn ra ngoài khung | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-054 |
| TC-R-UI-055 | F04 | - | - | - | chưa đo lại trên main | web | Hộp Nếp M2 nằm trọn trong màn ở C1–C3 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-055 |
| TC-R-UI-056 | F04 | - | - | - | chưa đo lại trên main | web | Sau khi đọc hỏng, lối nhập tay có ngay trên màn | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-056 |
| TC-R-UI-057 | F04 | - | - | - | chưa đo lại trên main | web | Ở màn tiền, không phần tử nào có nhãn hứa hành động mà không làm | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-057 |
| TC-R-UI-058 | F04 | - | - | - | chưa đo lại trên main | web | Sổ không còn khoản ngoài đợt thì không có nút mời một việc chắc chắn bị từ chối | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-058 |
| TC-R-UI-059 | F04 | - | - | - | chưa đo lại trên main | web | Ở C1–C3, dòng người trả luôn thấy chữ «trả» | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-059 |
| TC-R-UI-060 | F04 | - | - | - | chưa đo lại trên main | web | Tiêu đề mục khớp nội dung bên dưới | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-060 |
| TC-R-UI-061 | F04 | - | - | - | chưa đo lại trên main | web | Ở C2 câu giải thích ≤ 5 dòng | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-061 |

## F05

Đếm: PASS 0 · FAIL 8 · BLOCKED 0 · NOT_TESTED 9 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F05-SOAN-CAN | F05 | F05.S02 | - | ô soạn trống | mở chat | web C1 | một dòng chữ nằm giữa viên thuốc, thẳng hàng với «+» và mũi tên gửi (chú thích trong mã: «Centred on purpose») | FAIL (ô cao 64 (minHeight 48, rows=null); giữa dòng đầu y 785, giữa nút «+» y 805, lệch 20px) | RUNTIME-WEB | EV-F05-SOAN-rong-C1 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) |  |
| TC-F05-SOAN-NHIEU-DONG | F05 | F05.S02 | - | gõ 7 dòng, Enter giữa các dòng | gõ | web C1 | Enter xuống dòng, không gửi; ô cao dần tới trần 120 rồi cuộn; nút gửi vẫn thấy | FAIL (tin trong nhóm 40 → 40; ô cao 64 → 64, cuộn trong ô có; nút gửi thấy) | RUNTIME-WEB | EV-F05-SOAN-7-DONG-C1 (ngoài git) |  |
| TC-F05-URL-DAI | F05 | F05.S02 | - | tin của mình là một URL 158 ký tự (đoạn liền dài nhất 36 ký tự) | mở chat nhóm 20 người | web C1 | bong bóng ≤ 82% hàng, nằm trọn trong màn, chữ xuống dòng | FAIL (bong bóng x 28–374 (rộng 346, trần 82% = 294), cửa sổ 390) | RUNTIME-WEB | EV-F05-URL-C1 (ngoài git) |  |
| TC-F05-URL-DAI | F05 | F05.S02 | - | tin của mình là một URL 158 ký tự (đoạn liền dài nhất 36 ký tự) | mở chat nhóm 20 người | web C2 | bong bóng ≤ 82% hàng, nằm trọn trong màn, chữ xuống dòng | FAIL (bong bóng x -42–304 (rộng 346, trần 82% = 236), cửa sổ 320) | RUNTIME-WEB | EV-F05-URL-C2 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) |  |
| TC-F05-URL-DAI | F05 | F05.S02 | - | tin của mình là một URL 158 ký tự (đoạn liền dài nhất 36 ký tự) | mở chat nhóm 20 người | web C3 | bong bóng ≤ 82% hàng, nằm trọn trong màn, chữ xuống dòng | FAIL (bong bóng x -2–344 (rộng 346, trần 82% = 269), cửa sổ 360) | RUNTIME-WEB | EV-F05-URL-C3 (ngoài git) |  |
| TC-F05-URL-DAI | F05 | F05.S02 | - | tin của mình là một URL 158 ký tự (đoạn liền dài nhất 36 ký tự) | mở chat nhóm 20 người | web C5 | bong bóng ≤ 82% hàng, nằm trọn trong màn, chữ xuống dòng | FAIL (bong bóng x 68–414 (rộng 346, trần 82% = 326), cửa sổ 430) | RUNTIME-WEB | EV-F05-URL-C5 (ngoài git) |  |
| TC-R-UI-062 | F05 | F05.S02 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F05-SOAN-CAN C1, TC-F05-SOAN-NHIEU-DONG C1 | web C1 | web C1: ô trống thì giữa dòng chữ lệch ≤ 4px so với giữa «+» và nút gửi; gõ 7 dòng thì ô cao tới 120 rồi mới cuộn | FAIL (còn: SOAN-CAN: ô cao 64 (minHeight 48, rows=null); giữa dòng đầu y 785, giữa nút «+» y 805, lệch 20px; SOAN-NHIEU-DONG: tin trong nhóm 40 → 40; ô cao 64 → 64, cuộn trong ô có; nút gửi thấy) | RUNTIME-WEB | EV-F05-SOAN-rong-C1 (ngoài git) EV-F05-SOAN-7-DONG-C1 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) | UI-062 |
| TC-R-UI-063 | F05 | F05.S02 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F05-URL-DAI C1, TC-F05-URL-DAI C2, TC-F05-URL-DAI C3, TC-F05-URL-DAI C5 | web C1,C2,C3,C5 | tin có link dài ở C1, C2, C3, C5: bong bóng nằm trong [0, bề rộng cửa sổ] và ≤ 82% hàng | FAIL (còn: C1: bong bóng x 28–374 (rộng 346, trần 82% = 294), cửa sổ 390; C2: bong bóng x -42–304 (rộng 346, trần 82% = 236), cửa sổ 320; C3: bong bóng x -2–344 (rộng 346, trần 82% = 269), cửa sổ 360; C5: bong bóng x 68–414 (rộng 346, trần 82% = 326), cửa sổ 430) | RUNTIME-WEB | EV-F05-URL-C1 (ngoài git) EV-F05-URL-C2 (ngoài git) EV-F05-URL-C3 (ngoài git) EV-F05-URL-C5 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) | UI-063 |
| TC-R-UI-064 | F05 | - | - | - | chưa đo lại trên main | web | Đáy avatar = đáy bong bóng cuối của cụm ± 4px. Trong một quãng không nghỉ, trên màn chỉ có một nhãn giờ | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-064 |
| TC-R-UI-065 | F05 | - | - | - | chưa đo lại trên main | web | Bình chọn 3 lựa chọn, 0 phiếu: thẻ ≤ 25% chiều cao C1; «phiếu» xuất hiện ≤ 1 lần khi chưa ai bầu; ở C6 thẻ ≤ 560dp | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-065 |
| TC-R-UI-066 | F05 | - | - | - | chưa đo lại trên main | web | Esc đóng khay; focus về nút đã mở nó | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-066 |
| TC-R-UI-067 | F05 | - | - | - | chưa đo lại trên main | web | Quét DOM: 5 phần tử `role=radio`, đúng một cái `aria-checked=true`; axe sạch | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-067 |
| TC-R-UI-068 | F05 | - | - | - | chưa đo lại trên main | web | Cùng điều kiện: không có câu lỗi khi tin đã hiện, hoặc câu có «Thử lại» và nói đúng phần hỏng | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-068 |
| TC-R-UI-069 | F05 | - | - | - | chưa đo lại trên main | web | Cùng điều kiện khi đang đọc tin cũ: câu lỗi nằm trong vùng đang xem; thẻ không có biểu tượng hay màu của AI | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-069 |
| TC-R-UI-070 | F05 | - | - | - | chưa đo lại trên main | web | Khi sheet mở, dải bị làm mờ như phần còn lại của màn | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-070 |
| TC-R-UI-079 | F05 | - | - | - | chưa đo lại trên main | web | Cùng điều kiện, 20 s: không «Đang nối lại», không «Một lời mở đầu», không «Đi đâu không?»; phía người chặn có câu nói đã chặn | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-079 |
| TC-R-UI-080 | F05 | - | - | - | chưa đo lại trên main | web | Hàng lời mời có tên người mời và hai lựa chọn | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-080 |

## F06

Đếm: PASS 6 · FAIL 4 · BLOCKED 0 · NOT_TESTED 6 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F06-MOI-DANH-SACH | F06 | F06.S02 | - | vừa mời một người | Back về Thành viên | web C1 | người vừa mời hiện «Đã mời, chưa đồng ý»; đếm «1 đang được mời» | PASS («Đã mời, chưa đồng ý» có; tiêu đề phụ: «1 đang ở trong nhóm, 1 đang được mời.») | RUNTIME-WEB |  |  |
| TC-F06-MOI-GUI | F06 | F06.S03 | - | tên 59 ký tự, số của một người chưa dùng Rủ Đi | «Gửi lời mời» | web C1 | phong bì «Đã gửi», nói người kia thấy lời mời ở đâu; một thành viên «invited» | PASS (màn: « Mời vào nhóm Đã mời Bạn thân từ hồi cấp ba của mình, người hay trễ hẹn nhất hội Khi người này đăng nhập bằng số đó, lời mời hiện ở tab Tin nhắn và chính họ bấ»; thành viên đang được mời: 1) | RUNTIME-WEB | EV-F06-MOI-XONG-C1 (ngoài git) |  |
| TC-F06-MOI-THIEU | F06 | F06.S03 | - | form mời trống, rồi chỉ có số | «Gửi lời mời» hai lần | web C1 | mỗi lần một câu nói thiếu gì, trong tầm nhìn | PASS (vào /groups/[id]/invite; trống: «Chưa đúng dạng số di động Việt Nam.» y 484; thiếu tên: «Đặt tên cho người bạn đang mời, để cả nhóm biết đó là ai.» y 484) | RUNTIME-WEB |  |  |
| TC-F06-TAO-503 | F06 | F06.S01 | - | POST /contexts trả 503 | «Mở nhóm», rồi máy chủ ổn và chạm lại | web C1 | câu nói đúng lỗi máy chủ, trong tầm nhìn, tên còn nguyên; chạm lại mở đúng một nhóm | PASS (câu «Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút.» ở y 372; nhóm 1 → 2 sau lần chạm lại) | RUNTIME-WEB | EV-F06-TAO-503-C1 (ngoài git) |  |
| TC-F06-TAO-CHAM-DUP | F06 | F06.S01 | - | tên 63 ký tự có emoji | chạm «Mở nhóm» hai lần cách 60 ms | web C1 | đúng một nhóm được mở | PASS (nhóm của moi-51: 0 → 1; ô tên 230×44, chữ cuộn ngang trong ô) | RUNTIME-WEB |  |  |
| TC-F06-TAO-SAU | F06 | F06.S01 | - | vừa mở nhóm (người chưa có nhóm nào) | sau «Mở nhóm» | web C1 | tới nơi thấy nhóm vừa mở (chat hoặc mời), hoặc có một câu xác nhận kèm lối mời bạn; câu trên màn hứa «Mời bạn bè sau» | FAIL (tới /explore; tên nhóm mới không có trên màn) | RUNTIME-WEB | EV-F06-TAO-SAU-C1 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) |  |
| TC-F06-TAO-TRONG | F06 | F06.S01 | - | tên trống | chạm «Mở nhóm» | web C1 | một câu nói thiếu gì, trong tầm nhìn, cạnh ô; không tạo nhóm | PASS (câu «Đặt tên cho nhóm.» ở y 372; focus Mở nhóm) | RUNTIME-WEB |  |  |
| TC-R-UI-071 | F06 | F06.S01 | - | đo bằng kịch bản gốc chạy lại trên main | TC-F06-TAO-SAU C1 | web C1 | tạo nhóm xong tới nơi thấy nhóm vừa mở (chat hoặc mời), hoặc có câu xác nhận kèm lối mời bạn | FAIL (còn: tới /explore; tên nhóm mới không có trên màn) | RUNTIME-WEB | EV-F06-TAO-SAU-C1 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) | UI-071 |
| TC-R-UI-072 | F06 | - | - | - | chưa đo lại trên main | web | Mời số của một thành viên: câu nói người này đã ở trong nhóm | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-072 |
| TC-R-UI-073 | F06 | F06.S03 | - | số chưa từng đăng nhập, được moi-51 mời dưới tên «Bạn cũ tên do người mời đặt R61» | đăng nhập lần đầu bằng OTP qua UI; qua Sở thích với tên tự chọn; «Đồng ý vào nhóm»; một người lạ (chat-20) tra số | web C1 | số được mời đăng nhập lần đầu: vào Sở thích, ô tên điền sẵn và sửa được; tra số không lộ tên người khác đặt khi chính chủ chưa xác nhận | FAIL (còn: lời mời gửi: có; vào cửa tới /messages; ô tên «-» (điền sẵn tên người mời đặt: không), tên người mời đặt trên màn: không; sau Sở thích ở /messages; «Đồng ý vào nhóm»: có; trên máy chủ active, tên trong nhóm «Bạn cũ tên do người mời đặt R61»; người lạ tra số thấy tên người mời đặt (Gửi lời mời)) | RUNTIME-WEB | EV-R-UI-073-A-C1 (ngoài git) EV-R-UI-073-B-C1 (ngoài git) EV-R-UI-073-C-C1 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) | UI-073 |
| TC-R-UI-074 | F06 | F06.S02 | - | nhóm F06 có 2 quản trị; người xem là người lập nhóm (moi-51) | Thành viên → chạm «Bỏ quyền quản trị» trên hàng «… (bạn)» | web C1 | có bước hỏi; vai trò chỉ đổi sau khi xác nhận | FAIL (còn: nút «Bỏ quyền quản trị»; vai trò admin → member sau một chạm; bước hỏi: không; đã đặt lại vai trò qua API) | RUNTIME-WEB | EV-R-UI-074-C1 (ngoài git) [EV-R-F05-F06-ghep](evidence/EV-R-F05-F06-ghep.jpg) | UI-074 |
| TC-R-UI-075 | F06 | - | - | - | chưa đo lại trên main | web | Nhóm có hai quản trị: chỉ người lập nhóm mang nhãn đó | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-075 |
| TC-R-UI-076 | F06 | - | - | - | chưa đo lại trên main | web | Quét DOM: tên các nút vai trò khác nhau; chạm hàng mở hồ sơ | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-076 |
| TC-R-UI-077 | F06 | - | - | - | chưa đo lại trên main | web | Cùng điều kiện: danh sách còn, câu lỗi nằm cạnh hàng lời mời | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-077 |
| TC-R-UI-078 | F06 | - | - | - | chưa đo lại trên main | web | Tải lại hồ sơ người đã chặn: thấy «Đã chặn» và «Bỏ chặn», không thấy «Kết bạn» | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-078 |
| TC-R-UI-081 | F06 | - | - | - | chưa đo lại trên main | web | C6/C7: nút hành động cách tên ≤ 160px | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-081 |

## F07

Đếm: PASS 0 · FAIL 6 · BLOCKED 0 · NOT_TESTED 8 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-082-SO | F07 | F07.S02 | - | không phiên; link tờ giấy của cặp chat-4/chat-5 (sổ đã mở) | mở thẳng link; nếu ở lại trang: «Rủ đi chơi» → «Gửi cho người ấy» | web C1 | không phiên, mở link: tới cửa vào; không màn nào hiện «Đã gửi» khi chưa có lệnh ghi thành công | FAIL (phân xử bằng mắt: còn, và đổi: không phiên, link tờ giấy của một cặp thật ở lại /groups/[id]/to-giay và hiện sổ demo «Hội bạn · Người ấy» (tờ «Thứ Bảy 06/09 · Bún chả, quán góc phố», «Chủ nhật 14/09»), không nhãn demo, không lối đăng nhập. Bản demo nay là sổ «hội bạn», không còn «Rủ đi chơi» nên không còn đường tới «Đã gửi» giả; ảnh thứ hai trùng từng byte với ảnh đầu, bỏ (số đo tự động: còn: tới /groups/[id]/to-giay; nhãn demo/trải nghiệm: không; «Rủ đi chơi» không, «Gửi cho người ấy» không, «Đã gửi» không hiện; lệnh ghi tới API: 0; màn đầu « Tờ giấy của hai mình Hội bạn · Người ấy  Hai người cũng thành một hội Hẹn nhau như mọi hội bạn. Những tờ giấy cũ vẫn »)) | RUNTIME-WEB | [EV-R-UI-082-A-C1](evidence/EV-R-UI-082-A-C1.jpg) [EV-R-F07-ghep](evidence/EV-R-F07-ghep.jpg) | UI-082 |
| TC-R-UI-083 | F07 | F07.S02 | - | sổ chat-4/chat-5 đã mở; GET sổ và tờ trả 503 | mở tờ giấy; máy chủ ổn lại; «Thử lại» nếu có | web C1 | 503 khi đọc sổ: màn nói chưa đọc được, có «Thử lại»; không hiện bìa «chưa có sổ» | FAIL (còn: khi 503: bìa sổ chưa lập có, « Tờ giấy của hai mình Hội bạn · Chat Test 06  Chat Test 05 Chat Test 06 Một chỗ cho chuyện hai mình Mở sổ để gửi lời hẹn cho người thương. Cả hai đồng ý mở sổ, rồi cùng»; nút thử lại: không có; sau khi máy chủ ổn: « Tờ giấy của hai mình Hội bạn · Chat Test 06  Hai người cũng thành một hội Hẹn nhau như mọi hội bạn. Những t») | RUNTIME-WEB | EV-R-UI-083-C1 (ngoài git) [EV-R-F07-ghep](evidence/EV-R-F07-ghep.jpg) | UI-083 |
| TC-R-UI-084-A | F07 | F07.S02 | L23 | chat-4 vừa đề nghị lập sổ, sheet chờ để mở | chat-5 đồng ý trên máy của họ (API); chat-4 chờ, không tải lại | web C1 | (a) sheet chờ tự đóng khi sổ mở | FAIL (còn: đồng ý qua API 200; thân màn đổi sau 575 ms, sang «hoi»; sheet lúc cuối: de-nghi; chuỗi khung (thân/sheet) sau đồng ý: chua-lap-so/cho×53@0 → hoi/de-nghi×132@575) | RUNTIME-WEB | EV-R-UI-084-A-C1 (ngoài git) [EV-R-F07-ghep](evidence/EV-R-F07-ghep.jpg) | UI-084 |
| TC-R-UI-084-B | F07 | F07.S02 | L23 | chat-6 đã đề nghị lập sổ (API) | chat-7: «Xem lời đề nghị» → «Đồng ý» | web C1 | (b) 0 khung trạng thái mời sau «Đồng ý», 0 khung thân thường trước bìa | FAIL (còn: sau chạm: 9 khung sheet mời «Đề nghị lập sổ», 0 khung thân «Tuần này» thường trước bìa; bìa M6: không; sổ mở trên máy chủ: có; chuỗi: chua-lap-so/dong-y×26@0 → hoi/de-nghi×9@128 → hoi/-×225@336) | RUNTIME-WEB | EV-R-UI-084-B-C1 (ngoài git) | UI-084 |
| TC-R-UI-084-B | F07 | F07.S02 | L23 | giảm chuyển động; chat-10 đã đề nghị lập sổ (API) | chat-11: «Xem lời đề nghị» → «Đồng ý» | web C9 | (b) 0 khung trạng thái mời sau «Đồng ý», 0 khung thân thường trước bìa | FAIL (còn: sau chạm: 2 khung sheet mời «Đề nghị lập sổ», 0 khung thân «Tuần này» thường trước bìa; bìa M6: không; sổ mở trên máy chủ: có; chuỗi: chua-lap-so/dong-y×25@0 → hoi/de-nghi×2@123 → hoi/-×237@143) | RUNTIME-WEB | EV-R-UI-084-B-C9 (ngoài git) | UI-084 |
| TC-R-UI-085 | F07 | F07.S02 | - | cặp đôi chat-8/chat-9, tuần này có tờ chốt: có (A đồng ý tờ của B: 200) | chat-8: chi tiết Tiệm Nướng Xóm Lào → «Rủ Chat Test 10 tới đây» | web C1 | tuần đã có tờ chốt: số tờ không đổi và câu giải thích khớp với màn; hoặc bản phác mới mang chỗ vừa chọn | FAIL (còn: tới /groups/[id]/to-giay?ru=1&cho=p-tiem-nuong-xom-lao; tờ 1 → 2 (mới: nhap, mang chỗ vừa chọn: không); đầu màn « Tờ giấy của hai mình Một đôi · Chat Test 10  Tuần này Chat Test 10 lo Chat Test 10 hay gửi tờ trước và đề nghị sửa. · Đổi Tuần này hai bạn đã có tờ rồi. Chỗ bạn chọn chưa được thêm, để dành cho tuần sau nhé. 18:30 Ăn tối Thứ Bảy 03/10 BẢN PHÁC  Nếp phác, b») | RUNTIME-WEB | EV-R-UI-085-C1 (ngoài git) [EV-R-F07-ghep](evidence/EV-R-F07-ghep.jpg) | UI-085 |
| TC-R-UI-086 | F07 | - | - | - | chưa đo lại trên main | web | Sau «Để sau», câu chờ nằm trên màn | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-086 |
| TC-R-UI-087 | F07 | - | - | - | chưa đo lại trên main | web | Back về tờ giấy: 0 hộp thoại | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-087 |
| TC-R-UI-088 | F07 | - | - | - | chưa đo lại trên main | web | Chạm đúp: 1 sheet | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-088 |
| TC-R-UI-089 | F07 | - | - | - | chưa đo lại trên main | web | axe `aria-prohibited-attr` = 0 trong sheet | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-089 |
| TC-R-UI-090 | F07 | - | - | - | chưa đo lại trên main | web | Hai tên đọc được, hoặc ít nhất khác nhau, ở 320–1024 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-090 |
| TC-R-UI-091 | F07 | - | - | - | chưa đo lại trên main | web | Sheet mở khi chưa gõ: không có nút tắt không lý do | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-091 |
| TC-R-UI-092 | F07 | - | - | - | chưa đo lại trên main | web | Lá đang chọn nằm trọn trong dải khi mở | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-092 |
| TC-R-UI-093 | F07 | - | - | - | chưa đo lại trên main | web | C6/C7: tờ giấy và nội dung sheet ≤ 640px | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-093 |

## F08

Đếm: PASS 1 · FAIL 3 · BLOCKED 0 · NOT_TESTED 9 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-094 | F08 | F08.S03 | L25 | album «Kèo album retest», 3 ô ảnh (3 ảnh tổng hợp) | chạm ảnh dẫn; vuốt trái giữa khung; chạm đúp; chụm hai ngón | web C1 | web C1: ảnh cao > 0, trọn trong vùng; một cú vuốt sang đúng ảnh sau và bộ đếm «2 / 3»; chạm đúp ra scale(2); chụm không đổi visualViewport.scale | FAIL (còn: ảnh 390×0, trọn trong khung; vuốt: cuộn 0 → 780 (một trang = 390), bộ đếm «1 / 3» → «1 / 3»; chạm đúp: translateX(0px) translateY(0px) scale(1); sau chụm visualViewport.scale 4.12) | RUNTIME-WEB | EV-R-UI-094-MO-C1 (ngoài git) EV-R-UI-094-C1 (ngoài git) [EV-R-F08-F09-ghep](evidence/EV-R-F08-F09-ghep.jpg) | UI-094 |
| TC-R-UI-095 | F08 | F08.S01 | - | tường nhóm chat-test, 3 ảnh; máy chủ trả 503 cho tim | chat-1 cuộn tới kỷ niệm cuối, chạm «Thả tim» | web C1 | tim lỗi ở bất kỳ kỷ niệm nào: câu lỗi nằm trong khung nhìn ngay sau khi chạm | FAIL (còn: chạm nút tim thứ 3; câu «Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút.» ở y -1140 (ngoài màn)) | RUNTIME-WEB | EV-R-UI-095-C1 (ngoài git) [EV-R-F08-F09-ghep](evidence/EV-R-F08-F09-ghep.jpg) | UI-095 |
| TC-R-UI-096 | F08 | F08.S08 | - | bài «Bạn bè» của Chat Test 01, 2 bình luận của Chat Test 02 | mở bài dưới hai vai; tìm nút xoá bình luận; nhấn giữ bình luận «Đẹp quá» | web C1 | một chạm vào thùng rác không xoá; vùng chạm ≥ 48dp | PASS (đổi: màn bài không còn nút xoá bình luận nào, nên không còn xoá một chạm; cũng không còn lối xoá bình luận trên màn này (API DELETE /posts/{id}/comments/{id} vẫn có): người viết bình luận: nút xoá không, hành động trên bình luận: Thích bình luận, Trả lời Chat Test 02, Gửi bình luận, nhấn giữ mở 0 hộp; tác giả bài: nút xoá không, hành động trên bình luận: Thích bình luận, Trả lời Chat Test 02, Gửi bình luận, nhấn giữ mở 0 hộp) | RUNTIME-WEB | EV-R-UI-096-C1 (ngoài git) [EV-R-F08-F09-ghep](evidence/EV-R-F08-F09-ghep.jpg) | UI-096 |
| TC-R-UI-097 | F08 | F08.S04 | L34 | «Thả khoảnh khắc» và «Đăng story» có ảnh và câu, chưa gửi | Back trình duyệt (rồi Forward) hoặc «Quay lại» → mở lại từ cùng nút | web C1 | bốn đường: có câu hỏi, hoặc mở lại còn ảnh và câu | FAIL (còn: moments-back: rời về /groups/[id]/wall, 0 câu hỏi, Forward trống, mở lại khung trống; moments-nut: rời về /groups/[id]/wall, 0 câu hỏi, mở lại khung trống; stories-back: rời về /messages, 0 câu hỏi, Forward trống, mở lại khung trống; stories-nut: rời về /messages, 0 câu hỏi, mở lại khung trống) | RUNTIME-WEB | EV-R-UI-097-C1 (ngoài git) [EV-R-F08-F09-ghep](evidence/EV-R-F08-F09-ghep.jpg) | UI-097 |
| TC-R-UI-098 | F08 | - | - | - | chưa đo lại trên main | web | C1: lúc đóng có ít nhất một mẫu độ mờ giữa 0 và 1; C9 vẫn mất ngay | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-098 |
| TC-R-UI-099 | F08 | - | - | - | chưa đo lại trên main | web | Mọi chip nằm trọn trong sheet ở C1, C2 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-099 |
| TC-R-UI-100 | F08 | - | - | - | chưa đo lại trên main | web | Mở bài không dành cho mình: một khối, không «Thử lại» | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-100 |
| TC-R-UI-101 | F08 | - | - | - | chưa đo lại trên main | web | axe 0 lỗi `aria-prohibited-attr` trên trình xem; focus vào câu hỏi khi nó hiện | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-101 |
| TC-R-UI-102 | F08 | - | - | - | chưa đo lại trên main | web | Ảnh 9:16: phần hiện ở xem trước và trên tường trùng nhau | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-102 |
| TC-R-UI-103 | F08 | - | - | - | chưa đo lại trên main | web | Kệ và đầu album có ngày của chuyến | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-103 |
| TC-R-UI-104 | F08 | - | - | - | chưa đo lại trên main | web | Tường ghi ngày như các màn khác | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-104 |
| TC-R-UI-105 | F08 | - | - | - | chưa đo lại trên main | web | C6/C7: cột nội dung ≤ 640px; ảnh dọc trên tường không cao hơn cửa sổ | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-105 |
| TC-R-UI-106 | F08 | - | - | - | chưa đo lại trên main | web | C2 lần đầu: không từ nào bị bẻ; cột chữ ≥ 120px | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-106 |

## F09

Đếm: PASS 0 · FAIL 1 · BLOCKED 0 · NOT_TESTED 5 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-015 | F09 | - | - | - | chưa đo lại trên main | web | Không khung nào hiện tên/giá trị giữ chỗ | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-015 |
| TC-R-UI-107 | F09 | F09.S02 | - | Cài đặt của chat-2; máy chủ trả 503 cho PATCH /people/me | chạm công tắc «Cho tìm theo số điện thoại» | web C1 | lỗi khi lưu công tắc, chip hay ảnh: câu nằm trong khung nhìn, sát mục vừa chạm | FAIL (còn: công tắc ở y 422; câu «Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút.» ở y 1124 (ngoài màn)) | RUNTIME-WEB | EV-R-UI-107-C1 (ngoài git) [EV-R-F08-F09-ghep](evidence/EV-R-F08-F09-ghep.jpg) | UI-107 |
| TC-R-UI-108 | F09 | - | - | - | chưa đo lại trên main | web | Cả hai đường A và B: Back khi panel hay form mở thì ở lại Cá nhân và đóng panel; chữ đang gõ không mất mà không hỏi | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-108 |
| TC-R-UI-109 | F09 | - | - | - | chưa đo lại trên main | web | Từ «Đã lưu» mở được từng chỗ đã lưu | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-109 |
| TC-R-UI-110 | F09 | - | - | - | chưa đo lại trên main | web | Gõ «XOÁ» thì bật nút, hoặc có câu lý do ngay dưới nút; Back ở bước 2 về bước 1 | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-110 |
| TC-R-UI-111 | F09 | - | - | - | chưa đo lại trên main | web | Câu chỉ đúng chỗ đổi tên | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-111 |

## F10

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-115 | F10 | - | - | - | chưa đo lại trên main | web | Ở C1, kéo dọc bắt đầu trên tranh cuộn danh sách như bắt đầu ở chỗ khác | NOT_TESTED (chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau) | RUNTIME-WEB |  | UI-115 |

## F11

Đếm: PASS 0 · FAIL 3 · BLOCKED 1 · NOT_TESTED 0 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-R-UI-082-F11 | F11 | F11.S01 | - | chưa đăng nhập | mở thẳng năm tab và hai route demo «Lịch trình AI», «Ai dùng món nào?» | web C1 | F11: mọi màn demo có nhãn «Dữ liệu demo» và một lối «Đăng nhập» nhìn thấy được | FAIL (còn: Cộng đồng → /community, nhãn không, lối vào Đăng nhập; Khám phá → /explore, nhãn Dữ liệu demo, lối vào không; Lên plan → /plan, nhãn Dữ liệu demo, lối vào không; Tin nhắn → /messages, nhãn không, lối vào không; Cá nhân → /profile, nhãn Dữ liệu demo, lối vào không; Lịch trình AI → /trips/team-da-lat/itinerary, nhãn Nháp, lối vào không; Ai dùng món nào? → /smart-split/team-da-lat/assignment, nhãn Nháp, lối vào không) | RUNTIME-WEB | EV-R-UI-082-F11-C1 (ngoài git) [EV-R-F10-F11-E-ghep](evidence/EV-R-F10-F11-E-ghep.jpg) | UI-082 |
| TC-R-UI-116 | F11 | F11.S01 | - | chưa đăng nhập, tab Tin nhắn demo | mở /messages, đọc bong bóng «Plan xịn đó, mình bình chọn chỗ BBQ trước đi.» | web C1–C3 | /messages không phiên ở C1–C3: 0 chữ bị cắt trong bong bóng, câu dài xuống dòng | FAIL (còn: C1 ở /messages: câu 1 dòng, thừa 22px; chữ tràn mép phải 2 («Cuối tuần tháng 10 đi Đà Lạt…» thừa 32px, «Plan xịn đó, mình bình chọn …» thừa 22px); C2 ở /messages: câu 1 dòng, thừa 92px; chữ tràn mép phải 3 («Cuối tuần tháng 10 đi Đà Lạt…» thừa 102px, «Đi chứ! Tớ vote săn mây với …» thừa 49px); C3 ở /messages: câu 1 dòng, thừa 52px; chữ tràn mép phải 3 («Cuối tuần tháng 10 đi Đà Lạt…» thừa 62px, «Đi chứ! Tớ vote săn mây với …» thừa 9px)) | RUNTIME-WEB | EV-R-UI-116-C2 (ngoài git) [EV-R-F10-F11-E-ghep](evidence/EV-R-F10-F11-E-ghep.jpg) | UI-116 |
| TC-R-UI-117-A | F11 | F11.S01 | L28 | chưa đăng nhập; từ Khám phá chạm tab «Lên plan», mở «Tùy chọn» | Back trình duyệt; rồi, mỗi lần từ một lượt mới: chạm tim quán đầu, chạm thẻ «Bánh căn Lệ», chạm tab «Lên plan» | web C1 | sau Back không còn vùng inert nào ngoài hộp thoại; chạm tim lần đầu đổi thành «Bỏ lưu»; chạm thẻ quán mở đúng quán; thanh tab phản hồi | BLOCKED (đổi: Back rời app (UI-123), không tới được Khám phá nên tiêu chí không đo được trên đường này; xem TC-R-UI-117-B: sau Back ở about:blank, vùng inert ≥25% màn 0; tim «-» không thấy; chạm - (người dùng thấy «-») → blank; tab «Lên plan»: blank 0 hộp → blank 0 hộp) | RUNTIME-WEB | EV-R-UI-117-C1 (ngoài git) | UI-117 |
| TC-R-UI-117-B | F11 | F11.S01 | L28 | chưa đăng nhập; mở /community, chạm tab «Khám phá» rồi «Lên plan», mở «Tùy chọn» | Back trình duyệt; rồi chạm tab «Khám phá» | web C1 | sau Back không còn vùng inert nào ngoài hộp thoại; thanh tab phản hồi | FAIL (còn: Khám phá → /explore, Lên plan → /plan; sheet mở: có; sau Back ở /community, 1 hộp thoại thấy được, vùng inert ≥25% màn 8, tab «Khám phá» nằm trong vùng inert; chạm tab «Khám phá» → /community) | RUNTIME-WEB | EV-R-UI-117-B-C1 (ngoài git) [EV-R-F10-F11-E-ghep](evidence/EV-R-F10-F11-E-ghep.jpg) | UI-117 |

## N14

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-N-14-CONG-DONG | N14 | - | - | - | audit feature mới: tab Cộng đồng: bảng tin kiểm duyệt, đăng bài, thông báo, realtime (#14) | web | audit đủ chuỗi feature → màn → lớp → trạng thái như audit gốc | NOT_TESTED (feature mới trên main, trong hàng đợi sau retest; chưa audit) | RUNTIME-WEB |  |  |

## N15

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-N-15-NHAT-KY | N15 | - | - | - | audit feature mới: Sổ kỷ niệm Nếp v3 / Nhật ký chuyến, màn kết thúc kèo (#657, #15) | web | audit đủ chuỗi feature → màn → lớp → trạng thái như audit gốc | NOT_TESTED (feature mới trên main, trong hàng đợi sau retest; chưa audit) | RUNTIME-WEB |  |  |

## N21

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-N-21-HO-SO | N21 | - | - | - | audit feature mới: hồ sơ kể chuyện, sổ huy hiệu nhiều ngã rẽ, tường cá nhân v2 (#658, #21) | web | audit đủ chuỗi feature → màn → lớp → trạng thái như audit gốc | NOT_TESTED (feature mới trên main, trong hàng đợi sau retest; chưa audit) | RUNTIME-WEB |  |  |

## N22

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-N-22-AI-CHAT | N22 | - | - | - | audit feature mới: Rủ Đi AI trong chat, chat hai người (#654, #659, #22) | web | audit đủ chuỗi feature → màn → lớp → trạng thái như audit gốc | NOT_TESTED (feature mới trên main, trong hàng đợi sau retest; chưa audit) | RUNTIME-WEB |  |  |

## N26

Đếm: PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 1 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-N-26-HAI-LOP-CHAT | N26 | - | - | - | audit feature mới: hai lớp chat «đám bạn» / «cặp đôi» (#660, #26) | web | audit đủ chuỗi feature → màn → lớp → trạng thái như audit gốc | NOT_TESTED (feature mới trên main, trong hàng đợi sau retest; chưa audit) | RUNTIME-WEB |  |  |

## Hàng đã rút

Hàng do lỗi của harness (sai selector, sai tên nút) được rút khỏi bảng trên; sổ vẫn giữ nguyên dòng gốc.

| ID | Số hàng rút | Lý do |
|---|---|---|
| TC-R-UI-113 | 1 | lỗi harness: bộ tìm chip «Không ảnh» không bỏ ký tự icon trong tên nên không chạm chip (số đo lấy ở trạng thái mặc định của bảng); đo lại |
| TC-F06-DUOC-MOI-VAO-CUA | 1 | lượt chạy lại kịch bản F06 gốc trên main: bước mời lại không gửi lời mời nào cho moi-53, nên hàng này đo trên một số chưa được mời; UI-073 và UI-074 đo lại ở retest-main.mjs (r-f06) |
| TC-F05.S01-DONG-Y | 1 | lượt chạy lại kịch bản F06 gốc trên main: bước mời lại không gửi lời mời nào cho moi-53, nên hàng này đo trên một số chưa được mời; UI-073 và UI-074 đo lại ở retest-main.mjs (r-f06) |
| TC-F06-TU-BO-QUAN-TRI | 1 | lượt chạy lại kịch bản F06 gốc trên main: bước mời lại không gửi lời mời nào cho moi-53, nên hàng này đo trên một số chưa được mời; UI-073 và UI-074 đo lại ở retest-main.mjs (r-f06) |
| TC-F06-MOI-LAI | 1 | lượt chạy lại kịch bản F06 gốc trên main: không phân định được lỗi app hay kịch bản cũ lệch với màn Mời của main (không thấy câu sau lần mời lại); không dùng làm bằng chứng |
| TC-R-UI-117 | 1 | thay bằng TC-R-UI-117-A (đường cũ, Back rời app: UI-123) và TC-R-UI-117-B (bắt đầu ở tab đầu Cộng đồng) |

