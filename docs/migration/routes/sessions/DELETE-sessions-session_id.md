# DELETE /sessions/{session_id}

sessions · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đăng xuất **một thiết bị khác**. Phiên của người khác trả 404 chứ không 403: một 403 sẽ xác nhận rằng id đó đặt tên cho một phiên có thật (`routes/sessions.py:105-107`, `service.py:4519-4521`).

Khai báo **sau** `/sessions/current` để chữ `current` không bao giờ bị đọc thành một id.

## Xác thực và quyền

Thứ tự (`routes/sessions.py:95-109`, `service.py:4519-4527`):

1. Middleware idempotency: `DELETE` nằm trong `WRITE_METHODS` (`idempotency.py:76`); khoá rỗng bị từ chối trước routing.
2. Router: `/sessions/current` khớp trước; đuôi `/` → 307; `PATCH` → 405.
3. `get_actor` → 401 **trước** 422 của path; 422 `invalid_actor_id`, `invalid_actor_roles` (`deps.py:147`, `:152-154`).
4. Validation path `session_id`: UUID → 422 `uuid_parsing`.
5. `_require_permission("manage_own_sessions", actor, {"is_self": True})` (`service.py:4523`; `permissions.py:219`) → 403 `role_not_permitted` khi thiếu vai trò `member`.
6. `get_account_session(session_id)` (`repository.py:4621`). `None`, **hoặc** `record.person_id != actor.id` → 404 `session_not_found`, detail `Phiên này không còn.` Hai lý do một câu trả lời.

## Đầu vào

Path `session_id`: UUID. Không query, không thân.

## Đầu ra

**204**, không thân. Cùng lý do khai báo `-> Response` như route `current`.

## Tác dụng phụ

Đặt `revoked_at` trên đúng hàng được đặt tên (`repository.py:3574`). Không đụng phiên nào khác, kể cả phiên mà request này đang đi trên đó.

## Idempotency

`DELETE` nằm trong `WRITE_METHODS`. Lưu ý một bất đối xứng có thật: `get_account_session` (`repository.py:4621`) **không** lọc hàng đã thu hồi, trong khi `list_account_sessions` (`repository.py:4604-4619`) thì có. Nên thu hồi lần thứ hai trên cùng một id vẫn tìm thấy hàng, vẫn thuộc về người gọi, và vẫn trả 204 — dù hàng đó đã biến mất khỏi `GET /sessions`. Bước `friend_revokes_same_session_again` trong kịch bản là chỗ ghi lại sự thật đó, và bản Go phải chép đúng cả hai vế.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143` (dev), `:102`/`:106` (prod) |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:147`, `:152-154` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","session_id"]` | `main.py` handler |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:502-504`, `permissions.py:219` |
| 404 | `session_not_found` | `Phiên này không còn.` | `service.py:4526` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/sessions.py:95-109`
- Service: `services/api/app/api/service.py:4519-4527`
- Repository: `repository.py:4621` (`get_account_session`), `:3574` (`revoke_account_session`)
- Quyền: `services/api/app/domain/permissions.py:219`

## Test đang phủ

`services/api/tests/api/test_sessions_and_blocking.py`: `test_revoking_somebody_elses_session_is_the_same_404_as_a_made_up_id` (66), `test_the_session_list_is_only_ones_own_and_hides_dead_rows` (44).

## Kịch bản parity

`parity/scenarios/w9/sessions/DELETE-sessions-session_id.yaml`, id `w9/sessions/delete-sessions-session_id` (21 bước, `dev`):

- Trước khi có gì: `anonymous_revokes`, `anonymous_non_uuid_session`, `actor_id_not_uuid`, `owner_path_not_uuid`, `owner_unknown_session`.
- Fixture rồi một phiên cho `friend`: `owner_names_self` … `friend_redeems_invitation`, `friend_lists_before` (bind `/sessions/0/id`).
- Phiên của người khác là 404 chứ không 403: `stranger_revokes_friends_session`, `owner_revokes_friends_session`, `friend_lists_after_refused` (danh sách chưa suy suyển).
- Chủ hàng thì tiêu được: `friend_revokes_own_session`, `friend_lists_after_revoke`, `friend_revokes_same_session_again`.
- Thứ tự khai báo: `word_current_as_session_id` — chữ `current` tới handler kia, không tới route này.
- Framework: `trailing_slash`, `patch_not_allowed`.

Corpus sinh: `generated/w9-422/delete-sessions-session_id.yaml`.

## Chưa phủ / lưu ý cho bản Go

- Bản Go phải giữ **một** 404 cho cả «không có hàng» lẫn «hàng của người khác». Tách ra thành 403 là rò rỉ.
- Phiên hết hạn nhưng chưa thu hồi: không có bước (không lùi được đồng hồ). Lưu ý `get_account_session` không lọc `expires_at` — chỉ `list_account_sessions` lọc.
- Thứ tự route quan trọng: nếu bản Go đăng ký `/sessions/{session_id}` trước `/sessions/current`, chữ `current` sẽ thành 422 `uuid_parsing` thay vì tới cửa đăng xuất. Bước `word_current_as_session_id` là bước bắt lỗi đó.
