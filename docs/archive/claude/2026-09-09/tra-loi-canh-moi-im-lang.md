# Mỗi im lặng một cảnh — mười cảnh, và một luật được cưỡng chế thay vì được nhắc

Ngày: 09/09/2026. Nhánh: `claude/p0-w-ui4-canh-moi-im-lang` (xếp trên nhánh sticker).
Nền: review delta 08/09 §4 «với cảnh, mỗi cảnh cần một tình huống riêng; không đưa Nếp vào mọi
hàng dữ liệu chỉ để tăng độ trang trí» — và quyết định của lead: **phủ hết trạng thái rỗng**.

## Đếm trước khi vẽ

Vỏ có **46** chỗ mount `EmptyState`/`ErrorState`. **Năm** có hình. Hai trong năm cảnh đã vẽ
(`chua-co-hoi`, `chua-co-ban`) **không màn nào dùng**. Nên bài toán không phải «vẽ đẹp hơn» mà là
«bốn mươi mốt màn đang nói *không có gì ở đây* bằng một khoảng trắng».

Và năm cảnh đang có thì **dùng chung một dáng người**: `hinhNep` trước lượt này chỉ đổi tay, nên
ba trong năm cảnh là cùng một thân, cùng một khuôn mặt, chỉ khác đạo cụ — đúng thứ review cấm nhân
lên. Trục `BIEU_CAM` và `DANG` (nhánh sticker) là điều kiện để sửa được chỗ này.

## Mười cảnh, mười tình huống

| cảnh | tình huống | dáng · mặt |
|---|---|---|
| `chua-co-hoi` | kéo ghế ra, chừa chỗ | `dung` · `nhuong` |
| `chua-co-keo` | cúi xuống tờ hẹn trống, nét mực vừa bắt đầu | `dung` · `quyet` |
| `chua-co-anh` | nâng khung rỗng lên, **nhìn xuyên qua** nó | `dung` · `hoi` |
| `chua-co-ban` | **ngồi** ở bàn hai chỗ, tay mời sang ghế trống | `ngoi` · `nhuong` |
| `tim-khong-ra` | dò bản đồ, đường đi dừng giữa chừng | `dung` · `hoi` |
| `chua-co-tin-nhan` | trang giấy trắng, chấm mực đầu tiên | `dung` · `hoi` |
| `chua-co-loi-moi` | hai tay đưa một phong thư còn rỗng | `dung` · `nhuong` |
| `chua-co-ky-niem` | dây phơi ảnh, tấm đầu chưa được kẹp lên | `dung` · `binh-than` |
| `bo-loc-che-het` | tấm lưới che gần kín, còn đúng một khe | `dung` · `hoi` |
| `chua-doc-duoc` | tờ giấy rách góc, dòng chữ dừng ở vết rách | **không có người** |

`chua-co-hoi` và `chua-co-ban` từng là cùng một dáng cạnh một hoặc hai chiếc ghế. Nay một cái
**đứng kéo ghế** và một cái **ngồi mời sang chỗ đối diện** — hai tình huống, không phải hai bản
của một tình huống. Chỗ ngồi phải đặt theo **mặt ghế** chứ không theo sàn, nên nó là cảnh duy nhất
không dùng `nepTrenSan`.

![Mười cảnh, nền sáng](canh/bang-canh-sang.png)
![Mười cảnh, nền tối](canh/bang-canh-toi.png)

## Luật được cưỡng chế, không được nhắc

`DESIGN.md` có «Luật Nếp Đứng Xa Tiền»: nhân vật không bao giờ đứng cạnh lỗi, xung đột hay tiền.
Trạng thái lỗi **chính là** chỗ ấy, và nó có ở khoảng **hai mươi** màn, trong đó có màn sổ.

Một luật mà hai mươi nơi phải tự nhớ `nep={false}` là một luật sẽ hỏng ở lần thêm màn thứ hai mươi
mốt. Nên nó nằm trong `hinhCanh`: `CANH_KHONG_NEP` chứa `chua-doc-duoc`, và **yêu cầu vẽ Nếp bị bỏ
qua** cho id trong tập ấy. Cổng `art-duong` biết tập ấy và đòi bản có/không Nếp **giống hệt nhau**.

Và cảnh lỗi được gắn **một lần** vào `ErrorState`, nên khoảng hai mươi màn có hình mà không màn nào
phải nhớ gì.

## Nối vào đâu, và cố ý để trống chỗ nào

Nối mới: bình luận · kèo của nhóm · tường nhóm · tường người khác · bạn bè · hai tab lời mời ·
chat rỗng · điểm đến không khớp · story hết · danh sách nhóm. Cộng với `ErrorState` là khoảng ba
mươi chỗ.

**Cố ý để trống**: «Bạn chưa chặn ai», «Chưa có phiên nào», và các danh sách quản trị khác. Một
danh sách quản trị rỗng không cần ai kể chuyện; review đã cấm đưa hình kể chuyện vào mọi hàng dữ
liệu, và đây là chỗ áp dụng nó.

**Bỏ một cảnh đã vẽ**: `chua-co-thanh-tich`. Màn Thành tích luôn hiện đủ danh sách huy hiệu kèm
trạng thái nên **không có ca rỗng nào**; giữ một cảnh không màn nào dùng đúng là lỗi review đã nêu
về hai cảnh mồ côi. Vẽ rồi vẫn xoá.

## Lượt chấm bắt tôi tái phạm đúng lỗi review đã cấm

Finish review trả `rebuild`: **ba cảnh MỚI dùng lại pose của ba cảnh cũ**, chỉ đổi hình chữ nhật.
Đúng cái «một dáng đổi đạo cụ» mà review của team cấm, và tôi tái phạm ngay trong lượt sửa nó.

| cảnh | trùng với | thêm |
|---|---|---|
| `chua-co-tin-nhan` | `chua-co-keo`: cùng pose `ghi-lai` **và** cùng motif nền `gocGap` | ngòi bút chỉ vào khoảng không, cách chấm mực 13 sang phải, 26 xuống dưới |
| `chua-co-ky-niem` | `chua-co-anh`: cùng `giu-khung`, cùng điểm tay | câu cho trình đọc màn hình nói ngược với hình |
| `bo-loc-che-het` | `tim-khong-ra`: cùng `cam-ban-do` | chỉ khác hoa văn |

Vẽ lại bằng **ba pose mới** (`nang-bong`, `voi-len`, `ghe-nhin`) và **hai motif nền mới**, nên nay
mười cảnh khác nhau trên cả hai trục. Lượt chấm thứ hai xác nhận điều đó.

Hai lượt sau nữa bắt tiếp: một lỗi **quy trình** (ghi chú của tôi mô tả hình học **không có trong
commit** — lệnh sửa viết `cd apps/mobile && python3 <<PY` trong khi shell đã ở sẵn trong
`apps/mobile`, nên `cd` hỏng và cả chuỗi ngắt trước khi python chạy; tôi đọc dòng test xanh ngay
sau đó rồi báo là đã sửa), và một **hồi quy do chính lần sửa trước gây ra** (dời tay `ghe-nhin` làm
chi mực chạy xuyên tam giác gấp coral, đúng thứ vừa gỡ khỏi mày `hoi`, đến bằng đường khác).

Nó còn bắt bốn chỗ đặt sai mà tôi không thấy khi tự chấm: chiếc ghế đang ngồi bị thân người che
kín nên đọc ra «đứng trên ghế»; bàn tay mời dừng cách mặt bàn 15 đơn vị; `ghe-nhin` với hai tay
dang thẳng đọc ra **nhún vai** chứ không phải ghé nhìn, và ô hở của lưới nằm bên kia tấm lưới so
với cái đầu; tấm ảnh trong tay thấp thành mảng trắng **sau ống chân**, thò xuống dưới mặt sàn.

## Cổng

| Cổng | Kết quả |
|---|---|
| `npx tsc --noEmit` | sạch |
| `npm test` (cây sạch) | 770 pass, 0 fail |
| `art-duong` | mười cảnh, có và không Nếp, trong khung 144×112 |
| repo guard | pass, hai bảng ảnh ghim sha256 |

## Một ảnh chụp máy thật

![Cảnh «chưa có hội» trên màn «Chưa có nhóm nào», máy ảo, cỡ chữ 1.0](canh/native-chua-co-nhom-1.0.png)

Ảnh này chụp **trên máy ảo**, không phải bảng vector: cảnh `chua-co-hoi` ở 168dp trong màn «Chưa
có nhóm nào» của người mới — đúng chỗ lát này nối thêm. Nó dựng đúng cỡ, đứng trên nền giấy, cách
hai nút một khoảng đọc được.

**Nói rõ nguồn:** đây là ảnh `00-smoke-deeplink-FAILED.png` của một lượt bảng **đỏ**, và nó đỏ vì
lý do không liên quan tới lát này: máy dùng chung đang có **phiên đăng nhập Google thật** do lane
khác để lại, nên app mở thẳng vào «Nhóm của bạn» thay vì màn chào mà flow chờ. Ảnh vẫn là ảnh của
màn thật do bundle của cây này dựng. Flow đã được sửa để **đăng xuất nếu có** trước khi vào.

![Bốn cảnh mới, có Nếp và tắt Nếp, trên máy ảo](native/canh-moi-ab-1.0.png)

Và đây là bốn cảnh **mới** trên máy, ở **cả hai bản**. Ba điều đọc được từ ảnh mà bảng vector chỉ
gợi ý: mỗi cảnh đứng trọn vẹn khi tắt Nếp; câu cho trình đọc màn hình nằm ngay trên hình và **khớp
với hình** (bong bóng thoại rỗng, phong thư còn nguyên, hai chiếc kẹp trống, một ô để nhìn qua); và
ở cỡ thật, `bo-loc-che-het` đọc ra người ghé vào ô hở chứ không phải người đứng cạnh tấm lưới.

## Nợ ghi nhận, không sửa lượt này

**Chi mực đi qua góc gấp coral.** Góc gấp là dấu nhận diện của Nếp, và một nét mực đè lên nó là mất
dấu. Lượt này sửa hai chỗ **tôi vừa gây ra** (tay của `ghe-nhin` và `voi-len`) và một chỗ cũ (mày
`hoi`). Nhưng **năm khuôn mặt còn lại** — `binh-than`, `hao-hung`, `quyet`, `met`, `nhuong` — đều
kết thúc mày phải trong dải x ≥ 50, y < 38, tức bên trong tam giác gấp H(50,20)·G(69,38)·Bp(50,38).
Thấy rõ nhất ở `chua-co-ky-niem` (mặt `binh-than`).

Không sửa lượt này **có chủ ý**: đổi năm khuôn mặt là đổi hình của **mọi** pose và **mọi** cảnh đang
có, ở cuối một lượt đã dài; và hình gốc của Nếp đã được team duyệt với đúng hình học ấy. Ghi ra đây
kèm toạ độ để lượt sau sửa một lần cho cả bộ, chứ không phải để quên.

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
- Cỡ chữ 1.3/2.0 chưa đo với cảnh mới.
