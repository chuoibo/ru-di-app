# GET /people/{person_id}/photos/{photo_id}

photos · core · trạng thái trong bộ nhớ: không có

## Mục đích

Cửa duy nhất tới byte của một ảnh cá nhân (ADR-0022 §2.1). Chủ ảnh luôn đọc được; người khác đọc được đúng khi một bài đăng hoặc một story còn hạn hiển thị ảnh đó và bài/story đó đọc được với họ. Mọi từ chối là **cùng một 404**, không bao giờ 403: một 403 sẽ nói rằng ảnh tồn tại.

## Xác thực và quyền

Thứ tự (`services/api/app/api/service.py:1924-1953`), đo trên stack:

1. Router: đuôi `/` → 307, `location` tuyệt đối (`trailing_slash_redirects`); `HEAD`, `DELETE` → 405 `allow: GET` (`head_not_allowed`, `delete_not_allowed`).
2. `get_actor`: 401 đi trước 422 của path (`anonymous_non_uuid_both`); 422 `invalid_actor_roles` (`roles_unknown`).
3. Path `person_id`, `photo_id`: UUID lax, lỗi chung một danh sách (`stranger_non_uuid_both`).
4. `addressee` = `actor.id == person_id` **hoặc** `person_image_visible_to(person_id, photo_id, actor.id, now)` (`services/api/app/api/repository.py:4572-4600`). Câu SQL chạy **trước** kiểm vai trò, kể cả với id không tồn tại:
   - dựng `url = f"/people/{person_id}/photos/{photo_id}"` từ UUID đã parse (viết thường có gạch nối);
   - có một `posts` với `image_url = url` (so chuỗi) đọc được theo `_readable_by(reader)` (`repository.py:5423-…`, cùng quy tắc với feed: tác giả; `public` trừ khi hai người chặn nhau; `friends` với bạn `accepted` không chặn; `group` với thành viên `ACTIVE`, `left_at IS NULL` của đúng nhóm, không xét chặn);
   - hoặc một `stories` với `image_url = url` còn hạn tại `now` và đọc được theo `_story_readable_by(reader, now)` (`repository.py:4891-…`: bạn `accepted`, không chặn).
5. `_require_permission("view_person_photo", actor, {"is_photo_addressee": addressee})` (`services/api/app/domain/permissions.py:459-462`): vai trò (`group_admin`/`member`) rồi `is_photo_addressee`. **Mọi** `ApiProblem` ở đây được đổi thành 404 `photo_not_found`, kể cả `role_not_permitted` của chính chủ (`author_roles_guest_reads_own`, `author_roles_empty_reads_own`, `friend_roles_guest_reads_for_friends`).
6. `get_person_image(person_id, photo_id)` (`repository.py:4560-4570`): `owner_person_id = person_id AND id = photo_id AND purpose = 'personal'` → không có → 404. Avatar, ảnh nhóm, ảnh của người khác đặt dưới path người này đều 404, kể cả với chủ (`author_avatar_under_personal_path`, `author_group_photo_under_personal_path`, `author_reads_under_friends_path`, `friend_reads_under_own_path`).
7. `_stored_image_bytes`: tệp mất hoặc rỗng → 404 `photo_not_found`.

Ma trận đo được (ảnh của `author`):

| Ảnh hiển thị bởi | Chủ | Bạn | Lời mời đang chờ | Người lạ | Thành viên nhóm (không phải bạn) |
|---|---|---|---|---|---|
| chưa bài nào | 200 | 404 | — | 404 | — |
| bài `friends` | 200 | **200** | 404 | 404 | 404 |
| bài `public` | 200 | — | **200** | **200** | — |
| bài `group` | 200 | 404 (không phải thành viên) | — | 404 (kể cả có `X-Actor-Contexts`) | **200** |
| bài `only_me` | 200 | 404 | — | — | — |
| story còn hạn | — | **200** | — | 404 | 404 |
| story đã xoá | — | 404 | — | — | — |

- Ảnh ở bài `friends` mà sau đó được đăng lại trong một bài `public` mở cho người lạ (`stranger_reads_for_friends_after_public_post`): chỉ cần **một** bài đọc được.
- Người chặn tác giả mất cả ảnh `friends` lẫn `public` (`blocker_reads_for_friends_after_block`, `blocker_reads_public_after_block`).
- Rời nhóm mất ảnh bài `group` nhưng vẫn đọc ảnh bài `public` (`leaver_reads_group_after_leaving`, `leaver_reads_public_after_leaving`).

## Đầu vào

- Path `person_id`, `photo_id`: UUID lax. Viết hoa hay không gạch nối của id không tồn tại → 404 (`author_uppercase_unknown_photo`, `author_unhyphenated_unknown_photo`). `url` so với `posts.image_url` được in lại từ UUID, nên id viết hoa của một ảnh có thật vẫn khớp (không có kịch bản: harness không đổi hoa thường giá trị đã bind).
- Không query, không body. `Range` bị bỏ qua (`stranger_reads_with_range` 200 cả tệp).

## Đầu ra

- **200**, thân là byte tệp đã lưu; header `content-type` (`image/jpeg`/`image/png` từ hàng), `content-length`, `cache-control: private, max-age=300`; không `etag`, `last-modified`, `accept-ranges` (`services/api/app/api/routes/photos.py:145-164`).
- Mọi từ chối: 404 `{"code":"photo_not_found","detail":"Photo does not exist"}`.

## Tác dụng phụ

Chỉ đọc `posts`, `stories`, `friend_requests`, `memberships`, `uploaded_images` và tệp. Không idempotency, không limiter.

Xoá tài khoản chủ ảnh xoá hàng và tệp ảnh cá nhân cùng bài và story: người lạ và chính chủ (dev) đều nhận 404 (`stranger_reads_public_after_deletion`, `author_reads_own_after_deletion`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, `Session is not valid` (prod) | `services/api/app/api/deps.py:110-164`; `service.py:4453-4474` |
| 422 | `invalid_actor_roles` (và `invalid_actor_id`, `invalid_actor_contexts`) | `X-Actor-Roles contains an unknown role` | `deps.py:144-163` |
| 422 | (validation) | `uuid_parsing`, `loc` `["path","person_id"]` / `["path","photo_id"]` | `services/api/app/api/main.py:319-351` |
| 404 | `photo_not_found` | `Photo does not exist` (mọi từ chối, kể cả `role_not_permitted`) | `service.py:1935-1952` |
| 405 | — | `{"detail":"Method Not Allowed"}`, `allow: GET` | Starlette |
| 307 | — | thân rỗng, `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/photos.py:145-164` (`read_person_photo`)
- Service: `services/api/app/api/service.py:1924-1953`, `:652-679`
- Repository: `services/api/app/api/repository.py:4572-4600` (`person_image_visible_to`), `:4560-4570` (`get_person_image`), `:5423` (`_readable_by`), `:4891` (`_story_readable_by`)
- Domain: `services/api/app/domain/post_audience.py`, `services/api/app/domain/story_visibility.py`, `services/api/app/domain/photo_ref.py:77-78`
- Quyền: `services/api/app/domain/permissions.py:459-462`

## Test đang phủ

- `services/api/tests/api/test_person_photos_gate.py`: `test_the_owner_reads_their_own_photo_and_nobody_else_does_until_a_post_shows_it` (104), `test_a_group_photo_illustrates_only_a_post_addressed_to_that_group` (164), `test_the_url_is_stored_in_canonical_form` (206)
- `services/api/tests/api/test_person_photo_gate_is_one_door.py`: `test_the_visibility_question_is_asked_in_exactly_one_place` (30), `test_the_bytes_route_goes_through_that_gate` (41)
- `services/api/tests/api/test_photo_bytes_present_but_empty.py` (215-240)

## Kịch bản parity

`parity/scenarios/w6/photos/GET-people-person_id-photos-photo_id.yaml`, id `w6/photos/get-people-person_id-photos-photo_id` (86 bước, `dev`):

- Thứ tự: `anonymous_read`, `anonymous_non_uuid_both`, `roles_unknown`, `stranger_non_uuid_both`, `stranger_unknown_both`.
- Chuẩn bị: sáu ảnh cá nhân của `author` (JPEG, PNG alpha, WebP orientation 6, GIF, JPEG cho story, BMP không bao giờ hiển thị) và một avatar; bạn `friend`, lời mời của `requester`, bạn `blocker`; nhóm với `mate` và `leaver`; một ảnh nhóm.
- Chủ và cổng vai trò: `author_reads_for_friends`, `author_reads_for_public`, `author_reads_rotated_webp`, `author_reads_never_shown`, `author_roles_guest_reads_own`, `author_roles_empty_reads_own`, `author_unknown_photo`, `author_avatar_under_personal_path`, `stranger_reads_before_any_post`, `friend_reads_before_any_post`.
- Bài mở ảnh: `author_posts_for_friends`, `author_posts_public`, `author_posts_to_group`, `author_posts_only_me`; `friend_reads_for_friends`, `friend_roles_guest_reads_for_friends`, `requester_reads_for_friends`, `stranger_reads_for_friends`, `mate_reads_for_friends`, `stranger_reads_public`, `requester_reads_public`, `mate_reads_group`, `leaver_reads_group_while_member`, `friend_reads_group_without_membership`, `stranger_with_contexts_header_reads_group`, `friend_reads_only_me`, `author_reads_only_me`, `friend_reads_never_shown`, `author_posts_friends_photo_publicly`, `stranger_reads_for_friends_after_public_post`.
- Path phải là của chủ: `friend_reads_under_own_path`, `author_reads_under_friends_path`, `author_group_photo_under_personal_path`, `mate_group_photo_under_personal_path`.
- Story: `author_creates_story`, `friend_reads_story_photo`, `stranger_reads_story_photo`, `mate_reads_story_photo`, `author_deletes_story`, `friend_reads_story_photo_after_delete`.
- Chặn và rời: `blocker_reads_for_friends_before_block`, `blocker_blocks_author`, `blocker_reads_for_friends_after_block`, `blocker_reads_public_after_block`, `leaver_leaves`, `leaver_reads_group_after_leaving`, `leaver_reads_public_after_leaving`.
- Id và framework: `author_uppercase_unknown_photo`, `author_unhyphenated_unknown_photo`, `stranger_reads_with_range`, `trailing_slash_redirects`, `head_not_allowed`, `delete_not_allowed`.
- Xoá tài khoản: `author_deletes_account`, `stranger_reads_public_after_deletion`, `author_reads_own_after_deletion`.

`parity/scenarios/w6/crossreplay/POST-people-me-photos.yaml`: người lạ đọc ảnh qua cửa trước và qua Python sau khi một bài công khai tạo qua Python hiển thị nó.

`parity/scenarios/w6/photos/prod-auth.yaml`, id `w6/photos/prod-auth` (30 bước, `prod`, cả sáu route):

- Không bearer, header dev không bearer, bearer rác → 401 `Missing bearer session` / `Session is not valid` trước mọi thứ, trừ lỗi multipart 400 (`anonymous_group_upload`, `dev_headers_without_bearer`, `junk_bearer_personal_upload`, `junk_bearer_avatar_read`, `anonymous_multipart_without_boundary`).
- Vai trò lấy từ roster (`member`, `advancer`, `recipient`, `sender`, `creditor`, thêm `former_member` nếu đã rời một nhóm; `services/api/app/api/repository.py:3585-3642`), nên `X-Actor-Roles: guest` bị bỏ qua (`owner_uploads_with_guest_roles_header` 201) và `X-Actor-ID` không đổi được actor (`stranger_uploads_claiming_owner` 403).
- Ảnh nhóm: `owner_uploads_garbage` 415, `mate_reads_group_photo` 200, `stranger_reads_group_photo` 403, `anonymous_reads_group_photo` 401.
- Avatar: `owner_sets_avatar` 201, `owner_sets_mates_avatar` 403 `is_self`, `mate_reads_owners_avatar` 200, `stranger_reads_owners_avatar` 403.
- Ảnh cá nhân: `owner_uploads_personal` (khoá `w6-prod-personal`), `owner_personal_replay_with_mates_actor_header` (phát lại: scope là digest bearer), `mate_same_key_is_another_scope` (201 mới), `owner_reads_personal`, `mate_reads_personal_before_post` 404, `owner_posts_personal_to_group`, `mate_reads_personal_after_group_post` 200, `stranger_reads_personal_after_group_post` 404.
- Đăng xuất: `mate_signs_out`, `mate_reads_group_photo_after_sign_out` và `mate_uploads_after_sign_out` 401 `Session is not valid`, `owner_still_reads_group_photo` 200.

## Chưa phủ / lưu ý cho bản Go

- Mọi từ chối phải là cùng một 404; không được tách vai trò thành 403, không được bỏ qua câu SQL khi id không tồn tại.
- So `posts.image_url` và `stories.image_url` bằng **chuỗi** với url in lại từ UUID; `POST /posts` và `POST /stories` lưu dạng chuẩn (`test_the_url_is_stored_in_canonical_form`). Bản Go của `postaudience`/`storyvisibility` (W2) đã có; route này phải dùng đúng câu SQL `_readable_by`/`_story_readable_by`, không phải bản domain lọc sau.
- Story hết hạn sau 24 giờ không có kịch bản (harness không có bước đồng hồ); chỉ phủ story bị xoá.
- Chưa phủ: tác giả chặn người đọc (chỉ phủ người đọc chặn tác giả), huỷ kết bạn, bài `group` khi người đọc bị chặn (nhánh nhóm không xét chặn), id viết hoa của ảnh có thật, tệp mất/rỗng.
- Kho tệp dùng chung: xem thẻ `POST /contexts/{context_id}/photos`.

## Lỗi Python (chỉ báo, không sửa)

- Chính chủ có vai trò không được phép nhận 404 cho ảnh của mình: client không phân biệt được «không có ảnh» với «phiên thiếu vai trò». Có chủ ý (một cửa 404), chỉ ghi nhận.
- Câu SQL hiển thị chạy cho mọi request có danh tính, kể cả id rác hợp lệ về dạng, trước khi biết ảnh có tồn tại.
- `HEAD` trả 405; `Range` bị bỏ qua.
