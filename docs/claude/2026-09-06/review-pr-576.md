# Review PR #576 — L3 «tường có bình luận, phản ứng, quyền bình luận và ảnh cá nhân trên bài» (ADR-0022 §2.1–2.2)

- **Commit được review:** `668b34e269454015cac27fa21acd5fd45f52fc7d` (head PR). Lát là **một** commit trên base; đọc `git diff origin/claude/p0-w-m15-l2-dm..668b34e`: 37 file, +3692/−73.
- **Nhánh → base:** `claude/p0-w-m15-l3-tuong` → `claude/p0-w-m15-l2-dm` (`d8f6596`, PR #575 — APPROVE ở `docs/claude/2026-09-06/review-pr-575.md`). `git merge-base origin/main HEAD` = `d094deb` = `origin/main`: L1/L2/L3 xếp chồng thẳng trên main.
- **protocol_version:** `v1` (`docs/protocol/v1/01-giao-thuc-thuc-dia.md`), như các doc #574/#575.
- **Reviewer:** Claude, context độc lập (không phải tác giả, không phải reviewer của #574/#575), ADR-0007. Worktree sạch `/tmp/review-pr-576` detached tại `668b34e`, `.env` chép từ cây làm việc; **không sửa mã của PR**. Mọi probe và mọi đột biến chạy trên **bản sao** của `services/api` và `apps/mobile` trong scratchpad (`api-copy`, `api-mut`, `api-mut2`, `mobile-mut-root`), không file probe nào nằm trong cây PR. Postgres cho probe là container riêng của reviewer (`postgres:16-alpine`, `review576-pg-…`, cổng loopback ngẫu nhiên), không phải DB của lane nào; không chạm cổng 45933, không chạm `mobile-e2e-pg-*`, không adb/emulator.
- **Ngày:** 2026-09-06.

## VERDICT: APPROVE

**Blocker còn mở: không.**

Bảy điểm được yêu cầu nhìn kỹ đều đo được và đều khớp lời khai của thân PR; mọi cổng tôi tự chạy đều xanh với **đúng con số** thân PR ghi (3310/690/5440 · 628 · 88 · 705 · 0 lỗi tsc · rc=0 khắp nơi). Tôi lấp thêm được ô thân PR tự nhận «không chứng minh» — **backfill `purpose` trên hàng cũ** — bằng probe migration trên Postgres 16 thật có dữ liệu seed ở `6d2b8f4e0c53`: lên đúng (`avatar`/`group`), ba CHECK cắn trên hàng sống, xuống về schema y hệt, lên lại qua. 19 đột biến trên bản sao: **17 bị bắt bởi test của chính PR**, 1 (M10a — sai tên ràng buộc trong nhánh IntegrityError của `add_post_reaction`) **chỉ probe của tôi bắt**, 1 (M6 — dời backfill xuống sau CHECK) **chỉ probe có hàng cũ của tôi bắt** trong khi tầng Postgres của PR (schema rỗng) vẫn xanh đúng như PR tự khai. Hai chỗ ấy là lỗ trong *test*, không phải trong *mã* — hành vi đã đo đúng — nên là suggestion S2/S3, không phải blocker.

**Điều kiện merge** (giữ từ thân PR và thêm một dòng, không phải blocker của review này):
1. #575 merge trước rồi đổi base về `main`; chạy lại cổng gốc trên cây gộp (bài học «gộp tự động giữ pin»).
2. ADR-0022 phải được Lead đánh ĐÃ CHẤP NHẬN (#573 còn OPEN; dòng trạng thái ADR vẫn 🟡 ĐỀ XUẤT).
3. Thân PR phải nhận dòng «XANH:» + câu «máy chủ xác nhận» của **lượt c** (bảng `--otp` trên chính `668b34e`) như thân PR tự hứa — tại thời điểm review (`updatedAt 2026-09-06T10:23:15Z`) thân PR chưa có; xem mục «Không tái lập được».

## Bảy điểm phải nhìn kỹ — mã đọc gì, probe đo gì

### (1) Mọi route có `{post_id}` là MỘT 404 cùng byte, không bao giờ 403

**Mã.** `service.py::_readable_post_or_404`: `get_post` → None → 404 `post_not_found`; có bài → `_post_facts` (bạn bè `state == accepted` đọc lúc này, thành viên đọc từ roster) → `post_audience.can_read` sai → **cùng** 404 `post_not_found`. Sáu method mới (`react_to_post`, `unreact_to_post`, `post_comment`, `list_post_comments`, `delete_post_comment`, và `read_post` viết lại) đều gọi nó **trước** mọi thứ khác; 403 chỉ xuất hiện *sau* khi bài đã đọc được (`comments_closed`, `permission_denied` khi xoá bình luận không phải của mình) nên không là oracle.

**Đo (probe fake, duyệt `app.routes` — không liệt kê tay — so `r.content` từng byte).** Người lạ gọi bài `friends` thật và id ngẫu nhiên qua mọi route có `{post_id}` (`{comment_id}` = bình luận thật của bạn, `{kind}` = `heart`, thân hợp lệ cho POST):

```
DOORS [('GET', '/posts/{post_id}'), ('POST', '/posts/{post_id}/reactions'), ('DELETE', '/posts/{post_id}/reactions/{kind}'),
       ('GET', '/posts/{post_id}/comments'), ('POST', '/posts/{post_id}/comments'), ('DELETE', '/posts/{post_id}/comments/{comment_id}')]
DOOR GET    /posts/{post_id}                          404 56 bytes equal
DOOR POST   /posts/{post_id}/reactions                404 56 bytes equal
DOOR DELETE /posts/{post_id}/reactions/{kind}         404 56 bytes equal
DOOR GET    /posts/{post_id}/comments                 404 56 bytes equal
DOOR POST   /posts/{post_id}/comments                 404 56 bytes equal
DOOR DELETE /posts/{post_id}/comments/{comment_id}    404 56 bytes equal
post_reactions == {}  ·  post_comments == 1 (không hàng nào mọc)
```

Đột biến M1 (404 thứ hai → 403): ca của PR `test_every_post_route_answers_404_for_a_post_the_actor_may_not_read` đỏ + probe đỏ. **Ghi nhận (S1):** ca của PR liệt kê sáu cửa **bằng tay** (dict `attempts`) và so `.json()`; ADR-0022 §6 viết «duyệt `app.routes`, không liệt kê tay». Hôm nay đúng 6 cửa nên hai cách trùng nhau; cửa thứ bảy thêm sau này thì ca của PR không tự biết.

### (2) Ảnh cá nhân: một cửa `read_person_photo`

**Mã.** `routes/photos.py::read_person_photo` → `service.read_person_photo`: `addressee = actor.id == person_id or repository.person_image_visible_to(...)` → `_require_permission("view_person_photo")`, mọi từ chối bắt lại thành 404 `photo_not_found` → `get_person_image` (lọc `purpose = 'personal'`) → None cũng 404 cùng câu → `_stored_image_bytes` cùng mã 404. `person_image_visible_to` (SQL): `select(Post.id).where(Post.image_url == url, self._readable_by(reader_id)).limit(1)` — đúng SQL feed; `url` dựng từ hai `UUID` đã parse nên luôn chữ thường, khớp `PhotoRef.url` chuẩn hoá lúc ghi bài. Test AST `test_person_photo_gate_is_one_door.py` đếm đúng một lời gọi `person_image_visible_to` trong `service.py` và một trong `read_person_photo`; không module route nào khác chứa `get_person_image`.

**Đo (fake + Postgres thật qua HTTP).** Mọi đường lách tôi thử đều đóng:

| Thử lách | fake | PG thật |
|---|---|---|
| Bài `only_me` trỏ ảnh → bạn / cùng nhóm / lạ / đã rời nhóm | 404 ×4, chủ 200 | 404 ×4, chủ 200 |
| Path viết HOA khi chưa có bài đọc được | 404 | — |
| Bài `group` → thành viên 200; bạn-không-cùng-nhóm 404; lạ 404; **đã rời** (`left_at`) 404 | ✓ | ✓ |
| Path viết HOA bởi thành viên → 200; bởi người lạ → 404 **cùng byte** với ảnh không tồn tại | ✓ | ✓ |
| Route ảnh nhóm `/contexts/{ctx}/photos/{photo}` với ảnh cá nhân, bởi thành viên | 404 | 404 |
| Route avatar `/people/{id}/avatar` sau khi có ảnh cá nhân | 404 | 404 |
| Bài `public` → lạ 200; xoá bài `public` → lạ 404, thành viên (bài nhóm còn) 200; xoá hết bài → chỉ chủ 200 | ✓ | xoá bài nhóm → thành viên 404, bạn (bài bạn bè còn) 200 |
| Bài trỏ vào **ảnh đại diện** của chính mình như ảnh cá nhân | 404 `photo_not_found` | — |
| Người khác đăng bài trỏ ảnh của tôi (tồn tại / không tồn tại) | 403 **cùng byte** cả hai, không hàng bài | 403, lạ vẫn 404 |
| Kỷ niệm / tin nhắn nhận URL cá nhân | 422 (ca của PR; `RelativePhotoUrl` không đổi) | — |
| Không có role nào | 404 cùng byte | — |

Đột biến: M2 bỏ `_readable_by` khỏi gate SQL → ca PG của PR `test_the_gate_follows_the_post_and_closes_when_the_post_is_gone` đỏ; M14 `addressee = True` và M15 ném 403 thay 404 → ca fake của PR đỏ; M7 «trộm» (bỏ kiểm chủ + tra ảnh theo id trong URL) → ca của PR đỏ; M8 ảnh nhóm cho mọi audience → ca của PR đỏ.

### (3) `purpose`

**Mã.** Migration `7e3c9a5f1d64`: `add_column purpose DEFAULT 'group'` → `UPDATE … SET purpose='avatar' WHERE owner_person_id IS NOT NULL` → hai CHECK (`purpose IN (...)`, `(purpose='group') = (context_id IS NOT NULL)`) → đổi predicate `ix_uploaded_images_avatar` thêm `purpose='avatar'` → `ix_uploaded_images_personal` → `ix_posts_image_url` một phần. `models.py` khai đúng hai CHECK và hai index (ca «migration khớp models» nằm trong 3310 ca xanh). `get_latest_avatar` lọc `purpose == 'avatar'`; `get_person_image` lọc `'personal'`; upload avatar ghi `'avatar'`, upload cá nhân ghi `'personal'`, ảnh nhóm mặc định `'group'`.

**Đo — probe migration với DỮ LIỆU CŨ (ô thân PR tự nhận «không chứng minh»).** DB dùng-một-lần: `alembic upgrade 6d2b8f4e0c53` → seed 1 người, 1 nhóm, 1 ảnh nhóm (`context_id`), 1 ảnh đại diện (`owner_person_id`) → `upgrade 7e3c9a5f1d64`:

```
purpose after upgrade: {'group-img': 'group', 'avatar-img': 'avatar'}
wall_comment_policy of old person: readers
  constraint ck_uploaded_images_image_purpose_matches_owner = CHECK ((((purpose)::text = 'group'::text) = (context_id IS NOT NULL)))
  index ix_uploaded_images_avatar = btree (owner_person_id, created_at DESC) WHERE ((owner_person_id IS NOT NULL) AND ((purpose)::text = 'avatar'::text))
  CHECKs bite on live rows: 3/3        (personal có context_id · group không context_id · 'selfie')
tables touched by upgrade (columns/constraints/indexes): ['people', 'post_comments', 'post_reactions', 'posts', 'uploaded_images']   (posts: chỉ index)
```

Đột biến M6 — dời câu UPDATE xuống **sau** hai `create_check_constraint`: probe có hàng cũ **đỏ** (`psycopg.errors.CheckViolation: check constraint "ck_uploaded_images_image_purpose_matches_owner" … is violated by some row`), còn `tests/postgres/test_person_photo_gate_postgres.py + test_post_social_postgres.py` trên schema rỗng **vẫn 10 passed** — đúng lỗ PR tự khai (S3). M3 bỏ lọc `purpose` ở `get_latest_avatar` → ca PG của PR `test_the_newest_personal_photo_is_not_the_avatar` đỏ.

### (4) `can_comment` do máy chủ quyết, theo policy của CHỦ BÀI

**Mã.** `_wire_posts`: `author = get_person(record.author_id)`; `policy = "nobody" if author is None else author.wall_comment_policy`; `can_comment = post_audience.can_comment(_post_dict(record), policy=policy, reader_id=reader, is_friend, is_group_member)` — hai fact của **người đọc** với **tác giả**, policy của **tác giả**. Domain: `can_read` trước, tác giả miễn, `readers` → đúng, `friends` → `is_friend`, từ lạ → sai (bảng chân trị 3×4×2×2 = 48 ca + tác giả + từ lạ). `post_comment` tính lại cùng công thức rồi 403 `comments_closed`. `can_delete_comment`: tác giả bình luận hoặc chủ bài, không có vai moderator; `delete_post_comment` 404 `comment_not_found` khi bình luận thuộc bài khác.

**Đo (probe fake).** Người ĐỌC tự đặt `nobody` cho tường mình → trên bài `public` của tác giả `readers`: `can_comment` vẫn `true` với bạn / lạ / cùng nhóm, lạ bình luận 201. Tác giả đổi `friends` → bạn `true`, lạ `false` và 403; `/posts` (feed) và `/people/{id}/posts` (tường) mang cùng câu trả lời cho cùng người đọc; phản ứng vẫn mở cho mọi người đọc được. Đột biến M4a (`post_comment` bỏ policy) / M4b (`_wire_posts` bỏ policy) / M12 (bỏ miễn trừ tác giả) / M9 (bỏ kiểm bình luận thuộc bài) → ca của PR đỏ cả bốn.

### (5) Đếm bằng hai GROUP BY

**Mã.** `post_social_counts`: ba câu riêng trên một trang — `GROUP BY (post_id, kind)` cho phản ứng, `GROUP BY post_id` cho bình luận, và `(post_id, kind)` của chính người xem; không join. Không cột đếm nào trong migration.

**Đo (PG thật qua HTTP).** Hai bài trên một trang: A có 2 ❤ (bạn 1, bạn 2) + 3 bình luận, B có 1 🔥 + 1 bình luận → `/people/{id}/posts` và `/posts` đều trả A `[{heart,2}]`/`comment_count 3`/`my_reactions [heart]`, B `[{fire,1}]`/`1`/`[fire]` — không chảy sang nhau. Đột biến M5 (đếm bình luận qua join với `post_reactions`) → ca PG của PR `test_three_hearts_and_two_comments_read_three_and_two_over_http` đỏ. Thêm: **chạm gõ hai lần qua HTTP trên PG** → 200 cả hai, `count 1`, đúng **một** hàng; gỡ hai lần → 200 cả hai (nhánh IntegrityError thật của `add_post_reaction` chạy được). Phân trang bình luận keyset trên PG: 3 bình luận, `limit=2` → `[a0,a1]` có `next_cursor` → `[a2]`, `has_more=false`, không lặp không sót.

### (6) Client

`coTheBinhLuan` là `bai.can_comment === true` — chỉ đọc cờ, thiếu cờ là đóng; đột biến C1 (`!== false`) đỏ ở ca của PR. `nguonAnhBai` rẽ theo tiền tố `/people/`|`/contexts/`, nối `BASE_URL` **một** lần, URL tuyệt đối → `null`; C2 (trả source cho URL tuyệt đối) đỏ. `cauTuongTacBai` chỉ đếm `heart`; C3 (đếm mọi loại) đỏ. Grep các file client thêm/đổi: không hex (`#[0-9a-fA-F]{3,8}`), không `—`, không `?? id` (chỉ `?? ""`/`?? 0`/`?? []`). Route mới có literal client: `duongDanAnhNguoi`, `taiAnhCaNhanLen("/people/me/photos")`, sáu đường trong `bai-chi-tiet.ts` — `check_server_routes_called` rc=0 «Không có route mới nào bị bỏ rơi», `.server-routes-uncalled.json` không đổi (khớp «không thêm dòng»). Hai khai báo route bytes/204: `ROUTES_WITHOUT_RESPONSE_VALIDATION` +2, `test_photo_bytes_present_but_empty.py::ROUTES` +`read_person_photo` (đều có trong diff). `app/posts/[id].tsx` → `BaiChiTietScreen`; `check_screens_reachable` 42/42.

### (7) Migration `7e3c9a5f1d64`

Xuống được **với dữ liệu**: seed thêm 1 bài + 1 phản ứng + 1 bình luận + policy `nobody` rồi `downgrade 6d2b8f4e0c53` → `columns diff: none · constraints diff: none · indexes diff: none` so với snapshot trước khi lên; ảnh và bài giữ nguyên, hai bảng xã hội biến mất; `upgrade head` lại → `purpose` backfill lại đúng, policy về `readers`. Không bảng tiền nào bị chạm (danh sách bảng đổi ở trên). `check_alembic_heads`: một head `7e3c9a5f1d64`; biên dịch offline có dòng `Running upgrade 6d2b8f4e0c53 -> 7e3c9a5f1d64`.

## Flow 42 và harness — đối chiếu tĩnh (không có emulator)

Mọi nhãn trong `apps/mobile/.maestro/42-binh-luan-bai.yaml` có trong mã: «Bài bạn đã đăng và ai đọc được» (`Profile.tsx`), `Mở bài: ${bai.body}` (Pressable tường), «Ảnh bài đăng» / «Chưa tải được ảnh» (`onError`), «Ô viết bình luận» (`Field`), «Thích» + glyph `❤️` (`PHAN_UNG` trong `tin-song.ts`; byte U+2764 U+FE0F giống nhau ở YAML và mã), «Gửi bình luận», «1 tim · 1 bình luận» (`cauTuongTacBai` là một node Text riêng trên màn chi tiết; trên tường là `.* · 1 tim · 1 bình luận` trong một Text), «Hồ sơ của bạn», `Bình luận: ${c.nhan}` (Chip radiogroup), hai câu giải thích trong `CHINH_SACH`, «Đăng bài mới», «Ai đọc được?», «Chọn ảnh». `_dang-nhap-d.yaml`, `_bo-qua-dev-menu.yaml` có mặt. `tests/test_maestro_flows_dev_client.py` thêm 42 vào danh sách và vào case `--otp` (ca xanh trong cổng gốc).

Harness `scripts/mobile_native.sh`: `da_chay` giờ định nghĩa ở dòng 319, trước vòng lặp flow (~1632) và trước `chuan_bi_bai_cho_42` (1006)/`kiem_may_chu_sau_42` (1049); mọi hàm mà hai hàm ấy gọi (`dang_nhap_curl` 273, `hong` 130, `kiem_can_25` 1728 — gọi từ khối cuối, sau định nghĩa) tồn tại. Tôi quét lại lời khai «không còn hàm nào cùng lỗi» bằng script riêng (40 hàm; tìm lời gọi ở tầng ngoài thân hàm trước dòng định nghĩa): **không có**. `kiem_may_chu_sau_33` loại `Bai co anh QA` khỏi phép đếm «đúng 1 bài» — đúng như thân PR kể về lượt b.

## Cổng tự chạy (cây sạch `/tmp/review-pr-576` tại `668b34e`)

| Cổng | Kết quả của tôi | Thân PR |
|---|---|---|
| `python3 -m pytest services/api/tests tests -q` (gốc) | `3310 passed, 690 skipped, 2 warnings, 5440 subtests passed in 613.23s` · rc=0 | 3310 / 690 / 5440 ✓ |
| `scripts/postgres_tier.sh -q` (container riêng của script) | `tests/postgres`: `628 passed in 149.11s` · `../../tests/qa`: `88 passed, 19 subtests passed in 16.44s` · ĐẠT cả hai · rc=0 | 628 · 88 ✓ |
| `cd apps/mobile && npm ci --no-audit --no-fund` | `added 659 packages` rc=0 | — |
| `rm -rf dist-test && npx --no-install tsc --noEmit` | rc=0, 0 lỗi | 0 lỗi ✓ |
| `MOBILE_REQUIRE_WEB_A11Y=1 npm test` | `# tests 705 · # pass 705 · # fail 0` rc=0 | 705/705 ✓ |
| `python3 scripts/check_server_routes_called.py` | rc=0 «Không có route mới nào bị bỏ rơi.» | ✓ |
| `check_api_contract` / `check_screens_reachable` / `check_actor_headers` / `check_cors_contract` | rc=0 · «102 route … khớp hợp đồng» · «42/42 màn … 0 pin» · «249 lời gọi đều gửi X-Actor-ID» · «Mọi header và method … qua được preflight» | rc=0 · 42/42 ✓ |
| `python3 scripts/repo_guard.py range d094deb HEAD` · `tree HEAD` | «passed commit range: 12558 file scan(s) in 8 commit(s)» · «passed tracked tree: 1594 file scan(s)» · rc=0 | PASS ✓ |
| `scripts/ruff_changed.sh d094deb` | rc=0 · «All checks passed!» · «45 files already formatted» | 0 / 0 ✓ |
| migration biên dịch offline + `check_alembic_heads.py` | rc=0 · `Running upgrade 6d2b8f4e0c53 -> 7e3c9a5f1d64` · «mot head duy nhat (7e3c9a5f1d64)» | ok · một head ✓ |

Ghi nhận ngoài PR: ở SHA này **không file nào trong `apps/mobile/` đọc `MOBILE_REQUIRE_WEB_A11Y`** (các test web đọc cờ đã đi cùng App B ở `982afc0`/`b592833`/`1746426`; chỉ `.github/workflows/test.yml`, `scripts/gate.sh`, `tests/qa/qa-tt-0020/…` còn nhắc). `npm test` có hay không có cờ là cùng 705 ca — cờ là nghi thức thừa hưởng, không phải lỗi của lát này (S7).

## Đột biến (bản sao ngoài repo; mỗi đột biến vá → chạy → khôi phục file từ cây review; `__pycache__` xoá, `PYTHONDONTWRITEBYTECODE=1`)

| # | Đột biến | Mong | Thấy | Ai bắt |
|---|---|---|---|---|
| M1 | `_readable_post_or_404`: 404 thứ hai → 403 | ĐỎ | ĐỎ | ca PR `test_every_post_route_answers_404…` |
| M2 | gate SQL bỏ `_readable_by` | ĐỎ | ĐỎ | ca PG PR `test_the_gate_follows_the_post…` |
| M3 | `get_latest_avatar` bỏ lọc `purpose` | ĐỎ | ĐỎ | ca PG PR `test_the_newest_personal_photo_is_not_the_avatar` |
| M4a | `post_comment`: policy ép `readers` | ĐỎ | ĐỎ | ca PR `test_the_wall_owner_decides…` |
| M4b | `_wire_posts`: policy ép `readers` | ĐỎ | ĐỎ | ca PR (cùng ca) |
| M5 | đếm bình luận qua join `post_reactions` | ĐỎ | ĐỎ | ca PG PR `test_three_hearts_and_two_comments…` |
| M6 | migration: backfill dời xuống sau CHECK | ĐỎ | **PR tier XANH (10 passed) / probe hàng cũ ĐỎ** | chỉ probe của tôi |
| M7 | đăng ảnh người khác (bỏ kiểm chủ + tra theo id URL) | ĐỎ | ĐỎ | ca PR `test_a_post_may_show_ones_own_photo…` |
| M8 | ảnh nhóm cho mọi audience | ĐỎ | ĐỎ | ca PR `test_a_group_photo_illustrates_only…` |
| M9 | xoá bình luận không kiểm thuộc bài | ĐỎ | ĐỎ | ca PR `test_a_comment_is_deleted_by…` |
| M10a | `add_post_reaction`: sai tên ràng buộc, chạy 16 ca PR (PG + fake) | XANH | **XANH** | không ai — lỗ test |
| M10b | cùng đột biến, probe chạm-hai-lần qua HTTP trên PG | ĐỎ | ĐỎ | chỉ probe của tôi |
| M12 | domain bỏ miễn trừ tác giả | ĐỎ | ĐỎ | ca PR `test_the_author_may_always_comment…` |
| M14 | `read_person_photo`: `addressee = True` | ĐỎ | ĐỎ | ca PR `test_the_owner_reads_their_own_photo…` |
| M15 | `read_person_photo`: ném 403 thay 404 | ĐỎ | ĐỎ | ca PR (cùng ca) |
| C1 | `coTheBinhLuan` → `!== false` | ĐỎ | ĐỎ | ca PR «coTheBinhLuan đọc đúng cờ…» |
| C2 | `nguonAnhBai` nhận URL tuyệt đối | ĐỎ | ĐỎ | ca PR «nguồn ảnh của bài…» |
| C3 | `cauTuongTacBai` đếm mọi loại | ĐỎ | ĐỎ | ca PR «đếm và câu tương tác…» |

## Lời khai trong thân PR mà tôi KHÔNG tái lập được

- **Ba lượt bảng emulator (a/b/c)** và ba ảnh chụp `42-chi-tiet-bai` / `42-chinh-sach` / `42-dang-bai-co-anh`: không có emulator trong review này (theo phân công); chỉ đối chiếu tĩnh nhãn ↔ mã như trên. Riêng **lượt c** (trên chính `668b34e`) thân PR nói «đang chạy lúc mở PR» và hứa cập nhật — tại `updatedAt 10:23:15Z` chưa có dòng nào; đây là điều kiện merge 3.
- **`scripts/e2e_slice.sh` 10/10 ca node e2e**: không chạy — script tự dựng container `mobile-e2e-pg-*` và uvicorn cổng ngẫu nhiên, nhưng cùng họ tên với stack đang chạy bảng của người khác và không nằm trong danh sách cổng được giao; tránh nhiễu bảng đang chạy.
- **`repo_guard.py staged`**: trên cây sạch là phép rỗng; thay bằng `tree HEAD` + `range`.

## Suggestion (không chặn)

- **S1 — Ca «không bao giờ 403» nên duyệt `app.routes` và so byte.** `test_post_comments_reactions.py::test_every_post_route_answers_404…` liệt kê sáu cửa bằng tay và so `.json()`; ADR-0022 §6 đòi «duyệt `app.routes`, không liệt kê tay». Đoạn probe của tôi thay thế trực tiếp: lọc `APIRoute` có `"{post_id}" in route.path`, thế `{comment_id}`/`{kind}`, gọi bằng người lạ với bài thật và id ngẫu nhiên, assert `real.status_code == fake.status_code == 404` và `real.content == fake.content`; cửa thứ bảy thêm sau này tự rơi vào lưới.
- **S2 — Chạm gõ hai lần qua HTTP trên Postgres chưa có ca.** M10a: đổi tên ràng buộc trong `except IntegrityError` của `add_post_reaction` thành tên sai mà 16 ca PR (PG + fake) vẫn xanh — vì fake tái hiện «tồn tại thì trả» bằng vòng `if`, còn ca PG chỉ chạm unique qua ORM. Hôm nay tên đúng (probe của tôi: 200/200, một hàng); nhưng đổi tên ràng buộc ở migration sau sẽ làm chạm-hai-lần thành 500 mà không ca nào đỏ. Thêm vào `test_post_social_postgres.py` một ca POST hai lần cùng `kind` qua `_http` rồi assert 200 + `count 1` + một hàng.
- **S3 — Backfill `purpose` cần một ca có hàng cũ.** M6 cho thấy tier (schema rỗng) không nhìn thấy thứ tự UPDATE↔CHECK. Một ca trong `tests/postgres` dùng schema riêng: `command.downgrade(cfg, "6d2b8f4e0c53")` → seed ảnh có `owner_person_id` → `command.upgrade(cfg, "head")` → assert `purpose == 'avatar'`; hoặc giữ probe này dưới `tools/`. Thân PR đã khai đúng lỗ; đây chỉ là cách đóng.
- **S4 — Thân bình luận toàn khoảng trắng được nhận (201).** `PostCommentCreateRequest.body` `min_length=1` không strip; CHECK `body <> ''` cũng cho qua `"   "`. Client trim trước khi gửi, và `MemoryCommentCreateRequest` hiện cùng hình dạng, nên không phải hồi quy; một validator strip-rồi-kiểm sẽ đóng cả hai.
- **S5 — ADR-0022 §3 nói ghim URL ảnh vào `.server-routes-uncalled.json`; PR làm tốt hơn** (literal `duongDanAnhNguoi` + `taiAnhCaNhanLen`), nên khi Lead chấp nhận ADR thì sửa câu ấy cho khớp mã, tránh người sau đi thêm một pin thừa.
- **S6 — `DangBaiScreen`: `setAnh(null)` ngay sau khi tải ảnh, trước `guiBai`.** Nếu `POST /posts` hỏng (422/mạng), người dùng mất ảnh đã chọn, chọn lại → tải lần hai → ảnh cá nhân mồ côi (chỉ chủ đọc được, vô hại về riêng tư). Giữ `imageUrl` đã tải trong state để lần bấm «Đăng» sau dùng lại.
- **S7 — Cờ `MOBILE_REQUIRE_WEB_A11Y=1` trong bảng cổng là nghi thức** ở SHA này (xem ghi nhận trên); nên gỡ khỏi `scripts/gate.sh` và mẫu thân PR, hoặc dựng lại cổng web cho vỏ RuDi — việc ngoài lát này.
- **S8 — Hàng bài trên tường (`HoSoNguoiScreen`) chưa có `onError` «Chưa tải được ảnh»** như màn chi tiết; ADR §3 nói «ảnh trong bài có `onError`». Flow 42 đo màn chi tiết nên không hụt bằng chứng, nhưng thống nhất hai chỗ thì bảng sau đo được cả tường.

## Cách tái lập (mọi file nằm ngoài cây PR)

1. `git worktree add /tmp/review-pr-576 668b34e --detach`; chạy bảng cổng trên như liệt kê (log ở scratchpad phiên).
2. Bản sao: `cp -r services/api <scratch>/api-copy`; probe fake `tests/api/test_probe576_fake.py` (6 ca: duyệt `app.routes`, byte 404 của route ảnh, 9 đường lách, ảnh nhóm trên bài nhóm, policy của chủ bài, ghi nhận thân trắng) → `6 passed`; probe PG `tests/postgres/test_probe576_pg.py` (3 ca: chạm hai lần, gate theo `group`/đã rời/HOA/route nhóm/xoá bài, đếm nhiều bài + phân trang) với `MOBILE_TEST_DATABASE_URL` trỏ container riêng + `MOBILE_REQUIRE_POSTGRES_TESTS=1` → `3 passed`.
3. Probe migration `probe576_migration.py` (alembic `upgrade 6d2b8f4e0c53` → seed → `upgrade 7e3c9a5f1d64` → assert → seed xã hội → `downgrade 6d2b8f4e0c53` → so snapshot `information_schema.columns`/`pg_constraint`/`pg_indexes` → `upgrade head`) với `MOBILE_DATABASE_URL` trỏ DB mới trong container ấy → `MIGRATION PROBE OK`.
4. Đột biến: driver `mutate.py` (neo phải xuất hiện đúng một lần trước khi ghi; khôi phục từ `/tmp/review-pr-576/services/api`); client: bản sao `apps/mobile` đặt ở `<root>/apps/mobile` với `packages` và `node_modules` symlink, chuỗi `tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-bai-chi-tiet.test.mjs`.
