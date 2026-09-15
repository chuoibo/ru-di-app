# GET /people/me/contexts

people · core · trạng thái trong bộ nhớ: không có

## Mục đích

Danh sách cuộc trò chuyện của người gọi: mọi nhóm và pair người đó `active` hoặc `invited`, với tin mới nhất và số chưa đọc. Đọc từ roster và feed ở mỗi lần gọi; `X-Actor-Contexts` không bao giờ được hỏi (ADR-0014 §7).

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:48-62`, `services/api/app/api/service.py:4413-4421`), đo trên stack:

1. Router: đuôi `/` → 307; `POST` → 405.
2. `get_actor` → 401; 422 `invalid_actor_id`, `invalid_actor_roles`, `invalid_actor_contexts` (header vẫn được parse dù không dùng, `contexts_not_uuid`).
3. `_require_permission("view_own_contexts", {"is_self": True})` (`services/api/app/domain/permissions.py:187`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_lists_without_roles`).
4. Không tra hàng `people`: chưa đăng ký → 200 `{"contexts": []}` (`owner_lists_unregistered`).

## Đầu vào

Không path, không thân; query bị bỏ qua (`owner_lists_with_query`).

## Đầu ra

**200** `{"contexts": [ContextSummary, …]}` (`services/api/app/api/schemas.py:767-800`). Mỗi hàng, thứ tự khoá: `id`, `display_name`, `member_count`, `my_role`, `my_state`, `membership_id`, `joined_at`, `last_message`, `unread_count`, `theme`, `kind`, `counterpart`, `unavailable`.

Cách tính (`services/api/app/api/repository.py:3648-3809`, `service.py:4395-4411`, `:443-473`):

- Hàng: membership của người gọi có `state != 'left'` join `contexts`. Nhóm đã rời biến mất (`owner_leaves_beta`).
- `member_count`: số membership `active` của context (lời mời đang chờ không tính).
- `my_role` `admin`/`member`, `my_state` `active`/`invited`, `membership_id`, `joined_at` (null khi `invited`).
- `last_message`: tin mới nhất theo `(created_at DESC, id DESC)` (DISTINCT ON), `{id, kind, preview, author_id, author_display_name, created_at}`; `preview` từ `_message_preview` (tin đã xoá có `kind` `deleted`); tên tác giả tra lúc đọc (tác giả đã xoá tài khoản là `Người dùng đã rời`; tên rỗng/không có → null).
- `unread_count`: tin của người khác (và tin AI, `author_id` NULL) sau dấu đọc theo keyset `(created_at, id)` (`repository.py:3811-3827`); không có dấu đọc → mọi tin của người khác.
- `theme`: `contexts.theme`; `kind`: `group`/`pair`.
- Pair: `counterpart` = membership **khác `left`** của người kia `{id, display_name}`; `display_name` của hàng = tên người kia, rơi về `Thành viên` (`services/api/app/domain/direct.py:89-103`). Người kia đã xoá tài khoản → membership `left` → `counterpart` null, tên `Thành viên`.
- `unavailable` (chỉ pair có `counterpart`, `service.py:4403-4410`): `not dm_allowed(cạnh, other_deleted)` → `true` khi có cạnh `blocked` (ai chặn cũng vậy) hoặc người kia đã xoá. Nhóm luôn `false`; pair không còn `counterpart` cũng `false`.
- Thứ tự: hàng có tin trước, tin mới trước (`-created_at.timestamp()`, float giây), hàng không có tin sau; hoà thì theo `display_name` (so chuỗi Python, tức code point) (`repository.py:3799-3808`). Hai hàng cùng tên, cùng không có tin: thứ tự theo SQL (không xác định).

## Tác dụng phụ

Không. Một câu memberships+contexts, một đếm, một DISTINCT ON tin, một tên tác giả, một counterpart, một tên counterpart, rồi một đếm chưa đọc **mỗi hàng** và hai lần đọc mỗi pair.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | | `deps.py:144-164` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4420` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:48-62`
- Service: `services/api/app/api/service.py:4413-4421`, `:4395-4411`, `:443-473`
- Repository: `services/api/app/api/repository.py:3648-3827`
- Domain: `services/api/app/domain/direct.py:89-103`, `services/api/app/domain/blocking.py:67-74`
- Quyền: `services/api/app/domain/permissions.py:187`

## Test đang phủ

- `services/api/tests/api/test_person_contexts.py`: `test_lists_active_and_invited_groups_but_not_the_one_left` (57), `test_unread_counts_only_other_peoples_messages_and_the_mark_never_goes_back` (74), `test_the_conversation_list_orders_by_newest_message_then_name` (138)
- `services/api/tests/api/test_direct_messages.py`: `test_the_pair_appears_in_both_conversation_lists_named_after_the_other` (120), `test_a_blocked_pair_says_so_on_the_row_for_both_people` (237)
- `services/api/tests/postgres/test_person_contexts_postgres.py`: `test_the_list_has_active_and_invited_rows_with_real_membership_ids` (129), `test_unread_is_a_keyset_over_other_peoples_messages_and_the_mark_is_forward_only` (150)

## Kịch bản parity

`parity/scenarios/w10/people/GET-people-me-contexts.yaml`, id `w10/people/get-people-me-contexts` (50 bước, `dev`):

- Thứ tự: `anonymous_lists`, `actor_id_not_uuid`, `roles_unknown`, `contexts_not_uuid`, `owner_lists_without_roles` (`advancer,recipient`), `owner_lists_unregistered`, `owner_lists_nothing_yet`.
- Ba nhóm không tin: `owner_lists_without_messages` (hai nhóm `active` theo tên `Nhóm A` rồi `Nhóm Bê`, rồi `Nhóm Xê` `invited` cũng theo tên; `member_count` không tính lời mời), `owner_lists_claiming_contexts` (giống hệt).
- Tin: `owner_writes_beta`, `mate_writes_alpha_first`, `mate_writes_alpha_second`, `owner_themes_alpha`, `owner_lists_unread` (nhóm có tin mới nhất lên đầu, `preview` cắt ở 79 ký tự + `…`, `unread_count` 2, `theme`), `mate_lists_own_view` (tin của chính mình không chưa đọc), `owner_marks_first_read` (`PUT …/read-mark`), `owner_lists_after_mark` (1), `mate_deletes_second`, `owner_lists_after_delete` (`kind` `deleted`, `preview` `Tin nhắn đã bị xoá`).
- Pair: `owner_opens_pair`, `owner_lists_with_pair`, `friend_blocks_owner`, `owner_lists_blocked_pair`, `friend_lists_blocked_pair` (`unavailable: true` cả hai phía).
- Rời và xoá: `owner_leaves_beta` (hàng biến mất), `leaver_opens_pair`, `leaver_writes_pair`, `leaver_ends_account`, `owner_lists_after_counterpart_ended` (`Thành viên`, `counterpart` null, `member_count` 1, `unavailable` false, tác giả `Người dùng đã rời`), `leaver_lists_after_own_erasure` (rỗng).
- Framework: `owner_lists_with_query`, `trailing_slash`, `post_not_allowed` (405 `allow: GET`).

Đọc lại cũng có trong `POST-people-person_id-dm.yaml`, `POST-people-person_id-block.yaml`, `DELETE-people-person_id-block.yaml`, `DELETE-people-me-world.yaml` (`friend_lists_contexts_before`/`_after`, `mate_lists_contexts_after`, `leaver_lists_contexts_after` hiện lời mời mới tới id đã xoá), `crossreplay/POST-people-person_id-dm.yaml`, `concurrency/POST-people-person_id-dm.yaml`, `prod-auth.yaml` (`junk_bearer_lists_contexts`, `mate_lists_contexts`, `owner_lists_contexts_after`, `mate_lists_contexts_after`).

Corpus sinh: `generated/w10-422/get-people-me-contexts.yaml` (5 bước).

## Chưa phủ / lưu ý cho bản Go

- Sắp xếp là của Python sau truy vấn: khoá `(không có tin, -created_at.timestamp(), display_name)`; timestamp là float giây; tên so theo code point. Hai hàng cùng tên và cùng không có tin giữ thứ tự của câu SQL (không `ORDER BY`), không tất định.
- `preview`: `[Ảnh]`, `[Sticker]`, `Tin nhắn đã bị xoá`, nhãn thẻ AI, hoặc chữ đã strip, xuống dòng thành khoảng trắng, dài quá 80 thì 79 ký tự + `…` (`repository.py:2206-2225`).
- `unavailable` đọc thêm `get_friend_edge` và `get_person` cho mỗi pair có counterpart.
- Chưa phủ: `preview` ảnh, sticker, thẻ AI; tin AI (`author_id` null) được đếm chưa đọc; hai hàng cùng tên.

## Lỗi Python (chỉ báo, không sửa)

- Pair có người kia đã xoá tài khoản báo `unavailable: false` (xem thẻ `DELETE /people/me`).
- Tên hiển thị của pair khi người kia rời là `Thành viên`, trong khi `author_display_name` của tin cuối vẫn là `Người dùng đã rời`: một hàng hai tên cho cùng một người.
- Một truy vấn đếm chưa đọc cho mỗi hàng (N+1).
