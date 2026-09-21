# native-r14 — R3 Khám phá nói lời hứa một lần (tái audit 10/09)

Codex (R3, P2): «Gần bạn, đúng gu» → «HỢP GU» → «Hợp gu nhờ Chill và View đẹp» → «Nướng thơm lừng, view đồi cực chill»
— một lời hứa đọc bốn lần; ba thẻ đầu cùng một cái bát; ở 2.0 khối dẫn chiếm gần trọn màn. DESIGN.md đã có «Luật Nói
Một Lần» (`:1608`) và «Don't lặp một sự thật hai chỗ» (`:1747`) — code vi phạm chính luật của mình.

## Sửa (cơ chế, không phải sửa chữ)

- **Một dấu cho một địa điểm** (`explore/HangDiaDiem.tsx`): có lý do thì in dòng lý do (✦ + một dòng `label` tím) và
  **không** in con dấu; không có lý do thì con dấu như cũ. Lead (cả nhánh có ảnh), cặp so sánh, hàng. Tiêu đề mục là
  nơi duy nhất nói «gu».
- **Lý do là một tag chưa nói** (`kham-pha/ly-do.ts`, thuần, test): `chonLyDo(tags, sub)` — tag đầu mà không từ nào
  (bỏ dấu) có trong mô tả. Trên dữ liệu cũ của Codex nó ra «Nhóm đông» (Chill/View đẹp đều nằm trong mô tả); fixture
  viết lại 12 mô tả để mô tả nói điều tag chưa nói. Live giữ câu `reason` của mô hình nguyên văn.
- **Dấu loại nơi bằng giấy**: `PlaceGlyph` từ đĩa `accentSoft` + icon toàn coral → ô giấy (`card`, hairline `line`,
  `radius.small`) với hình gu vẽ bằng mực và đúng một chi tiết coral của nó — ngôn ngữ tờ giấy của Album, không ảnh
  stock, không «icon trong vòng tròn». `guTheoTag` cho tag có thật chọn hình trước danh mục: Bánh căn Lệ («Món local»)
  ra hình món địa phương, hai quán còn lại vẫn bát — thật thà.
- Chi tiết live không ảnh: bỏ khung 16:10 rỗng; đầu bài gọn (ô giấy + con dấu + tên). Chi tiết fixture: AiNote nói về
  nhóm («Nhóm 8 người ngồi được một bàn, món nướng chia nhau dễ.»), không đọc lại chip.
- Câu lỗi mồ côi (ảnh 20): `EmptyState` body qua `khongMoCoi()` (`ui/chu.ts`) — hai chữ cuối nối NBSP.

## Cổng: `kiem-lap-loi.mjs` trên XML uiautomator

0 node `/hợp gu/i` · đúng 1 node «Gần bạn, đúng gu» · sau tên dẫn là một lý do ≤ 4 chữ · mô tả không chứa từ của lý do.
Node chữ là glyph icon (Ionicons) bị bỏ qua để không lệch một node. **Đối chứng đỏ**: chạy trên chính `05-explore.xml`
của Codex → ĐỎ ba lý do (4 node «hợp gu»; lý do dài; mô tả lặp «chill, view»).

## Kết quả (`chup.sh 8096 … 80-kham-pha-mot-ly-do.yaml … kiem-lap-loi.mjs`)

| cấu hình | Maestro | cổng | lý do dẫn | mô tả dẫn |
|---|---|---|---|---|
| 1.0 sáng | rc=0 | XANH | «View đẹp» | «Nướng than ngoài hiên, thơm cả con dốc» |
| 1.0 tối | rc=0 | XANH | «View đẹp» | cùng |
| 2.0 sáng | rc=0 | XANH | «View đẹp» | cùng |
| 2.0 tối | rc=0 | XANH | «View đẹp» | cùng |

Ảnh/XML/kết quả: `anh/r14-80-kham-pha-*`. Chi tiết không ảnh: `anh/r14-81-chi-tiet-*` (rc=0 ×4; flow assert câu AiNote
mới). Màn lỗi: `anh/r13-77-loi-canh-moi-*` (rc=0 ×4; flow assert `thử.lại.` — XML có U+00A0 giữa «thử» và «lại.»).
Bàn thử ui-lab «Khám phá · renderer live» (trạng thái Không ảnh): `anh/r14-82-lab-*`.

Đặt cạnh ảnh của Codex: `docs/archive/codex/2026-09-10/reaudit-evidence/05-explore.png` (1.0 sáng), `43-explore-light2.png`
(2.0 sáng), `52-explore-dark2.png` (2.0 tối), `20-error.png`.

## Chưa chứng minh

- Người chưa đọc brief đọc ô giấy ra «chưa có ảnh, đây là loại quán» — cần con người.
- Live: câu `reason` của mô hình dài trên một dòng ở 2.0 sẽ bị «…» (`numberOfLines 1`) — chưa có stack live để chụp.
- iOS, tablet, TalkBack đọc dòng lý do (icon ✦ là trang trí).
