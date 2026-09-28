# Truy hồi lai trên Milvus: nhúng, chỉ mục, kiểm lại, xếp lại (nhánh `infra/retrieval-core`)

Ngày 2026-09-25. Gốc: `2ebeb0b` (hợp đồng lõi agent). Commit: xem `git log` của nhánh (một commit
không ghi được SHA của chính nó). protocol_version: không áp dụng. Verdict: chưa có reviewer.

## Cái gì đã dựng

| Gói | Việc | Test chứng minh |
|---|---|---|
| `aiharness/nhung` | `gemini-embedding-2`, 1536 chiều cho mọi collection; tác vụ viết vào văn bản (`task: search result \| query: …`, `title: … \| text: …`), không đặt TaskType/Title; kiểm `len(values)==1536`; `nhung.Dem` trần `MaxEmbedCallsPerTurn`, nhớ câu hỏi đã nhúng trong lượt | `TestGeminiWireContract` đọc body thật SDK gửi lên fake loopback; `TestGeminiRefusesAVectorOfTheWrongLength` |
| `nhungcache` | Cache Postgres cho nội dung công khai; khoá sha256(model, dims, prompt_version, prompt NFC có tiền tố); chỉ `places`/`manual`, CHECK ở bảng chặn cả ghi tay | `TestCacheRoundTripAndBoundaries` (tầng postgres) |
| `vectordb` | Milvus `client/v3` v3.0.0; một hàm dựng `ClientConfig`, telemetry `&TelemetryConfig{Enabled:false}`; schema places/manual/memories bằng mã; lọc cứng chỉ qua tham số template; `alias__vN` sau alias, cấm collection trùng tên alias; xoá + flush + nén L0 rồi mix; kho trí nhớ theo chủ (`ChuSoHuu`), không chữ | tầng `go-milvus` (dưới) |
| cổng thưa | `BM25` của Milvus (mặc định, analyzer standard + lowercase + asciifolding trên NFC) và `MILCO` qua HTTP tới `services/ai-infer` sau `MOBILE_MILCO_ENABLED=1` (mặc định tắt) | `TestMILCOContract` trên fake loopback |
| `thuoctinh` | Bảng `dia_diem_thuoc_tinh` (dị ứng/ăn kiêng/khung giờ do trích bằng LLM lúc nạp), một writer, bảng phiên bản riêng có checksum; `Doc` đọc hàng sống nối `places` | `TestGhiDocAndTheLiveRow` (tầng postgres) |
| `hybrid` | `truyhoi.Retriever`: một lần nhúng câu hỏi, Milvus dense + thưa RRF k=60 với lọc cứng trong biểu thức, kiểm lại mọi hit trên hàng Postgres sống (hỏng thì bỏ và đếm, lỗi DB thì từ chối), cờ `NoVector`/`NoSparse`/`NoRerank`, `BiLoai` đếm bằng `count(*)` theo từng ràng buộc | unit trên `vectordb.Fake`; `TestHybridDauCuoiKhongViPham` live |
| `rerank` | `truyhoi.Reranker` qua `POST /rerank`: kiểm index trong khoảng và không trùng, timeout riêng, không retry trong lượt, cầu dao 5 lỗi/30 s mở 60 s, giữ thứ tự RRF khi hỏng, lọc token đặc biệt của model, không log body; `rerank.Dem` trần `MaxRerankCallsPerTurn` | unit trên fake; `TestRerankGoldenQuaServer` live |

Hợp đồng (`truyhoi`, `trinho`, `testkit`, `tools`, `hieu`) **không đổi**. Chỉ `nhung.Nhung` được
**thêm** phương thức `NhungTaiLieu`, và giá trị của `nhung.TacVu` đổi từ chuỗi taskType của Gemini
sang id trung tính (`tai_lieu`, `cau_hoi`, `giong_nhau`) vì không còn gửi taskType.

## Hợp đồng wire MILCO cho `services/ai-infer` (phía Python chưa viết)

`POST {MOBILE_AI_INFER_URL}/v1/milco/encode`, loopback, body `{"kind": "query"|"doc", "texts": [...]}`
(văn bản đã NFC, ≤32 mỗi lô), trả `{"vectors": [{"indices": [...], "values": [...]}]}` đúng một
vector mỗi văn bản, đúng thứ tự; index tăng ngặt, < 2^30; value dương hữu hạn; ≤512 phần tử khác 0
(dịch vụ tự prune; Go từ chối chứ không cắt). Timeout 800 ms, không retry, hỏng thì cờ `NoSparse`.

## Số đo (máy chung 4 CPU, Milvus v3.0.2 cục bộ không Docker, reranker q8_0)

- Không vi phạm ràng buộc cứng: 120 bộ ràng buộc ngẫu nhiên × 3 dạng nhánh (lai, chỉ dense, chỉ BM25),
  5008 hit, 0 vi phạm; với mỗi bộ, `count(*)` theo biểu thức bằng đúng số hàng luật Go chấp nhận.
- Đầu cuối qua Milvus + Postgres với chỉ mục cố ý cũ (thêm dị ứng, gỡ, xoá, đổi giá 40/160 quán):
  60 lượt truy hồi, 545 mục đối chiếu hàng sống, 0 vi phạm, bước kiểm lại bỏ 199 hit cũ.
- Lai thắng dense-only trên bộ đồng nghĩa: lai 1.000, dense-only 0.500, BM25-only 0.750 (16 truy vấn).
- Reranker golden qua adapter: 3 ca, lệch lớn nhất so với tham chiếu numpy 1.35e-2 (dung sai 0.02), thứ hạng trùng.
- Telemetry: client canary bật telemetry hiện trong danh sách `GetClientTelemetry` của máy chủ; client dựng từ cấu hình của `vectordb` không hiện.

## Phát hiện mới khi chạy thật (chưa có trong báo cáo bring-up)

1. Tìm vector ngay sau upsert trả 0 hit trong khoảng 184–208 ms dù đọc `Strong`, trong khi `count(*)`
   `Strong` đã thấy hàng. Test chờ chỉ mục (`choThay`); sản phẩm không cần (chỉ mục là bản sao dẫn xuất).
2. Hybrid search đọc sub-request theo consistency mặc định của collection, bất kể request ghi `Strong`:
   collection tạo `Bounded` trả rỗng cho hàng vừa ghi. `Milvus.NhatQuan` giờ đặt cả mức của collection.
3. Flush bị giới hạn 0.1/s mỗi collection; `NenVaCho` phải flush trước khi nén L0 (xoá còn trong delta
   đang lớn thì nén L0 không lấy được), và `Flush` thử lại khi bị rate limit (đường bảo trì, không phải lượt).
4. `encoding/json` thoát `<` thành `<`: test "không gửi token đặc biệt" ban đầu soi byte thô nên
   không bao giờ đỏ; đột biến R3 phát hiện, test đã đổi sang đọc chuỗi đã giải mã.

## Còn mở

- Gọi Gemini thật (không có key; ADR-0034 §2.6): wire đã ghim bằng fake, số chất lượng tiếng Việt chưa có.
- MILCO: license trọng số chưa xác nhận (splade-v3 CC BY-NC-SA qua head), chưa đo tiếng Việt; phía Python chưa viết.
- Bước làm giàu bằng LLM ghi `dia_diem_thuoc_tinh`, worker nạp outbox → Milvus, đối soát, cổng eval §C4.
- Nối `hybrid.Kho` vào engine/tool (việc của nhánh agent-core), và `DocHuongDan` cho nguồn sổ tay.
- Đường Docker của `go_milvus_tier.sh` chưa chạy (không có Docker); bước CI đỏ tới khi runner có reranker.
- Lead: reranker 4B hay giữ 0.6B; hạ `dataCoord.gc.dropTolerance` cho SLO xoá vật lý; gỡ bộ đọc dị ứng
  bằng từ khoá của `rag` (ADR thay thế).
