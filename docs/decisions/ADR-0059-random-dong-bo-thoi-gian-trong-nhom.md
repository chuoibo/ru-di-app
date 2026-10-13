# ADR-0059 — Lượt chọn có lịch chung trong phòng mã hoá

- Ngày: 2026-10-14.
- Trạng thái: **Triển khai theo yêu cầu chủ sản phẩm; các cổng hành vi, hai máy native và thị giác còn mở.** Bổ sung ADR-0058/0057, không sửa grammar lượt `v=1` hoặc `docs/protocol/v1/`.

## 1. Phạm vi và lời hứa

Lượt `v=1` hiện lưu cùng lựa chọn/kết quả trong nhóm nhưng chạy lại chuyển động trên từng máy. Lượt mới `v=2` có một lịch chung: đếm ngược, diễn và hiện kết quả theo thời gian đã ghi trên làn. Đây là mô phỏng một kết quả đã khóa, không phải cuộc thi tính vị trí trên máy chủ. Không phát frame/tick qua mạng, không có cược, không tuyên bố chống gian lận hoặc công bằng do máy chủ chứng thực.

Đợt này chỉ kiểm trên các client cập nhật cùng hợp đồng trong QC cục bộ. Không có thương lượng phiên bản ứng dụng giữa các thiết bị; không tự nhận tất cả thành viên dùng client hỗ trợ. Client cũ có thể coi grammar mới là text thường, gồm trailer kỹ thuật. Không bật production hoặc nói tương thích hiển thị client cũ chỉ vì có dòng fallback. Lượt cũ vẫn được đọc và xem lại như cũ.

## 2. Snapshot mã hoá và mốc thời gian

Grammar text mới `/rudi-random-live-v2` đi trong op MLS `text`/`reply` sẵn có. Snapshot schema đóng giữ `id`, request, mode, 2–8 options, seed và winner như v1, thêm profile lịch `shared-v1`. Parser chỉ gắn metadata sau khi giải mã MLS và khi id trùng logical send id. Kiểm canonical JSON, winner, fallback, nguồn và trần 6.000 ký tự/12KiB như v1. Dòng fallback thông báo lượt đã khóa, không lộ tên thắng trước hạn. Tin edit không được nâng thành lượt; lượt đã nhận bất biến, xoá vẫn theo quyền tác giả.

Profile `shared-v1`: đếm ngược 6.000ms; vòng quay diễn 6.000ms, đua vịt diễn 12.000ms. Bắt đầu = `created_at` của chính event được làn nhận + đếm ngược. Kết thúc = bắt đầu + thời gian diễn. `created_at` là giờ DB khi INSERT, có thể trước lúc commit/ACK hoàn tất; phản hồi đến muộn phải vào đúng pha đã qua, không bắt đầu lại. Sequence mới là thứ tự bền của phòng. Retry cùng logical id giữ event/sequence/time cũ; không gửi control start thứ hai.

Sequence/time nằm ngoài chữ ký MLS/envelope: chúng là metadata của transport máy chủ được tin cậy về lịch, không phải receipt công bằng. Không lấy timestamp người tổ chức tự khai hoặc giờ nhận HTTP trên máy làm mốc chung. Không đọc nội dung ở máy chủ, không giữ khoá, không fallback plaintext, không đổi native ABI.

## 3. Đồng hồ và vòng đời

Go thêm GET clock chỉ đọc, xác thực bearer và quyền room/device như đọc event; lấy `clock_timestamp()` trên cùng DB chứa event. Không ghi event, sequence, outbox, DDL hoặc nội dung trò chuyện. HTTP no-store. Clock sample khác `created_at` lịch sử và khác HTTP Date của host Go.

Client lấy tối đa ba sample hữu hạn khi vào/resume, đo RTT bằng monotonic clock và chọn sample có bất định nhỏ nhất (gồm 1ms làm tròn timestamp). Chỉ dùng khi bất định ≤250ms và tuổi sample ≤30 giây. Giờ ước lượng tiến bằng monotonic elapsed, không dùng thay đổi giờ hệ thống để chạy cuộc đua. Timestamp event vượt quá giờ ước lượng + bất định + 1 giây bị từ chối, không tạo đếm ngược vô hạn. Không có monotonic clock, clock endpoint lỗi, timestamp sai hoặc clock không đủ chắc thì giữ snapshot và nói chưa đồng bộ giờ, không tự diễn như đang live. Refresh khi foreground và sample hết hạn; mỗi phòng chỉ một reader clock, không probe theo từng card/frame. Background huỷ timer, vô hiệu hoá response thuộc thế hệ cũ và không bắt đầu probe tiếp; request đã gửi vẫn có thể hoàn tất trong timeout 2 giây. Resume lấy sample mới rồi seek đúng pha.

Progress lấy từ elapsed của lịch chung, không cộng số tick hoặc thời gian mount. Mở muộn trong đếm ngược chỉ hiện phần còn lại; mở giữa cuộc đua vào vị trí hiện tại; quá hạn hiện kết quả. Poll HTTP 3 giây hiện có vẫn là đường giao nhận đầu tiên: không hứa mọi người thấy đủ đếm ngược hoặc đồng thời từng frame. Không thêm WebSocket trong lát này. Người/thiết bị gia nhập sau event không đọc được trước `first_sequence`; lịch sử mã hoá chuyển sang thiết bị mới là việc riêng.

Giảm chuyển động của **v2** giữ cùng hạn hiện kết quả: hiển thị pha/đếm ngược tĩnh, không chạy vịt hay vòng quay. Đây là bổ sung rõ cho chính sách “kết quả ngay” của lượt v1. Seed cho phép client sửa đổi biết kết quả sớm; UI không phải cơ chế bí mật hay chống gian lận. “Xem lại” là diễn riêng trên máy, không gửi tin và không đổi lịch nhóm. Hai người tổ chức có thể tạo hai lượt độc lập; không có lease “một lượt duy nhất toàn nhóm”, pause hoặc speed chung.

## 4. Cổng bắt buộc

Unit/behavior kiểm codec đóng/canonical/id binding, edit/delete/retry/replay, inbox không lộ trailer hoặc kết quả trước hạn; clock RTT/bất định/expiry, wall-clock jump, monotonic regression, latejoin/resume và hạn reveal chung. PostgreSQL thật kiểm clock chỉ đọc, đúng DB time, quyền/revoke/first-sequence không bị nới và route ownership. Native hai client thật với hai actor cùng lúc kiểm cùng snapshot/anchor và độ lệch pha/hạn trong ngân sách đã đo; chuyển actor tuần tự trên một máy không chứng minh đồng bộ. Thêm mất ACK, mạng chậm/mất mạng, mở muộn, background/kill/reopen, một máy giảm chuyển động, font 100/200% và theme sáng/tối. Mở screenshot/video thật đối chiếu hình tham chiếu; test xanh không chứng minh độ đẹp hoặc clock native.

Không compile/test/runtime/DB từ agent chuẩn bị source. Root chạy các cổng tuần tự trên checkpoint sạch; không commit, deploy hoặc đổi cấu hình worker/provider trong lát này.
