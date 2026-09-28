# Nghiên cứu stack agentic — tổng hợp, 2026-09-25

Mười chủ đề, mỗi chủ đề một agent nghiên cứu (nguồn chính chủ: tài liệu, mã nguồn, bài báo, model card)
và một agent kiểm chứng độc lập (lấy lại các khẳng định quan trọng nhất từ nguồn chính chủ; phần sửa của
người kiểm chứng thắng). Báo cáo đầy đủ kèm đường dẫn nguồn nằm ngoài repo, vì chứa mã arXiv, digest và
kích thước tệp mà repo guard chặn; file này chỉ giữ kết luận. Chưa có câu nào đo bằng model thật.

Ràng buộc nền: Go là backend nghiệp vụ, Python chỉ suy luận; Postgres là nguồn sự thật; mọi bước sinh
chữ dùng `gemini-3.5-flash-lite`; trần 8 lời gọi model mỗi lượt (mục tiêu p95 ≤4); **luật chủ sản phẩm:
không heuristic, không lọc từ khoá ở bước hiểu câu và quyết định** — model quyết, Go chỉ kiểm cấu trúc,
ngân sách, quyền, áp ràng buộc cứng model đã trích, kiểm bám bằng chứng bằng phép so tập.

## 1. Chatbot chuyên nghiệp định tuyến ý định thế nào (intent routing)

Đã xem: Intercom Fin, Sierra, Decagon, Rasa CALM, Salesforce Agentforce, Google Conversational Agents,
Amazon Bedrock Agents, Copilot Studio, OpenAI Agents SDK, Anthropic «Building effective agents»,
Google ADK, LangGraph, semantic-router, NeMo Guardrails, Meta AI trong nhóm chat.

Điểm chung của các hệ production: **một bước hiểu câu bằng LLM ra cấu trúc** (lệnh / chủ đề / ý định có
tập đóng), còn nghiệp vụ và quyền do mã tất định giữ. Rasa CALM gọi đó là «command generator», Agentforce
là «topic classification», Fin có bước «refine query» trước truy hồi.

Chọn cho Rủ Đi:
- **Một lời gọi router có structured output mỗi lượt** (thinking MINIMAL, JSON schema với enum đóng):
  1–3 ý định, slot nguyên văn + id, nhãn guard, loại việc tiền, hướng đi, câu hỏi con viết lại, có cần hỏi
  lại không. LLM zero-shot ngang bộ phân loại huấn luyện riêng trên tập 150 ý định, nên chưa cần train.
- **Không** định tuyến bằng chuyển agent (`transfer_to_agent`, supervisor): mỗi bước chuyển tốn thêm lời
  gọi trong trần 8.
- Định tuyến phẳng (Nếp 8 ý định, nhóm 5); tầng «miền» là kênh gọi (màn Nếp hay `@Rủ Đi` trong nhóm) —
  cái này tất định vì là cấu trúc, không phải đọc chữ.
- Schema của nhóm **không có trường trí nhớ**: nhóm không thể định tuyến vào trí nhớ riêng.
- `OutputSchema` của ADK-Go không biểu diễn được `$defs/$ref`: router gọi genai trực tiếp với
  `ResponseJsonSchema`, hoặc giữ schema phẳng.

## 2. Agentic RAG và khi nào gọi tool

- Router quyết **hướng**: trả lời thẳng · truy hồi một bước · vòng agent · hỏi lại · ngoài phạm vi.
  Đường một bước: Go chạy truy hồi lai → rerank → lọc cứng từ câu hỏi con của router, rồi một lời gọi trả
  lời (2 lời gọi). Đường agent: tối đa 6 (router + ≤4 bước, Nếp ≤3, + ≤1 sửa).
- Function calling: mode **VALIDATED** ở bước giữa (model được chọn gọi tool hoặc trả lời, tham số bị
  ràng theo schema), **NONE** ở bước cuối; không dùng ANY trong vòng. Đặt ThinkingLevel tường minh, không
  kèm ThinkingBudget (Gemini 3 trả 400).
- ADK-Go v1.7.0 **không tự chặn** số lời gọi model hay tool ngoài chế độ live và chạy các function call
  song song không giới hạn: trần phải đặt trong callback. Đếm tool và gộp ràng buộc cứng (chặt hơn thắng)
  trong **một** lớp callback — plugin trả kết quả thì callback của agent bị bỏ qua.
- Mô tả tool là yếu tố quan trọng nhất cho việc chọn đúng: 3–5 câu, nói làm gì, khi nào dùng, khi nào
  **không** dùng, trả về gì. Lỗi tool trả về có cấu trúc để model tự sửa đúng một lần.
- Tài liệu: agent gọi tool thừa hơn 30% số lần (SMART); model lớn hay trả lời sai thay vì từ chối khi bằng
  chứng thiếu (Sufficient Context, Google). Đo bằng kiểu When2Call: 4 lớp quyết định (trả lời, gọi tool,
  hỏi lại, không làm được), macro-F1 và tỉ lệ bịa tool.
- Self-RAG cần model đã fine-tune; FLARE, DRAGIN, SeaKR cần xác suất token hay trạng thái ẩn — không
  dùng được qua API Gemini.

## 3. Reflection và kiểm chứng câu trả lời

- **Kiểm tất định trước (0 lời gọi)**: id quán trích dẫn ⊆ sổ cái của lượt (model chỉ thấy bí danh p1,
  p2…), ràng buộc cứng kiểm lại trên hàng `places` thật, nhãn nút phải có trong đoạn sổ tay đã trả về.
- **Không để model viết giá, giờ, địa chỉ**: giao diện render từ sổ cái trên thẻ, nên kiểm số chỉ còn là
  luật cấm.
- Đường stream «chọn trước»: một lời gọi JSON stream trả `chon[]` trước `van_ban`
  (đặt `propertyOrdering`); Go kiểm `chon` trước khi phát delta đầu tiên, sai thì `lam_lai`.
- Tự sửa **không có phản hồi ngoài** thường không giúp (các nghiên cứu về «LLM chưa tự sửa được suy
  luận»); phản hồi từ công cụ/bằng chứng thì giúp. Verifier LLM chỉ dùng khi phép kiểm tất định không kết
  luận được; không có bằng chứng thì **không bao giờ** là «đạt» (NeMo mặc định fail-open — không chép).
- Đủ bằng chứng hay chưa (CRAG) nên tính từ điểm reranker, số ứng viên qua lọc cứng và độ phủ ràng
  buộc, hiệu chuẩn ngưỡng trên bộ gán nhãn tiếng Việt; tối đa một vòng sửa.
- API check-grounding của Vertex cần gói Go yêu cầu Go 1.26 ở bản mới nhất, và bộ chấm HHEM không có
  tiếng Việt: chưa dùng.

## 4. Milvus

- **Milvus v3.0.2 standalone**: etcd nhúng, lưu trữ cục bộ, WAL woodpecker (phải đặt
  `woodpecker.storage.type=local` tường minh vì mặc định là minio), bật auth. Không dùng file compose hay
  `standalone_embed.sh` gốc vì ở tag v3.0.2 chúng vẫn trỏ v3.0.1 — tự viết, ghim digest.
- Go SDK `github.com/milvus-io/milvus/client/v3`; **phải** truyền `&TelemetryConfig{Enabled:false}`
  (nil nghĩa là bật). Filter chỉ qua template param, không nối chuỗi.
- Embedding làm trong Go, sparse ở dịch vụ suy luận; Milvus chỉ lưu và tìm. Rerank gọi từ Go, không qua
  Model Ranker của Milvus.
- Mỗi phiên bản chỉ mục là một collection vật lý sau alias; promote = đổi alias dưới advisory lock rồi
  kiểm lại alias; rollback = đổi ngược.
- Collection trí nhớ: `owner_id` là partition key, **không có trường chữ**.
- Xoá trong Milvus là xoá logic tới khi compaction; «quên» = xoá cứng ở Postgres/Redis ngay, không còn
  thấy trong Milvus ≤60 s, biến mất vật lý ≤24 h (compaction tay + GC). Snapshot của backup có thể giữ
  segment tới 7 ngày: phải tính vào lời hứa «quên».
- Milvus Lite 3.x là bản Python viết lại, không auth, một writer: chỉ để thử nhanh, không thay server.

## 5. Gemini Embedding 2 (dense)

- Model `gemini-embedding-2` (GA); không trộn vector với bản preview hay `gemini-embedding-001`.
- **1536 chiều** cho mọi collection, chuẩn hoá L2 trong Go; đo lại 768/1536/3072 trên bộ tiếng Việt.
- Trên Developer API: phân biệt tài liệu/câu hỏi bằng tiền tố trong chữ (`title: … | text: …` và
  `task: search result | query: …`), không đặt TaskType; trên Vertex thì A/B tiền tố với task_type. Test
  hợp đồng dây trên body request, kiểm `len(values)==dims`.
- Không có tác vụ truy hồi tiếng Việt trong MMTEB công bố: phải tự đo.
- Nạp lớn/đổi model: Batch API (giá một nửa), chia lô ≤100, 429 thì lùi có jitter.

## 6. MILCO (sparse)

- Mô hình sparse đa ngôn ngữ ~560M, ICLR 2026, ánh xạ mọi ngôn ngữ vào không gian từ vựng tiếng Anh.
- **Chặn trước production: giấy phép chưa rõ.** MILCO nhúng đầu splade-v3 mà trọng số mang
  CC BY-NC-SA (không thương mại). Người có quyền vào HuggingFace phải đọc model card của cả hai.
- Chưa có số đo tiếng Việt công bố cho truy hồi: phải lập bộ ≥200 câu (có dấu, không dấu, teencode, lẫn
  tiếng Anh, tên riêng) và so Postgres lexical / BM25 Milvus / MILCO / dense / hybrid.
- Nếu dùng: ghim revision, tải offline, `max_length` 64 cho câu hỏi và 256 cho tài liệu, cắt tài liệu còn
  ~128 term, dùng đầu ra dạng dict đã gộp.
- Dự phòng mặc định: hàm BM25 của Milvus với tokenizer standard + lowercase + asciifolding trên chữ NFC
  (asciifolding đã gập đ→d và nguyên âm có dấu). Đây là hàm chấm điểm truy hồi, không phải luật quyết
  định.

## 7. Qwen reranker

- **Qwen3-Reranker-0.6B GGUF Q8_0 trên llama-server** cho CPU bây giờ, ghim sha256; khi có GPU chuyển
  4B trên vLLM (8B không hơn 4B trên MMTEB-R).
- Gọi `/rerank` (không phải `/v1/rerank`); chỉ đọc `index` và `relevance_score`, kiểm chỉ số nằm trong
  khoảng và không trùng; không log body.
- llama.cpp có lỗi đang mở về điểm rerank sai ở nhiều model: **bắt buộc** kiểm golden điểm của server so
  với bản tham chiếu cho đúng build đã ghim.
- Hạn giờ riêng (CPU 1–3 s cho lô nhỏ), không thử lại trong lượt, cầu dao; lỗi thì giữ thứ tự RRF.

## 8. mem0 (trí nhớ dài của Nếp)

- **mem0ai 2.2.1** (không phải 2.2.0), chỉ dùng như **thư viện** trong một sidecar FastAPI do ta sở hữu
  hợp đồng REST, chỉ Go gọi. Không chạy server hay OpenMemory của mem0.
- Mặc định mem0 **gửi telemetry** và **lưu nguyên văn 10 tin gần nhất cùng chữ ký ức cũ trong SQLite**,
  kể cả sau khi xoá. Phải: `MEM0_TELEMETRY=false` trước khi import + test đỏ nếu posthog được dựng; thay
  SQLiteManager bằng bản rỗng; không cài spaCy.
- `delete_all` có thể dừng sớm mà vẫn báo thành công: saga «quên» của Go phải đếm lại số hàng.
- Postgres giữ đồng ý và biên nhận xoá; Milvus/mem0 là chỉ mục dẫn xuất, dựng lại được.

## 9. Trí nhớ ngắn hạn trên Redis và cá nhân hoá

- Không dùng RedisVL, checkpointer LangGraph hay agent-memory-server: viết gói Go nhỏ (một writer), khoá
  mã hoá, TTL đặt nguyên tử.
- Nếp: client vẫn là nguồn của phiên và gửi lại các lượt; Redis chỉ giữ bộ đệm theo lượt (EX 300), xoá
  khi job xong. Prompt: sổ tay + phiếu màn hình + ≤5 ký ức đã rerank (chỉ khi «Nếp nhớ» bật) + ~12 lượt.
  Không tóm tắt cuộn bằng LLM (tốn lời gọi).
- Nhóm: chỉ gói ngữ cảnh người gọi gửi + chuỗi trả lời vào thẻ AI; lane cũ đệm tối đa EX 900; phòng E2EE
  v2 không ghi chữ nào vào Redis.
- Redis riêng cho AI: `save ""`, `appendonly no`, không replica, `volatile-ttl`, ACL chỉ các mẫu khoá cần.
- Cá nhân hoá kiểu ChatGPT/Gemini: bật tắt rõ, công bố, «bạn nhớ gì» nói thật, «quên» xoá thật; tránh
  suy diễn đặc điểm nhạy cảm. Pháp lý: Luật Bảo vệ dữ liệu cá nhân 2025 và Nghị định 356/2025 (thay Nghị
  định 13/2023) — cần người đọc pháp lý.
- Đo: kiểu LongMemEval / LoCoMo (nhớ, cập nhật, quên, rò rỉ), lập bộ nhỏ tiếng Việt.

## 10. SDLC production

- Postgres là nguồn sự thật, Go là writer duy nhất của mọi collection. Bắt thay đổi bằng trigger
  `rag_dirty` và outbox của gói `jobs`. Khoá chính tất định (băm của doc, facet, phiên bản chunker), mọi
  bước idempotent theo băm nội dung, cache embedding theo (băm, model, số chiều, tác vụ).
- Cổng promote offline: recall@10 ≥0,90, nDCG@10 ≥0,75, MRR@10 ≥0,70, vi phạm ràng buộc cứng = 0,
  chênh lệch có dấu/không dấu ≤0,05, không kém bản đang chạy (bootstrap cặp), có ablation dense / sparse /
  hybrid / rerank.
- Đánh giá nhiều lớp: eval tự động, giám sát production, A/B, phản hồi người dùng (👍/👎 không nội dung),
  review tay, nghiên cứu với người thật (khớp cổng «người dùng thực» riêng trong CLAUDE.md).
- Cổng cấm cài OTel provider toàn cục đã có trong `internal/aigate/otel_gate_test.go`: mở rộng, không
  viết lại.

## Việc cần người quyết

1. Đọc giấy phép MILCO và splade-v3 trên HuggingFace (và mở host `huggingface.co` trong network policy
   nếu muốn tải trọng số ở máy làm việc).
2. Duyệt ADR hạ tầng mới (Milvus + mem0 sidecar + dịch vụ suy luận Python + Redis riêng cho AI).
3. Ngân sách lời gọi thật để đo router, embedding, rerank và trí nhớ trên bộ tiếng Việt.
4. Đọc pháp lý cho trí nhớ cá nhân hoá.
