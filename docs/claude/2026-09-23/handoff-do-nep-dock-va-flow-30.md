# Bàn giao: đo Nếp dock trên máy, và flow 30 còn nợ

commit gốc khi viết: `d1cdd671` (origin/main) · ngày 2026-09-23 · phiên `chat-backend-go-migration`

Tài liệu này để một phiên khác cầm tiếp mà không phải dò lại. Phần «chưa làm»
quan trọng hơn phần «đã làm», nên nó nằm trước.

Tài liệu song sinh: **`lich-su-phien-do-nep-dock.md`** cùng thư mục, kể lại
**đã xảy ra chuyện gì và vì sao**, gồm cả những chỗ phiên này làm sai. Đọc nó
nếu cần hiểu bối cảnh; đọc file này nếu chỉ cần biết làm gì tiếp.

## 1. Việc còn nợ, theo thứ tự nên làm

### 1a. Ba phép đo mà phiên `ai-engine-unification` đang CHỜ

Họ đã commit dock Nếp và nhờ đo hộ vì họ không lái máy ảo. Nhánh
`claude/p0-w30-nep-dock-trong-le`, SHA **`656c1235`** (trên `e4e98ae0`, rẽ từ
main `01f58872`), mở thành **PR #646**. Nhánh **chưa lên main**.

Phiên ấy cũng đã bàn giao, ở **PR #643**
(`docs/claude/2026-09-23/ban-giao-mot-engine-ai-va-dock-nep.md`). Hai tài liệu
trỏ sang nhau: phần chuẩn bị máy nằm ở đây, phần thiết kế dock nằm bên họ.

Mục 4 bên họ ban đầu chép trạng thái máy từ tin tôi gửi **trước** lần khởi động
lại 21:41, nên có lúc nói «`emulator-5554` đang chạy». Họ đã sửa ở commit
`416c2654`. Ghi lại ở đây không phải để trách, mà vì nó là bài học đúng cho
người đọc: **một bảng trạng thái máy bắt đầu sai ngay khi ai đó dán nó đi chỗ
khác.** Đừng tin bảng nào, kể cả mục 2 của tài liệu này — kiểm bằng `ps` trước.

| # | Phép đo | Kỳ vọng |
|---|---|---|
| 1 | Flow 25, **không** kèm `--tat-nep` | nút «Đồng ý» phải nhận được lời mời |
| 2 | `adb shell input tap` vào giữa tờ cài (mép phải, cách mép ~5dp) | chuyển sang `nep-dia`, **không** nổ Back |
| 3 | `adb shell input swipe` dọc 200px trong dải mép | Nếp dời trên ray, **không** nổ Back |

Worktree đã dựng sẵn: `~/wt-do-dock` tại `656c1235` (detached).
**`npm ci` CHƯA chạy ở đó.** Họ yêu cầu `npm ci` thật, không symlink
`node_modules`.

Phép đo 2 và 3 đã được chứng minh **ở tầng hệ thống** (xem mục 3), nhưng chưa
đo **trên chính tờ Nếp cài** vì lúc đo bản `an` chưa commit. Giờ đã có SHA nên
đo được.

### 1b. Flow 30 chưa chạy trên máy (nợ của chính phiên này)

Commit `12fdb418` trên main sửa sub-flow AI: bước thoát khay dùng đúng nút có
thật («Bỏ bản nháp»), thay vì nút bịa. Bản sửa mới chỉ được soát **tĩnh** trên
YAML, **chưa** chạy trên máy ảo lần nào.

Mini-bảng cần chạy: `00-smoke-deeplink`, `30-chat-that`, `09-canary-phai-do`.
Không bỏ `00` và canary: thiếu chúng thì bảng mất tiền đề và mất ca biết đỏ.

## 2. Trạng thái máy lúc bàn giao

| Thứ | Trạng thái |
|---|---|
| Máy ảo `rudi` | **ĐÃ CHẾT** lúc 21:41 (xem dưới). Phải bật lại. Khi còn sống nó là `emulator-5554`, Android 15 / SDK 35, Expo Go đã cài |
| adb | server đã chết theo máy; khi bật lại dùng cổng **5038** (5037 bị nuốt trên WSL2) |
| Stack backend | **không có** — 8099 và 8199 đều im |
| Metro | **không chạy** |
| `~/wt-do-dock` | có, tại `656c1235`, **chưa `npm ci`** |
| `~/wt-chat-ui` | sạch, trên `main` |

### Bẫy máy ảo phải biết trước khi chạy bảng

`adb devices` hiện ra **hai dòng cho cùng một máy**:

```
127.0.0.1:5555  device      <- cùng máy, bị `adb connect` thêm lần nữa
emulator-5554   device
```

Harness tự chọn **dòng đầu tiên** (`scripts/mobile_native.sh:155`), nên nếu
không ghim thì nó lái `127.0.0.1:5555`. **Luôn truyền `--serial emulator-5554`.**

`--serial` **có** tới Maestro: `scripts/mobile_native.sh:88` gán vào `SERIAL`,
và `SERIAL` đi vào `maestro --device "$SERIAL"` ở 6 chỗ (dòng 1665, 1799, 2159,
2191, 2311, 2378). Ghi chú cũ trong bộ nhớ nói ngược là **đã lỗi thời**.

### Máy này đã khởi động lại BA lần trong ngày 23/09

Cả hai lần đều trùng lúc nhiều việc nặng chạy **song song** (pytest đầy đủ +
go postgres tier + máy ảo + Metro + nhập ảnh). WSL cạn RAM thì khởi động lại cả
máy ảo Linux, kéo theo mọi phiên khác. Mỗi lần mất sạch `/tmp`, scratchpad, và
container DB `--rm`.

Nên: **chạy tuần tự**, kiểm `free -g` trước mỗi việc nặng, và đừng để việc chưa
commit nằm trong `/tmp`.

**Lần thứ ba: 21:41**, ngay trong lúc viết tài liệu này. Ba mốc boot trong ngày:
11:22, 20:05, 21:41. Lần này mất máy ảo vừa bật, adb server, `/tmp/rd-emulator-rudi.log`
và toàn bộ scratchpad. **Các worktree `~/wt-chat-ui` và `~/wt-do-dock` sống sót
nguyên vẹn** — cây git là thứ duy nhất đáng tin qua một lần khởi động lại.

Nên bước đầu tiên của phiên sau là **bật lại máy ảo**, đừng tin bảng trạng thái
ở trên là vẫn đúng. Kiểm bằng `ps -eo args | grep qemu-system` chứ đừng kiểm
bằng `adb devices` (adb tự dựng server mới và im lặng trả danh sách rỗng).

## 3. Cái đã đo được và đã chốt

### Dải cử chỉ mép màn: 29.7dp, nhưng chỉ nuốt vuốt ngang vào trong

Đo trên `emulator-5554` (1080×2400, density 420 → scale 2.625),
`navigation_mode = 2` tức điều hướng cử chỉ:

```
type=systemGestures  insetsSize={left=78, right=0}
type=systemGestures  insetsSize={left=0,  right=78}   -> 78/2.625 = 29.7dp
mSystemGestureExclusion = SkRegion()                  -> app KHÔNG khai loại trừ
```

Bốn phép đo hành vi, đối chứng dương chạy **đầu và cuối**:

| Thao tác trong dải mép phải | Back nổ? |
|---|---|
| Vuốt ngang vào trong (1075→600) | **CÓ** (đối chứng dương, đạt cả hai lần) |
| Chạm x=1040 vùng trống | không |
| Vuốt dọc x=1040, y 1400→1100 | không |
| Chạm x=1020 trúng node clickable thật | không, **app nhận và điều hướng** |

Node dùng cho phép đo cuối: `"Tôi có lời mời"` ở màn đăng nhập, bounds
`[42,1419][1038,1556]` — mép phải 1038 nằm trong dải 1002–1080.

**Kết luận:** đặt mục tiêu **chạm** ở mép là hợp lệ; đừng thiết kế thao tác
**vuốt ngang vào trong** ở mép. Không cần `setSystemGestureExclusionRects`.

Hệ quả tổng quát: lề 16dp = 42px nên nội dung dừng ở x=1038, tức **36px cuối
của mọi phần tử sát lề vốn đã nằm trong dải cử chỉ** và vẫn bấm được từ trước
tới nay.

### Phương pháp, dùng lại được

`adb shell input swipe` **có** kích hoạt cử chỉ hệ thống, nên nó là máy đo hợp
lệ. Hai điều bắt buộc:

1. Chạy **đối chứng dương ở đầu và cuối** loạt (vuốt ngang vào trong phải nổ
   Back). Nó không nổ thì cả loạt đo vô nghĩa.
2. «Back không nổ» **không bằng** «app nhận được». Phải chạm trúng một node
   clickable thật rồi xem nó có kích hoạt không.

Và: **lấy bounds thật bằng `uiautomator dump`, đừng đoán toạ độ.** Phiên này
mất một vòng vì bấm ước lượng trượt nút rồi suýt kết luận «chạm bơm vào không
tới app».

## 4. Lệnh để chạy tiếp

```bash
export ANDROID_ADB_SERVER_PORT=5038

# máy ảo (nếu đã tắt)
cd ~/wt-chat-ui && RD_AVD=rudi scripts/android_emulator.sh up

# stack: cần MOBILE_OTP_DEBUG_CODE + log sender, nếu không --otp sẽ từ chối
cd ~/wt-chat-ui && scripts/e2e_slice.sh --keep     # in ra URL/cổng của nó

# mini-bảng flow 30 (trên main)
scripts/mobile_native.sh --otp --serial emulator-5554 --api-port <CỔNG> \
  --flows "00-smoke-deeplink,30-chat-that,09-canary-phai-do" --keep

# ba phép đo cho nhánh dock
cd ~/wt-do-dock/apps/mobile && npm ci          # thật, không symlink
```

Trước mỗi lượt bảng OTP: `adb -s emulator-5554 shell pm clear com.lakiet.rudi`.
Phiên sót của lượt trước làm lượt sau đỏ giả.

## 5. Liên lạc

`ai-engine-unification` là phiên đang chờ ba phép đo ở mục 1a. Họ đã nói rõ họ
không lái máy ảo và sẽ không đo trên máy thật khi chưa hỏi. Khi có kết quả thì
báo lại cho họ kèm số đo, đừng chỉ nói đạt hay không đạt.

Phiên ấy **cũng đã bàn giao** (PR #643), nên người nhận kết quả sẽ là phiên sau
của họ, không phải họ. Các PR còn mở của lane đó: #644 tải realtime, #646 dock
Nếp, #647 engine ADR-0034, #648 luôn bật, #649 chất lượng AI.

Họ cũng đang chờ phản hồi về một luật mới trong `scripts/a11y_native_audit.py`:
node tương tác sát cạnh màn, cao ít nhất 48dp, được xếp riêng là `mep` thay vì
`small`. Phiên này đã đồng ý với điều kiện luật ấy **chỉ áp cho mục tiêu chạm**,
vì số đo ở mục 3 cho thấy mép thuộc về app với thao tác chạm chứ không với vuốt
ngang vào trong.
