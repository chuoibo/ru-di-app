# Layout monorepo và ranh giới tầng

> Bản 2026-08-27 (`ADR-0006`) chia cây theo **hai người**, để hai engineer làm song song mà không chạm
> cùng file. **Viết lại 2026-09-22 theo `ADR-0032`**: chỉ còn một vai fullstack, nên cây này chia theo
> **tầng**, không chia theo người. Ranh giới còn lại là ranh giới kỹ thuật — và chúng được cưỡng chế
> bằng test, không bằng bảng phân công.

## Cây thư mục

> **Đổi từ 2026-09-14 theo `ADR-0029`:** lõi backend chuyển dần sang Go ở `services/core/` (cửa trước công khai,
> proxy route chưa chuyển về Python); `services/api/` chỉ còn "brain" AI sau decommission. Ai phục vụ route nào
> thì đọc `services/core/ownership/routes.json`, không đọc cây này.

```
services/core/                      Go 1.25 — cửa trước + lõi đang chuyển (ADR-0029)
  ownership/routes.json             manifest: route nào Go sở hữu, trạng thái, bằng chứng
parity/                             module Go riêng, hộp đen: so Python trước / Go sau
services/api/                       FastAPI, Python 3.12+ (đang chuyển; cuối cùng chỉ còn brain AI)
  app/
    domain/                         Thuần, không I/O, không framework
      allocator.py                  hiện thực ADR-0004
      contract.py                   hằng số + exception (từ phase0)
      ledger.py                     bất biến sổ, số dư tính lại được
      collection.py                 máy trạng thái đợt thu (spec mục 8)
    db/
      models.py · migrations/ · repository.py
    api/
      routes/ · deps.py · main.py
  tests/
    domain/ · db/ · api/            test đi cùng tầng nó kiểm

services/api/app/web/                Trang cho khách, render từ server
  templates/ · static/               Khách KHÔNG cài gì — nên đây là web, không phải RN

apps/mobile/                        Expo + TypeScript
phase0/                             ĐÓNG BĂNG TẠI CHỖ. Không sửa, không xoá
docs/protocol/v1/                   ĐÓNG BĂNG TẠI CHỖ
scripts/repo_guard.py               repo guard — fail closed
```

## Nguyên tắc phân tầng — không thương lượng

**`domain/` không được import bất cứ thứ gì từ `db/`, `api/`, hay `payments/`.**

Lý do là bất biến 3 của spec mục 6.8: *số dư luôn tính lại được từ sổ; cache không bao giờ là nguồn sự thật.* Nếu domain biết về ORM, sớm muộn có người tính số dư bằng một cột đã lưu.

Kiểm bằng test import, không bằng lời hứa.

## Ranh giới giữa các tầng

> **Đổi ngày 2026-09-22 theo `ADR-0032`.** Bảng sở hữu theo người hết hiệu lực. Một vai fullstack
> nhận trọn lát cắt: Go backend · SQL và migration · Python AI · TypeScript frontend · `apps/mobile/` ·
> trang khách · test mọi tầng.

Không có bảng «ai được viết file nào» nữa. Ba ranh giới dưới đây là **ranh giới kỹ thuật**, và chúng
không mềm đi chút nào khi chỉ còn một người — ngược lại, chúng là thứ duy nhất còn lại:

| Ranh giới | Cưỡng chế bằng |
|---|---|
| `domain/` không import `db` / `api` / framework | `tests/test_import_boundary.py` — parse AST, không phải lời hứa |
| Trang khách: template không bao giờ tự query; route và truy cập dữ liệu nằm ngoài template | test rò rỉ ở `app/web/guest_view.py`, không nằm trong file Jinja |
| Mỗi module có **đúng một writer** | `ADR-0031` — trong lúc chuyển Go, hai writer trên cùng một module là lỗi |

Domain là **thuần**: nhận `dict`, trả `dict`, ném `AllocationError`. Adapter sống ở `db/` và `api/`;
**không sửa domain để cho vừa framework**.

### Bảng sở hữu cũ, giữ để đối chiếu

Hai lane chạy từ 2026-08-27 tới 2026-09-16 (`ADR-0030` đóng lane backend) và tới 2026-09-22
(`ADR-0032` đóng phần còn lại):

| | Claude | Codex |
|---|---|---|
| ~~Sở hữu~~ | ~~`web/` (trang khách), `apps/mobile/`~~ | ~~`db/`, `api/`, `payments/`, `domain/`, test backend~~ |
| ~~Đụng vào của nhau~~ | ~~qua PR + review, không sửa thẳng~~ | ~~như trên~~ |
| ~~Nhánh~~ | ~~`claude/*`~~ | ~~`codex/*`~~ |

Hai mảnh domain từng ghi là «còn thiếu, thuộc Codex» — vòng đời `OffsetProposal` (spec mục 8.8) và
phạm vi capability của `GuestLink` (mục 8.2, bất biến 6) — nay không thuộc về ai riêng: chúng là việc
còn nợ, nằm trong `docs/team/hang-doi.md`.

## Lát cắt dọc đầu tiên

```
POST /expenses          tạo khoản chi, gọi allocator, trả đề xuất
POST /expenses/{id}/confirm    xác nhận → ghi ConfirmedAllocation vào sổ
POST /batches           gom nghĩa vụ chưa thanh toán thành đợt thu
POST /batches/{id}/publish     freeze → publish → sinh envelope
GET  /g/{token}         trang cho khách, KHÔNG cần cài app
POST /g/{token}/report  khách báo đã chuyển
POST /obligations/{id}/confirm-receipt   người nhận xác nhận
```

Đúng bước 2–4 của mục 14.3 trong spec. **Chưa làm Home, chưa làm tab, chưa làm vỏ chat** — mục 14.3 cấm thiết kế Home trước khi biết chính xác những hành động nào tồn tại.

## Ràng buộc mang từ spec sang, không được quên

| Ràng buộc | Nguồn |
|---|---|
| Tiền là **số nguyên đồng**, không float ở bất kỳ đâu | mục 4, bất biến 2 |
| `Σ ConfirmedAllocation == tổng khoản chi`, 100% | mục 4, bất biến 1 |
| Số dư **tính lại được** từ sổ; cache không phải nguồn sự thật | bất biến 3 |
| Sửa khoản chi tạo **phiên bản mới**, không ghi đè | mục 4 |
| `receiver_confirmed` **không phải** bằng chứng ngân hàng | mục 8, 15 |
| Không giữ tiền, không làm ví, và không nói chuyển vào đâu | mục 14.1 |
| `completed` chỉ do domain transition sinh ra, không có nút "đánh dấu xong" | bất biến 7 |
| Ghi riêng `recorded_by` · `paid_by/advancer` · `payer_acknowledgement` | mục 3 |
