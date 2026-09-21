# POST /contexts/{context_id}/memories/{memory_id}/reactions

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

F40: thả **một** tim lên một dòng của tường kỷ niệm. Người thả là actor; route không có body nên không có trường nào để nêu tên người khác. Tổng tim trong câu trả lời đếm lại từ các dòng sau khi ghi, không cộng dồn (`services/api/app/api/service.py:5147-5178`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_both`).
3. Hai path `context_id`, `memory_id` → một mảng 422, `context_id` trước (`stranger_non_uuid_both`, `stranger_non_uuid_memory`).
4. `_memory_of_member` (`service.py:5116-5145`): `_require_permission("post_group_memory", {"is_group_member": is_member(context_id, actor)})` (`permissions.py:423-426`) **trước** khi tra memory. Người lạ nhận cùng một 403 dù memory có thật hay không (`stranger_unknown_both`, `stranger_real_memory`, `stranger_unknown_memory_real_context`); người được mời (`invitee_hearts`), người đã rời (`leaver_hearts_after_leaving`) → 403; role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 201 (`owner_roles_group_admin_only`).
5. `get_context_memory(context_id, memory_id)` (`services/api/app/api/repository.py:5307-5327`) lọc cả `memories.context_id`: không có → 404 `memory_not_found` (`mate_unknown_memory`); memory của nhóm khác qua path nhóm này → 404 (`mate_other_groups_memory_via_this_context`), kể cả khi actor là thành viên cả hai nhóm (`owner_this_groups_memory_via_other_context`).
6. `add_memory_reaction` (`repository.py:5329-5354`): INSERT trong savepoint; vi phạm `uq_memory_reactions_person` → 409 `already_reacted` (`service.py:5160-5168`; `mate_hearts_photo_again`). Không kiểm trước.

## Đầu vào

- Path `context_id`, `memory_id` (UUID lax).
- Không body: body bị bỏ qua, kể cả body nêu người khác (`mate_body_naming_owner_ignored` → tim của mate) và JSON hỏng (`owner_malformed_body_ignored` → 201).
- Header: `Idempotency-Key` tuỳ chọn.

## Đầu ra

- **201** `MemoryReactionResponse` (`services/api/app/api/schemas.py:1532-1547`), thứ tự khoá `id`, `memory_id`, `person_id`, `created_at`, `reaction_count`.
  - `person_id`: actor; `created_at`: `_now()` Python (`service.py:5157`), pydantic UTC `Z`.
  - `reaction_count`: đọc lại memory và đếm (`service.py:5177`, `:5199-5207`), gồm cả tim của người đã rời.
- Check-in cũng thả tim được (`mate_body_naming_owner_ignored` trên `checkin`).
- Replay: 201 + body đã lưu (kể cả `reaction_count` cũ) + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `GET` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `memory_reactions (id, memory_id, person_id, created_at)` trong savepoint; 409 để lại không dòng nào. Unique `uq_memory_reactions_person (memory_id, person_id)`, FK `memories.id` ON DELETE CASCADE (`services/api/app/db/models.py:1872-1921`).
- SELECT: `memberships`, `memories` + đếm (tra lần một), `memories` + đếm (đếm lại). Mỗi lần tra có hai `GROUP BY` (`repository.py:5264-5305`).
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 409 nhả key (`idem_refusal_not_stored` → `mate_takes_back_heart_again` → `idem_same_key_after_refusal` 201). Fingerprint gồm path (`idem_reuse_other_memory` → 422).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:5137-5141`, `:504` |
| 404 | `memory_not_found` | `Memory does not exist` | `service.py:5142-5144` |
| 409 | `already_reacted` | `This person has already reacted to this memory` | `service.py:5162-5167`; `repository.py:5343-5353` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

## Mã Python

- Route: `services/api/app/api/routes/memories.py:148-168`
- Service: `services/api/app/api/service.py:5147-5178` (`react_to_memory`), `:5116-5145` (`_memory_of_member`), `:5199-5207` (`_reaction_count`)
- Domain: `services/api/app/domain/permissions.py:423-426`
- Repository: `services/api/app/api/repository.py:5329-5354`, `:5307-5327`, `:5264-5305`
- Model: `services/api/app/db/models.py:1872-1921`

## Test đang phủ

- `services/api/tests/postgres/test_memory_reactions_postgres.py`: `test_a_member_leaves_a_heart_and_it_is_their_own` (146), `test_a_body_naming_another_person_changes_nothing` (161), `test_an_outsider_cannot_leave_a_heart` (270), `test_an_invited_member_cannot_react_or_comment` (300), `test_a_departed_member_cannot_react_or_comment` (323), `test_a_declared_role_is_not_a_key` (346), `test_membership_is_read_from_the_database_not_from_the_header` (359), `test_the_same_person_cannot_react_twice` (394), `test_the_index_itself_refuses_a_duplicate_row` (419), `test_two_members_may_each_leave_their_own_heart` (438), `test_one_person_may_like_several_memories` (451), `test_another_groups_memory_is_not_reachable_through_this_context` (600), `test_an_outsider_learns_nothing_about_which_ids_exist` (624), `test_a_member_asking_for_an_unknown_memory_gets_404_without_an_echo` (643)

## Kịch bản parity

`parity/scenarios/w3/memories/POST-contexts-context_id-memories-memory_id-reactions.yaml`, id `w3/memories/post-contexts-context_id-memories-memory_id-reactions` (48 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown`, `anonymous_non_uuid_both`, `anonymous_empty_idempotency_key`, `stranger_non_uuid_both`, `stranger_non_uuid_memory`.
- 403 trước 404: `stranger_unknown_both`, `stranger_real_memory`, `stranger_unknown_memory_real_context`, `invitee_hearts`, `mate_roles_empty`, `leaver_hearts_after_leaving`.
- 404: `mate_unknown_memory`, `mate_other_groups_memory_via_this_context`, `owner_this_groups_memory_via_other_context`.
- Đường vui và 409: `mate_hearts_photo`, `mate_hearts_photo_again` (409), `owner_hearts_photo`, `mate_body_naming_owner_ignored`, `owner_malformed_body_ignored`, `leaver_hearts_photo`, `owner_roles_group_admin_only`, `owner_reads_wall`.
- Idempotency: `mate_takes_back_heart`, `idem_first`, `idem_replay`, `idem_reuse_other_memory`, `idem_refusal_not_stored` (409) + `mate_takes_back_heart_again` + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects`, `get_not_allowed`.

`prod` (`w3/memories/prod-auth`): `anonymous_non_uuid_reaction` (401), `owner_hearts_photo` (201), `owner_hearts_photo_again` (409).

Chạy với làn DB bật; `cursor` ở các bước chuẩn bị (`owner_posts_photo`, `owner_checks_in`, `owner_posts_in_other_group`) và ở `owner_reads_wall` được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` như ở card `POST /contexts/{id}/memories` (ADR-0029 §2.4). Corpus 422 sinh tự động: `parity/scenarios/generated/w3-422/post-contexts-context_id-memories-memory_id-reactions.yaml` (32 bước).

## Chưa phủ / lưu ý cho bản Go

- Hai lần thả tim đồng thời (một 201, một 409 qua savepoint) chưa phủ; lần thả lại tuần tự đi qua nhánh `IntegrityError`, nên bản Go phải cho 409 chứ không 500 hay 201.
- `reaction_count` là đếm sau khi ghi trong cùng transaction; replay trả số cũ.
- Membership phải được hỏi trước lookup, nếu không route thành oracle cho id memory.
