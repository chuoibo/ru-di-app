# ADR-0051 — mọi lời gọi LLM về Go qua agy, brain Python đã xoá

- Ngày: 2026-10-01. Nhánh: `claude/p0-ai-go-agy-bo-brain` (worktree `wt-ai-go-agy`), gốc `dcb2e976`.
  Cập nhật cùng ngày: đã merge vào `main` ở `16c0fdca` (0 xung đột, fast-forward `main`), chưa push.
- protocol_version: không đụng (`docs/protocol/v1/` đóng băng).
- Verdict: không có reviewer; đây là ghi chép của người làm, không phải review.
- Quyết định: ADR-0051 (chủ sản phẩm 2026-09-30, thêm 2026-10-01 «test thử xong thì xoá stale Python»).

## Đã làm, theo commit

| Commit | Lát |
|---|---|
| `db60723d` | nền `motluot`, ảnh inline, đo agy thật |
| `7c09a546` | ADR-0051 + dòng «Sửa bởi» ở ADR cũ |
| `8d6d2792` | bỏ cờ engine; engine Go là đường duy nhất của Nếp, bot nhóm, chia bill |
| `add7a953`, `6d48b5db` | đọc bill, ảnh chuyển khoản, khoản chi: golden → route Go → xoá Python |
| `2c3b2579`, `de985f8e` | gợi ý ×2, reel, câu dẫn hành trình |
| `08705dd5`, `f6df3591` | nhật ký (vòng viết + kiểm) |
| `ae7b2d7d` | duyệt bài + Nếp cộng đồng (lời dặn mới) |
| `609930ff`, `5f3b96e8` | tìm quán + lý do quán; `services/api` hết lời gọi LLM, hết `google-genai` |
| `e261c77e` | sửa test janitor xoá phiên của package bên cạnh (nguyên nhân các lần đỏ chập chờn của tầng Postgres) |

## Bằng chứng đã xem

**Cây sạch đúng `5f3b96e8`** (`~/.cache/rudi-bang-chung/agy/verify-5f3b96e8.log`): go vet + go test ./... xanh;
pytest 3022 passed, 744 skipped; check_route_ownership OK (227 hàng); check_go_owned_python_touch OK (144 route);
tầng Postgres ĐỎ ở hai test đọc phiên (401/resync) — nguyên nhân là test janitor, sửa ở `e261c77e`; sau sửa
tầng Postgres chạy hai lần liền: 3285 ca PASS, sentinel có mặt; parity: 351 kịch bản / 10756 bước, 9 / 209 và
23 / 605, cả ba `scenarios_diff=0 differences=0`, database lane và media lane bật.

**Request thật qua agy-proxy, từng bước** (`go run ./cmd/vnlocal-thu tinh-nang`, dữ liệu bịa, `gemini-3.5-flash-lite`):
mọi bước đã chạy XANH ít nhất 2 lần; số liệu từng lát nằm trong commit message của lát đó.

**E2E qua HTTP thật, cửa trước Go của nhánh.** Hai chỗ chạy, vì DB của stack vnlocal chưa có bảng hồ sơ/nhật
ký/cộng đồng (chính stack `main` đang chạy cũng trả 500/503 ở các route đó) và tôi không migrate DB dùng chung:

1. `core` của nhánh chạy cạnh stack `rudi-vnlocal` (project riêng `rudi-e2e-agy`, image riêng, cổng 8125,
   volume media riêng; dùng lại `api`, Milvus và DB của stack đó), 3 người tạo bằng `genesis_session.py`,
   không OTP (`e2e-lan3.log`):
   - quét bill 2,4 s (3 món, 165.000đ) · quét ảnh chuyển khoản 4,7 s (banking, 250.000đ) · thực đơn: 502
     `receipt_reader_unavailable` sau 45,8 s (agy chậm; lượt trước từ chối đúng `not_a_receipt` trong 1,9–4,9 s)
   - gợi ý theo cuộc trò chuyện 3,5 s và gợi ý nhóm 5,4 s: `source=ai` · reel 6,2 s: 6 ảnh
   - tìm quán 23,9 s: 2 quán thật của danh mục vnlocal · GET /places: 2/2 thẻ có lý do AI qua cổng số
   - Nếp 11,3 s · bot nhóm: hỏi quán 19,5 s, chia bill 15,5 s, đòi chuyển 200k → câu từ chối cố định «Rủ Đi AI
     không làm việc tiền nong giữa mọi người: …» (đọc thẳng từ thẻ trong phòng)
   - nháp khoản chi từ một tin: 403 `explicit_invocation_required` — đúng thiết kế khi chat bật (chỉ AI khi
     được gọi rõ); bước model của nó chạy qua `/chia-bill` và `vnlocal-thu khoan-chi`
2. `core` + `api` của nhánh trên DB riêng `claude-dev-pg/e2e_agy` (đã migrate đủ), agy thật
   (`e2e-local-lan1.log`): nhật ký AI 20,1 s (3 ảnh, 3 trang, `ai_generated`) · câu dẫn hành trình 6,7 s
   (`source=ai`) · cộng đồng: bài đúng chủ đề → `approved`, bài quảng cáo có câu đòi duyệt → `rejected`, Nếp
   viết nháp 200 · reel, quét bill/ảnh xanh. Tìm quán/gợi ý ở đây không có danh mục nên trả `unavailable` trung
   thực, không gọi model; Nếp/bot hỏi hết ngân sách lời gọi (không Milvus, danh mục rỗng) — đã xanh ở chỗ 1.
   Log của `api` nhánh: 0 request `/internal/brain/*`.
3. Container `api` của stack vnlocal không có biến AI nào (`GEMINI_API_KEY`, `AGY_PROXY_*`,
   `OPEN_ROUTER_API_KEY`: 0). (Tiến trình `api` cục bộ ở chỗ 2 thừa kế khoá từ script chạy tay của tôi — không
   phải cấu hình sản phẩm; compose của repo không đưa khoá vào `api`.)

Dữ liệu E2E còn lại trên DB vnlocal: 3 người «… E2E agy», một nhóm «Nhóm E2E agy (dữ liệu mẫu)», tin nhắn, chuyến
và kỷ niệm ảnh (file ảnh nằm ở volume `rudi-e2e-agy_e2e-media`, không ở volume của stack chính).

## Còn mở

Ở `docs/team/hang-doi.md` mục 2026-10-01: nháp khoản chi lặp chữ (0–30% lượt), độ trễ agy 30–90 s lúc bận,
duyệt bài cộng đồng chưa đánh giá có hệ thống, `face-boxes` → Go, eval T3 qua agy. (Khoản «pin bắc cầu của google-genai còn trong
requirements-dev» ghi trước đây là sai: tính lại bao đóng phụ thuộc, mọi gói đó vẫn có gói khai báo cần.)

## Sau merge (cập nhật cùng ngày)

Merge vào `main` ở `16c0fdca`: 0 xung đột chữ (hai bên cùng sửa `services/api/app/api/service.py`, git tự gộp); quét
xung đột ngầm — không mã nào của `main` dùng thứ nhánh đã xoá. Cây gốc có việc dở chưa commit của phiên
`mobile-ui-audit-upgrade`, nên `main` được fast-forward từ một worktree riêng; trước/sau mỗi lần đều so index và
`git status` của cây gốc — không đổi.

Cổng trên SHA đã merge `16c0fdca`: go vet/test xanh; pytest 3023 passed; tầng Postgres 3291 ca PASS; parity 351 kịch bản /
10758 bước, 9 / 209, 23 / 605 — `scenarios_diff=0 differences=0`. Trên `main` sau đó (`ae778c00` → `e182ce64`):
`npm test` mobile 1393/1393 (gồm `build:check`), `go_broker_tier.sh` 180 ca PASS, `eval_kich_ban.sh` 36/36 lượt (canary
đỏ đúng chỗ), `chat_e2e_go.sh` 43 ca PASS không SKIP, `e2e_slice.sh` 11 pass + 4 skip (đường vẽ ảnh Nếp, `NEP_PROXY_URL`
không cấu hình — ngoài ADR-0051), lint `ruff_changed.sh d95edb4c` sạch 31 file, contract/CORS/screens/money/Dockerfile
pinning xanh. Không chạy: `go_milvus_tier.sh`, `ai_infer_tier.sh` (runner riêng; đợt này không đổi Milvus hay
`services/ai-infer`).

Lỗi tìm ra sau merge và đã sửa:
- `ae778c00` — test mobile `goi-ai-chia-bill` đỏ: app còn câu cho mã `chia_bill_no_expenses`, mã chỉ đường brain ghi (đã xoá
  ở `8d6d2792`); engine Go trả «chưa thấy khoản» ngay trong phòng. Do tôi không chạy bộ test mobile trước khi merge;
  phiên `mobile-ui-audit-upgrade` phát hiện.
- `5ec73969` — ba file Python nhánh chạm chưa `ruff format` (cổng lint của CI).
- `478b656f` — `scripts/mutation_cong_cua_so_model.py` vào `scripts/archive/` (cổng và caller của nó đã xoá).
- `5d3e5476` — bổ sung bằng chứng route cho 11 route sổ hai người mà `a6341c19` (phiên khác) đổi Python + Go nhưng chưa
  ghi bằng chứng; `check_go_owned_python_touch.py` xanh lại.
