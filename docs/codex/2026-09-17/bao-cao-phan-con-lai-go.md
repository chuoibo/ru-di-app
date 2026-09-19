# Báo cáo phần còn lại chiến dịch Go — nhờ Claude quyết

**Ngày:** 2026-09-17
**Tác giả:** agent Cursor trên worktree riêng, không phải Claude
**Cây:** `/home/lakiet/wt-go-con-lai`
**Nhánh:** `go/p0-w-con-lai`
**HEAD đã commit:** `23ab52273ee80b7b605516194e547316e6809f88` (`docs(bàn giao): trạng thái, việc còn dở và giao thức agy cho người kế tiếp`)
**Trạng thái git:** **chưa commit, chưa push, chưa mở PR.** Toàn bộ phần dưới đây nằm unstaged / untracked trên HEAD đó.
**protocol_version:** `v1` (không đụng `phase0/` hay `docs/protocol/v1/`)
**Veredict của tác giả:** không ký. Charter cấm tự review. File này là đầu vào cho Claude.

Claude: đọc diff trên cây này, đối chiếu bàn giao `docs/claude/2026-09-16/ban-giao-chien-dich-go.md`, rồi ghi `APPROVE` / `REQUEST_CHANGES` / `REJECT` theo ADR-0007. Không lấy digest agent làm bằng chứng.

---

## 1. Claude cần quyết gì

Ba câu, không hơn:

1. Có nhận HTTP W7 + seam brain + 24 route WAI + `GET /healthz` PORTED + 5 hàng framework `DEFERRED` vào nhánh chiến dịch (rebase `--3way` sau khi W9/stub của Claude lên HEAD), hay trả về sửa?
2. Có chấp nhận hai chỗ **cố ý lệch** bàn giao 2026-09-16 không? (mục 4)
3. Phần chưa chứng minh (mục 7) là blocker hay suggestion trước khi gộp?

Không lật `LIVE-GO`. `owner` mọi hàng vẫn `python`.

---

## 2. Phạm vi Lead giao, và cái không đụng

Lead: làm nốt trên worktree riêng, **không đọc/chép/sửa** ba cây Claude đang chạy.

| Cây Claude (không đụng) | Việc Claude đang giữ |
|---|---|
| `/home/lakiet/wt-go0-valhalla-stub` | stub Valhalla tất định cho parity |
| `/home/lakiet/wt-go0-w9-domain` | domain auth/sessions |
| `/home/lakiet/wt-go0-w9-repo` | repository auth/sessions |

Cũng không làm trên `/home/lakiet/mobile` (cây bẩn). Không viết OTP, không viết stub, không sửa harness `ts#N` / bind INFRA / `free_port` (cổng unit không đỏ vì những chỗ đó).

Bàn giao lúc `cff1ba79`: 108/156 PORTED, còn 48 hàng. Domain/repo/thẻ W7 đã commit trên chiến dịch; **chưa có** `internal/routes/outings.go`.

---

## 3. Manifest sau phần này (chưa commit)

156 hàng, `owner: python` cả bảng, `LIVE-GO` = 0.

| state | Số | Ý nghĩa |
|---|---|---|
| `PORTED` | **144** | Go có handler; chỉ phục vụ khi `MOBILE_CORE_CANDIDATE_ROUTES=ported` |
| `PY` | **7** | W9: 4 sessions + 3 auth. Claude đang làm domain/repo |
| `DEFERRED` | **5** | OpenAPI / docs / oauth2-redirect / redoc / `MOUNT /static`. Lý do: `docs/migration/deferred-framework.md` |

Cổng ownership (cây bẩn, không phải SHA sạch ADR-0030): `156 rows, 0 served by Go`.

Đối chiếu bàn giao «còn lại»: outings 11 + WAI 24 + healthz 1 = 36 hàng chuyển `PORTED`; 5 framework `DEFERRED`; 7 W9 giữ `PY`. 108 + 36 = 144.

---

## 4. Hai chỗ lệch bàn giao — xin Claude phán

### 4.1 Port HTTP W7 khi chưa có stub Valhalla

Bàn giao §4.1: *«Không port route W7 trước khi có stub.»*

Lead sau đó duyệt làm HTTP W7 trên cây này: `configured_provider()` thiếu env → router nil → preview `status: unavailable` + `routing_unavailable`. Đó **đúng** Python trên HEAD khi `scripts/parity_stacks.sh` không đặt `MOBILE_VALHALLA_URL`. Stub Claude viết sẽ nối parity nhánh *routed*; cây này **không** đổi kịch bản `owner_previews_a_clean_day` kỳ vọng unavailable.

Client IO: `services/core/internal/routing/` (không nằm `domain/`). Thiếu env / URL xấu → `Configured()` nil. Có URL thì POST JSON, timeout 12s, không proxy, không follow redirect, semaphore 4 slot.

### 4.2 Brain không vào bảng route công / OpenAPI / manifest

Bàn giao §4.4: *«thêm route vào `create_app()` làm đổi bảng route, kéo theo golden router và manifest — phải thêm hàng manifest trong cùng PR.»*

Làm khác có chủ ý: `BrainDoor` ASGI **trong** CORS, **ngoài** guest/idempotency; `startswith("/internal/")`. `build_brain_app(token)` không `include_router` lên public app. Lý do: `TestMatchesStarletteGoldens` đòi FULL golden cho mọi hàng manifest và `len(app.routes) == len(manifest)` = 156. Brain trên public table sẽ phá cả hai.

Hệ quả cần Claude xác nhận:

- Python công vẫn gọi skill tại chỗ; Go candidate gọi `internal/brain` bằng `MOBILE_BRAIN_URL` hoặc `MOBILE_PYTHON_UPSTREAM` + `X-Internal-Token`.
- Cửa trước Go **404** `{"detail":"Not Found"}` cho `/internal` và `/internal/*` *trước* preflight/python, để cổng 8099 không lộ brain.
- Idempotency Python bỏ qua `path_parts[0]=="internal"`.
- Token fail-closed: vắng / rỗng / chỉ khoảng trắng → `create_app()` ném `InternalTokenMissing`. Compose: `MOBILE_INTERNAL_TOKEN` mặc định `local-brain-token`. Test/script: `test-brain-token` (chỉ chữ, repo-guard).

---

## 5. Việc đã làm (uncommitted)

Khoảng **+478 / −222** trên 24 file đã track, cộng **~7.8k dòng** untracked.

### 5.1 HTTP 11 route W7 (outings)

Handler: `services/core/internal/routes/outings.go` (+ `outing_store.go`, `outing_wire.go`, `outing_request.go`, `outings_test.go`).

| ID | status |
|---|---|
| `POST /outings/{outing_id}/itinerary/preview` | 200 — limiter rồi `PreviewOutingItinerary` → `itinerary.Build` |
| `PUT /outings/{outing_id}/itinerary` | 200 — replay idempotent (Idempotency-Replayed + no-store) hoặc `ReplaceOutingItinerary` |
| `POST /contexts/{context_id}/outings` | 201 |
| `GET /contexts/{context_id}/outings` | 200 |
| `PUT /outings/{outing_id}/timeline` | 200 |
| `POST /outing-stops/{stop_id}/checkins` | 201 |
| `GET /outings/{outing_id}/checkins` | 200 |
| `POST /outings/{outing_id}/invites` | 201 |
| `POST .../revoke` | 200 |
| `POST .../rotate` | 200 |
| `POST /outing-invites/{token}/accept` | 200 — digest token |

Limiter hook: `CallGetItineraryLimiter` no-op; handler gọi `Limits.ItineraryLimiter.Check`. 429: `itinerary_rate_limited`. `outingRefusal` chỉ map `*outingsteps.Refusal`; `permissions.Error` / invariant / conflict lạ = 500 như Python.

Validator W7 chuyển từ `oracle_ports_test.go` (tag `oracle`) sang `registerServedValidators` trong `ports.go` — binary phục vụ được mới bind.

### 5.2 Seam brain Python + client Go

Python:

- `services/api/app/api/internal_token.py` — `resolve_internal_token`, `tokens_match` (hmac)
- `services/api/app/api/routes/brain.py` — GET `/ready` + 11 POST: receipt-scan, screenshot-scan, chat-expense, companion-plan, companion-reply, place-search, place-reasons, suggestion, contextual-suggestion, reel, face-boxes
- `main.py` gắn `BrainDoor` trước CORS outermost
- `idempotency.py` bỏ qua `/internal`
- conftest + script cổng: `setdefault MOBILE_INTERNAL_TOKEN=test-brain-token` (future import đứng trước `os.environ` ở `render_route_manifest.py`)
- `check_pinned_import.sh` docker `-e MOBILE_INTERNAL_TOKEN=test-brain-token`

Go:

- `services/core/internal/brain/` — timeout 60s, 2 MiB, không proxy/redirect; nil client → 502 `brain_unavailable`
- `dispatch.go` chặn `/internal` công
- `endpoint.go` — `get_actor_optional` (thiếu credential → actor nil; có header sai vẫn 401) + no-op cho companion/readers/suggesters/reeler/face/receipt/screenshot/searcher/reason_writer và mọi limiter WAI

Test Python: `tests/api/test_internal_token.py`, `tests/api/test_brain_seam.py` (12 ca: token rỗng, CORS ngoài BrainDoor, không có `/internal` trên public/OpenAPI, 401 ready, 422 companion-plan, Idempotency-Key không reserve trên brain).

### 5.3 24 route WAI

Go domain mới (thuần, không import db/api): `album`, `catalog`, `chatintent`, `companion`, `conversation`, `faces`, `messageedit`, `promptsafety`, `reel`, `stickers`, `suggestion`.

Repo: `messages.go`, `destinations.go`, `place_photos.go`, `outing_memories.go`; `memories.go` tách `scanMemory` / social; `contexts.go` thêm `SetMembershipRole`.

HTTP: `messages_wai.go`, `places_wai.go`, `albums_wai.go`, `scans_wai.go`, `suggestions_wai.go`, `wai_support.go`. `All()` nối 24 id + `healthz` sau 11 outing.

Hành vi ghim theo Python (xin Claude soi chỗ dễ sai):

- POST messages: **lưu trước**, rồi parse intent; limiter 429 trên plan/mention/chia_bill thành `intent_error` trong 201, không 429 HTTP.
- Photo URL: `photoref` chỉ nhận path `contexts`, không `people`.
- Search: 200 source `none` khi brain hỏng; `_reject_blank` production (`query must not be blank`).
- Receipt `PRICE_LIST` → `not_a_receipt_price_list`.
- Face: 503 `face_detector_not_configured` vs 502 `face_detection_failed`.
- Suggestion: `summarise_history` / `summarise_conversation` local, rồi brain; `no_history` / `no_conversation` trước model.
- `TasteProfile` UNKNOWN, không `.unknown()`.
- Reeler `(trip, memories)`.
- `group_photos_at_place` không load social counts; `list_outing_memories` có.

### 5.4 Framework / healthz

- `GET /healthz` PORTED: `{"status":"ok"}`, không DB.
- Năm hàng framework `DEFERRED`, không handler Go, không field `reason` trên JSON (DisallowUnknownFields). Lý do tiếng Việt trong `docs/migration/deferred-framework.md`.

---

## 6. Cổng đã chạy trên cây bẩn (2026-09-17)

Không phải cổng ADR-0030 (cây sạch + SHA + đột biến người gộp).

```text
cd /home/lakiet/wt-go-con-lai/services/core
CGO_ENABLED=0 go build ./... && go vet ./... && test -z "$(gofmt -l .)"
go test -count=1 ./internal/routes/ ./internal/routing/ ./internal/httpapi/endpoint/ \
  ./internal/brain/ ./internal/httpapi/dispatch/ ./internal/pyval/
go test -count=1 ./internal/httpapi/router/ -run TestMatchesStarletteGoldens
go test -count=1 ./internal/domain/... ./internal/repo/
cd /home/lakiet/wt-go-con-lai
python3 scripts/check_route_ownership.py   # 156 rows, 0 served by Go
python3 scripts/repo_guard.py staged       # 0 file staged — cổng này gần như rỗng
cd services/api && python3 -m pytest tests/api/test_internal_token.py \
  tests/api/test_brain_seam.py tests/api/test_healthz.py -q
# 12 + 2 passed
```

Nhiều package domain mới **không có file test** (album, companion, conversation, faces, reel, suggestion, …). `chatintent` có test. `internal/repo` unit không đụng Postgres.

---

## 7. Việc chưa làm / chưa chứng minh — để Claude xếp blocker

1. **`gate.sh parity` W7 và WAI chưa chạy lại** trên SHA sạch. Unit xanh ≠ parity. Stub định tuyến (`3cff2c30`) đã đặt `MOBILE_VALHALLA_URL` cho cả hai stack — preview parity không còn dừng ở `unavailable`.
2. **Chưa commit.** Repo-guard `staged` không quét untracked. Trước commit phải `repo_guard.py staged` sau khi add.
3. **Chưa postgres live** cho messages / destinations / place_photos / outing_memories / SetMembershipRole.
4. **Harness** (`ts#N`, bind INFRA, `free_port`) không sửa. Ảnh AVIF/TIFF lệch Go/Python vẫn như QUEUE.
5. **W9** bảy hàng auth/sessions: domain (`8ebc2334`) và repo (`ca38f0e0`) đã trên chiến dịch; HTTP là việc của nhánh này sau rebase, không chép worktree Claude.
6. **Không LIVE-GO.** Candidate `ported` mới thấy handler.
7. Domain WAI phần lớn chưa có golden Python-cùng-hàm.
8. Brain Python: public route vẫn gọi skill local; chỉ Go candidate đi HTTP. So parity MIXED/AI cần cả hai stack cùng token và cùng brain URL.

---

## 8. Cách Claude đọc (không cần hỏi tác giả)

```bash
cd /home/lakiet/wt-go-con-lai
git status -sb
git diff HEAD
git ls-files --others --exclude-standard
```

Đọc trước: `outings.go`, `brain.py`, `internal_token.py`, `dispatch.go` (chặn `/internal`), `endpoint.go` (hook WAI), `messages_wai.go`, `places_wai.go`, `routes.json` (PY còn 7, DEFERRED 5), `deferred-framework.md`.

Ba cây Claude: `git status` trong từng worktree trước khi rebase. Công chưa commit của Claude nằm đó.

---

## 9. Đề nghị (không phải verdict)

Nhận vào nhánh chiến dịch **sau review độc lập**, commit tách theo sóng (W7 HTTP / brain / WAI / healthz+DEFERRED) nếu Claude muốn diff nhỏ. Giữ `owner: python`. Stub + W9 + parity + đột biến người gộp vẫn là cổng ADR-0030, không nằm trong session này.
