# Review độc lập — mốc đồng bộ client chat

Reviewer `/root/chat_live_stack`, tác giả implementation `/root`.
Verdict: **APPROVE cho thay đổi mốc nhận/phân trang trong `useTinNhan.ts`**,
không phải approve production hoặc toàn bộ feature chat.

Source được review có SHA-256
`d39f6bc31d8e65c23f1fc25455cc3d1278123e45691aa48e187f608dff050d7f`.
Test hook sau khi reviewer bổ sung có SHA-256
`393bc6dabfd861dc75290c1e65cecb93d96398a8b55e7f14dc597bbbabaf76c4`.

Đã đọc implementation và contract backend thật: trang đầu/newest-first;
trang `after`/oldest-first; `next_cursor` là phần tử cuối của từng trang.
POST ACK chỉ xác nhận tin vừa ghi, không xác nhận người gửi đã nhận toàn bộ
tin của những người khác trước thời điểm đó.

Reviewer yêu cầu sửa và tác giả đã đóng ba điểm:

1. Không dùng phần tử đầu của trang ASC làm mốc mới nhất. Trang tiến phải dùng
   `next_cursor`; snapshot đầu dùng cursor đầu trang newest-first.
2. Trang đầu và poll dùng cùng khoá đang nhận theo generation, tránh hai
   snapshot không liên tục ghi đè mốc nhau. Regression trả snapshot mới trước
   snapshot cũ, rồi kiểm 150 tin, bắt khoảng trống 50 tin ở giữa.
3. Nhóm ban đầu rỗng nhận burst vượt một trang phải cập nhật `hetTinCu` theo
   `has_more` của snapshot mới. Reviewer đã chạy test đỏ `true !== false`
   trước sửa; nếu giữ cờ nhóm rỗng cũ, người dùng không thể cuộn tới các tin
   bị đẩy khỏi trang mới nhất.

Reviewer tự chạy `tsc -p tsconfig.test.json`, fixup ESM và toàn bộ file
`tests/rudi-chat-useTinNhan.test.mjs`: **19/19 đạt, không skip**. Bao gồm own
send không nhảy mốc nhận, nhiều trang ASC, race hai snapshot, burst từ nhóm
rỗng, ACK bản nháp, retry và chuyển group/account/unmount. Đây là test hook
React thật với transport điều khiển thời điểm; không thay bài browser E2E.

Điều kiện nằm ngoài approval: root phải chạy lại 20 browser từ bundle mới;
polling hiện tại vẫn chưa truyền reaction/delete từ người khác tới tin đã
hiển thị. Backend legacy cursor `(created_at,id)` chưa chứng minh tránh
late-commit gap. Native Android/iOS, E2EE, crypto review, AI plan thật và test
tải v2 là các cổng riêng, không được suy ra từ 19 test xanh này.
