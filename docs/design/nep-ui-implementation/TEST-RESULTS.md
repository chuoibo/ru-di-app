# Kết quả kiểm có evidence

## Checkout tiếp tục — kết quả source/build đã chốt

Cây `/tmp/rudi-nep-ui-final-20261006`, nhánh `nep-ui-final-20261006`, ngày 06/10/2026; HEAD nền `811c893`, source mobile trước/sau checks giữ 954 files và cùng hash; [final source identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) ghi local feature SHA/guard/clean status sau khi root chốt, không là full strict clean exact-SHA gate. Node 22.23.2, Go 1.26.8, Python 3.12.11 pinned. CPU taskset 0–1, GOMAXPROCS=2 và Go -p2; không load/benchmark hoặc động audit. [Checkpoint summary](/tmp/rudi-nep-ui-final-evidence/checks/checkpoint-summary.md) giữ kết quả theo thời điểm, không phải final all-green.

| Lệnh/scope | Exit/count thực | Phạm vi và giới hạn |
|---|---|---|
| `npm ci --offline` |exit 0, 709 package | Dependency cài mới từ cache, không stub/package patch |
| `npm run typecheck` final |exit 0 |[log](/tmp/rudi-nep-ui-final-evidence/checks/npm-typecheck-final.log), [exit](/tmp/rudi-nep-ui-final-evidence/checks/npm-typecheck-final.exit); mobile manifest trước/sau đồng nhất |
| Dist compile/fixup + final focused |87/87 pass, 0 fail/skip/cancel; compile/fixup exit 0 |[summary](/tmp/rudi-nep-ui-final-evidence/checks/mobile-final-focused-summary.json), [compile](/tmp/rudi-nep-ui-final-evidence/checks/tsc-tests-final.log); unmodified per-file Node harness bổ sung, không thay npm test exit |
| Expo export `--platform all` final |exit 0; web 2267 / Android 2725 / iOS 2643 modules; 220 files/105.697.773 byte union |[summary](/tmp/rudi-nep-ui-final-evidence/checks/expo-export-all-final-summary.json); web main JS 5.437.972 byte; native Hermes compile/export không native launch, không benchmark |
| Mobile strict stage |typecheck/build:check đạt; `npm test` exit 1, make exit 2; 198/209 file wrappers xanh, 11 đỏ | Runner mất TAP child; không 198 assertions. Maestro source tại checkpoint và 10 file môi trường; không all-mobile green |
| Mobile từng file nguyên bản bổ sung |209 file, 1580 ca: 1534 pass, 38 fail, 8 cancel, 0 skip |20 lỗi source đã chuyển sửa, 18 lỗi subprocess+8 cancel môi trường. Lượt đầu là historical; không trộn với final rerun |
| Root narrow regression source sửa |23/23 pass, 0 skip |[root-regression-first.log](/tmp/rudi-nep-ui-final-evidence/root-regression-first.log); scanner red controls giữ. Không full `npm test`, tập có giao |
| `npm run test:e2e`, required |exit 1, 0/9 file wrappers; build:check đạt | Docker/world/API không dùng được; localhost 9 chủ ý tránh stack default. Bổ sung 19 ca: 3 orchestration pass, 16 thiếu world/API, 0 skip; không live E2E |
| `make gate STRICT=1` |9 chặng đạt; dừng ở API 9 ca, own exec Ctrl-C exit 130; không có gate exit cuối |Minimal empty FastAPI+TestClient 12 s cũng hang. [stop](/tmp/rudi-nep-ui-final-evidence/checks/make-gate-stop.json); không gọi toàn gate pass |
| Go unit trong gate |154 package đạt, 17 package socket/httptest EPERM, 25 no-test |Package counts không là test counts; không all-Go green |
|17 strict stages bổ sung |migration/shared đạt; mobile typecheck/build đạt; account-auth Go đạt, ASGI timeout; 13 infrastructure blocked |[stage records](/tmp/rudi-nep-ui-final-evidence/checks/remaining-stages-summary.json). Không là một full gate hoàn tất; native thiếu emulator access, cargo thiếu, Docker/socket bị chặn |
| Python domain/web/import |exit 0, 782 pass+1 N/A skip, 0 fail |[log](/tmp/rudi-nep-ui-final-evidence/checks/python-domain-web-boundary.log),3.65 s. payments layer removed ADR-0015; không SQL/API persistence |
| Python guest targeted |exit 0, 114 pass+1 N/A skip, 0 fail |[guest summary](/tmp/rudi-nep-ui-final-evidence/guest-checks/summary.json); có giao broad 782, không cộng |
| Go guest/static/dieuchinh cuối |exit 0, 3 package; 16 top-level+18 nested pass, 0 fail 0 skip |[JSONL](/tmp/rudi-nep-ui-final-evidence/checks/go-guest-static-amendment-final.jsonl); presentation/CSP/form/byte oracle, không postgres-tag/browser rebuild |
| Go amendment regression QA |identity 3/3; canary+2 mutants đỏ đúng predicted test, harness SHA giữ |[summary](/tmp/rudi-nep-ui-final-evidence/guest-checks/regression-qa/summary.json); remove script/CSP blocks/reject→accept không tương đương. Isolated Go overlay, không clean exact SHA hoặc browser proof |
| Guide regression |31/31 pass; 176 frozen questions, 1702 reachable ≥1500 floor; Maestro 5/5 pass |Root independently reviewed four first-rank changes, recall@5 unchanged, seven exact MRR pins (small declines≤0.01, pair case improves); alias/helper parser corrected. Giữ snapshot/helper behavior và controls, không scanner weakening |
| Staged guard checkpoint |198 files pass |[log](/tmp/rudi-nep-ui-final-evidence/guard/staged-after-reviewed-allowances.log); exact-path/SHA reviewed allowances. Không final whole candidate tree hoặc clean-SHA gate |
| Actor compiler canary trực tiếp |exit 2, TS2345 missing actorId, 0 source diagnostics |[log](/tmp/rudi-nep-ui-final-evidence/checks/actor-canary-direct.log); Node spawnSync capture còn blocked |
| Guard |checkpoint candidate tree exit 2; initial 89 findings |Geometry canonical 61 icon/11 paper exact equality; exact path+SHA allowances đã được root review cho long coordinate false positives, không nới scanner. [Final identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) ghi kết quả staged/whole-candidate cuối |

Guest 14 dark frames root đã mở; pending 30→20→Không đồng ý→biên nhận cũ 30.000đ qua ordinary form CSP cũ; [index](/tmp/rudi-nep-ui-final-evidence/guest-ui/index.json). Source amendment CSP same-origin-only đã sửa, chưa live; executor socket cản rebuild/HTTP/DB. Final Go↔Python oracle **0/12 completed, blocked**; 12 cặp cũ dưới đây là lịch sử. Historical JS 4.327 byte double-click thật đã có [browser+PG proof](/home/lakiet/.local/share/rudi-nep-ui/evidence/guest-browser-claim.md):0→1 report, UUIDv4 hợp lệ, Back vẫn 1 cùng ID/key. Latest JS 4.940 byte không claim POST; no-JS duplicate/failed-request pageshow/final-source recapture còn gap. Cross-site POST 201 vẫn lỗi server.

Expo **0 final-source captures**, 119 historical frames. Native 112 PNG là metadata inventory, root 4 final frame đã mở; dirty cutoff trước latest source. Finish review `recapture`; quyền browser bị người dùng từ chối không bypass. [VISUAL](./VISUAL-REVIEW.md)/[MOTION](./MOTION-REVIEW.md) mô tả scope thực đã quan sát. Final TS/dist/focused/export đã chốt như bảng; [final identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) ghi guard/commit status sau root chốt. Full strict gate vẫn chưa hoàn tất/xanh; không cộng subsets giao nhau thành test total.

[Final verification packet](/tmp/rudi-nep-ui-final-evidence/checks/final-verification-summary.md) và [JSON](/tmp/rudi-nep-ui-final-evidence/checks/final-verification-summary.json) giữ 33 relative artifact hashes, JSON SHA-256 `7d08b9216d6b686f1c85a3744dd69191fe19728e61842e2431a233572c96b748`. Go guide final 45 top-level+53 subtests pass,0 fail/skip; final 87/87, TS/dist/export và mobile 954 stable ghi theo scope riêng, không cộng overlaps. [Final source identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) ghi local feature commit và staged/whole-candidate/clean status ngoài Git, không là full strict clean exact-SHA gate hoặc visual approval.

## Packet execution cũ — lịch sử, giữ nguyên số để đối chiếu

Ghi nhận ngày 06/10/2026 trên execution worktree `nep-ui-20261005`, base `811c893fa76ab0d3f5583f18691c117c41439465`. Source chưa commit và còn đổi sau từng checkpoint. Kết quả dưới đây không phải full gate, gate-merge, canary hoặc gate trong clean tree ở exact SHA; không chứng minh đủ route/state/native/visual quality. Các tập focused có test giao nhau, không cộng 132+140+36 thành số test độc lập.

## Bootstrap và checkpoint compile/export

Node 22.23.2; Python venv 3.12.11 từ requirements-dev pinned; Go 1.26.8 pinned theo bootstrap root. Hai `npm ci` riêng baseline/execution hoàn tất. Export baseline ban đầu lỗi Metro do node_modules symlink qua worktree; đã bỏ symlink, cài deps thật và export lại. Lỗi launcher đầu của typecheck thiếu Node PATH (exit 127) đã được sửa bằng PATH đúng; đây không phải lỗi TypeScript.

| Lượt | Kết quả thực | Evidence | Giới hạn |
|---|---|---|---|
| Asset handoff | 686/686 checksum match; 0 thiếu/sai | [checksums handoff](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/checksums.sha256>) và [ASSET-PROVENANCE.md](./ASSET-PROVENANCE.md) | Read-only hash; không chạy builder hoặc sửa pack |
| Root central typecheck checkpoint4 | `npm run typecheck`, exit 0; Node PATH v22.23.2; gồm SoBia/shared keyboard geometry/calendar/Destination Fabric fix | [typecheck-checkpoint-4.log](</home/lakiet/.local/share/rudi-nep-ui/typecheck-checkpoint-4.log>) | Working tree tại checkpoint; không là clean exact-SHA gate |
| Root Expo web export checkpoint3 | Export hoàn tất: **93 assets, 5 web bundles, 3 files** | [after-export-checkpoint-3.log](</home/lakiet/.local/share/rudi-nep-ui/after-export-checkpoint-3.log>) | Có source changes sau export 3; chưa tuyên bố export bản cuối |
| Root export trước checkpoint3 | Export hoàn tất, log giữ riêng | [initial export](</home/lakiet/.local/share/rudi-nep-ui/after-export.log>), [checkpoint1](</home/lakiet/.local/share/rudi-nep-ui/after-export-checkpoint.log>), [checkpoint2](</home/lakiet/.local/share/rudi-nep-ui/after-export-checkpoint-2.log>) | Không dùng ảnh/export trước sửa làm state hiện tại |

Đọc thư mục export trên ổ tại checkpoint tài liệu: baseline/web 72 file, **22.370.373 byte**, 7 JS **5.993.860 byte**, không có WebP handoff; after/web 105 file, **52.256.755 byte**, 4 JS **6.073.520 byte**, 36 WebP **29.806.924 byte**. Đây là inventory file export, không là benchmark trước/sau, network gzip, memory hoặc cold-start. Số bundles trong log Expo và số JS file trên ổ là hai thước đo khác nhau. Không chạy benchmark/load thêm khi audit đang soak.

## Regression focused đã chạy

| Packet | Phạm vi có mục đích | Kết quả | Evidence chính | Không chứng minh |
|---|---|---|---|---|
| A–F | 12 suite contract/catalogue/credit/OTP/gu/kèo/map/thứ tự Tạo; `tsc -p tsconfig.test.json`, ESM fixup rồi Node tests | **132 pass, 0 fail, 0 skip, 0 todo**; TAP 814, 8 ms; typecheck exit 0 | [contracts.tap](</tmp/nep-af/contracts.tap>), [typecheck.log](</tmp/nep-af/typecheck.log>), [surface contract](</home/lakiet/project/mobile/wt-nep-ui-20261005/.impeccable/surfaces/apps-mobile-src-rudi-screens-welcome-tsx.md>) | Native permissions, provider/config thật, touch/font/visual trong mọi state |
| G–I + assets/draft | 16 focused file; compile test/fixup; group/friend/story/consent/paper/archive contracts và actual React hook lifecycle | **140 pass, 0 fail, 0 skip, 0 todo**; TAP 1412, 8 ms; fresh typecheck exit 0 | [targeted-gh-i.log](</tmp/rudi-nep-asset-lifecycle-ff1q_850/targeted-gh-i.log>), [result.json](</tmp/rudi-nep-asset-lifecycle-ff1q_850/result.json>), [typecheck log](</tmp/rudi-nep-asset-lifecycle-ff1q_850/typecheck-gh-i.log>), [handoff](</tmp/rudi-nep-asset-lifecycle-ff1q_850/GHI-handoff.md>) | Native UI-thread/worklets, delivery AccessibilityInfo/AppState/OS, E2EE v2, full Story timer/device photo rendering |
| J/K/L/P/Q/R/S | `rudi-cong-dong-b7`, `ky-niem-giay`, `thanh-tich`, `rudi-bai-chi-tiet`, `luong-anh-dai-dien`; compile/fixup + `node --test`; typecheck | **36 pass, 0 fail, 0 skip, 0 todo**; final 163, 6 ms; `tsc --noEmit` pass theo packet | [nep-ui-runtime-focused.log](</tmp/nep-ui-runtime-focused.log>), [handoff với lệnh/phạm vi](</tmp/nep-ui-jklpqrs-handoff.md>) | Provider/quota/media pipeline, moderator write thật, DB, keyboard/responsive/native |
| Root guest Python targeted | Guest/presentation và import-boundary regression được root ghi log | **51 passed, 1 skipped**, 0, 33 s; skip `payments layer not present` | [guest-test.log](</home/lakiet/.local/share/rudi-nep-ui/guest-test.log>) | Skip không tính pass; không là toàn product/meta/PG/parity gate |

G–I đã mở ảnh Invite light 390 và sửa duplication art/form; ảnh đó trước source final/fixture mới. Settlement light 390 cũng chỉ chứng minh ảnh đã mở tại state cũ. Các log test trên không chuyển các ô “Chưa kiểm” trong coverage thành runtime pass. Native/Web visual evidence được root và Android agent tổng hợp riêng; không suy từ APK build hoặc Unit tests.

## Identity và mutant có ý nghĩa

| Harness | Identity | Mutant không tương đương, điểm đỏ đã dự đoán | Evidence/giới hạn |
|---|---|---|---|
| Resolution + actual motion hooks | **23/23**, exit 0: 4 resolution + 19 lifecycle React effects | `cover-budget`: cover dùng budget contain → chọn small sai ở portrait; **exit 1/red**. `no-cancel`: settle value nhưng device clock còn chạy → **exit 1/red** | [identity.log](</tmp/rudi-nep-asset-lifecycle-ff1q_850/identity.log>), [cover mutant](</tmp/rudi-nep-asset-lifecycle-ff1q_850/mutant-cover-budget.log>), [cancel mutant](</tmp/rudi-nep-asset-lifecycle-ff1q_850/mutant-no-cancel.log>), [hashes/result](</tmp/rudi-nep-asset-lifecycle-ff1q_850/result.json>) |
| Story draft thật | **2/2**, exit 0 | Bỏ uploaded photo khi caption rỗng làm mất retry draft qua remount → **exit 1/red** | [draft identity](</tmp/rudi-nep-asset-lifecycle-ff1q_850/story-draft-identity.log>), [draft mutant](</tmp/rudi-nep-asset-lifecycle-ff1q_850/mutant-uploaded-draft.log>) |
| Kho chat cũ, real hook fetch boundary | **2/2**, exit 0 | Bỏ send gate → archived writes lọt network; bỏ read-marker gate → GET history sinh write viewport; mỗi mutant **exit 1/red đúng case** | [archive result/harness hash](</tmp/rudi-nep-gh-i-mutants-5shdvfms/result.json>), [identity](</tmp/rudi-nep-gh-i-mutants-5shdvfms/identity.log>), [send mutant](</tmp/rudi-nep-gh-i-mutants-5shdvfms/mutant-send-gate.log>), [read mutant](</tmp/rudi-nep-gh-i-mutants-5shdvfms/mutant-read-mark.log>) |

Lifecycle harness chạy production hooks và React effects thật; thay scheduler/haptics/platform/navigation boundary để điều khiển thời gian. Bao gồm chưa có response→không diễn, namespace event, live OS reduced giữa phiên, blur/background/hidden, cancel, remount/skip không replay/haptic thành công giả. Đây là evidence cô lập với artifact/harness hash, không phải canary/identity/mutants của full clean-tree gate. Các identity subsets nằm trong hoặc giao với focused packets; không cộng thêm vào 140.

## Runtime API/adapters và guest byte oracle

Nguồn [runtime-vertical-slices.md](</home/lakiet/.local/share/rudi-nep-ui/evidence/runtime-vertical-slices.md>), [summary.json](</home/lakiet/.local/share/rudi-nep-ui/evidence/runtime-verification-summary.json>), [live-read-adapters.json](</home/lakiet/.local/share/rudi-nep-ui/evidence/live-read-adapters.json>). Chỉ dùng synthetic fixture, stack QC riêng; không động audit đang soak và không có load/benchmark trong lượt này.

- Summary ghi **186 bước**, **185 expected-status pass, 1 fail**: cross-site guest POST nhận 201. Status totals:200 × 102, 201 × 65, 403 × 2, 202 × 2, 404 × 8, 204 × 2, 303 × 2, 409 × 2, 503 × 1. Đây là các request đã ghi, không là route/action coverage hoặc latency measure.
- **14/14 shipped adapter case, 14 HTTP request thật**, không mock fetch: memory/media, private/public diary và private 404, future Ending, post/comment/reply/saved/feed/catalogue stops, collection, Keeps empty, notifications, friend story. Evidence có SHA-256 source+compiled artifacts và Nodev22.23.2; không chứng nhận source hiện tại sạch.
- **12 cặp Go↔Python body cùng byte**: one/two-obligation pages, not-me, wrong-amount/evidence, revoked/expired/report limit, neutral unknown 404, CSS/JS và same-key claim replay. no-store/no-referrer/noindex headers khớp. Amendment Go-only không được gán parity Python.
- Publish/self-report/refresh/receiver confirmation/dispute/evidence/not-me/expiry/budget đã dùng endpoint thật. Same-key replay trả cùng reportID. Ordinary form body không key bị hai 201/hai ID ở probe trước JS/idempotency fix. [Browser proof lịch sử](/home/lakiet/.local/share/rudi-nep-ui/evidence/guest-browser-claim.md) sau fix JS 4.327 byte cho double-click 0→1 report, UUIDv4 hợp lệ và Back giữ 1 cùng ID/key; supersede pre-fix finding của runtime summary. No-JS double-submit và failed-request pageshow chưa chứng minh; latest JS 4.940 byte chưa claim POST.
- Amendment app-side proposed/applied chạy thật: proposed chưa đổi allocation; affected member accept sinh expense version mới; receipt obligation cũ bị 409 superseded. Anonymous review links chưa có do cả fixture actors có managed sessions.
- Nếp cộng đồng trả 503 `nep_unavailable` dù explicit consent/excerpt đúng; chỉ chứng minh error boundary và Keeps empty. Chưa chứng minh viết/review/edit/save Keep thành công.

## Cổng chưa đạt hoặc chưa chạy

Full `npm test` và strict stages đã được thử ở checkout tiếp tục nhưng chưa xanh; xem checkpoint phía trên. Product+meta+real PostgreSQL/parity tier và clean exact-SHA canary/identity/two mutants toàn gate chưa có final evidence. iOS/device crypto, Google/email delivery, real GPS/camera/native permissions, provider success, full web/native route/action/font/theme/keyboard matrix và startup/FPS/memory không được suy ra từ focused pass. Điều kiện và ưu tiên độc lập xem [BLOCKERS.md](./BLOCKERS.md).

Impeccable detector lịch sử **exit 2, 12 findings** giữ nguyên. Root đã hoàn tất [read-only triage](/tmp/rudi-nep-ui-final-evidence/impeccable-detector-triage.json), không chạy detector lần hai. Bốn side-tab notices đã sửa bằng horizontal rule; padding dark receipt 320–1366 đã mở; Roboto và 10/20/14 radii thuộc hệ incumbent; Jinja/comments và linked-CSS context làm một số nhận xét source không đại diện rendered copy. Không global metadata repair/full-app green; fresh finish vẫn recapture. [Original status](/home/lakiet/.local/share/rudi-nep-ui/evidence/impeccable-detect-status.json), [findings](/home/lakiet/.local/share/rudi-nep-ui/evidence/impeccable-detect.json), [stderr](/home/lakiet/.local/share/rudi-nep-ui/evidence/impeccable-detect.stderr).

## Hash log trọng yếu

SHA-256 tính lại từ file hiện hữu khi tổng hợp; path tuyệt đối ngoài mọi worktree chứa runtime evidence. Không đưa session/env/capability/token URL vào tài liệu.

| Evidence | SHA-256 |
|---|---|
| [contracts.tap](</tmp/nep-af/contracts.tap>) | `60659a58f847fa5d9e7cd614c527deacb08518fdc89941914a60a12726042331` |
| [result.json](</tmp/rudi-nep-asset-lifecycle-ff1q_850/result.json>) | `2e5d98b3bbf19dff32c2af4f68dc3c9611e115baa3f995756b8a5635b03700fb` |
| [targeted-gh-i.log](</tmp/rudi-nep-asset-lifecycle-ff1q_850/targeted-gh-i.log>) | `68226796af69fa4b9ca9f4c4b5ce2012a17288cc8945e0e80f7079dc50c4e523` |
| [result.json](</tmp/rudi-nep-gh-i-mutants-5shdvfms/result.json>) | `b1f79711b393172489d5c59cead398e3e6b91b26cd5fbe4a60b647e35d23c698` |
| [nep-ui-runtime-focused.log](</tmp/nep-ui-runtime-focused.log>) | `9ccf606d25c77acb16498c9eddbeb45ce4c930e1217f61e652379e7e42b3ffdd` |
| [guest-test.log](</home/lakiet/.local/share/rudi-nep-ui/guest-test.log>) | `802a8c66ce6d45cb7f62760c0be2d7e4a779eaec316bd4856915d444e1a9dda4` |
| [after-export-checkpoint-3.log](</home/lakiet/.local/share/rudi-nep-ui/after-export-checkpoint-3.log>) | `0219c407d1edce933daf81ae3b9889052b0e75320aae8b90eddb28065b9c7a79` |
| [typecheck-checkpoint-4.log](</home/lakiet/.local/share/rudi-nep-ui/typecheck-checkpoint-4.log>) | `6a5365a7bb40c3ca387f36db78409447c32d44f69ec1737ce498eae83f040589` |
| [runtime-verification-summary.json](</home/lakiet/.local/share/rudi-nep-ui/evidence/runtime-verification-summary.json>) | `67ad450cd06d269b41059d2bb1698b770d22598f4997de1d372a72edc2ca3f6c` |
| [live-read-adapters.json](</home/lakiet/.local/share/rudi-nep-ui/evidence/live-read-adapters.json>) | `057713ff277dd8c05c5083defd7e95c653912a59233fb06ebdd6e1c36f11387e` |
