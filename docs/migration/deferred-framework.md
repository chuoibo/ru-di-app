# Framework routes còn lại Python (DEFERRED)

Năm hàng `framework` / `static` trên manifest **không** có handler Go.
`owner` vẫn `python`. `state` là `DEFERRED`, không `LIVE-GO`.

Lý do: OpenAPI, Swagger UI, ReDoc, oauth2-redirect và `MOUNT /static`
là bề mặt FastAPI/Starlette. Port từng byte sang Go không đổi hành vi
sản phẩm (số tiền, quyền, AI) và sẽ làm lệch goldens `TestMatchesStarletteGoldens`
cũng như `len(app.routes) == len(manifest)`.

`GET /healthz` **không** nằm trong nhóm này: đó là liveness `{"status":"ok"}`,
không đụng database. Trạng thái hiện tại là `PORTED-UNPROVEN` (chưa có một lượt
`gate.sh parity` đầy đủ trên SHA sạch; xem `docs/migration/ported-unproven-w7-wai.md`).
