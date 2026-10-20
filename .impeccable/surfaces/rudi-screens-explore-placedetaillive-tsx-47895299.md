---
version: 1
slug: "rudi-screens-explore-placedetaillive-tsx-47895299"
primary_target: "apps/mobile/src/rudi/screens/explore/PlaceDetailLive.tsx"
related_targets: ["apps/mobile/src/screens/kham-pha/chi-tiet-dia-diem.ts"]
---

# Chi tiết quán · từng chỉ mục thay cho tường chữ (PR #667)

Mode: Read. Code-led, kế thừa Sân Khấu Giấy v3. Ghi sau khi đã sửa mã (đợt 20/10 theo ảnh iPhone của chủ sản phẩm): hợp đồng mô tả hướng đã làm để reviewer chấm.

## Direction contract

THESIS: Đọc một quán trong mười giây: vì sao đáng đến, món nên thử, giá, giờ; mỗi điều một chỉ mục có tiêu đề, không đoạn văn dài.

OWN-WORLD: Giấy, mực, coral; Card/SectionHeader/Chip sẵn có; ảnh thật qua MediaSlot với đúng một dòng ghi công; tranh cảnh vẽ dự phòng không nhãn chữ.

STORY: Một ảnh bìa (lật trang tại chỗ) → tên, điểm, trạng thái mở → Tại sao đáng đến → khối dữ kiện (giá «Ước khoảng» khi ước, giờ hôm nay + giờ đẹp nhất, thời lượng, đặt trước) → chip không khí → Món nên thử → Không khí → Hợp với ai → Giờ theo thứ → Trải nghiệm → Lưu ý.

FIRST VIEWPORT: 390×844: ảnh bìa ≤ 45% màn, tên, điểm/trạng thái, một câu «tại sao đáng đến», khối dữ kiện (giá, hôm nay, giờ đẹp nhất, thời gian, đặt chỗ). Hàng chip không khí ngay dưới nếp gấp: dữ kiện trả lời «có đi được không» trước «không khí ra sao».

FORM: Seed: none — mở rộng thế giới đã có (Sân Khấu Giấy v3, ADR-0037); concept-seed chỉ dành cho thế giới mới hoặc thay thế, nên không roll. Dữ liệu từ `highlights` máy chủ; mục rỗng ẩn hẳn, không «Chưa có…»; giá nói một lần; không bao giờ hiện bằng chứng hay khẳng định an toàn dị ứng.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
