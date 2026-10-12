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

**Quality bar lưu trong repo:** `docs/design/nep-ui-implementation/QUALITY-BAR-NATIVE-20261013.md`. Verdict và ảnh native AFTER đúng checkpoint còn chờ; không có approved comp mới.
