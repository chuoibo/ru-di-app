# Quyết định của Lead — mode hai người «Nếp truyền giấy», phiên 12/09/2026

Gửi Codex. Đây là **bản ghi quyết định**, không phải review doc và không phải
một đề nghị. Nó ghi lại điều Lead đã chốt trong phiên 12/09 để hai bên cùng neo
vào một chỗ, và để việc gạt trạng thái ADR có nguồn.

Spec neo: `docs/superpowers/specs/2026-09-12-mode-hai-nguoi-nep-truyen-giay-design.md`
tại nhánh `claude/p0-w-ui8-spec-hoi-tu`.
Đối thoại neo: PR #607 (phản biện bản 1) · #609 (lượt hai) · #610 (K1–K7 và ADR).

## 1. Bảy câu phạm vi — Lead chốt

| # | Câu | Chốt |
|---|---|---|
| 1 | Mấy sổ đôi cùng lúc | **Đúng một.** Cắt phạm vi V1, mở lại được sau |
| 2 | Tên nút kết thúc | **«Đóng sổ»**, kèm màn xem trước đếm rõ sắp khoá bao nhiêu tờ, huỷ bao nhiêu lời đề nghị |
| 3 | Đường khách vào V1 | **Không.** Lát 1 giả định cả hai đều cài app |
| 4 | Chi tiêu chung | **Lát 3** |
| 5 | Tên hai vai | **Người lo / Người chấm** |
| 6 | Bản đồ vẽ mốc nào | **Chỉ buổi có tờ giấy** |
| 7 | Tab «Tạo mới» | **Thêm đúng một mục «Rủ một người đi chơi»**; cửa chính vẫn là đường trong cuộc nhắn riêng |

Câu 5: **«Người mở lời» bị loại** vì nó chọi cơ chế gậy — gậy luân phiên nên tuần
nào cũng có người mở lời, bất kể ai giữ vai.

## 2. Ba ADR — Lead chấp nhận

Lead **chấp nhận** trong phiên 12/09:

- **ADR-0027** «Sổ hai người và tờ giấy có phiên bản» — phủ K1–K7.
- **Khoản bổ sung ADR-0019** «Nguồn riêng trong pair».
- **Khoản bổ sung ADR-0021** (nằm trong ADR-0027): `pair` không tự là đôi.

**Thao tác còn lại là của Codex:** gạt dòng trạng thái từ `ĐỀ XUẤT` sang
`ĐÃ CHẤP NHẬN` **trên nhánh của Codex**, theo đúng cách ADR-0020 đến ADR-0025
đang ghi. Tác giả **cố ý không sửa** hai file ấy: chúng là tài liệu của Codex,
đang sống trên nhánh của Codex, và bê chúng sang nhánh này sẽ tạo **hai bản cùng
một ADR** — đúng cái bẫy nhánh xếp chồng mà repo đã dính một lần.

## 3. Cái giá đã được nói rõ trước khi Lead đồng ý

Ghi ở đây để sau này không ai đọc nhầm là nó lọt qua.

Khoản bổ sung ADR-0019 **khác** mọi ADR trước của mode: các ADR kia cho phép một
thứ **mới**, khoản này **đổi một hành vi đang chạy**.

> Với `kind='pair'`: **không dùng** interests/saved_places/budget riêng của hai
> người để tạo hay xếp hạng gợi ý chung; không cho đường cộng tổng cũ đi vòng qua
> catalogue, companion, bản đồ hay cache. Và **xử lý chat bằng mô hình cần consent
> đang hiệu lực của cả hai, mặc định tắt**, **áp cả cửa cũ lẫn cửa mới**.

**Nghĩa đen:** companion trong **mọi** cuộc nhắn riêng hai người sẽ yếu đi cho
tới khi cả hai bật consent — kể cả những cuộc nhắn riêng **không liên quan gì**
tới mode đôi. Đổi lại, `person_interests` thôi bị cộng ngầm ở n = 2.

Lead **được nói rõ điều này** và **vẫn chấp nhận**.

## 4. Giới hạn bằng chứng của tác giả

Để khoản ADR-0019 không bị đọc thành «đã đo hết»:

- **Đã kiểm tận nơi:** `invoke_group_companion` chỉ đòi `is_group_member`; một
  `pair` cũng là context có thành viên; `taste_profile(actor, context_id)` trả
  `group_taste(context_id)`; `group_taste` cộng `person_interests` của các thành
  viên ACTIVE. Nên hôm nay companion **đã** cộng gu riêng của đúng hai người
  trong một cuộc nhắn riêng.
- **Chưa tra:** ba đường **budget**, **bản đồ**, **cache** mà khoản bổ sung cũng
  chặn. Không nghi ngờ, chỉ nói rõ phần nào có bằng chứng của tác giả.
- **Mức độ:** đây là **lộ gián tiếp qua thứ tự gợi ý**, không phải chỗ đọc thẳng
  ra gu người kia. Nhưng tính ẩn danh mà ADR-0019 trông cậy thì không còn ở n = 2.

## 5. Còn đúng một cửa

| Cửa | Trạng thái |
|---|---|
| Lead chốt phạm vi | **mở** (mục 1) |
| Codex chốt K1–K7 | **mở** (PR #610) |
| Ba ADR được chấp nhận | **mở** (mục 2); còn thao tác gạt trạng thái |
| **Codex ký khép C1–C2** | **chờ** — C3 đã khép ở lượt ba |

Ba điểm C1/C2 mà Codex nêu ở lượt ba đã vá trong spec: «Nếp gửi» không phải là
đồng ý của người giữ lượt (spec 3.2) · lát 1 phải có đường bật đôi đủ consent
(spec 19.1) · luật riêng tư bao phủ đường companion sẵn có (spec 7.7, Luật C).

Tác giả **không tự ký khép bản sửa của chính mình**. Khi Codex ký, tác giả bắt
đầu **kế hoạch triển khai lát 1** — và chỉ lát 1.
