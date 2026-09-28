# Pipeline agent AI v2 — đã dựng gì, còn thiếu gì, bạn tự làm gì

Nhánh `claude/peaceful-hopper-32kwjs`, PR #654 (draft). File này thay phần «chưa có» của
`ai-v2-ban-giao.md` (bản đó viết trước khi dựng stack agentic). **Chưa có một lời gọi model thật nào**:
mọi số đo dưới đây chạy bằng model giả (stub / cassette / fake loopback). Dấu xanh chứng minh đường ống
và luật an toàn, **không** chứng minh câu trả lời hay.

## 1. Đã dựng (trên nhánh, có test)

| Tầng | Có gì | Commit chính |
|---|---|---|
| Hàng đợi | outbox → RabbitMQ (quorum, DLQ), worker `core work`, poller dự phòng | `d76a0a4`, `849664a` |
| Lõi agent (không heuristic) | router LLM một lời gọi (structured output, enum đóng); tool do model chọn (ADK function calling, VALIDATED/NONE, bảng quyền theo bot); CRAG một vòng; trích dẫn `[[p:ID]]` từ sổ bằng chứng; verifier fail-closed; bất biến taint; trần 8 lời gọi/lượt (xấu nhất 7) | `1b12e8c`, `d918bcd`, `8dee673` |
| Truy hồi | Milvus v3 hybrid: Gemini Embedding 2 (1536) + BM25 có dấu + BM25 bỏ dấu, RRF có trọng số; lọc cứng trong Milvus **và** kiểm lại trên Postgres; reranker Qwen3-Reranker-4B qua API (tắt khi chưa có URL) | `5030b30`, `d918bcd` |
| Nạp dữ liệu (SDLC) | bắt thay đổi → làm giàu LLM có hàng duyệt → embed → collection có phiên bản sau alias → cổng eval (recall@10, nDCG@10, vi phạm = 0) → promote/rollback | `5030b30` |
| Trí nhớ Nếp | mem0 2.2.1 trong sidecar Python (`services/ai-infer`, telemetry tắt, không SQLite, kiểm chủ sở hữu); Go giữ chính sách (công tắc mặc định tắt, «quên» saga đếm tới 0, xoá tài khoản); Redis ngắn hạn mã hoá + TTL; chỉ ghi từ lời chính người dùng | `90a9f7e`, `ed9c314`, `8dee673` |
| Stream | nháp → verifier → nhả dần qua cửa sổ 48 ký tự → Redis Streams → SSE; app hiện chữ chạy ở bảng Nếp và phía người gọi trong nhóm | `713f960`, `79d67ce` |
| Bot nhóm | chạy trên engine Go sau cờ `MOBILE_AI_ENGINE_GROUP=go`; chia bill bằng số nguyên đồng, Σ = tổng | `a6d631d` |
| Đo chất lượng | T1 (stub, CI), T2 phát lại cassette, T3 model thật một lệnh có trần lời gọi cứng | `addc39e` |
| Nền | Go 1.26.8, ADK-Go v2.4.0 | `47f4442` |

Hai cờ engine vẫn mặc định `brain` (Python cũ): người dùng chưa thấy gì thay đổi cho tới khi bật.

## 2. Bạn tự làm — theo thứ tự nên làm

### 2.1 Chạy model thật lần đầu (quan trọng nhất)
1. Thêm `GEMINI_API_KEY` trong cài đặt môi trường (menu môi trường cloud trên thanh tiêu đề session →
   Edit → biến môi trường). **Không dán key vào chat.** Chỉ session mới đọc được key.
2. Xin Lead duyệt con số N (trần lời gọi). Dự toán tối thiểu: corpus Nếp 1 lần **561**, 5 lần **2 801**;
   corpus nhóm xem `rudi-eval --du-toan`; bộ dị ứng v2 **1 846**; bộ tiền v3 **4 546**.
3. Chạy: `scripts/eval_that.sh --bo <corpus> --tran-goi N`. Script tự phát lại cùng SHA (0 lời gọi) và
   in đoạn số đo để dán vào commit. Hướng dẫn đầy đủ: `chay-model-that.md`.
4. Điền giá thật vào `services/core/internal/aieval/testdata/gia-model.json` (đang để trống có chủ đích).
5. Chỉ bật `MOBILE_AI_ENGINE_NEP=go` / `MOBILE_AI_ENGINE_GROUP=go` khi T3 đạt ngưỡng M2/M3 ở
   `thiet-ke-ai/06-bo-do-chat-luong.md` và Lead ký.

### 2.2 Hạ tầng production
| Việc | Chi tiết |
|---|---|
| Reranker | Dựng Qwen3-Reranker-4B trên GPU (vLLM, `/rerank`), rồi đặt `MOBILE_RERANK_URL`, `MOBILE_RERANK_MODEL`, `MOBILE_RERANK_TIMEOUT`, `MOBILE_RERANK_TOKEN`. Chạy golden check so với bản tham chiếu fp32 khi đổi image. |
| Milvus | Milvus v3.0.2 standalone (etcd nhúng, `woodpecker.storage.type=local`, auth bật), ghim digest; `MOBILE_MILVUS_ADDR`, `MOBILE_MILVUS_USER`, `MOBILE_MILVUS_PASSWORD(_FILE)`, `MOBILE_MILVUS_DB`. |
| Sidecar | Build image `services/ai-infer` (Dockerfile đã ghim digest, chưa build lần nào); `AI_INFER_TOKEN`; phía Go `MOBILE_AI_INFER_URL`, `MOBILE_NEP_MEMORY_KEY`. |
| Redis riêng cho AI | `save ""`, `appendonly no`, `volatile-ttl`, ACL; `MOBILE_AICTX_REDIS_URL`, `MOBILE_AICTX_KEY`. Stream dùng `MOBILE_REDIS_URL` (compose chưa đặt cho `core`). |
| RabbitMQ | host production; `MOBILE_AMQP_URL`. |
| CI | job `milvus` cần runner tự host có nhãn `self-hosted, milvus, reranker` (có Milvus + reranker) **và** biến repo `MILVUS_RUNNER=true` (Settings → Secrets and variables → Actions → Variables). Thiếu biến thì job hiện **Skipped** (xám, không phải xanh) thay vì kẹt «Queued» mãi như trên PR #654 ngày 2026-09-28; khi đó chạy tay `scripts/go_milvus_tier.sh`. Đăng ký runner rồi đặt biến là bật lại. |

### 2.3 Máy thật và giao diện
- Kiểm trên iOS/Android: `fetch` có stream thật không; Reduce Motion hệ thống; VoiceOver/TalkBack với chữ
  đang lớn; độ trễ thật; ảnh chụp màn thật (hiện chỉ có ảnh trang lab trên Expo web).
- Làm đẹp bằng `/impeccable` (không có trong session cloud): hiệu ứng «đang nghĩ» của Nếp (cần sửa
  `DESIGN.md` và Lead ký), dòng trả lời đang chạy trong nhóm, chip.
- Maestro flow 49/50, `make parity`, compose: cần Docker.

### 2.4 Đã quyết — chủ sản phẩm chốt ngày 2026-09-27
Các mục dưới đây đã rời danh sách «cần quyết». Đây là quyết định của chủ sản phẩm, không phải chữ ký Lead.
- Bảy đề xuất ADR chấp nhận và chuyển khỏi `proposals/`. Bốn số trùng với `main` (đã có ADR-0037…0040)
  được đổi: đề xuất 0037 (engine) → **ADR-0044**, 0038 (hàng đợi và
  stream) → **ADR-0045**, 0039 (nhóm trong luồng) → **ADR-0046**, 0040 (RAG) → **ADR-0047**; ADR-0041,
  ADR-0042, ADR-0043 giữ số.
- Luồng AI trong chat là kiểu Meta AI trong Messenger: trong 1:1 hoặc nhóm, người gọi gắn `@Rủ Đi`, AI
  đọc đoạn hội thoại người gọi kèm theo (chip «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ») và trả lời
  trong luồng (ADR-0046 §8.1). 1:1 chưa làm — xem mục 3.
- Ngoại lệ `chatassist` đọc chữ tin đã lưu cho `chia_bill` (chỉ phòng này, chỉ tin người gọi chia sẻ,
  chỉ phòng legacy không E2EE): chấp nhận (ADR-0046 §8.3).
- Luật allowlist `aggregate-base64-fragments` cho `services/core/go.sum`: chấp nhận (ADR-0043 §7).
- Thứ tự merge: PR #654 vào `main` ngay; chat 1:1 đi PR mới. §7.1 của ADR-0046 thôi chặn merge;
  Maestro 49 trên máy thật và ảnh chụp thành việc tiếp theo, vẫn bắt buộc trước khi dựa vào cờ ở
  production (ADR-0046 §8.2).

### 2.5 Quyết định của Lead / chủ sản phẩm còn mở
- Mục tiêu «token đầu p50 ≤2,5 s» giờ tính cho chữ đầu **đã qua verifier** (gần cuối lượt): chấp nhận hay
  đổi thiết kế.
- Milvus GC `dropTolerance` và bản backup/snapshot so với lời hứa «quên»; trần chữ nhóm 1500.
- MILCO: đang gác. Muốn dùng lại thì đọc giấy phép `omai-research/milco-650m` và `naver/splade-v3` trên
  HuggingFace trước.
- Pháp lý cho trí nhớ cá nhân hoá (Luật BVDLCN 2025, Nghị định 356/2025); ADR-0041 còn trích Nghị định 13/2023.

### 2.6 Dữ liệu đo
- Viết bộ niêm phong mới (≥220 câu/lớp) cho router: bộ tiền/dị ứng cũ đã lộ, chỉ còn dùng bắt hồi quy.
- Bộ tiếng Việt cho truy hồi (có dấu, không dấu, teencode), cho trí nhớ, và ≥150 nhãn người để hiệu chỉnh
  judge. Mọi số golden hiện đo bằng embedding giả.

## 3. Còn mở phía code (tôi hoặc người làm tiếp)
- **Việc kế tiếp: Rủ Đi AI trong chat 1:1** (ADR-0046 §8.1), PR mới sau khi PR #654 vào `main`. Hiện
  `chatassist` từ chối ngữ cảnh kind `pair` bằng `409 group_plan_only` và client không gửi lời nhờ trong
  cặp. Lát phải tự có bằng chứng riêng của cặp (sổ đôi ADR-0027, `chia_gu` chỉ theo ADR-0034).
- Maestro 49 trên máy thật, flow 30/40 trên máy, ảnh chụp sáng/tối/Reduce Motion (ADR-0046 §8.2):
  không chặn merge nữa, vẫn bắt buộc trước khi dựa vào cờ ở production.
- Lát 12 (người khác trong phòng thấy chữ chạy qua frame WS) — đang làm.
- Review phản biện lát 9, lát 11 vòng 2 và phần mobile — đang làm.
- Client chưa gửi lệnh `hoi` cho nhóm; `set_reminder`; nhắc chủ động (lát 17); tín hiệu 👍/👎 (lát 18);
  E2EE v2 cho AI (lát 20); gỡ action Python sau khi bật cờ (lát 19).
- Xoá tài khoản chưa xoá 5 cột người dùng thuộc các gói chat khác.
- Các tính năng ADK v2 chưa dùng (graph workflow, compaction) — ghi ở hợp đồng §8.5.

## 4. Chạy các cổng
```bash
cd services/core && GOTOOLCHAIN=go1.26.8 go test ./...
scripts/go_postgres_tier.sh          # skip = đỏ
scripts/go_broker_tier.sh            # Redis + RabbitMQ
scripts/go_milvus_tier.sh            # Milvus + reranker (MOBILE_TEST_MILVUS_ADDR, MOBILE_TEST_RERANK_URL)
scripts/ai_infer_tier.sh             # sidecar, offline
scripts/gate.sh eval-kich-ban        # T1 Nếp + nhóm
python3 scripts/repo_guard.py tree HEAD
```
