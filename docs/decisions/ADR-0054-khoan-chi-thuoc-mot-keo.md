# ADR-0054 — Mỗi khoản chi thuộc nhiều nhất một kèo; «đã chia» của kèo đọc từ quy thuộc đó, không đoán theo ngày

- Ngày: 2026-10-04.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-10-04**, trả lời đề xuất
  `docs/claude/2026-10-01/ui-ux-upgrade/adr-de-xuat/UI-149-da-chia-theo-keo.md`: làm cả phương án C và A, «tính đường
  xa hơn cho production ready … giải quyết triệt để … consistent mọi conflict». Không phải chữ ký Lead.
- Chạm **luật tiền 3** («số dư tính lại được từ sổ»): văn bản này có trước mã, đúng quy trình của CLAUDE.md.
- Không đổi: luật tiền 1 (số nguyên đồng) và 2 (Σ phân bổ = tổng khoản chi); allocator; sổ phiên bản bất biến
  (`expense_versions` vẫn append-only); số dư nhóm (`/balances`) và tài chính cá nhân (`/people/{id}/finance`), vốn
  không dùng kèo.
- **Ngoại lệ Python có tên** (CLAUDE.md, ADR-0031): route tiền vẫn có Python làm oracle (`python: live`). Đây là sửa một
  lỗi **sai tiền**, loại blocker 2 của charter, nên Python được sửa cùng commit với Go, cùng kịch bản parity. Không thêm
  nghiệp vụ Python nào khác.

## 1. Bối cảnh

Bảng `expenses` không có cột kèo. Khoản chi được gán vào kèo bằng ngày: `on_date BETWEEN outings.starts_on AND ends_on`,
cùng nhóm (Go `repo.GroupRecap`, Python `group_recap`). Mọi con số tiền theo kèo đều đọc từ đó: recap, kệ và đầu album,
budget, hồ sơ gu (`preference-profile`), gợi ý (`suggestion`).

Hai kèo trùng ngày thì một khoản chi được đếm ở cả hai. QA (PR #663, UI-149) đo được:
- một khoản 13.705.678đ ghi «đã chia 13.705.678đ» ở cả hai kèo;
- hero quyết toán của nhóm ghi **27.411.356đ** cho một sổ chỉ có **13.705.678đ**.

Parity vẫn xanh vì hai bên cộng trùng giống nhau: kịch bản recap còn ghim chính lỗi này (một bữa tối nằm trong hai
chuyến chồng ngày). Một lỗi nữa cùng gốc: bill viết **sau** chuyến (`occurred_at` là lúc ghi) không thuộc chuyến nào,
dù được mở từ chính màn của chuyến.

## 2. Quyết định

### 2.1 Lược đồ (Alembic, chủ schema duy nhất theo ADR-0029)

- `expenses.outing_id uuid NULL`.
- Khoá ngoại ghép `(outing_id, context_id) → outings(id, context_id)`, kèm ràng buộc duy nhất
  `uq_outings_id_context_id`. Một khoản chi **không thể** thuộc kèo của nhóm khác: cưỡng chế ở tầng dữ liệu, không bằng
  lời hứa của route.
- Không `ON DELETE`: kèo đang giữ khoản chi thì không xoá được ở tầng dữ liệu. Tiền không bao giờ mất hay đi theo một
  kèo bị xoá. Hôm nay app không xoá kèo; một tính năng xoá kèo sau này phải tự quyết khoản chi của kèo đi đâu.
- Index `ix_expenses_outing_id` cho phép đọc theo kèo.
- `expenses` không phải bảng append-only. `outing_id` đặt một lần, không đổi qua các phiên bản (§2.2).

### 2.2 Luật quy thuộc: một luật cho dữ liệu cũ và mới

Một khoản chi thuộc kèo theo thứ tự:

1. **Kèo nó được ghi từ.** Thân yêu cầu có `outing_id` (khi mở chia bill từ màn kèo). Kèo đó phải thuộc đúng nhóm
   của khoản chi, nếu không: `422 outing_not_in_context`.
2. **Không ghi từ kèo nào:** kèo **duy nhất** của nhóm phủ ngày của khoản chi (ngày Việt Nam của `occurred_at`).
3. **Hai kèo trở lên phủ ngày đó, hoặc không kèo nào:** không thuộc kèo nào. Hệ thống không đoán.

Thời điểm đặt:
- `POST /expenses` (đề nghị) ghi `outing_id` khi thân có nó.
- Lần xác nhận đầu tiên (`POST /expenses/{id}/confirm`, phiên bản 1) áp luật 2 nếu chưa có quy thuộc.
- Khoản chi chưa thuộc kèo nào được quy thuộc vào kèo **đầu tiên** được nêu tường minh ở một lần xác nhận sau.
- Từ đó quy thuộc **cố định**. Một lần xác nhận sau mang `outing_id` khác: `409 expense_outing_mismatch`. Mang đúng kèo
  đó, hoặc bỏ trống: không đổi gì.

Backfill trong migration áp đúng luật 2 trên phiên bản mới nhất của mỗi khoản chi đã có. Dữ liệu cũ và dữ liệu mới vì
thế theo cùng một luật, không có hai thế hệ dữ liệu.

### 2.3 Đọc

- Mọi con số tiền theo kèo đọc `expenses.outing_id = outings.id`, không còn nối theo ngày. Ở mỗi bên chỉ có một nguồn:
  Go `repo.GroupRecap`, Python `group_recap`. Recap, album, budget, preference-profile, suggestion đi theo nó.
- Ảnh và check-in của kèo vẫn theo ngày như cũ: chúng không phải tiền, và một bức ảnh thật sự thuộc về ngày nó được
  chụp.
- **Phương án C** (hero quyết toán không bao giờ vượt sổ): tổng của hero là Σ các kèo đã kết thúc. Với §2.2, các tổng
  theo kèo rời nhau, nên Σ «đã chia» của mọi kèo ≤ tổng sổ của nhóm, và hero = tổng các khoản chi thuộc những kèo đó.
  Bất biến này có test ở cả hai bên.

### 2.4 App

- Chia bill mở từ màn kèo (`/smart-split/{outing}/review`) gửi `outing_id`. Đường dẫn đã mang id kèo từ trước, nhưng
  màn bỏ qua nó.
- Câu dưới hero của chuyến đang đi đổi từ «Tính từ sổ theo ngày của chuyến» thành câu nói đúng luật mới.

## 3. Phương án đã bỏ

| Phương án | Vì sao bỏ |
|---|---|
| Chỉ C (hero đọc Σ sổ, giữ nối theo ngày) | Sửa một con số, để «đã chia» của từng kèo vẫn sai. Nợ còn đó và lớn dần theo số kèo. |
| B (vẫn nối theo ngày, chọn một kèo: ngắn nhất, hoà thì tạo trước) | Thay cách đếm sai bằng cách đoán: bữa tối của chuyến 3 ngày có thể rơi vào kèo cà phê cùng tối. |
| `outing_id` trên `expense_versions` | Bảng append-only; quy thuộc đổi theo phiên bản sẽ làm «đã chia» của kèo cũ tự đổi khi ai đó sửa bill. |

## 4. Hệ quả

- Hai kèo trùng ngày, một khoản chi ghi từ kèo thứ nhất: kèo thứ hai «đã chia 0đ».
- Bill viết sau chuyến, từ màn của chuyến, nay thuộc chuyến đó.
- Khoản chi cũ nằm trong vùng hai kèo chồng ngày thành «chưa thuộc kèo nào»: không còn đếm hai lần, và không bị đoán.
- Client cũ không gửi `outing_id` vẫn chạy (trường tuỳ chọn) và được luật 2 quy thuộc.

## 5. Tiêu chí xong (bằng chứng phải có)

- Migration: up/down biên dịch offline; `test_migration_matches_models` xanh; backfill có ca PostgreSQL thật (một kèo
  phủ → gán; hai kèo phủ → NULL; không kèo → NULL).
- Go và Python trên PostgreSQL thật: quy thuộc tường minh, luật 2, kèo nhóm khác (`422`), đổi quy thuộc (`409`),
  bất biến Σ ≤ sổ, hai kèo chồng ngày không đếm hai lần.
- Oracle Go so với Python trên cùng SQL; parity hộp đen cho `POST /expenses`, `confirm`, `recap`.
- App: chia bill từ màn kèo gửi `outing_id`; hero dùng câu mới.
