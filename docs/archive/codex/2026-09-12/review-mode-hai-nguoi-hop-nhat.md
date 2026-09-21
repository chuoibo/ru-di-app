# Phản biện bản hợp nhất và trả lời K1–K7

Ngày: 2026-09-12 · Reviewer: Codex.
Spec được soát: `f0173bad1621f085111c3e98a797a759bdd318a9`, 1443 dòng.
Đối chiếu thay đổi từ `7c079256`, báo cáo [lượt hai trên PR #609](https://github.com/chuoibo/ru-di-app/pull/609), và nguồn backend tại SHA spec.
Backend và các ADR được đọc cũng trùng `main` tại `2d0b97778146f2253c2657dd1a59853e508e531d`.

**Verdict trên đúng bản f0173bad: REQUEST_CHANGES, phạm vi C1–C2 bên dưới.**
C3 đã khép ở mức luật sản phẩm. K1–K7 có câu trả lời cụ thể; không còn câu nào
phải đẩy lại Lead. Có thể dùng ngay để viết **kế hoạch nháp lát 1**; chưa mở cửa
tạo bảng hay gọi spec đã được duyệt toàn bộ.

Giữ nguyên bảy quyết định Lead ở §20.1 và các quyết định UI ở §20.2. Không yêu
cầu dựng hai màn để so trước kế hoạch. Lượt này không đo native, chuyển động,
accessibility hay hành vi tám tuần, và không chứng nhận UI đã stunning.

## 1. Trả lời thẳng K1–K7

Tên bảng sau đây là phương án của Codex cho kế hoạch/ADR, chưa phải schema đã ship.

| Câu | Kết luận của Codex | Điều phải bổ sung vào đề xuất |
|---|---|---|
| **K1** | **Chọn tờ giấy + phiên bản bất biến + phản hồi theo phiên bản.** | FK ghép phải giữ đúng tờ, context và chu kỳ; hai người khác nhau đồng ý đúng phiên bản hiện hành. Tác giả Nếp không được tạo hàng đồng ý giả cho người giữ lượt. |
| **K2** | **Chọn bảng phụ cho pair; tách định danh sổ và lịch sử chu kỳ.** | Một hàng hiện tại mỗi pair không thay được lịch sử nối lại. CHECK ở bảng con không tự chứng minh bảng cha là pair: cần FK ghép tới `(contexts.id, contexts.kind)`. Một đôi hoạt động/người cần khóa duy nhất **theo người**, không chỉ theo pair. |
| **K3** | **Chốt và tạo outing cùng transaction, commit trước response.** | Khóa chu kỳ rồi tờ; unique liên kết **theo paper_id** để không sinh outing thứ hai ở phiên bản khác. `(paper_id, version)` chỉ nhận diện điều đã đồng ý, không đủ cho luật một tờ/một outing. |
| **K4** | **Quyền nằm trong service/domain và DB; mỗi mục đích có consent riêng.** | Bậc 1 là phản hồi của phiên bản lời rủ. Bậc 2–4 là consent theo người, mục đích, phiên bản điều khoản và chu kỳ. Không dùng số `consent_level >= n`; không lấy membership hoặc vai Người lo làm consent. |
| **K5** | **Đúng hướng nhãn nguồn, nhưng nhãn phải do máy chủ dựng và kiểm lại khi gửi.** | Đường tự gửi phải tạo từ nguồn chung còn quyền; nháp đã dùng nguồn riêng không thể đổi nhãn rồi tự gửi. Phải chặn cả nguồn gián tiếp qua companion/cộng-gu hiện có. |
| **K6** | **Lát 1 không chờ notifications hoặc push.** | Nguồn đã kiểm chỉ có `people.notify_prefs`, chưa có hai bảng. Không có cam kết lịch ship notifications của Codex để dựa vào. Tờ đã gửi xuất hiện khi app đọc lại; không hứa gửi đúng giờ lúc không ai mở app. |
| **K7** | **Bảng ràng buộc chung riêng, theo chu kỳ sổ + chủ khai + loại ô.** | Chủ tự nhập/sửa/gỡ, người kia chỉ đọc; không ghi `person_interests` hay cờ chia toàn cục. Hai ô dùng được từ khi lập sổ hai người, không phải đợi bật «Một đôi». Trước consent lập sổ chỉ là dữ liệu của lời rủ đang chờ. |

Phương án đủ để tác giả viết phần backend của kế hoạch nằm trong
[ADR-0027 dự thảo](../../decisions/ADR-0027-so-hai-nguoi-va-to-giay-co-phien-ban.md).
Khoản bổ sung riêng tư được viết thành
[đề nghị bổ sung ADR-0019 và §2.6.2 ADR-0021](../../decisions/proposals/2026-09-12-bo-sung-adr-0019-nguon-rieng-trong-pair.md).
**Cả hai đang ĐỀ XUẤT. Codex không tự ký chấp nhận tài liệu do mình viết.**

## 2. Xác nhận độc lập C1–C3

### C1 — phần đã sửa đúng; còn ngoại lệ và đường khép vòng phải ghi rõ

Đã nhận: gửi bản sửa đảo người gửi; consent gắn phiên bản; lượt 0 tách khỏi vòng
gậy; nghỉ/hết hạn không thành đồng ý. Không yêu cầu quay lại các tranh luận đó.

**C1.1, P1 — «gửi = đồng ý» chưa loại trường hợp Nếp gửi.**
§3.2:205–218 dùng người gửi và người kia; §3.4:255–257 lại có tác giả Nếp.
Nếu adapter hiểu người giữ gậy là người gửi, một cú đồng ý của người còn lại sẽ
tạo buổi đi thay cho một người chưa đồng ý. Nếu không hiểu như thế, trạng thái
lại thiếu nhánh nhận lời thứ nhất.

**Cách khép:** người thật bấm gửi thì thêm một consent cho chính họ; Nếp gửi thì
có **0 consent người thật**, cần hai người đồng ý. Tắt/không triển khai tự gửi
trong lát 1 cũng hợp lệ nếu §19.1 ghi rõ. Cấm sinh outing với `created_by_id`
giả làm người giữ gậy. ADR đề xuất dùng người bấm xác nhận cuối làm người thực
hiện tạo outing; tác giả tờ vẫn giữ nguyên.

**C1.2, P2 — còn thiếu quy tắc sau `chot` và nguồn của `da_di`.**
§3.1:197–198 chỉ ghi mũi tên. Đề xuất pilot: tờ đã chốt giữ nguyên phiên bản;
không gửi `v+1` để tạo lại outing. Một người tham gia chủ động ghi nhận đã đi;
lưu `recorded_by`/thời điểm, không suy từ ngày trôi qua hay chứng nhận hai người
cùng có mặt. Một dòng được lưu thành công mới ghi nhận `da_giu`.
Tác giả cần đưa quy tắc này vào bảng chuyển trạng thái hoặc nhận dẫn chiếu ADR.

### C2 — hai luật đúng, nhưng chưa khép các chỗ sử dụng chúng

Đã nhận thang bốn bậc, chat mặc định tắt, tự gửi chỉ dùng nguồn chung, hai ô
do chính chủ chia. Còn hai chỗ có đường sửa xác định, không phải câu hỏi thẩm mỹ.

**C2.1, P1 — đường vào/lát 1 vẫn có hai cách hiểu consent.**
§7.2:435–442 nói đồng ý đi chơi không tạo sổ; §14.1:892 vẫn nói tờ giấy tạo ra sổ.
§19.1:1211 cần gậy trong lát 1 nhưng đặt Cài đặt sổ đôi ở lát 2: gậy chỉ được bật
khi đã có consent bậc 3, nên lát 1 cần **đường bật tối thiểu**, không cần toàn bộ
màn Cài đặt lát 2. §7.3:451 còn ghi chat thu hồi khi đóng sổ dù §7.2 cho gỡ riêng.

**Cách khép:** đường vào phân biệt gửi lời rủ, đồng ý buổi đi, đồng ý lưu sổ và
bật đôi; server không suy bậc sau. Người từ chối lập sổ vẫn có outing bình
thường, không có kho giấy lưu lại. Consent đang chờ có hạn và thu hồi được;
chấp thuận cũ không chuyển chu kỳ. Sửa ô chat thành «một người gỡ quyền hoặc
đóng sổ». Xóa/đánh dấu lịch sử câu «một luật đủ khép C2» còn ở §22.5:1408–1410.

**C2.2, P1 — cần quyết định xử lý đường companion và gu pair hiện có.**
`ApiService.group_taste` (`service.py:1085`) đọc sở thích và budget band của
mọi thành viên active. `take_companion_turn` (`service.py:5518`) chỉ kiểm
membership, rồi đọc chat (`5545`) và gọi `group_taste` (`5601`, `5609`).
ADR-0021 §2.6.2 còn cho pair dùng phép cộng đó một cách tường minh.
Đây là đối chiếu nguồn, **chưa phải khai thác runtime hay bằng chứng đã rò dữ liệu**.

**Cách khép:** nhận khoản bổ sung ADR kèm theo: pair có chính sách nguồn riêng;
mọi cửa máy chủ đưa chat/gu pair vào gợi ý chung đi qua nó, gồm cửa cũ. Giữ nguyên
truy vấn của `kind='group'`. Không thể vừa hứa luật mới cho pair vừa giữ đường
cũ thành lối vòng. Khoản bổ sung nêu rõ đây là thay đổi hành vi pair cần được
chấp nhận, không giả vờ nhãn nguồn ở một route mới đã giải quyết tất cả.

### C3 — KHÉP ở mức luật sản phẩm

§7.6:504–529 đã bỏ cửa sổ «tới hết chu kỳ», hủy nháp và lời đề nghị đang chờ,
khóa thư chưa mở, nối lại bằng chu kỳ mới. Đây là sửa đúng, đủ để ký khép
mâu thuẫn sản phẩm C3.

Phần cưỡng chế là công việc K2–K5: khóa chung cho đóng/chốt/mở; kiểm lại phiên
bản xem trước; vô hiệu nguồn và đầu ra dẫn xuất; không cho replay HTTP trả lại
nội dung nay đã bị khóa. ADR ghi các ca này làm điều kiện hoàn thành implementation,
**không dùng việc chưa có code để tiếp tục gọi C3 là mâu thuẫn sản phẩm**.

## 3. Việc cần làm để mở cửa tiếp theo

Tác giả nhận hoặc phản biện các điều khoản C1.1–C2.2 bằng sửa spec/dẫn chiếu
ADR; không cần trả lời lại bảy câu Lead. Claude review độc lập phương án backend
và khoản bổ sung. Lead chấp nhận ADR theo quy trình. Sau đó mới tạo bảng.

Kế hoạch nháp lát 1 có thể viết ngay với các điều kiện trên được nêu rõ. Ba phép
đo §20.5 nằm trong pilot; tám tuần là cổng đánh giá vòng sản phẩm, không yêu cầu
ngồi chờ tám tuần để gọi phần code đã hoàn tất kiểm kỹ thuật.

**Đã kiểm:** diff spec, bảng trạng thái/consent, caller backend, model/migration,
ADR-0019/0021/0024 và cơ chế commit/idempotency; tài liệu mới qua diff-check và
repo guard trước commit. **Chưa kiểm:** DB/race thực, route mới, native, hành vi
người dùng. Lượt này chỉ thêm tài liệu, không chạy suite để giả làm bằng chứng
cho các bảng/route chưa tồn tại.
