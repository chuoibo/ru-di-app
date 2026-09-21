# Bộ đọc hình — không đưa đáp án cho người đọc

Chỉ phát **thư mục `nguoi-doc`** (hoặc đúng file HTML của mã được giao).
`dap-an-dieu-phoi.md` dành riêng người điều phối; không gửi cùng thư mục cha,
không chiếu màn hình chứa đáp án hay dùng tên cảnh trong lời giới thiệu.

Mỗi trang chỉ có mã H01…H20, hình và hai câu hỏi mở. Tên file, title và alt chỉ
chứa mã trung tính. Trang không tải script, font, analytics hoặc dịch vụ ngoài;
có thể mở file HTML cục bộ. Không có ô lưu câu trả lời vào repository.

## Cách dùng

1. Chọn người chưa đọc brief hoặc các bảng có caption cũ. Người đã xem đáp án
   chỉ dùng để nhận xét chất lượng hình, không tính vào nhóm đọc lần đầu.
2. Giao một biến thể của mỗi ý nghĩa cho mỗi người: bản đủ **hoặc** gọn, sáng
   **hoặc** tối. Dùng đáp án riêng để phân nhóm; đừng cho xem hai bản cùng ý
   trước khi ghi câu trả lời đầu tiên. Đổi thứ tự mã giữa người đọc.
3. Mở từng trang mã. Hỏi đúng hai câu in sẵn; không nói “đây là quán nướng”,
   “có thấy cây thông không”, hay đọc nhãn nghiệp vụ trước.
4. Ghi nguyên câu trả lời và mức chắc chắn tự báo trước khi lộ nhãn. Với hình
   cảnh, tách “nhận ra đồ vật/cảnh” khỏi “đoán được trạng thái trong app”.
5. Sau lượt đầu mới cho xem hình trong màn thật kèm tiêu đề để kiểm tác dụng
   của hình trong ngữ cảnh. Đừng bắt hình minh hoạ thay toàn bộ câu chữ.

Cặp cùng ý nghĩa: H01/H11 (góc cửa), H02/H13 (xe), H03/H19 (thoại có/không Nếp),
H04/H16 (đồi), H05/H18 (quầy đêm), H07/H14 (hiên), H08/H17 (tiền), H10/H20
(loại chưa có hình). H06/H09/H12/H15 là các cảnh riêng.

Không bịa tỷ lệ hoặc diễn giải khác nghĩa thành “đạt”. Không đưa tên, bản ghi
âm hoặc câu trả lời thật của người tham gia vào Git/worktree; thực hiện theo
quy định storage và consent của team. Bộ này chỉ chuẩn bị được phép thử,
**chưa có lượt người dùng nào được thực hiện**.

## Nguồn và giới hạn

SVG inline được sinh từ `hinhKyHoa`, `hinhCanh`, `hinhSticker` đã biên dịch của app,
đúng nhánh đủ/gọn và màu token. Không sửa hình bằng tay, không cắt screenshot
có caption để gọi thành phép đọc mù sau khi người đọc đã thấy đáp án.

Đây là bản vẽ vector độc lập, **không phải ảnh chụp native**. Trang dùng nền
token phẳng để trình bày hình; không mô phỏng vân vải, viền/bo của container,
font scale hệ điều hành hay độ sáng OLED. Kích thước sticker giữ 64/120 CSS px,
không quy CSS px thành dp của mọi điện thoại.

Sinh lại vào thư mục rỗng (từ `apps/mobile`):

```sh
npx tsc -p tsconfig.test.json
node tools/fixup-esm.mjs
node tools/xuat-bo-doc-hinh.mjs /tmp/rudi-bo-doc-hinh-moi
```

Đối chiếu bộ mới trước khi phát. Generator từ chối ghi đè một thư mục đã có
nội dung để không âm thầm đổi mã/hình giữa các lượt đọc.
