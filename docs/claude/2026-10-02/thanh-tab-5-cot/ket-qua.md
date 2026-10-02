# Thanh tab 5 cột — kết quả

- Nhánh `claude/thanh-tab-5-cot`, base `bc8dbdfb`. Spec: `docs/claude/2026-10-01/thanh-tab-5-cot/spec.md` (mục «Bổ sung
  khi lập kế hoạch (02/10)» thắng phần trên). Kế hoạch: `ke-hoach.md` cạnh file này.
- Cách làm: native (executing-plans), frontend qua impeccable-pipeline (extension, code-led, comp = mockup ba bảng của chủ
  sản phẩm, giữ ngoài Git ở `~/.local/share/rudi-ux/thanh-tab-5/mockup/`).
- Verdict: finish reviewer Impeccable **ship** (8/8 việc đã sửa, hai lượt chấm lại). Review toàn nhánh (reviewer mới,
  opus): không Critical, 3 Important đã sửa ở `eb7b882e`, 9 Minor để lại (cuối file).

## Đã làm

| Phần | Commit |
|---|---|
| Logic thuần cột/ô con dấu/mục Khám phá sau cùng (`thanh-tab.ts`), `community` `href: null` | `7be94785` |
| Nếp: route không cột (`muc_trong_tab`), Go + JS + bộ rút | `0b40daec` |
| RudiTabBar năm ô, logo đầu rail | `5a9e777a` |
| `DauKhamPha` | `fd041138` |
| Địa điểm: hàng tiêu đề, «Chỗ hay ở …», Maestro 26/38 | `d39d750a` |
| Thẻ đầu theo mockup (`the-dan.ts`) | `982ac086` |
| Cộng đồng: chip-tab, `SearchField`, viết bài qua «Tạo», `_community.yaml` | `b368aa7d` |
| Thẻ bài: ảnh rộng hết cột, «n/N», nút Lưu (`album.ts`) | `07bc6fbb` |
| Nếp: sổ tay, đường ít bước không tên, ghim lại MRR | `309745dd` |
| Lỗi có sẵn: bảng tin trống trả 503 (`candidates.go`) | `4601085e` |
| Vòng kiểm hình 1 | `f54403ff` |
| Theo finish review (sân khấu, tim tròn, «bai-viet», «Tạo», cuộn tiêu đề…) | `3b9aaab7`, `2ffcfe4c` |
| DESIGN.md (documenter) | `0f8123e2` |
| Ảnh bằng chứng (4 JPEG, ghim allowlist) | `531ad335` |
| Comment ConDauTao | `d0a13b7b` |
| Sau review toàn nhánh: album giữ chỗ 4:3, flow chuyển động, ca Postgres bảng tin trống | `eb7b882e` |

## Số đo

- **Tâm con dấu − tâm màn = 0 dp** ở 390/320/430 trên `/explore`, `/community`, `/plan` (probe `kiem-ux/thanh-tab-5.mjs`,
  web export, 18/18 kiểm). Bốn cột 78/64/86 dp; đúng một cột sáng (Khám phá trên cả hai mục).
- Hàng tiêu đề ở 320 (cỡ chữ 1.0): Cộng đồng 272/288 dp, Địa điểm 216/288, demo 216/288. Ở 320 × 1.3: **chưa chụp native**
  (emulator chung thuộc phiên khác); proxy 246 dp (= 320 ÷ 1.3) cho thấy «Cộng đồng» trọn vẹn x 68–174, trước bản sửa
  `onLayout` nó nằm dưới nút cài đặt.
- Ảnh thẻ bài 358 dp = cột (390 − 2×16), 4:3, «1/2», nút Lưu đổi `aria-pressed` false → true.
- Tablet 1024: «Chỗ hay ở Đà Lạt» ở ~485 dp trên 768, hàng quán đầu trong màn đầu.

## Cổng (cây sạch tại `0f8123e2`)

npm ci · tsc --noEmit · npm test **1433/1433** (MOBILE_REQUIRE_WEB_A11Y=1) · expo export --platform all · gofmt sạch ·
go vet · go test ./... **166 ok / 0 FAIL** · check_screens_reachable 74/74 · eval_kich_ban (59 + 18 ca, hai lần trùng byte) ·
repo guard range + tree. Canary: `_layout.tsx` của `bc8dbdfb` → test thanh-tab đỏ ca 7; identity 7/7.

Đột biến (mỗi cái đỏ đúng chỗ dự đoán, đã hoàn nguyên): (a) bỏ lọc cột; (b) cột sáng theo route; (c) mặc định Cộng đồng;
(d) `tabState(!chon)`; (e) route không cột vẫn là cột (Go); (e2) bỏ cạnh từ route không cột; (f2) nối cả cột chủ;
(g) gương JS không loại route không cột; (h) giá lọt vào dòng phụ; (f) `floor` thay `round` ở album; (i) bỏ luật ít bước
không nhãn; (j) bản sao xếp hạng nil.

Sau `eb7b882e`: npm test **1436/1436**; tầng `go_postgres_tier.sh` **3321 ca PASS**, sentinel có mặt, không SKIP.

**Commit đỏ đã biết:** `7be94785` đứng một mình đỏ ở `go test ./internal/huongdan` (`+community`), vì `_layout.tsx`
ẩn `community` trước khi mô hình Nếp (`0b40daec`) học route không cột. Mọi commit từ `0b40daec` xanh.

## Critique Impeccable (dual-agent)

Nielsen **26/40 sau** (25/40 trước; H8 thẩm mỹ 2 → 3). P1 cả hai đều có sẵn từ trước: Cộng đồng xoá bảng tin mỗi lần
quay lại; nút ✨ ghi đè chữ và không gửi. P2: giá bị cắt ở hàng so sánh; chip-tab trông như bộ lọc; công tắc mục xa ngón
cái. Detector: mã nguồn 0; trang render 15/27 cảnh báo, gần hết là hiện vật react-native-web; axe chỉ `region`.
Snapshot: `.impeccable/critique/` (gitignore).

## Audit native (/20)

| Chiều | Điểm | Ghi chú |
|---|---|---|
| Trợ năng | 3 | tablist đúng vai ở thanh, đầu Khám phá, chip; 48 dp; cuộn thay vì cắt ở chữ to. Thiếu: chữ «Tạo» ngoài vùng chạm của con dấu; chế độ «Bài của tôi/đã lưu» không có tab chọn; không heading/landmark (có sẵn) |
| Hiệu năng | 3 | FlatList, PostCard `memo`; album `setState` chỉ đổi khi sang trang; Skia/SVG sân khấu |
| Giao diện & theme | 4 | chỉ token; chân trời tablet dùng `colors.ink`; tối kiểm bằng ảnh |
| Đúng nền tảng | 3 | thanh tab tự vẽ có từ trước; iOS `BlurView` chưa từng được chụp |
| Thích ứng | 3 | rail tablet, sân khấu theo cỡ lớp, tiêu đề cuộn; split-view/xoay chưa kiểm |
| **Tổng** | **16/20** | Tốt |

## Còn mở / không làm

- Android 320 × 1.3 native và iOS chưa chụp.
- Bảng tin «Dành cho bạn» trên stack QA chung vẫn lỗi tới khi stack chạy core có `4601085e`.
- Từ critique (cần chủ sản phẩm quyết): Cộng đồng giữ vị trí đọc khi quay lại; ✨; giá ở hàng so sánh; chip-tab khác
  dạng bộ lọc; chạm cột đang sáng cuộn lên đầu; chữ «Tạo» trong vùng chạm.
- Có sẵn, ngoài phạm vi: Back web sau khi quay lại một tab đã mở «đứng yên» một lần (`backBehavior="history"`);
  `POST /sessions/web/resume` treo khi chưa đăng nhập trên stack chung; mép kéo Nếp 10 px trên web.
- `.impeccable/design.json` lệch schema (CONTEXT_STALE `route`: đề nghị `document` tạo lại sidecar); `config.json` có khoá
  không ai đọc (`_`, `enabled`, `designSystem`, …).
- Badge Tin nhắn, «+»→«×», khay theo giai đoạn chuyến: ngoài phạm vi theo spec.

## Quyết định thay chủ sản phẩm (ghi đầy đủ ở sổ tiến độ)

- Surface brief ở `.impeccable/surfaces/apps-mobile-app-tabs-explore-tsx.md` (tên do công cụ chọn).
- Không chạy critique mốc trước khi sửa; Assessment A chấm cả bản trước lẫn bản sau trong một báo cáo.
- Mô hình Nếp làm trước RudiTabBar để các commit sau xanh; đầu dò thứ hai của `TestTabRutBangTabLayout` đổi sang `plan`.
- Luật hoà của `duongToi`: ít bước không nhãn hơn trên cả phần còn lại thắng, xét TRƯỚC luật «có nhãn» (thử luật «cột
  thanh tab thắng» rồi bỏ vì đổi `messages → outings/[id]`). `places/[id] → outings/new` đổi sang đường gọi tên được
  cả hai bước.
- Sổ tay Nếp không thêm mục mới (lần thử đầu kéo recall@5 0.9505 → 0.9396); MRR ghim lại đều tăng nhẹ.
- Chip-tab đang chọn giữ dấu ✓ (luật «chọn nói hai lần») dù mockup không vẽ.
- Sửa lỗi có sẵn ngoài kế hoạch: bảng tin trống 503 (`4601085e`); subflow chuyển động chờ tiêu đề Sở thích cũ.
- Không chụp Android 320 × 1.3 (emulator của phiên khác); đo bằng proxy 246 dp.
- Cửa «Dữ liệu demo» ở dòng vị trí thay vì ô phải của hàng tiêu đề (ở hàng tiêu đề nó đẩy chữ tràn ở 320).
- Giữ `assertNotVisible «Gần bạn, đúng gu»` ở flow 22 và đưa câu vào `daXoa` (reviewer đề nghị bỏ dòng đó).

## Minor để lại (review toàn nhánh)

1. Bước «Khám phá» của Nếp mở mục xem sau cùng, nên đường giả định Địa điểm có thể rơi vào Cộng đồng.
2. Đồ thị Nếp không có cạnh con dấu «Tạo mới» từ các route của thanh (có từ trước; giờ lệch với câu trong `cong-dong.md`).
3. Dòng phụ thẻ đầu mất icon sao («4,8 (120) · 1,2 km»).
4. PostDetail lề 20 quanh PostCard lề 16 (lệch 4 dp).
5. Bề rộng trang album làm tròn; native có thể lệch khi lật nhiều trang hoặc xoay máy.
6. Cộng đồng xoá bảng tin mỗi lần focus (có từ trước).
7. Nút Lưu không có trạng thái chờ (bấm nhanh gửi trùng, idempotent).
8. Worklet RudiTabBar lặp phép tính `oCuaCot` thay vì gọi hàm đã test.
9. Bản ghi: các quyết định trên đã được đính chính trong sổ tiến độ.

## Gộp vào main (02/10, tối)

main đi thêm 8 commit trong lúc nhánh làm (33daaa38..95755a71: B4–B7 của đợt nâng cấp UI/UX, AI chấm ảnh/video,
chặng go-media). Gộp một lần ở merge `8a244502`; cách giải từng xung đột ghi trong commit đó. Tóm tắt: bản sửa bảng
tin trống của B7 thay `4601085e`; thẻ đầu dùng `tachGia` của B5 (bỏ `the-dan.ts`); cách gập sân khấu của B5 chuyển
vào `SanThanhPho`; `CommunityScreen` lấy bản B7 làm gốc, trên tab ô phải của hàng đầu là chuông + cài đặt; dưới
360dp ô phải lên trên hàng chữ.

Cổng trên cây sạch đúng SHA `8a244502`:

- `gate.sh` guard · guard-range · screens · go-vet · go-test · eval-kich-ban · shared · mobile: **ĐẠT** (mobile: npm ci,
  tsc, npm test **1473/1473** với MOBILE_REQUIRE_WEB_A11Y=1, expo export --platform all).
- contract · client-routes · server-routes · cors · ownership · python-touch (`--strict`, trong container
  `rudi-ux-pytest-git:dev` vì máy không có fastapi): **ĐẠT 6/6**.
- pytest gốc trong container: 841 pass / 19 fail; main sạch 840 / 19 — **cùng 19 lỗi môi trường, 0 lỗi mới**.
- `go_postgres_tier.sh`: 3328 PASS, 0 SKIP, **1 FAIL có sẵn trên main**: `TestXoaTaiKhoanKhongConHangNaoCuaNguoi`
  (nepnho) — migration B7 `notification_source.sql` thêm `community_notifications.actor_id` mà danh sách cột cần dọn
  khi xoá tài khoản không có. Đỏ y hệt trên main `95755a71` sạch; đã báo phiên sở hữu module.
- Nếp: recall@5 không đổi ở mọi bộ; MRR ≥ main ở mọi bộ (ghi trong `truy_hoi_test.go`).

Hình (web export bản gộp, stack QA, cổng 8081 phiên kia nhường): probe `thanh-tab-5.mjs` **17/18** — tâm con dấu
lệch 0 dp ở 390/320/430; ở 320 đầu Cộng đồng: chuông + cài đặt lên trên, chữ cần 216/288 dp (× 1.3 ≈ 274/288);
ca còn lại («thẻ bài») không có dữ liệu: stack đã dựng lại, không còn bài có ảnh, và đăng bài mới sẽ gọi AI duyệt
(trả tiền) nên không làm; mã album không đổi qua gộp ngoài tên prop `khung` và `thanBiCat`. Probe `tt5-gap.mjs`:
gõ tìm thì sân khấu 163 → 0 dp, ô tìm ngay dưới dòng điểm đến (không đè), xoá chữ thì về đúng 274 dp, cả bản thật
lẫn demo. Ảnh đã mở xem: 390 sáng/tối, 320, 1024, demo, lúc đang tìm.
