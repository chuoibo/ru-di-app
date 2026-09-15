# GET /bills/{bill_id}

bills · core · trạng thái trong bộ nhớ: không có

(Thư mục là `bill/` chứ không phải `bills/`: repo guard chặn thành phần đường dẫn `bills`, `scripts/repo_guard.py:99-114`.)

## Mục đích

Đọc một bản nháp hoá đơn với các dòng, người được gợi ý hoặc đã được gán cho từng món, phụ phí và giảm giá. `assignment_state` và `suggested_item_keys` được **tính lại** mỗi lần đọc từ các hàng share, không có cột trạng thái (`services/api/app/api/routes/bills.py:46-56`).

## Xác thực và quyền

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_bill`); `X-Actor-ID` không phải UUID → 422 `invalid_actor_id` (`actor_id_not_uuid`).
2. Path `bill_id: UUID` → 422 (`stranger_non_uuid_bill`).
3. `_bill_for_actor` (`services/api/app/api/service.py:5919-5928`): `repository.get_bill` → không có → **404 `bill_not_found` trước mọi kiểm quyền** (`stranger_unknown_bill`).
4. `_require_permission("confirm_expense_proposal", {"is_group_member": is_member(bill.context_id, actor)})` (`service.py:5923-5927`; `services/api/app/domain/permissions.py:49`): 403 `is_group_member` cho người lạ (`stranger_real_bill`), người được mời (`invitee_reads`), người đã rời (`mate_reads_after_leaving`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` cũng là `role_not_permitted` vì hành động chỉ nhận `member` (`mate_reads_with_group_admin_role_only`).

## Đầu vào

- Path `bill_id` (UUID lax).
- Không body, không query (`undeclared_query_ignored` → 200).

## Đầu ra

- **200** `BillResponse` (`services/api/app/api/schemas.py:241-253`) qua `_wire_bill` (`service.py:731-792`), cùng hình như 201 của `POST /bills` (xem card đó cho thứ tự khoá và thứ tự mảng):
  - `items` theo `(position, item_key)`, `shares` trong món theo `participant_id`, `surcharges` theo `surcharge_key`, `discounts` theo `discount_key` (`services/api/app/api/repository.py:2444-2523`).
  - Share `ai_suggested`: `decided_by_id`/`decided_at` null; share `confirmed`: người quyết định và thời điểm (`owner_reads_confirmed_bill`).
  - `assignment_state` (`service.py:740-752`): `"confirmed"` khi mọi món có share và mọi share `confirmed` (`owner_reads_confirmed_bill`); một món không còn share nào kéo về `"ai_suggested"` dù không có gợi ý nào, và `suggested_item_keys` rỗng (`mate_reads_line_without_shares`); bill không dòng → `"ai_suggested"` (`owner_reads_bill_without_lines`).
  - `suggested_item_keys` sắp theo byte UTF-8 (`service.py:732-739`).
  - `created_at`, `decided_at`: đọc từ DB (`timestamptz`, session UTC), pydantic in ISO 8601 offset 0 `Z`. Hàng được ghi trong cùng request ghi mang giá trị Python; ở GET là giá trị Postgres trả về — cùng độ chính xác micro giây.
  - Số nguyên JSON; trần bigint đọc lại nguyên (`mate_reads_bill_at_bigint_ceiling`, `quantity` `2**31 − 1` là trần `integer`).
- Share vẫn nêu tên người đã rời nhóm (`owner_reads_after_mate_left`).
- Framework: 307 cho `/` cuối (`trailing_slash_redirects`); `POST /bills/{id}` → 405 `allow: GET` (`post_not_allowed`).

## Tác dụng phụ

- Không ghi, không khoá: `session.get(Bill, bill_id)` (`repository.py:5895-5897`) rồi các SELECT của `_bill_record`.
- Bốn truy vấn thêm (dòng, share, phụ phí, giảm giá) (`repository.py:2445-2482`), cộng `is_member` (`repository.py:2769-2782`).
- Delta DB rỗng; không idempotency (GET); không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` · prod: `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /bills` | `deps.py:144-163` |
| 422 | (không có `code`) | `{"detail":[…]}` của FastAPI cho path | `services/api/app/api/main.py:318-351` |
| 404 | `bill_not_found` | `Bill does not exist` | `service.py:5920-5922` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5923-5927`, `:504` |

## Mã Python

- Route: `services/api/app/api/routes/bills.py:46-56`
- Service: `services/api/app/api/service.py:6025-6026` (`get_bill`), `:5919-5928` (`_bill_for_actor`), `:731-792` (`_wire_bill`)
- Repository: `services/api/app/api/repository.py:5895-5897`, `:2435-2523`, `:2769-2782`
- Schema: `services/api/app/api/schemas.py:210-253`

## Test đang phủ

- `services/api/tests/api/test_bills.py`: `test_an_outsider_cannot_read_the_dishes_of_another_group` (394), `test_a_bill_that_does_not_exist_is_a_404` (429) — repository giả
- `services/api/tests/api/test_bills_surcharges.py::test_a_bill_that_kept_its_surcharges_can_be_read_back` (175); `tests/api/test_bill_participants_must_be_members.py::test_the_refusal_happens_before_the_share_is_stored` (67) đọc lại qua GET
- Postgres thật: `tests/postgres/test_bill_surcharges_postgres.py::test_the_surcharge_lines_read_back_with_their_kind_and_mode` (100), `test_an_item_scoped_discount_keeps_the_item_it_points_at` (135)

## Kịch bản parity

`parity/scenarios/w4/bill/GET-bills-bill_id.yaml`, id `w4/bill/get-bills-bill_id` (36 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_bill`, `anonymous_non_uuid_bill`, `actor_id_not_uuid`, `stranger_non_uuid_bill`, `stranger_unknown_bill` (404 trước 403).
- 403: `stranger_real_bill`, `stranger_claims_context_header`, `invitee_reads`, `mate_roles_empty`, `mate_reads_with_group_admin_role_only`, `mate_reads_after_leaving`.
- 200: `owner_reads_suggested_bill`, `owner_reads_bill_without_lines`, `mate_reads_bill_at_bigint_ceiling`, `owner_reads_confirmed_bill` (sau `mate_claims_nem`, `owner_assigns_pho`, `owner_assigns_tra`), `mate_reads_line_without_shares` (sau `owner_clears_tra`), `owner_reads_after_mate_left`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects` (307), `post_not_allowed` (405).

`prod` (`w4/bill/prod-auth`): `junk_bearer_reads` (401), `stranger_unknown_bill` (404), `stranger_reads_claiming_owner` (403, `X-Actor-*` bị bỏ qua), `owner_lowercase_scheme_reads` (200 với `bearer` chữ thường), `mate_reads_after_leaving` (403), `owner_after_revoke` (401).

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/get-bills-bill_id.yaml`, id `generated/w4-422/get-bills-bill_id` (18 bước: biến thể UUID của path, input không khai báo, thứ tự actor trước path).

## Chưa phủ / lưu ý cho bản Go

- **404 trước 403**: bất kỳ ai có phiên (hoặc header dev) cũng biết một `bill_id` có tồn tại hay không. Parity giữ nguyên hành vi này.
- `assignment_state` có hai định nghĩa trong Python: `_wire_bill` coi món không share là chưa xác nhận, còn phép chiếu của `split` coi đó là lỗi `ITEM_HAS_NO_ASSIGNEE`; bản Go phải giữ cả hai đúng chỗ.
- Thứ tự `surcharges`/`discounts` theo `ORDER BY` của DB (collation), `suggested_item_keys` theo byte UTF-8 trong ứng dụng.
- Dạng thời điểm: pydantic in `Z` cho offset 0 và bỏ phần lẻ khi micro giây bằng 0; bản Go phải in cùng dạng cho cả giá trị vừa ghi lẫn giá trị đọc từ DB.
- Không khoá khi đọc, nên một GET song song với `PUT …/assignments` có thể thấy trạng thái trước hoặc sau; harness chạy tuần tự nên không đo.
