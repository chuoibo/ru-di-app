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

## Checkpoint 22-09-2026 — bốn PR lên main, tầng E2E và tờ hẹn chung

Bốn lát đầu của chuỗi đã lên `main`, mỗi lát kèm lệnh đã chạy và phạm vi
chứng minh. Ghi ở đây để lượt sau đọc delta thay vì chạy lại từ đầu.

| PR | Nội dung | Bằng chứng |
|---|---|---|
| #624 | Replay authorization, read watermark nguyên tử | 16/16 check CI |
| #626 | Nền realtime Go v2: store, transport, admission, relay | `go_postgres_tier.sh` **1960 ca PASS**, 0 SKIP |
| #627 | Feed thay đổi bền + AI chỉ chạy khi được gọi rõ | `go_postgres_tier.sh` **1985 ca PASS**, 0 SKIP |
| #628 | Tầng E2E chat qua HTTP/WebSocket thật vào cửa trước Go | **31 ca PASS**, sentinel có mặt, 0 SKIP |
| #629 | Tờ hẹn nháp chung + kịch bản parity nhóm `messages` | **40 ca PASS**; parity dev **10.484 bước, 0 DIFF** |

### Tầng E2E: nó đóng lỗ nào

Mọi test Go khác dừng ở biên gói. Tầng `services/core/e2e/chat` chỉ biết một
base URL và một bearer, nên nó là chỗ duy nhất chứng minh được rằng một yêu
cầu đi qua dây, router, middleware phiên, lớp idempotency và quyết định proxy
vẫn ra cùng câu trả lời. Sentinel đòi một route **chỉ Go mới có**
(`/contexts/{id}/changes`) trả 200, nên tầng biết nó vừa ghi lại hành vi của
tiến trình nào — không phải của Python qua proxy.

Một câu hỏi còn treo trong bàn giao đã được trả lời bằng đo: bàn giao ghi
«phản ứng, xoá tin và kiểm phiếu chưa đồng bộ sang client khác» như blocker
thực nghiệm. Cả bốn đều tới. Feed mang **con trỏ** chứ không mang nội dung,
nên ai đọc thân câu trả lời của socket sẽ tưởng là rỗng.

### Điều chưa đạt, không được đọc thành đã xong

- **Tải vẫn trượt cổng ADR-0031.** p95 2427 ms so với ngưỡng 800 ms ở 1.000
  kết nối; burst 300/s còn FAIL; chưa có soak 24h.
- **Nhóm `messages` vẫn `PORTED-UNPROVEN`.** Đã có bốn kịch bản parity đầu
  tiên (`parity/scenarios/wai/`, trước đó thư mục này không tồn tại), nhưng
  lật nhãn cần một lượt cổng đầy đủ trên SHA sạch và sẽ đi riêng.
- **Chat v2 vẫn chỉ mở trong `chat-lab` loopback.** App đang chạy trên lane
  legacy có nhãn «Chưa mã hoá đầu cuối».
- **`packages/chat-crypto` chưa có cổng CI nào.**
- Native Android/iOS và crypto review độc lập vẫn là cổng riêng.
