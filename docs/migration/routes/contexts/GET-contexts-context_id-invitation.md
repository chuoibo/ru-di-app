# GET · DELETE /contexts/{context_id}/invitation

contexts · core · **chỉ Go** (`python: absent`) · trạng thái trong bộ nhớ: không có

## Mục đích

Lời mời vào nhóm, nhìn từ phía người được mời (QA UI-080, đợt nâng cấp UI/UX 01/10/2026, batch B8): ai mời mình, nhóm
đã có bao nhiêu người, và một cách nói «không».

- Trước đây hàng lời mời ở Tin nhắn chỉ có tên nhóm và một nút «Đồng ý vào nhóm». Không có tên người mời, không có cách
  từ chối; lời mời không muốn nhận nằm mãi trong danh sách.
- Người được mời không đọc được danh sách thành viên (`GET /contexts/{id}/members` chỉ cho thành viên), nên tên người
  mời lấy ở đây.
- Từ chối chỉ đóng hàng `invited` của chính người đó: `state = 'left'`, `left_at = now()`, đúng như
  `ck_memberships_left_state_matches_timestamp`. Không đụng hàng nào khác. Index một-hàng-mở-mỗi-người
  (`left_at IS NULL`) cho phép một lời mời sau tạo hàng mới.
- Không đổi luật thành viên: ai mời được ai, ai đồng ý được, vẫn như `POST /contexts/{id}/members` và lối đồng ý.

Vì sao chỉ Go: cửa sổ duy nhất khác của người được mời là `GET /people/me/contexts`, còn Python làm oracle
(`python: live`). Thêm trường vào đó là thêm nghiệp vụ mới vào Python, trái ADR-0031. `DELETE
/contexts/{id}/members/{person_id}` (rời nhóm) chỉ nhận hàng `active`; đổi nó cũng phải đổi Python. Route mới thuộc về Go.

## Xác thực và quyền

- `get_actor` (Bearer ở `prod`, header ở `dev`), rồi path `context_id` (UUID).
- Chỉ hàng lời mời đang mở của chính người gọi, trong một nhóm (`kind = 'group'`). Nhóm không tồn tại, nhóm của người
  khác, lời mời đã trả lời, thành viên đang ở trong nhóm và cặp hai người đều nhận cùng một `404 invitation_not_found`:
  route không cho biết gì về một nhóm mình không được mời.

## Đầu ra

- `GET` → `200` `{"context_id", "display_name", "invited_by": {"id", "display_name"} | null, "member_count",
  "invited_at"}`. `invited_by` là `null` khi tài khoản người mời đã xoá. `member_count` chỉ đếm hàng `active`.
- `DELETE` → `204`.
- Cả hai: `Cache-Control: private, no-store`.

## Bằng chứng

- Mã: `services/core/internal/loimoi/handler.go`.
- Không cần DB: `services/core/internal/loimoi/handler_test.go` (path, phương thức, id sai, không có DB).
- PostgreSQL thật: `services/core/internal/loimoi/postgres_test.go`, chạy qua `scripts/go_postgres_tier.sh`.
  - Người được mời đọc được tên người mời và số người.
  - Người không được mời, người đang ở trong nhóm, cặp hai người: 404.
  - Người mời đã xoá tài khoản: không nêu tên.
  - Từ chối đóng đúng một hàng, lần hai là 404, một lời mời sau mở lại được.
- Người dùng: `apps/mobile/src/rudi/screens/groups/Conversations.tsx` (hàng lời mời).
