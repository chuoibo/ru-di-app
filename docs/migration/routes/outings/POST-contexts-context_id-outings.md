# POST /contexts/{context_id}/outings

outings · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Mở một chuyến đi trên lịch của nhóm (F15): tên, ngày đi, ngày về, số người dự tính và ngân sách mỗi người. Chuyến sinh ra **rỗng**: chưa có chặng nào, `timeline_revision` 0, `itinerary_version` 1, `days` rỗng. Mọi thứ khác của chuyến — lịch trình, điểm danh, lời mời — đi qua id trả về ở đây.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/outings.py:81-93`, `services/api/app/api/service.py:2513-2534`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`): khoá rỗng hoặc dài quá 255 → 422 `invalid_idempotency_key` **trước mọi thứ**, kể cả 401 (`anonymous_empty_idempotency_key`); khoá đã xong phát lại 201 như đã lưu (`owner_replays_key`); cùng khoá thân khác → 422 `idempotency_key_reuse` (`owner_same_key_other_body`). Phạm vi khoá là người gọi, nên hai người dùng chung một chuỗi khoá không đụng nhau (`mate_same_key_own_scope`, `stranger_same_key_own_scope`).
2. Router: đuôi `/` → 307 (`trailing_slash`); `DELETE` → 405 (`delete_not_allowed`).
3. `get_actor` (`services/api/app/api/deps.py:110-165`) → 401 `Missing X-Actor-ID`, **trước** 422 của path (`anonymous_non_uuid_context`); 422 `invalid_actor_id` (`actor_id_not_uuid`), 422 `invalid_actor_roles` (`roles_unknown`).
4. Validation path `context_id` + thân `OutingCreateRequest` cùng một lần: path sai kèm thân sai trả **cả hai** mục `detail` (`owner_path_not_uuid_with_bad_body`).
5. `_require_permission("create_outing", …, {"is_group_member": repository.is_member(context_id, actor.id)})` (`service.py:2519-2523`; `services/api/app/domain/permissions.py:228-231`). Đây là **cửa duy nhất**: context không được đọc, nên context không tồn tại và nhóm mình không ở trong đều là một 403 `is_group_member` (`owner_creates_on_unknown_context`, `stranger_creates`, `mate_creates_after_leaving`), còn thiếu vai trò là 403 `role_not_permitted` (`owner_creates_as_guest_role`, `owner_creates_with_empty_roles`). `is_member` chỉ nhận membership `active` và `left_at IS NULL` (`repository.py:2769-2782`).
6. **Không có cửa `_require_group_kind`.** Thành viên của một pair mở được chuyến đi ngay trong cuộc trò chuyện hai người (`owner_creates_trip_on_pair` → 201). Cố ý: xem docstring `service.py:1477-1491` («Money, outings, votes and memories do not come through this method»).

## Đầu vào

- Path `context_id`: UUID.
- Thân `OutingCreateRequest` (`services/api/app/api/schemas.py:460-480`), `extra="forbid"`:
  - `title` `StrictStr` 1..200, `field_validator` cắt khoảng trắng hai đầu rồi từ chối chuỗi rỗng (`owner_title_blank`, `owner_title_is_trimmed` — tên lưu là bản đã cắt);
  - `starts_on`, `ends_on`: `date` (`owner_starts_on_not_a_date`, `owner_starts_on_datetime`);
  - `headcount`: int strict, `0 < n ≤ 1000` (`owner_headcount_zero`, `owner_headcount_over_ceiling`, `owner_headcount_string`, `owner_headcount_bool`);
  - `budget_per_person_vnd`: `NonNegativeMoneyVnd` = int strict ≥ 0 — không `float`, không chuỗi (`owner_budget_negative`, `owner_budget_float`, `owner_budget_zero` → 201);
  - `model_validator(mode="after")`: `ends_on < starts_on` → 422 (`owner_ends_before_starts`).

## Đầu ra

**201** `OutingResponse` (`schemas.py:619-633`, dựng ở `service.py:864-902`), thứ tự khoá:

`id`, `context_id`, `created_by_id`, `title`, `starts_on`, `ends_on`, `headcount`, `budget_per_person_vnd`, `created_at`, `stops` (`[]`), `timeline_revision` (`0`), `itinerary_version` (`1`), `days` (`[]`).

`starts_on`/`ends_on` là `date` thuần (`YYYY-MM-DD`, không múi giờ). `created_at` là `datetime` UTC lấy từ `_now()` (`service.py:406-407`, `datetime.now(UTC)`), kết thúc `Z`, có micro giây chỉ khi khác 0.

## Tác dụng phụ

Một hàng `outings` (`repository.py:2842-2866`): `context_id`, `created_by_id` = actor, `title`, `starts_on`, `ends_on`, `headcount`, `budget_per_person_vnd`, `created_at` = `now` do service truyền. `timeline_revision`, `itinerary_version`, `itinerary_days` **không được gán**, nên ba giá trị đó tới từ server default (`0`, `1`, `'[]'::jsonb`) và phải được đọc lại trước khi `_outing_record` dựng thân — hình dạng chính xác của câu INSERT (RETURNING hay SELECT sau đó) là việc của oracle repository, không phải của làn này. Không đụng bảng nào khác. `idempotency_keys` khi có header và 2xx; từ chối (403/422) nhả khoá.

## Idempotency

Không tự nhiên idempotent: hai lần gọi không khoá là hai chuyến. Khoá header lưu 201 và phát lại nguyên văn 201 (`Idempotency-Replayed: true`), kể cả qua cửa kia (`crossreplay/POST-contexts-context_id-outings.yaml`). 403 nhả khoá, nên cùng khoá mở được chuyến thật sau đó (`python_refuses_a_stranger` → `front_reuses_the_freed_key`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:148` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:153-155` |
| 422 | (validation) | `{"detail":[…]}` không kèm `input` | `main.py:319-351` |
| 403 | `permission_denied` | `is_group_member` hoặc `role_not_permitted` | `service.py:499-504`, `permissions.py:654-678` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-478` |
| 409 | `idempotency_request_in_flight` | | `idempotency.py:485-493` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/outings.py:81-93`
- Service: `services/api/app/api/service.py:2513-2534`, `:864-902` (`_wire_outing`), `:475-506` (`_require_permission`)
- Repository: `services/api/app/api/repository.py:2842-2866`, `:2324-2344` (`_outing_record`), `:2769-2782` (`is_member`)
- Schema: `services/api/app/api/schemas.py:460-480`, `:619-633`
- Quyền: `services/api/app/domain/permissions.py:228-231`
- Model DB: `services/api/app/db/models.py:1212-1270`

## Test đang phủ

- `services/api/tests/postgres/test_outings_postgres.py`: `test_a_member_creates_an_outing_and_reads_it_back` (178), `test_the_budget_crosses_the_wire_as_an_integer_of_dong` (216), `test_a_float_or_a_string_budget_is_refused_rather_than_coerced` (240), `test_the_budget_never_refuses_anything` (281), `test_an_outing_that_ends_before_it_starts_is_refused` (314), `test_a_stranger_can_neither_read_nor_create_outings` (353) — và `_make_outing` (159) là bước dựng của 15 ca khác trong tệp.
- `services/api/tests/postgres/test_outing_invite_source_must_match_roster.py`: fixture `standing` (102) gọi thẳng `service.create_outing`, 5 ca.
- Meta: `services/api/tests/api/test_idempotency.py::test_every_write_route_consults_the_store` (257); `tests/api/test_money_wire_type_gate.py::test_money_fields_refuse_non_integer_wire_values` (192).
- **Không có ca `tests/api/` nào (fake repository) lái route này qua HTTP** — mọi ca đều đòi marker `postgres`.

## Kịch bản parity

`parity/scenarios/w7/outings/POST-contexts-context_id-outings.yaml`, id `w7/outings/post-contexts-context_id-outings` (54 bước, `dev`):

- Trước khi có ai: `anonymous_creates`, `anonymous_non_uuid_context`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid_with_bad_body`.
- Một 403 cho mọi lý do: `owner_creates_on_unknown_context`, `stranger_creates`, `owner_creates_as_guest_role`, `owner_creates_with_empty_roles`, `mate_creates_after_leaving`.
- Thân: `owner_body_missing`, `owner_body_empty_object`, `owner_body_malformed_json`, `owner_body_not_an_object`, `owner_body_text_plain`, `owner_title_blank`, `owner_title_not_string`, `owner_ends_before_starts`, `owner_starts_on_not_a_date`, `owner_starts_on_datetime`, `owner_headcount_zero`, `owner_headcount_over_ceiling`, `owner_headcount_string`, `owner_headcount_bool`, `owner_budget_negative`, `owner_budget_float`, `owner_budget_zero`, `owner_extra_field`.
- Ghi: `owner_creates_trip`, `owner_creates_one_day_trip`, `owner_title_is_trimmed`, `mate_creates_trip`, `owner_creates_with_key`, `owner_replays_key`, `owner_same_key_other_body`, `mate_same_key_own_scope`, `stranger_same_key_own_scope`, `owner_lists_trips`.
- Pair: `owner_asks_mate` → `mate_accepts_friendship` → `owner_opens_pair` → `owner_creates_trip_on_pair` (201), `stranger_lists_pair_trips`.
- Rời nhóm: `mate_leaves_group`, `mate_creates_after_leaving`, `owner_lists_after_mate_left`.
- Framework: `trailing_slash`, `delete_not_allowed`.

`crossreplay/POST-contexts-context_id-outings.yaml` (12 bước). `prod-auth.yaml`: `anonymous_creates`, `owner_creates_trip_with_guest_roles_header` (201 — phiên mang `member`, header vai trò bị bỏ qua), `stranger_creates_claiming_owner` (403).

Corpus sinh: route này **hoãn** (`OutingCreateRequest` có `model_validator(mode="after")`), xem `scripts/render_parity_422_scenarios.py` mục `"w7"`.

## Chưa phủ / lưu ý cho bản Go

- Không phủ `headcount = 1000` (đúng biên trên) và `title` đúng 200/201 ký tự.
- Không phủ 409 `idempotency_request_in_flight` (đòi hai request chồng nhau trong 5 giây trên cùng một khoá; bước `concurrent` cho route này không nằm trong bộ).
- Chuyến tạo trên một context đã bị xoá mềm: không có đường HTTP nào xoá context, nên không đo được.
- Cổng duy nhất là `is_member`; **đừng** thêm kiểm tra context tồn tại trong bản Go, 403-thay-vì-404 ở đây là hợp đồng.
- `created_at` do service quyết (`_now()`), không phải `func.now()` của DB, dù cột có server default.

## Lỗi Python (chỉ báo, không sửa)

- Không có cửa `_require_group_kind`: mở được chuyến đi trong một cuộc trò chuyện hai người, và lời mời trên chuyến đó sau này lại bị 409 `not_a_group` — hai route nói hai điều khác nhau về cùng một chuyến.
- 403 trả nguyên tên predicate (`is_group_member`) làm `detail`. Đó là chuỗi nội bộ của bảng quyền, không phải câu cho người đọc.
