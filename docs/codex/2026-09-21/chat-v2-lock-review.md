# Review độc lập — đọc đồng thời và thu hồi quyền Chat v2

Reviewer: `/root/chat_live_stack`. Tác giả: `/root/chat_mass_realtime`.
Verdict: **APPROVE cho bản sửa khoá đọc và hai regression test nêu dưới**.
Đây là approval code, không phải approve production, MLS hoặc toàn harness tải.

## Phạm vi chính xác

| File | SHA-256 đã đọc |
|---|---|
| `services/core/internal/chatv2/store.go` | `00f3641f5765e9cda02b3f749e458fedd75e3d7ded5fd96647e50e27b2f002b3` |
| `services/core/internal/chatv2/load_concurrency_test.go` | `5fe8fe13d01e4c9cc24012c6257b6a31680c4745031143c3c7a206a139bc8a32` |
| `services/core/internal/chatv2http/load_postgres_test.go` | `9f2ed8fe9e3e3ad8b1dbac6d989f549abeb0394208a8fddb2c268daea3ccbf57` |

Reviewer đọc diff, toàn bộ đường authorize/send/events/mark, trigger invalidation
trong schema, test mới và helper chạy tiến trình thật. Parent đã tự chạy lại
hai test PostgreSQL/race và báo đạt; reviewer không trình bày đó là một lần
rerun do reviewer thực hiện. Không sửa product source trong review này.

## Kết luận về khoá và quyền

`Events` dùng `FOR SHARE` trên conversation, nên nhiều reader có thể giữ cùng
ranh giới epoch/sequence trong các transaction riêng. `Send` và `Mark` vẫn đi
qua wrapper exclusive, giữ `FOR UPDATE` trước tăng sequence hoặc ghi mark.
Không có đường đọc được đổi sang không khoá, và không có read transaction nào
nâng cấp SHARE sang UPDATE trong diff này.

Các hàng person/device/membership/chat-member vẫn được kiểm và giữ `FOR SHARE`
trước conversation; kiểm tài khoản đã xoá, device bị thu hồi, membership đúng
incarnation và pair block giữ nguyên. UPDATE thu hồi hoặc invalidation vẫn
phải chờ các khoá xung đột; sau commit, caller mới nhận từ chối hoặc epoch chưa
sẵn sàng. Không đổi migration, điều kiện ready hoặc chữ ký envelope.

`Events` materialize trang và commit trước khi handler ghi WebSocket hoặc chờ
ACK. Slow consumer vì thế không giữ SQL read transaction xuyên qua thời gian
chờ mạng. Diff không tạo thêm đường giữ khoá khi người dùng đứng yên.

## Giá trị của test mới

- Store test giữ một authorize/read transaction mở, rồi buộc reader thứ hai
  đọc dữ liệu thật trong deadline. Cơ chế exclusive cũ chặn ở đây. Sau đó test
  quan sát `pg_stat_activity.wait_event_type='Lock'` cho revoke, thả reader và
  kiểm cả device bị thu hồi lẫn reader còn lại trên epoch đã invalidated.
  Đây là regression có tác dụng, không chỉ đếm lời gọi mock.
- HTTP test dùng hai child process riêng, bearer session thật và PostgreSQL.
  Một client giữ ACK đầu tiên; client kia vẫn nhận event kế, session bị thu
  hồi làm socket quiet đóng, POST bị từ chối, slow client timeout và reconnect
  đúng sequence đã lưu. Test chứng minh cách ly **client không ACK**, không
  phải mô phỏng mọi trường hợp nghẽn TCP/băng thông, mobile background hoặc
  thước đo SLO tải.

Không thấy blocker trong thay đổi khoá nhỏ này. Các test không chứng minh toàn
bộ lịch xen kẽ của revoke/roster/rekey hoặc performance trên production.

## Ranh giới bằng chứng tải

Đã đọc thêm `cmd/chat-load`: báo cáo tự ghi rõ synthetic signed opaque
envelopes, HTTP/WebSocket/PostgreSQL thật, **không MLS/mobile E2E**. User/device
được provision trực tiếp vào database lab riêng; nhóm tối đa 100 người và 5
device/người, hai server process. Kết nối được tăng dần, reconnect cưỡng bức
10%; không gọi đây là kiểm connection storm hoặc toàn bộ chaos matrix.

RSS là mẫu theo process, không phải toàn bộ máy/PostgreSQL. Delivery latency
bắt đầu khi worker tạo send, không bao gồm mọi thời gian chờ trong generator.
Phải báo đúng duration/rate/connections và các failure metric của lần chạy;
không đổi một kết quả hữu hạn thành chứng nhận production-ready. Approval
trong tài liệu này chỉ dành cho ba file ở bảng phạm vi, không phải công bố SLO.
