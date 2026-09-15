# GET /contexts/{context_id}/batches

batches · core · trạng thái trong bộ nhớ: không có

## Mục đích

Danh sách các đợt thu của một nhóm, mới nhất trước, mỗi đợt tóm tắt từ chính bảng thu của nó (`GET /batches/{id}/obligations`) để hai màn không bao giờ lệch. Cho phép một thành viên tìm lại đợt sau khi điện thoại đã quên id. Chỉ đọc; mọi con số tính lại mỗi lần.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401 thắng lỗi path (`anonymous_non_uuid_context`); 422 `invalid_actor_id` (`actor_id_not_uuid`).
2. Path `context_id: UUID` → 422 (`stranger_non_uuid_context`).
3. `_require_permission("view_collection_board", {"is_group_member": is_member(context_id, actor)})` **trước mọi truy vấn đợt** (`services/api/app/api/service.py:6812-6816`; `services/api/app/domain/permissions.py:65`): nhóm không tồn tại và nhóm không phải của mình đều 403 `is_group_member` (`stranger_unknown_context`, `stranger_real_context`), người được mời (`invitee_reads`), người đã rời (`mate_after_leaving`); `X-Actor-Contexts` không được tin (`stranger_claims_context_header`); role rỗng → `role_not_permitted` (`mate_roles_empty`).

## Đầu vào

- Path `context_id` (UUID lax). Không query (`undeclared_query_ignored` → 200), không body.

## Đầu ra

- **200** `ContextBatchesResponse` (`services/api/app/api/schemas.py:2071-2073`), thứ tự khoá `context_id`, `batches`.
  - `batches[]`: `ContextBatchView` (`schemas.py:2053-2068`) theo thứ tự `batch_id`, `status`, `created_at`, `published_at`, `obligation_count`, `confirmed_count`, `disputed_count`, `total_vnd`.
  - Sắp `created_at DESC, id` (`services/api/app/api/repository.py:6928-6934`); `id` chỉ phá hoà hai đợt cùng thời điểm.
  - `status`: giá trị enum lưu trong `collection_batches.status` (`frozen` trong kịch bản; `published` sau publish).
  - `created_at`, `published_at`: đọc từ `timestamptz` (`published_at` `null` khi chưa gửi).
  - `obligation_count` = số nghĩa vụ phiên bản mới nhất; `confirmed_count` đếm `confirmed` **và** `over_confirmed`; `disputed_count` đếm `disputed`; `total_vnd` = tổng `amount_vnd` các nghĩa vụ (số nguyên Python) (`repository.py:6936-6955`).
- Nhóm chưa có đợt: `{"context_id":…,"batches":[]}` (`owner_no_rounds`).
- Giá trị trong kịch bản (dữ liệu mẫu, suy từ mã): đợt 1 một nghĩa vụ 50 000 (`owner_one_frozen_round`); đợt 2 do mate mở gồm owner → mate 30 000 và mate → owner 10 000, `total_vnd` 40 000, đứng trước đợt 1 (`mate_two_rounds_newest_first`); nhận đủ 50 000 ở đợt 1 → `confirmed_count` 1 (`owner_after_confirmed_receipt`); đợt 3 một nghĩa vụ 5 000 nhận `2**63 - 1` → `over_confirmed` vẫn đếm vào `confirmed_count` (`owner_three_rounds`).
- Framework: 307 cho `/` cuối; `POST` → 405.

## Tác dụng phụ

- Không ghi, không khoá; không idempotency (GET).
- N+1 có chủ ý: mỗi đợt gọi lại `list_batch_obligations` (`repository.py:6937`), tức thêm một `SELECT` receipts cho mỗi nghĩa vụ.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session` / `Session is not valid` | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /batches` | `deps.py:144-163` |
| 422 | (validation path) | `{"detail":[…]}` không có `input` | `services/api/app/api/main.py:318-351` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:6812-6816`, `:504` |

## Mã Python

- Route: `services/api/app/api/routes/batches.py:78-93`
- Service: `services/api/app/api/service.py:6800-6832` (`list_context_batches`)
- Repository: `services/api/app/api/repository.py:6916-6955`, `:6812-6914` (bảng thu dùng lại), `:2769-2782`
- Schema: `services/api/app/api/schemas.py:2044-2073`

## Test đang phủ

- `services/api/tests/api/test_context_batches.py`: `test_a_frozen_round_is_listed_before_it_is_published` (30), `test_publishing_and_a_confirmed_receipt_move_the_counts` (50), `test_a_group_with_no_round_gets_an_empty_list_not_an_error` (75), `test_somebody_from_another_group_cannot_read_it` (86), `test_an_unknown_group_refuses_the_same_way` (100) — repository giả
- `services/api/tests/postgres/test_repository_postgres.py::test_the_groups_rounds_are_listed_from_the_same_board_postgres` (851)

## Kịch bản parity

`parity/scenarios/w4/batches/GET-contexts-context_id-batches.yaml`, id `w4/batches/get-contexts-context_id-batches` (40 bước), lane DB bật:

- Thứ tự: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401), `actor_id_not_uuid`, `stranger_non_uuid_context` (422).
- 403: `stranger_unknown_context`, `stranger_real_context`, `invitee_reads`, `stranger_claims_context_header`, `mate_roles_empty`, `mate_after_leaving`.
- 200: `owner_no_rounds`, `owner_one_frozen_round`, `mate_two_rounds_newest_first`, `owner_after_confirmed_receipt`, `owner_three_rounds`.
- Dựng dữ liệu: `owner_batches_dinner`, `mate_batches_the_rest` (danh sách vắng), `owner_batches_lunch`, `owner_confirms_round_one`, `owner_confirms_int64_max`.
- Framework: `undeclared_query_ignored`, `trailing_slash_redirects`, `post_not_allowed`.

`parity/scenarios/w4/batches/prod-auth.yaml`: `basic_scheme_rounds` (401), `mate_lists_rounds` (200), `stranger_lists_rounds` (403).

Corpus 422 sinh tự động: `parity/scenarios/generated/w4-422/get-contexts-context_id-batches.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- `status: "published"`, `published_at` khác `null`, `confirmed_count` và `disputed_count` > 0: đã phủ bằng `GET-batches-batch_id-obligations-guest.yaml` (publish thật, khách báo đã chuyển và phản đối) và các bước danh sách đợt trong kịch bản publish thành công.
- `total_vnd` là tổng không giới hạn trong Python; nghĩa vụ bị chặn bởi phân bổ ≤ `10**12` mỗi khoản nên không vượt int64 trong thực tế, nhưng bản Go vẫn nên cộng an toàn.
- Giữ `confirmed_count` gồm cả `over_confirmed`.
- Bản Go có thể bỏ N+1 bằng một truy vấn gộp, miễn trạng thái vẫn tính theo đúng quy tắc của bảng thu (mọi receipts, không lọc người xác nhận).
