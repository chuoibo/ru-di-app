# Spec: Mode hai người — «Nếp truyền giấy»

Ngày: 2026-09-12 · **Bản 2**, viết lại tại chỗ, thay toàn bộ bản 1.
Trạng thái: **ĐÃ CHỐT** — Lead khép phiên 12/09. Đây là bản dùng để viết kế
hoạch triển khai lát 1. Codex nhận hướng này làm nền
cho **kế hoạch nháp lát 1** (PR #609, `6e3cd013`); ba điều khoản còn hở ở lượt
hai — C1, C2, C3 — đã hợp nhất trong bản này (mục 22.4). **Bốn cửa đều mở.** Lead chốt bảy câu phạm vi (20.1) · Codex chốt K1–K7 (11.3) ·
Lead chấp nhận ADR-0027 và khoản bổ sung ADR-0019/0021 (11.4) · Lead **khép
C1–C2** phiên 12/09 (20.4). Bước tiếp theo là **kế hoạch triển khai lát 1**.
Nguồn: tầm nhìn của Lead (phiên 12/09) · bản vision của team về Relationship
Twin · **phản biện của Codex** `docs/codex/2026-09-12/review-mode-hai-nguoi-nep-truyen-giay.md`
(PR #607, REQUEST_CHANGES; sáu mâu thuẫn C1–C6, tám điểm tranh luận D1–D8).
Sở hữu: màn hình và câu chữ là của Claude (`apps/mobile/`); mọi bảng và route
là của Codex, phải mở ADR trước (mục 11).

**Vì sao có bản 2.** Bản 1 dài 1667 dòng và tự mâu thuẫn, vì nó được **nối thêm
chương** qua bốn lượt thay vì sửa chỗ cũ. Codex chỉ đúng nguyên nhân. Bản 2
**ngắn hơn** và hợp nhất; mục 22 đối chiếu từng C và D.

Năm thay đổi lớn so với bản 1:

| # | Bản 1 | Bản 2 | Vì |
|---|---|---|---|
| 1 | «Không ai rủ ai nữa»: app chọn, không ai là tác giả | **Nghi thức lập sổ CHÍNH LÀ lời rủ đầu tiên**; người gửi luôn có thật | C1 · D1: hai lời hứa cũ không thể cùng đúng |
| 2 | Lát đầu: chín bề mặt sổ riêng | Lát đầu: **vòng mở lời → nhận lời → cùng đi → giữ một điều** | U2 · D2 · D8 |
| 3 | Ba thẻ trước chat | **Một tờ đang mở** trong không gian giấy riêng | U3 · D3 |
| 4 | Mặc định ba chặng (vì giấy gấp ba) | **Một chỗ chính, phần đi tiếp tuỳ chọn** | U4 · D4 |
| 5 | `inkFaint` trên giấy; ngưỡng nền 1.9:1; một coral mỗi màn | `inkSoft`; mép `lineStrong`; accent có phạm vi | U5 · C4 · D5 |

---

## 0. Tóm tắt điều hành

**Một lời hứa, và mọi thứ trong doc phải phục vụ nó:**

> **Một lời rủ. Một điều để nhớ.**
>
> Ở hội bạn, Nếp chừa một chỗ. Ở hai người, Nếp giữ một điều.

Rủ Đi thêm **một loại sổ**, không thêm app thứ hai. Hai người nâng cuộc nhắn
riêng thành **sổ của hai người**, và việc đó xảy ra **bằng một lời rủ có thật**,
không bằng một hộp thoại khai quan hệ.

Bốn phát hiện định hình thiết kế:

**① Cái đắt của câu «đi đâu không?» không phải nghĩ ra chỗ, mà là đứng ra rủ.**
Người rủ tự đặt mình vào chỗ có thể bị từ chối, và nhận trách nhiệm cho chỗ sắp
tới. Càng thân lâu càng ít ai rủ: không phải hết muốn đi, mà hết muốn là người
rủ.
→ **Nếp phác bản nháp, người ta vẫn là người gửi.** Mất cái giá của việc nghĩ
ra; giữ nguyên cái công của việc mở lời. Chỗ dở thì «Nếp phác, tôi gửi» là trách
nhiệm chia đôi, không phải một người chịu.

**② Người thường lo thiếu bộ nhớ và sự chủ động, không thiếu gu.** Người kia đã
nói ra hết: trong chat, bằng một chỗ đã lưu, buột miệng ba tuần trước. Người ấy
**có nghe**, chỉ là không giữ lại.

**③ Nói ra thì thành đòi hỏi.** «Em muốn đi chỗ yên tĩnh» nghe như đang phê bình
mấy chỗ đã chọn trước. Nên Nếp là **người thứ ba nói được**.

**④ Trung bình hai cái gu ra buổi tối tệ nhất.** Trộn hai vector sở thích rơi
vào thung lũng giữa hai đỉnh. Cách đúng là **chia lượt**.

**Mức độ chắc chắn.** Bốn phát hiện là **luận đề chưa có bằng chứng hành vi**;
ADR-0006 vẫn gác Giai đoạn 0. ① là giả định số một. Mục 19 viết trước cả cách đo
lẫn tiêu chí giết.

**Ba mệnh đề bản 1 nói quá, bản 2 hạ thành giả thuyết:** «cái nam thiếu» →
**người thường lo**; «đôi lâu năm đều…» → **đôi đã qua giai đoạn còn hỏi nhau đi
đâu**; «app quyết thay» → **app phác, người gửi**.

---

## 1. Câu chuyện

### 1.1 Rủ Đi là một hành động

Tên app không nói về du lịch, ăn uống hay hẹn hò. Nó là ba chữ người Việt nói
với nhau mỗi tuần: **«Đi đâu không?»** — và nói ra là đã mất gì đó rồi.

Việc của app: **giảm cái giá của câu rủ, không xoá câu rủ.** Bản 1 viết «không
ai rủ ai nữa», và đó là câu tự bắn vào chân: nó xoá đúng hành động mà cả cái tên
lẫn phần thưởng «được rủ một lần» muốn nuôi.

### 1.2 Nếp hôm nay: một nếp gấp trong cuốn sổ của cả hội

App không phải màn hình mà là **cuốn sổ chuyến đi của cả hội**: ngày là trang
giấy mở, đêm là sổ đóng trên bàn. Nếp là **một nếp gấp của giấy** — không mặt,
không mắt to, không nói thành tiếng. Nó là chỗ tờ giấy gập lại, nên vốn mang hai
tính chất: **giữ được cái gì đó bên trong**, và **mở ra được**.

Ở hội, Nếp là **một trang**: nhiều người viết vào, ai cũng đọc được; việc của nó
là hứng chữ và **chừa một chỗ**.

### 1.3 Sổ của hai người thì mỏng

Sổ của hai người mỏng: không còn sáu người viết, chỉ còn hai. Nên **Nếp tự gấp
mình nhỏ bằng lòng bàn tay**, vừa túi áo, và đổi việc:

> Ở hội, Nếp là **trang giấy**. Ở hai người, Nếp là **mảnh giấy được truyền tay**.

Hình này là vật có thật trong ký ức người Việt: **mảnh giấy gấp vuông truyền
dưới bàn**. Nó nhỏ, nó riêng, và khác tin nhắn ở chỗ **có người mang đi** và
**có lúc mở**.

### 1.4 Ba tính chất của tờ giấy gấp là ba tầng sản phẩm

Cả mode chỉ có **một vật**: giấy gấp. Không tầng thứ tư nào được mọc ra.

| Tính chất | Nghĩa trong quan hệ | Tầng |
|---|---|---|
| **gấp lại thì không ai đọc được** | có thứ chưa tới lúc; có món quà đang chuẩn bị | **riêng tư**: túi riêng, trang sổ riêng |
| **mở ra đúng lúc** | đúng thời điểm quan trọng hơn nội dung | **thân mật**: mảnh giấy, hẹn mở |
| **giữ được chữ viết tay** | thứ viết hôm nay là quà cho hai người của năm sau | **hiểu nhau**: sổ, ôn thẻ, giấy cũ quay lại |

Nếp giữ đúng ba thứ: **mảnh giấy hai người gửi nhau** · **tờ lời rủ đang mở** ·
**một túi chỉ một người mở được**.

### 1.5 Vì sao phải khác hội bạn, chứ không phải hội bạn thu nhỏ

| | Hội bạn | Hai người |
|---|---|---|
| Số ý kiến mỗi buổi | sáu | hai |
| Ra quyết định | **bình chọn** hợp lý | bình chọn giữa hai người là **thương lượng**, không phải bầu cử |
| Ai tham gia | ai cũng phần nào | **một người lo gần hết**, và điều này đã đóng cứng |
| Chỗ dở thì sao | chuyện cười | «anh chọn chỗ dở» |
| Tiền | chia bill, ai nợ ai | sổ nợ giữa hai người là **phản cảm** |
| Dữ liệu sinh ra | nhiều, nhanh | **ít**: khoảng bốn buổi một tháng |

Hàng cuối là bẫy lớn nhất của bản vision gốc: loại sổ được gọi là moat lại
**đói dữ liệu nhất**. Lời giải không phải mô hình giỏi hơn mà là **đừng suy ra,
hãy hỏi và hãy ghi**. Hàng «chỗ dở» là lý do mọi cơ chế phải **sai rẻ**.

### 1.6 Vật liệu

Tiếp nối cái đã ship, **không mở hệ hình ảnh thứ hai**:

| | Vật liệu | |
|---|---|---|
| Ngày, hội bạn | trang giấy mở, vân giấy | đã ship |
| Đêm, hội bạn | sổ đóng trên bàn: vân vải làm nền, hình vẽ là giấy đêm | đã ship (PR #603) |
| **Hai người** | **một tờ giấy gấp**: hẹp hơn, có vết gấp | **mới** |

Tờ giấy hai người có **hai vết gấp NGANG** chia **ba hàng**: **gấp làm ba như
một tờ thư cho vào bao**. Hợp đồng số ở **mục 16**.

Hai điều bản 1 nói sai, đã bỏ: «ba phần **dọc**» (cộng bề rộng ở 360dp thì không
đủ, xem 15.3), và **«hơi cũ mềm»** — không có mẫu tham chiếu thì đó là cửa để
thêm texture về sau; cần thật thì nó phải là một vân có tệp riêng, có số riêng,
đi qua đúng cổng như `vaiBia` đã đi.

### 1.7 Một câu

> **Một lời rủ. Một điều để nhớ.**

---

## 2. Vòng trải nghiệm

Trục của bản 2. **Mọi lát phải khép được vòng này**; không lát nào chỉ làm một
góc của nó.

```text
   MỞ LỜI            NHẬN LỜI           CÙNG ĐI          GIỮ MỘT ĐIỀU
      │                  │                  │                  │
 Nếp phác một       Người kia mở,      Buổi đi diễn ra    Mỗi người thêm
 tờ lời rủ.         ừ hoặc đề nghị     và được ghi        một dòng vào
 NGƯỜI GỬI bấm      sửa. Im lặng       nhận.              CHÍNH tờ giấy đó.
 gửi.               KHÔNG phải ừ.                         Tờ ấy thành ký ức.
      │                  │                  │                  │
      └──────────────────┴──────────────────┴──────────────────┘
                                 │
                    Lần sau, tờ cũ quay lại đúng lúc
```

**Một tờ giấy đi hết vòng đời**: từ lời rủ → thành kế hoạch → thành ký ức. Đây
là khả năng đặc trưng nhất của sản phẩm và là lý do mode tồn tại. Một lát không
khép được vòng này thì chưa chứng minh feature.

**Lời rủ đầu tiên giữa hai người chính là nghi thức lập sổ** (mục 14). Không có
hộp thoại khai quan hệ đứng trước.

---

## 3. Máy trạng thái của một tờ lời rủ

Khép **C1**. Đây là hợp đồng: mọi màn ở mục 15, mọi bảng ở mục 11 và mọi phép đo
ở mục 19 đọc **đúng** bảng này.

### 3.1 Trạng thái

| Trạng thái | Ai thấy | Đi tiếp |
|---|---|---|
| `nhap` — Nếp đã phác | **chỉ chủ lượt** | `da_gui` (chủ lượt bấm gửi) · `bo` · `het_han` |
| `da_gui` phiên bản `v` | cả hai | `da_xem` · `nghi_tuan` · `rut` (người gửi rút, khi **chưa ai xem và chưa có phản hồi** — ADR-0027) |
| `da_xem` | cả hai, kèm mốc xem **của người nhận** | `dong_y` · `de_nghi_sua` · `het_han` |
| `de_nghi_sua` | cả hai | `da_gui` phiên bản **v+1** |
| `dong_y` | cả hai | `chot` khi **cả hai** đồng ý **cùng một `v`** |
| `chot` | cả hai | `da_di` (**một người ghi nhận**, có `recorded_by` và mốc — không suy từ ngày hay vị trí) · `huy`. **Đóng băng**: sau `chot` không `de_nghi_sua`, không phiên bản mới |
| `da_di` | cả hai | `da_giu` khi có ít nhất một dòng được thêm |
| `nghi_tuan` · `het_han` · `rut` · `bo` · `huy` | tuỳ | trạng thái cuối của tuần đó |

### 3.2 Gửi là đồng ý của người gửi, và ai gửi bản sửa

Khép **C1(a)(b)**.

> **Một CON NGƯỜI bấm gửi một phiên bản = người đó đã đồng ý phiên bản ấy.**
> Nên `chot` cần **đồng ý của người kia** trên **cùng phiên bản**, không cần
> người gửi bấm thêm lần nữa.

**Ngoại lệ, và Codex đúng khi bắt (lượt ba):** khi **Nếp gửi hộ** (mục 3.4) thì
**không ai bấm gửi cả**, nên **không ai đã đồng ý**.

> **Tờ do Nếp gửi cần `dong_y` của CẢ HAI người mới `chot`.**

Không được đọc «Nếp đã gửi» thành «người giữ lượt đã đồng ý». Bảng ở 3.1 vì thế
có hai đường vào `chot`: **một người đồng ý** khi tờ do người kia gửi, và **cả
hai đồng ý** khi tờ do Nếp gửi.

Người gửi đổi ý thì **`rut`**, và chỉ rút được **khi chưa có phản hồi** — sau đó
thì đường đi là đề nghị một phiên bản mới, không phải rút.

**Ai gửi bản sửa:** người **đề nghị sửa** là người **gửi `v+1`**, và vai đảo lại
cho phiên bản ấy — người gửi `v` giờ là người phải đồng ý `v+1`. Nên luật trên
đúng cho mọi phiên bản, không phải chỉ phiên bản đầu.

```text
v    A gửi  →  A đã đồng ý v.   Cần B đồng ý v      →  chot
v+1  B sửa và gửi  →  B đã đồng ý v+1. Cần A đồng ý v+1  →  chot
```

### 3.3 Bảy luật không được nhập nhằng

1. **Đã phác ≠ đã gửi.** `nhap` chỉ chủ lượt thấy; máy của người kia **không
   nhận dữ liệu** của một bản nháp.
2. **Im lặng không phải đồng ý.** Không trạng thái nào tự sang `dong_y` vì hết
   giờ. Hết khung là `het_han`, không phải `chot`.
3. **Chấp thuận gắn với phiên bản.** `dong_y` trên `v` **không** chuyển sang
   `v+1`. Sửa là tạo phiên bản mới và **hiện cái gì đã đổi**.
4. **Đã nhận dữ liệu ≠ đã xem ≠ đã đồng ý.** Ba mốc khác nhau; chỉ **mốc xem của
   người nhận** được hiện cho người gửi (mục 7.5).
5. **`chot` sinh đúng một `outing`.** Gửi lại do mất mạng **không** nhân đôi;
   khoá **theo tờ giấy** — `UNIQUE(paper_id)` trên bảng nối outing. Bản 2 viết «theo
   `(kèo, phiên bản)`»; ADR-0027 K3 sửa: khoá theo cặp vẫn sinh được **hai** outing nếu
   hai phiên bản cùng chốt.
6. **Máy chủ là nguồn sự thật.** Màn hình và chuyển động chạy **sau** kết quả máy
   chủ, không chạy lạc quan rồi sửa.
7. **Đóng sổ không làm sống lại gì** (mục 7.6).

**Đường hết hạn, nghỉ, rút — đủ cho mọi trạng thái** (khép C1(d)):

| Từ | `rut` | `nghi_tuan` | `het_han` |
|---|---|---|---|
| `nhap` | — (chưa gửi thì là `bo`) | được, và **không** phác lại tuần đó | qua khung |
| `da_gui` | **được** (chưa ai xem **và** chưa có phản hồi) | được, tờ chuyển `huy` | qua khung |
| `da_xem` | **không** — đường đi là đề nghị phiên bản mới | được, tờ chuyển `huy` | qua khung |
| `de_nghi_sua` | không | được, tờ chuyển `huy` | qua khung |
| `dong_y` một phía | không | được, tờ chuyển `huy` | **qua khung là `het_han`, KHÔNG phải `chot`** |
| `chot` | không | không — đã là kế hoạch; huỷ buổi là `huy` | không |

«Nghỉ tuần» **luôn bấm được cho tới khi `chot`**, và nó tắt đề nghị của Nếp tuần
đó chứ không tắt việc hai người tự nhắn hay tự gửi giấy (mục 6.3).

### 3.4 Tác giả luôn có thật

| Ai gửi | Tờ giấy đứng tên |
|---|---|
| Chủ lượt bấm gửi | **người đó** |
| Chủ lượt không bấm trước khung, sổ có bật «cho Nếp gửi hộ» | **Nếp**, nói rõ «Nếp gửi vì chưa ai mở lời» |
| Sổ không bật | **không ai gửi**; tuần đó không có kèo |

**Cấm giả một người đã gửi khi họ chưa làm.** Và **bỏ** luật «phải sửa đúng một
thứ mới được gửi» của bản 1: nháp vừa ý thì gửi nguyên; công thật nằm ở **chọn
gửi**, không phải một thao tác diễn để nhận công trạng.

---

## 4. Hai vai và cái gậy

Không phải hai tính cách. Là **hai việc khác nhau trong cùng một buổi**, mỗi
người một việc.

### 4.1 Người lo và Người chấm

| | Người lo | Người chấm |
|---|---|---|
| Việc | quyết chỗ · lo hậu cần · **nhớ** · **để ý** · **mở lời** | **nói ra** (với Nếp) · **chấm** một chi tiết · **phán** sau buổi · **ghim** một trang vào sổ người kia |
| Thất bại hay gặp | né quyết, quên điều người kia đã nói, im ba tuần | không nói vì nói ra thành đòi hỏi |
| Được gì | công trạng của việc rủ mà không tốn công nghĩ | muốn thành tiếng mà không thành đòi hỏi; được ngạc nhiên |

Bốn trên năm việc của Người lo là **việc hiểu người kia** — đó là chỗ một trợ lý
có lý do tồn tại.

### 4.2 Gậy: lượt đầu thuộc về người ÍT mở lời

Bản 1 gán mặc định Người lo cho **người khởi xướng**, tức là người lâu nay vẫn
lo. Codex chỉ đúng: như thế là **đóng cứng đúng cái bất đối xứng mà feature định
gỡ**. Sửa:

> **Lời rủ lập sổ là lượt số 0, không tính vào vòng gậy.**
> **Người khởi xướng giữ vai Người lo. Gậy của tuần kế tiếp — lượt số 1 — thuộc
> về người kia.**

Khép **C1(c)**: bản 2 vừa nói lời rủ đầu tiên là của người khởi xướng (mục 14.1),
vừa nói gậy tuần đầu thuộc người ít mở lời — hai câu chọi nhau. Tách ra bằng lượt
số 0 thì cả hai cùng đúng: người khởi xướng mở lời **một lần** để lập sổ, rồi
**nhường lượt đầu tiên của vòng** cho người kia.

Mỗi tuần gậy đổi chủ. Chủ lượt thấy bản nháp **riêng**, trước; sửa nếu muốn;
bấm gửi. Người nhận thấy **một lời rủ từ người mình yêu**, vì đó là sự thật.

Nếu chủ lượt không gửi: không phạt, không nhắc lại, **tuần sau gậy vẫn sang
người kia**.

### 4.3 Chia lượt, không lấy trung bình

Khi một buổi có nhiều hơn một chặng, **mỗi chặng có một chủ**:

```text
chặng của Người chấm     ·     chặng vùng giao     ·     chặng của Người lo
```

Mỗi người được **đúng một thứ mình thật sự thích**, thay vì hai thứ cả hai chỉ
chịu được. Lượt bị bỏ được **trả lại buổi sau**.

**Nhưng số chặng KHÔNG bị ràng vào số nếp gấp** (mục 5.2).

---

## 5. Nhịp, hình dạng buổi đi, và độ mới

### 5.1 Khung tuần và bản nháp

Đôi khai **một khung** («tối thứ Bảy») và **routine hiện tại** («tụi mình hay ăn
tối rồi cafe»). Cả hai **do người dùng khai**, không suy ra; không cần lịch,
không cần thời tiết, **không cần quyền vị trí** (ADR-0018 giữ nguyên).

Trước khung một quãng, Nếp phác một tờ cho **chủ lượt**. Từ đó chạy mục 3.

### 5.2 Hình dạng mặc định: một chỗ chính

Bản 1 mặc định ba chặng **vì tờ giấy gấp làm ba**. Đó là bắt cuộc sống phục vụ
hình học, và đôi đã qua giai đoạn hỏi nhau đi đâu thì phần lớn buổi là «ăn gì đó
rồi về». Sửa:

> **Mặc định: một chỗ chính, phần đi tiếp là tuỳ chọn.**

Ba chặng chỉ xuất hiện khi **thời gian, ngân sách, quãng đi và ý muốn của hai
người** cho phép. **Hai nếp gấp là cấu trúc của một lá thư, không phải hạn mức
địa điểm.** Một buổi 45 phút vẫn là một tờ giấy hợp lệ.

### 5.3 Núm độ mới, ba khấc

| Khấc | Nghĩa |
|---|---|
| **Chỗ cũ** | lấy từ sổ của chính hai người |
| **Cùng kiểu, chỗ mới** | giữ hình dạng buổi, đổi quán. **Mặc định** |
| **Kiểu chưa thử** | một loại **không có trong sổ này** N buổi gần nhất |

Thứ tự ràng buộc, từ cứng tới mềm:

```text
1. điều KHÔNG thể (không ăn được, «đừng»)        ← ràng buộc cứng
2. khung giờ / ngân sách / khu vực người dùng khai
3. một việc CẢ HAI chấp nhận
4. mức mới                                        ← mềm nhất
```

**Mức mới không bao giờ được đẩy một trải nghiệm mà người kia không muốn.**

### 5.4 «Tháng này thử một điều mới?» — không phải món nợ

Bản 1 viết «còn nợ một lần lạ», trong khi mục 6.3 cấm cơ chế tội lỗi. Tự mâu
thuẫn. Sửa: một **lời mời** mỗi tháng, có **nghỉ / hẹn tuần khác**, và **không
đếm chuỗi, không đòi**.

---

## 6. Nếp

### 6.1 Năm luật

1. **Xen vào bằng sự thật, không bao giờ bằng cảm xúc.** «Ba tuần rồi» được.
   «Hai bạn có ổn không?» cấm. Không thanh sức khoẻ quan hệ, không chấm điểm.
2. **Hai giọng.** Với Người lo: **gợi ý**. Với Người chấm: **câu hỏi**. Không
   đưa nguyên văn lời người kia, không kể cho người kia biết đang chuẩn bị gì.
3. **Chỉ mang sang những gì người kia đã có thể biết** — bốn nguồn ở mục 7.3, và
   chỉ sau consent.
4. **Không biên nhận cho câu hỏi riêng** (mục 7.5).
5. **Có hạn mức nói** (mục 6.3).

### 6.2 Nếp không đứng cạnh cái gì

`nep.ts` đã ghi: Nếp là **lớp tháo được**, mọi cảnh trọn nghĩa khi không có nó,
và nó **không đứng cạnh tiền, lỗi hay xung đột**. Thêm một mục cho sổ hai người:

> **Nếp không xuất hiện khi hai người đang lệch nhau.**

«Đề nghị sửa» **không** phải lệch nhau — đó là một bước của vòng, Nếp ở đó được.
Nhưng khi một người `dong_y` còn người kia `de_nghi_sua` **trên cùng một phiên
bản**, Nếp **bước ra**: tờ giấy vẫn làm việc của nó, nhân vật không đứng nhìn.

### 6.3 Hạn mức nói

| Việc | Nhịp tối đa | Tắt được |
|---|---|---|
| Tờ lời rủ | **một lần một tuần** | có |
| Câu hỏi cho Người chấm | **một câu một tuần** | có |
| Ôn một thẻ | **một thẻ một ngày**, im trong app, **không push** | có |
| Nếp nhắc (sự thật) | **tối đa hai lần một tháng** | có |
| Nhắc ngày trong sổ | theo mốc, **trước năm ngày**, một lần | có |
| Mảnh giấy tới | theo sự kiện thật | có |

Ba luật kèm: **«Tuần này nghỉ» luôn có mặt** và bấm vào thì tuần đó Nếp im hoàn
toàn · **không streak, không badge, không «bạn chưa…»** · và bảng trên là hạn
mức **theo sổ**, nên còn cần **một trần toàn cục theo người** (mục 13.4) — thiếu
nó thì mục này chỉ đúng với người có đúng một sổ.

**«Nếp im» không có nghĩa là giấu món quà người kia tự gửi.** Nghỉ tuần tắt *đề
nghị của Nếp*, không tắt việc hai người chủ động nhắn hay gửi giấy cho nhau.

---

## 7. Consent, nguồn, vòng đời — một bảng

Khép **C2** và **C3**. Bản 1 rải luật riêng tư ra sáu mục và chúng cãi nhau; đây
là bảng duy nhất.

### 7.1 Ba vùng dữ liệu

```text
CHUNG      hành động của hai người: gửi/xem/đồng ý/sửa · buổi đi · chặng ·
           check-in · dòng thêm vào tờ giấy · lịch sử kèo
RIÊNG      gu cá nhân: person_interests · saved_places · trang sổ nhãn «riêng»
TÚI RIÊNG  người kia không thấy được: chuẩn bị quà
```

Vùng **CHUNG** là chung **theo cấu tạo** — nó là hành động của cả hai — nên nó
đi vòng qua cái bẫy ở mục 7.8 mà không phải nới luật riêng tư nào.

### 7.2 Thang consent bốn bậc — mỗi bậc là một việc riêng

Khép **C2**. Bản 2 gộp «nhận lời đi chơi» với «lập sổ» và ngầm cho Nếp đọc chat.
Codex đúng: đó là ba việc khác nhau, và bậc dưới **không** hàm ý bậc trên.

| Bậc | Là gì | Ai bấm | Cho phép điều gì |
|---|---|---|---|
| **1. Nhận lời đi chơi** | đồng ý **đúng buổi đó** | người nhận | **chỉ** buổi đó. Không tạo sổ, không bật gì |
| **2. Lập sổ hai người** | một chỗ chung **có lịch sử** | **cả hai** | lưu tờ giấy, buổi đi, ký ức của hai người |
| **3. Bật «Một đôi»** | hai vai · sổ về người ấy · túi riêng · nhịp tuần | **cả hai** | những thứ chỉ có ở sổ đôi |
| **4. Cho Nếp đọc chat chung** | Nếp được đọc chat để đề nghị trang và phác tờ | **cả hai**, **mặc định TẮT** | nguồn «chat chung» ở bảng 7.3 |

**Bậc 1 không kéo theo bậc 2.** Một người có thể đi chơi cùng mà **không** lập
sổ; khi đó buổi đi ghi như mọi buổi đi bình thường và không có tờ giấy nào được
giữ lại. Mục 14.1 nói lời rủ đầu tiên **mời** lập sổ — nó không **là** việc lập sổ.

**Bậc 4 gỡ được bất cứ lúc nào**, và gỡ thì Nếp thôi đề nghị trang mới; trang đã
ghim là của chủ sổ, giữ nguyên.

### 7.3 Nguồn → ai thấy → dùng cho gợi ý nào

| Nguồn | Ai thấy | Dùng cho gợi ý | Thu hồi khi |
|---|---|---|---|
| Chat chung của hai người | cả hai | có, **sau consent** | đóng sổ |
| Chỗ đã lưu **rồi chủ động đẩy vào sổ chung** | cả hai | có | chủ đẩy gỡ ra, hoặc đóng sổ |
| Lựa chọn trong buổi chung (chấm, check-in, đổi chặng) | cả hai | có | đóng sổ |
| Trả lời câu hỏi tuần của Nếp | người kia thấy **kết quả đã gói**, không thấy nguyên văn | có | đóng sổ |
| **Ghi tay của chính mình** trong sổ về người kia | **chỉ chủ sổ** | **chỉ cho gợi ý của chủ sổ** | chủ sổ xoá |
| `person_interests` / `saved_places` **chưa chia** | **chỉ chủ** | **chỉ seed view của CHÍNH chủ** | — |

**Hai luật, không phải một.** Luật một dòng của bản 2 chưa đủ: Codex chỉ ra một
đường rò thật — một bản `nhap` dựng từ nguồn **riêng** vẫn có thể bị **«Nếp gửi
hộ»** đẩy sang người kia (mục 3.4), và lúc đó dữ liệu riêng đi ra ngoài mà **chủ
của nó không hề duyệt**.

> **Luật A — mồi.** Gu toàn cục của một người chỉ mồi cho gợi ý **mà chính người
> đó nhìn thấy**, không bao giờ mồi cho gợi ý chung.
>
> **Luật B — đường ra.** Một bản nháp có dùng nguồn **riêng** thì **chỉ rời khỏi
> máy chủ sang người kia bằng một cú bấm gửi của chính chủ**. **«Nếp gửi hộ» chỉ
> được dựng từ nguồn CHUNG còn quyền sử dụng** (bảng 7.3, cột «ai thấy» là «cả
> hai»); không đủ nguồn chung thì **Nếp không gửi hộ**, tuần đó không có tờ.

Bản 1 cho cold start dùng gu toàn cục mà không nói của ai — đó là lỗ. Sổ chung
chưa có lịch sử thì gợi ý **thật thà là còn nghèo**, và Nếp nói thế.

### 7.4 Trước consent: sổ riêng chỉ là **ghi tay**

Bản 1 vừa đòi «người kia biết và sổ đối xứng» vừa cho dùng sổ trước khi người
kia đồng ý. Sửa:

| | Trước consent | Sau consent |
|---|---|---|
| Ghi tay vào sổ của mình | **được** | được |
| Ôn thẻ trên trang mình tự ghi | **được** | được |
| **Nếp đọc chat để đề nghị trang** | **KHÔNG** | được |
| Nếp đọc chỗ đã lưu của người kia | **KHÔNG** | chỉ cái đã chủ động chia |

Nên trong lúc chờ, phần dùng được là **sổ tay của chính mình về một người** — nó
không cần quyền nào của người kia, và đó là lý do nó hợp lệ.

### 7.5 Ba mốc khác nhau, đừng gộp

| Mốc | Hiện cho ai |
|---|---|
| **đã nhận dữ liệu** | không hiện cho ai |
| **đã xem** tờ lời rủ | hiện cho **người gửi** — vì mở lời mà không biết có tới nơi là tệ |
| **đã đồng ý / đề nghị sửa** | hiện cho cả hai, **gắn phiên bản** |
| **trả lời câu hỏi tuần của Nếp** | **không có biên nhận** — người trả lời không biết người kia đã xem chưa, có làm theo không |

Hàng cuối là chỗ giữ được khoảng mờ: người ta nói với Nếp, nên khi thứ Bảy người
kia xuất hiện với đúng chỗ yên tĩnh, **nó vẫn giống như người ấy tự nhớ**. Cái
bị ẩn là **thời điểm và cách gói**, không bao giờ là **việc Nếp có hỏi**.

### 7.6 Vòng đời: đóng sổ, và nối lại

| Vật | Khi đóng sổ | Khi nối lại |
|---|---|---|
| Tờ giấy **đã mở** | giữ, cả hai còn đọc được | giữ |
| Tờ giấy **chưa tới lúc mở** | **không bao giờ mở nữa**, với bất kỳ ai | **không hồi sinh** |
| Tờ giấy **đã tới lúc, chưa ai đọc** | **khoá luôn** — xem dưới | không hồi sinh |
| **Bản nháp** `nhap` | **huỷ** | không hồi sinh |
| Tờ **đang chờ trả lời** (`da_gui`, `da_xem`, `de_nghi_sua`, `dong_y` một phía) | **huỷ**, không chốt được nữa | không hồi sinh |
| **Sổ về người kia** | của chủ sổ, giữ nguyên, vẫn riêng | giữ |
| **Túi riêng** | giữ nguyên chủ | giữ |
| Buổi đi, chặng, ký ức | thuộc về sổ, **giữ** | giữ |
| Nhịp, hạn mức, gậy | dừng | **bắt đầu lại từ đầu** |

**Khép C3 — một luật, không có cửa sổ thời gian:** bản 2 viết tờ đã tới lúc mà
chưa ai đọc thì «mở được tới hết chu kỳ». Vô nghĩa, vì **đóng sổ chính là hết
chu kỳ**. Luật đúng và đơn giản hơn:

> **Đóng sổ là đóng. Cái gì chưa được mở thì thôi.**

Không phân biệt «chưa tới lúc» với «đã tới lúc mà chưa ai đọc» — cả hai đều khoá.
Nháp và mọi lời đề nghị đang chờ đều **huỷ**. Màn xem trước hậu quả phải **đếm và
nói rõ** có bao nhiêu tờ sắp bị khoá và bao nhiêu lời đề nghị sắp huỷ, **trước**
khi bấm xác nhận.

**Nối lại là một chu kỳ MỚI có định danh riêng**, cần **consent mới**. Nó không
khôi phục quyền đọc cũ và không khôi phục lịch gửi cũ. Một người đủ quyền đóng
sổ; trước khi xác nhận phải **xem trước hậu quả** theo đúng bảng này.

**Đổi tên hành động:** bản 1 gọi việc đóng sổ là «mở giấy ra». Codex đúng: không
được lấy chữ **mở giấy** — vốn nghĩa là đọc một lá thư — đặt tên cho việc phá
hiệu lực thư. Hành động gọi là **«Đóng sổ»**.

### 7.7 Đường companion sẵn có cũng nằm trong luật này

Codex nêu ở lượt ba, và tác giả đã **kiểm trên cây**: đây là **tình trạng sẵn
có**, không phải thứ mode này tạo ra.

- `invoke_group_companion` chỉ đòi `is_group_member`, mà **một `pair` cũng là một
  context có thành viên** — không có gì phân biệt nhóm với cuộc nhắn riêng.
- `taste_profile(actor, context_id)` có `context_id` thì trả `group_taste(context_id)`,
  và `group_taste` **cộng `person_interests` của các thành viên ACTIVE**.

Nên **hôm nay, trong một cuộc nhắn riêng hai người, companion đã cộng gu riêng
của đúng hai người** để xếp hạng gợi ý. Đó chính là chỗ luật «chỉ hiện tổng» của
ADR-0019 thoái hoá (7.8).

**Nói cho đúng mức:** đây là **lộ gián tiếp**, qua thứ tự gợi ý, **không** phải
một chỗ đọc thẳng ra «người kia thích X». Nhưng tính chất ẩn danh mà ADR-0019
trông cậy thì **không còn** ở n = 2.

> **Luật C — bao phủ đường sẵn có.** Luật A và B (7.3) áp cho **mọi** đường sinh
> gợi ý trong một context hai người, **kể cả `group_taste` và companion đang
> chạy**, không chỉ cho mã mới của mode này.

Việc sửa đường sẵn có thuộc `domain/` và `api/`, nên nó nằm trong **khoản bổ sung
ADR-0019** của Codex (11.4), không phải việc tác giả tự làm.

### 7.8 Vì sao luật «chỉ hiện tổng» của ADR-0019 không dùng được ở đây

ADR-0019 §2.1 bảo vệ gu cá nhân bằng cách chỉ cho hiện **tổng trên nhiều người**.
Trong nhóm hai người, **tổng trừ phần mình ra đúng người kia** — luật thoái hoá
hoàn toàn. Nên mode này **không đi đường tổng**; nó đi ba vùng ở 7.1 và bảng ở
7.2. Đây là khoản phải bổ sung cho ADR-0019 (mục 11.3).

---

## 8. Sự thật của sổ, không phải sự thật của đời

Khép **C5**. Đây là luật ngôn ngữ, và nó là idiom sẵn có của repo: docstring của
`places` đã viết rằng một cột rỗng làm màn hình nói «chưa có», **và đó là sự
thật**, còn một số hợp lý điền vào là **lời nói dối trong giọng của sự kiện**.

| Bản 1 nói | Bản 2 nói |
|---|---|
| «chưa từng đi» | **«chưa có trong sổ này»** |
| Nếp «không bao giờ sai» | Nếp nói **theo sổ này**, có mốc, và **sửa được** |
| ràng buộc cứng lọc được vì có một ghi chú | **ba trạng thái**: biết hợp · biết không hợp · **chưa biết** |
| «lần tới hai người đi cùng nhau» mở giấy | **bỏ hẳn lựa chọn đó** (mục 10.2) |

**Ba trạng thái, không phải hai.** Danh mục thiếu giờ mở cửa, thiếu món, thiếu
giá là chuyện thường. Một ghi chú «không ăn được tôm» **loại được** quán đã biết
là quán hải sản; nó **không** bảo đảm quán chưa biết là an toàn. Nên tờ giấy nói
được cả câu **«chỗ này mình chưa biết có hợp không»** — và câu đó là một tính
năng, không phải một thiếu sót.

**Check-in chỉ chứng minh một người đã bấm**, không chứng minh hai người ngồi
cạnh nhau (F46 là cái nút, không phải cảm biến; F47 chưa có). Mọi cơ chế phải
đứng được trên sự thật đó.

---

## 9. Hai quyển sổ

### 9.1 Hai quyển, riêng tư mặc định

> **Sổ của người này viết về người kia. Không ai đọc sổ của người kia.**

Phải là hai quyển riêng, vì trong một quyển chung **không ai viết thật** — nó sẽ
thành một bản tuyên ngôn cho nhau đọc.

Tên trong sản phẩm: **«Sổ về người ấy»**. Bản 1 dùng «Sổ về em / Sổ về anh»;
Codex đúng rằng không cần buộc một giới tính vào tên để có hai người.

### 9.2 Bảy mục, nhưng pilot chỉ hai

```text
GU          ăn gì · uống gì · KHÔNG ăn được gì · cỡ áo · màu hay mặc
ĐỪNG        sợ gì · không thích gì · chỗ nào đừng quay lại
NGÀY        sinh nhật · kỷ niệm · ngày giỗ trong nhà
NGƯỜI       tên mẹ · tên em gái · tên con mèo · tên đứa bạn thân
LÚC MỆT     mệt thì muốn gì: yên / được ăn / được ngủ / được để yên
MUỐN        thứ đã nhắc mà chưa làm
CHUYỆN CŨ   một mảnh ký ức
```

Ba mục là chỗ hay vỡ nhất và chưa app nào nghĩ tới: **«không ăn được gì»** (quên
là tai hoạ, nhớ là thương) · **«NGƯỜI»** (nhớ được tên mẹ người ta là món lãi cao
nhất trong cả cuốn sổ) · **«LÚC MỆT»** (đôi cãi nhau đúng chỗ này: một người mệt
thì muốn được để yên, người kia xông vào dỗ — viết một lần, hết một cái cãi lặp
lại nhiều năm).

**Ai nhập, ai thấy, dùng cho gợi ý nào** (Codex hỏi ở R1):

| | Ai nhập | Ai thấy | Dùng cho gợi ý nào |
|---|---|---|---|
| «không ăn được» của **chính mình** | mình | **cả hai** | **ràng buộc cứng** cho mọi tờ giấy của sổ này |
| «đừng» của **chính mình** | mình | **cả hai** | **ràng buộc cứng** cho mọi tờ giấy của sổ này |

Hai ô này **cố ý là vùng CHUNG**, không phải trang sổ riêng: một điều mình **tự
khai để người kia biết mà tránh** thì giấu đi là vô nghĩa. Người kia thấy được
nội dung, **không** sửa được — chỉ chủ sửa và gỡ. Gỡ thì ràng buộc mất ngay ở tờ
tiếp theo, không hồi tố tờ đã chốt.

Đây là ngoại lệ **duy nhất** so với bảng 7.3, và nó hợp lệ vì **chủ tự khai vào
vùng chung**, không phải Nếp mang sang.

**Pilot chỉ mang HAI mục: «không ăn được» và «ĐỪNG»** — vào **trong luồng lời
rủ**, không phải thành một màn sổ bảy mục. Lý do ở mục 19.2: không có hai mục
ấy, tờ lời rủ đầu tiên có thể đề nghị đúng chỗ người ta không ăn được, và đó là
ấn tượng đầu tệ hơn cả không có feature.

### 9.3 Ba người viết, và ai được viết khi nào

| Ai viết | Cách | Trước consent |
|---|---|---|
| **Mình tự ghi** | gõ một dòng | **được** |
| **Nếp đề nghị một trang** | từ bốn nguồn ở 7.2, bấm Ghim hoặc Bỏ | **không** |
| **Người kia ghim vào sổ mình** | tối đa **hai trang một tuần** | không |

Hàng thứ hai là câu trả lời cho phát hiện ②: không ai phải viết nhật ký để có
nhật ký. Hàng thứ ba giải bài «nói mà không thành nhắc dai» — và **có hạn nên
mới còn dễ thương**.

### 9.4 Bốn chỗ Nếp dùng sổ, không hơn

1. **Câu hỏi tuần nhắm vào mục trống nhất** — câu hỏi thôi ngẫu nhiên, thành một
   chương trình học về người kia.
2. **Ôn một thẻ mỗi ngày** — lật một trang của sổ **mình đang giữ**: «Còn đúng
   không?» → Đúng / Sửa. Nó giữ sổ không cũ, và **nhớ được là do lôi ra lại**,
   không do ghi vào.
3. **Ba dòng dặn trước buổi đi** — đang muốn gì · tránh gì · nhớ gì.
4. **Nhắc ngày, trước năm ngày**, kèm một việc làm được. Nhắc đúng hôm đó thì đã
   hết kịp chuẩn bị.

Ngoài bốn chỗ đó **sổ im lặng**.

---

## 10. Mảnh giấy

### 10.1 Mảnh giấy và tờ lời rủ là **cùng một vật**

Đây là chỗ bản 2 khác bản 1 nhiều nhất. Bản 1 để tờ lời rủ ở tầng quyết định và
mảnh giấy ở tầng thân mật, tận đợt 3 — nên tên feature hứa một vật được truyền
tay mà lát đầu giao một cuốn sổ. Sửa:

> **Tờ lời rủ là một mảnh giấy.** Sau buổi đi, mỗi người thêm một dòng vào chính
> tờ ấy, và nó thành ký ức.

Nên «mảnh giấy» không phải một feature riêng phải chờ; nó là **cùng cái vật** đã
có từ lát đầu, chỉ thêm các cách gửi khác về sau.

Đây cũng là hiện thực của **F38 «Locket Style Widget»** đang nằm trong
`product/feature_list.md` với ghi chú «Optional later».

### 10.2 Hẹn mở: ba, không phải bốn

| Hẹn mở | Dùng làm gì |
|---|---|
| **Bây giờ** | hiện ngay trên màn người kia |
| **Tối nay** | «có cái này cho em, tối mở»: cả buổi chiều có cái để chờ |
| **Một năm sau** | **thư gửi năm sau** |

**Bỏ «lần tới hai người đi cùng nhau».** Bản 1 định mở nó bằng một check-in, mà
check-in chỉ chứng minh **một người đã bấm** (mục 8). Thà bỏ còn hơn giả một lời
hứa cảm biến. Ba lựa chọn còn lại đều là **mốc thời gian thật**.

**Không có đồng hồ chạy nền.** Đã kiểm trên cây (kế hoạch 12/09): máy chủ **không có
việc nào chạy sau response** — `AfterResponse` mà ADR-0024 §2.3 mô tả **chưa từng được hiện
thực**; việc trễ duy nhất trong repo là script chạy ngoài (`story_purge.py`), và hạn dùng là
**luật đọc** (`expires_at > :now` trong mọi truy vấn). Nên «mở đúng lúc» là **điều kiện lúc đọc**: tờ giấy
có mốc `mo_tu`, **không đọc được** trước mốc, và hàng thông báo sinh ở **lần đọc
đầu tiên sau mốc**, idempotent nhờ một cột đã-báo. **Người nhận vẫn phải bấm mở**
— không có gì tự bung.

### 10.3 Thư gửi năm sau, và giấy cũ quay lại

> **Mỗi mảnh giấy gửi hôm nay là một món quà cho hai người của năm sau.**

Nó làm bề mặt Locket và bề mặt ký ức thành **cùng một vật ở hai độ tuổi**, không
phải hai tính năng phải nuôi riêng. Và Nếp **lôi ra một tờ cũ** khi tới ngày kỷ
niệm của chính tờ ấy, hoặc khi tờ lời rủ tuần này quay lại **cùng `place_id`**.
Tối đa **một lần một tháng** (mục 6.3).

### 10.4 Túi riêng

Một ngăn **chỉ một người mở được**, để dành sinh nhật, kỷ niệm, quà. **Không có
ngăn này thì không có bất ngờ nào tồn tại được.** Khớp đúng nhân vật: Nếp gấp lại
thì không ai đọc được.

---

## 11. Dữ liệu, và ADR phải mở

### 11.1 Đã có trên `main` — dùng lại

| Cần cho | Đã có |
|---|---|
| Quan hệ hai người | `contexts.kind = 'pair'` + `pair_key` unique (ADR-0021 §2.5), `memberships` |
| Buổi đi và chặng | `outings`, `outing_stops`, `outing_stop_checkins` (F46, **không** chứa vị trí) |
| Encoding độ mới | `places.category` · `destination_id` · `price_min_vnd`/`price_max_vnd` |
| Ký ức | `memories` (toạ độ **của địa điểm**), `memory_reactions`, `memory_comments` |
| Ảnh trong mảnh giấy | `uploaded_images.purpose = 'personal'` (ADR-0022 §2.1) |
| Tắt từng loại thông báo | **chỉ** `people.notify_prefs` — xem cảnh báo 11.2 |
| Chi tiêu chung | `confirmed_allocations` trong context đó (`spend_vnd` là **phần** của người) |

### 11.2 Ba cảnh báo đã kiểm trên cây, không phải phỏng đoán

1. **Bảng thông báo CHƯA có.** ADR-0024 đã **CHẤP NHẬN** nhưng `models.py` chỉ có
   `people.notify_prefs`; **không** có `notifications`, **không** có
   `notification_devices`, **không** migration nào chứa `notification` trong 36
   bản. Nên **Nếp chưa có đường nói ra ngoài app**, và lát đầu phải không cần nó.
2. **`guest_links` KHÔNG dùng lại được.** `guest_links.envelope_id` là FK
   **NOT NULL** tới `collection_envelopes`. Dùng nó cho lời mời của một đôi sẽ kéo
   chuyện hẹn hò vào miền thu tiền. Bản 1 viết sai chỗ này **hai lần**. Đường
   khách cho sổ hai người là một **capability mới phải định nghĩa**, không phải
   đồ dùng lại miễn phí.
3. **Tab «Tạo mới» hiện liệt kê hành động, không liệt kê loại sổ.** Nên mục 14
   cần **route map thật**: mục nào thêm, mục nào giữ. Không được nói «không tốn
   gì» chỉ vì cùng `context_id`.

### 11.3 Khái niệm mới — **Codex đã chốt hình** (K1–K7, PR #610 `7d38fdcd`)

| | Chốt | Ghi chú |
|---|---|---|
| **K1** | **Tờ giấy + phiên bản bất biến**; phản hồi gắn **đúng phiên bản** | phiên bản bất biến làm luật 3.2 cưỡng chế được ở tầng DB, không còn trông vào code nhớ |
| **K2** | **Bảng phụ trên `pair`**, lịch sử **chu kỳ riêng**; **khoá duy nhất theo NGƯỜI** | khoá theo người là cái giữ luật «một sổ đôi» (20.1 §1) chạy được; khoá theo `pair` thì không đủ |
| **K3** | Chốt và tạo `outing` **cùng transaction**; **unique theo TỜ GIẤY** | **sửa đề xuất của tác giả.** Tác giả đề nghị khoá theo `(tờ, phiên bản)` — vẫn **sinh được hai `outing`** nếu hai phiên bản cùng chốt. Unique theo **tờ** mới đóng được |
| **K4** | Consent theo **từng mục đích, từng người, từng chu kỳ**; máy chủ kiểm ở **mọi** đường đọc/ghi | khớp thang bốn bậc ở 7.2; «từng chu kỳ» khớp luật nối lại ở 7.6 |
| **K5** | **Máy chủ quản nguồn và kiểm lại lúc gửi**; bao phủ cả **đường companion / cộng-gu pair sẵn có** | mạnh hơn đề xuất «nhãn nguồn» của tác giả: kiểm **lại lúc gửi**, không tin nhãn dán lúc phác |
| **K6** | **Lát 1 không chờ thông báo hay push** | và **không có lịch ship nào được cam kết** để làm dependency |
| **K7** | Hai ô nằm **vùng chung của sổ**, **chỉ chủ sửa/gỡ**; **không đụng `person_interests`** | đúng đề xuất ở 9.2 |

Hai chỗ Codex **sửa** đề xuất của tác giả, ghi lại để không quay về bản cũ: **K3**
(unique theo tờ, không theo cặp tờ-phiên-bản) và **K5** (kiểm lại lúc gửi, không
dựa vào nhãn nguồn dán sẵn).

#### Danh sách khái niệm

1. **Trạng thái «đôi» trên một `pair`**: ai lập, ngày lập, ai giữ vai nào, khung
   tuần, routine đã khai, khấc núm. **Một bảng một hàng cho mỗi `pair` đã bật.**
   **CẤM thêm giá trị thứ ba vào `contexts.kind`** — đã tra bốn chỗ vỡ:
   `ContextKind = Literal["group", "pair"]`; `if summary.kind != "pair" … continue`
   (một `kind` thứ ba **rơi khỏi** đường nhắn riêng, mất tên người đối diện);
   `pair_ids = [… if context.kind == "pair"]`; hai chỗ `display_name_for("pair",…)`;
   cộng CHECK trong migration `6d2b8f4e0c53`. **Một đôi vẫn là một `pair`.**
2. **Tờ giấy** (thay cho «kèo» và «mảnh giấy» của bản 1, vì chúng là một vật):
   tác giả · context · **phiên bản** · trạng thái theo mục 3.1 · nội dung (chặng,
   hoặc chữ, hoặc `PersonPhotoUrl` của chính tác giả, hoặc `place_id`) · mốc
   `mo_tu` · mốc đã gửi/đã chốt · ai đọc được. **Phiên bản đã gửi bất biến; sau `chot`
   tờ đóng băng** (ADR-0027 K1, §3).
3. **Phản hồi trên tờ giấy**: tờ · **phiên bản** · người · `dong_y | de_nghi_sua`
   · lúc nào. Đây là **dữ liệu học của cặp** và là số đo chính ở mục 19. **Mốc đã xem
   tách khỏi phản hồi**, thành bảng riêng `pair_paper_views` (ADR-0027: «đã xem» không phải
   một phản hồi đồng ý; ADR cho phép DDL cụ thể qua migration review).
4. **Dòng thêm sau buổi đi** (phần «giữ một điều»): tờ · người · một dòng.
5. **Trang sổ**: chủ sổ · viết về ai · context · **mục** (tập đóng) · nội dung ·
   nhãn chia · ai tạo · lần ôn cuối. **Một bảng phục vụ cả hai quyển.**
6. **Túi riêng**: một nhãn trên tờ giấy nói ai đọc được. Không nên thành bảng thứ hai.
7. **Loại thông báo mới** (khi lát thông báo có): tờ giấy tới · nhắc ngày. **Ôn
   thẻ không phải thông báo.**

**`outings` KHÔNG được thêm cột trạng thái** — nó dùng chung với hội bạn
(Timeline, Hành trình ADR-0026, ngân sách, check-in). Tờ giấy được `chot` thì
**sinh** một hàng `outings`.

### 11.4 ADR phải mở hoặc sửa phạm vi

| ADR | Việc |
|---|---|
| **ADR-0027** | Codex viết. **Lead CHẤP NHẬN phiên 12/09.** Phủ K1–K7. Codex gạt dòng trạng thái trên nhánh của mình |
| **ADR-0021** | `pair` **không** tự là đôi. Khoản bổ sung nằm trong ADR-0027. **Lead CHẤP NHẬN phiên 12/09** |
| **ADR-0019** | luật «chỉ hiện tổng» **không bảo vệ được ai ở n = 2**; thay bằng ba vùng ở 7.1 **và phải bao phủ `group_taste` / companion đang chạy** (7.7). Codex đã viết khoản bổ sung. **Lead CHẤP NHẬN phiên 12/09**, sau khi được nói rõ cái giá ở 11.6 |
| **ADR-0024** | thêm `kind` thông báo; push **không mang nội dung** tờ giấy |
| **ADR-0022** | tờ giấy **không phải** story (người đọc, hạn, ý nghĩa đều khác) |
| **ADR-0018 / 0026** | **không đổi**: không quyền vị trí, bản đồ chiếu toạ độ **địa điểm** |
| **Mới, đường khách** | capability link khách cho sổ hai người (11.2 §2) |
| `feature_list.md` | **F38** ra khỏi «Optional later»; cơ chế mới lấy số từ F48 |

### 11.5 Cái giá của khoản bổ sung ADR-0019, đã nói và đã được chấp nhận

Khoản này khác mọi ADR trước của mode: các ADR kia **cho phép một thứ mới**, còn
khoản này **đổi một hành vi đang chạy**. Ghi lại để sau này không ai đọc nhầm là
nó lọt qua.

Mục 2 khoản 1 và 3 của bản bổ sung nói: với `kind='pair'`, **không dùng**
interests/saved_places/budget riêng của hai người để tạo hay xếp hạng gợi ý
chung, không cho đường cộng tổng cũ đi vòng qua catalogue/companion/bản đồ/cache;
và **xử lý chat bằng mô hình cần consent đang hiệu lực của cả hai, mặc định
tắt**, **áp cả cửa cũ lẫn cửa mới**.

**Nghĩa đen:** companion trong **mọi** cuộc nhắn riêng hai người sẽ yếu đi cho
tới khi cả hai bật consent — **kể cả những cuộc nhắn riêng không liên quan gì tới
mode đôi**. Đổi lại, `person_interests` thôi bị cộng ngầm ở n = 2.

**Lead được nói rõ cái giá này và chấp nhận** (phiên 12/09).

**Một giới hạn của bằng chứng, để không ai tưởng cả khoản đều đã được đo:** tác
giả **đã kiểm tận nơi** đường `group_taste` cộng `person_interests` (7.7). Khoản
bổ sung còn chặn cả **budget, bản đồ và cache** — ba đường ấy tác giả **chưa
tra**. Không nghi ngờ, chỉ là nói rõ phần nào có bằng chứng của tác giả.

### 11.6 Tuyệt đối không đụng

Ba luật về tiền · `phase0/` và `docs/protocol/v1/` · `db/ api/ domain/` là của
Codex · không đưa vào Git ảnh bill, số tài khoản, **tên người thật**, transcript,
export, `.env` thật. Mọi ví dụ trong doc dùng **tên vai**.

**Về tiền:** «chi tiêu chung» là mặc định hợp sản phẩm cho một đôi, nhưng **không
được làm người đã có nghĩa vụ mất đường đọc sổ cái**. Giữ cửa vào lịch sử, nói rõ
tổng tháng tính theo ngày nào và gồm phần nào, và **không gắn Nếp vào màn tiền**
(luật cũ của `nep.ts`).

---

## 12. Mode này không được đụng vào hội bạn

Thiết kế là **cộng thêm**: không tính năng nào của hội bạn bị sửa, bỏ hay đổi
nghĩa. Nhưng tám tầng dưới là **dùng chung**.

| # | Chỗ dùng chung | Rò kiểu gì | Luật chặn |
|---|---|---|---|
| 1 | `contexts.kind` | giá trị thứ ba làm đường nhắn riêng mất tên người đối diện ở bốn chỗ | cấm; «đôi» là hàng phụ (11.3 §1) |
| 2 | `outings` | cột trạng thái mới đổi Timeline, Hành trình, ngân sách, check-in **của nhóm** | tờ giấy là bảng riêng, chỉ **sinh** `outings` khi chốt |
| 3 | `person_interests`, `saved_places` | cờ «đã chia» rồi **lọc** theo nó làm lệch bảng gu cộng theo nhóm | truy vấn cộng-gu **của nhóm không thêm điều kiện nào** |
| 4 | `notifications.kind` | client gặp `kind` lạ thì vẽ trống | thêm `kind` **kèm** nhánh mặc định; lát đầu **không dùng** thông báo |
| 5 | **token màu** | đổi **giá trị** token đang dùng là đổi **mọi** màn hội bạn (PR #603 là tiền lệ) | chỉ **thêm**, không sửa giá trị |
| 6 | `ui.tsx`, `VeLop.tsx`, `Sticker.tsx` | sửa vật liệu ngay trong các file này thì hội bạn đổi mặt theo | giấy gấp là **lớp mới**; `mauLop` và `Sticker` không đổi vai màu |
| 7 | **chuỗi Maestro đã ghim** và **cổng XML Khám phá** | phần tử mới mang `text` làm cổng đỏ | xem 12.1 |
| 8 | **tầng art `nep.ts`** | đổi bản vẽ Nếp là đổi **mọi cảnh và sticker** của hội bạn | biến thể sau một trường `gap` mặc định `"trang"` (mục 17) |

### 12.1 Sửa luật «node mới không có `text`»

Bản 1 nói phần tử mới **không** mang `text` để giữ cổng XML. Codex đúng: như thế
là **giấu thông tin khỏi cây accessibility**. Sửa cho đúng phạm vi:

> Luật «không `text`» chỉ áp cho **art trang trí** (hình Nếp, vết gấp, nét ký hoạ).
> **Nhãn trạng thái và nhãn chức năng phải đọc được** — «Mở tối nay, 20:00»,
> «Đã gửi, chờ trả lời», «Đề nghị sửa».
> Khi feature hợp lệ làm đổi cây, **cập nhật cổng XML đúng phạm vi**, không né nó.

### 12.2 Cái gì chứng minh

| Chứng minh | Cổng |
|---|---|
| Không màn hội bạn nào đổi hình | **pixel diff trước/sau** các màn nhóm, sáng và tối, đúng cách PR #603 (sáng ra **0 pixel khác**) |
| Không luật màu nào lệch | `test_contrast_floor`, `test_shared_tokens`, `rudi-khong-hex`, `rudi-mau-chat` |
| Không hình vẽ nào lệch | `art-duong`, `art-ky-hoa`, `rudi-chat-sticker`, `test_sticker_vocabulary_matches_client` |
| Không đường bấm nào chết | bảng Maestro mặc định + `.maestro-bs-r3/65`, `.maestro-bs-r16/91`; cổng XML 4 cấu hình |
| Không tầng nào nhiễm | `pytest services/api/tests tests` ở **gốc repo** |
| Không luật tiền bị chạm | mode **không ghi cột tiền nào** |

### 12.3 Ba thứ hội bạn được lợi

Encoding độ mới (hội cũng bị rut: «lại quán nướng đó») · hạn mức «thử một điều
mới» · các câu sự thật theo mốc. Cùng truy vấn, khác nhịp.

### 12.4 Điều không cổng nào bắt được

Nếu Nếp học nói ở sổ hai người rồi bắt đầu nói trong nhóm, đó là đổi tính app mà
test không thấy. **Luật: Nếp ở hội bạn giữ đúng vai cũ, im lặng.** Phải nằm trong
checklist của `impeccable-finish-reviewer` ở mọi lát.

---

## 13. Một máy, nhiều loại sổ

### 13.1 Không có «mode» để switch

> **Không có hai mode. Có nhiều sổ, mỗi sổ một loại.**

Một người cùng lúc ở trong ba hội bạn, một sổ đôi, một sổ người nhà. «Đổi mode»
chính là **mở một sổ khác**, và việc đó vốn đã tự do: không cờ trên người dùng,
không trạng thái phải migrate, **log thuộc về sổ chứ không thuộc về mode**.

**Sửa một khẳng định quá tay của bản 1:** bản 1 viết «màn chính là danh sách sổ».
Không đúng với shell hiện tại — app có **bốn tab** và mở lên ở **Khám phá**; danh
sách hội thoại nằm ở tab **Tin nhắn**. Bản 2 nói đúng phạm vi: **danh sách sổ đã
có sẵn ở tab Tin nhắn**, mode không thêm tab, và mục 14 phải kèm **route map
thật** chứ không tuyên bố «không tốn gì» chỉ vì cùng `context_id`.

### 13.2 Bảy bộ phận dùng chung, một bản code

| Bộ phận | Hội bạn | Sổ đôi | Hai bản? |
|---|---|---|---|
| Sổ và thành viên | `contexts` + `memberships` | như thế | **một** |
| Encoding buổi đi | `outings` → `outing_stops` → `places` | như thế | **một** |
| Máy độ mới | N buổi gần nhất thuộc loại nào | như thế, khác N | **một** |
| Máy hạn mức | quota theo nhịp | như thế, khác nhịp | **một** |
| **Máy chia lượt** | chọn chủ bằng **phiếu** | **luân phiên hai vai** + chặng vùng giao | **một** |
| Sự thật theo mốc | «bảy tháng chưa quay lại» | như thế | **một** |
| Nếp | một nhân vật, năm luật | như thế, khác **từ vựng** | **một** |

Phép trừu tượng của hàng năm: **mỗi chặng có một *chủ*; khác nhau chỉ ở *luật
chọn chủ*.** Bình chọn của hội bạn **không bị sửa** — nó được đọc lại thành một
luật trong số nhiều luật.

### 13.3 Khác biệt sống ở một module — nhưng nó là **presentation policy**

```text
BẢN TÍNH CỦA SỔ (theo contexts.kind + trạng thái đôi)
  cách quyết định · có vai không · nhịp hạn mức ·
  Nếp nói được gì · bộ từ vựng · tiền hiện kiểu gì
```

Ngoài module này, không file nào rẽ nhánh theo loại sổ. Một test quét đếm giữ
điều đó.

**Sửa một khẳng định quá tay của bản 1:** cổng đếm ấy **không chứng minh quyền
truy cập**. Nó gác **khả năng và cách trình bày**. **Phân quyền vẫn phải là kiểm
tra phía máy chủ ở biên đọc/ghi**, và **không được đẩy kiểm consent vào cấu hình
UI** để qua một phép đếm chuỗi.

### 13.4 Hai thứ tự do switch làm lộ ra

1. **Nếp sẽ ồn.** Hạn mức ở 6.3 là **theo sổ**; năm sổ là năm lần nói. Cần thêm
   **trần toàn cục theo người**, sổ đôi được ưu tiên trong trần đó.
2. **Gu là của người, cách biểu hiện là của sổ.** Cùng một người: ở hội là nướng,
   bia, ồn; ở sổ đôi là yên, ngồi lâu. Máy gợi ý đọc gu **qua lăng kính của sổ**
   — và theo luật ở 7.2, gu toàn cục **chỉ mồi cho gợi ý mà chính chủ nhìn thấy**.

---

## 14. Đường vào

### 14.1 Nghi thức lập sổ CHÍNH LÀ lời rủ đầu tiên

Không có hộp thoại khai quan hệ đứng trước. Trong một cuộc nhắn riêng, có một
đường **«Rủ đi chơi»**: Nếp phác một tờ, người ấy gửi. **Tờ giấy ấy tạo ra sổ.**

| Giải được | Vì sao |
|---|---|
| **Tác giả có thật** (C1 · D1) | người gửi là người bấm gửi |
| **Lát đầu trả đúng lời hứa** (U2 · D8) | vòng mở lời → nhận lời → cùng đi → giữ một điều khép ngay |
| **Consent nhẹ hẳn** | không ai phải khai quan hệ để dùng |
| **Mất trạng thái «chờ mà không có gì làm»** | đang chờ trả lời một lời rủ cụ thể, không chờ một câu hỏi về quan hệ |

### 14.2 «Một đôi» là một hàng trong Cài đặt, không phải cửa vào

Sổ hai người mặc định là **sổ hai người bạn**. Trong ⚙ có một hàng
`Loại sổ` — ba lựa chọn **ngang hàng**: `Hai người bạn` · `Một đôi` · `Người nhà`.

Chọn «Một đôi» **chỉ đổi việc Nếp làm** (bản tính ở 13.3): thêm hai vai, thêm sổ
về người ấy, thêm túi riêng. Nó **cần cả hai đồng ý**, vì nó bật những thứ đọc
dữ liệu của cả hai.

**«Người nhà» chưa có hành vi.** Nên **hoãn nó khỏi bảng lựa chọn đầu tiên** cho
tới khi có hành vi thật; một lựa chọn ngang hàng mà bấm vào không đổi gì là lời
hứa suông.

### 14.3 Ba dấu hiệu, và không dán nhãn mode

**Cấm** badge kiểu `MODE: HỘI BẠN`: nó tạo ra một câu hỏi người dùng không có.
Loại sổ **không phải trạng thái người dùng đang ở**, nó là **nhãn của cái họ đang
mở**:

| Dấu hiệu | Hội bạn | Sổ đôi |
|---|---|---|
| Tên ở header | tên nhóm | tên sổ |
| **Vật liệu** | trang giấy mở | **tờ giấy gấp**, thấy vết gấp |
| Câu Nếp mở đầu | «Đi đâu cả hội?» | «Tối nay tụi mình làm gì?» |

Và ⚙ **luôn ghi rõ loại sổ bằng chữ** cho ai muốn chắc.

### 14.4 Cấm ở đường vào

Suy ra quan hệ từ hành vi · nhắc lời mời lần hai · onboarding hỏi «bạn muốn mode
nào» · badge mode.

---

## 15. Bề mặt

### 15.1 Một tờ đang mở, không phải ba thẻ

Bản 1 xếp ba thẻ trước chat. Codex đúng ở hai chỗ: mở cuộc chat để nói với người
kia mà gặp **một bảng việc**; và bản 1 **không nói** «ở đầu chat» là vùng ghim
ngoài list hay phần cuộn — hai cách cho hành vi hoàn toàn khác trên một
`FlatList inverted` có composer bám bàn phím. Sửa:

> Ngay dưới tên sổ là **một hàng «Tờ giấy của hai mình»** dẫn vào **không gian
> giấy** của đúng context. Chat giữ nhịp trò chuyện.
> Trong không gian ấy, **một tờ mở theo việc đang diễn ra**; các vật khác là
> **hàng đóng có nhãn ngắn**.

**Thứ tự ưu tiên theo tình huống**, không phải thứ tự cứng:

```text
1. việc cần quyết TRƯỚC buổi đi
2. giấy người kia vừa gửi, hoặc đã tới lúc mở
3. ký ức
4. ôn riêng  ← CHỈ khi người dùng mở sổ riêng, không chen vào nội dung chung
```

Bản 1 dùng thứ tự cứng `kèo > ôn > mảnh giấy`, tức là **luôn đặt món quà dưới
câu kiểm tra dữ liệu**. Bỏ.

**Cần frame native để chốt** giữa phương án này và phương án ít đổi điều hướng
hơn (một khe trong chat, thu gọn mặc định). Đây là **cổng bên trong lát đầu**,
không phải điều kiện trước khi lập kế hoạch — xem mục 22, R2.

### 15.2 Bảng kê bề mặt

| # | Bề mặt | Mới hay có rồi | Lát | Khoảnh khắc đẹp |
|---|---|---|---|---|
| 1 | Đường **«Rủ đi chơi»** trong nhắn riêng | thêm vào màn có rồi | 1 | lời rủ đầu tiên tạo ra cuốn sổ |
| 2 | **Tờ lời rủ** ở `nhap` (chỉ chủ lượt) | mới | 1 | đã có sẵn chữ, chỉ còn việc gửi |
| 3 | **Tờ lời rủ** đã gửi, phía người nhận | mới | 1 | một lời rủ **từ người mình yêu** |
| 4 | **Đề nghị sửa** và phiên bản v+1 | mới | 1 | thấy rõ cái gì đã đổi |
| 5 | **Đã chốt** | mới | 1 | tờ giấy khép lại, thành kế hoạch |
| 6 | **Giữ một điều**: thêm một dòng sau buổi | mới | 1 | tờ lời rủ thành ký ức |
| 7 | **Hàng «Tờ giấy của hai mình»** dưới header chat | thêm vào màn có rồi | 1 | một hàng, không phải một bảng việc |
| 8 | **Không gian giấy**: một tờ mở, các tờ khác đóng | mới | 1 | mở một tờ, không phải cuộn một feed |
| 9 | **Hai ô ràng buộc**: không ăn được · đừng | mới | 1 | hai ô cứu tờ lời rủ đầu tiên |
| 10 | **⚙ Loại sổ** ba lựa chọn | thêm vào màn có rồi | 1 | ba lựa chọn ngang hàng |
| 11 | **Cài đặt sổ đôi**: vai, khung tuần, núm độ mới | mới | 2 | núm ba khấc, chữ thật |
| 12 | **Sổ về người ấy**: bảy mục | mới | 2 | bảy mục trống là **bảy câu hỏi** |
| 13 | **Soạn một trang** · **ôn một thẻ** | mới | 2 | một câu về người mình yêu, hai nút |
| 14 | **Câu hỏi tuần** · **ba dòng dặn** | mới | 2 | ba dòng đọc mười giây |
| 15 | **Soạn tờ giấy** + hẹn mở · **tờ chưa tới lúc** · **túi riêng** | mới | 3 | «Mở tối nay, 20:00» |
| 16 | **Trang tặng** ngày kỷ niệm | mới | 3 | một năm để ý, trao lại thành một tờ |
| 17 | **Đóng sổ**, kèm xem trước hậu quả | mới | 1 | nói thật cái gì mất, cái gì giữ |
| 18 | **Rút lời rủ** · **hết hạn** · **nghỉ tuần** | mới | 1 | rút được, không phải xin lỗi |
| 19 | **Lỗi gửi / lỗi lưu / không phác được** | mới | 1 | nói rõ đang hỏng ở đâu, thử lại được |

Bản 1 thiếu hẳn ba hàng cuối. Không bắt buộc mỗi trạng thái thành một màn, nhưng
**phải có nơi xử lý**.

### 15.3 Giải phẫu tờ lời rủ, tính ở 360dp

```text
360  bề rộng máy   −32 lề (space.md hai bên)   = 328 thẻ
328  thẻ           −32 lòng thẻ                = 296 nội dung
```

| Thành phần | Số |
|---|---|
| Bo tờ | `radius.small` — bo của mọi tờ giấy trong app; **góc trên phải vuông** khi tờ mang nếp gấp coral. Bản 1 ghi `radius.base` 20: đọc mù 12/09 (Phase 1) đọc bo 20 đều bốn góc thành **thẻ**, không thành tờ |
| **Mép** | `lineStrong`, hairline (xem 16.2: `line` hụt 3:1 trên cả hai nền) |
| Giờ | `type.label` **14**, `tnum` |
| Việc | `type.body` **17** |
| Vết gấp | hairline `paperShade`, chạy **mép tới mép** của tờ (hết 328, không dừng ở lòng 296 — dừng ở lòng đọc thành divider của bảng), `space.sm` **10** trên dưới |
| Dòng lý do | `type.label` **14**, màu **`inkSoft`**, tối đa hai dòng; đứng **dưới tờ, ngoài tờ** — nằm trong hàng ba làm ba hàng lệch nhau và nếp gấp mất nghĩa (đọc mù 12/09) |
| Nút | cao **48**, `radius.control` **14** |

**Một quyết định rơi ra từ số:** ba nút ngang **không vừa**. `(296 − 20)/3 = 92`
mỗi nút, mà «Tuần này nghỉ» ở label 14 cần khoảng **100** cộng lòng nút. Nên
**hai nút chính nằm ngang (143 mỗi cái), «Tuần này nghỉ» là một dòng chữ bấm được
bên dưới**, vẫn cao 48. Đúng bài học TopBar 360dp: **cộng bề rộng trước khi vẽ**.

Ở `chuLon` (≥ **1.28**): hai nút **xếp dọc** hết 296; **bỏ Nếp** khỏi thẻ (Nếp là
lớp tháo được); vết gấp **giữ** vì nó là nghĩa.

**Tờ chưa tới lúc mở** mang **chữ**: «Mở tối nay, 20:00». Không chỉ dựa vào một
tam giác coral — SC 1.4.1 đòi thông tin không truyền **chỉ** bằng màu.

---

## 16. Hợp đồng màu và vật liệu

Bản 1 đặt ngưỡng rồi mới tính, nên **ba** ngưỡng không đạt. Bản 2 tính trước.
Số dưới đây tính trên token cùng revision, công thức sRGB của WCAG 2.2.

### 16.1 Cặp màu

| Cặp | Sáng | Tối | Ngưỡng | |
|---|---:|---:|---:|---|
| `ink` trên `paper` | 15.7924 | 10.6733 | 4.5 | ✓ |
| **`inkSoft`** trên `paper` (dòng lý do) | 7.4906 | 6.8596 | 4.5 | ✓ |
| ~~`inkFaint` trên `paper`~~ | 5.1306 | **4.3728** | 4.5 | **✗ — bỏ** |
| `paperShade` trên `paper` (vết gấp) | 1.3241 | 1.3966 | 1.25 | ✓ |
| ~~`line` trên `paper`~~ (mặt nâng cũ) | 1.3241 | **1.1146** | 1.25 | **✗ — bỏ** |
| `accent` trên `paper` | 5.5296 | 4.1233 | 3.0 | ✓ |

Hai hàng gạch là hai lỗi của bản 1. Hàng `inkFaint` do Codex tìm; hàng `line` do
tác giả tự tìm khi tính lại — nó giết chính luật «mặt nâng của nếp» ở bản 1.

### 16.2 Mép giấy, và vì sao ngưỡng 1.9 bị bỏ

| Mép trên nền | Tỉ số | ≥ 3.0 |
|---|---:|---|
| `line` trên nền sáng | 1.1971 | ✗ |
| **`lineStrong`** trên nền sáng | 4.0913 | ✓ |
| `line` trên nền tối **đo được** `#1c1f36` | 1.4970 | ✗ |
| **`lineStrong`** trên nền tối đo được | 4.3414 | ✓ |

**Bỏ ngưỡng «`paper` trên nền ≥ 1.9:1» của bản 1.** Nó là số mỹ thuật tự đặt,
không phải WCAG, và **không mặt phẳng nào đạt** — kể cả tính trên token
(`paper`/`ground` = 1.4472; trên nền đo được = 1.3431). Thay bằng điều **đáng
hỏi và đạt được**: **mép đọc được ≥ 3:1**, tức **mép là `lineStrong`**.

Bản 1 còn **trích nhầm**: số `1.06:1` là của `card` trong báo cáo PR #603, còn
thẻ mới dùng `paper`. Đã sửa.

*(Ghi chú phạm vi: `line` dưới 3:1 trên nền là tình trạng sẵn có của thẻ hiện tại,
không phải lỗi mode này tạo ra, và bản 2 **không** đề nghị đổi nó.)*

### 16.3 Vết gấp: một luật, hai scheme như nhau

Bản 1 định cho bản tối một «mặt nâng» bằng `line`. Số bác bỏ: `line`/`paper` tối
chỉ **1.1146**. Và bản sáng thì `paper` là trắng nên **không token nào sáng hơn**.
Kết luận đơn giản hơn bản 1:

> **Vết gấp là một nét `paperShade`, giống nhau ở cả hai scheme.**
> Nó đọc ra là nếp gấp nhờ **ba hàng đều nhau** cộng **mép `lineStrong`**, không
> nhờ một mẹo tô bóng.

### 16.4 Accent có phạm vi

Bản 1 đòi «đúng một coral trên toàn màn». Không khả thi khi màn còn bong bóng
chat, sticker và ảnh người dùng gửi. Sửa:

> **Một điểm nhấn hành động dẫn**, tính **trong phần art do mình vẽ** ở vùng
> thiết kế mới. **Không đếm nội dung người dùng.**

**Hai cổng khác nhau, vì một cái không thay được cái kia** (Codex đúng ở R4):

| Hỏi gì | Đo bằng |
|---|---|
| Có đúng một **lớp coral** trong art mình vẽ không? | **đếm ở nguồn**, như `art-duong` vẫn đếm lớp. Không đếm pixel |
| Trên **cả khung**, có **một hành động dẫn rõ** không? | **đọc mù ở cỡ thật**: người chưa đọc doc nhìn khung đầy đủ (có bong bóng chat, sticker, ảnh người dùng) và trả lời **«việc cần làm bây giờ là gì?»** — một câu trả lời, không phải hai |

Phép đếm ở nguồn **không** chứng minh vế thứ hai, và không được dùng thay.

### 16.5 Đo vật liệu: nói rõ phép đo đo cái gì

Bản 1 định dùng «stddev dải 3px» làm bằng chứng vết gấp. Codex đúng: stddev chỉ
đo **biến thiên pixel**; chữ, nhiễu hay lệch căn cũng làm nó tăng. Muốn dùng làm
guard thì phải khai đủ:

```text
ROI            toạ độ vùng, lấy từ bounds trong XML, không phải đoán
mật độ pixel   ghi rõ dpi của lần chụp
mask chữ       loại mọi node có text khỏi vùng đo
nền so sánh    một dải phẳng CÙNG thẻ, cùng kích thước
ý nghĩa        «có một biến thiên theo hàng ở đúng chỗ nếp gấp»
```

Và **số này là guard kỹ thuật, không phải giấy chứng nhận đẹp.** Phần nghĩa hình
chốt bằng **đọc mù ở cỡ thật** với người chưa đọc doc, như ba lượt UI trước đã làm.

---

## 17. Nếp đổi hình theo loại sổ

**Cùng một nhân vật, không phải hai.** Giải phẫu trong `nep.ts` đã chốt theo
concept sheet 08/09, và concept note **đã loại bỏ có chủ ý**: mắt to, má hồng,
con dấu máy bay giấy. Chỉ đổi **bốn thứ**.

### 17.1 Câu chuyện song song

```text
Ở HỘI BẠN                          Ở SỔ HAI NGƯỜI
Nếp là một TRANG                   Nếp là một MẢNH GIẤY
nhiều người viết vào               hai người truyền tay
nó HỨNG chữ                        nó MANG chữ đi
nó chừa một CHỖ                    nó giữ một ĐIỀU
gấp một nếp chéo                   gấp làm tư, hai nếp giao nhau
góc coral gấp xuống, LỘ ra         góc coral gấp VÀO TRONG
tay đang làm một việc              tay đang ĐƯA, hoặc đang GIỮ
đứng cỡ cảnh                       nhỏ bằng con tem
nói khi được gọi                   nói đúng nhịp, và IM giữa các nhịp
```

Hai dòng giữa là nghĩa của cả mode: **cái gì quan trọng thì gấp vào trong.** Ở
hội bạn góc coral lộ ra vì nó là **lời mời**; ở sổ hai người nó gấp vào vì nó là
**điều được giữ**.

### 17.2 Bốn thứ đổi

| # | Đổi | Hội bạn | Sổ đôi | Vì |
|---|---|---|---|---|
| 1 | Số nếp trên thân | một nếp chéo | **hai nếp giao nhau** | **cùng ngôn ngữ gấp, khác mục đích**: tờ giấy gấp làm **ba, ngang**, để **gửi đi**; Nếp gấp làm **tư** để **nằm trong túi** |
| 2 | Góc coral | gấp xuống, lộ ra | **gấp vào trong** | vẫn **đúng một** lớp coral, và nó có **lý do** |
| 3 | Tỉ lệ thân | tờ hơi rộng | **vuông hơn, ngắn hơn** | «gấp làm tư» phải đọc ra ở dáng ngoài |
| 4 | Họ tư thế | việc của nhóm | việc của **truyền tay** | mỗi tư thế là **một việc**, không phải cùng thân cầm vật khác |

**Không đổi, để còn là Nếp:** mắt mực hai chấm theo `nhin` · một bên mày · miệng
cười nghiêng · tay mitten · chân thon · sàn `CHAN_NEP`. **Cấm** mắt to, má hồng.

### 17.3 Sáu tư thế mới, một biểu cảm mới

| Tư thế | Việc | Cơ chế |
|---|---|---|
| `dua-giay` | đưa một tờ sang | tờ lời rủ gửi đi |
| `up-xuong` | úp tờ xuống, tay còn đè | chưa tới lúc mở |
| `mo-ra` | mở một tờ đã gấp | đã tới lúc |
| `gap-lai` | đang gấp, mắt xuống | túi riêng |
| `trao-gay` | đưa một cây bút sang | tới lượt ai mở lời |
| `lat-the` | lật một thẻ, nhìn vào nó | ôn một thẻ |

Biểu cảm mới **đúng một**: **`giu-kin`** — mày hạ, miệng một nét thẳng khép. Vì
điều cảm động nhất của Nếp ở sổ hai người là nó **biết mà không nói**. Làm được
bằng **mày và miệng**, đúng luật hiện tại, nên không mở cửa cho mắt to. Tình
huống nào cần «Nếp thấy thương» thì câu trả lời là **không vẽ Nếp ở đó**.

### 17.4 Cài mà không đụng hội bạn

Dùng đúng idiom `nep.ts` đã dùng cho chân (*«`dung` là bản vẽ cũ, không đổi»*):

```text
TuyChonNep thêm:   gap?: "trang" | "manh"     mặc định "trang"
  "trang" = bản vẽ HIỆN TẠI, không đổi một toạ độ
  "manh"  = biến thể gấp làm tư
```

**Cổng:** `art-duong` và các ca hình học chạy **cho cả hai biến thể**; một ca đòi
`gap` mặc định `"trang"` và đòi mọi lớp bản `"trang"` **trùng từng toạ độ** với
bản hiện tại; luật **đúng một lớp coral** kiểm trên cả hai; `rudi-chat-sticker`
không đổi.

**Sticker cho sổ đôi cần ADR**, vì `test_sticker_vocabulary_matches_client.py`
ghim bộ sticker với máy chủ. **Lát 1 và 2 không có sticker mới**; sáu tư thế trên
chỉ dùng **trong tờ giấy**, không vào khay sticker.

### 17.5 Bản tối

Thân là `paper`, hai nếp là `paperShade`, mực là `muc`, và **góc coral hé ra là
thứ ấm duy nhất trên cả màn tối**. Một mảnh giấy sáng trên bìa vải, có một góc
còn ấm.

---

## 18. Cố ý không làm

| Không làm | Vì sao |
|---|---|
| Điểm tương thích, «% match» | số bịa trong giọng của sự kiện; ADR-0017 cấm đúng loại này |
| AI đoán tâm trạng, thanh sức khoẻ quan hệ | không đo được, và một người đọc thấy một lần là mode chết |
| Thời tiết, lịch rảnh, vị trí sống, geofence | ADR-0018 giữ nguyên: không quyền vị trí |
| Reveal hẹn giờ có lời hứa về quán | danh mục không có giờ mở cửa |
| «Lần tới hai người đi cùng nhau» | check-in chỉ chứng minh **một** người đã bấm |
| Streak, badge, bảng xếp hạng, «còn nợ» | lấy cảm giác tội lỗi làm động lực |
| Chia bill mặc định trong sổ đôi | sổ nợ giữa hai người là phản cảm; nhưng **giữ cửa vào sổ cái** |
| Hệ hình ảnh thứ hai | tờ giấy gấp cùng hệ với trang giấy và sổ đóng |
| Suy ra «hai người này là đôi» | kịch bản tệ nhất app có thể tạo |
| Video kỷ niệm tự sinh, bưu thiếp AI | chưa giải bài nào ở mục 0 |

---

## 19. Lát, đo, và tiêu chí giết

### 19.1 Ba lát

```text
LÁT 1   VÒNG TRỌN VẸN
        đường «Rủ đi chơi» · tờ lời rủ với máy trạng thái mục 3 ·
        LẬP SỔ (bậc 2) và BẬT «MỘT ĐÔI» (bậc 3), đủ consent hai chiều ·
        gậy đổi lượt · hai ô ràng buộc (không ăn được, đừng) ·
        hàng «Tờ giấy của hai mình» + không gian giấy ·
        thêm một dòng sau buổi · đóng sổ · rút / hết hạn / nghỉ / lỗi ·
        module bản tính (13.3) + cổng đếm rẽ nhánh ·
        biến thể Nếp "manh" + giu-kin + ba tư thế
        KHÔNG cần: thông báo · sổ bảy mục · túi riêng · hẹn mở

LÁT 2   HIỂU NHAU
        sổ bảy mục · ôn thẻ · câu hỏi tuần · ba dòng dặn ·
        núm độ mới · lời mời «thử một điều mới» · Cài đặt sổ đôi

LÁT 3   THÂN MẬT VÀ KÝ ỨC
        hẹn mở · túi riêng · thư gửi năm sau · giấy cũ quay lại ·
        trang tặng · bản đồ hai người
```

**Lát 1 khép trọn vòng ở mục 2.** Đó là điều bản 1 không làm được và là lý do
Codex REQUEST_CHANGES.

**Codex bắt thêm ở lượt ba, đã sửa ở trên:** lát 1 **có gậy**, mà gậy là cơ chế
của **sổ đôi** (bậc 3), nên lát 1 **phải có cả đường bật đôi với đủ consent** —
không được giả định nó đã bật. Và **nhận lời đi chơi (bậc 1) không tự tạo sổ**
(7.2): một người đi chơi cùng mà không lập sổ là đường hợp lệ, buổi đi ghi như
mọi buổi bình thường và không tờ giấy nào được giữ lại.

### 19.2 Vì sao hai ô ràng buộc phải đi cùng lát 1

Đây là chỗ bản 2 **thêm** so với đề nghị của Codex, không phải chỉ nghe theo.
Codex đề nghị bỏ sổ khỏi pilot. Đúng phần lớn — nhưng **không có «không ăn được»
và «đừng» thì tờ lời rủ đầu tiên có thể đề nghị đúng chỗ một người không ăn
được**, và đó là ấn tượng đầu **tệ hơn cả không có feature**. Nên: **hai ô trong
luồng lời rủ**, không phải một màn sổ bảy mục.

### 19.3 Đo theo chuỗi sự kiện, không đo một tỉ lệ

```text
được đề nghị → thực sự thấy → một người GỬI → người kia PHẢN HỒI →
cả hai chốt CÙNG MỘT phiên bản → buổi đi được ghi nhận →
giữ một điều → chủ động quay lại
```

Phân biệt **lời rủ do Nếp đề nghị** với **lời do người dùng tự khởi xướng**. Khai
rõ mẫu số ở mỗi bước. **Không dùng nội dung ghi chú riêng hoặc danh mục sổ đã
điền làm payload analytics**, và **không lấy tỉ lệ điền bảy mục làm mục tiêu** —
đó là tối ưu cho việc thu thêm dữ liệu riêng.

### 19.4 Tiêu chí giết, và cái mà tám lần **không** chứng minh được

| Tiêu chí | Ngưỡng |
|---|---|
| Vòng khép | sau **tám** tuần, nếu **chưa từng** có một vòng đi hết «gửi → phản hồi → chốt → giữ một điều» thì vòng sai, không phải mô hình sai |
| Nghỉ tăng | số tuần bấm «nghỉ» **tăng đều** là **tín hiệu phải đi hỏi lý do**, không tự đọc thành «Nếp ồn». Lý do là tuỳ chọn khi bấm nghỉ; chỉ khi lý do **thật sự** là «nhiều quá» mới siết hạn mức |
| Ai mở lời | nếu **chỉ một người** gửi trong tám tuần thì cơ chế gậy **không** chia lại được sự chủ động |

**Nói rõ giới hạn:** tám đề nghị **không** chứng minh mô hình hiểu gu, và bản 2
không có kế hoạch mẫu đủ để kết luận thống kê. Bản 1 đặt tiêu chí giết «twin»
theo tỉ lệ Ừ — sai hai lần: nó kết luận về mô hình từ một số đo về luồng, và nó
đọc «nghỉ tuần» thành «Nếp ồn» mà không hỏi lý do. Đã bỏ.

---

## 20. Quyết định đã chốt, và câu còn lại

### 20.1 Lead đã chốt (phiên 12/09)

| # | Câu | Chốt | Kéo theo |
|---|---|---|---|
| 1 | Mấy sổ đôi cùng lúc | **Đúng một** | DB thêm một ràng buộc unique; gậy và trần nói giữ nguyên giả định một sổ. **Cắt phạm vi V1**, mở lại được sau |
| 2 | Tên nút kết thúc | **«Đóng sổ»** | kèm màn xem trước **đếm rõ** sắp khoá bao nhiêu tờ, huỷ bao nhiêu lời đề nghị (7.6) |
| 3 | Đường khách vào V1 | **Không** | lát 1 giả định **cả hai đều cài app**. `guest_links` không dùng lại được (11.2); nếu mở lại thì là capability mới + ADR riêng |
| 4 | Chi tiêu chung vào lát nào | **Lát 3** | không mở bề mặt tiền ở ấn tượng đầu |
| 5 | Tên hai vai | **Người lo / Người chấm** | «Người mở lời» bị loại vì **chọi cơ chế gậy**: gậy luân phiên nên tuần nào cũng có người mở lời, bất kể ai giữ vai |
| 6 | Bản đồ vẽ mốc nào | **Chỉ buổi có tờ giấy** | bản đồ của những lần **đáng nhớ**, không phải log mọi lần đi ăn |
| 7 | Tab «Tạo mới» | **Thêm đúng một mục «Rủ một người đi chơi»** | cửa chính vẫn là đường trong cuộc nhắn riêng; mục này là cửa cho người **chưa** có cuộc nhắn riêng |

### 20.2 Tác giả tự quyết — không hỏi ai

Thuộc `apps/mobile/` và `app/web/`, tức phần tác giả sở hữu theo charter: câu
chữ · bố cục · giải phẫu thẻ và mọi số ở mục 15 · hợp đồng màu và vật liệu ở mục
16 · tạo hình Nếp, tên tư thế, biểu cảm ở mục 17 · thứ tự bề mặt trong một lát ·
cách đo hình (đọc mù, pixel diff, cổng đếm lớp).

**Đã quyết trong bản này, không mở lại trừ khi có bằng chứng mới:** mép giấy là
`lineStrong` · dòng lý do là `inkSoft` · vết gấp một nét `paperShade` giống nhau
hai scheme · bỏ ngưỡng 1.9 · «một tờ đang mở» thay ba thẻ · một chỗ chính thay
ba chặng · biến thể Nếp sau trường `gap` mặc định `"trang"`.

### 20.3 Câu cho Codex — và vì sao tác giả không tự chốt được

Bảy câu. Cột cuối là phần quan trọng: **lý do không tự chốt**, để Codex biết đây
là câu thật hay là tác giả đẩy việc.

| # | Câu | Đề xuất của tác giả | Vì sao không tự chốt được |
|---|---|---|---|
| **K1** | Hình bảng của **tờ giấy và phiên bản** | một bảng tờ giấy + một bảng phiên bản (nội dung theo phiên bản); phản hồi trỏ vào **phiên bản**, không trỏ vào tờ | `db/` là của Codex. Và chọn sai hình thì luật «chấp thuận gắn phiên bản» (3.2) **không cưỡng chế được ở tầng DB**, chỉ còn trông vào code nhớ làm đúng |
| **K2** | **Trạng thái «đôi»** cắm ở đâu | một bảng riêng, một hàng mỗi `pair`, CHECK `kind = 'pair'`, cần **hai** hàng chấp thuận | Tác giả **cấm được** việc thêm giá trị vào `contexts.kind` vì đã tra bốn chỗ vỡ, nhưng «cột hay bảng» là quyết định **migration**, thuộc Codex |
| **K3** | **`chot` sinh `outing`** ở đâu, khoá bằng gì | cùng transaction với `chot`; idempotency theo `(tờ giấy, phiên bản)` | `outings` **dùng chung với hội bạn**. Một đường ghi mới vào bảng của hội bạn không phải việc tác giả tự mở, và luật «không việc nền» cộng `install_commit_before_response` là luật máy chủ |
| **K4** | **Thang consent bốn bậc** cưỡng chế ở đâu | mỗi bậc là một hàng consent có mốc; mọi route đọc/ghi của sổ đôi khai vào roster quyền như mọi hành động khác | Chính Codex nói ở Xác nhận 2: phân quyền là **server-side**. Repo đã có roster mà **một hành động mới phải khai vào**; tác giả không sở hữu chỗ đó |
| **K5** | **Luật B** («Nếp gửi hộ» chỉ dùng nguồn chung) cưỡng chế thế nào | nháp mang **nhãn nguồn**; đường tự-gửi **từ chối** nháp có nhãn riêng | Tác giả viết được luật, nhưng chỗ cưỡng chế nằm ở tầng service. **Nếu nó chỉ là một `if` trong client thì luật vô nghĩa** |
| **K6** | Lát 1 có cần **lát thông báo** không | **không cần** — tờ giấy là một thẻ trong sổ, thấy khi mở app | Bảng `notifications` chưa có trên `main` (11.2). Nhưng nếu Codex đang định ship lát ấy trong vài ngày tới thì **chờ rẻ hơn tự né**, và chỉ Codex biết điều đó |
| **K7** | **Hai ô ràng buộc** lưu ở đâu | vùng **chung** của sổ đôi, **không** đụng `person_interests` | Đụng bảng gu cá nhân là đụng **ADR-0019** và cả truy vấn cộng-gu của **hội bạn**. Tác giả không được tự mở đường đó |

### 20.4 C1–C2: Lead khép, và đây là loại chữ ký nào

**C3** Codex tự khép ở lượt ba. **C1 và C2**: Lead khép phiên 12/09.

Ghi rõ **loại** chữ ký, vì hai loại không giống nhau và sáu tháng nữa sẽ có người
đọc lại dòng này:

| | |
|---|---|
| **Đã xảy ra** | Codex nêu ba điểm cụ thể ở lượt ba; tác giả sửa cả ba (3.2 · 19.1 · 7.7); **Lead khép** |
| **Không xảy ra** | Codex **chưa xem** ba bản sửa ấy. Đây **không phải** «reviewer đã xác minh», mà là **Lead nhận rủi ro và đóng** |
| **Vì sao rủi ro có biên** | hợp đồng máy chủ nằm ở **ADR-0027 do chính Codex viết**, và mọi hiện thực vẫn phải qua PR mà Codex đọc được. Chữ ký này đóng **tranh luận về tài liệu**, không đóng cổng của code |
| **Mở lại khi nào** | nếu lúc hiện thực lát 1 lộ ra rằng một trong ba bản sửa không đủ, nó quay lại thành finding bình thường, không cần xin phép ai |

Tác giả **vẫn không tự ký bản sửa của chính mình** — luật đó giữ nguyên. Cái đổi
là **Lead dùng thẩm quyền của mình để đóng**, và điều đó được ghi thành nguồn chứ
không lẫn vào một dấu tick.

### 20.5 Ba thứ không phải quyết định, mà là phép đo

Không ai trong hai bên chốt được bằng lập luận; chúng chỉ trả lời được **bên
trong lát 1**, trên bản dựng thật:

1. **«Một tờ đang mở» có thật sự nhẹ hơn «ba thẻ» không** — Codex đã rút yêu cầu
   frame trước kế hoạch (22.4); so frame là cổng **bên trong** lát 1.
2. **Trên cả khung có một hành động dẫn rõ không** — đọc mù ở cỡ thật (16.4),
   không phép đếm nào thay được.
3. **Vòng có khép không** — tám tuần, theo chuỗi sự kiện ở 19.3.

## 21. Từ vựng

| Từ | Nghĩa |
|---|---|
| **Sổ hai người** | một `context kind='pair'` đã lập sổ |
| **Tờ giấy** | vật trung tâm: lời rủ → kế hoạch → ký ức. Có **phiên bản** |
| **Mở lời** | bấm gửi một tờ lời rủ. Người gửi luôn có thật |
| **Giữ một điều** | thêm một dòng vào tờ giấy sau buổi đi |
| **Gậy** | quyền mở lời của tuần này, luân phiên; **tuần đầu thuộc người ít mở lời** |
| **Người lo / Người chấm** | hai vai trong một buổi |
| **Bản tính của sổ** | module khai sáu khác biệt giữa các loại sổ; **presentation policy**, không phải phân quyền |
| **Chủ của chặng** | ai quyết chặng đó |
| **Trang / Mảnh** | hai biến thể tạo hình của Nếp (`gap`) |
| **Giữ kín** | biểu cảm mới: biết mà không nói |
| **Túi riêng** | ngăn chỉ một người mở được |
| **Đóng sổ** | kết thúc chu kỳ. **Không** gọi là «mở giấy» |

---

## 22. Đối chiếu phản biện của Codex

### 22.1 Sáu mâu thuẫn C1–C6: đã khép

| ID | Khép ở đâu |
|---|---|
| **C1** máy trạng thái | **mục 3** trọn vẹn. Lượt 2 bổ sung: **3.2** gửi = đồng ý của người gửi và người đề nghị sửa là người gửi `v+1`; **3.3** bảng đủ đường `rut`/`nghi_tuan`/`het_han` cho mọi trạng thái; **4.2** lời rủ lập sổ là **lượt số 0**, gậy lượt 1 thuộc người kia |
| **C2** quyền đọc mâu thuẫn | Lượt 2 bổ sung **7.2 thang consent bốn bậc** (nhận lời đi chơi · lập sổ · bật «Một đôi» · cho Nếp đọc chat, mặc định TẮT), và **Luật B** ở 7.3: nháp dùng nguồn riêng **chỉ ra ngoài bằng cú bấm của chính chủ**; «Nếp gửi hộ» chỉ dựng từ nguồn chung. Cùng **mục 7.3–7.4**: bảng nguồn → ai thấy → dùng cho gợi ý nào → thu hồi khi nào; gu toàn cục **chỉ mồi cho gợi ý chính chủ thấy**; trước consent sổ riêng **chỉ là ghi tay** |
| **C3** đóng rồi mở lại | **mục 7.6**. Lượt 2 bỏ cửa sổ «tới hết chu kỳ» (vô nghĩa vì đóng sổ **chính là** hết chu kỳ) và thay bằng một luật: **đóng sổ là đóng, cái gì chưa mở thì thôi**; nháp và mọi lời đề nghị đang chờ đều **huỷ**; màn xem trước **đếm và nói rõ** sắp khoá bao nhiêu tờ |
| **C4** số và dấu trạng thái | **mục 16**: `inkSoft`, mép `lineStrong`, bỏ 1.9, vết gấp một luật; **16.4** accent có phạm vi; **15.3** tờ chưa tới lúc **mang chữ** (SC 1.4.1) |
| **C5** sự thật ghi nhận | **mục 8**: «chưa có trong sổ này», ba trạng thái biết/không hợp/**chưa biết**, bỏ hẳn hẹn mở bằng check-in |
| **C6** roadmap và đo | **mục 19**. Lượt 2 sửa nốt dòng còn suy «nghỉ tăng → Nếp ồn»: nghỉ tăng là **tín hiệu đi hỏi lý do**, và chỉ siết hạn mức khi lý do **thật sự** là «nhiều quá» |

### 22.2 D1–D8: giữ hay sửa

| ID | Kết |
|---|---|
| **D1** | **SỬA.** Hai lời hứa cũ không thể cùng đúng. Tác giả luôn có thật (3.4); bỏ luật «sửa một thứ mới được gửi» |
| **D2** | **SỬA, có thêm.** Lát 1 là vòng trọn vẹn. **Nhưng** hai ô ràng buộc phải đi cùng (19.2) |
| **D3** | **SỬA.** Một tờ đang mở (15.1); thứ tự theo tình huống, không cứng |
| **D4** | **SỬA.** Mặc định một chỗ chính (5.2); số chặng **tách khỏi** số nếp |
| **D5** | **SỬA.** Tính trước, đặt ngưỡng sau (16). Tự tìm thêm lỗi thứ ba |
| **D6** | **GIỮ một phần.** Không biên nhận **chỉ** cho câu hỏi riêng; ba mốc tách bạch (7.5) |
| **D7** | **SỬA.** Trước consent sổ chỉ là ghi tay; cold start chỉ mồi cho chính chủ (7.3–7.4) |
| **D8** | **SỬA.** Tờ lời rủ **là** mảnh giấy (10.1), nên vòng gửi–mở–giữ có ngay lát 1 |

### 22.3 Bốn chỗ tác giả phản biện lại

**R1 — Đừng bỏ hết sổ khỏi pilot.** Codex đề nghị hoãn sổ. Đúng với **bảy mục**;
sai với **hai mục**. Không có «không ăn được» và «đừng» thì tờ lời rủ đầu tiên có
thể đề nghị đúng chỗ một người không ăn được. Đề nghị: hai ô nằm **trong luồng
lời rủ** (19.2). *Xin xác nhận điều này không phá phạm vi pilot mà Codex hình dung.*

**R2 — Không thể lấy frame native làm điều kiện TRƯỚC kế hoạch.** Codex đòi hai
frame 360dp/2.0/IME để chọn giữa «ba thẻ» và «khe thu gọn». Không dựng được frame
native mà chưa có code, và dựng code để so là chính cái đắt. Đề nghị: **chốt bằng
câu chuyện** — một cuốn sổ mở **một tờ** mỗi lúc, còn ba thẻ trên một
`FlatList inverted` là bảng điều khiển chứ không phải sổ — lấy 15.1 làm mặc định,
và đặt **so frame là cổng BÊN TRONG lát 1**, chạy trên bản dựng thật, trước khi
lát 1 được gọi là xong. Nếu Codex vẫn muốn cổng ấy đứng trước, xin nói rõ dựng
frame bằng đường nào mà không phải viết chính màn đó.

**R3 — Tác giả đi xa hơn C5.** Codex đề nghị «chốt điều kiện mở bằng thao tác
rõ». Tác giả đề nghị **bỏ hẳn** hẹn mở «lần tới hai người đi cùng nhau» (10.2):
nó chỉ là một lời hứa cảm biến trá hình, và một thao tác rõ để thay thế thì
không còn nghĩa gì. Ba lựa chọn còn lại đều là mốc thời gian thật.

**R4 — «Một điểm nhấn» phải đếm ở NGUỒN mới kiểm được.** Đồng ý rescope accent.
Nhưng «không đếm nội dung người dùng» chỉ thành cổng khi nói rõ đếm ở đâu: đề
nghị đếm **lớp coral do hàm art của mình phát ra**, như `art-duong` đang đếm lớp,
**không** đếm pixel trên ảnh chụp (16.4). *Xin Codex xác nhận cách đếm này đủ.*

### 22.4 Lượt hai của phản biện (Codex, PR #609 · `6e3cd013`)

| Codex nói | Xử lý |
|---|---|
| **R1** đồng ý hai ô ràng buộc; cần rõ ai nhập, ai thấy, dùng cho gợi ý nào | **9.2** thêm bảng ba cột. Hai ô cố ý là **vùng CHUNG**: điều mình tự khai để người kia tránh thì giấu đi là vô nghĩa. Người kia **thấy, không sửa**; gỡ thì mất ràng buộc từ tờ sau, **không hồi tố** tờ đã chốt |
| **R2** rút yêu cầu frame native trước kế hoạch; chọn §15.1 mặc định | Nhận. **15.1** là mặc định; so frame là cổng **bên trong lát 1** |
| **R3** đồng ý bỏ hẹn mở lúc đi cùng | Đã bỏ ở **10.2** |
| **R4** đếm nguồn đủ cho **số lớp coral**, chưa chứng minh **một hành động dẫn trên toàn frame** | Đúng. **16.4** tách thành **hai cổng**: đếm ở nguồn cho lớp coral; **đọc mù ở cỡ thật trên khung đầy đủ** cho câu «việc cần làm bây giờ là gì?». Phép đếm **không** thay được vế hai |
| **Xác nhận 1 bị bác**: nháp dùng dữ liệu riêng vẫn có thể bị «Nếp gửi hộ» đẩy sang người kia | Đúng, và đây là đường rò thật tôi không thấy. **7.3 Luật B** đóng nó |
| **Xác nhận 2 đúng**: cổng đếm nhánh là tầng trình bày; server vẫn kiểm quyền và consent | Giữ nguyên **13.3** |

### 22.5 Lượt ba (Codex, PR #610 · `7d38fdcd`)

**K1–K7 chốt hết** → hợp nhất vào **11.3**, kèm hai chỗ Codex **sửa** đề xuất của
tác giả (K3 unique theo tờ; K5 kiểm lại lúc gửi).
**C3 Codex khép.** **C1–C2 sau đó do Lead khép** (20.4). Ba điểm Codex nêu, và chỗ đã sửa:

| Codex nêu | Đã sửa ở đâu |
|---|---|
| «Nếp gửi» không đồng nghĩa người giữ lượt đã đồng ý; phải chờ **cả hai** nhận lời | **3.2**: luật «gửi = đồng ý» chỉ áp cho **một con người bấm gửi**. Tờ do **Nếp gửi hộ** cần `dong_y` của **cả hai** mới `chot`. Bảng 3.1 có **hai đường vào `chot`** |
| Lát 1 có gậy thì **phải có đường bật đôi đủ consent**; nhận lời đi chơi **không tự tạo sổ** | **19.1**: lát 1 thêm **lập sổ (bậc 2)** và **bật «Một đôi» (bậc 3)**. Và 7.2 đã nói bậc 1 không kéo theo bậc 2 — nay nói rõ **đi chơi mà không lập sổ là đường hợp lệ** |
| **Companion hiện tại vẫn đọc chat / cộng gu pair**; luật riêng tư mới phải bao phủ đường này | **7.7 mới**, và tác giả đã **kiểm trên cây**: `invoke_group_companion` chỉ đòi `is_group_member`, `pair` cũng là context có thành viên, `taste_profile` → `group_taste` cộng `person_interests`. Nên đây là **tình trạng sẵn có**. Thêm **Luật C**: luật A và B áp cho **mọi** đường sinh gợi ý trong context hai người, kể cả đường đang chạy |

**Một đính chính về mức độ**, để khoản ADR-0019 không viết quá tay: đường sẵn có
là **lộ gián tiếp qua thứ tự gợi ý**, **không** phải chỗ đọc thẳng ra gu người
kia. Nhưng tính ẩn danh mà ADR-0019 trông cậy thì **không còn** ở n = 2.

### 22.6 Hai điểm xin Codex xác nhận (lượt một, đã khép)

1. **C2 khép bằng một luật:** gu toàn cục của một người **chỉ mồi cho gợi ý mà
   chính người đó nhìn thấy**, không bao giờ mồi cho gợi ý chung. Đây là cách rẻ
   nhất để khép C2 — xin xác nhận nó đủ, trước khi dựng.
2. **Cổng đếm rẽ nhánh (13.3)** được khai lại là **presentation policy**, và phân
   quyền vẫn là kiểm tra phía máy chủ ở biên đọc/ghi. Xin xác nhận cách phát biểu
   này đúng ý C của Codex.

### 22.7 Những điểm «sửa gọn» đã sửa

«ba phần dọc» → gấp ngang (1.6) · «hơi cũ mềm» → **cắt** (1.6) · trích nhầm 1.06
→ sửa (16.2) · `guest_links` → **không dùng lại được** (11.2) · tab «Tạo mới»
liệt kê hành động → cần route map (11.2, 20.7) · cổng rẽ nhánh ≠ phân quyền
(13.3) · «người nhà» → **hoãn khỏi cửa vào** (14.2) · luật «không `text`» → chỉ
áp cho art trang trí (12.1) · «mở giấy» đổi tên thành **«Đóng sổ»** (7.6) ·
«Nếp im lặng» so với «nói khi được gọi» → hợp nhất ở 6.2–6.3 · bảng bề mặt thêm
**đóng sổ, rút, hết hạn, lỗi** (15.2) · «chỉ một đôi» → nói là **cắt phạm vi V1**
(20.1) · bỏ tên sổ buộc theo giới (9.1).

---

## 23. Điều kiện để bắt đầu viết code

Ba cửa, theo thứ tự:

1. ~~Lead chốt bảy câu~~ — **ĐÃ MỞ**, phiên 12/09, ghi ở **20.1**.
**Cập nhật lượt bốn:** **bốn cửa đều mở.** Bước tiếp theo là **kế hoạch triển
khai lát 1** — và **chỉ lát 1**.

2. ~~Codex ký khép C1–C2~~ — **ĐÃ ĐÓNG**, Lead khép phiên 12/09. Loại chữ ký
   ghi ở **20.4**: Lead nhận rủi ro, không phải reviewer đã xác minh.: C1 (gửi = đồng ý ·
   người sửa là người gửi `v+1` · lượt số 0 · bảng đường thoát đủ) · C2 (thang
   consent bốn bậc · Luật B cho «Nếp gửi hộ») · C3 (đóng sổ là đóng). R1–R4 và
   hai điểm xác nhận lượt một **đã khép** ở 22.4.
3. ~~Codex trả lời K1–K7~~ — **ĐÃ XONG** (11.3). ~~Ba ADR được chấp nhận~~ —
   **ĐÃ MỞ**, Lead chấp nhận phiên 12/09 (11.4, 11.5). Việc còn lại là **thao
   tác**: Codex gạt dòng trạng thái trên nhánh của mình.

Khi ba cửa mở: viết **kế hoạch triển khai cho đúng lát 1**, không viết cho cả ba
lát. Phần màn hình đi qua Impeccable pipeline như mọi việc frontend trong repo
này, và **đọc mù trước packet** ở mọi lát có hình.
