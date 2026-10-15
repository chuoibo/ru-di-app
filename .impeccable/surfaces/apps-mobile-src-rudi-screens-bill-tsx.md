---
version: 1
slug: "apps-mobile-src-rudi-screens-bill-tsx"
primary_target: "apps/mobile/src/rudi/screens/Bill.tsx"
related_targets: ["apps/mobile/src/rudi/screens/chia-bill/ChiaBillLive.tsx","apps/mobile/src/rudi/screens/dot-thu/DotThuLive.tsx","apps/mobile/src/rudi/ui.tsx","apps/mobile/src/rudi/asset-pack","services/api/app/web"]
---

# Biên nhận và nền sản phẩm Nếp

Phạm vi extension của DESIGN/ADR-0037 đã chốt; mode Operate (tiền/host), Read (biên nhận khách). Code-led, không đổi world/font/theme. Tiền/source/consent có ưu tiên hơn mockup. 48dp, integer VND, text và control ngoài artwork.

## Direction contract

**THESIS:** Trang sổ trả lời ai chuyển cho ai và bao nhiêu ngay trước mọi sơ đồ; các công cụ yên tĩnh vẫn nhận diện bằng nét và vật liệu có công việc.

**OWN-WORLD:** Ground nguyên token, card mang chữ dark; giấy hoá đơn răng cưa co theo nội dung, mực teal chỉ tiền/control tiền, cloth ở bìa. Không grain phủ form/số; geometry một nguồn.

**STORY:** Nhập bill → chọn người → xem phần đúng → ghi sổ → đọc nghĩa vụ → phát biên nhận riêng. Tự khai khác người nhận xác nhận; không bank proof.

**FIRST VIEWPORT:** 390dp: TopBar, dòng tổng sổ và các nghĩa vụ avatar+tên+amount thẳng hàng; diagram gấp dưới list. Guest một cột tối đa480px: người ghi/phần ai, amount copy teal, trạng thái chữ và CTA; nghĩa vụ thứ hai là biên nhận thứ hai.

**FORM:** Extension pinned world; seed key kế thừa `c8e88116`, ADR-0037 + asset-v1. Signature: dấu ĐÃ GHI SỔ chỉ sau expenseVersion response, phong bì đóng sau invitation response; useMotion token/registry, reduced/background tới state cuối, không chuyển số.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Kiểm

Pilot settlement populated/dark, bill320/keyboard, batch và guest 1/2 obligations/copy/claim/dispute/CSRF/isolation; web light/dark/mobile, Android riêng, iOS blocked. Asset nguồn giữ byte/hash; provenance manifest+SHA ngoài ảnh để không re-encode handoff bất biến.
