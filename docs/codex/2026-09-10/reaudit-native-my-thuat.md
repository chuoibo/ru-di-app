Method: dual-agent (A: /root/design_assessment · B: /root/technical_assessment), kèm kiểm tra native và kết luận trực tiếp của Codex chính.

# Tái audit Rủ Đi ngày 10/09/2026 — đã có bản sắc, chưa stunning xuyên suốt

**Phán quyết: REQUEST_CHANGES cho cổng mở rộng mỹ thuật và đóng §5 motion.** Các sửa F41, F43, F44 có bằng chứng mới đáng tin trong phạm vi Android đã thử. F42 có bằng chứng hook tốt hơn trước. Không đánh đồng phán quyết này với việc mọi PR sửa lỗi đều sai, hay phải làm lại toàn bộ giao diện.

Rủ Đi đã đẹp hơn và đã có ngôn ngữ riêng: giấy ngà, mực đậm, góc gấp coral, Nếp và những vật thể của một chuyến đi chung. Nhưng câu chuyện hiện nổi rõ ở bìa, Album và một số cảnh trống; khi bước vào Khám phá, chat, cài đặt, nó mỏng đi. **Độ hoàn thiện kỹ thuật của lớp hình đã tiến trước độ thuyết phục về nghĩa và cảm xúc.** Chưa nên lấy “10 pose khác nhau” hoặc reviewer đọc đúng bốn nhãn làm bằng chứng toàn bộ sản phẩm đã tinh tế.

## 1. Phạm vi và nguồn bằng chứng

- Checkout được kiểm: `main`, SHA `0c1a41698148268412eb55a4303d34537b54bfa9`, bằng local tracking `origin/main`; commit đã chứa merge #596. Không fetch GitHub trong lượt này nên không tuyên bố đây là HEAD remote ở mọi thời điểm sau đó.
- Máy ảo Android 15, `emulator-5554`, 1080×2400 px, 420dpi, khoảng 411×914dp; APK `com.lakiet.rudi` phiên bản 1.0.0, cờ `DEBUGGABLE`.
- Mở dev client thật bằng Metro cục bộ, `EXPO_NO_DOTENV=1`, fixture tổng hợp, fingerprint `codex-reaudit-0c1a4169`. Không mở backend/e2e stack. Endpoint thử lỗi là cổng loopback không có dịch vụ; đây là lỗi mạng thật trong renderer native, không phải dữ liệu chat live.
- Ảnh mới và XML ở [reaudit-evidence](reaudit-evidence/); [gallery](reaudit-evidence/index.html) để đọc hình, [manifest](reaudit-evidence/manifest.json) để kiểm hash/phân loại. PNG chụp trước XML khoảng một giây; chỉ dùng cặp khi trạng thái ổn định. Ảnh menu dev, chuyển cảnh và lần cuộn hụt có ghi rõ giới hạn.
- Codex chính trực tiếp điều khiển, nhìn ảnh native, xem khung trích từ video và đối chiếu XML. Assessment A đưa nhận xét mỹ thuật độc lập ở vòng hình ban đầu; phần mở rộng ảnh native do Codex chính chịu trách nhiệm. B kiểm nguồn/cổng độc lập, không điều khiển máy ảo và không đọc A. [Bản B đầy đủ](assessment-b-ky-thuat.md); log đã sao chép vào `reaudit-evidence/technical/` để không phụ thuộc `/tmp`.
- B chạy lại 66 ca trong 9 file, gồm 7 ca hook React thật; biên dịch test và sửa đường import ESM thành công. Không chạy lại toàn bộ 790 npm hoặc 3433 pytest rồi nhận đó là bằng chứng hôm nay.
- Detector chạy một lần: exit 0, `[]`, không rule/địa chỉ finding. Không có browser overlay. Quét này không kiểm nghĩa hình, không đo màn native, không xác nhận accessibility.

**Chưa chứng minh:** release/live qua HTTPS, F42 native đổi nhóm khi request đang bay, chat live hai bên, iOS, máy thật, tablet/màn rộng, màn compact hẹp hơn máy đã thử, TalkBack/VoiceOver, IME tiếng Việt, thời gian phản hồi đầu-cuối, pin/nhiệt, mọi nhánh lỗi/tất cả route. Không dùng chữ “toàn app đạt” cho tập mẫu này. Không đo lại tương phản pixel thành tỷ lệ mới; tỷ lệ team báo không được ghi thành phép đo của Codex.

## 2. Nhìn từng phần: đẹp ở đâu, câu chuyện đứt ở đâu

| Phần / ảnh mới | Nhận xét trực tiếp | Quyết định mỹ thuật |
|---|---|---|
| Welcome — `02-welcome` | Bìa indigo, chất giấy, chữ lớn và điểm coral tạo cảm giác một cuốn sổ đã được thiết kế. Mở đầu có khí chất, CTA rõ. | Giữ hướng này. Đừng tăng chi tiết chỉ để gây ấn tượng thêm. |
| Sở thích — `04-preferences`, `36-preferences-dark13` | Icon riêng và nhóm lựa chọn nối được với chất mực. Tuy nhiên nhiều ô cùng trọng lượng, lời dẫn và lựa chọn tạo cảm giác điền bước thiết lập. | Có bản sắc, chưa phải đỉnh cảm xúc; phân nhóm và rút lời dẫn trùng ý trước khi thêm Nếp. |
| Khám phá — `05-explore`, `06-explore-lower`, `37`, `43`, `52` | Đã có nhịp một địa điểm dẫn, hai địa điểm so sánh và các hàng sau. Nhưng ba địa điểm đầu dùng cùng hình bát nhỏ; chữ làm gần hết công việc. “Gần bạn, đúng gu” → dấu “HỢP GU” → “Hợp gu nhờ Chill và View đẹp” → mô tả tiếp tục “view… chill”. Mắt phải đọc lại cùng một lời hứa. Ở chữ 2.0, riêng khối dẫn chiếm gần một màn. | Đây là nút thắt lớn nhất để đạt stunning. Ba `anh:null` là fixture cố ý, không phải ảnh mạng tải hỏng. Cần làm trạng thái không ảnh có bố cục thuyết phục; không giả ảnh địa điểm thật. |
| Chi tiết địa điểm — `visual-normal-detail` | Tên, dữ kiện, lý do hợp nhóm và CTA có thứ tự rõ. Khi không ảnh, phía đầu vẫn chỉ là một icon bát nhỏ và khoảng trống; chưa mang không khí của địa điểm. | Tiếp tục cùng hướng Khám phá. Để một lý do cụ thể về nhóm làm trọng tâm, tránh nhắc lại các tính từ ở nhiều tầng. |
| Lên plan / lịch trình — `07-plan`, `13-timeline` | Ảnh mở đầu, ngày đi, người cùng đi, ngân sách, dấu đếm ngày và timeline tạo được một chuyến đi có hình dạng. Đây là một trong những màn mạnh nhất. | Giữ phân cấp hiện tại. Các dấu/vật thể phải phục vụ mốc chuyến đi, không thành một bộ tem để rải đều. |
| Tin nhắn — `08-messages` | Bong bóng thường dễ đọc, coral tạo giọng ấm. Khối AI dài và nặng làm gián đoạn nhịp hội thoại. Chất riêng hiện dựa nhiều vào màu và sticker. | Rút phần tóm tắt AI về lựa chọn có thể hành động; giữ thông tin cần thiết ở lớp mở thêm. Không biến mọi tin thành thẻ minh họa. |
| Khay sticker / bubble — `30–33`, `38`, `44` | Nhận ra cùng một nhân vật. Đi thôi, ăn, cà phê, vui mừng có động tác dễ bắt. Khay thích nghi 4→3→2 cột; chữ lớn vẫn chọn được. “Kẹt xe” và “Trả tiền nè” còn cần nhãn cứu nghĩa. | Giữ nét và nhân vật; sửa đạo cụ và động tác ở đúng 64dp/120dp trước khi thêm sticker. |
| Album — `12-album`, `54-album-dark2` | Hình lớn trong tờ ảnh, góc gấp coral, tiêu đề “Những ngày mình đi cùng nhau” và dải khoảnh khắc làm câu chuyện giấy–kỷ niệm trở nên có lý. Đây là màn gần “stunning” nhất trong tập đã xem. | Giữ cấu trúc. Ảnh fixture nhiều nhóm người/khung cảnh khác nhau chưa tạo cảm giác cùng một chuyến đi; cần biên tập bộ ảnh demo nhất quán và giữ nguồn gốc ảnh rõ ràng. |
| Hóa đơn — `14-bill` | Tờ hóa đơn trên mặt bàn nối tự nhiên với chất giấy, đúng vật thể của việc chia tiền. | Giữ sự mộc và độ đọc được. Đây là mẫu tổng hợp, không chứng minh OCR ảnh hóa đơn thật. |
| Gán phần / đối soát / sổ tiền — `15-assignment`, `16-settlement`, `10-finance` | Hàng, con số và teal tạo cảm giác bình tĩnh; ít tranh giành chú ý hơn Khám phá. Đúng chỗ để giao diện nhường cho độ rõ nghĩa. | Không cần ép các màn tiền thành màn trình diễn. Lời phân biệt dữ liệu demo, nháp và sổ thật phải đúng; có thể tổ chức gọn, không xóa sự thật để đẹp hơn. |
| Bình chọn — `17-vote` | Dễ quét, nhưng các phương án khá đồng hạng; biểu tượng gamepad cho Puppy Farm không góp thêm nghĩa cụ thể. | Đủ chức năng ở trạng thái mẫu, thiếu chất địa điểm. Không ghi nhận icon này là lỗi mới của ba PR art. |
| Tạo mới — `11-create`, `53-create-dark2` | Bốn lựa chọn, nhãn rõ, sheet gọn; chữ 2.0 tối vẫn đọc và chứa đủ hành động. | Đây là phần tinh tế theo nghĩa vừa đủ. Giữ gọn. |
| Tạo chuyến — `18-new-outing` | Form và roster thực dụng, rõ việc nhưng khá thông thường. | Chưa có cơ sở yêu cầu thêm art; ưu tiên nhịp nhập/chọn và tránh lặp hướng dẫn. |
| Cá nhân — `09-profile` | Thứ tự danh tính → chuyến sắp đi → các hàng tác vụ khá bình tĩnh. | Khá tốt; không cần đưa Nếp vào mọi hàng. |
| Cài đặt — `19-settings` | Các thẻ trắng nổi, bo và bóng nhiều hơn hệ sổ/ledger ở màn bên cạnh. Cảm giác trở lại một bộ UI dùng chung. | P3: giảm lớp thẻ/bóng, dùng khoảng cách và đường phân nhóm của hệ đã có. |
| Mất mạng — `20-error`, `50-error-dark2` | Hình giấy rách, coral ở mép rách đọc hợp tình huống; câu lỗi không lộ URL, nút thử lại rõ. | Đây là sửa F45 thành công nhất. Chữ nhỏ còn một dòng cụt “lại.” ở ảnh sáng, chỉ là polish. |
| Cảnh trống — `22-scenes-top`, `23-scenes-bottom` | Ghế chừa chỗ và kẹp ảnh trống gắn được vào lời hứa “Chừa một chỗ cho nhau”. Cảnh bộ lọc lại là lưới khá mở cộng một vòng hở coral, giống đang tải/tìm. | Giữ những cảnh có quan hệ vật thể rõ. Cảnh bộ lọc cần thay ý, không chỉ đổi pose. |

**Điểm tinh tế đã có:** nét viền cùng họ, coral không phủ hết mọi vật, Album dùng góc gấp đúng nơi một tờ ảnh có thể gấp, tiền dùng teal để lắng xuống, sheet bốn hành động vừa đủ.

**Điểm chưa tinh tế:** nhắc lại thông điệp “hợp gu”; nghĩa đạo cụ nhỏ chưa tự đứng được; chỗ nào cũng cùng nền giấy nhưng phân cấp bề mặt giữa cài đặt và ledger chưa thống nhất. Ở tối, nhiều cảnh mất phần cảm giác giấy và nổi thành sơ đồ nét; cần kiểm tương quan mặt giấy–nền–mực theo kích thước thật, không chỉ đảo màu palette.

**Câu chuyện toàn hành trình:** bìa hứa một chuyến đi chung → Khám phá hiện đưa người xem về danh mục nhiều chữ → Lên plan lấy lại cảm giác có nhóm và có ngày đi → Album kết thúc ấm áp. Đỉnh và đoạn kết đã có. Phần tìm nơi đi và trao đổi để chốt chưa nối được hai đầu ấy. Muốn nâng hạng, sửa chính đoạn giữa này; tăng số minh họa chưa giải quyết được.

## 3. F41–F46 sau phép đo mới

| Finding | Phán quyết mới | Bằng chứng và điều chưa chứng minh |
|---|---|---|
| F41 nút lỗi tràn | **Đóng trong ma trận Android đã thử.** | 6/6 cấu hình 1.0/1.3/2.0 × sáng/tối: đủ 3 nút và 3 node chữ, bounds nằm trong màn. “Bỏ” nhỏ nhất 126px = 48dp ở mật độ 2.625; không còn 47,2dp. Ảnh `26,34,39,41,45,49`. Lab dùng callback no-op, nên đây là bằng chứng bố cục, không phải retry gửi thật. |
| F42 phản hồi muộn nhầm nhóm | **Đóng ở hook; native live còn chưa chứng minh.** | 7 ca hook React thật qua; probe độc lập cũ giữ `B-existing`, read-mark chỉ A,B, không tái hiện. Probe exit 1 vì nó kỳ vọng lỗi lịch sử còn tồn tại. Chưa thực hiện chuyển nhóm native giữa hai request live. |
| F43 lỗi in URL | **Đóng đối với các đường sửa đã kiểm.** | Test trạng thái/quét src qua; cố ý endpoint không có dịch vụ, màn lỗi native `20,50` không in http/URL. Không suy rộng sang mọi lỗi backend tương lai. |
| F44 placeholder cắt | **Đóng trong ma trận Android đã thử.** | 6/6 cấu hình, hai hint một dòng, chiều cao 63/70/95px. `51-field-focused-dark2` đã nhập `Hue`, thấy chữ/caret/keyboard; không chỉ chụp một placeholder tĩnh. Chưa TalkBack/iOS/IME tiếng Việt. |
| F45 nghĩa hình | **Chỉ đóng một phần, chưa ký ship toàn bộ.** | Giấy rách rõ hơn; các động tác khác biệt hơn. Xe buýt vẫn giống vật đứng/điện thoại, các tờ xòe giống giấy/vé, cảnh gọi lời chưa rõ ở nhỏ. 9 pose có Nếp + null ở cảnh lỗi không phải phép thử hiểu 10 nghĩa. |
| F46 DESIGN drift | **Các đoạn drift được báo đã sửa.** | B đối chiếu các đoạn lịch sử/hiện tại và luật F41–45. Không có rà lại từng câu toàn DESIGN.md; tài liệu motion vẫn có sai lệch riêng. |

Đọc [matrix-results.json](reaudit-evidence/matrix-results.json) để xem từng bounds. Cổng bounds lịch sử chạy lại trên XML lỗi cũ vẫn đỏ đúng. Không dùng kết quả đỏ đó để nói UI mới còn tràn.

## 4. §5 motion: số mới và hai lý do chưa đóng

Lượt mới setup fixture trước, warm từng chuỗi, chụp màn đầu trước cửa sổ đo, **reset gfxinfo khi tiến trình đang sống**, chạy thao tác, lấy dump ngay sau và xác nhận PID không đổi, tổng khung >0. Không đưa onboard/logout vào cửa sổ này. Mỗi chuỗi chỉ có một lượt thường và một lượt giảm chuyển động: baseline hữu hạn, không phải benchmark nhiều lần hay ngưỡng release.

| Chuỗi | Thường: khung / janky | p50 / p90 / p99 (ms) | Giảm chuyển động: khung / janky | p50 / p90 / p99 (ms) |
|---|---|---|---|---|
| Đổi 4 tab ×5 vòng | 267 / 29 (10,86%) | 25 / 29 / 32 | 40 / 33 (82,50%) | 28 / 32 / 32 |
| Cuộn 4 vòng | 848 / 23 (2,71%) | 16 / 22 / 30 | 721 / 27 (3,74%) | 16 / 23 / 26 |
| Mở/đóng sheet ×5 | 317 / 16 (5,05%) | 16 / 28 / 31 | 264 / 19 (7,20%) | 16 / 28 / 32 |
| Mở/back chi tiết ×5 | 280 / 15 (5,36%) | 26 / 31 / 32 | 280 / 17 (6,07%) | 28 / 32 / 34 |

Không diễn giải 82,50% thành “Reduce Motion chậm gấp tám”: số khung render của đổi tab giảm 267→40, mẫu số khác. Không so phần trăm mới với 6–13% cũ như một mức cải thiện; workload, pacing và cửa sổ khác. `gfxinfo` cũng không đo được mọi chiều của cảm giác tương tác, độ trễ touch hoặc chuyển động nhạy cảm.

### R1 — P1: chi tiết vẫn trượt ngang khi giảm chuyển động

**Quan sát native mới:** đặt và đọc lại cả `window_animation_scale`, `transition_animation_scale`, `animator_duration_scale` bằng 0. Từ Khám phá mở Tiệm Nướng Xóm Lèo vẫn thấy màn mới trượt từ phải sang trái, đẩy màn cũ khỏi khung.

- [Read-back cấu hình](reaudit-evidence/visual-reduce-confirm-settings.json).
- [Video xác nhận](reaudit-evidence/visual-reduce-confirm.mp4).
- [Khung trước](reaudit-evidence/motion-frames/reduce-confirm-03.png), [đang trượt](reaudit-evidence/motion-frames/reduce-confirm-06.png), [đã vào](reaudit-evidence/motion-frames/reduce-confirm-09.png).
- Nguồn tương ứng: `apps/mobile/app/_layout.tsx:155` đặt `animation: "slide_from_right"` cố định. Đây là đường sửa cần kiểm, chưa phải chứng minh nó là nguyên nhân duy nhất trong thư viện native.

**Hệ quả:** ý muốn giảm chuyển động chưa được thực thi trọn cho điều hướng. Không thể ký §5 chỉ từ việc các chuỗi rc=0 hoặc unit duration trả 0.

**Điều kiện gỡ:** nối lựa chọn giảm chuyển động tới cấu hình stack; quay lại cùng luồng với read-back 0, không còn dịch chuyển toàn màn. Thử cả mở/back, thay đổi setting trong phiên, và các modal cùng họ; giữ lượt thường riêng. Không thêm animation mới trong bước sửa này. Không dùng các khung video nén để đánh giá độ sắc nét typography; chúng chỉ chứng minh vị trí màn theo thời gian.

### R2 — P1 đối với cổng nghiệm thu: script motion có thể xanh khi không đo được

`docs/claude/2026-09-10/motion/do-motion.sh` force-stop/reset trước helper còn launch/onboard/logout. README lại nói warm xong mới reset. B đã chạy canary riêng, không chạm máy ảo: cả bốn Maestro cố ý exit 42, số khung là `?`, nhưng wrapper vẫn **exit 0** vì chỉ ghi rc vào bảng rồi kết thúc bằng `cat`.

**Hệ quả:** con số cũ không cô lập thao tác như nhãn bảng nói, và exit 0 của wrapper không đủ làm cổng CI. Đây không phải khẳng định các lượt native cũ đều thất bại; là một đường thất bại thật mà cổng chưa chặn.

**Điều kiện gỡ:** setup/warm ngoài cửa sổ, xác nhận màn/PID đầu-cuối, fail khi bất kỳ flow/dump lỗi hoặc khung rỗng, canary phải exit khác 0. Lưu cấu hình gốc, restore bằng trap khi thành công/lỗi/ngắt. Log canary: `reaudit-evidence/technical/runner-canary/`.

**P2 đi kèm về diễn giải:** p99 150ms không phải trần histogram; dump cũ có bucket 200/250ms và các bucket tới 4950ms, một dòng p99 là 117ms. Dev overhead là khả năng ảnh hưởng, không bảo đảm release “ít nhất tốt bằng”. Helper live cũng chưa xử lý đúng điểm vào trực tiếp màn “Chào bạn”. Chi tiết và vị trí nguồn trong bản B.

## 5. Ưu tiên mỹ thuật cần gửi lại team

| Mục | Mức / hệ quả | Hướng sửa cụ thể | Điều kiện chốt |
|---|---|---|---|
| R3: Khám phá nhiều lời lặp | P2, ưu tiên cao cho mục tiêu stunning. Mất nhịp chọn nhanh, đặc biệt chữ 2.0. | Mỗi địa điểm một lý do hợp nhóm; chọn một cấp nhấn “hợp gu”; loại trùng Chill/View giữa lý do và mô tả. Biên tập trạng thái không ảnh bằng phân cấp, khoảng cách và dấu loại địa điểm có chủ ý. | Đặt cạnh `05/43/52` ở 1.0/2.0 sáng/tối: mắt tìm được tên–lý do–giá mà không đọc lại cùng lời hứa; không che dữ kiện hoặc giảm font hệ thống. |
| F45 còn mở: Kẹt xe / Trả tiền / gọi lời | P2. Phải dựa vào nhãn để hiểu, nên lời hứa “một hình thay một câu” chưa trọn. | Xe buýt có silhouette phương tiện rõ và quan hệ người–xe rõ; tờ tiền có một dấu nhận biết tiền ở nhỏ; tách đuôi bong bóng khỏi đầu/tay Nếp để thấy hành động gọi. | Render 64dp, 120dp sáng/tối; người chưa đọc brief mô tả tự do trước khi xem nhãn. Ghi đúng câu trả lời, không bịa tỷ lệ. Codex đã biết brief nên không tự xưng thử mù. |
| R4: bộ lọc che hết | P2. Lưới mở và vòng hở gợi loading/tìm kiếm hơn là kết quả bị lọc hết. | **Đề nghị bỏ vòng hở.** Dùng phần bị che có diện tích rõ và một ô hở; hoặc hình thao tác nới bộ lọc, giữ giới hạn chi tiết ở nhỏ. | Ảnh không nhãn có thể phân biệt với đang tải và lỗi mạng; hình thực sự diễn đạt “che gần kín”, không phải caption giải hộ. |
| R5: nhịp chat AI và bề mặt cài đặt | P3 về độ tinh gọn trong mẫu đã xem. Chất giọng riêng yếu dần ở màn thao tác. | Chat: ưu tiên quyết định kế tiếp và mở thêm phần giải thích. Cài đặt: giảm các lớp thẻ/bóng, dùng nhóm hàng của hệ giấy–mực. | So cạnh chat/ledger/profile, vẫn nhận ra cùng một sản phẩm ở sáng/tối; không làm mất thông tin trạng thái hay giúp đỡ. |

Với ba quyết định team nêu: chọn **HTTPS cho phép đo release tương đương** khi triển khai bước đó; không thay manifest/cleartext trong audit này. Thử nghĩa không nhãn với người chưa đọc brief vẫn cần con người. Về vòng hở ở bộ lọc, nhận xét mỹ thuật của Codex đủ rõ để đề nghị bỏ, không cần để nguyên chờ một lựa chọn trung lập.

## 6. Điểm heuristic trong đúng tập màn đã xem

Đây là chấm định tính của reviewer, không phải điểm usability từ người dùng. Không lấy tổng điểm làm release gate; R1/R2 vẫn cần gỡ riêng.

| Heuristic | /4 | Căn cứ |
|---|---:|---|
| Trạng thái hệ thống | 3 | Demo/đang chờ/lỗi có nhãn; chưa kiểm transport live. |
| Khớp ngôn ngữ đời thường | 3 | Giấy, ghế, ảnh, sổ tiền hợp sản phẩm; một số đạo cụ còn mơ hồ. |
| Quyền chủ động và thoát | 2 | Back/sheet hoạt động trong mẫu; Reduce Motion chưa kiểm soát được stack. |
| Nhất quán | 3 | Chữ, mực, palette có họ; cài đặt khác phân cấp bề mặt. |
| Ngăn lỗi | 3 | Roster/trạng thái disabled và guard hook có bằng chứng; chưa E2E live. |
| Nhận ra thay vì nhớ | 3 | Tab có nhãn, CTA rõ; sticker còn cần caption. |
| Hiệu quả thao tác | 2 | Lựa chọn nhanh có; Khám phá và AI còn nhiều lượt đọc thừa. |
| Thẩm mỹ và tối giản | 2 | Album/plan tốt; Khám phá lặp prose, bề mặt chưa đồng đều. |
| Nhận biết và phục hồi lỗi | 3 | Câu lỗi sạch, nút đủ bounds; retry transport chưa được thử. |
| Trợ giúp đúng chỗ | 2 | Có hướng dẫn, đôi chỗ dài; chưa audit luồng hỗ trợ hoàn chỉnh. |
| Tổng | **26/40** | Còn cải thiện đáng kể; không phải phần trăm app đã đạt. |

Tải nhận thức: Khám phá có nhiều tầng chọn/search/filter/AI và nhiều lý do lặp; Sở thích và khay sticker cùng lúc hiện 8 lựa chọn, nhưng icon và nhóm giúp quét nên không tự động kết luận “quá 4 là sai”. Sheet tạo mới 4 hành động có phân nhóm tốt. Với người dùng đang vội, điểm đỏ là thời gian đọc Khám phá; với người mới, điểm đỏ là nghĩa sticker; với người phụ thuộc trợ năng, F41/F44 đã khá hơn nhưng R1 là lỗi thật, còn TalkBack chưa đo.

## 7. Bàn giao và giới hạn thao tác

Không sửa mã sản phẩm, dependency, Android manifest hay dữ liệu backend. Không build/cài lại APK, không merge PR hoặc gửi thông báo cho team. Tài liệu và bằng chứng mới ở checkout hiện tại, **chưa commit/push**. Đã kiểm tracked diff trống và `git diff --check` qua; untracked cũ tồn tại từ trước nên không gọi toàn bộ cây sạch.

Phục hồi đã đọc lại: font 1.0, night `no`, cả ba animation scale = 1; gỡ reverse 8099 trả exit 0, ghi tại [cleanup.json](reaudit-evidence/cleanup.json). Metro audit đã dừng, không còn listener 8099; các dịch vụ có sẵn khác không bị dừng. ADB 5037 có một lần timeout khi đọc/phục hồi cuối phiên; 5038 trả lời và xác nhận cấu hình trên, không kill server của team. APK debuggable vẫn được giữ nguyên.

Các script thực thi lưu tại `reaudit-evidence/scripts/`; chúng ghi lại rig của lượt này, có đường dẫn `/tmp` và tọa độ phụ thuộc thiết bị, không phải bộ test portable. Ma trận dùng cổng có sẵn `native-r11/kiem-bounds.mjs` và `native-r12/kiem-placeholder.mjs` trên XML mới; nguồn cổng nằm ở commit được ghi ở trên.

Ảnh tĩnh phần lớn là fixture, ảnh lỗi là renderer native với endpoint không truy cập được. Video là `adb screenrecord`, khung PNG trích bằng ffmpeg; không dùng ảnh sinh hoặc chỉnh bố cục để minh họa kết quả. So sánh release cần workload và dữ liệu tương đương, không lấy fixture-dev so với live-release rồi quy mọi khác biệt cho build.

Questions skipped: yêu cầu hiện tại là tái audit và đưa phán quyết cụ thể, chưa yêu cầu chọn phạm vi triển khai sửa. Các đề nghị ở trên đã đủ để team thực hiện vòng tiếp theo.
