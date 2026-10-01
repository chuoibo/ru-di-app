# Đề xuất ADR: link chia sẻ của bài Cộng đồng trên app native (QA UI-136)

- Trạng thái: **đề xuất**, phần web đã sửa trong B1. Viết 01/10/2026.

## Đã sửa trong B1

- Web: «Chia sẻ» đi qua `chiaSe()` (Web Share, nếu không có thì chép link) và nói ngay trên nút kết quả («Đã chép link»).
  Link là địa chỉ https của chính trang đang mở (`location.origin` + `/community/posts/{id}`), mở được trong trình duyệt.

## Còn mở

Bản native vẫn chia sẻ `rudi://community/posts/{id}`. Ứng dụng nhắn tin thường không biến scheme riêng thành link bấm
được, và người nhận chưa cài app thì không mở được gì. Cần một host https công khai cho bài (universal link iOS / app link
Android, kèm trang web dự phòng), là việc hạ tầng: tên miền, `apple-app-site-association`, `assetlinks.json`, route web.

## Câu hỏi cho chủ sản phẩm

Bài công khai có được mở bởi người chưa đăng nhập trên web không (hiện route đòi phiên)? Câu trả lời quyết định trang dự
phòng là «bài + nút mở app» hay «cửa đăng nhập».
