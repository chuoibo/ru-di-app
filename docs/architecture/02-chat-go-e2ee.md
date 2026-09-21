# Chương trình chat Go/E2EE — tiến độ và cổng

Ngày bắt đầu: 2026-09-21. Quyết định: ADR-0031. Nhánh triển khai:
`codex/p0-w28-chat-go-e2ee`. Không phải bằng chứng phát hành.

## Hiện trạng đã khảo sát

- Baseline `24bc70f8`: 131 route decorator, 36 migration legacy.
- Chat poll 4 giây, không có durable mutation feed; thay đổi reaction/xoá
  tin cũ chưa đến client khác đang mở bằng forward poll.
- HTTP fake probe: replay Idempotency-Key trả body cũ sau khi revoke membership.
- Race timestamp-before-commit và read mark cần tái hiện trên PostgreSQL thật.
- Voice chưa có trong wire; Android đang chặn RECORD_AUDIO.
- 147 test API/domain qua trong khảo sát; không chứng minh native/concurrency.
- Chưa xác nhận iOS runner, máy thật, push credentials hoặc crypto review.

## Cập nhật checkout lúc triển khai

Trong lúc bắt đầu, checkout được phiên khác chuyển sang `534c0fd1` (PR #617).
Go core, ownership manifest, domain parity và adapters đã tồn tại. Tiếp tục
trên baseline đó; không ghi đè chúng. Các phát hiện ở `24bc70f8` phải được
tái kiểm trên Go hiện tại. Stack dùng chung thuộc worktree khác, không sửa.
Postgres kiểm thử của lát chat chạy riêng, không dùng dữ liệu app.

## Các lát và định nghĩa xong

| Lát | Sản phẩm cần giao | Trạng thái |
|---|---|---|
| A | Global rule, ADR, inventory contract, regression lỗi hiện tại | Đang làm |
| B | Go foundation, allocator/domain parity, PostgreSQL migration/repository | Chưa xong |
| C | Auth/session và toàn bộ API nghiệp vụ Go; Python chỉ AI | Chưa xong |
| D | MLS native spike, identity/storage/recovery, crypto review | Chưa xong |
| E | Durable realtime, media, AI jobs/grants và mobile integration | Chưa xong |
| F | UI Impeccable, native E2E, load/chaos, thử dùng thực tế | Chưa xong |
| G | Cutover, rollback tương thích E2EE và APPROVE độc lập | Chưa xong |

Không đánh dấu toàn chương trình xong khi chỉ có nền hoặc test fake. Không
bật route E2EE production trước gate native/crypto. Chuyển module từng bước
trong cùng chương trình; deployment cũ tiếp tục phục vụ tới cutover được kiểm.

## Contract triển khai

Go module tại `services/core`, module name `mobile/services/core`; domain thuần, transport
không tự quyết permission. Persistence PostgreSQL qua pgx. Test database riêng,
schema riêng được xoá đúng phạm vi. Client protocol v2 tách khỏi plaintext v1.
API/worker không log token, body, ciphertext, URL ký hay nội dung AI.

Từng checkpoint phải bổ sung lệnh đã chạy, kết quả, phạm vi chứng minh và
blocker còn lại vào tài liệu này hoặc báo cáo được liên kết. Commit có guard,
staging theo đường dẫn; không gom các artifact không liên quan trong worktree.
