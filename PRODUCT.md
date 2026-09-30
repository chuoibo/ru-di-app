# Product

<!-- impeccable:product-schema 1 -->

> **Nguồn lịch sử của bản ghi ban đầu.** Phiên chạy không có công cụ hỏi người dùng
> (`AskUserQuestion` không tồn tại trong tool surface, đã kiểm bằng ToolSearch),
> nên không có vòng phỏng vấn nào diễn ra. Mọi mục dưới đây rút từ ba nguồn văn
> bản đã có, và mục nào là **suy luận** thì ghi rõ `[suy luận]`:
> `/home/lakiet/mobile/product/feature_list.md` (spec 47 feature),
> `/home/lakiet/mobile/product/mockup.png` + 5 tờ trong `features/`,
> và `CLAUDE.md` của repo. Người chốt lại bản ghi này là Lead.

> **Cập nhật 27/09/2026.** Quyết định đã được người dùng duyệt và
> [ADR-0039](docs/decisions/ADR-0039-khep-cuoc-di-va-so-ky-niem.md) thay thế
> các mô tả cũ về nhóm 4–10 người, chế độ hai người riêng và AI tự giữ ngữ cảnh.
> Các phần không liên quan vẫn giữ nguyên; bằng chứng triển khai xem
> [Sổ kỷ niệm native](docs/testing/so-ky-niem-native.md).

## Platform

adaptive

Hai bề mặt cùng tồn tại và một người dùng thấy cả hai trong một buổi tối:
người tổ chức làm việc trong app Expo (`apps/mobile/`), người được rủ mở một
link web trên điện thoại của chính họ (`services/api/app/web/`). Điện thoại là
bề mặt chính. Đây không phải "web có bản mobile"; trang khách là một chặng thật
trong vòng lặp sản phẩm, nên nó phải dùng chung hệ thiết kế chứ không phải một
hệ thứ hai.

## Stack

Expo / React Native + TypeScript cho app; `packages/shared/` giữ hợp đồng
và token dùng chung. Theo ADR-0031, toàn bộ backend nghiệp vụ mới, API,
phân quyền, persistence, worker và migration thuộc Go/SQL; Python chỉ làm
inference, extraction và evaluation AI. FastAPI + Jinja + CSS thuần là runtime
trang khách legacy trong giai đoạn chuyển đổi, không phải mẫu cho backend mới.
Mỗi module có một writer; ownership hiện hành nằm trong
`services/core/ownership/routes.json`.

## Users

Hội bạn người Việt, phần lớn là sinh viên và người đi làm trẻ, đi chơi và ăn
uống theo **Hội bạn từ hai người** hoặc **Cặp đôi có đồng thuận**. Không có
chế độ hai người riêng. Sổ bạn bè cũ hiện như hội hai người; `pair` chỉ là
định danh lưu trữ/hội thoại, không tự xác nhận tình cảm hay mở lịch sử.
Trong một buổi có hai vai rõ rệt:

- **Người tổ chức.** Rủ, chốt chỗ, ứng tiền trả bill, rồi phải đòi lại. Đây là
  người chịu toàn bộ công việc khó chịu hôm nay, và là người mở app.
- **Người được rủ.** Muốn biết phần mình bao nhiêu và gồm những khoản nào.
  Người này thường **không cài app** — họ mở một link trong chat nhóm.

Việc thật đang diễn ra: chốt chỗ ăn giữa mười ý kiến, và chia một hoá đơn mà
mỗi người gọi món khác nhau.

## Product Purpose

Đưa cả vòng "tìm chỗ đi → rủ nhau → lên kế hoạch → đi chơi → ăn uống → chia
tiền → lưu kỷ niệm" vào app **Rủ Đi**. **Nếp** là bạn đồng hành kể chuyện,
không phải quyết định tự đổi tên ứng dụng. AI chỉ nhận phần người dùng chủ động
chọn và xác nhận chia sẻ; không tự đọc chat, gu hay lịch sử.

Đường đi PoC lịch sử tập trung vào chia bill là **một đường đi chạy thật,
đẹp thật**, không phải 47 feature nông:

```
mở app → đăng nhập → Khám phá (AI MATCH) → vào nhóm → chat, AI gợi ý chỗ ăn
→ chốt → chụp bill → AI đọc từng món → gán món cho người → allocator chia
→ kết quả: ai trả bao nhiêu → Cá nhân thấy tài chính cập nhật
```

## Positioning

Rủ Đi nối lời rủ, cuộc đi, phần tiền và câu chuyện mỗi người muốn giữ.
Buổi đi chơi ngắn thành **Khoảnh khắc**; du lịch thành **Sổ chuyến đi**.
Cùng ngày hay nhiều ngày chỉ là gợi ý loại, người dùng xác nhận.
Mỗi thành viên tự giữ bản của mình trên tường cá nhân, mặc định **Chỉ mình tôi**;
xem trước rồi chủ động chọn **Công khai**, không tự đăng lên tường người khác.

*Mô tả lịch sử đã thay thế:* AI "giữ context của nhóm xuyên suốt cả vòng"
không còn là cam kết sản phẩm. Ngữ cảnh cho mỗi lần AI làm việc phải có phạm vi
chia sẻ rõ ràng, không phải quyền đọc toàn nhóm.

Chốt về tiền: sản phẩm **không giữ tiền, không chuyển tiền, và không nói chuyển
vào đâu**. Nó nói mỗi người phải bỏ ra bao nhiêu và vì những khoản nào, rồi dừng.
Chuyển bằng cách nào là chuyện giữa hai người — đường thanh toán (VietQR, tài
khoản nhận) đã được gỡ khỏi sản phẩm.

## Operating Context

- Điện thoại, mạng di động, buổi tối, trong hoặc ngay sau bữa ăn. Người ta đang
  đứng dậy ra về khi chuyện chia tiền xảy ra.
- Hoá đơn là **ảnh chụp giấy** dưới ánh đèn quán: cong, loá, nghiêng.
- Người dùng tự thu xếp chuyển tiền ngoài app; mô tả PoC cũ về sinh QR đã
  được gỡ theo ADR-0015. Không có xác nhận ngân hàng chảy ngược về sản phẩm.
- Người được rủ mở link trên trình duyệt mặc định, thường không đăng nhập.

## Capabilities and Constraints

Đã chạy được và **không được viết lại**:

- Allocator chia tiền, có 41 golden vector tính tay.
- Sổ cái, máy trạng thái đợt thu, nghĩa vụ ai-nợ-ai.
- Trang khách `GET /g/{token}` với view model ở `app/web/guest_view.py`.

Ba luật về tiền, hiệu lực cả trong PoC (`CLAUDE.md`):

1. Số nguyên đồng. Không `float`, không `Decimal`.
2. `Σ` phân bổ `=` đúng tổng khoản chi, 100%.
3. Số dư luôn tính lại được từ sổ; cache không bao giờ là nguồn sự thật.

Ràng buộc riêng của tầng hiển thị:

- Template **không bao giờ tự query**. Chỉ render đúng view model backend trả về.
- Khách chỉ thấy envelope của chính mình: không số dư nhóm, không lịch sử,
  không allocation của người khác.
- `receiver_confirmed` không phải bằng chứng ngân hàng, và câu chữ không được
  nói như thể nó là.
- Câu chữ tiếng Việt, không dùng em-dash (có test bắt), không để lộ mã lỗi
  tiếng Anh ra màn hình người dùng.
- Phiên đăng nhập là Bearer từ `account_sessions` (ADR-0014, prod mặc định);
  `X-Actor-*` chỉ còn ở chế độ `dev`. Đăng nhập OTP/Google cho người mới: ADR-0016.

Đã quyết: bốn điểm đến Khám phá · Lên plan · Tin nhắn · Cá nhân + nút tạo mới nổi
(ADR-0013); phạm vi v1 = P0 của `product/feature_list.md` (ADR-0016, đề xuất).

### Khép cuộc đi và sổ kỷ niệm (ADR-0039)

- Người tổ chức khép cuộc đi; cặp đôi cho cả hai, hội cho chủ hội thay khi
  người tạo đã rời hội. Khép cuộc đi không làm hoàn tất đợt thu.
- Mỗi thành viên sở hữu, sửa và công khai bản của riêng mình. Ảnh nhóm mà họ
  được quyền đọc và chủ động chọn có thể nằm trong bản công khai, không cần
  xin phép từng tác giả; chủ sổ chịu trách nhiệm gỡ hoặc thay khi được yêu cầu.
  Công khai không mở URL ảnh nhóm gốc; thu hồi phải chặn cả metadata và byte ảnh.
- Trước khi dựng bằng AI, người dùng xem và xác nhận đúng gói ảnh/ngữ cảnh
  sẽ gửi. Có thể không gửi chat hoặc tự xếp trang không dùng AI. Hiện có ô dán
  trích đoạn được chọn thủ công; chưa có trình chọn chat E2EE trên thiết bị.
  Server không giữ khóa giải mã chat; AI không tự đọc chat, gu hoặc lịch sử.
- AI chỉ dựng nháp riêng, người dùng sửa trước khi lưu; dựng lại không ghi đè
  bản đã sửa. AI không quyết định quyền, không viết sổ cái, không tự công khai.
  Go/SQL sở hữu vòng đời, tác vụ và lời gọi model (qua agy-proxy, ADR-0051).

## Brand Commitments

Tên ứng dụng vẫn là **Rủ Đi**; Nếp là nhân vật đồng hành kể chuyện.
Hệ hiện hành giữ giấy, mực, coral và Bricolage theo `DESIGN.md`.
Những mô tả logo, slogan và ba tông dưới đây có nguồn từ mockup lịch sử;
không dùng chúng để khôi phục gradient trên control hoặc thay hệ đang chạy:

- Tên và wordmark **Rủ Đi**, chữ script nghiêng, dấu hỏi trên "u" là một phần
  của hình.
- Logo là squircle **gradient cam san hô**, cam ở trên trái chuyển sang hồng
  đỏ ở dưới phải. Đo được: `#fc7b37` → `#e75262`.
- Câu định vị đã có trên mockup: "AI đi chơi, chia bill thông minh".
- Câu hiệu triệu: "Rủ Đi thôi!".
- **Ba tông chức năng**, đọc được từ mockup và mang nghĩa, không phải trang trí:
  cam = thương hiệu và hành động chính · teal = chia bill và tiền ·
  tím = AI. Xem `DESIGN.md` để biết số đo và luật dùng.
- Giọng: xưng hô thân, ngắn, không hành chính. "Rủ Đi thôi!" chứ không phải
  "Khởi tạo chuyến đi".

## Evidence on Hand

Có thật, đường dẫn cụ thể:

- `/home/lakiet/mobile/product/mockup.png` (1448×1086) — 6 màn concept.
- `/home/lakiet/mobile/product/features/02..06-*.png` (1055×1491 mỗi tờ) —
  5 tờ feature, có màn chia bill 4 bước và màn AI chat.
- `/home/lakiet/mobile/product/feature_list.md` — spec 47 feature.
- Lượt Gemini thật được ghi trong tài liệu kiểm thử bên dưới; khóa provider
  thuộc cấu hình tiến trình `core` (ADR-0051), không phải bằng chứng để chép
  vào tài liệu/log.
- 41 golden vector allocator trong `services/api/tests/domain/golden/`.

Bổ sung 27/09/2026 theo [bằng chứng native](docs/testing/so-ky-niem-native.md):
Android Pixel 6 đã build/cài và chạy luồng sổ, sáng/tối, chữ lớn 1.3 và giảm
chuyển động; Gemini thật đã dựng từ ảnh được chọn. Đây là báo cáo của lượt
triển khai, không phải lượt kiểm thử chạy lại bởi người cập nhật tài liệu.
Chất lượng lời kể bám nguồn vẫn đang sửa: phát hiện suy diễn ngày/cảm xúc,
chưa có corpus evaluation chứng minh loại hết. Chưa có full gate xanh ở clean
tree tại SHA cuối, chưa kiểm native iOS, chưa audit câu chữ toàn bộ màn/layer.
Export iOS không thay bằng chứng native iOS.

Chưa có và **không được bịa**:

- Chưa có người dùng thật, chưa có testimonial, chưa có số liệu tăng trưởng,
  chưa có tên đối tác, chưa có đánh giá trên store.
- Ghi nhận lịch sử: ADR-0006 gác Giai đoạn 0 vì thiếu bằng chứng hành vi.
  Lượt native nêu trên chỉ chứng minh các luồng và cấu hình đã kiểm, không
  chứng minh người dùng thật hiểu sản phẩm hay mọi cổng phát hành đã đạt.
- Số liệu trong mockup (4.7 sao, 326 đánh giá, "AI MATCH 95%", tên Minh Anh /
  Quang Huy...) là **dữ liệu trình diễn**, phải dán nhãn là dữ liệu mẫu ở bất
  kỳ màn nào dùng lại chúng.
- Ảnh bill thật, số tài khoản thật, tên người tham gia thật: không bao giờ vào
  Git.

## Product Principles

1. **Vòng lặp thắng feature.** Một đường đi chạy trọn từ rủ tới chia tiền đáng
   giá hơn 47 màn không nối được với nhau.
2. **Tiền không được sai, kể cả trong PoC.** Một phép chia duy nhất trong sản
   phẩm; hai phép chia song song là cách chắc chắn nhất để hai màn hình hiện
   hai con số khác nhau cho cùng một bữa ăn.
3. **Người không cài app vẫn là người dùng hạng nhất.** Trang khách phải đẹp và
   rõ ngang màn trong app, và dùng chung hệ thiết kế.
4. **Vỏ thì nói là vỏ.** Feature nằm ngoài đường đi chính được làm đúng vỏ và
   dán nhãn. Giấu chuyện nó là vỏ mới là lỗi.
5. **AI có mặt thì phải nói rõ là AI.** Kết quả AI luôn sửa được bằng tay trước
   khi chốt; không có bước nào AI tự quyết chuyện tiền thay người dùng.

## Accessibility & Inclusion

- Chữ Việt đủ dấu là yêu cầu chức năng, không phải chi tiết đẹp: "ế ự ỡ ạ" phải
  render đúng ở mọi cỡ chữ và mọi font được chọn.
- Đích ngắm tối thiểu 44×44pt. Chuyện chia tiền xảy ra khi người ta đang đứng
  dậy ra về, một tay cầm điện thoại.
- Tương phản chữ đạt **WCAG AA: 4.5:1** cho chữ thường, **3:1** cho chữ lớn và
  cho thành phần giao diện. Đây là ràng buộc có số đo, và nó **đã buộc phải
  sửa màu của mockup** — xem bảng trong `DESIGN.md`.
- Không truyền đạt trạng thái chỉ bằng màu. Trạng thái tiền luôn kèm chữ.
- Tôn trọng `prefers-reduced-motion` và `prefers-color-scheme`.
