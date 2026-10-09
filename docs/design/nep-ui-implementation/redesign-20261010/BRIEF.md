# Thiết kế lại FE theo phản hồi 10/10/2026

**Chưa nghiệm thu toàn app.** Chủ sản phẩm đánh giá hình thức hiện tại khoảng
5/10; những verdict scoped trước không thay thế đánh giá này. Chỉ kiểm app
native trong Android emulator, không mở bản web. Giữ các tài nguyên audit tải dài.

## Hợp đồng hướng thiết kế

THESIS: Mỗi màn có một việc chính, chữ và điều khiển dẫn đường trước hình minh hoạ.
OWN-WORLD: Giữ nhận diện Rủ Đi, Bricolage, mực, giấy và coral; giữ Nếp canonical.
STORY: Danh sách giúp so sánh; form giúp viết/chọn; không biến mọi màn thành slide giới thiệu.
FIRST VIEWPORT: Thấy ngay tên màn, nội dung thật và thao tác chính. Lựa chọn phụ mở tại chỗ.
FORM: Gutter nhất quán, 48dp target, nhịp khoảng cách, chiều cao linh hoạt theo chữ và IME.
FINISH: Đối chiếu ảnh native, kiểm state/tương tác, review fresh-context và ghi rõ phạm vi còn mở.

## Chín yêu cầu cụ thể

- Hồ sơ: màn sửa riêng, cụm danh tính, trường tên/giới thiệu/thành phố gọn, giữ bản sửa và lỗi.
- Khám phá: điện thoại đọc theo một trục; thumbnail cùng kích thước, thông tin và giá đủ ngang.
- Danh sách địa điểm: tên, metadata, giá có thứ bậc, bỏ cột quá hẹp và căn giữa theo chiều dọc.
- Lên plan: dấu cộng nhỏ trong header; CTA tạo kèo ở empty state vẫn rõ.
- Khay Tạo: hình chia bill phải đọc ra hoá đơn và phần chia, không một mũi tên trên giấy trắng.
- Ngân sách: thanh kéo tương tác, nấc/giá trị/mức nhanh, trợ năng; số đồng nguyên và nhập chính xác.
- Tin nhắn: tìm tên người/nhóm và dấu cộng bên cạnh tiêu đề; giữ lời mời và E2EE.
- Back: glyph thẳng gutter nội dung, vẫn đủ vùng chạm và title không bị cắt.
- Viết bài: người đăng/người đọc nhỏ, vùng viết ưu tiên, công cụ phụ gọn; gửi công khai vẫn qua duyệt.

## Ảnh tham chiếu mới

[core-layout-reference-v1.png](core-layout-reference-v1.png) được sinh bằng
built-in ImageGen, từ bốn ảnh lỗi do chủ sản phẩm cung cấp. Đây là **reference
để phản biện cấu trúc**, chưa phải comp được chủ sản phẩm duyệt và chưa phải
ảnh app đã code. Người/tên/ảnh trong comp là tổng hợp. Không đưa các câu chữ,
ảnh người, trạng thái online hoặc dữ liệu tưởng tượng của comp vào app.
Giới hạn thật vẫn là bio500, bài5000; không chép số500/150 sai trong comp.

Prompt: A four-panel Android UI redesign reference for the existing Vietnamese
Rủ Đi app: compact profile editing with identity and composed fields; coherent
place lead and top-aligned list rows with full price; Messenger-like inbox with
small plus, search and real conversation hierarchy; compact community composer
with author/audience, generous writing area, secondary tools and honest moderation.
Preserve ivory #f7f4ec, ink #1f2230, coral #ba3e20, Bricolage-like sans, 16dp
aligned gutters and 48dp controls. No decorative artwork covering layout defects.

Bản mockup K và asset phong bì là candidate ngoài repo, chưa dùng/duyệt; không
sinh thêm giấy/hoa để che bố cục. Asset chia bill ở batch này là vector native
có nghĩa, đọc token light/dark, không cần raster mới. ImageGen hiện có thật;
config code-led cũ phản ánh preflight lịch sử, không chứng minh comp mới đã duyệt.

## Cổng đánh giá

Mỗi feature: hierarchy, typography, spacing/alignment, trạng thái, thao tác,
keyboard, chữ1.0/1.3/2.0, light/dark. Ảnh tĩnh không chứng minh motion hay TalkBack.
Mức đẹp phải được review độc lập theo rubric của audit A–S; test xanh chỉ là
bằng chứng kỹ thuật. Không chấm PASS toàn app hoặc landing main khi còn cổng mở.
