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

## Bổ sung (cùng ngày): vào/rời nhóm, và phiên web qua reload

### Vào/rời nhóm được đẩy ngay

Nguyên nhân gốc: trigger chỉ bắt *ảnh đổi*, không bắt *quyền xem đổi*. B đã
nhớ C là "không được xem" (vắng trong câu trả lời, ví dụ từ danh sách bạn
bè); C vào nhóm thì không ai bảo B hỏi lại. Chiều rời nhóm y như vậy.

Sửa: migration version 2 của `avatarfeed` (`membership.sql`): trigger trên
`memberships` phát `m:<context>:<person>` mỗi khi một hàng vào hoặc rời
trạng thái `active` (INSERT, UPDATE, DELETE; đổi context/person của một hàng
phát cho cả hai phía). Listener gửi `ready` cho mọi thành viên active của
context đó và cho chính người đó; client hỏi lại phiên bản và nhận đúng
quyền mới. Lời mời (`invited`) không phát gì. Version 1 giữ nguyên chữ nên
checksum cũ vẫn khớp.

### Phiên web sống qua lần tải lại trang

Nguyên nhân gốc: không phải lỗi vô tình. `src/phien.ts` cố ý giữ phiên chỉ
trong bộ nhớ trên web để không ghi bearer vào `localStorage`. Thiếu một nơi
cất an toàn thì mọi reload là đăng xuất.

Sửa (mẫu "cookie làm mới + token trong bộ nhớ" của SPA),
`services/core/internal/websession`, ba route Go-only:

- `POST /sessions/web` (Bearer) đặt cookie `rudi_web_session` =
  token phiên: `HttpOnly; Secure; SameSite=Strict; Path=/sessions/web`,
  hết hạn cùng phiên.
- `POST /sessions/web/resume` đọc CHỈ cookie, trả token/person/expiry/door/tên
  nếu phiên còn sống (chưa thu hồi, chưa hết hạn, người chưa xoá); phiên chết
  thì 401 và xoá cookie.
- `POST /sessions/web/clear` xoá cookie.
- CORS riêng có credentials; bắt buộc có `Origin` và phải là origin được
  phép (cùng host, danh sách `MOBILE_CORS_ALLOW_ORIGINS`, hoặc loopback khi
  danh sách trống); `*` không bao giờ được chấp nhận ở đây.

Client: `src/phien-web.ts` là kho phiên của trình duyệt (token trong bộ nhớ,
không đụng Web Storage); `khoiPhucPhien` đọc lại danh sách nhóm khi phiên
khôi phục không mang nhóm. Native vẫn dùng SecureStore, không đổi.

Giới hạn nói thẳng:

- Một XSS đang chạy trên trang vẫn gọi được `/resume` và lấy token, y như nó
  dùng được token trong bộ nhớ trước đây. Cái được so với `localStorage`:
  không script nào đọc được token lúc nó nằm yên (HttpOnly), cookie chỉ đi
  tới ba route này, không route nào khác đọc cookie nên không thêm quyền ngầm
  (CSRF) ở đâu cả, và thu hồi phiên là cookie chết theo. Cái KHÔNG được:
  cookie vẫn nằm trong kho cookie của profile trình duyệt trên đĩa, như mọi
  cookie đăng nhập; ai đọc được profile đó thì đọc được phiên.
- Cần web và API cùng *site* (ví dụ `app.x` và `api.x`) vì `SameSite=Strict`.
  Khác site thì cookie không đi, và kết quả an toàn: như cũ, reload là đăng
  xuất.
- Chọn nhóm đang xem không được nhớ qua reload; lấy nhóm mặc định như đăng
  nhập mới.

## Còn mở

- iOS/Android native chưa chạy: container không có KVM, và chính sách mạng
  chặn `dl.google.com`. Cần máy có KVM + `scripts/android_emulator.sh`.
- Danh sách thành viên trên màn đang mở không tự thêm người mới (danh sách là
  một lần đọc của màn); avatar thì đúng ngay khi người đó hiện ra.
