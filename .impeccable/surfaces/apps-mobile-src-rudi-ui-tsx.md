---
version: 1
slug: "apps-mobile-src-rudi-ui-tsx"
primary_target: "apps/mobile/src/rudi/ui.tsx"
related_targets: []
---

# RudiScreen · vùng nhập dài ổn định trước bàn phím

Mode Operate/Read; refinement harden→polish theo impeccable-pipeline. Giữ thế giới hiện hữu, không seed mới, comp mới hoặc raster. Quy tắc này sửa riêng điều phối reveal trong screen; không thay các màn khác về màu, chữ hoặc mật độ.

## Direction contract

**THESIS:** Khi đang viết, trang phải đứng yên ở chỗ có thể đọc và sửa; ô quá cao không kéo qua lại giữa hai mép không thể cùng vừa.

**OWN-WORLD:** Giữ giấy/mực/coral, header/footer và thang chữ native hiện tại. Không thêm khung, lớp che, spacer, icon hoặc token.

**STORY:** Mở bàn phím → đưa phần cuối ô nhập vào vùng nhìn thấy → tiếp tục nhập hoặc tự cuộn. Kéo bằng tay tiếp tục hủy reveal như hiện tại; chuyển focus và keyboard vẫn có cùng vòng đời/generation guard.

**FIRST VIEWPORT:** Composer vẫn có tác giả/người đọc, canvas, công cụ và nút gửi như checkpoint0cdc. Khi ô nhập cộng clearance trên16 và dưới đã commit cao hơn viewport đo thật, sửa overflow dưới rồi giữ ổn định, không kéo đầu xung đột trở lại. Các ô đủ chỗ giữ thuật toán cũ.

**FORM:** Mở rộng hệ đã chốt; không đổi identity. Phân nhánh bằng inputHeight+16+focusBottom<=viewportHeight từ phép đo hiện hữu, không đo caret hoặc thêm observer/timer/key. Không ghi selection/focus, nội dung, nháp, owner/lease, topics/media/ACK. Không kéo xuống để ép canh đáy nếu ô đã nằm cao hơn đáy.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Bằng chứng và cổng còn mở

0cdc native phát hiện non-idle khi ô dài và IME mở; parent drag làm ổn định ngay. Sau manual drag,1776 ký tự giữ nguyên qua100/200/return100, không coi là nghiệm thu autofocus. Ca TR07 mới chạy trên source cũ đỏ6pass/1fail tại lệnh kéo ngược thứ hai; TR01–06 giữ byte nguyên. Native AFTER phải kiểm đường nhập bình thường không cần drag để cứu, hot scale, refocus, chủ đề/helper/error và công cụ trên phone/thường/cao/tablet. Test host không mô phỏng Yoga/caret, không thay review đẹp. F1–F3/whole-app/main vẫn mở.
