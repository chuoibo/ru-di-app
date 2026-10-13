---
version: 1
slug: "apps-mobile-src-rudi-screens-profile-tsx"
primary_target: "apps/mobile/src/rudi/screens/Profile.tsx"
related_targets: []
---

# Cá nhân · hành trình và những cửa cần dùng

**Primary target:** `apps/mobile/src/rudi/screens/Profile.tsx`.
**Related targets:** `screens/profile/HoSoSong.tsx`, `screens/Create.tsx`, `screens/cai-dat/CaiDatScreen.tsx` trong `apps/mobile/src/rudi/`; các trang cài đặt con chỉ được xét khi nằm trong đường đi đang kiểm.
**Mode / flow:** Operate, có phần Read cho hành trình; code-led, `layout` → `polish`. Kế thừa `PRODUCT.md`, hệ giấy/mực/coral trong `DESIGN.md` và mã hiện hành. Đây là BODY draft để root ghi vào surface brief, chưa phải brief đã persist bằng launcher.

## Direction contract

THESIS: Cá nhân mở bằng con người và dấu chân đã có; công cụ theo sau bằng mục lục có nghĩa, không tranh nhau như một bảng tính năng.

OWN-WORLD: Giấy yên, mực rõ, Bricolage ở tiêu đề; nét hành trình nối năm sự thật. Coral đánh dấu hành động hoặc mục đang chọn, không rải khối màu.

STORY: Nhận ra mình → đọc hành trình → chọn trang đã giữ hoặc công cụ; Tạo chọn việc, Cài đặt đổi một lựa chọn rõ ràng.

FIRST VIEWPORT: Cá nhân: tên trang nhỏ, avatar/tên nổi trước, sửa hồ sơ yên, năm dấu chân rồi hai tab. Tạo: tiêu đề/ngữ cảnh, hàng hành động đầu. Cài đặt: top bar rồi nhóm Giao diện.

FORM: Kế thừa thế giới hiện hữu; mục lục phẳng và hàng ngữ nghĩa. Refinement layout→polish, không thay brand hay tổ chức concept-seed mới.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Cấu trúc và mật độ

Cụm nhận diện là điểm dẫn: tên là chính, thành phố/bio là phụ, «Chỉnh hồ sơ» là thao tác nhỏ có target đủ lớn. Năm số liệu là sự thật của máy chủ, đi cùng đủ danh từ: bạn bè, nhóm, kèo, điểm check-in, khoảnh khắc. Nét nối chỉ gợi hành trình; không gợi phần trăm hoàn thành hay thành tích chưa có.

Ở chữ thường và số ngắn, năm điểm tạo một dải có nhịp; chữ lớn, metrics thiếu hoặc số dài chuyển sang hàng đọc dọc. Số/danh từ không bị viết tắt, cắt dòng hay thu font để giữ hình. Các host, khóa và thứ tự đọc giữ ổn định khi 100→200→100, kể cả đổi rộng/hẹp.

Sau hai tab «Hồ sơ / Trang đã giữ», các cửa chia theo «Kết nối & gu / Sổ của bạn / Quản lý». Khoảng cách trong nhóm chặt, giữa nhóm rõ hơn; không đặt mỗi mục trong một card. Tài khoản, cài đặt, logout và hành động phá huỷ giữ đúng đường hiện có, không được nhân đôi để làm đầy bố cục.

Khay Tạo giữ sáu route và thứ tự theo ngữ cảnh hiện có: một tiêu đề, một câu ngữ cảnh, sáu hàng icon/chữ cùng trục. Nội dung phụ thuộc đúng hàng; không thêm tranh lớn. Chữ lớn được tăng chiều cao và cuộn để tới đủ sáu hàng; không buộc cả sáu nằm trong màn đầu.

Cài đặt dùng nhóm hàng và trạng thái lựa chọn thật. Giao diện trên máy phân biệt với quyền riêng tư gửi máy chủ; lỗi/saving nằm ngay lựa chọn tương ứng. Các trang bảo mật/phiên/thiết bị/xoá tài khoản giữ nghĩa và mức nhấn riêng, không kéo vào một khối nút ngang.

## Đường đi và trạng thái phải giữ

- Hồ sơ tải/rỗng/lỗi/null/zero dùng đúng dữ liệu và nhãn; zero là số thật, thiếu chưa biến thành zero. Năm field máy chủ không tráo nhau.
- Chỉnh hồ sơ: tên/bio/thành phố, avatar/crop, giữ nháp khi Back, Huỷ bỏ đúng nháp, lưu theo ACK; lỗi không xoá lời nhập. Khi IME mở, ô đang sửa và quyết định lưu còn nhìn thấy/tiếp cận được.
- Tạo: giữ route, context, điều kiện phiên, Back/scrim/drag và `onClosed`; đóng không vô tình kích hoạt route.
- Cài đặt: giữ khoá thao tác, retry ảnh, quyền riêng tư, phiên và E2EE; đổi cỡ chữ/theme không tạo thêm request hay thay người dùng.

## Tiêu chuẩn nghiệm thu và giới hạn hiện tại

Áp dụng `quality-bar.md` đi kèm: phone hẹp/tablet, sáng/tối, font200 và hot round-trip, IME, tên/bio/số dài, lỗi và disabled. Sửa cấu trúc trước, rồi chỉnh optical alignment, khoảng cách và type; không thêm art để che một cụm chưa có tổ chức.

Mốc tham chiếu mã là `75b7e52a486dc022d1977d075dd5ea37837ea262`. Mốc này có trước lần gọi pipeline hiện tại: hợp đồng này định hướng lần sửa tiếp theo, không hợp thức hoá ngược trình tự của mốc75. Bản hành trình hiện hành chưa được chủ sản phẩm nghiệm thu đẹp; hai gray-note cũ đã bị bác bỏ, không được phục hồi thành authority. Không có verdict `ship` mới trong draft. Root còn phải đưa ảnh AFTER đúng SHA cho reviewer mới và documenter theo FINISH.

**Quality bar lưu trong repo:** `docs/design/nep-ui-implementation/QUALITY-BAR-NATIVE-20261013.md`. Verdict và ảnh native AFTER đúng checkpoint còn chờ; không có approved comp mới.


## 13/10 · journal-first redesign

# Cá nhân — một người, một trang thật họ đã giữ

Base source `6c737f8df7f5861663ed8b34bf7bfe52744d6c9b`. Operate/Read, code-led trong thế giới hiện hữu; r3 là critique reference đã FIX direction, không phải approved comp. Không tạo brand seed mới, không ship raster từ board. Root ghi contract được chọn vào surface brief qua wrapper trước mọi W edit.

## Direction contract

THESIS: Identity và một trang thật đã giữ dẫn màn; con số và công cụ làm nền cho con người, không làm directory hoặc dashboard.

OWN-WORLD: Giấy/mực/coral, navy dark và native font/token hiện có. Một preview nhật ký bằng chữ/cover thật; không illustration, shading, count icon, metric card hoặc divider tạo cells.

STORY: Người → tabs → diary title, saved subtitle/excerpt → footer kind/privacy và Mở trang → quan hệ/gu thật → công cụ. Friends là một action48 duy nhất; bốn fact còn lại chỉ đọc theo server meanings. Mọi door hiện hữu còn tới được.

FIRST VIEWPORT: 390×844/100 ưu tiên identity/bio/year, tabs và title/nội dung nhật ký. Facts/tools được ở dưới fold. 320/200, unavailable và expanded dùng reading flow/wrap, không font clamp, fixed-height text hoặc remount. Bio/title nở tự nhiên.

FORM: Giữ Profile/HoSoSong/native metrics owner và editor draft/save/back/cancel. Một actor-scoped diary reader phục vụ preview/full shelf/pagination; chỉ focus, retry hoặc more mới đọc, không font/window/tab I/O. Loading/error/empty/no-cover riêng, không bịa text/ngày/chuyến đi hoàn tất. Existing world, không roll seed thay bản sắc.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Delta r3 đã chọn

Title diary đứng đầu entry, không static “Những ngày muốn giữ” hoặc book/privacy eyebrow. Metadata nằm sau saved body, gần Mở trang. Bỏ toàn stats table/cross rules/vertical cells; friends48 + groups text là nhóm mở, ba dấu chân là normal-type wrap không prose “Quanh bạn/Hành trình”. Không early gear, không duplicate Friends/Settings. Utility rows ở tầng thứ cấp được phép dùng chuẩn hiện tại.

## State và evidence contract

- Preview lấy phần tử đầu first page theo server created_at DESC/id DESC; đó là trang mới tạo/giữ, không ngày đi. Không suy từ memories count. Kind/audience dùng đúng enum, không “Bạn bè” cho Diary private/public.
- Cover dùng publishedPhoto/diaryImage auth hiện có; không cover/ảnh lỗi giữ title/saved text, không landscape thay thế. Excerpt chỉ là saved first-page text có route mở đầy đủ, không generated filler.
- Local collection cache có actor/owner/generation; stale response, retry trùng, pagination lỗi và unmount không được thay data của actor mới. Source image ổn định khi chỉ font/window đổi; không auth cache qua actor.
- HoSoSong read owner/form/input giữ identity qua metrics và profile/kept; account/saved panel giữ baseline hiện hữu. Không rewrite auth/store hoặc backend.
- New actual-source async regressions phải kiểm route+actor, one shared read across tabs/metrics, loading khác empty, lỗi/retry/pagination, stale actor/unmount. Host không chứng minh Yoga, native targets, IME hay beauty.
- Root native AFTER matrix: light/dark;320/390/768;100→200→100/unavailable; content/no-cover/image failure/loading/error/empty/long bio/title; lower doors/kept/account/saved/editIME. P1/P2 hiện partial, P3 unresolved; chưa có SHIP.

Questions skipped: người dùng đã cho phép tự redesign/audit; root là executor duy nhất, đã chọn corrected diary-first direction và sẽ review contract/patch trước W.
