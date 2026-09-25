# 25 route WAI chuyển hẳn sang Go (LIVE-GO)

Ngày 2026-09-22. Thực hiện: Claude, theo uỷ quyền ADR-0016/ADR-0029 và quy tắc
toàn repo ADR-0031.

Đợt này khép lại nhóm `PORTED-UNPROVEN`: 13 route thuần backend và 12 route
AI/lai. Sau khi lật, **151/156 route thuộc về Go**; 5 route còn lại là của
FastAPI (`/openapi.json`, `/docs`, `/docs/oauth2-redirect`, `/redoc`,
`MOUNT /static`).

## Vì sao 25 hàng này KHÔNG lật cùng 126 hàng hôm qua

Vì chúng chưa có kịch bản parity nào. Đo trước khi viết: trong 371 kịch bản
có sẵn, **12/13 route thuần backend có đúng 0 kịch bản** khai đích danh.

Phép đo ấy cũng trả về một con số không ai đi tìm, và nó nói về đợt trước:
**0/126 route đã LIVE-GO thiếu kịch bản**. Tức đợt lật hôm qua không phải lật
mù. Máy đo đầu tiên của tôi hỏng — nó trả 0 cho cả `POST /expenses`, route
chắc chắn đã chứng minh — nên con số này chỉ đáng tin sau khi dựng ca đối chứng.

Việc giữ 25 hàng lại hoá ra là quyết định đúng: viết kịch bản cho chúng tìm ra
**ba lệch Python↔Go thật**, và cả ba đều nằm trong chính 25 route ấy.

## Ba lệch tìm được, và vì sao không thứ gì ngoài parity thấy chúng

### 1. `Null{}` không phải nil — bẫy nil-CÓ-KIỂU của Go

Không có khoá Gemini thì brain trả `{"card": null}`. JSON `null` giải mã thành
`pyjson.Null{}` — một **struct** — nên `card == nil` là **false**.

| | |
|---|---|
| Python | `if raw is None: return _silent("unavailable")` — dừng ngay |
| Go | `if card == nil { ... }` không bắt được, rơi xuống `Ground()` → `"ungrounded"` |

Comment sẵn có trong `value.go` góp phần dựng bẫy: «A nil Value is treated as
Null by the encoders» đúng lúc **ghi**, nhưng dễ đọc ngược thành lúc **đọc**.

Lane chat sửa lỗi này trước tôi (`db52a39d`) và tìm được **năm** chốt, trong
khi tôi chỉ thấy **ba**. Hai chốt tôi bỏ sót: `picksVal` (cuốn phim) và
`stops`. Bản của họ ở `main`, tôi lấy bản ấy và bỏ bản trùng của mình.

### 2 và 3. Thứ tự khoá jsonb — `group_fit` và `reviews`

```
Python: {"min_people":2,"max_people":8,"relation":"Bạn thân, cặp đôi"}
Go:     {"relation":"Bạn thân, cặp đôi","max_people":8,"min_people":2}
```

**Độ dài thân bằng nhau từng byte: 5694 với 5694**, lệch từ byte 489. Với
`reviews` là 1051 với 1051.

Cột là `jsonb`, và jsonb **không giữ thứ tự ghi vào** — Postgres sắp khoá theo
độ dài rồi theo byte, nên `relation` (8 ký tự) ra trước `max_people`/
`min_people` (10). Python không bao giờ thấy thứ tự đó vì hàng đi qua
`class GroupFit(BaseModel)` ở `routes/places.py:117`, mà pydantic ghi theo thứ
tự khai báo trường. Go dội thẳng cột ra wire.

Đo thẳng mới ra, sau hai lần tôi đoán sai:

```
DB (jsonb):     {"relation": "Bạn thân", "max_people": 6, "min_people": 2}
Python phục vụ: {"min_people": 2, "max_people": 6, "relation": "Bạn thân"}
```

**Cùng JSON, cùng độ dài, khác wire.** Mọi bộ test so JSON sẽ xanh mãi mãi.

### Tập rủi ro đã đóng

Điều kiện để sinh lỗi này cần **hai** vế cùng lúc:

1. Go dội nguyên `json.RawMessage` ra wire, **và**
2. phía Python đi qua một `BaseModel` **lồng** có trường khai sẵn

Nếu Python khai `dict` trơn thì pydantic cũng dội nguyên xi — hai bên cùng trả
thứ tự jsonb và Go dội theo là **đúng**.

`models.py` có đúng 7 cột jsonb chứa khoá. Đã quét cả 7:

| Cột | Vế (a) | Vế (b) | Kết luận |
|---|---|---|---|
| `group_fit` | có | có | **đã sửa** |
| `reviews` | có | có | **đã sửa** |
| `card` (Message) | có | **không** — `dict` trơn | đúng |
| `content`, `nguon` (PairPaperVersion) | **không** — dựng tay | có | đúng |
| `notify_prefs`, `event_data` | — | — | không ra wire |

Đó cũng là lý do W8 `pair_papers` qua parity sạch từ trước: `wirePaperVersion`
dựng từng trường theo đúng thứ tự model khai. Nó đúng **một cách tình cờ**, nên
đã thêm comment cảnh báo tại chỗ: rút gọn nó thành dội thẳng sẽ mở lại đúng lỗ
này, ngắn hơn, cùng JSON, khác wire.

## Bằng chứng

13 kịch bản trong `parity/scenarios/wai/`, phủ **25/25** route (và 126/126
route đã LIVE-GO cũng có kịch bản).

Cổng parity đầy đủ, bốn pha, mỗi pha một cặp stack cô lập:

| Pha | Kết quả |
|---|---|
| dev main + probe | **352 kịch bản · 10.777 bước · 0 khác biệt** · tap `answered_in_core=10211`, `served_routes=151`, `unserved=0` · probe 22 ca, 10 lệch đều là ngoại lệ ADR-0029 §2.4, `unexpected=0 stale=0` |
| dev canary | `identity equal, every exercised damage caught` |
| dev limiter | 9 · 209 · 0 |
| prod main + canary | 23 · 604 · 0 · `every exercised damage caught` |

Cộng: `make e2e` 11/11, `go build`/`go vet`/`gofmt` sạch, repo guard đạt.

Hai lần chạy hỏng dọc đường, cả hai là hạ tầng chứ không phải lệch, và cả hai
được phân loại đúng ngay lúc đọc:

- Ba pha chết vì `auth.docker.io` timeout khi BuildKit nạp frontend. Kiểm lại
  ngay sau đó: HTTP 200, 1,25 s — mạng chập nhất thời.
- Pha canary chết vì **stack reference biến mất giữa lượt** (`connection
  refused`). Bằng chứng nó không phải lỗ hổng bộ so: canary của pha prod chạy
  trọn ở cùng lượt và kết luận `every exercised damage caught`. Chạy lại sau
  khi tắt máy ảo Android (RAM khả dụng 8,8 → 12 GiB) thì xanh.

Nhãn ba mức trong script — `0` xanh, `1` đỏ thật, `2` không dựng nổi stack —
là thứ giữ cho hai sự cố ấy không bị đọc thành lệch. Gộp chúng vào «đỏ» là cách
đi sửa một lỗi không tồn tại.

## Cái này KHÔNG chứng minh

Ghi ở đầu mỗi file kịch bản, để một bảng toàn xanh không ngụ ý quá phần nó đo:

- **Nhánh mô hình thật trả lời.** Stack parity không có khoá Gemini nên mọi
  đường AI rơi vào nhánh không-có-mô-hình. Nhánh ấy tất định hai phía nên so
  được từng byte; nhánh có mô hình thật cần khoá thật và một câu trả lời vốn
  không tất định.
- **Khả năng dò mặt.** Harness sinh ảnh từ seed, tức nhiễu màu không có mặt
  người. Bộ dò không thấy gì ở **cả hai** phía, nên sự khớp nhau ấy nói về câu
  trả lời rỗng.
- **Thân ảnh địa điểm.** Hàng trong `place_photos` chỉ đến từ trình nhập OSM,
  nên `/places/{id}/photos/{photo_id}` chỉ đi được tới 404 và 422. Bytes,
  `content-type` và header `public, max-age=86400` chưa được parity chứng minh.
- **Nhánh 429 của các route AI.** Cần cửa sổ limiter mới cho từng kịch bản,
  tức làn `lane: limiter`, không phải làn chính.
- **Tiêu chí phụ theo `id` của `/destinations`.** Cần hai điểm đến trùng toạ
  độ, mà seed không có và không route nào tạo điểm đến. Canh bằng test Go thay
  vì parity.

## Còn lại gì

1. **5 route framework.** Cần viết lệnh con `core openapi`, nhúng 3 file
   `/static` vào binary, và gỡ điều kiện `class == "framework"` trong
   `manifest.go` vốn đang cấm mọi route framework về Go.
2. **Chuyển nguồn manifest từ Python sang Go.** `check_route_ownership.py`
   hiện gọi `render_route_manifest.py --check`, tức dựng manifest **từ chính
   app Python**. Chưa làm bước này thì không xoá được dòng Python nào.
3. **Xoá Python** — khoảng 48.800 dòng trên tổng 50.421 của `services/api/app`,
   giữ lại ~1.600 dòng AI. Rồi tới 509 file / 129.513 dòng test Python, phần
   đắt nhất và dễ tự lừa nhất.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này đọc đồng ý của sổ đôi (qua `_pair_chat_consent`/gu nhóm hoặc trực tiếp): kết quả chỉ đổi khi một pair mang hai lời đề nghị cùng bậc song song, khi đó bậc KHÔNG còn được tính là đã bật (trước là bật nhầm) (với `live-go-25-route-wai.md`: áp cho các route AI/gợi ý đọc gu nhóm của pair; các route khác trong tệp chỉ bị chạm theo tên). Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `GET /contexts/{context_id}/contextual-suggestion`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /contexts/{context_id}/suggestion`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /places`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /places/{place_id}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /places/{place_id}/photos`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `POST /contexts/{context_id}/ai-turn`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `POST /contexts/{context_id}/messages`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `POST /places/search`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `GET /contexts/{context_id}/contextual-suggestion`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /contexts/{context_id}/suggestion`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /places`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /places/{place_id}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `GET /places/{place_id}/photos`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `POST /contexts/{context_id}/ai-turn`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `POST /contexts/{context_id}/messages`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
- `POST /places/search`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 (c) — xoá `ai-turn` và các nhánh AI tự kích hoạt của `POST /messages` (ADR-0036 §2.1, §3b)

Một đường gọi AI duy nhất là hàng đợi lời gọi (`POST /contexts/{id}/ai-invocations`). Diff này xoá, trong cùng một commit ở cả Go lẫn Python cùng manifest: route `POST /contexts/{context_id}/ai-turn` (Go `takeCompanionTurnRoute`/`takeCompanionTurn`, Python `take_companion_turn`), nhánh `/plan`·`@Rủ Đi` (companion) và nhánh `/chia-bill` của `actOnMessageIntent`/`act_on_message_intent`, nhịp `PlanTurn`/`plan_turn` cùng route brain `companion-plan`, hai cửa sổ `companion_turn_limiter`/`message_intent_limiter`. Từ đây hàng `ai-turn` không còn trong manifest; cửa trước Go không có route nên proxy sang Python và Python trả 404.

- `POST /contexts/{context_id}/messages`: hành vi ĐỔI có chủ ý. Chữ «/plan …», «@Rủ Đi …», «/chia-bill …» là tin nhắn thường: `intent` null, `intent_error` null, không gọi mô hình, không ghi thẻ. Thân trả lời bỏ hai trường `companion` và `expense_card`; `intent` chỉ còn `"vote"`, `intent_error` chỉ còn `"vote_malformed"`. Nhánh `/vote` không đổi một byte (kịch bản `wai/messages-flow.yaml`). Hai bên đổi cùng lúc, `PostedMessageResponse` và `clonePosted` cùng thứ tự trường `intent, vote, intent_error`.
- `POST /contexts/{context_id}/messages/{message_id}/expense-draft`: không đổi. Kịch bản `wai/ai-turn-and-expense-draft.yaml` giữ nửa này, bỏ sáu bước `ai-turn`.
