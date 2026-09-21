# Review PR #577 — L4 «story 24 giờ cho bạn bè, hết hạn ở domain và một cửa ảnh» (ADR-0022 §2.1, §2.3)

- **Commit được review:** `8e89996e045cf233cd883e55ae1dcea814163fd5` (head PR). Lát là **một** commit trên base; đọc `git diff origin/claude/p0-w-m15-l3-tuong..8e89996e`: 39 file, +3315/−24.
- **Nhánh → base:** `claude/p0-w-m15-l4-story` → `claude/p0-w-m15-l3-tuong` (`faf0cd9`, PR #576 — APPROVE ở `docs/archive/claude/2026-09-06/review-pr-576.md`). `git merge-base origin/main HEAD` = `d094deb` = `origin/main`: L1…L4 xếp chồng thẳng trên main.
- **protocol_version:** `v1` (`docs/protocol/v1/01-giao-thuc-thuc-dia.md`), như các doc #574/#575/#576.
- **Reviewer:** Claude, context độc lập (không phải tác giả, không phải reviewer của #574–#576), ADR-0007. Worktree sạch `/tmp/review-pr-577` detached tại `8e89996e`, `.env` chép từ cây làm việc; **không sửa mã của PR**. Mọi probe và mọi đột biến chạy trên **bản sao** của `services/api` và `apps/mobile` trong scratchpad (`api-copy`, `api-mut`, `mobile-mut-root`), không file probe nào nằm trong cây PR. Postgres cho probe là container riêng của reviewer (`postgres:16-alpine`, `review577-pg-3861063`, cổng loopback ngẫu nhiên 44670), đã xoá; `postgres_tier.sh` và `e2e_slice.sh` tự dựng container riêng của chúng (`mobile-tier-pg-3841226-…` / `mobile-e2e-pg-3871169-…`), đã xoá. Không chạm cổng 48689, không chạm `mobile-e2e-pg-*` của lane khác, không adb/emulator.
- **Ngày:** 2026-09-06.

## VERDICT: APPROVE

**Blocker còn mở: không.**

Chín điểm được yêu cầu nhìn kỹ đều đo được và đều khớp lời khai của thân PR. Mọi cổng tôi tự chạy đều xanh; con số khớp thân PR ở mọi chỗ trừ ba lệch nhỏ **đã giải thích được và đều theo hướng có lợi cho PR** (bảng cổng dưới): pytest gốc 3336/703 thay vì 3337/700 (một ca phụ thuộc `node_modules`), tier Postgres **640** thay vì 638 (thân PR đếm thiếu 2; `--collect-only` ra 640 ở head, 628 ở base), ruff 57 file thay vì 56. Tôi lấp thêm ba ô: (a) **migration L4 lên/xuống trên dữ liệu thật** kèm diff `information_schema` — chỉ thêm `stories`/`story_views`, không cột/ràng buộc/index nào của 12 bảng tiền đổi; (b) **duyệt `app.routes`** thay vì liệt kê tay cho «404 cùng byte» trên cả fake lẫn Postgres thật, thêm người **chờ duyệt**; (c) **stack prod-auth thật** (`e2e_slice.sh --keep`): `seen`/`DELETE` không kèm `Idempotency-Key` đúng như client gửi, người thứ ba 404 cùng byte ở ba cửa, không phiên → 401. 15 đột biến máy chủ trên bản sao: **13 bị test của chính PR bắt**, 1 (M15) xanh **đúng như dự đoán** (kiểm `can_view` lần hai là dư thừa có chủ ý, cùng hình với bài), 1 (M9 — purge quét cả ảnh không phải `personal`) **lọt** vì fixture ca sweep không có ảnh đại diện/ảnh nhóm — lỗ trong *test*, mã ship vẫn lọc đúng → suggestion S1. 5/5 đột biến client bị bắt.

**Điều kiện merge** (giữ từ thân PR và doc #576, không phải blocker của review này):
1. ADR-0022 phải được Lead đánh ĐÃ CHẤP NHẬN (#573 còn OPEN; file ADR chưa có trên `main`, dòng trạng thái vẫn 🟡 ĐỀ XUẤT). PR này còn **sửa hai câu của chính ADR** (§2.4, §6) cho khớp bản làm — khi #573 merge trước, hunk ấy phải còn áp được (S7).
2. #576 merge trước rồi đổi base về `main`; chạy lại cổng gốc trên cây gộp (bài học «gộp tự động giữ pin»).
3. Thân PR phải nhận dòng «XANH:» của **lượt d** (bảng `--otp` trên chính `8e89996e`) như thân PR tự hứa — tại thời điểm review (`updatedAt 2026-09-06T12:02:33Z`, thân PR không đổi từ lúc tôi bắt đầu) dòng ấy chưa có; lượt c chạy trên «commit này trừ màu PNG của harness», không phải SHA head. Xem «Không tái lập được».

## Chín điểm phải nhìn kỹ — mã đọc gì, probe đo gì

### (1) Hạn 24 giờ tính ở domain; SQL lọc theo `now` của service; nghiêm ngặt ở đúng mốc

**Mã.** `story_visibility.expires_at_for(created_at) = created_at + STORY_TTL` (24h, hằng chứ không phải tham số; từ chối datetime naive bằng `StoryError("NAIVE_DATETIME")`). `service.create_story` gọi `_now()` một lần rồi truyền cả `now` lẫn `expires_at=expires_at_for(now)` xuống `repository.create_story`; migration `8f4d0b6a2e75` tạo `expires_at` **không** `server_default` (chỉ `created_at` có `now()`), CHECK `expires_at > created_at`; `is_live` là `expires_at > now`, SQL `_story_readable_by` là `Story.expires_at > now` với `now` là bind param — `grep func.now()` trên `repository.py`/`story_purge.py`/`story_visibility.py` chỉ ra dòng docstring nói «never `func.now()`». `tests/db/test_story_deadline_has_no_server_default.py` đọc AST migration (và tự chứng minh máy quét nhìn thấy `server_default` của `created_at`) + đọc `Story.__table__.c.expires_at` (không `server_default`, không `default`, NOT NULL).

**Đo.** Probe P2 trên Postgres thật: `EXPIRES_AT_COLUMN ('stories', 'expires_at', 'timestamp with time zone', 'NO', '')` — NOT NULL, default rỗng. Đột biến: M5 (migration thêm `server_default`) → AST test đỏ `test_the_migration_gives_expires_at_no_server_default`; M6 (model thêm `server_default`) → `test_the_model_agrees` đỏ; M2 (SQL `>=`) và M1 (SQL bỏ lọc hạn) → `test_the_deadline_is_strict_and_closes_the_photo` đỏ; M3 (domain `>=`) → `test_liveness_is_strict_at_the_deadline[at-the-deadline]` đỏ. Ca PR đứng ở «một giây trước / đúng mốc / một giây sau» trên cả fake, domain và Postgres qua HTTP.

### (2) Hai cách viết một luật: `can_view` ↔ `_story_readable_by`

**Mã.** `can_view`: audience lạ → sai (kể cả tác giả); tác giả → đúng (kể cả hết hạn, để còn xoá); bị chặn → sai; hết hạn → sai; còn lại `bool(is_friend)`. SQL: `Story.author_id == reader OR (audience = 'friends' AND expires_at > now AND EXISTS friend_requests ACCEPTED hai chiều)`. `list_live_stories_for` thêm `expires_at > now` cho **mọi** hàng (kể cả của mình) rồi service kiểm lại từng hàng bằng `can_view` với `is_friend` đọc lúc này qua `_is_friend` (`state == accepted`, không tin caller). Bảng chân trị:

| Người đọc | trước hạn | đúng hạn | sau hạn |
|---|---|---|---|
| tác giả (`can_view` / SQL / dải) | ✓ / ✓ / ✓ | ✓ / ✓ / ✗ (dải lọc thêm `expires_at > now`) | ✓ / ✓ / ✗ |
| bạn (accepted) | ✓ / ✓ / ✓ | ✗ / ✗ / ✗ | ✗ / ✗ / ✗ |
| chờ duyệt (pending) | ✗ / ✗ / ✗ | ✗ | ✗ |
| người lạ | ✗ / ✗ / ✗ | ✗ | ✗ |
| bạn nhưng `is_blocked` | ✗ (domain; SQL chưa có — L5) | ✗ | ✗ |

**Đo.** Ca PR `test_the_domain_and_the_sql_agree_on_every_reader[0 | DAY | DAY+1s]` (author/friend/pending/stranger) xanh trong tier 640; probe P3 của tôi (Postgres thật qua HTTP) thêm người **chờ duyệt**: `PG_DOOR Chờ duyệt … 404 equal` ở cả hai cửa, feed rỗng, ảnh 404. Đột biến M13 (bỏ `expires_at > now` thứ hai trong `list_live_stories_for`) → ca `test_the_deadline_is_strict…` đỏ ở «dải của chính mình cũng chỉ còn story còn hạn». M15 (service bỏ kiểm `can_view` lần hai trên dải) → **18 passed** — không ca nào đỏ, đúng như tôi dự đoán: SQL và fake đã lọc; đây là lớp dư thừa có chủ ý, cùng hình với `read_post`, và ghi lại để người sau không đọc nhầm nó là lớp duy nhất. **Ghi nhận nhỏ (S9):** ở góc «audience lạ» hai cách viết không giống hệt — domain đóng cả với tác giả, SQL mở cho tác giả — nhưng CHECK `audience IN ('friends')` làm góc ấy không tới được (ca `test_the_checks_refuse_what_the_domain_refuses[audience]`).

### (3) Mọi route có `{story_id}` với người lạ: MỘT 404 cùng byte, không bao giờ 403; bạn xoá → 403

**Mã.** `_viewable_story_or_404`: `get_story` → None → 404 `story_not_found`; có → `can_view` sai → **cùng** `ApiProblem(404, "story_not_found", "Story does not exist")` (bọc `from denied`). `mark_story_seen` và `delete_story` đều gọi nó **trước**; 403 chỉ xuất hiện sau, ở `delete_own_story` (`is_author`).

**Đo (duyệt `app.routes`, so `r.content` từng byte).** Fake (probe P1): `DOORS [('DELETE', '/stories/{story_id}'), ('POST', '/stories/{story_id}/seen')]` — `DOOR DELETE … 404 58 bytes equal` · `DOOR POST … 404 58 bytes equal`; `FRIEND_DELETE 403 {'code': 'permission_denied', 'detail': 'is_author'}`; người lạ không để lại hàng `story_views`. Postgres thật (P3): người lạ **và** chờ duyệt, cả hai cửa `404 equal`; `PG_FRIEND_DELETE 403`. Stack prod-auth (curl, xem mục «Stack thật»): `BA_SEEN real=404 fake=404 bytes_equal=yes body={"code":"story_not_found","detail":"Story does not exist"}` · `BA_DELETE real=404 fake=404 bytes_equal=yes` · `TRANG_DELETE {"code":"permission_denied","detail":"is_author"} 403` · `MINH_DELETE_NO_KEY 204` · `MINH_DELETE_AGAIN 404`. Đột biến M4 (404 thứ hai → 403) → ca PR `test_seen_is_per_reader_and_the_first_look_counts` đỏ; M12 (bỏ `delete_own_story`) → `test_only_the_author_takes_a_story_down` đỏ. **Ghi nhận (S5, cùng S1 của #576):** ca của PR liệt kê hai cửa bằng tay và so `.json()`; ADR-0022 §6 đòi «duyệt `app.routes`». Hôm nay hai cách trùng nhau.

### (4) Cửa ảnh cá nhân vẫn là MỘT, mở thêm qua story còn hạn, đóng đúng hạn

**Mã.** `service.read_person_photo` vẫn là nơi duy nhất gọi `person_image_visible_to` (`grep` ra đúng một dòng 1760; `tests/api/test_person_photo_gate_is_one_door.py` xanh). `person_image_visible_to(…, now=_now())` giờ là `shown_by_post OR shown_by_story` với `shown_by_story = select(Story.id).where(Story.image_url == url, _story_readable_by(reader, now))` — cùng SQL với dải. `create_story` chỉ nhận `PersonPhotoUrl` (422 cho dạng nhóm), từ chối ảnh của người khác (403 `permission_denied`), đòi ảnh tồn tại với `purpose='personal'` (404 `photo_not_found`, nên ảnh đại diện không thành story).

**Đo.** Fake: `test_a_live_story_opens_the_photo_and_the_deadline_closes_it` (bạn 200 → đúng hạn 404, chủ 200, groupmate/lạ 404) và `test_a_post_showing_the_same_photo_keeps_it_open_after_the_story`. Postgres qua HTTP: `test_the_deadline_is_strict_and_closes_the_photo`; probe P3: bạn **đã có hàng `story_views`** vẫn mất ảnh ở đúng hạn (`404`). Stack thật: `TRANG_PHOTO 200` → sau khi chủ xoá story `TRANG_PHOTO_AFTER_DELETE 404`; `BA_PHOTO real=404 fake=404 bytes_equal=yes body={"code":"photo_not_found",…}`. Đột biến M14 (bỏ nhánh story khỏi cửa) → `test_a_story_reaches_a_friend_and_never_leaves_the_database_for_a_stranger` đỏ; M11 (bỏ kiểm chủ ảnh) → `test_only_ones_own_existing_photo_becomes_a_story` đỏ.

### (5) `story_views` khoá chính cặp; `seen` của người đọc; CASCADE

**Mã.** Migration: `PrimaryKeyConstraint("story_id","viewer_id", name="pk_story_views")`, FK `fk_story_views_story … ondelete="CASCADE"`; `mark_story_seen` = `INSERT … ON CONFLICT ON CONSTRAINT pk_story_views DO NOTHING` rồi SELECT `seen_at` (lần đầu là lần được ghi, không có «hỏi rồi mới ghi»); `list_live_stories_for` LEFT JOIN `StoryView` theo `(story_id, viewer_id == reader)` nên `seen` là của **người đọc**; `delete_story` là `session.delete(Story)` và DB cascade (không relationship ORM nào chen vào).

**Đo.** P2 (SQL tay trên schema thật): `ON CONFLICT ON CONSTRAINT pk_story_views DO NOTHING` giữ **1** hàng với `seen_at` của lần đầu; `DELETE FROM stories` → `count(story_views) = 0`; `pg_get_constraintdef`: `FOREIGN KEY (story_id) REFERENCES stories(id) ON DELETE CASCADE`. Ca PR `test_seen_is_one_row_per_look_and_goes_with_the_story` (Core insert thứ hai → `IntegrityError` có `pk_story_views`; xoá story → hàng xem biến mất). Stack thật: `TRANG_SEEN_NO_KEY 200` → `TRANG_FEED_AFTER all_seen= True seen= True` trong khi `MINH_FEED_OWN seen(by author)= False all_seen= False` — không lây. Đột biến M10 (`DO UPDATE seen_at`) → ca PR đỏ ở `again.json()["seen_at"] == first.json()["seen_at"]`.

### (6) Purge: chỉ story quá hạn > ân hạn, chỉ ảnh `personal` mồ côi, dry-run không ghi

**Mã.** `story_purge.purge_expired_stories`: `cutoff = now − older_than` (mặc định 7 ngày); `stale = expires_at <= cutoff`; xoá story rồi tính `referenced = Post.image_url ∪ Story.image_url` (dry-run thì loại stale ra khỏi tập story còn lại); mồ côi = `UploadedImage.purpose == 'personal'` không nằm trong `referenced`; `dry_run` không `DELETE` nào, script `rollback()` khi dry-run; file xoá **sau** hàng, `PhotoStorage.delete` nuốt `FileNotFoundError`. Từ chối `now` naive. Không job nền, không trigger (không có trong migration).

**Đo.** Ca PR `test_the_sweep_removes_only_what_nothing_points_at` (2 story cũ 8 ngày, 1 story hết hạn 1 phút còn ân hạn, 1 ảnh chung với bài, 1 ảnh mồ côi: dry-run báo (2, 2) và không xoá gì; chạy thật: bài giữ ảnh chung, story trong ân hạn giữ ảnh, ảnh mồ côi + ảnh story cũ đi cả hàng lẫn file). Đột biến M7 (bỏ ân hạn) và M8 (dry-run vẫn xoá) → ca đỏ. **M9 (bỏ lọc `purpose == 'personal'`) → 10 passed, lọt**: fixture không có ảnh đại diện hay ảnh nhóm nên phạm vi xoá mở rộng không đo được — S1.

### (7) Client

**Mã.** `story.ts`: bốn literal `"/stories"`, `` `/stories/${storyId}/seen` ``, `` `/stories/${storyId}` `` qua `translatedAsActor` (bearer); `dangStory` mang `attempt` (Idempotency-Key), `danhDauDaXem`/`xoaStory` không mang — **đúng với máy chủ**: middleware `app.api.idempotency` cho qua write không có key (`if key is None: await self.app(...)`), đo trên stack prod-auth `TRANG_SEEN_NO_KEY 200`, `MINH_DELETE_NO_KEY 204`. `nhanVong` = «Story của bạn» / «Story của <tên>, chưa xem|đã xem» từ `all_seen` của máy chủ, không rơi về id; `conHan` là `han > nowMs`, chuỗi hỏng đọc là hết hạn. `StoryRail`: ô «Đăng story» (label «Đăng story mới») render **ngoài** mọi nhánh `trang.pha`, nên có mặt khi đang đọc / rỗng / lỗi; vòng accent khi `!all_seen && !cuaToi`; đọc `GET /stories` ở mỗi `useFocusEffect`. `XemStoryScreen`: `daBao` (ref `Set`) báo «đã xem» một lần mỗi story mỗi lần mở; đồng hồ 5 s dừng ở story cuối (`het`) chứ không tự đóng; Reduce Motion → không chạy; chạm phải/trái; «Đóng story», «Xoá story» (chỉ chủ); nền bìa sơn qua `contentStyle` (prop có sẵn ở `RudiScreen`, PR không đổi `ui.tsx`). `Create.tsx` thêm «Đăng story» với `tone: mauSang.accent`. `_layout.tsx`: `stories/new` modal, `stories/[personId]` `fullScreenModal`. `DangStoryScreen`: ảnh lên trước qua `taiAnhCaNhan` rồi `dangStory`; giới hạn 200 ký tự có đếm «Còn N ký tự.».

**Đo.** `npm test` 713/713 gồm `rudi-story.test.mjs` (bốn đường, bearer, attempt, caption cắt khoảng trắng, nhãn vòng, `conHan` nghiêm ngặt, chỉ số mở, đồng hồ kẹp, câu tuổi, không «—»), `rudi-khong-hex` (`src/rudi`+`app`), `mac-dinh-am-tham-id` (AST `src/`), `dau-gach-dai`. `check_server_routes_called`: «Không có route mới nào bị bỏ rơi» (không thêm dòng `.server-routes-uncalled.json`); `check_screens_reachable` 45/45; `check_actor_headers` 260/260; `check_cors_contract` ĐẠT; `ROUTES_WITHOUT_RESPONSE_VALIDATION` +`("DELETE","/stories/{story_id}")` và meta-test trong 3336 xanh. Đột biến client trên bản sao (`tsc -p tsconfig.test.json` mỗi lượt): C1 `conHan >=` → `not ok 9 - còn hạn là NGHIÊM NGẶT ở đúng mốc`; C2a `display_name ?? id` **và** C2b `display_name || id` → `not ok 5 - không giá trị hiển thị nào lấy id thô làm mặc định` (cổng AST bắt cả `||`); C3 viewer luôn mở từ đầu → `not ok 10`; C4 gửi caption rỗng → `not ok 7`. **Không đo được bằng cổng unit** (không có test render component ở `apps/mobile/tests`): «ô Đăng story còn khi dải lỗi» và «báo đã xem một lần mỗi lần mở» — chỉ từ đọc mã và từ bảng emulator của tác giả (không tái lập).

### (8) Migration `8f4d0b6a2e75` xuống được, không đụng bảng tiền

**Đo (probe P2, Postgres 16 thật, schema riêng ở `7e3c9a5f1d64`, chụp `information_schema`/`pg_constraint`/`pg_indexes` trước và sau).** `MONEY_TABLES_PRESENT ['collection_batch_versions', 'collection_batches', 'collection_obligation_sources', 'collection_obligations', 'confirmed_allocations', 'expense_discounts', 'expense_item_shares', 'expense_items', 'expense_surcharges', 'expense_versions', 'expenses', 'receipt_confirmations']`; sau khi lên L4: bảng thêm = `{stories, story_views}`, **không cột / ràng buộc / index nào của bảng cũ đổi**; `NEW_INDEXES ['ix_stories_author_live', 'ix_stories_image_url', 'ix_story_views_viewer', 'pk_stories', 'pk_story_views']`; ba CHECK đúng tên `ck_stories_story_{audience_known,expires_after_created,caption_length}`. Cấy người + story + view → `downgrade 7e3c9a5f1d64` → ảnh chụp **y hệt** trước L4, `people` còn 2 hàng → `upgrade head` lại → ảnh chụp y hệt sau L4. `L4_MIGRATION_ROUNDTRIP ok`. Offline: `upgrade head --sql` rc=0 (hai `CREATE TABLE`), `downgrade 8f4d0b6a2e75:7e3c9a5f1d64 --sql` ra đúng 5 lệnh DROP; `check_alembic_heads`: «mot head duy nhat (8f4d0b6a2e75)»; `tests/db/test_migration_matches_models.py` xanh.

### (9) E2E node `story-het-han.test.mjs`

**Mã.** `scripts/e2e_slice.sh` +1 dòng `MOBILE_E2E_DATABASE_URL="$DATABASE_URL"` (URL host `127.0.0.1:<port>` của container dùng-một-lần, đúng thứ node trên host nối được); test tua hạn bằng `UPDATE stories SET created_at = now() − 25h, expires_at = now() − 1h WHERE id = :id` qua `python3 + SQLAlchemy` với `MOBILE_DATABASE_URL = DB_URL`; `skipUnlessMeasurable`: `MOBILE_REQUIRE_E2E` đặt mà thiếu server → `assert.fail`; thiếu phiên **hoặc thiếu URL DB** → `assert.fail`. Không route dev nào: `openapi.json` của stack thật chỉ có 4 route `stor*`, không route nào chứa `expire/rewind/clock/dev`.

**Đo.** `scripts/e2e_slice.sh --keep`: `# tests 11 · # pass 11 · # fail 0`, trong đó `ok 6 - story hết hạn rời GET /stories của cả hai và ảnh đóng lại, không xoá gì`. Probe âm trên cùng stack: `MOBILE_REQUIRE_E2E=1` + phiên + **không** `MOBILE_E2E_DATABASE_URL` → `not ok 1 … error: 'MOBILE_REQUIRE_E2E đặt rồi nhưng thiếu MOBILE_E2E_SESSIONS hoặc MOBILE_E2E_DATABASE_URL.'` (`# fail 1`); bỏ `MOBILE_REQUIRE_E2E` → `# SKIP thiếu phiên e2e hoặc URL cơ sở dữ liệu của stack` — đúng hai hành vi thân PR tả.

## Stack thật (prod-auth, `e2e_slice.sh --keep`, API 127.0.0.1:47737, DB 44671) — curl, nguyên văn

```
STORY_CREATED 744f02e8-… seen= False audience= friends
TRANG_FEED_BEFORE groups= 1 all_seen= False
TRANG_SEEN_NO_KEY 200
TRANG_FEED_AFTER  all_seen= True seen= True
MINH_FEED_OWN     seen(by author)= False all_seen= False
BA_FEED           has_minh= False
BA_SEEN real=404 fake=404 bytes_equal=yes body={"code":"story_not_found","detail":"Story does not exist"}
BA_DELETE real=404 fake=404 bytes_equal=yes
BA_PHOTO real=404 fake=404 bytes_equal=yes body={"code":"photo_not_found","detail":"Photo does not exist"}
TRANG_PHOTO 200
NO_SESSION_FEED 401
TRANG_DELETE {"code":"permission_denied","detail":"is_author"} 403
MINH_DELETE_NO_KEY 204
MINH_DELETE_AGAIN  404
TRANG_PHOTO_AFTER_DELETE 404
```
(MINH = tác giả, TRANG = bạn — hai người demo mà `story-het-han` đã kết bạn; BA = người thứ ba trong `sessions.json`, không phải bạn của MINH.) Stack, container và thư mục tạm của lượt `--keep` đã dọn ngay sau probe (`healthz` → 000).

## Bảng cổng — chạy trong worktree sạch `/tmp/review-pr-577` @ `8e89996e`, tuần tự

| Cổng | Kết quả của tôi (nguyên văn) | Thân PR | Khớp? |
|---|---|---|---|
| `python3 -m pytest services/api/tests tests -q` (gốc) | `3336 passed, 703 skipped, 2 warnings, 5449 subtests passed in 548.18s` | 3337 / 700 / 5449 | subtests khớp; lệch 1 pass do `tests/test_phone_path.py` skip «apps/mobile/node_modules chưa cài» lúc chạy gốc (sau `npm ci` chạy lại ba file phụ thuộc môi trường: `62 passed, 4 skipped`, 4 skip còn lại là 3× «không có file pin trên nhánh này» + 1× «stack 'mobile-local' chưa chạy»); còn lệch 2 skip là 2 ca thu thập thêm trong môi trường của tôi, không thuộc file story nào |
| `scripts/postgres_tier.sh -q` | `tests/postgres … 640 passed in 132.13s` · `../../tests/qa … 88 passed, 19 subtests passed` · `ĐẠT ĐẠT` | 638 · 88 | tier **640** (`--collect-only`: head 640, base `faf0cd9` 628, +12 = 10 ca `test_stories_postgres` + 1 `test_post_social` + 1 `test_l3_migration_backfill`); thân PR đếm thiếu 2 |
| `scripts/e2e_slice.sh` (`--keep` để probe, rồi tự dọn) | `# tests 11 · # pass 11 · # fail 0`, có `ok 6 - story hết hạn rời GET /stories…` | 11/11 | ✓ |
| `cd apps/mobile && npm ci --no-audit --no-fund && rm -rf dist-test && npx --no-install tsc --noEmit` | `NPMCI_RC=0` · `TSC_RC=0` (0 lỗi) | 0 lỗi | ✓ |
| `MOBILE_REQUIRE_WEB_A11Y=1 npm test` | `# tests 713 · # pass 713 · # fail 0 · # skipped 0` | 713/713 | ✓ (cờ a11y vẫn là nghi thức với vỏ RuDi — S7 của #576) |
| `scripts/ruff_changed.sh "$(git merge-base origin/main HEAD)"` | `ruff 0.9.2 (bản ghim)` · 57 file · `All checks passed!` · `57 files already formatted` | 0 finding, 56 file | 57 (đếm `git diff --name-only --diff-filter=ACMR d094deb -- '*.py'` = 57) |
| `scripts/repo_guard.py range "$(git merge-base origin/main HEAD)" HEAD` / `tree HEAD` | `Repo guard passed commit range: 15768 file scan(s) in 10 commit(s).` · `Repo guard passed tracked tree: 1615 file scan(s).` | PASS | ✓ |
| migration offline + `scripts/check_alembic_heads.py` | rc=0, 2 `CREATE TABLE`, downgrade 5 DROP · `Alembic guard: mot head duy nhat (8f4d0b6a2e75).` | ok · một head | ✓ |
| `check_server_routes_called` / `check_api_contract` / `check_screens_reachable` / `check_actor_headers` / `check_cors_contract` | rc=0 ×5 · «Không có route mới nào bị bỏ rơi» · «Máy chủ có 105 route … Client và máy chủ khớp hợp đồng.» · «45/45 màn có đường render từ cửa vào · 0 pin» · «ĐẠT — 260 lời gọi đều gửi X-Actor-ID.» · «Mọi header và method client gửi đều qua được preflight.» | rc=0 cả năm, 45/45, 260 | ✓ |
| Riêng file story (fake + domain + AST + một cửa + migration↔models) | `34 passed, 76 subtests passed` · `tests/test_maestro_flows_dev_client.py` `9 passed, 47 subtests passed` | — | — |

## Đột biến trên bản sao (`api-mut`, `PYTHONDONTWRITEBYTECODE=1`, neo phải xuất hiện đúng một lần; cây chép lại từ `api-copy` sau mỗi lượt)

| # | Đột biến | Ca đỏ (nguyên văn) |
|---|---|---|
| M1 | SQL bỏ `Story.expires_at > now` trong `_story_readable_by` | `test_the_deadline_is_strict_and_closes_the_photo` (`1 failed, 2 passed`) |
| M2 | SQL `>=` | cùng ca |
| M3 | domain `is_live` `>=` | `test_liveness_is_strict_at_the_deadline[at-the-deadline]` |
| M4 | `_viewable_story_or_404` ném 403 thay 404 | `test_seen_is_per_reader_and_the_first_look_counts` |
| M5 | migration thêm `server_default` cho `expires_at` | `test_the_migration_gives_expires_at_no_server_default` |
| M6 | model thêm `server_default` | `test_the_model_agrees` |
| M7 | purge `cutoff = now` (bỏ ân hạn) | `test_the_sweep_removes_only_what_nothing_points_at` |
| M8 | purge dry-run vẫn xoá story | cùng ca |
| **M9** | purge bỏ lọc `purpose == 'personal'` | **`10 passed` — lọt** (S1) |
| M10 | `mark_story_seen` `DO UPDATE seen_at` | `test_seen_is_one_row_per_look_and_goes_with_the_story` |
| M11 | `create_story` bỏ kiểm chủ ảnh | `test_only_ones_own_existing_photo_becomes_a_story` |
| M12 | `delete_story` bỏ `delete_own_story` | `test_only_the_author_takes_a_story_down` |
| M13 | `list_live_stories_for` bỏ `expires_at > now` thứ hai | `test_the_deadline_is_strict_and_closes_the_photo` |
| M14 | cửa ảnh bỏ nhánh story | `test_a_story_reaches_a_friend_and_never_leaves_the_database_for_a_stranger` |
| M15 | service bỏ kiểm `can_view` lần hai trên dải | `18 passed` — xanh **đúng dự đoán** (lớp dư thừa) |

Client (`mobile-mut-root/apps/mobile`, `node_modules` symlink, `tsc -p tsconfig.test.json` mỗi lượt): C1 `conHan >=` → `not ok 9`; C2a `display_name ?? id` → `not ok 5`; C2b `display_name || id` → `not ok 5`; C3 `chiSoBatDau` luôn 0 → `not ok 10`; C4 gửi caption rỗng → `not ok 7`. 5/5.

## Không tái lập được

- **Bảng emulator** (lượt a/b/c/d, `emulator-5554`, API 48689, dấu vân dev-client ghi trong thân PR, tiền tố SHA `668b34e2` là bản native dựng ở L3): tôi không có emulator và không được đụng bảng đang chạy. Đối chiếu tĩnh: flow `43-story-24h.yaml` và `_43b-story-het-han.yaml` dùng đúng nhãn trong mã — «Đăng story» (caption ô + nút gửi), «Đăng story mới» (label ô, cố ý khác caption để Maestro thấy một node), «Story của (An|Ban) QA, chưa xem|đã xem» (`nhanVong`), «Ảnh story» (label `Image`), «Story QA» (caption), «Đóng story», «Ô chú thích» (label `Field`), «Chọn ảnh», «Một tấm ảnh, chỉ bạn bè thấy, trong 24 giờ.»; `_43b` dùng `notVisible` regex + `assertNotVisible "Story QA"`. Harness: `chuan_bi_story_cho_43` đăng story cho NGƯỜI KIA qua đường sản phẩm (`POST /people/me/photos` → `POST /stories` với `Idempotency-Key`, PNG sinh tại chỗ), `kiem_may_chu_sau_43` kiểm `all_seen=true`/caption/ảnh 200, canary B (không thấy tác giả, ảnh 404 **cùng câu** với ảnh không tồn tại), rồi `hong` nếu thiếu `MOBILE_DATABASE_URL` («không đo được không phải xanh»), tua hạn thẳng trong DB, kiểm cả hai `GET /stories`, ảnh 404, rồi `chay_flow _43b`; vòng lặp bảng bỏ qua `_*` (`_*) continue`), `43-*` thuộc nhóm `--otp`, `tests/test_maestro_flows_dev_client.py` ghim cả hai điều đó (9 passed, 47 subtests). Cái tôi **không** chứng minh được: máy thật vẽ vòng, dump cây dưới tải, và lượt d trên chính `8e89996e` — thân PR hứa cập nhật dòng «XANH:» nhưng ở `updatedAt 12:02:33Z` chưa có (điều kiện merge 3).
- **Đồng hồ thật 24 giờ** — như thân PR tự khai; mọi ca hết hạn đều tua đồng hồ giả hoặc UPDATE thẳng DB.
- **`MOBILE_REQUIRE_WEB_A11Y=1`** không đo vỏ RuDi (S7 của #576, chưa đổi).

## Suggestion (không phải blocker)

- **S1 — Fixture ca sweep cần một ảnh đại diện và một ảnh nhóm.** M9 cho thấy bỏ `UploadedImage.purpose == "personal"` khỏi `purge_expired_stories` vẫn `10 passed`; ngoài đời đột biến ấy xoá mọi avatar (không bài/story nào trỏ tới avatar). Thêm vào `test_the_sweep_removes_only_what_nothing_points_at` một `POST /people/me/avatar` và một ảnh nhóm rồi assert cả hai còn nguyên hàng lẫn file.
- **S2 — Ảnh `personal` mồ côi bị dọn không theo tuổi.** Quy trình đăng là hai bước (ảnh lên trước, story/bài sau); một ảnh vừa lên mà script chạy đúng giữa hai bước sẽ bị xoá và `POST /stories` trả 404 `photo_not_found`. Thêm `UploadedImage.created_at <= cutoff` vào tập mồ côi (ân hạn cũng 7 ngày) đóng cửa sổ ấy mà không đổi luật ADR.
- **S3 — Thứ tự file ↔ commit trong script.** `purge_expired_stories` unlink file **trước** khi `scripts/purge_expired_stories.py` `commit()`; nếu commit hỏng thì hàng còn mà file mất. Trả `storage_keys` lên script và unlink **sau** commit (report đã có sẵn trường ấy).
- **S4 — «Đã xem» khi ảnh hiện.** ADR §2.4 nói «gọi «đã xem» khi ảnh hiện»; `XemStoryScreen` gọi khi `story` được chọn, trước khi `Image` tải xong (kể cả khi ảnh 404). Chuyển `danhDauDaXem` vào `onLoad` của `Image` thì lời khai và mã trùng nhau; hành vi hiện tại không rò rỉ gì.
- **S5 — Ca «không bao giờ 403» nên duyệt `app.routes`** (cùng S1 của #576): probe P1/P3 của tôi thay được trực tiếp cho `dict` hai cửa trong `test_stories.py`.
- **S6 — Con số trong thân PR:** tier Postgres là **640** (không phải 638); ruff **57** file; pytest gốc 3337/700 là số của cây có `node_modules` + stack `mobile-local` — ghi rõ môi trường để người sau không đọc 3336/703 thành hồi quy.
- **S7 — Hai câu ADR-0022 sửa trong PR này** (§2.4 «ô đầu là Đăng story…», §6 «`_43b`… `MOBILE_DATABASE_URL`») nằm trên file mà #573 đang mang lên `main`; khi #573 merge trước, kiểm hunk còn áp được, hoặc chuyển hai câu ấy sang #573 để ADR chỉ có một nguồn.
- **S8 — `_story_facts` gọi `_is_friend(actor, actor)` cho chính tác giả** — một truy vấn thừa mỗi lần tác giả tự mở/xoá story; `can_view` trả lời trước khi đọc `is_friend`.
- **S9 — Góc «audience lạ»** (mục 2): nếu muốn hai cách viết giống hệt, thêm `Story.audience.in_(STORY_AUDIENCES)` vào disjunct tác giả trong SQL; CHECK đã làm góc ấy không tới được nên chỉ là chuyện đối xứng.

## Bằng chứng đã xem

- Thân PR #577 (`updatedAt 2026-09-06T12:02:33Z`, không đổi trong suốt review), `git log/diff origin/claude/p0-w-m15-l3-tuong..8e89996e` (39 file), ADR-0022 §2.1–2.4, §6, CLAUDE.md, doc #574/#574-vòng-2/#575/#576.
- Log cổng: `/tmp/review-pr-577-logs/{pytest-root,postgres_tier,npm,e2e_slice,ruff,repo_guard_range,repo_guard_tree,alembic-offline.*,check_*}.log`; probe: `probe-prod.log`, `mutations-server.log`; probe file: `scratchpad/api-copy/tests/api/test_probe577_story_doors.py`, `tests/postgres/test_probe577_l4_migration.py`, `tests/postgres/test_probe577_story_doors_postgres.py`; đột biến: `scratchpad/mutate.py`.
- Sau khi xong: `dist-test` đã xoá, container của reviewer đã xoá, không tiến trình node/pytest nào còn lại; worktree `/tmp/review-pr-577` để nguyên theo yêu cầu.
