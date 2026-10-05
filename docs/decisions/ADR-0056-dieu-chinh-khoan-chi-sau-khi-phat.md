# ADR-0056 — Điều chỉnh khoản chi sau khi đợt thu đã phát: đề xuất, mọi bên chấp thuận, phiên bản đợt thu mới

- Ngày: 2026-10-05.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-10-05** khi duyệt kế hoạch sửa audit Codex (finding RS-03):
  «làm quy trình điều chỉnh đầy đủ»; khách duyệt **trên trang link khách mới, link cũ bị thu hồi**. Không phải chữ ký
  Lead.
- Chạm **luật tiền 3** và quy tắc «sửa khoản chi tạo phiên bản mới»: văn bản này có trước mã.
- Không đổi: luật 1 (số nguyên đồng), luật 2 (Σ phân bổ = tổng khoản chi, qua allocator), sổ phiên bản bất biến,
  `/balances` (vẫn tính từ phiên bản mới nhất của mỗi khoản chi).
- Thực hiện spec `docs/superpowers/specs/2026-08-25-group-hangout-ai-design.md` §8.1 (L374–378), §9.1 (L473–480),
  bất biến 5 (L295), L442, L505, L513, L557.

## 1. Bối cảnh

Audit 2026-10-05 tái hiện: khoản chi v1 đã nằm trong đợt thu đã phát; confirm v2 (nội dung y hệt) tạo phân bổ mới, phân
bổ mới không có nguồn nghĩa vụ nên được đưa vào **đợt thu thứ hai**. Kết quả: tổng nghĩa vụ phải thu 100.000đ trong khi
số dư chỉ 50.000đ. Không có khái niệm huỷ, thay thế hay điều chỉnh nghĩa vụ nào được hiện thực; `collection_batch_versions`
có chuỗi phiên bản nhưng chỉ bản 1 được ghi. App hiện không có luồng sửa khoản chi; v2 chỉ sinh ra khi gọi API thẳng.

## 2. Quyết định

### 2.1 Khoá sửa trực tiếp

`POST /expenses/{id}/confirm` trả **409 `expense_in_batch`** khi bất kỳ phân bổ nào của bất kỳ phiên bản trước của khoản
chi là nguồn của một nghĩa vụ trong đợt thu **không ở trạng thái `cancelled`**. Sửa khoản chi đã vào đợt thu chỉ đi qua
điều chỉnh (§2.2). `LoadBatchInputs` bỏ qua nguồn thuộc đợt thu đã huỷ. Go và Python sửa cùng commit (route `python:
live`), có kịch bản parity.

### 2.2 Điều chỉnh (amendment)

- **Ai đề xuất**: chủ đợt thu (`batch_owner`) hoặc người ghi/người trả khoản chi đó, khi còn là thành viên đang hoạt động.
  Người ngoài nhóm nhận 404 `batch_not_found` như đợt thu không tồn tại.
- **Nội dung**: đúng một khoản chi trong đợt thu, với phân bổ mới (cùng đầu vào như confirm). Phân bổ được tính qua
  allocator; Σ = tổng khoản chi; số nguyên đồng. Phân bổ đề xuất **không** ghi vào `expense_versions` trước khi áp dụng:
  số dư không đổi cho tới khi mọi bên đồng ý.
- **Bên bị ảnh hưởng**: sender và recipient của mọi cạnh nghĩa vụ (sender → recipient, đã cộng theo cặp như lúc freeze)
  bị thêm, bị bớt hoặc đổi số tiền giữa phiên bản đợt thu hiện tại và phiên bản sẽ có. Cạnh không đổi không cần duyệt.
  Người đề xuất, nếu bị ảnh hưởng, được tính là đã đồng ý.
- **Vòng đời**: `proposed` → `applied` | `rejected` | `expired`. Một người từ chối là `rejected`. Hết hạn sau 7 ngày là
  `expired`. Mỗi đợt thu có nhiều nhất một đề xuất `proposed`.
- **Trong lúc chờ**: đợt thu không nhận điều chỉnh khác; xác nhận đã nhận tiền và báo đã chuyển vẫn đi tiếp trên phiên
  bản hiện tại.

### 2.3 Khách duyệt trên link mới

- Khi đề xuất, link của mỗi **khách bị ảnh hưởng** bị thu hồi (`rotated`); khách nhận một link duyệt mới
  `/g/{token}/dieu-chinh`. Trang này chỉ hiển thị envelope của chính khách: số cũ → số mới từng nghĩa vụ, lý do, hạn,
  hai nút **Đồng ý** / **Không đồng ý**. View model nằm ở biên guest view; template không tự query.
- Áp dụng: cùng token đó trở thành link của envelope mới (`/g/{token}` hiển thị nội dung đã đồng ý). Từ chối hoặc hết
  hạn: cùng token trỏ về envelope cũ, trang ghi «đề xuất không được áp dụng». Khách chỉ giữ một URL suốt quá trình, và
  không URL nào âm thầm đổi nội dung: link cũ đã thu hồi, link mới mở ra đúng bằng đề xuất.
- **Thành viên có tài khoản chỉ duyệt trong app, không bao giờ qua link** (vá rà soát bảo mật 05/10): người đề xuất là
  người cầm và gửi link duyệt, nên một link trả lời được thay thành viên thì người đề xuất tự "đồng ý" thay họ được.
  «Có tài khoản» = người đã từng có phiên (`account_sessions`, kể cả phiên đã thu hồi). Link khách cũ của họ vẫn bị
  thu hồi lúc đề xuất (để không còn hiện số sắp đổi); trả lời qua link với người có tài khoản → 403
  `amendment_answer_in_app`.
- Khách mà đề xuất **thêm** vào (trước đó không nợ, không có envelope) cũng nhận link duyệt; link đó không thay link
  nào (`old_link_id` NULL) và hết hạn theo hạn muộn nhất của các link trong đợt (ít nhất 7 ngày).
- Token duyệt **không sống lâu hơn link nó thay** (`collection_amendment_links.expires_at` = hạn link cũ): quá hạn thì
  trang và câu trả lời đều 404 (vá rà soát bảo mật 05/10).
- Khách mà đề xuất **gỡ hết** lượt chuyển vẫn giữ một URL: sau khi áp dụng, token trỏ tới một envelope **rỗng** của
  phiên bản mới (trang nói họ không còn phải chuyển gì), không trỏ về envelope cũ (envelope cũ vẫn hiện số cũ).

### 2.4 Áp dụng (một transaction)

1. Ghi phiên bản khoản chi n+1 với phân bổ đã duyệt (đường confirm sẵn có, bất biến).
2. Ghi `collection_batch_versions` n+1 (CHECK chuỗi phiên bản đã cho phép), nghĩa vụ, nguồn và envelope mới, tính
   bằng đúng thuật toán freeze trên tập nguồn của đợt thu với phân bổ của khoản chi đã thay.
3. Bảng **kế nhiệm** nối nghĩa vụ cũ → mới theo cặp (sender, recipient). Receipt và báo đã chuyển trên nghĩa vụ cũ tính
   vào trạng thái của nghĩa vụ kế nhiệm (trạng thái vẫn suy ra từ receipt, không lưu). Số mới nhỏ hơn số đã nhận cho ra
   `over_confirmed`; **không bao giờ đòi trả lại** (L505).
4. Link của sender **không bị ảnh hưởng** được chuyển sang envelope mới có cùng nội dung (kiểm từng số tiền bằng nhau
   trước khi chuyển); link của khách bị ảnh hưởng theo §2.3.
5. Audit event cho đề xuất, từng quyết định và lần áp dụng.

### 2.5 Đọc

- Board và trang khách đọc phiên bản đợt thu mới nhất, kèm lịch sử các phiên bản đã đổi nghĩa vụ (L557).
- `POST /obligations/{id}/confirm-receipt` trên nghĩa vụ đã bị thay: 409 `obligation_superseded`, kèm
  `successor_obligation_id` — **chỉ khi bearer là thành viên đang hoạt động của nhóm** (vá rà soát bảo mật 05/10);
  người khác đi thẳng tới route và nhận đúng câu trả lời cũ, không biết gì về việc thay. Guard này chỉ là tiện lợi: một
  receipt lọt qua (đua với lúc áp dụng) vẫn tính đúng, theo cặp ở `/balances` và theo chuỗi kế nhiệm ở bảng thu.
- Đọc theo chuỗi kế nhiệm (receipt, mốc «đã báo chuyển») bật bằng cờ ngữ cảnh `repo.WithSuccessions` mà cửa trước của
  tính năng gắn cho mọi request. Không bật thì mỗi câu SQL là đúng câu Python, từng byte — điều parity so.
- `/balances` không đổi.

### 2.6 Lược đồ và chủ sở hữu

Bảng mới thuộc một module Go (`internal/dieuchinh`, SQL nhúng, lệnh `migrate-amendments`, khai vào compose,
`deploy/vnlocal` và `scripts/e2e_slice.sh`; bật bằng `MOBILE_AMENDMENTS_ENABLED=1`, `core serve` từ chối khởi động nếu
cờ bật mà schema chưa có), append-only bằng trigger chặn UPDATE/DELETE trừ cột trạng thái đề xuất: `collection_amendments`,
`collection_amendment_lines`, `collection_amendment_decisions`, `collection_obligation_successions`,
`collection_amendment_links`. Không sửa bảng Alembic. Cột chứa person id khai trong registry xoá tài khoản.

### 2.7 Python

Route điều chỉnh là Go-only (`python: absent`). Python chỉ đổi đúng §2.1 (khoá confirm). Python không bao giờ thấy dữ
liệu điều chỉnh, nên guard `obligation_superseded` là Go-only và không làm lệch parity.

## 3. Hệ quả

- Hết lỗi thu hai lần một khoản chi. Sửa sau khi phát luôn có dấu vết và sự đồng ý của người bị ảnh hưởng.
- Khách bị ảnh hưởng phải được chủ đợt thu gửi link duyệt mới (như lúc phát).
- Golden vector mới: tăng, giảm, giảm dưới số đã nhận, thêm cạnh, bớt cạnh, đề xuất song song bị từ chối.
- Trạng thái đề xuất hết hạn được kết thúc **lười**: lúc có người đọc danh sách, đề xuất mới, hoặc trả lời. Không có
  worker riêng; một đề xuất quá hạn chưa ai chạm vẫn ghi `proposed` trong DB nhưng mọi lối đọc/ghi đều coi là hết hạn.
