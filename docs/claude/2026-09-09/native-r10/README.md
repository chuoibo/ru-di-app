# Bảng native cho lượt sửa mày Nếp và phủ cảnh

Chạy trên `main` `cf770e3c`, một máy ảo `emulator-5554`, dev client 1.0.0.
Bộ flow: `apps/mobile/.maestro-bs-r10`.

## Đo được gì

| flow | câu hỏi nó trả lời | kết quả |
|---|---|---|
| `00-smoke-deeplink` | app mở được, và bundle trên máy **đúng là cây này** | xanh, dấu vân khớp |
| `09-canary-phai-do` | bảng có khả năng đỏ không | **đỏ đúng bước cuối** |
| `71-tam-sticker` | tám sticker ở khay 64 và bubble 120 | xanh |
| `73-canh-im-lang` | mười cảnh, cả bản có Nếp lẫn bản tắt Nếp | xanh |
| `74-tim-khong-ra` | cảnh trên **màn thật**, không phải bàn thử | xanh |
| `75-loi-co-canh` | màn lỗi thật có cảnh mặc định | xanh |

Điều đáng giá nhất: **tám góc coral trên máy thật đều là tam giác sạch**, không
còn vạch đen ở bất kỳ cỡ nào, và không path nào làm `PathParser` của
react-native-svg ném lúc mount — đó là rủi ro lớn nhất khi đụng vào tầng art.

Màn lỗi hiện `chua-doc-duoc` **không có Nếp**, và màn ấy không truyền
`illustration` dòng nào: cảnh đến từ chính `ErrorState`.

## Chưa đo được, nói thẳng

1. **Ô «Chưa có gợi ý» ở màn `ai-match` không bấm tới được.** Mọi chip lọc đều
   ra ít nhất ba kết quả trên dữ liệu mẫu, nên nhánh rỗng không tồn tại trên
   máy. Cảnh `bo-loc-che-het` của nó đã được nhìn ở bảng cảnh và ở màn Đi đâu,
   nhưng **đúng cái ô ấy thì chưa ai thấy trên máy**. Cần stack API thật.
2. **Ô rỗng của màn Đi đâu** cũng cần stack: không có máy chủ thì màn luôn ở
   trạng thái lỗi.
3. **Ảnh `04` bị thanh nổi bàn phím che phần trái.** Xem mục dưới.
4. Harness báo thiếu `android/.rudi-native-fingerprint`, nên **không kiểm được
   APK trên máy có khớp `package.json`/`app.json`**. Lượt này thuần JS nên
   bundle là thứ quyết định, nhưng dấu xanh không rộng hơn thế.

## Hai cái bẫy Maestro đã trả giá ở lượt này

- **`hideKeyboard` trên Android bấm Back**, và Back ở các màn này **pop ra khỏi
  màn**. Flow 74 và 75 bản đầu đỏ với *màn chào* và *màn Khám phá* trên hình,
  không phải màn đang đo. Bỏ `hideKeyboard` là xong: Maestro đọc cây view nên
  không cần giấu bàn phím mới thấy chữ. Giá phải trả là ảnh bị thanh nổi che.
- **Hai nhánh `when:` không tự loại trừ nhau.** Flow 75 bản đầu cho rằng «có ô
  tìm» nghĩa là danh sách đã tải; thật ra ô tìm hiện ở **cả** trạng thái lỗi,
  nên nhánh kia vẫn chạy trên màn đang lỗi rồi chờ một kết quả không bao giờ
  tới. Nhánh điều kiện phải kiểm cái **phân biệt** hai trạng thái.

## Ảnh

| file | là gì |
|---|---|
| `01-khay-tam-o.png` | khay tám ô, «Cà phê không?» hiện trọn không bị cắt |
| `02-tam-sticker-120.png` | sáu sticker ở 120dp: `met` và `quyet` dốc mày ngược nhau |
| `03-canh-moi-ab.png` | bốn cảnh mới, mỗi cảnh hai bản có và tắt Nếp |
| `04-tim-khong-ra-man-that.png` | cảnh trên màn Khám phá thật (bị bàn phím che một phần) |
| `05-loi-co-canh-man-that.png` | màn lỗi thật, cảnh mặc định, không có Nếp |
