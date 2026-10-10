# G / P / L — bố cục gọn, có tổ chức trên Android

Ngày 11/10/2026. Nền `2085f8b5`; refinement layout trong UI3 `c8e88116`, không thay nhận diện. Mode Operate / Read. Chủ sản phẩm yêu cầu ưu tiên chat, Cộng đồng và Cá nhân từ sáu ảnh native mới; chỉ test trong emulator.

## Direction contract — native

**THESIS:** Một việc chính mỗi vùng; thao tác quen tay, thông tin có thứ bậc.

**OWN-WORLD:** Giữ giấy, mực, coral và Bricolage của Nếp. Hộ chiếu là dấu nhận diện nhỏ, không chiếm màn đầu.

**STORY:** Nhận người và phạm vi → viết/đọc → hành động. Cá nhân tách hồ sơ khỏi trang đã giữ.

**FIRST VIEWPORT:** Chat có +, pill nhập ngắn, gửi cùng trục; khay thay vùng soạn. Bài có bốn vị trí thao tác ổn định. Menu là hàng icon/chữ căn trái.

**FORM:** Nhóm gần, nhóm khác cách rộng. Font200 đổi cấu trúc, giữ target48, cuộn nội dung; không thu chữ hệ thống. Wide giới hạn cột đọc.

FINISH: Capture → fresh finish-reviewer → disposition → documenter.

Contract native lưu trong brief vì React Native không có body/DOM; đối chiếu source, bundle provenance và ảnh emulator, không giả bằng chứng HTML.

## Bằng chứng trước và quyết định

- A độc lập mở 8 ảnh native100 tại SHA2085: bìa hộ chiếu dẫn màn hình, 8 mục menu ngang cấp; Tag bạn bè bẻ dòng; menu thao tác thiếu phân cấp; khay và composer cùng hiện. Long-placeholder DM và bookmark rơi hàng đến từ phản hồi chủ sản phẩm/source, chưa được chính baseline8 tái hiện.
- B độc lập: một scan layout / 9 TSX trả `[]`, regex-only; không đo native geometry, contrast hay độ đẹp. Bổ sung vùng mở bài ngắn cần48dp và câu trạng thái duyệt cần flex bound.
- Chat: + và gửi ngoài pill; placeholder không chứa tên người nhận đã có trên header. Mở tools dismiss IME, giữ draft. Tools/poll/shared sheet thay vùng soạn. Plan vẫn có đường về ô nhập theo ADR-0046; giữ E2EE, phản hồi gửi, retry, thiết bị và lời báo riêng tư ảnh.
- Cộng đồng: identity và người đọc cùng nhóm compact; toolbar ba công cụ cân bằng, font200 thành hàng. Audience edit vẫn khóa, group thật, moderation, draft và logical_id không đổi. Post actions bốn slot icon, count riêng khi có; label đầy đủ trong accessibility. Menu không nhuộm mọi thao tác thành nút primary. Personalization dùng switch; xóa lịch sử vẫn bước xác nhận với phạm vi rõ.
- Cá nhân: hộ chiếu thumbnail cạnh dữ liệu thật; tab Hồ sơ / Trang đã giữ. Ba nhóm Sổ của bạn / Kết nối & gu / Quản lý. Hành trình một cửa trên home, teaser nằm ở trang đã giữ. Tài khoản/Đăng xuất vẫn reachable tại Cá nhân. Không số đếm hay tiến độ bịa.

## Kiểm cần có trước khi đóng scope

Native light100, dark200, wide trên cùng emulator5558 (wide không phải thiết bị tablet độc lập); chat idle/IME/multiline/tools/plan/back/draft, composer audience/tools/IME, feed action rail, hai sheet, profile hai tab và edit/cancel. Mở ảnh thật để đánh giá. Không thao tác emulator5562 đang được chủ sản phẩm dùng.

Clean SHA typecheck, guide, export, Node suite; mutation và finish review riêng. Regex, unit hoặc export không thay visual/E2EE/device proof. J2085 vẫn candidate mở: evidence và việc còn lại tại `native-audit-2085f8b5/j-pivot-checkpoint-20261011.json`, không được ghi ship khi đổi ưu tiên.

## Một batch sửa sau inspection native vòng 1

Root đã mở ảnh light100/dark200, hẹp320 và rộng768 của candidate70938418. Sửa nhãn tab Cá nhân có chiều rộng nội tại sai, nút sửa thành thao tác gọn theo nội dung; bỏ subtitle lặp của DM, giữ tên và trạng thái mã hoá. Khay tools/poll thay composer phải tự giữ inset đáy, không để gesture handle nằm trên nút gửi bình chọn. Mở lựa chọn người đọc/công cụ bài viết dismiss IME trước khi đổi vùng thao tác.

Sheet dùng chiều cao host thật làm trần ở mọi chế độ; form ưu tiên footer chỉ cao bằng nội dung khi đủ chỗ, reference cuộn trước footer khi thiếu chỗ. Phần này dùng chung với J và cần kiểm lại check-in200/IME. Ảnh chat-idle wide-light100 bắt giữa chuyển route; ba ảnh composer dark200 hẹp còn IME từ run trước không dùng làm bằng chứng idle. Vòng 2 phải capture lại trạng thái ổn định, không sửa sản phẩm dựa trên lỗi driver.

## Batch theo fresh finish review c3

Reviewer mở đủ 105 ảnh hợp lệ và trả `fix`, gồm bốn finding: cột chat rộng,
readiness/ownership tải bảng tin, nhãn sửa hồ sơ xuống dòng ở100%, và toolbar
viết bài wide bị tách bởi hai khoảng trống. Một batch sửa: chat giới hạn640
trên nền phủ toàn màn; vùng viết wide dùng chiều cao còn lại, counter và
toolbar nối ở cuối; nút sửa có chiều rộng tối thiểu đủ cho nhãn thường.
Bảng tin chỉ retry trang đầu chưa nhận khi focus lại; trang rỗng đã nhận
vẫn là ready, chỉ request hiện hành hạ loading/fetching. Refresh chỉ cuộn
đầu khi response được áp và màn còn focus, giữ cards/expanded/offset khi
request cũ bị vô hiệu. Diagnostic actual TSX chứng minh race trong controlled
focus host, chưa quy nguyên nhân duy nhất cho ảnh native lỗi trước đó.

Batch này chưa được nhận verdict. Cần chụp lại cùng viewport và chấm từng
finding resolved/partial/unresolved; không chuyển `fix` thành ship từ source
hoặc test xanh.

## Verdict 7cf và sửa riêng nhãn hồ sơ

Verdict đã mở35 contact sheets, đối chiếu105 originals: F1/F4 resolved;
F2 resolved trong diagnostic actual TSX9/0, chưa là chứng minh nhân quả native.
F3 còn partial vì nhãn «Chỉnh hồ sơ» vẫn xuống dòng ở100% cả hẹp/rộng.
Không có regression mới trong vùng đã chụp; disposition vẫn `fix`.

Sửa caller hồ sơ: cho label nhận phần chiều rộng trong button, outer theo
nội dung ở100% và theo cột khi chữ lớn. Bỏ minWidth168 không giải quyết
phân bổ label; giữ target48, không cắt chữ hay thu font. Chưa nhận verdict
cho sửa này. Cần chụp «Chỉnh hồ sơ» / «Sửa tiếp hồ sơ» / Huỷ trở lại ở
light100, dark200, hẹp/rộng; chỉ mở→Back→mở tiếp→Huỷ, không nhập/lưu hồ sơ.

Verdict8ec mở đủ12 originals: hai nhãn hết wrap nhưng nút vẫn đầy cột,
icon và nhãn cách xa; F3 tiếp tục partial. Không coi width:auto trong source
là intrinsic width đã nhìn thấy. Thay riêng thao tác này bằng PressScale,
RudiIcon và typography.label hiện hành: nhóm icon/chữ không grow, outer
ôm nội dung với padding14×10, gap9, target48 và maxWidth100%; chữ lớn được
shrink để reflow trong cột. Giữ spring0.98, role button, màu/radius hiện hành,
handler và draft semantics; không đổi RudiButton dùng chung. Cần chấm lại
cùng12 vùng native sau commit mới; chưa đóng F3 từ source.

Probe native1e56 đo cùng font/nhãn: ở100%, «Chỉnh hồ sơ» cần khoảng75.219dp
nhưng Text chỉ nhận75; «Sửa tiếp hồ sơ»90.699 nhận90. Ở200%, nhãn
sửa tiếp167.746 nhận167. Đây là thiếu phần lẻ trong allocation, không
thiếu cột. Sửa bằng floor trên chính Text: native đo nhãn trong cột hữu hạn,
chỉ nhận intrinsic khi đúng một dòng, ceil theo pixel thiết bị rồi cap phần
còn lại sau icon/padding/border/gap. Key gồm nhãn/fontScale/density/cột và
bỏ event cũ. Measurer tuyệt đối, trong suốt và ngoài accessibility; không
chiếm layout. Gỡ toàn bộ log/probe DEV; giữ accessibilityLabel tường minh.
048 compatibility identity7/2 (L02/L03) là thiếu label ở frozen host, không
mutant kill/visual credit. Source cuối vẫn cần12 capture và clean gate mới.

Native814 đủ12 ảnh: nhãn đã một dòng nhưng control100% còn cao60dp,
chữ ở trên tâm icon; nhãn sửa tiếp200% còn control86dp. F3 vẫn partial.
Clean8149/9, subset1610/1610 và frozen behavior9/0 không chứng minh
geometry; detector chưa chạy. Giữ đo/cap/pixel-ceil hiện hành, chuyển
minWidth của Text thành width tường minh để Android đo cả chiều cao theo
chiều rộng đó. Đây là giả thuyết sửa dựa trên ảnh và width constraints của
RN, chưa kết luận cached height là nguyên nhân. Cần12 ảnh native mới trước
khi nhận verdict; không đổi handler, draft, font hay shared primitive.
