---
name: Rủ Đi
description: Cuốn sổ chuyến đi của cả hội, bìa vải indigo và trang giấy sáng, ba cuộn washi mang nghĩa, trạng thái là con dấu
colors:
  ground: "#f7f3ec"
  card: "#ffffff"
  line: "#e6dfd3"
  line-strong: "#777580"
  ink: "#1f2230"
  ink-soft: "#4e5563"
  ink-faint: "#676e7b"
  accent: "#ba3e20"
  accent-end: "#ba3e20"
  accent-ink: "#ffffff"
  accent-soft: "#fff0ea"
  split: "#00756b"
  split-ink: "#ffffff"
  split-soft: "#e6f2ee"
  ai: "#7356a6"
  ai-ink: "#ffffff"
  ai-soft: "#f1ebf7"
  warn: "#c2410c"
  cover: "#1d2140"
  cover-ink: "#f7f3ec"
  cover-ink-soft: "#c9c6d6"
  cover-line: "#3a3f63"
  cover-line-strong: "#8d92bd"
  ground-dark: "#151830"
  card-dark: "#1f2340"
  line-dark: "#363b5e"
  line-strong-dark: "#7d82a9"
  ink-dark: "#f4f1ea"
  ink-soft-dark: "#c4c2cf"
  ink-faint-dark: "#9b9aae"
  accent-dark: "#fb693e"
  accent-end-dark: "#fb693e"
  accent-ink-dark: "#1c0d06"
  accent-soft-dark: "#3d1a10"
  split-dark: "#74cec0"
  split-ink-dark: "#04201d"
  split-soft-dark: "#153734"
  ai-dark: "#c7b1e5"
  ai-ink-dark: "#150a30"
  ai-soft-dark: "#30273f"
  warn-dark: "#e8734b"
  cover-dark: "#0f1126"
  cover-ink-dark: "#f4f1ea"
  cover-ink-soft-dark: "#c4c2cf"
  cover-line-dark: "#2e3255"
  cover-line-strong-dark: "#9095c0"
  brand-glow: "#fc7b37"
  brand-coral: "#fb693e"
  brand-coral-ink: "#1f2230"
  brand-rose: "#e75262"
  brand-violet: "#8350f6"
  brand-teal: "#04a89d"
typography:
  hero:
    fontFamily: "BricolageGrotesque-ExtraBold"
    fontSize: "40sp"
    fontWeight: 800
    lineHeight: "44sp"
    letterSpacing: "-1.2"
  display:
    fontFamily: "BricolageGrotesque-ExtraBold"
    fontSize: "34sp"
    fontWeight: 800
    lineHeight: "39sp"
    letterSpacing: "-1.1"
  h1:
    fontFamily: "BricolageGrotesque-ExtraBold"
    fontSize: "28sp"
    fontWeight: 800
    lineHeight: "34sp"
    letterSpacing: "-0.65"
  h2:
    fontFamily: "BricolageGrotesque-Bold"
    fontSize: "21sp"
    fontWeight: 700
    lineHeight: "27sp"
    letterSpacing: "-0.3"
  title:
    fontFamily: "system (Roboto / SF)"
    fontSize: "17sp"
    fontWeight: 700
    lineHeight: "23sp"
    letterSpacing: "-0.15"
  body:
    fontFamily: "system (Roboto / SF)"
    fontSize: "17sp"
    fontWeight: 400
    lineHeight: "24sp"
  label:
    fontFamily: "system (Roboto / SF)"
    fontSize: "14sp"
    fontWeight: 600
    lineHeight: "19sp"
  caption:
    fontFamily: "system (Roboto / SF)"
    fontSize: "13sp"
    fontWeight: 600
    lineHeight: "18sp"
  stamp:
    fontFamily: "BricolageGrotesque-CondensedBold"
    fontSize: "12sp"
    fontWeight: 700
    lineHeight: "14sp"
    letterSpacing: "0.8"
    textTransform: "uppercase"
  money:
    fontFamily: "BricolageGrotesque-ExtraBold"
    fontSize: "21sp"
    fontWeight: 800
    lineHeight: "27sp"
    fontFeature: "tnum"
rounded:
  base: "20dp"
  stamp-cta: "16dp"
  control: "14dp"
  small: "10dp"
  stamp: "6dp"
  print: "4dp"
  cover-band: "28dp"
  pill: "999dp"
spacing:
  xs: "6dp"
  sm: "10dp"
  md: "16dp"
  lg: "24dp"
  xl: "36dp"
  xxl: "48dp"
components:
  stamp-button:
    backgroundColor: "{colors.brand-coral}"
    textColor: "{colors.ink}"
    typography: "BricolageGrotesque-Bold 21/26 +0.4"
    rounded: "{rounded.stamp-cta}"
    padding: "0 30dp"
    height: "60dp"
  stamp-button-form:
    backgroundColor: "{colors.brand-coral}"
    textColor: "{colors.ink}"
    typography: "BricolageGrotesque-Bold 17/22 +0.4"
    rounded: "{rounded.control}"
    padding: "0 24dp"
    height: "52dp"
  cover-button:
    backgroundColor: "transparent"
    textColor: "{colors.cover-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 18dp"
    height: "50dp"
  cover-link:
    backgroundColor: "transparent"
    textColor: "{colors.cover-ink}"
    typography: "{typography.label}"
    padding: "0 12dp"
    height: "48dp"
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 18dp"
    height: "52dp"
  button-outline:
    backgroundColor: "{colors.card}"
    textColor: "{colors.accent}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 18dp"
    height: "52dp"
  field:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "0 14dp"
    height: "52dp"
  chip:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink-soft}"
    typography: "{typography.caption}"
    rounded: "{rounded.pill}"
    padding: "10dp 12dp"
    height: "48dp"
  stamp:
    backgroundColor: "transparent"
    textColor: "{colors.accent}"
    typography: "{typography.stamp}"
    rounded: "{rounded.stamp}"
    padding: "4dp 8dp"
    height: "26dp"
  stamp-ink:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.stamp}"
    rounded: "{rounded.stamp}"
    padding: "4dp 8dp"
    height: "26dp"
  ledger-row:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    padding: "10dp 0"
    height: "52dp"
  print-frame:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    typography: "{typography.caption}"
    rounded: "{rounded.small}"
    padding: "8dp 8dp 14dp"
  ai-note:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    padding: "12dp 0"
  cover-band:
    backgroundColor: "{colors.cover}"
    textColor: "{colors.cover-ink}"
    rounded: "{rounded.cover-band}"
    padding: "16dp 16dp 24dp"
  tab-bar:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink-faint}"
    typography: "{typography.caption}"
    height: "64dp"
  tab-bar-active:
    textColor: "{colors.accent}"
  fab:
    backgroundColor: "{colors.brand-coral}"
    textColor: "{colors.brand-coral-ink}"
    rounded: "{rounded.pill}"
    size: "56dp"
---

# Design System: Rủ Đi

<!-- impeccable:design-schema 2 -->

Hệ thiết kế v2 của **Rủ Đi**, ghi từ artifact **đã ship** của đợt «chuyển
mình» (nhánh `claude/p0-w-ui2-bo-cuc-theo-nhiem-vu`, bảy commit `ab53e789` →
`06a4722f`, 2026-09-06). Finish reviewer trả `ship` sau khi tám mục vật chất
được chấm resolved: con dấu CTA, ô soạn chat, footer lịch trình, lưới album và
số ảnh thật, ảnh in trong khung có dòng xuất xứ, sổ thay hero metric, không
kicker trên tiêu đề và AI ký ở chân, huy hiệu đếm từ số thật. **Đợt 8** (ba
commit `4570fdd3` → `f838ca45` → `d2c51977` cùng ngày, cộng một sửa chưa
commit ở `ui/Stamp.tsx` + `ui.tsx` tách pha lún của con dấu ra shared value
riêng và kẻ tóc dưới khe `header`) thêm cú đóng dấu, phản hồi bấm trên UI
thread, ảnh chặng trên dòng thời gian, tay cầm sheet kéo được thật và header
tab chỉ còn wordmark; reviewer trả `ship` cho hai mục vật chất nó nêu. Mốc
trước (`d168b63`, UI-0/UI-1) và checkpoint trưa 2026-09-06 nằm dưới mốc này;
chỗ nào hợp đồng hướng đi và bản ship lệch nhau thì **bản ship thắng** và
được ghi rõ.

**Đợt «bản sắc và nhịp thị giác» (2026-09-08)**, nhánh
`claude/p0-w-ui3-hoan-thien-native`, commit `cdccb590` → `8c1d8600` trên
`3d89f070`, thêm **lớp vẽ** (`src/rudi/art/*.ts` + `src/rudi/ui/art/*.tsx`:
mascot Nếp, tám hình gu, ba motif, năm cảnh rỗng), bậc chữ `note`, khe
`leading` của chip, cặp so sánh địa điểm và lý do dưới ảnh dẫn, album theo
ngày, và bộ luật câu chữ §8.5 của báo cáo 07/09. Bằng chứng native trong
`.impeccable/review/ban-sac-2026-09-08/`: `sua2-sang-1.0/` (sáng 1.0, bản
cuối), `sua-sang-1.0/` (album, dòng thời gian, check-in, khay tạo), `sua-ab/`
(cảnh có / không Nếp), `sua-sang-1.3/bs-03*` (Sở thích ở font 1.3),
`toi-1.3/` (tối, **trước** loạt sửa cuối), `art-preview.png` và bảng art
sáng + tối. Finish reviewer trả `ship` cho màn fixture ở sáng 1.0 và Sở thích
ở 1.3; **chưa phủ**: các cửa live, tối sau loạt sửa, tương phản nút vô hiệu
của kit (nợ có tên, xem mục cuối).

**Bằng chứng của từng câu.** Số đo (dp, sp, opacity, tỉ lệ) đọc từ
`packages/shared/tokens.json`, `src/rudi/theme.ts`, `motion.ts`,
`adaptive.ts`, kit `src/rudi/ui.tsx` + `src/rudi/ui/*.tsx` và hai hàng
feature `screens/explore/HangDiaDiem.tsx`, `screens/keo/HangChang.tsx`. Cách
nó *trông* trên máy đối chiếu bằng ảnh native Android trong
`.impeccable/review/chuyen-minh/` (phone sáng 1.0 hành trình 01–22, phone tối
font 1.3, tablet sáng); tài liệu này đã mở `01-welcome`, `09-itinerary`,
`15-settlement`, `18-album`, `21-finance`, `tablet-light-explore`, và cho
đợt 8 thư mục `dot8/`: `phone-light-04-explore`, `07-plan`,
`12-create-sheet`, khung 26 của `clip-dau-tra-20fps` (con dấu «ĐÃ TRẢ» vừa
chạm mực đầy cạnh một con dấu còn nhạt). Câu nào chỉ có từ mã (iOS,
`BlurView`, OAuth, `/create` mở lạnh từ deep link) được đánh dấu như vậy;
không suy hành vi người dùng từ ảnh, và số mili giây của cú đóng dấu đọc từ
`Stamp.tsx`, không đo từ clip 20 fps.

Nguồn số duy nhất là `packages/shared/tokens.json` (`theme.ts` đọc, `guest.css`
soi gương, `test_shared_tokens.py` so từng token). Hai mục «Màu, kèm số đo
tương phản» và «Sàn phi-chữ 3:1» bên dưới do `scripts/sinh_token_ui_v2.py`
sinh ra và `test_contrast_floor.py` đối chiếu từng tỉ lệ; không sửa tay.
`.impeccable/design.json` là bản máy đọc, cùng script viết các khối số.

**Phạm vi.** Đợt này chuyển 21 màn sang bố cục theo nhiệm vụ: vào cửa
(Welcome, Login, OTP, Onboarding), Khám phá + chi tiết địa điểm, kèo/lịch
trình, nhóm + chat + thẻ AI, Bill (hoá đơn, gán món, quyết toán), Cá nhân
(sổ tài chính, huy hiệu), Kỷ niệm/album. Dữ liệu trên ảnh là fixture, dán
«Dữ liệu demo»/«Demo»/«Nháp»; tài liệu này không đọc bộ ảnh xanh thành «sản
phẩm đúng».

## Overview

**Creative North Star: "Nhật ký chuyến đi sau giờ làm"**

Một cuốn sổ chuyến đi cả hội cùng viết trong một buổi tối. *Bìa* vải indigo
là bề mặt thuyết phục (Welcome, `CoverBand` đầu Login/OTP); *trang giấy*
trắng ngà có vân là bề mặt làm việc, và trên giấy **hàng nằm thẳng trên trang,
ngăn bằng kẻ tóc**, không có thẻ lồng thẻ. Ba tông bão hoà mang nghĩa (cam =
lời rủ, teal = tiền, tím = AI) chỉ dán lên **vùng đang quan trọng**, và trên
màn tiền tông teal chỉ đậu trên **con số**, không tô cả khối. Trạng thái đúng
là *con dấu* có chữ; bản nháp là *nét chì đứt*; kế hoạch là *một đường mực
liên tục*; ảnh là *bản in* dán vào trang, có dòng xuất xứ; tiền là *dòng sổ*;
thứ máy sinh ra là *tờ giấy ký ở chân* «Rủ Đi AI gợi ý». Khung kẻ in trước,
màu đổ sau khi có dữ liệu. Lưới 4pt, snap ô nguyên. Hợp đồng hướng đi nằm
nguyên văn trong `apps/mobile/app/_layout.tsx` (seed `c8e88116`, hướng số 6);
ADR-0020 là thẩm quyền.

Thế giới này **từ chối mặc định của thể loại**: ảnh hoàng hôn + thẻ trắng +
pill cam, và dashboard số to + thanh tiến độ. Hai vòng review đã gọt nó: pill
coral thành con dấu (vòng 1); con dấu **rộng bằng chữ** với **một** vành mực
vẽ bằng đường SVG gãy nhẹ, không mũi tên, và hero metric thành sổ (vòng
2026-09-06). Cũng bị loại: dập nổi bằng bóng lệch cứng, kicker/eyebrow trên
tiêu đề, nhãn «AI» đặt trên đầu thẻ.

Mật độ: một quyết định mỗi màn; cột form tối đa 560dp; đích bấm 48dp; body
17sp; caption dùng chung 13sp; tem, nhãn tab 12sp và `DemoBadge` 10sp là cỡ
riêng. Không toast, không modal lỗi; lỗi là một câu `warn` dưới form. Mỗi màn
tiền có một câu nói rõ số này là gì và không phải gì («Chưa confirm sổ cái.
Đây không phải số dư ngân hàng»).

**Key Characteristics:**
- Hai bề mặt vật chất đo được bằng pixel: vải bìa (stddev ≈ 8 mức trên
  `#1d2140`), giấy (≈ 2 mức trên `#f7f3ec`), mực trong con dấu (≈ 8.6 mức
  trên coral).
- Ba tông mang nghĩa, một tông dẫn mỗi màn; `brand.coral` chỉ ở mảng lớn
  (washi, con dấu CTA, FAB, chặng đang ở); teal/tím trên giấy chỉ ở chữ, số,
  viền, con dấu, nút.
- Một display face tự host (Bricolage Grotesque, bốn instance tĩnh) cho tiêu
  đề, số tiền, chữ con dấu; body giữ system.
- Bốn hình dạng kể chuyện: con dấu (trạng thái đúng), nét chì đứt (nháp),
  đường mực liên tục (lịch trình), khung in (ảnh có xuất xứ).
- Sổ thay bảng điều khiển: hàng tên trái, số phải, kẻ tóc dưới; số nguyên
  đồng, tabular, không animate trước domain state.
- Bốn bậc chuyển động (100/200/300/550 ms), Reduce Motion đưa mọi bậc trừ
  `instant` về 0; scale và opacity là đường chính.
- Mọi cú bấm là một lò xo scale trên UI thread (`PressScale`); trạng thái
  thành đúng dưới ngón tay là **một cú đóng dấu** ba nhịp trong ngân sách
  `celebrate`, một lần mỗi sự kiện.

## Colors

Bảng màu có **hai bề mặt và hai scheme**: giấy (`ground`/`card`) và bìa
(`cover`), mỗi cái có bộ mực riêng; bốn nhóm đo đủ ở sáng và tối. Tầng
thương hiệu (`brand.*`) giữ nguyên số đo từ logo, không chỉnh theo tương phản,
và vì thế bị giới hạn công dụng.
Ba màu ngữ nghĩa hiện tại ở scheme sáng là cam đất (`accent`), teal tiền
(`split`) và tím dịu (`ai`), theo frontmatter đồng bộ `tokens.json`.
Màu thương hiệu vẫn giữ riêng trong `brand.*` cho mảng lớn; không lấy
gradient thương hiệu cũ làm giá trị của `accent` runtime.

### Primary
- **Cam hành động** (`accent`): tông thương hiệu và
  hành động chính. Chữ cam trên giấy, chỉ báo tab đang chọn, viền chip đã
  chọn. `RudiButton solid` dùng nền tông phẳng (nút «Thêm khoảnh khắc» ở
  album); `accentEnd` hiện bằng `accent` ở cả hai scheme. `StampButton` dùng
  `brand.coral` với mực tối.
- **Coral thương hiệu** (`brand.coral` #fb693e, cả hai scheme): washi cam,
  mặt con dấu CTA, FAB «Tạo mới», chặng đang ở trên route. Luôn là mảng lớn,
  luôn đi với mực tối tĩnh (xem luật Mực Tĩnh).

### Secondary
- **Teal tiền** (`split`): chia bill, tiền, quyết toán. Trên bản ship teal
  đậu ở **số** (`Money tone="split"`), con dấu «ĐÃ TRẢ»/«NGƯỜI THU BILL», viền
  nút outline «Đánh dấu đã trả», vòng avatar người thu; nền `splitSoft` chỉ
  ở đĩa icon nhỏ và ô đã chọn của `RosterPicker`. Washi teal là `brand.teal`
  #04a89d.
- **Tím AI** (`ai`): thứ máy sinh ra, người còn sửa được: ghi chú lề
  `AiNote`, chữ ký chân tờ AI, con dấu «HỢP GU», hai nút chân lịch trình AI
  («Chỉnh lịch trình» outline, «Dùng plan này» solid). Washi tím là
  `brand.violet` #8350f6.
- **Cảnh báo** (`warn` #c2410c / #e8734b): một câu lỗi dưới form, chữ `body`;
  trong sổ là màu của số **còn phải trả** khi lớn hơn 0 (`DongTien
  tone="warn"`), số 0 thì về `ink`.

### Neutral
- **Giấy** (`ground` #f7f3ec / #151830): nền trang, luôn có `Grain giayTrang`
  phủ 0.45 (tối 0.30).
- **Thẻ** (`card` #ffffff / #1f2340): thẻ, ô nhập, nút outline, thanh tab.
- **Mực** (`ink` #1f2230 / #f4f1ea) · **mực phụ** (`inkSoft`) · **mực nhạt**
  (`inkFaint`): ba bậc chữ trên giấy, tất cả qua AA ở cả hai nền.
- **Kẻ trang trí** (`line` #e6dfd3 / #363b5e): cạnh thẻ, divider, xương
  skeleton; **cố ý dưới 3:1**.
- **Viền control** (`lineStrong`): ô nhập, chip chưa chọn,
  nút outline, tay nắm sheet; qua sàn 3:1 trên mọi nền nó nằm lên.
- **Bìa** (`cover` #1d2140 / #0f1126) với **mực bìa** (`coverInk`,
  `coverInkSoft`) và hai viền bìa (`coverLine` trang trí, `coverLineStrong`
  control 3:1). Bìa tối ở cả hai scheme; tối chỉ tối hơn một bậc.

### Named Rules
**Luật Một Tông Dẫn.** Một màn có đúng một tông dẫn; hai tông dẫn cùng lúc là
lỗi, không phải lựa chọn. Welcome/Login/OTP dẫn bằng cam (washi + con dấu);
Quyết toán và Tài chính dẫn bằng teal; Lịch trình AI và thẻ AI dẫn bằng tím;
con dấu «HỢP GU» tím trên Khám phá là tông ở **thành phần**, không đổi tông
màn.

**Luật Màu Trên Số, Không Trên Khối.** Trên trang giấy, tông ngữ nghĩa đậu lên
chữ số, chữ, viền, con dấu; không tô một khối `<tone>Soft` to rồi đặt số lên.
Sổ quyết toán trên ảnh `15-settlement`: mọi teal là số hoặc viền, nền vẫn là
giấy.

**Luật Mực Tĩnh trên Coral.** `brand.coral` không đổi theo scheme nên chữ,
icon và viền đặt lên nó dùng `mauSang.ink` (#1f2230) **tĩnh**, không dùng
`colors.ink`. Đo: 5.41:1 ở cả hai scheme; mực sáng của scheme tối trên coral
chỉ 2.4:1 và đã ship nhầm một lần. Áp cho tagline trên washi, nhãn/viền/icon
`StampButton`, glyph chặng đang ở của `RouteLine`.

**Luật Cam Không Nhỏ trên Bìa.** `accent` sáng trên `cover` đo 2.83:1: cấm
chữ cam nhỏ trên bìa. Cam trên bìa là washi (mảng lớn) hoặc con dấu có mực
tối; chữ trên bìa là `coverInk`/`coverInkSoft`.

**Luật Hai Viền.** Ranh giới của thứ bấm được vẽ bằng `lineStrong` (hoặc
`coverLineStrong` trên bìa); cạnh của container vẽ bằng `line`. Thêm một
control là thêm một dòng trong `interactive_boundaries()` của
`test_contrast_floor.py`; control không có dòng ở đó là control không ai đo.

**Luật Vai Màu Của Nét Vẽ.** Lớp vẽ không biết màu. Mỗi lớp (`LopVe`) gọi
tên một **vai** trong bảy vai `MauVe` (`giay` giấy · `bong` mặt gấp trong
bóng · `muc` mực · `gap` góc gấp coral · `mo` màu rửa nhạt · `split` · `ai`),
và `VeLop.mauLop` mới đổi vai ra token của scheme: `giay → card`, `bong →
line`, `muc → ink`, `gap → accent`, `mo → accentSoft`, `split → split`, `ai →
ai`. Nhờ vậy một hình vẽ giữ bóng dáng trên cả giấy sáng lẫn vải tối (bảng
art hai nửa: giấy tối đi, mực sáng lên, coral y nguyên) và `rudi-khong-hex`
vẫn giữ `theme.ts` là file duy nhất viết hex. `doiMau` chỉ đổi **một** vai
tại chỗ (ô đã chọn: `muc → accent`), không tạo bảng màu riêng cho tranh.

## Màu, kèm số đo tương phản

50 cặp chữ trên nền mà hệ này thật sự dùng đều được đo, cả trang giấy lẫn bìa sổ. Thấp nhất **4.64:1**, cao nhất **16.50:1**, không cặp nào dưới ngưỡng AA 4.5:1.

Bảng này chỉ đo **chữ**. Ranh giới của thành phần giao diện đi theo ngưỡng khác và nằm ở mục "Sàn phi-chữ 3:1" bên dưới. Đọc thiếu mục đó là cách lỗi viền nút 1.21:1 đã lọt qua một lần.

### Chế độ sáng

| Cặp | Vai trò | Tỉ lệ | Ngưỡng |
|---|---|---|---|
| `ink` #1f2230 trên `ground` #f7f3ec | Chữ thân trên nền trang | **14.28:1** | AAA |
| `ink` #1f2230 trên `card` #ffffff | Chữ thân trên thẻ | **15.79:1** | AAA |
| `inkSoft` #4e5563 trên `card` #ffffff | Chữ phụ trên thẻ | **7.49:1** | AAA |
| `inkSoft` #4e5563 trên `ground` #f7f3ec | Chữ phụ trên nền | **6.77:1** | AA |
| `inkFaint` #676e7b trên `card` #ffffff | Chú thích trên thẻ | **5.13:1** | AA |
| `inkFaint` #676e7b trên `ground` #f7f3ec | Chú thích trên nền | **4.64:1** | AA |
| `accent` #ba3e20 trên `card` #ffffff | Cam trên thẻ | **5.53:1** | AA |
| `accent` #ba3e20 trên `ground` #f7f3ec | Cam trên nền | **5.00:1** | AA |
| `accentInk` #ffffff trên `accent` #ba3e20 | Nhãn trên nút cam | **5.53:1** | AA |
| `accent` #ba3e20 trên `accentSoft` #fff0ea | Cam trên chip cam nhạt | **4.98:1** | AA |
| `split` #00756b trên `card` #ffffff | Teal trên thẻ | **5.59:1** | AA |
| `split` #00756b trên `ground` #f7f3ec | Teal trên nền | **5.05:1** | AA |
| `splitInk` #ffffff trên `split` #00756b | Nhãn trên nút teal | **5.59:1** | AA |
| `split` #00756b trên `splitSoft` #e6f2ee | Teal trên chip teal nhạt | **4.87:1** | AA |
| `ai` #7356a6 trên `card` #ffffff | Tím trên thẻ | **5.82:1** | AA |
| `ai` #7356a6 trên `ground` #f7f3ec | Tím trên nền | **5.26:1** | AA |
| `aiInk` #ffffff trên `ai` #7356a6 | Nhãn trên nút tím | **5.82:1** | AA |
| `ai` #7356a6 trên `aiSoft` #f1ebf7 | Tím trên chip tím nhạt | **4.98:1** | AA |
| `warn` #c2410c trên `card` #ffffff | Cảnh báo trên thẻ | **5.18:1** | AA |
| `warn` #c2410c trên `ground` #f7f3ec | Cảnh báo trên nền | **4.68:1** | AA |
| `ink` #1f2230 trên `accentSoft` #fff0ea | Chữ thân trên chip cam | **14.22:1** | AAA |
| `ink` #1f2230 trên `splitSoft` #e6f2ee | Chữ thân trên chip teal | **13.76:1** | AAA |
| `ink` #1f2230 trên `aiSoft` #f1ebf7 | Chữ thân trên chip tím | **13.51:1** | AAA |
| `coverInk` #f7f3ec trên `cover` #1d2140 | Chữ trên bìa sổ | **14.13:1** | AAA |
| `coverInkSoft` #c9c6d6 trên `cover` #1d2140 | Chữ phụ trên bìa sổ | **9.33:1** | AAA |

### Chế độ tối

| Cặp | Vai trò | Tỉ lệ | Ngưỡng |
|---|---|---|---|
| `ink` #f4f1ea trên `ground` #151830 | Chữ thân trên nền trang | **15.45:1** | AAA |
| `ink` #f4f1ea trên `card` #1f2340 | Chữ thân trên thẻ | **13.56:1** | AAA |
| `inkSoft` #c4c2cf trên `card` #1f2340 | Chữ phụ trên thẻ | **8.72:1** | AAA |
| `inkSoft` #c4c2cf trên `ground` #151830 | Chữ phụ trên nền | **9.93:1** | AAA |
| `inkFaint` #9b9aae trên `card` #1f2340 | Chú thích trên thẻ | **5.56:1** | AA |
| `inkFaint` #9b9aae trên `ground` #151830 | Chú thích trên nền | **6.33:1** | AA |
| `accent` #fb693e trên `card` #1f2340 | Cam trên thẻ | **5.24:1** | AA |
| `accent` #fb693e trên `ground` #151830 | Cam trên nền | **5.97:1** | AA |
| `accentInk` #1c0d06 trên `accent` #fb693e | Nhãn trên nút cam | **6.48:1** | AA |
| `accent` #fb693e trên `accentSoft` #3d1a10 | Cam trên chip cam nhạt | **5.31:1** | AA |
| `split` #74cec0 trên `card` #1f2340 | Teal trên thẻ | **8.26:1** | AAA |
| `split` #74cec0 trên `ground` #151830 | Teal trên nền | **9.40:1** | AAA |
| `splitInk` #04201d trên `split` #74cec0 | Nhãn trên nút teal | **9.22:1** | AAA |
| `split` #74cec0 trên `splitSoft` #153734 | Teal trên chip teal nhạt | **6.96:1** | AA |
| `ai` #c7b1e5 trên `card` #1f2340 | Tím trên thẻ | **7.90:1** | AAA |
| `ai` #c7b1e5 trên `ground` #151830 | Tím trên nền | **9.00:1** | AAA |
| `aiInk` #150a30 trên `ai` #c7b1e5 | Nhãn trên nút tím | **9.70:1** | AAA |
| `ai` #c7b1e5 trên `aiSoft` #30273f | Tím trên chip tím nhạt | **7.29:1** | AAA |
| `warn` #e8734b trên `card` #1f2340 | Cảnh báo trên thẻ | **5.09:1** | AA |
| `warn` #e8734b trên `ground` #151830 | Cảnh báo trên nền | **5.80:1** | AA |
| `ink` #f4f1ea trên `accentSoft` #3d1a10 | Chữ thân trên chip cam | **13.75:1** | AAA |
| `ink` #f4f1ea trên `splitSoft` #153734 | Chữ thân trên chip teal | **11.44:1** | AAA |
| `ink` #f4f1ea trên `aiSoft` #30273f | Chữ thân trên chip tím | **12.51:1** | AAA |
| `coverInk` #f4f1ea trên `cover` #0f1126 | Chữ trên bìa sổ | **16.50:1** | AAA |
| `coverInkSoft` #c4c2cf trên `cover` #0f1126 | Chữ phụ trên bìa sổ | **10.60:1** | AAA |

## Sàn phi-chữ 3:1 (WCAG 1.4.11)

Bảng 50 cặp bên trên chỉ đo **chữ trên nền**. Ở bản cũ, nó không đo
token `line`, và đó là một lỗ thật chứ không phải thiếu sót hình thức: nút
`quiet` không có nền (`backgroundColor: "transparent"`), nên **viền là thứ duy
nhất cho biết nó là nút**. Viền đó vẽ bằng `line`, đo được **1.21:1** trên nền
trang. Một cổng chỉ đo chữ báo xanh hoàn hảo trong khi cái nút gần như vô hình.

WCAG 1.4.11 đòi **3:1** cho ranh giới của **thành phần giao diện**, tức thứ
người ta bấm được. Nó **không** đòi gì ở cạnh trang trí của một container. Hai
ngưỡng khác nhau thì cần hai token, nên `line` tách làm hai:

| Token | Việc của nó | Sàn |
|---|---|---|
| `line` | Cạnh thẻ, đường kẻ chia, rãnh trích dẫn. Container, không phải control | không có sàn, cố ý giữ mềm theo mockup |
| `lineStrong` | Ranh giới của thứ bấm được: nút quiet, ô nhập, chip chưa chọn, con trượt thanh cuộn | **3:1 trên mọi nền nó nằm lên** |

### Chế độ sáng

| Cặp | Vai trò | Tỉ lệ | Ngưỡng |
|---|---|---|---|
| `lineStrong` #777580 trên `ground` #f7f3ec | Viền control trên nền trang | **4.09:1** | 1.4.11 |
| `lineStrong` #777580 trên `card` #ffffff | Viền control trên thẻ | **4.53:1** | 1.4.11 |
| `coverLineStrong` #8d92bd trên `cover` #1d2140 | Viền control trên bìa sổ | **5.19:1** | 1.4.11 |
| `line` #e6dfd3 trên `ground` #f7f3ec | Cạnh thẻ trên nền trang | **1.20:1** | trang trí |
| `line` #e6dfd3 trên `card` #ffffff | Đường kẻ trong thẻ | **1.32:1** | trang trí |
| `coverLine` #3a3f63 trên `cover` #1d2140 | Đường kẻ trên bìa | **1.54:1** | trang trí |

### Chế độ tối

| Cặp | Vai trò | Tỉ lệ | Ngưỡng |
|---|---|---|---|
| `lineStrong` #7d82a9 trên `ground` #151830 | Viền control trên nền trang | **4.68:1** | 1.4.11 |
| `lineStrong` #7d82a9 trên `card` #1f2340 | Viền control trên thẻ | **4.11:1** | 1.4.11 |
| `coverLineStrong` #9095c0 trên `cover` #0f1126 | Viền control trên bìa sổ | **6.42:1** | 1.4.11 |
| `line` #363b5e trên `ground` #151830 | Cạnh thẻ trên nền trang | **1.61:1** | trang trí |
| `line` #363b5e trên `card` #1f2340 | Đường kẻ trong thẻ | **1.42:1** | trang trí |
| `coverLine` #2e3255 trên `cover` #0f1126 | Đường kẻ trên bìa | **1.51:1** | trang trí |

Số của `line` và `coverLine` ghi ra ở đây **chính vì chúng không đạt 3:1**. Người sau đọc bảng này phải thấy ngay chúng đứng ở đâu, thay vì thấy một token không có số rồi dùng nó cho một cái nút. `coverLineStrong` là viền của control đặt trên bìa sổ (Welcome, Login), đo trên cả hai scheme.

### Tầng thương hiệu, đo riêng và giới hạn riêng

Bốn màu này giữ **nguyên số đo từ logo**, không chỉnh theo tương phản, vì chỉnh
là mất nhận diện. Đổi lại chúng bị giới hạn công dụng:

| Màu | Mã | Với chữ trắng | Với `ink` #1f2230 | Được phép dùng |
|---|---|---|---|---|
| `glow` | #fc7b37 | 2.62:1 | 6.04:1 | mảng lớn, logo, hero. Cấm chữ nhỏ |
| `coral` | #fb693e | 2.92:1 | 5.41:1 | mảng lớn, logo, hero. Cấm chữ nhỏ |
| `rose` | #e75262 | 3.63:1 | 4.35:1 | mảng lớn, logo, hero. Cấm chữ nhỏ |
| `violet` | #8350f6 | 4.73:1 | 3.34:1 | mảng lớn, logo, hero. Cấm chữ nhỏ |
| `actionGradient` | #c93900 → #c9344a | 5.16:1 / 5.16:1 | | token gradient thương hiệu còn lưu; không phải nền `RudiButton solid` hiện tại |
Cam `coral` với chữ trắng chỉ đạt 2.92:1, dưới cả ngưỡng 3:1 của thành phần
giao diện. Chữ/icon trên coral dùng mực tối tĩnh theo luật Mực Tĩnh;
`RudiButton` dùng cặp `<tone>`/`<tone>Ink` của scheme hiện tại. Bảng thương
hiệu này không thay thế bảng token ngữ nghĩa và không cấp phép mực trắng
trên coral.

## Typography

**Display Font:** Bricolage Grotesque (OFL 1.1, tự host, bốn instance tĩnh cắt
bằng `fontTools.varLib.instancer`; nạp runtime qua `expo-font`, không nhúng
lúc build)
**Body Font:** system (Roboto trên Android, SF trên iOS)
**Wordmark:** SVG từ outline Baloo 2 ExtraBold nghiêng 9° (`ui/Wordmark.tsx`,
tỉ lệ 2.8804), không còn `fontStyle: "italic"` giả wordmark

**Character:** một grotesque «lắp ghép từ mảnh tìm được» cho tiêu đề, số tiền
và chữ con dấu, đứng trên body system trung tính. Bricolage có trục `wdth`
(instance Condensed cho tem) và `tnum` (số tiền tabular). Bộ chữ Việt 527
glyph, đã kiểm «ế ự ỡ ạ ổ ầ ẫ ỹ Đ» ở 12/17/28/40 sp trên emulator ở font 1.0
và 1.3 (ảnh trong `docs/claude/2026-09-05/`). Body giữ system là **quyết định**
(ADR-0020 §2.3): dấu tiếng Việt và cỡ chữ hệ thống chắc chắn, đổi face không
cần rebuild dev client.

### Hierarchy
Thang app trong `theme.ts` (`typography`), `lineHeight` tường minh vì RN không
tự tính; `fontWeight: "normal"` ở các bậc Bricolage vì độ đậm đã nướng vào
instance.

- **Hero** (ExtraBold, 40/44, -1.2): một lần mỗi màn, láng giềng của wordmark;
  «Chào bạn», «Nhập mã 6 số» trong `CoverBand`.
- **Display** (ExtraBold, 34/39, -1.1): số tiền tổng trên thẻ tông, một màn
  một lần.
- **H1** (ExtraBold, 28/34, -0.65): tiêu đề màn. Tiêu đề trang pager Welcome
  dùng cùng face ở 28/33.
- **H2** (Bold, 21/27, -0.3): `SectionHeader` («Chi theo nhóm», «Các khoản
  chuyển»), tên địa điểm dẫn (`PlaceLead`), tiêu đề trạng thái rỗng.
- **Title** (system 700, 17/23, -0.15): tiêu đề `TopBar`, tiêu đề thẻ, chữ số
  OTP.
- **Body** (system 400, 17/24): chữ thân, ô nhập, tên dòng sổ thường; bề
  rộng tối đa 520 khi là đoạn dẫn.
- **Label** (system 600, 14/19): nhãn nút, nhãn ô nhập, `CoverButton`, tên
  chặng và giờ (tabular) trên `HangChang`, tên dòng sổ **đậm** (`DongTien
  dam`), câu ghi chú AI.
- **Caption** (system 600, 13/18): chip, nhãn ngắn, pháp lý. Nhãn tab có override
  12/14; `Stamp` 12/14 và `DemoBadge` 10/12 là các cỡ riêng, không phải caption.
- **Note** (system 400, 13/18, `typography.note`, thêm 2026-09-08): dòng phụ
  **là một câu** ở cỡ caption nhưng độ đậm đọc: siêu dữ liệu («17 - 19/10/2026
  · 4 ảnh»), chú thích («Không bắt buộc · K là nghìn đồng»), đếm («Chọn ít
  nhất 3 để tiếp tục» khi đã đủ), nhãn ngày trong album, mô tả và sự thật
  trên ô so sánh, dòng chi tiết của khay tạo, câu lưu trữ cuối Sở thích, phần
  giải thích «Cách tính». Cùng cỡ `micro` của `tokens.json`, không phải bậc
  mới của thang chung.
- **Stamp** (CondensedBold, 12/14, +0.8, IN HOA): chữ trên con dấu trạng thái
  và tem; đây là chữ in hoa giãn duy nhất của hệ, và nó là **mực dấu**, không
  phải eyebrow.
- **Money** (ExtraBold, 21/27, `tabular-nums`): mọi số tiền qua `Money`
  (kích cỡ `display`/`money`/`body`/`label`/`caption`, luôn ép tabular).
- **Nhãn con dấu CTA** (Bold, 21/26 `lon` trên bìa, 17/22 `vua` trong form,
  +0.4): riêng cho `StampButton`; đo theo chữ, không theo cột.

Thang `tokens.json.type` dùng chung có display 34/700, h1 28/700, title 20,
body 17, label 14, micro 13. App đọc trực tiếp `body.size` và `micro.size`;
các face, độ đậm và line-height app còn lại theo `theme.ts` như bảng trên.
Không đồng nhất toàn bộ thang guest với thang native.

### Named Rules
**Luật Một Face.** Bricolage chỉ ở tiêu đề, số tiền, chữ con dấu và thương
hiệu. Nhãn nút, ô nhập, chip, body là system. Một face display thứ hai là
lỗi.

**Luật Số Tabular.** Số tiền luôn `fontVariant: ["tabular-nums"]`, luôn số
nguyên đồng, luôn là chuỗi máy chủ gửi. Một cột tiền mà chữ số nhảy bề
ngang là đọc sai. Cột giờ của lịch trình cũng tabular (`HangChang gio`).

**Luật Nhãn Đậm, Câu Nhẹ.** Ở 13sp, nhãn ngắn (chip, con số đếm chưa đủ,
nhãn ô) đi `caption` 600; một câu có chủ ngữ đi `note` 400. Caption 600 lên
một câu làm mọi dòng phụ cùng hét một cỡ (báo cáo 07/09 §4.2); `note` không
bao giờ lên chip hay nhãn nút.

**Luật Không Kicker.** Không có chữ in hoa giãn nhỏ đứng **trên** tiêu đề.
Chữ in hoa duy nhất của hệ là mực con dấu (`typography.stamp`), và con dấu
đứng **cạnh** hay **dưới** nội dung, không bao giờ làm nhãn mở đầu. Tờ AI
được ký ở **chân** («Rủ Đi AI gợi ý» 13sp tím + sparkles 15), câu hỏi là tiêu
đề, không có nhãn trên đầu.

## Layout

Lưới 4pt, thang khoảng cách sáu bước `xs` 6 · `sm` 10 · `md` 16 · `lg` 24 ·
`xl` 36 · `xxl` 48; không thêm bước thứ bảy. Nhịp giữa khối trong
`RudiScreen` là 18; cột form Login/OTP `gap` 20.

**Ba size class Android** (`src/rudi/adaptive.ts`, `useAdaptiveLayout`), tính
từ **cửa sổ hiện tại**, không từ thiết bị, nên xoay và split-screen phân
lớp lại:

| Size class | Bề rộng | `columns` | `gutter` | `rail` | `twoPane` | `maxContent` |
|---|---|---|---|---|---|---|
| `compact` | < 600dp | 1 | 16 | không, thanh tab đáy | không | bề rộng cửa sổ |
| `medium` | 600 đến 839 | 2 | 24 | rail trái 104 | chỉ khi cao ≥ 480 | 960 |
| `expanded` | ≥ 840 | 3 | 36 | rail trái 104 | có | 1200 |

`heightClass = short` dưới 480dp (máy nằm ngang hoặc IME đè sheet): Welcome hạ
wordmark xuống 72 và bỏ route; `CoverBand compact` rút vải trên dưới khi bàn
phím mở ở compact (Login/OTP).

**Lưới đo theo vùng nội dung.** `ResponsiveRow` đo chính vùng nội dung qua
`onLayout`, sau rail và lề; `gridFor(width, minItemWidth = 250, gap = 12,
maxColumns = 3)` trả số cột và **bề rộng ô làm tròn xuống dp nguyên** (ba phần
ba chính xác làm cột cuối gãy dòng trên máy, lỗi album 2026-09-06). `maxColumns`
mặc định 3 cho hàng thẻ; tường nhóm và album fixture truyền `maxColumns={6}
minItemWidth={104} gap={6}` nên tablet hiện sáu ô nhỏ chứ không ba poster;
album live theo ngày truyền `maxColumns={4} minItemWidth={150} gap={6}` (ô
150 tối thiểu, hai ô một hàng ở compact); `RosterPicker` ô 130, gap 8, tối đa
3. Vì đo vùng thật, màn medium vẫn có thể chỉ một cột. **Lưới đọc `fontScale`
khi ô chứa chữ**: ô gu ở Sở thích `minItemWidth = round(150 × max(1,
fontScale))`, nên ở 1.3 hai cột (mỗi nhãn còn một từ, Android bẻ đôi
«Shopping») rút về **một cột** thay vì cắt chữ (`sua-sang-1.3/bs-03*`).

**Tỉ lệ ảnh theo cỡ cửa sổ** (`Photo ratio`, `PlaceLead`): ảnh dẫn địa điểm
**16:10** ở compact (đầy bề ngang máy ở chiều cao đọc được), **21:9** ở
medium/expanded (đối chiếu `tablet-light-explore.png`: dải 21:9 trên cột
960). Không có chiều cao ảnh cố định cho ảnh dẫn; `height = 190` chỉ là mặc
định của `Photo` khi không truyền `ratio`.

**Bề mặt** (`RudiScreen surface`): `page` = nền `ground` + `Grain giayTrang`,
`SafeAreaView` cạnh top/left/right, lề ngang `md` (`lg` ở tablet),
`bottomInset` 32 (112 dưới thanh tab), status bar tối trên giấy sáng.
`cover` = nền `cover`, **không** cạnh top: `CoverBand underStatusBar` tự cộng
`insets.top` để vải bìa chạy liền dưới status bar, status bar sáng. `CoverBand`
`bleed` bằng lề màn (`md` compact, `lg` tablet) để vải chạm mép.

**Bốn khe của màn** (`RudiScreen`): `header` đứng **trên** hộp cuộn với kẻ
tóc `line` dưới nó (chat: `TopBar` + dòng ghim chuyến đi, nội dung cuộn dưới
kẻ như giấy dưới một đường kẻ; bằng chứng reviewer `dot8/10-chat`, không mở ở đây); `footer` ghim đáy (ô soạn
chat, hai nút chân lịch trình, «Thêm khoảnh khắc»), đệm dưới rút về 8 khi bàn
phím mở; `overlay` đặt sheet/scrim **ngoài** hộp cuộn để nó không cuộn theo
nội dung; `keepEnd` giữ cuộn ở cuối khi nội dung dài ra (luồng chat đọc từ
đuôi, và vì thế chat cần `header` để tên nhóm không trôi mất).
Nội dung cuộn **dưới** footer; ảnh `09-itinerary` chụp giữa chừng nên số tổng
đang nằm dưới footer là hành vi cuộn, không phải cắt.

**Login/OTP** là **một cột 560 bọc cả trang** (`alignSelf: center`).
**Welcome**: lề 20, wordmark 118 (compact) / 150 (medium+), route `maxWidth`
560, khối đáy `maxWidth` 640 ở medium+.

**Safe area cho thứ ghim đáy**: thanh tab `paddingBottom = max(insets.bottom,
10)`; sheet `max(insets.bottom, 16)`; Welcome `max(insets.bottom, 16) + 6`;
composer chat `max(insets.bottom, 8)`.

**Hàng chip cuộn ngang trong form**: ở font 1.3 lưới chip gập tám hàng đẩy
nút gửi khỏi màn; form dùng `ScrollView horizontal`, lưới gập chỉ khi không
có CTA bên dưới.

**`TopBar`** (`minHeight` 52): hai bên đo bề rộng tự nhiên và lấy `max` cho cả
hai để tiêu đề cân giữa; huy hiệu trong `TopBar` dùng `compactLabel`
(«Demo», «Nháp») vì ở 360dp font 1.3 không thể có cả tiêu đề cân giữa lẫn
nhãn dài. Tiêu đề có thể mang phụ đề (ảnh `18-album`: «Album Đà Lạt» + ngày).
Không có nút back thì ô trái là **wordmark trơn** (`Wordmark` `ink` cao 18;
header Khám phá tự vẽ cao 20): ô icon app gradient chỉ còn ở Welcome/Login,
không lặp lại ở đầu mỗi tab (ảnh `dot8/04-explore`, `07-plan`).

## Elevation & Depth

Hệ này lấy độ sâu từ **chất liệu, khung kẻ và kẻ tóc**, không từ bóng. Bìa và
giấy là hai lớp vật lý. Trên giấy, hàng (`ListRow`, `PlaceRow`, `DongTien`,
`HangChang`, hàng bình chọn) **không có thẻ**: kẻ tóc `line` dưới mỗi hàng là
toàn bộ cấu trúc, và ảnh `15-settlement`, `21-finance`, `tablet-light-explore`
xác nhận không còn thẻ trắng bo 20 xếp chồng. Chỉ hai thứ có bóng, và mỗi thứ
vì một lý do vật lý:

- **Con dấu** không có bóng: nó nằm **trong** trang. Vành mực và hạt mực nói
  «đã đóng lên».
- **Bản in** (`KhungAnh`) có `cardShadow` mềm: nó nằm **trên** trang, dán vào.
  Đây là bóng mềm ám nâu (`#5A3014`, iOS 0/8 đục 0.1 mờ 18; Android
  `elevation: 3`), không phải bóng lệch cứng.

Ba ô chất liệu (`assets/textures/`, 256×256, seed cố định 20260905, sinh bằng
Pillow) trải bằng `Grain`: **lưới Image thường**, ô ở đúng pixel máy, tối đa
60 view; không dùng `resizeMode="repeat"` vì Android raster một lần theo cỡ
view và vân dừng ở một phần ba trên. Opacity đo trên emulator 1x, dưới ngưỡng
này là màu phẳng:

| Chất liệu | Ô | Opacity | Đo (stddev) |
|---|---|---|---|
| Vải bìa | `vai-bia.png` | 0.30 | ≈ 8 mức trên `cover`, đều từ y 200 đến 2300 |
| Giấy | `giay-trang.png` | 0.45 sáng / 0.30 tối | ≈ 2 mức trên `ground` («ở ngưỡng, không hạ thêm») |
| Mực dấu | `muc-in.png` | 0.26 | ≈ 8.6 mức trên coral; ô giấy ở đây đo 2.1 nên có ô riêng |

Ô trắng đen trung bình trung tính nên màu token bên dưới đo vẫn đúng trong
một mức: bảng tương phản vẫn áp cho bề mặt có vân.

### Shadow Vocabulary
- **Bản in** (`cardShadow`: iOS `#5A3014` 0/8, đục 0.1, mờ 18; Android
  `elevation: 3`): chỉ `KhungAnh` và `Card` v1 còn sót; `Card` v1 ship **cả**
  viền `line` lẫn bóng vì elevation 3 gần như không thấy trên giấy và ở
  scheme tối không tách được gì. Không màn nào trong đợt này còn gọi `Card`.
- **FAB** (`elevation: 6`, 0/6, đục 0.22, mờ 10, màu `accent`): thứ duy nhất
  nổi trên thanh tab; vòng 4px màu `ground` tách nó khỏi thanh.
- **Scrim sheet** (`lopPhu.toi(0.42)`): lớp phủ ấm gần đen, không xám.
- **Tờ giấy AI** (`ToGiay` trong `TheAi.tsx`, khung `aiSheet` trong Group):
  nền `card`, viền 1px `line`, bo `base`, **không bóng**; một tờ giấy đặt lên
  trang, ký ở chân.

### Named Rules
**Luật Không Dập Nổi.** Không giả độ sâu bằng bóng lệch cứng (hard offset
shadow). Reviewer loại «wordmark dập nổi» ở vòng 1; `WordmarkEmbossed.tsx`
đã xoá khỏi cây ở `5cc57d2` và không phải hệ.

**Luật Trong Trang / Trên Trang.** Thứ *in vào* trang (con dấu, kẻ, đường
mực, chữ) không có bóng. Thứ *dán lên* trang (bản in) có một bóng mềm ám
nâu. Không có bậc thứ ba.

**Luật Không Thẻ Lồng Thẻ.** Một bề mặt `card` không chứa bề mặt `card` khác.
Danh sách là hàng + kẻ tóc trên giấy; tờ AI là một tờ, bên trong là hàng.

## Shapes

Bo góc ba bậc từ `tokens.json` giữ tỉ lệ thẻ:nút 2:1: **`base` 20** (sheet,
tờ AI, khung hoá đơn trên gỗ), **`control` 14** (nút, con dấu CTA cỡ `vua`,
ô nhập, `CoverButton`), **`small` 10** (khung bản in, thumbnail hàng địa
điểm, xương skeleton, ô bình chọn), `pill` 999 (chip bấm, huy hiệu, FAB tròn
56, nút back tròn 48, ô soạn chat bo 22). Ba giá trị bản ship dùng ngoài
`tokens.json`: **16** cho con dấu CTA cỡ `lon`, **6** cho con dấu trạng thái
(`Stamp`), **4** cho mép ảnh bên trong khung in, và **28** cho góc dưới
`CoverBand`.

Hình dạng đặc trưng của thế giới, mỗi cái mang một nghĩa:
- **Con dấu CTA** (`StampButton`): **rộng bằng chữ** (`alignSelf: center`),
  không kéo theo cột; vành là **một** đường SVG `duongVienDau(w, h, r, amp =
  1.3)`: chữ nhật bo góc mà mỗi điểm biên đẩy vào/ra tới 1.3dp theo hàm cố
  định của chỉ số và bề rộng, nên cùng một con dấu in giống nhau mỗi lần
  render và hai con dấu khác bề rộng gãy khác nhau. Nét 2 mực 0.88, tô
  `brand.coral`, hạt mực clip lùi 3. **Không có vành trong** (vòng review
  2026-09-06 đọc vành thứ hai thành ba vòng quanh chữ), **không mũi tên**
  (con dấu là động từ). Nghiêng -3 trên bìa, -1 trong form, 0 trong bảng.
  Khung đầu tiên trước `onLayout` tô coral phẳng, không bao giờ là lỗ.
- **Con dấu trạng thái** (`Stamp`): bo 6, viền 2 màu tông, chữ in hoa
  condensed, nghiêng ±2/±3 khi «đóng tay» (con dấu nhịp kèo «Còn 3 ngày»
  -2, «HỢP GU» trên ảnh -2), 0 trong bảng. Đặt lên ảnh thì có **miếng giấy**
  `card` dưới con dấu viền (`nen`), để mực không đọc trên ảnh (ảnh
  `dot8/04-explore`: «HỢP GU» trên ảnh dẫn).
- **Ảnh chặng** (`HangChang anh`): ảnh 44×44 trong khung giấy 2dp (`card` +
  kẻ tóc `line`, bo 10, ảnh bo 8; 48 tổng) ở khe phải của chặng có địa điểm
  **có ảnh**; chặng khác vẫn là một dòng gọn. Chênh lệch đó là nhịp của dòng
  thời gian (ảnh `dot8/07-plan`: 12:30 «Bánh căn Lệ» có ảnh, 07:00/11:00
  không). `AnhChang` không còn export (vòng 2, 08/09): ảnh vào chặng qua
  `anh: { anh: AnhCoGhiCong, alt, loai? }`, chặng tự in dòng ghi công
  (`caption inkFaint`, không trần số dòng) làm dòng cuối; ảnh hỏng thì cùng
  khung 44 vẽ hình gu theo `loai` và chặng in «Chưa tải được ảnh» (`caption
  warn`).
- **Nét chì đứt** (`HangChang phac`): đường 2dp `borderStyle: "dashed"` màu
  `inkFaint`, nút tròn viền `inkFaint`; là bản nháp/đề xuất AI chưa ai chốt.
  Ảnh `09-itinerary`: toàn bộ lịch trình AI là nét chì.
- **Đường mực liên tục**: trong lịch trình, trục 14 rộng, đường 2dp
  `lineStrong` liền giữa các chặng, nút 12 bo 6 viền 2 `ink` (đã tới: tô
  `split`, viền `split`); trên bìa, `RouteLine` đường cong S nét 3 đầu tròn,
  chặng vòng tròn nét 2.5 r 16 có glyph, chặng đang ở tô coral r 21 nét 3.
- **Bản in** (`KhungAnh`): giấy `card`, viền tóc `line`, đệm 8 ba cạnh và
  **14 ở đáy** (lề dưới dày của ảnh in), ảnh bo 4 trên nền `line` khi chưa
  tải, dòng xuất xứ caption `inkFaint` một dòng; nghiêng ±1/±2 khi đặt tay,
  0 trong danh sách.
- **Washi**: dải SVG mép xé hai đầu (`duongWashiXeMep`), vạch sáng 1.5px
  `card` 0.35 chạy dọc, `fillOpacity` 0.9, nghiêng ±1/±2°.
- **Mép bìa**: `CoverBand` bo hai góc dưới 28, tràn lề.
- **Kẻ**: mọi divider là `StyleSheet.hairlineWidth` màu `line` (dưới hàng
  địa điểm, dòng sổ, hàng bình chọn, cạnh trên thanh tab, cạnh phải rail,
  trên/dưới ghi chú AI). Không có viền trái màu dày hơn 1px.

Mọi góc nghiêng chỉ spread khi khác 0 (`transform: undefined` làm Reanimated
crash); mọi đường SVG có test parse theo cách Java parse (`react-native-svg`
ném lúc mount nếu chuỗi `d` hỏng).

## Components

Kit nằm ở `src/rudi/ui.tsx` (`RudiScreen`, `TopBar`, `Heading`,
`SectionHeader`, `ListRow`, `Chip`, `Segmented`, `Field`, `SearchField`,
`OtpBoxes`, `Photo`, `ResponsiveRow`, `AiNote`, `IconButton`, `RudiButton`,
`DemoBadge`, `Avatar`) và `src/rudi/ui/*.tsx`, **một file một primitive**
(`StampButton`, `Stamp`, `CoverButton`, `KhungAnh`, `DongTien`, `Money`,
`MediaSlot`, `Sheet`, `Skeleton*`, `EmptyState`, `ErrorState`, `RosterPicker`,
`ReorderList`, `CoverBand`, `Washi`, `RouteLine`, `Wordmark`, `Grain`,
`PressScale`, `PhotoViewer`). Hai hàng feature dùng lại nhiều nơi:
`screens/explore/HangDiaDiem.tsx` (`PlaceLead`, `PlaceRow`) và
`screens/keo/HangChang.tsx` (`HangChang`; ảnh chặng chỉ qua prop `anh`,
`AnhChang` không export). Số màn gọi (đếm `grep`
trong `screens/` ở `d2c51977`): `Stamp` 12 (3 truyền `dong`, 6 truyền `nen`),
`Heading` 25, `ListRow` 5, `AiNote` 4, `HangChang` 4, `Sheet` 4 (thêm khay
tạo), `KhungAnh` 3, `HangChang anh` 2 (Outing, Group), `DongTien` 2, `StampButton` 2, `CoverBand` 2,
`Washi` 1, `RudiScreen header` 1.

### Khi nào dùng cái gì
| Muốn nói | Dùng | Không dùng |
|---|---|---|
| Lời rủ / hành động chính của màn thuyết phục | `StampButton` (`lon` bìa, `vua` form) | pill cam, nút kéo hết cột |
| Hành động chính trên trang giấy | `RudiButton solid` tông của màn | con dấu (con dấu là lời rủ, không phải mọi nút) |
| Lựa chọn phụ dưới con dấu trên bìa | `CoverButton variant="link"` | outline kéo hết cột (lấn con dấu) |
| Một trạng thái **đã đúng** | `Stamp` có chữ | chip màu không chữ, chữ inline đổi màu |
| Bản nháp / đề xuất AI chưa chốt | `HangChang phac` (nét chì), nhãn «Nháp» | con dấu (chưa đúng thì chưa đóng dấu) |
| Một khoản tiền trong danh sách | `DongTien` + `Money` | thẻ số to, `Stat`, thanh tiến độ |
| Ảnh dẫn / ảnh có xuất xứ | `KhungAnh` (album, tường) hoặc `MediaSlot` có `attribution` (spread `khungAnh(anh)`) | ảnh tràn không nguồn; đọc trần `anh.source` |
| Ô ảnh nhỏ trong lưới | `Photo` trần bo 10 | khung in cho từng ô |
| Thứ máy sinh ra | `AiNote` (ghi chú lề) hay tờ `ToGiay` ký ở chân | nhãn «AI» trên đầu, tô cả khối tím |
| Danh sách nhiều mục | hàng + kẻ tóc (`ListRow`, `PlaceRow`) | thẻ mỗi mục |
| Sheet trên nội dung cuộn | `RudiScreen overlay={<Sheet/>}` | sheet trong hộp cuộn |
| Trạng thái vừa thành đúng **dưới ngón tay** | `Stamp dong` cho đúng hàng vừa bấm | hoạt hình khi mount, confetti, toast |
| Con dấu đặt lên ảnh | `Stamp nen` (miếng giấy dưới) | con dấu viền trơn trên ảnh |
| Chặng có địa điểm có ảnh | `HangChang anh={{ anh, alt, loai }}` (chặng tự in ghi công) | thẻ ảnh cho mọi chặng; `Image` tự đặt vào khe `phai` |
| Phản hồi bấm | `PressScale` (lò xo scale) | mờ `opacity` khi `pressed` |

### Buttons
- **Con dấu CTA (`StampButton`)**, nút chính của bề mặt thuyết phục: tô
  `brand.coral`, vành 2px mực 0.88 theo `duongVienDau`, `Grain mucIn` 0.26,
  nhãn Bricolage Bold màu `mauSang.ink` +0.4; cỡ `lon` cao tối thiểu 60, đệm
  ngang 30, chữ 21/26, bo 16 (Welcome «Rủ Đi thôi!» nghiêng -3); cỡ `vua` cao
  52, đệm 24, chữ 17/22, bo 14 (Login «Gửi mã»). Không icon, không vành
  trong, không bóng, rộng bằng chữ. Nhấn co 0.97 kèm haptic `impact`;
  `loading` thay bằng spinner mực trong hàng và khoá nút; `disabled` mờ 0.55.
- **Nút bìa (`CoverButton`)**: `outline` trong suốt, viền 1px
  `coverLineStrong` (5.19:1 sáng / 6.42:1 tối trên `cover`), bo 14, cao 50,
  đệm 18, nhãn `label` `coverInk`, icon 18, haptic `select`. `link`: không
  viền, rộng bằng chữ, cao tối thiểu 48, đệm 12; là lựa chọn phụ **dưới** con
  dấu (Welcome «Tìm hiểu thêm ›», ảnh `01-welcome`), vì một thanh outline kéo
  hết cột đã lấn át con dấu nó phải theo sau.
- **Nút giấy (`RudiButton`)**: cao 52 (48 `compact`), bo 14, nhãn `label` một
  dòng, icon 20; `solid` = nền `<tone>` phẳng và nhãn `<tone>Ink` theo
  scheme (album «Thêm khoảnh khắc» accent, lịch trình «Dùng plan này» ai);
  `outline` = nền `card` viền `lineStrong` chữ màu tông với `accent`, viền màu
  tông với `split`/`ai` («Đánh dấu đã trả» teal, «Chỉnh lịch trình» tím);
  `soft` = nền `<tone>Soft`; `ghost` trong suốt. Nhấn **co 0.98 bằng lò xo
  trên UI thread** (`PressScale`), không còn mờ 0.82; `disabled` 0.45. Hai nút chân đứng cạnh nhau trong `footer`, outline trái,
  solid phải.
- **Nút back** trên bìa: `PressScale` 48×48 co 0.92, mặt tròn là View con
  (nền `coverInk` 0.08, chevron 26); trên giấy `TopBar` dùng chevron `ink`
  trong ô 48.
- **`IconButton`**: 48×48 bo 16; `quiet` không viền không nền (nút «+» góc
  phải album, «Chọn ảnh»); có viền `lineStrong` khi đứng cạnh chip (nút lọc
  Khám phá). Nhấn co 0.94 (`PressScale`, cùng số với FAB).
- **Bốn scale bấm của kit** (đợt 8, thay mọi nhánh `pressed` mờ opacity trên
  JS thread): `RudiButton` 0.98 · `IconButton` 0.94 · `Chip` 0.96 · `ListRow`
  0.985; con dấu CTA 0.97, FAB 0.94, back 0.92 giữ nguyên. Lò xo nhấn {18,
  260, 0.6}, thả {20, 180, 0.8}; Reduce Motion đưa lò xo về tức thì.
- **Google ở Login**: chỉ hiện khi native có web client ID hợp lệ; iOS cần
  thêm iOS client ID. Điều kiện hiển thị đọc từ mã, chưa là bằng chứng OAuth.

### Chips
- **Chip bấm được**: cao 48, bo pill, icon Ionicons 18 tuỳ chọn **hoặc**
  `leading` (một `ReactNode` thế chỗ icon, thực tế là `GuGlyph` 22 để một
  phân loại được vẽ bằng một cây bút ở mọi nơi: hàng danh mục Khám phá
  fixture và live); có `leading` thì dấu check của trạng thái chọn nhường
  chỗ, chọn nói bằng nền `<tone>Soft` + viền tông + mực glyph đổi coral; chưa chọn
  nền `card` viền `lineStrong` chữ `inkSoft`; đã chọn nền `<tone>Soft` viền
  màu tông, chữ màu tông **và** dấu check (ảnh `09-itinerary`: «✓ Ngày 1»).
  Nhấn co 0.96. Bộ lọc Khám phá, chọn ngày lịch trình, «Theo ngày» ở album;
  hàng bộ lọc của AI match giờ là **một hàng cuộn ngang** như Khám phá
  (bằng chứng reviewer `dot8/05-ai-match`, không mở ở đây; đọc từ `Discovery.tsx`), không còn lưới gập.
- **Chip tĩnh** (không `onPress`): cao 30, bo 10, không role; kit còn giữ
  nhưng **đợt này trạng thái đi bằng `Stamp`**; chip tĩnh chỉ cho thẻ phân
  loại không phải trạng thái.
- **Huy hiệu demo (`DemoBadge`)**: viền `line`, chữ 10/700 `inkFaint`, icon
  `flask-outline`, bo pill; mặc định «Dữ liệu demo», trong `TopBar` rút về
  `compactLabel` («Demo», «Nháp»); **render rỗng ở chế độ live**. Trên Welcome
  nhãn là «Bản trải nghiệm».

### Con dấu trạng thái (`Stamp`)
Chữ `stamp` (CondensedBold 12 IN HOA +0.8), viền 2px màu tông, bo 6, đệm 8/4,
cao tối thiểu 26, `alignSelf: flex-start`, `accessibilityLabel` = nhãn, không
role, không press. `outline` mặc định (chữ màu tông); `ink` tô đầy với chữ
`<tone>Ink`, hiếm: một trạng thái quan trọng nhất mỗi màn. Con dấu **luôn
mang một chữ**, màu chỉ là tông của chữ đó: «ĐÃ TRẢ», «NGƯỜI THU BILL» (teal,
quyết toán), «HỢP GU» (tím, trên ảnh dẫn nghiêng -2 và trong hàng địa điểm),
«CÒN 3 NGÀY» / «HÔM NAY» / «ĐANG ĐI · CÒN 2 NGÀY» / «ĐÃ QUA» (nhịp kèo, tính
từ `starts_on`/`ends_on` máy chủ qua `keo/nhip-keo.ts`, không từ chuỗi
fixture; nghiêng -2 ở Cá nhân), «ĐÃ TỚI» (teal, nghiêng -2, điểm danh). Ship
trên 12 màn.

Hai prop đợt 8:
- **`nen`**: miếng giấy `card` dưới con dấu viền khi con dấu nằm trên ảnh
  (ảnh dẫn và chi tiết địa điểm: «HỢP GU», «GỢI Ý»); trên giấy vẫn trong
  suốt.
- **`dong`**: trạng thái này **vừa thành đúng do chính người đó bấm**. Con dấu
  *rơi xuống* trong ba nhịp, tổng dưới ngân sách `celebrate` 550: **lao** 130
  ms `Easing.in(quad)` scale 1.35 → 1, mực giữ 25 %; **chạm** 60 ms mực 25 %
  → 100 %, `haptic.success()` nổ đúng lúc chạm, không ở đầu; **lún và
  đứng** scale 1 → 0.97 → 1 trên lò xo nhấn, shared value riêng (`lun`) nên
  không pha nào đọc qua đường cong của pha khác. Reduce Motion gập cả ba
  thành con dấu tĩnh. Một cú zoom ease-out đơn đã bị reviewer đợt 8 đọc là
  «entrance mặc định của mọi thư viện». Ba nơi ship: «Đã trả» ở quyết toán
  fixture (`Bill`), «Đã tới» ở điểm danh fixture (`Outing`, chỉ khi điểm
  danh, không khi bỏ), con dấu nghĩa vụ ở đợt thu live **sau khi máy chủ xác
  nhận** (`DotThuLive`, trạng thái đọc lại, không giả định). Khung 21..32 của
  `clip-dau-tra` là bằng chứng.

### Sổ (`DongTien`) và tiền (`Money`)
Một dòng sổ: tên trái (`body`, hoặc `label` khi `dam`), dòng phụ `caption`
`inkSoft` (số ngữ cảnh: «Đã trả 587.500đ», «5 người chưa trả», «Nháp trên
máy, chưa confirm vào sổ cái»), số phải `Money` (`label` thường, `money` khi
`dam`), cao tối thiểu 52, đệm dọc 10, kẻ tóc `line` dưới, `cuoi` bỏ kẻ. Tông
chỉ đậu trên số: `split` cho phần của bạn / sẽ nhận, `warn` cho còn phải trả
> 0, `ink` cho mọi số khác. `dam` đậm **một** dòng người đọc tìm trước, không
đổi bậc cỡ. Đây là thứ thay cho hero metric ở Quyết toán và Tài chính (ảnh
`15-settlement`, `21-finance`). `Money`: `typography.money` 21/27 EB tabular;
cỡ `display`/`money`/`body`/`label`/`caption`; `countUp` chỉ khi
`domainStateValid`. Dưới mỗi sổ là một câu `caption` nói rõ số này là gì và
không phải gì.

### Ảnh: bản in (`KhungAnh`), ô ảnh (`MediaSlot`, `Photo`)
- **`KhungAnh`**: giấy `card` bo 10, viền tóc `line`, `cardShadow`, đệm
  8/8/14, ảnh con bo 4; `chuThich` là câu người viết (`body ink`), `xuatXu` là
  dòng app biết chắc («ai · ở đâu · khi nào», `caption inkFaint`, một dòng).
  Ảnh dẫn của album và bài tường; ô nhỏ trong lưới **không** khung. **Mỗi
  sự thật viết một lần** (08/09, `sua-sang-1.0/bs-18-album`): nơi chốn ở
  `TopBar` («Album Đà Lạt», không phụ đề ngày nữa), nhóm trên bản in
  (`xuatXu` «Team Đà Lạt»; live là «Ảnh của nhóm»), ngày và số ảnh trên
  tiêu đề («17 - 19/10/2026 · 4 ảnh», `note inkSoft`). Không lặp ngày ba lần
  trên một màn.
- **`MediaSlot`**: khung của ảnh **catalogue**. Không phải nơi duy nhất vỏ vẽ
  ảnh: ảnh của chính nhóm còn tới `ui.tsx Photo`, `AlbumAnh` và `PhotoViewer`
  (câu cũ ở đây nói «nơi duy nhất» là sai, review delta 08/09 F31). Khung vẽ
  trước, fallback là artwork của thế giới; nhận **một** prop `nguon:
  NguonKhung` chứ không phải `source` cạnh `attribution?` rời nhau, và in
  `caption inkFaint` tối đa hai dòng bên dưới; URL không qua
  `nguonAnh` bị từ chối trước khi tới đây. Nền khung trước/khi không có ảnh là
  `colors.card` của theme, không còn hằng beige tĩnh (`nenAnhTrong` đã xoá
  khỏi `theme.ts`; trên nền tối hằng ấy đọc thành mảng, review 08/09 vòng 2
  §4). Ảnh tải hỏng: khung giữ nguyên, in «Chưa tải được ảnh» `caption warn`
  dưới khung, ghi công vẫn in.
- **Luật Địa Chỉ Đi Cùng Câu Chữ** (08/09 vòng 2 F21, siết lại ở review delta
  F31). Câu cũ ở đây nói ba khung nhận `AnhCoGhiCong` «nên không có cách đưa
  ảnh mà bỏ ghi công»; **điều đó không đúng với API lúc ấy**: `MediaSlot` nhận
  `source` và `attribution?` **rời nhau**, và `PlaceCompare` đã đưa cho nó một
  địa chỉ trần thật. Nay bảo đảm ấy nằm trong kiểu, không nằm trong lời hứa:
  `AnhCoGhiCong` **giữ địa chỉ trong closure** và chỉ trả ra qua `ve()`, thứ
  luôn trả **cả** `source` **lẫn** `ghiCong`. Cụ thể: `p.source` và
  `const { source } = noi.anh` — đúng hai đường thoát probe của Codex đi qua
  được — nay là **lỗi biên dịch**. Đó là mức bảo đảm của kiểu, không hơn: gọi
  `ve()` rồi bỏ `ghiCong` vẫn biên dịch được, nên **chỗ ấy do máy dò gác**, và
  máy dò khoá cả ba cách viết (`x.anh.ve()`, `const f = x.anh.ve`,
  `x.anh["ve"]()`) cùng hai cách bỏ nửa quyết định. Một quyết định thuần
  `veKhung(nguon, {hong})` trả `{source, ghiCong, canhBao}` và khung chỉ render
  ba trường ấy, nên ảnh và câu chữ không còn là hai điều kiện phải tự khớp.
  Ảnh của nhóm đi nhánh `{loai:"nhom"}` và **không bịa giấy phép**.
  `tests/rudi-anh-ghi-cong.test.mjs` là **lớp hai**, cho thứ kiểu không thấy
  (`as any`, ai đó dựng lại cặp `{source, nguon}`, hay một khung vẽ ảnh mà bỏ
  nửa quyết định): quét AST **mọi `.ts` và `.tsx`**, **không bỏ qua file nào**,
  và tự kiểm bằng chính hai mẫu thoát ấy cùng ca «vẽ `ve.source` mà không in
  `ve.ghiCong`». Kiểm chuỗi `cauGhiCong(` có mặt trong file (đọc cả comment)
  đã bị bỏ; thay bằng bất biến trên `veKhung` kèm ca đột biến. Bình chọn **không có ảnh**
  (lead chọn 08/09): ba lựa chọn đều là ô vẽ `GuGlyph` 30 trên `card` viền
  hairline `line`, vì ô 56 không có chỗ cho câu ghi công và một phiếu bầu
  không được để một lựa chọn nổi hơn chỉ vì catalogue tình cờ có ảnh stock.
- **`Photo`**: `ratio` hoặc `height` (mặc định 190), bo mặc định 20, nền
  `colors.card` dưới ảnh (không hằng beige), `overlay`
  cho con dấu trên ảnh; `PhotoShade` gradient `lopPhu.xam(0.78)` từ 0.3 xuống
  đáy khi có chữ trên ảnh. Ô album 104 tối thiểu, gap 6, tối đa 6 cột.
- **Số ảnh thật**: «4 ảnh» đếm từ mảng ảnh, không từ chuỗi.
- **Album theo ngày** (chỉ `AlbumLive.tsx`, đọc từ mã, chưa có ảnh native):
  sau ảnh dẫn, `nhomTheoNgay` gom ảnh theo **ngày lịch Việt Nam (+07:00)**
  tính trên epoch (Hermes không hứa `Intl`); mỗi ngày một nhãn `note
  inkFaint` «17/10» (chỉ khi có hơn một ngày; tem không parse được gom dưới
  «Chưa rõ ngày», không bị rơi), ảnh **theo cặp** vuông (`ResponsiveRow
  maxColumns 4 minItemWidth 150 gap 6`), **ảnh lẻ cuối ngày đi ngang** ở tỉ
  lệ ảnh dẫn để ngày kết bằng một nhịp, không bằng lỗ hổng. Album fixture
  (`Memories.tsx`) vẫn là lưới đều ba cột.

### Lớp vẽ (`art/`): Nếp, hình gu, motif, cảnh rỗng
Đợt 08/09 thêm một lớp minh hoạ **vector thuần**, tách hình học khỏi màu và
khỏi React: `src/rudi/art/{net,nep,motif,gu,canh}.ts` chỉ trả mảng `LopVe`
(`d`, vai màu `mau`, `net` > 0 là nét, không có là tô); `src/rudi/ui/art/`
(`VeLop`, `Nep`, `GuGlyph`, `VongHo`/`DuongChuyen`/`GocGap`, `Canh`) vẽ mảng
đó bằng `react-native-svg`, nét tròn đầu tròn góc, tô phẳng, không bóng.
- **Ngữ pháp đường**: mọi `d` chỉ gồm lệnh tuyệt đối `M`/`L`/`C`/`Z`, số thập
  phân trơn (không mũ, không `-0`), dựng từ số lúc chạy qua `net.ts`
  (`daGiac`, `netGay`, `cong`, `qCong`, `tron`, `bau`, `cungTron`, `quat`,
  `khungBo`, `vien`, `thon`, `giot`). Bậc hai và cung tròn được đổi ra bậc ba
  ở builder, không bao giờ phát `Q`/`A`. Lý do là hai lỗi thật: Java
  `PathParser` ném lúc mount và app chết khung hình đầu (tsc, web export mù),
  và repo guard đọc chín chữ số cách nhau như số tài khoản.
  `tests/art-duong.test.mjs` parse từng hình đúng cách Java parse.
- **Ba lưới**: Nếp trong ô **96** (`KHUNG_NEP`), hình gu trong ô **48**
  (`KHUNG_GU`), cảnh trong khung ngang **144×112** (`KHUNG_CANH`). Ô đặt qua
  `bienDoi(x0, y0, tiLe)`; chưa có lưới 24 nào dùng ngoài bản 22/32 của
  `GuGlyph` co từ 48.
- **Nếp** (`hinhNep`, chín tư thế `moi` · `keo-ghe` · `giu-cho` · `gop-y` ·
  `cam-ban-do` · `giu-khung` · `doi` · `ghi-lai` · `vui`): tờ hẹn gấp, thân
  giấy hơi rộng chân, một nếp chéo
  (`bong`) như hai ve áo, **một** góc coral gấp xuống trên phải, mắt mực
  dưới **một** mày lệch, nụ cười nghiêng khép, chân thon và tay bao (`vien`)
  luôn đang làm gì đó. **Hai bản đọc**: 96 có mày, nếp, đạo cụ; dưới 72dp
  (`Nep size < 72`) vẽ **bản 48** dày nét bỏ chi tiết, không co bản 96.
  Trang trí, ẩn khỏi cây trợ năng; chữ bên cạnh mới nói.
- **Tư thế là cả người, không phải chỉ tay** (review 08/09, gói R2). Mỗi
  pose khai trong `TU_THE` hai con số: `nghieng` là quãng thân trên trượt
  ngang khi bàn chân giữ nguyên trên đường sàn `CHAN_NEP = 91` (một phép
  trượt tuyến tính theo chiều cao, không xoay), `nhin` là quãng dời của hai
  con mắt về phía vật đang làm, chặn trong ±1.6 ô. Bốn pose diễn có **điểm
  tay chạm vật** ghi thẳng trong chú thích cảnh: `keo-ghe` nắm đỉnh cọc lưng
  ghế ở ô (90, 34) và ngả người ra sau; `giu-cho` đặt tay lên thanh ghế bên
  cạnh ở (88, 48); `cam-ban-do` và `giu-khung` đặt **hai tay lên cùng một
  cạnh** vật (ô x 74..78), tay gần gập khuỷu ở (38, 72) nên cánh tay không
  xuyên qua thân. Đổi cảnh thì đổi cả hai đầu: toạ độ vật trong `canh.ts` và
  toạ độ tay trong `nep.ts` phải gặp nhau, không chỉnh một bên.
- **Hình gu** (`hinhGu`, tám id của máy chủ `an-uong` · `cafe` · `nightlife`
  · `mon-local` · `outdoor` · `shopping` · `karaoke` · `game`): vật trên bàn
  vẽ **một cây bút** (nét chính 2.4 trên lưới 48, nét mảnh 1.7), **tối đa
  một chi tiết coral** mỗi hình (giọt cà phê, bóng đèn sáng, thẻ menu, điểm
  đứng); id lạ vẽ **thẻ gấp**, không rỗng, không ném. `GuGlyph` 40 trên ô Sở
  thích, 22 trong chip Khám phá, 32 trong thumbnail hàng, ×1.15 trong đĩa
  `PlaceGlyph`; `tone="accent"` đổi mực sang coral cho ô/chip đã chọn.
- **Tư thế là bốn trục, không phải hai cánh tay** (09/09). Tới trước hôm ấy
  `hinhNep` chỉ đổi TAY: thân, mặt và chân giống hệt nhau ở cả chín pose, nên
  năm cảnh là một dáng người đổi đạo cụ và không sticker nào có hướng di
  chuyển. Nay mỗi pose khai `{nghieng, nhin, bieuCam, dang}`:
  **`BIEU_CAM`** (`binh-than` · `hao-hung` · `hoi` · `quyet` · `met` ·
  `nhuong`) đổi **mày và miệng**; mắt vẫn là chấm và vẫn theo `nhin`, vì mắt
  to má hồng là register concept note đã loại. **`DANG`** (`dung` · `buoc` ·
  `nhun` · `ngoi` · `chong`) đổi **chân**; `dung` là bản cũ nguyên vẹn nên mọi
  pose và cảnh đang có giữ nguyên hình. `dam` nhân **độ dày nét và bề dày chi**
  mà không đụng hình học, dành cho hình phải vẽ nhỏ trong khung của nó. Bản rút
  gọn vẫn bỏ mày, nên luật «rút gọn ít lớp hơn» giữ ở mọi biểu cảm.
- **Nếp có MỘT mày, và không nét mực nào SƠN lên góc coral** (09/09). Góc gấp
  (H 50,20 · G 69,38 · Bp 50,38) chiếm trọn phần trên phải của mặt, nên mày
  phải không có chỗ ở: vẽ đúng chỗ của một cái mày thì nó kẻ **vạch đen ngang
  dấu nhận diện**, còn hạ xuống dưới đường viền của chính nếp gấp thì nó dính
  vào mắt phải thành một khối tối. Hai cách đều đã dựng ra ảnh và so cạnh nhau
  trước khi chốt. Nay **nếp gấp che chỗ ấy** — đúng việc một tờ giấy gấp làm —
  và «mày lệch» của concept thành nghĩa đen: mày trái gánh toàn bộ biên độ,
  miệng gánh phần còn lại. Hai biểu cảm từng trùng nhau nay tách: `met` vẽ
  **nhầm chiều** (đầu trong chúc xuống, tức dáng quyết tâm) nên không phân biệt
  được với `quyet`, và `nhuong` lệch `binh-than` chưa tới một đơn vị nên cũng là
  cùng một khuôn mặt; nay `met` hếch đầu trong, `nhuong` là **cung cong xuống**,
  ảnh gương của `hao-hung`. Pose `nang-bong` **đã gỡ**: nó nâng một vật ngang
  đầu đúng chỗ góc gấp, không có vị trí tay nào vừa giữ được ý pose vừa tránh
  được coral trên cả dải nghiêng, và chưa màn nào dùng nó.
- **Cổng `khongCatNepGap`** (`tests/art-duong.test.mjs`) là cơ chế giữ luật
  trên, không phải lời hứa. Bốn điều nó phải làm đúng — và bản nháp đầu làm sai
  cả bốn, lượt chấm context mới bắt được:
  1. **Đo SƠN, không đo tâm nét.** Một nét dày 2.4 có tâm nằm ngoài 0.4 vẫn phủ
     1.6 đơn vị mực lên coral. Luật là `sâu + net/2 <= 0`.
  2. **Chặn trên từng đường cong, không chấm điểm.** Lấy N+1 mẫu để lại sai số
     dây cung tối đa `max|B''|/(8N²)`, tính được từ chính điểm điều khiển; cộng
     nó vào là thành chặn đúng nghĩa. Bao lồi cũng đúng nhưng quá rộng: bao của
     một hình tròn vượt bán kính ~14%, tức là vu oan mọi bàn tay và cả hai mắt.
  3. **Nhận DIỆN nếp gấp, không đoán.** «Mảng coral đầu tiên» không phải phép
     nhận diện — ngòi bút chì của `ghi-lai` cũng là tam giác coral ba đỉnh mà
     mực chạm vào là đúng. Nếp gấp nhận theo **hình dạng bất biến với phép đặt**
     (hai đỉnh cùng một đường ngang, đáy : cao = 19 : 18), vì cảnh và sticker
     đều truyền `x0`·`y0`·`tiLe` nên toạ độ 20/38 không còn.
  4. **Chạy trên thứ thật sự lên màn.** `hinhNep` đứng một mình không phải cái
     màn hình vẽ; cổng quét cả tám sticker (hai bản đọc) lẫn mười cảnh, và bỏ
     qua lớp nằm **trước** nếp gấp vì chúng bị chính mảng coral phủ lên.
  Phạm vi quét là `nghieng ∈ [−8, 13]` × `dam ∈ {1, 1.3}` chứ không chỉ giá trị
  mặc định của pose, vì cả hai là **override công khai** và sticker «Chờ tí»
  đang dùng cả hai. Sàn số điểm đọc từ số đo thật (8.577.360 điểm / 9.504 bản
  vẽ), không lấy tròn. **15 đột biến trên chính cổng đều đỏ**, gồm ba cái từng
  sống sót: bỏ bề dày nét, bỏ qua mọi lớp nét, và miễn trừ nhầm mọi lớp tô.
- **Chỗ hẹp nhất hiện nay là MẮT PHẢI, không phải mày hay tay.** Đo trên toàn
  bộ tám sticker: «Chờ tí» bản chi tiết còn cách mép nếp gấp **0,29 đơn vị**
  (bản rút gọn 0,58), rồi mới tới «Tuyệt vời» 0,95. Đó là biên sẽ vỡ trước, và
  nó đang là **hệ quả của cái chặn ±1.6 của `nhin`** chứ không phải một luật có
  tên. Ai dời mắt, đổi `nhin`, hay tăng `dam` cho một pose nghiêng nhiều thì
  nhìn số này trước — cổng sẽ đỏ, nhưng biết trước thì đỡ mất một vòng.
- **Cái cổng hình học không đo được thì ghim bằng ca riêng.** «Mày này đọc ra
  mệt hay đọc ra cáu» không phải chuyện hình học, nên vẽ `met` ngược chiều lại
  **không** làm cổng nếp gấp đỏ. Quyết định thiết kế được ghim thẳng: **sáu biểu
  cảm cho sáu đường mày khác nhau**, và **`met` dốc ngược `quyet`** — đó là cái
  bị vi phạm, chứ không phải một toạ độ cụ thể.
- **Khay sticker** (`KhaySticker`): nhãn **hai dòng** với chiều cao dành sẵn
  nên tám ô bằng nhau, và số cột tụt theo `fontScale` (4 → 3 → 2). «Cà phê
  không?» từng bị cắt thành «Cà phê khôn…»; từ vựng khoá ba nơi nên **layout
  nhường, không phải chữ**. Bề rộng viết literal chứ không ghép chuỗi, vì cổng
  `receipt.test.mjs` đọc mọi «…%» một build sinh ra (ADR-0009).
- **Ba motif** (`motif.ts`): **vòng hở** (`vongHo`, một nét coral, khe hở
  hơn 60°, mặc định mở trên phải) là cái bàn còn trống một bên, **chỉ trang
  trí**; **đường chuyền** (`duongChuyen`, đường S của kit với chấm coral ở
  điểm đặt bút) nối những thứ thuộc về nhau; **góc gấp** (`gocGap`, tờ giấy
  gấp góc trên phải cùng góc với Nếp, tỉ lệ 0.28). Ghế `hinhGhe` là đạo cụ
  chung của cảnh.
- **Một mặt sàn cho cả cảnh**: `SAN = 102` trong `canh.ts`. Chân ghế, chân
  khung ảnh, chân bản đồ và bàn chân Nếp (`nepTrenSan` đặt `y0 = SAN −
  CHAN_NEP × tiLe`) cùng kết thúc ở đó, nên không nhân vật nào lơ lửng cạnh
  đồ vật. Cảnh `chua-co-keo` là ngoại lệ có chủ ý: tờ hẹn bay, nhân vật viết
  bên cạnh, không có sàn nào để đứng.
- **Mười cảnh** (`hinhCanh`, `CANH_IDS`, 09/09). Trước đó có năm, và ba trong
  số đó dùng **cùng một dáng người đổi đạo cụ** — chính điều review cấm nhân
  lên. Lượt đầu của bản mười cảnh **tái phạm đúng lỗi ấy** ở ba cảnh mới (cùng
  pose `ghi-lai`/`giu-khung`/`cam-ban-do` với ba cảnh cũ, chỉ đổi hình chữ
  nhật); lượt chấm bắt được và ba cảnh ấy được **vẽ lại bằng pose mới**
  (`nang-bong`, `voi-len`, `ghe-nhin`). Nay mỗi cảnh có **một tình huống, một
  dáng và một khuôn mặt riêng**:
  `chua-co-hoi` kéo ghế (mặt `nhuong`) · `chua-co-keo` cúi viết (`quyet`) ·
  `chua-co-anh` nhìn xuyên khung rỗng (`hoi`) · `chua-co-ban` **ngồi** ở bàn
  hai chỗ, tay mời sang ghế trống (`nhuong`, `ngoi`) · `tim-khong-ra` dò bản đồ
  (`hoi`) · `chua-co-tin-nhan` · `chua-co-loi-moi` · `chua-co-ky-niem` ·
  `bo-loc-che-het` · và `chua-doc-duoc`.
- **`CANH_KHONG_NEP` là cơ chế, không phải lời hứa.** `chua-doc-duoc` **không
  bao giờ** vẽ Nếp, kể cả khi người gọi truyền `nep`: `hinhCanh` bỏ qua yêu
  cầu ấy. Lý do là luật «Nếp Đứng Xa Tiền» phải đúng ở **khoảng hai mươi** màn
  lỗi, trong đó có màn sổ, và một luật do hai mươi nơi tự nhớ là một luật sẽ
  hỏng. `ErrorState` gắn sẵn cảnh này nên mọi màn lỗi có hình mà không nơi nào
  phải nhớ gì. Cổng `art-duong` biết tập ấy và đòi bản có/không Nếp **giống hệt
  nhau** cho các id trong đó.
- **Im lặng hành chính cố ý để trống**: «Bạn chưa chặn ai», «Chưa có phiên
  nào», và dải «Thông báo» ở Khám phá. Không phải quên: hai cái đầu là danh
  sách quản trị rỗng, không cần ai kể chuyện, và review đã cấm đưa hình kể
  chuyện vào mọi hàng dữ liệu; cái thứ ba là **ghi chú kỹ thuật của bản trải
  nghiệm** («chưa có hộp thư máy chủ»), tức lời của hệ thống nói về chính nó,
  không phải im lặng của người dùng. Ba chỗ này là **toàn bộ** các ô rỗng không
  có hình; kiểm lại bằng cách quét `illustration=` trên mọi chỗ mount
  `EmptyState`, đừng tin danh sách này tự biết mình thiếu.
- **Mỗi cảnh trọn vẹn khi không có
  Nếp** (`nep: false`, so `sua-ab/A-co-nep` với `B-khong-nep`) và được trình
  đọc màn hình đọc thành **một câu**: `chua-co-hoi` «Một chiếc ghế được kéo
  ra, chừa sẵn chỗ» · `chua-co-keo` «Một tờ hẹn trống, nét mực bắt đầu từ
  đó» · `chua-co-anh` «Một khung ảnh còn trống, góc giấy gấp» · `chua-co-ban`
  «Hai chiếc ghế, một chỗ còn trống» · `tim-khong-ra` «Một tấm bản đồ gấp,
  đường đi chưa tới nơi». `Canh` khung chặt theo `hopNgang` + `viewBox`, nên
  bỏ Nếp thì cảnh không để lại khoảng thụt bên trái; `width` là bề rộng
  khung 144 đầy đủ để đạo cụ giữ một cỡ có hay không có nhân vật.
- **Quyết định mở rộng nhận diện (08/09, sau review đợt 1).** Nếp là **một
  lớp tháo được**, không phải nhân vật bắt buộc: mọi cảnh phải đọc được với
  `nep={false}`, và cổng A/B (`rudi://dev/ui-lab`, mục «Cảnh rỗng») dựng hai
  bản cạnh nhau trên cùng máy cùng dữ liệu để quyết định bằng ảnh (bàn thử
  còn mục «Chat · khay sticker và bong bóng»: `KhaySticker` thật qua khe
  `RudiScreen overlay`, bubble `Sticker` 120 hai hàng trái/phải). Giới hạn
  đi kèm: nhân vật **không tự xuất hiện** ở màn có dữ liệu thật của nhóm (ảnh
  nhóm, ledger, hội thoại), **không** vào thanh điều hướng hay biểu tượng app,
  **không** thay `Stamp`/`GuGlyph` trong vai trò thông tin. Muốn đưa Nếp ra
  ngoài trạng thái rỗng và cửa vào thì mở quyết định mới, đừng suy từ mục này.
- **Ngoại lệ sticker (đã bàn ở review đợt 1 dòng 125/132, chốt lại ở vòng 2
  dòng 81).** Sticker trong chat là **phát ngôn do người gửi chọn**, không phải
  mascot hệ thống, nên tám sticker của ADR-0021 được vẽ bằng ngôn ngữ Nếp mà
  không vi phạm câu trên: Nếp không *tự* bước vào hội thoại, một người *gửi*
  Nếp vào đó. Ranh giới: giữ đúng tám ID và nhãn; `tra-tien-ne` là lời người
  gửi, **không bao giờ** là trạng thái giao dịch hay dấu xác nhận tiền của hệ
  thống; Nếp-hệ-thống vẫn không đứng cạnh ledger, lỗi, conflict hay xác nhận
  tiền. Muốn cấm cả sticker thì trình Lead, không vừa ghi cấm vừa vẽ tám mẫu.
  Sticker đầu tiên theo ngoại lệ này đã có: `cho-ti` (mục Chat bên dưới);
  bằng chứng khay/bubble sáng-tối ở `docs/claude/2026-09-08/tra-loi-sticker-cho-ti.md`.
- **Luật Nếp Đứng Xa Tiền.** Nếp chỉ xuất hiện ở trạng thái rỗng và cửa vào;
  **không bao giờ** cạnh số tiền, lỗi, hay xung đột (báo cáo 07/09 §6.4).
  Không dấu chuyển động, không mặt hào hứng trên mọi tư thế: tay giơ đã nói.
- **Luật Vòng Hở Không Tiến Độ.** Vòng hở không bao giờ là progress ring:
  không animate, không đi cùng phần trăm, khe luôn rộng. Đường chuyền khi
  mang nghĩa tiến độ phải có chữ đi kèm.

### Hàng địa điểm (`PlaceLead`, `PlaceCompare`, `PlaceRow`, `PlaceGlyph`)
Một từ vựng `DiaDiemHienThi` cho catalogue fixture và màn live; đợt 08/09
thêm `loai` (id danh mục, để khung trống vẽ hình gu) và `lyDo` (một lý do có
căn cứ); vòng 2 (08/09) bỏ cặp `photo` + `attribution` rời nhau, thay bằng
**một** trường `anh: AnhCoGhiCong | null`, nên ảnh và ghi công đi cùng nhau
ở tầng kiểu và cả ba khung tự in `cauGhiCong(anh.nguon)`. Nhịp kết quả sau `taiSoSanh`: **một ảnh dẫn** (chỉ khi có ảnh) →
**một cặp so sánh** (khi còn ≥ 2) → **các hàng** (`sua2-sang-1.0/bs-04-kham-pha*`).
- **`PlaceLead`**: ảnh 16:10 compact / 21:9 rộng, bo 20, `Stamp` tím nghiêng
  -2 ở góc trên trái khi có `badge`; dưới ảnh tên `h2`, **một dòng lý do**
  `label` màu `ai` ngay dưới tên («Hợp gu nhờ Chill và View đẹp») chỉ khi
  match là thật và máy chủ gửi `reason`, không bao giờ là tagline hoá trang
  làm lý do; rồi mô tả `body inkSoft`, ba sự thật với icon 16 (`label
  inkSoft`); nút lưu `IconButton` phải. Ảnh là `MediaSlot` nhận
  `attribution={anh.nguon}` nên ghi công in ngay dưới khung; `anh === null`
  thì không khung 16:10 mà là đầu bài gọn (`PlaceGlyph` 34 + tên + sự thật).
- **`PlaceCompare`**: hai ứng viên **trên một trục**, không thẻ quanh ô nào:
  hàng `gap` 16, đệm dưới 12, kẻ tóc `line` dưới; mỗi ô `flex 1` gồm
  `MediaSlot` **4:3** với trái tim `IconButton` ở góc dưới phải **trên ảnh**
  (như ảnh dẫn, không hàng mồ côi dưới sự thật) và `Stamp` tím `nen` khi có
  badge; tên `title` hai dòng, mô tả `note inkSoft` hai dòng, các sự thật
  đầu nối « · » trên một dòng `note inkFaint`, **sự thật cuối (giá kèm đơn
  vị) đứng riêng một dòng** để ô nửa màn không bẻ «80K/người»; ghi công
  `note inkFaint` khi ảnh có giấy phép. Thuần: cùng hàm chia cho fixture và
  live.
- **`PlaceRow`**: thumbnail 56 bo 10 (`accentSoft` khi trống, bên trong
  `GuGlyph` 32 coral khi có `loai`, Ionicons 24 khi không), **con dấu đứng
  cạnh tên** trên cùng hàng (`rowTen`, tên rút về một dòng khi có dấu) để
  hàng có match cao bằng hàng thường; mô tả `caption inkSoft` một dòng, sự
  thật `caption inkFaint` một dòng, ghi công `caption inkFaint` tối đa hai
  dòng (`cauGhiCong`); nút tim phải; đệm dọc 10, gap 8, kẻ tóc dưới. Hàng
  nằm trên giấy, **không thẻ**; ở tablet hai cột. Ảnh hỏng (`onError`): ô
  56 vẽ lại hình gu và hàng in «Chưa tải được ảnh» `caption warn` dưới sự
  thật, ghi công vẫn in (ảnh `sua-review-2/native-kham-pha-anh-hong-*`).
- **`PlaceGlyph`**: đĩa `accentSoft` đường kính 1.7 × size, bên trong hình
  gu của danh mục (`guTheoLoai`: `quan-an-local → an-uong`, `cafe`, `vui-choi
  → game`, `di-choi-dem → nightlife`, còn lại → thẻ gấp) tô coral; Ionicons
  chỉ còn là fallback khi caller không truyền `loai`. Khung trống **không
  bao giờ** là ảnh stock. Đĩa **giữ** `accentSoft` cả trên nền tối (quyết
  định bằng ảnh, 08/09 vòng 2): đứng trên `card` nó đọc là tint ấm cùng họ
  accent; `ground` trên `card` tối gần như không thấy đĩa. Không thêm token.

### Chat: sticker, trích dẫn, tin đã xoá, theme bong bóng (M15 L1–L2)
- **Sticker** là hình vector từ từ vựng đóng (`chat/sticker.ts`, 8 hình, cùng
  danh sách với `packages/shared/stickers.json` và máy chủ, test ba chiều;
  bảng `HINH` giữ đúng tám key `"id": [`), vẽ bằng `ui/stickers/Sticker` cỡ
  120 trong hàng, không nền không viền; giữ lâu mở cùng `MenuTin` như bong
  bóng chữ. Không GIF, không ảnh raster. Một lớp (`LopSticker`) là mảng tô
  hoặc **nét** (`net` > 0, stroke bo tròn đầu và góc); vai màu `MauSticker`
  có `line` (sắc giấy) bên cạnh `accent`/`ink`/`split`/`card`/`coral`. **Hai
  cỡ đọc**: `hinhSticker(id, { chiTiet })` trả bản rút gọn (`HINH_RUT_GON`)
  khi có; `Sticker.tsx` chọn `chiTiet = size >= 72` như `Nep.tsx` (khay
  `KhaySticker` vẽ ô 64, bubble vẽ 120) — bản nhỏ là hình vẽ thứ hai, không
  phải bản 120 thu lại. Hình đầu tiên vẽ bằng ngôn ngữ Nếp là `cho-ti`
  («Chờ tí», 08/09): Nếp giữ một ghế, tay nắm đầu trụ lưng ghế,
  thân nghiêng về ghế, mắt hướng đồng hồ lớn tối giản góc trên phải (vòng,
  hai kim, chấm coral, không số); ở 64 ghế là đạo cụ chính, đồng hồ giữ cỡ,
  nét dày hơn. Nó đi qua adapter thuần `tuLopVe` đổi vai lớp vẽ
  (`giay/muc/gap/bong/split`) sang vai sticker và ném lúc nạp module nếu gặp
  vai không có màu. **Cả tám nay vẽ bằng ngôn ngữ Nếp** (09/09, sau khi review
  delta duyệt pilot): tám HÀNH ĐỘNG khác nhau, không phải một dáng đổi đồ vật —
  bước đi vẫy cờ · nâng tô hỏi · đẩy ly mời · ấn dấu chốt · giữ ghế nhìn đồng
  hồ · ngồi trên xe không nhúc nhích · hai tay đưa tiền · nhảy lên. Ngân sách
  đạo cụ mặc định là **một**: ba đồ vật đọc chậm hơn một khối. `tra-tien-ne`
  **không** có dấu tick, đồng xu hay ký hiệu tiền tệ, và là hai tờ chồng nhau
  chứ không phải tờ gấp góc (bản gấp góc đọc ra phong bì, đúng register concept
  note loại). `cho-ti` giữ nguyên bố cục đã duyệt và chỉ được **cân lại nét**
  qua `dam`, vì nó nhường nửa khung cho ghế nên đứng ở 0.726 và ra nhạt hơn bảy
  hình bên cạnh; phóng to thì ghế rơi khỏi khung.
- **Luật Một Lần Gửi Giữ Một Cái Chìa** (review delta 08/09, F32). Mỗi lần
  bấm gửi mint đúng một `Attempt`, và **hàng chờ giữ nó** (`chat/hang-cho.ts`,
  thuần): «Thử lại» gửi lại **cùng chìa ấy**, nên một yêu cầu máy chủ đã nhận
  mà client mất phản hồi được phát lại chứ không thành tin thứ hai
  (`app/api/idempotency.py`, `Replay`; `gopTin` dedupe theo id máy chủ). Chọn
  lại cùng một sticker **cố ý** là chìa mới và là tin thứ hai: hai việc khác
  nhau. Trước đó `useTinNhan` gọi `newAttempt()` **bên trong** hành động, trái
  đúng câu `api.ts` viết sẵn («mint on the press, never inside a retry»), nên
  cả sticker, chữ lẫn ảnh đều có nguy cơ ghi đôi; nay cả ba đi qua một đường.
  Trạng thái nằm **trên đúng tin**, không phải một thông báo chung: hàng đang
  đi vẽ chính sticker ấy mờ `opacity 0.62` kèm «Đang gửi...»; hàng hỏng vẽ rõ
  nét kèm câu của máy chủ và nút **«Thử lại»** viền (nhãn nhà, như
  `ErrorState`; **không** dùng «Gửi lại», chữ ấy đã có nghĩa «gửi cho người
  này lần nữa» ở đợt thu) cùng nút «Bỏ». Lỗi **vĩnh viễn** không mời thử lại:
  `sticker_unknown` (bản app không có hình), `reply_target_deleted`,
  `permission_denied`, và ba mã idempotency — `idempotency_request_in_flight`
  nói thẳng là đừng bấm nữa. Hàng chờ **không** vào `chat.tin`: `cursorMoiNhat`
  poll từ đầu danh sách ấy, nên một cursor bịa ở đầu sẽ đầu độc mọi lần poll.
  Gửi chữ và gửi ảnh dùng lại chìa **chỉ khi từng byte giống hệt** — thân, tin
  trả lời **và phụ đề ảnh**, vì cả ba đều vào thân yêu cầu — nên hàng chờ phải
  giữ đủ chúng. Cùng chìa khác thân là `422 idempotency_key_reuse`, mà bảng mã
  ở trên xếp là **vĩnh viễn**: gửi lại thiếu một trường sẽ báo người dùng rằng
  tin hỏng hẳn trong khi nó đã nằm trong nhóm. Bản nháp chữ giữ chìa trong một
  ref chứ không vào `hangCho`: ô soạn đã trả chữ về và thông báo đã nói lý do,
  một hàng nữa là cùng một tin hai lần (và một hàng vô hình từng nuốt mất màn
  rỗng của nhóm mới).
- **Luật Phản Hồi Về Đúng Nhà** (audit native 09/09, F42). Một phản hồi chỉ
  được ghi vào **cuộc hội thoại đã sinh ra nó**. `useTinNhan` đánh số **thế hệ**
  (`theHeRef`): mọi đường async chụp số ấy trước `await` đầu và so lại sau
  **mỗi** `await`, ở cả nhánh thành công lẫn `catch`; lệch là **bỏ trọn** — không
  ghép tin, không đặt lỗi, không poll, không ném. Số chỉ tăng ở **một** chỗ: cleanup
  của effect `[contextId, personId]`, chạy trước lượt đọc đầu của cuộc hội thoại
  kế và cả khi unmount, nên hai cửa là một. Vì sao không đủ nếu chỉ reset mảng:
  `navigate()` cùng route khác param **cập nhật tại chỗ** instance, và một
  sticker gửi ở nhóm A hạ cánh sau khi đã sang B từng được ghép vào danh sách B
  rồi `napMoi` **đóng trên A** đọc A bằng cursor của B (probe của Codex: lượt
  đọc `[A, B, A]`). Hai lớp vì hai loại state: route `groups/[id]/chat` **key
  theo id** để remount làm sạch state của màn (thông báo, chữ đang soạn, tin đang
  trả lời, khay đang mở); thế hệ lo thứ remount không với tới — lượt GET mà một
  phản hồi muộn còn kéo theo. `chay` trả `null` cho «không phải của mình», người
  gọi không cuộn, không báo. Read-mark có thêm điều kiện `trang.tin ===
  tinRef.current`: ở commit đổi nhóm effect ấy còn thấy danh sách **cũ** dưới id
  **mới**, và không có nó thì PUT read-mark của B mang id tin của A (đột biến bỏ
  điều kiện này đỏ ở `tests/rudi-chat-useTinNhan.test.mjs`). Chi phí chấp nhận:
  hàng chờ **không** đi theo người sang nhóm khác (R2 09/09) — một tin của A hỏng
  sau khi đã sang B mất nút thử lại; quay lại A thì trang đầu đọc lại và tin đã
  hạ cánh có sẵn ở đó. Cổng là hook thật chạy dưới React thật
  (`react-test-renderer` ghim đúng phiên bản React, `fetch` trả tay), bảy ca:
  gửi muộn, trang đầu muộn, lỗi muộn, unmount, đổi người, read-mark, và F32 giữ
  nguyên chìa khi thử lại trong cùng cuộc hội thoại.
- **Hai nút cùng một hàng thì cả hai `full={false}` và hàng được wrap** (audit
  native 09/09, F41). `RudiButton` mặc định `full` — `width: "100%"`,
  `flexShrink: 0` — nên hai nút đặt cạnh nhau là hai lần trọn bề rộng: «Thử
  lại» của hàng sticker hỏng bị đẩy khỏi **mép trái màn**, còn `assertVisible`
  vẫn xanh vì node có trong cây (XML của Codex có nút mà **không có node chữ**
  con). Sửa bằng layout, không bằng chữ: nút theo nội dung, hàng
  `flexWrap: "wrap"` canh phải, nên ở chữ lớn hai nút **xuống dòng** thay vì
  thu nhãn hay giấu «Thử lại». Cổng cho lỗi này không phải «có trong cây» mà là
  **bounds**: `docs/claude/2026-09-10/native-r11/kiem-bounds.mjs` đòi mỗi nút có
  node chữ con nằm trong nút với lề ≥ 30px hai bên và nút nằm trong màn —
  `uiautomator` **kẹp** bounds về mép màn nên `x ≥ 0` không chứng minh gì, và
  chữ rơi khỏi màn thì **không có node**. Hệ quả thứ hai của `full={false}`,
  reviewer context mới đo được: «Bỏ» theo nội dung còn **47,2dp** ngang — nên
  `RudiButton` có `minWidth: 48` cạnh `minHeight`, ở kit chứ không vá riêng hàng
  này, và cổng bounds đòi mỗi nút ≥ 48dp cả hai chiều.
- **Câu lỗi nói với người cầm máy, không nói với người đang debug** (audit native
  09/09, F43). Mất kết nối là **một câu** cho cả app — `LOI_KHONG_NOI_DUOC` trong
  `api.ts`: «Không nối được máy chủ. Kiểm tra mạng rồi thử lại.» — dùng ở
  `thongDiepNguoiDoc(0)` **và** năm chỗ dựng `ApiError(0, "unreachable")` thẳng,
  thay câu cũ in `BASE_URL` rồi hỏi «máy chủ có đang chạy không?». Nó gọi tên
  việc duy nhất người ấy làm được và **không hứa** «chưa ghi gì»: mất kết nối là
  ca duy nhất client thật sự không biết máy chủ đã ghi hay chưa. 404 không có câu
  Việt của máy chủ nói app và máy chủ chưa khớp, việc làm tiếp là cập nhật app —
  không «kiểm tra địa chỉ máy chủ ở cuối màn hình», và không «báo cho nhóm kỹ
  thuật» vì app không có kênh ấy (reviewer 10/09). Địa chỉ đi kênh dev: một
  `console.warn` gác `__DEV__` ở `call`, viết tiếng Anh không template. Cùng luật
  cho câu dự phòng của màn: `Bill`/`Profile` và sáu chuỗi legacy bỏ «tại
  `${BASE_URL}`». Cổng là **quét nguồn** (`trang-thai.test.mjs`): không template
  literal nào trong `src/**` trộn chữ Việt với `${BASE_URL}`/`${state.url}`/`${url}`
  — hẹp đúng các biến giữ địa chỉ máy chủ, vì **link khách** trong tin chia sẻ
  của đợt thu (`envelope.url`) là nội dung, không phải rò; bản đầu quét mọi
  `*url` và vu oan chỗ ấy.
- **Placeholder của ô nhập là chữ của nhà vẽ, không phải hint native** (audit
  native 09/09, F44). Android dàn hint theo bề rộng view và cho **xuống dòng ngay
  cả ở ô một dòng**, rồi cắt dòng hai ở đáy ô (ảnh 20: «Tìm quán,» / «món…»
  cụt); RN không lộ `ellipsize` cho `TextInput`, và `numberOfLines` mặc định đã là
  1 nên không phải cách sửa. `ui/Field.tsx` (lõi tách khỏi `ui.tsx` để node test
  render được — `ui.tsx` kéo `expo-image`/vector-icons/Reanimated, không nạp được
  dưới node) vẽ `Text numberOfLines={1}` phủ đúng ô input khi giá trị rỗng, **không
  truyền `placeholder` xuống native** ở ô một dòng; ô nhiều dòng giữ hint native
  vì ở đó xuống dòng là đúng. Tên trợ năng vẫn trọn câu, lớp phủ ẩn khỏi screen
  reader để không đọc hai lần; `paddingHorizontal: 0` ở input để mép chữ vẽ và
  caret trùng nhau. **Hàng tìm thích ứng**: ở `chuLon(fontScale)` ô tìm chiếm cả
  hàng (`flexBasis: "100%"`, hàng `flexWrap` canh phải) và các nút icon xuống
  dòng — cùng mẫu khay sticker; câu 30 ký tự của Khám phá live ở 2.0 vẫn «…» kể
  cả full-width, đó là điểm dừng chấp nhận. Cổng: markup RNW không có attribute
  `placeholder` trên input, câu là node chữ riêng, `aria-label` trọn câu
  (`o-tim-placeholder.test.mjs`); xuống dòng chỉ đo được trên máy —
  `native-r12/kiem-placeholder.mjs` đòi node placeholder cao ≤ 1,5 dòng ở cỡ chữ
  đang đo (hint native **không** là node chữ nên không đo được — thêm một lý do
  để nó là chữ của nhà vẽ).
- **Trích dẫn trả lời** đứng TRÊN bong bóng, trong khối của hàng: viền
  `line`, vạch trái 3dp màu `accent` của theme, tên `caption inkSoft`, một
  dòng xem trước `caption ink`. Thanh «Đang trả lời …» cùng hình dạng, nằm
  ngay trên ô soạn, có nút «Bỏ trả lời».
- **Tin đã xoá** là bong bóng giấy (`card`/`line`) với `caption` nghiêng
  `inkFaint` «Tin nhắn đã bị xoá»; không trích dẫn, không giữ lâu.
- **Theme bong bóng** (`mau-chat.ts`, 5 bảng trong `tokens.json` khoá
  `chatTheme`, `mac-dinh` = accent của scheme) chỉ tô bong bóng của người gửi
  và viền chip phản ứng của mình; tông dẫn của màn vẫn là accent thương hiệu.
- **Pill dưới tiêu đề**: hàng hai pill cân giữa (`pills`), «N thành viên ·
  xem và mời ›» (cặp: «Xem hồ sơ ›») và «Cài đặt» (mở `CaiDatNhomSheet`);
  cùng `caption inkSoft` + icon 15 `inkFaint`, cao 40, không viền.
- **Cặp (nhắn riêng)** dùng nguyên màn chat với tên người kia làm tiêu đề;
  ở Conversations là chữ cái đầu (`Avatar` 44) thay glyph nhóm, dòng phụ
  «Nhắn riêng».

### Hai cột trên cửa sổ rộng (`HaiCot`, `AiCoGi`)
`HaiCot` đọc `twoPane` từ hợp đồng thích ứng: expanded (≥840dp) hoặc medium
đủ cao xếp hai cột 3:2 (`gap: space.lg`, mỗi cột giữ nhịp dọc 18 của màn),
điện thoại xếp dọc theo thứ tự; `phaiChiKhiRong` bỏ hẳn cột phải trên điện
thoại khi nội dung ấy đã có ở cột trái. Tờ bill dùng nó: bước gán món có
`AiCoGi` («Ai có gì» — mỗi người một hàng `label` + `caption` liệt kê món,
món chưa có người ở cuối bằng `warn`; chỉ đếm và tên, không tiền), bước kết
quả có «ai đã trả» và tên khoản bên phải sổ. Không thẻ, không cột nào có nền.

### Mục tiêu chạm và trình đọc màn hình
Mọi node bấm được ≥48×48dp — kể cả `TextInput` bên trong `Field` (52dp hộp,
48dp ô), ô soạn chat, pill dưới tiêu đề chat, pill điểm đến. Một câu chỉ là
`Pressable` khi còn việc để bấm (`Pressable` bị `disabled` vẫn là «nút» với
TalkBack). Hai control cùng chữ trên một màn phải khác nhau ở `accessibilityLabel`
(«Đánh dấu Minh Anh đã trả»). Đo bằng `scripts/a11y_native_audit.py`.

### Lịch trình (`HangChang`)
Giờ trái (`label` tabular, rộng tối thiểu 46, canh phải), trục 14 với nút
12 và đường 2dp, thân phải (`label ink` tiêu đề, `caption` dòng phụ với
`phuTone` `inkSoft`/`inkFaint`/`accent`, `caption inkSoft` ghi chú), khe
`phai` cho con dấu/nút/menu, cao tối thiểu 64, thân đệm dưới 18. Mực liền
`lineStrong` cho chặng nhóm giữ; `phac` nét chì đứt `inkFaint`; `daToi` tô nút
`split`; `cuoi` không vẽ đường dưới. **Mọi trạng thái cũng là một chữ**, không bao giờ chỉ là nét: `phu` mang
tên địa điểm khi có, «Chọn địa điểm» khi chưa có; trạng thái nháp nói **một
lần** ở đầu màn (badge «Nháp», AiNote), không lặp «Có thể thay đổi» dưới từng
hàng (báo cáo 07/09 §4.6).
**Ảnh chặng qua `anh`** (`{ anh: AnhCoGhiCong, alt, loai? } | null`, thay
`AnhChang` ở khe `phai`): ảnh 44 (`expo-image` `cover`, `alt` = tên địa
điểm) trong khung giấy đệm 2 `card` + kẻ tóc `line` bo 10, nền `line` khi
chưa tải; chặng **tự** in `cauGhiCong(anh.nguon)` là dòng cuối của thân
(`caption inkFaint`, không trần số dòng: cột hẹp cạnh giờ và ảnh, ở 1.3 tên
tác giả xuống dòng chứ không ba chấm; ảnh
`sua-review-2/native-lich-trinh-ngay-2-1.3`). Ảnh hỏng: cùng khung vẽ
`GuGlyph` theo `loai`, nhãn a11y «Chưa tải được ảnh: <alt>», và chặng in
«Chưa tải được ảnh» `caption warn` trước dòng ghi công. Khi có `anh`, khe
`phai` nhường chỗ cho ảnh; chế độ chỉnh ở Outing giữ ba nút ở `phai` và
không ảnh. Chỉ cho chặng có địa điểm **có ảnh** (`place.anh`), nên tab Lên
plan và tờ lịch trình AI trong chat có nhịp «điểm đến / đường đi»
(`dot8/07-plan` đã mở; `09-itinerary` và `phone-dark-font13-plan` là bằng chứng reviewer, không mở ở đây).

### Ghi chú AI (`AiNote`) và tờ AI (`ToGiay`)
- **`AiNote`**: ghi chú **lề**: hàng với icon `sparkles` 17 tím, câu `label
  ink`, chữ ký `caption` tím «Rủ Đi AI gợi ý» **dưới** câu; kẻ tóc trên và
  dưới, đệm dọc 12; **không nền tím, không viền trái**. Bốn màn dùng.
- **`ToGiay`** (`chat/TheAi.tsx`), nền cho mọi thẻ AI trong chat: `card`
  viền 1px `line` bo 20, nội dung là mực thường, lịch trình bên trong là
  `HangChang phac`, **ký ở chân** bằng icon 15 + `caption` tông (`sparkles`
  «Rủ Đi AI gợi ý» tím; `receipt-outline` với tông `split` khi là tờ tiền).
  Câu hỏi là tiêu đề của tờ, không có nhãn trên đầu.

### Cards / Containers
- **Hàng + kẻ tóc là container mặc định** trên giấy. `Card` v1 (bo 20, đệm
  16, viền `line` + `cardShadow`) còn trong kit nhưng **không màn nào trong
  đợt này gọi**.
- **`CoverBand`**: nền `cover` + `Grain vaiBia` 0.3, bo góc dưới 28, đệm trên
  `md` (+`insets.top` khi `underStatusBar`), đệm dưới `lg`, tràn lề theo
  `bleed`; `compact` rút vải khi bàn phím mở; chứa logo compact, `hero`
  `coverInk`, đoạn dẫn `body` `coverInkSoft`, nút back tròn.
- **`Sheet`**: nền `card`, bo trên 20, đệm ngang `md`, đệm dưới
  `max(insets.bottom, 16)`, cao tối đa 82% cửa sổ (hoặc `maxHeight`), nội
  dung cuộn; vào bằng spring, ra bằng `standard`; scrim `lopPhu.toi(0.42)`;
  nút cứng back Android và scrim đều đóng; `onClosed` nổ **sau khi** tấm đã
  rời màn. **Tay cầm là vùng nắm thật** (đợt 8): hàng cao tối thiểu 36 rộng
  cả tấm, vạch 40×4 `lineStrong` ở giữa, `accessibilityLabel` «Tay cầm»,
  hint «Kéo xuống để đóng»; `Gesture.Pan` `activeOffsetY` 6, tấm đi theo
  ngón tay, thả quá **90 dp** hoặc vẩy **900 dp/s** thì đóng, thả ngắn hơn
  thì lò xo về; scrim mỏng dần theo kéo (`progress × (1 − kéo/480)`). Vùng
  nắm chỉ là hàng tay cầm, không phải cả tấm, nên cuộn của nội dung không
  đánh nhau với kéo (`clip-keo-sheet`). Đặt qua `RudiScreen overlay`, hoặc
  trong một route trong suốt.
- **Khay tạo** (`screens/Create.tsx`) giờ **là** `Sheet` đó, không còn bản
  chép tay: route `create` chỉ `fade` với `contentStyle` trong suốt
  (`app/_layout.tsx`), tab bên dưới còn nguyên dưới scrim (ảnh
  `dot8/12-create-sheet`); nội dung `maxWidth` 560, hành động cao 72 kẻ
  tóc, `PressScale` 0.985; **dòng chi tiết (`note inkFaint`) chỉ ở hành
  động dễ nhầm với hành động khác** («Đăng kỷ niệm» · «Ảnh lên tường nhóm»
  và «Đăng story» · «Một tấm 24 giờ, chỉ bạn bè thấy»), «Tạo cuộc hẹn» và
  «Chia hóa đơn» chỉ một dòng (`sua-sang-1.0/bs-10-khay-tao`); đóng xong
  mới `router.back()`. Mở lạnh (deep
  link, thông báo) `app/create.tsx` dựng vỏ tab trước rồi mở lại sheet: **chỉ
  đọc từ mã**, chưa kiểm trên máy.
- **Ô soạn chat** (Group): hàng bo 22 nền `card` viền 1px `line`, đệm 6, ở
  `footer` của màn; không có dải giấy trống thứ hai dưới nó.
- **Hoá đơn trên gỗ** (Bill): khung tối thiểu 420 bo 20 nền `giayHoaDon.khung`
  với ảnh gỗ, viền `lopPhu.trang(0.22)`; tờ hoá đơn gradient `giayHoaDon.nen`,
  chữ đen nâu hoá đơn, tổng tabular. Đây là **artwork của fixture** (theme.ts
  cho phép hex ở đây), không phải token.
- **`Washi`**: dải mép xé cao 30 (34/40 dưới wordmark), đệm ngang 18, rộng
  tối thiểu 120, tô `brand.coral`/`brand.teal`/`brand.violet` ở 0.9; con là
  chữ mực tối tĩnh. Chỉ Welcome dùng trong đợt này.

### Inputs / Fields
- **`Field`**: cao 52 (108 `multiline`), nền `card`, viền 1px `lineStrong`, bo
  14, chữ `body`, placeholder `inkFaint`, nhãn `label` `ink` phía trên, icon
  dẫn 20 `inkFaint`. **`SearchField`** cùng khung, bo pill, placeholder «Tìm
  quán, món...».
- **`OtpBoxes`**: 6 ô 44×54, viền 1.5, một `TextInput` thật phủ lên
  (`autoComplete="sms-otp"`); ô đang nhập viền `accent`, ô khác `lineStrong`;
  chữ số `title`.
- **`Segmented`**: khung `card` viền `line` bo 14, đoạn 48, chọn = `<tone>Soft`
  bo 10, `role="tab"`.
- **`RosterPicker`**: checkbox cao 48; chưa chọn `card`/`lineStrong`, chọn
  `splitSoft`/`split` + check; tên `ink` xuống dòng; lưới ô 130 gap 8 tối đa
  3 cột.
- **Lỗi**: một câu `body` màu `warn` ngay dưới control; không toast, không
  modal. Đang tải: `StampButton loading` tại chỗ vừa bấm, hoặc `Skeleton`.

### Navigation
- **`RudiTabBar`** tự vẽ: nền `card`, cạnh trên hairline `line`, cao **64 +
  max(insets.bottom, 10)**; bốn tab `role="tab"` cao tối thiểu 48, icon
  Ionicons 24 (outline → filled khi chọn), nhãn 12/14 một dòng; đang chọn
  `accent`, còn lại `inkFaint`; chỉ báo băng 28×4 `accent` treo ở cạnh trên
  cột đang chọn, trượt `standard` 200ms; haptic `select`.
- **FAB «Tạo mới»**: cột giữa, tròn 56, nền `brand.coral`, glyph `add` 30
  `brand.coralInk` tĩnh, vòng 4px `ground`, nhô lên 22, elevation 6, nhấn co
  0.94 haptic `impact`, mở `/create`.
- **Rail** (medium+): rộng 104, cạnh phải hairline, mỗi mục 72 với icon +
  nhãn `caption`, chỉ báo vạch 4px `accent` bên trái trượt theo `translateY`;
  FAB nằm trong rail, vòng `ground`, không nhô (`tablet-light-explore`).
- **`TopBar`** trên giấy: tiêu đề `title` cân giữa, phụ đề `caption inkSoft`,
  back chevron 48 hoặc wordmark `ink` 18 khi là đầu tab, phải là `DemoBadge
  compactLabel` hay `IconButton quiet`. Ô icon app chỉ ở Welcome/Login.
- iOS: `BlurView` 78 theo scheme thay nền `card` (chỉ đọc từ mã, chưa có
  ảnh iOS).

### Trạng thái rỗng, tải, lỗi
- **`EmptyState`** năm loại (`first-use`, `no-results`, `filtered`,
  `permission`, `failure`): `h2` + một câu `body` `inkSoft` rộng tối đa 420,
  **một** hành động `RudiButton compact` (`outline` khi `failure`) và một cửa
  phụ `ghost`; khe `illustration` từ 08/09 nhận `<Canh>` **rộng 168**:
  Album «Chưa có kèo nào» → `chua-co-keo`, «Chưa có khoảnh khắc» →
  `chua-co-anh`, Khám phá «Chưa thấy nơi phù hợp» → `tim-khong-ra`
  (`sua2-sang-1.0/bs-04-tim-khong-ra`: cảnh, `h2`, một câu, một nút). Hai
  cảnh còn lại (`chua-co-hoi`, `chua-co-ban`) đã vẽ, chưa có màn gọi. Câu
  thân rút về một câu vì cảnh đã nói vế đầu.
- **`Skeleton`**: xương màu `line`, bo 10, băng sáng `card` 0.55 chạy 1400ms;
  tắt hẳn dưới Reduce Motion. `SkeletonLines` dòng cuối 62%.
- **`ErrorState`**: cùng khung với `EmptyState kind="failure"`.
- **Bộ lọc vắng vì không có dữ liệu** được nói thẳng bằng một câu `body
  inkSoft` («Không có dữ liệu tháng khác nên không hiện bộ lọc kỳ»), không
  vẽ control chết.

### Signature: Bìa mở ra trang (Welcome → Login)
Welcome là bìa đóng (ảnh `01-welcome`): indigo tràn màn với vân vải, wordmark
rất lớn ở phần ba trên (`coverInk`), washi cam nghiêng -2° cao 34 mang «AI đi
chơi, chia bill thông minh» bằng mực tối, route mực 4 chặng với glyph (people
· compass · receipt · images) và chặng đang ở tô coral chạy theo trang pager,
pager 4 trang (tiêu đề EB 28/33, body `coverInkSoft`), chấm 7 (đang ở 20
rộng, cam), rồi con dấu «Rủ Đi thôi!» nghiêng -3 rộng bằng chữ và liên kết
«Tìm hiểu thêm ›» dưới nó. Bấm con dấu: bìa **nhấc** (translateY -48, mờ tới
0.65) trong `shared` 300ms easing `accelerate`, rồi push `/login`; Login mở
với `CoverBand` dưới status bar và con dấu «Gửi mã» cỡ `vua`, tức lời rủ có
một ngôn ngữ ở cả hai màn. Reduce Motion: pager nhảy thẳng, bìa không nhấc.

### Signature: Con dấu rơi xuống (trạng thái thành đúng dưới ngón tay)
Dòng FORM của hợp đồng gọi tên «một cú đóng dấu khi một trạng thái thành
đúng»; đợt 8 là nơi nó tồn tại. Bấm «Đánh dấu đã trả» ở quyết toán: nút co
0.98, hàng đổi sang con dấu «ĐÃ TRẢ» teal và con dấu **lao** xuống từ 1.35
với mực nhạt, **chạm** trang (mực đầy, rung `success`), **lún** 0.97 rồi
đứng; hàng bên cạnh đã đóng từ trước không nhúc nhích (khung 26 của
`clip-dau-tra`: một con dấu đầy mực, một con dấu còn nhạt đang rơi). Cùng cú
đó ở «Đã tới» khi điểm danh và ở nghĩa vụ vừa được máy chủ xác nhận. Reduce
Motion: con dấu chỉ xuất hiện. Không có confetti, không toast, không đếm số.

### Chuyển động (đặt cùng thành phần)
`tokens.motion` qua `src/rudi/motion.ts` và `useMotion`: **instant 100**
(bấm, chip, haptic) · **standard 200** (đổi trạng thái, chỉ báo tab, sheet
đóng, skeleton → nội dung) · **shared 300** (bìa mở, thẻ sang chi tiết, kèo
sang timeline, ảnh sang viewer) · **celebrate 550** (một lần mỗi sự kiện, ba
khoảnh khắc: chốt kèo, xong bill, mở huy hiệu; `celebrateOnce` giữ ngân sách).
Easing `standard` [0.2,0,0,1], `decelerate` [0,0,0.2,1], `accelerate`
[0.3,0,1,1]. Spring nhấn {damping 18, stiffness 260, mass 0.6}, thả {20, 180,
0.8}. `PressScale` chỉ scale (0.98 nút giấy, 0.985 hàng/khay tạo, 0.97 con
dấu CTA, 0.96 chip, 0.94 FAB/icon, 0.92 back), không opacity; kit không còn
nhánh `pressed` mờ. Cú đóng dấu (`Stamp dong`) là cách `celebrate` được tiêu:
130 + 60 ms + lò xo nhấn, haptic `success` ở nhịp chạm. **Reduce Motion đưa
mọi bậc trừ `instant` về 0** và gập cú đóng dấu thành con dấu tĩnh. **Tiền
không animate trước khi domain state hợp lệ**: `moneyCountUpMs` trả 0 khi
chưa hợp lệ; con dấu live chỉ rơi sau khi máy chủ xác nhận. `useMotion` đọc
cài đặt ban đầu và nghe `reduceMotionChanged` trong phiên.

**Luật Một Cú Đóng Mỗi Sự Kiện.** `dong` chỉ truyền cho **hàng người đó vừa
bấm** (`vuaTra`, `vuaToi`, `vuaNhan` là state của màn, không phải của dữ
liệu); màn mount với trạng thái đã đúng thì con dấu chỉ *có ở đó*. Remount
không bao giờ phát lại; bỏ điểm danh không đóng dấu; một danh sách không
bao giờ rơi cả loạt.

### Câu chữ (copy) trên control
Tiếng Việt, không gạch dài (em dash); dùng « », dấu chấm giữa « · » để nối
sự thật. Control **gọi tên hành động** («Gửi mã», «Đánh dấu đã trả», «Dùng
plan này», «Thêm khoảnh khắc»), không «OK»/«Tiếp». `accessibilityLabel` mở
bằng động từ: «Mở {tên địa điểm}», «Mở ảnh 3», «Chọn ảnh», «Đóng chọn ảnh»,
«Thêm ảnh», «Lưu …». Con dấu là danh từ/trạng thái ngắn IN HOA. Dữ liệu
fixture luôn dán «Dữ liệu demo» (đầy đủ), «Demo»/«Nháp» (trong `TopBar`),
«Bản trải nghiệm» (Welcome). Số tiền viết «1.106.250đ»; không có số nào màn
tự bịa: đếm ảnh, huy hiệu, ngày còn lại đều tính từ dữ liệu.

**Luật Nói Một Lần** (08/09, báo cáo 07/09 §8.5, §4.6): mỗi trạng thái và
mỗi sự thật có **một** chỗ trên màn.
- `HangChang phu` mang **tên địa điểm** khi có, «Chọn địa điểm» khi chưa có
  (live: «Mở địa điểm» khi máy chủ chỉ có id), không «Đã gắn địa điểm · bấm
  để mở».
- Trạng thái nháp nói **một lần** ở đầu màn (badge «Nháp», `AiNote`), không
  «Có thể thay đổi» dưới từng hàng.
- Hàng check-in là **một con dấu**: đã tới thì chỉ `Stamp`, chưa tới thì
  một `note` «Chưa tới»; không caption «Đã check-in» đứng cạnh dấu «ĐÃ TỚI»
  (`sua-sang-1.0/bs-20-check-in`).
- Luật tính toán đứng sau **một cửa mở** «Cách tính» (Thành tích:
  `Pressable` cao 48, `label inkSoft` + chevron, `accessibilityState
  expanded`), không làm chân trang dưới mọi danh sách; luật vẫn giữ nguyên
  câu.
- Khay tạo: dòng chi tiết chỉ ở hành động dễ nhầm.
- Nút làm mới gọi «Làm mới», không «Đọc lại từ máy chủ»; fixture bill nói
  «Bill mẫu · N dòng · tổng …. Bạn đang thử bằng dữ liệu mẫu.» thay vì giải
  thích canonical/OCR; ô tìm Khám phá tự nói bằng placeholder («Tìm quán,
  món… hoặc hỏi Rủ Đi AI»), không đoạn hướng dẫn dưới ô.

### Trợ năng (sàn)
Đích bấm 48dp (nút 52/60, `compact` 48, chip 48, tab 48, back 48, link bìa
48, tay nắm kéo 48×56, checkbox 48); chữ nhỏ nhất 13sp caption, trừ ba cỡ
riêng có kiểm ở font 1.3 (tem 12, nhãn tab 12, demo 10); mọi cặp chữ/nền
trong bảng đo dưới; viền control ≥ 3:1; con dấu có `accessibilityLabel`,
`Stamp` không role; `role="tab"` cho tab và segmented; Reduce Motion tôn
trọng; chụp lại ở font 1.3 trước khi nói «không cắt».

## Do's and Don'ts

### Do:
- **Do** dùng `mauSang.ink` tĩnh cho mọi chữ/icon/viền đặt lên `brand.coral`
  (washi, con dấu, chặng route); đo 5.41:1 ở cả hai scheme.
- **Do** vẽ ranh giới control bằng `lineStrong`/`coverLineStrong` và thêm dòng
  trong `interactive_boundaries()`; cạnh container bằng `line`.
- **Do** giữ đích bấm 48dp, body 17/24 và caption 13/18; kiểm riêng nhãn
  tab/tem/demo có cỡ nhỏ hơn, và chụp lại ở font 1.3 trước khi nói «không
  cắt».
- **Do** trải chất liệu bằng `Grain` (lưới ô) ở đúng opacity đo được: vải
  0.30, giấy 0.45/0.30, mực 0.26; dưới ngưỡng là màu phẳng.
- **Do** để `CoverBand underStatusBar` khi màn có bề mặt `cover`, `StatusBar`
  sáng trên bìa, tối trên giấy sáng.
- **Do** làm con dấu rộng bằng chữ, một vành, không mũi tên; `lon` trên bìa,
  `vua` trong form; nghiêng -3/-1/0 theo bìa/form/bảng.
- **Do** đóng dấu **chỉ khi trạng thái đã đúng**, và con dấu luôn mang một
  chữ; nháp là nét chì đứt và nhãn «Nháp».
- **Do** viết tiền thành dòng sổ `DongTien` + `Money`: số nguyên đồng,
  tabular, tông trên số, `countUp` chỉ khi domain state hợp lệ, và một câu
  nói số này chưa phải sổ cái.
- **Do** để danh sách nằm thẳng trên giấy với kẻ tóc; thẻ chỉ cho tờ AI và
  bản in.
- **Do** cho mỗi ảnh dẫn một khung in và dòng xuất xứ; ảnh có giấy phép in
  `Attribution` bên dưới; số ảnh đếm từ dữ liệu.
- **Do** ký tờ AI ở chân («Rủ Đi AI gợi ý») và để ghi chú AI là ghi chú lề
  với kẻ tóc.
- **Do** dùng `Photo ratio` 16:10 compact / 21:9 rộng cho ảnh dẫn, và
  `ResponsiveRow maxColumns` khi ô là thumbnail (album 6).
- **Do** giữ một tông dẫn mỗi màn; teal ở màn tiền, tím ở màn AI, cam ở lời
  rủ.
- **Do** đặt sheet qua `RudiScreen overlay`, composer và nút chân qua
  `footer`, luồng chat với `keepEnd` và thanh tên nhóm qua `header`.
- **Do** cho mọi thứ bấm được một lò xo scale qua `PressScale` (0.98 nút,
  0.96 chip, 0.985 hàng, 0.94 icon); haptic `select`/`impact` ở bấm,
  `success` chỉ ở nhịp chạm của con dấu.
- **Do** truyền `Stamp dong` cho đúng hàng vừa bấm, và ở màn live chỉ sau khi
  máy chủ xác nhận; `nen` khi con dấu nằm trên ảnh.
- **Do** cho chặng có địa điểm có ảnh `HangChang anh={{ anh, alt, loai }}`;
  chặng tự vẽ khung 44 và in ghi công; chặng khác để trống khe `phai`.
- **Do** đưa ảnh catalogue vào màn chỉ qua `MediaSlot`, `HangDiaDiem` hay
  `HangChang` (kiểu `AnhCoGhiCong`, spread `khungAnh(anh)`); thêm khung mới
  thì thêm tên vào `tests/rudi-anh-ghi-cong.test.mjs`.
- **Do** cấp mọi màu mới qua `tokens.json` → script → `guest.css` + DESIGN.md
  cùng PR; `rudi-khong-hex` giữ `theme.ts` là file duy nhất viết hex.
- **Do** vẽ minh hoạ qua `art/*.ts` → `VeLop`: chỉ `M`/`L`/`C`/`Z` tuyệt
  đối dựng từ số, vai màu `MauVe` thay vì màu, và thêm hình mới vào
  `tests/art-duong.test.mjs`.
- **Do** để Nếp chỉ ở trạng thái rỗng và cửa vào, dưới 72dp thì bản 48; cảnh
  phải đứng được khi `nep={false}` và đọc thành một câu.
- **Do** vẽ sticker ở cả hai cỡ đọc (`chiTiet` true/false) và nhìn ô khay 64
  trước khi đọc nhãn; hình mới thêm vào `tests/rudi-chat-sticker.test.mjs`
  chạy cả hai cỡ.
- **Do** vẽ phân loại bằng `GuGlyph` (chip `leading`, ô gu, khung trống
  `PlaceGlyph`), mực đổi coral khi chọn; id lạ là thẻ gấp.
- **Do** dùng `note` (13/400) cho dòng phụ là một câu và giữ `caption` (600)
  cho nhãn ngắn.
- **Do** xếp kết quả Khám phá dẫn → cặp so sánh (4:3, tim trên ảnh, giá
  đứng riêng dòng) → hàng; lý do dưới ảnh dẫn chỉ khi máy chủ gửi.
- **Do** viết mỗi sự thật một lần trên màn: tên địa điểm ở `phu`, nháp ở đầu
  màn, check-in là một con dấu, luật tính sau «Cách tính».
- **Do** nhân `minItemWidth` với `fontScale` khi ô lưới chứa nhãn chữ.

### Don't:
- **Don't** đặt chữ nhỏ hay icon lên `brand.*` bằng mực của scheme; coral với
  chữ trắng 2.92:1.
- **Don't** đặt chữ `accent` nhỏ trên `cover` (2.83:1 ở scheme sáng).
- **Don't** đặt kicker/eyebrow (chữ in hoa giãn nhỏ) trên tiêu đề; chữ in
  hoa của hệ là mực con dấu và nó đứng cạnh/dưới nội dung.
- **Don't** mở màn tiền bằng hero metric (số to trong khối màu, nhãn nhỏ
  trên, thanh tiến độ dưới); sổ là hàng.
- **Don't** dùng thanh tiến độ hay vòng tiến độ **thay** nội dung; số phiếu
  bình chọn có `ProgressBar` là ngoại lệ đã có chữ đi kèm («5 phiếu của bạn
  trên bản này»), không phải mẫu.
- **Don't** lồng thẻ trong thẻ, hay kẻ viền trái màu dày hơn 1px.
- **Don't** thêm vành trong hay mũi tên vào con dấu; đừng kéo con dấu hết
  cột.
- **Don't** đóng dấu lên bản nháp hay đề xuất AI chưa chốt; đừng để trạng
  thái chỉ là màu hay chỉ là nét.
- **Don't** giả độ sâu bằng bóng lệch cứng hay dập nổi; con dấu và bìa không
  có bóng.
- **Don't** dùng `resizeMode="repeat"` cho chất liệu trên Android.
- **Don't** animate `opacity` trên một pressable tròn nằm trên nền có vân;
  animate scale, mặt tròn là View con. Đừng thêm nhánh `pressed` mờ mới:
  kit đã bỏ hết, chúng chạy trên JS thread và mù với Reduce Motion.
- **Don't** truyền `dong` lúc mount hay cho cả danh sách; đừng thay cú đóng
  dấu bằng một zoom ease-out, confetti hay toast.
- **Don't** vẽ tay cầm sheet không kéo được; tay cầm là lời hứa kéo-để-đóng.
- **Don't** lặp ô icon app ở đầu mỗi tab; header tab là wordmark trơn.
- **Don't** ghi `transform: undefined` vào style Reanimated; chỉ spread khi
  có góc nghiêng.
- **Don't** dựng hình dạng máy chủ chưa có: không ô mã chuyển khoản, không
  VietQR, không số tài khoản (ADR-0015/0016); sản phẩm nói phần của mỗi người
  rồi dừng.
- **Don't** in một số màn tự bịa: đếm ảnh, huy hiệu, ngày còn lại, tổng đều
  từ dữ liệu; fixture phải dán nhãn demo/nháp.
- **Don't** cho ảnh xuất hiện không xuất xứ, ảnh stock cho địa điểm thật, ảnh
  người thật cho avatar (ADR-0020 §2.5).
- **Don't** thêm face display thứ hai, hay đưa Bricolage vào body/nhãn/ô nhập.
- **Don't** thêm toast hay modal lỗi; lỗi là một câu `warn` dưới form.
- **Don't** dùng gạch dài trong câu chữ; đừng đặt nhãn «OK»/«Tiếp» lên control.
- **Don't** ship nhãn demo trên tiền thật; `DemoBadge` phải rỗng ở phiên live.
- **Don't** đặt Nếp cạnh số tiền, lỗi hay xung đột; đừng cho Nếp dấu chuyển
  động hay mặt hào hứng.
- **Don't** biến vòng hở thành progress ring (animate, phần trăm, khe hẹp);
  đừng đặt đường chuyền làm tiến độ mà không có chữ.
- **Don't** phát `Q`/`A`/lệnh tương đối hay số mũ trong `d`; đừng gõ đường
  SVG bằng literal chín chữ số.
- **Don't** đưa hex hay màu vào `art/*.ts`; lớp vẽ chỉ biết vai.
- **Don't** thu bản sticker 120 xuống 64 cho khay; dưới 72dp là bản rút gọn
  riêng, hoặc hình đã đủ đọc như vẽ.
- **Don't** lặp một sự thật hai chỗ trên màn (ngày ba lần ở album, «Đã
  check-in» cạnh dấu «ĐÃ TỚI», «Có thể thay đổi» dưới mỗi chặng).
- **Don't** vẽ ảnh địa điểm mà không nói được nguồn (M12, ADR-0017 §2.5): ảnh có giấy phép thì tác giả + giấy phép ngay dưới ảnh (kể cả ô nhỏ trên hàng); ảnh của nhóm chỉ người trong nhóm thấy và máy chủ lọc; không xuất xứ thì về dải typographic, không mượn ảnh khác.

## Những gì bản ship KHÔNG phong thánh

Có trong cây nhưng không phải hệ; người sau đừng lấy làm mẫu:

- `Eyebrow` và `SurfaceLabel` **vẫn còn export trong `ui.tsx`** (dòng 237 và
  958 ở head này) dù tài liệu mốc trước ghi đã xoá; không màn nào gọi
  (`grep` 0). Là kicker/eyebrow bị craft floor cấm; giữ lại là nợ dọn kit,
  không phải thành phần. `FloatingGlass`, `Stat`, `ProgressBar` cùng số phận:
  `Stat` và `FloatingGlass` 0 màn gọi; `ProgressBar` một chỗ (kết quả bình
  chọn Group.tsx:362) chỉ được để lại vì có chữ đi kèm, không phải mẫu.
- `WordmarkEmbossed.tsx` (dập nổi bằng bóng lệch): đã xoá ở `5cc57d2`.
- `Card` v1 và chip tĩnh: còn trong kit, không màn nào trong đợt này dùng
  cho trạng thái; không lấy làm container mặc định.
- Ảnh fixture trong album, tường và Khám phá là ảnh Commons đã nhập theo
  mapping duyệt (`tools`, commit `5d853f4`) với dòng xuất xứ của **fixture**
  («Team Đà Lạt · Đà Lạt · …»); dòng đó là mẫu định dạng, không phải xuất xứ
  thật của tấm ảnh.
- Hoá đơn trên gỗ (`giayHoaDon`, `demoAssets.wood`) là artwork fixture của
  màn Bill; `bangMauFixture`, `mauSao`, `mauLogo` trong `theme.ts` là màu của
  thế giới fixture, không phải token.
- iOS (`BlurView`, `cardShadow` iOS) và OAuth Google chỉ đọc từ mã; chưa có
  ảnh iOS trong `.impeccable/review/`.
- **Nhánh `pressed` mờ opacity còn trong màn feature** (đợt 8 chỉ chuyển
  kit): `Group.tsx` dòng ghim chuyến và ô bình chọn, `HangDiaDiem.tsx`
  `PlaceLead`/`PlaceRow`, `Outing.tsx` hàng điểm danh và «Chọn tất cả»,
  `Bill.tsx` dòng món, `Profile.tsx`, `GroupWallLive.tsx`; `styles.pressed`
  0.68 và `buttonPressed` vẫn khai trong `ui.tsx` mà không ai gọi. Là nợ
  đồng bộ với `PressScale`, không phải hai kiểu phản hồi bấm của hệ.
- `/create` mở lạnh (replace `/explore` rồi push lại sheet) chỉ đọc từ mã;
  reviewer đợt 8 không kiểm trên máy.
- Số ms của cú đóng dấu đọc từ `Stamp.tsx`; clip 20 fps chỉ chứng minh thứ
  tự nhịp (nhạt → đầy, hàng bên không động), không đo được 130/60.
- **Nút vô hiệu của kit** («Tiếp tục» mờ trên `sua2-sang-1.0/bs-03-so-thich`):
  chữ trắng trên coral nhạt, reviewer 08/09 ghi là nợ tương phản có tên,
  không đo ở đây và không có tỉ lệ nào được in để hợp thức nó. Không lấy làm
  mẫu trạng thái vô hiệu.
- **Chế độ tối sau loạt sửa cuối** chỉ có bảng art (`art.png` nửa dưới) và
  `toi-1.3/` chụp **trước** loạt sửa; cảnh trong `EmptyState`, cặp so sánh
  và ô gu ở dark chưa có ảnh.
- Album theo ngày chỉ ở `AlbumLive.tsx` và chỉ đọc từ mã; album fixture
  `Memories.tsx` vẫn lưới đều ba cột (ảnh `bs-18-album`), là hai nhịp của
  hai cây, không phải hai kiểu album của hệ.
- Hai cảnh `chua-co-hoi`, `chua-co-ban` và bốn tư thế Nếp ngoài `moi`,
  `ghi-lai` mới có trên bảng art, chưa màn nào gọi; ghi ở đây để không bị vẽ
  lại khác, không phải để nói chúng đã lên máy.
- Bảy sticker cũ (`di-thoi` … `tuyet-voi`, khối màu lớn, chỉ lớp tô) và
  `cho-ti` (nét + hai mảng sắc độ) đang là hai ngữ pháp trong một khay; sự
  lệch ấy là trạng thái chờ quyết định, không phải hai kiểu sticker của hệ.
- Icon Ionicons vẫn là ngôn ngữ của control (tab, sự thật, nút tròn, chip
  không `leading`); lớp vẽ chỉ thay icon ở **nội dung phân loại**, không
  phải một cuộc thay icon toàn hệ.

## Cổng phải xanh trước khi đổi hệ này

```bash
python3 -m pytest services/api/tests/web -q                   # token guest.css khớp tokens.json; đọc ui/**.tsx và DESIGN.md; mọi tỉ lệ in ở đây đo lại được
python3 scripts/sinh_token_ui_v2.py                           # đổi màu: sinh lại 4 gương, không gõ tay
cd apps/mobile && node --test tests/rudi-khong-hex.test.mjs   # không file nào trong vỏ RuDi tự gõ mã màu ngoài theme.ts
cd apps/mobile && node --test tests/duong-svg.test.mjs        # đường SVG parse được theo cách Java parse
cd apps/mobile && node --test tests/art-duong.test.mjs        # mọi hình của lớp vẽ (Nếp, gu, motif, cảnh) chỉ M/L/C/Z tuyệt đối, vai màu hợp lệ
cd apps/mobile && npx tsc -p tsconfig.test.json && node --test tests/rudi-chat-sticker.test.mjs   # tám id khớp stickers.json; mọi lớp của mọi sticker ở cả hai cỡ đọc parse như Java; lớp tô kín, lớp nét dương; id lạ vẽ «khac»
cd apps/mobile && npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-anh-ghi-cong.test.mjs   # ảnh catalogue chỉ tới Image trong ba khung in ghi công; không ai đọc trần anh.source, không ai gọi AnhChang
python3 -m pytest tests/test_chat_lieu_tiles.py -q            # ô mực đo trên coral ở 0.26 nằm 6 đến 12 mức (gốc repo)
```

Màn native thì cổng là **emulator**, không phải web export (dòng FINISH của
hợp đồng): light/1.0, dark/1.3, tablet bằng `wm size`, rồi đọc ảnh và đo
pixel; tsc, web export và detector đã mù với ba lỗi thật ở lát này (crash
`transform: undefined`, crash parse `d`, vân dừng ở một phần ba).
