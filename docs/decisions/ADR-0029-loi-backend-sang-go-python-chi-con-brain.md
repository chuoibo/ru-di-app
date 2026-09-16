# ADR-0029 — Lõi backend chuyển sang Go từng router, Python chỉ còn "brain" AI

- Trạng thái: **Đã chấp nhận** — Lead duyệt kế hoạch ngày 2026-09-14
  (`/home/lakiet/.claude/plans/i-want-to-do-stateful-lamport.md`).
- Quyết định bởi: Lead (chuoibo).
- Hiện thực: Claude, theo sóng W0 → W10 → WAI → decommission (mục 2.3).
  Backend do Claude làm theo uỷ quyền ADR-0016 §2.3, mở rộng cho chiến dịch này; charter không đổi.
- Sửa: `docs/architecture/00-layout-va-so-huu.md` («FastAPI, Python 3.12+»),
  ADR-0011 quyết định 10 (mục 2.7), ADR-0010 §6.4 (mục 2.11).
- Không đổi: ba luật về tiền, ADR-0004, ADR-0014 (hợp đồng phiên), ADR-0015.

## 1. Bối cảnh

Backend là một FastAPI duy nhất (`services/api`): ~45k dòng app, ~77k dòng test, Postgres 16 + Alembic.
`main` tại `87653135` phục vụ **150 route** trong 33 router, cộng `/healthz` và `/static`. Quanh route là cả
một lớp cơ chế dùng chung: bảng `idempotency_keys`, rate limit trong bộ nhớ, commit trước response, auth
`prod`/`dev`, CORS, header riêng tư của trang khách.

Lead muốn: **mọi thứ AI giữ Python** (companion, đọc hoá đơn/ảnh chụp màn hình, gợi ý, reel, lý do và tìm
địa điểm, nhận diện mặt); **mọi thứ còn lại sang Go** (API cho mobile, CRUD, auth, sổ tiền, trang khách).
Và một luật nghiệm thu: mỗi API chỉ tính là xong khi tester chứng minh **Python trước và Go sau** giống nhau về
cơ chế, đầu ra và hành vi.

Hai sự thật làm việc này khó hơn một lần viết lại:

- **Không có mốc so sánh nào được ghi sẵn.** Không có fixture HTTP, không có snapshot OpenAPI.
  ~860 ca `tests/api` chạy trong tiến trình trên repository giả nên không bắn được vào một binary Go.
- **Dây nối đầy bẫy byte.** Float Python giữ `1.0`; thứ tự khoá theo khai báo pydantic; datetime kết thúc `Z`
  và chỉ có micro giây khi khác 0; câu chữ lỗi 422 của pydantic; fingerprint idempotency chuẩn hoá JSON kiểu
  Python; Jinja escape khác `html/template`; Pillow nén lại ảnh; Starlette khớp route theo thứ tự đăng ký.

Seam AI thì sạch: **không module AI nào chạm DB**. Mọi route AI cùng một hình: auth + đọc DB → gọi model →
grounding thuần → tối đa một lần chèn `ai_card`.

## 2. Quyết định

### 2.1 Phân loại 150 route và manifest sở hữu

| Loại | Số | Đi đâu |
|---|---|---|
| CORE | 138 | Go |
| AI | 9 | Go giữ route (auth, DB, limiter), gọi brain Python cho bước model |
| MIXED | 3 | như AI: `POST /contexts/{id}/messages`, `GET /places`, `GET /places/{id}` |

Chín route AI: `POST …/messages/{mid}/expense-draft` · `POST /contexts/{id}/ai-turn` · `POST /places/search` ·
`POST /receipts/scan` · `POST /screenshots/scan` · `GET /contexts/{id}/suggestion` ·
`GET /contexts/{id}/contextual-suggestion` · `GET …/albums/{oid}/reel` · `POST …/photos/{pid}/face-boxes`.

Nguồn sự thật duy nhất về ai phục vụ route nào là **`services/core/ownership/routes.json`**: mỗi route một dòng
`{id, order, group, class, owner, python, limiter, state, evidence}`. Máy sinh nó từ `create_app().routes`;
`scripts/check_route_ownership.py` bắt nó khớp thứ tự đăng ký của Python, khớp tập route Go khai, và bắt các
route dùng chung một trạng thái trong bộ nhớ (limiter, cache) phải chuyển cùng nhau — ví dụ `GET /places` và
`GET /places/{place_id}` dùng chung `reason_writer`. (`POST /friends/lookup` **không** dùng chung limiter với
`POST /identity/person-id`: `friend_lookup_limit` được tách riêng có chủ ý.)
Lane nào thêm route hoặc biến môi trường mà không thêm dòng thì cổng đỏ.

### 2.2 Cổng trước, proxy, IP khách

- Binary Go `core` (`services/core`) nhận cổng công khai (8099 → 8000). URL gốc của mobile **không đổi**.
- Route Go sở hữu đi qua chuỗi Go: servererror → CORS → header `/g` → idempotency → handler. Mọi thứ khác
  (kể cả `/openapi.json`, `/docs`, `/static`, và mọi đường không khớp trọn) được reverse-proxy sang Python.
- **Python giữ đủ 150 route tới lúc decommission.** Nhờ vậy rollback, fallback và mốc so sánh đều chính xác.
- Rollback: `MOBILE_FORCE_PYTHON=all|<group>|METHOD /path`, đọc một lần lúc khởi động và ghi log; token lạ
  hoặc route đã `frozen` thì từ chối khởi động. Chỉ Lead lật máy đang chạy, mỗi lần lật một PR hoặc issue.
- IP khách: proxy **xoá** mọi `X-Forwarded-*`/`Forwarded` đến từ ngoài rồi đặt XFF bằng địa chỉ socket; giữ
  `Host` (307 dựng `Location` tuyệt đối từ đó). uvicorn chạy `--proxy-headers --forwarded-allow-ips='*'`, an
  toàn **chỉ vì** container `api` chỉ tới được từ `core` trên mạng `backend` — cổng docker khẳng định điều đó.
- Idempotency: middleware Go chỉ bọc route Go sở hữu; middleware Python bọc cái gì tới được nó. Hai bên cùng
  bảng, cùng scope, cùng fingerprint chuẩn + legacy, cùng nhịp thăm dò 20 → 100 ms trong 5 s, cùng luật nhả
  khoá khi không-2xx, cùng header replay. Khoá sống qua cutover và rollback theo cả hai chiều.
- pgx `MaxConns=15`, để Go + SQLAlchemy dưới mức mặc định 100 kết nối của Postgres.
- `contracts/openapi.json` được commit và gác lệch từ W0.

### 2.3 «Xong» là verdict từng route; một PR mỗi router

Mỗi route đi qua các trạng thái, ghi trong manifest, bằng chứng ở `docs/migration/`:

| Trạng thái | Bằng chứng để vào |
|---|---|
| PY | Dòng manifest sinh từ `main` |
| CARDED | Route card đủ (mục đích, auth, đầu vào, đầu ra theo từng nhánh, tác dụng phụ, mã lỗi, file:line Python, test đang phủ); kịch bản ghi từ **image Python ghim ở merge-base**, tự so K ≥ 3 lần rỗng; corpus 422; ma trận status × kịch bản và bảng × kịch bản đầy |
| PORTED | Mã Go đã merge sau manifest (owner vẫn `python`); `go test`, tầng Postgres Go, golden và vi sai domain xanh |
| PARITY-LOCAL | `make parity` 0 khác biệt trên các làn main/limiter/concurrency, corpus 422, replay chéo hai chiều; **mọi route còn proxy cũng 0 khác biệt**; tap chứng minh Go phục vụ; canary đỏ đủ |
| AGY-PASS | agy PASS; script tự tính lại số; mọi đột biến BREAKS đỏ, KEEPS sống; kiểm giả mạo sạch |
| RERUN-PASS | Claude chạy lại trong worktree tách rời sạch ở đúng SHA, tái hiện đột biến và khác biệt bằng tay |
| LIVE-GO | PR merge, owner lật `go`; e2e lát cắt dọc + e2e mobile + Maestro smoke qua `core`; tập rollback |
| FROZEN | Hết cửa sổ kép (mục 2.9) |
| PY-DELETED | Xoá ở decommission; bản ghi thành bộ hồi quy của Go |

**Route ứng viên.** Từ PORTED tới RERUN-PASS, mã Go đã merge nhưng owner vẫn `python`. Để có bằng chứng
PARITY-LOCAL trước khi lật owner, `core` phục vụ từ Go các route nêu trong `MOBILE_CORE_CANDIDATE_ROUTES` (id
route, tên group, hoặc `ported` là mọi hàng ở các trạng thái trên). Chỉ hàng ở các trạng thái đó mới được nêu;
nêu hàng khác thì `core` từ chối khởi động. `MOBILE_FORCE_PYTHON` vẫn thắng, và ứng viên không được tách state
trong bộ nhớ (limiter, cache) khỏi route Python còn phục vụ. Stack candidate của `parity` và `scripts/e2e_slice.sh`
đặt `ported`; compose để trống, nên máy dev và host thật vẫn do Python trả lời cho tới khi owner lật.

Một PR mỗi router group; PR chỉ merge khi **mọi** route trong đó ở RERUN-PASS, trừ khi Lead đánh dấu một route
DEFERRED (ở lại Python, manifest ghi lý do). Route trượt ở cổng nào thì quay về PORTED kèm phát hiện.

### 2.4 Hợp đồng wire bất biến

Port không được đổi byte nào ở biên HTTP: status, header (trừ `date`/`server`, xem mục 8), thân JSON/HTML sau
khi thay placeholder cho id/token/thời điểm **nhưng giữ nguyên định dạng**, và trạng thái DB sau **từng bước**.
Công cụ đo là bộ kiểm parity `parity/` (module Go riêng, hộp đen, không được import `services/core`):

- Giá trị ngẫu nhiên kịch bản không đặt tên được bind theo lần xuất hiện và vẫn giữ định dạng: uuid4 viết thường
  có gạch thành `<uuid#n>`, chuỗi đúng 64 hex thường (fingerprint, digest token) thành `<digest#n>`, chuỗi đúng 32
  hex thường thành `<hex32#n>` (`uploaded_images.storage_key = secrets.token_hex(16)`, không bao giờ lên wire nên
  chỉ làn database thấy), cursor phân trang (base64url không đệm của `thời điểm|uuid4`) thành
  `<b64u:<ts#r|dạng>|<uuid#n>>` với hai phần bên trong được bind như chữ thường, token đúng 43 ký tự base64url
  (`secrets.token_urlsafe(32)`, như token trong đường dẫn khách `/g/<token>`) thành `<token43#n>`. Cái phải khớp là hàng nào dùng chung một giá trị: viết hoa, độ dài khác, dùng lại một
  khoá hay đổi sang dạng khác vẫn đỏ. Một giá trị sai mà vẫn duy nhất thì bị che, như uuid4.
- Làn database: sau mỗi bước, hàng thêm, sửa, xoá của từng quan hệ được quan sát theo chữ đã che uuid, thời điểm,
  hex, cursor và token. Hàng giống nhau sau khi che được phân định bằng giá trị đã đánh số mà nó chứa (hai
  `guest_links` trỏ hai envelope, hai share của một người trên hai món), tính lại tới khi không còn hàng nào phân
  định thêm được, nên số thứ tự không do id ngẫu nhiên quyết. Hàng không gì phân biệt được giữ thứ tự cũ; hàng trỏ
  sai (hai share cùng một món) vẫn đỏ. Làn media (`--reference-media`/`--candidate-media`, chặng `parity` truyền
  `PARITY_REF_MEDIA`/`PARITY_CAND_MEDIA`) chụp kho ảnh của từng phía sau mỗi bước ngay sau DB và so file tạo, đổi,
  xoá (kể cả file tạm `.<key>.*.tmp` sót lại) cùng mode của nó và của các thư mục cha, mode thư mục còn lại sau khi
  xoá, và thư mục khoá có file đến rồi đi ngay trong bước (thư mục khoá «mới tạo» hay «hết rỗng» thì không so, vì
  khoá ngẫu nhiên nào chung `k[0:2]/k[2:4]` với khoá cũ là may rủi), cỡ và sha256 để nguyên chữ, còn khoá trong
  đường dẫn cùng `k[0:2]/k[2:4]` bind qua đúng `<hex32#n>` của hàng `uploaded_images` nêu khoá đó, nên file thiếu,
  thừa, sai chỗ, sai mode hay khác một byte đều đỏ dù wire và DB khớp.
- Mốc so sánh luôn là Python. File kịch bản **không có trường kết quả mong đợi**; `parity lint` từ chối các khoá
  `expect`, `status`, `body`, `assert`. Nhờ vậy agy viết được đầu vào mà không vi phạm ADR-0010 §6.1.
- Chống xanh giả: tự so K lần trên stack sạch; canary là proxy cố tình làm sai từng bẫy (`Z`→`+00:00`, `1.0`→`1`,
  đổi thứ tự khoá, 201→200, mất header replay, gzip, đi theo redirect, nuốt lệnh ghi, xoá file ảnh vừa lưu, route proxy mạo nhận là Go)
  và mọi chế độ phải đỏ; tap ghi ai thực sự phục vụ từng bước.
- Replay chéo: một bước có `via: python` đi tới Python của stack mà không qua `core` (ở candidate là qua cổng proxy
  của tap, nên tap vẫn ghi bước đó là tới Python; ở reference mọi bước vốn là Python). Nhờ vậy khoá
  `Idempotency-Key` do Python lưu được Go trả lời và ngược lại, trong cùng một kịch bản. Stack thiếu client tới
  Python mà kịch bản có bước như vậy là lỗi dựng, không bao giờ được gửi thay qua cửa trước.
- Làn limiter: kịch bản có `lane: limiter` cố ý tiêu một limiter trong bộ nhớ theo địa chỉ (`person_id_limit`,
  `friend_lookup_limit`). Lượt chính và canary bỏ qua chúng: hai lượt đó lặp corpus, nên 429 sẽ rơi vào lượt nào gặp
  cửa sổ trước. `parity run --lane limiter` chạy chúng cuối pha dev: mỗi kịch bản chờ tới ngay sau một biên
  `int(CLOCK_MONOTONIC / 60)` (Python `time.monotonic()` trong container và `limit.Monotonic()` của Go đọc cùng đồng
  hồ nhân trên máy này), chạy reference rồi candidate trong cùng cửa sổ, và là `INFRA` nếu chạy sang cửa sổ sau. Hai
  tiến trình đếm riêng, nên mỗi phía gặp limiter khi chưa đếm gì và bước bị 429 phải trùng nhau. Không chứng minh:
  nhiều replica, địa chỉ khách thật sau proxy (mọi stack ở đây gọi từ 127.0.0.1), lăn cửa sổ giữa chừng, xoá bảng khi
  quá 10 000 cặp.
- Đồng thời: bước `concurrent: N` gửi N bản của cùng một request cùng lúc (`{{burst}}` đánh số từng bản). Câu trả
  lời được xếp theo nội dung đã che id, digest và thời điểm (bản giống nhau giữ thứ tự số bản), không theo thứ tự
  về, nên bản nào thắng cuộc đua không làm lệch phép so. Mọi thời điểm cả loạt ghi (câu trả lời và hàng DB) chung
  một hạng, vì bản nào xong trước là lịch chạy chứ không phải hành vi; định dạng vẫn được so. Cả loạt so như một đa
  tập, kèm DB sau loạt.
  Loạt có thể tranh chấp ngay trên Python (`PUT /people/me/interests` với `uq_person_interests_person_tag`), nên
  reference chạy kịch bản có loạt K lần (`--burst-repeats`, mặc định 3) và candidate phải khớp trọn một lượt, wire
  và DB cùng lúc; bước khác nhau giữa các lượt reference được in `RACY`.
- Parity giữ nguyên cả lỗi của Python. Nó chứng minh «giống», không chứng minh «đúng». Lỗi tìm thấy trong lúc
  port thành phát hiện riêng, **không sửa trong PR port**.

**Ngoại lệ thứ hai, Lead duyệt ngày 2026-09-14: `MALFORMED-REQUEST-LINE`.** net/http của Go phân tích dòng
request trước mọi handler, nên cửa trước trả lời khác uvicorn/httptools cho những dòng request mà không client nào
của sản phẩm gửi. Đo bằng socket thô trên hai stack (22 dòng, Python trực tiếp so với Python qua `core`), 11 dòng
khác và được chấp nhận có tên:

| Nhóm | Ví dụ | Python (uvicorn) | Go (`core`) |
|---|---|---|---|
| escape hỏng trong path | `/%zz`, `/healthz%zz`, `/contexts/%zz`, `/healthz%` | định tuyến (404/401) | 400 `400 Bad Request` |
| byte non-ASCII thô trong target | `/h\xc3\xa9`, `/h\xe9` | 400 `Invalid HTTP request received.` | 404 |
| cả hai 400 nhưng khác thân/header | byte DEL, dấu cách trong query, target không bắt đầu bằng `/` | 400 `Invalid HTTP request received.` | 400 `400 Bad Request` |
| fragment | `/healthz#frag` | bỏ fragment → 200 | 404 |
| `OPTIONS *` | `OPTIONS *` | 404 JSON | 200 rỗng |

Mọi dòng request hợp lệ khác trong bộ đo đều bằng nhau (escape hỏng trong query, `//`, absolute-form, `%2F`,
`%0A`, 307 dấu gạch chéo cuối, method viết thường/lạ, `CONNECT`). Danh sách không được lớn lặng lẽ: lượt probe
(`parity probe`, chặng `parity`) đỏ khi một dòng NGOÀI danh sách bắt đầu khác, và đỏ khi một dòng TRONG danh sách
hết khác (danh sách cũ). Muốn thêm dòng là sửa ADR này.

**Ngoại lệ thứ ba, Lead duyệt ngày 2026-09-15: `RESPONSE-204-CONTENT-LENGTH`.** Khi một `Idempotency-Key` được
dùng lại cho route trả 204, middleware idempotency của Python phát lại kèm `content-length: 0`. net/http của Go
không bao giờ gửi `Content-Length` cho 204 (RFC 9110 §8.6 cấm), nên qua `core` header đó biến mất — cả khi Python
trả lời qua proxy lẫn khi route đã do Go phục vụ. Đo trên stack dev với
`DELETE /contexts/{context_id}/members/{person_id}`: lần đầu không phía nào có `content-length`; lần phát lại
Python gửi `content-length: 0` và `idempotency-replayed: true`, qua `core` chỉ còn `Idempotency-Replayed: true`.
Status, các header khác và thân rỗng bằng nhau. Bộ so chấp nhận **đúng** cặp đó: 204 ở cả hai phía, reference có
đúng một giá trị `0`, candidate không có header. Chiều ngược lại, giá trị khác `0`, header lặp hay status khác vẫn
là khác biệt. Kịch bản `w0/replay-204` phải cho thấy khác biệt này ở mọi lượt: `parity run` in số lần chấp nhận
và đỏ `STALE` khi kịch bản đã chạy mà hai phía không còn khác, để ngoại lệ không nằm lại khi nguyên nhân đã hết.

Header được so theo ngữ nghĩa HTTP: tên không phân biệt hoa thường, thứ tự giữa các tên khác nhau không được so
(net/http viết tên dạng chuẩn và tự sắp xếp); số lần lặp và thứ tự giá trị của cùng một tên vẫn phải khớp.

### 2.5 Ba luật tiền trong Go — không đổi luật

1. Số nguyên đồng: `money.VND` là `int64`; `moneylint` (go/analysis) cấm kiểu float, literal phân số,
   `big.Float`, `ParseFloat` trong gói tiền; mọi trường `*_vnd` lưu hoặc nhận vào phải là `money.VND`.
   Tổng dẫn xuất (tổng theo cặp, số dư, số tiền chuyển) là `*big.Int`, không bao giờ thu về `int64`: số tiền
   biên nhận không có trần, hai biên nhận ở trần `int64` đã cho tổng vượt `int64`, và `int` của Python vẫn trả
   đúng từng chữ số (đo ở W3, `GET /contexts/{context_id}/balances`). Ngoại lệ thứ hai (W4): số tiền request mà
   pydantic nhận không trần rồi Python so sánh, in lại hoặc gửi xuống database được đọc ở biên thành `*big.Int`
   chính xác — `items_total_vnd`/`line_total_vnd` của `POST /bills` (chi tiết 422 in trọn chữ số),
   `candidate_per_person_vnd` của ngân sách (response in lại), `amount_vnd` của confirm-receipt (so với biên nhận
   đã lưu, vượt BIGINT thì PostgreSQL từ chối 22003 như với psycopg). Số tiền vào allocator vẫn `money.VND` qua
   `allocator.Saturate`, vì allocator chỉ so với 0 và `MAX_AMOUNT_VND`. Hai ngoại lệ đều vẫn là số nguyên.
2. `Σ` phân bổ `=` tổng: allocator dùng `math/big.Rat`; 41 golden vector và 10 vector tất toán được Go **đọc tại
   chỗ**, không chép; fuzz vi sai Python ↔ Go gồm cả mã lỗi.
3. Số dư tính lại được từ sổ: đọc chéo — Go ghi Python đọc, Python ghi Go đọc — trên Postgres thật.

Domain Go thuần: `tools/boundary/domainpure_test.go` là bản Go của `test_import_boundary.py`, chỉ cho import
danh sách trắng (math/big, strings, time, …), cấm db/net/os/http; có canary phải đỏ.

### 2.6 Transaction, commit trước response, gọi brain trong transaction

Một transaction READ COMMITTED mở lười mỗi request, commit **trước khi ghi response** — như
`install_commit_before_response` hôm nay. Thứ tự khoá hàng (35 chỗ `FOR UPDATE`) port nguyên văn; log câu lệnh
Postgres được so theo từng route. Với `/chia-bill`, `/plan`, `@rudi`: chèn tin nhắn → gọi brain → chèn
`ai_card` → commit → trả lời, **trong cùng một transaction** như Python; lỗi truyền tải cho ra đúng thân
`intent_error`/`unavailable` như nhánh except của Python.

### 2.7 Python chỉ còn brain

Python kết thúc là dịch vụ nội bộ `/internal/brain/v1/*`: không có trong OpenAPI, chỉ tới được trên mạng
`backend`, gác bằng `X-Internal-Token` (token trống thì từ chối khởi động), **không có credential DB** (cổng
compose khẳng định). Lỗi chỉ trả mã, không trả câu prompt hay chữ của model.

- **Sửa ADR-0011 quyết định 10:** nhận diện mặt chạy ở tiến trình brain **cùng máy**; byte ảnh do Go đọc từ
  storage và không rời máy. Mọi ràng buộc khác của quyết định 10 giữ nguyên.
- ADR-0025: bộ dựng reel ở cùng máy, dùng chung volume media — đúng như ADR đó yêu cầu.
- ADR-0024: việc sau response vào `internal/afterresponse`, giới hạn đúng các việc ADR đó liệt kê.

### 2.8 Ảnh upload làm bằng Go, giống từng byte với Pillow

Ban đầu mục này duyệt trước một ngoại lệ parity cảm nhận (`IMG-REENCODE`: JPEG lệch byte nhưng SSIM kênh sáng
≥ 0,98 và sai lệch trung bình ≤ 2/255). Đo ở W6: `UploadedImageResponse.byte_size` và cột `uploaded_images.byte_size`
là độ dài đầu ra Pillow, nên lệch byte làm thân JSON và hàng DB khác nhau — ngoại lệ đó không giữ được chính yêu cầu
«thân JSON bằng nhau». Bản port Go thuần (`internal/media/sanitize`, `CGO_ENABLED=0`) đạt giống từng byte, nên
`IMG-REENCODE` không được dùng:

- Phải bằng nhau tuyệt đối: status, thân JSON (kể cả `byte_size`, `width`, `height`, `content_type`), mã và detail từ
  chối, byte file đã lưu, byte GET của file đó.
- Đã chứng minh bằng oracle chạy `sanitize_image` thật trong image ghim trên corpus sinh lúc test (không commit byte
  ảnh): JPEG (baseline, progressive, MPO, CMYK), PNG, WebP, GIF, BMP/DIB, PPM, hướng EXIF/XMP, và các plugin đơn giản
  (TGA, SGI, SUN, IM, PSD, PCX, XPM, XBM, QOI, …).
- Phụ thuộc phiên bản: Pillow 12.2.0 với libjpeg-turbo 3.1.4.1, libwebp 1.6.0 và zlib 1.3.1 (libz của Debian mà
  `_imaging` liên kết, dù `features` ghi zlib-ng). Đổi Pillow, image nền hay gói zlib1g phải chạy lại
  `go test -tags oracle ./internal/media/sanitize/...` trước khi merge.
- Định dạng Pillow mở được mà Go chưa port (AVIF, JPEG2000, TIFF nén, khung ICO/CUR/ICNS, DDS/FTEX nén BCn, FLI, PCD,
  BLP1/IPTC có JPEG, LAB, …) trả `*UnsupportedError`; chính sách cho chúng (trả 415 `not_an_image` như lệch có ghi,
  port codec, hay giữ route ảnh ở Python) là câu hỏi mở ở mục 8, chờ Lead.
- GET của file đã lưu vẫn phải giống từng byte.

### 2.9 Đóng băng và cửa sổ kép

- Khi một group bắt đầu ghi mốc parity, route của nó được ghi vào `docs/codex/QUEUE.md`.
- `scripts/check_go_owned_python_touch.py` dựng đồ thị gọi AST (route → service → repository → domain) và làm
  đỏ mọi diff Python chạm được tới route Go đang sở hữu, trừ khi PR mang kèm thay đổi Go với bằng chứng mới hoặc
  lật route về Python.
- Sau khi lật: **14 ngày** (hoặc tới lần lật group kế tiếp, lấy cái muộn hơn) tính năng mới trên route đó vào cả
  hai ngôn ngữ; hết cửa sổ thì `python: frozen`, việc mới chỉ vào Go.
- Bằng chứng phải mới hơn 24 giờ so với `main` lúc merge; `gate_merge.sh` chạy lại
  `ownership go-schema go-postgres parity` trên kết quả gộp.
- Alembic vẫn là chủ schema duy nhất; Go không chạy DDL. Migration mới của lane khác kiểm lại kiểu câu truy vấn
  Go qua `schema.sql` render từ `alembic upgrade head --sql`.

### 2.10 Luật xoá test

Không xoá test Python nào mà không có thay thế. `services/core/testmap/<group>.json` ánh xạ từng test id sang
test Go hoặc golden; cổng từ chối xoá khi thiếu ánh xạ. `tests/postgres` chuyển sang Go với `-tags postgres`;
golden chuyển sang `contracts/golden/` không sửa nội dung.

### 2.11 Sửa ADR-0010 §6.4 cho agy

§6.4 cấm `--dangerously-skip-permissions`, nhưng `scripts/agent_supervisor.py` đang truyền cờ đó cho agy, và
thiếu cờ thì agy headless từ chối tool `command`. Việc kiểm parity của agy **không bắt đầu** cho tới khi một
trong hai đường sau chạy được, thử theo thứ tự:

1. allow-rule tiền tố hẹp đúng cho `./parity/bin/parity`, `timeout`, `git -C /tmp/agy-parity-*`,
   `curl -s …/healthz`, `printf` — Lead áp theo §9.2 (agent không được sửa allow-list của agent khác);
2. chạy agy trong container chỉ mount worktree kiểm và thư mục báo cáo.

§6.4 **không nới**: cả hai đường đều hẹp hơn cờ bị cấm.

## 3. Hệ quả

- Một ngôn ngữ mới trong repo: toolchain Go 1.23.4 ghim trong `go.mod`, image `golang`/`distroless` ghim digest.
- Hai triển khai cùng một cơ chế (idempotency, CORS, limiter) sống song song tới decommission.
- `scripts/gate.sh` thêm các chặng `ownership go-vet go-test go-schema go-postgres parity`; chặng bị bỏ qua là
  hỏng khi `--strict`.
- Các cổng hợp đồng (`check_api_contract.py`, `check_server_routes*.py`, `check_actor_headers.py`,
  `check_cors_contract.py`) giữ nguyên tới decommission, rồi đọc `contracts/openapi.json` do `core openapi` phát.
- Chiến dịch dài: 150 route, 12 sóng, ~25 PR group, mỗi PR một lượt agy và một lượt chạy lại sạch.

## 4. Cái này KHÔNG chứng minh

- Hành vi trên đầu vào ngoài corpus kịch bản; các nhánh coverage liệt kê là chưa phủ.
- Rằng Python đúng — parity giữ nguyên lỗi của nó.
- Hình dạng dữ liệu production (hàng cũ, fingerprint legacy ngoài fixture, Unicode lạ trong tên thật).
- Tải, query plan, tranh chấp khoá dưới lưu lượng thật; limiter khi chạy nhiều replica.
- Tần suất của kết cục tranh chấp: làn đồng thời chỉ đòi candidate ra một kết cục mà Python đã ra trong K lượt. K lượt
  có thể bỏ sót kết cục hiếm của Python (đỏ giả, chạy lại) và không phân biệt được Go thường thua một cuộc đua mà
  Python thường thắng.
- Đường hạnh phúc thật của Google sign-in và SMS gateway (stack kiểm dùng stub và cửa Google đóng).
- Bất cứ điều gì về hành vi Gemini.
- Chất lượng thị giác của ảnh ngoài hai chỉ số ở mục 2.8.

## 5. Phương án đã bác

- **Viết lại một lần rồi thay (big bang):** không có điểm rollback, không có mốc so sánh từng route.
- **Để Python làm cửa trước, chuyển tiếp route đã port sang Go:** mọi request vẫn trả giá Python, và lúc
  decommission phải đảo topology lần hai.
- **chi / `net/http` ServeMux:** khớp theo độ cụ thể của đường, không theo thứ tự đăng ký; ServeMux còn 301
  thay vì 307. Thay bằng router có thứ tự mô phỏng Starlette.
- **`html/template` cho trang khách:** escape theo ngữ cảnh (`+` thành `&#43;`), không khớp byte Jinja. Thay
  bằng `text/template` + lượt duyệt cây bọc mọi action bằng `pyescape` (luật markupsafe).
- **Sidecar Pillow ở Python cho ảnh:** giữ một phụ thuộc không-AI ở Python mãi mãi; Lead chọn Go-native.
- **Giữ nguyên route AI trong Python kèm truy cập DB:** mọi thay đổi schema phải làm hai lần mãi mãi; Lead chọn
  brain không DB.
- **ORM cho Go:** giấu thứ tự flush và khoá — đúng thứ phải so. Thay bằng pgx + sqlc typecheck trên `schema.sql`.

## 6. Cách kiểm chứng

Mỗi group, những gì đã chạy được:

```bash
make parity                              # = scripts/gate.sh parity, dev rồi prod, mỗi chế độ một cặp stack:
                                         #   lượt chính có ảnh chụp DB sau từng bước và tap, rồi canary (mọi
                                         #   chế độ bẫy ĐỎ, identity XANH; chạy sau vì chế độ hỏng làm hai DB
                                         #   lệch nhau), corpus 422 (scenarios/generated), bước replay
                                         #   chéo `via: python`, bước đồng thời `concurrent: N`, probe dòng request (chỉ dev)
python3 scripts/render_parity_422_scenarios.py   # sinh lại corpus 422 từ model pydantic thật
cd services/core && go vet ./... && go test ./...
scripts/go_postgres_tier.sh              # skip là hỏng
```

Chưa có (đã hoạch định, chưa dựng): lọc theo group (`GROUP=`), `STRICT=1`, làn limiter, fuzz vi
sai domain (`parity-domain`), `scripts/agy_parity_qc.sh`. Tới khi có, bằng chứng từng route ghi rõ làn nào đã
chạy.

Rồi người merge chạy lại trong worktree tách rời sạch. Sau mỗi merge: `scripts/gate.sh --strict`,
`scripts/e2e_slice.sh` và e2e mobile qua `core`, pytest gốc so số đếm với `main`, Maestro smoke.

Thoát W0 (không route nào do Go sở hữu): `gate.sh --strict` đạt với e2e đi qua `core` ở chế độ prod;
`make up`/`smoke`/`demo` chạy qua 8099; lượt «trong suốt» (Python trực tiếp so với Python qua `core`) 0 khác
biệt; canary đỏ đủ; `MOBILE_FORCE_PYTHON=all` khởi động được, token lạ bị từ chối.

## 7. Đường lùi

- Một route hoặc group: `MOBILE_FORCE_PYTHON` rồi khởi động lại `core`. Idempotency cùng bảng nên replay vẫn
  đúng theo cả hai chiều (có kịch bản replay chéo chứng minh).
- Cả chiến dịch trước decommission: `MOBILE_FORCE_PYTHON=all` — Python vẫn giữ đủ route.
- Sau khi một route `frozen`, rollback cho route đó bị từ chối có chủ ý; muốn lùi phải sửa ADR này.

## 8. Câu hỏi mở cho Lead

1. **Chủ schema sau decommission:** giữ image migrate một lần với Alembic + models (công cụ, không phải dịch vụ),
   hay chuyển sang migration SQL (goose/atlas) từ một baseline đóng băng. Cần ADR riêng.
2. **Header `server: uvicorn`:** Go có tiếp tục phát chuỗi đó để khớp không, hay đánh dấu là header thay đổi được.
3. **Định dạng ảnh lệch:** danh sách định dạng Pillow nhận mà Go không giải mã được, đo ở W6, cần Lead duyệt
   từng dòng.
