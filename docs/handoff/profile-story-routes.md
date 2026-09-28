# Handoff · Hồ sơ, huy hiệu nhiều ngã rẽ và tường cá nhân

Cập nhật: 2026-09-28 (lượt tích hợp main). Người nhận: agent tiếp tục vertical slice hồ sơ.

## 0. Lượt tích hợp với `main` (2026-09-28)

Nhánh `integ/profile-story-routes-main` gộp `main` ba lượt: `0f902b72`
(main `95388021`, 22 xung đột giải tay), `9d02ae67` (main `04a086f9`, AI v2
#654, 3 xung đột), `3eb79500` (main `b2dc599d`, #659, gộp sạch), cộng hai
commit sửa `26e05829`, `475da4d5`.

**Gate 22 chặng strict tại checkout sạch `3eb79500`: ĐẠT 22, HỎNG 0, BỎ QUA 0.**
API 3857 pass / 804 skip · mobile 1350/1350 · PostgreSQL 723 + QA 88 ·
Go PostgreSQL 3208 PASS có sentinel · chat E2E 43 PASS · client/server E2E
11 pass, **4 skip** (media service chưa cấu hình, không tính là đạt).

### Ranh giới tường ↔ Cộng đồng đã chốt (thay cho mục 5.2)

| Câu hỏi | Quyết định | Bằng chứng |
| --- | --- | --- |
| Một bài, một writer | `socialv2` không INSERT `post_comments`/`post_reactions`; gọi `community.WriteComment`/`WriteLike` (`internal/community/wall.go`) | `TestPostgresWallAndCommunityShareOneCommentParent` |
| Bình luận public | Cộng đồng bật → 202 + nháp chỉ tác giả thấy, vào hàng duyệt. Tắt → bài từng qua Cộng đồng trả 409 `community_unavailable`; bài public legacy đăng như route legacy | `TestPostgresPublicWallCommentWaitsForReview`, `TestPostgresReviewedPostRefusesWallCommentWithoutModerator` |
| Hai nguồn `parent_id` | Bỏ cột `post_comments.parent_id`; nguồn duy nhất `community_comment_meta.parent_id`; `socialv2.Migrate` chạy `community.Migrate` trước | ca cha/con đọc chéo hai phía |
| Like | Nút «Thích» ghi kind `heart` như Cộng đồng (trước là `like` = 👍 «Đồng ý», hai nơi đếm lệch) | `TestPostgresWallLikeIsTheCommunityLike` |
| ACL / bài chờ duyệt | Mọi route `/social/v2` thêm `community.Readable`; bài chờ duyệt 404 với người khác ở detail/comment/like/repost | `TestPostgresPendingCommunityPostStaysOffEveryWallRoute` |
| Đăng lại công khai | Cộng đồng bật → 422 `public_repost_needs_review`; nút «Mọi người» bỏ khỏi khay chia sẻ; trích đoạn gốc mất khi hết quan hệ bạn | `TestPostgresPublicRepostNeedsReviewAndOriginFollowsFriendship` |
| Manifest | Theo quy ước native của main (`native: true`, `python: absent`, `LIVE-GO`); bỏ state `GO-NATIVE` | `check_route_ownership.py` OK 227 hàng |

Ba đột biến cùng harness, dự đoán ghi trước: bỏ kiểm duyệt → đỏ «public wall
comment skipped review»; đếm lại kind `like` → đỏ «wall still counts a like
removed in Cộng đồng»; cho repost public → đỏ «public repost skipped review».
Identity xanh. Log ở `~/.cache/rudi-bang-chung/` (máy QA).

### Lỗi Git gộp «sạch» nhưng sai, hoặc chỉ lộ ở gate trên kết quả gộp

- `MAN_NEP_LUI` bị nhánh thêm màn hồ sơ; trả về danh sách màn tiền (main ghi
  rõ danh sách này không được phình). Test Go đối chiếu TS bắt được.
- Trigger sự kiện tường của `socialv2` chưa nằm trong cổng `aigate` của AI v2.
- `cmd/core` treo 30 phút khi từ chối khởi động (schema Cộng đồng thiếu):
  `socialv2.Run` giữ kết nối LISTEN, `pool.Close` chạy trước `stopChat`
  (defer LIFO). Run nay có context riêng huỷ trước pool.
- Upload native dùng `phanTepAnh` của main thay `new File(uri)` của nhánh.
- Mobile: chữ cam trên giấy, nút tắt không nói lý do, `_rut.json` rút lại,
  nhãn cũ trong flow Maestro 32/33/41/45/canary và test no-WebGL.
- Một lần `TestPostgresCommunityStableRankingReusesReaderSnapshot` trả 401
  khi cả tier chạy song song; không tái hiện ở hai lượt sau. Chập chờn, chưa
  rõ nguyên nhân — không ghi là đã sửa.

### QA native sau tích hợp (Android, 2026-09-28)

APK debug build lại từ worktree tích hợp (main thêm module native Skia /
expo-video), hai emulator (`rudi_profile_qa` sáng, `rudi_profile_qa_b` tối),
stack `chat_e2e_stack.sh` tại `3eb79500`, hai tài khoản tổng hợp đăng nhập
bằng OTP. Đã thao tác và đối chiếu từng bước với hàng PostgreSQL:

- Kết bạn bằng số điện thoại → `friend_requests.state=accepted`.
- A đăng bài có ảnh (multipart qua `phanTepAnh` của main) → một hàng `posts`.
- B mở hồ sơ A (hộ chiếu main + tường v2), mở ảnh + khay bình luận, bình
  luận → `post_comments` + `community_comment_meta` + nháp `approved`: đi qua
  writer Cộng đồng. Thích → kind `heart`.
- A trả lời bình luận của B → parent nằm ở `community_comment_meta`; trang
  bài đang mở trên máy B tự đổi «2 bình luận» và hiện trả lời, không tải lại.
- Sổ ngã rẽ: A đạt «Lời kể đầu tiên», Nếp M8 giơ tem; mở lại không lặp.
- Story đăng được ở lần đầu; khay chia sẻ không còn «Mọi người»; đăng lại
  cho bạn bè → `social_reposts` có bài gốc.

Hai lỗi UI chỉ thấy trên máy, đã sửa trong cùng lượt: chạm «Gửi» đầu tiên
khi bàn phím mở chỉ đóng bàn phím (thiếu `keyboardShouldPersistTaps`);
tiêu đề tường bị kẹp trên mục diary rỗng (đổi thứ tự). Kiểm lại trên máy:
một chạm là gửi, thứ tự đúng. Ảnh chụp ở `~/.cache/rudi-qa-couple/shots/
integ-*` trên máy QA, chưa đưa vào Git.

Chưa làm ở lượt này: Cộng đồng bật trên máy (bình luận chờ duyệt, bài chờ
duyệt) — chỉ có bằng chứng PostgreSQL; mất mạng giữa upload và tạo story;
iOS; Maestro; chữ lớn / Reduce Motion.

### Cần chủ sản phẩm quyết

- Màn Thành tích cũ (cấp độ, **thử thách tuần**, bảng tem) được thay bằng sổ
  ngã rẽ; khoảnh khắc Nếp M8 đã cấy lại, thử thách tuần không còn.
- Tường cá nhân dùng long poll `social_wall_changes`, Cộng đồng dùng
  WebSocket (ADR-0040). Hợp nhất hay giữ hai đường cần ADR.

### Còn mở sau tích hợp

- Hàng trên tường chưa hiện media của bài Cộng đồng (`community_media`); bài
  Cộng đồng vẫn mở bằng `PostDetail` của main nên không mất ảnh ở trang bài.
- DB dev đã cài schema socialv2 cũ sẽ lệch checksum; dựng stack mới.
- Flow Maestro đã sửa nhãn nhưng chưa chạy trên máy ở SHA này.
- Mục P1/P2 bên dưới giữ nguyên.

## 1. Trạng thái cần biết ngay

| Mục | Trạng thái khi bàn giao |
| --- | --- |
| Nhánh công việc | `feat/profile-story-routes` |
| Commit sản phẩm cuối | `343185a2` |
| `main` đã đối chiếu | `95388021` |
| PR | Mở dạng draft để review phần đã triển khai; chưa merge |
| Tích hợp với `main` hiện tại | **Đã gộp** tới `main=b2dc599d` trên nhánh `integ/profile-story-routes-main`; xem mục 0 |
| Giao diện hồ sơ / huy hiệu / tường | Đã triển khai; đã mở ảnh và thao tác Android bằng dữ liệu tổng hợp |
| Cổng phát hành | Chưa hoàn tất; xem mục 6 và 7 |

**Việc tiếp theo trước tiên:** tích hợp với Cộng đồng và sổ kỷ niệm trên `main`
mới, giữ một writer cho mỗi tài nguyên và quyền xem/kiểm duyệt của bài. Không
chọn toàn bộ `ours` hoặc `theirs` cho các file xung đột. Gate của nhánh riêng
không chứng minh kết quả gộp hoạt động.

Đọc [AGENTS.md](../../AGENTS.md), [CLAUDE.md](../../CLAUDE.md),
[kiến trúc hồ sơ](../architecture/03-profile-story-social.md) và các quyết định
ADR-0031/0032. Khi tích hợp, đọc thêm ADR-0039 và ADR-0040 có trên `main`.
Backend nghiệp vụ mới dùng Go/SQL; Python chỉ inference. Giữ test legacy làm
oracle; `phase0/` và `docs/protocol/v1/` vẫn đóng băng.

## 2. Người dùng đã yêu cầu gì

1. Hồ sơ có câu chuyện riêng, UI đẹp với skill **Impeccable**.
2. Huy hiệu được thiết kế bằng trình sinh ảnh; tách agent thiết kế huy hiệu và
   agent xây routing achievement. Hai phần đó đã được thực hiện.
3. Dấu mốc đi chung, số điểm đến theo `destination_id`, ảnh và bài kể mở kết,
   kết giao nhau và quyền lợi của Nếp. Thứ tự chọn ảnh hưởng đường tiếp theo.
4. Nếp gợi ý cá nhân hóa sau consent; Go quyết định thành tích và quyền lợi.
5. Tường nhiều người dùng: đăng ảnh/bài, like, bình luận cha/con, like bình luận,
   nhấn ảnh để xem và bình luận, chia sẻ lại, cập nhật khi người khác tương tác.
6. Mở native app, dùng thực tế và xem UI bằng mắt; build/test end to end.
7. Chốt phần đã làm bằng PR; ghi chi tiết phần thiếu để agent khác tiếp tục.

## 3. Đã làm gì và phần nào đã kiểm chứng

| Lát cắt | Đã triển khai | Bằng chứng hiện có | Giới hạn |
| --- | --- | --- | --- |
| Thiết kế huy hiệu | 13 ảnh tổng hợp, nguồn/prompt, bản dùng trong app, trạng thái mở/khóa, bảng sáng/tối | Đã mở contact sheet và ảnh UI Android/browser | Chưa có nghiên cứu người dùng về khả năng nhận biết ở mọi cỡ |
| Hồ sơ kể chuyện | Hero hành trình, kết đang theo, dấu mốc, lượt MP4; tối đa ba huy hiệu chủ hồ sơ chọn | Android mở hồ sơ mình/bạn bè; badge tải thật; ảnh sáng/tối đã xem | Không tuyên bố mọi viewport/trạng thái đã được quét |
| Graph thành tích | Bốn mở đầu, ba hướng, sáu kết theo hướng, hai kết giao nhau, một kết cuối; lịch sử chọn có ý nghĩa | Go + PostgreSQL kiểm điều kiện, lịch sử và trao thưởng | Danh mục hiện tại có 13 huy hiệu; chưa có campaign mới do AI sinh tự do |
| Điểm đến | Tính theo `destination_id` của chuyến đã có dữ liệu | Ca PostgreSQL về hai điểm đến và lịch sử chuyến | Không suy ra thành phố từ tên tự nhập hoặc chuỗi địa chỉ |
| Gợi ý AI | Preview số đếm, đồng ý từng lần, adapter Python inference; Go lọc ứng viên và fallback | Contract, test adapter, native đường fallback/consent | Chưa chạy model thật để đánh giá mức đa dạng hay chất lượng câu kể |
| Quyền lợi MP4 | Sổ cấp lượt, reserve nguyên tử, idempotency, hoàn lượt khi hỏng, thư viện riêng | PostgreSQL + MP4 H.264 tổng hợp giải mã bằng ffmpeg; kiểm range và owner | Renderer thật và phát MP4 end to end trên native chưa được xác nhận |
| Ảnh native | Multipart bằng File của Expo, giữ ảnh đã tải và khóa retry khi lưu bài lỗi | Android chọn ảnh, upload, đăng bài, mở ảnh | Cần kiểm lại sau gộp luồng media mới trên `main` |
| Bình luận quanh ảnh | Cha/con một tầng, like bình luận, khay bình luận trong ảnh toàn màn hình | Native thao tác cha/con và nhập trong khay ảnh; test PostgreSQL | Parent của Cộng đồng mới nằm ở metadata riêng, cần thống nhất khi tích hợp |
| Chia sẻ | Đăng lại lên tường, audience chọn được, kiểm quyền bài gốc | Native đăng lại; test ACL/idempotency | Chia sẻ vào chat v2 chưa nối |
| Realtime | Sự kiện bền vững + long poll, LISTEN đánh thức; đọc lại khi foreground; loại phản hồi phân trang cũ | Hai Android: bài mới khoảng 6 giây, like/comment đổi trên tường; tám reader PostgreSQL cùng nhận đúng một sự kiện | Chưa chứng minh tải production, nhiều instance hoặc mọi kiểu reconnect |
| Story | Upload native, retry giữ ảnh, xem trên hai Android | Lượt sau tạo và xem được đúng ảnh; test hết hạn/quyền | Lượt đầu sau khôi phục stack từng chỉ upload ảnh, chưa tạo story; chưa tìm được nguyên nhân |
| Bản đồ web | Kiểm WebGL2 trước MapLibre, giữ trang ngày khi thiếu WebGL2 | Browser fallback, WebGL2 phần mềm, hai mutant UI của commit trước | Không suy rộng thành native map hay toàn bộ hành trình đã được quét |

### Địa chỉ mã và tài liệu

| Phần | Điểm vào |
| --- | --- |
| UI hồ sơ chính | `apps/mobile/src/rudi/screens/Profile.tsx`, `screens/profile/HanhTrinhTeaser.tsx` |
| Tường/hồ sơ từng người | `apps/mobile/src/rudi/screens/nguoi/HoSoNguoiScreen.tsx` |
| Bài/ảnh/bình luận | `apps/mobile/src/rudi/screens/tuong/BaiChiTietScreen.tsx` |
| UI achievement | `apps/mobile/src/rudi/screens/ky-niem/AchievementsLive.tsx` |
| Contract journey và câu kể | `apps/mobile/src/rudi/ky-niem/achievement-routes.ts`, `journey-view.ts` |
| UI video / client | `apps/mobile/src/rudi/nep/NepPhim.tsx`, `profile-video.ts`, `NepBang.tsx` |
| Badge art | `apps/mobile/src/rudi/ui/BadgeArt.tsx`, `apps/mobile/assets/rudi/badges/` |
| Go graph / điều kiện | `services/core/internal/domain/achievement/routes.go`; `services/core/internal/achievementv1/handler.go`, `store.go`, `schema.sql` |
| Go tường / tương tác / sự kiện | `services/core/internal/socialv2/wall.go`, `interactions.go`, `notify.go`, `schema.sql` |
| Go video / credits / proxy | `services/core/internal/profilemedia/handler.go`, `jobs.go`, `proxy.go`, `schema.sql` |
| Adapter inference | `services/api/app/api/achievement_gemini.py`, `services/api/app/api/routes/brain.py` |
| Mount/migration | `services/core/cmd/core/main.go`, `docker-compose.yml`, `scripts/chat_e2e_stack.sh` |
| Contract/ownership | `services/core/ownership/routes.json`, `.api-contract-unresolved.json` |
| Thiết kế và nguồn ảnh | [huy hiệu](../design/huy-hieu-ho-so.md), `docs/assets/profile-badges/` |
| Native QA trước | [hai Android](../testing/profile-native-qa-2026-09-24.md) |
| Native QA chốt | [ảnh sáng/tối/reconnect](../testing/profile-final-pr-2026-09-28.md) |

Tên file ở bảng cần được đối chiếu lại sau tích hợp; không di chuyển file chỉ
để làm đẹp handoff. Comment/docstring bằng tiếng Anh, docs/commit tiếng Việt.

Các lượt native bổ sung dùng APK debug dev client đã có, nạp JavaScript của
nhánh qua Metro. Chưa có bằng chứng rebuild APK/native iOS từ checkout sạch
tại commit sản phẩm cuối. Cổng export ba platform chỉ chứng minh bundle.

## 4. Đang làm gì khi chốt PR

Đã chốt bằng chứng gate trong checkout sạch tại `343185a2`, kiểm tra mutant
và đưa ảnh tổng hợp đã xem vào Git theo digest. Bước bàn giao là commit tài
liệu và mở draft PR. Việc tích hợp `main` chờ agent tiếp tục theo mục 5–6;
không có merge đang thực hiện dở trong cây này.

Hai mutant MP4 đã chạy ở checkout riêng với cùng test harness:

| Biến thể | Đánh giá trước khi chạy | Kết quả |
| --- | --- | --- |
| Identity trước/sau | Giữ validator đầy đủ | Xanh, exit 0 ở cả hai lượt |
| Chỉ kiểm `ftyp` như validator cũ | Không tương đương: chấp nhận file không có video track/sample | Đỏ tại `accepted an ftyp-only file with no video samples` |
| Cắt box khai báo quá dài về số byte còn lại | Không tương đương: chấp nhận sample bị cắt một byte | Đỏ tại `accepted a truncated video sample` |

Harness là `TestMP4ValidationRequiresPlayableContainerStructure`; file test
không đổi giữa các lượt. MP4 validator kiểm cấu trúc container H.264/H.265 không
phân mảnh; không kiểm giải mã codec hoặc đối chiếu mọi offset của sample.

Kết quả tại checkout sạch **`343185a2`**:

| Tầng | Đạt | Skip | Ghi chú |
| --- | --- | --- | --- |
| 22 chặng được chọn trong `make gate STRICT=1 ONLY=...` | 22 chặng | 0 chặng | Exit 0; đây là bộ chọn, chưa phải toàn bộ gate |
| Mobile | 955 | 0 | TypeScript, web a11y bắt buộc và bundle web/iOS/Android đạt |
| API/product/meta | 3690 | 807 | Các ca skip không được coi là bằng chứng đạt |
| PostgreSQL sản phẩm | 726 | 0 | PostgreSQL dùng một lần |
| PostgreSQL QA | 88 | 0 | Process QA riêng |
| Go PostgreSQL | 2056 | 0 | Có sentinel |
| Client/server E2E | 11 | 4 | Bốn ca cần media service thật chưa chạy |
| Chat E2E | 41 | 0 | HTTP/WebSocket, có sentinel |
| Identity / hai mutant MP4 | 2 lượt xanh / 2 lượt đỏ | 0 | Đỏ đúng dự đoán, cùng harness |

Docker pinned, migration và các cổng static đều exit 0. Handoff là commit chỉ
thay tài liệu sau commit sản phẩm; PR ghi riêng kết quả guard ở SHA bàn giao.
Toàn bộ strict gate và gate trên kết quả gộp vẫn thuộc phần còn thiếu.

Kết quả gate cuối được ghi trong body PR với SHA; kiểm tra body trước khi dùng
lại các con số. Log máy này ở `~/.cache/rudi-profile-native/`, gồm
`gate-pr-343185a2.log`, `mp4-mutants-pr.json` và bốn log identity/mutant.
Không coi log cũ là kết quả của commit sau tích hợp.

## 5. Chuẩn bị làm gì: giải xung đột với main

### 5.1 Các file báo xung đột tại `343185a2` + `95388021`

| Nhóm | File | Hướng tiếp tục |
| --- | --- | --- |
| Profile / diary | `HoSoNguoiScreen.tsx`, `AchievementsLive.tsx` | Giữ huy hiệu/graph của nhánh và `DiaryWall`/sổ kỷ niệm của main; tránh hai danh sách bài dùng hai định nghĩa quyền |
| Bài và story | `BaiChiTietScreen.tsx`, `DangStoryScreen.tsx` | Giữ ảnh native, draft/retry, comment viewer; nối đúng bài Cộng đồng, kiểm duyệt và media access hiện tại |
| Nếp | `NepBang.tsx` | Giữ video/credits của nhánh và ngữ cảnh/consent mới của main; không làm mất đường gọi Nếp mới |
| Client contract | `apps/mobile/src/api.ts`, `.api-contract-unresolved.json` | Giữ toàn bộ route hiện có trên main, thêm contract profile, đọc lỗi và body theo writer thực |
| Dependencies / test | `apps/mobile/package.json`, `package-lock.json`, `tsconfig.test.json`, `tests/rudi-hanh-trinh-web.test.mjs` | Gộp dependency trực tiếp, sinh lại lockfile bằng npm; giữ tests của cả hai nhánh |
| Bootstrap / SQL | `docker-compose.yml`, `scripts/chat_e2e_stack.sh`, `services/core/cmd/core/main.go` | Giữ migrate-chat mặc định, migrate-diaries/community của main và migrate-profile của nhánh; migration là lệnh tường minh |
| Ownership / gates | `services/core/ownership/manifest.go`, `routes.json`, `scripts/render_route_manifest.py`, `scripts/check_go_owned_python_touch.py`, `scripts/gate.sh`, `services/core/internal/httpapi/router/router_test.go` | Giữ các cổng mới của main, khai báo chính xác Go-native routes; không mở ngoại lệ chỉ để gate xanh |
| AI contract test | `services/api/tests/api/test_brain_seam.py` | Giữ kiểm thử inference mới của main và bộ lọc ứng viên achievement |
| Artifact review | `.repo-guard-allowlist.json` | Gộp theo path, tính hash từ bytes cuối; ảnh/fixture đã đổi phải review lại, không giữ hai digest cho cùng path |

Các đường dẫn UI rút gọn ở bảng nằm trong
`apps/mobile/src/rudi/screens/` hoặc `apps/mobile/src/rudi/nep/`; bảng mục 3
ghi điểm vào đầy đủ. Danh sách trên có 22 file, không phải danh sách toàn bộ
vấn đề tích hợp. Những file Git tự gộp sạch vẫn có thể sai hành vi.

### 5.2 Ranh giới cần giải quyết bằng ca PostgreSQL thật

1. **Một bài, một writer.** ADR-0040 dùng chung `posts`, like và comment giữa
   Cộng đồng/tường. Nhánh này có các mutation ở `socialv2/interactions.go`;
   main có writer `community`. Chọn một đường ghi cho mỗi loại bài, hoặc cho
   social API gọi writer sở hữu bài. Không cho bài đang kiểm duyệt nhận ghi
   trực tiếp qua social v2/legacy rồi bỏ qua revision hoặc audit.
2. **ACL và nội dung đang chờ duyệt.** Kiểm tra bài private/pending/rejected,
   sửa public thành revision đang xét, bài bị gỡ và media mất quyền. Cả tường,
   ảnh viewer, trích đoạn repost, comment, like và changes phải kiểm cùng quyền.
   `community.GuardLegacy` chỉ là một điểm cần đối chiếu; không giả định nó
   đã bao phủ các route `/social/v2/...` mới.
   Kiểm cả hai cấu hình bật/tắt Cộng đồng; feature flag không được tạo đường
   vòng cho bài đã thuộc writer kiểm duyệt.
3. **Bình luận cha/con.** Nhánh thêm `post_comments.parent_id`; main dùng
   `community_comment_meta.parent_id`. Tránh hai nguồn parent bất đồng và
   comment vòng qua hàng đợi kiểm duyệt. Giữ contract cha/con và like phù hợp
   với comment đã công bố; xác nhận bằng trường hợp một bài nhìn từ hai tab.
4. **Realtime và vị trí cuộn.** Main có stream Cộng đồng và outbox; nhánh có
   `social_wall_changes` long poll. Chứng minh không mất/nhân đôi hoạt động và
   quyền bị thu hồi áp dụng trên kết nối đang mở. Kiểm tra refresh khi đang
   xem trang cũ: nhánh hiện đọc lại trang đầu ở background, cần xác nhận UX
   khi đã tải nhiều trang và tránh tự đẩy vị trí cuộn theo ADR-0040.
5. **Migration và khởi động.** Dựng database mới và nâng database đã có dữ liệu
   tổng hợp. Giữ một chủ sở hữu schema, chạy từng migration đúng thứ tự;
   không chạy migration trong request hoặc bỏ test oracle để che khác biệt.
6. **AI boundary.** Achievement gửi số đếm và ID ứng viên sau consent; không
   được tự đọc diary/chat/private history khi gắn với Nếp/Cộng đồng mới.

## 6. Thiếu gì và thứ tự agent sau nên làm

### P0 · Đủ điều kiện gộp

- [ ] Tích hợp `main` hiện tại; giải 22 xung đột và kiểm lại phần tự merge.
- [ ] Thống nhất writer/ACL/parent/revision giữa profile social và Cộng đồng.
- [ ] Thêm ca PostgreSQL cho bài pending/rejected, media revocation, repost
  mất quyền và comment đi đúng hàng đợi; không chỉ mở rộng fake repository.
- [ ] Chạy cổng trên **kết quả gộp** và checkout sạch tại SHA sẽ đưa vào main.
- [ ] Canary đỏ như dự đoán, identity xanh, ít nhất hai mutant không tương
  đương ở cùng harness; ghi số vào commit và mở ảnh UI cuối để xem trực tiếp.
- [ ] Cập nhật handoff/body PR rồi bỏ draft khi các mục P0 thực sự đạt.

### P1 · Hoàn tất lời hứa sản phẩm và native

- [ ] Tái hiện lượt story đầu sau khôi phục stack: tài khoản mới, ảnh mới,
  lần submit đầu, mất mạng giữa upload và tạo story, retry cùng idempotency.
  Ghi HTTP/status và trạng thái database bằng dữ liệu tổng hợp; chưa có root
  cause hoặc fix được chứng minh cho hiện tượng cũ.
- [ ] Chạy inference thật có consent bằng dữ liệu tổng hợp; so sánh người có
  cùng số đếm nhưng lịch sử chọn khác nhau, kiểm diversity, câu dẫn có căn cứ,
  lỗi/timeout và fallback. Go vẫn quyết định eligibility và credits.
- [ ] Nếu tiếp tục yêu cầu campaign đa dạng do AI cá nhân hóa: mở rộng nhiệm
  vụ/ứng viên được Go kiểm chứng, luật và version của campaign trước khi nối
  model; đo rằng những lựa chọn khác nhau tạo hướng tiếp theo khác nhau. Bản
  hiện tại chỉ xếp thứ tự ứng viên trong danh mục và tạo câu dẫn.
- [ ] Nối renderer MP4 thật, các mẫu sáng tạo còn ghi “sắp dùng được”; kiểm
  create → progress → file → phát trong `NepPhim` trên Android/iOS, thư viện
  sau relaunch, range/seek, lỗi render, hoàn lượt và hai lần click đồng thời.
- [ ] Chia sẻ vào chat v2 chỉ qua E2EE; không fallback plaintext/legacy.
- [ ] Native Android sau tích hợp: hai tài khoản, camera/gallery permissions,
  ảnh lớn, bàn phím, back, draft, reconnect, chọn/trưng bày huy hiệu, post/reply/
  like/repost/thu hồi quyền, xem trang cũ và dark mode/large text/Reduce Motion.
- [ ] iOS native riêng; bundle iOS thành công chưa chứng minh UI hoặc crypto.
- [ ] Maestro/native automation và kiểm chứng crypto độc lập theo cổng repo.

### P2 · Đo quy mô và chất lượng

- [ ] Đo nhiều instance/thiết bị, sự kiện sau commit, reconnect/cursor lâu,
  thay đổi quyền trong phiên, mutation đồng thời và kế hoạch truy vấn.
- [ ] Đo mốc tải/soak của ADR-0040; tám reader hiện tại chỉ là ca đồng thời nhỏ.
- [ ] Review UI bằng Impeccable trên trạng thái thật: profile trống, nhiều badge,
  nội dung dài, ảnh dọc/ngang, comment nhiều trang, lỗi API, bị chặn và mất quyền.

## 7. Lệnh và cách tiếp tục

Tạo checkout/worktree riêng cho tích hợp; giữ branch review hiện tại để so sánh.
Chỉ dùng PostgreSQL dùng một lần và dữ liệu tổng hợp. Không mang `.env`, ảnh
thật, danh tính thật hoặc transcript vào repository/worktree.

```bash
git fetch origin main
git switch feat/profile-story-routes
git merge --no-commit origin/main
# Resolve and review the writers, ACL and migrations before committing.
```

Các lệnh xác nhận theo repo:

```bash
make gate
make gate-merge REF=feat/profile-story-routes
python3 scripts/repo_guard.py staged
python3 scripts/repo_guard.py tree HEAD
python3 scripts/check_route_ownership.py
scripts/go_postgres_tier.sh
scripts/postgres_tier.sh
make parity
scripts/chat_e2e_go.sh
```

Lượt chốt nhánh này chọn 22 chặng, không thay cho toàn bộ strict gate:

```bash
make gate STRICT=1 ONLY="guard guard-range ruff contract client-routes server-routes screens cors ownership python-touch go-vet go-test api migration pinned-import shared mobile docker postgres go-postgres e2e chat-e2e"
```

Test MP4 và reader để tìm ca cụ thể:

```bash
cd services/core
go test -count=1 -run '^TestMP4ValidationRequiresPlayableContainerStructure$' ./internal/profilemedia
cd ../..
scripts/go_postgres_tier.sh -- -run 'TestPostgresTierReachesDatabase|TestPostgresManyFriendsReceiveOnePostAndReconnectFromCursor|TestProfileVideoUsesOneCreditAndKeepsFilePrivate' ./...
```

Máy QA dùng Python/Git ở ngoài repo để tránh Git hệ thống cũ trong meta-tests:

```bash
PATH="$HOME/.cache/rudi-profile-venv/bin:$HOME/.local/share/git-moi/bin:$PATH" make gate
```

Không sửa package đã cài hoặc mock dependency để làm gate xanh. `scripts/ruff_pinned.sh`
là ruff chuẩn; Docker/pinned-import kiểm runtime ghim. Cần kiểm version thật
khi đổi máy; log máy cũ có thể không còn tồn tại.

Dựng stack tổng hợp bằng `scripts/chat_e2e_stack.sh up`; script trả đường dẫn
state ngoài repo. Sau khi xong dùng `scripts/chat_e2e_stack.sh down <state-dir>`.
`connection.json` chứa secret QA: đọc trường cần dùng trong process, không
paste cả file vào PR/log. Không tự dùng script seed chat legacy làm bằng chứng
E2EE; luồng chat native và kiểm chứng crypto có cổng riêng.

Điểm cấu hình của nhánh khi nối dịch vụ thật: Go gọi brain qua
`MOBILE_BRAIN_URL` (hoặc `MOBILE_PYTHON_UPSTREAM`) và `MOBILE_INTERNAL_TOKEN`;
adapter inference Python đọc `GEMINI_API_KEY`. Profile media proxy đọc
`NEP_PROXY_URL`, `NEP_PROXY_TOKEN` và khóa danh tính `MOBILE_PERSON_ID_KEY`.
Đối chiếu lại các tên này với cấu hình trên main sau gộp. Chỉ ghi tên biến
trong docs; secret thực ở môi trường ngoài repo, không đưa vào log/PR.

Dùng emulator, Metro và stack riêng cho lượt tiếp tục. Stack/Metro/emulator
riêng của lượt QA vừa rồi đã được dọn; không dừng các tiến trình của lane khác
trên máy dùng chung.

Stack dựng mới có khóa danh tính mới: đăng nhập lại và dùng ảnh/job của đúng
actor trong stack đó. Không dùng token, actor ID hoặc ảnh/job từ stack cũ để
kết luận về lỗi phân quyền; tình huống này đã gây nhiễu một lượt QA trước.

Đọc skill Impeccable trước khi sửa UI. Skill đã được dùng trong phiên này ở
`~/.claude/plugins/cache/impeccable/impeccable/4.1.2/skills/impeccable/SKILL.md`;
nếu máy mới có catalog/path khác thì tìm đúng skill, giữ thế giới sổ giấy,
dấu mực, bìa indigo và điểm nhấn coral trong hệ thiết kế đang có.

## 8. Bàn giao kết thúc ở đâu

Nhánh được đẩy và mở draft PR với các mục trên. Không merge main, không claim
“đã test mọi ngách” hoặc “state of the art hoàn tất”. Agent sau có thể review
phần hiện có rồi làm P0 → P1 → P2 bằng các vertical slice chạy được. Mỗi slice
phải mang bằng chứng thực ở layer nó thay đổi và cập nhật trạng thái trong PR.
