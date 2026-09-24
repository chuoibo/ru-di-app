# Ảnh đại diện nhất quán giữa nhiều người dùng

- Ngày: 2026-09-24 · nhánh `claude/stoic-davinci-hiwiym` · nền `b8b66f7`
- protocol_version: không áp dụng (không đụng tiền, không đụng `docs/protocol/v1`)
- Verdict: không có reviewer; cổng bằng chứng tự chạy, số đo trong commit message

## Vấn đề đo được trước khi sửa (web, stack thật, băm SHA-256 ảnh đang hiển thị)

A đổi ảnh, B chung nhóm đang mở app:

| Lúc | B thấy |
|---|---|
| ngay sau, sau 30 s, rời màn quay lại, mở lại app | ảnh cũ |
| sau khi hết `Cache-Control: private, max-age=300` | ảnh mới |
| A tải ảnh lần đầu, B đang thấy chữ cái | chữ cái tới 10 phút (bộ nhớ 404 phía client) |

Nguyên nhân gốc: URL avatar `/people/{id}/avatar` không đổi khi ảnh đổi, và
chỉ máy người vừa tải biết ảnh đã đổi. Trên native còn nặng hơn (đọc mã
nguồn, chưa đo trên máy thật): expo-image cache đĩa theo khoá URL (Glide
`GlideUrl`, SDWebImage `cacheKey`), `cachePolicy` mặc định `disk`, nên ảnh
cũ có thể sống vô thời hạn.

## Thiết kế

1. **Phiên bản do máy chủ nói.** `GET /people/avatars?ids=...` (Go-only,
   `services/core/internal/avatarfeed`) trả `{id: avatar_id | null}` cho đúng
   những người route `GET /people/{id}/avatar` cho xem (`view_person_avatar`:
   chính mình hoặc cùng active trong một context); người không được xem thì
   vắng mặt. Thứ tự "ảnh mới nhất" chép đúng `GetLatestAvatar`.
2. **URL theo phiên bản.** Client vẽ `/people/{id}/avatar?v=<avatar_id>`: ảnh
   đổi thì URL đổi, nên cache web lẫn native tự đúng. `null` là chữ cái,
   không request nào (bỏ luôn 404 và bộ nhớ 10 phút).
3. **Đẩy qua WebSocket riêng** `GET /people/avatars/stream`. Trigger
   `avatar_feed_capture` trên `uploaded_images` (chỉ `purpose='avatar'`)
   `pg_notify` id người; listener đọc lại khán giả và phiên bản lúc giao.
   Khung `ready` sau mỗi lần nối và mỗi lần listener nối lại: client hỏi lại
   phiên bản cho mọi người đã vẽ. Đẩy là gợi ý, không phải giao hàng: cái gì
   lỡ thì lần đồng bộ lại sửa.

### Vì sao không nhét vào change feed của chat (lựa chọn ban đầu)

- Client chat hiện tại (`docTrangThayDoi`) từ chối cả trang nếu `type`
  không phải `message`/`vote` và đòi `sequence` liền mạch; thêm loại `avatar`
  làm mọi bản app đã cài rơi vào vòng "đang khôi phục". Lọc theo client cũ
  thì phá tính liền mạch.
- Socket chat chỉ mở khi đang ở màn chat; socket riêng mở suốt lúc app ở
  foreground, nên Thành viên, Bạn bè, Tường cũng nhận.

### Không làm / không đổi

- Không đổi route `GET/POST /people/{id}/avatar` (vẫn so parity với Python).
- Không đổi quyền: bạn bè không chung nhóm vẫn chỉ thấy chữ cái.
- Migration là version 1 riêng của `avatarfeed`, chạy chung lệnh
  `core migrate-chat`; `serve` từ chối khởi động nếu thiếu trigger, cùng
  thông báo như schema chat.

## Còn mở

- iOS/Android native chưa chạy: container không có KVM, và chính sách mạng
  chặn `dl.google.com`. Cần máy có KVM + `scripts/android_emulator.sh`.
- Người mới vào nhóm: người khác thấy ảnh của họ ở lần đồng bộ kế tiếp (mở
  màn, lên foreground, nối lại socket), chưa được đẩy tức thì.
- Bản web mất phiên khi tải lại trang (có từ trước, ngoài phạm vi).
