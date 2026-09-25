# Thiết kế 01 — Harness agent Go: ADK-Go, Gemini gọi từ Go, một pipeline có hậu kiểm

- Ngày: 2026-09-25.
- Commit gốc: `f251db7`.
- protocol_version: không áp dụng (không phải lượt thí nghiệm).
- Trạng thái: **thiết kế đã được người dùng duyệt, chờ Lead ký ADR**. ADR đi kèm:
  `docs/decisions/proposals/ADR-0037-khung-agent-go-adk-gemini.md`.
- Phạm vi: lõi engine `services/core/internal/aiharness` dùng chung cho «Rủ Đi AI» (nhóm) và Nếp.
  Hàng đợi, streaming, RAG, trí nhớ, UI nhóm và eval có bản thiết kế riêng trong cùng thư mục;
  ở đây chỉ nói chỗ harness chạm vào chúng.
- Thứ tự ưu tiên khi lệch: bảng «Hợp đồng chung» của kế hoạch đã duyệt (chép vào
  `docs/architecture/03-ai-engine-hop-dong.md` ở lát 0) đè lên bản thiết kế gốc.
- Quy ước đánh dấu: «(sửa theo phản biện)» là chỗ khác bản thiết kế gốc do hai bản phản biện đối
  kháng. `[P1-n]` là phát hiện số n của phản biện «ràng buộc, riêng tư, bảo mật, đúng đắn»;
  `[P2-n]` là của phản biện «khả thi, thứ tự, đầy đủ». «(theo hợp đồng chung)» là chỗ đổi để khớp
  bảng hợp đồng.

## 0. Sự thật đã kiểm, là nền của thiết kế

Kiểm tại `f251db7` và trong module cache (`google.golang.org/adk@v1.7.0`, `genai@v1.71.0`,
`golang.org/x/text@v0.40.0`).

| Sự thật | Chỗ |
|---|---|
| Mỗi job nhóm gọi model đúng một lần, qua brain Python | `chatassist/worker.go:124-125` (`companion-reply`, 60 s) |
| Nếp gọi một lần qua brain | `chatassist/nep.go:407-408` (`nep-reply`) |
| `chia_bill` gọi `chat-expense` tuần tự, tối đa 8 lần, trên `gemini-2.5-flash` | `chiabill.go:41`, `chiabill.go:114`, `services/api/app/api/chat_expense_gemini.py:15` |
| `processChiaBill` tự `publish`, tức tự đóng job | `chiabill.go:310`, `chiabill.go:330` |
| Không có `system_instruction`: prompt dán chung với dữ liệu | `companion_gemini.py:149`, `nep_gemini.py:69` |
| Nhiệt độ hiện tại: nhóm 0.0, Nếp 0.4 | `companion_gemini.py:194`, `nep_gemini.py:97` |
| Handler cắt ngữ cảnh 8 s | `chatassist/handler.go:83` |
| `available` hỏi brain mỗi lần, chờ tới 2 s | `handler.go:227-233` |
| Giới hạn 8 lời gọi/phút đếm **lời gọi**, không đếm lời gọi model | `handler.go:414-418`, `nep.go:307-311` |
| Brain client 60 s và đọc hết thân phản hồi | `internal/brain/client.go:28`, `client.go:124` |
| Hai worker poll 250 ms; lease 75 s; job 70 s | `worker.go:33-37`, `worker.go:77`, `worker.go:89` |
| Pool Postgres 15 kết nối mỗi tiến trình | `internal/db/db.go:34` |
| Go 1.23 / toolchain 1.23.4 ở core và parity; Dockerfile chỉ `COPY go.mod ./` | `services/core/go.mod:3,5`, `parity/go.mod:3,5`, `services/core/Dockerfile:4,11` |
| ADK v1.7.0 đòi `go 1.25.0` và khai genai v1.57.0; MVS sẽ chọn v1.71.0 | `adk@v1.7.0/go.mod` |
| x/text v0.40.0 vẫn dựng bảng Unicode 15.0 khi Go < 1.27 | `x/text@v0.40.0/unicode/norm/tables15.0.0.go` (`//go:build !go1.27`) |
| ADK chạy mỗi function call trong một goroutine riêng | `adk@v1.7.0/internal/llminternal/base_flow.go:1130` |
| ADK không có trần bước; `RunConfig` chỉ có `StreamingMode` | `adk@v1.7.0/agent/run_config.go:29` |
| ADK ghi nguyên JSON kết quả tool vào span | `adk@v1.7.0/internal/telemetry/telemetry.go:165` |
| `promptsafety` và `companion` là bản port có oracle Python | `domain/promptsafety/oracle_test.go`, `domain/companion/oracle_test.go` |
| `fold` của promptsafety chưa export | `domain/promptsafety/promptsafety.go:29` |
| `GroupTaste` đọc sở thích và dải ngân sách **theo từng người** | `service/catalogue.go:74`, `catalogue.go:78` |
| `prepare`, `dapThem`, `roster`, `hoiThoai` chưa export | `worker.go:161`, `worker.go:144`, `roster.go:73`, `boicanh.go:153` |
| Cổng đọc Nếp chỉ quét file của chính gói, gốc là ba hàm | `nep_khong_doc_test.go:31`, `nep_khong_doc_test.go:46` |
| Khởi động chỉ kiểm bảng tồn tại, không kiểm version schema | `cmd/core/main.go:168` |
| `GEMINI_API_KEY` hiện chỉ đưa vào service `api` | `docker-compose.yml:136` |

## 1. Gói và ranh giới

Mọi thứ mới nằm dưới `services/core/internal/aiharness/`:

| Đường dẫn | Vai |
|---|---|
| `harness.go` | `Engine.Run(ctx, Turn, Sink) (Result, error)`: điều phối các chặng |
| `preprocess/` | chuẩn hoá, đồng hồ, bảng tham chiếu, bản sao định tuyến |
| `guard/` | `patterns.go`, `input.go`, `policy.go`, `output.go`, `stream.go` (cửa sổ 48 rune) |
| `understand/` | lời gọi Understand, schema enum, tập ý định đóng theo bot |
| `agent/` | dựng `llmagent` + runner mỗi lượt, callback ngân sách |
| `tools/` | registry duy nhất, `quyen.go`, `testdata/quyen.golden.json`, mỗi tool một file, `ledger.go` |
| `ground/` | `GroundReply` cho thẻ `tra_loi`, token địa điểm, provenance |
| `llm/` | `gemini.go` (bọc `gemini.NewModel`), `stub.go`, `dem.go` (bộ đếm và trần), `loi.go` |
| `prompts/` | `*.txt` qua `//go:embed`; `Version()` = 12 ký tự hex đầu của sha256 |
| `cau/` | câu cố định: từ chối, trạng thái, «bị ngắt» |
| `obs/` | `TurnRecord`, chỉ enum và số |

**Gói lá chống vòng import (sửa theo phản biện) [P1-12].** `roster`, `tenDoc`, `hoiThoai`, kiểu gói
bối cảnh và một `ServerFacts` exported chuyển sang gói lá `internal/aiboicanh`. `chatassist` import
`aiboicanh` và `aiharness`; `aiharness` import `aiboicanh`; không có chiều ngược lại. Viết lại các hàm
đó trong engine là tự mở đường lệch luật tên hiển thị của ADR-0036 §5, nên phải di chuyển, không chép.
Golden `chatassist/testdata/hoi_thoai_golden.json` giữ **đúng đường dẫn**, vì bộ đo Python đọc đúng chỗ
đó (`services/api/tests/skills/tra_loi_trong_nhom.py:63`).

**File hiện có thay đổi:**
- `chatassist/worker.go` và `nep.go`: gọi `Engine.Run` khi `MOBILE_AI_ENGINE_{NEP,GROUP}=go`. Cờ đọc
  một lần lúc khởi động và ghi log; mặc định `brain`.
- `chatassist/handler.go` (`available`): thành kiểm cục bộ, không round-trip 2 s sang brain.
  Khoá Gemini chỉ nằm ở tiến trình chạy worker (sửa theo phản biện) [P1-26]. Khi `serve` và
  `core work` tách nhau (lát 4), khả dụng suy từ nhịp tim worker, không từ khoá trong `serve`.
- `domain/promptsafety`: thêm `Fold` (export lại `fold`, không đổi hành vi) và `SafeDeep` (mới, phủ
  cả mô tả, review, hoạt động). Regex `instruction` (`promptsafety.go:22`) **không sửa**: nó là bản
  port có oracle. Mẫu mở rộng sống trong `aiharness/guard/patterns.go`.
- `domain/chatintent`: thêm `Mentions()` exported. `Parse` (`chatintent.go:62`) không đổi, vì là oracle.
- `domain/companion`: thêm `GroundReply` cho thẻ `tra_loi`; `GroundCard` (`companion.go:193`) giữ
  nguyên cho oracle (theo hợp đồng chung).

## 2. Seam và kiểu dữ liệu

```go
type Turn struct {
    Bot          Bot               // nhom | nep
    InvocationID string
    Lenh         Lenh              // plan | chia_bill | hoi
    Luc          time.Time         // chat_ai_invocations.created_at: nguồn DUY NHẤT của «bây giờ»
    LoiNho       string            // lời người gọi
    Goi          *aiboicanh.Goi    // gói client trao; nil = «Chỉ gửi lời nhờ»
    PhieuNep     *PhieuNep         // phiếu ngữ cảnh của màn (Nếp)
    LuotNep      []LuotNep         // các lượt của phiên bảng Nếp đang mở
    Server       aiboicanh.ServerFacts
    GiuLuot      func(context.Context) error // worker tiêm; giữ một lượt trên chat_ai_invocations.model_calls; nil = chỉ đếm RAM (lát 6, 9)
}
type Sink interface {
    TrangThai(ma cau.TrangThai, n int) // -> trang_thai{cau}; n KHÔNG đi trong sự kiện: SSE người gọi lấy n từ gói của mình, frame phòng mang n ở phong bì so_tin (thiết kế 02 §5.3)
    Phan(i int, kind PhanKind, v json.RawMessage) // -> phan{kind,json}; chỉ sau khi phần đó grounded
    Delta(p int, text string)          // -> delta{p,text}; CHỈ guard/stream.go được gọi
    LamLai()                           // -> lam_lai; chỉ trước delta đầu tiên
}
```

- `Engine.Run` là seam chung cho worker và eval (theo hợp đồng chung). Eval cần luồng sự kiện thì
  truyền một `Sink` ghi lại; không có `iter.Seq2` thứ hai (sửa theo phản biện) [P1-16]. Chế độ thành
  phần của eval gọi `Engine.Hieu(ctx, Turn)` (chỉ bước Understand).
- **Engine không biết lane (sửa theo phản biện) [P1-5].** `Turn` không có trường lane. Worker dựng `Sink`
  từ `aistream` với lane đọc từ hàng `chat_ai_invocations` (máy chủ tự suy), nên engine không thể ghi
  nhầm khoá phòng cho job E2EE v2.
- `xong`, `that_bai`, `hello`, `huy`, `thu_hoi`, `ket_noi_lai`, `: ping` **không** do engine phát.
  `xong` chỉ đi sau khi transaction publish của worker commit (lúc đó mới có `message_id`); phần còn
  lại là việc của tầng truyền.
- `Result{Phan, Chips, Nguon, Code, Meta}`. Nhóm: worker dựng `ai_card` kind `tra_loi`
  `{ban, tac_gia:"rudi-ai", invocation_id, lenh, doc{so_tin, chi_loi_nho}, phan[≤3], outing_id?}`,
  author DB = NULL, `reply_to` = tin tag (theo hợp đồng chung; bản gốc dùng `payload.parts`) [P1-7].
  Nếp: `result = {text, chips, nguon}`, kín về đúng người hỏi, như `nepXong` (`nep.go:449`).

## 3. Pipeline mỗi lượt

### 3.1 Preprocess (tất định, mục tiêu ≤20 ms)
- NFC bằng `norm.NFC`; bỏ **mọi** `@Rủ Đi`/`@rudi`/`@ru di` ở bất kỳ vị trí nào (`chatintent.Mentions`).
- Bỏ ký tự zero-width và điều khiển bidi, ghi cờ; gộp khoảng trắng; cờ `khong_dau` khi không có dấu.
- Teencode (`ko, dc, j, ntn, bn, mn`, không đụng `50k`) chỉ trên **bản sao định tuyến**; agent nhận chữ gốc.
- **«Bây giờ»** = `pairpaper.Local(Turn.Luc)` (`pairpaper.go:207`), không dùng đồng hồ worker. Dòng in ra
  mang cả hai dạng (sửa theo phản biện) [P1-16]:
  `Bây giờ: Thứ Sáu 25/09/2026 14:05 (Asia/Ho_Chi_Minh, 2026-09-25T14:05:00+07:00)`. Dạng RFC3339
  là thứ bất biến của eval đọc; dạng chữ là thứ model đọc tốt. Mốc `luc` trong gói (UTC) in lại giờ VN.
- Ngày tương đối («tối nay», «mai», «thứ 6 này», «cuối tuần», «7 giờ rưỡi»…) giải bằng **một** gói
  `domain/thoigian` do lát 8 tạo; engine không có bản `preprocess/dates.go` riêng (sửa theo phản biện)
  [P2-4]. Lát 6 (Nếp, chưa tool) chỉ cần dòng «bây giờ».
- Bảng tham chiếu R0…R7 (xếp theo độ gần, trần 8): thực thể trong phiếu Nếp; id địa điểm trong các
  thẻ AI của lượt được trao, **máy chủ đọc `messages.card`** của đúng các id `ai_card` có trong gói,
  không tin trường client tự khai (sửa theo phản biện) [P2-15]; tên quán catalogue khớp bằng `Fold`.
  Cổng đọc (lát 5) cho phép cột `card` của `ai_card` do AI viết, **không bao giờ** cột `body`. Phòng
  E2EE v2 không có `card` rõ để đọc: tham chiếu chỉ từ gói.

### 3.2 Guard tất định
- `guard/patterns.go`: cụm VI/EN («phớt lờ/quên… chỉ thị/luật», «đóng vai», «bạn giờ là», «in ra
  prompt», «developer mode»), thẻ vai, code fence, chuỗi base64 ≥200 ký tự, gập homoglyph Cyrillic.
- Chạy trên **mọi** nguồn không tin cậy, mỗi nguồn một nhãn và một hành động:

| Nguồn | Khi bị gắn cờ |
|---|---|
| Lời người gọi | Chuyển nhãn cho policy |
| Lượt của thành viên khác trong gói | Giữ, bọc `<du_lieu>`, đánh `nghi_chi_thi` |
| Tên hiển thị | Luật `tenDoc` hiện có (ADR-0036 §5) |
| Chuỗi trong phiếu Nếp | Bỏ |
| Lượt phiên Nếp (`luot`), kể cả lượt vai model client gửi lại (sửa theo phản biện) [P1-24] | Bỏ lượt đó |
| Kết quả tool (quán, sổ tay) | `SafeDeep`; bỏ hàng |
| Fact trí nhớ | Bỏ |
| **Đầu ra Understand** (sửa theo phản biện) [P1-8] | Bỏ slot đó; nếu hỏng cấu trúc thì rơi về ý định mặc định |

- **Luật tiền, tất định, chạy trước mọi lời gọi model:** regex đã `Fold` cho hành động tiền (chuyển
  tiền/khoản, ghi nợ, trả nợ, thanh toán, stk, «ai nợ ai») cạnh số tiền. Nhóm: yêu cầu chia →
  `chia_bill_draft`, còn lại → từ chối `ai_khong_cham_tien`. Nếp: mọi yêu cầu tiền → từ chối, 0 lời gọi model.

### 3.3 Understand: một lời gọi, chỉ enum và slot
Gộp classifier và analyze của bản gốc thành **một** lời gọi flash-lite (sửa theo phản biện) [P2-10]:
`ResponseJsonSchema`, nhiệt độ 0, `ThinkingLevel=MINIMAL`, `MaxOutputTokens` 256, hạn 2.5 s.

```json
{"nhan":{"tan_cong":"none|direct|indirect","pham_vi":"in|out|borderline",
         "tien":"none|split_draft|money_action",
         "lam_dung":"none|harassment|self_harm|sexual|hate"},
 "y_dinh":[{"loai":"<enum đóng theo bot>",
   "khe":{"khu_vuc":"<id trong areas.All()>","ngay":"YYYY-MM-DD","gio_tu":"HH:MM","gio_den":"HH:MM",
          "ngan_sach_moi_nguoi_vnd":0,"so_nguoi":0,"di_ung":["<enum>"],"an_kieng":["<enum>"],
          "loai_cho":["<enum>"],"tham_chieu":["R0"],"chu_the":"toi|nguoi_khac|khong_ro"}}],
 "can_hoi_lai":false,"thieu":"none|khu_vuc|ngay|gio|ngan_sach|so_nguoi"}
```

- **Bỏ `cau_doc_lap`, `cau_con`, `cau_hoi_lai` tự do của bản gốc (sửa theo phản biện) [P1-8].** Không
  trường nào chở được chữ tự do đi tiếp. Câu hỏi lại do bước trả lời viết, hoặc câu cố định theo `thieu`.
  `khu_vuc` là enum id vùng dựng từ `areas.All()` (`areas.go:32`); tập `di_ung`/`an_kieng`/`loai_cho`
  lấy từ từ vựng catalogue, lát 8 chốt.
  `chu_the` chỉ có nghĩa với ý định `remember` của Nếp (thiết kế 05 §6, `kiemSuThat` bước 3); nhóm bỏ qua.
- Kiểm bằng Go: 1–3 ý định, ý định lạ bị bỏ, hết thì rơi về `chat_answer` (nhóm) hoặc `smalltalk`
  (Nếp); ngày phải là ứng viên của `thoigian` hoặc ISO hợp lệ; tham chiếu phải có trong bảng; tiền là
  số nguyên đồng.
- Sau khi kiểm, JSON đưa vào prompt **bên trong** `<du_lieu nguon="hieu">…</du_lieu>` và qua
  `guard/patterns.go` thêm một lần (theo hợp đồng chung). Có ca tấn công «injection được Understand
  kể lại».
- **Tập ý định đóng.** Nhóm: `chat_answer, plan, find_places, chia_bill_draft, app_help` (+`poll_draft`,
  xem mục 9 câu 1). Nếp: `app_help, find_places, explain_screen, plan_help, remember, forget,
  what_you_remember, smalltalk` (+`remind` từ lát 17, bật cùng hàng `set_reminder` trong
  `quyen.golden.json`). `lenh=plan` ép có ý định `plan`; `lenh=chia_bill` ép `chia_bill_draft`.
  `what_you_remember` do Go liệt kê hết, không qua model (ADR-0041).
- **Truy hồi suy đoán song song (sửa theo phản biện) [P2-9].** Từ kết quả preprocess (vùng khớp `Fold`,
  ngày, tham chiếu), Go khởi động `rag.Retrieve` ngay, song song với Understand. Slot trả về lệch ở bộ lọc
  cứng thì bỏ kết quả đó và chạy lại.
- **Policy** `(bot, nhãn, cờ tất định) → proceed | proceed_restricted | refuse(code) | template(code)`.
  `proceed_restricted` = chỉ tool đọc, không nháp, không ghi trí nhớ. Understand hết hạn hoặc lỗi →
  `proceed_restricted` + ý định mặc định; guard tất định vẫn chạy.
- **Tự hại, không lưu nhãn theo người (sửa theo phản biện) [P1-3].** Nếp: trả câu hỗ trợ cố định, kín về
  người hỏi. Nhóm: phòng và hàng DB chỉ thấy mã chung `ai_tu_choi`, không bao giờ thấy loại. Câu hỗ trợ
  đi riêng tới người gọi qua khoá stream của người gọi (TTL ≤ cửa sổ chia sẻ), không ghi Postgres. Nhãn
  `lam_dung` chỉ sống trong RAM của lượt.

### 3.4a Fast path
Một ý định duy nhất thuộc `find_places | app_help | explain_screen`, và `can_hoi_lai=false`: Go gọi
retriever thẳng từ slot (hoặc dùng kết quả suy đoán đã khớp), rồi **một** lời gọi trả lời có bằng chứng,
`Mode NONE`, không tool. Tổng 2–3 lời gọi, kể cả chấm lại trong `search_places` (sửa theo phản biện) [P2-9].

### 3.4b Agent loop (ADK-Go)
- Mỗi bot một `llmagent`; mỗi lượt một `runner.New` với `session.InMemoryService()`, bỏ khi lượt xong.
  Không dùng `memory.Service` của ADK: nó nạp cả phiên, trái luật trí nhớ.
- **Sửa câu «không gì tồn tại» của bản gốc (sửa theo phản biện) [P1-9].** Phiên ADK không tồn tại, nhưng
  trí nhớ Nếp có tồn tại, chỉ qua hàm của `nepnho` (writer duy nhất) dưới ADR-0041, ADR sửa rõ
  ADR-0036 §2.7 và §4. Engine không tự ghi bảng nào.
- Lượt phiên Nếp nối trước thành sự kiện user/model; gói nhóm là một khối dữ liệu, vì chat nhiều người
  không ánh xạ vào hai vai.
- Toolset mỗi lượt: `tool.FilterToolset(registry, duocPhep(bot, yDinh, policy))` (`tool/tool.go:89`).
- Cấu hình sinh: nhóm nhiệt độ 0.0, `MaxOutputTokens` 1024; Nếp 0.4, 768; `ThinkingLevel` LOW (eval
  chốt); `SafetySettings` BLOCK_MEDIUM_AND_ABOVE cho bốn loại.
- **Callback:**
  - BeforeModel: đếm bước; bước cuối (4 nhóm, 3 Nếp), hoặc khi chỉ còn một lời gọi trong trần, hoặc khi
    tổng prompt token vượt 60k, đặt `FunctionCallingConfig{Mode: NONE}` và nối câu «trả lời ngay bằng dữ
    liệu đã có».
  - BeforeTool: kiểm quyền lại (phòng thủ hai lớp), trần 10 tool call, semaphore 5 mỗi lượt **và**
    semaphore DB toàn tiến trình ≤ pool worker/2 (sửa theo phản biện) [P1-17][P2-11], đặt hạn từng tool.
  - AfterTool: ghi kết quả vào **ledger** của lượt (có mutex, vì ADK chạy tool song song), lọc `SafeDeep`.
  - OnToolError: trả `{"loi":"<mã>"}` để model tự sửa.
  - AfterModel: đưa chữ partial vào `guard/stream.go`, cộng usage.
- Tool đọc mở `BeginTx(ReadOnly)` trên pool riêng của worker (lát 4): DB tự từ chối ghi, SQLSTATE 25006.
  Tool nháp không chạm DB: kiểm với ledger rồi thêm phần vào trạng thái lượt.
- **`chia_bill_draft` không phải tool agent, và không publish song song (sửa theo phản biện)
  [P1-11][P2-12].** Tách `chiaBillParts()` thuần từ `processChiaBill`; đọc các tin bằng **một** lời gọi
  flash-lite gộp trong Go qua `aiharness/llm` (tính vào trần), thay 8 lời gọi tuần tự trên
  `gemini-2.5-flash` [P1-25][P2-10]. Worker publish **một** lần ở cuối. Nháp nằm trong `result`, phần
  `expense_draft` của thẻ chỉ mang con trỏ tới nháp, không mang số tiền hay id người trả; dấu «Đã ghi
  vào sổ» suy từ dòng chi tiêu thật (lát 14) [P1-10].

### 3.5 Hậu kiểm
- **Token địa điểm.** Văn xuôi chỉ nêu quán bằng `[[p:ID]]`; bộ lọc giữ `[[` mở cho tới `]]` rồi thay
  bằng tên trong ledger. Id lạ → «một chỗ», đếm. Token không đóng trong 48 rune → bỏ, đếm `token_hong`.
  Tên catalogue xuất hiện không qua token → `ungrounded_mention` (số đo, không chặn).
- Giá và giờ mở cửa chỉ nằm trong thẻ. Thẻ `text/places/itinerary` qua `GroundReply` trên **đúng tập
  quán tool đã trả trong lượt**, không phải 40 dòng cố định (theo hợp đồng chung).
- Provenance trên mỗi phần: `{nguon:[{loai:"catalogue"|"lich-su-nhom"|"so-tay-app"|"tri-nho", id?}],
  doc_tin:N, cong_cu:[tên]}`, trong đó `id` là id mục sổ tay hoặc `f<id>` của fact (thiết kế 05 §3,
  §6) (ADR-0036 §2.8).
- **Output guard, cửa sổ 48 rune, là nơi duy nhất sinh `delta` (theo hợp đồng chung; sửa theo phản
  biện) [P1-4].** Thay «giữ theo câu» của bản gốc.
  - Bất biến: rune thứ i chỉ được nhả khi đã thấy 48 rune sau nó (hoặc đã hết luồng) và guard đã quét
    cả bộ đệm. Mọi mẫu guard có độ dài tối đa ≤48 rune, nên một vi phạm vắt qua ranh giới chunk luôn
    nằm trọn trong bộ đệm trước khi rune đầu của nó được nhả.
  - Mẫu: nonce canary trong system instruction (≤24 rune); SĐT, email, dạng số tài khoản; tự nhận hành
    động chưa làm («đã chuyển», «đã ghi nợ», «chốt kèo», «tạo kèo»); từ đặc điểm nhạy cảm (ADR-0034
    §2.3); luật `CheckAnswer` của RAG chạy được cục bộ: giá/giờ đứng gần `[[p:` phải khớp ledger.
  - Vi phạm: ngừng luồng, **không rút lại** phần đã nhả, 0 byte của cửa sổ vi phạm tới Redis. Phần chữ
    cuối = phần đã nhả + câu cố định `ai_tra_loi_bi_chan`; thẻ đã grounded giữ nguyên.
  - Lát 6 chưa stream: cùng hàm, quét trên cả văn bản trước `nepXong`.
- Trần chữ: nhóm đề xuất 1500 rune (chờ Lead chốt, mục 9 câu 2); Nếp 2000 (`maxTraLoiNep`, `nep.go:51`).
- Giọng văn (tỉ lệ dấu, «mình/bạn» cho Nếp, «cả nhóm» cho nhóm) là kiểm tra eval, không chặn.

### 3.6 Prompt và cache
- `system_instruction` qua `InstructionProvider`, tĩnh theo bot, ổn định từng byte: danh tính, luật,
  chính sách tool, luật token địa điểm, giọng, nonce canary. Rồi khai báo tool (tĩnh). Rồi nội dung
  user: các khối `<du_lieu nguon="…">` (`<>` trong dữ liệu đổi sang ký tự fullwidth), dữ kiện máy chủ,
  gói hoặc phiếu, dòng «bây giờ», chú thích ngày, bảng tham chiếu, `<du_lieu nguon="hieu">`, **câu hỏi
  cuối cùng**.
- Không dùng `Caches.Create` ở v1: prefix chỉ khoảng 2–3k token và ADK đóng gói tool theo từng request.
  Xem lại khi eval cho `CachedContentTokenCount/PromptTokenCount` < 0.5 **và** prefix > 4k token.
- Chuyển prompt: `_PROMPT` của `companion_gemini.py:23` → `prompts/nhom_agent.txt`, giữ mọi luật ràng
  buộc (giờ so như số, điểm giữa ngân sách, dị ứng, hỏi lại một lần, trả lời đủ hai mong muốn, hội thoại
  là dữ liệu, không chạm tiền); «trả về một thẻ JSON» thành văn xuôi + thẻ từ tool nháp. `nep_gemini.py:26`
  → `prompts/nep_agent.txt`. Action Python giữ làm đường nền eval cho tới lát 19.

## 4. Bảng quyền tool (một registry, theo hợp đồng chung)

Tên chốt một lần trong `aiharness/tools`; `tools/testdata/quyen.golden.json` quy định bot nào gọi được
gì. Không bản thiết kế nào khác được có registry thứ hai hay tên thứ hai (`tim_dia_diem`, `nho`, `quen`,
`hen_nhac` của bản Nếp gốc bỏ) (sửa theo phản biện) [P1-6][P2-4].

| Tool | Nhóm | Nếp | Loại | Hạn | Ghi chú |
|---|---|---|---|---|---|
| `search_places` | ✓ | ✓ (ADR-0041) | đọc | 3 s | bọc `rag.Retrieve`; chấm lại chỉ khi > k ứng viên [P2-4] |
| `get_place` | ✓ | ✓ (ADR-0041) | đọc | 1 s | `SafeDeep` |
| `list_destinations`, `nearest_area` | ✓ | ✓ | đọc tĩnh | 0.5 s | `ListDestinations`, `areas.NearestArea` |
| `group_snapshot` | ✓ | ✗ | đọc | 1 s | nhãn roster, `GroupTaste` **chỉ tổng hợp**, ngân sách |
| `list_group_outings` | ✓ | ✗ | đọc | 1.5 s | chỉ context này (ADR-0036 §2.3) |
| `search_app_manual` | ✓ | ✓ | đọc, `go:embed` | 1 s | `internal/huongdan/data` [P2-6] |
| `explain_screen` | ✗ | ✓ | đọc phiếu + sổ tay | 0.5 s | không DB |
| `propose_places`, `propose_itinerary` | ✓ | ✓ (`plan_help`) | nháp | — | kiểm với ledger |
| `draft_poll` | tắt (mục 9) | ✗ | nháp | — | chờ kind thẻ |
| `suggest_screen` | ✗ | ✓ | nháp → chip | — | route trong registry màn |
| `my_upcoming_outings` | ✗ | ✓ khi chính người đó hỏi | đọc | 1.5 s | ADR-0041 |
| `recall_memory` | ✗ | ✓ khi công tắc bật | đọc | 1 s | chỉ từ gốc scope=me [P1-1] |
| `remember_fact` | ✗ | ✓ khi bật **và** ý định `remember` | tự nhớ | 1 s | qua `nepnho`; fact phải nằm trong lời người dùng; từ chối fact về người khác [P1-9] |
| `forget_fact` | ✗ | ✓ | tự nhớ | 1 s | **xoá cứng** + tombstone băm [P1-2] |
| `set_reminder` | ✗ | ✓ khi `nhac` bật **và** ý định `remind` (lát 17) | tự nhớ | 1 s | xếp vào lớp ghi-của-chính-mình [P2-4] |

- **Cấm mọi tool:** `messages.body`; bảng sổ tiền, khoản chi, bill, thanh toán; album; hồ sơ sở thích;
  bài đăng; `saved_places`; sở thích theo người. Ngoại lệ có tên duy nhất: `GroupTaste` qua
  `group_snapshot`, chỉ tổng hợp (ADR-0036 §2.3) (sửa theo phản biện) [P1-13]. Không tool nào ghi kèo,
  bình chọn, tin nhắn, tiền hay nghĩa vụ.
- Cột Nếp có ghi «ADR-0041» bật trong golden ở đúng lát thi công hành vi đó (13, 15, 17), không sớm hơn.
- `list_open_votes` của bản gốc không có trong registry hợp đồng: muốn thêm phải sửa hợp đồng và ADR.

## 5. Ngân sách và mục tiêu

| Hạng mục | Trần | Khi chạm |
|---|---|---|
| Lời gọi model mỗi lượt | `llm.MaxModelCallsPerTurn = 8`, **đếm cả retry**, mục tiêu p95 ≤4 (theo hợp đồng chung) [P1-6][P2-10] | giữ 1 lời gọi cho bước trả lời; hết hẳn → `ai_het_ngan_sach` |
| Bước agent | 4 (Nếp 3) | bước cuối `Mode NONE` |
| Tool call | 10 mỗi lượt, 5 song song, DB toàn tiến trình ≤ pool/2 | `{"loi":"het_luot_cong_cu"}` / `{"loi":"ban"}` |
| Understand | 2.5 s | `proceed_restricted` + ý định mặc định |
| `search_places` | 3 s kể cả chấm lại | dùng thứ tự RRF hoặc từ chối nêu ràng buộc |
| Retry model | 2 lần trên 429/503, 300 ms → 1.2 s, **chỉ trước delta đầu** | sau đó: giữ phần đã nhả + câu «bị ngắt», mã `provider_*` |
| Prompt token cộng dồn | 60k | ép trả lời |
| Hạn lượt | 40 s, trong khung 70 s của `ProcessOne` | trả thứ đang có + `ai_het_ngan_sach` |

- **Retry do `llm` tự làm (sửa theo phản biện) [P1-6].** Tắt `HTTPRetryOptions` của genai; mỗi lần thử
  đi qua bộ đếm (và bộ giới hạn tốc độ theo lời gọi của lát 10), nên eval `--du-toan` đọc được trần
  thật. Từ lát 10, trước mỗi lời gọi, `llm/dem.go` gọi `Turn.GiuLuot`. Hàm này do worker tiêm và chạy
  `UPDATE … SET model_calls=model_calls+1 WHERE id AND lease_id AND model_calls<$tran` (thiết kế 02 §6),
  nên trần đúng qua mọi lần thử, kể cả khi worker chết. `aiharness` vẫn không có SQL.
- Phân bổ trần 8: Understand 1 · agent ≤4 · chấm lại trong `search_places` ≤2 (viết lại truy vấn là tất
  định, 0 lời gọi, thiết kế 04 §5.5) · `chia_bill` 1.
  `rag` nhận client `llm` qua ctx, không có client riêng.
- **Latency (theo hợp đồng chung; sửa theo phản biện) [P1-16][P2-9]:** sự kiện trạng thái đầu p95
  ≤300 ms; token chữ đầu p50 ≤2.5 s, p95 ≤5 s; `plan` trọn lượt p95 ≤8 s. Bản gốc (TTFT p50 ≤3 s) bị
  thay. Engine phát `TrangThai(dang_doc, n)` hoặc `dang_hieu` **trước mọi I/O**; fast path và truy hồi
  suy đoán là hai cơ chế để kịp mốc token đầu. Cửa sổ 48 rune cộng thêm thời gian sinh 48 rune trước
  delta đầu; eval T4 đo trên `geministub` có độ trễ cấu hình.

## 6. Quan sát không nội dung

- `obs.TurnRecord` → một dòng `slog` JSON mỗi lượt + bảng `ai_turn_metrics`:
  `(invocation_id uuid REFERENCES chat_ai_invocations ON DELETE CASCADE, lan_thu int, bot, created_at,
  y_dinh text[], guard, buoc, tool_calls jsonb /*tên→số*/, tool_loi, so_goi_model, tokens_in/out/cached/
  thoughts, ms_trang_thai_dau, ms_chu_dau, ms_tong, stage_ms jsonb, ground_vi_pham, out_guard,
  prompt_version char(12), ket_thuc, code)`, giữ 30 ngày.
- **`guard ∈ {proceed, restricted, refused}` và `out_guard ∈ {none, chan}`, không loại con (sửa theo phản
  biện) [P1-3].** Không cột nào nối một nhãn nhạy cảm với một người. Loại con của output guard chỉ có
  trong lần chạy eval.
- Mọi trường chuỗi là enum có `Valid()`; test phản chiếu đỏ khi có trường chuỗi tự do. Không ghi đối
  số tool, không ghi chữ. Lỗi genai qua `errors.As(*genai.APIError)` → `provider_{timeout,429,5xx,
  safety,bad_response}`, chỉ giữ mã.
- **Migration có chủ (sửa theo phản biện) [P2-1].** `ai_turn_metrics` vào chuỗi migration `chatassist`,
  số version **cấp theo thứ tự lên main**, không đặt trước (theo hợp đồng chung). Writer duy nhất là
  worker `chatassist`, trong transaction kết thúc; `aiharness` không có SQL. `serve`/`work` đòi version
  ≥ N thay cho kiểm `to_regclass` (`main.go:168`).
- Xoá 30 ngày: tới lát 10 chạy trong sweep có advisory lock, 10 phút một lần; lát 10 chuyển vào registry
  tác vụ định kỳ [P2-14]. Xoá tài khoản: cascade qua `invocation_id`; test Postgres của lát 15 liệt kê
  cả bảng nối gián tiếp này [P1-14][P2-8].
- OTel: core không bao giờ cài provider toàn cục. Cổng đỏ khi mã không-test import
  `go.opentelemetry.io/otel/sdk` hoặc gọi `otel.Set*Provider`, vì `TraceToolResult` ghi nguyên kết quả tool.

## 7. Cổng, test, canary, đột biến

**Cổng cấu trúc**
- `aiharness/ranh_gioi_test.go`: không literal SQL, không import `pgx`/`repo` ngoài `tools/`.
- Cổng đọc xuyên gói là **một** cổng của lát 5 (`go/packages` có type, allowlist bảng theo từng gốc,
  canary method value, ngoại lệ `GroupTaste`), thay đề xuất `tools/quyen_doc_test.go` riêng của bản gốc.
  Harness không được chuyển mã trước khi cổng đó vào main (sửa theo phản biện) [P1-13][P2-5].
  `khong_doc_chat` mở rộng ra `internal/**`.
- `quyen.golden.json`: bảng bot × tool; test so `FilterToolset` từng bot với golden; tool không được
  phép bị BeforeTool từ chối kể cả khi model tự bịa tên.
- `apps/mobile/tests/cau-chu-goi-ai.test.mjs` đọc thêm danh sách mã trong `aiharness/cau/cau.go`.
- Cổng phản chiếu `obs`, cổng import OTel, cổng «CI chỉ stub»: `MOBILE_GEMINI_BASE_URL` chỉ nhận
  loopback; dưới `go test` không dựng được client Gemini tới host thật.

**Ổn định byte**
- `llm/stub.go` hiện thực `model.LLM`, kịch bản `testdata/kich_ban/*.json` theo chặng (Understand, bước
  agent N): function call, chunk partial + tổng hợp, `APIError`.
- Mọi `LLMRequest` ghi JSON chuẩn (khoá sắp, system instruction, khai báo tool, config) vào
  `testdata/yeu_cau/*.golden.json`, cập nhật bằng `-update`: đổi prompt là thấy diff golden.
- Đồng hồ qua `platform.WithTimeProvider`, id qua `platform.WithUUIDProvider`; ledger có thứ tự. Mỗi kịch
  bản chạy hai lần dưới `-race`, GOMAXPROCS 1 và 8, phải ra cùng byte.
- `e2e/geministub` (tag `e2e`) nói REST `:generateContent` và `:streamGenerateContent?alt=sse` qua
  `HTTPOptions.BaseURL`: đường truyền genai thật được chạy. **Một** stub, **một** binary eval
  `cmd/rudi-eval` trên `Engine.Run`: sinh ở lát 6 với chế độ kịch bản, đủ ba chế độ ở lát 18 (thiết kế
  06 §13); harness không có binary riêng (sửa theo phản biện) [P1-16][P2-17].

**Test đơn vị và kịch bản** (đồng hồ cố định 14:05 ICT ngày 25/09/2026): «thứ 6 này» vào thứ Sáu, «tối nay»
lúc 23:30, «mai» lúc 00:30; bỏ mention giữa câu; 60 chuỗi tấn công + 60 câu vô hại giống ngoài («bỏ qua
quán đó đi»); golden policy; `FuzzHieuParse`; thay token địa điểm; cửa sổ 48 rune với vi phạm vắt qua
ranh giới chunk; trần ép `Mode NONE`; tool không được phép; kịch bản stub: plan + nhiều phần, lượt gói
có injection, yêu cầu tiền (nhóm → chia_bill, Nếp → từ chối, 0 lời gọi), 429 rồi thành công, đứt giữa
câu, id bịa → tool lỗi → kịch bản tự sửa → thẻ grounded. Tầng Postgres: tool trên schema thật, ghi trong
tool đọc → 25006, outing khác context → rỗng, `SafeDeep` bỏ hàng.

**Canary** (đỏ đúng chỗ dự đoán, ca identity xanh)
1. Id quán ngoài ledger cắm vào thẻ → `GroundReply` đỏ; id trong ledger → xanh.
2. Nonce bí mật trong system instruction không bao giờ xuất hiện trong log, `slog`, metrics hay sink.
3. Câu vi phạm guard để lại **0 byte** trong sink ghi lại (và trong Redis ở lát 11).
4. Stub Understand trả `khu_vuc` chứa chuỗi injection → Go bỏ slot, golden request không chứa chuỗi đó.
5. Fact trí nhớ đã seed không bao giờ có trong request nào của nhóm (bộ ghi request do harness cấp, lát 15 dùng).

**Đột biến tự nghĩ** (kiểm tương đương trước; mỗi cái đỏ đúng bước dự đoán)
- M1 bỏ `Mode NONE` bước cuối → test ngân sách đỏ ở bước 4 (stub gọi tool mãi).
- M2 `search_places` mở tx không ReadOnly → ca Postgres 25006 đỏ.
- M3 bỏ guard trên đầu ra Understand → canary 4 đỏ.
- M4 dùng UTC thay `pairpaper.Local` → ca «tối nay» lúc 23:30 đỏ.
- M5 bộ đếm không đếm retry → kịch bản «429 rồi thành công» đỏ ở `so_goi_model` (dự đoán 3, ra 2).
- M6 nhả rune trước khi đủ 48 rune phía sau → canary 3 với SĐT vắt qua chunk đỏ.

## 8. Lát cắt (số theo kế hoạch đã duyệt)

| Lát | Phần harness | Cổng riêng |
|---|---|---|
| 0 | Tài liệu này + đề xuất ADR-0037 | — |
| 3 | Go 1.23.4 → 1.25: `go 1.25.0` và toolchain 1.25 patch mới nhất ở core và parity; `ARG GO_IMAGE` ghim digest; `COPY go.mod go.sum ./`; sửa cảnh báo vet mới (định dạng printf không hằng từ 1.24, analyzer `waitgroup`/`hostport` của 1.25); soát GODEBUG đổi theo dòng `go`; test ghim `norm.Version=="15.0.0"`; dòng «Go 1.23» của CLAUDE.md. **Chưa thêm ADK.** | `go vet`, `go test ./...`, `scripts/go_postgres_tier.sh`, `make parity`, `scripts/chat_e2e_go.sh`, build Docker, chạy lại cây sạch |
| 4 | (hạ tầng) worker tách, pool riêng; harness dựa vào đó cho semaphore DB | — |
| 5 | (cổng) cổng đọc xuyên gói — **điều kiện trước** lát 6 | — |
| 6 | S1: ADK + genai vào `go.mod`; `llm` (gemini, stub, bộ đếm, trần), preprocess (NFC, mention, «bây giờ»), guard tất định kể cả `luot` Nếp, output guard chạy trên cả văn bản, `nep_agent.txt` có `system_instruction`, agent không tool, `obs` + `ai_turn_metrics`, `aiboicanh`, cờ `MOBILE_AI_ENGINE_NEP=go`, `cmd/rudi-eval --mo-hinh kich-ban` | eval T1 (stub) trong CI |
| 9 | S2: Understand, fast path, agent loop, registry + quyền, `search_places` = `rag.Retrieve` (3 s), lệnh `hoi` nhiều phần, `chiaBillParts()` + một lời gọi gộp, `GroundReply`, cờ `MOBILE_AI_ENGINE_GROUP=go` | eval T3 lõi ≥14/16 ổn định qua 5 lần |
| 10 | giới hạn tốc độ theo lời gọi trong `llm/gemini.go`; retry cấp job + `LamLai` | tầng broker |
| 11 | `StreamingModeSSE`; `guard/stream.go` là nơi duy nhất sinh delta; `Sink` nối `aistream` | canary 3 trên Redis |
| 13 | `explain_screen`, `search_app_manual`, `suggest_screen`, `my_upcoming_outings`; hash bản build trong phiếu v2 | — |
| 15 | `recall_memory`, `remember_fact`, `forget_fact` qua `nepnho` | canary 5 |
| 17 | `set_reminder` | — |
| 18 | binary eval (có từ lát 6) thêm chế độ cassette/thật; `geministub` | — |
| 19 | xoá action `companion-reply`/`nep-reply` + mã Go gọi brain + hàng manifest trong **một** commit; bỏ cờ | `check_route_ownership.py` |

## 9. Chưa chốt

1. **`draft_poll` lệch hợp đồng thẻ.** Registry có `draft_poll`, nhưng `phan` của `tra_loi` chỉ nhận
   `text/places/itinerary/expense_draft`. Tới khi ADR-0039 thêm kind (hoặc bỏ tool), golden quyền **tắt**
   `draft_poll` cho nhóm và ý định `poll_draft` rơi về `chat_answer`.
2. Trần chữ nhóm: đề xuất 1500 rune; Lead chốt.
3. Câu chữ và đường dây nóng của câu hỗ trợ tự hại; cách giao riêng cho người gọi trong nhóm.
4. Khoá Gemini nằm trong tiến trình worker của core: cần review bảo mật.
5. `ThinkingLevel` từng chặng: eval quyết.
6. Có đếm gộp `lam_dung` hay không. Mặc định **không lưu gì**; nếu Lead cần, chỉ gộp theo tuần toàn hệ, không id.
7. Giá trị `MaxModelCallsPerTurn = 8`: xác nhận với dự toán eval ở lát 9.
8. (Đã đóng: bộ đếm nằm trên hàng job, thiết kế 02 §6.)
