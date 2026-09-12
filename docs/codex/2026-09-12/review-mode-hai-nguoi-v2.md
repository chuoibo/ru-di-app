# Review bản 2 và trả lời tác giả — Nếp truyền giấy

**VERDICT: REQUEST_CHANGES tại `7c079256ce361c69c7b8d80a49cb878adbc63ba0`.** Chấp nhận hướng trải nghiệm bản 2 để tiếp tục hoàn thiện spec/kế hoạch nháp. Chưa xác nhận C1–C6 đều khép, chưa duyệt merge hoặc hiện thực. Không yêu cầu native frame trước khi lập kế hoạch.

Đọc đủ 1284 dòng, mục 0–23. Mọi số dòng dưới đây chỉ về [spec tại SHA được review](https://github.com/chuoibo/ru-di-app/blob/7c079256ce361c69c7b8d80a49cb878adbc63ba0/docs/superpowers/specs/2026-09-12-mode-hai-nguoi-nep-truyen-giay-design.md). Root đọc độc lập toàn bộ; hai lượt bổ sung `/root/design_review_a` và `/root/evidence_review_b` kiểm state/story và nguồn/quyền/màu. Đây là tái soát contract, không chạy lại full critique hoặc điểm heuristic. `protocol_version` snapshot v1 không đổi.

## 1. Kết luận về hướng sản phẩm

Bản 2 đã sửa đúng phần lớn vấn đề lớn của bản 1: lời rủ có người gửi, tờ lời rủ trở thành kỷ niệm ngay trong lát đầu, một không gian giấy có một tờ mở, buổi một chỗ chính hợp lệ, và bỏ món nợ «một lần lạ». Tôi giữ các quyết định đó; không yêu cầu quay lại tranh luận toàn bộ concept.

Hướng này có câu chuyện rõ và đáng làm: **Nếp giúp một người mở lời, rồi giữ lại điều hai người đã sống qua.** Những chỗ chưa khép nằm ở chuyển trạng thái và quyền sử dụng dữ liệu; chúng sửa được bằng một lượt hợp nhất ngắn, không cần xây thêm màn để trả lời.

## 2. Trả lời R1–R4 và hai xác nhận

### R1 — Đồng ý giữ hai ô ràng buộc trong pilot

«Không ăn được» và «Đừng» trong luồng lời rủ không phá phạm vi pilot. Chúng là đầu vào cho việc chọn buổi đi; không bắt buộc kéo theo màn sổ bảy mục, ôn thẻ hay chương trình học về người kia.

Cần ghi rõ ai nhập, điều đó nói về ai, ai được xem và dùng tới khi nào. Nếu được nhập như ghi tay riêng thì chỉ lái nháp riêng; lúc chia lời rủ áp dụng luật chuyển riêng → chung ở mục 3 bên dưới. Nếu người dùng chủ động khai ràng buộc dùng chung, hiển thị đúng phạm vi ấy ngay bên ô. Hai ô không nên bắt điền để đủ biểu mẫu; «chưa biết» vẫn là giá trị hợp lệ, không suy thành không có hạn chế. §8 đã phân biệt biết hợp/biết không hợp/chưa biết, và phải giữ điều này khi lọc.

### R2 — Đồng ý; sửa yêu cầu cũ của reviewer

Tôi rút cách đặt yêu cầu có thể khiến native frame thành điều kiện trước kế hoạch. **Chọn §15.1 làm mặc định:** một hàng vào không gian giấy riêng, một tờ đang mở. Trước code chỉ cần route map và wireframe theo kích thước với đủ trạng thái. Chúng mô tả dự định, không phải native evidence.

**Bên trong lát 1:** làm lát UI nhỏ đầu tiên, xem trên native ở 360dp, font 1.0/2.0, sáng/tối, IME và tin mới; kiểm Back, cuộn, composer, trạng thái normal/reduced motion. Sửa trên lát ấy trước khi nhân các biến thể hoặc gọi lát 1 hoàn tất. Không cần hiện thực cả hai phương án để thi đấu; chỉ thử phương án khe thu gọn nếu mặc định bộc lộ vấn đề đáng kể.

Các câu hỏi thuộc lát 3, chẳng hạn mốc bản đồ, không cần trở thành cổng chặn kế hoạch lát 1. Lead vẫn quyết phạm vi; đề nghị §23 chỉ ghim quyết định làm đổi lát 1 hoặc quyền dữ liệu của nó.

### R3 — Đồng ý bỏ hẹn mở lúc đi cùng khỏi phạm vi

Ba mốc thời gian đủ cho lựa chọn hiện tại. Không cần đưa một thao tác xác nhận hiện diện vào để cứu tính năng này.

Tôi không đồng ý mệnh đề tuyệt đối «một thao tác rõ thì không còn nghĩa gì»: một nghi thức «Cùng mở» vẫn có thể có nghĩa nếu sau này người dùng chủ động muốn nó. Nhưng đó là tính năng khác, không phải cảm biến, và không cần đưa lại vào bản này. Mốc thời gian còn lại chỉ bảo đảm **được phép mở từ lúc đó**, không hứa máy tự đánh thức app hoặc đẩy thư đúng giờ khi không có scheduler/push.

### R4 — Đủ cho guard số lớp art, chưa đủ cho thứ bậc thị giác

Đếm lớp coral do hàm art phát ra là đúng chỗ để kiểm anatomy/vocabulary. Hãy đặt tên component/hàm và nói đếm theo từng hình hay theo tập hình cùng render. Không đếm pixel ảnh/sticker người dùng.

Tuy nhiên, hai CTA đều nổi bật vẫn có thể vượt qua guard khi art chỉ có một lớp coral. Một lớp cũng có thể vẽ nhiều vùng tách rời. Vì vậy ghi thành hai yêu cầu:

1. **Guard nguồn:** số lớp coral và hình học trong art được chỉ định.
2. **Review toàn frame trong lát 1:** có một hành động dẫn rõ giữa nội dung, nút và art.

Không thay cổng thứ hai bằng cổng thứ nhất. Đây không phải yêu cầu viết công cụ mới để đo «đẹp».

### Xác nhận 1 — Luật seed riêng đúng tại bước đọc, chưa đủ đóng C2

§7.2 cho dữ liệu riêng mồi nháp chỉ chính chủ thấy (393–399). §3.1 sau đó cho nháp thành tờ chung, và §3.3 còn cho Nếp gửi hộ (189–190, 219). Đây là đường chuyển dữ liệu suy ra sang người kia.

Ví dụ tổng hợp: A lưu riêng địa điểm X. Nếp dùng X trong nháp riêng. B chưa thấy X ở bước đó là đúng. Nhưng nếu Nếp tự gửi nháp ấy khi A im lặng, X hoặc lý do chứa ghi chú riêng đã đi sang B mà chưa có hành động chia sẻ. Không gửi cột nguồn vẫn có thể tiết lộ nội dung được suy ra từ nguồn.

**Bổ sung đủ để khóa đường này:** chủ được xem đúng nội dung sẽ gửi và chủ động cho phép chia payload ấy; hành động đó không tự biến nguồn riêng thành dữ liệu chung. Lý do/excerpt/metadata riêng không tự đính kèm. Nếp gửi hộ chỉ dựng từ nguồn chung còn quyền sử dụng, không lấy nháp có nguồn riêng rồi tự xuất bản. Gỡ nguồn đã chia phải kiểm lại trước gửi; quyền của A không cho phép chia nguồn riêng của B.

Đây là xác nhận nguyên tắc thiết kế. Khi hiện thực phải có ca server/DB thật cho private draft, manual share, auto-send và thu hồi; chưa có implementation để xác nhận cổng kỹ thuật đã chạy.

### Xác nhận 2 — Đúng, nhưng thu hẹp câu «không file nào»

Presentation policy và phân quyền phía máy chủ ở biên đọc/ghi là cách phân vai đúng. Cổng đếm nhánh chỉ gác tổ chức mã, không chứng minh quyền.

Dòng 771 vẫn nói **ngoài module này, không file nào rẽ nhánh theo loại sổ**. Sửa thành quy tắc cho tầng trình bày; server policy/authorization được kiểm loại sổ, chu kỳ, membership và consent khi cần. Phạm vi bảo vệ phải bao gồm list/detail/media, sinh gợi ý và gửi tự động, không chỉ nút trên màn.

## 3. Trạng thái C1–C6 sau kiểm lại

| Mục | Kết quả | Phần đã đóng / phần còn lại |
|---|---|---|
| **C1** | **Còn mở, P1** | Tác giả thật, phiên bản và không coi im lặng là đồng ý đã đúng; actor/event và đường vào đầu tiên chưa khớp |
| **C2** | **Còn mở, P1** | Nguồn private và ghi tay trước consent đã tách; thiếu luật phát hành đầu ra suy ra và consent theo năng lực |
| **C3** | **Còn mở, P1** | Chu kỳ mới/không hồi sinh giấy chưa tới lúc đúng; chưa xác định quyền của vật đang chờ và thời điểm khóa |
| **C4** | **Đóng lỗi số và nhãn hẹn mở; còn sửa scope art, P2** | Màu mới đạt ngưỡng spec; §17.5 còn câu độc quyền coral toàn màn. Guard art không phải proof hierarchy |
| **C5** | **Đóng lỗi cũ về log/cảm biến** | §8 và việc bỏ hẹn mở đã giải quyết; cần áp dụng cùng nguyên tắc cho chuyển sang `da_di` thuộc C1 |
| **C6** | **Đóng hướng roadmap; còn một mâu thuẫn đo, P2** | Vòng và chuỗi sự kiện đã tốt; dòng 1152 vẫn suy nghỉ tăng thành Nếp ồn dù 1158 nói đã bỏ |

### C1: bốn quyết định nhỏ còn thiếu, không cần thêm màn

**Một: bảng state chưa thực hiện được luật kèm theo.** `da_gui` không có cạnh `het_han` (190) dù 203 bắt hết khung thì hết hạn. `da_xem` không có nghỉ tuần (191) dù 358 nói nghỉ luôn có; không có rút dù điều kiện «chưa phản hồi» vẫn đúng khi mới xem. Các trạng thái cuối ghi người thấy là «tuỳ» (196), chưa đủ để hai client vẽ cùng nghĩa.

**Hai: Gửi có tính là chấp thuận của người gửi không?** Dòng 193 yêu cầu cả hai đồng ý cùng v nhưng không nói hành động tạo chấp thuận của người gửi. Bản sửa v+1 ai soạn/gửi? Người nhận chỉ yêu cầu sửa hay có thể trở thành người gửi mới? Phản ứng của mỗi người phải tách khỏi trạng thái tổng của tờ. Cần một ví dụ v1 → yêu cầu sửa → v2 → chốt, có actor ở từng bước. Quyết rõ đóng băng tờ đã chốt hoặc đường sửa sau chốt; khóa `(kèo, phiên bản)` một mình chưa ngăn hai phiên bản tạo hai outing cho cùng buổi.

**Ba: lượt lập sổ khác lượt luân phiên đầu.** Người khởi xướng phải soạn/gửi lời đầu để tạo sổ (793–794), nhưng nháp chỉ chủ lượt thấy (189), còn tuần đầu gậy thuộc người kia (250–251). Đề nghị gọi lời đầu là **lượt mở đầu**, thuộc người khởi xướng; lượt tiếp theo mới trao người kia. Đổi «người ít mở lời» thành «người kia» vì hệ thống chưa có căn cứ biết ai ít mở lời.

**Bốn: `da_di` và `da_giu` chưa có contract hoàn chỉnh.** Dòng 194–195 chưa nói ai ghi nhận đã đi và theo sự kiện nào. Một check-in không chứng minh hai người cùng đi (§8). `da_giu` xuất hiện làm đích nhưng không có hàng: người thứ hai có thêm dòng sau người thứ nhất không; cả hai bỏ qua thì tờ được giữ và tìm lại thế nào? Chỉ cần quyết hành vi và ghi rõ điều đó là người dùng tự ghi nhận, không suy thành xác minh hiện diện.

**Tiêu chí gỡ:** một bảng event–actor–điều kiện–kết quả cho các điểm trên và các cặp đến gần đồng thời: đồng ý/rút, đồng ý/nghỉ, sửa/đồng ý, đóng/chốt. Đây là mô tả cần cho kế hoạch; không bắt chạy race test trước khi có code.

### C2: hai cấp consent đang dùng chung một từ

§14.1 cho lời rủ tạo sổ; §14.2 nói mặc định là hai người bạn, bật Một đôi mới thêm vai/sổ riêng/túi và cần cả hai đồng ý (805–810). §7.3 cho đọc chat «sau consent». Nhận lời đi chơi chưa nói rõ có cấp quyền dùng chat hay không.

**Phải ghi rõ:** nhận lời đi chơi không tự bật Một đôi, không tự cấp quyền đọc lịch sử chat. Trước khi bật năng lực đọc dữ liệu, hai người cần thấy ngắn gọn dữ liệu nào được dùng, cho việc gì, và cùng đồng ý đúng đề nghị đang còn hiệu lực. Không cần ba dialog; cần ba ý nghĩa riêng trong contract.

Bảng năng lực tối thiểu cần có: **pair chỉ chat / sổ hai người bạn / sổ đôi**, riêng quyền rủ–giữ, gậy, hai ô, đọc chat, sổ riêng, túi. Hiện lát 1 có gậy (1107) nhưng bật đôi ở 808 mới thêm vai và Cài đặt sổ đôi tới lát 2 (881). Tác giả phải chọn quyền nào tồn tại từ lượt đầu, không để frontend tự suy.

Đề nghị bật Một đôi cần có người đề nghị, đang chờ, rút/từ chối/hết hạn/đồng ý. Chấp thuận tới muộn sau khi đã rút hoặc đóng sổ không được bật lại. Đây là phần lifecycle consent còn thiếu, không phải yêu cầu thêm cả hệ tài khoản.

### C3: «tới hết chu kỳ» chưa rõ tại chính sự kiện kết thúc chu kỳ

Dòng 438 cho giấy đã tới giờ nhưng chưa đọc được mở «tới hết chu kỳ, rồi đóng»; dòng 1194 định nghĩa Đóng sổ là **kết thúc chu kỳ**. Chọn rõ một nghĩa: khóa ngay tại `closed_at`, hoặc có hạn gia hạn tuyệt đối được định nghĩa. Không để một client tiếp tục cho mở vì nghĩ chu kỳ còn tồn tại tới khi nối lại.

Bảng §7.5 thiếu nháp, lời rủ chờ trả lời, đề nghị sửa, phản ứng chờ chốt và đề nghị bật đôi. Thêm mỗi loại còn quyền đọc/ghi/gửi/chốt gì sau đóng; tách **giữ lịch sử đã chia** khỏi **còn được dùng nguồn cho gợi ý mới**. Luật áp dụng cho media và đầu ra đã tính/cache phía máy chủ. Thu hồi truy cập về sau không thể hứa xóa ảnh chụp hay bản sao đã tới máy người dùng.

**Tiêu chí gỡ:** đóng sổ chặn gửi/chấp thuận/chốt/mở trái luật của chu kỳ cũ; nối lại không hồi sinh draft/token/lịch cũ. Quyền bản nháp do chủ tự ghi có thể giữ nếu sản phẩm chọn, nhưng không còn tự phát hành vào chu kỳ đã đóng.

### C6: sửa một hàng để phần đo tự nhất quán

Đổi dòng 1152 thành **nghỉ tăng là tín hiệu cần tìm hiểu**, không tự kết luận Nếp ồn hay bắt siết nhịp. Ghi nhận đã thấy lời rủ, lý do tự nguyện và nhu cầu thực tế trước khi đổi nhịp. Dòng 1158 đang nói chính điều này đã sửa rồi.

Tiêu chí tám tuần là giả thuyết vận hành; cần nêu phạm vi theo cặp/cohort và mức phơi nhiễm khi viết kế hoạch nghiên cứu. Chưa có bằng chứng hành vi hoặc quyền mở nghiên cứu mới trong lượt này.

## 4. Những sửa nhỏ về câu chuyện và hình, không mở lại hướng thiết kế

- §1.2 dòng 83 vẫn «không mặt», §17.2 dòng 1037 có mắt/mày/miệng. Xóa mô tả không mặt; giữ canon hiện có.
- Dòng 726 Nếp hội im lặng; 1021 lại «nói khi được gọi». Tách mascot khỏi người phát ngôn AI, không dùng hai câu ngược nhau.
- Dòng 1077–1078 vẫn «góc coral là thứ ấm duy nhất trên cả màn tối». Đổi về phạm vi hình Nếp như R4.
- §16.3 còn nói nếp đọc ra nhờ **ba hàng đều nhau** (970), trong khi default một chỗ chính. Không ép thêm nội dung cho đủ hàng. Kiểm specimen một chỗ chính trong lát 1; nếp nằm ở vật liệu, số hàng do nội dung.
- Số `lineStrong` đã đúng, nhưng không suy rằng mọi container đều bắt buộc viền 3:1. `tokens.json` phân biệt viền trang trí `line` và biên control `lineStrong`. Nếu toàn tờ là control và viền cần để nhận ra, dùng ngưỡng cho control; nếu chỉ là nền nội dung có nút riêng thì mép mạnh là lựa chọn mỹ thuật cần xem trên specimen. Không biến mọi giấy thành khung đậm để sửa một lỗi cũ.
- Copy auto-send «Nếp gửi vì chưa ai mở lời» (219) gợi lại việc ai đã không làm. Đề nghị **«Nếp có một lời rủ cho tuần này»**; tên tác giả Nếp đã đủ trung thực.
- Read receipt mới của tờ lời rủ là quyết định sản phẩm, không cần để biết đã được đồng ý. Không chặn spec chỉ vì chọn nó; cần xem liệu trạng thái đã xem/chưa trả lời có tạo áp lực trong vòng kiểm sau.
- Cùng một vật thị giác không bắt buộc một máy trạng thái cho mọi loại: §11.3 dòng 638–641 đang cho cả thư chữ/ảnh dùng §3.1, vốn đi qua chốt outing/đã đi. Ghi §3.1 áp dụng **tờ lời rủ**; lifecycle thư hẹn giờ thuộc lát 3. Không cần thiết kế toàn bộ schema thư ngay để lập kế hoạch lát 1.
- Hai vai còn cố định trong khi gậy đổi tuần: cần ai nhận nháp/câu hỏi/ba dòng dặn trong từng lượt. Đây là lỗ thao tác cụ thể, không phải yêu cầu phải đổi tên vai theo sở thích reviewer.
- §15.2 dòng 880 còn ba lựa chọn loại sổ sau khi §14.2 đã hoãn Người nhà. Sửa inventory về năng lực thực sự có trong lát.

## 5. Kiểm số và ranh giới bằng chứng

Root và B tính độc lập từ `packages/shared/tokens.json@7c079256`, sRGB. Kết quả:

| Cặp | Sáng | Tối |
|---|---:|---:|
| `inkSoft` / `paper` | 7.490608 | 6.859643 |
| `lineStrong` / `paper` | 4.525271 | 3.232331 |
| `lineStrong` / `ground` | 4.091339 | 4.677954 |

`lineStrong` tối trên literal `#1c1f36` spec dùng = **4.3414** làm tròn bốn chữ số. Đây là kiểm token/literal, không đo lại nền native. Các số sửa về chữ và mép đủ khép lỗi toán cũ; không dùng chúng để nói specimen đã đẹp hoặc đọc được trên mọi mật độ.

Đã đối chiếu anatomy Nếp, token và `motion.ts` cùng SHA. Không chạy detector lại trên Markdown vì regex không giúp xác nhận những điểm còn mở; kết quả `[]` của lượt trước cũng không được tái dùng như chứng cứ mới. Không chạy app, backend suite hoặc người thật. Thay đổi giao hàng chỉ là báo cáo review, kiểm repo guard và whitespace trước commit.

## 6. Cửa tiếp theo

**Có thể tiếp tục hoàn thiện kế hoạch nháp lát 1 ngay theo hướng bản 2.** Chưa gọi đó là kế hoạch đã duyệt hay mở cửa viết code. Tác giả sửa bảng event, ranh giới phát hành nháp riêng, consent theo năng lực và bảng đóng sổ; sửa các câu sót ngay tại chỗ. Sau đó Codex kiểm đúng phần thay đổi để khép C1–C3, C6 và phần scope còn lại của C4, không yêu cầu viết lại toàn bộ bản 3 hoặc dựng hai UI trước kế hoạch.

Native proof nằm trong lát 1. ADR và các quyết định Lead làm đổi lát 1 vẫn phải xong trước khi hiện thực tương ứng. Việc đã trả lời đủ D1–D8 là tiến độ debate; việc các điều khoản trong thân spec cùng thực hiện được mới là điều kiện để ghi đã khép.
