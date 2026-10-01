# Surface brief · Cộng đồng những cuộc đi

<!-- impeccable:surface-brief 1 -->

**Route:** `apps/mobile/app/(tabs)/community.tsx` và `apps/mobile/app/community/`.
**Mode:** Operate / Read. Đọc, chia sẻ khoảnh khắc, gặp người cùng sở thích.
Đây là phần mở rộng bản sắc hiện hữu; giữ nguyên DESIGN.md và token dùng chung.
Hướng triển khai bằng code theo kế hoạch đã chốt, không có comp được duyệt.

## Direction contract

**THESIS:** Bảng tin tập trung vào câu chuyện chuyến đi, liên tục như cuộc trò
chuyện; ảnh và nhật ký là nội dung chính, không biến thành trang chỉ số.

**OWN-WORLD:** Nền giấy, mực đậm, tiêu đề Bricolage; cam cho hành động, tím
riêng cho Nếp. Giữ kích thước chạm và Sheet native của hệ hiện tại.

**STORY:** Người đọc chọn Dành cho bạn / Đang theo dõi / Thịnh hành, xem bài,
trò chuyện, rồi kể ngày của mình. Công khai nghĩa là gửi cộng đồng qua duyệt;
Bạn bè là những người đã kết bạn, không phải follower.

**FIRST VIEWPORT:** (đổi 02/10 theo mockup chủ sản phẩm, xem surface brief của
`apps/mobile/app/(tabs)/explore.tsx`) Cộng đồng là mục thứ hai của Khám phá: hàng
«Địa điểm | Cộng đồng» cố định trên đầu, nút cài đặt bảng tin ở bên phải; ô tìm chủ đề;
ba chip-tab Dành cho bạn / Đang theo dõi / Thịnh hành; consent ngắn ở lần đầu; luồng bài
với ảnh rộng hết cột. Viết bài qua con dấu «Tạo» (thẻ đầu khay). Trang chủ đề giữ tiêu
đề và nút viết riêng.

**FORM:** Mở rộng thế giới “Nhật ký chuyến đi sau giờ làm” hiện hữu; không có
seed chọn lại bản sắc. Feed một cột, khay bình luận, album vuốt/phóng ảnh.
Tương tác đặc trưng: thông báo có cập nhật giữ nguyên vị trí đang đọc; người
dùng chủ động mở phần mới. PressScale, chuyển ảnh và khay dùng useMotion,
tôn trọng giảm chuyển động. Không tự phát âm thanh video.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Phạm vi bằng chứng

Chỉ dùng dữ liệu tổng hợp. Không tạo raster mới; ảnh mẫu có sẵn của repo dùng
cho kiểm thử. Android emulator, web và iOS là các phạm vi kiểm chứng riêng;
không dùng ảnh web để kết luận về native hoặc độ mượt trên thiết bị thật.

## Đối chiếu hệ thiết kế sau triển khai · 27/09/2026

Lượt documenter đã đọc `PRODUCT.md`, các luật và token liên quan trong
`DESIGN.md`, sidecar `.impeccable/design.json`, `theme.ts`, `ui.tsx`,
`RudiTabBar.tsx`, `useMotion.ts` và các màn Community. Đây vẫn là phần mở
rộng hệ hiện hữu, không có quyết định đổi hệ thiết kế hay token dùng chung.

- **Màu và chất liệu:** `RudiScreen` dùng vân giấy có sẵn; màn đọc lấy
  `ground`, `ink`, `inkSoft`, `inkFaint` từ theme. Cam dẫn hành động và tab
  được chọn; nút gọi Nếp giữ tông `ai`. Không có raster được tạo mới để ship.
- **Chữ và cấu trúc:** tiêu đề dùng Bricolage `display` 34/39, `h1` 28/34,
  `h2` 21/27; nội dung dùng system 17sp, thân bài có giãn dòng 26sp.
  Các bài nằm trực tiếp trên trang, phân cách bằng đường mảnh; tái sử dụng
  `RudiButton`, `Field`, `Sheet`, `Avatar`, `PhotoViewer` và `PressScale`.
- **Ba điểm sửa đã đối chiếu:** mã đặt vùng chạm tối thiểu 48dp cho nút đầu
  màn, tab, hồ sơ, theo dõi, lựa chọn bài, chủ đề, hành động và trả lời;
  nút đếm có nhãn đầy đủ như “Thích bài, 3 lượt thích” và “Mở bình luận,
  2 bình luận”; cột bảng tin giới hạn 560dp, nằm giữa vùng đọc trên desktop.
  Số đo vùng chạm và nhãn đếm ở đây là bằng chứng từ mã, không phải kết quả
  chạy trình đọc màn hình của lượt documenter.
- **Chuyển động:** mã dùng `useMotion` cho nhịp xuất hiện cập nhật, chuyển
  ảnh và phản hồi nhấn của kit; video khởi tạo tắt tiếng và dừng khi rời
  trạng thái hoạt động. Ảnh tĩnh không chứng minh nhịp khung hình.

Đã mở và nhìn trực tiếp đủ năm ảnh:
[bảng tin Android](../review/community-phone-android.png),
[soạn bài Android](../review/community-composer-android.png),
[bình luận Android](../review/community-comment-android.png),
[web điện thoại](../review/community-mobile-web.png) và
[web desktop](../review/community-desktop-web.png). Ảnh sáng xác nhận chất
liệu giấy, phân cấp chữ, bố cục một cột và điều hướng thích ứng trong các
trạng thái đã chụp; dữ liệu mẫu có nhãn. Phạm vi này không bao gồm native
iOS, chữ lớn, chế độ tối hay đo hiệu năng trên thiết bị vật lý.

Không chuẩn hoá thành luật mới và không sửa ngoài phạm vi: thay đổi có sẵn
trong `DESIGN.md`, sidecar còn cấu trúc lịch sử chưa theo schema 2, cùng
độ mờ nút vô hiệu kế thừa từ kit. `DESIGN.md` và `.impeccable/design.json`
được giữ nguyên trong lượt documenter; các ghi nhận này không hợp thức hoá
sai lệch thành hệ dùng cho màn tiếp theo.

Finish reviewer đã chấm `ship` ở verdict pass: cả ba mục vùng chạm, tên truy cập
và độ dài dòng đã resolved. Kết luận này chỉ dành cho ba sửa đổi được yêu cầu
trên Android emulator và web đã chụp, không phải chứng nhận toàn nền tảng.

### Bổ sung bằng chứng video web

Documenter đã đọc lại `CommunityVideo` trong `PostCard.tsx`, runner
`apps/mobile/tests/community-video.e2e.mjs` và mở trực tiếp
[ảnh video web](../review/community-video-web.png). Do video HTML không gắn
header xác thực từ nguồn của `expo-video`, bản web tải byte qua `fetch` có
xác thực và `cache: "no-store"`, rồi phát từ URL blob tạm; mã thu hồi URL và
hủy yêu cầu khi rời trạng thái hoạt động. Native tiếp tục dùng header trên
nguồn video. Thay đổi này không thêm token hay quy tắc thị giác.

Kết quả chạy trình duyệt do lượt triển khai cung cấp: `PASS`,
`authorizedMedia=true`, `currentTime=0.600474`, `duration=2`, `blob=true`,
không có lỗi JavaScript. Runner kiểm byte trả về 200 có header xác thực,
video giải mã và thời gian phát tăng; documenter đối chiếu mã và ảnh,
không chạy lại runner. Video xanh dài hai giây được tạo làm dữ liệu QA
tổng hợp, không phải tài sản hình ảnh của sản phẩm.

Finish reviewer đã xem phần bổ sung và trả `ship` trong phạm vi sửa phát
video web này. Kết luận trước về ba sửa vùng chạm, nhãn truy cập và chiều
dài dòng giữ nguyên phạm vi Android emulator và web đã kiểm. Các verdict
không chứng nhận toàn bộ Community, native iOS hay hiệu năng chuyển động
trên thiết bị vật lý. `DESIGN.md` và `.impeccable/design.json` tiếp tục được
giữ nguyên.

### Bổ sung cầu nối chia sẻ bài cũ

Documenter đã đối chiếu `apps/mobile/app/posts/[id].tsx` và phần
`onShareCommunity`/`Sheet` trong `BaiChiTietScreen.tsx`, đồng thời mở
[ảnh xác nhận Android](../review/community-legacy-share-android.png) và
[ảnh xác nhận web](../review/community-legacy-share-web.png). Nút “Chia sẻ
lên cộng đồng” chỉ hiện cho tác giả của bài cũ công khai không có ảnh, khi
đường Community đã sẵn sàng. Khay xác nhận dùng lại `Sheet`, `RudiButton`,
thang chữ và màu hiện có; giải thích chỉ tác giả thấy bài lúc chờ duyệt,
sau khi duyệt mọi người mới xem và bình luận. Hai nút “Xác nhận gửi duyệt”
và “Để sau” nằm trọn khay ở cả hai ảnh.

Bằng chứng do lượt triển khai cung cấp: Maestro `_community-legacy.yaml`
đạt luồng xác nhận → gửi → màn quản lý bài → chờ duyệt; HTTP giữ cùng ID,
revision 1 và trả 404 cho người lạ. Bộ mobile đạt 1097/1097; typecheck và
các cổng contract, reachability, CORS liên quan đạt. Documenter không chạy
lại các cổng này.

Reviewer mới `legacy_share_finish` trả `SHIP` riêng cho cầu nối: câu chữ
và lựa chọn rõ, khay vừa màn, điều kiện tác giả/công khai/không ảnh giữ
đường đọc và phản ứng cũ; không có finding trong phạm vi đã xem. Reviewer
đã mở hai ảnh và đọc mã, không chạy lại kiểm thử hành vi độc lập. Tất cả
dữ liệu và ảnh là tổng hợp; không có bằng chứng native iOS hoặc nhịp khung
hình trên thiết bị vật lý. Không đổi `DESIGN.md`, sidecar hay hợp thức hoá
sai lệch có sẵn của màn đọc cũ thành luật thiết kế mới.

### Bổ sung sửa upload Android và bằng chứng media/realtime

Theo lượt triển khai, Blob cục bộ thiếu MIME khiến native bỏ `Content-Type`
và máy chủ trả 422. Bản sửa tạo body bằng `slice` có MIME đúng, rồi đóng
riêng body và Blob nguồn trong `finally` sau khi nhận response. Composer
đổi lời hứa AI kiểm tra thành nội dung cần được duyệt; lỗi media có câu
giải thích dễ hiểu. Reviewer `legacy_share_finish` xác nhận `SHIP` riêng
finding Blob này, không mở rộng verdict sang toàn bộ Community.

Bằng chứng do lượt triển khai cung cấp: 4 kiểm thử hồi quy đạt, typecheck
đạt, `npm test` đạt 1101 và không bỏ qua bài nào. Hai flow trong repo
`_community-media` và `_community-video`, với bản QA chỉ đổi `applicationId`,
đã chạy trọn và đạt trên Android emulator: chọn gallery → upload → tag
An QA → đăng cho Bạn bè. Video qua worker đến trạng thái ready rồi đăng;
bước phát thực từ 00:00 đến 00:02 được thao tác bằng ADB và đối chiếu ảnh,
không phải assertion phát video của Maestro. Hai flow Maestro gửi và nhận
trên hai emulator khác nhau cũng đạt cho realtime.

Documenter đã mở [ảnh bài có media và tag](../review/community-photo-tag-android.png),
[ảnh hai người bình luận](../review/community-two-people-android.png) và
ảnh kết thúc video tạm tại `/tmp/rudi-community-final-video-ended.png`,
thấy thanh phát ở 00:02/00:02. Ảnh được upload là screenshot QA cũ, nên
giao diện xuất hiện bên trong media là nội dung kiểm thử; đây không phải
lỗi lồng giao diện. Ảnh cuối chỉ chứng minh trạng thái đã chụp; diễn tiến
phát và realtime dựa trên lượt chạy nêu trên, không được suy riêng từ ảnh.

Tất cả media và tài khoản là tổng hợp. Lượt documenter này không chạy lại
test và không mở review mới. Chưa có bằng chứng native iOS, thiết bị vật lý
hay số đo frame timing; không diễn giải các lượt đạt thành chứng nhận toàn
nền tảng. `DESIGN.md`, sidecar và các sai lệch có sẵn tiếp tục được giữ
nguyên; bản sửa không tạo thêm tài sản thị giác hoặc luật thiết kế.


### Tích hợp main ngày 28/09/2026

Đã gộp cùng main `0b51d5be`; giữ theme/FlatList/View mới của main và
bridge chia sẻ lên cộng đồng. Browser trên source `0ec19fa9`: feed200,
legacy submit202 → pending, người lạ404, không lỗi JavaScript. Parent và
finish reviewer đã mở ảnh phone/desktop/sheet/pending; reviewer SHIP riêng
phạm vi bridge/UI sau merge, không chứng nhận native trên main mới.

Ảnh tổng hợp đưa kèm commit: [phone](../../docs/assets/community/feed-phone.png),
[desktop](../../docs/assets/community/feed-desktop.png),
[pending](../../docs/assets/community/legacy-pending.png).
Composer dùng helper coTuongNhom tối thiểu trong module chính sách
ban-tinh; giữ nguyên ba loại sổ của main. Sửa độ tương phản nút Để sau
trên nền giấy và dùng focus border theo theme cho hai ô nhập. Không sửa
DESIGN.md hoặc sidecar trong bước tích hợp.


Xác nhận cuối trên cây sạch `f496af16`: browser feed200, 0 lỗi JS;
parent và finish reviewer mở ảnh phone/desktop/composer-focus mới,
reviewer SHIP riêng sửa tích hợp. Nút Để sau đọc rõ trên paper, composer
có border focus theo theme. [Composer đang focus](../../docs/assets/community/composer-focus.png).
Mobile 1.239 PASS, 0 fail/skip; export web/iOS/Android PASS (bundle,
không phải native E2E). Không suy rộng kết luận sang frame timing hay
native sau merge. Các ảnh feed kèm commit được cập nhật từ lượt này;
ảnh legacy-pending vẫn là lượt `0ec19fa9`, bridge không đổi sau đó.
