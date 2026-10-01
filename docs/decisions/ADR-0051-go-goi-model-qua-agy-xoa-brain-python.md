# ADR-0051 — Mọi lời gọi LLM của Python chuyển sang Go qua agy-proxy; bỏ cờ engine; xoá brain Python

- Ngày: 2026-09-30.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-09-30** trong phiên lập kế hoạch (câu trả lời có ghi lại ở
  kế hoạch đã duyệt; thêm 2026-10-01: «migrate qua Go, test thử xong thì xoá stale Python, không cần giữ»).
  Không phải chữ ký Lead: ghi rõ để không ai đọc thành Lead đã ký.
- Thay một số điều khoản của ADR-0029, ADR-0036, ADR-0042, ADR-0044, ADR-0047, ADR-0049, liệt kê ở mục 5;
  **không sửa bản lịch sử** của ADR nào (mỗi ADR đó chỉ thêm một dòng «Sửa bởi»).
- Không đổi bởi văn bản này: ba luật tiền; chat v2 E2EE; AI chỉ nhận nội dung được gọi/chia sẻ rõ; ADR-0034
  và ADR-0048 (gu, đồng ý); câu từ chối cố định; không tool nào ghi sổ tiền; ADR-0049 §4 (không provider
  thật trong CI/test/parity, không mở cho người dùng thật khi chưa có hồ sơ chuyển dữ liệu xuyên biên giới).

## 1. Bối cảnh

Sau ADR-0049, Go đã gọi được agy-proxy (`llm.GeminiFromEnv`), nhưng bước model của 13 tính năng vẫn đi vòng
qua brain Python (`services/api`, `/internal/brain/v1/*`, mỗi module tự dựng client Gemini bằng
`GEMINI_API_KEY`), và Nếp cùng bot nhóm còn nằm sau hai cờ `MOBILE_AI_ENGINE_NEP/GROUP` mặc định `brain`.
Hai cộng đồng (`community-moderate`, `community-nep`) đi qua Python chỉ để chuyển tiếp sang một
`COMMUNITY_INFERENCE_URL` không có trong repo.

Lát 1 của đợt này (commit `db60723d`) đo thật qua agy: `gemini-3.5-flash-lite` đọc đúng bill giả (3/3 dòng,
tổng chép đúng), nhận ra thực đơn, đọc ba ảnh trong một request, nhận body 15,7 MB. Không cần đổi model.

## 2. Quyết định

1. **Mọi bước model đang ở Python chuyển sang Go, gọi qua agy-proxy**, cùng cửa với engine chat
   (`AGY_PROXY_URL`/`AGY_PROXY_KEY`; không có thì `GEMINI_API_KEY` trực tiếp). Gồm: đọc bill, đọc ảnh chuyển
   khoản, đọc khoản chi trong tin nhắn, gợi ý và gợi ý theo ngữ cảnh, reel, gợi ý thành tựu, nhật ký, tìm quán
   và lý do gợi ý quán, duyệt bài và Nếp viết nháp của cộng đồng. Prompt, luật tiền và luật quyền riêng tư của
   bản Python được chép **nguyên văn**; bộ kiểm sau model được port sang `internal/domain/*` và chứng minh
   bằng golden render từ chính hàm Python trước khi xoá nó.
2. **Hai cờ `MOBILE_AI_ENGINE_NEP` và `MOBILE_AI_ENGINE_GROUP` bị xoá.** Engine Go là đường duy nhất cho Nếp,
   bot nhóm và chia bill. `core work` không còn đòi cấu hình brain; nó từ chối khởi động khi không có model.
3. **Python cũ bị xoá hẳn sau khi bản Go đã chạy và đã thử thật**, không giữ làm oracle: module gọi Gemini,
   route brain tương ứng, handler công khai của các route AI thuần (hàng manifest sang `PY-DELETED`), bộ kiểm
   Python chỉ phục vụ đường LLM, test đi kèm, và `google-genai` khỏi `services/api`. Route trộn (`GET /places`,
   `GET /places/{id}`) chỉ mất phần viết lý do bằng LLM. Golden đã render trước khi xoá là vector cố định.
4. **Khoá AI vào tiến trình `core serve`.** Các route trên là đồng bộ nên `serve` cần khoá; trước đây chỉ
   worker giữ. `core serve` không khoá vẫn khởi động và mỗi route trả đúng câu từ chối nó đã trả khi thiếu khoá.
5. **Không đụng**: tìm kiếm đang chạy trên Milvus (RAG phía Go) và sidecar `services/ai-infer` (rerank qua
   OpenRouter, trí nhớ mem0 kể cả lời gọi Gemini bên trong nó). Phân loại/intent/guard qua OpenRouter (jev)
   vẫn là việc của sidecar Python khi được xây.
6. **`face-boxes` ở lại brain Python tạm thời** (OpenCV, không phải LLM). Là việc còn nợ: chủ sản phẩm sẽ làm
   lại bằng cơ chế khác hoàn toàn bằng Go. Đến lúc đó `internal/brain`, `/internal/brain/v1/*` và
   `MOBILE_INTERNAL_TOKEN` chỉ còn phục vụ đúng một action này.
7. **Bằng chứng gọi thật** đi bằng `go run ./cmd/vnlocal-thu …` và E2E trên stack vnlocal với dữ liệu giả
   (`scripts/sinh_anh_gia_ai.py`, ghi ngoài Git), vì test binary không được ra khỏi loopback. Cổng eval T3 của
   ADR-0042 chưa chạy được qua agy; commit gỡ đường brain ghi `Eval-Chua-Do:` kèm lý do.

### Dữ liệu nào đi đâu (thêm vào bảng ADR-0049)

| Bên nhận | Nhận gì | Không bao giờ nhận |
|---|---|---|
| agy-proxy (máy vnlocal → Google) | ảnh bill / ảnh chuyển khoản đã sanitize (bỏ EXIF/GPS), tin nhắn được bấm đọc khoản chi, danh sách quán rút gọn, dữ kiện chuyến đi đã lọc, ảnh nhật ký người dùng chọn, bài đăng cộng đồng khi duyệt | id người, số dư, lịch sử không được chia sẻ, phòng v2 E2EE |

## 3. Hệ quả

- `services/api` không còn dòng nào gọi LLM; không service Python nào trong compose của `services/api` cần
  `GEMINI_API_KEY`.
- Một nhà cung cấp chung cho sinh chữ, trích xuất và thị giác: agy hết ghế thì trả 429 + `Retry-After`; mỗi
  tính năng có ngân sách lời gọi riêng (tính cả lần thử lại) và ghế song song của tiến trình (serve 4, work 2).
- Mất đường rollback về Python cho các route AI thuần: `MOBILE_FORCE_PYTHON` gọi đích danh bị từ chối, `all`
  và tên nhóm để chúng ở lại Go.

## 4. Cái này KHÔNG cho phép

- Không gọi provider thật trong CI, test hay parity (giữ ADR-0049 §4).
- Không để khoá agy/Gemini vào Git, app mobile hay log; không body nào của model vào log, metrics hay span.
- Không đổi model âm thầm: đổi model cho một bước phải sửa văn bản này.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

| Điều khoản | Nay |
|---|---|
| ADR-0029 §2.7 «Python chỉ còn brain» | Brain chỉ còn `face-boxes` (tạm, §2.6); không bước model nào ở Python |
| ADR-0036 §2.10 «một action brain cho Nếp» | Xoá; Nếp chỉ chạy trên engine Go |
| ADR-0042 §2.8 / §3 (commit gỡ đường brain mang T3) | T3 chưa chạy qua agy; trailer `Eval-Chua-Do:` (§2.7) |
| ADR-0044 §2.11 (khoá chỉ ở worker) | Khoá ở cả `core serve` (§2.4) |
| ADR-0044 §2.12 (cờ `MOBILE_AI_ENGINE_*` mặc định `brain`) | Cờ bị xoá (§2.2) |
| ADR-0044 §2.13 / ADR-0049 §2.4 (Python còn thị giác + sidecar) | Python còn sidecar OpenRouter (rerank, mem0) và `face-boxes` tạm |
| ADR-0047 §2.14 (gửi brain shortlist ≤30) | Shortlist gửi thẳng từ Go qua agy, cùng trần |
| ADR-0049 §2.5 (xoá brain theo từng tính năng, bật cờ) | Làm hết trong đợt này, cờ bị xoá thay vì bật |
| ADR-0049 §2.8 (trí nhớ Nếp dựng lại bằng Go) | Hoãn: mem0 ở lại sidecar (§2.5) |

## 6. Tiến độ (nhánh `claude/p0-ai-go-agy-bo-brain`, 2026-10-01) và chỗ lệch so với văn bản trên

| Lát | Commit | Nội dung |
|---|---|---|
| nền | `db60723d` | `motluot`, ảnh inline, `vnlocal-thu anh`; đo agy thật |
| cờ | `8d6d2792` | bỏ `MOBILE_AI_ENGINE_NEP/GROUP`, engine Go là đường duy nhất |
| bill/ảnh/khoản chi | `add7a953`, `6d48b5db` | golden rồi chuyển route, xoá Python |
| gợi ý/reel/thành tựu | `2c3b2579`, `de985f8e` | prompt khớp từng byte, xoá Python |
| nhật ký | `08705dd5`, `f6df3591` | vòng viết + kiểm khớp 16/16 ca, xoá `diary_gemini` |
| cộng đồng | `ae7b2d7d` | lời dặn mới, xoá `community_inference` |
| tìm quán + lý do | `609930ff`, `5f3b96e8` | 229/229 ca, xoá phần LLM của Python, `google-genai` rời `services/api` |

Lệch, có chủ ý:

- Bộ kiểm của tìm quán nằm ở `aiharness/timquan`, không ở `internal/domain/*`: nó đọc giá trị pyjson (số
  nguyên lớn, thứ tự khoá) mà biên domain thuần không cho import. Golden vẫn chứng minh như §2.1.
- Cộng đồng không có prompt cũ để chép (§1): `aiharness/congdong` viết lời dặn mới; `media_checked` do Go
  đặt (chỉ đúng khi mọi tệp là ảnh đã gửi cùng request), không hỏi model.
- `GET /places`, `GET /places/{id}`: handler Python không đổi một dòng; `create_app` cài một reason writer
  không trả lời ai (`no_reasons`). Đó đúng là điều core không khoá phục vụ, và giữ hai route làm oracle
  parity cho phần danh mục (§2.3 «chỉ mất phần viết lý do»).
- Nhật ký: trần ảnh 14 MiB (Python 24 MiB) vì agy nhận thân ≤ 20 MB; job 150 s, lease 160 s, mỗi lời gọi
  45 s — qua agy một lời gọi có lúc treo quá một phút.
- Câu trả lời của model có thể bọc trong rào ```json dù đã xin MIME JSON (qua agy): `motluot.BoRao` bỏ đúng
  một rào trước khi đọc. Python không có bước này.
- Tìm quán: hai cổng từng dòng (số không nguồn, chép lại câu người gõ) route Python có mà bản Go trước đợt
  này thiếu — nay có ở Go.

Còn mở: `docs/team/hang-doi.md` mục 2026-10-01.

