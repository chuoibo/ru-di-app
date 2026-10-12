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

**Quality bar lưu trong repo:** `docs/design/nep-ui-implementation/QUALITY-BAR-NATIVE-20261013.md`. Ảnh native AFTER và verdict e338 được đối chiếu bên dưới; không có approved comp mới.

## Đối chiếu built artifact e338 · 13/10/2026

Correction đã nằm trong `e338740b73e98eada580bffae3edb239acb9ab5c`: content/stage/writing/body grow ở mọi chiều rộng, shrink0/basis auto; bỏ bottomInset16, giữ footer/safe-area shell. `expanded` chỉ còn chính sách maxHeight input theo width. MediaPicker/MentionPicker/Field, `topicsTools` và mọi handler giữ trong cây cuộn; không có một toolbar thứ hai hoặc field dài ghim chân màn. Ảnh [phone tối100](../review/e338740b/composer-mobile.png) và [cửa sổ tablet tối100](../review/e338740b/composer-tablet.png) cho thấy rail sát vùng đăng, khoảng dư nằm trong canvas. Documenter đã mở cả hai, thêm ảnh chủ đề200/IME trong bó private; không đo pixel/hitbox hoặc suy mượt từ ảnh.

Bó native ngoài repo: `native-audit-e338740b` dưới `~/.local/share/rudi-nep-ui/runtime-private-merged-20261006/`. `source-screen-receipt.json` gắn fingerprint sourcee338/PID2884; `root-visual-review.json` ghi root mở26 PNG, pin52 PNG/XML cho cả Composer và Search. Đây không phải26 trạng thái Composer. `ready-e338` là loading trắng; `search-after-dark-100-sample-settled` thực tế là Profile, loại khỏi verdict Search. `sample-ime` là mẫu câu đã hiện nhưng bàn phím đóng, chỉ dùng cho sample-state.

AFTER Composer hiện phủ form rỗng, focus và chủ đề: light/dark100, phone dark200 với cuộn và IME body/topic, cùng cửa sổ768×1024 font100 sáng/tối trên một emulator. Chưa có native e338 cho bài đã nhập dài, media/pending/retry, validation/conflict, audience/edit/nhóm hoặc đăng/ACK; không dùng host tests thay các scene đó. Phone thường/landscape, tablet200/IME, TalkBack, contrast/target đo thật, reduced motion và phần cứng/iOS cũng chưa phủ đầy quality bar.

Root chạy clean exacte3389/9: Node subset246 file,1880/1880, fail/skip/cancel/todo0; Go chỉ gói hướng dẫn98/98. Năm file browser bị loại theo receipt. Không phải full gate/main/deploy hoặc nghiệm thu đẹp. Native555 mẫu, PID2884 không đổi, OOM0 ở các scope được theo dõi chỉ mô tả lượt kiểm; không chứng nhận hiệu năng hay mọi hành vi.

Verdict độc lập `impeccable-finish-review-e338-private-20261013-r1/scoped-verdict-five-sections.md` là **FIX**, sau khi reviewer mở23 ảnh (12 Composer,10 Search,1 source). F1 đã giải quyết trong dock thu gọn đã chụp; F2 đã giải quyết trong canvas rỗng đã chụp. F3 chỉ đạt một phần: cụm author/audience và ô chủ đề rỗng khi focus/IME được kiểm, còn typed body/topic, caret/selection với nội dung, Mention và helper/error/phục hồi chưa có native proof mới. Không suy những state đó hỏng, cũng không gọi F3 resolved.

FINISH và quality bar rộng còn mở. Đây là tài liệu lịch sử e338; sửa tab Khám phá sau e338 là slice khác, chưa được chứng minh bởi các ảnh này. Nếu Composer thay đổi tiếp, cần đối chiếu source và capture mới trước khi cập nhật kết luận; không tái dùng ship lịch sử.
