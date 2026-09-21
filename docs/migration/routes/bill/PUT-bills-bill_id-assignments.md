# PUT /bills/{bill_id}/assignments

bills · core · trạng thái trong bộ nhớ: không có

(Thư mục là `bill/` chứ không phải `bills/`: repo guard chặn thành phần đường dẫn `bills`, `scripts/repo_guard.py:99-114`.)

## Mục đích

Một thành viên quyết định ai ăn từng món được nêu: mọi share trên các món có trong body bị xoá và ghi lại thành `confirmed`, người quyết định là người gọi. Món không được nêu giữ nguyên share (gợi ý hay đã xác nhận). Vẫn là bản nháp: không chạm sổ (`services/api/app/api/routes/bills.py:59-70`).

## Xác thực và quyền

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`): khoá rỗng/quá dài 422, dùng lại cho body khác 422, phát lại 2xx đã lưu; refusal nhả khoá. Chi tiết: card `POST /bills`.
2. Giải mã JSON → 422 trước actor (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401 (`anonymous_unknown_bill`), 422 `invalid_actor_id` (`actor_id_not_uuid`).
4. Path + body (pydantic) → 422 (`stranger_non_uuid_bill`, `owner_invalid_assignment`, `owner_assignments_missing`).
5. `_bill_for_actor` (`services/api/app/api/service.py:5919-5928`): 404 `bill_not_found` **trước** quyền và trước kiểm thành viên của body (`stranger_unknown_bill_naming_stranger`); rồi 403 `confirm_expense_proposal` / `is_group_member` (`stranger_real_bill`, `invitee_assigns`, `third_assigns_after_leaving`) hoặc `role_not_permitted` (`mate_roles_empty`).
6. Mọi `participant_ids` phải là thành viên ACTIVE (`service.py:6041-6048` → `:6336-6371`) → 422 `participant_not_in_context` (`owner_assigns_non_members`, `owner_assigns_the_leaver`), chạy **trước** kiểm món (`owner_non_member_on_unknown_item`).
7. Repository (`services/api/app/api/repository.py:5899-5949`): khoá `bills` FOR UPDATE (`:5907-5911`; không thấy → `BILL_NOT_FOUND` → 404, không tới được qua HTTP vì bước 5), khoá mọi `bill_items` của bill FOR UPDATE (`:5913-5918`), `item_key` lạ → `UNKNOWN_BILL_ITEM` → **409** (`:5922-5923`; `service.py:6062-6065`) trước khi xoá gì.
8. Xoá share của các món được nêu (khoá FOR UPDATE rồi DELETE, `:5925-5934`), chèn share `confirmed` (`:5936-5948`). Một người lặp hai lần trong cùng một món vi phạm `uq_bill_item_shares_item_participant` ở flush; không có nhánh nào bắt `IntegrityError` → **500** (`owner_duplicate_participant_on_item`), transaction rollback (`deps.py:202-204`, `owner_reads_after_500`).

## Đầu vào

- Path `bill_id` (UUID lax).
- Body `BillAssignmentsRequest` (`services/api/app/api/schemas.py:201-202`): `assignments: list[BillAssignment]` bắt buộc; `BillAssignment` (`:196-198`): `item_key` str ≤ 64 ký tự, `participant_ids: list[UUID]`; `extra="forbid"`.
- `item_key` so chính xác, phân biệt hoa thường (`owner_assigns_item_key_other_case` → 409).
- Cùng `item_key` hai lần: dict theo khoá, **mục sau thắng** (`repository.py:5919-5921`, `owner_same_item_twice_last_wins`).
- `participant_ids: []` xoá hết share của món đó (`owner_clears_item`); `assignments: []` không đổi gì và vẫn 200 (`owner_assigns_nothing`, `owner_assigns_nothing_on_bill_without_lines`).

## Đầu ra

- **200** `BillResponse` (`schemas.py:241-253`) qua `_wire_bill` (`service.py:731-792`); hình, thứ tự khoá và thứ tự mảng như card `POST /bills`. Bản ghi được đọc lại sau flush (`repository.py:5949`).
- Share mới: `source: "confirmed"`, `decided_by_id` = người gọi, `decided_at` = `datetime.now(UTC)` của request (`service.py:6059-6060`).
- `assignment_state` chuyển `"confirmed"` khi món cuối cùng có share xác nhận (`mate_confirms_last_item`); xoá hết share một món kéo về `"ai_suggested"` (`owner_clears_item`).
- Gợi ý của AI trên món được nêu biến mất kể cả khi người được gợi ý không có trong danh sách mới (`owner_overrules_the_suggestion`).
- Phát lại idempotency trả **trạng thái lúc lưu**, dù share đã đổi sau đó (`owner_changes_tra_without_key` rồi `owner_key_replay_reordered`).
- Framework: `GET /bills/{id}/assignments` → 405 `allow: PUT` (`get_not_allowed`).

## Tác dụng phụ

- Khoá hàng `bills`, mọi `bill_items` của bill, và share hiện có của các món được nêu, giữ tới commit trước response (`services/api/app/api/unit_of_work.py:41-48`). Hai lần gán cùng lúc trên một bill xếp hàng ở khoá `bills`.
- DELETE `bill_item_shares` của các món được nêu; INSERT `bill_item_shares` `confirmed`. Không đổi `bills`, `bill_items`, phụ phí, giảm giá.
- Không audit event, không chạm sổ. Có header `Idempotency-Key` → hàng `idempotency_keys`.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` · prod: `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /bills` | `deps.py:144-163` |
| 422 | (không có `code`) | `{"detail":[…]}` của pydantic, bỏ `input` | `services/api/app/api/main.py:318-351` |
| 422 / 409 | `invalid_idempotency_key` / `idempotency_key_reuse` / `idempotency_request_in_flight` | xem card `POST /bills` | `idempotency.py:432-493` |
| 404 | `bill_not_found` | `Bill does not exist` | `service.py:5920-5922` (và `:6063-6064`, không tới được) |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5923-5927`, `:504` |
| 422 | `participant_not_in_context` | `Not members of this group: ` + id sắp theo byte, nối `, ` | `service.py:6358-6371` |
| 409 | `UNKNOWN_BILL_ITEM` (viết hoa) | `Bill assignment conflicted` | `repository.py:5922-5923`; `service.py:6065` |
| 500 | — | `Internal Server Error` (`text/plain`) khi một người lặp trong một món | `repository.py:5936-5948`; `services/api/app/api/guest_privacy.py:65` |

## Mã Python

- Route: `services/api/app/api/routes/bills.py:59-70`
- Service: `services/api/app/api/service.py:6028-6066` (`confirm_bill_assignments`), `:5919-5928`, `:6336-6371`, `:731-792`
- Repository: `services/api/app/api/repository.py:5899-5949`, `:2444-2523`
- Schema: `services/api/app/api/schemas.py:196-202`, `:241-253`

## Test đang phủ

- `services/api/tests/api/test_bills.py`: `test_confirming_marks_the_shares_as_decided_and_names_the_decider` (164), `test_a_person_may_overrule_the_ai_about_who_ate_what` (178), `test_an_unmentioned_item_leaves_the_bill_unconfirmed` (203), `test_an_unknown_item_key_is_refused_rather_than_ignored` (217), `test_an_outsider_cannot_reassign_who_ate_what` (402) — repository giả
- `services/api/tests/api/test_bill_participants_must_be_members.py`: 58, 67, 86, 100
- Postgres thật: `tests/postgres/test_bill_surcharges_postgres.py::test_the_four_routes_split_a_vat_bill_to_the_printed_total_over_http` (217) đi qua route này
- Không có test cho người lặp trong một món (500) hay `item_key` lặp.

## Kịch bản parity

`parity/scenarios/w4/bill/PUT-bills-bill_id-assignments.yaml`, id `w4/bill/put-bills-bill_id-assignments` (47 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_bill`, `anonymous_malformed_json`, `actor_id_not_uuid`, `stranger_non_uuid_bill`, `stranger_unknown_bill_naming_stranger`.
- 403: `stranger_real_bill`, `invitee_assigns`, `mate_roles_empty`, `third_assigns_after_leaving`.
- 422 thành viên: `owner_assigns_non_members`, `owner_non_member_on_unknown_item`, `owner_assigns_the_leaver`.
- 409: `owner_assigns_unknown_item`, `owner_assigns_item_key_other_case`, `owner_assigns_on_bill_without_lines`.
- 200: `owner_assigns_nothing`, `owner_confirms_one_item`, `owner_overrules_the_suggestion`, `mate_confirms_last_item`, `owner_clears_item`, `owner_same_item_twice_last_wins`, `owner_assigns_nothing_on_bill_without_lines`.
- 500: `owner_duplicate_participant_on_item`, rồi `owner_reads_after_500`.
- Pydantic: `owner_invalid_assignment`, `owner_assignments_missing`.
- Idempotency: `owner_key_first`, `owner_changes_tra_without_key`, `owner_key_replay_reordered` (phát lại trạng thái cũ), `owner_key_reused_other_body`, `owner_key_refused_non_member` + `owner_key_after_refusal`.
- Framework: `get_not_allowed` (405).

`prod` (`w4/bill/prod-auth`): `stranger_assigns` (403), `owner_assigns_pho` (200).

Replay chéo `parity/scenarios/w4/crossreplay/PUT-bills-bill_id-assignments.yaml` (`w4/crossreplay/put-bills-bill_id-assignments`, 22 bước): phát lại trạng thái cũ sau khi share đã đổi, cùng khoá trên bill khác bị từ chối (`core_refuses_python_key_other_bill`), đổi thứ tự người/thứ tự assignment là request khác; 422 và 409 nhả khoá.

Đồng thời `parity/scenarios/w4/concurrency/PUT-bills-bill_id-assignments.yaml` (`w4/concurrency/put-bills-bill_id-assignments`, 9 bước): `same_key_burst`, `no_key_same_body_burst` (hai bản giống hệt xếp hàng ở khoá `bills`), `same_key_other_body_burst`.

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/put-bills-bill_id-assignments.yaml` (55 bước).

## Chưa phủ / lưu ý cho bản Go

- **500 khi một người lặp trong một món**: Python không bắt `IntegrityError` ở đây. Parity giữ 500; sửa thành 409/422 là phát hiện riêng.
- Món lạ ở route này là **409 `UNKNOWN_BILL_ITEM` viết hoa**, còn ở `POST …/my-items` là **422 `unknown_bill_item`**; bản Go giữ cả hai.
- Thứ tự khoá phải giữ: `bills` → `bill_items` → share của các món được nêu; không giữ thì hai lần gán song song có thể deadlock hoặc đọc share cũ.
- `item_key` lặp: mục sau thắng; thứ tự INSERT theo thứ tự dict Python (lần xuất hiện đầu tiên của khoá, giá trị của lần cuối).
- **Một người trên hai món trong một lần gán**: đã phủ bằng `PUT-bills-bill_id-assignments-many.yaml` (một người hai dòng, hai người hai dòng, mọi người mọi dòng, khoá lặp quanh dòng khác, 500 ở dòng thứ hai, xoá hai dòng).
- Không kiểm share mới có trùng với share của món khác; không kiểm người gọi có phải người tạo bill.
