# Ảnh trong `assets/rudi`

Chỉ còn `badges/` (hình huy hiệu của Thành tích).

Năm ảnh stock (`friends-rooftop.jpg`, `dalat-cafe.jpg`, `dalat-friends.jpg`,
`vietnam-road.jpg`, `dark-wood-grain.jpg`) từng là ảnh của câu chuyện trình diễn
Team Đà Lạt. Ngày 2026-10-03 app chạy production trên dữ liệu thật, câu chuyện đó
bị gỡ cùng các màn của nó, nên năm ảnh và mục ghim của chúng trong
`.repo-guard-allowlist.json` cũng bị gỡ. Ảnh địa điểm giờ chỉ đến từ kho ảnh của
máy chủ (`GET /places/{id}/photos/{photo_id}`), kèm ghi công của nguồn.

Luật quan hệ ảnh–địa điểm (ADR-0017 §2.4) vẫn áp dụng: một ảnh chỉ được gắn vào
địa điểm khi nói được *chính địa điểm*, *quanh đây* hay *minh hoạ*; khung nào vẽ
ảnh thì khung ấy in ghi công.
