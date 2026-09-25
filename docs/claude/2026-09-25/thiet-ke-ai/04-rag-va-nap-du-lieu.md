# Thiết kế 04 — RAG agentic và vòng đời nạp dữ liệu

- Ngày: 2026-09-25.
- Commit gốc: `f251db7`.
- protocol_version: không áp dụng (không phải lượt thí nghiệm).
- Trạng thái: **thiết kế đã được người dùng duyệt, chờ Lead ký ADR**. ADR đi kèm:
  `docs/decisions/proposals/ADR-0040-rag-agentic-va-vong-doi-nap-du-lieu.md`.
- Phạm vi: truy hồi cho hai bot (catalogue quán, sổ tay app, lịch sử chuyến của nhóm, móc xếp hạng cho
  trí nhớ Nếp) và vòng đời đưa dữ liệu vào chỉ mục. Router, vòng lặp agent, output guard, hàng đợi,
  stream và phía ghi của trí nhớ thuộc các bản khác trong cùng thư mục; ở đây chỉ nói chỗ RAG chạm vào.
- Thứ tự ưu tiên khi lệch: bảng hợp đồng chung (`docs/architecture/03-ai-engine-hop-dong.md`) đè bản gốc.
- Quy ước: «(sửa theo phản biện)» là chỗ khác bản gốc do hai phản biện đối kháng; `[P1-n]` là phát hiện
  n của phản biện «ràng buộc, riêng tư, bảo mật, đúng đắn», `[P2-n]` của phản biện «khả thi, thứ tự,
  đầy đủ». «(theo hợp đồng chung)» là chỗ đổi để khớp bảng hợp đồng.

## 0. Sự thật đã kiểm, là nền của thiết kế

| Sự thật | Chỗ |
|---|---|
| Ảnh DB là `postgres:16-alpine`; không migration nào tạo extension. `CREATE EXTENSION` duy nhất là `pgrowlocks` (contrib) trong test, nên contrib có sẵn trên ảnh này; pgvector thì không | `docker-compose.yml:78`; `internal/repo/wai_repo_oracle_postgres_test.go:348` |
| `places` chỉ có một index `(destination_id, category, id)`, không index chữ hay toạ độ | `services/api/app/db/models.py:2639` |
| Hàng OSM buộc có `source_ref` và `license` (ODbL) | `models.py:2624`; ADR-0017 §2.2 |
| Cột không biết thì để trống, «không bịa và không nhờ AI điền» | ADR-0017 §2.3 |
| Giờ mở cửa là chữ: seed «HH:MM – HH:MM», có ca qua nửa đêm; OSM giữ nguyên chuỗi `opening_hours` | `services/api/app/places/catalog.py:91` («18:00 – 01:00»), `app/places/osm.py:232`, `services/api/tests/places/test_osm_mapping.py:80` |
| Bốn category sản phẩm | `services/api/app/places/catalog.py:25-30` |
| Seed có 2 điểm đến (`d-da-lat` sort 10, `d-tphcm` sort 20); trình nhập có 15 | `services/api/app/places/seed_catalog.py:31,41,44,54`; `app/places/destinations_vn.py:20-202` |
| `ModelPlaceRows` luôn lấy điểm đến mặc định (sort nhỏ nhất = Đà Lạt), cắt 40 | `service/catalogue.go:28`, `catalogue.go:106-117`, `catalogue.go:309-310` |
| Bốn chỗ gọi `ModelPlaceRows`; ba chỗ là route có oracle Python | `chatassist/worker.go:214`; `routes/messages_wai.go:120`; `routes/suggestions_wai.go:53,187` |
| `/places/search` nạp **mọi** quán rồi gửi hết sang brain `place-search` | `routes/places_wai.go:387`, `:395`, `:405` |
| `promptsafety.Safe` chỉ kiểm name, address, open_hours, kinds, traits; `fold` chưa export | `domain/promptsafety/promptsafety.go:98`, `:29`; Python `app/places/prompt_safety.py:80` |
| Vùng (`areas`) là 8 quận/khu có toạ độ, không mang `destination_id`; điểm đến có bbox | `domain/areas/areas.go:20-32`; `models.py` lớp `Destination` (cột `bbox_*`) |
| `HaversineKm`, `NearestArea` đã có trong Go | `domain/areas/geo.go:85`, `geo.go:143` |
| Mẫu migration có version và checksum; trigger Go trên bảng Alembic đã có tiền lệ | `chatassist/migrate.go:32-67`; `chatlegacychange/schema.sql:52` |
| Khởi động chỉ kiểm bảng tồn tại; câu gợi ý tắt nằm ở `chatOffHint` | `cmd/core/main.go:168`, `main.go:386` |
| Pool Postgres 15 kết nối mỗi tiến trình | `internal/db/db.go:34` |
| Chỉ script nhập được gọi OSM/Wikimedia; không module nào dưới `app/` nêu host đó | `tests/test_only_the_importer_talks_out.py` |
| Corpus eval nhóm: 26 quán **bịa** ở Đà Nẵng, nhãn `nhan`/`khu` chỉ để chấm | `services/api/tests/skills/corpus/tra-loi-trong-nhom.json` (`ghi_chu_du_lieu`) |
| Oracle giờ mở cửa của bộ chấm | `services/api/tests/skills/tra_loi_trong_nhom.py:332` (`_opening`, `open_at`, `open_within`) |
| Ca trượt hôm nay: 01, 16 dị ứng; 03, 13 giờ/giá | `docs/claude/2026-09-24/chot-dot-ai-engine-va-dock-nep.md` §4 |

## 1. Cơ chế: Adaptive-CRAG trên retriever lai có kiểu, toàn bộ trong Go

Các ca trượt hôm nay hỏng ở **ràng buộc** (dị ứng, giờ, giá), không hỏng ở độ gợi nhớ. Vì vậy truy hồi phải
lọc cứng bằng SQL trước, rồi mới xếp hạng; model chỉ chấm lại trên tập đã hợp lệ.

- **Adaptive:** ý định từ Understand (bản 01) chọn nguồn. `find_places`/`plan` → catalogue (và lịch sử
  nhóm cho `plan`); `app_help`/`explain_screen` → sổ tay; `what_you_remember` → Go liệt kê, không truy hồi;
  `chat_answer`, `smalltalk`, `chia_bill_draft` → không truy hồi.
- **CRAG:** chấm lại → đủ thì trả; thiếu thì sửa đúng một vòng (nới ràng buộc mềm, mở rộng từ vựng) → vẫn
  thiếu thì hỏi lại **một** câu hoặc từ chối và nói rõ ràng buộc nào không thoả.
- **RAG không phải orchestrator (sửa theo phản biện) [P1-6][P2-4].** Bản gốc có router riêng
  (`KeHoachTraCuu`, enum `tim_cho|len_ke_hoach|…`), vòng ReAct riêng và trần «≤5 lời gọi» riêng. Bỏ cả ba.
  Router là Understand của harness; vòng lặp duy nhất là agent loop ADK; RAG là **ruột** của tool
  `search_places`, `get_place`, `list_group_outings`, `search_app_manual`, `explain_screen`. Mọi lời gọi model
  bên trong đi qua client `aiharness/llm` truyền qua ctx và bị đếm vào `MaxModelCallsPerTurn`.

| Phương án | Vì sao không làm cơ chế chính |
|---|---|
| Naive RAG (top-k theo độ giống) | Không ép được dị ứng, giờ, ngân sách; đúng thứ làm hỏng ca 01, 03, 13, 16 |
| Self-RAG | Cần model fine-tune token phản tư; giả lập tốn thêm lời gọi mỗi đoạn, hỏng mục tiêu token đầu |
| ReAct tự do trên tool | Số lời gọi không chặn được, flash-lite hay sai đối số, khó phát lại cho eval. Chỉ giữ agent loop có trần của harness |
| GraphRAG | Dữ liệu đã là quan hệ SQL: join chính là đồ thị. Sổ tay có một đồ thị màn nhỏ, tất định (§4b) |

## 2. Thành phần

### 2.1 Gói `services/core/internal/rag/` (writer duy nhất của mọi bảng `rag_*`)

| File | Vai |
|---|---|
| `migrate.go`, `schema_tu_vung.sql`, `schema_lam_giau.sql`, `schema_vector.sql` | theo mẫu `chatassist/migrate.go`, bảng `rag_schema_migrations` riêng |
| `retrieve.go`, `loc.go`, `rrf.go`, `chamlai.go`, `suachua.go` | `Retrieve(ctx, YeuCau) (KetQua, error)`: lọc cứng, xếp hạng, chấm lại, một vòng sửa |
| `diemden.go` | `ResolveDestination`: không bao giờ mặc định im lặng |
| `xephang/` | thuần, không SQL: `Fold` + song âm tiết + điểm từ vựng; dùng chung cho sổ tay và `nepnho` |
| `chunk_place.go`, `build.go`, `dedupe.go`, `embed.go`, `embed_stub.go` | nạp dữ liệu |
| `danhgia.go`, `promote.go` | cổng eval, promote, rollback, tombstone |
| `retrieve_test.go`, `testdata/truy-hoi-dia-diem.json` | tập vàng tất định (§8.3) |

### 2.2 Gói domain thuần, mỗi thứ **một** bản (sửa theo phản biện) [P2-4]
- `domain/thoigian`: giải `{ngay, gio_tu, gio_den}` của Understand theo `Turn.Luc` ở giờ Việt Nam
  (`pairpaper.Local`, `pairpaper.go:207`). Harness dùng đúng gói này, không có `preprocess/dates.go`.
- `domain/giomo`: `open_hours` → `int4multirange` phút-trong-tuần (0…10079). Nhận dạng seed, ca qua nửa đêm
  (tách thành hai đoạn khi vắt qua Chủ nhật → thứ Hai), tập con OSM (`Mo-Fr 08:00-22:00; Sa,Su …`, `24/7`).
  Không hiểu thì NULL (chưa rõ). Golden đối chiếu `_opening`/`open_at`/`open_within` của bộ chấm.
- `domain/tuvung`: từ vựng đóng cho `di_ung`, `an_kieng`, `loai_cho` (= bốn category), `khi_chat`; bảng đồng
  nghĩa; từ khoá dị ứng đã `Fold` («hai san», «tom», «cua», «dau phong»…). Understand lấy enum từ đây.
- `promptsafety.Fold` (export lại `fold`, không đổi hành vi) và `promptsafety.SafeDeep` (mới, phủ cả mô tả,
  review, activities). `Safe`/`Filter` và Python `prompt_safety.py` **không đổi**, vì là oracle (sửa theo phản
  biện) [P1-15]: bản gốc sửa cả hai phía trong một commit; bỏ, mọi đường engine dùng `SafeDeep`.

### 2.3 Chỗ nối với registry tool (theo hợp đồng chung)

| Tool (`aiharness/tools`) | Gọi vào | Bot |
|---|---|---|
| `search_places` | `rag.Retrieve`, hạn 3 s kể cả chấm lại | nhóm; Nếp khi ADR-0041 bật (lát 13) |
| `get_place` | đọc `places` sống + `SafeDeep` | như trên |
| `list_group_outings` | SQL có kiểu theo `context_id` của job (§4c) | chỉ nhóm |
| `search_app_manual`, `explain_screen` | `huongdan.Tim`, `huongdan.TheoMan` (§4b) | `search_app_manual` cả hai; `explain_screen` chỉ Nếp |
| `recall_memory` | `nepnho` (dùng `rag/xephang`), **chỉ từ gốc scope=me** [P1-1] | chỉ Nếp |

### 2.4 CLI, compose, khởi động
- `core migrate-rag`; `core rag build|eval|promote|rollback|tombstone|enrich|status`. Nạp dữ liệu là CLI,
  không phải một lane hàng đợi (sửa theo phản biện) [P2-14]. Compose thêm service `migrate-rag` sau `migrate-chat`.
- Khi `MOBILE_AI_ENGINE_GROUP=go` mà schema từ vựng chưa cài, `work` từ chối khởi động kèm câu gợi ý kiểu
  `chatOffHint` (theo hợp đồng chung: đòi version ≥ N). Thiếu schema vector thì chạy chỉ-từ-vựng, ghi log
  một lần, `core rag status` báo `degraded`. `serve` không đòi schema RAG: route công khai duy nhất dùng nó
  (`/places/search`, §7) có đường lùi trên hàng sống.

### 2.5 Ảnh Postgres (lát 16)
- `infra/postgres/Dockerfile`: `FROM postgres:16-alpine@sha256:…`, pgvector v0.8.x dựng từ tarball ghim sha256.
  Giữ **musl**: ảnh Debian `pgvector/pgvector:pg16` đổi collation libc dưới các btree khoá chữ của volume cũ,
  buộc REINDEX. Thêm file vào `scripts/check_dockerfile_pinning.sh`.
- Không dùng `unaccent`: Go `Fold` cả phía chỉ mục lẫn phía truy vấn bằng một hàm, tránh hai bộ chuẩn hoá lệch.
- **Đếm lại chỗ ghi cứng ảnh (khác bản gốc «~12»).** Biến `MOBILE_TEST_POSTGRES_IMAGE` đã có ở
  `parity_stacks.sh:43`, `postgres_tier.sh:90`, `e2e_slice.sh:78`, `go_postgres_tier.sh:27`, `gate.sh:1005-1041`.
  Còn ghi cứng 7 chỗ: `docker-compose.yml:78`, `.github/workflows/postgres-repository.yml:28`,
  `.github/workflows/test.yml:907`, `scripts/chat_v2_postgres.sh:25`, `chat_mass_realtime.sh:31`,
  `chat_e2e_stack.sh:57`, `internal/idem/oracle_postgres_test.go:137`. Lát 16 đưa cả 7 về biến.

## 3. Schema (Go sở hữu)

Số version **không đặt trước**: cấp theo thứ tự lên main trong bảng `rag_schema_migrations` (theo hợp đồng
chung) [P1-7][P2-1]. «Từ vựng», «làm giàu» và «vector» dưới đây là tên nội dung, không phải số.

```sql
-- schema_tu_vung.sql (chạy trên ảnh hiện tại; pg_trgm là contrib như pgrowlocks — lát 8 vẫn chạy thật để chắc)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
rag_index_versions(id bigint identity PK, corpus text CHECK (corpus IN ('place')),
  state text CHECK (state IN ('building','built','evaluated','active','retired','failed')),
  chunker text, embed_model text DEFAULT 'none', embed_dims int DEFAULT 0, source_digest bytea,
  parent_id bigint REFERENCES rag_index_versions, eval jsonb, created_at, promoted_at, retired_at);
  UNIQUE (corpus) WHERE state='active';
rag_docs(version_id → rag_index_versions ON DELETE CASCADE, doc_id text, destination_id text,
  category text, price_min_vnd bigint, price_max_vnd bigint,
  open_week int4multirange /* NULL = chưa rõ */, di_ung_nguon text[], an_kieng_nguon text[],
  khi_chat text[] /* ba cột này quét tất định từ chữ của chính hàng */,
  name_fold text, lat, lng, license text, canonical_id text,
  PRIMARY KEY (version_id, doc_id));  INDEX (version_id, destination_id, category);
  GIN (name_fold gin_trgm_ops);
rag_chunks(version_id, chunk_id, doc_id, facet text CHECK (facet IN ('ho_so','danh_gia')),
  body text CHECK (char_length(body) BETWEEN 1 AND 2000) /* đã qua SafeDeep */,
  search_text text /* Fold + song âm tiết */,
  tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', search_text)) STORED,
  content_hash bytea, PRIMARY KEY (version_id, chunk_id), FK (version_id, doc_id) CASCADE); GIN (tsv);
rag_tombstones(corpus, doc_id, reason CHECK (reason IN ('unsafe','takedown','closed','source_deleted')),
  created_at, PRIMARY KEY (corpus, doc_id));
rag_query_log(at, corpus, y_dinh, version_id, n_loc, n_ung_vien, vong smallint, cham_lai bool,
  ket_qua CHECK (ket_qua IN ('tra_loi','hoi_lai','khong_thoa','degraded')), ms int,
  co text[] CHECK (co <@ ARRAY['khong_dau','gio_chua_ro','gia_chua_ro','nhan_nhieu_diem_den']));
-- schema_lam_giau.sql (lát 16; KHÔNG cần pgvector, nên thiếu vector không chặn làm giàu và độ tươi)
ALTER TABLE rag_docs ADD di_ung_suy text[] /* chỉ để loại */, ADD an_kieng_duyet text[] /* chỉ bản đã duyệt */;
rag_dirty(corpus, doc_id, noticed_at, PRIMARY KEY (corpus, doc_id));  -- trigger AFTER I/U/D trên places
place_enrichments(place_id → places ON DELETE CASCADE, extractor, source_hash, model, output jsonb,
  review CHECK (review IN ('auto','reviewed','rejected')), created_at, PRIMARY KEY (place_id, extractor));
-- schema_vector.sql (lát 16; cần pgvector, thiếu thì fail closed, phục vụ vẫn chỉ-từ-vựng)
CREATE EXTENSION IF NOT EXISTS vector;
ALTER TABLE rag_chunks ADD embedding vector(768);
rag_embedding_cache(content_hash, model, dims, task, embedding vector(768), PRIMARY KEY (…4 cột));
```

- **Sổ tay không nằm trong DB (sửa theo phản biện) [P2-6].** Bản gốc có corpus `manual`, `rag_manual_edges` và
  version theo `app_build`. Bỏ: core build với `context: ./services/core` (`docker-compose.yml:57`), `go:embed`
  không với ra ngoài module. Sổ tay là `services/core/internal/huongdan/data/*.md` nhúng vào binary (§4b).
- **Không bảng `rag_*` nào có `person_id`** (sửa theo phản biện) [P1-14][P2-8]. Không có gì cho trigger xoá
  trên `people.deleted_at`; test Postgres liệt kê cột `person_id` của lát 15 phải thấy 0 cột trong `rag_*`.
- `rag_query_log` không chữ, không id người; xoá sau 30 ngày bằng tác vụ định kỳ `jobs.DinhKy` (bản 02) [P2-14].
- Không thêm cột `evidence_ids` lên job (sửa theo phản biện) [P1-15]; bằng chứng sống trong ledger của lượt.
- **Version:** mỗi version chép đủ hàng; promote/rollback chỉ lật `state` trong một transaction dưới advisory
  lock. Dọn version `retired` quá 14 ngày, giữ tối đa 3. **Tombstone không thuộc version nào**: anti-join lúc
  truy vấn, nên rollback không làm sống lại tài liệu đã gỡ.
- **Chưa có ANN index.** Tập sau lọc nhỏ (một điểm đến ≤ ~2k quán) nên quét `<=>` chính xác sau lọc SQL. Thêm
  HNSW (`hnsw.iterative_scan`) chỉ khi p95 `n_loc` vượt 20k.

## 4. Bốn nguồn

**(a) Quán** (chỉ mục chụp nhanh, có version). Mỗi quán hai facet: `ho_so` (tên, category, kinds, vùng, giá
bằng chữ, giờ, traits, activities, khí chất, món chính, mô tả ≤1500) và `danh_gia` (≤5 review an toàn, **bỏ
tên tác giả**). Lọc cứng trong SQL: điểm đến; `NOT (di_ung_nguon || di_ung_suy) && $di_ung`; `open_week @> $phut`
cho một thời điểm, `open_week && $khung` cho một khung (đúng ngữ nghĩa `open_within` của oracle);
`price_min_vnd <= $ngan_sach` (số nguyên đồng); `(an_kieng_nguon || an_kieng_duyet) @> $an_kieng`; không
tombstone; `canonical_id IS NULL`. Chưa rõ thì giữ nhưng gắn
cờ và xếp cuối (`gio_chua_ro`, `gia_chua_ro`); khi người hỏi nêu giờ, quán `gio_chua_ro` chỉ được vào khi còn
dưới 3 quán biết chắc mở, và câu trả lời phải nói «chưa có giờ mở cửa».
- **Kiểm ràng buộc hai lần** (bổ sung của bản này): SQL trên chỉ mục để thu hẹp; sau đó hydrate từ `places`
  sống và Go kiểm lại cùng bộ lọc trên giá trị sống. Id không còn trong `places` bị bỏ (thế giới đóng). Chỉ mục
  cũ vì thế chỉ làm giảm độ gợi nhớ, không bao giờ làm sai giá, giờ hay dị ứng.

**(b) Sổ tay app** (sửa theo phản biện) [P2-6]. Gói `internal/huongdan`, dữ liệu `data/*.md` qua `go:embed`, tìm
từ vựng trong bộ nhớ trên khoảng 300 đoạn bằng `rag/xephang` (không DB, hạn 0.3 s). Mỗi màn một file: front
matter `man` (đúng route id của `PhieuNguCanh.man`), `tieu_de`, `nut[]`, `di_toi[{nut, man}]`, `tien: bool`;
mỗi mục H2 là một việc («Thêm một chặng») và là một đoạn. `huongdan.BanDung()` = 12 hex đầu sha256 của dữ liệu
nhúng; phiếu v2 mang hash bản build của client, lệch thì gắn cờ `ban_app_khac` và câu trả lời nói có thể khác
bản app. Màn tiền chỉ có đoạn điều hướng. «Làm sao tới X»: BFS trên đồ thị `di_toi` từ màn hiện tại, trả các bước
tất định. Đoạn của màn hiện tại luôn được ghim vào bằng chứng.

**(c) Lịch sử chuyến của nhóm** (sống, không chỉ mục). `list_group_outings` chạy SQL có kiểu theo `context_id`
của job và tư cách thành viên đang hoạt động; **model không truyền đối số danh tính nào**. Trả kèo, chặng, số
check-in; chữ tự do (tiêu đề, nhãn chặng) qua `TextSafe` lúc đọc; tên hiển thị không bao giờ lưu. Câu kiểu «chỗ
view đẹp lần trước» xếp hạng trong đúng tập `place_id` của các chặng nhóm. Nếp **không** dùng tool này (ADR-0036
§4); `my_upcoming_outings` là tool riêng của ADR-0041.

**(d) Trí nhớ Nếp: chỉ phần xếp hạng** (sửa theo phản biện) [P1-1][P1-2][P2-7]. Bản gốc cho `rag.NhoVeToi` đọc
fact có vector, xoá mềm bằng `forgotten_at`, và luôn kèm fact `an_uong` cho `tim_cho` — mà enum router dùng
chung cho cả nhóm, nên fact riêng chảy vào câu trả lời cả phòng thấy. Nay:
- Schema của `nepnho` là chuẩn (`nep_su_that`, trường `cau`); không vector, không phụ thuộc pgvector.
- `rag` **không** import `nepnho` và không nêu tên bảng `nep_*`; `nepnho` gọi `rag/xephang` (thuần) trên các
  fact còn hiệu lực. «Quên» là xoá cứng, việc của `nepnho`.
- `recall_memory` chỉ tới được từ gốc scope=me; `search_places` của nhóm không nhận fact nào. Canary ở §8.4.

## 5. Pipeline lúc hỏi

**5.1 Đầu vào, không có router thứ hai.** `YeuCau` dựng từ slot đã kiểm của Understand (`khu_vuc`, `ngay`,
`gio_tu`, `gio_den`, `ngan_sach_moi_nguoi_vnd`, `so_nguoi`, `di_ung`, `an_kieng`, `loai_cho`, `tham_chieu`) cộng
phần tất định của preprocess: chữ của chính người hỏi (bản định tuyến, đã `Fold`), khí chất và dị ứng quét bằng
`tuvung` trên lời người gọi và gói chat được trao (lưới dự phòng cho «mình dị ứng hải sản» ở tin cũ). Slot đã
qua kiểm của Go và guard, JSON Understand nằm trong `<du_lieu nguon="hieu">` (bản 01); RAG không nhận chữ tự do
nào do model viết.

**5.2 Điểm đến** (`ResolveDestination`), theo thứ tự: (1) `khu_vuc` → điểm đến có bbox chứa tâm vùng; (2) tên điểm
đến khớp `Fold` trong lời hỏi (danh sách đóng từ `ListDestinations`); (3) Nếp: điểm đến của các id trong
`thay.diaDiem` của phiếu v2; (4) nhóm: điểm đến đa số trong chặng các kèo sắp tới, rồi 90 ngày gần nhất. Vẫn
không rõ mà cần quán → `can_hoi_lai{thieu: khu_vuc}`. **Không bao giờ mặc định im lặng.** Sửa lỗi «luôn Đà Lạt»
chỉ trên đường engine Go (sửa theo phản biện) [P1-15], xem §7.

**5.3 Truy hồi.** Chạy suy đoán song song với Understand từ kết quả preprocess; slot về lệch ở bộ lọc cứng thì
chạy lại [P2-9]. Sau lọc cứng, các danh sách top 50: `ts_rank_cd` trên `tsv`; trigram trên `name_fold`; (lát 16)
vector câu hỏi, `RETRIEVAL_QUERY`, cache theo hash chữ. Nhóm cộng một danh sách tiên nghiệm gu theo
`service.ScoreOrZero` (`catalogue.go:198`) trên hồ sơ **tổng hợp** do `group_snapshot` đưa vào; `rag` không tự
gọi `GroupTaste` (ngoại lệ có tên của cổng lát 5 chỉ nằm ở một chỗ). Nếp không có tiên nghiệm gu nhóm. Khi gốc
là scope=me và `nho` bật, harness truyền trong `YeuCau` một danh sách tăng hạng dựng từ các fact mà `recall_memory`
đã trả (thiết kế 05 §6); `rag` không tự đọc `nep_*`. Gộp bằng RRF
(k=60), điểm tính bằng số nguyên `⌊10⁹/(60+hạng)⌋`, phá hoà bằng id, để golden ổn định từng byte. Lấy top 20.

**5.4 Chấm lại.** Chỉ khi còn hơn k=8 ứng viên [P2-4]. Một lời gọi flash-lite, bằng chứng gọn ≤300 ký tự mỗi
quán, bọc trong `<du_lieu nguon="catalogue">`, id đổi thành bí danh `c1…c20` và schema đầu ra là enum trên đúng
tập bí danh: `[{id, muc: 0|1|2}]`. Model không bịa được id, và đầu ra chỉ đổi thứ tự, không vào prompt nào. Hạn 1.2 s; lỗi hoặc hết hạn thì giữ thứ tự RRF, kết quả `degraded`.

**5.5 Đủ chưa, và đúng một vòng sửa.** Đủ = ≥3 quán `muc=2` (không chấm lại thì ≥3 quán qua lọc cứng và có trúng
từ vựng). Thiếu thì nới ràng buộc **mềm** theo thứ tự khí chất → loại chỗ → khu vực (bỏ độ gần vùng, vẫn trong
điểm đến). Dị ứng, giờ đã nêu, trần ngân sách, ăn kiêng và điểm đến **không bao giờ nới**. «Viết lại» là mở
rộng từ vựng tất định bằng `tuvung` (đồng nghĩa, từ gốc, bỏ các từ đã thành slot), **không** phải chữ model viết
(sửa theo phản biện) [P1-8]: không kênh chữ tự do nào đi từ model vào truy vấn hay prompt. Chấm lại vòng hai chỉ
khi còn ≥1.2 s trong hạn 3 s và còn chỗ trong trần lời gọi. Tối đa 2 vòng.

**5.6 Kết quả trả agent.** `{ket_qua: co|can_hoi_lai|khong_thoa, quan: [{id, ten, khu, gia, gio, co[],
ghi_chu ≤300}], gia_dinh[], thieu, rang_buoc_khong_thoa[]}`. Id vào ledger; văn xuôi nêu quán bằng `[[p:ID]]`.
Câu hỏi lại và câu từ chối là câu cố định của harness theo `thieu`/`rang_buoc_khong_thoa`, 0 lời gọi model.
Phần `places`/`itinerary` của thẻ `tra_loi` (≤3 phần) do tool nháp `propose_places`/`propose_itinerary` dựng từ
đúng ledger này và qua `GroundReply` (theo hợp đồng chung).

**5.7 Sau khi sinh (sửa theo phản biện) [P1-4].** Bản gốc cho stream «tạm», rồi thay bằng bản đã kiểm và sinh lại
một lần. Bỏ: phòng lane cũ thấy mọi byte, và không có «rút lại» (theo hợp đồng chung). Luật `CheckAnswer` tách hai:
- Luật cục bộ chạy trong output guard cửa sổ 48 rune của harness (nơi duy nhất sinh `delta`): giá/giờ đứng gần
  `[[p:` phải khớp ledger; tên quán catalogue (đã `Fold`, ≥2 âm tiết) không qua token thì đếm
  `ungrounded_mention`; «nhãn nút» trích dẫn phải có trong `nut` của đoạn sổ tay đã trả.
- Luật cấp thẻ chạy trong `GroundReply` trên đúng tập ledger: id trích ⊆ bằng chứng.
- Không sinh lại sau delta đầu; trước delta đầu được sinh lại một lần (sự kiện `lam_lai`) nếu trần còn chỗ.
- Không đổi grounding của `POST /messages` (`messages_wai.go:120-124`): route đó có oracle Python [P1-15].

**5.8 Giới hạn**

| Hạng mục | Trần |
|---|---|
| Lời gọi model trong `search_places` | ≤2 (chấm lại vòng 1 và 2), đếm vào `MaxModelCallsPerTurn` của `aiharness/llm` (theo hợp đồng chung) [P1-6][P2-10] |
| Lời gọi embedding mỗi lượt (lát 16) | ≤2, bộ đếm riêng `MaxEmbedCallsPerTurn` (đề xuất §10 câu 1, ADR-0040 §3), không ăn vào trần sinh chữ `MaxModelCallsPerTurn`; cùng limiter theo lời gọi |
| `search_places` trọn gói | 3 s kể cả chấm lại [P2-4]; SQL `SET LOCAL statement_timeout='800ms'`, tx ReadOnly |
| Bằng chứng | ≤8 quán × 600 ký tự, ≤5 đoạn sổ tay, ≤5 kèo; tổng ≤12k ký tự |
| Kết nối DB | theo semaphore DB toàn tiến trình của worker (bản 01, lát 4) [P1-17][P2-11] |
| Latency | mục tiêu hợp đồng: token chữ đầu p50 ≤2.5 s, p95 ≤5 s. Cần gạt đầu tiên nếu eval T4 trượt: bỏ chấm lại cho truy vấn tên riêng khi top 3 của hai danh sách từ vựng trùng nhau |

**5.9 Hỏng thì sao**

| Hỏng | Hành vi |
|---|---|
| API embedding lỗi / chưa có schema vector | chỉ từ vựng, `degraded` |
| Chưa có version `active` | `rag.Retrieve` báo `degraded`, dùng hàng `places` sống của điểm đến đã giải, lọc cứng bằng Go |
| Chấm lại hết hạn | thứ tự RRF |
| Điểm đến không rõ | hỏi lại một câu, không đoán |
| Không quán nào qua lọc cứng | `khong_thoa` kèm tên ràng buộc; không bao giờ nới ràng buộc cứng |

**5.10 Quan sát.** Chỉ id, enum, số đếm, thời gian, điểm (`rag_query_log`, `slog`). Không chữ truy vấn, không
chữ bằng chứng (ADR-0037 §2.8, bản đề xuất).

## 6. Vòng đời nạp dữ liệu

**6.1 Quán** (mỗi bước có đầu ra kiểm được, chạy lại là no-op theo hash):
1. **Hợp đồng nguồn** `place.v1` (struct Go, đọc từ `places`): name ≤120, address ≤200, mô tả ≤1500, review
   ≤20×600, activities ≤20×60; hàng `osm` mang `license` và siêu dữ liệu đó đi theo vào bằng chứng.
2. **An toàn:** `SafeDeep` trên **mọi** trường chữ. Tên/địa chỉ không an toàn → bỏ cả hàng, tombstone `unsafe`.
   Mô tả/review/activities không an toàn → cách ly từng trường, đếm trong báo cáo nạp.
3. **Chuẩn hoá:** `Fold`, song âm tiết (`ca_phe`), đồng nghĩa `tuvung`; `giomo` ra `open_week`; dị ứng/ăn kiêng
   quét từ chữ **của chính hàng** bằng từ khoá đóng (tất định, không AI) vào `di_ung_nguon`.
4. **Làm giàu** (lát 16): `core rag enrich` gửi ≤20 quán đã an toàn mỗi lời gọi, 4 song song, hạn 30 s, trần
   2000 lời gọi mỗi lượt chạy, tiếp tục theo `source_hash`, tới action brain `place-enrich` (Python, flash-lite,
   `response_schema` enum; brain không có credential DB, Go ghi). Đầu ra `{id, khi_chat[], di_ung_co_the[],
   an_kieng[], mon_chinh[≤5], tom_tat ≤300, tin_cay}`. Go bỏ mọi giá trị ngoài enum, chạy `SafeDeep` trên
   `tom_tat`. Seed/curated được người duyệt 100%; tin cậy thấp hoặc có nhãn dị ứng vào hàng đợi duyệt (`--review`).
5. **Dedupe** (lát 16): trong một điểm đến, trigram ≥0.8 **và** `HaversineKm` ≤80 m; giữ curated > seed > osm, rồi
   hàng giàu hơn; hàng kia nhận `canonical_id`. Không sửa `places` (chỉ Python seed/import ghi bảng đó).
6. **Embed** (lát 16): `gemini-embedding-001`, 768 chiều, chuẩn hoá L2 trong Go (cắt MRL làm mất chuẩn),
   `RETRIEVAL_DOCUMENT` với `Title=name`; lô 100, hạn 20 s, 3 lần thử lại trên 429/5xx; lấy từ cache theo hash
   trước. Đổi model là version mới và qua cổng lại.
7. **Build:** chèn, `ANALYZE`, `built`. Hàng `building` không bao giờ thấy được từ truy vấn.
8. **Cổng eval** (§8.3) → `evaluated`, trượt thì `failed`.
9. **Promote/rollback:** đổi model hoặc chunker luôn promote tay; build tăng dần đổi ≤5% tài liệu, cùng model và
   chunker, qua cổng thì tự promote. Rollback về `parent_id`.
10. **Độ tươi:** trigger trên `places` ghi `rag_dirty`; indexer là một `jobs.DinhKy` (advisory lock, nhịp 60 s,
    gom 5 phút). DELETE tombstone ngay (`source_deleted`). Trước lát 16 chỉ build bằng CLI; hydrate sống (§4a) giữ
    cho sự thật luôn đúng.
11. **Giám sát** (`core rag status`, `slog`, `rag_query_log`): tỉ lệ `hoi_lai`/`khong_thoa`/`degraded`; 0 kết quả
    theo từng bộ lọc; tỉ lệ vi phạm trích dẫn (phải ≈0); latency từng chặng; tuổi hàng dirty cũ nhất; version
    active và eval của nó; số trường bị cách ly.

**6.2 Sổ tay app.** `apps/mobile/tools/rut-huong-dan.mjs` (mới; công cụ mobile nằm ở `tools/`, không có
`scripts/`) rút route id, nhãn JSX, `accessibilityLabel` và đích `router.push` từ `apps/mobile/app/**` vào
`huongdan/data/_rut/*.json` (commit, sinh lại phải giống từng byte) → người review viết văn → cổng lệch (§8.1)
→ build core nhúng bản mới → `BanDung()` đổi → eval sổ tay. Kiểm an toàn: không số tiền, không tên người,
không dữ liệu thật.

**6.3 Luật của làm giàu** (ADR-0017 §2.3, ODbL). Làm giàu **không bao giờ** ghi vào cột của `places` và không bao
giờ hiện trên wire như một sự thật của quán. Nó chỉ là tín hiệu truy hồi: dị ứng suy ra (kể cả `auto`) chỉ được
dùng để **loại bớt** (thận trọng), không bao giờ để khẳng định quán «không có» chất đó; ăn kiêng suy ra chỉ vào
bộ lọc cứng khi đã `reviewed`, bản `auto` chỉ cộng điểm xếp hạng; câu trả lời nêu «gợi ý, nên hỏi quán».
Làm giàu từ hàng OSM là cơ sở dữ liệu phái sinh: giữ `license`, không vào Git.

## 7. Sửa bắt buộc
- **«Luôn Đà Lạt»** (sửa theo phản biện) [P1-15]. `ModelPlaceRows` và ba route có oracle (`suggestions_wai.go:53,187`,
  `messages_wai.go:120`) giữ nguyên, Python `model_place_rows` không sửa. Đường engine Go dùng `rag.Retrieve` +
  `ResolveDestination`; `worker.go:214` hết dùng `ModelPlaceRows` khi `MOBILE_AI_ENGINE_GROUP=go`.
- **`/places/search` gửi cả danh mục** (`places_wai.go:387`). Thay bằng shortlist lai ≤30 hàng (lọc theo điểm đến
  khi suy ra được), qua `Filter` rồi `SafeDeep`: dùng `rag.Retrieve` khi có version `active`, không thì chấm từ
  vựng bằng `rag/xephang` trên hàng `places` sống; không bao giờ quay về gửi cả danh mục. Đây là **lệch Go-only trên payload brain** của một route
  `LIVE-GO`: parity chạy không khoá nên cả hai phía rơi vào nhánh `unavailable` và vẫn giống từng byte, tức parity
  **không thấy** thay đổi này. Bằng chứng là test Go trên payload (canary 3). ADR-0040 nêu tên ngoại lệ này.

## 8. Cổng, test, canary, đột biến

**8.1 Cổng cấu trúc**
- Cổng đọc xuyên gói của lát 5 (một cổng, không cổng riêng) (sửa theo phản biện) [P1-13][P2-5], allowlist theo
  gốc: `rag.Retrieve` → `rag_*`, `places`, `destinations`; `list_group_outings` → `outings`, `outing_stops`,
  `outing_stop_checkins`, `memberships`. **Không bao giờ**: `messages`, `chat_v2_events`, bảng tiền,
  `person_interests`, `saved_places`, `posts`, `pair_shared_constraints`, `nep_*`.
- `rag` chỉ import `aiharness/llm` (gói lá, để mọi lời gọi model bị đếm); không import `aiharness` gốc, `nepnho`,
  `chatassist` (chống vòng import và giữ trí nhớ ngoài tầm với của nhóm).
- `apps/mobile/tests/huong-dan-khop-ma.test.mjs`: mọi route có sổ tay hoặc nằm trong `KHONG_HUONG_DAN` kèm lý do;
  mọi «nhãn» trong văn là literal có thật trong `apps/mobile/{src,app}`; mọi `di_toi` tồn tại; `_rut` tươi.

**8.2 Tầng test**
- `go test ./internal/rag/... ./internal/huongdan/... ./internal/domain/{giomo,thoigian,tuvung}`: `Fold` bằng
  `fold` cũ trên mọi golden promptsafety; `giomo` với oracle `_opening` và golden OSM; số học RRF; golden
  `content_hash` của chunk; `ResolveDestination`; BFS màn; luật cục bộ đưa vào output guard.
- `scripts/go_postgres_tier.sh` (skip là đỏ): migration từ vựng trên ảnh hiện tại; migration vector fail closed
  khi thiếu pgvector; build → eval → promote → rollback; tombstone sống qua rollback; SQL multirange, dị ứng,
  ngân sách; hàng `building` không lộ; trigger dirty; `statement_timeout`; tool đọc trong tx ReadOnly.

**8.3 Tập vàng và ngưỡng** (một bộ, không chép) [P1-16][P2-17]. Tên và thang 0–3 theo bản eval:
`rag/testdata/truy-hoi-dia-diem.json`, `huongdan/testdata/truy-hoi-so-tay.json` (trí nhớ thuộc `nepnho`). RAG
thêm trường `phai_loai` cho violation. Toàn bộ tổng hợp: 12 quán seed (bịa), 26 quán bịa của corpus nhóm, và
một fixture sinh ra 300 quán trên 3 điểm đến. Khoảng 120 truy vấn quán theo nhóm `ten_rieng`, `khong_dau`,
`khi_chat`, `rang_buoc`, `di_ung`, `lien_diem_den`, `bay_injection`; khoảng 80 truy vấn sổ tay.
- CI (stub embedder + từ vựng): số tất định ghim cứng, lệch một chữ số là đỏ.
- Cổng promote, eval thật (embedding thật do Lead duyệt số lời gọi; kết quả ngoài repo ở `~/.cache/rudi-bang-chung/`):
  quán recall@10 ≥0.90, nDCG@10 ≥0.75, MRR@10 ≥0.70, **violation@10 = 0** (tuyệt đối); sổ tay recall@5 ≥0.90;
  nhóm `khong_dau` cách nhóm có dấu ≤0.05; mọi số ≥ bản active − 0.01. Hiệu chuẩn lại sau đường nền đầu tiên.
- Đầu-cuối: `tra_loi_trong_nhom.py` chạy qua engine Go (lát 9): ≥14/16 ổn định qua 5 lần (theo hợp đồng
  chung); riêng ca 01, 03, 13, 16 đạt cả 5 lần (bản gốc: 3 lần chạy).

**8.4 Canary** (đỏ đúng chỗ dự đoán, ca identity xanh)
1. Quán có injection trong **review** bị cách ly trường; cùng quán review sạch thì được chỉ mục.
2. Nhóm HCM có kèo sắp tới ở HCM: mọi bằng chứng là `d-tphcm`. Hôm nay đỏ (qua `ModelPlaceRows`).
3. `/places/search` trên fixture 5k quán: payload brain ≤30 hàng. Hôm nay đỏ.
4. Tài liệu đã tombstone không trở lại sau rollback.
5. Fact trí nhớ đã seed không bao giờ có trong `YeuCau` hay bằng chứng của `search_places` ở lượt nhóm
   (dùng bộ ghi request của harness) [P1-1].

**8.5 Đột biến tự nghĩ** (kiểm tương đương trước; mỗi cái đỏ đúng bước dự đoán)
- M1 bỏ lọc dị ứng → violation@10 >0 ở nhóm `di_ung`.
- M2 bỏ `Fold` phía truy vấn → recall nhóm `khong_dau` tụt ≥0.3 (thay đột biến «bỏ unaccent» của bản eval, thứ
  RAG không dùng) [P1-16].
- M3 RRF chỉ dùng danh sách `tsv` (bỏ trigram) → MRR nhóm `ten_rieng` có lỗi gõ tụt.
- M4 bỏ anti-join tombstone → canary 4 đỏ.
- M5 `SafeDeep` bỏ qua review → canary 1 đỏ.
- M6 `ResolveDestination` rơi về sort nhỏ nhất → canary 2 đỏ.
- M7 chấm lại gọi model ngoài bộ đếm `llm` → kịch bản stub đỏ ở `so_goi_model` (dự đoán 3, ra 2).

## 9. Lát (số theo kế hoạch đã duyệt)

| Lát | Phần RAG | Cổng riêng |
|---|---|---|
| 0 | Tài liệu này + đề xuất ADR-0040 | — |
| 8 | `rag` từ vựng: schema, build/eval/promote/rollback/tombstone, chunker quán; `thoigian`, `giomo`, `tuvung`; `Fold`, `SafeDeep`; `ResolveDestination`; shortlist `/places/search` | tập vàng tất định trong CI; canary 1, 3, 4 |
| 9 | `search_places` = `rag.Retrieve` (3 s), chấm lại, một vòng sửa, hỏi lại/từ chối, luật cục bộ vào output guard, `list_group_outings`; engine Go thôi dùng `ModelPlaceRows` | canary 2, 5; T3 ≥14/16 |
| 10 | (hạ tầng) xoá 30 ngày `rag_query_log` đăng ký vào `jobs.DinhKy` | — |
| 13 | `huongdan` (`go:embed`, `_rut`, BFS, `BanDung`), `search_app_manual`, `explain_screen`; cổng lệch sổ tay | `huong-dan-khop-ma` |
| 15 | `rag/xephang` cho `nepnho` | canary 5 lặp lại với fact thật |
| 16 | ảnh pgvector + 7 chỗ ghi cứng; schema vector; embed + cache; `place-enrich` + hàng duyệt; dedupe; `rag_dirty` + indexer; cổng promote thật | ngưỡng §8.3 |
| 18 | bộ truy hồi trong binary eval duy nhất | — |

## 10. Chưa chốt

1. **Lời gọi embedding và trần.** Hợp đồng chung chỉ có `MaxModelCallsPerTurn` cho lời gọi sinh chữ. Đề xuất
   thêm `MaxEmbedCallsPerTurn = 2` trong cùng `aiharness/llm`, eval đọc cả hai; sửa `03-ai-engine-hop-dong.md`
   cùng commit lát 16.
2. **Có cần slot `diem_den` trong Understand?** Bản này giải điểm đến tất định (vùng, tên, phiếu, lịch sử) để không
   đổi schema của bản 01. Nếu eval định tuyến cho thấy nhầm nhiều, thêm slot enum lấy từ `ListDestinations`.
3. **Bình chọn trong lịch sử chuyến.** ADR-0036 §2.3 không nêu; registry không có `list_open_votes`. Đề xuất: chỉ
   bình chọn đã đóng gắn với kèo, chỉ nhóm, qua sửa hợp đồng.
4. **`pair_shared_constraints.khong_an_duoc`** có được vào bộ lọc dị ứng tất định mà không tới model không: cần hợp
   đồng dữ liệu ADR-0034 §3.
5. **Chỉ mục production dựng ở đâu:** bước deploy chạy CLI, hay chỉ indexer định kỳ. Chưa có cấu hình host prod.
6. **Quyền tạo extension ở prod:** role ứng dụng có được `CREATE EXTENSION` không; không thì migration fail closed.
7. **Soạn nháp sổ tay bằng model:** mặc định người viết văn; nếu dùng model thì là lời gọi thật, Lead duyệt số
   lượng, chạy ngoài CI, không commit bản nháp chưa review.
8. **Lệch Go-only của `/places/search`** (§7): Lead xác nhận ngoại lệ, hoặc đợi Python bị gỡ khỏi route này.
