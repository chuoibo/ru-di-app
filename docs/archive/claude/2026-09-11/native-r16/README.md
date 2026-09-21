# native-r16 — R5: nhịp tờ AI trong chat và bề mặt Cài đặt về hệ giấy–mực (tái audit 10/09)

Codex (R5, P3): tờ AI trong chat «dài và nặng làm gián đoạn nhịp hội thoại» → ưu tiên quyết định kế tiếp, mở thêm phần giải
thích; Cài đặt «nhiều thẻ trắng nổi, bo và bóng nhiều hơn hệ sổ/ledger bên cạnh… cảm giác trở lại một bộ UI dùng chung» →
giảm lớp thẻ/bóng, dùng nhóm hàng của hệ giấy–mực. Điều kiện: đặt cạnh chat/ledger/profile vẫn nhận ra cùng một sản phẩm
ở sáng/tối; không mất thông tin trạng thái hay giúp đỡ.

DESIGN.md đã nói «Hàng + kẻ tóc là container mặc định» và «Không màn nào trong đợt này còn gọi `Card`» — câu sau **sai**:
họ Cài đặt gọi `Card` 14 lần (8 ở `CaiDatScreen`, kể cả `Card` bọc `Segmented` = thẻ lồng thẻ).

## Sửa

- **Kit `NhomHang`** (`ui.tsx`): hàng trên giấy, mỗi con một kẻ tóc `line`, không thẻ/bóng/bo — cùng hình Profile,
  DiemDenScreen, HangDiaDiem đã chép tay ≥ 3 lần (đủ điều kiện extract).
- **Họ Cài đặt** (5 màn): bỏ **14** `Card`. `CaiDatScreen`: mỗi mục là một `NhomHang`; `Segmented` nằm thẳng trên giấy;
  lỗi là chữ `warn` trần; chân trang `Heading title=""` thành caption. `PhienScreen`/`DaChanScreen`: hàng phiên/người là
  con của `NhomHang`; skeleton không thẻ. `VeRuDiScreen`/`XoaTaiKhoanScreen`: khối chữ là `View` thường. Mọi nhãn flow 44
  giữ nguyên văn («Phiên đăng nhập», «Cho tìm theo số điện thoại», «Chỉ bạn bè», «Người đã chặn», «Giao diện», «Điều khoản và
  dữ liệu», «Xoá tài khoản»).
- **Tờ AI fixture** (`Group.tsx`): tiêu đề `title` (không `h2` trong luồng chat) → một dòng cốt «3 ngày 2 đêm · đồ ăn local ·
  săn mây» → nút «Xem lịch trình» + «Mở bình chọn» (nhãn pinned giữ) → **một cửa mở** «Vì sao phác vậy» (mẫu «Cách tính»:
  `Pressable` 48, `label inkSoft` + chevron, `accessibilityState.expanded`) → footer «Rủ Đi AI» + badge «AI nháp». Bỏ câu
  «Nhóm sửa được trước khi chốt.» (badge đã nói một lần — Luật Nói Một Lần). `TheAi.tsx` thẻ lịch trình live: `h2` → `title`.

## Cổng

- `tests/rudi-khong-card-trong-cai-dat.test.mjs`: quét nguồn — không file nào trong `screens/cai-dat/` import `Card`; `NhomHang`
  có trong kit và `CaiDatScreen` dùng ≥ 5 lần; **in tên** hai màn ngoài phạm vi còn dùng `Card` (`story/DangStoryScreen`,
  `tuong/BaiChiTietScreen`) — nợ có tên, không đỏ.
- Maestro `.maestro-bs-r16/`: `90-cai-dat-giay-muc` (nửa trên + nửa dưới), `91-chat-ai-gon` (assert «Rủ Đi đã phác một plan»,
  «AI nháp», **không** «Nhóm sửa được trước khi chốt», rồi mở «Vì sao phác vậy»). 1.0/2.0 × sáng/tối: 8/8 xanh.

## Ảnh (`anh/`)

`r16-90-cai-dat-tren-*` / `r16-90-cai-dat-duoi-*` (4 cấu hình) — đặt cạnh `docs/archive/codex/2026-09-10/reaudit-evidence/19-settings.png`
(trước) và `09-profile.png` (bề mặt đích). `r16-91-chat-ai-*` (đóng) / `r16-91-chat-ai-mo-*` (mở cửa «Vì sao») — đặt cạnh
`08-messages.png` (trước).

## Bốn màn con (`92-cai-dat-man-con`, deep link ấm)

`anh/r16-92-ve-rudi-*`, `r16-92-xoa-tai-khoan-{1,2}-*` (khối chữ không thẻ, hai bước), `r16-92-phien-*`, `r16-92-da-chan-*`.
Fixture không có máy chủ và cổng trống trên WSL2 nuốt SYN nên fetch **treo**: Phiên/Đã chặn dừng ở **skeleton** (hai hàng
xám trên giấy, không thẻ — reviewer bắt tôi viết nhầm «ErrorState»; ảnh cho thấy skeleton) — **hàng dữ liệu live của hai màn
này không chụp được ở đây**; hình hàng đọc từ nguồn (`NhomHang` + `hang` minHeight 56, không đệm đôi —
reviewer bắt lỗi 6+6 dp, đã bỏ). `Segmented` ở 2.0 tối: nhãn «Theo hệ thống» xuống hai dòng và chạm mép ô (nợ có sẵn, giờ
lộ) → thêm `paddingVertical 8` + căn giữa cho nhãn ô; nhãn giữ nguyên văn vì flow 44 `tapOn` nó.

## Chưa chứng minh

Phép «đặt cạnh nhận ra cùng sản phẩm» bằng người thật; thẻ AI **live** ở 2.0 và **hàng phiên/người đã chặn live** (không có
stack live); **đường lỗi lưu Cài đặt** (`loi` là dòng `warn` dưới «Xoá tài khoản», chưa ép lỗi để render — vị trí có sẵn
từ trước); hai màn ngoài phạm vi còn `Card` (story, bài chi tiết tường) — ghi nợ; TalkBack đọc cửa mở
(`accessibilityState.expanded` có, chưa nghe).
