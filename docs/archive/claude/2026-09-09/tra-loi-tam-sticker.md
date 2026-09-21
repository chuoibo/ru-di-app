# Tám sticker Nếp — nhân ngôn ngữ hình, không nhân một dáng

Ngày: 09/09/2026. Nhánh: `claude/p0-w-ui4-tam-sticker-nep` (xếp trên nhánh đóng F31/F32).
Nền: `docs/archive/codex/2026-09-08/review-delta-584-585.md` §4 — duyệt pilot `cho-ti`, cho vẽ bảy hình
còn lại, kèm một lời dặn cụ thể.

## Cái chặn thật, và nó không phải bảy bức vẽ

Review viết: «nhân ngôn ngữ hình, **không nhân cùng một pose rồi đổi đồ vật**». Đọc lại tầng art
thì thấy điều đó **không làm được** với `hinhNep` như nó đang có: pose chỉ đổi **tay**. Thân, mặt
và chân giống hệt nhau ở cả chín pose — một khuôn mặt duy nhất (một mày ngang, một mày vống, một
nụ cười xéo) và bốn lớp chân cố định đứng trên `CHAN_NEP`. Vì thế:

- «Đi thôi!» **không thể** có hướng di chuyển: không có dáng bước.
- «Kẹt xe» không có gì để ngồi lên.
- «OK, chốt!» và «Tuyệt vời» buộc phải đeo cùng một khuôn mặt, nên không thể khác cảm xúc.
- Và đó cũng là lý do **năm cảnh** trong `canh.ts` đang là một dáng người đổi đạo cụ.

Nên việc đầu tiên là mở ngữ pháp, không phải vẽ.

## Hai trục mới trong `art/nep.ts`

| trục | giá trị | đổi cái gì |
|---|---|---|
| `BIEU_CAM` | `binh-than` · `hao-hung` · `hoi` · `quyet` · `met` · `nhuong` | mày và miệng |
| `DANG` | `dung` · `buoc` · `nhun` · `ngoi` · `chong` | chân |

Mắt vẫn là **chấm** và vẫn theo `nhin`: mắt to, má hồng, ngôi sao là đúng register mà concept
note của team đã loại. `dung` là bản cũ **nguyên vẹn**, nên chín pose và năm cảnh đang có giữ
nguyên hình từng nét — cổng `art-duong` so lớp theo lớp và vẫn xanh.

Thêm `dam`: nhân **độ dày nét và bề dày chi**, không đụng hình học. Nó có vì `cho-ti` phải nhường
nửa khung cho ghế nên đứng ở 0.726 và ra **nhạt hơn** bảy hình bên cạnh; phóng to thì ghế rơi khỏi
khung (quan hệ `tiLeGhe = 57/46 × tiLeNep` là cứng, tính từ điểm tay chạm đầu trụ). Lead chọn
**cân lại nét, giữ nguyên bố cục** — đó chính là `dam: 1.16` cho một hình đã được duyệt, và PR
này nói rõ là đã đụng vào nó.

## Tám hành động

| id | nhãn | việc đang làm | dáng | mặt | đạo cụ |
|---|---|---|---|---|---|
| `di-thoi` | Đi thôi! | đang bước, vẫy theo | `buoc` | `hao-hung` | cờ + hai vạch chuyển động |
| `an-gi` | Ăn gì? | nâng tô rỗng lên hỏi | `dung` | `hoi` | tô |
| `cafe-khong` | Cà phê không? | đẩy ly về phía người nhận | `dung` | `nhuong` | ly |
| `ok-chot` | OK, chốt! | ấn dấu xuống tờ giấy | `chong` | `quyet` | giấy + dấu |
| `cho-ti` | Chờ tí | giữ ghế, nhìn đồng hồ | `dung` | `binh-than` | ghế + đồng hồ |
| `ket-xe` | Kẹt xe | ngồi trên xe không nhúc nhích | `ngoi` | `met` | xe |
| `tra-tien-ne` | Trả tiền nè | hai tay đưa tiền, hơi cúi | `dung` | `nhuong` | hai tờ |
| `tuyet-voi` | Tuyệt vời | hai chân rời sàn | `nhun` | `hao-hung` | tia |

Ngân sách đạo cụ mặc định là **một**. Đó là bài học đắt nhất của pilot: một người đọc ở 64dp mô tả
`cho-ti` là «ba đồ vật», và ba đồ vật đọc chậm hơn một khối.

`tra-tien-ne` **không** có dấu tick, đồng xu hay ký hiệu tiền tệ, đúng ranh giới ADR-0021: đây là
lời người gửi, tuyệt đối không được trông như hệ thống xác nhận tiền đã chuyển. Bản đầu vẽ một tờ
gấp góc và **đọc ra phong bì** — đúng register concept note loại — nên đổi sang hai tờ chồng nhau
với một vạch coral.

Ba lần sửa sau khi **mở ảnh ra nhìn**, không phải sau khi test xanh: tô che mất thân người nên
dời ra trước; ly quá nhỏ và tay với xuống nên nâng lên ngang tầm; cúi quá sâu nên đọc ra ngã.

![Tám sticker, nền sáng](tam-sticker/bang-tam-sang.png)
![Tám sticker, nền tối](tam-sticker/bang-tam-toi.png)
![Mười sáu pose ở ba cỡ](tam-sticker/bang-pose-sang.png)

## Khay: nhãn không bị cắt nữa

«Cà phê không?» bị cắt thành «Cà phê khôn…» vì ô `width: "22%"` với `numberOfLines={1}`. Tám nhãn
là từ vựng khoá ở ba nơi, nên **layout nhường, không phải chữ**: nhãn hai dòng với chiều cao dành
sẵn (tám ô bằng nhau), và số cột tụt theo `fontScale` (4 → 3 → 2). Bề rộng viết literal chứ không
ghép chuỗi — cổng `receipt.test.mjs` đọc **mọi** «…%» một build sinh ra, vì ADR-0009 cấm hiện phần
trăm của mô hình, và một bề rộng tính bằng template literal rơi đúng vào danh sách ấy.

## Lượt chấm mỹ thuật bắt ba hình đọc sai

Finish review trong context mới **mở ảnh ra nhìn** và trả `rebuild` cho ba hình. Cả ba đều là lỗi
hình, không phải lỗi gu, và tôi đã không thấy khi tự chấm:

| hình | lỗi | sửa |
|---|---|---|
| «Ăn gì?» | cả hai tay chạy từ vai TRÁI thành một thanh mực **bắc ngang thân và ngang miệng**; vành tô thấp hơn tay 4 đơn vị và rộng hơn khoảng tay 8, nên không tay nào chạm nó | thêm khuỷu như `dua-hai-tay` (chính pose bên cạnh đã ghi comment rằng chạy thẳng là sai); vành tô đặt đúng tầm tay, không rộng hơn khoảng tay |
| «Trả tiền nè» | hai tờ lệch 5 đơn vị dính thành **một thẻ bo tròn**, vạch coral nằm ngang giữa thân = ký hiệu **dải từ**; trên nền tối đọc ra **thẻ ngân hàng**. Cả hai bàn tay nằm TRONG khung tờ tiền và tờ tiền vẽ đè lên người, nên **không bàn tay nào hiện ra** | tiền bắt đầu TỪ tay và chạy ra xa thân, mép gần gấp về phía người đưa, người vẽ SAU tiền |
| «OK, chốt!» | **giống hệt «Trả tiền nè» ở 64dp**: cùng nghiêng, cùng khoảng chân, cùng mảng trắng cầm ngang hông phải; bàn tay ấn xuống lại bị giấy vẽ đè | ấn xuống sát sàn, tay kia chống ngược ra sau, giấy vẽ trước người |

Lượt chấm thứ hai xác nhận **không cặp nào còn trùng bóng**. Cặp gần nhau nhất còn lại là «Đi
thôi!» và «Tuyệt vời» (cùng một tay giơ lên tới dấu coral nhỏ góc trên phải); khoảng cách giữa
chúng do **dáng chân** giữ, nên ai làm phẳng `nhun` hay `buoc` là hai hình nhập một. Ghi ra đây để
người sau biết.

Nó cũng bắt được một lỗi tài liệu đúng loại tôi hay mắc: lần sửa đầu tôi hạ mày `hoi` xuống rồi
**viết comment nói đã tránh góc coral**, trong khi hình học nói ngược lại. Nay mày được nhướn là
mày TRÁI, phía ngoài tam giác gấp.

## Ảnh chụp máy thật

![Khay tám ô trên máy ảo, cỡ chữ 1.0](native/khay-tam-o-1.0.png)

Đây là câu trả lời cho hai câu hỏi mà bảng vector **không** trả lời được.

- **Nhãn «Cà phê không?» xuống hai dòng, không còn bị cắt.** Đúng thứ review nêu, và chỉ ảnh chụp
  máy mới chứng minh được vì nó phụ thuộc bề rộng thật của ô ở DPI thật.
- **Tám ô đọc ra tám hành động khác nhau ở cỡ thật**, trên nền ô `ground`, hai hàng bốn cột, ô bằng
  nhau vì chiều cao hai dòng nhãn được dành sẵn.

![Sáu sticker ở hai cỡ đọc trên bàn thử](native/tam-sticker-hai-co-1.0.png)

Bản 120 và bản rút gọn 64 cạnh nhau trên máy: bản nhỏ là hình vẽ thứ hai chứ không phải bản lớn thu
lại.

Một lỗi chỉ ảnh chụp máy mới lộ, đã sửa cùng lượt: nhãn trong hàng của bàn thử bị bóp còn một từ nên
«Chờ tí» hiện ra «Chờ». `Text` đứng cuối một hàng `flexWrap` giữ bề rộng một từ.

## Cổng

| Cổng | Kết quả |
|---|---|
| `npx tsc --noEmit` | sạch |
| `npm test` (cây sạch) | 770 pass, 0 fail |
| `tests/test_sticker_vocabulary_matches_client.py` | 4 passed (tám ID và nhãn không đổi) |
| `art-duong` + `rudi-chat-sticker` | xanh ở cả hai cỡ đọc, mười sáu pose |
| repo guard | pass, ảnh ghim sha256 |

Cổng `notDeepEqual` không còn ghim tên `cho-ti`: nay **lặp trên mọi id**, vì cả tám đều có bản đọc
thứ hai cho khay. Dòng cũ khẳng định `di-thoi` **không** có bản rút gọn đã bỏ.

## Bảng native: đi tới đâu, và tại sao dừng

Bốn lượt bảng trên máy ảo dùng chung. Kết quả sau cùng:

| flow | trạng thái | ghi chú |
|---|---|---|
| `00-smoke-deeplink` | xanh ở lượt 03:15 và 03:22, đỏ ở lượt cuối | app mở lại vào một trạng thái thứ ba mà nhánh đăng-xuất-nếu-có chưa xử |
| `71-tam-sticker` | **xanh** | khay tám ô, bubble, hai cỡ đọc — ảnh trong doc này |
| `73-canh-im-lang` | **xanh** | mười cảnh A/B — ảnh trong doc cảnh |
| `72-hang-cho` | đỏ | ba lần sửa mới tới được tiêu đề mục; lần cuối vẫn chưa cuộn tới hàng |

Ba lỗi flow đã sửa và đã ghi vào file: cuộn qua rồi đòi cuộn xuống; Maestro so **trọn** văn bản
node nên `text: "Cảnh rỗng"` không khớp tiêu đề đầy đủ; và `visibilityPercentage` mặc định 100%
làm phần tử cuối nội dung «không thấy» đúng lúc nó vừa ló ra.

**Không tiếp tục lặp**: ảnh cần cho lát này đã có và đã ghim. `72-hang-cho` chụp trạng thái hàng
chờ, thuộc PR #586 đã merge, và máy trạng thái ấy có mười ca test đơn vị. Ghi ra đây để lượt sau
biết nó dừng ở đâu chứ không phải để bỏ qua.

## Chưa chứng minh

- **Chưa chụp native, và lần này đã thử.** Máy ảo được lane khác trả lại lúc ~02:0x. Bảng
  `.maestro-bs-r9` (khay tám ô, ba trạng thái hàng chờ, cảnh) đã viết xong nhưng harness báo
  **máy chưa cài dev client**. Đã `expo prebuild` và `gradlew :app:assembleDebug` **thành công**
  (JDK 21 trên PATH là JRE, không có `javac`; phải chỉ `-Dorg.gradle.java.home` sang JDK 17), ra
  APK debug 263 MB gồm bốn ABI. `adb install` chạy **23 phút không tiến triển, 0% CPU**, rồi
  `adb shell` ngừng trả lời hẳn. Đã dừng lại thay vì làm hỏng máy ảo của lane khác. Nên: **hình
  đã được nhìn ở bảng vector, chưa được nhìn trên máy**. Lượt sau nên dựng APK **chỉ ABI x86_64**
  cho máy ảo, không phải cả bốn.
- **Chưa ai đọc mù.** Reviewer vòng trước nói đúng rằng biết trước đề bài thì không gọi là blind
  test. Bảng trên có nhãn ngay dưới hình; muốn đo thật thì che nhãn và hỏi người chưa đọc doc.
- Bộ này thay **cả bảy** hình cũ cùng lúc, đúng lời review («trình một bộ có chủ đích để so sánh»,
  không xin phép từng hình). Nếu team thấy một hình lệch, sửa hình ấy chứ không quay về khối phẳng.
