# Tờ hẹn chung trên máy thật: lượt chạy đầu tiên của flow 48

**Ngày 22-09-2026.** `48-to-hen-chung.yaml` được viết ở lượt trước nhưng **chưa
bao giờ chạy**. Đây là lần đầu. Máy ảo `emulator-5554`, dev client nạp bundle từ
Metro của `wt-chat-ui` (`bcc64609`), backend là stack `e2e_slice --keep` có bật
`MOBILE_CHAT_CHANGES_CANDIDATE=1`.

## Cái phải sửa trước khi đo được gì

`scripts/e2e_slice.sh` **không** truyền `MOBILE_CHAT_CHANGES_CANDIDATE` sang
core, và lược đồ chat candidate do Go sở hữu nên không migrate thì route không
sống. Hệ quả: stack lên xanh, `/healthz` 200, mà `/contexts/{id}/shared-drafts`
và `/contexts/{id}/changes` đều **404** — đúng hai đường flow 48 đo.

Đã nối cờ **và** migration đi cùng nhau, theo đúng luật file ấy tự ghi cho OTP
và media root: *"whatever the API is started with, core is started with"*.
Kiểm trước/sau bằng số, không bằng niềm tin:

| | trước | sau |
|---|---|---|
| `GET /contexts/{id}/changes` | 404 | **401** |
| `GET /contexts/{id}/shared-drafts` | 404 | **405** |
| `POST /contexts/{id}/shared-drafts` | 404 | **400** |

## Flow 48 đỏ, và đỏ ĐÚNG

Lượt đầu chạy tới bước cuối rồi đỏ ở `.*đã thành kèo.*`. Mở ảnh ra nhìn thì
màn đang nói, bằng chữ cảnh báo ngay dưới tiêu đề khay:

> **Thêm ngày đi và ngày về trước khi chốt.**

Hai ô ngày còn là placeholder. `useToHenChung.confirm` từ chối chốt một tờ hẹn
chưa có khoảng ngày — và nói rõ vì sao, đúng chỗ. **Sản phẩm đúng; flow sai.**
Tôi viết flow ấy mà chưa từng chạy nó.

Flow đã sửa để **đo luôn cửa từ chối**: bấm «Chốt thành kèo» khi chưa có ngày,
đòi đúng câu ấy hiện ra, chụp ảnh, rồi mới điền ngày kiểu Việt và chốt. Lỗi của
tôi thành một ca âm vĩnh viễn, và tiện thể đo `ngayVeISO` trên bàn phím thật.

## Những gì máy thật đã chứng minh

Trước khi đỏ, lượt đầu đã đi qua — tất cả COMPLETED:

- `/vote` sinh thẻ bình chọn;
- **«Mở tờ hẹn chung» KHÔNG hiện khi phiếu còn mở** (ca âm đi trước ca dương);
- bỏ phiếu → chốt bình chọn → lối vào tờ hẹn chung hiện ra;
- mở tờ hẹn chung từ phiếu đã chốt; khay hiện «Tờ hẹn chung của hội»;
- sửa số người → «Lưu cho cả hội» → nhãn bản nhảy lên **bản 2**;
- «Bỏ tờ hẹn» hỏi lại trước khi phá, «Giữ lại» quay về đúng khay.

Nửa còn lại đo riêng sau khi sửa flow (`verify/chot-thanh-keo.yaml`): điền
03/10/2026 và 04/10/2026, lưu, rồi «Chốt thành kèo». Thẻ **đổi tại chỗ trong
luồng**:

> **Da Lat** · 19:00 Da Lat · **1 chặng · đã thành kèo**
> [ **Mở kèo của hội** ] · *Tờ hẹn chung đã chốt*

«Sửa cùng hội» biến mất đúng như thiết kế. Đây cũng là bằng chứng cho bản vá
trước đó về thẻ tự mâu thuẫn trên dây: thẻ giờ nhất quán là *đã chốt*, không
vừa `outing_id` vừa `draft.status: open`.

## Ba chỗ cổng bắt được mà CI không thấy

Bảng `--otp` chạy 25 flow. Ba đỏ là **của PR này**, và không cổng nào khác thấy:

| Flow | Tôi đổi gì | Tôi quên gì |
|---|---|---|
| 30 | vẽ lại trạng thái rỗng của chat | flow còn đòi câu cũ «Chưa có tin nhắn nào» |
| 34 | gộp hai nút gửi của khay soạn thành một nút «+» | flow còn đòi «Gửi ảnh» |
| 37 | cùng thay đổi ấy | flow còn bấm «Gửi sticker» |

Lý do CI mù: job «RuDi driven on a real Android emulator» của PR chạy **bảng
mặc định**, còn cả ba flow này đều sau cờ `--otp`. 901 test web và `tsc` cũng
xanh. Chỉ máy thật chạy đúng flow mới thấy.

Bản sửa flow 37 đã được máy thật xác nhận ngay trong lượt (khay → «Sticker» →
«Khay sticker» → gửi được). Flow 30 và 34 chỉ mới sửa theo chuỗi ký tự đọc từ
mã nguồn; phải chạy lại bảng sạch mới tính là bằng chứng.

## Ba đỏ KHÔNG phải của PR này

| Flow | Vì sao |
|---|---|
| 23 | ảnh chụp là **launcher Android**, app chưa kịp vẽ lúc assert chạy |
| 26 | chữ được đòi là câu máy chủ sinh lúc nhập; stack `e2e_slice` không sinh |
| 27 | flow bấm chặng **một** lần rồi đợi chuyển màn; mã luôn đòi **hai** lần khi chặng có `place_id`. `OutingLive.tsx` diff với `main` **rỗng**, và handler ấy giống hệt trên `main` — flow sai, và sai từ trước |

Tôi đã nghi bản sửa `HangChang` của chính mình gây ra flow 27 và **đã sai**.
Kiểm rồi mới nói, thay vì sửa một thứ không hỏng.

## Cái lượt này KHÔNG chứng minh

- Bảng này là **lượt trộn**: tôi sửa file flow trong lúc bảng chạy và harness
  đọc file theo từng lượt, nên flow 34 chạy bản cũ còn flow 37 chạy bản mới.
  Phải có một lượt sạch mới lấy làm bằng chứng đồng nhất.
- Đỏ của flow 37 (`Long press "Tin nhắn: Dep qua"`) là **hệ quả dây chuyền**:
  flow 34 dừng trước khi kịp gửi tin ấy.
- `--keep` giữ Metro và stack nhưng **không giữ `adb reverse`**. Chạy flow lẻ
  sau bảng thì dev client `ECONNREFUSED` tới `localhost:8095`, và Maestro báo
  y như «màn không hiện» — chỉ ảnh chụp và `DevLauncherErrorActivity` mới nói
  ra `java.net.ConnectException`. Cắm lại `tcp:8095` là xong.
- Flow 48 chạy **lần hai trên cùng nhóm** thì đỏ ở `assertNotVisible
  ".*Mở tờ hẹn chung.*"` vì phiếu đã chốt của lượt trước còn trên màn. Flow
  đúng cho một lượt trong bảng; chạy lặp thì thừa hưởng trạng thái cũ.
