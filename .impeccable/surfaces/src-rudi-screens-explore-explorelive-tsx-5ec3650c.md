---
version: 1
slug: "src-rudi-screens-explore-explorelive-tsx-5ec3650c"
primary_target: "apps/mobile/src/rudi/screens/explore/ExploreLive.tsx"
related_targets: []
---

# Surface brief · Hàng tìm địa điểm và hỏi Rủ Đi AI

**Route:** `apps/mobile/src/rudi/screens/explore/ExploreLive.tsx`, dưới route Khám phá / Địa điểm đã có. `apps/mobile/src/rudi/ui.tsx` chỉ mở khả năng bố trí cho nút hiện hữu; mặc định của các nút khác giữ nguyên.

**Mode:** Operate; refinement `layout → polish`, code-led. Kế thừa brief `apps-mobile-app-tabs-explore-tsx.md`, giấy/mực/coral và sắc tím có nghĩa AI. Ảnh người dùng ngày 13/10 là bằng chứng lỗi, không phải comp đã được duyệt. Chỉ kiểm native Android theo yêu cầu người dùng; detector HTML/CSS không áp dụng. Không sinh raster cho lỗi hình học này.

## Direction contract

**THESIS:** Nhập điều muốn tìm là đường chính; hỏi Rủ Đi AI là hành động phụ ngay bên cạnh. Hai điều khiển chia sẻ mép trên, mép dưới và đường tâm, tạo một hàng thao tác có chủ đích thay vì hai khối lệch nhau.

**OWN-WORLD:** Giữ nền và viền của ô tìm, nút AI tím, bộ icon, chữ, bo góc và phản hồi nhấn đã có. Không dùng thêm thẻ, gradient hay artwork để che lỗi.

**STORY:** Người dùng chọn nơi, nhập ý muốn, rồi lọc danh mục hoặc hỏi AI bằng các handler hiện tại. Không đổi nhãn, mẫu câu, luồng gửi hay state.

**FIRST VIEWPORT:** Trên điện thoại 320dp và tablet, ô tìm chiếm phần rộng còn lại; nút AI rộng 48dp, cách ô 8dp. Chiều cao hàng do ô tìm và metrics native quyết định, nút theo đúng chiều cao đó.

**FORM:** Giữ một hàng và thứ tự đọc ô tìm → AI. Không đo chiều cao bằng state, không đóng băng font scale, không hạ chiều cao input. Kế thừa owner metrics hiện tại; ở chữ 200%, fallback và sau khi quay về 100%, hai mép vẫn đồng nhất. Placeholder được rút gọn bằng cơ chế hiện tại; tên truy cập vẫn đầy đủ.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Hợp đồng chất lượng và bằng chứng phải bổ sung

- Bố cục: cùng mép trên/dưới trong ảnh và bounds native sau khi bỏ nhấn; glyph AI nằm giữa nút, search glyph và chữ nằm giữa ô. Không lấy hai đáy trùng nhau làm đủ.
- Thích ứng: phone 320×640 font 1.0 và 2.0 rồi quay về 1.0; tablet theo lớp thiết bị app đang kiểm; theme sáng/tối. Không được đánh giá từ web.
- Đường thao tác: focus ô và IME, thay từ khoá, xoá từ khoá, hiện mẫu câu qua AI; không thực hiện truy vấn AI tốn tài nguyên hay ghi dữ liệu ngoài scope để chứng minh căn lề.
- Tính toàn vẹn: giữ nguyên ref, handlers, query/choGui/timKiem, tuyến điều hướng, cùng owner native và default `IconButton` của các màn khác.
- Finish: ảnh AFTER có source fingerprint được root mở và chuyển reviewer độc lập; kiểm chức năng và đánh giá đẹp ghi riêng. Tài liệu này chưa phải kết quả native hoặc phê duyệt.

**Quality bar lưu trong repo:** `docs/design/nep-ui-implementation/QUALITY-BAR-NATIVE-20261013.md`. Ảnh native AFTER và verdict e338 được đối chiếu bên dưới; không có approved comp mới.

## Đối chiếu built artifact e338 · 13/10/2026

Tại `e338740b73e98eada580bffae3edb239acb9ab5c`, `timRow` stretch hai control, gap8; caller AI dùng height auto/minHeight48/alignSelf stretch, giữ width48 của IconButton. `style` opt-in được đặt sau style kit; mặc định caller khác vẫn là ô48×48. SearchField và query/ref/bamHoi/hoi vẫn là đường cũ, không đo chiều cao bằng state hay tạo token mới.

Documenter mở [phone tối100](../review/e338740b/explore-mobile.png), [cửa sổ tablet tối100](../review/e338740b/explore-tablet.png) và ảnh phone dark200 trong bó private. Ảnh cho thấy control AI theo chiều cao hàng tìm; đây là quan sát frame đã chụp, không phép đo hitbox/contrast hoặc kết luận toàn Khám phá đẹp. Native gồm sáng/tối100 phone và cửa sổ768×1024, phone dark200/quay về100, query với IME và mẫu câu từ nút AI. Không thực hiện query AI thật hay business write trong phase này.

Canonical `source-screen-receipt.json` và `root-visual-review.json` ở bó `native-audit-e338740b` gắn sourcee338/PID2884,26 PNG root mở/52 pin cho cả Search và Composer. Loại `ready-e338` (loading trắng) và `sample-settled` (thực tế Profile) khỏi verdict bề mặt tương ứng. `sample-ime`/`sample` cùng mẫu câu với bàn phím đóng; không dùng làm IME proof. Bốn ảnh review trong repo là bản copy nguyên byte của bó này.

Verdict độc lập `impeccable-finish-review-e338-private-20261013-r1/scoped-verdict-five-sections.md` ghi S1 resolved trong scale/theme/window đã chụp: hai mép ngoài search/AI đồng nhất, glyph AI ở giữa; inset1px của EditText không phải lệch chiều cao. Reviewer mở23 ảnh trong cả bó Composer/Search/source, không mở26 ảnh của root. Disposition chung vẫn **FIX** vì F3 Composer partial; không suy S1 thành nghiệm thu cả Khám phá. Section tab cắt chữ200 là finding riêng tại e338, không phải hồi quy S1; bản sửa tab sau đó nằm ngoài đối chiếu này và chưa được các ảnh e338 chứng minh.

Quality bar rộng còn thiếu landscape/phone thường, tablet200/IME, query dài/lỗi/permission/network, TalkBack, contrast/target đo thật, reduced motion, phần cứng/iOS. Clean9/9 cùng Node1880 và Go hướng dẫn98 chỉ là cổng exact source, không verdict đẹp hoặc full gate/main/deploy. FINISH chưa đóng; đây là bản ghi lịch sử e338, cần source/capture mới nếu UI đổi tiếp.

## 20/10 · Khám phá nhanh và điểm đến bằng ảnh thật (PR #667)

Ghi sau khi đã sửa mã: hợp đồng mô tả hướng đã làm để reviewer chấm.

## Direction contract

THESIS: Khám phá hiện ngay: trang đầu 30 quán, cuộn thêm khi gần cuối, đổi tỉnh thấy ngay danh sách cũ trong lúc làm mới.

OWN-WORLD: Giữ thế giới hiện hữu; ảnh tỉnh là ảnh quán thật có ghi công qua MediaSlot; thiếu ảnh là tranh cảnh vẽ không nhãn chữ. Chữ «Minh hoạ» không bao giờ xuất hiện.

STORY: Chọn tỉnh (danh sách một cột: 33 tỉnh dò theo tên nhanh hơn lưới; thẻ đầu là tỉnh đang xem với ảnh thật, các hàng cùng một lối vẽ và cao bằng nhau) → danh sách hiện trong một nhịp → lý do gợi ý của AI tới sau, thay câu tại chỗ không xô bố cục.

FIRST VIEWPORT: Thanh tìm + chip loại + quán đầu; skeleton chỉ khi tỉnh chưa đọc bao giờ.

FORM: limit/offset, SWR theo người + điểm đến, reasons_for không chặn, nút «Xem thêm» dự phòng; tranh cảnh dự phòng chọn theo loại quán, không bao giờ cảnh mốc của vùng khác. Seed: none — mở rộng thế giới đã có (Sân Khấu Giấy v3, ADR-0037); concept-seed chỉ dành cho thế giới mới hoặc thay thế, nên không roll.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
