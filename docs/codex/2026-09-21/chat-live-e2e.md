# Chat nhiều người: đã chạy thật, chưa đạt production

Ngày 21/09/2026, bật stack riêng và thao tác Expo web bằng Chrome với **20 tài
khoản, 20 browser context độc lập**. Không mock API hoặc phản hồi AI. Cùng nhóm
có 21 thành viên: 20 người trong phép đo và một tài khoản kiểm thiết kế.
Đăng nhập OTP qua UI, vào nhóm bằng điều khiển app, rồi 20 người cùng bấm gửi.
API Go candidate 151 routes, PostgreSQL 16 bật durability; chi tiết ở
[stack](chat-live-stack.md). Không đổi ownership manifest hoặc production writer.

## Kết quả app thật trước/sau sửa

| Phép kiểm | Trước sửa | Sau sửa cursor |
|---|---:|---:|
| Tài khoản đăng nhập UI, context độc lập | 20 | 20 |
| Tin gửi cùng lúc / lưu ở server | 20/20 | 20/20 |
| Lượt tin xuất hiện trong DOM từng app / kỳ vọng | 234/400 | **400/400** |
| Tin trùng trong DOM sau ổn định | 0 | 0 |
| Thứ tự 20 tin so với trang server | Chưa đạt đủ tin | **20/20 client đúng** |
| p50 / p95 / p99 đến khi DOM chứa bubble | 331/3.940/3.999 ms | 241/1.404/3.055 ms |
| Receiver mất mạng, có tin trong lúc offline, tự nhận khi mạng trở lại | Đạt | Đạt |

Số đo DOM gồm phần tử được FlatList dựng ngoài viewport; **không có nghĩa 20
tin cùng nhìn thấy trong màn hình điện thoại**. Vòng đầu đã cuộn lịch sử để
phân biệt mất dữ liệu client với virtualisation: một client vẫn chỉ có 11/20
tin, xen giữa tin mới và lịch sử cũ. Backend giữ đủ 20. Vòng sau còn chạy một
phép đối chiếu độc lập thứ tự DOM với dữ liệu server, không chỉ đếm HTTP 201.

Thời gian tính từ lúc bắt đầu bấm gửi đồng thời tới MutationObserver thấy
bubble, cùng clock trên cùng máy. Có chi phí trình duyệt; không phải thời gian
paint/frame hay benchmark điện thoại. App dùng polling 4 giây nên chưa đạt
trải nghiệm realtime đã đặt ra. Một batch 20 tin không chứng minh soak hoặc
mọi race của cursor timestamp legacy.

Bằng chứng: [trước](chat-live-e2e/before.json), [sau](chat-live-e2e/after.json),
[thứ tự](chat-live-e2e/order.json), [provenance và bundle hashes](chat-live-e2e/provenance.json),
[ảnh UI 20 người](chat-live-e2e/01-after-20-users.png).
Artifact JSON trong Git bỏ nhật ký request và ma trận từng tin; bản đầy đủ
ngoài repo ở `/tmp/rudi-chat-browser-run2` và `/tmp/rudi-chat-browser-fixed`.
Không lưu token hoặc số điện thoại trong artifact Git.

## Lỗi đã sửa và kiểm lại

`useTinNhan` lấy tin mới nhất đang giữ làm cursor, trong đó có POST ACK của
chính người gửi. Nếu B gửi trước A một chút, A nhận ACK của mình rồi GET
`after=A`, tin của B bị bỏ qua vĩnh viễn dù server đã lưu. Cursor nhận nay chỉ
tiến từ trang GET; POST không thay nó. Bootstrap và catch-up cùng một khoá
in-flight theo generation, trang xuôi dùng `next_cursor` của server, tối đa
năm trang mỗi lượt rồi tiếp tục ở nhịp sau. Nhóm ban đầu rỗng vẫn mở lịch sử
khi một burst vượt kích thước trang.

Reviewer độc lập phát hiện thêm ba mép trước khi chấp thuận: trang xuôi ASC,
hai initial GET về ngược nhau tạo khoảng trống 150 tin, và history bị khoá
sau burst vào nhóm rỗng. Đều có regression. Parent chạy **54 test chat đạt,
không skip**, typecheck và export web đạt; reviewer chạy riêng 19 hook tests.
[Review và hashes](chat-cursor-review.md). Các test hook hỗ trợ hồi quy;
kết quả 400/400 là từ browser/backend/PostgreSQL thật sau build mới.

## Tính năng nhỏ và chat riêng

| Tình huống thực tế | Kết quả |
|---|---|
| Sticker Nếp: chọn trên khay, peer nhận | Đạt trước và sau sửa |
| Ảnh tổng hợp: picker → upload → peer tải/hiển thị | Đạt trước và sau sửa |
| Reply: long-press → trích dẫn → peer nhận đúng ID gốc | Đạt trước và sau sửa |
| Gửi khi offline, gõ draft mới, kết nối lại và retry | Đạt; draft mới còn nguyên |
| Reaction: bên bấm thấy, peer đang mở chat thấy sau 8,5 giây | **Không đạt: peer không thấy** |
| Xoá tin: người gửi thấy tombstone, peer sau 8,5 giây | **Không đạt: peer vẫn thấy nội dung cũ** |
| `/vote`: tạo poll, bỏ phiếu thật | Đạt thao tác của người bấm |
| Phiếu của người khác tự cập nhật | **Không đạt: một UI có 2 phiếu, UI kia/API có 3** |
| DM hai chiều bằng UI, đối chiếu dữ liệu server | Đạt trước sửa và kiểm lại sau build mới |
| Người ngoài đọc DM | API từ chối 403; pair không có trong danh sách |
| `/plan` gọi thật | Lưu yêu cầu 201, AI `unavailable`; **chưa có lịch trình** |
| `/chia-bill` | `chia_bill_not_available`; chưa đạt tác vụ |
| Voice | Chưa có tính năng; API không nhận `kind=voice` |

Các phép thử UI feature đã chạy lại trên bundle sửa cursor, vẫn đỏ đúng
reaction/delete; script trả exit 1 khi có finding, không đổi expected để xanh.
Hai lượt đầu dò automation có selector sheet/timing sai; không dùng timeout
đó làm bằng chứng lỗi product. Sau sửa automation, chờ sheet đóng thật và
thao tác picker thật, các kết quả ở bảng mới là kết quả nghiệm thu.
[Feature trước](chat-live-e2e/features-before.json),
[feature sau](chat-live-e2e/features-after.json),
[DM kiểm lại](chat-live-e2e/dm-after.json),
[DM/poll](../../chat-dm-poll.md),
[ảnh xoá chưa đồng bộ](chat-live-e2e/04-stale-deletion-before.png).

Forward poll chỉ đọc tin **mới** theo `created_at`, không mang thay đổi trên
tin đã có. `ThePoll` chỉ nạp lúc mount và sau phiếu của chính người dùng.
Đây là cơ chế phù hợp với các lỗi peer stale đã tái hiện, không phải lỗi
PostgreSQL không lưu reaction/xoá/phiếu. Cần luồng thay đổi có cursor bao gồm
message, reaction, tombstone và poll; kiểm lại cả trích dẫn và danh sách nhóm.
Không vá bằng vòng tải vô hạn toàn bộ lịch sử hoặc tuyên bố v2 đã nối app.

## UI và câu chuyện Nếp

Method: dual-agent — A `/root/chat_visual_review`, B `/root/chat_live_stack`.
A kết thúc trước khi parent nhận findings B. [Assessment A](chat-design-a.md)
chấm **24/40** theo Nielsen trên bề mặt live; [Assessment B](chat-design-b.md)
kiểm source và inject detector trên browser thật. CLI không báo finding ở
file chính; browser báo ba animation thuộc layout. Chưa có phép đo frame
chứng minh chúng gây lag, và lỗi extractor khiến chưa định vị đủ selector.

Nền giấy, mực đậm, đỏ gạch và [sticker Nếp](chat-live-e2e/03-nep-stickers.png)
giữ bản sắc. Nếp hợp ở việc mở lời, hỏi ăn, chờ nhau, chốt hẹn. Luồng nhiều
sender đọc được trong viewport đã xem, không thấy bubble chồng nhau. Tuy
nhiên câu chuyện từ **mở lời → cả hội quyết định → cùng đi** đang đứt ở plan
không chạy và phiếu chưa thống nhất. Không thêm linh vật vào lỗi để che vấn đề.

P1 riêng Expo web: Enter/Space không mở menu tin dù bubble nhận focus; chỉ
long-press chuột đã hoạt động. P2: lời giải thích AI lỗi mất khi gửi tin sau,
lệnh `/vote` thô lặp nội dung poll và đòi người mới nhớ cú pháp.
[Ảnh gọi plan thật](chat-live-e2e/02-plan-unavailable.png) cho thấy chính giới
hạn hiện tại. Chưa chốt UI “state of the art”; chưa kiểm VoiceOver/TalkBack,
IME, native FPS/jank hoặc release build Android/iPhone ở checkpoint này.
Questions skipped: người dùng đã yêu cầu kiểm toàn luồng; không cần hỏi lại
phạm vi hoặc xin quyền chạy các phép thử tổng hợp đã được giao.

## Cơ chế backend và cổng release

Tải Go v2 là phép đo **riêng**, chưa phải transport mà app trên đang dùng:
hai server process, 200 tài khoản × 5 thiết bị, 1.000 WebSocket, 5.996 tin mới
và 60 retry trong 60 giây. Nhận đủ 2.998.000 lượt, không trùng/sai sequence,
100 reconnect đạt. Đổi reader từ FOR UPDATE sang FOR SHARE giữ permission và
epoch ổn định; regression real PostgreSQL/race đạt, review độc lập chấp thuận
diff hẹp. Parent còn chạy riêng ca slow consumer, live revoke và resume qua
hai process thật, đạt. [Báo cáo tải](chat-mass-realtime.md).

**Latency vẫn không đạt:** p95 2.427 ms, p99 3.023 ms, vượt ngưỡng 800 ms/2 giây.
Chưa soak 30 phút/24 giờ, burst 300 tin/s, kill giữa transaction hoặc failover.
Hai lượt đầu generator thiếu workers không được dùng suy ra trần throughput.
Không tắt durability hay bỏ authorization để tăng số đo.

App hiện còn plaintext/polling, kho cũ chưa khoá ghi. Server Go legacy còn đọc
cửa sổ lịch sử và gu khi dựng input AI; đây là finding source, chưa gửi dữ
liệu sang model ở stack này vì provider chưa cấu hình. Nó chưa đáp ứng rule
v2 chỉ chia sẻ lời gọi/trích đoạn có consent của tác giả. Thiếu native MLS,
enrollment/rekey/recovery, media/voice E2EE và crypto review độc lập.

**Verdict: REQUEST_CHANGES cho release.** Các ưu tiên đóng cổng là đồng bộ đủ
loại sự kiện và thay cursor timestamp legacy bằng thứ tự commit đáng tin cậy;
hoàn tất E2EE/native và consent AI; tạo/sửa/chốt plan qua model thật; đạt lại
latency/load dài trên hạ tầng đại diện; rồi test Android/iPhone thật. Không
deploy, đổi writer hoặc coi Expo web thay các cổng đó.

## Chạy lại

```sh
bash scripts/chat_e2e_stack.sh up
node scripts/chat_e2e_seed.mjs /tmp/rudi-chat-e2e.XXXXXX/connection.json
# Lấy apiUrl từ connection.json ngoài repo; không in nội dung credential.
cd apps/mobile
EXPO_NO_DOTENV=1 EXPO_PUBLIC_API_URL=http://127.0.0.1:PORT \
  npx expo export --platform web --output-dir /tmp/rudi-chat-export
node tools/chat-e2e-web.mjs /tmp/rudi-chat-export 8177
# Ở terminal khác, cùng thư mục apps/mobile:
CHAT_E2E_SESSIONS=/tmp/rudi-chat-e2e.XXXXXX/sessions.json \
CHAT_E2E_OUTPUT=/tmp/rudi-chat-ui-run CHAT_E2E_KEEP_BROWSER=1 \
  node tools/chat-live-e2e.mjs
# Khi batch xong, dùng cùng hai biến trên cho:
# node tools/chat-live-integrity.mjs
# node tools/chat-live-features.mjs
# Sau cùng: touch /tmp/rudi-chat-ui-run/stop
```

`CHROME_PATH` trỏ bản Chrome có sẵn trên máy. Browser giữ 20 context để kiểm
tiếp; focus emulation giúp mọi app thực sự ở trạng thái active, không thay
đáp án network. OTP rate limit giữ nguyên, harness chờ khi gặp 429. Xem
[script DM/poll](../../../tools/chat-live-dm-poll.mjs) để kiểm hai người
trong cùng browser được giữ. Dừng static server bằng PID/terminal đã chạy;
dọn đúng stack bằng `bash scripts/chat_e2e_stack.sh down /tmp/rudi-chat-e2e.XXXXXX`.

Đã dọn stack, hai static server và các browser riêng sau khi giữ bằng chứng.
Không đụng dịch vụ/lane khác. Hai checkpoint sửa đã lưu local:
`35798a31` (khoá đọc/tải Go), `bea1f7e5` (cursor mobile); chưa push/merge/deploy.
