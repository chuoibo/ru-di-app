# Đề nghị bổ sung ADR-0019 và ADR-0021: nguồn riêng trong pair

**Trạng thái: ĐỀ XUẤT**, 2026-09-12 · Người đề xuất: Codex.
Chờ review độc lập và chấp nhận; **chưa thay hiệu lực** ADR đã được chấp nhận.
Đi kèm [ADR-0027](../ADR-0027-so-hai-nguoi-va-to-giay-co-phien-ban.md).

## 1. Vấn đề cần quyết định

ADR-0019 §2.1 không công khai gu cá nhân, nhưng cho hiện tổng hợp nhóm.
ADR-0021 §2.6.2 cho `pair` dùng cùng phép cộng. Với hai người, người biết phần
mình có thể suy ra phần người kia; ẩn tên trong kết quả không bảo đảm riêng tư.

Tại `f0173bad`, `ApiService.group_taste` đọc interests/budget của tất cả thành
viên active; companion gọi nó rồi tạo thẻ chung trong context. Vì vậy chỉ thêm
luật vào bộ phác giấy mới không đủ để giữ lời hứa của spec §7.3.

## 2. Khoản bổ sung được đề nghị chấp nhận

1. **Đối với `contexts.kind='pair'`**, không dùng interests/saved_places/budget
   riêng của hai người để tạo hoặc xếp hạng gợi ý chung. Không cho phép đường
   cộng tổng cũ đi vòng bằng catalogue, companion, bản đồ hay cache. Đây là
   ngoại lệ thay §2.6.2 ADR-0021 cho pair; không sửa thuật toán của `group`.
2. Gu riêng chỉ mồi cho view của chính chủ. Chia một lựa chọn tạo đối tượng
   chia cụ thể, có phạm vi người nhận và quyền thu hồi; không biến toàn bộ hồ
   sơ thành chung. Gửi nháp đã xem trước chia đúng phiên bản đầu ra, không chia
   quyền truy vấn ngược dữ liệu nguồn.
3. Với pair, xử lý **chat bằng mô hình để làm gợi ý chung** cần consent đang
   hiệu lực của cả hai, mặc định tắt. Áp cả cửa cũ và mới khi chuyển sang hợp
   đồng này. Một người gọi companion không thể đồng ý thay người còn lại.
   Quyền con người đọc DM theo ADR-0021 không đổi.
4. Quyền đọc nguồn và quyền đưa đầu ra sang người kia được kiểm riêng. Tự gửi
   chỉ dùng nguồn chung còn quyền; private-derived draft không được tự đổi nhãn
   thành public. Nguồn gồm cả lý do, budget, xếp hạng, tóm tắt và cache.
5. Một người thu hồi chat hoặc đóng sổ thì dừng xử lý mới và vô hiệu hóa các
   đầu ra chưa được công bố dựa vào quyền ấy. Một trang đã ghim có thể được
   giữ cho chủ đọc theo spec, nhưng không tự cấp lại quyền xử lý chat nguồn.
6. Hai ô «không ăn được»/«đừng» do chủ tự chia vào sổ nằm ngoài
   `person_interests`; người kia đọc, không sửa. Không nhập thay hoặc lấy từ gu
   toàn cục. Không tự gửi dữ liệu này tới mô hình bên ngoài.

## 3. Phạm vi đổi và đường chuyển tiếp

Đây **có thay đổi hành vi AI của pair hiện có**, kể cả pair chưa bật đôi.
Không suy consent hồi tố từ membership, friendship, lịch sử `/plan` hoặc từ
việc người ta tiếp tục dùng app. Trước consent, nguồn chung cho đề nghị là nội
dung chủ động chia để gợi ý và danh mục chung; không đọc lịch sử chat/gu riêng.
Không tắt chức năng con người nhắn tin, đọc tiền, tạo outing và giữ kỷ niệm.

Mọi caller nguồn pair dùng một policy ở máy chủ. Lát 1 phải liệt kê và kiểm
các caller thực tế, có ca âm trên cả route cũ. Đây là thay đổi được công khai
để người review chấp nhận, không được mô tả là «chỉ thêm bảng, hành vi cũ nguyên vẹn».

Không tuyên bố ADR này giải hết nguy cơ suy luận riêng tư của mọi nhóm nhỏ.
`kind='group'` giữ hợp đồng hiện có; nếu muốn thay luật nhóm hai thành viên,
đó là một thay đổi phạm vi riêng. Phần tin nhắn thoại của ADR-0019 giữ nguyên.

## 4. Điều kiện chấp nhận và kiểm chứng

Review độc lập cần xác nhận rõ ngoại lệ cho pair ở §2.6.2 ADR-0021, không chỉ
ký vào câu «n = 2 không nên cộng gu». Khi được chấp nhận, cập nhật dẫn chiếu
trạng thái ở ADR-0019/0021/0027 trong cùng PR quyết định; không gắn chữ đã chấp
nhận trước sự kiện ấy.

Khi hiện thực: synthetic dữ liệu riêng của mỗi người; kiểm cả nguồn đi vào
mô hình và đầu ra catalogue/companion. Consent off, một người grant, một người
revoke, cycle mới, cache cũ và draft tự gửi đều phải có ca âm. Group có fixture
hồi quy riêng. Những ca này chưa được chạy ở lượt tài liệu này.
