# ADR-0034 — Gậy, vai, nhịp, chia gu và câu hỏi tuần của sổ đôi

**Trạng thái:** 🟢 **ĐÃ CHẤP NHẬN** 2026-09-25 — Lead chọn §4 và xác nhận bản sửa §2.3–2.4 («oke») · **Lựa chọn gốc:** Lead, 2026-09-23 (trả lời trong phiên QA cặp đôi)
**Soạn:** Claude, 2026-09-24 · **Sửa bổ sung:** ADR-0019 §2.1, ADR-0027 §4/§7, spec «Nếp truyền giấy» §4.2/§6.1,
dòng `CLAUDE.md` «AI … không tự đọc chat/gu/lịch sử»
**Liên quan:** ADR-0021 §2.5 (pair), ADR-0027 (sổ hai người), ADR-0031 (chat E2EE), ADR-0033 (Nếp nổi)

## 1. Bối cảnh

QA cặp đôi 23/09 (`docs/claude/2026-09-23/qa-cap-doi-minh-linh.md`) chấm «insight cho đôi» **1/10**: Nếp phác
«18:30 · Ăn tối» tuần nào cũng vậy; gu, ràng buộc, hồ sơ không được dùng; vai «Người lo / Người chấm», nhịp,
câu hỏi tuần chỉ khai trong `so/ban-tinh.ts` mà không màn nào đọc. Đợt 4 (24/09) đã làm phần **không cần quyết
định mới**: bản phác đọc lịch sử tờ đã chốt của chu kỳ (được phép theo ADR-0027 §4) và danh mục cùng thành phố.
Phần còn lại cần đổi những luật đang cấm nó, nên phải có ADR này trước.

Lead đã chọn, trong phiên 23/09:

1. **Gu:** «phải tìm ra insight gu chung của 2 người và tiết lộ gu của đối phương cho người còn lại để tiến dần
   đến sự companion nhất».
2. **Vai:** «default cho bên Nam lo, tuy nhiên có thể config nếu ý của user là hôm nay share».
3. **Câu hỏi tuần:** «phải làm ngay nhưng là LLM phân tích deep dive mỗi thời gian, time by time, không cứng».
4. **Chặng:** giữ tối đa 2 chặng mỗi tờ.

## 2. Quyết định đề xuất

### 2.1 Consent `chia_gu`, theo TỪNG NGƯỜI

Một mục đích consent mới, `chia_gu`, **mỗi người tự bật cho riêng mình** trong một sổ đôi đang bật («Cho người
ấy thấy gu của mình và cho Nếp dùng gu mình trong sổ này»). Không phải consent của cặp: bật là chia gu **của
người bật**, không kéo theo gu người kia. Thu hồi được bất cứ lúc nào; thu hồi thì người kia thôi thấy và Nếp
thôi dùng ngay lượt sau. Không bật = không ai đọc gu người đó (giữ nguyên ADR-0019 §2.1 cho mọi trường hợp khác).

### 2.2 Gu chung và gu của người kia

- **Gu chung** = phần giao của `person_interests` hai người, chỉ khi **cả hai** đã bật `chia_gu`: «Hai bạn cùng
  thích …».
- **Gu của người kia** hiện ở khối «Linh thích …» trên sổ/hồ sơ, chỉ khi **người đó** đã bật `chia_gu`.
- Nếp chỉ dùng gu của người đã bật; nguồn ghi vào `nguon.dung` («gu:<người>») như mọi nguồn khác.

### 2.3 Không có trường giới tính

Lead 25/09: «không cần thiết là Nam — AI tự phân tích xem ai có vai đó hơn qua mọi thứ hai người tương tác».
Vì vậy **không** thêm cột giới tính, và vai **không** mang tên giới («Người lo», không phải «bên Nam»). App
không suy ra giới tính hay đặc điểm nhạy cảm nào từ hành vi (đoán giới từ cách cư xử là gán khuôn cho một người
không hề đồng ý). Cái được suy ra là **ai hay chủ động lo**, và chỉ từ nguồn đã được phép (§2.4).

### 2.4 Vai và gậy

- **Người lo** mặc định = người **chủ động hơn trong sổ**, suy ra chỉ từ nguồn đã được phép: ai gửi tờ trước,
  ai đề nghị sửa, ai khởi xướng `lap_so`, trong chu kỳ này (ADR-0027 §4 cho đọc); chat **chỉ** khi `doc_chat` còn
  hiệu lực (chat là E2EE — máy chủ không đọc được nếu không có consent đó). Lời gọi mô hình tuần (§2.6) trả
  kèm đề xuất người lo — **không thêm lời gọi**; không có lời gọi thì máy đếm các tín hiệu trên theo luật tất
  định; hoà hoặc chưa có dữ liệu → người lập sổ. Lý do đề xuất ghi vào `nguon.dung` như mọi nguồn khác.
- Mỗi tuần đổi được: «Anh lo / Em lo / Hôm nay mình share» (share = cả hai cùng là Người lo, tuần đó không có
  Người chấm).
- **Gậy** tính, không lưu: bắt đầu từ Người lo tuần của `lap_so`, luân phiên theo tuần; không gửi vẫn sang; nối
  lại sổ thì tính lại. Gậy **không** là cổng quyền — chỉ quyết ai nhận bản phác của Nếp tuần đó.

### 2.5 Nhịp, hạn mức, nghỉ tuần

`packages/shared/nep-nhip.json`: 1 bản phác Nếp/tuần/sổ, 1 câu hỏi tuần/sổ, trần theo người 3 tờ/tuần. «Tuần này
nghỉ» dùng tờ `nghi_tuan` sẵn có (không bảng mới). Nếp **bước ra** khi cả hai đều đã từng đề nghị sửa trên cùng
một tờ (hai người tự lo được).

### 2.6 Câu hỏi tuần và insight bằng mô hình

Mỗi tuần một lần mỗi sổ đôi đang bật, brain Python (`/internal/brain/v1/…`, Python chỉ inference — `CLAUDE.md`)
nhận **chỉ nguồn đã consent**: lịch sử tờ/kèo/dòng giữ lại của chu kỳ (chung), gu của người đã bật `chia_gu`,
chat chỉ khi `doc_chat` còn hiệu lực trên đúng đề nghị (ADR-0027). Trả một câu hỏi cho Người chấm và một insight
cho Người lo. Kết quả **đóng băng vào DB** theo hash của đầu vào (bảng `pair_week_insights`), không bao giờ gọi lại
cho cùng dữ liệu; lỗi/thiếu khoá → không có câu hỏi, không bịa. Câu trả lời không có biên nhận, chỉ đi vào bản phác
chặng của chính người trả lời.

**Chi phí:** 1 lời gọi mô hình / sổ đôi đang bật / tuần. Mọi test và parity dùng brain stub tất định (0 lời gọi).
Một lượt đo chất lượng thật chỉ chạy khi Lead duyệt riêng số lời gọi (luật toàn cục về API trả phí).

### 2.7 Dữ liệu mới

Migration Alembic: mở CHECK `ck_pair_consent_proposals_consent_purpose_known` thêm `chia_gu`
(consent theo người: một đề nghị có một người nhận là chính người đề nghị); bảng `pair_cycle_rhythms` (khung tuần,
vai tuần đã chọn), `pair_week_insights` (hash đầu vào, câu hỏi, insight, `created_at`). Xoá tài khoản xoá các hàng
của người đó (`repo/erasure.go` + Python).

## 3. Cái này KHÔNG cho phép

- Không thu, không suy ra giới tính; không đặt tên vai theo giới.

- Không cho Nếp đọc gu của người chưa bật `chia_gu`, kể cả khi người kia đã bật.
- Không cho vai cấp quyền sửa hộ dữ liệu riêng của người kia (ADR-0027 §4 giữ nguyên).
- Không gọi mô hình ngoài lượt tuần đã khai, không gọi lại cho cùng hash, không đưa hai ô ràng buộc ra dịch vụ
  ngoài khi chưa có hợp đồng dữ liệu (ADR-0027 §7).

## 4. Lead đã chọn (2026-09-25)

1. §2.1–2.2 — **đồng ý**: `chia_gu` mỗi người tự bật; đổi ADR-0019 §2.1 và dòng `CLAUDE.md` cho đúng trường hợp
   đã bật.
2. §2.3–2.4 — **không dùng giới tính**; AI xác định người lo từ tương tác. Bản ở trên là cách làm điều đó trong
   luật E2EE/consent — Lead xác nhận 2026-09-25.
3. §2.6 — **đồng ý**: vận hành 1 lời gọi/sổ đôi/tuần; một lượt đo chất lượng `gemini-3.1-flash-lite`, **trần 30
   lời gọi**, gọi một lần, đóng băng kết quả ra file, báo lại số lời gọi thực tế.
