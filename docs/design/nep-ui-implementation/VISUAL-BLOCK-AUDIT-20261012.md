# Rà thẩm mỹ các khối giao diện — 12/10/2026

## Quy tắc bắt buộc từ chủ sản phẩm

Kỹ sư tự mở và đánh giá từng màn, từng feature, từng scene; tự tìm và sửa các khối còn thô, nhồi chức năng, phân bố lệch, chữ/nút/spacing thiếu tổ chức. Không chờ người dùng nhắc từng chỗ. Đẹp, gọn, có thứ bậc và có câu chuyện riêng của Rủ Đi là điều kiện độc lập với dùng được và test xanh. Đối chiếu mockup; tài sản hình ảnh chỉ thêm khi có vai trò thật. Không thêm minh hoạ để che bố cục yếu.

Audit chỉ app Android trong emulator. Mỗi kết luận cần ảnh thật của đúng source, mở xem, chỉ rõ light/dark/cỡ chữ/loading/empty/error/busy nào thực sự được kiểm. Sửa code rồi kiểm lại; những cảnh chưa mở vẫn là chưa kiểm. Theo dõi host/guest/PSS/PID/OOM và dừng native sở hữu khi chạy gate/code. Giữ tài nguyên audit khác.

| Khối/cảnh | Bằng chứng hiện có | Đánh giá và bước tiếp |
|---|---|---|
| Cá nhân: identity→năm số→tabs100 | Ảnh1ff `source-account100.png`, chủ sản phẩm chỉ rõ khối số xấu | Thiết kế lại cấu trúc kể chuyện; bỏ cảm giác bảng thống kê. Giữ đúng5count server và chức năng editor; kiểmzero/số dài/200/dark. |
| Bảng tin: tác giả other100→200→100 | Ảnh1ff `community-confirmed100`, `community-author-hot200`, `community-author-return100` đã mở | Tên/ngày230dp và controls48 đã reflow; density tabs→author41dp còn rộng. Own/longname/dark mới chưa mở. |
| Bảng tin: header/menu/query | Ảnh f501 sáng và source1ff mới | Header/menu có cải thiện; chủ động rà group khoảng cách, scene saved/mine/empty/error/hidden-undo. |
| Composer/các khay bài viết | Có ảnh lịch sử; chưa có review toàn cảnh1ff | Rà tổ chức identity/audience, nhóm công cụ/chips/CTA và hierarchy khay, chụp lại source sau sửa. |
| Inbox/chat/công cụ đính kèm | Có ảnh lịch sử; chưa có review toàn cảnh1ff | Rà metadata thừa, search/tạo nhóm nhỏ, composer gọn, tooltray có tổ chức tại100/200/dark/IME. |
| Khám phá/place/Plan/create/bill/group/diary/settings | Review có giới hạn ở các pha trước; chưa phủ mọi scene | Mở từng luồng và các state riêng, ghi lỗi cụ thể trước sửa; không nhận toàn màn đạt từ source hoặc filename capture. |

Các đường ảnh trên thuộc `/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-1ff5c7d8/root-native-screens/`; ledger [native](NATIVE-UI-QC-20261008.md) ghi source, raw gate, bộ nhớ và giới hạn. Toàn app chưa hoàn tất và chưa merge main.
