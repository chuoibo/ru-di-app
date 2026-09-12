# ADR-0027 — Sổ hai người theo chu kỳ; tờ giấy bất biến theo phiên bản

- **Trạng thái:** 🟢 **ĐÃ CHẤP NHẬN** 2026-09-12 — Lead đánh dấu trong phiên 2026-09-12 («oke đồng ý chốt hết docs đi»), sau khi được nói rõ cái giá của khoản bổ sung ADR-0019 (spec 11.5). Codex viết; Lead chấp nhận. Backend do Claude làm theo uỷ quyền ADR-0016 §2.3; charter không đổi. Bảng được tạo ở Phase 3 của kế hoạch, sau khi Phase 0–2 vào `main`.
- Người đề xuất: Codex, trả lời K1–K7 của spec `f0173bad` §20.3.
- Chờ review độc lập và chấp nhận theo quy trình; không phải tự xác nhận C1–C3.
- Phạm vi hiện thực đầu: lát 1. Hẹn mở, túi riêng, sổ bảy mục, thông báo,
  chi tiêu chung, bản đồ và khách không được kéo vào migration lát 1.
- Đi kèm [khoản bổ sung ADR-0019/0021](proposals/2026-09-12-bo-sung-adr-0019-nguon-rieng-trong-pair.md).

## 1. Bối cảnh và các lựa chọn đã có

Lead đã chọn một sổ đôi hoạt động/người, không khách V1, nối lại được bằng
chu kỳ mới. `contexts.kind` chỉ có `group|pair`; outings được nhiều feature
dùng chung. Máy chủ có `install_commit_before_response`, `_require_permission`
và middleware idempotency, nhưng chưa có bảng giấy/consent của sổ.

Một hàng tờ giấy ghi đè nội dung làm mất nghĩa của đồng ý phiên bản. Một cờ
`is_couple` không giữ được lịch sử đóng/mở. Unique theo `(paper, version)`
không ngăn cùng tờ sinh hai outing từ hai phiên bản. Vì vậy chọn cấu trúc sau.

## 2. K1–K2: định danh, chu kỳ và các ràng buộc DB

Các tên là tên dự kiến để lập kế hoạch; DDL cụ thể phải qua migration review.

| Quan hệ | Khóa và trách nhiệm |
|---|---|
| `pair_notebooks` | Một định danh sổ trên `context_id` unique; tồn tại hàng không có nghĩa đã được consent. |
| `pair_notebook_cycles` | `id` mới mỗi lần lập/nối lại; thuộc đúng sổ; trạng thái pending/active/closed, `closed_at`, phiên bản chính sách/quyền. Tối đa một chu kỳ chưa kết thúc trên sổ. |
| `pair_cycle_participants` | Đúng hai danh tính của pair; snapshot cho chu kỳ. PK `(cycle_id, person_id)`; không lấy id do client khai làm bằng chứng tham gia. |
| `pair_consent_proposals` + `pair_consents` | Đề nghị có purpose, phiên bản điều khoản, hạn, chu kỳ; consent của từng người trỏ đúng đề nghị, có granted/revoked. Không kế thừa sang đề nghị hay chu kỳ mới. |
| `active_couple_members` | `person_id` unique/PK, trỏ chu kỳ đang bật đôi. Hai hàng sinh/xóa cùng transaction; giữ lịch sử consent ở bảng khác. Không giữ slot chỉ vì có lời mời pending. |
| `pair_papers` | Định danh tờ, context, chu kỳ nếu đã lưu sổ; có chủ nháp, loại tác giả human/nep, trạng thái, phiên bản hiện hành. Lời rủ trước consent có phạm vi tạm theo pair. |
| `pair_paper_versions` | UNIQUE `(paper_id, version)`; nội dung/lý do/nguồn theo phiên bản. Bản đã gửi bất biến; sửa sinh phiên bản khác. Nháp mới vẫn riêng dù tờ cũ đã được gửi. |
| `pair_paper_responses` | FK tới đúng `(paper_id, version)`, người thật và loại phản hồi; đồng ý chỉ một lần mỗi người/phiên bản. Mốc xem riêng, không phải response đồng ý. |
| `pair_paper_outings` | `paper_id` PK, `outing_id` unique, FK đúng phiên bản chốt và cùng context. Một tờ có nhiều phiên bản nhưng tối đa một outing. |
| `pair_paper_keeps` | Dòng giữ lại, tác giả thật, thời điểm; không tự đưa nội dung vào analytics. |
| `pair_shared_constraints` | PK `(cycle_id, owner_id, kind)`; `kind` chỉ hai ô pilot. Chỉ chính chủ ghi; nội dung chia cho đúng người còn lại. |

CHECK local `context_kind='pair'` phải đi cùng FK ghép
`(context_id, context_kind) → contexts(id, kind)` và UNIQUE đích tương ứng.
Chỉ viết CHECK ở bảng phụ thì một group vẫn có thể bị gắn nhãn pair giả.
Các FK ghép tương tự giữ cùng context/sổ/chu kỳ/tờ/phiên bản; nullable của lời
rủ tạm cần CHECK phân biệt rõ tạm và đã gắn chu kỳ, tránh FK ghép bị bỏ qua do NULL.

DB cưỡng chế bất biến phiên bản bằng trigger chống sửa/xóa bản đã gửi (trừ
đường xóa dữ liệu được thiết kế riêng), FK/unique cho người và phiên bản.
Điều kiện liên hàng «hai người khác nhau, đúng pair, cùng phiên bản» và «bật
đôi có đủ hai consent + hai slot» cần constraint trigger kiểm lúc kết thúc
transaction; CHECK đơn lẻ không đếm được hàng ở bảng khác. Mọi đường sửa
consent, participants, slots và trạng thái liên quan đều phải tham gia kiểm.
Chuyển sang `chot` phải có đúng một liên kết outing và đủ hai consent hợp lệ;
DB từ chối trạng thái chốt không có liên kết, liên kết sai phiên bản hoặc thao
tác xóa liên kết để tạo outing lần nữa. Các hàng consent cần thiết cho snapshot
đã chốt không bị viết lại bởi việc thu hồi quyền xử lý dữ liệu về sau.
Repository khóa các hàng liên quan trước khi quyết định; trigger không thay
thế giao thức khóa khi nhiều transaction chạy đồng thời.

## 3. K3: chốt, chỉnh sửa và retry

Trình tự ghi: khóa pair/context và chu kỳ trước, rồi tờ, rồi các hàng con theo
thứ tự ID ổn định. Các thao tác nhiều người, như bật đôi, khóa `people` theo
ID trước khi lấy khóa context; các đường chặn/xóa tài khoản liên quan phải cùng
thứ tự này. Không gọi mô hình hay mạng ngoài trong đoạn đang giữ khóa ghi.

1. Đọc quyền/membership/block/chu kỳ từ DB; từ chối nếu đã đóng, hết hạn, đã hủy
   hoặc client đang trả lời một phiên bản đã bị thay.
2. Người thật gửi thì ghi consent cho **chính người gửi** trên phiên bản đó.
   Nếp gửi thì **không** ghi consent người nào. Nếp không có person_id giả.
3. Ghi phản hồi người thật. Khi đủ đúng hai người đồng ý phiên bản hiện hành,
   tạo outing và liên kết `pair_paper_outings`, chuyển `chot` trong **cùng session,
   cùng transaction**. Outing có `headcount=2`; ngày/budget được kiểm như cửa
   tạo outing hiện có, VND nguyên. `created_by_id` là người thực hiện xác nhận
   cuối; tác giả giấy và các đồng ý vẫn là các sự kiện riêng.
4. Commit thành công rồi mới trả thành công, dùng cơ chế unit-of-work hiện có.
   Lỗi commit không được để client thấy đã chốt. Retry sau commit hoặc với một
   Idempotency-Key mới tìm liên kết đã tồn tại, không tạo outing mới.

`UNIQUE(paper_id)` là cổng một outing, không phụ thuộc client giữ key đúng.
Retry phiên bản cũ sau khi có phiên bản mới nhận lỗi stale; không âm thầm đồng
ý bản mới. Hai người sửa đồng thời chỉ một bản dựa trên current-version cũ
thắng; người còn lại đọc lại phần thay đổi trước khi gửi.

Pilot đóng băng phiên bản giấy sau `chot`, không tạo `v+1` qua route giấy.
Buổi đi hiện hữu vẫn theo quyền và đường sửa outing hiện có; bản giấy là snapshot
đã đồng ý, không bị sửa lén theo outing. Nếu outing đã đổi, projection phân biệt
kế hoạch hiện tại với bản đã chốt; không lấy đồng ý cũ chứng nhận lịch mới.
Không thêm cột trạng thái vào `outings` hay đổi nghĩa nghĩa vụ tiền.

`da_xem` là mốc đã xem, không phải consent. Để hết nhập nhằng giữa bảng §3.1 và
§3.3 spec, pilot chỉ cho `rut` khi người nhận **chưa xem và chưa phản hồi**.
Nghỉ trước chốt hủy tờ pending và ghi nghỉ tuần; không cấm giấy người dùng tự
gửi. Mốc hạn kiểm ở mọi biên đọc/ghi, không chờ job đổi enum mới có hiệu lực.

Đề xuất cho đường còn thiếu `da_di`: một participant chủ động ghi nhận, có
`recorded_by` và thời điểm. Không suy từ ngày hoặc vị trí. `da_giu` chỉ được ghi
khi lưu thành công một dòng sau ghi nhận đó. Đây là dữ liệu tự khai; không phải
bằng chứng cả hai cùng có mặt. Cần tác giả đồng bộ vào §3 trước khi code.

## 4. K4: bốn mục đích consent và permission roster

| Việc | Bằng chứng và phạm vi |
|---|---|
| Nhận buổi đi | Hai consent người thật trên **phiên bản lời rủ**; không bật sổ. |
| Lập sổ | Cả hai chấp nhận cùng đề nghị lưu lịch sử của chu kỳ; trước đó chỉ xử lý lời rủ tạm. |
| Bật đôi | Cả hai chấp nhận mục đích couple trên chu kỳ đang hoạt động; lấy đủ hai slot trước khi có vai/gậy. |
| Cho Nếp đọc chat | Cả hai chấp nhận mục đích chat processing đang còn hiệu lực; mặc định tắt. Không lấy một lần `/plan` hay friendship làm đồng ý của cả hai. |

Lát 1 cần đường consent tối thiểu bậc 2–3 nếu có lưu sổ và gậy. Màn cài đặt đầy
đủ vẫn ở lát 2. Bậc 4 là lựa chọn độc lập; tắt thì phác từ nguồn khác được phép.
Quyền tự gửi, khi triển khai, là tùy chọn riêng được cả hai chấp thuận; bậc 3/4
không tự bật quyền ấy.

Lời rủ ban đầu nằm trong pair đã có quyền liên lạc, chỉ hai participant đọc;
không tự lập kho lịch sử. Nếu chỉ đồng ý outing mà không lập sổ, tạo outing
bình thường và kết thúc dữ liệu tạm: không lưu nội dung giấy/ràng buộc vào kho
sổ; chỉ giữ định danh/trạng thái tối thiểu cần chống retry nhân đôi. Không dùng
payload cache idempotency làm một kho giấy thứ hai. Đề nghị hết hạn cùng hạn
đã hiển thị cho lời mời; không dùng một consent cũ cho một đề nghị mới.

Service dựng `AuthorizationFacts` từ repository, gọi `_require_permission`;
domain chỉ nhận facts và quyết định, không import DB/API. Khai action đọc/ghi
giấy, gửi, phản hồi, mở, giữ dòng, sửa ràng buộc, grant/revoke, đóng sổ vào
`permissions._TABLE`. Vai Người lo/Người chấm không cấp quyền sửa hộ dữ liệu riêng.
Roster phải kể cả list/detail/projection lịch sử, preview, media, nguồn mô hình
và replay; UI capability config chỉ dùng trình bày.

Một người revoke chat là đủ: dừng xử lý ngay, vô hiệu nguồn/nháp dẫn xuất chưa
được gửi. Grant lại cần hai chấp thuận cho đề nghị mới. Trang đã ghim giữ để
chủ đọc theo spec, nhưng việc giữ một trang không tự khôi phục quyền AI dùng
chat nguồn. Đóng sổ thu hồi toàn bộ capability hoạt động của chu kỳ.
Chặn/xóa tài khoản vẫn áp quy tắc deny của ADR-0023; lịch sử tài chính có đường
đọc riêng, không bị xóa chỉ vì đóng sổ đôi.

## 5. K5: nguồn có dấu vết và biên gửi

Máy chủ lập manifest nguồn cho toàn bộ đầu vào: nội dung, lý do, xếp hạng địa
điểm, budget, chat, tóm tắt, cache và dữ liệu mô hình dùng lại. Mỗi nguồn có
owner/scope/source-id/source-version và phiên bản quyền; không tin nhãn client.
Đầu ra thừa kế phạm vi hạn chế nhất của các đầu vào. Thiếu provenance thì không
đủ điều kiện tự gửi.

Nháp có nguồn riêng chỉ chính chủ xem. Gửi thủ công là chia **nội dung phiên bản
đã xem trước**, không chia toàn bộ dữ liệu nguồn; request ghim version/hash, mô
hình không được sinh lại nội dung khác sau cú bấm. Người gửi không được duyệt
thay dữ liệu riêng thuộc người kia.

Tự gửi phải dựng mới từ nguồn chung còn quyền; không xóa nhãn private hay chỉ
bỏ câu nhạy cảm khỏi một nháp đã dùng nguồn riêng. Nguồn thiếu thì không gửi.
Đọc snapshot nguồn trước bước tính; trước publish, kiểm lại source-version và
epoch quyền dưới khóa. Có thay đổi thì bỏ kết quả hoặc yêu cầu phác lại, không
publish kết quả cũ. Kiểm soát nguồn pair ở các cửa cũ theo khoản bổ sung ADR.

## 6. K6: không phụ thuộc thông báo, không thêm việc nền

Lát 1 đọc lại giấy khi mở/quay lại app. Khi cần phác theo tuần, client gọi một
lệnh khi đang hoạt động; thao tác sinh dữ liệu không núp trong GET. Unique theo
chu kỳ/tuần/loại đề nghị chặn mở nhiều thiết bị sinh nhiều tờ. Tuần tính theo
timezone/khung của sổ; thay timezone không được cấp lại hạn mức tuần đã dùng.

Không có cam kết phát tờ đúng giờ khi không có request. Đề xuất pilot chỉ gửi
thủ công; nếu tác giả giữ tự gửi ở lát 1, phải ghi rõ chỉ xét khi có request hợp
lệ, trước hạn, với hai consent người thật để chốt như §3. Không mở job thứ ba
trong AfterResponse. Không tạo notifications/device/token để giải việc này.
Hẹn mở và loại notification mới phải quay lại ADR-0024 ở lát sau.

## 7. K7: hai ô ràng buộc và nguồn không biết

Hai ô là khai báo chung **của chính người nhập**, phục vụ đúng sổ này; không
ghi/đọc chéo person_interests hoặc sổ khác. Không cho người kia sửa hộ. Trước
lập sổ, chỉ chia tạm theo lời rủ sau preview rõ người nhận; không tự lưu lâu dài.
Sau lập sổ, dùng `pair_shared_constraints`, kể cả sổ bạn chưa bật đôi.

Mỗi lần thêm/sửa/gỡ tăng version; phác/gửi/chốt kiểm lại ràng buộc hiện hành.
Thêm một điều cấm không được để nháp cũ chưa chốt tiếp tục đề nghị điều đó.
Gỡ không sửa hồi tố tờ đã chốt; nháp chưa chốt phải kiểm theo phiên bản mới.
Đóng sổ dừng dùng; chu kỳ mới không tự chép ràng buộc sang, chỉ chia lại rõ ràng.

«Ràng buộc cứng» nghĩa là không cố ý đề nghị điều đã biết là vi phạm. Danh mục
hiện không chứng minh thành phần món ăn; không hiểu được điều cấm hoặc không
đủ thông tin thì trả trạng thái cần người dùng kiểm/chọn, không gắn nhãn đã
bảo đảm phù hợp. Không đưa nội dung hai ô vào analytics, logs hoặc dịch vụ ngoài;
nếu cần xử lý bằng mô hình ngoài, phải có hợp đồng dữ liệu được chấp nhận trước.

## 8. Đóng sổ và dữ liệu đã trả về

Đóng một chiều, khóa chu kỳ và tờ theo cùng thứ tự với chốt/mở/gửi. Preview
trả revision của tập bị ảnh hưởng + số tờ sẽ khóa/hủy; xác nhận trên revision
đó. Nếu có tờ mới/mở/chốt trong lúc xem, trả conflict và preview mới, không
âm thầm đóng theo con số đã lỗi thời.

Commit đóng trước thì mọi chốt/mở mới bị từ chối. Commit chốt trước thì outing
đã tồn tại thuộc lịch sử giữ lại. Thư chưa mở bị khóa vĩnh viễn, nháp và lời
đề nghị pending bị hủy. Nguồn/tóm tắt/cache/nháp của chu kỳ không được dùng để
phác hay gửi mới. Nối lại tạo ID mới, không reset cờ closed của chu kỳ cũ.

Không hứa lấy lại bytes đã được thiết bị nhận. Tuy vậy máy chủ phải chặn mọi
đường **đọc mới**, gồm detail/list, preview, file media và replay. Middleware
idempotency hiện có có nhánh replay trả response trước service; route giấy
không được lưu nội dung riêng/khóa được vào response replay. Chọn response
lệnh chỉ gồm ID/trạng thái tối thiểu, client đọc nội dung qua route kiểm quyền;
kể cả replay metadata cũng phải xác thực và kiểm quyền hiện hành trước khi trả.
Kế hoạch phải mở rộng seam này đúng phạm vi, không đổi replay của đường tiền.
File ở lát sau không được dùng URL công khai như một cách bỏ qua `mo_tu`/closed.

## 9. Điều kiện kiểm chứng khi hiện thực

- Domain: human-send một consent, nep-send không consent; stale revision,
  nghỉ/hết hạn, ghi nhận đã đi, giữ dòng; không đồng ý từ im lặng.
- API prod-session: outsider, người còn lại đọc nháp, sửa hộ ràng buộc, nâng
  bậc consent bằng một cú nhận lời; grant/revoke và tất cả projection có ca âm.
- PostgreSQL thật: FK chéo pair/tờ/phiên bản, hai consent cùng người, group giả
  pair, sửa version bất biến; mỗi trường hợp ghi SQL sai phải bị DB từ chối.
- PostgreSQL với hai connection: chốt/chốt, sửa/chốt, đóng/chốt, đóng/mở,
  revoke/publish, hai pair tranh một người, retry khác key sau mất response.
  Assert cả response lẫn dữ liệu, không chỉ đếm lời gọi fake.
- Thu hồi: nháp/cache lineage cũ không ra ngoài; replay không trả nội dung đã
  khóa; quyền mới của chu kỳ sau không đọc được thư khóa của chu kỳ trước.
- Lát nguồn pair: cấm đi vòng qua companion/catalogue cũ; ca hồi quy group
  vẫn đọc tổng gu như trước. Không chạy dữ liệu thật qua dịch vụ mô hình.
- Migration upgrade → downgrade → upgrade trong schema Postgres riêng; DDL
  khớp model, import boundary, permission roster và các cổng tiền hiện có.

Tài liệu này không chứng minh các ca đã chạy. Chấp nhận ADR cho phép triển khai
theo hợp đồng; không phải nghiệm thu feature hoặc phê duyệt thẩm mỹ.
