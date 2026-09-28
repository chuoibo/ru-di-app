# Hợp nhất truy hồi + nạp trên Milvus (nhánh `infra/rag-unified`)

- Nhánh: `infra/rag-unified`, dựng trên `1b12e8c` (lõi agent), rebase lên `2bfd0c8` (sidecar ai-infer +
  trí nhớ `nepnho`/`aictx`) để fast-forward. Gộp `infra/retrieval-core` (`116206d`, review APPROVE) và
  `infra/ingest-sdlc` (`184abe0`, review REQUEST_CHANGES) bằng cherry-pick, rồi sửa. SHA cuối: xem commit
  message (một commit không ghi được SHA của chính nó).
- protocol_version: không áp dụng (không đụng `docs/protocol/`).
- Verdict: chưa có reviewer; đây là ghi chép của người làm.
- Review gốc: `review.md` của lượt review phản biện hai nhánh (F1–F13 cho nạp, F1–F10 cho truy hồi).

## Quyết định của chủ sản phẩm áp trong nhánh (2026-09-27)

- Embedding chỉ `gemini-embedding-2` 1536, tiền tố một lần trong `aiharness/nhung`; không còn
  `gemini-embedding-001`/768 trong mã, test, tài liệu kiến trúc; kho ví dụ router nhúng qua cùng cửa.
- Không HyDE: viết lại truy vấn chỉ là trường structured output của router (03 §8).
- MILCO gác lại: nhánh thưa là BM25 của Milvus (hai trường); adapter MILCO tắt mặc định, gỡ khỏi đường
  truy hồi của engine (`cmd/core` luôn dùng `vectordb.BM25`), không tầng/test nào cần nó.

## Quyết định

1. **Một schema, một writer.** Schema của `internal/vectordb` thắng (retrieval-core vào trước, nạp theo):
   `rd_places__vN`/`rd_manual__vN` sau alias `rd_places`/`rd_manual`; mỗi hàng là một chunk (khoá = chunk id tất
   định của nạp), `doc_id` là quán; thuộc tính lọc cứng dạng của vectordb (`allergens` với `khong_ro`,
   `price_min = -1` khi chưa rõ, `open_slots` nửa giờ). Nạp đổi ô giờ 15 → 30 phút, bỏ các trường mềm khỏi
   Milvus, bỏ `Loc.Man` không ai dùng. `rag/nap/milvuskho` (hàm dựng client thứ hai) bị xoá; adapter duy nhất là
   `internal/vectordb/napkho` trên `vectordb.Ket`. Cấu hình nạp ghi `luoc_do: "rd.v2"`; `napkho.KiemKhop` từ
   chối khi lệch bản schema, số chiều, RRF, ô giờ, alias.
2. **Kiểm lại đọc đúng bảng nạp ghi.** `thuoctinh` không còn bảng riêng (`dia_diem_thuoc_tinh` chỉ test ghi)
   — nó là bộ đọc: `places` + `place_enrichments` qua `nap.ApDung` + `rag_tombstones`. Giờ mở kiểm lại tính từ
   giờ sống bằng cùng hàm ô giờ của nạp.
3. **Luật «chưa rõ»**: giữ + cờ khi ràng buộc đó không được hỏi, loại khi ràng buộc đó cứng
   (`docs/architecture/03` §8.4); dị ứng không rõ luôn loại dưới bộ lọc dị ứng. Áp ở biểu thức Milvus, `Dat`,
   `nap.Loc.Khop`, và lọc lại ở fallback lexical (`aidoc.GiuChuaRo`); `/places/search` giữ §4a.
4. **Hybrid nối vào engine**: `cmd/core` `quanRetriever` — có `MOBILE_MILVUS_ADDR` thì
   `hybrid.DuPhong{hybrid.Kho, aidoc.Lexical}`, không có thì `aidoc.Lexical`. Ngân sách nhúng của lượt đi theo
   context (`nhung.VoiDemLuot`/`nhung.TheoLuot`), một ngân sách cho kho ví dụ router và mọi truy hồi.
   `hybrid.DocSong` là interface để cổng đọc tĩnh (aigate) đi theo được vào bảng; `place_enrichments` vào danh
   sách bảng của đường Nếp có lý do.
5. **Hai trường BM25 + trọng số**: hợp nhất bằng RRF có trọng số trong Go (Milvus RRF không nhận trọng số),
   các nhánh tìm song song; trọng số ở `cauhinh.json` «hop», truy hồi dùng đúng trọng số đó.
6. **Truy hồi theo ngữ cảnh**: làm giàu (offline) viết `ngu_canh_ho_so`/`ngu_canh_danh_gia` (≤160 ký tự,
   `TextSafe`), đứng đầu văn bản chunk.

## Kết quả theo phát hiện

Nạp: F1 sửa (chắc chắn dị ứng chỉ khi đã duyệt; mọi khẳng định chắc chắn vào hàng duyệt) · F2 sửa (1/6 quán nền
dị ứng không rõ, giá/giờ không rõ đã có; oracle vàng tính vi phạm «chưa rõ dưới ràng buộc cứng»; 74 hàng chưa
rõ trong test đối chiếu Milvus↔Go) · F3 sửa (một schema, một writer, một client, một tier script) · F4 sửa (tiền
tố chỉ trong nhung, test wire trên Gemini thật qua loopback) · F5 sửa (unit + vòng đời) · F6 sửa (phán quyết theo
`ban`) · F7 sửa (upsert từng phần sang collection khác cấu hình, hỏng thì xoá) · F8 sửa (promote kiểm vân tay +
sha vàng) · F9 quyết một luật · F10, F11, F12 để lại (nhỏ, không đổi) · F13 hết (cửa nhúng là GE2).

Truy hồi: F1 = F3 nạp · F2 ghi vào §8.4 · F3 job CI `milvus` riêng · F4 `DiemXepLai` · F5 lọc token cấu trúc
NFKC + khoảng trắng + token thêm của Qwen3 · F6 cắt truy vấn 512 rune · F7 `nhung.Dem` kiểm số vector · F8 test
`MaxTaiLieu` · F9 `MOBILE_RERANK_TIMEOUT`, mặc định 3 s ghi rõ · F10 để lại (đếm `BiLoai` vẫn 4 truy vấn count).

Tập vàng: `ha-cao-lau-gieng-co` nhãn tay nay có `lua_mi` (cao lầu là sợi lúa mì; `phai_loai` của d08 đã nói
vậy) — tập vàng hết tự mâu thuẫn; số ghim của `rag` không đổi.

## Số đo (cây `2bfd0c8` + nhánh)

- gofmt 0 file; go vet sạch với tag mặc định, postgres, broker, eval, milvus, và cả bốn cùng lúc.
- `go test ./...` xanh (mọi gói).
- `-tags postgres` trên các gói chạm (rag/..., thuoctinh, aidoc, jobs, cmd/core, aigate, nhungcache, hybrid,
  vectordb/..., aiharness/...): 443 PASS, 0 SKIP.
- `scripts/go_milvus_tier.sh` (Milvus v3.0.2 tại chỗ, llama-server Qwen3-Reranker, Postgres ở Alembic head):
  exit 0, 60 ca PASS trên 4 gói, 16 sentinel, 0 SKIP. vectordb: 120 tập ràng buộc, 5008 hit, 0 vi phạm;
  hybrid 16 câu: lai 1.000, chỉ dense 0.500, chỉ BM25 0.750; hybrid đầu–cuối: 60 truy hồi, 514 mục kiểm trên
  hàng sống, 0 vi phạm, kiểm lại bỏ 397 hit cũ; rerank golden lệch tối đa 1.35e-2 (ngưỡng 0.02); nạp: 228
  thăm dò, 0 vi phạm, 259 bộ lọc Milvus = Go, 74 hàng có thuộc tính chưa rõ; vàng trên Milvus recall@10 0.9333,
  nDCG@10 0.8439, violation@10 0 (trùng tầng unit); lát bỏ dấu 0.9133 với nhánh gấp dấu, 0.7733 khi tắt.
- `scripts/gate.sh eval-kich-ban`: ĐẠT (56 ca, 75 lượt, canary đỏ đúng chỗ, đồng nhất xanh).
- aigate xanh; `check_route_ownership.py` selftest + chạy thật OK (158 hàng); repo guard tree/staged exit 0.
- Đột biến tự nghĩ: 30 chạy, 30 giết, mỗi cái đỏ đúng test dự đoán (F1 ×3, F2 ×4 gồm 2 trên Milvus thật,
  F4 ×2, F5 ×2, F6 ×2, F7 ×2 gồm 1 trên Milvus thật, F8 ×2, luật chưa rõ ×2, thấp của truy hồi ×7, SOTA ×4).
- Python `tests/`: 857 PASS; 5 hỏng + 1 bỏ chọn vì môi trường (không docker, `column`, `alembic` trên python hệ
  thống), không liên quan nhánh.

## Còn mở

- Mọi số vàng và trọng số đo trên encoder stub; đo lại khi có khoá và Lead duyệt số lời gọi.
- Adapter MILCO còn trong `vectordb` (tắt, không nối): gỡ hẳn hay bật lại là việc sau khi license rõ.
- Reranker chưa gắn vào engine (`aiharness.WithXepLai`); hybrid trong engine báo `no_rerank`.
- Hai bảng cache nhúng (`rag_embedding_cache`, `nhung_cache`) chưa gộp.
- Luật `aggregate-base64-fragments` trên go.sum (từ retrieval-core) vẫn cần Lead xác nhận.
- Job CI `milvus` cần runner tự host có nhãn `milvus` + `reranker`.
