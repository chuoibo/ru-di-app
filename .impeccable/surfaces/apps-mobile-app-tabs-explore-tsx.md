---
version: 1
slug: "apps-mobile-app-tabs-explore-tsx"
primary_target: "apps/mobile/app/(tabs)/explore.tsx"
related_targets: ["apps/mobile/app/(tabs)/community.tsx","apps/mobile/src/rudi/ui/RudiTabBar.tsx"]
---

# Surface brief · Khám phá và thanh tab

<!-- impeccable:surface-brief 1 -->

**Route:** `apps/mobile/app/(tabs)/explore.tsx` (mục Địa điểm), `apps/mobile/app/(tabs)/community.tsx` (mục Cộng
đồng, route không cột), thanh tab `apps/mobile/src/rudi/ui/RudiTabBar.tsx` + con dấu `ConDauTao.tsx`.
**Mode:** thanh tab = Operate; Khám phá = Experience nhẹ (`docs/claude/2026-10-01/ui-ux-upgrade/direction.md`).
Phần mở rộng bản sắc hiện hữu (giấy, mực, con dấu, băng coral); giữ `DESIGN.md` và token dùng chung. Build
code-led; tham chiếu duyệt là mockup ba bảng của chủ sản phẩm (01/10: «Bố cục đề xuất», «Tối & responsive»,
«Thanh tab 5 cột»), giữ ngoài Git ở `~/.local/share/rudi-ux/thanh-tab-5/mockup/`. Spec:
`docs/claude/2026-10-01/thanh-tab-5-cot/spec.md`.

## Direction contract

**THESIS:** Một tab trả lời câu «đi đâu?» bằng hai mục, Địa điểm và Cộng đồng; con dấu «Tạo» nằm đúng tâm một thanh
năm ô. Từ chối thanh sáu cột (con dấu lệch), từ chối băng cuộn ngang thay cho thẻ đầu, từ chối bọc bài viết trong
thẻ trắng.

**OWN-WORLD:** Nền giấy `ground` có vân, mực `ink`; chữ hai mục Bricolage `h2`, mục đang mở màu mực trên băng coral
28×4 như thanh tab, mục kia `inkFaint`. Chip viền `lineStrong`, chip chọn nền `accentSoft` chữ coral. Thẻ đầu nền
`card` viền `line` bo `control`, tim tròn trên ảnh, chip lý do tím `aiSoft`, chip giá nền giấy. Bài cộng đồng nằm
thẳng trên trang, ngăn bằng nét mảnh, ảnh rộng hết cột.

**STORY:** Mở app là thấy thành phố đang chọn và những chỗ hay ở đó; chạm «Cộng đồng» để đọc chuyện người khác đã đi;
chạm «+» ở giữa thanh để làm việc hợp với chỗ đang đứng (Viết bài ở Cộng đồng, Tạo cuộc hẹn ở Địa điểm).

**FIRST VIEWPORT:** (390dp) hàng «Địa điểm  Cộng đồng» trên cùng, cố định; dòng 📍 tên thành phố màu mực + «· đổi nơi
khác ▾» coral; sân khấu thành phố tràn hết bề ngang; ô tìm + nút ✨; hàng chip loại; «Chỗ hay ở …» với «N nơi» nhỏ;
nửa trên thẻ đầu. Thanh tab năm ô bằng nhau, con dấu coral 56dp nhô nửa trên mép ở ô giữa.

**FORM:** Extension của thế giới «nhật ký chuyến đi» đã có (không concept-seed, không đổi bản sắc). Hàng tiêu đề là
`tablist` hai tab; ba chế độ bảng tin là chip-tab; route Cộng đồng `href: null` sáng cột Khám phá. Chuyển động:
chỉ báo băng trượt `standard` như hiện có, không thêm hiệu ứng mới.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Phạm vi bằng chứng

Dữ liệu tổng hợp (persona `dalat-0` của stack QA, hoặc bản demo). Không tạo raster mới để ship. Web export cho bố cục
và đo đạc; Android emulator cho cỡ chữ 1.3 và cảm giác native; iOS không chụp được trên máy Linux này.
