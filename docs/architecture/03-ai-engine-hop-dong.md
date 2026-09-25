# Engine AI v2 — hợp đồng chung giữa các mảng, tiến độ và cổng

Ngày bắt đầu: 2026-09-25. Commit gốc: `f251db7`. Nhánh: `claude/peaceful-hopper-32kwjs`.
Quyết định: đề xuất ADR-0037…0042 (`docs/decisions/proposals/`), **chờ Lead ký**. Ghi chú yêu
cầu của người dùng: `docs/claude/2026-09-25/ai-chat-v2-ghi-chu.md`. Thiết kế chi tiết từng mảng:
`docs/claude/2026-09-25/thiet-ke-ai/01…06`. Không phải bằng chứng phát hành.

Tài liệu này là **nguồn sự thật cho mọi thứ nhiều mảng cùng chạm vào**. Một thiết kế mảng lệch
với bảng dưới đây thì bảng dưới đây thắng. Muốn đổi thì sửa ở đây trước, trong cùng commit với
thay đổi mảng.

## 1. Mục tiêu

Hai con AI dùng chung một engine nhưng đóng hai vai:
- **Rủ Đi AI** trong nhóm;
- **Nếp**, trợ lý riêng.

Mục tiêu là cả hai **thật sự thông minh**, đo được theo thang M0–M4 ở mục 6, không phải chỉ nhìn đẹp hơn.

## 2. Pipeline (một cho cả hai bot, chính sách theo bot)

```
client ──POST invocation──► Go handler (auth, kiểm gói, idem, hạn mức) ── INSERT job + outbox (1 tx)
                                  │ 202 {id}   ← task id
outbox relay ──► RabbitMQ (lane ai.group / ai.nep / memory / notify, DLQ) ──► `core work`
   (RabbitMQ chết → poller SKIP LOCKED tiếp quản; không mất job)
worker: aiharness.Engine.Run(turn, sink)
  1 Tiền xử lý tất định: NFC, bỏ @mention, «bây giờ» Asia/Ho_Chi_Minh từ created_at,
    giải ngày tương đối (domain/thoigian), bảng tham chiếu R0..R7, teencode chỉ cho bản định tuyến
  2 Guard tất định trên MỌI nguồn không tin cậy; luật tiền
  3 Understand: MỘT lời gọi flash-lite, output toàn enum + slot (nhãn guard, 1–3 ý định, slot,
    có cần hỏi lại). Đầu ra được bọc <du_lieu> và qua guard lần nữa. Truy hồi chạy suy đoán song song.
  4a Fast path (một ý định find_places / app_help / explain_screen): Go gọi retriever thẳng từ slot
  4b Agent loop ADK-Go: toolset lọc theo bảng quyền; ≤4 bước (Nếp ≤3); ≤10 tool call; bước cuối Mode NONE
  5 Hậu xử lý: token [[p:ID]] → tên từ ledger; GroundCard / GroundReply trên đúng tập tool đã trả;
    provenance; output guard cửa sổ 48 ký tự (nơi DUY NHẤT sinh delta)
  sink → Redis Streams → SSE /events (người gọi, Nếp) + frame WS `chatlegacychange` (người xem khác)
  publish đúng một lần trong một transaction
```

## 3. Hợp đồng

| Mục | Chốt |
|---|---|
| Số ADR | 0037 engine · 0038 hàng đợi và stream · 0039 nhóm trong luồng · 0040 RAG và nạp dữ liệu · 0041 Nếp (phiếu v2, trí nhớ, nhắc) · 0042 bộ đo chất lượng. Nếu main đã dùng số nào thì lấy số trống kế tiếp, lúc vào main |
| Số migration | **Không đặt trước.** Cấp theo thứ tự lên main. `serve`/`work` từ chối chạy khi version < N. Gói mới (`aiharness`, `jobs`, `rag`, `nepnho`, `nepnhac`, `push`) có bảng version riêng theo mẫu `services/core/internal/chatassist/migrate.go`, nên không giành số của `chatassist` |
| Model | `gemini-3.5-flash-lite` cho mọi bước sinh chữ, gọi từ Go qua `google.golang.org/genai`. Embedding: `gemini-embedding-001`, 768 chiều |
| Trần lời gọi model | Hằng `MaxModelCallsPerTurn` = 8 (trần cứng) trong `aiharness/llm`, đếm cả retry (retry riêng của genai tắt, chỉ `aiharness/llm` tự thử). Mục tiêu p95 ≤4 lời gọi mỗi lượt. Từ lát 10 bộ đếm nằm nguyên tử trên hàng job (`model_calls`), đúng qua mọi lần thử. Embedding đếm riêng: `MaxEmbedCallsPerTurn` = 2. Eval đọc hai hằng này để dự toán. Hạn 8 lời gọi/phút/người vẫn đếm theo invocation, và có thêm limiter theo từng lời gọi model |
| Sự kiện stream (enum đóng, gói `aistream`) | `hello`, `trang_thai{cau}`, `phan{kind,json}`, `delta{p,text}`, `lam_lai`, `xong{message_id \| text,chips,nguon}`, `that_bai{code}`, `huy`, `thu_hoi`, `ket_noi_lai`, cộng dòng `: ping`. Không có sự kiện «rút lại»: guard chặn trước khi phát |
| Đường stream | SSE `GET /contexts/{c}/ai-invocations/{id}/events` và `GET /me/nep/ai-invocations/{id}/events` cho người gọi. Người xem khác trong phòng lane cũ nhận frame `ai` trên WS `chatlegacychange` sẵn có. Frame này bật bằng một trường trong frame authenticate, để client cũ không vỡ. Phong bì frame mang `tin` (id lời gọi) và `so_tin` (số tin đã đọc), vì `trang_thai{cau}` không có chỗ cho con số. Capability: một trường `ai.stream` ∈ {`phong`, `nguoi_goi`, `khong`}. Phòng v2: key phòng **không bao giờ** được ghi; `lane` do máy chủ tự suy |
| Thẻ nhóm | `ai_card` kind `tra_loi` `{ban, tac_gia:"rudi-ai", invocation_id, lenh, doc{so_tin, chi_loi_nho}, phan[≤3: text/places/itinerary/expense_draft], outing_id?}`. Author trong DB là NULL (sống qua E2EE). `reply_to_id` là tin tag. Kiểm bằng `GroundReply` mới; `GroundCard` giữ nguyên cho oracle |
| Registry tool (một nơi: `aiharness/tools`) | `search_places`, `get_place`, `list_destinations`, `nearest_area`, `group_snapshot`, `list_group_outings`, `search_app_manual`, `explain_screen`, `propose_places`, `propose_itinerary`, `draft_poll`, `suggest_screen`, `my_upcoming_outings`, `recall_memory`, `remember_fact`, `forget_fact`, `set_reminder`. Bot nào gọi được tool nào do `quyen.golden.json` quyết |
| Quyền tool | Tool đọc chạy trong tx ReadOnly. Tool nháp không chạm DB. Không tool nào ghi tiền, nghĩa vụ, hay chốt kèo. `recall_memory`/`remember_fact`/`forget_fact`/`set_reminder` chỉ tới được từ gốc scope=me |
| Trí nhớ Nếp | Gói `nepnho` là writer duy nhất. «Quên» là **xoá cứng** kèm tombstone băm. Fact bị thay hoặc hết hạn xoá ngay khi củng cố. Chỉ trích từ lời của chính người dùng; từ chối fact về người khác. Xoá qua **một trigger Go trên `people.deleted_at`** phủ mọi bảng Go có `person_id` |
| Quan sát | Chỉ id, enum, số đếm, thời gian. Không nội dung, không tham số tool, không nhãn nhạy cảm gắn với người. Không cài OTel provider toàn cục |
| Latency | Sự kiện trạng thái đầu tiên (animation) p95 ≤300ms. Token chữ đầu p50 ≤2.5s, p95 ≤5s. Plan tổng p95 ≤8s |

## 4. Các lát (thứ tự; đường găng 0 → 3 → 4 → 5 → 6 → 8 → 9 → 11 → 12)

| # | Lát | Trạng thái |
|---|---|---|
| 0 | Ghi chú, hợp đồng này, thiết kế 01–06, đề xuất ADR-0037…0042 | xong (commit này) |
| 1 | Sửa nhanh: route `chatassist` vào manifest; «Vẽ» của Nếp kiểm màn tiền; ảnh Nếp tải có header | xong: `2d12c58` (Vẽ + ảnh), `bd923e6` (22 route chỉ-Go vào khối `features`) |
| 2 | Eval M0: harness cũ 16 ca × 5 lần, có khoảng tin cậy (cần Lead duyệt 80 lời gọi) | công cụ xong `6aa2335`; lượt thật chờ Lead duyệt 80 lời gọi |
| 3 | Go 1.23.4 → 1.25 | xong: `637b7f3` (1.25.14) |
| 4 | Tách worker: `claimByID`, heartbeat, `core work`, pool riêng | xong: `d596621` (pool riêng và semaphore tool để lát 6/10) |
| 5 | Cổng đọc xuyên gói (`go/packages`), phải có trước khi engine chuyển code | xong: `71fb311` (`internal/aigate`) |
| 6 | Engine S1: Nếp qua Go, chưa stream; `Engine.Run`; eval T1 trong CI | **chưa xong** — tách hai phần, lát 6 chỉ xong khi cả hai xong và review phản biện chấp nhận. **6a (engine)**: `0a752a7` + commit sửa theo review (`7bde12b`). `Sink` đúng thiết kế 01 §2 (engine không phát xong/thất bại); «bây giờ» có test đỏ được; khoá Gemini chỉ vào core khi ghép rõ `docker-compose.nep-go.yml`. Review lại trả REQUEST_CHANGES: luật tiền viết lại khớp corpus 157+104 câu mà tác giả đã đọc (không phải số giữ riêng), trên 87+80 câu mới của reviewer bắt 66/87 nhưng chặn nhầm 14/80 câu hỏi quán/ngân sách; output guard để lọt «Mình đã gửi cho bạn 200k». **6b (eval T1)**: commit gộp ngay sau `7bde12b` — `internal/aieval`, `cmd/rudi-eval --mo-hinh kich-ban`, corpus Nếp 27 ca, `scripts/eval_kich_ban.sh`, chặng `eval-kich-ban` và job CI; T1 xanh tại máy, canary đỏ đúng `khong_bia_dia_diem`; job CI chưa thấy chạy trên Actions; T1 chưa thấy phần SĐT/trích lời nhắc của output guard. **Cờ `MOBILE_AI_ENGINE_NEP` ở `brain`** tới khi luật tiền đạt số trên corpus giữ riêng thật, ADR-0037 được ký và review bảo mật khoá trong core xong. `aiboicanh` dời sang lát 9. `make parity` chưa chạy (không có Docker) |
| 7 | Nhóm trong luồng (lõi, còn đi brain): tin @ là tin thường, chip, `tra_loi`, `reply_to` | **chưa xong, không vào main** trước khi Lead ký ADR-0039 (ADR-0039 §7.1). Lõi `5af8655`, sổ tay `603515f`; review phản biện ra `REQUEST_CHANGES` (10 phát hiện), đã sửa ở commit sửa lát 7 (review lại: APPROVE, 3 phát hiện nhỏ): khoá chéo publish ↔ xoá tin/thả cảm xúc (giờ head → tin tag → job, có test đua Postgres), replay trước kiểm tin tag, thử lại xét còn thử được trước hạn phòng, dòng trích khớp máy chủ theo một tệp vector chung, flow 30 hai nhánh AI và máy kiểm sau flow 40 đọc `tra_loi`. **Còn mở:** Maestro 49 chưa viết, flow 30/40/49 chưa chạy trên máy, chưa mở ảnh chụp (sáng, tối, Reduce Motion); tin @ có thể không được trả lời mà không có dấu hiệu nào khi rời màn lúc tin đang gửi — mục «Nhờ Rủ Đi AI trả lời tin này» chưa làm (§7.4); thành viên dùng app cũ thấy «Một thẻ bản này chưa hiển thị được.» (§7.3, chấp nhận) |
| 8 | RAG S1 từ vựng; `thoigian`, `giomo`, `Fold`, `SafeDeep`; sửa lỗi quán mặc định Đà Lạt trên đường Go | đã gộp, **chưa xong**: `giomo` `2acd75b`, `thoigian` + `Fold` trong `0a752a7`, `rag/xephang` `544ebc7`, gói `rag` + `tuvung` + `SafeDeep` + shortlist `/places/search` (commit gộp lát 8). Review phản biện REQUEST_CHANGES: blocker — dị ứng nói bằng nhãn một âm tiết («dị ứng cua/trứng/sữa/mè…») không được đọc, tập vàng di_ung viết né; nhãn ăn kiêng gắn cho quán nói ngược; trường bị cách ly làm mất từ dị ứng; hai tính chất có test không đỏ được. Sửa lỗi «luôn Đà Lạt» trên engine Go còn ở lát 9; compose chưa có `migrate-rag` |
| 9 | Engine S2: nhóm qua Go, understand, fast path, agent, `chia_bill` port; cổng ≥14/16 | chưa |
| 10 | Hàng đợi: outbox, RabbitMQ, poller dự phòng, tác vụ định kỳ, limiter theo lời gọi | một phần: gói `jobs` `d76a0a4`, tầng broker `7246744`; chưa nối vào `chatassist` (migration, trigger, consumer, `retryLater`, định kỳ, limiter) |
| 11 | Stream SSE; bảng Nếp mới và animation (`/impeccable`, sửa `DESIGN.md`) | một phần: gói `aistream` `9bb26b0`, client `ai/sse.ts` `aab71de`; chưa có route `/events`, writer trong worker, cửa sổ 48 ký tự, UI |
| 12 | Stream cả phòng qua frame WS; UI nhóm hoàn thiện | chưa |
| 13 | Nếp tại chỗ: phiếu v2, sổ tay app, chip, tool phía máy chủ | một phần: dữ liệu sổ tay 13 màn + cổng lệch `e69012b`, sửa theo lát 7 `603515f`; gói thuần `internal/huongdan` (`TheoMan`, `Tim` qua `rag/xephang`, `DuongToi` BFS ≤5 bước, `BanDung` + hằng `nep/huong-dan-ban.ts`, bộ vàng 91 câu recall@5 0.9725) đã gộp nhưng review phản biện REQUEST_CHANGES: luật màn tiền lách được bằng một `di_toi` về chính màn đó; nhóm teencode recall@5 0.8333 dưới ngưỡng và chưa ghim theo nhóm. Chưa: phiếu v2 (`buoc`, `hanhDong`, `banBuild`), tool `search_app_manual`/`explain_screen` (lát 9), chip, `useNepMoc`, sửa câu gợi ý |
| 14 | Chia bill từ thẻ, dấu «Đã ghi vào sổ» suy từ dòng chi tiêu thật | chưa |
| 15 | Trí nhớ Nếp | chưa |
| 16 | RAG vector, làm giàu, độ tươi | chưa |
| 17 | Nhắc chủ động: trong app, rồi push | chưa |
| 18 | Eval M3 và tín hiệu phản hồi | chưa |
| 19 | Gỡ action Python `companion-reply`/`nep-reply` (một commit cùng manifest) | chưa |
| 20 | E2EE v2 (chat-lab) | chưa |

## 5. Cổng mỗi lát (CLAUDE.md)

- Chạy lại trong cây sạch đúng SHA.
- Canary đỏ ở chỗ đã dự đoán; identity xanh.
- Ít nhất hai đột biến tự nghĩ, đã kiểm tương đương trước, mỗi cái đỏ đúng bước đã dự đoán.
- Với UI: mở ảnh chụp ra nhìn (sáng, tối, Reduce Motion).
- Số đo viết vào commit message tiếng Việt.
- Lời gọi model thật cần Lead duyệt số lượng từng lần (ADR-0034 §2.6).

## 6. Thang chất lượng

| Mốc | Nghĩa |
|---|---|
| M0 | Đường nền thật trên harness cũ, có khoảng tin cậy |
| M1 | Đo được: T0/T1 xanh trên engine Go |
| M2 «an toàn» | ASR hệ thống 0/≥80; luật tiền 0; lộ trí nhớ chéo người 0; bịa id quán 0 |
| M3 «thật sự thông minh» | plan lõi ≥14/16 ổn định qua 5 lần, pass@1 ≥0.85 (cận dưới ≥0.78); định tuyến ≥0.92; truy hồi quán recall@10 ≥0.90; grounding ≥0.98; Nếp hướng dẫn đúng bước ≥0.85, bịa UI ≤2%; trí nhớ ≥0.90, quên 100% trong store; latency như mục 3 |
| M4 | 14 ngày online: 👎 ≤8%, thử lại ≤5% |

## 7. Việc cần người/Lead quyết

- Ký ADR-0037…0042 và phần sửa `DESIGN.md` (animation «đang nghĩ» của Nếp).
- Ngân sách lời gọi model thật: M0 (80), mỗi lần T3, và việc soạn nháp sổ tay app.
- Host production cho RabbitMQ và Redis; FCM credentials cho push.
- Trần chữ cho câu trả lời nhóm (đề xuất 1500).
- Có cho `dieu_da_dan` giữ điều dị ứng do chính người dùng nói không.
- Judge cùng họ model với generator: chấp nhận rủi ro tự thiên vị, chỉ giảm bằng hiệu chuẩn κ.
- `/impeccable` không có trong session cloud: các lát UI (11, 12, 13) chạy ở nơi có skill.
