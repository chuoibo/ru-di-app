# Trả lời review 11/09 (Codex) trên `main 5061b610` — «đã tinh tế hơn thật, chưa stunning»

Trả lời cho `docs/archive/codex/2026-09-11/review-main-5061b610.md` (+ `assessment-a-my-thuat.md`, `assessment-b-ky-thuat.md`,
hai thư mục canary). Phán quyết của họ: **APPROVE** R1/R3/R5 và cấu trúc R4 trong phạm vi đã nhìn, F45 giữ bản mới,
**REQUEST_CHANGES riêng cổng motion R2** (B11 · B12). Bằng chứng của họ được commit **một phần**: ba bản `.md`, hai
`canary-summary.json`, `limitations.json`, `manifest.json`, và đúng những PNG được trích dẫn ở đây (pin sha256); video
`.mp4` không vào Git.

Thứ tự làm theo mức của họ: B11/B12 (P1, chốt độc lập không chờ TLS) → A4 nút bình chọn (P2 nhỏ) → A3 bản tối → A1
Khám phá có nơi chốn. A2 (hai cảnh còn phải nhờ brief) không vẽ tiếp: Codex đã chuyển sang thử người ngoài, vẽ thêm
một vòng nữa theo tên pose là đoán.

## PR A — B11 · B12: cổng motion fail-closed thật

Codex đúng cả hai, và bảng canary 13 nhánh của họ là bằng chứng sạch hơn mọi lời tôi tự khai: `reset-failed`,
`dump-malformed`, `pid-malformed`, `put-failed`, `restore-failed` **exit 0**; `original-empty` exit 1 nhưng để
`animator_duration_scale` ở 0. Nguyên nhân trong `do-motion.sh` v2: rc của reset và dump bị `>/dev/null 2>&1` nuốt;
`hop_le` chỉ xét ba điều; `tren150` in `0` khi vắng histogram; gốc không đọc được chỉ đặt `that_bai=1` rồi vẫn `put 0`;
`khoi_phuc` `|| true`. Tôi đã viết «fail-closed» cho một cổng chỉ đóng ba trong chín cửa.

### v3 (`docs/archive/claude/2026-09-10/motion/do-motion.sh`)

Thứ tự bất biến và mỗi bước xác minh trước bước sau: đọc đủ gốc (không phải số → **exit 3** trước mọi `put`/Maestro) →
`reduce` ghi 0 và **đọc lại** bằng 0 (sai → trả gốc, **exit 4**) → warm-up một lần → mỗi chuỗi: pid → reset **kiểm rc**
(hỏng → không chạy chuỗi) → Maestro → dump **kiểm rc** → pid → trap trả gốc, **đọc lại, so**; lệch → **exit 5**.
Chín điều kiện của một hàng hợp lệ (Maestro rc 0 · dump rc 0 · pid trước là số · pid sau bằng · **pid trong header dump
bằng** · frames > 0 · đủ janky/percentile/bộ đếm · có `HISTOGRAM` · **tổng bucket = frames**) liệt kê trong README của
cổng; hàng hợp lệ không bao giờ chứa `?`; lý do hỏng in ngay trong ô.

Điều kiện «pid trong header dump» và «tổng histogram = frames» là hai điều Codex B gợi («ràng buộc dump đúng tiến
trình», «tổng khớp frames ở cả tám») — kiểm trên **16 dump** đã lưu (v1 + v2, cả thường lẫn reduce): đúng cả 16.

### Bằng chứng

- `do-motion-canary.sh` 16 nhánh (bảng trong README): đỏ đúng mã 1/3/4/5/130 ở bước cuối, hai đối chứng xanh (reduce
  và thuong) exit 0 với bốn hàng số và gốc `0.5/1.5/2` trả đúng. **Cùng canary trên v2**: 11/16 nhánh SAI — cổng mới đỏ
  trên script cũ trước khi được tin.
- Bản sao `independent-canary.py` của Codex chạy trên v3 (OUT trỏ scratchpad): `original-empty → 3`, `put-failed → 4`,
  `restore-failed → 5`, `reset-failed`/`dump-malformed`/`pid-malformed → 1`, `interrupt → 130`. **Ca `normal` của họ
  đỏ**: pidof giả trả `4242` còn dump mẫu là của pid `4143` — đó là điều kiện mới cắn, không phải hồi quy; muốn xanh,
  fake phải trả pid của header (canary của tôi làm thế). Xin nói rõ để team không đọc thành «cổng chặn nhầm».
- Lượt thật trên máy ảo sau v3 (`dev-client-v3/`): xem mục ngay dưới — canary không chạm đường hạnh phúc, nên lượt thật
  là điều kiện để mở PR.

### Lượt thật sau v3 (`docs/archive/claude/2026-09-10/motion/dev-client-v3/`)

Cả hai chế độ exit 0, 8/8 hàng qua chín điều kiện, `scale-sau.txt` khớp cả ba khoá. Số khung lặp lại được so với
v2 (thường 268/924/406/274, reduce 45/708/62/59). Điểm mới không giấu: cuộn Khám phá có 3 khung ≥150 ms (thường) và
2 (reduce) ở lượt này, v2 là 0 — một lượt mỗi chuỗi chưa tách nhiễu máy ảo khỏi hồi quy; ghi số thật, không kết luận.

### Cố ý không làm
Đo release qua HTTPS local (chuỗi v3 chạy được ngay khi có stack, `FLOWS_DIR=.maestro-motion-live`); iOS/máy thật;
nhiều lượt để có khoảng tin cậy. R2 đóng ở **phương pháp và lời hứa fail-closed** trong phạm vi canary 16 nhánh + một
lượt thật trên dev client, không tuyên «§5 đạt toàn app».

## PR B — A4: nút bình chọn mang chữ

Codex A đúng: icon cột biểu đồ không nhãn là một cái đoán; nhãn a11y đúng không giúp mắt. Sửa ở `Group.tsx` (tờ AI
fixture; thẻ live không có nút): `RudiButton outline ai` «Bình chọn» cạnh `soft ai` «Xem lịch trình», a11y «Mở bình
chọn» giữ nên hai flow đang bấm nó (06, 65) chạy lại rc 0; ở chữ lớn hai nút xếp dọc hết cột.

Finish reviewer (context mới) trả `fix` một điểm tôi không thấy: ở 2.0 xếp dọc, **viền tím** của nút phụ (5.8:1) sắc
hơn **nền tô nhạt** của nút chính (1.17:1) nên mắt rơi vào «Bình chọn» trước — hai CTA ngang hàng, trái luật một quyết
định. Sửa: viền `lineStrong`, tông chỉ ở chữ; reviewer đo lại, chấm **resolved**, `ship` trong phạm vi này. DESIGN.md
có luật «Hành động phụ trong tờ mang chữ» và ngoại lệ viền ở mục Buttons. Bằng chứng:
`docs/archive/claude/2026-09-11/binh-chon-co-chu/`. Chưa chứng minh: người ngoài phân biệt «Bình chọn» với «xem thống kê»
(Codex yêu cầu người thật — để team); TalkBack.

## PR C — A3: bản tối là «sổ đóng trên bàn»

Codex A và Codex chính cùng nói một điều: bản tối giữ màu, mất chất liệu. Đo trên token và ảnh của họ thấy ba
nguyên nhân, không nguyên nhân nào là «thiếu noise»: (1) vân giấy đêm 0.30 đo 0.8–2.1 mức — dưới ngưỡng nhìn, nghĩa
là nền tối **chưa từng** có chất liệu; (2) thân giấy của mọi hình vẽ là `card #1f2340` trên nền `#151830`, 1.14:1;
(3) bóng gấp là `line #363b5e` **sáng hơn** mặt giấy — nếp gấp lộn trong ra ngoài, nên Nếp thành sơ đồ nét.

Câu chuyện chọn cùng bạn: ngày là trang giấy mở; **đêm là cuốn sổ đóng lại trên bàn** — nền là vải bìa (vân đã đo ≈ 8
mức, đang dùng trên Welcome), mọi hình vẽ là **tờ giấy đêm** đặt lên vải (`paper #2e335c`, cao hơn nền 13.5 bậc L* —
cùng quan hệ mặt/nền như giấy sáng; bóng gấp `paperShade #181b36`, thấp hơn mặt 11.8 bậc — như `line` dưới `card` ban
ngày). Thẻ và chữ **không đổi token**; `paper`/`paperShade` sáng trùng `card`/`line` nên sáng không đổi một pixel
(đo: 0 pixel khác dưới status bar ở Khám phá và màn lỗi, 1.0 và 2.0).

Số đo cặp trước/sau cùng màn (`docs/archive/claude/2026-09-11/toi-giay-tren-vai/`): stddev nền tối 0.79–2.11 → **8.05–8.59**
(Khám phá 1.0/2.0, khay, màn lỗi); tông giấy đêm trong ô glyph Khám phá 1.1% → **88.6%**, trong cảnh tờ rách 3.0% →
**29.7%**. Codex A gợi «có thể chọn và ghi rõ bản tối là bìa vải thay vì giấy, nhưng phải nhìn thấy vật liệu ấy ở cỡ
máy thật» — đúng hướng này; chưa nhìn trên máy thật.

Finish reviewer (context mới) **đọc mù ba cặp A/B trước packet**: gọi bản «vật liệu, tờ giấy có thân» ở cả ba
cặp, đối chiếu sau đều là bản *sau*. Phán quyết `fix` với hai việc tài liệu, không có việc mã, rồi `ship` sau khi
sửa: (1) vân vải 0.30 **nâng nền tối thực** từ `#151830` lên `#1c1f36` (+7 mức xám, đo cả ba màn) — câu «ô vân trung
tính không đổi màu token» của DESIGN.md sai trên nền tối nhất; bảng tương phản tính trên token cao hơn thực ~8%
(`inkFaint` 5.87, `lineStrong` 4.34 vẫn qua sàn; `card` trên nền thực **1.06:1** → thẻ tối phải giữ viền `line`).
Ghi vào `Grain.tsx`, DESIGN.md «Elevation & Depth» và README; **không** hạ opacity vải hay đổi `paper` để «bù». (2)
Ô khay sticker (`KhaySticker` nền `ground` phẳng trong tấm `card`) là chỗ duy nhất giấy đêm nằm trên màu không vân —
ghi là giới hạn, không sửa trong PR này.

Chưa chứng minh: cảm giác vật liệu trên màn OLED thật; các màn tối khác chỉ đổi vân nền, chưa chụp hết; khay ở 2.0
không có ảnh (flow ui-lab đỏ ở chữ lớn trước và sau, không liên quan).

## PR D — A1: Khám phá có nơi chốn — «Ký hoạ trong sổ»

Codex A1 gọi đúng nút thắt: sau R3 Khám phá sạch nhưng là **danh mục có style** — ô giấy + glyph bát không nói gì về
ánh sáng, món, chỗ ngồi. Họ đề đường ảnh thật có ghi công; repo không có nguồn ảnh địa điểm hợp pháp và tôi không bịa.
Hướng chọn cùng lead: **nơi chưa có ảnh nhận một nét ký hoạ**, như bạn vẽ vội vào sổ chuyến đi cạnh tên quán.

Cơ chế (`art/ky-hoa.ts` thuần, `ui/art/KyHoa.tsx`): sân khấu theo loại nơi (hiên quán · góc cửa kính · đồi thông
nhìn từ lan can · quầy đêm dưới dây đèn · tờ ghi cho loại lạ) + tối đa hai đạo cụ theo tag có thật (so cả từ bỏ dấu,
chỉ đạo cụ sân khấu ấy vẽ) + **đúng một điểm coral là nguồn sáng**; ba cỡ nét theo độ sâu; mảng tô đi đúng đường
cong của nét viền; **một hình, hai khung cắt** (3:1 chữ 1.0, 4:1 chữ lớn). Thật thà: vẽ *một loại nơi*, không tên,
không bảng hiệu; câu a11y mở bằng «Ký hoạ»; live chỉ có loại → hai quán ăn cùng một hiên. Lắp ở lead Khám phá và đầu
bài chi tiết không ảnh; hàng/so sánh giữ ô giấy.

Cách vẽ: ứng viên render bằng primitives thật; **agent context mới chưa đọc brief** đọc mù bản v2 và đánh trượt (bát
treo dưới mép bàn, vạch mái = thước, cửa kính = laptop có biểu đồ, thông = mũi tên, mây = Venn, móc dây = refresh,
«bộ icon không phải ký hoạ») → v3 sửa từng thứ. Đọc mù lần hai trên **dải cắt từ ảnh native**: hiên quán và góc cửa kính đọc đúng loại nơi
(«quán ăn bình dân có mái che, ngồi ghế đẩu», «quán cà phê có cửa sổ lớn»), đồi và chợ đêm đúng, tờ ghi loại lạ đọc
là icon tài liệu (chấp nhận, đó là fallback); còn sai: nồi vuông = «hộp bưu kiện», thông đứng trên thanh ngang cửa,
đồi bên hiên «không gọi tên được», dải tô giữa đồi = ruy-băng, bóng đèn rỗng = khoen, bản 4:1 cafe mất khói/cây/ghế →
**v4** sửa từng thứ (nồi tròn có nắp, đồi có thân tô, bỏ thanh ngang và hạ rừng lên đồi, bỏ dải tô, bóng đèn giọt tô,
trăng lưỡi liềm, quầy hai có lồng đèn, bản gọn giữ khói/ghế/hai quầy). Reader cũng nói «không phải ký hoạ tay mà là
line-art phẳng» — đó là ngôn ngữ chung của Nếp, không đổi riêng ở đây.

Số: cổng XML r14 xanh 4/4 (không thêm chữ); khối dẫn cao thêm +83dp ở 1.0, +61dp ở 2.0 (mục tiêu ≤ +80 ở 2.0);
chi tiết không ảnh mở bằng cùng tờ. Finish reviewer (context mới) **đọc mù dải cắt từ ảnh native trước packet**: hiên quán = «quán ăn ngoài trời dưới mái
hiên, nồi tròn có nắp bốc khói, ngồi ghế đẩu», đồi = «điểm ngắm cảnh có lan can», chợ đêm đúng. `fix` năm điểm — cây
treo không gọi tên được, lồng đèn = trứng, câu a11y kể đạo cụ bản gọn không vẽ và gọi «rèm» ở hiên quán, tờ ghi gọn
mất dòng, thiếu test buộc câu a11y khớp lớp — sửa cả năm (chậu treo có thân và lá, lồng đèn nắp–thân–tua, bảng đạo cụ
theo sân khấu và theo khung đọc, bản gọn là **tập con** của bản đủ, test «bỏ một tag làm câu đổi ⇔ làm hình đổi»),
chụp lại, reviewer chấm **resolved cả năm**, `ship` trong phạm vi này.

Chưa chứng minh: người ngoài đọc ra «quán nướng ngoài hiên» hay chỉ «có bàn có đèn» — cần người thật (bảng không
nhãn `ky-hoa-trong-so/anh/r17-97-*`); live chưa có stack; máy thật; TalkBack (câu a11y có, chưa nghe).

## Tổng kết lượt 3 (11/09) — đối chiếu bảng bàn giao của Codex

| Mục Codex | PR | Kết quả | Còn để team |
|---|---|---|---|
| 1. Sửa R2 theo B11/B12, canary đỏ trên reset/dump/scale hỏng | #601 | `do-motion.sh` v3: chín điều kiện hợp lệ, exit 3/4/5, canary 16 nhánh (đỏ 11/16 trên v2), lượt thật 8/8 hàng | Đo release qua HTTPS local |
| 2. Giữ các sửa UI đã đạt | — | Không mở lại R1/R3/R5/F45/R4 | — |
| 3. Thử nghĩa không nhãn với người ngoài | — | Chưa làm (cần người thật); reviewer AI đọc mù chỉ là lớp lọc trước | F45, hai cảnh A2, **ký hoạ mới** (bảng không nhãn `ky-hoa-trong-so/anh/r17-97-*`) |
| 4a. Stunning: Khám phá có cảm giác nơi chốn | PR D | Ký hoạ trong sổ ở lead và chi tiết không ảnh; cổng XML R3 giữ; khối dẫn +83/+61dp | Live có stack; người ngoài đọc «quán nướng hiên» hay chỉ «có bàn có đèn» |
| 4b. Stunning: tương quan chất giấy ở tối | #603 | Sổ đóng trên bàn: vải nền (stddev 0.8–2.1 → 8.1–8.6), giấy đêm cho hình; thẻ/chữ giữ token; nền thực `#1c1f36` ghi vào DESIGN | Màn OLED thật; các màn tối khác chỉ đổi vân nền, chưa chụp hết |
| A4 nút bình chọn | #602 | «Bình chọn» có chữ, viền trung tính; flow 06/65 vẫn bấm | Người ngoài phân biệt với «xem thống kê» |

Ba bẫy mới ghi vào memory: ô vân «trung tính» nâng nền tối nhất +7 mức; `rudi-khong-hex` đọc cả comment và ruff format
phải chạy SAU lần sửa cuối; chuỗi `cd apps/mobile && …` đứt khi cwd đã ở đó → build/test trên `dist-test` cũ.
