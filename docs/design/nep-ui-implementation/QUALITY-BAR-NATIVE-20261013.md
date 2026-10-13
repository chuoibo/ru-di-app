# Quality bar · Cá nhân, Tạo, Cài đặt và Composer

Đây là thẻ tiêu chuẩn cho hai BODY draft cùng thư mục, chưa là kết quả kiểm. Phạm vi Android native; root là người duy nhất vận hành emulator/runtime. Thế giới giấy/mực/coral đã được pin bởi sản phẩm; chất riêng phải đến từ nhóm nội dung, nhịp chữ, nét hành trình và phản hồi có nghĩa. Không đánh đồng «xài được» với «đẹp», không tạo artwork để thay việc sắp xếp.

## Những điều ảnh phải cho thấy

| Bề mặt | Dấu hiệu bố cục đạt | Dấu hiệu phải sửa |
|---|---|---|
| Cá nhân | Nhận diện dẫn; sửa hồ sơ là thao tác phụ; năm sự thật có số và đủ danh từ; hai tab rồi nhóm cửa theo nghĩa | Năm số như dashboard vô nghĩa; bio/menu/chỉnh hồ sơ cùng trọng lượng; chữ hoặc số bị ép/cắt; một card cho mỗi feature |
| Tạo | Tiêu đề/ngữ cảnh một cụm; hàng icon/chữ đều trục; mỗi hàng có vai rõ; cuộn tới đủ sáu việc | Tranh lớn lấn công cụ; card khác kích thước vô cớ; ngữ cảnh lặp; chữ lớn bị cắt hoặc route cuối không tới được |
| Cài đặt | Nhóm Giao diện, đăng nhập/phiên và riêng tư tách rõ; trạng thái/lỗi bám đúng lựa chọn | Chồng thẻ, switch/radio lệch chữ; trạng thái chỉ bằng màu; lỗi ở xa control gây lỗi |
| Composer | Tác giả/phạm vi thành một cụm; vùng viết là chính; tool rail liên tục; nút đăng và lý do gắn nhau | Audience rơi hàng không có chủ đích; công cụ rời rạc; khoảng trống chia form thành slide; footer/IME giấu lời nhập hoặc recovery |

Mỗi màn có một điểm dẫn, một nhóm hỗ trợ và những khoảng ngắt có lý do. Kiểm cùng trục trái, baseline icon/nhãn, optical back alignment, độ cao các hàng tương đương và nhịp chặt/rộng. Dùng type/token hiện có; Bricolage chỉ đúng vai, body/control dùng system. Không kicker, không font nhỏ hoặc clamp chuỗi để vừa hình. Chữ thường/placeholder đo tối thiểu4.5:1; chữ lớn và ranh giới control3:1 ở cả hai theme. Không suy contrast từ ảnh.

## Ma trận native tối thiểu cho vòng review

- Phone hẹp320dp và phone thường; tablet/expanded768dp; portrait và landscape theo size class hỗ trợ. Nếu đổi `wm size` trên cùng emulator, ghi đúng đó là mô phỏng cửa sổ, không tablet vật lý độc lập.
- Light/dark; font100 và200. Chạy hot100→200→100 khi màn còn mounted, thêm tablet→phone round-trip. Nhãn/action vẫn tới được, host/input/draft vẫn giữ; không thu system font hoặc remount để làm ảnh xanh.
- IME thật: tên/bio/thành phố, body và chủ đề đang focus; selection/caret và chữ đang sửa thấy được; hành động hoàn tất và lỗi không bị bàn phím che. Chụp được text có focus, không dùng tên file thay chứng cứ.
- Copy thường/dài, tên nhóm dài, bio nhiều dòng; số0/null/thiếu và số dài; form rỗng/đã nhập/có ảnh. Composer có tệp đang chuẩn bị, upload lỗi/offline/retry/bỏ, conflict, locked audience và disabled có lý do. Cài đặt có saving/error/retry; Tạo kiểm sáu route và dismiss.
- Mọi target Android tối thiểu48×48dp, tách đủ để không bấm nhầm; nét icon nhỏ không quyết định hitbox. Thứ tự đọc/label/state đúng, không đọc lặp số và danh từ; Back/scrim/drag/insets hoạt động. Kiểm target thực, không chỉ width nguồn.
- Chuyển động chỉ báo state/feedback, tôn trọng giảm chuyển động và không chặn thao tác. Ảnh tĩnh không chứng minh dropped frames hay latency.

## Cách đưa bằng chứng tới verdict

Trước lần chỉnh UI tiếp theo, root persist đúng BODY vào surface brief, giữ hợp đồng ngoài TSX/bundle/accessibility text. Root cung cấp ảnh native theo từng size class, source fingerprint/SHA, theme/font/IME và đường đi đã thao tác. Reviewer mới mở ảnh gốc, đối chiếu hợp đồng và craft-floor; review layout tách khỏi số test. Các đồ thị/host green chỉ giữ chức năng và cấu trúc mà chúng thật sự chạy.

Vòng mặc định: build đầy đủ → một batch ảnh/trạng thái → sửa một batch → tối đa một batch kiểm lại; finding ngoài phạm vi ghi nợ riêng. Người dùng đang theo dõi có quyền mở rộng, không dùng giới hạn để bỏ lỗi đã thấy. Native không chạy detector HTML/CSS hoặc gọi kết quả là detector-clean. Không cấp điểm critique, resolved hay ship khi chưa có snapshot/reviewer tương ứng.

Finish còn mở tới khi có review, verdict có phạm vi, và documenter xác nhận hệ hiện hành trong `DESIGN.md`/surface brief; extension không viết lại toàn bộ DESIGN. Không tạo raster trong task này. Raster ship thêm ở lượt sau phải có nguồn/quyền/provenance riêng; synthetic QC không trở thành art sản phẩm. Verdict ship cũ, export web và mô phỏng phone không thay nghiệm thu native mới; iOS/phần cứng chỉ được nêu khi thật sự kiểm.

**Reference craft floor:** `/home/lakiet/.claude/plugins/cache/impeccable/impeccable/4.5.2/skills/impeccable/reference/craft-floor.md`. Nguồn hợp đồng FINISH: `reference/new-work.md`, §5. Nguồn flow: skill `impeccable-pipeline`, `reference/layout.md`, `polish.md`, `operate.md`, `android.md`.


## Bổ sung sau phản hồi Composer75 · dock hành động

R2 phản ánh việc người dùng bác Composer75 ở ảnh `Ziur7u`; thẻ này định nghĩa kết quả phải đạt, chưa chứng nhận implementation. Giữ nguyên nháp Profile/quality r1; scope mới chỉ Composer.

## Cấu trúc phải đọc được ngay

- Cụm đầu: avatar/tên, audience và giải thích duyệt/phạm vi thành một đơn vị. Audience dài wrap có tổ chức; không đẩy giải thích tới chân đăng.
- Canvas: phần co giãn chính của form; khoảng trống giúp viết nằm trong vùng này. Counter hỗ trợ ở cuối canvas, không tạo một khoảng ngắt lớn trước công cụ.
- Dock thu gọn: rail «Ảnh/video / Chủ đề / Gắn bạn», lỗi/lý do và một quyết định đăng nằm liền nhau, cùng trục/cột. Không có toolbar nổi giữa canvas và nút Submit xa bên dưới. Safe-area/IME padding không trở thành khoảng trống nằm giữa các phần của dock.
- Phần mở: media/recovery, MentionPicker và Field chủ đề/helper/error cuộn trong thân; không ghim một form cao ở footer. Công cụ đủ nhãn ở200, không clamp/thu font hoặc giảm target để ép vừa.

Không đạt nếu chỉ đổi màu/bo góc, bớt vài pixel mà mối quan hệ rail→Submit vẫn đứt. Không dùng illustration, shadow hoặc border mới để giả một nhóm đã không cùng chỗ. Mục tiêu quen thuộc, chặt chẽ và có chất giấy/mực/coral; không biểu diễn sự khác lạ ở control chuẩn.

## Native và state

Kiểm phone320/phone thường, tablet768, hai hướng hỗ trợ, light/dark; hot100→200→100 và tablet→phone trong cùng phiên mounted. Đọc đủ nhãn/tên/phạm vi, không clip/overlap; target Android48×48dp được đo thật và tách đủ để bấm. Body/topic input, controller, draft, selection và actor giữ nguyên; đổi layout/font không phát I/O hoặc child subscription mới.

Chụp form rỗng/đã nhập/dài; tools đóng/mở; ảnh/tệp đang chuẩn bị/upload lỗi/retry/bỏ; validation, locked edit audience, conflict, disabled có lý do và submit thành công. Với IME thật ở body/chủ đề, field/caret/lời nhập và quyết định/recovery tiếp cận được; không dùng ảnh IME đóng để kết luận. Ở chiều cao hẹp, cho thân cuộn thay vì ép mọi thứ lên viewport đầu.

Type/token hiện hành, baseline/trục icon-nhãn nhất quán, nhóm chặt và khoảng ngắt có lý do; chữ thường/placeholder4.5:1, chữ lớn/control3:1 ở cả theme phải đo, không suy từ ảnh. Motion chỉ phản hồi state, tôn trọng reduced motion; ảnh tĩnh không đo performance.

## Bàn giao và giới hạn

Root đã chọn narrow growth tại caller sau reviewer độc lập: giữ nguyên cây MediaPicker/Mention/Field và đo topics, canvas nhận phần dư ở mọi chiều rộng; không reparent shell. Persist hợp đồng r2 trước live edit tiếp theo, đọc lại craft-floor ngay trước private patch. Khi phần thêm mở, nội dung liên quan được đứng giữa rail và Submit trong vùng cuộn; tiêu chuẩn là quan hệ rõ và tới được, không bắt cả nhóm lên một màn. Root vận hành runtime/test, cung cấp ảnh AFTER đúng sourceSHA/state/scale/IME. Nếu IME vẫn giấu field/recovery hoặc phần dư vẫn chia cắt hành động, candidate chưa đạt. Reviewer mới mở ảnh và đối chiếu contract/craft-floor; một batch kiểm, một batch sửa, tối đa batch kiểm lại mặc định. Native không chạy detector HTML/CSS hoặc gọi là detector-clean.

Source75 đã có tests xanh vẫn bị người dùng bác; không nâng source assertion thành nghiệm thu thẩm mỹ. Hoàn tất review/verdict/documenter có phạm vi, không viết lại toàn DESIGN cho extension, không tạo raster trong task này. FINISH chưa đóng.

Craft floor: `/home/lakiet/.claude/plugins/cache/impeccable/impeccable/4.5.2/skills/impeccable/reference/craft-floor.md`.

## Hai section Khám phá · yêu cầu bổ sung sau ảnh e338

Hai mục Địa điểm/Cộng đồng phải đọc đầy đủ cùng lúc ở100/200/return100, không clip mục bên cạnh hoặc thu font để giữ thanh thấp. Khi thiếu bề rộng, nhãn được wrap và hàng tăng chiều cao; cả hai target tối thiểu48dp. Giữ thứ tự, typeh2 native, băng coral và selected state. Nút overflow Community ở ngoài tablist, vẫn tới được. Kiểm cả Explore và Community, light/dark, phone320/window768; bounds chưa thay việc nhìn glyph/dấu và optical alignment. Giữ owner/input/ref/focus/query qua native metrics, không phát I/O khi chỉ đổi font. Năm feed khác vẫn có scroll riêng; chưa được suy đã đạt. Đây là tiêu chuẩn candidate mới, chưa native AFTER hoặc verdict.


## Composer r3 · sau native phone cao ee42

Phần dư phải nằm trong chính EditText có thể nhập, không chuyểncounter xuống để che đuôi không thể nhập. Compact ceiling tối thiểu420dp và tối thiểu một chiều cao cửa sổ, vẫn hữu hạn với bài dài; native complete-line minimum thắngceiling. Expanded giữ policy hiện tại. Kiểm phone320×640,390×844,320×960 rỗng và bài tổng hợp1776ký tự với IME100/200/return100, caret/reveal/cuộn; không suy geometry dài giữ nguyên từ testowner. Ba target tool tách8dp cả hàng100 vàstack200, min48dp, đủ nhãn/count/baseline. Caption/recovery/topic có thể cuộn nhưng field/action phải thực sự tớiđược; gestureQC phải bắt đầu trong boundsScrollView hiện tại, không ởfooter. Đây là tiêu chuẩn correction mới, chưa AFTER/verdict.


### Sau diagnostic 0cdc · bài dài và banner còn mở

Bài 1.776 ký tự phải nhập/focus bình thường không manual escape, giữ đủ nội dung và cho phép sửa đầu/giữa/cuối qua IME100→200→100, dismiss/reopen/refocus. Parent drag, topic helper/error và công cụ phải tiếp cận được mà không vòng cuộn hai mép. Kiểm phone hẹp/ordinary/tall, expanded, light/dark và target công cụ; không lấy raw non-idle hoặc manual rescue làm acceptance.

Banner cập nhật phải cùng reading flow, không che search, giữ refresh/offset/accessibility và native font reflow. Host/AST xanh chỉ chứng minh guards đã chạy. Giữ lịch sử và source fingerprint; crash buffer 0cdc có 4 Bluetooth events theo correction, memory chỉ có hiệu lực trong cửa sổ đã đo. Chờ candidate AFTER/reviewer; DESIGN cuối, FINISH/main/toàn A–S/iOS/phần cứng vẫn mở.
