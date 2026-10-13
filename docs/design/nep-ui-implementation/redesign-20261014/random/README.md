# Đua vịt — hướng sân sông liên tục

Chủ sản phẩm yêu cầu tham khảo [Online Stopwatch Duck Race](https://www.online-stopwatch.com/duck-race/). Root đã mở game thật qua trình duyệt: setup, xuất phát, đang đua và kết thúc. Chỉ tham khảo cấu trúc sân, cá tính vịt và nhịp so kè; không lấy mã hay asset của website. Bản web ru-di-app không được dùng làm bằng chứng UI.

`duck-race-reference-v2.png` là mockup built-in ImageGen, **critique reference**, không phải approved comp hay screenshot native. Prompt nguyên văn nằm ở `.prompt.txt`; metadata nhúng bằng `imp embed-prompt`, provenance ghim SHA và decoded RGBA trước/sau. Root và reviewer độc lập đã mở ảnh.

Giữ mặt sông liên tục, vịt có phụ kiện, số theo vịt và đích caro. Khi code: preview thu lại trên màn thấp/chữ lớn, đồng hồ nhỏ, tên thắng đầy đủ trước sân thu gọn; không chép poster kết quả khổng lồ, cờ “1” trên vịt số 2, quảng cáo hoặc nhãn người đang xem. Copy vẫn là tìm **quán** có nguồn; Tạo kèo mở form thật.

Asset product mới ở `apps/mobile/assets/rudi/random/`: `river-arena-v2.png` và bốn sprite alpha `duck-{coral,mint,navy,lilac}-v3.png`, mỗi file có prompt và provenance riêng. Bốn phụ kiện cộng số/palette tám lựa chọn; số và tên không bake vào ảnh. Ảnh chỉ chứa nhân vật và sân tổng hợp. Tool dùng: built-in ImageGen; không CLI/API key, không sửa pixels bằng Python. PIL chỉ đọc/kiểm RGBA/alpha; copy và nhúng metadata không đổi pixels.

Đường chạy được tái hiện từ seed, có nhịp tiến khác nhau và vượt nhau, không lấy mẫu lại kết quả. Hai phiên bản tách rõ: v1 diễn riêng; v2 dùng mốc DB và monotonic clock theo ADR-0059. Mockup và unit không chứng minh motion, độ đẹp native hoặc hai máy live: các cổng đó còn mở cho đến khi có capture/đo thật.
