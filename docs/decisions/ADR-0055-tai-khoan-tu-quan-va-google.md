# ADR-0055 — Tài khoản tự quản và Google, bỏ danh tính điện thoại

Ngày: 2026-10-04. Trạng thái: quyết định sản phẩm đã được chủ sản phẩm chốt
trong phiên lập kế hoạch; các cổng triển khai và nghiệm thu vẫn chưa hoàn tất.

## Quyết định

Ứng dụng chưa ra mắt. Không di chuyển tài khoản điện thoại. Người dùng mới có
UUID ngẫu nhiên bất biến, tên hiển thị riêng và `@username` công khai, cố định
trong đợt đầu. Username gồm 3–32 ký tự ASCII chữ, số, dấu chấm hoặc gạch dưới,
chuẩn hóa thành chữ thường. Tra cứu bạn và mời vào nhóm dùng username, có tùy
chọn cho phép tìm thấy. Giữ lời mời vào nhóm và buổi đi; bỏ lời mời đích danh
đổi lấy phiên, OTP SMS, suy UUID từ số và tra cứu bằng số.

Tài khoản tự quản đăng nhập bằng username và mật khẩu. Email phải xác minh
bằng mã sáu số trước khi tạo tài khoản hoạt động, dùng cho khôi phục mật khẩu
và thay email. Email cắt khoảng trắng, chữ thường, không gộp dấu chấm hay
`+tag`; lưu bản mã và HMAC phục vụ tra cứu. Không dùng email làm UUID.

Mật khẩu 8–128 ký tự theo điều chỉnh của chủ sản phẩm ngày 2026-10-04,
nhận Unicode và khoảng trắng, NFC, không cắt hoặc trim,
chặn mật khẩu thông dụng. Argon2id ít nhất 19 MiB, hai lượt, một luồng, salt
riêng và tham số có phiên bản. Không yêu cầu đổi định kỳ hoặc thành phần ký tự.
Phiên opaque hiện có giữ TTL 30 ngày, chỉ lưu digest. Reset mật khẩu thu hồi
tất cả phiên, đăng nhập lại bình thường. Không thêm MFA cho ứng dụng đợt này.

Mốc tám ký tự là quyết định trải nghiệm của sản phẩm, không phải bằng chứng
đáp ứng chính sách mật khẩu single-factor của NIST SP 800-63B-4. Chuẩn này
yêu cầu tối thiểu 15 ký tự cho single-factor và cho phép tám ký tự khi mật
khẩu chỉ là một phần của MFA. Xác minh email khi đăng ký không biến các lượt
đăng nhập chỉ dùng mật khẩu thành MFA. Vẫn khuyến nghị câu mật khẩu dài,
giữ blocklist, Argon2id và giới hạn thử dùng chung giữa replica.

Google xác minh server bằng thư viện chính thức, kiểm chữ ký, issuer, audience,
hạn và nonce một lần gắn với bí mật của lượt bắt đầu. Danh tính là `(issuer, sub)`.
Người dùng Google mới chọn username trước khi tạo person. Không gộp theo email.
Liên kết phải có phiên vừa xác thực lại và chứng minh Google hiện tại; không
chuyển danh tính đã thuộc người khác, không chuyển dữ liệu giữa hai tài khoản.
Không được gỡ cách đăng nhập cuối cùng. Thay mật khẩu, email và liên kết cần
xác thực lại không quá năm phút. Android dùng Credential Manager, web dùng
Google Identity Services; iOS là cổng riêng chưa được nhận là đã kiểm chứng.

Go sở hữu toàn bộ nghiệp vụ và migration SQL mới. PostgreSQL là nguồn sự thật,
Redis hạn chế thử dùng chung giữa replica và đóng khi lỗi. Thử thách email có
TTL 300 giây, tối đa năm lần sai, chờ gửi lại 60 giây, tối đa năm yêu cầu trong
15 phút mỗi email. Mỗi thử thách có bí mật ngẫu nhiên phía client; DB chỉ giữ
digest. Gửi lại vô hiệu mã cũ. PostgreSQL outbox chứa payload mã hóa, worker Go
retry có giới hạn và không gửi mã hết hạn. Production không có debug OTP hoặc
fallback ghi mã ra log. Giới hạn hash đồng thời, request, timeout và pool.

Đường auth mới không đi qua kho idempotency ghi request/response. Không ghi
mật khẩu, mã, token, email hoặc credential vào log, Git, screenshot bàn giao.
Native giữ phiên trong SecureStore; web dùng cookie HttpOnly/Secure/Strict hiện
có và bearer trong bộ nhớ. Khôi phục tài khoản không khôi phục khóa E2EE;
không thêm đường chat plaintext hoặc khóa giải mã server.

Ngoại lệ Python chỉ để gỡ đăng ký route cũ và vá tương thích/bảo mật runtime
legacy. Theo ADR-0036, retirement Go, Python và manifest phải cùng thay đổi;
giữ bằng chứng lịch sử và fixture tổng hợp cô lập, không xóa test để gọi xanh.

## Hạ tầng và dữ liệu nghiệm thu

Admin do chủ sản phẩm chỉ định: hộp thư rudiappmobile. Đợt nghiệm thu ban đầu
dùng HTTPS Tailscale Funnel và Brevo Free SMTP, không mua tên miền. Giới hạn
gửi và hạ tầng miễn phí phải ghi rõ; trước mở rộng công khai cần tên miền
gửi đã xác thực và đánh giá lại dung lượng. Google Cloud hiện yêu cầu admin
bật xác minh hai bước; việc này không phải MFA của app.

Gỡ công cụ/demo seed của sản phẩm và dữ liệu demo có nguồn gốc xác định; giữ
catalog thật và fixture CI cô lập. Không reset toàn DB hay sửa sổ cái để dọn.
Dữ liệu thật và credential nằm ngoài mọi worktree, chỉ đưa số đo đã khử danh
tính vào tài liệu. Nghiệm thu trên stack prelaunch bằng hai tài khoản tạo qua
UI: A đăng ký local, B Google; Android bản release và web. Không seed hoặc
dùng mã debug để gọi là dữ liệu thật.

## Cổng bắt buộc và giới hạn bằng chứng

Chạy gate trong cây sạch đúng SHA, PostgreSQL/Redis thật, kiểm race, replay,
email lỗi, giới hạn nhiều replica, token Google sai audience/nonce, link sai
người, CSRF, thu hồi phiên và quyền giữa hai tài khoản. Canary đỏ đúng bước,
ít nhất hai mutant không tương đương đỏ đúng bước, mở screenshot UI để xem.
Đo tải cô lập hai replica, 100 client đồng thời, 20 login hợp lệ/giây trong
15 phút, p95 ≤ 1 giây, lỗi ngoài dự kiến < 0,1%; không tắt rate limit để đo.
Kiểm phục hồi sau lỗi Redis/PG và restart riêng. Theo dõi ít nhất một giờ sau
triển khai. Các consumer profile, bạn, nhóm, buổi đi phải giữ UUID và quyền.
Khôi phục test bản đồ web thường và không WebGL bằng fixture tổng hợp.

Green unit test không chứng minh email thật, Google thật, crypto native, tải
hay vận hành. Chỉ nhận production-ready khi từng cổng có bằng chứng; công việc
chưa đạt phải ghi rõ, không thay bằng bảng xanh hoặc tài khoản seed.

## Nguồn

- [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [NIST SP 800-63B](https://pages.nist.gov/800-63-4/sp800-63b.html)
- [Google ID token verifier](https://pkg.go.dev/google.golang.org/api/idtoken)
- [Android Credential Manager](https://developer.android.com/identity/sign-in/credential-manager-siwg)
- [Google Identity Services](https://developers.google.com/identity/gsi/web/reference/js-reference)
