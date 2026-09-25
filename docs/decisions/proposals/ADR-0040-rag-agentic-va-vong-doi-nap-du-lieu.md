# ADR-0040 — RAG agentic trên retriever lai có kiểu, và vòng đời nạp dữ liệu

- Ngày: 2026-09-25.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0040 khác thì văn bản
  này nhận số trống kế tiếp, như ADR-0036 từng đổi số.
- Quyết định sản phẩm: người dùng chốt trong phiên lập kế hoạch 2026-09-25. Hai điều người dùng quan tâm
  nhất là cơ chế RAG agentic nào tốt nhất, và dữ liệu được nạp theo quy trình nào. Embedding bằng model
  Gemini (không phải model chat) được chấp nhận. Kế hoạch đã duyệt; ADR này chờ chữ ký Lead.
- Thiết kế chi tiết: `docs/claude/2026-09-25/thiet-ke-ai/04-rag-va-nap-du-lieu.md` (commit gốc `f251db7`).
- Cùng đợt: ADR-0037 engine, ADR-0038 hàng đợi và stream, ADR-0039 nhóm trong luồng, ADR-0041 Nếp, ADR-0042
  eval. Văn bản này chỉ quyết truy hồi và nạp dữ liệu.
- Sửa và thay một số điều khoản cũ, liệt kê ở mục 5; **không sửa bản lịch sử** của ADR nào.
- Không đổi bởi văn bản này: ba luật tiền; ADR-0017 §2.2 và §2.4 (chỉ script nhập gọi OSM/Wikimedia);
  ADR-0029 §2.7 (brain nội bộ, không credential DB); ADR-0033 §2.2; ADR-0036 §2.5 (ADR-0039 sửa riêng cho
  chip xem trước).

## 1. Bối cảnh

- Eval nhóm 10/16. Bốn ca trượt (01, 16 dị ứng; 03, 13 giờ/giá) hỏng ở **ràng buộc**, không phải ở độ gợi
  nhớ. Top-k theo độ giống không ép được ràng buộc.
- `ModelPlaceRows` luôn lấy điểm đến có `sort_order` nhỏ nhất (`service/catalogue.go:309-310`), nên nhóm ở
  TP.HCM được đưa 40 quán Đà Lạt. Ba route có oracle Python cũng gọi hàm này.
- `/places/search` nạp mọi quán và gửi hết sang brain (`routes/places_wai.go:387`, `:405`).
- `promptsafety.Safe` không kiểm mô tả, review, activities (`promptsafety.go:98`), mà đó là chỗ chữ lạ dễ vào.
- Không có index chữ hay vector; giờ mở cửa là chuỗi tự do; chưa có sổ tay app nên Nếp không trả lời được
  «muốn làm X thì bấm đâu».
- ADR-0017 §2.3 cấm nhờ AI điền cột không biết của `places`.

## 2. Quyết định

1. **Cơ chế là Adaptive-CRAG trên retriever lai có kiểu, toàn bộ trong Go.** Understand của engine (ADR-0037)
   chọn nguồn; truy hồi lọc cứng rồi xếp hạng; chấm lại; sửa đúng một vòng; rồi trả lời, hỏi lại **một**
   câu, hoặc từ chối và nêu ràng buộc không thoả. RAG **không** là orchestrator: không router riêng, không
   vòng lặp riêng. Nó là ruột của `search_places`, `get_place`, `list_group_outings`, `search_app_manual`,
   `explain_screen` trong registry duy nhất `aiharness/tools`.
2. **Lọc cứng không bao giờ nới:** điểm đến, dị ứng, giờ đã nêu, trần ngân sách (số nguyên đồng), ăn kiêng,
   tombstone. Lọc chạy trong SQL trên chỉ mục, rồi Go kiểm lại trên hàng `places` sống sau khi hydrate. Id
   không còn trong `places` bị bỏ. Chỉ khí chất, loại chỗ và khu vực trong điểm đến được nới, theo thứ tự đó.
3. **Xếp hạng lai.** Từ vựng (`tsvector` + `pg_trgm` trên chữ đã `promptsafety.Fold` bằng Go, không `unaccent`);
   vector `gemini-embedding-001` 768 chiều qua pgvector (từ lát 16); gộp RRF k=60 bằng số nguyên. Chấm lại bằng
   `gemini-3.5-flash-lite` chỉ khi hơn k ứng viên, trên bí danh id có enum, đầu ra không vào prompt nào.
   «Viết lại truy vấn» là mở rộng từ vựng tất định, không phải chữ model viết.
4. **Điểm đến không bao giờ mặc định im lặng.** Giải tất định từ vùng, tên, phiếu, rồi kèo của nhóm; không
   rõ thì hỏi lại. Lỗi «luôn Đà Lạt» sửa **chỉ trên đường engine Go**; `ModelPlaceRows`, ba route có oracle và
   Python `model_place_rows` giữ nguyên.
5. **Trần.** `search_places` 3 s trọn gói; ≤2 lời gọi model bên trong, đếm vào `MaxModelCallsPerTurn` của
   `aiharness/llm`; SQL `statement_timeout` 800 ms trong tx ReadOnly; bằng chứng ≤12k ký tự.
6. **Kiểm câu trả lời không đi đường riêng.** Luật cục bộ (giá/giờ gần `[[p:ID]]` khớp ledger, «nhãn nút» có
   thật trong sổ tay) chạy trong output guard cửa sổ 48 rune của ADR-0037, nơi duy nhất sinh `delta`. Luật cấp
   thẻ chạy trong `GroundReply` trên đúng ledger của lượt. Không «stream tạm rồi thay», không rút lại, không
   sinh lại sau delta đầu.
7. **Schema do Go sở hữu.** Gói `internal/rag` là writer duy nhất của `rag_*`, có bảng version riêng
   `rag_schema_migrations`; số version cấp theo thứ tự lên main. Không bảng `rag_*` nào có `person_id`.
   `rag_query_log` chỉ id, enum, số, thời gian; xoá sau 30 ngày. Chỉ mục có version; promote/rollback lật
   `state` trong một transaction; tombstone nằm ngoài version nên rollback không làm sống lại tài liệu đã gỡ.
8. **Vòng đời nạp dữ liệu quán, 11 bước:** hợp đồng nguồn `place.v1` → an toàn (`promptsafety.SafeDeep` trên
   mọi trường chữ; tên/địa chỉ bẩn thì bỏ hàng, trường khác bẩn thì cách ly) → chuẩn hoá (`Fold`, song âm tiết,
   từ vựng đóng, giờ mở cửa thành `int4multirange`) → làm giàu → dedupe → embed (cache theo hash) → build →
   cổng eval → promote/rollback → độ tươi (trigger `rag_dirty` + indexer định kỳ, DELETE tombstone ngay) →
   giám sát. Nạp là CLI (`core rag …`), không phải lane hàng đợi.
9. **Cổng promote:** quán recall@10 ≥0.90, nDCG@10 ≥0.75, MRR@10 ≥0.70, **violation@10 = 0**; sổ tay recall@5
   ≥0.90; mọi số ≥ bản active − 0.01; đổi model hoặc chunker thì promote tay. CI chỉ chạy stub embedder và số
   tất định; eval thật cần Lead duyệt số lời gọi.
10. **Sổ tay app là dữ liệu sản phẩm, nhúng vào core.** `services/core/internal/huongdan/data/*.md` qua
    `go:embed`; nhãn, route và điều hướng rút từ mã; người viết văn; cổng lệch đòi mọi «nhãn» là literal có
    thật trong `apps/mobile`; hash bản build đi trong phiếu; màn tiền chỉ có đoạn điều hướng.
11. **Làm giàu là tín hiệu truy hồi, không phải sự thật của quán.** Action brain `place-enrich` (extraction lúc
    nạp dữ liệu, flash-lite, schema enum, không credential DB) trả nhãn; Go kiểm enum, chạy `SafeDeep`, ghi
    `place_enrichments`. Không bao giờ ghi vào cột của `places`, không lên wire như sự thật. Dị ứng suy ra chỉ
    để **loại**; ăn kiêng suy ra chỉ vào lọc cứng khi đã có người duyệt.
12. **Lớp «dữ liệu công khai do máy chủ sở hữu»:** catalogue đã qua `SafeDeep`, làm giàu, sổ tay app. Đọc lớp
    này **không phải** đọc hội thoại, gu hay lịch sử theo nghĩa ADR-0036 §4 và ADR-0033 §4. Việc bật tool đọc
    lớp này cho Nếp thuộc ADR-0041, dẫn chiếu điều này.
13. **Trí nhớ và lịch sử.** `rag` chỉ cho `nepnho` một thư viện xếp hạng thuần; không import `nepnho`, không
    nêu bảng `nep_*`; `recall_memory` chỉ tới được từ gốc scope=me. Lịch sử chuyến là SQL có kiểu theo
    `context_id` của job, chỉ cho nhóm; model không truyền đối số danh tính.
14. **Ngoại lệ có tên:** `/places/search` (LIVE-GO) gửi brain shortlist ≤30 hàng thay vì cả danh mục. Đây là
    lệch Go-only trên payload brain; parity chạy không khoá nên không thấy. Bằng chứng là test Go trên payload.
15. **Ảnh Postgres** tự build trên `postgres:16-alpine@sha256`, thêm pgvector từ tarball ghim; giữ musl để không
    phải REINDEX volume cũ; vào `check_dockerfile_pinning.sh`; 7 chỗ ghi cứng ảnh đưa về
    `MOBILE_TEST_POSTGRES_IMAGE`.

## 3. Hệ quả

- Một gói Go mới, ba gói domain thuần (`thoigian`, `giomo`, `tuvung`), hai hàm mới trong `promptsafety`
  (`Fold` export lại, `SafeDeep`); `Safe`/`Filter` và bản Python không đổi vì là oracle.
- Trigger Go thứ hai trên một bảng Alembic (`places`), theo tiền lệ `chatlegacychange/schema.sql:52`. Câu hỏi
  chủ schema sau decommission (ADR-0029 §8.1) giờ có thêm một phụ thuộc cần ghi.
- Compose thêm `migrate-rag`; service `postgres` đổi từ `image:` sang `build:` ở lát 16. Lệnh
  `docker compose up -d postgres` của CLAUDE.md vẫn đúng.
- Thiếu pgvector thì migration vector fail closed và phục vụ chỉ-từ-vựng; thiếu schema từ vựng khi engine nhóm
  chạy Go thì `work` từ chối khởi động.
- Chỉ mục cũ chỉ làm giảm độ gợi nhớ; giá, giờ, dị ứng luôn kiểm trên hàng sống.
- Hợp đồng chung cần thêm một dòng cho lời gọi embedding (đề xuất `MaxEmbedCallsPerTurn = 2`) trước lát 16.

## 4. Cái này KHÔNG cho phép

- Không nới dị ứng, giờ đã nêu, trần ngân sách, ăn kiêng hay điểm đến, kể cả khi không còn kết quả.
- Không mặc định điểm đến im lặng; không đưa quán của điểm đến khác khi chưa hỏi.
- Không để chữ tự do do model viết đi vào truy vấn, chỉ mục hay prompt kế tiếp.
- Không để AI điền cột của `places`, không hiện làm giàu như sự thật, không dùng nó để khẳng định «không có» chất
  gây dị ứng.
- Không cho `rag` đọc `messages`, `chat_v2_events`, bảng tiền, `person_interests`, `saved_places`, `posts`,
  `pair_shared_constraints` hay `nep_*`.
- Không ghi chữ truy vấn hay chữ bằng chứng vào log, metrics hay bảng nào.
- Không đưa dữ liệu địa điểm thật, bản tải OSM hay làm giàu vào Git; tập vàng chỉ dùng dữ liệu tổng hợp.
- Không promote một version trượt cổng; không rollback làm sống lại tài liệu đã tombstone.
- Không thêm router, vòng lặp hay registry tool thứ hai trong `rag`.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

| Điều khoản | Hiện nói | Sau ADR này |
|---|---|---|
| ADR-0036 §2.3 (quyết định 3) | Máy chủ đắp roster, gu nhóm, ngân sách, lịch sử chuyến, catalogue đã qua `promptsafety` | Thêm sổ tay app và làm giàu quán. «Qua `promptsafety`» trên đường engine nghĩa là `SafeDeep`. Lịch sử chuyến và catalogue tới qua tool có kiểu, không đắp sẵn 40 dòng |
| ADR-0036 §4, dòng «Không cho Nếp đọc hội thoại nhóm, gu, hay lịch sử» | Không nói gì về dữ liệu công khai | Giữ nguyên ba thứ bị cấm. Dữ liệu công khai do máy chủ sở hữu (§2.12) không thuộc ba thứ đó; quyền đọc cụ thể của Nếp do ADR-0041 bật |
| ADR-0017 §2.3, «không nhờ AI điền» | Cột không biết để trống | Giữ nguyên cho mọi cột `places`. Làm giàu ở bảng riêng, chỉ là tín hiệu truy hồi (§2.11) |
| ADR-0017 §2.2, ODbL | Hàng OSM ghi nguồn | Áp thêm cho làm giàu phái sinh từ hàng OSM: giữ `license`, không vào Git |
| ADR-0029 §2.7, danh sách việc của brain | Brain làm bước model cho route AI | Thêm action `place-enrich` (extraction lúc nạp), đúng ràng buộc của §2.7. Khớp ADR-0037 §2.13 |
| Hành vi `POST /places/search` | Gửi cả danh mục (khớp Python) | Shortlist ≤30 hàng; ngoại lệ Go-only có tên (§2.14) |

Không đổi bởi văn bản này: ADR-0033 §2.5 và §4, ADR-0036 §2.7 (thuộc ADR-0041); hình dạng thẻ `tra_loi`
(ADR-0039); grounding của `POST /messages` (`GroundCard`, oracle).

## 6. Cổng nghiệm thu

- Lát 8: tập vàng truy hồi tất định trong CI; canary injection trong review, shortlist ≤30 hàng trên fixture 5k
  quán, tombstone sống qua rollback; `scripts/go_postgres_tier.sh` (skip là đỏ) có migration, multirange, dị ứng,
  ngân sách, promote/rollback.
- Lát 9: canary nhóm HCM chỉ nhận bằng chứng `d-tphcm`; canary fact trí nhớ đã seed không có trong request nhóm;
  eval T3 lõi ≥14/16 ổn định qua 5 lần, ca 01, 03, 13, 16 đạt cả 5 lần.
- Lát 13: `huong-dan-khop-ma.test.mjs` xanh, đỏ khi một «nhãn» không còn trong mã.
- Lát 16: ngưỡng promote ở §2.9 trên eval thật đã được Lead duyệt số lời gọi.
- Mỗi lát: ít nhất hai đột biến tự nghĩ, kiểm tương đương trước, đỏ đúng bước dự đoán; chạy lại trong cây
  sạch đúng SHA; số đo ghi thẳng vào commit message.
