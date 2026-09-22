# Một flow đỏ đẻ ra sáu flow đỏ giả: phiên đăng nhập là biến toàn cục

**Ngày 22-09-2026.** Bảng native lượt 2 có 10 đỏ, trong đó sáu flow (41, 42, 43,
44, 45, 47) chưa từng đỏ ở lượt trước. Cả sáu đỏ ra thành **«không tìm thấy
người»** — đọc y hệt lỗi sản phẩm. Không phải.

## Gốc rễ

Phiên đăng nhập là **biến toàn cục của bảng**. Phần lớn flow dùng khuôn:

```yaml
- runFlow:
    when:
      visible: "Rủ Đi thôi!"
    file: _dang-nhap-d.yaml
```

Đó là **«đăng nhập nếu đang đăng xuất»**, không phải **«đảm bảo là D»**. App đã
đăng nhập sẵn bằng ai đó thì bước này **SKIPPED** và flow chạy tiếp dưới danh
nghĩa người đang có — im lặng, không cảnh báo.

Bảng có hai loại flow:

| Loại | Làm gì | Ví dụ |
|---|---|---|
| **Neo** | tự đăng xuất rồi đăng nhập một người cụ thể | 22, 23(B), 24/25(C,D), 30(C), 36(**E**), 37(C), 46(F), 47 |
| **Thừa hưởng** | chỉ đăng nhập có điều kiện | 26–29, 31–35, **38–45** |

Một flow neo hỏng **trước** bước đăng xuất của nó sẽ để lại người sai cho mọi
flow thừa hưởng phía sau.

## Chuỗi thật, đọc từ log lượt 2

```
36 đăng nhập E  (người MỚI, chưa đặt tên)        → XANH, E giữ phiên
37 hỏng ở bước 13 = assertion ĐẦU TIÊN,
   trước `_dang-xuat-neu-co`                     → E Ở LẠI
38–45 đăng nhập có điều kiện                     → SKIPPED, chạy bằng E
   41 đỏ  E không có bạn        → «Nhắn tin cho ...» vắng
   42 đỏ  E không có bài        → «Mở bài: Bai co anh QA» vắng
   43 đỏ  E không có story
   44 đỏ  hồ sơ là «Thành viên mới»  ← ảnh chụp khớp chính xác
   45 đỏ  E chưa chặn Ut QA
   41 đỏ ⇒ cặp C–D KHÔNG được tạo
47 đăng nhập C ĐÚNG, nhưng không có cặp          → «Nhắn riêng» vắng → đỏ
```

**Một lỗi thật, sáu lỗi giả, qua hai tầng dây chuyền.** Tầng một là phiên sai;
tầng hai là dữ liệu mà flow 41 lẽ ra phải tạo.

## Bản sửa

`_tra-phien-ve-goc.yaml` + một dòng móc trong `chay_flow`: **sau mỗi flow đỏ,
đưa máy về màn chào**, để flow sau đăng nhập đúng người nó khai.

Đi đúng **đường đăng xuất của sản phẩm**, không `clearState`/`pm clear` — cái đó
đá dev client về launcher và xoá bundle. Flow reset tự `stopApp`/`launchApp` và
chờ màn ổn định trước, vì `_dang-xuat-neu-co` dùng `when:` mà `when:` không chờ.

Nếu **không** trả được về gốc, harness nói thẳng rằng các flow sau chạy bằng
người của lượt trước và đừng đọc màu của chúng như lỗi sản phẩm. Im lặng ở đây
chính là thứ đã tạo ra sáu đỏ giả.

## Đo, có đối chứng âm

Mini-bảng ba flow dựng lại đúng kịch bản: **22** đăng nhập E (neo, xanh) → **23**
cố tình đỏ bằng chuỗi không bao giờ có trên màn (biến độc lập) → **31** chỉ có
một assertion: `"Rủ Đi thôi!"` phải hiện.

| | 22 neo | 23 cố tình đỏ | **31 phép đo** |
|---|---|---|---|
| **gỡ lời gọi `tra_phien_ve_goc`** | xanh | đỏ | **ĐỎ** |
| **có bản sửa** | xanh | đỏ | **XANH** |

Có bản sửa, harness in: `sau 23-co-tinh-do đỏ: đã trả phiên về màn chào; flow
sau đăng nhập lại từ đầu`.

Phép đo cố tình **không** phụ thuộc tài khoản có tên hay dữ liệu seed — nó chỉ
đo đúng biến đã gây ra sáu đỏ giả.

Mini-bảng không commit: nó chứa một flow **cố tình đỏ**, và thư mục mini-bảng
vứt đi từng tích tụ thành rác trong repo này. Công thức dựng lại nằm ngay trên.

## Còn lại, không sửa ở đây

Khuôn «đăng nhập nếu đang đăng xuất» vẫn **lẫn lộn** «đảm bảo đã đăng nhập» với
«đảm bảo là ĐÚNG người». Ngay cả khi mọi flow đều xanh, các flow 31–35 và 38–45
chạy bằng **người mà flow neo gần nhất để lại** — sau flow 30 và 37 thì đó là
**C**, dù tên file chúng gọi là `_dang-nhap-d`.

Hôm nay chúng xanh vì phần lớn assertion chấp nhận cả C lẫn D (ví dụ flow 44 hỏi
`"(An|Ban) QA"`). Sửa triệt để phải cho mỗi flow **khai và kiểm** người nó cần,
nhưng đăng xuất/đăng nhập lại ở mọi flow sẽ đụng trần OTP (5 mã / 15 phút cho
mỗi số). Cần một cách nhận diện rẻ hơn — chưa có, và ghi ra đây thay vì để lần
sau tự phát hiện lại.
