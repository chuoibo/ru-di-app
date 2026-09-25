# ADR-0037 — Khung agent Go: ADK-Go, Gemini gọi từ Go, một pipeline có hậu kiểm

- Ngày: 2026-09-25.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0037 khác thì
  văn bản này nhận số trống kế tiếp, như ADR-0036 từng đổi số.
- Quyết định sản phẩm: người dùng chốt trong phiên lập kế hoạch 2026-09-25 (model, ADK-Go, Gemini từ
  Go, Python chỉ còn eval/extraction/skill cũ). Kế hoạch đã duyệt; ADR này chờ chữ ký Lead.
- Thiết kế chi tiết: `docs/claude/2026-09-25/thiet-ke-ai/01-harness-agent.md` (commit gốc `f251db7`).
- Cùng đợt: ADR-0038 hàng đợi + streaming, ADR-0039 nhóm trong luồng, ADR-0040 RAG + nạp dữ liệu,
  ADR-0041 Nếp, ADR-0042 eval. Văn bản này chỉ quyết khung engine.
- Sửa và thay một số điều khoản cũ, liệt kê ở mục 5; **không sửa bản lịch sử** của ADR nào.
- Không đổi bởi văn bản này: ba luật tiền, ADR-0004, ADR-0033 §2.2 (Nếp lui ở màn tiền), ADR-0036 §2.2,
  §2.4 và §2.9 (ADR-0036 §2.1, §2.5 do ADR-0039 sửa; §2.3 do ADR-0040 sửa).

## 1. Bối cảnh

Hai con AI («Rủ Đi AI» trong nhóm, Nếp riêng từng người) dùng chung `services/core/internal/chatassist`,
và ở `f251db7` mỗi job chỉ làm một việc: gửi một payload sang brain Python, nhận về một câu trả lời.

- Một lời gọi model mỗi job (`worker.go:125`, `nep.go:408`), không `system_instruction` (prompt dán
  chung với dữ liệu, `companion_gemini.py:149`), không biết «bây giờ» theo giờ Việt Nam, không tool,
  không truy hồi, không streaming. Eval nhóm 10/16.
- `chia_bill` gọi `chat-expense` tuần tự tới 8 lần trên `gemini-2.5-flash` (`chiabill.go:41`,
  `chat_expense_gemini.py:15`), trái lựa chọn một model cho mọi bước sinh chữ.
- Giới hạn 8 lần/phút đếm **lời gọi**, không đếm lời gọi model (`handler.go:414-418`).
- Muốn có guard, hiểu câu hỏi, nhiều ý định, tool, truy hồi và trí nhớ thì vòng lặp phải nằm cạnh dữ
  liệu và cạnh cổng quyền. Các thứ đó đều là Go. Để Python giữ vòng lặp là để Python làm orchestration,
  điều ADR-0031 §1 và CLAUDE.md đã cấm.
- ADK-Go v1.7.0 đòi `go 1.25.0`; ở `f251db7` core và parity ở Go 1.23, toolchain 1.23.4 (lát 3 đã
  nâng lên 1.25.14 ở `637b7f3`).
- ADK tự ghi nguyên JSON kết quả tool vào span telemetry, và không có trần bước nào.

## 2. Quyết định

1. **Nâng Go lên 1.25** cho `services/core` và `parity`: dòng `go 1.25.0`, toolchain 1.25 patch mới
   nhất lúc thi công, image build ghim digest, Dockerfile chép cả `go.sum`. Có test ghim
   `norm.Version == "15.0.0"`, vì so khớp Unicode với oracle Python 3.12 phụ thuộc vào nó.
2. **ADK-Go (`google.golang.org/adk` v1.7.0) và genai (`google.golang.org/genai` v1.71.0).** Gemini
   được gọi **trực tiếp từ Go**. Mọi bước sinh chữ dùng một hằng model duy nhất, `gemini-3.5-flash-lite`.
   Model embedding là việc của ADR-0040.
3. **Một engine, một seam.** Gói `internal/aiharness`, `Engine.Run(ctx, Turn, Sink)`. Worker và eval
   gọi đúng hàm này. Không bản thiết kế nào khác được có vòng lặp, router hay registry tool thứ hai.
   RAG là phần ruột của tool, không phải một orchestrator.
4. **Pipeline cố định:** preprocess tất định → guard tất định → Understand → fast path hoặc agent loop
   → hậu kiểm → worker publish **một lần** trong một transaction.
   - Understand là **một** lời gọi, đầu ra chỉ enum và slot, được bọc `<du_lieu>` và qua guard trước khi
     vào prompt kế tiếp.
   - Hậu kiểm gồm grounding trên đúng tập tool đã trả, provenance, và output guard cửa sổ 48 rune.
     Output guard là nơi **duy nhất** sinh `delta`; không có «rút lại» sau khi đã nhả.
5. **Trần cứng.** `MaxModelCallsPerTurn` trong `aiharness/llm` đếm mọi lời gọi, kể cả retry, và eval đọc
   hằng đó để tính dự toán. Retry do `llm` tự làm, chỉ trước delta đầu. Có trần bước, trần tool call,
   semaphore DB toàn tiến trình, hạn lượt; bước cuối ép `Mode NONE`.
6. **Bảng quyền tool.** Một registry (`aiharness/tools`), tên chốt trong hợp đồng chung, golden
   `quyen.golden.json` nói bot nào gọi được gì, và BeforeTool kiểm lại.
   - Tool đọc chạy trong transaction `ReadOnly`.
   - Tool nháp không chạm DB.
   - Ghi duy nhất là «ghi của chính mình» của Nếp, qua writer `nepnho` (luật ở ADR-0041).
   - Không tool nào ghi tiền, nghĩa vụ, kèo, bình chọn hay tin nhắn.
7. **Phiên ADK sống đúng một lượt** (`session.InMemoryService`, bỏ khi xong). Không dùng `memory.Service`
   của ADK.
8. **Quan sát không nội dung.** Chỉ enum và số (`ai_turn_metrics`, một dòng `slog` mỗi lượt).
   - Không chữ, không đối số tool, không nội dung lỗi của provider.
   - Nhãn guard lưu đúng ba giá trị `proceed | restricted | refused`; không nhãn nhạy cảm nào gắn với người.
   - Core không bao giờ cài provider OTel toàn cục.
   - **Luật vận hành, cổng mã không thấy được:** không chạy tác tử eBPF auto-instrumentation cho Go
     (`go.opentelemetry.io/auto` hoặc công cụ dựng trên nó) trên host chạy `core serve` hay `core work`.
     Tracer toàn cục của otel đã import `go.opentelemetry.io/auto/sdk` để tác tử đó gắn vào tiến trình lúc
     chạy và bật lên; khi đó span của ADK (mang nguyên câu hỏi, câu trả lời, kết quả tool) ra khỏi tiến
     trình mà không đổi dòng mã nào và không cần cài provider. Cổng OTel (`aigate/otel_gate_test.go`) chỉ
     giữ phần mã: không import SDK, exporter, `adk/telemetry` hay `go.opentelemetry.io/auto`, không gọi
     `Set…Provider`, và `auto/sdk` chỉ được vào đồ thị qua `otel/internal/global`.
9. **Câu từ chối là câu cố định** trong `aiharness/cau`, không phải chữ model. Mọi mã có câu tiếng Việt,
   và cổng `cau-chu-goi-ai.test.mjs` đọc cả danh sách này.
10. **CI chỉ dùng stub:** `model.LLM` kịch bản và `geministub` loopback. Lời gọi thật chỉ đi qua binary
    eval, và số lời gọi mỗi lần cần Lead duyệt (luật toàn cục về API trả phí, như ADR-0034 §2.6).
11. **Khoá Gemini chỉ ở tiến trình chạy worker**, không bao giờ ghi log. `available` thành kiểm cục bộ,
    không round-trip sang brain.
12. **Chuyển dần sau cờ.** `MOBILE_AI_ENGINE_{NEP,GROUP}` mặc định `brain`, đọc một lần lúc khởi động.
    Action Python `companion-reply`/`nep-reply` giữ làm đường nền eval. Khi Go ≥ Python trên corpus, chúng
    bị xoá cùng mã Go gọi brain và hàng manifest trong **một** commit (mẫu ADR-0036 §3b), rồi bỏ cờ.
13. **Python còn đúng ba việc AI:** eval, extraction lúc nạp dữ liệu (ADR-0040), và skill cũ (đọc hoá đơn,
    ảnh chụp màn hình, vision). Không thêm action Python nào cho bot chat.

## 3. Hệ quả

- Đồ thị phụ thuộc của core lớn lên đáng kể (ADK kéo cloud SDK); `go.sum` phải vào build Docker để
  checksum được kiểm. Nâng Go làm lộ cảnh báo vet mới và đổi mặc định GODEBUG theo dòng `go`; lát nâng
  cấp tách riêng, chưa có ADK, để mọi đỏ trong lát đó chỉ có một nguyên nhân.
- `chatassist` import `aiharness`, nên `roster`, `tenDoc`, `hoiThoai` chuyển sang một gói lá để tránh
  vòng import. Golden `hoi_thoai_golden.json` giữ nguyên đường dẫn vì bộ đo Python đọc nó.
- Bảng `ai_turn_metrics` vào chuỗi migration `chatassist`; số version cấp theo thứ tự lên main, không đặt
  trước. `serve`/`work` đòi version ≥ N. Xoá tài khoản đi theo trigger Go trên `people.deleted_at` của ADR-0041.
- Cổng đọc xuyên gói (lát 5) phải vào main **trước** khi engine chuyển mã; cổng hiện tại chỉ quét file
  của chính `chatassist` và sẽ mù ngay khi đường Nếp đi qua `aiharness`.
- Chữ chạy chậm hơn model khoảng 48 rune. Đó là giá của việc không byte nào tới cả phòng trước khi
  guard thấy nó.
- Latency có mục tiêu hợp đồng: sự kiện trạng thái đầu p95 ≤300 ms; token chữ đầu p50 ≤2.5 s, p95 ≤5 s;
  `plan` p95 ≤8 s. Fast path và truy hồi suy đoán tồn tại để kịp các mốc đó.
- Dự toán lời gọi thật tính từ `MaxModelCallsPerTurn`, không đoán.

## 4. Cái này KHÔNG cho phép

- Không thêm backend nghiệp vụ hay orchestration AI bằng Python. Không thêm action brain cho bot chat.
- Không có vòng lặp agent, router hay registry tool thứ hai ngoài `aiharness`.
- Không đưa chữ tự do do model viết lại vào prompt kế tiếp ngoài khối `<du_lieu>` đã qua guard.
- Không để tool nào ghi sổ tiền, nghĩa vụ, kèo, bình chọn hay tin nhắn. Không nới quyền bằng cách bịa
  tên tool: BeforeTool vẫn từ chối.
- Không gọi model thật trong CI, test hay parity. Không trỏ `BaseURL` tới host không phải loopback khi test.
- Không ghi chữ người dùng, chữ model, đối số tool hay nhãn nhạy cảm theo người vào log, metrics hay span.
- Không gắn tác tử eBPF auto-instrumentation cho Go vào `core serve`/`core work` (§2.8): nó xuất span
  của ADK ra ngoài mà không cần một dòng mã nào của ta.
- Không bật `MOBILE_AI_ENGINE_NEP=go` ở bất kỳ host nào trước khi: eval T1 (stub) xanh trong CI (lát 6b),
  ADR này được ký, review bảo mật việc giữ khoá Gemini trong tiến trình core (thiết kế 01 §9 câu 4)
  xong, **và hai ngưỡng sau qua trên một corpus niêm phong mà tác giả luật chưa từng mở, xét theo cận
  của khoảng tin cậy 95% chứ không theo số điểm: (a) luật tiền tất định (`guard/tien.go`) ĐỨNG MỘT MÌNH
  có cận trên của tỉ lệ bắt nhầm ≤ 0,02; (b) HỢP của luật tiền và bộ phân loại tiền của bước Understand
  (lát 9, lời gọi model thật, số lời gọi do Lead duyệt) có cận dưới của recall ≥ 0,95** (khoảng Wilson
  hai phía), do người khác đo trên đúng SHA sau khi commit (Lead có thể đổi hai con số khi ký).
  Recall không còn đòi ở luật tất định một mình (sửa ngày 2026-09-25 sau hai lần đo niêm phong): v2
  cho 0,873 / bắt nhầm 0,015, v3 cho 0,850 / 0,029 — mỗi vòng thêm mẫu regex để đuổi recall lại sinh
  bắt nhầm mới ở chính câu hỏi quán (review vòng 4: 21/29 câu hỏi quán/kế hoạch của reviewer bị chặn
  so với 4/29 ở bản trước). Luật tất định vì thế đóng băng về recall: chỉ nhận sửa làm GIẢM bắt nhầm
  hoặc sửa hồi quy, không thêm mẫu mới để đuổi recall; recall là việc của lớp phân loại.
  Corpus đó phải có **ít nhất 220 câu mỗi lớp** (tiền / không phải tiền): dưới 189 câu không phải
  tiền thì ngưỡng bắt nhầm không chứng minh được kể cả khi 0 lỗi (0/188 có cận trên 0,02002), còn
  ở đúng 220 câu mỗi lớp, «đạt» nghĩa là 0 câu bắt nhầm (1/220 có cận trên 0,025) và ít nhất 216/220
  câu tiền bị bắt (cận dưới 0,954; 215/220 chỉ còn 0,948). Vì thế số của vòng 3 — recall 131/150 =
  0,873 (KTC 0,811–0,917), bắt nhầm 2/136 = 0,015 (KTC 0,004–0,052) — trượt cả hai ngưỡng, dù số
  điểm bắt nhầm dưới 0,02. Người viết corpus niêm phong không để generator, bản nháp hay bất kỳ tệp
  nào chứa nửa niêm phong với tới được từ DEV hay từ repo; nửa DEV trong repo không ghi tên hay chỗ
  của generator. Số trên corpus tác giả đã đọc (DEV, bộ giữ riêng cũ, câu của review, câu tự viết)
  không tính: chúng chỉ là test hồi quy. Compose không đưa khoá cho `core` trừ khi ghép rõ
  `docker-compose.nep-go.yml`.
- Không đọc luật tiền như hàng rào duy nhất. Nó là lớp 0 của một chồng phòng thủ, chỉnh để **ưu tiên
  độ chính xác**: bắt nhầm một câu hỏi quán/ngân sách là chặn thẳng một câu hỏi hợp lệ với 0 lời gọi
  model và không có cơ hội thứ hai; lọt một câu tiền thì câu đó còn gặp (1) system instruction cấm
  tạo, chia, tất toán, nhắc tiền và cấm tự nhận đã làm gì; (2) engine không có tool nào chuyển, ghi
  hay chia tiền — model chỉ viết được chữ; (3) output guard chặn câu tự nhận đã chuyển/ghi/gửi, mọi
  số điện thoại, số tài khoản, email; (4) từ lát 9, bước Understand là bộ phân loại thứ hai, độc lập
  (`tien: none|split_draft|money_action`, thiết kế 01 §3.3). Không nới lớp 1–3 với lý do «luật tiền
  đã chặn rồi».
- Không hứa «rút lại» một đoạn đã stream. Không nhả byte nào trước khi output guard quét nó.
- Không để engine chọn khoá phòng hay lane; lane do máy chủ suy từ dữ liệu của chính nó.
- Không mở quyền đọc mới cho Nếp (catalogue, kèo của chính mình, trí nhớ) bằng văn bản này. Các quyền
  đó thuộc ADR-0041.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

| Điều khoản | Hiện nói | Sau ADR này |
|---|---|---|
| ADR-0029 §2.7 «Python chỉ còn brain» | Python là dịch vụ brain nội bộ; mọi bước model đi qua đó | Bước sinh chữ của hai bot chat chạy trong Go. Brain còn cho eval, extraction, skill cũ. Ràng buộc của brain (nội bộ, không credential DB, lỗi chỉ trả mã) giữ nguyên cho phần còn lại |
| ADR-0029 §2.1, hàng «AI» | Go giữ route, gọi brain cho bước model | Đúng cho skill cũ; route `chatassist` gọi Gemini từ Go |
| ADR-0029 §1, câu «Mọi route AI cùng một hình: auth + đọc DB → gọi model → grounding thuần → tối đa một lần chèn `ai_card`» | Một lời gọi | Nhiều lời gọi có trần, tool đọc trong Go; vẫn tối đa một lần publish |
| ADR-0029 §3, «toolchain Go 1.23.4» | Go 1.23.4 | Go 1.25 |
| ADR-0036 §2.10 (quyết định 10), câu «phần Python duy nhất được thêm là một action brain cho Nếp» | `nep-reply` là đường thật | `nep-reply` thành đường nền eval, bị xoá ở lát 19; không thêm action Python nào nữa |
| CLAUDE.md, «Python chỉ inference/extraction/evaluation AI» | Python làm inference | Python làm eval, extraction lúc nạp dữ liệu, skill AI cũ; inference của bot chat ở Go. Sửa dòng khi ADR được ký |
| CLAUDE.md, bảng kiến trúc «Go 1.23» | Go 1.23 | Go 1.25 (lát 3) |
| ADR-0031 §1 «Python chỉ AI» | — | Không mâu thuẫn; thu hẹp thêm như hàng CLAUDE.md |

Không đổi bởi văn bản này:
- ADR-0034 §2.6: câu hỏi tuần của sổ đôi vẫn ghi đi brain. Chuyển nó sang Go cần quyết định riêng.
- ADR-0036 §2.3: thuộc ADR-0040. ADR-0033 §2.5, §4, ADR-0036 §2.7 và §4 (vế Nếp): thuộc ADR-0041.
  ADR-0036 §2.1, §2.5, §2.8 và §4 (vế nhóm): thuộc ADR-0039.

## 6. Cổng nghiệm thu của khung

- Lát 3: `go vet`, `go test ./...`, `scripts/go_postgres_tier.sh` (skip là đỏ), `make parity`,
  `scripts/chat_e2e_go.sh`, build Docker, chạy lại trong cây sạch đúng SHA.
- Lát 6 (tách 6a engine, 6b eval T1 trong CI; lát 6 chỉ xong khi cả hai xong): eval T1 bằng stub chạy trong
  CI; request golden ổn định từng byte dưới `-race`, GOMAXPROCS 1 và 8; cổng ranh giới, cổng quan sát,
  cổng OTel, cổng «chỉ stub» đều xanh, canary đỏ đúng chỗ.
- Lát 9: eval T3 lõi ≥14/16, ổn định qua 5 lần, với số lời gọi thật đã được Lead duyệt.
- Mỗi lát: ít nhất hai đột biến tự nghĩ, kiểm tương đương trước, đỏ đúng bước dự đoán; số đo ghi
  thẳng vào commit message.
