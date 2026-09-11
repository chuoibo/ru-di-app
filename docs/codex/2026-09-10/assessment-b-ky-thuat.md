# Assessment B độc lập — tái kiểm kỹ thuật 10/09/2026

Commit kiểm: `0c1a41698148268412eb55a4303d34537b54bfa9`. Không đọc Assessment A, không sửa nguồn sản phẩm, không điều khiển máy ảo, không mở Metro/API. Đây là nhánh kiểm mã/cổng; ảnh native mới do nhánh chính thu riêng. Không đưa ra phán quyết mỹ thuật.

## Kiểm lại đã thực hiện

- Node `v20.20.2`; biên dịch lại `tsc -p tsconfig.test.json`, chạy `tools/fixup-esm.mjs`, đều exit 0. Chạy trực tiếp từng file test để lấy số ca bên trong: **66 ca qua, 0 lỗi, 0 bỏ qua** trong 9 file. Gồm hook thật 7; hàng chờ 10; tin sống 7; trả lời/xóa 9; sticker 4; motion 5; placeholder 3; trạng thái/lỗi 8; đường nét/cảnh 13. Log và bảng đếm: `/tmp/rudi-reaudit-tech/direct-test-summary.json` cùng các log theo tên file. Không chạy lại toàn bộ npm/pytest vì không có thay đổi sản phẩm trong nhánh này.
- Runner canary cố tình ném lỗi trả exit 1. Không sửa thư viện cài đặt để làm xanh.
- Probe F42 cũ chạy lại trên nguồn hiện hành: trước/sau đều chỉ `B-existing`, lượt đọc chỉ `A,B`, `reproduced: false`. Exit 1 là đúng với probe **đòi lỗi lịch sử tái hiện**; không phải sửa mới thất bại. Log: `/tmp/rudi-reaudit-tech/old-probe.log`.
- Cổng bounds chạy trên XML lịch sử `16-failed-sticker-dark.xml`: exit 1, bắt đúng mất node chữ «Thử lại». Đây là canary của cổng, không phải phép đo giao diện hiện tại. Log: `/tmp/rudi-reaudit-tech/bounds-canary.log`.
- Detector đúng một lần: `node .claude/skills/impeccable/scripts/detect.mjs --json apps/mobile/src`; exit 0, stdout `[]`, stderr rỗng. **0 findings; không có tên rule hoặc vị trí finding.** Log: `/tmp/rudi-reaudit-tech/detector.{json,stderr,exit}`. Đây là quét mã nguồn, không chứng minh bố cục native, tương phản pixel, nghĩa hình hoặc chuyển động. Lượt này không phát banner DEGRADED; không được sao chép trạng thái DEGRADED của lượt cũ thành kết quả mới. Không inject browser overlay: mục tiêu là React Native, nhánh này được giới hạn không mở server; không có overlay được tạo.

## F41–F46: điều mã và cổng thực sự chứng minh

| Mục | Kết quả kiểm lại | Biên giới còn lại |
|---|---|---|
| F41 | Hai nút lỗi dùng `full={false}`; hàng cho wrap; kit có sàn rộng 48dp và cao 48/52dp. Cổng lịch sử đỏ đúng. | Bounds mới phải chạy trên XML native mới. Cổng hardcode mật độ 2.625; không áp trực tiếp cho tablet/mật độ khác. |
| F42 | Hook React thật 7/7 qua. Cleanup tăng thế hệ; completion cũ bỏ cả thành công/thất bại; chặn poll cũ; read-mark không lấy tin A dưới id B. Probe độc lập cũ không tái hiện lỗi. | Không có tái hiện native chuyển nhóm giữa live request trong nhánh này; test dùng transport trả bằng tay và router/RNW stub. |
| F43 | 8/8 test trạng thái/lỗi qua, gồm quét toàn src cho prose nội suy địa chỉ máy chủ và câu mất mạng/404. | Quét nhận dạng các biến cụ thể; không phải chứng minh mọi thông báo lỗi từ backend đều an toàn. |
| F44 | 3/3 test markup qua: bỏ hint native, tên trợ năng đầy đủ, placeholder biến mất khi value có chữ; nguồn đặt một dòng. | RNW không chứng minh Android/iOS hiển thị hay TalkBack. Component nhận `defaultValue` nhưng state rỗng không đọc giá trị này; chưa thấy caller dùng `defaultValue`, nên đây là góc API chưa dùng, không nâng thành lỗi luồng đang có. |
| F45 | 13/13 test art qua; pose đọc từ dữ liệu vẽ; 9 cảnh có Nếp dùng 9 pose riêng, cảnh lỗi thứ 10 có giá trị null. Cảnh lỗi không còn cung coral, coral đi theo vết rách. | Tên pose khác nhau không tự chứng minh silhouette khác hoặc người xem hiểu đúng; đánh giá hình thuộc A/người dùng. `bo-loc-che-het` còn vòng hở cần quyết định bằng ảnh. |
| F46 | `DESIGN.md:1135` phân biệt lịch sử và hiện hành; hai đoạn lịch sử tại 1793/1797 ghi rõ tới 08/09; luật F41–F45 có tại 1297–1373. Drift được báo trước đây đã sửa ở các chỗ này. | Không có rà từng câu toàn DESIGN.md; tài liệu motion mới vẫn có sai lệch riêng bên dưới. |

## Finding mới về phương pháp motion

### B1 — P1: bảng hiện tại trộn khởi động/đăng nhập với thao tác cần đo

`motion/do-motion.sh:35–39` ghi chú warm-up nhưng thực thi **force-stop → reset gfxinfo → chạy Maestro**. Mỗi `m*.yaml` lại gọi `_vao-app-sach.yaml` hoặc `_vao-live.yaml` ngay đầu; các helper còn `stopApp`/`launchApp`. Không có reset sau warm-up. Helper fixture còn vào trải nghiệm, bỏ sở thích, đi Cá nhân/Tài khoản, đăng xuất rồi vào lại (`_vao-app-sach.yaml:39–85`). Toàn bộ khung của đoạn mở app và setup nằm trong histogram sau cùng.

`motion/README.md:16–18` mô tả ngược: vào app rồi reset rồi chạy chuỗi. Vì vậy con số 6–13% không phải số riêng của đổi tab/cuộn/sheet/back. Điều này không biến rc=0 thành giả, nhưng làm tên bảng và kết luận về thao tác thiếu căn cứ.

Điều kiện gỡ: setup/warm riêng; xác nhận màn đầu; reset khi tiến trình đang sống; chỉ chạy thao tác; lấy gfxinfo cùng tiến trình ngay sau đó. Giữ startup thành phép đo khác nếu cần. Bổ sung mã exit thất bại khi Maestro lỗi, khung rỗng hoặc dump lỗi: script hiện chỉ lưu rc trong bảng, cuối cùng `cat` nên bản thân script vẫn có thể exit 0 khi mọi flow đỏ. Đây chưa phải cổng tự chặn hồi quy. Đã kiểm hành vi này bằng chính script với `adb`/`maestro` giả lập nằm riêng trong `/tmp` (không chạm máy ảo): cả bốn Maestro cố ý exit 42, mọi trường khung là `?`, nhưng script vẫn **exit 0**. Trace, stdout và exit tại `/tmp/rudi-reaudit-tech/runner-canary/`; đây chỉ là canary điều khiển shell, không phải dữ liệu motion/native.

### B2 — P2: diễn giải p99 và bảo đảm release vượt dev không có đủ căn cứ

`motion/README.md:51` nói p99=150ms ở mọi hàng và là trần histogram. Dữ liệu chính trong repo bác cả hai: hàng cuộn thường có p99 **117ms**; histogram m1 thường vẫn có **200ms=4, 250ms=1**, tiếp đến các bucket tới **4950ms** (`dev-client/thuong/m1-doi-tab.gfxinfo.txt:20`). 150ms là nhãn bucket được chọn cho percentile ở các hàng ấy, không phải giới hạn trên của histogram. Không thể dùng câu “trần bucket” để xóa ý nghĩa của đuôi chậm.

Đếm lại tám dump bằng script độc lập: tổng histogram khớp khung bảng, có 2–6 khung ở bucket trên 150ms mỗi lượt. `/tmp/rudi-reaudit-tech/histogram-check.json` lưu kết quả. Đây là kiểm số cũ, không phải đo lại tốc độ hôm nay.

`README.md:28–29` bảo dev là giới hạn trên và “release ít nhất tốt bằng”. Không có phép đo release cùng workload để bảo đảm điều đó; nên chuyển thành giả thuyết dev overhead có thể ảnh hưởng. Cả hai build còn dùng dữ liệu/luồng khác (fixture so với live), nên chưa có cặp so sánh tương đương.

### B3 — P2: bộ chạy chưa phục hồi cấu hình hệ thống chắc chắn

`do-motion.sh:29,50` đặt ba animation scale về 0 rồi cuối cùng hardcode về 1; không chụp giá trị ban đầu, không trap INT/TERM/EXIT. Dừng giữa lượt có thể để Reduce Motion bật, còn giá trị ban đầu khác 1 sẽ bị ghi đè. Điều kiện gỡ: lưu cả ba giá trị và restore qua trap; báo thiếu dữ liệu trước khi gọi là lượt hợp lệ.

Helper live nhận màn «Chào bạn» là điểm bắt đầu hợp lệ (`_vao-live.yaml:11`) nhưng nhánh nhập OTP chỉ chạy nếu thấy «Rủ Đi thôi!» (:15); nếu vào thẳng «Chào bạn», nó bỏ qua OTP rồi đợi «Khám phá» tới timeout. Cần xử lý cả hai điểm vào trước khi coi chuỗi live “chạy ngay”.

## Kết luận phạm vi B

Các sửa nguồn F41–F46 có bằng chứng mới ở mức nguồn, React hook và canary; không thấy F42 tái hiện trong hai mô hình hook đã chạy. **Chưa đủ cơ sở đóng §5 motion**: phải sửa cách khoanh cửa sổ đo và diễn giải số trước, rồi mới đo lại đúng workload. Bộ đếm gfxinfo hay rc=0 không thay thế xem chuyển cảnh và cảm giác chạm trên máy thật.
