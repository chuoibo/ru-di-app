# Handoff session chuyển mình UI Rủ Đi

Tài liệu này bàn giao trạng thái thực tế của session Codex ngày 06/09/2026 cho
agent tiếp theo. Đọc cùng báo cáo gốc
[`bao-cao-chuyen-minh-ui-rudi.md`](bao-cao-chuyen-minh-ui-rudi.md), nhật ký
[`trien-khai-chuyen-minh-ui.md`](trien-khai-chuyen-minh-ui.md) và bằng chứng
[`wave1-native-evidence/README.md`](wave1-native-evidence/README.md).

## 1. Trạng thái Git và ranh giới

- Repo: `/home/lakiet/mobile`.
- Branch: `codex/ui-chuyen-minh-20260906`.
- Baseline trước session: `931d9c3`.
- HEAD khi viết handoff: `2490e6772e24d01d3ad2958186208360f471d621`.
- Chưa push, chưa merge, chưa có verdict độc lập `APPROVE` cho toàn app.
- Không sửa ba luật tiền, không thêm payment rail/VietQR/tài khoản ngân hàng,
  không sửa `phase0/` hoặc `docs/protocol/v1/`.
- Các báo cáo/audit untracked có sẵn thuộc người dùng và phải giữ nguyên; không
  stage cả thư mục `docs/archive/codex/2026-09-06/` một cách mù quáng.

Hướng thiết kế đã chốt là **“Nhật ký chuyến đi sau giờ làm”**: giấy ấm, bìa
indigo, mực rõ; coral cho thương hiệu/hành động, tím cho nội dung AI còn sửa
được, teal cho tiền. Body 17/24, caption 13/18, control tối thiểu 48dp, grid đo
chiều rộng content còn lại sau navigation rail và tối đa ba cột.

## 2. Các checkpoint đã tạo

### `c29c54f` — timeline revision nguyên tử

- Thêm `timeline_revision` vào outing và migration
  `c9d0e1f2a3b4_them_phien_ban_lich_trinh.py`.
- `PUT /outings/{id}/timeline` nhận `expected_revision`; khóa row, so sánh và
  tăng revision trong cùng transaction.
- Bản nháp stale nhận HTTP 409 `timeline_conflict`; client cũ không gửi revision
  vẫn tăng revision.
- Giữ identity/order stop và check-in hiện có; không tự sort lại theo giờ.
- Test PostgreSQL có race hai session và migration upgrade → downgrade → upgrade.

### `2fb215f` — foundation ảnh địa điểm công khai

- Thêm `PlacePhoto`, repository, migration
  `d0e1f2a3b4c5_them_anh_dia_diem_co_giay_phep.py` và hai route đọc:
  `GET /places/{place_id}/photos` và
  `GET /places/{place_id}/photos/{photo_id}`.
- Chỉ nhận ảnh Wikimedia Commons có license trong allowlist: CC0-1.0,
  CC-BY-3.0/4.0, CC-BY-SA-3.0/4.0.
- Metadata provenance bắt buộc; storage key opaque; unique theo
  `(place_id, source_url)`; re-encode pixel để bỏ EXIF.
- Namespace `licensed-places` tách khỏi ảnh riêng tư của nhóm; route công khai
  không thể đọc private bytes dù trùng storage key.
- Hai lỗi được bắt và sửa trước commit: class `PlacePhoto` từng chèn giữa
  `GuestLink`, và method `list` từng che builtin trong type annotation.

### `5d853f4` — importer Commons có kiểm soát

- CLI `python3 -m app.places.import_photos` mặc định dry-run; chỉ `--apply` mới
  ghi database/file.
- Chỉ đọc mapping ngoài mọi Git worktree có `human_reviewed: true`, tối đa 50
  entry/256 KiB. Không tự tìm ảnh, không đoán theo tên/toạ độ, không nhận URL
  tuỳ ý.
- Giới hạn HTTPS host/path/redirect, timeout, 10 MiB, 50 triệu pixel; kiểm
  license name và URL cụ thể, chuyển author HTML sang text an toàn.
- Output chỉ là aggregate counters; không log địa điểm, source record, path hay
  traceback có thể chứa dữ liệu nhạy cảm.
- Runbook vận hành: [`nhap-anh-dia-diem.md`](nhap-anh-dia-diem.md).
- Chưa chạy import Commons thật và chưa tạo mapping thật.

### `2490e67` — checkpoint nền UI/native

- Đồng bộ `packages/shared/tokens.json`, `DESIGN.md`, sidecar Impeccable và CSS
  guest theo palette semantic sáng/tối. `tokens.json` vẫn là nguồn sự thật.
- Thêm `expo-system-ui ~57.0.3` để Android phản hồi appearance tự động; pin
  `react-native-gesture-handler ~2.32.0` đúng Expo SDK 57.
- Body thành 17/24, caption 13/18; cập nhật contrast docs/tests. Caption tổng
  bill dùng teal `split` trên `splitSoft`, đo 4,87:1 thay cho 4,47:1 trước fix.
- `ResponsiveRow` đo container bằng `onLayout`, xử lý width bất thường và tối
  đa ba cột; không dựa vào tên thiết bị/window toàn màn.
- `RosterPicker` dùng checkbox rõ chọn/chưa chọn; bill fixture và live chỉ mở
  một món để chỉnh, các món còn lại là summary.
- Create sheet dùng context nhóm hiện hành, danh sách hành động phẳng, scroll,
  max width 560dp và dark mode riêng.
- `ReorderList` dùng long-press drag trên handle, row cao biến thiên, action
  accessibility tăng/giảm vị trí; `OutingLive` giữ draft revision, báo conflict,
  reload/reconcile thứ tự theo ID và retry idempotent.
- `PhotoViewer` có pager, double tap, pinch/pan, private image cache policy
  `none`. Fix sau review: giữ đúng current page khi đổi width và clamp pan
  offset theo scale mới khi pinch thu nhỏ.
- `AlbumLive` mở toàn bộ ảnh và viewer; không còn nút giả video.
- Login chỉ hiện Google trên native khi client IDs hợp lệ; dynamic import SDK,
  hủy chọn không gọi API, chỉ đổi ID token lấy session Rủ Đi. Không hiển thị
  Apple giả. Hướng dẫn: [`cau-hinh-google-native.md`](cau-hinh-google-native.md).
- `useMotion` theo dõi Reduce Motion runtime; route line dùng SVG mask để nét
  đường không xuyên qua node; MediaSlot bỏ fade khi giảm chuyển động.
- Có probe fixture-only `app/dev/ui-lab.tsx` và helper chụp có kiểm text trước
  khi ghi `apps/mobile/tools/native-wave-proof.mjs`.

## 3. Review Impeccable

Review độc lập đầu tiên trả `fix` với bốn finding:

1. Viewer resize có thể quay ảnh về trang đầu nhưng header/caption ở trang hai.
2. Pinch từ 4× xuống 1,2× có thể giữ offset vượt giới hạn mới.
3. Caption summary bill 13sp chỉ đạt 4,47:1.
4. `DESIGN.md`/sidecar còn palette và body 16/23, caption 12/16 cũ.

Cả bốn đã được sửa trong một batch. Reviewer ban đầu bị treo ở verdict pass;
reviewer fallback mới, không kế thừa context, chấm:

- resize viewer: **Resolved**;
- pinch clamp: **Resolved ở mức code/hàm thuần**, chưa có gesture nhiều ngón thật;
- contrast caption bill: **Resolved**;
- DESIGN/sidecar: **Resolved**.

Disposition là **SHIP chỉ trong phạm vi bốn finding/checkpoint**, không phải
`APPROVE` toàn app hay production. Detector HTML/CSS không chạy cho React
Native theo hướng dẫn Impeccable.

## 4. Bằng chứng đã chạy

- Mobile cuối: **669 passed**, typecheck đạt.
- Viewer helper: **2 passed**; phủ current index và offset 4× → 1,2× → 1.
- Token/contrast: **13 passed, 165 subtests passed**.
- Importer Commons mock HTTP/repository: **56 passed**.
- PostgreSQL foundation/timeline chạy lại bởi main:
  **36 passed, 72 subtests passed**.
- Reviewer foundation chạy PostgreSQL riêng:
  **129 passed, 88 subtests**, rồi **63 regression passed**.
- API + domain regression sau checkpoint:
  **1.928 passed, 4.657 subtests passed**.
- Ruff targeted đạt; `git diff --check` đạt.
- Repo guard staged checkpoint UI: 50 file đạt; toàn tracked tree:
  **1.550 file đạt**.

Không dùng các con số trên để tuyên bố production E2E. Importer unit dùng mock;
PostgreSQL dùng dữ liệu tổng hợp; Android không chứng minh iOS.

## 5. Bằng chứng native bền vững

Sáu PNG và mô tả cấu hình nằm tại
[`wave1-native-evidence/`](wave1-native-evidence/README.md). Tất cả chụp từ APK
debug trên Android 15/API 35, AVD riêng `rudi-qa3`, serial `emulator-5560`, bằng
fixture/stock công khai; từng file được main mở xem và ghim SHA-256 trong repo
guard.

Đã chứng minh trong phạm vi ảnh/tương tác đã đi qua:

- phone light và phone dark phản hồi appearance;
- roster 2 cột phone, 3 cột tablet;
- Create max 560dp ở tablet/font 1,3×;
- Explore 2 cột trong content sau rail;
- drag probe thực sự đổi thứ tự;
- viewer vuốt sang trang hai, double tap zoom, hardware back;
- sau resize 1600 → 1800 px, ảnh 2, `2 / 2` và caption vẫn khớp; vuốt về ảnh 1
  được;
- composer chat đứng trên IME trong fixture đã kiểm trước checkpoint.

Chưa chứng minh pinch đa điểm thật, FPS/haptic phần cứng, TalkBack/VoiceOver,
iOS/iPad, OAuth thật, live API timeline conflict trên hai client hoặc private
album live end-to-end.

## 6. Working tree lúc bàn giao

Khi viết file này, HEAD là `2490e67`; có một cập nhật mới của nhật ký
`trien-khai-chuyen-minh-ui.md` đang staged nhưng chưa commit. Agent kế tiếp nên
kiểm lại `git status --short`, stage riêng file handoff này và commit hai tài
liệu bằng một commit `docs:` nếu diff vẫn đúng.

Các path untracked có trước/ngoài checkpoint cần **giữ nguyên** gồm `.claude/`,
`.impeccable/critique/`, các báo cáo Claude/Codex và `ui-native-evidence/` cũ.
Đặc biệt không chạy `git add docs/archive/codex/2026-09-06` vì sẽ kéo báo cáo/ảnh audit
ngoài phạm vi vào commit.

## 7. Việc agent kế tiếp nên làm

Ưu tiên tiếp tục theo thứ tự này:

1. Commit tài liệu handoff + dòng nhật ký staged sau khi chạy repo guard.
2. Tích hợp `public_photo_snapshots()` vào contract catalogue `/places` và
   `photoUrl/photoCount` của mobile; hiện route ảnh đã có nhưng Explore live
   chưa dùng. Không gán stock fixture cho địa điểm thật.
3. Tạo/duyệt mapping Commons ở ngoài Git và chạy dry-run; chỉ nhập thật sau khi
   người vận hành xác nhận từng ảnh đúng địa điểm và license.
4. Thêm quan hệ riêng giữa ảnh nhóm và địa điểm; không dùng bảng public
   `place_photos` cho ảnh nhóm.
5. Hoàn thiện timeline: auto-scroll khi kéo danh sách dài, test native conflict
   hai client và refresh/reconcile.
6. Tiếp tục các wave còn lại: Explore/detail/outing composition, chat/social,
   vote/bill, memory/profile. Không gọi checkpoint nền này là toàn bộ kế hoạch.
7. Chạy iOS/iPad, TalkBack/VoiceOver, gesture pinch đa điểm và phần cứng thật.
   OAuth Google cần credentials/signature thật ngoài Git; nếu thiếu cấu hình,
   nút phải tiếp tục ẩn.
8. Mỗi lát phải có screenshot native hợp lệ, review Impeccable độc lập và
   checkpoint riêng; không tự merge, push hoặc tự cấp `APPROVE`.

## 8. Câu lệnh tái kiểm tra nhanh

```bash
cd /home/lakiet/mobile/apps/mobile
npm test
npm run typecheck

cd /home/lakiet/mobile
python3 -m pytest services/api/tests/web/test_contrast_floor.py services/api/tests/web/test_shared_tokens.py -q
python3 -m pytest services/api/tests/places/test_import_photos.py -q
python3 scripts/repo_guard.py tree HEAD
git status --short
```

Test persistence mới phải chạy theo
`docs/testing/postgres-repository.md` với PostgreSQL thật. Không thay bằng
SQLite hoặc fake repository rồi gọi đó là bằng chứng database.
