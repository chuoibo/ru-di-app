# POST /bills

bills · core · trạng thái trong bộ nhớ: không có

(Thư mục là `bill/` chứ không phải `bills/`: repo guard chặn thành phần đường dẫn `bills`, `scripts/repo_guard.py:99-114`.)

## Mục đích

Lưu một bản nháp hoá đơn đã quét: các dòng món, người mà AI gợi ý cho từng món, phụ phí và giảm giá. Bản nháp **không** chạm sổ: không có `expenses`, không có `confirmed_allocations`; chia tiền là việc của `POST /bills/{bill_id}/split`, ghi sổ là việc của `POST /expenses/{expense_id}/confirm`. Người được gợi ý chỉ là gợi ý (`source = ai_suggested`), chưa ai quyết định (`services/api/app/api/routes/bills.py:32-43`).

## Xác thực và quyền

Thứ tự (theo mã, cùng khuôn đo ở W3):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`), trước cả định tuyến: `Idempotency-Key` rỗng hoặc dài hơn 255 → 422 `invalid_idempotency_key` (`:432-439`); khoá đã dùng cho request khác → 422 `idempotency_key_reuse` (`:473-480`); khoá đang chạy dở quá 5 s → 409 `idempotency_request_in_flight` (`:481-493`); khoá đã xong → phát lại nguyên văn kèm `idempotency-replayed: true` (`:494-502`, `:585-599`). Phạm vi khoá: digest bearer, nếu không thì `X-Actor-ID`, nếu không thì `anonymous` (`:447-449`). Chỉ 2xx được lưu; mọi status khác và mọi exception đều nhả khoá (`:522-546`).
2. Giải mã JSON của FastAPI → 422 thắng 401 (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu `X-Actor-ID` → 401 (`anonymous_creates`); không phải UUID → 422 `invalid_actor_id` (`actor_id_not_uuid`); role lạ → 422 `invalid_actor_roles` (`roles_unknown`). Ở `prod`: bearer (`deps.py:93-107`, `services/api/app/api/service.py:4452-4473`), `X-Actor-*` bị bỏ qua.
4. Validation body (pydantic) → 422 `{"detail":[…]}` đã bỏ `input` (handler `services/api/app/api/main.py:318-351`).
5. `_require_permission("confirm_expense_proposal", {"is_group_member": is_member(body.context_id, actor)})` (`service.py:5931-5939`; bảng `services/api/app/domain/permissions.py:49`): 403 `permission_denied` với detail `role_not_permitted` khi thiếu role `member` (`mate_roles_empty`), `is_group_member` khi không phải thành viên ACTIVE — nhóm không tồn tại (`stranger_unknown_context`), nhóm thật (`stranger_real_context`), người được mời chưa nhận (`invitee_creates`), người đã rời (`mate_creates_after_leaving`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header`). `is_member` đòi `state = active` và `left_at IS NULL` (`services/api/app/api/repository.py:2769-2782`).
6. Mọi `suggested_participant_ids` phải là thành viên ACTIVE (`service.py:5949-5956` → `_require_participants_are_members` `:6336-6371`) → 422 `participant_not_in_context`. Chạy **trước** kiểm tổng (`owner_non_member_and_total_mismatch`).
7. `items_total_vnd` phải bằng tổng `line_total_vnd` (`service.py:5971-5978`) → 422 `bill_items_total_mismatch`. Tổng tính bằng số nguyên Python không giới hạn; phụ phí và giảm giá nằm ngoài tổng (`owner_items_total_includes_surcharge`).
8. Ghi trong savepoint (`repository.py:5818-5892`): `IntegrityError` → `RepositoryConflict(code)` theo tên constraint (`repository.py:1134-1138`, mặc định `BILL_WRITE_CONFLICT`) → 409 (`service.py:6021-6022`).
9. Lỗi DB không phải `IntegrityError` (vd. số vượt bigint) không bị bắt → 500 `text/plain` `Internal Server Error` (`services/api/app/api/guest_privacy.py:65`, đăng ký ở `api/main.py:272`); transaction rollback (`deps.py:202-204`).

## Đầu vào

- Không path, không query.
- Body `BillCreateRequest` (`services/api/app/api/schemas.py:185-193`, `extra="forbid"`), thứ tự khai báo:
  - `context_id: UUID` (lax: chữ hoa, không gạch… được pydantic nhận);
  - `printed_total_vnd: int ≥ 0 | null` — **bắt buộc có khoá**, được `null` (`owner_nullable_fields_missing`);
  - `items_total_vnd: int ≥ 0` strict; `confidence: int 0..100` strict; `needs_review: bool` strict;
  - `items: list[BillItemCreateRequest]` (`:141-147`): `item_key` str ≤ 64 ký tự (không có min, chuỗi rỗng và khoảng trắng đầu/cuối qua được), `name` str, `quantity` int > 0, `unit_price_vnd: int | null` bắt buộc có khoá, **không có cận** (âm được), `line_total_vnd` int > 0 không có trần, `suggested_participant_ids: list[UUID]`;
  - `surcharges` mặc định `[]` (`:150-154`): `surcharge_key` ≤ 64, `kind` ≤ 32 ký tự (rỗng qua được), `amount_vnd` int > 0, `mode` `proportional|even`;
  - `discounts` mặc định `[]` (`:157-182`): `discount_key` ≤ 64, `amount_vnd` int > 0, `scope` `global_proportional|item`, `item_key` ≤ 64 hoặc null; model validator: `scope == "item"` ⇔ có `item_key` (`:172-182`, `owner_discount_scope_target_mismatch`). Không kiểm `item_key` có trùng dòng nào.
- Số tiền: strict int (`schemas.py:26-28`), `1.0`/`"0"`/`true` bị từ chối (`owner_wrong_types`); không có trần trên, nên số vượt int64 qua được validation.

## Đầu ra

- **201** `BillResponse` (`schemas.py:241-253`) qua `_wire_bill` (`service.py:731-792`), thứ tự khoá: `id`, `context_id`, `printed_total_vnd`, `items_total_vnd`, `needs_review`, `created_by_id`, `created_at`, `assignment_state`, `suggested_item_keys`, `items`, `surcharges`, `discounts`.
  - `items[]`: `item_key`, `name`, `quantity`, `unit_price_vnd`, `line_total_vnd`, `position`, `shares`; sắp theo `(position, item_key)` (`repository.py:2445-2451`); `position` là chỉ số trong mảng request (`service.py:5987-5999`).
  - `shares[]`: `participant_id`, `source`, `decided_by_id`, `decided_at`; trong mỗi món sắp theo `participant_id` kiểu `uuid` của Postgres (= thứ tự byte) (`repository.py:2456-2467`). Lúc tạo: `source = "ai_suggested"`, `decided_by_id = null`, `decided_at = null` (`repository.py:5847-5857`).
  - `surcharges[]`: `surcharge_key`, `kind`, `amount_vnd`, `mode`, sắp theo `surcharge_key` bằng `ORDER BY` của DB (`repository.py:2469-2475`); `discounts[]`: `discount_key`, `amount_vnd`, `scope`, `item_key` (từ cột `target_item_key`), sắp theo `discount_key` (`:2476-2482`). Thứ tự không theo request.
  - `suggested_item_keys`: các `item_key` có ít nhất một share `ai_suggested`, sắp theo **byte UTF-8** (`service.py:732-739`) — `"Nem"` đứng trước `"pho"` (`owner_creates_full_bill`); món không có gợi ý không có mặt.
  - `assignment_state`: `"confirmed"` chỉ khi có ít nhất một món và mọi món có share và mọi share `confirmed`; ngược lại `"ai_suggested"` (`service.py:740-752`). Bill không có dòng → `"ai_suggested"` (`owner_creates_bill_without_lines`).
  - `confidence` được lưu nhưng **không** lên wire.
  - `created_at`: `datetime.now(UTC)` của Python (`service.py:406-407`, `:6019`) gán vào ORM rồi đọc lại từ object, pydantic in ISO 8601 offset 0 dạng `Z`, phần lẻ tới micro giây.
  - Số tiền là số nguyên JSON; trần bigint `2**63 − 1` và `unit_price_vnd` `−2**63` lưu và trả nguyên (`owner_creates_bill_at_bigint_ceiling`).
- Framework: `GET /bills` → 405 `allow: POST` (`get_not_allowed`); `POST /bills/` → 307 (`trailing_slash_redirects`).

## Tác dụng phụ

- Một hàng `bills` (`context_id`, `created_by_id` = actor, `printed_total_vnd`, `items_total_vnd`, `confidence`, `needs_review`, `created_at`) (`repository.py:5820-5830`; CHECK `services/api/app/db/models.py:107-112`).
- `bill_items` mỗi dòng một hàng (UNIQUE `(bill_id, item_key)`, CHECK `line_total_vnd > 0`, `quantity > 0`; `models.py:139-141`), `bill_item_shares` mỗi gợi ý một hàng (UNIQUE `(bill_item_id, participant_id)`, CHECK nguồn/quyết định `models.py:233-244`), `bill_surcharges` (UNIQUE `(bill_id, surcharge_key)` `models.py:166-171`), `bill_discounts` (UNIQUE `(bill_id, discount_key)`, CHECK scope/target `models.py:196-206`).
- Tất cả trong `session.begin_nested()`; lỗi ràng buộc huỷ savepoint, rồi `ApiProblem` làm cả request rollback — refusal không để lại hàng.
- Không khoá hàng, không audit event, không chạm sổ.
- Có header `Idempotency-Key`: một hàng `idempotency_keys` (2xx: kèm status, body, media type).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) · `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | `X-Actor-ID must be a UUID` / `X-Actor-Roles contains an unknown role` / `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:144-163` |
| 422 | (không có `code`) | `{"detail":[{type,loc,msg,…}]}` của pydantic, bỏ `input` | `api/main.py:318-351` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5931-5939`, `:504`; `permissions.py:673-677` |
| 422 | `participant_not_in_context` | `Not members of this group: ` + các id (chữ thường, có gạch) sắp theo byte, nối bằng `, `, mỗi id một lần | `service.py:6358-6371` |
| 422 | `bill_items_total_mismatch` | `Declared items total {items_total_vnd} does not match the sum of the lines {tổng}` (số nguyên thập phân đầy đủ) | `service.py:5971-5978` |
| 409 | `DUPLICATE_BILL_ITEM_KEY` / `DUPLICATE_BILL_SURCHARGE_KEY` / `DUPLICATE_BILL_DISCOUNT_KEY` / `BILL_WRITE_CONFLICT` (viết hoa) | `Bill creation conflicted` | `repository.py:1134-1138`, `:5879-5892`; `service.py:6021-6022` |
| 500 | — | `Internal Server Error` (`text/plain`) khi giá trị vượt bigint | `repository.py:5818-5892` (chỉ bắt `IntegrityError`); `guest_privacy.py:65` |

## Mã Python

- Route: `services/api/app/api/routes/bills.py:32-43`
- Service: `services/api/app/api/service.py:5930-6023` (`create_bill`), `:6336-6371` (`_require_participants_are_members`), `:731-792` (`_wire_bill`), `:475-504` (`_require_permission`)
- Repository: `services/api/app/api/repository.py:5804-5893` (`create_bill`), `:2444-2523` (`_bill_record`), `:2435-2442` (`_bill_share_record`), `:1134-1138` (`_BILL_WRITE_CONFLICTS`), `:2749-2782` (`list_members`, `is_member`)
- Schema: `services/api/app/api/schemas.py:141-193`, `:210-253`
- Model: `services/api/app/db/models.py:102-262`

## Test đang phủ

- `services/api/tests/api/test_bills.py`: `test_an_ai_assignment_is_stored_as_a_suggestion_not_a_decision` (122), `test_a_fresh_draft_reports_itself_as_unconfirmed` (134), `test_the_confidence_score_stays_off_the_wire` (137), `test_a_draft_creates_no_obligation` (154), `test_an_outsider_cannot_create_a_bill_in_someone_elses_group` (387) — repository giả
- `services/api/tests/api/test_bill_items_total_matches_lines.py`: 49, 64, 80, 96, 106, 136, 163, 180
- `services/api/tests/api/test_bills_discount_scope.py`: 46, 63, 79; `test_bills_duplicate_item_key.py`: 56, 64; `test_bills_surcharges.py::test_a_bill_that_kept_its_surcharges_can_be_read_back` (175)
- Postgres thật: `tests/postgres/test_bill_duplicate_item_key_postgres.py` (39, 64, 104, 137), `test_bill_items_total_matches_lines_postgres.py` (150, 165, 181), `test_bill_surcharges_postgres.py` (100, 135, 217)
- Không tìm thấy test HTTP nào gửi `suggested_participant_ids` là người ngoài nhóm (các test thành viên trong `test_bill_participants_must_be_members.py` đi qua `PUT …/assignments`), cũng không có test cho số vượt bigint.

## Kịch bản parity

`parity/scenarios/w4/bill/POST-bills.yaml`, id `w4/bill/post-bills` (47 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_creates` (401), `anonymous_malformed_json` (422 JSON), `actor_id_not_uuid`, `roles_unknown`.
- 403: `stranger_unknown_context`, `stranger_real_context`, `stranger_claims_context_header`, `invitee_creates`, `mate_roles_empty`, `mate_creates_after_leaving`.
- 422 thành viên: `owner_suggests_non_members` (người được mời, người lạ lặp hai lần, id literal), `owner_non_member_and_total_mismatch` (thành viên trước tổng), `owner_suggests_the_leaver`.
- 422 tổng: `owner_items_total_above_lines`, `owner_items_total_includes_surcharge`, `owner_line_sum_past_int64` (hai dòng `2**63 − 1`, detail in tổng 20 chữ số).
- 201: `owner_creates_full_bill` (ba dòng, một dòng không gợi ý, hai phụ phí, hai giảm giá; thứ tự request khác thứ tự trả), `owner_creates_bill_without_lines`, `owner_creates_bill_at_bigint_ceiling`, `mate_creates_bill` (thành viên không phải admin).
- 409: `owner_duplicate_item_key`, `owner_duplicate_surcharge_key`, `owner_duplicate_discount_key`, `owner_duplicate_suggested_participant` (`BILL_WRITE_CONFLICT`).
- 500: `owner_items_total_past_bigint` (dòng `2**63 − 1` và `1`, `items_total_vnd` `2**63`).
- Pydantic: `owner_discount_scope_target_mismatch`, `owner_field_bounds`, `owner_nullable_fields_missing`, `owner_wrong_types`.
- Idempotency: `owner_key_first`, `owner_key_replay_reordered`, `owner_key_reused_other_body`, `owner_key_refused_mismatch` + `owner_key_after_mismatch` (422 nhả khoá), `owner_key_refused_conflict` + `owner_key_after_conflict` (409 nhả khoá).
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307).

`prod` (`w4/bill/prod-auth`, 24 bước): `anonymous_create_missing_bearer` (401), `owner_creates_bill_empty_roles_header` (201, header bị bỏ qua), `stranger_creates_in_group` (403).

Replay chéo `parity/scenarios/w4/crossreplay/POST-bills.yaml` (`w4/crossreplay/post-bills`, 17 bước): Python lưu → Go phát lại (kể cả đổi thứ tự khoá), Go lưu → Python phát lại; 403, 422 dịch vụ và 409 ràng buộc đều nhả khoá ở phía kia.

Đồng thời `parity/scenarios/w4/concurrency/POST-bills.yaml` (`w4/concurrency/post-bills`, 5 bước): `same_key_burst` (3 bản một khoá), `distinct_key_burst` (3 khoá, bill **không có dòng**), `same_key_other_body_burst`.

Corpus 422 sinh tự động: **hoãn** — bộ sinh từ chối với lý do `carries ['default_factory', 'default_factory_takes_data']` (`surcharges`/`discounts` dùng `default_factory`), `scripts/render_parity_422_scenarios.py` wave `w4`.

## Chưa phủ / lưu ý cho bản Go

- **Tổng dòng vượt int64**: Go phải cộng `line_total_vnd` bằng số không tràn (`big.Int`) và in đủ chữ số trong detail (`owner_line_sum_past_int64`); JSON decoder của Go không được từ chối số > int64 trước service bằng một 422 khác.
- **Vượt bigint mà khớp tổng** → Python 500 `text/plain`. Bản Go phải cho ra đúng 500 đó (parity giữ lỗi), không được "sửa" thành 422 trong PR port.
- Ánh xạ tên constraint → code phải đọc `ConstraintName` của lỗi Postgres; constraint lạ là `BILL_WRITE_CONFLICT`; code 409 **viết hoa**, khác hầu hết code khác.
- Thứ tự `surcharges`/`discounts` do `ORDER BY` varchar theo collation của DB; bản Go phải sắp trong SQL như Python, không sắp trong Go.
- `suggested_item_keys` sắp theo byte UTF-8 trong ứng dụng, còn `items` theo `position`.
- Không kiểm: `unit_price_vnd` âm hay lệch `quantity × unit_price`; `item_key` rỗng/có khoảng trắng; `kind` rỗng; giảm giá `item` trỏ tới dòng không tồn tại — tất cả được lưu và chỉ bị allocator từ chối ở `split`.
- **Khoảng trống harness (làn DB)**: trong một bước, các hàng mới cùng bảng giống hệt nhau sau khi che uuid4/thời điểm được sắp theo id ngẫu nhiên nên đánh số khác nhau giữa hai stack. Vì vậy kịch bản không có bước nào gợi ý **một người trên hai món** trong một lần tạo (hai hàng `bill_item_shares` chỉ khác `bill_item_id`), và loạt khoá khác nhau chỉ tạo bill không dòng. Hai trường hợp đó chưa được so.
- Không có hàng `audit_events`; không khoá; `confidence` chỉ nằm trong DB.
