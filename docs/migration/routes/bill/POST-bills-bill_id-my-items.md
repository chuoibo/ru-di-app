# POST /bills/{bill_id}/my-items

bills · core · trạng thái trong bộ nhớ: không có

(Thư mục là `bill/` chứ không phải `bills/`: repo guard chặn thành phần đường dẫn `bills`, `scripts/repo_guard.py:99-114`.)

## Mục đích

F22 tự nhận món: người gọi gửi **toàn bộ** danh sách món của mình trên bill. Mọi share của chính người đó trên bill (gợi ý hay đã xác nhận) bị xoá, rồi các món được gửi, bỏ trùng, được ghi lại thành `confirmed` với người quyết định là chính người đó. Share của người khác không đổi. Body không có trường nào nêu được tên người, nên route không kiểm roster (`services/api/app/api/routes/bills.py:73-92`, `services/api/app/api/service.py:6068-6092`).

## Xác thực và quyền

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-553`), xem card `POST /bills`.
2. Giải mã JSON → 422 trước actor (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401 (`anonymous_unknown_bill`), 422 `invalid_actor_id` (`actor_id_not_uuid`).
4. Path + body (pydantic) → 422 (`stranger_non_uuid_bill`, `owner_names_someone_else`, `owner_invalid_item_keys`, `owner_item_keys_not_a_list`).
5. `_bill_for_actor` (`service.py:5919-5928`): 404 `bill_not_found` trước quyền (`stranger_unknown_bill`); 403 `is_group_member` (`stranger_real_bill`, `invitee_claims`, `mate_claims_after_leaving`) hoặc `role_not_permitted` (`mate_roles_empty`).
6. Repository `claim_bill_items` (`services/api/app/api/repository.py:5951-6020`): khoá `bills` FOR UPDATE (`:5978-5982`), khoá mọi `bill_items` FOR UPDATE (`:5984-5989`), bỏ trùng giữ thứ tự (`dict.fromkeys`, `:5993`), khoá lạ → `UNKNOWN_BILL_ITEM` → **422 `unknown_bill_item`** (`:5994-5996`; `service.py:6105-6110`) trước khi xoá gì (`owner_claims_unknown_item`, `owner_claims_item_key_other_case`, `owner_claims_on_bill_without_lines`).
7. Nếu bill có dòng: khoá rồi xoá mọi share của người gọi trên bill (`:5998-6007`); chèn share `confirmed` cho từng khoá (`:6009-6019`).

## Đầu vào

- Path `bill_id` (UUID lax).
- Body `BillSelfClaimRequest` (`services/api/app/api/schemas.py:293-310`): `item_keys: list[str ≤ 64 ký tự]` strict, `extra="forbid"` — `participant_id` hay bất kỳ khoá thêm nào là 422.
- `item_keys: []` nhả mọi món của người gọi (`mate_releases_everything`, `owner_claims_nothing_on_bill_without_lines`); khoá lặp chỉ ghi một lần (`owner_claims_tra_twice`); khoá phân biệt hoa thường.

## Đầu ra

- **200** `BillResponse` (`schemas.py:241-253`) qua `_wire_bill` (`service.py:731-792`), hình và thứ tự như card `POST /bills`; đọc lại sau flush (`repository.py:6020`).
- Share mới: `source: "confirmed"`, `participant_id` = `decided_by_id` = người gọi, `decided_at` = `datetime.now(UTC)` (`service.py:6096-6101`).
- Gợi ý AI của người gọi trên **món khác** cũng bị xoá (`mate_claims_pho` bỏ gợi ý `Nem` của người đó); gợi ý của người khác trên cùng món vẫn còn, nên `assignment_state` có thể vẫn là `"ai_suggested"`.
- Gửi lại cùng danh sách cho cùng trạng thái (`mate_repeats_the_same_claim`), nhưng `decided_at` là thời điểm mới.
- Phát lại idempotency trả trạng thái lúc lưu (`owner_claims_tra_without_key` rồi `owner_key_replay_spaced`).
- Framework: `GET /bills/{id}/my-items` → 405 `allow: POST` (`get_not_allowed`).

## Tác dụng phụ

- Khoá `bills`, mọi `bill_items` và share của người gọi, giữ tới commit trước response (`services/api/app/api/unit_of_work.py:41-48`).
- DELETE share của người gọi trên bill; INSERT share `confirmed`. Không chạm dòng, phụ phí, giảm giá, sổ; không audit event.
- Có header `Idempotency-Key` → hàng `idempotency_keys`.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` · prod: `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /bills` | `deps.py:144-163` |
| 422 | (không có `code`) | `{"detail":[…]}` của pydantic, bỏ `input` | `services/api/app/api/main.py:318-351` |
| 422 / 409 | `invalid_idempotency_key` / `idempotency_key_reuse` / `idempotency_request_in_flight` | xem card `POST /bills` | `idempotency.py:432-493` |
| 404 | `bill_not_found` | `Bill does not exist` | `service.py:5920-5922` (và `:6103-6104`, không tới được) |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5923-5927`, `:504` |
| 422 | `unknown_bill_item` | `Bill does not contain one of these items` | `repository.py:5994-5996`; `service.py:6105-6110` |
| 409 | mã `RepositoryConflict` khác (viết hoa) | `Bill assignment conflicted` | `service.py:6111` (không tới được: khoá đã bỏ trùng) |

## Mã Python

- Route: `services/api/app/api/routes/bills.py:73-92`
- Service: `services/api/app/api/service.py:6068-6112` (`claim_bill_items`), `:5919-5928`, `:731-792`
- Repository: `services/api/app/api/repository.py:5951-6020`, `:2444-2523`
- Schema: `services/api/app/api/schemas.py:293-310`

## Test đang phủ

- Postgres thật: `services/api/tests/postgres/test_bill_self_claim_postgres.py`: `test_the_row_that_lands_names_the_caller` (199), `test_claiming_a_shared_dish_does_not_evict_the_other_claimant` (212), `test_the_list_is_the_whole_claim_so_a_mistap_can_be_undone` (233), `test_releasing_everything_leaves_the_others_alone` (249), `test_repeating_the_same_claim_is_idempotent` (258), `test_the_body_cannot_name_anybody` (269), `test_a_stranger_cannot_claim_a_dish_on_a_group_they_are_not_in` (295), `test_an_item_that_is_not_on_this_bill_is_refused` (306), `test_self_tagging_writes_nothing_to_the_ledger` (318), `test_the_split_of_a_self_tagged_bill_still_sums_to_the_total` (343), `test_retagging_changes_the_split_and_still_sums_to_the_total` (377), `test_every_amount_is_a_whole_number_of_dong` (402)
- Không tìm thấy test HTTP với repository giả cho route này trong `tests/api`.

## Kịch bản parity

`parity/scenarios/w4/bill/POST-bills-bill_id-my-items.yaml`, id `w4/bill/post-bills-bill_id-my-items` (42 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_bill`, `anonymous_malformed_json`, `actor_id_not_uuid`, `stranger_non_uuid_bill`, `stranger_unknown_bill`.
- 403: `stranger_real_bill`, `invitee_claims`, `mate_roles_empty`, `mate_claims_after_leaving`.
- 422 món lạ: `owner_claims_unknown_item`, `owner_claims_item_key_other_case`, `owner_claims_on_bill_without_lines`.
- 200: `mate_claims_pho`, `owner_claims_tra_twice`, `third_claims_nem` (mọi món đã xác nhận), `mate_repeats_the_same_claim`, `mate_releases_everything`, `owner_claims_nothing_on_bill_without_lines`.
- Pydantic: `owner_names_someone_else`, `owner_invalid_item_keys`, `owner_item_keys_not_a_list`.
- Idempotency: `owner_key_first`, `owner_claims_tra_without_key`, `owner_key_replay_spaced`, `owner_key_reused_repeated_key` (`["Nem","Nem"]` là body khác), `owner_key_refused_unknown_item` + `owner_key_after_refusal`.
- Framework: `get_not_allowed` (405).

`prod` (`w4/bill/prod-auth`): `actor_headers_ignored_claims` (401), `mate_claims_tra` (200).

Replay chéo `parity/scenarios/w4/crossreplay/POST-bills-bill_id-my-items.yaml` (`w4/crossreplay/post-bills-bill_id-my-items`, 20 bước): phát lại trạng thái cũ; khoá lặp hay đã bỏ trùng là body khác; 403 và 422 nhả khoá.

Đồng thời `parity/scenarios/w4/concurrency/POST-bills-bill_id-my-items.yaml` (`w4/concurrency/post-bills-bill_id-my-items`, 9 bước): `same_key_burst`, `no_key_same_body_burst` (xếp hàng ở khoá `bills`), `same_key_other_body_burst`.

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/post-bills-bill_id-my-items.yaml` (56 bước).

## Chưa phủ / lưu ý cho bản Go

- **Nhận hai món trở lên trong một lần**: đã phủ bằng `POST-bills-bill_id-my-items-many.yaml` (nhận 2, 3, 4 món, lặp khoá, nhả một phần, món lạ ở giữa, idempotency) sau khi làn DB phân định hàng hoà bằng id món đã đánh số.
- Xoá theo **người**, không theo món: gợi ý AI của người gọi trên món không nêu cũng mất. Bản Go phải xoá đúng tập đó (mọi share của `participant_id` trên các dòng của bill), không chỉ share `confirmed`.
- Bỏ trùng giữ thứ tự lần đầu; thứ tự INSERT theo đó.
- Món lạ là 422 thường ở đây, 409 viết hoa ở `PUT …/assignments`.
- Kiểm món lạ chạy trước khi xoá, trong khoá; bản Go phải kiểm trước DELETE để 422 không để lại thay đổi (dù rollback cũng che).
