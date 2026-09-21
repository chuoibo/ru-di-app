# POST /obligations/{obligation_id}/confirm-receipt

obligations · core · trạng thái trong bộ nhớ: không có

## Mục đích

Người nhận của đúng một nghĩa vụ ghi rằng tiền đã tới, kèm số tiền. Đây là sự kiện tài chính duy nhất làm trạng thái nghĩa vụ đổi (`outstanding → partially_confirmed → confirmed → over_confirmed`) và làm số dư nhóm giảm. Không phải bằng chứng ngân hàng: là một người bấm nút. Sự kiện append-only; không có sửa hay xoá.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`): header rỗng → 422 `invalid_idempotency_key` (`owner_empty_idempotency_key`); cùng khoá, thân khác → 422 `idempotency_key_reuse` (`owner_header_key_other_body`); cùng khoá, cùng thân (sau chuẩn hoá JSON) → phát lại 201 kèm `idempotency-replayed: true` (`owner_header_key_replay_reordered`).
2. JSON hỏng → 422 trước actor (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 (`anonymous_unknown_obligation`), 422 `invalid_actor_id` (`actor_id_not_uuid`), 422 `invalid_actor_roles` (`roles_unknown`).
4. Validation path và thân `ReceiptConfirmationRequest` (`services/api/app/api/schemas.py:1979-1982`) **trước khi tra nghĩa vụ**: `stranger_non_uuid_obligation`, `stranger_zero_amount_unknown_obligation`, `owner_amount_float`, `owner_amount_string`, `owner_amount_negative`, `owner_body_key_not_uuid`, `owner_body_key_missing`, `owner_extra_field`.
5. `get_receipt_target` khoá `collection_obligations` `FOR UPDATE` (`services/api/app/api/repository.py:6957-6969`); không có → 404 `obligation_not_found` **trước kiểm quyền** (`services/api/app/api/service.py:6965-6967`; `stranger_unknown_obligation`, và cả `mate_roles_empty_unknown_obligation` với role rỗng).
6. `_require_permission("confirm_receipt", {"is_recipient_of_this_obligation": actor.id == recipient_id})` (`service.py:6968-6972`; `services/api/app/domain/permissions.py:101-104`): role `recipient` bắt buộc (`mate_roles_member_only` → `role_not_permitted`); không phải người nhận → 403 `is_recipient_of_this_obligation` (`stranger_confirms`, `third_confirms_owners_obligation`, `owner_confirms_as_sender`). **Không kiểm membership**: người đã rời nhóm vẫn xác nhận được (`mate_confirms_after_leaving`). Ở prod mọi phiên mang `recipient` (`repository.py:3629`).

## Đầu vào

- Path `obligation_id` (UUID lax).
- Thân (`extra="forbid"`): `amount_vnd: int` strict `> 0` **không có trần**, `idempotency_key: UUID` bắt buộc, `payment_report_id: UUID | null` (mặc định `null`).
- Header tuỳ chọn `Idempotency-Key` (lớp idempotency thứ hai, độc lập với khoá trong thân).

## Đầu ra

- **201** `ReceiptConfirmationResponse` (`schemas.py:1985-1991`), thứ tự khoá `receipt_confirmation_id`, `obligation_id`, `amount_vnd`, `obligation_status`.
  - `amount_vnd` là số tiền của dòng được ghi (hoặc dòng đã có).
  - `obligation_status` = `obligation_status(amount_vnd nghĩa vụ, mọi receipts xếp confirmed_at, id)` (`services/api/app/domain/ledger.py:163-216`, `repository.py:7032-7039`), cộng số nguyên Python không giới hạn. Không bao giờ ra `outstanding` qua route này (luôn có ít nhất một receipt > 0).
- **Khoá trong thân đã có** (`repository.py:6981-6999`): cùng `obligation_id`, `confirmed_by_id`, `amount_vnd`, `payment_report_id` → trả lại dòng cũ với **201** (không phải 200), không ghi thêm, `obligation_status` **tính lại theo receipts hiện tại** (`owner_same_body_key_again`); khác bất kỳ trường nào → 409 `idempotency_key_reused` (`owner_same_body_key_other_amount`, `owner_same_body_key_other_obligation`, `owner_same_body_key_with_report`, và `mate_reuses_owners_body_key`: khoá là duy nhất **toàn bảng**, người nhận khác cũng thấy khoá đã có).
- Giá trị trong kịch bản (dữ liệu mẫu, suy từ mã): hai nghĩa vụ 25 001 và 25 000 về owner, một nghĩa vụ 20 000 về mate. Nhận 10 000 → `partially_confirmed` (`owner_confirms_partial`); thêm 20 000 → `over_confirmed` với cả hai số (`owner_confirms_past_amount`); `mate_same_key_after_failed_insert` nhận đúng 20 000 → `confirmed`; một lần `2**63 - 1` trên nghĩa vụ thứ hai → 201 `over_confirmed` (`owner_confirms_int64_max`).
- Framework: `GET` → 405 (`get_not_allowed`).

## Tác dụng phụ

- **Khoá dòng** `collection_obligations` `FOR UPDATE` tới commit: các lần xác nhận cùng nghĩa vụ chạy tuần tự (`w4/concurrency/post-obligations-obligation_id-confirm-receipt`).
- Ghi (`repository.py:7004-7024`), cùng `now`:
  - `receipt_confirmations`: `obligation_id`, `payment_report_id`, `confirmed_by_id`, `amount_vnd`, `idempotency_key` (`UNIQUE`, `services/api/app/db/models.py:797-799`), `confirmed_at`; `CHECK amount_vnd > 0` (`models.py:780`); FK tổng hợp `(payment_report_id, obligation_id)` → `payment_reports` (`models.py:775-779`).
  - `audit_events`: `receipt_confirmed`, `aggregate_type='collection_obligation'`, `aggregate_id=obligation_id`, `request_id = idempotency_key`, `event_data = {"receipt_confirmation_id"}`.
  - `idempotency_keys` khi có header.
- Hai bảng đầu append-only (trigger, `services/api/app/db/migrations/versions/20260827_0001_initial_api_schema.py:666-688`).
- **Số tiền không có trần**: cột `BIGINT`. `2**63 - 1` ghi được. `2**63` với khoá mới → INSERT lỗi `bigint out of range` ở flush (không phải `RepositoryConflict`) → **500** `text/plain` `Internal Server Error` (`services/api/app/api/guest_privacy.py:65`), rollback, không dòng nào (`mate_amount_past_int64`); cùng khoá sau đó ghi bình thường (`mate_same_key_after_failed_insert`). `2**63` với khoá đã có → so sánh trong Python rồi 409, không chạm cột (`owner_int64_key_reused_past_int64`).
- View `collection_obligation_progress` ép tổng đã nhận **của một nghĩa vụ** về `::bigint` (`20260827_0001_initial_api_schema.py:621-646`): khi tổng một nghĩa vụ vượt `2**63 - 1`, mọi SELECT trên view lỗi SQLSTATE 22003. Route không đọc view; làn DB của harness thì đọc, nên kịch bản chỉ cho một lần nhận lớn trên mỗi nghĩa vụ.
- Từ chối (401/403/404/409/422): không ghi; header idempotency được nhả.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | `X-Actor-ID must be a UUID` / `X-Actor-Roles contains an unknown role` / `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:144-163` |
| 422 | (validation) | `{"detail":[…]}` không có `input` | `services/api/app/api/main.py:318-351` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |
| 404 | `obligation_not_found` | `Obligation does not exist` | `service.py:6966-6967` |
| 403 | `permission_denied` | `role_not_permitted` / `is_recipient_of_this_obligation` | `service.py:6968-6972`, `:504` |
| 409 | `idempotency_key_reused` | `Receipt confirmation conflicted` | `service.py:6982-6985`; `repository.py:6986-6993` |
| 409 | `payment_report_not_for_obligation` | `Receipt confirmation conflicted` | `service.py:6982-6985`; `repository.py:7000-7003` |
| 409 | mã `LedgerError` viết hoa | `Receipt events are invalid` (không tới được) | `service.py:6991-6992` |
| 500 | (text/plain) | `Internal Server Error` khi `amount_vnd >= 2**63` với khoá mới | `repository.py:7012-7013`; `guest_privacy.py:65` |

## Mã Python

- Route: `services/api/app/api/routes/obligations.py:22-39`
- Service: `services/api/app/api/service.py:6959-6998` (`confirm_receipt`)
- Domain: `services/api/app/domain/ledger.py:143-216`; `services/api/app/domain/permissions.py:101-104`
- Repository: `services/api/app/api/repository.py:6957-6969` (`get_receipt_target`), `:6971-7030` (`save_receipt_confirmation`), `:7032-7039`
- Schema: `services/api/app/api/schemas.py:1979-1991`

## Test đang phủ

- `services/api/tests/api/test_obligations.py`: `test_only_exact_recipient_can_confirm_receipt` (21), `test_recipient_confirmation_derives_confirmed_status_from_event_sum` (37), `test_receipt_money_rejects_string_before_repository` (56), `test_receipt_confirmation_is_idempotent` (71) — repository giả
- `services/api/tests/api/test_payment_report_board.py::test_confirming_receipt_does_not_erase_the_claim` (137)
- `services/api/tests/postgres/test_repository_postgres.py`: `test_repository_lifecycle_reaches_confirmed_receipt` (333), `test_receipt_report_must_belong_to_the_same_obligation` (586), `test_every_material_fact_table_rejects_in_place_updates` (623), `test_append_only_trigger_rejects_delete` (665)
- `services/api/tests/postgres/test_rollback_when_route_fails_postgres.py`: `test_a_route_that_writes_then_fails_leaves_no_row` (153), `test_a_cancelled_request_leaves_no_row` (181), `test_a_route_that_succeeds_keeps_its_row` (215)
- `services/api/tests/test_money_api_boundary_is_integer.py::test_every_money_field_refuses_a_fractional_value` (343)
- Chưa có test Python cho số tiền `>= 2**63`, cho khoá thân dùng lại bởi người nhận khác, hay cho người đã rời nhóm.

## Kịch bản parity

`parity/scenarios/w4/obligations/POST-obligations-obligation_id-confirm-receipt.yaml`, id `w4/obligations/post-obligations-obligation_id-confirm-receipt` (55 bước), lane DB bật. Khoá trong thân mượn 23 ký tự đầu của id nhóm (`kp`) do server sinh trong lượt chạy, vì khoá duy nhất toàn bảng:

- Thứ tự: `anonymous_unknown_obligation` (401), `anonymous_malformed_json`, `actor_id_not_uuid`, `roles_unknown`, `stranger_non_uuid_obligation`, `stranger_zero_amount_unknown_obligation` (422), `stranger_unknown_obligation`, `mate_roles_empty_unknown_obligation` (404).
- 403: `stranger_confirms`, `third_confirms_owners_obligation`, `owner_confirms_as_sender`, `mate_roles_member_only`, `mate_header_key_refused`.
- 201: `owner_confirms_partial`, `owner_same_body_key_again`, `owner_confirms_past_amount`, `owner_over_confirms`, `owner_confirms_int64_max`, `mate_same_key_after_failed_insert`, `owner_header_key_first`, `mate_header_key_after_refusal`, `mate_confirms_after_leaving`.
- 409: `owner_same_body_key_other_amount`, `owner_same_body_key_other_obligation`, `mate_reuses_owners_body_key`, `owner_same_body_key_with_report` (`idempotency_key_reused`), `owner_unknown_payment_report` (`payment_report_not_for_obligation`), `owner_int64_key_reused_past_int64`.
- 500: `mate_amount_past_int64`.
- 422 thân: `owner_amount_float`, `owner_amount_string`, `owner_amount_negative`, `owner_body_key_not_uuid`, `owner_body_key_missing`, `owner_extra_field`.
- Header idempotency: `owner_header_key_first`, `owner_header_key_replay_reordered`, `owner_header_key_other_body`, `mate_header_key_refused` → `mate_header_key_after_refusal`, `owner_empty_idempotency_key`.
- Framework: `get_not_allowed`.

`parity/scenarios/w4/obligations/prod-auth.yaml` (`w4/obligations/prod-auth`, 20 bước): `anonymous_missing_bearer`, `junk_bearer`, `actor_headers_ignored` (401), `owner_lowercase_scheme_unknown_obligation` (`bearer` chữ thường → 404), `mate_confirms_as_sender`, `stranger_confirms`, `stranger_claims_owner_headers` (403), `owner_confirms_partial`, `owner_same_body_key_again`, `owner_header_key_first`, `owner_header_key_replay`, `stranger_same_header_key` (phạm vi header là digest bearer → 403), `owner_revokes_session` + `owner_after_revoke` (401).

`parity/scenarios/w4/crossreplay/POST-obligations-obligation_id-confirm-receipt.yaml` (`w4/crossreplay/post-obligations-obligation_id-confirm-receipt`, 27 bước): header lưu ở Python → core phát lại/từ chối và ngược lại; khoá thân lưu ở một phía → phía kia trả lại dòng cũ hoặc 409; 404 và 409 của mỗi phía nhả header cho phía kia ghi.

`parity/scenarios/w4/concurrency/POST-obligations-obligation_id-confirm-receipt.yaml` (`w4/concurrency/post-obligations-obligation_id-confirm-receipt`, 14 bước): `header_same_key_burst` (3 bản, một dòng), `body_same_key_burst` (2 bản không header, cùng khoá thân: khoá dòng xếp hàng, bản sau trả dòng cũ), `header_same_key_other_body_burst` (mọi bản 422).

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/post-obligations-obligation_id-confirm-receipt.yaml` (69 bước).

## Chưa phủ / lưu ý cho bản Go

- `payment_report_id` hợp lệ (dòng `payment_reports` thật của nghĩa vụ): đã phủ bằng bước xác nhận trong `batches/GET-batches-batch_id-obligations-guest.yaml`, sau khi khách báo đã chuyển qua link của publish thật.
- **Loạt nhiều khoá thân khác nhau trên một nghĩa vụ chưa phủ (lỗ của harness)**: các dòng `receipt_confirmations` mới chỉ khác nhau bởi uuid4 nên làn DB xếp chúng theo id ngẫu nhiên, và trạng thái của từng bản phụ thuộc thứ tự khoá. Bản Go phải giữ khoá `FOR UPDATE` trên nghĩa vụ để trạng thái mỗi response đúng với receipts tính tới lúc đó.
- Tổng nhận của một nghĩa vụ vượt int64 không có trong kịch bản (view hỏng, làn DB dừng). Python cộng không giới hạn trong `obligation_status`; bản Go phải cộng bằng `big.Int` hoặc so sánh trước khi cộng để trạng thái đúng.
- `amount_vnd` đầu vào vượt int64: bản Go phải parse số JSON lớn hơn int64 mà không trả lỗi decode, so sánh với dòng đã có (409), và cho INSERT với khoá mới thất bại thành 500 như Python (hoặc ADR hoá một thay đổi). Hiện 500 có thân `text/plain` `Internal Server Error`.
- Trả dòng cũ theo khoá thân dùng mã 201 và trạng thái tính lại, không phải trạng thái lúc ghi.
- 404 trước 403: oracle tồn tại nghĩa vụ; khoá thân toàn bảng: oracle giữa những người nhận. Giữ nguyên để parity bằng nhau.
- Không kiểm membership: giữ nguyên.
