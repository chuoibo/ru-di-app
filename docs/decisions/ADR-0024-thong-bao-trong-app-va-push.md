# ADR-0024 — Thông báo là những hàng sinh cùng transaction; push đi qua một sender cắm được; việc sau-response chỉ có một cửa

- **Trạng thái:** 🟡 **ĐỀ XUẤT** 2026-09-06 — chờ Lead đánh ĐÃ CHẤP NHẬN. Lát L7 của nhánh `claude/p0-w-m15-social-v1-1` không merge trước khi dòng này đổi. **Push thật chưa đo được cho tới khi Lead cấp cấu hình FCM** (mục 4). Lead đã duyệt kế hoạch chứa các quyết định này trong phiên 2026-09-06 và giữ mục push dù Claude không khuyến nghị.
- **Quyết định bởi:** Lead (phiên 2026-09-06; ghi lại ở mục 2).
- **Hiện thực:** nhánh `claude/p0-w-m15-social-v1-1`, lát L7; kế hoạch `~/.claude/plans/mellow-waddling-lantern.md` mục 5.8.
- **Thêm hai bảng, một họ route, một cơ chế chạy-sau-response, ba module native và một lần dựng lại dev client**; không đổi ba luật tiền (thông báo về đợt thu chỉ là một hàng chèn thêm, không chạm cột `_vnd`); không đổi cách chat đọc tin (vẫn poll khi màn mở).

## 1. Bối cảnh

App không có thông báo: tắt màn chat là không biết có tin mới, lời mời kết bạn, bình luận hay lời mời nhóm. `product/feature_list.md` EPIC 15 xếp «Smart notifications» ở P1, nhưng với một app nhắn tin đó là điều kiện dùng được. Máy chủ hiện **không có bất kỳ việc nền nào**: mọi thứ là request/response trong một transaction, `install_commit_before_response` (`app/api/unit_of_work.py`) commit trước khi gửi thân phản hồi. Client chưa có `expo-notifications`, `expo-video`, `expo-sharing`; thêm module native là dựng lại dev client.

Backend do Claude làm theo uỷ quyền ADR-0016 §2.3; charter không đổi.

## 2. Quyết định

### 2.1 Thông báo là hàng dữ liệu sinh cùng transaction với sự kiện
1. `notifications(person_id, kind, actor_id, context_id, subject_type, subject_id, created_at, read_at, pushed_at)` với `kind` đóng: `friend_request`, `friend_accepted`, `group_invite`, `post_comment`, `direct_message`, `debt_published`. Điểm sinh nằm **trong service, cùng transaction** với sự kiện: gửi/chấp nhận lời mời bạn, mời vào nhóm, bình luận bài của người khác, tin nhắn riêng (một hàng mỗi cặp mỗi ngày, cập nhật lại khi có tin mới), và `publish_batch` (một hàng cho mỗi người có nghĩa vụ — chỉ chèn hàng, không chạm bảng tiền).
2. Không sinh hàng cho từng tin nhắn nhóm (đã có đếm chưa đọc theo nhóm). Không sinh khi người nhận đã xoá tài khoản hoặc cặp bị chặn.
3. `GET /notifications` (cursor), `PUT /notifications/read-mark` (đánh dấu đọc tới một id), số chưa đọc trả kèm. `people.notify_prefs` (JSONB) tắt được từng `kind`; tắt chỉ ảnh hưởng push, hàng trong app vẫn có.

### 2.2 Push là một sender cắm được, mặc định là log
1. `app/api/push.py`: Protocol `PushSender.send(*, messages)`; `LogPushSender` (mặc định, ghi **số lượng**, không bao giờ ghi token hay nội dung); `ExpoPushSender` gọi `https://exp.host/--/api/v2/push/send` bằng `urllib`, timeout 5 giây, lỗi chỉ log tên kiểu exception. Chọn theo `MOBILE_PUSH_MODE=log|expo`; giá trị lạ **từ chối khởi động** (cùng hình `OtpConfigInvalid`).
2. Thiết bị: `notification_devices(person_id, platform, expo_push_token unique, created_at, last_seen_at)`; `POST /devices` (201/200, trả `id`), `DELETE /devices/{id}` (theo id vì token `ExponentPushToken[…]` có ngoặc). Token của người khác đăng ký lại → chuyển chủ.
3. Payload push chỉ có tiêu đề/câu đóng theo `kind` (ví dụ «An QA đã bình luận bài của bạn») và `data = {kind, id đối tượng, route}`; **không bao giờ mang nội dung bình luận hay tin nhắn**.
4. Gửi là idempotent nhờ `pushed_at`: worker `push_pending` lấy hàng chưa đẩy, gom theo người, tôn trọng `notify_prefs`, gửi, đánh dấu.

### 2.3 Việc sau-response có đúng một cửa
1. Máy chủ chấp nhận **một** ngoại lệ cho luật «không việc nền»: lớp `AfterResponse` trong `app/api/deps.py` bọc `BackgroundTasks` của Starlette, với đúng hai công việc được phép — `schedule_push(sender)` (ADR này) và `schedule_reel_render(render_id)` (ADR-0025). Route khai `after: AfterResponse` qua dependency và gọi phương thức; không route nào chạm `BackgroundTasks` hay `.add_task` trực tiếp.
2. Mỗi công việc **tự mở session mới** (`app.db.session.get_session_factory()`), tự commit/close; không dùng lại repository của request vì session ấy đã đóng khi task chạy (FastAPI 0.115.6 ghim, teardown dependency trước khi gửi phản hồi). Lỗi trong task chỉ log tên kiểu, không bao giờ lộ đường dẫn hay dữ liệu.
3. Test AST gốc `services/api/tests/test_background_tasks_boundary.py` quét `app/**`: tên `BackgroundTasks` và `.add_task(` chỉ được xuất hiện trong `app/api/deps.py`; không `Thread(`, `asyncio.create_task(`, `ThreadPoolExecutor(` ở đâu; `AfterResponse` chỉ có hai method job.

### 2.4 Client
Chuông + số chưa đọc ở tab Tin nhắn (pill/hàng dưới tiêu đề, không ở góc phải trên vì bánh răng dev-launcher che), màn `/notifications` chạm mở đúng đối tượng theo `kind`; đăng ký token một lần mỗi phiên sau đăng nhập — `getExpoPushTokenAsync` thất bại (thiếu FCM hoặc thiếu `extra.eas.projectId`) là **đường có kiểm soát**: không gọi `POST /devices`, không lỗi trên màn, mục Thông báo trong Cài đặt nói thật «Thông báo đẩy chưa bật trên bản này». Chạm thông báo hệ thống đi qua expo-router tới `data.route`.

### 2.5 Một lần dựng lại dev client cho cả L7 và L8
`expo-notifications`, `expo-video`, `expo-sharing` vào cùng một PR; `npx expo prebuild --platform android` không `--clean` (giữ `debug.keystore` → SHA-1 Google), gradle x86_64, `adb install -r`, ghi `android/.rudi-native-fingerprint`. Các lát L1–L6 không chạm `package.json`/`app.json`. Thêm `apps/mobile/tests/quyen-app-json.test.mjs` ghim danh sách quyền Android vì hiện không cổng nào canh `app.json`.

## 3. Hệ quả

- Migration `1c7a3e9d5b08`. Sáu route sinh thông báo gọi `after.schedule_push(...)`; `publish_batch` nằm trong số đó — thay đổi ở đường tiền chỉ là một dòng chèn hàng và bộ test tiền phải xanh y nguyên.
- `e2e_slice.sh` và compose có thể đặt `MOBILE_PUSH_MODE=expo` + `MOBILE_EXPO_ACCESS_TOKEN` khi có credentials, **không đổi code**; file `google-services.json` để ngoài Git và ngoài worktree (repo guard chặn key).
- `erase_person` (ADR-0023) xoá thiết bị và thông báo của người rời đi.

## 4. Cái này KHÔNG chứng minh

- Push **giao tới máy thật**: emulator không có FCM; `getExpoPushTokenAsync` chắc chắn thất bại trên dev client hiện tại. Chứng minh được: hàng thiết bị lưu qua route, `LogPushSender` ghi số lượng không ghi token, payload của `ExpoPushSender` với HTTP giả, thứ tự `commit → thân phản hồi → push`, trung tâm thông báo trong app trên emulator (flow 48). Khi Lead cấp FCM và `projectId`: đặt env, dựng lại với `google-services.json`, chạy flow 48 và đọc log «handed to expo»; iOS ngoài phạm vi.
- Expo Push API trả lỗi theo lô (token hỏng, hết hạn): worker chỉ log, không thu hồi thiết bị tự động trong v1.
- Thứ tự teardown giữa FastAPI ghim (0.115.6) và bản trên máy dev (0.135.x) có thể khác — luật «task tự mở session» làm nó không quan trọng, nhưng không được đo trên cả hai.

## 5. Phương án đã bác

| Phương án | Vì sao không |
|---|---|
| Celery/RQ + Redis | Không có hạ tầng trong compose; một dịch vụ mới để gửi vài trăm push mỗi ngày |
| WebSocket cho chat và thông báo | ADR-0016 đã bác; poll khi màn mở + push khi màn tắt đủ cho v1 |
| Gọi FCM trực tiếp | Expo Push API trừu tượng hoá FCM/APNs với một token dạng; đổi sau nếu cần |
| `add_task` rải ở từng route | Không cổng nào đếm được; gom về một lớp để test AST canh |
| Gửi push đồng bộ trong request | Độ trễ mạng của bên thứ ba vào mọi lần gửi tin |
| Không làm push, chỉ trung tâm trong app | Lead chọn push; phần trong app vẫn là phần được chứng minh trước |

## 6. Cách kiểm chứng

- Domain: `should_notify`, `push_text` (không chứa nội dung), `is_same_day`.
- API (fake): sinh đúng người, không sinh cho chính mình, gộp DM theo ngày, read-mark, unread; `ExpoPushSender` với `urlopen` giả (payload đúng, không nội dung, lỗi mạng chỉ log tên kiểu); `create_app` với `MOBILE_PUSH_MODE` lạ → từ chối khởi động; `test_push_runs_after_response.py`: thứ tự `commit → body → push`, session request đã đóng khi task chạy.
- Postgres: index inbox; `pushed_at`; unique token đổi chủ; `publish_batch` sinh `debt_published` mà `test_money_*` vẫn xanh.
- Gốc: `test_background_tasks_boundary.py`; `quyen-app-json.test.mjs`.
- Emulator: flow 48 (badge → danh sách → chạm mở đúng bài; Cài đặt nói «chưa bật» thật) + `kiem_may_chu_sau_48` (hàng `read_at` đổi; `POST /devices` curl 201; log có `LogPushSender`; canary: hàng của D không có trong danh sách của C).
- Đột biến phải đỏ: đưa `.add_task` vào một route → test AST đỏ; task dùng lại `repository` của request → test AST đỏ; payload chứa `body` bình luận → test payload đỏ; bỏ `pushed_at` → ca idempotent đỏ.
