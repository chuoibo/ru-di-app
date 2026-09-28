# Gap analysis: pipeline agent của «Rủ Đi AI» / «Nếp» so với SOTA cuối 09/2026

- Ngày: 2026-09-27 · Chỉ đọc repo, không sửa · Người viết: subagent nghiên cứu
- Nhãn nguồn: **[P]** nguồn gốc (docs chính thức, blog kỹ thuật, paper, release notes) · **[S]** thứ cấp hoặc chỉ thấy qua đoạn trích tìm kiếm · **[U]** chưa kiểm được.
- ArXiv id viết có dấu cách hoặc trích theo tên để không có chuỗi số dài.

_Bản hoàn chỉnh; ghi chép thô ở cuối file._

## 0. Tóm tắt điều hành

**Kết luận chung: thiết kế đạt mức "adequate tới SOTA" ở kiến trúc điều khiển, an toàn và kỷ luật đo. Nó tụt lại ở ba chỗ: kinh tế context/cache, độ bền truy hồi tiếng Việt, và độ chín của các thành phần mượn (reranker trên CPU, mem0, ADK-Go v1).** Phần lớn các lỗ hổng sửa được mà **không tốn thêm lời gọi model nào**. Chúng nằm ở cách xếp prompt, trường ra của router, analyzer của chỉ mục và cách viết eval.

Ba lưu ý về phương pháp. Đọc trước khi tin các nhãn dưới đây.
1. **Máy này không vào được** ai.google.dev, docs.cloud.google.com, developers.googleblog.com, security.googleblog.com, deepmind.google, manus.im, milvus.io, opentelemetry.io, simonwillison.net. Các khẳng định về tài liệu Gemini, Milvus và bài phòng thủ của Google vì vậy là **[S]** (đoạn trích tìm kiếm của chính trang đó). Người có mạng cần mở lại trước khi chốt. Nguồn đọc được tận gốc gồm GitHub (release note ADK-Go, issue go/python-genai, repo semconv), proxy.golang.org (kiểm bằng shell), anthropic.com và claude.com.
2. **Đa số pipeline mô tả trong đề bài chưa có mã.** Theo `docs/architecture/03-ai-engine-hop-dong.md`, các lát sau đều chưa xong: lát 9 (understand, fast path, agent), 15 (trí nhớ), 16 (RAG vector), 17 (nhắc chủ động) và 18 (eval M3). Verdict ở đây vì vậy chấm **thiết kế**, không chấm hành vi đã đo.
3. Hợp đồng engine ghi embedding là `gemini-embedding-001` 768 chiều. Đề bài ghi Gemini Embedding 2 1536 chiều. **Phải chốt một bên** trước lát 16, vì collection, alias và golden eval phụ thuộc vào số chiều.

---

## A. Verdict theo từng chặng

| Chặng | Verdict | Lý do và bằng chứng |
|---|---|---|
| Job + outbox + RabbitMQ + worker | **SOTA** (với quy mô app) | Ghi job và outbox trong cùng một tx. Có poller SKIP LOCKED dự phòng, DLQ và publish exactly-once. Đây là mẫu chuẩn của hệ phân tán, không phải đặc thù AI. Không có gap |
| Tiền xử lý cấu trúc (NFC, bỏ mention, «bây giờ» theo giờ VN) | **Adequate** | Thiếu hai thứ: một dạng **bỏ dấu** song song cho truy hồi từ vựng, và kế hoạch cho câu **không dấu / teencode** ở tầng truy hồi. Paper IR tiếng Việt 2025 sinh biến thể bỏ dấu để tăng độ bền (arXiv 03/2025) [S] |
| STM Redis (TTL, mã hoá) | **Adequate, có xung đột chính sách** | Về kỹ thuật thì đúng mẫu. Nhưng ADR-0036 §2.7/§4 nói phiên Nếp giữ trên máy, không lên máy chủ (xem `stm-personalization.md`). Thiếu compaction có cấu trúc: Anthropic 2025-09-29 và ADK-Go v2.3.0 đều có compaction [P] |
| Router 1 lời gọi, structured output, enum đóng | **SOTA** về hình dạng | Trùng mẫu "action-selector" trong paper Design Patterns (arXiv 06/2025) [S]. Route được chốt **trước** khi đọc dữ liệu không tin cậy, đúng tinh thần plan-then-execute. Gap: router **chưa sinh truy vấn truy hồi độc lập** (rewrite) và chưa sinh dạng khôi phục dấu, dù việc này không tốn lời gọi nào (gap #3) |
| Agent loop ADK (VALIDATED, NONE ở bước cuối, ≤4 bước, ≤10 tool) | **Adequate → behind về framework** | Ngân sách và quyền tốt hơn thông lệ. Nhưng ADK-Go **v1.7.0 (2026-09-14) chỉ là dòng backport**. Dòng `adk/v2` đã có từ 2026-06-30 và v2.4.0 ra 2026-09-11, với graph workflow, compaction, OTel có content opt-in và collaboration agents [P proxy.golang.org + release notes] (gap #2). Chưa có kế hoạch tool call song song. Anthropic đo parallel tool calls giảm tới 90% thời gian [P] |
| Retrieval hybrid (dense + BM25, RRF, lọc cứng trong expr, kiểm lại trên Postgres) | **Adequate** | Kiểm lại trên Postgres là điểm hơn thông lệ. Thiếu contextual retrieval lúc nạp: Anthropic −49% lỗi top-20, −67% khi có rerank [P] (gap #6). Thiếu analyzer bỏ dấu. RRF k=60 mặc định, chưa có fusion chỉnh theo nhãn [S] |
| Rerank Qwen3-Reranker 0.6B trên CPU | **Behind (rủi ro latency)** | Tự đo trong `qwen-reranker.md` [P]: 30 cặp × 200 token ≈ 23.5 s trên 4 vCPU (Q8_0). Con số này không vừa mục tiêu token đầu p50 ≤2.5 s (gap #1) |
| Sufficiency + ≤1 vòng sửa (Adaptive-CRAG) | **SOTA về khái niệm** | Chỉ nới ràng buộc mềm, và ràng buộc cứng do model trích ra. Đúng hướng. Rủi ro nằm ở **ngân sách lời gọi** (xem D.6) |
| LTM mem0 (sidecar Python, Milvus, telemetry tắt, xoá cứng) | **Adequate, thành phần non** | mem0 2.x OSS dùng thuật toán ADD-only, đã gỡ graph (PR #4805, 2026-04-14). Telemetry bật mặc định, và các hàm theo `memory_id` không kiểm chủ sở hữu (`mem0.md`, đọc mã wheel [P]). Với ≤5 fact opt-in thì đủ. Chưa có củng cố bất đồng bộ kiểu sleep-time của Letta [S] |
| Sinh câu trả lời + token `[[p:ID]]` từ evidence ledger | **SOTA / đi trước** | Model không bao giờ tự viết URL hay tên quán. Tên quán được thay từ ledger. Cách này chặn luôn kênh exfil bằng link/markdown mà lớp phòng thủ của Google phải sanitize sau [S] |
| Reflection: kiểm grounding tất định + verifier LLM fail-closed, ≤1 lần sinh lại | **SOTA** | Khớp bằng chứng 2023–2026: tự sửa chỉ có ích khi có tín hiệu ngoài (`reflection-verification.md`). Rủi ro: judge/verifier cùng họ model (hợp đồng §7 đã ghi nhận) |
| PII + cửa sổ output 48 rune → Redis Streams → SSE | **SOTA về an toàn stream** | Guard chạy trước khi phát, không có sự kiện «rút lại». Đổi lại mất một ít TTFT (48 rune ≈ vài chục ms ở ~350 tok/s [S]) |
| Quan sát (id/enum/đếm/thời gian) | **Đi trước về riêng tư, behind về chuẩn** | OTel GenAI semconv đặt mọi thuộc tính nội dung ở chế độ **opt-in** [P repo semconv-genai]. Ta chặt hơn thế. Nhưng tên span/metric chưa theo `gen_ai.*`, và semconv vẫn ở mức "Development" (tách repo 2026-06-12) [S] (gap #14) |
| Nạp dữ liệu (CDC, làm giàu LLM + hàng duyệt người, collection có phiên bản sau alias) | **SOTA** | Thiếu một bước: sinh "context header" cho từng chunk lúc làm giàu (gap #6) |
| Eval (cổng offline recall@10 ≥0.90, nDCG@10 ≥0.75, vi phạm = 0; T1 stub, T2 cassette, T3 thật có trần; skip = đỏ) | **Adequate → SOTA về kỷ luật; behind về đa lượt** | Kỷ luật cổng hơn hẳn thông lệ. Thiếu ba thứ. (1) Mô phỏng người dùng đa lượt kiểu τ²-bench với **pass^k** [P github sierra]. (2) Hiệu chỉnh judge bằng nhãn người, chưa rõ đã có tập đủ lớn: ARES dùng ~150 nhãn [P paper]. (3) Chấm quỹ đạo theo từng bước (gap #8) |
| Red team | **Behind** | Có ngưỡng M2 (ASR 0/≥80) nhưng chưa có công cụ sinh tấn công tự động và đa lượt: garak v0.15 có GOAT multi-turn và agent-breaker (2026-05) [S]; PyRIT [S] (gap #9) |
| Cá nhân hoá / chủ động | **Adequate (thận trọng đúng)** | Opt-in ≤5 fact là chặt hơn Gemini/ChatGPT memory [S]. Chủ động (lát 17) chưa có thiết kế đo tỉ lệ can thiệp bị chấp nhận. ProactiveBench (2026) cho thấy model thiếu tính chủ động đã hiệu chỉnh [S] |
| Tài nguyên tiếng Việt | **Behind** | Chưa đưa VN-MTEB, ViRanker hay bộ truy vấn không dấu vào golden. Chưa có số tiếng Việt riêng cho Gemini Embedding 2 và Qwen3-Reranker. Ta phải tự đo |

---

## B. Danh sách gap, xếp theo lợi ích / chi phí

Quy ước: "lời gọi" = lời gọi model sinh chữ, tính vào trần 8. Mọi đề xuất dưới đây đã được soi theo các luật: Go backend, flash-lite, không heuristic từ khoá, không nội dung trong quan sát, E2EE.

### #1 Reranker 0.6B trên CPU không vừa ngân sách latency — **adopt now (đo rồi quyết)**
- **Hệ thống dẫn đầu làm gì**: rerank trên GPU hoặc qua API managed như Cohere Rerank 4 hay voyage rerank-2.5 [S]. Một nhánh khác là reranker listwise nhỏ chấm cả danh sách trong một lượt, ví dụ jina-reranker-v3 (0.6B, 64 tài liệu/lượt, BEIR 61.94) [S].
- **Bằng chứng**: tự đo 23.5 s cho 30 cặp trên 4 vCPU (`qwen-reranker.md` [P]). Mục tiêu của ta là token đầu p50 ≤2.5 s.
- **Phương án xếp theo độ hợp**:
  (a) Giảm ứng viên xuống top-10–15, cắt passage còn ~96 token (tên, loại, quận, 1–2 câu), chạy song song với việc dựng evidence. Cần đo lại.
  (b) Thử **ViRanker** (cross-encoder BGE-M3, NDCG@3 0.6815 trên MMARCO-VI [S]). Kiến trúc encoder nên nhanh hơn Qwen decoder cùng cỡ trên CPU [U, phải đo]. Còn phải kiểm license.
  (c) Chuyển 4B lên GPU.
  (d) Cho phép **bỏ rerank** khi RRF đã đủ tốt theo eval, rồi để sufficiency evaluator lo phần còn lại.
  Loại jina-reranker-v3 vì license CC BY-NC 4.0 [S]. Loại LLM listwise rerank bằng flash-lite vì tốn thêm một lời gọi.
- **Chi phí**: 0 lời gọi. Latency là thứ đang được mua lại.
- **Lợi ích**: gỡ chặn mục tiêu TTFT. Anthropic đo: thêm rerank nâng mức giảm lỗi top-20 từ 49% lên 67% so với nền [P], nên đừng bỏ rerank mà không đo.

### #2 ADK-Go v1.7 là dòng bảo trì; v2 có sẵn những thứ ta định tự viết — **adopt now (spike + quyết trước lát 9)**
- **Bằng chứng [P]**: `google.golang.org/adk/v2` có v2.0.0 (2026-06-30), v2.3.0 (2026-08-31) và v2.4.0 (2026-09-11). v1.7.0 (2026-09-14) chỉ gồm backport. Các tính năng của v2 theo release note:
  - graph workflow có nhánh điều kiện, song song, retry/timeout, kiểm schema và HITL (v2.0);
  - `agent.Context` thống nhất (breaking);
  - context compaction theo ngưỡng, giữ "durable facts" (v2.3);
  - OTel 0.21, trong đó gen_ai content attributes là **opt-in** (v2.3);
  - REST có authn/authz (v2.4).
- **Vì sao gấp**: lát 9 (engine S2) sẽ viết agent loop trên API v1. Mỗi dòng code viết trên v1 làm chi phí chuyển sang v2 tăng thêm (module path `/v2`, gộp context).
- **Hợp ràng buộc**: đều là Go, và trần bước/tool vẫn do ta cưỡng chế. **ADK context caching (`ContextCacheConfig`) chưa có cho Go**: docs chỉ ghi Python/Java/Kotlin [P raw adk-docs]. Explicit cache vì vậy phải tự gọi qua go-genai.
- **Khuyến nghị**: làm spike 1–2 ngày port harness agent hiện tại lên v2.4. Nếu cổng T1/T2 xanh thì chốt v2 trước lát 9. Nếu không thì ghi ADR giữ v1 kèm ngày xem lại.

### #3 Router chưa sinh truy vấn truy hồi độc lập (rewrite) và dạng khôi phục dấu — **adopt now**
- **Hệ thống dẫn đầu làm gì**: viết lại câu đa lượt thành truy vấn tự đứng được trước khi truy hồi. Các đội SemEval-2026 Task 8 (multi-turn RAG) đều dùng rewriting kết hợp hybrid [S]. Rewriter riêng làm TTFT tăng 2.4x [S], nên xu hướng là gộp rewriting vào bước đã có.
- **Đề xuất**: thêm vào schema router ba trường. `truy_van_truy_hoi` là câu tự đủ nghĩa, đã giải tham chiếu tới lượt trước. `truy_van_co_dau` là dạng khôi phục dấu khi người dùng gõ không dấu. `ngon_ngu_nhap` là enum. Dense dùng `truy_van_truy_hoi`. BM25 dùng cả hai dạng. HyDE thì bỏ, vì tốn lời gọi và tăng latency.
- **Chi phí**: 0 lời gọi, thêm vài chục token output ở router.
- **Hợp luật không heuristic**: model quyết, code chỉ kiểm schema.

### #4 Truy hồi từ vựng chưa bền với câu không dấu / teencode — **adopt now**
- **Bằng chứng**: tiếng Việt gõ không dấu là mẫu nhập liệu phổ biến. Paper IR tiếng Việt 2025 thêm biến thể bỏ dấu vào dữ liệu huấn luyện để tăng bền [S]. Milvus có ICU tokenizer và filter `asciifolding`, cho phép cấu hình analyzer theo trường [S, milvus.io bị chặn].
- **Đề xuất**: thêm một trường BM25 thứ hai dùng analyzer đã fold dấu, rồi hybrid 3 nhánh (dense, BM25 có dấu, BM25 không dấu) qua RRF. Golden eval thêm một lát truy vấn không dấu và teencode, đo recall@10 **riêng** lát đó.
- **Hợp luật**: fold dấu là chuẩn hoá cấu trúc giống NFC, không phải phân loại ý nghĩa. Nên ghi rõ điều này vào ADR để không bị đọc thành heuristic.
- **Chi phí**: 0 lời gọi. Chỉ mục phình thêm một cột sparse.

### #5 Context chưa được thiết kế cho cache (implicit caching) — **adopt now**
- **Hệ thống dẫn đầu làm gì**: Manus coi KV-cache hit rate là metric số 1. Cách làm của họ: giữ prefix ổn định, context chỉ nối thêm, và **"mask, don't remove" tool** [S]. Gemini bật implicit caching mặc định từ 2.5 trở lên. Ngưỡng tối thiểu với Flash là ~1024 token, giảm giá 75% (số thời 2.5) [S].
- **Rủi ro thực tế [P GitHub]**: python-genai #2064 (2026-02-16, còn mở) báo cache của gemini-3-flash-preview về 0 trong khoảng ~9K–17K token dù prefix y hệt. vercel/ai #11513 (2026-01-04) báo implicit cache không trúng khi có tool definitions trên Gemini 3 Flash.
- **Đề xuất**:
  1. Xếp prompt theo thứ tự: system cố định theo bot → định nghĩa tool **cố định theo bot**. Với tool bị cấm trong bước, giới hạn bằng `allowed_function_names` ở chế độ VALIDATED thay vì bỏ khỏi danh sách. Tiếp theo là sổ tay/tham chiếu tĩnh → phần động (thời gian «bây giờ», STM, evidence) **ở cuối**.
  2. «bây giờ» hiện đi vào prompt mỗi lượt. Phải để nó sau prefix tĩnh, không để trong system.
  3. Đã có `CachedContentTokenCount` trong metric (`aiharness/agent/agent.go:117`). Hãy thêm **tỉ lệ token trúng cache theo bước** vào bảng số và vào T3.
- **Chi phí**: 0 lời gọi. Có thể giảm TTFT và giá input. Mức giảm phải đo, vì prompt của ta có thể dưới ngưỡng cache.

### #6 Contextual retrieval lúc nạp — **adopt now (gắn vào bước làm giàu LLM đã có)**
- **Bằng chứng [P]**: Anthropic (2024-09-19) đo contextual embeddings giảm 35% lỗi top-20, thêm contextual BM25 thì giảm 49%, thêm rerank thì giảm 67%.
- **Đề xuất**: bước làm giàu LLM đã có sinh thêm 1–2 câu ngữ cảnh cho mỗi chunk sổ tay app hoặc mô tả quán (quận, loại, hợp dịp nào), đi qua hàng duyệt người như hiện nay. Embed và index cả ngữ cảnh lẫn nội dung.
- **Chi phí**: 0 lời gọi mỗi lượt. Chỉ tốn lời gọi offline lúc nạp, tính vào ngân sách làm giàu.

### #7 Chỉnh cấu hình suy nghĩ và tốc độ theo từng bước — **adopt now**
- **Bằng chứng [S]**: 3.5 Flash-Lite mặc định thinking ở mức minimal. Nâng lên medium/high được khuyên cho subagent tự dùng tool. Tốc độ ~350 tok/s (Artificial Analysis).
- **Đề xuất**: đặt `thinking_level` tường minh cho từng bước. Router, trả lời và verifier dùng minimal. Bước lập kế hoạch trong agent loop thử low/medium. Chấm bằng T3 (quality vs p95). Tránh để mặc định thay đổi theo đợt model.
- **Chi phí**: 0 lời gọi. Thinking token có thể tăng ở agent loop.

### #8 Eval đa lượt bằng user simulator, pass^k, và judge đã hiệu chỉnh — **adopt now (một phần), phần simulator để later**
- **Hệ thống dẫn đầu làm gì**:
  - τ²-bench (Sierra, 2025-06) dùng user simulator LLM trong môi trường dual-control và đo **pass^k**, tức độ ổn định qua k lần [P].
  - Paper 2026 về simulator không hợp tác và persona thực tế: arXiv 05/2026, VISTA 06/2026 [S].
  - ARES dùng judge theo thành phần cùng prediction-powered inference trên ~150 nhãn người [P].
  - Về judge: "Judging the Judges" chỉ ra position bias; "Reliability without Validity" (2026) cho thấy judge nhất quán chưa chắc hợp lệ [S].
- **Đề xuất**:
  - **Now (0 lời gọi thêm mỗi lượt sản phẩm)**: báo pass^k bên cạnh pass@1 (M3 đã có "ổn định qua 5 lần", nên đổi tên cho khớp chuẩn). Dựng tập nhãn người ≥150 cho grounding/verifier và đo κ. Chấm **từng bước** của quỹ đạo (router đúng route? tool đúng? số bước?), không chỉ trạng thái cuối.
  - **Later**: simulator đa lượt trong T3, vì mỗi hội thoại mô phỏng tốn 2–3 lần số lời gọi. Hợp đồng đã yêu cầu Lead duyệt ngân sách lời gọi thật.
  - Verifier/judge cùng họ Gemini thì giữ như hợp đồng §7, nhưng **mỗi quý nên chạy một judge khác nhà cung cấp** trên mẫu đã khử định danh, để đo độ lệch tự thiên vị.

### #9 Red team tự động và đa lượt, nhắm prompt injection gián tiếp — **adopt later (trước M2)**
- **Bằng chứng**: garak v0.15.0 (2026-05-01) có GOAT multi-turn, agent-breaker và system-prompt extraction. v0.14.1 thêm bootstrap CI cho attack-success rate [S]. PyRIT có orchestrator đa lượt [S]. Google phòng thủ nhiều lớp cho Gemini: adversarial training, classifier, security thought reinforcement, sanitize markdown/URL, xác nhận người dùng [S].
- **Đề xuất**: đưa garak/PyRIT vào T3 với trần lời gọi, nhắm vào các nguồn không tin cậy của ta: mô tả quán, sổ tay, nội dung chat được chia sẻ rõ ràng và fact trong trí nhớ. Báo ASR kèm khoảng tin cậy, đúng như mốc M2 yêu cầu. Cả hai là Python. Dùng như **công cụ QA ngoài sản phẩm** là hợp quy tắc "Python chỉ cho AI/eval".

### #10 Chốt bất biến chống injection trong agent loop — **adopt now (tất định, rẻ)**
- **Bằng chứng**: paper Design Patterns (arXiv 06/2025) [S] và CaMeL (arXiv 03/2025) [S] đều nói: sau khi agent đã đọc dữ liệu không tin cậy, dữ liệu đó không được quyết định hành động có hậu quả.
- **Hiện trạng**: tool chỉ đọc hoặc chỉ tạo nháp, có bảng quyền, output bọc `<du_lieu>`. Tốt. Còn hở một chỗ: bước giữa của agent loop đọc output tool rồi mới chọn tool kế.
- **Đề xuất (taint tất định)**: đối số tool ở bước ≥2 chỉ được là (i) giá trị router đã trích từ lời người dùng, hoặc (ii) id có trong evidence ledger. Chuỗi tự do lấy từ output tool không được làm đối số. Ràng buộc cứng của router phải được kiểm lại trên đối số, và đối số lệch thì bị từ chối. Tool nháp (`draft_poll`, `propose_itinerary`) luôn cần người bấm xác nhận.
- **Chi phí**: 0 lời gọi.

### #11 Tool call song song và truy hồi suy đoán — **adopt now cho song song, suy đoán giữ như thiết kế**
- **Bằng chứng**: Anthropic đo parallel tool calling giảm tới 90% thời gian [P]. Gemini hỗ trợ nhiều function call trong một lượt [S]. Speculative RAG (ICLR 2025) [S]; Speculative Actions (10/2025) đoán đúng bước kế ~55% [S].
- **Hiện trạng**: thiết kế đã có "truy hồi chạy suy đoán song song" với router. Đó là điểm mạnh.
- **Đề xuất**: khi model trả nhiều function call trong một bước, worker chạy chúng song song (errgroup, semaphore tool đã có ở lát 4). Mỗi call vẫn tính vào trần 10. Truy hồi suy đoán chỉ dùng **câu đã chuẩn hoá** làm truy vấn, và phải bỏ kết quả nếu router ra route khác, để không rò evidence sai route.
- **Chi phí**: 0 lời gọi. Giảm latency agent loop.

### #12 Cascade theo độ khó (leo thang model) — **later, cần ADR**
- **Hệ thống dẫn đầu làm gì**: định tuyến theo độ khó, dùng model rẻ trước và leo lên khi có tín hiệu thất bại. Google ra 3 bản Flash trong 6 tuần, bản 3.8 Flash ra 2026-09-02 (GitHub changelog 2026-09-03) [S].
- **Đề xuất**: giữ flash-lite cho mọi bước. Đo trước ở T3 xem **khi verifier fail, sinh lại bằng Flash có sửa được nhiều hơn sinh lại bằng flash-lite không**. Nếu có thì mở ADR cho một bước leo thang duy nhất.
- **Chi phí**: 0 lời gọi thêm (thay model cho lời gọi sinh lại), nhưng đắt hơn và p95 dài hơn.

### #13 Compaction STM có cấu trúc thay cho cắt theo TTL — **later (sau khi giải xung đột ADR-0036)**
- **Bằng chứng**: Anthropic khuyên compaction và structured note-taking [P]. ADK-Go v2.3 có compaction giữ durable facts [P]. ACE (arXiv 10/2025) cập nhật context dạng delta để tránh "context collapse" khi tóm tắt lặp [S].
- **Đề xuất**: với nhóm, dùng một "bản ghi trạng thái kèo" có schema (ai, ở đâu, khi nào, ràng buộc đã chốt) do code tất định cập nhật từ output router. Không dùng tóm tắt tự do bằng LLM, để không tốn lời gọi và không có tóm tắt chứa nội dung chat nằm ở máy chủ. Với Nếp thì theo ADR-0036: phiên ở trên máy.
- **Chi phí**: 0 lời gọi nếu làm bằng code. Dùng summarizer LLM thì loại.

### #14 Đặt tên span/metric theo OTel GenAI semconv, vẫn không có nội dung — **later**
- **Bằng chứng**: semconv GenAI tách repo từ v1.42.0 (2026-06-12), vẫn "Development" [S]. Cây span chuẩn là `invoke_agent` → `chat` → `execute_tool`, cùng các operation `retrieval`, `plan` và memory. `gen_ai.input/output.messages`, `system_instructions` và `tool.definitions` là opt-in [P repo]. Langfuse self-host có masking phía server [S].
- **Đề xuất**: dùng tên `gen_ai.operation.name`, `gen_ai.usage.*`, `gen_ai.response.finish_reasons` cho metric nội bộ, và pin phiên bản semconv. **Không bật** thuộc tính nội dung, không cài provider toàn cục (đúng hợp đồng). Việc này giúp sau này đổi backend quan sát mà không phải đặt lại tên.

### #15 Học quy trình ngoại tuyến (playbook kiểu ACE) từ ca eval thất bại — **later**
- **Bằng chứng [S]**: ACE dùng Generator/Reflector/Curator, đạt +10.6% trên agent và +8.6% trên tài chính. Code đã mở. LangMem prompt optimizer đi theo hướng tương tự.
- **Đề xuất**: chỉ chạy **offline**. Reflector đọc ca T3 thất bại (dữ liệu tổng hợp, không phải chat thật), đề xuất delta cho system prompt, rồi đưa qua hàng duyệt người và cổng eval. Không bao giờ học online từ nội dung người dùng: vi phạm E2EE và luật AI không tự đọc chat.

### #16 Fusion có trọng số hoặc học được thay cho RRF k=60 — **later (khi golden đủ nhãn)**
- **Bằng chứng [S]**: weighted fusion thắng RRF về nDCG@10 ở 5/7 dataset khi có dữ liệu chỉnh. RRF k=60 thường chưa tối ưu.
- **Đề xuất**: dò k, hoặc học trọng số trên golden, và giữ RRF làm baseline trong cổng. 0 lời gọi.

### #17 Củng cố trí nhớ bất đồng bộ (sleep-time) — **later (lát 15)**
- **Bằng chứng [S]**: Letta sleep-time agents củng cố memory ngoài đường nóng.
- **Đề xuất**: trích và củng cố fact chạy trên lane `memory` của RabbitMQ, không nằm trong lượt. Lượt chỉ đọc ≤5 fact. Việc này bỏ lời gọi trích fact khỏi trần 8 của lượt. Lời gọi đó cần một ngân sách riêng.

### Các hướng nên **reject** hiện tại (và lý do)
| Hướng | Lý do từ chối |
|---|---|
| Multi-agent / deep-research planner | Tốn 3–10x token so với single agent, và Anthropic khuyên không dùng khi tool ≤15–20 [P claude.com]. Mô hình 15x chỉ đáng cho research giá trị cao [P]. Trần 8 lời gọi không chứa nổi |
| GraphRAG / LightRAG | GraphRAG-Bench (ICLR 2026) cho thấy RAG thường ngang hoặc hơn ở truy xuất đơn giản [S]. Quan hệ của ta (nhóm, kèo, quán) đã nằm trong Postgres, nên tool có tham số làm tốt hơn |
| Text-to-SQL tự do | Mở mặt tấn công và phá "một writer", "tool read-only có tham số". Router trích slot rồi dùng tool SQL cố định đã là dạng an toàn của text-to-SQL |
| Semantic cache câu trả lời | Có tấn công va chạm khoá và poisoning (NDSS 2026; arXiv 01/2026) [S]. Cache chia sẻ giữa user làm rò rỉ, câu trả lời lại phụ thuộc «bây giờ»/nhóm/E2EE. Chỉ cache chính xác cho embedding của văn bản quán/sổ tay |
| Late interaction / multi-vector (ColBERT trên Milvus StructArray) | Milvus hỗ trợ MAX_SIM (2.6.4+/3.0) [S], nhưng chưa có model ColBERT tiếng Việt được kiểm chứng, và chỉ mục phình nhiều lần. Xem lại nếu golden cho thấy dense+sparse bão hoà |
| Graph memory (Zep/Graphiti, Mem0g) | ≤5 fact opt-in không cần KG thời gian. Mem0g chỉ hơn 68.44 vs 66.88, search chậm ~3x [S]. Số LongMemEval mem0 49.0 vs Zep 63.8 đến từ bài bên thứ ba [S] |
| MCP / A2A cho tool nội bộ | Tool là Go trong cùng tiến trình, có bảng quyền. MCP 2026-07-28 (stateless, Tasks) [S] và A2A v1.0 (2026-04-09) [S] chỉ đáng khi mở tool cho bên thứ ba |
| HyDE / multi-query | Mỗi cái thêm ≥1 lời gọi và làm TTFT tăng [S]. Đã có #3 thay thế |
| jina-reranker-v3 | License CC BY-NC 4.0 [S] |

---

## C. Những chỗ ta đi trước thông lệ

1. **Trần lời gọi cứng, đếm nguyên tử trên hàng job, kể cả retry**, và retry riêng của genai bị tắt. Hầu hết framework (OpenAI Agents SDK, ADK) chỉ có `max_turns` và không đếm retry.
2. **Citation bằng token id rồi thay từ ledger, kèm kiểm `cited ⊆ evidence` tất định.** Model không tự viết tên, URL hay số, nên một lớp exfil mà Google phải sanitize sau [S] bị chặn ngay từ thiết kế.
3. **Verifier fail-closed + ≤1 lần sinh lại, không tự sửa nội tại.** Khớp bằng chứng khoa học tốt hơn mẫu "reflect until good" còn phổ biến.
4. **Route và ràng buộc cứng chốt trước khi đọc dữ liệu không tin cậy.** Tính chất này giống action-selector / plan-then-execute trong paper Design Patterns [S]. Tool chỉ đọc hoặc nháp, có bảng quyền theo bot.
5. **Lọc cứng trong expr của Milvus rồi kiểm lại trên Postgres.** Chỉ mục vector không bao giờ là nguồn sự thật. Đa số hệ RAG tin thẳng metadata trong vector DB.
6. **Quan sát không nội dung theo mặc định và bắt buộc.** Chặt hơn semconv, vốn chỉ đặt nội dung ở opt-in [P].
7. **Skip = đỏ, cassette T2, trần lời gọi ở T3, cổng eval có ngưỡng vi phạm = 0.** Đó là kỷ luật cổng mà hầu hết đội sản phẩm AI chưa có.
8. **Xoá cứng + tombstone băm cho trí nhớ, telemetry mem0 tắt, trigger xoá theo `people.deleted_at`.** Mạnh hơn điều khiển "delete chat, có độ trễ" của trợ lý lớn [S].
9. **Outbox + publish exactly-once, và guard trước khi phát, không có sự kiện «rút lại».** Nhiều sản phẩm stream trước rồi mới moderation.
10. **Collection có phiên bản sau alias, cùng hàng duyệt người cho làm giàu LLM.** Đây là hình mẫu LLMOps đúng.

---

## D. Rủi ro trong các lựa chọn hiện tại

1. **Qwen3-Reranker 0.6B trên CPU**: ~23.5 s cho 30 cặp (đo [P]), nên vỡ TTFT. Chưa có số tiếng Việt riêng. Xem gap #1.
2. **MILCO**: README không ghi license (`milco.md` [P]). Model ~560M chạy mỗi truy vấn (encode_query) cũng tốn CPU. Gắn "khi license rõ" là đúng. Nên có phương án cuối là **BM25 + analyzer fold dấu** (gap #4) thay vì chờ.
3. **mem0 2.x non và đổi nhanh**: 2.1.0 (2026-09-18), 2.2.0 (09-23), 2.2.1 (09-25). Thuật toán ADD-only, graph đã bị gỡ khỏi OSS, telemetry PostHog bật mặc định, hàm theo `memory_id` không kiểm chủ sở hữu (`mem0.md` [P]). Cách giảm: pin phiên bản, sidecar do ta viết kiểm chủ sở hữu, Postgres giữ sổ gốc, test tắt telemetry là test đỏ được.
4. **ADK-Go v1 thành dòng bảo trì** (gap #2). Càng viết nhiều trên v1 thì càng tốn công chuyển.
5. **Model churn**: 3 bản Flash trong 6 tuần [S]. Ngày phát hành 3.5 Flash-Lite còn xung đột giữa các nguồn [U]. Cassette T2 sẽ cũ đi nhanh, và `thinking_level` mặc định có thể đổi. Nên pin id model đầy đủ, và coi mỗi lần đổi model là một lần chạy lại T3 có trần.
6. **Ngân sách lời gọi xấu nhất có thể vượt 8.** Tính cho route agent: router 1 + agent ≤4 (bước cuối là câu trả lời) + sufficiency evaluator (nếu là LLM) 1 + verifier 1 + sinh lại 1 + verifier lần 2 1 = **9**. Phải ghi rõ bước nào là LLM, thứ tự ưu tiên khi cạn trần (ví dụ bỏ vòng sửa CRAG trước, không bao giờ bỏ verifier), và test đỏ được cho ca này. Sufficiency có thể gộp vào bước agent kế thay vì là một lời gọi riêng.
7. **Implicit caching không đáng tin tuyệt đối** trên Gemini 3.x khi có tool, và có "vùng chết" 9K–17K token [P GitHub]. Đừng đưa lợi ích cache vào dự toán chi phí trước khi đo.
8. **Judge/verifier cùng họ với generator** nên có rủi ro tự thiên vị (Panickssery NeurIPS 2024, xem `reflection-verification.md`). Hợp đồng §7 đã ghi. Cách giảm là κ trên nhãn người và judge khác nhà cung cấp theo định kỳ.
9. **Căng giữa luật "không heuristic từ khoá" và luật tiền bằng regex** (`aiharness/guard/tien.go`, lớp 0 trước mọi lời gọi). Tài liệu biện minh bằng cổng FRR ≤0.02 và nói đây là lớp tiết kiệm lời gọi. Nhưng đề bài nêu "no keyword heuristics anywhere". Cần Lead/owner ghi **ngoại lệ có tên** vào ADR, nếu không thì đây là vi phạm spec.
10. **Xung đột STM Redis với ADR-0036** (phiên Nếp không lên máy chủ). Phải giải trước lát 15 (`stm-personalization.md`).
11. **Lệch số chiều embedding** giữa hợp đồng (001, 768) và đề bài (Embedding 2, 1536). Collection và golden sẽ sai nếu không chốt.
12. **Milvus v3 còn mới** (3.0.x). Dựa vào tính năng mới (StructArray, BM25 function) nghĩa là dựa vào mã ít người chạy. Chỉ mục dựng lại được từ Postgres là lưới an toàn đúng.
13. **Chủ động (lát 17)**: model chưa hiệu chỉnh tốt việc khi nào nên can thiệp (ProactiveBench [S]). Nên kích hoạt tất định (nhắc do chính người dùng đặt), opt-in, và đo tỉ lệ bị tắt hoặc bỏ qua.

---

## Nguồn chính (URL + ngày; nhãn)

- Anthropic, Effective context engineering for AI agents, 2025-09-29 [P] — https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
- Anthropic, How we built our multi-agent research system, 2025-06 [P] — https://www.anthropic.com/engineering/multi-agent-research-system
- Claude blog, When to use multi-agent systems [P] — https://claude.com/blog/building-multi-agent-systems-when-and-how-to-use-them
- Anthropic, Contextual Retrieval, 2024-09-19 [P] — https://www.anthropic.com/engineering/contextual-retrieval
- Manus, Context Engineering for AI Agents, 2025-07 [S] — https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus
- LangChain, Context engineering for agents [S] — https://www.langchain.com/blog/context-engineering-for-agents
- ACE, Agentic Context Engineering, arXiv 10/2025, 10/2025 [S]
- ADK-Go releases v2.0.0 (2026-06-30), v2.3.0 (2026-08-31), v2.4.0 (2026-09-11), v1.7.0 (2026-09-14) [P] — https://github.com/google/adk-go/releases ; thời điểm tag kiểm qua https://proxy.golang.org/google.golang.org/adk/v2/@v/list
- ADK docs, context caching (Python/Java/Kotlin, không có Go) [P] — https://raw.githubusercontent.com/google/adk-docs/main/docs/context/caching.md
- python-genai issue #2064, 2026-02-16 [P] — https://github.com/googleapis/python-genai/issues/2064
- vercel/ai issue #11513, 2026-01-04 [P] — https://github.com/vercel/ai/issues/11513
- Gemini API caching docs [S] — https://ai.google.dev/gemini-api/docs/caching
- Gemini 3.5 Flash-Lite (thinking minimal, ~350 tok/s, giá) [S] — https://blog.google/innovation-and-ai/models-and-research/gemini-models/gemini-3-6-flash-3-5-flash-lite-3-5-flash-cyber/
- Gemini 3.8 Flash trên GitHub Copilot, 2026-09-03 [S] — https://github.blog/changelog/2026-09-03-gemini-3-8-flash-is-now-available-in-github-copilot/
- Gemini structured output / streamFunctionCallArguments [S] — https://ai.google.dev/gemini-api/docs/structured-output , https://ai.google.dev/gemini-api/docs/tools
- MCP spec 2026-07-28 [S] — https://blog.modelcontextprotocol.io/posts/2026-07-28/
- A2A một năm, v1.0, 2026-04 [S] — https://opensource.googleblog.com/2026/04/a-year-of-open-collaboration-celebrating-the-anniversary-of-a2a.html
- Zep, arXiv 01/2025 [S]; so sánh mem0/Zep của bên thứ ba [S] — https://vectorize.io/articles/mem0-vs-zep
- mem0 v2 bỏ graph khỏi OSS [S] — https://docs.mem0.ai/migration/platform-v2-to-v3 (bị chặn, dẫn qua bài thứ cấp) ; mã wheel đọc trực tiếp trong `mem0.md` [P]
- Letta sleep-time compute [S] — https://www.letta.com/blog/sleep-time-compute/
- Milvus StructArray / MAX_SIM [S] — https://milvus.io/blog/milvus-3-0-structarray.md
- Milvus analyzer [S] — https://milvus.io/docs/choose-the-right-analyzer-for-your-use-case.md
- Bảng so reranker 2026 (bên thứ ba) [S] — https://futureagi.com/blog/best-rerankers-for-rag-2026/
- jina-reranker-v3 (CC BY-NC 4.0), arXiv 09/2025 [S] — https://huggingface.co/jinaai/jina-reranker-v3
- ViRanker, arXiv 09/2025 [S]
- VN-MTEB, arXiv 07/2025; ACL Findings EACL 2026 [S] — https://aclanthology.org/2026.findings-eacl.86/
- Advancing Vietnamese IR with Learning Objective and Benchmark, arXiv 03/2025 [S]
- Gemini Embedding 2 paper, arXiv 05/2026 [S]
- GraphRAG-Bench, ICLR 2026, arXiv 06/2025 [S] — https://github.com/GraphRAG-Bench/GraphRAG-Benchmark
- τ²-bench, arXiv 06/2025 [P repo] — https://github.com/sierra-research/tau2-bench
- BFCL v4 (và audit 48% câu lỗi của Epoch) [S] — https://epoch.ai/benchmarks/berkeley-function-calling-leaderboard/review
- ARES, arXiv 11/2023 [P paper]
- Judging the Judges (IJCNLP 2025) [S] — https://aclanthology.org/2025.ijcnlp-long.18/ ; Reliability without Validity, arXiv 06/2026 [S]
- garak [S] — https://garak.ai/ ; PyRIT [S] — https://microsoft.github.io/PyRIT/latest/
- Design Patterns for Securing LLM Agents against Prompt Injections, arXiv 06/2025 [S]; CaMeL, arXiv 03/2025 [S]
- Google, layered defense chống prompt injection, 2025-06 [S] — https://security.googleblog.com/2025/06/mitigating-prompt-injection-attacks.html ; whitepaper Lessons from Defending Gemini, 2025-05 [S]
- Speculative RAG, ICLR 2025, arXiv 07/2024 [S]
- Semantic cache poisoning, NDSS 2026; Key Collision Attack, arXiv 01/2026 [S]
- OTel GenAI semconv, repo riêng [P] — https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-agent-spans.md ; tình trạng "Development" [S] — dev.to/azena-ai
- Langfuse masking [S] — https://langfuse.com/docs/observability/features/masking
- ProactiveBench, arXiv 03/2026 [S]
- Gemini past chats + temporary chats [S] — https://blog.google/products-and-platforms/products/gemini/temporary-chats-privacy-controls/

---


## Ghi chép thô (nối dần)
### 1. Context engineering
- Anthropic "Effective context engineering for AI agents", 2025-09-29 [P] https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents — just-in-time retrieval qua id nhẹ; tool ít chồng chéo; compaction; structured note-taking; sub-agent trả tóm tắt 1-2k token; "smallest set of high-signal tokens".
- Manus (Yichao Ji), 2025-07 [S, manus.im bị chặn; snippet + bản Medium] — KV-cache hit rate là metric số 1; prefix ổn định, context append-only; "mask, don't remove" tools (giữ định nghĩa tool cố định, giới hạn bằng logit masking / tên tool có tiền tố); giữ lỗi trong context; recitation (todo.md).
- LangChain "Context engineering for agents" (write/select/compress/isolate) [S] https://www.langchain.com/blog/context-engineering-for-agents
- ACE (Agentic Context Engineering, Stanford/SambaNova/Berkeley, arXiv 10/2025, 10/2025) [S] — Generator/Reflector/Curator, playbook tăng dần dạng delta, chống "context collapse"; +10.6% agent, +8.6% finance; mã mở (sambanova.ai/blog/ace-open-sourced-on-github).
- Gemini implicit caching: bật mặc định cho 2.5+; giảm 75% (thời 2.5; 3.x có thể 90% [U]); ngưỡng tối thiểu Flash 1024 token [S qua snippet ai.google.dev/firebase]; `cachedContentTokenCount` trong usage metadata.
- RỦI RO cache thực tế [P, GitHub]: python-genai #2064 (2026-02-16, mở, p2) gemini-3-flash-preview cache về 0 trong khoảng ~9K-17K token dù prefix y hệt, phân bậc theo khối 2048/8192; vercel/ai #11513 (2026-01-04, đóng not planned) implicit cache không trúng khi có tool definitions trên Gemini 3 Flash. => phải ĐO cached_content_token_count chứ không giả định.
### 2. Kiến trúc agent
- **ADK-Go có dòng v2** [P, proxy.golang.org, kiểm bằng shell 2026-09-27]: `google.golang.org/adk/v2` v2.0.0 (2026-06-30), v2.1.0, v2.2.0, v2.3.0 (2026-08-31), v2.4.0 (2026-09-11). Dòng v1: v1.7.0 (2026-09-14) chỉ là backport sửa lỗi (release note github.com/google/adk-go/releases/tag/v1.7.0) → v1 thành dòng bảo trì [suy luận, release note không nói thẳng].
  - v2.0.0 [P release note]: graph workflow engine (điều kiện, song song fan-out/fan-in, retry/timeout, schema validation, HITL pause/resume); Collaboration agents (chat/task/single_turn, history cô lập); gộp ToolContext+CallbackContext thành agent.Context; module path /v2, Go ≥1.25, README-v2.md hướng dẫn di chuyển.
  - v2.3.0 [P]: context compaction (tóm tắt invocation đã xong, compaction giữa invocation khi prompt vượt ngưỡng, giữ "durable facts"); OpenTelemetry 0.21 cho logging; **opt-in** gen_ai content attributes trên span generate_content (tức mặc định không ghi nội dung — hợp luật no-content); agent registry; BigQuery Agent Analytics plugin; skilltoolset.
  - v2.4.0 [P, WebFetch]: REST API có authn/authz; memory service xếp hạng; MCP toolset result handling; OpenAI model.
  - ADK context caching (ContextCacheConfig: min_tokens, ttl_seconds, cache_intervals) chỉ ghi hỗ trợ Python ≥1.15, Java, Kotlin — **không có Go** [P raw adk-docs/docs/context/caching.md]. Compaction ghi Python/Java/TS/Kotlin trong docs, Go có từ v2.3.0 theo release note.
- Anthropic multi-agent research system [P] https://www.anthropic.com/engineering/multi-agent-research-system (2025-06-13): agent ~4x token so với chat, multi-agent ~15x; token giải thích 80% phương sai; parallel tool calls giảm tới 90% thời gian; eval bắt đầu ~20 ca, LLM judge có rubric, đánh giá trạng thái cuối.
- Claude blog "When to use multi-agent systems" [P] https://claude.com/blog/building-multi-agent-systems-when-and-how-to-use-them — multi-agent 3-10x token; bắt đầu đơn giản; tránh chia theo loại việc; ≤15-20 tool thì không cần.
- A2A v1.0 dưới Linux Foundation 2026-04-09 [S] (opensource.googleblog.com/2026/04/...), signed Agent Cards, AP2.
- MCP spec 2026-07-28 [S qua snippet blog.modelcontextprotocol.io]: lõi stateless, bỏ session + handshake, Tasks extension, MCP Apps, auth hardening (CIMD), deprecation policy.
- Bộ nhớ: Zep/Graphiti temporal KG (arXiv 01/2025, 01/2025) [S]; LongMemEval: số so sánh mem0 49.0% vs Zep 63.8% là từ bài vectorize.io (bên thứ ba, [S], có xung đột lợi ích tiềm năng). mem0 2.x OSS đã bỏ graph, ADD-only (xem mem0.md, đọc mã wheel [P]).
### 3. Retrieval + tiếng Việt
- Anthropic Contextual Retrieval (2024-09-19) [P] https://www.anthropic.com/engineering/contextual-retrieval — context embeddings −35% lỗi top-20 (5.7→3.7%), +contextual BM25 −49% (→2.9%), +rerank −67% (→1.9%). Chi phí ở lúc nạp, không ở lượt.
- Milvus multi-vector: Array of Structs + MAX_SIM từ 2.6.4; Milvus 3.0 StructArray (milvus.io blog, [S] vì milvus.io bị chặn) — ColBERT/ColPali-kiểu một hàng/tài liệu.
- Rerank 2026 [S, blog bên thứ ba futureagi/mixpeek/zeroentropy]: jina-reranker-v3 (~0.6B, listwise, 64 tài liệu trong 131k ctx); Cohere Rerank 4 (managed); voyage rerank-2.5 (instruction-following); Qwen3-Reranker Apache-2.0. Không có số tiếng Việt độc lập.
- ViRanker (BGE-M3 + blockwise, arXiv 09/2025, hội nghị 04/2026) [S]: NDCG@3 0.6815 trên MMARCO-VI, vượt PhoRanker/BGE. Có trên HF.
- VN-MTEB (arXiv 07/2025; ACL Findings EACL 2026, aclanthology 2026.findings-eacl.86) [S]: 41 dataset dịch từ MTEB, 6 task; top multilingual-e5-large-instruct 67.99, e5-Mistral-7B 67.67, gte-Qwen2-7B 65.84 (bản paper chưa có Gemini Embedding 2).
- "Advancing Vietnamese Information Retrieval with Learning Objective and Benchmark" (arXiv 03/2025) [S]: sinh biến thể bỏ dấu + lỗi chính tả để tăng bền cho embedding.
- Gemini Embedding 2 paper (arXiv 05/2026) [S]: MTEB multilingual 69.9; không có số riêng tiếng Việt.
- Milvus analyzer: có ICU tokenizer, multi-language analyzer by_field, filter asciifolding [S qua snippet milvus.io]. Tiếng Việt: token âm tiết theo khoảng trắng; truy vấn không dấu ("quan nhau binh thanh") là mẫu nhập liệu phổ biến → cần trường BM25 thứ hai đã fold dấu (đây là chuẩn hoá cấu trúc, không phải heuristic hiểu câu).
### 4. Evaluation
- τ²-bench (Sierra, arXiv 06/2025, 2025-06) [P github sierra-research/tau2-bench]: dual-control, user simulator LLM, pass^k (độ tin cậy lặp lại). 2026 có nhiều paper về user simulator thực tế hơn (arXiv 05/2026 "Beyond Cooperative Simulators"; VISTA 06/2026; diversity-guided 04/2026) [S].
- BFCL v4 [S]: trọng số 40% agentic (web search, memory), 30% multi-turn, 10% irrelevance; Epoch audit thấy 24/50 câu có lỗi (48%) → đừng chọn model theo BFCL một mình.
- ARES (arXiv 11/2023) [P paper]: judge riêng từng thành phần + prediction-powered inference, cần ~150 nhãn người để hiệu chỉnh. RAGAS: faithfulness/answer relevancy/context precision/recall.
- LLM judge: "Judging the Judges" position bias (IJCNLP 2025) [S]; "Reliability without Validity" (arXiv 06/2026) [S] — judge nhất quán nhưng không hợp lệ; khuyến nghị: tập nhãn người nhỏ đo đồng thuận, judge khác nhà cung cấp với generator, đổi thứ tự.
- Red team: garak v0.15.0 (2026-05-01) có GOAT multi-turn + agent-breaker probe; v0.14.1 bootstrap CI cho attack-success rate [S qua snippet, trang tổng hợp]; PyRIT v0.11 (02/2026) [S].
### 5. Guardrails
- Design Patterns for Securing LLM Agents against Prompt Injections (Beurer-Kellner et al., arXiv 06/2025, 06/2025) [S]: action-selector, plan-then-execute, LLM map-reduce, dual LLM, code-then-execute, context-minimization. Nguyên tắc: khi agent đã đọc dữ liệu không tin cậy thì không được để dữ liệu đó kích hoạt hành động hậu quả.
- CaMeL "Defeating Prompt Injections by Design" (Google DeepMind, arXiv 03/2025) [S]: LLM đặc quyền sinh chương trình, LLM cách ly parse dữ liệu, capability/taint tracking.
- Google: "Lessons from Defending Gemini Against Indirect Prompt Injections" (whitepaper 2025-05, storage.googleapis.com/deepmind-media) [S]; security.googleblog.com 2025-06 layered defense (model hardening bằng adversarial training, classifier nội dung, "security thought reinforcement", sanitize markdown, redact URL nghi ngờ, xác nhận người dùng) [S — trang bị chặn, nội dung từ snippet/tin].
### 6. Latency/cost
- Gemini 3.5 Flash-Lite [S]: mặc định thinking "minimal"; ~350 tok/s (Artificial Analysis); $0.30/1M in, $2.50/1M out; ngày phát hành xung đột giữa nguồn (07/2026 vs 2026-09-02) [U]. Gemini 3.8 Flash ra 2026-09-02 (github.blog changelog 2026-09-03) [S].
- Speculative RAG (Google, ICLR 2025, arXiv 07/2024) [S]; Speculative Actions (MIT/Cornell 10/2025) ~55% đoán đúng bước kế [S]; "Speculative Interaction Agents" (arXiv 05/2026) [S].
- Semantic cache risk: NDSS 2026 "When Cache Poisoning Meets LLM Systems"; "Key Collision Attack on LLM Semantic Caching" (arXiv 01/2026) [S] — va chạm embedding gây hit sai, cache chia sẻ giữa user làm rò rỉ.
### 7. Observability
- OTel GenAI semconv: 2026-06-12 (v1.42.0) tách sang repo open-telemetry/semantic-conventions-genai; tới giữa 07/2026 mọi gen_ai.* vẫn "Development" [S dev.to + P repo]. Agent spans: invoke_agent → chat → execute_tool, operation names gồm retrieval, plan, memory [P github semantic-conventions-genai gen-ai-agent-spans.md]. gen_ai.input.messages/output.messages/system_instructions/tool.definitions là **opt-in** vì chứa PII — tương thích luật no-content.
### 8. Cá nhân hoá / chủ động / bộ nhớ khác
- Gemini app: cá nhân hoá từ past chats + Temporary Chat 72h, điều khiển trong "Personal context" (blog.google, 2025-08) [S].
- Proactive: ProactiveBench (arXiv 03/2026) 22 MLLM thiếu tính chủ động, không tương quan với cỡ model, lịch sử hội thoại gây thiên lệch âm [S]; π-Bench (05/2026), ProactBench (05/2026) [S].
- Letta sleep-time compute: agent nền củng cố memory bất đồng bộ lúc rảnh (letta.com/blog/sleep-time-compute, 2025-04) [S]. LangMem: memory manager + prompt optimizer (procedural memory) [S].
- mem0: OSS gỡ graph (PR #4805, 2026-04-14), graph chỉ còn ở Platform trả phí; paper mem0 (arXiv 04/2025) Mem0g chỉ hơn 68.44 vs 66.88 trên LOCOMO, search chậm ~3x, token ~2x [S qua bài bên thứ ba; khớp mem0.md đọc mã wheel].
- GraphRAG-Bench "When to use Graphs in RAG" (ICLR 2026, arXiv 06/2025) [S]: RAG thường ngang/hơn GraphRAG ở truy xuất sự kiện đơn giản; Graph chỉ hơn ở suy luận nhiều bước/tóm tắt.
- Fusion: RRF vs weighted: weighted nDCG@10 tốt hơn 5/7 dataset khi có dữ liệu chỉnh [S, blog]; RRF k=60 mặc định thường chưa tối ưu [S].
- Query rewriting: rewriter riêng làm TTFT tăng 2.4x (bài tổng hợp, [S]); SemEval-2026 Task 8 multi-turn RAG các đội dùng rewriting + hybrid [S].
- Gemini streaming function-call arguments (streamFunctionCallArguments) cho Gemini 3+, **không hỗ trợ ở 3.1 Flash-Lite** [S snippet ai.google.dev]; 3.5 Flash-Lite chưa rõ [U].
- Langfuse: masking phía server khi self-host, mask_otel_spans từ 06/2026 [S].
### 9. Đối chiếu repo (chỉ đọc)
- docs/architecture/03-ai-engine-hop-dong.md: lát 9 (understand/fast path/agent), 15 (trí nhớ), 16 (RAG vector), 17 (nhắc chủ động), 18 (eval M3) **chưa làm**; thiết kế đã có "truy hồi chạy suy đoán song song" ở bước understand, guard tất định trên mọi nguồn không tin cậy, output bọc <du_lieu>.
- Hợp đồng ghi embedding `gemini-embedding-001` 768 chiều, còn đề bài ghi Gemini Embedding 2 1536d → lệch cần chốt.
- `aiharness/agent/agent.go:117` đã cộng CachedContentTokenCount vào metric → hạ tầng đo cache đã có.
- `aiharness/guard/tien.go`: luật tiền là regex tất định trước mọi lời gọi model (ADR-0033 §2.2) → căng với câu "không heuristic từ khoá ở bất kỳ đâu" trong đề bài; tài liệu tự biện minh là lớp 0 ưu tiên precision, có cổng FRR ≤0.02.
- go.mod: `google.golang.org/adk v1.7.0`.
