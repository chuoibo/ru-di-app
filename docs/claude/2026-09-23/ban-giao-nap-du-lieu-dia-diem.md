# Bàn giao: nạp dữ liệu địa điểm thật vào RuDi

- Ngày: 2026-09-23
- Nhánh: `claude/p0-w31-nap-du-lieu-dia-diem` (worktree `~/wt-ingest`)
- Commit cuối có code: `6dbf71bc`
- Kế hoạch đã được leader duyệt: [`ke-hoach-nap-du-lieu-dia-diem.md`](ke-hoach-nap-du-lieu-dia-diem.md), bản sao nguyên văn của `~/.claude/plans/l-n-plan-v-ph-i-staged-newt.md`
- Người làm tiếp đọc mục 4 (việc dở) và mục 5 (cổng chưa chạy) trước khi làm gì khác

## 1. Việc này là gì

App trước đây chỉ có 12 địa điểm seed tự bịa. Bên data (`/home/lakiet/automate`,
pipeline `vnlocal-ai`) đã crawl khoảng 6.500 địa điểm thật, kèm khung hình cắt từ
video TikTok/Threads. Nhánh này dựng đường nạp dữ liệu đó vào Postgres của ta
(Go, `services/core`), phơi nó qua `/places`, và làm cho màn hình mobile không bể
khi gặp dữ liệu thật.

Hợp đồng dữ liệu là `place.v1` (NDJSON + manifest có sha256), thống nhất với
session bên data. Các lô nằm ở
`/home/lakiet/automate/vnlocal-ai/data/export/place-v1/`. Mới nhất là `0007`
(6.549 dòng, sha256 `9f216015…b95f0e`). Nhánh này đã chạy thật với `0006`.

## 2. Quyết định của leader (không mở lại)

| Chủ đề | Quyết định |
|---|---|
| Ảnh | Nhập ảnh đang có và dùng ngay. **Chưa xét giấy phép khi develop.** |
| Toạ độ | Bên data geocode bằng agy; ta dựng khung trước, dữ liệu điền sau |
| Ngưỡng nhận | Chỉ dòng đã review xong |
| Chữ | Chỉ văn tổng hợp, bỏ trích dẫn nguyên văn (`bang_chung`, `trich_dan` bị từ chối) |
| ADR | Không mở ADR cho việc này, leader tự quyết |
| Điểm đến | 34 tỉnh (33 có hộp bao) + giữ 15 điểm đến du lịch cũ |
| Đường giao | Kafka ngay từ đầu (topic `ai.places.v1`), còn backfill lô đầu đi bằng file |
| Ảnh ở production | MinIO ở mốc M5 |
| Production | Làm song song hạ tầng deploy (W2) |

## 3. Đã xong

### Commit trên nhánh (cũ → mới)

| Commit | Nội dung |
|---|---|
| `6c9e6fc4` | Sườn: migration Go 7 bảng (`internal/ingest/schema.sql`, khuôn chatv2, advisory lock), Alembic `b3f19c7d2a04` (lat/lng nullable, `geo_precision`, `province_code`, `source_updated_at`, `status`/`superseded_by`, `source='vnlocal'`, `content_sha256`) |
| `d13b3b62` | Hợp đồng `place.v1` + bộ kiểm từng dòng, kèm mã từ chối (`place_v1.go`) |
| `f967fee8` | Tách «có toạ độ» (`HasPoint`) khỏi «vẽ được lên bản đồ» (`MappablePoint`) |
| `d1bca497` | Đếm khoá lạ cả trong túi `geo` |
| `dc509e5e` | Buồng đệm: `land` hạ JSONB nguyên văn vào `ingest_place_raw`, mỗi đợt một giao dịch |
| `a69b9cb0`, `11094167` | `mon_an` (đặc sản vùng): giữ dòng nhưng KHÔNG bao giờ có toạ độ |
| `a61ed12f` | Bước chiếu `apply` sang `places` + `place_source_post`, chặn bản cũ đè bản mới bằng `source_updated_at` |
| `ea1493b0` | 33 điểm đến cấp tỉnh + hộp bao (Bắc Ninh 24 chưa có hộp nên dòng của nó bị từ chối có mã) |
| `9ed2e658` | Ba tầng hợp đồng lệch pha (CHECK DB / Literal Pydantic / kiểu TS), kèm cổng `test_wire_vocabulary_matches_the_database.py` |
| `6dbf71bc` | Lượt e2e native: FE không còn bể với dữ liệu thật (chi tiết ngay dưới) |

### Lượt e2e native (`6dbf71bc`)

Đo trên máy ảo `rudi-qa3` (emulator-5556), lô 0006 (6.476 dòng hạ, 6.454
`vnlocal` vào `places`) và 5.105 ảnh của TP.HCM (1.945 địa điểm, 623,7 MB).

| Lỗi thấy trên máy | Đã sửa |
|---|---|
| Khám phá gắn cả 2.391 dòng TP.HCM vào một ScrollView; khung hình treo 11,5 s, Java heap 167/191 MB, **lowmemorykiller giết app ở foreground** | Hiện từng đợt 20 dòng + nút «Xem thêm 20 nơi» (`HANG_MOI_LUOT`, `cauXemThem` trong `rudi/kham-pha/dia-diem.ts`). Đo lại 5 vòng chi tiết → quay lại: Java heap 28–42 MB, app sống |
| Đổi điểm đến xong vẫn thấy danh sách thành phố cũ (khoảng 20 s) dưới tên thành phố mới | Hiện khung đang tải khi id đã lưu khác id đang hiện (`dangHien` trong `ExploreLive.tsx`) |
| 1.945 chỗ có ảnh mà thẻ nào cũng hiện ô biểu tượng (cổng bìa đòi tác giả + giấy phép) | `anhBiaThe` cho ảnh `source='vnlocal'` lên thẻ, kèm dòng «Ảnh từ bài đăng mạng xã hội»; nguồn khác giữ luật cũ |
| Dưới dải ảnh in «Ảnh có giấy phép… từ Wikimedia Commons», sai với khung hình TikTok | `cauNguonAnh()` dựng câu từ chính các ảnh đang hiện: «Ảnh lấy từ bài đăng trên TikTok. Không phải ảnh do nơi này cung cấp.» |
| Bộ chọn điểm đến in «Thành phố Hồ Chí Minh» hai lần | `dongPhuDiemDen` bỏ tỉnh khi tỉnh trùng tên; màn không vẽ dòng rỗng |
| Trạng thái rỗng viết cứng «Rủ Đi mới biết mười lăm nơi» trong khi có 48 | Đếm từ danh sách |
| Go: `cannot scan NULL into *float64` / `*string` | lat/lng và photo author/license là con trỏ trên toàn đường Go (repo, `places_wai`, `meet`, `social_map`, `memory_wall`, `outing_store`) |

Không bể: tên dài nhất (78 ký tự) xuống 4 dòng ở màn chi tiết; địa chỉ và nhãn
dài cắt «…» một dòng trên thẻ; mô tả dài xuống dòng bình thường; ảnh dọc
576×1024 cắt giữa trong khung ngang vẫn đọc được.

Finish reviewer (impeccable, context mới) chấm **fix**. Đã sửa cách viết câu
nguồn («của người dùng» dễ hiểu thành người dùng Rủ Đi) và điều kiện so id của
khung đang tải. Việc còn mở ghi ở mục 4.

**Ảnh chụp của lượt này đã mất**: `/tmp` bị xoá khi WSL khởi động lại (mục 6).
Người làm tiếp phải chụp lại trên commit này.

## 4. Chưa làm / bỏ dở (ưu tiên từ trên xuống)

1. **Chạy đủ cổng trên `6dbf71bc`** (mục 5). Chưa có lượt nào chạy trọn trên
   commit này.

2. **Nút «Chỉ đường» và bản đồ hành trình dẫn tới tâm tỉnh.** Dòng có
   `geo_precision` là `province_centroid` hoặc `suy_luan` vẫn mang lat/lng (vì
   `HasPoint` đúng), nhưng không được vẽ (`MappablePoint` sai). Phía client
   chưa phân biệt hai điều này. Có hai chỗ: `duongChiDuong` trong
   `rudi/kham-pha/dia-diem.ts` dùng lat/lng nếu có, và `choHopLe` trong
   `rudi/hanh-trinh/chieu.ts` chỉ kiểm `Number.isFinite`. Hậu quả: chỉ đường
   tới tâm tỉnh, và cắm ghim chồng lên tâm tỉnh. `parsePlace` đã đọc sẵn
   `geoPrecision`, nên sửa bằng một hàm dùng chung: chỉ `rooftop`, `street`,
   `ward_centroid` mới coi là vẽ được. Lô 0006: 6.330 dòng có toạ độ, chỉ 4.558
   dòng vẽ được.

3. **Chiếu lô 0007: địa chỉ hiển thị.** `project.go` hiện chỉ đọc `dia_chi`,
   nên 3.595 dòng có địa chỉ thật đang hiện «Chưa có địa chỉ». Lô 0007 thêm ba
   khoá `dia_chi_hien_thi`, `dia_chi_hien_thi_dang`, `dia_chi_hien_thi_nguon`.
   Phân bố `dang`: `so_nha` 2.985 · `ten_duong` 2.360 · `moc` 238 ·
   `chi_vung` 698 · null 268. Luật đã thống nhất với bên data:
   - hiện `dia_chi_hien_thi` khi dạng ∈ {so_nha, ten_duong, moc}
   - `chi_vung` thì null (đó là vùng, không phải địa chỉ)
   - thiếu khoá (lô cũ) thì lùi về `dia_chi`
   - không chuỗi nào trong số này là địa chỉ đã xác minh (`dia_chi_xac_nhan`
     chỉ đúng ở 197 dòng), nên không dùng để dẫn đường

   Thêm ba khoá vào `knownKeys` trong `place_v1.go`; hiện bộ dò đang báo trôi
   ở mọi dòng của 0007. Sửa luôn `TestTheProjectionRefusesToCallAnAreaAnAddress`
   cho khớp luật mới.

4. **Nhãn loại hiển thị không dấu** («banh mi thit nuong», «san khau kich»).
   Bên data xác nhận `category` được LLM sinh không dấu **cố ý**
   (`services/place-critic/.../schema.py:203`), nên không có bản gốc có dấu
   để khôi phục. Đề xuất (đã đo trên 0007, chưa code):
   - lấy tối đa 3 tên món có dấu từ `review.mon_phai_thu[].ten`: có ở
     3.620/6.549 dòng, độ dài trung vị 17 ký tự, p90 29 ký tự
   - nếu không có món thì dùng `tom_luoc.loai_noi_nay`, với điều kiện không có
     `_` và không quá 30 ký tự: 6.267 dòng đạt. Có 282 dòng xấu, gồm slug
     `hoat_dong` và câu bình luận LLM dài 87 ký tự
   - còn lại để rỗng

   Ràng buộc: `kinds` còn đi vào suy gu nhóm (`app/domain/preferences.py` và
   `internal/domain/preferences` phía Go, có oracle parity). Nếu thay nội dung
   `kinds` thì khoá gu đổi theo. Nếu giữ `kinds` làm khoá thì cần thêm một
   trường hiển thị trên wire, sửa cả Go lẫn Python cho khớp byte (cổng parity).
   Hãy chọn một trong hai và ghi lý do vào commit.

5. **Picker gắn địa điểm trong kèo (`screens/keo/OutingLive.tsx`).**
   - Gọi `docDanhMuc()` **không kèm điểm đến**, nên chỉ thấy danh mục mặc định
     (Đà Lạt, 8 dòng seed). Không gắn được địa điểm thật vào chặng.
   - `danhMuc.map(...)` render toàn bộ thành chip: nếu danh mục mặc định thành
     một tỉnh lớn thì đây chính là lỗi lowmemorykiller lặp lại.

   `ky-niem/GroupWallLive.tsx` đã có khuôn đúng (ô tìm + tối đa 12 chip). Nên
   dùng lại khuôn đó và đọc điểm đến đã lưu (`docDanhMucCoLui(docDiemDenDaChon())`).
   GroupWallLive cũng đang gọi `docDanhMuc()` không kèm điểm đến.

6. **Nút Nếp nổi đè tên quán ở cặp thẻ so sánh đầu danh sách** (reviewer phát
   hiện). Vị trí nút thuộc lane dock Nếp (session `nep-floating-assistant`),
   không sửa ở đây; cần phối hợp.

7. **Hai TP.HCM trong bộ chọn điểm đến**: «TP. Hồ Chí Minh» (điểm đến cũ, 4
   dòng seed) và «Thành phố Hồ Chí Minh» (tỉnh, 2.391 dòng thật). Tương tự với
   Hà Nội, Đà Nẵng, Huế, Cần Thơ, Ninh Bình. Đây là hệ quả của quyết định «34
   tỉnh + 15 điểm đến cũ». **Cần leader quyết**: gộp, ẩn điểm đến cũ trùng
   tỉnh, hay đổi tên cho phân biệt.

8. **«Món phải thử» và các khối review không lên màn hình.** Feed gửi `review`
   là object (`mo_ta_tong_quan`, `mon_phai_thu`, `diem_manh`, `luu_y`,
   `khung_gio_dep_nhat`…). Ta ghi nó vào cột `reviews` (vốn là JSONB list), và
   wire trả `reviews: []`. Hiện chỉ `description` (= `mo_ta_tong_quan`) được
   hiện. Cần thiết kế cách hiện, không làm phẳng thành text.

9. **Hiệu năng `/places`**: TP.HCM trả 1,5 MB và client đọc lại **mỗi lần màn
   Khám phá được focus** (quay lại từ chi tiết cũng đọc lại). Máy chủ trả trong
   0,4 s; phần chậm nằm ở phía máy. Cân nhắc phân trang hoặc giới hạn phía máy
   chủ, và bỏ đọc lại khi điểm đến không đổi.

10. **Theo kế hoạch, chưa bắt đầu**:
    - consumer Kafka `ai.places.v1` (`twmb/franz-go`), đổ vào cùng `ingest_place_raw`
    - M5 MinIO: tách `Store` interface, driver S3 tự viết SigV4, cổng
      conformance, flip 6 route ảnh sang `owner: go` **trước** khi bật S3,
      `pending_object_deletes` + reaper
    - M8 `rudi-reconcile`
    - W2 deploy
    - M0 còn nửa: comment sai «The store is content-addressed» ở
      `parity/internal/mediasnap/mediasnap.go:58`
    - JSON Schema cho `place.v1`: chưa có file, bộ kiểm Go là nguồn duy nhất

11. **Cần leader quyết, còn treo**: chạy agy thêm khoảng 18 giờ quota để
    geocode 1.750 dòng `province_centroid`? Bên data khuyên dồn quota cho các
    dòng chưa lên bản đồ. Và có đưa nhánh này thẳng vào `main` theo ADR-0032
    không, hay review qua PR như lần này leader yêu cầu.

12. **Chất lượng dữ liệu (báo bên data, không sửa ở đây)**: một số khung hình
    mang chữ in cứng của người đăng («Lưu ý: bạn nên xem đến cuối video…»);
    `source_url` có dạng `tiktok.com/@/video/…` (thiếu handle); nhiều dòng TP.HCM
    thực ra ở Vũng Tàu, đúng với địa giới 34 tỉnh mới nhưng có thể làm người
    dùng bất ngờ.

## 5. Cổng: đã chạy / chưa chạy trên `6dbf71bc`

| Cổng | Trạng thái |
|---|---|
| `gofmt -l` | 0 file lệch |
| `go vet ./...` và `go vet -tags postgres ./...` | sạch |
| `go test` đơn vị (`ingest`, `repo`, `routes`, `cmd`) | xanh |
| `npx tsc --noEmit` | sạch |
| `node --test` bốn file liên quan (`mac-dinh-am-tham-id`, `rudi-kham-pha-dia-diem`, `rudi-diem-den`, `kham-pha`) | 72/72; hai đột biến đều bị bắt |
| `npm test` đầy đủ | lượt trước commit: 949/950. Ca đỏ (cổng id-mặc-định) đã sửa bằng danh sách cho phép có lý do. **Chưa chạy lại trọn bộ** |
| `scripts/go_postgres_tier.sh` | **bị cắt giữa chừng** vì WSL khởi động lại. Lượt sạch gần nhất (trước `photos.go`): 102 gói ok |
| `pytest services/api/tests tests` | **chưa chạy trên commit này**. Lượt trước: 3.328 qua, 1 đỏ là `test_motion_gate_rejects_corrupt_measurements_and_restores_settings`; chạy riêng thì xanh trong 12 s (nhạy tải máy) |
| `repo_guard.py staged` | sạch |

## 6. Cảnh báo môi trường: WSL khởi động lại

`journalctl --list-boots` cho thấy ba lần khởi động lại trong ngày 23/09: kết
thúc boot lúc 11:22, 20:05 và khoảng 21:38. Cả ba lần đều rơi vào lúc máy đang
chạy việc nặng: pytest đầy đủ, go postgres tier, máy ảo Android, nhập ảnh, cộng
stack của các session khác. Lần thứ ba chỉ còn go tier đang chạy, với 7 GB RAM
trống, nên **nguyên nhân chưa được chứng minh**. Mỗi lần khởi động lại mất sạch
`/tmp` (scratchpad, ảnh chụp, env), container DB `--rm` và mọi tiến trình nền.
Cách làm an toàn:

- Chạy tuần tự: cổng → commit → dựng stack → máy ảo. Kiểm `free -g` trước việc nặng.
- Log và bản sao lưu để ngoài `/tmp`. Tôi dùng `~/.wip-ingest/`, trong đó có
  `gates.sh` chạy đủ cổng tuần tự.

## 7. Dựng lại stack e2e (công thức đã chạy)

```bash
SP=~/.wip-ingest/stack; mkdir -p $SP/media
MAN=/home/lakiet/automate/vnlocal-ai/data/export/place-v1/manifest-0006-20260923T015147Z.json
NAME=rudi-e2e-$$
docker run -d --rm --name $NAME -e POSTGRES_PASSWORD=mobile-dev-only -e POSTGRES_USER=mobile \
  -e POSTGRES_DB=mobile -p 127.0.0.1:0:5432 postgres:16-alpine
PORT=$(docker port $NAME 5432/tcp | head -1 | sed 's/.*://')
DSN="postgresql+psycopg://mobile:mobile-dev-only@127.0.0.1:${PORT}/mobile"
(cd ~/wt-ingest/services/api && MOBILE_DATABASE_URL="$DSN" alembic upgrade head)
# seed 12 dòng + 15 điểm đến cũ: app.places.seed_catalog.seed_place_catalog(session)
(cd ~/wt-ingest/services/core && go build -o $SP/rudi-ingest ./cmd/rudi-ingest && go build -o $SP/core ./cmd/core)
MOBILE_DATABASE_URL="$DSN" $SP/rudi-ingest migrate          # 34 tỉnh · 33 điểm đến cấp tỉnh
MOBILE_DATABASE_URL="$DSN" $SP/rudi-ingest land --manifest $MAN
MOBILE_DATABASE_URL="$DSN" $SP/rudi-ingest apply --batch 0006-20260923T015147Z   # chạy lần hai phải không đổi gì
MOBILE_DATABASE_URL="$DSN" MOBILE_MEDIA_ROOT=$SP/media $SP/rudi-ingest photos \
  --batch 0006-20260923T015147Z --frames-root /home/lakiet/automate/vnlocal-ai/data/frames --province 79 --per-place 3
# Go core 8124, Python upstream riêng (uvicorn app.api.main:app) với MOBILE_AUTH_MODE=dev
# core env: MOBILE_CORE_LISTEN, MOBILE_PYTHON_UPSTREAM, MOBILE_DATABASE_URL, MOBILE_AUTH_MODE=dev,
#   MOBILE_INTERNAL_TOKEN, MOBILE_PERSON_ID_KEY, MOBILE_BRAIN_URL, MOBILE_MEDIA_ROOT, MOBILE_OTP_DEBUG_CODE=246810
```

Mobile:
- Metro: `EXPO_PUBLIC_API_URL=http://localhost:8124 npx expo start --dev-client --localhost --port 8131`
- Máy ảo: `export ANDROID_ADB_SERVER_PORT=5038`, rồi
  `adb -s emulator-5556 reverse tcp:8131 tcp:8131` và
  `adb -s emulator-5556 reverse tcp:8124 tcp:8124`
- Mở app: deep link `rudi://expo-development-client/?url=http%3A%2F%2Flocalhost%3A8131`
- Đăng nhập OTP bằng mã debug `246810`. Deep link `rudi://explore` và
  `rudi://destinations` mở thẳng hai màn này.
- Luôn truyền `-s emulator-5556`. Máy ảo của lane khác là 5554.
- Lái máy bằng `input tap` + `uiautomator dump` + `screencap`.
- Luôn **mở ảnh chụp ra nhìn**.

## 8. Tệp chính

- Go: `services/core/internal/ingest/` (`place_v1.go`, `project.go`,
  `apply.go`, `land.go`, `photos.go`, `provinces.go`, `province_boxes.go`,
  `schema.sql`) và `services/core/cmd/rudi-ingest/main.go`
- Alembic: `services/api/app/db/migrations/versions/b3f19c7d2a04_danh_muc_nhan_nguon_ngoai.py`
- Client: `apps/mobile/src/screens/kham-pha/places.ts` (parser wire),
  `src/rudi/kham-pha/dia-diem.ts`, `src/rudi/kham-pha/diem-den.ts`,
  `src/rudi/screens/explore/{ExploreLive,PlaceDetailLive,DiemDenScreen}.tsx`,
  `src/rudi/ui/ghi-cong.ts`
- Liên lạc bên data: session `automate-*` trong `/home/lakiet/automate`. Hỏi
  thẳng khi không rõ dữ liệu; họ trả lời bằng số đo.
