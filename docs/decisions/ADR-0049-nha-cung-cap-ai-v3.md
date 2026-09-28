# ADR-0049 — Nhà cung cấp AI v3: Go sinh chữ qua agy-proxy, Python chỉ còn sidecar OpenRouter, Milvus GPU

- Ngày: 2026-09-28.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-09-28** trong phiên lập kế hoạch (câu trả lời có ghi lại
  ở kế hoạch đã duyệt). Không phải chữ ký Lead: ghi rõ để không ai đọc thành Lead đã ký.
- Thay một số điều khoản của ADR-0043, ADR-0044, ADR-0047, liệt kê ở mục 5; **không sửa bản lịch sử** của ADR nào.
- Không đổi bởi văn bản này: ba luật tiền; chat v2 E2EE (phòng v2 vẫn trả 409 `encrypted_invocation_required`);
  AI chỉ nhận nội dung được gọi/chia sẻ rõ; ADR-0034 và ADR-0048 (gu, đồng ý); câu từ chối cố định
  (ADR-0044 §2.9); không tool nào ghi sổ tiền.

## 1. Bối cảnh

- Chatbot nhóm và ~15 tính năng AI khác đi qua brain Python (`services/api`, `/internal/brain/v1/*`), mỗi
  module tự dựng client Gemini bằng `GEMINI_API_KEY`. Engine Go (`internal/aiharness`) đã có cho nhóm và
  Nếp nhưng mặc định vẫn `brain`.
- Máy vnlocal phục vụ **agy-proxy**: cổng nói giao thức OpenAI và Gemini trước một pool seat Gemini dùng
  chung, bắt buộc khoá trong allowlist (vnlocal `HANDOFF-KET-NOI.md` §5). Client Go `internal/agyproxy`
  (commit `695896d1`) đo được: câu ngắn p50 ~3–4 s ở mọi model (sàn là proxy), câu dài 5,2 s với
  `gemini-3.5-flash-lite` (0 token nghĩ) so với 7–8 s của các model 3.8.
- Milvus/RAG phía Go đã có code và 17 test tầng Milvus (schema `rd.v2`, hai hàm BM25, hybrid RRF,
  reranker client) nhưng chưa có Milvus chạy thật, chưa có batch embedding, chưa có index GPU.
- Máy dev có RTX 4070 16 GB, nvidia-container-toolkit 1.20.1.

## 2. Quyết định

1. **Sinh chữ và trích xuất có trường tự do → Go, qua agy-proxy.** Engine Go giữ ADK + genai; chỉ đổi base
   URL sang `AGY_PROXY_URL` với khoá `AGY_PROXY_KEY` (header `x-goog-api-key`). Model sinh chữ giữ
   `gemini-3.5-flash-lite`. Trích xuất (slot của router, đọc bill, enrichment khi nạp) dùng cùng đường với
   `json_schema`. Mọi lần thử lại vẫn tính vào trần 8 lời gọi/lượt.
2. **Phân loại → `typesafe/jev-1.13` qua OpenRouter.** Guard (sạch / nhạy cảm / chèn lệnh / hành động tiền),
   intent, loại tiền, đường đi, CRAG liên quan/không, kiểm chứng đạt/không. Jev là model «decisions»:
   `POST https://openrouter.ai/api/alpha/decisions` với `{model, state, questions}`; mỗi câu hỏi kiểu
   `choice` (criteria là map lựa chọn → mô tả), `score` (criteria là mảng hai đầu) hoặc `noul`
   (criteria `true`/`false`). Đo thử 2026-09-28: một lượt ba câu hỏi 0,5 s, $0,00002. Chính sách tiền
   (`hieu.QuyetDinhCho`) vẫn là mã tất định; jev chỉ gắn nhãn.
3. **Rerank → `qwen/qwen3-reranker-8b` qua OpenRouter**, provider chỉ Fireworks, không fallback.
   `POST https://openrouter.ai/api/v1/rerank` trả đúng hình Cohere/vLLM mà `internal/rerank` đang đọc
   (`results[].index`, `relevance_score`). Đo thử: 3 tài liệu, $0,000056.
4. **Python chỉ còn một sidecar mỏng** (`services/ai-infer`): gọi OpenRouter cho jev và rerank, bind loopback,
   token nội bộ. Sidecar đưa ra `POST /rerank` đúng hợp đồng vLLM nên `internal/rerank` không đổi mã, chỉ đổi
   `MOBILE_RERANK_URL` sang loopback. Ngoài ra chỉ còn thị giác máy không phải LLM (`face-boxes`).
   Khoá `OPEN_ROUTER_API_KEY` chỉ nằm ở sidecar.
5. **Brain Python bị xoá theo từng tính năng** khi bản Go thay nó: mã Go gọi brain, module Python, route
   nội bộ và test đi kèm bị xoá trong **một** commit (mẫu ADR-0036). Đường chatbot (companion-reply,
   nep-reply, chat-expense) đi trước; bật `MOBILE_AI_ENGINE_GROUP=go` và `MOBILE_AI_ENGINE_NEP=go`.
6. **Tìm kiếm → Milvus GPU chạy trên máy dev** (`milvusdb/milvus:v3.0.2-gpu`, cùng bản mà tầng test Milvus
   đang ghim), chỉ bind loopback, bật auth. Dense dùng index GPU; BM25 built-in của Milvus (hai trường có
   dấu / không dấu) chạy CPU như cũ; gộp hybrid RRF như ADR-0047.
7. **Embedding tài liệu bằng Gemini Embedding 2 qua Batch API** (giá rẻ hơn online). Cùng model, cùng
   1536 chiều, cùng định dạng prefix với đường online (`aiharness/nhung`), ghi vào `rag_embedding_cache`,
   nên bản build chỉ đọc cache. Câu truy vấn vẫn embed online. Khoá là `GEMINI_API_KEY` (gọi thẳng Google:
   agy-proxy không phục vụ embedding).
8. **Trí nhớ Nếp dựng lại bằng Go trên Milvus** (collection trí nhớ có partition key `owner_id`), thay mem0
   Python: trích sự kiện bằng agy + schema, jev quyết thêm/sửa/xoá/bỏ qua, embed online.

### Dữ liệu nào đi đâu

| Bên nhận | Nhận gì | Không bao giờ nhận |
|---|---|---|
| agy-proxy (máy vnlocal trong LAN → Google) | câu hỏi được gọi, bối cảnh máy đã gom, dữ liệu quán, ảnh bill khi người dùng quét | id người, số dư, lịch sử không được chia sẻ |
| OpenRouter → TypeSafe (jev) | câu hỏi đã qua `preprocess`, nhãn cần chọn | bối cảnh chat dài, ảnh, id người |
| OpenRouter → Fireworks (rerank) | câu truy vấn + mô tả quán | bối cảnh chat, id người |
| Google Batch API (embedding) | mô tả quán, review đã lọc (không tác giả) | mọi dữ liệu người dùng |

Không body nào của các bên này vào log, metrics hay span (giữ ADR-0044 §4).

## 3. Hệ quả

- Brain Python mất dần route; `core work` không còn đòi `MOBILE_BRAIN_URL` khi đường chatbot đã sang Go.
- Thêm phụ thuộc mạng: máy vnlocal (agy) và OpenRouter. agy hết seat trả 429 + `Retry-After`; OpenRouter lỗi
  thì rerank giữ thứ tự RRF (`no_rerank`), phân loại lỗi thì lượt trả câu từ chối cố định chứ không đoán.
- Milvus chiếm một phần VRAM của máy dev (giới hạn pool trong `milvus.yaml`).

## 4. Cái này KHÔNG cho phép

- Không gọi provider thật trong CI, test hay parity; base URL khi test vẫn chỉ loopback.
- Không để khoá agy/OpenRouter/Gemini vào Git, app mobile hay log.
- **Không mở cho người dùng thật** khi chưa có hồ sơ đánh giá tác động chuyển dữ liệu xuyên biên giới
  (Luật 91/2025/QH15, NĐ 356/2025/NĐ-CP — như ADR-0043 đã ghi): OpenRouter, TypeSafe, Fireworks, Google
  đều ở ngoài Việt Nam. Văn bản này chỉ cho phép chạy trên máy dev với dữ liệu thử và tài khoản nội bộ.
- Không gửi phòng v2 E2EE sang bất kỳ bên nào.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

- ADR-0043 §2.6 (Qwen3-Reranker-4B tự host sau vLLM): **thay** bằng §2.3 ở đây. Hợp đồng `/rerank` và quy tắc
  đọc kết quả giữ nguyên.
- ADR-0043 (mem0 trong sidecar Python): **thay** bằng §2.8.
- ADR-0044 §2.2 («Gemini được gọi trực tiếp từ Go», một hằng model duy nhất): **sửa** — gọi qua agy-proxy,
  hằng model giữ nguyên cho sinh chữ; phân loại đi jev (§2.2 ở đây).
- ADR-0044 §2.13 («Python còn ba việc AI»): **sửa** — Python còn sidecar OpenRouter và thị giác không-LLM;
  eval vẫn được phép.
- ADR-0047 §2.3 (`gemini-embedding-001` 768 trên pgvector): đã lỗi thời từ quyết định 2026-09-27
  (`gemini-embedding-2` 1536 trên Milvus); văn bản này thêm Batch API cho tài liệu.

## 6. Cổng nghiệm thu

- Tầng Milvus (`scripts/go_milvus_tier.sh`) xanh trên Milvus GPU của máy, skip là đỏ.
- Nạp danh mục: 100% vector lấy từ cache sau batch; `v-eval` đạt ngưỡng trong `rag/nap/cauhinh.json`.
- E2E nhóm trên stack vnlocal: `@Rủ Đi` gợi ý quán thật từ Milvus có rerank; `/chia-bill` ra nháp không ghi sổ;
  câu đòi chuyển tiền bị từ chối cố định; độ trễ từng bước ghi vào commit.
