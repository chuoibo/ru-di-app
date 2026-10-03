# Cộng đồng năm chữ-tab và ba nhóm từ critique — kết quả

- Nhánh `claude/cong-dong-chu-tab`, base `d97430db` (thanh tab 5 cột đã vào main). Nguồn: critique Impeccable 02/10
  (`docs/claude/2026-10-02/thanh-tab-5-cot/ket-qua.md`), chủ sản phẩm chọn cả bốn nhóm và chọn «Chữ-tab nhỏ, 5 mục» cho
  điều hướng Cộng đồng.
- Cách làm: native trong phiên, impeccable-pipeline (Extension, code-led, decision reference = lời chọn của chủ sản phẩm).
  Plugin Impeccable 4.5.0 đổi chỗ cài; hai agent của nó (finish reviewer, documenter) không đăng ký trong phiên, nên
  chạy bằng agent general-purpose đọc đúng file hướng dẫn của chúng.
- Verdict finish review (ngữ cảnh mới): `recapture` → `fix` (5 mục) → xác nhận 4/5 + 1 partial → `fix` mục 1 →
  **mục 1–5 resolved**, không hồi quy vật chất; mục 6 (tài liệu) qua documenter.

## Đã làm

| Phần | Commit |
|---|---|
| Năm chữ-tab `HangChuTab`; menu cài đặt bỏ «Bài đã lưu», «Bài của tôi», «Thông báo»; trạng thái rỗng hai tab mới; thẻ xin cá nhân hoá chỉ ở «Dành cho bạn»; chạm lại cột đang sáng cuộn lên đầu (`useChamLaiTab`); sao trước điểm đánh giá (`moDauBangSao`); chữ «Tạo» trong nút con dấu; sổ tay Nếp `cong-dong.md` | `5874620d` |
| Theo finish review: khoảng tab tự chỉnh để mép cắt chữ (`khoangCachLo`), tab cạnh ló khi cuộn, dòng sự thật hai dòng (`chiaDongSuThat`), nhịp menu 6/16/8, kẻ tóc trong cột tablet, chữ trạng thái rỗng | `39ab9dc2` |
| Mép trái cũng cắt chữ khi chọn tab cuối (đệm cuối 16…24dp) | `de432f63` |
| DESIGN.md (documenter), hai surface brief, comment RudiTabBar, ảnh bằng chứng, file này | commit tài liệu |

Nhóm «Nút ✨» và phần giá của nhóm «Giá và thẻ quán» đã do B5 (`f808dbbe`) sửa; đợt này chỉ kiểm lại không tốn lượt
gọi AI (`kiem-ux/tt5-ai-nut.mjs`): ô trống bấm ✨ đặt câu mẫu, con trỏ vào ô, có lời hướng dẫn; câu dài tìm ra 0 thì mời
«Hỏi Rủ Đi AI», không «Chưa thấy nơi phù hợp». Bấm ✨ khi đã có chữ gọi AI trả tiền nên chỉ đọc mã (`bamHoi`).

## Số đo

- npm test 1490/1490 (MOBILE_REQUIRE_WEB_A11Y=1) ở `de432f63`; tsc sạch; go test `./internal/huongdan` ok, recall@5 và
  mọi MRR giữ đúng số ghim sau khi sửa sổ tay.
- Đột biến (mỗi cái đỏ đúng test đoán, identity xanh): M1 `tabState(!chọn)`; M2 bỏ kẹp 0 của `cuonDeThay`; M3 sao theo
  phần tử đầu; M4 xin cá nhân hoá mọi tab; M5 trả «Bài đã lưu» vào menu; M6 bỏ chỉnh khoảng; M7 lề phải dùng lề trái;
  M8 giờ giữ «·»; M9 giờ còn ở dòng đầu; M10 bỏ điều kiện mép trái.
- Probe `kiem-ux/cham-lai-tab.mjs` (web, 390×520): main `d97430db` đỏ 6/6 ca đo được → 6/6 xanh (/explore, /explore
  demo, /community, /plan, /profile từ 300 về 0, URL giữ; chạm chữ «Tạo» mở /create). /messages không đủ dài: không đo.
- Probe `kiem-ux/thanh-tab-5.mjs` 22/23: hàng năm tab đúng thứ tự; mép cắt chữ ở 390 (lúc nghỉ x 374–445; chọn tab cuối
  mép trái x −54…39, tab cuối kết thúc 375) và 320; menu 16/8/8/8; kẻ tóc tablet trùng ô tìm 300–828. Ca «thẻ bài»
  không có bài có ảnh trên stack.
- Android (Pixel 6 AVD riêng, chỉ-đọc, cổng 5610; dev client từ debug APK của cây gốc 01/10 — không đổi native từ đó;
  bundle từ Metro của nhánh, «Android Bundled … 2597 modules»): chạm lại cột Khám phá về đúng đầu (uiautomator: «Chỗ hay
  ở» y 1226, dòng điểm đến y 276 trước/sau), Cá nhân về đầu; chạm chữ «Tạo» và nửa nhô lên của con dấu (y 2150, trên mép
  thanh 2170) đều mở khay; 1.0 «Bài» bị mép cắt; 1.3 khoảng tự về 16dp, «Đã lưu» hiện 17dp; chọn tab cuối: 1.3 mở bằng
  «ang theo dõi», 1.0 bằng «ho bạn»; tablet 800dp năm tab vừa cột.

Ảnh: `evidence/EV-CDT-WEB.jpg`, `evidence/EV-CDT-ANDROID.jpg` (ghim sha256). Ảnh đầy đủ của hai vòng và các lượt chụp
lại ở `.impeccable/review/cdt*/` (gitignore).

## Quyết định thay chủ sản phẩm

- Chạm lại cột đang sáng cuộn lên đầu cho **mọi** cột, không chỉ Khám phá (quy ước của app điện thoại). Nếu sai: bỏ
  `useChamLaiTab` khỏi `RudiScreen`, giữ ở Cộng đồng/Khám phá.
- Sao ở cả thẻ so sánh và hàng, không chỉ thẻ đầu: «4.8 (64)» trần không nói nó đếm gì ở chỗ nào cũng vậy.
- **«Điều mình muốn giữ» giữ trong menu cài đặt** dù chủ sản phẩm liệt kê ba mục: đó là lối vào duy nhất của
  `/community/keeps`. **Chủ sản phẩm xác nhận 03/10: giữ trong menu** (chọn giữ nguyên, không dời sang tab Cá nhân
  hay «Đã lưu», không bỏ tính năng).
- «Thông báo» bỏ khỏi menu vì chuông đã ở hàng đầu (có chấm khi có tin mới).
- Thẻ xin cá nhân hoá chỉ ở «Dành cho bạn» (chỉ bảng tin đó học từ tương tác).
- Không làm mờ mép hàng tab: `direction.md` cấm gradient; thay bằng khoảng tab 12…32dp và đệm cuối 16…24dp.
- Dòng giờ đổi « · » thành «, » giữa trạng thái và giờ (chỉ ở thẻ quán; chuỗi gốc `cauMoCua` và trang chi tiết không đổi).
- Stack QA chung tắt theo lần khởi động lại máy: bật lại đúng ba container cũ (không build, không migrate, không reseed).
- Máy ảo của chiến dịch `rudi-b9a` đang chạy không bị mượn; dùng AVD riêng ở chế độ chỉ-đọc, gỡ bản release có dấu vân
  lạ trên AVD đó (mất khi tắt máy ảo).

## Tiếp theo (03/10, sau khi vào main `cff7e288`)

- Gạch mực của `HangChuTab` trượt sang tab mới trong `standard`, nhảy khi giảm chuyển động (`viTriGach`; DESIGN.md).
  Web 390: ở 60 ms gạch x 160 (giữa 16 và 228), 700 ms trùng tab; `reducedMotion` 60 ms đã trùng. Android 30 fps:
  43–267 → 520–711 → 553–741 → 613–797 px trong ~5 khung; tắt hoạt ảnh hệ thống: nhảy trong một khung (43–267 →
  615–799). Tablet web 530/75 = tab; Android 800 dp x 810–951 ≈ tab 809–950. Finish review `recapture` → **`ship`**.
- `TestQuenKhiConHangThiThuLai` (nepnho) hết giẫm việc xoá tài khoản của gói khác: lượt xoá chạy mọi việc tới hạn của
  cả database dùng chung, nên test kiểm việc của chính nó và rollback khi hỏng (trước đó treo 30 phút).
- Lỗi quyền riêng tư có từ B7 (`community_notifications.actor_id` không được dọn khi xoá tài khoản; `ON DELETE SET
  NULL` không chạy vì xoá mềm) **không vào main ở đợt này**: phiên đang giữ việc dở B8 ở cây gốc đã tự sửa cùng lỗi
  (migration cộng đồng thứ 6 `notification_actor_erasure.sql`, chưa commit). Bản của mình (`e2dc63f1` trên nhánh
  `claude/cong-dong-chu-tab`, kèm test PostgreSQL chạy trong transaction rollback) tương đương; đưa nó vào main sẽ đè
  lên file đang sửa dở của phiên kia. Bản của phiên kia vào main thì `TestXoaTaiKhoanKhongConHangNaoCuaNguoi` xanh.
- Không làm: ký hoạ cho trạng thái rỗng — bốn trạng thái rỗng của Cộng đồng sẽ thành hai kiểu nếu chỉ đổi hai cái mới.

## Việc nhỏ để lại của đợt thanh tab (03/10)

| Việc | Commit |
|---|---|
| Nếp biết con dấu «Tạo mới» trên mọi màn có thanh tab và thẻ «Viết bài» của khay: «viết bài ở đâu» từ mọi tab là [Tạo mới, Viết bài] (trước: ba chạm vòng qua Lên plan, bước cuối không nhãn) | `0e4a6126` |
| Album giữ đúng bề rộng đo được (không làm tròn, hết lệch khi lật nhiều trang); nút Lưu chờ như nút thích; vạch thanh tab đi theo ô do `oCuaCot` tính (Lên plan → Tin nhắn hết nhảy một ô ở cuối) | `314a7e5f` |

Không làm: bước «Khám phá» của Nếp mở mục xem sau cùng (có thể là Cộng đồng) — hàng «Địa điểm | Cộng đồng» ngay trên
đầu màn, một chạm sửa được; thêm bước «Địa điểm» vào mọi đường tới Khám phá sẽ dài thêm cho trường hợp thường gặp.
Lề PostDetail (20) quanh PostCard (16) không đo được trên stack (không có bài), để lại.

## Còn mở

- `TestXoaTaiKhoanKhongConHangNaoCuaNguoi` còn đỏ trên main tới khi bản sửa của phiên B8 vào main (xem trên).
- Đổi cỡ chữ Android khi app đang chạy làm chữ bị cắt khắp nơi tới khi mở lại app (có sẵn, ngoài phạm vi).
- Ceiling reviewer gợi ý: gạch trượt đã làm; ký hoạ cho trạng thái rỗng để đợt vẽ lại cả bốn.
- `.impeccable/design.json` lệch schema (CONTEXT_STALE), `Chip vaiTab` không còn nơi gọi.
- iOS chưa chụp (máy Linux).
