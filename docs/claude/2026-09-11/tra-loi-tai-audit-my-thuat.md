# Trả lời tái audit 10/09 (Codex) — «đã có bản sắc, chưa stunning xuyên suốt»

Trả lời cho `docs/codex/2026-09-10/reaudit-native-my-thuat.md` (+ `assessment-a-my-thuat.md`, `assessment-b-ky-thuat.md`)
trên `main 0c1a4169`. Phán quyết của họ: **REQUEST_CHANGES cho cổng mở rộng mỹ thuật và đóng §5 motion**. Tài liệu này
ghi từng finding: đã đo gì, chưa đo gì, quyết định nào để lại cho team. Bằng chứng của Codex được commit **một phần**
(ba bản `.md`, manifest, kết quả ma trận/motion, và đúng những PNG được trích dẫn ở đây — pin sha256); video `.mp4` của họ
không vào Git, chỉ dẫn đường dẫn trong checkout.

Thứ tự làm theo mức của họ: R1/R2 (P1) → R3 (P2, ưu tiên cho «stunning») → F45 còn mở + R4 (P2) → R5 (P3).

## PR 1 — R1 · R2: Reduce Motion tới stack, cổng đo fail-closed

### R1 (P1) — đóng trên Android dev client, có đối chứng hai chiều

Codex đúng, và đúng sâu hơn dòng họ chỉ: `_layout.tsx:155` là literal, **và** react-native-screens Android chuyển
cảnh bằng Fragment `Animation` không đi qua ba `*_animation_scale` (không có mã Reduce Motion nào trong thư viện), nên
scale 0 không bao giờ tắt được cú trượt — app phải tự xin cắt. Thêm một chỗ họ chưa thấy: spring của kit `Sheet` dùng
`ReduceMotion.System` của Reanimated, cờ đọc một lần lúc khởi động → đổi setting giữa phiên, sheet «Tạo mới» vẫn spring
(đo được 7 khung ở scale 0 sau khi chỉ sửa `_layout`).

Sửa: `stackAnimation()` trong `motion.ts` (thuần, test) → `_layout.tsx` đọc `useMotion().reduced` cho `screenOptions`
và bảy `Stack.Screen`; `useMotion` truyền `Always/Never` theo bit sống thay cho `System`.

Đo bằng khung hình, không đọc mã (`docs/claude/2026-09-11/motion-v2/README.md`): screenrecord quanh một flow chỉ có hai
lần đổi màn, `so-khung.py` đếm số khung mỗi lần đổi. Cổng tự kiểm trên **chính video của Codex** trước: `visual-reduce-
confirm.mp4` cho max 8 khung (còn trượt — đúng như họ thấy). Sau sửa, cùng một phiên app, đổi scale khi app foreground:

| lượt | flow | scale | khung mỗi lần đổi màn (thô · gộp) | đọc |
|---|---|---|---|---|
| a | chi tiết «Tiệm Nướng Xóm Lèo» + Back | 1 | [4, 2, 9] · [7, 9] | trượt (đối chứng dương) |
| b | cùng flow, scale về 0 **giữa phiên**, cùng pid | 0 | [2, 1] · [2, 1] | cắt thẳng |
| c | cùng flow, scale về 1 | 1 | [8, 8] | trượt trở lại |
| d / e | sheet «Tạo mới» (transparentModal + Sheet) + Back | 1 / 0 | [6, 5] / [1, 1] | trượt / cắt |
| f / g | `rudi://check-ins/new` (`presentation: modal`) + Back | 1 / 0 | [5, 2, 7] / [1, 1] | trượt / cắt |

Cây đo = commit `1cfe0ee4`; pid 26102 không đổi qua cả bảy lượt (`r1/*.scale.txt`). Run 2 khung ở lượt b là một khung
**nền giấy + tab bar** (màn cũ đã gỡ, màn mới chưa vẽ) rồi chi tiết trọn — không phải trượt dở; cắt thẳng để lộ ~33 ms
nền, ghi là quan sát mở (crossfade là lựa chọn Android cho phép, chưa làm vì lượt này không thêm animation). Khung trước/giữa/sau ở
`motion-v2/r1/*.png`; trạng thái cuối khi cắt là màn trọn (sheet đủ bốn hành động, check-in đủ tiêu đề + nút) — chỉ mất
chuyển động. **Chưa đo:** bản release, iOS (đã nối nhưng không có clip), máy thật, 120 Hz, TalkBack.

Reviewer Impeccable context mới (11/09) phán `fix` với bảy điểm — cả bảy đúng và đã sửa: trap INT/TERM phải `exit`
(không thì Ctrl-C xong vẫn đo tiếp ở scale sai dưới nhãn cũ); kit R1 thiếu pid/scale đọc lại và cây đo chưa commit;
`so-khung.py` chưa gộp run bị khung trùng cắt vụn; canary chưa thử nhánh pid đổi / khung 0; `ExploreLive` còn
`ReduceMotion.System`; README thiếu release/iOS/TalkBack ở «chưa chứng minh»; runner có thể `put ""` khi đọc gốc hỏng.

### R2 (P1 cho cổng) — sửa phương pháp, đo lại; canary đỏ phải đỏ

Đúng cả năm ý của B1–B3 (xem README motion). `do-motion.sh` v2: warm-up **một lần** ngoài mọi cửa sổ; mỗi chuỗi
`pid` trước → `gfxinfo reset` khi tiến trình sống → flow **chỉ thao tác** (m1–m4 bỏ `runFlow _vao-*`, mở đầu
`assertVisible "Khám phá"`) → dump → `pid` sau; hàng hợp lệ khi `rc=0 && pid không đổi && khung>0`, **exit 1** nếu có
hàng không hợp lệ; ba scale được đọc, lưu và **trả đúng giá trị gốc bằng trap**; cột «khung>150ms» = tổng bucket ≥150
(150 là **một bucket**, histogram tới 4950 ms — README cũ nói «trần bucket» là sai, đã sửa); `_vao-live.yaml` rẽ cả
hai điểm vào («Rủ Đi thôi!» và «Chào bạn») qua `_nhap-otp.yaml`.

`do-motion-canary.sh` (adb/maestro giả, không chạm máy): maestro exit 42 → runner exit 1 ✓; maestro exit 0 + dump
thật → exit 0 và hàng có số ✓ (khung>150ms = 12 = 7+4+1 đúng histogram m1 cũ). Bài học đắt một lần: canary bản đầu
để `~/.maestro/bin` đứng trước PATH giả → chạy Maestro **thật** lên máy ảo; runner giờ chỉ **nối thêm** đường tool khi
PATH chưa có.

Bảng v2 (dev client, sau sửa R1; `docs/claude/2026-09-10/motion/dev-client-v2/`, pid không đổi, exit 0 cả hai lượt):

| chuỗi | thường: khung / janky / p99 / khung>150ms | Reduce Motion: khung / janky / p99 / khung>150ms |
|---|---|---|
| m1 đổi 4 tab ×5 | 271 / 13,28% / 85ms / 0 | 45 / 80,00% / 97ms / 0 |
| m2 cuộn ×4 | 907 / 2,54% / 31ms / 0 | 704 / 1,99% / 27ms / 0 |
| m3 sheet ×5 | 406 / 8,62% / 34ms / 0 | 63 / 33,33% / 31ms / 0 |
| m4 chi tiết + Back ×5 | 273 / 10,62% / 65ms / 0 | 60 / 31,67% / 40ms / 0 |

Số khung lượt thường (271 / 907 / 406 / 273) gần với cửa sổ Codex tự đo (267 / 848 / 317 / 280): hai phép đo độc lập
khoanh được cùng một thao tác. Không so % với bảng v1 (khác cửa sổ), không so % thường–reduce (khác mẫu số: cắt cảnh làm
mất gần hết khung chuyển cảnh — đổi tab 20 lần còn 45 khung). p99 là nhãn bucket; không khung nào ≥150 ms ở cả tám hàng.
Đây là **baseline một lượt trên máy ảo**, không phải ngưỡng release.

## PR 2 — R3: Khám phá nói lời hứa một lần; dấu loại nơi bằng giấy; câu lỗi không mồ côi

Codex đúng, và luật đã có sẵn trong DESIGN.md («Luật Nói Một Lần», `:1608`; «Don't lặp một sự thật hai chỗ», `:1747`)
— code vi phạm chính luật của mình. Trước: «Gần bạn, đúng gu» → con dấu «HỢP GU» → «Hợp gu nhờ Chill và View đẹp»
(= `"Hợp gu nhờ " + tags.slice(0,2)`) → mô tả «Nướng thơm lừng, view đồi cực chill». Bốn lần một lời hứa; ba thẻ đầu
cùng một cái bát trong đĩa hồng; ở 2.0 khối dẫn chiếm trọn màn.

**Cơ chế (không phải sửa chữ):**
- **Một dấu cho một địa điểm** (`HangDiaDiem.tsx`): có `lyDo` thì in dòng lý do (✦ + một dòng `label` tím), **không** in
  con dấu; không có lý do thì con dấu như cũ (live rows với «AI MATCH 95%» giữ nguyên, `kham-pha.test` không đổi).
  Áp cho lead (cả nhánh có ảnh), cặp so sánh và hàng. Tiêu đề mục «Gần bạn, đúng gu» là nơi **duy nhất** nói «gu».
- **Lý do = một tag chưa nói** (`kham-pha/ly-do.ts` thuần, test 3 ca): `chonLyDo(tags, sub)` lấy tag đầu mà không từ
  nào (bỏ dấu) xuất hiện trong mô tả — với dữ liệu cũ của Codex nó ra «Nhóm đông» vì «Chill»/«View đẹp» đều nằm trong
  mô tả. Fixture viết lại 12 mô tả để nói điều tag chưa nói (xom-leo: «Nướng than ngoài hiên, thơm cả con dốc»); live
  giữ nguyên câu `reason` của mô hình.
- **Dấu loại nơi có chủ ý**: `PlaceGlyph` từ đĩa `accentSoft` + icon toàn coral → **ô giấy** (`card`, hairline `line`,
  bo `radius.small`) với hình gu vẽ bằng **mực** và đúng một chi tiết coral của chính hình — cùng ngôn ngữ tờ giấy của
  Album, không ảnh stock, không «icon trong vòng tròn». `guTheoTag(tags)` cho tag có thật chọn hình trước danh mục
  («Món local» → món địa phương): Bánh căn Lệ hết là cái bát thứ ba; hai quán còn lại vẫn bát — **thật thà**, không
  bịa khác biệt. Hàng (`PlaceRow`) dùng cùng ô giấy thay ô vuông hồng.
- Chi tiết live không ảnh (`PlaceDetailLive.tsx`): bỏ khung 16:10 `card` rỗng với icon giữa (đúng «một icon bát nhỏ và
  khoảng trống») — đầu bài gọn: ô giấy + con dấu + tên, cùng luật F01 08/09 mà fixture đã theo. Chi tiết fixture:
  AiNote «View thoáng, món nướng dễ chia sẻ…» → «Nhóm 8 người ngồi được một bàn, món nướng chia nhau dễ.» — một lý do
  về **nhóm**, không đọc lại chip.
- **Câu lỗi mồ côi** (ảnh 20): `EmptyState` body đi qua `khongMoCoi()` (`ui/chu.ts`, test): hai chữ cuối nối bằng NBSP nên
  «thử lại.» xuống dòng cùng nhau. Hai flow `77-loi-*` (r12, r13) đổi khoảng trắng cuối thành `.` trong regex.

**Đo (`docs/claude/2026-09-11/native-r14/`):** cổng `kiem-lap-loi.mjs` trên XML uiautomator — 0 node `/hợp gu/i`, đúng 1
node «Gần bạn, đúng gu», sau tên dẫn là một lý do ≤ 4 chữ, mô tả không chứa từ của lý do. **Đối chứng:** chạy trên chính
`05-explore.xml` của Codex → ĐỎ ba lý do (4 node «hợp gu»; lý do dài; mô tả lặp «chill, view»). Sau sửa: 1.0/2.0 × sáng/tối
đều XANH (`anh/r14-80-*.{png,xml,kiem.txt}`), lý do «View đẹp», mô tả «Nướng than ngoài hiên, thơm cả con dốc».

Ảnh chi tiết không ảnh (`anh/r14-81-*`), màn lỗi (`anh/r13-77-*`, XML có U+00A0 giữa «thử» và «lại.»), bàn thử ui-lab
(`anh/r14-82-*`). **Quyết định 08/09 bị lật có ghi lý do:** «đĩa giữ `accentSoft` cả trên nền tối» — đĩa hồng + icon
toàn coral là mặc định của mọi app; ô giấy giữ đúng bảng giấy/mực/một coral của hệ (DESIGN.md ghi lịch sử «không khôi phục»).

Reviewer Impeccable context mới phán `fix` với năm điểm, cả năm đúng và đã sửa: câu `reason` live bị cắt ở
`numberOfLines 1` → 2 dòng (3 khi chữ lớn); `chonLyDo` so chuỗi con nên «Săn mây» bị «sáng» nuốt → so cả từ (test thêm);
DESIGN.md lệch → documenter + sửa tay; live không có `gu` → live `Place` không có tag, ghi rõ thay vì bịa; hai fixture còn
tag lặp mô tả → sửa. Verdict pass: **ship** (trong phạm vi năm fix và bề mặt đã chụp lại).

**Chưa chứng minh:** người chưa đọc brief đọc ô giấy; câu `reason` live dài ở 2.0 (không có stack live); TalkBack — hàng có
`accessibilityLabel` «Mở …» nên lý do/giá không được đọc (nợ có sẵn, có tên); iOS/tablet. Về «khối dẫn ở 2.0»: bớt con dấu
và một dòng lý do (~90 px) nhưng mô tả mới dài hơn một dòng; mắt tìm tên → lý do → giá không đọc lại lời hứa — **đúng
điều kiện của Codex**, không tuyên «ngắn hẳn».

## PR 3 — F45 còn mở · R4: nghĩa hình vòng 2, đọc mù trước khi tin

Codex đúng ở cả bốn: bus đọc ra kiosk vì là khối 27×59 **không bánh**; ba tờ xoè không có dấu tờ bạc nào; đuôi bong bóng
là **đỉnh của cùng đa giác** nằm dưới tờ giấy Nếp (x 16…52) nên bị che trọn; lưới thoáng + `vongHo` coral = spinner.

**Cách làm:** ứng viên bằng chính primitives, render cạnh bản cũ ở 64/120 sáng/tối, nhìn rồi chọn, ghi vào nguồn — rồi
**reviewer Impeccable context mới đọc mù bảng không nhãn trước khi đọc packet**. Vòng mù đầu (v2) đánh trượt hai hình: đuôi
cắt từ góc dưới-trái đọc ra «bảng có góc vạt», lưới tô đọc ra «bảng tính/lịch có ô được chọn»; xe nửa đạt («xe tải/xe đẩy»).
Sửa hình học theo ba gợi ý rồi đọc mù lại (v3): xe → «kẹt sau đuôi xe buýt/van nhỏ, hai ô kính, đèn hậu»; bong bóng →
«bong bóng thoại trống, đuôi trỏ về miệng — đang nói, chưa nói gì»; bộ lọc → «tấm che trên tờ giấy có chữ, một ô nhìn qua»;
tiền → «tờ bạc/tiền mặt» (đạt từ v2). Phán quyết **ship** trong phạm vi bốn fix.

**Hình học chốt (`docs/claude/2026-09-11/nghia-hinh-v2/README.md`):**
- «Kẹt xe»: Nếp 0.66/x0 −9 (khung xe máy co theo `k`), đuôi xe buýt **rộng hơn cao** (≈42×38), **hai ô kính** `bong`, hai bánh
  mực trên `CHAN_NEP`, **một** vạch đèn hậu coral — ngoại lệ ngân sách coral có ghi lý do (một vạch nói «đuôi xe»; hai chấm
  từng bị loại vì thành mặt).
- «Trả tiền nè»: tờ trên có **ô bầu dục** `bong` (chân dung) + **khung đôi** (chỉ ở 120) — hai dấu mọi tờ bạc có mà vé không
  có; không xu/tick/ký hiệu tiền/QR/ngân hàng (ADR-0021 giữ).
- `chua-co-tin-nhan`: thân bong bóng đặt cao hơn miệng, đuôi là **tam giác riêng mọc từ cạnh đáy**, mũi (53,58) ở độ cao miệng
  ngoài mép tờ giấy; bong bóng **trống**, bỏ chấm coral (chấm làm nó thành bảng).
- `bo-loc-che-het`: **bỏ `vongHo`** (quyết định của Codex); «che» có **cái bị che** — tờ bốn dòng chữ, tấm che `bong` lệch để lộ
  đầu dòng, lưới 2×2, một ô hở viền coral bốn nét thẳng lộ mẩu dòng chữ.

**Cổng cơ chế (không phải nghĩa):** cấm cung coral ở cả `chua-doc-duoc` và `bo-loc-che-het`; mọi đỉnh bong bóng ≥ x 52 và
đỉnh trái nhất ở độ cao miệng; `ket-xe` ≥ 4 hình tròn tô; `tra-tien-ne` có ô bầu dục `line`, không teal. 21/21 art+sticker,
vocabulary test, npm test 795/795. Trên máy: khay 64 và cảnh ui-lab (`nghia-hinh-v2/native/`).

**Chưa chứng minh:** người chưa đọc brief đọc bảng không nhãn (`khong-nhan-v2-{sang,toi}.png` — 4 sticker + 4 cảnh, hai
sticker và hai cảnh không đổi làm đối chứng) — vẫn cần con người; iOS; khay 2.0 (flow bàn thử 71 đỏ ở bước cuộn ở 2.0, có
từ r13).

## PR 4 — R5: nhịp tờ AI trong chat; Cài đặt về hệ giấy–mực

Codex đúng ở cả hai, và câu «Không màn nào trong đợt này còn gọi `Card`» của DESIGN.md hoá ra **sai**: họ Cài đặt gọi
`Card` 14 lần (8 ở `CaiDatScreen`, kể cả thẻ bọc `Segmented` — thẻ lồng thẻ, điều chính DESIGN.md cấm).

- **Kit `NhomHang`** (`ui.tsx`): hàng trên giấy, mỗi con một kẻ tóc, không thẻ/bóng/bo — hình Profile, DiemDenScreen,
  HangDiaDiem đã chép tay ≥ 3 lần nên extract là hợp lệ. **Cả năm màn họ Cài đặt** bỏ 14 `Card`; `Segmented` nằm thẳng
  trên giấy; lỗi là chữ trần; nhãn flow 44 giữ nguyên văn. Hai màn ngoài bề mặt audit còn `Card` (`story/DangStoryScreen`,
  `tuong/BaiChiTietScreen`) — **nợ có tên**, test in ra mỗi lần chạy, không sửa lượt này.
- **Tờ AI fixture** (`Group.tsx`): tiêu đề `title` (không `h2` trong luồng chat) → một dòng cốt «3 ngày 2 đêm · đồ ăn local ·
  săn mây» → «Xem lịch trình» + «Mở bình chọn» (nhãn pinned giữ) → **một cửa mở** «Vì sao phác vậy» (mẫu «Cách tính»:
  `Pressable` 48, `label inkSoft` + chevron, `accessibilityState.expanded`) → «Rủ Đi AI» + badge «AI nháp». Bỏ câu «Nhóm sửa
  được trước khi chốt.» (badge nói trạng thái nháp một lần — Luật Nói Một Lần). Thẻ live `TheAi`: tiêu đề lịch trình và
  câu hỏi bình chọn `h2` → `title` cùng nhịp.
- **Cổng**: `tests/rudi-khong-card-trong-cai-dat.test.mjs` (không `Card` dưới `screens/cai-dat/`, `NhomHang` có và được dùng,
  in nợ còn lại); board `.maestro-bs-r16/` 90 (Cài đặt hai nửa) + 91 (tờ AI đóng/mở, assert **không** còn «Nhóm sửa được
  trước khi chốt») — 8/8 xanh ở 1.0/2.0 × sáng/tối (`docs/claude/2026-09-11/native-r16/`). `npm test` 800/800; `imp detect`
  `[]` trên tám file.

**Chưa chứng minh:** phép «đặt cạnh nhận ra cùng sản phẩm» bằng người thật; thẻ AI live ở 2.0 (không có stack live); TalkBack
đọc cửa mở; hai màn còn `Card`.

## Tổng kết lượt 11/09 — đối chiếu bảng phán quyết của Codex

| Mục của Codex | Lượt này | Còn để team |
|---|---|---|
| R1 (P1) Reduce Motion chưa tới stack | **Đóng trên Android dev client**, có đối chứng hai chiều trong cùng phiên (#597) | iOS, release, máy thật |
| R2 (P1) script đo xanh khi không đo được | **Đóng về phương pháp**: fail-closed + canary ba nhánh đỏ + trap; bảng v2 dev client (#597) | Đo release qua HTTPS local |
| R3 (P2) Khám phá lặp lời | **Đóng** trên fixture 1.0/2.0 × sáng/tối bằng cổng XML có đối chứng đỏ (#598) | Người chưa đọc brief đọc ô giấy; câu `reason` live ở 2.0 |
| F45 còn mở | **Ba hình đọc mù đạt** (v3) sau một vòng trượt (#599) | Người ngoài đọc bảng không nhãn `khong-nhan-v2-*` |
| R4 bộ lọc | **Đóng**: bỏ `vongHo`, tấm che trên tờ có chữ (#599) | — |
| R5 (P3) chat AI · Cài đặt | **Đóng** trên fixture (#600); hai màn ngoài phạm vi còn `Card` ghi nợ | Thẻ AI live 2.0; hàng Phiên/Đã chặn live; đường lỗi lưu Cài đặt |

Phương pháp đã đổi so với lượt 10/09 và nên giữ: **reviewer context mới đọc mù bảng không nhãn trước khi đọc packet** — chính
vòng đọc mù ấy đánh trượt hai hình mà tôi (đã biết brief) thấy «đã ổn». Mọi cổng mới đều được cho **đỏ trên bằng chứng lỗi
của Codex** trước khi tin (video reduce-confirm, XML 05-explore).

**Không làm lượt này, có tên:** ảnh demo Album nhất quán + ghi công (nợ §4 cũ); Sở thích phân nhóm; glyph gamepad Puppy Farm;
«bản tối mất chất giấy» (quan sát của Codex, chưa có điều kiện đóng — cần một lượt nhìn cảnh ở cỡ thật trên nền tối);
iOS/máy thật/TalkBack/chat live; animation mới (§5 bước 3 vẫn là đề xuất).
