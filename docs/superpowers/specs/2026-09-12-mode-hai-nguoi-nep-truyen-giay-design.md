# Spec: Mode hai người — «Nếp truyền giấy»

Ngày: 2026-09-12
Trạng thái: **BẢN THIẾT KẾ CHỜ SOÁT** — chưa phải quyết định đã chốt, chưa phải giấy phép viết code (xem mục 21).
Nguồn: tầm nhìn của Lead (phiên 2026-09-12) về «couple mode» cho đôi lâu năm, cộng bản vision của team về Relationship Twin; bốn vòng thu hẹp trong cùng phiên.
Phạm vi sở hữu: phần màn hình và câu chữ là của Claude (`apps/mobile/`); mọi bảng và route là của Codex và **phải mở ADR trước** (mục 9.3).

---

## 0. Tóm tắt điều hành

Rủ Đi thêm **một loại quan hệ**, không thêm một app thứ hai: hai người có thể nâng cuộc nhắn riêng của mình thành **sổ của hai người**, và nhân vật Nếp đổi việc trong đó.

Bốn phát hiện định hình toàn bộ thiết kế:

**① Đôi lâu năm không thiếu thông tin. Họ thiếu người chịu trách nhiệm.**
«Hôm nay ăn gì?» không phải câu hỏi tìm quán. Nó là **cú đẩy trách nhiệm**: ai quyết thì người đó chịu nếu chỗ đó dở. TikTok, Google Maps và ChatGPT đều đã giải xong bài thông tin. Không ai giải bài trách nhiệm. Suy ra: với đôi lâu năm, sản phẩm **không phải cái máy gợi ý, mà là một quyết định đã xảy ra rồi** — app chọn, nên không ai trong hai người phải là tác giả của lựa chọn đó.

**② Cái nam thiếu không phải gu, mà là bộ nhớ và sự chủ động.**
Người kia đã nói ra hết rồi: nói trong chat, lưu một cái quán, buột miệng ba tuần trước. Anh ấy **có nghe**, chỉ là không giữ lại. Nên việc đầu tiên của Nếp không phải đoán gu, mà là **không cho những câu đó bay mất**.

**③ Nói ra thì thành đòi hỏi.**
Người kia không giấu gu. Vấn đề là «em muốn đi chỗ yên tĩnh» nghe như đang phê bình mấy chỗ đã chọn trước đó. Nên Nếp phải là **người thứ ba mà người ta nói được**, để không phải nói thẳng với nhau. Đây là lý do bản «Nếp hỏi công khai» cho ra **nhiều** thông tin hơn bản «Nếp thu thập ngầm», chứ không ít hơn.

**④ Trung bình hai cái gu ra đúng buổi tối tệ nhất.**
Trộn hai vector sở thích rơi vào thung lũng giữa hai đỉnh: chỗ nào cũng tàm tạm, chẳng ai thích thật. Cách đúng là **chia lượt trong cùng một buổi**, không phải lấy trung bình (mục 4.5).

**Mức độ chắc chắn.** Cả bốn phát hiện là **luận đề chưa có bằng chứng hành vi**, và ADR-0006 vẫn đang gác Giai đoạn 0. Phát hiện ① là giả định số một: nếu người ta thật ra muốn tự chọn, toàn bộ cơ chế «kèo tự tới» sai. Mục 13 tồn tại để đo nó với một tiêu chí giết viết trước.

---

## 1. Câu chuyện

Mục này là phần dài nhất **có chủ ý**. Mọi cơ chế ở mục 4, 5, 6 đều suy ra được từ đây; nếu một cơ chế không suy ra được từ câu chuyện thì nó là feature vay của app khác và phải bị cắt.

### 1.1 Rủ Đi là một hành động, không phải một danh mục

Tên app không nói về du lịch, không nói về ăn uống, không nói về hẹn hò. Nó là **một câu người Việt nói với nhau mỗi tuần**:

> «Đi đâu không?»

Ba chữ đó có một tính chất lạ: **nói ra là đã mất gì đó rồi.** Người rủ tự đặt mình vào chỗ có thể bị từ chối, và tự nhận trách nhiệm cho cái chỗ sắp tới. Nên càng thân lâu, càng ít ai rủ. Không phải vì hết muốn đi, mà vì hết muốn là người rủ.

Việc của app, nói cho gọn nhất: **nhận lấy phần mất mát của câu rủ.**

### 1.2 Nếp hiện tại: một nếp gấp trong cuốn sổ của cả hội

Trong hệ hình ảnh v2 (ADR-0020) và ba đợt bản sắc sau đó, app không phải cái màn hình mà là **một cuốn sổ chuyến đi của cả hội**: ngày là **trang giấy mở**, đêm là **sổ đóng lại trên bàn, tờ giấy nằm trên bìa vải**. Nơi nào chưa có ảnh thì có **một nét ký hoạ vẽ vội** cạnh tên, như người ta vẽ vào sổ.

Nếp là nhân vật của cuốn sổ đó: **một nếp gấp của giấy**. Nó không có mặt, không có mắt, không nói chuyện thành tiếng. Nó là chỗ tờ giấy gập lại — nên nó vốn đã mang hai tính chất mà không cần giải thích: **nó giữ được cái gì đó bên trong**, và **nó mở ra được**.

Ở hội bạn, Nếp là **một trang**. Nhiều người viết vào, ai cũng đọc được, và việc của Nếp là hứng chữ.

### 1.3 Sổ của hai người thì mỏng

Đây là chỗ chuyển của câu chuyện, và nó phải là **một sự kiện có thật trong app**, không phải một lời giới thiệu.

Khi hai người quyết định có một cuốn sổ riêng, sổ ấy mỏng đi: không còn sáu người viết vào, chỉ còn hai. Nên **Nếp tự gấp mình lại nhỏ bằng lòng bàn tay**, để nằm được trong túi áo. Và nó đổi việc:

> Ở hội, Nếp là **trang giấy**.
> Ở hai người, Nếp là **mảnh giấy gấp được truyền tay**.

Hình này không phải ẩn dụ đi vay. Nó là vật mà người Việt nào cũng có trong ký ức: **mảnh giấy gấp vuông truyền dưới bàn, trong lớp học.** Nó nhỏ, nó riêng, nó không phải tin nhắn — tin nhắn thì gửi xong là xong, còn mảnh giấy thì **có người mang đi**, và **có lúc mở**.

### 1.4 Ba tính chất của tờ giấy gấp là ba tầng sản phẩm

Cả mode chỉ có **một vật**: giấy gấp. Ba tính chất của nó sinh ra đúng ba tầng tính năng, và không tầng thứ tư nào được phép mọc ra.

| Tính chất | Nghĩa trong quan hệ | Tầng sản phẩm |
|---|---|---|
| **Nếp gấp lại thì không ai đọc được** | có thứ chưa tới lúc; có món quà đang chuẩn bị | **riêng tư**: túi riêng, trang sổ riêng |
| **Nếp mở ra đúng lúc** | đúng thời điểm quan trọng hơn nội dung | **thân mật**: mảnh giấy, hẹn mở, thư gửi năm sau |
| **Nếp giữ được chữ viết tay** | thứ viết hôm nay là quà cho hai người của năm sau | **hiểu nhau**: hai quyển sổ, ôn thẻ, giấy cũ quay lại |

Và vì Nếp là cái **đi lại giữa hai người**, nó có một vị trí kiến trúc rất rõ:

```text
      Sổ về em                  Nếp                   Sổ về anh
      (một người giữ)      truyền tay, giữ giùm      (người kia giữ)
            │                     │                        │
            └──── đọc bên này, hỏi bên kia, mang trang qua ─┘
```

Nếp giữ đúng **ba** thứ, và việc giới hạn ở ba là một quyết định thiết kế:

1. **mảnh giấy** hai người gửi nhau,
2. **cái kèo tuần này**,
3. **một cái túi mà chỉ một người mở được**.

### 1.5 Vì sao mode này phải khác hội bạn, chứ không phải hội bạn thu nhỏ

Bốn khác biệt cấu trúc, mỗi cái giết một feature của hội bạn nếu bê nguyên sang:

| | Hội bạn | Hai người |
|---|---|---|
| Số ý kiến mỗi buổi | sáu | hai |
| Cách ra quyết định | **bình chọn** hợp lý | bình chọn giữa hai người là **một cuộc thương lượng**, không phải bầu cử |
| Ai tham gia | ai cũng phần nào | **một người lo gần hết** — và sau nhiều năm điều này đã đóng cứng |
| Chỗ dở thì sao | câu chuyện cười | «anh chọn chỗ dở» |
| Tiền | chia bill, ai nợ ai | sổ nợ giữa hai người là **phản cảm**; đúng hình là **chi tiêu chung** |
| Dữ liệu sinh ra | nhiều, nhanh | **ít** — khoảng bốn buổi một tháng |

Hàng cuối là cái bẫy lớn nhất của bản vision gốc: mode được gọi là moat lại chính là mode **đói dữ liệu nhất**. Lời giải không phải mô hình giỏi hơn, mà là **đừng suy ra, hãy hỏi và hãy ghi** (mục 4.1, mục 6).

Hàng «chỗ dở thì sao» là lý do mọi cơ chế trong mode này phải **sai rẻ**: một cú bấm đổi, không kèm lời xin lỗi.

### 1.6 Vì sao đôi lâu năm, không phải couple nói chung

Bản vision gốc viết cho couple năm nhất. Dấu hiệu nằm ở con số: **một buổi khoảng tám trăm nghìn**. Đôi năm thứ năm không chi như thế mỗi cuối tuần; họ đi **nhiều hơn, rẻ hơn, gần hơn**, và phần lớn buổi là «ăn gì đó rồi về». Mọi mặc định của mode phải theo đối tượng thật: **ngân sách thấp hơn, bán kính nhỏ hơn, tần suất cao hơn, và kỳ vọng mới lạ thì cao hơn** (vì họ đi hết chỗ quen rồi).

Ba việc họ không tự làm được, lấy nguyên văn theo Lead:

1. **không tìm được quán mới**,
2. **không có sự đổi mới**,
3. **không tự lên routine**.

Cả ba đều là việc app làm thay, và mục 4.3–4.6 là câu trả lời cho từng cái. Đặc biệt (3): **routine chính là sản phẩm**, không phải một tính năng phụ.

### 1.7 Vật liệu và hình

Tiếp nối cái đã ship, không mở hệ hình ảnh thứ hai:

| | Vật liệu | Đã có |
|---|---|---|
| Ngày, hội bạn | trang giấy mở, vân giấy | đã ship |
| Đêm, hội bạn | sổ đóng trên bàn: vân vải làm nền, hình vẽ là giấy đêm | đã ship (PR #603) |
| **Hai người** | **mảnh giấy gấp trong túi áo**: nhỏ hơn, có vết gấp, hơi cũ mềm | **mới** |

Khớp đẹp nhất của mode này: ở hội, Nếp là *một nếp gấp trong cuốn sổ*; ở hai người, **cả màn hình là một mảnh giấy gấp, và Nếp chính là đường gấp đó**. Đường gấp thành chữ ký thị giác — cùng token, cùng mực, cùng nét ký hoạ, **không** thêm hệ màu, **không** thêm bộ icon.

Chi tiết hình học để lượt sau đo: tờ giấy hai người **hẹp hơn** tờ của hội (mảnh giấy gấp vuông), có **hai vết gấp** chia ba phần dọc — và ba phần đó trùng đúng ba chặng của một buổi đi (mục 4.5), nên bố cục mang nghĩa chứ không trang trí.

### 1.8 Một câu

> **Không ai rủ ai nữa. Để Nếp truyền giấy.**

---

## 2. Hai vai

Không phải hai tính cách, không phải hai theme. Là **hai việc khác nhau trong cùng một buổi đi, mỗi người một việc, không ai làm cả hai.**

Tên trong sản phẩm: **Người lo** và **Người chấm**. Hai chữ này là tiếng Việt đời thường («anh lo hết», «em chấm chỗ này»), nên không cần ai học.

### 2.1 Người lo

Năm việc. Bốn trong năm là **việc hiểu người kia** — đây là lý do một trợ lý có chỗ đứng ở vai này.

| Việc | Thất bại thường gặp | Nếp gánh phần nào | Cơ chế |
|---|---|---|---|
| **Quyết** chỗ | né quyết, đẩy lại «em muốn gì?» | phác sẵn cả buổi, chỉ sửa một thứ | 4.1, 4.2 |
| **Lo hậu cần**: đặt chỗ, mở cửa không, đi bằng gì | quên, tới thì đóng | nhắc trước một ngày | 4.1 |
| **Nhớ**: người kia từng nói muốn thử chỗ nào | **thất bại nặng nhất** | giữ hộ, không cho bay mất | 6.3, 6.4 |
| **Để ý**: hôm nay muốn ồn hay yên | đoán sai | hỏi giùm từ đầu tuần | 6.4 |
| **Chủ động**: rủ mà không bị đòi | im ba tuần | trao lượt rủ kèm kèo làm sẵn | 4.2 |

### 2.2 Người chấm

Phải nói rõ **vai này được gì**, không thì mode một chiều, người ở vai này không mở app, và cơ chế chết vì thiếu một nửa.

| Việc | Được gì |
|---|---|
| **Nói ra** — nhưng nói với Nếp, không phải đòi người kia | muốn cái gì mà không biến thành yêu cầu |
| **Chấm** — chọn đúng một chi tiết: món tráng miệng, nhạc, chỗ đi sau | góp phần mà không phải lên kế hoạch |
| **Phán** sau buổi: thích / thôi khỏi | lần sau tốt lên thật |
| **Ghim** một trang vào sổ người kia | nhắc mà không thành nhắc dai |
| **Gửi giấy** | phần thân mật |

Câu tóm lại phần thưởng của vai này: **được muốn thành tiếng mà không thành đòi hỏi, và được ngạc nhiên bởi một người bỗng nhớ hết mọi thứ.**

### 2.3 Gán vai và đổi vai

- Lúc gấp giấy, **mặc định** gán **Người lo** cho người khởi xướng nghi thức, vì người khởi xướng thường là người lâu nay vẫn lo.
- Vai **đổi được** trong Cài đặt của sổ, và **gậy đổi lượt hàng tuần** (4.2) vẫn luân phiên bất kể ai giữ vai.
- Vai **không** lưu theo giới tính. Lý do kỹ thuật, không phải lý do khác: cơ chế cốt lõi là **chuyển cái gậy sang người lâu nay không cầm**, nên chủ thể phải là *người đang giữ vai*, không phải một cột giới tính. Một cột giới tính còn làm mọi truy vấn sai với những đôi mà người lo là người còn lại.

### 2.4 Hai giọng của Nếp

Luật, không phải hướng dẫn viết câu:

> **Với Người lo, Nếp nói bằng gợi ý.**
> **Với Người chấm, Nếp nói bằng câu hỏi.**

Hệ quả bắt buộc:

- Nếp **không** kể cho Người chấm biết Người lo đang chuẩn bị gì (chết phần bất ngờ).
- Nếp **không** đưa Người lo nguyên văn lời Người chấm (Người lo thành thụ động, Người chấm thành bị soi). Chỉ đưa **một gợi ý đã gói lại**: «đang muốn yên tĩnh», không phải bản ghi.

### 2.5 Không biên nhận

Cơ chế nhỏ nhất trong doc này và là cơ chế quan trọng nhất về mặt cảm xúc.

Khi Người chấm trả lời câu hỏi tuần của Nếp, người ấy **biết** câu trả lời sẽ tới tay người kia. Nhưng:

> **Người chấm không thấy biên nhận.** Không biết người kia đã xem chưa, có chọn theo không, lúc nào.

Nên khi thứ Bảy người kia xuất hiện với một chỗ yên tĩnh và đúng cái món đã nhắc, **nó vẫn giống như người ấy tự nhớ**. Có nói với Nếp, nhưng không phải người sắp xếp. Đôi nào cũng ngầm muốn giữ đúng khoảng mờ đó, và đây là chỗ khoảng mờ ấy sống.

Cái bị ẩn là **thời điểm và cách gói**, không bao giờ là **việc Nếp có hỏi**. Bản ngầm hoàn toàn thì được thêm vài tuần rồi vỡ đúng một lần và vỡ hẳn: người ấy nhận ra app là thứ đã đi báo cho người kia, và mất cả hai người cùng lúc.

---

## 3. Vào mode và ra khỏi mode

### 3.1 Vào là một nghi thức, không phải cái nút

Không có switch `Hội bạn | Hai người` ở tầng app, vì **không có mode nào để
switch** — có nhiều sổ, mỗi sổ một loại (mục 17). Cái duy nhất cần một nghi
thức là **đổi loại của một sổ**: **một mảnh giấy phải hai người cùng gấp**,
một người gấp nửa đầu và gửi, người kia gấp nửa sau.

Ba thứ được giải bằng một cử chỉ:

1. **Consent hai chiều, tường minh.** Không ai bị kéo vào. Đây là điều kiện bắt buộc, xem mục 10.
2. **Khoảnh khắc cảm xúc lúc onboard** — thay cho một trang giới thiệu tính năng.
3. **Một mốc ngày có thật** để sau này Nếp nhắc «hai bạn gấp mảnh giấy đầu tiên ngày …».

Lời mời gấp giấy **hết hạn** (đề xuất: bảy ngày) và **không nhắc lại**. Một lời mời loại này bị nhắc lần thứ hai là một áp lực.

### 3.2 Ra là mở giấy ra

Một cử chỉ đối xứng và có thật: **mở mảnh giấy ra**. Không phải dialog «xoá quan hệ?».

Luật sau khi mở giấy, cần Lead chốt ở mục 14:

- Sổ hai người **đóng lại**, không xoá. Quan hệ trở về cuộc nhắn riêng bình thường (`pair` như hiện nay).
- **Sổ về người kia** của mỗi người là của người ấy: giữ nguyên, riêng như cũ.
- **Mảnh giấy đã mở** thuộc cả hai, giữ nguyên. **Mảnh giấy chưa tới lúc mở** thì không bao giờ mở nữa.
- **Túi riêng** giữ nguyên chủ.
- Một người mở giấy là đủ. Không cần hai người đồng ý mới ra được — điều kiện «phải cả hai đồng ý mới ra» là một cái bẫy.

### 3.3 Ranh giới với cuộc nhắn riêng đang có

Rất quan trọng, vì hạ tầng đã có sẵn và dễ hiểu lầm.

`contexts.kind` hiện có đúng hai giá trị `('group', 'pair')` theo ADR-0021 §2.5, trong đó **`pair` là cuộc nhắn riêng giữa hai người bất kỳ**, kèm `pair_key` là khoá hai người có thứ tự, unique.

> **«Đôi» không phải là `pair`.** Mọi đôi là một `pair`, nhưng hầu hết `pair` không phải đôi.

Nên «đôi» là **một trạng thái được bật thêm trên một `pair` bằng nghi thức gấp giấy**, và:

- **Tuyệt đối không suy ra từ hành vi.** Không có chuyện «hai người này nhắn nhau nhiều nên chắc là đôi». Đây là kịch bản tệ nhất mà app này có thể tạo ra.
- Một người **chỉ có một sổ hai người tại một thời điểm** (đề xuất; xem mục 14).

---

## 4. Tầng quyết định

### 4.1 Kèo tự tới

**Vấn đề nó giải:** phát hiện ① — không ai muốn là người quyết.

**Cài một lần:** đôi khai **một khung** («tối thứ Bảy») và **routine hiện tại** của họ («tụi mình hay ăn tối rồi cafe»). Hai thứ này do người dùng khai, **không** suy ra, **không** cần lịch, **không** cần thời tiết, **không** cần vị trí thiết bị (ADR-0018 vẫn cấm quyền vị trí).

**Mỗi tuần, một mốc trước khung ấy** (đề xuất: 48 giờ trước): hai máy thấy **cùng một thẻ**. Một buổi tối. Đã chọn. Kèm lý do. **Không phải feed, không phải mười thẻ — một.**

```text
Thứ Bảy này

18:30  (một quán trong danh mục)
20:00  Đi bộ, rồi chè

Vì: ba tuần liền hai bạn ăn ở cùng một khu.
Chỗ này cách mười phút, chưa từng đi.

[ Ừ ]      [ Đổi ]      [ Tuần này nghỉ ]
```

**Ba luật làm nên toàn bộ giá trị:**

1. **«Đổi» rẻ một cú bấm và không kèm lời xin lỗi.** App đổi một lần rồi gửi lại; hết hai lần thì nhường lại cho người chọn tay. **App được phép sai, miễn sai rẻ.**
2. **«Tuần này nghỉ» phải có.** Đôi lâu năm có tuần về nhà, có tuần mệt. Thiếu nút này thì Nếp thành đứa nhắc dai và bị tắt thông báo trong ba tuần.
3. **Không ai trong hai người là tác giả của lựa chọn.** Đây là điều khiến «Đổi» không còn là chối người kia.

**Và đây là chỗ hay nhất về kỹ thuật:** mỗi *Ừ / Đổi* là **một nhãn dữ liệu của cặp**, mỗi tuần một cái, rẻ, và **thuộc về cặp chứ không thuộc về cá nhân nào**. Nó đi vòng qua đúng cái bẫy riêng tư ở mục 10.1 — hành vi chung là dữ liệu chung **theo cấu tạo**, không phải do nới luật riêng tư ra.

**Cold start:** bốn tuần đầu chạy bằng routine **do họ tự khai**, không phải suy ra. Sáu tuần là có sáu ví dụ có nhãn.

### 4.2 Gậy đổi lượt

**Vấn đề nó giải:** phát hiện ② và vế «không ai chủ động» của mục 2.1.

Mỗi tuần Nếp **trao quyền rủ cho một người**, luân phiên.

| | Người giữ gậy tuần này | Người kia |
|---|---|---|
| Thấy gì | kèo **đã phác xong**, riêng mình, **trước** người kia | một lời rủ, **từ người yêu mình** |
| Làm gì | **sửa đúng một thứ** rồi gửi | nhận, và **chấm một chi tiết** |
| Được gì | công trạng của việc rủ, tốn công gần bằng không | sau nhiều năm, **được rủ một lần** |

Đây là cơ chế tôi đánh giá cao nhất trong cả doc: nó chuyển **cái công** sang app, nhưng để lại **cái công trạng** cho người giữ gậy. Và app biến mất đúng lúc cần biến mất — người nhận thấy một lời rủ của người mình yêu, không thấy một thông báo của phần mềm.

Luật: nếu người giữ gậy **không gửi** trước khung, Nếp gửi thẳng dưới tên Nếp (không để buổi tối chết vì một người quên), và **tuần sau gậy vẫn sang người kia** (không phạt, không nhắc lại chuyện đã quên).

### 4.3 Núm độ mới

**Vấn đề nó giải:** «không tìm được quán mới», «không có sự đổi mới».

Với đôi lâu năm, trục đáng quan tâm **không phải** *romantic / chill / active*. Nó là **y như cũ ↔ chưa từng thử**. Một núm, ba khấc:

| Khấc | Nghĩa | Nói được lý do |
|---|---|---|
| **Chỗ cũ** | lấy từ log của chính họ | «lần cuối bảy tháng trước» |
| **Cùng kiểu, chỗ mới** | giữ hình dạng buổi tối, đổi quán. **Khấc mặc định** | «cùng kiểu chỗ hai bạn hay đi, quán này chưa» |
| **Kiểu chưa thử** | một loại **không** có trong N buổi gần nhất | «hai bạn chưa từng đi loại này» |

Tính được từ dữ liệu **đã có trên `main`**: `outing_stops.place_id` → `places.category`, `places.destination_id`, `places.price_min_vnd`/`price_max_vnd`. Không cần bảng mới cho phần này.

Đây là bản **thấy được** của «novelty score» trong bản vision: người dùng xoay núm, mô hình không đoán giùm. Và nó buộc app trung thực: khấc «chưa thử» chỉ đề nghị được những loại mà danh mục thật sự biết (ADR-0017 cấm điền số hợp lý vào cột rỗng).

### 4.4 Hạn mức, không phải điểm số

**Vấn đề nó giải:** «không tự lên routine» — và nó là câu trả lời của tôi cho «routine phải mới lạ hơn».

```text
Mỗi tuần    một kèo tự tới          routine do app dựng, họ chỉ ừ
Mỗi tháng   MỘT LẦN LẠ              bắt buộc: một loại chưa từng thử
Mỗi quý     một chuyến xa hơn       ra khỏi bán kính quen thuộc
```

Hạn mức hơn điểm số ở ba điểm, và đây là một lựa chọn thiết kế có chủ ý:

1. **Thấy được** — họ biết tháng này còn nợ một lần lạ. Một câu Nếp nói được, và nó làm người ta đi.
2. **Công bằng** — không phải một mô hình phán «hai bạn đang nhàm».
3. **Bảo đảm** — đổi mới xảy ra theo nhịp đã hứa, **không phụ thuộc mô hình đoán đúng**. Với một mode chỉ có bốn điểm dữ liệu mỗi tháng, đây là khác biệt giữa có đổi mới và không.

### 4.5 Chia lượt ba chặng, không lấy trung bình

**Vấn đề nó giải:** phát hiện ④, và yêu cầu của Lead về «chung hoà sở thích hai người để buổi đi vui hơn».

Trung bình hai cái gu ra thung lũng. Cách đúng:

```text
Chặng 1     gu của Người chấm        người ấy thích thật
Chặng 2     vùng giao của hai người  cả hai đều được
Chặng 3     gu của Người lo          người ấy thích thật
```

Mỗi người **được đúng một thứ mình thật sự thích mỗi buổi**, thay vì hai thứ cả hai đều chỉ chịu được. Nếp theo dõi **lượt nào bị bỏ** (buổi bị cắt ngắn, chặng bị đổi) và **trả lại lượt đó buổi sau**, nên công bằng theo thời gian mà không ai phải đếm.

Và **hạn mức một lần lạ mỗi tháng không thuộc ai cả.** Đây là chỗ ý hay nhất của bản vision («A cộng B là một entity khác») có cơ chế thật: **cặp có cái gu thứ ba, không của người này, không của người kia.** Chỗ chưa ai từng thử là chỗ duy nhất hai người **cùng là người mới**.

Ba chặng này trùng ba phần của mảnh giấy gấp (mục 1.7): bố cục mang nghĩa.

### 4.6 Nghi thức có tên

Cho họ **tự đặt tên một nghi thức và chia hai việc**:

```text
Sáng chủ nhật     một người chọn quán, người kia chọn nhạc
Tối thứ Bảy       một người chọn món, người kia chọn chỗ đi sau
```

Nếp **bảo vệ** nghi thức: không bao giờ đề nghị đè lên nó. Và chỉ **đổi bên trong** nó: cùng hình dạng, chỗ mới.

Đây là lời giải cho câu hỏi khó nhất của bản vision — *lúc nào cần quen, lúc nào cần mới* — trả bằng **một cái điều khiển người dùng thấy**, không bằng một điểm số ẩn. Và nó là chỗ hai vai ở mục 2 trở thành **của họ** chứ không phải của app: tên nghi thức và hai việc do họ viết.

---

## 5. Tầng thân mật

### 5.1 Mảnh giấy

Đây là hiện thực của **F38 «Locket Style Widget»** đang nằm ở `product/feature_list.md` với ghi chú «Optional later» — mode hai người là ngữ cảnh làm nó có nghĩa.

Gửi cho **đúng một người**. Trong giấy để được: một tấm hình · một dòng chữ · một chỗ muốn đi · một mảnh ký ức.

Khác story (ADR-0022 §2.3) ở ba điểm, nên **không** dùng lại `stories`:

| | Story | Mảnh giấy |
|---|---|---|
| Người đọc | bạn bè | **đúng một người** |
| Hạn | sống 24 giờ rồi hết | **không hết hạn**; chỉ có mốc **bắt đầu mở được** |
| Ý nghĩa | khoe một lúc | **giữ cho về sau** |

### 5.2 Hẹn mở

| Hẹn mở | Dùng làm gì |
|---|---|
| **Bây giờ** | như Locket thường: hiện ngay trên màn người kia |
| **Tối nay** | «có cái này cho em, tối mở»: cả buổi chiều có cái để chờ |
| **Lần tới hai người đi cùng nhau** | mở đúng lúc đang ngồi cạnh nhau |
| **Một năm sau** | thư gửi năm sau (mục 5.3) |

**Ràng buộc kỹ thuật quyết định cách làm:** máy chủ **không có việc nền định kỳ**. ADR-0024 §2.3 cho đúng một cửa `AfterResponse` với **đúng hai job** được phép, có test AST gốc gác. Nên:

> **Nếp không có đồng hồ chạy nền.** Mọi «mở đúng lúc» là **một điều kiện lúc đọc**: mảnh giấy có mốc `mo_tu`, và nó **không đọc được** trước mốc đó. Hàng thông báo sinh **ở lần đọc đầu tiên sau mốc**, idempotent nhờ một cột đã-báo.

Hàng «lần tới hai người đi cùng nhau» **không** được làm bằng vị trí: ADR-0018 cấm quyền vị trí, ADR-0026 §2.2 nói rõ bản đồ chiếu toạ độ **địa điểm**, không toạ độ người, và check-in là **cái nút** (F46), không phải cảm biến. Nên điều kiện là: **có check-in ở một chặng của một buổi đi chung**. Đúng thứ đang có, không thêm quyền nào.

### 5.3 Thư gửi năm sau

Hàng cuối của bảng trên là cái móc mạnh nhất và rẻ nhất:

> **Mỗi mảnh giấy gửi hôm nay là một món quà cho hai người của năm sau.**

Nó biến bề mặt Locket và bề mặt ký ức thành **cùng một vật ở hai độ tuổi khác nhau**, không phải hai tính năng phải nuôi riêng.

### 5.4 Hâm nóng: Nếp mở lại một mảnh giấy cũ

Không gợi ý gì cả. Nếp **lôi ra một mảnh giấy hai người từng gửi nhau**, chọn thời điểm:

- tới **ngày kỷ niệm của chính mảnh giấy đó**, hoặc
- khi **kèo tuần này quay lại đúng chỗ đã viết nó** (so `place_id`, không so vị trí).

Đây là vũ khí cảm xúc mạnh nhất của mode, và nó gần như không tốn gì: một truy vấn ngày trên đồ của chính họ. Nó cũng là **lý do để gửi giấy ngay từ đầu** — nếu không có vòng quay lại này thì mảnh giấy chỉ là tin nhắn có hiệu ứng.

### 5.5 Túi riêng

Một ngăn **chỉ một người mở được**. Để dành sinh nhật, kỷ niệm, quà.

**Không có ngăn này thì không có bất ngờ nào tồn tại được** — không thể bất ngờ một người đang đọc cùng cái lịch trình. Và nó khớp đúng nhân vật: **Nếp gấp lại thì không ai đọc được.**

Đây là **khái niệm dữ liệu mới duy nhất** mà tầng thân mật cần (mục 9.2).

---

## 6. Tầng hiểu nhau: hai quyển sổ

Đây là phần Lead thêm vào ở vòng cuối, và nó chữa điểm yếu còn lại của ba vòng trước: tới đó Nếp chỉ là trợ lý của **một** người. Có sổ thì **cả hai đều thành người giữ**, và cả hai đều có nhịp mở app hàng ngày.

### 6.1 Hai quyển, không phải một

> **Hai người, hai quyển sổ. Sổ của người này viết về người kia.**
> **Không ai đọc sổ của người kia.**

Phải là hai quyển riêng và **riêng tư mặc định**, vì trong một quyển chung thì **không ai viết thật**. Một quyển chung sẽ thành một bản tuyên ngôn cho nhau đọc, không phải một cuốn sổ tay.

Tên trong sản phẩm: **«Sổ về em»** / **«Sổ về anh»** — nói đúng nó là gì, không cần giải thích.

### 6.2 Bảy mục đặt sẵn

Không phải một ô ghi chú trống. Ô trống không ai điền; **mục đặt sẵn là thứ làm người ta nhận ra mình chưa biết cái gì.**

```text
GU           ăn gì · uống gì · KHÔNG ăn được gì · cỡ áo · màu hay mặc
ĐỪNG         sợ gì · không thích gì · chỗ nào đừng quay lại
NGÀY         sinh nhật · kỷ niệm · ngày giỗ trong nhà
NGƯỜI        tên mẹ · tên em gái · tên con mèo · tên đứa bạn thân
LÚC MỆT      mệt thì muốn gì: yên / được ăn / được ngủ / được để yên
MUỐN         thứ đã nhắc mà chưa làm
CHUYỆN CŨ    một mảnh ký ức
```

Ba mục là chỗ đôi lâu năm hay vỡ nhất, và chưa app nào nghĩ tới:

- **«không ăn được gì»** — quên là tai hoạ, nhớ là thương. Và nó **lọc thẳng vào gợi ý** (mục 6.7): quán chỉ có một loại món mà người kia không ăn được thì **bị bỏ im lặng**, không hỏi lại.
- **«NGƯỜI»** — «tên em gái của người ta là gì ấy nhỉ». Nhớ được tên mẹ người ta là món lãi cao nhất trong cả cuốn sổ, và là thứ không một mô hình gu nào sinh ra được.
- **«LÚC MỆT»** — mục sâu nhất. Đôi lâu năm cãi nhau đúng chỗ này: một người mệt thì muốn được để yên, người kia xông vào dỗ, và cả hai làm ngược cái người kia cần. **Viết một lần, hết một cái cãi lặp lại nhiều năm.**

### 6.3 Ba người viết vào sổ

| Ai viết | Cách | Giải bài gì |
|---|---|---|
| **Mình tự ghi** | gõ một dòng, lúc nào cũng được | — |
| **Nếp ghi giùm** | Nếp thấy gì thì **đề nghị một trang**, bấm **Ghim** hoặc **Bỏ** | «giúp Người lo ít quên hơn»: không phải viết gì cả |
| **Người kia ghim vào** | «muốn anh nhớ cái này» → thành **một trang trong sổ của người kia** | nói mà không thành nhắc dai |

Hàng thứ hai là toàn bộ câu trả lời cho phát hiện ②: người kia nhắc một món trong chat, Nếp gợi ra một trang, chỉ cần bấm một cái. **Không ai phải viết nhật ký để có nhật ký.**

Hàng thứ ba có **hạn hai trang một tuần**. Đúng vì có hạn nên nó còn dễ thương; bỏ hạn thì nó thành danh sách việc phải làm.

**Luật cho hàng thứ hai, bắt buộc, xem mục 7:** Nếp chỉ đề nghị được những gì **người kia đã có thể biết**.

### 6.4 Bốn chỗ Nếp dùng sổ, không hơn

**① Câu hỏi mỗi tuần giờ có mục tiêu.** Nếp nhìn sổ, thấy mục nào trống nhất thì hỏi đúng mục đó.

```text
Sổ chưa có mục LÚC MỆT:

Hôm nào mệt, em muốn được để yên
hay muốn có người bên cạnh?

[ Được để yên ]   [ Có người bên cạnh ]   [ Tuỳ hôm ]
```

Câu hỏi thôi ngẫu nhiên. Nó thành **một chương trình học về người kia**, và mỗi câu trả lời lấp một ô có tên.

**Một câu mỗi tuần, không hơn** — mười câu là bảng khảo sát, và bảng khảo sát thì người ta bỏ giữa.

**② Ôn một thẻ mỗi ngày** — mục 6.5.

**③ Ba dòng dặn trước buổi đi**, lấy từ sổ nên nó đúng chứ không đoán:

```text
THỨ BẢY NÀY

Em ấy đang muốn: yên tĩnh
Tránh: chỗ đông, ồn
Nhớ: em ấy nhắc bánh canh hai lần rồi
```

Ba dòng, đọc mười giây, làm được ngay. Đây là khoảnh khắc trợ lý thật sự của mode.

**④ Nhắc ngày, trước năm ngày.** Mục **NGÀY** nhắc **sớm** kèm một việc làm được. Nhắc đúng hôm đó thì đã hết kịp chuẩn bị, và đó là lỗi của mọi app nhắc sinh nhật.

```text
Còn năm ngày là sinh nhật mẹ em ấy.
Sổ ghi bà thích hoa lay-ơn.
```

**Ngoài bốn chỗ đó, sổ im lặng.** Không badge, không «bạn chưa ghi gì hôm nay», không chuỗi ngày liên tiếp.

### 6.5 Ôn một thẻ

Món tôi thích nhất trong tầng này. Một lúc rảnh, Nếp lật một trang của sổ **mình đang giữ**:

```text
Em ấy không ăn được tôm.
Còn đúng không?

[ Đúng ]              [ Sửa ]
```

Ba việc cùng lúc:

1. **Giữ sổ không cũ.** Gu người ta đổi; một cuốn sổ không ai soát lại sẽ thành sai sau một năm và làm gợi ý sai theo.
2. **Thật sự khiến người ta nhớ.** Nhớ được là do **lôi ra lại**, không do ghi vào. Đây là lý do cơ chế này là ôn thẻ chứ không phải một trang danh sách.
3. **Một khoảnh khắc ấm mỗi ngày mà Nếp không phải nói câu nào tình cảm.** Nội dung là người mình yêu; Nếp chỉ đưa thẻ ra.

Và đây là **nhịp mở app hàng ngày mà mode đang thiếu**, có cho **cả hai người**, không phải chỉ cho vai Người lo.

Nhịp: **một thẻ một ngày**, bỏ qua được, không đếm chuỗi.

### 6.6 Trang tặng, và cuối năm thì đổi sổ

Mỗi trang gắn nhãn được: **riêng** (mặc định) hoặc **cho người kia đọc**.

Tới kỷ niệm, Nếp đề nghị: **gấp hết những trang «cho người kia đọc» thành một tờ và đưa qua.**

Một năm âm thầm để ý người ta, trao lại thành một vật. Và nó **cùng họ với thư gửi năm sau** (mục 5.3): cả mode chỉ có một vật, khác nhau ở chỗ **bao giờ mở**.

Lý do phải gắn nhãn theo từng trang, không phải mở cả quyển: **biết sẽ bị đọc thì người ta viết khác.** Sổ phải riêng để còn thật; món quà là **bản đã chọn lọc**, do người giữ sổ chọn.

### 6.7 Sổ lái gợi ý, và đây là ràng buộc cứng

Đây là chỗ hai quyển sổ thôi là một app ghi chú và trở thành bộ phận của máy gợi ý.

| Mục sổ | Vai trò trong gợi ý |
|---|---|
| **không ăn được** | **ràng buộc cứng**: loại thẳng, không cần hỏi, không giải thích dài |
| **ĐỪNG** (sợ, chỗ đừng quay lại) | **ràng buộc cứng** |
| **MUỐN** | **ưu tiên**: lấy trước khi đi tìm chỗ mới |
| **GU** | chấm điểm mềm |
| **LÚC MỆT** | chọn hình dạng buổi khi câu hỏi tuần nói đang mệt |
| **NGÀY** | mốc để nhắc, và cớ để nâng hạng buổi đi |
| **NGƯỜI** | **không lái gợi ý**. Chỉ để nhắc và để Nếp gọi đúng tên |
| **CHUYỆN CŨ** | **không lái gợi ý**. Là nguồn cho giấy cũ quay lại (mục 5.4) |

Và đây là câu trả lời thật cho lo lắng «dữ liệu quán còn thiếu»: **ràng buộc đến từ chính hai người, không từ danh mục.** Một danh mục nghèo vẫn gợi ý được đàng hoàng nếu nó biết hai điều họ không ăn được và ba chỗ đừng quay lại. Lead nói phần dữ liệu quán lo sau ở tầng data; mục này là lý do việc đó không chặn mode.

---

## 7. Năm luật cứng của Nếp

Năm luật này là phần **không thương lượng** của thiết kế. Bỏ bất kỳ luật nào thì Nếp thôi là nhân vật và thành một cái máy nhắc.

### Luật 1 — Nếp xen vào bằng sự thật, không bao giờ bằng cảm xúc

Nếp **được phép** xen vào không cần ai hỏi, nhưng chỉ trên một con số lấy từ log của chính hai người:

```text
ĐƯỢC                            CẤM
«Ba tuần rồi.»                  «Hai bạn có ổn không?»
«Bảy tháng chưa quay lại.»      «Dạo này em ấy ít vui.»
«Hôm nay, ba năm trước.»        «Có vẻ hai bạn đang xa nhau.»
«Bốn buổi gần nhất cùng kiểu.»  «Anh nên quan tâm em ấy hơn.»
```

Đây chính là đường ngăn giữa **một nhân vật đáng yêu** và **một app đi hỏi thăm hôn nhân của người ta**. Và vì là sự thật từ log nên nó **không bao giờ sai**: Nếp không cần thông minh, Nếp cần **nhớ dai**.

Hệ quả: Nếp **không** suy diễn trạng thái tình cảm, **không** chấm điểm quan hệ, **không** có thanh «sức khoẻ mối quan hệ». Cái đó vừa không đo được, vừa là câu mà **nếu một người đọc thấy một lần thì mode này chết**.

### Luật 2 — Hai giọng

Với Người lo: **gợi ý**. Với Người chấm: **câu hỏi**. Không bao giờ đưa nguyên văn (mục 2.4).

### Luật 3 — Nếp chỉ mang sang những gì người kia đã có thể biết

Nếp được ghi vào sổ, hoặc gói thành gợi ý, **đúng bốn nguồn**:

1. người ấy **nói trong chat chung** của hai người,
2. người ấy **lưu một chỗ rồi đẩy vào sổ chung**,
3. người ấy **chọn trong một buổi đi chung** (chấm, check-in, đổi chặng),
4. người ấy **trả lời Nếp**, khi đã biết câu trả lời tới tay người kia.

**Nếp không bao giờ mang sang một thứ người ấy giữ riêng** — `saved_places` chưa chia, `person_interests` (ADR-0019 §2.1 đã nói rõ những hàng ấy là của riêng người đó và `GET /people/{id}` không bao giờ mang chúng), túi riêng, trang sổ nhãn «riêng».

Một luật, **kiểm được bằng test**, và nó là lý do hai quyển sổ là *sổ tay của người đang yêu* chứ không phải *hồ sơ*.

### Luật 4 — Không biên nhận

Người chấm không thấy người kia đã xem chưa, có làm theo không, lúc nào (mục 2.5).

### Luật 5 — Nếp có hạn mức nói

Mục 8. Một nhân vật xen vào mỗi tuần một lần là bạn; cùng nhân vật đó xen vào mỗi ngày là cái app bị tắt thông báo.

---

## 8. Hạn mức nói

Đây là thứ duy nhất tôi cho là **sẽ giết mode này** nếu làm sai: **Nếp nói nhiều.**

| Việc | Nhịp tối đa | Tắt được |
|---|---|---|
| Kèo tự tới | **một lần một tuần** | tắt cả mode kèo |
| Câu hỏi cho Người chấm | **một câu một tuần** | có |
| Ôn một thẻ | **một thẻ một ngày**, im lặng trong app, **không** push | có |
| Nếp nhắc (sự thật) | **tối đa hai lần một tháng** | có |
| Nhắc ngày trong sổ | theo mốc, **trước năm ngày**, một lần | có |
| Mảnh giấy tới | theo sự kiện thật, **không hạn** | có |
| Giấy cũ quay lại | **tối đa một lần một tháng** | có |

Ba luật kèm theo:

1. **«Tuần này nghỉ» luôn có mặt** trên thẻ kèo, và bấm vào thì tuần đó Nếp im hoàn toàn.
2. **Không streak, không badge, không «bạn chưa …».** Một app về quan hệ mà dùng cơ chế chuỗi ngày là đang lấy cảm giác tội lỗi làm động lực; trong mode này nó độc.
3. **Bảng trên là hạn mức *theo sổ*.** Một người có năm sổ thì Nếp nói năm
   lần, nên còn cần **một trần toàn cục theo người** — xem mục 17.6. Thiếu tầng
   đó thì mục này là lời hứa chỉ đúng với người có đúng một sổ.
4. Push tuân đúng ADR-0024: `people.notify_prefs` tắt được **từng loại**, và payload push **không bao giờ mang nội dung** — thông báo mảnh giấy nói «có một mảnh giấy», không nói trong đó viết gì.

---

## 9. Dữ liệu

### 9.1 Đã có trên `main` — dùng lại, không dựng lại

| Cần cho | Đã có |
|---|---|
| Quan hệ hai người | `contexts.kind = 'pair'` + `pair_key` unique (ADR-0021 §2.5), `memberships` |
| Buổi đi và chặng | `outings` (ngày, headcount, ngân sách tham chiếu), `outing_stops` (`position`, `minute_of_day`, `label`, `place_id`), `outing_stop_checkins` (F46, **không** chứa vị trí) |
| Encoding độ mới | `places.category`, `places.destination_id`, `places.price_min_vnd`/`price_max_vnd`, `destinations` |
| Ký ức, bản đồ của hai người | `memories` (có toạ độ **của địa điểm**), `memory_reactions`, `memory_comments`; chế độ xem Hành trình của ADR-0026 |
| Ảnh trong mảnh giấy | `uploaded_images.purpose = 'personal'` + `/people/{id}/photos/{id}` (ADR-0022 §2.1) |
| Tắt từng loại thông báo | **chỉ** `people.notify_prefs` (JSONB). Xem cảnh báo dưới bảng |
| Gu cá nhân (riêng) | `person_interests`, `saved_places` |
| Chi tiêu chung | `confirmed_allocations` trong context đó — `spend_vnd` là **phần** của người, nên «chi tiêu chung tháng này» là **một phép đọc**, không cần luật domain mới |
| Người kia chưa cài app | `guest_links` (tồn tại một lần, máy chủ chỉ giữ digest) |

**Cảnh báo, đã kiểm trên cây ngày 12/09:** ADR-0024 đã **CHẤP NHẬN** nhưng
**lát thông báo chưa có trên `main`**. `services/api/app/db/models.py` chỉ có
`people.notify_prefs` (comment ngay tại cột nói nó được tạo sớm «so the
notifications slice does not…»); **không có bảng `notifications`, không có
`notification_devices`, không có migration nào chứa `notification`** trong 36
bản. Nên **Nếp chưa có đường nói ra ngoài app**, và Đợt 1 phải thiết kế để
không cần nó (mục 12).

### 9.2 Khái niệm mới cần thêm — đề xuất để Codex quyết hình

Liệt kê theo **khái niệm**, không phải theo DDL, vì hình bảng là quyền của Codex.

1. **Trạng thái «đôi» trên một `pair`.** Ai gấp giấy, ngày gấp, ai giữ vai nào, khung tuần đã khai, routine đã khai, khấc núm độ mới. **Phải là một bảng một hàng cho mỗi `pair` đã bật.**

   **CẤM thêm giá trị thứ ba vào `contexts.kind`.** Đã tra bốn chỗ vỡ nếu làm
   vậy: `schemas.py` `ContextKind = Literal["group", "pair"]` (hợp đồng wire),
   `service.py` `if summary.kind != "pair" or summary.counterpart is None:
   continue` (một `kind` thứ ba **rơi khỏi** đường nhắn riêng, mất tên người
   đối diện), `repository.py` `pair_ids = [… if context.kind == "pair"]`, và
   hai chỗ `display_name_for("pair", …)`; cộng CHECK `kind IN ('group','pair')`
   trong migration `6d2b8f4e0c53`. Một đôi **vẫn là một `pair`** theo mọi
   nghĩa hệ thống; «đôi» chỉ là một hàng phụ trỏ vào nó.

   **Ràng buộc bắt buộc:** bật được chỉ khi `kind = 'pair'`, và cần **hai**
   hàng chấp thuận (nghi thức gấp giấy), không phải một.
2. **Mảnh giấy.** Tác giả · context · nội dung (chữ, hoặc `PersonPhotoUrl` của chính tác giả, hoặc `place_id`, hoặc trỏ tới một ký ức) · **mốc mở được** · **điều kiện mở** (bây giờ / tối nay / lần tới đi cùng / một năm sau) · lúc đã mở · lúc đã báo. Không hết hạn. **Đọc được khi và chỉ khi** đã qua mốc, và người đọc là một trong hai người.
3. **Túi riêng.** Đơn giản nhất: một nhãn trên mảnh giấy nói ai đọc được — **người kia** hay **chỉ mình**. Không nên thành bảng thứ hai.
4. **Trang sổ.** Chủ sổ · viết về ai · context · **mục** (một trong bảy, tập đóng) · nội dung · **nhãn chia** (riêng / cho người kia đọc) · ai tạo (mình / Nếp đề nghị / người kia ghim) · lần ôn cuối. **Một bảng phục vụ cả hai quyển** nhờ cặp «chủ sổ, viết về ai». Ôn thẻ chỉ cần cột lần-ôn-cuối, **không** cần bảng riêng.
5. **Kèo tuần.** Một kèo là một buổi đi **được đề nghị**. `outings` hiện **không có cột trạng thái**, và **không được thêm**: `outings` là bảng **dùng chung với hội bạn** (Timeline,
   Hành trình ADR-0026, ngân sách, check-in đều đọc nó), nên một cột trạng thái
   mới ở đó đổi hành vi của nhóm. **Luật: một bảng kèo riêng, sinh ra một hàng
   `outings` khi kèo được chốt.** Giữ `outings` đúng nghĩa «kế hoạch đã có
   thật»; kèo chưa ai ừ thì chưa phải kế hoạch.
6. **Nhãn Ừ / Đổi.** Kèo · người · phán quyết · lúc nào. Đây là **dữ liệu học của cặp** và là số đo chính ở mục 13.
7. **Thêm loại thông báo.** `notifications.kind` là **tập đóng** (ADR-0024 §2.1) nên các loại mới phải khai vào đó: **kèo tuần**, **mảnh giấy tới**, **nhắc ngày trong sổ**. Kèm `notify_prefs` cho từng loại.
   **Ôn thẻ không phải thông báo**: nó là một thẻ nằm trên màn sổ, im lặng, không sinh hàng, không bao giờ push (mục 8).

**Không cần bảng cho:** hạn mức (suy từ log), gậy đổi lượt (suy từ tuần và một cột ai-giữ), chia lượt ba chặng (suy từ chặng và chủ của từng chặng), độ mới (suy từ `outing_stops` và `places`).

### 9.3 Phải mở ADR trước khi viết code

| ADR | Việc |
|---|---|
| **ADR mới** | «Đôi là một trạng thái bật thêm trên `pair` bằng nghi thức hai chiều; hai quyển sổ riêng tư mặc định; mảnh giấy có mốc mở và không có việc nền» |
| **ADR-0021** | mở rộng nghĩa của `kind`/`pair`: `pair` **không** tự là đôi |
| **ADR-0019** | thêm khoản: (a) trong nhóm hai người, luật «chỉ hiện tổng» **không còn bảo vệ được ai** (mục 10.1); (b) Nếp nói **giọng cặp**, không giọng cá nhân; (c) sổ là **quan sát của một người**, không phải bản sao hàng gu của người kia |
| **ADR-0024** | thêm các `kind` thông báo mới; khẳng định push không mang nội dung mảnh giấy |
| **ADR-0022** | khẳng định mảnh giấy **không phải** story (người đọc, hạn, ý nghĩa đều khác) |
| **ADR-0018 / ADR-0026** | **không đổi**: không xin quyền vị trí, không geofence. «Lần tới đi cùng nhau» dùng check-in F46 |
| `product/feature_list.md` | **F38** ra khỏi «Optional later»; các cơ chế mới lấy số từ F48 trở lên |

### 9.4 Tuyệt đối không đụng

- Ba luật về tiền (số nguyên đồng · tổng phân bổ đúng bằng khoản chi · số dư tính lại được từ sổ). Mode này **không chạm cột tiền nào**; «chi tiêu chung» là phép đọc.
- `phase0/` và `docs/protocol/v1/` đóng băng.
- `db/`, `api/`, `domain/` là của Codex. Claude chỉ làm `apps/mobile/` và `app/web/`.
- Không đưa vào Git: ảnh bill, số tài khoản, **tên người thật**, transcript thô, export, `.env` thật. Mọi ví dụ trong doc này dùng **tên vai**, không dùng tên người.

---

## 10. Riêng tư và consent

Mục này không phải phần phụ lục. Trong một sản phẩm về hai người, **consent là cơ chế lõi**; làm sai thì mất cả hai người cùng lúc, và mất một chiều không quay lại được.

### 10.1 Luật «chỉ hiện tổng» sụp ở nhóm hai người

ADR-0019 §2.1 bảo vệ gu cá nhân bằng cách **chỉ cho hiện tổng cộng trên nhiều người**. Trong nhóm sáu người, luật đó hoạt động.

**Trong nhóm hai người thì tổng trừ đi phần mình ra đúng người kia.** Luật thoái hoá hoàn toàn — nó không còn che gì cả.

Nên mode này **không được** đi đường «hiện tổng của cặp» cho các hàng gu cá nhân. Đường đúng là ba cái:

1. **Dữ liệu của cặp** (Ừ/Đổi, buổi đi, chặng, check-in) là dữ liệu chung **theo cấu tạo** — dùng thoải mái.
2. **Gu cá nhân** giữ riêng, chỉ vào sổ chung khi **người đó chủ động đẩy vào**.
3. **Sổ về người kia** là **quan sát của người giữ sổ**, và chỉ nhận được bốn nguồn ở Luật 3.

### 10.2 Vạch đúng: hành vi chung so với gu cá nhân

```text
LUÔN CHUNG        là hành động của hai người
                  Ừ / Đổi · buổi đi · các chặng · check-in · lịch sử kèo

LUÔN RIÊNG        là gu của một người
                  saved_places · person_interests
                  chỉ vào sổ chung khi người đó chủ động đẩy vào

TÚI RIÊNG         người kia không thấy được
                  chuẩn bị sinh nhật, kỷ niệm, quà
```

Vạch này không phải do tôi đặt thêm: `outings`, `outing_stops`, `vote_ballots` trong một context **vốn đã** là chung; `saved_places`, `person_interests` **vốn đã** per-person và đã được gác. Mode chỉ cần **không phá** vạch có sẵn, cộng **một** khái niệm mới là túi riêng.

### 10.3 Sổ về người kia: vì sao nó hợp lý, và điều kiện để nó hợp lý

Một quyển sổ chứa dữ liệu **về** người kia mà người kia **không đọc được**. Nghe như một hồ sơ. Ba điều kiện làm nó thành một cuốn sổ tay:

1. **Nội dung là quan sát của một người**, không phải bản sao dữ liệu hệ thống. Luật 3 gác điều này và **kiểm được bằng test**.
2. **Người kia biết cuốn sổ tồn tại** — nói rõ ngay trong nghi thức gấp giấy, không ẩn trong điều khoản. Cả hai đều có một quyển; sự đối xứng chính là lời giải thích.
3. **Người kia cũng đang giữ một quyển về mình.** Không có bên nào chỉ bị ghi.

### 10.4 Đường chia tay

Đã có ở mục 3.2. Nhắc lại vì đây là chỗ dễ bỏ quên nhất khi làm: **một người mở giấy là đủ**, mảnh giấy chưa tới lúc thì **không bao giờ mở nữa**, sổ ai người ấy giữ, túi riêng giữ nguyên chủ.

### 10.5 Không có suy diễn cảm xúc, ở bất kỳ đâu

Luật 1. Nhắc lại ở mục riêng tư vì đây cũng là một luật riêng tư, không chỉ luật giọng: một suy diễn về trạng thái tình cảm của một người, lưu lại thành hàng và đưa cho người kia đọc, là thứ nặng nhất mode này có thể làm sai.

---

## 11. Cố ý KHÔNG làm

Ghi ra để lượt sau không ai lặng lẽ thêm vào.

| Không làm | Vì sao |
|---|---|
| **Điểm tương thích / «92% match»** | số bịa trong giọng của sự thật, đúng cái ADR-0017 cấm ở cột danh mục. Và với đôi năm thứ năm thì nó hơi xúc phạm |
| **AI «giúp hiểu người kia hơn» giọng cá nhân** | phản ứng sẽ là «tôi hiểu người ta hơn mày», và **họ đúng**. Phân vai đúng: **app biết cái log, họ biết nhau** |
| **Thời tiết, lịch rảnh, vị trí sống** | ba tích hợp, ba consent; và ADR-0018 cấm quyền vị trí |
| **Reveal hẹn giờ có lời hứa về quán** | danh mục không có giờ mở cửa; hẹn giờ kèm lời hứa «chỗ này đang mở» là app nói dối. Giấu **hình dạng** buổi tối thì được |
| **Geofence «tới chỗ thì mở giấy»** | không quyền vị trí. Dùng check-in F46 |
| **Thanh sức khoẻ quan hệ, chuỗi ngày, badge, bảng xếp hạng** | lấy cảm giác tội lỗi làm động lực |
| **Video kỷ niệm tự sinh, bưu thiếp AI, scrapbook** | ADR-0025 đã có đường reel; mode này chưa cần, và nó không giải bài nào ở mục 0 |
| **Chia bill mặc định trong mode đôi** | sổ nợ giữa hai người là phản cảm. Hình đúng là **chi tiêu chung tháng này**, và nó là một phép đọc |
| **Hệ hình ảnh thứ hai** | mảnh giấy gấp là **cùng** hệ với trang giấy và sổ đóng |
| **Suy ra «hai người này là đôi»** | kịch bản tệ nhất app này có thể tạo ra |

---

## 12. Thứ tự làm

Ba đợt. Mỗi đợt **tự nó có nghĩa** nếu đợt sau không bao giờ tới.

**Đã chỉnh sau mục 18.5:** lát **đầu tiên** của Đợt 1 là **cửa vào cộng sổ
một người dùng được**, vì nó bỏ được điểm ma sát chờ người kia và đo được sớm
hơn. Cơ chế quyết định dưới đây là lát **thứ hai**.

### Đợt 1 — cửa vào, rồi quyết định

Cần đúng ba khái niệm mới ở mục 9.2: **trạng thái «đôi»** (1), **kèo tuần**
(5), **nhãn Ừ/Đổi** (6). Không cần sổ, không cần mảnh giấy, không cần túi riêng.

**Và không cần thông báo.** Vì lát thông báo chưa có trên `main` (cảnh báo ở
mục 9.1), Đợt 1 thiết kế **không phụ thuộc push**: kèo tuần là **một thẻ trên
màn chính của sổ hai người**, thấy khi mở app. Đây không phải phương án tạm:
nó còn đúng với hạn mức nói ở mục 8, và nó làm Đợt 1 đo được **mà không chờ**
một lát của lane khác.

- Nghi thức gấp giấy (bật «đôi» trên một `pair`, hai chiều, có hạn).
- Kèo tự tới + gậy đổi lượt + ba nút.
- Núm độ mới ba khấc, tính từ `outing_stops` và `places`.
- Chia lượt ba chặng.
- Hạn mức: một kèo một tuần, một lần lạ một tháng.
- Vật liệu: mảnh giấy gấp, hai vết gấp, ba phần dọc.
- **Module bản tính của sổ** (mục 17.3) cộng **cổng quét đếm chỗ rẽ nhánh theo
  loại sổ**, ngay từ lát đầu. Làm sau là «sync» đã trôi: rẽ nhánh rải ra rồi thì
  gom lại đắt hơn nhiều lần.
- **Câu hỏi định tuyến của Nếp** (mục 17.4) và **trần nói toàn cục** (mục 17.6).

**Đo được ngay:** tỉ lệ Ừ không kèm Đổi; số buổi ngoài năm loại gần nhất; số tuần bấm «nghỉ».

### Đợt 2 — hiểu nhau

- Hai quyển sổ, bảy mục, ba người viết.
- Câu hỏi tuần nhắm vào mục trống nhất.
- Ôn một thẻ.
- Ba dòng dặn trước buổi đi; nhắc ngày trước năm ngày.
- Sổ lái gợi ý: hai ràng buộc cứng (không ăn được, ĐỪNG).
- Khoản bổ sung cho ADR-0019 về giọng cặp và về sổ-là-quan-sát.

### Đợt 3 — thân mật và ký ức

- Mảnh giấy + bốn hẹn mở + túi riêng.
- Thư gửi năm sau.
- Giấy cũ quay lại.
- Trang tặng, đổi sổ ngày kỷ niệm.
- Bản đồ của hai người, dựng lại trên chế độ xem Hành trình (ADR-0026), **chỉ** toạ độ địa điểm.

### Cửa vào của mỗi đợt

Mỗi lát màn hình đi qua **Impeccable pipeline** như mọi việc frontend trong repo này: craft-floor trước khi sửa, detector, reviewer **context mới** có **đọc mù bảng không nhãn trước packet**, rồi documenter. Cổng thường lệ: `npm test`, `tsc --noEmit`, `pytest services/api/tests tests`, repo guard, và bảng Maestro trên một máy ảo.

---

## 13. Đo, và tiêu chí giết viết trước

Bản vision gốc không có số nào để biết Relationship Twin là thật hay là sơ đồ. Đây là số đó.

| Số | Nghĩa | Kỳ vọng nếu thiết kế đúng |
|---|---|---|
| **Ừ không kèm Đổi** | app chọn đúng mà không cần sửa | **tăng theo tuần** |
| Buổi ngoài năm loại gần nhất | đổi mới có thật | đạt **hạn mức một lần mỗi tháng** |
| Lặp lại của kèo tuần | routine đã thành thói quen | tuần thứ tám vẫn còn mở thẻ |
| Câu hỏi tuần được trả lời | vai Người chấm có sống | quá nửa |
| Ô sổ được lấp | chương trình học có chạy | bảy mục đều có ít nhất một trang trong tám tuần |
| Thẻ ôn được bấm | nhịp hàng ngày có thật | **đo cả hai người riêng** |
| Tuần bấm «nghỉ» | app có bị coi là nhắc dai không | **không tăng** theo thời gian |

**Tiêu chí giết.** Sau **tám** lần kèo tự tới, nếu tỉ lệ «Ừ không kèm Đổi» **dưới khoảng một nửa và không tăng**, thì cái twin không thật: cắt về đúng cơ chế quyết định (bản thân nó vẫn có giá trị vì nó giải bài trách nhiệm), **đừng nuôi tiếp mô hình gu**.

**Tiêu chí giết thứ hai.** Nếu số tuần bấm «nghỉ» tăng đều, Nếp đang nói nhiều: siết hạn mức ở mục 8 trước khi thêm bất kỳ tính năng nào.

**Cảnh báo về số.** Cả bảng trên chỉ đo được trên người thật. ADR-0006 vẫn gác Giai đoạn 0, và một bộ test xanh **không** đọc thành «thiết kế này đúng».

---

## 14. Câu hỏi còn mở, cần Lead chốt

1. **Một người có được nhiều sổ *đôi* cùng lúc không?** Lead đã chốt (phiên
   12/09) rằng **dùng nhiều loại sổ cùng lúc là tự do** và đổi qua lại không
   phải vấn đề — mục 17 làm điều đó thành cấu trúc. Câu còn lại hẹp hơn: **hai
   sổ đôi** cùng lúc. Đề xuất: **một**, vì gậy đổi lượt và hạn mức đều giả
   định một; và vì hai sổ đôi cùng lúc là một tính năng không ai nên xin app
   làm hộ.
2. **Mở giấy rồi gấp lại được không?** Đề xuất: **được**, nhưng là một nghi thức mới với một mốc ngày mới, và **không** phục hồi những mảnh giấy đã vĩnh viễn không mở.
3. **Một người xoá tài khoản (ADR-0023) thì các trang sổ của người kia viết về mình xử lý sao?** Đây là câu hỏi riêng tư thật và tôi không tự quyết. Ba đường: giữ nguyên (là quan sát của người còn lại) · xoá phần Nếp ghi giùm, giữ phần người ấy tự viết · xoá hết.
4. **Người kia chưa cài app thì mode chạy tới đâu bằng `guest_links`?** Cơ hội rất rẻ để giải rào «cả hai phải cài»: gửi kèo bằng link khách, bấm vào xem và phản ứng, không cần cài. Nhưng link khách hiện **tồn tại một lần** và máy chủ chỉ giữ digest, nên cần Codex nói cái gì khả thi.
5. **Khung tuần cố định hay nhiều khung?** Đề xuất: **một khung**, thêm khung là thêm nhịp nói.
6. **«Chi tiêu chung tháng này» có vào Đợt 1 không?** Nó là một phép đọc nên rẻ, nhưng nó mở một bề mặt tiền trong mode đôi và có thể làm lệch câu chuyện.
7. **Tên hai vai.** «Người lo» / «Người chấm» là đề xuất. Đây là câu chữ sẽ đi khắp app nên Lead nên chốt sớm.

---

## 15. Từ vựng

| Từ | Nghĩa trong sản phẩm |
|---|---|
| **Sổ hai người** | trạng thái «đôi» bật trên một cuộc nhắn riêng |
| **Gấp giấy** | nghi thức vào mode, hai chiều |
| **Mở giấy** | đường ra khỏi mode |
| **Nếp** | nhân vật: nếp gấp của giấy. Ở hai người là mảnh giấy được truyền tay |
| **Mảnh giấy** | một thứ gửi cho đúng một người, có mốc mở |
| **Hẹn mở** | bây giờ / tối nay / lần tới đi cùng / một năm sau |
| **Túi riêng** | ngăn chỉ một người mở được |
| **Sổ về em, Sổ về anh** | hai quyển sổ riêng, mỗi người giữ một quyển viết về người kia |
| **Trang** | một mục trong sổ, có nhãn riêng hoặc cho người kia đọc |
| **Ôn thẻ** | mỗi ngày một trang được lật lại để xác nhận còn đúng |
| **Kèo tự tới** | buổi đi đã được chọn sẵn, gửi theo nhịp tuần |
| **Gậy** | quyền rủ của tuần này, luân phiên |
| **Người lo, Người chấm** | hai vai trong một buổi đi |
| **Lần lạ** | hạn mức một loại chưa từng thử, mỗi tháng |
| **Loại sổ** | hội bạn · đôi · người nhà. Suy từ `contexts.kind` cộng trạng thái đôi |
| **Bản tính của sổ** | một bảng khai sáu điều khác nhau giữa các loại sổ; mọi màn và mọi máy đọc nó (mục 17.3) |
| **Cửa vào** | ba chỗ tìm thấy loại sổ: «Tạo mới» · hàng `Loại sổ` trong ⚙ · một thẻ thông báo một lần (mục 18.3) |
| **Trang / Mảnh** | hai biến thể tạo hình của Nếp: `gap: "trang"` là bản vẽ cũ ở hội bạn, `gap: "manh"` là bản gấp làm tư ở sổ đôi (mục 19.5) |
| **Giữ kín** | biểu cảm mới duy nhất: mày hạ, miệng một nét khép. Nếp biết mà không nói (mục 19.4) |
| **Nửa chung** | phần chỉ mở khi người kia đồng ý: kèo, gậy, mảnh giấy. Sổ về người kia **không** thuộc nửa chung (mục 18.5) |
| **Chủ của chặng** | ai quyết chặng đó. Hội bạn chọn chủ bằng phiếu, sổ đôi luân phiên hai vai (mục 17.2) |

---

## 16. Mode này không được đụng vào hội bạn

Yêu cầu của Lead (phiên 12/09): thêm mode **không ảnh hưởng** các tính năng
đã có của hội bạn. Đây không phải một lời hứa, nên mục này liệt kê **chỗ có
thể rò**, và **cổng nào chứng minh là không rò**.

Thiết kế vốn là **cộng thêm**: không tính năng nào của hội bạn bị sửa, bị bỏ
hay bị đổi nghĩa. Nhưng bốn tầng dưới đây là **dùng chung**, và rò là rò ở đó.

### 16.1 Bảy chỗ rò, và luật chặn từng chỗ

| # | Chỗ dùng chung | Rò kiểu gì | Luật chặn |
|---|---|---|---|
| 1 | `contexts.kind` | thêm giá trị thứ ba làm **đường nhắn riêng** mất tên người đối diện ở bốn chỗ | **cấm** thêm giá trị; «đôi» là hàng phụ (mục 9.2 §1) |
| 2 | `outings` | thêm cột trạng thái đổi Timeline, Hành trình, ngân sách, check-in **của nhóm** | **bảng kèo riêng**, chỉ sinh `outings` khi chốt (mục 9.2 §5) |
| 3 | `person_interests`, `saved_places` | thêm cờ «đã chia» rồi **lọc** theo cờ đó làm lệch bảng gu cộng theo nhóm của ADR-0019 §2.1 | truy vấn cộng-gu **của nhóm không được thêm điều kiện nào**; cờ mới chỉ đọc ở đường sổ |
| 4 | `notifications.kind` (khi lát đó có) | client gặp `kind` lạ thì vẽ trống | thêm `kind` **kèm** nhánh mặc định ở client; Đợt 1 **không dùng** thông báo |
| 5 | **token màu** `packages/shared/tokens.json` | đổi **giá trị** một token đang dùng làm đổi **mọi** màn hội bạn (đúng chuyện PR #603 đã làm) | chỉ **thêm** token mới; **không sửa giá trị** token đang có |
| 6 | **thành phần dùng chung**: `ui.tsx` (`Grain`, `RudiScreen`), `ui/art/VeLop.tsx`, `ui/stickers/Sticker.tsx` | vật liệu «giấy gấp» sửa ngay trong các file này thì hội bạn đổi mặt theo | giấy gấp là **lớp mới** chỉ mount trong sổ hai người; `mauLop` và `Sticker` **không đổi vai màu** |
| 7 | **chuỗi ghim của Maestro** và **cổng XML Khám phá** (`kiem-lap-loi.mjs` đọc **chỉ** `text=`) | phần tử mới mang `text` làm cổng đỏ; đổi câu chữ cũ làm flow đỏ | node mới **không có `text`** (nhãn a11y là `content-desc`); không sửa chuỗi đã ghim |
| 8 | **tầng art `nep.ts`** | đổi bản vẽ Nếp là đổi **mọi cảnh và mọi sticker** của hội bạn | biến thể mới sau một trường `gap` mặc định `"trang"`; một ca đòi bản `"trang"` **trùng từng toạ độ** với bản hiện tại (mục 19.5) |

### 16.2 Cái gì chứng minh, chứ không phải cái gì hứa

| Chứng minh | Cổng |
|---|---|
| Không màn hội bạn nào đổi hình | **pixel diff trước/sau** các màn nhóm ở sáng và tối, đúng cách PR #603 đã làm (sáng ra **0 pixel khác**) |
| Không luật màu nào lệch | `test_contrast_floor`, `test_shared_tokens` (đòi **mỗi** khoá `color.*` có biến CSS ở cả hai scheme), `rudi-khong-hex`, `rudi-mau-chat` |
| Không hình vẽ nào lệch | `art-duong`, `art-ky-hoa`, `rudi-chat-sticker`, `test_sticker_vocabulary_matches_client` |
| Không đường bấm nào của nhóm chết | bảng Maestro mặc định + `.maestro-bs-r3/65`, `.maestro-bs-r16/91`; cổng XML Khám phá 4 cấu hình |
| Không tầng nào bị nhiễm | `pytest services/api/tests tests` ở **gốc repo** (gồm `test_import_boundary`, migration-khớp-models, DESIGN gates) |
| Không luật tiền nào bị chạm | mode này **không ghi cột tiền nào**; «chi tiêu chung» là phép đọc trên `confirmed_allocations` |

### 16.3 Ba thứ hội bạn **được lợi** từ mode này

Không phải mode sống ký sinh. Ba miếng dùng chung được làm ở đây và hội bạn
dùng lại được nguyên vẹn:

1. **Encoding độ mới** (`outing_stops` → `places.category` / bán kính / khoảng
   giá). Hội bạn cũng bị rut: «lại quán nướng đó». Cùng một truy vấn.
2. **Hạn mức một lần lạ** áp được cho nhóm, cùng cơ chế, khác nhịp.
3. **Ký ức có mốc** («bảy tháng chưa quay lại») là sự thật từ log, đúng với
   nhóm như với đôi.

### 16.4 Điều duy nhất không chứng minh được bằng test

Rằng người trong hội bạn **không thấy app đổi tính**. Mode mới thêm một nhân
vật biết nói; nếu Nếp học nói ở sổ hai người rồi bắt đầu nói trong nhóm, đó là
một thay đổi mà không cổng nào bắt. **Luật: Nếp ở hội bạn giữ đúng vai cũ, im
lặng.** Việc này chỉ người soát bắt được, nên nó phải nằm trong checklist của
`impeccable-finish-reviewer` ở mọi lát của mode.

---

## 17. Một máy, nhiều loại sổ

Lead yêu cầu (phiên 12/09): **sync kiến trúc và logic**; Nếp phải **hiểu đối
tượng của mình**, **hỏi han** rồi đưa người dùng vào **đúng loại sổ**; và nếu
muốn dùng cả hai thì **đổi qua lại thoải mái, không vấn đề gì**.

Ba yêu cầu đó được giải bằng **một** quyết định kiến trúc.

### 17.1 Không có «mode» để switch

> **Không có hai mode. Có nhiều sổ, mỗi sổ một loại.**

Một người cùng lúc ở trong: ba hội bạn, một sổ đôi, một sổ người nhà. «Đổi
mode» chính là **mở một sổ khác** — và việc đó **vốn đã tự do**, vì nó chỉ là
điều hướng, không phải đổi trạng thái.

Đây là lý do yêu cầu «dùng cả hai không vấn đề gì» **không tốn gì để làm**:

| Cái người ta sợ khi có hai mode | Ở đây |
|---|---|
| một cờ «đang ở mode nào» trên người dùng | **không có** |
| trạng thái phải migrate khi switch | **không có** |
| mất dữ liệu / mất ngữ cảnh khi đổi | **không** — log thuộc về **sổ**, không thuộc về mode |
| hai màn chính phải nuôi song song | **một** màn chính: **danh sách sổ** |
| hai bộ code | **một máy**, khác **bản tính** (17.3) |

Và nó khớp đúng nền đang có: `context_id` đã xuyên mười tám bảng, `contexts.kind`
đã là chỗ khai loại sổ. **Không thêm trục nào.**

### 17.2 Bảy bộ phận dùng chung, một bản code

| Bộ phận | Ở hội bạn | Ở sổ đôi | Hai bản code? |
|---|---|---|---|
| **Sổ và thành viên** | `contexts` + `memberships` | như thế | **một** |
| **Encoding buổi đi** | `outings` → `outing_stops` → `places` (loại, bán kính, khoảng giá) | như thế | **một** |
| **Máy độ mới** | «N buổi gần nhất thuộc loại nào» | như thế, khác N | **một** |
| **Máy hạn mức** | quota theo nhịp | như thế, khác nhịp | **một** |
| **Máy chia lượt** | quyết bằng **phiếu** | **luân phiên hai vai** + một chặng chung | **một** (xem dưới) |
| **Sự thật theo mốc** | «bảy tháng chưa quay lại» | như thế | **một** |
| **Nếp** | một nhân vật, **năm luật** ở mục 7 | như thế, khác **từ vựng** | **một** |

Hàng «máy chia lượt» là chỗ sync đẹp nhất, và nó cần một phép trừu tượng hoá
đúng. Đừng nghĩ «hội thì vote, đôi thì chia ba chặng». Nghĩ:

> **Mỗi chặng của một buổi đi có một *chủ*. Khác nhau chỉ ở *luật chọn chủ*.**

```text
Hội bạn      chủ = cả nhóm,        chọn bằng phiếu
Sổ đôi       chủ = luân phiên,     chặng 1 người này, chặng 3 người kia,
                                   chặng 2 chủ là «cả hai» (vùng giao)
Người nhà    chủ = người mời,      (sau này, cùng máy)
```

Một khái niệm «chủ của chặng» phục vụ cả ba. Bình chọn của hội bạn **không bị
sửa** — nó chỉ được đọc lại thành một luật chọn chủ trong số nhiều luật.

### 17.3 Khác biệt sống ở **một** bảng, không rải khắp code

Đây là phần «sync logic» cụ thể. Mỗi loại sổ có một **bản tính** khai đúng sáu
điều, và **mọi** màn hình cùng **mọi** máy ở 17.2 đọc bản tính đó:

```text
BẢN TÍNH CỦA SỔ  (theo contexts.kind + trạng thái đôi)

  cách quyết định      phiếu | kèo tự tới + gậy
  có vai không         không | hai vai
  nhịp hạn mức         số của từng quota
  Nếp nói được gì      danh sách việc Nếp được phép làm ở sổ này
  bộ từ vựng           câu chữ, tên nút, lời Nếp
  tiền hiện kiểu gì    chia bill | chi tiêu chung
```

**Luật kiến trúc:** ngoài module bản tính, **không file nào được rẽ nhánh theo
loại sổ.** Không `if (loaiSo === "doi")` rải trong màn hình, không `if kind ==
"pair"` mới trong service.

**Cổng chứng minh:** một test quét nguồn đếm chỗ rẽ nhánh theo loại sổ và đòi
con số bằng không ngoài module bản tính — đúng idiom đã có trong repo
(`test_import_boundary.py` chặn domain import lên trên,
`test_background_tasks_boundary.py` đòi `BackgroundTasks` chỉ xuất hiện ở một
file). Cổng này là thứ giữ cho «sync» không trôi sau ba lát.

Bảng khác biệt, để bản tính có gì thì đọc ở đây:

| | Hội bạn | Sổ đôi |
|---|---|---|
| Quyết định | phiếu | kèo tự tới, ba nút |
| Vai | không | Người lo / Người chấm |
| Tiền | chia bill, ai nợ ai | chi tiêu chung tháng này |
| Nếp | **im lặng**, giữ vai cũ | bốn việc ở mục 6.4 |
| Sổ về người kia | không có | có |
| Mảnh giấy, túi riêng | không có | có |
| Câu mở đầu | «Đi đâu cả hội?» | «Tối nay tụi mình làm gì?» |

### 17.4 Nếp hỏi han để vào đúng sổ

Luật đứng trên tất cả, và nó là cách giữ Luật 3 ở mục 7:

> **Nếp hỏi về cái *sổ*, không bao giờ hỏi về *con người*.**

Nên câu hỏi không phải «hai bạn đang yêu nhau à?» mà là «**sổ này là sổ gì?**».
Khác biệt này nhỏ trong câu chữ và rất lớn trong cảm giác.

Ba câu, đúng lúc **tạo** sổ, không phải lúc onboard người dùng:

1. **Mấy người** — **không hỏi**, suy từ thành viên.
2. **Nếu sổ có đúng hai người**, Nếp hỏi, và hỏi **cả hai**:

   ```text
   Sổ này là:

   [ Hai người bạn ]     [ Một đôi ]     [ Người nhà ]
   ```

   Với lựa chọn «một đôi», chính câu trả lời của **cả hai** là **nghi thức gấp
   giấy** ở mục 3.1. Consent không phải một bước thêm; nó **là** câu hỏi định
   tuyến.
3. **Một câu hiệu chỉnh**, chỉ cho sổ đôi: routine hiện tại («tụi mình hay ăn
   tối rồi cafe»), và câu trả lời đặt hạn mức ban đầu ở mục 4.4.

Hai điều **cấm** ở bước này:

- **Không suy ra loại sổ từ hành vi.** Không bao giờ «hai người này nhắn nhau
  nhiều nên chắc là đôi» (mục 3.3).
- **Không hỏi lại.** Một sổ hai người trả lời «hai người bạn» thì Nếp **không**
  hỏi lần hai. Đổi loại là việc người dùng chủ động vào Cài đặt của sổ.

Với sổ nhiều người, Nếp **không hỏi gì cả**: loại sổ suy ra được, và một câu
hỏi ở đó là một câu hỏi vô ích.

### 17.5 Đổi loại sổ không mất gì

| Đổi | Nghi thức | Dữ liệu |
|---|---|---|
| hai người bạn → **đôi** | gấp giấy, hai chiều | log **giữ nguyên**, chỉ đổi bản tính |
| **đôi** → hai người bạn | mở giấy (mục 3.2) | log **giữ nguyên** |

Cái gì **thuộc về sổ** (buổi đi, chặng, ký ức, chi tiêu, tin nhắn) thì **giữ**.
Cái gì **chỉ có ở sổ đôi** (sổ về người kia, mảnh giấy, túi riêng) thì **ngủ** —
**không xoá** — và **tỉnh lại** nếu hai người gấp giấy lần nữa.

Đây là lý do yêu cầu «switch thoải mái» rẻ: **log là của sổ, không của mode.**
Không có bước migrate nào, nên không có bước nào để làm sai.

### 17.6 Cái mà tự do switch làm lộ ra: Nếp sẽ ồn

Hạn mức ở mục 8 viết **theo sổ**. Một người có năm sổ thì Nếp nói **năm lần**.
Tự do switch biến thành ồn, và ồn là tiêu chí giết thứ hai ở mục 13.

**Sửa: hạn mức phải có thêm một tầng toàn cục theo *người*.**

```text
Theo sổ     nhịp ở mục 8
Theo người  một trần cho tất cả sổ cộng lại, mỗi tuần
            sổ đôi được ưu tiên trong trần đó
```

Không có tầng này thì mục 8 là một lời hứa chỉ đúng với người có một sổ.

### 17.7 Gu là của người, cách biểu hiện là của sổ

Phần cuối của việc sync, và là ý đúng nhất trong bản vision của team
(«preference phải contextual theo relationship»):

```text
Của NGƯỜI, toàn cục, riêng tư      person_interests, saved_places
Của SỔ, dùng chung trong sổ        outings, outing_stops, chấm, check-in
```

Cùng một người: ở hội bạn là nướng, bia, ồn; ở sổ đôi là yên, ngồi lâu, nói
chuyện được. **Máy gợi ý phải đọc gu *qua lăng kính của sổ*** — không đọc gu
toàn cục rồi áp cho mọi sổ. Đó là lỗi làm một app gợi ý cafe cho cả hội đi
nướng.

Và điều này **không cần bảng mới**: `outings` đã khoá theo `context_id`, nên
lịch sử đã sẵn tách theo sổ. Việc phải làm là **không** trộn hai nguồn: gu toàn
cục chỉ dùng khi một sổ **chưa có lịch sử**, và nhường chỗ ngay khi sổ có buổi
đi đầu tiên.

---

## 18. Tìm thấy, biết mình ở đâu, và nâng sổ lên

Câu hỏi của Lead (phiên 12/09): làm sao **rõ ràng** khi switch · người mới làm
sao **biết mình đang dùng loại nào** · người đang dùng hội bạn mà **có người
yêu** thì đường nào qua · và **có dễ dùng không**.

Bốn câu này lộ ra một lỗ trong mục 17: kiến trúc «không có mode» **rõ ràng**
nhưng **khó tìm thấy**. Mục 17 làm việc switch thành không tốn gì, rồi bỏ quên
việc **làm sao người ta biết là có cái để switch**. Mục này bù chỗ đó.

### 18.1 Không dán nhãn mode lên màn hình

**Cấm** một badge kiểu `MODE: HỘI BẠN` trên đầu màn. Nó tạo ra một câu hỏi mà
trước đó người dùng không có: «vậy tôi đang thiếu mode gì?». Một app có badge
mode là một app bắt người ta quản lý trạng thái của chính nó.

Trong kiến trúc ở mục 17, loại sổ **không phải trạng thái người dùng đang ở**,
nó là **nhãn của cái họ đang mở** — như mở một nhóm so với mở một tin nhắn
riêng: không ai đọc nhãn, họ **thấy**.

### 18.2 Ba dấu hiệu luôn bật, không cần dạy

| Dấu hiệu | Hội bạn | Sổ đôi |
|---|---|---|
| Tên ở header | tên nhóm | tên sổ hai người |
| **Vật liệu** | trang giấy mở | **mảnh giấy gấp**, thấy vết gấp (mục 1.7) |
| Câu Nếp mở đầu | «Đi đâu cả hội?» | «Tối nay tụi mình làm gì?» |

Đây là lý do vật liệu ở mục 1.7 **không phải trang trí**: nó **là** cái chỉ báo
loại sổ. Và cho ai muốn chắc bằng chữ: ⚙ của mọi sổ **luôn có một hàng ghi rõ
loại sổ**. Sự rõ ràng nằm ở Cài đặt, không nằm trên đầu màn.

### 18.3 Ba cửa vào, một cửa chính

**Cửa chính: nút «Tạo mới».** Người ta vào đây khi **muốn** một cái gì mới, nên
đây là chỗ dạy khái niệm mà không phải dạy: bảng chọn liệt kê **loại sổ**, kèm
một dòng nói mỗi loại làm được gì. Người mới tạo nhóm đầu tiên **đã nhìn thấy**
dòng «sổ hai người» ngay hôm đó, không popup, không tutorial, và họ sẽ quay lại
đúng chỗ này.

**Cửa hai: chính chỗ nhắn riêng.** Người đã có cuộc nhắn riêng với người ấy sẽ
**không** đi «Tạo mới» — họ đã có cuộc chat rồi. Nên ⚙ của **mọi** sổ hai người
có một hàng **luôn hiện**: `Loại sổ — Hai người bạn ›`, bấm vào ra ba lựa chọn
**ngang hàng** (`Hai người bạn` · `Một đôi` · `Người nhà`).

Ba lựa chọn ngang hàng, và một trong đó **là hiện trạng**. Nhờ vậy hàng này
hỏi được ở mọi sổ hai người **mà không hàm ý gì về ai** — đây là cách duy nhất
tìm được để mở cửa mà vẫn giữ luật cấm suy ra quan hệ ở mục 3.3. App **không
bao giờ** đoán «hai người này nhắn nhau nhiều nên chắc là đôi».

**Cửa ba: một thẻ thông báo, đúng một lần**, cho người đã tạo hết sổ của họ từ
trước khi có mode. Bỏ qua được, **không nhắc lại**, và nằm trong trần nói ở
mục 17.6.

### 18.4 Số cú bấm

| Việc | Số bấm |
|---|---|
| Người mới lập sổ đôi | Tạo mới → Sổ hai người → chọn người → một câu = **bốn** |
| Đã có nhắn riêng, nâng lên | ⚙ → Loại sổ → Một đôi = **ba** |
| Người kia đồng ý | mở thông báo → Đồng ý = **hai** |
| Dùng hằng tuần | Ừ / Đổi / nghỉ = **một** |
| Dùng hằng ngày | Đúng / Sửa = **một** |

### 18.5 Điểm ma sát duy nhất, và nó đổi thứ tự làm

Gấp giấy cần **người kia cũng bấm**. Nếu họ chưa mở app thì người khởi xướng
bị treo, và cảm giác «tôi vừa gửi một lời đề nghị về quan hệ rồi ngồi chờ» là
cảm giác tệ nhất mà luồng này có thể tạo ra.

**Luật: nửa của mình phải dùng được ngay một mình.**

> **Sổ về người kia là sổ *riêng* của mình — nó không cần người kia.** Ghi được
> ngay, ôn thẻ được ngay, trong lúc chờ.

Người kia đồng ý thì **mở thêm nửa chung**: kèo tự tới, gậy đổi lượt, mảnh
giấy. Nên tính năng **có giá trị với một người**, và người thứ hai là phần mở
rộng chứ không phải điều kiện khởi động.

**Hệ quả cho mục 12:** Đợt 1 và Đợt 2 **đảo một phần**. Hai quyển sổ (Đợt 2)
có phần **chạy được với một người**, nên lát đầu tiên nên gồm:

```text
Lát đầu    cửa vào (18.3) · nghi thức gấp giấy · SỔ VỀ NGƯỜI KIA một người
           dùng được · module bản tính (17.3) và cổng của nó
Lát sau    kèo tự tới · gậy đổi lượt · núm độ mới · hạn mức
```

Cách này còn đo được sớm hơn: **bao nhiêu người bấm vào cửa** và **bao nhiêu
người kia đồng ý** là hai số biết được trước khi xây máy gợi ý.

### 18.6 Bốn thứ không được làm ở đường vào

| Không | Vì sao |
|---|---|
| Badge mode trên màn | tạo ra câu hỏi người dùng không có (18.1) |
| Nhắc lần hai | một lời đề nghị về quan hệ bị nhắc lại là một áp lực (mục 3.1) |
| Suy ra quan hệ để mời | kịch bản tệ nhất app này có thể tạo (mục 3.3) |
| Onboarding hỏi «bạn muốn mode nào» | người mới chưa có quan hệ nào trong app thì hỏi là hỏi vô ích |

---

## 19. Nếp đổi hình theo loại sổ

Lead yêu cầu (phiên 12/09): Nếp phải **đổi UI design và tạo hình** giữa mode
hội bạn và mode đôi, và phải có **câu chuyện song song**.

Nguyên tắc chặn trước: **cùng một nhân vật, không phải hai nhân vật.** Bản vẽ
hiện tại (`src/rudi/art/nep.ts`, theo concept sheet 08/09) đã có một giải phẫu
chốt: tờ giấy hơi rộng, **một nếp gấp chéo dưới thân như hai ve áo**, **một góc
coral gấp xuống trên phải**, mắt mực dưới **một** bên mày (góc gấp che chỗ bên
mày kia), miệng cười nghiêng, chân thon, tay mitten **đang làm gì đó**. Concept
note đã **loại bỏ có chủ ý**: mắt to, má hồng, con dấu máy bay giấy, hai gạch
trên thân.

Đổi hình mà phá những thứ trên là thay nhân vật. Nên đổi **đúng bốn thứ**.

### 19.1 Câu chuyện song song

```text
Ở HỘI BẠN                          Ở SỔ HAI NGƯỜI

Nếp là một TRANG                   Nếp là một MẢNH GIẤY
nhiều người viết vào               hai người truyền tay
nó HỨNG chữ                        nó MANG chữ đi
nó giữ CHỖ cho bạn                 nó giữ ĐIỀU cho hai người
gấp một nếp chéo                   gấp làm tư, hai nếp giao nhau
góc coral gấp xuống, LỘ ra         góc coral gấp VÀO TRONG, hé một tam giác
tay đang làm một việc              tay đang ĐƯA, hoặc đang GIỮ
đứng cỡ cảnh                       nhỏ bằng con tem
nói khi được gọi                   nói đúng nhịp, và IM giữa các nhịp
```

Hai dòng giữa là nghĩa của cả mode: **cái gì quan trọng thì gấp vào trong.** Ở
hội bạn góc coral lộ ra vì nó là lời mời. Ở sổ hai người nó gấp vào vì nó là
điều được giữ.

### 19.2 Bốn thứ đổi, và không gì khác

| # | Đổi | Ở hội bạn | Ở sổ đôi | Vì sao |
|---|---|---|---|---|
| 1 | **Số nếp trên thân** | một nếp chéo (hai ve áo) | **hai nếp giao nhau** (gấp làm tư) | hai nếp này **trùng hai vết gấp của tờ giấy màn hình** ở mục 1.7: nhân vật và cái sổ mang cùng một vết gấp |
| 2 | **Góc coral** | gấp xuống trên phải, lộ một tam giác | **gấp vào trong**, chỉ hé ở giao điểm hai nếp | vẫn **đúng một** lớp coral (luật của `gu.ts`); và nó có **lý do** chứ không phải biến thể |
| 3 | **Tỉ lệ thân** | tờ hơi rộng | **vuông hơn, ngắn hơn** | «gấp làm tư» phải đọc ra được ở dáng ngoài |
| 4 | **Họ tư thế** | việc của nhóm: kéo ghế, giữ chỗ, cầm bản đồ | việc của **truyền tay** (19.3) | luật cũ: mỗi tư thế là **một việc khác nhau**, không phải cùng thân cầm vật khác |

**Không đổi, để còn là Nếp:** mắt mực hai chấm theo `nhin` · một bên mày ·
miệng cười nghiêng · tay mitten · chân thon · sàn `CHAN_NEP` · và **cấm** mắt
to, má hồng.

### 19.3 Sáu tư thế mới, mỗi cái là một cơ chế

Theo đúng luật «một tư thế là một việc để LÀM», và mỗi cái ứng đúng một cơ chế
trong doc này:

| Tư thế | Việc | Cơ chế |
|---|---|---|
| `dua-giay` | đưa một mảnh giấy sang | mảnh giấy tới (5.1) |
| `up-xuong` | úp mảnh giấy xuống, tay còn đè | chưa tới lúc mở (5.2) |
| `mo-ra` | mở một mảnh đã gấp | hẹn mở tới (5.2) |
| `gap-lai` | đang gấp, mắt xuống | túi riêng (5.5) |
| `trao-gay` | đưa một cây bút sang phía người kia | tới lượt ai rủ (4.2) |
| `lat-the` | lật một thẻ lên, nhìn vào nó | ôn một thẻ (6.5) |

Mỗi tư thế khai đủ bốn trường như `TU_THE` đang có: `nghieng` · `nhin` ·
`bieuCam` · `dang`. Hai cái dùng `dang: "ngoi"` (`gap-lai`, `lat-the`) vì Nếp ở
sổ đôi thường **ngồi trên mép giấy** chứ không đứng giữa cảnh.

### 19.4 Một biểu cảm mới, đúng một

`BIEU_CAM` đang có sáu: `binh-than` `hao-hung` `hoi` `quyet` `met` `nhuong`.
Sổ đôi cần thêm **đúng một**:

> **`giu-kin`** — mày hạ, miệng thành một nét thẳng khép.

Vì điều cảm động nhất của Nếp ở sổ hai người là nó **biết mà không nói**: túi
riêng, mảnh giấy chưa tới lúc, món quà đang chuẩn bị. Biểu cảm này làm được
bằng **mày và miệng**, đúng luật hiện tại (mắt vẫn là hai chấm theo `nhin`),
nên nó không mở cửa cho mắt to hay má hồng.

Không thêm biểu cảm nào khác. Nếu một tình huống cần «Nếp thấy thương», câu trả
lời là **không vẽ Nếp ở đó** (19.6).

### 19.5 Cách cài mà không đụng hội bạn

Dùng đúng idiom `nep.ts` đã dùng cho chân: *«`dung` là bản vẽ cũ, không đổi,
nên mọi tư thế và cảnh hiện có giữ đúng hình nó đang có.»*

```text
TuyChonNep thêm một trường:

   gap?: "trang" | "manh"        mặc định "trang"

   "trang"  = bản vẽ HIỆN TẠI, không đổi một toạ độ
   "manh"   = biến thể gấp làm tư của mục 19.2
```

**Mặc định là `"trang"`**, nên **không một cảnh, sticker hay màn nào của hội bạn
phải sửa**. Ba cỡ đọc hoá ra là hai cỡ nhân hai biến thể, dùng lại `chiTiet`
đang có:

| | `chiTiet: true` (96) | `chiTiet: false` (48) |
|---|---|---|
| `"trang"` | bản cảnh của hội bạn | bản sticker/gọn |
| `"manh"` | Nếp trong thẻ của sổ đôi | **Nếp bằng con tem**, cạnh một dòng chữ |

**Cổng chứng minh không rò** (nối vào mục 16.2): `art-duong` và các ca hình học
chạy **cho cả hai biến thể**; thêm một ca đòi `gap` mặc định là `"trang"` và
đòi mọi lớp của bản `"trang"` **trùng từng toạ độ** với bản hiện tại; luật
**đúng một lớp coral** kiểm trên cả hai; `rudi-chat-sticker` không đổi.

### 19.6 Nếp vẫn là lớp tháo được, và luật «không đứng cạnh» phải rộng ra

`nep.ts` đang ghi: *Nếp là lớp tháo được; mọi cảnh trọn nghĩa khi không có nó;
và nó **không bao giờ đứng cạnh tiền, lỗi hay xung đột**.*

Luật đó phải rộng thêm một mục cho sổ đôi:

> **Nếp không xuất hiện khi hai người đang lệch nhau.**

«Đổi» **không** phải lệch nhau: đó là một cú hoán, Nếp ở đó được. Nhưng khi
**một người Ừ và người kia Đổi**, Nếp **bước ra**: thẻ vẫn làm việc của nó,
nhân vật không đứng nhìn. Cùng lý do với luật cũ về tiền và lỗi.

Và vì Nếp là lớp tháo được, **cỡ chữ lớn có câu trả lời sạch**:

> Ở `chuLon(fontScale)`, **bỏ Nếp** khỏi thẻ, **không thu nhỏ nó**.

Chữ được chỗ, và ta không sinh ra một cỡ đọc thứ ba phải nuôi.

### 19.7 Bản tối

Theo luật đã ship (PR #603): mọi **hình vẽ** là **giấy đêm** `paper` /
`paperShade`, nền là vân vải. Với Nếp `"manh"`: thân là `paper`, **hai nếp gấp**
là `paperShade`, mực là `muc`, và **góc coral hé ra là thứ ấm duy nhất trên cả
màn tối**. Đúng câu chuyện: một mảnh giấy sáng trên bìa vải, có một góc còn ấm.

---

## 20. Những chỗ còn thiếu — đề xuất

Lead yêu cầu đề xuất **hết** những chỗ còn thiếu. Tám mục dưới đây là những chỗ
doc đã nói *cơ chế* mà chưa nói *hình dạng*, xếp theo thứ tự cần quyết.

### 20.1 Màn chính của sổ đôi

**Không phải màn mới.** Vẫn là màn chat của hai người, thêm **ba thứ ở đầu**,
đúng thứ tự ưu tiên, và **không bao giờ quá ba**:

```text
┌──────────────────────────────────┐
│  ←  (tên sổ)                ⚙    │
├──────────────────────────────────┤
│  THẺ KÈO          tuần một lần   │
│  ba chặng · một dòng lý do       │
│  [ Ừ ]  [ Đổi ]  [ Tuần này nghỉ]│
├──────────────────────────────────┤
│  THẺ ÔN           ngày một thẻ   │
│  một dòng · [ Đúng ] [ Sửa ]     │
├──────────────────────────────────┤
│  MẢNH GIẤY        khi có         │
│  «có một mảnh giấy, mở tối nay»  │
├──────────────────────────────────┤
│  ─── tin nhắn như cũ ───         │
└──────────────────────────────────┘
```

Vắng mặt có chủ ý: không feed, không nhiều thẻ gợi ý, không điểm, không badge
mode (18.1), không streak (mục 8).

### 20.2 Trạng thái trống

Bốn trạng thái, mỗi cái một tư thế Nếp và **một** câu:

| Khi nào | Nếp | Ý |
|---|---|---|
| Vừa gấp giấy, chưa có gì | `dua-giay`, `hoi` | mời khai khung tuần và routine |
| Đã gửi nửa giấy, đang chờ | `up-xuong`, `giu-kin` | và **lối vào sổ riêng dùng được ngay** (18.5) |
| Tuần này bấm nghỉ | `gap-lai`, `nhuong` | không có thẻ nào, và **Nếp im** |
| Sổ chưa có buổi đi nào | **không vẽ Nếp** | chưa có gì để nói thì không cần nhân vật |

### 20.3 Bộ từ vựng hai giọng

Bản tính của sổ (17.3) khai «bộ từ vựng». Đề xuất luật cụ thể:

| | Hội bạn | Sổ đôi |
|---|---|---|
| Gọi tập thể | «cả hội» | «hai bạn» |
| Câu mở | «Đi đâu cả hội?» | «Tối nay tụi mình làm gì?» |
| Chủ ngữ khi nhắc | nhóm | **luôn là cặp**, không bao giờ một người |
| Cấm tuyệt đối | — | mọi câu về **tâm trạng** một người (mục 7, Luật 1) |

Một dòng kiểm được: câu của Nếp ở sổ đôi **không được chứa tên riêng của một
trong hai người làm chủ ngữ của một động từ cảm xúc**. Đây là thứ quét được.

### 20.4 Nghi thức gấp giấy cần chuyển động, và phải tôn trọng Reduce Motion

Gấp giấy mà chỉ là một cú bấm thì nó là cái checkbox, không phải nghi thức. Đề
xuất: **một nếp gấp chạy qua tờ giấy** khi cả hai đã bấm.

Và nó phải đi qua đường đã có của R1/R2: `useMotion` đọc bit Reduce Motion sống,
`stackAnimation` về `none`. **Khi Reduce Motion bật, nếp gấp thành một cú chuyển
mờ** — không phải mất nghi thức, mà là nghi thức không chuyển động. Đo bằng
chuỗi `do-motion.sh` v3 đã có.

### 20.5 Sticker của sổ đôi cần ADR, không tự thêm được

`test_sticker_vocabulary_matches_client.py` **ghim bộ sticker với máy chủ**. Nên
sticker cho sổ đôi (đưa giấy, trao gậy, giữ kín) **không phải việc frontend**:
phải mở vocabulary phía máy chủ trước, tức là Codex cộng một ADR. **Đề xuất:
Đợt 1 và 2 không có sticker mới**; sáu tư thế ở 19.3 chỉ dùng **trong thẻ**,
không vào khay sticker.

### 20.6 A11y

Nếp **không mang chữ** (đúng như `KyHoa` đã làm), nhãn đi bằng `content-desc`,
nên **cổng XML Khám phá không đổi** (mục 16.1 hàng 7). Mỗi thẻ mới ở 20.1 cần
một nhãn nói **việc**, không nói hình: «thẻ kèo thứ Bảy, ba chặng» chứ không
«hình Nếp đưa giấy». Ba nút của thẻ kèo là ba đích bấm 48.

### 20.7 Hai chỗ doc vẫn chưa có hình dạng

Ghi ra để không ai tưởng là đã xong:

1. **Sổ về người kia** — bảy mục đã định nghĩa, **màn hình chưa**. Câu hỏi mở:
   bảy mục là bảy thẻ cuộn dọc, hay một danh sách gập được?
2. **Bản đồ của hai người** — dựng trên chế độ xem Hành trình (ADR-0026) nhưng
   **chưa biết mốc nào được vẽ**: mọi buổi đi, hay chỉ buổi có mảnh giấy?

### 20.8 Thứ tự đề nghị cho phần hình

```text
Cùng lát đầu (18.5)   biến thể "manh" + `giu-kin` + ba tư thế: dua-giay,
                      up-xuong, gap-lai  (đủ cho cửa vào và trạng thái chờ)
Lát thứ hai           trao-gay, lat-the, mo-ra  (đi cùng kèo và thẻ ôn)
Chưa làm              sticker mới (20.5) · bản đồ hai người (20.7)
```

---

## 21. Đây chưa phải giấy phép viết code

Doc này là **thiết kế**, và cố ý dừng trước hai cửa:

1. **Cửa của Lead.** Bảy câu hỏi ở mục 14 chưa có câu trả lời, và ba trong số đó (một sổ hay nhiều · xoá tài khoản · tên hai vai) đổi cả hình dữ liệu lẫn câu chữ.
2. **Cửa của Codex.** Mọi bảng và route ở mục 9.2 là của Codex, và mục 9.3 liệt kê **sáu** ADR phải mở hoặc sửa phạm vi. Viết màn hình trước khi có ADR là tự đặt mình vào chỗ phải bỏ.

Bước đúng tiếp theo, sau khi Lead soát doc này: chuyển sang kế hoạch triển khai cho **Đợt 1** và **chỉ** Đợt 1, kèm một bản ADR nháp cho Codex đọc.
