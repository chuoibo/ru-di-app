# Ghi chú: người dùng muốn đổi cơ chế AI chat (25-09)

Phiên làm việc ngày 2026-09-25. Commit gốc: `f251db7`. protocol_version: không áp dụng (không
phải lượt thí nghiệm). Verdict: không có reviewer người; thiết kế qua hai lượt phản biện độc lập
(ràng buộc/riêng tư và khả thi/thứ tự) trước khi người dùng duyệt plan.

Ghi chú này ghi lại **cái người dùng yêu cầu**, **cái đã chốt**, và **cái không làm kèm lý do**.
Thiết kế chi tiết từng mảng nằm ở `docs/claude/2026-09-25/thiet-ke-ai/`; hợp đồng chung giữa
các mảng ở `docs/architecture/03-ai-engine-hop-dong.md`; đề xuất ADR ở
`docs/decisions/proposals/ADR-0037…0042`.

## 1. Hiện trạng đã đọc lại từ code (trước khi đổi)

- **Rủ Đi AI trong nhóm.**
  - Gõ `/plan`, `/chia-bill` hay `@Rủ Đi` thì tin **không được gửi**: `goiMoHinh` chặn lại và mở khay
    «Phác một tờ hẹn» (`apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx:85-87, 358-366`).
  - Người dùng xác nhận trong khay. Máy chủ xếp hàng một lời gọi, worker gọi model **một lần**, rồi
    đăng một tin `ai_card` **không tác giả, không reply** vào phòng
    (`services/core/internal/chatassist/worker.go`, `publish`).
  - Client hỏi lại mỗi 2 giây.
  - Không có lệnh «nói chuyện» riêng.
- **Nếp** là trợ lý riêng nổi ở mép phải.
  - Nếp chỉ nhận một phiếu khoá đóng gồm tên màn, tiêu đề và vài số đếm, cùng các lượt của phiên đang mở.
  - Máy chủ không đắp thêm gì; kết quả trả kín về đúng người hỏi.
  - Nếp không biết app chạy thế nào, không nhớ gì qua phiên, không tự nhắc.
- **Chung cho cả hai.**
  - Không có system instruction, không cho model biết «bây giờ» ở giờ Việt Nam.
  - Không có tool, RAG hay streaming.
  - Eval nhóm đạt 10/16 (`services/api/tests/skills/tra_loi_trong_nhom.py`).

## 2. Người dùng yêu cầu

### 2.1 Cơ chế (thứ nhất)

1. **Tag AI trong nhóm phải giống Meta AI trong Messenger.**
   - Tin `@Rủ Đi …` hiện trong luồng như tin thường.
   - AI trả lời ngay trong luồng như một thành viên, không bật ra một khay hay thẻ riêng.
2. **Câu trả lời thường phải stream bằng SSE**, chữ chạy live cho người dùng xem thời gian thực.
   Không đợi xong cả cục mới hiện.
3. **Trước khi token đầu tiên ra phải có animation «đang suy nghĩ» thật đẹp.** Phía Nếp có một icon Nếp
   hiện ra với animation loading, giống icon Meta AI.
4. **UI bot phải đẹp, có thẩm mỹ**: bong bóng chat, hiệu ứng. Làm frontend bằng skill `/impeccable`.

### 2.2 Chất lượng (thứ hai, trọng tâm)

5. Dựng một **hệ chatbot chuẩn chỉnh**:
   - guard;
   - query transformation;
   - multi-intent;
   - RAG, dùng **agentic RAG** để không bịa;
   - trí nhớ ngắn hạn và dài hạn;
   - tiền xử lý và hậu xử lý;
   - function calling.
6. Điều người dùng coi trọng nhất:
   - **cơ chế agentic RAG nào là tốt nhất**;
   - **SDLC nạp dữ liệu** vào RAG như thế nào.
7. **Chạy bất đồng bộ kiểu ChatGPT.** Gọi xong nhận task id, đi làm việc khác, không chặn luồng chính,
   xong thì tự lấy kết quả theo task id. Người dùng nêu RabbitMQ, Celery, Redis.
8. **Nếp hỗ trợ đúng lúc, đúng chỗ.**
   - Biết người dùng đang ở màn nào và màn đó đang hiện gì.
   - Đọc được luồng logic của app để hướng dẫn bằng ngôn ngữ tự nhiên, ví dụ «muốn chọn A B C thì làm sao».
   - Cá nhân hoá: nhớ hành động và thói quen.
   - Tự bắn thông báo gợi ý đúng lúc.

## 3. Đã chốt với người dùng (qua câu hỏi trong phiên)

| Chủ đề | Chốt |
|---|---|
| Ngữ cảnh khi tag | Tin tag + N tin gần. Có một **chip xem trước gọn** ngay trên nút gửi: «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ». Giữ nguyên ADR-0036 §2.5 |
| Ai thấy chữ chạy trong nhóm | Lane cũ (chưa E2EE): **cả phòng thấy**. Phòng E2EE v2: chỉ người gọi thấy stream, cả phòng thấy «Rủ Đi AI đang trả lời…», máy người gọi mã hoá câu trả lời rồi đăng. Chấp nhận hai hành vi |
| Hàng đợi | **Go + RabbitMQ + Redis**, Postgres vẫn là nguồn sự thật. **Không Celery** |
| Model | Giữ **`gemini-3.5-flash-lite`** cho mọi bước sinh chữ. Được dùng framework agentic |
| Nơi chạy agent | **ADK-Go** (`google.golang.org/adk` v1.7.0, cần Go 1.25) gọi Gemini **trực tiếp từ Go** |
| Animation Nếp | Sửa `DESIGN.md` có ghi lý do, cho animation «đang nghĩ» theo **ngôn ngữ giấy**, thiết kế bằng `/impeccable` |
| Trí nhớ Nếp | **Lặng lẽ nhưng có công bố** (mục 4) |

## 4. Không làm, và vì sao

- **Celery.** CLAUDE.md và ADR-0031 cấm worker và điều phối bot mới bằng Python. Celery sẽ tạo writer
  thứ hai cho cùng một module. Mọi thứ người dùng muốn từ Celery (task id, không chặn, lấy kết quả
  sau) đều làm được bằng outbox Postgres → RabbitMQ → worker Go.
- **Trí nhớ ẩn hoàn toàn.** Người dùng từng muốn Nếp ghi nhớ mà «không cho user biết». Bản đó không
  thiết kế, vì ba lý do:
  - **Luật:** Nghị định 13/2023 và Luật Bảo vệ dữ liệu cá nhân 2025 (hiệu lực 01/01/2026) đòi thông báo
    và đồng ý khi dùng dữ liệu để lập hồ sơ hành vi.
  - **Chợ ứng dụng:** App Store và Google Play bắt khai dữ liệu dùng cho cá nhân hoá.
  - **Luật của repo:** ADR-0036 §2.5 cấm đơn phương rút một lời hứa riêng tư đã nói ra.

  Người dùng chọn bản **lặng lẽ, có công bố**:
  - Báo **một lần** lúc bật, kèm **một công tắc** trong Cài đặt, mặc định tắt.
  - **Không** có trang ký ức, **không** có toast.
  - Nếp tự thêm, sửa, thay và cho hết hạn ký ức theo mốc thời gian.
  - Chỉ nhớ hành vi trong app và điều người dùng dặn. Không nhớ chat nhóm, tiền, hay đặc điểm nhạy cảm.
  - Hỏi «bạn nhớ gì về mình?» thì Nếp kể thật. Nói «quên chuyện đó đi» thì Nếp xoá cứng.
  - Tắt công tắc hoặc xoá tài khoản thì xoá sạch.
  - Nhắc chủ động cũng opt-in: có trần, có giờ yên, không bao giờ ở màn tiền.
- **Vẽ hình Nếp lên thẻ trả lời trong nhóm.** Giữ ADR-0036 §2.6: trong nhóm là «Rủ Đi AI» với chữ ký
  sparkles; icon Nếp động chỉ có trong bảng Nếp.

## 5. Hai điểm plan lệch khỏi lời người dùng (đã nêu, người dùng duyệt plan)

- **Người xem khác trong phòng nhận chữ chạy qua WebSocket đang mở sẵn**
  (`chatlegacychange`, thêm một loại frame), không qua SSE thứ hai. Người gọi và Nếp dùng SSE.
- **Chữ chạy chậm hơn model khoảng 48 ký tự.** Đây là cửa sổ output guard kiểm trước khi byte nào tới
  cả phòng. Không có «rút lại» sau khi đã lộ.

## 6. Cái còn mở

Xem mục «Việc cần người/Lead quyết» trong hợp đồng chung. Nổi bật:
- Lead ký ADR-0037…0042 và phần sửa `DESIGN.md`.
- Ngân sách lời gọi model thật cho từng lượt đo.
- Host production cho RabbitMQ và Redis; FCM credentials cho push.
- Skill `/impeccable` không có trong session cloud này, nên các lát UI chạy ở nơi có skill.

## 7. Bằng chứng đã xem

- Đọc code qua 6 agent chỉ đọc: engine, Nếp, realtime/E2EE, dữ liệu cho RAG, brain Python, luật/ADR.
  Mỗi agent trích file:line; vài điểm đã kiểm lại bằng tay:
  - `ai-invocations` vắng trong `services/core/ownership/routes.json`;
  - nút «Vẽ» không xét `duocHoi`;
  - `qa-nep.ts` nói chưa có gì gửi việc cho dock.
- Kiểm module Go trên proxy:
  - `google.golang.org/adk` v1.7.0 đòi `go 1.25.0`;
  - `google.golang.org/genai` v1.71.0;
  - `github.com/rabbitmq/amqp091-go` v1.15.0;
  - `github.com/pgvector/pgvector-go` v0.4.1.
- `services/core/Dockerfile` đang ở `golang:1.23.4-bookworm`.
