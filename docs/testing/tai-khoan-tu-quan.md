# Tài khoản tự quản: vận hành và nghiệm thu

Áp dụng ADR-0053. Đây là danh sách cổng phải đo, không phải tuyên bố đã sẵn
sàng production. Bằng chứng cuối phải ghi SHA, số ca, thời điểm và giới hạn
trong commit bàn giao. Log, ảnh có dữ liệu thật và secret nằm ngoài mọi worktree.

## Writer và đường chạy

Go sở hữu tài khoản, thử thách, outbox mail và migration mới. Python chỉ gỡ
route runtime cũ và giữ oracle lịch sử cho kiểm thử; không thêm auth Python.
Manifest `services/core/ownership/routes.json` là nguồn quyền sở hữu. Auth mới
không đi qua kho idempotency chứa request/response. Profile, nhóm và sổ cái
tiếp tục dùng UUID person; cột discovery cũ được cập nhật trong cùng giao dịch
để các reader legacy giữ quyền truy cập đúng.

Chạy migration Alembic hiện hữu, `core migrate-profile`, rồi
`core migrate-accounts` trước `core serve`. Migration tài khoản giữ person và
sổ cái, thu hồi phiên cũ, gỡ danh tính điện thoại. Không có chuyển tài khoản
điện thoại sang tài khoản mới. Không dùng `MOBILE_ACCOUNT_AUTH_ENABLED=0`
để chữa lỗi production; giá trị này chỉ phục vụ harness legacy cô lập.

## Cấu hình ngoài kho mã

Đặt file cấu hình và secret trong thư mục riêng của operator, quyền thư mục
0700, file 0600. `deploy/vnlocal/up.sh` đọc `~/.config/rudi/accounts.env` và
`ai.env`; không sao chép `.env` vào worktree, build context hoặc log.

Hai khóa `MOBILE_ACCOUNT_ENCRYPTION_KEY` và `MOBILE_ACCOUNT_LOOKUP_KEY` là hai
giá trị base64 ngẫu nhiên 32 byte khác nhau. Sao lưu cùng DB bằng kênh riêng;
mất khóa làm mất khả năng giải mã mail/thử thách và tra cứu tài khoản. Không
đổi khóa tại chỗ khi chưa có migration xoay khóa.

Redis auth riêng có mật khẩu, AOF và `noeviction`. Khi Redis lỗi hoặc đầy,
auth đóng bằng 503; không fallback limiter trong RAM. Chỉ cấu hình CIDR proxy
thật sự thay thế `X-Forwarded-For` bằng đúng một địa chỉ client. Proxy không
được giữ header do client cung cấp. Khi chưa kiểm chứng proxy, để danh sách
trống và chấp nhận giới hạn theo địa chỉ proxy.

SMTP yêu cầu TLS và kiểm chứng certificate. Cấu hình host, port, username,
password và sender đã xác minh; không bật chế độ bỏ TLS hoặc ghi OTP ra log.
Brevo Free có hạn mức 300 mail/ngày. Sender Gmail và tên miền Funnel phục vụ
nghiệm thu ban đầu; mở rộng công khai cần đánh giá hạn mức, tên miền gửi,
SPF/DKIM/DMARC và quyền sở hữu tên miền OAuth.

Google backend chỉ nhận web client ID đã chỉ định. Android Credential Manager
cũng dùng audience này. Web phải có origin HTTPS trùng bản dựng; Android phải
có package và SHA-1 đúng certificate của APK dùng để nghiệm thu. Client debug
không chứng minh release. Google Testing chỉ phục vụ tài khoản test được cho
phép; không gọi là ứng dụng OAuth đã được xuất bản hoặc xác minh.

Web dùng cửa sổ GIS `popup`, không tự chọn tài khoản. Khi proxy bỏ prefix
`/api`, phải đổi Path của riêng cookie `rudi_web_session` từ `/sessions/web`
sang `/api/sessions/web` trên cả set và clear. Giữ HttpOnly, Secure và
SameSite=Strict; không mở Path thành `/`. Ví dụ trong Caddy `reverse_proxy`:

```caddyfile
header_down Set-Cookie "(^rudi_web_session=[^;]*; Path=)/sessions/web(;)" "$1/api/sessions/web$2"
```

Sau mỗi lần export web, recreate container phục vụ thư mục export để mount
nhận đúng thư mục mới. Kiểm cả trang HTML và health API; chỉ health xanh
không chứng minh bản web đã được phục vụ. Trang bảo mật phải đợi khôi phục
phiên hoàn tất trước khi quyết định chuyển sang đăng nhập.

## Cổng máy và đối chứng

Trong checkout sạch đúng SHA, chạy `make gate` với toàn bộ dependency của CI,
hoặc `scripts/gate.sh --strict` để thiếu prerequisite là lỗi. Không sửa script
đang chạy. PostgreSQL và Redis thật là bắt buộc; skip không thay bằng pass.

- `scripts/go_postgres_tier.sh` kiểm migration, tranh chấp, thu hồi phiên,
  nonce một lần, liên kết sai tài khoản, discovery và xóa tài khoản.
- `scripts/account_auth_mutants.sh` nhận cây sạch, chạy identity xanh và
  canary sai kỳ vọng đỏ, rồi hai mutant: bỏ kiểm nonce và bỏ thu hồi phiên
  reset. Mỗi mutant phải đỏ đúng test hành vi, không phải lỗi compile.
- `scripts/mobile_native_gate.sh` dựng APK và stack QA riêng, chạy hai lượt
  Maestro, kiểm fingerprint bản dựng và canary sai fingerprint. Mở ảnh để xem.
- `scripts/e2e_slice.sh` và `scripts/chat_e2e_stack.sh` chỉ tạo fixture tổng hợp
  trong DB dùng một lần. Kiểm bản đồ web thường/không WebGL không chứng minh
  tile mạng, email thật, Google thật hoặc crypto native.
- Đo tải 15 phút bằng tag `authload` trên tầng PostgreSQL: hai replica,
  100 client, 18.000 login được pace 20/giây, rate limit vẫn bật. p95 tối đa
  một giây và lỗi ngoài dự kiến dưới 0,1%. Ghi mức đồng thời thực đo;
  100 client worker không đồng nghĩa luôn có 100 request đang xử lý.

## Nghiệm thu bằng dữ liệu thật

Chuẩn bị stack prelaunch riêng, catalog thật và không có dữ liệu demo. Sao lưu
DB/khóa trước migration, thử restore vào DB riêng và kiểm số hàng/sổ cái.
Rollback auth không được bật lại cửa bootstrap điện thoại hoặc mất thu hồi
phiên; xử lý lỗi bằng bản vá forward hoặc bản backup đã kiểm chứng.

Tạo A qua form đăng ký, nhận OTP trong hộp thư thật; tạo B bằng Google thật.
Người dùng tự nhập password, OTP và thông tin xác minh trong UI, không gửi vào
chat. Không seed user, đọc mã từ DB hoặc dùng debug OTP cho nghiệm thu này.
Kiểm cả web HTTPS và Android release: đăng nhập sai/đúng, restart giữ phiên,
logout thu hồi, reset vô hiệu mọi phiên, thay email/password sau reauth,
liên kết/gỡ Google, tra username khi discovery bật/tắt, chặn người khác, bạn,
nhóm và buổi đi. Kiểm B không đọc/ghi dữ liệu riêng của A.

Recovery tài khoản không recovery khóa E2EE. Không fallback chat plaintext.
Các cổng crypto Android/iOS, kiểm chứng độc lập và tải chat vẫn là cổng riêng.
Sau triển khai theo dõi ít nhất một giờ: lỗi auth/5xx, latency, Redis,
PostgreSQL, outbox và SMTP. Chỉ ghi số đếm/tổng hợp, không lưu credential hoặc
nội dung mail vào bằng chứng trong Git.
