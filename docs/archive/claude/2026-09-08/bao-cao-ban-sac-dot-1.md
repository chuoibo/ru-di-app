# Đợt 1 «Chừa một chỗ cho nhau»: tầng art, ba màn chứng minh, biên tập chữ

**Ngày:** 08/09/2026. **Nhánh:** `claude/p0-w-ui3-hoan-thien-native`, HEAD `8c1d8600` (bảy commit
trên `3d89f070`: `cdccb590` art → `c6fc7921` ba màn → `464c8726` chữ → `96bfcaa5` lưới font lớn →
`a97cb7ce` sửa theo finish review → `6069ac97` nối lại fixture Khám phá → `8c1d8600` loạt sửa hai). **Đầu vào:** báo cáo Codex 07/09
`docs/archive/codex/2026-09-07/bao-cao-ban-sac-va-nhip-thi-giac-rudi.md` và concept Nếp 08/09
`docs/archive/codex/2026-09-08/nep-concept/README.md`. **Phạm vi do bạn chốt:** A2 + A1 + B1 của §13.1
(tầng art, ba màn Sở thích / Khám phá / Album, bảng biên tập chữ §8.5), dừng lại để xem ảnh native
trước khi trải ra 41 bề mặt rỗng và huy hiệu. **Không đổi:** ADR-0020/0021, tagline, wordmark, token
màu, backend, luật tiền, id từ vựng sở thích và sticker.

Quy trình: Impeccable pipeline (preflight → context → Flow B cho tầng art, Flow C với `delight` ·
`distill` · `layout` cho ba màn → craft-floor → detector → finish-reviewer context mới → documenter
ghi DESIGN.md từ bản đã ship). Mini-bảng chụp ảnh `apps/mobile/.maestro-bs*` để untracked (không phải
cổng, không nằm trong `.maestro/`).

## 1. Ba quyết định đã nhận và cách hiểu

| Câu hỏi | Trả lời | Hệ quả trong code |
|---|---|---|
| Linh vật | README concept: «Chọn phương án 1, có Nếp nhưng tách lớp» | `art/canh.ts`: năm cảnh hoàn chỉnh khi `nep: false`; Nếp là lớp trên cùng, không đứng cạnh tiền/lỗi/conflict |
| Phạm vi | Tầng art + ba màn + chữ, rồi dừng | Không đụng huy hiệu, khay Tạo (ngoài chữ), Check-in (ngoài chữ), sticker |
| Nhánh | Làm tiếp trên nhánh hiện tại | Ba commit xếp sau 10 commit UI3 chưa có PR |

## 2. Đã làm gì

### 2.1 Tầng art (`cdccb590`)

`apps/mobile/src/rudi/art/` thuần (không React, không màu) + `apps/mobile/src/rudi/ui/art/` render qua theme:

- `net.ts`: builder chỉ phát `M/L/C/Z` tuyệt đối, số thập phân thường; Q và cung tròn đổi sang cubic.
  Lý do: react-native-svg parse `d` ở Java lúc mount, một token lạ là app chết ở khung hình đầu (bảng
  05/09), tsc và web export mù.
- `nep.ts`: Nếp theo concept 08/09 (thân giấy gấp ngắn hơi bè, vạt chéo, một góc coral, mày lệch, tay
  bao, chân thon), sáu pose, bản 96 và 48 vẽ riêng. Đã bỏ con dấu máy bay và hai gạch trên thân như
  README yêu cầu; rút chiều cao so với sheet.
- `motif.ts`: vòng hở (không bao giờ là progress ring: không animate, không đi cùng phần trăm, khoảng hở
  ≥ 60°), đường chuyền, góc gấp.
- `gu.ts`: tám sở thích trên lưới 48, một nét, tối đa một chi tiết coral; món local là nồi + thẻ món.
- `canh.ts`: năm cảnh rỗng (chưa có hội / kèo / ảnh / bạn, tìm không ra).
- Màu là vai (`giay`/`bong`/`muc`/`gap`/`mo`/`split`/`ai`) → `card`/`line`/`ink`/`accent`/`accentSoft`…
  Sáng: giấy trắng mực đậm; tối: giấy đậm mực sáng; cùng silhouette. Không token mới, không hex.
- Cổng mới `tests/art-duong.test.mjs`: parse mọi đường của mọi pose × 2 cỡ × phép đặt, motif, glyph,
  cảnh có/không Nếp, theo đúng ngữ pháp Java; kiểm kín, trong khung, tất định, vai màu. Bộ đọc tự
  kiểm mình bằng mẫu `Q`, `L Z`, số mũ, `-0`.

Preview render bằng Chrome headless của repo (không phải native):
`.impeccable/review/ban-sac-2026-09-08/art-preview.png` (thư mục gitignored; bản mới nhất sau hai loạt
sửa là `xem-art4` trong scratchpad phiên, đã chép vào cùng thư mục dưới tên `art-preview-sau-sua.png`).

### 2.2 Ba màn (`c6fc7921`)

| Màn | Silhouette mới | Chữ điều phối thường trực |
|---|---|---|
| Sở thích `Onboarding.tsx` | Tiêu đề → tám đồ vật (`GuGlyph` 40dp) → đếm → một hàng ngân sách → CTA | 9 câu / 81 từ → 6 câu / 44 từ (−46%) |
| Khám phá `ExploreLive.tsx` + `Discovery.tsx` + `HangDiaDiem.tsx` | Lead (chỉ khi có ảnh) → **hai ứng viên cạnh nhau** (`PlaceCompare`) → hàng; khung không ảnh vẽ đồ vật theo loại | Bỏ đoạn hướng dẫn 5 ý dưới ô tìm (≈30 từ, dùng `{CAU_MAU}` nên bộ rút chuỗi không đếm được); placeholder dài hơn 4 từ |
| Album `AlbumLive.tsx` | Tiêu đề mang ngày/nhóm **một lần**; ảnh dẫn chỉ caption + «Ảnh của nhóm»; ảnh sau lead gom theo **ngày Việt Nam** (`nhomTheoNgay`) xếp cặp, tấm lẻ cuối ngày đi rộng | 13 câu / 124 từ → 12 câu / 89 từ (−29%) |

Thêm vai chữ `typography.note` (13/400). `canh.ts` không để id rơi qua làm mặc định. Fixture album
`Memories.tsx` cũng theo luật một nơi (nơi ở thanh đầu, nhóm trên ảnh in, ngày + số ảnh ở tiêu đề).

### 2.3 Biên tập chữ (`464c8726`)

Bảng §8.5 áp cho: Welcome trang 3, Tin nhắn, Đợt thu («Làm mới»), lịch trình fixture + AI + live (tên
địa điểm thay «Đã gắn địa điểm · bấm để mở»/«Có thể thay đổi»; nháp nói một lần), Check-in (con dấu là
trạng thái), Thành tích («Cách tính» mở luật), khay Tạo (dòng phụ chỉ ở hai việc dễ nhầm), Đăng ảnh
(một ô caption), Bill mẫu. Mỗi dòng đổi giữ sự thật cũ ở một chỗ khác trên cùng luồng. Không chuỗi nào
bị 47 flow Maestro hay test ghim (đã grep; danh sách ghim ở plan). `DESIGN.md:979` và
`.impeccable/design.json` (ItineraryAI) cập nhật quy tắc `phu`.

## 3. Bằng chứng native

Emulator `rudi` (Android 15, 1080×2400, `emulator-5554`), dev client `com.lakiet.rudi`, cửa fixture
(không API). Mini-bảng `apps/mobile/.maestro-bs/` (untracked) chạy qua `scripts/mobile_native.sh
--flows .maestro-bs`. Ảnh ở `.impeccable/review/native/<ts>-<sha>/`. **Ảnh là của fixture** — cùng bề
mặt với ảnh 03/04/10/12/18/20 của báo cáo 07/09; hai màn live (`ExploreLive`, `AlbumLive`) chưa được
chụp vì cần stack OTP (mục 6).

### 3.1 Sáng, font 1.0 (`sang-1.0/`, hai lượt chạy)

| Ảnh | Đối chiếu 07/09 | Thấy gì |
|---|---|---|
| `bs-03-so-thich`, `bs-03-so-thich-da-chon` | 03 | Tám đồ vật khác nhau, chọn = nền + glyph coral + dấu; không còn đoạn dẫn và tiêu đề hỏi gu |
| `bs-04-kham-pha`, `bs-04-kham-pha-cuon` | 04 | Lead, rồi hai ứng viên cạnh nhau trên cùng trục, rồi hàng có hairline; không còn hint 5 ý |
| `bs-04-tim-khong-ra` | (chưa có) | Cảnh bản đồ gấp + vòng hở với Nếp, 168dp, trái-canh cùng inline empty state |
| `bs-18-album` | 18 | Ngày một lần, nhóm một lần trên ảnh in, nơi ở thanh đầu |
| `bs-12-lich-trinh-ai` | 12 | «Nháp» một lần; chặng mang tên địa điểm; chặng không nơi không có dòng phụ |
| `bs-05-timeline-cuoi` | 05 | Chặng có nơi mang tên địa điểm (accent = bấm được), chặng nhóm ghi «Cả nhóm». Nit: tiêu đề fixture đã chứa tên quán («Ăn trưa - Bánh căn Lệ») nên dòng phụ lặp tên |
| `bs-20-check-in` | 20 | Một trạng thái mỗi hàng: con dấu «ĐÃ TỚI» hoặc «Chưa tới» + vòng rỗng |
| `bs-10-khay-tao` | 10 | Mỗi việc một dòng; dòng phụ chỉ ở «Đăng kỷ niệm» / «Đăng story»; phạm vi nhóm một lần |

Lượt 1 đỏ vì harness (thiếu `00-smoke` trong mini-dir, «Đến nơi rồi?» ngoài khung cần cuộn); lượt 2
XANH: 3 flow, NEO 2b cắn, canary đỏ đúng thiết kế. Bộ ảnh ổn định: `.impeccable/review/ban-sac-2026-09-08/sang-1.0/`.

### 3.2 Tối, font 1.3 (`.impeccable/review/ban-sac-2026-09-08/toi-1.3/`, bảng XANH 3 flow)

- Tám glyph gu và Nếp giữ silhouette trên nền tối (giấy đậm, mực sáng, góc coral `accent` tối); cảnh
  «tìm không ra», so sánh hai ứng viên, album đọc được.
- **Lỗi thật bắt được:** ở font 1.3, lưới hai cột làm nhãn «Shopping» bị bẻ từ («Shoppin / g») và
  «Món local» xuống hai dòng. Sửa: `minItemWidth` của lưới nhân theo `fontScale` (≥ 1.3 → một cột trên
  compact), commit riêng; kiểm lại ở mục 3.4.

### 3.3 A/B một cảnh có / không Nếp (README concept, hợp đồng 6)

Cùng máy, cùng dữ liệu, cùng sáng/1.0, chỉ đổi `nep` trong mã giữa hai lượt; ảnh ở
`.impeccable/review/ban-sac-2026-09-08/ab/`: `A-co-nep.png` (Nếp cầm bản đồ gấp) và `B-khong-nep.png`
(bản đồ gấp với vòng hở, đứng một mình). B vẫn kể được «đường đi chưa tới nơi»; A thêm người đang nhìn
bản đồ. **Chưa chọn**: mã mặc định `nep=true` theo concept; quyết định A/B là của bạn trên hai ảnh này
(không dựa mô tả).

### 3.4 Sau hai loạt sửa theo finish review

`sua-sang-1.0/` (loạt một, 3 flow XANH), `sua-ab/` (A/B sau căn lề), `sua-sang-1.3/bs-03*` (ngân sách
ở 1.3; hai ảnh `bs-04*` của lượt này đã xoá vì chụp lúc `Discovery.tsx` bị hoàn nguyên nhầm),
`sua2-sang-1.0/` (loạt hai, flow 50 XANH). Ảnh quyết định từng mục ghi ở mục 7.

### 3.5 Tối / font 1.3 sau loạt sửa (`sua2-toi-1.3/`, bảng XANH 3 flow)

Đóng khoảng trống reviewer nêu: một cột, dây đèn, câu hỏi ngân sách + ghi chú, cặp so sánh với giá
dòng riêng, chip Gu, câu lý do đều giữ ở nền tối và font 1.3. Nit còn lại (có từ trước đợt này):
dòng dữ kiện của **hàng** vẫn `numberOfLines 1`, ở 1.3 «120K - 200…» bị cắt; ghi vào mục 6.

### 3.6 Kiểm lại lưới Sở thích ở font 1.3 sau khi sửa (`sang-1.3-sau-sua/`, bảng XANH)

Commit `96bfcaa5`: `minItemWidth` nhân theo `fontScale`. Ở 1.3 lưới thành một cột, tám nhãn nguyên
chữ, màn cuộn tới CTA; ở 1.0 vẫn hai cột (ảnh `sang-1.0/`).

## 4. Cổng đã chạy (số thật, không phải dấu xanh)

| Cổng | Kết quả |
|---|---|
| `npx tsc --noEmit` | 0 lỗi (sau mỗi lát) |
| `npm test` (build:check web + node) | 738/738 pass ba lần (lần đầu 737/738 vì `mac-dinh-am-tham-id` bắt `laCanhId(id) ? id : …` → đã sửa); lần 4 (`6069ac97`) và lần 5 trên HEAD `8c1d8600`: 738/738, exit 0 |
| `python3 -m pytest services/api/tests/web tests -q` | 929 passed, 22 skipped (contrast floor, meta-test đọc cây mobile) |
| `tests/art-duong.test.mjs` | 6/6 |
| `rudi-khong-hex`, `dau-gach-dai`, `gate`, `test-gate-discovery`, `duong-svg`, `rudi-chat-sticker`, `vong-import`, `rudi-ky-niem` (+2 ca ngày VN) | pass |
| Detector Impeccable (`imp detect --json`) | `[]` trên `ui/art`, `art/`, 6 file màn, 10 file chữ |
| `tools/cap-mau-tinh.mjs` 11 file | 0 cặp hỏng, 68 chỗ không đọc được (vùng mù đã biết) |
| Repo guard staged | pass cả ba commit |
| Mini-bảng native | trước finish review: sáng/1.0 · tối/1.3 · A/B · sáng/1.3 đều XANH; sau loạt sửa: sáng/1.0 (3 flow) · A/B (2 flow) · sáng/1.3 (2 flow) đều XANH; mỗi lượt NEO 2b cắn và canary đỏ đúng thiết kế |

## 5. Ba câu tự kiểm của §14.4

1. **Bỏ mascot còn nhận ra app không?** Có ở ba màn: Sở thích không dùng Nếp (tám đồ vật là chủ thể),
   Khám phá nhận ra nhờ nhịp lead/so sánh + khung vẽ theo loại, Album nhờ ảnh in + nhịp ngày. Năm cảnh
   rỗng đứng được khi `nep: false` (test khẳng định cảnh không Nếp là phần đầu của cảnh có Nếp).
2. **Thiếu ảnh composition còn tốt không?** Khung không ảnh vẽ đồ vật theo loại (thay icon trong đĩa);
   cảnh rỗng là vector. Chưa có ảnh native của trường hợp catalogue không ảnh (fixture luôn có ảnh).
3. **Bỏ lời giải thích action/state còn rõ không?** Sở thích: đếm «Chọn ít nhất 3» + CTA mờ nói lý do;
   Check-in: con dấu; Lịch trình: badge «Nháp». Chưa có người dùng thử — đây là đọc của người làm.

## 6. Chưa làm, và vì sao

- **Đợt 3** (§13.1 C2/D1): huy hiệu 6 silhouette (hai hệ: `Profile.tsx` fixture và `AchievementsLive`),
  khay Tạo bốn vật thể, Check-in cảnh chừa chỗ, 41 bề mặt rỗng/lỗi còn lại, 8 sticker vẽ lại thành
  Nếp — chờ bạn xem ảnh đợt này, đúng khuyến nghị §13.1.
- **«Dựng thước phim» → «Tạo câu chuyện ảnh»**: flow 32 ghim nút, hai sub-flow và unit test ghim câu
  `cauThuocPhim` («Rủ Đi AI dựng thước phim này…»); đổi phải đi cùng nhau trong một lát riêng.
- **Ảnh fixture sai ngữ cảnh** (§4.4, §7.5): gốc là `fixtures.ts` chỉ có 5 ảnh; thêm ảnh là binary
  phải ghim sha256 vào `.repo-guard-allowlist.json` và phải có nguồn/giấy phép (ADR-0017). Đợt này đi
  đường vector; ảnh cần một quyết định biên tập riêng.
- **Chọn ảnh album trên live**: live không có tính năng chọn/chia sẻ (không backend) — không bịa.
- **Nợ nhỏ có tên:** cặp màu nút disabled của kit `RudiButton` (trắng trên hồng ~2:1, ngoài phạm vi;
  reviewer xếp là nợ, không phải mục mở); dòng dữ kiện của `PlaceRow` ở font 1.3 cắt đơn vị giá
  (`numberOfLines 1`, có từ trước); tiêu đề chặng fixture chứa tên quán nên dòng phụ lặp tên.
- **Chưa đo:** iOS, TalkBack/VoiceOver, hiệu năng release, hai màn live trên stack OTP, bảng đầy đủ 47
  flow (đợt này chạy mini-bảng chụp ảnh; bảng gốc `make mobile-native` nên chạy trước khi mở PR).

## 7. Finish review (Impeccable, `impeccable-finish-reviewer` trong context mới)

**Lượt 1 — `fix`**, tám mục theo thứ tự (không có mục nào là «đẹp hơn»): (1) chip lọc Khám phá vẫn
dùng icon hệ thống trong khi Sở thích đã vẽ bằng bút Gu; (2) `nightlife`, `outdoor` không đọc được nếu
bỏ nhãn; (3) Nếp còn dáng icon tài liệu (vạt gấp đặt ở góc dưới thay vì chạy chéo qua thân), cười mở
rộng, `vui` có vạch động; (4) cặp so sánh thiếu dữ kiện, tim rơi ~80dp, hàng có con dấu cao hơn hàng
khác; (5) lead thiếu câu lý do có căn cứ; (6) câu hỏi ngân sách ngang hàng với helper, nút disabled
trắng-trên-hồng ~2:1; (7) dòng xuất xứ album lặp «Đà Lạt»; (8) cảnh rỗng không Nếp thụt lề ~110dp.
Reviewer cũng ghi: ba silhouette có nhưng hai trong ba đã có từ trước, mắt xích họ hàng là chip; motif
chưa xuất hiện ở màn có dữ liệu; câu chia sẻ vị trí ở Check-in vẫn ba câu ngôn ngữ người xây.

**Loạt sửa (`a97cb7ce` + `6069ac97`)**, một lượt, chụp lại cùng viewport (thư mục `sua-sang-1.0/`,
`sua-ab/`, `sua-sang-1.3/`): chip lọc live + fixture vẽ bằng `GuGlyph` 22 qua prop mới `Chip.leading`;
đèn treo + chùm sáng + cuống vé / lối mòn + tán thông; vạt gấp chạy chéo từ mép trên-trái, góc coral
lớn hơn, cười khép, bỏ vạch động; cặp so sánh in đủ điểm · khoảng cách · giá, tim lên góc ảnh, con
dấu hàng đứng cạnh tên; lead có «Hợp gu nhờ …» (live: `match.reason` khi thật; fixture: hai tag đã
khớp; không có thì không dòng); câu hỏi ngân sách ở cỡ tiêu đề + ghi chú đơn vị; ảnh in album chỉ
mang nhóm; cảnh không Nếp đóng khung sát đồ vật cùng tỉ lệ; câu vị trí Check-in còn một câu.
**Không sửa:** cặp màu nút disabled của kit (`RudiButton`, dùng chung toàn app) — nợ riêng.

**Lượt 2 — chấm từng mục:** 7/8 resolved (1, 3, 4, 5, 6a, 7, 8); 2 partial (`nightlife` đọc như
compa); 6b unresolved, ngoài phạm vi; hai hồi quy do loạt sửa: dải giá bị ngắt trong ô so sánh
(«40K - 80K/ người»), chip «Quán ăn» mượn nồi của «Món local».

**Loạt sửa hai (`8c1d8600`):** `nightlife` = dây đèn võng ba bóng, bóng giữa coral; ô so sánh in dải
giá trên dòng riêng; `guTheoLoai("quan-an-local") → "an-uong"`. Chụp lại flow 50 sáng/1.0
(`sua2-sang-1.0/`).

**Lượt 3 — `ship`.** Ba mục còn lại resolved, không hồi quy mới. Phạm vi phán quyết (lời reviewer):
màn fixture Sở thích / Khám phá / Album / cảnh tìm không ra ở sáng 1.0 và Sở thích ở 1.3; **không
gồm** cửa live, tối sau loạt sửa (bổ sung ở 3.5), và cặp màu nút disabled của kit (nợ có tên cho đợt sau).

## 7b. DESIGN.md sau khi ship

`impeccable-documenter` (context mới) ghi DESIGN.md từ bản đã ship: mốc đợt (L257–271), **Luật Vai Màu
Của Nét Vẽ**, vai chữ `note` + **Luật Nhãn Đậm, Câu Nhẹ**, quy tắc lưới × fontScale, `Chip.leading`,
KhungAnh/album theo ngày, mục mới **Lớp vẽ (`art/`)** (ngữ pháp đường, ba lưới, Nếp hai bản đọc, tám
hình gu, ba motif, năm cảnh với câu đọc, **Luật Nếp Đứng Xa Tiền**, **Luật Vòng Hở Không Tiến Độ**),
Hàng địa điểm (`PlaceLead` + `lyDo`, `PlaceCompare`, `PlaceRow`, `PlaceGlyph`), **Luật Nói Một Lần** ở
mục copy, Do/Don't mới, dòng cổng `art-duong.test.mjs`. Khối token/contrast sinh máy giữ nguyên byte;
`.impeccable/design.json` thêm `shippedV5`; contrast floor 54 passed sau khi ghi. Nợ (nút disabled
của kit, cảnh/pose chưa nối, lưới fixture album) ghi là trạng thái build, không phải luật.

## 8. Tái lập

```bash
cd apps/mobile && node --test tests/art-duong.test.mjs
cd apps/mobile && rm -rf dist-test && npm test
python3 -m pytest services/api/tests/web tests -q
ANDROID_ADB_SERVER_PORT=5038 ANDROID_SERIAL=emulator-5554 scripts/mobile_native.sh --flows .maestro-bs
```
