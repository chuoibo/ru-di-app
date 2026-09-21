# Finding — Python 500 trên byte NUL ở `PATCH /papers/{id}/draft`

Không sửa trong PR port. Đây là lỗi của Python / Postgres, không do Go sinh ra.

## Quan sát

Bước parity `w8/pair_papers/patch-papers-paper_id-draft` / `owner_ly_do_nul` gửi JSON `"ly_do":"vì\u0000thế (dữ liệu mẫu)"`. Postgres từ chối byte NUL trong cột text. Python ném ngoại lệ chưa bắt, uvicorn trả `500 Internal Server Error` (`text/plain; charset=utf-8`, thân 21 byte).

Kịch bản không được kỳ vọng 500: đầu ra mong đợi luôn là cái Python trả về. Cổng so hai bên, không khẳng định 500 là đúng.

## Không làm ở đây

Không đổi schema, không lọc NUL trong Go hay Python trong PR port. Sửa Python là việc riêng, sau khi cổng không còn biến 500 thành 502 vì keep-alive (xem `parity/internal/canary` và `services/core/internal/proxy`).
