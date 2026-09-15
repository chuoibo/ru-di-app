# GET /people/me

people · core · trạng thái trong bộ nhớ: không có

## Mục đích

Hồ sơ của chính người gọi, với các con số máy chủ đếm lại từ nguồn ở mỗi lần đọc (M2): bạn bè, nhóm, kèo, điểm đã check-in, kỷ niệm; cùng cách đăng nhập, sở thích, band ngân sách và hai cài đặt quyền riêng tư.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:65-75`, `services/api/app/api/service.py:4070-4077`), đo trên stack:

1. Router: đuôi `/` → 307; `POST`, `HEAD` → 405.
2. `get_actor` → 401, 422 `invalid_actor_id` / `invalid_actor_roles`.
3. `_require_permission("view_own_profile", {"is_self": True})` (`services/api/app/domain/permissions.py:190`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_reads_without_roles`).
4. `get_person(actor.id)` không có → 404 `person_not_found` `Chưa có hồ sơ cho tài khoản này.` (`service.py:4072-4076`; `owner_reads_unregistered`). Không đọc `deleted_at`: trong dev người đã xoá tài khoản đọc được hàng ẩn danh của mình (`leaver_reads_after_erasure`); trong prod phiên đã bị thu hồi nên 401.

## Đầu vào

Không path, không thân. Query và thân lạ bị bỏ qua (`owner_reads_with_query_and_body`).

## Đầu ra

**200** `ProfileResponse` (`services/api/app/api/schemas.py:827-853`, dựng ở `service.py:4167-4189`), thứ tự khoá:

`id`, `display_name`, `bio`, `city`, `created_at`, `counts` {`friends`, `contexts`, `outings`, `places_checked_in`, `memories`}, `login_methods`, `interests`, `budget_band`, `wall_comment_policy`, `discoverable_by_phone`.

- `counts` (`services/api/app/api/repository.py:3928-3986`): `friends` = hàng `accepted` một trong hai chiều (không `pending`, không `blocked`); `contexts` = membership `active` của context `kind='group'` (không pair, không `invited`); `outings` = kèo của mọi context người đó `active` (pair cũng tính); `places_checked_in` = số `stop_id` khác nhau trong `outing_stop_checkins` của người đó (check-in `POST /contexts/{id}/checkins` **không** tính ở đây); `memories` = mọi kỷ niệm người đó tạo, kể cả check-in (`kind='checkin'`).
- `login_methods`: `account_identities.provider` khác nhau, xếp chữ cái (`repository.py:3988-3996`).
- `interests`: thẻ đã lưu, xếp theo thứ tự từ vựng (`normalise_interests`), không theo thứ tự gửi (`owner_states_interests` gửi `outdoor, cafe`).
- `budget_band`, `wall_comment_policy`, `discoverable_by_phone`: giá trị cột.

## Tác dụng phụ

Không. Sáu truy vấn đếm/đọc mỗi lần.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4071` |
| 404 | `person_not_found` | `Chưa có hồ sơ cho tài khoản này.` | `service.py:4072-4076` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:65-75`
- Service: `services/api/app/api/service.py:4070-4077`, `:4167-4189`
- Repository: `services/api/app/api/repository.py:3928-4010`
- Domain: `services/api/app/domain/interests.py` (`normalise_interests`)
- Quyền: `services/api/app/domain/permissions.py:190`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_my_profile_counts_come_from_the_five_sources` (109), `test_profile_routes_need_a_signed_in_person` (235)
- `services/api/tests/api/test_direct_messages.py`: `test_a_pair_is_not_a_group_on_the_profile_counts` (221)
- `services/api/tests/postgres/test_profile_postgres.py`: `test_the_counts_read_the_right_rows` (210)
- `services/api/tests/postgres/test_direct_message_postgres.py`: `test_a_pair_does_not_count_as_a_group_on_the_profile` (247)

## Kịch bản parity

`parity/scenarios/w10/people/GET-people-me.yaml`, id `w10/people/get-people-me` (40 bước, `dev`):

- Thứ tự: `anonymous_reads`, `actor_id_not_uuid`, `roles_unknown`, `owner_reads_without_roles` (`group_admin`), `owner_reads_unregistered`.
- Hồ sơ mới và đầu vào bị bỏ qua: `owner_reads_fresh`, `owner_reads_with_query_and_body`.
- Đếm: `owner_reads_two_friends` (hai `accepted`, một `pending` không tính); sau khi chặn một bạn, tạo một nhóm, được mời vào nhóm khác, mở một pair, một check-in và một kỷ niệm ảnh: `owner_reads_full` = `friends 1, contexts 1, outings 0, places_checked_in 0, memories 2` (check-in `POST /contexts/{id}/checkins` là một kỷ niệm `kind: checkin`, nên tính vào `memories`, không vào `places_checked_in`); sở thích gửi `outdoor, cafe` trả `cafe, outdoor`.
- `friend_reads_own`, `leaver_reads_after_erasure` (200 hàng ẩn danh, mọi đếm 0), `owner_reads_after_friend_ended` (bạn đã xoá không còn được đếm).
- Framework: `trailing_slash` (307), `post_not_allowed`, `head_not_allowed` (405 `allow: GET`).

`DELETE-people-me-world.yaml` `leaver_reads_profile_before`: `outings 1` đến từ kèo mà tờ sổ đôi sinh ra trong pair.

`prod-auth.yaml`: `anonymous_reads_profile` (401), `owner_reads_profile`, `owner_reads_profile_with_guest_roles` (200: vai trò từ roster), `stranger_reads_profile_claiming_owner` (hồ sơ của stranger), `owner_reads_profile_after` (401 sau xoá), `mate_reads_own_profile_after`.

Corpus sinh: `generated/w10-422/get-people-me.yaml` (5 bước).

## Chưa phủ / lưu ý cho bản Go

- Sáu truy vấn độc lập; đếm `memories` gồm cả check-in (kỷ niệm `kind='checkin'`).
- `places_checked_in` và `login_methods` khác rỗng chưa phủ: cần check-in điểm dừng của kèo (W7, chưa đóng băng) và danh tính OTP/Google (số điện thoại không được viết vào kịch bản).
- `interests` theo thứ tự từ vựng của `app/domain/interests.py`, không theo `tag` chữ cái.

## Lỗi Python (chỉ báo, không sửa)

- Không đọc `deleted_at`: trong dev người đã xoá tài khoản đọc được hồ sơ ẩn danh (200), trong khi `GET /people/{chính mình}` trả 404.
- Tên trường `places_checked_in` gợi ý check-in địa điểm, nhưng check-in qua `POST /contexts/{id}/checkins` không được đếm ở đó mà đếm vào `memories`.
