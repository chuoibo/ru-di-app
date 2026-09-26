# Framework routes còn lại Python (DEFERRED)

**Bốn** hàng `framework` trên manifest không có handler Go: `GET /openapi.json`,
`GET /docs`, `GET /docs/oauth2-redirect`, `GET /redoc`. `owner` vẫn `python`,
`state` là `DEFERRED`.

`MOUNT /static` **đã rời nhóm này** (2026-09-23). Lý do nó khác hẳn bốn hàng
kia: nó không phải bề mặt FastAPI sinh ra từ chính ứng dụng, nó phục vụ ba file
trên đĩa — và **trang khách đã là của Go** trong khi stylesheet thì chưa, nên bỏ
lại là để Go trả HTML trỏ vào CSS của Python. Xem
`docs/migration/routes/static/MOUNT-static.md`.

Bốn hàng còn lại thì đúng là bề mặt FastAPI/Starlette: OpenAPI, Swagger UI,
ReDoc và oauth2-redirect được **dựng từ chính `create_app()`**. Port từng byte
sang Go không đổi hành vi sản phẩm (số tiền, quyền, AI).

## Cái gì thật sự chặn việc xoá Python

Đo ngày 2026-09-23: **10 script dựng sự thật bằng cách chạy `create_app()`**.
Chúng không cùng một loại, và gộp lại thì kế hoạch xoá Python sai cỡ:

| Loại | Số | Khi Python biến mất |
|---|---|---|
| **Renderer** (`render_*`, `mutation_*`) | 8 | Hết chạy được, và **đó là đích đến** — goldens đã commit thì đóng băng thành bộ hồi quy của Go |
| **Cổng chạy mỗi lượt** | 3 | Vỡ, trừ một cái tự nghỉ |

Ba cổng:

- `check_api_contract.py` — dựng OpenAPI từ `create_app()`. **Phải trỏ sang Go**
  (cần lệnh con `core openapi`, hiện chưa có).
- `check_route_ownership.py` — gọi `render_route_manifest.py --check`, tức dựng
  manifest từ chính app Python. **Phải trỏ sang Go.**
- `check_go_owned_python_touch.py` — dựng đồ thị gọi AST trên cây Python.
  **Tự nghỉ**: không còn Python thì nó vô nghĩa, xoá cùng.

Tức **hai** cổng phải chuyển nguồn sự thật, không phải một đống mơ hồ. Và đó là
lý do việc chuyển nguồn manifest thuộc về *lúc* xoá, không phải trước đó: Go
dựng router **từ** manifest, nên bắt Go làm nguồn của manifest là vòng tròn;
chỉ khi phía Python biến mất thì phép kiểm hai chiều mới thành một chiều.

## Ai còn gọi bốn route này

Đo, không đoán:

| Route | Người gọi |
|---|---|
| `GET /openapi.json` | `scripts/hero_walk.sh` (curl thật) · `Makefile` (demo-check đếm đủ route) |
| `GET /docs` | chỉ một dòng in đường dẫn trong `Makefile`, không ai đọc nội dung |
| `GET /redoc` · `GET /docs/oauth2-redirect` | **không có** người gọi trong mã |

Nên `/openapi.json` là hàng duy nhất trong bốn hàng có người gọi thật, và nó
trùng đúng với cổng `check_api_contract.py` cần `core openapi`. Làm một lần
được cả hai.

## `/healthz` không nằm trong nhóm này

Đó là liveness `{"status":"ok"}`, không đụng database.
