# Chuyển mình UI Rủ Đi: bằng chứng và bàn giao

- Nhánh: `claude/p0-w-ui2-bo-cuc-theo-nhiem-vu`, gốc từ `bd32e4d` (bàn giao của Codex).
- Commit: `ab53e789` (đợt 3–4) · `7ae05b08` (đợt 5) · `20de622c` (đợt 6) · `e2c8b1da` (đợt 7) · `cc5c6795` (lô sửa sau vòng soi native) · `e550c058` (lô sửa theo finish review) · `06a4722f` («Tìm hiểu thêm» thành liên kết) · `9997e835` (tài liệu) · `1ea76577` (nhãn huy hiệu) · `4570fdd3`, `f838ca45` (đợt 8).
- Diff so với gốc: 56 file, toàn bộ trong `apps/mobile/` (ranh giới sở hữu của Claude).
- Quy trình: skill `impeccable-pipeline` (preflight → hợp đồng hướng trong `app/_layout.tsx` → dựng theo đợt → một vòng soi native có giới hạn → một lô sửa → finish reviewer ở ngữ cảnh mới → documenter).
- Ngày: 2026-09-06.

## 1. Nguyên nhân gốc và cách giải

Báo cáo `docs/codex/2026-09-06/bao-cao-chuyen-minh-ui-rudi.md` chẩn đoán đúng: bộ da v2 (màu, chữ, con dấu) đã có, nhưng **bố cục vẫn là dashboard v1**: thẻ lồng thẻ, ô thống kê, banner AI, và số liệu màn tự bịa. Người dùng nhìn thấy một bảng điều khiển chứ không thấy việc mình cần làm. Cách giải, áp cho từng màn:

| Triệu chứng trong báo cáo | Cách giải đã ship |
|---|---|
| Thẻ lồng thẻ, mọi thứ đều là card | Hàng phẳng có hairline (`ListRow`, `HangDiaDiem.PlaceRow`, `HangChang`), thẻ chỉ còn ở nơi cần bao một đối tượng (ảnh dẫn, sheet) |
| Số liệu tự bịa (XP, huy hiệu, video, bản đồ) | Bỏ hẳn; cái gì có thật thì đếm thật (`keo/nhip-keo.ts` đếm ngày tới kèo từ ngày thật, con dấu «CÒN N NGÀY» / «HÔM NAY» / «ĐÃ QUA») |
| Banner AI ở mọi màn | AI là một nút tím cạnh việc đang làm (ô tìm, lịch trình), không phải banner đầu trang |
| Tiền lẫn với chữ thường | `Money` tabular, một tông teal cho tiền, không co chữ (`Stat` bỏ `adjustsFontSizeToFit`) |
| Trạng thái là chip màu | `Stamp` (con dấu mực) cho trạng thái đã đúng, viền chì đứt nét cho bản nháp |
| Ảnh stock không nguồn | `MediaSlot` có dòng provenance, khung rỗng vẽ glyph danh mục, không ảnh giả |
| Mỗi màn một tông rực | Một tông dẫn mỗi màn: coral = hành động, teal = tiền, tím = AI; bìa indigo cho màn thuyết phục, giấy ấm cho màn thao tác |

## 2. Kit và thành phần

- Mới: `ui/useKeyboardOpen.ts`, `ui/KhungAnh.tsx` (bản in trong khung giấy, dòng xuất xứ «ai · ở đâu · khi nào»), `ui/DongTien.tsx` (một dòng sổ: tên, tiền, hairline), `duongVienDau` trong `ui/duong-svg.ts` (vành con dấu mép gãy, có test ngữ pháp), `screens/explore/HangDiaDiem.tsx` (ảnh dẫn + hàng địa điểm), `screens/keo/HangChang.tsx` (hàng dòng thời gian, đường mực liền / chì đứt nét), `keo/nhip-keo.ts` (+ test `tests/rudi-nhip-keo.test.mjs`).
- Sửa: `RudiScreen` có `overlay` (sheet/viewer nằm ngoài KeyboardAvoidingView) và `keepEnd` (chat cuộn tới cuối), `Sheet` có `maxHeight` và đóng bằng state React sau khi animation xong, `StampButton` là con dấu thật (rộng theo chữ, nghiêng, vành SVG mép gãy, không mũi tên, hai cỡ), `AiNote` là ghi chú lề tím có hairline (không fill, không nhãn trên câu), `CoverBand compact`, `RosterPicker` có `tone` và `nhanCho`, `Photo` có `ratio`, `ResponsiveRow`/`gridFor` có `maxColumns` và làm tròn dp nguyên. Fixture album còn bốn ảnh thật (không lặp nguồn).
- Màn viết lại (37 file dưới `src/rudi/screens/`): Welcome, Login, Otp, Onboarding, Discovery/ExploreLive, PlaceDetailLive, DiemDen, PlanLive, PickOutingLive, CreateOutingLive, OutingLive, Outing, TheAi, GroupChatLive, Group, Conversations, Members, Invite, New, Empty, Friends, AddFriend, HangNguoi, LoiMoi, ChiaBillLive, DotThuLive, Bill, GroupWallLive, AlbumLive, ShareMomentLive, AchievementsLive, HoSoSong, HoSoNguoi, DangBai, Profile, Memories.

## 3. Maestro: ba flow đổi có chủ đích

Chuỗi ghim của mọi flow khác giữ nguyên. Ba flow đổi vì câu chữ trên màn đổi có lý do:

- `01-welcome-auth.yaml`: nút Google **vắng** khi thiếu client id (thay vì hiện rồi báo chưa cấu hình) → `assertNotVisible: "Tiếp tục với Google"`.
- `10-gan-mon-chay-sang-quyet-toan.yaml`: dòng món gấp lại mặc định, bấm dòng để mở, bấm **tên người** (không còn chữ cái đầu) → `tapOn: "Bò nướng"` rồi `"Minh Anh · Bò nướng"`.
- `36-so-thich.yaml`: nút gọi đúng việc nó làm → `tapOn: "Lưu sở thích"`.

## 4. Bằng chứng

| Cổng | Kết quả | Ghi chú |
|---|---|---|
| `npm test` (build:check web + tsc test + node --test) | 676/676 xanh, exit 0 | chạy ở cây sạch `dist-test` sau lô sửa cuối (`e550c058`) |
| `tsc --noEmit -p tsconfig.json` | sạch | sau mỗi đợt và sau mỗi lô sửa |
| Detector Impeccable (`imp detect --json`, quét nguồn) | `[]` trên mọi file đổi | quét nguồn không đo được tương phản trên nền ảnh; phần đó soi bằng mắt trên ảnh chụp native |
| Repo guard | `range bd32e4d..06a4722f` qua từng commit | dòng lỗi số điện thoại ở Login phải viết lại vì luật vn-phone |
| Vòng soi native trên AVD `rudi-qa3` (1080×2400 @420) | 22 màn fixture sáng/1.0 · 9 màn tối/1.3 · 4 màn tablet 1800×2400 @320 · 1 màn sáng/1.3 | mỗi ảnh được `uiautomator dump` xác nhận đúng màn trước khi chụp; animation scale 0 để không có phần tử ẩn vì thời điểm |
| Lô sửa sau soi (`cc5c6795`) | đo lại bằng uiautomator: lưới album 3 cột điện thoại / 6 cột tablet, «07:00» một dòng, «tháng 10» một dòng ở 1.3, không còn cảnh báo Reanimated | |
| Lô sửa theo finish review (`e550c058`) | chụp lại 12 màn sáng + 4 màn tối 1.3 + 2 màn tablet; xem mục 4b | |
| Bảng Maestro fixture (`scripts/mobile_native.sh`, Metro riêng cổng 8097, `emulator-5560`) | **XANH** lượt 23:16 (dấu vân `b74b8c10`): 10 flow qua, NEO 2b cắn, canary 09 đỏ đúng ở bước cuối | Trước đó ba lượt đỏ vì: harness không truyền `--device` cho maestro (lái nhầm máy phiên khác, đã sửa ở `a395c7e9`); flow vào app chung còn pin «Tạo không gian của tôi» của màn Sở thích cũ; ba flow và canary pin câu tổng của Tài chính/Quyết toán trước khi thành sổ phẳng. Mọi pin đã cập nhật cùng nhánh; `tests/test_maestro_flows_dev_client.py` 9 passed |
| Finish reviewer Impeccable (ngữ cảnh mới, không thừa kế thread dựng) | vòng 1: `fix`, 8 điểm material; vòng chấm lại: xem mục 4b | |
| DESIGN.md / `.impeccable/design.json` | documenter (agent `impeccable-documenter`, ngữ cảnh mới) viết lại từ artifact đã ship: +698/−309 dòng; không hex nào ngoài tokens.json/theme.ts (kiểm bằng script); ghi rõ phần suy từ ảnh và phần suy từ mã | nợ kit documenter nêu: `Eyebrow`, `SurfaceLabel` vẫn export trong `ui.tsx`, `Stat`, `FloatingGlass`, một `ProgressBar` ở kết quả bình chọn (`Group.tsx`) được ghi là ngoại lệ, không phải thành phần |
| `pytest tests` gốc repo (cổng đọc cây client: header actor, route gọi máy chủ, màn tới được, Maestro flow) + `services/api/tests/api/test_person_identity.py` | 932 passed, 22 skipped, **1 đỏ sẵn**: `tests/test_server_routes_called_gate.py::TheMechanismIsLoadBearing::test_without_the_debt_file_the_real_tree_is_red` đỏ y hệt ở commit gốc `bd32e4d` (chạy trong worktree tạm), không do nhánh này | hai ca `test_actor_header_contract.py` từng đỏ vì cổng đọc nhãn `${…}/${…} huy hiệu đã mở` ở `Profile.tsx` như một URL không phân giải được; đã dời nhãn ra hàm `nhanHuyHieuDaMo()` ngoài component gọi API → 18 passed. Toàn bộ `services/api/tests` (domain/db) không đọc cây mobile và không nằm trong diff; chưa chạy trọn trong lượt này vì ~45 phút |
| `pytest services/api/tests/web` (test backend đọc cây mobile) | 43 passed sau khi cập nhật `test_contrast_floor.py::test_assignment_summary_caption_uses_readable_semantic_text` | ca này ghim câu chú thích cũ «6 khoản · tổng không đổi khi bạn sửa người» trên khối `splitSoft`; màn gán món giờ là `Heading` trên giấy (đợt 6), nên ca đổi sang ghim phụ đề Heading (`inkSoft`) trên `ground`. Đây là file trong `services/api/tests/` nhưng chỉ đọc `apps/mobile`; reviewer PR xem lại nếu muốn giữ nguyên chủ quyền |

Ảnh chụp nằm ngoài Git (repo guard từ chối binary): `.impeccable/review/chuyen-minh/*.png` (36 ảnh, thư mục bị ignore) và scratchpad phiên. Muốn xem thì mở thư mục đó trên máy này; muốn giữ lâu thì chép ra `~/rudi-video` như các đợt trước.

## 4b. Finish review: tám điểm material và cách đóng

Reviewer (agent `impeccable-finish-reviewer`, ngữ cảnh mới, chỉ đọc ảnh và mã) trả **`fix`** với tám điểm; bảng fidelity: THESIS/OWN-WORLD/STORY/FIRST VIEWPORT «partial», FORM «met». Lô `e550c058` đóng cả tám:

| # | Reviewer thấy (ảnh) | Đã đổi |
|---|---|---|
| 1 | CTA «Rủ Đi thôi!» là nút pill cam full-width có mũi tên, nghiêng không thấy: đúng cái THESIS từ chối | `StampButton` viết lại: rộng theo chữ, -3° trên bìa, -1° ở Login, vành SVG mép gãy hai tần số (`duongVienDau`), không mũi tên |
| 2 | Chat: ô soạn nổi giữa màn, ~200px giấy trống trước thanh tab, tin cuối bị che | `footerInset` 92→12 (tab navigator đã chừa thanh tab), `RudiScreen keepEnd` |
| 3 | Footer Lịch trình AI nằm dưới vạch gesture | hai nút thành `footer` với `insets.bottom` |
| 4 | Album sáng còn 2 cột (ảnh cũ trước `cc5c6795`), tablet 6 cột + 1 ô lẻ, 4 ảnh nguồn lặp hai lần đọc là giả | chụp lại đúng 3 cột; fixture còn 4 ảnh thật, hết ô lẻ |
| 5 | Ảnh kỷ niệm không có khung Instax + dòng xuất xứ | `KhungAnh` cho ảnh dẫn album (fixture + live), bài tường (fixture + live), màn chia sẻ |
| 6 | Hero-metric ở Quyết toán và Tài chính (khối teal, số to, ô stat, thanh tiến độ) | `DongTien`: sổ phẳng, teal chỉ trên số và dấu, bỏ ProgressBar |
| 7 | Kicker trên heading trong thẻ AI (chat, TheAi, ghi chú AI ở địa điểm) | ký ở chân thẻ; `AiNote` thành ghi chú lề có hairline |
| 8 | Số bịa ở Thành tích/Hồ sơ («12 huy hiệu», «Hoàn thành 10 chuyến» khi hồ sơ nói 1 chuyến) | huy hiệu đếm từ số thật của fixture: 0/6, tiến độ từng dòng |

Vòng chấm lại (verdict pass) trên ảnh mới: 7/8 **resolved**, fix 2 **partial** chỉ vì ảnh chat tối/1.3 lúc đó còn là ảnh cũ (lần chụp lại thất bại: tôi chờ chữ «Team Đà Lạt» ở TopBar đã cuộn khỏi khung). Reviewer nêu hai regression do lô sửa:

- Lịch trình AI: dòng «2.500.000đ» bị footer đè ở nếp gấp. Đo lại khi cuộn hết bằng uiautomator: dòng tiền [42,1960][1038,2031], nút «Dùng plan này» [716,2233][951,2283]: đọc trọn, không đè (ảnh `phone-light-09-itinerary-end.png`).
- Bìa: «Tìm hiểu thêm» full-width viền sáng lấn thứ bậc con dấu → `CoverButton variant="link"` (không viền, rộng theo chữ), commit sau `e550c058`.

Chat tối/1.3 đã chụp lại thật (7:44): ô soạn dính trên thanh tab, thẻ AI trọn. Phán quyết vòng cuối: **`ship`** (ba mục còn mở đều resolved, không regression mới trên ảnh đã đọc). Phạm vi của chữ «ship» này: 8 fix đã chấm + 3 mục còn mở, trên ảnh native của màn fixture. Reviewer nói rõ nó **không** phủ các màn live (chưa có ảnh native) và các gợi ý taste vẫn mở.

Gợi ý taste reviewer nêu nhưng chưa làm (không chặn): icon app gradient lặp ở header Explore/Plan/Chat; dòng dấu vân cây trên bìa là neo của harness native (`EXPO_PUBLIC_TREE_FINGERPRINT`, flow 00 assert) nên giữ; dấu «HỢP GU» tím trên vùng vàng của ảnh dẫn hơi chìm; chip «Dưới 250K» mồ côi dòng hai ở màn AI match.

## 4c. Đợt 8: hoàn thiện toàn hệ (commit `4570fdd3`, `f838ca45`)

Theo báo cáo §5.2 (ba chữ ký), §8.22 (khay Tạo mới), §9 (motion) và kế hoạch đợt 8:

| Việc | Đã làm | Bằng chứng |
|---|---|---|
| Con dấu đóng xuống khi trạng thái thành thật (chữ ký FORM của hợp đồng) | `Stamp dong`: một nhịp celebrate 550 ms, scale 1.35→1, mực nhạt→đậm, haptic thành công; chỉ hàng vừa tác động, không phát lại khi mở lại; Reduce Motion → dấu tĩnh. Gắn ở quyết toán fixture («Đã trả»), check-in fixture («Đã tới»), đợt thu live (sau máy chủ xác nhận) | clip screenrecord `dot8/clip-dau-tra.mp4` + 3 khung hình 0.1/0.3/0.7 s |
| Phản hồi nhấn trên UI thread | RudiButton/IconButton/Chip/ListRow dùng `PressScale` (spring) thay opacity đổi ngay | đọc mã |
| Dòng thời gian có nhịp (§5.2 B) | chặng có địa điểm mang ảnh 48dp (`AnhChang`), chặng khác là dòng gọn; Lên plan và Lịch trình AI | `dot8/phone-light-07-plan`, `-09-itinerary`, `phone-dark-font13-plan` |
| Dấu trên ảnh có nền giấy; header tab chỉ wordmark; bộ lọc AI match cuộn ngang | gợi ý taste của reviewer vòng trước | `dot8/phone-light-04-explore`, `-05-ai-match`, `-06-place`, `-10-chat` |
| Tay cầm sheet kéo được thật (§8.22) | `Sheet`: Gesture.Pan trên hàng tay cầm, thả quá 90dp hoặc vẩy thì đóng, thả ngắn bật về; `onClosed` cho route chủ; khay Tạo mới dùng Sheet của kit trong route trong suốt chỉ fade | `dot8/phone-light-12-create-sheet`, `clip-keo-sheet.mp4` + 3 khung hình; Back đóng khay (uiautomator: 0 node sau Back) |

Reviewer đợt 8 (vòng 1) trả **`fix`** hai điểm: con dấu còn là zoom-in một pha giống entrance mặc định; clip sheet quay từ `/create` lạnh nên nền là ô xám. Lô sửa (commit sau `f838ca45`): con dấu ba nhịp (lao 130 ms → chạm: mực đậm 60 ms + haptic tại lúc chạm → lún–bật spring), `/create` tới lạnh đặt vỏ tab bên dưới rồi mở lại khay, clip sheet quay lại từ FAB (Explore mờ bên dưới, không fade lại). Ba gợi ý cũng làm: scrim nhạt theo tay kéo, ảnh chặng có viền giấy, `RudiScreen header` giữ tên nhóm và chuyến ghim trên luồng chat. Vòng chấm lại: **`ship`** (hai fix resolved; ca `/create` tới lạnh chỉ có mã, chưa có ảnh trên máy vì dev client không tái hiện được deep link vào tiến trình chưa nạp bundle; cần một lần `am start -d rudi://create` trên release build). Hai điểm craft nhỏ reviewer nêu đã sửa ngay sau: nhịp lún của con dấu tách ra shared value riêng (trước đó đi qua nhánh của nhịp lao nên nở 1 % thay vì lún), hairline dưới `RudiScreen header` để bong bóng chat cuộn dưới header đọc là giấy dưới kẻ chứ không phải lỗi vẽ.

Chưa làm trong đợt 8 (cần API sống hoặc quyết định riêng): skeleton→nội dung fade trên màn live, ma trận trạng thái §10.2 cho từng màn live, tablet hai cột, iOS/TalkBack, vòng tròn «Chưa tới» vẽ bằng View cùng nét với node timeline (gợi ý taste).

## 4d. Bảng Maestro live OTP (API `e2e_slice --keep` + `make demo-rudi`, `rudi-qa3`)

Đây là vòng soi native đầu tiên cho các màn live (report §8.23, kế hoạch đợt 0/1). Ba lượt trong đêm 06→07/09:

| Lượt | Dấu vân | Kết quả | Việc sửa sau lượt |
|---|---|---|---|
| 1 (23:18) | `b74b8c10` | 3/15 xanh | flow ghim theo màn mới (22/26 cuộn tới địa điểm ghim vì Khám phá sống dẫn bằng một địa điểm lớn; 24 «… · 1 nhóm · …»; 27; 35 cuộn); flow 36 cần số CHƯA đăng nhập → harness cấp `OTP_PHONE_E`; **hai lỗi UI thật**: nút bước của Chia bill live bị editor món đẩy dưới nếp gấp → nút chính từng bước thành `footer`; chat live: tin mới nhất bị ô soạn che khi bàn phím mở → danh sách bám đầu mới khi có hàng mới/bàn phím mở (chỉ khi đang ở cuối) |
| 2 (23:50) | `8e4b3267` | 9/15 xanh | 27 ghim nhầm cả hai chỗ (chi tiết «1 chặng», danh sách «một người · 1 chặng»); 26 khẳng định số nơi trước khi cuộn; chat: thông báo «AI chưa nối được mô hình» là header của danh sách đảo nên `maintainVisibleContentPosition` để nó dưới ô soạn → kéo về đầu khi thông báo đổi; 28 và 32 đỏ vì lượt vắt qua nửa đêm (kèo lập 06/09 thành «đã kết thúc», check-in 00:0x ngoài ngày kèo); 35 đỏ vì `/destinations` của stack chỉ có hai nơi có địa điểm (điều kiện dữ liệu máy chủ) |
| 3 (00:16) | `f5e4ac0b` | 12/15 xanh | 26 đỏ ở `tapOn "Cafe"` (chip lọc nằm ngoài màn sau ảnh dẫn → cuộn tới chip trước khi bấm); 30 và 35 như lượt 2 |
| 4 (00:42) | `4eb70163` | 13/15 xanh | 30 vẫn đỏ (xem 4f); 35 là điều kiện dữ liệu máy chủ (stack chỉ có hai điểm đến có địa điểm, flow ghim «Hội An») — không phải lỗi màn |

Ảnh `takeScreenshot` của từng flow live nằm ở `.impeccable/review/native/<lượt>/` (ngoài Git): đây là bằng chứng native đầu tiên của các màn live sau đợt chuyển mình.

## 4e. Đặt lane frontend lên `origin/main`

`origin/main` đã nhận M12 (PR #570–#572: ảnh địa điểm có giấy phép, ảnh nhóm, «nên làm gì ở đây») sau khi nhánh Codex `bd32e4d` tách ra. Nhánh Codex mang ba commit backend (`c29c54f4` phiên bản lịch trình, `2fb215f8` kho ảnh địa điểm, `5d853f44` nhập ảnh Commons) **chưa lên main** và va thẳng với M12: `place_photos` định nghĩa hai lần trong `models.py`, hai migration cùng id `c9d0e1f2a3b4`; `e2e_slice` dựng từ cây merge chết ngay ở alembic. Backend là lane Codex nên tôi không hoà giải hộ.

Cách làm: merge thử để lấy bản hoà giải của các file lane mình (cấy M12 vào bố cục mới: `anhBiaThe` cho thẻ và ảnh dẫn, dải ảnh có credit và ảnh nhóm ở màn địa điểm, «nên làm gì», đăng ảnh gắn địa điểm, `MediaSlot prefix` + dòng «Chưa tải được ảnh»), lưu ra ngoài, huỷ merge, tạo nhánh mới từ `origin/main` và chép đúng danh sách file lane frontend (bỏ `services/api` trừ `app/web` và `tests/web`). Kết quả: `claude/p0-w-ui2-chuyen-minh-tren-main`, một commit `73b13add`; lịch sử từng đợt vẫn ở nhánh cũ.

Lưu ý cho client khi backend của Codex chưa lên main: `timeline_revision`/`expected_revision` (khoá ghi lịch trình) do nền UI của Codex thêm vào client; API main không có trường này, client chỉ gửi `expected_revision` khi có giá trị nên không vướng `extra=forbid`, và không có xung đột giả vì hai phía cùng `undefined`. Khi Codex land phiên bản lịch trình, client đã sẵn.

Kiểm trên nhánh mới: tsc sạch, detector `[]` trên `src/rudi`, npm test 688/688, cổng pytest gốc repo + web chỉ còn ca đỏ sẵn `test_without_the_debt_file_the_real_tree_is_red`, repo guard staged qua. Bảng trên nhánh mới, cùng máy `rudi-qa3` (emulator-5560), dấu vân `73b13add`:

| Bảng | Giờ | Kết quả |
|---|---|---|
| Fixture (10 flow) lần 7, sau `pm clear` | 01:25 | **XANH**, NEO 2b cắn, canary đỏ đúng bước cuối |
| Live OTP (15 flow, API 47789 từ `e2e_slice --keep` + `make demo-rudi`) lần 5 | 01:45 | 14/15 xanh — **flow 30 xanh** sau sửa ở 4f (ảnh `30-ai-im-lang-that.png`: bong bóng trọn, thẻ «Rủ Đi AI chưa nối được mô hình» ngay trên ô soạn); DB stack xác nhận nhóm «Hoi QA» có 5 tin chữ, 1 thẻ bình chọn, 1 ảnh, 1 ❤; 35 đỏ vì stack seed chỉ có hai điểm đến |
| Mini-bảng 24→25→26→35 (D phải có nhóm trước 35) sau khi thêm hàng `d-hoi-an` (từ `destinations_vn`) vào DB dùng một lần | 02:12 | **4/4 xanh**; máy chủ xác nhận «3 điểm đến, Hội An có trong danh sách và trả đúng 0 địa điểm»; canary OTP không chạy vì thư mục mini thiếu flow 22 (lỗi dựng mini-bảng, không phải app) — lượt live đầy đủ lần 6 chạy lại trên commit này |

Flow 38 (`--anh`) không chạy: xem 4f.

Bẫy gặp trong bước này (đã ghi memory): bảng fixture chạy ngay sau bảng live thì phiên live còn trên máy, flow 00 thấy Explore sống thay vì bìa → `pm clear` trước; `git stash` ở repo dùng chung lấy nhầm stash của phiên khác → chép file ra ngoài thay vì stash.

## 4f. Flow 30: vì sao câu «Rủ Đi AI chưa nối được mô hình» không hiện, và cách chữa

Bốn lượt live đỏ cùng một chỗ. Ảnh lúc đỏ (`30-chat-that-FAILED.png`): bong bóng vừa gửi bị ô soạn che nửa dưới, câu thông báo (header của danh sách đảo, tức nằm dưới bong bóng) ngoài màn. Máy chủ không có lỗi: tái hiện bằng HTTP trên chính API 47789 (số mới, nhóm mới, `POST /contexts/{id}/messages` với «@Ru Di goi y quan an») trả `intent: mention`, `companion: {spoke: false, reason: "unavailable"}` — đúng thứ `cauYDinh` cần để in câu ấy. Vậy `thongBao` có được đặt; danh sách chỉ không về offset 0.

Gốc rễ nằm ở tổ hợp `maintainVisibleContentPosition` với chính các phép «bám cuối» thêm ở lượt 1–2:

- Native (`MaintainVisibleScrollPositionHelper.kt`, RN 0.86): mỗi lần nội dung đổi, helper giữ ô đầu tiên còn thấy rồi `reactSmoothScrollTo(0)` khi trước đó ở gần đầu; hai lần đổi liền nhau (hàng mới, rồi header đổi từ «Đang gửi» sang thông báo) biến cuộn mượt thứ nhất thành một cú fling dở dang qua `scrollToPreservingMomentum`, dừng ở vị trí lưng chừng.
- JS (`VirtualizedList.getDerivedStateFromProps`): có prop này thì hàng mới ở đầu KHÔNG được render cho tới khi một sự kiện cuộn quay lại (`pendingScrollUpdateCount`).
- `scrollEventThrottle={64}` trên Android là **bỏ** sự kiện trong cửa sổ, không phải dồn; sự kiện cuối «đã về 0» bị bỏ nên `ganCuoi` kẹt ở `false`, và mọi nhánh bám cuối (`onContentSizeChange`, effect `thongBao`, bàn phím) đều không chạy.

Chữa (commit sau `73b13add`): neo vị trí **chỉ khi đang đọc lịch sử** (`maintainVisibleContentPosition={oCuoi ? undefined : {minIndexForVisible: 0}}`) — ở cuối, danh sách đảo có offset 0 *là* đầu mới, hàng mới và header rơi vào tầm nhìn mà không cần cuộn; `scrollEventThrottle={16}` (dưới ngưỡng 17 ms nên Android không bỏ sự kiện) cùng `onMomentumScrollEnd`/`onScrollEndDrag` ghi lại vị trí; sau khi gửi thì đặt cờ «ở cuối» rồi nhảy về 0 không animation trước khi hàng được commit. Kết quả đo ở bảng live lần 5 (mục 4e).

Cũng trong commit này: `PlaceRow` in dòng ghi công dưới ảnh nhỏ (ADR-0017 §2.5 — ảnh có giấy phép không xuất hiện ở đâu mà không nêu tác giả), và `hienThiDiaDiem` gắn tiền tố «Ảnh quanh đây: » vào credit của ảnh dẫn (cùng `TIEN_TO_ANH` với màn chi tiết, đúng điều flow 38 ghim). Flow 38 không chạy được trên stack demo: 8 địa điểm seed là dữ liệu bịa, importer main từ chối gắn ảnh thật (`1c52531b`), nên bảng chạy không `--anh`.

## 5. Chưa kiểm, còn nợ

- **Màn live chưa chụp native.** ExploreLive, PlanLive, ChiaBillLive, GroupChatLive, AlbumLive, HoSoSong… dùng chung thành phần với màn fixture và qua typecheck + test, nhưng cần API + thế giới seed (`make demo-rudi`, `make mobile-native-live`) để soi trên máy. Chưa làm trong lượt này.
- **Màn OTP** chưa chụp (cần cửa OTP thật).
- **iOS, TalkBack/VoiceOver** chưa đo. Nhãn a11y đã đặt trên mọi nút mới (`Mở …`, `Lưu …`, `Chọn ảnh N`) nhưng chưa có ai nghe thử.
- **Tablet hai cột** (`twoPane` trong `adaptive.ts`) chưa màn nào dùng; tablet hiện là một cột rộng có rail, ảnh dẫn giãn 21:9.
- **Google sign-in** vắng cho tới khi có client id (quyết định ở checkpoint `2490e67` của Codex).

## 6. Bẫy đã gặp trong lượt này (để người sau khỏi mất giờ)

- **Metro chạy với `CI=1` thì tắt watch mode.** Sửa file xong máy vẫn chạy bundle cũ; dấu hiệu là dòng `Bundled … (1 module)` sau khi sửa nhiều file. Khởi động Metro để soi thì bỏ `CI=1` (stdin `/dev/null` là đủ để không hỏi), và grep `CI mode` trong log ngay sau khi bật.
- **Nạp bundle vào dev client** bằng `rudi://expo-development-client/?url=http%3A%2F%2F127.0.0.1%3A8096` (sau `adb reverse`); scheme `com.lakiet.rudi://` chỉ mở launcher. Deep link ấm `rudi://<route>` chỉ dùng khi bundle đã nạp.
- **Ba phần ba chính xác thì cột cuối rớt hàng** trên Android (làm tròn pixel); `gridFor` giờ trả dp nguyên.
- **Đọc `sharedValue.value` lúc render** bị Reanimated strict cảnh báo; `Sheet` chuyển sang state React đóng sau `withTiming` qua `runOnJS`.
- **`scripts/mobile_native.sh --serial` không tới được Maestro.** Có hai máy ảo thì `maestro test` chạy trên máy đầu tiên của `adb devices`, còn adb/screencap của script chạy trên máy được chỉ định: ảnh FAILED chụp đúng máy mình, flow lại lái máy người khác. Chỉ chạy bảng khi máy mình là máy duy nhất, hoặc thêm `--device "$SERIAL"` vào lệnh maestro.
- **`pkill -f "<mẫu>"`** giết luôn shell đang chạy lệnh (exit 144); tắt Metro bằng `kill -- -<pgid>` sau khi bật bằng `setsid`.

## 7. Tái lập bằng chứng

```bash
# từ apps/mobile
rm -rf dist-test && npm test
npx tsc --noEmit -p tsconfig.json
~/.claude/skills/impeccable-pipeline/scripts/imp detect --json src/rudi
# từ gốc repo
python3 scripts/repo_guard.py range bd32e4d cc5c6795
ANDROID_SERIAL=<serial> scripts/mobile_native.sh --port 8097 --serial <serial>
```
