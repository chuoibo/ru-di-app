# QC frontend: vì sao Rủ Đi trông như «tờ điền chữ», rà từng màn, từng pop-up

**Ngày 24-09-2026.** Người viết: phiên QC frontend (vai QA/QC, nộp phát hiện, **không nộp
diff mã sản phẩm**). Đo trên `main` @ `11fb3df`.

- protocol_version: không áp dụng (không phải lượt thí nghiệm).
- Verdict: không có (không có reviewer người; đây là báo cáo phát hiện).
- Phản hồi gốc của người dùng: *«UI cực kì đơn giản, chỉ giống một cái tờ điền chữ vô, thiếu
  chiều sâu về art, xấu, thiếu sức sống và câu chuyện của Nếp; frontend như AI thiếu kinh
  nghiệm develop.»*
- Kết luận một câu: **phản hồi đúng, và nó có nguyên nhân hệ thống đo được**, không phải do
  từng màn bị làm ẩu. Hệ thiết kế và kho hình vẽ của Rủ Đi rất giàu, nhưng gần như toàn bộ nằm
  ở màn chào, bản demo fixture và các trạng thái rỗng. Mọi màn **có dữ liệu thật** và mọi
  **pop-up tạo mới** đều rơi về cùng một khuôn: thanh trên, tiêu đề to, hai dòng giải thích xám,
  chồng ô nhập, một nút chữ nhật cam ở đáy.

---

## 0. Đọc nhanh (5 phút)

### 0.1 Bảy nguyên nhân gốc

| # | Nguyên nhân | Số đo | Hệ quả người dùng thấy |
|---|---|---|---|
| G1 | **Hai sản phẩm**: bản demo (fixture) có ảnh, hoá đơn giấy, ảnh bìa chuyến đi; bản thật (live) không có | Cùng màn «Lên plan»: demo có ảnh đồi Đà Lạt full-bleed; live là một thẻ hồng nhạt chữ. `DESIGN.md` tự ghi: reviewer trả `ship` cho **màn fixture**, «**chưa phủ: các cửa live**» | Leader duyệt cái đẹp trên demo; người dùng thật nhận cái trơn |
| G2 | **Một khuôn màn duy nhất** cho mọi việc | 22/41 màn dùng đúng `TopBar` + `Heading`; `RudiButton` (nút chữ nhật) xuất hiện **218** lần, `StampButton` (nút con dấu, chữ ký thương hiệu) **2** lần, cả hai ở Welcome/Login | Tạo nhóm, tạo kèo, mời, thêm bạn, đăng bài… nhìn như cùng một form |
| G3 | **Kho vẽ bị nhốt vào trạng thái rỗng** | Nếp có **24 tư thế**, 7 biểu cảm, 10 cảnh rỗng, 8 sticker. `Washi` dùng ở **1** màn, `ToGiay` (tờ thư gấp ba) **1**, `Motif` **1**, `CoverBand` **2** (chỉ Login/OTP). **7 màn không có một nét vẽ, con dấu hay ảnh nào**, trong đó có Kèo mới, Nhóm mới, Mời. Trên ảnh, Nếp/cảnh vẽ hiện chủ yếu khi màn **rỗng** | Có dữ liệu là mất hết art; app càng dùng nhiều càng trơn |
| G4 | **Nếp, nhân vật dẫn chuyện, bị thu nhỏ thành một mép giấy** | Trên mọi màn, Nếp là dải ~10px ở mép phải. Nếp chỉ xuất hiện thành hình ở 5 chỗ trong mã màn (dock, bảng Nếp, chat rỗng, sổ đôi đã đóng, sổ đôi **sau khi đã có buổi đi**) | Không có khoảnh khắc nào Nếp «làm việc» trước mắt người dùng: ghi sổ, lập sổ, tiền về, chốt kèo đều vắng Nếp |
| G5 | **Chữ gánh hết câu chuyện, và là chữ pháp lý** | Hầu hết màn có 2–4 dòng xám giải thích luật («Đây không phải số dư ngân hàng», «Nếp không đọc tin nhắn», «im lặng không phải đồng ý») | Cảm giác đọc điều khoản, không phải đọc nhật ký chuyến đi |
| G6 | **Không có khoảnh khắc**: kết thúc một luồng = một con tem 26dp + hai nút + 60% màn trống | Bước 5/5 chia bill, «Đã đề nghị lập sổ», «Đồng ý» lập sổ, «Gửi bình chọn» | Việc khó nhất (đòi tiền, rủ người yêu) kết thúc không có cảm xúc nào |
| G7 | **Danh sách đồng dạng, người không có mặt** | Thành viên, bạn bè, đợt thu, kết quả chia, «Rủ ai?»: 7–8 hàng giống hệt; avatar là một chữ cái trong vòng tròn cùng màu; «Rủ ai?» dùng **cùng một icon người** cho mọi bạn | Hội bạn, thứ app nói là trung tâm, không có gương mặt |

### 0.2 Mười việc nên làm trước (theo tác động / công)

1. **Chia bill bước 1 và bước 5**, **Đợt thu**: biến thành tờ hoá đơn + trang sổ có con dấu
   lớn, Nếp giữ máy ảnh ở bước 1 và dập dấu ở bước 5 (mục 5.4).
2. **Sổ hai người, lần đầu**: «Chưa có sổ hai người» và «Chưa có tờ nào tuần này» hiện
   **tờ thư gấp ba đang chờ** + Nếp `dua-giay`; hiện tại **cố ý không có hình** ở lần đầu
   (`KhongGianGiay.tsx:134-143`, `:176`) (mục 5.6).
3. **Khay «+» (Mình làm gì tiếp?)**: từ danh sách cài đặt thành bàn làm việc năm vật thể
   (lịch hẹn, hoá đơn, ảnh in, polaroid, tờ thư) (mục 6.1).
4. **Kèo mới** thành «thiệp rủ» có bản xem trước là tấm thiệp thật (mục 5.3).
5. **Nhóm mới** thành «bìa sổ mới» dùng `CoverBand` indigo (mục 5.2).
6. **Một hệ avatar có danh tính**: màu theo người (băm id), ảnh khi có, vai là con dấu
   (mục 7.4). Sửa một chỗ, 15 màn sống lại.
7. **Nếp có vai ở mỗi lần đổi trạng thái** (bảng ở mục 7.3), không chỉ ở mép.
8. **Khám phá live** có ảnh hoặc ký hoạ cho mọi quán, và một thẻ dẫn đầu (mục 5.1).
9. **Một khuôn màn thứ hai: «đầu trang sổ»** thay `TopBar + H1 + 2 dòng xám` (mục 7.1).
10. **Cổng mỹ thuật trên live**: review hình ảnh phải chạy trên thế giới seed thật
    (`npm run seed:rudi`), không chỉ fixture (mục 8).

---

## 1. Phạm vi và cách đo

### 1.1 Đã làm

- Dựng stack **thật** cô lập: PostgreSQL 16 cục bộ (Docker Hub trả 429 nên không dùng
  container), `alembic upgrade head`, seed danh mục + 15 điểm đến, API Python, cửa trước Go
  (`core serve`, `MOBILE_CORE_CANDIDATE_ROUTES=ported`), rồi `npm run seed:rudi` dựng
  «Team Đà Lạt»: 8 người, 13 tin + 1 bình chọn, 1 kèo 3 chặng, 1 hoá đơn 1.280.000đ,
  1 đợt thu, 5 kỷ niệm.
- Chạy app bằng `expo start --web` (react-native-web), viewport 412×915 @2x (cỡ Pixel 7),
  đăng nhập **bằng OTP thật** với 5 tài khoản seed khác nhau, đi bằng điều hướng trong app
  (phiên trên web chỉ nằm trong bộ nhớ, tải lại trang là mất phiên, đúng thiết kế).
- Chụp **~100 ảnh**: 41 route, 14 pop-up/sheet, 5 bước chia bill, luồng lập sổ hai người
  bằng **hai máy** (Tuấn Kiệt đề nghị, Minh Anh đồng ý), bản tối, và bản demo fixture để so.
- Đo trong mã: đếm cách dùng thành phần kit và lớp vẽ trên 41 file màn `src/rudi/**`.

### 1.2 Không làm, đừng đọc thành xanh

- **Không chạy native Android/iOS.** Web khác native ở bóng, font rendering, cử chỉ. Các nhận
  xét về bố cục/màu/mật độ chuyển được sang native; nhận xét gắn nhãn *[web]* thì không.
- **Không có khoá AI** trên stack: `Hỏi Rủ Đi AI`, Tờ hẹn AI, đọc bill từ ảnh chưa đo được
  nội dung thật.
- **Bản đồ không tải tile** (sandbox chặn mạng ngoài): màn Hành trình có vùng bản đồ trống.
  Toast đỏ «AJAXError: Failed to fetch» trên nhiều ảnh là lớp phủ dev của web vì lý do đó,
  **không phải phát hiện**.
- Chưa chụp: Welcome slide 2–4, Onboarding của tài khoản mới hoàn toàn, xem story, bài chi
  tiết, album có ảnh thật, trình xem ảnh, quét bill bằng camera, trang khách web
  (`/g/{token}`), cỡ chữ 1.3, tablet.
- Ảnh chụp **không đưa vào Git** (luật repo, ảnh đã bị gỡ khỏi cây vì dung lượng). Bản gộp
  24 tấm đã gửi kèm phiên này; cách tái lập ở mục 9.

### 1.3 Thang chấm dùng trong báo cáo

- **Sức sống** 1–5: màn có vật liệu, hình, nhịp, màu mang nghĩa hay chỉ là chữ trên nền.
- **Câu chuyện** 1–5: màn có nói mình đang ở đâu trong «buổi đi của hội», có Nếp, có người.
- Mức: 🔴 định nghĩa sản phẩm, sửa trước · 🟠 nhìn thấy nhiều, sửa sớm · 🟡 đánh bóng.

Đây là mức **mỹ thuật/câu chuyện**, không phải blocker theo 5 loại của charter. Các lỗi chức
năng thật gặp dọc đường nằm riêng ở mục 4.

---

## 2. Cái đang tốt (chuẩn tham chiếu để kéo phần còn lại lên)

Không phải mọi thứ đều trơn. Những chỗ dưới đây chứng minh đội **làm được** chất Rủ Đi; vấn
đề là chúng không lan ra.

| Chỗ | Vì sao tốt | Ảnh |
|---|---|---|
| **Welcome** | Bìa vải indigo có vân, wordmark rất to, dải washi cam nghiêng mang câu định vị, đường mực nối 4 chặng, CTA con dấu «Rủ Đi thôi!» có mép mực gãy | `00-welcome` |
| **Login / OTP** | `CoverBand` indigo ôm đầu trang, nút «Gửi mã» là con dấu nghiêng | `23-…` |
| **8 sticker Nếp** trong khay chat | Mỗi sticker là một hành động khác nhau («Đi thôi!», «Kẹt xe», «Trả tiền nè»), nét ký hoạ có cá tính, đây là chất Nếp đúng nghĩa | `18-…` |
| **Cảnh rỗng** (album, tường cá nhân, tìm không ra) | Nếp ký hoạ + tờ giấy, mỗi cảnh một tư thế | `17-…`, `20-…` |
| **Chi tiết địa điểm** | Ký hoạ quán (mái hiên, bàn, nồi lẩu), chip gu, khối thông tin gọn | `13-…` |
| **Tờ hoá đơn ở bước 2 chia bill** | Đầu tờ «HOÁ ĐƠN», đường chấm dẫn từ tên món tới tiền, vạch đứt, tổng ở đáy: đúng hướng «vật thể thay form» | `07-…` |
| **Stepper 5 bước teal**, **con dấu trạng thái** «CÒN 23 NGÀY», «ĐÃ TỚI», «CHƯA CHUYỂN» | Ngôn ngữ con dấu nhất quán | nhiều |
| **Hệ màu ba tông mang nghĩa** (cam lời rủ, teal tiền, tím AI) | Được giữ đúng trên màn tiền | `07`, `08`, `09` |

---

## 3. Bằng chứng cho từng nguyên nhân gốc

### G1. Hai sản phẩm: demo đẹp, live trơn

So cùng route, cùng dữ liệu «Đà Lạt cuối tuần» (`02-bon-tab-ban-demo` vs `01-bon-tab-live`,
`03-ban-demo-plan-bill-tai-chinh`):

| Màn | Bản demo (fixture) | Bản thật (live) |
|---|---|---|
| Khám phá | Hàng dẫn có ký hoạ lớn, lý do gu tím «View đẹp», ảnh quán thật | 8 hàng icon glyph 40dp trong ô vuông, không ảnh, không thẻ dẫn |
| Lên plan | Ảnh đồi Đà Lạt full-bleed có tên chuyến trắng, hàng avatar, dấu «CÒN 23 NGÀY», lịch trình đường mực | Một thẻ hồng nhạt: «17 tháng 10 · Đà Lạt cuối tuần · 2.500.000đ» |
| Chia hoá đơn bước 1 | «Giấy mẫu Tiệm Nướng Xóm Lèo»: **ảnh chụp hoá đơn giấy thật** trên mặt bàn gỗ | Hai nút: «Chọn ảnh bill», «Nhập tay». 60% màn trống |
| Tài chính | Số teal lớn, «Ngân sách vui chơi», «Giao dịch gần đây» có icon | Ba dòng số nhỏ, khoảng trống lớn |
| Tờ giấy hai người | Tờ có hai chặng, dấu «KÝ ỨC», dòng «Giữ lại: Hàng chè đầu hẻm», nút «Giữ một tấm ảnh» | «Chưa có sổ hai người» + một nút |

Tài liệu thiết kế nói thẳng điều này: `DESIGN.md` (đợt «bản sắc», 2026-09-08) ghi finish
reviewer trả `ship` cho **màn fixture ở sáng 1.0**, và «**chưa phủ: các cửa live**». Mọi vòng
«ship» mỹ thuật đều đứng trên thế giới demo có ảnh do người viết tay. Thế giới thật chưa từng
qua một vòng review mỹ thuật nào.

### G2. Một khuôn màn cho mọi việc

Khuôn chung (đo trong mã và thấy trên ảnh):

```
[‹]        Tiêu đề TopBar          [⋯]
Tiêu đề H1 lặp lại ý TopBar
Hai dòng body xám giải thích luật
Nhãn
[ ô nhập 52dp, viền 1dp xám ]
Nhãn
[ ô nhập ]
…
[████████ nút cam chữ nhật full-width ████████]
```

- 22/41 file màn dùng đúng cặp `TopBar` + `Heading`.
- `RudiButton`: 218 lần trên 60 file. `StampButton`: 2 lần (Welcome cỡ `lon`, Login cỡ
  `vua`). Cỡ `vua` (52dp, token `stamp-button-form` trong `DESIGN.md`, «the same seal on a
  form» theo chính comment của `StampButton.tsx`) chỉ dùng ở **một** form: Login. Không màn
  tạo mới nào sau đăng nhập dùng nó.
- `Field` (ô nhập trắng viền xám): 63 lần. Không có biến thể «viết lên giấy» (gạch chân,
  chữ tay, ô trong thiệp).
- Kết quả: Nhóm mới, Mời vào nhóm, Thêm bạn, Kèo mới, Chỉnh hồ sơ, Đăng bài, Cài đặt nhóm,
  Sửa bản phác, Chặng mới, Tạo bình chọn, Tờ hẹn **có cùng một hình dạng**. Người dùng không
  phân biệt được «mình đang rủ bạn đi chơi» với «mình đang đổi tên nhóm».

### G3. Kho vẽ bị nhốt vào trạng thái rỗng

Kho đã có (đếm trong `src/rudi/art/*.ts`):

- `nep.ts` (708 dòng): **24 tư thế** (`moi`, `keo-ghe`, `giu-cho`, `gop-y`, `cam-ban-do`,
  `giu-khung`, `doi`, `ghi-lai`, `vui`, `buoc-di`, `nang-to`, `moi-ly`, `dat-tay`, `ngoi-xe`,
  `dua-hai-tay`, `nhay`, `voi-len`, `ghe-nhin`, `goi-loi`, `dua-giay`, `up-xuong`, `gap-lai`…),
  7 biểu cảm, 5 dáng, 2 bản (trắng / mảnh).
- `canh.ts`: 10 cảnh rỗng. `gu.ts`: 8 hình gu. `ky-hoa.ts`: ký hoạ quán. `motif.ts`: 3 motif.
- `ui/`: `Washi`, `Stamp`, `StampButton`, `ToGiay`, `KhungAnh` (khung in), `CoverBand`,
  `RouteLine`, `Grain`, `Sticker`.

Chỗ dùng ngoài trang thử `ui-lab`:

| Thành phần | Số file màn dùng | Ở đâu |
|---|---|---|
| `Washi` | 1 | Welcome |
| `StampButton` | 2 | Welcome, Login |
| `CoverBand` | 2 | Login, OTP |
| `ToGiay` (tờ thư gấp ba) | 1 | `ToLoiRu` (chỉ khi đã có tờ) |
| `Motif` | 1 | `HangToGiay` |
| `RouteLine` | 2 | Welcome, PlanLive |
| `Nep` thành hình | 4 | dock, bảng Nếp, chat rỗng, sổ đôi (2 trạng thái) |

Và 7 màn **không import một thành phần vẽ, con dấu hay ảnh nào** (đo theo import:
`ui/art/*`, `EmptyState`, `ErrorState`, `Washi`, `ToGiay`, `StampButton`, `KhungAnh`,
`MediaSlot`, `CoverBand`, `RouteLine`, `Stamp`, sticker, `expo-image`): `LoiMoi`,
`cai-dat/CaiDatScreen`, `cai-dat/XoaTaiKhoanScreen`, `friends/AddFriend`,
`keo/CreateOutingLive`, `groups/Invite`, `groups/New`. Ba trong số đó (Kèo mới, Nhóm mới,
Mời) là **ba cửa tạo mới quan trọng nhất** của vòng «rủ nhau».

Ở nhiều màn còn lại, «có art» nghĩa là một con dấu trạng thái 26dp hoặc một cảnh vẽ trong
`EmptyState`. Trên ảnh chụp, Nếp và cảnh vẽ hiện ở: Tường cá nhân rỗng, Album rỗng, Tìm không
ra, Hành trình chưa có đường. Nghĩa là **hình biến mất đúng lúc người dùng bắt đầu dùng
thật**.

Trường hợp rõ nhất: sổ hai người. `KhongGianGiay.tsx:134-143`, trạng thái «Chưa có sổ hai
người» **không truyền `illustration`**. `:176`, «Chưa có tờ nào tuần này» chỉ có Nếp khi
`coBuoiNao` (đã từng có buổi chốt). Nghĩa là **lần đầu** gặp tính năng chữ ký của sản phẩm là
lần duy nhất nó không có hình (`04-khay-tao-va-so-hai-nguoi`, `05-lap-so-va-ban-phac`).

### G4. Nếp chỉ còn là một mép giấy

- Trên **mọi** ảnh live, Nếp là một dải giấy ~10px ở mép phải giữa màn. Người mới không biết
  đó là gì.
- Mở ra (`18-nep-khay-chat-sticker`, ảnh 1–2): tờ giấy nhỏ có Nếp 48dp, rồi một sheet chat
  «Nếp · Trợ lý riêng của bạn · Mình đang thấy: Bạn đang ở Khám phá» + chip câu hỏi + ô nhập.
  Tức là Nếp = một chatbot trong bottom sheet, không phải một nhân vật sống trong sổ.
- Nếp **không** có mặt ở: ghi sổ xong, lập sổ hai người, đồng ý lập sổ, bản phác Nếp viết
  («Nếp phác sẵn» nhưng tờ chỉ có «18:30 · Ăn tối», không có dấu vết Nếp đã phác), chốt kèo,
  tiền đã về, huy hiệu mở, kỷ niệm mới.
- QA 23/09 (`docs/claude/2026-09-23/qa-cap-doi-minh-linh.md`) đã chấm «Câu chuyện & cảm xúc
  **3/10**» và «Insight **1/10**» với lý do «Nếp phác "18:30 · Ăn tối"». Lượt này thấy nguyên
  trạng.

### G5. Chữ gánh hết câu chuyện

Đếm trên ảnh, những câu người dùng phải đọc **trước** khi làm được việc:

- Lập sổ hai người: 8 dòng («Cho phép» 3 gạch, «Không kéo theo» 3 gạch) + 2 nút.
- Đăng bài: tiêu đề «Ai đọc được?» + đoạn 3 dòng + 4 lựa chọn mỗi cái 2 dòng mô tả.
- Đợt thu: «0/7 người đã xong phần mình. Đã phát: mỗi người xem phần của mình qua link riêng.»
  + «Người được nhận tiền là người bấm «Tiền đã về»; số ở trên đếm từ biên nhận, không phải ai
  tự khai.»
- Tài chính: «Người khác đang nợ bạn 1.120.000đ. Số này đọc từ sổ cái, không phải số dư ngân
  hàng.»
- Thành viên: đoạn 4 dòng ở đáy về quyền quản trị.

Mỗi câu đều **đúng** (nhiều câu là hàng rào pháp lý/quyền riêng tư có lý do). Vấn đề là chúng
chiếm vị trí của câu chuyện: cùng cỡ, cùng màu xám, ngay dưới tiêu đề. Cuốn nhật ký biến thành
tờ điều khoản.

### G6. Không có khoảnh khắc

- **Chia bill 5/5** (`08-…`, ảnh 4): dấu «ĐÃ GHI SỔ» 26dp ở góc trái, tiêu đề «Đã ghi: Hóa đơn
  của nhóm», một câu, hai nút; 55% màn dưới trống. Đây là giây phút người ứng tiền được «giải
  thoát»; màn đối xử với nó như một thông báo lưu thành công.
- **Đã đề nghị lập sổ** (`05-…`): câu xám «Đã đề nghị. Chờ Minh Anh đồng ý trên máy của họ;
  im lặng không phải đồng ý.» Một hành động có tính rủ rê, tình cảm, được xác nhận bằng một câu
  pháp lý.
- **Đồng ý lập sổ**: sheet đóng, màn đổi sang «Chưa có tờ nào tuần này». Không dấu, không mở
  sổ, không Nếp.
- `Stamp.tsx` đã có cú đóng dấu ba nhịp trong ngân sách `celebrate` 550ms. Nó đang được dùng
  cho con dấu nhỏ; chưa màn nào dùng nó làm **trung tâm** của một khoảnh khắc.

### G7. Danh sách đồng dạng, người không có mặt

- Thành viên (`10-…`, ảnh 2): 8 hàng, avatar chữ cái cùng viền cam nhạt, 7 nút cam «Đặt làm
  quản trị» giống hệt nhau chạy dọc màn. Nút lặp lấn át tên người.
- Bạn bè (`14-…`, ảnh 2): 7 hàng, 7 nút «Nhắn tin» giống hệt.
- «Rủ ai?» (`14-…`, ảnh 4): 7 bạn, **cùng một icon người outline**, cùng phụ đề «Mở tờ giấy
  của hai bạn». Đây là màn chọn *người yêu/người bạn thân* để rủ đi chơi.
- Kết quả chia (`08-…`, ảnh 2): 8 hàng «Tên ………… 97.500đ» giống hệt.
- Đợt thu (`09-…`, ảnh 3): 7 khối «X → Minh Anh · CHƯA CHUYỂN · 160.000đ · [Tiền đã về từ X]».
- Hai «Thu» cùng chữ «T», hai «Minh…» cùng chữ «M»: avatar chữ cái không phân biệt được người.

---

## 4. Lỗi chức năng và hiển thị gặp dọc đường

Không thuộc phạm vi mỹ thuật nhưng thấy tận mắt, ghi lại để không mất.

| # | Mức | Phát hiện | Tái lập | Tiêu chí gỡ |
|---|---|---|---|---|
| B1 | 🔴 | **Ngõ cụt ở sổ hai người.** Khi A mở «Rủ một người đi chơi» → chọn B lúc **chưa có sổ**, route `?ru=1` tự xin bản phác: máy chủ tạo `pair_papers` trạng thái `nhap`, `cycle_id` NULL, chủ là A. Sau khi sổ lập xong, B thấy «Tuần này bạn mở lời. Nếp phác sẵn…», bấm «Rủ đi chơi» → **«Tờ giấy không ở trạng thái làm được việc này.»**, màn không đổi. Bản phác của A chỉ A thấy (đúng luật riêng tư), nên B kẹt tới khi A gửi/bỏ hoặc tới `expires_at` (Chủ nhật). | Stack seed; Tuấn Kiệt: khay «+» → «Rủ một người đi chơi» → Minh Anh → «Đề nghị lập sổ»; Minh Anh: «Xem lời đề nghị» → «Đồng ý» → «Rủ đi chơi». DB: `select state,cycle_id,draft_owner_id from pair_papers` → `nhap · NULL · <Tuấn Kiệt>` | B không bao giờ được mời «mở lời» khi tuần đã có bản phác của A; hoặc bản phác trước khi có sổ không được tạo/không chặn |
| B2 | 🟠 | **Thanh «Cùng chọn» ghim đè lên bong bóng tin nhắn** trong chat nhóm: phần trên của tin «Ngân sách mỗi người khoảng 2 triệu rưỡi, ổn không?» lộ ra bị cắt dưới thanh | Mở chat Team Đà Lạt, cuộn ở giữa | Nội dung chat bắt đầu dưới thanh ghim, hoặc thanh có nền che trọn |
| B3 | 🟠 | **Hồ sơ người cùng nhóm (chưa là bạn)**: «Nhắn tin» xám, câu «Kết bạn để nhắn riêng.», nhưng không có nút kết bạn nào; «Thêm hành động» chỉ có «Chặn», «Báo cáo». Người dùng phải biết số điện thoại để thêm bạn | Hà Vy mở hồ sơ Tuấn Kiệt | Có «Kết bạn» ngay trên hồ sơ người cùng nhóm |
| B4 | 🟡 | **Album chuyến đi của nhóm nói «Chưa có kèo nào»** trong khi nhóm có kèo 17–19/10 (album chỉ liệt kê chuyến đã bắt đầu: `GroupRecap(ctx, today)`) | Mở Tường nhóm → Album | Câu phân biệt «chưa có kèo» với «kèo chưa tới ngày» |
| B5 | 🟡 | **Route fixture hiện dữ liệu demo trên phiên thật** khi vào thẳng: `/votes/[id]` («BBQ tối thứ Bảy ở đâu?», «Bản trải nghiệm chỉ ghi phiếu của bạn»), `/check-ins/new` (Lan Anh, Minh Khoa…), `/trips/[id]/itinerary`, `/ai-match` («Không phải kết quả LLM»). Trong app chỉ màn fixture dẫn tới đó; deep link/thông báo thì có thể | Đăng nhập thật, mở các URL trên | Các route này hoặc đọc live, hoặc chuyển hướng khi có phiên |
| B6 | 🟡 | «Hỏi Rủ Đi AI» ở Khám phá điền câu mẫu «quán nướng cho 6 người, 200k mỗi người» và trả **0 kết quả** + cảnh «Chưa thấy nơi phù hợp». Lần chạm đầu vào AI kết thúc bằng thất bại | Stack không khoá AI; **cần đo lại trên máy có khoá** | Câu mẫu luôn ra kết quả, hoặc khi AI tắt thì nút không giả vờ hỏi |
| B7 | 🟡 *[web]* | Ô đang nhập có **hai viền** (viền accent 2dp + outline vàng của trình duyệt); nhóm 6 ô OTP bị một khung vàng bao | Mọi `Field` trên web | `outlineStyle: none` trên web cho `TextInput` đã có viền focus riêng |
| B8 | 🟡 *[web, dev]* | Cảnh báo React «`<button>` cannot contain a nested `<button>`» trên nhiều màn (Pressable lồng Pressable). Trên web đây là HTML sai và hỏng trình đọc màn hình | Mở Bạn bè, Cài đặt… trên web dev | Không lồng vai `button` |

---

## 5. Rà từng màn theo phase của vòng lặp sản phẩm

Thứ tự đi theo vòng «mở app → khám phá → rủ → lên plan → đi → chia tiền → kỷ niệm», rồi sổ
hai người và Cá nhân. Mỗi màn: *thấy gì* → *vì sao thiếu sức sống* → *đề xuất dùng tài sản
đã có*.

### 5.0 Phase vào cửa

| Màn | Sức sống | Câu chuyện | Mức | Ghi chú |
|---|---|---|---|---|
| Welcome | 5 | 4 | — | Chuẩn tham chiếu. Chỉ tiếc slide 1 minh hoạ bằng icon trong vòng tròn; có thể đổi thành 4 ký hoạ Nếp nhỏ ở 4 chặng |
| Login | 4 | 3 | 🟡 | Bìa + con dấu tốt; nửa dưới quay về khuôn form trắng; nút «Vào bản trải nghiệm» hồng nhạt chỉ dev |
| OTP | 3 | 2 | 🟡 | Sáu ô trắng; có thể là sáu ô tem/sáu lỗ đục trên vé |
| Sở thích (Personalization) | 2 | 2 | 🟠 | Lưới 8 thẻ, mỗi thẻ **hình gu cỡ icon + chữ + nút radio tròn**. Hình gu (`GuGlyph`) đã được dùng nhưng co lại bằng icon, đứng cạnh một radio như form khảo sát; nên phóng to thành thân thẻ. Chọn = **dán sticker** lên thẻ, không phải tick radio. Ba mức chi = ba **phong bì** dày mỏng |

### 5.1 Phase Khám phá

**Khám phá (tab 1, live)** · Sức sống 2 · Câu chuyện 2 · 🔴
- Thấy: ô tìm, 4 chip, «8 nơi ở Đà Lạt», rồi danh sách hàng: ô icon 40dp (tay cầm game,
  cốc cafe) + tên + 3 dòng meta xám + tim. Hai hàng đầu xếp đôi nhưng vẫn chỉ là icon.
- Vì sao: ảnh địa điểm (`place_photos`) và ký hoạ quán (`KyHoa`) có, nhưng hàng live dùng
  glyph. Không có thẻ dẫn. Không có «vì sao hợp hội mình» (lý do gu tím chỉ có ở demo).
- Đề xuất: (1) hàng đầu là **thẻ dẫn** dạng bản in dán trang (`KhungAnh` + dòng xuất xứ) với
  ảnh hoặc ký hoạ lớn; (2) mọi hàng có thumbnail ảnh hoặc ký hoạ, không glyph; (3) dòng lý do
  tím ngắn «hợp gu 5/8 người»; (4) meta gom thành một dòng, phần còn lại vào chi tiết.

**Hỏi Rủ Đi AI** · 🟠 (xem B6). Kết quả rỗng dùng cảnh Nếp cầm bản đồ, tốt; nhưng không được
để lần đầu hỏi AI ra rỗng.

**Đổi điểm đến («Đi đâu?»)** · Sức sống 1 · Câu chuyện 2 · 🟠
- Thấy: 15 thành phố là 15 hàng chữ: tên, tỉnh, một câu mô tả, mũi tên.
- Đề xuất: mỗi thành phố một **tem bưu chính/bưu thiếp** ký hoạ (đồi thông, Chùa Cầu, cầu
  Rồng…), lưới 2 cột. Thành phố đang chọn có dấu bưu điện «ĐANG Ở».

**Chi tiết địa điểm** · Sức sống 3 · Câu chuyện 3 · 🟡
- Tốt: ký hoạ đầu trang. Thiếu: ảnh thật khi có; nút «Rủ hội tới đây» nên là con dấu cam
  (đây là lời rủ, đúng nghĩa tông cam).

**Match gu cả nhóm** · (route fixture, xem B5).

### 5.2 Phase Rủ nhau: nhóm, mời, chat

**Tin nhắn (tab 3)** · Sức sống 2 · Câu chuyện 2 · 🟠
- Thấy: vòng tròn đứt «Đăng story», rồi danh sách hội: icon người trong ô hồng, tên, «8 thành
  viên · bạn quản trị», tin cuối, badge đỏ.
- Đề xuất: mỗi hội là **gáy sổ** (màu bìa riêng do hội chọn, xem Cài đặt nhóm đã có «Màu
  bong bóng»), hoặc cụm avatar chồng nhau; nhắn riêng là **phong bì**.

**Nhóm mới** · Sức sống 1 · Câu chuyện 1 · 🔴 (một trong 7 màn không có nét vẽ nào)
- Thấy: «Nhóm mới» / «Đặt tên cho hội» / 2 dòng xám / câu gợi ý «Chỉ hai người?…» / ô «Tên
  nhóm» / nút «Mở nhóm». Dưới là 55% trống.
- Vì sao: tạo hội là **mở một cuốn sổ mới**, khoảnh khắc đầu của cả câu chuyện, mà màn không
  có bìa, không có Nếp, không có màu.
- Đề xuất: đầu màn là `CoverBand` indigo có vân vải; ô tên nhóm là **nhãn dán trên bìa**
  (chữ Bricolage to, gạch chân mực); chọn màu bìa (5 màu đã có ở «Màu bong bóng») ngay
  đây; Nếp `keo-ghe` kéo ghế mời người vào. Nút «Mở nhóm» thành `StampButton` cỡ form. Sau khi
  tạo: bìa **mở ra** (chữ ký chuyển động của Welcome) vào trang nhóm.

**Mời vào nhóm** · Sức sống 1 · Câu chuyện 1 · 🔴
- Thấy: «Mời bằng số điện thoại» / 2 dòng / ô số / ô tên / nút «Gửi lời mời» / 50% trống.
- Đề xuất: bố cục **thiệp mời**: tên hội ở đầu thiệp, hàng ghế trống có avatar đứt nét cho
  người sắp tới, ô số điện thoại nằm trên phong bì. Sau khi gửi: phong bì dán tem bay đi.

**Thành viên** · Sức sống 2 · Câu chuyện 2 · 🟠
- Thấy: xem G7. 7 nút «Đặt làm quản trị» lặp.
- Đề xuất: đưa «Đặt làm quản trị» vào menu dài hoặc một nút sửa vai ở đầu danh sách; vai là
  con dấu («QUẢN TRỊ», «MỚI VÀO»); avatar có danh tính (mục 7.4); đoạn luật 4 dòng ở đáy thu
  thành một dòng «i».

**Cài đặt nhóm (sheet)** · Sức sống 2 · Câu chuyện 2 · 🟡
- Tốt: 5 ô màu «Aa» có tính chơi. Thiếu: xem trước bong bóng thật với màu đã chọn; «Rời
  nhóm» đỏ nằm ngay dưới, nên tách xa.

**Chat nhóm** · Sức sống 3 · Câu chuyện 3 · 🟠
- Tốt: bong bóng có màu hội, thẻ AI tím, sticker Nếp.
- Thiếu: thẻ bình chọn là **danh sách radio** «Bánh mì / 0 phiếu» như form khảo sát; đề
  xuất thành các **mảnh giấy nhớ** mỗi lựa chọn, phiếu là chấm mực/dấu tay, người đã bầu hiện
  avatar nhỏ. Dòng «Chưa mã hoá đầu cuối» (đúng luật) nên là biểu tượng khoá nhỏ trong header
  thay vì một dòng riêng. Xem B2.

**Khay «+» trong chat** (Ảnh / Sticker / Bình chọn / Tờ hẹn) · Sức sống 2 · 🟡
- Bốn icon outline trong ô vuông. Đề xuất: bốn **đồ vật** ký hoạ nhỏ (máy ảnh, Nếp, giấy
  nhớ, tờ thư).

**Tạo bình chọn (panel)** · Sức sống 1 · Câu chuyện 1 · 🟠
- Thấy: «Hội mình chọn gì?» + 3 ô trắng «Câu hỏi», «Lựa chọn 1», «Lựa chọn 2» + nút.
- Đề xuất: soạn **ngay trên tấm bình chọn** sẽ gửi (WYSIWYG): ghi câu hỏi lên đầu tấm, mỗi lựa
  chọn là một mảnh giấy nhớ thêm vào bằng «+». Không có «ô Lựa chọn 1».

**Tờ hẹn (panel)** · Sức sống 2 · Câu chuyện 2 · 🟠
- Tên «Tờ hẹn» nhưng hình là một ô textarea + hộp tím «Mình đang thấy». Đề xuất: panel là
  **một tờ giấy có chặng kẻ sẵn** (giờ · chỗ), AI phác bằng **nét chì đứt** (đúng ngôn ngữ
  «nháp là nét chì» trong DESIGN.md) rồi người dùng tô mực.

**Bạn bè / Thêm bạn / Hồ sơ người khác** · Sức sống 1–2 · 🟠
- Xem G7 và B3. «Thêm bạn» là màn không có nét vẽ nào: một ô số + nút «Tìm» + 60% trống.
  Đề xuất: danh thiếp/QR của chính mình ở trên (đưa máy cho bạn quét), ô số ở dưới.
- Hồ sơ người khác: «Tường cá nhân» rỗng dùng cảnh Nếp treo đèn lồng, tốt. Đầu hồ sơ nên có
  «những lần đi cùng nhau» (số chuyến chung, ảnh chung) thay vì chỉ ngày tham gia.

### 5.3 Phase Lên plan

**Lên plan (tab 2, live)** · Sức sống 2 · Câu chuyện 2 · 🔴
- Thấy: một thẻ hồng nhạt (ngày to, dấu «CÒN 23 NGÀY», tên kèo, dòng meta). Bản demo cùng
  route có ảnh bìa chuyến full-bleed, avatar, lịch trình đường mực ngay trên tab.
- Đề xuất: mỗi kèo là **tấm vé/thiệp** có ảnh bìa (ảnh địa điểm của chặng đầu, hoặc ký hoạ
  thành phố), hàng avatar người đi, đường mực mini các chặng. Kèo đã qua xếp thành **chồng
  vé đã xé**.

**Kèo mới** · Sức sống 1 · Câu chuyện 2 · 🔴 (không có nét vẽ nào)
- Thấy: «Hội mình đi đâu?» + ô «Tên kèo» + hai ô ngày «24/09/2026» + dòng xám định dạng
  ngày + ô «Số người» + ô «Ngân sách» + 4 chip + nút «Tạo kèo». Bản xem trước chỉ hiện sau khi
  gõ tên, là một khung viền đứt.
- Vì sao: comment trong mã (`CreateOutingLive.tsx`) nói «An invitation, not a form to file»,
  nhưng thứ render ra vẫn là form. Ngày gõ tay dạng `dd/mm/yyyy` với câu giải thích định dạng
  là dấu hiệu rõ nhất của «tờ điền chữ».
- Đề xuất: đảo ngược: **tấm thiệp là màn hình**, ô nhập nằm trong thiệp. Tên kèo viết lên dải
  washi; ngày là **hai tờ lịch xé** (chạm để chọn, không gõ); số người là hàng ghế/avatar của
  hội (bỏ bớt ai không đi); ngân sách là phong bì với 4 mức. Nút «Tạo kèo» = `StampButton`
  cỡ form, bấm là dấu «ĐÃ RỦ» dập lên thiệp rồi thiệp trượt vào tab Lên plan.

**Chi tiết kèo** · Sức sống 3 · Câu chuyện 3 · 🟠
- Tốt: tab Lịch trình/Hành trình, đường mực các chặng, dấu «Đã tới» teal, tay kéo sắp xếp.
- Thiếu: không có ảnh bìa; hai số «2.500.000đ / 20.000.000đ» đứng trơ; «Chia bill buổi này»
  là nút viền teal giống nút thường. Chặng có thể có thumbnail quán.

**Chặng mới (sheet)** · Sức sống 1 · 🟠
- Hai ô «Giờ · Chặng» + dãy chip tên quán. Đề xuất: chọn quán bằng **thẻ ký hoạ nhỏ** có ảnh;
  giờ là mặt đồng hồ/cuộn giờ.

**Hành trình** · (bản đồ không tải trong sandbox). Cảnh «Mở một trang đường mới» có Nếp, tốt.

**Check-in** · (route fixture, xem B5).

### 5.4 Phase Chia tiền 🔴 (phần người dùng chỉ đích danh)

**Bước 1/5 «Bill hôm nay»** · Sức sống 1 · Câu chuyện 1 · 🔴
- Thấy: stepper, «Bill hôm nay», 3 dòng xám, nút teal «Chọn ảnh bill», nút viền «Nhập tay».
  **62% màn trống**.
- So: bản demo cùng bước có ảnh hoá đơn giấy trên bàn gỗ.
- Đề xuất: giữa màn là **một tờ hoá đơn trắng ký hoạ, cong nhẹ, mép răng cưa**, Nếp pose
  `giu-khung` cầm máy ảnh ngắm vào tờ; chạm tờ = chụp/chọn ảnh; «Nhập tay» là dòng bút chì dưới
  tờ («hoặc tự viết»). Nếu đi từ một kèo, đầu tờ đã in tên quán và ngày.

**Bước 2/5 «Xem lại hoá đơn»** · Sức sống 3 · Câu chuyện 3 · 🟠
- Tốt: tờ HOÁ ĐƠN có đầu tờ, chấm dẫn, tổng (đã sửa sau QA 23/09).
- Còn: dòng đang mở vẫn bung **ba ô form** («Món», «Số phần», «Thành tiền») bên trong tờ; câu
  cảnh báo đỏ «Một món chưa có tên…» chạy trên đầu. Đề xuất: sửa **tại chỗ trên dòng in**
  (chạm tên để gõ, chạm số để gõ), không bung form; phông chữ dạng in nhiệt cho dòng món.

**Bước 3/5 «Ai dùng món nào?»** · Sức sống 2 · Câu chuyện 2 · 🔴
- Thấy: mỗi món lặp **tên 8 người hai lần**: một lần trong phụ đề «8 người · Minh Anh, Tuấn
  Kiệt, Bảo Châu…», một lần thành lưới 8 ô teal có dấu tick. 3 món = 24 ô teal + 3 hàng chip.
  Một bức tường teal.
- Đề xuất: đảo trục: **hàng avatar ở trên cùng** (8 gương mặt), mỗi món là một dòng hoá đơn,
  gán bằng cách **kéo avatar thả vào món** hoặc chạm món rồi chạm mặt; món ghi bằng **chấm
  avatar nhỏ** bên phải (không lặp tên). «Cả nhóm» là trạng thái mặc định hiện bằng một dấu
  «CHIA ĐỀU».

**Bước 4/5 «Kết quả»** · Sức sống 2 · Câu chuyện 2 · 🔴
- Thấy: «Phần của bạn 97.500đ», rồi 8 hàng «Tên ………… 97.500đ», rồi **sau kết quả** mới hỏi «Ai
  đã trả bill?» (8 chip) và ô «Gọi khoản này là».
- Vì sao: câu hỏi về người trả và tên khoản đứng sau con số, nên màn là bảng tính rồi form.
- Đề xuất: mỗi người là **một cuống phiếu xé từ hoá đơn** (avatar, tên, số teal lớn); người đã
  trả có dấu «ĐÃ ỨNG»; phiếu của mình nổi lên trên cùng. «Ai đã trả» hỏi ngay ở bước 1 hoặc
  2 (đã biết từ lúc chụp), tên khoản tự điền từ tên quán/kèo.

**Bước 5/5 «Đã ghi sổ»** · Sức sống 1 · Câu chuyện 1 · 🔴
- Xem G6. Đề xuất: **khoảnh khắc**: tờ hoá đơn gập đôi trượt vào trang sổ, con dấu «ĐÃ GHI
  SỔ» cỡ lớn dập giữa màn (dùng đúng cú đóng dấu 550ms đã có), Nếp pose `vui`; dưới là 8 cuống
  phiếu xoè ra, và **một** nút tiếp «Gửi phần cho từng người».

**Đợt thu** · Sức sống 2 · Câu chuyện 2 · 🔴
- Thấy: «0/7 lượt chuyển đã về», 2 dòng xám, «Ai chuyển cho ai», 2 dòng xám, 7 khối giống hệt,
  mỗi khối có nút teal nhạt «Tiền đã về từ X».
- Đề xuất: **trang sổ thu tiền**: mỗi người một dòng kẻ, cột phải là ô con dấu; bấm «Tiền đã
  về» là **dấu «ĐÃ VỀ» dập lên dòng** (một lần mỗi sự kiện, đúng luật celebrate); tiến độ 0/7
  là dải washi đầy dần. Hai đoạn luật thành chú thích lề.

**Quyết toán** · Sức sống 2 · Câu chuyện 2 · 🟠
- Thấy: 7 hàng «X → Minh Anh · Đề xuất, chưa phải nghĩa vụ · 160.000đ». Khối đầu «Chi tiêu
  theo chuyến» có cột phải «Chưa có chuyến» bị ép hẹp.
- Đề xuất: sơ đồ **mũi tên mực** từ người tới người (ít mũi tên nhất là điểm tự hào của thuật
  toán, nên cho thấy), số trên mũi tên.

**Tài chính của tôi** · Sức sống 1 · Câu chuyện 1 · 🟠
- Thấy: «Phần chi của bạn 160.000đ», «Còn phải trả 0đ», «Sẽ nhận 1.120.000đ», «Chi theo
  nhóm», câu luật. 50% trống. Bản demo cùng route có «Giao dịch gần đây» với icon.
- Đề xuất: đầu trang là **một trang sổ cái** (số lớn Bricolage, teal chỉ trên số), rồi dòng
  thời gian các khoản theo buổi đi (ảnh quán nhỏ + tên buổi), không chỉ theo nhóm.

### 5.5 Phase Kỷ niệm

**Tường nhóm** · Sức sống 3 · Câu chuyện 3 · 🟡
- Tốt: ảnh trong khung in có dòng xuất xứ «Hà Vy · 16:40 24-09». (Ảnh sọc là PNG mẫu của
  seed, không phải lỗi.)
- Thiếu: mọi bài cùng khung, cùng thẳng; «Thích / Bình luận / 0 tim · 0 bình luận» là thanh
  mạng xã hội chung. Đề xuất: ảnh **nghiêng ±2°** như dán tay, vài miếng washi giữ góc, tim là
  dấu tay mực.

**Thả khoảnh khắc** · Sức sống 1 · Câu chuyện 2 · 🔴
- Thấy: khung hồng lớn có icon máy ảnh + «Chưa có ảnh. Chọn một tấm từ thư viện.», nút «Chọn
  ảnh», ô «Một câu cho khoảnh khắc (300 ký tự còn lại)», dòng luật EXIF, nút đáy.
- Đề xuất: ô ảnh là **khung instax trống** (`KhungAnh`) với dòng xuất xứ đã điền sẵn tên nhóm
  + ngày, câu chú thích viết **ngay trên lề trắng dưới ảnh** như viết bút lên instax. Bấm
  «Thả» = ảnh bay vào tường.

**Đăng bài** · Sức sống 1 · Câu chuyện 1 · 🟠
- Thấy: textarea + «Chọn ảnh» + «Ai đọc được?» 3 dòng + 4 radio 2 dòng mỗi cái + nút.
- Đề xuất: 4 mức quyền là **4 phong bì** mở ít tới nhiều (dán kín / hé / mở / tờ rơi), chọn là
  thả bài vào phong bì. Mô tả dài vào «i».

**Đăng story** · Sức sống 1 · Câu chuyện 1 · 🟠
- Thấy: khung đứt nét trống «Chưa có ảnh nào.», nút, ô chú thích, đếm ký tự. Đề xuất: khung
  **polaroid 24 giờ** có đồng hồ cát nhỏ ở góc.

**Album chuyến đi** · 🟡 (xem B4). Cảnh rỗng có Nếp, tốt.

### 5.6 Sổ hai người («Nếp truyền giấy») 🔴

Đây là tính năng chữ ký, và ảnh là bằng chứng rõ nhất cho phản hồi của người dùng.

| Checkpoint | Thấy | Sức sống | Câu chuyện |
|---|---|---|---|
| Khay «+» → «Rủ một người đi chơi» | Hàng thứ 5 trong danh sách icon | 1 | 1 |
| «Rủ ai?» | Danh sách bạn, cùng một icon người, «Mở tờ giấy của hai bạn» | 1 | 2 |
| «Tờ giấy của hai mình», chưa có sổ | Tiêu đề + 2 dòng + nút «Đề nghị lập sổ». **Không có tờ giấy, không có Nếp** | 1 | 1 |
| Sheet «Lập sổ hai người» | 8 dòng điều khoản («Cho phép», «Không kéo theo») + 2 nút | 1 | 1 |
| Đã đề nghị | Câu xám «…im lặng không phải đồng ý.» | 1 | 1 |
| Người kia: «Xem lời đề nghị» → «Đồng ý» | Sheet đóng; không dấu, không mở sổ | 1 | 1 |
| «Chưa có tờ nào tuần này» | Tiêu đề + 1 câu + nút. Không hình | 1 | 1 |
| Bản phác của Nếp | Một bảng 2 hàng («18:30 · Ăn tối», «Thứ Bảy 26/09 · BẢN PHÁC») góc gấp cam; 4 nút | 2 | 2 |
| Sheet «Sửa bản phác» | Chip 11 ngày, ô Giờ, ô Chỗ chính, «Chọn chỗ», ô Giờ, ô Đi tiếp, textarea «Vì sao đổi», nút | 1 | 1 |

Vì sao: tên gọi hứa **tờ thư gấp ba truyền tay**, Nếp bản mảnh với ba tư thế truyền giấy
(`dua-giay`, `up-xuong`, `gap-lai`), motif `thuGapBa`; `DESIGN.md` ghi rõ Phase 1 «Nếp truyền
giấy» chỉ đưa chúng lên `ui-lab`, «không màn người dùng nào đổi». Màn thật dùng lại khuôn form.

Đề xuất theo checkpoint:
1. **Chưa có sổ**: một **cuốn sổ nhỏ đóng**, bìa có hai ô tên (mình · người ấy); Nếp mảnh
   `gap-lai` đứng cạnh. Nút «Đề nghị lập sổ» là con dấu.
2. **Lập sổ**: sheet là **một giao kèo viết tay** hai cột (được / không) chữ nhỏ trên giấy
   kẻ, có **hai chỗ ký**; bấm đề nghị = mình ký, chỗ kia để trống «chờ Minh Anh ký».
3. **Người kia đồng ý**: chữ ký thứ hai hiện, dấu «SỔ ĐÃ MỞ», bìa **mở ra** (tái dùng chuyển
   động mở bìa của Welcome). Đây là khoảnh khắc cảm xúc nhất của tính năng.
4. **Chưa có tờ tuần này**: **tờ thư gấp ba đang gấp**, Nếp `dua-giay` chìa ra; chạm tờ =
   «Rủ đi chơi».
5. **Bản phác**: `ToGiay` gấp ba thật, nét **chì đứt** cho phần Nếp phác, dòng ký nhỏ «Nếp
   phác, bạn sửa»; khi người dùng sửa, nét chì thành mực.
6. **Sửa bản phác**: sửa **trên tờ**, không mở sheet form. Ngày là lịch xé tuần này; chỗ là
   thẻ ký hoạ quán; «Vì sao đổi» là **ghi chú bên lề** viết tay.

### 5.7 Phase Cá nhân và cài đặt

| Màn | Sức sống | Câu chuyện | Mức | Thấy → đề xuất |
|---|---|---|---|---|
| Cá nhân (tab 4) | 2 | 2 | 🟠 | Avatar chữ «A», dòng đếm «7 bạn bè · 1 nhóm · 1 kèo…», 8 hàng menu icon. → Đầu trang là **hộ chiếu/thẻ thành viên** có ảnh, các con tem chuyến đã đi; menu gom xuống dưới |
| Chỉnh hồ sơ (inline) | 1 | 1 | 🟡 | 3 ô + nút cam + «Huỷ». → Sửa trên thẻ hộ chiếu |
| Thành tích | 2 | 1 | 🟠 | Thanh tiến độ, 8 hàng như menu: khoá xám, «Chưa đo được». → Huy hiệu là **tem/sticker Nếp** (kho có sẵn tư thế), chưa mở là tem còn trên giấy bóc, «Chưa đo được» là tem mờ có dấu hỏi |
| Cài đặt, Phiên, Người đã chặn, Về Rủ Đi, Xoá tài khoản | 2 | — | 🟡 | Được phép trơn. Chỉ cần: nhóm mục có tiêu đề nhỏ, «Xoá tài khoản» có cảnh Nếp `gap-lai` để câu vĩnh viễn nặng đúng mức |

### 5.8 Bản tối (`22-ban-toi`)

- Nền vải indigo giữ được, màu ba tông đọc được.
- Mất chất giấy: hàng, ô, thẻ đều là khối xanh đen phẳng; icon ô vuông xanh đậm. Nút «Chọn
  ảnh bill» teal sáng trên nền tối rất gắt so với phần còn lại. QA 23/09 đã ghi «dark mất chất
  giấy», chưa đổi.

---

## 6. Pop-up và sheet: danh mục đầy đủ

| # | Pop-up / sheet | Mở từ | Sức sống | Vấn đề chính | Đề xuất ngắn |
|---|---|---|---|---|---|
| 6.1 | **«Mình làm gì tiếp?»** (khay tạo) | FAB «+» | 2 | 5 hàng icon + tiêu đề + mũi tên, giống menu cài đặt; «Chia hóa đơn» chỉ là một dòng như mọi dòng | **Bàn làm việc**: 5 vật thể ký hoạ xếp trên mặt giấy (tờ lịch = hẹn, hoá đơn = chia, ảnh in = kỷ niệm, polaroid = story, tờ thư gấp = rủ một người); Nếp `moi` giữa; vật thể chính theo ngữ cảnh (đang trong kèo thì hoá đơn to nhất) |
| 6.2 | Bảng Nếp | Chạm mép Nếp | 2 | Chatbot trong sheet; Nếp 48dp ở góc | Nếp to, pose theo màn; câu mở đầu do Nếp nói, không phải «Mình đang thấy: Bạn đang ở Khám phá» |
| 6.3 | Lập sổ hai người | Sổ đôi | 1 | 8 dòng điều khoản | Giao kèo có hai chỗ ký (5.6) |
| 6.4 | Sửa bản phác | Sổ đôi | 1 | 7 ô form | Sửa trên tờ (5.6) |
| 6.5 | Chặng mới | Chi tiết kèo | 1 | 2 ô + chip chữ | Thẻ quán ký hoạ, cuộn giờ |
| 6.6 | Hội mình chọn gì? (bình chọn) | Khay chat | 1 | 3 ô «Câu hỏi / Lựa chọn 1 / 2» | Soạn trên tấm giấy nhớ |
| 6.7 | Phác một tờ hẹn | Khay chat | 2 | textarea + hộp tím | Tờ kẻ chặng, nét chì AI |
| 6.8 | Sticker | Khay chat | 4 | Tốt | Giữ; dùng chính các sticker này làm minh hoạ cho các khoảnh khắc khác |
| 6.9 | Thêm vào cuộc trò chuyện | «+» chat | 2 | 4 icon outline | 4 đồ vật ký hoạ |
| 6.10 | Cài đặt nhóm | «⋯» chat | 2 | Form + 5 ô màu | Xem trước bong bóng; tách «Rời nhóm» |
| 6.11 | Thêm hành động (hồ sơ) | Hồ sơ người khác | 1 | Chỉ Chặn / Báo cáo (B3) | Thêm «Kết bạn», «Rủ đi chơi» |
| 6.12 | Chỉnh hồ sơ | Cá nhân | 1 | Form inline | Sửa trên thẻ |
| 6.13 | Hỏi AI (khám phá) | ✦ | 2 | Ra rỗng (B6) | — |
| 6.14 | Đổi điểm đến | Dòng «Đà Lạt · đổi nơi khác» | 1 | 15 hàng chữ | Tem bưu thiếp (5.1) |

Chung cho mọi sheet: nền trắng tinh, tay cầm xám, «×» góc phải. **Sheet nào cũng giống sheet
nào.** Tối thiểu nên có: nền `paper` có vân (như trang), mép trên răng cưa hoặc gấp như một tờ
rút ra từ sổ, và tiêu đề sheet dùng đầu trang sổ (7.1).

---

## 7. Đề xuất cấp hệ thống (sửa một lần, nhiều màn sống lại)

### 7.1 Khuôn màn thứ hai: «đầu trang sổ»

Thay `TopBar(tiêu đề) + Heading(H1 lặp ý) + 2 dòng xám` bằng một thành phần:

- **Vật thể** của màn (ký hoạ nhỏ 64–96dp: hoá đơn, thiệp, tờ thư, tem, instax) đặt lệch
  trái, hơi nghiêng.
- **Tiêu đề một lần** (không lặp giữa TopBar và H1).
- **Một câu kể** (tối đa một dòng), giọng hội bạn: «Tối nay ai trả trước?», không phải luật.
- Luật/quyền riêng tư chuyển thành **chú thích lề** (chữ 13sp, có mũi tên mực) hoặc «i».

Áp trước cho: Nhóm mới, Mời, Kèo mới, Chia bill 1/4/5, Đợt thu, Thả khoảnh khắc, Đăng bài,
Story, Sổ hai người.

### 7.2 Luật «mỗi màn một vật thể»

| Việc | Vật thể | Tài sản có sẵn |
|---|---|---|
| Tạo hội | Bìa sổ | `CoverBand`, `Grain vaiBia` |
| Mời | Thiệp + phong bì | `Washi`, `Stamp` |
| Kèo | Vé / thiệp rủ | `Washi`, `RouteLine` |
| Chặng | Thẻ quán | `KyHoa` |
| Chia bill | Hoá đơn in nhiệt | tờ HOÁ ĐƠN bước 2 |
| Kết quả chia | Cuống phiếu xé | `Stamp` |
| Đợt thu | Trang sổ thu | `Stamp` + celebrate |
| Kỷ niệm | Ảnh instax | `KhungAnh` |
| Story | Polaroid | `KhungAnh` |
| Sổ hai người | Tờ thư gấp ba | `ToGiay`, `Motif thuGapBa`, Nếp mảnh |
| Huy hiệu | Tem | `Sticker`, Nếp |
| Hồ sơ | Hộ chiếu / thẻ thành viên | `CoverBand` |

### 7.3 Nếp có vai ở mỗi lần đổi trạng thái

| Sự kiện | Tư thế đã có | Chỗ hiện |
|---|---|---|
| Mở khay tạo | `moi` | giữa khay |
| Chụp bill | `giu-khung` | bước 1 |
| Ghi sổ xong | `vui` | bước 5, cạnh dấu lớn |
| Tiền đã về | `dat-tay` | dòng vừa đóng dấu |
| Đề nghị lập sổ | `dua-giay` | chỗ ký |
| Sổ mở | `nhay` | trên bìa mở |
| Bản phác của Nếp | `ghi-lai` | lề tờ |
| Tờ đã khép | `gap-lai` | cuối sổ |
| Chốt kèo | `buoc-di` | trên vé |
| Chưa ai tới chặng | `doi` | cạnh chặng |
| Huy hiệu mới | `nang-to` | trên tem |

Luật để không thành ồn: một lần mỗi sự kiện (đúng luật `celebrate` đã có), Reduce Motion thì
đứng yên, không Nếp nào che nút (bài học 23/09).

### 7.4 Avatar có danh tính

- Màu nền avatar **băm từ person id** trong một bảng 8 màu lấy từ tầng `brand.*`/giấy (không
  đụng ba tông ngữ nghĩa); chữ cái giữ, nhưng hai «T» khác màu nhau.
- Ảnh đại diện khi có (đã có «Đổi ảnh đại diện» trong Cài đặt).
- Cụm avatar chồng nhau cho nhóm/kèo.
- Áp một lần ở `ui/Avatar.tsx`, ảnh hưởng: Thành viên, Bạn bè, Rủ ai?, Tin nhắn, chat, kết
  quả chia, đợt thu, quyết toán, check-in, hồ sơ, tường.

### 7.5 Nút con dấu cho mọi quyết định tạo mới

`StampButton` cỡ form (token `stamp-button-form` 52dp đã có trong `DESIGN.md`) thay
`RudiButton` ở **nút chính của mọi màn tạo mới** (Mở nhóm, Tạo kèo, Gửi lời mời, Ghi vào sổ,
Đề nghị lập sổ, Gửi cho người ấy, Thả khoảnh khắc). Các nút phụ/cài đặt giữ `RudiButton`.
Hệ quả: người dùng học được «dấu mực = mình vừa quyết định một việc có thật».

### 7.6 Chuyển cảnh giữa các bước

Chia bill 5 bước, sổ hai người, kèo mới đang đổi bước bằng thay nội dung tại chỗ. Đề xuất:
**lật trang** (bậc `shared` 300ms) giữa bước, và **một** khoảnh khắc `celebrate` 550ms ở bước
cuối. Không thêm hiệu ứng ở màn cài đặt.

### 7.7 Câu chữ

- Luật cho câu hiển thị ngay dưới tiêu đề: **một câu kể**. Mọi câu luật/quyền riêng tư đi vào
  chú thích lề hoặc «i», **không xoá** (chúng có lý do).
- Bỏ lặp: TopBar «Nhóm mới» + H1 «Đặt tên cho hội» → chỉ giữ một.
- Giọng Nếp cho các khoảnh khắc: «Xong! Sổ đã ghi, giờ đòi nhẹ nhàng thôi.» thay «Đã ghi: Hóa
  đơn của nhóm».

### 7.8 Bản tối

Giữ vân giấy tối (`paper-dark`) cho hàng/thẻ thay vì khối xanh phẳng; hạ độ sáng nút teal
trên nền tối.

---

## 8. Quy trình: vì sao cái đẹp không tới được màn thật, và cổng đề nghị

1. Mọi vòng review mỹ thuật đã ghi (`DESIGN.md`: chuyển mình 06/09, bản sắc 08/09, Nếp truyền
   giấy 12/09) đo trên **fixture** hoặc `ui-lab`. Phần live «chưa phủ».
2. Luồng live được viết theo từng lát cắt chức năng (API, idempotency, quyền riêng tư), mỗi lát
   cắt chọn thành phần kit gần nhất, và kit gần nhất luôn là `Field` + `RudiButton`.
3. Không có cổng nào đỏ khi một màn **không có vật thể/không có Nếp**; có cổng tương phản,
   em-dash, placeholder, nhưng không có cổng «sức sống».

Đề nghị (cần leader chốt, không tự áp):

- **Bảng ảnh live bắt buộc** cho mọi thay đổi UI: `npm run seed:rudi` trên stack cô lập rồi
  chụp đúng các checkpoint ở mục 5 (sáng, tối, 1.3). Reviewer đọc mù trên bảng live, không
  trên fixture.
- **Checklist 5 câu cho mỗi màn tạo mới**: có vật thể không? Nếp ở đâu? nút chính có là con
  dấu không? câu dưới tiêu đề là kể hay luật? kết thúc có khoảnh khắc không?
- Một cổng node nhỏ: mỗi file màn trong danh sách «màn tạo mới» phải import ít nhất một thành
  phần từ `ui/art` hoặc `Washi`/`ToGiay`/`KhungAnh`/`StampButton`. Rẻ, và bắt được đúng lỗi
  lượt này tìm ra (7 màn không nét vẽ). Cổng này chứng minh **có mặt**, không chứng minh
  **đẹp**: ảnh vẫn phải có người mở ra nhìn.

---

## 9. Tái lập

```bash
# Postgres 16 cục bộ (Docker Hub có thể 429)
initdb -D <dir> -U mobile --auth=trust && pg_ctl -D <dir> -o '-p 55432' start
createdb -h 127.0.0.1 -p 55432 -U mobile mobile

# API + cửa Go, cùng biến như scripts/e2e_slice.sh, thêm CORS cho web
export MOBILE_DATABASE_URL=postgresql+psycopg://mobile@127.0.0.1:55432/mobile
export MOBILE_OTP_DEBUG_CODE=000000 MOBILE_OTP_LOG_CODES=1 MOBILE_CORS_ALLOW_ORIGINS=http://localhost:8081
# + MOBILE_PERSON_ID_KEY, MOBILE_INTERNAL_TOKEN, MOBILE_MEDIA_ROOT (giá trị của lượt chạy)
(cd services/api && alembic upgrade head && python -m app.places.seed_catalog && uvicorn app.api.main:app --port 58098)
core migrate-chat && MOBILE_CORE_LISTEN=127.0.0.1:58099 MOBILE_PYTHON_UPSTREAM=http://127.0.0.1:58098 \
  MOBILE_CORE_CANDIDATE_ROUTES=ported core serve

# Thế giới seed
cd apps/mobile && npm run seed:rudi -- --api http://127.0.0.1:58099 --otp-code 000000

# App web
EXPO_PUBLIC_API_URL=http://127.0.0.1:58099 npx expo start --web --port 8081
```

Đăng nhập bằng số của người thứ `i` trong `ROSTER` (`soDienThoai(i)` trong
`tools/seed-rudi-world-lib.mjs`), mã `000000`. OTP có hạn mức theo số: một số dùng quá nhiều
lần trong giờ sẽ bị từ chối, đổi sang người khác. Trên web phiên chỉ nằm trong bộ nhớ, nên đi
bằng điều hướng trong app (`history.pushState` + `popstate`), không tải lại trang.

---

## 10. Cái còn mở

- Đo lại trên **native Android** các nhận xét về bóng/độ sâu và cú đóng dấu.
- Đo lại B6 và «Tờ hẹn AI» trên máy **có khoá AI**.
- Chụp nốt các màn ở mục 1.2.
- B1 cần người sửa sổ hai người xác nhận luật mong muốn (bản phác trước khi có sổ có được tồn
  tại không).
- Mọi đề xuất ở mục 5–7 là **hướng**; cần một lượt thiết kế (phác trên `ui-lab`, đọc mù) trước
  khi vào mã, và cần leader chọn thứ tự.

## 11. Bằng chứng đã xem

- ~100 ảnh chụp web 412×915 @2x trên stack cô lập đo tại `11fb3df`, gộp thành 24 bảng
  (`00-welcome` … `23-dang-nhap-otp-khoanh-khac-story`), gửi kèm phiên, **không** đưa vào Git.
- Mã: `src/rudi/screens/**` (41 màn), `src/rudi/ui.tsx`, `ui/Field.tsx`, `ui/art/*`,
  `art/nep.ts`, `art/canh.ts`, `screens/hai-nguoi/KhongGianGiay.tsx`,
  `screens/chia-bill/ChiaBillLive.tsx`, `screens/keo/CreateOutingLive.tsx`,
  `screens/groups/New.tsx`, `screens/Create.tsx`.
- Cơ sở dữ liệu của stack: `pair_papers`, `pair_notebook_cycles` (cho B1).
- Tài liệu: `DESIGN.md`, `PRODUCT.md`, `docs/claude/2026-09-23/qa-cap-doi-minh-linh.md`,
  `docs/claude/2026-09-24/chot-dot-ai-engine-va-dock-nep.md`.
