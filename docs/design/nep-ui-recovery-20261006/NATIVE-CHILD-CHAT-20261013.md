# Native chat · khay con và bình chọn · 13/10

Frontend toàn app vẫn OPEN. Lát này chưa có verdict SHIP, chưa merge main. Kiểm chức năng xanh không thay gate thị giác hoặc native.

## Pha native đã nhìn

Source `7d31ef7109caf0d39953bdc025072995440dd44f`, emulator Android320×640/density160, font100/200 rồi hot-return100, light/dark. Root và người kiểm độc lập đã mở42 PNG gốc, kiểm168 tệp quartet;42/42 receipt khớp hash PNG/XML. Dữ liệu fixture tổng hợp, app PID3125 giữ nguyên. Build marker và actor được mở trên màn Tài khoản của tab Cá nhân; marker chỉ chứng minh ảnh đó, không độc lập chứng minh từng frame/APK/backend.

| Khay | Quan sát hữu hạn |
| --- | --- |
| Sticker | Hai cột139dp đều nhau, đủ tám caption qua scroll cuối, chữ200 wrap không cắt; hot-return100 khôi phục khung/cỡ chữ ở cả theme. Không press gửi sticker. |
| Màu chat | Năm target rộng51–52dp/cao52dp ở100 và58dp ở200. Aa/tick/selected ring hiện rõ; preview/caption reflow và return/reopen khớp100. Không đổi màu qua API. |
| Bình chọn |100/noIME hiện ba vùng nhập.200/IME làm body và focused choice2 chỉ còn17dp; hot-return100 còn36dp intersection, nhãn/đường gạch cắt. FAIL, cần sửa. |

Một số tên ảnh không đúng scene, giữ nguyên và phân loại trong inventory riêng: account-source-marker là hồ sơ; account-identity-source-visible/account-build-actor-actual là Cài đặt; account-build-identity-confirmation là Tài khoản & bảo mật; chat-settings-dark100-open cũng là Cài đặt. Ảnh dark chat settings đúng bắt đầu ở settings-dark100-five-swatches. poll-light200-body-lower chưa cuộn khác; choice-two-scroll mới có thay đổi. chat-dark100-final-empty chỉ composer trống, vẫn có tin fixture.

Dừng native có kế hoạch trước sửa code:2.948 mẫu; host available thấp nhất18,6616GiB, guest ready thấp nhất627,168MiB, app PSS cao nhất792,341MiB; một app PID,0 thay PID/0 safety-stop, sampled scope OOM/kill0. Crash buffer chỉ newline;16 exit record đều ngày11/10, không thuộc PID3125. Không gọi telemetry vắng là0 hoặc coi lịch sử exit là crash của pha này. Core/legacy và tài nguyên audit được giữ nguyên.

## Hướng sửa đang kiểm

ImageGen built-in tạo poll-ime-reference-v1 trước code; prompt nguyên văn2912byte, wrapper embed và decoded pixels khớp original. Fresh direction review giữ một editor/một action, helper/lỗi trong body; thêm D1 loại dải chỉ hé avatar/bubble ở noIME. Bỏ boxed fields, gradient, tỷ lệ màn cao và fit chữ lớn giả của mockup. Không cần companion raster trang trí.

Source sửa cục bộ GroupChatLive/SoHen và hook usePollFocusReveal: actual visible poll thu wrapper lịch sử nhưng giữ mounted native reader ở chiều cao dương đã đo; guard callbacks ẩn, phục hồi sau positive geometry. Editor/body grow khi IME mở, noIME giữ natural height. Helper/lỗi tổng quát cuộn cùng fields, footer chỉ send. Hook đo actual body/current focused input và group label, hủy async measurement cũ khi focus/viewport/offset/IME/metrics thay đổi; không observer mới/remount/writer mới. Draft/validation/submitted-snapshot giữ nguyên.

Review tĩnh đã tìm và sửa F1 collapse sai khi quyền editor mất và F2 grow cả noIME. Source-level resolution không thay execution. Pagination sau restore còn cần trace actual RN ordering; native anchor, field/selection/caret/glyph, sáu lựa chọn/errors/undo và aesthetics AFTER vẫn OPEN.

Kiểm nhẹ candidate sau hai amend: typecheck8,520s, peak sampled1.034,758MiB; test compile3,015s; focused3 files15/15,0fail/skip/cancel/todo. Fixup/focused chạy quá ngắn, scope events null/peak0 là unavailable, không phải OOM0. Chưa có clean9 của checkpoint mới, host controls mới hoặc native AFTER; các packet7d chuẩn bị trước đó chưa chạy và giữ nguyên.

Tiếp theo: clean tree đúng checkpoint SHA, bind full source graph/harness mới (thêm hook), kiểm controls có first failure hợp lệ, bật emulator với memory watch, chụp lại cùng cảnh poll100/200/IME/hot-return và normal/tall viewports. Sau cùng mới fresh finish review và documenter. Toàn bộ scene map/A–S, strict32, iOS, máy thật, TalkBack/motion/crypto vẫn là các cổng riêng chưa đóng.
