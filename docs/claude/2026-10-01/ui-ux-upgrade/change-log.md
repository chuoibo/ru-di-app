# Change log: nâng cấp UI/UX từ audit QA PR #663

Mỗi mục: thay đổi gì · vì sao · nhóm (BUG_FIX / UX_IMPROVEMENT / VISUAL_UPGRADE / MOTION_UPGRADE /
DESIGN_SYSTEM_IMPROVEMENT) · file · phạm vi hồi quy cần kiểm. Ghi theo commit, mới nhất ở cuối.

## B0 · Nền (không đổi hành vi app)

- Tài liệu đợt: `direction.md`, `feature-plan.md` (167 hàng), file này, `qa-handoff.md`.
- Baseline trên `d95edb4`: `npm run typecheck` 0 lỗi; `npm test` 1384/1384 (test QA ghi hỏng sẵn
  `rudi-hanh-trinh-web.test.mjs:76` nay qua). Trước đó `node_modules` của máy dev lệch lockfile (thiếu
  `@shopify/react-native-skia`), đã `npm ci` lại; không đổi file nào trong repo.
- Môi trường kiểm chứng ngoài repo: stack loopback riêng (Postgres 16, API Python, core Go `ported`, Cộng đồng bật,
  OTP debug chỉ trong env của container), dữ liệu tổng hợp `seed:rudi`, `chat_e2e_seed.mjs`, `seed-bien-the.mjs`;
  harness QA lấy từ nhánh PR bằng `git archive`, không commit lại.

## B1 · P1, quyền riêng tư, lối vào và lối ra

### Chặn thì sổ hai người dừng (UI-120, P1) · BUG_FIX
- **Máy chủ**, Go và Python cùng commit (route `pair_papers`/`notebook` có `python: live`; ngoại lệ bảo mật theo
  CLAUDE.md): 11 lệnh ghi «hướng ra ngoài» đọc chặn ngay sau bước kiểm quyền, qua đúng cổng của chat đôi
  (`_require_pair_is_alive` / `requirePairIsAlive`, mã `direct_message_unavailable`, một câu cho cả «bị chặn» lẫn «tài
  khoản đã kết thúc»): phác, sửa nháp, gửi, đã xem, trả lời, đã đi, giữ dòng, đề nghị và đồng ý bậc sổ, đặt ô ràng buộc,
  vai tuần. Lệnh tự rút lui vẫn làm được: rút tờ, nghỉ tuần, thu hồi, xoá ô, đóng sổ. Căn cứ: ADR-0027 §3 bước 1.
- File: `services/api/app/api/service.py`, `services/core/internal/domain/pairsteps/{pairsteps,papers,notebook}.go`,
  `services/core/internal/service/pair_store.go`; goldens `pairsteps/testdata/python_pair_steps*.json` sinh lại bằng
  `scripts/render_domain_w8_goldens.py` (thêm 33 ca `stopped/*`); mô hình route của oracle repo
  (`internal/repo/pair_repo_routes_postgres_test.go`); hai ca oracle trên cặp «người kia đã rời» nay ghi kết quả
  `409:direct_message_unavailable`, ca danh mục quán đưa thành viên trở lại để vẫn phủ nhánh đọc danh mục.
- **App**: `SoDoiApi.daDung` (đọc `phien.contexts[].unavailable`, hoặc mã vừa bị từ chối); màn tờ giấy khi dừng là
  trang chỉ đọc «Sổ này đã dừng» (không lộ nguyên nhân), không còn nút hướng ra ngoài, cài đặt sổ bỏ «Loại sổ» và
  «Những điều cần tránh»; hàng ghim «Tờ giấy» trong chat đôi đã dừng không còn; `LOI_TO_GIAY` có câu cho mã mới.
- Goldens có 44 ca `stopped/*` (11 lệnh × 4 thế giới: bị người kia chặn, tự chặn, tài khoản không còn hàng, tài khoản đã
  xoá mà hàng thành viên còn). Thế giới thứ tư được thêm sau khi một đột biến sống sót: bỏ `person.Deleted` khỏi cổng Go
  vẫn xanh, vì ba thế giới đầu chỉ mô tả tài khoản đã xoá bằng «không có hàng». Nhánh `deleted_at`/`Deleted` là nhánh
  repository thật đi qua.
- Đột biến tự nghĩ, mỗi cái đỏ đúng chỗ dự đoán:
  - Go: bỏ cổng ở `DraftPaper` → đúng 3 ca `stopped/*` của `draft_pair_paper`; coi tài khoản đã xoá là còn → đúng 11 ca
    `stopped/account_deleted`, mỗi lệnh 1.
  - Python: bỏ cổng ở `draft_pair_paper` → test mới đỏ ở bước phác sau khi chặn; bỏ cổng ở `respond_pair_paper` → đỏ ở
    bước trả lời (200 thay vì 409).
- Hồi quy cần kiểm: mọi luồng sổ hai người của cặp còn sống (Maestro `_diary*`, `4x-*`), chat đôi bị chặn, bỏ chặn rồi
  phác lại.

### UI-131 → đề xuất ADR (không sửa máy chủ)
- Gác «tờ giấy chỉ cho cặp đôi» ở máy chủ là xoá tờ tạm mà ADR-0027 §4 cố ý giữ và ADR-0038 §2.1 đã bác việc từ chối.
  Xem `adr-de-xuat/UI-131-to-giay-chi-cho-cap-doi.md`.

### Gói bối cảnh AI mặc định 20 tin (UI-163) · BUG_FIX (quyền riêng tư)
- `SO_LUOT_MAC_DINH = 20` theo ADR-0046 §2; trần 40 vẫn là giới hạn, số xin thêm bị giữ ở trần. Chip và tấm «Xem» đếm
  từ chính gói nên tự nói «20 tin».
- File: `src/rudi/ai/boi-canh.ts`, `src/rudi/chat/boi-canh-chat.ts`; test `tests/chip-boi-canh.test.mjs`.
- Đột biến: mặc định 40 → đỏ ca UI-163; bỏ trần `Math.min(…, 40)` → đỏ ca UI-163.

### Back không rời app sau khi đổi tab (UI-123) · BUG_FIX
- `Tabs backBehavior="history"`. File: `app/(tabs)/_layout.tsx`.
- Hồi quy cần kiểm: Back hệ thống trên Android ở màn tab (đi lùi qua các tab đã ghé rồi mới thoát).

### Một cửa đăng nhập cho link lạnh, và quay về đúng link (UI-121, UI-082, UI-137) · UX_IMPROVEMENT
- `duongTiep`/`duongDangNhap`/`manSauDangNhap(phien, tiep)` trong `src/rudi/duong-vao.ts`; `?tiep=` đi qua Login →
  OTP (`otp-dang-cho.ts`) → Sở thích của người mới; chỉ nhận đường nội bộ, không trỏ lại vào chính các cửa.
- Đột biến của `duongTiep`: nhận `//host` → đỏ 3 ca (`duongTiep`, `duongDangNhap`, `manSauDangNhap`); bỏ lọc cửa vào → đỏ
  ca `duongTiep`.
- `ui/CuaDangNhap.tsx` (`CuaDangNhap`, `CanPhien`) thay ~12 chỗ chuyển thẳng về `/welcome` hay `/login`: chat nhóm,
  kèo, `outings/chon`, khép cuộc đi, sổ chuyến đi, album nhóm, bạn bè, thêm bạn, thành viên, lập nhóm, mời; bốn route
  Cộng đồng (bài, viết bài, thông báo, hàng duyệt) và tìm kiếm, «Điều mình muốn giữ».
- Tờ giấy: chưa đăng nhập thì link sổ thật đi qua cửa đăng nhập (giữ `?ru=1&cho=`); chỉ `cap-demo` là sổ mẫu, và sổ mẫu
  có nhãn «Demo».
- Chưa có phiên, thanh tab có thêm một cột «Đăng nhập» ở cuối (rail: mục ở chân rail), nhãn đọc «Bạn đang xem bản trải
  nghiệm. Đăng nhập», đăng nhập xong về đúng tab vừa bấm. Là một cột chứ không phải dải trên thanh: bản dải đầu tiên lấy
  48dp của mọi màn và đẩy hộp chọn của bản đồ Hành trình ra khỏi bản đồ ở 390×844 (test `rudi-hanh-trinh*` bắt được).
  File: `ui/DaiTraiNghiem.tsx`, `ui/RudiTabBar.tsx`.
- Màn demo dạng stack (sổ demo, Lịch trình AI, Ai dùng món nào?, Bình chọn…): `TopBar` có **một cửa demo**: bình thí
  nghiệm + «Demo» (12sp) + biểu tượng đăng nhập, tên trợ năng «Dữ liệu demo. Đăng nhập». Bấm thì tới cửa đăng nhập với
  `?tiep=` của màn đang xem. Không có trên chính các cửa vào.
  - `DemoBadge` đặt trong một TopBar đã có cửa thì nhường chỗ, nên không nơi nào có hai chữ «Demo».
  - Dưới 360dp cửa chỉ còn hai biểu tượng.
  - Bản đầu đặt nhãn và một nút đăng nhập rời nhau. Detector Impeccable trên trang đã render bắt được hệ quả:
    - tiêu đề sổ demo còn 6px ở 390dp, 0px ở 320dp;
    - «Ai dùng món nào?» còn 40/147px ở 320dp.
- `TopBar` không cắt tiêu đề nữa:
  - Hai bên chỉ bằng nhau khi tiêu đề (đo bằng một bản sao ẩn) vẫn vừa; không vừa thì mỗi bên giữ bề rộng của mình.
  - Tiêu đề được xuống hai dòng trước khi có dấu «…».
  - Đo lại: 0 tiêu đề bị cắt ở sổ demo (C1, C2), Lịch trình AI (C1), Ai dùng món nào? (C2).
  - Hồi quy cần kiểm: mọi màn có TopBar với vế phải rộng; cỡ chữ 1.3 và 2.0.
- Hai nhãn «Nháp» chỉ hiện ở bản demo nay nói đúng là «Demo» (nhãn đọc «Dữ liệu demo · AI nháp», «Dữ liệu demo · nháp trên
  máy»).
- File: `src/rudi/ui.tsx`, `screens/Group.tsx`, `screens/Bill.tsx`.
- Cộng đồng chưa đăng nhập không có dữ liệu mẫu (chỉ lời mời đăng nhập), nên không mang nhãn demo; nút «Đăng nhập» của
  nó nay giữ `?tiep=/community`.

### Chia sẻ và chỉ đường trên web (UI-049 P1, UI-136, UI-022) · BUG_FIX + UX_IMPROVEMENT
- `src/rudi/web/chia-se.ts`: `chiaSe()` trả «đã mở khay» / «đã chép» / «huỷ» / «không được»; Web Share, không có thì chép
  vào clipboard.
- Đợt thu: kết quả nói ngay dưới tên người vừa gửi («Đã chép link. Dán vào tin nhắn gửi …»); không được thì hiện chính
  link để chép tay. Không còn câu «Kiểm tra mạng» ở đầu trang.
- Cộng đồng: nhãn nút «Chia sẻ» nói kết quả 4 giây; web chia sẻ link https của trang. Native vẫn `rudi://` (đề xuất
  `adr-de-xuat/UI-136-link-https-cho-bai-viet.md`).
- `chiaSe()` nhận link riêng (`url`): Web Share và iOS nhận link như một link, Android và clipboard nhận chữ + link trong một
  tin. Bài Cộng đồng chia sẻ bằng `url`.
- Câu sau «đã mở khay» trên đợt thu: «Đã mở khay chia sẻ cho <tên>.» thay cho «…, chưa rõ đã gửi link chưa.» (hai chữ
  «chưa» trong một câu; nút ngay cạnh đã là «Gửi lại cho <tên>»).
- Test `tests/chia-se-web.test.mjs` (5 ca, chạy module thật qua react-native-web). Ba đột biến tự nghĩ, mỗi cái đỏ đúng ca
  dự đoán: bỏ nhánh AbortError → ca 2; nối chữ và link bằng dấu cách → ca 3; bỏ `url` khỏi Web Share → ca 1.
- Chỉ đường: bản web mở URL bản đồ https (`duongChiDuongWeb`), native giữ `geo:`.

### Sheet: Back trình duyệt đóng sheet, không để lại vùng khoá (UI-005 P1, UI-038, UI-117, UI-087) · BUG_FIX
- Trả `aria-hidden`/`inert` chỉ khi thuộc tính vẫn là giá trị sheet đã đặt (navigator đổi trước thì để yên).
- Web: một listener `popstate` duy nhất, đăng ký lúc nạp module (trước router — Chrome gọi listener trên `window` theo
  thứ tự đăng ký, cờ capture không đổi thứ tự ở chính target). Có sheet mở thì nó chặn sự kiện, đặt lại đúng mục lịch sử
  vừa rời, rồi đóng sheet trên cùng; không thêm mục lịch sử nào khi mở (bản đầu đẩy một mục khi mở, mục đó bị vùi khi
  hành động trong sheet điều hướng nên đã bỏ).
- Màn chứa sheet mất focus (sang tab khác, đẩy màn khác) thì sheet đóng.
- File: `src/rudi/ui/Sheet.tsx`. Hồi quy cần kiểm: mọi sheet có hành động điều hướng (cài đặt sổ, khay tạo, menu tin),
  Back Android, Esc.

## B2 · Primitive dùng chung

### Sheet v2 (UI-006, 007, 013, 040, 070, 088, 089, 093, 166) · DESIGN_SYSTEM_IMPROVEMENT + MOTION_UPGRADE
- **Trần 82% áp cho cả panel** (tay cầm, đầu trang, nội dung và lề đáy), không chỉ cho ScrollView bên trong; nội dung co
  và cuộn trong trần (UI-007, UI-040).
- **Trượt theo chiều cao thật của panel** (đo bằng `onLayout`), không cố định 480dp. Mờ dần ở đoạn cuối của pha đóng, nên
  khung cuối không còn panel đứng giữa màn rồi biến mất (UI-013).
- **Chặn chạm 250ms sau khi mở**, cả nền lẫn panel: chạm thứ hai của một chạm đúp không còn đóng sheet vừa mở (UI-088),
  không còn rơi vào thẻ của khay đang trượt lên (UI-006).
- **Tay cầm** là cử chỉ cho tay, không phải control cho trình đọc màn hình: ẩn khỏi cây truy cập. «Đóng bảng» là lối ra trợ
  năng; hết `aria-prohibited-attr` (UI-089).
- Tay cầm có `testID="tay-cam"` để công cụ đo tìm được; trước đó harness tìm bằng nhãn «Tay cầm».
- **Tablet:** panel rộng tối đa 640dp, canh giữa (UI-093).
- **Thứ tự lớp:** lớp sheet `zIndex: 10`, nên dải ghim của chat (z 1) không còn vẽ đè lên nền mờ và tấm «Xem» (UI-070,
  UI-166).
- File: `src/rudi/ui/Sheet.tsx`.
- Hồi quy cần kiểm: mọi sheet (39 chỗ gọi), nhất là sheet có trình soạn dài ở C8 và sheet mở rồi điều hướng ngay.

### Khay trong màn: một hợp đồng đóng (UI-066, phần khay của UI-038) · UX_IMPROVEMENT
- `ui/useDongKhay.ts`: Escape và Back trình duyệt trên web, Back hệ thống trên Android, trả focus về nút đã mở.
- Khay công cụ của chat chuyển sang dùng hook này; khay «Tờ hẹn chung» lần đầu có nó.
- Back trình duyệt đi qua `ui/lui-web.ts`: listener `popstate` duy nhất của B1, tách khỏi `Sheet.tsx` để sheet và khay dùng
  chung một ngăn xếp. Back khi khay công cụ mở nay ở lại chat (`TC-L20-VONGDOI` FAIL → PASS).

### Câu lỗi theo mã (UI-019, UI-029, UI-072, nền cho UI-100) · UX_IMPROVEMENT
- `src/cau-loi-theo-ma.ts`: mã máy chủ chọn câu **trước** mã HTTP. Có câu riêng cho các mã người dùng gặp; còn lại theo
  dạng mã (`*_not_found`, `*_wrong_state`, `*_conflict`, `*_expired`, `*_unavailable`, `*_too_large`).
- `laTuChoiVinhVien`: màn chỉ mời «Thử lại» khi bấm lại có thể đổi kết quả.
- `thongDiepNguoiDoc` nhận `code`:
  - 401 nói phiên hết, tách khỏi 403;
  - 404 «cập nhật app» chỉ còn cho route thiếu;
  - 409 không mã riêng không còn mặc định là câu idempotency.
- Mã mời sai: bảng `LOI_DOI_LOI_MOI` thiếu `invite_not_found` (tên Go đặt) nên câu rơi về «Cập nhật app». Nay là một câu
  gọi tên cả bốn khả năng (gõ sai, hết hạn, đã dùng, đã huỷ) vì máy chủ không nói là cái nào.
- `membership_already_open` vào bảng mời.
- Khám phá truyền câu nguyên nhân vào `ErrorState` (503 khác mất mạng).
- Test `tests/cau-loi-theo-ma.test.mjs` (9 ca).

### `CauTaiCho`: câu lỗi đặt tại chỗ · DESIGN_SYSTEM_IMPROVEMENT
- Một câu `warn` cỡ `body` kèm icon, đặt ngay dưới control vừa thất bại hoặc trong chân dính trên nút.
- `aria-live=polite`, mờ vào theo `standard`.
- Trên web: tự cuộn vào khung nếu còn nằm ngoài cửa sổ.
- Một lối đi tiếp tuỳ chọn («Nhập tay», «Thử lại»).
- Dùng lần đầu ở sheet «Những điều cần tránh». Các màn còn lại chuyển dần theo batch của từng feature.

### Back luôn tới đâu đó, và focus vào màn mới (UI-018, UI-112) · BUG_FIX
- `lui-ve.ts`:
  - `luiVe(router, pathname)`: có lịch sử thì lùi; không có thì về tab của route.
  - `luiVeVe(router, cha)`: cho màn biết rõ cha của mình.
- Áp cho `TopBar`, `CoverBand`, 15 chỗ `router.back()` trần, và cả việc đóng khay Tạo mới.
- Trên web, màn nhận focus điều hướng thì focus vào tiêu đề `TopBar` (`role="heading"`, `tabIndex -1`), trừ khi đang gõ
  hay có sheet.
- Test `tests/lui-ve.test.mjs`.

### Trạng thái tới DOM, và phím Space (UI-003, UI-053) · BUG_FIX
- `src/ui/a11y.ts`:
  - `toggleState(role, on, onToggle)` gắn Space cho checkbox, radio, switch trên web;
  - `tabState` + `TABLIST`;
  - `giuState` cho nút giữ: `aria-pressed` trên web, `selected` trên native;
  - `expandState`.
- 37 chỗ chuyển từ `accessibilityState` (web nhận được không gì) sang các helper này hoặc `aria-*`: thanh tab, Segmented,
  tab Cộng đồng, tab Thành tích, ghế bàn ăn, danh sách người, phiếu bầu, ngân sách, …
- Test render qua react-native-web `tests/trang-thai-tro-nang.test.mjs` (5 ca).

### Con dấu «Tạo mới» trên mọi tab · UX_IMPROVEMENT + VISUAL_UPGRADE + MOTION_UPGRADE
- Spike hai biến thể: chọn **cột giữa nhô lên** (lý do và ảnh ở `direction.md`).
- `ui/ConDauTao.tsx`, `src/rudi/tao-moi.ts`, `RudiTabBar` (tablist + ô rỗng `aria-hidden` cho con dấu), khay Tạo mới
  xếp theo tab (`?tu=`) và có «Viết bài».
- `/create` mở lạnh mở đúng khay (UI-010).
- Bản demo: cột «Đăng nhập» của B1 thay bằng nhãn `DemoBadge` làm cửa. `DaiTraiNghiem.tsx` bỏ.
- Rail: vạch chỉ báo đo từ lề trên của rail, và tab rail cao 72 thật trên web (UI-004).
  - react-native-web viết `flex: 0` thành `flex-basis: 0`, thứ thắng `height`, nên mỗi tab rail chỉ còn 48dp trong khi vạch
    bước 72.
  - Style rail nay không có `flex`. Đo lại 10/10 vạch nằm trên tab đang chọn ở C6, C7 (trước 2/10).

### Nút phá huỷ, ô 48, tiền không cắt, chip không tràn, cột đọc · DESIGN_SYSTEM_IMPROVEMENT
- `RudiButton`:
  - `tone="warn"` (outline/ghost, màu `warn`);
  - bản dev cảnh báo một lần mỗi nhãn khi nút tắt không có `lyDo` (ADR-0038 §2.2).
- `RangBuoc`: nút lưu chỉ hiện khi có thay đổi (UI-091).
- `ONhapMuc`: ô một dòng cao 48 (UI-001). 4dp lấy từ khe trên gạch nên hàng không đổi chiều cao.
- `Money`: `flexShrink: 0`, số tiền trong một hàng không bao giờ bị cắt (UI-048, phần primitive).
- `Chip`: `maxWidth: 100%`, nhãn dài có dấu «…» trong chip (UI-099, phần primitive).
- `RudiScreen`:
  - `cot` (`doc` 640, `form` 560, `rong` 960) cho nội dung, đầu và chân;
  - cột tablet có `width: 100%`, nên không còn co theo phần tử con rộng nhất;
  - tờ giấy dùng `doc` (UI-093).

### Đột biến tự nghĩ (mỗi cái đỏ đúng chỗ dự đoán)
- Bỏ đọc mã trước trạng thái → UI-019, UI-072 và ca 404 đỏ.
- 409 trở lại câu idempotency → ca 409 đỏ.
- `*_unavailable` nói về mạng → ca UI-029 đỏ.
- Cộng đồng đặt «Tạo cuộc hẹn» lên đầu → ca theo tab đỏ.
- Thẻ đầu không bỏ khỏi phần còn lại → ca «không giấu việc nào» đỏ.
- `luiVe` luôn `back()` → hai ca không lịch sử đỏ.
- Space nhận cả Enter → ca phím đỏ.
- Tab mang `aria-checked` → ca tab đỏ.
- Ô cao 44 → ca chiều cao đỏ.

## B3 · Pilot: Kèo · Lên plan · Hành trình (F03 + E2)

### Kèo nhiều ngày: chặng chưa xếp ngày được nêu tên và xếp được (UI-032) · BUG_FIX + UX_IMPROVEMENT
- Máy chủ để `day` rỗng cho chặng của kèo dài hơn một ngày, nên chặng ấy không thuộc trang ngày nào. Bản đồ ba ngày đều
  trống, và còn bảo người dùng «gắn một quán» họ đã gắn.
- Trang ngày nay nêu tên các chặng ấy: «3 chặng của kèo chưa xếp ngày: Cà phê sáng, Ăn trưa, Tối nướng.».
  - Ngày trống: tiêu đề «Ngày này chưa có chặng nào», kèm nút chính «Xếp cả 3 chặng vào ngày này».
  - Ngày đã có chặng: thêm một dòng, kèm nút ghost «Xếp vào ngày này».
- Xếp là **bản nháp**: mốc hiện ngay trên bản đồ, «Lưu những thay đổi» hiện ra, và cả hội chỉ thấy sau khi lưu. Không
  đoán ngày cho lịch cũ (giữ đúng luật `nhapTuKeo`): chỉ xếp khi người dùng chạm.
- Hàm thuần `changChuaXep`, `xepVaoNgay` trong `hanh-trinh/ke-hoach.ts`, có test. Chặng xếp vào nằm sau các chặng sẵn có
  của ngày, giữ thứ tự của chúng.
- Chip chặng trong sheet sửa đổi «chưa chia ngày» thành «chưa xếp ngày»: một tên cho một việc.

### Ngày trống trên Bản đồ: «Về Lịch trình» và lời giải thích luôn thấy (UI-033) · UX_IMPROVEMENT
- Trang ngày chỉ bị giới hạn 60% chiều cao khi ngày có mốc. Ngày trống thì bản đồ không có gì để xem, nên nhường chỗ cho
  lời giải thích và lối về.
- Ở cửa sổ thấp, trang của ngày trống co lại vừa khung: phần giữa cuộn, còn đầu trang và «Về Lịch trình» thì giữ nguyên.
  Vòng đo đầu cho thấy bỏ trần mà không cho co thì nút còn tụt sâu hơn (C8: thấy 40% → 0%), nên đã thêm cả phần co.
- Hàng «Thu gọn» không vẽ ở ngày trống, vì không có gì để gập. Trang đang gập ở ngày khác sẽ mở lại khi sang ngày trống.
- Dải mờ ở đáy vùng cuộn chỉ phủ danh sách chặng, không phủ dòng giải thích (QA thấy dòng thứ hai bị cắt ở C1).
- Tranh 144dp chỉ vẽ khi khung cao ≥560dp; ở cửa sổ thấp (390×460) nó đẩy câu và nút ra ngoài.

### Tạo kèo: ô ngân sách không còn trông như đã điền; lỗi nói ở đúng ô (UI-034) · UX_IMPROVEMENT
- Gợi ý ô ngân sách đổi từ «250000» sang «ví dụ 250.000». Một số trơn trên dòng mực đọc như đã có người viết.
- `kiemTraTaoBuoiDi` trả thêm `o` (ô bị từ chối: tên, ngày, số người, ngân sách).
  - Câu lỗi hiện **dưới đúng ô đó** (dòng mực đổi màu `warn`), và con trỏ được đặt vào ô.
  - Số người: câu nằm dưới cả hàng, vì ô chỉ rộng 64.
  - Ngày: câu nằm dưới hàng lịch.
- Lỗi máy chủ hoặc lỗi nguồn tờ hẹn nằm trong `footer` dính đáy, ngay trên con dấu «Tạo kèo» (`CauTaiCho`). Trước đó câu ở
  cuối form, dưới nút, ngoài khung nhìn.
- `ONhapMuc` nhận `oRef` để màn đặt con trỏ.

### `/trips/[id]/timeline` có phiên thì về kèo thật (UI-035) · BUG_FIX
- Giống `itinerary` bên cạnh: người đã đăng nhập được chuyển tới `/outings/[id]`, không còn thấy lịch trình demo dưới id kèo
  thật.

### Đổi thứ tự chặng bằng bàn phím; ARIA hợp lệ (UI-036, UI-042) · BUG_FIX
- Trên web, tay nắm là `slider` thật: nhận focus (`tabIndex 0`), có `aria-valuenow/min/max/valuetext`, hướng dọc.
  - Mũi tên xuống/phải dời chặng xuống, lên/trái dời lên, Home/End về đầu/cuối.
  - Focus theo chặng vừa dời, nên bấm tiếp vẫn dời đúng chặng đó.
- Hàng chặng bỏ `aria-selected` (không hợp lệ trên `button`); trạng thái chọn đi qua `giuState` thành `aria-pressed`.
- Native giữ `adjustable` với hai thao tác tăng/giảm như cũ.

### Nếp có mặt ở Lịch trình (UI-037) · UX_IMPROVEMENT
- Bản đồ vẫn mount dưới chế độ Lịch trình để giữ camera. Nó xin Nếp nhường chỗ vô điều kiện, nên Nếp vắng cả ở Lịch trình.
- `ManHinhHanhTrinh` nhận `nhuongNep`; `SoHanhTrinh` chỉ bật khi đang ở Bản đồ.

### Sheet «Sửa trang ngày» phủ cả màn (UI-041) · DESIGN_SYSTEM_IMPROVEMENT
- Primitive mới `ui/KheLop.tsx`: khe lớp phủ của màn, với tới được từ sâu bên trong.
  - `RudiScreen` giữ khe cạnh `overlay`.
  - `LenLop` đưa con của nó lên khe, nên sheet phủ cả màn (kể cả đầu màn) mà vẫn giữ state và callback của component đã vẽ
    nó.
  - Không có khe (trang lab, test) thì vẽ tại chỗ.
- Con được đưa lên trong layout effect, cập nhật được flush cùng commit, trước khi ô nhập controlled khôi phục giá trị. Ô
  nhập trong khe không bao giờ hiện chữ chậm một phím.
- Hai sheet của bản đồ (sửa trang ngày, điểm hẹn) dùng khe. Tiêu đề sheet đổi từ «Những hẹn quan trọng» thành «Sửa trang
  ngày», đúng tên nút đã mở nó.
- Sheet sửa trang ngày dùng ô dòng mực `ONhapMuc` thay cho ô hộp `Field`, cho đúng luật «mọi ô nhập là dòng mực»: đây là
  màn duy nhất của luồng còn ô hộp. «Bỏ chặng» mang tông `warn` (nút phá huỷ của B2). · VISUAL_UPGRADE

### Bản đồ nói tiếng Việt; Esc đóng danh sách điểm gần nhau (UI-043) · UX_IMPROVEMENT
- Truyền `locale` cho MapLibre: «Đóng danh sách điểm gần nhau», «Hiện hoặc ẩn nguồn bản đồ», «Bản đồ hành trình»…
- Esc đóng popup cụm như nút đóng, và trả focus về đúng cụm (tìm lại theo `data-nhom` vì lệnh dời bản đồ vẽ lại mọi
  marker).

### Vé kèo xuống dòng theo cụm (UI-044) · UX_IMPROVEMENT
- Dòng thông tin của vé kèo ở Lên plan ghép theo cụm «·», mỗi cụm giữ liền bằng khoảng trắng không ngắt, tối đa 3 dòng.
  Trước đó một dòng bị cắt đúng ở «12 chặng · Còn 23 ngày», phần đáng mở vé nhất.

### Cột tên chặng không dưới 120px (UI-045) · VISUAL_UPGRADE
- `HangChang` đo bề rộng hàng và nút bên phải. Nếu tên chặng còn dưới 120px thì nút («Tôi đã tới», «Đã tới») xuống một
  hàng riêng dưới chặng, trục mực chạy tiếp bên cạnh.
- Ngưỡng tính trên số đo thật, nên đổi cỡ chữ hay nhãn dài hơn thì ngưỡng đi theo.

### `/outings/chon` thiếu địa điểm (UI-046) · BUG_FIX
- Không còn skeleton vô hạn: «Chưa có địa điểm để thêm», kèm cách làm và nút «Mở Khám phá». Lỗi ghi kèo dùng
  `CauTaiCho`.

### «Thêm chặng» chỉ mở (UI-039) · BUG_FIX
- Sheet v2 đã chặn chạm đúp; nay nút cũng chỉ mở (`setMoThem(true)`), còn đóng thuộc về sheet, đúng đề xuất của QA.

### Kèo tạo từ chat để lại dấu trong chat (UI-118) · UX_IMPROVEMENT
- Chat là E2EE nên máy chủ không đăng được gì vào cuộc trò chuyện. Thay vào đó, dải ghim đầu chat đọc kèo sắp tới của
  nhóm, từ cùng nguồn với tab Lên plan.
  - Thẻ «Kèo sắp tới · <tên> · Còn N ngày», chạm thì mở kèo.
  - Đọc lại mỗi lần chat được focus, nên Back từ kèo vừa tạo là thấy.
  - Giống nhau trên máy mọi thành viên, và không thêm gì vào cuộc trò chuyện.
- Dải ghim cho thấy cả việc hội đang chọn lẫn kèo sắp tới.
  - Có cả hai: kèo là một dòng mảnh phía trên tờ hẹn hoặc phiếu đang mở. Dải chỉ dài thêm một hàng, không thêm một thẻ.
  - Cửa sổ thấp (<600dp): dải giữ một mục, và việc đang chọn được ưu tiên.
  - Thẻ «Đã thành kèo» nhường chỗ cho kèo sắp tới, vì nó chỉ trỏ tới một kèo.
  - Lý do: nhóm Đà Lạt có một phiếu bầu chưa đóng. Nếu việc đang chọn luôn thắng thì nhóm ấy không bao giờ thấy kèo của
    mình.
- Đọc kèo hỏng thì giữ dải đang có, không thay bằng lỗi.

### «Tôi đã tới» mời giữ khoảnh khắc; album nói nó đếm gì (UI-119, phần UI) · UX_IMPROVEMENT
- Ngay sau «Tôi đã tới», dưới chặng hiện: «Cả hội đã thấy bạn tới. Ảnh ở đây thì lên tường kỷ niệm.» và nút «Thêm khoảnh
  khắc ở đây». Nút mở màn thả khoảnh khắc, gắn sẵn nhóm và quán của chặng.
- `HangChang` có khe `duoi` nằm ngoài vùng bấm của hàng, nên không thành nút lồng nút.
- Album đổi «0 chỗ đã tới · 0 check-in» thành «N check-in ở M chỗ». «Đã tới» là chữ của kèo, album không đếm nó.
- Tường trống nói rõ hai việc khác nhau: «Tôi đã tới» ở kèo chỉ báo cho hội, không tự lên tường.
- Đổi luật đếm (lượt tới chặng thành check-in của album) vẫn là ADR đề xuất `adr-de-xuat/UI-119-da-toi-la-check-in.md`;
  không làm trong batch này.

### Lên plan giữ nút «Tạo mới» ở đầu màn (quyết định còn treo từ B2)
- Con dấu mở khay từ mọi tab. Ở Lên plan, tạo kèo là việc chính của tab, nên tab có thêm một cửa có chữ ở đầu màn.
- Cạnh `plan → create` của sổ tay hướng dẫn cũng được rút từ chính nút này (`huong-dan-khop-ma.test.mjs`). Bỏ nút thì phải
  sửa danh sách cạnh ở cả app lẫn máy chủ (`huongdan/nap.go`) mà không đổi được gì cho người dùng.

### Android, hai lỗi nhỏ thấy sau B2
- Nhãn «Tạo» dưới con dấu nay thẳng hàng với nhãn các tab: cột con dấu bố trí như một cột tab (căn giữa, đệm trên 8),
  con dấu tính như icon 24dp, phần còn lại nhô lên khỏi dải.
- `TopBar` đọc navigator của chính màn, không đọc route hiện tại của app. Dưới route trong suốt «Tạo mới», route hiện tại
  là `/create`, nên đầu màn tab phía sau khay từng vẽ cửa demo.

### Sau critique (Flow D): một đợt sửa ở trang ngày, form tạo kèo và sheet sửa · UX_IMPROVEMENT + BUG_FIX
Critique trước (24/40) và sau (26/40), mỗi lần hai subagent cô lập, chỉ ra những điểm dưới đây; tất cả nằm trong mặt
pilot.
- **Tạo kèo:**
  - Bỏ trống ngân sách được nói đúng là chưa chọn: «Chọn một mức hoặc gõ số tiền mỗi người.». Câu cũ «… viết bằng chữ số»
    khiến người dùng đi tìm lỗi gõ.
  - Nút ± số người 48dp (trước 44).
- **Sheet sửa trang ngày:**
  - Dưới tiêu đề ghi ngày đang sửa, và «· bản nháp, cả hội chưa thấy» khi có bản nháp.
  - Có bản nháp thì «Lưu cho cả hội» là nút chính trong sheet; «Xem trên bản đồ» lùi thành viền.
- **Trang ngày trên bản đồ:**
  - Ngày trống không còn «Tính lại đường» (không có gì để tính), và bỏ ghi chú `empty_day` của máy chủ (nhắc lại đúng câu
    của trang trống).
  - Có bản nháp ở ngày có mốc thì nút ghim đáy là «Lưu cho cả hội»; «Xem cách đi gọn hơn» lùi vào hàng nút. Ngày có bản
    nháp mà chưa có mốc thì «Lưu cho cả hội» nằm trong trang. Một động từ lưu cho mọi chỗ.
  - Lỗi lưu, lỗi xem trước và xung đột hiện bằng `CauTaiCho` (mực `warn`). Trước đó chúng cùng màu inkSoft với «Đã giữ
    trang ngày cho cả hội», nên lưu hỏng trông như đã lưu.
  - Điện thoại trên web được bảo «Giữ trên bản đồ…» thay vì «Nhấp chuột phải…»: `(hover: none) and (pointer: coarse)`.
    Giữ trên Chrome Android phát `contextmenu`, đúng sự kiện bản đồ nghe.
- **Dòng nguồn bản đồ luôn thấy được (hồi quy của chính B3, probe bắt được):**
  - Bản đồ web có sàn `minHeight: 220`. Khi trang của ngày trống lấy đủ chỗ, khung bản đồ chỉ còn 152px. Bản đồ thòng
    68px xuống dưới trang, kéo theo dòng «© OpenStreetMap · OpenFreeMap».
  - Nay bản đồ cao đúng phần chỗ được chia, và khung cắt phần thừa. Lề khớp camera co theo khung, tối đa một phần tư mỗi
    cạnh; nếu không, MapLibre không còn chỗ để khớp và giữ camera cũ.
  - Ngày trống trong cửa sổ thấp (<560): bản đồ, vốn không có gì để xem, ẩn hẳn, cùng câu gợi ý ghim điểm. Một dải bản đồ
    không có dòng nguồn là điều bản đồ không được phép.
- **Test:** `khong-vien-web.test.mjs` quét `<TextInput` như chuỗi, nên đọc nhầm `useRef<TextInput>` và `Ref<TextInput>`
  thành thẻ JSX. Nay bỏ qua chỗ đứng sau một ký tự định danh (generic luôn như vậy, thẻ JSX thì không). Canary: bỏ
  `KHONG_VIEN_WEB` khỏi ô của `ONhapMuc` → đỏ.
- **Sổ tay hướng dẫn:** rút lại `_rut.json` cùng mã. Thêm ba cạnh mới: kèo → «Thêm khoảnh khắc ở đây», `/outings/chon` →
  «Mở Khám phá», timeline → kèo/Lên plan. `huong-dan-ban.ts` đổi theo. Test Go `./internal/huongdan` xanh.
  - `keo.md` (sổ tay của Nếp) đổi bước lưu ở chế độ hành trình theo nút mới: «Lưu cho cả hội», trong khay hoặc ở trang
    ngày. Test mobile «nhãn phải là literal của mã» bắt đúng nhãn cũ «Lưu những thay đổi».

### Sau finish review (disposition `fix`): tám điểm, một đợt · VISUAL_UPGRADE + UX_IMPROVEMENT
Reviewer chạy ngữ cảnh mới, chấm trên ảnh bản cuối, đối chiếu DESIGN.md và direction. Con dấu «Tạo» ngoài phạm vi.
1. **Popup cụm điểm thành mẩu giấy của sổ** (UI-043):
   - viền bút chì, độ cao giấy 2, đầu mẩu ghi «N điểm gần nhau» và nút đóng tự vẽ 48dp;
   - mỗi chặng một hàng trên kẻ tóc, số chặng là con dấu 26dp như dải chặng của trang ngày, giờ chữ số đều;
   - `offset` để mẩu không che chính con dấu cụm;
   - focus vào chặng đầu, không vào nút đóng;
   - bỏ hộp trắng và mũi của MapLibre.
2. **«ĐÃ TỚI» là con dấu** (teal, nghiêng -2, hạ xuống một lần ở chặng vừa tới), đúng DESIGN.md. Trước đó là chip bo
   tròn có dấu tích; bản demo đã đúng từ trước.
3. **«Tôi đã tới» thành nút ghost.** Mười hai chặng không còn đọc như một cột nút viền.
4. **Công tắc trong sheet sửa trang ngày** theo thế giới của app, như Cài đặt: rãnh `accent` / `lineStrong`, núm `card`.
   Trước đó là màu xám của nền tảng.
5. **Chip chặng trong sheet:**
   - mỗi chip tối đa 240dp, tên cắt sau giờ;
   - hàng tràn tới mép sheet, nên chip bị cắt đọc như «còn nữa» chứ không như vỡ.

   Dải chặng của trang ngày tràn đúng tới mép trang (16).
6. **Ngày trống, thứ bậc hành động:**
   - một nút chính đặc («Xếp cả N chặng…»), gợi ý ghim điểm ngay dưới nó;
   - «Sửa trang ngày» viền, đủ rộng;
   - «Về Lịch trình» vẫn luôn thấy (UI-033) nhưng là ghost, vì công tắc «Lịch trình | Bản đồ» ở trên đã nói điều đó.
7. **Cuống vé ở Lên plan** in tháng «Th 10» bằng chữ con dấu, như lá lịch của «Kèo mới». «tháng 10» gãy hai dòng và chạm
   đường đục lỗ ở 320.
8. **Giờ xuất phát và chặng giữ giờ sớm hơn.**
   - Dưới ô «Giờ xuất phát» có một câu `warn`: «‹chặng› giữ giờ 06:00, sớm hơn giờ xuất phát nên sẽ không kịp. Đổi giờ
     xuất phát hoặc bỏ giữ giờ ở chặng đó.».
   - Đúng ngữ nghĩa của máy chủ: `journey.Schedule` chỉ báo `late_fixed_stop` sau khi xem trước tuyến.

**Verdict pass** (cùng reviewer, ảnh dựng lại cùng các khung nhìn). Kết quả 6/8 resolved, 2 partial, 1 hồi quy:
- **Resolved:** popup cụm, con dấu «ĐÃ TỚI», công tắc, thứ bậc ngày trống, cuống vé, câu giờ xuất phát.
- **Partial:**
  - chip và dải chặng vẫn cắt cách mép khoảng 16dp;
  - «Tôi đã tới» ghost lệch khỏi cột chữ.
- **Hồi quy:** một mép hộp lộ giữa con dấu cụm và mẩu giấy.

Đợt chỉnh cuối, không chấm lại bằng reviewer mà kiểm bằng ảnh và probe của chính mình:
- **Cắt 16dp.** Đó là hộp cuộn của `Sheet`, nằm cách mép panel một lề `space.md` và cắt phần tràn ngang. Kéo qua nó thì
  phải đổi lề của `Sheet` dùng chung (39 chỗ gọi). Thay vào đó, hai hàng cuộn ngang có `MoNgang`: đầu cắt mờ dần vào
  giấy, nên chip bị cắt đọc như «còn nữa».
- **«Tôi đã tới» ghost** nằm dưới chặng thì chữ bắt đầu đúng cột chữ của chặng. `HangChang` nhận `phaiLeChu`, bù lề trong
  14dp của nút compact. Khoảng dọc giữ nguyên, để đích bấm 48dp không chồng lên hàng trên.
- **«Mép hộp lộ ra»** là con dấu của chặng thứ ba trên bản đồ, nằm dưới mẩu giấy, không thuộc mẩu. Popup phủ lên mốc của
  bản đồ là thứ tự lớp bình thường, nên không ẩn nội dung bản đồ.
- **Nhóm «Chọn điểm hẹn gần nhau»** (`role=group`) nay chỉ chứa các hàng chặng, không chứa nút đóng của mẩu.
  `rudi-hanh-trinh-web.test.mjs` đếm đúng 3 lựa chọn. Test tìm hàng theo tên truy cập «3, 20:00, Chợ đêm Đà Lạt», vì chữ
  hiển thị nay tách số, giờ và tên.

## B4 · Tiền: chia bill · quyết toán · đợt thu · tài chính (F04 + E3)

Không đụng allocator, sổ cái hay ba luật tiền. Mọi số trên màn vẫn là số máy chủ gửi; màn chỉ cộng lại để *cho thấy* tổng
khớp, không chia, không sửa.

### Số tiền không bị cắt trên bàn gán món (UI-048) · BUG_FIX
- Thẻ món trên bàn có bề rộng tối thiểu, lớn dần theo số tiền và không vượt bàn (`theMonToiDa`: bàn tròn +20%, bàn dài
  bằng mặt bàn trừ lề). Trước đó bề rộng cố định cắt «12.345.678đ» còn «12.345.6…» ở 320.
- Tên món xuống tối đa hai dòng, căn giữa. Ở thẻ hẹp hơn 124dp, số tiền viết nhỏ một bậc, không bao giờ cắt.
- Bàn tròn: `ry` tối thiểu 44 để đĩa không nằm dưới thẻ ở 288dp (test `hinh-tien`, bề rộng 288 thêm vào bộ đo).
- Quyết toán demo (F11): con dấu «Người thu bill» xuống dưới số tiền. Đứng cạnh số, nó để lại cho «1.106.250đ» 114 trên
  116dp cần ở 320.

### Nhóm đông ngồi bàn dài (UI-050) · UX_IMPROVEMENT + VISUAL_UPGRADE
- Từ 9 người, bàn tròn nhường cho **bàn dài nhìn từ trên xuống**, như bàn dài ở quán: mỗi bên một ghế mỗi hàng, ghế đủ
  48dp, thẻ món ở đầu bàn, hàng ghế đầu nằm dưới thẻ nên không đĩa nào bị che. Mặt bàn 120–180dp, để hai dãy ghế nhìn
  nhau qua bàn chứ không qua một mảng trắng 220dp.
- **Tên ghế rút từ phía họ** (`tenVua`): giữ phần đuôi dài nhất còn vừa ô, như cách cả nhóm gọi nhau («Nguyễn Thị Minh
  Anh» → «Minh Anh»). Lượt đo đầu của bàn dài cho thấy 20 ghế cùng đọc «Chat Test…», không ai phân biệt được ai. Tên đầy
  đủ vẫn ở tên truy cập «Ghế …». `RosterPicker` (dùng ở chia bill, quyết toán, kèo, kỷ niệm) dùng cùng quy tắc cho ô 72dp.
- Vẽ cùng ngữ pháp với bàn tròn (bóng, cạnh, mặt), không thêm token. Vẫn kéo món tới ghế và chạm ghế như cũ.
- Plan ban đầu nói «trên 10 người chuyển sang danh sách». Bàn dài giữ được cử chỉ kéo-thả và cái nhìn «cả hội quanh một
  bàn», nên chọn nó thay cho danh sách.

### Lý do chặn và lỗi nằm ngay trên nút (UI-051) · UX_IMPROVEMENT
- Câu của bước (thiếu tên món, chưa ai dùng món, 503…) đi vào `footer` dính đáy qua `CauTaiCho`, ngay trên nút chính.
  Trước đó ở đầu trang, y −248 / −256 / −578, ngoài khung nhìn.
- «còn 12.345.678đ chưa có người trả»: thêm «đ», số trơn đọc như số đếm.

### Bill đang gõ không mất (UI-052) · UX_IMPROVEMENT
- Bản nháp giữ theo nhóm ở mỗi thay đổi (`chia-bill/nhap-bill.ts`): trên web trong `sessionStorage` (sống qua Back,
  Forward, tải lại tab, chết cùng tab), trên điện thoại trong bộ nhớ suốt đời app. Không gửi gì lên máy chủ.
- Bỏ khi đã ghi sổ, hoặc khi món cuối bị bỏ (tải lại không được hồi sinh món đã xoá).
- **Mở lại ở đúng bước** khi vừa rời (tải lại, Back rồi Forward; bản nháp sửa trong 15 phút): bước Xem lại, hoặc bước Gán
  món với bill máy chủ đang giữ (`BillWire` cất cùng bản nháp). Bước Kết quả mở lại thành bước Gán món: cách chia luôn
  được tính lại, không bao giờ hiện lại số cũ. Bản nháp để lâu hơn thì không tự mở, mà được mời ở bước 1.
- Bước 1 khi có bản nháp: **chính tờ hoá đơn đang gõ nằm trên bàn và là nút** («Bill đang gõ», 3 món đầu, tổng, «Tiếp tục
  bill này»). Các lối khác thành nét bút chì: «Chọn ảnh bill khác», «Bắt đầu bill mới». Bắt đầu mới hỏi tại chỗ «Bỏ N món
  đang gõ để làm bill mới?» với «Bỏ, làm bill mới» (tông `warn`) / «Giữ lại».
- Test `rudi-nhap-bill.test.mjs`: sống qua một lần nạp lại module (= tải lại tab), không lẫn nhóm, kho bị chặn vẫn giữ bản
  trong bộ nhớ, dòng trống không tính là bill.

### Sơ đồ quyết toán biết lúc nào thôi vẽ (UI-054) · VISUAL_UPGRADE
- `soDoChuyen` chọn bề rộng nhãn theo hàng đông nhất: nhãn đầy đủ 88dp, chật thì 44dp với tên gọi (chữ cuối). Khi cả tên
  gọi cũng đè nhau thì không vẽ, và nói một câu: «Nhóm đông nên không vẽ sơ đồ: từng khoản chuyển ở danh sách dưới.».

### Nếp M2 ở trong màn (UI-055) · VISUAL_UPGRADE
- Tiêu đề «Ảnh này đúng bill chứ?» co lại nhường chỗ, nên Nếp ở bước xem ảnh không còn bị đẩy ra ngoài mép phải.

### Đọc ảnh hỏng: «Nhập tay» ngay chỗ báo (UI-056) · UX_IMPROVEMENT
- Câu lỗi đọc ảnh mang hành động «Nhập tay» ngay dưới nó (có bill đang gõ thì «Nhập tay» mở tiếp bill đó).
- Máy chủ chưa bật phần đọc (`receipt_reader_not_configured`) hay không có quyền: «Dùng ảnh này» rút đi, vì bấm lại cũng
  không được. Lỗi tạm thời: nút đổi thành «Đọc lại ảnh này».

### Mép Nếp trên màn tiền không hứa điều nó không làm (UI-057) · BUG_FIX
- Trên màn tiền Nếp bị đẩy ra mép và ở đó (luật «Nếp Không Chạm Số»). Mép ấy nay là trang trí: không role, không nhãn,
  không Tab stop, `aria-hidden`. Trước đó là nút «chạm để kéo ra» mà chạm không làm gì.

### «Tạo đợt thu từ sổ» chỉ mời khi máy chủ sẽ nhận (UI-058) · UX_IMPROVEMENT + Go
- Route Go mới, chỉ đọc: `GET /contexts/{context_id}/unbatched-expenses` → `{context_id, unbatched_expense_count}`.
  - Gói native `internal/gomdot`, theo mẫu `achievementv1`: không có hợp đồng Python, `python: absent`, `native: true`.
  - Phép chọn giống hệt `moneysteps.FreezeBatch` (bản mới nhất của mỗi khoản có phân bổ, chưa dòng thu nào làm nguồn cho
    nghĩa vụ), đọc không khoá dòng, trong giao dịch chỉ đọc. Chỉ một con số đếm, không đọc số tiền.
  - Thành viên mới đọc được (403 `is_group_member`, nhóm lạ cũng vậy); id sai 422; không cache.
  - Test: `repo` trên PostgreSQL (`TestCountUnbatchedExpenses`: 1 khoản ngoài đợt → 0 khi phần của Bình làm nguồn cho
    nghĩa vụ; bản không phân bổ và nhóm khác không tính), unit `gomdot` (đường dẫn, phương thức, id sai, không có sổ;
    người ngoài nhóm bị từ chối **trước khi** sổ được đếm).
  - Chạy thật trên stack riêng: thành viên 200 `unbatched_expense_count: 1`, không phiên 401, người ngoài 403, id sai 422,
    cả bốn `Cache-Control: private, no-store`.
- Màn quyết toán: 0 khoản chưa vào đợt thì nút ẩn, thay bằng câu «Mọi khoản đã ghi đều đã vào một đợt thu ở trên: chưa có gì
  mới để gom.». Có thì caption nói gom bao nhiêu khoản.

### «Đã trả» luôn thấy (UI-059) · UX_IMPROVEMENT
- Trang sổ «Đã ghi»: người trả có con dấu «Đã trả» cạnh tên (`DongSo nhan`), ngoài phần tên bị cắt. Trước đó «(trả)» nằm
  trong tên và mất cùng chữ cuối khi tên dài.

### Tài chính: tiêu đề nói đúng điều bên dưới (UI-060) · UX_IMPROVEMENT
- «Chi theo nhóm» → «Ai nợ ai». Wire `Finance` không có số theo nhóm; thêm số theo nhóm là đổi ngữ nghĩa đọc tiền, nên
  ghi thành việc sau (route Go tổng hợp chỉ đọc), không làm trong batch này.
- Sau finish review, mục này bỏ hẳn (điểm 10 dưới): «Ai nợ ai» chỉ lặp số «Sẽ nhận».

### Dòng đầu sổ quyết toán ở 320dp (UI-061) · VISUAL_UPGRADE
- Trạng thái không phải số («Chưa có chuyến») xuống dưới câu thay vì chiếm cột phải, nên câu không bị ép.

### Đánh giá thiết kế (luật của chủ sản phẩm 01/10): những chỗ yếu nhất của màn tiền, làm lại
1. **Bước «Kết quả» là một dải phiếu, có dòng cộng.** Trước: tám cuống phiếu rời giống hệt nhau, mỗi cái một mép răng
   cưa, một sọc màu, chip viền («Đã trả bill», «+lẻ đồng») trông như nút. Nay:
   - phần của bạn vẫn là cuống xé rời, nổi cao một lớp giấy;
   - phần của mọi người khác còn **trên một dải hoá đơn**, ngăn bằng đường đục lỗ;
   - dải kết bằng **gạch đôi và dòng cộng** «Cộng N phần · 13.705.678đ — Đúng bằng tổng bill: chia lẻ không làm mất hay
     thừa đồng nào.». Luật tiền số 2 được cho thấy, không chỉ hứa. Cộng lệch (không thể xảy ra với máy chủ đúng) thì số và
     câu đổi sang `warn`;
   - người trả nhận **con dấu «Đã trả bill» hạ xuống dưới tên** của đúng hàng khi chip bên phải chọn họ (đặt cạnh tên, nó
     ép «Phần của bạn» thành một chữ mỗi dòng ở 390);
   - chip «+lẻ đồng» và câu «Lẻ đồng dồn về: …» bỏ đi. Dòng cộng nói một lần «N phần được làm tròn lên 1đ để cộng lại
     đủ» (mỗi phần đúng 1đ: phần dư lớn nhất của allocator). Ghi dưới từng hàng thì nó lặp 6 trên 8 hàng.
   - Hàm thuần `dongCong` (chỉ cộng số nguyên máy chủ gửi), có test.
2. **Đợt thu: hành động của hàng, không phải tường nút** (sau finish review: gom theo người nhận, điểm 2 dưới). 19 nút đầy chiều ngang «Tiền đã về từ X» thành nút gọn «Đã về»
   **dưới số tiền** của hàng, chỉ ở hàng chưa về; «Gửi cho X» ×19 thành «Gửi»/«Gửi lại», cũng dưới số tiền. Lượt đo đầu
   để nút ở một dòng riêng, mỗi hàng còn cao ~290px. Tên đầy đủ nằm ở tên truy cập. Lỗi của một hàng
   nói dưới đúng hàng đó; lỗi phát dưới con dấu phát; lỗi làm mới cạnh «Làm mới».
3. **Quyết toán nói một lần.** «Đề xuất, chưa phải nghĩa vụ» lặp dưới mỗi hàng (19 lần ở chuyến 20 người) và lặp thêm ở
   ghi chú cuối, ở dòng đợt thu. Nay một câu trên danh sách: «Đề xuất tính từ sổ, chưa phải nghĩa vụ: nghĩa vụ chỉ có khi
   một đợt thu được phát.». Hàng gọn lại (64 → 56dp).
4. **«Bỏ món này» là hành động phá huỷ.** Trước: nút teal căn giữa dưới mỗi món, đọc như bước tiếp. Nay ghost tông
   `warn`, cuối hàng, tên truy cập «Bỏ món ‹tên›».

### Sau finish review (disposition `fix`): một đợt · VISUAL_UPGRADE + UX_IMPROVEMENT + BUG_FIX
Reviewer chạy ngữ cảnh mới (`impeccable-finish-reviewer`), chấm 14 ảnh của bản cuối và 6 ảnh ghép trước/sau, đối chiếu
DESIGN.md, direction.md, craft-floor. Giữ nguyên hai thứ reviewer nói đừng làm loãng: dải phiếu có dòng cộng, và tờ
«Bill đang gõ» làm nút tiếp tục.
1. **Trang sổ «Đã ghi» (UI-059 hồi quy):** trang hẹp cạnh Nếp nên dòng người trả còn «C… [ĐÃ TRẢ]». `DongSo` nay đặt
   con dấu ở dòng kẻ dưới tên (hàng cao đúng hai dòng kẻ, các hàng dưới vẫn nằm trên dòng kẻ). Số tiền trên trang sổ là
   teal (màu của tiền), chữ số đều. Câu «Đã ghi: …» lên trên trang, không còn nằm dưới 20 hàng.
2. **Đợt thu gom theo người nhận:** mỗi người được nhận một mục «Chuyển cho X · N lượt», dưới đó từng người chuyển.
   Bỏ hình nhỏ người nhận ở góc, bỏ «→ người nhận» lặp mỗi hàng. Hàng chưa về **không còn con dấu**. Con dấu chỉ còn
   cho điều đã xảy ra (đã về, về dư, thắc mắc…); hàng chưa về ghi một chữ «chưa về». «Đã về» là nút viền gọn dưới số
   tiền.
3. **Đầu trang đợt thu:** bỏ dòng mắt chữ hoa «AI CHUYỂN CHO AI» (sàn chất lượng cấm). Câu «N/M người đã xong phần mình»
   chỉ còn khi nó khác số lượt ở tiêu đề (một người có hai lượt). Giữ dải băng teal và Nếp M4.
4. **Bước gán món:**
   - hàng món không còn khối nền teal: món đang trên bàn nói bằng chữ «· đang trên bàn», teal chỉ ở số tiền;
   - nhóm từ 9 người: hàng mở ra không lặp 20 hình nhân của bàn dài, chỉ còn «Cả nhóm / Bỏ hết / Như món trên».
   - Nhóm nhỏ giữ lưới hình nhân (gọn, và là lối bàn phím QA đã đo).
5. **Bàn tròn:** thẻ món giữ một dòng tên, con dấu «Chia đều» là dấu đóng chéo ở góc thẻ, nằm ngoài chiều cao thẻ. Hai
   dòng tên cộng con dấu từng che bốn đĩa. Bàn dài vẫn hai dòng tên, vì thẻ ở đầu bàn có chỗ.
6. **Quyết toán:**
   - danh sách chuyển gom theo người nhận, cùng ngữ pháp với đợt thu, số tiền cỡ label thay cỡ display;
   - tiêu đề «Quyết toán chuyến đi» thành «Quyết toán» (ở 320 nó gãy ra một chữ «đi» mồ côi);
   - đầu sổ khi chưa có chuyến: dòng trạng thái tự nói «Chưa có chuyến đang đi hay đã xong», câu dưới còn một dòng (trước:
     bốn dòng ở 320 trước khi tới con số nào).
7. **Cuống «Phần của bạn» bỏ sọc màu bên trái** (sàn chất lượng cấm sọc màu cạnh). Xé rời và nổi cao một lớp giấy đã
   đủ nói đó là của bạn.
8. **Đọc ảnh tắt hẳn** (`receipt_reader_not_configured`, `permission_denied`):
   - «Nhập tay» thành nút chính của bước (có bill đang gõ thì «Tiếp tục bill đang gõ»);
   - «Chọn ảnh khác» rút đi như «Dùng ảnh này», vì ảnh khác cũng qua đúng bộ đọc đang tắt;
   - câu lỗi chỉ còn nói vì sao.

   Lỗi đọc tạm thời vẫn giữ «Nhập tay» cạnh câu.
9. **Bước xem lại chỉ ra đúng dòng:** khi «Tiếp» bị chặn, món có lỗi mở ra, tên món đổi màu `warn`, ô cần sửa mang câu
   ngay dưới nó («Đặt tên cho món này.» / «Gõ thành tiền của cả dòng.») và nhận con trỏ. Tên món dài sẽ gãy dòng thì
   không vẽ dấu chấm dẫn: một mẩu chấm từng trôi lơ lửng cạnh số tiền.
10. **Tài chính:** bỏ mục «Ai nợ ai». Câu của nó chỉ lặp số «Sẽ nhận» ngay trên. Dưới trang sổ còn một chú thích «Các số
    trên đọc từ sổ cái, không phải số dư ngân hàng.» và nút «Xem quyết toán». Sổ tay `tai-chinh.md` đổi bước theo.

**Thấy thêm trên Android (ngoài review): bill 0đ không bị chặn.**
- Bước xem lại để đi tiếp một món chưa gõ tiền, máy chủ từ chối (`line_total_vnd: PositiveMoneyVnd`), và màn nói «App
  gửi lên một yêu cầu không hợp lệ… lỗi của app»: đổ lỗi cho app về một số chưa ai gõ.
- `blockingProblem` của `receipt.ts` nay chặn món 0đ ngay ở bước này, sau món chưa tên: «Món "X" chưa có số tiền. Gõ
  thành tiền của cả dòng, hoặc bỏ món này.».
- Câu tương ứng ở bước gán món (`assignment.ts`, «Quay lại màn trước…») không còn tới được. Test của nó đổi có chủ đích.

**Cổng phụ đổi theo nhãn mới, cùng commit:**
- 6 flow Maestro: «Quyết toán», «Các số trên đọc từ sổ cái.*», «Chuyển cho .*», «chưa về»,
  «Đề xuất tính từ sổ.*»;
- `chuoi-maestro-dong.json` gạch «1.106.250đ» (nay có nguyên văn trong mã; danh sách chỉ được co lại);
- `mac-dinh-am-tham-id` thêm `daGoTruoc?.payerId ?? phien.person_id` vào danh sách «id dùng làm id», có lý do: trạng
  thái chọn người trả, mọi tên hiện ra đều lấy từ roster;
- `chia-bill-buoc.test.mjs` đọc thẻ `RudiScreen` có footer bọc `{nutChinh}` (footer nay mang câu chặn ngay trên nút);
- `nap_test.go`, `huong-dan-khop-ma.test.mjs`: câu bước mới của `tai-chinh.md`.

**Sổ tay truy hồi (`truy_hoi_test.go`):**
- Hai MRR ghim lại: `duongVang` 0.9179 → 0.9177, `[co_dau]` 0.8978 → 0.8972. Recall@5 không đổi ở mọi bộ, mọi nhóm.
- Viết câu bước không có «ở» thì «bo fieu o dau v» rơi khỏi top 5. Câu hỏi đó nằm cách mép đúng sức nặng của một token.

**Verdict pass** (cùng reviewer, ảnh chụp lại cùng khung nhìn trên thế giới dựng lại):
- **Resolved, 6/8:** đợt thu (hàng, đầu trang), bảng gán món, quyết toán, cuống không sọc, đọc ảnh tắt và bước xem lại.
- **Partial, 1:** tên trên trang sổ «Đã ghi» vẫn mỗi người một màu.
- **Unresolved và hồi quy, 1:** con dấu «Chia đều» đặt ở góc thẻ che đúng đĩa của một ghế và đuôi tên món; thẻ còn che
  nửa ba đĩa chéo.

Đợt chỉnh cuối, không chấm lại bằng reviewer mà kiểm bằng test hình học, ảnh và probe:
- **Bàn tròn:** độ sâu `ry` tính sao cho **cả chiếc đĩa** (14×7, đúng như vẽ) ra ngoài thẻ món ở bề rộng lớn nhất và
  chiều cao thật (`CAO_THE_TRON` 56: một dòng tên, số tiền, đệm).
  - Test `hinh-tien` nay so cả hộp đĩa, không chỉ tâm đĩa.
  - Luật chặt hơn bắt thêm một chỗ sai có từ trước: đĩa của cặp đôi ở 288 nằm nửa dưới thẻ khi thẻ nở 1.2×. Đĩa cặp đôi
    nay đặt qua mép thẻ ở bề rộng lớn nhất.
- **«Chia đều» rời khỏi bàn hẳn:** hàng món dưới bàn nói «Chia đều cả N người». Đĩa của mọi ghế đã cho thấy ai dùng món.
- **Trang sổ «Đã ghi»:** tên mực, số teal, như dải phiếu ở bước trước.
- **Đầu sổ quyết toán khi chưa có chuyến:** «Chưa có chuyến nào bắt đầu». Chuyến đang đi và chuyến đã xong đều đã bắt
  đầu, nên câu vẫn đúng ý QA 23/09, và vừa một dòng ở 320 (bản trước để «xong» mồ côi).
- **Đọc ảnh tắt hẳn:** câu còn hai, ba dòng mà vẫn chỉ lối đi tiếp, đúng tiêu chí QA UI-056 («một câu nói vì sao, và
  lối đi tiếp mà câu đó chỉ tới có ngay trên màn»): «Phần đọc ảnh bill của Rủ Đi chưa bật; ảnh của bạn không có lỗi.
  Bạn có thể nhập món bằng tay.».
  - Bản rút gọn trước đó bỏ mất vế chỉ đường, và `TC-F04-ANH-DOC` đỏ ở lượt đo.

## B5 · Khám phá (F02 + F10)

Chủ sản phẩm đang có spec làm lại đầu tab Khám phá (`claude/thanh-tab-5-cot`, `93fed2a5`, chờ duyệt, chưa code). Batch
này không đụng hàng tiêu đề, tiêu đề mục kết quả hay kiểu thẻ đầu. Nó chỉ sửa đúng các issue trong cấu trúc hiện có, và
giữ hai điều spec cũng đòi: giá không bao giờ bị cắt, tim nằm trên ảnh.

### Giá có dòng riêng, không bao giờ cắt (UI-021) · UX_IMPROVEMENT
- Hàng địa điểm và cặp so sánh coi phần tử **cuối** của danh sách sự kiện là giá. `chiTietNgan` lại đặt giờ mở sau
  giá, nên giá rơi vào dòng meta một dòng và bị cắt (8/8 dòng ở C1–C4), còn giờ mở chiếm dòng của giá.
- `tachGia` (thuần, có test) tìm giá theo loại (biểu tượng ví). Giá có dòng riêng, không `numberOfLines`, mực đậm hơn
  một bậc. Điểm, khoảng cách và giờ mở chung một dòng, gãy tối đa hai dòng (ba ở ô so sánh nửa màn).
- Mỗi mẩu của một sự kiện giữ liền: khoảng trắng không ngắt bên trong, dấu nối từ (U+2060) sau gạch «–». Vì vậy dòng chỉ
  gãy ở « · ». Lượt đo giữa chừng thấy «07:00 – / 22:00» ở hàng và «09:00 –…» ở ô so sánh 320, vì gạch ngang cho ngắt
  dòng sau nó kể cả giữa hai NBSP.

### Nhãn nút đọc trọn (UI-023) · UX_IMPROVEMENT
- Chân trang chi tiết quán: «Lưu địa điểm» → «Lưu» / «Đã lưu». Trên trang của chính quán thì một chữ là đủ; tên truy
  cập giữ đủ câu «Lưu địa điểm này».
- Hai nút xếp theo nhãn chứ không chia theo tỉ lệ 1 : 1.4: cùng một dòng khi cả hai vừa, mỗi nút một dòng đầy khi không
  (`flexGrow` + `flexShrink: 0` + `wrap`).
- Bản demo (F11) dùng cùng cách cho «Chỉnh lịch trình / Dùng plan này» và «Nhắc thành viên / Tôi đã tới».
- Sổ tay `dia-diem.md` và flow Maestro 26 theo nhãn mới. Câu sổ tay giữ cụm «lưu địa điểm» («Bấm «Lưu» để lưu địa
  điểm.»): bỏ cụm ấy thì câu hỏi «luu dia diem» rơi khỏi top 5 của bộ truy hồi; câu này giữ nguyên mọi chỉ số đã ghim.

### ✦ hỏi thật; câu hỏi không lọc theo tên (UI-024) · UX_IMPROVEMENT
- ✦ có câu trong ô thì gửi câu đó. Ô trống thì đặt câu mẫu, đặt con trỏ vào ô, và nói «Sửa câu cho đúng ý bạn, rồi chạm
  ✦ hoặc Enter để hỏi Rủ Đi AI.».
- Câu đang chờ gửi, hoặc câu đã hỏi, **không lọc danh sách theo tên** (`nenLocTheoTen`, thuần, có test). Trước đó câu mẫu lọc ra «0 kết quả / Chưa thấy nơi
  phù hợp» trước khi có câu hỏi nào.
- Tìm theo tên ra 0 mà ô trông như một câu (≥3 chữ): trạng thái rỗng mời «Hỏi Rủ Đi AI», «Xóa lọc» là lối phụ.
- `source: "none"`: «Rủ Đi AI chưa trả lời được câu này lúc này. Danh mục bên dưới vẫn đủ để bạn tự chọn.». Câu cũ đoán
  lỗi ở người hỏi («Thử nói rõ số người…») cho một câu có thể chẳng sai gì; máy chủ cố ý giữ lý do (`tim-kiem.ts`).
- `SearchField`/`Field` nhận `oRef` để màn đặt con trỏ (cùng mẫu `ONhapMuc`).

### Sân khấu thành phố: giữ chỗ trước, không dựng lại (UI-025, UI-026) · MOTION_UPGRADE
- Khung sân khấu mang tỉ lệ của chính bức vẽ (`aspectRatio`, rộng tối đa 480) ngay lần dựng đầu. Danh sách được xếp dưới
  nó một lần, không còn bị đẩy xuống 149dp khi bề rộng đo xong.
- Khi tìm hay lọc, sân khấu không bị gỡ: nó gập lại bằng chiều cao và độ mờ (`standard`), rồi đứng lại như cũ khi bỏ lọc.
  Không phát lại cú bật dựng. Giảm chuyển động thì cắt thẳng.

### Giảm chuyển động: một bức vẽ từ đầu, không khung trống (UI-027) · BUG_FIX + MOTION_UPGRADE
- **Giảm chuyển động:** `KhungSkia` chỉ vẽ bản SVG (cùng bức vẽ). Lớp Skia có để làm chuyển động; khi giảm chuyển động,
  lúc đổi sang Skia là một bức thứ hai của cùng thứ đó ~500 ms sau bức đầu, hoặc một khung trống khi canvas còn khởi động.
  - Nếp M3/M4/M5 đổi ảnh (`TC-MO13-M3` C9 FAIL ở B4).
  - Bảng dev trống ở 603 và 704 ms.
- **Chuyển động bình thường:** SVG ở dưới cho tới khi cú fade xong **và** lớp Skia đã có `GIU_SVG_MS` (1 giây) để vẽ
  khung thật đầu tiên.
  - Canvas trên web vẽ khung thật 430–700 ms sau khi mount (QA đo); hai `requestAnimationFrame` không phải khung đó.
  - `react-native-skia` 2.6 không có callback «đã vẽ» mà `KhungSkia` với tới được.

### Thành phố chưa có quán (UI-028) · UX_IMPROVEMENT
- «Hội An chưa có địa điểm nào», câu nói vì sao, nút «Đổi điểm đến». Không còn «Xóa lọc» khi không có bộ lọc.
- Dòng «Xếp theo mức chi bạn đã chọn.» không in trên một danh sách rỗng.

### Mất mạng không làm mất danh sách đã tải (UI-030) · UX_IMPROVEMENT
- Nạp lại lỗi khi danh sách đang có: giữ danh sách, một câu `CauTaiCho` «Chưa cập nhật được danh mục: …». Chỉ lần đọc đầu
  lỗi mới là màn lỗi.
- UI-029 (503 khác mất mạng) đã đạt từ B2: ảnh baseline cho hai câu khác nhau.

### Lưới Điểm đến 3 cột ở màn rộng (UI-031) · VISUAL_UPGRADE
- `cotDiemDen` (thuần, có test) dùng `gridFor` như các lưới thẻ khác: điện thoại vẫn 2 cột, từ 200dp mỗi thẻ thì 3 cột.

### Cặp so sánh: tim trong ô, không lồng nút (UI-113, UI-114) · BUG_FIX
- Không ảnh: biểu tượng và tim chung một dòng, dấu «HỢP GU» xuống dòng dưới. Cả ba chung một dòng đẩy tim ra ngoài mép
  ở 320.
- Có ảnh: tim nằm đè góc trên phải của ảnh nhưng **ngoài** nút «Mở …». axe báo `nested-interactive`.

### Kéo dọc trên tranh vẫn cuộn trang (UI-115) · BUG_FIX
- `GestureDetector` của sân khấu nghiêng mang `touchAction="pan-y"` trên web. RNGH mặc định `none` chặn cú cuộn bắt đầu
  trên tranh.
- Cùng mẫu cho sân khấu thử ở bảng dev và `KeoTab`: tab kéo ngang chừa `pan-y`, tab kéo xuống chừa `pan-x`.

### Chủ động: chi tiết quán nói mỗi điều một lần · UX_IMPROVEMENT
- Giá đã ở hàng sự kiện đầu trang, trạng thái mở là con dấu «Đang mở / Đã đóng» ở đó.
- Khối dưới mô tả chỉ còn khung giờ («Giờ mở cửa: …»), và câu «Chưa có giá» khi đầu trang không có giá. Trước đó giá và
  «Đang mở» mỗi thứ in hai lần.

## B6 · Chat (F05, N22, N26) — tham chiếu Messenger

Chat là chỗ «quen tay thắng biểu cảm» (Operate): giấy chỉ ở chất liệu, không được cản việc gõ. Batch này đưa luồng tin, ô
soạn và thẻ bình chọn về quy ước người dùng đã quen (Messenger, người yêu cầu đã chọn làm thước), và gom mọi câu lỗi
của chat về một ngữ pháp: một câu tại chỗ, bằng giọng của app, không bao giờ mặc áo AI.

### Nhịp của luồng tin: avatar theo đáy bong bóng, giờ theo quãng nghỉ (UI-064) · VISUAL_UPGRADE
- Hàng đo được (`chat-message-*`) giờ chỉ chứa avatar và bong bóng. Tên, trích dẫn và cảm xúc nằm quanh hàng, thụt theo
  cột bong bóng. Đáy avatar = đáy bong bóng cuối của cụm (trước: thấp hơn 22px, đứng ngang dòng giờ).
- Giờ là một **dải giữa luồng** ở nơi cuộc nói chuyện nghỉ: «Hôm nay · 13:27» ở đầu một ngày, «15:40» khi nói tiếp sau
  hơn 15 phút (`nhomTheoQuang`, thuần, có test). Trước: «13:27» lặp dưới mọi cụm, 5 lần cho 11 tin cùng phút.
- Giờ của một tin nằm trong menu tin («Gửi lúc 13:27 · hôm nay»), một chạm là thấy.
- Ngày tính theo giờ máy, không theo UTC: tin 06:30 ở Hà Nội là tin của hôm nay (`nhomTheoNgay` cũ cắt theo UTC).
- Cụm bong bóng thành một hình: phía cụm treo khít góc lại (6), phía ngoài giữ tròn (18), chân tin cuối của người khác
  tròn vì avatar đứng đó (`gocBong`). Các cụm cách nhau 12dp, tin trong cụm 2dp.

### Ô soạn: một dòng, cao dần, thẳng hàng (UI-062) · BUG_FIX
- Web: `rows={1}`, và ô tự đo chữ ở chiều cao 0 rồi nhận đúng chiều cao đó, kẹp giữa một dòng và trần 120dp (bốn dòng,
  đúng trần tiêu chí gỡ của QA; không quá 30% cửa sổ thấp), quá trần thì cuộn trong ô. Bản giữa chừng để trần sáu dòng
  (168dp) và trượt tiêu chí `≤ 122`; đã hạ về 120.
- Một dòng = 24 + 2×12 = 48dp, đúng bằng «+» và nút gửi, nên ba thứ chung một đường giữa. Nhiều dòng thì «+» và nút gửi ở
  đáy, như Messenger.
- Có bàn phím cứng (con trỏ «fine») thì Enter gửi, Shift+Enter xuống dòng; IME đang ghép chữ (Telex) giữ Enter của nó.
  Điện thoại vẫn xuống dòng bằng Enter.

### Link dài xuống dòng trong bong bóng (UI-063, UI-116) · BUG_FIX
- Web: chữ trong bong bóng `wordBreak: "break-word"`, cột bong bóng `minWidth: 0`. `overflow-wrap: break-word` mặc định
  vẫn lấy cả từ làm bề rộng tối thiểu, nên bong bóng giữ 341px ở mọi bề rộng và mất đầu link ở 320.
- Chat demo: khối tin `flexShrink: 1`, `minWidth: 0`.

### Thẻ bình chọn gọn (UI-065) · VISUAL_UPGRADE
- Mỗi lựa chọn một hàng 48dp: vòng chọn, tên, rồi tối đa ba dấu vân tay và con số. Dấu vân tay vẫn là đơn vị đếm
  (ADR-0037 D1), nhưng không còn chạy dài 12 dấu.
- Dưới mỗi tên một thanh mực mảnh trên **cùng một thang** (phần của tổng phiếu), `role=progressbar`: đọc được bên nào
  hơn mà không phải đếm.
- «N phiếu» một lần, ở chân thẻ. Chưa ai bầu thì không có dòng nào nói «0 phiếu». Người tạo có «Chốt bình chọn» cùng
  dòng chân.
- Thẻ là tin của người tạo: tên ở trên, avatar bên cạnh như mọi tin. Chữ ký chân thẻ chỉ còn ở bình chọn đã đóng.
- Trần 560dp ở tablet; các thẻ AI khác trần 640dp.

### Lỗi tại chỗ, bằng giọng của app (UI-068, UI-069) · UX_IMPROVEMENT
- Thả cảm xúc / xoá tin hỏng: một câu `CauTaiCho` **dưới đúng tin đó**, nêu đúng thao tác («Chưa thả ❤️: Rủ Đi đang trục
  trặc. Chưa có gì thay đổi.»), «Thử lại» chỉ khi bấm lại có ích (`cauLoiThaoTac`).
- Lỗi của ô soạn (ảnh, `/vote` sai dạng): một câu ở đầu mới của luồng, ngay trên ô soạn; luồng tự về cuối để câu hiện.
- Thẻ «Đã hiểu» mang ✦ và màu AI đã bỏ: không lỗi hệ thống nào còn trông như AI trả lời.
- Trang đầu không tải được: chỉ nói khi màn chưa có tin nào («Chưa tải được tin nhắn…» + «Thử lại»). Trang tin cũ hơn
  không tải được: câu ở đỉnh luồng, nơi tin cũ lẽ ra hiện, có «Thử lại».

### Báo cáo: năm lý do là radio thật (UI-067) · UX_IMPROVEMENT
- Mỗi lý do một hàng 48dp có vòng chọn mực, `role=radio` + `aria-checked`, Space chọn được trên web. Câu lỗi là
  `CauTaiCho`. Áp cho mọi nơi dùng `NoiDungBaoCao` (người, bài, tin, bình luận).

### Phòng đã dừng (UI-079) · UX_IMPROVEMENT
- Máy chủ trả 403 `chat_unavailable` cho luồng thay đổi: app thôi nối lại (`connection = "dung"`), nên «Đang nối lại»
  không còn hiện mãi.
- Không mời mở lời, không mời hẹn trong phòng đã dừng.
- Người chặn đọc danh sách chặn **của chính mình** và thấy «Bạn đã chặn X.» + «Xem danh sách đã chặn». Người bị chặn chỉ
  thấy «Cuộc trò chuyện này không còn nhận tin.» (ADR-0023 §2.3.2).

### Số thành viên theo kịp (UI-122) · BUG_FIX
- Đọc lại danh sách khi một tin mới tới từ người danh sách chưa tính là đang ở đây, không chỉ từ người lạ tên. Người được
  mời đã có tên từ lời mời, nên tin đầu của họ trước đây không đổi gì.

### Chat rỗng ở cửa sổ thấp (UI-124) · BUG_FIX
- Trang rỗng là phần co giãn của cột và tự cuộn; ô soạn luôn là khối cuối. Hình Nếp nhường chỗ trước khi cửa sổ thấp hơn
  600 hoặc khay đang mở.
- Khay công cụ co lại và tự cuộn thay vì đẩy ô soạn; câu ghi chú ảnh nằm **sau** lưới công cụ, nên khi khay bị ép thì câu
  ghi chú bị cắt trước, không bao giờ là một công cụ.
- Cửa sổ thấp (< 600) và khay đang mở: dải ghim (kèo sắp tới / tờ hẹn) gập lại, trở lại khi đóng khay. Lượt đo giữa chừng
  cho thấy harness đạt nhưng ảnh C8 của phòng cặp đôi chỉ còn icon, nhãn công cụ bị che; gập dải ghim trả lại chỗ cho nhãn.

### Lớp phòng không nhảy (UI-125) · UX_IMPROVEMENT
- Quay lại cùng một phòng giữ capabilities và lời gọi đang có trong lúc đọc lại; trước đây chúng về `null` mỗi lần focus và
  hàng ghim Tờ giấy tắt rồi bật (nhảy 78dp).
- Phòng hai người đọc lại capabilities mỗi 5 giây khi đang mở: người kia đồng ý «Một đôi» thì hàng ghim và công cụ thứ năm
  tới mà không phải rời chat.

### Chữ của phòng hai người (UI-128) · UX_IMPROVEMENT
- Nút và sheet cài đặt: «Cài đặt cuộc trò chuyện» (nhóm giữ «Cài đặt nhóm»); màu bong bóng «Hai bạn thấy cùng một màu.»;
  «Rủ Đi AI tự gợi ý» nói cho hai người.
- Trạng thái rỗng: «Rủ đi một buổi».
- Form «Kèo mới» từ chat hai người: «Hai bạn đi đâu?», bỏ câu «X hiện có 2 người».
- Maestro 47 và sổ tay `to-giay.md` theo nhãn mới, cùng commit. Quyết định giữ nhãn cũ ghi ở
  `docs/claude/2026-09-28/chay-may-that.md` §6 được đảo ở đây, có Maestro đi cùng.

### Tấm «Gu của hai bạn» (UI-129) · UX_IMPROVEMENT
- Gu bật theo lời cũ (`can_bat_lai`): dòng của mình chỉ nói phần sổ («Minh thấy gu của bạn, và Nếp dùng nó khi phác
  tờ.»), để dòng «Bật lại cho chat» nói phần chat. Không đổi lời đồng ý, không đổi luồng ADR-0048 §3.2.
- Câu lỗi nằm **trong tấm**, nói công tắc đang ở đâu (`cauLoiGu`). Bật lại hỏng ở bước hai: «Bật lại chưa xong: gu của bạn
  đang tắt…», không mượn câu «Chưa có gì bị ghi sai» vì bước tắt đã ghi.

### AI trong chat (UI-164, UI-165, UI-167) · UX_IMPROVEMENT + VISUAL_UPGRADE
- Khối «Đang hỏi Rủ Đi AI…» chỉ hiện khi lệnh sẵn sàng, đúng như chip vừa nói.
- Vùng thông báo: dòng «đang đọc / đang nghĩ», câu lỗi của Nếp, khối «Đang hỏi», hàng lời nhờ hỏng là `polite`. Chữ hiện
  dần được báo **một lần khi xong**: web giữ `aria-busy` trong lúc chạy; native chỉ thành vùng live khi đã xong
  (`vungSong`).
- Chip: ✦ và câu là một khối không bao giờ tách dòng; hai nút là khối riêng, xuống dòng khi chật. Câu chưa sẵn sàng ngắn
  lại «AI chưa sẵn sàng · gửi như tin thường» (tên đọc vẫn đủ câu cũ). «Xem» và «Chỉ gửi lời nhờ» là đích 48dp thật
  (trả lại 10dp trên dưới) thay cho `hitSlop`, thứ web không có.

### Chủ động
- Tấm «Xem»: bản ghi gom theo người, bong bóng nhỏ, của mình bên phải; câu «Rủ Đi AI sẽ đọc đúng N tin…» đứng yên trên
  cùng trong lúc bản ghi cuộn (`gomTheoNguoi`, có test). Vẫn liệt kê đúng từng lượt của gói.
- Một khuôn cho lời nhờ đang chờ hoặc hỏng (`HangLoiNho`): lời gọi không tới máy chủ và lời gọi hỏng là cùng một tin cho
  người hỏi. «Thử lại lời nhờ» tắt thì nói vì sao (ADR-0038).

### Vòng soát hoàn thiện (subagent ngữ cảnh mới) · VISUAL_UPGRADE + UX_IMPROVEMENT
Vòng đầu trả «fix» với 8 mục; vòng chấm thứ hai còn 3 mục một phần và một hồi quy; vòng ba «ship» (phạm vi: các mục đó).
- Nút «Tin mới nhất» nổi trên danh sách (`position: absolute`), không lấy 60dp của nó; luồng thôi nhảy khi nút hiện/ẩn.
- Câu lỗi dưới một tin dùng biến thể gọn `CauTaiCho co="nho"`: cỡ note, «Thử lại» ngay sau câu (đích 48dp trả lại chiều
  cao), treo phía phải dưới tin của mình, ở cột bong bóng dưới tin người khác.
- Tiêu đề chat dưới 360dp chỉ ghi số thành viên; «· sổ hẹn của hội» từng xuống một dòng riêng làm đầu màn cao bốn dòng.
- Tấm «Xem»: lượt của mình tông của tin mình (accent-soft), tím chỉ cho AI.
- Thẻ không ai trong phòng gửi (của AI, tờ hẹn chung) căn giữa cột đọc trên tablet, đầy màn trên điện thoại; giờ của chặng
  trong thẻ chat thẳng mép với tiêu đề (`HangChang sat`, màn plan không đổi).
- Cửa sổ thấp, khay mở: hàng tờ giấy và dải ghim cùng gập; trang rỗng không vẽ chữ (trước đó «Một» bị cắt mất dấu nặng
  khi bị ép); trần khay tính cả câu ghi chú, phần cuộn không bao giờ hẹp hơn một hàng công cụ.
- Menu tin: ngày trước giờ như dải giờ («Hôm nay · 11:08»); «Trả lời», «Sao chép» không mũi tên (`ListRow chevron`).
- Trang đầu không tải được: hết phiên thì «Đăng nhập lại» (về đúng chat sau khi vào), bị từ chối vĩnh viễn thì «Về Tin
  nhắn», còn lại «Thử lại». `cauLoiThaoTac` không mời thử lại với 401.

## B7 · Cộng đồng (N14) + Go

Cộng đồng là bề mặt «Experience» nhưng các issue của nó là việc nền: bảng tin không được báo lỗi khi chỉ là chưa có bài,
không được dựng lại dưới tay người đọc, câu lỗi phải nằm chỗ người vừa bấm. Batch này sửa ba route Go (chỉ Go phục vụ,
`python: absent`, không có oracle) kèm test trên PostgreSQL thật, và làm lại phần trạng thái của các màn Cộng đồng. Không
đổi luật duyệt, không đổi điều kiện ai đọc được gì.

### Bảng tin rỗng là bảng tin rỗng (UI-132) · BUG_FIX (Go)
- `candidates.go`: bảng xếp hạng chung được chép bằng `append([]string(nil), …)`, nên khi chưa có bài công khai nào được
  duyệt nó là `nil`; `nil` xuống `community_feeds.post_ids` thành NULL, cột NOT NULL từ chối, và «Dành cho bạn», «Thịnh
  hành» trả 503 `community_unavailable` cho một cộng đồng chỉ là chưa có bài. Nay chép thành mảng rỗng; `feed.go` thêm
  một chốt trước khi ghi snapshot.
- Test PostgreSQL: `TestPostgresCommunityEmptyDiscoveryIsAnEmptyFeed` (for_you, trending → 200, 0 bài).
- Màn: trạng thái rỗng sẵn có («Một ngày đáng kể» + «Kể khoảnh khắc đầu tiên») nay hiện đúng chỗ của nó.

### Chủ đề nói điều cần sửa, trước khi gửi (UI-133) · UX_IMPROVEMENT
- `chu-de.ts` (thuần, có test) kiểm chủ đề như `normalizeTopics` của máy chủ: tối đa 5, mỗi chủ đề 2–40 ký tự, không
  `/ \ < > @`. Câu nằm ngay dưới ô «Chủ đề» trong lúc gõ; «Gửi» chờ kèm lý do.
- `too_many_topics`, `invalid_topic` có câu riêng; không còn «lỗi của app». Lỗi máy chủ của ô soạn nằm ngay trên nút gửi.
- Nút gửi tắt luôn nói vì sao (UI-091); form theo cột 560 trên tablet (UI-093).

### Nối lại không dựng lại (UI-134, UI-135) · BUG_FIX
- Chi tiết bài: khung `sync` đọc lại bài tại chỗ; bài, chữ đang gõ trong ô bình luận và sheet Nếp còn nguyên.
- Bảng tin: quay về từ một bài giữ danh sách, vị trí cuộn và bài đang mở hết; chỉ đổi tab/chủ đề mới bắt đầu lại.
  Trên web, trình duyệt đưa danh sách bị che về đầu; màn ghi vị trí trong lúc đang ở trước mặt và đặt lại khi quay về.
- Luồng sự kiện: khung `sync` đầu tiên là lúc mở kết nối, không phải tin mới, nên không bật dải «Bảng tin có cập nhật»;
  chỉ một lần nối lại (có thể đã lỡ thay đổi) hay `feed.changed` mới bật. Lượt đo đầu của B7 cho thấy dải hiện ngay khi
  mở tab và đè đích bấm của thẻ đầu.
- Bình luận: đọc lại không làm trống danh sách.

### Lỗi nằm chỗ vừa bấm (UI-138, UI-148) · UX_IMPROVEMENT
- Gọi Nếp hỏng: câu trong sheet Nếp, kèm «Thử lại»; nút gọi tắt khi chưa có lời nhờ thì nói vì sao.
- Sheet quản lý bài: bước bị từ chối nói trong sheet.
- Bình luận: đọc hỏng có «Thử lại» và giữ những bình luận đang hiện; gửi hỏng nói ngay ô bình luận.

### Thẻ bài (UI-139, UI-140, UI-143) · UX_IMPROVEMENT + BUG_FIX
- Thân không bị cắt thì lần chạm đầu mở bài (`thanBiCat`, cùng phép thử in «Đọc tiếp»).
- Theo dõi / bỏ theo dõi đổi mọi thẻ của cùng tác giả (`doiTheoDoiTacGia`).
- Khung ảnh theo bề rộng album thật (trần 296, giữ tỉ lệ); dải «Bảng tin có cập nhật» nằm dưới phần đầu màn theo chiều
  cao đo được, không ở `top: 136` cố định.

### «Không quan tâm» lấy lại được (UI-141) · UX_IMPROVEMENT (Go + màn)
- Thẻ nhường chỗ cho một dòng tại chỗ: «Đã ẩn bài của X…» + «Hoàn tác». Không toast, không biến mất không một lời.
- Go: mode `hidden` của bảng tin liệt kê đúng bài người đó đã ẩn; sheet «Bảng tin của bạn» có «Bài đã ẩn», mỗi bài có
  «Bỏ ẩn». Test PostgreSQL: `TestPostgresCommunityHiddenModeListsAndRestores`.
- «Xóa lịch sử đề xuất» hỏi trước (nói cái gì mất, cái gì giữ), xoá xong nói đã xoá.

### Tác giả không mất bài của mình khi sửa (UI-142) · BUG_FIX (Go)
- `feed.go` bỏ điều kiện giấu bài đang chờ duyệt bản sửa khỏi bảng tin của chính tác giả. Bài hiện như ở trang chi tiết:
  chữ mới nhất của tác giả dưới dải «Đang chờ duyệt»; người khác vẫn chỉ thấy bản đã duyệt.
- Test PostgreSQL: `TestPostgresCommunityAuthorKeepsEditedPostInFeed` (gồm vế người đọc khác không thấy bản sửa).

### Hàng duyệt, tìm, ghi chép, chủ đề (UI-144, UI-145, UI-146) · UX_IMPROVEMENT
- Hàng duyệt: «Chờ duyệt», «Cần người xem lại»… thay mã thô; lỗi quyết định nằm dưới đúng mục; nút tắt nói vì sao.
- Tìm ra 0: «Không tìm thấy chủ đề hay người nào khớp «q»» + gợi ý; tiêu đề nhóm chỉ in khi có mục.
- «Điều mình muốn giữ» rỗng: nói ghi chép tới từ đâu và mở bằng gì.
- Trang chủ đề: «Quay lại», «Theo dõi chủ đề» / «Đang theo dõi» tại chỗ; không còn hàng tab của bảng tin chính.

### Thông báo nói ai nhắc, ở đâu (UI-147) · UX_IMPROVEMENT (Go + migration + màn)
- Migration cộng đồng thứ 5 `notification_source.sql`: `actor_id`, `comment_id` cho `community_notifications`. Lời nhắc
  trong bài ghi tác giả; lời nhắc trong bình luận ghi người viết bình luận và bình luận đó. Dòng cũ để NULL, không đoán.
- Route trả thêm `actor`, `from_comment`, `excerpt` (120 ký tự của bình luận, hoặc của bài như đã công bố).
- Màn: «Lan nhắc bạn trong một bình luận» + vài chữ trích; chuông ở đầu tab Cộng đồng có chấm khi có thông báo mới hơn lần
  xem trên máy này. Test PostgreSQL: `TestPostgresCommunityNotificationNamesWhoAndWhere`.

### Đầu màn Cộng đồng giữ một dòng · VISUAL_UPGRADE
- Chuông thông báo là ô 48dp thứ ba cạnh tiêu đề. Ở 390dp nó đẩy dòng phụ xuống hai dòng («nối» đứng một mình); ở 320dp nó
  bẻ «Cộng đồng» làm đôi. Bắt được trên ảnh lượt đo đầu, không phải từ bảng số.
- Dòng phụ nay chạy hết bề ngang dưới hàng tiêu đề. Dưới 360dp, hàng nút lên trên, căn phải, tiêu đề lớn nằm dưới, như
  thanh tiêu đề lớn của ứng dụng hệ thống. Thứ tự đọc vẫn là tiêu đề trước.

### Bình luận và bản sửa nói rõ (UI-096, UI-091 gặp lại ở N14) · UX_IMPROVEMENT
- «Xóa» dưới bình luận của mình hỏi ngay tại hàng: «Xóa bình luận này? Không lấy lại được.», rồi «Xóa» (tông warn) và
  «Thôi». Tên truy cập nói xoá bình luận nào. Xoá hỏng thì câu nằm dưới đúng bình luận đó, kèm «Thử lại», không rơi xuống
  ô gửi.
- «Gửi bình luận» tắt thì nói vì sao: chưa có chữ, hoặc ảnh còn đang tải.
- Sửa bài: chỉ hiện người đọc hiện tại (khoá), kèm câu «Bản sửa giữ người đọc như lúc đăng…» chỉ ra «Thêm lựa chọn cho
  bài» (⋯) ở trang bài. Bỏ «Lựa chọn khác» và danh sách nhóm, vì không lựa chọn nào trong đó dùng được khi sửa.

### Câu lỗi là một `alert` đọc lịch sự · DESIGN_SYSTEM_IMPROVEMENT
- `CauTaiCho` và câu lỗi dưới `Field` mang `role="alert"` cùng `aria-live="polite"`: công nghệ hỗ trợ (và bộ đo) tìm được
  chúng như câu lỗi, mà vẫn không cắt ngang điều người dùng đang gõ hay đang nghe. Trước đó Cộng đồng có 10 chỗ
  `role=alert` tự viết; nay câu lỗi tại chỗ của mọi màn cùng một vai.

### Test hành trình web hết chập chờn · BUG_FIX (test)
- `tests/rudi-hanh-trinh-web.test.mjs` chờ đúng trạng thái thay vì chờ thời gian: bước «về lịch trình» chờ
  `che-do-lich-trinh` mang `aria-selected="true"`; bước tô sáng tìm trong các nút đang thấy (`getClientRects` +
  `checkVisibility`) một nút mang tên chặng và `aria-pressed="true"`. Phiên song song báo test này chập chờn trên `main`.

### Chưa làm trong B7, chuyển batch
- Gộp hai hệ bình luận (Cộng đồng và trang tường kể chuyện) về một ngữ pháp hiển thị: chuyển sang B9, cùng lúc với UI-157,
  UI-158, UI-159 ở `BaiChiTietScreen`, để chỉ chạm màn đó một lần.

## Tích hợp PR #664 (Codex) · 04/10

Trong lúc đợt này dừng vì hết lượt, Codex làm 13 ID và để PR #664 ở dạng draft. Danh sách ID nằm ở đầu `feature-plan.md`;
bằng chứng của Codex ở `docs/codex/2026-10-02/`. Codex dành lại 44 ID của B8/B9 cho đợt này để hai bên không làm trùng.

- `a0fb9ba3`: gộp `codex/ui-ux-qa-handoff` vào `main` (`d3f74730`). Bảy xung đột được giải theo bảng của Codex và theo
  hướng production của `4f74b011`:
  - Không dựng lại demo: `Discovery.tsx` và test hành trình demo vẫn bị xoá.
  - `NepBang` đọc `actorId` từ phiên live.
  - `mobile_native.sh` giữ phần chọn Metro host, bỏ phần fixture.
  - Sổ tay hướng dẫn được sinh lại từ mã nguồn.
  - Test gutter của Nếp chuyển sang danh mục live.
  - Trạng thái «Chưa mở được trang cuối» không có nút thử lại của `EndingScreen` nay có cảnh `chua-doc-duoc`, đúng cổng
    «ô rỗng phải có cảnh».
  - Phần trùng của đợt này ở trang cuối (`cannot_end_reason`, «chưa tới ngày») được bỏ; bản Codex giữ nguyên.
- `76637302` · BUG_FIX (Go + migration): xoá tài khoản nay xoá luôn dấu «ai nhắc» trong thông báo Cộng đồng
  (`community_notifications.actor_id`, cột thêm ở B7 với UI-147).
  - Migration cộng đồng thứ 6 `notification_actor_erasure.sql` thay `community_person_erasure()`. Không sửa migration đã
    chạy.
  - Đăng ký cột trong `nepnho/dangky.go`.
  - Test PostgreSQL `TestPostgresCommunityErasureForgetsWhoMentioned` chạy trong một transaction rồi rollback, vì DB của
    tầng này dùng chung.
  - Phiên thanh tab phát hiện lỗi này.

## B8 · Nhóm · Người · Sổ hai người (F06, F07, N26) + Go

Đây là các màn «Operate»: nhóm, bạn bè, hồ sơ người, sổ hai người. Lỗi ở đây chủ yếu là màn nói sai điều vừa xảy ra:
- lập nhóm xong bị đưa về Khám phá;
- lời mời không nói ai mời;
- mọi quản trị đều mang nhãn «Người lập nhóm»;
- tải lại thì mất dấu «Đã chặn»;
- một lời đồng ý trong sổ đôi bị vẽ lại thành lời mời.

Batch này thêm hai route Go (chỉ Go phục vụ, `python: absent`) cho lời mời vào nhóm. Phần còn lại là sửa ở màn. Không đổi
luật ai được vào nhóm, ai đọc được danh sách thành viên, và không đổi luật tra số điện thoại.

### Lập nhóm xong là vào nhóm (UI-071) · UX_IMPROVEMENT
- «Mở nhóm» chọn nhóm vừa lập (`chonNhom`) rồi vào thẳng chat của nhóm, không về Khám phá.
- Chat của nhóm chỉ có mình bạn không phải màn rỗng câm. Tiêu đề là «Hội mới, mới có mình bạn.», kèm «Mời bạn vào nhóm»
  (vào danh sách thành viên, nơi có lời mời) và «Rủ hội một buổi».
- Hai flow Maestro mới, `_vao-nhom-vua-lap.yaml` và 24/39, đi theo đường mới.

### Lời mời nói ai mời, và từ chối được (UI-080) · UX_IMPROVEMENT (Go + màn)
- Gói Go mới `loimoi`, hai route mới:
  - `GET /contexts/{id}/invitation`: tên nhóm, người mời, số người đang ở trong, lúc mời.
  - `DELETE /contexts/{id}/invitation`: đổi hàng `invited` của chính người đó sang `left`.
- Người được mời không đọc được danh sách thành viên, nên tên người mời phải đi qua route riêng. Không thêm trường vào
  `GET /people/me/contexts`, vì Python vẫn là oracle của route đó (ADR-0031).
- Từ chối là cùng câu `UPDATE` mà `leave_context` dùng, chỉ khác điểm xuất phát là `invited`. Một lời mời sau đó tạo hàng
  mới (partial unique index chỉ áp lên các hàng đang mở).
- Đã có:
  - test đơn vị;
  - test PostgreSQL: đọc, từ chối, mời lại, người ngoài 404, mã lỗi;
  - hai hàng `routes.json` kèm evidence;
  - khai trong `GO_FEATURE_HANDLERS`;
  - `nativeRouteIDs`.
- Màn Tin nhắn:
  - Lời mời xếp đầu danh sách, đọc «Chat Test 01 mời bạn · 2 người trong nhóm».
  - Hai nút «Đồng ý vào nhóm» và «Từ chối» nằm cạnh nhau. «Từ chối» hỏi lại ngay trên hàng.
  - Trả lời hỏng thì câu lỗi nằm dưới đúng hàng đó; danh sách vẫn giữ nguyên.

### Người vào bằng lời mời đi qua Sở thích, có ô tên (UI-073) · BUG_FIX (phần UI)
- Đăng nhập xong, người được mời đi qua Sở thích (`manSauDangNhap` thêm `?moi=1`).
- Màn hiện cái tên nhóm đang gọi họ, đã điền sẵn: «Nhóm mời bạn đang gọi bạn là «…». Sửa nếu bạn muốn được gọi khác.»
- Giữ nguyên tên thì không ghi gì.
- Luật «tên do người mời đặt khi người lạ tra số» vẫn chỉ ở dạng ADR đề xuất (`adr-de-xuat/UI-073-…`, đã cập nhật).

### Quản trị và thành viên (UI-074, UI-075, UI-076, UI-081) · UX_IMPROVEMENT + BUG_FIX + VISUAL_UPGRADE
- Tự bỏ quyền quản trị phải hỏi lại ngay trên hàng: «Bỏ quyền» (tông `warn`) hoặc «Thôi».
- «Người lập nhóm» chỉ còn ở người đã lập nhóm (`created_by_id` đọc từ chính nhóm). Quản trị khác mang con dấu
  «Quản trị»; người được mời mang «Đã mời, chưa đồng ý».
- Mỗi nút có tên truy cập riêng, ví dụ «Đặt Chat Test 07 làm quản trị», không còn 19 nút cùng tên. Chạm vào hàng thì mở hồ
  sơ người đó.
- Thêm primitive `ui/LuoiNguoi.tsx`:
  - Điện thoại: một cột, kẻ tóc bắt đầu từ cột chữ.
  - Chỗ đủ hai cột ≥280dp (cột đọc của tablet): hai cột, nút nằm dưới tên.
  - Bề rộng lấy từ chính danh sách, không lấy từ cửa sổ.
  - Trước đây nút cách tên 487–679px.
- Màn Thành viên và Bạn bè dùng `LuoiNguoi` theo cột đọc `doc`.

### Bạn bè và hồ sơ người (UI-077, UI-078) · UX_IMPROVEMENT + BUG_FIX
- «Đồng ý» hay «Nhắn tin» hỏng thì câu lỗi nằm dưới đúng hàng, kèm «Thử lại». Danh sách đã đọc không bị thay bằng màn lỗi.
- Hồ sơ người đọc thêm danh sách chặn của chính mình. Sau khi tải lại vẫn thấy dấu «Đã chặn» ở chỗ quan hệ, cùng
  «Bỏ chặn»; không còn mời «Kết bạn» và «Chặn» lại.

### Sổ hai người (UI-083, UI-084, UI-085, UI-086, UI-090, UI-092, UI-126, UI-127, UI-130) · BUG_FIX + UX_IMPROVEMENT
- Đọc sổ lỗi thì hiện `ErrorState` kèm «Thử lại». Trước đây là «Chưa có sổ» kèm lời mời lập sổ lại (UI-083).
- Vừa được đồng ý (UI-084):
  - Sheet «Lập sổ» đóng trên cả hai máy trước khi vẽ khung (`useLayoutEffect`), không còn một hai khung mời lại.
  - «Đồng ý bậc» và «Loại sổ» giữ nguyên điều chúng vừa nói trong lúc đóng (`useGiuKhiDong`, primitive mới).
- «Rủ … tới đây» (UI-085, UI-130):
  - Tuần đã chốt hẹn không mời phác thêm tờ (`nenXinTo` bỏ tuần `chot`). Quán đi vào kèo đã hẹn qua «Thêm vào kèo».
  - Hai người bạn không phải «Một đôi» thì quán mở một kèo của hai người, quán là chặng đầu. Thêm prop `placeId` cho
    `CreateOutingLive` và `/outings/new`.
- Người đề nghị đóng sheet vẫn thấy «đang chờ» trên màn (UI-086).
- Hàng mời «Một đôi» dẫn tới màn nói về lời đề nghị, kể cả đề nghị bật đôi (UI-126).
- Bìa sổ: tên xuống hai dòng, không cắt «Chat Tes…» (UI-090).
- Lá ngày: cuộn tới lá đang chọn, kể cả ngày xa (UI-092).
- Khoảnh khắc M6 «sổ mở» diễn một lần, ở bước mở sổ, trong cả nhánh «hội» (UI-127). Giảm chuyển động thì hiện khung cuối.
- Không còn `DemoBadge` trong không gian giấy live.

### Phân biệt việc phá huỷ với việc tạm hoãn · VISUAL_UPGRADE (chủ động)
- «Tờ lời rủ»: hai việc phá huỷ («Bỏ bản phác này», «Huỷ buổi này») mang tông `warn`, không còn trông giống «Tuần này nghỉ».
- «Xác nhận việc» nhận cờ `nguyHiem`: nút xác nhận một việc phá huỷ mang tông `warn`; tờ xác nhận vẫn là nơi nói ra hậu quả.

### Sửa sau lượt đo đầu của B8 (cùng batch)
- **Hồi quy B8 đo được, đã sửa: cửa «Mở sổ cặp đôi» thường trực.**
  - Lượt đo đầu cho thấy B8 (UI-126) đã giấu nút «Mở sổ cặp đôi» khi người kia đề nghị «Một đôi», thay bằng con dấu «Xem
    lời đề nghị».
  - Lối quen của người dùng, flow Maestro 47 và harness QA đều đi qua nút đó, nên chuỗi đồng ý đứt: `CHUYEN-DONG-Y`,
    `CHUYEN-BEN-KIA`, `CHUYEN-CHAT-TOI` tụt từ PASS xuống FAIL.
  - Nay cửa thường trực luôn hiện, cùng tên ở mọi trạng thái; con dấu là lối tắt theo ngữ cảnh. Cả hai mở cùng một
    sheet. Đo lại: ba hàng PASS.
- **UI-073, lỗi thật: người được mời không qua Sở thích.**
  - Luật của B8 đọc `membership_state` của phiên, nhưng phiên đăng nhập bằng OTP không gắn nhóm nào (`null`); lời mời
    nằm trong `contexts`. Test của B8 dựng phiên theo giả định sai, nên xanh mà luật không bao giờ chạy.
  - Nay `laVaoQuaLoiMoi` đọc `contexts` (có lời mời đang chờ, chưa ở nhóm nào). Màn Sở thích đọc điều đó từ phiên,
    không từ cờ `?moi=1` trên URL, vì cờ đó ai cũng thêm hay bớt được.
  - Test dựng đúng dạng phiên mà máy chủ trả.

## B9 · Kỷ niệm · Sổ chuyến đi · Hồ sơ kể chuyện · Cài đặt (F08, F09, N15, N21) + Go

Đây là các bề mặt «Experience»: tường nhóm, album, story, sổ hành trình, hồ sơ kể chuyện. Cùng batch còn có các màn
«Operate» của Cài đặt. Phần lớn lỗi là mất thứ người dùng đang làm hoặc đang đọc:
- rời màn đăng là mất ảnh;
- tường cắt về trang đầu dưới tay người đọc;
- câu lỗi rơi xa chỗ vừa chạm;
- ngày viết ba kiểu.

Batch có ba thay đổi Go, đều ở route chỉ Go phục vụ (`python: absent`) và đều kèm ca PostgreSQL thật:
- ảnh bài Cộng đồng lên tường;
- tên trang đọc được trong bài dựng từ sổ;
- dấu «đã thấy» huy hiệu theo tài khoản (route mới).

Codex đã làm UI-150, 151, 152 và 154 (B9a, trang cuối) trong PR #664. Batch này không làm lại các mục đó; phần trang cuối
ở đây chỉ dùng tên trang đọc được cho sổ đã lưu.

### Cài đặt và tài khoản (UI-015, UI-107, UI-108, UI-109, UI-110, UI-111) · BUG_FIX + UX_IMPROVEMENT
- Cài đặt không vẽ giá trị giữ chỗ («Bạn», «B», công tắc sai) rồi mới đổi. Tên đọc từ phiên, công tắc chờ máy chủ trả
  lời (UI-015).
- Công tắc lưu hỏng thì câu lỗi nằm ngay dưới chính công tắc đó, không ở cuối trang (UI-107).
- Panel trong Cá nhân («Chỉnh hồ sơ», «Đã lưu»):
  - Back đóng panel và ở lại tab, không rời tab (UI-108).
  - Chữ đang gõ trong «Chỉnh hồ sơ» còn nguyên sau Back.
  - Primitive mới `ui/useLuiLop.ts`: Back đóng lớp mà màn tự vẽ (BackHandler Android + lịch sử web).
- «Đã lưu» liệt kê từng chỗ theo tên, mỗi chỗ là lối tới nó, thay cho một con số (UI-109).
- Xoá tài khoản (UI-110):
  - Nhận «XOÁ» có dấu hoặc không dấu.
  - Nút tắt nói lý do (`lyDo`).
  - Back ở bước 2 về bước 1, không rời trang.
- Câu cuối Cài đặt chỉ đúng chỗ đổi tên, là «Chỉnh hồ sơ» (UI-111).

### Tường nhóm, ảnh, story (UI-094, UI-095, UI-097, UI-098, UI-101, UI-102, UI-105) · BUG_FIX + UX_IMPROVEMENT + MOTION_UPGRADE
- Viewer ảnh trên web (UI-094):
  - Ô ảnh có chiều cao; trước đây ảnh và vùng cử chỉ cao 0px.
  - Bộ đếm «2/3» theo `onScroll`.
  - Vuốt dừng đúng một ảnh (`scrollSnapStop`).
  - Chụm hai ngón không phóng cả trang (`touchAction`).
- Đóng viewer mờ dần như lúc mở (UI-098).
- Thả tim hay bình luận hỏng thì câu lỗi nằm dưới đúng khoảnh khắc đó, không ở đầu tường (UI-095).
- «Thả khoảnh khắc» và «Đăng story» giữ bản nháp (ảnh và chữ) khi người dùng rời đi, bằng Back hay đổi tab (UI-097).
  - Module mới `ky-niem/nhap-dang.ts`, giữ trong bộ nhớ suốt đời app, một bản cho mỗi màn, người và nhóm.
  - Đăng, «Bỏ ảnh» hay «Bỏ bản nháp» mới bỏ bản nháp.
- Xem trước vẽ ảnh như tường vẽ (`cover`, cùng khung). Ảnh 9:16 thấy cùng một vùng ở cả hai nơi (UI-102).
- Tablet: tường theo cột đọc, ảnh không cao hơn cửa sổ (UI-105).
- Trình xem story (UI-101):
  - Hai vùng chạm «Story trước» / «Story tiếp theo» là nút, nên axe không còn báo `aria-prohibited-attr`.
  - Câu hỏi xoá hiện ra thì focus vào «Giữ lại», lựa chọn không làm mất gì; giữ lại thì focus về «Xoá story».
  - Hai vùng chạm ngủ trong lúc câu hỏi mở.
  - Nút tròn và nút chữ cao 48dp.

### Album nói ngày của chuyến (UI-103) · UX_IMPROVEMENT
- Kệ album và đầu album ghi «28 - 29/09 · đang đi · 20 người». Năm chỉ ghi khi không phải năm nay; trước đây chỉ có
  «2026».
- Route album vẫn có Python làm oracle (`python: live`), nên ngày không được thêm vào wire của nó (ADR-0031). Ngày lấy từ
  danh sách kèo của nhóm, đúng danh sách tab Lên plan đọc. Đọc hỏng thì giữ nhãn năm của máy chủ.
- Thêm hàm `khoangNgayChuyen` vào `ngay-viet.ts`.

### Một cách viết ngày (UI-104) · VISUAL_UPGRADE
- `ngay-viet.ts` (`ngayVN`, `gioNgayVN`): «28/09/2026», «10:07 · 28/09».
- Áp cho: tường nhóm, Đã chặn, Phiên đăng nhập, hồ sơ người, Bạn bè, Cộng đồng. Không còn «10:07 28-09» hay «28/9/2026».

### Sổ hành trình (UI-106, UI-157, UI-160, UI-161, UI-162) · BUG_FIX + UX_IMPROVEMENT (Go + màn)
- Thẻ huy hiệu giữ cột chữ ≥120dp. Ở 320dp không còn «Mở hà / ng» (UI-106).
- Chọn nhánh, nhận kết hay trưng huy hiệu bị từ chối thì câu nằm ngay dưới thứ vừa chạm (UI-157). Trước đây câu nằm ở cuối
  sổ, cách 850px.
- Chạm huy hiệu thứ tư khi đã trưng ba: không gửi PATCH, nói lý do ngay tại chỗ (UI-161).
- «MỚI MỞ» nhớ theo tài khoản (UI-160):
  - Go: migration thứ hai của `achievementv1` thêm `achievement_earned.seen_at`. Huy hiệu đạt quá 48 giờ trước lúc
    migrate được coi là đã thấy.
  - `GET /me/achievement-routes` trả thêm `seen` cho từng huy hiệu, chỉ trong sổ của chính chủ.
  - Route mới `POST /me/achievement-seen`, chỉ Go: manifest, evidence `docs/migration/routes/me/POST-me-achievement-seen.md`.
  - Màn trình bày huy hiệu mới nhất chưa thấy, rồi đánh dấu. Máy mới không chúc mừng lại một huy hiệu cũ.
  - Người khác xem huy hiệu được trưng thì không thấy dấu này.
  - Test PostgreSQL: `TestSeenBadgesFollowTheAccountInPostgres`.
- Hàng menu ở Cá nhân mang tên màn nó mở: «Hành trình · Sổ huy hiệu và các ngã rẽ của bạn», không còn «Thành tích · Cấp và
  huy hiệu…» (UI-162). Flow Maestro 32 chạm theo dòng phụ, dòng duy nhất trên tab.

### Hồ sơ kể chuyện và trang viết (UI-100, UI-153, UI-155, UI-156, UI-158, UI-159) · BUG_FIX + UX_IMPROVEMENT (Go + màn)
- Kệ «Những ngày muốn giữ» trống của chính chủ là một `EmptyState` (UI-153):
  - cảnh «chưa có kỷ niệm» và đúng một hành động;
  - có kèo đã qua: «Mở «tên kèo»», tới màn kèo, nơi có «Giữ lại cuộc đi»;
  - chưa có: «Xem kèo của nhóm».
  - Hàm thuần `diary/trang-dau.ts`.
- Tường cá nhân (UI-155):
  - Lượt long-poll hay lúc app quay lại ghép trang đầu mới vào những gì đang hiện (`lamMoiDauTuong`), không thay cả danh
    sách.
  - Các trang đã mở bằng «Xem những trang trước» và con trỏ của chúng còn nguyên.
  - Bài đã xoá trong cửa sổ trang đầu thì rời đi.
  - Làm mới lặng không huỷ trang đang tải thêm.
- Bài Cộng đồng có ảnh lên tường cá nhân kèm ảnh (UI-156, Go):
  - `community.WallImage` đọc ảnh của bản đang hiện, kể cả bản sửa.
  - `socialv2.wireWallPost` dùng ảnh đó.
  - Test ranh giới PostgreSQL: `TestPostgresCommunityPhotoReachesTheWall`.
- Bài dựng từ sổ có tên trang là ngày đọc được (UI-154 ở phía Cộng đồng, Go):
  - `community/diary.go` dùng `diary.TenTrang`;
  - trang cuối đọc sổ đã lưu cũng đổi tên trang ISO thành ngày đọc được (`diary/ten-trang.ts`).
- Trang viết, `BaiChiTietScreen`:
  - Bài «Chỉ mình tôi» mở bởi người khác là một câu với cảnh «chưa đọc được», không có hai khối lỗi, không «Thử lại» vô
    ích (UI-100).
  - Xoá bình luận của mình hỏi lại ngay tại hàng (UI-158).
  - Câu xác nhận đăng lại và lỗi của bài nằm ngay dưới các hành động của bài (UI-159).
  - Theo cột đọc trên tablet.

### Sửa sau các lượt đo của B9 (cùng batch)
- Viewer ảnh trên web (UI-094): ba vòng đo lộ ba lỗi.
  - `touchAction: "none"` của bản đầu làm khung lật trang đứng yên.
  - Trình duyệt khớp trang lúc nhấc tay rồi để đà cuộn trôi tiếp một trang (0 → 390 → 780px, «1 / 5» → «3 / 5»); không
    `scroll-snap-stop` nào với tới lớp bọc react-native-web đặt quanh mỗi trang.
  - Nay trên web, JS lật trang: trang theo ngón tay, nhấc tay thì đúng một trang (kéo quá 48px hoặc hất nhanh), phím ← →
    lật trang, con lăn và trackpad cuộn tự do rồi khớp về ảnh gần nhất. Native giữ cách lật của FlatList.
  - Đo trên bản cuối: một cú vuốt 0 → 390px, «2 / 5»; chạm đúp `scale(2)`, lần hai `scale(1)`; chụm `scale(1.78)`, trang ×1.
- Nháp story mất khi Back trình duyệt (UI-097): lần render đầu khi phiên chưa đọc xong dùng khoá nháp «-», rồi ghi biểu mẫu
  rỗng dưới khoá thật khi phiên tới, và nháp rỗng là nháp bị xoá. Phần soạn story nay chỉ dựng khi phiên đã đọc, mỗi người
  một lần (`key`). Bốn đường (khoảnh khắc, story × Back, «Quay lại») còn nháp.

## Hai quyết định nghiệp vụ, chủ sản phẩm chốt 04/10 (UI-131, UI-149)

Chủ sản phẩm chọn theo khuyến nghị, với yêu cầu «tính đường xa cho production, giải quyết triệt để và nhất quán mọi
xung đột». Hai ADR thật thay cho hai đề xuất.

### UI-131 · ADR-0053 «Tờ giấy» là sổ của mọi cặp · BUG_FIX (luật)
- Gỡ mâu thuẫn giữa ADR-0027 §4 / ADR-0038 §2.1 (tờ tạm trước khi lập sổ) và ADR-0046 §8.4 (lớp cặp đôi trong chat).
  Sổ hai người có hai loại «Hội bạn» và «Một đôi»; tờ giấy thuộc về sổ, còn lối tắt trong khay chat là của cặp đôi.
- Máy chủ không đổi: phác tờ cho cặp bạn là hành vi có chủ đích. Mọi tính năng riêng của cặp đôi (gu cho Nếp,
  «Người lo», `chia_gu`) vẫn đòi `CanBatDoi`.
- Ghim cả hai phía:
  - ca oracle mới Go so với Python «an active friends' notebook, «Một đôi» not on» (thành công ở cả hai máy chủ;
    canary đổi kỳ vọng sang `409` đỏ đúng ca);
  - phía app, test sẵn có ghim công cụ «Tờ giấy» trong khay chỉ cho cặp đôi.
- Phần app (lối «Rủ … tới đây» không phác tờ bỏ quán) đã ở B8.

### UI-149 · ADR-0054 mỗi khoản chi thuộc nhiều nhất một kèo · BUG_FIX (tiền: Go + Python oracle + migration + app)
- Lỗi: «đã chia» của kèo tính theo ngày. Hai kèo trùng ngày cùng tính một khoản, nên hero quyết toán ghi 27.411.356đ
  cho một sổ 13.705.678đ. Parity vẫn xanh vì hai bên cộng trùng giống nhau.
- Lược đồ (Alembic `d5e1a7c3b902`):
  - `expenses.outing_id`, khoá ghép `(outing_id, context_id) → outings(id, context_id)`;
  - `uq_outings_id_context_id`, index `ix_expenses_outing_id`;
  - không `ON DELETE`: kèo đang giữ tiền thì không xoá được.
- Một luật quy thuộc cho cả dữ liệu cũ (backfill) và mới:
  1. kèo bill được ghi từ;
  2. không có thì kèo duy nhất phủ ngày Việt Nam;
  3. hai kèo trở lên hoặc không kèo nào thì không thuộc kèo nào.
  Quy thuộc đặt một lần; đổi kèo là `409 expense_outing_mismatch`, kèo ngoài nhóm là `422 outing_not_in_context`.
- Đọc: Go `GroupRecap` và Python `group_recap` nối `expenses.outing_id = outings.id`, nên recap, album, budget, hồ sơ
  gu, gợi ý đều đúng theo. Kỷ niệm vẫn theo ngày. Phương án C (hero không vượt sổ) thành bất biến có test:
  Σ «đã chia» các kèo ≤ tổng sổ.
- Ngoại lệ Python có tên trong ADR: sửa sai tiền là blocker loại 2. Python sửa cùng commit với Go.
- Contract IR: chỉ thêm định nghĩa trường `outing_id` của `ExpenseInput` (15 dòng). IR trên `main` đã lệch sẵn với mã
  Python (thứ tự route và dependency sau ADR-0052); bản đó không trộn vào đây, ghi ở phần việc còn mở.
- App:
  - chia bill mở từ màn kèo (`/smart-split/{kèo}/review`) gửi `outing_id`; đường dẫn đã mang id kèo nhưng màn bỏ
    qua nó;
  - bản nháp bill tách theo nhóm và kèo, nên bill bắt đầu ở kèo này không lẫn sang kèo khác;
  - câu dưới hero của chuyến đang đi nói đúng luật mới.
- Bằng chứng:
  - Go, PostgreSQL thật: 1193 ca, gồm oracle tiền với 7 ca mới và oracle recap trên fixture có quy thuộc.
  - Python, PostgreSQL thật: 702 ca, cộng 9 ca mới trong `test_expense_belongs_to_one_trip_postgres.py`.
  - Đột biến: 2 Python và 2 Go, mỗi cái đỏ ở đúng ca dự đoán.
  - Parity dev, hai stack dựng từ cây: recap 48 bước EQUAL ×2; expenses, crossreplay, concurrency, albums,
    preference-profile, budget: 11 kịch bản EQUAL, 323 bước.
  - Cổng python-touch: các route Go phục vụ đọc `group_recap` (budget, albums, preference-profile) có mục ADR-0054 trong
    tài liệu bằng chứng của chúng.

## B11 · Rà nhất quán xuyên feature, critique cuối, Android · DESIGN_SYSTEM_IMPROVEMENT + BUG_FIX + UX_IMPROVEMENT

### Một ngữ pháp cho việc phá huỷ và cho nút tắt
- Nút phá huỷ mang tông `warn` ở mọi nơi (16 nút): «Xoá vĩnh viễn» tài khoản, «Rời nhóm», «Chặn», «Xoá tin», «Xóa cuốn
  sổ» (trước là nút cam đặc), «Bỏ tờ hẹn», «Bỏ bản nháp»… Test quét toàn cây `nut-pha-huy-tong-warn.test.mjs`: nút phá huỷ
  mới thiếu `warn` thì đỏ, và phép quét tự kiểm đã quét đủ nhiều nút.
- Hỏi tại hàng là một primitive, `ui/HoiTaiHang.tsx`: câu, hành động màu `warn`, «Thôi»; focus tới câu hỏi khi nó hiện
  (web: «Thôi», lựa chọn không làm mất gì; native: đọc câu). Dùng ở Thành viên, Cuộc trò chuyện, trang viết, bình luận
  Cộng đồng, «Điều mình muốn giữ». «Xóa ghi chép» trước xoá ngay sau một chạm.
- Nút tắt lâu dài luôn nói vì sao (`lyDo`): Sổ hành trình ×3, OTP (đếm ngược vào `lyDo`), tạo kèo, kèo, tường nhóm, thả
  khoảnh khắc, «Giữ một điều», đề nghị sửa, thư bỏ giấy, Nếp. Một nút tắt chết ở trang cuối bị gỡ. Test
  `nut-tat-co-ly-do.test.mjs` (canary đỏ đúng chỗ đã thử).

### Kích thước, cột, lưới
- Đích bấm còn 44dp nâng lên 48dp: nút «Khớp» của bản đồ hành trình, hàng người ở Bạn bè, hành động của bình luận ở trang
  viết, dòng kèo gọn ở chat, mũi tên chương của sổ hành trình.
- Lịch chọn ngày `ChonNgayLich`: bảy cột chia bề ngang thật, tối đa 48dp, hàng cao 48, hai nút tháng 48×48. Trước là bảy
  ô 44 cố định (≈ 326dp), tràn cột 288 của màn 320.
- Một số cho cột đọc: `adaptive.ts` `COT_DOC = 640`, dùng cho cột của `RudiScreen` và bề rộng tối đa của `Sheet`.
- Hành trình (`/achievements`) theo cột đọc ở cả ba trạng thái: trên tablet thẻ ngã rẽ và hàng huy hiệu từng trải
  720/912px, nay cùng cột với bản đồ.

### Trạng thái rỗng có cảnh, câu chữ đúng chỗ
- Cảnh cho hai ô rỗng còn trơn: chọn người (`chua-co-ban`) và chọn kèo (`tim-khong-ra`); `CHUA_VE` trống.
- Hồ sơ của chính mình: trang viết rỗng là tiêu đề ngắn cộng thân (trước: cả câu dựng thành tiêu đề h2), cảnh
  `chua-co-bai` thay cho cảnh trùng với kệ ngay trên, chính chủ có «Viết bài đầu tiên».
- Hồ sơ người bị từ chối cố định (403/404, `laTuChoiVinhVien`): không còn «Thử lại» gọi lại 403; 403 dẫn «Mở Bạn bè» (câu
  báo khuyên gửi lời mời kết bạn), 404 «Quay lại». Cùng luật UI-100 của trang bài.
- «Đã lưu»: tên chỗ đọc hỏng thì hàng ghi «Một chỗ đã lưu», không «Đang đọc tên chỗ…» mãi (nhãn đó còn lọt vào sổ tay Nếp).
- Sổ đôi chưa lập: câu không gán sẵn loại («Một cuốn sổ chỉ hai bạn đọc… là «Hội bạn» hay «Một đôi», hai bạn chọn sau»);
  quán mang tới chỉ có một lối «Rủ … tới đây», mang theo quán.
- Chuỗi máy «lượt dựng MP4» → «lượt dựng phim» (thẻ Hành trình ở Cá nhân, sổ huy hiệu, Nếp Phim). MediaPicker giữ «tệp
  MP4» vì đó là định dạng tệp người dùng chọn.

### Trợ năng
- `Chip vaiRadio`: «Ai được bình luận tường tôi» (Cài đặt, hồ sơ của mình) là `radio` có `aria-checked` trong
  `radiogroup`, không còn nút `aria-pressed`.
- `BadgeArt`: ảnh huy hiệu là nét vẽ của khung `role="img"` đã mang tên; `accessibilityLabel=""` cho expo-image web ra
  `alt=""` (axe image-alt critical ×5 trên Hành trình).

### Critique cuối (Flow D) và đợt sửa theo nó
Hai subagent cô lập đọc ảnh web `b9-sau`, `b8-sau3` và ảnh Android: **27/40 và 26/40** (pilot B3: 24 → 26).
- Công tắc: helper `ui/cong-tac.ts`. Tắt là vạch `lineStrong` (vạch `line` gần như mất trên nền kem); núm giấy trên cả web
  (`activeThumbColor`; react-native-web tự tô núm teal, màu của tiền). Cài đặt, Cài đặt nhóm, Sổ hành trình dùng chung.
- Phiên đăng nhập: hàng của phiên khác ghi «Đăng xuất phiên đó» (trước «phiên này» trên một phiên không phải phiên đang
  cầm), tên trợ năng nói phiên nào; phiên hiện tại luôn đứng đầu. Máy chủ không lưu tên thiết bị nên màn không đoán.
- Hộ chiếu Cá nhân: «6 khoảnh khắc» (trước «6 kỷ niệm» nằm ngay trên «Chưa giữ ngày nào»).
- Một chữ cho mỗi thứ: khoảnh khắc trên tường nhóm «Thả tim / Đã thả tim · N tim» (trước nút «Thích» cạnh «0 tim», bấm
  xong «Đã tim», tên trợ năng «Thả tim»); bài viết «Thích · N thích», cả ở thẻ trên tường cá nhân.
- «Thêm vào buổi đã hẹn» không còn bị cắt tên quán (tên quán vào tên trợ năng); «diary công khai» → «sổ chuyến đi của
  mình»; ngân sách kèo «dự kiến một người / dự kiến cả kèo»; quyết toán bỏ chữ «nghĩa vụ».
- DESIGN.md: «Công tắc (Switch)», «Chip vaiRadio», «Luật Một Thứ Một Chữ»; dòng `ChonNgayLich` ở Inputs sửa cho khớp lưới
  mới. Documenter (Impeccable) cập nhật DESIGN.md và `.impeccable/design.json` trước đó trong batch.
- Không tự đổi, ghi ở việc còn mở: thanh tab và con dấu «Tạo» (chủ sản phẩm giữ); tông `warn` gần `accent` (đổi token là
  quyết định hệ màu); hàng «Đặt làm quản trị» lặp mỗi thành viên.

### Native: e2e Android
Ba lượt `scripts/mobile_native.sh --otp` trên AVD chỉ-đọc, stack riêng thứ hai (API 25299). Lỗi lộ ra, đã sửa:
- Flow theo kịp app: 29, 33, 41, 42, 45 cuộn tới hàng menu Cá nhân (kệ rỗng UI-153 đẩy menu xuống dưới mép), nhắm dòng phụ
  duy nhất của hàng «Bạn bè» (hai dạng, có/không lời mời chờ), chờ đầu màn «Không gian của riêng bạn» trước khi cuộn.
- Flow 30: thẻ bình chọn sau B6 không còn in «· của bạn»; chờ radio đã chọn (trên Android tên mang giá trị: «Bỏ phiếu Bun
  bo, 1 phiếu»). Fixture chuỗi Maestro co lại một dòng.
- Flow 42: ô bình luận của trang viết tên «Viết bình luận» từ trước đợt này, flow vẫn tìm «Ô viết bình luận» (lệch có sẵn
  trên `main` 2b6c9360, lộ ra khi chuỗi đỏ dây chuyền hết); sau «Thích» flow chờ chip «❤️ 1» của trang bài cũ, nay chờ
  dòng đếm «1 thích · 0 bình luận», và thẻ trên tường cá nhân «1 thích · 1 bình luận».
- Flow 45: sau khi chặn, flow chờ «Kết bạn để nhắn riêng.»; màn không mời kết bạn lại người vừa bị chặn (có trên `main`).
  Flow kiểm cửa nhắn riêng đóng.
- Script chạy e2e của người sửa (ngoài repo) truyền DSN của stack riêng (`MOBILE_DATABASE_URL`, venv riêng có SQLAlchemy và
  psycopg) cho `scripts/mobile_native.sh`, để phép kiểm máy
  chủ sau flow 43 (story hết hạn) và 45 (hàng «reports») chạy được; trước đó chúng «không đo được», tức đỏ.
- Harness: sau một lần trả phiên về màn chào, đợi 66s trước flow kế (máy chủ chặn xin mã lại trong 60s, nên một flow đỏ kéo
  đỏ sáu flow sau); `lai_la_c` cho tám bước chuẩn bị và kiểm biết ai đang cầm máy sau khi trả phiên.
- `tests/test_maestro_flows_are_all_reachable.py`: hồi quy từ tích hợp PR #664 (c8328b0e thêm `00-*)` cho flow smoke).
  Test so với mọi flow có số, không chỉ dải sống; đột biến `77-*)` đỏ.
