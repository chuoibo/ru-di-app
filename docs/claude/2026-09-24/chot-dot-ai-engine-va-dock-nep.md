# Chốt đợt «một engine AI + dock Nếp» (24-09)

Người viết: phiên nhận bàn giao từ `ai-engine-unification` (PR #643) và
`chat-backend-go-migration` (PR #642). Ghi chép này đóng hai PR bàn giao đó: mọi việc
chúng liệt kê đã xong hoặc được chuyển tiếp ở mục 4.

protocol_version: không áp dụng (không phải lượt thí nghiệm). Verdict: không có
reviewer người; finish reviewer impeccable độc lập chấm dock Nếp `ship` ở vòng 5.

## 1. Cái đã vào `main`

| Commit | Nội dung |
|---|---|
| `79847ac2` | Gate `check_go_owned_python_touch` bỏ qua hàng mount; main đỏ từ `62a591fa` |
| `fd2ff16a` | Cổng tương phản đọc viền nghỉ của `Field` theo hình dạng mới; main đỏ từ `c36b114b` |
| `97ec1725` | #646 dock Nếp: tờ giấy cài trong lề sổ (squash) |
| `09d68856` | #647 một đường gọi AI, ADR-0036 (merge commit, giữ 13 commit gốc) |
| `8f027476` | #649 gu, ngân sách, roster tên thật, prompt, bộ đo chất lượng (squash) |
| `c0f60847` | #648 AI nhóm và feed thay đổi bật mặc định ở prod (squash) |

Số đo từng PR nằm trong commit message của chính nó.

## 2. Quyết định của leader trong đợt (24-09)

- Roster gửi model dùng **tên hiển thị thật**; câu hứa trên màn đổi trước (ADR-0036 §5).
- Model của AI nhóm: **`gemini-3.5-flash-lite`**.
- Đo chất lượng: duyệt 38 rồi thêm 16 lời gọi; dùng đúng 54.
- Đo native làm ngay, tuần tự; ba việc tiếp theo (xoá v1, `chia_bill`, não chữ Nếp) làm
  sau khi bốn PR vào main.

## 3. Cái tìm ra dọc đường, đáng nhớ

- **Kéo Nếp dọc ray làm app Android sập** («Tried to synchronously call a remote
  function»): callback cử chỉ gọi hàm JS thường trên luồng UI. Có sẵn trên main; bản
  web chạy worklet trên luồng JS nên không lộ. Chỉ máy ảo bắt được.
- **Nếp mất phiếu ngữ cảnh sau mỗi lần đổi màn** (effect của màn con chạy trước khi
  provider thấy pathname mới). Main và #646 sửa độc lập; đã hợp nhất về một cơ chế.
- **Trên web, bấm mép Nếp cuộn cả trang sang ngang 46px.** Phép đo xanh, chỉ ảnh bắt được.
- **Hai phiên cùng dùng số ADR-0034** trong cùng một ngày; ADR của đợt này đổi thành
  ADR-0036.
- Máy khởi động lại lần thứ tư lúc 14:45 (RAM cạn khi nhiều phiên chạy song song).

## 4. Cái còn mở

1. **Flow Maestro 22–25 đỏ sẵn trên main** từ `71e50a9f` (người mới vào thẳng Tin nhắn;
   flow 22 vẫn đòi «Khám phá» không hiện; 23–25 đỏ dây chuyền). Vì vậy flow 25
   («Đồng ý» không bị Nếp nuốt) và flow 30 chưa chạy được trên máy.
2. **Chất lượng AI nhóm 10/16** trên flash-lite (1 lượt/ca). Trượt: 01, 16 dị ứng; 03, 13
   giờ/giá; 08 không hỏi lại; 12 hồi quy (hỏi lại thay vì đề xuất giờ gặp).
3. **`LOI_GOI_AI` chưa có cổng** nối với mã từ chối của `chatassist` (nhiều mã chưa có câu).
4. Ba việc tiếp theo của ADR-0036: xoá v1 (bản đồ đầy đủ đã lập; điều kiện tiên quyết
   #648 vào main đã đạt), `command=chia_bill`, não chữ Nếp.
5. Tải realtime: lượt 30 phút trên main hiện tại, soak 24 giờ, `Mark` giữ `FOR UPDATE`.

## 5. Bằng chứng đã xem

- Ảnh dock (web, sáng/tối) và kết quả đo: `~/.cache/rudi-bang-chung/dock-1225/`, canary
  `dock-1233/` (ngoài repo, máy của leader).
- Native: log `native-a*.log`, `do-mep-5.log` cùng thư mục.
- Chất lượng AI: `649-nen/`, `649-sau/`, `649-sau2/` (payload + thẻ thô + bảng điểm).
