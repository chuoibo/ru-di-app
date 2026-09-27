# ADR-0038 — Năm quyết định sau lượt đọc mù: lời rủ tạm thành trang đầu, nút chưa dùng được phải nói vì sao, mép Nếp là dải ruy băng, câu chữ, mức chi thứ tư

- **Trạng thái:** 🟢 **ĐÃ CHẤP NHẬN** 2026-09-26. Lead giao quyền quyết cả năm mục trong phiên 26/09 («think outside
  of the box để quyết định hành vi và design nào tốt nhất thì làm»), sau lượt đọc mù ở lát S9
  (`docs/claude/2026-09-25/san-khau-giay/README.md`, `docs/team/hang-doi.md` mục 2026-09-25 điểm 1 và 5).
- **Quyết định bởi:** Claude, theo uỷ quyền của Lead. **Hiện thực:** Claude, lát S10 trên nhánh
  `claude/practical-faraday-mswgmv`.
- **Bổ sung:** ADR-0027 §4 (lời rủ tạm trước khi lập sổ), ADR-0035 §2.1 (mặt của mép 10dp), ADR-0037 D8.
- **Không đụng:** hình học và mọi luật đo của ADR-0035 (lộ 10dp, tờ thứ hai 4dp, không tự nở, nhường chỗ), ba
  luật tiền, «một tờ mở mỗi sổ» (`uq_pair_papers_open_per_context`), route và mã lỗi hiện có.

## 1. Bối cảnh

Lát S9 cho một subagent context mới đọc 11 ảnh không nhãn. Nó đoán đúng việc chính ở 11/11 màn, nhưng đọc sai
năm loại chỗ mà S9 không tự quyết được, vì mỗi chỗ là một quyết định chứ không phải một lỗi vẽ. Thêm B1 của QC
24/09, vốn đã chờ Lead chốt luật.

## 2. Quyết định

### 2.1 B1: lập sổ thì lời rủ tạm đang viết thành trang đầu của sổ

Hiện trạng đo được: khi pair chưa có chu kỳ sống, `POST /contexts/{id}/papers/draft` tạo một tờ tạm
(`is_temporary`, `cycle_id` NULL). Đây là tính năng có chủ ý của ADR-0027 (lời rủ trước khi lập sổ), không phải
lỗi. Lỗi nằm ở chỗ nối: khi cả hai đồng ý `lap_so`, chu kỳ chuyển `active` nhưng tờ tạm đang mở vẫn là tờ tạm,
chỉ chủ nó đọc được, và nó giữ luật «một tờ mở mỗi sổ». Người kia bấm «Rủ đi chơi» nhận `paper_wrong_state` về
một tờ mình không thấy, tới hết tuần.

Đã cân ba hướng:

| Hướng | Vì sao không / vì sao có |
|---|---|
| Từ chối tờ tạm (`cycle_not_active` khi chưa lập sổ) | Xoá một tính năng ADR-0027 cố ý giữ, để chữa một lỗi ở chỗ nối. |
| Lập sổ thì huỷ tờ tạm | Mất chữ người ta đang viết, đúng lúc hai người vừa đồng ý với nhau. |
| **Lập sổ thì nhận tờ tạm vào chu kỳ** | Lời rủ đầu tiên thành trang đầu của sổ. Không mất chữ, không đổi luật một tờ mở. **Chọn.** |

Luật: trong cùng giao dịch kích hoạt chu kỳ (`grant_pair_consent`, mục đích `lap_so`, lúc cả hai đã đồng ý), mọi
tờ tạm của context có trạng thái hiệu lực thuộc `OPEN_STATES` được gắn `cycle_id` của chu kỳ vừa mở và
`is_temporary = false`. Tờ tạm đã khép hoặc đã hết hạn giữ nguyên. Nội dung, phiên bản, chủ nháp không đổi.

Sau đó tờ là tờ thường của tuần: nếu còn là nháp thì người kia thấy «Tờ tuần này ở phía …» (client đã có từ S2),
khi chủ gửi thì tờ tới tay người kia như mọi tờ khác.

Hiện thực ở **cả Python lẫn Go, cùng một commit**, theo tiền lệ ADR-0034 (hành vi mới sau khi port thì Python,
Go và app cùng mang). Python là oracle của parity cho route này (`routes.json`: `LIVE-GO`, `python: live`); sửa
một bên thì phép so vỡ. Đây là ngoại lệ có tên của luật «Python legacy chỉ sửa bảo mật/hồi quy»: một lỗi dữ
liệu chặn người dùng, sửa đúng một chỗ nối, ở cả hai stack.

### 2.2 Nút chưa dùng được phải nói vì sao, hoặc không hiện

Đo trên ảnh: nút tắt dùng `opacity` 0,45 (`RudiButton`) và 0,55 (`StampButton`). Chữ cam nhạt trên nền hồng
nhạt, người đọc mù hỏi «nút hỏng hay còn thiếu bước» ở 5 màn.

Luật mới, theo thứ tự ưu tiên:

1. **Không hiện** nút mà việc của nó chưa có nghĩa: «Lưu tên» chỉ hiện khi tên đã khác; người cùng nhóm (chưa
   là bạn) không thấy nút «Nhắn tin» khoá, chỉ thấy «Kết bạn» và câu «Kết bạn để nhắn riêng.».
2. Nút còn lại mà chưa dùng được thì **nói lý do ngay dưới nó**, bằng chữ nhìn thấy, không nằm dưới nắp
   (prop `lyDo`): «Viết vài chữ hoặc chọn một ảnh để đăng.», «Chọn một tấm ảnh trước đã.», «Chọn thêm 2 mục nữa.».
3. Trạng thái tắt **không mờ đi**: viền đứt `lineStrong`, chữ `inkSoft` đạt ≥ 4,5:1 trên nền của nó. Mờ bằng
   opacity bị bỏ ở cả hai nút.

Nút tắt vẫn là `disabled` với trình đọc màn hình, và lý do được gắn vào `accessibilityHint`.

### 2.3 Mép Nếp là dải ruy băng đánh dấu trang

Người đọc mù đọc mép 10dp là «khung trắng bị cắt, lỗi hiển thị» ở 8/11 ảnh. Hình học của ADR-0035 đúng (không
che chữ, đã đo); cái sai là mặt của nó: 10dp giấy trắng có góc gập không nói «kéo ở đây».

Mặt mới: phần lộ 10dp là **dải ruy băng đánh dấu trang** màu coral của Nếp, đuôi cắt chữ V hướng vào trang. Ruy
băng kẹp sổ là vật ai cũng biết là để kéo. Coral là màu góc gấp của chính Nếp, nên dải là một phần của Nếp chứ
không phải một vật mới. Kéo Nếp ra thì ruy băng thu về và mặt Nếp hiện như cũ.

Không đổi: bề rộng, vùng chạm, tờ thứ hai 4dp và màu của nó (§6 ADR-0035), luật trên màn tiền (mép chỉ là cửa),
nhường chỗ thì không vẽ gì. `tools/xem-dock-nep.mjs` giữ nguyên mọi phép đo.

### 2.4 Câu chữ

| Cũ | Mới | Vì sao |
|---|---|---|
| «Mở nhóm này» (dưới tên quản trị đầu tiên) | «Người lập nhóm» | Đọc như một nút «mở nhóm». |
| Tab «Lịch trình» / «Hành trình» | «Lịch trình» / «Bản đồ» | Hai chữ gần như một; tab thứ hai là bản đồ đường đi. |
| «Trang ngày của hội» | «Các chặng trong ngày» | Nói thẳng trang đó chứa gì. |
| «Mở một trang đường mới» | «Ngày này chưa có điểm nào trên bản đồ» | Câu cũ là lời mời làm một việc không có nút. Câu mới là câu flow 08 từng đợi. |
| Nightlife, Outdoor, Shopping, Game | Chơi đêm, Ngoài trời, Mua sắm, Chơi game | Một ngôn ngữ trên một bảng. Id không đổi. |

Nhãn gu đổi ở client, Python và Go cùng lúc: câu Nếp viết lý do phác («Minh thích …») đọc nhãn của máy chủ, nên
đổi một bên là hai giọng trên cùng một tờ.

### 2.5 Mức chi thứ tư, không có trần

Ba mức hiện có dừng ở 500K. Người chi nhiều hơn không có ô đúng và phải chọn sai hoặc bỏ qua. Thêm
`rong-tay`: «Trên 500K», chữ phụ «Rộng tay», `min_vnd` 500.000, không có trần (`max_vnd` null). Kiểu dữ liệu đã
dành chỗ cho mức không trần từ đầu (`den: null`, `openTopEnd`). Id cũ giữ nguyên nên hồ sơ đã lưu không đổi
nghĩa. Ba chỗ cùng mang: `so-thich.ts`, `app/domain/interests.py`, `internal/domain/interests`.

## 3. Cái này KHÔNG cho phép

- Không cho tự huỷ hay tự gửi tờ của ai khi lập sổ.
- Không cho nút nào mờ đi mà không có chữ nói vì sao, trừ lúc đang tải (`loading`).
- Không cho mép Nếp rộng hơn 10dp, mang chữ, hay tự nở.
- Không cho đổi id gu hay id mức chi đã có.

## 4. Bằng chứng phải có

- Test Python và Go cho luật 2.1: tờ tạm đang mở được nhận, tờ tạm đã hết hạn không; người kia xin tờ sau đó
  nhận `paper_wrong_state` về một tờ nay thuộc sổ (đọc được trạng thái), không còn tờ mồ côi.
- Tầng Postgres thật phía Go và parity cho route `grant_pair_consent` và mức chi mới.
- Ca ghim client cho 2.2–2.4 và ảnh chụp mở ra xem ở 412 sáng, 360 tối.
