# Thanh tab 5 cột: gộp Cộng đồng vào Khám phá

- Ngày: 2026-10-01 · nhánh `claude/thanh-tab-5-cot` · base `bc8dbdfb`
- Trạng thái: **chờ chủ sản phẩm duyệt spec**. Chưa có dòng code nào.
- Nguồn: chủ sản phẩm chê thanh 6 cột (ảnh chụp iPhone 20:15, 01/10) và gửi mockup ba bảng: «Bố cục đề xuất»,
  «Tối & responsive», «Thanh tab 5 cột». Mockup là ảnh của chủ sản phẩm, không đưa vào Git.
- Gỡ chặn mục «Con dấu Tạo mới trên mọi tab» (`ui-ux-upgrade/feature-plan.md`, BLOCKED chờ mockup).

## Vì sao

Thanh hiện có 5 tab cộng con dấu «Tạo» = 6 cột (`RudiTabBar.tsx`, con dấu ở cột thứ ba). Số cột chẵn thì không có
cột giữa, nên con dấu luôn lệch. Đổi padding không cứu được, phải đổi số cột.

Đánh giá vai trò cho thấy Cộng đồng và Khám phá cùng trả lời một câu: «đi đâu?». Cộng đồng trả lời bằng chuyện của
người khác, Khám phá bằng danh mục quán. Gộp hai tab này về một thì thanh còn 4 tab + Tạo = 5 cột, Tạo nằm đúng tâm.

Hướng đã loại:
- Bỏ Lên plan như mockup tìm trên mạng: đẩy việc chính của app vào menu, danh sách kèo sắp tới mất chỗ ở.
- Avatar thay tab Cá nhân.
- Viên thuốc + nút tách: chật ở 320dp.

## Đã chốt với chủ sản phẩm

1. Thanh: `Khám phá · Lên plan · (+) · Tin nhắn · Cá nhân`.
2. Khám phá có hai mục `Địa điểm | Cộng đồng`, **mặc định Địa điểm**.
3. Khay «Mình làm gì tiếp?» giữ nguyên thiết kế. Không dùng menu xòe 3 bong bóng.
4. Thân mục Địa điểm giữ bố cục hiện tại: một thẻ lớn → hai quán so → hàng. Chỉ mặc thêm lớp áo của mockup; không
   dùng băng cuộn ngang.
5. Các khung trong mockup là chuẩn hình ảnh, trừ những chỗ ghi ở «Lệch so với mockup».

## Thiết kế

### 1. Thanh tab (điện thoại)

- 5 cột bằng nhau: 78dp ở 390, 64dp ở 320. Con dấu ở cột thứ ba, tâm con dấu trùng tâm màn (±1dp).
- Con dấu giữ như hiện tại: tròn coral 56dp, viền màu nền, nhô nửa trên mép thanh, nhãn «Tạo» thẳng hàng nhãn tab.
- Icon và vạch giữ như hiện tại: la bàn, bản đồ, bóng chat, avatar; tab đang chọn có icon đặc, coral, băng keo trên mép.
- Đang ở mục Cộng đồng thì cột **Khám phá** sáng: Cộng đồng là một phần của Khám phá.
- Chữ to (320 × 1.3): nhãn được xuống hai dòng, thanh tự cao lên. Cơ chế này đã có (`tabBarHeight`).

### 2. Thanh dọc (tablet ≥600dp)

Từ trên xuống: logo «Rủ Đi», con dấu «Tạo», rồi 4 tab. Vạch dọc bên trái đánh dấu tab đang chọn.

### 3. Hàng tiêu đề Khám phá (dùng chung cho hai mục)

```
  Địa điểm   Cộng đồng                 [icon phải]
  ▔▔▔▔▔▔
```

- Hai chữ cỡ tiêu đề. Mục đang chọn: chữ mực đậm, có băng keo coral bên dưới. Mục kia: chữ `inkFaint`.
- Icon bên phải:
  - mục Cộng đồng: «Cài đặt bảng tin» (`options-outline`);
  - mục Địa điểm: không có icon;
  - bản demo chưa đăng nhập: nhãn «Dữ liệu demo» đứng ở chỗ icon.
- Hàng này dính trên đầu khi cuộn.
- Hai chữ là `tablist` có hai `tab` (`aria-selected`). Icon nằm ngoài danh sách, giống cách con dấu nằm ngoài thanh tab.
- Màn 320 × 1.3: không cắt chữ, không khoá cỡ chữ. Nếu đo thấy tràn thì hàng tiêu đề cuộn ngang.
- Logo «Rủ Đi» ở đầu Khám phá được bỏ, đúng như mockup.

### 4. Mục Địa điểm

Thứ tự từ trên xuống giữ nguyên: dòng điểm đến «📍 Đà Lạt · đổi nơi khác ▾» → sân khấu thành phố → ô tìm + nút ✨ →
hàng chip loại. Đổi hai chỗ:

- **Tiêu đề mục kết quả.** Thay «48 nơi ở Đà Lạt» bằng tiêu đề lớn **«Chỗ hay ở Đà Lạt»**, dòng phụ nhỏ «48 nơi».
  - Khi đang lọc hoặc tìm, giữ như hiện tại: «N kết quả» + «Xóa lọc».
  - Không dùng chữ «Gần bạn, đúng gu»: server xếp quán theo `places.id` (`repo/places.go`), không theo vị trí,
    không theo gu. Bản demo đang có dòng đó cũng là nói sai, sửa luôn cho cùng câu.
- **Thẻ đầu theo kiểu thẻ mockup.** Ảnh rộng hết cột, tim ở góc trên phải *trên ảnh*. Dưới ảnh: tên quán, một dòng
  mô tả (cắt bằng dấu …), rồi một hàng chip «✨ View đẹp» và giá. Giá không bao giờ bị cắt; chật thì chip xuống
  dòng. Phần «hai quán đặt cạnh để so» và các hàng bên dưới giữ nguyên.

### 5. Mục Cộng đồng

- Bỏ khối tiêu đề cũ (chữ «Cộng đồng» cỡ display + dòng phụ). Hàng tiêu đề chung đã gánh việc đó.
- Bỏ nút coral «Đăng khoảnh khắc» ở đầu màn: con dấu Tạo mở từ mục này đã đưa «Viết bài» lên đầu khay.
- Ô tìm «Đi đâu, ăn gì, trải nghiệm gì?» giữ nguyên.
- Ba tab gạch chân `Dành cho bạn · Đang theo dõi · Thịnh hành` thành **ba chip**: chip đang chọn nền coral nhạt, chữ
  coral. Hàng chip cuộn ngang khi chật. Vẫn là `tablist` để trình đọc màn hình biết đang chọn mục nào.
- Đang xem một chủ đề (`?topic=`): dưới hàng chip có một dòng «#chủ đề ×»; chạm × để bỏ lọc. Trước đây chủ đề thay
  chữ «Cộng đồng» ở đầu màn.
- Chế độ «Bài đã lưu» và «Bài của tôi» (mở từ sheet cài đặt) giữ dòng tiêu đề h2 hiện có của chúng.

### 6. Khay Tạo

- Thiết kế khay giữ nguyên. Thẻ đầu theo chỗ mở:
  - Khám phá › Địa điểm → «Tạo cuộc hẹn»;
  - Khám phá › Cộng đồng → «Viết bài»;
  - Lên plan → «Tạo cuộc hẹn»;
  - Tin nhắn → «Hẹn người thương» nếu có chat đôi, không thì «Đăng story»;
  - Cá nhân → «Đăng kỷ niệm».
- Các thẻ còn lại giữ thứ tự ổn định cũ. Ví dụ mở từ Cộng đồng: Viết bài → Tạo cuộc hẹn → Chia hóa đơn →
  Đăng kỷ niệm → Đăng story → Hẹn người thương.
- Dòng phụ của «Viết bài» đổi thành «Kể một điều hay với cộng đồng», theo mockup.
- Khay phủ lên thanh tab như hiện tại. Tab đang đứng vẫn giữ trạng thái sáng bên dưới.

### 7. Điều hướng

- Mở app (cold start) → Khám phá › Địa điểm.
- Rời Khám phá rồi chạm lại tab Khám phá → về mục xem sau cùng. Mở lại app → về Địa điểm.
- Đổi mục bằng chạm. Không vuốt ngang: mục Địa điểm có hàng chip và thẻ cuộn ngang, hai cử chỉ sẽ tranh nhau.
- Đổi mục là một bước trong lịch sử, như đổi tab: Back trở về mục trước, như thanh địa chỉ trên web.
- Link và thông báo tới `/community`, `/community/posts/…` không đổi đường dẫn; chúng mở Khám phá › Cộng đồng.

## Cách làm

**Hai route, một cột trên thanh.** `app/(tabs)/explore.tsx` và `app/(tabs)/community.tsx` đều ở lại.

- `_layout.tsx` đưa `explore` lên đầu. `community` vẫn là một màn trong `Tabs` nhưng được đánh dấu không có cột riêng.
- `RudiTabBar` chỉ dựng cột cho các route có cột (4 route + con dấu = 5 cột, con dấu ở vị trí 2). Khi route đang mở là
  `community`, bar coi như `explore` đang chọn (cả vạch lẫn `aria-selected`).
- Hàm thuần mới tính phần đó: vào là danh sách route + route đang mở; ra là các cột, vị trí con dấu, cột đang chọn.
  Test đơn vị chạy trên hàm này.
- `DauKhamPha` (mới, `src/rudi/ui/`) là hàng tiêu đề chung, nằm đầu cả `ExploreLive`, `Discovery` (bản demo) và
  `CommunityScreen`. Chạm một mục gọi `router.navigate("/explore" | "/community")`. Mục xem sau cùng là một biến trong
  bộ nhớ (không lưu đĩa); thanh tab đọc biến đó khi chạm cột Khám phá.

Vì sao không gộp thành một route có state nội bộ:
- giữ nguyên `/community` cho link, thông báo, `lui-ve.ts`, `PostDetail` (`router.replace("/(tabs)/community")`) và
  `create.tsx`;
- giữ `useNepNguCanh` riêng cho từng màn, và bộ rút hướng dẫn (`tools/rut-huong-dan.mjs`) vẫn đọc được hai màn;
- mỗi màn giữ cuộn và `FlatList` của nó; diff trong hai màn lớn nhỏ hơn.

`tao-moi.ts` không đổi: `?tu=community` vẫn là một chỗ mở hợp lệ.

### Chỗ chạm

| File | Đổi |
|---|---|
| `app/(tabs)/_layout.tsx` | thứ tự, `community` không có cột |
| `src/rudi/ui/RudiTabBar.tsx` (+ hàm thuần mới) | lọc cột, gộp trạng thái chọn, logo đầu thanh dọc |
| `src/rudi/ui/DauKhamPha.tsx` | mới |
| `src/rudi/screens/explore/ExploreLive.tsx` | hàng tiêu đề chung, tiêu đề mục kết quả |
| `src/rudi/screens/explore/HangDiaDiem.tsx` | kiểu thẻ đầu (`PlaceLead`) |
| `src/rudi/screens/Discovery.tsx` | hàng tiêu đề chung, bỏ «Gần bạn, đúng gu» |
| `src/rudi/community/CommunityScreen.tsx` | hàng tiêu đề chung, chip, bỏ nút đăng, dòng chủ đề |
| `src/rudi/screens/Create.tsx` | dòng phụ «Viết bài» |
| `services/core/internal/huongdan/data/cong-dong.md`, `kham-pha.md`, `_rut.json` | hướng dẫn của Nếp: «tab Cộng đồng» thành «mục Cộng đồng trong Khám phá»; rút lại `_rut.json` |

Maestro: không flow nào chạm nhãn «Cộng đồng»; nhãn «Khám phá» giữ nguyên.

## Lệch so với mockup (có lý do)

- **Chữ «Gần bạn, đúng gu ›» và mũi tên ›**: thứ tự quán không theo vị trí hay gu, và chưa có trang «tất cả» cho
  mũi tên dẫn tới. Dùng «Chỗ hay ở Đà Lạt» + «48 nơi», không có ›.
- **Băng cuộn ngang**: không dùng, theo mục 4 phần «Đã chốt».
- **Khung 03 không có «Tạo cuộc hẹn»** và tắt sáng Khám phá khi khay mở: theo mục 6 ở trên.
- **Thanh tab vẫn hiện dưới khay**: đợt này khay phủ thanh như hiện tại.
- **Badge số trên Tin nhắn**: app chưa có nguồn số tin chưa đọc, nên không làm. Luật cho lúc có badge: chỉ hiện chấm
  số đỏ, icon và nhãn vẫn xám.
- **Thân Lên plan, bản đồ, thẻ «Kế hoạch Đà Lạt»** trong khung thanh tab: chỉ là minh hoạ, không thuộc đợt này.

## Đã đóng (trước là «Còn mở»)

- **Màu nhãn «Tạo»: coral, theo mockup.** Chủ sản phẩm duyệt spec với lời dặn «bám sát mockup» (02/10), nên giữ
  mặc định đã ghi. Hai nhãn coral cùng lúc là cái giá đã biết.

## Bổ sung khi lập kế hoạch (02/10)

Rà code và hỏi lại chủ sản phẩm ra 8 điểm spec trên chưa đúng hoặc chưa đủ. Chúng thắng phần tương ứng ở trên.
Kế hoạch: `docs/claude/2026-10-02/thanh-tab-5-cot/ke-hoach.md`.

1. **Chủ đề** không sống trên tab: `/community/topic` là route stack re-export `CommunityScreen`. Trang đó giữ đầu
   màn cũ (tiêu đề = chủ đề, cài đặt, nút đăng), không có hàng «Địa điểm | Cộng đồng». Bỏ ý «dòng #chủ đề ×» ở mục 5.
2. **Nếp**: test Go (`duong_test.go`: `tabLayout`, `TestTabRutBangTabLayout`, `TestTabKhopLayout`) và test JS
   `huong-dan-khop-ma` coi mọi route trong `app/(tabs)/` là một cột cách nhau một chạm. Bộ rút ghi thêm
   `muc_trong_tab` (`{"community": "explore"}`); Go và JS học «route không cột»: không là tab của thanh, thanh vẫn
   hiện trên nó nên các cột khác (trừ cột chủ) cách một chạm; tới nó từ tab khác là hai chạm («Khám phá», «Cộng đồng»).
3. **Maestro**: `26-kham-pha-that.yaml` (dòng 26, 58, 124) chờ `"[0-9]+ nơi ở Đà Lạt"`, `38-anh-dia-diem.yaml:33`
   chờ `"[0-9]+ nơi ở .*"`, subflow `_community.yaml:11` bấm «Đăng khoảnh khắc». Sửa theo chữ và lối mới. Mục «Chỗ
   chạm» ở trên nói «không flow nào chạm» là sai.
4. **Cỡ chữ 1.3** không chụp được trên web (react-native-web ghim `fontScale` = 1). Bằng chứng 320 × 1.3 lấy từ
   Android emulator.
5. **Thẻ đầu** giữ một dòng phụ mờ (sao · km · giờ) dưới mô tả: mockup vẽ dữ liệu mẫu ít trường, bỏ thì thẻ đầu nói
   ít hơn các thẻ so sánh bên dưới.
6. **Chi tiết mockup chưa có ở trên**: sân khấu thành phố tràn hết bề ngang ở điện thoại; «· đổi nơi khác ▾» coral,
   tên thành phố màu mực; ô tìm Cộng đồng là `SearchField` viền tròn như Khám phá; thanh dọc tablet có logo «Rủ Đi»
   trên con dấu.
7. **Màu nhãn «Tạo»**: coral (xem «Đã đóng»).
8. **Thẻ bài Cộng đồng** (chủ sản phẩm chọn 02/10 sau khi hỏi ý kiến thẩm mỹ): ảnh/video rộng hết cột, album nhiều
   ảnh lật từng trang rộng hết cột có số «1/3»; nút Lưu 🔖 cuối hàng nút. **Không** bọc thẻ trắng (bài vẫn nằm trên
   nền giấy, ngăn bằng nét mảnh — `direction.md`); **không** dòng «· Đà Lạt» (bài không có trường địa điểm).

## Ngoài phạm vi

- «+» xoay thành «×», khay mọc ra từ con dấu (để dành B11).
- Khay sắp thứ tự theo giai đoạn chuyến đi (vừa đi xong → chia bill → kỷ niệm).
- Mục «Sắp tới» của Cá nhân lặp lại Lên plan.

## Cổng và bằng chứng

- `apps/mobile`: `npm test` toàn bộ, `tsc`, `build:check`. Go: `go test ./internal/huongdan/...` sau khi rút lại hướng dẫn.
- Test hàm thuần của thanh: 5 cột; con dấu ở vị trí 2; `community` đang mở → cột Khám phá chọn; thanh dọc → con dấu ở
  đầu. Đột biến tự nghĩ, dự đoán trước chỗ đỏ:
  - bỏ bước lọc route không có cột → 6 cột, đỏ ở ca «5 cột»;
  - `community` lấy chỉ số của chính nó → đỏ ở ca «cột Khám phá chọn»;
  - mục mặc định là Cộng đồng → đỏ ở ca mặc định của `DauKhamPha`.
- Ảnh chụp (mở ra nhìn, không chỉ bảng xanh):
  - C1 390 sáng: Địa điểm, Cộng đồng, khay mở từ Cộng đồng;
  - 390 tối;
  - C2 320 × 1.3;
  - tablet ngang.
- Số đo: tâm con dấu so với tâm màn (dp); bề rộng hàng tiêu đề so với chỗ trống ở 320 × 1.3. Số đo ghi vào commit message.
