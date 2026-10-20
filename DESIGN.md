---
name: Rủ Đi
description: Cuốn sổ chuyến đi của cả hội, bìa vải indigo và trang giấy sáng (đêm là sổ đóng trên bàn), ba cuộn washi mang nghĩa, trạng thái là con dấu
colors:
  ground: "#f7f3ec"
  card: "#ffffff"
  line: "#e6dfd3"
  paper: "#ffffff"
  paper-shade: "#e6dfd3"
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
  paper-dark: "#2e335c"
  paper-shade-dark: "#181b36"
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
  button-warn:
    backgroundColor: "{colors.card}"
    textColor: "{colors.warn}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 18dp"
    height: "52dp"
  button-disabled:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink-soft}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 18dp"
    height: "52dp"
  field-line:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    padding: "0 0 4dp"
    height: "48dp"
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
  to-giay:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.small}"
    padding: "{spacing.md}"
---

# Design System: Rủ Đi

<!-- impeccable:design-schema 2 -->

> **Đọc bằng chứng trong tài liệu này.** Mỗi khẳng định dưới đây dẫn về một bó
> bằng chứng ở `docs/archive/claude/<ngày>/<chủ đề>/` hoặc `docs/archive/codex/<ngày>/...`.
> Bài viết (`README.md`, báo cáo, bảng số đo), dump `hierarchy.xml` và mọi file
> đo dạng text **vẫn nằm trong cây** — có cổng đọc chúng làm fixture. Riêng
> **ảnh chụp màn** đã được gỡ ở đợt dọn repo và **chỉ còn trong lịch sử Git**.
> Muốn xem lại ảnh của một bó, lấy ở commit mà chính bó đó ghi, ví dụ:
>
> ```bash
> git show <sha>:docs/archive/claude/2026-09-12/nen-to-giay/anh/r18-99-to-giay-tren-fs1.0-sang.png > /tmp/xem.png
> git log --oneline --all -- docs/archive/claude/2026-09-12/nen-to-giay/   # tìm sha nếu bó không ghi
> ```
>
> Lý do gỡ: 382 file ảnh chiếm 191 MB, tức phần lớn dung lượng repo, và là kết
> quả của một lần chạy chứ không phải nguồn. Kết luận của lần chạy nằm ở bài
> viết — thứ được giữ lại. File text ở lại vì rẻ và vì có script đọc chúng.

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

**Phase 1 «Nếp truyền giấy» (2026-09-12)**, nhánh
`claude/p0-w-hn-1-nen-to-giay`, head `137c6c04`, là **nền không cần máy chủ**
của sổ hai người: tờ thư gấp ba (`ui/ToGiay.tsx`), Nếp bản **mảnh** (`gap:
"manh"`, biểu cảm `giu-kin`, ba tư thế truyền giấy), motif `thuGapBa`, và
module «bản tính của sổ» (`so/ban-tinh.ts`). **Không màn người dùng nào đổi,
không token, không route**; mọi thứ mới chỉ lên bảng `app/dev/ui-lab.tsx`.
Bằng chứng: ba vòng đọc mù (reviewer context mới, ảnh cắt không nhãn, trước
khi mở packet) ghi ở `docs/archive/claude/2026-09-12/nen-to-giay/README.md`, 16 PNG
native `r18-99-*` ở `anh/` của cùng thư mục, Maestro `.maestro-bs-r18/99` (flow còn trong lịch sử Git @ `137c6c04`) exit 0
ở bốn cấu hình, và bốn cổng node: `art-duong` (baseline sha256 bản `trang`,
`laDaiGap`, độ lấp đầy, bề dày dải), `so-ban-tinh-mot-cho`, `dau-gach-dai`,
`rudi-khong-hex`. Bốn vòng đọc mù đã ghi trong README (vòng 4: «fix rồi
ship»); mục này phân biệt rõ câu nào là **luật đo được** (một cổng giữ) và câu
nào là **quyết định đọc mù** (một reviewer đọc đúng, chưa ai đo).

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

**Cập nhật có phạm vi 10/10/2026, candidate native `797a3f52`.** Chủ sản phẩm
đánh giá hình thức app trước lượt này khoảng 5/10 và yêu cầu đẹp ở từng trang,
không chỉ dùng được. Bản ghi này đọc source
`797a3f52d77919adcfa86ed639e17b01476a8f2d` và mở ảnh Android sau sửa; chỉ cập
nhật chín bề mặt được nêu trong
[BRIEF](docs/design/nep-ui-implementation/redesign-20261010/BRIEF.md).
Giấy, mực, coral, Bricolage và Nếp canonical giữ nguyên; không đổi bảng token
hay căn tính Rủ Đi. Ảnh `core-layout-reference-v1.png` là tham chiếu phản biện
cấu trúc, chưa phải comp được duyệt, không đặt nghĩa vụ raster, seed hoặc
nhận diện mới. Các verdict Welcome, Story, editor Nếp, Gallery và Gu bên dưới
giữ nguyên SHA và phạm vi lịch sử của chúng.

Các mô tả source mới trong mục «Chín bề mặt native» thắng mô tả bố cục cũ của
đúng những thành phần được gọi tên; chúng chưa phải chuẩn thị giác đã nghiệm
thu cho toàn hệ. Fresh review trước ở `d9c06bf4` trả **fix**; verdict sau sửa
797 còn chờ. Audit rộng A–S và nghiệm thu toàn app vẫn mở.

**Bàn giao I1 tại `7dcacedf` (10/10).** Bảy finding về chọn người/cùng mở
sổ đã đóng: F1/F2/F3/F4/F6/F7 giữ verdict và ảnh `62c1a73b`; F5 nhận verdict
**ship** sau 24 viewport mới tại `7dcacedf`. Hợp đồng hẹp nằm trong Typography,
Layout và Components bên dưới, kế thừa seed `c8e88116` và FORM của
[I1 brief](docs/design/nep-ui-implementation/redesign-20261010/I1-PAIR-CONSENT-BRIEF.md).
Không mở rộng kết quả này thành nghiệm thu toàn I, I2 hoặc A–S.

**Bàn giao I2 tại `814ea5c9` (10/10).** Mục lục, loại sổ, điều cần tránh,
Gu, xem trước đóng sổ và tem trạng thái kế thừa cùng thế giới. Full review
`a24cb4bf` mở 84 viewport tĩnh + 12 viewport tương tác, trả **fix** cho một
P2 ở cụm lỗi/phục hồi Gu. Verdict cuối `814ea5c9` chấm **F1 resolved / ship**
trên 12 frame Gu mới sáng100/tối200 và regression hẹp. Bảy target còn lại
giữ byte và bằng chứng a24; đây không phải full review mới hoặc nghiệm thu
toàn I/A–S. Hợp đồng của lát này nằm trong Layout/Components và
[I2 brief](docs/design/nep-ui-implementation/redesign-20261010/I2-NOTEBOOK-SETTINGS-BRIEF.md).

**Bàn giao GPL/J có phạm vi 11/10/2026.** Vẫn UI3/seed `c8e88116`, giấy,
mực, coral, Bricolage và Nếp canonical; không có comp được duyệt mới.
[GPL brief](docs/design/nep-ui-implementation/redesign-20261010/GPL-COMPACT-NATIVE-BRIEF.md)
và [J brief](docs/design/nep-ui-implementation/redesign-20261010/J-MOMENTS-ALBUM-BRIEF.md)
ghi hợp đồng native. Chỉ các mô tả Layout/Components gọi đúng GPL/J bên dưới
thắng bố cục cũ của những caller đó; không thay token hoặc toàn hệ.

Native `061d8d7a` chấm **F3 resolved / ship** ở thao tác sửa hồ sơ; F1/F4
và F2 có giới hạn diagnostic giữ verdict `7cf58966`. J chấm **P1/P2 resolved /
ship** tại `7cf58966`, các target J giữ nguyên byte đến061. Hai verdict chỉ
đóng finding đã chấm, không nghiệm thu toàn GPL, J, app hoặc main. Candidate797
và các mốc cũ vẫn giữ phạm vi/bằng chứng lịch sử của chúng.

## v3 «Sân khấu giấy» (ADR-0037, Lead 2026-09-24, làm xong 2026-09-25)

Mục này là hợp đồng hiện hành và **thắng mọi câu cũ bên dưới** ở chỗ hai bên nói khác nhau.
Phần còn lại của file vẫn đúng ở chỗ mục này không nói tới: màu, chữ, tương phản, washi,
con dấu. Kế hoạch và nhật ký: `docs/architecture/04-ui-v3-san-khau-giay.md`,
`docs/claude/2026-09-25/san-khau-giay/README.md`.

**Mỗi việc là một vật giấy, không phải một tờ điền chữ.** Bảng dưới là primitive cho mỗi việc;
màn mới phải dùng primitive có sẵn trước khi tự vẽ (`tests/suc-song-man-tao.test.mjs` gác):

| Việc | Vật | Primitive |
|---|---|---|
| Khay «Tạo mới», khay công cụ chat | vật ký hoạ trên bàn («Viết bài» là `bai-viet`: trang viết dở nghiêng + bút chì coral, từ 02/10 thay `phieu-bau` mượn của bình chọn) | `art/vat-ban.ts` + `ui/art/VeLop` |
| Kèo | thiệp dán washi, xem trước là vé | `ChonNgayLich`, `TheVe`, `StampButton` |
| Chia bill | hoá đơn nhiệt, bàn pop-up, cuống phiếu | `HoaDonGiay`, `BanGanMon`, `CuongPhieu` |
| Sổ, đợt thu, tài chính | trang sổ kẻ dòng | `TrangSo` / `DongSo`, `DaiTienDo` |
| Quyết toán | mũi tên mực tự vẽ, không mang số | `SoDoChuyen` |
| Sổ hai người | bìa sổ, giao kèo có chữ ký, tờ bút chì | `SoBia`, `ChuKy`, `ToGiay`, `LaLich`, `BanXoay` |
| Nhóm mới, nhóm trên kệ | bìa sổ, gáy sổ theo màu chat | `SoBia` (`nhan`), `bangMauChat` |
| Mời, lời mời | phong bì | `PhongBi` |
| Kết bạn | danh thiếp, người là hình nhân | `HinhNhan` |
| Khám phá, Đi đâu | sân khấu thành phố, bưu thiếp | `art/thanh-pho.ts`, `SanKhau`, `SanThanhPho` |
| Lên plan | vé; kèo đã qua là cuống | `TheVe`, `CuongPhieu` |
| Tường, khoảnh khắc | ảnh in nghiêng có washi, instax | `KhungAnh` + `nghiengAnh`, `Washi` |
| Thành tích | tờ tem | `Tem` |
| Hồ sơ của mình và của người khác | trang hộ chiếu; tên người khác in bằng mực của họ | `DauLon co="nho"`, `mucNguoi` |
| Hành trình bản đồ | bản đồ giấy là mặt bàn; trang ngày xé khỏi sổ đặt đè lên, mép xé và lỗ gáy quay về phía bản đồ; ghim là con tem giấy đánh số mang trạng thái; kế hoạch là một nét mực coral, nháp là một nét chì đứt; đầu trang đóng dấu nét ấy là gì (chi tiết: «Hành trình bản đồ» dưới «Luật đã thay luật v2») | `NenGiay` + `hinhTrangXe`, `hinhTem`, `mocChum`, `lopDuong`, `kieuBanDo`, `veDenDau` + `useNetMuc` |
| Cài đặt nhóm | góc trang chat xem trước màu bong bóng; «Rời nhóm» tách xa dưới nét kẻ | `bangMauChat` |
| Thành viên | người trước, vai trò là dòng chữ phụ tự xuống hàng (H tại `d91e66d1`) | `AvatarNguoi`, `typography.note`, `LuoiNguoi` |

Sau lượt đọc mù 26/09 (ADR-0038):

- **Nút chưa dùng được phải nói vì sao, hoặc không hiện.** Không mờ bằng opacity nữa: `RudiButton` và
  `StampButton` tắt là viền đứt `lineStrong` trên `card`, chữ `inkSoft` (≥ 4,5:1, `test_contrast_floor.py`),
  và prop `lyDo` in lý do ngay dưới nút. Nút mà việc chưa có nghĩa thì không vẽ («Lưu tên» khi tên chưa đổi).
- **Mép Nếp là dải ruy băng đánh dấu trang** màu `accent`, đuôi chữ V (`hinhRuyBang`), trong đúng 10dp của
  ADR-0035. Kéo Nếp ra thì ruy băng mờ đi, mặt Nếp hiện.
- Câu chữ: tab «Lịch trình / Bản đồ»; «Người lập nhóm»; nhãn gu tiếng Việt; mức chi thứ tư «Trên 500K».
| Sở thích | bảng sticker (chọn là dán), mức chi là phong bì | `GuGlyph`, `StampButton` |
| Đăng bài, story | trang thư và bốn phong bì người đọc; polaroid 24 giờ | `ONhapMuc`, `NapGiay` |
| Bình chọn trong chat | giấy nhớ, mỗi phiếu là một dấu vân tay mực | — |
| Ô nhập | dòng mực, không hộp | `ONhapMuc` (`Field` chỉ còn ở màn chưa làm lại) |

**Luật đã thay luật v2:**
- «Nếp Đứng Xa Tiền» → **«Nếp Không Chạm Số»**. Nếp diễn đúng tám khoảnh khắc (M1 khay tạo, M2
  chụp bill, M3 ghi sổ, M4 tiền về, M5 tạo kèo, M6 sổ đôi mở, M7 gửi tờ, M8 huy hiệu mới), mỗi khoá
  sự kiện một lần, trong vùng riêng của bố cục (`NepDien` giữ chỗ 128/112/88/0dp). Nếp cách số
  tiền ≥ 16dp, không bao giờ ở lỗi hay xung đột; dock nhường chỗ khi Nếp trong trang hiện. Khung cuối
  của mọi tiết mục nằm trong hộp (`nep-roi.test.mjs`, kể cả `buoc-di`).
- «Trong / Trên Trang» → **Độ Cao Giấy 0–3** (`tokens.json` → `sanKhau.cao`, `bongGiay(level)`).
- Avatar mang **mực người** (`mucNguoi`, tám màu, FNV-1a theo person id). `AvatarNguoi` chuyền
  `personId` xuống, nên mọi avatar sống và tên người (chat, thành viên, bạn bè) cùng một mực.
- Chuyển động: bật dựng ≤ 420ms, tiết mục ≤ 1400ms, lật trang 300ms. Giảm chuyển động thì khung
  cuối tĩnh, cắt thẳng, không 3D. Control không xoay 3D.
- **Nền `paper` ở theme tối không phải mặt chữ** cho accent/warn/faint (khoảng 4:1). Vật mang chữ
  lỗi hay nút ghost dùng nền `card` (`chu-tren-giay.test.mjs`, không còn nợ).
- Ô nhập trên web tắt viền trình duyệt (`ui/khong-vien-web.ts`, `khong-vien-web.test.mjs`).
- Cảnh ký hoạ: mỗi cảnh đúng **một lớp cam** làm nguồn sáng; mặt giấy vẽ trước viền mực
  (`thanh-pho.test.mjs`, `giay-vat-the.test.mjs`).

**Hành trình bản đồ** (FINISH M7 bản đồ; đọc từ build `3a37be6b` và ảnh `.impeccable/review/*.png`, 29/09;
hợp đồng hướng đi ở đầu `hanh-trinh/ManHinhHanhTrinh.tsx`):
- **Nền bản đồ là giấy, im** (`kieuBanDo(toi)`, vector OpenFreeMap, mọi màu từ `tokens.json`): nền `ground`;
  khu dân cư, cây, công viên, nước là lớp `paperShade` đậm dần (0.22 → 0.4 → 0.6 → đặc); nhà `line` 0.55 từ
  z14. Đường **ba bậc**, mỗi bậc là dải giấy `card` trên mép bút chì: lớn (motorway/trunk) > chính
  (primary/secondary) > phố (từ z12.5), bậc trên luôn rộng hơn. Chữ nền chỉ tên đường, nước, phường/quận,
  thành phố; **không POI bên thứ ba**, không «Khu phố N». Địa điểm duy nhất trên bản đồ là của nhóm.
- **Con tem là trạng thái điểm hẹn** (`hinhTem`, web và native vẽ cùng một hình): tem vuông 44dp bo 8,
  nghiêng ±3° xen kẽ theo số, vùng bấm ≥ 48dp. *Sắp tới*: giấy `card`, viền bút chì `lineStrong` 2, số coral.
  *Điểm tiếp theo*: tô coral, số `accentInk`, nổi (Độ Cao Giấy 2). *Đã tới*: số và nét bút chì `inkFaint`
  cộng **tick vẽ** bằng mực (đường SVG, không glyph) trên nút giấy tròn 20 ở góc — nhạt bằng màu và dấu,
  không bằng opacity. *Đang chọn*: 52dp, viền coral 3, đứng thẳng, nổi. Thanh chặng trên trang lặp lại
  tem ở cỡ 26: cùng số, cùng trạng thái.
- **Tem gộp giữ trạng thái gấp nhất** (`mocChum`): các điểm trùng một chỗ thành một tem «1 · 3»; còn một điểm
  tiếp theo trong nhóm thì tem vẫn coral, «đã tới» chỉ khi tất cả đã tới, nhãn neo được giữ; chạm mở danh
  sách chọn.
- **Neo ngày**: dưới tem là nhãn giấy viền coral 1.5, chữ coral 12/800 in hoa, nghiêng -2: «XUẤT PHÁT»,
  «KẾT THÚC», «XUẤT PHÁT · VỀ».
- **Nét mực, nét chì** (`lopDuong`, một nguồn paint cho hai nền tảng): đường thật là **một nét coral liên
  tục** 5 (chọn 7) trên vỏ giấy `card` 10 (13), đầu tròn; chỉ khi một chặng khác đang chọn thì các chặng còn
  lại lùi về 0.42. Mũi tên chiều đi coral trên vỏ giấy ở một phần ba chặng. Nháp (geodesic) là **một nét chì
  đứt** `inkSoft` 2.5, gạch [1.2, 2.2], không số, không mũi tên; chặng nháp đi ngược đúng chặng nháp trước
  thì không vẽ lại — bút chì đi mỗi đường một lần.
- **Số phút là nhãn ở điểm giữa mỗi chặng thật** (giữa theo chiều dài, không theo hai đầu): Noto Sans Bold
  12 `ink`, quầng giấy 2.4, từ z10, được đè tên đường nền. Chặng nháp không có số. Chặng đang chọn thay
  nhãn bằng thẻ giấy nghiêng -2 («1,9 km · 3 phút» / «Rời HH:MM để tới lúc HH:MM»).
- **Đầu trang đóng dấu nét là gì** (`Stamp`): «ĐƯỜNG THẬT · XE MÁY» (phương tiện đang chọn) tông coral nghiêng
  -2; «NÉT NHÁP», «ĐANG TÍNH ĐƯỜNG», «CHƯA TÍNH ĐƯỜNG» tông mực, thẳng. Nét thẳng không bao giờ được đọc
  thành đường: khi còn chặng nháp, dòng số chỉ in «n điểm trên bản đồ», không km, không phút.
- **Một khoảnh khắc chuyển động duy nhất: «nét mực tự vẽ»** (`veDenDau` + `useNetMuc`). Chỉ đường thật; chỉ
  khi tuyến của ngày tới lần đầu hoặc khi gợi ý thay tuyến; mỗi tuyến một lần mỗi phiên (mở lại không diễn
  lại). Chờ bản đồ tải xong **và** điểm của kèo đọc xong, trễ 320ms cho camera fit, rồi mực chạy 1080ms (cả khoảnh khắc 1400ms, đúng trần) nhịp
  bút (cos vào–ra, không vượt) qua 24 bước, cắt theo tỉ lệ chiều dài cả ngày. Tem chưa tới lơ lửng 4dp trên
  bóng cao; mực tới thì hạ xuống và nén như `Stamp` (`useNhipDau`, không haptic) — nhấc lên, không phóng to.
  Mũi tên, nhãn phút, thẻ chặng đợi nét xong. Nét chì không bao giờ diễn. Giảm chuyển động: khung cuối ngay.
  Không ẩn nội dung: tem có mặt từ khung đầu.
- **Điện thoại**: bản đồ trên, trang ngày dưới; mép xé ở cạnh trên chồng 6dp lên bản đồ, lỗ gáy cách mép 13dp.
  Trang là `card` Độ Cao Giấy 2 (theme tối vẫn đọc được accent/faint). Đầu trang ba hàng: tên ngày + dấu nét;
  dòng số + «Thu gọn / Mở trang»; chọn phương tiện. Chỉ phần giữa cuộn, và trên native cao cố định
  (max(92 × font ≤ 1.3, 20% chiều cao), không bao giờ thấp hơn một hàng thanh chặng) để camera không lệch khi
  chọn; web giới hạn cả trang 60%. Mép dưới phần cuộn mờ 40dp vào `card`, nút bị cắt đọc thành «còn nữa».
  Chặng là **thanh cuộn ngang**; ô chặng không phải thẻ, chỉ ô đang chọn có nền `accentSoft` + viền coral.
- **Màn rộng** (≥ 840dp, font < 1.8): trang 360dp bên phải bản đồ, mép xé và lỗ gáy ở cạnh trái quay vào bản
  đồ; chặng là **hàng kẻ tóc** `line` của trang sổ, tên hai dòng; không mép mờ.
- **«Khớp hành trình» là con tem 44dp**, không nhãn chữ, góc trên trái bản đồ: giấy `card`, viền bút chì 2,
  bo 8, nghiêng -2, icon mực `scan-outline` 22, nổi; bấm thì hạ về Độ Cao 1. Camera fit chừa 64 trên/dưới,
  40 hai bên để tem không nằm dưới nút hay dòng bản quyền.
- **Miếng giấy bản quyền** (native): giấy `card` bo 4 góc trên phải, chữ bút chì `inkSoft` 11 «©
  OpenStreetMap · OpenFreeMap», bấm mở trang bản quyền OSM; không logo, không nút «i» hệ thống. Web tắt xoay
  nên không có la bàn; native chỉ hiện la bàn khi đã bị xoay.
- **Còn mở, chưa xong:** bản đồ mới chiếm ≈34% khung đầu trên điện thoại (mục tiêu ≥ 45%); nhãn phút có thể
  đè tem và nhãn neo trên điện thoại ở zoom fit (`native-412-sang`: «4 phút» dưới tem 3, «phút» dưới tem 2);
  iOS chưa xem; web còn dùng nút bản quyền thu gọn mặc định của MapLibre (có «i») thay cho miếng giấy.

**Vẫn cấm:** confetti và hạt bay, toast, modal lỗi, hero metric, thẻ lồng thẻ, nút lồng nút, animation
lặp vô hạn ngoài Skeleton. Chỉ một vật bay một lần (thư M7).

**Creative North Star: "Nhật ký chuyến đi sau giờ làm"**

Một cuốn sổ chuyến đi cả hội cùng viết trong một buổi tối. *Bìa* vải indigo
là bề mặt thuyết phục (Welcome, `CoverBand` đầu Login/OTP); *trang giấy*
trắng ngà có vân là bề mặt làm việc (đêm sổ đóng lại: nền mang vân vải, hình vẽ
là tờ giấy đêm `paper`), và trên giấy **hàng nằm thẳng trên trang,
ngăn bằng kẻ tóc**, không có thẻ lồng thẻ. Ba tông bão hoà mang nghĩa (cam =
lời rủ, teal = tiền, tím = AI) chỉ dán lên **vùng đang quan trọng**, và trên
màn tiền tông teal chỉ đậu trên **con số**, không tô cả khối. Trạng thái đúng
là *con dấu* có chữ; bản nháp là *nét chì đứt*; kế hoạch là *một đường mực
liên tục*; ảnh là *bản in* dán vào trang, có dòng xuất xứ; tiền là *dòng sổ*;
thứ máy sinh ra là *tờ giấy ký ở chân* «Rủ Đi AI gợi ý». Khung kẻ in trước,
màu đổ sau khi có dữ liệu. Lưới 4pt, snap ô nguyên. Dấu vết thế giới kế thừa ở mục này và FORM của
`docs/design/nep-ui-implementation/redesign-20261010/I1-PAIR-CONSENT-BRIEF.md`
(I2 tiếp tục tại `docs/design/nep-ui-implementation/redesign-20261010/I2-NOTEBOOK-SETTINGS-BRIEF.md`,
cùng seed `c8e88116`, hướng số 6); `apps/mobile/app/_layout.tsx` hiện không còn
contract lịch sử. ADR-0020 là thẩm quyền.

**Cập nhật 27/09/2026, ADR-0037:** chất liệu giấy, mực, coral và Bricolage
được giữ nguyên trong lát sổ kỷ niệm. Ẩn dụ «cả hội cùng viết» không có nghĩa
sổ trên tường thuộc chung: mỗi thành viên giữ bản riêng. Nếp là bạn đồng hành
kể chuyện trong Rủ Đi, không đổi tên ứng dụng. Hội bạn từ hai người và Cặp đôi
có đồng thuận dùng cùng hệ; các nhắc tới «sổ hai người» ở mốc 12/09 bên dưới
là lịch sử triển khai, không phải chế độ thứ ba hiện hành.

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
  trên coral); ở scheme tối `ground` mang vân vải (≈ 8 mức trên #151830),
  giấy chỉ còn là tờ `paper` của hình vẽ (11/09).
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
- **Giấy** (`ground` #f7f3ec / #151830): nền trang; sáng có `Grain giayTrang` 0.45, **tối có `Grain vaiBia` 0.30**
  (sổ đóng trên bàn, 11/09).
- **Giấy đêm** (`paper` #ffffff / #2e335c, `paperShade` #e6dfd3 / #181b36): tờ giấy
  của **lớp hình** — Nếp, cảnh, sticker, ô giấy loại nơi — và bóng gấp của nó;
  sáng trùng `card`/`line`, tối sáng hơn nền 13.5 bậc L* với bóng thấp hơn mặt
  11.8 bậc. Không dùng cho thẻ hay chữ.
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
lỗi, không phải lựa chọn. Welcome/Login/OTP dẫn bằng cam (con dấu; Welcome
thêm chỉ báo trang coral);
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
và `VeLop.mauLop` mới đổi vai ra token của scheme: `giay → paper`, `bong → paperShade` (11/09; trước là `card`/`line` — xem
«Bản tối là sổ đóng trên bàn» ở Do/Don't), `muc → ink`, `gap → accent`, `mo → accentSoft`, `split → split`, `ai →
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
| `paper` #ffffff trên `ground` #f7f3ec | Tờ giấy vẽ (Nếp, cảnh, sticker, ô giấy) trên nền trang | **1.11:1** | trang trí |
| `paperShade` #e6dfd3 trên `paper` #ffffff | Bóng gấp trên tờ giấy vẽ | **1.32:1** | trang trí |

### Chế độ tối

| Cặp | Vai trò | Tỉ lệ | Ngưỡng |
|---|---|---|---|
| `lineStrong` #7d82a9 trên `ground` #151830 | Viền control trên nền trang | **4.68:1** | 1.4.11 |
| `lineStrong` #7d82a9 trên `card` #1f2340 | Viền control trên thẻ | **4.11:1** | 1.4.11 |
| `coverLineStrong` #9095c0 trên `cover` #0f1126 | Viền control trên bìa sổ | **6.42:1** | 1.4.11 |
| `line` #363b5e trên `ground` #151830 | Cạnh thẻ trên nền trang | **1.61:1** | trang trí |
| `line` #363b5e trên `card` #1f2340 | Đường kẻ trong thẻ | **1.42:1** | trang trí |
| `coverLine` #2e3255 trên `cover` #0f1126 | Đường kẻ trên bìa | **1.51:1** | trang trí |
| `paper` #2e335c trên `ground` #151830 | Tờ giấy vẽ (Nếp, cảnh, sticker, ô giấy) trên nền trang | **1.45:1** | trang trí |
| `paperShade` #181b36 trên `paper` #2e335c | Bóng gấp trên tờ giấy vẽ | **1.40:1** | trang trí |

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
và 1.3 (ảnh trong `docs/archive/claude/2026-09-05/`). Body giữ system là **quyết định**
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
- **H1** (ExtraBold, 28/34, -0.65): tiêu đề màn. Tiêu đề pager Welcome tại
  `34d57ca9` dùng cùng face ở 34/40, -0.7; cửa sổ dưới 360dp dùng 30/36.
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

**I1 — chữ ký phải đọc được (`ChuKy`).** Giao kèo bật `wrapName` và
`showStatus`: vai trò `caption inkSoft` đứng trên, tên Bricolage ExtraBold
(20/26sp) không nghiêng giả, hoặc câu chờ `note inkSoft`, rồi nét ký/nét đứt
và chữ «Đã ký»/«Chưa ký». Tên và câu chờ không giới hạn một dòng; hai phần
ký giữ vai trò riêng dù xếp dọc. Nét vẽ chỉ trang trí, trạng thái đồng thuận
phải có chữ. Hai prop này là lựa chọn của caller giao kèo, không đổi cách
hiển thị mặc định của mọi `ChuKy`.

## Layout

**GPL/J — một cột cho cùng một việc (11/10/2026).** Chat giới hạn header,
luồng đọc, tools/plan/poll và composer trong cùng cột giữa (640dp), nền vẫn
phủ toàn màn; phone giữ trục +/pill/gửi. Khay tools/poll thay vùng soạn,
plan giữ ngoại lệ quay lại composer/draft đã chốt. Composer bài viết wide
nối vùng viết, counter và toolbar thành một vùng liên tục, không chèn thêm
khoảng trống sau toolbar. Đây là sửa F1/F4 được nhìn thấy ở native7cf,
không thay `RONG_COT` hoặc mặc định mọi form.

**Composer và hàng tìm địa điểm — cục bộ tại `e338740b` (13/10/2026).**
Composer bài viết cho content/stage/writing/body nhận phần cao dư ở mọi
chiều rộng (`flexGrow: 1`, `flexShrink: 0`, basis tự nhiên), thay nhánh chỉ
grow ở wide. Khoảng trống thuộc canvas; counter theo cuối vùng viết, rail
công cụ theo sau. Bỏ đệm cuối body, dùng khe đầu footer của shell; không
ghim MediaPicker/MentionPicker/Field vào footer. Khi phần thêm mở hoặc chữ
lớn, nó cuộn trong thân; không ép toàn nhóm vào màn đầu. Giải thích duyệt
bài vẫn cạnh audience, lý do/lỗi đăng vẫn cạnh quyết định đăng. Đây là bố
cục của caller Composer, không là mặc định mọi `RudiScreen`.

Khám phá › Địa điểm giữ ô tìm là chính và nút hỏi AI bên cạnh: hàng stretch
với gap8, nút AI rộng48, chiều cao auto/min48 theo hàng. `IconButton` nhận
style opt-in tại caller; các caller không truyền style vẫn giữ ô48×48.
Không thêm token, font, world hay raster mới. Đối chiếu built artifact và
phạm vi ảnh e338 ở [brief Composer](.impeccable/surfaces/apps-mobile-src-rudi-community-composer-tsx.md)
và [brief hàng tìm](.impeccable/surfaces/src-rudi-screens-explore-explorelive-tsx-5ec3650c.md).
Verdict độc lập e338 là **FIX**: F1 đạt ở dock thu gọn, F2 đạt ở canvas
rỗng, S1 đạt ở hàng tìm trong các scale/theme/window đã chụp; F3 còn một
phần vì chưa có nội dung đã nhập, Mention và phục hồi qua IME. FINISH còn
mở; đây là đối chiếu lịch sử e338, không phê duyệt đẹp/ship toàn app hoặc
chứng cứ cho sửa tab Khám phá sau đó. Không nâng kích thước cục bộ hay sai
lệch có sẵn thành luật hệ thiết kế.

Composer Khoảnh khắc J dùng cùng cột đọc (640dp) cho ảnh/caption, privacy
và footer audience/action/helper; không giữ body640/footer560. Khi IME đóng,
footer lấy inset đáy thiết bị cộng khoảng thở (12dp); khi mở giữ nhánh IME
của shell. Native7cf chấm helper ở trên gesture indicator và field/action
ở trên IME200. Cột và inset này thuộc caller J, không là công thức mới cho
mọi footer. Wide (768×1024) là cùng emulator5558 đổi kích thước, chưa phải
bằng chứng từ tablet độc lập.

**I1 — người trước, cảnh sau (`7dcacedf`).** Picker giữ cột form (560dp),
người thành hàng ngang có hình nhân, tên tự xuống hàng và chevron; thêm bạn
là icon quiet (48dp) ở header. Trang chưa mở sổ giữ cột đọc (640dp): hai tên
`h2` cùng mực `ink`, chữ «và» và lời mở `body inkSoft`, tất cả theo trục trái.
Cảnh bìa/Nếp hỗ trợ phía sau cụm người, không lặp tên trong nhãn bìa nhỏ.
Từ rộng 600dp và font dưới 1.3, cụm người và cảnh thành hai vùng cạnh nhau;
ở chữ lớn trở lại một cột. Phone giữ cửa vào giao kèo ở footer ngoài vùng
cuộn, `footerInset = max(insets.bottom, 12)`; màn rộng đặt nó sau cụm mở sổ.

`BacDongY` dùng chung cho Lập sổ/Một đôi: thân giao kèo cuộn riêng, footer
giữ phản hồi/lỗi, con dấu và «Để sau» theo đúng thứ tự đọc. Sheet giữ đệm
dưới `max(insets.bottom, 16)` của kit, không kéo tấm lên hết trần chỉ để đặt
hai lựa chọn. Quyền rồi giới hạn xếp dọc khi rộng dưới 600dp hoặc font từ
1.3; cột dọc stretch hết bề ngang, bỏ flex-basis của cột ngang để không co
mất phần chữ. Đây là hợp đồng của I1, không áp hình học này cho mọi sheet.

**I2 — mục lục theo nghĩa, phục hồi tại hành động (`814ea5c9`).** Cấu hình
sổ (loại, điều cần tránh, Gu khi được phép) thành nhóm đầu; tin nhắn/kỷ niệm
thành nhóm thứ hai; đóng sổ đứng riêng sau kẻ tóc và khoảng cách. Hội bạn
giữ tiêu đề, lời dẫn, con dấu rủ và «Chọn loại sổ» cùng trục trái. Nội dung
dài tự wrap; chữ lớn cuộn thân sheet, không thu font để ép vừa một viewport.
`RangBuoc` opt-in `avoidKeyboard`/`prioritizeFooter`: thân nhập cuộn riêng,
lỗi, Lưu và Bỏ thay đổi ở footer trên IME. Riêng Gu, khi có lỗi mới đưa câu
lỗi và chính các nút lệnh vào footer có hairline; own state và đầy đủ phạm
vi chia sẻ vẫn ở thân cuộn trước hành động. Trạng thái thường giữ nút trong
cụm của mình, trước phần tham khảo. Hai lựa chọn footer này thuộc I2,
không áp `prioritizeFooter` cho mọi sheet hay thay bố cục giao kèo I1.

**Header/form ở candidate 797 (10/10).** `TopBar` giữ target icon 48dp,
`minHeight` 52dp; hàng header mở vào gutter 12dp mỗi bên và chevron dịch
−7dp trong target, để nét back thẳng lề nội dung. Hai slot hai bên cân bằng
khi tiêu đề đủ ngang; nếu không, mỗi bên lấy bề rộng tự nhiên tối thiểu 52dp
để dành chỗ cho chữ. Tiêu đề dùng `title`, tối đa hai dòng, không thu font.
Ảnh `after-light100-profile-edit` đã mở có «Chỉnh hồ sơ» một dòng. Đây không
phải tuyên bố đo native toàn bộ target, geometry hoặc contrast.

`RudiScreen avoidKeyboard` ở form hồ sơ/soạn bài giữ header và footer ngoài
vùng cuộn; source đo viewport form sau resize để đưa ô đang focus vào vùng
nhìn thấy. Ảnh hồ sơ nhập thành phố đã mở có field và nút Lưu trên IME.
Footer hồ sơ trên phone dùng đệm 24dp khi đóng bàn phím, 8dp khi mở; thanh tab
đã chiếm chiều cao trong normal flow. Sửa 797 bỏ phần cộng lại chiều cao tab
vào footer từng tạo khoảng trống ở 9a. Con số 24dp thuộc bố cục hồ sơ này,
không phải token mới hay công thức safe-area cho mọi màn. Large text dùng
`chuLon(fontScale)` từ 1.28 để bao bước Android 1.3; những đổi cột cụ thể nằm
trong mục chín bề mặt, không thay ba size class lịch sử.

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

`heightClass = short` dưới 480dp (máy nằm ngang hoặc IME đè sheet).
Welcome tại `34d57ca9` còn chọn bố cục ngắn khi chiều cao sau safe area dưới
660dp hoặc `round(fontScale × 100) ≥ 130`: chữ, cảnh và hành động xếp dọc
trong vùng cuộn; cảnh vẫn hiện ở cao 144dp. `CoverBand compact` rút vải trên
dưới khi bàn phím mở ở compact (Login/OTP).

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

**Bề mặt** (`RudiScreen surface`): `page` = nền `ground` + `Grain giayTrang` 0.45
(sáng) hoặc `Grain vaiBia` 0.30 (tối, sổ đóng — `ui.tsx`),
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

**Cột đọc của `RudiScreen` (`cot`, 01–04/10, QA UI-093, UI-047).** Ở mọi
`sizeClass` khác `compact`, nội dung, `header` và `footer` của màn giữ
**cùng một cột** `maxWidth = RONG_COT[cot]` (`ui.tsx`): `doc` **640** cho
trang để đọc (tờ giấy, bài, hồ sơ, album, thành viên, bạn bè), `form`
**560** cho form (trang cuối sổ chuyến đi, soạn bài Cộng đồng), `rong`
**960** (mặc định) cho lưới và danh sách rộng. Trên điện thoại cột là cửa
sổ. Header đi cùng cột nên tiêu đề không lệch khỏi nội dung dưới nó. Sheet
có trần bề rộng riêng (640, căn giữa; xem `Sheet`). **Lưới trong cột đo cột,
không đo cửa sổ**: `LuoiNguoi` chỉ xếp hai cột khi chính danh sách rộng đủ
2 × 280 + 16 (xem Components).

**Khe lớp của màn (`KheLop`, `LenLop`).** Ngoài `overlay`, `RudiScreen` giữ
một khe lớp vẽ **trên cả màn** (`ui/KheLop.tsx`): component nằm sâu trong
thân màn bọc lớp phủ của nó trong `<LenLop>` thì lớp ấy lên khe, phủ cả
header, mà vẫn giữ state và callback của component đã vẽ nó (trình sửa ngày
cạnh bản đồ của `hanh-trinh/SoHanhTrinh.tsx`; trước đó scrim dừng ở mép trên
thân màn và header vẫn sáng, bấm được: QA UI-041). Không có host (trang lab,
test) thì con ở yên chỗ cũ.
**Welcome** (`34d57ca9`): lề ngang 24, cột bìa `maxWidth` 688, vùng nội dung
`min(windowWidth - 48, 640)`, khối đáy `maxWidth` 640 ở medium+; đầu bìa là
`Logo compact` với wordmark cao 18. Bố cục ngắn giữ Nếp canonical 96dp bên
mép giấy và cảnh 144dp, không xoá artwork để dành chỗ cho chữ lớn.

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
Không có nút back thì ô trái là **wordmark trơn** (`Wordmark` `ink` cao 18).
Khám phá là ngoại lệ có tên từ 02/10: đầu tab là hàng hai mục `DauKhamPha`
(xem Navigation), không wordmark; wordmark ở đầu rail trên tablet. Ô icon app gradient chỉ còn ở Welcome/Login,
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
| Giấy | `giay-trang.png` | 0.45 (chỉ nền sáng) | ≈ 2 mức trên `ground` sáng; **ở nền tối 0.30 đo 0.8–2.1 = phẳng, đã bỏ (11/09)** |
| Vải trên nền tối | `vai-bia.png` | 0.30 | ≈ 8.1–8.6 mức trên `ground` tối (Khám phá, khay, màn lỗi — `docs/archive/claude/2026-09-11/toi-giay-tren-vai/`) |
| Mực dấu | `muc-in.png` | 0.26 | ≈ 8.6 mức trên coral; ô giấy ở đây đo 2.1 nên có ô riêng |

Ô trắng đen trung bình trung tính nên trên nền **cỡ trung** màu token bên dưới đo
vẫn đúng trong một mức, và bảng tương phản áp cho bề mặt có vân. **Ngoại lệ đo
được (11/09, finish review PR bản tối):** trên nền tối nhất của app, ô vải (xám
127.5, alpha 45/255) ở 0.30 **nâng** `ground` tối #151830 lên **#1c1f36** đo trên ảnh
(+7 mức xám, L* 9.2 → 12) ở cả ba màn đã chụp. Số bảng tối tính trên token vì thế cao
hơn thực ~8%: trên nền thực `ink` 14.34, `inkSoft` 9.21, `inkFaint` **5.87** (p99 của
nền có vân: 4.97 — vẫn qua sàn 4.5), `accent` 5.54, `ai` 8.35, `split` 8.73, `warn`
5.38, `lineStrong` **4.34** (viền control vẫn qua 3:1); `card` trên nền thực **1.06:1**
(token 1.14) — thẻ tối **phải** giữ viền `line`, fill không còn là một bậc sáng;
`paper` 1.34 (token 1.45). Không hạ opacity vải hay đổi `paper` để «bù» số: quan hệ
mặt/nền +13.5 L* và bóng gấp −11.8 L* là cái làm tờ giấy có thân.

### Shadow Vocabulary
- **Bản in** (`cardShadow`: iOS `#5A3014` 0/8, đục 0.1, mờ 18; Android
  `elevation: 3`): chỉ `KhungAnh` và `Card` v1 còn sót; `Card` v1 ship **cả**
  viền `line` lẫn bóng vì elevation 3 gần như không thấy trên giấy và ở
  scheme tối không tách được gì. *Lịch sử tới 10/09:* tài liệu ghi «không màn
  nào còn gọi `Card`» trong khi họ màn Cài đặt gọi 14 lần (8 ở
  `CaiDatScreen`) — tái audit Codex 10/09 R5 thấy hai hệ bề mặt cạnh nhau.
  **Hiện hành (11/09):** họ Cài đặt là hàng trên giấy (`NhomHang`); `Card`
  còn đúng hai người gọi có tên là nợ: `story/DangStoryScreen`,
  `tuong/BaiChiTietScreen`. Cổng:
  `node --test tests/rudi-khong-card-trong-cai-dat.test.mjs`.
- **Con dấu «Tạo mới»** (`elevation: 6`, 0/6, đục 0.22, mờ 10, màu `accent`):
  thứ duy nhất nổi trên thanh tab; vòng 4px màu `ground` tách nó khỏi thanh.
- **Scrim sheet** (`lopPhu.toi(0.42)`): lớp phủ ấm gần đen, không xám.
- **Tờ giấy AI** (`ToGiay` trong `TheAi.tsx`, khung `aiSheet` trong Group):
  nền `card`, viền 1px `line`, bo `base`, **không bóng**; một tờ giấy đặt lên
  trang, ký ở chân. Nhịp của tờ (11/09, tái audit R5 «khối AI dài và nặng»):
  tiêu đề cỡ `title` — cỡ một tin nhắn, không phải một màn — quyết định
  đứng trước, lý do sau **một cửa mở**, trạng thái nháp nói một lần bằng badge.

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
- **Tờ giấy gấp ba** (`ui/ToGiay.tsx`, 12/09): giấy `paper`, **mép tóc
  `lineStrong`** (không `line`), bo **`small` 10** như mọi tờ giấy trong app
  (`KyHoa`, `KhungAnh`); khi là tờ dẫn (`dan`) thì **góc trên phải vuông** và
  bị **cắt** thành góc gấp 22dp (`GOC_GAP`, cùng cỡ góc gấp bản in). Không
  bóng. Chi tiết và lý do từng số ở mục «Tờ giấy gấp ba» trong Components.
- **Kẻ**: mọi divider là `StyleSheet.hairlineWidth` màu `line` (dưới hàng
  địa điểm, dòng sổ, hàng bình chọn, cạnh trên thanh tab, cạnh phải rail,
  trên/dưới ghi chú AI). Không có viền trái màu dày hơn 1px.
  Vết gấp `VetGap` của tờ giấy là hairline `paperShade` chạy mép tới mép,
  **không phải divider** (mục «Tờ giấy gấp ba»).

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
`PressScale`, `PhotoViewer`; đợt nâng cấp 01–04/10 thêm `CauTaiCho`,
`LuoiNguoi`, `KheLop`/`LenLop`, và ghi lại `ONhapMuc`, `ChonNgayLich`), cùng
hai hook lớp phủ `useLuiLop` và `useGiuKhiDong` (`ui/giu-khi-dong.ts`) và
một module chữ ngày `src/rudi/ngay-viet.ts`. Hai hàng feature dùng lại nhiều nơi:
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
| Một việc vừa bấm không thành | `CauTaiCho` ngay dưới control đó (hoặc trong `footer` ngay trên nút) | toast, modal lỗi, câu `warn` tự viết ở đầu trang |
| Hành động xoá/bỏ thứ người ta đã làm | `RudiButton tone="warn"` `outline`/`ghost`, rồi **hỏi tại hàng** | nút cam đặc, hộp thoại xác nhận |
| Nút chưa dùng được | `disabled` + `lyDo` (viền đứt, lý do in dưới) | nút mờ không lời |
| Danh sách người có hành động trên mỗi hàng | `LuoiNguoi` | một hàng 590dp kéo nút xa khỏi tên |
| Lớp phủ vẽ từ sâu trong màn | `<LenLop>` (lên khe lớp của `RudiScreen`) | `Sheet` trong thân màn (scrim dừng ở mép header) |
| Một ngày, một giờ | `ngay-viet.ts` | `toLocaleDateString`, `Intl`, chuỗi tự ghép |

### Buttons
- **Con dấu CTA (`StampButton`)**, nút chính của bề mặt thuyết phục: tô
  `brand.coral`, vành 2px mực 0.88 theo `duongVienDau`, `Grain mucIn` 0.26,
  nhãn Bricolage Bold màu `mauSang.ink` +0.4; cỡ `lon` cao tối thiểu 60, đệm
  ngang 30, chữ 21/26, bo 16 (Welcome `34d57ca9` «Rủ Đi thôi!» `tilt={0}`,
  rộng hết cột); cỡ `vua` cao
  52, đệm 24, chữ 17/22, bo 14 (Login «Gửi mã»). Không icon, không vành
  trong, không bóng, mặc định rộng bằng chữ. Nhấn co 0.97 kèm haptic `impact`;
  `loading` thay bằng spinner mực trong hàng và khoá nút; `disabled` mờ 0.55.
  **I1 tại `7dcacedf`:** `wrapLabel` là opt-in cho con dấu bị giới hạn bề
  ngang: caller chặn `maxWidth: "100%"`, nhãn `flexShrink: 1` căn giữa,
  thân thêm đệm dọc (12dp) để tự cao theo chữ hệ thống. Giữ cỡ chữ và mặt
  dấu; không biến mọi con dấu thành thanh toàn ngang. «Để sau» và «Chọn loại
  sổ» dùng `label inkSoft` trên target tối thiểu (48dp), nhãn stretch trong
  chính target để đủ chữ; không tạo điểm coral thứ hai.
- **Nút bìa (`CoverButton`)**: `outline` trong suốt, viền 1px
  `coverLineStrong` (5.19:1 sáng / 6.42:1 tối trên `cover`), bo 14, cao 50,
  đệm 18, nhãn `label` `coverInk`, icon 18, haptic `select`. `link`: không
  viền, mặc định rộng bằng chữ, cao tối thiểu 48, đệm 12; là lựa chọn phụ
  **dưới** con dấu. Welcome `34d57ca9` dùng `link full`: hàng rộng hết cột,
  nhãn `flex: 1` căn giữa lấy phần còn lại bên cạnh chevron 18, giữ đủ
  «Tìm hiểu thêm» / «Xem lại từ đầu» cả khi tăng chữ. `full` không thêm viền;
  link ở màn khác giữ chiều rộng theo chữ khi không truyền tuỳ chọn này.
- **Nút giấy (`RudiButton`)**: cao 52 (48 `compact`), bo 14, nhãn `label` một
  dòng, icon 20; `solid` = nền `<tone>` phẳng và nhãn `<tone>Ink` theo
  scheme (album «Thêm khoảnh khắc» accent, lịch trình «Dùng plan này» ai);
  `outline` = nền `card` viền `lineStrong` chữ màu tông với `accent`, viền màu
  tông với `split`/`ai` («Đánh dấu đã trả» teal, «Chỉnh lịch trình» tím) —
  ngoại lệ duy nhất: `outline ai` đứng cạnh `soft ai` trong tờ AI của chat
  giữ viền `lineStrong`, tông chỉ ở chữ (xem «Nhịp của tờ AI trong luồng
  chat»);
  `soft` = nền `<tone>Soft`; `ghost` trong suốt. Nhấn **co 0.98 bằng lò xo
  trên UI thread** (`PressScale`), không còn mờ 0.82. Hai nút chân đứng cạnh nhau trong `footer`, outline trái,
  solid phải.
  - **Tắt (`disabled`, ADR-0038 §2.2)**: không mờ opacity nữa (dòng cũ
    «`disabled` 0.45» đã hết hiệu lực): nền `card`, **viền đứt 1.5
    `lineStrong`**, nhãn và icon `inkSoft`, ở mọi `variant`. `lyDo` in lý do
    **ngay dưới nút** (icon `information-circle-outline` 16 + `caption
    inkSoft`, ẩn khỏi cây trợ năng) và trao câu ấy cho trình đọc làm
    `accessibilityHint` của nút. Tắt mà thiếu `lyDo` thì bản dev
    `console.warn` một lần mỗi nhãn, và **cổng** `tests/nut-tat-co-ly-do.test.mjs`
    quét mọi `RudiButton`: điều kiện tắt có vế kéo dài (không chỉ «đang chạy»:
    `busy`, `ban`, `dang…`) thì phải có `lyDo`, ngoại lệ ghi tên và lý do. Nói
    vì sao, hoặc đừng vẽ nút. Đang
    `loading` thì nút giữ mặt của nó (spinner màu chữ), không thành viền đứt.
  - **Tông `warn` = hành động phá huỷ** («Xoá cuốn sổ», «Bỏ bản phác», «Rời
    nhóm», «Chặn», «Xoá tin», «Xoá vĩnh viễn»): **chỉ `outline` hoặc
    `ghost`**, chữ (và viền outline) màu `warn`. Palette không có nền warn:
    gọi `solid` hay `soft` với `warn` thì kit vẽ `outline`. *Luật đo được*:
    `tests/nut-pha-huy-tong-warn.test.mjs` quét mọi `<RudiButton label=…>`
    trong `src/rudi` mở bằng «Xoá/Xóa, Rời nhóm, Chặn, Bỏ bản/nháp/tờ/thứ
    tự, Huỷ buổi, Thu hồi» và đỏ khi thiếu `tone="warn"`; ngoại lệ có tên
    kèm lý do (`EndingScreen` «Bỏ trang này»: trang còn thêm lại được); phép
    quét tự kiểm đã thấy ≥ 10 nút. Không chứng minh: nút vẽ bằng
    `Pressable` hay nhãn dựng lúc chạy.
- **Nút back** trên bìa: `PressScale` 48×48 co 0.92, mặt tròn là View con
  (nền `coverInk` 0.08, chevron 26); trên giấy `TopBar` dùng chevron `ink`
  trong ô 48.
- **`IconButton`**: 48×48 bo 16; `quiet` không viền không nền (nút «+» góc
  phải album, «Chọn ảnh»); có viền `lineStrong` khi đứng cạnh chip (nút lọc
  Khám phá). Nhấn co 0.94 (`PressScale`, cùng số với FAB). `tron` (02/10): bo
  24, thành đĩa tròn, **chỉ cho control đặt TRÊN ảnh** (tim của thẻ dẫn Khám
  phá), không cho control đặt trên giấy.
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
- **Chip đổi nội dung (`vaiTab`)** (02/10; **từ 03/10 không màn nào gọi**):
  prop vẫn còn trên `Chip` (cùng hình chip bấm được, `role="tab"` +
  `aria-selected` trong `tablist` của nơi gọi thay cho `button` +
  `aria-pressed`, chọn giữ dấu check và nền `<tone>Soft`), nhưng nơi gọi duy
  nhất, các bảng tin Cộng đồng, đã sang hàng chữ-tab `HangChuTab` (xem
  Navigation). Bộ lọc thu hẹp danh sách vẫn là chip thường.
- **Chip trả lời một câu hỏi chọn một (`vaiRadio`)** (04/10, B11): cùng hình
  chip bấm được, `role="radio"` + `aria-checked` trong `radiogroup` của nơi
  gọi, Space bấm trên web (`toggleState`). Dùng cho «Ai được bình luận tường
  tôi» (Cài đặt, hồ sơ của mình); trước đó là `button` + `aria-pressed`
  trong một `radiogroup`, công nghệ hỗ trợ đọc ba nút rời.
- **Chip tĩnh** (không `onPress`): cao 30, bo 10, không role; kit còn giữ
  nhưng **đợt này trạng thái đi bằng `Stamp`**; chip tĩnh chỉ cho thẻ phân
  loại không phải trạng thái.
- **Huy hiệu demo (`DemoBadge`)**: viền `line`, chữ 10/700 `inkFaint`, icon
  `flask-outline`, bo pill; mặc định «Dữ liệu demo», trong `TopBar` rút về
  `compactLabel` («Demo», «Nháp»); **render rỗng ở chế độ live**. Welcome
  `34d57ca9` không dùng `DemoBadge`; «Bản trải nghiệm» là nhãn ở mốc cũ.

### Con dấu trạng thái (`Stamp`)
Chữ `stamp` (CondensedBold 12 IN HOA +0.8), viền 2px màu tông, bo 6, đệm 8/4,
cao tối thiểu 26, `alignSelf: flex-start`, `accessibilityLabel` = nhãn, không
role, không press. `outline` mặc định (chữ màu tông); `ink` tô đầy với chữ
`<tone>Ink`, hiếm: một trạng thái quan trọng nhất mỗi màn. Con dấu **luôn
mang một chữ**, màu chỉ là tông của chữ đó: «ĐÃ TRẢ», «NGƯỜI THU BILL» (teal,
quyết toán), «HỢP GU» (tím, trên ảnh dẫn nghiêng -2 và trong hàng địa điểm —
từ 11/09 chỉ in khi địa điểm **không có dòng lý do**: hàng live không có
`reason` từ máy chủ, bìa chi tiết; `dauCon()` trong `HangDiaDiem.tsx`),
«CÒN 3 NGÀY» / «HÔM NAY» / «ĐANG ĐI · CÒN 2 NGÀY» / «ĐÃ QUA» (nhịp kèo, tính
từ `starts_on`/`ends_on` máy chủ qua `keo/nhip-keo.ts`, không từ chuỗi
fixture; nghiêng -2 ở Cá nhân), «ĐÃ TỚI» (teal, nghiêng -2, điểm danh). Ship
trên 12 màn.

**Opt-in I2 (`ToLoiRu`, nguồn a24 giữ nguyên tại814).** Tem trạng thái dùng
nhãn in hoa tiếng Việt, `minWidth` (88dp) và `labelStyle` stretch/căn giữa
để đọc đủ «ĐÃ CHỐT» trong các ảnh100/200 đã chụp. Đây là hình học riêng của
tem ở hàng này; mặc định `Stamp` và các caller khác không đổi. Nhãn trợ năng
đúng không thay bằng chứng glyph thực sự được vẽ; ảnh tĩnh không chấm lại
cú đóng dấu hoặc Reduce Motion.

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
khỏi React: `src/rudi/art/{net,nep,motif,gu,canh,ky-hoa}.ts` chỉ trả mảng `LopVe`
(`d`, vai màu `mau`, `net` > 0 là nét, không có là tô); `src/rudi/ui/art/`
(`VeLop`, `Nep`, `GuGlyph`, `VongHo`/`DuongChuyen`/`GocGap`, `Canh`, `KyHoa`) vẽ mảng
đó bằng `react-native-svg`, nét tròn đầu tròn góc, tô phẳng, không bóng.
`KyHoa` (11/09) là tờ ký hoạ của nơi chưa có ảnh: sân khấu theo loại + ≤ 2
đạo cụ theo tag, một điểm coral, hai khung cắt — xem «Luật Ký Hoạ Trong Sổ».
Nếp ngoài trạng thái rỗng còn một chỗ đứng thường trực: tờ cài trong lề phải,
xem «Dock Nếp: tờ giấy cài trong lề sổ» sau mục «Tờ giấy gấp ba».
- **Ngữ pháp đường**: mọi `d` chỉ gồm lệnh tuyệt đối `M`/`L`/`C`/`Z`, số thập
  phân trơn (không mũ, không `-0`), dựng từ số lúc chạy qua `net.ts`
  (`daGiac`, `netGay`, `cong`, `qCong`, `tron`, `bau`, `cungTron`, `quat`,
  `khungBo`, `vien`, `thon`, `giot`). Bậc hai và cung tròn được đổi ra bậc ba
  ở builder, không bao giờ phát `Q`/`A`. Lý do là hai lỗi thật: Java
  `PathParser` ném lúc mount và app chết khung hình đầu (tsc, web export mù),
  và repo guard đọc chín chữ số cách nhau như số tài khoản.
  `tests/art-duong.test.mjs` parse từng hình đúng cách Java parse.
- **Bốn lưới**: Nếp trong ô **96** (`KHUNG_NEP`), hình gu trong ô **48**
  (`KHUNG_GU`), cảnh trong khung ngang **144×112** (`KHUNG_CANH`), ký hoạ
  trong khung ngang **288×96** (`KHUNG_KY_HOA`, cắt `slice` về 3:1/4:1). Ô đặt qua
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
  thích, 22 trong chip Khám phá, `round(1.15 × size)` `tone="ink"` trong ô
  giấy `PlaceGlyph` (kể cả thumbnail hàng, 11/09); `tone="accent"` đổi mực
  sang coral cho ô/chip đã chọn.
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
     màn hình vẽ; cổng quét cả tám sticker (hai bản đọc) lẫn mọi cảnh của
     `CANH_IDS` (mười hai từ 03/10), và bỏ
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
  trí** — và từ 11/09 chỉ ở nơi cái bàn **thật sự** còn trống một bên
  (`chua-co-hoi`, `chua-co-ban`, `tim-khong-ra`); ở cảnh lỗi `chua-doc-duoc` và
  cảnh lọc `bo-loc-che-het` nó đọc như spinner nên bị **cấm bằng test**
  (`art-duong.test.mjs`, cung tròn coral = 0); **đường chuyền** (`duongChuyen`, đường S của kit với chấm coral ở
  điểm đặt bút) nối những thứ thuộc về nhau; **góc gấp** (`gocGap`, tờ giấy
  gấp góc trên phải cùng góc với Nếp, tỉ lệ 0.28). Ghế `hinhGhe` là đạo cụ
  chung của cảnh.
- **Một mặt sàn cho cả cảnh**: `SAN = 102` trong `canh.ts`. Chân ghế, chân
  khung ảnh, chân bản đồ và bàn chân Nếp (`nepTrenSan` đặt `y0 = SAN −
  CHAN_NEP × tiLe`) cùng kết thúc ở đó, nên không nhân vật nào lơ lửng cạnh
  đồ vật. Cảnh `chua-co-keo` là ngoại lệ có chủ ý: tờ hẹn bay, nhân vật viết
  bên cạnh, không có sàn nào để đứng.
- **Mười hai cảnh** (`hinhCanh`, `CANH_IDS`; mười từ 09/09, thêm hai 03/10 cho
  hai danh sách riêng của bảng tin Cộng đồng). Trước 09/09 có năm, và ba trong
  số đó dùng **cùng một dáng người đổi đạo cụ** — chính điều review cấm nhân
  lên. *Lịch sử 09/09:* lượt đầu của bản mười cảnh tái phạm đúng lỗi ấy ở ba
  cảnh mới; lượt chấm bắt được và ba cảnh ấy được vẽ lại bằng pose mới — trong
  đó `nang-bong` (vẽ 08/09, **gỡ 09/09** cùng ngày, xem mục mày Nếp ở trên) đã
  được thay bằng `dua-hai-tay` ở `chua-co-loi-moi`. *Hiện hành (10/09, audit
  F45a):* mỗi cảnh có Nếp đứng bằng **một pose riêng**, và điều ấy là **dữ liệu**
  chứ không phải lời hứa — `POSE_CANH` trong `canh.ts` đọc từ đúng entry
  `hinhCanh` vẽ, `tests/art-duong.test.mjs` đòi đúng một giá trị cho mỗi id
  của `CANH_IDS`, không hai cảnh trùng pose (03/10 ca đổi tên từ «mười cảnh»
  thành «mọi cảnh», nên thêm cảnh không phải sửa số):
  `chua-co-hoi` `keo-ghe` (mặt `nhuong`) · `chua-co-keo` `ghi-lai` cúi viết
  (`quyet`) · `chua-co-anh` `giu-khung` nhìn xuyên khung rỗng (`hoi`) ·
  `chua-co-ban` `giu-cho` **ngồi** ở bàn hai chỗ, tay mời sang ghế trống
  (`nhuong`, `ngoi`) · `tim-khong-ra` `cam-ban-do` (`hoi`) · `chua-co-tin-nhan`
  `goi-loi` **gọi lời** — tay khum cạnh miệng, bong bóng mọc từ miệng (trước
  đó cùng `ghi-lai` với `chua-co-keo`: hai cảnh một dáng, audit F45a) ·
  `chua-co-loi-moi` `dua-hai-tay` · `chua-co-ky-niem` `voi-len` ·
  `bo-loc-che-het` `ghe-nhin` · `chua-co-bai` `dua-giay` (03/10) — **bảng tin
  khu phố**: hai cột chạy từ sàn lên một mái ván, bảng trống giữa hai cột,
  **một** ghim coral ở phía gần chờ tờ đầu tiên; Nếp chìa tờ gấp nhỏ và dừng
  một bước trước mép bảng — đưa, chưa ghim. Hai bản bị cắt ở review: bảng trên
  chân dưới mái phẳng đọc thành **tủ** (đoạn cột nhô lên trên bảng mới giữ nó
  là bảng tin), và ghim mực còn lại của tin cũ đọc thành **trời sao** ở nền
  tối · `chua-luu-bai` `gap-lai`, mặt **`binh-than`** (03/10) — **hộp mở
  nắp**, nhìn đủ cao để thấy lòng: miệng sau cao hơn miệng trước **14 đơn
  vị**, lòng hộp `bong` ở cả hai scheme, một nét góc trong phía sau chạy
  xuống miệng trước, và **dải đánh dấu coral vắt qua miệng trước**, cùng dấu
  với nút lưu trên thẻ bài. Bản đầu cho miệng hộp là khe 8 đơn vị gần như
  nhìn thẳng: «rỗng» được nói chứ không được thấy, ở nền tối không nói được cả
  thế; mặt `giu-kin` mặc định của pose có miệng phẳng đọc thành **dỗi** trước
  hộp rỗng, và giữ một bài để đọc sau không phải bí mật · và `chua-doc-duoc`
  **không pose**. *Hình và mặt là quyết định đọc ảnh ở review; pose không trùng
  là luật đo được.*
- **`CANH_KHONG_NEP` là cơ chế, không phải lời hứa.** `chua-doc-duoc` **không
  bao giờ** vẽ Nếp, kể cả khi người gọi truyền `nep`: `hinhCanh` bỏ qua yêu
  cầu ấy. Lý do là luật «Nếp Đứng Xa Tiền» phải đúng ở **khoảng hai mươi** màn
  lỗi, trong đó có màn sổ, và một luật do hai mươi nơi tự nhớ là một luật sẽ
  hỏng. `ErrorState` gắn sẵn cảnh này nên mọi màn lỗi có hình mà không nơi nào
  phải nhớ gì. Cổng `art-duong` biết tập ấy và đòi bản có/không Nếp **giống hệt
  nhau** cho các id trong đó.
- **Ô rỗng không có cảnh là một danh sách trong CỔNG, không trong văn này**
  (03/10). `apps/mobile/tests/trang-rong-co-hinh.test.mjs` quét mọi
  `<EmptyState …/>` trong `src/` và `app/` (trừ chính component và
  `app/dev/`): ô nào thiếu `illustration=` phải có tên — đúng tệp, đúng tiêu đề
  như viết trong mã — ở một trong hai bảng, và tên nào không còn ứng với ô rỗng
  không cảnh nào cũng đỏ. **`KHONG_HINH`: bảy im lặng cố ý, mỗi cái một lý
  do.** Ba danh sách quản trị («Bạn chưa chặn ai», «Chưa có phiên nào», «Chưa
  ẩn bài nào» của Cộng đồng) không cần ai kể chuyện, và review đã cấm đưa hình
  kể chuyện vào mọi hàng dữ liệu; dải «Thông báo» ở Khám phá là **ghi chú kỹ
  thuật của bản trải nghiệm** («chưa có hộp thư máy chủ»), lời hệ thống nói
  về chính nó, không phải im lặng của người dùng; «Chưa có sổ nào để quyết
  toán» là màn tiền — Nếp đứng xa tiền, và sổ không nhận chuyện kể; «Chưa có
  lời nhắn nào» là một dòng dưới bài trên tường, nơi một cảnh sẽ nặng hơn chính
  bài nó trả lời; «Chưa có ghi chép nào» là danh sách ghi chép riêng mở từ khay
  cài đặt bảng tin. **`CHUA_VE` trống từ 04/10**: hai ô nợ cuối cùng đã có
  cảnh — «Chưa có bạn nào để rủ» (`ChonNguoi`) → `chua-co-ban`, «Chưa có địa
  điểm để thêm» (`PickOutingLive`) → `tim-khong-ra`. Bảng vẫn tách đôi để không
  ai đọc nợ thành quyết định. *Lịch sử:* tới 03/10 văn ở đây
  kể ba chỗ là «toàn bộ» trong khi mã đã nhiều hơn — lý do danh sách chuyển vào
  cổng. Ô rỗng mới không cảnh thì ghi tên và lý do vào bảng của cổng, không ghi
  vào đây.
- **Mỗi cảnh trọn vẹn khi không có
  Nếp** (`nep: false`, so `sua-ab/A-co-nep` với `B-khong-nep`) và được trình
  đọc màn hình đọc thành **một câu**: `chua-co-hoi` «Một chiếc ghế được kéo
  ra, chừa sẵn chỗ» · `chua-co-keo` «Một tờ hẹn trống, nét mực bắt đầu từ
  đó» · `chua-co-anh` «Một khung ảnh còn trống, góc giấy gấp» · `chua-co-ban`
  «Hai chiếc ghế, một chỗ còn trống» · `tim-khong-ra` «Một tấm bản đồ gấp,
  đường đi chưa tới nơi» · `chua-co-bai` «Một bảng tin còn trống, chiếc ghim
  đầu tiên chờ sẵn» · `chua-luu-bai` «Một chiếc hộp mở nắp, bên trong còn
  trống» (đủ cả mười hai câu ở `MO_TA`, `canh.ts`). `Canh` khung chặt theo
  `hopNgang` + `viewBox`, nên bỏ Nếp thì cảnh không để lại khoảng thụt bên
  trái; `width` là bề rộng
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
  bằng chứng khay/bubble sáng-tối ở `docs/archive/claude/2026-09-08/tra-loi-sticker-cho-ti.md`.
- **Luật Nếp Đứng Xa Tiền.** Nếp chỉ xuất hiện ở trạng thái rỗng và cửa vào;
  **không bao giờ** cạnh số tiền, lỗi, hay xung đột (báo cáo 07/09 §6.4).
  Dock Nếp là cửa vào thường trực nên cũng theo luật: trên màn tiền chỉ còn
  mép giấy trơn 10dp, không mặt, không tờ thứ hai, chạm không đưa Nếp ra
  (ADR-0035 §2.4).
  Không dấu chuyển động, không mặt hào hứng trên mọi tư thế: tay giơ đã nói.
- **Luật Vòng Hở Không Tiến Độ.** Vòng hở không bao giờ là progress ring:
  không animate, không đi cùng phần trăm, khe luôn rộng. Đường chuyền khi
  mang nghĩa tiến độ phải có chữ đi kèm.

- **Nếp mảnh (`gap: "manh"`, 12/09) — cùng nhân vật, tờ khác.** `GAP_NEP =
  trang | manh`; `trang` là trang của sổ hội bạn, **trùng từng toạ độ với
  `main`**: cổng `art-duong` băm sha256 toàn bộ lớp của 19 pose × 2 cỡ (680
  lớp) và so với `tests/fixtures/nep-trang-baseline.json` lấy từ `main
  2d0b9777`; đổi một toạ độ của bản trang là đỏ (lưu hash, không lưu path, vì
  repo guard đọc chuỗi toạ độ như số tài khoản). *Luật đo được.* `manh` là
  cùng tờ **gấp làm tư** cho sổ hai người: cùng mắt, cùng một mày, cùng
  miệng, cùng tay; mép cắt H..G giữ nguyên chỗ nên mày và mọi thứ trên mắt
  không dời.
  - **Thân mở sang trái 53/56** (x 19..72 trên y 20..76), **cạnh trái thẳng,
    chân thẳng**, sáu đỉnh; vai gần dời ra theo mép (x 25 → 20). Ba vòng đọc
    mù dạy điều này từng bước: 40 → «hẹp hơn chứ không vuông hơn»; 48 với vát
    góc → «lục giác, bo góc»; 48 thẳng cạnh → «hết lục giác, còn hẹp»; nên
    rộng ở **đường bao**, không chỉ ở hộp bao. Cổng đo **độ lấp đầy hộp bao**
    của lớp `giay` đầu (mảnh ≥ 0.87 và hơn trang ≥ 0.03) **và** tỉ lệ hộp bao
    (mảnh ≥ 0.94 và hơn trang ≥ 0.05). *Luật đo được; ngưỡng là quyết định
    đọc mù.*
  - **Hai nếp `bong` chia bốn ô gần vuông**, vẽ **trước** mặt (mắt và miệng
    nằm trên nếp như mực trên chỗ gấp): dọc x 47 (y 23..75), ngang **y 50.5**
    giữa mắt và miệng (nếp ở gấu y 62 làm ô trên gấp đôi ô dưới, hình đọc
    cao); nét 1.8 / 2.4 ở bản rút gọn. Đọc mù vòng 1 và 3: «vết gấp tờ giấy,
    gấp tư, không phải sống mũi». Cổng: đúng hai nét `bong` ở cả hai cỡ.
  - **Góc coral gấp VÀO TRONG**, chỉ hé **một dải** dọc mép cắt H..G (tam giác
    H·G·M): M cách mép **3 đơn vị ở ≥ 72dp**, **6 ở dưới 72** (bản 48 vẽ nửa
    dp mỗi đơn vị; dải 3 ở đó đọc mù là «gần như mất»). Viền mực của dải chỉ
    có ở bản chi tiết. Cổng `laDaiGap` nhận dải **theo hình** (ba mức y khác
    nhau, mảnh: tỉ lệ dày/dài 0.05..0.32) trên 22 pose × 7 biểu cảm × 2 cỡ × 4
    độ nghiêng: đúng **một** dải, không mang nếp gấp của bản trang, **không
    nét mực nào sơn lên dải**, bề dày ≥ 2.5 / ≥ 5.5 đơn vị đo lúc đứng thẳng.
    *Luật đo được.*
  - **Đổi biến thể không thêm hay bớt coral ở pose nào**: cổng đếm **lớp**
    `gap` của `manh` bằng của `trang` ở mọi pose (không né theo hình, vì cổng
    hình không thấy coral khác hình). *Luật đo được.*
- **Biểu cảm `giu-kin`** (thứ bảy trong `BIEU_CAM`): miệng **một nét thẳng
  khép**, ngắn hơn và **phẳng tuyệt đối** so với `quyet` (nghiêng); mày hạ
  hai đơn vị, phẳng. Biết mà không nói. Cổng: bảy mày là bảy đường khác nhau;
  miệng `giu-kin` phẳng, ngắn hơn `quyet`. *Luật đo được.*
- **Ba tư thế truyền giấy** (`dua-giay` · `up-xuong` · `gap-lai`), mỗi cái là
  một việc làm với **một tờ giấy**, theo luật **nghiêng về vật, nhìn vào
  vật, tay ở giữa vật**: `dua-giay` {nghieng 6, nhìn (1.4, 0.4), `nhuong`}
  đưa tờ nhỏ gấp sang phải, tờ vẽ **sau** bàn tay để nằm trong lòng tay;
  `up-xuong` {5, (1.2, 1.6), `giu-kin`} tờ nằm **phẳng 4:1** trên đường sàn
  (x 64..96, y 83..91), tay gần thẳng xuống, mitten ở **giữa** tờ, thân
  nghiêng và nhìn xuống (hai bản trước đọc mù thành «kéo que», «kéo va li»
  vì tờ dày, tay chạm mép, người đứng thẳng nhìn trước); `gap-lai` {0, (0.6,
  1.4), `giu-kin`} **đứng**, tờ hai mảng gập thật trước ngực (mảng phải là
  hình bình hành hẹp đang quay về người xem, sống gấp là nét mực). Cổng: ba
  pose đúng **một** lớp coral ở cả hai biến thể. *Tay và tờ là quyết định
  đọc mù; số coral là luật đo được.* Từ 03/10 hai trong ba còn đứng trong
  cảnh rỗng (bản trang): `dua-giay` ở `chua-co-bai`, `gap-lai` ở
  `chua-luu-bai` với mặt `binh-than` đè `giu-kin` của pose qua `them`
  (xem «Mười hai cảnh»); `up-xuong` chưa cảnh nào dùng.
- **Vạt của tờ nhỏ là `giay` dưới viền mực, không `bong`.** Trên nền tối
  `paperShade` tối hơn `paper`, vạt `bong` đọc thành **lỗ khoét** thay vì góc
  lật lên (vòng 3, motif). Áp cho tờ trong tay `dua-giay`, tờ sàn `up-xuong`
  và motif `thuGapBa`. *Quyết định đọc mù; cổng `thuGapBa` giữ «không mảng tô
  nào ngoài giấy».*
- **Motif `thuGapBa`** (`ThuGapBa`, lùi 1.5 trong khung): tờ **năm đỉnh** với
  góc cắt trên phải (`c = 0.22 × min(w, h)`), vạt `giay` dưới viền mực 0.8×,
  **hai vết `bong` ở h/3 và 2h/3**, viền mực. **Không coral**: coral thuộc tờ
  dẫn và do màn hình đặt qua `ToGiay dan`. Khung bo + hai kẻ đọc mù thành
  «thẻ index kẻ dòng / icon list rỗng» (vòng 2); góc cắt mới nói «giấy» (vòng
  3: «tờ giấy có kẻ dòng, viền mực, góc trên phải gấp lại»). Cổng: 0 lớp
  `gap`, 5 đỉnh, đúng hai nét mực (tờ + vạt), không mảng tô ngoài `giay`, hai
  vết đúng ở một phần ba và hai phần ba. *Luật đo được.*
- **Luật Góc Cắt, Không Badge.** Coral ở góc một tờ giấy chỉ hợp lệ khi góc ấy
  **bị cắt**: tam giác nền lộ qua, mép cắt nối tiếp viền, vạt là mặt sau. Tam
  giác coral trong góc còn nguyên là badge/notification dù ai vẽ. Áp cho
  `ToGiay dan`; góc gấp nhận diện bản trang (`laNepGap`) và bản mảnh
  (`laDaiGap`) là hai ngữ pháp riêng của nhân vật, không đổi.
- **Luật Một Nhân Vật, Hai Tờ.** Biến thể của Nếp đổi **tờ** (bao thân, nếp,
  chỗ hé coral), không đổi mặt, tay, tư thế, số coral. Bản mặc định bị băm;
  nhân vật thứ hai là việc của ADR, không của một prop.

### Hàng địa điểm (`PlaceLead`, `PlaceCompare`, `PlaceRow`, `PlaceGlyph`)
Một từ vựng `DiaDiemHienThi` cho catalogue fixture và màn live; đợt 08/09
thêm `loai` (id danh mục, để khung trống vẽ hình gu) và `lyDo` (một lý do có
căn cứ); vòng 2 (08/09) bỏ cặp `photo` + `attribution` rời nhau, thay bằng
**một** trường `anh: AnhCoGhiCong | null`, nên ảnh và ghi công đi cùng nhau
ở tầng kiểu và cả ba khung tự in `cauGhiCong(anh.nguon)`. Nhịp kết quả sau `taiSoSanh`: **một ảnh dẫn** (chỉ khi có ảnh) →
**một cặp so sánh** (khi còn ≥ 2) → **các hàng** (`sua2-sang-1.0/bs-04-kham-pha*`).
- **`PlaceLead`** — **hiện hành (02/10): một thẻ.** Nền `card`, viền kẻ tóc
  `line`, bo `radius.control` (14), `overflow` cắt; ảnh (hoặc tranh cảnh `PlaceArtwork`, 21/10; tới 11/09 là tờ ký hoạ `KyHoa`)
  **tràn mép trên thẻ**, 16:9 compact / 21:9 rộng,
  `MediaSlot radius={0}`. Tim là `IconButton tron` treo **trên ảnh** góc trên
  phải (10/10), anh em với vùng bấm mở thẻ (trình đọc màn hình gặp hai control).
  Dưới ảnh, đệm 14 gap 6: tên `h2 ink`; phụ đề `body inkSoft` **một dòng**;
  các sự thật còn lại thành **hai dòng lặng** `caption inkFaint` (03/10, xem
  «Hai dòng sự thật» dưới); rồi hàng chip (gap 8, gập dòng): **chip lý do** nền `aiSoft` chữ `label ai`
  + `sparkles` 13 và **chip giá** nền `ground` chữ `label inkSoft` +
  `pricetag-outline` 13, cả hai bo `radius.control`, đệm 10/6, **không bao giờ
  cắt «…»** — tiền và lý do gập dòng chứ không giấu (`tachGia` của `dia-diem.ts`, tách giá
  theo icon `wallet-outline`). Không có `lyDo` thì con dấu tím như cũ (trên
  ảnh, hoặc trên tên khi là ký hoạ). Tiêu đề mục trên nhịp kết quả là
  **«Chỗ hay ở <thành phố>»** (demo: «Chỗ hay ở Đà Lạt»), dòng đếm «N nơi»
  `caption inkFaint` sát dưới (−14); **không** «Gần bạn, đúng gu»: danh mục đi
  theo thứ tự máy chủ, tiêu đề không hứa gần hay hợp gu. Phần dưới là lịch sử
  của dạng lead trên giấy trước 02/10, giữ để đọc lý do của luật một dấu.
  *Lịch sử tới 01/10:* ảnh 16:10 compact / 21:9 rộng, bo 20. *Lịch sử tới 10/09:*
  `Stamp` «HỢP GU» trên ảnh **và** một dòng «Hợp gu nhờ Chill và View đẹp»
  dưới tên, chỉ khi máy chủ gửi `reason`; tái audit Codex 10/09 (R3) đọc ra
  cùng một lời hứa bốn lần (tiêu đề mục → con dấu → «Hợp gu nhờ…» → mô tả
  lặp lại tag). **Hiện hành (11/09):** **một dấu cho một địa điểm** —
  `lyDo` có thì in **dòng lý do** (`LyDo`: sparkles 13 ẩn khỏi cây trợ năng +
  `label` màu `ai`, tối đa 2 dòng, 3 khi chữ lớn — một tag fixture chỉ cần một,
  câu của mô hình live cần chỗ, dấu «…» ở đây là giấu đúng sự thật hàng tồn tại
  để nói) ngay dưới tên `h2` và **không in con dấu**; không có `lyDo`
  thì con dấu tím nghiêng -2 trên ảnh như cũ (`dauCon`). Lý do là **một tag**
  nhóm thật sự match (fixture: `chonLyDo(tags, sub)` chọn tag đầu mà mô tả
  chưa nói — «View đẹp», «Nhóm đông», «Nhẹ nhàng» trên
  `native-r14/anh/r14-80-kham-pha-*`, màn dán «Dữ liệu demo») hoặc **câu của
  mô hình** (live, `reason`); không bao giờ là tagline hoá trang làm lý do và
  không bắt đầu bằng «Hợp gu». Rồi mô tả `body inkSoft` (không nhắc lại từ
  của lý do), ba sự thật với icon 16 (`label inkSoft`); nút lưu `IconButton`
  phải. Ảnh là `MediaSlot` nhận
  `attribution={anh.nguon}` nên ghi công in ngay dưới khung; `anh === null`
  thì không khung 16:10 mà là **tờ ký hoạ** (`KyHoa`, 3:1 · 4:1 ở chữ lớn) rồi
  tên + sự thật, cùng bố cục cột như lead có ảnh (tới 11/09 là ô giấy 34 + tên;
  xem «Luật Ký Hoạ Trong Sổ»; *hiện hành 21/10:* tranh cảnh `PlaceArtwork`
  trong khung 16:9, không `KyHoa`).
- **`PlaceCompare`**: hai ứng viên **trên một trục**, không thẻ quanh ô nào:
  hàng `gap` 16, đệm dưới 12, kẻ tóc `line` dưới; mỗi ô `flex 1` gồm
  `MediaSlot` **4:3** với trái tim `IconButton` ở góc dưới phải **trên ảnh**
  (như ảnh dẫn, không hàng mồ côi dưới sự thật) và `Stamp` tím `nen` khi có
  badge và không có `lyDo`; tên `title` hai dòng, **dòng lý do `LyDo`** khi
  có, mô tả `note inkSoft` hai dòng, **hai dòng sự thật** `note inkFaint`
  (03/10, mỗi dòng tối đa hai dòng hiện; xem «Hai dòng sự thật»), **giá kèm
  đơn vị đứng riêng một dòng** `note inkSoft` để ô nửa màn không bẻ «80K/người»; ghi công
  `note inkFaint` khi ảnh có giấy phép. Thuần: cùng hàm chia cho fixture và
  live.
- **`PlaceRow`**: thumbnail 56 bo 10 khi có ảnh; trống thì **`PlaceGlyph`
  33** (ô giấy 56, cùng cỡ ảnh) thay cho ô `accentSoft` cũ; **con dấu đứng
  cạnh tên** trên cùng hàng (`rowTen`, tên rút về một dòng khi có dấu) để
  hàng có match cao bằng hàng thường — chỉ khi hàng không có `lyDo`; có thì
  **dòng lý do `LyDo`** dưới tên thay con dấu; mô tả `caption inkSoft` một dòng,
  **hai dòng sự thật** `caption inkFaint` (03/10, mỗi dòng tối đa hai dòng hiện),
  giá riêng một dòng `caption inkSoft` không cắt, ghi công `caption inkFaint` tối đa hai
  dòng (`cauGhiCong`); nút tim phải; đệm dọc 10, gap 8, kẻ tóc dưới. Hàng
  nằm trên giấy, **không thẻ**; ở tablet hai cột. Ảnh hỏng (`onError`): ô
  56 vẽ lại hình gu và hàng in «Chưa tải được ảnh» `caption warn` dưới sự
  thật, ghi công vẫn in (ảnh `sua-review-2/native-kham-pha-anh-hong-*`).
- **Hai dòng sự thật (`DongSuThat`, `chiaDongSuThat`)** (03/10, cả ba khung):
  dòng đầu là điểm đánh giá và khoảng cách (cả địa chỉ khi có), nối « · »;
  khi dòng mở bằng điểm (`moDauBangSao`) thì trước điểm là **một ngôi sao
  Ionicons `star` 12 vẽ bằng chính mực của chữ** (icon vẽ, không phải ký tự
  «★» trong chuỗi), nằm trong dòng, giữ với điểm bằng NBSP: «4.8 (64)» một
  mình không nói nó đếm gì.
  Dòng sau là giờ mở, «Đang mở, 08:00 – 21:00», chỉ gãy sau dấu phẩy. Mỗi sự
  thật giữ liền bằng NBSP (và word joiner sau gạch), nên dòng chỉ gãy giữa hai
  sự thật và **không dòng nào mở hay kết bằng «·»**. Giá không ở hai dòng này:
  nó vẫn là dòng riêng hoặc chip giá (`tachGia`). Trình đọc màn hình nghe chữ,
  không nghe sao.
- **`PlaceGlyph`**: *Lịch sử tới 10/09:* đĩa `accentSoft` tròn, hình gu tô
  coral toàn phần; tái audit 10/09 (R3) đọc «ba quán ăn là ba cái bát giống
  nhau» và đĩa tint + icon một màu là mặc định của mọi app. **Hiện hành
  (11/09):** **ô giấy** vuông cạnh `round(1.7 × size)` (cùng dấu chân đĩa cũ
  nên bố cục không dời), nền `card`, viền kẻ tóc `line`, bo `radius.small`
  (cùng bo với khung ảnh); bên trong `GuGlyph` `round(1.15 × size)` **tô
  `ink`** với **một** chi tiết coral của riêng hình (`tone="ink"`). Hình chọn
  theo **tag thật trước, danh mục sau**: `gu` (từ `guTheoTag(tags)`: «Món
  local» → `mon-local`, «Ngoài trời» → `outdoor`, karaoke, game, mua sắm, cà
  phê, đi đêm) rồi mới `guTheoLoai(loai)` (`quan-an-local → an-uong`, `cafe`,
  `vui-choi → game`, `di-choi-dem → nightlife`, còn lại → thẻ gấp); Ionicons
  màu `ink` chỉ khi không có cả hai. Cùng một ô cho fallback `MediaSlot` (44),
  cặp so sánh (24/34), thumb hàng (33). **Lead không ảnh và đầu bài chi tiết
  không bìa (fixture + live `dauGon`) từ 11/09 dùng tờ ký hoạ `KyHoa` thay ô
  34/36** — vẫn không khung 16:10 rỗng (`ky-hoa-trong-so/`). Khung trống
  **không bao giờ** là ảnh stock. Không thêm token. *Hiện hành (21/10):* lead
  không ảnh và bìa trang quán không ảnh vẽ `PlaceArtwork` theo `canh-ve.ts`
  (khung 16:9 của bìa, không nhãn), xem mục tinh chỉnh 21/10 trong Components.
- **Sân khấu thành phố (`SanThanhPho`)** (02/10): đầu Khám phá › Địa điểm,
  dưới dòng vị trí («<thành phố>» `ink` + « · đổi nơi khác» `accent`), cả
  fixture lẫn live; gập đi khi đang tìm hay lọc. Sân khấu **dâng lên 32 dp dưới
  dòng vị trí** (dòng đó `zIndex: 1`, nằm trên trời của tranh). Điện thoại:
  tràn hai mép màn (trả lại lề 16). Tablet: tranh giữ **tỉ lệ gốc, rộng tối
  đa 480**, ở giữa một dải rộng cả cột, và **vạch đất kéo tiếp tới hai mép
  cột** cùng mực `ink`, cùng nét (`NET_KY_HOA.gan` theo tỉ lệ) — một chân trời
  liền, không phóng tranh theo cột (nét dày gấp đôi, đẩy địa điểm khỏi màn đầu).

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
  **không** có dấu tick, đồng xu, ký hiệu tiền tệ, QR hay dấu ngân hàng (ranh
  giới ADR-0021) — cái nói «tiền» phải là silhouette. *Lịch sử tới 09/09:* hai
  tờ chồng nhau, không gấp góc (bản gấp góc đọc ra phong bì); *10/09:* ba tờ
  xoè, tờ trên mang **góc gấp coral** — cùng nếp gấp Nếp đeo. **Hiện hành
  (11/09):** vẫn ba tờ xoè quanh trục sát tay xa, tờ trên (tờ thấy trọn) mang
  thêm **ô bầu dục tô `bong`** ở cả hai cỡ và **khung đôi** nét mảnh chỉ ở bản
  120 — hai dấu mọi tờ bạc có mà vé không có; nét mảnh ở 64 là nhiễu nên bản rút
  gọn bỏ khung. `ket-xe` **hiện hành (11/09):** Nếp `ngoi-xe` chống cằm ở
  0.66/x0 −9 (khung xe máy co theo `k = 0.66/0.76`), trước mặt là **đuôi xe
  buýt** — thân giấy **rộng hơn cao** (≈42×38) trên **hai bánh mực** cùng sàn
  `CHAN_NEP`, **hai ô kính** `bong` (một ô đọc ra màn hình), và **một** vạch đèn
  hậu coral. Khối cao không bánh (10/09) đọc ra điện thoại/kiosk ở khay 64; bản
  có bánh nhưng thân dọc một ô kính (11/09 sáng) đọc mù ra «xe tải/xe đẩy». Vạch coral là **ngoại lệ có ghi lý do** của
  ngân sách coral: một vạch nói «đuôi xe» và không gì khác; hai chấm bị loại vì
  hai chấm trên một vạch **thành khuôn mặt**. Cổng cơ chế: ≥ 4 hình tròn tô ở
  cả hai cỡ (`ket-xe`), ô bầu dục vai `line` và không teal (`tra-tien-ne`).
  `cho-ti` giữ nguyên bố cục đã duyệt và chỉ được **cân lại nét**
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
  **bounds**: `scripts/do/kiem-bounds.mjs` đòi mỗi nút có
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
- **Nghĩa hình phải tự đứng; sửa bằng hành động, không bằng chi tiết** (audit
  native 09/09, F45). Bốn hình đọc sai nghĩa được sửa theo cùng một phép: dựng
  ứng viên bằng chính primitives, render cạnh bản cũ, **nhìn** rồi chọn
  (`docs/archive/claude/2026-09-10/nghia-hinh/`). «Kẹt xe»: xe + mặt mệt chỉ nói «đi
  xe»; nay **đuôi xe buýt** (khối cao, dải kính `bong`, một vạch cản) chạm bánh
  trước và người **chống cằm** (`ngoi-xe` đổi tay, mắt chúc xuống) — hai đèn hậu
  coral bị loại vì hai chấm trên một vạch **thành khuôn mặt**; vệt khói sau bánh
  cũng bỏ (reviewer 10/09): một tín hiệu **chuyển động** trong bức tranh nói
  «không đi». «Trả tiền nè»:
  hai tờ chồng góc coral đọc ra vé; nay **ba tờ xoè** ngang hơn cao — silhouette
  tiền mặt không cần ký hiệu tiền, giữ ranh giới ADR-0021; hoá đơn xé đôi bị
  loại vì **đưa hoá đơn** là đòi tiền. `chua-co-tin-nhan` từng cùng dáng `ghi-lai`
  với `chua-co-keo`; nay pose **`goi-loi`** — tay khum cạnh miệng ở (37,58), tay
  xa mở ra (92,66), và bong bóng có **đuôi trỏ vào miệng**, vẽ trước người để
  mặt che đầu đuôi; bong bóng cầm trên tay bị loại vì đọc như cầm bảng.
  `chua-doc-duoc` bỏ **`vongHo`** (đọc như spinner đứng yên — vòng hở của
  `motif.ts` là «bàn trống một bên», sai nghĩa ở cảnh lỗi): coral nằm **trên vết
  rách**, mảnh rách trượt sang bên — «đã rách», không «đang tải». Cơ chế: bảng
  `NEP` của `canh.ts` là **dữ liệu** `{pose, x0, y0?, tiLe, them?, truoc?}` và
  `POSE_CANH` đọc từ đúng entry `hinhCanh` vẽ, nên `art-duong.test.mjs` đòi mười
  pose không trùng và cảnh lỗi không có cung tròn coral; refactor bảng so byte
  10/10 cảnh với mốc trước khi đổi pose nào. Việc còn của team: kiểm chứng
  **không nhãn** với người chưa đọc brief bằng `khong-nhan-sang/toi.png`.
  **Vòng 2 (11/09**, tái audit Codex 10/09: F45 mới đóng một phần, R4;
  `docs/archive/claude/2026-09-11/nghia-hinh-v2/`): «Kẹt xe» — khối 10/09 đọc ra kiosk,
  nay **xe buýt có bánh** (hai bánh mực trên sàn) + một vạch đèn hậu coral,
  người nhỏ hơn để xe cao hơn đầu. «Trả tiền nè» — tờ trên mang **ô bầu dục +
  khung đôi**, không ký hiệu tiền. `chua-co-tin-nhan` — ý «vẽ trước người để
  mặt che đầu đuôi» của 10/09 **bị thay**: đuôi chạy dưới tờ giấy nên bị che
  trọn, bong bóng đọc như bảng có chấm; nay thân bong bóng đặt **cao hơn
  miệng** và đuôi là **tam giác riêng mọc từ cạnh đáy** (gốc x 64–78, có cạnh
  bong bóng ở hai bên), mũi (53,58) **dừng ở mép tờ giấy** đúng độ cao miệng theo
  quy ước tranh — bản đuôi cắt từ góc dưới-trái đọc mù ra «bảng có góc vạt»;
  bong bóng **trống**, không chấm coral — chấm làm nó thành bảng.
  `bo-loc-che-het` — **bỏ `vongHo`** (quyết định của Codex cho mục mở vòng 1):
  «che» phải có **cái bị che** — một **tờ có bốn dòng chữ** vẽ trước, **tấm che**
  `bong` lệch xuống-phải để lộ dải trên-trái của tờ (đầu các dòng chữ), lưới chỉ
  2×2, **một ô hở** viền coral bốn nét thẳng cho lộ một mẩu dòng chữ; panel tô
  `bong` + lưới 4×4 không có gì bên dưới đọc mù ra «bảng tính/lịch», lưới mực dày
  đọc như giấy kẻ ô. Cổng mới: `art-duong.test.mjs` cấm cung
  tròn coral ở **cả** `chua-doc-duoc` và `bo-loc-che-het`, mọi đỉnh bong bóng
  ≥ x 52 và đỉnh trái nhất ở y 56–62; `rudi-chat-sticker.test.mjs` đòi `ket-xe`
  ≥ 4 hình tròn tô và `tra-tien-ne` có ô bầu dục vai `line`, không teal. Cổng
  pin **cơ chế** («có bánh», «có bầu dục»), không pin nghĩa.
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

### Danh sách người (`LuoiNguoi`)
Danh sách người mà mỗi hàng có hành động (Thành viên nhóm, Bạn bè) giữ hành
động **cạnh tên nó tác động** (`ui/LuoiNguoi.tsx`). Điện thoại: một cột, kẻ
tóc `line` giữa hai hàng **bắt đầu từ cột chữ** (lề trái 52 = avatar 40 +
gap 12). Khi **chính danh sách** (đo bằng `onLayout`, không đọc cửa sổ) chứa
được hai cột ≥ 280dp (`gridFor(rong, 280, 16, 2)`, gap 16): hai cột, mỗi ô kẻ
tóc dưới riêng, và hàng nhận `luoi = true` để **đặt hành động dưới tên, trong
cột chữ** (cạnh tên trong ô 280 bẻ tên làm đôi). Lý do: một hàng 590dp ở
tablet đặt «Nhắn tin», «Đặt làm quản trị» cách tên 330 đến 680px (QA
UI-081). Hàng tự kẻ viền của nó thì truyền `vachTrong={false}`. Hàng nhắc
lại hành động trên mỗi người dùng `RudiButton compact ghost` («một chữ lặng,
không phải viên»), người là một `Pressable` mở hồ sơ («Xem hồ sơ <tên>»),
hành động đứng cạnh, không bao giờ nằm trong nó.

### Chat, bảng tin và hồ sơ — GPL, bàn giao giới hạn 11/10/2026

Hợp đồng compact/layout giữ UI3, không thêm art hoặc đổi shared primitive
cho F3. Cột chat và vùng viết wide theo Layout; nguồn và lịch sử trong
[GPL brief](docs/design/nep-ui-implementation/redesign-20261010/GPL-COMPACT-NATIVE-BRIEF.md).

- **Bảng tin giữ điều đã nhận.** Chỉ retry trang đầu chưa nhận khi focus lại;
  trang rỗng đã nhận vẫn ready. Chỉ request hiện hành hạ loading/fetching.
  Refresh chỉ cuộn đầu khi response được áp và màn còn focus; request cũ
  bị vô hiệu giữ cards, phần mở và offset. Diagnostic actual TSX D01–D07
  cùng current-refresh positive có (9/0), không chứng minh nhân quả của
  sequence native lỗi cũ hoặc toàn bộ navigation/E2E.
- **Thao tác sửa hồ sơ ôm đúng nhãn.** «Chỉnh hồ sơ» và «Sửa tiếp hồ sơ» dùng
  `PressScale`, icon bút chì và `typography.label` hiện hành thành nhóm gọn
  căn trái; đệm (14×10dp), gap (9dp), target source tối thiểu (48dp) và trần
  theo cột. Nhãn thường một dòng, căn giữa theo chiều cao; chữ lớn được
  reflow khi chiều rộng khả dụng thực sự cần, không thu font hoặc cắt chữ.
  Back giữ nháp trong phiên mounted, Huỷ bỏ nháp và trả lại nhãn sửa.
- **Đo chữ phục vụ bố cục tại caller.** Text cùng style đo trong toàn cột
  hữu hạn; chỉ dùng intrinsic width khi đúng một dòng, làm tròn lên pixel
  thiết bị rồi cap phần còn lại sau icon/đệm/viền/gap. Key gồm nhãn,
  fontScale, density và cột; event cũ bị loại. Final061 cấp `width` tường
  minh cho Text để native đo lại chiều cao theo chiều rộng đó, thay
  `minWidth` của814. Text đo absolute, opacity0, ẩn trợ năng/pointer-none là
  phần đo chức năng, không chiếm layout; diagnostic DEV logs đã bỏ. Đây là
  sửa cục bộ, không đổi `RudiButton` hoặc tạo token đo nhãn toàn hệ.

Verdict7cf mở (35/35) contact, kiểm (105/105) original và mở lại sáu cặp
old/new: F1/F4 resolved, F2 resolved trong diagnostic nêu trên, F3 partial.
Verdict061 mở (4/4) contact, kiểm (12/12) original, mở riêng sáu original:
**F3 resolved / ship**, chỉ bốn profile sáng100/tối200 × hẹp/rộng và ba
trạng thái edit/resume/cancel-restored. Không thấy regression mới trong vùng
đã chấm. Ảnh8ec và814 vẫn là partial, không đổi nhãn thành061.

Hồ sơ private ngoài repo:
[GPL7cf](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-7cf58966/gpl-verdict-review-7cf.md),
[F3 cuối061](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-061d8d7a/f3-verdict-review-061.md),
[cổng061](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-061d8d7a/final-mechanical-verification.md).
Cổng clean exact061 ghi (9/9) bước và Node subset (214 file, 1610/1610,
0 fail/skip/cancel/todo), không full gate. Frozen GPL identity (9/0), canary
và hai mutant riêng từng (8/1), setup0; frozen host không phát
`onLayout/onTextLayout`, nên không chứng minh phép đo width/height native.
Detector061 đúng một invocation/một file `HoSoSong.tsx`, raw `[]`, regex-only;
không đo geometry, contrast, target thực hoặc độ đẹp.

### Composer Khoảnh khắc — J, bàn giao giới hạn 11/10/2026

Ảnh/caption, privacy và footer cùng cột; helper ở trên gesture indicator,
IME giữ field và nhóm gửi theo Layout. Không thêm vật trang trí bù lề.
[J brief](docs/design/nep-ui-implementation/redesign-20261010/J-MOMENTS-ALBUM-BRIEF.md)
giữ lịch sử upload/retry/media/comment/check-in của2085 và full review c3.

Verdict7cf mở (21/21) contact chứa (58/58) required frame, kiểm hash/kích
thước cả58 baseline và58 current, mở năm cặp original composer: **P1/P2
resolved / ship**, chỉ hai finding. Root inspection mở lại bốn original;
không cộng original thành viewport mới. Bộ ảnh gồm29 phone (320×640) và29
wide (768×1024), cùng emulator5558. Năm target production J, gồm shared
`Sheet`, và test gốc byte-identical7cf↔061; carry verdict/ảnh7cf, không gọi
là native J chạy lại tại061. PureJ trước đó identity (5/0), canary/hai mutant
mỗi variant (4/1), setup0 chỉ là pure-helper/static scope.

Bằng chứng private:
[J verdict7cf](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-7cf58966/j-verdict-review-7cf.md),
[root inspection](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-7cf58966/j-root-verdict-capture-inspection.json),
[byte identity](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-061d8d7a/J-production-identity-proof.json).
Native F3 dùng Metro JS061; APK nền vẫn checkpoint
`2f20a30179d0e21a1f4b3244dfbd685b5bade7e9`, SHA256
`963bd5daa0be4348606158dd3c06848e7ef604e06ebbdf035136a718cae0d514`.
Theo `native_provenance` của
[packet runtime061](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-061d8d7a/f3-verdict-packet.json),
API runtime giữ source `c3b04ae8afc1cb005c650e7d35de522d7c5ac231`, binary SHA256
`ecf961065bbd1848d5e6267acd8fb54effd796321eca33041a8fe46094988de6`.
Cổng clean061 chỉ build/hash core, không thay runtime; không gọi APK/API là
rebuild061. Lượt documenter chỉ đọc hồ sơ đã có,
không mở ảnh mới, chạy source/test/device/API/runtime/detector/browser.

### Nhóm và bạn bè — bàn giao giới hạn 10/10/2026

Đọc sáu màn H và `chat/CaiDatNhom.tsx` tại source
`d91e66d16a06b3beea166963c75890ef2b69da7a`, cùng `Sheet`, `SoBia` và
`ONhapMuc`; kế thừa hệ giấy/mực/bìa vải/coral/Bricolage và lối gọi Nếp hoạt động.
Không thêm token, world hoặc raster. Mockup03 chỉ là tham chiếu phản biện,
không phải comp được duyệt.

- **Bìa chứa ô nhập thật.** `GroupNewScreen` dùng `SoBia fitContent` và
  `ONhapMuc co="lon"` hai dòng trên nhãn giấy. Ở bố cục expanded (rộng từ
  600dp và cao từ 720dp), bìa và action chung trục, rộng tối đa (360dp).
  Nhãn cao theo nội dung; khoảng tên–người mở (24dp). Chiều cao tối thiểu
  để giữ dáng bìa nằm ở vùng vải bao ngoài nhãn, không kéo tờ nhãn thành
  khoảng trắng dài. Đây là cách áp dụng bìa đóng tự cao đã có, không đổi
  hình học bìa đang lật.
- **Form theo người nhận.** New/Invite/AddFriend giữ cột form (560dp) và
  nhịp khối (24dp). Trên phone, header và action ở khe riêng của
  `RudiScreen avoidKeyboard`, footer khi IME đóng dùng `insets.bottom + 12`,
  mở IME theo khe (8dp) của shell. Expanded đặt action ngay sau vật giấy
  trong nội dung, không giữ một nút ở đáy xa ô nhập. Tài khoản → người
  nhận đã tìm thấy → gửi → chờ đồng ý; chỉ hiện thành công sau response.
  Thành công kết bạn và recovery lời mời đã tồn tại đều tới Bạn bè/Đã gửi.
- **Người dẫn danh sách.** Members đặt roster và vai trò trước lối sang
  tường/album; mời/thêm bạn là `IconButton quiet` (48dp) ở header cố định.
  Vai trò dùng `note inkSoft` tự xuống hàng, không con dấu cắt chữ.
  Ghi chú quản trị là câu nhẹ, giải thích dài mở tại chỗ. Nút đổi vai trò
  đứng riêng khỏi nút hồ sơ; ở cột hẹp/chữ lớn chuyển dưới tên. Friends giữ
  icon nhắn tin cạnh người; cặp Đồng ý/Từ chối có thể xuống dưới cột chữ.
- **Quyết định ngắn thành một cụm.** Trong cài đặt nhóm, bước Rời nhóm
  thay nội dung cài đặt bằng tiêu đề, hậu quả và hai lựa chọn. `Sheet`
  lấy chiều cao nội dung dưới trần; (82%) là trần, không phải chiều cao
  bắt buộc. Hai nút ở `footer` ngoài vùng cuộn, cách prompt (24dp),
  phá huỷ `warn outline` và Ở lại `ghost`. Back của bước này trở về cài
  đặt; không bật `prioritizeFooter` vốn dành cho editor cần chiếm trần.

Fresh verdict **ship chỉ sáu fixes**: reviewer mở 6 file chứa 16 viewport.
F3/F5 đọc ảnh d91; F1/F2/F4/F6 giữ provenance ảnh c941 và năm artifact
byte-identical, không đổi nhãn ảnh cũ. Lượt documenter này mở hai ảnh New
expanded sáng/tối và contact sheet invite/accept/leave/cancel gồm sáu
viewport, không vận hành runtime. Exact d91 có subset kỹ thuật 9 bước,
Node (1608/1608) và repo-tree (5107); regex TSX `[]` không chứng minh full
gate, geometry, contrast hay chất lượng thị giác. H/A–S, motion, TalkBack,
iOS, 390/430dp, phần cứng expanded độc lập, expanded chữ lớn/IME,
network/retry/lifecycle và phần chưa chụp vẫn mở.

### Chọn người và cùng mở sổ — I1, bàn giao giới hạn 10/10/2026

Nguồn là `ChonNguoi`, `KhongGianGiay`, `DongYBac`, `ChuKy`, `StampButton`
và ánh xạ trạng thái sổ tại `7dcacedf164f1a84192be62392c6dbfa4148174e`.
Giữ giấy/mực/bìa vải/coral/Bricolage và Nếp canonical; mockup06 là tham chiếu
phản biện, chưa phải comp được duyệt. Thẩm quyền hướng hiện tại là Creative
North Star trong file này và [I1 brief](docs/design/nep-ui-implementation/redesign-20261010/I1-PAIR-CONSENT-BRIEF.md),
không phải contract lịch sử đã rời `_layout.tsx`.

- **Chọn người rồi mới giao kèo.** Hàng picker giữ hình nhân (46dp), tên,
  chevron và kẻ tóc; không lưới thẻ 2+1 hay caption lặp dưới mọi tên. Trang
  chưa mở cho hai tên dẫn mắt, bìa đóng nhỏ mang nhãn «Hai/mình». Grain giấy
  sáng/vải tối vẫn là chất liệu journal hiện có.
- **Hiểu quyền trước khi ký.** `TrangSo ke={false}` chứa «Cho phép» rồi
  «Không kéo theo», sau đó hai phần ký cách cụm quyền (24dp), cuối cùng
  «Im lặng không phải đồng ý». Người đề nghị thấy câu chờ có tên người kia;
  người nhận có «Xem lời đề nghị» trên trang và «Đồng ý» trong giao kèo.
  Phản hồi proposal và lỗi ở footer để còn đọc được khi body ở đầu quyền.
  Vai trò/tên/câu chờ/trạng thái ký đi theo Typography; nét vẽ không giả
  chữ ký tay và một bên ký chưa mở sổ.
- **Mặt sổ theo response.** UI đọc trạng thái sổ từ provider/ánh xạ hiện có,
  không tự mở sau cú bấm. Mở sổ không tự bật Một đôi hoặc quyền đọc chat;
  giao kèo Một đôi nói AI chỉ nhận phần người dùng chọn và xác nhận gửi.
  Back/«Để sau» đóng lớp, không cấp quyền. Bằng chứng proposal/cancel/receiver
  grant và M6 mở thật thuộc `62c1a73b`, chỉ `lap_so`; không reset hay diễn lại
  cặp active để gắn nhãn bằng chứng mới.
- **Sau mở, một hành động dẫn.** «Rủ hội mình đi chơi» là con dấu có thể
  wrap; «Chọn loại sổ» là dòng mực phụ căn trái, cả parent và nhãn stretch
  trong cột, `minHeight` (48dp), đệm dọc (12dp). Nhãn đọc trọn trên một dòng
  ở các ảnh 100/200 đã mở; đây không phải luật cấm wrap ở mọi cỡ cửa sổ.
  Cảnh M6 cho bìa (140dp) và Nếp wrap theo khoảng trống, dấu «Sổ đã/mở» có
  hai dòng có chủ ý. Ảnh tĩnh không xác nhận chất lượng chuyển động.

Fresh verdict `62c1a73b` chấm bảy finding: sáu resolved, F5 partial do nhãn
phụ bị co. Verdict `7dcacedf` đóng F5 và regression hẹp trên 3 contact/24
viewport Android mới, hai phía sáng/tối font100/200; sáu finding còn lại giữ
nguồn ảnh62. Documenter mở đủ ba contact mới và hai contact62 (entry dark200,
recipient từ lời đề nghị tới mở sổ), không chạy runtime hoặc detector.
Exact clean7d có 9 kiểm tra xanh, Node (1608/1608), repo-tree (5108);
identity (12/12), canary (10/12), hai mutant (11/12, 10/12) đỏ đúng dự đoán.
Đây là subset kỹ thuật; regex TSX `[]` chỉ quét source, không đo contrast
native. APK giữ checkpoint độc lập `2f20a301`, fontScale đã trả về 1.
I2/toàn I/A–S, web/iOS, motion, TalkBack, lifecycle/network, full npm và
strict32 vẫn mở; bảy finding đóng không thành chứng nhận đẹp toàn app.

### Mục lục và quyết định trên giấy — I2, bàn giao giới hạn 10/10/2026

Nguồn cuối là `814ea5c979629acb8548215c24a5ff5f5c560b73`; lát gồm
`KhongGianGiay`, `LoaiSo`, `RangBuoc`, `GuHaiBan`, `DongSo`, `ToLoiRu`,
`ui.tsx` và `Stamp`. Hợp đồng layout/harden, Operate + Read trong
[I2 brief](docs/design/nep-ui-implementation/redesign-20261010/I2-NOTEBOOK-SETTINGS-BRIEF.md)
kế thừa UI3/seed `c8e88116`, giấy/mực/coral/Bricolage và Nếp canonical.
Mockup06 là critique-reference, không approved comp mới; không thêm art,
token hay thẻ trang trí để bù cấu trúc.

- **Đọc đúng điều đang có.** Mục lục gom ba nhóm bằng proximity/hairline
  như Layout. «Chọn loại sổ» căn trái, target source (48dp); `LoaiSo`
  highlight Hội bạn/Cặp đôi theo trạng thái thật, không theo lời đề nghị
  hay cú bấm vừa lỗi. Busy/disabled và lỗi thuộc sheet đang thao tác;
  «Xong» ở footer. Native timing của loading chưa được chấm.
- **Điều cần tránh giữ nội dung.** Hai `Field` multiline tự cao theo
  content-size từ 3 đến 8 dòng `body.lineHeight × fontScale`; vượt khung
  thì cuộn, không ellipsis một dòng. Người kia là phần đọc riêng sau
  hairline. Nháp chỉ hiện Lưu khi khác dữ liệu đang lưu; «Bỏ thay đổi»
  đóng lớp, không gửi. Lỗi và đúng nút lưu ở footer trên IME; các ảnh200
  chỉ chứng minh đoạn đang đọc/caret cuối, không toàn chuỗi cùng hiện.
- **Gu dẫn bằng quyết định của mình.** Own state → phạm vi cho người kia,
  Nếp và chat → bật/tắt hoặc bật lại nếu đủ điều kiện → sửa gu → tham khảo
  người kia/gu chung. Private dùng nút solid, shared dùng outline; mỗi
  người chỉ tự bật gu của mình. F1 gom lỗi và chính control hiện có vào
  footer cố định, `prioritizeFooter` chỉ khi có lỗi; không thêm lệnh retry,
  rút câu quyền hoặc đổi handler. Ảnh tối200 mới cho thấy toàn câu lỗi và
  nút hai dòng cùng hiện, own state vẫn đọc được khi cuộn thân.
- **Đóng sổ là đọc hậu quả trước.** Số dương dùng `ink`/600 nổi hơn dòng
  zero `inkSoft`, vẫn giữ đủ tờ chờ, buổi đã chốt, lời đề nghị và ký ức.
  Chưa có preview thì nói đang đếm/lỗi và không mời đóng; lệnh dùng revision
  của preview. Bằng chứng I2 chỉ đọc preview/cancel, không thực hiện đóng
  sổ hay cấp đồng thuận để lấy ảnh. Tem theo opt-in riêng ở mục `Stamp`.

Full review a24 mở 11 contact chứa **84 viewport tĩnh + 12 tương tác**,
20 original và mockup06; trả **fix** cho một P2 F1 của Gu. 84 ảnh tĩnh thuộc
nội dung tổng hợp (106/v2, 91/v1); 12 frame tương tác sau đó theo tiến trình
thật, retry lưu lên (116/v3, 101/v2). Không gọi ảnh lúc IME còn mở là ảnh
sau lưu. Verdict814 mở 2 contact/**12 frame Gu mới** và 4 original, chấm
**F1 resolved / ship** cùng regression hẹp. Chỉ `GuHaiBan` đổi trong 8 target;
bảy target còn lại carry nguyên byte từ a24. Original kiểm lại frame đã
tính, không cộng thành viewport mới; các ảnh top/bottom là viewport cuộn.

Documenter mở đủ hai contact814 và tám contact đại diện a24 (mục lục hai
theme, couple100/200, expanded sáng100 và ba contact tương tác); đối chiếu
hash 8/8 contact tĩnh, 12/12 functional original và 12/12 Gu original.
Chỉ đọc số kỹ thuật có sẵn ở từng SHA: 9 clean-check xanh, Node subset
(214 file, 1608/1608, 0 fail/skip/cancel/todo); Gu identity (12/12), canary
(10/12), hai mutant (10/12, 11/12) đỏ đúng dự đoán/cùng test SHA. Detector
a24 có 8 target,814 có 1 target Gu, đều regex TSX `[]`/exit0. Những số này
không là full gate/strict hoặc điểm đẹp. Không có tổng beauty /16; artwork
N/A và interaction/motion N/E không được đổi thành điểm2. Toàn I/A–S,
native computed contrast/geometry, target thực, motion/reduced motion,
TalkBack/focus order, loading timing/lifecycle, iOS và hardware chưa đo;
expanded (768×1024) là cùng AVD đổi kích thước.

### Hỏi tại hàng (xác nhận hành động phá huỷ)
Hành động xoá thứ không lấy lại được **hỏi ngay tại hàng**, không mở hộp
thoại, không toast. Một primitive: **`ui/HoiTaiHang.tsx`** (B11, 04/10), dùng ở
`community/Keeps.tsx`, `community/Comments.tsx`,
`screens/tuong/BaiChiTietScreen.tsx`, `screens/groups/Members.tsx` (tự bỏ
quyền quản trị) và `screens/groups/Conversations.tsx` (từ chối lời mời):
- Chạm nút `warn` (thường `ghost`) thay chỗ của nó trong hàng bằng **một câu
  hỏi `ink`** nói cái mất và vì sao không quay lại được («Xoá ghi chép này?
  Không lấy lại được.», «Bỏ quyền quản trị của bạn? Sau đó bạn không tự lấy
  lại được; một quản trị khác phải đặt lại cho bạn.», «Từ chối lời mời vào
  <nhóm>? Nếu đổi ý, bạn cần được mời lại.»), cỡ `note`, rồi **hai nút
  `compact full={false}`**: động từ phá huỷ `tone="warn"` **`outline`** và
  «Thôi» `ghost`. Nút phá huỷ mang `loading` tại chỗ trong lúc chạy. Nút mở
  câu hỏi cũng mang màu `warn`.
- Câu hỏi hiện ra thì focus tới nó: trên web vào «Thôi» (câu trả lời không
  làm mất gì), trên điện thoại trình đọc màn hình đọc chính câu hỏi.
- Thất bại thì `CauTaiCho` ngay dưới **đúng hàng đó** (cỡ `nho` trong
  luồng/danh sách dày), kèm «Thử lại» khi bấm lại có thể đổi kết quả.
- Một hàng hỏi một lúc: mở câu hỏi ở hàng khác thì câu cũ đóng (state là id
  của hàng đang hỏi), và mở câu hỏi xoá lỗi cũ của hàng.

### Mục tiêu chạm và trình đọc màn hình
Mọi node bấm được ≥48×48dp — kể cả `TextInput` bên trong `Field` (52dp hộp,
48dp ô) và **ô nhập của `ONhapMuc` (48dp, QA UI-001 đo 44)**, ô soạn chat, pill dưới tiêu đề chat, pill điểm đến.
Đợt 01–04/10 nâng các đích còn 44 lên 48: nút «Khớp» của bản đồ hành trình,
hàng người ở Bạn bè, hành động của bình luận, dòng kèo gọn ở chat, mũi tên
chương của sổ hành trình. **Lưới ngày của `ChonNgayLich`**: mỗi hàng cao
**48dp**; bảy cột chia bề ngang thật của tháng, tối đa 48dp mỗi cột (ở 320dp
khoảng 38dp, đích bấm 38×48). Hai nút tháng 48×48. Trước 04/10 là bảy ô 44 cố
định, cộng đệm và viền ≈ 326dp, tràn cột 288 của màn 320. Một câu chỉ là
`Pressable` khi còn việc để bấm (`Pressable` bị `disabled` vẫn là «nút» với
TalkBack). Hai control cùng chữ trên một màn phải khác nhau ở `accessibilityLabel`
(«Đánh dấu Minh Anh đã trả»). **Hành động lặp trên mỗi hàng giữ nhãn nhìn
thấy ngắn, tên trợ năng mang tên của hàng**: «Đặt làm quản trị» → «Đặt
<tên> làm quản trị», «Bỏ quyền quản trị của <tên>», «Bỏ quyền quản trị của
bạn» (`Members.tsx`); «Đồng ý vào nhóm» → «Đồng ý vào nhóm <tên nhóm>»,
«Từ chối» → «Từ chối lời mời vào nhóm <tên nhóm>» (`Conversations.tsx`);
«Xóa» → «Xóa bình luận của bạn: «<40 chữ đầu>»» (`Comments.tsx`). Đo bằng `scripts/a11y_native_audit.py`.

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
- **Nhịp của tờ AI trong luồng chat** (11/09, tái audit Codex 10/09 R5 và §2
  «Tin nhắn»: khối AI dài, nặng, lặp trạng thái nháp): tiêu đề ở cỡ
  `title` (trước là `h2`) — trong một luồng, tờ nói ở cỡ một tin nhắn; áp cả
  khung `aiSheet` fixture (Group) và tiêu đề lịch trình live của `TheAi`
  (`the.tieuDe`) và câu hỏi của thẻ bình chọn (`the.question`). Thứ tự: tiêu đề → **một** dòng gist («3 ngày 2 đêm · đồ ăn
  local · săn mây») → quyết định ngay (`RudiButton soft ai` «Xem lịch trình»
  kéo hàng + `RudiButton outline ai` «Bình chọn» vừa chữ, **viền
  `lineStrong`, tông chỉ ở chữ** — viền tím sắc hơn nền tô nhạt nên khi xếp
  dọc nó thắng mắt (finish review 11/09); a11y «Mở bình chọn»; ở chữ lớn hai
  nút xếp dọc, mỗi nút hết cột) → lý do sau **một cửa
  mở** («Vì sao phác vậy»: `Pressable` 48, `label inkSoft` + chevron,
  `accessibilityState expanded` — cùng hình «Cách tính» của Thành tích) →
  chân ký `caption ai` «Rủ Đi AI» + badge «AI nháp». Thân **không** nhắc
  «nhóm sửa được trước khi chốt»: badge đã nói nháp một lần.
- **Hành động phụ trong tờ mang chữ** (11/09, review Codex A4): nút bình
  chọn từng là `IconButton stats-chart-outline` không nhãn và được đọc thành
  «xem thống kê»; nhãn a11y đúng không cứu được mắt đang nhìn icon. Trong một
  tờ, mọi hành động có tên bằng chữ; icon-only chỉ cho hành động đã có quy ước
  toàn cầu trên chính màn ấy (tim, chuông, back, đóng, gửi).

### Trả lời của Rủ Đi AI trong luồng (`chat/TraLoiAi.tsx`, `ChipBoiCanh.tsx`)
ADR-0046 (đề xuất, chờ Lead ký), lát 7 của kế hoạch AI v2. Bản này là **lõi
dùng lại thành phần sẵn có**; hình hoàn thiện (hàng «đang đọc» có shimmer,
chữ chạy) là lát 12, làm ở nơi có `/impeccable`.
- **Tin `@Rủ Đi` là tin thường**: bong bóng của người gửi như mọi tin. AI
  trả lời bằng một tin `ai_card` kind `tra_loi` **trả lời vào đúng tin đó**,
  như một thành viên trả lời trong luồng.
- **Câu trả lời căn trái**, không avatar: trích tin tag ở trên (viền trái
  tông `ai`, nền `card`, cùng hình trích của tin thường), chữ là bong bóng
  `card` viền `line` bo 18; phần quán hay lịch trình là tờ `ToGiay` anh em
  (`TheAiView`), **không thẻ lồng thẻ**.
- **Ký ở chân**: `sparkles` 15 + `caption` tông `ai`, «Rủ Đi AI · đọc {n}
  tin» hoặc «Rủ Đi AI · chỉ đọc lời nhờ». `n` là số máy chủ đã kiểm
  (`doc.so_tin`), không phải số client khai. Trong chat cặp đôi, câu trả lời
  đã đọc gu đã chia nói thêm «· dùng gu của Linh» (hoặc «của Linh và Tú») từ
  nhãn máy chủ ghi ở `doc.gu` (ADR-0048 §3.5); không đọc gu thì không nói gì
  về gu. **Không mặt Nếp** (ADR-0036 §2.6), không màu mới: token có sẵn.
- **Chip xem trước trên nút gửi** thay khối «Mình đang thấy» của khay: một
  dòng nền `aiSoft`, chữ `ink`, hai chữ bấm được tông `ai` (cặp `ai` trên
  `aiSoft` qua 4.5:1, `mau-tren-nen-ai`), «Kèm {n} tin gần đây · Xem · Chỉ
  gửi lời nhờ». `n` đọc từ chính gói sẽ gửi; «Xem» mở `Sheet` liệt kê đúng
  gói đó. Vùng chạm 48dp đến từ `hitSlop`.

### Tờ giấy gấp ba (`ui/ToGiay.tsx`: `ToGiay`, `VetGap`, `GocGapThat`)
Tờ của **sổ hai người** (spec «Nếp truyền giấy» §1.6, §15.3, §16): một lá
thư gấp ba, các hàng của một lá thư ngăn bằng vết gấp. **Không phải `Card`**,
không phải tờ AI `ToGiay` cục bộ trong `chat/TheAi.tsx` (trùng tên, xem mục
cuối). Ở head này chỉ bảng `ui-lab` dựng nó; chưa màn người dùng nào.

- **Nền `paper`, mép tóc `lineStrong`, bo `radius.small`, đệm `space.md`, không
  bóng.** Vì sao `lineStrong` chứ không `line`: theo chú thích mã dẫn spec
  §16.2, `line` trên nền sáng và trên nền tối đo được đều dưới sàn 3:1 của
  cạnh phi-chữ, `lineStrong` qua sàn ở cả hai scheme (số của hệ ở mục «Sàn
  phi-chữ 3:1»). Vì sao `radius.small` chứ không `radius.base`: bản cắt đầu
  dùng `base` 20 và **đọc mù vòng 1 gọi ba tờ là «ba tấm thẻ»** («bo góc bốn
  góc đều»); bốn góc 20 đều là chữ ký của thẻ, và mọi tờ giấy khác trong app
  (`KyHoa`, `KhungAnh`) đã ở `small`. Elevation khai một lần: tờ có **mép**,
  không có bóng rơi. *Quyết định đọc mù; detector Impeccable `[]`.*
- **Góc gấp là góc CẮT, không phải badge** (`GocGapThat`, ô 22dp = `GOC_GAP`,
  cùng cỡ góc gấp bản in `KhungAnh`). Hai bản cắt cùng sai một cách: tam
  giác coral đặt **trong** góc mà viền tờ vẫn chạy tới góc vuông; hai vòng
  đọc mù gọi nó là «badge», «nhãn dán», «dog-ear trên bao thư», nền tối là
  «notification», và reviewer vòng 2 chẩn đúng cơ chế: «phần trắng của thẻ
  vẫn lồi tới tận góc». Giấy gấp ở góc thì **mất** góc ấy. Nên vẽ theo thứ tự
  cố định: (1) tam giác **xoá** bằng màu nền `nen` phủ lên góc (kể cả góc
  của chính viền, nhờ offset âm một hairline); (2) vạt `accent` là mặt sau
  lật lên; (3) mép cắt chéo hairline `lineStrong` **nối tiếp viền tờ**; (4)
  hai cạnh tự do của vạt bằng mực `ink` 1dp, lùi 0.5 để không bị clip nửa
  nét. **Góc trên phải vuông** khi mang nếp (`borderTopRightRadius: 0`): nếp
  gấp không thể bắt đầu trên góc bo. Không `overflow: hidden` (vòng 1: góc
  coral bị cắt cong). Vòng 3 đọc: «góc giấy gấp lại, tam giác đỏ ở đúng vị
  trí góc bị mất, không phải badge vì không tròn và không nổi lên trên
  viền»; nền tối: «không badge, không chấm thông báo». *Quyết định đọc mù đã
  khép sau ba vòng; chưa cổng đo.*
- **`nen` là cái lộ qua chỗ cắt**, mặc định `colors.ground`; tờ đặt trên bề
  mặt khác phải gọi tên bề mặt ấy. Tờ không tự biết nó nằm trên gì.
- **`VetGap` là vết gấp, không phải divider**: một `View` hairline
  `paperShade`, `marginVertical: space.sm`, **`marginHorizontal: -space.md`**
  để chạy xuyên đệm **mép tới mép** chạm viền `lineStrong`. Vết dừng ở lòng tờ
  là «gạch phân cách hàng» (đọc mù vòng 1); reviewer vòng 2 đo pixel xác nhận
  vết đã chạm viền hai bên, và vòng 3 đọc thành «dòng kẻ của tờ». Là `View`,
  không SVG: bản sáng không có màu nào sáng hơn `paper` để bắt sáng (spec
  §16.3), nên vết là **cùng một nét ở cả hai scheme**, không có trick đổ bóng.
- **Dòng lý do đứng DƯỚI tờ, NGOÀI tờ.** Ba hàng của tờ phải đều; lý do nằm
  trong hàng ba làm ba hàng lệch (vòng 1, sửa #4). Vòng 2 đo 24px trên / 54px
  dưới và đọc chú thích thuộc «khối phía trên». *Quyết định đọc mù, luật bố
  cục của màn gọi, không phải của component.*
- **`dan` do màn hình quyết; một coral dẫn mỗi surface.** Component không bao
  giờ tự đặt `dan`; motif `thuGapBa` không mang coral. Vòng 1 có hai tờ `dan`
  trên một mặt (sửa #3). Vòng 3: «tờ đầu là việc bây giờ nhờ tam giác đỏ ở
  góc, dấu duy nhất». Cùng luật với «Luật Một Tông Dẫn» của Colors.
- **Không chữ trong container.** `ToGiay` không có nhãn của riêng nó, không
  xuất hiện trong cây trợ năng như một label; các hàng bên trong nói.
- **Ba tờ không `dan` xếp đều vẫn có thể đọc thành danh sách thẻ** — điều
  còn treo cho Phase 2 (spec §20.5 phép đo 2); đòn bẩy vật chất duy nhất còn
  lại là ngữ pháp góc gấp ở tờ dẫn. Ghi để người sau không «sửa» bằng cách
  thêm bóng hay đổi bo.

### Dock Nếp: tờ giấy cài trong lề sổ (`nep/NepDock.tsx`)
*Extension build trong thế giới đã có (ngữ pháp ToGiay): không roll concept mới;
dock kế thừa vật liệu, nếp gấp và lề của ToGiay.* Nếp là «mẩu lời hẹn gấp
giấy», tờ giữ chỗ cho mình (mục «Lớp vẽ» ở trên). Nên dock **không phải nút nổi
trên trang**: nó là một tờ giấy **cài vào lề phải** của trang, như mẩu giấy
đánh dấu chỗ đang đọc trong một cuốn sổ thật, và mọi trạng thái là cùng tờ ấy
cài sâu hay nông. Câu chuyện gốc là «Chừa một chỗ cho nhau»: khi trang đặt một
tờ khác lên mình, tờ của Nếp rút vào sổ và nhường chỗ. ADR-0033 (dock bám mép)
và ADR-0035 (cài trong lề) là nguồn; reducer ở `nep/trang-thai.ts`, hình học ở
`nep/dock-vi-tri.ts`.

- **Lề trang là ngân sách đo được, không phải phong cách.** Đo trên bản chạy
  (23/09): giờ tin nhắn trong hội thoại kết thúc đúng **16dp** từ mép phải
  (`LE_TRANG` = `space.md`). Nên mọi thứ nằm nghỉ phải nằm gọn trong 16dp:
  mép cài lộ **10dp** (`NEP_MEP_HEP`), có việc thì thêm tờ thứ hai lộ **4dp**
  (`TO_SAU_LO`), tổng **14dp** (`NEP_MEP_DAY`). Vùng chạm mượn phần lề còn
  lại và dừng đúng ở lề: `slopTrai` = 16 − 10 = 6dp khi cài, **0** khi đã kéo
  ra. Kéo ra, tờ rộng **56dp** (`NEP_DIA`, bằng con dấu tạo của thanh tab) ×
  cao **64dp** (`NEP_TO_CAO`: tờ, không phải đồng xu), Nếp tư thế `doi` cỡ 44
  đứng trên tờ. Ray dọc bên phải, cách đỉnh 16 và cách thanh tab 24 (thanh tab
  là control, header thì không); vị trí lưu là **tỷ lệ 0..1** trên ray
  (`rudi.nep.dock.v3`, chỉ `tyLe`), nên xoay máy hay đổi máy vẫn về đúng chỗ.
  *Lịch sử:* bản đầu nghỉ dạng đĩa 57dp và cắt «20|0», «22:|» trên Khám phá,
  vùng chạm nuốt «Đồng ý» của lời mời (flow 25).
- **Ba trạng thái, một chỗ nghỉ.** `an` (mặc định, chỗ nghỉ duy nhất) chỉ lộ
  mép, **không vẽ Nếp**. `nghi` (đã kéo ra) là **lối đi tới bảng, không phải
  chỗ đứng**: đóng bảng, đổi màn, một tờ khác đóng lại, hoặc **6 giây** không
  chạm lần hai (`TU_CAT_MS`) đều đưa về mép; đang kéo dọc thì đồng hồ dừng;
  trên Android/iOS khi bật trình đọc màn hình thì không tự cất (web không biết
  được, vẫn tự cất); **không bao giờ lưu xuống đĩa**, mỗi lần mở app bắt đầu
  cài. `mo` là bảng đang mở, dock không vẽ. **Không có trạng thái `he`**: dòng
  hé bốn giây từng rộng 234dp và nằm đè giá, giờ mở cửa của thẻ; việc **không
  bao giờ làm Nếp nở rộng**, không trạng thái nào rộng hơn 56dp đến được mà
  người dùng không chạm.
- **Vật liệu là giấy của `ToGiay`.** Mặt `paper`, viền hairline `lineStrong`,
  **không bóng**: theo «Luật Trong Trang / Trên Trang», bóng thuộc về bản in
  *dán lên* trang, còn tờ này nằm *trong* mép trang. Góc trên trái **gấp**
  theo ngữ pháp `ToGiay` (góc mất khỏi đường viền, viền rẽ theo đường chéo,
  vạt `paperShade` nằm trên mặt, hai cạnh tự do của vạt bằng `ink` 1dp; ô góc
  8dp, nhỏ hơn mép 10dp để dưới nếp còn một dải giấy thẳng). Bo `radius.small`
  ở góc dưới trái; **cạnh chạy vào mép màn không viền, không bo**, vì nó chạy
  tiếp vào trong sổ. Vẽ bằng SVG chứ không View có viền: nếp gấp là góc bị
  thiếu, và tờ này trôi trên thẻ, ảnh, chat nên không có màu nền nào để «xoá».
  Đang nhấn thì mặt thành `paperShade`. **Không coral**: theo «Luật Góc Cắt,
  Không Badge», góc coral thuộc Nếp và `dan`, và góc coral của chính Nếp đã
  nằm trên tờ khi kéo ra.
- **Có việc là tờ thứ hai, không phải chấm đỏ.** Một tờ giấy ấm hơn một nấc
  trượt ra từ sau tờ Nếp, một nhịp `standard`/decelerate rồi đứng yên; hết
  việc thì cắt, không chào. Màu: `accentSoft` (#fff0ea) ở sáng, **`line`
  (#363b5e) ở tối**, vì `accentSoft` tối (#3d1a10) cạnh giấy xanh đậm đọc ra
  vệt gỉ sét (cùng lý do `AlbumAnh` đã loại). Không chữ, không số đếm. Không
  bao giờ hiện trên màn tiền, cạnh một tờ đang mở, hay sau bảng đang mở: bản
  ghi (`coViec`) và tín hiệu là hai sự thật, chỉ `hienToSau` nối chúng.
  *Hiện chưa gì trong app gửi việc cho dock; trạng thái này chỉ tới được qua
  bản build QA `EXPO_PUBLIC_QA_NEP_VIEC`.*
- **Màn tiền: chỉ mép trơn** (Luật Nếp Đứng Xa Tiền). Mép 10dp, không mặt,
  không tờ thứ hai, và **chạm hay kéo cũng không đưa mặt Nếp ra**: mép ở đây là
  cánh cửa về Nếp từ chỗ khác, không phải khuôn mặt cạnh con số. Luật nằm trong
  reducer (`luiLai`) chứ không trong component, để màn sau không thừa kế một
  Nếp đã bật ra cạnh quyết toán.
- **Nhường chỗ thì không vẽ gì.** Khi bất kỳ tờ nào nằm trên trang (khay,
  bottom sheet qua `ui/Sheet.tsx`, story, bản đồ; `useNhuongChoNep(true)`, có
  đếm lồng nhau), dock không vẽ gì, **kể cả mép**: mép còn vẽ đè góc khay cạnh
  nút ✕ là lỗi xếp lớp, không phải chiều sâu. Không báo gì; tờ cuối đóng thì
  Nếp về đúng chỗ người dùng để. Bảng của chính Nếp không bắt Nếp nhường.
- **Cử chỉ.** Chạm mép để kéo ra; chạm lần hai mở bảng (một chạm lỡ không mở
  cả một tờ đè lên trang). Cất: vuốt **ra ngoài** (> 56dp hoặc > 700dp/s),
  hoặc thao tác trợ năng **«Cất Nếp vào mép»**. Kéo dọc để dời trên ray.
  **Không bao giờ vuốt ngang vào trong**: dưới điều hướng cử chỉ, dải Back của
  Android đo được **29.7dp** và chỉ nuốt cú vuốt ngang vào trong (chạm trong dải
  vẫn tới app), nên nửa kéo vào trong bị kẹp bỏ chứ không giành; **không khai
  `setSystemGestureExclusionRects`**, cú vuốt ấy là nút Back của người dùng.
- **Web: `overflowX: "clip"` trên lớp dock.** Tờ cài rộng 56dp, 46dp nằm ngoài
  mép phải; bấm vào là focus, và trình duyệt cuộn ngang cả trang để lộ phần
  bị giấu (đo 24/09 trên `/finance`: trang trượt 46px). `clip` cắt tràn mà
  không biến lớp thành vùng cuộn; trục dọc để nguyên cho mép trên tờ thứ hai.
- **Cổng đo: `apps/mobile/tools/xem-dock-nep.mjs`** (puppeteer trên bản web,
  sáng và tối). Lúc nghỉ, trên tab đầu và trong hội thoại cuộn hết từng nấc:
  **0 chữ bị che** (153 và 363–414 hộp chữ đã quét ở lượt `dock-1225`); khay
  mở thì dock không vẽ; đóng bảng thì về mép; kéo ra rồi để yên thì 6 giây sau
  về mép; màn tiền chỉ còn mép 10dp, chạm không ra mặt; có việc thì không gì
  rộng hơn 56dp. **Canary:** kéo Nếp ra thì phép đo **phải** thấy chữ bị che
  (ảnh `thuong-dark-4-keo-ra`: tờ 56dp nằm trên «200…» và «22:…»); canary
  không đỏ thì cổng mù. Bằng chứng ở ngoài checkout
  (`~/.cache/rudi-bang-chung/dock-1225/`). *Chưa chứng minh:* ảnh story và bản
  đồ lúc nhường chỗ (mới là bằng chứng trên mã + test reducer), và bản native
  thật.

### Bản tính của sổ (`so/ban-tinh.ts`)
Chỗ **duy nhất** trong `src/` khai các loại sổ khác nhau ở đâu (spec §13.3,
§17). Không có «mode»; có nhiều sổ, mỗi sổ một loại, người mở sổ này hay sổ
kia.

- **Hiện hành 27/09:** `LOAI_SO = hoi | doi`; `pair` chưa bật đồng thuận đôi
  trở về `hoi`. **Lịch sử 12/09:** từng có `hoi | hai-nguoi | doi`, nay không
  còn chế độ `hai-nguoi` riêng. **Sáu trường** của
  `BanTinhSo`: `quyetDinh` (`phieu` | `to-giay`), `coVai` (hai vai Người lo /
  Người chấm hay không), `nhip` (`toMoiTuan`, `lanLaMoiThang`, `nhacMoiThang`;
  0 là không bao giờ), `nepDuocLam` (danh sách `ViecNep`, rỗng là im lặng),
  `tuVung` (`goiTapThe`, `cauMo`, `nutMoLoi`, `tenKhongGian`; tiếng Việt,
  không gạch dài, cổng `dau-gach-dai`), `tienHien` (`chia-bill` |
  `chi-tieu-chung`). `hoi` là bản đang ship, không đổi: Nếp giữ ghế và không
  tự nói (`nhip` 0, `nepDuocLam` rỗng). Nhánh `hai-nguoi` cũ từng thêm tờ
  giấy và `phac-to`; hiện chỉ `doi` giữ tờ giấy, hai vai và danh sách việc riêng.
- **`loaiSoCua(nhom, doi)`** suy loại từ hai sự thật máy chủ nói: `kind`
  (`group` | `pair`, ADR-0021) và cờ đôi đang bật; `pair` **không tự là đôi**
  (ADR-0027). **`banTinhCua(loai)`** trả bản tính.
- **Là chính sách trình bày, không phải phân quyền.** Nó nói màn hình đưa ra
  gì và gọi tên gì; một nút hiện ra chưa cấp cho ai điều gì; phân quyền là của
  máy chủ ở mọi biên đọc/ghi (ADR-0027 §4).
- **Cổng đo được** (`tests/so-ban-tinh-mot-cho.test.mjs`, 4 ca): ngoài
  `rudi/so/ban-tinh.ts`, **không file nào** trong `src/` so loại sổ, dù là
  `kind === "pair"` hay `loaiSo === "doi"` (regex neo vào **định danh**
  `kind`/`loaiSo`, không bắt chuỗi trần: `san === "doi"` của ký hoạ là sân
  khấu «đồi»). **Allowlist ba chỗ có sẵn** so `kind === "pair"` trước module
  này (`phien.ts`, `rudi/nhan-rieng/nhan-rieng.ts`,
  `rudi/screens/chat/CaiDatNhom.tsx`) được khai tường minh vì gom chúng là
  đụng màn hội bạn; cổng kiểm **hai chiều**: file ngoài danh sách mà so → đỏ,
  file trong danh sách mà **không còn** so → cũng đỏ, để danh sách không hoá
  di tích. Thêm: module là **lá** (không import tương đối) để chính sách
  không kéo màn hình vào.
- **Lịch sử 12/09:** chuỗi `tuVung` của `hai-nguoi` và `doi` **chưa lên màn nào**;
  là dữ liệu, không phải câu chữ đã đọc mù.

### Khoảnh khắc và Sổ chuyến đi (27/09/2026; cập nhật K 10/10/2026)

Đọc từ `diary/BookView.tsx`, `Wall.tsx`, `EndingScreen.tsx` và
`DiaryScreen.tsx`; đây là phần mở rộng UI v3 «Sân khấu giấy», kế thừa seed
`c8e88116` trong DESIGN.md, `theme.ts` và kit hiện hành, không có bộ token mới.

- Sổ chuyến đi dùng bìa vải `SoBia`, nhãn nền `card` chứa ảnh trên, lời dưới.
  Bìa đóng dùng `fitContent`: rộng hết cột, tối đa (420dp), nội dung nhãn quyết
  định chiều cao trong luồng với đệm dọc (24dp) mỗi phía; không đo chiều cao
  nhãn rồi hồi tiếp vào bìa. Đây là nhánh bìa đóng, không đổi hình học bìa có
  chuyển động mở. Tiêu đề `h1` (bản gọn `h2`), lời mở `body`, được xuống dòng.
  Ảnh bìa tỉ lệ 4:3, bản gọn 16:9; ảnh trang đơn 4:3, collage hai cột 3:4.
  Khung tỷ lệ và nền `paperShade` có ngay trước khi ảnh tải; ảnh lỗi giữ khung
  cùng câu báo tại ảnh, không làm co vùng đọc.
  Trang đọc dùng `TrangSo tone="accent" ke={false}`: giấy có lề, không kẻ ngang,
  tiêu đề `h2`. Lời kể dùng `body` với line-height (28), không đổi thang chữ chung.
- `DiaryScreen` đọc một trang đang chọn qua `BookView paged`: tên sổ `h2` và
  lời mở `note` ở trên trang, thay bìa ở chế độ đọc có trang. Khi có nhiều
  trang, chỉ một pager dưới trang ghi «Trang n / tổng» bằng `caption`, hai
  mũi tên (48×48dp) giữ bề ngang; counter được wrap. Biên đầu/cuối dùng viền
  đứt và hint trợ năng, không lặp câu biên hay số trang trên giấy. Sổ một
  trang không hiện pager. Bản xem trước chưa lưu vẫn dùng bìa và các trang;
  không áp bố cục paged cho `BookView compact` trên tường.
- Khoảnh khắc dùng ảnh dán `KhungAnh` nghiêng (-1°), ảnh tỉ lệ 3:2 và
  `Washi` nghiêng (-2°); tiêu đề `h2`, lời phụ `note`. Tường xếp một cột
  ảnh/bìa qua `BookView compact`, ẩn các trang bên trong; khoảnh khắc có
  nhãn tháng khi bắt đầu hoặc đổi tháng so với mục trước.
- Cột nội dung đọc `BookView` và các khối chọn/sửa `EndingScreen` rộng
  100%, tối đa (560dp), căn giữa; tường dùng cùng giới hạn cho mỗi ảnh/bìa.
  Khoảng cách giữa bìa và trang (28dp), trong trang (16dp), giữa mục tường
  (24dp). Nhãn quyền xem và các nút ngoài `BookView` ở trình đọc/tường
  không có giới hạn (560dp) riêng.
- Trang cuối dùng `RudiScreen cot="form"`, giữ cùng màn khi đổi bước/chế độ;
  `cuonVeDau` theo `phase` và `edit` đưa nội dung về đầu mà không remount.
  Header cùng cột form, ghi «Trang cuối» khi tải hoặc chưa khép cuộc đi.
  Tờ mời ghi tên cuộc đi và ngày thật: «Ngày hẹn» nếu cùng ngày, hoặc
  «Bắt đầu»/«Ngày về» nếu nhiều ngày; không dùng `RouteLine` trang trí.
  Chỉ khi `can_end` mới hiện bộ chọn loại và hành động khép; trạng thái chờ
  có lời dẫn theo ngày/quyền và lối «Về cuộc hẹn».
- Header nhỏ có icon đổi «Sửa theo cách mình nhớ»/«Xem như người đọc».
  Khi sửa, ẩn bản xem trước; tên sổ và lời mở ở vùng chung, rồi chỉ một trang
  đang sửa nằm trên `TrangSo`: pager (hoặc «Trang 1» khi chỉ có một trang),
  tên trang, chuyện của trang, ảnh đang chọn và toolbar cùng một tờ. Toolbar
  cuối trang dùng icon (48×48dp) cho ảnh, đưa trang lên trước và bỏ trang,
  có tên trợ năng theo việc/trang và đếm ảnh; không nhân thành form của mọi
  trang. Chuyển trang giữ bản nháp trong `document` của phiên đang mở.
  Tên sổ/tên trang dùng ô multiline ít nhất hai dòng; công cụ bìa được wrap,
  chuyển thành cột toàn ngang từ font scale (1.25), để nhãn được đọc đủ.
  Lựa chọn «Chỉ mình tôi»/«Công khai» nằm cuối phần xem trước hoặc sau các
  trường sửa, trong đoạn có kẻ mảnh. Nút «Lưu riêng tư»/«Đăng sổ công khai» dùng `StampButton`,
  giữ trong footer cùng lỗi lưu `CauTaiCho`; `footerInset={insets.bottom}`
  và `avoidKeyboard` dành chỗ cho vùng an toàn và bàn phím.
- Ô nhập dùng `ONhapMuc`; lúc đang lưu, trường sửa không cho nhập và các
  điều khiển sửa/quyền xem bị khóa. Ref `operation` khóa đồng bộ trước khi
  gửi để bấm nhanh không tạo request trùng. Lưu thất bại giữ bản đang soạn;
  lỗi nằm cạnh hành động lưu, chỉ lỗi có thể thử lại mới có «Thử lại».
- Bộ chọn bìa ghi «Bìa hiện tại», dùng vai trợ năng `radio` cùng trạng thái
  `checked`; chọn một ảnh thì đóng bộ chọn. Bộ chọn ảnh trang dùng
  `checkbox`/`checked`, nhãn «Đã chọn», tối đa bốn ảnh mỗi trang.
- Trình đọc đặt icon sửa yên ở header cho chủ sổ; «Quản lý sổ» mặc định
  thu gọn ở dưới vùng đọc, mở tại chỗ mới hiện gửi cộng đồng, cất riêng tư
  và xoá. Tường ghi rõ loại và «Chỉ mình tôi»/«Công khai» bằng chữ. Các hành
  động quyền xem và «Tự xếp trang, không gửi AI» giữ nghĩa đang có.
  Nhãn «Có Nếp giúp viết» nói nguồn hỗ trợ, không bảo đảm lời AI đúng.
  Sau dựng/lưu, khối sổ dịch nhẹ và trở về vị trí qua `useMotion`, theo
  thiết lập giảm chuyển động; không suy thành hiệu ứng lật bìa.
- Giữ luật không kicker/eyebrow trên tiêu đề. Nếu còn trong lát đang sửa,
  đó là lỗi cần dọn ở mã bởi phiên chính, không phải mẫu của hệ.

**Giới hạn bằng chứng lịch sử 27/09:** chỉ đối chiếu mã nguồn và tài liệu,
không chạy lại native hay mở lại ảnh. Theo bàn giao của người dùng, ma trận
emulator trước đó gồm điện thoại, tối/chữ 1.3, giảm chuyển động, màn nhỏ/chữ
200% và tablet đã được xem; thiết bị thật được người dùng hoãn. Ma trận đó
không xác nhận các sửa mới nhất về cột đọc/sửa, ẩn bản xem trước, vị trí quyền
xem/lưu, đặt lại cuộn và trạng thái chọn bìa. Chưa có kết quả native cho các
sửa này trong lượt ghi tài liệu; không suy rộng thành bằng chứng iOS.

**Phạm vi bằng chứng 02/10:** đối chiếu mã và mở sáu ảnh native B9a tại
`~/.local/share/rudi-b9a/`; xem [bàn giao B9a](docs/codex/2026-10-02/ui-ux-b9a.md).
Ảnh ghi trạng thái chờ, lỗi lưu, bàn phím và cột form trên Android emulator
điện thoại (411dp), màn ảo (320dp, chữ 1.3, tối, giảm chuyển động) và tablet
ảo (768dp). Đây là tinh chỉnh từ mã trong hệ hiện có, không có comp mới;
ảnh không chứng minh thiết bị thật, native iOS hay độ mượt khi chạy.

**Phạm vi K 10/10, source `5cab4701199a4ab9332fc20e28c0f0e3d67ddf89`.**
Các quy tắc K trên đối chiếu `BookView`, `EndingScreen`, `DiaryScreen`,
`SoBia` và `ONhapMuc`; hướng đi ở
[K-DIARY-BRIEF](docs/design/nep-ui-implementation/redesign-20261010/K-DIARY-BRIEF.md).
Không đổi world/seed `c8e88116`, token hay thêm raster trang trí.
[Verdict b76](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-b76f616d/fresh-k-verdict.md)
đóng finding 1/3/4 về chữ, hierarchy reader và nhóm trang sửa; finding 2
chưa đóng ở b76 vì nét kẻ vẫn xuyên glyph ở chữ lớn.
[Verdict 5cab](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-5cab4701/fresh-k-rule-verdict.md)
đóng riêng finding 2 sau khi mở (16) capture editor/shared caption bắt buộc,
không thấy regression mới trong phạm vi đó. Không đổi nhãn bằng chứng b76
về chức năng, quyền xem hay motion thành 5cab.

Lượt documenter đọc hai report và mở bốn mẫu native có sẵn: b76 editor-page-one
và reader-page-one light100; 5cab editor-page-two và moment-caption dark200.
Hai mẫu dark200 giữ chữ/dấu không bị nét kẻ nội thất cắt, gạch dưới còn rõ;
hai mẫu b76 cho thấy tờ sửa chung nhóm và một pager reader. Mẫu cuộn chỉ
chứng minh viewport của nó. Không vận hành emulator hoặc chạy lại gate.
Verdict chỉ gồm bốn finding và regression cụ thể trên Android (320×640),
light/dark ở font (1.0/1.3/2.0); không phải full K, toàn story/moment,
toàn app, /16, tablet/iOS, TalkBack, contrast hay blanket motion. Detector
đã báo trong report là regex TSX, không đo geometry/contrast native.

### Story viewer và xác nhận xoá — bàn giao giới hạn 08/10/2026

Ghi từ `screens/story/XemStoryScreen.tsx` tại
`c9a3f541d0a3e0a02aae608ed66f01f447b37005`, giữ cover indigo và giấy hiện
hành. Ảnh là nội dung chính: `Image key={story.id}` dùng `contentFit="contain"`
trong khung riêng, giữ tỷ lệ và bounds khi câu hỏi mở. Đầu màn chứa progress,
tên người đăng, tuổi story và nút đóng/xoá; controls chữ ở dưới ảnh, caption
trong vùng cuộn cao tối đa 180dp. Chữ lớn nhường diện tích cho nhãn/caption
nên ảnh nhỏ hơn theo tỷ lệ, không phải câu hỏi thu ảnh thành tem.

| Phần xác nhận | Artifact c9 đã ship |
|---|---|
| Lớp native | React Native `Modal transparent`, `animationType="none"`; phủ ảnh thay vì lấy chiều cao của khung ảnh |
| Giấy | Card bo 16dp, đệm 16dp, gap 12dp, viền 1dp `line`, `bongGiay(2, dark)`; lớp đặt cuối màn, lề ngang 16dp/đáy 24dp |
| Thứ bậc | «Xoá story này?» dùng `title` đậm 17/23 `ink`; câu «Bạn bè sẽ không thấy story này nữa.» là `body` 17/24 `inkSoft` riêng |
| Hai câu trả lời | Cùng `flex: 1`, cao tối thiểu 48dp, bo 12dp; «Giữ lại» nền `card`/viền `lineStrong`/chữ `ink`, «Xoá» nền và viền `warn`/chữ `accentInk` |
| Cách ly nền | Header `pointerEvents="none"` và ẩn khỏi accessibility tree, page zones disabled; toolbar nền giữ bố cục nhưng ẩn và không nhận chạm khi hỏi |
| Đường an toàn | Back đóng câu hỏi trước khi DELETE; «Giữ lại» trả về cùng ảnh. Source khoá request, disable hai nút và chặn đóng khi đang xoá; state DELETE chờ còn thiếu evidence native |

Đây là ngoại lệ của Story viewer đối với luật hỏi tại hàng: lớp native cách
ly controls trên ảnh, không biến Modal hoặc nền `warn` đặc thành mẫu cho
mọi hành động phá huỷ. Không thêm Nếp vào quyết định xoá. Câu hậu quả giữ
đúng phạm vi quyền bạn bè hiện hành; không chép «khỏi nhóm»/«không thể khôi
phục» từ reference khi domain không bảo đảm. Source gọi focus «Giữ lại»
khi Modal hiện, chưa thay bằng chứng TalkBack focus/trap/restore.

[Verdict c9](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-c9a3f541/story-independent-verdict-c9a3f541.md)
trả **`ship` chỉ hai P2: hierarchy câu hỏi và cách ly header nền đều
resolved ở question không pending**. Question tăng **13→14/16**;
preview/viewer **14/16** trong state đã chụp. Root/reviewer mở đủ **17 ảnh
after** Android portrait 320×640: viewer/question light và dark 1.0, light
font 1.3/2.0, Back/Keep, publish và xoá thật. Bản
[review 3b](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-3b71838e/story-independent-verdict-3b71838e.md)
vẫn giữ lịch sử `fix` hai P2; chuỗi composer replacement/draft sắc thuộc
`04f20b88`, không nhận đã chạy lại toàn chuỗi ở c9. Composer c9 có ảnh/caption
light 1.0 và publish, initial portrait còn ở offset cuộn; không suy toàn ảnh
vừa một viewport. Reference board10-panel1/2 là đối chiếu cấu trúc, không
chép artwork reference thành ảnh người dùng.

Năm flow c9 đều exit 0: light/publish **60,291s**, dark **57,836s**, font 1.3
**48,276s**, font 2.0 **48,326s**, delete thật **41,066s**. Xoá portrait về
rail, mở lại còn landscape/caption, không còn caption portrait; đây là state
đã kiểm, không nghiệm thu cả rail Tin nhắn. Runtime: JS c9, APK `b87a16f1`,
Go `54b5dc0e`; flow có `not_full_gate: true`.

Chưa phủ DELETE chờ/lỗi/bấm đôi/Back khi chờ, loading/error/empty/expiry,
TalkBack, motion normal/reduced/background/replay, retry-upload, audience
bạn bè, composer dark/font/bàn phím/crop và tablet/iOS/long caption.
Dark 1.0 cộng light 1.3/2.0 không thành dark ở mọi font. Plugin/detector và
craft-floor vẫn thiếu; không detector clean, không APPROVE toàn Story/app.

### Cards / Containers
- **Hàng + kẻ tóc là container mặc định** trên giấy. *Lịch sử tới 10/09:*
  tài liệu ghi `Card` v1 (bo 20, đệm 16, viền `line` + `cardShadow`) «không
  màn nào gọi» — sai: họ Cài đặt (`CaiDat`, `Phien`, `DaChan`, `VeRuDi`,
  `XoaTaiKhoan`) xếp 14 thẻ trắng nổi cạnh Cá nhân là hàng phẳng (tái audit
  Codex 10/09 §2 «Cài đặt», R5). **Hiện hành (11/09):** không file nào trong
  `screens/cai-dat/` import `Card`: nhóm mục là `NhomHang`, `Segmented` đặt
  thẳng trên giấy, lỗi là `body warn` trần, chân trang là `caption`
  (ảnh `native-r16/anh/r16-90-cai-dat-*`). `Card` còn trong kit với **hai
  người gọi là nợ có tên**: `story/DangStoryScreen` (khung ảnh) và
  `tuong/BaiChiTietScreen`; test in danh sách này để nợ không tàng hình.
- **`NhomHang`** (`ui.tsx`): nhóm hàng trên giấy — mỗi con nằm trong một ô
  `paddingVertical` 6 có kẻ tóc `line` dưới (`StyleSheet.hairlineWidth`);
  không nền, không bo, không bóng, không đệm ngang. Trích ra sau khi
  `Profile`, `DiemDenScreen` và `HangDiaDiem` đã vẽ tay cùng một hình ba
  lần và Cài đặt xếp `Card` bên cạnh; luật trích: **ba bản chép tay trở lên
  thì thành primitive**. Đứng cạnh `ListRow`/`Divider`: `ListRow` là một
  hàng, `NhomHang` là cái kẻ giữa các hàng, `Divider` là kẻ đơn ngoài nhóm.
- **`CoverBand`**: nền `cover` + `Grain vaiBia` 0.3, bo góc dưới 28, đệm trên
  `md` (+`insets.top` khi `underStatusBar`), đệm dưới `lg`, tràn lề theo
  `bleed`; `compact` rút vải khi bàn phím mở; chứa logo compact, `hero`
  `coverInk`, đoạn dẫn `body` `coverInkSoft`, nút back tròn.
- **`Sheet`** (v2 từ 01/10, một hợp đồng mở/đóng cho mọi sheet và khay):
  nền `card`, bo trên 20, đệm ngang `md`, đệm dưới
  `max(insets.bottom, 16)`; một hàng lỗ giấy `paperShade` 5×5 dọc mép trên
  (tờ xé khỏi tập, ẩn khỏi trợ năng); scrim `lopPhu.toi(0.42)`; `onClosed`
  nổ **sau khi** tấm đã rời màn. Hợp đồng v2, đọc từ `ui/Sheet.tsx`:
  - **Trần 82% là của cả tấm** (tay cầm, `dauTrang`, nội dung, `footer` và
    đệm đáy cộng lại), không riêng hộp cuộn: trước đó tay cầm và inset cộng
    thêm trên trần thành 92% ở 320×640, 96% ở 390×460 (UI-007, UI-040).
    `maxHeight` thay trần khi truyền; `avoidKeyboard` co trần theo khung còn
    lại trên bàn phím.
  - **Ưu tiên quyết định trong form (`eba8678a`) là opt-in**:
    `prioritizeFooter` mặc định `false`. Khi bật và có `footer`, tấm lấy
    `height = panelMaxHeight`; vùng tham chiếu cuộn dùng `flexBasis: 0`,
    `flexGrow: 1` và nhường phần cao còn lại cho editor/action ở chân.
    CTA phải nằm ngoài vùng overflow của editor; đây không phải footer cố
    định mới cho mọi sheet. Bằng chứng native hẹp xem bàn giao editor Nếp dưới đây.
  - **Trượt theo chiều cao của chính tấm** (đo `onLayout`, + 24), không một
    hằng 480 để lại một phần ba tấm cao trên màn (UI-013); **khép thì mờ** ở
    đoạn cuối (opacity theo `progress` 0 → 0.12 → 1), nên khung cuối trước
    khi gỡ không còn gì. Vào bằng lò xo `settle`, ra bằng `standard`.
  - **Rộng tối đa 640, căn giữa** ở tablet (đo bề rộng host, UI-093).
  - **Back đóng sheet trên mọi nền tảng**: nút cứng Android, Esc trên web, và
    **Back của trình duyệt** qua `lui-web.ts` (một listener `popstate` cài
    trước router, đặt lại mục lịch sử đang đứng rồi đóng lớp trên cùng, nên
    trang không rời màn: UI-038, UI-117). `onBack` cho một bước nội tuyến
    tiêu Back/Esc trước khi đóng.
  - **Màn mất focus thì sheet đóng** (đổi tab, route đẩy từ trong sheet), để
    không còn sheet mở trên màn không ai thấy giữ phần còn lại `inert`.
  - **Nền chết trong lúc mở**: 250 ms đầu (`CHAN_CHAM_MS`) cả scrim lẫn tấm
    không nhận chạm, vì cú chạm thứ hai của chạm đúp rơi ~60 ms sau lên thứ
    vừa hiện (UI-088, UI-006). Trên web, sheet là `role="dialog"`
    `aria-modal`, anh em của nhánh nó được `inert` + `aria-hidden`, focus vào
    control đầu, Tab quay vòng trong tấm, chỉ sheet trên cùng nghe phím; khi
    đóng chỉ trả lại thuộc tính **còn đúng là của nó** (Back trong lúc mở để
    navigator đổi trước, trả mù từng ẩn lại màn vừa về: UI-005).
  - **Lớp trên cùng của màn** (`zIndex` 10): dải ghim của chat không còn vẽ
    đè scrim (UI-070, UI-166). Nếp không vẽ đè sheet đang mở.
  - **Tay cầm là vùng nắm thật cho tay, không phải control cho trình đọc**:
    hàng cao tối thiểu 36 giữa hai ô 48, vạch 40×4 `lineStrong`;
    `Gesture.Pan` `activeOffsetY` 6, tấm đi theo ngón tay, thả quá **90 dp**
    hoặc vẩy **900 dp/s** thì đóng, ngắn hơn thì lò xo về; scrim mỏng dần
    theo kéo (`progress × (1 − kéo/cao tấm)`). Tay cầm **ẩn khỏi cây trợ
    năng** (`aria-hidden`; nhãn không role trên nó là lỗi axe
    `aria-prohibited-attr` ở mọi sheet, UI-089); lối ra trợ năng là nút
    **«Đóng bảng»** 48×48 (`close` 22 `ink`) ở góc phải cùng hàng. Vùng nắm
    chỉ là hàng tay cầm, nên cuộn của nội dung không đánh nhau với kéo
    (`clip-keo-sheet`).
  - **Thân tấm khép giữ lời cuối** (`useGiuKhiDong(open, value)`,
    `ui/giu-khi-dong.ts`): lúc mở, giá trị đi thẳng; lúc khép, giữ giá trị
    lần cuối còn mở. State đóng sheet thường đổi luôn điều thân sheet sẽ nói
    (sổ mở trong lúc «Lập sổ» còn lên làm thân đang khép vẽ lại thành lời mời
    đề nghị lại trong bốn khung, UI-084). Dùng ở `hai-nguoi/LoaiSo.tsx`,
    `DongYBac.tsx`.
  Đặt qua `RudiScreen overlay`, qua `<LenLop>` khi sheet được vẽ từ sâu trong
  thân màn, hoặc trong một route trong suốt.
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
- **Hàng bài Cộng đồng (`PostCard`)** (02/10): bài là **hàng trên giấy kẻ tóc
  dưới**, không thẻ; lề ngang 16 (= `space.md`, cùng cột với đầu Khám phá và
  Địa điểm nên đổi mục không xô cột), đệm trên 24 dưới 16, gap 14. Album lật
  **từng trang rộng hết cột, 4:3**, bo `radius.control`; nhiều hơn một ảnh thì
  viên «n/N» góc trên phải (nền `card`, bo pill, `caption ink`, ẩn khỏi trình
  đọc vì mỗi ảnh tự có tên). Hàng nút Thích · Bình luận · Chia sẻ, **dấu lưu
  bookmark đứng cuối hàng** (ô 48, `marginLeft: auto`, `bookmark` `accent` khi
  đã lưu, `bookmark-outline` `inkSoft` khi chưa, `aria-pressed`). Trên tab,
  đầu là `DauKhamPha` (chuông «Thông báo» có chấm và nút cài đặt bảng tin ở ô phải), rồi `SearchField`, rồi
  hàng chữ-tab `HangChuTab` năm chế độ (03/10, xem Navigation); viết bài đi qua con dấu «Tạo», màn không tự vẽ tiêu đề hay
  nút soạn. Mở theo chủ đề (route stack) thì giữ tiêu đề và nút soạn riêng.
  Sheet «Bảng tin của bạn» (nút cài đặt) từ 03/10 chỉ còn cá nhân hoá, xoá lịch
  sử đề xuất, «Bài đã ẩn» và «Điều mình muốn giữ»; «Đã lưu» và «Bài của tôi»
  rời sheet lên hàng tab. Nhịp sheet: tiêu đề `h1` cách câu dưới 6, 16 tới khối
  nút, 8 giữa mọi nút kể cả nút `ghost`.
- **Ô soạn chat** (Group): hàng bo 22 nền `card` viền 1px `line`, đệm 6, ở
  `footer` của màn; không có dải giấy trống thứ hai dưới nó.
- **Hoá đơn trên gỗ** (Bill): khung tối thiểu 420 bo 20 nền `giayHoaDon.khung`
  với ảnh gỗ, viền `lopPhu.trang(0.22)`; tờ hoá đơn gradient `giayHoaDon.nen`,
  chữ đen nâu hoá đơn, tổng tabular. Đây là **artwork của fixture** (theme.ts
  cho phép hex ở đây), không phải token.
- **`Washi`**: dải mép xé cao 30 (34/40 dưới wordmark), đệm ngang 18, rộng
  tối thiểu 120, tô `brand.coral`/`brand.teal`/`brand.violet` ở 0.9; con là
  chữ mực tối tĩnh. Chỉ Welcome dùng trong đợt này.

### Editor Nếp ở bài Cộng đồng — bàn giao giới hạn 09/10/2026

Artifact được review là `eba8678a3418271645f8332ce30261938c8a1fbc`, nguồn
`community/PostDetail.tsx` và `ui/Sheet.tsx`. Diff hai file từ `eba8678a` tới
HEAD `6f3f5af2` rỗng; giữ provenance JS của lượt editor, không gán ảnh cũ cho
SHA mới. Giữ hệ giấy/mực, Bricolage ở tiêu đề, body system và Nếp canonical;
không thêm hero/artwork cho form này.

Editor dùng `Sheet avoidKeyboard prioritizeFooter` với hai vùng cuộn:
đoạn bài/dẫn là tham chiếu nhường chiều cao; field và câu lỗi nằm trong
`ScrollView` riêng ở footer (`minHeight` 128, `flexShrink: 1`, gap 8,
đệm đáy 4). CTA là sibling **ngoài** vùng cuộn đó, nên «Đồng ý chia sẻ và
gọi Nếp» hoặc «Lưu ghi chép riêng» vẫn dành được chỗ trên IME.
`keyboardShouldPersistTaps="handled"` cho phép bấm action khi bàn phím mở.
Nháp có nhãn «Bản nháp do Nếp viết», sửa bằng tay trước khi lưu; lời nhờ gốc
vẫn sửa được trong vùng tham chiếu khi đã có nháp. Lý do nút tắt giữ ngay tại
form, không biến lỗi thành một lớp khác.

[Verdict editor eba8678a](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-eba8678a/nep-editor-independent-verdict-eba8678a.md)
chốt **ship chỉ P2-NEP-EDITOR-01, resolved**. Root và reviewer đã mở toàn bộ
17 ảnh packet; reviewer mở thêm ảnh lỗi cuối font 1.0, tổng 18 ảnh after.
Android portrait 320×640: sáng 1.0, sáng 2.0, tối 1.0; request/draft khi IME
mở đều thấy CTA đầy đủ và có thao tác bấm thật tới nháp/ghi chép đã lưu.
OS font được đặt trước fresh attach; chưa có telemetry số RN về font/window.

Runner sáng 1.0 **exit 1, 69.962s**: đã lưu thật, sau đó timeout vì selector
cuối tìm «Ghi chép riêng» trong khi màn có title «Điều mình muốn giữ».
Ảnh lỗi có ghi chép mới; không gọi runner này xanh. Sáng 2.0 **exit 0,
41.118s** và tối 1.0 **exit 0, 53.484s** dùng đích `community-keeps`.
Tất cả là focused native receipt `not_full_gate: true` ở SHA eba8678a,
không thay canary/mutant hoặc cổng repo.

Điểm tĩnh trong repair **9 → 12/14**; artwork/crop **N/A**, motion **N/E**.
Không quy đổi thành /16. **P3 còn mở ở font 2.0**: autoscroll tới caret có
thể đưa nhãn lời nhờ/nguồn nháp ra khỏi viewport và cắt nhãn/dòng đầu ở mép
vùng nhập. Chưa có lượt cuộn lại lên đầu riêng để kiểm mọi dòng/offset;
không nhận «không cắt chữ» hoặc toàn editor hoàn hảo.

Chưa kiểm font 1.3, tối 2.0, input rất dài/error/retry/mọi đường lưu;
TalkBack/focus trap/restore, target/contrast đo native, motion/timing/reduced/
background lifecycle, iOS/tablet, chất lượng lời AI và privacy. Plugin/detector
và craft-floor chính thức vẫn thiếu; không detector clean hoặc APPROVE toàn
editor/frontend.

### Community/Khám phá viewport đầu — bàn giao giới hạn 09/10/2026

Artifact được review là `6f3f5af2d1ae2b32145908f2da9409b254b1535d`, nguồn
`community/CommunityScreen.tsx`, `screens/explore/ExploreLive.tsx`,
`screens/explore/HangDiaDiem.tsx`, `adaptive.ts` và `ui/chu.ts`.
Giữ mockup ở vai trò concept reference và vật liệu yên của refinement hiện
hành; không thay token, shading, canonical Nếp hoặc rải grain khắp surface.

- **Community:** bài đầu đứng trước consent. Chỉ hỏi ở «Dành cho bạn» khi
  chưa trả lời, không có topic và có bài; feed rỗng không mang câu hỏi này.
  Consent giữ đủ phạm vi «Không đọc chat hay sổ riêng», body 15/22,
  gap copy 3, gap toàn khối 4, đệm dọc 8 và kẻ tóc `line`.
  Hai action là `RudiButton compact full={false} variant="ghost"` trong
  hàng `flexWrap`, gap 4. Nhãn nhìn dùng `Để${NBSP}sau`, nhãn trợ năng
  «Để sau» dùng khoảng trắng thường; handler vẫn `consent(false)`.
  Không khoá width/font để giữ nhãn.
- **Explore:** `compactLead` chỉ khi rộng dưới 600, cao dưới 700 và
  `!chuLon(fontScale)` (helper nhận chữ lớn từ 1.28 để bao cả bước OS 1.3).
  Nhịp khối rút về gap 12, cảnh còn cao 120 thay vì 150. Từ artifact
  `3bb47ef84bf294a242fbcc5f8910ceeb02478ca0`, đầu lead gọn đặt hình vuông
  112×112 bên trái, tên hai dòng/phụ đề bên phải `flex: 1`, `minWidth: 0`,
  đệm 12. Hai dòng sự thật và chip lý do/giá xuống dưới, dùng toàn chiều
  rộng thẻ (`leadGonDuoi`: gap 6, đệm 14, đệm trên 0). Tim giữ control riêng
  trên hình. Nhánh chữ lớn/cửa sổ khác vẫn dùng layout thường.

[Verdict compact 3bb47ef8](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-3bb47ef8/cd-finish-review/verdict.md)
chốt **ship chỉ C/D–01, resolved**, từ `screens/explore/HangDiaDiem.tsx`.
Documenter mở trực tiếp tám PNG trong
[packet](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-3bb47ef8/cd-finish-review/verdict-packet.json):
top/lead sáng và tối 1.0, lead sáng và tối 1.3, lead-lower sáng 2.0 và
lead-tail tối 2.0. Ảnh 1.0 cho thấy facts, lý do dài và giá dùng chiều rộng
thẻ; ảnh đại diện 1.3/2.0 vẫn có ảnh ngang toàn chiều rộng, và tail tối 2.0
hiện đủ giá kèm đơn vị sau cuộn. Đây là đối chiếu hình học tại các vùng đã
chụp, không nhận toàn surface/app, detector native, tương phản/target đo
native, motion hoặc iOS.

[Verdict gallery 6f3f5af2](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-6f3f5af2/gallery-independent-verdict-6f3f5af2.md)
chốt **ship chỉ P2-01 composition viewport đầu và P2-02 «Để sau», cả hai
resolved**. Root và reviewer mở đủ 10/10 ảnh packet, gồm fingerprint đúng
JS. Baseline Android portrait 320×640/font 1.0 sáng/tối: Community có tác giả
và bốn dòng body trước nav, consent sau bài đầu; Explore có tên hai dòng/fact
đầu và artwork rõ trước nav. Dataset/địa điểm dẫn đã đổi sau lưu gu thật,
nên đối chiếu hierarchy và vùng đọc, không yêu cầu nội dung pixel giống nhau.

Community top, Explore top, consent sau bài đầu và settings scheme snapshots
được chấm **14/14 tĩnh** ở baseline sáng/tối; interaction/motion **N/E**,
responsive/accessibility chỉ cấu trúc nhìn thấy. Ảnh font 2.0 chỉ chứng minh
đủ nhãn/control tại viewport đã cuộn: hai action xuống hàng riêng, không
chồng nhau. Không nhận toàn màn font 2.0 hoặc mọi breakpoint đã đạt.
Runner sáng **35.859s**, tối **19.539s**, font 2.0 **13.933s**, đều exit 0;
không thay full gate. Settings ảnh Tối/Sáng chỉ chứng minh state được chụp,
không toàn lifecycle đổi theme.

Không mở rộng verdict sang search/filter/save/error/empty/feed actions,
mọi route/layer, PostDetail/Nếp keyboard, privacy hay motion. Chưa TalkBack,
target/contrast đo native, iOS/tablet. Plugin/detector và craft-floor vẫn
thiếu; không detector clean, không quy /14 thành /16 và không APPROVE toàn
Community/Explore/frontend.

### Gu — đo nhãn theo giấy, bàn giao giới hạn 09/10/2026

Artifact được review là `d3c4a3aa3db5ee40ca28229103ae87f874a04dcd`, nguồn
`screens/Onboarding.tsx`. Sửa production chỉ thêm `alignSelf: "stretch"`
cho `styles.giua`, giữ `textAlign: "center"`, kèm comment đo nhãn.
Nhãn sticker, band ngân sách và dòng phụ lấy toàn bề rộng nội dung của giấy
để đo/wrap; không giảm font, đổi ngưỡng tiền, từ vựng hoặc handler.

Giữ câu hỏi Bricolage → dẫn system → tám sticker `GuGlyph` → count → bốn
phong bì → `StampButton`. Sticker hiện hành có GuGlyph 42, giấy note co theo
layout, grain chỉ trong object, dấu chọn riêng; sáng/tối dùng token của hệ.
Ở 320dp, sticker hai cột; ngân sách 2×2 khi font 1.0, một cột khi
`Math.round(fontScale * 100) >= 130`. Chữ lớn cuộn qua các vùng, không ép
toàn màn vào một viewport. Các mô tả Sở thích ở mốc cũ vẫn là lịch sử;
không lấy kích thước/chip của mốc đó thay cho artifact này.

[Verdict Gu d3c4a3aa](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-d3c4a3aa/gu-independent-verdict-d3c4a3aa.md)
chốt **ship chỉ sửa nhãn giữa và state tĩnh đã chụp, finding resolved**.
Reviewer context riêng đã mở **27 ảnh**: 24 after (top/three-selection/
budget/save-action, sáng/tối × font 1.0/1.3/2.0), reference board02-panel4,
fingerprint đúng JS và before130. Before130 chỉ thấy «Trên»; after130
save-action sáng/tối thấy đủ «Trên 500K» và «Rộng tay». Mép phong bì cuối
trong budget130 là vị trí cuộn; ảnh save-action cùng case cho thấy đủ nhãn.

Android portrait **320×640**, state **3-selected / `vua-phai` / enabled**:
**14/14 tĩnh** trên bảy trục quan sát được, interaction/motion **N/E**.
Không quy thành /16 hoặc vượt cổng đại diện toàn sản phẩm. Sáu receipt
after đều exit 0, cùng SHA, `not_full_gate: true`: sáng1.0 **17.988s**,
sáng1.3 **18.840s**, tối1.0 **18.038s**, tối1.3 **19.040s**,
sáng2.0-visible **25.798s**, tối2.0-visible **25.648s**. Hai lượt font2.0
đầu đỏ vì assertion count ngoài fold được giữ nguyên; lượt visible là capture
cuộn bổ sung riêng, không sửa lịch sử thành xanh hoặc coi đó là lỗi UI.
Font theo OS trước fresh attach, chưa telemetry số RN về font/window.

Packet và report nằm ngoài mọi worktree. Reviewer thấy PNG mode0664 ở lần
đọc đầu; sau phát hiện, parent siết quyền và reviewer kiểm lại **27/27 mode0600**.
Không gán quyền hiện tại cho thời điểm đọc ban đầu. Documenter đọc source,
packet/receipt và verdict; không tự chạy lại native hoặc chấm lại ảnh.

Save/cancel/restore tại `6f3f5af2` là bằng chứng chức năng riêng; diff handler
rỗng không kiểm lại flow đó. Chưa nhận toàn Gu/onboarding: new-account/name,
disabled/loading/error/offline/retry, TalkBack/focus/target/contrast đo native,
motion/reduced/lifecycle, native390/430/tablet/iOS hoặc toàn frontend.

**Đính chính availability 09/10:** runtime plugin/detector vẫn không tìm
thấy, `imp preflight` exit127; không có detector clean. Tuy nhiên craft-floor
**có và đã được đọc** tại
`/home/lakiet/.claude/plugins/marketplaces/impeccable/plugin/skills/impeccable/reference/craft-floor.md`.
Các dòng trước ghi «craft-floor thiếu» được giữ như lịch sử và được đính chính
bởi ghi chú này; không tiếp tục kết luận bản reference không tồn tại.

### Inputs / Fields

**Luật Nét Kẻ Nhường Chữ.** Nét kẻ trang trí của ô multiline phải nhường
chữ và dấu tiếng Việt. Từ scale hiệu dụng (1.25), `ONhapMuc` bỏ các nét
bên trong, giữ gạch ranh giới/focus dưới vùng nhập; không ép baseline native
khớp một nhịp trang trí bằng phép tính line-height.

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
- **`ONhapMuc`** (ô nhập của v3, «dòng mực, không hộp»; `ui/ONhapMuc.tsx`):
  chữ nằm thẳng trên trang, **ranh giới duy nhất là một gạch mực dưới chữ**:
  1dp `lineStrong` khi nghỉ (≥ 3:1 trên mọi mặt nó đậu,
  `test_contrast_floor.py`), **2dp `accent` khi đang viết** (đó là chỉ báo
  focus; vòng focus xanh của trình duyệt tắt qua `KHONG_VIEN_WEB` vì gạch đã
  nói «bạn đang viết ở đây»), 2dp `warn` khi giá trị sai. Dp gạch dày thêm
  lấy từ đệm dưới (4), nên chữ không nhích. **Ô nhập một dòng cao 48** (QA
  UI-001 đo 44; 4dp thêm cũng lấy từ khe chữ–gạch nên hàng không đổi cao).
  Chữ `body`; `co="lon"` viết giá trị bằng `h2` (tên trên bìa, số trên hoá đơn).
  Placeholder `inkFaint` do kit vẽ một dòng; tên trợ năng là nhãn hoặc placeholder;
  `helper` là `note inkSoft`; `error` (`caption warn`) thay dòng `helper` và được đọc lịch sự.
  Vùng `multiline` lấy chiều cao dòng chữ nhân scale hiệu dụng: (1) nếu tắt
  font scaling, nếu không lấy font scale cửa sổ và giới hạn bởi
  `maxFontSizeMultiplier` khi giá trị từ (1) trở lên. Chiều cao tối thiểu theo số dòng yêu cầu
  trong (2–8), tối đa tám dòng đã scale rồi cuộn. Chỉ khi scale dưới (1.25)
  mới có nét kẻ trang trí `line` theo nhịp đó; từ (1.25) bỏ nét bên trong
  theo Luật Nét Kẻ Nhường Chữ. Gạch control/focus/error dưới vẫn giữ nguyên.
  `oRef` để form bị từ chối đặt con trỏ
  vào đúng chỗ cần sửa. `Field` có hộp chỉ còn ở màn chưa làm lại.
- **`ChonNgayLich`**: một lá lịch xé (số ngày Bricolage 34/40) hoặc gõ tay
  `dd/mm/yyyy`; tháng mở **tại chỗ** dưới lá (sheet trong trang cuộn sẽ nổi
  dưới khung nhìn): bảy cột chia bề ngang thật, tối đa **48dp**, hàng cao 48
  (ở 320dp cột còn khoảng 38dp, xem «Mục tiêu chạm»), thứ Hai trước, hôm nay có vòng, ngày chọn tô; mỗi ngày là một
  nút đọc đủ «<thứ>, <ngày> tháng <tháng> năm <năm>», thêm «, hôm nay».
- **Lỗi**: một câu `warn` qua **`CauTaiCho`** ngay dưới control (xem «Trạng
  thái rỗng, tải, lỗi»); lỗi của một ô `ONhapMuc` là `error` của chính ô.
  Không toast, không modal. Đang tải: `loading` tại chỗ vừa bấm, hoặc `Skeleton`.

**Công tắc (`Switch`)** (04/10, critique B11): màu qua `ui/cong-tac.ts`
`congTac(colors, tone)`, một hình ở mọi nơi. Tắt là vạch `lineStrong` (sàn 3:1
của mép control; vạch `line` gần như mất trên nền kem); bật là tông của thứ
công tắc bật (`accent`, hoặc `ai` cho công tắc của Rủ Đi AI); núm là giấy
(`card`) trên cả hai nền tảng, web phải có `activeThumbColor` vì
react-native-web tự tô núm teal, màu của tiền.

### Navigation
- **`RudiTabBar`** tự vẽ: nền `card`, cạnh trên hairline `line`, cao
  **`tabBarHeight(fontScale)` + max(insets.bottom, 10)**: 64 ở cỡ chữ ≤ 1.15,
  rồi cộng `round((fontScale − 1.15) × 44)` tới trần 2.0 (`adaptive.ts`), vì
  nhãn được hai dòng thay vì «Khám …»; màn dưới thanh đọc cùng số qua
  `RudiScreen bottomInset="tab"`; **bốn cột** — Khám phá · Lên plan · Tin nhắn · Cá
  nhân — và con dấu «Tạo» **ở ô giữa của năm ô bằng nhau** (`thanh-tab.ts`
  `xepThanh`: số ô lẻ mới có tâm; 02/10, thay bố cục năm tab + dấu ở cột thứ
  ba). Bốn cột trong một `role="tablist"`, mỗi cột `role="tab"` +
  `aria-selected` (`tabState`), cao tối thiểu 48, icon Ionicons 24 (outline →
  filled khi chọn), nhãn 12/14 tối đa hai dòng; đang chọn `accent`, còn lại
  `inkFaint`; chỉ báo băng 28×4 `accent` treo ở cạnh trên cột đang sáng, trượt
  `standard` 200ms; haptic `select`.
- **Route không cột** (02/10): một route của tab navigator có thể không có
  cột (`href: null` ở `app/(tabs)/_layout.tsx`) mà vẫn giữ thanh, URL và deep
  link của nó; `MUC_TRONG_TAB` (`thanh-tab.ts`) nêu cột chủ, và **cột chủ sáng**
  khi route đó mở. Hiện chỉ có Cộng đồng → Khám phá. Chạm cột Khám phá **mở lại
  mục đang xem lần cuối** (Địa điểm hay Cộng đồng, chỉ trong bộ nhớ; mở app
  lạnh là Địa điểm). Chạm lại một cột **đang sáng** thì thanh không điều hướng,
  không haptic, và **màn của cột đó cuộn về đầu** (03/10, như app của chính
  điện thoại; `useChamLaiTab`): đúng với mọi cột, qua `RudiScreen
  bottomInset="tab"`; trên Cộng đồng cột Khám phá mở chính Cộng đồng (mục xem
  cuối) nên bảng tin (`FlatList`) về bài đầu. Dưới Reduce Motion là nhảy về
  đầu, không trượt. Test giữ map và layout đi cùng nhau.
- **Con dấu «Tạo»** (`ConDauTao`, B2 01/10, về đúng tâm 02/10): ô giữa thanh;
  tròn 56, nền `brand.coral`, glyph `add` 30 `brand.coralInk`, vòng 4px
  `ground`, nhô lên nửa trên mép thanh, nhãn «Tạo» 12/14 `accent` (một tên trên
  cả thanh lẫn rail). **Cả cột, con dấu coral lẫn chữ «Tạo», là MỘT nút**
  (03/10: `PressScale`, `testID con-dau-tao`, `accessibilityLabel` «Tạo mới»,
  chữ `importantForAccessibility="no"`): chạm chữ mở khay như chạm con dấu,
  giống mỗi cột bên cạnh mở trên chữ của nó. Nhấn thì cả cột co 0.92, để lại
  một vòng mực mở ra và tan trong `standard` (không có khi Reduce
  Motion). Chạm mở `/create?tu=<tab>`: khay đưa việc hợp tab lên đầu
  (`tao-moi.ts`; trên Cộng đồng là «Viết bài»); giữ lâu đi thẳng tới việc đó.
  Con dấu là nút **ngoài** tablist, phủ lên một ô rỗng `aria-hidden`.
  Spike (a) vắt góc / (b) cột giữa: chọn (b), lý do và ảnh ở
  `docs/claude/2026-10-01/ui-ux-upgrade/direction.md`.
- **Rail** (medium+): rộng 104, cạnh phải hairline; đầu rail là **hàng
  wordmark 56** (`Wordmark` `ink` cao 18, ngoài tablist), rồi ô 96 của con dấu
  (nhãn «Tạo»), rồi bốn hàng 72 với icon + nhãn `caption`; chỉ báo vạch 4px
  `accent` bên trái, đo từ lề trên của rail cộng 56 + 96 (lệch khi đo từ 0: QA
  UI-004).
- **Đầu Khám phá (`DauKhamPha`)** (02/10): hai mục «Địa điểm | Cộng đồng» là
  một `tablist`, cả hai chữ `h2` (một cỡ nên đổi mục không xô), gap 24; mục mở
  `ink` trên băng coral 28×4 bo 2 (cùng băng của thanh), mục kia `inkFaint`;
  mỗi mục cao tối thiểu 48. Ô phải (trên Cộng đồng: chuông «Thông báo» và cài đặt bảng tin, mỗi nút 48) nằm
  **ngoài** tablist; dưới 360dp ô phải lên trên hàng chữ. Chữ to vượt hàng thì hàng **cuộn ngang thay vì cắt**, và
  tự cuộn mục đang mở vào tầm nhìn. Đặt qua `RudiScreen header` nên đứng yên
  khi nội dung cuộn; mục mặc định là Địa điểm. Điều hướng giữa hai mục viết ở
  file route (bộ rút hướng dẫn của Nếp đọc ở đó), component chỉ vẽ.
- **Hàng chữ-tab (`HangChuTab`)** (03/10, chủ sản phẩm 02/10): các chế độ
  bảng tin Cộng đồng trên tab là **một hàng chữ-tab nhỏ** «Dành cho bạn ·
  Đang theo dõi · Thịnh hành · Đã lưu · Bài của tôi», một `tablist` dưới
  `SearchField`. Chữ `label`; tab chọn `ink` trên **gạch 2 dp `ink`** (bo 1),
  tab khác `inkSoft`; **không viền, không nền, không dấu check**, vì tab chọn
  danh sách *là gì* còn chip lọc thu hẹp nó; và thấp hơn đầu Khám phá một bậc
  (`h2` trên băng coral) nên hai hàng không đọc thành một control. Mỗi tab cao
  tối thiểu 48, chữ đặt thấp để gạch nằm trên **kẻ tóc `line`** chạy dưới cả
  hàng. Gạch là **một vạch trượt** (`viTriGach`) từ tab cũ sang tab mới trong
  `standard` (đường cong `standard`), như băng chỉ báo của thanh tab; dưới
  Reduce Motion và ở lần đặt đầu thì nhảy thẳng; khi bề rộng các tab chưa đo
  xong, mỗi tab tự vẽ gạch của nó nên khung đầu không thiếu chỉ báo. **Khoảng giữa các tab là số tính, không phải hằng**: thường 22; khi
  hàng rộng hơn cửa sổ, `khoangCachLo` chọn khoảng **12–32** gần 22 nhất sao
  cho **cả hai mép cắt ngang một chữ** (lộ ≥ 16, giấu ≥ 10) mỗi khi hàng cuộn,
  mép phải lúc nghỉ, mép trái khi cuộn hết về cuối; đệm cuối **16–24** (lề 16
  cộng tối đa 8 để đặt mép trái); hàng vừa thì giữ 22 và lề. Không fade, không
  mũi tên: chữ bị cắt là lời nói hàng còn tiếp. Chọn một tab đang khuất thì
  hàng cuộn (không animation) để nó vào tầm; với tab giữa hàng, **tab kề vẫn
  lộ một mẩu**. Chỉ ở `sizeClass` compact hàng mới **tràn hai mép màn** (chữ
  bắt đầu ở lề 16, kẻ tóc tràn theo); trong cột đọc của tablet hàng giữ trong
  cột, chữ và kẻ tóc như nhau. Chế độ không có tab («Bài đã ẩn») thì không tab
  nào sáng và màn in tiêu đề `h2` riêng; chạm tab đang chọn không làm gì; đổi
  tab haptic `select`. Ảnh `.impeccable/review/cdt-fix2/cdt7-*`,
  `phone-android-community-cua-toi-1.0/1.3`.
- **Bản demo** (chưa đăng nhập): không có cột thêm trên thanh. Nhãn
  `DemoBadge` của mỗi màn demo **là cửa đăng nhập** (bình thí nghiệm + chữ +
  biểu tượng đăng nhập `accent`, tên «Dữ liệu demo. Đăng nhập», tới
  `/login?tiep=<màn này>`); `cua={false}` cho nhãn gọi tên một phần của màn
  («AI nháp»).
- **`TopBar`** trên giấy: tiêu đề `title` **tối đa hai dòng**, cân giữa khi
  vừa (hai bên bằng nhau), không vừa thì hai bên giữ bề rộng riêng; phụ đề
  `caption inkSoft`; back chevron 48 hoặc wordmark `ink` 18 khi là đầu tab.
  Back đi theo lịch sử, không có lịch sử thì về tab của route (`luiVe`). Trên
  web, màn nhận focus điều hướng thì focus vào tiêu đề (`role="heading"`),
  trừ khi đang gõ hay có sheet. Màn demo dạng stack có **một** cửa demo ở bên
  phải (dưới 360dp chỉ còn hai biểu tượng). Ô icon app chỉ ở Welcome/Login.
- **Back đóng lớp trên cùng trước khi rời màn.** Sheet tự lo (xem `Sheet`);
  lớp mà màn **tự vẽ** (một bảng của Cá nhân, bước hai của xoá tài khoản,
  hồ sơ sống) đăng ký `useLuiLop(mo, dong)` (`ui/useLuiLop.ts`): trong lúc
  `mo`, Back cứng Android và Back trình duyệt (`lui-web.ts`, chạy trước
  router) đóng lớp đó, không rời màn (QA UI-108, UI-110). Người gọi:
  `Profile.tsx`, `profile/HoSoSong.tsx`, `cai-dat/XoaTaiKhoanScreen.tsx`.
- iOS: `BlurView` 78 theo scheme thay nền `card`, **chỉ ở thanh đáy**, rail
  giữ `card` (chỉ đọc từ mã, chưa có ảnh iOS).

### Trạng thái rỗng, tải, lỗi
- **`EmptyState`** năm loại (`first-use`, `no-results`, `filtered`,
  `permission`, `failure`): `h2` + một câu `body` `inkSoft` rộng tối đa 420,
  **một** hành động `RudiButton compact` (`outline` khi `failure`) và một cửa
  phụ `ghost`. **Cả hai nút `full={false}`, rộng theo nhãn trên mọi nền
  tảng** (03/10): `full` là `width: 100%`, web tính nó theo hàng nút đã co
  theo nội dung còn Yoga tính theo cột, nên trên Android nút duy nhất giãn
  mép tới mép trong khi web là viên gọn; vẫn cao ≥ 48 (`compact`), và hàng hai
  nút giờ theo đúng luật «Hai nút cùng một hàng» (F41). *Luật đo được*
  (`cong-dong-trang-rong.test.mjs` đòi đúng hai `RudiButton`, cả hai
  `full={false}`). Hàng hai nút trên Android, trước là hai nút giãn chồng
  nhau, nay một hàng: đã chụp `ErrorState` của `PlaceDetailLive` («Thử lại» +
  «Về Khám phá») ở 411×914 và 360×640, cỡ chữ 1.0 và 1.3 — một hàng, không
  cắt; Conversations «Tôi có lời mời», Khám phá «Xóa lọc» và hai `ErrorState`
  còn lại cùng component nhưng chưa chụp. Khe `illustration` từ 08/09 nhận
  `<Canh>` **rộng 168**. **Mọi `EmptyState` mang một cảnh của thế giới
  app** (`<Canh id=…>`, id trong `art/canh.ts`), trừ ô rỗng được đặt tên
  kèm lý do trong `tests/trang-rong-co-hinh.test.mjs` (từ 03/10 danh sách
  sống ở test, không ở đây): `KHONG_HINH` là im lặng có chủ ý (danh sách
  quản trị: «Bạn chưa chặn ai», «Chưa có phiên nào», «Chưa ẩn bài nào»,
  «Chưa có ghi chép nào»; tiền: «Chưa có sổ nào để quyết toán»; lời nhắn
  dưới một bài tường, nơi cảnh sẽ nặng hơn chính bài), `CHUA_VE` là **nợ
  chưa vẽ**, tách riêng để không ai đọc nợ thành quyết định. Mount mới không
  cảnh thì test đỏ tới khi có người nói vì sao.
  Album «Chưa có kèo nào» → `chua-co-keo`, «Chưa có khoảnh khắc» →
  `chua-co-anh`, Khám phá «Chưa thấy nơi phù hợp» → `tim-khong-ra`
  (`sua2-sang-1.0/bs-04-tim-khong-ra`: cảnh, `h2`, một câu, một nút).
  `chua-co-hoi` (Nhóm «Chưa có nhóm nào») và `chua-co-ban` (Bạn bè «Chưa có
  bạn nào», Cộng đồng «Chưa theo dõi ai») nay đều có màn gọi; cảnh duy nhất
  đã vẽ mà **chưa màn nào gọi** là `chua-co-tin-nhan` — chỉ bàn thử
  `app/dev/ui-lab` (grep `<Canh id=` trong `src/` và `app/`, 03/10). Câu
  thân rút về một câu vì cảnh đã nói vế đầu. Từ 11/09 câu thân đi qua
  `khongMoCoi` (`ui/chu.ts`): hai chữ cuối nối bằng NBSP nên không dòng nào
  kết bằng một chữ lẻ («…rồi thử / lại.» — tái audit 10/09 ảnh 20; câu dưới
  bốn chữ giữ nguyên); `r13-77-loi-canh-moi-*`.
- **Cộng đồng rỗng (Khám phá › Cộng đồng, 03/10)**: năm danh sách là
  `EmptyState` `inline` trên gutter 16 của ô tìm và hàng chữ-tab
  (`marginHorizontal: 16`, không đệm trên — vạch của hàng chữ-tab đã tách),
  không còn icon Ionicons trên `h1` — những ô rỗng cuối cùng ngoài
  `EmptyState`. «Dành cho bạn»/«Thịnh hành» → `chua-co-ky-niem` «Một ngày đáng
  kể» + «Viết bài»; «Đang theo dõi» → `chua-co-ban` «Chưa theo dõi ai» + «Xem
  bài thịnh hành» (danh sách theo dõi đầy lên bằng cách tìm người trong các
  bảng tin, viết bài của mình không làm nó đầy); «Đã lưu» → `chua-luu-bai`
  «Chưa lưu bài nào» + «Xem bài thịnh hành»; «Bài của tôi» → `chua-co-bai`
  «Chưa kể chuyện nào» + «Viết bài» (tên của khay Tạo); «Bài đã ẩn» **không
  cảnh, không nút**: danh sách quản trị (`KHONG_HINH`). **Một cỡ cảnh cho cả
  năm tab**, để tiêu đề không nhảy khi đổi tab: **168**, chỉ **120** khi cửa
  sổ thấp (`height < 700`) **và** chật (`width < 360` hoặc `fontScale >
  1.15`) — ở 320×640 cỡ 168 (và cả 144) đẩy nút xuống dưới thanh tab, còn
  375×667 dư chỗ và giữ 168; không bao giờ hạ cả một lớp máy chỉ vì chiều
  cao. Thẻ xin cá nhân hoá **chờ tới khi «Dành cho bạn» có bài**: trên bảng
  tin rỗng nó là lời mời coral thứ hai cạnh nút của cảnh, và cá nhân hoá một
  bảng tin rỗng không đổi gì. Tiêu đề riêng của danh sách chỉ hiện **trên
  danh sách có bài**: trên danh sách rỗng nó chồng cùng hạng với tiêu đề của
  ô rỗng, trên khung xương nó đổi chữ lúc danh sách về rỗng. Tìm trong Cộng
  đồng không thấy → `tim-khong-ra` 168 như Khám phá, và **cảnh lui khi bàn
  phím mở** (`useKeyboardOpen`: ô tìm chạy theo từng chữ gõ nên «không
  thấy» hiện khi phím còn lên), câu gợi ý ở ngay dưới ô. *Luật đo được*
  (`cong-dong-trang-rong.test.mjs`); ảnh `.impeccable/review/cdr2/`, `cdr3/`.
- **`Skeleton`**: xương màu `line`, bo 10, băng sáng `card` 0.55 chạy 1400ms;
  tắt hẳn dưới Reduce Motion. `SkeletonLines` dòng cuối 62%.
- **`ErrorState`**: cùng khung với `EmptyState kind="failure"`. Chỉ dùng khi
  **chưa có gì** để hiện; xem luật dưới.
- **`CauTaiCho`** (`ui/CauTaiCho.tsx`, «câu tại chỗ»): câu lỗi nói **ở đúng
  chỗ ngón tay vừa chạm**. Trước nó, ~60 màn tự viết câu `warn` và đặt nơi
  bố cục còn chỗ: y −2855 ở màn lô (UI-049), dưới mép đáy ở ô soạn Cộng đồng
  (UI-133), cuối form có nút dính đáy (UI-051). Hợp đồng:
  - **Vị trí là quyết định duy nhất của nơi gọi**: ngay dưới control vừa
    thất bại, hoặc trong `footer` dính đáy ngay trên nút. Còn lại cố định.
  - Hình: icon `alert-circle-outline` 18 + câu `body`, cả hai màu `warn`;
    câu dàn theo cột rộng tối thiểu 220 và xuống dòng; **một lối đi tiếp**
    tuỳ chọn (`hanhDong`: «Thử lại», «Nhập tay») là chữ `label ink` gạch
    chân, cao 48, đứng cạnh câu hoặc gập xuống dưới. Cỡ `nho` cho lỗi dưới
    một tin/hàng trong luồng dày: icon 15, câu `note`, hành động `caption`
    ngay sau câu (không đẩy ra mép), đích 48 trả lại chiều cao (lề âm 14) để
    dòng vẫn là một dòng.
  - Trợ năng: `accessibilityRole="alert"` + `aria-live="polite"`: tìm được
    theo vai, đọc một lần khi hiện, không ngắt câu đang đọc.
  - Hiện bằng mờ vào `standard`; dưới Reduce Motion hiện ngay. Trên web,
    nếu bố cục vẫn để câu ngoài cửa sổ, cuộn khung gần nhất **vừa đủ** để
    thấy nó (`scrollIntoView nearest`).
  - `cau = null` không vẽ gì, nên màn giữ một khe cố định; câu biến mất khi
    lần làm lại thành công.
  Dùng ở 26 file (03–04/10), gồm footer lưu của trang cuối sổ chuyến đi, lỗi
  dưới hàng trong luồng chat, thành viên, bình luận, ghi chép.
- **Luật Lỗi Không Thay Dữ Liệu.** Một lần tải lại thất bại **không bao giờ
  thay dữ liệu đã hiện**: câu `CauTaiCho` đứng cạnh dữ liệu cũ (QA UI-030,
  UI-077, UI-083). `ErrorState` toàn màn chỉ khi lần tải đầu đã hỏng và
  không có gì để giữ. Đây là nửa hợp đồng của nơi gọi, và là lý do
  `CauTaiCho` là một dòng chứ không phải một trạng thái.
- **Câu lỗi đọc `code` trước mã HTTP** (`src/cau-loi-theo-ma.ts` →
  `thongDiepNguoiDoc` trong `src/api.ts`). Thứ tự: câu tiếng Việt máy chủ
  đã viết (có dấu) đi thẳng; không nối được (status 0) là
  `LOI_KHONG_NOI_DUOC` «Không kết nối được Rủ Đi. Kiểm tra mạng rồi thử
  lại.»; rồi bảng riêng của màn (`IDEMPOTENCY_REFUSALS`, `SCAN_REFUSALS`,
  `ANH_REFUSALS`…); rồi `cauTheoMa(code)`: mã có câu riêng (`invite_not_found`
  gọi tên cả gõ sai, hết hạn, đã dùng; `membership_already_open`;
  `otp_*`…), rồi theo **hình** của mã (`*_not_found` → «Không tìm thấy
  <thứ này>. Có thể nó đã bị xoá, hoặc bạn không còn quyền xem.»,
  `*_wrong_state`/`*_already_*`, `*_conflict` → «Ai đó vừa sửa chỗ này trước
  bạn…», `*_expired`, `*_unavailable`, `*_too_large`), để mã máy chủ thêm
  ngày mai vẫn rơi vào câu đúng loại; **chỉ khi đó** mới đoán theo status.
  Mỗi câu nói hỏng gì, vì sao khi biết, làm gì tiếp; **không bao giờ in mã,
  status hay địa chỉ** (địa chỉ chỉ ra console dev).
  - **Mất mạng và máy chủ tạm ngưng là hai câu khác nhau**: không nối được →
    «Kiểm tra mạng»; một phần của Rủ Đi trả `*_unavailable` (503) → «Phần
    này của Rủ Đi đang tạm ngưng. Thử lại sau ít phút.», không bảo người ta
    đi kiểm tra mạng đang tốt (UI-029); 5xx khác → «Rủ Đi đang gặp sự cố…
    Chưa có gì bị ghi sai».
  - **Từ chối vĩnh viễn không mời «Thử lại»**: `laTuChoiVinhVien(status,
    code)` đúng với `*_not_found`, `*_wrong_state`, `*_already_*`,
    `*_expired`, `membership_required`, `direct_message_unavailable`, và
    status 403, 404, 410; màn chỉ đưa «Thử lại» khi nó sai (QA UI-100: hai
    «Thử lại» dưới «Bài này không dành cho bạn»). 401 nói «Phiên đăng nhập
    đã hết. Đăng nhập lại để tiếp tục.»; luồng chat (`chat/nhip-tin.ts`)
    cũng không đưa «Thử lại» cho nó.
  - 409 `idempotency_request_in_flight` nói thật là **chưa biết** đã ghi hay
    chưa và dặn **đừng bấm lại ngay**: bấm lại ở đây là cách một khoản trả
    thành hai.
- **Bộ lọc vắng vì không có dữ liệu** được nói thẳng bằng một câu `body
  inkSoft` («Không có dữ liệu tháng khác nên không hiện bộ lọc kỳ»), không
  vẽ control chết.

### Signature: Bìa mở ra trang (Welcome → Login)
Welcome tại `34d57ca9` là bìa indigo tràn màn với `Grain vaiBia` 0.30,
`Logo compact` ở đầu, bốn chương dẫn từ lời rủ tới chọn nơi, rõ phần mỗi
người và giữ kỷ niệm. Tiêu đề ExtraBold 34/40 (30/36 dưới 360dp), body
17/24 `coverInkSoft`. Cảnh `scene-street` ở trang 1/3, `scene-cafe` ở trang
2 và `scene-coast` ở trang 4; chúng là illustration địa phương, không mang
thông tin thay chữ. Chỉ báo gồm bốn chấm 6dp, chấm đang chọn dài 22dp màu
coral, mỗi vùng chạm 48×48dp; dưới là «Trang n trên 4», con dấu «Rủ Đi
thôi!» rộng hết cột và link đầy đủ. Link tới chương sau, trang cuối trở lại
đầu; pager giữ cùng chương khi bề rộng đổi.

Bấm con dấu khoá lần mở cho tới khi màn nhận focus lại và push `/login`
trực tiếp; source chuẩn bị Login bằng `prefetch`, không còn pha nhấc bìa
và chờ trước điều hướng. Artwork xuất hiện bằng opacity và translateY 12
với `motion.timing("shared", "decelerate")`; pager không animate khi giảm
chuyển động. Đây là mô tả cơ chế từ mã, chưa phải chứng nhận motion của
source `34d57ca9`.

### Bàn giao Welcome 08/10/2026 — phạm vi đã ship

Ghi từ source `34d57ca9dbcf032bf9c8b21b41e6b059a6f4e00c`:
`screens/Welcome.tsx`, `ui/CoverButton.tsx`, `theme.ts`, `ui/Grain.tsx` và
`ui/art/Nep.tsx`. Concept `board02-panel1-welcome-cover.png` là reference,
**chưa phải approved comp**; giữ Nếp canonical, không thay bằng mascot của
concept. Bố cục xếp dọc ở 320dp/font lớn là adaptation của bìa, giấy, cảnh,
nhân vật và hành động. Mã bố cục cao còn có cảnh cạnh cột chữ và
`spot-memory`; verdict dưới đây không xác nhận bố cục rộng đó.

| Phần bố cục ngắn | Artifact đã ship |
|---|---|
| Cảnh | Cao 144dp, rộng vùng nội dung, `fit="cover"`; giữ focal của cảnh trên màn ngắn |
| Nếp | `pose="moi"`, 96dp, trái 12dp/đáy 4dp; giữ mặt, tay và chân canonical |
| Mép giấy/vải | Giấy rộng 136dp, bo dưới phải 72dp; vải rộng 124dp, bo dưới phải 104dp; mép giấy khoảng 12dp ở đầu mở dần về chân Nếp |
| Vật liệu | Giấy theo scheme, gradient ngang `paperShade → paper → paperShade` tại 0/0.55/1, `Grain giayTrang` 0.45, `bongGiay(1, dark)`; vải `cover` + `Grain vaiBia` 0.30 |
| Hành động | CTA `tilt={0}`, link `full`; nhãn phụ đủ chữ, nội dung và action đi trong vùng cuộn khi font lớn |

[Verdict độc lập Welcome](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-peer-r1/welcome-independent-verdict-34d57ca9.md)
chốt **15/16, `ship` chỉ cho ba finding nhãn phụ, artwork và mép giấy:
cả ba resolved**. Reviewer đã mở 17 ảnh Android portrait 320×640: thường
3, font 1.3 có 4, font 2.0 có 4, giảm chuyển động 3, tối 3. Ảnh của source
này phủ trang 1/2/4, không tự nhận đã mở trang 3. Năm functional case exit 0
(22,745s / 21,493s / 29,903s / 20,941s / 22,595s) không thay verdict thị
giác hoặc full gate. Nền APK là `b87a16f1`, backend `54b5dc0e`, không gán
chúng thành build `34d57ca9`; diễn biến ghi tại
[RECOVERY-20261008.md](docs/design/nep-ui-implementation/RECOVERY-20261008.md).

Clip motion chỉ thuộc `f26c3dd7`, không chứng minh motion `34d57ca9`.
Plugin/detector và craft-floor hiện thiếu, không có kết luận detector clean.
Phạm vi này không phải APPROVE toàn app, không phủ iOS, tablet/cửa sổ rộng,
TalkBack hoặc toàn bộ trạng thái Welcome.

### Welcome CTA → Login — bằng chứng bổ sung normal/warm 3b

[Review motion 3b](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-peer-r1/welcome-independent-motion-3b71838e.md)
trả **`ship` riêng đoạn CTA→Login normal/warm đã quay**, đóng finding thiếu
pha giữa của clip04. Source
`3b71838e057c2daef6500f04a91b44edf7d20b4e`; diff34→3b rỗng cho Welcome,
CoverButton và `_layout.tsx`. Không đổi provenance clipf26 hoặc verdict34
ở mục trên, không cộng điểm interaction/motion hay nâng tổng **15/16**.

Reviewer mở đủ **29 frame gốc** và ảnh before/after trên Android emulator
5560, 320×640, font 1.0: Welcome rút sang trái, Login trượt vào từ phải với
header, lời chào, input và action đã vẽ; không thấy frame trắng/trống, Nếp
và fold đi cùng bìa. Đây là native slide giữa hai trang của sổ, không phải
pagecurl/3D hoặc signature nhấc bìa lịch sử. Clip encoded **2,436411s**, tốc
độ biến thiên trung bình khoảng **11,9fps**; không suy timing animation hoặc
60fps từ duration/frame count. Warm flow exit 0/**16,086s**, một tap CTA thật.

Phạm vi chưa phủ reduced lifecycle, Back/cancel, double tap, background/resume,
cold transfer, iOS hoặc timing/FPS chính xác. Đoạn này không thay full motion
gate hay APPROVE toàn app; detector vẫn thiếu.

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

**Luật Reduce Motion Tới Tận Navigator** (11/09, `1cfe0ee4`). Dưới Reduce
Motion chuyển cảnh là **cắt thẳng** (Android «Remove animations» cho phép
crossfade hoặc cắt; hệ này chọn cắt, không crossfade). Cơ chế: mọi
`animation:` của `Stack`/`Stack.Screen` trong `app/_layout.tsx` đi qua
`stackAnimation(wanted, motion.reduced)` (`src/rudi/motion.ts`, thuần, pin ở
`tests/motion.test.mjs`) — trả `"none"` khi giảm, giữ `slide_from_right` /
`slide_from_bottom` / `fade` khi không; **một literal `animation: "…"` trên
Stack là vi phạm**. Mọi config Reanimated (`timing`, `spring.press`,
`spring.settle`, và `reanimated` cho layout animation kiểu
`FadeIn.reduceMotion(motion.reanimated)`) lấy `ReduceMotion.Always`/`Never`
từ bit `reduced` sống của `useMotion`, **không bao giờ `ReduceMotion.System`**.
Vì sao: react-native-screens trên Android chuyển Fragment bằng
`android.view.animation`, ba `*_animation_scale` = 0 không chạm tới và thư
viện không có mã đọc Reduce Motion, nên app phải tự xin cắt; cờ `System` của
Reanimated chỉ đọc một lần lúc khởi động nên đổi setting giữa phiên bị bỏ qua
(tái audit 10/09 R1: chi tiết quán vẫn trượt, sheet «Tạo mới» còn 7 khung ở
scale 0). Chứng minh bằng **số khung, không bằng `rc=0`**: quay `screenrecord`
quanh một flow, tách 30 fps, đếm chuỗi khung đổi liên tiếp, có đối chứng hai
chiều **trong cùng một phiên app** (scale 1 trượt 7–9 khung → scale 0 cắt 1
khung → scale 1 trượt lại) cho push stack, sheet `transparentModal` và
`presentation: modal` — `docs/archive/claude/2026-09-11/motion-v2/README.md`; phương
pháp đo v2 (warm-up ngoài cửa sổ, kiểm pid, fail-closed, trap trả scale, p99 là
nhãn bucket) ở `docs/archive/claude/2026-09-10/motion/README.md`.

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
«Bản trải nghiệm» (Welcome ở mốc cũ, không có tại `34d57ca9`). Số tiền viết
«1.106.250đ»; không có số nào màn
tự bịa: đếm ảnh, huy hiệu, ngày còn lại đều tính từ dữ liệu.

**Luật Một Cách Viết Ngày** (01–04/10, QA UI-103, UI-104): ngày và giờ đi qua
`src/rudi/ngay-viet.ts`, **viết tay, không qua `Intl`/`toLocaleDateString`**
(locale vi-VN nối ngày tháng bằng gạch ngang, và Hermes với trình duyệt gọi
tên tháng khác nhau). Ngày là ngày địa phương của máy, ngày và tháng luôn hai chữ số:
- `ngayVN` → «28/09/2026» (cài đặt, phiên);
- `gioNgayVN` → «10:07 · 28/09», năm chỉ khi không phải năm nay (tường, ảnh in);
- `khoangNgayChuyen` → ngày của một chuyến từ ngày lịch máy chủ giữ
  (`YYYY-MM-DD`): «28 - 29/09», «30/09 - 02/10», «28/09» khi một ngày, năm chỉ
  khi khác năm nay; trước đó kệ album chỉ in năm nên hai chuyến cùng năm
  không phân biệt được;
- `thangNamVN` → tiêu đề tháng «Tháng 9, 2026» (kệ sổ).
Chuỗi không đọc được thì trả rỗng. Tờ giấy của sổ đôi có bộ chữ riêng cùng
ngữ pháp (`to-giay/ngay.ts`: «Thứ Bảy 19/09», cũng không `toLocaleDateString`).

**Luật Một Thứ Một Chữ** (04/10, critique B11): một thứ có đúng một tên,
động từ và số đếm cùng họ, không bao giờ hai họ trên một thẻ.
- Khoảnh khắc trên tường nhóm được **tim**: «Thả tim» / «Đã thả tim», «N tim».
- Bài viết (trang viết, Cộng đồng) được **thích**: «Thích», «N thích».
- Đếm trên hộ chiếu Cá nhân là **khoảnh khắc** (`counts.memories`), không
  «kỷ niệm»: ngay dưới nó là kệ «Những ngày muốn giữ», một thứ khác.
- Ngân sách của kèo luôn kèm «dự kiến»: nó nằm ngay trên «Chia bill buổi
  này» và không phải tiền đã chi.
- Thuật ngữ của sổ (`nghĩa vụ`, `obligation`) không lên màn; «đợt thu» và
  «phát» là tên thao tác có thật (ADR-0037 D14) nên được dùng.

**Luật Nói Một Lần** (08/09, báo cáo 07/09 §8.5, §4.6): mỗi trạng thái và
mỗi sự thật có **một** chỗ trên màn.
- `HangChang phu` mang **tên địa điểm** khi có, «Chọn địa điểm» khi chưa có
  (live: «Mở địa điểm» khi máy chủ chỉ có id), không «Đã gắn địa điểm · bấm
  để mở».
- Trạng thái nháp nói **một lần** ở đầu màn (badge «Nháp», `AiNote`), không
  «Có thể thay đổi» dưới từng hàng. Tờ AI trong chat (11/09): badge «AI nháp»
  ở chân là nơi duy nhất; thân tờ không còn «Nhóm sửa được trước khi chốt».
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

**Luật Một Dấu Cho Một Địa Điểm** (11/09, tái audit Codex 10/09 R3 —
`docs/archive/codex/2026-09-10/reaudit-evidence/`): một địa điểm mang **lý do hoặc
con dấu, không cả hai**; **tiêu đề mục** («Chỗ hay ở <thành phố>» từ
02/10, tới 01/10 «Gần bạn, đúng gu») là nơi **duy nhất** nói lời hứa, nên lý do không mở bằng «Hợp gu…» và con dấu «HỢP GU»
không đứng cạnh một dòng lý do; **mô tả không nhắc lại từ của lý do** — lý do
phải thêm một sự thật mà dòng dưới nó chưa nói.
- Cơ chế trong mã: `dauCon(dd)` (`HangDiaDiem.tsx`) trả `null` khi có `lyDo`
  ở cả bốn khung; fixture chọn lý do bằng `chonLyDo(tags, sub)`
  (`kham-pha/ly-do.ts`: bỏ dấu, thường hoá, loại tag mà mô tả đã chứa; hết
  tag chưa nói thì lấy tag đầu) và `fixtures.ts` viết mô tả bằng một sự thật
  các tag không có; live in `reason` của mô hình nguyên câu. `AiNote` ở chi
  tiết nói về **nhóm này** («Nhóm 8 người ngồi được một bàn…»), không đọc lại
  chip thành tính từ.
- Cổng: `node --test tests/kham-pha-ly-do.test.mjs` (luật chọn) và
  `node scripts/do/kiem-lap-loi.mjs <hierarchy.xml>`
  trên dump uiautomator của Khám phá fixture: đỏ khi còn node «hợp gu», khi
  tiêu đề mục không đúng một node, khi dẫn không có dòng lý do là một tag,
  hoặc mô tả lặp từ của lý do (`r14-80-kham-pha-*.kiem.txt`).

### Trợ năng (sàn)
Đích bấm 48dp (nút 52/60, `compact` 48, chip 48, tab 48, back 48, link bìa
48, tay nắm kéo 48×56, checkbox 48); chữ nhỏ nhất 13sp caption, trừ ba cỡ
riêng có kiểm ở font 1.3 (tem 12, nhãn tab 12, demo 10); mọi cặp chữ/nền
trong bảng đo dưới; viền control ≥ 3:1; con dấu có `accessibilityLabel`,
`Stamp` không role; `role="tab"` cho tab và segmented; Reduce Motion tôn
trọng; chụp lại ở font 1.3 trước khi nói «không cắt». Thêm từ 01–04/10: ô
nhập `ONhapMuc` 48, ngoại lệ duy nhất có tên là lưới ngày 44 của
`ChonNgayLich`; câu lỗi tại chỗ là `alert` + `aria-live="polite"`
(`CauTaiCho`), lỗi của ô nhập đọc lịch sự; nút tắt nói lý do qua
`accessibilityHint` (`lyDo`); hành động lặp theo hàng có tên trợ năng
riêng mang tên của hàng; tay cầm sheet ẩn khỏi trình đọc, lối ra là
«Đóng bảng»; Back (cứng, trình duyệt, Esc) đóng lớp trên cùng trước khi rời
màn.

### Chín bề mặt native — candidate 797, ghi 10/10/2026

**Phạm vi và nguồn.** Đây là hồ sơ build sau lượt sửa review `d9c06bf4`, chưa
có verdict ship sau sửa. Đọc `ui.tsx`, `screens/Profile.tsx`,
`screens/profile/HoSoSong.tsx`, `screens/explore/HangDiaDiem.tsx`,
`screens/keo/PlanLive.tsx`, `screens/keo/CreateOutingLive.tsx`,
`ui/ThanhNganSach.tsx`, `screens/groups/Conversations.tsx`,
`community/Composer.tsx`, `community/MediaPicker.tsx` và `screens/Create.tsx`
(đường dẫn tương đối dưới `apps/mobile/src/rudi/`). Route viết bài hiện dùng
`community/Composer.tsx`, không có `screens/community/New.tsx`; hàng địa điểm
ở `screens/explore/`, MediaPicker ở `community/`.

| Bề mặt | Điều có trong source 797 và mẫu native đã mở |
|---|---|
| Sửa hồ sơ | Form riêng cùng component mounted giữ bản sửa; header «Chỉnh hồ sơ»/«Huỷ» và footer «Lưu hồ sơ» cố định, cụm avatar 64/đổi ảnh + lời dẫn gọn, gap form 20. Tên tối đa 200, giới thiệu 500 có counter, thành phố 120. Back giữ bản nháp trong phiên mounted; Huỷ bỏ bản nháp; không khẳng định lưu qua process death. Source giữ lỗi và khóa gửi khi đang lưu. |
| Khám phá | Phone đi một trục lead rồi hàng để so; source lead dùng ảnh 16:9 compact/21:9 rộng, tranh cảnh cao 160dp ở phone, tên `h2`, phụ đề wrap, facts rồi hai chip lý do/giá như source. Tranh cảnh vẽ thay chỗ ảnh thật **không mang nhãn chữ** (chủ sản phẩm 20/10/2026, bỏ «Minh hoạ»): nó trông là tranh vẽ, không giả ảnh địa điểm, và ảnh thật luôn đi với dòng ghi công; không nhập người, số liệu hay online status từ reference. Đây cập nhật dạng source, không khôi phục lead ngang 112 của mốc 3bb. |
| Hàng địa điểm | Ở chữ thường, thumbnail 88×88 và cột chữ top-aligned, gap 14; đệm dọc 16, hairline dưới. Thứ tự: tên/mô tả/facts → giá kèm đơn vị dùng toàn bề ngang → lý do AI đọc hết, không `numberOfLines` trên lời giải thích hàng. Từ `chuLon`, ảnh rộng toàn cột/cao 132dp, chữ xuống dưới, tim ở góc phải ảnh; tranh cảnh dự phòng không nhãn chữ (20/10/2026). Mẫu supplement 200 đã mở thấy đủ giá 200.000–250.000đ mỗi người và câu so ngân sách 175k, rồi hàng tiếp; không coi một viewport cuộn là toàn danh sách. |
| Lên plan | Plus nhỏ `IconButton quiet` ở header cho cả nhánh có nhóm và `contextId===null`. Empty state có nhóm vẫn có CTA «Tạo kèo»; chưa nhóm dẫn «Tới Tin nhắn». Chữ lớn chuyển vé/cuống sang luồng dọc; ngày và tháng cùng hàng, tên/facts được wrap. Ảnh plan-header 200 có ngày 11/TH10 và dữ kiện vé đủ. Hai ảnh visible-final light100 của thiết bị 5562 đã mở thấy empty chưa nhóm và khay Tạo sau chạm plus header; bằng chứng ADB có phạm vi ghi riêng bên dưới. |
| Khay Tạo | `BillShareIllustration` là SVG native 76×72: giấy biên lai mép răng cưa, tiêu đề coral, dòng món/tổng và đường chia ra hai phần. Đọc token light/dark, không raster mới; Nếp canonical và các vật lịch/ảnh/thư giữ world. Ở chữ lớn, các lựa chọn chuyển một cột ngang hình/chữ thay vì hai ô hẹp. Đây cấu trúc có trong source, ảnh đã mở chỉ trực tiếp chứng minh khay light100. |
| Ngân sách | `ThanhNganSach` cùng trường nhập chính xác chia sẻ một giá trị nguyên đồng. Thanh 0–2 triệu, bước 25.000đ, target cao 56dp, thumb 28dp, mức nhanh 200 nghìn/500 nghìn/1 triệu minHeight 48. Exact entry dùng parser tiền chung; vượt thang vẫn giữ số, hiện ngay câu «Ngoài thang kéo 0–2 triệu. Giữ nguyên số bạn nhập.» và thumb đổi chấm thành vạch. Supplement200 đã mở giữ 3.250.123đ với câu giải thích; endpoint không được hiểu thành giá trị đã clamp. Role adjustable/actions có trong source; chưa nghiệm thu TalkBack hoặc haptic bằng ảnh. |
| Tin nhắn | Heading + plus nhỏ tạo nhóm, search «Tìm người hoặc nhóm» và xoá search; danh sách lọc tên người/nhóm từ dữ liệu đang đọc. Giữ story rail, lời mời và đường E2EE. Ảnh dark100 đã mở chứng minh hierarchy title/search/hội thoại; không chứng minh trọn search hoặc crypto từ ảnh. |
| Back/header | Cùng primitive TopBar và quy tắc nhường ngang cho title được ghi ở Layout; không sửa bằng cách thu font hay target. Có thể wrap khi chữ lớn. Không biến offset của chevron thành offset cho mọi icon. |
| Viết bài | Header gọn, avatar 44 + tên/người đọc ở đầu; canvas viết là vùng chính, giới hạn 5000 có counter; minHeight theo chiều cao cửa sổ trong 112–240dp, maxHeight 420 và lineHeight 26. Công cụ ảnh/video là ghost compact; chủ đề/tag mở tại chỗ. Footer giữ lời duyệt công khai, lỗi và CTA trên IME. Ảnh dark100-writing và light200-empty đã mở có canvas/counter/footer tương ứng; công cụ dưới fold cuộn tới được theo cấu trúc, ảnh tĩnh không tự chứng minh thao tác đó. |

**Bằng chứng chức năng khác bằng chứng thị giác.** Bó evidence ngoài repo:
[/native-audit-797a3f52/](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-797a3f52/).
`after-matrix-results.json` ghi 24/24 flow exit0, tổng 714.665s: social,
places, budget, budget-exact × light/dark × font 1.0/1.3/2.0, source và harness
đều 797. `after-matrix-restore.json` ghi font đã phục hồi, viewport 320×640.
Lượt documenter chỉ đọc kết quả có sẵn và mở mẫu ảnh, không chạy lại matrix.

`profile-functional-result-r2.json` và `supp-profile-draft-save-r2.result.json`
ghi Back giữ nháp, Huỷ bỏ nháp, native save quay lại hồ sơ và server giữ
`display_name`, `bio`, `city`, `avatar_url`; flow r2 exit 0/53.785s. Đây là
bằng chứng chức năng riêng của hồ sơ; không đưa giá trị fixture vào hệ thiết
kế, không gọi các lỗi assertion ở lượt harness trước là chính sách bố cục.
Supplement chữ 200 `supp-large-rows-tabs-budget.result.json` exit 0/43.672s.
Ảnh `supp-large-community-selected` chỉ cho thấy tab Cộng đồng đã chọn và
skeleton đang tải; không chứng minh feed đã tải, empty hoặc error đã đạt.

**Nhận diện runtime.** `runtime-provenance.json` ghi JS 797 từ tree sạch,
APK vẫn ở checkpoint `2f20a30179d0e21a1f4b3244dfbd685b5bade7e9`, SHA-256
`963bd5daa0be4348606158dd3c06848e7ef604e06ebbdf035136a718cae0d514`.
API chạy source `9a6d69563a503860774f291f22b4806082b970a6`;
`services/core` không đổi từ 9a đến 797. Không gọi APK/API là bản rebuild 797;
đây là thay đổi JS. Documenter không mở web, điều khiển UI hoặc khởi động server.

**Mẫu đã mở.** `captures/**/takeScreenshot/`: light100 profile-edit,
profile-keyboard, explore-lead, explore-rows, create-tray; dark100
composer-writing/inbox; light200 composer-empty/plan-header; dark200
explore-rows; supplement row-price-reason, next-row, community-selected,
exact-outside-scale. Bổ sung đã mở hai ảnh ngoài thư mục captures:
`no-group-plan-light100-visible-final.png` và
`no-group-create-from-header-plus-visible-final.png`; tổng cộng 16 ảnh native
được lượt documenter mở. Các ảnh có tên after là viewport/state của flow tương
ứng, không phải chứng nhận cả route. Reference cấu trúc cũng đã mở riêng,
không dùng nó làm ảnh after.

**Bổ sung Plan chưa nhóm, chỉ light100.** Ảnh visible-final đầu có title,
plus nhỏ, bản đồ, empty «Gom hội mình trước» và CTA «Tới Tin nhắn»; ảnh sau
có khay Tạo thật với lời dẫn chọn hội ở bước tiếp theo. Phiên tổng hợp không
có nhóm đã đăng nhập bằng UI native và giữ session; kiểm HTTP login xác nhận
không có nhóm. `no-group-native-adb-visible-final.result.json` ghi chạm
«Tạo mới» theo bounds XML thực `[256,32][304,80]` và mở sheet.
`no-group-visible-window-receipt.json` ghi thiết bị 5562 có cửa sổ thật,
pid 888644, chạy SDK launcher. Đây là ADB XML/input/screencap, không phải
Maestro pass, không cộng vào matrix 24/24. Các lỗi transport Maestro, thiếu
reverse mapping và capture headless trước vẫn ngoài nghiệm thu thị giác;
chỉ hai ảnh visible-final này là bằng chứng bổ sung được ghi ở đây. Thiết
bị chính 5558 và peer 5560 không bị thay đổi bởi lượt bổ sung này.

**Cổng kỹ thuật có giới hạn.** `clean-checks.json` ghi đúng SHA 797,
clean_before/after đều true: typecheck, guide, build-check, compile, fixup,
Go guide và repo-tree exit0; Node subset 214 file, 1608/1608 pass, 0 skip.
Năm driver browser bị loại có tên trong record: `che-chu-lo-to-cha`,
`che-chu`, `rudi-nep-web`, `rudi-opening-web`, `rudi-welcome-web`.
Đây không phải full strict 32, full gate hay merge main.
`final-tsx-detector.json`/log ghi quét chín file TSX ở source 797,
exit 0/`[]`/0.040s và byte khớp commit 797. Detector này chỉ là regex TSX,
không phải phép đo geometry/contrast native; lượt documenter chỉ đọc kết quả. Motion, TalkBack, tablet,
iOS, state tải/lỗi/offline/retry toàn bộ và verdict thị giác mới vẫn chưa
được hồ sơ này chứng minh; không có điểm 14/16, total-app acceptance hoặc
quyền merge vào main từ các số trên.

### Khám phá, trang quán và Cá nhân — tinh chỉnh `2d0ced17..670a2a28`, ghi 21/10/2026

**Phạm vi và nguồn.** Mở rộng thế giới Sân Khấu Giấy v3, không thêm màu, font,
radius hay tranh mới. Đọc từ mã đã ship tại `670a2a28` (đường dẫn dưới
`apps/mobile/src/rudi/`): `screens/explore/PlaceDetailLive.tsx`,
`kham-pha/chi-tiet-gon.ts`, `kham-pha/canh-ve.ts`,
`screens/explore/HangDiaDiem.tsx`, `screens/explore/ExploreLive.tsx`,
`screens/explore/DiemDenScreen.tsx`, `screens/Profile.tsx`,
`screens/profile/HoSoSong.tsx`, `screens/profile/DiaryLead.tsx`,
`screens/groups/Conversations.tsx`, `ui/EmptyState.tsx`, `ui/ghi-cong.ts`.
Ảnh đã mở: `.impeccable/review/02c94551/place-detail-390x3600.png`,
`profile-empty-390x844.png`, `.impeccable/review/52b4c7d6/destinations-390x2400.png`
(dữ liệu seed vứt đi; ảnh đồi gradient phẳng là placeholder, không phải
chuẩn ảnh). Hợp đồng hướng: mục «20/10» trong ba surface brief Khám phá,
trang quán và Cá nhân. Chỗ build lệch hợp đồng: brief Cá nhân kể «dấu chân →
tab → sổ rỗng», build ship **tab trước dấu chân** (thẻ hồ sơ → hai tab →
dấu chân → sổ); bản ghi theo build.

- **Trang quán: một bìa lật trang** (`BiaTrang`). Một khung **16:9** rộng hết
  cột, bo `radius.base`, `ScrollView horizontal pagingEnabled`; mỗi trang là
  một `MediaSlot` với **đúng một dòng ghi công của chính ảnh ấy**, nên ảnh và
  ghi công đi cùng nhau và màn chỉ có một ghi công. Nhiều hơn một ảnh thì
  viên «n/N» góc trên phải (nền `card`, viền kẻ tóc `line`, bo `radius.small`,
  `caption ink`, trình đọc nghe «Ảnh n trên N»). Chạm trang mở `PhotoViewer`.
  **Không còn dải ảnh nhỏ dưới bìa**; dải ngang duy nhất còn lại là «Ảnh của
  nhóm» ở cuối trang (ô 280, ảnh nhóm, không phải bìa). Không ảnh thì
  `PlaceArtwork fillFrame` trong cùng khung 16:9, nên tên không dời chỗ.
  Ghi công ảnh quanh quán mở bằng «Ảnh quanh đây: » (`TIEN_TO_ANH`).
- **Trang quán: thứ tự đọc.** Tên `h1 ink` → phụ đề `body inkSoft` → hàng
  điểm (sao `accent` 15 + điểm **một chữ số thập phân**) và chip trạng thái
  «Đang mở»/«Đã đóng» → **«Tại sao đáng đến»** (`SectionHeader`, `body`, ba
  dòng, «Đọc thêm» chỉ khi ba dòng thật sự cắt) → **khối dữ kiện**
  (`dongChinh`) → hàng chip không khí → khối địa chỉ → các mục.
- **Khối dữ kiện (`dongChinh`).** Hàng nhãn/giá trị: nhãn `label inkSoft`
  cột rộng 96, giá trị `body ink` (dòng phụ `note inkSoft` khi có), gap 12,
  căn baseline; không thẻ, không icon. Thứ tự cố định, mỗi dòng chỉ khi máy
  chủ gửi: **Giá** («Ước khoảng 35–90k/người» khi máy chủ gọi là ước, «Khoảng …»
  khi không), **Hôm nay** (giờ của thứ hôm nay, «qua đêm» khi có), **Giờ đẹp
  nhất**, **Thời gian** («Khoảng 60–120 phút»), **Đặt chỗ**. **Giá nói một
  lần**: không còn mục «Giá» riêng, không còn giá trong hàng chip.
- **Chip trang quán chỉ nói loại nơi** (`nhanKhiChat`): danh mục, khí chất,
  trong nhà/ngoài trời, traits; **khử trùng không phân biệt hoa thường, dấu,
  «đ» và khoảng trắng** (`khoaNhan`), giữ cách viết đầu, tối đa 10. Không
  chip nào lặp một dữ kiện của khối trên.
- **Các mục**, mỗi mục `SectionHeader` + thân, **mục rỗng ẩn hẳn** (không
  «Chưa có…»): **Giới thiệu** (chỉ khi mô tả khác câu «Tại sao đáng đến») ·
  **Món nên thử** (thẻ món: nền `card`, viền kẻ tóc `line`, bo `radius.small`,
  tên `label ink` + dòng `note inkSoft`; chỉ có tên món thì chip) · **Không
  khí** (`body`) · **Hợp với ai** · **Giờ** (bảng tuần: thứ `label inkSoft`
  cột 84, giờ `body ink`, ghi chú `note inkSoft`) · **Trải nghiệm nên thử** ·
  **Lưu ý**. Danh sách dùng chấm `ellipse` 6 `inkFaint` + `body ink`, gap 10.
- **Địa chỉ rộng hết cột, «Chỉ đường» bên dưới.** Khối nền `card`, viền 1
  `line`, bo `radius.small`, đệm 14/12: ghim `location-outline` 20 `accent`
  + địa chỉ `label ink` + câu đường đi `caption inkSoft` (cả hàng bấm được,
  cao ≥ 48); dưới là `RudiButton outline compact` «Chỉ đường» thụt 30 cho
  thẳng cột chữ. Địa chỉ không bao giờ bị nút chia bề ngang.
- **Tranh cảnh dự phòng (`PlaceArtwork` + `kham-pha/canh-ve.ts`)** — **không
  bao giờ có nhãn chữ** (chủ sản phẩm 20/10/2026, «Minh hoạ» đã xoá). Vì
  không nhãn, tranh được đọc là chính nơi ấy, nên **mỗi cảnh mang vùng**:
  `scene-hills`/`scene-cafe` = Lâm Đồng, `scene-street`/`scene-lanterns` = Hội
  An, `scene-coast` = biển, `scene-atelier` = nội thất, đúng ở mọi nơi. Chọn
  **theo loại trước** (`cafe` → cafe/atelier, `quan-an-local` → street/atelier,
  `di-choi-dem` → lanterns/street, `vui-choi` → atelier, khác → hills/coast)
  **rồi lọc theo vùng** của điểm đến hoặc chữ trong địa chỉ. **Hai thẻ liền
  nhau không mang cùng một tranh** (`phanCanhDanhSach`: chỉ thẻ đang vẽ tranh
  mới tính là hàng xóm); khi tranh trung thực duy nhất trùng hàng xóm, thẻ vẽ
  **đồ vật của loại nơi** (`GuGlyph tone="ink"`) thay vào. Bản chỉ-đồ-vật nằm
  trên **`paperShade`**, không trên `paper` trắng: giấy trắng trần cạnh thẻ có
  tranh đọc thành ảnh hỏng. Cảnh mốc không bao giờ vẽ cho vùng khác.
- **Khám phá: danh sách phân trang.** `TRANG_DIA_DIEM` = 30 nơi một trang;
  cuộn tới trong một màn từ cuối thì đọc trang sau (`RudiScreen onGanCuoi`),
  nút «Xem thêm N nơi» (`cauXemThem`, không hứa nhiều hơn số còn) là lối dự
  phòng, «Đang tải thêm…» khi đang đọc. Stale-while-revalidate theo người +
  điểm đến: đổi lại tỉnh đã đọc thì danh sách cũ hiện ngay trong lúc làm mới;
  skeleton chỉ khi tỉnh chưa đọc bao giờ. Lý do AI tới sau và thay câu tại
  chỗ, không xô bố cục.
- **Sân khấu thành phố dùng ảnh bìa thật của điểm đến** khi có: `SanThanhPho`
  vẽ `MediaSlot` (bo 8) với ghi công, tranh vẽ thành phố là fallback.
- **Chọn điểm đến (`DiemDenScreen`): danh sách một cột, không lưới.** Thẻ đầu
  là **điểm đang xem**: viền `accent`, ảnh bìa thật 3:2 qua `MediaSlot` với
  ghi công (không ảnh thì tranh vùng `canhDiemDen` hoặc ký hoạ đường nét), tên,
  phụ đề, và con dấu `DauLon co="nho"` «ĐANG XEM» mực `ink` nghiêng -8. Mọi
  hàng sau là thẻ nền `card` viền kẻ tóc `line` bo 8 đệm 10 gap 12, cao tối
  thiểu 84, **cùng một ký hoạ đường nét thành phố trong khung 96×64** (bo 4,
  viền kẻ tóc, nền `paper`) — hàng không in ảnh vì thumbnail không chở trọn
  ghi công (ADR-0017 §2.5); chevron `inkFaint` hoặc `checkmark-circle`
  `accent`. **Tên và phụ đề không bao giờ bị cắt**: không `numberOfLines`,
  hàng **giữ chỗ hai dòng phụ đề** (`minHeight` = 2 × lineHeight) để 33 hàng
  cùng một chiều cao, dài hơn thì gập tiếp (320 dp, chữ lớn). Một dòng lặng
  mỗi hàng: phụ đề, hoặc blurb khi không có phụ đề.
- **Cá nhân: thẻ hồ sơ một khung.** Chỉ viền của chính thẻ (`radius.base`,
  đệm `space.md`); dải coral trên, kẻ đứt bên trong và dấu triện chìm đã bỏ
  (ba khung quanh một cái tên). Avatar 56 (48 ở chữ lớn, co lại chứ không xếp
  chồng) – tên `h2` + «thành phố · Tham gia năm» `note inkSoft` – nút sửa
  cùng một hàng, căn giữa dọc, gap 14. Tiêu đề trang «Cá nhân» là `h1`, như
  Tin nhắn.
- **Cá nhân: dấu chân một tầng chứa** (`ProfileFacts`). Một thẻ viền 1, hai
  hàng lưới **ba cột bằng nhau** cách bởi kẻ tóc `line`; mỗi ô **số trên
  (`h2`, tabular) danh từ dưới (`note inkSoft`)**, cao tối thiểu 56, nên không
  danh từ nào chung dòng với số và mọi ô một hàng cao bằng nhau. Hàng 1: bạn
  bè (danh từ `accent`, là cửa duy nhất, nhãn lời mời đang chờ `caption accent`
  trên `accentSoft`), nhóm, kèo; hàng 2: điểm check-in, khoảnh khắc, ô thứ ba
  để trống giữ lưới. **Số 0 in mực `inkSoft`**, số khác `ink`: thẻ toàn 0 là
  một sự thật, không phải tiêu đề. Không ô tô màu, không icon dấu chân.
- **Cá nhân: thứ tự theo dữ liệu.** Có trang đã giữ thật thì trang ấy dẫn
  (hướng 13/10); chưa có thì dấu chân dẫn và sổ rỗng **gọn** theo sau
  (`DiaryLead compact`: tranh 62×48, tiêu đề `title`, câu `note`, nút ghost
  compact). Tờ sổ (`JournalSheet`): ray gáy 24 là anh em trong luồng, không
  đè nội dung; tờ căn giữa bù thêm 24 bên phải nên **trục tranh–chữ–cửa trùng
  trục thẻ**. Câu giữ «của kèo.» liền bằng NBSP để dòng không kết bằng một chữ lẻ. Trang
  đã giữ không bìa là **chỉ chữ**, không icon thế chỗ. «Sở thích» là `ListRow`
  trong thẻ viền.
- **Trạng thái rỗng cả trang (`EmptyState layout="page"`).** Mọi thứ trên một
  trục giữa: khối rộng tối đa 420, chữ căn giữa, nút xếp dọc căn giữa rộng
  tối đa 360 và rộng theo nhãn. Tin nhắn rỗng dùng nó với «Thêm bạn bằng tên
  tài khoản» là **hành động phụ** của chính EmptyState, không còn nút ghost rời
  bên dưới.

**Chưa chứng minh.** Bản ghi đọc mã và mở ảnh web 390 dp/320 dp với dữ liệu
seed; không có verdict ship, không chạy native, TalkBack, iOS, tablet hay đo
tương phản trên máy cho các bề mặt này.

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
  0.30 (bìa **và nền tối**), giấy 0.45 (chỉ nền sáng), mực 0.26; dưới ngưỡng là màu phẳng.
- **Luật Ký Hoạ Trong Sổ** (11/09, review Codex A1 «Khám phá vẫn là danh mục có style»; *hiện hành 21/10:* lead Khám phá và bìa trang quán không ảnh
  đã rời `KyHoa` sang tranh cảnh `PlaceArtwork` chọn theo loại rồi vùng, `canh-ve.ts`; đoạn dưới là lịch sử của tờ ký hoạ): nơi **chưa có ảnh** ở vị trí
  dẫn của Khám phá và ở đầu bài chi tiết không bìa nhận **một tờ ký hoạ** (`KyHoa`, `art/ky-hoa.ts`) — tờ `paper` rộng
  hết cột, 3:1 ở chữ 1.0 và 4:1 ở chữ lớn, **một hình, hai khung cắt** (`preserveAspectRatio slice`, bản `gon` ít lớp
  hơn). Hình gồm **sân khấu theo loại nơi** (`quan-an-local` hiên quán bàn dài · `cafe` góc cửa kính · `vui-choi` đồi
  nhìn từ lan can · `di-choi-dem` quầy đêm dưới dây đèn · loại lạ: tờ ghi gấp trên sàn), **tối đa hai đạo cụ theo
  tag có thật** («View đẹp» đồi phía sau · «Nhóm đông»/«Lẩu» bốn bát · «Chill»/«Nhẹ nhàng» rèm, cây treo · «Món local»
  nồi · «Ngoài trời»/«Săn mây» mây · «Đi đêm»/«Nhộn nhịp» thêm đèn · «BBQ» khói · «Hoa»/«Chụp ảnh» hoa · «Cà phê»/«Trà»
  cốc) và **đúng một điểm coral = nguồn sáng** (bóng đèn dưới mái, đèn thả, mặt trời sát đồi, bóng đèn giữa dây). Ba cỡ
  nét theo độ sâu (gần 3.0 · vừa 2.4 · xa 1.7), bóng là mảng `bong`, mọi mảng tô đi theo đúng đường cong của nét
  viền. **Thật thà:** ký hoạ vẽ *một loại nơi*, không vẽ nơi cụ thể — không tên, không bảng hiệu, không giả ảnh, **không nhãn chữ «Minh hoạ»** (sự thật nằm ở hình vẽ, không ở nhãn); câu
  a11y mở bằng «Ký hoạ …» và **chỉ kể sân khấu/đạo cụ mà khung đọc ấy thật sự vẽ**; bản đồi gọn không có cây gọi
  «đồi nhìn từ lan can», không gọi «đồi thông». Bộ đọc không nhãn chỉ mang mã trung tính trong cả pixel, alt,
  title và tên file; caption/đáp án phải nằm ngoài thư mục phát cho người đọc. (Bản gọn bớt đạo cụ thì câu cũng bớt; test
  «bỏ một tag làm câu đổi ⇔ làm hình đổi»); **bản gọn là tập con của bản đủ** — hai đạo cụ chọn cho bản đủ, bản gọn
  chỉ bỏ, không bù bằng tag sau; live chỉ có loại (không tag) nên hai quán ăn nhận cùng một hiên — đó là sự
  thật của dữ liệu.
  Hàng và cặp so sánh **giữ ô giấy loại nơi**: một khoảnh khắc hình mỗi màn. Không thay ảnh thật có ghi công khi có.
  Đọc mù (agent context mới, chưa biết brief) trên bản v2 đã đánh trượt: bát treo dưới mép bàn, vạch mái = thước, cửa
  kính = laptop có biểu đồ, thông = mũi tên, mây = Venn, móc dây = icon refresh — v3 sửa từng thứ; những gì người ngoài
  đọc được vẫn là câu hỏi cho team (`docs/archive/claude/2026-09-11/ky-hoa-trong-so/`).
- **Bản tối là sổ đóng trên bàn** (11/09, review Codex A3 «bản tối giữ màu thương hiệu nhưng chưa giữ cảm giác giấy»):
  ngày là trang giấy mở (`ground` có vân `giayTrang` 0.45); **đêm là cuốn sổ đóng lại trên bàn** — nền tối trải vân
  **vải bìa** (`Grain vaiBia` 0.30, đo trên nền tối stddev ≈ 8.1–8.6 mức, trước đó vân giấy 0.30 chỉ ≈ 0.8–2.1),
  và mọi **hình vẽ** — Nếp, cảnh, sticker, ô giấy loại nơi — là **tờ giấy đêm** đặt trên vải: `paper` #2e335c (L* 22.7,
  cao hơn nền `#151830` khoảng 13,5 bậc — cùng quan hệ mặt/nền như giấy sáng trên trang) với bóng gấp `paperShade` #181b36
  (thấp hơn mặt 11.8 bậc, như `line` dưới `card` ở scheme sáng). Lý do đổi: tới 11/09 vai `giay` là `card` #1f2340
  (1.14:1 trên nền, thân giấy hoà vào nền) và `bong` là `line` #363b5e **sáng hơn** mặt giấy nên nếp gấp lộn trong ra
  ngoài — hình thành sơ đồ nét. **Thẻ, hàng, chữ giữ token cũ** (`card`, `line`, `ink`): bảng tương phản chữ không
  đổi; `paper`/`paperShade` sáng trùng `card`/`line` nên scheme sáng không đổi một pixel. Đo bằng
  `scripts/do/do-chat-lieu.py` trên cặp native cùng màn.
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
- **Do** dùng `Photo ratio` 16:9 compact / 21:9 rộng cho ảnh dẫn Khám phá, 16:9 cho bìa lật trang của trang quán, và
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
  `PlaceGlyph` ở hàng, cặp so sánh, fallback ảnh hỏng), mực đổi coral khi chọn;
  id lạ là thẻ gấp. Lead không ảnh và bìa trang quán không ảnh là tranh cảnh
  `PlaceArtwork` (21/10), không ô gu.
- **Do** dùng `note` (13/400) cho dòng phụ là một câu và giữ `caption` (600)
  cho nhãn ngắn.
- **Do** xếp kết quả Khám phá dẫn → cặp so sánh (4:3, tim trên ảnh, giá
  đứng riêng dòng) → hàng; lý do dưới ảnh dẫn chỉ khi máy chủ gửi.
- **Do** viết mỗi sự thật một lần trên màn: tên địa điểm ở `phu`, nháp ở đầu
  màn, check-in là một con dấu, luật tính sau «Cách tính».
- **Do** nhân `minItemWidth` với `fontScale` khi ô lưới chứa nhãn chữ.

- **Do** cho tờ giấy một **mép** (`lineStrong` hairline, bo `small`) và một
  **góc cắt** khi nó dẫn; vết gấp `paperShade` chạy mép tới mép; lý do đứng
  dưới tờ, ngoài tờ.
- **Do** đọc loại sổ qua `banTinhCua(loaiSoCua(nhom, doi))`; muốn thêm khác
  biệt giữa các loại thì thêm **trường** vào `BanTinhSo`, không thêm nhánh.
- **Do** giữ bản `trang` của Nếp trùng sha256 baseline; biến thể mới đi qua
  `GAP_NEP` và phải qua cùng bốn cổng của `manh`.
- **Do** nói lỗi bằng `CauTaiCho` ngay dưới control vừa thất bại (hoặc trong
  `footer` ngay trên nút dính đáy), với **một** lối đi tiếp khi có; giữ dữ
  liệu đã hiện khi lần tải lại hỏng.
- **Do** chọn câu lỗi theo `code` của máy chủ trước mã HTTP (`cauTheoMa`), và
  chỉ đưa «Thử lại» khi `laTuChoiVinhVien` sai.
- **Do** cho hành động phá huỷ `RudiButton tone="warn"` `outline`/`ghost` và
  hỏi tại hàng: câu nói cái mất, rồi động từ `warn` + «Thôi».
- **Do** truyền `lyDo` cho mọi nút tắt; nút mà việc chưa có nghĩa thì không vẽ.
- **Do** chọn cột đọc của màn bằng `RudiScreen cot` (`doc` 640, `form` 560,
  `rong` 960) và đo lưới trên chính vùng của nó (`LuoiNguoi`, `gridFor`).
- **Do** đóng lớp trên cùng bằng Back trước khi rời màn: `Sheet` tự làm,
  lớp tự vẽ thì `useLuiLop`; lớp phủ vẽ từ sâu trong màn thì `<LenLop>`;
  thân sheet đang khép đọc `useGiuKhiDong`.
- **Do** viết ngày giờ qua `ngay-viet.ts` («28/09/2026», «10:07 · 28/09»,
  «28 - 29/09», «Tháng 9, 2026»).
- **Do** giữ nhãn nhìn thấy của hành động lặp theo hàng ngắn và đặt tên trợ
  năng mang tên của hàng («Đặt <tên> làm quản trị»).
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
- **Don't** gõ literal `animation: "slide_from_right"` (hay `fade`,
  `slide_from_bottom`) lên `Stack`/`Stack.Screen`, và đừng truyền
  `ReduceMotion.System` cho Reanimated: scale hệ thống không tắt được
  Fragment animation, cờ `System` không thấy setting đổi giữa phiên. Đi qua
  `stackAnimation(...)` và `motion.reanimated`; chứng minh bằng đếm khung.
- **Don't** truyền `dong` lúc mount hay cho cả danh sách; đừng thay cú đóng
  dấu bằng một zoom ease-out, confetti hay toast.
- **Don't** vẽ tay cầm sheet không kéo được; tay cầm là lời hứa kéo-để-đóng.
- **Don't** lặp ô icon app ở đầu mỗi tab; header tab là wordmark trơn (Khám phá: hàng hai mục `DauKhamPha`; tablet: wordmark ở đầu rail).
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
- **Don't** đặt tam giác coral vào góc còn nguyên của một tờ; đừng cho tờ
  giấy bo 20, bóng rơi, hay `overflow: hidden` khi mang nếp.
- **Don't** để `VetGap` dừng ở lòng tờ; đừng dùng nó làm divider bảng.
- **Don't** đặt hai tờ `dan` trên một mặt; đừng nướng coral vào motif
  `thuGapBa`.
- **Don't** tô vạt của tờ nhỏ bằng `bong` (nền tối đọc thành khoét).
- **Don't** so `kind === "pair"` hay `loaiSo === "doi"` ngoài
  `so/ban-tinh.ts`; đừng đọc `BanTinhSo` thành quyền.
- **Don't** thay dữ liệu đang hiện bằng `ErrorState` khi một lần tải lại hỏng;
  đừng đặt câu lỗi nơi bố cục còn chỗ (đầu trang dài, đáy hộp cuộn).
- **Don't** mời «Thử lại» dưới một từ chối vĩnh viễn (403, 404, 410,
  `*_not_found`, `*_expired`…); đừng nói «Kiểm tra mạng» khi máy chủ trả
  `*_unavailable`, hay «Cập nhật app» cho một thứ không tìm thấy; đừng in mã,
  status hay địa chỉ.
- **Don't** tô nền `warn`, hay vẽ hành động phá huỷ bằng nút cam đặc; đừng
  mở hộp thoại để xác nhận xoá. Ngoại lệ giới hạn là Story viewer c9: Modal
  native cách ly nền và nút «Xoá» dùng `warn` đặc như mục bàn giao Story;
  không dùng ngoại lệ đó cho hàng trên giấy.
- **Don't** mờ nút tắt bằng opacity, hay để nút tắt không lời.
- **Don't** đặt `Sheet` trong thân màn khi nó phải phủ header; đừng để
  `Sheet` nhận chạm trong pha mở, hay trượt một quãng cố định thay vì chiều
  cao của nó.
- **Don't** viết ngày bằng `toLocaleDateString`/`Intl`, hay ghép chuỗi ngày
  tay ngoài `ngay-viet.ts`.

## Những gì bản ship KHÔNG phong thánh

**GPL/J 11/10:** không phong thánh nhãn wrap/đầy cột/chiều cao thừa của
7cf/8ec/814, hoặc giả thuyết cached-height thành nguyên nhân đã chứng minh.
Probe1e56 chỉ là chẩn đoán;048 compatibility identity7/2 không có kill/visual
credit,814 detector0 không đổi thành scan061. Text opacity0 đo chức năng
không cho phép giấu nội dung người dùng. F3 ship061 và J P1/P2 ship7cf chỉ
đóng finding đã chấm; F2 (9/0), Node subset và regex `[]` không chứng minh
native causal sequence, geometry/contrast, target thực, TalkBack, motion,
hardware/iOS, strict gate/E2E hoặc deployment. Bản ghi mechanical và verdict
là snapshot khác thời điểm, không sửa câu pending cũ thành cùng chứng kiến
nhau. Toàn app/M–N/I/P–Q/L–R/A–B–S/C–D–E–F/G/K và các cổng cuối còn ngoài
phạm vi bàn giao này; không tuyên bố main/app đã ship.

**I2 10/10 tại814:** ship chỉ F1 và regression hẹp; bảy target còn lại
carry full review a24, không đổi nhãn 96 frame cũ thành814. Không biến
footer phục hồi Gu thành mặc định mọi sheet, giới hạn ô nhập 3–8 dòng thành
lý do cắt nội dung, hoặc minWidth88 thành token tem chung. Giữ harness đỏ
r0 tối200: default scroll chạm footer mới đã thất bại sau assertion lỗi/nút
visible; r1 vuốt riêng thân cuộn, source không đổi. Không nâng witness API,
regex rỗng hay unit xanh thành thị giác; không tổng điểm đẹp hoặc full I/A–S.

**I1 10/10 tại 7d:** không biến F5 partial của62, lỗi nhãn «Để» của4a hay
scan nhầm đường dẫn `ChuKy` (`[]`/exit1) thành pass. Sáu finding carry vẫn
thuộc62; M6 thật không đổi nhãn thành7d. Opt-in wrap không thu chữ hoặc đổi
mặc định mọi caller; nét ký trang trí không thay đồng thuận thật. Ở mốc I1,
I2 còn mở; bàn giao I2 có bằng chứng riêng bên trên. Expanded là cùng
AVD đổi kích thước; subset/regex không thay nghiệm thu thị giác, computed
contrast, native iOS, motion/TalkBack hoặc full strict gate.

**H 10/10 tại d91:** tên dài trong ô Android vẫn cuộn nội bộ hai dòng;
ảnh long chỉ hiện vùng gần caret cuối, không chứng minh toàn bộ (95 ký tự)
cùng thấy. Không biến giới hạn này thành quy tắc cắt tên hoặc chuẩn ô nhập
đã nghiệm thu. Min-height của vùng vải New expanded không thành token
chiều cao nhãn giấy. Sáu fixes resolved không nghiệm thu toàn H/A–S; gate
subset/regex không thành đo tương phản, motion hay điểm đẹp. Nhánh pressed
opacity của hàng người và drift cũ ngoài phạm vi giữ nguyên, không thành
phản hồi bấm thứ hai của hệ.

**K 10/10:** không biến giả định baseline Android thành luật kẻ dòng, hoặc
việc bỏ nét trang trí ở chữ lớn thành lý do thu font/khóa chiều cao. Toolbar
icon chỉ là công cụ ngữ cảnh của trang sửa K, không thay luật nhãn chữ cho
mọi hành động của hệ. Bốn finding đã đóng không thành nghiệm thu toàn K hay
toàn app; phạm vi chưa kiểm giữ mở. Không tạo world/raster mới từ mockup K
chưa duyệt, và không dùng regex TSX làm chứng nhận geometry/contrast native.

**Chín bề mặt 10/10, candidate 797:** không phong thánh điểm 5/10 cũ thành
ngưỡng chấp nhận, verdict fix d9 thành ship 797, số 24/24 thành pass toàn app,
hoặc screenshot Cộng đồng đang tải thành feed đã đạt. Hai ảnh Plan chưa nhóm
visible-final chỉ chứng minh light100 và chạm plus mở khay; không dùng capture
headless, lỗi transport hoặc log regex TSX để tuyên bố native matrix/geometry
đã đạt. Source Composer còn override nút tắt bằng nền
`line`/viền solid thay disabled primitive ADR-0038; đây là drift cục bộ,
không thành mẫu disabled mới, và chưa có đo contrast native cho biến thể.
Metadata/story label inbox rút ngắn ở chữ lớn vẫn là điểm cần review theo
ngữ cảnh, không phải giấy phép cắt dữ kiện. Các nợ cũ ngoài chín bề mặt giữ
nguyên; không dọn source hoặc tạo quy tắc để hợp thức lỗi trong lượt tài liệu.

**Giới hạn lượt 27/09:** theo
[biên bản native](docs/testing/so-ky-niem-native.md), Pixel 6 Android đã build
và kiểm sáng/tối, font scale 1.3, giảm chuyển động; ảnh được mở xem trong lượt
triển khai. Lượt ghi tài liệu này đối chiếu mã và biên bản, không tự kiểm lại
ảnh. Gemini thật đã chạy nhưng sửa chất lượng lời kể bám nguồn vẫn tiếp tục.
Chưa có native iOS, audit copy toàn ứng dụng hay full gate clean-tree tại SHA
cuối. Không suy rộng bằng chứng này sang các màn hoặc cấu hình chưa kiểm.
Sidecar không cập nhật vì phạm vi được giao chỉ gồm PRODUCT.md và DESIGN.md.

Có trong cây nhưng không phải hệ; người sau đừng lấy làm mẫu:

- `Eyebrow` và `SurfaceLabel` **vẫn còn export trong `ui.tsx`** (dòng 237 và
  958 ở head này) dù tài liệu mốc trước ghi đã xoá; không màn nào gọi
  (`grep` 0). Là kicker/eyebrow bị craft floor cấm; giữ lại là nợ dọn kit,
  không phải thành phần. `FloatingGlass`, `Stat`, `ProgressBar` cùng số phận:
  `Stat` và `FloatingGlass` 0 màn gọi; `ProgressBar` một chỗ (kết quả bình
  chọn Group.tsx:362) chỉ được để lại vì có chữ đi kèm, không phải mẫu.
- `WordmarkEmbossed.tsx` (dập nổi bằng bóng lệch): đã xoá ở `5cc57d2`.
- *Lịch sử tới 10/09:* «`Card` v1 và chip tĩnh: còn trong kit, không màn nào
  dùng» — nửa đầu sai, họ Cài đặt dùng 14 lần. **Hiện hành (11/09):** Cài đặt
  là `NhomHang`; `Card` còn ở `story/DangStoryScreen` và
  `tuong/BaiChiTietScreen` là nợ có tên, không phải container của hệ. **Chưa
  chứng minh:** một người đọc đặt Cài đặt cạnh Cá nhân có nhận ra cùng một
  bề mặt không (chỉ có ảnh `r16-90-cai-dat-*`, chưa có thử nghiệm nhìn
  cạnh nhau); tờ AI live (`TheAi`) ở 2.0 chưa có ảnh — chỉ khung `aiSheet`
  fixture được chụp (`r16-91-chat-ai-*`); hai màn còn `Card` chưa đo lại;
  hàng phiên/người đã chặn **live** chưa có ảnh (fixture không máy chủ, fetch
  treo nên hai màn dừng ở skeleton — `r16-92-phien-*`, `r16-92-da-chan-*`); đường lỗi
  lưu Cài đặt (`loi`, dòng `warn` dưới «Xoá tài khoản») chưa ép ra để render.
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
  mẫu trạng thái vô hiệu. **Hiện hành (ADR-0038, kit 04/10):** `RudiButton`
  và `StampButton` tắt là nền `card` viền đứt `lineStrong` chữ `inkSoft` kèm
  `lyDo` (mục Buttons); dòng trên là lịch sử, ảnh `bs-03` chưa chụp lại.
- **Chế độ tối sau loạt sửa cuối** chỉ có bảng art (`art.png` nửa dưới) và
  `toi-1.3/` chụp **trước** loạt sửa; cảnh trong `EmptyState`, cặp so sánh
  và ô gu ở dark chưa có ảnh.
  **11/09:** ba màn tối (Khám phá, khay sticker, màn lỗi có cảnh) đã có cặp native trước/sau ở
  `docs/archive/claude/2026-09-11/toi-giay-tren-vai/`; câu hỏi «bản tối mất chất giấy» đóng bằng vải nền + giấy đêm, không
  phải bằng token. Còn chưa chụp: các cảnh còn lại của `EmptyState` ở tối, cặp so sánh, Album.
- Album theo ngày chỉ ở `AlbumLive.tsx` và chỉ đọc từ mã; album fixture
  `Memories.tsx` vẫn lưới đều ba cột (ảnh `bs-18-album`), là hai nhịp của
  hai cây, không phải hai kiểu album của hệ.
- *Lịch sử tới 08/09 (`a03f563d`):* hai cảnh `chua-co-hoi`, `chua-co-ban` và
  bốn tư thế Nếp ngoài `moi`, `ghi-lai` mới có trên bảng art, chưa màn nào
  gọi. **Hiện hành (09/09, #588):** mười cảnh đều đã nối vào màn — xem mục
  «Mười cảnh» và bảng ở `docs/archive/claude/2026-09-09/tra-loi-canh-moi-im-lang.md`.
- *Lịch sử tới 08/09:* bảy sticker cũ (khối màu lớn, chỉ lớp tô) và `cho-ti`
  (nét + hai mảng sắc độ) là hai ngữ pháp trong một khay, chờ quyết định.
  **Hiện hành (09/09, #587):** cả tám cùng một ngữ pháp nét–giấy–nếp gấp, hai cỡ
  đọc, cùng sàn `CHAN_NEP`; audit 09/09 §2 xác nhận «không còn khay pha hai
  phong cách». Ghi hai dòng cũ lại để người sau không khôi phục quyết định cũ
  (audit F46).
- *Lịch sử tới 10/09:* `_layout.tsx` khai `animation: "slide_from_right"`
  cố định và `useMotion` truyền `ReduceMotion.System`; tái audit 10/09 (R1) quay
  được chi tiết quán vẫn trượt với cả ba scale = 0. **Hiện hành (11/09,
  `1cfe0ee4`):** stack và Reanimated theo bit `reduced` sống (luật ở mục
  «Chuyển động»). Điều **chưa** chứng minh và không phong thánh: bản release và
  iOS chưa có clip (`animationDuration` riêng của native-stack iOS chưa đụng);
  chuyển tab `fade → none` chỉ đo gián tiếp qua `m1-doi-tab`; mọi số là dev
  client trên máy ảo, không phải cảm giác chạm trên điện thoại thật.
- *Nghĩa hình vòng 2 (11/09):* bốn hình sửa lại (`ket-xe`, `tra-tien-ne`,
  `chua-co-tin-nhan`, `bo-loc-che-het`) mới có bảng có nhãn/không nhãn do người
  đã đọc brief nhìn; **người chưa đọc brief** chưa đọc `khong-nhan-v2-*.png`,
  khay 64 **trên máy** chưa chụp (chỉ bảng web), iOS chưa có. Vạch đèn hậu coral
  của `ket-xe` là **ngoại lệ có ghi lý do** (một vạch nói «đuôi xe»), không phải
  ngân sách mới cho sticker: hình sau vẫn tối đa một chi tiết coral ngoài nếp
  gấp.
- Icon Ionicons vẫn là ngôn ngữ của control (tab, sự thật, nút tròn, chip
  không `leading`); lớp vẽ chỉ thay icon ở **nội dung phân loại**, không
  phải một cuộc thay icon toàn hệ.
- *Lịch sử tới 10/09:* `PlaceGlyph` là đĩa `accentSoft` giữ cả ở dark theo
  quyết định 08/09 vòng 2; `PlaceLead` in con dấu «HỢP GU» **và** «Hợp gu
  nhờ …». **Hiện hành (11/09, nhánh `ui6-kham-pha-mot-ly-do`):** ô giấy và
  một dấu cho một địa điểm (mục «Hàng địa điểm», Luật Một Dấu); cùng ngày
  (nhánh `ui7-ky-hoa-trong-so`) lead không ảnh và đầu bài không bìa rời ô giấy
  sang tờ ký hoạ `KyHoa`. Ghi lại để người sau không khôi phục đĩa. **Chưa chứng minh:** người đọc không được
  báo trước có nhận ô giấy là «danh mục» hay đọc là nút; lý do live (câu của
  mô hình) ở 2.0 — `LyDo` cho 2 dòng (3 khi chữ lớn), câu dài hơn nữa vẫn
  «…»; ảnh `r14-80-*-fs2.0-*` chỉ có tag một–hai chữ của fixture, chưa có ảnh
  live; iOS và tablet không có ảnh ở lát này.

- **Phase 1 «Nếp truyền giấy» (12/09, head `137c6c04`)** chỉ có trên bảng
  `app/dev/ui-lab.tsx` và Maestro `.maestro-bs-r18/99` (lịch sử Git @ `137c6c04`); **không màn người
  dùng nào** dựng `ToGiay`, `Nep gap="manh"`, `ThuGapBa` hay đọc `BAN_TINH`.
  Bố cục «ba tờ + lý do dưới tờ» là bố cục **lab**, không phải màn. Chuỗi
  `tuVung` của `hai-nguoi`/`doi` chưa render ở đâu.
- **Vòng đọc mù 4** đọc đúng thân 53/56 («vuông vắn, bè hơn hẳn»), nếp ngang
  qua mặt («nếp gấp giấy»), `gap-lai` («đang gấp giấy»), `up-xuong` («đè tờ
  giấy dưới đất»), motif tối («góc gấp lên»), dải 6 ở 48 («thấy được, ở
  ngưỡng»). Hai sửa toạ độ **sau** vòng 4 (`up-xuong` nhìn [1.6, 3.0] sau khi
  nới kẹp `nhin` ±3; dải ở bản chi tiết 3 → 4.8) là **quyết định của tác giả**,
  chưa ai đọc mù lại; phán quyết «fix rồi ship» cho phép điều đó.
- **Đọc mù = một reviewer context mới đọc ảnh cắt mỗi vòng**, không phải
  người dùng; ảnh là fs1.0/2.0 sáng-tối trên máy ảo (`r18-99-*`), không có iOS,
  không tablet.
- **Hai `ToGiay` trùng tên**: `chat/TheAi.tsx` có hàm cục bộ `ToGiay` (tờ AI
  `card` bo 20, mục «Ghi chú AI») và `ui/ToGiay.tsx` export `ToGiay` (tờ thư
  gấp ba). Không phải hai biến thể của một component; tờ AI vẫn bo `base` và
  chưa được đo lại theo luật tờ giấy. Nợ đặt tên, ghi để người sau không hợp
  nhất nhầm.
- **`KyHoa` (PR #604) trên cùng trang lab** mang coral trên tờ không `dan`;
  ghép chung một surface với `ToGiay dan` sẽ vỡ «một coral dẫn». Reviewer
  vòng 3 ghi ngoài phạm vi; quyết ở Phase 2 khi lắp màn thật.
- **Mép cắt chéo hairline của `GocGapThat` hiển thị mờ hơn viền tờ**
  (anti-alias nửa trên coral nửa trên nền); không đổi cách đọc, không có số
  nào được in để hợp thức nó.
- **Con số tương phản `line`/`lineStrong` ở chú thích `ToGiay.tsx`** là trích
  spec §16.2, không phải đầu ra của `sinh_token_ui_v2.py`; số của hệ nằm ở
  hai mục do script sinh.
- Đề nghị đổi nền tờ khỏi `paper` (vòng 2) **không nhận**: đổi token là việc
  của spec/ADR, không của Phase 1.

**Đợt nâng cấp UI/UX 01–04/10 (B1–B11, `direction.md`), ghi 04/10.** Lượt
ghi tài liệu này **chỉ đọc mã** trong cây làm việc (head `a58d11a8` cộng sửa
chưa commit); không chạy cổng, không mở ảnh. Bằng chứng native và QA retest
nằm ở `docs/claude/2026-10-01/ui-ux-upgrade/` (`change-log.md`,
`qa-handoff.md`), không suy rộng ở đây. Có trong cây nhưng chưa là hệ:

- **Hợp đồng `Sheet` v2 không có test node riêng** (`khe-lop.test.mjs` chỉ
  giữ khe lớp; Back trình duyệt qua `lui-web.ts`, trần 82%, 250 ms, khép mờ,
  `useLuiLop` và `useGiuKhiDong` đo bằng harness QA và ảnh, không bằng node).
  Bề rộng tối đa của sheet nay là `COT_DOC` (`adaptive.ts`), cùng một số với cột
  đọc.
- Năm chỗ lệch khác mà lượt ghi này thấy đã được sửa trước khi bàn giao (B11):
  - lưới ngày và hai nút tháng của `ChonNgayLich`;
  - nút mở xoá bình luận ở `BaiChiTietScreen` mang màu `warn`;
  - hỏi tại hàng gom về `HoiTaiHang`;
  - `CHUA_VE` trống;
  - `lyDo` có cổng `nut-tat-co-ly-do.test.mjs`.

**Đợt hoàn thiện Đua vịt và Chat E2EE 15/10 (nhánh `codex/nep-ui-recovery-handoff-20261014`).**
Đã đóng các finding mở từ đợt bàn giao 14/10 và kiểm chứng trực tiếp trên Android emulator (`320×640`, `390×844`, `768×1024`, sáng/tối, chữ `1.0`/`2.0`, IME mở):

- **Sân sông và làn vịt (`duong-dua.ts`, `RandomGame.tsx`):** pitch dọc tách đủ cho cả full stage và compact stage ở 2–8 lựa chọn (4 lựa chọn: full `216dp` / compact `176dp`; 8 lựa chọn: full `306dp`, sprite `36dp` / compact `256dp`, sprite `28dp`, badge thu gọn `18×18dp` khi `> 6` làn) để không chồng vịt hoặc badge tại vạch xuất phát và vạch đích caro. Nền sông và các cutout vịt đang dùng được preload bằng `Image.prefetch` với timeout hữu hạn `450ms` trước khi hiển thị lớp nghệ thuật của sân, không để nhịp đếm ngược đầu tiên hiện nền phẳng hoặc thiếu vịt.
- **Thẻ Đua vịt trong chat (`ChatRandomCard.tsx`):** sân thu gọn nằm trong `stageWrap` có đệm ngang `16dp` khớp trục lề trái của dòng trạng thái `Đang đua · Xs`; ẩn dòng tóm tắt lặp khi sân thu gọn đang hiển thị trực tiếp.
- **Ranh giới cụm tin và khung ảnh (`nhip-tin.ts`, `anh-niem.ts`, `GroupChatLive.tsx`):** tin đã giải mã thành thẻ bình chọn hoặc thẻ Nếp chọn giúp (`poll`/`random`) là tin độc lập, không nhập vào cụm `dau`/`giua`/`cuoi` của tin chữ liền kề; khung ảnh co theo bề rộng khả dụng của cột (`availableWidth - 18`, sàn `132dp`, trần `208dp`); chú thích ảnh chưa tải được của tin mình dùng bảng màu theo nền bong bóng thật, đảm bảo tương phản ở cả giao diện sáng và tối.

## Cổng phải xanh trước khi đổi hệ này

```bash
python3 -m pytest services/api/tests/web -q                   # token guest.css khớp tokens.json; đọc ui/**.tsx và DESIGN.md; mọi tỉ lệ in ở đây đo lại được
python3 scripts/sinh_token_ui_v2.py                           # đổi màu: sinh lại 4 gương, không gõ tay
cd apps/mobile && node --test tests/rudi-khong-hex.test.mjs   # không file nào trong vỏ RuDi tự gõ mã màu ngoài theme.ts
cd apps/mobile && node --test tests/duong-svg.test.mjs        # đường SVG parse được theo cách Java parse
cd apps/mobile && node --test tests/motion.test.mjs           # stackAnimation → none khi Reduce Motion, giữ nguyên khi không; durationFor/moneyCountUpMs; cổng khung hình thật ở docs/archive/claude/2026-09-11/motion-v2/
cd apps/mobile && node --test tests/rudi-khong-card-trong-cai-dat.test.mjs   # không file nào trong screens/cai-dat import Card; NhomHang có trong kit; in các màn còn dùng Card (nợ có tên)
cd apps/mobile && node --test tests/art-duong.test.mjs        # mọi hình của lớp vẽ (Nếp, gu, motif, cảnh) chỉ M/L/C/Z tuyệt đối, vai màu hợp lệ
cd apps/mobile && node --test tests/art-ky-hoa.test.mjs       # ký hoạ: lớp hợp lệ trong 288×96 ở cả hai khung đọc, đúng một lớp coral, bản gọn ⊂ bản đủ, ≤ 2 đạo cụ, câu a11y đổi ⇔ hình đổi
cd apps/mobile && npx tsc -p tsconfig.test.json && node --test tests/rudi-chat-sticker.test.mjs   # tám id khớp stickers.json; mọi lớp của mọi sticker ở cả hai cỡ đọc parse như Java; lớp tô kín, lớp nét dương; id lạ vẽ «khac»
cd apps/mobile && npx tsc -p tsconfig.test.json && node --test tests/kham-pha-ly-do.test.mjs tests/khong-mo-coi.test.mjs   # chonLyDo bỏ tag mô tả đã nói, guTheoTag trước guTheoLoai; khongMoCoi nối hai chữ cuối bằng NBSP
node scripts/do/kiem-lap-loi.mjs <hierarchy.xml>   # dump uiautomator Khám phá fixture: 0 node «hợp gu», tiêu đề mục đúng một node, dẫn có một lý do là tag, mô tả không lặp từ
cd apps/mobile && npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-anh-ghi-cong.test.mjs   # ảnh catalogue chỉ tới Image trong ba khung in ghi công; không ai đọc trần anh.source, không ai gọi AnhChang
python3 -m pytest tests/test_chat_lieu_tiles.py -q            # ô mực đo trên coral ở 0.26 nằm 6 đến 12 mức (gốc repo)
cd apps/mobile && npx tsc -p tsconfig.test.json && node --test tests/art-duong.test.mjs   # thêm 12/09: bản trang trùng sha256 fixtures/nep-trang-baseline.json; manh đúng một dải coral (laDaiGap), không mực lên dải, dày ≥ 2.5/5.5; lấp đầy ≥ 0.87, tỉ lệ ≥ 0.94; ba pose mới một coral; thuGapBa 0 coral, 5 đỉnh, hai vết ở 1/3, 2/3
cd apps/mobile && node --test tests/so-ban-tinh-mot-cho.test.mjs   # ngoài so/ban-tinh.ts và ba chỗ có sẵn không file nào so loại sổ; ba chỗ ấy vẫn còn; ban-tinh.ts là lá
cd apps/mobile && node --test tests/dau-gach-dai.test.mjs          # tuVung của BAN_TINH và mọi chuỗi app không có gạch dài
```

Cổng riêng của v3 «Sân khấu giấy» (đều nằm trong `npm test`, chạy lẻ được như sau từ
`apps/mobile`, sau `npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs`):

```bash
node --test tests/chu-tren-giay.test.mjs          # D14: không chữ cam/cảnh báo/mờ trên nền paper; danh sách nợ đã về rỗng (S9)
node --test tests/giay-vat-the.test.mjs           # vật giấy (hoá đơn, vé, tem, cuống, phong bì, trang xé) đúng ngữ pháp Java, trong hộp, tất định
node --test tests/thanh-pho.test.mjs              # 15 sân khấu thành phố + bưu thiếp chung, id đọc từ destinations_vn.py, câu mô tả «Ký hoạ …»
node --test tests/ky-niem-giay.test.mjs           # ảnh in nghiêng tất định theo id, album rỗng tách «chưa có kèo» khỏi «kèo chưa tới ngày», huy hiệu mới
node --test tests/loi-qc-nguoi-chat.test.mjs      # ghim bản sửa B2/B3/B8, mực người qua AvatarNguoi, bong bóng xem trước dùng đúng bangMauChat
node --test tests/route-fixture-co-phien.test.mjs # B5: ba route fixture chuyển về màn live khi có phiên
node --test tests/khong-vien-web.test.mjs         # B7: mọi TextInput mang KHONG_VIEN_WEB, không còn khung focus của trình duyệt
node --test tests/suc-song-man-tao.test.mjs       # màn tạo, màn tiền, sổ đôi và các màn trong phạm vi phải render ít nhất một vật sân khấu
node --test tests/san-khau.test.mjs tests/nep-roi.test.mjs tests/muc-nguoi.test.mjs tests/token-san-khau.test.mjs tests/skia-ranh-gioi.test.mjs tests/chuoi-maestro-con-song.test.mjs
```

Cổng của đợt nâng cấp UI/UX 01–04/10 và hoàn thiện Đua vịt / Chat E2EE 15/10 (cũng trong `npm test`, cùng bước dựng như trên):

```bash
node --test tests/nut-pha-huy-tong-warn.test.mjs  # mọi RudiButton có nhãn phá huỷ mang tone="warn"; ngoại lệ có tên và lý do; quét ≥ 10 nút
node --test tests/trang-rong-co-hinh.test.mjs     # mọi EmptyState có illustration, trừ KHONG_HINH (im lặng có lý do) và CHUA_VE (nợ chưa vẽ)
node --test tests/cau-loi-theo-ma.test.mjs        # câu lỗi theo code trước status qua thongDiepNguoiDoc thật; quy tắc theo hình mã
node --test tests/o-nhap-muc.test.mjs             # ONhapMuc: không hộp, chỉ gạch; ô nhập 48; gạch 2dp khi viết/sai mà chữ không nhích; lỗi thay dòng phụ và được đọc
node --test tests/khe-lop.test.mjs                # LenLop đưa con lên khe lớp của RudiScreen, giữ state và callback
node --test tests/rudi-b9-ke-va-tuong.test.mjs    # khoangNgayChuyen («28 - 29/09», «30/09 - 02/10»), thangNamVN
node --test tests/rudi-random-native.test.mjs tests/rudi-chat-nhip-tin.test.mjs tests/rudi-e2ee-anh-niem.test.mjs  # pitch làn vịt 2..8 lựa chọn, preload sân, lề 16dp thẻ chat, tách cụm poll/random và co khung ảnh
```

Màn native thì cổng là **emulator**, không phải web export (dòng FINISH của
hợp đồng): light/1.0, dark/1.3, tablet bằng `wm size`, rồi đọc ảnh và đo
pixel; tsc, web export và detector đã mù với ba lỗi thật ở lát này (crash
`transform: undefined`, crash parse `d`, vân dừng ở một phần ba).

