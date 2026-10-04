# Ảnh trong `assets/rudi`

Chỉ còn `badges/` (hình huy hiệu của Thành tích) và `wordmark-splash.png` (chữ hiệu khi mở native, mục cuối).

Năm ảnh stock (`friends-rooftop.jpg`, `dalat-cafe.jpg`, `dalat-friends.jpg`,
`vietnam-road.jpg`, `dark-wood-grain.jpg`) từng là ảnh của câu chuyện trình diễn
Team Đà Lạt. Ngày 2026-10-03 app chạy production trên dữ liệu thật, câu chuyện đó
bị gỡ cùng các màn của nó, nên năm ảnh và mục ghim của chúng trong
`.repo-guard-allowlist.json` cũng bị gỡ. Ảnh địa điểm giờ chỉ đến từ kho ảnh của
máy chủ (`GET /places/{id}/photos/{photo_id}`), kèm ghi công của nguồn.

Luật quan hệ ảnh–địa điểm (ADR-0017 §2.4) vẫn áp dụng: một ảnh chỉ được gắn vào
địa điểm khi nói được *chính địa điểm*, *quanh đây* hay *minh hoạ*; khung nào vẽ
ảnh thì khung ấy in ghi công.

## Chữ hiệu khi mở native

`wordmark-splash.png` là bản raster 828×288 lấy nguyên bốn outline và
viewBox của `src/rudi/ui/Wordmark.tsx` (Baloo 2 ExtraBold, Ek Type, SIL OFL
1.1 đã ghi ở source), mực `coverInk` hiện hành, chuyển SVG sang PNG bằng
CairoSVG 2.8.2. Config plugin splash cần raster; không tạo logo/font mới.
Khi đổi chữ hiệu, tạo lại raster từ cùng source vector.
