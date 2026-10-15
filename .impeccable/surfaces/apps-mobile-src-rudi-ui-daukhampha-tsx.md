---
version: 1
slug: "apps-mobile-src-rudi-ui-daukhampha-tsx"
primary_target: "apps/mobile/src/rudi/ui/DauKhamPha.tsx"
related_targets: []
---

# Đầu Khám phá · hai mục đọc trọn

**Primary target:** `apps/mobile/src/rudi/ui/DauKhamPha.tsx`.
**Mode / flow:** Operate, code-led `layout` → `polish`; refinement kế thừa giấy/mực/coral.
**Quality bar:** `docs/design/nep-ui-implementation/QUALITY-BAR-NATIVE-20261013.md`.

## Direction contract

THESIS: Hai mục Địa điểm và Cộng đồng phải đọc trọn cùng lúc. Đây là hai cửa của một màn, không phải một kho tab dài để giấu mục bên cạnh. Giữ thứ tự trái → phải và thay tràn ngang bằng chữ tự xuống dòng trong hàng có giới hạn.

OWN-WORLD: Giữ Bricolage `h2` qua native metrics hiện tại, mực đậm/nhạt và băng coral 28×4. Không thu nhỏ/cap font, thêm card, đổi màu hay kiểu navigation. Điều khiển phụ vẫn đứng ngoài tablist.

STORY: Người dùng thấy cả hai mục, chọn mục còn lại, rồi đọc hoặc tìm như hiện tại. Mục đang mở không tự điều hướng. Không cần đoán rằng phần chữ bị cắt là một tab phải kéo ngang.

FIRST VIEWPORT: Ở 320dp chữ 100%, giữ hai nhãn một dòng theo bề rộng nội dung và khoảng 24dp. Khi chữ lớn hơn chỗ còn lại, hai tab co theo hàng, nhãn wrap đủ từ và chiều cao tự tăng; không clip Cộng đồng. Nút overflow của Community giữ target và luồng cũ.

FORM: Bỏ cơ chế scroll của đúng hai mục; dùng hàng flex giới hạn, tab `flexShrink`/`minWidth: 0`, nhãn đầy đủ không numberOfLines. Không thêm observer/state đo chữ hoặc dùng `fontScale` cửa sổ để đoán native metrics. Font fallback và hot return đi qua cùng cấu trúc host; route callbacks và owner không đổi.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Kiểm chứng riêng cho amendment

- Ảnh native phone 320 chữ 100/200/100 và tablet, theme sáng/tối; cả hai nhãn phải hiện đầy đủ cùng lúc, không cắt ở viewport, không có target thấp hơn 48dp.
- Đối chiếu Explore và Community có overflow. Ảnh/font lớn phải kiểm cả glyph, dấu tiếng Việt và hai băng chỉ mục, không chỉ bounds.
- Giữ nhãn/action và selected states; đang ở Community bấm Địa điểm chỉ navigate một lần và ngược lại. Font-only event không gửi I/O hoặc imperatively scroll header, không thay observer/search ref/draft.
- Không thay bằng web hoặc dùng detector HTML/CSS. Cặp search/AI đã có bằng chứng AFTER riêng; amendment này chưa có AFTER, chưa đạt.
