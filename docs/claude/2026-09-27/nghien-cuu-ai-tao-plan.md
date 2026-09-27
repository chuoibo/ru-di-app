# Nghiên cứu AI tạo plan: hiện trạng và hướng đi đầu tiên

Ngày: 27/09/2026. Khảo sát checkout tại HEAD `c8f092c1`, **có thay đổi chưa commit**
từ công việc đang diễn ra. Đây là báo cáo đọc code và nghiên cứu, không phải ADR,
bằng chứng production hay kết quả kiểm thử model mới. Không sửa runtime.

## 1. Kết luận đề xuất

Xây một bộ lập kế hoạch hiểu **những người thực sự tham gia**, **mục đích cuộc đi**
và **ràng buộc đã xác nhận**. AI hiểu lời nhờ, đề xuất trải nghiệm và giải thích;
Go kiểm dữ kiện, ngân sách, lịch và tính khả thi. Người dùng sửa và xác nhận.

Hai trục độc lập: Hội bạn/Cặp đôi có đồng thuận, và Đi chơi/Du lịch.
Không suy quan hệ từ số người hoặc `context.kind = pair`; không suy du lịch
chỉ từ số ngày. Có chuyến du lịch trong ngày và cuộc đi casual kéo qua nửa đêm.
ADR-0037 đã xác lập các loại quan hệ và quyền người dùng xác nhận loại cuộc đi.

Điểm khác biệt cần kiểm chứng bằng người dùng: một plan cân bằng mong muốn của
từng người, có nhịp phù hợp, giải thích được đánh đổi và sửa cục bộ được.
Đây là giả thuyết sản phẩm, chưa phải bằng chứng về tính độc nhất trên thị trường.

## 2. Flow đang chạy trong code

### Từ chat tới nháp AI

1. Mobile nhận `/plan`, `@Rủ Đi` hoặc lời nhờ trong khay công cụ.
   `apps/mobile/src/rudi/chat/ai-invocations.ts` có `docLenhAi`, `goiAi`.
2. `SoHen.tsx` hiển thị gói chat sẽ gửi; người dùng có lựa chọn chỉ gửi lời nhờ.
   Gói chỉ đính kèm khi capabilities công bố `caller_attached`.
3. `POST /contexts/{context}/ai-invocations` gửi `logical_id`, `command`, `prompt`,
   `boi_canh` tùy chọn. Chưa có brief plan theo trường riêng.
4. Go `chatassist/handler.go` kiểm phiên, membership, giới hạn và idempotency;
   `boicanh.go` giới hạn 40 lượt, 300 ký tự mỗi lượt. Nội dung do client trao,
   server kiểm metadata tin thuộc đúng phòng, không tự đọc body tại đường này.
5. `chatassist/worker.go` nhận job, dựng roster, hồ sơ gu/ngân sách và catalogue.
   Hai worker; inference có timeout. Job cá nhân chuyển sang Nếp riêng.
6. Go gọi brain action `companion-reply`; Python `companion_gemini.py` gọi Gemini.
   Mặc định trong code: `gemini-3.5-flash-lite`; `MOBILE_GEMINI_MODEL` có thể đổi.
   Không xác minh cấu hình model của deployment hiện tại.
7. Model trả một thẻ `text`, `places` hoặc `itinerary`. Schema itinerary chỉ có
   `title`, các stop gồm `place_id`, `time_text`, `note`. Tối đa 6 stop;
   thẻ địa điểm tối đa 5 chỗ. Temperature đang là 0.0.
8. Go `domain/companion.GroundCard` kiểm cấu trúc, ID thuộc catalogue, giới hạn
   chữ/số chặng; gắn dữ kiện địa điểm từ catalogue rồi publish thẻ vào phòng.

### Từ nháp tới kèo và lịch trình

9. `CreateOutingLive.tsx` lấy title/chặng từ thẻ. Người dùng xác nhận ngày đi,
   ngày về, số người, ngân sách; sửa giờ nếu AI trả giờ chưa đúng `HH:MM`.
   Ngày mặc định hôm nay; số người mặc định số thành viên nhóm.
10. `POST /contexts/{context}/plan-promotions` kiểm nguồn thẻ, giờ, địa điểm,
    chống tạo trùng và ghi outing/chặng trong transaction Go/PostgreSQL.
    Stop promotion chưa có trường ngày riêng, thời lượng hay phương tiện.
11. `hanh-trinh/SoHanhTrinh.tsx` và `ke-hoach.ts` mới cung cấp draft theo ngày,
    thời lượng, khóa giờ, điểm hẹn, phương tiện, sửa/xóa/đổi thứ tự, revision.
12. Preview itinerary dùng Valhalla, tính thời gian/đường đi và có đề xuất đổi
    thứ tự. `source.traffic = none`: không phải thời gian có traffic trực tiếp.
    Bộ này **chưa được gọi ở pipeline tạo thẻ AI**. Nó không thay kiểm giờ
    mở cửa, khẩu phần, ngân sách hay mức độ phù hợp trải nghiệm.

Tab `PlanLive.tsx` chủ yếu đọc danh sách cuộc đi. Màn fixture `Group.tsx` có
lịch trình mẫu; không dùng fixture để kết luận khả năng production.

### Những đường recommendation khác vẫn tồn tại

`routes/suggestions_wai.go` còn hai route Go-owned trong ownership manifest:

- `GET /contexts/{context_id}/suggestion`: tổng hợp lịch sử chuyến/check-in,
  gọi brain `suggestion` để đề xuất buổi đi chơi. Đây là pipeline riêng.
- `GET /contexts/{context_id}/contextual-suggestion`: đọc `ListMessages`,
  tổng hợp các dòng chat rồi gọi brain `contextual-suggestion`.

Đường thứ hai là một **điểm lệch với nguyên tắc server không tự đọc chat cho AI**
trong ADR-0036/0037. Khẳng định ở đây là từ handler và manifest, chưa probe
deployment. Có adapter mobile `screens/ai-hieu-nhom/ai-hieu-nhom.ts`, nhưng tìm
tham chiếu chưa thấy màn hiện hành gọi `napAiHieuNhom`; không kết luận người dùng
đang đi qua nó. Cần xử lý/đóng đường lệch trước khi dùng làm nền planner mới.

Đường `chatassist` được khảo sát còn công bố `protocol: legacy`, yêu cầu
`g.kind == group`, từ chối phòng đã chuyển v2 bằng `encrypted_invocation_required`.
Chưa thể coi nó là flow planner chung đã hỗ trợ cặp đôi và chat E2EE.

## 3. Cá nhân hóa hiện tại thực chất đến đâu?

`service.GroupTaste` đọc sở thích và budget band của các thành viên active.
`taste.ProfileForGroup` lấy **hợp các tag** và **trung bình nguyên các midpoint
budget band có trả lời**. Không có trọng số người hay veto trong profile này.
Người chưa trả lời không được coi là chi 0 đồng.

`scoring.ScorePlace` chấm từng địa điểm với trọng số: ngân sách 40, gu 35,
khoảng cách 15, sức chứa 10; thiếu dữ kiện thì bỏ term và chuẩn hóa lại.
Điểm này là heuristic có giải thích, không phải xác suất người dùng thích.

`service.ModelPlaceRows` lấy điểm đến mặc định bằng `DestinationOrDefault(..., nil)`:
điểm đến đầu tiên theo `sort_order, id`. Seed đặt Đà Lạt đầu tiên; DB runtime
chưa được đọc trong lượt này. Sau đó xếp theo điểm gu nhóm và lấy 40 dòng.
Địa điểm không được truy xuất theo destination trong lời nhờ.

Model chỉ nhận 9 trường địa điểm: ID, tên, địa chỉ, giá min/max, rating,
khoảng cách, giờ mở cửa, category. Không nhận traits/kinds, group capacity,
metadata nguồn/độ mới hay evidence riêng về độ ồn, thời lượng, accessibility.
`members` chỉ có tên hiển thị. Tag sở thích từng người không được gửi trực tiếp;
gu tác động gián tiếp qua thứ tự catalogue và trực tiếp qua chat được chia sẻ.

Hệ quả:

- Chưa phân biệt người ở trong phòng với người thực sự đi.
- Hợp các tag làm mất thông tin ai thích gì, độ mạnh và điều không chấp nhận.
- Budget trung bình nhóm không phải mức trần từng người, cũng không phải budget
  đã xác nhận cho lần đi này.
- Top 40 địa điểm riêng lẻ dễ thiếu loại hoạt động cần cho toàn plan.
- Rating và match cao chưa chứng minh một chuỗi chặng có trải nghiệm hay.

## 4. Các khoảng trống làm plan chưa chuẩn chỉnh

| Khoảng trống | Tác động | Ưu tiên |
|---|---|---|
| Catalogue theo điểm đến mặc định | Lời nhờ thành phố khác không có ứng viên phù hợp | P0 |
| Chưa có brief có cấu trúc | Model đoán mục đích, phạm vi tiền, người đi | P0 |
| Grounding chỉ kiểm ID/shape | Địa điểm có thật nhưng lịch vẫn không đi được | P0 |
| Ngân sách kiểm mỗi chỗ trong prompt | Nhiều chỗ đều dưới budget nhưng tổng vượt budget | P0 |
| Giờ và diet chủ yếu giao prompt kiểm | Không có validator nghiệp vụ đảm bảo | P0 |
| Plan tối đa 6 stop và giờ dạng chữ | Chưa có plan nhiều ngày đủ cấu trúc | P0 cho du lịch |
| Planner group-only; E2EE bị từ chối | Chưa có một flow thống nhất cho bốn tình huống | P0 về phạm vi |
| Chưa có evidence về vibe/nhịp | Dễ cá nhân hóa bằng lời văn mà lịch không khác | P1 |
| Chưa có sửa AI cục bộ giữ phần đã chọn | Đổi một chặng dễ làm mất ý đồ ban đầu | P1 |

Prompt hiện yêu cầu kiểm giờ, midpoint giá và dị ứng. Đó là lời yêu cầu model,
không phải phép kiểm của server. Đặc biệt không thể xác nhận an toàn dị ứng chỉ
từ tên/category quán: thiếu dữ kiện thì phải ghi chưa xác minh, chọn phương án
khác hoặc yêu cầu xác nhận với địa điểm, không biến suy luận thành bảo đảm.

Nhật ký `docs/claude/2026-09-24/chot-dot-ai-engine-va-dock-nep.md` ghi lần đo
10/16 trên flash-lite, một lượt/ca; lỗi có dị ứng, giờ/giá và hỏi lại. Đây là
**số lịch sử trong báo cáo**, không phải lần đo tôi chạy lại ở checkout này.
Corpus hiện có dùng được để mở rộng, chưa đủ đánh giá đủ bốn loại plan.

## 5. Nghiên cứu bên ngoài và điều áp dụng được

- [Google Research, 06/06/2025](https://research.google/blog/optimizing-llm-based-trip-planning/):
  kết hợp LLM hiểu mục tiêu mềm với thuật toán xử lý thời gian, giờ mở cửa và
  đường đi. Có bước lấy ứng viên thay thế và tối ưu theo ngày/toàn chuyến.
  Áp dụng: thêm kiểm/tối ưu vào Go trước khi trình bày plan, giữ AI cho ý đồ.
<!-- repo-guard: allow=long-number reason=public-research-source-identifier -->
- [TravelPlanner, 2024](https://arxiv.org/abs/2402.01622): benchmark đặt bài toán
  với nhiều ràng buộc đồng thời. Áp dụng: đánh giá toàn plan theo brief,
  không chỉ JSON hợp lệ hoặc địa điểm tồn tại. Kết quả model đời cũ không
  dùng để đánh giá trực tiếp model hiện hành của Rủ Đi.
<!-- repo-guard: allow=long-number reason=public-research-source-identifier -->
- [Planning với formal verification, bản sửa 2025](https://arxiv.org/abs/2404.11891):
  nghiên cứu formal hóa ràng buộc, dùng solver và giải thích trường hợp vô nghiệm.
  Áp dụng: khi không có plan phù hợp, chỉ ra điều xung đột và hỏi người dùng
  muốn nới điều nào. Không bê tỷ lệ benchmark sang KPI sản phẩm.
- [Sequential group recommendations, 2021](https://link.springer.com/article/10.1007/s10844-021-00652-x):
  xem satisfaction/disagreement qua nhiều lượt để cân bằng thành viên.
  Áp dụng là suy luận thiết kế: cân bằng trên toàn cuộc đi; chỉ dùng feedback
  và lịch sử đã được đồng ý chia sẻ, không tự theo dõi chat.
- [Place Details API](https://developers.google.com/maps/documentation/places/web-service/place-details):
  có các trường giờ hiện tại, giờ thường lệ, giá và thuộc tính như goodForGroups.
  Áp dụng: nghiên cứu nguồn bổ sung có timestamp/provenance và mức phủ; chưa
  quyết định mua provider, chưa xác minh mức phủ ở Việt Nam hoặc điều kiện lưu.
- [Routing with time windows](https://developers.google.com/optimization/routing/vrptw):
  minh họa bài toán xếp tuyến trong khung thời gian. Đây là tài liệu tham khảo
  thuật toán, không đề xuất đưa backend nghiệp vụ sang Python hay cài solver
  vào runtime ngay. Bộ Go hiện có là điểm khởi đầu.

## 6. Thiết kế trải nghiệm cho bốn tình huống

Các mục dưới là **đề xuất mặc định để người dùng chọn**, không định kiến về quan hệ.

| | Đi chơi thường ngày | Du lịch |
|---|---|---|
| Cặp đôi | Mục đích buổi hẹn, nhịp trò chuyện/hoạt động, điều cả hai muốn, ít di chuyển nếu chọn | Cân bằng gu hai người qua ngày, thời gian riêng/nghỉ, điểm nhấn chung, nơi ở và ngày đến/về |
| Hội bạn | Người thực sự đi, khung giờ giao nhau, hoạt động cùng tham gia, mức chi phù hợp từng người | Nhịp nhóm, logistics và nơi ở, chặng chung/tùy chọn nếu nhóm muốn, không để một người liên tục phải nhường |

Ví dụ tổng hợp: hai người đều chọn cặp đôi nhưng một đôi muốn đạp xe và thử
món mới, đôi khác muốn ngồi nói chuyện ở chỗ yên. Planner phải thay chặng và nhịp,
không chỉ đổi title thành “lãng mạn”. Hai bạn đi ăn tối vẫn là hội bạn nếu đó là
quan hệ họ chọn. Du lịch trong ngày vẫn cần điểm đến, giờ đến/về, di chuyển.

### Brief trước khi tạo

Thu thập ít trường bắt buộc, mở rộng theo mục đích:

- Đi với ai: danh sách người tham gia; quan hệ từ trạng thái được xác nhận.
- Ở đâu: khu vực/điểm đến, điểm xuất phát tự chọn, nơi ở nếu có.
- Khi nào: ngày, giờ có mặt, giờ phải kết thúc, timezone; với du lịch thêm giờ đến/về.
- Tiền: tham chiếu hay trần cứng; một người hay cả nhóm; toàn cuộc đi hay mỗi ngày;
  gồm/không gồm di chuyển, vé, nơi ở. Không tự chuyển budget band thành trần.
- Muốn gì: mục đích lần này, nhịp nhẹ/vừa/dày, một điều rất muốn và điều muốn tránh.
- Ràng buộc: ăn uống, accessibility, phương tiện, đặt chỗ và các chặng đã khóa.
- Chia sẻ: đúng nguồn được dùng lần này. Cho người dùng xem và xác nhận gói;
  không tự lấy gu/chat/lịch sử làm bộ nhớ vĩnh viễn. Tôn trọng phạm vi ADR hiện hành.

Thiếu đầu vào quyết định tính khả thi thì hỏi một câu quan trọng nhất, ưu tiên
chip/field. Thiếu sở thích thì cho plan trung tính có giả định rõ, không hỏi dài.
Gu lâu dài chỉ là gợi ý; mong muốn chuyến này được xác nhận có ưu tiên cao hơn.

### Output phải là plan có cấu trúc

Mỗi chặng có ID ổn định, ngày, bắt đầu/kết thúc, thời lượng, place/activity,
chi phí ước tính theo khoảng và phạm vi, di chuyển, trạng thái khóa, nguồn dữ kiện.
Plan có ràng buộc đã dùng, giả định, phần chưa xác minh, tổng chi phí, revision.
Thẻ chat chỉ là bản tóm tắt trỏ về plan; 6 dòng trên thẻ không giới hạn số chặng
của plan. Thời gian nghỉ/di chuyển không cần giả vờ là địa điểm catalogue.

Tách estimated cost khỏi ledger thật. VND nguyên; tính tổng bằng Go, không lấy
tổng model nói làm bằng chứng. Giá thiếu không coi là miễn phí. Khi chi phí chưa
đủ căn cứ, plan là nháp cần kiểm, không mang nhãn đã đạt trần ngân sách.

### Cá nhân hóa và tùy chỉnh tạo khác biệt

- Cân bằng toàn plan: mỗi người có phần họ mong đợi; một chặng không cần phục vụ
  tất cả mọi gu. Veto cứng và ràng buộc an toàn không được đổi lấy điểm trung bình.
- Nhịp có chủ đích: mở đầu dễ gặp, hoạt động chính, nghỉ, kết thúc đúng giờ;
  ít chặng vẫn có thể tốt hơn lịch đầy. Chọn mật độ theo brief, không theo quan hệ.
- Lý do bám dữ kiện: “giữ hoạt động bạn chọn, bỏ chặng xa để đủ giờ về” hữu ích
  hơn lời khen chung. Không công khai gu riêng của người chưa đồng ý chia sẻ.
- Hai phương án có đánh đổi thật khi hữu ích: ít di chuyển/đúng budget và thêm
  trải nghiệm mới; không bắt buộc sinh nhiều thẻ cho mọi lời nhờ.
- “Giữ chỗ này”, “đổi chặng ăn”, “giảm đi bộ”, “rẻ hơn” tạo patch; chỉ ảnh hưởng
  phần cần sửa, kiểm lại phần phụ thuộc, hiện thay đổi trước khi áp dụng, có undo.
- Sau cuộc đi hỏi feedback ngắn với lý do: sai giá, ồn, lịch quá dày, không hợp,
  đóng cửa. Đổi chặng chưa chứng minh ghét chặng; check-in chưa chứng minh thích.

## 7. Hướng triển khai đầu tiên theo lát chạy trọn

### Lát 1: một buổi đi trong một khu vực có dữ liệu tốt

Chọn pilot theo mức phủ catalogue thực tế, không chọn thành phố từ thói quen
default. Có thể là buổi tối 2–4 giờ, 2–3 chặng; con số là phạm vi thử đề xuất.
Dùng brief và người tham gia cho cả hội bạn và cặp đôi theo quyền đã xác nhận.

Đường đi: nhập/xác nhận brief → catalogue đúng khu vực, đủ loại hoạt động → AI
đề xuất → Go kiểm lịch/giờ/chi phí → nháp có lý do và phần chưa xác minh → khóa/
đổi một chặng → kiểm lại → người dùng xác nhận tạo kèo trên writer hiện hành.

Tái sử dụng brain seam, queue, catalogue, Valhalla và editor; xây domain kiểm
plan ở Go. Cần mở ADR trước các thay đổi contract, quyền chia sẻ hoặc boundary.
Không mở quyền ghi thứ hai cho cùng module, không tuyên bố E2EE xong bằng unit test.
Đầu tiên đồng bộ các đường recommendation còn đọc chat server với contract.

### Lát 2: du lịch ngắn trong một điểm đến

Mở từ day trip tới 2 ngày/1 đêm sau khi lát 1 đo được chất lượng. Ngày đến/về,
nơi ở, hoạt động nghỉ và chi phí ngoài chặng phải có mặt. Nếu nơi ở/di chuyển
đã đặt thì dùng dữ kiện người dùng chọn; nếu chưa có nguồn giá/tồn chỗ thì ghi
chưa xác minh, không tự hứa có booking. Planner nhiều thành phố để sau.

### Lát 3: học gu có đồng thuận

Dùng phản hồi có lý do và nguồn người dùng cho dùng; lưu preferences có phạm vi,
nguồn và khả năng thu hồi. Chưa có user thật thì chưa chọn collaborative filtering
hay fine-tuning làm bước đầu. Đo retrieval, validator và model riêng để biết lỗi
do thiếu địa điểm, do logic lịch hay do hiểu sai mong muốn.

## 8. Đo “chuẩn chỉnh” và “hay” bằng gì?

Đề xuất corpus mới: 10 brief cho mỗi ô của ma trận bốn tình huống và 20 edge
case, tổng 60 ca tổng hợp; đây là **kế hoạch đo, chưa tạo hoặc chạy**.
Edge case gồm khác thành phố default, diet thiếu evidence, dị ứng, budget không
rõ phạm vi, qua nửa đêm, giờ nghỉ giữa ngày, ngày đến/về, sở thích xung đột,
thiếu tọa độ/giá, hết giờ, route không sẵn sàng và sửa chặng đã khóa.

Máy kiểm mọi hard constraint của **mọi chặng và toàn plan**; dữ kiện unknown
phải hiện unknown. Xếp `valid`, `needs_review`, `infeasible` có lý do rõ.
Không tính câu trả lời từ chối tất cả là thành công recommendation.

Người đánh giá riêng: hợp mục đích, cân bằng thành viên, nhịp dễ đi, lý do có ích,
đánh đổi hiểu được, hai phương án khác nhau thực chất và sửa dễ. So với baseline
hiện hành trên cùng brief, không chỉ dùng LLM tự chấm LLM.

Ca đối chiếu quan trọng: giữ destination/budget/time và thay mong muốn thì plan
phải thay hợp lý; chỉ đổi nhãn cặp đôi/hội bạn khi mong muốn giống hệt không
được ép plan thành khuôn mẫu. Sửa chặng phải giữ các chặng khóa/ID và revision.

Theo dõi time-to-first-useful-plan, số câu hỏi bổ sung, tỷ lệ giữ/sửa/chốt,
chi phí inference, latency và tỷ lệ không thể đưa phương án vì thiếu dữ kiện.
Không ghi nội dung riêng vào telemetry để lấy những số này.

Trước khi implementation lên main: contract/PostgreSQL thật, clean-tree gate
tại SHA, canary/identity cùng harness, hai mutant không tương đương, mở ảnh UI
và bằng chứng native theo yêu cầu repo. Lượt nghiên cứu này chưa chạy các cổng đó.
