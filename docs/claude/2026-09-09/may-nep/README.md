# Nếp có một mày, và cổng giữ góc coral không bị sơn mực

Nhánh `claude/p0-w-ui4-may-nep-khong-cat-nep-gap` · sau khi #586 · #587 · #588 đã vào `main`.

## Vì sao đụng vào hình đã duyệt

Team đã APPROVE hướng mỹ thuật, nên đây **không** phải đổi hướng. Đây là trả một
món nợ tôi tự ghi lúc gửi #587: *«năm khuôn mặt còn kết thúc mày phải bên trong
tam giác nếp gấp coral»*. Lúc ấy tôi để lại vì đang đóng F31/F32; lượt này mở ảnh
ra nhìn thì nó không phải chuyện nhỏ — ở bubble 120dp nó là **một vạch đen kẻ
ngang dấu nhận diện** của cả bộ.

## Bốn lỗi trong hình

1. **Mày phải cắt vào góc coral ở 5/6 biểu cảm.** `binh-than` · `hao-hung` ·
   `quyet` · `met` · `nhuong`. Chỉ `hoi` đã sửa ở #587. Xem `01-sau-mat-truoc.png`.
2. **`met` vẽ nhầm chiều.** Đầu trong chúc xuống là dáng **quyết tâm**, không phải
   mệt. Vì thế `met` và `quyet` gần như giống hệt nhau, và «Kẹt xe» đọc ra
   *cáu kỉnh* chứ không phải *chịu trận*.
3. **`binh-than` và `nhuong` cũng là cùng một khuôn mặt** — hai mày lệch nhau
   chưa tới một đơn vị trên cả quãng. Nay `nhuong` là **cung cong xuống**, ảnh
   gương của `hao-hung`.
4. **Pose `nang-bong` đóng một đĩa đen lên góc coral** — 2,78 đơn vị vào trong.
   Cái này **mắt thường không bắt được**, và cũng chưa ai thấy: chưa màn nào
   dùng pose ấy. Cổng mới bắt nó ngay lượt chạy đầu.

Ba lỗi đầu tìm ra bằng cách **mở ảnh ra nhìn**; lỗi thứ tư thì chỉ cổng thấy.

## Chọn phương án bằng cách dựng ảnh, không bằng suy luận toạ độ

`02-ba-phuong-an.png` dựng cạnh nhau cho cả sáu biểu cảm:

| | |
|---|---|
| **a · như đang ship** | vạch đen trên coral |
| **b · hạ mày xuống dưới nếp** | **tệ hơn** — mày dính vào mắt phải thành một khối tối |
| **c · bỏ mày phải** | sạch; mặt vẫn đủ diễn |

Chọn **c**. Nếp gấp che chỗ ấy, đúng việc một tờ giấy gấp làm, và «mắt dưới cặp
mày lệch» của concept thành nghĩa đen: **một** mày, lệch hẳn.

## `nang-bong` đã gỡ, không phải chỉnh cho vừa cổng

Pose ấy nâng một vật **ngang đầu**, đúng ô mà góc gấp chiếm. Trên cả dải nghiêng
đang dùng thì không có vị trí tay nào vừa giữ được ý của pose vừa tránh được
coral: đẩy tay gần vào thì tay kia và cánh tay xa lại cắt vào từ bên phải. Kéo
nó vào vừa đủ để cổng xanh ở **đúng độ nghiêng mặc định của nó** thì chỉ là làm
vừa lòng máy đo — nghiêng −6 (một giá trị `keo-ghe` đang dùng) là lại 2,3 đơn vị
vào trong. Không màn nào dùng pose này, nên gỡ.

## Cổng: cơ chế, và bốn thứ bản nháp đầu làm sai

`khongCatNepGap` trong `apps/mobile/tests/art-duong.test.mjs`. Lượt chấm trong
context mới bắt được cả bốn — ghi ra đây vì chúng là bốn cách một cổng **xanh mà
mù**:

1. **Đo sơn, không đo tâm nét.** Bản nháp so tâm nét với dung sai nửa đơn vị.
   Một nét dày 2.4 có tâm nằm **ngoài** 0.4 vẫn phủ **1,6 đơn vị** mực lên coral
   — nhiều hơn cả cái mày cũ mà cổng này sinh ra để chặn — và bản nháp **nhận**
   nó. Luật đúng là `sâu + net/2 <= 0`.
2. **Chặn trên đường cong, không chấm điểm.** Lấy mẫu ở t = ¼, ½, ¾ không phải
   là chặn: đường cong phình được ở giữa hai mẫu. Nay lấy N+1 mẫu **cộng sai số
   dây cung `max|B''|/(8N²)`** tính từ chính điểm điều khiển — vừa chặt (cỡ
   1e-3) vừa đúng. Bao lồi cũng đúng nhưng quá rộng: bao của một hình tròn vượt
   bán kính ~14%, đủ để vu oan mọi bàn tay **và cả hai con mắt**.
3. **Nhận diện nếp gấp, không đoán.** «Mảng coral đầu tiên» không phải phép nhận
   diện: ngòi bút chì của `ghi-lai` cũng là tam giác coral ba đỉnh, và mực chạm
   vào nó là đúng. Nay nhận theo **hình dạng bất biến với phép đặt** — hai đỉnh
   cùng một đường ngang, đáy : cao = 19 : 18 — vì cảnh và sticker đều truyền
   `x0`·`y0`·`tiLe` nên toạ độ 20/38 không còn. Bản kiểm theo toạ độ tuyệt đối
   **không nhận ra nếp gấp trong cả 25 bố cục thật** và im lặng báo sạch.
4. **Chạy trên thứ thật sự lên màn.** `hinhNep` đứng một mình không phải cái màn
   hình vẽ. Nay quét thêm **tám sticker × hai bản đọc** và **mười cảnh**, bỏ qua
   lớp nằm trước nếp gấp (bị chính mảng coral phủ lên) và miễn trừ **nét** vẽ
   trùng đường với một mảng — đó là đường viền của mảng ấy, không phải vết trên
   nó. Viết miễn trừ ấy cho **mọi** lớp thay vì chỉ cho nét thì nó nuốt luôn mọi
   **mảng mực** — hai con mắt, mọi bàn tay — mà sàn số điểm vẫn xanh.

Phạm vi quét là `nghieng ∈ [−8, 13]` × `dam ∈ {1, 1.3}`, không chỉ giá trị mặc
định của pose: cả hai là **override công khai**, và sticker «Chờ tí» đang dùng
cả hai cùng lúc (`nghieng: 5, dam: 1.16`). Quét theo mặc định thôi thì **không
bao giờ nhìn vào bản vẽ đang ship**.

Sàn số điểm đọc từ số đo thật: **8.577.360 điểm mực / 9.504 bản vẽ**, bản mỏng
nhất 826. Sàn đầu tiên tôi viết là «> 4000», lỏng gấp mười lần số thật.

**15 đột biến trên chính cổng, cả 15 đều đỏ** — gồm ba cái từng sống sót qua bản
nháp: bỏ bề dày nét, bỏ qua mọi lớp nét, và miễn trừ nhầm mọi lớp tô. Cũng đỏ khi
thu dải quét `nghieng` về một giá trị, khi bỏ quét sticker hoặc cảnh, và khi hạ
số mẫu trên mỗi cubic.

## Cái cổng KHÔNG đo được

Hình học thì gác được; «cái mày này đọc ra mệt hay đọc ra cáu» thì không. Vẽ
`met` ngược chiều lại **không** làm cổng nếp gấp đỏ, vì nó không cắt vào đâu cả.
Nên phần ấy được ghim bằng một ca riêng, đúng cái quyết định thiết kế chứ không
phải cái hình: **sáu biểu cảm phải cho sáu đường mày khác nhau**, và **`met` phải
dốc ngược `quyet`**. Hai đột biến tương ứng (trả `met` về chiều cũ, trả `nhuong`
về đúng đường của `binh-than`) đều đỏ.

## Ảnh

| file | là gì |
|---|---|
| `01-sau-mat-truoc.png` | sáu biểu cảm **trước**, crop đầu, 3 cỡ |
| `02-ba-phuong-an.png` | ba phương án cạnh nhau |
| `03-sau-mat-sau.png` | sáu biểu cảm **sau** |
| `04-tam-sticker-sau.png` | tám sticker ở 120 và 64 sau khi sửa |
| `05-muoi-canh-sau.png` | mười cảnh, có và tắt Nếp, sau khi sửa |
| `06-bang-pose-sau.png` | mười tám pose còn lại sau khi gỡ `nang-bong` |

## Không đụng tới

Bố cục, đạo cụ, dáng, tám id và nhãn sticker, mười id cảnh, `CANH_KHONG_NEP`.
Ảnh ghim của #587 và #588 **giữ nguyên** — chúng là biên bản của cái đã review,
không phải tài liệu sống.

## Chưa đo

Chưa chạy bảng native cho lượt này. Mọi ảnh ở đây là bản dựng headless từ chính
`dist-test`, không phải ảnh chụp máy thật.
