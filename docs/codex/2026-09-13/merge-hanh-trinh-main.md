# Gộp hành trình bản đồ vào main

Người dùng yêu cầu merge trực tiếp vào main. Bản ghép gồm main
`8371ef5776b1100d9018ed1e3d5025aceb2b352b` và nhánh hành trình
`24bc70f8379182814df3c704489e7d85abcc611a`, được chuẩn bị trong worktree
`/home/lakiet/mobile-journey-merge`. Checkout `/home/lakiet/mobile` giữ nguyên
nhánh tính năng và các tệp ngoài phạm vi.

## Điều chỉnh khi ghép

- Thêm merge revision `d6a2f93b81e7` nối `27a9b83e10c4` (hành trình) và
  `c4f27a90d1e3` (sổ hai người). Giữ nguyên cha của các revision đã có.
- Bộ đọc migration tĩnh duyệt đủ hai nhánh theo thứ tự phụ thuộc; từ chối
  thiếu cha, chu trình, nhiều head hoặc nhiều root. Vẫn so schema với models.
- ADR hành trình chuyển sang **ADR-0028** vì main đã dùng ADR-0027 cho sổ
  hai người. Cập nhật tài liệu vận hành theo head hợp nhất.
- Web đóng gói CSS MapLibre từ dependency đã cài, bỏ tải CSS qua unpkg.
  Browser test chặn CDN cũ, kiểm CSS thực sự có hiệu lực và chờ hit-test đúng
  nút popup trước khi bấm bằng con trỏ thật. Giữ các assertion về điểm được
  chọn, bản nháp và chuyển sang timeline.

## Kiểm chứng bản ghép

- TypeScript typecheck và Expo web export đạt; export chứa CSS MapLibre.
- Toàn bộ mobile: **861 passed, 0 failed, 0 skipped** trên export mới.
  Trước sửa, suite từng đỏ ở thao tác popup dù ca riêng đạt. Dấu vết ghi
  nhận con trỏ rơi vào attribution thay vì nút popup. Chưa kết luận CSS
  hay một frame cụ thể là nguyên nhân duy nhất; bản cuối kiểm hit-test.
- Backend ghép: **235 passed, 90 subtests passed**, gồm API pair, journey,
  idempotency, schema/models, PostgreSQL itinerary/pair và Valhalla thật.
- Hai ca PostgreSQL bổ sung: **2 passed**. Từ từng head riêng → merged head
  → common base → head; kiểm đủ 13 bảng/4 hàm pair, 9 cột itinerary và
  revision duy nhất. Root tự chạy lại sau khi reviewer viết test.
- Ruff check/format của bốn tệp Python phục vụ merge đạt; diff check đạt.

Reviewer độc lập `merge_main_review` đã ghi **APPROVE** phạm vi engineering
merge sau sửa bộ đọc migration, và **APPROVE** delta CSS/browser test.
Reviewer tự chạy bộ đọc migration (10 tests/90 subtests) và browser test
riêng; đã kiểm full log 861/861. Test PostgreSQL mới do reviewer viết được
root review và chạy lại độc lập.

Bằng chứng native, visual và các giới hạn còn lại nằm trong
[báo cáo triển khai](../2026-09-12/hanh-trinh-trien-khai/bao-cao.md).
Việc merge không đồng nghĩa deploy API, chuẩn bị graph production hoặc
phê duyệt toàn bộ accessibility/iOS/hiệu năng trên thiết bị thật.
