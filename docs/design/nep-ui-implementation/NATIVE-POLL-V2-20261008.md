# Bình chọn trong chat v2 — kiểm native 08/10/2026

## Lỗi đã quan sát

Emulator 320×640, JS `377cf3bd`: form đã nhập đủ câu hỏi/Cafe/Com qua bộ gõ Android thật. Gửi thành công nhưng chat chỉ hiện `/vote Native poll QA? Cafe | Com`, không có radio để bỏ phiếu. Peer Rust/MLS giải mã đúng câu đó. Probe đỏ ở assertion «Bỏ phiếu Cafe»; root mở ảnh failed. Full mobile sạch cùng SHA đạt 1608/1608, 0 fail/skip/cancel/todo, 35,628s — không chứng minh chức năng bình chọn.

## Phạm vi sửa

Dùng các operation `text`, `vote`, `edit`, `delete` đã có trong ADR-0057 §5 và Rust. Không đổi protocol/version, native crypto, schema hoặc writer Go; không gửi tạo phiếu hay kết quả tới `/votes`. Nội dung `/vote câu hỏi? lựa chọn | lựa chọn` vẫn đọc được như text trên client cũ. Client mới chỉ dựng thẻ sau khi MLS đã xác thực/giải mã operation. Client cũ chưa có giao diện bỏ phiếu cần cập nhật; không coi việc đọc text là tương đương tính năng mới.

- Option ID suy ra cố định từ logical send ID của poll và thứ tự lựa chọn, có scope trong poll, đúng dạng UUID Rust chấp nhận. Không phụ thuộc random state hoặc API poll legacy.
- Mỗi actor một phiếu; đổi phiếu thay lựa chọn, replay không tăng tổng. Target không tồn tại/ngoài lựa chọn, poll xoá hoặc đóng không nhận phiếu.
- Chỉ tác giả đóng bằng edit mang đúng nội dung ban đầu cùng dòng «Bình chọn đã đóng.»; không sửa câu/lựa chọn sau phát, không mở lại poll đã đóng. Xoá vẫn cần tác giả.
- Kết quả native là projection của sổ operation niêm trên thiết bị theo sequence Go, có kiểu view riêng; không giả làm `CuocBinhChonWire`, không tự nhận thẻ AI/receipt máy chủ. Poll legacy tiếp tục dùng response và writer Go hiện có.
- Phiếu chờ/lỗi chưa được tính. Sau sequence/receipt thật mới thay kết quả. Thao tác chưa xác nhận báo rõ cần xem lại/thử lại, không hứa tự phục hồi.

UI dùng thẻ phiếu hiện có với writer native rõ ràng: bỏ phiếu/đóng/đọc lại đi qua v2, không gọi các route phiếu legacy. Thẻ native giữ tác giả người dùng. Liên kết từ poll native đã đóng sang tờ hẹn chung chưa tích hợp: không đưa UUID v2 vào `from_vote_id` của API legacy; nút tạo kèo riêng của chat vẫn có. Đây là phần còn phải xử lý, không nhận toàn luồng đã đủ.

Focused candidate: 27/27 PASS, 0 fail/skip/cancel/todo, gồm 8 ca bình chọn và hồi quy sổ/AI/khay. Typecheck exit 0; guide 62 route/hash `f14772123307` không đổi. Chưa nhận full clean-tree ở candidate, native hai người bỏ/đổi phiếu/đóng/relaunch, media, canary/mutant cuối hoặc crypto review độc lập.

Full mobile tại cây sạch `6a8883c3`: **1616 ca, 1615 PASS, 1 FAIL, 0 skip/cancel/todo**, 35,076s; cây sạch trước/sau. Gate vòng import nhận dependency type-only giữa projection và view là vòng mới. Bỏ dependency ngược bằng kiểu input tối thiểu, giữ scanner/danh sách ghim nguyên bản. Focused vòng import + 8 ca poll đạt 11/11, typecheck exit 0; cần full rerun ở SHA follow-up trước khi nhận xanh. Chưa nạp candidate lên Metro hoặc nhận native poll đã đạt.


## Bổ sung kiểm native và đăng ký thiết bị

Full mobile cây sạch `04b95dd4`: **1616/1616 PASS, 0 fail/skip/cancel/todo**, 35,049s, sạch trước/sau. Metro đã nạp đúng SHA; APK nền `b87a16f1` có SHA256 `34e48965242785651cc982199e4b3d823d45d0b4b30a79a98a622080efd63967`. Không nhận APK là build từ SHA JS mới.

Native dựng được poll cũ và tạo poll mới qua form câu hỏi/hai lựa chọn; root đã mở ảnh. Peer mới chưa nhận Welcome: runtime Go `1b0e022f` trả `ready=true` với một member nhưng hai expected device. Đăng ký device mới không invalidates các phòng đã ready. Test PostgreSQL mới `TestNewDeviceInvalidatesReadyRoomsUntilAdded` đỏ đúng «new device left a stale room ready» trên baseline (0,778s); sửa đăng ký device mới đặt `ready=false` cho các phòng active trong cùng transaction. Đăng ký lại cùng khóa giữ idempotency, không ngắt phòng. Package chatv2 PostgreSQL thật **37/37 PASS, 0 FAIL/SKIP**, 6,926s, gồm chặn send trước Add và send được sau commit Add. Đây là candidate check, chưa phải gate cây sạch/main hoặc proof peer native bỏ phiếu.

Ngoài ra, đăng xuất session peer qua API hiện chưa thu hồi chat device trên runtime Go `1b0e022f`, trái ADR-0057 §2. Thiết bị cũ đã được thu hồi riêng qua `DELETE /v2/chat/devices/{id}` (204); không nhận việc đăng xuất là PASS, không gán thao tác API thành bằng chứng UI native. Cần xử lý vòng đời thiết bị/phiên trước khi nhận toàn luồng thiết bị.
