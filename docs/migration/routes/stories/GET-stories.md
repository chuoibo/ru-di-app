# GET /stories

stories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Thanh story: mọi story còn sống mà actor được xem, gom theo tác giả. Thứ tự do server quyết: story của chính mình trước, rồi tác giả còn story chưa xem, rồi phần còn lại, mỗi nhóm xếp theo story mới nhất trước. Client vẽ đúng thứ tự này, không tự sắp.

## Xác thực và quyền

- `get_actor` (`services/api/app/api/deps.py:110-164`) là bước duy nhất có thể từ chối: 401 khi thiếu danh tính; ở `dev` 422 `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts`.
- **Không có `_require_permission`** (`services/api/app/api/service.py:2020-2066`): role rỗng vẫn đọc được thanh (`viewer_roles_empty_feed` → 200), khác với `POST /stories` và `POST /stories/{story_id}/seen`, nơi thiếu role là 403/404. Người chưa đăng ký nhận `{"authors":[]}` (`ghost_empty_feed`).
- Không có 403 hay 404: người không được xem chỉ đơn giản vắng mặt khỏi danh sách.
- Quy tắc xem (`services/api/app/domain/story_visibility.py:92-117`): tác giả luôn xem được story của mình; người khác cần **cả ba**: là bạn (`friend_requests.state = 'accepted'`, đọc tại thời điểm gọi, `service.py:2127-2136`), không có chặn theo chiều nào (`service.py:2138-2148`), story còn sống (`expires_at > now`). Cùng quy tắc được viết lại bằng SQL ở `_story_readable_by` (`services/api/app/api/repository.py:4891-4947`), và service lọc lại từng dòng bằng hàm domain.

## Đầu vào

- Không path param, không body. Query bị bỏ qua (`viewer_query_string_ignored`).
- Header chỉ có danh tính.

## Đầu ra

- **200** `StoryFeedResponse` (`services/api/app/api/schemas.py:1387-1392`): `{"authors":[...]}`, không bao giờ bỏ khoá `authors`.
- Mỗi phần tử `StoryAuthorFeed` (`schemas.py:1381-1384`), thứ tự khoá `author`, `stories`, `all_seen`:
  - `author` (`StoryAuthor`, `schemas.py:1376-1378`): `id`, `display_name` (tên trong `people`, rơi về chuỗi id nếu trống, `repository.py:2529-2548`).
  - `stories[]`: `StoryResponse` (`schemas.py:1363-1373`) với thứ tự khoá như card `POST /stories`; `seen` là của **người đang đọc** (LEFT JOIN `story_views` theo `viewer_id`, `repository.py:5006-5023`).
  - `all_seen`: mọi story của tác giả đó đều đã được người đọc xem (`service.py:2055`).
- Thứ tự story trong một tác giả: `created_at` tăng dần rồi `id` (`repository.py:5013`) — cũ trước.
- Thứ tự tác giả (`story_visibility.order_authors`, `story_visibility.py:120-135`): khoá `(mine ? 0 : 1, all_seen ? 1 : 0, -latest_created_at)`, sort ổn định trên thứ tự đầu vào (theo `author_id`). Hệ quả đo được:
  - Của mình luôn đầu, kể cả khi đã xem hết (`viewer_feed_all_seen_newest_first`).
  - Tác giả vừa được xem hết tụt xuống sau tác giả còn story mới dù không có gì mới (`viewer_feed_seen_author_last`); đăng thêm story thì lên lại (`viewer_feed_unseen_again_newest_first`).
  - Trong cùng nhóm, story mới nhất trước (`viewer_feed_two_unseen_newest_first`).
- Story hết hạn bị loại khỏi thanh **kể cả của chính mình** (`repository.py:5012`); tác giả chỉ còn tới được bằng id để xoá.
- Datetime `created_at`, `expires_at` như card `POST /stories` (UTC `Z`, 6 chữ số lẻ). Không có float.
- Framework: 307 cho `/stories/` (`location: http://<Host>/stories`); `DELETE` và `PATCH /stories` → 405 với **`allow: POST`** (Starlette lấy route khớp một phần đầu tiên; `POST /stories` khai báo trước).

## Tác dụng phụ

Chỉ đọc: SELECT `stories` LEFT JOIN `story_views`, EXISTS trên `friend_requests` (hai lần: bạn, chặn), `people`; sau đó service đọc lại cạnh bạn cho từng tác giả (`service.py:2029-2033`). `now` là `_now()` của Python truyền làm tham số, không phải `now()` của SQL (`service.py:2023`). Không idempotency (GET). Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:102-106`, `:143`; `service.py:4464-4468` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:144-147` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:149-154` |
| 422 | `invalid_actor_contexts` | `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:156-163` |

Route không ném `ApiProblem` nào của riêng nó.

## Mã Python

- Route: `services/api/app/api/routes/stories.py:60-68`
- Service: `services/api/app/api/service.py:2020-2066` (`list_stories`), `:2127-2148` (`_is_friend`, `_is_blocked_with`), `:565-585` (`_story_dict`, `_wire_story`)
- Domain: `services/api/app/domain/story_visibility.py:76-79` (`is_live`), `:92-117` (`can_view`), `:120-135` (`order_authors`); `services/api/app/domain/blocking.py:40-64`
- Repository: `services/api/app/api/repository.py:4999-5023` (`list_live_stories_for`), `:4891-4947` (`_story_readable_by`), `:4949-4962` (`_story_record`)

## Test đang phủ

- `services/api/tests/api/test_stories.py`: `test_a_story_reaches_friends_and_nobody_else` (78), `test_seen_is_per_reader_and_the_first_look_counts` (130), `test_the_rail_is_ordered_mine_then_unseen_then_seen` (222)
- `services/api/tests/postgres/test_stories_postgres.py`: `test_a_story_reaches_a_friend_and_never_leaves_the_database_for_a_stranger` (65), `test_the_deadline_is_strict_and_closes_the_photo` (154), `test_the_domain_and_the_sql_agree_on_every_reader` (180)
- `services/api/tests/postgres/test_blocking_visibility_postgres.py::test_a_block_hides_a_story_both_ways` (84)
- `services/api/tests/domain/test_story_visibility.py`: `test_liveness_is_strict_at_the_deadline` (50), `test_who_may_view` (73), `test_the_rail_orders_mine_then_unseen_then_seen_newest_first` (112), `test_ordering_is_stable_for_ties` (144)

## Kịch bản parity

Hai file cho route này, tách theo lane (lý do ở mục Chưa phủ).

`parity/scenarios/w2/stories/GET-stories.yaml`, id `w2/stories/get-stories` (20 bước), được xác nhận với lane DB bật (`--reference-dsn`/`--candidate-dsn`), không có bước nào chèn hay xoá dòng `uploaded_images`; thanh luôn rỗng:

- Danh tính: `anonymous_feed`, `anonymous_junk_bearer_ignored_in_dev` (401 `Missing X-Actor-ID`: `dev` không đọc bearer), `actor_id_not_uuid`, `roles_unknown`, `contexts_header_not_uuid` (422 `invalid_actor_contexts`).
- Thanh rỗng: `ghost_empty_feed`, `viewer_empty_feed`, `viewer_roles_empty_feed` (200, không kiểm role), `viewer_query_string_ignored`, `viewer_feed_friend_without_stories` (bạn không có story thì vắng mặt, không thành nhóm rỗng), `viewer_feed_after_friend_deleted`, `friend_feed_after_own_account_deleted`.
- Framework: `trailing_slash_redirects` (307), `delete_not_allowed`, `patch_not_allowed` (405, `allow: POST`).
- Chuẩn bị: `register_viewer`, `register_friend`, `viewer_asks_friend`, `friend_accepts`, `friend_deletes_account`.

`parity/scenarios/w2/stories-photo/GET-stories.yaml`, id `w2/stories-photo/get-stories` (51 bước, lane DB):

- Ma trận xem: `viewer_feed_groupmate_is_not_friend` (cùng nhóm ACTIVE, không phải bạn), `viewer_feed_own_only`, `viewer_feed_pending_is_not_friend` + `friend_a_feed_pending_is_not_friend` (lời mời chưa trả lời), `viewer_feed_friend_unseen`, `stranger_feed`, `viewer_feed_blocked_by_author` + `friend_b_feed_blocker_sees_nothing_of_viewer` (tác giả chặn người đọc), `viewer_feed_blocker_sees_nothing` + `friend_a_feed_blocked_by_reader` (người đọc chặn tác giả), `viewer_feed_after_unblock_not_friend` (gỡ chặn thành `declined`), `viewer_feed_refriended` (kết bạn lại; dòng `story_views` cũ vẫn còn nên hiện `seen: true`), `viewer_feed_after_author_deleted`, `viewer_feed_after_own_account_deleted`.
- Thứ tự và `seen`: `viewer_feed_two_unseen_newest_first`, `viewer_feed_seen_author_last`, `viewer_feed_unseen_again_newest_first`, `viewer_feed_all_seen_newest_first`, `friend_a_feed_own_first_then_unseen` (`seen` theo từng người đọc); `viewer_roles_empty_feed`, `viewer_query_string_ignored` lặp lại khi thanh có story.
- Chuẩn bị: `register_*`, `upload_*_photo`, `viewer_creates_group`, `viewer_invites_friend_a`, `friend_a_joins_group`, `*_story_*`, `viewer_asks_friend_a`, `friend_a_accepts`, `friend_b_asks_viewer`, `viewer_accepts_friend_b`, `viewer_sees_*`, `*_blocks_*`, `*_unblocks_*`, `friend_a_asks_again`, `viewer_accepts_again`, `*_deletes_account`.

`prod`:

- `parity/scenarios/w2/stories/prod-auth.yaml`, id `w2/stories/prod-auth` (lane DB): `anonymous_feed` (`Missing bearer session`), `junk_bearer_feed` (`Session is not valid`), `lowercase_scheme_feed` (scheme `bearer` viết thường vẫn nhận), `author_feed_empty`, `friend_feed_friend_without_stories`, `author_feed_after_sign_out` (401).
- `parity/scenarios/w2/stories-photo/prod-auth.yaml`, id `w2/stories-photo/prod-auth`: `friend_feed_not_friend_yet`, `friend_feed_sees_story` (tên tác giả là `people.display_name` được seed), `author_feed_own_story`, `friend_feed_after_delete`.

## Chưa phủ / lưu ý cho bản Go

- **Hết hạn 24h không tới được**: harness không dời được đồng hồ và không có bước ghi DB bằng tay, nên nhánh `expires_at <= now` (story của bạn biến mất, story của chính mình cũng rời thanh) chỉ có test Python phủ.
- **`storage_key`, lý do corpus tách đôi**: mỗi lần tải ảnh cá nhân (`POST /people/me/photos`) ghi `uploaded_images.storage_key = secrets.token_hex(16)` (`services/api/app/media/storage.py:28-31`, gọi ở `services/api/app/api/service.py:1817`): 32 ký tự hex ngẫu nhiên, **không bao giờ lên wire** (không có trong `UploadedImageResponse`, `services/api/app/api/schemas.py:1334-1342`). Harness bind chuỗi đúng 32 hex thường theo lần xuất hiện thành `<hex32#n>` (ADR-0029 §2.4), nên mọi đường cần ảnh thật (`w2/stories-photo/*`, kể cả `prod-auth`) chạy với lane DB; khoá dùng lại, viết hoa hay độ dài khác vẫn đỏ. `w2/stories/*` giữ các đường không cần ảnh.
- Hai tác giả có `latest_at` bằng nhau tới micro giây: sort ổn định theo thứ tự `author_id` tăng dần của câu SQL; bản Go phải sort ổn định trên cùng thứ tự đầu vào. Kịch bản không tạo được hai story cùng instant.
- `order_authors` so `timestamp()` float của Python; hai instant cách nhau 1 µs vẫn phân biệt được với epoch hiện tại, nhưng bản Go nên so `time.Time` trực tiếp.
- Route không kiểm role: nếu bản Go "sửa" cho giống `view_story` thì `viewer_roles_empty_feed` lệch; muốn đổi thì mở ADR.
- Mỗi tác giả đọc lại cạnh bạn hai lần (bạn, chặn) ngoài câu SQL; thứ tự và số câu truy vấn không nhìn thấy qua HTTP.
- `allow: POST` cho `DELETE /stories` như card `POST /stories`.
