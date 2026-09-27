# Các khoảng trống dữ liệu cần xử lý cho AI tạo plan

Ngày: 27/09/2026. Trạng thái: **OPEN — ghi nhận để chốt phạm vi bổ sung**.
Chưa là ADR, migration hay cam kết triển khai. Không thay đổi database/runtime.

Tài liệu này tách ba loại vấn đề: trường đã có nhưng dữ liệu thiếu; thông tin
chưa được mô hình hóa đủ; dữ liệu đã có nhưng pipeline AI chưa sử dụng đúng.
Không mở migration chỉ để giải quyết vấn đề nhập dữ liệu hoặc truyền payload.

## 1. Phạm vi bằng chứng

Snapshot đọc tổng hợp khoảng 14:05 giờ Việt Nam ngày 27/09/2026, từ database
cấu hình của container local `rudi-vnlocal-api-1`. Query chạy READ ONLY rồi
rollback. Không đọc/xuất tên người, body chat, ảnh hoặc hồ sơ cá nhân.

**Đây là DB local, không phải production hay số liệu toàn sản phẩm.** Snapshot
cần đo lại khi làm implementation; chưa có audit production trong task này.
Code khảo sát ở HEAD `c8f092c1` và working tree có thay đổi chưa commit.

Bằng chứng chi tiết:
[kiểm kê dữ liệu](insight-va-du-lieu-ca-nhan-hoa-plan.md),
[flow code](nghien-cuu-ai-tao-plan.md),
[analysis và thuật toán](phan-tich-gioi-tre-viet-va-thuat-toan-planner.md).

## 2. Trường đã có nhưng thiếu dữ liệu trong snapshot local

Tổng 9.363 địa điểm. Các tỷ lệ dưới tính trên số địa điểm, không phải chất lượng
hay độ đúng của dữ liệu. Non-null không chứng minh trường hợp lệ/còn cập nhật.

| ID | Trường / dữ liệu | Hiện trạng | Ảnh hưởng | Ưu tiên |
|---|---|---|---|---|
| D01 | `price_min_vnd`, `price_max_vnd` | 0 / 0 điểm có giá | Không kiểm được trần chi, không rank giá đáng tin | P0 |
| D02 | `open_hours` | 0 điểm có giờ | Không chứng minh điểm mở đúng lúc đến | P0 |
| D03 | `activities` | 0 điểm có dữ liệu không rỗng | Không có supply hoạt động để recommend workshop/trải nghiệm có thật | P0 cho pilot hoạt động |
| D04 | `license` địa điểm | 0 điểm có thông tin | Chưa xác minh quyền sử dụng nguồn catalogue | P0 trước mở rộng nguồn |
| D05 | `traits` | 0 điểm có dữ liệu không rỗng | Thiếu thuộc tính phù hợp nói chuyện, nhịp, không gian | P1 |
| D06 | `group_fit` | 0 điểm có dữ liệu | Thiếu bằng chứng phù hợp nhóm/số người | P1; giới hạn sức chứa cần thiết là P0 |
| D07 | `rating` | 0 điểm có rating | Không dùng được tín hiệu này; không cản một planner dựa trên facts/brief | P2, tùy quyền nguồn |
| D08 | `address` | 8.152 có, 1.211 null | Khó hướng dẫn đến nơi; có địa chỉ vẫn cần kiểm tọa độ/lối vào | P1; điểm được chọn cần chỉ dẫn đủ |
| D09 | Ảnh địa điểm | 5.600 có, 3.763 chưa có | Hạn chế preview; ảnh không chứng minh độ hợp/giá/giờ | P2 |
| D10 | Phủ địa lý | 33 destination, 8 có POI; một destination có 5.288 điểm | Dễ trả ít lựa chọn hoặc lệch vùng | P0 xác định vùng pilot; P1 mở rộng |

P0: chặn khả năng hứa plan khả thi hoặc chặn supply cụ thể của pilot.
P1: cần cho độ hợp/cá nhân hóa và mở rộng. P2: cải thiện discovery/trình bày.
Không coi P0 nghĩa là phải điền đầy toàn bộ 9.363 điểm trước khi pilot; cần đủ
inventory đã kiểm trong phạm vi pilot và biết abstain ngoài phạm vi đó.

`source = vnlocal` và `source_ref` có ở toàn bộ địa điểm, nhưng chưa tìm thấy
importer này trong checkout đã khảo sát. Cần xác minh image/importer/nguồn gốc
trước khi dùng description làm bằng chứng. `license` địa điểm null không có
nghĩa đã kết luận nguồn vi phạm quyền; cũng không kết luận license ảnh vì ảnh
là bảng/nguồn riêng. D04 là **chưa xác minh**, không phải cáo buộc.

## 3. Thông tin chưa được mô hình hóa đủ cho planner

Đây là đề xuất yêu cầu dữ liệu/contract, **không phải danh sách cột đã duyệt**.
Cần audit migrations/schema đầy đủ trước khi quyết định thêm bảng/cột; thông
tin có thể đã tồn tại ở dạng khác nhưng chưa thành contract của planner.

| ID | Thông tin cần biểu diễn | Vì sao trường hiện tại chưa đủ | Yêu cầu tối thiểu đề xuất | Ưu tiên |
|---|---|---|---|---|
| M01 | Giá có nghĩa và phạm vi | Min/max chưa nêu người/nhóm/vé, gồm gì, ngày áp dụng | VND integer; đơn vị, khoản gồm/chưa gồm, nguồn và thời điểm kiểm | P0 |
| M02 | Giờ dùng được để lập lịch | Chuỗi giờ chưa đủ lịch ngày cụ thể/ngoại lệ | Timezone, khoảng phục vụ, ngày đóng/ngoại lệ, last entry nếu có | P0 |
| M03 | Trải nghiệm và suất cụ thể | Tag/activity JSON không tự chứng minh có workshop đặt được | Activity gắn venue; ngày/suất, duration, giá, cần đặt trước, capacity/availability nếu có nguồn | P0 cho activity |
| M04 | Độ mới và xác minh từng fact | Source cấp địa điểm/updated_at không chứng minh giá/giờ vừa được kiểm | Evidence từng thuộc tính, checked_at, hạn dùng, trạng thái unknown/stale/conflict | P0 |
| M05 | Brief của lần đi | Lời nhờ chưa thành intent/ràng buộc có cấu trúc | Nơi/hoạt động muốn thử, ngày/khung giờ, người đi, mục đích, pace, novelty, travel mode | P0 |
| M06 | Phạm vi ngân sách lần này | Budget band onboarding không phải budget du lịch | Trần mỗi người/cả nhóm, mỗi ngày/cả chuyến, khoản được tính | P0 |
| M07 | Constraint từng người | Gu nhóm gộp không giữ điều tránh/ngưỡng chi từng người | Điều bắt buộc/không muốn, phạm vi, quyền chia sẻ, chưa biết nếu chưa khai | P0 |
| M08 | Familiarity theo người/nơi | City và check-in không nói người đã quen nơi đến đến đâu | Tự khai lần đầu/đã ghé/sống tại đây; đã biết/đã thử/muốn thử lại | P1 |
| M09 | Sắc thái phù hợp trải nghiệm | Category café/vui chơi quá rộng | Tương tác, nhịp, indoor/outdoor, gắng sức, điều kiện tham gia với evidence | P1 |
| M10 | Chi và thời gian di chuyển theo ngữ cảnh | Khoảng cách hoặc travel_minutes tĩnh không đủ theo mode/ngày | Điểm đầu/cuối, mode, duration/cost estimate, nguồn và giới hạn ước tính | P0 cho kiểm lịch/chi |
| M11 | Plan đa ngày và sửa cục bộ | Stop AI hiện chỉ place/time_text/note; itinerary UI có nền phong phú hơn | Day, duration, stable IDs, revision/lock, activity reference, facts đã kiểm | P0 trước hứa multiday/edit |
| M12 | Feedback và exposure cho recommendation | Check-in/bookmark không phải nhãn thích | Phương án đã hiện/thứ tự, giữ/đổi/bỏ, lý do tùy chọn, có đi/thích/muốn thử lại | P1 thiết kế; thu sau consent |

Không cần tạo mọi field trong onboarding. M05–M08 có thể khởi đầu bằng brief
tạm thời do người dùng xác nhận; chỉ lưu thành lịch sử/gu khi có nhu cầu và consent.
Không infer tình cảm từ `pair`, income từ chi tiêu hay gu từ raw chat.

## 4. Dữ liệu cá nhân hóa chưa có corpus trong DB local

| Dữ liệu | Số đo snapshot | Ý nghĩa đúng |
|---|---:|---|
| People | 1 | Không chứng minh có người dùng thật |
| Người có city / budget band | 0 / 0 | Chưa có tín hiệu tự khai này trong snapshot |
| Person interests | 0 | Chưa có corpus gu onboarding |
| Saved places | 1 | Một bookmark không đủ suy thích |
| Outings / stops | 0 / 0 | Chưa có lịch sử cuộc đi để đánh giá kết quả |
| Memories, gồm check-in | 0 | Không có corpus ghé địa điểm ở snapshot |
| Pair constraints | 0 | Không có dữ liệu ràng buộc ở snapshot |
| AI invocations | 0 | Chưa có log lượt gọi để phân tích chất lượng |

Không “bổ sung” bằng hồ sơ giả rồi gọi là lịch sử thật. Synthetic data dùng cho
test/demo phải có nhãn. Cold start giải bằng brief, facts và feedback tự nguyện;
chưa có cơ sở chọn collaborative filtering/fine-tuning từ DB này.

## 5. Đã có trong code/schema nhưng flow AI chưa khai thác đúng

| ID | Khoảng trống pipeline | Việc cần xử lý | Loại việc |
|---|---|---|---|
| F01 | Model chỉ nhận 9 trường địa điểm; bỏ description/kinds/traits/activities/provenance | Thiết kế payload có chọn lọc và evidence, không gửi cả catalogue mù quáng | Contract/pipeline; không tự cần migration |
| F02 | Hợp interest và trung bình budget nhóm | Giữ utility/constraint từng người đã cho chia sẻ, không suy đồng thuận từ trung bình | Recommendation/consent |
| F03 | Saved places và profile hành vi không được worker plan gọi | Quyết định tín hiệu nào được phép dùng và ý nghĩa; save/check-in không bằng thích | Product/consent/pipeline |
| F04 | Worker dùng destination mặc định khi truyền nil | Resolve nơi từ brief, hỏi khi chưa rõ, kiểm supply trước retrieval | Orchestration |
| F05 | Grounding hiện kiểm schema/ID/card limits | Thêm kiểm giờ/giá/suất/điều tránh; unknown không được pass thành fact | Domain validation |
| F06 | Route scheduler và UI day/lock/revision chưa nối vào vòng AI | Contract plan/patch, kiểm phần phụ thuộc và bảo toàn lock | Vertical slice |

Các file bằng chứng:
[place record](../../../services/core/internal/repo/places.go),
[catalogue](../../../services/core/internal/service/catalogue.go),
[worker](../../../services/core/internal/chatassist/worker.go),
[AI prompt/schema](../../../services/api/app/api/companion_gemini.py),
[grounding](../../../services/core/internal/domain/companion/companion.go),
[scheduler](../../../services/core/internal/domain/itinerary/itinerary.go).

## 6. Gói dữ liệu đầu tiên đề xuất

Một vùng pilot, đủ venue ăn uống/ngồi cùng nhau và activity thực tế để tạo
phương án khác nhau. Không chốt số điểm tùy ý: đo đủ supply bằng corpus brief
pilot, bao gồm budget/time khác nhau và case không có kết quả.

Điểm/hoạt động được dùng cho một **plan đã kiểm khả thi** cần:

1. ID và liên kết venue/activity rõ, địa lý/chỉ dẫn tới nơi đủ dùng.
2. Giá phù hợp phạm vi chi, giờ/ngày/suất áp dụng; duration và booking requirement.
3. Nguồn được phép dùng, evidence đủ kiểm, thời điểm xác minh và trạng thái còn hiệu lực.
4. Điều kiện tham gia liên quan brief; không tự xác nhận dị ứng/tiếp cận/availability.
5. Brief và constraints đã xác nhận, route/cost estimate có giới hạn được nêu rõ.

Nếu thiếu một fact quyết định, trả draft có nhãn chờ kiểm hoặc hỏi thêm; không
coi thiếu giá là miễn phí, thiếu giờ là mở hoặc thiếu capacity là còn chỗ.
Một địa điểm không có rating/ảnh vẫn có thể đủ điều kiện nếu các facts cần thiết
đã được xác minh. Không coi API cũ `open_now` là bằng chứng mở ở thời điểm tới.

## 7. Tiêu chí đóng từng issue

- D01–D03/M01–M04: có nguồn, quyền dùng, coverage trong pilot; kiểm được giá,
  giờ và activity instance, unknown/stale/conflict được giữ nguyên nghĩa.
- D04: xác định nguồn/importer/image và quyền dùng venue/ảnh theo nguồn tương ứng.
- D10/F04: chọn đúng destination từ brief; trả thiếu supply minh bạch; đo riêng
  từng vùng, không lấy tổng POI để tuyên bố phủ toàn quốc.
- M05–M07/F02: case khác budget/gu giữa thành viên giữ được ràng buộc từng người,
  theo consent; không bị trung bình hóa mất veto.
- M10–M11/F05–F06: kiểm bằng facts, lịch và chi thực tế trong phạm vi; sửa một
  phần giữ revision/lock, có case bất khả thi và dữ liệu thiếu.
- M12/F03: định nghĩa exposure/action/outcome và retention; phân biệt đã biết,
  đã đi, thích và muốn thử lại; chưa tuyên bố học hành vi khi corpus chưa đủ.

Schema/persistence mới cần Go/SQL, contract và test PostgreSQL thật theo quy
tắc repo; Python chỉ extraction/inference/evaluation. AI không tự quyết quyền
truy cập/ghi domain. Chat v2 E2EE không fallback plaintext để lấy gu.

Mọi mục còn **OPEN**. Trong lượt này chỉ lập tài liệu từ audit đã có, kiểm đường
dẫn và whitespace; chưa thêm dữ liệu, migration, collector hay nguồn external.
