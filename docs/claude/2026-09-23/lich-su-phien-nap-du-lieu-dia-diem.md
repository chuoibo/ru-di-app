# Lịch sử phiên: nạp dữ liệu địa điểm thật cho RuDi (22–23/09/2026)

File này để **nạp vào agent làm tiếp**. Nó kể lại cả phiên: người dùng giao gì,
tôi đã làm gì theo thứ tự, quyết định nào đã chốt, dữ liệu thật trông ra sao,
tôi đã sai ở đâu, và phiên dừng ở trạng thái nào. Danh sách việc dở chi tiết
nằm ở [`ban-giao-nap-du-lieu-dia-diem.md`](ban-giao-nap-du-lieu-dia-diem.md);
kế hoạch gốc đã duyệt nằm ở [`ke-hoach-nap-du-lieu-dia-diem.md`](ke-hoach-nap-du-lieu-dia-diem.md).

- Session: `d1bcf336-02bf-448f-85ec-9617083b3f39`, tên hiển thị `import-real-location-data-production`
- Repo: `/home/lakiet/mobile`, worktree làm việc `~/wt-ingest`
- Nhánh: `claude/p0-w31-nap-du-lieu-dia-diem`
- PR: https://github.com/chuoibo/ru-di-app/pull/645
- Commit cuối trước file này: `c8a2c8bc`

## 1. Tóm tắt một đoạn

App RuDi (Rủ Đi) chỉ có 12 địa điểm seed tự bịa. Người dùng (leader) giao việc
dựng một hệ thống production-ready nạp dữ liệu địa điểm thật (quán ăn, chỗ vui
chơi) mà pipeline crawl ở `/home/lakiet/automate` (`vnlocal-ai`) đã thu thập, và
phối hợp trực tiếp với session bên data. Tôi đã:
- chốt hợp đồng `place.v1` với bên data
- lập kế hoạch kiến trúc (được duyệt)
- dựng đường nạp bằng Go: migration, `land`/`apply`/`photos`, 34 tỉnh
- sửa ba tầng hợp đồng wire bị lệch
- nạp thật lô 0006 (6.454 địa điểm, 5.105 ảnh TP.HCM)
- test end-to-end trên máy ảo Android. Tìm thấy một lỗi làm app bị hệ điều hành
  giết và vài lỗi hiển thị; đã sửa hết.

Phiên kết thúc bằng commit, push, PR #645 và tài liệu bàn giao. Máy WSL khởi
động lại ba lần trong ngày 23/09, nên vài cổng kiểm chưa chạy trọn trên commit cuối.

## 2. Những người và phiên liên quan

| Ai | Vai trò |
|---|---|
| Người dùng | Leader, người quyết mọi thứ. Viết tiếng Việt không dấu câu, ngắn. Muốn thấy app chạy thật trên máy, không tin dấu xanh của test |
| Session bên data | Lúc đầu tên «Crawled data storage», id `b1d0449d-871f-4249-b2fe-8856adb43c27`, sau hiện là `automate-84`. Giữ kho crawl `/home/lakiet/automate`. Trả lời bằng số đo, và đã sửa tôi ba lần bằng bằng chứng |
| Tiến trình song sinh | Tiến trình `claude` thứ hai mang **cùng session id** (sinh 22/09 23:09, socket 6693, ref `c82986`). Nó làm trong cùng `~/wt-ingest`; theo phiên chat-backend, `9ed2e658` là do nó commit, tôi chưa tự kiểm. Hai bên không biết nhau cho tới khi phiên `chat-backend-go-migration` phát hiện. Sau khi máy khởi động lại, nó không còn |
| `chat-backend-go-migration` | Lane chat. Dùng máy ảo `rudi` (emulator-5554); phát hiện chuyện hai tiến trình trùng id |
| `nep-floating-assistant` / `ai-engine-unification` | Lane dock Nếp. Nút Nếp nổi thuộc lane này |

## 3. Dòng thời gian yêu cầu của người dùng và việc tôi làm

**22/09 15:08**: «hiện tại ở session này sẽ là nơi ta bàn luận về cách nạp data
production ready stable cho production… repo hiện tại chỉ có data synthetic tự
tạo chứ chưa hề nạp data mới chuẩn»
- Việc: khảo sát hiện trạng. DB có 12 dòng `places` đều là `source='seed'`, và 2 `destinations`.

**22/09 15:14**: dán id session bên data, rồi viết: «tôi muốn bạn giao tiếp với
agent bên này để hiểu cấu trúc data họ đang lưu như nào, họ crawl data làm sao,
bên ta sẽ phải ingest data như nào, infra ra làm sao, SDLC như nào, CRUD như
nào… thảo luận về system design để build một system stable cho production»
- Việc: trao đổi trực tiếp với session bên data qua SendMessage. Chốt hợp đồng
  `place.v1` (NDJSON + manifest có sha256, `dot_seq`/`dot_truoc`, `kieu_dot`).
- Việc: kiểm độc lập lô `0001` (6.113 dòng, sha256 khớp manifest, 19.704
  frame, 0 dòng lộ `bang_chung`).
- Bên data sửa tôi ba chỗ bằng bằng chứng:
  - một post nằm ở nhiều địa điểm là **lỗi gom cụm**, không phải chuyện bình thường
  - `bo_qua` là lỗi viết, không phải phán quyết chất lượng
  - 51 file WebP và 12 file PNG mang đuôi `.jpg`

**22/09 15:25**: trả lời bốn câu hỏi chọn:
- Ảnh: «cứ nhập ảnh cho tôi sử dụng ảnh hiện tại đang có ưu tiên cho tôi»
- Toạ độ: «nhờ bên họ chạy hết long lat bằng agy services cho tất cả các địa điểm sau đó sử dụng sau nhé»
- Ngưỡng: «Đã review xong»
- Chữ: «Chỉ nạp văn tổng hợp, bỏ trích nguyên văn»

**22/09 15:30**: bác bốn đề xuất của tôi:
- ADR: «bỏ cái này đi tôi quyết»
- Điểm đến: «bên kia đang update thêm long lat cho ta, cứ set up hết trước đi đã,
  rồi sau này data đủ thì sẽ update vô… giờ ta làm sườn adapt hết đi»
- Giấy phép: «bỏ giấy phép gì đi chưa cần quan tâm ta develop đã»
- Còn lại: «cứ làm set up hết đi đã rồi ta sẽ update sau khi họ filled đủ»

**22/09 (xen giữa)**: «không rõ gì về data nhắn tin hỏi session bên kia cho rõ nhé»
- Từ đó mọi câu hỏi về dữ liệu đều đi qua session bên data; không đoán.

**22/09 15:40**: «lên plan… system design, architecture, techstack, sync data từ
bên crawl qua bên ta, database lưu trữ làm sao, SDLC, tính lâu dài và consistent
của dữ liệu, CRUD… hình ảnh store ở minio… ultrathink»
- Việc: viết kế hoạch đầy đủ. Kiến trúc buồng đệm (dữ liệu thô JSONB nguyên
  văn, bước chiếu chạy lại được), danh tính neo vào (platform, post_id), CRUD
  theo nguồn sở hữu, không xoá cứng, ảnh giữ khoá ngẫu nhiên (không content-addressed),
  MinIO sau interface kèm cổng conformance, các mốc M0–M8 và luồng W2.

**22/09 15:53**: bốn câu chọn tiếp:
- Production: «Làm song song»
- Điểm đến: «34 tỉnh + 15 điểm đến cũ»
- Đường giao: «Kafka ngay từ đầu»
- MinIO: «Gắn ở M5, cùng lúc nạp ảnh»

Kế hoạch được duyệt qua ExitPlanMode.

**22/09 16:10**: «continue»
- Việc (đêm 22 → rạng sáng 23/09), 9 commit từ `6c9e6fc4` tới `ea1493b0`:
  - migration Go 7 bảng
  - Alembic `b3f19c7d2a04`
  - bộ kiểm `place.v1`
  - tách «có toạ độ» khỏi «vẽ được»
  - `land` + `apply`
  - xử lý `mon_an`
  - 34 tỉnh + 33 hộp bao

**23/09 01:56**: «test e2e trước, bật app lên test thật sự xem nó đã vận hành
đúng với cách ta expected chưa»

**23/09 02:59**: «ý là đã mở app ra test e2e chưa»
- Việc: nạp lô 0006 vào DB thật, bật API, và **thấy HTTP 500**. Có ba lỗi cùng
  một họ, do một từ vựng enum sống ở ba chỗ (CHECK của DB, Literal của
  Pydantic, kiểu TS) và ba chỗ đó lệch nhau.
- Commit `9ed2e658` sửa ba lỗi đó, kèm cổng
  `test_wire_vocabulary_matches_the_database.py`, cổng này được kiểm bằng đột biến.
- Mở app bằng bản web và thấy dữ liệu thật.

**23/09 (khoảng 03:00–03:30)**: «phải test end to end app trên native nhé, xem
khi gắn data vào hình ở các địa điểm rồi chữ ở các quán có bị broken có bị
overflow không, phải rà kĩ test trên app mobile nhé, xem FE có bị bể khi nạp
data đúng không»
- Việc: dựng máy ảo Android riêng `rudi-qa3` (emulator-5556), Metro 8131, Go
  core 8124, nạp ảnh TP.HCM (`rudi-ingest photos`). Đăng nhập bằng OTP debug,
  rồi lái máy bằng `adb input` + `uiautomator dump` + `screencap`, mở từng ảnh
  chụp ra nhìn. Kết quả ở mục 6.
- Chạy impeccable-pipeline (harden) cho phần sửa UI, cộng một finish reviewer
  context mới, người này chấm «fix».

**23/09 12:57 và 14:34**: «continue» (sau hai lần máy khởi động lại)
- Việc: dựng lại stack, chạy cổng. Máy lại khởi động lại giữa chừng.

**23/09 14:58**: «tạm thời bạn đang làm gì thì commit lên push hết lên nhánh, tạo PR,
rồi để lại 1 handoff docs là bạn đang làm gì, đang chưa làm gì, bỏ dở gì, sau
đó sẽ có 1 agent khác làm tiếp việc của bạn nhé»
- Việc: chạy kiểm tra nhẹ rồi commit `6dbf71bc` (sửa FE + Go) và `c8a2c8bc`
  (tài liệu bàn giao + bản sao kế hoạch). Push, tạo PR #645.

**23/09 16:58**: yêu cầu tổng hợp cả phiên thành file lịch sử này và push lên PR.

## 4. Quyết định đã chốt (không mở lại nếu leader không mở)

| Chủ đề | Quyết định |
|---|---|
| Ảnh | Dùng ảnh đang có ngay. **Chưa xét giấy phép khi develop** |
| Toạ độ | Bên data geocode bằng agy; ta dựng khung chịu được dòng thiếu toạ độ |
| Ngưỡng | Chỉ dòng đã review xong |
| Chữ | Chỉ văn tổng hợp; từ chối dòng có `bang_chung`, `trich_dan`, `nhan_xet_lap_lai` |
| ADR | Không mở ADR, leader tự quyết |
| Điểm đến | 34 tỉnh + giữ 15 điểm đến du lịch cũ |
| Đường giao | Kafka `ai.places.v1` (Redpanda `localhost:19092`, compact, 6 partition, key = place_id) cho dòng chảy; backfill lô đầu bằng file |
| Ảnh production | MinIO ở mốc M5 |
| Production | Làm song song hạ tầng deploy (W2) |
| Quy tắc repo | Go cho backend, Python chỉ cho AI (ADR-0031). Commit viết tiếng Việt, comment viết tiếng Anh |

## 5. Dữ liệu thật trông ra sao (đã đo)

**Các lô** (thư mục `/home/lakiet/automate/vnlocal-ai/data/export/place-v1/`)

| Lô | Số dòng | Ghi chú |
|---|---|---|
| 0001 | 6.113 | 0% có toạ độ |
| 0006 | 6.476 | đã nạp thật |
| 0007 | 6.549 | mới nhất; thêm `dia_chi_hien_thi*` |

**Toạ độ** (lô 0006)
- 6.330 dòng có toạ độ; chỉ 4.558 dòng (70,4%) **vẽ được**, tức có `geo_precision` là `rooftop`, `street` hoặc `ward_centroid`.
- 1.750 dòng `province_centroid`. Muốn chạy thêm agy cho chúng thì tốn khoảng 18 giờ quota; đang chờ leader quyết.
- Các mức `province_centroid`, `suy_luan`, `none` và mọi dòng `mon_an` không bao giờ được vẽ lên bản đồ.

**TP.HCM**: 2.391 địa điểm; qua Go thì 1.927 vẽ được (80,6%). Go và Python trả ra giống nhau tới từng byte (1.344.063 byte).

**Ảnh TP.HCM**
- 5.105 ảnh, 1.945 địa điểm, 623,7 MB.
- Phần lớn khổ dọc 576×1024. Nguồn: TikTok 4.975, Threads 130.
- Có khung mang chữ in cứng của người đăng.

**`mon_an`**: 6 dòng, là đặc sản vùng chứ không phải quán. Giữ dòng, không bao giờ có toạ độ.

**`category`**
- Sinh **không dấu cố ý**. Luật của LLM bên data yêu cầu «chữ thường không dấu», nên không có bản có dấu để khôi phục.
- `phu_hop_voi` và `huong_vi_chu_dao` lẫn cả có dấu lẫn không dấu.

**Nhãn có dấu thay thế**
- `review.mon_phai_thu[].ten` có ở 3.620/6.549 dòng, độ dài trung vị 17 ký tự.
- `tom_luoc.loai_noi_nay` có ở 100% dòng, nhưng rất chung chung («nơi ăn uống» chiếm 2.603 dòng) và có 282 dòng xấu (slug, hoặc câu bình luận của LLM).

**Địa chỉ**
- `dia_chi` chỉ có ở khoảng 30% số dòng.
- `dia_chi_day_du` đôi khi chỉ là tên vùng.
- Lô 0007 thêm `dia_chi_hien_thi_dang` (`so_nha` / `ten_duong` / `moc` / `chi_vung`). Hiện được 85% số dòng, so với 30% nếu chỉ đọc `dia_chi`.
- `dia_chi_xac_nhan` chỉ đúng ở 197 dòng.

**Khác**
- Không có rating thật. `diem_xep_hang_llm` là điểm do model chấm, **không** được hiện thành sao (ADR-0017).
- Nhiều địa điểm «TP.HCM» nằm ở Vũng Tàu, đúng với địa giới 34 tỉnh mới.

## 6. Kết quả e2e native (lô 0006, máy ảo Android)

Lỗi tìm thấy và đã sửa trong `6dbf71bc`:

1. **App bị lowmemorykiller giết khi đang hiện trên màn hình.** Khám phá gắn
   2.391 dòng vào một ScrollView: có khung hình treo 11,5 s, Java heap 167/191 MB.
   Nay danh sách hiện từng đợt 20 dòng kèm nút «Xem thêm». Đo lại: heap 28–42 MB
   qua 5 vòng mở chi tiết rồi quay lại.
2. Ảnh thật không lên thẻ, vì cổng bìa đòi tác giả + giấy phép. Nay ảnh vnlocal
   lên thẻ, kèm dòng «Ảnh từ bài đăng mạng xã hội».
3. Câu nguồn ảnh ghi «Wikimedia Commons» cho khung hình TikTok. Nay câu được
   dựng từ chính các ảnh đang hiện.
4. Đổi điểm đến xong vẫn thấy danh sách thành phố cũ khoảng 20 s. Nay hiện khung đang tải.
5. Bộ chọn điểm đến in tên tỉnh hai lần, và viết cứng «mười lăm nơi».
6. Go: `cannot scan NULL` với lat/lng và photo author/license.

Kiểm rồi, không bể:
- tên dài nhất (78 ký tự) xuống 4 dòng
- địa chỉ và nhãn dài cắt «…»
- mô tả dài xuống dòng bình thường
- ảnh dọc cắt giữa trong khung ngang vẫn đọc được

Tìm thấy nhưng **chưa sửa**: xem mục 4 của tài liệu bàn giao. Quan trọng nhất:
- nút «Chỉ đường» và bản đồ hành trình dẫn tới tâm tỉnh với dòng `province_centroid`
- 3.595 dòng hiện «Chưa có địa chỉ» dù có địa chỉ ở lô 0007
- nhãn loại không dấu
- picker gắn địa điểm trong kèo chỉ đọc danh mục mặc định và render toàn bộ
- nút Nếp nổi đè tên quán
- hai mục TP.HCM trong bộ chọn điểm đến
- «món phải thử» không lên màn hình
- `/places` TP.HCM nặng 1,5 MB và bị đọc lại mỗi lần màn được focus

## 7. Tôi đã sai ở đâu (để agent sau khỏi lặp lại)

- **`mon_an` sai ba lần.**
  - Lần 1: vứt dòng.
  - Lần 2: biến thành quán ăn, vì đọc *tên* trường `dia_chi_day_du` mà không đọc *nội dung*.
  - Lần 3 (đúng): giữ dòng, không cho toạ độ.

  Bên data bác lần 2 bằng bằng chứng: 4/5 dòng `dia_chi` null, và `ly_do_gom` ghi rõ là món đặc sản.
- **Hai số lệch nhau (84 và 85)** vì một dòng `mon_an` bị vứt *trước* khi đếm.
- **Test bằng curl vào Python bị mù**: `/places` là route LIVE-GO, nên phải đo qua Go core.
- **Tưởng tsc sạch nhưng thực ra tsc chưa chạy** (thiếu `node_modules`). Symlink `node_modules` lại làm Metro 404; phải `npm ci` thật.
- **Nói kiểu ở client «không kiểm lúc chạy», sai**: `parsePlace` có kiểm runtime. Đã đính chính.
- **`pkill -f` giết nhầm chính lệnh đang chạy**; phải giết theo cổng bằng `ss`.
- **Commit message ghi sai số liệu**, phải amend.
- **Repo guard chặn**: id TikTok 19 chữ số, id OSM 9 chữ số, toạ độ 9 chữ số trong fixture. Phải dùng id tổng hợp và làm tròn.
- **Vòng chờ «231 giây»**: tôi đo nhầm khi app đang ở màn khác, không phải app chậm.
- **Chạy song song pytest + go tier + máy ảo + nhập ảnh** rồi máy khởi động lại (mục 8).
- **Cổng id-mặc-định (`mac-dinh-am-tham-id.test.mjs`)** bắt biểu thức `daChon ?? destination?.id`. Đây là id dùng để so, không phải để hiện, nên đã thêm vào danh sách cho phép kèm lý do; không lách cổng.

## 8. Sự cố môi trường

- **WSL khởi động lại ba lần** ngày 23/09: kết thúc boot lúc 11:22, 20:05 và khoảng 21:38.
  - Hai lần đầu trùng lúc chạy song song nhiều việc nặng. Lần ba chỉ còn go postgres tier, RAM trống 7 GB.
  - **Nguyên nhân chưa chứng minh.**
  - Mỗi lần mất sạch `/tmp`, container DB `--rm` và mọi tiến trình nền; cây git còn nguyên.
  - Đã ghi vào memory `wsl-sap-khi-chay-song-song-cong-may-ao-va-nhap-anh`.
  - Từ đó, log và bản sao lưu để ở `~/.wip-ingest/`.
- **Hai tiến trình cùng session id** cùng ghi vào `~/wt-ingest`. Đã phối hợp để một bên không checkout/stash mất việc của bên kia.
- **Máy ảo**
  - Của tôi: `rudi-qa3`, emulator-5556, adb server 5038.
  - Lane chat: emulator-5554. Không chạm.
  - Không dùng Maestro khi có hai máy ảo.

## 9. Cách người dùng muốn làm việc (rút ra trong phiên)

- Muốn thấy **app chạy thật trên máy Android**. «Test xanh» không đủ; phải mở ảnh chụp ra nhìn.
- Không rõ dữ liệu thì **hỏi session bên data**, không đoán.
- Quyết nhanh, gạt bớt thủ tục (ADR, giấy phép) để develop trước.
- Mọi việc frontend đi qua skill `impeccable-pipeline` (memory `frontend-luon-dung-skill-impeccable-pipeline`).
- Commit message tiếng Việt, ghi số đo; comment trong code tiếng Anh.
- Khi dừng giữa chừng: commit, push, PR, cộng tài liệu bàn giao cho agent sau.

## 10. Trạng thái cuối

| Commit | Nội dung |
|---|---|
| `6c9e6fc4` … `ea1493b0` | Đường nạp Go (9 commit, 23/09 00:36–01:42) |
| `9ed2e658` | Ba tầng hợp đồng wire + cổng từ vựng (09:28) |
| `6dbf71bc` | Lượt e2e native, FE không bể (22:01) |
| `c8a2c8bc` | Tài liệu bàn giao + bản sao kế hoạch (22:03) |

**Cổng xanh trên `6dbf71bc`**
- gofmt
- go vet (cả `-tags postgres`)
- go test đơn vị
- tsc
- 72/72 ca ở bốn file test mobile liên quan (hai đột biến đều bị bắt)
- repo guard

**Chưa chạy trọn**
- `scripts/go_postgres_tier.sh`: bị cắt vì máy khởi động lại
- `pytest services/api/tests tests` đầy đủ
- `npm test` đầy đủ

**Việc đầu tiên cho agent sau**
1. Chạy đủ các cổng trên, **tuần tự**, trên nhánh này.
2. Làm theo mục 4 của [`ban-giao-nap-du-lieu-dia-diem.md`](ban-giao-nap-du-lieu-dia-diem.md).
3. Dựng lại stack e2e theo mục 7 của tài liệu đó, rồi chụp lại ảnh bằng chứng (ảnh cũ đã mất cùng `/tmp`).
