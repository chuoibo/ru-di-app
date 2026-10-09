# K — sửa sổ theo từng trang

Phạm vi: editor, preview và pager của sổ kỷ niệm trên Android. Đây là bước
tiếp theo của audit A–S, chưa nghiệm thu toàn K hoặc toàn app.

THESIS: Sửa một trang đang chọn, thấy rõ trang nào và xem lại như người đọc.
OWN-WORLD: Bìa vải chàm, nhãn giấy, Bricolage và mực/coral sẵn có; ảnh thật
của cuốn sổ mang nội dung. Không thêm tranh trang trí để che form.
STORY: Tên/lời mở → trang đang sửa → quyền xem → lưu có xác nhận thật.
FIRST VIEWPORT: Header nhỏ với đổi chế độ đọc/sửa; mở thẳng vào nội dung.
FORM: Bìa tự giãn, ảnh có tỷ lệ ngay khung đầu; pager 48dp, form một trang,
công cụ phụ gọn và chữ lớn được xuống dòng. Giữ bản nháp khi chuyển trang.
FINISH: Mở ảnh native light/dark và chữ1/1.3/2, kiểm IME/lưu/pager, xem clip
normal/Reduce Motion, review độc lập và ghi phạm vi chưa kiểm.

Before: source797, sổ tổng hợp private revision2/hai trang. Native thêm trang,
lưu và đọc trước/sau thành công. Form cũ xếp tất cả trang thành danh sách dài.
Clip trước phát hiện nguy cơ bìa co/nhảy do width1 và đo chiều cao hồi tiếp;
ảnh tải chưa có nền giấy. Không nhận motionPASS chỉ từ ảnh tĩnh.

Giữ contract revision/audience, lỗi và retry, chọn ảnh rõ ràng, nguồn AI thật,
không đọc chat tự động, không đổi E2EE hoặc domain ledger. Bản K ImageGen
ngoài repo vẫn chỉ là candidate chưa duyệt; batch này không dùng raster mới.
