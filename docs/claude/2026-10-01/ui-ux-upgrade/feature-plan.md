# Kế hoạch theo feature và trạng thái từng issue

Nguồn: `issues.md` của hai thư mục audit trong PR #663 (`3bd112e5`). Mỗi hàng giữ ID gốc của QA. Cột «Tiêu chí» là
«Tiêu chí gỡ» của QA rút gọn; khi QA viết chi tiết hơn thì bản của QA thắng.

Trạng thái: PLANNED → IMPLEMENTED_UNVERIFIED → VERIFIED_LOCALLY → READY_FOR_QA. Không hàng nào được ghi QA_ACCEPTED
cho tới khi QA retest độc lập. Cột «Trạng thái» là nguồn sự thật nếu đợt bị ngắt. BLOCKED: chờ một quyết định không thuộc
người sửa (ADR đề xuất đã viết). READY_FOR_QA nghĩa là có hàng tự kiểm và ghi chú trong `qa-handoff.md`.

Batch: B1 P1 + quyền riêng tư + lối vào/ra · B2 primitive dùng chung · B3 pilot Kèo · B4 Tiền · B5 Khám phá · B6 Chat ·
B7 Cộng đồng · B8 Nhóm, người, sổ hai người · B9 Kỷ niệm, sổ chuyến đi, hồ sơ, cài đặt · B10 Vỏ, vào cửa, demo, Nếp ·
B11 rà nhất quán. Primitive ở B2 được xác nhận lại ở batch của từng màn dùng nó.

Codex làm 13 ID trong lúc đợt này tạm dừng (PR #664, `codex/ui-ux-qa-handoff`): UI-002, 008, 009, 011, 012, 014, 016,
017, 020 (B10, CP06–09) và UI-150, 151, 152, 154 (B9a, trang cuối). Bản đó được gộp vào `main` ở `a0fb9ba3`, theo hướng
production của `4f74b011` (không dựng lại demo). Hàng của Codex ghi READY_FOR_QA theo bằng chứng Codex tự kiểm, nằm ở
`docs/codex/2026-10-02/`. Ở đây chỉ ghi lại các cổng chạy trên cây đã gộp, không coi đó là tự kiểm lại từng ca.
Phần trùng của đợt này (lý do `cannot_end_reason`, «chưa tới ngày» ở trang cuối) được bỏ; bản Codex giữ nguyên.

| ID | Mức | Nhóm | Feature | Vấn đề | Batch | Tiêu chí | Trạng thái |
|---|---|---|---|---|---|---|---|
| UI-001 | P3 | UX_IMPROVEMENT, DESIGN_SYSTEM_IMPROVEMENT | F01+ | Ô nhập một dòng 44dp (<48); nút cảm xúc, nút story nhỏ; hitSlop bị web bỏ qua | B2 | mọi ô nhập một dòng ≥48dp | READY_FOR_QA (lượt cuối: `TC-R-UI-001` PASS, 10 ô, 0 ô dưới 48dp) |
| UI-002 | P2 | UX_IMPROVEMENT | F00 | Có phiên mà mở URL lạ thì về Welcome rồi bị đòi đăng nhập | B10 | URL lạ + phiên → về tab, hoặc 404 có lối ra | READY_FOR_QA (Codex CP06, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-003 | P2 | BUG_FIX, DESIGN_SYSTEM_IMPROVEMENT | F00 hệ | `accessibilityState` không tới DOM: tab, radio, checkbox, mở gập không có `aria-*` | B2 | mọi control có trạng thái mang `aria-*` khớp; tab đang chọn có `aria-selected` | READY_FOR_QA |
| UI-004 | P2 | BUG_FIX | F00 | Rail (≥600dp): vạch chỉ báo lệch khỏi tab | B2 (sửa sớm: rail và tablist) | vạch nằm giữa tab đang chọn ở 600/768/839/840/1024 | READY_FOR_QA |
| UI-005 | P1 | BUG_FIX | F00 | Đóng khay «Tạo mới» bằng Back trình duyệt để lại `aria-hidden`/`inert` trên cả màn và thanh tab | B1 | 0 vùng inert ≥25% màn sau Back | READY_FOR_QA |
| UI-006 | P2 | BUG_FIX | F00 | Chạm «+» hai lần nhanh: lần hai rơi vào khay đang mở | B2 | chạm đúp = 1 hộp thoại | READY_FOR_QA |
| UI-007 | P3 | UX_IMPROVEMENT | F00 | Khay tạo cao 92% (C2) / 96% (C8), vượt trần 82% | B2 | panel ≤82% ở C2, C8 | READY_FOR_QA |
| UI-008 | P3 | UX_IMPROVEMENT | F00 | Điểm dừng Tab đầu tiên là khối Nếp không tên | B10 | không còn điểm dừng không tên | READY_FOR_QA (Codex CP08, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-009 | P3 | UX_IMPROVEMENT | F00 | Khôi phục phiên chậm: vùng nội dung trống, không chỉ báo | B10 | skeleton/chỉ báo ≤300ms | READY_FOR_QA (Codex CP06/09, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-010 | P3 | BUG_FIX | F00 | `/create` mở lạnh không mở khay | B2 (sửa sớm: khay Tạo mới) | mở lạnh /create → 1 hộp thoại | READY_FOR_QA |
| UI-011 | P2 | BUG_FIX | F00 | Nút «Vẽ» của bảng Nếp chết trên web (`Alert.alert` rỗng) | B10 | có phản hồi thấy được trên web | READY_FOR_QA (Codex CP08, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-012 | P3 | UX_IMPROVEMENT | F00 | Chip gợi ý của bảng Nếp cao 36dp | B10 | ≥48dp | READY_FOR_QA (Codex CP08, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-013 | P3 | BUG_FIX, MOTION_UPGRADE | F00 | Sheet đóng: panel còn lộ rồi biến mất đột ngột | B2 | khung cuối ra ngoài màn hoặc mờ ≈0 | READY_FOR_QA |
| UI-014 | P3 | VISUAL_UPGRADE | F00 | Ở 320dp mép Nếp đè chữ hàng chip | B10 | không chữ nào bị che ở C2 | READY_FOR_QA (Codex CP08, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-015 | P3 | UX_IMPROVEMENT | F09 | Cài đặt hiện giá trị giữ chỗ («Bạn», «B», công tắc sai) rồi mới đổi | B9 | không khung giữ chỗ sai | READY_FOR_QA |
| UI-016 | P2 | BUG_FIX, MOTION_UPGRADE | F01 | Welcome web: chấm trang và mốc đường đứng yên ở trang 1 khi vuốt | B10 | chấm/nhãn khớp trang đang xem | READY_FOR_QA (Codex CP07, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-017 | P3 | UX_IMPROVEMENT, MOTION_UPGRADE | F01 | Welcome: vuốt nhanh nhảy hai trang | B10 | vuốt nhanh = 1 trang | READY_FOR_QA (Codex CP07, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-018 | P2 | UX_IMPROVEMENT, DESIGN_SYSTEM_IMPROVEMENT | F00 toàn app | «Quay lại» chết khi màn mở thẳng bằng link (TopBar không kiểm `canGoBack`) | B2 | nút lui luôn tới một màn | READY_FOR_QA |
| UI-019 | P2 | UX_IMPROVEMENT | F01 | Mã lời mời sai báo «Cập nhật app rồi thử lại» | B2 | câu theo `code` (mã sai/hết hạn) | READY_FOR_QA |
| UI-020 | P3 | UX_IMPROVEMENT | F01 | Chấm trang Welcome không đọc được; pager không nhận focus | B10 | axe 0 trên /welcome | READY_FOR_QA (Codex CP07, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-021 | P2 | UX_IMPROVEMENT | F02 | Hàng địa điểm: dòng giá bị cắt ở 8–10/10 hàng | B5 | 0 dòng giá bị cắt ở C1–C6 | READY_FOR_QA |
| UI-022 | P2 | BUG_FIX | F02 | «Chỉ đường» trên web không làm gì (`geo:`) | B1 | mở bản đồ hoặc nói vì sao | READY_FOR_QA |
| UI-023 | P2 | UX_IMPROVEMENT | F02/F11 | Nhãn «Lưu địa điểm» và nút demo bị cắt | B5 | nhãn trọn ở C2 | READY_FOR_QA |
| UI-024 | P2 | UX_IMPROVEMENT | F02 | Nút ✦ chỉ điền câu mẫu, danh sách báo «0 kết quả» trước khi có câu hỏi | B5 | không «0 kết quả» trước câu trả lời | READY_FOR_QA |
| UI-025 | P3 | VISUAL_UPGRADE, MOTION_UPGRADE | F02 | Danh sách nhảy 149dp khi sân khấu chen vào | B5 | 0dp ở C1, C9 | READY_FOR_QA |
| UI-026 | P3 | VISUAL_UPGRADE, MOTION_UPGRADE | F02 | Bỏ lọc làm sân khấu dựng lại từ đầu | B5 | không dựng lại | READY_FOR_QA |
| UI-027 | P3 | BUG_FIX, MOTION_UPGRADE | F02/F10 | Giảm chuyển động: sân khấu trống một lúc; Nếp M5 nhảy tư thế | B5 | không khung trống ở C9; M5 một hình | READY_FOR_QA |
| UI-028 | P3 | UX_IMPROVEMENT | F02 | Thành phố chưa có quán khuyên bỏ bộ lọc không tồn tại | B5 | không «Xóa lọc» khi không có lọc | READY_FOR_QA |
| UI-029 | P3 | UX_IMPROVEMENT | F02 | 503 và mất mạng cùng một câu | B2 | 503 → câu máy chủ; mất mạng → câu mạng | READY_FOR_QA |
| UI-030 | P3 | UX_IMPROVEMENT | F02 | Mất mạng rồi về tab: danh sách đã tải bị thay bằng màn lỗi | B5 | giữ danh sách đã tải | READY_FOR_QA |
| UI-031 | P3 | VISUAL_UPGRADE | F02 | Điểm đến luôn 2 cột kể cả expanded | B5 | 3 cột ở C7 | READY_FOR_QA |
| UI-032 | P2 | BUG_FIX | F03 | Kèo nhiều ngày: Bản đồ không hiện chặng nào | B3 | tổng ghim = số chặng, hoặc dòng «chưa xếp ngày» | READY_FOR_QA |
| UI-033 | P2 | UX_IMPROVEMENT | F03 | Ngày trống: dòng giải thích dưới «Về Lịch trình» còn bị cắt (nút đã đạt) | B3 | thấy trọn ở C1–C4, C8 | READY_FOR_QA |
| UI-034 | P2 | UX_IMPROVEMENT | F03 | Ô ngân sách trông như đã điền; bỏ trống thì lỗi ngoài màn | B3 | lỗi thấy ngay sau khi chạm | READY_FOR_QA |
| UI-035 | P2 | BUG_FIX | F03 | `/trips/[id]/timeline` hiện demo cho người đã đăng nhập | B3 | có phiên → /outings/[id] | READY_FOR_QA |
| UI-036 | P2 | BUG_FIX | F03 | Đổi thứ tự chặng chỉ bằng kéo; tay nắm không nhận focus | B3 | bàn phím đổi được; ARIA hợp lệ | READY_FOR_QA |
| UI-037 | P3 | UX_IMPROVEMENT | F03 | Nếp vắng ở chế độ Lịch trình | B3 | Nếp ở Lịch trình, ẩn ở Bản đồ | READY_FOR_QA |
| UI-038 | P3 | UX_IMPROVEMENT | F03+ | Back trình duyệt khi sheet mở rời cả màn, mất chữ đang gõ | B1 (Sheet) + B6 (khay trong màn chat) | Back đóng sheet, giữ URL | READY_FOR_QA |
| UI-039 | P3 | UX_IMPROVEMENT | F03 | «Thêm chặng» là công tắc: chạm đúp mở rồi đóng | B2 (nhờ Sheet v2) | chạm đúp → 1 sheet | READY_FOR_QA |
| UI-040 | P3 | VISUAL_UPGRADE | F03+ | Sheet cao 90–96% ở cửa sổ thấp; nút chính dưới mép | B2 | ≤82% ở C8 | READY_FOR_QA |
| UI-041 | P3 | UX_IMPROVEMENT | F03 | Sheet «Sửa trang ngày» không làm mờ đầu màn mà chặn chạm | B3 | đầu màn mờ; chạm ngoài thì đóng | READY_FOR_QA |
| UI-042 | P3 | BUG_FIX | F03+ | ARIA sai vai trò ở màn kèo | B3 | axe 0 critical ở màn kèo | READY_FOR_QA |
| UI-043 | P3 | UX_IMPROVEMENT | F03 | Điều khiển bản đồ tên tiếng Anh; Esc không đóng popup cụm | B3 | không tiếng Anh; Esc đóng | READY_FOR_QA |
| UI-044 | P3 | UX_IMPROVEMENT | F03 | Dòng thông tin vé kèo bị cắt (mất số chặng) | B3 | C2 giữ số chặng | READY_FOR_QA |
| UI-045 | P3 | VISUAL_UPGRADE | F03 | Ở 320dp cột tên chặng 53px | B3 | ≥120px | READY_FOR_QA |
| UI-046 | P3 | BUG_FIX | F03 | `/outings/chon` thiếu `?place` kẹt skeleton | B3 | câu + lối ra ≤1s | READY_FOR_QA |
| UI-047 | P3 | VISUAL_UPGRADE | F03+ | Tablet: đầu màn co vào giữa, lệch cột nội dung | B3 | nút lui thẳng mép cột | READY_FOR_QA |
| UI-048 | P2 | BUG_FIX | F04/F11 | Số tiền món bị cắt «12.3…» | B4 | 0 số tiền bị cắt | READY_FOR_QA |
| UI-049 | P1 | BUG_FIX | F04 | Web: «Gửi cho <tên>» không gửi được link, báo «Kiểm tra mạng» ở y −2855 | B1 | có lối chép link; không báo lỗi khi đóng khay | READY_FOR_QA |
| UI-050 | P2 | UX_IMPROVEMENT | F04 | Từ 9 người ghế đè nhau; 20 người chạm ghế này đổi ghế khác | B4 | chạm đúng ghế tới n=20 | READY_FOR_QA |
| UI-051 | P2 | UX_IMPROVEMENT | F04 | Lý do chặn và lỗi 503 ở đầu trang, ngoài màn | B4 | câu trong khung nhìn sau khi chạm | READY_FOR_QA |
| UI-052 | P2 | UX_IMPROVEMENT | F04 | Lùi bước 1, Back hay tải lại đều mất bill đang gõ | B4 | không mất mà không hỏi | READY_FOR_QA |
| UI-053 | P3 | BUG_FIX, DESIGN_SYSTEM_IMPROVEMENT | F04+ | Phím Space không đổi checkbox/radio dựng bằng Pressable | B2 | Space đổi trạng thái | VERIFIED_LOCALLY |
| UI-054 | P3 | VISUAL_UPGRADE | F04 | Sơ đồ quyết toán từ 10 người: nhãn đè nhau | B4 | 10 người: 0 chỗ đè | READY_FOR_QA |
| UI-055 | P3 | VISUAL_UPGRADE | F04 | Nếp M2 bị đẩy ra ngoài màn | B4 | trong màn ở C1–C3 | READY_FOR_QA |
| UI-056 | P3 | UX_IMPROVEMENT | F04 | Đọc ảnh bill lỗi khuyên nhập tay mà không có nút | B4 | có lối nhập tay tại chỗ | READY_FOR_QA |
| UI-057 | P3 | UX_IMPROVEMENT | F04 | Mép Nếp hứa «chạm để kéo ra» mà chạm không làm gì | B4 | không hứa điều không làm | READY_FOR_QA |
| UI-058 | P3 | UX_IMPROVEMENT | F04 | «Tạo đợt thu từ sổ» vẫn mời khi mọi khoản đã vào đợt | B4 | không có nút chắc chắn hỏng | READY_FOR_QA |
| UI-059 | P3 | UX_IMPROVEMENT | F04 | Dòng người trả mất «(trả)» | B4 | «trả» luôn thấy | READY_FOR_QA |
| UI-060 | P3 | UX_IMPROVEMENT | F04 | Tài chính: «Chi theo nhóm» không có hàng | B4 | tiêu đề khớp nội dung | READY_FOR_QA |
| UI-061 | P3 | VISUAL_UPGRADE | F04 | Quyết toán ở 320dp: dòng đầu sổ ép thành 9 dòng | B4 | ≤5 dòng ở C2 | READY_FOR_QA |
| UI-062 | P2 | BUG_FIX | F05 | Ô soạn web là textarea 2 hàng không cao lên; chữ lệch 20px | B6 | lệch ≤4px; cao dần tới trần | READY_FOR_QA |
| UI-063 | P2 | BUG_FIX | F05 | Bong bóng có link dài tràn cột; 320dp mất đầu link | B6 | bong bóng trong khung | READY_FOR_QA |
| UI-064 | P3 | VISUAL_UPGRADE | F05 | Avatar thấp hơn bong bóng 22px; giờ lặp dưới mọi cụm | B6 | avatar ±4px; giờ theo khoảng thời gian | READY_FOR_QA |
| UI-065 | P3 | VISUAL_UPGRADE | F05 | Thẻ bình chọn chiếm 39–59% màn, «phiếu» lặp 4 lần, trải hết tablet | B6 | ≤25% chiều cao C1; «phiếu» ≤1; ≤560 ở C6 | READY_FOR_QA |
| UI-066 | P3 | UX_IMPROVEMENT | F05 | Khay «Tờ hẹn chung» không đóng bằng Esc | B2 | Esc đóng, focus trả về | READY_FOR_QA |
| UI-067 | P3 | UX_IMPROVEMENT | F05+ | Sheet báo cáo: lý do chọn chỉ khác màu nền | B6 | 5 radio, một đang chọn | READY_FOR_QA |
| UI-068 | P3 | UX_IMPROVEMENT | F05 | Một request tin hỏng: câu lỗi chung, không «Thử lại», dù tin vẫn hiện đủ | B6 | không câu thừa, hoặc có «Thử lại» | READY_FOR_QA |
| UI-069 | P3 | UX_IMPROVEMENT | F05 | Thẻ thông báo lỗi mọc ở cuối chat, mặc áo AI | B6 | trong khung nhìn, không kiểu AI | READY_FOR_QA |
| UI-070 | P3 | VISUAL_UPGRADE | F05 | Dải ghim sáng trên nền mờ khi sheet mở | B2 | dải bị mờ cùng nền | READY_FOR_QA |
| UI-071 | P3 | UX_IMPROVEMENT | F06 | Lập nhóm xong về Khám phá, không thấy nhóm vừa lập | B8 | vào nhóm mới + lời mời | READY_FOR_QA |
| UI-072 | P3 | BUG_FIX | F06 | Mời người đã ở trong nhóm: 409 hiện thành câu «lần bấm trước» | B2 | câu «đã ở trong nhóm» | READY_FOR_QA |
| UI-073 | P2 | BUG_FIX | F06/E1 | Người vào bằng lời mời bỏ qua Sở thích; tên người mời đặt thành tên công khai | B8 + ADR | Sở thích có ô tên sửa được (luật tra số: ADR đề xuất) | READY_FOR_QA (lượt cuối: `TC-R-UI-073` PASS, tới Sở thích, ô tên điền sẵn tên người mời đặt) |
| UI-074 | P2 | UX_IMPROVEMENT | F06 | Quản trị tự bỏ quyền bằng một chạm | B8 | hỏi trước khi tự hạ quyền | READY_FOR_QA |
| UI-075 | P3 | BUG_FIX | F06 | Mọi quản trị mang nhãn «Người lập nhóm» | B8 | chỉ người lập | READY_FOR_QA |
| UI-076 | P3 | UX_IMPROVEMENT | F06 | 19 nút cùng tên «Đặt làm quản trị»; hàng không mở hồ sơ | B8 | tên nút riêng; hàng mở hồ sơ | READY_FOR_QA |
| UI-077 | P3 | UX_IMPROVEMENT | F06 | «Đồng ý» lỗi thì cả danh sách bạn thành màn lỗi | B8 | lỗi theo hàng, danh sách giữ | READY_FOR_QA |
| UI-078 | P3 | UX_IMPROVEMENT | F06 | Sau khi chặn, tải lại mất dấu «Đã chặn» | B8 | «Đã chặn» + «Bỏ chặn» sau tải lại | READY_FOR_QA |
| UI-079 | P3 | UX_IMPROVEMENT | F05/F06 | Chat đôi bị chặn: «Đang nối lại» mãi, vẫn mời nhắn | B6 | 20s: không còn; người chặn thấy lý do | READY_FOR_QA |
| UI-080 | P3 | UX_IMPROVEMENT | F05/F06 | Lời mời vào nhóm không nói ai mời, không từ chối được | B8 | tên người mời + hai lựa chọn | READY_FOR_QA |
| UI-081 | P3 | VISUAL_UPGRADE | F06 | Tablet: danh sách trải hết, nút cách tên 487–679px | B8 | ≤160px | READY_FOR_QA |
| UI-082 | P1 | BUG_FIX | F07/F11/E6 | Không phiên: link thật mở sổ demo không nhãn, «Gửi» báo đã gửi | B1 | cửa đăng nhập rồi về link; không báo gửi giả | READY_FOR_QA |
| UI-083 | P2 | BUG_FIX | F07 | Đọc sổ lỗi thì vẽ «Chưa có sổ» và mời lập lại | B8 | lỗi + «Thử lại» | READY_FOR_QA |
| UI-084 | P2 | BUG_FIX | F07 | Sheet «Lập sổ» quay về trạng thái mời khi vừa được đồng ý | B8 | 0 khung mời sau khi đồng ý | READY_FOR_QA |
| UI-085 | P2 | BUG_FIX | F02→F07 | «Rủ … tới đây» phác thêm tờ không mang quán, câu nói ngược | B8 | số tờ không đổi + câu đúng | READY_FOR_QA |
| UI-086 | P3 | UX_IMPROVEMENT | F07 | Người đề nghị đóng sheet thì không còn thấy «đang chờ» | B8 | câu chờ trên màn | READY_FOR_QA |
| UI-087 | P3 | UX_IMPROVEMENT | F07 | «Cài đặt sổ» vẫn mở khi quay lại | B1 (sheet đóng khi màn mất focus) | 0 hộp thoại khi quay lại | READY_FOR_QA |
| UI-088 | P3 | UX_IMPROVEMENT | F07+ | Nền sheet nhận chạm ngay lúc mở | B2 | chạm đúp → 1 sheet | READY_FOR_QA |
| UI-089 | P3 | BUG_FIX | mọi sheet | Tay cầm `div` có `aria-label` không role | B2 | axe 0 | READY_FOR_QA |
| UI-090 | P3 | VISUAL_UPGRADE | F07 | Tên trên bìa sổ bị cắt «Chat Tes…» | B8 | tên phân biệt được | READY_FOR_QA |
| UI-091 | P3 | UX_IMPROVEMENT | F07/F08+ | Nút tắt không nói lý do | B2 | không nút tắt nào thiếu lý do | READY_FOR_QA |
| UI-092 | P3 | VISUAL_UPGRADE | F07 | Lá ngày đang chọn nằm khuất | B8 | lá chọn thấy trọn (cả ngày xa) | READY_FOR_QA |
| UI-093 | P3 | VISUAL_UPGRADE | F07+ | Tablet: tờ giấy, sheet trải hết bề ngang | B2 | ≤640 | READY_FOR_QA |
| UI-094 | P2 | BUG_FIX | F08 | Viewer web: ảnh cao 0, vuốt nhảy hai ảnh, chụm phóng cả trang | B9 | ảnh >0; «2/3»; không phóng trang | READY_FOR_QA |
| UI-095 | P2 | UX_IMPROVEMENT | F08 | Thả tim lỗi ở cuối tường: câu lỗi ở đầu tường | B9 | trong khung nhìn | READY_FOR_QA |
| UI-096 | P2 | UX_IMPROVEMENT | F08 | (đã hết: nút xoá bình luận bị gỡ; lối xoá chuyển thành UI-158) | B9 | xem UI-158 | READY_FOR_QA |
| UI-097 | P2 | UX_IMPROVEMENT | F08 | Rời «Thả khoảnh khắc»/«Đăng story» mất ảnh và chữ, không hỏi | B9 | hỏi hoặc giữ nháp | READY_FOR_QA |
| UI-098 | P3 | VISUAL_UPGRADE, MOTION_UPGRADE | F08 | Viewer mờ dần khi mở nhưng biến mất ngay khi đóng | B9 | ≥1 khung mờ giữa chừng | READY_FOR_QA |
| UI-099 | P3 | UX_IMPROVEMENT | F08 | Chip tên quán dài tràn mép sheet check-in | B2 (Chip `maxWidth`) | chip trong sheet | READY_FOR_QA |
| UI-100 | P3 | UX_IMPROVEMENT | F08 | Bài «Chỉ mình tôi» mở bởi người khác: hai khối lỗi, hai «Thử lại» vô ích | B9 | một khối, không «Thử lại» | READY_FOR_QA |
| UI-101 | P3 | UX_IMPROVEMENT | F08 | Viewer story: vùng chạm không role; câu hỏi xoá không nhận focus | B9 | axe 0; focus vào câu hỏi | READY_FOR_QA |
| UI-102 | P3 | UX_IMPROVEMENT | F08 | Ảnh dọc 9:16: xem trước trọn, lên tường bị cắt | B9 | cùng vùng thấy | READY_FOR_QA |
| UI-103 | P3 | UX_IMPROVEMENT | F08 | Album chỉ ghi năm, không ghi ngày chuyến | B9 (Go) | có ngày chuyến | READY_FOR_QA |
| UI-104 | P3 | VISUAL_UPGRADE | F08/F09 | Ngày viết «28-09», «28/9/2026» lẫn lộn | B9 | một định dạng | READY_FOR_QA |
| UI-105 | P3 | VISUAL_UPGRADE | F08/F09 | Tablet: ảnh tường 702–894px, cao hơn cửa sổ | B9 | cột ≤640 | READY_FOR_QA |
| UI-106 | P3 | UX_IMPROVEMENT | F08/N21 | Ở 320px thẻ huy hiệu bẻ đôi chữ «châ/n» | B9 | cột chữ ≥120px | READY_FOR_QA |
| UI-107 | P2 | UX_IMPROVEMENT | F09 | Lưu công tắc lỗi: câu lỗi ở cuối trang | B9 | cạnh control | READY_FOR_QA |
| UI-108 | P3 | UX_IMPROVEMENT | F09 | Panel trong Cá nhân trông như màn con, Back rời tab | B9 | Back đóng panel | READY_FOR_QA |
| UI-109 | P3 | UX_IMPROVEMENT | F09 | «Đã lưu» chỉ có con số | B9 | mở được từng chỗ | READY_FOR_QA |
| UI-110 | P3 | UX_IMPROVEMENT | F09 | Xoá tài khoản: «XOA» không dấu; «XOÁ» tắt nút không lý do; Back rời trang | B9 | nhận «XOÁ» hoặc nói lý do; Back về bước 1 | READY_FOR_QA |
| UI-111 | P3 | UX_IMPROVEMENT | F09 | Câu cuối Cài đặt chỉ sai chỗ đổi tên | B9 | chỉ đúng chỗ | READY_FOR_QA |
| UI-112 | P3 | UX_IMPROVEMENT, DESIGN_SYSTEM_IMPROVEMENT | F00 | Sang màn mới focus ở `body` | B2 | focus trong màn mới | READY_FOR_QA |
| UI-113 | P2 | UX_IMPROVEMENT | F02 | 320dp: tim «Lưu» của cặp so sánh bị đẩy ra ngoài | B5 | cả hai tim trong ô | READY_FOR_QA |
| UI-114 | P3 | BUG_FIX | F02 | Nút «Lưu» lồng trong nút «Mở …» | B5 | không nút lồng nút | READY_FOR_QA |
| UI-115 | P3 | BUG_FIX | F10 | Sân khấu kéo nghiêng chặn cuộn dọc | B5 | kéo dọc vẫn cuộn | READY_FOR_QA |
| UI-116 | P2 | BUG_FIX | F11 | Chat demo: bong bóng không xuống dòng, chữ tràn | B6 | 0 chữ bị cắt ở C1–C3 | READY_FOR_QA |
| UI-117 | P2 | BUG_FIX | F11 | Back khi sheet demo mở: Khám phá bị khoá, chạm rơi vào sheet vô hình | B1 | không inert sót; chạm hoạt động | READY_FOR_QA |
| UI-118 | P3 | UX_IMPROVEMENT | E2 | Kèo tạo từ chat không để lại dấu trong chat | B3 | chat có kèo + lối mở | READY_FOR_QA |
| UI-119 | P2 | UX_IMPROVEMENT | E2 | «Tôi đã tới» không thành kỷ niệm; album «0 chỗ đã tới» | B3 + ADR | lối thêm khoảnh khắc ngay sau khi tới; album nói rõ nó đếm gì (luật: ADR đề xuất) | READY_FOR_QA (phần UI) · BLOCKED (luật: chờ quyết ADR UI-119) |
| UI-120 | P1 | BUG_FIX | E5 | Bị chặn vẫn gửi được tờ hẹn tới người đã chặn mình | B1 (Go+Py) | bên bị chặn gửi → từ chối; bên chặn không nhận gì | READY_FOR_QA |
| UI-121 | P2 | UX_IMPROVEMENT | E6 | Đăng nhập từ link không quay về link đó | B1 | tới đúng link gốc | READY_FOR_QA |
| UI-122 | P3 | BUG_FIX | E1 | Đầu chat giữ số thành viên cũ khi có người mới vào | B6 | cập nhật trong vài giây | READY_FOR_QA |
| UI-123 | P2 | BUG_FIX | F00 | Web: đổi tab thay mục lịch sử, Back rời app | B1 | Back qua lại giữa các tab | READY_FOR_QA |
| UI-124 | P2 | BUG_FIX | N26 | Chat hai người rỗng ở cửa sổ thấp đẩy ô soạn ra ngoài | B6 | nút gửi trong khung ở C2/C4/C8 | READY_FOR_QA |
| UI-125 | P3 | UX_IMPROVEMENT | N26 | Hàng ghim tắt/bật mỗi lần focus (nhảy 78dp); không thấy phòng thành cặp đôi | B6 | 0 khung mất hàng; cập nhật ≤8s | READY_FOR_QA |
| UI-126 | P3 | UX_IMPROVEMENT | N26 | Hàng mời «Một đôi» dẫn tới màn không nhắc lời đề nghị | B8 | màn tới nói về lời đề nghị | READY_FOR_QA |
| UI-127 | P3 | UX_IMPROVEMENT, MOTION_UPGRADE | N26 | Khoảnh khắc M6 «sổ hai người mở» không diễn | B8 | M6 diễn một lần; C9 khung cuối | READY_FOR_QA |
| UI-128 | P3 | UX_IMPROVEMENT | N26 | Chat hai người còn chữ của nhóm | B6 | không «nhóm/hội» trong chat hai người | READY_FOR_QA |
| UI-129 | P3 | UX_IMPROVEMENT | N26 | Sheet «Gu của hai bạn»: hai dòng ngược nhau; lỗi nằm dưới lớp phủ | B6 | một câu đúng; lỗi trong sheet | READY_FOR_QA |
| UI-130 | P2 | UX_IMPROVEMENT | N26×F02 | «Rủ X tới đây» cho cặp bạn: chỗ vừa chọn bị bỏ | B8 | màn tới mang quán, hoặc nút ẩn | READY_FOR_QA (lượt cuối: `TC-N26-RU-BAN-CHUA-SO`, `-CO-SO` PASS, màn tới nhắc tên chỗ) |
| UI-131 | P3 | BUG_FIX | N26 | Máy chủ vẫn phác tờ cho cặp chưa «Một đôi» | ADR-0053 (B8 phần app) | QA: draft cho cặp bạn → 4xx; chủ sản phẩm chọn B (ADR-0053): 201 có chủ đích, khay chat chỉ cho cặp đôi | READY_FOR_QA (phương án B: máy chủ giữ tờ tạm có chủ đích, ADR-0053; phần app «Rủ … tới đây» ở B8) |
| UI-132 | P2 | BUG_FIX | N14 | Cộng đồng mới: tab đầu báo «chưa kết nối được», «Thử lại» không bao giờ được | B7 (Go) | 200 với 0 bài; trạng thái rỗng mời kể chuyện | READY_FOR_QA |
| UI-133 | P2 | BUG_FIX | N14 | Sáu chủ đề hay chủ đề một ký tự → câu «lỗi của app», dưới mép màn | B7 | câu theo chủ đề, thấy lúc gửi | READY_FOR_QA |
| UI-134 | P2 | BUG_FIX | N14 | Stream nối lại làm mất bình luận đang gõ | B7 | nháp sống qua nối lại | READY_FOR_QA |
| UI-135 | P2 | UX_IMPROVEMENT | N14 | Mở bài rồi lui: bảng tin về đầu, bài gập lại | B7 | giữ vị trí cuộn ±24px và trạng thái mở | READY_FOR_QA |
| UI-136 | P2 | BUG_FIX | N14 | «Chia sẻ» trên web không làm gì; có Web Share thì gửi chuỗi `rudi://` | B1 | có phản hồi; link https | VERIFIED_LOCALLY |
| UI-137 | P2 | UX_IMPROVEMENT | N14 | Mở màn trong bằng link khi không phiên: chờ mãi, không lối đăng nhập | B1 | cả 4 route có «Đăng nhập» | READY_FOR_QA |
| UI-138 | P2 | UX_IMPROVEMENT | N14 | Gọi Nếp lỗi: câu lỗi nằm sau sheet | B7 | lỗi trong sheet + thử lại | READY_FOR_QA |
| UI-139 | P3 | UX_IMPROVEMENT | N14 | Bài ngắn: chạm đầu vào thân không làm gì | B7 | chạm mở chi tiết khi không bị cắt | READY_FOR_QA |
| UI-140 | P3 | BUG_FIX | N14 | Theo dõi chỉ cập nhật một thẻ của tác giả | B7 | mọi thẻ cùng tác giả | READY_FOR_QA |
| UI-141 | P3 | UX_IMPROVEMENT | N14 | «Không quan tâm» không hoàn tác; «Xóa lịch sử đề xuất» một chạm | B7 (Go) | hoàn tác tại chỗ; xem lại bài đã ẩn; hỏi trước khi xoá | READY_FOR_QA |
| UI-142 | P3 | UX_IMPROVEMENT | N14 | Sửa bài thì bài rời bảng tin của chính tác giả | B7 (Go) | tác giả thấy bản đã duyệt + dải chờ duyệt | READY_FOR_QA |
| UI-143 | P3 | VISUAL_UPGRADE | N14 | 320: ảnh cắt 8px; dải cập nhật đè tab 13px | B7 | 0px cắt; 0px đè | READY_FOR_QA |
| UI-144 | P3 | UX_IMPROVEMENT | N14 | Hàng duyệt in «· pending» | B7 | nhãn tiếng Việt | READY_FOR_QA |
| UI-145 | P3 | UX_IMPROVEMENT | N14 | Tìm không ra và «Điều mình muốn giữ» không có trạng thái rỗng | B7 | có câu rỗng | READY_FOR_QA |
| UI-146 | P3 | UX_IMPROVEMENT | N14 | Trang chủ đề không có «Quay lại» | B7 | TopBar lui + theo dõi chủ đề | READY_FOR_QA |
| UI-147 | P3 | UX_IMPROVEMENT | N14 | Thông báo không nói ai nhắc | B7 (Go) | tên người nhắc; lối vào ngoài sheet | READY_FOR_QA |
| UI-148 | P3 | UX_IMPROVEMENT | N14 | Đọc bình luận lỗi không có «Thử lại» | B7 | «Thử lại» đọc lại | READY_FOR_QA |
| UI-149 | P2 | BUG_FIX | N15 | «Đã chia» tính theo ngày: hai kèo trùng ngày cùng ghi một khoản | ADR-0054 (Go + Python + migration + app) | mỗi khoản chi thuộc nhiều nhất một kèo; Σ «đã chia» ≤ sổ; hai kèo trùng ngày không đếm hai lần | READY_FOR_QA (ADR-0054; bằng chứng máy chủ: oracle Go/Python, 9 ca PostgreSQL gồm backfill, parity; thế giới harness không có khoản chi trùng ngày) |
| UI-150 | P2 | UX_IMPROVEMENT | N15 | Lưu sổ lỗi: câu lỗi ở đầu màn, trên nút hơn 1000px | B9 | lỗi thấy ngay sau khi chạm | READY_FOR_QA (Codex B9a, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-151 | P2 | BUG_FIX | N15 | Kèo chưa tới ngày vẫn có «Khép cuộc đi»; chạm thì 409 | B9 (Go) | `can_end` false + lý do; không nút | READY_FOR_QA (Codex B9a, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-152 | P3 | UX_IMPROVEMENT | N15 | Thành viên thấy bộ chọn loại mà không có tác dụng | B9 | không bộ chọn với người không tổ chức | READY_FOR_QA (Codex B9a, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-153 | P3 | UX_IMPROVEMENT | N15 | Kệ trống chỉ có một câu, không hành động | B9 | một hành động qua EmptyState | READY_FOR_QA |
| UI-154 | P3 | UX_IMPROVEMENT | N15 | Tên trang tự xếp là «2026-09-29» | B9 (Go) | không ngày ISO | READY_FOR_QA (Codex B9a, PR #664 — tích hợp `a0fb9ba3`; bằng chứng: `docs/codex/2026-10-02/`) |
| UI-155 | P2 | BUG_FIX | N21 | Long poll trả về thì tường cắt về trang đầu | B9 | giữ đủ bài sau sự kiện hoặc 30s | VERIFIED_LOCALLY |
| UI-156 | P2 | BUG_FIX | N21 | Bài Cộng đồng có ảnh lên tường không ảnh | B9 (Go) | thẻ tường có ảnh | VERIFIED_LOCALLY |
| UI-157 | P2 | UX_IMPROVEMENT | N21 | Lỗi thao tác sổ hành trình nằm ở cuối sổ | B9 | thấy ngay sau khi chạm | READY_FOR_QA |
| UI-158 | P2 | UX_IMPROVEMENT | N21 | Không còn lối xoá bình luận của mình ở trang viết | B9 | xoá có hỏi | READY_FOR_QA |
| UI-159 | P3 | UX_IMPROVEMENT | N21 | Câu xác nhận đăng lại ngoài khung | B9 | trong khung nhìn | READY_FOR_QA |
| UI-160 | P3 | UX_IMPROVEMENT | N21 | «MỚI MỞ» nhớ theo máy | B9 (Go) | máy mới không «MỚI MỞ» huy hiệu đã thấy | READY_FOR_QA |
| UI-161 | P3 | UX_IMPROVEMENT | N21 | Chạm huy hiệu thứ tư không phản hồi mà vẫn gửi PATCH | B9 | không PATCH; nói lý do | VERIFIED_LOCALLY |
| UI-162 | P3 | UX_IMPROVEMENT | N21 | Mục «Thành tích…» mở màn «Hành trình» | B9 | tên mục khớp màn tới | READY_FOR_QA |
| UI-163 | P2 | BUG_FIX | N22 | Gói bối cảnh mặc định 40 tin, gấp đôi mức 20 của ADR-0046 | B1 | chip ≤20; gói = số trên chip | READY_FOR_QA |
| UI-164 | P3 | UX_IMPROVEMENT | N22 | Chip nói «gửi như tin thường» mà vẫn hiện «Đang hỏi Rủ Đi AI…» | B6 | không khối AI khi AI chưa sẵn sàng | READY_FOR_QA |
| UI-165 | P3 | UX_IMPROVEMENT | N22 | Chữ hiện dần, «đang đọc/nghĩ», lỗi Nếp không trong vùng aria-live | B6 | vùng live lịch sự, báo một lần | READY_FOR_QA |
| UI-166 | P2 | BUG_FIX | N22 | Tấm «Xem» trượt dưới dải ghim | B2 | sheet phủ dải; trần 82% cả sheet | READY_FOR_QA |
| UI-167 | P3 | VISUAL_UPGRADE | N22 | Ở 320 chip gãy dòng, ✦ đứng riêng | B6 | ✦ cùng dòng; chip ≤36 cao | READY_FOR_QA |

Đếm theo batch: ADR 1 · B1 13 · B10 11 · B2 20 · B3 16 · B4 12 · B5 12 · B6 17 · B7 15 · B8 18 · B9 32

## Ngoài danh sách issue

| Mục | Nhóm | Batch | Tiêu chí | Trạng thái |
|---|---|---|---|---|
| Con dấu «Tạo mới» trên mọi tab (yêu cầu của chủ sản phẩm) | UX_IMPROVEMENT + VISUAL_UPGRADE | B2, làm lại sau mockup | mọi tab có lối Tạo mới; không che nội dung; không tranh màu với tab đang chọn; theo mockup của chủ sản phẩm | CHUYỂN GIAO (01/10: chủ sản phẩm đánh giá con dấu tròn chưa đạt và tự đảm nhận thanh tab dưới cùng; ra khỏi danh sách việc của đợt này) |
| `KheLop`: khe lớp phủ của màn (sheet sâu trong màn phủ cả đầu màn) | DESIGN_SYSTEM_IMPROVEMENT | B3 | sheet phủ đầu màn; state giữ; ô nhập không chậm phím | READY_FOR_QA |
| Dòng nguồn bản đồ luôn thấy khi bản đồ hiện | BUG_FIX | B3 | probe `NGUON-BAN-DO` | READY_FOR_QA |
| Bước «Kết quả» chia bill: cuống của bạn + dải phiếu đục lỗ + dòng cộng «Cộng N phần» (luật tiền 2 hiện ra màn) | VISUAL_UPGRADE + UX_IMPROVEMENT | B4 | dòng cộng = tổng bill; con dấu người trả hạ đúng hàng; không chip giả nút | READY_FOR_QA |
| Bill 0đ chặn ở bước xem lại, chỉ ra đúng dòng (thấy trên Android, ngoài QA) | BUG_FIX | B4 | không gửi món 0đ; câu ngay trên nút; ô cần sửa có câu và con trỏ | READY_FOR_QA |
| Đợt thu và quyết toán gom theo người nhận; hàng chưa về không con dấu | VISUAL_UPGRADE + UX_IMPROVEMENT | B4 | người nhận nói một lần mỗi mục; 0 con dấu «chưa chuyển» | READY_FOR_QA |
| Route Go `GET /contexts/{id}/unbatched-expenses` (gói native `gomdot`) | UX_IMPROVEMENT (backend cho UI-058) | B4 | thành viên đọc được số đếm, người ngoài 403; tầng PostgreSQL | READY_FOR_QA |
| Tài chính: số theo từng nhóm (route Go đọc tổng hợp) | UX_IMPROVEMENT | sau B11, cần chủ sản phẩm chọn ngữ nghĩa «chi theo nhóm» | — | PLANNED |
| Test `nepnho.TestQuenKhiConHangThiThuLai` chập chờn, treo tới timeout thay vì đỏ (t.Fatal khi còn giữ transaction) | — (ngoài phạm vi UI) | ghi lại, không sửa | đỏ ngay khi hỏng, không treo | PLANNED |
