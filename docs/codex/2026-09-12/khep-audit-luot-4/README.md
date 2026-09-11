# Audit độc lập lượt 4 — mốc `f9f3b2d1`

Audit này trả lời câu hỏi “đã đủ state of the art chưa?” bằng bằng chứng có
phạm vi rõ. Kết luận là **chưa thể gọi toàn app state of the art**. Hướng
giấy–mực đã có bản sắc và đáng giữ; lượt này đóng hai lỗi còn thấy được trong
chữ lớn, siết cổng đo motion, và chuẩn bị một phép đọc mù có thể chạy lại.

## Thay đổi đã đưa vào PR

| Mục | Trước | Sau | Điều kiện còn mở |
|---|---|---|---|
| Cổng motion | Bộ đọc cho phép histogram sai định dạng và bỏ qua một số lỗi đọc scale | Runner v4 kiểm tra mã trả về, cấu hình scale ở biên từng cửa sổ, pid, histogram đúng cú pháp/thứ tự/tổng; canary 31 nhánh | Chưa có số đo release HTTPS, iOS hoặc máy thật |
| Hàng chuyến đi | Khoảng ngày bị cắt thành `17 - 1…` ở chữ lớn | Hàng hiển thị trọn `17 - 19/10/2026`; ngữ cảnh vẫn nằm ở biểu tượng và tiêu đề màn | Đã spot-check native; cần lặp trên máy thật |
| Ký hoạ bản gọn | Mô tả trợ năng còn nói “đồi thông” dù bản gọn bỏ cây | Mô tả theo đúng cảnh còn lại: “đồi nhìn từ lan can” | Độ hiểu không nhãn vẫn cần người ngoài brief |
| Bộ đọc mù | Gói cũ có caption/đáp án gần tệp phát | Generator phát 40 trang HTML, mã H01–H20, SVG inline và nhãn trung tính; đáp án nằm ngoài `nguoi-doc/` | Chưa có lượt người dùng nào được thực hiện |

## Bằng chứng

- [Canary v4 cuối](motion-canary-v4-final.txt): **31/31** nhánh đúng kỳ vọng,
  gồm histogram hỏng, lỗi lệnh, drift scale, restore lỗi và ngắt.
- [Đối chứng v3](motion-canary-v3-doi-chung.txt): bản runner ở base để lọt
  **14/31** nhánh; đây là lần chạy độc lập, không suy từ diff.
- [Review runner độc lập](review-motion.md): reviewer ghi `APPROVE` trong phạm
  vi runner/canary/test; không ký duyệt native, release hay toàn app.
- [Assessment A](assessment-a.md): đọc trực tiếp capture native tại mốc base,
  giữ hướng nghệ thuật nhưng chấm phần chưa có bằng chứng là `Unknown`.
- [Native spot-check](native-check.txt): emulator Android, font scale 2.0,
  UIAutomator đọc nguyên văn khoảng ngày sau khi bundle từ nhánh này nạp.
- [Hướng dẫn bộ đọc mù](bo-doc-hinh/HUONG-DAN.md): quy trình phát tệp, consent,
  câu hỏi mở và giới hạn diễn giải.

Ảnh native không được nhân bản vào PR này để tránh biến một bản chụp cũ thành
bằng chứng mới. Spot-check mới chỉ lưu kết quả XML dạng văn bản; nó không thay
cho iOS, máy thật, TalkBack/VoiceOver hoặc kiểm tra người dùng.

## Cổng closure

1. Chạy `node apps/mobile/tools/xuat-bo-doc-hinh.mjs` từ output rỗng và kiểm tra
   người đọc chỉ nhận `nguoi-doc/`; người điều phối giữ `dap-an-dieu-phoi.md`.
2. Chạy canary và runner trên dev client; không gọi kết quả fixture là release
   evidence. Spot-check font 2.0 phải giữ nguyên ngày trong XML và pixel.
3. Đo release qua HTTPS local, sau đó lặp ở iOS và máy thật với Reduce Motion,
   TalkBack/VoiceOver và cỡ chữ lớn.
4. Cho người chưa đọc brief xem các mã đã đổi thứ tự; ghi câu trả lời trước khi
   lộ tiêu đề. Không ghi tên, bản ghi âm hay câu trả lời thật vào Git/worktree.

## Kiểm tra lượt này

- `npm test`: **811 passed**.
- TypeScript app và test config: đạt; test art: đạt.
- Browser smoke: **40/40** trang HTML mở được, mỗi trang có heading/mã trung
  tính và đúng một SVG inline.
- `python3 -m pytest tests/test_motion_measurement_gate.py -q`: canary wrapper
  đạt.
- Full backend suite tại checkout audit: `3432 passed, 1 failed, 712 skipped`;
  ca fail là `PhotosSurviveANewContainer` vì stack container ảnh không sẵn, nằm
  ngoài các tệp PR. Không dùng lượt chạy này làm tuyên bố backend xanh.

Không có dữ liệu người dùng thật, credential, ảnh hóa đơn hoặc transcript trong
packet. Bản audit này là tài liệu review và chuẩn bị thử nghiệm, chưa phải kết
quả nghiên cứu người dùng.
