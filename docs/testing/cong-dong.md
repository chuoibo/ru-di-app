# Cộng đồng — triển khai và bằng chứng

Triển khai ADR-0040 từ cây `c8f092c1` đã có thay đổi diary/UI chưa commit.
Chưa phải bằng chứng tại SHA sạch. Người dùng xác nhận môi trường đích là
local hiện tại; đã bật API `8199` sau E2E Android, chi tiết ở mục cuối.
`LIVE-GO` xác định writer khi bật cờ, không thay cổng phát hành. Backend mới là Go/SQL; Python
mới chỉ làm adapter inference, không có writer nghiệp vụ Python mới.

## Phạm vi đã nối

- Tab riêng: Dành cho bạn, Đang theo dõi, Thịnh hành; chủ đề, tìm kiếm,
  follow, lưu/ẩn bài, thông báo tag, bài của mình và trạng thái duyệt.
- Cùng ID bài/like/comment với tường. Bạn bè là quan hệ accepted hiện tại;
  follow/tag không cấp quyền. Giữ Chỉ mình tôi và nhóm. Bài public cũ chỉ
  vào cộng đồng khi tác giả chủ động gửi; nút trên màn tường cũ hỗ trợ bài
  public chỉ có chữ. Ảnh cũ cần tạo bài mới qua uploader có kiểm quyền.
- Public mới chờ duyệt. Bản sửa có revision, người khác chỉ thấy bản đã
  duyệt. Endpoint cũ không được sửa vòng qua kiểm duyệt. Có hàng đợi người
  vận hành, cấm tự duyệt, lý do và audit.
- Ảnh/album/video có xác thực; bình luận, trả lời một cấp, ảnh bình luận,
  tag bạn, xóa. Public comment chờ duyệt.
- Chia sẻ bản nhật ký đã chọn và sao chép media; sửa/xóa nguồn thu hồi bản
  chia sẻ. Nếp cần đồng ý đúng đoạn trích, trả nháp sửa được, lưu riêng.
- Cá nhân hóa khi đồng ý; tắt/xóa lịch sử/không quan tâm. Chỉ dùng tương tác
  cộng đồng. Xếp hạng v1 dùng độ mới, tương tác, chủ đề thường xem, follow
  và đa dạng tác giả; không phải thuật toán độc quyền TikTok.

## Chạy tính năng

1. Migrate nền trên PostgreSQL, chạy `core migrate-community`. Lệnh kiểm
   checksum và khóa migration; bao gồm phụ thuộc diary và bốn migration
   cộng đồng (số 4, 2026-10-02: bản cắt duyệt của video). DB đã bật cộng
   đồng phải chạy lại lệnh này, nếu không `core serve` từ chối khởi động.
   Không chạy migration từ HTTP.
2. API: `MOBILE_AUTH_MODE=prod`, `MOBILE_COMMUNITY_ENABLED=1`, DB và phiên
   thật. Cờ backend mặc định tắt. Composer tường dùng bản cũ khi capability
   trả 404; lỗi mạng không được rơi xuống đường cũ.
3. Duyệt bài và Nếp chạy trong `core serve` qua agy-proxy (ADR-0052):
   đặt `AGY_PROXY_URL`/`AGY_PROXY_KEY` cho core. Không có model thì bài
   công khai nằm chờ duyệt, Nếp trả 503 `nep_unavailable`. Python không còn
   bước nào của cộng đồng; `COMMUNITY_INFERENCE_URL` đã bỏ.
4. Build codec: `docker build -f services/core/Dockerfile.community-media
   -t rudi-community-media services/core`. Image mặc định chạy
   `core community-media-worker`, cùng DB và volume `MOBILE_MEDIA_ROOT`
   của API, quyền ghi UID 10001. API không chạy codec.
5. Build lại native vì thêm `expo-video`; Metro reload không đủ. Cấu hình
   origin CORS và proxy WebSocket theo môi trường.

**Từ ADR-0052 (2026-10-01) model đọc bài chạy qua agy-proxy**; trước đó người
dùng chọn giữ bài chờ duyệt vì chưa có model. Chất lượng phán đoán mới chỉ
được thử bằng vài bài bịa (`vnlocal-thu tinh-nang`), chưa đánh giá có hệ thống. Người vận hành cấp vai trò bằng
`community_moderators`; UI `/community/review` duyệt bài và bình luận.

Model đọc bài (`aiharness/congdong`, lời dặn mới từ ADR-0052) nhận chữ cùng
**mọi** ảnh và video của bài, trả `relevant`, `safe`, `confidence_milli`
(0..1000, phần nghìn: 900 = 90%), `reason`. Từ 2026-10-02 (chủ sản phẩm: giữ
ngưỡng 90%, AI phải chấm):

- Ảnh được thu nhỏ trước khi gửi (JPEG, cạnh dài 1536 → 1024 → 768 → 512 px,
  lấy nấc lớn nhất mà cả bài vừa 14 MiB), nên bài 10 ảnh điện thoại hay nhật
  ký 40 ảnh vẫn được xem đủ. Trước đó ảnh cộng dồn quá 14 MiB thì không gửi
  ảnh nào.
- Video gửi bằng **bản cắt duyệt**: media worker cắt lúc xử lý upload, mỗi
  đoạn 45 s, 1 khung/giây, cao ≤ 360 px, kèm tiếng mono (cột
  `community_media.review_keys`, migration cộng đồng số 4). Mỗi đoạn một lời
  gọi; kết quả gộp: liên quan nếu một lần đọc nói có, an toàn nếu mọi lần đọc
  nói có, độ tự tin lấy thấp nhất. Gửi nguyên video 180 s qua agy mất 171 s
  rồi lần sau không trả lời trong 400 s; một đoạn 45 s mất 4–30 s. Video xử lý
  trước migration 4 không có bản cắt: không gửi, bài vào duyệt tay.

`media_checked` do Go tự đặt, chỉ đúng khi mọi tệp đính kèm đã đi cùng các lần
đọc. Go chỉ tự duyệt từ 900 và media đã kiểm đủ; một lời gọi hỏng thì cả lần đọc
hỏng, job chờ 1 phút rồi đọc lại (lease 8 phút đủ cho 4 đoạn × 85 s). Nếp chỉ
trả `draft`. Không cấp DB/chat cho model. Test cắt video thật mang tag
`communitymedia` (cần ffmpeg) chạy ở chặng `go-media` của `scripts/gate.sh`
(bước «Community video review cut with a real ffmpeg» của job `core`); thiếu
ffmpeg là HỎNG, không phải bỏ qua.
Stub chỉ chứng minh orchestration, không chứng minh chất lượng AI guard.

## Realtime và lưu trữ

`/v2/community/stream`: bearer ở frame đầu trong 5 giây, không đặt trong URL;
40 bài/stream, 4 stream/người/instance, 12.000 stream/instance, queue 32 và
ngắt client chậm. Reconnect yêu cầu snapshot mới; không hứa exactly once.

PostgreSQL giữ thứ tự sự kiện sau commit. Replica reconcile 500 ms, gom thay
đổi cùng bài, drain backlog theo batch. `MOBILE_COMMUNITY_REDIS_URL` chỉ đánh
thức; mất Redis vẫn reconcile. Kiểm ACL theo batch người nhận; kiểm thu hồi
phiên mỗi 10 giây và khi có sự kiện quyền. Feed hint không chứa nội dung/ID
bài riêng. Client hiện báo cập nhật để người dùng giữ vị trí cuộn.

Feed: 500 ứng viên, 20 bài/trang. Cache một giây chỉ giữ đầu vào public;
mỗi lần đọc vẫn kiểm ACL/ẩn bài hiện tại. Snapshot cùng người/mode/thứ tự
ID được dùng lại; refresh không ghi lại snapshot còn hạn trên một phút.
Counter like/comment do trigger bảng gốc cập nhật cùng
transaction, có thể dựng lại từ bảng gốc. Lịch sử 90 ngày, snapshot 30 phút,
sự kiện 1 ngày; cleanup định kỳ. Cần đo disk/WAL và autovacuum trong soak.

Ảnh decode/encode bỏ metadata, tối đa 12 MB/24 triệu pixel. Video tối đa
64 MB/3 phút; worker ffmpeg tạo MP4 H.264/AAC cạnh dài tối đa 1280. Mỗi bài
tối đa 64 MB media. Media GET kiểm ACL, `private, no-store`, hỗ trợ Range.
API giới hạn 4 upload đồng thời; codec worker xử lý từng job bằng lease.
Image codec không có shell; binary/container đã chạy ffmpeg thật.

## Kiểm chứng

| Lớp | Kết quả và giới hạn |
|---|---|
| PostgreSQL | **32 PASS**, `-race -tags postgres,communitymedia`, sentinel có mặt, 0 skip; gồm ca tái hiện race khởi động relay |
| HTTP E2E | **33 request PASS** qua front door thật và WebSocket khác replica |
| Android | Maestro `_community.yaml` qua hai lượt: public → pending → Bạn bè → bình luận; bản chạy QA chỉ đổi applicationId sang package tổng hợp |
| Hai người Android | Hai emulator/phiên riêng; peer gửi comment, người nhận thấy “Có bình luận mới”, chạm đọc được không rời màn. Bước peer dùng ADB/UI hierarchy, không ghi thành Maestro pass |
| Video | Video ffmpeg tổng hợp 1920×1080 được xử lý 1280×720; GET 200/Range 206, outsider 404 trên PostgreSQL |
| Python AI adapter | **8 PASS**; không đo model |
| Mobile | Typecheck và **1097 test PASS**; web export có dữ liệu, không JS error |
| Contract/ownership | 189 route server, 209 path/222 call client, 13 unresolved đã pin; manifest 206 route/199 Go |
| CORS | 7 header/5 method qua preflight |
| Go | Unit toàn module và vet đạt sau tối ưu counter |
| Repo guard | staged đạt (0 file staged), tree HEAD đạt 3749 file; scan_entry riêng mã nguồn cộng đồng đạt sau annotation UUID zero tổng hợp; số cuối ghi dưới |

DB test có retry đồng thời, revision cũ, friend/follow/block, pending không
lọt endpoint cũ, image/comment ACL, cache warm sau đổi riêng tư, snapshot
dùng lại/xóa lịch sử, race consent, diary thu hồi, private keeps, moderation
comment đúng một lần, websocket hai server/thu hồi phiên, counter theo cả
writer tường cũ, retry/xóa/rollback.

Hai mutant không tương đương đều đỏ đúng chỗ: mở friends cho stranger làm
`FriendsFollowAndBlock` nhận 200 thay 404; bỏ media_checked làm
`ModerationFailsClosed` nhận approved thay review. Identity DB đạt. Chưa
phải bộ mutant đóng dấu harness SHA tại commit sạch.

Impeccable đã mở đủ [Android feed](../../.impeccable/review/community-phone-android.png),
[composer](../../.impeccable/review/community-composer-android.png),
[comments](../../.impeccable/review/community-comment-android.png),
[web mobile](../../.impeccable/review/community-mobile-web.png),
[web desktop](../../.impeccable/review/community-desktop-web.png).
Verdict pass `ship` cho ba sửa đổi: vùng chạm 48dp, tên truy cập có động từ
và số đếm, cột desktop 560px. Documenter giữ DESIGN.md/sidecar. Ảnh ignored
chỉ ở workspace, không chứng minh iOS/dark/chữ lớn/frame thiết bị thật.

## Chạy lại trên dữ liệu tổng hợp

Cách chạy HTTP/WebSocket E2E tự chứa (cần Docker, Go, Node 22+, curl):

```sh
scripts/community_e2e.sh
```

Lệnh tự dựng PostgreSQL với durability mặc định, migrate nền từ API image
của cây hiện tại, migrate Go, tạo bốn phiên tổng hợp và hai replica Go.
Không kế thừa env ứng dụng, không dùng `.env` thật hay model. Readiness phải
chạm endpoint cộng đồng; kết quả phải có đủ request và replica riêng.
Lượt đã chạy: 33 request PASS, WebSocket khác replica PASS, 0 skip; tự dọn
process/container/image tạm, giữ log/fixture quyền 0600 dưới `/tmp`.
Không thay thế E2E native, codec hoặc đánh giá model.

Tạo DB dùng một lần tên `community_synthetic_<tên>`, migrate nền. Đặt
`MOBILE_DATABASE_URL` của DB này ngoài repo. Từ `services/core`:

```sh
go run ./cmd/community-fixture -actors 20000 -out /tmp/community-actors.json
go run ./cmd/community-load -base http://127.0.0.1:18178 \
  -actors /tmp/community-actors.json -sockets 10000 -reads 1000 -writes 200 -duration 60s
```

Fixture từ chối DB không rỗng/prefix sai/ngoài loopback, file trong worktree;
tạo 500 bài tổng hợp approved bằng SQL, không đo model. Bốn người đầu là
tác giả, bạn accepted, người lạ, moderator. File token giả có quyền 0600.
CLI fixture đã chạy trên PostgreSQL rỗng riêng với 4 người/500 bài.

Từ root, sau khi mở hai API cùng DB/media:

```sh
node scripts/community_e2e.mjs http://127.0.0.1:18184 \
  /tmp/community-actors.json http://127.0.0.1:18182
```

Load chỉ nhận loopback, một socket/người cùng xem bài nóng, GET feed và
PUT/DELETE like. Histogram cố định; đếm dropped slot, timeout, socket đóng
và số client nhận cập nhật. Clock cùng host. Dựng fixture mới hoặc reset
reaction trên **DB tổng hợp** trước lượt độc lập để PUT không thành no-op.
Không reset trong lúc E2E chạy. `-duration 24h` là soak, chưa chạy trong phiên.

## Cổng còn mở

Không coi tải like là tải đăng video/comment/AI. Còn cần soak 24 giờ,
tải media/AI, model/evaluation thật, iOS/thiết bị thật và gate tại SHA sạch.
Cờ production tiếp tục tắt. Kết quả tải/gate cuối được ghi dưới đây.


## Kết quả tải và gate tại lượt cuối

Máy local dùng chung với các gate khác; Go thường (không `-race`), PostgreSQL
thật, 500 bài/20.000 tài khoản tổng hợp, 15 kết nối DB mỗi API. Không có CDN,
không có Redis, không có inference thật trong lượt tải. Chỉ là workload
feed và like trên cùng một bài nóng, chưa phải chứng nhận môi trường thật.

| Lượt | Socket | Read/write mỗi giây | Thời gian | p95 feed / like / event | Kết quả |
|---|---:|---:|---:|---|---|
| Mục tiêu, 3 API, counter cuối | 10.000 | 1.000 / 200 | 60s | 223 / 438 / 542 ms | PASS; 60.000 GET + 12.000 like; 0 lỗi/drop/socket đóng; đủ 10.000 người nhận event |
| Burst, 3 API | 20.000 | 2.000 / 400 | 30s | 1.489 / 2.981 / 22.182 ms | FAIL; 13.570 lượt đọc không phát kịp; mọi socket còn mở |
| Burst, 5 API | 20.000 | 2.000 / 400 | 30s | 1.833 / 3.738 / 20.712 ms | FAIL; 19.508 lượt đọc không phát kịp; mọi socket còn mở |
| Mục tiêu, 3 API, xếp hạng cuối | 10.000 | 1.000 / 200 | 60s | 329 / 665 / 749 ms | PASS; 60.000 GET + 12.000 like; 0 lỗi/drop/socket đóng; đủ 10.000 người nhận event |
| Burst, 3 API, xếp hạng cuối | 20.000 | 2.000 / 400 | 30s | 3.488 / 5.003 / 23.283 ms | FAIL; 30.034 read + 25 write không phát kịp; 5.227 write timeout/transport error; mọi socket còn mở |

Không bỏ lượt fail khỏi kết luận. Tăng API trên cùng host chưa giải quyết
burst. Cổng tải cao chỉ đạt mốc ngắn cơ bản; burst và soak còn mở.
Snapshot/counter và xếp hạng cuối đã qua 31 test PostgreSQL với race detector.
`rank` chỉ dịch mảng chỉ số thay vì sao chép struct ứng viên khi chọn bài;
600 tổ hợp seed/mode/consent đối chiếu bản trước giữ nguyên thứ tự. Benchmark
500 ứng viên ba lượt giảm 0,76–0,80 ms xuống 0,60 ms/lượt, nhưng không suy
thành SLA tốt hơn: bài tải cuối trên host dùng chung vẫn fail burst như bảng.
Benchmark để lại tại `rank_benchmark_test.go`; kiểm đối chiếu bản cũ dùng
Go overlay ngoài repo, không thêm implementation oracle vào production.

Lượt `make gate` toàn repo đã xong: **18 PASS / 4 FAIL / 5 SKIP**. Ba lỗi
screens/CORS/API ở lượt đầu đã sửa và gate tương ứng chạy lại xanh. Lỗi
Go PostgreSQL dẫn tới kiểm tra lại race khởi động relay và assertion chưa
chấp nhận sự kiện `sync` hợp lệ; cả hai được sửa và kiểm chứng ở lượt cuối
bên dưới. Các skip: guard-range không có commit mới, demo-watch,
hero-walk chưa dựng demo, Maestro không ở PATH của lượt toàn repo, Cargo
chưa có. Các flow Android cộng đồng được chạy riêng với Maestro thật.

Parity toàn dev/prod đã PASS: dev 354 scenario/10.835 step, prod 23
scenario/604 step, HTTP/DB/media 0 khác biệt; lane limiter 9 scenario đã
xong. Các canary identity xanh, mọi hỏng được thử đều bị bắt; riêng
media-file-dropped dev 157/459 và prod 5/30. Probe 22 ca, 10 khác biệt được
ADR chấp nhận, 0 bất ngờ/0 waiver cũ. Không phải gate ở cây sạch.

Lượt API cuối sau sửa applicationId Maestro: **3784 passed, 802 skipped,
0 failed**, 3 warning, 682,64 giây; gate API PASS. Các ca PostgreSQL nằm ở
tier riêng đã chạy thật ở trên, không tính 802 skip thành bằng chứng DB.
Lượt đầu còn lỗi reachability/CORS đã sửa và gate tương ứng đã xanh. Chưa
tuyên bố full gate xanh từ việc cộng các lượt chạy này.

Log phiên ở ngoài repo: `/tmp/rudi-community-gate.log`,
`/tmp/rudi-community-postgres-final.log`, `/tmp/rudi-community-http-e2e-final.log`,
`/tmp/rudi-community-load-counted-target.json`,
`/tmp/rudi-community-load-counted-burst.json`,
`/tmp/rudi-community-load-five-burst.json` và
`/tmp/rudi-community-remaining-gates.log`. Chúng không chứa dữ liệu người dùng
thật và không phải artifact có thể viện dẫn sau khi máy dọn thư mục tạm.


Bổ sung cuối: video web đã chạy qua HTTP có bearer → blob tạm → giải mã/phát
thật trong Chrome; currentTime tăng, duration 2s, không lỗi JS. Đã sửa điểm
`expo-video` web không hỗ trợ header như native. Runner lưu tại
`apps/mobile/tests/community-video.e2e.mjs` (tham số: URL web, fixture JSON,
ID bài video tổng hợp đã duyệt). [Ảnh video](../../.impeccable/review/community-video-web.png)
đã mở và reviewer chấm `ship` riêng cho sửa chức năng này. Native không đổi.

Các gate chạy bổ sung: PostgreSQL Python **721 + 88 PASS**, PostgreSQL Go
**2159 PASS**, E2E client TypeScript PASS, E2E chat **43 PASS**, sentinel
có mặt. Crypto **SKIP vì thiếu Cargo**, không tính xanh. Gate thay đổi
screens/CORS/client-routes/ownership/go-vet: **5 PASS, 0 fail, 0 skip**.
Codec container nonroot đã transcode clip tổng hợp với network none,
read-only filesystem, no-new-privileges và 512 MB RAM.

Sau sửa xếp hạng: Go unit/vet và tier PostgreSQL 31 PASS. Log tải bản cuối ở
`/tmp/rudi-community-load-ranked-target.json`,
`/tmp/rudi-community-load-ranked-burst.json`; PostgreSQL ở
`/tmp/rudi-community-postgres-ranked.log`. Mobile sau sửa video vẫn
1097 PASS, 0 skip; browser video runner đạt với currentTime 0,600474s.

Lượt hoàn thiện tường cũ: browser mở xác nhận trên web, Maestro
`_community-legacy.yaml` gửi thành công sang màn chi tiết mới; HTTP xác nhận
cùng ID, revision 1/pending cho tác giả và 404 cho người lạ. Chạy cùng
synthetic QA package; cần mở đúng package trước flow vì cả package thật và
QA dùng scheme `rudi`. Ảnh đã mở:
[Android](../../.impeccable/review/community-legacy-share-android.png),
[web](../../.impeccable/review/community-legacy-share-web.png).
Finish reviewer mới chấm `SHIP` riêng cho cầu nối này sau khi mở hai ảnh
và đọc diff; documenter đã ghi phạm vi vào brief. Không gồm iOS, thiết bị
thật hoặc chạy lại bằng chứng hành vi do lượt triển khai cung cấp.
Mobile sau cầu nối vẫn 1097 PASS; typecheck và bốn gate
screens/CORS/client-routes/contract xanh; meta Maestro/screen 23 PASS,
3 skip do pin không tồn tại.

Race cuối: relay có thể lấy max event sau khi một reader đã kết nối và có
mutation mới, làm bỏ qua mutation. Test mới cố ý nối reader, commit like,
rồi mới khởi động relay: đỏ trước sửa ở đúng read timeout 8 giây. Sửa lấy
mốc nhỏ nhất của các reader đã nối và xếp `sync` trước khi công bố subscriber.
Tier đầy đủ của module với race detector **32 PASS, 0 skip**; E2E tự dựng
hai replica vẫn PASS cả bài mới và comment mới. Log:
`/tmp/rudi-community-relay-red.log`, `/tmp/rudi-community-relay-green.log`,
`/tmp/rudi-community-e2e-relay-final.log`.

Khi chạy chung, các test module khác đổi phiên/bạn bè nên relay có quyền
gộp cập nhật thành `sync`. Assertion cũ chỉ chờ `post.changed`, làm đỏ dù
client đã được yêu cầu tải lại. Assertion mới yêu cầu cursor đi qua đúng
commit của like và GET lại thấy like đó; không tăng timeout. Thu hồi phiên
phải nhận close/EOF từ server trước deadline, không nhận timeout của test
làm bằng chứng. Gate Go PostgreSQL toàn repo cuối **2160 PASS, 0 skip** tại
`/tmp/rudi-community-relay-combined-final.log`. Hai ca realtime được lặp
10 lần với race detector: **20 lượt PASS**, thêm 10 lượt sentinel PASS ở
`/tmp/rudi-community-relay-stress-verified.log`.

Source guard cuối quét 69 file mới/sửa trong phạm vi cộng đồng, 0 findings.
Đã soát 12 dòng thêm `expo-video` trong lockfile và cập nhật đúng digest
allowlist cũ; không mở rộng rule. `staged` vẫn 0 file, `tree HEAD` 3749 file
đạt; cả hai không thay bằng chứng commit sạch của thay đổi hiện tại.


## Điều tra nguyên nhân và lượt hoàn thiện local 27/09/2026

Các số bên dưới bổ sung bằng chứng mới, không xoá các lượt fail ở trên.

- **Realtime tranh pool DB:** profile đo relay chờ Acquire 30,41 giây trong
  khoảng burst 30 giây. Mỗi batch người nhận lại xếp sau request HTTP.
  Relay nay giữ riêng một connection trong giới hạn pool 15 có sẵn và nối
  lại khi connection chết. Test chiếm hết pool bằng request đỏ trước sửa,
  xanh sau sửa; vẫn kiểm session/ACL hiện tại, không cache quyền truy cập.
- **Feed lặp việc và round trip:** dùng chung thứ hạng của snapshot public
  một giây cho trending/foryou chưa consent; personalized/following vẫn
  tính theo từng người. Gộp kiểm session thành một SQL giữ thứ tự khoá
  person rồi session; chỉ query media cho bài có media. Snapshot không đổi
  được đọc lại, chỉ gia hạn gần hết hạn; test PostgreSQL kiểm xmin không
  đổi khi refresh và cursor hết hạn được phục hồi khi tải feed mới.
- **Android upload thiếu MIME:** file Blob không có type khiến Android gửi
  Content-Type rỗng dù JS đặt header. Trace ngoài repo đo 245.541 byte bị
  422, cùng ảnh có typed Blob được nhận. Đặt MIME trên body và header;
  đóng cả Blob nguồn lẫn slice sau response, kể cả lỗi. Bốn regression
  cases bảo vệ byte, MIME và vòng đời hai reference, không thay E2E native.
- **Schema cũ chỉ lỗi sau khi nhận request:** API và codec worker nay từ
  chối khởi động nếu ledger migration thiếu, thiếu version hoặc lệch hash.
  Không sửa checksum/migration cũ. DB QA development cũ lệch hash đã được
  thay bằng DB tổng hợp mới; local thật kiểm checksum diary đủ ba version
  khớp trước migration. CheckSchema không phải audit mọi DDL vật lý.

Kiểm chứng bản cuối:

| Lớp | Bằng chứng |
|---|---|
| Go PostgreSQL đầy đủ | **2163 PASS**, sentinel có mặt, 0 skip |
| Community/core/db với race và codec | **47 PASS**, sentinel có mặt, 0 skip |
| Mobile | **1101 PASS**, 0 skip, typecheck PASS; dựng web mới trước chạy test |
| HTTP/WebSocket | **33 request PASS**, hai replica thật, cả publication và comment realtime |
| Android ảnh | `_community-media.yaml` chạy trọn: picker → upload → tag An QA → Bạn bè → bài chi tiết |
| Android video | `_community-video.yaml` chạy trọn: picker → processing/ready → Bạn bè → bài; ADB phát clip và ảnh xác nhận 00:00 → 00:02 |
| Android hai người | Sender và receiver đều Maestro PASS trên hai emulator, người nhận mở bình luận mới ngay tại màn chi tiết |
| Gate chọn theo thay đổi | screens/CORS/client-routes/contract/ownership/go-vet: **6 PASS**, 0 skip |
| Meta native | **23 PASS, 3 SKIP** do pin không tồn tại; không tính skip là bằng chứng native |

Bản QA Android chỉ đổi applicationId; app native thật được build lại có
expo-video. Đã mở ảnh [hai người](../../.impeccable/review/community-two-people-android.png),
[ảnh và tag](../../.impeccable/review/community-photo-tag-android.png),
[video phát hết](../../.impeccable/review/community-video-android.png).
Ảnh chọn trong picker là screenshot QA tổng hợp cũ, nên hình chứa UI;
đó là nội dung ảnh được upload. Finish reviewer Impeccable xác nhận SHIP
riêng sửa vòng đời Blob. Chưa đo frame timing, iOS hoặc thiết bị vật lý.

Lỗi harness được giữ rõ: một lượt mobile đọc bundle cũ bị từ chối rồi chạy
lại bằng npm test có build; một lượt tier chọn thiếu package sentinel bị
đỏ, lượt 47 PASS đã thêm sentinel. Tag Docker dùng chung từng bị worktree
khác thay image, gây lỗi schema ở module ngoài cộng đồng. Tier cuối dùng
image dựng từ cây này và pin bằng immutable image ID. Không sửa test hay
schema sản phẩm để làm xanh image không đúng nguồn.

### Đo đối chứng tải

Ba API, 15 connection/API, 20.000 actor/socket độc lập, 500 bài tổng hợp,
PostgreSQL 16 fsync/synchronous_commit bật; 2.000 feed GET và 400 like/s.
Cùng harness, cùng DB tổng hợp được reset giữa lượt. Dừng hai emulator QA
và không chạy gate/build nặng cùng lúc; host vẫn dùng chung, không phải
máy benchmark độc quyền. Mỗi lượt tạo mới LB/API; profiling chỉ bằng Go
overlay ngoài repo, không có pprof trong binary phục vụ local.

| Bản/lượt | Thời gian | p95 feed / like / event (ms) | Kết quả |
|---|---|---|---|
| Trước sửa pool/ranking/session | 30s | 526 / 1055 / 3986 | FAIL; 60.000 đọc + 12.000 ghi, 0 lỗi/drop |
| Sau sửa pool/ranking/session | 30s | 157 / 326 / 182 | PASS; 60.000 đọc + 12.000 ghi, 0 lỗi/drop |
| Xác nhận kéo dài | 60s | 808 / 1652 / 188 | FAIL; 119.900 đọc, 100 drop; 24.000 ghi thành công |
| Bản cuối có snapshot ít ghi | 60s | 1151 / 2296 / 202 | FAIL; 117.260 đọc, 2.740 drop; 24.000 ghi thành công |

Mốc mục tiêu trên **bản cuối** chạy riêng 60 giây: **10.000 socket,
1.000 đọc/s, 200 ghi/s PASS**. p95 feed **44 ms**, like **50 ms**, event
**99 ms**; 60.000 GET và 12.000 like HTTP 200, 0 lỗi/drop/socket đóng;
đủ 10.000 người nhận thay đổi. Log
`/tmp/rudi-community-isolated-target-final.log`. Đây vẫn là bài ngắn trên
loopback, không thay soak hoặc benchmark trên hạ tầng triển khai phân tán.

Cả bốn lượt đủ 20.000 người nhận thay đổi, 0 socket đóng bất ngờ.
Không suy từ 30s PASS thành sustained 20.000 đạt. Global advisory lock
bảo đảm thứ tự commit outbox vẫn là điểm tranh chấp: lượt cuối trung bình
38,40 ms chờ/ghi trong pg_stat_statements. Sửa snapshot loại ghi WAL cho
94.935 refresh, nhưng không đủ làm burst kéo dài đạt; không ghi thành cải
thiện latency. Cần giải quyết giới hạn writer/DB và đo trên tài nguyên ổn
định trước khi cam kết mốc này. Soak 24h vẫn chưa chạy.

Log mới: `/tmp/rudi-community-rootcause-race-final.log`,
`/tmp/rudi-community-refresh-full-postgres.log`,
`/tmp/rudi-community-upload-mobile-built.log`,
`/tmp/rudi-community-rootcause-http-final.log`,
`/tmp/rudi-community-final-media-ready.log`,
`/tmp/rudi-community-final-video.log`,
`/tmp/rudi-community-native-send.log`,
`/tmp/rudi-community-native-receive.log`,
`/tmp/rudi-community-isolated-{before,after,confirm,final}.log`.
Đây là log phiên ngoài repo, không phải artifact bền vững ở SHA sạch.

### Local đang phục vụ

Theo xác nhận của người dùng, đích là stack local đang có: Go `8199`,
liveness `8299`, Python `8198`, Metro `8095`. Đã chạy migration tường minh,
bật `MOBILE_COMMUNITY_ENABLED=1`, giữ auth prod và cấu hình hiện có.
Binary đang chạy khớp SHA256 bản Go đã kiểm thử, không chứa overlay trace.
API trả livez 200 và route cộng đồng trả 401 khi chưa xác thực. APK thật
`com.lakiet.rudi` có expo-video đã cài cập nhật lên emulator `5570`, giữ dữ
liệu app. Mở bằng Metro `8095` (đã xác minh API target `8199`); Maestro xác
nhận Dành cho bạn/Thịnh hành/Đăng khoảnh khắc hiển thị, không ở dev launcher.
Log `/tmp/rudi-community-local-screen.log`. Không tạo account QA hoặc ghi
nội dung tổng hợp vào DB local này; không chọn consent thay người dùng.

Hai user service `rudi-community-local-api` và `rudi-community-local-media`
đang chạy, Restart=on-failure. Chưa enable tự chạy khi đăng nhập vì DB và
Python vẫn do stack local hiện hữu quản lý. Cấu hình riêng, log và binary
cũ để khôi phục nằm ngoài repo ở `~/.local/state/rudi-community-local/`;
không đưa cấu hình thật vào Git. Dùng `systemctl --user status` hoặc
`restart` với hai tên service trên để kiểm tra/khởi động lại.

Chưa có model inference: nội dung public vẫn pending, không tự duyệt;
người vận hành phải được cấp vai trò rõ ràng mới dùng `/community/review`.
Chưa chứng nhận phát hành ở SHA sạch, chưa chạy lại một full make gate
xanh duy nhất; các thay đổi của phiên và thay đổi có sẵn vẫn chưa commit.


Source guard sau bàn giao quét **79 file** cộng đồng và các điểm tích hợp
trong working tree, **0 findings**; staged 0 file và tree HEAD 3749 file
cũng đạt. `git diff --check` đạt. Đã dừng các service, Metro và PostgreSQL
QA thuộc phiên sau kiểm thử; giữ log/fixture ngoài repo. Emulator `5570`
đang mở app thật với local `8199`; emulator `5574` và các stack khác không
bị thay đổi.


## Bàn giao lên main ngày 28/09/2026

Tách 110 file tính năng và phụ thuộc nhật ký sang checkout riêng, gộp với
main `0b51d5be` có thêm 68 commit. Không lấy thay đổi copy/UI/nghiên cứu
không liên quan trong workspace gốc. Do main đã dùng số ADR 0037/0038,
hai tài liệu mới chuyển thành ADR-0039 (nhật ký) và ADR-0040 (cộng đồng).
Giữ các thay đổi Skia, FlatList/View, gzip và janitor hiện có trên main.

Trên cây sạch `cf2dcf60`: 61 test PostgreSQL với race/codec, sentinel,
0 skip. Sau đổi số ADR (source runtime giữ nguyên), cây sạch `0ec19fa9`
chạy HTTP/WS33 request PASS. Hai mutant không tương đương đã thử lại
cùng harness: friends cho người lạ đọc200 thay404 và bỏ media_checked
choapproved thayreview đều đỏ ở đúng assertion; identity3PASS gồm sentinel.
Browser thật: feed200; legacy submit202 → pending, người lạ404; 0 lỗi JS.
Ảnh đã mở và reviewer Impeccable SHIP riêng UI bridge sau merge:
[phone](../assets/community/feed-phone.png),
[desktop](../assets/community/feed-desktop.png),
[pending](../assets/community/legacy-pending.png).

Lượt gate đầu trên cây sạch bắt hai thiếu sót tích hợp: evidence route
còn trỏ số ADR cũ và Composer import helper của phần workspace chưa
commit. Bản bàn giao sửa đường dẫn evidence và thêm helper coTuongNhom tối
thiểu vào module chính sách, không kéo theo thay đổi loại sổ khác.
Số gate sau cùng nằm trong commit bàn giao; không coi số test trước gộp
là số của main mới. Full native, burst/soak và chất lượng AI không được
suy rộng từ kiểm tra browser/contract sau gộp.


Lượt sạch `90f2efa6`: PostgreSQL race/codec 61 PASS, sentinel, 0 skip;
HTTP/WS hai replica 33 request PASS; identity3PASS và hai mutant ACL/media
đỏ đúng assertion, cùng harness SHA. Hai lượt dựng fixture HTTP đầu dừng
trước khi tạo fixture; lượt sau hoàn tất, không thay source hay giảm assertion.

Mobile sau sửa typecheck: 1.232 PASS, 6 FAIL, 0 skip. Nguyên nhân và bản sửa:
- Nút Để sau màu accent trên paper: dùng chữ ink, giữ hit target 48.
- Hai TextInput thiếu focus theo quy ước web mới: bỏ viền browser, vẽ
  border theo theme khi focus, không bỏ dấu hiệu focus.
- Bộ phân loại sổ phải ở một module: helper tối thiểu đặt trong ban-tinh.
- Test nhật ký mang theo giả định hai loại sổ của workspace cũ: giữ ba
  loại của main theo ADR-0027; pair không có consent vẫn là hai-nguoi.
- Fixture Maestro chưa biết các flow mới: bổ sung tên screenshot và chín
  chuỗi động/nhãn Android kèm nguồn giải thích. Đây là mở rộng fixture cho
  flow mới, ngoại lệ với ghi chú chỉ co danh sách cũ; không đổi selector,
  không nới bộ quét, không xoá test. Chưa gọi đó là native pass sau merge.

Lượt kiểm lại bản sửa được ghi bằng SHA và số cụ thể trong commit bàn giao.


## Kết quả chốt để push main

Cây sạch có code cuối: `f496af16897ee2b5de2185feae9cac136786725e`.
- `make gate ONLY="guard guard-range ruff contract client-routes server-routes
  screens cors ownership python-touch go-vet go-test shared mobile"`: 14 PASS,
  0 FAIL, 0 SKIP. Mobile 1.239 test PASS, 0 fail/skip; export cả web/iOS/Android.
- PostgreSQL race/codec community + diary + db + startup: 61 PASS, sentinel,
  0 SKIP. HTTP/WS: 33 request PASS, hai replica độc lập, thấy bài/bình luận mới.
- Identity 3 PASS; hai mutant không tương đương ACL friends và media_checked
  đều đỏ đúng assertion dự đoán. Harness và source cùng SHA `f496af16`, dùng
  overlay ngoài checkout; không sửa source đang đo.
- Browser bản cuối: feed200, 0 lỗi JS; đã mở phone/desktop/composer-focus,
  finish reviewer Impeccable SHIP riêng phần tích hợp frontend.

Lượt full gate trước sửa frontend/metadata ở `0ec19fa9`: API 3.799 PASS,
804 SKIP; migration/pinned-import/docker PASS; parity baseline dev có
354 scenario, 10.836 bước, 0 sai khác, DB/media lane bật, 199 route Go.
Full gate bị dừng chủ động sau baseline ở vòng canary mở rộng để chốt
bàn giao theo yêu cầu; không có kết luận PASS cho full parity, prod parity,
full PostgreSQL/Python/Go, e2e/chat-e2e/crypto ở lượt sau merge. Native gate
bỏ qua vì runner không có Maestro trên PATH; demo-watch/hero-walk không
có demo8099. Android cộng đồng đã chạy thật ở lượt trước merge, không
được thay bằng bundle pass hay nói đã chạy lại trên SHA mới.

Các commit sau SHA code cuối chỉ chốt tài liệu/ảnh đã mở và số kiểm chứng.
Guard tree/range sẽ chạy trên commit bàn giao; không gọi bản này là full
gate xanh hoặc đã đạt mọi yêu cầu tải/AI/native. Cờ mặc định vẫn tắt; local
8199 đang giữ bản runtime đã kiểm chứng trước merge, không tự thay bằng
binary mới chỉ vì push. Chưa có model thì bài public giữ pending như đã
thống nhất. Burst20k/soak24h/native iOS/frame timing vẫn là các cổng mở.
