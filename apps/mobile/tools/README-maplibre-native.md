# Bounds marker trên Android

`sua-maplibre-marker.mjs` áp dụng khi `postinstall`, trước khi dựng APK.
Script chỉ nhận MapLibre React Native 11.3.10 và hash source đã kiểm;
chạy lại không sửa thêm, source hoặc version khác sẽ dừng để review.
`node tools/sua-maplibre-marker.mjs --kiem` chỉ kiểm, không ghi.

Trên Android, `MLRNMarkerViewContent` là ReactViewGroup được SDK tạo ngoài
UIManager. RN đã layout child 52×52 nhưng wrapper vẫn 0×0: tem nhìn thấy,
nhãn/action trợ năng bị loại khỏi cây native. Bản sửa measure/layout wrapper
theo kích thước child ngay trước đặt vị trí, chỉ khi kích thước đổi; không
layout lại child, không đổi anchor, clustering, màu hoặc hình tem.

Phải dựng và cài lại APK sau khi áp dụng, rồi chạy flow native
`.maestro/lifecycle/map-ready.yaml` với fixture thật. Metro reload không
thay được Kotlin trong APK. Kiểm cây Android có nhãn mốc và bounds thật,
thao tác mốc, ba lần cold launch, pan/khớp và ảnh đối chiếu mockup;
typecheck hoặc script exit0 không thay các bằng chứng này.
