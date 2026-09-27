# Bàn giao DB/data: requirements cho search và AI planner

Ngày: 27/09/2026. Trạng thái: **ĐỀ XUẤT — cần phía DB/data phản hồi khả năng đáp ứng**.
Đây là entry point của PR; chưa duyệt schema/API, chưa triển khai migration.
Người yêu cầu muốn nhận đầy đủ requirements; ưu tiên dưới đây là thứ tự thực
hiện, không phải loại phần khó khỏi phạm vi. Mục chưa làm được cần phản hồi lý
do, nguồn còn thiếu, phương án thay thế và khả năng trả dữ liệu unknown.

## 1. Kết quả cần đạt

Search cần tìm được địa điểm và **trải nghiệm thực sự phù hợp với brief**, gồm
casual/du lịch, người quen nơi đến/lần đầu, đi với bạn/cặp đôi, mức chi và nhịp
khác nhau. Planner cần facts đủ để kiểm lịch/chi/điều kiện, có provenance và độ
mới. Không chỉ tăng số địa điểm hoặc thêm một trường embedding.

Đề nghị bên DB/data phản hồi từng ID ở mục 4–8 theo bảng mục 10. Bao gồm cả
schema, supply, ingestion, query và chất lượng dữ liệu; phần cần app/AI/domain
phối hợp được ghi rõ ở mục 9. Không giao toàn bộ personalization cho DB tự suy.

## 2. Bằng chứng và phạm vi

Snapshot DB **local**, khoảng 14:05 giờ Việt Nam ngày 27/09/2026:
9.363 địa điểm; 33 destination, 8 có địa điểm; giá/giờ/rating/group_fit đều
thiếu; activities/traits không có dữ liệu không rỗng. Có description/kinds,
nhưng importer `vnlocal` chưa xác minh trong checkout đã khảo sát. Chưa có
corpus hành vi để học recommendation. Không suy hiện trạng production từ đây.

Code được nghiên cứu tại HEAD `c8f092c1` và working tree có thay đổi chưa commit.
PR tài liệu được tách trên `origin/main` tại `7ea1a7c2`; cần đối chiếu runtime và
migrations mới nhất trước implementation. Không coi journal là đặc tả runtime.

Đọc kèm:

- [Issue dữ liệu thiếu, ưu tiên và tiêu chí đóng](khoang-trong-du-lieu-ai-planner.md).
- [Inventory local và dữ liệu planner đang dùng](insight-va-du-lieu-ca-nhan-hoa-plan.md).
- [Flow code tạo plan hiện tại](nghien-cuu-ai-tao-plan.md).
- [Insight người dùng và hướng thuật toán](phan-tich-gioi-tre-viet-va-thuat-toan-planner.md).

Không có dữ liệu cá nhân, body chat, ảnh hóa đơn hoặc export DB trong PR.

## 3. Quy ước chung cho contract dữ liệu

- Field names ở tài liệu là tên gợi ý; phía DB có thể đề xuất mapping vào schema
  hiện có. Phân biệt giá trị không biết, không áp dụng, không cung cấp và false/0.
- VND là integer. Không chuyển luật tiền hoặc ledger. Giá tham khảo/ước tính
  khác giá đã công bố; không dùng range mơ hồ để bảo đảm nằm trong budget.
- Facts, inference và preference match là ba lớp riêng. LLM extraction có
  evidence chưa tự trở thành fact đã xác minh.
- Dữ liệu phụ thuộc thời gian có thời điểm áp dụng/kiểm, timezone và expiry.
  `updated_at` của record không chứng minh mọi thuộc tính vừa được xác minh.
- Một venue có nhiều activity; một activity có nhiều session/offer. Không gộp
  tên venue, loại trải nghiệm và suất bán cụ thể thành một chuỗi mô tả.
- ID ổn định, reference và version giúp dedup, sửa plan, kiểm lại facts. Không
  dùng tọa độ gần nhau làm bằng chứng hai hoạt động là một.
- Go/SQL sở hữu API, persistence, auth, ingestion orchestration và workers.
  Python chỉ AI extraction/inference/evaluation. Mỗi module có một writer.

## 4. Requirements dữ liệu venue và experience

| ID | Requirement | Nội dung cần đáp ứng | Ưu tiên |
|---|---|---|---|
| V01 | Định danh và tên tìm kiếm | Stable place ID, tên hiển thị, aliases/tên không dấu/tên nguồn; quan hệ chi nhánh, trùng nguồn và trạng thái hoạt động | P0 |
| V02 | Địa lý thực tế | Lat/lng có nguồn/độ chính xác; địa chỉ/lối vào; destination và khu vực hành chính/khu tìm kiếm với mapping có version khi thay đổi | P0 |
| V03 | Taxonomy đủ chi tiết | Café, món/ẩm thực, bảo tàng, di tích, cảnh quan, workshop, game, thể thao…; nhiều loại khi phù hợp; taxonomy version và mapping nguồn | P0 |
| V04 | Giá có đơn vị | Giá/range VND, người/nhóm/vé/session, minimum spend, khoản bao gồm/ngoài giá, ngày áp dụng, estimate hay advertised | P0 |
| V05 | Lịch sử dụng | Giờ theo ngày/interval, timezone, đóng nghỉ/ngoại lệ, last entry, thời gian phục vụ; overnight hours, unknown và tạm đóng | P0 |
| V06 | Duration | Thời lượng activity/khuyến nghị visit và range, nguồn hoặc đánh dấu estimate; không tự coi mọi stop 60 phút | P0 cho lập lịch |
| V07 | Venue → activity | Inventory hoạt động thực tế, mô tả làm gì, người tổ chức, venue reference và trạng thái có hiệu lực; một nơi có nhiều trải nghiệm | P0 cho activity |
| V08 | Activity → session/offer | Ngày/giờ, duration, giá/đơn vị, capacity/availability nếu nguồn có, thời điểm kiểm; không suy còn chỗ từ capacity | P0 cho suất cố định |
| V09 | Đặt trước và điều kiện mua | Có cần đặt, deadline/lead time, booking/contact URL chính thức, chính sách thay/hủy nếu công bố; ghi rõ không có feed realtime | P0 cho activity cần đặt |
| V10 | Điều kiện tham gia nhóm | Số người min/max, private/shared session, ngôn ngữ hướng dẫn, yêu cầu tuổi/kỹ năng, thiết bị cần có, mức gắng sức khi nguồn nêu | P1; điều kiện bắt buộc là P0 |
| V11 | Sắc thái trải nghiệm có evidence | Cùng làm/chủ yếu xem/trò chuyện; indoor/outdoor, tiếng ồn theo nguồn/khung giờ, seating, thời gian ngồi; không suy từ ảnh/tên | P1 |
| V12 | Ăn uống và khả năng tiếp cận | Món/ẩm thực, chế độ ăn/menu và chính sách dị ứng nếu công bố; accessibility cụ thể; unknown không bằng phù hợp | P1; constraint người dùng là P0 |
| V13 | Bản địa/văn hóa | Đặc trưng món/nghề/câu chuyện/di sản có nguồn; phân biệt tourist landmark, trải nghiệm văn hóa, local food với category chung | P1 |
| V14 | Ảnh và review | URL/source/license/attribution, ngày; rating kèm scale/count/source, review verified chỉ khi nguồn xác nhận; không trộn điểm khác scale | P2 |
| V15 | Sự kiện/hoạt động có thời hạn | Sự kiện local, triển lãm/festival/pop-up có ngày, địa điểm, vé, người tổ chức và cập nhật hủy/đổi lịch | P1 |
| V16 | Crowd/queue nếu có nguồn | Estimated crowd/wait theo thời điểm, freshness và uncertainty; không gắn “ít đông” vì ít review | P2 |
| V17 | Logistics phụ thuộc trải nghiệm | Meeting point, parking/access/transport liên quan, thời gian check-in/setup, điều kiện thời tiết/đổi lịch nếu công bố | P1; hạn chế bắt buộc là P0 |

V12 không yêu cầu DB tự xác nhận một món an toàn dị ứng. Chỉ lưu thông tin nguồn
công bố và trạng thái cần hỏi venue; app/domain phải tránh cam kết không có bằng chứng.
Các fact noise/crowd có thể đổi theo thời điểm, không xem là traits vĩnh viễn.

## 5. Provenance, ingestion và chất lượng

| ID | Requirement | Nội dung cần đáp ứng | Ưu tiên |
|---|---|---|---|
| Q01 | Xác minh nguồn hiện có | Đối chiếu `vnlocal`, importer/image/schema version, source_ref và quyền dùng venue/ảnh/mô tả; báo phần không xác minh được | P0 |
| Q02 | Evidence cấp thuộc tính | source URL/ref, observed_at/checked_at, validity, evidence reference, verified/inferred/unknown/conflict và confidence có nghĩa rõ | P0 |
| Q03 | Quyền nguồn | License/terms và attribution đúng cấp nguồn/asset; hạn chế lưu, cache, gửi AI và redistribution nếu có; nguồn thiếu quyền không tự reuse | P0 |
| Q04 | Refresh theo volatility | Giá, giờ, session và availability có chu kỳ/expiry khác description; refresh failures, tombstone và last good data có nhãn stale | P0 |
| Q05 | Dedup và entity resolution | Merge đa nguồn có audit/mapping; không mất source identity; tách chi nhánh/activity/session; giải quyết giá/giờ xung đột minh bạch | P0 |
| Q06 | Validation nhập | VND/unit hợp lệ, lat/lng, hours/timezone, duration, session/date, reference và range; reject/quarantine record sai thay vì điền giả | P0 |
| Q07 | Extraction có kiểm | Extraction text bằng AI kèm evidence, version model/rule và trạng thái review; không tự bổ sung giá/giờ/workshop bằng kiến thức model | P1 |
| Q08 | Coverage và báo chất lượng | Theo destination/category/activity: số ứng viên đủ facts, missing/stale/conflict, nguồn chưa rõ; raw POI count và verified count riêng | P0 |
| Q09 | Theo dõi thay đổi | Version/audit facts dùng trong plan; phát hiện tạm đóng/hủy suất/đổi giá để planner có thể kiểm lại, không silent overwrite meaning | P1 |
| Q10 | Sửa dữ liệu có căn cứ | Workflow flag/correction/review, quyền writer rõ, nguồn và lịch sử; không coi một report người dùng là fact đã duyệt | P1 |

Đề nghị data team nói rõ field nào lấy được trực tiếp, field cần extraction,
field cần đối tác/venue xác nhận và field không có nguồn khả thi. Không yêu cầu
scrape trái quyền hoặc mua một provider cụ thể trong PR này.

## 6. Requirements search/query/index

Đây là yêu cầu hành vi; DB/data đề xuất SQL/index/search engine phù hợp, không
chốt thêm dịch vụ hay vector DB trước benchmark.

| ID | Requirement | Hành vi cần có | Ưu tiên |
|---|---|---|---|
| S01 | Search tiếng Việt | Có dấu/không dấu, alias, loại/món/hoạt động; thử typo phổ biến và từ đồng nghĩa; không chỉ exact name | P0 |
| S02 | Search theo ý định | Query như “cùng làm gì đó”, “ngồi trò chuyện”, “tìm hiểu lịch sử”; trả evidence/category match và uncertainty | P1 |
| S03 | Geo retrieval | Destination/area/radius hoặc bounding box; distance từ điểm bắt đầu; sắp theo địa lý đúng và có fallback khi thiếu supply | P0 |
| S04 | Lọc theo thời điểm | Interval đủ duration, ngày/ngoại lệ/overnight, activity session; trả trạng thái thiếu giờ thay vì mặc định mở | P0 |
| S05 | Lọc theo giá có scope | Per-person/group/session với số người, inclusions và required fees; separate unknown; không lọc nhầm quote nhóm thành giá/người | P0 |
| S06 | Lọc theo activity/điều kiện | Loại, suất/ngày, group size, booking requirement, indoor/outdoor và hạn chế có evidence; unknown policy explicit | P0 cho điều kiện bắt buộc |
| S07 | Multi-channel candidate | Text/taxonomy/geo và semantic nếu cần; quota theo intent/experience, dedup; không để một category chiếm hết top K | P1 |
| S08 | Novelty/familiarity input | Nhận danh sách đã biết/đã đi/exclude từ app theo quyền; novelty theo người, không dùng unpopular thay cho novel | P1 |
| S09 | Evidence payload | Stable IDs, đủ facts/units/provenance/freshness cho ranker và validator; field projection và batch lookup theo IDs | P0 |
| S10 | Pagination và ổn định | Cursor/stable ordering/tie-break, bounded candidate count, filter consistency; không thay thứ tự tùy DB query plan | P0 |
| S11 | Dữ liệu thiếu và empty results | Trả missing reason, coverage, unknown/stale flags, applied filters; gợi ý nới filter nhưng không tự bỏ hard constraint | P0 |
| S12 | Benchmark truy vấn | EXPLAIN/query plan, indexes, latency p50/p95 với catalogue/queries đại diện; thống nhất target sau baseline, không đặt SLA chưa đo | P0 trước release |
| S13 | So sánh chiến lược search | Baseline SQL/text vs semantic/hybrid; relevance theo người đánh giá, recall@K ứng viên hợp constraints, theo destination/intent | P1 |
| S14 | Freshness và cache | Cache key chứa filters/time/versions phù hợp; invalidation đổi facts/session; không serve availability cũ như realtime | P1 |
| S15 | Search an toàn với context | Filters/exclusions theo quyền được Go kiểm; description/review untrusted, không trở thành instruction cho AI | P0 |

Không yêu cầu S04/S05 tự bảo đảm **toàn hành trình**: search cung cấp facts và
candidate hợp điều kiện cục bộ; Go validator/solver kiểm tổng chi/thời gian và
di chuyển. Không suy thời gian thực tế từ đường chim bay.

Một nhóm candidate có thể gồm record chưa đủ facts cho discovery, nhưng API
phải phân biệt rõ với candidate đủ điều kiện cho verified plan. Keyword/embedding
match không đủ chứng minh giờ, giá, localness hoặc activity còn hoạt động.

## 7. Dữ liệu cá nhân hóa và nhóm — cần contract/consent phối hợp

| ID | Requirement | Dữ liệu tối thiểu | Ưu tiên |
|---|---|---|---|
| P01 | Brief theo lần đi | Intent, nơi/activity muốn thử, khoảng giờ, participants, mode, pace, mức mới; chỉ lưu khi phù hợp | P0 |
| P02 | Budget constraint đúng đơn vị | Per-person/group, day/trip, inclusions, hard ceiling vs preference; không suy từ income hay ledger | P0 |
| P03 | Gu/ràng buộc từng người | Interest/avoidance/mandatory và scope consent; không chỉ union tag/average budget | P0 |
| P04 | Familiarity | Tự khai theo người/destination/khu/hoạt động; đã biết/đã thử/thích/muốn thử lại tách biệt | P1 |
| P05 | Preference có sắc thái | Mục đích café/activity, ngưỡng đi xa/chờ/đặt trước, interaction preference; tạm thời vs bền vững | P1 |
| P06 | Exposure/action/outcome | Đã hiển thị, thứ tự/policy/revision, giữ/đổi/bỏ với lý do tùy chọn; có đi và đánh giá tự nguyện | P1 |
| P07 | Consent và lifecycle | Allowed purpose/scope, quyền xem/chia sẻ, opt-in, revoke/delete/retention và version; không lưu raw chat cho recommender | P0 trước dùng lịch sử |
| P08 | Roster đúng buổi | Người thực sự tham gia khác thành viên nhóm; chưa khai gu là unknown, không utility bằng 0 | P0 |

City/budget/interests/bookmark/check-in có một số schema/API hiện có. Đề nghị
map/reuse sau audit thay vì thêm bảng trùng. Pair notebook có boundary riêng;
DB có dữ liệu không đồng nghĩa AI được phép đọc. Không suy romance từ pair,
không tự đọc chat E2EE hoặc tự lấy excerpts chưa được đồng ý.

## 8. Acceptance cases bắt buộc cho bàn giao

Đề nghị phía DB/data cung cấp fixtures tổng hợp/public hợp quyền và expected
facts do người kiểm; contract và persistence mới có test PostgreSQL thật.

1. “Cà phê/cafe/ca phe” và aliases tìm cùng entity; không trả trùng chi nhánh.
2. Query workshop gốm trả activity có thật, venue/session reference và evidence;
   không dùng demo seed làm supply thị trường.
3. Search khoảng 19:00–21:00 không coi venue đóng 20:00 đủ cho activity 90 phút
   bắt đầu 19:00; overnight/holiday/last-entry có expected cases riêng.
4. Giá nhóm và giá/người lọc đúng với headcount; null không bằng 0; required
   fee/inclusions có nghĩa rõ; tổng budget của plan vẫn do domain kiểm.
5. Availability chưa có feed trả unknown; capacity 10 không chứng minh còn 10 chỗ.
6. Nơi gần nhưng thiếu facts, nơi đủ facts nhưng xa: filters và lý do trả rõ;
   thiếu tọa độ không được tự xếp thành gần nhất.
7. Không có kết quả khi giữ hard constraints: trả empty/coverage/missing reason,
   không silent relax giá/giờ hoặc chuyển destination không được chọn.
8. Hai nguồn có giá/giờ khác nhau: giữ evidence, chính sách xung đột và freshness;
   venue đóng/session hủy không tiếp tục được coi hợp lệ vì cache.
9. Casual tại nơi quen và du lịch quay lại vẫn tìm được activity mới với người;
   du lịch lần đầu vẫn tìm được di tích/điểm tiêu biểu; không ép preset trái ngược.
10. Hai người khác gu/ngưỡng chi: giữ constraints riêng theo consent; revoke
    thông tin thì lần truy vấn sau không dùng payload/cache vượt quyền.
11. Saved/check-in không thành liked; “đã biết” không đồng nghĩa “không thích”.
12. Explainability trả evidence nào khớp, facts nào thiếu; không bịa yên tĩnh,
    local/hidden gem hoặc an toàn dị ứng từ category/rating.
13. Kiểm riêng coverage/latency/relevance theo vùng và loại trải nghiệm; một
    destination nhiều POI không che vùng khác không có verified candidate.

## 9. Phân chia phần việc theo boundary kỹ thuật

- **DB/data:** audit schema và nguồn, modelling, ingestion/refresh, provenance,
  quality, SQL/index/query, payload facts và persistence lifecycle.
- **Go/domain/API:** auth/consent, brief và unit validation, orchestration,
  search contract, aggregate budget/scheduling, revision/locks/idempotency.
- **Python AI:** intent/extraction với evidence, proposal/ranking thử nghiệm và
  evaluation; không tự write domain hoặc quyết quyền sử dụng dữ liệu.
- **Mobile:** nhập/xác nhận brief, cung cấp thông tin tự nguyện, explainability,
  chọn/sửa plan và feedback. Không đẩy toàn bộ yêu cầu thành onboarding dài.

Đây là boundary của công việc, không thay đổi quy tắc engineer sở hữu vertical
slice. Phase triển khai đề xuất: facts/search hợp lệ → personalization/group →
multiday/edit → học từ feedback khi đủ corpus. Tất cả requirements vẫn giữ lại.

## 10. Mẫu phản hồi phía DB/data

Vui lòng phản hồi theo từng ID hoặc nhóm ID có cùng giải pháp:

| ID | Có sẵn / làm được / chưa làm được | Mapping/schema hoặc nguồn dự kiến | Thiếu gì / phụ thuộc | Phương án thay thế / trạng thái unknown | Acceptance evidence |
|---|---|---|---|---|---|
| Vxx/Qxx/Sxx/Pxx | … | … | … | … | … |

Kèm các quyết định cần chốt: nguồn chính thức cho giá/giờ/activity, vùng và độ phủ
khởi đầu, refresh/freshness, taxonomy, scope consent, search strategy và target
latency sau baseline. Mục chưa có nguồn cần nói rõ thay vì hứa model suy ra được.

## 11. Phạm vi PR và kiểm tra

PR này chỉ thêm requirements và bốn báo cáo nghiên cứu hỗ trợ. Không có schema,
collector, runtime, frontend hay dữ liệu thật được đưa vào Git. Kiểm đường dẫn
Markdown local, whitespace và repo guard trên branch tách riêng; chưa chạy
evaluation chất lượng search, user study hay migration PostgreSQL.
