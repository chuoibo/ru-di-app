# POST /me/achievement-seen

achievements · core · **chỉ Go** (`python: absent`) · trạng thái trong bộ nhớ: không có

## Mục đích

«MỚI MỞ» trong sổ hành trình nhớ theo tài khoản, không theo máy (QA UI-160, đợt nâng cấp UI/UX 01/10/2026, batch B9).

- Trước đây danh sách «đã thấy» nằm trong bộ nhớ giao diện của máy. Một máy hay trình duyệt mới không có danh sách đó,
  nên trình bày huy hiệu đạt sớm nhất, từ nhiều ngày trước, như vừa mở. Huy hiệu mới thật thì không được trình bày.
- Sau khi sổ trình bày một huy hiệu như vừa mở (khoảnh khắc Nếp M8), app gọi route này. Từ đó mọi máy của người đó đọc
  được huy hiệu ấy đã được thấy.
- Lần trình bày đầu tiên được giữ: đánh dấu lại một huy hiệu đã thấy không đổi `seen_at`.

Vì sao chỉ Go: các route hành trình (`/me/achievement-*`) vốn chỉ có bản Go (`docs/migration/go-native-profile.md`).
`achievementv1` là writer duy nhất của bảng thành tựu.

## Lược đồ

Migration thứ hai của `achievementv1` (`services/core/internal/achievementv1/seen.sql`). Migration thứ nhất giữ nguyên
digest.
- Thêm cột `achievement_earned.seen_at timestamptz`.
- Huy hiệu đạt quá 48 giờ trước lúc migrate được coi là đã thấy. Đây đúng là luật app đã dùng trên máy mới, nên sau
  migrate không có huy hiệu cũ nào bỗng thành «MỚI MỞ».
- Xoá tài khoản: hàng `achievement_earned` xoá theo `people` (`ON DELETE CASCADE`), như trước.

## Xác thực và quyền

- `get_actor` (Bearer ở `prod`, header ở `dev`).
- Chỉ huy hiệu của chính người gọi. Không có tham số người.

## Đầu vào và đầu ra

- Thân: `{"badge_ids": [...]}`. Mỗi id phải là huy hiệu người đó đã đạt, không lặp. Trường lạ, danh sách rỗng, JSON hỏng
  → `400 invalid_request`. Id chưa đạt hoặc lặp → `422 badge_not_earned`.
- `200` `{"badge_ids": [...]}`.
- `GET /me/achievement-routes` trả thêm `seen` (bool) cho mỗi huy hiệu trong `earned_badges`.
- `GET /people/{person_id}/achievements` (người khác xem huy hiệu được trưng) **không** có `seen`: lúc nào một người xem
  sổ của mình là chuyện của họ.

## Bằng chứng

- Mã: `services/core/internal/achievementv1/handler.go`, `store.go` (`MarkSeen`, `Earned`), `migrate.go`.
- Không cần DB: `handler_test.go` (`Matches` nhận đường mới).
- PostgreSQL thật, `seen_postgres_test.go`, chạy qua `scripts/go_postgres_tier.sh`:
  - huy hiệu vừa đạt chưa thấy;
  - đánh dấu một huy hiệu không chạm huy hiệu khác;
  - đánh dấu lại giữ mốc đầu tiên;
  - huy hiệu chưa đạt, id lặp, danh sách rỗng, trường lạ đều bị từ chối;
  - người khác xem không có `seen`;
  - migration chạy hai lần không lỗi;
  - luật 48 giờ của migration.
- Người dùng: `apps/mobile/src/rudi/screens/ky-niem/AchievementsLive.tsx` (khối «MỚI MỞ»).
