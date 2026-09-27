# CRAG, grounding và verifier cho lượt truy hồi (find_places, sổ tay app)

Ngày 2026-09-25 · nhánh `agent-core/crag`, dựng trên hợp đồng `2ebeb0b` (nhánh `agent-core/contract`) ·
protocol_version: không áp dụng (không chạm `docs/protocol/v1/`) · verdict: chưa có reviewer · luật áp
dụng: «Luật không heuristic» (`docs/architecture/03-ai-engine-hop-dong.md` §8).

## 1. Đã dựng gì

| Gói | Nội dung |
|---|---|
| `aiharness/cautruc` (mới) | Một lời gọi có cấu trúc: system instruction tĩnh, một lượt user gồm các khối `<du_lieu>`, `ResponseSchema`, `ThinkingLevel` MINIMAL đặt tường minh, safety tường minh; `Them` cho lượt sinh lại; `KhoiBangChung` đặt bằng chứng dưới bí danh, mỗi mục một dòng, giá trị datamark và cắt 300 rune. |
| `aiharness/crag` | `ChamLLM` (người chấm, một lời gọi flash-lite, schema `LuocDo`, đọc bằng `Doc`); `TruyHoi`: truy hồi → rerank → ghi sổ cái → chấm → tối đa MỘT vòng sửa do model chọn. Go kiểm: thứ tự nới (khí chất → loại chỗ → khu vực, `KiemThuTu`), `SuaYeuCau` không chạm ràng buộc cứng, bất biến `Cung` không đổi (`ErrCungDoi`), reranker không được thêm mục, trần reranker theo LƯỢT (`NganSachXepLai`), hạn 3 s mỗi lần truy hồi, chỉ chấm khi còn ≥3 lời gọi. |
| `aiharness/kiemchung` | `VerifierLLM`: một lời gọi cho cả câu trả lời, ngữ cảnh mới, câu trả lời trình bày như của «một trợ lý khác», bằng chứng dưới bí danh cục bộ `e1…` (enum đóng), đầu ra chỉ chỉ số câu + enum; bí danh đổi lại id thật sau `Doc`. |
| `aiharness/traloi` (mới) | Bước trả lời: schema `{hanh_dong, rang_buoc_khong_dat, cau[{chu, bang_chung, trich}]}`; token `[[p:bí danh]]` → tên từ sổ cái (lạ → «một chỗ», đếm); `«…»` là nhãn nút; `kiemBan` (id ⊆ sổ cái, giá trị khai = trường bằng chứng và phải được viết trong câu, cạnh thẻ của chỗ đó, nhãn nút ∈ nhãn sổ tay của lượt, định dạng riêng tư); verifier; sinh lại MỘT lần với phát hiện là dữ liệu; dự phòng cố định; kiểm định dạng riêng tư cuối cùng trên toàn văn bản phát ra; nguồn gốc `{nguon, so_bang_chung, cong_cu}` trên mọi phần. |
| `aiharness/cau/rangbuoc.go` (mới) | Câu cố định: `TuChoi`, `HoiLai`, `DaNoiLong`, `DuPhong`, `DuPhongHuongDan`, `MotCho`, cụm từ cho từng tên ràng buộc. Không đụng bảng `bang` mà test mobile đọc. |

Thêm hai tệp giống từng byte bản trên nhánh anh em để gộp không xung đột: `guard/dinhdang.go`
(`guard.DinhDang`, của `agent-core/tools`) và phần `prompts.DanhDau`/`BocDuLieuDanhDau` + hằng khối
(của `agent-core/router`).

## 2. Ai quyết định gì

- **Model**: bằng chứng đủ/thiếu/mâu thuẫn; ràng buộc nào chưa thoả (enum); bước sửa (nới mềm hay viết
  lại truy vấn); trả lời / hỏi lại / từ chối; văn xuôi; mỗi câu có được bằng chứng hỗ trợ không; có hứa
  hành động không công cụ nào làm không; có đụng tiền không. Phán đoán «tôi đã chuyển tiền» nằm ở verifier,
  không còn ở luật cụm từ.
- **Go**: kiểm cấu trúc (schema, enum đóng, bí danh thuộc tập), ngân sách (≥3 lời gọi mới chấm, ≥2 mới trả
  lời hay sinh lại, trần reranker theo lượt, trần tool của sổ cái, hạn giờ), thứ tự nới, ràng buộc cứng
  không bao giờ đổi, thuộc tập và bằng nhau tuyệt đối, đọc hai dấu phân cách của định dạng do ta đặt
  (`[[p:…]]`, `«…»`, bằng `strings.Index`), kiểm định dạng dữ liệu riêng tư (SĐT, số tài khoản, số thẻ,
  email — kiểm định dạng, không phải hiểu ngôn ngữ). `TestKhongHeuristic` giữ điều này bằng cấu trúc: bốn
  gói của đường này không import `regexp`, `tuvung`, `rag`, `preprocess`, `promptsafety`, và từ `guard`
  chỉ dùng `DinhDang`/`RaSach`.
- **Câu cố định, 0 lời gọi**: hỏi lại và từ chối (thiết kế 04 §5.6), điền bằng cụm từ của enum model chọn;
  không chữ model nào vào câu đó. Dị ứng và ăn kiêng không bao giờ được đưa thành chip «đổi điều kiện».

## 3. Draft-then-verify và chi phí độ trễ

Không có sự kiện rút lại và `lam_lai` chỉ hợp lệ trước delta đầu (bất biến 8 của `aieval`). Verifier giờ
mang phán đoán tiền và hứa hành động, nên không thể bỏ qua hay chạy sau khi đã phát. Vì vậy mọi câu
trả lời có khẳng định sự thật đi theo **sinh trọn → kiểm tất định → verifier → mới phát** (phương án C+D
của `reflection-verification` §4). Không byte nào của bản nháp rời bước này trước khi qua hết; phần
`places` cũng chỉ phát sau khi câu trả lời qua.

Chi phí (số ước lượng của báo cáo nghiên cứu, **chưa đo** — phải đo ở T3/M0 bằng cassette):

| Đoạn trước byte đầu | Ước lượng p50 |
|---|---|
| router | ~0,7 s |
| truy hồi + rerank | ~0,4 s |
| người chấm (flash-lite, MINIMAL, ≤256 token ra) | ~0,5–0,7 s |
| bản nháp trọn (150–350 token) | ~0,8–1,7 s |
| verifier (≤512 token ra) | ~0,6–1,5 s |
| **Tổng tới byte đầu** | **~3,0–5,0 s** |

So với stream thẳng (phương án A, ~1,6 s) là **thêm ~1,5–3 s**, và nhiều khả năng **vượt mục tiêu p50
token đầu ≤2,5 s** của hợp đồng. Trạng thái `dang_nghi` vẫn phát ngay nên màn hình không đứng. Cần Lead
chọn: (a) chấp nhận độ trễ này cho câu trả lời có khẳng định; (b) phương án B của nghiên cứu (JSON stream,
`chon` trước, kiểm trước delta đầu) — nhưng khi đó verifier chỉ còn chạy được trên đường không stream,
và phán đoán tiền/hứa hành động phải có chỗ khác; (c) chạy verifier song song với việc sinh phần thẻ.
Sinh lại cộng thêm một bản nháp + một verifier (~1,4–3,2 s) và chỉ nên xảy ra ở ≤5% lượt (mục tiêu của
nghiên cứu, chưa đo).

## 4. Bảng số lời gọi theo đường (tính cả router, trần 8)

Đo bằng `TestDuong` với stub có kịch bản, tại commit cuối của nhánh:

| Đường | Kết thúc | Lời gọi |
|---|---|---|
| đủ | tra_loi | 4 |
| sửa: nới mềm | tra_loi | 4 |
| sửa: viết lại truy vấn | tra_loi | 4 |
| người chấm đòi nới dị ứng (bị từ chối, không có vòng) | tra_loi | 4 |
| sổ tay | tra_loi | 4 |
| verifier hỏng (fail closed) | du_phong | 4 |
| hỏi lại | hoi_lai | 3 |
| từ chối | tu_choi | 3 |
| từ chối, không tìm thấy gì | tu_choi | 2 |
| vi phạm grounding → sinh lại | tra_loi | 5 |
| cấu trúc hỏng → sinh lại | tra_loi | 5 |
| nhãn nút không có trong sổ tay → sinh lại | tra_loi | 5 |
| số điện thoại → sinh lại | tra_loi | 5 |
| token lạ hai lần → phát với «một chỗ» | tra_loi | 5 |
| verifier bắt giá không có → sinh lại | tra_loi | 6 |
| verifier bắt «đã chuyển tiền» hai lần → dự phòng | du_phong | 6 |

Mọi đường không sinh lại ≤4. p95 ≤4 đúng khi sinh lại ≤5% lượt — **tỉ lệ đó chưa đo**. `TestKhongVuotNganSach`
chạy đường tệ nhất với 0…8 lời gọi đã tiêu trước: lượt không bao giờ quá 8 và không bao giờ lỗi vì ngân sách
(bỏ chấm, bỏ sinh lại hoặc bỏ trả lời rồi rơi về dự phòng).

## 5. Nghiên cứu: nhận và không nhận

Đã đọc `reflection-verification` (không có mục Kiểm chứng), `agentic-rag-tools` và `intent-routing` (cả
mục Kiểm chứng), `stm-personalization` (không có gì áp dụng cho bước này: bước trả lời không đọc trí nhớ).

Nhận: verifier ngoài thay tự sửa nội tại; ngữ cảnh mới, trình bày như câu của trợ lý khác; một lời gọi
cho cả câu trả lời; đầu ra chỉ chỉ số + enum, không chữ tự do vào prompt nào; tối đa một vòng sửa, có chẩn
đoán cụ thể kiểu CRITIC (phát hiện của Go/verifier là dữ liệu); không stream trước rồi kiểm sau; bí danh
thay id thật, enum đóng; model không tự viết tên chỗ (token), giá/giờ khai kèm và phải khớp; dự phòng
tất định bằng thẻ từ sổ cái; giữ lời gọi dự trữ trước lời gọi tuỳ chọn; MINIMAL đặt tường minh (mục Kiểm
chứng của `agentic-rag-tools`: MINIMAL được cho lời gọi một bước; LOW là biến cần đo); trần reranker riêng
theo lượt; thứ tự nới khí chất → loại chỗ → khu vực; model không bao giờ nới ràng buộc cứng; đếm số bị loại
theo ràng buộc cứng để trả lời trung thực.

Không nhận (trái luật chủ sản phẩm hoặc hợp đồng): CRAG tất định theo ngưỡng điểm reranker; K6 bắt số bằng
regex quanh token (thay bằng giá trị khai trong `trich` + verifier); K7 nhãn nút sau «bấm/nhấn» (thay bằng
định dạng `«…»` + thuộc tập); K9 mẫu câu an toàn dị ứng (thay bằng instruction + verifier); nhãn verifier
năm bậc; «parse hỏng thì coi như không có giám khảo» (hợp đồng: verifier hỏng là không phát); không chạy
verifier trên đường stream của Nếp (nhiệm vụ yêu cầu verifier thay luật cụm từ tiền, nên nó chạy trên mọi
câu trả lời có khẳng định; chi phí ở mục 3).

## 6. Còn mở

- Chưa nối vào engine: `traloi.Chay` chưa được `aiharness.Engine`/tool `search_places` gọi; adapter
  `truyhoi.Retriever` thật (lexical trên `rag`, Milvus) và reranker Qwen đến từ nhánh tools/hạ tầng.
- Chưa stream: phát sau khi kiểm xong; nối `Phan` vào `Sink.Phan` là việc tích hợp.
- Độ trễ và tỉ lệ sinh lại chưa đo; precision/recall của verifier chưa có nhãn người (cổng đề xuất ở
  `reflection-verification` §5.7).
- Mỗi mục sổ tay mang một nhãn nút (`nhan_nut`); mục có nhiều nút cần hợp đồng cho phép nhiều nhãn.
- Output guard cũ (`guard.DauRa.Kiem` với luật cụm từ «tôi đã chuyển tiền») vẫn nằm trên đường cũ; gỡ nó
  khỏi đường quyết định là việc của bước tích hợp khi đường này thay đường cũ.
