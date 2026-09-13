# Review độc lập — Hành trình

Hai agent mới đọc code/ảnh, không viết thay đổi. Bản ghi này lưu kết luận
được gửi trong phiên; không mở rộng phạm vi verdict.

## final_visual_review

`disposition: ship`

Đầu vào: ảnh 08, 09, 10, 11, 13, 15, 16; ManHinhHanhTrinh và SoHanhTrinh;
DESIGN.md/PRODUCT.md; craft floor và Android reference của Impeccable.
Không có approved comp, QUALITY BAR card hoặc bản gốc toàn bộ câu trả lời
người dùng; reviewer dựa trên brief extension giấy–mực–coral hiện có.

Reviewer không thấy material finding chặn trong bảy trạng thái: typography,
giấy–mực, tối/sáng, hai cột và CTA phù hợp hệ hiện có. Tên rút gọn ở font 2.0
được đọc đầy đủ qua chọn mốc/accessibility label. Khoảng trắng ở wide là cơ
hội tăng độ hoàn thiện, không phải blocker trong capture.

Giữ: bản đồ làm trọng tâm, số thứ tự tương ứng điểm hẹn, đường coral và
Nếp tiết chế ở trạng thái trống. Không xác nhận iOS, thiết bị thật, motion,
toàn bộ tương tác hoặc toàn ứng dụng.

## final_engineering_review

`APPROVE — phạm vi engineering được giao`

Không phát hiện blocker trong `_number` chặn số cực lớn trước math.isfinite;
save khóa ghi, loại preview cũ, giữ idempotency attempt; hoàn tác dùng revision;
refresh lỗi mạng giữ editor; đổi chế độ không unmount; biến đổi giữ ID/ngày
khác và từ chối đề xuất thiếu/trùng chặng; timeline v2 dùng endpoint đúng.
Phần fit ngày trống đang chờ commit lúc review cũng được xem.

Reviewer tự chạy typecheck, 22 routing tests và 11 tests biến đổi/lịch trên
dist-test hiện có. Không dùng emulator, không sửa file. Không xác nhận toàn
bộ persistence, native E2E hoặc khả năng phát hành.

Sau review, thay đổi `edfb9fef` chỉ thêm testID của tiêu đề chi tiết và đợi
đúng chi tiết trước khi kiểm đổi chế độ trong Chrome; không thay đổi hành vi.

## Follow-up vòng đời popup

`APPROVE — riêng thay đổi vòng đời popup`

Reviewer kiểm tra việc bỏ `chooser.remove()` khỏi `draw()` của web. Popup
gắn vào map, không lệ thuộc marker được vẽ lại; vì vậy giữ DOM giúp thao tác
nhấn/thả đi tới cùng nút khi wheel zoom kết thúc. Đường đóng khi chọn mốc,
chọn nút, cleanup props/unmount vẫn tồn tại. Reviewer đọc mã MapLibre cài
tại máy: closeOnClick mặc định bật và popup tự cập nhật vị trí theo camera.
Reviewer không chạy suite lần này; parent chạy toàn bộ và đạt 837/837.

## Review artifact theo repo guard

`APPROVE` cho 12 PNG theo path và digest của manifest. Reviewer đã xem đủ
ảnh, đối chiếu 12/12 SHA-256 và đọc cấu trúc PNG: chỉ IHDR/IDAT/IEND/sBIT/sRGB,
không EXIF, text metadata hoặc byte nối sau PNG. Không thấy credentials
hay dữ liệu người tham gia thật; dữ liệu chuyến thử và địa danh công khai.

Reviewer bắt lỗi nhãn ảnh 16 khi recapture trước đó chụp nhầm ngày. Đã thay
bằng ảnh đúng Ngày 2 trống, có Nếp, không marker. Reviewer kiểm lại độc lập
và `APPROVE` digest `787b1e61075a08f6fcec286f075bf825ff98c44efbd63533551d2e218b6cd556`,
đóng finding. Approval 11 ảnh còn lại giữ nguyên. Allowlist chỉ miễn đúng
path/digest và rule controlled-artifact, không mở rộng ngoại lệ scanner.
