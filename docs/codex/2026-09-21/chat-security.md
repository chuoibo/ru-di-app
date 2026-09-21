# Vá bảo mật chat legacy và bản Go — 2026-09-21

## Phạm vi và ngoại lệ Python

Đây là ngoại lệ sửa bảo mật/hồi quy được AGENTS.md cho phép. Khi tái đặt PR
lên main `63959c1d`, manifest đã chuyển 126 route sang LIVE-GO qua PR #623.
Bản vá Go bảo vệ đường chat hiện hành; bản vá Python đóng cùng hai lỗi ở
runtime compatibility và giữ đối chiếu hồi quy. PR này không đổi manifest
writer, không thêm tính năng backend Python mới, không đổi schema hoặc luật tiền.

1. Idempotency từng trả nội dung chat đã cache mà bỏ qua phiên đăng nhập,
   membership và chặn DM hiện tại. Cả hai runtime nay đưa replay vào pipeline
   xác thực rồi kiểm quyền đọc hội thoại, trạng thái pair, và đích tin/author
   khi URL chỉ định tin. Replay hợp lệ trả nguyên byte đã lưu; không gửi lại
   tin, không gọi AI, không thực hiện lại mutation. Replay delete của chính
   tác giả vẫn thành công với tombstone; reaction/expense-draft của tin đã
   xoá bị từ chối.
2. Read mark từng SELECT rồi INSERT/UPDATE không có khoá: hai lần đọc đầu có
   thể đụng unique constraint; lần đọc cũ có thể ghi đè vị trí mới. Nay dùng
   một UPSERT PostgreSQL so tuple `(last_read_at, last_read_message_id)`.
   Python dùng RETURNING với `populate_existing` để identity map không trả
   vị trí cũ. Gửi lại cùng/vị trí cũ giữ nguyên watermark và updated_at.

## Bằng chứng

- Trước sửa, dùng Go overlay và plugin kiểm thử tạm bên ngoài repo để nạp
  đúng hai method Python tại HEAD, không tráo source tracked. Ca HTTP trả
  `201` kèm body riêng tư sau rời nhóm, thu hồi bearer và chặn DM. Ca hai
  connection thật chứng minh watermark lùi và lỗi `pk_context_read_marks`.
  Năm ca PostgreSQL Python cùng đỏ ở bản cũ.
- Bản sửa Go: HTTP qua dispatch/endpoint thật, session production, membership,
  blocked pair; replay trả đúng byte và không nhân đôi tin; mọi loại mutation
  chat phải đi qua reauth. PostgreSQL kiểm contention có quan sát lock,
  first-read, watermark hiện hữu, timestamp bằng nhau và UUID tie-break.
  `-race` qua repo/routes/idem đều đạt trong phạm vi các ca này.
- Bản sửa Python: 7 ca tập trung đạt (5 PostgreSQL + 2 API/service);
  84 ca API chat, sticker/reply/delete, idempotency và expense đạt.
- Đã chạy lại trên Python 3.12.3, virtualenv riêng ngoài worktree cài đúng
  `services/api/requirements-dev.txt`: 7 ca tập trung đạt trong 5,33 giây;
  84 ca hồi quy đạt trong 19,34 giây. Ruff 0.9.2 check/format-check và
  `git diff --check` đạt. Kết quả Python 3.13 trước đó chỉ là bổ sung.

Lệnh tái chạy (URL database thử nghiệm do người chạy cấp qua environment;
fixture PostgreSQL Python tự tạo schema riêng rồi xoá schema):

```sh
MOBILE_REQUIRE_POSTGRES_TESTS=1 python -m pytest \
  services/api/tests/postgres/test_chat_security_postgres.py \
  services/api/tests/api/test_chat_replay_authorization.py -q
```

```sh
cd services/core
CORE_REQUIRE_POSTGRES_TESTS=1 go test -race -tags postgres \
  ./internal/routes ./internal/repo ./internal/idem \
  -run 'Test(ChatReplay|DirectChatReplay|ReadMark|EveryChatWrite|TheItineraryReplay)' \
  -count=1
```

## Delta parity và giới hạn

Kiểm lại khi đặt PR lên main `63959c1d`: Go unit packages `idem`, `routes`,
`repo` đạt; tier PostgreSQL thật với `-race` báo 16 ca PASS (kể cả subtest),
có `TestPostgresTierReachesDatabase`, không SKIP. Các ca tập trung replay nhóm,
DM block, read mark contention/tie-break và dispatch reauth. Lượt đầu thiếu
package chứa sentinel nên tier từ chối; lượt sửa lệnh đã đạt. Các kết quả Python
ở trên thuộc checkpoint gốc, chưa được chạy lại trong worktree PR này.

- Với quyền còn hiệu lực, response/body thành công giữ nguyên contract.
  Với quyền đã thu hồi, baseline cũ trả cache được thay bằng 401/403/409;
  không sửa golden để hợp thức hoá baseline bảo mật sai.
- SQL trace read mark đổi từ SELECT/INSERT/UPDATE sang UPSERT RETURNING;
  đó là thay đổi chủ ý. Kết quả tuần tự và thứ tự tuple giữ nguyên.
- Chưa chứng minh load, native Android/iOS, MLS, realtime nhiều replica,
  media/voice, bot consent hoặc toàn bộ cutover. Các bản vá không phải bằng
  chứng chat đã E2EE hoặc production-ready. Chưa deploy hoặc merge.
