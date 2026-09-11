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

## Cố ý không làm trong lượt này

- Đo release qua HTTPS (Codex chọn HTTPS): việc dựng stack sau TLS là một lượt riêng; tầng 1 v2 đủ để đóng R2 về
  **phương pháp**, không tuyên «§5 đạt».
- Ảnh demo Album nhất quán + ghi công (nợ §4 cũ), Sở thích phân nhóm, glyph gamepad Puppy Farm, «bản tối mất chất
  giấy» (ghi câu hỏi mở), iOS/máy thật/TalkBack/chat live, animation mới.
