# «Ký hoạ trong sổ» — nơi chưa có ảnh nhận một nét ký hoạ (review Codex 11/09 · A1)

Codex A1 (P2, «nút thắt stunning»): Khám phá vẫn là danh mục có style — nơi dẫn không ảnh chỉ có ô giấy + glyph bát,
không gợi ánh sáng, món, chỗ ngồi. Không được bịa ảnh; repo không có nguồn ảnh địa điểm hợp pháp (chỉ ba ô texture
sinh bằng Pillow). Lead chọn hướng **ký hoạ trong sổ**: như bạn vẽ vội vào sổ chuyến đi cạnh tên quán.

## Cơ chế (`apps/mobile/src/rudi/art/ky-hoa.ts`, `ui/art/KyHoa.tsx`)

- **Sân khấu theo loại nơi**: `quan-an-local` hiên quán (mái vạt lượn, bàn dài, bát, bếp than khói) · `cafe` góc cửa
  kính (khung chữ thập, rèm, đồi có rừng xa, bàn tròn cốc bốc hơi, ghế) · `vui-choi` đồi nhìn từ lan can (ba
  dãy đồi chạy hết mép, mặt trời sát đồi) · `di-choi-dem` quầy đêm dưới dây đèn (mái vạt lượn trên cột, bát trên quầy,
  trăng) · loại lạ: tờ ghi gấp trên sàn.
- **Đạo cụ theo tag có thật**, tối đa hai, so cả từ bỏ dấu (dùng lại `gapChu` của `ly-do.ts`): View đẹp → đồi phía
  sau · Nhóm đông/Lẩu → bốn bát · Chill/Nhẹ nhàng → cây treo, rèm · Món local → nồi · Ngoài trời/Săn mây → mây · Đi
  đêm/Nhộn nhịp → thêm đèn · BBQ → khói · Hoa/Chụp ảnh → hoa · Cà phê/Trà → cốc.
- **Đúng một coral = nguồn sáng** (bóng đèn dưới mái · đèn thả · mặt trời · bóng giữa dây), như luật một coral của gu.
- **Ba cỡ nét theo độ sâu** (gần 3.0 · vừa 2.4 · xa 1.7); mảng tối là `bong`; mảng tô đi đúng đường cong của nét viền
  (cùng cubic đóng xuống sàn).
- **Một hình, hai khung cắt**: 3:1 ở chữ 1.0, 4:1 ở chữ lớn (`chuLon`), `preserveAspectRatio slice`; bản `gon` ít lớp
  hơn (bỏ ghế, thông, đạo cụ nhỏ). Tờ `paper` viền `line` bo `radius.small`, role image, câu a11y «Ký hoạ …».
- **Thật thà**: vẽ *một loại nơi*, không tên, không bảng hiệu; live chỉ có loại → hai quán ăn cùng một hiên.
- Lắp ở lead Khám phá không ảnh (bố cục cột như lead có ảnh) và đầu bài chi tiết không ảnh (fixture + live). Hàng và
  cặp so sánh giữ ô giấy loại nơi.

## Cách vẽ: ứng viên → đọc mù → sửa

Ứng viên render bằng primitives đã biên dịch (`scratchpad/ky-hoa/ve.mjs`). **Agent context mới, chưa đọc brief**, đọc
bốn dải không nhãn (sáng, tối, bản 5:1) và đánh trượt bản v2: bát treo dưới mép bàn («bát úp»), vạch mái cách đều =
thước kẻ, cửa kính = **laptop mở biểu đồ**, thông = mũi tên, mây hai elip = Venn, móc dây đèn = icon refresh, cốc lơ
lửng to bằng ghế; «bộ icon line-art, không phải ký hoạ». v3 sửa từng thứ (mục trên); bản 4:1 giữ sàn và mái nhờ dời
sàn về 82 và mái về 18 trong khung 96.

**Đọc mù lần hai, trên dải cắt từ ảnh native v3** (agent context mới): hiên quán và góc cửa kính đọc đúng loại nơi
(«quán ăn bình dân có mái che… ngồi ghế đẩu dưới hiên», «quán cà phê có cửa sổ lớn»), đồi và chợ đêm đọc đúng; tờ ghi
loại lạ đọc là icon tài liệu (chấp nhận: đó là fallback). Còn sai: nồi vẽ vuông = «hộp bưu kiện bốc khói»; thông đứng
trên thanh ngang khung cửa; hai đường đồi bên phải hiên «không gọi tên được»; dải tô giữa đồi = ruy-băng; bóng đèn rỗng
= khoen/nhẫn; bản 4:1 của cafe mất khói, cây, ghế → «phòng trống». **v4**: nồi tròn có nắp và hai tai, đồi xa có thân
tô (chỉ dưới phần nhìn), bỏ thanh ngang cửa và hạ rừng xuống đứng trên đồi, bỏ dải tô giữa đồi và hạ lan can xuống
nền gần, bóng đèn thành giọt tô, trăng lưỡi liềm, quầy hai có lồng đèn, bản gọn giữ khói + ghế + cả hai quầy. Reader
cũng nói «không đọc là ký hoạ tay mà là line-art phẳng» — đó là ngôn ngữ chung của Nếp (nét đều, hình chuẩn), không
đổi riêng ở đây.

**Finish reviewer (context mới), đọc mù các dải cắt từ ảnh native v4 rồi mới nhận packet:** hiên quán đọc là «quán ăn
ngoài trời dưới mái hiên, nồi tròn có nắp bốc khói, ngồi ghế đẩu», đồi là «điểm ngắm cảnh có lan can», chợ đêm đúng,
tờ ghi là icon tài liệu. Phán quyết `fix` năm điểm: (1) cây treo ở hiên quán không gọi tên được («móc treo bốc khói»);
(2) câu a11y kể đạo cụ mà bản gọn không vẽ và gọi «rèm» ở hiên quán; (3) lồng đèn đọc là trứng/bóng bay; (4) tờ ghi bản
gọn mất hai dòng thành «cái hộp»; (5) thiếu test buộc câu a11y khớp lớp đã vẽ. **v5**: chậu treo có thân tô + ba lá
tô; bảng đạo cụ theo sân khấu ghi rõ đạo cụ nào sống ở bản gọn, `moTaKyHoa(loai, tags, {gon})` chỉ kể cái đã vẽ và
gọi tên theo sân khấu («rèm kéo» chỉ ở cửa kính, «cây treo» ở hiên); lồng đèn có nắp – thân tô – tua; tờ ghi gọn giữ
hai dòng; test mới: bỏ một tag làm câu đổi ⇔ làm hình đổi, ở cả hai khung đọc (test này đã bắt được đúng ca cafe bản
gọn kể «mây» mà không vẽ trước khi sửa). Và **bản gọn là tập con của bản đủ**: hai đạo cụ chọn cho bản đủ, bản gọn chỉ bỏ
cái không vẽ, không bù bằng tag sau (trước đó lead 2.0 vẽ bốn bát trong khi 1.0 vẽ hai — bắt được trên câu a11y trong XML).

## Bằng chứng native (`anh/`, Android 15, 1080×2400, fixture)

| ảnh | cấu hình | thấy gì / đo gì |
|---|---|---|
| `r14-80-kham-pha-fs{1.0,2.0}-{sang,toi}.png` + `.xml` + `.kiem.txt` | Khám phá, 4 cấu hình | lead «Tiệm Nướng Xóm Lèo» mở bằng tờ ký hoạ hiên quán (mái vạt lượn, bàn dài, bát, khói than, bóng đèn coral, đồi + thông sau cột phải; cây treo vì «Chill»); 3:1 ở 1.0, 4:1 ở 2.0 vẫn giữ mái và sàn. **Cổng XML r14 xanh 4/4** (một «đúng gu», lý do «View đẹp», mô tả không lặp). |
| chiều cao node lead (bounds, dp) | 1.0 · 2.0 | **1.0: 162 → 245 (+83)** · **2.0: 309 → 370 (+61)** — mục tiêu ≤ +80 ở 2.0 đạt; ở 1.0 chấp nhận vì thay chỗ của ô glyph 58 và hàng facts nay một dòng |
| `r14-81-chi-tiet-fs*-*.png` | chi tiết không ảnh, 4 cấu hình | đầu bài là cùng tờ ký hoạ, rồi tên h1, mô tả, facts, chip tag, «Vì sao hợp»; không còn ô glyph 36 lẻ loi |
| `r17-97-ky-hoa-lab-{tren,duoi}-fs*-*.png` | bảng ui-lab, 4 cấu hình | năm sân khấu × hai khung cắt (3:1 · 4:1) cạnh nhau; câu a11y in trên mỗi cặp — **chỉ kể đạo cụ sân khấu thật sự vẽ** (lỗi «quầy đêm… nồi giữa bàn» bắt được trên ảnh lượt đầu, sửa `daoCuTheoTag(tags, sanKhau)` rồi chụp lại) |

Sáng/tối: tờ dùng `paper`/`paperShade` của PR C nên ở tối là tờ giấy đêm trên vải, nét mực sáng, coral y nguyên.

## Chưa chứng minh
Người ngoài đọc ký hoạ ra «quán nướng ngoài hiên» hay chỉ ra «có bàn có đèn» — reviewer AI đọc mù không thay được;
team thử với người chưa đọc brief. `anh/r17-97-ky-hoa-lab-*` **có caption đáp án**,
không dùng làm bảng không nhãn. Bộ chỉ có mã hình để phát cho người đọc nằm ở
`docs/codex/2026-09-12/khep-audit-luot-4/bo-doc-hinh/nguoi-doc/`; hướng dẫn và
đáp án điều phối nằm ngoài thư mục ấy. Live: chưa có stack, chỉ chứng minh
bằng test rằng `tags: []` cho sân khấu trần. Máy thật, iOS, TalkBack (câu a11y có nhưng chưa nghe).
