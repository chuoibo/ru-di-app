# Bàn giao Codex → Claude · 13 ca và phần cần tiếp tục

Cập nhật 04/10/2026. Đây là điểm bắt đầu để tiếp tục session
`c97c4813-340b-485b-a30b-32b6828ad498`, không phải báo cáo audit mới.
Audit gốc: [PR #663](https://github.com/chuoibo/ru-di-app/pull/663).

**PR bàn giao chứa 13 ID do Codex triển khai và tự kiểm; chưa QA_ACCEPTED.**
44 ID B8/B9 được dành lại cho Claude theo yêu cầu không làm trùng/ghi đè việc
đang dở. Ba ID liên quan quyết định nghiệp vụ được tách riêng bên dưới.
Không dùng tài liệu này để kết luận toàn bộ 167 ID đã giải quyết.

## Đọc theo thứ tự này

1. Tài liệu này: phạm vi, bằng chứng, việc tiếp theo và lý do giữ lại.
2. [Ledger 167 ID và checkpoint](ui-ux-campaign.md): ID, finding, tiêu chí QA và trạng thái kế thừa.
3. [B9a · Trang cuối](ui-ux-b9a.md) và [B10 · CP06–09](b10-handoff.md): brief, implementation, retest và lịch sử các lượt đỏ/xanh.
4. [Bàn giao Claude](../../claude/2026-10-01/ui-ux-upgrade/qa-handoff.md), [feature-plan](../../claude/2026-10-01/ui-ux-upgrade/feature-plan.md), [direction](../../claude/2026-10-01/ui-ux-upgrade/direction.md), rồi diff chưa commit của session gốc.

Các đường dẫn Claude ở trên đi từ thư mục này tới `docs/claude/`; report gốc
không bị sửa để thay finding hoặc ghi nhận Codex như QA độc lập.

## Ranh giới nhánh và tích hợp

Nhánh xuất bản: `codex/ui-ux-qa-handoff`, worktree riêng
`/home/lakiet/project/mobile/wt-ui-ux-qa-handoff`.
Nó gồm B10 tới `8ec9e18d`, cherry-pick **chỉ hai commit B9a hoàn tất**:
`88282a87` → `9bb87c58` và `b4f4b189` → `5ea557d6`, rồi gộp main
`2b6c9360` thành commit sản phẩm **`44feee05`**. Chỉ hợp nhất các entry
allowlist đã review. Lượt gộp đầu làm mất digest lịch sử; đã sửa riêng để giữ
đủ entry cùng path/rule nhưng khác hash, không repin dữ liệu chưa review. Bản đồ
hướng dẫn được sinh lại và không đổi byte: `99be676b0199`.

**Chưa tích hợp được main mới `4f74b011`**: commit này bỏ bản demo, chuyển
người chưa đăng nhập tới cửa đăng nhập. Thử merge bằng `git merge-tree` không
chạm working tree; báo bảy xung đột:

| File | Việc cần đối chiếu khi tích hợp production |
|---|---|
| `apps/mobile/app/_layout.tsx` | Giữ cửa auth production mới cùng lớp cover/khóa scene khi restore; không dựng lại route demo. |
| `apps/mobile/assets/rudi/README.md` | Giữ tài liệu asset production và nguồn wordmark splash. |
| `apps/mobile/src/rudi/nep/NepBang.tsx` | Mang form đồng ý, review draft và lỗi cạnh action sang Nếp production; giữ việc bỏ entry demo. |
| `apps/mobile/src/rudi/nep/huong-dan-ban.ts` | Sinh lại từ source sau khi giải quyết route; không chọn một hash cũ bằng tay. |
| `apps/mobile/src/rudi/screens/Discovery.tsx` | Main đã xóa màn demo. Giữ việc xóa; gutter live ở `ExploreLive.tsx` là phần cần bảo toàn. |
| `apps/mobile/tests/rudi-hanh-trinh-web.test.mjs` | Main đã xóa test demo. Giữ việc xóa; thiết kế lại regression trên flow production nếu cần. |
| `scripts/mobile_native.sh` | Giữ harness production mới; mang khả năng chọn Metro host khi còn phù hợp, chạy canary/fingerprint lại. |

PR để **draft**, chưa landing main. Ngoài bảy xung đột, mobile bản gộp còn
đỏ ở `trang-rong-co-hinh.test.mjs`: `EndingScreen.tsx`, trạng thái non-retry
«Chưa mở được trang cuối» thiếu `illustration`. Đây là điểm không nhất quán
với gate thiết kế mới từ main; Claude cần chọn cảnh phù hợp, xem render native
state quyền/lỗi này, không thêm tên vào allowlist test chỉ để cho qua. Bằng chứng dưới đây thuộc các SHA được
ghi, không phải chứng nhận source production `4f74b011`, merge-result tương
lai hoặc current main. Không giải quyết xung đột bằng cách phục hồi demo đã xóa.

## Codex đã làm gì

Hướng thiết kế giữ giấy/mực, bìa indigo, coral, chữ hiệu và Bricolage của app.
Phần mở đầu nối bìa native → chờ restore → Welcome; trang cuối dùng ngày và
nội dung thật; Nếp trình bày quyết định chia sẻ theo từng bước. Mỗi thay đổi
có mục tiêu trải nghiệm cụ thể, không thêm trang trí/animation chỉ để khác đi.
Các checkpoint dùng Impeccable pipeline và review kết quả render/motion;
finish `ship` chỉ trong phạm vi đã chấm, không thay QA hoặc release gate.

| Hoàn tất triển khai/tự kiểm | ID | Thay đổi và giá trị trải nghiệm | Nhóm |
|---|---|---|---|
| [x] B9a | UI-150 | Lỗi lưu cạnh nút ở footer, giữ bản soạn/quyền xem; retry đúng dữ liệu, khóa request khi chạm nhanh. | BUG_FIX, UX_IMPROVEMENT |
| [x] B9a | UI-151 | GET `can_end` xét ngày Việt Nam giống POST; không đề nghị thao tác máy chủ sẽ từ chối. | BUG_FIX |
| [x] B9a | UI-152 | Khi chưa được khép, có lời chờ và đường về cuộc hẹn; bỏ bộ chọn giả. | UX_IMPROVEMENT |
| [x] B9a | UI-154 | Heading trang tự xếp là ngày đọc được; giữ ISO nguồn và tên người dùng tự đặt. | UX_IMPROVEMENT |
| [x] CP06 | UI-002 | Link lạ chờ restore rồi về đúng cửa theo phiên; heading nhận focus, CTA và Back có lối về. | BUG_FIX, UX_IMPROVEMENT, VISUAL_UPGRADE |
| [x] CP06/09 | UI-009 | Native splash dùng chữ hiệu trên bìa; React cover nối tiếp tới màn sẵn sàng, scene dưới không nhận pointer/focus. | BUG_FIX, VISUAL_UPGRADE, MOTION_UPGRADE |
| [x] CP07 | UI-016 | Copy, mốc và chấm Welcome theo trang đang cuộn; resize giữ trang. | BUG_FIX |
| [x] CP07 | UI-017 | Vuốt nhanh theo một chặng; Welcome → Login một handoff, chạm lặp không push nhiều lần, reduced motion giữ nội dung. | BUG_FIX, MOTION_UPGRADE |
| [x] CP07 | UI-020 | Pager 48dp có tên/trạng thái, phím điều hướng và focus; trang ngoài khung ẩn khỏi accessibility. | UX_IMPROVEMENT |
| [x] CP08 | UI-008 | Không còn Tab stop Nếp không tên; vùng tĩnh đang diễn không giả làm nút. | BUG_FIX, UX_IMPROVEMENT |
| [x] CP08 | UI-011 | Vẽ dùng sheet xem/sửa → đồng ý → gửi; lỗi giữ draft, hủy không gửi, xác nhận gửi một lần. | BUG_FIX, UX_IMPROVEMENT, VISUAL_UPGRADE |
| [x] CP08 | UI-012 | Chip gợi ý đạt vùng bấm 48dp. | UX_IMPROVEMENT |
| [x] CP08 | UI-014 | Gutter giấy giữa chip cuộn và Nếp trên màn nhỏ, ở demo lịch sử và live. | BUG_FIX, VISUAL_UPGRADE |

Cải thiện ngoài finding: B9a thay ba chặng trang trí bằng ngày thật, quyền xem
sau nội dung, cột header/form thống nhất, không remount khi chuyển sửa/xem;
B10 nối các lớp startup và đặt lời đồng ý/footer/IME Nếp trong cùng trải nghiệm.
Không thêm feature, đổi lời hứa chia tiền, quyền AI hoặc contract backend.

Shared component: thêm `OpeningApp`; `Sheet` mở rộng cơ chế footer/keyboard
theo opt-in, không mặc định migrate mọi caller; `NepDien` có semantics phù hợp.
Không thêm palette/token mới. UI-009 thêm module Expo SDK57
`expo-splash-screen` cùng phiên bản SDK và PNG từ wordmark hiện có; nguồn/license
trong `apps/mobile/assets/rudi/README.md`. Backend B9a sửa Go/domain/handler và
test PostgreSQL, không thêm runtime Python, schema hay writer mới.

## Đã test gì, ở đâu và giới hạn nào

| Source | Kiểm chứng thực tế | Kết luận cho phép |
|---|---|---|
| B9a `88282a87` | Cây sạch, npm ci: 13 chặng chọn lọc đạt/0 hỏng/0 skip; mobile 1459/0/0 + typecheck + export ba nền tảng; PostgreSQL thật 13 ca, sentinel có mặt, không skip; unit/vet Go đạt. | B9a có bằng chứng source và DB thật; export iOS không chứng minh chạy iOS. |
| B9a native | AVD Android, phone 411dp sáng, override 320dp/font1.3/tối/reduced và tablet768dp; lỗi503, bàn phím, retry, double tap 1 PUT, DB revision/private/document đúng. Mở nhìn ảnh thật. | Các state và cấu hình đã chạy; APK dev client tái dùng chưa có fingerprint đầy đủ, không coi là release proof. |
| B9a negative controls | Identity trước/sau xanh; canary luôn-started đỏ; hai mutant UTC calendar/raw heading đỏ đúng test, có phản ví dụ không tương đương; cùng harness. | Harness bắt được sai lệch đã dự đoán, không bao phủ mọi lỗi domain. |
| B10 `6144ff2c` | Cây sạch, npm ci: mobile 1479/0/0, typecheck, export Android/iOS/web; native 12 flow default fixture, 1 stage đạt/0 hỏng/0 skip, 830s; fingerprint sai và canary đỏ đúng bước. | Scope B10 đã tự kiểm trên harness này; chưa phải tất cả flow QA hoặc flow production mới. |
| B10 native startup/motion | Dựng release/debug Android x86_64, đối chiếu source APK; guest cold link lạ/CTA/Back/lời mời synthetic/icon, phone, 320dp/font1.3/tối/reduced, tablet; video Welcome/Login/Back được giải mã và mở nhìn. Release ký debug cục bộ, không Metro. | Có render và chuyển động thực, không chứng nhận production signing, FPS hoặc thời gian native ≤300ms. |
| B10 negative controls `6144ff2c` | Identity trước/sau 2/0/0; canary bỏ cover đỏ; hai mutant bỏ inert/CTA sai cửa có phiên đỏ; cùng hash harness, mỗi variant export mới, khôi phục sạch. | Có kiểm tra cả phản ví dụ; phép paint web từ resume request không đo cold startup Android. |
| Bản gộp `44feee05` | Kết quả kiểm tra cây sạch khi xuất bản được cập nhật ở mục cuối. | Chỉ áp dụng bản gộp main `2b6c9360`; chưa áp dụng main mới. |

Lịch sử đỏ giữ nguyên: full strict tại clean `dc1bd772` là **26 đạt / 7 hỏng /
0 skip**, không có full strict xanh. Các chặng đỏ: ruff, demo-watch, hero-walk,
mobile, mobile-native, Go PostgreSQL và crypto. Các lượt mobile/native/crypto
riêng về sau xanh như [B10](b10-handoff.md), không ghi đè verdict full gate cũ.
Go PostgreSQL lúc đó đỏ vì registry xóa tài khoản thiếu
`community_notifications.actor_id`; source original đang dở thuộc phần B8
nên không patch để ép cổng xanh. Parity cùng SHA cũ: dev351/10758,
limiter9/209, prod23/605 scenario/step đều 0 diff; live PostgreSQL702+51 pass.

Concern test Hành trình: lượt clean B10 đầu 1478/1/0, chạy riêng rồi full repeat
1479/0/0. Chưa bắt được trace thất bại để kết luận nguyên nhân; instrumentation
được gỡ, không sửa assertion/source Hành trình để cho qua. Detector Welcome
và startup vẫn có finding; triage và contrast incomplete ghi nguyên trong B10.
Không tuyên bố detector sạch hoặc accessibility compliance toàn app.

Chưa kiểm: native iOS, máy thật, TalkBack toàn flow, predictive Back, hiệu năng
FPS/frame time/native300ms, cold signed-in release và production signing.
Narrow Login font1.3 có caption sát/che bởi navigation bar; chưa có baseline
để khẳng định regression, cần retest riêng. B9a còn revision conflict runtime,
ảnh lỗi, Back lúc lưu và danh sách24 trang. Không dùng screenshot thay proof motion.

## Evidence có thể đọc lại

- [B9a trước/sau](ui-ux-b9a.md#evidence): chín PNG native synthetic đã commit, hash ghim trong repo guard.
- [UI-009 trước/sau và clip](evidence/ui009/README.md): hai PNG và MP4 native đã commit; nguồn/state/cấu hình/giới hạn ghi cạnh evidence.
- Các log, ảnh/video CP06–08, mutants và packet build còn ở máy này, ngoài Git: `/home/lakiet/.local/share/rudi-b9a/campaign/`; B9a log ở thư mục cha. Các link tuyệt đối trong B10 chỉ dùng được trên máy này, không coi như artifact có sẵn cho người clone khác.

Không đưa session JSONL, `.env`, app-private backup, ảnh hóa đơn hoặc dữ liệu
người thật vào PR. Ảnh/clip đã commit chỉ từ dữ liệu tổng hợp được review.

## Vì sao 44 ca giữ cho Claude

Yêu cầu của người giao việc là không làm lại hoặc ghi đè phần Claude đã/đang làm.
Session đã tiếp tục tới 21:20 ngày02/10; worktree gốc có patch chưa commit,
tài liệu Claude chưa cập nhật B8/B9. Tại lần đọc04/10 worktree gốc ở `2b6c9360`,
có85 mục dirty; ledger vẫn103 READY_FOR_QA,3 VERIFIED_LOCALLY,58 PLANNED,
1 partial và2 BLOCKED. Codex đọc đối chiếu rồi tách nhánh, không stage/reset/
stash/chạy tiếp session hoặc sửa plan của Claude.

**RESERVED_CLAUDE là ranh giới tránh làm trùng, không chứng minh từng ca đã có
patch hoặc đã test.** Không có log todo đáng tin để nói cả44 ca đang implement.
Claude cần đọc diff và xác minh từng ID, sau đó đánh tick đúng bằng chứng.

CP01 Codex từng làm trước khi phát hiện overlap: patch nằm **chỉ ở nhánh local
`codex/ui-ux-b9a`, commit `a4d42fd3`**, IMPLEMENTED_UNVERIFIED. Có1460 test
xanh ở lượt cũ; sửa contain-pan/native focus đã có source nhưng render cuối
chưa kiểm. **Commit này không có trong PR bàn giao**. Chỉ dùng `git show`
để so sánh khi hữu ích; không cherry-pick cả nhánh B9a hoặc thay viewer Claude
đang làm. Hai commit B9a hoàn tất đã ở PR nên không cherry-pick lại.

## Ba ca nghiệp vụ cần quyết định, không phải 44 ca overlap

| ID | Đã có / còn thiếu | Lý do Codex không tự triển khai |
|---|---|---|
| UI-119 | Claude đã thêm lối «Thêm khoảnh khắc ở đây», album nói check-in/chỗ; yêu cầu tự biến «Tôi đã tới» thành dấu kỷ niệm còn BLOCKED. | Đổi ý nghĩa arrival/check-in, cần quyết định [ADR UI-119](../../claude/2026-10-01/ui-ux-upgrade/adr-de-xuat/UI-119-da-toi-la-check-in.md). Không làm lại phần UI đã có. |
| UI-131 | BLOCKED: máy chủ còn phác tờ cho cặp chưa «Một đôi». | Đổi eligibility/quyền của cặp, chờ [ADR UI-131](../../claude/2026-10-01/ui-ux-upgrade/adr-de-xuat/UI-131-to-giay-chi-cho-cap-doi.md). |
| UI-149 | BLOCKED: «Đã chia» đang gắn ngày, có thể nhập nhằng hai kèo cùng ngày. | Đổi mô hình liên kết dữ liệu tiền, cần quyết định [ADR UI-149](../../claude/2026-10-01/ui-ux-upgrade/adr-de-xuat/UI-149-da-chia-theo-keo.md) trước code. |

UI-073 trong44 ca cũng có ADR riêng về tên do người mời đặt; xử lý phần UI
độc lập, không âm thầm đổi luật tra số/quyền danh tính. UI-096 đã superseded
bởi UI-158, không mở một implementation thứ hai.

## Số liệu để không đếm nhầm

| Nhóm trong167 ID | Số | Diễn giải |
|---|---:|---|
| Claude triển khai/tự kiểm theo ledger | 106 | 103 READY_FOR_QA +3 VERIFIED_LOCALLY, chưa Codex xác minh lại tất cả. |
| Codex bàn giao trong PR này | 13 | 4 B9a +9 B10; READY_FOR_QA, các giới hạn kiểm chứng vẫn mở. |
| Claude UI partial / luật còn chờ | 1 | UI-119. |
| BLOCKED nghiệp vụ | 2 | UI-131, UI-149. |
| RESERVED_CLAUDE | 44 | 18 B8 +26 B9, trạng thái từng patch chưa xác minh. |
| Superseded | 1 | UI-096 → UI-158. |
| **Tổng** | **167** | 119 triển khai/tự kiểm không có nghĩa167 QA_ACCEPTED. |

## Cách Claude tiếp tục mà không làm lại phần đã giao

- [ ] Đọc session/plan local và diff gốc trước; không reset/stash/copy toàn working tree để checkout PR. Nếu cần, dùng worktree sạch mới để review nhánh remote.
- [ ] Tích hợp13 ca với hướng production `4f74b011`, giải quyết bảy file nêu trên, giữ việc bỏ demo; sinh lại hướng dẫn. Không kéo CP01 WIP vào cùng lượt.
- [ ] Tiếp tục B8 và CP01–05 theo44 ID ở bảng tiếp theo, so patch thực với finding/acceptance trước khi code. Không bắt đầu lại B1–B7 hay triển khai lại13 ca trong PR.
- [ ] Mỗi checkpoint dùng Impeccable pipeline, review cả screen/overlay/state; quan sát animation native khi chạy, không kết luận từ source hoặc ảnh tĩnh. Nếu UI yếu, cải thiện có lý do, bảo toàn rule/data/API.
- [ ] Ghi IMPLEMENTED_UNVERIFIED/VERIFIED_LOCALLY/READY_FOR_QA theo chứng cứ; giữ report gốc, cập nhật change-log/handoff riêng. QA_ACCEPTED chỉ khi có xác nhận QA độc lập.
- [ ] Chạy gate trong cây sạch đúng SHA tích hợp, canary/identity/hai mutant cùng harness; full strict và merge-result gate phải được xử lý trước landing main.
- [ ] QA retest13 ca với flow production: link có/không phiên; splash/Welcome/Login/Back normal+reduced; Nếp consent/hủy/sửa/retry/IME; Ending tương lai/quyền/lưu503/revision; heading ngày. Retest shared Sheet callers và gutter live.
- [ ] Chờ quyết định ADR cho phần business của119/131/149 (và073 khi chạm luật); tiếp tục các việc độc lập trong lúc chờ.

## Danh sách44 ID dành lại · đối chiếu từng patch trước khi tiếp tục

Các dòng dưới là finding/expected từ ledger, không phải tuyên bố Codex đã
chạy lại từng ca. B8 ưu tiên hoàn tất phần đang dở; B9 theo CP01–05.

### B8 · 18 ca

| Cần tiếp tục | ID | Finding → expected |
|---|---|---|
| [ ] | UI-071 | Lập nhóm xong về Khám phá, không thấy nhóm vừa lập · vào nhóm mới + lời mời |
| [ ] | UI-073 | Người vào bằng lời mời bỏ qua Sở thích; tên người mời đặt thành tên công khai · Sở thích có ô tên sửa được (luật tra số: ADR đề xuất) |
| [ ] | UI-074 | Quản trị tự bỏ quyền bằng một chạm · hỏi trước khi tự hạ quyền |
| [ ] | UI-075 | Mọi quản trị mang nhãn «Người lập nhóm» · chỉ người lập |
| [ ] | UI-076 | 19 nút cùng tên «Đặt làm quản trị»; hàng không mở hồ sơ · tên nút riêng; hàng mở hồ sơ |
| [ ] | UI-077 | «Đồng ý» lỗi thì cả danh sách bạn thành màn lỗi · lỗi theo hàng, danh sách giữ |
| [ ] | UI-078 | Sau khi chặn, tải lại mất dấu «Đã chặn» · «Đã chặn» + «Bỏ chặn» sau tải lại |
| [ ] | UI-080 | Lời mời vào nhóm không nói ai mời, không từ chối được · tên người mời + hai lựa chọn |
| [ ] | UI-081 | Tablet: danh sách trải hết, nút cách tên 487–679px · ≤160px |
| [ ] | UI-083 | Đọc sổ lỗi thì vẽ «Chưa có sổ» và mời lập lại · lỗi + «Thử lại» |
| [ ] | UI-084 | Sheet «Lập sổ» quay về trạng thái mời khi vừa được đồng ý · 0 khung mời sau khi đồng ý |
| [ ] | UI-085 | «Rủ … tới đây» phác thêm tờ không mang quán, câu nói ngược · số tờ không đổi + câu đúng |
| [ ] | UI-086 | Người đề nghị đóng sheet thì không còn thấy «đang chờ» · câu chờ trên màn |
| [ ] | UI-090 | Tên trên bìa sổ bị cắt «Chat Tes…» · tên phân biệt được |
| [ ] | UI-092 | Lá ngày đang chọn nằm khuất · lá chọn thấy trọn (cả ngày xa) |
| [ ] | UI-126 | Hàng mời «Một đôi» dẫn tới màn không nhắc lời đề nghị · màn tới nói về lời đề nghị |
| [ ] | UI-127 | Khoảnh khắc M6 «sổ hai người mở» không diễn · M6 diễn một lần; C9 khung cuối |
| [ ] | UI-130 | «Rủ X tới đây» cho cặp bạn: chỗ vừa chọn bị bỏ · màn tới mang quán, hoặc nút ẩn |

### B9 · 26 ca

| Cần tiếp tục | ID | Finding → expected |
|---|---|---|
| [ ] | UI-015 | Cài đặt hiện giá trị giữ chỗ («Bạn», «B», công tắc sai) rồi mới đổi · không khung giữ chỗ sai |
| [ ] | UI-094 | Viewer web: ảnh cao 0, vuốt nhảy hai ảnh, chụm phóng cả trang · ảnh >0; «2/3»; không phóng trang |
| [ ] | UI-095 | Thả tim lỗi ở cuối tường: câu lỗi ở đầu tường · trong khung nhìn |
| [ ] | UI-097 | Rời «Thả khoảnh khắc»/«Đăng story» mất ảnh và chữ, không hỏi · hỏi hoặc giữ nháp |
| [ ] | UI-098 | Viewer mờ dần khi mở nhưng biến mất ngay khi đóng · ≥1 khung mờ giữa chừng |
| [ ] | UI-100 | Bài «Chỉ mình tôi» mở bởi người khác: hai khối lỗi, hai «Thử lại» vô ích · một khối, không «Thử lại» |
| [ ] | UI-101 | Viewer story: vùng chạm không role; câu hỏi xoá không nhận focus · axe 0; focus vào câu hỏi |
| [ ] | UI-102 | Ảnh dọc 9:16: xem trước trọn, lên tường bị cắt · cùng vùng thấy |
| [ ] | UI-103 | Album chỉ ghi năm, không ghi ngày chuyến · có ngày chuyến |
| [ ] | UI-104 | Ngày viết «28-09», «28/9/2026» lẫn lộn · một định dạng |
| [ ] | UI-105 | Tablet: ảnh tường 702–894px, cao hơn cửa sổ · cột ≤640 |
| [ ] | UI-106 | Ở 320px thẻ huy hiệu bẻ đôi chữ «châ/n» · cột chữ ≥120px |
| [ ] | UI-107 | Lưu công tắc lỗi: câu lỗi ở cuối trang · cạnh control |
| [ ] | UI-108 | Panel trong Cá nhân trông như màn con, Back rời tab · Back đóng panel |
| [ ] | UI-109 | «Đã lưu» chỉ có con số · mở được từng chỗ |
| [ ] | UI-110 | Xoá tài khoản: «XOA» không dấu; «XOÁ» tắt nút không lý do; Back rời trang · nhận «XOÁ» hoặc nói lý do; Back về bước 1 |
| [ ] | UI-111 | Câu cuối Cài đặt chỉ sai chỗ đổi tên · chỉ đúng chỗ |
| [ ] | UI-153 | Kệ trống chỉ có một câu, không hành động · một hành động qua EmptyState |
| [ ] | UI-155 | Long poll trả về thì tường cắt về trang đầu · giữ đủ bài sau sự kiện hoặc 30s |
| [ ] | UI-156 | Bài Cộng đồng có ảnh lên tường không ảnh · thẻ tường có ảnh |
| [ ] | UI-157 | Lỗi thao tác sổ hành trình nằm ở cuối sổ · thấy ngay sau khi chạm |
| [ ] | UI-158 | Không còn lối xoá bình luận của mình ở trang viết · xoá có hỏi |
| [ ] | UI-159 | Câu xác nhận đăng lại ngoài khung · trong khung nhìn |
| [ ] | UI-160 | «MỚI MỞ» nhớ theo máy · máy mới không «MỚI MỞ» huy hiệu đã thấy |
| [ ] | UI-161 | Chạm huy hiệu thứ tư không phản hồi mà vẫn gửi PATCH · không PATCH; nói lý do |
| [ ] | UI-162 | Mục «Thành tích…» mở màn «Hành trình» · tên mục khớp màn tới |


## Kiểm chứng bản gộp khi xuất bản

Cây detached sạch `44feee05`, npm ci độc lập, chạy:

```sh
make gate STRICT=1 ONLY="guard guard-range contract client-routes server-routes screens cors ownership python-touch go-vet go-test shared mobile"
```

- **11 chặng đạt / 2 hỏng / 0 skip**. Mobile **1511 pass / 1 fail / 0 skip**; Typecheck và web export phục vụ harness đã đạt; export `--platform all` của chặng mobile chưa chạy vì test đỏ. Lỗi là EmptyState nonretry ở Ending thiếu illustration theo gate mới. Các test khác không đỏ trong lượt này.
- Guard tree đỏ do lúc hợp nhất config làm mất entry digest lịch sử Go module. Đã sửa bằng cách trả nguyên các entry main, giữ metadata/ảnh Codex đã review. Kết quả rerun guard ghi ở dưới; không đổi source sản phẩm hoặc assertion để sửa số mobile.
- PostgreSQL thật **13 PASS**, sentinel có mặt, **0 skip**, cùng SHA `44feee05`. Dùng image oracle `dc1bd772` vì schema/migration trong scope diary không đổi; đây là tier chọn lọc diary+db, không phải toàn bộ Go PostgreSQL.
- Lượt gate đầu ngày03/10 bị gián đoạn khi mobile đang chạy, không có verdict hoàn chỉnh. Lượt04/10 ở trên là kết quả đầy đủ, giữ cả log đầu.
- Log máy local: `campaign/cp09/publish-gate.log`, `publish-gate-repeat.log`, `publish-postgres.log`, `publish-guard-fixed.log`. Không có native rerun trên bản gộp; evidence native riêng B9a/B10 không chứng nhận main production mới.

Guard đã sửa ở `3463df14`; rerun cây sạch **2 chặng đạt / 0 hỏng / 0 skip**, tree4868 file, range159 file/15 commit. Staged tài liệu **4 file đạt** trước commit xuất bản. Commit sau đó chỉ thay tài liệu; source app/Go giữ nguyên bytes của `44feee05`. Chưa đủ điều kiện bỏ draft hoặc landing main.
