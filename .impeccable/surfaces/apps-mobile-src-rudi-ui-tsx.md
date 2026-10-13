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


## 13/10 · actual input viewport amendment

# Composer · vùng viết khi IME mở

Base6c737f8df7f5861663ed8b34bf7bfe52744d6c9b. Code-led Operate, `layout → polish`, giữ world và topology. Contract này viết trước private candidate; root chọn trước W.

THESIS: Người viết thấy đoạn đang sửa, kể cả đầu hoặc giữa bài dài; ô viết dùng vùng thực còn lại trên bàn phím.

OWN-WORLD: Giấy yên, mực rõ, coral cho đăng. Không tranh, token hay hộp phụ; sự gọn đến từ canvas và action dock cùng trục.

STORY: Đặt caret → sửa đoạn → đọc counter → thêm công cụ → đăng. Khi IME mở, cuộn trong ô viết giữ đoạn đang sửa; cuộn trang vẫn dẫn tới tools/topic.

FIRST VIEWPORT: Ô viết đủ nằm trong actual scroll viewport đã trừ header/footer; counter trọn và khoảng thở hiện hữu. IME đóng trả lại full canvas, không để một editor thấp vĩnh viễn.

FORM: RudiScreen công bố height từ onLayout viewport đang có qua scoped context, dùng observer keyboard hiện hữu. ComposerBodyInput chỉ cap khi IME mở, trừ top16 và actual body tail; complete native line thắng nếu vùng quá nhỏ. Không đoán keyboard height, polling, owner/key/input replacement.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Cổng riêng

Counter6c đã đủ glyph trong captures root nêu; chưa nhận toàn Composer. Agent mở composer-long-edit-start-light100.png: startZ/caret không thấy dù success receipt xác minh exact1777 tại offset0. Proposal chỉ bảo đảm input box có declared cap theo actual viewport; Android internal caret scroll phải được kiểm lại FIRST/MIDDLE/END với closeddriver, font100/200/return100 và IME open/close. Giữ R5 draft/lease/ACK/retry/media/topic clearance, toàn nhãn/scaling và reveal guard oversized. Unavailable font giữ nguyên RN scaling và minimum an toàn hiện hữu; nếu complete line lớn hơn viewport thì không ép nhỏ chữ và không nhận caret-fit PASS. Host rectangles không chứng minh pixels/caret.


## 13/10 · actual context compatibility

# Composer · compatibility sau full gate f3 RED

Basef3c001a87f45330c5eff6b682064187c74f4a422. Giữ direction IME viewport đã chọn, chỉ làm actual context và style forwarding minh bạch. Contract viết trước private representation change; root chọn trước W.

THESIS: Editor dùng đúng viewport từ owner thật và giữ toàn style của caller; host không bỏ qua context để đổi lấy test xanh.

OWN-WORLD: Giấy/mực/coral hiện hữu. Không đổi typography, khoảng cách, rail, footer hoặc artwork; đây là compatibility của cùng layout.

STORY: Actual screen đo viewport → context thật tới input → bounds an toàn → draft/attachment/retry giữ nguyên. Owner absence có defaultnull thật; owner có height phải được đọc.

FIRST VIEWPORT: Canvas khôngIME và editor IME giữ contract hiện tại. Không thay caret finding bằng một stub hook luônnull; native recapture vẫn phải làm.

FORM: Tách pure viewport context/hook thành leaf module thật; ui dùng/reexport cùng context. Legacy host delegate actualhook và có explicit actualprovider seam. Wrapper nhận props identifier, local rest bỏ bottomClearance, explicit props.style tới native input ở cả hai nhánh.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

Không sửa scanner để bỏ assertion. Regression bổ sung phải cho thấy height được provide thay đổi maxHeight qua actual hook, cùng native input/body/draft; null/default thuộc actual context absence. TR01–09/LP08 thật vẫn kiểm owner/reveal/line floor riêng. Native first/middle/end caret và beauty chưa được nhận PASS.
