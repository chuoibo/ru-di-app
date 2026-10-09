# H — Nhóm và bạn bè

Tinh chỉnh layout/harden, mode Operate, kế thừa UI3/seed `c8e88116`.
Không mở world mới hoặc thêm raster. Mockup03 là critique-reference về
nhãn tên trên bìa, người nhận rõ và thứ bậc thao tác; không sao chép tranh nền.

THESIS: Người và lời rủ dẫn mắt; bìa/phong bì phải chứa thao tác thật.

OWN-WORLD: Giữ bìa vải, nhãn giấy, mực người, coral và Bricolage.

STORY: Tên hội → mở nhóm; tài khoản → người nhận → gửi → chờ đồng ý.

FIRST VIEWPORT: Thấy ô nhập hoặc người ngay. Bàn phím vẫn thấy nội dung
đang viết và nút chính. Danh sách không dành đáy màn cho CTA thêm bạn lớn.

FORM: Tên nằm trên nhãn bìa tự giãn. Mời/kết bạn là thư có người nhận.
Header cố định, nút thêm nhỏ48dp, danh sách thành viên trước lối sang tường/
album; quyền quản trị và xác nhận giữ nguyên. Chữ130/200 xuống dòng.

FINISH: Chụp Android thực, so mockup, reviewer fresh, sửa finding rồi verdict
pass; documenter ghi từ bản đã kiểm. Không đổi test xanh thành điểm thẩm mỹ.

Before source5cab:18 PNG, hai flow sáng100/tối200,43,170s/44,924s exit0;
root mở đủ18. New/Invite/AddFriend đẩy ô nhập khỏi vùng thấy khi IME mở;
Friends có CTA footer lớn; Members đặt hai shortcut trước danh sách người.
Lưu ngoài repo tại `native-audit-5cab4701/captures/h-before-*`.

Phạm vi source: groups/New, Invite, Members; friends/AddFriend, Friends và
HangNguoi. Giữ API/idempotency/role/consent/draft/error tại chỗ, không đổi
E2EE/ledger hoặc tạo quyền/membership giả. Thành công chỉ sau response thật.

QC cần: sáng/tối ×100/130/200, IME trên/dưới, tên dài; lookup invalid/self/
not-found/found, gửi pending/duplicate; tạo nhóm/đọc membership, invite/accept/
reject/role/last-admin/leave theo route thật; error/retry/double tap, back.
Không gán review phần form/list thành nghiệm thu toàn H hoặc A–S.

Candidate a202: sáu flow Android sáng/tối ×100/130/200 exit0, 54 PNG;
root mở 9 ảnh sáng100 và 5 contact sheet đủ45 ảnh còn lại. First-pass
thấy footer ba form sát gesture bar khi IME đóng; lượt sửa thêm inset
thiết bị +12dp, IME mở vẫn do shell chừa8dp. Không sửa vòng lặp thẩm mỹ.
Clean exact-a202: typecheck7,873s xanh, guide1,388s đỏ vì bản đồ hướng dẫn
tự sinh cũ. Regenerate đúng tool, không sửa test hoặc bỏ bước. Capture/check
candidate kế tiếp và finish-review vẫn đang chờ; toàn H chưa nghiệm thu.

Clean af37: Node214 file1607/1608, fail1 vì flow24 còn đợi heading
"Gửi một lời rủ" đã đổi; sửa flow thành "Bạn muốn rủ ai?", không đổi corpus
hay miễn test. Matrix af37 sáng100/tối100 xanh, sáng130 dừng ở transport
"device not connected" khi máy5562 vừa boot. Giữ nguyên log/capture đỏ,
chạy lại sau khi cả hai thiết bị ổn định. Máy5562 giữ userdata fixture_n5g;
attach ban đầu dùng nhầm scheme exp+rudi, sửa về rudi như helper chuẩn.
