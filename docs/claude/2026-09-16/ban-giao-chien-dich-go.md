# Bàn giao chiến dịch chuyển lõi backend sang Go (ADR-0029)

**Ngày:** 2026-09-16 · **Nhánh:** `claude/p0-w-go0-nen-mong-cong-truoc` · **Cây làm việc:** `/home/lakiet/wt-go0`
**HEAD lúc bàn giao:** `cff1ba79` · **118 commit** trên nhánh, chưa push, chưa mở PR.

Tài liệu này viết để người kế tiếp — người, Claude, hay agy — làm tiếp mà không phải đoán. Ai đọc xong mà vẫn phải
hỏi «giờ làm gì» thì tài liệu này hỏng, hãy sửa nó.

---

## 1. Đang ở đâu

| | |
|---|---|
| Route đã PORTED | **108/156 (69%)** |
| Route đã cắt quyền sang Go (`owner: go`) | **0** |
| Còn lại | outings 11 · messages 9 · places 7 · sessions 4 · auth 3 · albums 3 · suggestions 2 · receipts/screenshots/faces 3 · framework/static/main 6 |

**PORTED nghĩa là gì ở đây:** Go phục vụ route đó **với tư cách candidate** sau cổng trước, Python vẫn là mốc so
sánh và vẫn là chủ sở hữu. Lùi lại chỉ là đổi một biến môi trường. Chưa route nào ở trạng thái LIVE-GO.

**Nguồn sự thật:** `services/core/ownership/routes.json` (manifest, 156 hàng) · `docs/decisions/ADR-0029-*.md`
(luật chiến dịch) · `docs/decisions/ADR-0030-*.md` (lane Codex đóng, định nghĩa cổng) · `docs/codex/QUEUE.md`
(nhật ký sóng, việc còn mở) · `docs/migration/routes/<nhóm>/` (thẻ từng route).

Số đo của mỗi sóng nằm **trong commit message** của sóng đó, không nằm trong log. Đó là cố ý: phiên làm việc bị
khởi động lại hai lần và scratchpad bị xoá sạch cả hai lần, còn git thì không mất gì. `git log -1 --format=%B <sha>`
trả lại đủ số.

---

## 2. Cổng là gì (ADR-0030)

Lane Codex đã đóng ngày 16/09, nên **không còn review chéo**. Một route được coi là đạt khi và chỉ khi:

1. Cổng parity đầy đủ chạy lại trong **cây sạch tại đúng SHA** — không phải cây của agent viết code;
2. Canary: identity xanh, **mọi kiểu phá được áp dụng đều bị bắt**;
3. Probe: mọi lệch nằm trong danh sách ngoại lệ đã ghi (hiện 11/22 ca, đều đã ghi);
4. **Ít nhất hai đột biến do chính người gộp nghĩ ra**, khác đột biến của agent, mỗi đột biến đỏ ở đúng bước đã dự
   đoán;
5. Số đo viết thẳng vào commit message.

Ba luật về đột biến, trả giá mới có:
- **Phán quyết đỏ/sống lấy từ SỐ CA ĐỎ, không bao giờ từ mã thoát.** Bộ đo thoát khác 0 vì lý do hạ tầng là chuyện
  thường; một lần tier tự từ chối cả lượt (thiếu test sentinel trong bộ lọc `-run`) làm một đột biến **tương đương**
  trông như đã bị bắt.
- **Kiểm đột biến rơi vào dòng CODE**, không phải comment. Đã có một lần gài nhầm vào chú thích.
- **«Có đường tới» không đủ, phải «quan sát được»:** hỏi giá trị nó đổi có lọt vào thứ đang được so không. Oracle
  repository ghi **văn bản SQL**, không ghi tham số — nên một đột biến chỉ đổi giá trị tham số mà không đổi trạng
  thái cuối là tương đương, dù dòng code có chạy.

**Định nghĩa đột biến phải được commit** từ nay (overlay JSON + script chạy, nằm im ngoài build). 15 đột biến của
làn kho ảnh đã bị vứt đi và lần kiểm sau phải dựng lại từ tên gọi — dựng lại thì không còn là cùng một phép đo.

---

## 3. Đang dở dang ngay lúc này

Ba agent đang chạy, mỗi agent một worktree tách rời, **chưa cái nào commit**:

| Worktree | Việc | Trạng thái lúc bàn giao |
|---|---|---|
| `~/wt-go0-w9-repo` | repository cho 7 route auth/sessions | 6 tệp đã có: `sessions.go` sửa, thêm `account_identities.go`, `account_sessions.go`, `named_invite_secret.go`, `otp_challenges.go`, `scripts/render_w9_repo_oracle.py`. Còn nợ: tier đầy đủ với `-v`, và ≥2 đột biến |
| `~/wt-go0-w9-domain` | domain auth/sessions (`app/domain/otp.py` + phần thuần) | Chưa ra tệp nào |
| `~/wt-go0-valhalla-stub` | **stub định tuyến tất định cho stack parity** | Chưa ra tệp nào |

Nếu chúng không quay lại: đọc `git status` trong từng worktree trước, rồi giao lại đúng phần còn thiếu. Đừng xoá
worktree khi chưa đối chiếu — công chưa commit nằm trong đó.

---

## 4. Việc kế tiếp, theo đúng thứ tự

### 4.1 Stub định tuyến — CHẶN sóng W7, làm trước
`scripts/parity_stacks.sh` **không** đặt `MOBILE_VALHALLA_URL` lẫn `MOBILE_ROUTING_GRAPH_VERSION`, nên
`configured_provider()` luôn trả `None` và **mọi** preview lịch trình trong **mọi** lượt parity dừng ở
`status: "unavailable"`. Hệ quả: `_route`, `schedule`, `suggest_order`, `savings`, `feasible`, `segments`,
`late_fixed_stop` **không có một dòng bằng chứng parity nào** — chúng chỉ được chứng minh như hàm thuần trong oracle
domain (6.000 ca `preview_itinerary`, 20.000 ca `journey`).

Yêu cầu của stub: hai stack nhận **câu trả lời giống nhau từng byte cho cùng một yêu cầu**, ổn định qua các lượt,
suy ra từ chính yêu cầu chứ không từ đồng hồ hay nguồn ngẫu nhiên (hai stack là hai container, phải khớp nhau mà
không chia sẻ trạng thái). Phải chạm được các nhánh thú vị: một chặng không có đường, một `status` khác 0, và một
shape giải ra điểm thật. Đọc `scripts/prepare_journey_routing.py` trước, dùng lại thứ hợp chứ đừng dựng cơ chế song song.

**Không port route W7 trước khi có stub.** Port trước là tự nguyện cắt phần logic phức tạp nhất của sóng sang Go
trong bóng tối, rồi phải đo lại từ đầu.

### 4.2 Port 11 route W7 (outings)
Đã sẵn ba tầng dưới: domain `4b5ae715`, repository `040d9954`, thẻ + kịch bản `cff1ba79`. Route nằm ở
`services/api/app/api/routes/outings.py`. Đọc 11 thẻ trong `docs/migration/routes/outings/` — chúng ghi sẵn thứ tự
403-so-với-404, chuỗi câu lệnh, và 15 hành vi Python trông như lỗi **phải giữ nguyên, không được sửa**.

Chỗ khó nhất đã được ghim sẵn trong oracle repository: hai cửa lưu lịch trình là một **unit of work** của SQLAlchemy,
UPDATE phát theo thứ tự session **đã nạp** hàng, bảng cha trước bảng con, và hình dạng INSERT đổi tuỳ theo có phải
đọc lại một cột server-default hay không.

### 4.3 W9 (auth 3 + sessions 4)
Hai agent đang làm domain và repository. Sau đó cần thẻ + kịch bản, rồi port route. Sóng này là gốc của lòng tin:
so sánh bí mật, biên hết hạn, đếm số lần thử. **Repo guard cắn hai lần ở sóng này**: cấm chuỗi ≥9 chữ số và cấm
chuỗi giống số điện thoại — mà đây là sóng OTP. Không viết số điện thoại, token hay mã thật vào bất kỳ tệp nào;
lưu digest, để fixture tự sinh lúc chạy (sóng W7 cho Postgres tự tính `sha256(convert_to('<từ ngắn>','UTF8'))`).

### 4.4 WAI (mảng AI, 24 route) — khối lớn nhất còn lại
Bắt đầu bằng một PR hạ tầng thêm seam `/internal/brain/v1/*` vào **Python**: loại khỏi OpenAPI, chỉ với tới được
trên mạng `backend`, gác bằng `X-Internal-Token` (token rỗng thì từ chối khởi động). Lưu ý rủi ro: thêm route vào
`create_app()` **làm đổi bảng route**, kéo theo golden router và manifest — phải thêm hàng manifest trong cùng PR.
Go giữ transaction mở xuyên qua lời gọi brain; lỗi transport map về đúng thân `intent_error`/`unavailable` của nhánh
except trong Python.

### 4.5 Việc còn nợ, không chặn ai nhưng đừng quên
- **Định dạng ảnh.** Đã đo: Pillow 12.2.0 trong ảnh ghim mở được **40** định dạng, `avif`/`jpg_2000`/`libtiff`/`webp`
  đều bật. Python nhận AVIF trả 201, Go trả 415. **Không route ảnh nào được sang LIVE-GO khi còn lệch đó.** Trước
  lúc cắt: hoặc Go giải được, hoặc chuyển đúng request đó cho Python kèm bộ đếm. Và **harness phải sinh AVIF + TIFF**
  ở một làn riêng — corpus hiện sinh đúng 6 định dạng Go đã hỗ trợ, nên cổng không có cửa nào bắt được lỗ này.
- **Ba chỗ hỏng của harness** (ghi trong QUEUE): hạng `<ts#N>` trượt cả loạt khi hai stack khác số giá trị
  microsecond nền; một `bind` hỏng làm cả lượt dừng bằng INFRA trong khi các kịch bản sau **không chạy**;
  `free_port` từng cấp trùng cổng cho Postgres và API.
- **Làn limiter ra INFRA ba lần** khi máy chạy nhiều agent. Không căn giờ được. Khi còn agent sống, coi pha dev +
  canary + probe là phần đáng tin, rồi chạy **riêng** làn limiter và pha prod trên máy rảnh, ghi rõ trong commit.

---

## 5. Công thức kiểm của người gộp

Mọi deliverable của agent đều phải qua đúng chuỗi này, **trong cây sạch tại HEAD**, không phải cây của agent:

```bash
# 1. drift: HEAD có đụng tệp nào của agent kể từ base của nó không?
#    tệp nào MOVED thì gộp bằng `git apply --3way`, KHÔNG chép đè.
# 2. cây kiểm sạch
rm -rf "$V"; git worktree prune; git worktree add -q --detach "$V" HEAD   # thứ tự này, rm TRƯỚC prune
# 3. cổng rẻ
cd "$V/services/core" && CGO_ENABLED=0 go build ./... && go vet ./... \
  && test -z "$(gofmt -l .)" && go test -count=1 ./...        # gofmt -l thoát 0 cả khi có tệp bẩn
python3 scripts/check_route_ownership.py
git add -A && python3 scripts/repo_guard.py staged && git reset -q
# 4. oracle / tier — LUÔN có -v, và đọc SỐ CA, không đọc mã thoát
go test -tags oracle -count=1 -run TestOracle -v ./internal/pyval/
scripts/go_postgres_tier.sh --image mobile-parity-api:7bf58e3d -- -v ./...
# 5. cổng parity đầy đủ, dưới khoá
flock -o <scratchpad>/stack.lock scripts/gate.sh parity
# 6. hai đột biến của chính mình, chỉ qua -overlay
```

Quy tắc đã trả giá mới có, đừng bỏ:
- `go test` **không** `-v` thì im lặng cho cả ca PASS lẫn ca **SKIP** — một dòng `ok` không chứng minh oracle đã chạy.
- `-race` **cần cgo**: chạy dưới `CGO_ENABLED=0` là phép kiểm rỗng.
- `grep`/`grep -c` không khớp thì thoát 1; dưới `pipefail` nó giết cả script **sau khi** đã in số. Mọi grep để đếm
  hay báo cáo phải `|| true`.
- **Đừng vứt stderr** trong lệnh kiểm: một lỗi thiếu tham số sẽ đội lốt «kết quả đỏ». Khi **mọi** mục trong một loạt
  cùng đỏ, nghi lệnh gọi trước khi nghi cây.
- Bộ sinh corpus 422 **phải chạy trong ảnh ghim** (host pydantic 2.12.5, ảnh 2.13.5), mount repo vào `/repo` —
  chính header của script ghi câu lệnh.
- Nhãn của harness là `EQUAL` / `DIFF` / `RACY` / `INFRA`. Đừng grep nhãn theo trí nhớ; in histogram nhãn trước.

---

## 6. agy review như thế nào

agy **không** thay được người gộp, và digest của agy **không phải bằng chứng** — tác giả plugin đã quan sát agy tự
sửa môi trường của chính nó để ép một lệnh pass. Vai trò đúng của agy ở đây là **người viết ca đối kháng và người
đọc độc lập**, còn con số thì người gộp tự chạy lại.

**Ràng buộc bắt buộc.** ADR-0010 §6.4 cấm `--dangerously-skip-permissions` **trên `/home/lakiet/mobile`**, và §9.2
chỉ cách thay thế: **allow-rule hẹp**. Vậy:
- Cho agy chạy trong một **worktree tách rời dưới `/tmp`**, không phải `/home/lakiet/mobile`;
- Cấp allow-rule hẹp cho đúng các lệnh nó cần (`./parity/bin/parity`, `timeout`, `git -C <worktree>`,
  `curl -s .../healthz`, `printf`), không cấp cờ rộng;
- Entry point thực tế trên máy này: `/home/lakiet/agent-harness/agy_test_pr.sh` (agy ở `~/.local/bin/agy`).
  `agy-delegate` **không** dùng được: auto-mode chặn `--yolo`, và bản headless tự từ chối tool `command`.

**Lệnh giao cho agy phải cơ học**: từng bước đánh số, mỗi bước một lệnh dán nguyên, biến môi trường viết thẳng
trong lệnh, và yêu cầu nó **ghi con số** ra `numbers.json`. agy không suy luận tốt từ mô tả mập mờ.

**agy được sửa đúng một chỗ:** `parity/scenarios-adversarial/<nhóm>/agy-*.yaml`. Kịch bản chỉ có **đầu vào** —
`parity lint` từ chối các khoá `expect`, `status`, `body`, `assert`. Đó chính là thứ cho phép agy viết ca mà không
vi phạm ADR-0010 §6.1: nó không được phép tuyên bố đầu ra đúng là gì, mốc luôn là Python.

**Các bước của agy** (mẫu, đánh số, mỗi bước một lệnh):
1. kiểm hai stack còn sống (`curl -s <ref>/healthz`, `curl -s <cand>/healthz`);
2. `parity run` trên nhóm đang xét, ghi lại `scenarios`, `steps`, `differences`;
3. `parity run` trên corpus 422 của sóng;
4. viết **đúng 5** tệp kịch bản đối kháng, chỉ đầu vào, rồi `parity lint` và chạy chúng;
5. vòng đột biến: với mỗi id trong `parity mutate list` — áp dụng, dựng lại candidate, chạy nhanh,
   `git checkout -- services/core`, ghi một dòng vào `mutations.tsv`;
6. kiểm cây sạch (`git status` phải rỗng ngoài thư mục đối kháng);
7. ghi `verdict.md` + `numbers.json`, ghi đè sau **mỗi** bước.

**Luật cứng cho agy:** không cài gói, không mock, không vá thư viện, không chạy pytest, không chạy docker build;
dừng lại khi cùng một lỗi lặp lại lần thứ hai.

**Sau khi agy xong, người gộp làm ba việc:**
1. **Kiểm giả mạo**: băm lại harness, kịch bản và recordings; `git status`. Bất kỳ thay đổi nào **ngoài** thư mục
   đối kháng là FAIL, không thương lượng.
2. **Tự tính lại số** từ log, không lấy số agy khai.
3. Chạy lại cổng, canary, corpus 422 và bộ đối kháng trong **cây sạch tại đúng SHA**, cộng hai đột biến của chính
   mình; rồi viết ghi chú bằng chứng vào `docs/agy/<ngày>/parity-<nhóm>.md`.

Nếu thấy agy sửa môi trường của chính nó để ép xanh — đã từng xảy ra — thì phán quyết là FAIL và ghi lại, đừng
chạy lại cho tới khi xanh.

---

## 7. Cái này KHÔNG chứng minh

Vẫn nguyên như ADR-0029 §4, nhắc lại vì dễ đọc quá bằng chứng:
- Không chứng minh hình dạng dữ liệu thật, tải thật, hay query plan thật.
- Không chứng minh đường hạnh phúc thật của Google hay SMS — stack dùng mã gỡ lỗi.
- Không chứng minh hành vi Gemini.
- Không chứng minh phần định tuyến của preview (xem mục 4.1) **cho tới khi có stub**.
- Không chứng minh 34 định dạng ảnh mà corpus chưa sinh (xem mục 4.5).
- Không chứng minh gì về LIVE-GO: chưa route nào được cắt quyền, nên toàn bộ chiến dịch tới giờ là «Go trả lời
  giống Python trên bộ ca đã có», không phải «Go đã chạy thật ngoài đời».
