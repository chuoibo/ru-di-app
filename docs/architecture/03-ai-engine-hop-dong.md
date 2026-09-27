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
| Model | `gemini-3.5-flash-lite` cho mọi bước sinh chữ, gọi từ Go qua `google.golang.org/genai`. Embedding: **chỉ** `gemini-embedding-2`, 1536 chiều cho mọi collection (quyết định chủ sản phẩm 2026-09-27); trên Developer API tác vụ viết vào văn bản bằng tiền tố, và tiền tố chỉ được thêm **một lần, trong `aiharness/nhung`** (nạp, truy hồi và kho ví dụ của router đều đi qua cửa đó) |
| Trần lời gọi model | Hằng `MaxModelCallsPerTurn` = 8 (trần cứng) trong `aiharness/llm`, đếm cả retry (retry riêng của genai tắt, chỉ `aiharness/llm` tự thử). Mục tiêu p95 ≤4 lời gọi mỗi lượt. Từ lát 10 bộ đếm nằm nguyên tử trên hàng job (`model_calls`), đúng qua mọi lần thử. Embedding đếm riêng: `MaxEmbedCallsPerTurn` = 2. Eval đọc hai hằng này để dự toán. Hạn 8 lời gọi/phút/người vẫn đếm theo invocation, và có thêm limiter theo từng lời gọi model |
| Sự kiện stream (enum đóng, gói `aistream`) | `hello`, `trang_thai{cau}`, `phan{kind,json}`, `delta{p,text}`, `lam_lai`, `xong{message_id \| text,chips,nguon}`, `that_bai{code}`, `huy`, `thu_hoi`, `ket_noi_lai`, cộng dòng `: ping`. Không có sự kiện «rút lại»: guard chặn trước khi phát |
| Đường stream | SSE `GET /contexts/{c}/ai-invocations/{id}/events` và `GET /me/nep/ai-invocations/{id}/events` cho người gọi. Người xem khác trong phòng lane cũ nhận frame `ai` trên WS `chatlegacychange` sẵn có. Frame này bật bằng một trường trong frame authenticate (`"ai": true`), để client cũ không vỡ. Phong bì frame mang `inv` (id lời gọi), `tin` (id tin tag mà câu trả lời nằm dưới) và `so_tin` (số tin đã đọc), vì `trang_thai{cau}` không có chỗ cho con số (§4.2). Capability: một trường `ai.stream` ∈ {`phong`, `nguoi_goi`, `khong`}. Phòng v2: key phòng **không bao giờ** được ghi; `lane` do máy chủ tự suy |
| Thẻ nhóm | `ai_card` kind `tra_loi` `{ban, tac_gia:"rudi-ai", invocation_id, lenh, doc{so_tin, chi_loi_nho}, phan[≤3: text/places/itinerary/expense_draft], outing_id?}`. Author trong DB là NULL (sống qua E2EE). `reply_to_id` là tin tag. Kiểm bằng `GroundReply` mới (phần chữ ≤1500 rune cắt ở ranh giới câu, chờ Lead chốt; `expense_draft` chỉ `so_khoan` 1..8 và `da_ghi`); `GroundCard` giữ nguyên cho oracle |
| Registry tool (một nơi: `aiharness/tools`) | `search_places`, `get_place`, `list_destinations`, `nearest_area`, `group_snapshot`, `list_group_outings`, `search_app_manual`, `explain_screen`, `propose_places`, `propose_itinerary`, `draft_poll`, `suggest_screen`, `my_upcoming_outings`, `recall_memory`, `remember_fact`, `forget_fact`, `what_you_remember`, `set_reminder`. Bot nào gọi được tool nào do `quyen.golden.json` quyết (mục 8) |
| Quyền tool | Tool đọc chạy trong tx ReadOnly. Tool nháp không chạm DB. Không tool nào ghi tiền, nghĩa vụ, hay chốt kèo. `recall_memory`/`remember_fact`/`forget_fact`/`set_reminder` chỉ tới được từ gốc scope=me |
| Trí nhớ Nếp | Gói `nepnho` là writer duy nhất. «Quên» là **xoá cứng** kèm tombstone băm. Fact bị thay hoặc hết hạn xoá ngay khi củng cố (`DinhKyDon` mở xoá `mot` cho fact quá `den_luc`). Chỉ trích từ lời của chính người dùng (đoạn nguyên từ của tin nhắn lượt này); từ chối fact về người khác. Xoá qua **một trigger Go trên `people.deleted_at`** phủ mọi bảng Go có `person_id` |
| Quan sát | Chỉ id, enum, số đếm, thời gian. Không nội dung, không tham số tool, không nhãn nhạy cảm gắn với người. Không cài OTel provider toàn cục |
| Latency | Sự kiện trạng thái đầu tiên (animation) p95 ≤300ms. Token chữ đầu p50 ≤2.5s, p95 ≤5s. Plan tổng p95 ≤8s |

## 4. Các lát (thứ tự; đường găng 0 → 3 → 4 → 5 → 6 → 8 → 9 → 11 → 12)

| # | Lát | Trạng thái |
|---|---|---|
| 0 | Ghi chú, hợp đồng này, thiết kế 01–06, đề xuất ADR-0037…0042 | xong (commit này) |
| 1 | Sửa nhanh: route `chatassist` vào manifest; «Vẽ» của Nếp kiểm màn tiền; ảnh Nếp tải có header | xong: `2d12c58` (Vẽ + ảnh), `bd923e6` (22 route chỉ-Go vào khối `features`) |
| 2 | Eval M0: harness cũ 16 ca × 5 lần, có khoảng tin cậy (cần Lead duyệt 80 lời gọi) | công cụ xong `6aa2335`; lượt thật chờ Lead duyệt 80 lời gọi. Đường Go tương ứng cho T3 (nhánh `track-e-eval-that`, chưa vào main): `scripts/eval_that.sh --bo <bộ> --tran-goi N` chạy `rudi-eval --mo-hinh that` rồi phát lại cùng SHA với 0 lời gọi; sổ tay `docs/claude/2026-09-25/chay-model-that.md`. **Chưa có lượt thật nào**: chờ `GEMINI_API_KEY` trong cài đặt môi trường và N Lead duyệt |
| 3 | Go 1.23.4 → 1.25 | xong: `637b7f3` (1.25.14); nâng tiếp go1.26.8 cùng ADK v2 (§8.5) |
| 4 | Tách worker: `claimByID`, heartbeat, `core work`, pool riêng | xong: `d596621` (pool riêng và semaphore tool để lát 6/10) |
| 5 | Cổng đọc xuyên gói (`go/packages`), phải có trước khi engine chuyển code | xong: `71fb311` (`internal/aigate`) |
| 6 | Engine S1: Nếp qua Go, chưa stream; `Engine.Run`; eval T1 trong CI | **chưa xong** — tách hai phần, lát 6 chỉ xong khi cả hai xong và review phản biện chấp nhận. **6a (engine)**: `0a752a7`, sửa review vòng 1 `7bde12b`: `Sink` đúng thiết kế 01 §2 (engine không phát xong/thất bại); «bây giờ» có test đỏ được; khoá Gemini chỉ vào core khi ghép rõ `docker-compose.nep-go.yml`. **6b (eval T1)**: `84e3c31` — `internal/aieval`, `cmd/rudi-eval --mo-hinh kich-ban`, corpus Nếp, `scripts/eval_kich_ban.sh`, chặng `eval-kich-ban` và job CI; T1 xanh tại máy, canary đỏ đúng `khong_bia_dia_diem`; **job CI chưa thấy chạy trên Actions**. Review vòng 2 (trên `7bde12b` và `84e3c31`) trả REQUEST_CHANGES; sửa ở `982ec8e`: luật tiền ưu tiên độ chính xác (câu hỏi quán/ngân sách không bị chặn trước lời gọi model), đo trên corpus DEV v2 và 35 câu của review, cách phòng thủ nhiều lớp ghi ở ADR-0037 §4; output guard chặn lại «Mình đã gửi cho bạn …»; model Gemini vẫn báo backend Gemini API; T1 thấy phần số điện thoại, email, số tài khoản và trích lời nhắc của output guard. Review vòng 3 (trên `982ec8e`) trả REQUEST_CHANGES: corpus niêm phong v2 đo luật `982ec8e` được recall 131/150 (KTC 95% 0,811–0,917), bắt nhầm 2/136 (0,004–0,052) — chưa đạt, và corpus đó đã dùng hết (review liệt kê nguyên văn mọi câu trượt). B1 (số điện thoại viết theo cặp gạch ngang lọt output guard) sửa ở `8e14975`; B2 (tên người sau động từ tiền bị đọc thành địa điểm hay cụm từ: «Gửi Quân 200k», «Trả Gia 100k»), M1 (output guard: «gửi cho bạn link chuyển khoản 200k», «đã gửi Nam 200k», «đã giúp bạn chuyển …»; T1 thêm ca 32–33), M2 (ADR-0037 §4 xét ngưỡng theo cận KTC 95% trên corpus niêm phong ≥ 220 câu mỗi lớp; ghi chú DEV thôi trỏ tới generator) và ba nit N-a/N-b/N-c sửa ở commit ngay sau `849664a`. **Cờ `MOBILE_AI_ENGINE_NEP` ở `brain`** tới khi: trên một corpus niêm phong mới (≥ 220 câu mỗi lớp, không generator nào với tới được từ DEV), do người khác đo, cận dưới KTC 95% của recall ≥ 0,95 và cận trên của tỉ lệ bắt nhầm ≤ 0,02 (ADR-0037 §4), job CI T1 thấy chạy xanh, ADR-0037 được ký và review bảo mật khoá trong core xong. `aiboicanh` dời sang lát 9. `make parity` chưa chạy (không có Docker). **Vòng 3–4 (2026-09-25)**: `f587575` sửa tên người bị đọc thành quán; review vòng 4 đo trên corpus niêm phong v3 (260 câu tiền, 245 câu không phải tiền, người sửa không mở): recall 221/260 = 0,850 (KTC 0,802–0,888), bắt nhầm 7/245 = 0,029 (KTC 0,014–0,058) — trượt cả hai cận; REQUEST_CHANGES vì bản sửa thêm hồi quy cùng loại B2 (chặn nhầm câu hỏi quán/kế hoạch, «chuyen 3 tram cho nam» lọt). **Quyết định**: luật tiền tất định đóng băng về recall (chỉ sửa giảm bắt nhầm và hồi quy); ADR-0037 §4 đổi ngưỡng bật cờ thành (a) luật một mình: cận trên bắt nhầm ≤ 0,02, (b) hợp luật + bộ phân loại tiền của Understand (lát 9, lời gọi thật do Lead duyệt): cận dưới recall ≥ 0,95. **Vòng sửa 4** (commit ngay sau `5a97262`; luật chỉ được hẹp lại hoặc sửa hồi quy): R1 «chuyen 3 tram cho nam» lại bị từ chối như ở `849664a` («cho» không dấu không bao giờ là «chỗ»); R2 thu hẹp mọi cách đọc chặn nhầm câu hỏi quán/kế hoạch («ai lo/chịu phần» phải có tiền, «tab» phải có số tiền hay là tab của một người, «lương» đọc theo dấu và phải có người nhận hay số tiền, mua/đặt giúp cả nhóm chỉ khi thu/đòi lại tiền, cụm ghép xét từng từ, «cành» chỉ ở chỗ số tiền đứng được); R-pre «báo/bảo trước» không còn là «bao trước»; R3 output guard đọc «tra» theo dấu và không coi quán/kèo giữa động từ và số tiền là tự nhận; nit: tự nhận sau mệnh đề «gửi cho bạn …», không chủ ngữ, «ghi Nam nợ bạn 200k»; số điện thoại gạch ngang dài, gạch dưới, chấm cách; tọa độ chỉ qua khi là cặp vĩ/kinh độ ≤ 6 chữ số lẻ; T1 thêm ca 36 (ký tự ẩn trong từ tiền, 0 lời gọi) và 37 («gửi cho bạn 200k» bị chặn). Probe của reviewer: câu tiền 0/4 → 4/4, bắt nhầm 27/39 → 0/39; niêm phong v2 149/150 → 150/150, bắt nhầm 0/136; giá phải trả: 1 câu tiền tự viết («Ai chịu phần bánh sinh nhật của Hoa»). Cờ vẫn `brain`; niêm phong v3 chờ người khác đo lại trên SHA mới. **Vòng 5 (`b624ae1`)**: chỉ thu hẹp luật tiền theo chính sách đóng băng; trên corpus niêm phong v3 bắt nhầm 5/245 = 0,0204 (KTC 0,0087–0,0469; trước 7/245), recall 221/260 không đổi — §4(a) vẫn trượt (245 câu cần 0/245). Review REQUEST_CHANGES: output guard nay để lọt câu tự nhận đã trả tiền cho quán («Mình đã gửi quán 200k tiền cọc») mà bản trước chặn; còn vài bắt nhầm cùng loại dd61637. **Vòng sửa 5** (commit ngay sau `d65a03e`; sửa review vòng 5 của `b624ae1`, bản gốc `69a677a`): NEW-1 output guard lại chặn câu tự nhận trả tiền cho quán — sau «chuyển/đưa/bắn/gửi», quán viết đúng dấu giữa động từ và số tiền chỉ được miễn khi «sang/qua» theo ngay động từ hoặc sau số tiền là chỗ xếp quán («lên/xuống/vào … danh sách», «gần», «ở»); tiền, cọc, «rồi», hết câu vẫn chặn (12 câu của reviewer 0/12 → 12/12; 13 câu R3 vẫn qua); NEW-2 «tra» là tra cứu chỉ khi ngay sau là từ tra cứu viết đúng dấu, không theo dấu cả câu (0/5 → 5/5); NEW-3 luật tiền chỉ hẹp lại: «khoản» là tiền khi sau nó có tiền, số tiền hay «này/đó»; lương sau «cho/trước» cần số tiền, «giúp» hoặc hết câu; «my/his tab» cần số tiền (probe của reviewer bắt nhầm 13/22 → 0/22, kể cả «bào trước» và «Chuyen di Nha Trang»); NEW-4 bốn câu chặn nhầm (câu điều kiện «… thì …», «gửi xe», «nó») qua; NEW-5 test giết N2, N11; nit: số điện thoại chấm lệch một bên và chấm giữa bị chặn; T1 thêm ca 38 (tự nhận chuyển tiền cọc cho quán). Recall trên mọi corpus nhìn thấy không đổi, 0 câu mới bị từ chối. Cờ vẫn `brain`; niêm phong v3 chờ người khác đo lại trên SHA mới. **Tích hợp lõi agent không heuristic (nhánh `agent-core/integrated`, chưa review, chưa vào main)**: `Engine.Run` của Nếp chỉ còn đường router, đường cũ bị gỡ: màn tiền (kiểm cấu trúc route) → tiền xử lý cấu trúc (NFC, ký tự ẩn, mention) → `hieu` (một lời gọi có schema: nhãn guard, lớp tiền, ý định, đường, ràng buộc, ngày ISO) → `money_action`/`split_draft` ra câu cố định sau đúng 1 lời gọi; `chen_lenh` giữ chữ làm dữ liệu nhưng chỉ còn tool đọc; hỏi lại một câu → đường truy hồi (`crag` chấm + tối đa một vòng sửa do model chọn, rồi `traloi` trả lời có cấu trúc, grounding thuộc tập, verifier, sinh lại một lần) / trả lời thẳng / `tactu` (đường nhanh `explain_screen` hoặc vòng ADK, bước giữa `AUTO`, bước cuối `NONE`) → mọi văn xuôi phát ra qua kiểm cấu trúc rồi verifier LLM ở ngữ cảnh mới (verifier mang phán đoán «tự nhận hành động» và tiền; đầu ra verifier hỏng thì không phát). Gỡ khỏi đường quyết định **và xoá**: `guard.LaTien` (`guard/tien.go` cùng hai file test), `guard.Nghi` (mẫu từ khoá chèn lệnh), luật cụm từ «tự nhận» của output guard, `preprocess.DongMayChu` và `thoigian.Giai` (đọc ngày bằng từ khoá). Output guard chỉ còn kiểm cấu trúc: mã kiểm, trích nguyên câu lời nhắc, định dạng số điện thoại/email/số tài khoản/thẻ (kiểm định dạng, không đọc nghĩa). Câu viết tay của năm vòng review guard giữ làm dữ liệu đo ở `aieval/testdata/hieu/nguon/` (`guard_cu_cau.json`, `tien_dev_v2.json`, `tien_giu_rieng.json`). Hệ quả: luật tiền tất định và ngưỡng bật cờ §4 của ADR-0037 không còn đối tượng; tiền giờ là nhãn của router, đo bằng bộ T3 `tien_v2/v3` khi có khoá và Lead duyệt số lời gọi — **cờ `MOBILE_AI_ENGINE_NEP` vẫn ở `brain`**. T1 Nếp phiên bản 2 (47 ca) chạy đủ chặng `hieu`/`tra_loi`/`cham`/`tra_loi_cau_truc`/`kiem` trên thế giới giả theo ca. `ai_turn_metrics` phiên bản 2 thêm 11 cột nhãn/đếm (nhãn guard, ý định đầu, số ý định, tiền, hướng, đường, tool đã chạy, vòng sửa, phán quyết verifier, sinh lại, số lần rerank). |
| 7 | Nhóm trong luồng (lõi, còn đi brain): tin @ là tin thường, chip, `tra_loi`, `reply_to` | **chưa xong, không vào main** trước khi Lead ký ADR-0039 (ADR-0039 §7.1). Lõi `5af8655`, sổ tay `603515f`; review phản biện ra `REQUEST_CHANGES` (10 phát hiện), đã sửa ở commit sửa lát 7 (review lại: APPROVE, 3 phát hiện nhỏ): khoá chéo publish ↔ xoá tin/thả cảm xúc (giờ head → tin tag → job, có test đua Postgres), replay trước kiểm tin tag, thử lại xét còn thử được trước hạn phòng, dòng trích khớp máy chủ theo một tệp vector chung, flow 30 hai nhánh AI và máy kiểm sau flow 40 đọc `tra_loi`. **Còn mở:** Maestro 49 chưa viết, flow 30/40/49 chưa chạy trên máy, chưa mở ảnh chụp (sáng, tối, Reduce Motion); tin @ có thể không được trả lời mà không có dấu hiệu nào khi rời màn lúc tin đang gửi — mục «Nhờ Rủ Đi AI trả lời tin này» chưa làm (§7.4); thành viên dùng app cũ thấy «Một thẻ bản này chưa hiển thị được.» (§7.3, chấp nhận) |
| 8 | RAG S1 từ vựng; `thoigian`, `giomo`, `Fold`, `SafeDeep`; sửa lỗi quán mặc định Đà Lạt trên đường Go | **một phần**: `giomo` `2acd75b`, `thoigian` + `Fold` trong `0a752a7`, `rag/xephang` `544ebc7`, gói `rag` + `tuvung` + `SafeDeep` + shortlist `/places/search` `a97b6e9`; sửa theo review phản biện vòng 1 ở `a609187` (dị ứng một âm tiết, ăn kiêng chỉ từ kinds/traits với phủ định rộng, dị ứng quét cả trường bị cách ly, savepoint/timeout và tombstone trên hàng sống có test đỏ được, cặp từ có dấu nối riêng — schema rag v2, eval thử khung giờ, tombstone tay chỉ `takedown`/`closed` + `untombstone`, service compose `migrate-rag`). Review lại `a609187` trả REQUEST_CHANGES (blocker N1: ba luật bỏ dị ứng khớp corpus giữ riêng làm sót dị ứng nói rõ; major N2: nhãn «Chay» của importer OSM không đọc được). Vòng sửa 2 là commit con trực tiếp của `a609187` (một commit không ghi được SHA của chính nó): bộ đọc dị ứng người hỏi viết lại theo nguyên tắc lưới an toàn (tối đa recall, đọc thừa là an toàn — thiết kế 04 §5.1), bỏ cả ba luật bỏ dị ứng, cổng cứng chỉ còn recall và violation@10; ăn kiêng quán đọc cả tên, nhãn nguyên trường «Chay», phủ định false/0/null/N/A/pending/nope/đóng cửa/chỉ vài ngày/ngoặc/gạch; dấu thanh quyết theo cả hàng; test takedown ở phần đệm shortlist có phiên bản active; `SafeDeep` xét cả khoá object; `core` chỉ đợi `migrate-rag` khởi động. Vòng sửa 2 là `71dd295`. Review lại `71dd295` (commit con của `a609187`; trên nhánh nằm dưới `be0f7a0`): trên nửa niêm phong của corpus dị ứng v2 (người sửa không mở) bộ đọc người hỏi đọc đủ 164/171 câu (95,9%; bản `a609187` 86/171), 0/14 câu giống bị đọc, quán sót dị nguyên 1/67, gắn nhầm ăn kiêng 1; nhưng REQUEST_CHANGES với hai blocker mới: câu «X thì mình dị ứng, còn Y thì ăn được» đọc Y và bỏ X (hồi quy so với `a609187`), và cụm «tôm mực» nuốt «tôm». Vòng sửa 3 là commit con trực tiếp của `be0f7a0` (một commit không ghi được SHA của chính nó): theo quyết định của Lead (thiết kế 04 §5.1), câu có từ kích dị ứng đọc HỢP mọi dị nguyên nêu trong câu, trước hay sau từ kích, qua «nhưng/còn/but» và qua cả yêu cầu; câu chỉ toàn phủ định không từ ngoại lệ thì không đọc; mọi cụm ở mọi vị trí được đọc nên cụm dài không nuốt cụm ngắn (test trên toàn danh sách); quán thêm «các loại hạt»…, «thịt/chung nồi/chung dầu» làm hỏng chay, «expired/không chứng nhận/hết hạn» làm hỏng halal; truy vấn ứng viên của chỉ mục luôn lập kế hoạch theo tham số (generic plan làm câu không nêu điểm đến vượt 800 ms và lùi về hàng sống); tập vàng thêm 46 câu di_ung và 3 quán neo của review vòng 3, violation@10 = 0 cả hai đường. **Chưa xong** vì năm việc: số mù của vòng sửa 3 là số đã đo (`8808fa4`, review vòng 4): trên corpus niêm phong v3 (339 câu người hỏi, 122 quán) bộ đọc người hỏi đọc đủ 258/266 câu (97,0%, KTC 94,2–98,5; `be0f7a0` 78,6%), đọc thừa ngoài danh sách chấp nhận 8/339, 0/53 câu giống bị đọc; quán sót dị nguyên 3/66 (`be0f7a0` 9/66), gắn nhầm ăn kiêng 1/122. REQUEST_CHANGES không blocker: dị ứng sau «;», xuống dòng hay câu kề bị bỏ; danh sách từ kích/ngoại lệ đóng («riêng tôm thì có», «avoid», «hives»); luật hợp đọc cả món được hỏi khi câu chỉ phủ định («không dị ứng gì nhưng muốn tìm quán hải sản» → hải sản); luật hợp cả câu giấu cả món được hỏi tìm trong câu (8/24 truy vấn di_ung có quán đúng mất quán đó: «dị ứng tôm, tìm quán ốc», «bánh ngọt…, dị ứng sữa») — Lead xác nhận giá này hay chỉnh luật; sửa lỗi «luôn Đà Lạt» trên engine Go (lát 9: engine gọi `rag.Retrieve`/`ResolveDestination`, `worker.go` thôi dùng `ModelPlaceRows`); Lead chưa xác nhận lệch Go-only của payload brain ở `/places/search` (ghi ở `docs/migration/live-go-25-route-wai.md`) — không vào `main` trước khi Lead xác nhận; và `make parity`, compose chưa chạy (không có Docker) **Tích hợp lõi agent (`agent-core/integrated`)**: phía câu hỏi của Nếp không còn đọc chữ bằng từ vựng hay bộ giải điểm đến: ràng buộc cứng (điểm đến là id trong danh sách đóng, dị ứng/ăn kiêng là id, ngân sách số nguyên đồng, giờ mở tính từ ngày ISO của router) do model trích, Go ép thành bộ lọc không nới qua `aidoc.Lexical` → `rag.Retrieve` (cờ `lexical_only`). `tuvung.DiUngNguoiHoi`, `rag.DocCau`, `rag/diemden` **còn giữ** vì `POST /places/search` (route parity, `routes/places_wai.go`) dùng — không thuộc đường engine; gắn nhãn phía nạp (`rag/chunk_place.go`) chờ hàng hạ tầng thay bằng làm giàu LLM. |
| 9 | Engine S2: nhóm qua Go, understand, fast path, agent, `chia_bill` port; cổng ≥14/16 | **Một phần.** Lõi Nếp (`agent-core/integrated`): router `hieu`, tool + vòng ADK `tactu`, CRAG + `traloi` + verifier. **Nhóm qua engine Go (nhánh `ai/group-engine`, sau cờ `MOBILE_AI_ENGINE_GROUP=go`, mặc định `brain`)**: xem §8.7. Chưa: T3 thật (cần khoá, Lead duyệt số lời gọi) nên cổng ≥14/16 chưa đo; client chưa gửi `hoi` (mention vẫn là `plan`); ADR-0039 chưa ký. |
| 10 | Hàng đợi: outbox, RabbitMQ, poller dự phòng, tác vụ định kỳ, limiter theo lời gọi | **phần máy chủ đã nối, chưa xong lát**: gói `jobs` `d76a0a4`, tầng broker `7246744`, nối vào `chatassist` ở `849664a`. Review phản biện của `849664a` ra `REQUEST_CHANGES` (2 blocker: trần 15 phút chữ rõ khi worker co về 0 không có test đỏ được; `core work` tự bỏ đói ở cỡ pool cấu hình cho phép) và 10 phát hiện nhỏ; vòng sửa 2 sửa cả 12 ở `3b1e819`, review lại ở vòng 3 (dưới). Có: `chatassist` phiên bản 5 (`available_at`, `enqueue_seq`, `first_token_at`, `model_calls`, giữ mọi DEFAULT; mọi DEFAULT ổn định — `now()`, không ghi lại bảng — và `lock_timeout` 5 s; trigger BEFORE đánh số khi vào `queued`, trigger AFTER gọi `jobs_them` khi số đổi — quyết định nằm một chỗ, lệch chữ «cùng điều kiện» của thiết kế 02 §3.2 nhưng cùng tập sự kiện); `migrate-chat` cài `jobs` trước `chatassist`; `serve`/`work` đòi đủ phiên bản của cả hai bảng; claim theo (id, `enqueue_seq`); consumer `ai.group`/`ai.nep` trong `core work` (`MOBILE_WORKER_QUEUES`), một relay mỗi process (LISTEN trên kết nối riêng, ngoài pool), Ack sau commit; lỗi DB → consumer tạm dừng và **giữ** tin (không trả về hàng, nên tạm dừng không tính vào `x-delivery-limit`), chạy lại khi DB trả lời; tin vào DLQ → một dòng cảnh báo chỉ id; poller 2 s (trễ 5 s) luôn chạy, 250 ms khi không có broker; lease sau nội dung đầu; `retryLater`/`release`; ghi cuối của job hỏng vì DB → job về hàng ngay (lượt thử đã tiêu) thay vì chờ hết lease; trần `model_calls` trong hàng qua `Turn.GiuLuot`, trần từ chối một lần thử lại sau 429/5xx thì mã là `provider_unavailable` với lớp lỗi của nhà cung cấp, không phải `ai_het_ngan_sach`; registry `jobs.DinhKy` (dọn outbox, sweep, xoá 30 ngày `ai_turn_metrics` và `rag_query_log`), mỗi lượt chạy trong transaction giữ `pg_try_advisory_xact_lock` của nó — một kết nối, không hai; `serve` luôn chạy sweep và dọn outbox kể cả `MOBILE_INPROC_WORKER=0` (test đơn vị và test `serve` thật trên Postgres), và chờ worker trong process nhả job xong mới đóng pool; `MOBILE_WORKER_DB_CONNS` có sàn = worker + 4 tác vụ + 1 relay, thấp hơn thì từ chối khởi động; limiter GCRA Redis `MOBILE_MODEL_RPM` fail-open; cổng `aigate` đọc mọi SQL có trigger mà binary Go nhúng, khai `job_outbox` cho gốc Nếp và gốc nhóm, `chat_legacy_changes`/`chat_legacy_change_outbox` cho gốc nhóm; compose `rabbitmq`/`redis`/`worker` ghim digest sau profile `hang-doi`. Sửa kèm: relay đọc hết `basic.return` (bản cũ để kênh return đầy làm treo cả kết nối). **Còn mở:** review lại vòng sửa 3; compose chưa dựng thử trên máy có Docker, job CI broker chưa thấy chạy trên Actions; tầng broker ở đây chạy RabbitMQ 3.12 — «tạm dừng không tính vào giới hạn» và ngưỡng DLQ chưa đo trên 4.x; `retryLater` chỉ áp cho engine Go, đường brain giữ hành vi cũ; ân hạn 60 s khi SIGTERM cho job đã có nội dung chờ writer của lát 11 (hôm nay chưa gì đặt `first_token_at`); nối lại broker chờ job dài nhất (≤70 s, poller gánh trong lúc đó); replica cũ trong lúc rollout bỏ qua backoff và không đặt lại `model_calls` khi `/retry` (ghi chú rollout ở thiết kế 02 §8); giữ trước số lời gọi dự kiến lúc claim chưa làm; `MOBILE_MODEL_RPM` chờ hạn mức thật của khoá; lease 30 s chưa bật; Nếp vẫn không biết có worker nào đang sống (`nepSanSang`) **Review vòng 3 (sau `3b1e819`)**: hai blocker vòng 2 đã sửa (sweep khi worker = 0 có test đỏ được; pool không tự bỏ đói), 6/6 đột biến cũ đỏ; REQUEST_CHANGES với blocker mới: relay không bao giờ trả lỗi khi kết nối LISTEN chết (nhánh chết vì `cancel()` trước `wait.Err()`), sau khi Postgres khởi động lại mỗi `core work` chạy ~3 900 giao dịch/giây — có từ trước commit này; và consumer tạm dừng quay ~1 000 lần/giây khi DB trả ping nhưng từ chối claim. **Vòng sửa 3 (commit ngay sau `5a97262`, chưa có review lại)**: relay đọc `wait.Err()` trước `cancel()`, kết nối LISTEN chết thì `Run` trả lỗi (57P01 ngay lập tức; test broker giết backend bằng `pg_terminate_backend`, đòi ≤1 s) và Ket nghe lại trên kết nối mới **mà không quay số lại broker** (LISTEN mới sau lần chờ đầu 250 ms, consumer không tách lần nào, job ghi sau đó tới consumer qua NOTIFY với nhịp 1 phút) — lệch chữ «Ket nối lại» của vòng 2 có chủ ý: quay số lại trả mọi tin consumer đang giữ về hàng, mỗi tin tính thêm một lần giao; consumer tạm dừng chờ 250 ms nhân đôi tới 30 s trước mỗi lần chạy lại tin đang giữ, một dòng log mỗi đợt tạm dừng, channel đóng giữa lúc tạm dừng thì consumer trả lỗi để Ket nối lại (điều thiết kế 02 §8.1 đã hứa mà code chưa làm); đo PB1 (CHECK NOT VALID chặn `status='running'`) và PB2 (pool chỉ đọc): 5 claim trong 5 s và 1 dòng «paused» mỗi ca (bỏ lần chờ: hàng nghìn claim trong 5 s); heartbeat dừng trước khi trả job (`sync.OnceFunc`) và không lần gia hạn nào bắt đầu sau khi dừng (trước đó `select` chọn ngẫu nhiên giữa «dừng» và nhịp đã tới, nên việc dừng có thể chờ một chuỗi lần gia hạn 2 s sau DB đang kẹt — tìm ra khi chạy lại probe PP1 của review: 1/6 lần không trả job trong 15 s), hai nhánh lần thử cuối/đã có nội dung có test Postgres; tin tới sau khi tạm dừng bắt đầu được giữ (test đơn vị với kênh giao do test bơm); canary `TestHangDoiRelayGiuHangKhiBiTraVe` chập chờn vì relay khai báo topology (gắn lại binding) sau khi test đã gỡ — test nay chờ relay LISTEN rồi mới gỡ, 20/20 lần xanh dưới `-race`. Đột biến N1, N2, N3, N5, N6 của review đều đỏ. **Vẫn mở thêm**: Ket quay số lại ngay sau một phiên ngắn (NewRelay hay consumer hỏng ngay khi nối thì quay vòng nhanh — có từ trước, chưa có test); poller không lùi dần khi claim hỏng (nhịp 250 ms, 4 lần/giây); một lần gia hạn đã hết giờ phía client vẫn có thể nằm chờ khoá phía server rồi chạy khi khoá nhả (xếp trước câu trả job nên thực tế chạy trước; chưa có test). **Vòng 4 (`12ccd0f`)**: relay trả lỗi khi kết nối LISTEN chết và Ket nghe lại (0 s, nghe lại sau 257 ms), consumer tạm dừng lùi dần 250 ms→30 s, heartbeat dừng trước khi trả job, canary chập chờn tìm ra nguyên nhân (20/20). Review lại: **APPROVE**, hai phát hiện thấp (`Song()` vẫn báo sống khi relay không nghe được → poller chậm 5–6 s; ba hành vi chưa có test đỏ được). Phía máy chủ lát 10 xong; còn mở: compose `worker`/`rabbitmq`/`redis` chưa chạy lần nào (không có Docker), job CI broker chưa thấy chạy trên Actions, RabbitMQ 4.x chưa đo. |
| 11 | Stream SSE; bảng Nếp mới và animation (`/impeccable`, sửa `DESIGN.md`) | **chưa xong lát**: phía máy chủ ở commit `feat(aistream)` con trực tiếp của `8dee673` (review vòng 1 của `22664ca` trả REQUEST_CHANGES: 1 blocker, 4 major, 7 minor/nit — đã sửa hết ở commit này, chưa có review vòng 2); phía mobile ở nhánh `mobile/ai-stream` (con của `713f960`): `ai/tra-loi-song.ts` + `useAiStream` nối `sse.ts` vào bảng Nếp (`NepPhien`) và hàng trả lời chờ của người hỏi trong luồng nhóm (`TraLoiAiDangViet`), lùi về polling cũ khi stream hỏng 2 lần/429/503/nền tảng không stream được; chưa animation. Trước đó: gói `aistream` `9bb26b0`, client `ai/sse.ts` `aab71de`. **Thiết kế trên lõi agent: nháp → verifier → stream.** Engine Nếp không stream model nữa: câu trả lời (đường thẳng, công cụ hay truy hồi, kể cả câu hỏi lại) được viết trọn, qua kiểm cấu trúc (`kiemDauRa`, nay dùng chính phép quét của cửa sổ) và verifier (`kiemchung`), ghi trí nhớ đã xếp hàng, **rồi mới** được nhả vào `Sink.Delta` qua cửa sổ (`aiharness.phatRa` → `guard.PhatTheoNhip`): không một byte nào verifier chưa duyệt rời engine (test `TestNepKhongNhaTruocKiemChung`, bất biến 8 của T1 đỏ khi có delta trong lượt verifier không duyệt, ca T1 55–57). **Giá về latency**: delta đầu tới sau router + câu trả lời + verifier, tức gần trọn lượt — hàng Latency ở §3 («token chữ đầu p50 ≤2,5 s, p95 ≤5 s») nay đo ở delta đã duyệt, chưa có số T3 thật, cần Lead chấp nhận (§7); `trang_thai` đầu vẫn ra trước mọi I/O. Nhả theo nhịp 16 rune mỗi 25 ms (`aiharness.NhipPhat`), cả câu trong tối đa 800 ms (`guard.TranNhip`), để màn hình hiện dần; Sink bỏ đi (`BoQua`, không có stream) thì không chờ. Câu brain (nhóm và Nếp) chỉ qua kiểm của cửa sổ khi có stream, và chữ của nó vào stream **sau** commit, theo cùng nhịp, ngay trước `xong`. **Bất biến cửa sổ (chính xác)**: một rune rời cửa sổ chỉ khi (1) phép quét của guard trên mọi thứ đã thấy sạch, (2) đã thấy ít nhất 48 rune sau nó, và (3) điểm nhả là một khoảng trắng cách cuối phần đã thấy ≥48 rune — hoặc câu đã hết và quét cả câu sạch. Guard nay chỉ còn kiểm định dạng (mã kiểm, email, số điện thoại, số tài khoản/thẻ, trích lời nhắc); mẫu không có khoảng trắng (email, mã kiểm, chuỗi số liền) không bao giờ rời một phần vì (3), mẫu có khoảng trắng bị cờ trong 48 rune từ rune đầu (chuỗi số bị cờ khi đủ 9 chữ số, tối đa 33 rune với dấu nối « - »; trích lời nhắc đọc 40 rune đầu). Luật cụm từ về tự nhận hành động/tiền đã chuyển sang verifier. **Worker**: `khoa(job)` một chỗ (Nếp → khoá lời gọi; nhóm lane `legacy` → khoá phòng, entry mang `inv`; lane khác kể cả `v2` → khoá lời gọi); `first_token_at` đặt dưới lease trước nội dung đầu; SIGTERM: job chưa có nội dung được chốt không nhận nội dung nữa (`Writer.ChanNoiDung`, nguyên tử với nội dung đầu) và trả về hàng, job đã có nội dung chạy tiếp tối đa 60 s (`WorkerConfig.AnHanDung`), quá hạn thì `failed/worker_interrupted` + `that_bai`; sweep fail job có nội dung thì ghi `that_bai{worker_interrupted}` vào đúng khoá; `trang_thai` lúc claim, `xong` chỉ sau commit, `that_bai{code}` sau thất bại, `huy` khi huỷ/rút, `lam_lai` khi về hàng trước nội dung; nhóm bị guard chặn → `ai_tu_choi` (câu ở `aiharness/cau` bảng `bangNhom`, app giữ đúng từng chữ ở `LOI_KET_QUA_AI`). **Route SSE** `GET …/ai-invocations/{id}/events` và `GET /me/nep/ai-invocations/{id}/events` (hai hàng `GO-ONLY`): như §4.1; hàng Postgres là sự thật — kết thúc của hàng (đọc lúc mở và mỗi lần kiểm quyền lại 10 s) đóng luồng sau một nhịp đối soát nếu Redis không có sự kiện kết thúc; quá 64 entry tồn thì gộp delta; hạn 8 s bỏ theo pattern route của mux, không theo đuôi đường dẫn; đếm lần mở INCR+EXPIRE trong một script. Cổng AST: chỉ cửa sổ gọi `Delta`; chỉ `aistream` XADD; hằng `aistream.Delta/Phan/Xong`, literal `DeltaData`, `Writer.Ghi`, `Stream.Append*` ngoài `aistream` chỉ trong đúng phương thức của `luongViec`; không lời gọi log nào trong `aistream`/`jobs` nhận text/delta/chunk/payload. **Còn mở**: animation «đang nghĩ» của Nếp (cần `/impeccable`, sửa `DESIGN.md`, Lead ký); chạy trên thiết bị thật (expo/fetch đưa body từng mẩu, Reduce Motion hệ thống, VoiceOver/TalkBack với chữ lớn dần); ảnh chụp mới có trên web export (bảng lab `app/dev/tra-loi-song.tsx`); người xem khác trong phòng nhận delta qua frame `ai` WS và `ai.stream=phong` (lát 12; hôm nay chỉ người gọi thấy chữ chạy); số latency T3/T4 thật và `cmd/chat-load -mode ai-stream`; chữ trong thẻ places/itinerary của brain chưa qua cửa sổ; compose chưa đặt `MOBILE_REDIS_URL` cho `core`; rollout không bật khi còn replica trước lát 10 (thiết kế 02 §8.1) |
| 12 | Stream cả phòng qua frame WS; UI nhóm hoàn thiện | **phần frame phòng xong, chưa xong lát** (nhánh `s12/room-stream`, con của `a6d631d`): thành viên khác trong phòng lane cũ thấy chữ hiện dần qua frame `ai` trên WS `chatlegacychange` (§4.2). Máy chủ: `aistream.TheoPhong` (enum đóng của phòng, dựng lại dữ liệu từng trường, phát lại khi vào/nối lại, bơm theo hub), `chatlegacychange` opt-in trong frame authenticate, bơm chỉ sau trang đầu đã ack và chỉ khi `Page.Lane` = `legacy` (đọc trong cùng snapshot với `authorize`), tối đa 256 bơm mỗi phòng mỗi process, ghi ≤10 s; entry khoá phòng mang `tin`/`so`; writer khoá phòng giữ mọi chữ tới sau commit (`SauChotThoi`) và engine Go của nhóm chạy với `aiharness.ChiTrangThai` (trạng thái đi ngay, chữ không đi, không chờ nhịp) — **đổi so với lát 9**: chữ nhóm trên engine Go không còn chạy trước khi thẻ commit, mà nhả từ chính thẻ sau commit, như §4.1 vốn ghi; `ai.stream` = `phong` khi WS có frame. Mobile: `ai/phong-ai.ts` (đọc frame, gộp qua đúng `buocTraLoi`, dọn), `useRoomAi`, `useChatChanges` rẽ frame `ai` trước làn trang, không ack, làm lại kho ở mỗi socket; hàng `HangTraLoiAiDangViet` với `nguoiXem="thanh_vien"` (câu mới ở `CAU_CHO_TRA_LOI`, cổng `cau-chu-goi-ai`). Số đo fan-out ở commit. **Còn mở**: phòng v2 chỉ thấy «Rủ Đi AI đang trả lời…» cần kênh sự kiện tạm của chat v2 (lát 20; hôm nay WS lane cũ không gửi gì cho phòng v2 và không job nào chạy ở phòng v2); `/impeccable` cho UI nhóm; Maestro trên máy thật; `cmd/chat-load -mode ai-stream` |
| 13 | Nếp tại chỗ: phiếu v2, sổ tay app, chip, tool phía máy chủ | một phần: dữ liệu sổ tay 13 màn + cổng lệch `e69012b`, sửa theo lát 7 `603515f`; gói thuần `internal/huongdan` (`TheoMan`, `Tim` qua `rag/xephang`, `DuongToi` BFS ≤5 bước, `BanDung` + hằng `nep/huong-dan-ban.ts`) `5c3a3c1`; review phản biện REQUEST_CHANGES (10 phát hiện) đã sửa ở `c7a3cde`: luật màn tiền không còn lách bằng `di_toi` về chính màn hay khai nút trả tiền làm lối ra (cửa phải là cạnh có nhãn của mã, `_rut.json` `canh`), tiêu đề mục màn tiền cố định; bộ vàng thứ hai 46 câu hỏi từ màn khác (viết và băm trước khi đổi xếp hạng); luật ghim theo tỷ lệ điểm 1/2, bảng teencode, cắt 2000 rune. Số đo: bộ 91 câu recall@5 0.9505 MRR 0.9211, bộ màn khác 0.9130 / 0.7880; **nhóm teencode của bộ 91 câu recall@5 0.8333, dưới ngưỡng 0.90** (hai câu tiếng Anh «checkin», «log out», không dịch là chủ ý). Review lại bản sửa đó APPROVE với 8 phát hiện nhỏ, sửa ở `79c5ed8` (review lại vòng 2: APPROVE, 3 nit, sửa ở `658c909` (cherry-pick của `1dff602`, con trực tiếp của `be0f7a0`): quy tắc «ít màn tiền nhất» có test đếm trên cả đường — đồ thị màn tiền cách hai bước của review, và đường qua một màn tiền ngay bước kế thắng đường qua hai màn tiền phía sau; tiêu đề của chính màn tiền không phải cửa, có ca fixture ở Go và mobile, in trên màn hay không; « và » phải thành cặp trên từng dòng theo thứ tự, dấu đảo ngược, lẻ, lồng hay vắt dòng bị từ chối ở Go và mobile, probe Q1 của review thành canary trên dữ liệu thật; review lại bản sửa vòng 3 (review lát 13 vòng 4): APPROVE, 1 nit NF4 — luật « » theo dòng chưa có ca ở dòng tiêu đề mục ở cả hai phía, phía Go chưa có ca ở dòng văn trong mục — sửa ở commit `fix(huongdan)` vòng 4 là con trực tiếp của `591cce5`: ca «## Đi sang »Nút B«» và «Xem »Nút Z«.» trên fixture Go, ở Go và cổng mobile): bộ rút chỉ ghép mỗi cú bấm với nhãn gọi tên nó (`onAction` ↔ `action`, `onPress`/`href` ↔ `label`/`accessibilityLabel`/`title`), nên tiêu đề mục «Chi theo nhóm» không còn là cạnh có nhãn (`_rut.json` 129 cạnh có nhãn, băm `27a579cd74f3`); bước màn tiền chỉ được trích cửa; khoá front matter trùng hay sai hoa thường bị từ chối (Go và mobile); cổng mobile soi lại fixture Go của luật màn tiền; số đo bộ vàng không đổi. Chưa: phiếu v2 (`buoc`, `hanhDong`, `banBuild`; cách `buoc` đi qua `buocMuc` ở thiết kế 05 §3), tool `search_app_manual`/`explain_screen` (lát 9), chip, `useNepMoc`, sửa câu gợi ý; tổng quan của màn tiền chưa theo luật chỉ trích cửa (probe Q3; hiện không vào `Doan` và không ra API nào — phải có luật cửa trước khi API nào đưa nó ra); văn xuôi dạy trả tiền không «…» trên dòng có cửa (P8) luật không thấy, kể cả tên nút đặt trong dấu na ná « » như ‹…›, "…" (hay “…”) và 《…》 — luật nhãn và luật cặp chỉ đọc « và », nên ‹Đánh dấu đã trả› cạnh một cửa trên `tai-chinh.md` nạp được ở cả Go và mobile (probe Y2 của review vòng 4); người review văn giữ **Tích hợp lõi agent**: `search_app_manual`/`explain_screen` chạy phía máy chủ trong engine (đường truy hồi đọc sổ tay qua `tools.SoTay`, đường nhanh `explain_screen`); nút «…» trong câu trả lời phải trùng nhãn nút sổ tay trả về trong lượt. Chip chưa. |
| 14 | Chia bill từ thẻ, dấu «Đã ghi vào sổ» suy từ dòng chi tiêu thật | chưa |
| 15 | Trí nhớ Nếp | **một phần — lát `infra/memory-policy`, chờ Lead ký ADR-0043 (đề xuất)**: `internal/nepnho` là writer duy nhất của `nep_*` (bảng version riêng; biên nhận không chữ; tombstone HMAC; sự kiện kiểu đóng 30 ngày); client HTTP của sidecar `services/ai-infer` (sidecar là writer duy nhất của Milvus trí nhớ); nhớ lại chỉ khi cờ bật; «quên» là saga sidecar xoá → `remaining` 0 → Go liệt kê lại → biên nhận, thử lại trên làn `memory` của outbox; tắt cờ xoá hết; xoá tài khoản qua trigger `people.deleted_at` + `purge_user` tới 0, biên nhận chỉ giữ HMAC; 4 route GO-ONLY `/me/nep/tri-nho*`, `/me/nep/su-kien`; `internal/aictx` bộ đệm theo lượt trên `redis-ai` (TTL nguyên tử, AES-GCM, UNLINK, nhóm legacy EX 900, không gì cho v2, kiểm `save`/`appendonly` lúc khởi động); cá nhân hoá ≤5 ký ức vào khối dữ liệu của Nếp khi cờ bật, bot nhóm không có (canary `aigate`). Chưa: bộ eval trí nhớ tiếng Việt, model trích thật, reranker, UI công bố/cài đặt, các quyết định mở của ADR-0043 §5 |
| 16 | RAG vector, làm giàu, độ tươi | **một phần (nhánh `infra/rag-unified`, gộp `infra/retrieval-core` và `infra/ingest-sdlc`, chưa vào main)**: một hệ truy hồi + nạp trên **Milvus v3.0.2** — **một schema, một writer** (§8.4): `internal/vectordb` khai schema duy nhất (`rd.v2`: chunk id, `doc_id`, dense 1536, MILCO, `text` giữ dấu + `text_khong_dau` gấp dấu với hai hàm BM25, thuộc tính lọc cứng), tên `rd_places__vN`/`rd_manual__vN` sau alias `rd_places`/`rd_manual`, một hàm dựng client (telemetry tắt); `rag/nap` (pipeline nạp: làm giàu LLM enum đóng + dòng ngữ cảnh mỗi chunk, hàng duyệt, phiên bản collection, cổng eval, promote/rollback bằng alias) ghi qua `vectordb/napkho`, adapter duy nhất; `internal/hybrid` (dense `gemini-embedding-2` + BM25 có dấu + BM25 không dấu — nhánh thưa **chỉ là hàm BM25 của Milvus**, MILCO gác lại chờ xác nhận license (chủ sản phẩm 2026-09-27), mã adapter MILCO tắt mặc định và không nối vào đường truy hồi; RRF có trọng số cấu hình ở `rag/nap/cauhinh.json` «hop», gấp chunk về quán, kiểm lại mọi hit) đọc đúng bảng nạp ghi (`thuoctinh` đọc `places` + `place_enrichments` qua `nap.ApDung` + `rag_tombstones`), nối vào `truyhoi.Retriever` của tool khi có `MOBILE_MILVUS_ADDR`, không có thì `aidoc.Lexical`; `internal/rerank` (Qwen3 qua `/rerank`, lọc token cấu trúc NFKC, điểm riêng `DiemXepLai`, timeout `MOBILE_RERANK_TIMEOUT`); embedding **chỉ** `gemini-embedding-2` 1536, tiền tố thêm một lần trong `aiharness/nhung`; `scripts/go_milvus_tier.sh` (skip là đỏ, 17 sentinel) chạy ở job CI `milvus` riêng (runner tự host có Milvus + reranker). **Chưa**: gọi Gemini thật (không khoá), đo bằng encoder thật (trọng số hợp nhất và số vàng hiện đo trên stub), bộ eval tiếng Việt §C4, hai bảng cache nhúng (`rag_embedding_cache` của nạp, `nhung_cache` của truy hồi) chưa gộp. |
| 17 | Nhắc chủ động: trong app, rồi push | chưa |
| 18 | Eval M3 và tín hiệu phản hồi | **một phần (nhánh `track-e-eval-that`, chưa vào main, chưa review)**: `aieval` có cassette (`CassetteLLM` bọc `model.LLM` của ADK v2: ghi yêu cầu/đáp gồm chunk stream và `UsageMetadata`, phát lại không dựng client; khoá = phạm vi `<ca>@<lap>` + sha256 JSON chuẩn trừ `httpOptions` và id `functionCall`/`functionResponse` + thứ tự lần gặp; trượt khoá là `bang_lech`, không bao giờ ra mạng), cùng cơ chế cho nhúng `gemini-embedding-2` và reranker; `rudi-eval --mo-hinh kich-ban\|ghi\|phat-lai\|that`, `--chi-buoc hieu` trên bộ router (recall tiền, từ chối nhầm, KTC Wilson), `--du-toan` = trần trên chính xác (ca × lap × (8 model + 2 nhúng + 2 xếp lại) + 1), `--tran-goi N` cứng có watchdog; kho bằng chứng `~/.cache/rudi-bang-chung/eval/<run_id>/` (manifest không nội dung, vết, cassette, `bang-diem.md`); chi phí `math/big.Rat`, giá `gia-model.json` để null «cần người xác nhận»; T2 offline (ghi từ bản giả loopback → phát lại 0 lời gọi, điểm trùng) nằm trong chặng `eval-kich-ban`. Chưa: lượt thật, vân tay + `check_eval_trailer.py`/`check_eval_release.py`, judge + κ, bộ M3 (red team, định tuyến 150, trí nhớ, hướng dẫn), T4 `geministub`, tín hiệu phản hồi (migration, route, UI) |
| 19 | Gỡ action Python `companion-reply`/`nep-reply` (một commit cùng manifest) | chưa |
| 20 | E2EE v2 (chat-lab) | chưa |

### 4.1 Hợp đồng client của stream (lát 11)

Cho `apps/mobile/src/rudi/ai/sse.ts` và màn hình dùng nó (bảng Nếp, câu trả lời của Rủ Đi AI trong luồng nhóm).

- **Mở**: sau `202` của `POST`, mở `GET /me/nep/ai-invocations/{id}/events` (Nếp) hoặc `GET /contexts/{c}/ai-invocations/{id}/events` (người gọi trong nhóm) với `Authorization: Bearer`. `503 stream_unavailable|stream_capacity` và `429 stream_rate_limited` (có `Retry-After`) nghĩa là quay về hỏi `GET …/{id}` như trước; `404` là không phải lời gọi của mình. `chat-capabilities.ai.stream` = `nguoi_goi` khi người gọi xem được, `khong` khi máy chủ không có stream.
- **Thân**: dòng `retry: 2000`, rồi `hello{nhip_ms}` (chu kỳ ping), rồi các sự kiện; dòng `: ping` mỗi 15 s. Mọi sự kiện đọc từ Redis có `id:`; sự kiện dựng từ hàng Postgres **không có** `id:` và không được dùng làm vị trí nối lại.
- **Thứ tự**: `trang_thai{cau}` (`dang_xep_hang` khi job còn chờ, `dang_doc` lúc worker nhận, `dang_nghi` khi model chạy) → không hoặc nhiều `delta{p,text}` → đúng một sự kiện kết thúc: `xong`, `that_bai{code}`, `huy` hoặc `thu_hoi`. `lam_lai` chỉ có thể tới **trước** delta đầu. `ket_noi_lai{sau_ms}` đóng kết nối (180 s, hoặc process dừng): mở lại ngay với `Last-Event-ID` (native) hoặc `?after=` (web), nhận đúng phần sau vị trí đó.
- **Thời điểm**: `trang_thai` đầu tới ngay khi mở (không chờ hàng đợi); delta đầu tới **sau khi verifier đã duyệt cả câu** (nháp → verifier → stream), nên giữa `dang_nghi` và delta đầu là cả lượt suy nghĩ; sau đó các delta tới dồn trong tối đa 800 ms (16 rune mỗi 25 ms, gom ≤60 ms mỗi lần ghi). Với nhóm, delta tới sau khi thẻ đã đăng vào phòng, ngay trước `xong{message_id}` — cả engine Go từ lát 12 (lát 9 từng nhả trước commit).
- **Hiển thị**: `trang_thai` → trạng thái «đang nghĩ» với câu cố định của mã; `delta` → nối `text` vào phần `p` (Nếp chỉ có `p=0`; một delta không bao giờ kết thúc giữa một từ); `lam_lai` → bỏ phần đã hiện (thực tế chưa có gì) và về «đang nghĩ»; `xong` → Nếp: `text` là câu đã niêm phong, thay phần đang hiện bằng nó (bằng đúng các delta nối lại); nhóm: `message_id` là thẻ đã đăng, lấy từ luồng tin như thường; `that_bai{code}` → giữ phần đã hiện (nếu có — chỉ xảy ra khi worker bị ngắt sau khi đã nhả, `worker_interrupted`) và nối câu của mã (`LOI_KET_QUA_NEP`, `LOI_KET_QUA_AI`); `huy` → lời gọi đã huỷ; `thu_hoi` → mất quyền đọc, ẩn phần đang hiện; `ket_noi_lai` → nối lại, không coi là lỗi.
- **Không có**: sự kiện «rút lại» (guard và verifier chặn trước khi nhả), `phan` (chưa phát ở lát này). Người xem khác trong phòng: §4.2.

### 4.2 Frame `ai` cho người xem khác trong phòng (lát 12)

Cho `apps/mobile/src/rudi/chat/useChatChanges.ts` và `ai/phong-ai.ts`.

- **Kênh**: WS `GET /contexts/{c}/changes/stream` sẵn có của luồng tin, không SSE thứ hai. Opt-in: frame
  authenticate `{"type":"authenticate","token":"…","ai":true}`. Không opt-in, hay xác thực bằng header, thì không
  có frame nào; máy chủ cũ bỏ qua trường lạ.
- **Quyền**: đúng `authorize` của luồng tin (phiên, người, thành viên `active`). Bơm frame bắt đầu **sau** trang đầu
  đã ack; mỗi nhịp đối soát (1 s) trang chạy lại `authorize`, thành viên bị rút mất kết nối và bơm cùng lúc.
- **Chỉ lane cũ**: `lane` đọc trong cùng snapshot với trang; phòng ở v2 không bao giờ khởi động bơm, phòng vừa
  chuyển v2 dừng bơm ở trang kế tiếp. Job v2 không bao giờ ghi khoá phòng (`khoa(job)`), nên đây là bức tường thứ hai.
- **Frame**: `{"type":"ai","inv":"…","tin":"…","so_tin":n,"id":"<redis id>","e":"…","d":{…}}`; trang thay đổi không có
  `type`. `e` ∈ {`trang_thai`, `phan`, `delta`, `lam_lai`, `xong`, `that_bai`, `huy`}; `d` dựng lại từng trường:
  `trang_thai{cau}`, `delta{p,text}`, `phan{kind,json}`, `xong{message_id}` (không bao giờ chữ, chips, nguồn),
  `that_bai{code}` (mã guard đã gộp về `ai_tu_choi` ở writer), `lam_lai{}`, `huy{}`. Client **không** ack frame
  `ai`, frame không đổi con trỏ `after`.
- **Thời điểm**: `trang_thai` khi worker nhận và khi engine đổi trạng thái; chữ **chỉ sau khi thẻ đã commit**, lấy
  từ chính thẻ qua cửa sổ 48 rune có nhịp (16 rune mỗi 25 ms, ≤800 ms), rồi `xong{message_id}`; thẻ cũng tới như
  một trang của luồng tin trên cùng socket.
- **Vào muộn / nối lại**: mỗi kết nối mới là một lần vào: máy chủ phát lại khoá phòng (≤15 phút, ~2048 entry), bỏ
  hẳn lời gọi đã kết thúc, mỗi lời gọi đang chạy tới như các sự kiện đến giờ với các delta liền nhau gộp một mỗi
  phần (id của entry cuối), rồi theo trực tiếp. Client xoá kho phòng ở mỗi socket mới (`moi`) rồi dựng lại từ phát
  lại; trong một kết nối id tăng dần và `buocTraLoi` bỏ frame trùng.
- **Hiển thị**: cùng máy trạng thái với người hỏi (`buocTraLoi`), cùng Reduce Motion (`chuHienThi`); hàng
  «Rủ Đi AI đang đọc {n} tin…» + `CAU_CHO_TRA_LOI.thanh_vien`, rồi chữ, rồi nhường chỗ khi thẻ `message_id` có trong
  luồng; `that_bai`/`huy` ẩn hàng (mã là của người hỏi); không frame nào 30 s thì bỏ; câu trả lời của chính mình để
  hàng người hỏi vẽ.
- **Giới hạn**: slot của luồng tin (1000 mỗi process, 5 mỗi người); ≤256 bơm mỗi phòng mỗi process (quá thì thành
  viên vẫn có luồng tin và thẻ, chỉ không có chữ chạy); ghi mỗi frame ≤10 s không thì đóng kết nối; không log nội
  dung frame (cổng `aigate` quét log của `chatlegacychange`).

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
- Lát 11 trên lõi agent: stream chỉ nhả câu đã qua verifier, nên «token chữ đầu» ở hàng Latency §3 thành «delta đầu đã duyệt», gần bằng trọn lượt. Chấp nhận mốc này, hoặc đặt lại mốc cho delta đầu.

## 8. Luật không heuristic (chủ sản phẩm, 2026-09-25)

Luật này **thắng** đề xuất ADR-0037 §4 và thiết kế 01, 04 ở mọi chỗ chúng lệch nhau. Chỗ lệch trong
mục 2 ở trên (bước 1 «giải ngày tương đối», «teencode chỉ cho bản định tuyến»; bước 2 «luật tiền»;
bước 3 «qua guard lần nữa») đọc theo mục này.

**Luật.** Tuyệt đối không heuristic, không lọc theo từ khoá ở bước hiểu câu, định tuyến, chọn tool,
quyết định truy hồi hay trích ràng buộc. **Model quyết** (structured output + function calling).
Mã Go tất định chỉ được làm những việc sau:

- kiểm **cấu trúc**: JSON schema, enum đóng (`Parse` từ chối giá trị lạ), id thuộc danh sách đóng của
  lượt, kiểu tham số;
- cưỡng chế **ngân sách, quyền, hạn giờ**;
- áp **ràng buộc cứng** model đã trích làm bộ lọc không nới được;
- **số học chính xác**: ngày từ giá trị ISO do model viết cộng «bây giờ» của lượt; tiền là số nguyên đồng;
- kiểm **grounding bằng thuộc tập** trên bằng chứng của lượt;
- kiểm **định dạng dữ liệu** ở đầu ra vì quyền riêng tư: số điện thoại, số tài khoản ngân hàng, email,
  số thẻ thanh toán. Đây là kiểm định dạng dữ liệu, không phải hiểu ngôn ngữ, nên được giữ.

Mọi thứ đọc **nghĩa** của chữ bằng danh sách từ hay regex phải rời đường quyết định. Chống prompt
injection thành **cấu trúc**: mọi chữ không tin cậy chỉ nằm trong khối `<du_lieu>` có datamarking;
system instruction nói dữ liệu không bao giờ là lệnh; tool chỉ có tác dụng phụ trong danh sách cho
phép; nháp cần người bấm. Thêm vào đó là nhãn guard do router (LLM) gán.

**Không HyDE** (chủ sản phẩm, 2026-09-27): không có bước sinh «tài liệu giả định» nào để nhúng thay câu hỏi,
ở bất kỳ đường nào. Viết lại truy vấn chỉ là các trường structured output của chính router (câu hỏi tự đủ
nghĩa, câu đã khôi phục dấu) trong cùng một lời gọi `hieu`; truy hồi nhúng đúng chữ đó.

**Rời đường quyết định** (việc của các builder sau hợp đồng này; commit hợp đồng chưa xoá mã nào):

| Hiện có | Thay bằng |
|---|---|
| `guard.LaTien` (regex tiền, chặn trước model) | trường `tien` của router `hieu` (`none`/`split_draft`/`money_action`) + `kiemchung.PhanTu.Tien` ở đầu ra; không tool nào ghi tiền |
| `guard.Nghi` (mẫu từ khoá injection) | datamarking `<du_lieu>` + instruction + nhãn `chen_lenh` của router |
| `preprocess.DongMayChu` (đọc ngày bằng từ khoá) | router viết `ngay_iso`/`khung_gio`; Go chỉ kiểm dạng và cộng với «bây giờ» |
| luật cụm từ «tôi đã chuyển tiền» trong output guard | `kiemchung.PhanTu.HuaHanhDongKhongCo` / `Tien` (một lời gọi verifier) |
| `tuvung` đọc câu hỏi (dị ứng, ăn kiêng, điểm đến từ chữ) | slot enum `di_ung`/`an_kieng` (id của `tuvung`, chỉ dùng danh sách id) + `di_ung_ngoai_danh_muc` |
| `rag/diemden` phân giải điểm đến từ chữ **phía câu hỏi** | router chọn `diem_den_id` trong danh sách đóng của lượt |
| `KhongDau` dùng để định tuyến | bỏ khỏi định tuyến; chỉ còn là nhãn phân tầng của eval |

**Không đổi:** các route đã có parity (`services/core/internal/routes/*`), `domain/promptsafety` mà các
route port từ Python dùng, cách đặt tên roster của `chatassist` — giữ nguyên từng byte. Phía nạp dữ
liệu (`rag/chunk_place.go` gắn nhãn quán qua `tuvung`) đổi sang làm giàu bằng LLM ở mảng hạ tầng/SDLC
riêng; hợp đồng này không chạm nạp dữ liệu. `chatintent.Parse` (lệnh `/plan`, `@rudi` người dùng gõ)
là cú pháp gọi tường minh, cổng consent, không phải hiểu ý định — giữ, nhưng không được mở rộng thành
đoán ý định.

**Cổng (port) của lõi agent**, tất cả trong `services/core/internal/aiharness`:

| Gói | Hợp đồng | Go làm gì | Model làm gì |
|---|---|---|---|
| `dong` | `Tap[T]`: enum đóng, `Parse`/`ParseAll` từ chối giá trị lạ và trùng | — | — |
| `hieu` | `Vao`, `KetQua`, `Slots`, `LuocDo(Vao) *genai.Schema` (nguồn sự thật duy nhất của schema), `Doc` (đọc chặt), interface `Hieu` | kiểm cấu trúc; id thuộc danh sách; ngày ISO hợp lệ lịch; tiền nguyên ≥0; ý định và nguồn theo bot; các trường nhất quán với nhau (`huong` ↔ `truy_van` ↔ `can_hoi_lai`). Sai một chỗ là từ chối cả kết quả, không lặng lẽ bỏ ràng buộc cứng | nhãn guard, 1–3 ý định, tiền, `huong`, slot, nguồn cần truy hồi, câu truy vấn viết lại, câu hỏi lại, độ tin |
| `truyhoi` | `YeuCau{Nguon, Cau, CauCoDau, Cung, Mem, K}`, ngữ cảnh `VoiXepLai`/`HoanXepLai`, `BangChung`, `KetQuaTruyHoi{BangChung, Degraded, BiLoai}`, `Retriever`, `Reranker`, `Passthrough`, `Cung.HopChat` | lọc cứng không nới; hợp ràng buộc của router vào mọi lời gọi tool, chặt hơn thắng; xếp hạng (BM25, dense, sparse, RRF, reranker) | viết `Cau`; chọn nguồn |
| `trinho` | `TriNho{Nho, Ghi, Quen, LietKe}`, `SuThat`, `NganHan{Doc, Them, Xoa}`, `Luot` | một chủ mỗi fact; quên là xoá cứng; ≤8 lượt ngắn hạn | quyết điều gì đáng nhớ, «quên» chỉ vào đâu, fact nào liên quan |
| `tools` | 18 tên tool (enum), `DangKy` (mô tả một dòng, lớp tác dụng, phạm vi), schema tham số, bảng quyền `testdata/quyen.golden.json`, `SoCai` (sổ cái lượt, bí danh `p1`/`m1`…), mã lỗi tool đóng | lọc toolset theo quyền và chính sách; `proceed_restricted` chỉ còn tool đọc; đếm lời gọi; chống gọi lặp y hệt; ghi bằng chứng; prompt chỉ thấy bí danh, id thật không vào prompt | chọn tool và tham số |
| `crag` | `DanhGia{KetLuan, RangBuocThieu, NoiLong, VietLai}`, `LuocDo`, `Doc`, `SuaYeuCau` | áp đúng một bước sửa model đề xuất; `NoiLong` chỉ nhận ràng buộc mềm; ≤1 vòng | chấm bằng chứng đủ/thiếu/mâu thuẫn; đề xuất nới mềm hoặc viết lại |
| `kiemchung` | `TuyenBo`, `KiemTra` + `Kiem` (tất định), `PhanTu{MenhDe{So, BangChungIDs, Ket}}`, `LuocDo`, `Doc`, interface `Verifier` | id ∈ sổ cái, số/giờ bằng đúng trường bằng chứng, nhãn nút có trong bằng chứng sổ tay; verifier chỉ trả chỉ số câu + enum (`ho_tro`/`khong_ho_tro`/`khong_thong_tin`), không chữ tự do; **mỗi câu 1..n phải được chấm đúng một lần** (schema `minItems = maxItems = n`, `Doc` từ chối thiếu câu); verifier hỏng là không phát | từng mệnh đề có được bằng chứng hỗ trợ không; có hứa hành động không tool nào làm không; có đụng tiền không |
| `testkit` | bản giả trong bộ nhớ của `TriNho`, `NganHan`, `Retriever` | không xếp hạng gì (xếp hạng là của adapter thật) | — |
| `llm` (`ngansach.go`) | `MaxToolCallsPerTurn`=10 (Nếp `MaxToolCallsNep`=6), `MaxStepsNhom`=4, `MaxStepsNep`=3, `MaxCorrectiveRounds`=1, `MaxEmbedCallsPerTurn`=2, `MaxRerankCallsPerTurn`=2, cạnh `MaxModelCallsPerTurn`=8; `buoc.go`: loại lời gọi, mức nghĩ từng bước, kế hoạch xấu nhất `KeHoach` và thứ tự cắt (§8.6) | cưỡng chế | — |

Adapter Milvus (dense + sparse), reranker Qwen qua HTTP, sidecar mem0 và Redis cho ngắn hạn đến từ
mảng hạ tầng và hiện thực đúng các interface trên; engine chỉ biết chúng qua cờ `Degraded`.

**Bảng quyền tool** (`tools/testdata/quyen.golden.json`, bộ nạp từ chối bảng lệch mã):

| Bot | Tool |
|---|---|
| cả hai | `search_places`, `get_place`, `list_destinations`, `nearest_area`, `search_app_manual`, `propose_places`, `propose_itinerary` |
| chỉ Nếp (phạm vi `me`) | `explain_screen`, `suggest_screen`, `my_upcoming_outings`, `recall_memory`, `remember_fact`, `forget_fact`, `what_you_remember` |
| chỉ nhóm (phạm vi `nhom`) | `draft_poll`, `group_snapshot`, `list_group_outings` |
| chưa bật | `set_reminder` (bật cùng lát 17) |

Không có lớp tác dụng nào ghi tiền, nghĩa vụ, kèo, bình chọn hay tin nhắn; tool không khai được nếu
cần lớp đó.

**Nghiên cứu đã nhận vào hợp đồng** (`intent-routing`, `agentic-rag-tools`, `reflection-verification`;
`stm-personalization` chưa có lúc viết): model chọn `huong` (trả lời thẳng / truy hồi một bước / tác tử / hỏi lại), router
viết `truy_van` theo nguồn, cờ dị ứng ngoài danh mục, `mo_ho_voi`, lựa chọn cho câu hỏi lại,
`tra_loi_cau_cho`, số người, người tham gia (enum id thành viên, chỉ nhóm), hợp ràng buộc chặt hơn
thắng, đếm số bị loại theo từng ràng buộc cứng, trần tool của Nếp 6, chống gọi lặp theo tham số chuẩn
hoá, ngắn hạn 4 lượt trao đổi, verifier một lời gọi cho cả câu trả lời trong ngữ cảnh mới, chỉ trả chỉ số
câu và enum, bằng chứng dưới bí danh, trần reranker riêng, giữ một lời gọi dự trữ trước mọi lời gọi tuỳ
chọn, không stream trước rồi kiểm sau. Chế độ gọi
hàm: bước giữa `VALIDATED`, bước cuối `NONE`, không dùng `ANY` trong vòng agent. Không truy hồi suy
đoán trên chỉ mục cá nhân hay nhóm; truy hồi suy đoán trên chỉ mục công khai chỉ được giữ khi ràng buộc
cứng của router khớp đúng. **Không nhận:** hợp luật tiền tất định với router (trái luật này); để
router chép nguyên văn thời gian/ngân sách rồi Go tự giải (trái luật này và trái hợp đồng: model viết
ISO và số nguyên đồng); điểm đến dạng tên tự do (hợp đồng: id trong danh sách đóng); các luật K6/K7/K9
(bắt số, nhãn nút sau «bấm/nhấn», mẫu câu khẳng định an toàn dị ứng) và CRAG tất định theo ngưỡng điểm
reranker (trái luật này: đọc nghĩa bằng từ hoặc ngưỡng tự chế; CRAG là việc của `crag` bằng LLM); coi
verifier hỏng là đạt (verifier giờ mang phán đoán tiền và hứa hành động, nên hỏng là không phát); nhãn
verifier năm bậc (hợp đồng chốt `ho_tro`/`khong_ho_tro`, thêm `khong_thong_tin` cho câu không nêu gì để đối chiếu — §8.3).

**Router `hieu` đã hiện thực** (nhánh `agent-core/router`, chờ tích hợp): `hieu.Router` gọi đúng một
lời flash-lite qua `llm.Dem` với `ResponseSchema = LuocDo(Vao)`, `application/json`,
`ThinkingLevel MINIMAL`, không đặt nhiệt độ (Gemini 3.5 bỏ qua), trần 1024 token ra. System
instruction tĩnh theo bot (`hieu/loi_nhac/{chung,nep,nhom}.txt`, ghép lúc khởi động, có phiên bản
sha256[:12]): định nghĩa từng nhãn guard, tiền (`money_action`/`split_draft` theo bot), ý định kèm ví dụ
có dấu / không dấu / teencode, `huong`, cách giải ngày ra `ngay_iso` từ dòng «Bây giờ» và lịch 14 ngày
máy chủ tính sẵn, điểm đến chỉ là id trong danh sách, khi nào hỏi lại đúng một câu. Lượt người dùng là
các khối `<du_lieu>`; mọi chữ từ ngoài (câu hỏi, ngắn hạn, phiếu màn, tên điểm đến/thành viên) được
**datamark** (`prompts.BocDuLieuDanhDau`, dấu U+02C6 thay khoảng trắng, dấu giả trong dữ liệu bị xoá).
Few-shot động chọn **theo embedding** (`nhung`, `KhoViDu`, 4 ví dụ, ≤2 mỗi ý định, kho tổng hợp
`hieu/vi_du.json` qua `Doc` lúc khởi động); embedder hỏng thì chạy không ví dụ. Đọc chặt bằng `Doc`;
sai cấu trúc thì **sửa một lần** (đầu ra hỏng thành lượt của model, lý do thành khối `loi_cau_truc`)
chỉ khi sau đó còn ≥1 lời gọi cho câu trả lời (`DuTru`), rồi rơi về `ErrKhongHieu` → câu cố định
`invalid_ai_result` («hỏi lại theo cách khác»). Chính sách `QuyetDinhCho` là hàm thuần của nhãn model:
Nếp từ chối mọi `tien ≠ none` (0 lời gọi thêm), nhóm từ chối `money_action` và chỉ cho `split_draft`
thành nháp; `chen_lenh` giữ chữ của người hỏi làm dữ liệu nhưng `HanChe` (chỉ tool đọc); `hoi_lai` kết
thúc lượt bằng câu hỏi của model (qua output guard). Engine có `WithHieu`: đường Nếp qua router không
gọi `guard.LaTien`, `guard.Nghi`, `preprocess.DongMayChu` (test AST giữ điều đó); đường cũ còn nguyên
cho tới khi tích hợp bật router mặc định. Bộ đo: `aieval/testdata/hieu/` — corpus tiền/dị ứng đã lộ
chuyển nguyên vị trí vào `nguon/` và đổi thành bộ T3 (`tien_v2/v3`, `di_ung_v2/v3`, 1 335 ca, đo hồi
quy, **không** phải cổng), bộ T1 `t1-hieu.json` (20 ca, mọi ý định hai bot) chạy bằng
`rudi-eval --mo-hinh kich-ban --chi-buoc hieu --bo …`. Chưa có: chạy T3 thật (cần khoá và Lead duyệt
số lời gọi), câu cố định cho `nhay_cam`/`ngoai_pham_vi` và cho nhóm (chưa có mã trong `cau`), token
của lời gọi router chưa cộng vào hàng metrics.

### 8.1 Hiện thực tool và vòng agent (nhánh `agent-core/tools`)

- `tools`: mỗi tool là một ADK `functiontool` với struct tham số có kiểu; schema JSON chuyển từ đúng
  schema `genai` của hợp đồng (`sangJSON`, từ khoá lạ thì dừng lúc khởi động), đối tượng cấm thuộc
  tính lạ, mảng cấm phần tử lặp. Mô tả tool = mục đích một dòng + «trả gì, khi nào không gọi»
  (`MoTaDay`) để model tự quyết. `BoiCanh` giữ danh tính lấy từ job (người hỏi, nhóm, màn hình),
  ràng buộc của router, danh sách đóng, sổ cái và nháp; không tool nào có tham số danh tính.
- `BeforeTool` (`TruocTool`): tên phải là tool bot được phép trong lượt (bảng quyền, chế độ hạn chế);
  tham số qua schema rồi qua kiểm thuộc tập (bí danh `p1`/`m1`/`f1`/`t1`, `diem_den_id` trong danh
  sách lượt, mã khu vực); lời gọi y hệt trả kết quả cũ kèm `lap_lai`; đếm ngân sách tool. Lời gọi hỏng
  đầu tiên được trả `{"loi","truong","yeu_cau"}` để sửa; lời hỏng thứ hai trả thêm `tra_loi_ngay` và
  bước kế bị ép `NONE`. `AfterTool` (`SauTool`) ghi bằng chứng vào `SoCai` rồi mới dựng kết quả
  cho model: bằng chứng chỉ mang bí danh, nằm trong `<du_lieu nguon="ket_qua_cong_cu">`. `OnToolError`
  (`LoiTool`) đổi tên tool bịa hay lỗi chạy thành mã đóng, không lộ văn bản lỗi hay danh sách tool.
- `search_places`: ràng buộc cứng của router hợp vào mọi lời gọi (`HopRangBuoc`, chặt hơn thắng);
  tham số model chỉ thêm hoặc siết, điểm đến/giờ mở khác router thì bị từ chối. `remember_fact` chỉ
  lưu khi chính model phân loại `ca_nhan` và nội dung qua kiểm định dạng riêng tư
  (`guard.DinhDang`: email, số điện thoại, số tài khoản/thẻ; kiểm định dạng, không đọc nghĩa).
  `nearest_area` trả danh sách khu vực của điểm đến để model chọn; Go không so mô tả với tên.
  `set_reminder` chưa cấp cho bot nào và trả `chua_co`.
- `internal/aidoc` (ngoài `aiharness`, vì engine không giữ mã database — `TestRanhGioiEngine`): mọi đọc chạy trong giao dịch `READ ONLY` dưới semaphore (mặc định 4). Retriever từ vựng
  ánh xạ `truyhoi.YeuCau` sang `rag.YeuCau` từng trường, không `DocCau`, không `tuvung` trên câu hỏi;
  chữ truy vấn chỉ vào xếp hạng BM25/trigram của `rag`. Kết quả luôn gắn `lexical_only`.
- `tactu`: đường nhanh khi router chọn `truy_hoi_mot_buoc` với đúng một ý định trong
  `find_places`/`app_help`/`explain_screen` và đủ slot (điểm đến; truy vấn sổ tay; phiếu màn hình),
  nhãn guard `sach`, tiền `none`: Go gọi thẳng tool qua cùng cổng kiểm, rồi một lời gọi trả lời chế
  độ `NONE`. Còn lại là vòng ADK: Nếp ≤3 bước, nhóm ≤4, bước giữa `VALIDATED` với `AllowedFunctionNames` của bước (§8.6), bước cuối `NONE`. Ngắn
  hạn (`trinho.NganHan`) vào prompt trong một khối `<du_lieu nguon="lich_su">`, bằng chứng cũ là
  `t1`, `t2`…; `GhiLuot` chỉ gọi sau khi câu trả lời đã qua mọi kiểm đầu ra.
- Đã chốt (§8.6): bước giữa `VALIDATED` + `AllowedFunctionNames`, khai báo tool cố định theo bot; còn mở
  đo trên cassette model thật (một hằng `agent.CheDoGiua`); `BiLoai` của retriever từ vựng chưa
  đếm theo từng ràng buộc (rag chưa trả); gu nhóm (ADR-0034 `chia_gu`) chưa có trong
  `group_snapshot`; engine (`aiharness.Engine`) chưa nối `tactu` — khi nối, cổng `aigate` của Nếp
  phải mở allowlist có lý do và canary cho `places`, `destinations`, `outings`, `memberships`.

### 8.2 Tích hợp: đường Nếp trong engine (nhánh `agent-core/integrated`)

Ba nhánh `agent-core/router`, `agent-core/tools`, `agent-core/crag` gộp một chỗ và nối vào `Engine.Run`
của Nếp (sau cờ `MOBILE_AI_ENGINE_NEP=go`, cờ vẫn ở `brain`). Đường duy nhất (`aiharness/dinhtuyen.go`):

1. màn tiền của phiếu (kiểm cấu trúc route, 0 lời gọi) → tiền xử lý cấu trúc (NFC, ký tự ẩn, mention);
2. `hieu` (1 lời gọi, sửa tối đa 1 lần khi còn ngân sách): danh sách điểm đến đóng đọc qua cổng
   `DocCho`; `money_action`/`split_draft` → `nep_khong_cham_tien`; `chen_lenh` → `HanChe` (chỉ tool
   đọc); `nhay_cam`/`ngoai_pham_vi` → trả lời thẳng không tool, không truy hồi, thêm lời dặn của nhãn;
   `hoi_lai` → câu hỏi và các lựa chọn của model qua kiểm cấu trúc rồi qua verifier;
3. ràng buộc của router thành `truyhoi.Cung`/`Mem` (`tools.RangBuocTuRouter`, số học trên ngày ISO);
4. đường: router chọn `truy_hoi_mot_buoc` với đúng một ý định `find_places`/`app_help` và đủ slot →
   `traloi.Chay` (retriever `aidoc.Lexical` hoặc `tools.SoTay`, `crag` chấm, tối đa một vòng sửa do
   model chọn, trả lời có cấu trúc, grounding thuộc tập, verifier, sinh lại một lần, dự phòng cố định);
   `tra_loi_thang` → một lời gọi không tool; còn lại → `tactu` (đường nhanh `explain_screen` hoặc vòng
   ADK ≤3 bước, bước cuối `NONE`, giữ một lời gọi cho verifier);
5. văn xuôi (đường thẳng và `tactu`): token `[[p:…]]` render từ sổ cái, kiểm cấu trúc trước (mã kiểm,
   trích lời nhắc, định dạng liên lạc — không tốn lời gọi, không gửi rò rỉ sang model khác), rồi
   verifier: cờ `hua_hanh_dong_khong_co`/`tien` luôn chặn; câu `khong_ho_tro` luôn chặn, có bằng chứng
   hay không (§8.3); ghi trí nhớ model xếp hàng trong lượt chỉ được ghi sau khi câu trả lời qua hết.

Số lời gọi mô hình theo đường (stub, đo ở T1 và test engine): từ chối tiền 1; hỏi lại 2 (router, verifier); thẳng 3;
truy hồi 4 (router, chấm, trả lời, verifier) — 6 khi verifier bắt và sinh lại; vòng tool 4 với một
bước tool; trần 8 giữ qua `llm.Dem` và `DaGoiTruoc`.

Dữ liệu tool: `cmd/core` nối `aidoc` (catalogue, điểm đến, khu vực, chuyến sắp tới của chính người
hỏi) cho Nếp; cổng trí nhớ dài hạn chưa có adapter sản xuất (hàng hạ tầng) nên tool trí nhớ trả
`loi_nguon`. Ngắn hạn: thiết bị là nguồn phiên và gửi lại mỗi câu (phương án A của nghiên cứu
stm-personalization, ADR-0036 §4), máy chủ không giữ phiên. Ví dụ few-shot của router nhúng lười ở
lượt đầu (`hieu.KhoViDuLuoi`): khởi động không gọi nhà cung cấp embedding.

`aigate`: đường Nếp nay đọc bảng qua tool — danh sách cho phép có lý do từng bảng (`nepCongCuDoc`:
catalogue và chỉ mục của nó, điểm đến, chuyến đi/thành viên của chính người hỏi) và canary: mỗi mục
phải thật sự bị với tới; cổng nay thấy hàm đăng ký trong biến cấp gói (bảng tool) và câu SQL nối chuỗi.

Còn mở: xem cuối §8.3.

### 8.3 Sửa sau ba review của nhánh tích hợp (no-heuristics, correctness, privacy)

**Verifier và đầu ra.**

- Verifier phải chấm **mọi** câu: enum `ket` thêm `khong_thong_tin` (câu chào, câu hỏi lại, lời mời
  xem thẻ) để việc «câu này không nêu gì» cũng là một phán đoán được ghi ra; `Doc` từ chối đầu ra thiếu
  câu, nên `{"menh_de":[]}` (model lười hay lệnh cài trong bằng chứng) không còn phát được gì. Trên
  đường truy hồi (`traloi`), chỉ câu `khong_ho_tro` là phát hiện; câu `khong_thong_tin` không bắt sinh lại.
- Câu `khong_ho_tro` chặn trên mọi đường, kể cả khi lượt không có bằng chứng: câu trả lời thẳng hay
  vòng tool không gọi tool nào không phát được quán, giá, giờ bịa.
- Câu hỏi lại của router (và từng lựa chọn còn lại sau kiểm cấu trúc) qua verifier như mọi văn xuôi:
  hỏi lại tốn 2 lời gọi.
- Cờ dị ứng ngoài danh mục đi vào yêu cầu mà bộ chấm và bước trả lời đọc (`cung.di_ung_ngoai_danh_muc`),
  và mọi câu trả lời của lượt mở đầu bằng câu lưu ý cố định `cau.DiUngNgoaiDanhMuc`, bất kể đường nào.

**Nhãn của router.** `nhay_cam` và `ngoai_pham_vi` đưa lượt ra khỏi tool và truy hồi (trả lời thẳng,
vẫn qua verifier). Lượt `nhay_cam` **không phân biệt được với một lượt sạch** qua bất cứ gì được lưu hay
gửi đi: bản ghi và dòng log ghi `nhan_guard = sach` và, trên đường thẳng mà nhãn ép, các nhãn chuẩn của
một lượt chào hỏi sạch (`huong = tra_loi_thang`, `y_dinh = smalltalk`, `so_y_dinh = 1`); lời dặn khi tin
nhắn chạm chuyện nhạy cảm (`prompts.LoiDanThang`) nằm trong **mọi** câu trả lời thẳng, nên yêu cầu gửi
provider — và số token — giống từng byte. Chỉ `ngoai_pham_vi` (được lưu) nối thêm lời dặn riêng. Metrics
schema v3 bỏ nhãn khỏi CHECK; v4 viết lại hàng cũ (nhãn rỗng mà còn cột router) theo cùng cách và CHECK
không nhận hàng như thế nữa — hàng metrics trỏ tới lời gọi, lời gọi trỏ tới người.

**Tool và chèn lệnh (cấu trúc, không đọc chữ).**

- Hai cổng, mỗi cổng tự đứng được:
  - **Ý định của người hỏi.** `remember_fact` chỉ có trong bộ tool khi router (đọc tin nhắn của chính
    người hỏi, đầu ra có schema) ghi ý định `remember`; `forget_fact` chỉ khi có `forget`
    (`BoiCanh.YDinh`). Không có ý định thì model không thấy tool, gọi bừa thì bị từ chối, không xếp
    hàng gì. Chữ vào dưới dạng dữ liệu (quán, sổ tay, trí nhớ, phiếu màn hình, lịch sử) không thêm được
    ý định nào.
  - **Dữ liệu đã vào lượt.** Sau khi một tool trả dữ liệu bất kỳ (quán, chuyến đi của thành viên khác,
    hàng danh mục, sổ tay) không tool có tác dụng phụ nào (lớp `tri_nho`, `nhac`) chạy nữa. Sau trí nhớ
    của chính người hỏi thì không ghi điều mới; chỉ `forget_fact` còn chạy (người hỏi đã nhờ quên, và
    cần bí danh `recall_memory` vừa cho; xoá chỉ thu hẹp cái được giữ).
  - **Chỉ lời của chính người hỏi.** `noi_dung` của `remember_fact` phải là một đoạn nguyên từ của tin
    nhắn lượt này (so từng từ bằng đồng nhất, bỏ dấu datamark), và đoạn được lưu lấy từ tin nhắn chứ
    không từ chữ model viết (`tools.kiemGhiNho`). Chữ từ lịch sử thiết bị, phiếu màn hình, dữ liệu tool,
    hay lời model diễn giải thì không thành ký ức được, dù model bị bảo gì. `nepnho.Ghi` chỉ gửi sang
    trích xuất của sidecar (vai `user`) fact nguồn `noi_ro`. Theo quyết định sản phẩm «lặng lẽ, có công
    bố»: ghi không cần chạm xác nhận, câu trả lời công bố việc sẽ nhớ.
  - **Cá nhân hoá là bằng chứng.** ≤5 ký ức nhớ lại vào sổ cái của lượt như bằng chứng nguồn `memory`
    (bí danh f1…), verifier nhận chúng, khối `tri_nho` render qua `cautruc.DongBangChung` (datamark), và
    sau đó không ghi điều mới (`BoiCanh.NapTriNho`). Lượt router đọc là nhờ nhớ thì không cá nhân hoá.
- `remember_fact`/`forget_fact` chỉ **xếp hàng**; `BoiCanh.CamKet` ghi sau khi câu trả lời qua verifier
  và mọi kiểm đầu ra (kể cả lần kiểm cấu trúc cuối khi mở đầu bằng câu lưu ý dị ứng) — lượt không phát
  gì thì không ghi gì. Mỗi lượt tối đa một điều mới, ghi **sau** mọi lệnh quên: lỗi giữa chừng không để
  lại điều mới nào của lượt bị giữ lại (quên đã chạy thì chỉ thu hẹp). Verifier nhận các việc đã xếp
  (`BoiCanh.ViecCho`): «mình sẽ nhớ …» chỉ không bị tính là hứa suông khi thật sự có việc ghi đó.
- Lời gọi tool trả về ở bước `NONE` (hay sau `tra_loi_ngay`) bị từ chối, không chạy (`agent.CauHinh.BuocCuoi`).
- Dữ liệu tool trên đường `tactu` được datamark từng trường, cắt 300 rune, JSON không escape `<>` để khối
  tự đổi chúng sang toàn khổ.

**Truy hồi.**

- Khung giờ router viết là **khung** (`truyhoi.Cung.MoTrong`, ánh xạ `rag.YeuCau.Khung`, «mở vào lúc nào
  đó trong khung»); chỉ khi không có `den` mới là thời điểm (`MoLuc`). Qua nửa đêm thì `den` là ngày sau.
- Mọi `truy_van` router viết cho nguồn đều chạy, cùng ràng buộc cứng, kết quả trộn xoay vòng.
- Lỗi retriever trên đường truy hồi không còn ghi là lỗi mô hình: lượt chuyển sang đường tool, tool trả
  `loi_nguon`.
- `huongdan.Tim` bỏ bảng teencode, luật f→ph/w→qu và cổng từ dừng: BM25 trên mọi âm tiết của truy vấn
  model viết, ghim theo id màn. Bộ vàng thứ ba `truy-hoi-truy-van-model.json` (26 câu teencode viết lại
  như router được dặn) đạt recall@5 1.0000, MRR 0.8942.

**Ngân sách.** Kho ví dụ của router dựng một lần, hạn riêng 10 s, không giữ khoá qua mạng; lượt chờ tối
đa 2 s rồi định tuyến không ví dụ. `MaxEmbedCallsPerTurn` nay được cưỡng chế (`nhung.DemLuot`, engine
đặt vào `hieu.Vao.DemNhung`), embedding câu hỏi có hạn riêng 3 s.

**Cổng.** `aiharness/khong_heuristic_test.go` đi theo chữ của lượt (câu hỏi, lịch sử, truy vấn và nội
dung trí nhớ model viết) qua biến, trường, tham số, giá trị trả về và closure trên 13 gói của đường
engine (gốc `aiharness`, `agent`, `cautruc`, `crag`, `hieu`, `kiemchung`, `tactu`, `tools`, `traloi`,
`trinho`, `truyhoi`, `aidoc`, `huongdan`): chữ chỉ được làm sạch cấu trúc, cắt, vào khối datamark, kiểm
rỗng/độ dài/UTF-8/định dạng riêng tư, hoặc đưa cho bộ xếp hạng. Canary `testdata/khongheuristic` đỏ ở
sáu cách đọc nghĩa mà review thấy lọt. `traloi`: đường import cấm sửa thành `domain/tuvung`, có canary
đường tồn tại. `aigate`: nhóm không với tới tool trí nhớ, Nếp thì có.

Cổng cũng đi theo **văn xuôi model viết** (câu trả lời của vòng agent `agent.Chay`, `tactu.Ra.Text`,
câu nháp `traloi.Cau.Chu`, `traloi.KetQua.Chu`, câu hỏi lại và lựa chọn của router): luật cụm từ đoán
«tự nhận hành động/tiền» từ câu trả lời đã gỡ và không được quay lại (phán đoán đó là của verifier).
Ngoại lệ có tên, đọc tay: ba bộ tách cấu trúc (`traloi.tachCau` đọc token `[[p:…]]` và nhãn «…» của
schema mình, `GhepVanXuoi` dựng token từ sổ cái, `TachCauVanXuoi` tách câu ở `. ! ? …` và xuống dòng),
output guard `guard.DauRa.Kiem` (mã canary, trích lời nhắc, định dạng riêng tư), và ba trường chứa bí
danh/nhãn đem so thuộc tập (grounding). Ba canary mới (cụm từ trên câu trả lời, trên câu hỏi lại, trên
kết quả của `agent.Chay`) đỏ; cổng cũ xanh trên đột biến «chặn khi câu trả lời chứa "đã chuyển"».

**Để lại có lý do (minor).**

- `tuvung.LaTuDung` trong `rag/yeucau.go` bỏ từ dừng khỏi truy vấn OR của `rag`: đây là **chấm điểm
  truy hồi** (thứ hạng), không lọc và không quyết định; dùng chung với route parity `/places/search` nên
  không đổi byte ở đây.
- Output guard dùng đơn vị tiền (`đồng`, `nghìn`, `triệu`, `k`) để phân biệt một dãy số là số tiền hay số
  tài khoản (`tienSau`, `tienDonVi` trong `guard/output.go`): đây là **kiểm định dạng dữ liệu vì quyền
  riêng tư**, không phải hiểu câu; hệ quả đã biết: một dãy chín chữ số theo sau là «đồng» được phát, cùng dãy đó đứng một mình bị chặn.
- `tu_tin` và `mo_ho_voi` của router chỉ được ghi (eval), chưa đổi đường; đổi cần đo T3.
- `my_upcoming_outings` giữ trong bộ tool của Nếp chờ Lead quyết theo ADR-0033 §4; tiêm lệnh qua tiêu
  đề chuyến đi đã bị chặn về cấu trúc (datamark, không ghi sau dữ liệu ngoài).
- Câu trả lời thẳng bị verifier chấm không có bằng chứng: câu nhắc lại điều ở lịch sử (quán lượt trước)
  có thể bị chấm `khong_ho_tro` và bị giữ. Hướng an toàn (giữ, không phát bịa); nới cần đưa bằng chứng
  tham chiếu của lượt trước vào verifier, đo ở T3.
- Câu trả lời dài hơn `kiemchung.MaxMenhDe` câu: phần dư gộp vào câu cuối (`TachCauVanXuoi`) nên vẫn được
  chấm; câu hỏi lại cộng lựa chọn vượt 12 câu thì verifier từ chối và lượt giữ câu trả lời (fail-closed).

Còn mở: T3 thật (khoá + Lead duyệt số lời gọi); độ trễ nháp-rồi-kiểm (chưa stream, `crag-kiem-chung.md`
§3 cần Lead chọn); `set_reminder`, gu nhóm, adapter trí nhớ sản xuất; bước giữa `AUTO` hay `VALIDATED` (đã chọn `VALIDATED`, §8.6);
verifier nay nhận các việc ghi/quên đã xếp (`BoiCanh.ViecCho`), nhưng việc model verifier thật cho qua
«mình sẽ nhớ …» đúng khi có việc và giữ khi không có mới chỉ đo trên stub — cần đo ở T3; ADR-0033 §4 cho
`my_upcoming_outings`; bot nhóm vẫn đi brain.

### 8.4 Truy hồi vector: một schema, một writer, và luật «chưa rõ» (nhánh `infra/rag-unified`)

**Một schema, một writer.** Chỉ mục vector là bản sao dựng lại được của PostgreSQL. Schema collection
khai **một lần**, trong `internal/vectordb` (`PhienBanLuocDo = "rd.v2"`); cấu hình nạp (`rag/nap/cauhinh.json`)
chỉ ghi tên bản schema nó viết cho, và `vectordb/napkho` từ chối dựng collection khi hai bên lệch (bản schema,
số chiều, hằng RRF, độ rộng ô giờ, tên alias). Nạp là writer duy nhất của mọi collection `rd_*` (qua `napkho`,
trên hàm dựng client duy nhất `vectordb.Ket`) và của `place_enrichments`; truy hồi (`hybrid`) chỉ đọc. Bước kiểm
lại của truy hồi đọc **đúng bảng nạp ghi**: `thuoctinh.Doc` đọc hàng `places` sống, áp `nap.ApDung` lên
`place_enrichments` theo mã băm nguồn hiện tại, và `rag_tombstones` — cùng một luật với lúc nạp, nên chỉ mục cũ
chỉ làm mất kết quả, không bao giờ làm hiện một quán vi phạm ràng buộc cứng.

**Luật «chưa rõ»** (một luật cho mọi đường truy hồi của engine — hybrid, fallback lexical, bộ lọc Go của nạp,
biểu thức Milvus):

| Thuộc tính | Ràng buộc đó là **cứng** (model trích vào `truyhoi.Cung`) | Không có ràng buộc đó |
|---|---|---|
| Dị ứng chưa xác lập (`khong_ro`, hoặc làm giàu chưa có người duyệt) | **Loại** với mọi dị ứng | Giữ |
| Giá chưa rõ | **Loại** khi có ngân sách | Giữ, bằng chứng mang `chua_ro: gia_chua_ro` |
| Giờ mở chưa rõ | **Loại** khi có thời điểm hay khung giờ | Giữ, bằng chứng mang `chua_ro: gio_chua_ro` |

Đây là lựa chọn chặt hơn thiết kế 04 §4a (ở đó quán «chưa rõ» được giữ, gắn cờ, xếp cuối và vào khi còn dưới
3 quán biết chắc): mọi ràng buộc engine chuyển xuống đều là ràng buộc cứng và luật «không bao giờ nới» thắng —
ít kết quả hơn là câu trả lời đúng, và câu trả lời nói rõ chỗ chưa biết thay vì đoán. Cờ «chưa rõ» dùng khi
ràng buộc đó không được hỏi, để câu trả lời nói «chưa có giờ mở cửa/giá». `POST /places/search` (route parity)
vẫn giữ hành vi §4a của `rag.Retrieve`; đường engine lọc lại (`aidoc.GiuChuaRo`).

**Dị ứng chắc chắn chỉ khi có người duyệt.** Làm giàu tự động nói «không có chất dị ứng» (`di_ung: []`,
`tin_cay: cao`) **không** làm quán qua bộ lọc dị ứng: `ApDung` chỉ tính chắc chắn khi `review = reviewed`,
và mọi khẳng định chắc chắn về dị ứng đều vào hàng duyệt. Phán quyết gắn với phiên bản đã xem
(`MucDuyet.Ban` = 16 hex sha256 của mã băm nguồn, phiên bản prompt và output): output bị thay giữa lúc liệt kê
và lúc duyệt thì phán quyết bị từ chối.

**Nhánh thưa = BM25.** Tìm kiếm lai là dense `gemini-embedding-2` hợp với hàm BM25 dựng sẵn của Milvus trên
hai trường văn bản. MILCO (sparse học được) **gác lại** chờ xác nhận license (chủ sản phẩm 2026-09-27): mã
adapter còn đó nhưng tắt mặc định (`MOBILE_MILCO_ENABLED`, `milco_bat: false` trong cấu hình nạp), không nối vào
đường truy hồi của engine, và không tầng/test nào cần nó.

**Hai trường BM25.** `text` giữ dấu (analyzer chỉ lowercase: «bún» khác «bùn»), `text_khong_dau` gấp dấu
(thêm `asciifolding`). Gấp dấu là **chuẩn hoá cấu trúc ký tự** — dấu thanh và đ bị bỏ như khi người gõ không bộ
gõ — không đọc nghĩa và không quyết định gì về câu hỏi; nó chỉ để nhánh BM25 thứ hai khớp được truy vấn không
dấu. Trọng số hợp nhất (dense 1, BM25 có dấu 0,1, BM25 không dấu 1) nằm ở `cauhinh.json` «hop», truy hồi phục vụ
với đúng trọng số cổng eval đo. Đo trên tập vàng với encoder stub (KhoNho, ghim ở `rag/nap/vang_test.go`): tổng
recall@10 0,9333 · nDCG@10 0,8439 · violation@10 0 · khoảng cách không dấu 0,0369; lát «bỏ dấu» (mọi câu vàng
gấp dấu) recall@10 0,9133 với nhánh gấp dấu, 0,7733 khi tắt nhánh đó. Trọng số chỉnh trên stub, phải đo lại khi
có encoder thật.

**Truy hồi theo ngữ cảnh.** Lời gọi làm giàu (offline, `core rag v-enrich --tran-goi N` hay indexer, không bao
giờ trong lượt) viết thêm một câu ngữ cảnh ngắn (≤160 ký tự, qua `TextSafe`) cho mỗi chunk (hồ sơ, đánh giá);
câu đó đứng đầu văn bản chunk nên cả nhánh dense lẫn hai nhánh BM25 đều đọc, và mã băm nội dung phủ nó. Làm giàu
cũ, bị từ chối hay mang nhãn chèn lệnh thì không có câu ngữ cảnh.

**Thuộc tính tới bản đang phục vụ.** Khi cấu hình đổi mà bản mới chưa promote, collection đang phục vụ được dựng
với cấu hình khác: indexer ghi lại thuộc tính lọc cứng của quán thay đổi vào mọi collection sống khác cấu hình
(upsert từng phần), ghi không được thì xoá quán khỏi đó (fail closed). Promote kiểm lại vân tay cấu hình và
sha256 tập vàng của phán quyết đang dùng.

### 8.5 ADK-Go v2 và Go 1.26 (nhánh `toolchain/adk-v2`, quyết định của owner 2026-09-27)

`google.golang.org/adk` v1.7.0 → `google.golang.org/adk/v2` v2.4.0 (bản mới nhất; go.mod của nó đòi
`go 1.26.6`), nên Go 1.25.14 → go1.26.8. Hành vi engine giữ nguyên: ngân sách (`llm.Dem`,
`MaxModelCallsPerTurn`, trần bước `MaxBuoc`), luật chế độ AUTO/NONE (nay VALIDATED/NONE, §8.6), bảng quyền, từ chối ở BeforeTool, cổng OTel.

Những gì v2 bắt phải đổi:

- **Một `agent.Context` thống nhất**: `agent.CallbackContext` và `agent.ToolContext` gộp làm một; callback
  model/tool và `functiontool.Func` nhận `agent.Context`. `session.NewEventWithContext` → `session.NewEvent(ctx, id)`.
- **Runner chạy LlmAgent gốc qua runtime node của workflow** (workflow một node START → agent). Agent của ta
  không khai `Mode`, gốc được coi là chat, không có sub-agent, `Compaction` để nil: đường model/tool như v1;
  `ErrHetBuoc` vẫn tới người gọi (`agent_test.go`).
- **Lượt chỉ có thought**: v2 gọi lại model (tối đa 10 lần, kèm lượt user tổng hợp «Continue processing…»).
  v1 kết thúc lượt với câu trả lời rỗng. `sauMoHinh` thêm một phần text rỗng vào phản hồi chỉ-thought để lượt kết
  thúc như v1 (`TestThoughtOnlyReplyEndsTheTurn`; bỏ shim thì lượt vượt trần bước).
- **Model gemini** thôi chèn «Handle the requests as specified in the System Instruction.» khi `Contents` rỗng.
  Mọi lời gọi của engine đều có ít nhất một content người dùng, nên không đổi gì.
- **Telemetry**: nội dung prompt/câu trả lời lên span/log record chỉ khi
  `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT` bật. Gói `aiharness/otelchan` xoá biến đó lúc init;
  `llm` và `agent` import nó, và `aigate` gác rằng mọi gói dùng ADK đều phụ thuộc nó. Cổng provider cấm thêm
  `adk/v2/server` (debug telemetry của nó cài exporter SDK), `contrib/detectors` và đường `adk/telemetry` của v1.
  Tham số/kết quả tool vẫn lên span vô điều kiện như v1; không có provider thì chúng không đi đâu.

Go 1.26 đổi DCT của `image/jpeg` (fdct.go/idct.go → dct.go). Mã sản phẩm không dùng `image/jpeg` (sanitize dùng
`jpegenc`/`jpegdec` riêng), nhưng corpus test dựng input JPEG bằng encoder chuẩn, nên cùng spec ra byte khác và
golden (đáp án Pillow cho byte cũ) lệch 6 ca. Encoder của go1.25.14 được đóng băng ở
`media/sanitize/internal/corpus/stdjpeg125` (chỉ dùng cho test); golden giữ nguyên. Parity `imagegen` vẫn dùng
encoder chuẩn: byte input đổi nhưng hai stack nhận cùng byte.

**Việc sau (chưa dùng trong thay đổi này):**

- *Workflow đồ thị* (`workflow`: node hàm/agent/join, rẽ nhánh, HITL qua long-running tool): có thể thay chuỗi
  hiểu → truy hồi → agent → kiểm chứng viết tay bằng một đồ thị có span theo node. Phải giữ ngân sách lời gọi
  model xuyên node và luật một writer.
- *Chế độ task / single_turn* của llmagent (`finish_task`, lịch sử theo phạm vi cô lập): hợp với bước
  «cấu trúc» và «kiểm chứng» chạy một lượt không cần hội thoại.
- *Nén ngữ cảnh* (`runner.Config.Compaction`, `session/compaction`: cửa sổ trượt + giữ đuôi, summarizer
  mặc định là model của agent gốc): engine đang dựng session trong bộ nhớ cho từng lượt và tự cắt lịch sử panel,
  nên chưa cần; nếu bật thì mỗi lần tóm tắt là một lời gọi model phải vào `llm.Dem` và phải qua cổng riêng tư
  (bản tóm tắt là nội dung chat).
- *Context caching*: ADK-Go v2.4.0 **không** có cấu hình cache tường minh như `ContextCacheConfig` của
  adk-python. Cache ngầm của Gemini vẫn tự áp và đã được đếm (nay `llm.Dem.Token` trên mọi lời gọi, §8.6). Muốn cache tường minh thì
  phải gọi API cache của genai ngoài ADK.
- *`platform.WithTaskRunner` / `WithTimeProvider` / `WithUUIDProvider`*: tool call và id/thời điểm event
  tất định, dùng được cho test engine phát lại byte.
- *`agent.StrictContextMock`*: fake context cho test callback mà không phải vá lại khi interface lớn thêm.

### 8.6 Reranker trong engine và năm lỗ SOTA không tốn lời gọi (nhánh `ai/rerank-sota`, 2026-09-27)

Nguồn: quyết định reranker của chủ sản phẩm (ADR-0043 §2.6) và `docs/claude/2026-09-25/doi-chieu-sota.md`
(gap #3, #5, #7, #10, rủi ro D.6). Không thêm lời gọi model nào; mọi thứ dưới đây là cấu trúc, tập hợp và đếm.

**Reranker** (chi tiết ở ADR-0043 §2.6). `cmd/core` dựng `rerank.TuEnv` từ `MOBILE_RERANK_URL`,
`MOBILE_RERANK_MODEL` (mặc định `Qwen3-Reranker-4B`), `MOBILE_RERANK_TIMEOUT` (mặc định 3s),
`MOBILE_RERANK_TOKEN` (bearer tuỳ chọn) và đưa vào `aiharness.WithXepLai`. Mỗi lượt Nếp bọc reranker trong
`rerank.Dem` (`MaxRerankCallsPerTurn` = 2) và mang nó trong ngữ cảnh (`truyhoi.VoiXepLai`): retriever hybrid
dùng chung rerank tối đa 30 ứng viên RRF đầu cho tool `search_places`; vòng sửa của đường truy hồi rerank một
lần trên ứng viên đã trộn của mọi truy vấn router, retriever thì nhường (`truyhoi.HoanXepLai`). Không cấu
hình, hỏng, quá hạn, mạch mở hay hết ngân sách: thứ tự RRF và cờ `no_rerank`, không thử lại trong lượt. Điểm
chỉ ở `DiemXepLai`, không bao giờ là ngưỡng.

**Hai dạng truy vấn của router (gap #3).** Mỗi phần tử `truy_van` có `cau` — câu **tự đủ nghĩa**, mọi tham
chiếu tới lượt trước đã giải, giữ cách viết của người hỏi (không dấu thì để không dấu) — và `cau_co_dau`
(tuỳ chọn trong schema, nhưng không được rỗng khi có; bằng `cau` thì coi như không có) — cùng câu đó đã
**khôi phục dấu**, teencode viết ra. Cả hai do model viết (khôi phục dấu là đọc chữ, không phải việc của
Go). Retriever dùng: nhánh dense, trường BM25 có dấu và reranker đọc `cau_co_dau`; trường BM25 gấp dấu đọc
`cau` (`vectordb.ThuaTruyVan.TextKhongDau`). Router viết `truy_van` cho cả `tac_tu`. Không HyDE, không lời
gọi viết lại riêng. Vòng sửa viết lại truy vấn thì bỏ dạng có dấu cũ.

**Bố cục cho cache ngầm của Gemini (gap #5).**
- System instruction tĩnh theo bot và theo bước (router, chấm, trả lời, verifier: file nhúng; agent:
  `NepAgent` + `CongCu`, đường trả lời thẳng dùng `NepAgent` là tiền tố byte của nó, rồi `LoiDanThang`
  cố định). Ngoại lệ có chủ ý: lượt `ngoai_pham_vi` nối lời dặn của nhãn vào system — an toàn thắng cache;
  `nhay_cam` không nối gì (không để yêu cầu lộ nhãn).
- **Khai báo tool cố định theo bot**: mọi bước, mọi lượt khai đúng tập bảng quyền cấp cho bot, theo thứ tự
  sổ đăng ký (`tools.BoiCanh.BoCongCu`). Cái bước được gọi thu hẹp bằng
  `FunctionCallingConfig{Mode: VALIDATED, AllowedFunctionNames}` (`tools.BoiCanh.TenChoPhep`: bỏ tool
  hạn chế theo `chen_lenh`, theo ý định ghi, và tool ghi trí nhớ sau khi đã có dữ liệu; rỗng thì `NONE`) —
  «che, không gỡ». Tool bảng quyền không cấp cho bot thì không bao giờ khai (quyền, không phải chính sách).
  `BeforeTool` vẫn từ chối y như cũ. Đã đọc go-genai v1.71.0: chú thích của hằng
  `FunctionCallingConfigModeValidated` nói `allowed_function_names` giới hạn lời gọi ở chế độ này; chú thích
  của trường `AllowedFunctionNames` còn ghi «chỉ khi ANY» — hai chú thích lệch nhau, cần cassette model thật
  để xác nhận phía server.
- Phần động ở cuối: lượt người dùng của router xếp danh sách đóng → ví dụ → phiếu → ngắn hạn → `may_chu`
  («bây giờ» + lịch) → câu hỏi; lượt của agent để khối ký ức trước khối `may_chu`, câu hỏi cuối cùng.
- Metrics: `llm.Dem` cộng `PromptTokenCount`, `CandidatesTokenCount`, `CachedContentTokenCount`,
  `ThoughtsTokenCount` của **mọi** lời gọi (router, chấm, trả lời, verifier, bước agent) vào `tokens_in`,
  `tokens_out`, `tokens_cached`, `tokens_thoughts` của hàng metrics (trước chỉ bước agent). Chỉ số đếm. Tỉ lệ
  cache theo từng bước chưa có cột riêng.

**Mức nghĩ tường minh từng bước (gap #7).** `llm.MucNghi`: router, sửa router, chấm, trả lời, sinh lại,
verifier và bước trả lời của agent `MINIMAL`; bước lập kế hoạch của agent (gọi hàm bật) `LOW`. Chỉ đặt
`ThinkingLevel`, không bao giờ `ThinkingBudget` (`TestMucNghiTuongMinh`, `TestMucNghiVaKhaiBaoCoDinh`).

**Bất biến taint trong vòng agent (gap #10), tất định, 0 lời gọi** (`tools/taint.go`). Từ bước 2 (sau khi
model đã đọc kết quả tool), mọi tham số của lời gọi tool phải là: giá trị router trích từ lời người hỏi
(slot: điểm đến, ngày, giờ, ngân sách, dị ứng, ăn kiêng, loại chỗ, khí chất — danh sách chỉ được lặp lại
phần tử của router), hoặc câu truy vấn router viết (một trong hai dạng, được đưa vào prompt trong khối
`<du_lieu nguon="truy_van">`), hoặc chữ tự do model đã viết ở bước 1 (trước khi đọc kết quả nào), hoặc id có
trong sổ của lượt (bí danh bằng chứng — mỗi tool tự kiểm — hay id hàng danh mục tool trả trong lượt: điểm đến,
khu vực), hoặc số đếm `k`/enum đóng không có slot router. Ràng buộc cứng được kiểm lại trên tham số (ngày, giờ,
ngân sách phải đúng của router). Tool nháp giữ nội dung nháp của nó (người bấm mới thành gì), nhưng ngày phải
là của router khi router có. Tham số lạ bị từ chối (`TestTaintBietMoiThamSo`). Vi phạm trả `tham_so_sai` như
lỗi tham số, tính vào một lần sửa. Đây là so khớp nguyên chuỗi và thuộc tập; `TestKhongDocNghiaTrenDuongEngine`
vẫn xanh với hai trường mới được gieo.

**Ngân sách xấu nhất (rủi ro D.6).** `llm.KeHoach` liệt kê các bước LLM của từng đường và số lời gọi tối đa:

| Đường | Bước (tối đa) | Tổng |
|---|---|---|
| truy hồi | router 1 · sửa router 1 · chấm 1 · trả lời 1 · verifier 1 · sinh lại 1 · verifier 1 | 7 |
| tác tử Nếp | router 1 · sửa 1 · bước agent 3 (bước cuối `NONE`) · verifier 1 | 6 |
| tác tử nhóm | router 1 · sửa 1 · bước agent 4 · verifier 1 | 7 |
| trả lời thẳng | router 1 · sửa 1 · trả lời 1 · verifier 1 | 4 |
| hỏi lại | router 1 · sửa 1 · verifier 1 | 3 |

Không đường nào vượt `MaxModelCallsPerTurn` = 8. Sufficiency **không** là lời gọi riêng: bộ chấm `crag` là phán
đoán duy nhất và **bị cắt đầu tiên** — `crag.DuTruCham` = 5 (bộ chấm cộng cả chu trình trả lời sau nó), trước là
3 nên với ngân sách hụt nó ăn mất lần sinh lại. Thứ tự cắt: (1) bộ chấm, (2) sinh lại và verifier của nó,
(3) sửa router; không bao giờ cắt router, câu trả lời hay verifier (không còn lời gọi cho verifier thì không
viết câu trả lời, câu dự phòng cố định đứng). `llm.Dem` vẫn là chặn cứng. Test: `TestKeHoachTrongTran` (tĩnh,
gắn với `MaxStepsNep`/`MaxStepsNhom`), `TestXauNhatMoiDuongTrongTran` (chạy từng đường Nếp tới trường hợp xấu
nhất qua `Engine.Run`: số lời gọi bằng đúng kế hoạch), `TestCatTheoThuTu` (lượt thử lại còn 5/3/2 lời gọi).

### 8.7 Nhóm qua engine Go (lát 9, nhánh `ai/group-engine`)

Sau cờ `MOBILE_AI_ENGINE_GROUP=go` (mặc định `brain`; một engine phục vụ cả hai bot, mỗi đường chỉ với tới cổng
của mình). Worker (`chatassist.processNhomEngine`) dựng lượt từ đúng job và phòng: gói người gọi chia sẻ (tin gần
đây, chuỗi trả lời vào thẻ AI), tác giả từng tin theo `messages.author_id` (không đọc `body`), thành viên đang ở
dưới nhãn roster; lệnh cú pháp (`/plan`, `/chia-bill`, `@Rủ Đi` đầu tin) bỏ qua `chatintent.Parse`. Engine
(`Engine.RunNhom`, `aiharness/nhom.go`):

1. tiền xử lý cấu trúc (NFC, ký tự ẩn, mention) trên lời nhờ và tin chia sẻ; phòng v2 không đưa tin nào tới model
   hay kho ngắn hạn (`luot_bo` đếm); lane cũ đệm tin vào `aictx` khoá nhóm (EX 900, UNLINK cuối lượt);
2. router nhóm: schema không có nguồn/ý định trí nhớ; thành viên là bí danh `m1…` (id người không vào prompt);
   router đọc tới 40 tin chia sẻ (`trinho.MaxLuotNhom`, đúng số thẻ nói đã đọc);
3. chính sách (hàm thuần của nhãn model và lệnh tường minh): `money_action` → câu cố định
   `cau.NhomKhongChamTien` làm thẻ chữ, 1 lời gọi, không tool; `/chia-bill` hay `split_draft` → nháp;
   `/plan` ép ý định `plan` và đi vòng tool; lệnh không bao giờ hạ nhãn tiền; `chen_lenh` chỉ còn tool đọc;
4. `find_places` một bước → `traloi` (CRAG, trả lời có cấu trúc, verifier), phần `places` là id bằng chứng; còn
   lại trả lời thẳng hoặc vòng ADK với bộ tool nhóm (bảng riêng `congCusNhom`, `draft_poll` bị che tới khi có
   kind thẻ bình chọn nháp); nháp lịch trình thành phần `itinerary`;
5. verifier ở ngữ cảnh mới, kiểm cấu trúc đầu ra (trần 1500, lời nhắc nhóm), rồi `phatRa` qua cửa sổ 48 rune
   như Nếp; thẻ `tra_loi` qua `GroundReply` với hàng danh mục worker đọc (`places` READ ONLY) rồi publish một lần,
   trả lời vào tin tag. Từ lát 12 worker chạy engine với `aiharness.ChiTrangThai` (trạng thái đi ngay, chữ không
   đi, không chờ nhịp) và chữ nhả từ chính thẻ **sau** commit (§4.2); lát 9 từng nhả trước commit.

`chia_bill`: **một** lời gọi flash-lite có cấu trúc (`aiharness/chiabill`, thay 8 lời gọi `chat-expense` của
brain): mỗi khoản là bí danh tin (chỉ tin có tác giả đã xác nhận, hoặc lời nhờ của người gọi), tiêu đề chỉ giữ khi
là đoạn nguyên từ của chính tin đó (so đồng nhất, lấy từ tin), số tiền là số nguyên JSON trong
`[1, allocator.MaxAmountVND]` — số lẻ/số mũ/chuỗi bị từ chối, không làm tròn. `chiaBillParts` thuần: tổng
int64 có chặn trần, xem trước chia đều do `allocator.Allocate` (phần dư lớn nhất), Σ phải bằng tổng không thì không
dựng gì; thẻ là mẫu cố định + phần `expense_draft{so_khoan, da_ghi:[]}`; nháp (v1 `expense_draft` + `chia_deu`)
ở cột `result`. Không verifier trên đường này (`llm.KhongKiem`): không có văn xuôi model nào được nhả.
Lệnh `hoi` của nhóm: `chatassist` version 6 nới `chat_ai_command_scope`; handler chỉ nhận khi cờ bật và có tin
tag; `chat-capabilities` có `ai.hoi`. `ai_turn_metrics` version 5 thêm đường `nhap_chia_bill`.
Cổng: `aigate` thêm gốc `processNhomEngine` (15 gốc), canary với tới `RunNhom`, tool nhóm, `chiabill.Goi`, khoá
`aictx` nhóm và đỏ khi thêm `HoSoNep`; `khong_heuristic` thêm `chiabill` và các trường chữ nhóm, hai canary mới;
T1 nhóm `aieval/testdata/corpus/nhom-kich-ban.json` (bất biến 4 và 9 của thiết kế 06 §6.1).
