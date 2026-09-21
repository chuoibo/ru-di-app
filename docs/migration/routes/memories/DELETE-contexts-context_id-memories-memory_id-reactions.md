# DELETE /contexts/{context_id}/memories/{memory_id}/reactions

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Rút lại tim **của chính mình** trên một dòng tường kỷ niệm. Không có đường nào để gỡ tim của người khác: người bị gỡ luôn là actor (`services/api/app/api/service.py:5180-5197`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_both`).
3. Hai path → một mảng 422 (`stranger_non_uuid_both`).
4. `_memory_of_member(…, "post_group_memory")` (`service.py:5116-5145`; `permissions.py:423-426`): 403 trước lookup (`stranger_unknown_both`, `stranger_real_memory`, `invitee_takes_back`, `leaver_takes_back_after_leaving`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 204 (`mate_roles_group_admin_only`).
5. Memory không thuộc nhóm của path → 404 `memory_not_found` (`mate_unknown_memory`, `mate_other_groups_memory_via_this_context`).
6. `remove_memory_reaction` (`services/api/app/api/repository.py:5356-5371`): actor chưa có tim → 404 `reaction_not_found` (`mate_never_hearted`, `owner_takes_back_again`).

## Đầu vào

- Path `context_id`, `memory_id` (UUID lax).
- Không body: body bị bỏ qua, kể cả JSON hỏng (`mate_malformed_body_ignored` → 204).
- Header: `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **204**, thân rỗng (`services/api/app/api/routes/memories.py:196-197`).
- Replay: 204 + `idempotency-replayed: true` + `content-length: 0` do middleware; qua `core` mất `content-length`, ngoại lệ đã duyệt `RESPONSE-204-CONTENT-LENGTH` (ADR-0029 §2.4), harness đếm `accepted` (`idem_replay`).
- Framework: 307 cho `/` cuối; `PUT` → 405 `allow: POST` (route POST cùng path được đăng ký trước).

## Tác dụng phụ

- `SELECT memory_reactions … one_or_none()` theo `(memory_id, person_id = actor)`, rồi DELETE dòng đó (`repository.py:5356-5371`). Tim của người khác không đổi (`owner_takes_back` chỉ gỡ tim owner; `owner_reads_wall` còn tim của mate và leaver).
- Tim của người đã rời nằm lại và họ không gỡ được (`leaver_takes_back_after_leaving` → 403).
- SELECT `memberships`, `memories` + hai truy vấn đếm.
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 404 nhả key (`idem_refusal_not_stored` → `mate_hearts_once_more` → `idem_same_key_after_refusal` 204). Fingerprint gồm path (`idem_reuse_other_memory` → 422, dù actor không phải thành viên nhóm kia: middleware trả lời trước xác thực).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5137-5141`, `:504` |
| 404 | `memory_not_found` | `Memory does not exist` | `service.py:5142-5144` |
| 404 | `reaction_not_found` | `This person has not reacted to this memory` | `service.py:5194-5197` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

## Mã Python

- Route: `services/api/app/api/routes/memories.py:171-197`
- Service: `services/api/app/api/service.py:5180-5197` (`unreact_to_memory`), `:5116-5145`
- Domain: `services/api/app/domain/permissions.py:423-426`
- Repository: `services/api/app/api/repository.py:5356-5371`, `:5307-5327`
- Model: `services/api/app/db/models.py:1872-1921`

## Test đang phủ

- `services/api/tests/postgres/test_memory_reactions_postgres.py`: `test_a_member_can_take_their_heart_back` (460), `test_taking_back_a_heart_that_was_never_left_is_a_404` (473), `test_one_member_cannot_remove_another_members_heart` (484)
- `services/api/tests/api/test_route_declarations_under_pinned_fastapi.py`, `services/api/tests/api/test_bodyless_status_declarations.py` (khai báo 204 dưới FastAPI đã ghim)

## Kịch bản parity

`parity/scenarios/w3/memories/DELETE-contexts-context_id-memories-memory_id-reactions.yaml`, id `w3/memories/delete-contexts-context_id-memories-memory_id-reactions` (46 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown`, `anonymous_non_uuid_both`, `anonymous_empty_idempotency_key`, `stranger_non_uuid_both`.
- 403: `stranger_unknown_both`, `stranger_real_memory`, `invitee_takes_back`, `mate_roles_empty`, `leaver_takes_back_after_leaving`.
- 404: `mate_unknown_memory`, `mate_other_groups_memory_via_this_context` (`memory_not_found`), `mate_never_hearted`, `owner_takes_back_again` (`reaction_not_found`).
- Đường vui: `mate_hearts_photo`, `owner_hearts_photo`, `leaver_hearts_photo`, `owner_takes_back`, `owner_reads_wall`, `mate_malformed_body_ignored`, `mate_hearts_again`, `mate_roles_group_admin_only`.
- Idempotency: `mate_hearts_for_idempotency`, `idem_first`, `idem_replay` (204 replay), `idem_reuse_other_memory`, `idem_refusal_not_stored` (404) + `mate_hearts_once_more` + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects`, `put_not_allowed`.

`prod` (`w3/memories/prod-auth`): `owner_takes_back_heart` (204).

Chạy với làn DB bật; `cursor` ở các bước chuẩn bị `owner_posts_photo`, `owner_posts_in_other_group` và ở `owner_reads_wall` được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` như ở card `POST /contexts/{id}/memories` (ADR-0029 §2.4). Corpus 422 sinh tự động: `parity/scenarios/generated/w3-422/delete-contexts-context_id-memories-memory_id-reactions.yaml` (32 bước).

## Chưa phủ / lưu ý cho bản Go

- Hai lần gỡ đồng thời chưa phủ; Python đọc rồi xoá không khoá, nên lần thứ hai có thể 204 hoặc 404 tuỳ lịch.
- Header `allow` của 405 là `POST`, không phải `DELETE, POST`.
- `content-length: 0` khi replay 204 là ngoại lệ đã duyệt; bản Go không được thêm header đó cho 204 lần đầu.
