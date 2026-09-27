# Sổ kỷ niệm — triển khai và bằng chứng Android

Ngày kiểm tra: 2026-09-27. Toàn bộ tài khoản, hội, ảnh và lời kể dùng trong
các lượt dưới đây là dữ liệu giả lập. Không chép khóa Gemini vào tài liệu/log.

## Chạy lát sản phẩm

1. PostgreSQL đã có schema hiện hành; chạy `core migrate-diaries` trước khi
   bật API Go. Migration có checksum, khóa giao dịch; không tự chạy khi nhận request.
2. Go dùng `MOBILE_DATABASE_URL`, `MOBILE_MEDIA_ROOT`, upstream brain và internal
   token của môi trường. Worker diary chạy cùng `core serve`; hai worker, lease,
   tối đa ba lượt, tác vụ hết hạn sau một giờ.
3. Brain Python chỉ làm inference. Cấu hình `GEMINI_API_KEY` ở tiến trình brain;
   `GEMINI_DIARY_MODEL` là tùy chọn. Không truyền khóa vào Expo hay API công khai.
4. Build/cài Expo development client Android, chạy Metro với
   `EXPO_PUBLIC_API_URL` trỏ tới Go. Emulator dùng địa chỉ host `10.0.2.2`.
5. Các flow `.maestro/_diary*.yaml` là flow chuyên biệt, cần tài khoản đã đăng
   nhập và outing giả lập. Truyền `OUTING_ID`, `MOMENT_ID`; không chạy chung trên
   dữ liệu thật. Flow lỗi AI cần brain không có provider; flow AI thật cần provider.

## Hành vi đã quan sát trên native

Pixel 6, Android 15/API 35 x86_64, APK `com.lakiet.rudi`, Expo development
client; không dùng trình duyệt thay cho Android. Build native hoàn tất 754 task.

- Khép chuyến nhiều ngày, chọn ảnh, tự dựng, lưu riêng, mở từ tường, sửa và công khai.
- Một ngày gợi ý Khoảnh khắc; lưu lên tường theo dạng dòng thời gian.
- Người khác đọc được sổ và byte ảnh sau công khai; cả hai trả 404 sau thu hồi.
- Lỗi provider giữ nguyên bản đã lưu, cho quay lại sửa hoặc tự xếp trang.
- Gemini thật nhận ba ảnh đã chọn, trả bản ba trang; xem trước và lưu riêng
  qua Android. Lượt đầu có cuộn thủ công để sửa harness; lượt sau Maestro chạy
  hết luồng mà không can thiệp.
- Bộ chọn ảnh hệ thống Android chọn ảnh giả lập, upload thành ảnh thứ tư,
  tự xếp và lưu thành công.
- Sửa tên sổ qua bàn phím Android, lưu và đọc lại PostgreSQL revision 7.
- Hộp xóa: hủy giữ nguyên; xác nhận xóa làm bản ghi diary biến mất trong PostgreSQL.
- Light mặc định; dark với font scale 1.3 và giảm chuyển động. Đã mở ảnh chụp
  để phát hiện và sửa collage trắng, ký hiệu bị cắt, cuộn còn sót giữa các bước.

Maestro đôi khi báo `device offline` trong bước dọn driver sau lượt test.
Không coi file PNG rỗng là bằng chứng; chụp lại bằng adb và mở kiểm tra.
Ảnh nằm ngoài repo tại `/tmp/rudi-native-evidence/`, chỉ có dữ liệu giả lập.

## Cổng đã chạy

- Mobile: 1.097 test đạt; typecheck và export Android/iOS/web đạt. Export không
  thay bằng chứng APK ở trên.
- Diary Go: 12 test PostgreSQL thật và hai test domain đạt.
- Seam AI Python: 18 test đạt. Đây là test hợp đồng, tách biệt lượt Gemini thật.
- Gate tổng lượt đầu: 17 đạt, 5 hỏng, 5 bỏ qua. Sau sửa, chín chặng
  `ruff contract client-routes server-routes screens ownership go-vet go-test mobile`
  chạy lại đều đạt, không bỏ qua. Không cộng hai lượt thành một full gate xanh.
- Product/meta Python lượt tổng: 3.766 đạt, 802 bỏ qua; PostgreSQL: 721 đạt,
  QA archive: 88 đạt. Go PostgreSQL và chat E2E của lượt tổng đạt.
- Hai mutant không tương đương: mở ACL riêng tư cho người ngoài; bỏ kiểm tra
  photo ID do model trả. Cả hai đỏ đúng test dự đoán; identity xanh, cùng SHA256
  harness `4db4f7888a463c71a7beab6c2aad0437bb6c0d7957f4968bf1ceecb10e1cbf50`.
- Canary Android đổi nhãn mong đợi riêng tư thành công khai: identity exit 0,
  canary exit 1 đúng assertion; cùng SHA256 flow
  `df77bdb832d3228d0c31ff207f4baead97debe0f220b4f23b3382100c95bb24d`.

## Giới hạn bằng chứng

Chưa tuyên bố toàn bộ cổng ở clean tree tại SHA cuối đã xanh hay đã lên main.
Parity lượt đầu dừng ở fixture đòi tờ thứ tư sau đóng sổ, trái hạn mức ADR-0034.
Fixture được sửa để so phản hồi từ chối và đọc lại tờ cũ, không sửa Python oracle;
chạy riêng 36 bước khớp cả HTTP, DB và media. Các fixture done/skip/withdraw/responses
cũng cần chia lượt phác cho hai người để không vượt quota, giữ nguyên quan hệ
người gửi/người nhận và mọi bước đối chiếu. Ca done riêng 42 bước khớp.
Đối chiếu hợp của lượt dev chính và phần cuối: đủ 354 scenario khác nhau,
không có khác biệt; riêng phần cuối 27 scenario/962 bước đạt. Lane prod: 23 scenario/604 bước khớp HTTP, DB và media. Canary trên năm
fixture đã sửa: identity khớp, mọi phép phá dữ liệu được thực thi đều bị bắt;
các mutant không được năm fixture chạm tới không được tính là đạt. Chưa gọi
full parity gate xanh: chưa chạy lại toàn bộ canary/limiter trong cùng lượt.

Lời AI vẫn cần người dùng xem và sửa: ràng buộc schema/nguồn ảnh không chứng minh
mọi câu kể đều đúng. Lượt AI thật phát hiện suy diễn ngày/cảm xúc từ ảnh minh họa;
đã siết prompt và thêm lượt kiểm tra độc lập trên cùng gói nguồn (từ chối nếu không
đủ căn cứ hoặc verdict sai kiểu). Lượt Android sau nhập đoạn kể minh họa rõ ràng,
AI dựng hai trang về vẽ tranh tại nhà và lưu riêng revision 6; không còn gán
chuyến thăm núi/ngày đi cho ảnh minh họa. Chưa có evaluation corpus chứng minh
loại hết lỗi ngữ nghĩa; hai lượt model không thay kiểm chứng độc lập.
Chat chỉ nhận đoạn người dùng chủ động dán; chưa có trình chọn trích đoạn E2EE
ngay trong chat. Chưa có bằng chứng native iOS hay đo hiệu năng trên máy cấu hình thấp.
Copy đã chỉnh các cửa vào chính, quan hệ, ending, nguồn, trình đọc và tường;
chưa được coi là kiểm duyệt hết mọi màn/layer trong ứng dụng.

## Kiểm tra tách khỏi cây làm việc

Snapshot review `ad55da3a6a5ffd4bd56131a333c69f72ad92c309`, không cập nhật main:
repo guard staged 60 file và tracked tree 3.774 file đạt; không đưa thay đổi
CLAUDE.md của người dùng vào snapshot. Chín chặng kể trên chạy lại đạt trong
worktree sạch; diary PostgreSQL/domain và 17 test AI cũng đạt. Bốn sửa fixture
parity bổ sung sau snapshot này cần được phân biệt với mã sản phẩm đã kiểm.

Impeccable finish review đã đóng ba finding trong phạm vi ảnh được xem:
bỏ eyebrow thừa, kiểm tra lại lời AI ở ca minh họa, đồng bộ PRODUCT/DESIGN.
Không suy rộng verdict đó thành duyệt mọi màn trong app.

Đo thử HWUI trên development client, emulator SwiftShader trong lúc chạy
scroll/bàn phím và các gate: 881 frame, 365 janky (41,43%), p95 44ms. Đây là
một phép đo có tải nền/GPU phần mềm, **không phải bằng chứng đạt độ mượt**.
Video 19,28 giây lưu ngoài repo tại `/tmp/rudi-native-evidence/diary-motion.mp4`.
Cần đo bản release trên thiết bị đích để chốt hiệu năng; không thay kết quả này
bằng test timing xanh.


## Lượt kiểm chứng Android release bổ sung

APK local test release đã cài trên cùng emulator, package không có cờ DEBUGGABLE;
không phụ thuộc Metro. SHA256 APK:
`e829d38337865fd49ba381e5381fb903b8d911308c0526f1b3283f9295208a98`.
Artifact: `/tmp/rudi-native-evidence/rudi-diary-release-test.apk`. APK này chỉ
cho phép HTTP tới địa chỉ backend emulator `10.0.2.2`; cấu hình native tạm đã
được hoàn nguyên sau build. Không coi đây là APK phát hành production.

Lượt AI trên release ban đầu bị kiểm tra ngữ nghĩa từ chối; sổ cũ không đổi.
Đã thêm tối đa một lần viết lại và kiểm tra lại, tối đa bốn lời gọi model,
mỗi lời gọi timeout 12 giây, không retry SDK, nằm trong ngân sách Go 60 giây.
Verdict sai kiểu hoặc lần viết lại vẫn thiếu căn cứ đều thất bại đóng.
Lượt release sau đó hoàn thành toàn bộ assertion Maestro: dựng, xem trước,
lưu riêng; PostgreSQL revision 10 ghi đúng trang về ba tranh minh họa vẽ ở nhà.
Maestro exit 0 nhưng driver vẫn báo offline lúc dọn dẹp; không che lỗi môi trường.
Ảnh đã mở kiểm tra: `release-ai-current.png`, `release-ai-page.png` trong thư mục
bằng chứng ngoài repo. Backend nhận khóa từ `.env` sẵn có, không đưa khóa vào APK.

Ca đổi bìa và đảo trang đã đối chiếu revision 9 trong PostgreSQL, không chỉ nhãn
UI. Đã sửa nhãn trợ năng thành số trang cụ thể để harness không chạm nhầm
TextView con của nút trang đầu bị vô hiệu. Chọn bìa ghi rõ một ảnh, khác bốn ảnh
cho một trang. Flow tái chạy nằm tại `apps/mobile/.maestro/_diary-edit.yaml`.

Migration diary v3 xóa sổ, phiên bản, liên kết ảnh và tác vụ chứa nguồn AI khi
`people.deleted_at` chuyển từ NULL; áp dụng cả tài khoản đã xóa trước migration.
Test PostgreSQL thật xác nhận không còn dữ liệu đó, người ngoài nhận 404,
phiên đăng nhập của tài khoản đã xóa không thể ghi sổ lại (401).

Đo cùng chuỗi 12 lần vuốt trang đọc trên emulator: development 266 frame,
3 janky (1,13%), legacy janky 18,05%, p95 30ms; release 305 frame, 2 janky
(0,66%), **legacy janky 77,38%**, p95 31ms. Hai cách đếm không đồng nhất;
không lấy riêng con số 0,66% để khẳng định 60fps. Chưa có phép đo máy thật.


Rà soát copy bổ sung: inventory 532 dòng cùng nhánh chữ động/component. Sửa
lời đồng ý cặp đôi để không hứa cấp quyền đọc chat; lời đề nghị đọc chat cũ chỉ
còn nhãn lịch sử (màn sổ không có hành động đồng ý `doc_chat`). Sửa mô tả ảnh
nhóm/sổ công khai và việc xóa sổ, phạm vi bài bạn bè, xem trước phần gửi Nếp,
những điều cần tránh, câu giữ kỷ niệm và thông báo lỗi mặc định. Nhãn số nháp
bỏ từ “confirm”. Đây là audit tĩnh, không phải đã đi qua tất cả nhánh UI native.

Snapshot sản phẩm sau cùng được kiểm sạch:
`71091b64915895b87d1d3cea9c223f588f764b18`, không cập nhật main. Guard staged
74 file, tracked tree 3.776 file đạt; worktree không có thay đổi sau test.
Tám cổng ruff/contract/client-routes/server-routes/screens/ownership/go-vet/go-test
đạt; cổng mobile đạt (1.097 test, typecheck, export ba nền tảng); 12 test diary
PostgreSQL, hai domain, 18 AI đạt. Hai mutant đã chạy lại với harness hash nêu
trên. Đây là các cổng liên quan, không phải full strict gate toàn repo.
Phần ghi bằng chứng này được thêm sau snapshot, không đổi mã sản phẩm.

APK cập nhật copy `/tmp/rudi-native-evidence/rudi-diary-release-copy.apk`, SHA256
`eb94bfcfad331e861e6a8023e2c44e69cb3cfa28173c5db85637b03327cd7cf9`:
build release 910 task, 31 chạy; đã cài và chạy native dark/font 130%. Flow
`/tmp/rudi-native-copy-final.log` qua giới thiệu quyền ảnh → tìm tài khoản giả
lập → gửi lời mời → xem câu xác nhận → chọn người đọc bài → mở lại diary đã lưu.
Lượt đầu harness đòi câu xác nhận ngay khi mở trang tìm bạn nên đỏ; đã đưa flow
qua đúng thao tác, không thay mã sản phẩm để làm xanh. Parent mở ảnh cả ba màn.

Đã mở trên native khay Hội bạn/Cặp đôi và khay xin đồng ý: nhãn Hội bạn được
chọn khi chưa có đồng ý hai bên; câu không cấp quyền AI đọc chat hiển thị đủ ở
cỡ chữ 130%. Fixture chu kỳ sổ là dữ liệu giả lập seed trong PostgreSQL; không
coi đó là chứng minh thao tác đồng ý cả hai người qua hai thiết bị. Test quyền
cặp đôi của Go dùng consent thật và constraint PostgreSQL.

Khay “Những điều cần tránh” đã mở bằng thao tác ADB sau khi Maestro lặp lỗi
mất thiết bị tại bước tap; parent xem ảnh `copy-constraints-settled.png`, chữ,
ô nhập và nút nằm đủ trong màn 130%. Không tính các flow Maestro bị ngắt này
là đạt. Câu đồng ý cặp đôi đã qua assertion sau sửa selector có dấu đầu dòng.
