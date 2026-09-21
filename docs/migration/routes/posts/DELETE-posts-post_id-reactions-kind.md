# DELETE /posts/{post_id}/reactions/{kind}

posts · core · trạng thái trong bộ nhớ: không có

## Mục đích

ADR-0022 §2.2: người đọc được bài gỡ cảm xúc **của chính mình** thuộc một loại. Trả 200 kèm tổng đếm lại (không phải 204), để màn hình vẽ lại theo số của máy chủ. Gỡ thứ không có vẫn là 200.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware`: DELETE là phương thức ghi (`services/api/app/api/idempotency.py:76`), nên có `Idempotency-Key` là được đặt chỗ; key rỗng → 422 trước xác thực (`anonymous_empty_idempotency_key`).
2. `get_actor` (`services/api/app/api/deps.py:110-164`): 401 thắng mọi lỗi path (`anonymous_bad_kind`, `anonymous_non_uuid_post`).
3. Path `post_id` (UUID lax) và `kind` (`PostReactionKind`) validate cùng lúc, một mảng 422, `post_id` trước (`stranger_non_uuid_and_bad_kind`). Chạy trước handler: bài không tồn tại + `kind` sai → 422 (`stranger_bad_kind_unknown_post`). Path được giải mã phần trăm trước khi so: `%68eart` là `heart` (`stranger_kind_percent_encoded` → 404 vì bài không có).
4. `_readable_post_or_404` (`services/api/app/api/service.py:2246-2270`) → 404 `post_not_found` cho bài không có hoặc không đọc được (ma trận như card `POST /posts/{post_id}/reactions`).
5. `_require_permission("react_to_post", {"may_read_post": True})` (`service.py:2399`) → 403 `role_not_permitted` khi thiếu cả `member` lẫn `group_admin`; chạy sau bước 4 (`stranger_roles_empty_unreadable` → 404, `stranger_roles_advancer_only` → 403).

Mất quyền đọc là mất cách gỡ cảm xúc của chính mình: sau khi chặn tác giả (`rival_removes_like_after_blocking`) hoặc rời nhóm (`mate_removes_after_leaving`) → 404, trong khi dòng vẫn được đếm cho người khác (`author_sees_rivals_like_still_counted`).

## Đầu vào

- Path `post_id` (UUID lax) và `kind` ∈ `heart|haha|like|wow|sad|fire` (`services/api/app/api/schemas.py:1584`), phân biệt hoa thường (`Heart` → 422). `kind` rỗng là `/reactions/`: router trả 307 về `/posts/{id}/reactions` (`stranger_empty_kind_redirects`), không phải 422.
- Header: `Idempotency-Key` tuỳ chọn. Body **không được đọc**: `{"kind":"heart"}` gửi kèm DELETE `.../like` vẫn gỡ `like` (`stranger_body_is_ignored`). Nhưng body có tham gia fingerprint idempotency (`idem_reuse_with_body` → 422 reuse).
- Không query.

## Đầu ra

- **200** `PostReactionsResponse` (`schemas.py:1596-1601`), thứ tự khoá `post_id`, `reactions`, `my_reactions`; `reactions[]` sắp theo tên loại, `my_reactions[]` của người gọi, sắp theo tên (`service.py:2367-2379`). Bài không còn cảm xúc nào → `{"post_id":…,"reactions":[],"my_reactions":[]}`.
- Không float, không datetime.
- Replay idempotency: trả **đúng body đã lưu** lúc lần đầu, thêm `idempotency-replayed: true`, kể cả khi trạng thái đã đổi sau đó (`friend_rewows_friends` rồi `idem_replay_after_state_changed` vẫn trả tổng cũ không có `wow`).
- Framework: 307 cho `/` cuối (`location: http://<Host>/posts/<id>/reactions/<kind>`); 405 + `allow: DELETE` cho GET và POST.

## Tác dụng phụ

- `post_reactions`: SELECT dòng `(post_id, person_id, kind)` rồi DELETE nếu có (`services/api/app/api/repository.py:5620-5634`). Không có thì không ghi gì. Không bao giờ chạm dòng của người khác (`stranger_removes_heart_not_own`).
- Idempotency: lưu câu trả lời 200; 404/422 nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`). Fingerprint gồm path nên cùng key cho `kind` khác → 422 reuse (`idem_reuse_different_kind`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | như card `POST /posts/{post_id}/reactions` | `deps.py:147`, `:152-154` |
| 404 | `post_not_found` | `Post does not exist` | `service.py:2258`, `:2269` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:502-504` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework đã đo: `uuid_parsing` (`loc ["path","post_id"]`), `literal_error` với `loc ["path","kind"]` (khác card POST ở chỗ `path` thay cho `body`).

## Mã Python

- Route: `services/api/app/api/routes/posts.py:144-158` (`unreact_to_post`)
- Service: `services/api/app/api/service.py:2394-2403` (`unreact_to_post`), `:2367-2379`, `:2246-2270`, `:2150-2163`
- Domain: `services/api/app/domain/post_audience.py:126-201`
- Repository: `services/api/app/api/repository.py:5620-5634` (`remove_post_reaction`), `:5550-5583` (`post_social_counts`)

## Test đang phủ

- `services/api/tests/api/test_post_comments_reactions.py`: `test_a_reader_reacts_once_per_kind_and_the_post_recounts_from_the_rows` (83), `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` (260)
- `services/api/tests/postgres/test_post_social_postgres.py::test_three_hearts_and_two_comments_read_three_and_two_over_http` (120)

## Kịch bản parity

`parity/scenarios/w2/posts/DELETE-posts-post_id-reactions-kind.yaml`, id `w2/posts/delete-posts-post_id-reactions-kind` (74 bước):

- Thứ tự từ chối: `anonymous_unknown_post`, `anonymous_bad_kind`, `anonymous_non_uuid_post` (401), `anonymous_empty_idempotency_key` (422 middleware), `stranger_non_uuid_post`, `stranger_bad_kind_unknown_post` (422 trước 404), `stranger_kind_wrong_case`, `stranger_kind_percent_encoded`, `stranger_non_uuid_and_bad_kind` (hai lỗi path), `stranger_empty_kind_redirects` (307), `stranger_unknown_post`, `stranger_uppercase_unknown_post`, `stranger_unknown_post_roles_empty` (404 trước 403).
- Đường vui: `friend_removes_heart_public`, `friend_removes_heart_public_again`, `friend_removes_kind_never_added`, `stranger_removes_heart_not_own`, `author_removes_on_own_only_me`, `mate_removes_fire_group`, `stranger_body_is_ignored`.
- Ma trận đọc: `stranger_on_only_me`, `stranger_on_friends`, `friend_on_group`, `mate_removes_after_leaving`.
- Role: `stranger_roles_advancer_only` (403), `stranger_roles_empty_unreadable` (404), `stranger_roles_group_admin_only` (200), `stranger_roles_unknown`, `actor_id_not_uuid`.
- Chặn và xoá tài khoản: `rival_blocks_author`, `rival_removes_like_after_blocking` (404), `author_sees_rivals_like_still_counted`, `ghost_hahas_public`, `ghost_deletes_account`, `ghost_removes_haha_after_deletion` (200, dòng đã bị xoá cùng tài khoản).
- Idempotency: `idem_first`, `idem_replay`, `friend_rewows_friends` + `idem_replay_after_state_changed`, `idem_reuse_different_kind`, `idem_reuse_with_body`, `idem_second_distinct_key`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal`.
- Framework: `trailing_slash_redirects` (307), `get_not_allowed`, `post_not_allowed` (405).

## Chưa phủ / lưu ý cho bản Go

- `prod` không có trong kịch bản này (lát posts core giữ kịch bản prod của nhóm).
- 409 in-flight và gỡ đồng thời hai lần không tạo được (harness tuần tự).
- Body của DELETE bị bỏ qua khi xử lý nhưng **có** trong fingerprint: bản Go không được bỏ body ra khỏi fingerprint chỉ vì route không đọc nó.
- Router Starlette giải mã phần trăm trước khi khớp `Literal`; Go dùng `r.URL.Path` (đã giải mã) sẽ khớp, `RawPath` thì không.
- Replay trả body cũ dù bảng đã đổi; đây là hợp đồng idempotency, không phải lỗi cần sửa ở bản Go.
- Người mất quyền đọc không gỡ được cảm xúc của mình và cảm xúc đó vẫn được đếm. Hành vi hiện tại, ghi lại để bản Go không "sửa" lén.
