# Coverage matrix: audit UI/UX app mobile RuDi

Sinh bởi `tests/qa/mobile-ui-audit/tong-hop.mjs` từ sổ `results.jsonl`; không sửa tay. Mỗi hàng là một
(test case × nền tảng × nhóm cấu hình). Cấu hình C1–C9 ở `report.md` §A. Method: RUNTIME-WEB là đã chạy
trên bản web trong Chromium; STATIC là chỉ đọc mã; HYPOTHESIS là nghi vấn chưa kiểm chứng. PASS chỉ có
ở hàng RUNTIME-WEB.

## Đếm

| Phạm vi | Đếm |
|---|---|
| Tất cả (339 hàng) | PASS 51 · FAIL 25 · BLOCKED 180 · NOT_TESTED 82 · NOT_APPLICABLE 1 |
| Web (Chromium) | PASS 51 · FAIL 25 · BLOCKED 60 · NOT_TESTED 82 · NOT_APPLICABLE 1 |
| Android native | PASS 0 · FAIL 0 · BLOCKED 60 · NOT_TESTED 0 · NOT_APPLICABLE 0 |
| iOS native | PASS 0 · FAIL 0 · BLOCKED 60 · NOT_TESTED 0 · NOT_APPLICABLE 0 |
| Method RUNTIME-WEB | PASS 51 · FAIL 25 · BLOCKED 0 · NOT_TESTED 82 · NOT_APPLICABLE 1 |
| Method STATIC | PASS 0 · FAIL 0 · BLOCKED 180 · NOT_TESTED 0 · NOT_APPLICABLE 0 |
| Method HYPOTHESIS | PASS 0 · FAIL 0 · BLOCKED 0 · NOT_TESTED 0 · NOT_APPLICABLE 0 |

## F00

Đếm: PASS 33 · FAIL 17 · BLOCKED 15 · NOT_TESTED 4 · NOT_APPLICABLE 1

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F00-CREATE-LANH | F00 | F00.S04 | L01 | mở lạnh bằng link | mở /create | web C1 | khay mở trên tab Khám phá (theo chú thích app/create.tsx) | FAIL (history: replace /explore, không có push /create; 0 dialog suốt 3 s; cả khi có và không có phiên) | RUNTIME-WEB | [EV-F00-CREATE-LANH-C1](evidence/EV-F00-CREATE-LANH-C1.jpg) | UI-010 |
| TC-F00-DT-chat-20 | F00 | F00.S01 | - | người chưa có nhóm | mở / | web C1 | đi tới /messages | PASS (tới /messages) | RUNTIME-WEB | EV-F00-DT-chat-20-goc (ngoài git) |  |
| TC-F00-DT-dalat-0 | F00 | F00.S01 | - | người có nhóm đang hoạt động | mở / | web C1 | đi tới /explore | PASS (tới /explore) | RUNTIME-WEB | EV-F00-DT-dalat-0-goc (ngoài git) |  |
| TC-F00-DT-goc-chua-dang-nhap | F00 | F00.S01 |  | chưa đăng nhập | mở / | web C1,C2,C3 | về /welcome | PASS | RUNTIME-WEB | EV-F00.S01-BASE-C1 (ngoài git) |  |
| TC-F00-DT-la-chua-dang-nhap | F00 | F00.S02 |  | chưa đăng nhập | mở /khong-co-trang-nay | web C1,C2,C3 | về /welcome | PASS (đúng hợp đồng deep link cũ) | RUNTIME-WEB | EV-F00.S02-BASE-C1 (ngoài git) |  |
| TC-F00-LA-DANG-NHAP | F00 | F00.S02 | - | URL lạ khi đã đăng nhập | mở /khong-co-trang-nay | web C1 | không lạc vào màn chào/đăng nhập khi đã có phiên | FAIL (tới /welcome) | RUNTIME-WEB | [EV-F00-DT-dalat-0-la](evidence/EV-F00-DT-dalat-0-la.jpg) |  |
| TC-F00-RAIL | F00 | F00.S03 | L05 | cửa sổ ≥600dp | mở /plan, /messages, /profile | web B600,C6,B839,B840,C7 | rail trái; vạch chỉ báo cạnh tab đang chọn | FAIL (rail đúng từ 600dp; vạch lệch: Tin nhắn ở y156–204, vạch y216–288; Cá nhân y204–252, vạch y288–360) | RUNTIME-WEB | [EV-F00-RAIL-chibao-cat](evidence/EV-F00-RAIL-chibao-cat.jpg) [EV-F00-TAB-ghep-b](evidence/EV-F00-TAB-ghep-b.jpg) | UI-004 |
| TC-F00-RESUME-CHAM | F00 | F00.S01 | L35 | khôi phục phiên chậm (trễ 5 s giả lập ở /sessions/web/resume) | mở /plan lạnh | web C1 | có chỉ báo đang tải trong lúc chờ, không để vùng nội dung trống | FAIL (1,8 s: vùng nội dung trống hoàn toàn, chỉ có thanh tab và mép Nếp; 0 progressbar. 44 ký tự đếm được là nhãn tab) | RUNTIME-WEB | [EV-F00-RESUME-CHAM-1800ms-C1](evidence/EV-F00-RESUME-CHAM-1800ms-C1.jpg) EV-F00-RESUME-CHAM-6800ms-C1 (ngoài git) | UI-009 |
| TC-F00-TAB-A11Y | F00 | F00.S03 | L05 | 4 tab | đọc cây truy cập của thanh tab | web C1,C2,C3,C6,C7 | tab đang chọn có aria-selected=true; các tab nằm trong role=tablist | FAIL (aria-selected vắng ở cả 4 tab, không có tablist, ở mọi cấu hình đo) | RUNTIME-WEB | EV-F00-TAB-C1 (ngoài git) | UI-003 |
| TC-F00-TAB-DOI | F00 | F00.S03 | L05 | 4 tab | chạm lần lượt 4 tab | web C1 | URL và nội dung đổi theo tab được chạm; tab đang chọn có dấu hiệu nhìn thấy (màu + icon đặc + dải washi) | PASS (Lên plan→/plan, Tin nhắn→/messages, Cá nhân→/profile, Khám phá→/explore) | RUNTIME-WEB | EV-F00-TAB-C1 (ngoài git) |  |
| TC-F00.S01-BASE | F00 | F00.S01 | - | baseline | mở / | web C1,C2,C3 | chuyển đúng theo phiên, không trang lỗi | PASS (chưa đăng nhập → /welcome; có nhóm → /explore; không nhóm → /messages) | RUNTIME-WEB | EV-F00.S01-BASE-C1 (ngoài git) |  |
| TC-F00.S01-FONT | F00 | F00.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở / | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F00.S01-NATIVE | F00 | F00.S01 | - | mọi trạng thái | mở / | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F00.S01-NATIVE | F00 | F00.S01 | - | mọi trạng thái | mở / | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F00.S02-BASE | F00 | F00.S02 | - | baseline | mở URL lạ | web C1,C2,C3 | không trang lỗi, không lạc | FAIL (chưa đăng nhập: đúng /welcome; đã đăng nhập: cũng /welcome) | RUNTIME-WEB | [EV-F00-DT-dalat-0-la](evidence/EV-F00-DT-dalat-0-la.jpg) | UI-002 |
| TC-F00.S02-FONT | F00 | F00.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở [...legacy] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F00.S02-NATIVE | F00 | F00.S02 | - | mọi trạng thái | mở [...legacy] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F00.S02-NATIVE | F00 | F00.S02 | - | mọi trạng thái | mở [...legacy] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F00.S03-BASE | F00 | F00.S03 | L05 | baseline Khám phá | nhìn thanh tab | web C1,C2,C3 | 4 nhãn một dòng, nút «+» giữa, tab chọn phân biệt bằng màu + icon đặc + dải washi | PASS (đã mở ảnh C1, C2, C3, biên 599; lỗi a11y và rail ghi riêng (UI-003, UI-004)) | RUNTIME-WEB | [EV-F00-TAB-ghep-a](evidence/EV-F00-TAB-ghep-a.jpg) |  |
| TC-F00.S03-FONT | F00 | F00.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở (tabs) | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F00.S03-NATIVE | F00 | F00.S03 | - | mọi trạng thái | mở (tabs) | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F00.S03-NATIVE | F00 | F00.S03 | - | mọi trạng thái | mở (tabs) | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F00.S04-BASE | F00 | F00.S04 | L01 | khay mở | mở khay | web C1,C3 | khay đủ 5 việc, tiêu đề, nút đóng, không tràn | PASS (C1 cao 75%, C3 80%) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) EV-F00-KT-mo-C3 (ngoài git) |  |
| TC-F00.S04-BASE | F00 | F00.S04 | L01 | khay mở | mở khay | web C2 | khay đủ 5 việc, cao ≤82% | FAIL (92%) | RUNTIME-WEB | [EV-F00-KT-mo-C2](evidence/EV-F00-KT-mo-C2.jpg) | UI-007 |
| TC-F00.S04-FONT | F00 | F00.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /create | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F00.S04-NATIVE | F00 | F00.S04 | - | mọi trạng thái | mở /create | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F00.S04-NATIVE | F00 | F00.S04 | - | mọi trạng thái | mở /create | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F00.S05-BASE | F00 | F00.S05 | L02 | Khám phá, Nếp thu | nhìn mép phải | web C1,C3 | mép 10dp trong lề, không che chữ | PASS | RUNTIME-WEB | [EV-F00-NEP-mep-C1](evidence/EV-F00-NEP-mep-C1.jpg) |  |
| TC-F00.S05-BASE | F00 | F00.S05 | L02 | Khám phá, Nếp thu | nhìn mép phải | web C2 | mép 10dp trong lề, không che chữ | FAIL (mép x310–320 đè chữ chip «Vui chơi» trong hàng chip tràn mép) | RUNTIME-WEB | EV-F00-TAB-C2 (ngoài git) | UI-014 |
| TC-F00.S05-FONT | F00 | F00.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở (toàn cục) | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F00.S05-NATIVE | F00 | F00.S05 | - | mọi trạng thái | mở (toàn cục) | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F00.S05-NATIVE | F00 | F00.S05 | - | mọi trạng thái | mở (toàn cục) | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L01-CHAM-DUP | F00 | F00.S04 | L01 | khay đóng | chạm «+» hai lần cách 60ms | web C1 | mở đúng một khay, không kích hoạt gì trong khay | FAIL (chạm thứ hai rơi vào thẻ trong khay đang trượt lên; điều hướng tới /hai-nguoi/chon-nguoi) | RUNTIME-WEB | [EV-F00-KT-cham-dup-C1](evidence/EV-F00-KT-cham-dup-C1.jpg) | UI-006 |
| TC-L01-DONG-back | F00 | F00.S04 | L01 | khay đang mở | Back trình duyệt | web C1 | đóng hẳn, không còn phần tử inert/aria-hidden trên trang | FAIL (khay đóng, chạm vẫn được, nhưng explore-screen (gồm cả thanh tab) giữ aria-hidden=true) | RUNTIME-WEB | EV-F00-KT-back-sau-C1 (ngoài git) | UI-005 |
| TC-L01-DONG-esc | F00 | F00.S04 | L01 | khay đang mở | đóng bằng esc | web C1 | đóng hẳn, không sót lớp chặn, không còn inert | PASS (mở 18 ms; còn 0 dialog; lưới đổi 0; inert 0; focus sau: Tạo mới; /explore) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L01-DONG-fling | F00 | F00.S04 | L01 | khay đang mở | vuốt nhanh tay cầm dưới 90dp | web C1 | đóng khi nhanh hơn 900dp/s; ở lại khi chậm | PASS (sự kiện chạm mang timestamp giả lập: 60dp ở 1250px/s đóng; 40dp ở 825px/s và 80dp ở 250px/s ở lại. Lần đo đầu (không timestamp) cho kết quả sai vì độ trễ CDP 30–125ms/sự kiện) | RUNTIME-WEB |  |  |
| TC-L01-DONG-keo-dai | F00 | F00.S04 | L01 | khay đang mở | đóng bằng keo-dai | web C1 | đóng hẳn, không sót lớp chặn, không còn inert | PASS (mở 29 ms; còn 0 dialog; lưới đổi 0; inert 0; focus sau: Tạo mới; /explore) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L01-DONG-keo-ngan | F00 | F00.S04 | L01 | khay đang mở | đóng bằng keo-ngan | web C1 | bật về, vẫn mở | PASS (mở 15 ms; còn 1 dialog; lưới đổi 12; inert 2; focus sau: Đóng bảng; /create) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L01-DONG-nen | F00 | F00.S04 | L01 | khay đang mở | đóng bằng nen | web C1 | đóng hẳn, không sót lớp chặn, không còn inert | PASS (mở 21 ms; còn 0 dialog; lưới đổi 0; inert 0; focus sau: Tạo mới; /explore) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L01-DONG-x | F00 | F00.S04 | L01 | khay đang mở | đóng bằng x | web C1 | đóng hẳn, không sót lớp chặn, không còn inert | PASS (mở 12 ms; còn 0 dialog; lưới đổi 0; inert 0; focus sau: Tạo mới; /explore) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L01-FOCUS | F00 | F00.S04 | L01 | mở khay | mở rồi đóng bằng X | web C1 | focus vào trong khay khi mở; trả về nút «Tạo mới» khi đóng | PASS (khi mở: {"ten":"Đóng bảng","trongDialog":true,"tag":"button","testid":"create-sheet"}; sau khi đóng: {"ten":"Tạo mới","trongDialog":false,"tag":"button","testid":null}) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L01-KICH-C2 | F00 | F00.S04 | L01 | khay mở | mở khay | web C2 | panel ≤82% chiều cao (DESIGN), nút đóng trong màn | FAIL (panel 589/640 = 92%; nút đóng vẫn thấy) | RUNTIME-WEB | [EV-F00-KT-mo-C2](evidence/EV-F00-KT-mo-C2.jpg) | UI-007 |
| TC-L01-KICH-C3 | F00 | F00.S04 | L01 | khay mở | mở khay | web C3 | panel ≤82% cao, nút đóng trong màn, nội dung dài cuộn được, không tràn | PASS (panel top 163, cao 637/800, rộng 360/360, nút đóng top 163, cuộn false) | RUNTIME-WEB | EV-F00-KT-mo-C3 (ngoài git) |  |
| TC-L01-KICH-C6 | F00 | F00.S04 | L01 | khay mở | mở khay | web C6 | panel ≤82% cao, nút đóng trong màn, nội dung dài cuộn được, không tràn | PASS (panel top 412, cao 612/1024, rộng 768/768, nút đóng top 412, cuộn false) | RUNTIME-WEB | EV-F00-KT-mo-C6 (ngoài git) |  |
| TC-L01-KICH-C7 | F00 | F00.S04 | L01 | khay mở | mở khay | web C7 | panel ≤82% cao, nút đóng trong màn, nội dung dài cuộn được, không tràn | PASS (panel top 754, cao 612/1366, rộng 1024/1024, nút đóng top 754, cuộn false) | RUNTIME-WEB | [EV-F00-KT-mo-C7](evidence/EV-F00-KT-mo-C7.jpg) |  |
| TC-L01-KICH-C8 | F00 | F00.S04 | L01 | khay mở | mở khay | web C8 | panel ≤82% chiều cao, còn nền để chạm ra ngoài | FAIL (panel 441/460 = 96%, chỉ còn 19px nền) | RUNTIME-WEB | [EV-F00-KT-mo-C8](evidence/EV-F00-KT-mo-C8.jpg) | UI-007 |
| TC-L01-NGAT | F00 | F00.S04 | L01 | khay đang mở dở | Esc 50ms sau khi chạm «+», rồi mở lại | web C1 | đóng sạch; mở lại được | PASS (dialog biến mất sau khoảng 0,6s, về /explore; mở lại sau 13ms. Lần FAIL trước là hệ quả của ca chạm đúp chạy trên cùng trang) | RUNTIME-WEB | EV-F00-KT-ngat-molai-C1 (ngoài git) |  |
| TC-L01-TAB-THU-TU | F00 | F00.S04 | L01 | khay mở | phím Tab 9 lần | web C1 | mọi điểm dừng focus đều có tên; thứ tự theo thị giác | FAIL (điểm dừng đầu tiên là khối Nếp M1 112×112, không role, không tên; sau đó 5 thẻ, rồi «Đóng bảng»; vòng lặp trong khay đúng) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) | UI-008 |
| TC-L01-VONGDOI | F00 | F00.S04 | L01 | đóng → mở → dùng → đóng → mở lại | vòng đời khay tạo | web C1 | mở đúng chỗ, đóng hết mọi cách, không sót, focus trả về | FAIL (X, nền, Esc, kéo dài, fling, kéo ngắn bật về, ngắt giữa chừng, focus: đạt; Back để sót aria-hidden; chạm đúp kích hoạt nhầm) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) | UI-005, UI-006 |
| TC-L02-AN-KHI-SHEET | F00 | F00.S05 | L02 | khay tạo mở | nhìn lề phải | web C1 | Nếp không vẽ trên sheet đang mở | PASS (không có) | RUNTIME-WEB | [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) |  |
| TC-L02-CHAM-KHONG-CUON | F00 | F00.S05 | L02 | Nếp thu | chạm mép | web C1 | Nếp rút ra; trang không cuộn ngang (ADR-0035) | PASS (rút ra 56×64; scrollX 0→0) | RUNTIME-WEB | EV-F00-NEP-rut-C1 (ngoài git) |  |
| TC-L02-MEP | F00 | F00.S05 | L02 | Khám phá, Nếp thu | nhìn mép | web C1 | chỉ lộ mép hẹp trong lề phải, không che chữ | PASS (lộ 10dp, cao 64; chữ bị che: 0) | RUNTIME-WEB | [EV-F00-NEP-mep-C1](evidence/EV-F00-NEP-mep-C1.jpg) |  |
| TC-L02-TU-THU | F00 | F00.S05 | L02 | Nếp rút ra | chờ 6,6 s | web C1 | tự thu vào mép sau 6 s | PASS (đã thu) | RUNTIME-WEB |  |  |
| TC-L02-VONGDOI | F00 | F00.S05 | L02 | thu → rút → tự thu | chạm mép, chờ 6,6 s | web C1 | rút ra 56×64, không cuộn ngang, tự thu, ẩn khi có sheet | PASS | RUNTIME-WEB | EV-F00-NEP-rut-C1 (ngoài git) |  |
| TC-L03-CHIP | F00 | F00.S05 | L03 | bảng Nếp mở | đo chip gợi ý | web C1 | chip ≥48dp (DESIGN) | FAIL (3 chip 150–184 × 36) | RUNTIME-WEB | [EV-F00-NEP-BANG-C1](evidence/EV-F00-NEP-BANG-C1.jpg) | UI-012 |
| TC-L03-DONG-esc | F00 | F00.S05 | L03 | bảng Nếp mở | Esc | web C1 | đóng, không sót inert/aria-hidden | PASS (sót 0) | RUNTIME-WEB |  |  |
| TC-L03-MO-C1 | F00 | F00.S05 | L03 | Khám phá | chạm mép Nếp, chạm Nếp | web C1 | bảng Nếp mở; ô hỏi nằm trong màn | PASS (mở 33 ms; panel top 463, cao 381/844; ô hỏi 782–826) | RUNTIME-WEB | [EV-F00-NEP-BANG-C1](evidence/EV-F00-NEP-BANG-C1.jpg) |  |
| TC-L03-MO-C3 | F00 | F00.S05 | L03 | Khám phá | chạm mép Nếp, chạm Nếp | web C3 | bảng Nếp mở; ô hỏi nằm trong màn | PASS (mở 29 ms; panel top 419, cao 381/800; ô hỏi 738–782) | RUNTIME-WEB | EV-F00-NEP-BANG-C3 (ngoài git) |  |
| TC-L03-MO-C8 | F00 | F00.S05 | L03 | Khám phá | chạm mép Nếp, chạm Nếp | web C8 | bảng Nếp mở; ô hỏi nằm trong màn | PASS (mở 35 ms; panel top 79, cao 381/460; ô hỏi 398–442) | RUNTIME-WEB | [EV-F00-NEP-BANG-C8](evidence/EV-F00-NEP-BANG-C8.jpg) |  |
| TC-L03-VE | F00 | F00.S05 | L03 | bảng Nếp, đã gõ mô tả | bấm «Vẽ» | web C1 | hỏi xác nhận rồi vẽ, hoặc nói vì sao không vẽ được | FAIL (không dialog, không alert, không request, không chữ trạng thái) | RUNTIME-WEB | [EV-F00-NEP-VE-C1](evidence/EV-F00-NEP-VE-C1.jpg) | UI-011 |
| TC-L03-VONGDOI | F00 | F00.S05 | L03 | đóng → mở → Esc | mở bảng Nếp từ mép, đóng bằng Esc | web C1,C3,C8 | mở, ô hỏi trong màn, đóng không sót | PASS (nút «Vẽ» ghi riêng (UI-011); chip gợi ý 36dp (UI-012)) | RUNTIME-WEB | [EV-F00-NEP-BANG-C1](evidence/EV-F00-NEP-BANG-C1.jpg) [EV-F00-NEP-BANG-C8](evidence/EV-F00-NEP-BANG-C8.jpg) |  |
| TC-L04-VONGDOI | F00 | (nhiều màn) | L04 | M1 lần 2 trong một phiên | mở khay tạo hai lần | web C1 | M1 chỉ diễn một lần mỗi phiên | NOT_TESTED (đã quay khung hai lần mở; tư thế Nếp đổi ở cả hai lần, khung hình không đủ để tách diễn lại M1 khỏi tư thế nghỉ; cần đọc trạng thái useKhoanhKhac hoặc thiết bị) | RUNTIME-WEB | [EV-F00-M1-lan1-C1](evidence/EV-F00-M1-lan1-C1.jpg) [EV-F00-M1-lan2-C1](evidence/EV-F00-M1-lan2-C1.jpg) |  |
| TC-L05-VONGDOI | F00 | F00.S03 | L05 | đóng → mở → dùng → đóng → mở lại | vòng đời Thanh tab / rail | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L29-VONGDOI | F00 | (nhiều màn) | L29 | đóng → mở → dùng → đóng → mở lại | vòng đời Nắp gấp NapGiay | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L35-VONGDOI | F00 | (mọi màn) | L35 | đóng → mở → dùng → đóng → mở lại | vòng đời Kit Skeleton / ErrorState / EmptyState | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-MO01 | F00 | (stack) | - | push Cá nhân → Cài đặt | chạm «Cài đặt» | web C1,C9 | trượt từ phải (C1), cắt thẳng (C9) | NOT_APPLICABLE (trên web native-stack không có hoạt ảnh: màn mới ở x=0 ngay khung đầu ở cả C1 và C9; không khung trắng. Hoạt ảnh native: BLOCKED) | RUNTIME-WEB | [EV-F00-MO01-push-C1](evidence/EV-F00-MO01-push-C1.jpg) |  |
| TC-MO02 | F00 | F00.S03 | L05 | đổi tab | Khám phá → Cá nhân | web C1,C9 | dải washi trượt tới tab mới; C9 tức thì | PASS (C1: x 0→312, 1 bản, không đổi chiều; C9: tới nơi sau khoảng 108 ms (1 khung)) | RUNTIME-WEB |  |  |
| TC-MO04-DONG | F00 | F00.S04 | L01 | đóng khay | chạm X | web C1 | panel ra khỏi màn rồi mới gỡ; không bật mất | FAIL (khung cuối còn thấy: y=667 (lộ 177px), op 1, rồi biến mất) | RUNTIME-WEB | [EV-F00-MO04-dong-khay-C1](evidence/EV-F00-MO04-dong-khay-C1.jpg) | UI-013 |
| TC-MO04-DONG | F00 | F00.S04 | L01 | đóng khay | chạm X | web C9 | cắt thẳng | PASS (biến mất ngay từ y=207) | RUNTIME-WEB |  |  |
| TC-MO04-MO | F00 | F00.S04 | L01 | mở khay | chạm «+» | web C1,C9 | lò xo lên rồi đứng; C9 hiện ngay | PASS (C1: y 687→207, 1 lần vượt đích; C9: 6 ms. Khung đầu đã lộ 157px đỉnh panel vì dịch cố định 480px (xem UI-013)) | RUNTIME-WEB | [EV-F00-MO04-mo-khay-C1](evidence/EV-F00-MO04-mo-khay-C1.jpg) |  |

## F01

Đếm: PASS 18 · FAIL 7 · BLOCKED 15 · NOT_TESTED 0 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F01-LOGIN-503 | F01 | F01.S02 | - | máy chủ trả 503 khi gửi mã | «Gửi mã» | web C1 | câu lỗi tiếng Việt, không lộ mã lỗi, gửi lại được | PASS («Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút.») | RUNTIME-WEB | [EV-F01-LOGIN-503-C1](evidence/EV-F01-LOGIN-503-C1.jpg) |  |
| TC-F01-LOGIN-BACK-LANH | F01 | F01.S02 | - | mở /login bằng link (không lịch sử) | chạm «Quay lại» | web C1 | về màn trước hợp lý (Welcome), không đứng im | FAIL (sau khi chạm vẫn /login) | RUNTIME-WEB | [EV-F01-LOGIN-back-lanh-C1](evidence/EV-F01-LOGIN-back-lanh-C1.jpg) | UI-018 |
| TC-F01-LOGIN-C8 | F01 | F01.S02 | - | cửa sổ 390×460 | mở /login | web C8 | ô số và «Gửi mã» trong màn | PASS (nút 378–430/460) | RUNTIME-WEB | [EV-F01-LOGIN-C8](evidence/EV-F01-LOGIN-C8.jpg) |  |
| TC-F01-LOGIN-SO-SAI | F01 | F01.S02 | - | gõ 12345 | «Gửi mã» | web C1 | một câu lỗi tiếng Việt dưới ô, không gửi | PASS («Chưa đúng dạng số di động Việt Nam: 10 chữ số, bắt đầu bằng 0.») | RUNTIME-WEB | [EV-F01-LOGIN-so-sai-C1](evidence/EV-F01-LOGIN-so-sai-C1.jpg) |  |
| TC-F01-MOI-SAI | F01 | F01.S04 | - | mã lời mời sai | «Nhận lời mời» | web C1 | câu nói đúng chuyện: mã không đúng hoặc đã hết hạn | FAIL (máy chủ: 404 code invite_not_found; app: «Phần này chưa mở được trên bản app này. Cập nhật app rồi thử lại.») | RUNTIME-WEB | [EV-F01-MOI-sai-C1](evidence/EV-F01-MOI-sai-C1.jpg) | UI-019 |
| TC-F01-OTP-BACK | F01 | F01.S03 | - | đang ở OTP | Back trình duyệt | web C1 | về Đăng nhập, giữ số đã nhập (giống «Đổi số») | PASS (tới /login; ô số còn 10 ký tự) | RUNTIME-WEB |  |  |
| TC-F01-OTP-DUNG | F01 | F01.S03 | - | mã đúng, tài khoản mới | gõ 000000 | web C1 | vào Sở thích (người mới) | PASS (tới /personalization) | RUNTIME-WEB | [EV-F01-SO-THICH-moi-C1](evidence/EV-F01-SO-THICH-moi-C1.jpg) |  |
| TC-F01-OTP-SAI | F01 | F01.S03 | - | mã sai | gõ 111111 | web C1 | một câu lỗi (còn bao nhiêu lượt), các ô OTP xoá để gõ lại | PASS («Mã chưa đúng. Còn 4 lần thử.»; 6 ô OTP rỗng sau lỗi, focus về «Ô nhập mã» (đo lại 2 lần với 2 tài khoản mới)) | RUNTIME-WEB | [EV-F01-OTP-sai-C1](evidence/EV-F01-OTP-sai-C1.jpg) |  |
| TC-F01-SO-THICH-A11Y | F01 | F01.S05 | - | đã chọn 3 gu | đọc thuộc tính ARIA của chip | web C1 | chip đã chọn có aria-checked=true (hoặc aria-selected) | PASS ([{"g":"Ăn uống","checked":"true","selected":null,"role":"checkbox"},{"g":"Cafe","checked":"true","selected":null,"role":"checkbox"},{"g":"Chơi đêm","checked":"true","selected":null,"role":"checkbox"}]) | RUNTIME-WEB | [EV-F01-SO-THICH-da-chon-C1](evidence/EV-F01-SO-THICH-da-chon-C1.jpg) |  |
| TC-F01-SO-THICH-CHUA-CHON | F01 | F01.S05 | - | chưa chọn gu | bấm «Lưu sở thích» | web C1 | không lưu; lý do nhìn thấy được | PASS (nút «Lưu sở thích» aria-disabled=true; câu «Chọn ít nhất 3 để tiếp tục.» dưới lưới gu (cách nút khoảng 250dp)) | RUNTIME-WEB | [EV-F01-SO-THICH-moi-C1](evidence/EV-F01-SO-THICH-moi-C1.jpg) |  |
| TC-F01-SO-THICH-LUU | F01 | F01.S05 | - | đủ 3 gu + mức chi | «Lưu sở thích» | web C1 | lưu và sang màn đầu của người chưa có nhóm | PASS (tới /messages) | RUNTIME-WEB | EV-F01-SAU-SO-THICH-C1 (ngoài git) |  |
| TC-F01-SO-THICH-MOI | F01 | F01.S05 | - | tài khoản mới | đặt tên dài có emoji, chọn 3 gu, mức chi, lưu | web C1 | lưu và sang màn đầu của người chưa có nhóm | PASS (chưa đủ 3 gu: nút aria-disabled và câu «Chọn ít nhất 3 để tiếp tục.»; đủ: tới /messages) | RUNTIME-WEB | [EV-F01-SO-THICH-da-chon-C1](evidence/EV-F01-SO-THICH-da-chon-C1.jpg) |  |
| TC-F01-WEL-AXE | F01 | F01.S01 | - | Welcome | axe WCAG 2 A/AA | web C1,C3 | 0 vi phạm | FAIL (aria-prohibited-attr×1 (cụm chấm có aria-label trên div không role), scrollable-region-focusable×1 (pager không nhận focus bàn phím)) | RUNTIME-WEB |  | UI-020 |
| TC-F01.S01-BASE | F01 | F01.S01 | - | chưa đăng nhập | mở /welcome | web C1,C2,C3,C8 | bìa, trang 1, CTA và «Tìm hiểu thêm» trong màn; C8 rút gọn | PASS (đã mở ảnh 4 cấu hình; axe: 2 vi phạm ghi riêng (UI-020)) | RUNTIME-WEB | [EV-F00.S01-BASE-ghep](evidence/EV-F00.S01-BASE-ghep.jpg) |  |
| TC-F01.S01-FONT | F01 | F01.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /welcome | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F01.S01-NATIVE | F01 | F01.S01 | - | mọi trạng thái | mở /welcome | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F01.S01-NATIVE | F01 | F01.S01 | - | mọi trạng thái | mở /welcome | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F01.S02-BASE | F01 | F01.S02 | - | chưa đăng nhập | mở /login | web C1,C2,C3,C8 | đủ ô số, «Gửi mã», «Tôi có lời mời», không tràn/cắt/che | PASS (ô số 44dp ghi riêng (UI-001); axe 0 vi phạm) | RUNTIME-WEB | [EV-F01.S02-BASE-ghep](evidence/EV-F01.S02-BASE-ghep.jpg) |  |
| TC-F01.S02-FONT | F01 | F01.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /login | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F01.S02-NATIVE | F01 | F01.S02 | - | mọi trạng thái | mở /login | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F01.S02-NATIVE | F01 | F01.S02 | - | mọi trạng thái | mở /login | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F01.S03-BASE | F01 | F01.S03 | - | vừa gửi mã (tài khoản mới) | vào /otp | web C1 | 6 ô, số đã che, «Đổi số», «Gửi lại mã» vô hiệu kèm đếm ngược | PASS (số hiện dạng che «••• ••• xxx»; lý do vô hiệu là dòng đếm ngược dưới nút (đúng ADR-0038)) | RUNTIME-WEB | [EV-F01-OTP-sai-C1](evidence/EV-F01-OTP-sai-C1.jpg) |  |
| TC-F01.S03-FONT | F01 | F01.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /otp | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F01.S03-NATIVE | F01 | F01.S03 | - | mọi trạng thái | mở /otp | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F01.S03-NATIVE | F01 | F01.S03 | - | mọi trạng thái | mở /otp | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F01.S04-BASE | F01 | F01.S04 | - | chưa đăng nhập | mở /moi | web C1,C2,C3,C8 | phong bì, ô mã, «Nhận lời mời»; không lộ «bản trải nghiệm» ở bản prod | PASS (nút «Xem bản trải nghiệm» vắng đúng ADR-0016; ô mã 44dp (UI-001)) | RUNTIME-WEB |  |  |
| TC-F01.S04-FONT | F01 | F01.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /moi | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F01.S04-NATIVE | F01 | F01.S04 | - | mọi trạng thái | mở /moi | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F01.S04-NATIVE | F01 | F01.S04 | - | mọi trạng thái | mở /moi | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F01.S05-BASE | F01 | F01.S05 | - | đã đăng nhập | mở /personalization | web C1,C2,C3,C8 | lưới gu 3 cột (2 ở 320), 4 mức chi có «Trên 500K · Rộng tay», nút lưu, không tràn | PASS (axe 0 vi phạm; chip gu role=checkbox có aria-checked; mức chi role=radio có aria-checked) | RUNTIME-WEB |  |  |
| TC-F01.S05-FONT | F01 | F01.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /personalization | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F01.S05-NATIVE | F01 | F01.S05 | - | mọi trạng thái | mở /personalization | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F01.S05-NATIVE | F01 | F01.S05 | - | mọi trạng thái | mở /personalization | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L30-TIM-HIEU | F01 | F01.S01 | L30 | Welcome trang 4 (sau khi vuốt) | chạm «Tìm hiểu thêm» | web C1 | sang trang kế (từ trang cuối về trang đầu) | FAIL (sang trang 2 vì state trang vẫn là 0) | RUNTIME-WEB |  | UI-016 |
| TC-L30-VONGDOI | F01 | F01.S01 | L30 | pager | vuốt, chấm, «Tìm hiểu thêm» | web C1 | trang, chấm, mốc khớp nhau | FAIL | RUNTIME-WEB | [EV-F01-WEL-trang2-C1](evidence/EV-F01-WEL-trang2-C1.jpg) | UI-016, UI-017 |
| TC-L30-VUOT-NHANH | F01 | F01.S01 | L30 | Welcome trang 1 | vuốt nhanh (khoảng 1077 px/s) và chậm (278, 400 px/s) | web C1 | mỗi lần vuốt sang đúng một trang | FAIL (chậm: sang 1 trang; nhanh: nhảy 2 trang (scroll-snap-stop normal)) | RUNTIME-WEB |  | UI-017 |
| TC-L30-VUOT | F01 | F01.S01 | L30 | Welcome trang 1 | vuốt trái 3 lần | web C1 | chấm trang và mốc trên đường theo đúng trang đang hiện | FAIL (nội dung sang trang 3 và 4, chấm và mốc đứng ở trang 1 suốt; aria-label luôn «Trang 1 trên 4») | RUNTIME-WEB | [EV-F01-WEL-trang2-C1](evidence/EV-F01-WEL-trang2-C1.jpg) | UI-016 |
| TC-MO09-C1 | F01 | F01.S01 | - | Welcome | chạm «Rủ Đi thôi!» | web C1 | lật bìa rồi sang Đăng nhập | PASS (tới /login; 20 khung trong 1,6 s (xem ảnh ghép để đọc trình tự)) | RUNTIME-WEB | [EV-F01-MO09-bia-C1](evidence/EV-F01-MO09-bia-C1.jpg) |  |
| TC-MO09-C9 | F01 | F01.S01 | - | Welcome | chạm «Rủ Đi thôi!» | web C9 | không lật bìa, sang Đăng nhập ngay | PASS (tới /login; 5 khung trong 1,6 s (xem ảnh ghép để đọc trình tự)) | RUNTIME-WEB | EV-F01-MO09-bia-C9 (ngoài git) |  |
| TC-MO09 | F01 | F01.S01 | - | Welcome | chạm «Rủ Đi thôi!» | web C1,C9 | C1: lật bìa rồi sang Đăng nhập; C9: không lật, cắt thẳng | PASS (C1: bìa quay khoảng 180–370 ms lộ trang giấy trống, Đăng nhập hiện ở 476 ms. C9: một khung bìa ở tư thế cuối (khoảng 16 ms) rồi Đăng nhập ở 156 ms) | RUNTIME-WEB | [EV-F01-MO09-bia-C1](evidence/EV-F01-MO09-bia-C1.jpg) |  |

## F02

Đếm: PASS 0 · FAIL 0 · BLOCKED 12 · NOT_TESTED 5 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F02.S01-BASE | F02 | F02.S01 | - | baseline, dữ liệu seed | mở /explore | web C1,C2,C3 | Khám phá: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F02.S01-FONT | F02 | F02.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /explore | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F02.S01-NATIVE | F02 | F02.S01 | - | mọi trạng thái | mở /explore | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F02.S01-NATIVE | F02 | F02.S01 | - | mọi trạng thái | mở /explore | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F02.S02-BASE | F02 | F02.S02 | - | baseline, dữ liệu seed | mở /destinations | web C1,C2,C3 | Điểm đến: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F02.S02-FONT | F02 | F02.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /destinations | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F02.S02-NATIVE | F02 | F02.S02 | - | mọi trạng thái | mở /destinations | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F02.S02-NATIVE | F02 | F02.S02 | - | mọi trạng thái | mở /destinations | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F02.S03-BASE | F02 | F02.S03 | - | baseline, dữ liệu seed | mở /places/[id] | web C1,C2,C3 | Chi tiết quán: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F02.S03-FONT | F02 | F02.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /places/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F02.S03-NATIVE | F02 | F02.S03 | - | mọi trạng thái | mở /places/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F02.S03-NATIVE | F02 | F02.S03 | - | mọi trạng thái | mở /places/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F02.S04-BASE | F02 | F02.S04 | - | baseline, dữ liệu seed | mở /ai-match | web C1,C2,C3 | AI match (demo): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F02.S04-FONT | F02 | F02.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /ai-match | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F02.S04-NATIVE | F02 | F02.S04 | - | mọi trạng thái | mở /ai-match | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F02.S04-NATIVE | F02 | F02.S04 | - | mọi trạng thái | mở /ai-match | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L33-VONGDOI | F02 | F02.S03 | L33 | đóng → mở → dùng → đóng → mở lại | vòng đời Link chỉ đường | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F03

Đếm: PASS 0 · FAIL 0 · BLOCKED 24 · NOT_TESTED 16 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F03.S01-BASE | F03 | F03.S01 | - | baseline, dữ liệu seed | mở /plan | web C1,C2,C3 | Lên plan: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S01-FONT | F03 | F03.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /plan | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S01-NATIVE | F03 | F03.S01 | - | mọi trạng thái | mở /plan | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S01-NATIVE | F03 | F03.S01 | - | mọi trạng thái | mở /plan | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S02-BASE | F03 | F03.S02 | - | baseline, dữ liệu seed | mở /outings/new | web C1,C2,C3 | Tạo kèo: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S02-FONT | F03 | F03.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /outings/new | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S02-NATIVE | F03 | F03.S02 | - | mọi trạng thái | mở /outings/new | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S02-NATIVE | F03 | F03.S02 | - | mọi trạng thái | mở /outings/new | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S03-BASE | F03 | F03.S03 | - | baseline, dữ liệu seed | mở /outings/[id] | web C1,C2,C3 | Kèo: Lịch trình: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S03-FONT | F03 | F03.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /outings/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S03-NATIVE | F03 | F03.S03 | - | mọi trạng thái | mở /outings/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S03-NATIVE | F03 | F03.S03 | - | mọi trạng thái | mở /outings/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S04-BASE | F03 | F03.S04 | - | baseline, dữ liệu seed | mở /outings/[id] | web C1,C2,C3 | Kèo: Bản đồ: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S04-FONT | F03 | F03.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /outings/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S04-NATIVE | F03 | F03.S04 | - | mọi trạng thái | mở /outings/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S04-NATIVE | F03 | F03.S04 | - | mọi trạng thái | mở /outings/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S05-BASE | F03 | F03.S05 | - | baseline, dữ liệu seed | mở /outings/chon | web C1,C2,C3 | Chọn kèo để thêm quán: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S05-FONT | F03 | F03.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /outings/chon | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S05-NATIVE | F03 | F03.S05 | - | mọi trạng thái | mở /outings/chon | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S05-NATIVE | F03 | F03.S05 | - | mọi trạng thái | mở /outings/chon | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S06-BASE | F03 | F03.S06 | - | baseline, dữ liệu seed | mở /check-ins/new | web C1,C2,C3 | Check-in (demo): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S06-FONT | F03 | F03.S06 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /check-ins/new | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S06-NATIVE | F03 | F03.S06 | - | mọi trạng thái | mở /check-ins/new | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S06-NATIVE | F03 | F03.S06 | - | mọi trạng thái | mở /check-ins/new | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S07-BASE | F03 | F03.S07 | - | baseline, dữ liệu seed | mở /trips/[id]/itinerary | web C1,C2,C3 | Lịch trình AI (demo): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S07-FONT | F03 | F03.S07 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /trips/[id]/itinerary | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S07-NATIVE | F03 | F03.S07 | - | mọi trạng thái | mở /trips/[id]/itinerary | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S07-NATIVE | F03 | F03.S07 | - | mọi trạng thái | mở /trips/[id]/itinerary | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F03.S08-BASE | F03 | F03.S08 | - | baseline, dữ liệu seed | mở /trips/[id]/timeline | web C1,C2,C3 | Timeline chuyến (demo): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F03.S08-FONT | F03 | F03.S08 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /trips/[id]/timeline | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F03.S08-NATIVE | F03 | F03.S08 | - | mọi trạng thái | mở /trips/[id]/timeline | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F03.S08-NATIVE | F03 | F03.S08 | - | mọi trạng thái | mở /trips/[id]/timeline | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L08-VONGDOI | F03 | F03.S03 | L08 | đóng → mở → dùng → đóng → mở lại | vòng đời Chặng mới | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L09-VONGDOI | F03 | F03.S03 | L09 | đóng → mở → dùng → đóng → mở lại | vòng đời Gắn quán | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L10-VONGDOI | F03 | F03.S04 | L10 | đóng → mở → dùng → đóng → mở lại | vòng đời Sửa ngày | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L11-VONGDOI | F03 | F03.S04 | L11 | đóng → mở → dùng → đóng → mở lại | vòng đời Điểm hẹn | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L12-VONGDOI | F03 | F03.S04 | L12 | đóng → mở → dùng → đóng → mở lại | vòng đời Popup cụm bản đồ | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L13-VONGDOI | F03 | F03.S04 | L13 | đóng → mở → dùng → đóng → mở lại | vòng đời Trang ngày gập được | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L14-VONGDOI | F03 | F03.S02 | L14 | đóng → mở → dùng → đóng → mở lại | vòng đời Lá lịch tháng | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L15-VONGDOI | F03 | F03.S02 | L15 | đóng → mở → dùng → đóng → mở lại | vòng đời Mặt quay giờ | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F04

Đếm: PASS 0 · FAIL 0 · BLOCKED 15 · NOT_TESTED 6 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F04.S01-BASE | F04 | F04.S01 | - | baseline, dữ liệu seed | mở /smart-split/[id]/review | web C1,C2,C3 | Chia bill 5 bước: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F04.S01-FONT | F04 | F04.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /smart-split/[id]/review | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F04.S01-NATIVE | F04 | F04.S01 | - | mọi trạng thái | mở /smart-split/[id]/review | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F04.S01-NATIVE | F04 | F04.S01 | - | mọi trạng thái | mở /smart-split/[id]/review | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F04.S02-BASE | F04 | F04.S02 | - | baseline, dữ liệu seed | mở /smart-split/[id]/assignment | web C1,C2,C3 | Gán món (demo): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F04.S02-FONT | F04 | F04.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /smart-split/[id]/assignment | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F04.S02-NATIVE | F04 | F04.S02 | - | mọi trạng thái | mở /smart-split/[id]/assignment | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F04.S02-NATIVE | F04 | F04.S02 | - | mọi trạng thái | mở /smart-split/[id]/assignment | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F04.S03-BASE | F04 | F04.S03 | - | baseline, dữ liệu seed | mở /settlements/[id] | web C1,C2,C3 | Quyết toán: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F04.S03-FONT | F04 | F04.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /settlements/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F04.S03-NATIVE | F04 | F04.S03 | - | mọi trạng thái | mở /settlements/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F04.S03-NATIVE | F04 | F04.S03 | - | mọi trạng thái | mở /settlements/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F04.S04-BASE | F04 | F04.S04 | - | baseline, dữ liệu seed | mở /batches/[id] | web C1,C2,C3 | Đợt thu: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F04.S04-FONT | F04 | F04.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /batches/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F04.S04-NATIVE | F04 | F04.S04 | - | mọi trạng thái | mở /batches/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F04.S04-NATIVE | F04 | F04.S04 | - | mọi trạng thái | mở /batches/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F04.S05-BASE | F04 | F04.S05 | - | baseline, dữ liệu seed | mở /finance | web C1,C2,C3 | Tài chính: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F04.S05-FONT | F04 | F04.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /finance | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F04.S05-NATIVE | F04 | F04.S05 | - | mọi trạng thái | mở /finance | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F04.S05-NATIVE | F04 | F04.S05 | - | mọi trạng thái | mở /finance | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L32-VONGDOI | F04 | F04.S04 | L32 | đóng → mở → dùng → đóng → mở lại | vòng đời Chia sẻ | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F05

Đếm: PASS 0 · FAIL 0 · BLOCKED 12 · NOT_TESTED 11 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F05.S01-BASE | F05 | F05.S01 | - | baseline, dữ liệu seed | mở /messages | web C1,C2,C3 | Tin nhắn: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F05.S01-FONT | F05 | F05.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /messages | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F05.S01-NATIVE | F05 | F05.S01 | - | mọi trạng thái | mở /messages | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F05.S01-NATIVE | F05 | F05.S01 | - | mọi trạng thái | mở /messages | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F05.S02-BASE | F05 | F05.S02 | - | baseline, dữ liệu seed | mở /groups/[id]/chat | web C1,C2,C3 | Chat nhóm: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F05.S02-FONT | F05 | F05.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/chat | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F05.S02-NATIVE | F05 | F05.S02 | - | mọi trạng thái | mở /groups/[id]/chat | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F05.S02-NATIVE | F05 | F05.S02 | - | mọi trạng thái | mở /groups/[id]/chat | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F05.S03-BASE | F05 | F05.S03 | - | baseline, dữ liệu seed | mở /groups/[id]/chat | web C1,C2,C3 | Chat đôi (DM): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F05.S03-FONT | F05 | F05.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/chat | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F05.S03-NATIVE | F05 | F05.S03 | - | mọi trạng thái | mở /groups/[id]/chat | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F05.S03-NATIVE | F05 | F05.S03 | - | mọi trạng thái | mở /groups/[id]/chat | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F05.S04-BASE | F05 | F05.S04 | - | baseline, dữ liệu seed | mở /votes/[id] | web C1,C2,C3 | Bình chọn (demo): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F05.S04-FONT | F05 | F05.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /votes/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F05.S04-NATIVE | F05 | F05.S04 | - | mọi trạng thái | mở /votes/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F05.S04-NATIVE | F05 | F05.S04 | - | mọi trạng thái | mở /votes/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L16-VONGDOI | F05 | F05.S02 | L16 | đóng → mở → dùng → đóng → mở lại | vòng đời Báo cáo tin | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L17-VONGDOI | F05 | F05.S02 | L17 | đóng → mở → dùng → đóng → mở lại | vòng đời Cài đặt nhóm | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L18-VONGDOI | F05 | F05.S02 | L18 | đóng → mở → dùng → đóng → mở lại | vòng đời Menu tin | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L19-VONGDOI | F05 | F05.S02 | L19 | đóng → mở → dùng → đóng → mở lại | vòng đời Khay sticker | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L20-VONGDOI | F05 | F05.S02 | L20 | đóng → mở → dùng → đóng → mở lại | vòng đời Khay công cụ | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L21-VONGDOI | F05 | F05.S02 | L21 | đóng → mở → dùng → đóng → mở lại | vòng đời Khay tờ hẹn chung | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L22-VONGDOI | F05 | F05.S02 | L22 | đóng → mở → dùng → đóng → mở lại | vòng đời Thẻ thông báo chat | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F06

Đếm: PASS 0 · FAIL 0 · BLOCKED 24 · NOT_TESTED 9 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F06.S01-BASE | F06 | F06.S01 | - | baseline, dữ liệu seed | mở /groups/new | web C1,C2,C3 | Lập nhóm: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S01-FONT | F06 | F06.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/new | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S01-NATIVE | F06 | F06.S01 | - | mọi trạng thái | mở /groups/new | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S01-NATIVE | F06 | F06.S01 | - | mọi trạng thái | mở /groups/new | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S02-BASE | F06 | F06.S02 | - | baseline, dữ liệu seed | mở /groups/[id]/members | web C1,C2,C3 | Thành viên: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S02-FONT | F06 | F06.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/members | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S02-NATIVE | F06 | F06.S02 | - | mọi trạng thái | mở /groups/[id]/members | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S02-NATIVE | F06 | F06.S02 | - | mọi trạng thái | mở /groups/[id]/members | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S03-BASE | F06 | F06.S03 | - | baseline, dữ liệu seed | mở /groups/[id]/invite | web C1,C2,C3 | Mời vào nhóm: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S03-FONT | F06 | F06.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/invite | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S03-NATIVE | F06 | F06.S03 | - | mọi trạng thái | mở /groups/[id]/invite | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S03-NATIVE | F06 | F06.S03 | - | mọi trạng thái | mở /groups/[id]/invite | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S04-BASE | F06 | F06.S04 | - | baseline, dữ liệu seed | mở /groups/empty | web C1,C2,C3 | redirect: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S04-FONT | F06 | F06.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/empty | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S04-NATIVE | F06 | F06.S04 | - | mọi trạng thái | mở /groups/empty | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S04-NATIVE | F06 | F06.S04 | - | mọi trạng thái | mở /groups/empty | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S05-BASE | F06 | F06.S05 | - | baseline, dữ liệu seed | mở /friends | web C1,C2,C3 | Bạn bè: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S05-FONT | F06 | F06.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /friends | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S05-NATIVE | F06 | F06.S05 | - | mọi trạng thái | mở /friends | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S05-NATIVE | F06 | F06.S05 | - | mọi trạng thái | mở /friends | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S06-BASE | F06 | F06.S06 | - | baseline, dữ liệu seed | mở /friends/add | web C1,C2,C3 | Thêm bạn: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S06-FONT | F06 | F06.S06 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /friends/add | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S06-NATIVE | F06 | F06.S06 | - | mọi trạng thái | mở /friends/add | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S06-NATIVE | F06 | F06.S06 | - | mọi trạng thái | mở /friends/add | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S07-BASE | F06 | F06.S07 | - | baseline, dữ liệu seed | mở /people/[id] | web C1,C2,C3 | Hồ sơ người khác: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S07-FONT | F06 | F06.S07 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /people/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S07-NATIVE | F06 | F06.S07 | - | mọi trạng thái | mở /people/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S07-NATIVE | F06 | F06.S07 | - | mọi trạng thái | mở /people/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F06.S08-BASE | F06 | F06.S08 | - | baseline, dữ liệu seed | mở /people/[id] | web C1,C2,C3 | Hồ sơ của mình: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F06.S08-FONT | F06 | F06.S08 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /people/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F06.S08-NATIVE | F06 | F06.S08 | - | mọi trạng thái | mở /people/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F06.S08-NATIVE | F06 | F06.S08 | - | mọi trạng thái | mở /people/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L06-VONGDOI | F06 | F06.S07 | L06 | đóng → mở → dùng → đóng → mở lại | vòng đời Hành động hồ sơ | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F07

Đếm: PASS 0 · FAIL 0 · BLOCKED 6 · NOT_TESTED 3 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F07.S01-BASE | F07 | F07.S01 | - | baseline, dữ liệu seed | mở /hai-nguoi/chon-nguoi | web C1,C2,C3 | Chọn người rủ: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F07.S01-FONT | F07 | F07.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /hai-nguoi/chon-nguoi | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F07.S01-NATIVE | F07 | F07.S01 | - | mọi trạng thái | mở /hai-nguoi/chon-nguoi | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F07.S01-NATIVE | F07 | F07.S01 | - | mọi trạng thái | mở /hai-nguoi/chon-nguoi | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F07.S02-BASE | F07 | F07.S02 | - | baseline, dữ liệu seed | mở /groups/[id]/to-giay | web C1,C2,C3 | Sổ hai người: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F07.S02-FONT | F07 | F07.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/to-giay | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F07.S02-NATIVE | F07 | F07.S02 | - | mọi trạng thái | mở /groups/[id]/to-giay | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F07.S02-NATIVE | F07 | F07.S02 | - | mọi trạng thái | mở /groups/[id]/to-giay | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L23-VONGDOI | F07 | F07.S02 | L23 | đóng → mở → dùng → đóng → mở lại | vòng đời Các sheet sổ đôi | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F08

Đếm: PASS 0 · FAIL 0 · BLOCKED 27 · NOT_TESTED 15 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F08.S01-BASE | F08 | F08.S01 | - | baseline, dữ liệu seed | mở /groups/[id]/wall | web C1,C2,C3 | Tường nhóm: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S01-FONT | F08 | F08.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/wall | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S01-NATIVE | F08 | F08.S01 | - | mọi trạng thái | mở /groups/[id]/wall | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S01-NATIVE | F08 | F08.S01 | - | mọi trạng thái | mở /groups/[id]/wall | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S02-BASE | F08 | F08.S02 | - | baseline, dữ liệu seed | mở /groups/[id]/album | web C1,C2,C3 | Album nhóm: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S02-FONT | F08 | F08.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /groups/[id]/album | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S02-NATIVE | F08 | F08.S02 | - | mọi trạng thái | mở /groups/[id]/album | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S02-NATIVE | F08 | F08.S02 | - | mọi trạng thái | mở /groups/[id]/album | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S03-BASE | F08 | F08.S03 | - | baseline, dữ liệu seed | mở /trips/[id]/album | web C1,C2,C3 | Album chuyến: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S03-FONT | F08 | F08.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /trips/[id]/album | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S03-NATIVE | F08 | F08.S03 | - | mọi trạng thái | mở /trips/[id]/album | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S03-NATIVE | F08 | F08.S03 | - | mọi trạng thái | mở /trips/[id]/album | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S04-BASE | F08 | F08.S04 | - | baseline, dữ liệu seed | mở /moments/new | web C1,C2,C3 | Thêm kỷ niệm: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S04-FONT | F08 | F08.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /moments/new | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S04-NATIVE | F08 | F08.S04 | - | mọi trạng thái | mở /moments/new | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S04-NATIVE | F08 | F08.S04 | - | mọi trạng thái | mở /moments/new | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S05-BASE | F08 | F08.S05 | - | baseline, dữ liệu seed | mở /stories/new | web C1,C2,C3 | Đăng story: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S05-FONT | F08 | F08.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /stories/new | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S05-NATIVE | F08 | F08.S05 | - | mọi trạng thái | mở /stories/new | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S05-NATIVE | F08 | F08.S05 | - | mọi trạng thái | mở /stories/new | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S06-BASE | F08 | F08.S06 | - | baseline, dữ liệu seed | mở /stories/[personId] | web C1,C2,C3 | Xem story: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S06-FONT | F08 | F08.S06 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /stories/[personId] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S06-NATIVE | F08 | F08.S06 | - | mọi trạng thái | mở /stories/[personId] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S06-NATIVE | F08 | F08.S06 | - | mọi trạng thái | mở /stories/[personId] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S07-BASE | F08 | F08.S07 | - | baseline, dữ liệu seed | mở /posts/new | web C1,C2,C3 | Đăng bài: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S07-FONT | F08 | F08.S07 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /posts/new | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S07-NATIVE | F08 | F08.S07 | - | mọi trạng thái | mở /posts/new | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S07-NATIVE | F08 | F08.S07 | - | mọi trạng thái | mở /posts/new | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S08-BASE | F08 | F08.S08 | - | baseline, dữ liệu seed | mở /posts/[id] | web C1,C2,C3 | Chi tiết bài: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S08-FONT | F08 | F08.S08 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /posts/[id] | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S08-NATIVE | F08 | F08.S08 | - | mọi trạng thái | mở /posts/[id] | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S08-NATIVE | F08 | F08.S08 | - | mọi trạng thái | mở /posts/[id] | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F08.S09-BASE | F08 | F08.S09 | - | baseline, dữ liệu seed | mở /achievements | web C1,C2,C3 | Thành tích: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F08.S09-FONT | F08 | F08.S09 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /achievements | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F08.S09-NATIVE | F08 | F08.S09 | - | mọi trạng thái | mở /achievements | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F08.S09-NATIVE | F08 | F08.S09 | - | mọi trạng thái | mở /achievements | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L07-VONGDOI | F08 | F08.S08 | L07 | đóng → mở → dùng → đóng → mở lại | vòng đời Báo cáo bài | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L24-VONGDOI | F08 | F08.S01 | L24 | đóng → mở → dùng → đóng → mở lại | vòng đời Check-in | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L25-VONGDOI | F08 | F08.S02 | L25 | đóng → mở → dùng → đóng → mở lại | vòng đời Xem ảnh (PhotoViewer) | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L26-VONGDOI | F08 | F08.S06 | L26 | đóng → mở → dùng → đóng → mở lại | vòng đời Xem story + xác nhận xoá | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L31-VONGDOI | F08 | (nhiều màn) | L31 | đóng → mở → dùng → đóng → mở lại | vòng đời Bộ chọn ảnh | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L34-VONGDOI | F08 | (route modal) | L34 | đóng → mở → dùng → đóng → mở lại | vòng đời 3 route modal trượt từ dưới | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F09

Đếm: PASS 0 · FAIL 1 · BLOCKED 18 · NOT_TESTED 8 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F09-TAI-GIU-CHO | F09 | F09.S02 | - | mở Cài đặt từ Cá nhân | chạm «Cài đặt» | web C1 | không hiện dữ liệu giả rồi đổi; công tắc không tự lật | FAIL (130 ms: avatar «B», tên «Bạn», công tắc tắt; 162–197 ms: «Minh Anh», công tắc lật sang bật) | RUNTIME-WEB | [EV-F00-MO01-push-C1](evidence/EV-F00-MO01-push-C1.jpg) | UI-015 |
| TC-F09.S01-BASE | F09 | F09.S01 | - | baseline, dữ liệu seed | mở /profile | web C1,C2,C3 | Cá nhân: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F09.S01-FONT | F09 | F09.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /profile | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F09.S01-NATIVE | F09 | F09.S01 | - | mọi trạng thái | mở /profile | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F09.S01-NATIVE | F09 | F09.S01 | - | mọi trạng thái | mở /profile | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F09.S02-BASE | F09 | F09.S02 | - | baseline, dữ liệu seed | mở /settings | web C1,C2,C3 | Cài đặt: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F09.S02-FONT | F09 | F09.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /settings | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F09.S02-NATIVE | F09 | F09.S02 | - | mọi trạng thái | mở /settings | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F09.S02-NATIVE | F09 | F09.S02 | - | mọi trạng thái | mở /settings | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F09.S03-BASE | F09 | F09.S03 | - | baseline, dữ liệu seed | mở /settings/phien | web C1,C2,C3 | Phiên đăng nhập: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F09.S03-FONT | F09 | F09.S03 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /settings/phien | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F09.S03-NATIVE | F09 | F09.S03 | - | mọi trạng thái | mở /settings/phien | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F09.S03-NATIVE | F09 | F09.S03 | - | mọi trạng thái | mở /settings/phien | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F09.S04-BASE | F09 | F09.S04 | - | baseline, dữ liệu seed | mở /settings/da-chan | web C1,C2,C3 | Đã chặn: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F09.S04-FONT | F09 | F09.S04 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /settings/da-chan | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F09.S04-NATIVE | F09 | F09.S04 | - | mọi trạng thái | mở /settings/da-chan | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F09.S04-NATIVE | F09 | F09.S04 | - | mọi trạng thái | mở /settings/da-chan | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F09.S05-BASE | F09 | F09.S05 | - | baseline, dữ liệu seed | mở /settings/ve-rudi | web C1,C2,C3 | Về Rủ Đi: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F09.S05-FONT | F09 | F09.S05 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /settings/ve-rudi | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F09.S05-NATIVE | F09 | F09.S05 | - | mọi trạng thái | mở /settings/ve-rudi | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F09.S05-NATIVE | F09 | F09.S05 | - | mọi trạng thái | mở /settings/ve-rudi | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F09.S06-BASE | F09 | F09.S06 | - | baseline, dữ liệu seed | mở /settings/xoa-tai-khoan | web C1,C2,C3 | Xoá tài khoản: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F09.S06-FONT | F09 | F09.S06 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /settings/xoa-tai-khoan | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F09.S06-NATIVE | F09 | F09.S06 | - | mọi trạng thái | mở /settings/xoa-tai-khoan | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F09.S06-NATIVE | F09 | F09.S06 | - | mọi trạng thái | mở /settings/xoa-tai-khoan | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L27-VONGDOI | F09 | F09.S06 | L27 | đóng → mở → dùng → đóng → mở lại | vòng đời Xác nhận xoá tài khoản | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-L36-VONGDOI | F09 | F09.S01 | L36 | đóng → mở → dùng → đóng → mở lại | vòng đời Panel trong màn Hồ sơ | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

## F10

Đếm: PASS 0 · FAIL 0 · BLOCKED 6 · NOT_TESTED 2 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F10.S01-BASE | F10 | F10.S01 | - | baseline, dữ liệu seed | mở /dev/ui-lab | web C1,C2,C3 | Bảng component (dev): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F10.S01-FONT | F10 | F10.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /dev/ui-lab | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F10.S01-NATIVE | F10 | F10.S01 | - | mọi trạng thái | mở /dev/ui-lab | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F10.S01-NATIVE | F10 | F10.S01 | - | mọi trạng thái | mở /dev/ui-lab | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F10.S02-BASE | F10 | F10.S02 | - | baseline, dữ liệu seed | mở /dev/san-khau | web C1,C2,C3 | Sân khấu gập (dev): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F10.S02-FONT | F10 | F10.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở /dev/san-khau | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F10.S02-NATIVE | F10 | F10.S02 | - | mọi trạng thái | mở /dev/san-khau | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F10.S02-NATIVE | F10 | F10.S02 | - | mọi trạng thái | mở /dev/san-khau | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |

## F11

Đếm: PASS 0 · FAIL 0 · BLOCKED 6 · NOT_TESTED 3 · NOT_APPLICABLE 0

| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |
|---|---|---|---|---|---|---|---|---|---|---|---|
| TC-F11.S01-BASE | F11 | F11.S01 | - | baseline, dữ liệu seed | mở (tabs) chưa đăng nhập | web C1,C2,C3 | 4 tab demo: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F11.S01-FONT | F11 | F11.S01 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở (tabs) chưa đăng nhập | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F11.S01-NATIVE | F11 | F11.S01 | - | mọi trạng thái | mở (tabs) chưa đăng nhập | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F11.S01-NATIVE | F11 | F11.S01 | - | mọi trạng thái | mở (tabs) chưa đăng nhập | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-F11.S02-BASE | F11 | F11.S02 | - | baseline, dữ liệu seed | mở (route demo) | web C1,C2,C3 | route demo khi chưa đăng nhập: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt | NOT_TESTED | RUNTIME-WEB |  |  |
| TC-F11.S02-FONT | F11 | F11.S02 | - | cỡ chữ hệ thống 1.3 và 2.0 | mở (route demo) | web | không mất nội dung, không mất vùng bấm | BLOCKED (react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật) | STATIC |  |  |
| TC-F11.S02-NATIVE | F11 | F11.S02 | - | mọi trạng thái | mở (route demo) | android | như web, trên thiết bị Android | BLOCKED (không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)) | STATIC |  |  |
| TC-F11.S02-NATIVE | F11 | F11.S02 | - | mọi trạng thái | mở (route demo) | ios | như web, trên thiết bị iOS | BLOCKED (không chạy được iOS: không có macOS / iOS Simulator) | STATIC |  |  |
| TC-L28-VONGDOI | F11 | F11.S01 | L28 | đóng → mở → dùng → đóng → mở lại | vòng đời Tuỳ chọn chuyến (demo) | web C1 | mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về | NOT_TESTED | RUNTIME-WEB |  |  |

