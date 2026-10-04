# Bảng Maestro trên Android

Chạy bằng `scripts/mobile_native_gate.sh` (chặng `mobile-native` của
`scripts/gate.sh` và job `mobile-native` của CI): dựng APK debug từ cây này, dựng
stack QA cô lập (`scripts/e2e_slice.sh --native`), tạo thế giới tài khoản tổng hợp
`native` qua cửa HTTP thật (`TestProvisionSyntheticAccountWorld`), rồi
`scripts/mobile_native.sh --account` chạy mọi flow có số theo `case` của nó.

Người của bảng theo vai, mỗi lượt bảy tài khoản `fixture_n<lượt><a..g>`, mật
khẩu sinh lúc chạy, tệp `credentials.json` nằm trong thư mục tạm của stack và chết
cùng stack. Flow chỉ nhận `TK_A..TK_F` / `MK_A..MK_F` qua `-e`; đăng nhập đi qua
`_dang-nhap-tai-khoan.yaml` (username + mật khẩu, ADR-0055).

## Flow đã gỡ cùng cửa cũ (ADR-0055)

Chỉ gỡ flow mà TOÀN BỘ mục đích là một cửa đã bỏ; mọi flow còn đo sản phẩm được
chuyển sang tài khoản.

| Flow | Mục đích cũ | Vì sao gỡ |
|---|---|---|
| `20-the-gioi-seed.yaml` | Thế giới seed «Team Đà Lạt» nhìn từ máy, đăng nhập bằng số của một người seed (`--live --otp-phone`) | Cửa số điện thoại và công cụ seed (`seed:rudi`) đã gỡ; không còn người seed nào đăng nhập được |
| `21-dang-nhap-that.yaml` | Đổi một lời mời đích danh lấy phiên (`--dang-nhap`, `rudi://moi/<mã>`) | Cửa lời mời lấy phiên đã gỡ: `/moi` giờ chuyển về màn đăng nhập |
| `_dang-nhap-so.yaml` | Đăng nhập bằng số cố định cho flow 20 | Đi cùng flow 20 |

Bản gốc của ba file trên nằm ở `apps/mobile/tests/fixtures/maestro-da-go/`, không
chạy ở bảng nào; chỉ `scripts/bang_doi_chieu_mockup.py` đọc tên ảnh lịch sử của
chúng (khai báo, không phải bằng chứng).

Đã chuyển, không gỡ:

- `22-dang-nhap-otp.yaml` → `22-dang-nhap-tai-khoan.yaml`: phần ô số + mã 6 số
  + «Mã chưa đúng. Còn 4 lần thử.» thay bằng mật khẩu sai → «Tên tài khoản hoặc
  mật khẩu chưa đúng.» rồi mật khẩu đúng; trạng thái rỗng, mở nhóm và Khám phá
  sống giữ nguyên. Canary của bảng chạy lại flow này với mật khẩu sai.
- 24: mời vào nhóm bằng tên tài khoản (không còn đặt tên thay người được mời —
  D tự đặt «Ban QA»); 25: tìm bạn bằng username; 44: công tắc «Cho tìm theo
  username» ở Tài khoản & bảo mật thay «Cho tìm theo số điện thoại».
- `_dang-nhap-c/d/f.yaml`: gọi `_dang-nhap-tai-khoan.yaml` với người tương ứng.
- `50-account-login.yaml` (mới cùng ADR-0055) chạy cuối mỗi lượt bằng người B.

Flow `_community*`, `_diary*` là flow lẻ chạy bằng tay, không thuộc bảng (như
trên `main`).
