# Chạy hai class chat hai người trên máy thật, và ảnh web của trang lab

- Ngày: 2026-09-28 · nhánh `claude/peaceful-hopper-32kwjs`, dựng trên `8207f34b`; ghi chú này đi
  trong commit lát P6 ngay sau đó (SHA ở `git log -- docs/claude/2026-09-28/chay-may-that.md`).
- protocol_version: không áp dụng (không đụng tiền, không đụng `docs/protocol/v1`). Verdict: chưa có
  (chưa có reviewer thật).
- Mô hình (chủ sản phẩm, ADR-0046 §8.4): **đám bạn** = nhóm và chat hai người thường, y hệt nhóm
  («@Rủ Đi», «/plan», «/chia-bill», khay có «Tờ hẹn»); **cặp đôi** = chat hai người mà cả hai đã bật
  «Một đôi» (cờ máy chủ `cap_doi`), thêm hàng ghim «Tờ giấy», công cụ thứ năm «Tờ giấy» trong khay và
  bốn sticker «Cho hai người». Đám bạn có lời đề nghị của người kia đang chờ thì thấy hàng mời mỏng
  (`hangGhimChat` trong `so-doi-map.ts`).

## 1. Chưa có bằng chứng máy thật

Máy làm lát này không có emulator. **Chưa flow Maestro nào dưới đây chạy trên máy.** Ảnh ở mục 5 là
**bằng chứng web** (`expo export --platform web` + Chromium headless), không phải bằng chứng máy:
react-native-web là một target khác target ship, và `scripts/mobile_native.sh` ghi lại bốn lần nó đã
nói dối. Web chỉ nói trang vẽ được và đọc được ở các ô đã chụp.

## 2. Điều kiện trước khi chạy

- `adb` trên `PATH` (mặc định lấy từ `$ANDROID_HOME/platform-tools`, `ANDROID_HOME` mặc định
  `~/Android/Sdk`), một máy Android đang ở trạng thái `device` (`adb devices`); nhiều máy thì đặt
  `--serial`.
- `maestro` trên `PATH` (script thêm `~/.maestro/bin`).
- Dev client của chính app, `com.lakiet.rudi`, đã cài trên máy. Dựng: `cd apps/mobile && npx expo
  prebuild --platform android`, rồi `cd android && ./gradlew :app:assembleDebug
  -PreactNativeArchitectures=x86_64`, rồi `adb install -r` file apk vừa ra. Thiếu thì script thoát 2
  và nói thiếu gì.
- `npm ci` trong `apps/mobile`; cổng Metro trống (mặc định 8095, đổi bằng `MOBILE_METRO_PORT`). Cổng
  đã có người nghe là ĐỎ: Metro của cây khác phục vụ một bundle hợp lệ mà máy không phân biệt được.
- Một API chế độ **prod** (không đặt `MOBILE_AUTH_MODE=dev`) có cửa trước Go, chạy với
  `MOBILE_OTP_DEBUG_CODE=000000` và log sender (`MOBILE_OTP_LOG_CODES=1`), cho cả uvicorn lẫn `core
  serve`. Cách gọn nhất: `scripts/e2e_slice.sh --keep`, nó in ra URL của cửa trước; cổng trong URL đó
  là `<port>`. Script kiểm mã debug trước bảng: API không nhận mã thì thoát 2.
- Cho `--ai`, thêm:
  - khoá mô hình còn sống (`GEMINI_API_KEY` ở tiến trình chạy worker AI). Script thăm dò khoá bằng
    chính đường sản phẩm trước bảng; khoá chết là ĐỎ, không phải bỏ qua;
  - `MOBILE_AI_ENGINE_GROUP=go` cho mọi tiến trình `core` (serve, và tiến trình worker nếu tách). Mặc
    định là `brain`, mà brain Python không có đường cho chat hai người (ADR-0046 §8.1): capabilities
    của cặp báo `provider_unavailable`, và `_41-ai-hai-nguoi-co-khoa.yaml` đỏ ở bước chờ «Hỏi Rủ Đi
    AI». `scripts/e2e_slice.sh` hiện **không** đặt biến này, nên phải tự đặt khi khởi động `core`;
  - lược đồ chat và RAG đã cài (`core migrate-chat`, `core migrate-rag`): engine Go từ chối khởi
    động khi thiếu.

## 3. Lệnh

```bash
# Bảng OTP đầy đủ (22..47, trừ 38/40), máy chủ KHÔNG có khoá mô hình:
scripts/mobile_native.sh --otp --api-port <port>

# Thêm flow 40 và nhánh có khoá của 30/41 (cần mục 2, phần --ai):
scripts/mobile_native.sh --otp --api-port <port> --ai
```

Mã thoát: 0 xanh, 1 đỏ, 2 **không đo được** (thiếu adb/maestro/máy/dev client/API). Mã 2 không bao giờ
là xanh.

CI: job `mobile-native` trong `.github/workflows/test.yml` chạy trên `vars.MOBILE_NATIVE_RUNNER` (vắng
thì `ubuntu-latest`). Khi biến trống, mã 2 thành cảnh báo; khi đã đặt runner, mã 2 thành lỗi (đã hứa có
máy thì phải đo được). Job đó gọi `scripts/mobile_native.sh` **không có** `--otp`, nên flow 41 và 47 không
chạy ở CI kể cả khi có runner; chúng chỉ chạy khi có người lái lệnh trên với một API prod.

## 4. Flow nào phủ hai class, và mỗi flow khẳng định gì

Thứ tự trong một lượt: 41 chạy sau 39 và để lại một cặp thật giữa C và D; 47 cần đúng cặp đó.

- `41-nhan-rieng.yaml` (đám bạn): mở chat hai người từ «Bạn bè» → «Nhắn tin cho …»; có «Xem hồ sơ»,
  không có «… thành viên · xem và mời»; gửi «Chao rieng»; hàng «Nhắn riêng» đọc «Bạn: Chao rieng»; mở
  lại từ hồ sơ vẫn là cặp đó. Gõ «/»: gợi ý «Rủ Đi AI phác lịch trình» và «Rủ Đi AI gom khoản chi để
  hai bạn xác nhận», **không** có «… để cả hội xác nhận». Harness kiểm máy chủ sau flow
  (`kiem_may_chu_sau_41`): người kia thấy đúng một context `pair`, tin nằm trong cặp; canary mở DM với
  người chưa là bạn → 404, đổi tên cặp → 409.
- `_41-ai-hai-nguoi-khong-khoa.yaml` (AI=0): chip nói «Rủ Đi AI chưa sẵn sàng · Gửi như tin thường»
  trước khi gửi, không có «Chỉ gửi lời nhờ»; tin vào luồng như tin thường, không có «Rủ Đi AI chưa
  nhận lời nhờ»; khay có «Tờ hẹn», **không** có «Tờ giấy»; bảng tờ hẹn nói «AI chưa sẵn sàng. Bạn vẫn
  có thể tự tạo kèo.», không có «Hỏi Rủ Đi AI».
- `_41-ai-hai-nguoi-co-khoa.yaml` (AI=1): khay có «Tờ hẹn», không có «Tờ giấy»; bảng tờ hẹn nói «…
  hiện cho cả hai bạn …»; «Hỏi Rủ Đi AI» điền «/plan »; chip «Kèm N tin gần đây» và «Chỉ gửi lời
  nhờ»; tấm «Xem» nói «… cả hai bạn …», không «cả nhóm»; câu trả lời ký «Rủ Đi AI · đọc N tin» (hoặc
  «· chỉ đọc lời nhờ») trích «Trả lời Bạn: /plan goi y cho hai minh».
- `47-to-giay-hai-nguoi.yaml` (đám bạn → cặp đôi, hai tài khoản trên một máy): C chưa là cặp đôi,
  **không** có hàng ghim «Tờ giấy của hai mình», đề nghị lập sổ qua «Cài đặt nhóm» → «Tờ giấy của hai
  mình»; D thấy hàng mời «… đề nghị lập sổ lời hẹn. Mở để xem và trả lời.» trong chat, đồng ý, sổ mở
  mà chưa là cặp đôi (không có «Rủ đi chơi»), đề nghị bật «Một đôi»; C vẫn không có hàng ghim nhưng
  thấy hàng mời «… đề nghị hai bạn là «Một đôi». …», đồng ý; D giờ là cặp đôi, hàng ghim hiện, phác và
  gửi một tờ; C mở tờ, «Ừ» → «Đã chốt». Chưa có hàm kiểm máy chủ sau flow 47 trong harness.
- `30-chat-that.yaml` + `_30-ai-*` và `40-ai-plan.yaml` phủ phía nhóm của cùng luồng AI (ADR-0046 §8.2),
  không phủ gì riêng của chat hai người.

Chưa flow nào lái khay cặp đôi với năm công cụ, bốn sticker «Cho hai người», hay lời đề nghị đến từ
phía cặp đôi đã thành. Ba thứ đó hiện chỉ có ảnh web ở mục 5.

## 5. Ảnh web đã chụp (trang lab `app/dev/hai-lop-chat.tsx`)

Trang lab chỉ có ở bản dev có `EXPO_PUBLIC_RUDI_FIXTURE=1` (như `ui-lab`, `tra-loi-song`), vẽ bằng đúng
thành phần màn chat dùng, dữ liệu bịa («Linh», «Minh»). Dựng: `EXPO_PUBLIC_RUDI_FIXTURE=1 npx expo
export --platform web --dev --output-dir /tmp/rudi-web`, phục vụ bằng `node tools/chat-e2e-web.mjs
/tmp/rudi-web <cổng>`, lái bằng `puppeteer-core` với Chromium, giả lập `prefers-color-scheme` và
`prefers-reduced-motion`. 390x844 @2x, và 360x780 @2x cho khay. Ảnh nằm ngoài repo; mỗi ảnh đã mở ra
nhìn.

| Ảnh | Nhìn thấy |
|---|---|
| `01-dau-trang-sang/toi` | Đầu trang, hai hàng mời và hai hàng ghim; không chữ nào bị cắt; tối đọc rõ. |
| `02-hang-moi-va-to-giay-sang/toi` | Hàng mời `lap_so` và `bat_doi` xuống hai dòng gọn; hàng ghim «Tờ giấy của hai mình» tuần yên và khi có đề nghị. |
| `03-chip-sang/toi` | Ba chip hai người: «Kèm 3 tin gần đây · Xem · Chỉ gửi lời nhờ», «Rủ Đi AI chưa sẵn sàng · Gửi như tin thường», «Hai bạn chưa có tin nào…»; không «nhóm». |
| `04-tra-loi-sang/toi` | Hàng đang đọc «Rủ Đi AI đang đọc 3 tin…» rồi hàng đang viết với chữ lớn dần; trích «Bạn». |
| `05-khay-dam-ban-390-sang/toi` | Khay đám bạn bốn công cụ một hàng, không có «Tờ giấy». |
| `06-khay-to-hen-hai-nguoi-sang/toi` | Bảng «Phác một tờ hẹn»: «… hiện cho cả hai bạn …», «Hỏi Rủ Đi AI», «Tự tạo kèo». |
| `07-khay-cap-doi-390-sang/toi` | Khay cặp đôi năm công cụ **vừa một hàng** ở 390 (mỗi ô 65 px, sát mép). |
| `08-chip-xem-sang/toi` | Tấm «Xem» nói «cả hai bạn», liệt kê đúng 3 tin — nhưng **neo sai chỗ** (lỗi 1 dưới, đã sửa ở `762d5c59`). |
| `09-sticker-dam-ban-sang/toi` | Tám sticker, không có mục «Cho hai người». |
| `10-sticker-cap-doi-sang/toi` | Tám sticker cộng mục «Cho hai người» bốn sticker; nhãn hai dòng không bị cắt. |
| `11-khay-cap-doi-360-sang/toi` | Ở 360 khay cặp đôi **xuống hàng 4 + 1**, «Tờ giấy» nằm một mình và nhãn bị cắt đáy (lỗi 2, đã sửa ở `762d5c59`). |
| `12-khay-dam-ban-360-sang/toi` | Khay đám bạn bốn công cụ vẫn một hàng ở 360. |
| `13-tra-loi-giam-chuyen-dong` | Reduce Motion: hàng đang viết chỉ hiện câu trọn «… một vòng hồ.». |
| `14-khay-cap-doi-giam-chuyen-dong` | Reduce Motion (390): khay cặp đôi giống `07`, không có gì phụ thuộc chuyển động. |

## 6. Lỗi tìm thấy ở thành phần sản phẩm (đã sửa ở `762d5c59`)

Mô tả gốc của từng lỗi giữ nguyên bên dưới; phần «Đã sửa» sau mỗi lỗi là số đo chụp lại trên bản web
dựng từ `762d5c59` (cùng cách ở mục 5, thêm 320x640 cho khay). Ảnh mới nằm ngoài repo, cạnh ảnh cũ, và
đã mở từng ảnh so với ảnh cũ. Chưa kiểm trên máy thật.

1. **Tấm «Xem» của chip bị neo vào chính chip, không vào màn.** `ChipBoiCanh.tsx:46` đặt `<Sheet>`
   bên trong `View` của chip (`styles.chip`, dòng 62); `Sheet` là `StyleSheet.absoluteFill` của cha gần
   nhất (`ui/Sheet.tsx:178`). Đo trên web 390x844: lớp phủ 356x40 đúng bằng chip, tấm kết thúc ở đáy chip
   (y 397) và mọc lên trên; không có lớp tối phủ màn, phần dưới chip vẫn bấm được. Trong màn chat, chip
   nằm ngay trên ô soạn nên tấm sẽ che luồng tin chứ không trượt lên từ đáy màn. Chưa kiểm trên máy thật.
   Maestro `_41-ai-hai-nguoi-co-khoa` chỉ chờ chữ, nên vẫn xanh với lỗi này. Tiêu chí gỡ: tấm phủ cả màn
   (ví dụ nâng `Sheet` ra gốc màn như `KhaySticker`), ảnh `08` chụp lại.
   **Đã sửa ở `762d5c59`.** Tấm tách thành `TamXemBoiCanh`; chip chỉ gọi `onXem`; `GroupChatLive` và
   trang lab gắn tấm ở gốc màn, cùng tầng `KhaySticker`. Ảnh `08` mới (sáng/tối): cả màn tối, tấm trượt
   lên từ đáy cửa sổ, rộng hết màn. Đo: lớp phủ 390x844 từ (0,0), tấm y 582–844, không nằm trong chip;
   điểm giữa màn ở y 60 thuộc lớp «Đóng» (chặn chạm); tiêu điểm vào «Đóng bảng», Tab ở lại trong tấm,
   phần ngoài `inert`; Escape đóng và trả tiêu điểm về «Xem»; chạm lớp tối cũng đóng.
2. **Khay cặp đôi ở màn 360 dp xuống hàng 4 + 1, «Tờ giấy» bị cắt.** `SoHen.tsx:233-234`: `flexWrap:
   "wrap"` với ô `minWidth: 62, flex: 1`; năm ô cần 5×62 + 4×8 = 342 px, màn 360 trừ lề 32 còn 328. Ô
   thứ năm giãn ra cả hàng (đo: «Tờ giấy» rộng 328 ở y 682, bốn ô kia ở y 573), và `ScrollView` của
   khay bị trần `min(260, 25% chiều cao)` (`SoHen.tsx:177`, 195 px ở 780) nên nhãn «Tờ giấy» bị cắt
   đáy, phải cuộn trong khay mới thấy. Ở 390 vừa đúng một hàng (chưa đo ở cỡ chữ lớn hơn; ô có `minWidth` cố định nên nhiều khả năng cũng xuống hàng).
   Tiêu chí gỡ: năm công cụ đọc trọn ở 360 và ở cỡ chữ 1.3, ảnh `11` chụp lại.
   **Đã sửa ở `762d5c59`.** Bố cục khay tính bằng hàm thuần `boCucKhay` (`chat/khay-cong-cu.ts`): một
   hàng khi mỗi ô còn rộng bằng chữ dài nhất (50 dp × cỡ chữ; «Sticker» đo 43.4 ở caption 13 px), không
   thì chia hàng cân (năm ô: 3 + 2, không bao giờ 4 + 1), ô bằng nhau; trần cuộn của khay công cụ
   (`tranKhay`) nâng lên bằng chiều cao lưới, tối đa 60% cửa sổ. Ảnh `11-khay-cap-doi-360` mới: năm ô
   một hàng, mỗi ô 59.2, ô vuông 56, «Bình chọn» xuống hai dòng dưới ô vuông, không nhãn nào bị cắt.
   `11-khay-cap-doi-320-sang` (mới): năm ô 51.2, ô vuông 51, đọc trọn. 390: năm ô 65.2 như trước. Khay
   đám bạn không đổi: 83.5 ở 390, 76 ở 360 (ảnh `05`, `12`). Cỡ chữ 1.3: web không đổi được
   `fontScale`, nên chỉ tính và giữ bằng test — 360 cho 3 + 2 ô rộng 104, lưới cao 268, trần mới 268
   (trần cũ 195 sẽ cắt). Chưa chụp ở 1.3.
3. Chữ đọc sai cho hai người (gợi ý, không chặn):
   - `SoHen.tsx:169`, dòng chú thích khay: «… bạn đồng hành chọn vào sổ chuyến đi công khai … nhắn
     người giữ sổ» — câu của nhóm/chuyến đi, hiện nguyên cho cả chat hai người (ảnh `05`, `07`, `11`).
   - `SoHen.tsx:40`, nhãn hành động của hàng tờ hẹn chung: «Sửa cùng hội» — `ToHen` không nhận
     `haiNguoi`, nên cặp đám bạn có tờ hẹn chung sẽ đọc «hội». Không có trong ảnh (lab không vẽ `ToHen`).
   - `GroupChatLive.tsx:780`, nhãn trợ năng nút cài đặt «Cài đặt nhóm» cũng dùng cho chat hai người
     (Maestro 47 bấm đúng nhãn này, nên đổi nhãn phải đổi flow cùng lúc).

   **Đã sửa ở `762d5c59`** (hai ý đầu). Câu chú thích áp cho cả hai người — sổ chuyến đi của một kèo
   trong phòng đọc ảnh của phòng đó, kể cả phòng hai người — nên không ẩn mà đổi chữ: hai người đọc «Ảnh
   gửi vào đây có thể được người kia chọn vào một sổ chuyến đi công khai. Nếu muốn gỡ, hãy nhắn người
   ấy nhé.» (ảnh `05`, `07`, `11` mới). `ToHen` nhận `haiNguoi`; hai người đọc «Sửa cùng nhau», nhóm
   giữ «Sửa cùng hội» (Maestro 48 và `chat-nhom.md` trích). Cả hai câu nằm trong `chuKhay`, test sẵn có
   cấm «hội»/«nhóm» trong mọi chữ khay của cặp. Ý thứ ba **cố ý không đổi**: nhãn «Cài đặt nhóm» do
   hướng dẫn và Maestro 47 trích.

## 7. Còn mở

- Chạy mục 3 trên máy thật, cả hai dạng (có và không `--ai`), và mở ảnh Maestro của 41/47.
- Hàm kiểm máy chủ sau flow 47 (cờ `cap_doi` thật sự `true` sau khi cả hai bật «Một đôi»).
- Hai lỗi ở mục 6 đã sửa ở `762d5c59` trên web; còn chụp khay ở cỡ chữ 1.3 và mở tấm «Xem» trên máy
  thật (Android Back, TalkBack).
