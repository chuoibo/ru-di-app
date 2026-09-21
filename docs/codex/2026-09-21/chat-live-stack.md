# Stack thật cho kiểm thử chat nhiều người

Stack riêng được dựng từ checkout hiện tại ngày 21/09/2026 bằng
`scripts/chat_e2e_stack.sh`. Đây là **candidate Go**, không phải bằng chứng
writer production đã chuyển sang Go. Chat trên giao diện hiện tại vẫn là chat
legacy qua HTTP/polling; kết quả này không chứng minh MLS/E2EE hoặc WebSocket v2.

## Thành phần và tái lập

```sh
bash scripts/chat_e2e_stack.sh up
node scripts/chat_e2e_seed.mjs /tmp/rudi-chat-e2e.XXXXXX/connection.json
# Sau khi toàn bộ browser/native đã kiểm xong:
bash scripts/chat_e2e_stack.sh down /tmp/rudi-chat-e2e.XXXXXX
```

Thay `XXXXXX` bằng thư mục mà lệnh đầu in ra. Không đọc `.env`, không dùng dữ
liệu người thật. Metadata kết nối và session tổng hợp chỉ nằm ngoài repo, trong
thư mục tạm quyền 0700. Token không in ra log kiểm thử hoặc báo cáo.

- PostgreSQL 16 riêng, migration thật đến `d6a2f93b81e7`, seed catalog thật.
- `fsync`, `synchronous_commit`, `full_page_writes` đều **on**. Không dùng cờ
  tăng tốc tắt durability từ harness parity cũ.
- Go `cmd/core` được build từ checkout, `MOBILE_CORE_CANDIDATE_ROUTES=ported`,
  log khởi động ghi 151 route Go. Auth `prod`, bearer session thật.
- Python upstream chạy Docker image build từ checkout hiện tại, dùng chung
  database/media trong stack riêng này. Không thêm backend Python mới.
- Không có API giả, model giả, routing stub hoặc khoá AI bên ngoài. Python còn
  là upstream cho route chưa chuyển và AI; đây là giới hạn migration hiện tại.
- Cả ba container tồn tại sau khi shell khởi tạo kết thúc. Media nằm ngoài repo.
  `down` chỉ xoá container được ghi trong danh sách của lần chạy này.

Seed tạo 22 tài khoản tổng hợp qua **OTP request → verify thật**, cập nhật tên
qua API, tạo nhóm, mời và nhận lời qua API. Không seed bảng people/session trực
tiếp bằng SQL. OTP debug chỉ dùng trong stack cô lập, không gửi SMS. Limiter
10 request/phút giữ nguyên; harness tôn trọng 429 và chờ cửa sổ kế tiếp.
Hai cặp DM được kết bạn, chấp nhận rồi mở DM qua API. Gọi DM khi chưa kết bạn
trả 404, đúng ràng buộc hiện tại.

Lần chạy thực tế dùng API `http://127.0.0.1:57648`; fixture nằm ở
`/tmp/rudi-chat-e2e.Nllrhk/sessions.json`. Seed ban đầu có 20 thành viên nhóm;
sau đó thêm user thứ 21 cho kiểm UI độc lập. User thứ 22 giữ ngoài nhóm để kiểm
từ chối truy cập. Tải 20 browser phải ghi **20 client đồng thời / 21 thành viên
nhóm**, không đổi thành 21 client. CORS preflight từ web `127.0.0.1:8177` trả 204.

## Kiểm chức năng qua HTTP thật

Probe dùng một nhóm riêng, không bơm dữ liệu vào nhóm đo đồng bộ chính. Kết quả
thô tổng hợp lưu ngoài repo ở `api-probes.json`, `api-probes-extra.json` trong
thư mục chạy. Đây là kiểm API trên stack thật, không thay bằng chứng UI E2E.

| Ca thực thi | Kết quả |
|---|---|
| Gửi chữ, trả lời tin | 201; lưu và đọc lại qua PostgreSQL |
| Sticker `di-thoi`, reaction heart | 201 |
| Upload PNG tổng hợp, gửi image, thành viên tải ảnh | 201 / 201 / 200 |
| Người ngoài đọc nhóm / đọc DM | 403 |
| Xoá tin của người khác | 403 |
| Chủ tin xoá, sau đó thả reaction | 204, rồi 409 `message_deleted` |
| Gửi lại cùng Idempotency-Key | 201, cùng message ID |
| Đánh dấu đọc với `message_id` đúng contract | 200 |
| Payload read-mark sai field | 422, không ghi thành công |
| Voice `kind=voice` | 422: schema chưa hỗ trợ voice |
| `/vote Uống gì? Cà phê \| Trà` | 201, tạo vote thật và poll card, không cần model |
| `/plan Đi cà phê ngày mai` | 201 lưu lệnh, nhưng `companion.spoke=false`, `reason=unavailable` |
| `/chia-bill` | 201 lưu lệnh, nhưng `intent_error=chia_bill_not_available` |
| ai-turn explicit riêng | 200 nhưng `spoke=false`, `reason=unavailable` |
| DM hai người đã kết bạn | Gửi 201, người kia đọc 200 |

Không được ghi bot đã hoạt động chỉ vì ai-turn trả HTTP 200: provider AI chưa
cấu hình và không sinh phản hồi. Không được gọi bộ probe này là kiểm tải,
native hai hệ điều hành, test người thật, hay bằng chứng production-ready.

Script khởi tạo cuối cùng đã được chạy lại trên một stack mới độc lập:
health thành công, OTP request trả 202 có challenge, cả ba thiết lập durability
đều on. Sau đó `down` xoá đúng ba container của stack kiểm tái lập; stack bài
20 browser vẫn giữ nguyên. `bash -n` và `node --check` đạt. Seed đầy đủ 22 người
đã được thực thi trên stack chính qua các API nêu trên; không chạy lại seed đè
lên session của browser đang kiểm.

Lưu ý lần chạy chính khởi tạo trước khi sửa healthcheck của harness: Docker
image mặc định thăm port 8000 nên trạng thái container báo `unhealthy` dù API
dynamic port vẫn phục vụ bình thường. Script hiện override healthcheck đúng
port; đã dựng thêm stack riêng và chạy chính command healthcheck của cả API/Go
thành công. Không restart stack đang đo chỉ để đổi trạng thái Docker.
