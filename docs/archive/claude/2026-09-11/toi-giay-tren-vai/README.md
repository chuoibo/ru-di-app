# Bản tối là «sổ đóng trên bàn» — giấy đêm trên vải bìa (review Codex 11/09 · A3)

Codex A3 (P2 cổng mỹ thuật): bản tối «giữ màu thương hiệu nhưng chưa giữ cảm giác giấy» — nền indigo phẳng, thân giấy
Nếp gần nền nên cảnh thành sơ đồ nét. Đo trên ảnh của họ và trên token: vân giấy đêm `giayTrang` 0.30 ≈ 2 mức (README
texture: «ở ngưỡng, không hạ thêm»); thân giấy = `card #1f2340` trên nền `#151830` (L* 14.8 vs 9.2); bóng gấp =
`line #363b5e` (L* 25.9) **sáng hơn** mặt giấy — nếp gấp lộn trong ra ngoài.

## Câu chuyện và cơ chế

Ngày: trang giấy mở, vân giấy 0.45. **Đêm: cuốn sổ đóng lại trên bàn** — nền là vải bìa (`Grain vaiBia` 0.30, ô vân đã
đo ≈ 8 mức trên `cover`), và mọi *hình vẽ* là **tờ giấy đêm** đặt lên vải: `paper #2e335c` (L* 22.7, +13.5 bậc so với
nền — cùng quan hệ mặt/nền như giấy sáng), bóng gấp `paperShade #181b36` (L* 10.9, −11.8 bậc dưới mặt — như `line`
dưới `card` ban ngày). Chữ và thẻ **không đổi token**; `paper`/`paperShade` ở scheme sáng trùng `card`/`line` nên sáng
không đổi một pixel.

| chỗ | trước | sau |
|---|---|---|
| `ui/art/VeLop.tsx` `mauLop` | `giay → card`, `bong → line` | `giay → paper`, `bong → paperShade` |
| `ui/stickers/Sticker.tsx` | `card → colors.card`, `line → colors.line` | `→ paper`, `→ paperShade` |
| `HangDiaDiem.tsx` `PlaceGlyph` | nền ô `card` | nền ô `paper` |
| `ui.tsx` `RudiScreen` | `Grain giayTrang` 0.30 ở tối | `Grain vaiBia` 0.30 ở tối; sáng giữ `giayTrang` 0.45 |
| `tokens.json` (+ `guest.css`, DESIGN frontmatter/bảng, `design.json` sinh lại) | — | `paper`, `paperShade` hai scheme |

## Bằng chứng: cặp trước/sau cùng màn, cùng máy (Android 15, 1080×2400)

| màn · cấu hình | đo | trước | sau |
|---|---|---|---|
| Khám phá · tối 1.0 | stddev nền không chữ (200×200 gần token nền) | 2.11 | **8.21** |
| Khám phá · tối 1.0 | pixel tông giấy đêm trong ô glyph 150×150 | 1.1% | **88.6%** |
| Khám phá · tối 2.0 | stddev nền (120×120) | 0.79 | **8.05** |
| Khám phá · tối 2.0 | pixel tông giấy đêm trong ô glyph | 1.1% | **88.8%** |
| Khay sticker (ui-lab) · tối 1.0 | stddev nền (80×80) | 0.00 | **8.59** |
| Khay sticker · tối 1.0 | pixel tông giấy đêm trong ô «Ăn gì?» (hộp cố định 200×170) | 2.3% | **5.0%** (thân tờ giấy) |
| Màn lỗi có cảnh · tối 1.0 | stddev nền | 0.85 | **8.08** |
| Màn lỗi · tối 1.0 · 2.0 | pixel tông giấy đêm trong node `canh-chua-doc-duoc` | 3.0% | **29.7%** |
| Khám phá · sáng 1.0 · 2.0 | pixel khác trước/sau dưới status bar | — | **0** |
| Màn lỗi · sáng 1.0 · 2.0 | pixel khác trước/sau | — | **0** |
| Khay · sáng 1.0 | pixel khác | — | 8.4% — **không so được**: flow cuộn ui-lab tới vị trí khác ở hai lượt; sáng không đổi theo token (giống hệt `card`/`line`) và hai màn kia bằng 0 |

Ảnh: `truoc/` và `sau/` cùng tên (`r14-80-kham-pha-*`, `r13-71-khay-tam-o-*`, `r13-77-loi-canh-moi-*`) × `fs1.0|fs2.0` ×
`sang|toi`; khay ở 2.0 không có ảnh vì flow ui-lab cuộn tới «Mở khay sticker» đỏ ở chữ lớn (đỏ cả trước và sau, không
liên quan thay đổi này). Vân vải đo ≈ 8 mức trên nền tối đúng như đã đo trên `cover` trong DESIGN.md.

Đo bằng `do-chat-lieu.py <png> <xml> [--dem-mau HEX --hop x0,y0,x1,y1]`: vùng nền không chữ ≥ 200×200 px chọn tự
động từ bounds uiautomator; stddev mức xám = «có vân hay không»; đếm pixel mang tông giấy đêm trong ô sticker/cảnh =
«tờ giấy có xuất hiện». Sáng: cùng lệnh, kỳ vọng số trước = sau.

## Finish reviewer (context mới) — đọc mù rồi mới nhận packet
Ba cặp tối A/B không nhãn: reviewer gọi bản «vật liệu, tờ giấy có thân» ở cả ba cặp — đối chiếu sau đều là bản *sau*.
Phán quyết `fix` với hai việc tài liệu, không có việc mã: (1) vân vải 0.30 **nâng nền tối thực** từ `#151830` lên
`#1c1f36` (+7 mức, đo cả ba màn) nên câu «ô vân trung tính không đổi màu token» sai trên nền tối nhất — đã ghi vào
`Grain.tsx` và DESIGN.md kèm bảng số tính lại trên nền thực (`inkFaint` 5.87, `lineStrong` 4.34, `card` 1.06:1 → thẻ
tối phải giữ viền `line`); (2) hai giới hạn dưới đây.

## Chưa chứng minh
- Nền tối thực lệch +7 mức so token vì vân vải (số ở DESIGN.md «Elevation & Depth»); chữ vẫn qua sàn nhưng bảng token
  không còn là số đo tuyệt đối trên nền trang tối.
- Ô khay sticker (`KhaySticker.tsx` nền `colors.ground` phẳng trong tấm `card`) là chỗ duy nhất giấy đêm nằm trên màu
  nền **không vân** — chấp nhận dưới scrim, nhưng câu «giấy trên vải» không đúng nguyên văn ở đó; không sửa trong PR này.
Cảm giác vật liệu trên máy thật (mật độ điểm ảnh và độ sáng màn khác emulator); tương phản chữ trên `paper` không đo
vì không có chữ trên tờ giấy vẽ; toàn bộ các màn tối khác chỉ đổi vân nền — chưa chụp hết; bản tối của bìa (`cover`)
không đổi.
