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
