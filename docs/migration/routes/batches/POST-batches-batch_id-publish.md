# POST /batches/{batch_id}/publish

batches · core · trạng thái trong bộ nhớ: không có

## Mục đích

Chủ đợt gửi đợt thu đã đóng băng: kiểm cổng gửi của spec mục 8.3 (người trả hộ đã thừa nhận), chuyển trạng thái `frozen → published`, và tạo **một phong bì + một link khách** cho mỗi người nợ. Link khách là bearer capability: token thô chỉ xuất hiện một lần trong response (`path: "/g/<token>"`), máy chủ chỉ giữ digest. Đây là cửa vào trang khách `GET /g/{token}`, báo đã chuyển và phản đối.

## Xác thực và quyền

Thứ tự (đọc từ mã; kịch bản chỉ đo được các nhánh từ chối):

1. Middleware idempotency: khoá rỗng → 422 `invalid_idempotency_key` (`services/api/app/api/idempotency.py:432-439`, `owner_empty_idempotency_key`).
2. JSON hỏng → 422 trước actor (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 (`anonymous_unknown_batch`), 422 `invalid_actor_id` (`actor_id_not_uuid`).
4. Validation path (`batch_id: UUID`) và thân `BatchPublishRequest` (`services/api/app/api/schemas.py:1902-1906`): `stranger_non_uuid_batch`, `owner_expiry_without_offset`, `owner_unknown_delivery_method`, `owner_missing_delivery_method`.
5. `load_batch_for_publish` (`services/api/app/api/repository.py:6389-6458`) khoá `collection_batches` `FOR UPDATE`; không có → 404 `batch_not_found` **trước mọi kiểm quyền** (`services/api/app/api/service.py:6599-6601`): người lạ biết id đợt có tồn tại hay không (`stranger_unknown_batch` 404 so với `stranger_publishes` 403).
6. `_require_permission("publish_batch", {"owns_batch": actor.id == owner_id}, extra_roles={"batch_owner"} chỉ khi là chủ)` (`service.py:6602-6616`; `services/api/app/domain/permissions.py:69-75`):
   - không phải chủ, role không có `batch_owner` → 403 `role_not_permitted` (`stranger_roles_member_only`, `mate_roles_member_only`; ở prod mọi người không phải chủ đều vào nhánh này vì phiên không mang `batch_owner`, `repository.py:3629`);
   - không phải chủ nhưng header dev có `batch_owner` (role mặc định của harness) → 403 `owns_batch` (`stranger_publishes`, `mate_publishes`, `mate_ready_batch_not_owner`); không có kiểm membership, nên người lạ và thành viên giống nhau;
   - chủ đợt luôn qua dù header role là `member` hay rỗng (`owner_roles_member_only_gate`, `owner_roles_empty_gate`).
7. Quyền trước hạn: `mate_expiry_in_past` → 403.
8. `guest_link_expires_at <= now` → 422 `guest_link_expiry_not_future` (`service.py:6617-6623`), trước cổng: cả với đợt đã thừa nhận (`owner_ready_batch_expiry_in_past`).
9. `unmet_publish_gates` (`services/api/app/domain/collection.py:71-89`) → 409 với mã đầu tiên còn thiếu (`service.py:6624-6630`): `advancer_acknowledgement_required` khi **có ít nhất một** phiên bản nguồn chưa `acknowledged` (`repository.py:6416-6439`; `owner_gate_unacknowledged`, `mate_gate_mixed_acknowledgement`). `delivery_method_required` không tới được: `delivery_method` là `Literal["personal_link"]` bắt buộc.
10. `transition(status, "publish")` (`collection.py:92-124`) → 409 `ILLEGAL_TRANSITION` khi đợt không còn `frozen` (`service.py:6631-6634`), tức gửi lần hai.

## Đầu vào

- Path `batch_id` (UUID lax).
- Thân `BatchPublishRequest` (`extra="forbid"`): `delivery_method: "personal_link"`, `guest_link_expires_at: datetime` bắt buộc có offset (`schemas.py:70-73`).
- Header tuỳ chọn `Idempotency-Key`.

## Đầu ra

- **200** `BatchPublishResponse` (`schemas.py:1921-1924`), thứ tự khoá `batch_id`, `status` (`"published"`), `guest_links`.
  - `guest_links[]`: `PublishedGuestLink` (`schemas.py:1914-1918`) = `sender_id`, `path`, `expires_at`, `obligations`; một phần tử mỗi người nợ, sắp theo **byte của UUID** người nợ (`service.py:6643-6645`).
  - `path` = `"/g/" + secrets.token_urlsafe(32)` (43 ký tự base64url, `service.py:6658`, `:6673`).
  - `expires_at` = giá trị request (offset giữ nguyên), không đọc lại từ DB.
  - `obligations[]` = `{"obligation_id","amount_vnd"}` (`schemas.py:1909-1911`), thứ tự nạp `ORDER BY sender_id, recipient_id` (`repository.py:6406-6415`).
- Từ chối: thân `{"code","detail"}` (`services/api/app/api/main.py:296-316`) hoặc `{"detail":[…]}` cho validation (`main.py:318-351`).

## Tác dụng phụ

- **Khoá dòng** `collection_batches` `FOR UPDATE` từ bước 5 tới commit (hoặc rollback khi từ chối).
- Chỉ nhánh thành công ghi (`repository.py:6460-6514`), cùng `now`:
  - `collection_batches`: UPDATE `status='published'`, `published_at=now` (bảng này không append-only). Trạng thái `collecting` không bao giờ được đặt: không route nào gọi `expose_capability`.
  - `collection_envelopes`: một dòng mỗi người nợ, `UNIQUE(batch_version_id, sender_id)` (`services/api/app/db/models.py:670-675`), append-only (trigger, `20260827_0001_initial_api_schema.py:666-688`).
  - `guest_links`: `envelope_id`, `token_digest = sha256(token)` 32 byte `bytea UNIQUE` (`models.py:712-714`, `service.py:410-411`), `status='active'`, `expires_at`, `created_at`, `CHECK expires_at > created_at` (`models.py:700`). Token thô không được lưu.
  - `audit_events`: `collection_batch_published`, `{"batch_version_id","guest_link_count"}`.
  - `idempotency_keys` khi có header.
- Phạm vi capability được kiểm bằng `capability_scope` (`services/api/app/domain/capability.py:14-43`) trước khi ghi.
- Mọi từ chối không ghi gì; khoá idempotency được nhả, nên cùng khoá gửi lại (cùng hay khác thân) chạy lại chứ không phát lại hay báo reuse (`owner_key_gate_refused_again`, `owner_key_other_body_after_release`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /batches` | `deps.py:144-163` |
| 422 | (validation) | `{"detail":[…]}` không có `input` | `main.py:318-351` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse`; 409 `idempotency_request_in_flight` | xem card `POST /batches` | `idempotency.py:432-493` |
| 404 | `batch_not_found` | `Batch does not exist` | `service.py:6600-6601` |
| 403 | `permission_denied` | `role_not_permitted` / `owns_batch` | `service.py:6603-6616`, `:504` |
| 422 | `guest_link_expiry_not_future` | `guest_link_expires_at must be in the future` | `service.py:6618-6623` |
| 409 | `advancer_acknowledgement_required` (`delivery_method_required` không tới được) | `A publish gate is not satisfied` | `service.py:6628-6630`; `collection.py:84-89` |
| 409 | `ILLEGAL_TRANSITION` (mã miền viết hoa) | `Batch cannot be published` | `service.py:6631-6634`; `collection.py:97-99` |
| 409 | mã `CapabilityScopeError` viết hoa | `A guest capability cannot be built` (không tới được) | `service.py:6684-6687` |
| 409 | `batch_not_found` (chữ thường từ `RepositoryConflict`) | `Batch publication conflicted` (không tới được) | `service.py:6697-6700`; `repository.py:6470-6471` |
| 500 | `guest_link_write_mismatch` | `Guest link write was incomplete` (không tới được) | `service.py:6701-6704` |
| 500 | (text/plain) | `Internal Server Error` khi `BATCH_HAS_NO_VERSION` (không tới được) | `repository.py:6403-6404` |

## Mã Python

- Route: `services/api/app/api/routes/batches.py:46-57`
- Service: `services/api/app/api/service.py:6593-6709` (`publish_batch`), `:410-411` (`token_digest`)
- Domain: `services/api/app/domain/collection.py:71-124`; `services/api/app/domain/capability.py:14-43`; `services/api/app/domain/permissions.py:69-75`
- Repository: `services/api/app/api/repository.py:6389-6458` (`load_batch_for_publish`), `:6460-6514` (`save_published_batch`)
- Schema: `services/api/app/api/schemas.py:1902-1924`

## Test đang phủ

- `services/api/tests/api/test_batches.py`: `test_publish_checks_ack_gate_separately_from_expense_confirmation` (57), `test_publish_creates_one_sender_scoped_link_carrying_only_the_amount` (77), `test_non_owner_cannot_publish_even_with_batch_owner_role` (102) — repository giả
- `services/api/tests/postgres/test_repository_postgres.py::test_repository_lifecycle_reaches_confirmed_receipt` (333), `test_guest_is_not_told_the_money_arrived_before_it_did` (388)
- `services/api/tests/api/test_app_wiring.py` (khai báo route)

## Kịch bản parity

`parity/scenarios/w4/batches/POST-batches-batch_id-publish.yaml`, id `w4/batches/post-batches-batch_id-publish` (42 bước), **chỉ nhánh từ chối**:

- Thứ tự: `anonymous_unknown_batch` (401), `anonymous_malformed_json`, `actor_id_not_uuid`, `stranger_non_uuid_batch` (422), `stranger_unknown_batch` (404).
- 403: `stranger_publishes`, `mate_publishes`, `mate_ready_batch_not_owner` (`owns_batch`); `stranger_roles_member_only`, `mate_roles_member_only` (`role_not_permitted`); `mate_expiry_in_past` (quyền trước hạn).
- 422: `owner_expiry_in_past`, `owner_ready_batch_expiry_in_past` (`guest_link_expiry_not_future`), `owner_expiry_without_offset`, `owner_unknown_delivery_method`, `owner_missing_delivery_method`.
- 409 cổng: `owner_gate_unacknowledged` (đợt từ khoản `acknowledge_as_advancer:false`), `owner_roles_member_only_gate`, `owner_roles_empty_gate` (role của chủ được suy ra), `mate_gate_mixed_acknowledgement` (một khoản đã, một khoản chưa thừa nhận).
- Idempotency: `owner_key_gate_refused`, `owner_key_gate_refused_again`, `owner_key_other_body_after_release`, `owner_empty_idempotency_key`.
- Framework: `get_not_allowed` (405).

`parity/scenarios/w4/batches/prod-auth.yaml` (`w4/batches/prod-auth`): `actor_headers_ignored_publish` (401), `mate_publishes`, `mate_publishes_with_batch_owner_header`, `stranger_publishes` (403 `role_not_permitted`), `owner_publish_expiry_in_past` (422), `owner_publish_gate` (409).

`parity/scenarios/w4/crossreplay/POST-batches-batch_id-publish.yaml` (`w4/crossreplay/post-batches-batch_id-publish`, 18 bước): 409 cổng, 422 hạn, 404, 403 dưới khoá ở một phía rồi cùng khoá ở phía kia chạy lại.

Không có tệp concurrency: loạt duy nhất có nghĩa (hai lần gửi một đợt: một 200 và một 409 `ILLEGAL_TRANSITION` nhờ khoá dòng) cần nhánh thành công.

Corpus 422 sinh tự động: hoãn, `'function-after' is not probed` (validator múi giờ của `guest_link_expires_at`).

## Chưa phủ / lưu ý cho bản Go

- **Nhánh thành công đã có kịch bản** từ khi harness gắn token 43 ký tự thành `<token43#n>` và làn DB phân định hàng hoà: `POST-batches-batch_id-publish-success.yaml` (hai người gửi, phát lại cùng khoá, thân tương đương, khoá dùng lại, publish lần hai 409, token gắn theo tên mở trang khách), `prod-publish-success.yaml`, `crossreplay/POST-batches-batch_id-publish-success.yaml` (mỗi bên phát lại 200 bên kia đã lưu với cùng token) và `concurrency/POST-batches-batch_id-publish.yaml`.
- Bản Go phải sinh token 32 byte ngẫu nhiên, base64url không đệm (43 ký tự), lưu đúng sha256 32 byte vào `bytea`, và không bao giờ lưu token thô.
- 404 trước 403: bản Go giữ nguyên thứ tự để parity bằng nhau, dù đây là oracle tồn tại.
- Chủ đợt nhận `batch_owner` từ tài nguyên, không từ header hay phiên; ở dev, người không phải chủ mà header có `batch_owner` được `owns_batch`, không có header thì `role_not_permitted`.
- Cổng thừa nhận tính trên các phiên bản nối qua `collection_obligation_sources`, không phải trên mọi khoản của nhóm; nếu nguồn rỗng thì `bool(expense_versions)` là false và cổng trượt.
- `expires_at` trong response là giá trị request; `guest_links.expires_at` là `timestamptz`.
