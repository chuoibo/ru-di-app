# Rà thẩm mỹ các khối giao diện — 12/10/2026

## Quy tắc bắt buộc từ chủ sản phẩm

Kỹ sư tự mở và đánh giá từng màn, từng feature, từng scene; tự tìm và sửa các khối còn thô, nhồi chức năng, phân bố lệch, chữ/nút/spacing thiếu tổ chức. Không chờ người dùng nhắc từng chỗ. Đẹp, gọn, có thứ bậc và có câu chuyện riêng của Rủ Đi là điều kiện độc lập với dùng được và test xanh. Đối chiếu mockup; tài sản hình ảnh chỉ thêm khi có vai trò thật. Không thêm minh hoạ để che bố cục yếu.

Audit chỉ app Android trong emulator. Mỗi kết luận cần ảnh thật của đúng source, mở xem, chỉ rõ light/dark/cỡ chữ/loading/empty/error/busy nào thực sự được kiểm. Sửa code rồi kiểm lại; những cảnh chưa mở vẫn là chưa kiểm. Theo dõi host/guest/PSS/PID/OOM và dừng native sở hữu khi chạy gate/code. Giữ tài nguyên audit khác.

## Lượt mở thật tại e1f77205

Root đã mở 17 PNG trong [review riêng](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-e1f77205-root-review-20261012-r1/root-review.json), pin 34 ảnh/XML; marker thật ở panel Tài khoản, PID 1861. `ready100` còn Bundling 34%, không tính là app sẵn sàng; hai ảnh tên `create-outing-before*` thực ra là khay Tạo từ tab Plan, không phải form. Form đúng là `outing-form-before*`. Không dùng tên file để suy diễn scene.

Chủ sản phẩm **tiếp tục từ chối** Profile hai dòng ghi chú tại e1: chữ nhỏ/xám, gạch lề, thiếu cấu trúc đẹp. Chưa đóng finding dù 1.877 test và typecheck xanh. Đã dùng skill imagegen built-in để thử ba hướng mockup; bản xem trước ở [concept board](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/profile-route-mockup-preview-20261012-r1/concept-board.png). Hướng đang triển khai là đường nối năm facts với icon giấy có nghĩa; dùng vector/native code hiện có, không chèn bitmap trang trí. Chữ lớn/fallback chuyển thành các hàng đầy đủ, không ép năm cột. Đây vẫn là thiết kế thử cho tới khi mở AFTER và đánh giá trên emulator.

Tự phát hiện thêm: khay Tạo 100% chỉ đọc trọn ba thao tác đầu vì hình lớn; hot 200% cắt title/context/nhãn. Form Kèo mới có hai tầng tiêu đề và nhiều từ nối cỡ headline; hot 200% nhiều chữ/ngày bị cắt. Cài đặt có hàng Back trống trước h1; hot 200% cả radio/helper/list text bị cắt, XML chưa đổi từ 100%. Các lỗi hot-font này cần owner/leaf phù hợp, không chỉ đổi spacing. Avatar chữ F của tác giả trên bảng tin cũng bị cắt tại 200%, là finding riêng đang mở.

Bảng tin e1 có khoảng đầu gọn hơn và tác giả đọc đầy đủ ở 100/200; return100 PNG và XML trùng baseline. Dark100 đã mở nhưng banner cập nhật đè lên search, nên không gọi đó là cùng trạng thái. Chưa kiểm own/long-name/all menus. Native lượt này có 483 mẫu, RAM host thấp nhất 18,775 GiB, guest sẵn sàng thấp nhất 538,207 MiB, app PSS cao nhất 866,890 MiB, PID 1861 không đổi, không safety stop hoặc scope OOM/OOM-kill. Đã dừng emulator/Metro sở hữu trước code/gate; core/legacy vẫn được xác minh còn chạy. Không nhận lỗi crash Yoga lịch sử đã được giải quyết.

Checkpoint thử tiếp theo gồm ba production paths: HoSoSong dùng năm fact có icon/đường nối, chữ lớn/fallback/số từ năm chữ số trở lên đọc theo hàng; Create dùng sáu hàng cùng trục, giữ nguyên routes/order/context/dismiss và Nếp nhỏ; CaiDatScreen dùng một TopBar và native metrics owner của chính màn, kế thừa owner hiện có kể cả unavailable. Không đổi shared defaults, money/backend/E2EE. PH04 chỉ đổi contract presentation đã bỏ, giữ năm giá trị/noun chính xác/a11y/readorder/host retention/zero-số dài/no-I/O; PH01–03/05 giữ nguyên byte. [Focused thật](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/memory-bounded-checks/profile-route-create-settings-e1f-focused-r1/result.json): 27/27, không fail/cancel/skip/todo, 4,019 s, peak 176,492 MiB, host min 22,723 GiB, không stop. Đây là cây triển khai trước commit; clean đúng SHA mới và native AFTER còn chờ. Host mới cho actual Create/Settings owner và controls canary/identity/hai mutants trước main vẫn mở; các static tests cũ không chứng minh hot-font trên hai màn này.

| Khối/cảnh | Bằng chứng hiện có | Đánh giá và bước tiếp |
|---|---|---|
| Cá nhân: identity→năm số→tabs100 | Ảnh1ff `source-account100.png`, chủ sản phẩm chỉ rõ khối số xấu | Thiết kế lại cấu trúc kể chuyện; bỏ cảm giác bảng thống kê. Giữ đúng5count server và chức năng editor; kiểmzero/số dài/200/dark. |
| Bảng tin: tác giả other100→200→100 | Ảnh1ff `community-confirmed100`, `community-author-hot200`, `community-author-return100` đã mở | Tên/ngày230dp và controls48 đã reflow; density tabs→author41dp còn rộng. Own/longname/dark mới chưa mở. |
| Bảng tin: header/menu/query | Ảnh f501 sáng và source1ff mới | Header/menu có cải thiện; chủ động rà group khoảng cách, scene saved/mine/empty/error/hidden-undo. |
| Composer/các khay bài viết | Có ảnh lịch sử; chưa có review toàn cảnh1ff | Rà tổ chức identity/audience, nhóm công cụ/chips/CTA và hierarchy khay, chụp lại source sau sửa. |
| Inbox/chat/công cụ đính kèm | Có ảnh lịch sử; chưa có review toàn cảnh1ff | Rà metadata thừa, search/tạo nhóm nhỏ, composer gọn, tooltray có tổ chức tại100/200/dark/IME. |
| Khám phá/place/Plan/create/bill/group/diary/settings | Review có giới hạn ở các pha trước; chưa phủ mọi scene | Mở từng luồng và các state riêng, ghi lỗi cụ thể trước sửa; không nhận toàn màn đạt từ source hoặc filename capture. |

Các đường ảnh trên thuộc `/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-1ff5c7d8/root-native-screens/`; ledger [native](NATIVE-UI-QC-20261008.md) ghi source, raw gate, bộ nhớ và giới hạn. Toàn app chưa hoàn tất và chưa merge main.
