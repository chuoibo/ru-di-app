# ADR-0058 — Nếp chọn giúp trong hội bạn: vòng quay và đua vịt

- Ngày: 2026-10-13.
- Trạng thái: **Quyết định sản phẩm đã được chủ sản phẩm yêu cầu trực tiếp; triển khai và các cổng còn mở.** Người dùng yêu cầu hai mode, `/random [yêu cầu]`, tìm lựa chọn rồi mô phỏng trong nhóm. Văn bản không chứng nhận production, crypto hay độ đẹp.
- Bổ sung ADR-0046/0057; không sửa `docs/protocol/v1/`, luật tiền, quyền chat hoặc wire Rust.

## 1. Trải nghiệm và nguồn

Tool «Nếp chọn giúp» và `/random` mở cùng một editor trong hội bạn. `/random yêu cầu` vẫn là tin thường được mã hóa trước; sau đó client gọi hàng đợi Go bằng command `random`, cùng preview/ngữ cảnh/quyền/retry hiện hành. `/random` trống chỉ mở editor, không bịa lời nhờ hay sáu món mặc định. Bình chọn vẫn là tính năng riêng.

Command `random` chỉ tìm ứng viên qua retrieval và grounding hiện có, không tạo lịch trình, poll hoặc chi tiêu. Kết quả AI giữ part `places` tối đa năm địa điểm và receipt/digest nguyên byte. Catalogue hiện có địa điểm, không có menu: UI nói «Tìm quán», hiện tên và nguồn địa điểm, không tự nhận tìm ra món hoặc xác minh ngân sách/thực đơn. Không tìm được thì nói rõ và cho tự thêm món/quán. Người dùng có thể thêm, sửa, bỏ tới tám lựa chọn; sửa tên có nguồn chuyển thành lựa chọn tự nhập. Không sửa `ai_card` để làm game.

Tên «Nếp chọn giúp» là ngoại lệ sản phẩm mới cho công cụ chọn ngẫu nhiên trong nhóm so với tên bot nhóm lịch sử ADR-0036/0046. Không thay danh tính/receipt/giọng của các trả lời AI cũ. Nếp giấy canonical giữ nguyên; vịt chỉ là quân đua.

## 2. Một lượt, hai cách diễn

Hai mode `wheel`/`ducks` dùng cùng danh sách đã khóa và cùng thuật toán rút. Người tổ chức sửa bản nháp; tính năng không hứa đồng biên tập. Trước khi chơi cần 2–8 tên khác nhau, mỗi tên tối đa 80 ký tự, lời nhờ tối đa 300 ký tự. Không có cược, tiền thưởng hoặc ghi sổ.

Client tạo một seed 128 bit từ bốn prefix uint32 độc lập của UUID v4 native Expo Core, không dùng `Math.random` hoặc fallback không an toàn. Android hiện dùng `UUID.randomUUID`; iOS dùng UUID native. Thuật toán `v=1` đọc bốn uint32 big-endian theo thứ tự: lấy từ đầu tiên nhỏ hơn `floor(2^32/n)*n`, rồi modulo n; nếu không có thì từ chối seed và tạo seed mới trước khi khóa lượt. Máy nhận tính lại winner, không tin winner khai trong JSON.

Đây là công cụ chọn vui do người tổ chức khởi tạo: người tổ chức có thể thử seed trên client sửa đổi. Không tuyên bố máy chủ chứng thực tính công bằng hay chống gian lận. Ticket server chỉ cần ADR bổ sung nếu sau này sản phẩm yêu cầu quyền rút độc lập với người tổ chức.

## 3. Lưu trong E2EE, bất biến khi thử lại

Một bản ghi `RandomRound` có version, id, request, mode, options, seed, winner. Grammar text `/rudi-random-v1` có dòng kết quả đọc được trước JSON, trần 6.000 ký tự/12KiB; parser đóng schema và kiểm kết quả/các giới hạn. Chỉ parse sau MLS xác thực và giải mã. Tin là của người tổ chức, **không phải câu trả lời AI đã xác minh**. `placeId` chỉ là dẫn nguồn người tổ chức đính kèm, không được trình bày như receipt AI.

`id` của lượt vẫn là UUID viết thường; `placeId` là khóa text opaque của catalogue, gồm cả `q-<UUID>`, slug seed và `osm-<loại>-<id>`, không bắt buộc UUID. Giữ nguyên codepoint của khóa, kể cả NFD và khoảng trắng; không trim, đổi chữ hoa/thường hoặc chuẩn hóa NFC vì có thể trỏ sang khóa khác. Hợp đồng của lượt giới hạn khóa không trắng, tối đa 200 Unicode scalar/800 byte UTF-8, không ký tự điều khiển, bidi hay surrogate đơn. Hai khóa đúng bằng `.` và `..` chưa được hỗ trợ vì có thể bị chuẩn hóa thành điều hướng thư mục. Khóa chứa percent escape `/%[0-9a-f]{2}/i` cũng chưa được hỗ trợ: Expo Router hiện giải mã route param hai lần, làm đổi các khóa như `a%2Fb`, `%2e%2e`, `%252e`. Parser catalogue vẫn giữ nguyên khóa; riêng luồng random loại các khóa ngoài giới hạn trước khi hiện nút nguồn và trước khi phát/đọc round. Không double-encode để giả bảo toàn identity; cần sửa router dùng chung và kiểm chứng native trước khi mở rộng hợp đồng này. Nút nguồn luôn đi vào `/places/` nội bộ với toàn bộ khóa hợp lệ được encode thành một segment; dấu `/`, `?`, `#` và chuỗi giống URL trong khóa không trở thành URL hoặc tham số điều hướng. Dấu `%` không tạo escape hợp lệ vẫn được giữ. Nhãn và lời nhờ vẫn trim/NFC như trước.

Gửi một lần qua outbox MLS cùng logical id; lỗi gửi giữ nguyên round/seed/id và dùng retry outbox hiện có. Không rút lại khi render, background, đổi theme/font hoặc retry. Tin round hợp lệ không sửa được kể cả tác giả; xóa vẫn theo quyền tác giả sẵn có. Tin thường được sửa không được nâng thành round mới. Xem lại chạy cùng kết quả; «Lượt mới» tạo bản nháp/lượt mới. Không gửi choices plaintext qua legacy API, không thêm Rust op/native ABI.

Khi chưa gửi thành công, UI nói đang gửi/chưa gửi được; không nói cả nhóm đã nhận. Mô phỏng không tự tạo kèo. «Tạo kèo» đi vào luồng tạo thủ công để người dùng kiểm và xác nhận.

## 4. Hình thức và trợ năng

Hướng ImageGen `random-reference-v1` là tham chiếu phản biện, không phải ảnh native/comp đã nghiệm thu: vòng quay giấy làm trọng tâm; ao có làn bằng nhau; lời nhờ, mode và editor gọn; kết quả là thẻ trong chat. Sprite vịt riêng có provenance. Wheel/ao/control là native code, không raster toàn màn. Không thêm ảnh món bịa hoặc minh họa lấp chỗ.

Danh sách ngoài hình giữ tên đầy đủ; số trên wedge/làn nối với tên. Font lớn reflow và cuộn, không cap font hoặc cắt nội dung để giả fit. Target tối thiểu 48dp, cùng gutter; bàn phím không che input đang viết. Chuyển động hữu hạn, giảm chuyển động cho kết quả ngay; rời/mở lại không tự rút. Lịch sử nhường chỗ khi editor mở nhưng giữ reader/scroll instance và cổng quyền.

## 5. Cổng và giới hạn hiện tại

Go command/SQL migration/capability/metrics và client enums phải cùng hợp đồng; thêm migration, không đổi checksum migration cũ. Unit/contract kiểm grounding, parsing, winner/rejection, replay/immutability/retry. PostgreSQL thật kiểm enum/migration; không lấy fake làm bằng chứng DB. Native Android kiểm cả mode, nguồn thật/no source, editor/IME, light/dark, font 100/200%, reduced motion, retry/reload và hai thành viên thấy cùng lượt. Mở từng screenshot đối chiếu mockup; đánh giá đẹp độc lập với test xanh.

Tất cả cổng trên đang mở cho tính năng này. Không deploy production, không merge main, không thay thế audit tải/crypto/iOS/hardware/TalkBack bằng unit test. Tài nguyên audit được bảo vệ giữ nguyên.
