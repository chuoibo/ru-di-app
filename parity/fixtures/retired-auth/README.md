# Oracle đăng nhập đã nghỉ

Các scenario này là bằng chứng lịch sử của danh tính số điện thoại, Google cũ
và lời mời đổi phiên. ADR-0055 gỡ các cửa ấy; không còn nạp chúng từ
`parity/scenarios/`. Giữ nguyên yêu cầu cũ để đối chiếu, không nhận chúng làm
nghiệm thu hiện tại. Tài khoản mới là Go-only, đo qua PostgreSQL/Redis thật,
client HTTP, Android và web; không thêm writer Python để làm parity xanh.

Chỉ file nào mà gần như mọi bước gọi cửa đã nghỉ mới nằm ở đây:
`/auth/otp/*`, `/auth/google`, `/identity/person-id`, `POST /sessions` (đổi
lời mời lấy phiên) và `/friends/lookup` (nay là route tài khoản tự quản, chỉ
Go phục vụ nên Python không còn là oracle). Bỏ các bước đó đi thì file không
còn gì để so.

## Phiên đọc/xoá vẫn nằm trong suite đang chạy

Ba file từng bị dời cả khối sang đây đã về chỗ cũ trong `parity/scenarios/`,
chỉ bỏ đúng các bước gọi cửa đã nghỉ:

| File | Bỏ | Còn chạy |
|---|---|---|
| `w2/friends/prod-auth.yaml` (`prod`) | 2 bước `POST /friends/lookup` | 26 bước: lời mời kết bạn, phản hồi, danh sách, `DELETE /people/me`, `DELETE /sessions/current` |
| `w9/sessions/GET-sessions.yaml` (`dev`) | 2 bước `POST /sessions` | 21 bước, thêm 1 bước `current` tính từ bearer của chính người gọi |
| `w9/sessions/DELETE-sessions-session_id.yaml` (`dev`) | 1 bước `POST /sessions` | 20 bước |

Phiên không còn lấy qua HTTP được trên cả hai stack, nên harness ghi thẳng
vào database của từng stack trước bước đầu tiên, cùng một cách cho cả hai
(`seedSessions` trong `parity/internal/runner/runner.go`, `issued_via =
'genesis'`). Persona `prod` vẫn có một phiên như trước. Persona `dev` chỉ có
phiên khi khai `sessions: N` (tối đa 4); bearer của phiên thứ n là
`{{token.<persona>}}` (n = 1) hoặc `{{token.<persona>.<n>}}`, phiên sau mới
hơn phiên trước một giây. Bước `dev` vẫn gửi `X-Actor-ID`, không tự gửi bearer.

Khác với bản cũ, các hàng phiên mang `issued_via` `genesis` thay vì `invite`
và không trỏ tới lời mời nào: không còn cửa nào sinh ra hàng `invite`, nên
không có gì để so ở chỗ đó. Bước tạo lời mời đích danh vẫn giữ, và giờ cho
thấy tạo lời mời không đăng nhập ai.

`w0-transparency/actor-headers-and-guest-privacy.yaml` ở đây là bản trước khi
bỏ bước `lookup_not_json`; bản đang chạy giữ mọi bước còn lại.
