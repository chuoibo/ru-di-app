# POST /batches

batches · core · trạng thái trong bộ nhớ: không có

## Mục đích

Gom các phân bổ đã xác nhận của một nhóm thành **một đợt thu đóng băng** (`frozen`): mỗi cặp người nợ → người trả hộ thành một nghĩa vụ, kèm nguồn là từng dòng `confirmed_allocations`. Đây là bước ghi sổ thứ hai của lát cắt dọc (`confirm` → `POST /batches` → `publish`). Người gọi trở thành chủ đợt (`owner_id`), không phải người trả hộ. Không sinh link khách; chưa gửi gì cho ai.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo từng nhánh):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`) chạy trước routing: header `Idempotency-Key` rỗng hoặc dài quá 255 → 422 `invalid_idempotency_key` (`:432-439`), trước cả actor (`owner_key_*` trong kịch bản; khoá rỗng có ở `w4/batches/post-batches-batch_id-publish`).
2. FastAPI giải mã JSON trước dependency: thân hỏng → 422 `json_invalid` thắng 401 (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): thiếu `X-Actor-ID` → 401 (`anonymous_creates`); không phải UUID → 422 `invalid_actor_id` (`actor_id_not_uuid`). Ở `prod` đọc `Authorization: Bearer` (`deps.py:93-107`, `service.py:4452-4473`).
4. Validation thân (`BatchCreateRequest`, `services/api/app/api/schemas.py:1878-1883`): `due_at` không có offset → 422 (`owner_due_without_offset`), id phiên bản không phải UUID → 422 (`owner_version_id_not_uuid`).
5. `_require_permission("create_batch", {"is_group_member": is_member(context_id, actor)})` (`services/api/app/api/service.py:6475-6483`; `services/api/app/domain/permissions.py:67`): role `member` bắt buộc (role rỗng → 403 `role_not_permitted`, `mate_roles_empty`); membership ACTIVE, `left_at IS NULL` (`repository.py:2769-2782`) → 403 `is_group_member` cho nhóm không tồn tại (`stranger_unknown_context`), nhóm thật (`stranger_real_context`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header`). Quyền đứng trước hạn: người lạ với `due_at` quá khứ vẫn 403 (`stranger_due_in_past`).
6. `_require_permission("freeze_batch", {"owns_batch": True}, extra_roles={"batch_owner"})` (`service.py:6486-6491`; `permissions.py:68`): role và vị từ đều do service tự đặt, **không bao giờ trượt**.
7. `due_at <= now` → 422 `due_at_not_future` (`service.py:6492-6494`, `owner_due_in_past`). `now` là `datetime.now(UTC)` (`service.py:406-407`).

## Đầu vào

- Thân JSON `BatchCreateRequest` (`extra="forbid"`): `context_id: UUID`, `expense_version_ids: list[UUID] | null` (mặc định `null`), `due_at: datetime` bắt buộc có offset (`_require_timezone`, `schemas.py:70-73`).
- `expense_version_ids` vắng hoặc `null`: mọi phiên bản **mới nhất** đã xác nhận của nhóm (`service.py:6496-6498`); danh sách tường minh: đúng những id đó, id lặp chỉ tính một lần (`owner_batches_opposite_pairs`), chữ hoa/không gạch được UUID lax chấp nhận.
- Header tuỳ chọn `Idempotency-Key`.

## Đầu ra

- **201** `BatchCreateResponse` (`schemas.py:1895-1899`), thứ tự khoá `batch_id`, `batch_version_id`, `status` (luôn `"frozen"`), `obligations`.
  - `obligations[]`: `ObligationResponse` (`schemas.py:1886-1892`) theo thứ tự `obligation_id`, `sender_id`, `recipient_id`, `amount_vnd`, `due_at`, `source_expense_version_ids`.
  - Thứ tự nghĩa vụ: `merge_obligations` sắp theo byte chuỗi `(sender_id, recipient_id)` (`services/api/app/domain/ledger.py:126-139`); với UUID chữ thường, thứ tự này trùng thứ tự byte.
  - `amount_vnd`: tổng số nguyên các phân bổ của cặp (`ledger.py:115-124`), > 0. Cặp ngược chiều **không bù trừ**: `owner_batches_opposite_pairs` cho mate → owner và owner → mate là hai nghĩa vụ.
  - `due_at`: `FrozenObligation` mang **giá trị của request**, không đọc lại từ DB (`repository.py:6362-6368`), nên pydantic viết lại đúng offset người gọi gửi (`+07:00` giữ nguyên; phần giây lẻ in theo cách pydantic: `mate_batches_the_rest` gửi `2030-02-01T23:59:59.5+07:00`). Cột DB là `timestamptz`.
  - `source_expense_version_ids`: `sorted(set(...))` theo chuỗi UUID (`ledger.py:135-137`).
  - Người chỉ tự chia cho mình, phần 0 đồng, hay người trả hộ không có trong danh sách chia đều không sinh nghĩa vụ (`ledger.py:88-94`); phần 0 của `owner_confirms_coffee` không thành nghĩa vụ.
- Mọi số tiền là số nguyên đồng; số tiền phân bổ bị allocator giới hạn `10**12` mỗi khoản (`services/api/app/domain/contract.py:12`), nên tổng một cặp không vượt int64 trong thực tế.

## Tác dụng phụ

- **Khoá dòng**: `load_batch_inputs` chọn phiên bản mới nhất với `FOR UPDATE OF expense_versions` (`repository.py:6190-6208`), rồi mỗi phiên bản `SELECT confirmed_allocations … ORDER BY participant_id FOR UPDATE` (`:6215-6222`). Khi danh sách vắng, hàm được gọi **hai lần** trong cùng giao dịch (`service.py:6497`, `:6501`). Khoá giữ tới commit trước response (`services/api/app/api/unit_of_work.py:41-48`). Hai lần tạo đợt cùng lúc trên cùng phiên bản xếp hàng; bản sau đọc lại `collection_obligation_sources` đã commit và trả 409 (`w4/concurrency/post-batches`).
- Tính khả dụng (`repository.py:6210-6262`): phiên bản không có dòng phân bổ → không khả dụng; ở nhánh danh sách tường minh, một phiên bản bị loại nếu **bất kỳ** dòng phân bổ thu được (người khác người trả, > 0) đã là nguồn. Phiên bản không có dòng thu được (chỉ người trả, hoặc tổng 0) **không bao giờ** có nguồn nên luôn khả dụng.
- Ghi (`repository.py:6312-6387`), mọi dòng cùng `now`:
  - `collection_batches`: `status='frozen'`, `created_at = frozen_at = now`, `published_at` null, `owner_id` = người gọi.
  - `collection_batch_versions`: `version_number=1`, `previous_version_number` null, `created_by_id`.
  - `collection_obligations`: một dòng mỗi cặp, `due_at`, `created_at`; `UNIQUE(batch_version_id, sender_id, recipient_id)`, `CHECK sender_id <> recipient_id`, `CHECK amount_vnd > 0` (`services/api/app/db/models.py:614-623`).
  - `collection_obligation_sources`: một dòng mỗi dòng phân bổ nguồn của cặp (`service.py:6528-6532`, `repository.py:6352-6360`), `amount_vnd` = số tiền phân bổ.
  - `audit_events`: `collection_batch_frozen`, `event_data = {"batch_version_id", "obligation_count"}`.
  - `idempotency_keys` khi có header (middleware, giao dịch riêng sau commit).
- Bốn bảng `collection_batch_versions`, `collection_obligations`, `collection_obligation_sources`, `audit_events` là append-only bằng trigger `reject_*_mutation` (`services/api/app/db/migrations/versions/20260827_0001_initial_api_schema.py:666-688`). `collection_batches` thì không (publish UPDATE nó).
- View `collection_obligation_progress` (`20260827_0001…py:621-646`) đọc nghĩa vụ mới; không route nào đọc view này.
- Mọi từ chối: không ghi dòng nào (rollback ở `deps.py:196-209`), khoá idempotency được nhả (`idempotency.py:544-546`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | `X-Actor-ID must be a UUID` / `X-Actor-Roles contains an unknown role` / `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:144-163` |
| 422 | (không có `code`) | `{"detail":[…]}` của FastAPI đã bỏ `input` | `services/api/app/api/main.py:318-351` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:475-504`, `:6475-6483` |
| 422 | `due_at_not_future` | `due_at must be in the future` | `service.py:6493-6494` |
| 409 | `expense_versions_unavailable` | `A selected version is missing, superseded, or already batched` | `service.py:6502-6507` |
| 409 | `no_unbatched_allocations` | `No allocations are available` | `service.py:6508-6511` |
| 409 | mã `LedgerError` viết hoa | `Allocations cannot form obligations` (không tới được) | `service.py:6534-6537` |
| 409 | `no_obligations` | `The selected allocations owe no money` | `service.py:6538-6541` |
| 409 | mã `CollectionError` viết hoa | `Batch cannot be frozen` (không tới được) | `service.py:6544-6547` |
| 500 | `unexpected_batch_state` | `Domain returned an invalid state` (không tới được) | `service.py:6548-6551` |

## Mã Python

- Route: `services/api/app/api/routes/batches.py:32-43`
- Service: `services/api/app/api/service.py:6472-6591` (`create_batch`), `:475-504` (`_require_permission`)
- Domain: `services/api/app/domain/ledger.py:63-103` (`obligations_from_allocations`), `:106-140` (`merge_obligations`); `services/api/app/domain/collection.py:56-68`, `:92-124` (`transition` freeze); `services/api/app/domain/permissions.py:67-68`
- Repository: `services/api/app/api/repository.py:6171-6268` (`load_batch_inputs`), `:6312-6387` (`save_frozen_batch`), `:2769-2782` (`is_member`)
- Schema: `services/api/app/api/schemas.py:1878-1899`

## Test đang phủ

- `services/api/tests/api/test_batches.py`: `test_batch_uses_domain_merge_for_same_sender_recipient_pair` (14), `test_a_batch_freezes_without_anybody_registering_an_account` (33) — repository giả
- `services/api/tests/api/test_membership_is_not_self_certified.py::test_an_outsider_cannot_open_a_collection_batch_on_someone_elses_group` (151)
- `services/api/tests/postgres/test_repository_postgres.py::test_repository_lifecycle_reaches_confirmed_receipt` (333), `test_every_material_fact_table_rejects_in_place_updates` (623), `test_append_only_trigger_rejects_delete` (665)
- `services/api/tests/api/test_app_wiring.py` (khai báo route), `services/api/tests/api/test_idempotency.py` (middleware chung)
- Không có test Python cho khoá `FOR UPDATE` hai đợt đồng thời, cho nhánh `expense_versions_unavailable` với phiên bản bị thay thế hay nhóm khác, hay cho phiên bản chỉ người trả.

## Kịch bản parity

`parity/scenarios/w4/batches/POST-batches.yaml`, id `w4/batches/post-batches` (56 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_creates` (401), `anonymous_malformed_json` (422 thắng 401), `actor_id_not_uuid`.
- 403: `stranger_unknown_context`, `stranger_real_context`, `stranger_claims_context_header`, `mate_roles_empty`, `stranger_due_in_past` (quyền trước hạn).
- 422: `owner_due_in_past` (`due_at_not_future`), `owner_due_without_offset`, `owner_version_id_not_uuid`.
- 409 `no_unbatched_allocations`: `owner_nothing_confirmed` (chưa có gì), `owner_empty_version_list` (`[]`), `owner_everything_batched` (mọi phiên bản đã vào đợt, danh sách vắng, `due_at` viết `Z`).
- 409 `expense_versions_unavailable`: `owner_unknown_version` (một id lạ trong danh sách), `owner_superseded_version` (phiên bản 1 sau khi `owner_confirms_dinner_v2` sửa), `owner_version_of_other_group` (phiên bản của `ctx2`), `owner_already_batched`, `owner_key_refused_already_batched`.
- 409 `no_obligations`: `owner_payer_only_version` (khoản chỉ người trả chia, ở `ctx2`), `owner_zero_total_version` (tổng 0), `owner_list_omitted_zero_total_left` (danh sách vắng, chỉ còn khoản tổng 0: nó vẫn “khả dụng”).
- 201: `owner_batches_opposite_pairs` (bữa tối mate → owner 120 000 và taxi owner → mate 30 000, third → mate 15 000; id lặp), `mate_batches_the_rest` (danh sách `null`, người gọi là mate, ba nghĩa vụ từ bữa trưa và bữa sáng), `owner_key_first`, `owner_key_after_refusal`.
- Phân bổ dữ liệu mẫu: taxi 75 001 = món 60 001 chia owner + mate (người trả hộ mate lấy đồng lẻ) + món 15 000 của third; bữa sáng 10 000 / 6 000 / 4 000; cà phê có phần 0 của mate.
- Idempotency: `owner_key_first`, `owner_key_replay_reordered` (phát lại, khoá đổi thứ tự), `owner_key_reused_other_due` (422 reuse), `owner_key_refused_already_batched` → `owner_key_after_refusal` (409 nhả khoá, cùng khoá ghi được).
- Framework: `get_not_allowed` (405).

`parity/scenarios/w4/batches/prod-auth.yaml` (`w4/batches/prod-auth`, 27 bước): `anonymous_create_missing_bearer`, `stranger_creates_batch` (403), `owner_creates_batch_empty_roles_header` (201: header role bị bỏ qua), `owner_key_first` + `owner_key_replay` + `stranger_same_key` (phạm vi khoá là digest bearer → chạy như người lạ, 403).

`parity/scenarios/w4/crossreplay/POST-batches.yaml` (`w4/crossreplay/post-batches`, 26 bước): Python lưu → core phát lại (cả khi đổi thứ tự khoá) và từ chối thân khác; core lưu → Python phát lại/từ chối; 409 của mỗi phía nhả khoá cho phía kia ghi.

`parity/scenarios/w4/concurrency/POST-batches.yaml` (`w4/concurrency/post-batches`, 15 bước): `same_key_burst` (3 bản, một lần đóng băng), `no_key_same_versions_burst` (2 bản cùng danh sách: một 201, một 409), `same_key_other_body_burst` (mọi bản 422), `no_key_list_omitted_burst` (2 bản danh sách vắng: một 201, một 409 `no_unbatched_allocations`).

Corpus 422 sinh tự động: hoãn, bộ sinh từ chối với `'function-after' is not probed` (validator múi giờ của `due_at`); các lỗi thân tay nằm ở kịch bản trên.

## Chưa phủ / lưu ý cho bản Go

- **Gộp hai khoản chi vào một nghĩa vụ chưa phủ (lỗ của harness)**: `source_expense_version_ids` được sắp theo chuỗi UUID ngẫu nhiên nên thứ tự so với số thứ tự `<uuid#n>` đổi mỗi lượt. Bản Go phải sắp đúng theo chuỗi (`ledger.py:135-137`) và cộng tiền theo cặp; test miền `test_batch_uses_domain_merge_for_same_sender_recipient_pair` phủ việc cộng.
- **Hai dòng nguồn cùng số tiền trong một đợt chưa phủ (lỗ của harness)**: dòng mới trong một bước chỉ khác nhau bởi uuid4 được làn DB xếp theo id ngẫu nhiên; kịch bản giữ số tiền mọi nguồn trong một đợt khác nhau.
- Hai lần gọi `load_batch_inputs` khi danh sách vắng: bản Go gộp thành một câu vẫn phải cho cùng kết quả khả dụng và giữ khoá trên cùng các dòng.
- Khoá `FOR UPDATE OF expense_versions` rồi `confirmed_allocations`: bỏ khoá sẽ vẫn xanh phần lớn parity tuần tự, nhưng loạt đồng thời không khoá có thể ra hai đợt trên cùng nguồn.
- `due_at` trả về là giá trị request, không phải giá trị đọc lại từ `timestamptz`; bản Go đọc lại từ DB sẽ đổi offset thành `Z`.
- Khoản chỉ người trả chia hoặc tổng 0 không bao giờ có nguồn: nhóm có một khoản như vậy sẽ không bao giờ trả `no_unbatched_allocations` cho danh sách vắng nữa (luôn `no_obligations`).
- Sửa (xác nhận lại) một khoản đã vào đợt tạo phiên bản mới không có nguồn, và phiên bản đó lại vào được đợt mới: cùng khoản chi có thể bị thu hai lần. Đọc từ `repository.py:6182-6246`, chưa có kịch bản.
- Các 409 mang mã miền viết hoa (`LedgerError`, `CollectionError`) và 500 `unexpected_batch_state` không tới được qua HTTP.
