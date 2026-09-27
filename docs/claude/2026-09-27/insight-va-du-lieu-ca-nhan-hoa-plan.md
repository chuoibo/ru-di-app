# Insight và dữ liệu cho planner cá nhân hóa

Ngày: 27/09/2026. Bổ sung cho `nghien-cuu-ai-tao-plan.md` cùng ngày.
Đây là nghiên cứu và đề xuất, chưa là ADR hay implementation.
Code khảo sát tại HEAD `c8f092c1` và working tree có thay đổi chưa commit.
Số liệu DB là phép đọc tổng hợp từ database cấu hình của container local
`rudi-vnlocal-api-1`, khoảng 14:05 giờ Việt Nam; không phải inventory production.
Không đọc/xuất body chat, hồ sơ cá nhân, ảnh hay danh sách địa điểm.
Các query chạy trong transaction READ ONLY và kết thúc rollback.

## 1. Đơn vị cá nhân hóa là người trong một hoàn cảnh

Đề xuất mô hình: gu tương đối ổn định + mức quen thuộc + mong muốn lần này +
những người cùng đi + giới hạn thực tế. Không rút gọn người thành tag “cafe”.

Một người có thể thích cafe để trò chuyện với bạn, đi một mình để đọc sách,
hoặc khi du lịch để thử thức uống địa phương. Cùng tag, khác nhu cầu. Một người
có thể thích trải nghiệm mới nhưng hôm nay chỉ muốn về sớm vì ít năng lượng.

Casual/du lịch và hội bạn/cặp đôi là ngữ cảnh giúp chọn câu hỏi và mặc định;
không phải nhãn quyết định con người muốn gì. Không suy tình cảm từ `pair`,
không suy mức gắn bó hay tình trạng quan hệ từ chat hoặc lịch sử.

“Độc lạ” là tương đối với người, hoạt động và những người cùng trải nghiệm.
Một workshop quen thuộc với thị trường vẫn mới với người chưa thử. Một quán
ít review chưa chắc là quán local tốt. Ghé một di tích lần hai vẫn có thể mới
nếu người dùng chọn tìm hiểu sâu hoặc chọn một trải nghiệm khác có thật ở đó.

## 2. Những nhu cầu cần nghiên cứu, không coi là insight đã chứng minh

| Giả thuyết nhu cầu | Điều muốn đạt | Planner nên khác thế nào | Cách xác nhận |
|---|---|---|---|
| Kết nối | Có thời gian chất lượng cùng nhau | Hoạt động tạo tương tác, đủ khoảng nói chuyện | Quan sát lựa chọn và feedback, hỏi mục đích |
| Đổi nhịp | Thoát routine, có điều mới | Hoạt động chưa thử hoặc khu chưa khám phá | Hỏi mức quen thuộc và phần mới mong muốn |
| Hồi phục | Nhẹ đầu, ít quyết định, ít mệt | Ít chuyển chỗ, linh hoạt, không lịch dày | Hỏi năng lượng/nhịp của lần này |
| Khám phá nơi đến | Hiểu văn hóa, cảnh quan, lịch sử | Điểm tiêu biểu có giải thích, trải nghiệm bản địa | Hỏi điều muốn hiểu hoặc không muốn bỏ lỡ |
| Chơi cùng hội | Mọi người có thể tham gia | Sức chứa, nhịp nhóm, có phần hợp từng người | Hỏi riêng điều muốn/không muốn; không suy đồng thuận từ im lặng |
| Có một dịp đáng nhớ | Một trải nghiệm phù hợp dịp | Điểm nhấn có chủ đích và khả thi | Người dùng chủ động khai dịp; không tự đoán |

Không bắt người dùng chọn đúng một nhu cầu. Có thể “muốn mới nhưng đừng mệt”,
“muốn xem điểm nổi bật nhưng không chạy lịch”. Những đánh đổi này là chỗ
recommendation tạo giá trị, không phải chỉ là câu văn trang trí.

### Bốn tình huống khởi đầu

- Cặp đôi casual: cùng thử điều mới hoặc dành thời gian cho nhau trong nơi quen;
  khám phá mức mới vừa đủ và ngưỡng bất tiện cả hai chấp nhận.
- Hội bạn casual: chơi cùng nhau mà ít mất công chốt; hoạt động có thể đông
  vui nhưng cũng có thể là buổi nói chuyện nhẹ. Số người/quan hệ không định vibe.
- Cặp đôi du lịch: cân bằng mong muốn hai người, mức quen thuộc khác nhau,
  logistics và nhịp nghỉ. Một người lần đầu, một người đã đi rồi là case quan trọng.
- Hội bạn du lịch: không bỏ lỡ trải nghiệm phù hợp nhóm nhưng cũng không ép
  mọi người theo cùng một mật độ. Hoạt động tùy chọn chỉ khi nhóm muốn và khả thi.

Du lịch lần đầu có thể ưu tiên điểm tiêu biểu; du lịch quay lại có thể ưu tiên
một sở thích sâu hoặc trải nghiệm mới. Casual có thể tìm hiểu di tích. Đây là
defaults mềm phải cho sửa, không phải bốn preset cứng.

## 3. Dữ liệu thật đang có trong DB local

### Kho địa điểm

| Chỉ số | Số đo |
|---|---:|
| Điểm đến khai báo | 33 |
| Điểm đến có địa điểm | 8 |
| Tổng địa điểm | 9.363 |
| Ăn uống (`quan-an-local`) | 4.254 |
| Cafe | 1.743 |
| Đi chơi đêm | 217 |
| Vui chơi | 3.149 |
| Có địa chỉ không null | 8.152 |
| Có mô tả không null | 9.363 |
| Có `kinds` không rỗng | 9.363 |
| Có `traits` không rỗng | 0 |
| Có `activities` không rỗng | 0 |
| Có giá min/max | 0 / 0 |
| Có giờ mở cửa | 0 |
| Có rating | 0 |
| Có `group_fit` | 0 |
| Địa điểm có ảnh | 5.600 |
| Số record ảnh | 14.863 |

Tất cả dòng ghi `source = vnlocal`, có `source_ref`; trường license địa điểm
đều null. Điều này không kết luận license ảnh: ảnh là bảng/nguồn riêng.
Không mở ảnh hoặc audit điều khoản nguồn trong lượt này. Mô tả dài 202–1.056
ký tự, trung bình khoảng 591; độ dài không chứng minh chất lượng hay độ đúng.
Một điểm đến chứa 5.288/9.363 địa điểm, cho thấy mức phủ không đồng đều.

Không tìm thấy importer `vnlocal` trong checkout này. Container có thể dùng
image/cây mã khác: không suy implementation nhập dữ liệu từ tên nguồn. Inventory
DB là một bằng chứng riêng với đọc code; chưa đối chiếu provenance deployment.

Như vậy kho này dùng được để tìm ứng viên địa lý/loại hình và có nguồn mô tả
để nghiên cứu extraction, nhưng chưa đủ cam kết giá/giờ, booking, phù hợp nhóm
hay trải nghiệm “local ngóc ngách”. Không lấy nội dung mô tả làm sự thật đã
kiểm chỉ vì có đủ chữ; phải giữ nguồn và mức xác minh của từng thuộc tính.

### Dữ liệu người dùng và cuộc đi trong cùng DB

| Chỉ số | Số đo |
|---|---:|
| Record người | 1 |
| Người có city / budget band | 0 / 0 |
| Record person_interests / người có interests | 0 / 0 |
| Địa điểm đã lưu | 1 |
| Cuộc đi / chặng | 0 / 0 |
| Memories, gồm check-in | 0 |
| Ràng buộc sổ đôi | 0 |
| AI invocations | 0 |

Không coi record người này là bằng chứng người dùng thật. DB local này chưa có
corpus hành vi hoặc sở thích để học cá nhân hóa. Không suy toàn sản phẩm có
một người dùng, không suy DB production rỗng, không suy một bookmark là gu.

## 4. Dữ liệu code/schema có thể lưu và planner có dùng không?

| Dữ liệu | Nơi trong code | Vai trò hiện tại / giới hạn |
|---|---|---|
| 8 tag sở thích | `domain/interests/interests.go`, mobile onboarding | Ăn uống, cafe, nightlife, món local, outdoor, shopping, karaoke, game; thiếu sắc thái mục đích và nhịp |
| 3 budget band | Cùng vocabulary; `people.BudgetBand` | Dưới 100K, 100–250K, 250–500K/người/cuộc đi; không tự dùng làm budget du lịch |
| City, bio | `repo/people.go` | Có schema, sửa được; city tự khai không chứng minh độ quen từng khu |
| Gu nhóm | `service.GroupTaste`, `taste.ProfileForGroup` | Hợp tag + trung bình budget; ảnh hưởng thứ tự candidate, không giữ gu từng người cho model |
| Bookmark | `repo/saved_places.go` | Có API/lưu trữ; worker plan chưa đọc; lưu có thể chỉ là cân nhắc hoặc lưu cho người khác |
| Check-in và lịch sử chi | `routes/preference_profile.go` | Có profile suy từ nhóm; không gọi profile này trong worker plan; đã đến/đã chi không chứng minh thích/khả năng chi |
| Outing và timeline | `repo/recap.go` | Có ngày, budget, số người; chặng hỗ trợ ngày/thời lượng/khóa giờ; không có mục đích/feedback sở thích theo trường riêng |
| Sổ đôi và ràng buộc | `repo/pair_notebook.go`, domain pairnotebook | Có consent, `chia_gu`, `khong_an_duoc`, `dung`; không tự mở chúng cho planner group-only |
| Gói chat được trao | `chatassist/boicanh.go` | Có thể chứa mong muốn lần này; văn bản chưa thành brief/ràng buộc có cấu trúc |
| Catalogue giàu hơn payload | `repo/places.go` | Có kinds, traits, activities, description, source; model companion nhận 9 trường, bỏ những trường này |

Không nhầm schema có trường với DB có dữ liệu, hoặc API có dữ liệu với model
đã nhận được. Comment lịch sử trong `screens/vao-cua/so-thich.ts` còn mô tả
chưa lưu; flow thật `so-thich-song.ts` và Onboarding đã gọi PUT lên server.

Trong importer OSM của repo, `restaurant`, `fast_food`, `food_court` đều vào
`quan-an-local`. Đó là nhãn loại hình, chưa là chứng nhận món bản địa hay quán
người địa phương hay ăn. `museum`, `viewpoint`, cinema, theme park cùng vào
`vui-choi`, làm mất độ phân giải nếu chỉ dùng category. Mapping không bao phủ
`historic=*` độc lập; chưa thấy inventory workshop thật với lịch/slot/booking.
Seed có workshop gốm, nhưng seed là dữ liệu tổng hợp cho demo/test.
Không áp mapping OSM này để giải thích nguồn `vnlocal` chưa xác minh.

## 5. Những thông tin còn thiếu để hiểu “đi như nào”

| Cần hiểu | Câu hỏi tự nhiên | Khi nào cần | Loại dữ liệu |
|---|---|---|---|
| Kết quả mong muốn | “Lần này bạn muốn dành thời gian cho nhau, thử điều mới hay nghỉ nhẹ?” | Trước nháp, nếu lời nhờ chưa nói | Ý định lần đi |
| Quen nơi đến đến đâu | “Bạn sống ở đây, đã ghé vài lần hay lần đầu tới?” | Khi đổi điểm đến | Theo điểm đến/người, được khai |
| Mức mới mong muốn | “Một hoạt động mới hay muốn cả buổi khác hẳn mọi lần?” | Khi cần cân bằng novelty | Preference mềm theo lần |
| Nhịp/năng lượng | “Nhẹ nhàng hay muốn chơi nhiều?” | Casual sau giờ làm hoặc chuyến dày | Tạm thời, không lưu thành tính cách |
| Tương tác mong muốn | “Muốn cùng làm gì đó hay chủ yếu ngồi trò chuyện?” | Chọn activity | Theo người cùng đi/lần đi |
| Ngưỡng bất tiện | “Bạn có muốn đặt trước, đi xa hoặc chờ để thử chỗ mới không?” | Khi các phương án có đánh đổi đó | Điều kiện cụ thể |
| Gu có sắc thái | “Bạn thích cafe vì không gian, đồ uống, hay có chỗ nói chuyện?” | Chỉ hỏi khi nó đổi đề xuất | Preference tương đối bền, tùy chọn |
| Điều không muốn | “Có món/hoạt động nào cần tránh?” | Khi ảnh hưởng khả thi | Veto riêng, có phạm vi |
| Thành viên thực sự đi | “Những ai tham gia lần này?” | Trước tính hợp nhóm | Roster cuộc đi |
| Phạm vi tiền | “Mức này cho cả chuyến hay mỗi ngày; gồm những khoản nào?” | Khi chưa rõ | Constraint đã xác nhận |

Không đưa tất cả lên onboarding. Nhận lời nhờ tự nhiên, trích những gì đã có,
cho xem brief ngắn; hỏi phần thiếu quyết định đề xuất. Dùng lựa chọn minh họa
và sửa tại chặng để người dùng nói được điều họ chưa có từ vựng mô tả.

## 6. Dữ liệu địa điểm cần bổ sung tương ứng với nhu cầu

Đơn vị phù hợp là **trải nghiệm** gắn với địa điểm: một bảo tàng có thể có xem
trưng bày và workshop, một quán có nhiều không gian/khung giờ. Không tự tạo
trải nghiệm từ kiến thức model khi chưa có nguồn.

- Loại trải nghiệm: làm cùng nhau, xem/tìm hiểu, ăn/thử món, vận động, trò chuyện.
- Sắc thái có nguồn: nhịp hoạt động, có hướng dẫn, tương tác, trong/ngoài trời,
  độ gắng sức, khả năng cùng tham gia, điều kiện lứa tuổi/tiếp cận nếu được công bố.
- Giá: đơn vị người/nhóm/vé, bao gồm gì, khoảng chi, nguồn/ngày kiểm, phí thêm.
- Thời gian: thời lượng, lịch có mặt, slot, giờ theo ngày, last entry, cần đặt trước.
- Local/văn hóa: món hoặc câu chuyện liên quan nơi đến với nguồn hỗ trợ;
  không gắn “authentic”, “hidden gem”, yên tĩnh hay ít đông chỉ từ category/rating.
- Nhận diện: place ID, activity ID, source, provenance, thời điểm kiểm và mức chắc.

Phân biệt ba lớp: fact từ nguồn; inference như “có thể hợp người muốn làm cùng
nhau”; và preference match với người dùng. LLM extraction chỉ tạo đề xuất
thuộc tính có evidence, không tự nâng chúng lên fact. Go sở hữu nhập/ghi/vòng đời;
Python chỉ extraction/inference/evaluation. Review dữ liệu có quyền sử dụng ở
ngoài worktree, không đưa dữ liệu địa điểm tải về hay dữ liệu người vào Git.

## 7. Học cá nhân hóa thế nào khi hiện chưa có hành vi?

### Bắt đầu bằng điều người dùng nói cho lần đi

Ví dụ hoàn toàn tổng hợp: hai người ở một thành phố đã lâu. Người A muốn làm
gốm lần đầu; B muốn thử món local nhưng không muốn ngồi lớp quá lâu. Cả hai có
3 giờ và mức chi đã xác nhận. AI cần tìm trải nghiệm phù hợp với thời lượng,
không ép A vào lớp dài hoặc B vào tuyến ăn chỉ vì cả hai tick “ăn uống”.

Nếu kho chưa có workshop có giờ/giá xác minh, hệ thống phải nói phần đó cần
kiểm thêm hoặc đưa lựa chọn có dữ kiện hơn, không bịa để thỏa câu chuyện.

### Khi người dùng sửa, giữ nguyên lý do và phạm vi

“Quán này xa quá” là preference về khoảng cách trong ngữ cảnh này, chưa phải
ghét món ăn. “Bỏ workshop vì hôm nay mệt” không phải dislike workshop lâu dài.
“Không ăn món này” cần làm rõ phạm vi khi cần; không suy dị ứng từ dislike.
“Đã đi rồi nhưng muốn dẫn bạn tới” không được chặn novelty theo lịch sử visit.

Mỗi thông tin nên có xuất xứ, scope (lần đi/điểm đến/người cùng đi/lâu dài),
thời điểm, confidence và trạng thái được người dùng xác nhận. Cho xem/sửa/xóa
và thu hồi việc dùng làm cá nhân hóa. Không biến chat E2EE hoặc diary riêng
thành nguồn học tự động. Người dùng chủ động chia sẻ đúng phần cần dùng.

### Đọc tín hiệu đúng mức

| Tín hiệu | Có thể biết | Chưa thể kết luận |
|---|---|---|
| Người dùng nói mục đích | Mong muốn lần đi này | Tính cách cố định |
| Chọn/khóa một chặng | Muốn giữ chặng trong plan này | Đã đi hoặc đã thích |
| Lưu địa điểm | Có quan tâm hoặc muốn giữ để xem | Người đó thích/đã trải nghiệm |
| Check-in | Có ghi nhận đến nơi | Chất lượng trải nghiệm hay nguyên nhân đến |
| Sửa/xóa chặng | Nháp chưa hợp một điều kiện | Dislike lâu dài nếu không có lý do |
| Chi phí đã ghi | Khoản phát sinh trong phạm vi sổ | Thu nhập/khả năng chi/mức sẵn sàng trả |
| Feedback có lý do | Đánh giá trải nghiệm được khai | Có thể áp mọi lần/người nếu chưa rõ phạm vi |

Ưu tiên constraint rõ và intent hiện tại, rồi dùng gu có nguồn hỗ trợ để xếp
phương án. Chấm satisfaction trên toàn plan và từng người, giữ unknown là unknown.
Novelty cá nhân chỉ xếp sau constraint và phù hợp mục đích; không luôn đẩy người
đã thích chỗ quen sang chỗ mới. Unknown không được cho điểm “mới với bạn” chắc chắn.

## 8. Insight phải được kiểm với người thật

Nghiên cứu tài liệu cho cơ sở, không thay user research Việt Nam:

- [Aron và cộng sự, 2000](https://pubmed.ncbi.nlm.nih.gov/10707334/): khảo sát và
  thí nghiệm về hoạt động mới/kích thích cùng nhau và chất lượng quan hệ được
  cảm nhận. Hỗ trợ giả thuyết shared novelty; không chứng minh mọi cặp đôi muốn
  workshop hoặc một app planner cải thiện quan hệ. Không dùng làm lời hứa sản phẩm.
<!-- repo-guard: allow=long-number reason=public-research-source-identifier -->
- [Context-aware recommendation review](https://doi.org/10.1016/j.cosrev.2020.100255):
  preferences thay đổi theo hoàn cảnh; áp dụng là tách gu bền và intent lần đi.
<!-- repo-guard: allow=long-number reason=public-research-source-identifier -->
- [Usage-related preference questions](https://arxiv.org/abs/2111.13463): hỏi về
  cách dùng giúp khai preference khi người dùng chưa biết các thuộc tính của
  lựa chọn. Áp dụng là hỏi “muốn làm gì cùng nhau” trước “thích category nào”.
<!-- repo-guard: allow=long-number reason=public-research-source-identifier -->
- [Conversational recommendation survey](https://arxiv.org/abs/2004.00646):
  hội thoại khai preference, hỏi về đề xuất và phản hồi. Áp dụng là vòng sửa
  cục bộ có lý do, không chỉ một lần trả danh sách.
- [NN/g về attitude và behavior](https://www.nngroup.com/articles/attitudinal-behavioral/):
  lời kể và quan sát bổ sung nhau; hỏi có thể dẫn dắt hoặc nhớ sai. Không coi
  “tôi sẽ dùng” hay lựa chọn trong interview là bằng chứng hành vi thực tế.
- [NN/g diary studies](https://www.nngroup.com/articles/diary-studies/):
  ghi nhận theo thời gian trong hoàn cảnh giúp hiểu nhu cầu thay đổi; phù hợp
  nghiên cứu nhiều cuộc đi chứ không chỉ một phiên phỏng vấn.

### Vòng khám phá đề xuất

Tuyển khoảng 12–16 người có cuộc đi gần đây, cân đối người thường lên plan và
người được rủ, các kiểu quan hệ/mục đích, mức quen nơi đến và ngưỡng chi.
Đây là mẫu khám phá định tính đề xuất, không đại diện thống kê và chưa tuyển ai.

Phỏng vấn từng người về cuộc đi cụ thể: điều muốn đạt, các lựa chọn đã cân
nhắc, lý do bỏ, ai nhường điều gì, sự khác biệt giữa plan và cuộc đi thật.
Không mở đầu bằng “bạn có thích hidden gem/workshop không?”. Lời nhớ lại vẫn
chỉ là tự khai; kết hợp quan sát họ tạo/chỉnh một plan và nhật ký cuộc đi.

Cho họ so hai plan cùng giá/thời gian: một plan quen/ít bất tiện, một plan có
hoạt động mới. Không gắn nhãn “AI cá nhân hóa tốt hơn” để dẫn dắt. Hỏi chặng
nào giữ/bỏ, vì sao, và phần nào họ không tin hoặc phải tự kiểm.

Nếu người dùng đồng ý, theo dõi 1–2 cuộc đi tiếp theo: intent trước khi đi,
thay đổi khi đi, feedback sau khi đi. Không xin raw chat/export; ghi chép nghiên
cứu thật nằm ngoài repo, chỉ tóm tắt đã ẩn danh và được phép vào tài liệu.
Không liên hệ người tham gia hay gửi dữ liệu ra dịch vụ ngoài ở lượt này.

Mỗi insight phải nối: evidence quan sát/tự khai → cách diễn giải → giả thuyết
thay thế → thay đổi thiết kế → cách đo. Ví dụ đổi quán có thể do xa, đóng cửa,
giá, hoặc gu; không quy về novelty khi chưa có lý do.

## 9. Hướng đi đầu tiên sau khảo sát này

Hai việc cần tiến song hành trong cùng lát sản phẩm:

1. Brief ngắn và sửa theo ý đồ: mục đích, quen nơi đến, nhịp, người tham gia,
   mức chi có phạm vi; model không tự đọc thêm dữ liệu riêng. Chuẩn bị nghiên
   cứu để xác nhận giả thuyết về cuộc đi và câu hỏi người dùng hiểu được.
2. Pilot trải nghiệm có dữ kiện: chọn một khu vực theo dữ liệu thực tế, làm
   giàu số lượng vừa đủ các kiểu trải nghiệm; kiểm giá/giờ/thời lượng/booking,
   sắc thái và nguồn. Đánh giá nháp theo từng người và toàn buổi.

Đề xuất khởi đầu 30–50 trải nghiệm kiểm được trong một khu vực, là phạm vi
thử nghiệm chứ không KPI đã chốt. Giữ catalogue rộng cho discovery; có tập
trải nghiệm đủ dữ kiện cho plan đã kiểm. Không xóa dữ liệu hay dời writer.

Chưa ưu tiên collaborative filtering hoặc fine-tune từ dữ liệu hành vi rỗng.
Chưa làm ontology hàng trăm thuộc tính trước user research. Chưa chọn provider
mới: source vnlocal/ảnh và quyền sử dụng cần được đối chiếu riêng theo ADR.

Chất lượng cần đo: đúng intent, có phần phù hợp từng người, mức mới đúng mong
muốn, nhịp dễ đi, ít sửa vì lỗi dữ kiện, giữ những gì đã khóa, giải thích nguồn
và unknown rõ. Ngoài tỷ lệ chốt, cần feedback sau cuộc đi vì chốt nhanh chưa
chứng minh trải nghiệm tốt. Số liệu nghiên cứu này chưa là bằng chứng các KPI đó.
