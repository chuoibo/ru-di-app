# Phase 1 «Nếp truyền giấy» — nền không cần máy chủ: bằng chứng và hai vòng đọc mù

Nhánh `claude/p0-w-hn-1-nen-to-giay`. Kế hoạch: Phase 1 của
`~/.claude/plans/home-lakiet-mobile-docs-codex-2026-09-0-stateless-river.md`. Spec: mục 16, 17, 19
của `docs/superpowers/specs/2026-09-12-mode-hai-nguoi-nep-truyen-giay-design.md`.

## Cái gì được dựng

| Phần | File | Cổng |
|---|---|---|
| Nếp biến thể `gap: "manh"` (gấp làm tư, góc coral gấp vào trong hé một dải dọc mép cắt) | `apps/mobile/src/rudi/art/nep.ts` | `art-duong`: baseline sha256 của bản `trang` trùng `main 2d0b9777` (680 lớp, 19 pose); `laDaiGap` nhận diện dải theo hình, quét 22 pose × 7 biểu cảm × 2 cỡ × 4 độ nghiêng, không mực sơn lên dải; mảnh vuông hơn trang (đo tỉ lệ) |
| Biểu cảm `giu-kin`; ba tư thế `dua-giay` · `up-xuong` · `gap-lai` | `nep.ts` | bảy mày khác nhau; `giu-kin` phẳng và ngắn hơn `quyet`; ba tư thế mới đúng **một** coral; đổi biến thể không thêm/bớt coral ở pose nào |
| Motif `thuGapBa`; component `ToGiay` + `VetGap` | `art/motif.ts`, `ui/art/Motif.tsx`, `ui/ToGiay.tsx` | thư gấp ba đúng hai vết ở h/3, 2h/3, không coral; detector Impeccable `[]` |
| Module bản tính của sổ | `src/rudi/so/ban-tinh.ts` | `so-ban-tinh-mot-cho`: chỗ duy nhất so loại sổ; ba chỗ `kind === "pair"` có sẵn khai tường minh, kiểm **hai chiều** |
| Bảng ui-lab «Tờ giấy» và «Nếp trang/mảnh»; Maestro `.maestro-bs-r18/99` | `app/dev/ui-lab.tsx` | Maestro exit 0 ở bốn cấu hình |

Không màn hình người dùng nào đổi. Không token, không route.

## Vòng đọc mù 1 — reviewer context mới, ảnh cắt không nhãn, **trước** khi mở packet

Mười hai ảnh cắt từ bản chụp fs1.0 sáng (commit `b068dcfe`). Câu trả lời nguyên văn rút gọn:

| Hỏi | Trả lời mù | Ý định | Kết |
|---|---|---|---|
| A1/A2 một hay hai nhân vật | «Cùng một nhân vật, hai bản vẽ» | cùng Nếp | khớp |
| Nét chạy qua mặt A2 là gì | «Vết gấp tờ giấy, gấp tư; không phải sống mũi» | vết gấp | **khớp** — điều tác giả không chắc số 1 |
| Coral ở cỡ 48 | «Thấy khe đỏ nhỏ ở góc thân cả D48 và E48» | còn nhận diện | **khớp** — điều tác giả không chắc số 2 |
| Hình nào «đang giữ điều gì» | A2, B2 | mảnh gấp vào = điều được giữ | khớp |
| `up-xuong` đang làm gì | «Đẩy/kéo vật nhỏ trên sàn bằng que, kiểu quét» | úp một tờ, tay đè | **lệch** |
| Coral trên C2 (`dua-giay`) | «Hai chỗ: khe ở góc thân + chấm ở góc tờ nhỏ trong tay» | một coral | **lệch** |
| F_ba: một tờ, ba thẻ, hay ba tờ | «**Ba tấm thẻ.** Bo góc bốn góc đều, kẻ ngang chia hàng như bảng, góc đỏ đọc như badge, không vết gấp» | ba tờ thư gấp ba | **lệch** |
| Hai kẻ ngang trong tờ | «Gạch phân cách hàng (divider)» | vết gấp | **lệch** |
| Tờ nào là «việc cần làm bây giờ» | G_dan, «nhưng chỉ nhờ chữ» | góc coral | khớp đích, lệch cơ chế |

Phán quyết vòng 1: **fix**, bảy điều vật chất. Sửa ở commit `bab00fb0`:

1. Tờ giấy đọc thành thẻ → `radius.small` (bo của mọi tờ giấy trong app), **góc trên phải vuông** khi mang nếp gấp, bỏ `overflow: hidden` (góc coral không bị cắt cong), `VetGap` chạy **mép tới mép** bằng margin ngang âm.
2. Hai coral ở `dua-giay` → tờ trong tay dùng `bong`; cổng đếm **lớp** (không né theo hình).
3. Hai tờ `dan` trên một mặt → đúng một.
4. Ba hàng không đều (lý do nằm trong hàng ba) → lý do đứng **dưới tờ, ngoài tờ**.
5. `up-xuong` đọc thành «kéo que» → tờ sàn cao hơn, có góc gấp thường, tay gập khuỷu đưa xuống, mitten nổi trên tờ.
6. Mảnh hẹp hơn chứ không vuông hơn → mở rộng 40 → 48 (chân neo ở y 76 nên không rút chiều cao); cổng đo tỉ lệ.
7. Lab khai trạng thái `khoa` chưa có vật liệu → bỏ; hẹn mở là lát 3.

Hai điều **giữ** vì đọc mù xác nhận đúng ý: hai nếp giao nhau vẽ trước mặt; dải coral hé ở mép cắt.

## Vòng đọc mù 2 — ảnh chụp lại ở `bab00fb0`, mười ảnh cắt, reviewer context mới

| Hỏi | Trả lời mù | Kết |
|---|---|---|
| Ba khối xếp dọc là gì | «Ba tấm thẻ trắng viền mảnh; hai kẻ ngang đọc là divider hàng của list; thẻ đầu là việc bây giờ nhờ góc đỏ, và đó là dấu duy nhất» | **lệch** |
| Hình đỏ ở góc | «Tam giác dán ở góc, badge / dog-ear trên bao thư; **không** đọc là góc giấy gấp vì phần trắng của thẻ vẫn lồi tới tận góc»; nền tối: «notification» | **lệch** — chẩn đoán đúng cơ chế |
| Chú thích nhỏ thuộc khối nào | «Khối phía trên» (đo 24 px trên, 54 px dưới) | khớp |
| `dua-giay` mảnh: bao nhiêu chỗ đỏ | «Một, nét cam mảnh góc trên phải» | khớp |
| `up-xuong` | «Đặt/đè tay xuống một tấm thẻ trên sàn, dáng chờ» | khớp |
| trang \| mảnh: cái nào vuông/bè hơn | «**Trái (trang)** vuông vắn bè hơn; phải (mảnh) hẹp hơn, bo góc, **lục giác**» | **lệch** |
| Cỡ 48 còn đỏ | «Có, mỗi hình một nét cam, rất nhỏ» | khớp |
| `gap-lai` | «Cúi khom, hai tay ôm vật chữ nhật sát bụng, co lại» | khớp |
| Motif | «Thẻ index kẻ dòng, hoặc icon list rỗng» | **lệch** |

Phán quyết vòng 2: **fix**. Reviewer đo pixel xác nhận `VetGap` đã chạm viền hai bên — tức sửa #1 vòng 1 *đã làm đúng như bảng ghi*,
nhưng cái được sửa không phải cái làm người đọc gọi nó là thẻ. Cái đó là **góc**: viền tờ vẫn đi tới góc vuông, coral nằm trong
góc còn nguyên. Sửa ở commit kế tiếp:

1. `ToGiay` góc gấp là góc **cắt**: tam giác xoá màu nền (`nen`, mặc định `ground`) lên góc, vạt coral, mép cắt `lineStrong` nối tiếp viền,
   hai cạnh tự do của vạt bằng mực. Không còn `NepGoc` trên tờ.
2. Mảnh: cạnh trái thẳng, chân thẳng, sáu đỉnh; cổng đo **độ lấp đầy hộp bao** (mảnh 0.876 / trang 0.838, sàn 0.87) thay tỉ lệ hộp bao.
3. `thuGapBa` mang góc cắt với vạt `bong`.

Không nhận: đề nghị đổi nền tờ khỏi `paper` (spec §16.3 đã chọn `paper`, và bản sáng không có token nào sáng hơn) — đổi token là việc
của spec, không của Phase 1. Điều còn treo để đo ở Phase 2 khung đầy đủ (spec 20.5 phép đo 2): ba tờ **không** `dan` xếp đều trên nền
vẫn có thể đọc thành danh sách thẻ; đòn bẩy vật chất duy nhất còn lại là ngữ pháp góc gấp ở tờ dẫn.

## Vòng đọc mù 3 — ảnh chụp lại ở commit sửa vòng 2, mười ảnh cắt, reviewer context mới

| Hỏi | Trả lời mù | Kết |
|---|---|---|
| Ba khối xếp dọc | «**Ba tờ giấy** trắng xếp dọc, không phải bảng, không phải thẻ bo tròn kiểu app; kẻ ngang là dòng kẻ của tờ; tờ đầu là việc bây giờ nhờ tam giác đỏ ở góc, dấu duy nhất» | **khớp** |
| Cận góc | «Góc giấy **gấp lại**: tam giác đỏ ở đúng vị trí góc bị mất, cạnh chéo trùng cạnh vát; mặt sau lật lên, không phải badge vì không tròn và không nổi lên trên viền» | **khớp** — điều cốt lõi ba vòng đã khép |
| Nền tối, góc cam | «Góc gấp lộ mặt sau cam, không badge, không chấm thông báo» | **khớp** |
| Motif (sáng) | «Tờ giấy có kẻ dòng, viền mực, góc trên phải gấp lại (tai chó)» | khớp |
| Motif (tối) | «Giấy có góc gấp, nhưng mặt gấp tối hơn thân làm góc trông như bị **khoét**» | lệch |
| trang \| mảnh | «Cùng nhân vật; trái vuông vắn bè hơn; phải **hẹp hơn** và thẳng đứng hơn; 5 cạnh thẳng, góc cạnh, không tròn» | lệch nửa: hết «lục giác/tròn», còn «hẹp» |
| `up-xuong` | «Kéo hay đẩy một vật thấp, hộp nhỏ, kiểu va li» | lệch |
| 48 | «Có nét đỏ rất mảnh, phải nhìn kỹ mới thấy, gần như mất» | lệch |
| `gap-lai` | «Cầm một tờ nhỏ hai tay trước ngực, một chân đá ra sau; không đọc ngồi, không đọc gấp» | lệch |
| Nếp dọc qua mặt | đọc ra «nếp dọc» ở ba ảnh, không phải sống mũi | khớp (đóng điều tác giả không chắc số 1) |

Phán quyết vòng 3: **fix** — năm điều, không điều nào chạm ngữ pháp góc cắt đã đọc đúng. Sửa ở commit kế tiếp: thân mảnh mở sang trái
thành 53/56 (0.946) với vai trái đi theo, hai nếp chia bốn ô gần vuông (nếp ngang lên y 50.5 giữa mắt và miệng); dải coral dày 6 đơn vị ở
bản rút gọn (cổng đo bề dày); `up-xuong` tờ nằm phẳng 4:1, tay đè giữa tờ, thân nghiêng và nhìn xuống; `gap-lai` đứng, tờ hai mảng gập
thật; vạt `bong` → `giay` dưới viền mực ở motif và hai tờ nhỏ (nền tối không còn đọc «khoét»).

Reviewer ghi thêm hai điều **ngoài phạm vi nhánh** để Lead biết: hai ô `KyHoa` (PR #604) cùng trang lab mang coral trên tờ không phải
`dan`, khi ghép chung một surface với `ToGiay` sẽ phá luật «một coral dẫn» — cần quyết ở Phase 2 khi lắp màn thật; mép cắt chéo hairline
của `GocGapThat` hiển thị mờ hơn viền tờ (anti-alias nửa trên coral nửa trên nền), không đổi cách đọc.

## Vòng đọc mù 4 — ảnh chụp lại ở commit sửa vòng 3, tám ảnh cắt, reviewer context mới

| Hỏi | Trả lời mù | Kết |
|---|---|---|
| trang \| mảnh | «Hình phải **vuông vắn và bè hơn hẳn**; thân là hình vuông hơi lệch, góc trên phải tai chó; hai đường be là nếp gấp giấy, gấp bốn» | **khớp** |
| Nếp ngang qua mặt | «**Nếp gấp giấy** ngay từ nhìn đầu, cùng màu với đường dọc, chạy hết chiều ngang; không kính, không băng» | khớp |
| `up-xuong` | «Cúi tay xuống, đè hoặc vuốt một tờ giấy nhỏ có góc gấp nằm dưới đất»; nhưng «mắt nhìn về người xem, không thấy hướng xuống vật rõ ràng» | khớp hành động; hướng nhìn lệch |
| `gap-lai` | «Đang gấp giấy: tờ trong tay **đang bị gập lại**, cạnh phải gãy góc, không phẳng» | khớp |
| Cột 48 | «Mỗi hình có chấm cam ở góc gấp; thấy được nhưng nhỏ, hàng giữa phải căng mắt» (đo pixel: ba hàng 66–68 px bằng nhau) | khớp cơ học, ở ngưỡng |
| Motif (tối, ảnh cắt lại) | «Góc **gấp lên**, không phải lỗ khoét» | khớp |
| Ba tờ (sáng) | «Ba tấm thẻ (ba phiên bản của cùng một tờ); khối trên là việc bây giờ nhờ góc gấp đỏ» | tờ dẫn đúng; hai tờ không hành động **không có dấu giấy** → nợ |
| Nền tối, hàng `dua-giay` | «Hình giữa vuông hơn rõ rệt; hình nhỏ nhất có thấy cam» | khớp |

Phán quyết vòng 4: **fix rồi ship**. Hai sửa toạ độ (commit kế tiếp): `up-xuong` nhìn [1.6, 3.0] sau khi nới kẹp `nhin` ±1.6 → ±3
(không pose/cảnh nào truyền quá 1.6 nên bản có sẵn không đổi — cổng sha256 xác nhận); dải coral ở bản chi tiết 3 → 4.8 đơn vị
(bản rút gọn đã 6). Không mở vòng 5: hai sửa chỉ là toạ độ, và phán quyết cho ship sau khi sửa; ảnh ghim ở `anh/` là bản sau sửa.

**Nợ ghi lại, không chặn Phase 1:**
1. Hai tờ không hành động trong lab trơn (chỉ tờ `dan` có góc cắt) nên reviewer vòng 4 đọc «ba tấm thẻ» trong khi vòng 3 đọc «ba tờ giấy».
   Spec 15.1 chỉ có **một tờ đang mở** trên màn thật, nên đo lại ở Phase 2 trong khung đầy đủ trước khi quyết có cho mọi tờ một góc cắt `giay`.
2. Bản trang và bản mảnh chưa đọc là **cùng một nhân vật ngay lập tức** (một reviewer đọc trang là «bản lỗi, gấp chéo hoặc cắt rời»). Bản
   trang khoá toạ độ; nếu muốn kéo hai bản lại gần nhau thì đó là quyết định về bản trang, ngoài phạm vi nhánh này.
3. Hai ô `KyHoa` (PR #604) mang coral trên tờ không phải `dan`: khi ghép cùng surface với `ToGiay` sẽ phá luật «một coral dẫn».

## Ảnh

Lượt chụp đầu sau sửa để lộ hai lỗi **flow** (không phải hình): `scrollUntilVisible` neo vào tiêu đề bảng Nếp không cuộn vì
tiêu đề đã hé ở đáy khung trước, nên hai ảnh trùng nhau; và hàng `gap-lai` bị thanh điều hướng che chân. Flow đổi neo sang
`lab-nep-trang-dua-giay-96` và `lab-nep-trang-gap-lai-96` (`centerElement: true`, an toàn vì còn bảng phía dưới); ảnh đầu neo vào `lab-to-giay-nhap`
vì ở cỡ chữ 2.0 tiêu đề bảng «Tờ giấy» cũng hé ở đáy khung. Ảnh trong `anh/` chụp trên cây **`af060fbc`** (bản cuối, sau sửa vòng 4).

`anh/` — 16 PNG của bản **sau sửa** (`r18-99-{to-giay-tren,to-giay-chot,nep-manh-tren,nep-manh-duoi}-fs{1.0,2.0}-{sang,toi}.png`),
pin sha256 trong `.repo-guard-allowlist.json`. Bản trước sửa không lưu vào Git; bảng vòng 1 ở trên là bản ghi.

## Cổng đã chạy trên cây cuối `af060fbc`

`npm test` đủ bộ: 842/842 · `tsc --noEmit` sạch · `python3 -m pytest services/api/tests tests`: 3433 passed, 712 skipped (chạy trên
`137c6c04` và chạy lại trên `af060fbc`) · `art-duong` 24 ca cấp một (baseline sha256 bản trang trùng `main 2d0b9777`, `laDaiGap`, độ lấp
đầy ≥ 0.87 và rộng/cao ≥ 0.94, bề dày dải ≥ 4.5/5.5, bảy mày, một coral ở ba tư thế mới) · `so-ban-tinh-mot-cho` 4 ca · `dau-gach-dai` ·
`rudi-khong-hex` · detector Impeccable `[]` trên `ToGiay.tsx`, `nep.ts`, `motif.ts` · repo guard staged từng commit · Maestro r18 flow 99
exit 0 × 4 cấu hình × 5 lượt chụp (mỗi lượt sau một commit sửa).

Số đo hình học cuối: mảnh lấp 0.886 hộp bao, rộng/cao 0.946; trang 0.838 / 0.821. Dải coral: 4.8 đơn vị ở bản chi tiết, 6 ở bản rút gọn.

## Bẫy gặp trong lượt này (đã ghi memory)

Worktree với `node_modules` là **symlink** thì Metro dev server không bundle được cây đó — dev client báo «There was a
problem loading the project», Metro không log request nào; `cp -al` (hardlink) chữa. `mobile_native.sh` chạy từ worktree
thiếu `android/.rudi-native-fingerprint` nên không mở dev client và báo đỏ «không nạp bundle» — đỏ vì thiếu bước mở.
Thăm dò Metro bằng `index.bundle` là sai URL với expo-router.
