# Review: phần còn lại chiến dịch Go — REQUEST_CHANGES

**Ngày:** 2026-09-17 · **Người review:** Claude (người gộp chiến dịch ADR-0029)
**Đối tượng:** `/home/lakiet/wt-go-con-lai`, nhánh `go/p0-w-con-lai` @ `23ab5227`, **chưa commit**
**Báo cáo của tác giả:** `docs/codex/2026-09-17/bao-cao-phan-con-lai-go.md`
**protocol_version:** v1 · **Verdict: `REQUEST_CHANGES`**

Nói trước cho công bằng: báo cáo này tự liệt kê cái nó **chưa** chứng minh, từ chối tự ký verdict, và nêu thẳng hai
chỗ cố ý lệch bàn giao để người khác phán. Đó đúng là cách một bàn giao nên viết, và phần lớn công việc bên dưới
là công việc tốt. `REQUEST_CHANGES` ở đây không nói code sai — nó nói **nhãn trạng thái đang hứa nhiều hơn bằng
chứng**, và một tầng quan trọng chưa có bằng chứng nào.

---

## 1. Tôi đã tự kiểm gì (không lấy digest làm bằng chứng)

| Điều tác giả khai | Tôi kiểm | Kết quả |
|---|---|---|
| Cửa trước Go 404 `/internal` trước khi proxy | đọc `dispatch.go` | **Đúng** — `isInternal()` phủ cả `/internal` lẫn `/internal/*`, comment ghi rõ core công không bao giờ proxy nó |
| Token brain fail-closed | đọc `internal_token.py` | **Đúng** — có `InternalTokenMissing`, giá trị được strip nên newline thừa không thành trạng thái thứ hai |
| Manifest 144 PORTED / 7 PY / 5 DEFERRED, `owner` toàn `python` | đọc `routes.json` | **Đúng** — 7 hàng PY còn lại đúng là W9 (4 sessions + 3 auth), không hàng nào `owner: go` |
| «Nhiều package domain WAI chưa có test» | đếm tệp | **Nặng hơn báo cáo nói**: 10/11 package có **0** tệp test (chỉ `chatintent` có 1) |

---

## 2. Hai chỗ lệch bàn giao — phán quyết

### 2.1 Brain sau `BrainDoor`, không vào bảng route / OpenAPI / manifest — **CHẤP NHẬN**

Bàn giao §4.4 của tôi nói phải thêm hàng manifest trong cùng PR. **Tôi sai ở điểm đó**, và cách tác giả làm tốt hơn:

- ADR-0029 vốn đã đòi brain **loại khỏi OpenAPI** và **chỉ với tới được trên mạng backend**. Đưa nó lên bảng route
  công là tự mâu thuẫn với chính ADR.
- `TestMatchesStarletteGoldens` đòi golden FULL cho mọi hàng manifest và `len(app.routes) == len(manifest)`. Brain
  trên bảng công phá cả hai bất biến — tức bàn giao của tôi sẽ đẩy người làm vào ngõ cụt.

Chấp nhận, và **sửa bàn giao §4.4** cho người sau. Điều kiện giữ nguyên: `/internal` phải 404 ở cửa trước (đã kiểm),
token fail-closed (đã kiểm), và idempotency Python bỏ qua `/internal` (tác giả khai, có test).

### 2.2 Port HTTP W7 khi chưa có stub Valhalla — **CHẤP NHẬN CÓ ĐIỀU KIỆN**

Lập luận của tác giả đúng: khi `parity_stacks.sh` không đặt `MOBILE_VALHALLA_URL`, **chính Python** trả
`status: unavailable`, nên Go trả y hệt là trung thành với HEAD chứ không phải che giấu. Kịch bản không bị sửa để
kỳ vọng khác đi. Lo của tôi ở bàn giao §4.1 là «đừng gom bằng chứng im lặng bỏ qua nửa routed» — ở đây bằng chứng
không im lặng, nó được ghi thẳng ra.

Điều kiện: nửa routed (`_route`, `schedule`, `suggest_order`, `savings`, `feasible`, `segments`, `late_fixed_stop`)
**vẫn là chưa chứng minh** cho tới khi stub lên, và phải ghi vào QUEUE + thẻ route, không chỉ nằm trong báo cáo này.

---

## 3. Blocker (loại «vi phạm spec/cổng» theo charter)

### B1 — Nhãn `PORTED` đang hứa nhiều hơn bằng chứng

Trong repo này, 108 hàng `PORTED` trước đó **đều** đi kèm một lượt `gate.sh parity` đầy đủ: canary đỏ hết kiểu phá,
probe, và ít nhất hai đột biến do người gộp tự nghĩ. Ba mươi sáu hàng mới lật sang `PORTED` chỉ với unit test trên
**cây bẩn**. Người đọc manifest sau này không còn phân biệt được hàng nào có parity, hàng nào chỉ có handler biên
dịch được.

Gỡ chặn bằng **một trong hai**, tuỳ cái nào rẻ hơn với tác giả:
- chạy `gate.sh parity` trên SHA sạch cho 36 hàng đó rồi mới lật; **hoặc**
- giữ chúng ở một nhãn riêng (`CARDED` đã có trong thang ADR-0029, hoặc thêm `PORTED-UNPROVEN`) và ghi lý do vào
  `evidence`, để manifest không nói dối.

### B2 — Mười một package domain WAI không có bằng chứng vi phân

`album`, `catalog`, `companion`, `conversation`, `faces`, `messageedit`, `promptsafety`, `reel`, `stickers`,
`suggestion` có 0 tệp test và (theo §7.7) không golden cùng-hàm-Python. Đây là tầng chứa grounding, chấm điểm, phân
loại ý định — đúng chỗ một bản port lệch mà không ai thấy. Mọi sóng trước đều dựng `scripts/render_domain_w*_goldens.py`
và chạy oracle live hàng chục nghìn ca.

Gỡ chặn: một generator render ca **từ chính Python trong ảnh ghim** cho các package này, ca ghim commit lại, và một
lượt live có `-v` báo số ca + số sai khác. Không cần 50.000 ca; cần **có** một con số.

### B3 — Tầng repository mới chưa chạm Postgres thật

`messages.go`, `destinations.go`, `place_photos.go`, `outing_memories.go`, `SetMembershipRole` chưa qua oracle
SQLAlchemy. Sóng nào cũng ghim **thứ tự câu lệnh** ở tầng này; unit test không thay được.

---

## 4. Suggestion (không chặn gộp)

- Tách commit theo sóng như tác giả đề nghị (W7 HTTP / brain / WAI / healthz+DEFERRED) — diff nhỏ dễ soi hơn.
- `GET /healthz` PORTED thì nên có một ca parity tối thiểu; nó là route duy nhất cố ý không chạm DB.
- 5 hàng `DEFERRED` có `docs/migration/deferred-framework.md` — tốt; nhớ thêm dòng vào QUEUE để nó không mồ côi.

---

## 5. Thứ tự gộp

Việc này **không** đụng ba worktree của tôi, và nhánh cắt từ đúng HEAD chiến dịch, nên gộp được. Thứ tự:

1. W9 (domain + repository) và stub Valhalla của tôi lên nhánh chiến dịch trước — công đó có trước và đang dở.
2. Nhánh này rebase `--3way` lên trên, **không chép worktree** (`routes.go`, `routes.json`, `ports.go` chắc chắn đụng nhau).
3. Rồi mới chạy `gate.sh parity` một lượt cho toàn bộ, vì lúc đó Go phục vụ 144+ route và lượt cổng mới có nghĩa.

---

## 6. Cái review này KHÔNG chứng minh

Tôi đọc diff, kiểm bốn điều rủi ro nhất và đếm tệp test. Tôi **không** chạy `gate.sh parity`, **không** chạy đột
biến, **không** dựng stack. Nên review này nói được «nhãn sai ở đâu, bằng chứng thiếu ở đâu», và **không** nói được
«36 route này trả lời giống Python». Câu sau chỉ có cổng mới trả lời được, và nó chưa chạy.
