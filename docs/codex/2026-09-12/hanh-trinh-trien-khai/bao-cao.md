# Hành trình — triển khai và kiểm chứng

Đã triển khai trên nhánh `codex/hanh-trinh-ban-do-cua-hoi`, từ main
`2d0b97778146f2253c2657dd1a59853e508e531d`. Đây là bản triển khai để review,
chưa merge hoặc phát hành. Quyết định sản phẩm nằm trong
[ADR-0027](../../../decisions/ADR-0027-hanh-trinh-ban-do-cua-hoi.md).

## Hành vi đã có

- Lịch trình và bản đồ dùng chung ID, giờ, ngày và revision. Đổi chế độ giữ
  lựa chọn, bản nháp và hoàn tác. Tải lại thất bại giữ bản đang sửa; phản hồi
  tải lại cũ không thay thế revision mới hơn.
- Bản đồ có tên đường/địa danh thật, mốc đánh số, bộ chọn điểm gần nhau,
  mũi tên chiều đi, chọn chặng và khớp tuyến. Nét đứt chỉ thể hiện thứ tự khi
  chưa có đường bộ; không công bố km/ETA đường bộ từ khoảng cách chim bay.
- Trang ngày thu gọn được, cuộn riêng, nút chính luôn có chỗ. Màn rộng đặt
  trang bên cạnh bản đồ; chữ lớn giữ khả năng cuộn và đọc chi tiết.
- Chọn xe máy, ô tô hoặc đi bộ theo ngày; sửa giờ xuất phát, thời lượng,
  giờ ghim, ngày của chặng, điểm đầu/cuối và quay về điểm đầu.
- Điểm hẹn do người dùng giữ trên bản đồ rồi đặt tên và xác nhận; giới hạn
  50 chặng, không GPS và không đưa điểm riêng vào danh mục công khai.
- Xem trước hiện tại/gợi ý, nêu km/phút chênh lệch và giờ sẽ đổi. Áp dụng và
  hoàn tác có revision/idempotency; xung đột buộc đối chiếu bản của hội.
- Valhalla tự vận hành tính ma trận có hướng và tuyến thật của cả hai phương
  án. Không có routing công khai dự phòng hoặc lời hứa tối ưu tuyệt đối.

## Câu chuyện và chất lượng hình ảnh

Luồng chính là mở trang ngày → nhìn đường đi → xem điều gì sẽ đổi → giữ
phương án cho cả hội. Chất giấy–mực, màu mốc và ký hoạ dùng hệ hình ảnh hiện
có của Rủ Đi. Nếp ở trang trống là lớp nghệ thuật tháo được, không thay thế
thông tin đường đi và không biến thành điều kiện sử dụng tính năng.

Đánh giá của người triển khai sau khi nhìn và thao tác native: bản đồ rõ địa
lý hơn, có tương tác phục vụ quyết định và hợp với thế giới của app. Reviewer
hình ảnh độc lập trả `disposition: ship` cho bảy trạng thái đã xem, không có
material finding cần chặn. Reviewer ghi nhận khoảng trắng lớn của bản wide
là cơ hội hoàn thiện; không chứng nhận “stunning”, motion hoặc phát hành.
Đây cũng không phải bằng chứng người dùng thực đã hiểu câu chuyện Nếp.

## Bằng chứng thực thi

Tất cả chuyến đi, thành viên và điểm hẹn dùng kiểm chứng đều là dữ liệu tổng
hợp. Ảnh live dùng API local, PostgreSQL 16 và Valhalla với graph Việt Nam
2026-09-11; không phải hội thật hoặc production.

| Kiểm chứng | Kết quả và giới hạn |
|---|---|
| Mobile toàn bộ | `npm test`: 837 qua, 0 lỗi, 0 bỏ qua. Gồm export web, biên dịch test và Chrome thật. |
| Sau các sửa cuối | Typecheck và bài Chrome giữ lựa chọn/bản nháp chạy lại thành công; đổi nét vẽ và nhãn nút được kiểm native. |
| Journey + import boundary | 55 qua và 40 kiểm tra con; gồm graph thật Hà Nội/Đà Lạt cho ba phương tiện. |
| PostgreSQL, migration/race, outing, idempotency, model parity | 66 qua và 77 kiểm tra con trên PostgreSQL thật; schema thử riêng. |
| Provider bất thường | 22 ca qua, gồm số dương/âm cực lớn trên HTTP adapter thật; lỗi được chuẩn hoá. Đây là tập con của journey. |
| Áp dụng native | Từ A→B→C→D, 9,1 km/14 phút sang A→C→B→D, 5 km/9 phút; giờ C 11:00→08:32, B 10:00→09:08; A 08:00 và D 12:00 giữ nguyên. |
| Hoàn tác native | Sau apply revision 3, chuyển timeline rồi trở lại map, bấm hoàn tác: DB revision 4 khôi phục A→B→C→D và 08:00/10:00/11:00/12:00. |
| Xung đột + mất API | Sửa native giờ xuất phát 07:45 trên revision 4. Một PUT độc lập nâng DB lên 5; lưu bản cũ bị từ chối. Bỏ ADB reverse cổng API, bấm tải bản mới: lỗi mạng hiện, bản nháp 07:45 còn. Nối lại, tải được bản hội và bấm bỏ nháp có chủ ý. |
| Valhalla vận hành | Container được tạo lại bằng compose đã sửa và báo `healthy`; không publish cổng routing. |

Các số kiểm thử có phần giao nhau; không cộng thành một tổng coverage.
Lần chạy cuối sau sửa vòng đời popup cũng đạt 837/837. Test Chrome chọn
Chợ đêm trong cụm 1·3, đợi đúng chi tiết rồi đổi về timeline và xác nhận
lựa chọn được giữ. Lần chạy đồng thời trước đó đã bắt lỗi popup bị đóng
bởi `moveend` giữa nhấn/thả; bỏ thao tác đóng khỏi lần vẽ lại marker,
giữ các đường đóng khi chọn, bấm nền hoặc unmount. Reviewer độc lập duyệt
riêng delta này và xác minh lifecycle từ mã MapLibre đang cài.

## Thiết bị và nguồn ảnh

Máy kiểm chính: Android emulator `rudi-journey-audit`, serial `emulator-5562`,
ADB 5038; APK debug có MapLibre, bundle Metro cổng 8098, API cổng 8100.
Chỉ thao tác máy này ở vòng hoàn tất; `emulator-5554` thuộc phiên khác.

GPU `swiftshader` đã cho ảnh không có chữ bản đồ dù glyph và vector feature
có dữ liệu. Cùng APK và style, chuyển emulator sang `-gpu swangle` cho chữ
đường hiện lại. Đây là điều kiện môi trường đã quan sát, không phải sửa GPU
trên thiết bị thật. Tham khảo báo cáo upstream
[MapLibre Native #3648](https://github.com/maplibre/maplibre-native/issues/3648).
Emulator từng treo khi đổi cấu hình; đã khởi động riêng lại với 3 GB RAM.
Không dùng lần treo đó làm bằng chứng crash của app hoặc hiệu năng thiết bị.

Ảnh `01` là fixture mở nhầm lúc đầu. `02`–`07` là ảnh trung gian trước khi
đổi renderer; `07` chứng minh thứ tự sau apply, không chứng minh chữ bản đồ.
Ảnh `08` trở đi là bằng chứng native với renderer có glyph. `12` chụp trước
khi đổi nét nối thành nét đứt; `14` đã có nét đứt, trước khi rút gọn nhãn nút
đối chiếu cho font lớn. Không gọi toàn bộ ảnh trung gian là ảnh bản cuối.
Ảnh `16` được chụp lại sau khi reviewer phát hiện lần ghi trước đã bắt
nhầm Ngày 1 trong lúc chuyển màn. Digest cuối chỉ được ghi sau khi nhìn
trực tiếp đúng trạng thái và kiểm lại độc lập.

Xem [bộ ảnh](evidence/index.html) và [manifest](evidence/manifest.json) để đối
chiếu file, SHA-256 và trạng thái. Ảnh được chụp trực tiếp bằng
`adb exec-out screencap -p`, không chỉnh sửa hình.

## Review và giới hạn còn lại

Review độc lập đã tìm thấy: mất bản nháp khi đổi host/tải lại, race preview
với save, hiển thị tiết kiệm 0 thành 1 phút, thiếu giới hạn ghim và số âm cực
lớn làm tràn phép chuyển số. Các lỗi nêu trên đã được sửa; kiểm native và
adapter ở bảng trên đóng những ca tương ứng trong phạm vi đã thử.

Reviewer ban đầu hết quota trước verdict; sau đó đã thực hiện hai review
độc lập mới. `final_visual_review`: `disposition: ship` cho bảy capture đã
chỉ định. `final_engineering_review`: `APPROVE` phạm vi routing `_number`,
save/preview/reload, biến đổi nháp và ghi v2; reviewer tự chạy typecheck,
22 routing tests và 11 tests biến đổi/lịch trên dist-test hiện có. Không
biến các verdict có phạm vi này thành phê duyệt phát hành toàn bộ app.
Nhánh chưa merge hoặc deploy.
Chưa chứng minh iOS, máy Android vật lý, hiệu năng animation dưới tải thật,
thử nghiệm hiểu biết với người dùng, hoặc vận hành production nhiều worker.
Graph có giới hạn ngày và không có giao thông trực tiếp. Bản nháp/hoàn tác
trong bộ nhớ được giữ qua đổi chế độ và lỗi tải lại, chưa lưu qua kill app.

Giảm chuyển động được bật qua Android animator duration scale = 0 và thao
tác đổi ngày/trang trống hoạt động; chưa đo frame timing. TalkBack đã bind
và bật touch exploration, nhưng vòng thử chỉ xác nhận cây accessibility và
khởi tạo dịch vụ: không ghi nhận được một flow điều hướng/giọng đọc hoàn
chỉnh, nên chưa đóng gate screen reader. Đã phục hồi font 1.0, size/density
gốc, sáng, animator scale 1, TalkBack tắt và kết nối API trên emulator riêng.

Hướng dẫn dựng graph, cấu hình, feature flag và rollback:
[van-hanh.md](van-hanh.md).
