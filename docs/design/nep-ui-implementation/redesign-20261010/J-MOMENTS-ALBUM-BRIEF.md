# J — Ảnh dẫn câu chuyện, thao tác ở đúng chỗ

Layout/harden, Operate + Read; kế thừa UI3/seed c8e88116. Mockup10 là
critique-reference, không phải approved comp mới. Không gen tranh bù bố cục.

THESIS: Chọn ảnh, viết một câu, biết ai xem rồi gửi; đọc lại ảnh trọn vẹn.

OWN-WORLD: Giấy/mực/coral/Bricolage, Instax/washi tiết chế; từng sự thật
viết một lần. Không đổi thế giới, token hoặc mascot Nếp.

STORY: Nhóm → ảnh/caption → tương tác; mục lục → album → viewer → đúng lớp cha.

FIRST VIEWPORT: Composer đưa picker/ảnh lên đầu thay hai tiêu đề và đoạn dẫn.
Shelf cho tên/ngày/count dễ quét; ảnh bìa hỗ trợ nhận ra cuộc đi.

FORM: Seed c8e88116. Một cột đọc/form, nhóm gần/xa theo nghĩa; 200% wrap,
IME giữ field và gửi. Footer gom audience/error/retry; viewer có nút48dp.

FINISH: Chụp Android sáng/tối/100–200%/IME/rộng, mở ảnh thật; review độc lập,
sửa finding rồi verdict/documenter. Unit xanh không thay bằng chứng đẹp.

Before814: root mở8contact/72viewport (69hash độc nhất), originals và mockup10.
A độc lập mở8contact/14original/mockup, disposition fix:4P1/6P2/1P3.
B độc lập TSX regex đúng1lần, raw[]/0; không computed geometry/contrast native.
A hoàn tất trước khi B đi vào tổng hợp. Hai ảnh tổng hợp portrait640×800 và
landscape960×640 đăng native thật sau TCP-reset/retry; mỗi lần đúng1bài.
Album API có2ảnh, outsider403. Chưa có native comment/check-in post, media-load
error, multi-day/multi-album, permission denial/camera/ratio biên/TalkBack.

Batch: (A01) actions/count reflow; (A02) comment/check-in keyboard-fit với
editor/footer riêng; (A03/A05) audience/error/retry cùng quyết định gửi;
(A04) bỏ heading/copy/filler trùng, ảnh thật dẫn trước; (A06/A07) caption
đọc đầy đủ và nút viewer tương đương gesture; (A08) nơi chọn wrap nguyên tên;
(A09) empty nhỏ/có cửa đi đúng tác vụ; (A10) shelf topology ổn định theo
font/container, không đổi vì có cover; (A11) bỏ author/time trùng trong ảnh.
B02 target cách nhau≥8; B03 media error/retry tại khung/viewer; B04 danh mục
tách loading/error/retry. Nút/gesture dùng chung zoom bounded; contain ảnh
để không silently crop, giữ clamp khung có chủ ý. Không bịa phần trăm upload.

Giữ API/writer, integer tiền, E2EE, privacy/context adapter, idempotency;
draft/file chỉ mất khi gửi thành công hoặc người dùng bỏ. Busy ref chặn
double tap cho tương tác mới. Caption/comment giữ riêng theo bài khi đóng
sheet; lỗi đọc bình luận không biến thành lỗi gửi. Không đổi source backend.

Sau batch cần clean exact SHA, native actual-data/IME/error/retry/double tap,
media byte failure/recovery, close/Back/swipe/zoom/buttons và review fresh.
Hai fixture được phép không chứng minh ratio9:16 hoặc permission/camera.
Màn768×1024 cùng AVD không phải tablet độc lập; toànJ/A–S/finalgate còn mở.
Giữ red outer143 và revealr0 (harness cuộn quá lỗi), không relabel thành pass.

## Batch sau inspection native vòng 1

Root đã mở ảnh chức năng upload/cancel/retry/check-in và ảnh lỗi tải media → tải lại có ảnh thật ở candidate2085f8b5. Sửa cùng batch GPL vì dùng chung Sheet: form ngắn cao theo nội dung, trần theo host thật; CTA chia sẻ ngắn, bỏ subtitle album lặp; bìa album chưa có ảnh dùng nét khung ảnh có contrast theo theme; chú thích viewer giữ cột đọc640 trên màn rộng. Không thêm ảnh trang trí thay cho ảnh nhóm.

## Batch theo fresh finish review c3

Reviewer mở đủ58 ảnh hợp lệ, gồm field/caption trên IME200, và trả `fix` cho
hai finding ở composer: helper chạm gesture indicator trên wide-dark200 và
footer lệch40dp so với cột ảnh/caption/privacy. Một batch sửa cùng GPL:
footer dùng inset đáy thiết bị cộng khoảng thở12dp, giữ nhánh IME của shell;
toàn form dùng chung cột đọc640 thay vì body640/footer560. Không thêm asset
trang trí hoặc đổi ảnh/privacy/draft/API. Cần recapture cùng viewport và
verdict hai finding; chưa nghiệm thu toànJ/A–S.

## Bàn giao verdict7cf — chỉ hai finding composer

Reviewer mở21/21 contact chứa58/58 required frame, kiểm hash/kích thước58
baseline và58 current, mở5 cặp original composer (10 original): **P1/P2
resolved / disposition ship**. P1: helper có khoảng thở trên gesture
indicator, field/audience/action/helper giữ trên IME200. P2: ảnh/caption,
privacy và footer cùng cột640, hết thụt40 mỗi bên. Root inspection mở lại4
original, không cộng chúng thành viewport mới. Không thấy regression trong
region đã cung cấp; không nghiệm thu toànJ/app.

Ảnh native giữ nhãn JS `7cf58966f0da6871f5e3a21534bd7d8c636a38af` trên5558:
29frame320×640 và29frame768×1024; wide là cùngAVD, không tablet độc lập.
Footer inset đáy thiết bị+12 khi IME đóng; nhánh IME của shell giữ nguyên.
Năm production targets J (ShareMomentLive/AlbumLive/PhotoViewer/Sheet/pure
photo-viewer helper) cùng test gốc byte-identical đến061d8d7a theo
`native-audit-061d8d7a/J-production-identity-proof.json`; không rerun/relabel
native hoặc matrix7cf thành061. PureJ prior identity5/0, canary và hai mutant
mỗi variant4/1/setup0 chỉ chứng minh pure-helper/static scope.

APK vẫn nền2f20, không phải APK rebuild7cf/061. Upload/retry/media/comment/
check-in2085 và full reviewc3 giữ nhãn gốc, không được coi là thử lại ở7cf.
Cổng exact0619/9 và Node subset1610/1610 là chứng cứ kỹ thuật riêng; detector
061 chỉ một HoSoSong.tsx regex[], không phải Jscan/geometry/contrast/beauty.
Bàn giao này không bổ sung TalkBack, motion, target thực, hardware/iOS,
fullJ/A–S, strict gate/E2E hoặc deployment. Giữ nguyên các red/hạn chế trước;
không thêm ảnh trang trí, comp được duyệt mới hoặc ngoại lệ privacy/draft.
