# Ảnh stock dùng trong RuDi Mobile

Tất cả ảnh dưới đây là dữ liệu trình diễn công khai, không chứa người dùng RuDi
hay dữ liệu nghiên cứu. Ảnh được tải ngày 2026-09-01 và chỉ dùng làm nội dung mẫu
cho các màn native.

| File | Nguồn | Tác giả / giấy phép | SHA-256 |
|---|---|---|---|
| `friends-rooftop.jpg` | [Unsplash](https://unsplash.com/photos/TB8vUq5vQBA) | Vitaly Gariev, Unsplash License | `0441952ff19dcbbf7ccfea3b80185a8f6a0da079055bf35a5f5059d7548bdd49` |
| `dalat-cafe.jpg` | [Pexels](https://www.pexels.com/photo/6040374/) | Kien Tran, Pexels License | `1efcdd2bc9c02c6c3727e8e02ccec6808469ce4fb86e20a8fac10d507727cbc3` |
| `dalat-friends.jpg` | [Pexels](https://www.pexels.com/photo/33865851/) | Luong Minh Toan, Pexels License | `9febeb1cb3f9361951755682664aa388bbaa522c844e6e8f5af84fa0c1b9ec3c` |
| `vietnam-road.jpg` | [Unsplash](https://unsplash.com/photos/LK-zehgutJc) | Hieu Do Quang, Unsplash License | `4c3d9e080fd4018f795e62258bb7e15071b92e6e84d9f21c7f001c470e840f8a` |
| `dark-wood-grain.jpg` | [Unsplash](https://unsplash.com/photos/05aeUGmSw5w) | Mike, Unsplash License | `dad8ec3b4e3d26cd9691451283d1656ce9488f2fb0fc08c8655d790cb68a4958` |

Không thay một file bằng ảnh khác mà giữ nguyên tên và digest. Nếu đổi ảnh, cập
nhật đồng thời nguồn, tác giả, giấy phép và SHA-256 trong bảng này.

## Quan hệ ảnh với địa điểm mẫu (từ 08/09)

Một ảnh chỉ được gắn vào địa điểm mẫu khi nói được **quan hệ** của nó (review
08/09 F01; ADR-0017 §2.4). Ba quan hệ được phép: *chính địa điểm* (chưa có ảnh
nào như vậy trong repo), *quanh đây* (khu vực, không phải quán), *minh hoạ*
(ảnh stock minh hoạ loại chỗ, không phải chỗ đó). Ảnh không có quan hệ thật thì
địa điểm để `anh: null` và khung vẽ đồ vật theo loại (`GuGlyph`).

| File | Quan hệ được phép | Dùng ở | Cỡ dùng thật |
|---|---|---|---|
| `dalat-cafe.jpg` | minh hoạ · quán cà phê | `PLACES` «Still Cafe Đà Lạt» với tiền tố «Ảnh minh hoạ: »; album mẫu | lead 16:10 và 21:9 · ô 4:3 · thumb 56 · ô 1:1 |
| `vietnam-road.jpg` | minh hoạ · đồi, đường đèo | `PLACES` «Đồi Thiên Phúc Đức» với tiền tố «Ảnh minh hoạ: »; ảnh bìa chuyến mẫu; album mẫu | như trên |
| `dalat-friends.jpg` | ảnh nhóm (người, không phải nơi) | album mẫu, ảnh đăng mẫu | 4:3 · 1:1 |
| `friends-rooftop.jpg` | ảnh nhóm | album mẫu, tường nhóm mẫu | 4:3 · 1:1 |
| `dark-wood-grain.jpg` | chất liệu (mặt bàn dưới tờ bill) | `Bill.tsx` | nền |

Không dùng `dark-wood-grain`, `dalat-friends` hay `friends-rooftop` làm ảnh của
một địa điểm. Ba đối tượng đầu của danh sách Khám phá mẫu (Tiệm Nướng, Bánh căn,
Lẩu gà) cố ý **không có ảnh**: không có ảnh nào trong repo nói được quan hệ với
món nướng, bánh căn hay lẩu gà.
