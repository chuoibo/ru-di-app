---
version: 1
slug: "apps-mobile-src-rudi-community-composer-tsx"
primary_target: "apps/mobile/src/rudi/community/Composer.tsx"
related_targets: []
---

# Composer · vùng viết và dock hành động

**Primary target:** `apps/mobile/src/rudi/community/Composer.tsx`.
**Related targets:** `community/MediaPicker.tsx`, `community/MentionPicker.tsx`, shell `RudiScreen` và bộ đo chữ native dùng sẵn; chưa đề xuất sửa shared source.
**Mode / flow:** Operate, Read cho giải thích phạm vi; code-led, `layout` → `polish`. Kế thừa giấy/mực/coral hiện hữu, không chọn brand/seed mới. Đây là BODY r2 prospective, thay định hướng Composer r1 sau phản hồi của người dùng; r1 giữ nguyên.

## Direction contract

THESIS: Vùng viết nhận khoảng thở; công cụ và đăng thành một dock liền mạch, không có hàng nút nổi giữa khoảng trống.

OWN-WORLD: Giấy yên, mực rõ, một trục nội dung. Coral chỉ dẫn chọn và hành động chính; đường nét phân vai, không thêm tranh hoặc nhiều hộp.

STORY: Biết ai đọc → viết → bổ sung nếu cần → đăng. Phạm vi và duyệt bài đi cùng tác giả; tệp lỗi ở gần cách phục hồi.

FIRST VIEWPORT: Top bar; avatar/tên/phạm vi và giải thích; canvas co giãn ở giữa. Khi thu gọn, rail ba công cụ sát quyết định đăng ở đáy, cùng cột; lý do/lỗi thuộc dock. Phần mở thêm cuộn trong thân.

FORM: Form một cột với dock hành động; bố cục native thích ứng trước, polish sau. Kế thừa world, code-led refinement; không concept-seed mới.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Quyết định về bố cục

Khoảng dư trên một form ngắn phải nằm trong canvas viết. Rail «Ảnh/video / Chủ đề / Gắn bạn» và nút đăng đọc như một nhóm thao tác ở đáy: không có dải trống vô nghĩa tách chúng, không dùng spacer cao cố định hoặc phép trừ chiều cao màn hình để giả tạo vị trí. Tên tác giả, audience và lời duyệt bài vẫn là một cụm ở đầu, không kéo lời duyệt xuống dock để lấp khoảng trống.

Dock không được trở thành một form dài bị ghim. Ảnh, tiến trình/retry/bỏ tệp, bộ chọn bạn và ô chủ đề/lỗi/helper của ô nằm trong vùng cuộn; nhãn công cụ giữ đủ chữ. Chữ lớn và IME được đổi cấu trúc/scroll theo khoảng thật. Nếu chiều cao chỉ đủ cho một phần, người dùng vẫn thấy field đang focus và tới được quyết định đăng cùng lý do; không thu font hay giấu recovery.

Counter thuộc cuối canvas. Tool rail, counter, ảnh và footer giữ cùng cột ở phone/tablet; safe area ở ngoài nhóm hành động, không chen giữa rail và Submit. Loading/disabled/error/selected có thứ bậc rõ trong sáng/tối. Không nhân đôi toolbar hoặc hành động đăng để đạt ảnh mong muốn.

## Hợp đồng chức năng

Giữ một ComposerForm, một MediaPicker và một MentionPicker ổn định qua font/layout/theme; không remount để chữa ảnh. Giữ body/topic refs, `topicsTools`, đo helper/error và callback-generation safeguards; trường đang focus không bị che bởi dock hay IME. Giữ native font owner, input/selection, RAM draft/lease/actor, attempts, payload, ACK/newer-ACK/retry, pending-file/send/pick/canEdit locks và mọi handler hiện có.

Giữ tất cả audience/nhóm/moderation/edit scope,5000 ký tự, tag/chủ đề và lý do disabled. Upload thất bại không làm mất tệp hoặc lời nhập; conflict giữ nháp cho đối chiếu. Đổi font/theme/layout không phát I/O. Câu giải thích có nghĩa không bị xoá để làm footer ngắn.

## Bằng chứng và hướng đã chốt

Người dùng đã bác Composer hiện tại từ `/tmp/codex-clipboard-Ziur7u.png`: rail nổi, khoảng lớn trước Submit, thiếu nhóm thao tác hợp lý. Agent đã mở độc lập ảnh này; source75 do root xác nhận, ảnh riêng không chứng minh fingerprint, scale hoặc hitbox. Phản hồi này thắng mọi kết quả source/host trước đó.

Ở mã75, `expanded` chỉ là `width >= 600`; ở phone, content không opt-in grow, stage dùng `flexGrow: 0`, còn shell giữ ScrollView flex1 và footer bên ngoài. Đây là lời giải thích từ cấu trúc, chưa là đo Yoga native. Sau reviewer độc lập, root chọn bước nhỏ: content/stage/writing nhận growth ở mọi chiều rộng, basis tự nhiên và không co nội dung; bỏ đệm cuộn cuối để dùng khe8 của shell trước Submit. MediaPicker/Mention/Field và `topicsTools` giữ nguyên cây cuộn, không reparent shell hoặc chuyển field vào footer.

Đây là candidate cần native xác nhận, không lời hứa flex tự bảo đảm geometry. Khi phần mở đang hiện, nó thuộc cụm công cụ cuộn và có thể đứng giữa rail với quyết định đăng; không giấu phần ấy để giả hai nút sát nhau. Ở200/IME, field và các công cụ phải tiếp cận được bằng cuộn, không ép vừa viewport. Nếu ảnh thật vẫn chia cắt quyết định hoặc IME che field/recovery, slice này chưa đạt và phải ghi finding, không ghim thêm nội dung dài để chữa vội.

Mốc base `75b7e52a486dc022d1977d075dd5ea37837ea262` có trước invocation; r2 áp dụng cho lần sửa tiếp theo. Dùng `quality-bar.md` đi kèm để review native AFTER đúng SHA, không dùng ship lịch sử hoặc host green thay phán đoán đẹp. FINISH còn mở.

**Quality bar lưu trong repo:** `docs/design/nep-ui-implementation/QUALITY-BAR-NATIVE-20261013.md`. Verdict và ảnh native AFTER đúng checkpoint còn chờ; không có approved comp mới.
