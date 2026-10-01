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
