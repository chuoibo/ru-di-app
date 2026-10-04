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

## Bổ sung sau review trước merge (2026-10-05)

Ba review độc lập (backend, migration/retirement, frontend) trước khi merge
tìm ra các đường lạm dụng giới hạn thử. Quyết định đã triển khai:

- **Đăng nhập chỉ đếm lần sai.** Mười lần sai từ một địa chỉ cho một username
  trong 15 phút thì tạm dừng đúng cặp đó; một trăm lần sai từ mọi nơi trong
  24 giờ thì tạm dừng username; một trăm lần sai từ một địa chỉ trong một giờ
  thì tạm dừng địa chỉ — trừ các địa chỉ chủ tài khoản đã đăng nhập đúng
  trong 30 ngày, nơi trần theo username không áp dụng. Mỗi lượt giữ chỗ
  nguyên tử trên mọi ngân sách trước khi băm và chỉ hoàn lại khi không sai,
  nên request song song không vượt được trần. Đăng nhập đúng xoá bộ đếm của
  cặp; đặt lại mật khẩu qua email xoá bộ đếm của username. Người lạ không còn khoá được chủ tài
  khoản chỉ bằng một lần sai mỗi sáu giây. Xác thực lại: 10 lần sai/15 phút,
  30 lần/ngày mỗi người.
- **Mã email có trần xuyên qua các lần gửi lại.** Trong 24 giờ, mỗi loại
  thử thách của một email nhận tối đa mười lần nhập sai từ một địa chỉ và ba
  mươi lần từ mọi nơi, kể cả nhiều thử thách sống cùng lúc; hết trần thì mã
  đúng cũng bị từ chối và không phát mã mới (`challenge_attempts_exhausted`).
  Trước đây gửi lại tạo bộ đếm mới, cho khoảng 2.400 lần đoán/ngày.
- **Phát mã có ngân sách theo client**: 20/giờ và 50/ngày mỗi địa chỉ, cộng
  5/15 phút mỗi email; hạn mức mail của nhà cung cấp (`MOBILE_EMAIL_DAILY_LIMIT`,
  mặc định 300) được kiểm lúc phát mã, trả `mail_unavailable` như nhau dù email
  có tài khoản hay không, và chỉ mail được nhận mới tính vào hạn mức. Đăng ký
  dừng ở bốn phần năm hạn mức để phần còn lại luôn dành cho khôi phục và đổi
  email.
  `challenge_resend_limited` chỉ còn nghĩa chờ 60 giây; quá năm mã/15 phút là
  `challenge_quota_reached`.
- **Đổi email không còn cho biết địa chỉ đã có chủ.** Yêu cầu luôn trả 202
  và gửi mã; trùng chỉ báo `email_unavailable` sau khi nhập đúng mã. Mỗi
  người tối đa năm yêu cầu đổi email mỗi giờ.
- **Username công khai và cố định cả sau khi xoá.** Đăng ký báo username đã
  có ngay, vì username hiện trên hồ sơ và lời mời. Xoá tài khoản giữ digest
  SHA-256 của username trong `retired_usernames`; trigger từ chối cấp lại, để
  người sau không mạo danh người đã xoá.
- **Đặt lại mật khẩu kiểm thử thách trước khi băm**; hàng đợi Argon2 chờ tối
  đa hai giây thay vì từ chối ngay. JSON từ chối khoá trùng nhau chỉ khác hoa
  thường (bộ giải mã Go ghép field không phân biệt hoa thường).
- **Migration có phiên bản, chỉ thêm.** Phiên bản 1 đã cài trên stack
  nghiệm thu không bao giờ sửa tại chỗ; phiên bản 2 (`schema_v2.sql`) sửa
  trigger để người đã xoá không còn `discoverable_by_phone`, thêm
  `retired_usernames` và xoá `otp_challenges` (HMAC số điện thoại) còn sót.
  `core serve` từ chối chạy khi thiếu phiên bản nào; vận hành chạy lại
  `core migrate-accounts` sau khi nâng bản.

- **Phiên sống không thuộc tài khoản tự quản** (phiên genesis của operator)
  nhận 403 `managed_account_required` ở các cửa tài khoản, không phải 401:
  client coi 401 `authentication_required` là phiên đã bị thu hồi và đăng xuất.

Giới hạn còn lại, ghi rõ: trần theo username vẫn cho phép người có nhiều địa
chỉ tạm dừng đăng nhập mật khẩu của một username tới 24 giờ ở những địa chỉ
chủ tài khoản chưa từng dùng (gỡ bằng đặt lại mật khẩu); trần mã theo email cho phép người có từ ba địa chỉ
trở lên làm một email không xin được mã reset trong 24 giờ. Đổi lại, xác suất
đoán trúng mã sáu số giảm từ khoảng 7%/tháng xuống khoảng 0,09%/tháng mỗi tài
khoản. Mọi
giới hạn theo địa chỉ chỉ đúng khi `MOBILE_AUTH_TRUSTED_PROXY_CIDRS` trỏ đúng
proxy làm sạch `X-Forwarded-For`; để trống sau proxy thì mọi người dùng chung
một địa chỉ. `PATCH /people/me` vẫn trả lại giá trị `discoverable_by_phone`
được gửi lên dù trigger giữ giá trị của tài khoản tự quản: route này còn so
từng câu SQL với oracle Python, nên sửa phải gỡ field ở cả hai phía theo
ADR-0036; client hiện không còn gửi field này.

## Nguồn

- [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [NIST SP 800-63B](https://pages.nist.gov/800-63-4/sp800-63b.html)
- [Google ID token verifier](https://pkg.go.dev/google.golang.org/api/idtoken)
- [Android Credential Manager](https://developer.android.com/identity/sign-in/credential-manager-siwg)
- [Google Identity Services](https://developers.google.com/identity/gsi/web/reference/js-reference)
