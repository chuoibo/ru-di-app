# Sổ kỷ niệm — triển khai và kiểm chứng native

> Trạng thái mới nhất: [bàn giao 28/09/2026, mục «Lượt tiếp nối»](ban-giao-so-ky-niem-2026-09-28.md): bảng broad OTP+AI xanh 26/26, chuỗi diary release, một lỗi xoá-từ-link đã sửa. Số đo cổng đầy đủ nằm ở commit gộp vào main.

## Phạm vi

Go sở hữu ending, sổ cá nhân, quyền xem, ảnh và worker. Python chỉ dựng/kiểm
lời AI từ phần người dùng đã chọn. Khép cuộc đi không hoàn tất nghĩa vụ tiền.
Sổ mặc định riêng tư; công khai, thu hồi và xóa áp dụng cả đường đọc byte ảnh.

Bản tích hợp mở rộng UI v3, danh mục địa điểm và Cộng đồng trên main
`33d29fe5`. Giữ nút gửi sổ công khai lên Cộng đồng để duyệt.
Máy Android thật được người dùng chủ động dời tới đợt chốt toàn bộ tính năng;
Android emulator là phạm vi kiểm ở đây. Chưa có bằng chứng iOS runtime.

## Dựng và chạy lại

1. PostgreSQL riêng cho dữ liệu giả lập, áp dụng Alembic hiện hành rồi
   `core migrate-chat` và `core migrate-diaries`. Migration diary có checksum
   và khóa giao dịch, không tự chạy khi nhận request.
2. API Go: `MOBILE_DATABASE_URL`, `MOBILE_MEDIA_ROOT`, `MOBILE_PERSON_ID_KEY`,
   `MOBILE_AUTH_MODE=prod`, `MOBILE_INTERNAL_TOKEN`, `MOBILE_PYTHON_UPSTREAM`.
   Chỉ môi trường thử dùng log sender và `MOBILE_OTP_DEBUG_CODE=000000`.
3. Brain nhận `GEMINI_API_KEY` qua môi trường; `GEMINI_DIARY_MODEL` tùy chọn.
   Không đưa khóa vào Expo, APK, Git hay log. Worker có hai consumer, lease,
   tối đa ba lượt và hạn một giờ; model bị giới hạn tối đa một lần viết lại.
4. Trong `apps/mobile`, chạy `tools/seed-diary-native.mjs` bằng Node với
   `DIARY_TEST_API=http://127.0.0.1:<cổng API>` và `DIARY_TEST_OUTPUT` ngoài
   mọi worktree. Seeder tạo tài khoản, hội, hai cuộc đi và ba tranh sọc giả lập.
   File kết quả chứa phiên thử, tuyệt đối không commit. Ngày fixture dùng
   Asia/Ho_Chi_Minh, cùng phép chiếu ngày ảnh của API; không dùng ngày UTC.
5. Dựng/cài APK Expo native, chọn rõ serial emulator. Release không cần Metro;
   development client cần bundle đúng cây và dấu vân của lượt chạy.
   APK local test cho HTTP tới loopback/emulator không phải APK production.
6. Các flow `.maestro/_diary*.yaml` cần truyền `OUTING_ID`, `MOMENT_ID`,
   `DIARY_ID`, `PHOTO_DAY` từ fixture. Đăng nhập đúng chủ fixture trước khi chạy.
   `_diary.yaml` và `_diary-moment.yaml` bắt đầu từ cuộc đi chưa khép;
   `_diary-edit.yaml` cần sổ hai trang; `_diary-ai.yaml` cần provider thật;
   `_diary-ai-unavailable.yaml` cần provider bị tắt tại môi trường thử riêng.
7. `_diary-cold.yaml` đo link lạnh trên **release**, không qua launcher Expo.
   `_diary-privacy.yaml` dùng `EXPECTED_AUDIENCE`: identity «Chỉ mình tôi»
   phải xanh, canary «Công khai» phải đỏ ở assertion cuối, cùng file/harness.
8. `scripts/mobile_native.sh --serial <serial> --port <Metro riêng>
   --api-port <API riêng> --otp --ai` kiểm các luồng sống và hậu điều kiện API.
   Bảng fixture chạy riêng không có `--otp`; không cộng hai chế độ thành một.

## Bằng chứng và giới hạn

Chỉ dùng ảnh giả lập và trích đoạn tự viết cho thử nghiệm. Mọi PNG/video/log
native ở thư mục bằng chứng ngoài Git hoặc `.impeccable/review/` được bỏ qua.
Phải mở PNG để kiểm mắt; màn trắng/lỗi launcher không phải ảnh duyệt UI.
Chọn bìa, đổi thứ tự trang, công khai/thu hồi cần đối chiếu API/PostgreSQL,
không chỉ nhìn thông báo trên màn.

Ma trận kiểm: phone 1080×2400, tối/chữ 1.3/giảm chuyển động, phone 720×1280
chữ 2.0, màn lớn 1600×2560. Màn lớn là đổi kích thước emulator, không phải
thiết bị tablet vật lý. Các trạng thái đọc, sửa, chọn bìa và Khoảnh khắc được
chụp riêng. UI mới giữ `SoBia`, `TrangSo`, `KhungAnh`, `Washi`, `ONhapMuc`,
`StampButton`; reviewer yêu cầu cột 560dp, vào sửa không cuộn qua cả sách,
và trạng thái bìa hiện tại. Xem `.impeccable/surfaces/diary.md` và DESIGN.md.

Bằng chứng trước lượt sửa review, sau tích hợp `0b51d5be`:

- 12 ca PostgreSQL diary và 2 ca domain đạt; 18 ca hợp đồng AI đạt.
- 11 chặng preflight đạt: guard, ruff, contract, client-routes, server-routes,
  screens, cors, ownership, python-touch, go-vet, go-test. Cây này đang có diff;
  đây **không thay** cổng trên worktree sạch ở SHA sẽ đưa lên main.
- Hai mutant không tương đương: mở ACL sổ riêng cho người ngoài và bỏ kiểm tra
  ảnh do model trả. Identity xanh; mỗi mutant đỏ đúng test dự đoán, cùng harness.
- Release có 614 frame trong phép cuộn lên/xuống, Android ghi 30 janky (4,89%),
  p50 25ms, p95 36ms, 3 missed-vsync. Chỉ số legacy ghi 74,76% janky; GPU
  SwiftShader không cho phép kết luận đạt 60fps hay suy rộng sang máy thật.
  Log `/tmp/rudi-v3-gfxinfo.txt`, video `/tmp/rudi-v3-evidence/motion.mp4`.

Lịch sử UI v3 trước merge danh mục: candidate `79650fe8` chạy full gate
23 đạt, 0 hỏng, 4 bỏ qua (demo-watch, hero-walk, mobile-native, crypto).
Crypto chạy riêng sau đó đạt. Mobile 1.224 ca; Python product/meta 3.780 đạt,
803 bỏ qua; parity 354 kịch bản/10.835 bước không khác; chat E2E 43 đạt.
Đây là lịch sử, **không phải kết quả của SHA mới**. Không cộng các lượt thành
«27 cổng xanh». Hai chặng demo-watch/hero-walk đo vận hành demo trên máy,
không thể thay bằng test tính năng hoặc tạo cron chỉ để có màu xanh.

Lượt release diary lịch sử đã đi qua khép chuyến/Khoảnh khắc, tự dựng,
AI thật, lỗi AI giữ revision cũ, đổi bìa/đảo trang, picker ảnh Android và
link lạnh. Người đọc độc lập nhận 200 cho sổ/ảnh khi công khai, 404 sau thu hồi.
Một lần Gemini từ chối đầu ra; lần thử lại được kiểm và lưu. Không che lần đỏ
đó hoặc coi output model ngẫu nhiên là kết quả tất định.

Cổng đưa lên main phải chạy lại trên cây sạch đúng SHA; commit và bàn giao
cuối ghi số liệu, phạm vi thực chạy và các ngoại lệ, không lấy bảng lịch sử
ở trên thay kết luận hiện hành.

Sau ghép Cộng đồng `33d29fe5`: typecheck đạt; mobile 1.239 ca đạt trước
khi khôi phục thêm test tường nhóm của upstream; lượt targeted sau đó 11 ca đạt.
PostgreSQL diary 12 ca và domain 2 ca đạt; AI 18 ca đạt. Preflight 10 chặng
đạt; ruff không chạy vì diff so với main không còn sửa Python (strict ghi
1 hỏng do không có đầu vào, không phải lỗi lint). Cổng cuối tách ruff không
áp dụng khỏi các chặng bắt buộc; không sửa Python giả tạo để biến thành xanh.
