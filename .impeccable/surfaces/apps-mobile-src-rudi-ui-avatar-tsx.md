---
version: 1
slug: "apps-mobile-src-rudi-ui-avatar-tsx"
primary_target: "apps/mobile/src/rudi/ui/Avatar.tsx"
related_targets: []
---

# Avatar · chữ đầu giữ đúng khung khi chữ hệ thống đổi

Mode Operate/Read; refinement harden→polish. Base6c737f8df7f5861663ed8b34bf7bfe52744d6c9b, Avatar bytes bằngd8. Không seed/brand/asset mới.

## Direction contract

**THESIS:** Khi không có ảnh, chữ đầu nhận diện người vẫn đọc được như một dấu mực trọn vẹn; không thành mảnh glyph khi đổi chữ hệ thống.

**OWN-WORLD:** Giữ vòng mực riêng của người, giấy sáng/tối, badge42 trong target48 và đường ảnh thật hiện có. Không thêm tranh mặt người, phóng ảnh hoặc đổi màu để che lỗi.

**STORY:** Ảnh người dùng nếu được phép → chữ đầu khi không có/ảnh lỗi; tên đầy đủ và route hồ sơ vẫn là nghĩa chính. Chuyển font giữ cùng người, badge, Text host và action.

**FIRST VIEWPORT:** Native d8 phone/tablet200 có F bị cắt ngay cạnh fixture_trang; chữ này là Text fallback ngoài media test fixture. Source42dp hiện suy ra16sp, thiếu trong bảng native nên rơi về RN raw. Chọn cặp17/23sp hiện có của title cho đúng monogram suy ra16 này; vẫn giữ weight600 của label, không thay ảnh/hitbox hoặc bóp font.

**FORM:** Sửa một leaf trong Avatar, không sửa AvatarNguoi/cache/session/permission hoặc native module. Các cỡ hiện dùng khác và nhánh Image giữ nguyên. Chữ đầu tiếp tục nhận native converted metrics từ owner sẵn có, không observer/provider/key mới, không cap/unscale/adjustsFontSizeToFit. Fallback owner unavailable giữ RN scaling cùng host; chưa hứa không clip ở mọi cỡ/font khi thiếu metrics.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Quality bar cho bước này

AFTER đúng SHA: phone320/tablet768,100→200→100, sáng/tối; F trọn nét/đúng tâm, vòng42 và target48 không đổi, tên/date/routes vẫn đọc đúng. Kiểm chữ Việt có dấu, dấu hỏi và kích thước32/44/48/64 với ảnh thật/fallback, không tự lấy ảnh profile tổng hợp làm fallback proof. Kiểm absent/unavailable/partial metrics không mất nghĩa hay owner; host chỉ chứng minh props/identity, không native pixels. Không whole-avatar/whole-app/main approval trước ảnh mới.
