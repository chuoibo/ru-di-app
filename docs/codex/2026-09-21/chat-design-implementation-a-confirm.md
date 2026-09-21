# Xác nhận bản sửa chat: Reviewer A

Phương pháp: Assessment A độc lập của `/root/chat_visual_a`, một ma trận xác nhận sau một đợt sửa của tác giả. Chỉ đọc lại báo cáo A của mình, không đọc detector hoặc Assessment B. Không sửa product source, không commit.

**Điểm mới: 32/40, tăng từ 28/40.** Các sửa lỗi đã có bằng chứng thực tế; chưa đạt mục tiêu 40/40. **REQUEST_CHANGES đối với tuyên bố hoàn thành toàn bộ trải nghiệm “Sổ hẹn của hội”.** Không còn P0/P1 từ danh sách A được tái hiện trong ma trận xác nhận này. A1, A2 và A4 được đóng trong phạm vi web đã kiểm; A3 đóng phần nhãn sai, A5 cải thiện rõ nhưng câu chuyện chung vẫn còn thiếu.

## Bản chạy và bằng chứng

- Reviewer tự khởi động Chrome headless mới, tạo context/tab mới, đăng nhập bằng OTP qua UI với tài khoản tổng hợp index 18. Không dùng browser hoặc tab của tác giả. Browser riêng đã đóng sau khi thu chứng cứ.
- API tổng hợp Go/PostgreSQL `127.0.0.1:45801`, web Docker `127.0.0.1:8178`. Đọc script đang tải trong trang xác nhận chính xác bundle `entry-5c477e5372d98f5ce4adba7628905d24.js`.
- 430×932; sáng và tối bằng Cài đặt → Tối, có kiểm `aria-selected=true`. Không sửa DOM để dựng lại trạng thái sản phẩm và không giả response API.
- Nguồn ở HEAD `ef1ee46d9943b62d31d8083cbca850d6aa5f862d` cùng thay đổi chưa commit. `manifest.json` lưu SHA-256 của năm file liên quan, toàn bộ ảnh và kết quả thao tác; HEAD riêng không xác định hết bản chạy.
- Chứng cứ ngoài repo: `/tmp/rudi-chat-e2e.JpMlFu/design-a-confirm/`. Các file chính: `checks.json`, `manifest.json`, `01-chat-light.png` đến `19-dark-final.png`, nội dung màn `.txt`, script `review.mjs` và `launch.mjs`.
- Lượt này không chứng minh native Android/iOS, TalkBack/VoiceOver, crypto, E2EE hay tải production. Màn vẫn nói “Chưa mã hoá đầu cuối”. Không tự gửi invocation AI trong review; reviewer kiểm lời xin phép, nháp và mở thẻ AI đã được tạo thật trong phòng.

## Ma trận xác nhận

| Finding cũ | Thao tác độc lập | Kết quả | Bằng chứng |
|---|---|---|---|
| A1: mất nháp poll | Nhập câu hỏi và hai lựa chọn, đóng khay, rời phòng, mở lại nhóm và poll | **Đóng**: giữ đúng cả ba giá trị, hiện “Đã khôi phục bản nháp” | `04-poll-draft-before-leave.png`, `05-poll-restored.png`; `checks.json: pollAfterRouteReturn` |
| A1 mở rộng kiểm AI | Nhập lời nhờ, rời phòng rồi quay lại; mở Tự tạo kèo rồi quay lại | **Đạt**: lời nhờ còn nguyên ở cả hai lần | `06-ai-draft-before-leave.png`, `07-ai-restored.png`; `aiAfterRouteReturn`, `aiAfterManualReturn` |
| Nháp tách theo form | Bỏ riêng nháp AI, mở lại poll; gửi poll hợp lệ và mở lại form | **Đạt**: bỏ AI không xoá poll; poll gửi thành công mới về rỗng | `aiDiscardCleared`, `pollSurvivedAiDiscard`, `pollDraftClearedAfterSend` |
| A2: lỗi lẫn khay | Gửi poll rỗng, mở công cụ rồi Tờ hẹn, quay lại sửa đầy đủ | **Đóng**: không còn lỗi ở công cụ/AI; sửa hợp lệ làm lỗi hết | `02-empty-error-footer.png`, `03-tools-no-poll-error.png`; `pollErrorLeakedToTools=false`, `pollErrorLeakedToPlan=false`, `validInputClearsError=true` |
| A2: CTA/lỗi dưới scroll | Xem form poll sáng và tối tại 430×932 | **Đóng**: lỗi nằm y=756–774; nút Gửi nằm dưới lỗi, cả hai trong màn, không cần cuộn tới cuối form | `02-empty-error-footer.png`, `18-dark-poll-footer.png` |
| A3: nhãn tự viết sai đích | Nhấn “Tự tạo kèo” | **Đóng phần nhãn**: tên hành động đúng với `/outings/new` và CTA “Tạo kèo” | `07-ai-restored.png`, `08-manual-honest-route.png`; `manualRoute` |
| A4: Escape | Focus một tin thật, Enter mở menu; Escape đóng | **Đóng**: menu biến mất và focus về đúng tin gốc; có nút × nhìn được | `13-menu-keyboard.png`, `14-menu-escaped.png`; `menuEscape` |
| A5: poll sau chốt | Reviewer tạo poll mới, bỏ phiếu, xác nhận đóng, mở “Xem các phiếu” | **Đạt phần thu gọn**: trạng thái mặc định cao 194px, không dựng hàng radio; mở đầy đủ 328px với hai lựa chọn | `10-poll-close-confirm.png`, `11-closed-poll-collapsed.png`, `12-closed-poll-expanded.png` |
| A5: thẻ/pin đã thành kèo | Xem thẻ rút còn tiêu đề, hai chặng và CTA; bấm tờ ghim | **Đạt phần cấu trúc**: pin là trạng thái + hành động; mở đúng kèo thật có hai chặng | `01-chat-light.png`, `15-promoted-plan-open.png`, `16-dark-compact-chat.png` |
| Ghi chú nhãn trợ năng | Đọc tên điều khiển của pin đã thành kèo | **Đóng**: “Mở lịch trình đã tạo”, khớp trạng thái nhìn thấy | `checks.json: promotedPinRoute`, `SoHen.tsx` |

Nháp mới được lưu **trong bộ nhớ phiên**, theo tài khoản/nhóm. Bằng chứng trên chỉ đóng lỗi rời phòng rồi quay lại trong phiên đang chạy. Không suy diễn thành phục hồi sau tắt ứng dụng, reload hoặc đăng xuất; source `ban-nhap-cong-cu.ts` chủ ý không ghi nội dung plaintext ra đĩa.

## Điểm sau xác nhận

| # | Nguyên tắc | Trước | Sau | Lý do |
|---|---|---:|---:|---|
| 1 | Hiển thị trạng thái | 3 | 4 | Khôi phục nháp, lỗi form, phiếu đã đóng và kèo đã tạo đều rõ trong các trạng thái đã thao tác. |
| 2 | Khớp ngôn ngữ đời thực | 3 | 3 | Nhãn “Tự tạo kèo” đã trung thực; form tạo kèo vẫn dùng ngày năm-tháng-ngày và nhánh thủ công chưa là tờ nháp chung. |
| 3 | Quyền kiểm soát, đường thoát | 2 | 3 | Escape, focus return và giữ nháp đã hoạt động; Bỏ bản nháp xoá ngay, chưa có hoàn tác. |
| 4 | Nhất quán và chuẩn | 3 | 3 | Sáng/tối và trạng thái pin thống nhất hơn; trải nghiệm vẫn gồm khay form, thẻ và composer cùng tồn tại, chưa kiểm chuẩn native. |
| 5 | Phòng ngừa lỗi | 3 | 3 | Nháp được bảo vệ khỏi điều hướng, chốt poll có xác nhận, AI có consent; thao tác bỏ nháp chưa có cách cứu khi bấm nhầm. |
| 6 | Nhận biết thay vì ghi nhớ | 3 | 3 | Bốn công cụ dễ nhận biết, pin có hành động tiếp; chưa có quan hệ hiện rõ giữa kết quả poll và tờ hẹn cụ thể. |
| 7 | Linh hoạt và hiệu quả | 3 | 4 | Có khay và slash shortcut, pin trực tiếp, trạng thái thu gọn/mở rộng; CTA poll luôn nhìn được và menu thao tác bằng bàn phím. |
| 8 | Thẩm mỹ và tối giản | 3 | 3 | Thẻ AI đã thành kèo gọn hơn nhiều, poll đóng thu nhỏ; khay đang nhập vẫn chia màn với chat và composer, còn vùng trống quanh hành động phụ. |
| 9 | Nhận biết và hồi phục lỗi | 2 | 3 | Lỗi không lẫn công cụ, hết khi sửa hợp lệ, giữ việc đang viết; thông báo poll vẫn chung cho câu hỏi/hai lựa chọn, chưa gắn vào từng ô. |
| 10 | Trợ giúp đúng lúc | 3 | 3 | Consent và lời báo khôi phục đúng chỗ; chưa giúp người mới hiểu cách chuyển một bình chọn thành tờ hẹn chung. |
| | **Tổng** | **28/40** | **32/40** | **Good. Không tự nâng lên 40 vì test chức năng đã qua.** |

## Khoảng cách còn lại với mục tiêu

### C1 · P2 · Cùng chọn và cùng sửa chưa thành một đối tượng chung

Nhãn “Tự tạo kèo” đã sửa đúng lời hứa. Nó cũng xác nhận phạm vi thực tế: nhánh thủ công tạo kèo trực tiếp, chưa cho hội chia sẻ/sửa tờ hẹn nháp trước khi chốt. Poll hiện như một nội dung riêng trong chat; sau chốt, chưa có thao tác/ngữ cảnh dẫn lựa chọn ấy vào tờ hẹn cụ thể. Đây là phần còn mở của A3/A5, không được gọi là đã hoàn tất nhờ đổi nhãn và thu thẻ.

Tiêu chí đóng: từ một poll đã chọn, người dùng nhận biết được tờ hẹn nào đang tiếp nhận lựa chọn, mở sửa nháp, rồi thấy chính tờ đó chuyển thành kèo. Nếu chủ đích phát hành phạm vi nhỏ hơn, phải ghi rõ phạm vi ấy thay cho tuyên bố đã có toàn bộ chuỗi của brief.

### C2 · P2 · Bỏ nháp không có hoàn tác

Reviewer bấm “Bỏ bản nháp”; lời nhờ biến mất ngay, không có xác nhận hoặc nút hoàn tác. Đây là thao tác chủ động nên không phải lỗi mất nháp cũ; tuy nhiên lời nhờ dài tới 2.000 ký tự có thể mất vì một lần chạm nhầm. Chứng cứ: `aiDiscardCleared=true`, cùng hàm `discard` trong `SoHen.tsx`.

Tiêu chí đóng: sau bỏ nháp có một cách khôi phục ngắn, hoặc xác nhận khi có nội dung đáng kể. Không cần thêm hộp hỏi khi chỉ đóng khay, vì thao tác đó đã giữ nháp đúng.

### C3 · P3 · Form trong khay vẫn cần nhiều chuyển trọng tâm

Ảnh `05-poll-restored.png`, `17-dark-ai-form.png`, `18-dark-poll-footer.png`: nút Gửi đã dễ thấy, nhưng vùng trường nhập thấp khiến lựa chọn thứ hai bị cắt ở mép cuộn; composer chat vẫn hiện ngay dưới CTA form. Đây là đánh đổi bố cục còn tồn tại, không phải hồi quy CTA. Một trạng thái nhập tập trung hơn hoặc tạm thu composer có thể làm việc chọn/gửi rõ hơn, cần kiểm cùng bàn phím native trước khi thay đổi.

## Phán quyết theo phạm vi

- **Chấp nhận bằng chứng sửa A1/A2/A4 và phần nhãn A3.** Không tái hiện P1 cũ. Pin, thẻ AI sau chốt và poll đã đóng đã gọn, dễ lướt hơn.
- **Chưa chấp nhận mục tiêu hoàn tất 40/40.** Bản sắc giấy–mực/Nếp hiện rõ hơn nhưng chuỗi cùng chọn → cùng sửa → chốt vẫn chưa liền mạch ở nhánh thủ công. Đây là kết luận thiết kế có phạm vi, không phải chứng nhận phát hành hay E2EE.
- Không mở vòng đánh bóng tiếp trong lượt này. Những khoảng trống trên được bàn giao để người điều phối quyết định phạm vi, thay vì tăng điểm theo số lần sửa.

Điểm đã chốt trước khi nhận Assessment B. Câu hỏi định hướng từ báo cáo A trước vẫn áp dụng; reviewer không lặp lại yêu cầu hỏi người dùng trong subtask xác nhận mà root đã chỉ định rõ.
