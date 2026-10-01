# ADR-0051 — Milvus là bản phục vụ cho tìm quán: bỏ bước kiểm lại Postgres mỗi lượt tìm, đổi bằng đồng bộ theo sự kiện và SLO độ tươi

- Ngày: 2026-10-01.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-10-01** trong phiên làm việc. Ghi lại các câu trả lời: chấp nhận
  chỉ mục lệch «vài giây tới vài chục giây», **kể cả dị ứng và quán đóng**; trường hiển thị lưu trong `mo_rong`;
  cache vector câu hỏi dùng phương án hợp techstack nhất; cảnh báo bằng healthcheck + log + `v-status`. Không phải
  chữ ký Lead.
- Thay: `docs/architecture/03` §8.4 ở đúng một điểm: «mỗi hit được kiểm lại trên hàng sống của PostgreSQL». Phần
  còn lại của §8.4 giữ nguyên: lọc cứng không nới, thuộc tính chưa biết bị loại khi có ràng buộc trên nó, và được
  ghi «chưa rõ» khi không có ràng buộc. **Không sửa bản lịch sử** của tài liệu nào.
- Dựa trên: ADR-0047 (vòng đời nạp), ADR-0049 (nhà cung cấp AI v3, Milvus).

## 1. Bối cảnh

Đo trên `rd_places__v2` (10.384 quán, 3072 chiều), 5 câu thật: đầu-cuối **987 ms p50 / 1.312 ms p95**. Trong đó,
bước kiểm lại Postgres (máy vnlocal qua WiFi, khoảng 7 lượt đi-về) chiếm **351 ms p50 / 1.257 ms p95**; Milvus chỉ
1 ms p50.

Bước kiểm lại tồn tại vì không tin được Milvus:

- indexer chỉ chạy mỗi 60 s;
- tombstone không bao giờ tới Milvus;
- đổi chữ mà embed lỗi thì thuộc tính mới (dị ứng) cũng không tới;
- ingest hỏi vnlocal mỗi 10 phút.

Nghiên cứu các hệ production (Debezium outbox, Grab, DoorDash, Uber Eats, Instacart, Zalando, Pinterest, Elastic,
Pinecone, Milvus) đều theo cùng một mẫu, không có hệ nào đọc lại DB gốc cho từng ứng viên:

- chỉ mục mang đủ trường lọc và hiển thị;
- giữ chỉ mục tươi bằng outbox/CDC theo sự kiện;
- đo và cảnh báo độ tươi.

## 2. Quyết định

1. **Milvus là bản phục vụ** của tìm quán. Mỗi hàng quán mang **trường hiển thị** (`mo_rong.hien_thi`:
   `ten, loai, diem_den, dia_chi, gio, gia_min_vnd, gia_max_vnd, gia, chua_ro`) do ingest ghi cùng hàng
   (`nap.TruongHienThi`). Một hit có trường hiển thị được trả thẳng từ Milvus: bộ lọc của chính lượt tìm đã giữ
   ràng buộc cứng.
2. **Độ lệch được phép**: tối đa **60 s** so với PostgreSQL, **kể cả dị ứng và quán đóng** (chủ sản phẩm chấp
   nhận). Số này là SLO, được đo chứ không giả định (`nap.DoTuoi`):
   - `rag-indexer` trả `/slo` 503 khi lệch > 60 s kéo dài ≥ 2 phút, khi có quán vào DLQ, hoặc khi ingest không xong
     vòng nào trong 3 phút;
   - healthcheck compose đọc `/slo`; log ghi WARN `slo_vi_pham`;
   - `core rag v-status` in đủ các số.
3. **Giữ chỉ mục tươi theo sự kiện**:
   - trigger trên `places`, `rag_tombstones`, `place_enrichments` gọi `rag_danh_dau` (đánh dấu + `NOTIFY rag_dirty`);
   - indexer LISTEN, chạy lượt trong vài giây, chỉ ghi phần khác (hash + dấu vân tay), và ghi thuộc tính ngay cả khi
     vector chưa có;
   - ingest LISTEN `rudi_doi` từ vnlocal (handoff vnlocal PR #8) và hỏi dự phòng mỗi 20 s;
   - một quán vào DLQ bị xoá khỏi mọi collection (fail closed).
4. **Hit thiếu trường hiển thị** (hàng ghi trước quyết định này, hoặc collection được rollback về) vẫn được đọc
   sống và kiểm lại như cũ (`thuoctinh.Doc`, cùng luật với ingest). Thiếu bộ đọc sống thì hit đó bị bỏ, không bao
   giờ hiện mà chưa đọc.
5. **Trường `gia`** là chuỗi giá đúng như câu trả lời viết (`35.000–60.000 đ/người`, «khoảng … (ước)» khi
   `place_facts.gia_uoc`). Trước đây bộ kiểm trích dẫn so trường `gia` nhưng bằng chứng quán không có khoá này, nên
   mọi câu trích giá đều trượt.

## 3. Hệ quả

- Tìm quán không còn lượt đọc Postgres nào khi mọi hit có trường hiển thị. Độ trễ còn lại là embedding câu hỏi
  (P5: cache vector câu hỏi) và Milvus.
- Một thay đổi ở vnlocal tới được ô tìm của app trong vài giây, tối đa 20 s cộng một lượt indexer khi chưa có
  trigger phía vnlocal. Khi vượt 60 s, healthcheck chuyển `unhealthy`.
- Rủi ro được chấp nhận: trong ≤ 60 s, một quán vừa đóng hoặc vừa thêm dị ứng vẫn có thể được gợi ý.
- Rollback: đưa alias về collection cũ (hàng không có `hien_thi`) thì đường đọc sống tự chạy lại, không cần đổi mã.
- Bổ sung trường hiển thị cho bản đang phục vụ: `core rag v-danh-dau-lai place`. Lệnh này đánh dấu nền mọi quán mà
  collection đang giữ; indexer cập nhật một phần, **0 lượt gọi API**.
