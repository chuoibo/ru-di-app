# ADR-0041 — Nếp thấy rõ màn, nhớ lặng lẽ có nói trước, nhắc khi được cho phép

- Ngày: 2026-09-25.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0041 khác thì
  văn bản này nhận số trống kế tiếp, như ADR-0036 từng đổi số.
- Quyết định sản phẩm: người dùng chốt trong phiên lập kế hoạch 2026-09-25 (trí nhớ «lặng lẽ nhưng có
  công bố», hỗ trợ đúng lúc đúng chỗ, nhắc chủ động opt-in). Kế hoạch đã duyệt; ADR này chờ chữ ký Lead.
- Thiết kế chi tiết: `docs/claude/2026-09-25/thiet-ke-ai/05-nep-tai-cho-tri-nho-nhac.md` (commit gốc `f251db7`).
- Cùng đợt: ADR-0037 engine, ADR-0038 hàng đợi và stream, ADR-0039 nhóm trong luồng, ADR-0040 RAG,
  ADR-0042 eval.
- Thay ADR-0033 §2.5 và gạch đầu 1 của §4; thay ADR-0036 §2.7 và ba gạch đầu về Nếp của §4; bổ sung
  ADR-0024 §2.1–§2.4 và §4; thêm một vế cho dòng quy tắc AI của CLAUDE.md. Chi tiết ở mục 5; **không
  sửa bản lịch sử** của ADR nào.
- Không đổi: ba luật tiền; ADR-0033 §2.2 (Nếp lui ở màn tiền) và §2.6 (Nếp im trong nhóm); toàn bộ
  ADR-0035; ADR-0036 §2.6 (không mặt Nếp trên thẻ nhóm) và §2.9; bot nhóm không có trí nhớ dài hạn.

## 1. Bối cảnh

- Nếp hôm nay chỉ nhận một phiếu đếm số (`apps/mobile/src/rudi/nep/phieu.ts:57-65`): không biết app
  chạy thế nào, không nhớ gì qua phiên, không tự nhắc. Vài câu gợi ý trên màn hỏi đúng thứ nó bị cấm
  làm (`apps/mobile/app/(tabs)/plan.tsx:10`, `apps/mobile/src/rudi/screens/Profile.tsx:64`).
- Người dùng muốn Nếp biết người ta đang ở màn nào và màn đó có gì, đọc được luồng app để chỉ đường
  bằng lời, cá nhân hoá theo trí nhớ, và tự nhắc đúng lúc.
- Đang chặn: CLAUDE.md (AI không tự đọc chat, gu, lịch sử); ADR-0033 §4 («muốn thế thì mở quyết định
  khác»); ADR-0036 §4 (không giữ phiên Nếp ở máy chủ); ADR-0036 §2.5 (không rút lời hứa riêng tư).
- Trí nhớ ẩn hoàn toàn bị bỏ: Nghị định 13/2023 và Luật Bảo vệ dữ liệu cá nhân 2025 (hiệu lực
  01/01/2026) đòi thông báo và đồng ý khi lập hồ sơ hành vi; hai chợ ứng dụng bắt khai dữ liệu dùng cho
  cá nhân hoá; ADR-0036 §2.5 cấm rút lời hứa riêng tư.
- ADR-0024 đã chấp nhận nhưng chưa xây: ở `f251db7` không có bảng `notifications`,
  `notification_devices`, và không có `expo-notifications`.

## 2. Quyết định

1. **Phiếu ngữ cảnh v2, vẫn khoá đóng.** Thêm `ban`, `buoc` (registry đóng), `thay` (id catalogue, giờ
   chặng, điểm đến, danh mục), `hanhDong` (hành động đang có trên màn), `chon`, `banBuild`. Vẫn không
   tên người, chữ chat hay tiền dưới bất kỳ khoá nào; client gửi id, máy chủ tự tra catalogue. Khai cho
   7 màn; cổng quét nguồn quét cả `src/` lẫn `app/`.
2. **Sổ tay app là dữ liệu sản phẩm**, ở `services/core/internal/huongdan/data/`, nhúng vào binary. Nếp
   chỉ nêu một nhãn UI khi nhãn đó có trong sổ tay; một cổng giữ mọi nhãn là literal có thật trong app.
3. **Nếp chỉ hành động bằng chip có cấu trúc:** `mo`, `chi`, `mo_to`, `dien_nhap` (điền sẵn form trong
   RAM; người dùng tự bấm tạo). Chip không rút ra từ chữ trả lời, không dẫn tới màn tiền, không điền
   ngân sách.
4. **Nếp được đọc thêm đúng ba thứ:** (a) catalogue địa điểm công khai, qua registry tool của ADR-0037;
   (b) kèo sắp tới của **chính người đó**, không cột ngân sách, không lưu, chỉ khi họ hỏi trong lời gọi
   của chính họ hoặc đã bật «Nếp nhắc»; (c) trí nhớ về chính người đó khi đã bật «Nếp nhớ». Chat nhóm,
   gu, sở thích theo người, sổ tiền và lịch sử nhóm vẫn cấm.
5. **Trí nhớ lặng lẽ, có công bố.** «Nếp nhớ» mặc định tắt; một thẻ công bố hiện một lần và một dòng
   trong Cài đặt; không trang ký ức, không toast «đã nhớ». Máy chủ ghi bản văn công bố người đó đã đồng
   ý; đổi văn là tăng bản và hỏi lại.
6. **Hai nguồn, không hơn.** Thao tác trong app (sự kiện loại đóng, chỉ id và enum, ≤30 ngày) và lời
   chính người đó nói với Nếp (chỉ câu hỏi `hoi`, giữ ≤48 giờ để trích rồi xoá). Không lấy chữ chat
   nhóm, câu trả lời của model hay kết quả tool. Không nhớ tiền, người khác hay đặc điểm nhạy cảm; sức
   khoẻ chờ Lead quyết.
7. **Tự sửa theo thời gian.** Fact có mốc hiệu lực và độ tin cậy suy giảm; Nếp tự thêm, cập nhật, thay,
   cho hết hạn. Fact bị thay hay hết hạn bị xoá ở lần củng cố kế tiếp, không giữ bản cũ.
8. **Nói thật, quên thật.** «Bạn nhớ gì về mình?» do Go liệt kê hết, không qua model: fact, số thao tác
   30 ngày, câu hỏi đang chờ, lời nhắc đã hẹn, trạng thái công tắc. «Quên chuyện đó đi» là **xoá cứng**
   cộng tombstone băm HMAC. Tắt công tắc xoá sạch trong cùng transaction. Xoá tài khoản đi qua **một
   trigger Go trên `people.deleted_at`**, phủ mọi bảng Go có cột người qua một sổ đăng ký; bản đồ
   `ERASURE` của Python và `repo/erasure.go` không đổi.
9. **Mỗi bảng một người ghi:** `nepnho`, `nepnhac`, `push`. `chatassist` gọi hàm của `nepnho`, không ghi
   bảng của nó. `recall_memory`, `remember_fact`, `forget_fact`, `set_reminder` chỉ tới được từ gốc
   scope=me; bot nhóm không bao giờ tới `nep_*`.
10. **Nhắc chủ động opt-in.** «Nếp nhắc» (trong app) và «Nếp nhắc qua thông báo» mặc định tắt. Câu nhắc
    là mẫu cố định theo loại, không do model viết. Trần 3 lần/ngày trong app, 1 push/ngày; không push
    trong giờ yên 21:30–08:00 giờ Việt Nam; có khoảng nghỉ; bỏ qua 2 lần liền thì loại đó tắt 14 ngày.
    Không nhắc về tiền, không dẫn tới màn tiền; trong app chỉ đến qua tờ thứ hai của ADR-0035.
11. **Push tối thiểu của ADR-0024, bằng Go.** Đúng tên và schema ADR-0024, thêm `installation_id` cho
    thiết bị; kind `nep_nhac`; sender `log|expo`. Payload không nội dung (không tiêu đề kèo, tên quán,
    lời hẹn). Token không đổi chủ giữa hai lần cài app khác nhau. Chỉ xin quyền khi người dùng bật push.
12. **Câu hỏi của Nếp sống khi đóng bảng.** `{id, luot}` trong RAM của `NepProvider`, không xuống đĩa;
    xong thì báo bằng tờ thứ hai, tờ đó ẩn ở màn tiền.

## 3. Hệ quả

- Route mới (`/me/nep/su-kien`, `/me/nep/cai-dat`, `/me/nep/viec…`, `/devices…`) vào
  `ownership/routes.json` loại `GO-ONLY` cùng commit với handler. Bảng mới có bảng version riêng theo
  gói, số cấp theo thứ tự lên main.
- Củng cố và nhắc 15 phút chạy trong `jobs.DinhKy`; trích trí nhớ và push đi qua hàng `memory`, `notify`
  (ADR-0038). Trích thêm tối đa một lời gọi flash-lite mỗi job, đếm bằng bộ đếm của `aiharness/llm`.
- Cổng đọc xuyên gói phải vào main **trước** khi phiếu v2 tra catalogue; allowlist khai theo từng gốc.
- `expo-notifications` bắt dựng lại native; FCM credentials do Lead cấp; iOS ngoài phạm vi. Chưa có
  credentials thì chỉ `LogSender`, như ADR-0024 §4.
- Trigger nằm trên `people`, bảng do Alembic sở hữu; dựng lại bảng là mất trigger, nên `serve`/`work`
  từ chối khởi động khi trigger vắng.
- Văn công bố là một lời hứa riêng tư: đổi hành vi trước khi đổi văn là vi phạm ADR-0036 §2.5.

## 4. Cái này KHÔNG cho phép

- Không cho Nếp đọc chat nhóm, gu, sở thích theo người, lịch sử nhóm hay sổ tiền, qua bất kỳ đường nào.
- Không cho nhớ khi công tắc tắt; không nhớ về người khác, tiền, đặc điểm nhạy cảm; không suy giới tính.
- Không cho trang ký ức hay thông báo «đã nhớ».
- Không cho giữ câu trả lời của model hay lượt phiên ở máy chủ; không giữ `hoi` quá 48 giờ.
- Không cho «quên» bằng xoá mềm; không giữ fact đã bị thay hay đã hết hạn.
- Không cho gốc nào không phải scope=me, kể cả bot nhóm, tới được bảng `nep_*`.
- Không cho nhắc khi chưa bật, câu nhắc do model viết, hay push mang tiêu đề kèo, tên quán, lời hẹn.
  Không nhắc về tiền, không dẫn tới màn tiền.
- Không cho Nếp tự nở rộng hay tự viết lên trang (ADR-0035 giữ nguyên).
- Không cho tool nào của Nếp ghi kèo, tiền, bình chọn hay tin nhắn; `dien_nhap` chỉ điền form.
- Không cho sửa `repo/erasure.go` hay bản đồ `ERASURE` để xoá bảng Go.
- Không cho token push đổi chủ giữa hai lần cài app khác nhau.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

| Điều khoản | Hiện nói | Sau ADR này |
|---|---|---|
| ADR-0033 §2.5 | Phiếu khoá đóng: route, tiêu đề, nhịp, loại sổ, số đếm, gợi ý | Phiếu v2 ở §2.1; vẫn khoá đóng, vẫn không tên người, chat, tiền |
| ADR-0033 §4, gạch đầu 1 | Không tự đọc hội thoại, gu, lịch sử; muốn thế thì mở quyết định khác | Đây là quyết định đó, và nó hẹp: chỉ ba thứ ở §2.4 |
| ADR-0036 §2.7 | Nếp nhận lượt của phiên đang mở; phiên chỉ trên máy | Giữ; thêm task id trong RAM (§2.12), `hoi` ≤48 h khi bật nhớ, và §2.4 (b), (c) |
| ADR-0036 §4 «Không cho AI tự nói khi không ai gọi» | — | Giữ cho mọi đường có model. Ngoại lệ có tên: lời nhắc của Nếp, câu mẫu, không gọi model, chỉ khi đã bật, chỉ qua tờ thứ hai hoặc push. Bot nhóm không đổi (ADR-0039) |
| ADR-0036 §4 «Không cho Nếp đọc hội thoại nhóm, gu, hay lịch sử» | — | Chat nhóm và gu vẫn cấm; «lịch sử» mở đúng §2.4 (b), (c) |
| ADR-0036 §4 «Không cho giữ phiên Nếp ở máy chủ hay xuống đĩa thiết bị» | — | Giữ cho lượt phiên và cho đĩa; ngoại lệ: `hoi` ≤48 h khi bật nhớ |
| ADR-0024 §2.1.1 | `kind` đóng, 6 giá trị | Thêm `nep_nhac`; CHECK chỉ liệt kê kind đã có writer |
| ADR-0024 §2.2.1, §2.2.4 | Sender ở `app/api/push.py`; worker `push_pending` | Sender Go `internal/push`, cùng chế độ và luật log; gửi qua hàng `notify`, `pushed_at` vẫn là idempotency |
| ADR-0024 §2.2.2 | «Token của người khác đăng ký lại → chuyển chủ» | Chỉ đổi chủ khi cùng `installation_id`; khác thì 409 |
| ADR-0024 §2.3 | `AfterResponse` là cửa duy nhất cho việc sau response | Không dùng cho push Go; giữ nguyên cho Python |
| ADR-0024 §2.4 | Đăng ký token một lần mỗi phiên sau đăng nhập | Chỉ khi người dùng bật «Nếp nhắc qua thông báo» |
| ADR-0024 §4, gạch đầu 2 | Không thu hồi thiết bị tự động trong v1 | `DeviceNotRegistered` xoá thiết bị |
| CLAUDE.md, dòng «AI chỉ nhận nội dung…» | Ngoại lệ duy nhất là `chia_gu` (ADR-0034) | Thêm vế dưới đây khi ADR được ký |

Vế mới cho CLAUDE.md, nối sau «— ADR-0034»:

> ; trí nhớ riêng của Nếp về chính người đã bật «Nếp nhớ», chỉ từ thao tác trong app và lời người đó
> nói với Nếp, không chat nhóm, không tiền, không đặc điểm nhạy cảm; và kèo sắp tới của chính người đó
> khi họ hỏi hoặc đã bật «Nếp nhắc» — ADR-0041

## 6. Văn bản công bố (bản 1)

Thẻ một lần trong bảng Nếp, và dòng «Nếp nhớ» trong Cài đặt:

> Nếp có thể nhớ vài điều về cách bạn dùng Rủ Đi — loại chỗ bạn hay chọn, giờ bạn hay đi, điều bạn dặn
> Nếp — để gợi ý hợp hơn. Để làm vậy, Nếp ghi lại thao tác của bạn trong app trong 30 ngày, và đọc lại
> câu bạn hỏi Nếp rồi xoá sau tối đa 48 giờ. Câu bạn hỏi được gửi tới mô hình AI (Gemini của Google) để
> trả lời và để rút ra điều cần nhớ. Nếp không đọc chat nhóm, không nhớ tiền nong, không nhớ gì về người
> khác, không đoán giới tính hay điều riêng tư. Hỏi «bạn nhớ gì về mình?» là Nếp kể hết; nói «quên chuyện
> đó đi» là Nếp xoá hẳn. Tắt trong Cài đặt, hoặc xoá tài khoản, thì mọi điều đã nhớ bị xoá ngay.

«Nếp nhắc»: *Nếp nhắc bạn khi tới giờ bạn hẹn, khi kèo ngày mai chưa có chặng, hay khi kèo sắp bắt đầu.
Tối đa 3 lần mỗi ngày trong app, không bao giờ ở màn tiền.*

«Nếp nhắc qua thông báo»: *Tối đa 1 thông báo mỗi ngày, không gửi từ 21:30 đến 08:00. Thông báo không
kèm tên kèo, tên quán hay lời bạn dặn.*

## 7. Cổng nghiệm thu

- **Lát 13:** `nep-phieu-kin` quét `src/` và `app/`; `huong-dan-khop-ma` xanh; canary khoá `tenNguoi` đỏ ở
  cổng TS và bị Go trả 400; eval hướng dẫn đúng bước ≥0.85, bịa UI ≤2%.
- **Lát 15** (`scripts/go_postgres_tier.sh`, skip là đỏ): tắt công tắc và xoá tài khoản để lại 0 hàng;
  test liệt kê cột người của bảng Go đỏ khi có cột chưa đăng ký; canary fact đã seed không có trong
  request nào của nhóm; «quên» 100% trong store.
- **Lát 17:** giờ yên đúng ở 21:29, 21:30, 07:59, 08:00; payload không tiêu đề kèo; token khác
  `installation_id` → 409; mô phỏng 1.000 người-ngày 0 vi phạm; push tới máy thật chỉ chứng minh khi có FCM.
- **Mỗi lát:** chạy lại trong cây sạch đúng SHA; ít nhất hai đột biến tự nghĩ, kiểm tương đương trước,
  đỏ đúng bước dự đoán; mở ảnh chụp sáng, tối, Reduce Motion ra nhìn; số đo ghi thẳng vào commit message.
