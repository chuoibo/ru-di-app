# Nạp chỉ mục vector vào Milvus (lát 16, nhánh `infra/ingest-sdlc`)

- Ngày: 2026-09-25. Gốc: `2ebeb0b` (hợp đồng lõi agent). protocol_version: không áp dụng (không đổi `docs/protocol/v1`).
- Verdict: chưa có reviewer thật. Mọi số dưới đây đo trên máy này, cây làm việc của nhánh; commit message ghi lại số đo ở SHA cuối.
- Nguồn: `research/sdlc-production.md` (§C, §D, §E3 và mục Kiểm chứng), `research/milvus.md` (Kiểm chứng #3–#4, #8–#9),
  `research/gemini-embedding-2.md` (Kiểm chứng #2–#4), `research/milco.md` (Kiểm chứng #11, #15); thiết kế 04 §3, §6, §8.

## Cái gì đã dựng

| Chặng | Ở đâu | Ghi chú |
|---|---|---|
| S0 bắt thay đổi | `schema_nap_1.sql`: trigger `rag_nap_places_dirty` trên `places` → `rag_dirty` (một hàng mỗi tài liệu, `lan` đếm, `noticed_at` giữ lần đổi đầu) + `jobs_them('rag', …)` mỗi phút một tin | lane `rag` mở bằng outbox v2 (`jobs/schema_2.sql`, `SchemaVersion` = 2) |
| S1–S2 | `hoso.go`: `promptsafety.SafeDeep` trên mọi trường, `source_hash` trên hàng an toàn | hàng bị bỏ → tombstone `unsafe` |
| S5 làm giàu | `lamgiau.go` + `lamgiau_prompt.txt`: ≤20 quán/lời gọi, datamarking `<du_lieu nguon="quan" bi_danh="pN">`, enum đóng từ **id** của `tuvung` + `khong_ro`, nhãn `chen_lenh` của model, `llm.Dem` đếm mọi lời gọi và retry, trần bắt buộc | không danh sách từ nào đọc chữ của quán |
| S5r duyệt | `place_enrichments(review auto|reviewed|rejected, can_duyet)`, `core rag v-review list|approve|reject` | seed/curated 100%; mọi nhãn dị ứng/ăn kiêng, tin cậy ≠ cao, `chen_lenh`, `khong_ro` vào hàng; ≈5% mẫu tất định còn lại |
| Luật áp | `ApDung`: dị ứng chỉ thêm loại trừ; làm giàu cũ/bị từ chối/bị cờ/không có ⇒ mất «dị ứng đã rõ» (quán bị ẩn với người dị ứng); ăn kiêng vào lọc cứng chỉ khi `reviewed` | test thuộc tính `TestDiUngChiThemLoaiTru` |
| S6 dedupe | `trung.go`: cùng điểm đến, haversine ≤80 m **và** cosine dense ≥0.92; curated > seed > osm | không so chuỗi tên |
| S7 chunk | `ChunkID = hex(sha256(doc‖facet‖chunker))[:32]`, khoá chính VarChar, không autoID | ghim bằng test |
| S8 | dense qua `NhungTaiLieu` (adapter cửa `nhung` ở `cmd/core`, tiền tố GE2 trong chữ), cache `rag_embedding_cache`, kiểm `len == dims` + L2; sparse = hàm BM25 của Milvus, MILCO sau cờ `milco_bat` (mặc định tắt) | cửa `nhung` hôm nay là `gemini-embedding-001`/768 nên lệnh thật **từ chối** tới khi cửa chuyển GE2/1536 |
| S9 | `rd_<corpus>__v<N>` (tên sinh từ id Postgres), chỉ mục tạo và chờ từng cái | SDK `CreateCollection` nuốt lỗi chỉ mục (`return nil`) nên không dùng đường đó |
| S10 đối soát | 0 thiếu, 0 thừa, 0 lệch hash, `count(*)` strong; sự thật **tính lại từ Postgres**, không đọc từ collection đang kiểm | lỗi này bị test bắt trong lúc làm (bản đầu đọc tập doc từ collection) |
| S11 cổng | dò lọc cứng trên chính collection (228 phép dò/3 điểm đến) + tập vàng dựng tạm với cùng cấu hình | ngưỡng trong `cauhinh.json`, violation không dung sai |
| S12 | promote/rollback = đổi alias dưới advisory lock + đọc lại alias; bộ đối chiếu 60 s đưa alias về Postgres; dọn bản retired quá 3 bản/14 ngày | tự promote chỉ khi ≤5% tài liệu đổi và cùng vân tay cấu hình |
| Vận hành | `core migrate-rag-vector`, `core rag v-build|v-eval|v-promote|v-rollback|v-status|v-enrich|v-review|v-dlq|v-index|v-reconcile`, `core rag-indexer` | trạng thái chỉ số và id phiên bản, không chữ |

## Số đo

- Tập vàng (143 câu, 52 quán neo + 240 quán nền), embedder stub 1536 chiều, kho trong bộ nhớ (ghim):
  recall@10 0.9267, nDCG@10 0.8285, MRR@10 0.8124, violation@10 0, khoảng không dấu 0.
  Chỉ dense 0.8133/0.7340/0.7171; chỉ BM25 0.9800/0.8973/0.8762 với 1 vi phạm.
- Cùng tập trên Milvus thật (HNSW + BM25 function, RRF k=60): 0.9267/0.8292/0.8131, violation 0.
  Kho trong bộ nhớ khác Milvus ≤0.0007 ở nDCG — cái giả được đo, không chỉ được tin.
- 143 bộ lọc câu vàng + 228 phép dò: Milvus nhận **đúng** cùng tập chunk với bộ lọc viết bằng Go.
- Canary: bỏ lọc dị ứng → violation@10 0.5315 (133 vi phạm), bỏ điểm đến → 0.4965; dò lọc khi bỏ dị ứng → 55 vi phạm.

## Điều cần người đọc

1. **Vàng tự mâu thuẫn một chỗ**: d08 liệt `ha-cao-lau-gieng-co` vào `phai_loai` trong khi nhãn tay của quán nói không gluten
   (sợi gạo, sửa ở lát 8 vòng 1). Chỉ nhánh BM25 đơn đẩy nó vào top 10; cổng đọc hybrid. Cần người sửa tập vàng.
2. Stub dense yếu nên hybrid < BM25 đơn trên vàng; nghiên cứu đòi «hybrid+rerank ≥ nhánh tốt nhất − 0.01» — chỉ đo được với embedding thật.
3. `.repo-guard-allowlist.json`: hai mục **có sẵn** `services/core/go.mod`/`go.sum` được ghim lại digest vì thêm SDK Milvus
   (quyết định chủ sản phẩm); `go.sum` thêm luật `aggregate-base64-fragments` (checksum h1:). Không thêm mục mới.
4. Cổng aigate: allowlist của rag thêm 7 bảng của `nap` + đọc `job_schema_migrations`; bỏ qua phương thức `Error` trong luật
   «không chạm gói cấm» vì bước đi quy mọi `err.Error()` trong `aiharness/llm` về mọi `Error` của module (chỉ định dạng chữ).
5. `retrieval-core` chưa có commit: `KhoVector` khai trong `nap`, adapter ở `nap/milvuskho`; lúc gộp chuyển adapter ra sau gói `vectordb`.
   Tên trường schema do `nap` (người ghi duy nhất) định; đường đọc phải dùng đúng tên này.

## Còn mở

Embedding GE2 thật và lượt làm giàu thật (cần khoá, Lead duyệt số lời gọi); MILCO (license chưa đọc được); đường truy hồi đọc alias
(retrieval-core); dedupe tăng dần (chỉ chạy ở lượt dựng đầy đủ); compose/Docker cho `migrate-rag-vector` và `rag-indexer`, và đường
container dùng một lần của `go_milvus_tier.sh` chưa chạy (máy không có Docker; tầng đã chạy với Milvus cục bộ); bootstrap CI cho
«không kém active» (hiện so điểm, dung sai 0.01); drift/giám sát metric Prometheus chưa nối.
