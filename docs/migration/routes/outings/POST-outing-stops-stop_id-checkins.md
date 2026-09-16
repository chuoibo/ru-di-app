# POST /outing-stops/{stop_id}/checkins

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

F46 — «đã tới». Ghi lại **ai** bấm và **lúc nào**, cả hai đều là thứ server đã biết. Route cố ý **không khai thân**: một thân là chỗ cho một toạ độ đi vào, và bảng `outing_stop_checkins` không có cột vị trí nào (`db/models.py:1327-1390`). Mỗi lượt tới làm `outings.timeline_revision` nhích lên, nên bản nháp lịch trình mà người khác đang mở thành cũ.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:123-140`, `services/api/app/api/service.py:3527-3555`):

1. Middleware idempotency: khoá rỗng → 422 trước 401 (`anonymous_empty_idempotency_key`).
2. Router: đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401 trước 422 của path (`anonymous_non_uuid_stop`); 422 `invalid_actor_id`, `invalid_actor_roles`.
4. Validation path `stop_id`. **Không có thân khai báo**, nên thân gửi kèm bị bỏ qua hoàn toàn — kể cả JSON hỏng (`owner_checks_in_with_a_body`, `mate_checks_in_with_broken_json`).
5. `repository.get_outing_stop(stop_id)` (`repository.py:3286-3295`) → None → 404 `stop_not_found` **`Stop does not exist`** (`service.py:3529-3530`; `owner_unknown_stop`). Hàm trả về **cặp** (chặng, chuyến), nên id chặng tự nêu nhóm của nó.
6. `_require_permission("check_in_to_stop", …, {"is_group_member": is_member(outing.context_id, actor.id)})` (`service.py:3532-3536`; `permissions.py:248-251`) → 403. `is_member` chỉ nhận `active`, nên người mới nhận link (membership `invited`) và người đã rời nhóm đều 403 (`invitee_checks_in`, `mate_checks_in_after_leaving`), và chặng của nhóm khác cũng 403 chứ không 404 (`owner_checks_in_at_another_groups_stop`, `stranger_checks_in_at_our_stop`).
7. `repository.create_stop_checkin` (`repository.py:3297-3342`):
   - `session.get(OutingStop, stop_id)` → None → `STOP_NOT_FOUND`;
   - `SELECT … FOR UPDATE` hàng `outings`, rồi kiểm lại chặng còn sống (người sửa lịch có thể vừa bỏ nó) → `STOP_NOT_FOUND` → 404 `stop_not_found` `Chặng đã được bỏ khỏi lịch trình.`;
   - `INSERT` trong `begin_nested()`; `IntegrityError` mang `constraint_name == "uq_outing_stop_checkins_person"` → `ALREADY_CHECKED_IN` → 409 `already_checked_in` `You have already checked in at this stop` (`owner_checks_in_at_coffee_again`). **Chỉ số là luật**: mã không hỏi trước rồi rẽ nhánh.
   - `outing.timeline_revision += 1`.

## Đầu vào

Path `stop_id`: UUID. Không query, **không thân**.

## Đầu ra

**201** `StopCheckinResponse` (`schemas.py:597-612`), thứ tự khoá: `id`, `stop_id`, `person_id`, `display_name`, `created_at`.

`display_name` đọc từ `get_person(record.person_id)` một lần cho mỗi hàng (`service.py:3576-3584`); `null` khi không có hàng người. `created_at` là `_now()` của service. Không có `lat`, `lng`, `accuracy` — không ở model, không ở bảng.

## Tác dụng phụ

Một hàng `outing_stop_checkins` (`stop_id`, `person_id` = actor, `created_at` = `now`) và `outings.timeline_revision + 1`, dưới khoá hàng `outings`. Hai lệnh, một giao dịch: UPDATE cha ra sau INSERT con vì con được thêm trong savepoint riêng rồi flush. Khi chặng bị bỏ khỏi lịch trình, hàng «đã tới» của nó bị xoá theo (`owner_reads_checkins_after_the_drop`). `idempotency_keys` khi có header và 2xx; 409/403/404 nhả khoá (`mate_key_frees_after_conflict` → `mate_reuses_the_freed_key_for_another_stop`).

## Idempotency

Không tự nhiên idempotent: lần thứ hai là 409, không phải 201 lặp lại. Khoá header lưu 201 và phát lại 201 (`mate_checks_in_at_dinner_with_key`, `mate_replays_key`), kể cả qua cửa kia (`crossreplay/POST-outing-stops-stop_id-checkins.yaml`). Ba lượt đồng thời dưới một khoá là một lần ghi và hai lần phát lại; ba lượt đồng thời không khoá là một 201 và hai 409 (`concurrency/POST-outing-stops-stop_id-checkins.yaml`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:148`, `:153-155` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","stop_id"]` | `main.py:319-351` |
| 404 | `stop_not_found` | `Stop does not exist` | `service.py:3529-3530` |
| 403 | `permission_denied` | `is_group_member` / `role_not_permitted` | `service.py:499-504` |
| 409 | `already_checked_in` | `You have already checked in at this stop` | `service.py:3545-3549` |
| 404 | `stop_not_found` | `Chặng đã được bỏ khỏi lịch trình.` (chỉ khi đua) | `service.py:3551-3553` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:123-140`
- Service: `services/api/app/api/service.py:3527-3555`, `:3576-3584`
- Repository: `services/api/app/api/repository.py:3286-3295`, `:3297-3342`
- Schema: `services/api/app/api/schemas.py:597-612`
- Quyền: `services/api/app/domain/permissions.py:248-251`
- Model DB: `services/api/app/db/models.py:1327-1390` (`uq_outing_stop_checkins_person`)

## Test đang phủ

- `services/api/tests/postgres/test_stop_checkins_postgres.py` (helper `_check_in` 78): `test_a_member_can_check_in_to_a_stop` (135), `test_the_same_person_cannot_check_in_twice` (147), `test_one_person_may_check_in_to_each_of_several_stops` (195), `test_two_members_may_check_in_to_the_same_stop` (209), `test_an_outsider_cannot_check_in` (229), `test_an_invited_member_cannot_check_in` (249), `test_a_departed_member_cannot_check_in` (282), `test_a_checkin_from_another_group_never_appears` (306), `test_no_response_field_carries_a_location` (362), `test_a_body_offering_coordinates_changes_nothing` (393), `test_an_unknown_stop_is_404_and_echoes_nothing` (420), `test_adding_a_stop_keeps_the_checkins_of_the_stops_that_did_not_change` (434), `test_reordering_the_timeline_carries_each_checkin_with_its_stop` (462), `test_a_stop_dropped_from_the_new_plan_takes_only_its_own_checkins` (485), `test_editing_a_stop_drops_the_checkins_of_the_stop_it_replaced` (507), `test_deleting_one_stop_takes_only_its_own_checkins` (532), `test_the_index_itself_refuses_a_duplicate_row` (175)
- `services/api/tests/postgres/test_outings_postgres.py`: `test_a_stop_keeps_its_catalogue_place_and_a_reattach_keeps_its_checkins` (869)

## Kịch bản parity

`parity/scenarios/w7/outings/POST-outing-stops-stop_id-checkins.yaml`, id `w7/outings/post-outing-stops-stop_id-checkins` (48 bước, `dev`):

- Cửa: `anonymous_checks_in`, `anonymous_non_uuid_stop`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid`, `owner_unknown_stop`.
- Quyền: `owner_checks_in_at_another_groups_stop`, `stranger_checks_in_at_our_stop`, `invitee_checks_in` (membership `invited`), `owner_checks_in_as_guest_role`, `mate_checks_in_after_leaving`.
- Ghi: `owner_reads_list_before_arriving` / `owner_checks_in_at_coffee` / `owner_reads_list_after_arriving` (chỉ `timeline_revision` đổi), `owner_checks_in_at_coffee_again` (409), `mate_checks_in_at_coffee`, `owner_checks_in_at_dinner`, `owner_reads_checkins`.
- Thân bị bỏ qua: `owner_checks_in_with_a_body`, `mate_checks_in_with_broken_json`.
- Khoá: `mate_checks_in_at_dinner_with_key`, `mate_replays_key`, `mate_same_key_other_stop`, `owner_same_key_own_scope`, `mate_key_frees_after_conflict`, `mate_reuses_the_freed_key_for_another_stop`.
- Chặng bị bỏ: `owner_drops_the_coffee_stop`, `owner_checks_in_at_the_dropped_stop` (404 `Stop does not exist`), `owner_reads_checkins_after_the_drop`.
- Framework: `trailing_slash`, `get_not_allowed`.

`crossreplay/POST-outing-stops-stop_id-checkins.yaml` (16 bước). `concurrency/POST-outing-stops-stop_id-checkins.yaml` (13 bước): `owner_arrives_three_times_at_once`, `mate_arrives_three_times_under_one_key`, `mate_arrives_three_times_under_three_keys`.

`prod-auth.yaml`: `anonymous_checks_in`, `owner_checks_in`, `stranger_checks_in_claiming_owner` (403 dù gửi `X-Actor-ID` của người khác).

Corpus sinh: `generated/w7-422/post-outing-stops-stop_id-checkins.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- 404 `Chặng đã được bỏ khỏi lịch trình.` **không tới được tuần tự**: cần một lần lưu lịch trình xen vào giữa `get_outing_stop` và `create_stop_checkin`. Bước đồng thời trong bộ không dựng được tình huống đó (mỗi bước đồng thời là một đường dẫn, một persona).
- Không phủ hai người **khác nhau** cùng bấm một lúc (bước `concurrent` là N bản sao của một request của một persona).
- Thân bị bỏ qua vì route không khai body param: bản Go phải **không** đọc và **không** từ chối thân, kể cả thân không phải JSON.
- Thứ tự `GET /outings/{id}/checkins` là `(created_at, id)`; hai lượt tới trong cùng một micro giây sẽ rơi vào id ngẫu nhiên. Không ghim đồng hồ trong kịch bản chính là cố ý.

## Lỗi Python (chỉ báo, không sửa)

- 404 của route có **hai câu khác nhau** cho cùng một mã: `Stop does not exist` (tiếng Anh, đường thường) và `Chặng đã được bỏ khỏi lịch trình.` (tiếng Việt, đường đua).
- Một lượt «đã tới» tăng `timeline_revision`, nên nó làm hỏng bản lưu lịch trình của người khác đang soạn dở — đúng ý về mặt đồng bộ, nhưng hai hành động rất khác nhau dùng chung một bộ đếm.
