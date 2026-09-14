# Kịch bản chờ harness

Các kịch bản trong thư mục này hợp lệ (`parity lint pending-scenarios`) và đã chạy
được, nhưng **không nằm trong cổng** `gate.sh parity`: cổng chỉ đọc
`parity/scenarios`. Mỗi bộ ở đây chờ một việc cụ thể của harness; xong việc đó thì
chuyển bộ về `parity/scenarios/w2/` và chạy lại cổng.

| Bộ | Vì sao chưa vào cổng | Việc cần làm | Đã đo |
|---|---|---|---|
| `w2-storage-key-gap/stories/` | Tải ảnh ghi `uploaded_images.storage_key = secrets.token_hex(16)`: 32 hex ngẫu nhiên không bao giờ xuất hiện trong câu trả lời, nên normaliser không bind được và làn database luôn khác ở cột đó | Bind các chuỗi đúng 32 hex theo lần xuất hiện (`<hex32#n>`, như `<digest#n>`), thêm vào `orderMask` của dbsnap, có canary đổi khoá phải đỏ | Chỉ làn wire: 4 kịch bản/147 bước 0 khác biệt, kể cả khi Go phục vụ bốn route story; làn database: 9 khác biệt, đều là `storage_key` |
| `w2-limiter-bound/friends/` | Tra số đã đăng ký cần gọi `POST /identity/person-id` có bind, route này có limiter theo IP trong bộ nhớ; canary lặp qua mọi chế độ có thể chạm 429 ở phía reference và dừng INFRA | Làn limiter: stack khởi động lại theo kịch bản, cửa sổ phút tách khỏi canary | Chạy riêng: 1 kịch bản/28 bước 0 khác biệt |
