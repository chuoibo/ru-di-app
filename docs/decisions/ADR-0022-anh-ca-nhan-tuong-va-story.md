# ADR-0022 — Ảnh cá nhân đọc được khi và chỉ khi có bài hoặc story đọc được trỏ tới; tường có bình luận và phản ứng; story sống 24 giờ cho bạn bè

- **Trạng thái:** 🟡 **ĐỀ XUẤT** 2026-09-06 — chờ Lead đánh ĐÃ CHẤP NHẬN. Lát L3 (tường) và L4 (story) của nhánh `claude/p0-w-m15-social-v1-1` không merge trước khi dòng này đổi. Lead đã duyệt kế hoạch chứa các quyết định này trong phiên 2026-09-06.
- **Quyết định bởi:** Lead (phiên 2026-09-06; ghi lại ở mục 2).
- **Hiện thực:** nhánh `claude/p0-w-m15-social-v1-1`, lát L3 và L4; kế hoạch `~/.claude/plans/mellow-waddling-lantern.md` mục 5.4–5.5.
- **Thay đổi bảng `uploaded_images`, `posts`, `people`; thêm bốn bảng và hai họ route**; không đổi ba luật tiền; không đổi quy tắc «404 thay 403» của F39/F42; không đổi cách ảnh nhóm hoạt động.

## 1. Bối cảnh

Bài đăng (F39/F42, #308) có bốn audience `only_me/friends/group/public` và một luật đọc viết hai lần (`post_audience.can_read` và SQL `_readable_by`). Nhưng bài chỉ có chữ: `image_url` bị pin vào ảnh **của nhóm** (`/contexts/{id}/photos/{id}`), mà ảnh nhóm chỉ thành viên đọc được, nên ảnh trên bài `friends` hay `public` sẽ là một địa chỉ phần lớn người đọc không mở được. Không có bình luận, không có phản ứng trên bài; bình luận và tim chỉ có ở kỷ niệm trong nhóm (M6).

Bảng `uploaded_images` đã có `owner_person_id` với CHECK «đúng một chủ» (context hoặc người) từ khi tạo — đó là ảnh đại diện. Chưa có khái niệm story; `product/feature_list.md` chỉ có F38 widget kiểu Locket «optional later».

Backend do Claude làm theo uỷ quyền ADR-0016 §2.3; charter không đổi.

## 2. Quyết định

### 2.1 Ảnh cá nhân dùng lại `uploaded_images`, có `purpose`
1. Không tạo bảng ảnh mới. Thêm cột `uploaded_images.purpose ∈ ('group','avatar','personal')` với backfill `owner_person_id IS NOT NULL → 'avatar'` và CHECK `(purpose = 'group') = (context_id IS NOT NULL)`. **Lý do bắt buộc**: `get_latest_avatar` lấy ảnh mới nhất của người; không có `purpose` thì ảnh vừa đăng bài sẽ thành ảnh đại diện.
2. URL ảnh cá nhân có hình dạng riêng `/people/{person_id}/photos/{photo_id}` với kiểu wire riêng `PersonPhotoUrl`. **`RelativePhotoUrl` (ảnh nhóm) không mở rộng**: kỷ niệm, tin nhắn và album vẫn chỉ nhận ảnh nhóm. Bài đăng nhận `PostPhotoUrl = PersonPhotoUrl | RelativePhotoUrl`, nhưng ảnh nhóm trên bài **chỉ khi `audience = 'group'` cùng nhóm** — thắt chặt so với hiện nay.
3. **Luật đọc ảnh cá nhân**: người đọc mở được ảnh khi là chủ ảnh, hoặc khi **tồn tại ít nhất một bài hoặc một story còn hạn trỏ tới ảnh đó mà người đọc đọc được** (`_readable_by` cho bài; luật story ở 2.4). Bài bị xoá thì ảnh của nó không còn mở được với ai ngoài chủ. Mọi từ chối là 404 `photo_not_found`, không 403. Mọi route đọc ảnh cá nhân đi qua **một** hàm gate duy nhất; có test AST đếm.

### 2.2 Tường có bình luận và phản ứng, theo mẫu của kỷ niệm
1. `post_reactions(post_id, person_id, kind)` với `kind` thuộc đúng sáu loại của tin nhắn, unique `(post_id, person_id, kind)`; `post_comments(post_id, author_id, body)`. Xoá bài cascade cả hai. Không có cột đếm lưu sẵn; đếm bằng hai `GROUP BY` riêng (bài học «3 tim × 2 bình luận không ra 6» của kỷ niệm).
2. Ai đọc được bài thì phản ứng được. Bình luận thêm một điều kiện do **chủ tường** đặt: `people.wall_comment_policy ∈ ('readers','friends','nobody')`, mặc định `readers`. Luật `post_audience.can_comment = can_read ∧ (chủ bài ∨ policy = readers ∨ (policy = friends ∧ là bạn))`, policy lạ → không. Máy chủ trả `can_comment` trên mỗi bài; client không suy từ quan hệ.
3. Xoá bình luận: tác giả bình luận hoặc chủ bài. Không sửa bình luận, không sửa bài (cùng lý do «lịch sử sửa là một câu hỏi riêng tư chưa trả lời» đã ghi ở `Post`).
4. **Không có «đăng lên tường người khác».** Bài luôn là của tác giả trên tường tác giả; muốn nhắc ai thì bình luận hoặc nhắn.

### 2.3 Story là ảnh cá nhân sống 24 giờ, chỉ bạn bè thấy
1. `stories(author_id, image_url: PersonPhotoUrl của chính tác giả, caption ≤ 200, audience = 'friends', created_at, expires_at)`; `story_views(story_id, viewer_id, seen_at)` với khoá chính cặp. Không có audience khác trong v1.
2. `expires_at = created_at + 24h` **tính ở domain** (`story_visibility.expires_at_for`), cột không có `server_default`; mọi câu SQL lọc story so với `now` **do service truyền vào làm bind param**, không dùng `now()` của Postgres, để tier test giả được đồng hồ và luật viết hai nơi đối chiếu được nhau.
3. Luật xem `story_visibility.can_view`: tác giả luôn xem được; người khác xem được khi là bạn, không bị chặn (ADR-0023) và story còn hạn. Danh sách `GET /stories` nhóm theo tác giả, chưa xem trước.
4. Story quá hạn vẫn nằm trong bảng cho tới khi script vận hành `scripts/purge_expired_stories.py` xoá (hàng quá hạn > 7 ngày, kèm file ảnh không còn ai trỏ tới). Không có job nền trong request; không có `pg_cron`/trigger.

### 2.4 Chỗ đặt trên app
Dải story ở đầu tab Tin nhắn (ô đầu là «Đăng story», luôn có mặt kể cả khi dải rỗng hay lỗi; vòng «Story của bạn» chỉ hiện khi mình có story còn hạn); trình xem là một route toàn màn (`fullScreenModal`) có thanh tiến trình, tự chuyển 5 giây giữa các story của một tác giả (tôn trọng Reduce Motion) và dừng ở story cuối chứ không tự đóng; nút «Đóng story», Back hoặc chạm phải trên story cuối mới đóng; chạm phải/trái chuyển; gọi «đã xem» khi ảnh hiện. Bài có màn chi tiết `/posts/{id}` với bình luận theo cursor. Đăng bài và đăng story chọn ảnh từ thư viện, nén như ảnh nhóm (`nenLai`), **tải byte trước rồi mới tạo bài/story**.

## 3. Hệ quả

- Migration `7e3c9a5f1d64` (L3, có backfill `purpose` trước khi thêm CHECK) và `8f4d0b6a2e75` (L4).
- Route bytes mới `GET /people/{person_id}/photos/{photo_id}` khai vào hai bảng của tier test (`test_photo_bytes_present_but_empty.py::ROUTES` và `ROUTES_WITHOUT_RESPONSE_VALIDATION`) và ghim `cong-mu` trong `.server-routes-uncalled.json` vì URL do máy chủ trả trong `PostResponse.image_url`.
- `tests/api/test_photo_url_context_guard.py::MALFORMED` mở rộng: URL cá nhân bị kỷ niệm/tin nhắn từ chối 422.
- `PATCH /people/me` nhận `wall_comment_policy` ngay ở L3 (màn Cài đặt đầy đủ đến ở L5, nhưng chip trên tường của mình cần nó để canary đo được).
- Client có `nguonAnhBai` rẽ theo tiền tố `/people/` hoặc `/contexts/`; ảnh trong bài có `onError` in «Chưa tải được ảnh» để flow đo được ảnh có hiện hay không (bài học #570: bảng xanh mà ảnh rỗng).

## 4. Cái này KHÔNG chứng minh

- Ảnh cá nhân không rò qua CDN hay cache trung gian: hiện không có CDN; `Cache-Control: private` là tất cả những gì máy chủ nói.
- Story «riêng tư» theo nghĩa xã hội: người xem vẫn chụp màn được. Hạn 24 giờ là hạn hiển thị, không phải hạn tồn tại byte cho tới khi script chạy.
- Hai người có thấy bình luận «hợp lý» không — chưa có bằng chứng hành vi.

## 5. Phương án đã bác

| Phương án | Vì sao không |
|---|---|
| Bảng ảnh cá nhân riêng | `uploaded_images` đã có chủ-là-người từ đầu; hai bảng cho một thứ là hai luật đọc phải giữ đồng bộ |
| Mở rộng `RelativePhotoUrl` thêm nhánh cá nhân | Kỷ niệm và tin nhắn sẽ nhận ảnh cá nhân mà gate của chúng không biết đọc → 500 hoặc lộ |
| Cho phép ảnh nhóm trên bài `friends`/`public` như hiện tại | Địa chỉ mà người đọc không mở được; khoá luật ảnh nhóm chỉ cho audience `group` |
| Cột đếm `reaction_count` trên `posts` | Cache là nguồn sự thật sai (luật 3 áp cho tiền, tinh thần áp cho mọi số) |
| Hết hạn story bằng `pg_cron`/trigger | Không có trong compose; luật «không job nền trong máy chủ» giữ nguyên, script vận hành như `import_place_photos.py` |
| Story cho `public`/nhóm | Mở rộng bề mặt riêng tư trước khi có bằng chứng ai dùng |
| Đăng lên tường người khác | Pattern Facebook 2010, rủi ro quấy rối; quyền bình luận đã đủ |

## 6. Cách kiểm chứng

- Domain: bảng chân trị `can_comment` (3 policy × 4 audience × 3 người đọc); `can_view` biên `now == expires_at` (hết) và `now − 1 s` (còn); `parse_photo_url`.
- Postgres: unique phản ứng; hai `GROUP BY` không nhân bản; gate ảnh cá nhân qua `_readable_by` thật (người lạ 404, bạn 200, **cùng ảnh sau khi bài bị xoá → 404**); backfill `purpose` và `get_latest_avatar` không trả ảnh `personal`; story: `_now` giả +24h+1s → rỗng; story của người không phải bạn «không rời DB»; gate ảnh qua story còn hạn 200 / hết hạn 404; PK `story_views`.
- API: mọi route có `{post_id}` hoặc `{story_id}` với người lạ chỉ trả 404/422, không bao giờ 403 (duyệt `app.routes`, không liệt kê tay); kỷ niệm/tin nhắn từ chối URL cá nhân 422.
- E2E node: `story-het-han.test.mjs` đăng story rồi `UPDATE expires_at` thẳng vào DB dùng-một-lần của lượt đo → `GET /stories` rỗng (biến `MOBILE_E2E_DATABASE_URL` thêm vào `e2e_slice.sh`). Không có route chỉ-dev để tua thời gian.
- Emulator: flow 42 (bình luận, tim, ảnh trên bài, đổi policy → ô soạn biến mất), flow 43 (dải story, xem, đã xem; `_43b` mở lại sau khi `kiem_may_chu_sau_43` tua story qua hạn thẳng trong DB qua `MOBILE_DATABASE_URL` của stack dùng-một-lần; thiếu URL thì bảng đỏ, không phải bỏ qua), canary curl (người lạ đọc ảnh → 404; policy `nobody` → 403 và `can_comment=false`).
- Đột biến phải đỏ: bỏ `NOT EXISTS`/EXISTS trong gate ảnh → ca người lạ đỏ; đổi 404 thành 403 ở `read_post` → ca «không bao giờ 403» đỏ; thêm `server_default` cho `expires_at` → test AST đỏ; bỏ lọc `purpose` ở `get_latest_avatar` → ca avatar đỏ.
