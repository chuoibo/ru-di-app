# POST /sessions

sessions · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đổi một lời mời **đích danh** lấy một phiên. Đây là route duy nhất của sản phẩm trả lời mà không cần danh tính, vì nó chính là nơi danh tính được cấp — hỏi danh tính ở đây là hỏi đúng thứ đang được xin (`services/api/app/api/routes/sessions.py:1-19`).

Thứ làm nó an toàn: **người gọi không nói gì về mình**. Thân chỉ mang một bí mật, không có `person_id`. Phiên được cấp cho `outing_invites.invited_person_id` — giá trị mà một thành viên đã ghi khi đặt tên lời mời — nên sở hữu bí mật là lời khai về **một người đã có người khác bảo lãnh**, không phải lời khai người giữ tự nói về mình. Một route nhận cả token lẫn tên sẽ quay lại chỗ tin client về danh tính, chỉ là với một credential dài hơn: đúng cái lỗ ADR-0014 tồn tại để bịt (`schemas.py:970-977`).

Nó **không** biến ai thành thành viên. Membership tạo ở đây là `INVITED`, y như cửa link để lại; dữ liệu nhóm vẫn nằm sau lần duyệt cũ.

## Xác thực và quyền

Không có `get_actor`. Thứ tự (`routes/sessions.py:41-54`, `service.py:3731-3800`):

1. Middleware idempotency: `POST` nằm trong `WRITE_METHODS` (`services/api/app/api/idempotency.py:76`, kiểm tại `:405`), nên `idempotency-key` rỗng bị từ chối **trước cả routing**. Phạm vi là `anonymous` (`idempotency.py:78`) vì route này không có actor.
2. Router: đuôi `/` → 307; `PATCH` → 405.
3. Model pydantic `SessionBootstrapRequest` (`schemas.py:970-979`): đúng một trường `invite_token`, `StrictStr`, `min_length=1`, `max_length=512`. Thiếu, `null`, không phải chuỗi, rỗng, hay dài quá đều là 422 trước khi chạm repository.
4. `get_outing_invite_by_digest(token_digest(token))` (`repository.py:3405`). **Mọi** lý do chết đều là một 404 `invite_not_found` với nguyên văn tiếng Anh `Invite link is not valid`:
   - không có hàng nào mang digest đó;
   - hàng là lời mời **link** (`invited_person_id IS NULL`) — link không đặt tên ai nên không có người để cấp phiên; đây là bản đối xứng của việc bí mật đích danh bị từ chối ở cửa link, hai cửa không được chung một lần đổi;
   - `revoked_at` khác NULL, hoặc `expires_at <= now`;
   - chuyến sau lời mời không còn (`get_outing` trả None) — một token không được tiết lộ thứ sau nó có tồn tại hay không;
   - `consume_named_invite_secret` ném `RepositoryConflict`, tức bí mật đã bị tiêu (`repository.py:3495`).

Không có bậc 403 nào trên route này: một 403 sẽ xác nhận rằng id là thật.

## Đầu vào

Thân JSON, đúng một trường:

| trường | kiểu | ràng buộc |
|---|---|---|
| `invite_token` | `StrictStr` | `min_length=1`, `max_length=512` |

Không path, không query. **Không có `person_id`** — xem `schemas.py:971-977`.

## Đầu ra

**201** `SessionResponse` (`schemas.py:1047-1070`), đúng thứ tự khoá: `token`, `person_id`, `expires_at`, `issued_via`, `is_new_person`, `profile`, `contexts`, và nhóm mà lời mời thuộc về.

- `token` là bí mật thô, trao **đúng một lần**; kho chỉ giữ SHA-256 của nó (`repository.py:3536`).
- `issued_via` là `invite` cho cửa này. Ràng buộc CHECK `invite_matches_via` ở `db/models.py:2828-2830` bắt `(issued_via = 'invite') = (issued_from_invite_id IS NOT NULL)`, nên Go không được ghi `invite` mà bỏ trống `issued_from_invite_id`.
- `is_new_person` là `False` ở cửa này: người đã tồn tại từ lúc thành viên kia đặt tên lời mời.
- Nhóm mà lời mời thuộc về được mang theo, vì một phiên thiếu nó là phiên không hiển thị được gì.

## Tác dụng phụ

Trong một transaction: tiêu bí mật (`consume_named_invite_secret`, `repository.py:3495`), bảo đảm một membership `INVITED` với `origin="named"` (`ensure_invited_membership`, `repository.py:4464`), và tạo một hàng `account_sessions` (`repository.py:3536`). `origin="named"` là thứ `accept_context_membership` đọc về sau để quyết người được mời có tự đồng ý cho mình được không.

## Idempotency

`POST` nằm trong `WRITE_METHODS`, nên middleware áp dụng. Khoá rỗng bị từ chối trước routing. Một lần đổi thứ hai với **cùng** khoá phát lại câu trả lời đã lưu; không có khoá thì lần thứ hai gặp `RepositoryConflict` và thành 404.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 422 | (validation) | `missing`, `string_type`, `string_too_short`, `string_too_long`, `loc` `["body","invite_token"]` | `main.py` handler + `schemas.py:979` |
| 422 | `invalid_body` | thân không phải JSON | Starlette/FastAPI |
| 404 | `invite_not_found` | `Invite link is not valid` | `service.py:3761`, `:3764`, `:3770`, `:3783` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/sessions.py:41-54`
- Service: `services/api/app/api/service.py:3731-3800`
- Repository: `repository.py:3405` (`get_outing_invite_by_digest`), `:3495` (`consume_named_invite_secret`), `:4464` (`ensure_invited_membership`), `:3536` (`create_account_session`)
- Schema: `services/api/app/api/schemas.py:970-979`, `:1047-1070`
- Model + ràng buộc: `services/api/app/db/models.py:1424-1430` (`uq_outing_invites_person`), `:2828-2830` (`invite_matches_via`)

## Test đang phủ

`services/api/tests/postgres/test_session_bootstrap_postgres.py`: `test_a_named_invitation_becomes_a_session_for_the_person_it_names` (161), `test_the_stored_row_holds_a_digest_and_never_the_token` (208), `test_the_same_secret_cannot_be_spent_twice` (230), `test_a_forwardable_link_cannot_become_a_session` (259), `test_a_named_secret_is_refused_at_the_link_door` (292), `test_a_second_named_invitation_is_refused_which_is_why_rotation_exists` (321), `test_rotation_kills_the_old_secret_and_signs_the_same_person_back_in` (334), `test_an_active_member_who_signs_in_again_stays_active` (377), `test_the_session_names_the_group_the_invitation_belonged_to` (572), `test_phien_mang_dung_membership_va_nguoi_do_tu_dong_y_duoc` (672).

## Kịch bản parity

`parity/scenarios/w9/sessions/POST-sessions.yaml`, id `w9/sessions/post-sessions` (23 bước, `dev`):

- Từ chối không cần fixture: `no_body`, `body_not_object`, `body_broken_json`, `token_missing`, `token_null`, `token_not_string`, `token_empty`, `token_unknown`.
- Fixture: `owner_names_self`, `owner_names_friend`, `owner_names_mate`, `owner_creates_group`, `owner_creates_trip`, `owner_mints_a_link`, `owner_invites_friend_by_name`.
- Bí mật link không phải bí mật phiên: `link_token_refused` — cùng 404 với token chưa từng tồn tại.
- Cửa mở và chỉ mở một lần: `named_token_opens_a_session`, `named_token_spent_twice`.
- Lời mời đã thu hồi là lời mời chết: `owner_invites_mate_by_name`, `owner_revokes_mate_invite`, `revoked_token_refused`.
- Framework: `trailing_slash`, `patch_not_allowed`.

Corpus sinh: `generated/w9-422/post-sessions.yaml` (45 bước), sinh trong ảnh ghim `mobile-parity-api:7bf58e3d` từ wave `w9` của `scripts/render_parity_422_scenarios.py`.

`owner_names_mate` có mặt vì lời mời đích danh cho một người **chưa tồn tại** là 409 chứ không phải 201, nên bước sau sẽ không có `id` để bind và cả lượt chết bằng INFRA. Bước đó được thêm sau khi lượt chạy đầu tiên hỏng đúng như vậy.

## Chưa phủ / lưu ý cho bản Go

- Lời mời **hết hạn** (`expires_at <= now`) chưa có bước: kịch bản không lùi được đồng hồ và `expires_at` mặc định còn xa. Nhánh này chỉ có test Postgres phủ.
- `max_length=512` không có bước tay; corpus sinh phủ hai đầu của model.
- Bản Go phải giữ **đúng một** 404 cho cả năm lý do. Tách bất kỳ lý do nào thành 403 hay 409 là một rò rỉ năng lực, không phải một cải tiến thông báo lỗi.
- `token` trả về phải là `secrets.token_urlsafe(32)` — 43 ký tự base64url. Harness đặt chỗ token theo **độ dài**: một token ngắn hơn, dài hơn hay có đệm sẽ không khớp đặt chỗ và lộ ra ngay.
- Thứ tự khoá trong thân 201 là một phần của hợp đồng dây (ADR-0029 §2.4), không phải chi tiết trình bày.
