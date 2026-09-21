# Phase 2 «Nếp truyền giấy» — lát 1 trên fixture: bằng chứng

Nhánh `claude/p0-w-hn-2-to-giay-fixture`. Kế hoạch: Phase 2 của
`~/.claude/plans/home-lakiet-mobile-docs-codex-2026-09-0-stateless-river.md`. Spec: §3, §5.2, §6, §7, §12, §15, §16, §20.2.
Nền: Phase 1 (#612) — `ToGiay`/`VetGap`, Nếp mảnh, `ban-tinh`.

## Cái gì được dựng

| Phần | File | Cổng |
|---|---|---|
| Mô hình tờ giấy phía client soi hợp đồng wire ADR-0027: 13 trạng thái, phiên bản bất biến, `daDongY` (gửi = đã ừ; tờ Nếp không ai ừ sẵn), `coTheChot` (cả hai cùng phiên bản), `coTheRut` (chưa xem, chưa phản hồi), `khacGi`, `cauTrangThai`, `nutChoTo`, `toUuTien` (MỘT tờ đang mở), `demHauQuaDongSo` | `src/rudi/to-giay/to-giay.ts` | `to-giay-luat` 10 ca |
| Chuyển trạng thái fixture thay máy chủ tới Phase 4 (đồng hồ là tham số; `chot` khoá một outing theo tờ; `dongSo` không làm sống lại gì) | `src/rudi/to-giay/so-fixture.ts` | `to-giay-fixture` 5 ca |
| Provider sổ hai người trong bộ nhớ, hành động «người kia» chỉ dưới `CUA_FIXTURE_DEV` | `src/rudi/to-giay/SoDoi.tsx`, gắn ở `app/_layout.tsx` | `tsc`, detector |
| Fixture cặp «Người ấy» (tên vai), hai tờ cũ (một đã giữ, một hết khung), ràng buộc mẫu, dữ liệu Nếp phác | `src/rudi/to-giay/fixtures-doi.ts` | không tên người thật |
| 13 bề mặt lát 1: `KhongGianGiay` (+ route `groups/[id]/to-giay`), `ToLoiRu`, `HangToGiay`, `DeNghiSua`, `GiuMotDieu`, `LapSo`/`BatMotDoi`, `RangBuoc`, `DongSo`, `LoaiSo`, `ChonNguoi` (+ route `hai-nguoi/chon-nguoi`), sheet Cài đặt sổ | `src/rudi/screens/hai-nguoi/` | detector `[]`, `check_screens_reachable` |
| Đụng hội bạn đúng ba chỗ, đều là thêm: `Group.tsx` rẽ header + thân theo `contextId === CAP_DEMO.id`; `Create.tsx` một mục; `CaiDatNhom.tsx` một `ListRow` cho pair | | `so-ban-tinh-mot-cho` hai chiều |
| Maestro `.maestro-bs-r18/100–106` trên nhãn đọc được | | exit 0 × 4 cấu hình (bảng dưới) |

## Điều đo được đã đổi hướng thiết kế trong lượt này

1. **Không gian giấy mở vào ký ức.** Với hai tờ cũ trong sổ, `toUuTien` trả tờ «đã giữ» tuần trước làm tờ trên bàn (spec §15.1 ưu tiên
   ba), nên trạng thái trống «Chưa có tờ nào tuần này» chỉ hiện khi sổ chưa từng có buổi. Flow neo lại; hàng ghim trong chat chỉ đọc
   `cauTrangThai` cho tờ **còn chơi**, ký ức thì đọc «một tuần yên».
2. **Chat cặp cần thân riêng.** Thân chat Team Đà Lạt dưới header «Người ấy» đọc lệch ngữ cảnh; `ThanCapDemo` hai người, tên vai.
3. **Con dấu «ĐÃ GIỮ» vẽ thành «ĐÃ».** Biến thể `ink` của `Stamp` trên emulator bỏ chữ thứ hai dù cmap font có Ữ (composite như Ử vẫn
   vẽ được ở «ĐÃ GỬI»). Đổi nhãn `da_giu` thành «Ký ức» (đúng nghĩa hơn) và **ghi nợ nguyên nhân**.
4. `khacGi` không lặp lý do (tờ đã in «Vì: …»).

## Đọc mù khung đầy đủ, vòng 1 — reviewer context mới, 12 khung fs1.0 sáng, câu hỏi «việc cần làm bây giờ là gì?»

Xác nhận đúng luật, không sửa: tờ đọc là **tờ giấy** (bo nhỏ, mép, vết gấp mép tới mép); góc coral đọc là **góc gấp** ở cả sáng lẫn tối;
con dấu có chữ nên trạng thái đọc được không nhờ màu; «Những tuần trước» là hàng; sheet dùng đúng chỗ; không kicker, không gạch dài.
Phán quyết: **fix**, mười hai điều vật chất (sửa ở `e1f6016d`):

| # | Đọc mù thấy | Sửa |
|---|---|---|
| 1 | Dấu đặc coral «KÝ ỨC»/«ĐÃ CHỐT» tranh mắt với nút chân coral; việc bây giờ tìm ra bằng **loại trừ** | `Stamp` thêm tone `ink`: trạng thái đã thành dấu mực đặc, tuần khép dấu mực viền; coral chỉ ở tờ đang hỏi và nút dẫn |
| 2 | Khối «đổi gì» in trên tờ ký ức, đọc là thừa | chỉ khi tờ còn chờ quyết và version > 1 |
| 3 | Tờ đã chốt: «Đã đi rồi» (soft) + «Huỷ buổi này» + nút chân «Rủ đi chơi» = ba việc | ẩn nút chân khi tờ trên bàn là chốt/đã đi; «Đã đi rồi» về ghost |
| 4 | `outline` mạnh hơn `soft`: «Sửa trước khi gửi», «Đề nghị sửa» đọc thành nút chính | nút chính `solid`, nút hai `outline` — **ngược giả định spec §15.3** (soft/outline); giữ số đo này |
| 5 | Dấu «ĐÃ GỬI» trên tờ người nhận đang cầm đọc như «tôi đã gửi» | `nhanDau(to, toiId)` theo vai: «Gửi cho bạn», «Chờ bạn», «Bạn đã ừ»/«Người ấy đã ừ», «Bạn đã rút» |
| 6 | Xem trước đóng sổ nói «1 tờ sẽ khoá» rồi đóng thành «Đã bỏ» | tách `so_to_huy` (đang chờ → huỷ) khỏi `so_to_khoa` (đã chốt → khoá); số 0 thành câu. **Wire `close/preview` cần thêm `so_to_huy`** |
| 7 | Segmented «Một đôi» sáng khi mới đề nghị | chỉ theo `batDoi`; đang chờ là dấu mực «Đã đề nghị» |
| 8 | «Những tuần trước» hai khung hai thứ tự (sắp theo `sent_at` cuối) | theo lúc tạo, mới nhất trước |
| 9 | Khối bản trải nghiệm ba dòng coral ngang hàng hành động thật | một `ListRow` mở sheet «Đóng vai người ấy» |
| 10 | «Giữ lại: …» nhạt nhất trong khi là nội dung có giá trị nhất | `body`/`ink` |
| 11 | «Rủ một người đi chơi» không phân biệt với «Tạo cuộc hẹn» | icon họ tờ giấy, mô tả «Một tờ giấy cho hai người, mỗi tuần» |
| 12 | «Vì: …» dưới tờ ở v2 là lý do lần sửa | dưới tờ là lý do của buổi (v1); lý do sửa vào khối «đổi gì» |

Hai điều tác giả không chắc: **ký ức trên bàn** đọc đúng là chuyện đã qua và việc bây giờ vẫn ra «Rủ đi chơi» (giữ ưu tiên ba §15.1,
sửa 1/2/10 là đủ); **khối bản trải nghiệm** có làm mờ (sửa 9). Ghi nhận không sửa: avatar «Người ấy» lấy chữ «Ấ» từ chữ cuối tên vai.

## Đọc mù vòng 2 — 12 khung fs1.0 sáng trên `0c0093ba`, reviewer context mới

Cả 12 điều vòng 1 **khớp**, không hồi quy (dấu mực không tranh mắt nữa, coral chỉ ở tờ đang hỏi; khối «đổi gì» hết trên ký ức; `solid`
đọc là nút chính; dấu «GỬI CHO BẠN» đúng vai; Segmented không sáng khi mới đề nghị; thứ tự nhóm dưới ổn định; hàng bản trải nghiệm không
tranh việc; «Giữ lại» đọc được; «Rủ một người» phân biệt được). Phán quyết **fix rồi ship** với tám điều chữ/điều kiện (sửa ở commit kế):

| # | Đọc mù thấy | Sửa |
|---|---|---|
| 1 | Lý do fixture «Sớm hơn một chút» khi giờ đổi 18:30 → 19:00 (muộn hơn) | hướng đổi quyết lý do |
| 2 | «Sổ này đã đóng» nói «tờ đã chốt và điều đã giữ» khi không có tờ chốt | thân rẽ theo dữ liệu |
| 3 | «Đã đi rồi» hiện trước ngày hẹn | **không sửa ở client**: client không đọc đồng hồ (§3.3 luật 6); Phase 4 để máy chủ gác `can_record_done` |
| 4 | Xem trước gọi bản phác «chỉ bạn thấy» là «đang chờ» | `so_nhap_bo` tách khỏi `so_to_huy`; wire `close/preview` cần cả hai |
| 5 | «Những tuần trước» chứa ngày muộn hơn tờ trên bàn | «Tờ đã khép» — nhóm theo trạng thái |
| 6 | Câu giữ của flow trùng câu fixture → đọc thành lặp | câu khác |
| 7 | «Vì: …» còn trên tờ ký ức | ẩn ở `da_giu` |
| 8 | Dấu mực đặc chữ trắng đọc như nút | mọi dấu là viền |

Ghi nhận không sửa (không vật chất): badge «Dữ liệu demo» lặp hai lần trên khung «Tạo mới» (có sẵn ở hội bạn); avatar «Người ấy» lấy chữ «Ấ».

**Nợ ghi lại, không chặn Phase 2:**
1. **«Đã đi rồi» hiện trước ngày hẹn.** Client không đọc đồng hồ (§3.3 luật 6) và `content.ngay` là chuỗi người đọc («Thứ Bảy 20/09»),
   không phải mốc máy so được. Phase 3 cho `PaperResponse` một cờ do máy chủ tính (tên đề xuất `co_the_ghi_da_di`), Phase 4 đọc cờ ấy.
2. **Xem trước đóng sổ có năm dòng**, ba trong đó là «Không có …». Rõ và thành thật trên một hộp thoại phá huỷ, nhưng hai dòng có hậu quả
   bị chìm. Đo lại ở lát 2 khi sổ có đủ loại tờ; nếu vẫn chìm thì chỉ in dòng có hậu quả.
3. **Con dấu «ĐÃ GIỮ» vẽ thiếu chữ thứ hai** trên emulator (đã đổi nhãn thành «Ký ức»); nguyên nhân chưa tìm ra, cmap font có đủ glyph.

## Điều `npm test` đủ bộ bắt được mà cổng lẻ không

Cổng «không giá trị hiển thị nào lấy id thô làm mặc định» (`mac-dinh-am-tham-id`) đọc `to.outing_id ?? outingId` trong
`so-fixture.ts` là **giá trị hiển thị rơi về id thô**. Ở đây nó là khoá idempotency của K3 (gọi lại thì giữ liên kết cũ), không
phải chữ hiện ra, nhưng cổng không phân biệt được — nên viết bằng **nhánh tường minh** thay `??`, và nói lý do ngay tại chỗ.
Bài học cũ, lặp lại: chạy `npm test` **đủ bộ**, không chỉ các file test mình vừa viết.

## Bảng Maestro

Bảy flow `.maestro-bs-r18/100–106` × bốn cấu hình (chữ 1.0/2.0 × sáng/tối) = **17 trạng thái × 4 = 68 ảnh**, mọi flow exit 0
(flow 104 ở fs1.0 sáng phải chạy lại một lần, xem «Máy ảo dùng chung» dưới). Mọi `assertVisible` đặt trên **nhãn đọc được**
(spec §12.1), không trên id của hình.

| Flow | Chứng minh |
|---|---|
| 100 | Một tuần trọn vòng: trống → Nếp phác → gửi → người kia xem → người kia ừ → chốt → đã đi → giữ một điều |
| 101 | Đề nghị sửa là **phiên bản 2 do người kia gửi**; «đổi gì» in đúng chiều (18:30 → 18:00) kèm «Vì sao sửa»; tôi ừ → chốt |
| 102 | Đóng sổ có xem trước nói đúng số phận từng loại tờ; đóng rồi thì sổ khép |
| 103 | Loại sổ là **đề nghị**; chỉ khi người kia đồng ý mới thành «Một đôi», header đổi theo |
| 104 | Rút lại (chưa ai xem) và nghỉ tuần đều là trạng thái cuối, không còn nút nào |
| 105 | Chat cặp mang hàng ghim «Tờ giấy của hai mình» và dẫn vào không gian giấy |
| 106 | «Tạo mới» → «Rủ một người đi chơi» → tờ đã phác sẵn |

**Hồi quy hội bạn:** bảng fixture mặc định `.maestro/00–11` chạy lại trên cây này: **11/11 xanh**, canary 09 đỏ đúng.
Lượt đầu flow 05 và 07 đỏ; hoá ra là **cổng cũ mù** (khẳng định câu mô tả địa điểm đã bị đổi từ `4481762f`, không còn
trong `fixtures.ts` của nhánh này lẫn của `main`) — bảng chạy tay nên nó đỏ trong im lặng từ đó. Sửa assert ở `d087f826`.

## Máy ảo dùng chung: một bảng xanh có thể là ảnh của nhánh người khác

Giữa hai cấu hình, **lane khác khởi động lại Metro của họ ở cổng 8098** và mở dev client của họ; URL đã lưu của dev client
trên máy ảo trỏ sang cây `/home/lakiet/mobile`, nên `stopApp`/`launchApp` của tôi nạp **bundle của họ**. `adb reverse tcp:8096`
vẫn thành công, không dấu hiệu nào khác. Cả cấu hình tối chụp nhầm nhánh người khác, và **mọi flow chỉ đọc màn hội bạn vẫn
xanh** — chỉ màn đăng nhập thiếu cửa `CUA_FIXTURE_DEV` mới lộ.

Chốt: `assertVisible: ".*${TREE_FINGERPRINT}.*"` vào **`_vao-app-sach`** (flow mọi flow r18 đi qua, không chỉ flow 00), bộ chạy
truyền `-e TREE_FINGERPRINT=` và **trỏ lại dev client trước mỗi cấu hình**. Sau chốt, lane kia đá app về màn chào giữa flow 104
thì flow **đỏ** thay vì chụp nhầm — đúng cái ta muốn; chạy lại là xanh.

## Cổng đã chạy trên cây cuối

`npm test` đủ bộ · `tsc --noEmit` sạch · `pytest services/api/tests tests` · `to-giay-luat` 10 ca · `to-giay-fixture` 5 ca ·
`so-ban-tinh-mot-cho` 4 ca (hai chiều) · `dau-gach-dai` · `rudi-khong-hex` · `check_screens_reachable` 65/65, 0 pin ·
detector Impeccable `[]` trên mọi file mới và file đổi · repo guard theo từng commit · Maestro r18 68 ảnh × 4 cấu hình ·
bảng fixture mặc định 11/11 + canary.
