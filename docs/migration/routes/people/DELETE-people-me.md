# DELETE /people/me

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Kết thúc tài khoản của chính người gọi (ADR-0023 §2.1). «Xoá» ở đây không phải `DELETE FROM people`: hàng `people` giữ id (khoá ngoại của sổ tiền) và bị ẩn danh hoá; dữ liệu cá nhân bị xoá theo bản đồ đóng `ERASURE` (`services/api/app/domain/account_lifecycle.py:64-167`); mọi phiên bị thu hồi; mọi membership chưa rời thành `left`; mọi bảng tiền, tin nhắn, kỷ niệm, bình chọn, sổ đôi và báo cáo giữ nguyên từng byte. Tệp ảnh cá nhân và avatar bị gỡ khỏi kho sau khi các câu SQL đã chạy, **trước** commit.

`me` chứ không phải id: không có route xoá tài khoản của người khác.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:158-181`, `services/api/app/api/service.py:4528-4567`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`), vì DELETE là lệnh ghi: khoá rỗng → 422 `invalid_idempotency_key` trước mọi thứ (`anonymous_empty_idempotency_key`); khoá đã xong → phát lại 204 (`owner_replays_key`), khác thân → 422 `idempotency_key_reuse` (`owner_key_other_body`). Scope là `X-Actor-ID` (dev) hoặc digest bearer (prod).
2. Router: `DELETE /people/me/` → 307 (`trailing_slash`); `POST /people/me` → 405 `allow: GET` (`post_not_allowed`): Starlette chỉ lấy phương thức của route **đầu tiên** khớp path (`GET /people/me`), không gộp `PATCH` và `DELETE`. `PUT /people/me` không tới đây: nó khớp `PUT /people/{person_id}` và là 422 `uuid_parsing` (xem thẻ PUT).
3. FastAPI giải mã JSON trước dependency: thân hỏng → 422 `json_invalid` kể cả khi ẩn danh (`anonymous_malformed_json`).
4. `get_actor` (`services/api/app/api/deps.py:110-164`): dev thiếu `X-Actor-ID` → 401 (`anonymous_deletes`, `anonymous_confirm_false`), id không phải UUID → 422 `invalid_actor_id`, vai trò lạ → 422 `invalid_actor_roles`; prod → 401 `Missing bearer session` / `Session is not valid` (`services/api/app/api/service.py:4452-4474`), kể cả phiên của người đã xoá.
5. Model thân `AccountDeleteRequest` (`services/api/app/api/schemas.py:1040-1044`): thân bắt buộc, `confirm: StrictBool`, `extra=forbid`. Lỗi → 422 danh sách `detail` (không có `input`, `services/api/app/api/main.py:319-351`): thiếu thân `missing` `["body"]` (`ghost_body_missing`), `{}` `missing` `["body","confirm"]` (`ghost_empty_object`), `"true"`/`1`/`null` `bool_type` (`ghost_confirm_string`, `ghost_confirm_one`, `ghost_confirm_null`), khoá lạ `extra_forbidden` (`ghost_confirm_extra_field`), mảng `model_attributes_type` (`ghost_body_array`), `text/plain` cũng là `model_attributes_type` vì thân không được đọc như JSON (`ghost_text_plain`). Không có `content-type` thì thân được đọc như JSON (`ghost_confirm_true_no_content_type`).
6. `_require_permission("delete_own_account", {"is_self": True})` (`service.py:4540`; `services/api/app/domain/permissions.py:220`): thiếu `member` → 403 `permission_denied` `role_not_permitted`, **trước** kiểm `confirm` (`ghost_confirm_false_without_roles`, `ghost_confirm_true_without_roles`).
7. `check_confirmation` (`services/api/app/domain/account_lifecycle.py:247-250`): `confirm` khác `True` → 422 `confirm_required` `Cần xác nhận rõ ràng để xoá tài khoản.` (`service.py:4541-4548`; `ghost_confirm_false`, `owner_confirm_false_registered`).
8. `erase_person` (`services/api/app/api/repository.py:4770-4880`): không có hàng `people` → `RepositoryConflict("PERSON_NOT_FOUND")` → 404 `person_not_found` `Chưa có hồ sơ cho tài khoản này.` (`service.py:4549-4554`). Các câu DELETE/UPDATE trước đó đã chạy trong transaction nhưng bị rollback (`ghost_confirm_true_unregistered`: làn DB không đổi).
9. Gỡ tệp (`service.py:4555-4567`), rồi 204.

Người đã xoá gọi lại bằng header dev: không có lớp nào chặn (`get_actor` dev không đọc `deleted_at`), nên lần hai lại chạy toàn bộ và ghi thêm một `audit_events` (`twice_deletes_second`).

## Đầu vào

- Không path, không query (query lạ bị bỏ qua).
- Thân JSON đúng một khoá `confirm`, giá trị `true` literal.

## Đầu ra

**204**, thân rỗng, không `content-type`. Phát lại qua khoá header: 204, `content-length: 0`, `idempotency-replayed: true`.

## Tác dụng phụ (thứ tự, một transaction)

`erase_person` chạy theo khoá ngoại (`repository.py:4786-4880`), cùng một `now`:

| # | Bảng | Việc | Điều kiện |
|---|---|---|---|
| 0 | `uploaded_images` | SELECT `storage_key` | `owner_person_id = me` (ảnh `personal` và `avatar`; ảnh nhóm có `owner_person_id` NULL nên không nằm đây) |
| 1 | `post_comments` | DELETE | `author_id = me` **hoặc** `post_id` thuộc bài của me (bình luận của người khác dưới bài của me cũng mất) |
| 2 | `post_reactions` | DELETE | `person_id = me` hoặc `post_id` thuộc bài của me |
| 3 | `story_views` | DELETE | `viewer_id = me` hoặc `story_id` thuộc story của me |
| 4 | `posts` | DELETE | `author_id = me` |
| 5 | `stories` | DELETE | `author_id = me` |
| 6 | `uploaded_images` | DELETE | `owner_person_id = me` |
| 7 | `person_interests` | DELETE | `person_id = me` |
| 8 | `saved_places` | DELETE | `person_id = me` |
| 9 | `context_read_marks` | DELETE | `person_id = me` |
| 10 | `pair_paper_views` | DELETE | `person_id = me` |
| 11 | `pair_shared_constraints` | DELETE | `owner_id = me` |
| 12 | `active_couple_members` | DELETE | `person_id = me` (chỉ hàng của me; hàng của người kia ở lại) |
| 13 | `account_identities` | DELETE | `person_id = me` |
| 14 | `friend_requests` | DELETE | `requester_id = me` hoặc `addressee_id = me`, mọi trạng thái (pending, accepted, declined, blocked cả hai chiều) |
| 15 | `account_sessions` | UPDATE `revoked_at = now` | `person_id = me AND revoked_at IS NULL` (`repository.py:4625-4641`) |
| 16 | `memberships` | SELECT … FOR UPDATE rồi `state = 'left'`, `left_at = now` | `person_id = me AND left_at IS NULL` (cả `invited` và `active`; `joined_at` giữ) |
| 17 | `people` | SELECT … FOR UPDATE rồi UPDATE | `display_name = 'Người dùng đã rời'`, `bio`, `city`, `budget_band` NULL, `discoverable_by_phone = false`, `wall_comment_policy = 'nobody'`, `deleted_at = now`; `id`, `created_at`, `notify_prefs` giữ (`account_lifecycle.py:253-274`) |
| 18 | `audit_events` | INSERT | `actor_id = aggregate_id = me`, `event_type = 'account.deleted'`, `aggregate_type = 'person'`, `event_data = {"counts": {…}}`, `occurred_at = now` |

`event_data.counts` có đúng 16 khoá, kể cả giá trị 0: `post_comments`, `post_reactions`, `story_views`, `posts`, `stories`, `uploaded_images`, `person_interests`, `saved_places`, `context_read_marks`, `pair_paper_views`, `pair_shared_constraints`, `active_couple_members`, `account_identities`, `friend_requests`, `account_sessions`, `memberships`. Cột là JSONB nên Postgres lưu theo thứ tự khoá của nó (ngắn trước, rồi theo byte), không theo thứ tự chạy; đo ở làn DB: `{"posts": 3, "stories": 1, "memberships": 4, "story_views": 2, "saved_places": 2, "post_comments": 2, "post_reactions": 2, "friend_requests": 7, "uploaded_images": 3, "account_sessions": 0, "pair_paper_views": 1, "person_interests": 2, "account_identities": 0, "context_read_marks": 1, "active_couple_members": 1, "pair_shared_constraints": 1}`. Giá trị là  `rowcount` của mỗi DELETE (bình luận dưới bài của me đếm vào `post_comments`). Không tên, không chữ.

**Giữ nguyên** (không câu SQL nào chạm): `messages`, `message_reactions`, `memories`, `memory_comments`, `memory_reactions`, `outing_stop_checkins`, `votes`, `vote_options`, `vote_ballots`, `outings`, `outing_stops`, `outing_invites`, `contexts` (kể cả `created_by_id = me`), 20 bảng của `MONEY_TABLES` (`account_lifecycle.py:188-209`, gồm `guest_links`), `reports`, `audit_events` cũ, và mười bảng sổ đôi (`pair_notebooks`, `pair_notebook_cycles`, `pair_cycle_participants`, `pair_consent_proposals`, `pair_consents`, `pair_papers`, `pair_paper_versions`, `pair_paper_responses`, `pair_paper_outings`, `pair_paper_keeps`). Không chạm: `places`, `place_photos`, `destinations`, `idempotency_keys`, `otp_challenges`.

**Tệp** (`service.py:4555-4567`, `services/api/app/media/storage.py:64-79`): với mỗi `storage_key` đã SELECT ở bước 0, `PhotoStorage().delete(key)` gỡ `MOBILE_MEDIA_ROOT/<key[0:2]>/<key[2:4]>/<key>`; tệp không còn → bỏ qua; `OSError` → log cảnh báo, request vẫn 204. Thư mục con không bị gỡ. Ảnh nhóm (`/contexts/{id}/photos/{photo_id}`) người đó tải lên còn cả hàng lẫn tệp. Log `account.deleted: %d photo file(s) removed of %d`. Harness chưa so cây tệp: tệp chỉ được chứng minh gián tiếp qua 404 của `GET /people/{id}/photos/{photo_id}` và `/avatar` (hàng đã mất trước khi đọc tệp).

## Người khác thấy gì sau đó

Đo trong `DELETE-people-me-world.yaml` (dev):

- Nhóm người rời tạo: `GET /contexts/{id}` vẫn 200; roster không còn người rời trong danh sách `active`; `balances` bằng từng đồng trước và sau (`mate_reads_balances_before`/`_after`); tin nhắn, kỷ niệm, bình luận kỷ niệm, bình chọn giữ nguyên với tên `Người dùng đã rời`; đợt thu, nghĩa vụ, trang khách còn nguyên; ảnh nhóm 200.
- Bài của người rời biến mất khỏi feed, tường và `GET /posts/{id}` (404); bình luận của người rời dưới bài người khác mất, bình luận của người lạ ở lại; story mất.
- `GET /people/{id}` → 403 `person_not_visible`, cùng byte với id bịa (xem thẻ GET); `GET /people/{id}/avatar` → 403 `permission_denied` `shares_a_group_with_subject` (không còn nhóm chung); `GET /people/{id}/photos/{photo_id}` → 404 `photo_not_found`.
- Pair: hàng trong `GET /people/me/contexts` của người kia mất `counterpart`, tên thành `Thành viên`, `member_count` 1, `unavailable` **false**, tác giả tin cuối `Người dùng đã rời`; `GET /contexts/{pair}` cũng tên `Thành viên`; `POST …/messages` vào pair → 409 `direct_message_unavailable`; sổ (`cycle_state: active`, chỉ còn ràng buộc của người kia) và tờ (`da_giu`, cả hai dòng giữ) vẫn đọc được; mở lại DM → 404.
- Người chặn không còn thấy người rời trong `GET /people/me/blocked`; lời mời kết bạn đang chờ biến mất.
- Người khác vẫn gửi được lời mời kết bạn tới id đã xoá và mời id đó vào nhóm (xem Lỗi Python).

## Idempotency

Khoá header: 204 được lưu và phát lại (body rỗng, `media_type` NULL). Phát lại **không** chạy xoá lần hai. Trong prod, phát lại theo khoá vẫn trả 204 sau khi phiên đã bị thu hồi, vì middleware trả trước `get_actor` (`prod-auth.yaml` `owner_replays_erasure_key`). Không khoá: gọi lại (dev) chạy lại toàn bộ và ghi thêm một audit event; prod thì 401.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `deps.py:93-143`; `service.py:4452-4474` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | `X-Actor-ID must be a UUID` / `X-Actor-Roles contains an unknown role` | `deps.py:144-155` |
| 422 | (validation) | `{"detail":[…]}`: `json_invalid`, `missing`, `bool_type`, `extra_forbidden`, `model_attributes_type` | `main.py:319-351` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:475-505` |
| 422 | `confirm_required` | `Cần xác nhận rõ ràng để xoá tài khoản.` | `service.py:4541-4548` |
| 404 | `person_not_found` | `Chưa có hồ sơ cho tài khoản này.` | `service.py:4549-4554` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | `Idempotency-Key must be 1..255 characters` / `Idempotency-Key was already used for a different request` | `idempotency.py:432-481` |
| 307 / 405 | — | thân rỗng / `{"detail":"Method Not Allowed"}` | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:158-181`
- Service: `services/api/app/api/service.py:4528-4567`
- Repository: `services/api/app/api/repository.py:4770-4880` (`erase_person`), `:4625-4641` (`revoke_all_account_sessions`), `:3585-3646` (`actor_grants`, lớp hai của prod)
- Domain: `services/api/app/domain/account_lifecycle.py` (`ERASURE`, `check_confirmation`, `anonymised_person`)
- Kho tệp: `services/api/app/media/storage.py:64-79`
- Quyền: `services/api/app/domain/permissions.py:220`

## Test đang phủ

- `services/api/tests/api/test_delete_account.py`: `test_the_door_will_not_open_without_a_literal_confirmation` (66), `test_the_row_stays_anonymised_and_everything_personal_goes` (73), `test_the_ended_account_stops_being_a_person_the_product_answers_about` (95), `test_a_session_of_an_ended_account_is_not_an_actor` (120), `test_the_group_itself_survives_the_person_leaving_it` (128)
- `services/api/tests/postgres/test_delete_account_postgres.py`: `test_the_ledger_does_not_move_when_an_account_ends` (57), `test_the_map_names_every_table_the_schema_has` (153), `test_a_private_conversation_stops_taking_messages_when_one_side_ends` (171), `test_an_ended_id_cannot_be_claimed_or_renamed_by_anybody` (234), `test_both_reasons_a_pair_dies_answer_with_the_very_same_bytes` (288), `test_the_notebook_of_two_survives_one_of_them_ending` (350)

## Kịch bản parity

`parity/scenarios/w10/people/DELETE-people-me.yaml`, id `w10/people/delete-people-me` (35 bước, `dev`):

- Thứ tự dây: `anonymous_deletes`, `anonymous_malformed_json`, `anonymous_confirm_false`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`.
- Model thân (người gọi chưa đăng ký, không ghi gì): `ghost_body_missing`, `ghost_empty_object`, `ghost_confirm_string`, `ghost_confirm_one`, `ghost_confirm_null`, `ghost_confirm_extra_field`, `ghost_body_array`, `ghost_text_plain`.
- Vai trò trước `confirm`, `confirm` trước hàng: `ghost_confirm_false_without_roles`, `ghost_confirm_true_without_roles`, `ghost_confirm_false`, `ghost_confirm_true_unregistered`, `ghost_confirm_true_no_content_type` (404, rollback).
- Khoá header: `owner_deletes_with_key` (204, một `audit_events`), `owner_replays_key`, `owner_key_other_body`.
- Sau khi xoá (dev): `owner_reads_after` (200 hàng ẩn danh), `owner_reads_own_public_profile_after` (404), `owner_reclaims_name_after` (404 `Chưa có ai dùng số này trong Rủ Đi.`), `twice_deletes_first`, `twice_deletes_second` (204 và audit thứ hai), `twice_reads_after`.
- Framework: `trailing_slash`, `post_not_allowed`.

`parity/scenarios/w10/people/DELETE-people-me-world.yaml`, id `w10/people/delete-people-me-world` (132 bước, `dev`): dựng đời sống đầy đủ (bước 1-87), đọc trước (88-91), xoá (`leaver_ends_account`), đọc sau (93-132). Làn DB của bước xoá: `-post_comments:2 -post_reactions:2 -story_views:2 -posts:3 -stories:1 -uploaded_images:3 -person_interests:2 -saved_places:2 -context_read_marks:1 -pair_paper_views:1 -pair_shared_constraints:1 -active_couple_members:1 -friend_requests:7 ~memberships:4 ~people:1 +audit_events:1`, không một hàng nào ở bảng tiền, tin nhắn, kỷ niệm, bình chọn, sổ đôi, báo cáo. Đọc sau: `mate_reads_balances_after` bằng `mate_reads_balances_before` từng đồng; `guest_opens_link_after` in `Người dùng đã rời đã ghi`; `friend_lists_contexts_after`, `friend_reads_pair_after`, `friend_writes_in_pair_after` (409), `friend_reads_notebook_after`, `friend_reads_sheet_after`; `stranger_asks_leaver_after` và `mate_invites_leaver_back` (201).

Cũng xoá tài khoản làm bước dựng: `GET-people-me.yaml`, `GET-people-me-blocked.yaml`, `GET-people-me-contexts.yaml`, `GET-people-me-saved-places.yaml`, `GET-people-person_id.yaml`, `PATCH-people-me.yaml`, `POST-people-person_id-block.yaml`, `POST-people-person_id-dm.yaml`, `PUT-people-person_id.yaml`.

`crossreplay/DELETE-people-me.yaml` (10 bước): 422 `confirm_required` nhả khoá; 204 lưu qua Python được cửa trước phát lại và ngược lại, không xoá lần hai (một audit mỗi người); thân khác cùng khoá 422.

`concurrency/DELETE-people-me.yaml` (6 bước): ba lần xoá cùng lúc → ba 204, ba `audit_events` (một hàng đếm `saved_places: 1`, hai hàng đếm 0), `people` cập nhật một lần trong delta; ba lần cùng khoá → một xoá, hai phát lại.

`prod-auth.yaml` (60 bước, `prod`): `anonymous_malformed_json_delete` (422 trước 401), `owner_confirm_false`, `owner_ends_account_with_key` (204, `account_sessions.revoked_at` ghi ở làn DB), `owner_replays_erasure_key` (204 phát lại sau khi phiên đã chết), `owner_ends_account_again` và mọi route sau đó của `owner` → 401 `Session is not valid`; `mate_reads_owner_after` 403, `mate_opens_pair_after` 404, `mate_names_owner_after` 404, `mate_blocks_owner_after` 200.

Corpus sinh: `parity/scenarios/generated/w10-422/delete-people-me.yaml` (38 bước).

## Chưa phủ / lưu ý cho bản Go

- Thứ tự SQL là một phần của hợp đồng: DELETE bình luận và phản ứng chạy trước DELETE bài nên bình luận của người khác dưới bài của người rời đếm vào `post_comments`; `event_data.counts` có 16 khoá kể cả 0 và làn DB so văn bản JSONB đã chuẩn hoá. Mọi cột thời gian dùng cùng một `now`.
- Chọn `storage_key` **trước** các DELETE; gỡ tệp sau các câu SQL nhưng trước commit (xem Lỗi Python). Không gỡ thư mục. Lỗi `OSError` chỉ log.
- Không kiểm `deleted_at` ở `get_actor` dev; prod thì `actor_for_session_token` và `actor_grants` đều trả 401 cho người đã xoá (phiên đã bị thu hồi ở bước 15 là lớp một).
- Phát lại 204 qua cửa trước Go không có `content-length: 0` (ngoại lệ đã duyệt RESPONSE-204-CONTENT-LENGTH).
- Chưa phủ: `account_identities` (cần OTP hoặc Google), nhiều phiên cho một người trong prod (harness chỉ gieo một phiên mỗi persona), `outing_stop_checkins`, `OSError` khi gỡ tệp, commit thất bại sau khi gỡ tệp, cây tệp trên đĩa (harness chưa so), tin do AI viết (`author_id` NULL) trong nhóm của người rời.
- Kho ảnh: mỗi phía có một thư mục media trên host dùng chung giữa core và Python của nó; bản Go gỡ tệp phải dùng đúng đường dẫn `<root>/<key[0:2]>/<key[2:4]>/<key>` và bỏ qua tệp đã mất.

## Lỗi Python (chỉ báo, không sửa)

- Tệp ảnh bị gỡ **trước** commit: `delete_own_account` gỡ ngay sau `erase_person` (`service.py:4555-4567`), còn commit chạy khi dependency `get_repository` đóng (`services/api/app/api/deps.py:196-209`). Docstring của `erase_person` (`repository.py:4779-4783`) nói điều ngược lại. Commit thất bại → hàng còn, tệp mất.
- Chỉ hàng `active_couple_members` của người rời bị xoá; người còn lại vẫn là «một nửa cặp đôi», chu kỳ sổ vẫn `active` với `participants` gồm người đã rời, `their_consents_granted` vẫn true (`friend_reads_notebook_after`).
- Nhóm do người rời tạo mà người rời là admin duy nhất còn lại không có admin nào (`mate_reads_roster_after`: chỉ còn `member`).
- Id đã xoá vẫn nhận được lời mời kết bạn (201, `stranger_asks_leaver_after`), lời mời vào nhóm (201, `mate_invites_leaver_back`) và bị chặn (200, `mate_blocks_owner_after`, `owner_blocks_ended_account`), cả trong prod.
- Hàng pair của người còn lại sau khi người kia xoá tài khoản: `counterpart` null, `unavailable` **false**, trái ADR-0023 §2.3.2 («người kia đã xoá tài khoản» → `unavailable: true`), vì truy vấn counterpart bỏ membership `left` và `_context_summaries` bỏ qua pair không có counterpart (`service.py:4403-4405`).
- Trong dev, header của người đã xoá vẫn đọc hàng ẩn danh (`GET /people/me` 200), đổi tên và mở lại `discoverable_by_phone`/`wall_comment_policy` (`PATCH`), xoá lần nữa với audit thứ hai; nhưng `GET /people/{chính mình}` là 404: hai cửa hồ sơ trả lời khác nhau cho cùng người.
- Người gọi chưa đăng ký với `confirm: true`: toàn bộ DELETE/UPDATE chạy rồi mới biết không có hàng (404, rollback).
