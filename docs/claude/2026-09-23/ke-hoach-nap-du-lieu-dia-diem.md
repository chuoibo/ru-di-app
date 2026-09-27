# Kế hoạch: hệ nạp dữ liệu địa điểm thật vào production

## Context — vì sao làm việc này

App RuDi gợi ý chỗ ăn chơi ở Việt Nam. **Danh mục trong database hiện có đúng 12 dòng tự bịa** (`select count(*) from places` → 12, toàn bộ `source='seed'`; `destinations` → 2). Đó là vật liệu demo, không phải sản phẩm.

Song song, pipeline crawl ở `/home/lakiet/automate` đã chạy thật hai tuần và tích được **7.904 địa điểm** trong MongoDB, kèm **19.704 khung hình / 2,5 GB**. Hai bên đã trao đổi trực tiếp, chốt hợp đồng `place.v1`, và **lô đầy đủ 6.113 dòng đã nằm trên đĩa, đã kiểm sha256 và toàn vẹn**.

Nguồn này **bất ổn định theo bản chất, không phải do làm ẩu**: tên địa điểm do LLM đặt, lược đồ trôi theo `review_prompt_version`, kho phình 24/7, và cùng một quán sinh ra hai dòng vì LLM gọi hai tên. Toàn bộ thiết kế dưới đây tồn tại để một nguồn như thế **không làm hỏng dữ liệu phía ta**.

---

## Bằng chứng — đã đo, không phỏng đoán

**Lô `0001-20260922T154756Z`**, kiểm độc lập chứ không tin mô tả:

```
sha256 3a85592d…972f0c  khớp manifest từng ký tự
6.113 dòng · 46.347.369 byte · 19.704 frame
0 post thiếu platform · 0 frame thiếu width/height · 0 file ảnh mất
0 dòng rò bang_chung (trích nguyên văn lời người dùng)
content_type: jpeg 19.641 · webp 51 · png 12
7 dòng có trung_lap_voi · 22 dòng thiếu province_code (0,36%)
```

| Mảng | Số đo |
|---|---|
| Toạ độ | 0% trong lô 0001. Geocode đang chạy: **61,2 giây/địa điểm**, 8 ca song song, ETA 13–16 giờ |
| Tỉ lệ geocode ra kết quả | Lô đo 12 chỗ: rooftop 9 · **none 3 (25%)** · `suy_luan` 0 · bị hạ cấp 0 |
| Địa chỉ | 21% có `dia_chi`. Geocode đang lấp: 6/12 chỗ chưa từng có địa chỉ giờ đã có |
| Phủ vùng | Nặng Nam Bộ. Bắc và Trung gần như trắng |
| Độ sâu | 70% địa điểm chỉ dựa trên **một bài đăng** |
| Lưu ảnh phía ta | Volume một container, không backup — chặn **B4**, hạng mất dữ liệu |
| Deploy production | **Không tồn tại** — không fly.toml, không .tf, không script deploy. Chặn **B3** |
| CI | **Chưa khởi động job nào từ 2026-08-29** (billing). Cổng thật là `make gate` local |
| Go deps | 4 dep trực tiếp: pgx, go-redis, coder/websocket, x/text |

**Hệ quả sản phẩm từ tỉ lệ `none` 25%:** nếu giữ nguyên trên toàn bộ thì khoảng **4.500 chỗ lên được bản đồ, 1.600 chỗ không**. Màn hình bản đồ **không được** thiết kế với giả định mọi địa điểm đều có toạ độ.

Ba chỗ tôi hiểu sai và bên kia sửa bằng bằng chứng: post nằm ở nhiều địa điểm là **lỗi gom cụm** (họ mở caption thật: "Bún Thái 640"/"Bún Thái Sư Vạn Hạnh"/"Bún Thái 65 Nguyễn Biểu" là *một* quán) · `bo_qua` là lỗi viết chứ không phải phán quyết chất lượng · 51 file WebP + 12 file PNG đội tên `.jpg`.

---

## Bốn quyết định đã chốt

| | Quyết định |
|---|---|
| **Production** | Làm **song song**: nạp vào stack local theo M1–M8, đồng thời dựng hạ tầng deploy (W2) |
| **Điểm đến** | **34 tỉnh + giữ 15 điểm đến du lịch đã biên soạn** |
| **Đường giao** | **Kafka ngay từ đầu** — topic `ai.places.v1` trên Redpanda bên họ |
| **Ảnh** | **MinIO ở M5**, cùng lúc nạp ảnh. Gỡ luôn chặn B4 |

---

## Nguyên tắc

**P1 — Danh tính không suy từ thứ nguồn có quyền nghĩ lại.** Neo là `(platform, post_id)` do nền tảng cấp.

**P2 — Mọi thứ dẫn xuất phải tính lại được từ thứ đã nhận.** Bất biến 3 của repo (*số dư luôn tính lại được từ sổ*) áp cho địa điểm. Luật ánh xạ một nguồn LLM **chắc chắn sai lần đầu**.

**P3 — Nạp lại không đổi kết quả, thứ tự không phá nhau.** Chặn bằng `source_updated_at`.

**P4 — Không biết thì phải *nhìn thấy được* là không biết.**

---

## Kiến trúc

```
  vnlocal                                    RuDi
  ────────────────────────────               ─────────────────────────────────────────
  export_place_v1.py ──► NDJSON + manifest ──┐
  place-builder ────────► ai.places.v1 ──────┤  rudi-ingest (Go)
                          (Redpanda)         │    1. xác minh nguồn
  data/frames/ 2,5 GB ───────────────────────┤    2. → ingest_place_raw (JSONB nguyên văn)
                                             │    3. ánh xạ → places · place_source_post
                                             │    4. ảnh → sanitize → object store
                                             │    5. hỏng → ingest_reject (mã lý do)
                                             ▼
                                      PostgreSQL 16 ──► đường đọc Go ──► app
                                             ▲
                                      rudi-reconcile (đối soát định kỳ)
```

**Một đường ánh xạ, hai nguồn nạp.** `ingest_place_raw` là khớp nối: file và Kafka đổ vào cùng chỗ, bước 3–5 không biết dữ liệu tới bằng đường nào.

**Kafka client:** `twmb/franz-go` — thuần Go, consumer group đúng, client Redpanda khuyến nghị. Đây là dependency thứ năm của một `go.mod` rất mỏng, cần là quyết định có chủ ý.

**Backfill lô đầu đi bằng file** (44 MiB đã nằm trên đĩa, đã kiểm sha256) — đường ngắn nhất tới dòng dữ liệu thật đầu tiên. Đường Kafka dựng ngay ở M3 cho dòng chảy liên tục.

---

## Lược đồ

### Bảng mới — migration Go, khuôn `chatv2`

`internal/db/db.go:15` ghi thẳng *"Alembic owns the schema. Nothing here issues DDL."* — migration Go **chỉ tạo bảng mới biệt lập**, theo khuôn `internal/chatv2/migrate.go`: `//go:embed schema.sql` + `pg_advisory_xact_lock` + `<pkg>_schema_migrations` giữ digest sha256; gọi từ sub-command `migrate`, **không bao giờ từ đường serve**.

| Bảng | Việc |
|---|---|
| `ingest_batch` | `dot_seq` · `dot_truoc` · `kieu_dot` · `file_sha256` · `rows` · `updated_at_min/max` · `applied_at` |
| `ingest_place_raw` | `(batch_id, line_no)` PK · `payload` JSONB **nguyên văn** · `payload_sha` · `source_key` · `source_updated_at` |
| `ingest_reject` | `batch_id` · `line_no` · `reason` (mã máy đọc) · `detail` |
| `place_source_post` | `(platform, post_id, place_id)` — **nhiều-nhiều** |
| `place_override` | Người sửa tay: `(place_id, field)` + `value` + `actor` + `reason` |
| `admin_province` | 34 tỉnh, lấy **đúng** `crawl-data/seeds/admin_units_v2_34provinces.json` của họ |
| `pending_object_deletes` | Hàng đợi xoá object — xem mục Ảnh |

### Cột thêm vào `places` — Alembic, một head duy nhất

| Cột | Vì sao |
|---|---|
| `lat`, `lng` → **nullable** | 0/6.113 có toạ độ, và ~26% sẽ **không bao giờ** có |
| `geo_precision`, `geo_evidence` | Sáu mức: `rooftop` · `street` · `ward_centroid` · `province_centroid` · `suy_luan` · `none` |
| `province_code` | FK `admin_province` |
| `source_updated_at` | Chặn bản cũ đè bản mới (P3) |
| `status` + `superseded_by` | **Không bao giờ xoá cứng** dòng người dùng đã lưu |
| `confidence`, `evidence_posts` | Lọc lúc **hiển thị**, không lọc lúc **nạp** |
| `source_kind` | Giữ nguyên 7 giá trị `loai` gốc |
| `source` CHECK += `'vnlocal'` | |

`places` **đã có sẵn** `description` (text) và `reviews` (JSONB list) — văn mô tả và `mon_phai_thu` (mảng object) vào thẳng đó, **không làm phẳng thành text**.

`content_sha256 TEXT NULL` + index **không unique** thêm vào `uploaded_images` và `place_photos` — xem mục Ảnh.

### Wire

`places_wai.go:663` phát `lat`/`lng` bằng `pyjson.Float(row.Lat)`, `repo/places.go` scan vào `float64` không con trỏ. Cho nullable ⇒ đổi struct, serializer và parser client. Đây là thay đổi hợp đồng.

### Ánh xạ danh mục 7 → 4

`quan_an` + `khu_am_thuc` → `quan-an-local` · `cafe` → `cafe` · `diem_tham_quan` + `trai_nghiem` → `vui-choi` · `giai_tri` → `vui-choi`, **trừ** bar/pub/club/karaoke/cocktail → `di-choi-dem`. **`mon_an` (4 dòng) bỏ**. Bên kia cảnh báo riêng: "sân khấu kịch"/"kịch nói" đi buổi tối nhưng **không** phải nightlife.

---

## Danh tính và gộp trùng

`place_id` (`plc_…`) **ổn định** — vì nó là khoá chính document trong Mongo, không bao giờ bị ghi lại. Lần nạp thứ hai là **cập nhật**, không nhân bản.

Rủi ro đi hướng ngược: một `plc_` **mới** xuất hiện cho cùng một quán, dòng cũ ở lại.

1. `places.id` do **ta** mint, ổn định vĩnh viễn. `source_ref = plc_…` chỉ để truy ngược.
2. Gộp **chỉ** khi `trung_lap_voi[]` báo trọn tập post giống hệt.
3. **Không đặt khoá ngoại cứng từ `trung_lap_voi`** — nó quét toàn kho nên trỏ được tới `plc_` chưa có trong lô (3/7 dòng của lô này như vậy).
4. Gộp = `status='superseded'` + `superseded_by`, **không xoá**.
5. **"Đã đóng cửa" không bao giờ suy từ việc một dòng vắng mặt.**

---

## CRUD — ai được ghi cái gì

Mỗi dòng thuộc sở hữu của `source` sinh ra nó, và **chỉ writer của source đó được sửa**.

| | Ai | Cách |
|---|---|---|
| Create | Chỉ worker nạp. Người chỉ tạo dòng `curated` | |
| Read | Đường đọc Go, đắp `place_override` lên trên | |
| Update | Worker sửa dòng gốc, gác bằng `source_updated_at`. **Người ghi vào `place_override`** | Nạp lại không phá việc của người |
| Delete | **Không có xoá cứng.** `status`: `active` → `hidden` → `superseded` / `stale` | |

---

## Ảnh, MinIO và cổng parity

### Parity đang so cái gì — và nó khắt khe hơn tôi tưởng

`parity_stacks.sh:211` đặt `MOBILE_CORE_CANDIDATE_ROUTES=ported`, mà 6 route ảnh đều ở trạng thái `PORTED` ⇒ **trong stack candidate, Go ĐANG tự ghi và xoá file ngay bây giờ**. Giả định "route vẫn `owner: python` nên vô hại" của tôi là **sai**.

`parity/internal/mediasnap` chụp **toàn bộ cây filesystem sau mỗi step**, hai bên, rồi so: sha256 nội dung (viết HOA để không bị mask) · size · mode file · **mode của mọi thư mục cha** · layout `kk/kk/key` · **cả tên file tạm thoáng qua** · symlink · mode thư mục sau khi xoá. Cộng canary `media-file-dropped` cố tình xoá một file để chứng minh cổng biết đỏ.

Đổi backend chỉ ở một bên ⇒ mọi entry `created` chỉ còn ở reference ⇒ đỏ tuyệt đối. Và hai điểm dễ nhầm: **làn wire vẫn xanh** (Go đọc S3 trả đúng byte) và **làn database vẫn xanh** (khoá vẫn 32-hex). Wire xanh không cứu được làn media.

### Phương án chọn: fs mặc định + driver S3 sau interface + cổng tương đương

Ba mệnh đề tách bạch, viết rõ để không ai nhầm cái này chứng minh cái kia:

- **P1 — Parity chứng minh Go ≡ Python, ở driver `fs`.** Không sửa một dòng nào trong `parity/`, `parity_stacks.sh`, `e2e_slice.sh`.
- **P2 — Cổng conformance chứng minh driver `s3` ≡ driver `fs` ở mức *hợp đồng quan sát được***: byte vào/ra, `Delete` trả true/false, `ErrInvalidKey`, `fs.ErrNotExist`, thứ tự thao tác. **Không** chứng minh mode bits, inode, tên tempfile — đó là sự thật của filesystem, S3 không có và **không giả vờ có**.
- **P3 — Production chạy `s3`.** P1 ∧ P2 **không** bằng "Go trên S3 ≡ Python trên fs ở mức syscall". Nó bằng: "Go trên S3 trả đúng byte và đúng lỗi mà Python trên fs trả". Đó là mức duy nhất người dùng nhìn thấy và là mức duy nhất trung thực để tuyên bố.

**Layout gương**: object key = **đúng** `k[0:2]/k[2:4]/k`. Migration là copy một-một, rollback là copy ngược.

**Điều kiện tiên quyết, không phải việc làm sau:** flip 6 route ảnh sang `owner: go` **trước** khi bật S3. Nếu không, `MOBILE_FORCE_PYTHON` rollback sẽ trỏ Python vào filesystem rỗng → ảnh 404, rollback biến thành sự cố. Chặn ở khởi động, không viết vào runbook.

### Khoá ảnh: GIỮ ngẫu nhiên, KHÔNG content-addressed

Tôi đã đề xuất khoá theo nội dung `sha256(byte)` để dedup và idempotent. **Bỏ đề xuất đó.** Năm lý do, ba cái đầu là chí mạng:

1. **Vỡ UNIQUE constraint.** `uq_uploaded_images_storage_key` và `uq_place_photos_storage_key` — hai người upload cùng một tấm ảnh → khoá giống nhau → `IntegrityError` **sau khi object đã ghi**. Muốn dedup thật phải bỏ cả hai unique và thêm refcount.
2. **Parity coi khoá trùng là khác biệt phải báo** (`storage_key_test.go:36`). Một dedup-hit xảy ra ở stack này mà không ở stack kia là đỏ không tái lập được.
3. **sha256 hex 64 ký tự vi phạm grammar khoá** 32-hex ở cả Python lẫn Go.
4. **GDPR erasure yếu đi ở chỗ khó biện minh nhất**: người A xin xoá, refcount còn 1 vì B cùng byte → object vẫn nằm đó sau khi A được trả lời "đã xoá".
5. **Khoá thành oracle suy ra được**: ai có byte là suy ra khoá → thăm dò "hệ thống đã có tấm ảnh này chưa".

**Thay bằng:** giữ `secrets.token_hex(16)`, thêm cột `content_sha256` (index **không** unique) + ghi vào `x-amz-meta-sha256`. Được: phát hiện trùng khi import, verify migration, kiểm toàn vẹn định kỳ, đường mở sang dedup-có-refcount sau — mà **không** đụng grammar khoá, unique constraint, `normalize.go`, hay lập luận erasure.

### Thứ tự ghi và xoá

**Ghi: object trước, row sau.** Đây là **hợp đồng có văn bản** (`repo/photos.go:8-11`), không phải tai nạn. Row-trước đổi loại lỗi từ "rác dọn được" sang "row trỏ vào hư không". Đừng "cải tiến".

**Xoá: row trước, object sau — nhưng phải thêm hàng đợi.** Với fs, nuốt lỗi unlink nghĩa là để lại rác. Với S3, nuốt lỗi DeleteObject nghĩa là **ảnh của người đã yêu cầu xoá vẫn nằm trong bucket** — nghĩa vụ chưa hoàn thành, không phải việc dọn dẹp. Cần `pending_object_deletes` ghi **trước** khi thử xoá + reaper + timeout 2–3s trong request.

**Versioning bucket phải TẮT.** Bật "cho an toàn" → object đã xoá sống lại dưới dạng version → erasure không hoàn thành và không ai biết.

**Hai người xoá, không phải một**: `service.py:4559` (erasure) và `story_purge.py:107` (story hết hạn). Quên cái thứ hai = rò ảnh vĩnh viễn.

### Thư viện S3

`go.mod` có đúng 4 dep trực tiếp; `minio-go` kéo thêm ~10 module. Bề mặt cần là PUT/GET/DELETE/LIST + SigV4 ≈ 300 dòng stdlib, và repo có truyền thống tự port (`jpegdec`, `pngenc`, `exif` đều tự viết). **Đề nghị tự viết SigV4**, nhưng phải là quyết định có chủ ý chứ không phải `go get` phản xạ.

### WebP không cần nới CHECK

Sanitizer chỉ xuất JPEG hoặc PNG, nên 51 file WebP là kiểu *đầu vào*, không phải kiểu *lưu trữ*. `place_photo_content_type_allowed` giữ nguyên. `internal/webpdec` đã có.

### docker-compose

Thêm `minio` (image pin digest, volume `mobile-minio-data`, healthcheck) + `minio-init` one-shot tạo bucket, theo khuôn service `migrate`. Ba ràng buộc bắt buộc: **không tạo network mới** (daemon đã cạn subnet pool 2026-09-14) · không publish port trừ khi qua biến · `depends_on: minio-init: service_completed_successfully`.

Mặc định compose giữ `MOBILE_MEDIA_BACKEND=fs` ⇒ `parity_stacks.sh`, `e2e_slice.sh`, stage `parity` và `go-postgres` **không đổi một dòng**.

---

## Nhất quán lâu dài

| Rủi ro | Chặn bằng |
|---|---|
| Đổi tên → nhân bản | Gộp theo `trung_lap_voi` + `superseded_by` |
| Dòng biến mất | Đợt `toan_bo` định kỳ; có ở ta mà không có trong đợt → `stale`, **không xoá** |
| Ánh xạ sai | Giữ raw → sửa luật → chạy lại (P2) |
| Lược đồ họ trôi | `schema_version`; thiếu trường bắt buộc thì **loại dòng**, thừa trường lạ thì **bỏ qua nhưng đếm** |
| Object mồ côi | Reaper quét object không có row, grace period ≥ 24h |
| Trôi số lượng | `rudi-reconcile` đối soát theo tỉnh/loại, ghi báo cáo lệch |

Nhịp đã thống nhất: **tăng dần mỗi ngày, toàn bộ mỗi thứ Hai.** `dot_seq` phát hiện đợt thiếu, `dot_truoc` nối thành chuỗi.

---

## SDLC

Cổng CI **không chạy từ 2026-08-29** — cổng thật là `make gate` (24 stage).

- Thêm stage `ingest` vào `scripts/gate.sh` và job trong `test.yml` theo mô-típ đang dùng: **thiếu thư mục thì skip + notice, có thư mục mà thiếu file mốc thì `::error` + exit 1**.
- Test Go chạm DB: `//go:build postgres` + `testdb.Pool(t)`, qua `scripts/go_postgres_tier.sh`. `Migrate` idempotent vì test gọi hai lần.
- Cổng conformance driver chạy MinIO qua testcontainers, **skip phải có tiếng** — `gate_merge.sh` từ chối coi skip là xanh.
- Fixture: **tự viết dữ liệu tổng hợp**, không commit lô thật. `.ndjson`/`.json`/ảnh đều là `controlled-artifact`, phải pin sha256 + lý do ≥12 ký tự.
- Migration Alembic: **một head duy nhất**.
- Comment/docstring tiếng Anh, commit message tiếng Việt.

---

## Hai luồng việc song song

### W1 — Hệ nạp dữ liệu

| Mốc | Việc | Tiêu chí ra **đo được** |
|---|---|---|
| **M0** | Vá `core` thiếu `MOBILE_MEDIA_ROOT` + volume trong compose; sửa comment sai ở `mediasnap.go:58` ("store is content-addressed" — không đúng, khoá là ngẫu nhiên) | `check_media_persists.sh` với `MOBILE_CORE_CANDIDATE_ROUTES=ported` |
| **M1 Sườn** | Migration Go (7 bảng) + Alembic (cột `places`, `place_photos`, `content_sha256`) + `admin_province` 34 tỉnh + 15 điểm đến | `go_postgres_tier.sh` xanh; `Migrate` gọi hai lần không đổi gì; một head |
| **M2 Hợp đồng** | JSON Schema `place.v1` + fixture tổng hợp + bộ kiểm dòng | Fixture hỏng cố ý → đúng mã `ingest_reject` |
| **M3 Worker** | `cmd/rudi-ingest` + consumer Kafka `ai.places.v1` đổ vào cùng bảng raw | Nạp lô thật; chạy **lần hai** không đổi dòng nào |
| **M4 Lô đầy đủ** | 6.113 dòng (6.109 sau khi bỏ `mon_an`) | `count(*) where source='vnlocal'` khớp manifest trừ reject; mỗi reject có lý do đọc được |
| **M5 Ảnh + MinIO** | Tách `Store` interface → `content_sha256` → driver S3 + conformance → MinIO vào compose → `core media migrate --verify` → **flip 6 route ảnh sang `owner: go`** → bật `s3` → `pending_object_deletes` + reaper → `core import-photos` | 19.704 ảnh vào store; giết giữa chừng rồi chạy lại không nhân bản; **mở ảnh ra nhìn** |
| **M6 Đường đọc** | Go phơi trường mới; client có trạng thái "chưa biết chỗ" | Màn Khám phá hiện địa điểm thật trên emulator; ảnh chụp mở ra nhìn |
| **M7 Toạ độ** | Nhận đợt `dot_seq 2` (tăng dần, chỉ dòng có toạ độ mới) | Bản đồ vẽ chấm chắc, **không vẽ chấm `suy_luan`**; ~26% chỗ hiện đúng trạng thái "không lên bản đồ" |
| **M8 Đối soát** | `rudi-reconcile` + đợt toàn bộ định kỳ | Cố tình xoá 10 dòng → báo cáo chỉ đúng 10 |

M0–M4 không phụ thuộc bên kia. M5 bước 1–4 không đụng một dòng nào trong `parity/` — đó là toàn bộ điểm của thiết kế.

### W2 — Hạ tầng deploy (gỡ chặn B3)

Theo `docs/architecture/01-duong-toi-production.md:178-187`, giữ nhỏ: managed container host + managed Postgres, **không Kubernetes, không Terraform** · secret ra khỏi compose · TLS + tên miền · `alembic upgrade head` là bước deploy riêng trước khi container mới nhận traffic. Ước lượng trong doc: 2–3 người-ngày.

---

## Rủi ro phải nói thẳng

**Canary `media-file-dropped` tụt xuống "not exercised" KHÔNG phải FAIL** (`parity/cmd/parity/main.go:665-667`). Nếu có ngày cả hai bên rời filesystem, bảng canary vẫn in "ok" trong khi ta đã mất bằng chứng cổng biết đỏ — **im lặng**. Phải đổi verdict đó thành FAIL hoặc viết `objsnap` thay thế.

**Cache của mediasnap có thể cho XANH SAI.** Nó tái dùng hash khi identity inode không đổi, tự biện minh bằng "cả hai writer đều rename từ tempfile". Driver nào ghi **đè tại chỗ** phá giả định đó → harness dùng lại hash cũ → xanh sai. Tệ hơn đỏ.

**`make clean` sẽ nói dối** nếu không thêm `mobile-minio-data` vào dòng cảnh báo ở `Makefile:184`.

**Geocode có thể không kịp**: 61,2 s/chỗ × 6.113 chỗ, 8 ca song song → ETA 13–16 giờ. M7 **không nằm trên đường găng** của M1–M6 — app phải dùng được khi chưa có toạ độ.

**Auth vẫn là header client tự khai** ở chế độ `dev` (chặn B1, 4–6 người-ngày, cần ADR). Ngoài phạm vi hệ nạp nhưng chặn "người thật dùng".

**Kafka là dependency thứ năm** và là thành phần hạ tầng mới trong đường production.

---

## Kiểm chứng

```bash
# M1
cd services/core && go test ./... && ../../scripts/go_postgres_tier.sh
python3 services/api/scripts/check_alembic_heads.py

# M3/M4 — nạp thật rồi đếm, không tin log
./rudi-ingest load --manifest /…/manifest-0001-20260922T154756Z.json --dry-run
./rudi-ingest load --manifest /…/manifest-0001-20260922T154756Z.json
docker exec mobile-local-postgres-1 psql -U mobile -d mobile -c \
  "select source, status, count(*) from places group by 1,2;
   select reason, count(*) from ingest_reject group by 1;"
# chạy lại lần hai — đếm phải KHÔNG đổi

# M5 — mở ảnh ra nhìn, không tin con số
docker exec mobile-local-postgres-1 psql -U mobile -d mobile -c \
  "select count(*), sum(byte_size) from place_photos;"

# cổng
make gate ONLY="guard api migration go-test go-postgres parity" STRICT=1
```

Ba phép chống "xanh giả", theo bài học đã ghi trong repo: **chạy lần hai phải không đổi gì** · **skip không phải là xanh** (`STRICT=1`) · **mở ảnh chụp ra nhìn** thay vì tin assertion đếm.
