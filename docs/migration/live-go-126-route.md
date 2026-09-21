# 126 route chuyển hẳn sang Go (LIVE-GO)

Ngày 2026-09-21. Thực hiện: Claude, theo uỷ quyền ADR-0016/ADR-0029 và quy tắc
toàn repo ngày 2026-09-21 (ADR-0031): backend nghiệp vụ là Go, Python chỉ còn AI.

## Trước bước này cây đang ở đâu

Manifest ghi `owner: python` trên **cả 156 hàng**. Go có code cho 151 route,
nhưng chỉ phục vụ khi bật cờ `MOBILE_CORE_CANDIDATE_ROUTES=ported` — một cờ của
làn đo parity. Cờ ấy mặc định **trống** trong `docker-compose.yml`, nên
`make up` bình thường cho Go phục vụ **0 route** và proxy toàn bộ sang Python.

Nói cách khác: code đã chuyển, bằng chứng đã có, nhưng cái công tắc định tuyến
vẫn ở vị trí Python. Quy tắc toàn repo nói thẳng điều đó chưa phải là xong:
«Không coi proxy sang Python là hoàn tất migration.»

## Bước này đổi gì

126 hàng `PORTED` trong `services/core/ownership/routes.json`:

| Trường | Trước | Sau |
|---|---|---|
| `state` | `PORTED` | `LIVE-GO` |
| `owner` | `python` | `go` |
| `evidence` | thiếu ở 86 hàng | trỏ vào thẻ route trong `docs/migration/routes/` |
| `python` | `live` | `live` — **không đổi** |

`python: live` giữ nguyên có chủ đích: code Python vẫn nằm đó để
`MOBILE_FORCE_PYTHON` lùi về được mà không cần dựng lại ảnh. Đóng băng
(`FROZEN`) rồi xoá (`PY-DELETED`) là hai bậc sau, mỗi bậc một PR.

Không hàng nào khác bị đụng: 25 hàng `PORTED-UNPROVEN` (24 route WAI + `/healthz`)
chưa có kịch bản parity, và 5 hàng `DEFERRED` là route do FastAPI tự sinh.

## Bằng chứng

Đo trong worktree sạch cắt từ `origin/main` tại `534c0fd1`:

| Cổng | Kết quả |
|---|---|
| `check_route_ownership.py --selftest` | mọi phép kiểm đỏ được ở ca hỏng, xanh ở ca lành |
| `check_route_ownership.py` | `156 rows, 126 served by Go` |
| `check_go_owned_python_touch.py` | `126 Go-served route(s), 0 changed function(s)` |
| `go build ./...` · `go vet ./...` · `gofmt -l` | 0 lỗi · 0 lỗi · sạch |
| `go test ./...` | thoát 0, không gói nào đỏ |
| `repo_guard.py staged` | 1 file, đạt |
| `core routes --json` | 151 handler (126 LIVE-GO + 25 ứng viên) |
| `make demo-check` | `129 route, không thiếu, không thừa` |

Cổng parity đầy đủ, bốn pha, mỗi pha một cặp stack cô lập (reference là ảnh
Python ghim theo digest, candidate là `core` + Python sau tap):

| Pha | Kịch bản | Bước | Khác biệt |
|---|---|---|---|
| dev main | 339 | 10.369 | 0 |
| dev canary | — | — | 11 chế độ đột biến, `identity` 0/0, tất cả `ok` |
| dev limiter + probe | 9 | 209 | 0 · probe 22 ca, 10 lệch đều là ngoại lệ ADR, `unexpected=0 stale=0` |
| prod main + canary | 23 | 604 | 0 · canary 13 chế độ `ok` |

Tap chứng minh Go thật sự trả lời chứ không proxy lén: `answered_in_core=9857`
trên 10.369 bước ở pha dev, 601 trên 604 ở pha prod, `served_routes=151`,
`unserved=0` cả hai.

Hàng `identity 0/0 ok` của canary là hàng dễ bỏ qua nhất mà lại quan trọng
nhất: nó nói bộ so **không** đỏ khi không có gì sai. Mười hai hàng còn lại nói
nó đỏ khi có — gồm đúng những dạng lệch wire mà app ăn ngay:
`float-lost-point`, `zone-written-as-offset`, `first-keys-swapped`,
`cursor-padding-kept`, `token-shortened`.

Lượt chạy đầu bị `timeout 5400` cắt sau pha canary (`rc=124`). Đó là mã của
`timeout`, không phải mã đỏ của cổng; hai pha còn lại được chạy lại riêng trên
máy rảnh bằng đúng lệnh của CI matrix.

Phép thử quyết định là log khởi động của `core` sau `make up` **không đặt
biến môi trường nào**:

```
"go_served":126  "candidates":0  "manifest_routes":156  "force_python":null
```

`candidates: 0` là chỗ đáng đọc kỹ. Trước bước này con số `go_served` chỉ khác 0
khi có người bật cờ; giờ Go phục vụ 126 route vì manifest nói thế. Đó là khác
biệt giữa «chạy được trong phòng thí nghiệm» và «đang chạy».

Log vẫn là lời của chính chương trình, nên phép thử cuối là tắt hẳn container
Python rồi gọi thật qua cổng 8099:

| Gọi | Kết quả | Đọc ra |
|---|---|---|
| `GET /interests`, `GET /areas` (LIVE-GO) | `200` | Go trả lời khi Python không còn tiến trình nào |
| `GET /contexts/{id}/members` (LIVE-GO, có DB) | `403 {"code":"permission_denied","detail":"role_not_permitted"}` | không phải 502: Go tự xác thực và tự đọc bảng `memberships` để biết vai trò không đủ quyền |
| `GET /healthz` (PORTED-UNPROVEN) | `502` | vẫn là của Python, đúng như manifest ghi |
| `GET /openapi.json`, `GET /docs` (DEFERRED) | `502` | vẫn là của Python, đúng như manifest ghi |

Hàng thứ hai là hàng quan trọng. Một route tĩnh trả 200 chỉ chứng minh Go có
handler; một câu trả lời 403 đúng mã lỗi chứng minh Go đã mở kết nối database,
đọc vai trò và tự quyết định — không có Python nào ở giữa.

## Cái này KHÔNG chứng minh

- Không chứng minh 25 route WAI đúng: chúng vẫn do Python phục vụ.
- Không chứng minh Python đã bỏ được. Python vẫn chạy, vẫn giữ DSN, và vẫn là
  nơi 5 route framework được phục vụ.
- Không chứng minh tải thật, kế hoạch truy vấn thật, hay hành vi Gemini.

## Còn lại gì trước khi bỏ hẳn Python

1. **25 hàng `PORTED-UNPROVEN`** — chạy kịch bản parity cho 24 route WAI và
   `/healthz`, rồi lật. Sau đó Go phục vụ 151/156.
2. **5 hàng `DEFERRED`** — `/openapi.json`, `/docs`, `/docs/oauth2-redirect`,
   `/redoc`, `MOUNT /static`. Hai việc phải làm: viết lệnh con `core openapi`
   (hiện `core` chỉ có `serve`, `healthcheck`, `routes`) và bỏ điều kiện
   `class == "framework"` trong `manifest.go`, vốn đang cấm mọi route framework
   về Go. `/static` chỉ có 3 file nên nhúng thẳng vào binary là đủ.
3. **Xoá code Python** — khoảng 48.800 dòng trên tổng 50.421 dòng của
   `services/api/app`. Giữ lại ~1.600 dòng AI: sáu module Gemini,
   `routes/brain.py`, `media/face_detection.py`, `places/prompt_safety.py`.
4. **Hai quyết định phải mở ADR trước, đừng sửa code trước:**
   - **Alembic.** Python đi thì ai sở hữu schema — giữ Alembic làm ảnh migrate
     một-lần, hay chuyển goose/atlas từ một baseline đóng băng.
   - **509 file / 129.513 dòng test Python**, nhiều gấp 2,5 lần code Python.
     Đây là phần đắt nhất và dễ tự lừa nhất: xoá chúng mà không ánh xạ từng ca
     sang test Go thì cây trông sạch hơn trong khi thứ vừa mất là bằng chứng,
     không phải legacy.
