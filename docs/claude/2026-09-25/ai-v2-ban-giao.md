# Bàn giao engine AI v2 — trạng thái thật, 2026-09-25

Nhánh `claude/peaceful-hopper-32kwjs`. Người làm tiếp đọc file này trước bảng lát ở
`docs/architecture/03-ai-engine-hop-dong.md` §4. Không có câu nào dưới đây đo bằng model thật: container
làm việc không có `GEMINI_API_KEY`, mọi test chạy stub. Dấu xanh chứng minh đường ống, **không** chứng
minh chất lượng câu trả lời.

## 1. Câu hỏi của chủ sản phẩm, trả lời thẳng

| Hạng mục | Đã có chưa | Có gì thật trên nhánh |
|---|---|---|
| **Agentic RAG** | **Chưa** | Chỉ có tầng truy hồi (gói `services/core/internal/rag`): lọc cứng bằng SQL (dị ứng, giờ mở cửa, ngân sách, ăn kiêng, điểm đến) + xếp hạng từ vựng RRF(tsvector, trigram, từ vựng), chỉ mục có phiên bản. Chưa có vòng agent dùng truy hồi làm tool, chưa có LLM chấm lại, chưa có vòng sửa, chưa có `CheckAnswer`. Engine chưa gọi `rag.Retrieve`; nhóm vẫn đi Python brain với `ModelPlaceRows`. |
| **Vector database** | **Chưa** | Không có embedding, không có pgvector. Thiết kế 04 chọn pgvector trong Postgres (lát 16), cần ảnh Postgres có pgvector và khoá cho `gemini-embedding-001`. |
| **SDLC nạp dữ liệu** | **Một phần** | Vòng đời chỉ mục từ vựng: chunker `place.v1` có băm nội dung, `SafeDeep` (cách ly trường có câu chèn lệnh), build → eval (probe lọc cứng) → promote/rollback dưới advisory lock, tombstone ngoài phiên bản, CLI `core rag build|eval|promote|rollback|status|tombstone|untombstone`, service compose `migrate-rag`. Chưa có: làm giàu bằng LLM, dedupe, embed, độ tươi (trigger dirty + indexer), giám sát. |
| **Trí nhớ (mem0 / Mongo)** | **Chưa, và không dùng hai thứ đó** | Không có trí nhớ dài hạn nào. Thiết kế (lát 15) để trí nhớ Nếp ở bảng Postgres do Go ghi (`nep_su_that`, xoá cứng khi «quên»). Luật repo: Postgres là nguồn sự thật, backend nghiệp vụ là Go; thêm Mongo hay mem0 (thư viện Python) cần ADR trước. |
| **Redis** | **Có, nhưng không cho RAG/trí nhớ** | Redis Streams cho token stream (`internal/aistream`), pub/sub đánh thức, limiter GCRA theo lời gọi model. |
| **Hàng đợi** | **Có** | Outbox không payload → RabbitMQ có publisher confirm, quorum queue + DLQ, consumer, poller dự phòng, tác vụ định kỳ (`internal/jobs`, nối vào `chatassist`). Review phản biện: APPROVE. |
| **Pipeline agent chuẩn** | **Một phần nhỏ** | Nếp qua ADK-Go: preprocess → guard tất định → **một** lời gọi model (llmagent không tool) → output guard. Chưa có bước Understand (LLM trích ý định + slot), chưa có function calling, chưa có vòng nhiều bước, chưa có trí nhớ. Chạy sau cờ `MOBILE_AI_ENGINE_NEP`, mặc định vẫn `brain`. |
| **Streaming SSE** | **Chưa xong** | Gói `aistream` (ghi/đọc Redis Streams, SSE, resume) và client `apps/mobile/src/rudi/ai/sse.ts` có; writer trong worker, cửa sổ 48 ký tự, route `/events` đang làm dở ở lát 11 (chưa nằm trên nhánh). UI chưa làm. |

## 2. Phần heuristic — bàn giao, không làm tiếp theo hướng này

Hai bộ luật từ khoá/regex được viết như **lưới an toàn** cạnh LLM, nhưng đã thành việc chính qua 5 vòng
review mỗi bộ. Kết quả đúng kiểu thất bại của heuristic: vá lớp này sinh lỗi lớp khác.

- **Luật tiền của Nếp** — `services/core/internal/aiharness/guard/tien.go` (từ chối việc tiền với 0 lời gọi
  model). Đo trên corpus niêm phong v3 (260 câu tiền, 245 câu không phải tiền): recall 221/260 = 0,850, bắt
  nhầm 5/245 ở `b624ae1`. Recall đã **đóng băng** trong đề xuất ADR-0037 §4: recall là việc của bộ phân loại
  LLM (Understand), luật chỉ còn là lưới.
- **Bộ đọc dị ứng người hỏi** — `services/core/internal/domain/tuvung/nguoi_hoi.go` (đọc «mình dị ứng X» để làm
  bộ lọc cứng). Trên nhánh là bản vòng 3 (`8808fa4`): corpus niêm phong v3 đọc đủ 258/266 câu. Bản vòng 4
  (`24e35d2`) **không gộp**: review tìm 4 hồi quy làm mất dị ứng/ăn kiêng người dùng đã nói.
- **Output guard** — `guard/output.go` (chặn số điện thoại, số tài khoản, email, câu tự nhận đã chuyển tiền):
  hậu kiểm an toàn, không phải hiểu câu. Commit cuối `6ff8be3` sửa hồi quy «Mình đã gửi quán 200k tiền cọc»
  lọt qua; **chưa có review phản biện**.

Hướng đúng (thiết kế 01 §3.3, 04 §5): bước Understand một lời gọi flash-lite trả JSON có schema (ý định, khu
vực, thời gian, ngân sách, dị ứng, ăn kiêng, loại việc tiền). Bộ lọc cứng lấy **hợp** slot LLM ∪ lưới từ
khoá; từ chối việc tiền = bộ phân loại LLM ∪ regex. Đo chất lượng cần khoá và ngân sách lời gọi do Lead duyệt.

Corpus đã dùng để đo (câu bịa, không dữ liệu thật), giữ để làm test hồi quy — **đã lộ, không còn là niêm
phong**, muốn đo mù phải viết bộ mới:

- `services/core/internal/aiharness/guard/testdata/da_lo/tien_niem_phong_v2.json`, `tien_niem_phong_v3.json`
- `services/core/internal/domain/tuvung/testdata/da_lo/di_ung_niem_phong_v2.json`, `di_ung_niem_phong_v3.json`

## 3. Đã xong và có review phản biện APPROVE

- Lát 7 — Rủ Đi AI trả lời trong luồng (tin `@Rủ Đi` là tin thường, trả lời là reply, chip ngữ cảnh); không vào
  `main` trước khi Lead ký ADR-0039; chưa chạy Maestro 49, chưa mở ảnh chụp.
- Lát 10 — hàng đợi phía máy chủ.
- Sổ tay app cho Nếp (`internal/huongdan`: `TheoMan`, `Tim`, `DuongToi`, `BanDung`) + cổng lệch mã.
- Hạ tầng: Go 1.25.14, tách worker (`core work`), cổng đọc xuyên gói `aigate`, tầng test broker, eval T1 (stub).

## 4. Chưa chạy được ở máy này

`make parity`, compose (`worker`, `rabbitmq`, `redis`), tầng Postgres đầy đủ bằng Docker, Maestro/ảnh chụp,
mọi lời gọi model thật. ADR-0037…0042 vẫn là đề xuất chờ Lead ký.
