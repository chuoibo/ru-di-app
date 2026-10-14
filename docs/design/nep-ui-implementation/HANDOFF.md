# Bàn giao frontend Rủ Đi cùng Nếp — bản có thể review, chưa nghiệm thu cuối

> Bản này là bàn giao lịch sử ngày 06/10. Khi tiếp nhận nhánh recovery hiện tại, đọc [HANDOFF-20261014](../nep-ui-recovery-20261006/HANDOFF-20261014.md) trước: có toàn bộ commit/file index, evidence portable, QC hai Android của Random, các lỗi còn mở và cảnh báo receipt runtime đã hết hiệu lực sau đổi host boot. Không dùng blocker/PID/đường dẫn lịch sử bên dưới như trạng thái live hiện tại.

Ngày 06/10/2026. Source tiếp tục ở `/tmp/rudi-nep-ui-final-20261006`, nhánh `nep-ui-final-20261006`, nền `811c893fa76ab0d3f5583f18691c117c41439465`. [Migration packet](/tmp/rudi-nep-ui-final-evidence/source-migration.json) ghi 178 file đã sao chép từ execution worktree `nep-ui-20261005`, rồi sửa tiếp trong checkout mới. Main repo và execution cũ được giữ. Đây là source feature có thể review; [final source identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) ghi commit local, trạng thái clean tree và kết quả guard sau khi root chốt. Không là landing `main` hoặc full strict gate xanh.

Phạm vi source là toàn A–S, **62 cửa Expo +2 layout =64 file**, và **4 cửa guest ngoài Expo**. Alias dùng renderer/context thật, không phải 62 business screen khác nhau. Inventory cuối theo packet guest là **700 hàng layer/action: 669 reachable +31 N/A có lý do**; source inventory không phải 669 runtime pass. [ROUTE-COVERAGE](./ROUTE-COVERAGE.md) và [LAYER-ACTION-COVERAGE](./LAYER-ACTION-COVERAGE.md) là nơi đọc từng role/state/action; các header/index 693/663/30 của packet web cũ giữ giá trị lịch sử, không dùng làm tổng cuối.


Ma trận cuối ghi649 hàng trước đây “Chưa kiểm” thành **BỊ CHẶN — kiểm web cuối** với điều kiện browser/socket từng hàng; 20 quan sát guest giữ scope bounded, 31 N/A giữ reason.64 Expo route/layout rows cũng giữ blocker kiểm cuối. Trạng thái này không là fail hoặc pass của từng chức năng, và không phủ nhận API/native checkpoint riêng.

## Thiết kế và source đã thay

Đây là extension của **Sân khấu giấy/sổ chuyến đi** hiện hữu, seed `c8e88116`, PRODUCT/DESIGN/ADR-0037 và hai surface contracts đã pin. `DESIGN.md`, `.impeccable/design.json`, config giữ nguyên. Coral cho thao tác, teal cho tiền, violet cho AI; Bricolage ở title/money/stamp, body hệ thống; giấy/vải gắn tác vụ. Không đổi hệ hay repair drift ngoài phạm vi.

| Nhóm | Vật/task và sửa cụ thể | Source/primitive để tiếp tục |
|---|---|---|
| A–B | Welcome giữ 4 trang có art slot hữu hạn, auth/form/gu giữ state thật; không copy fixture/kicker. Keyboard/font có shared constraint | `screens/Welcome.tsx`, `screens/auth/*`, `Onboarding.tsx`, `ui.tsx` |
| C–F | Bưu thiếp ảnh/credit/fallback có nhãn, vé kèo/lịch/map/editor và hub object đúng context. PlaceArtwork chọn resolution tự động dp × DPR, không ép medium/small; RouteLine circles giữ trong slot. ContextId/ctx/UUID aliases và calendar Android Back có regression | `screens/explore/*`, `screens/keo/*`, `hanh-trinh/*`, `Create.tsx`, `ui/RouteLine.tsx` |
| G–I | Inbox/bìa, invite envelope response, story draft/upload; hai chỗ ký và consent riêng. Kho legacy giữ read-only, chặn send/read-marker writes; v2 không fallback plaintext | `screens/chat/*`, `screens/groups/*`, `screens/hai-nguoi/*`, archive hook/registry |
| J–L | Media/album/wall thật, Ending/manual diary/reader, profile/person/badges; SoBia cloth/BookView và neutral PaperPalette type graph, caption ink theo giấy thực | `screens/ky-niem/*`, `diary/*`, profile/people/achievement, `ui/SoBia.tsx`, `diary/BookView.tsx` |
| M–N | Receipt/bàn gán món sạch, integer amount; nghĩa vụ dẫn trước sơ đồ gấp. Stamp ĐÃ GHI SỔ chỉ response, self-report/receiver-confirmed nghĩa riêng | `screens/chia-bill/*`, `Settlement.tsx`, `screens/dot-thu/*`, `Finance.tsx` |
| O | Guest receipt/error/inactive/static presentation; Go amendment review có same-origin CSP script và native named accept/reject form regression. Python truyền thống chỉ giữ presentation oracle parity, không backend nghiệp vụ mới | `services/core/internal/web/guest`, `web/static`, `internal/dieuchinh`; `services/api/app/web` oracle |
| P–Q | Feed/post/comments/media/consent thật, Keeps riêng saved posts, utility Search clear, Review cảnh nhỏ không Nếp; 9 disabled reasons đúng busy/biên trang | Community/PostCard/PostDetail/Comments, Search/Review/Keep/notification sources |
| R–S | Settings rows yên tĩnh, destructive/state theo contract; unknown heading/focus/session, root/tab/aliases/context tái dùng primitives | Account/settings/session sources, routing wrappers, `ui.tsx` |

Boundary giữ nguyên: route ownership là `services/core/ownership/routes.json`; template guest chỉ render view model, không query; một module một writer; tiền là integer/Σ/version/ledger; consent/audience/privacy theo API; `docs/protocol/v1` bất biến. Ngoại lệ legacy Python ở lượt này là presentation/static oracle và regression đối chiếu, không thêm business writer.

## Assets đang ship và provenance

Registry chung `apps/mobile/src/rudi/asset-pack/react-native/AssetIllustration.tsx` + `source/raster-metadata.json`; geometry chung và `usePaperPalette` đọc theme hiện hữu. Runtime **36 WebP =12 design × 3 resolution**, **29.806.924 byte**:384=1.589.886, 768=6.034.952, master=22.182.086. Cả 36 khớp byte/hash handoff; 686/686 pack checksums khớp. 61 icon và 11 paper geometry giữ equality với canonical source. Runtime không mang master PNG/reference/Studio/QC/pose static/font duplicate.

Raster crop/fit/focal theo manifest: cảnh cover có nhãn minh hoạ khi không là ảnh venue; spot/decor contain. Variant theo rendered width × DPR; hero density cao có thể dùng master. UI chữ/số/control không baked vào artwork. Nếp dùng rig `art/nep.ts`/`nep-roi.ts` và NepDien hiện hữu; Wordmark glyph canonical, font/texture giữ provenance. [ASSET-PROVENANCE](./ASSET-PROVENANCE.md) có từng file/SHA, source roles và giới hạn bytes.

## Phần đã chạy và phần chưa đủ proof

| Platform/role/scope | Kết quả có thật | Giới hạn |
|---|---|---|
| Synthetic API/adapter world, owner/member/guest/pair/operator | Packet cũ 186 request steps, 185 expected-status pass, 1 CSRF fail; 14/14 adapter HTTP read cases thật. Bill→ledger/batch/receiver/amendment, consent/diary/community permissions có API evidence | Không 186 UI flows, không tất cả actions. Cross-site known-capability POST 201 chưa sửa server; AI success 503 chưa có |
| Anonymous amendment HTTP |49 HTTP requests/61 checks; fresh Quan:11 HTTP requests/14 checks trong packet trước | Partial, không pass trọn; không đọc thành UI/SQL final proof |
| Guest LIVE-GO dark |14 ảnh root đã mở; 320/430/768/1366 receipt, copy/disclosure/not-me/wrong-amount preview, two obligations/confirmed/revoked/expired/reported; pending 30→20 rồi Không đồng ý→biên nhận cũ của chính mình 30.000đ, GET lại review chuyển về receipt | Runtime Go cũ CSS 10534/JS 4940; amendment CSP mới source-only. Decline là ordinary native form do CSP cũ chặn JS. Không JS-enhanced/double-submit/no-JS-repeat/PG snapshot proof. Reported không là 429 |
| Guest browser JS lịch sử |[JS 4.327 byte double-click + PG snapshots](/home/lakiet/.local/share/rudi-nep-ui/evidence/guest-browser-claim.md) chứng minh 0→1 report, UUIDv4 hợp lệ; Back thật vẫn 1 cùng ID/key |Supersede pre-fix finding. Latest JS 4.940 byte không claim POST; no-JS duplicate/failed-request pageshow/final-source recapture còn gap. Không cộng vào totals khác |
| Android phone emulator guest/account |Actual native password login; A–F và một số G–S checkpoints, font 1.3/2, Back/keyboard/reduced/map state theo transcript. Root mở 4 final native frame; documenter mở Welcome/Fix4 normal | Android 15/API 35 emulator 5630, DPR 2.625, dev APK+Metro 8097; dirty cutoff trước latest source. Debug anchor không release proof; iOS/tablet/hardware thiếu. Fabric crash uncertainty còn ghi |
| Expo web |49 light+51 dark after lịch sử, 19 before/17 family; montage/root review ở checkpoint |0 final-source captures; permission denial chặn fresh UI. Source late changes không được gán thành final visual pass |
| Motion |Production React effects/registry resolution identity 23/23 với cover-budget/no-cancel mutants đỏ; story/archive mutants riêng. Android navigation normal/reduced clips tồn tại | Không mọi success moment/device lifecycle; screenshot không timing proof |
| Source regression/build |Final focused 87/87, 0 fail/skip, typecheck+dist compile pass; root narrow 23/23 là checkpoint lịch sử; Go guest 16 top-level+18 nested/3 packages không skip; Python guest 114 pass+1 N/A, broader domain/web/import 782 pass+1 N/A |Tập có giao, không cộng. Guide 31/31 và Maestro 5/5 đã pass; staged guard 198 files là checkpoint pass. Expo all-platform export exit 0; full strict gate vẫn chưa hoàn tất/xanh; đọc TEST-RESULTS |

Evidence trước/sau từng family và scoring giới hạn: [VISUAL-REVIEW](./VISUAL-REVIEW.md). Clips/cơ chế: [MOTION-REVIEW](./MOTION-REVIEW.md). [Fresh finish review](/tmp/rudi-nep-ui-final-evidence/impeccable-final-review.md) trả **recapture**, không ship/approval; 9 ảnh đã mở không là full matrix.

## Sổ 13 luồng theo §16 — ranh giới bước đã kiểm

Mỗi hàng ghi phần đã chạy, không phải một verdict cho cả luồng. “API thật” là synthetic world ở checkpoint cũ; “native thật” là dev-client Android phone emulator; “guest thật” là browser LIVE-GO ở version đã ghi. Bằng chứng source/unit hoặc ảnh route lịch sử không chuyển thành UI-flow pass. Chi tiết HTTP/roles ở [runtime packet](/home/lakiet/.local/share/rudi-nep-ui/evidence/runtime-vertical-slices.md), [14 adapter cases](/home/lakiet/.local/share/rudi-nep-ui/evidence/live-read-adapters.json); native ở [device](/home/lakiet/.local/share/rudi-nep-ui/evidence/android/device.json) và [capture inventory](/tmp/rudi-nep-ui-final-evidence/native/metadata-consolidation.json). Các nhận xét pre-fix guest/anonymous của runtime packet được supersede bằng các packet mới đã nêu.

| Luồng | Role/platform và bước hoàn tất có bằng chứng | Ranh giới còn thiếu / link UI proof |
|---|---|---|
| 1 Welcome→auth→gu→Explore |Guest/account; actual Android password login qua Go API thành công. Welcome warm và một số A–F state có native captures; source giữ 4 trang |Register/verify/reset/OTP/resend/error/logout/restore và trọn chuỗi tới gu/Explore ở source cuối **chưa kiểm trọn**; Google/email success **chưa kiểm**, chưa có provider-success proof. [Visual A/B/C](./VISUAL-REVIEW.md) |
| 2 Destination→search/filter→save→place→kèo |Account; API catalogue/stops thật; adapter catalogue timeline đọc HTTP thật. Native A–F checkpoints và ảnh Explore/place lịch sử có scope |Không tự nhận toàn search/filter/save rollback/picker/add-to-kèo UI pass. Latest high-DPR variant/crop và conflict/error layers chưa recapture; [coverage C/D](./LAYER-ACTION-COVERAGE.md#c) |
| 3 Nhóm→invite→accept/reject→members→leave/remove |Owner/member/người ngoài; API tạo synthetic group, invite và người được mời tự accept thành active. Roster/server roles thật |Invite envelope/lookup/duplicate/reject/leave/remove và nested Back trên UI source cuối **chưa kiểm trọn**; không provider blocker. [Coverage H](./LAYER-ACTION-COVERAGE.md#h) |
| 4 Kèo→chặng→map→check-in |Organizer/member; past/upcoming có 3 stops catalogue thật, organizer khép past qua API. Android create/calendar Back/map-state checkpoints; Fix4 font2/IME field+CTA đã mở |Add/edit/reorder/remove/conflict/map editor/check-in UI source cuối chưa phủ; routing/GPS/camera success chưa có device/provider proof, manual/error actions độc lập. [Visual E/F](./VISUAL-REVIEW.md) |
| 5 Chatv2 keys/device→send/receive/media/tools |Account; archive hook real fetch boundary đã kiểm send/read-marker không ghi legacy; UI archive có nhãn read-only |V2 native transport/key adapter chưa có:send/receive/E2EE success **BỊ CHẶN**; không plaintext fallback. Screenshot/unit không crypto evidence. [Blocker v2](./BLOCKERS.md) |
| 6 Hai người→offer/consent→settings/gu→tờ hẹn→outing |Pair actors; API cả hai consent lap_so/bat_doi, constraints/weekly role ghi thật, paper draft thật; cặp một phía còn pending |UI offer/reject/consent từng nấc, nested settings, tờ hẹn edit/send/accept→outing, recap/close theo điều kiện **chưa kiểm trọn**. [Coverage I](./LAYER-ACTION-COVERAGE.md#i) |
| 7 Media→moment/story→viewer→album/wall→delete |Account/friend; API upload synthetic photo, tạo 2 memories; personal store story đúng contract, friend đọc được qua adapter HTTP |Native picker/crop/permission, upload fail/retry/cancel/remount/viewer/pause/delete UI source cuối chưa phủ. Camera/hardware là cổng riêng. [Coverage G/J](./LAYER-ACTION-COVERAGE.md#g) |
| 8 Ending→consent→manual diary→private reader→edit/AI |Owner/reader; manual private/public 2-page documents qua API; owner private200, other private404, public200; adapter diary/future Ending thật |Trọn editor/manual save/edit/paging UI source cuối chưa kiểm; Nếp503 là **provider blocker AI success**, không chặn manual path. [Coverage K](./LAYER-ACTION-COVERAGE.md#k) |
| 9 Bill→assignment→ledger→settlement→batch→finance |Owner/affected member; API bill 120.000đ, 4 allocations 30.000đ, batch 90.000đ; receiver confirmation và amendment proposed/applied tạo version/superseded409; board adapters thật |Manual320/keyboard/roster/result/write/error/double-submit/publish/share/finance UI final chưa đầy đủ. Không bank proof, không đổi integer/Σ. [Coverage M/N](./LAYER-ACTION-COVERAGE.md#m) |
| 10 Guest→self-report→receipt→dispute/not-me/evidence/limit |Anonymous; API token synthetic phát thật, isolation/self-report/receiver/dispute/evidence/not-me/expiry/reportbudget. Browser JS 4.327 byte +PG double-click 0→1 và Back giữ 1; latest 14 dark frames disclosure/copy/preview/expiry/decline30→20→biên nhận cũ 30.000đ qua ordinary form |Cross-site known-capability POST 201 **lỗi server**. Latest JS 4.940 byte không claim POST; no-JS duplicate/failed-request pageshow/final CSP/light/429/evidence-requested còn gap; final HTTP/PG socket **BỊ CHẶN**. [Guest proof](/home/lakiet/.local/share/rudi-nep-ui/evidence/guest-browser-claim.md), [14 images](/tmp/rudi-nep-ui-final-evidence/guest-ui/index.json) |
| 11 Feed→topic/search→composer→reaction/save→comments→moderation |Author/friend/reader/operator; API public/friends/pending, media/comment/reply/follow/like/saved và 5 feed modes; operator queue200/nonoperator403; review writes thật |Composer/error/draft/media menus/reaction rollback/comment actions/actual moderator UI source cuối chưa phủ; AI Keep503 tách riêng. [Coverage P/Q](./LAYER-ACTION-COVERAGE.md#p) |
| 12 Profile/avatar→people/achievements→settings/account/delete |Account; profile/settings/people route frames lịch sử; notification/read adapters có HTTP proof. Native account login đã chạy, focused contracts giữ money/privacy/role semantics |Không nhận edit/avatar/privacy/session revoke/unblock/password/logout/delete UI flow pass từ hình/fixture. Dùng disposable synthetic account cho destructive; provider/hardware riêng. [Coverage L/R](./LAYER-ACTION-COVERAGE.md#r) |
| 13 Deep links/alias/unknown→Back→session restore |Guest/account; source context/UUID/unknown/calendar regression; native Back ở các checkpoint, Fix4 keyboard actual frames |Full web Escape/focus/tab history/cold/warm/logout/restore/deeplink/source cuối chưa kiểm; Expo browser denial **BỊ CHẶN** new UI pass. Aliases không business screen mới. [Coverage S](./LAYER-ACTION-COVERAGE.md#s) |

Không cộng 13 hàng, 186 API steps, 14 adapter cases, 14 guest images hoặc 112 native PNG thành tổng luồng đã pass. Mỗi bước còn chưa kiểm có action/state/role ở coverage và lịch tái kiểm trong BLOCKERS.

## Chạy lại và tiếp tục kiểm

Dùng checkout feature mới, runtimes pinned Node 22.23.2/Python 3.12.11/Go 1.26.8 và cấu hình được quản lý ngoài repo. Không sao chép credential/env/session/capability URL vào tài liệu, logs công khai hoặc worktree. Tái dùng stack synthetic QC được phép; không reset/restart/prune container/volume/process audit. Không benchmark/load mới khi audit soak còn chạy.

Lệnh source/build thường từ `apps/mobile`:

```sh
npm ci
npm run typecheck
npm run build:check
npm test
npm run test:e2e
```

Khi runtime/browser được phép, dùng `npm run web`, `npm run android` hoặc `npm run ios` theo toolchain đã có; dùng dev-client phù hợp repo, không thay native bằng web. API front door là Go, quyền route đọc ownership. Stack/config/credentials theo README và tài liệu môi trường hiện hành. Các lệnh này là hướng tái chạy, không ghi là đã pass tại source cuối. Browser Expo bị từ chối phải được cấp lại access trước chụp; không đổi origin/port/tool để vượt chặn.

Ở root, chạy `make gate STRICT=1`; merge-result dùng `make gate-merge` khi phù hợp. PostgreSQL/parity/crypto/native là cổng riêng; thiếu sentinel/skip không green. Repo guard cần staged diff và whole candidate tree, exact path+SHA allowlist cho binary/geometry, không giảm scanner. Trước landing `main`: clean tree ở exact feature SHA, identity xanh/canary đỏ/hai mutants không tương đương cùng harness SHA, ảnh UI thật đã mở và số gates trong commit message tiếng Việt. Local feature commit không tự thành landing approval.

Để tái kiểm UI, chọn actor/context synthetic đúng role, chạy entry→write/error/retry/cancel→re-read; giữ API/domain bằng chứng, mở screenshot từng state. Ưu tiên các lượt độc lập manual diary, consent, story/media, feed/comments, utilities/account, money và aliases trong BLOCKERS. Provider AI 503 không chặn manual flows. Capture final source/bundle hash ở light/dark, 320/390/430/768/1366, font 1.3/2/keyboard; lifecycle dùng clip/event khử dữ liệu. Không copy share tray thành “đã gửi”.

## Điều kiện còn lại để nghiệm thu

Nguồn và inventory đã phủ phạm vi A–S, nhưng Definition of Done visual/runtime/gate chưa đóng. Browser denial, executor socket/stdio/Docker restriction, ASGI TestClient hang, iOS/hardware/provider/E2EE v2 và performance audit tách theo bằng chứng ở [BLOCKERS](./BLOCKERS.md). **CSRF server defect và no-JS ordinary duplicate claim gap là rủi ro thật**, không che bằng presentation. Final source captures, full UI layers/actions, native classes/crypto và clean exact SHA gate còn cần chứng minh.

Product mobile manifest trước/sau checks gồm 954 files, cùng SHA-256 `361e94c4c3582709fa2c3c1ecf66f09630889e114e0c5f14fc2226ad571050a5`. [Final export summary](/tmp/rudi-nep-ui-final-evidence/checks/expo-export-all-final-summary.json) ghi exit 0: web 2267 / Android 2725 / iOS 2643 modules, 220 files và 105.697.773 byte union. Main web JS 5.437.972 byte; Android/iOS Hermes chỉ compile/export, chưa launch source cuối. [Final source identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) là nơi tra local feature SHA và staged/whole-candidate guard status, tránh vòng lặp SHA trong tài liệu. Full strict gate không là clean exact-SHA pass; visual/flow/lifecycle DoD còn thiếu như trên. Không công bố 100%, state-of-the-art toàn app, ship hoặc main landing.

Bó kiểm cuối bất biến: [final verification packet](/tmp/rudi-nep-ui-final-evidence/checks/final-verification-summary.md) (33 relative artifact hashes; JSON SHA-256 `7d08b9216d6b686f1c85a3744dd69191fe19728e61842e2431a233572c96b748`). Go guide final 45 top-level+53 subtests pass,0 skip; subsets không cộng vào Go guest hoặc test totals. Source/guard/clean branch được ghi ở external final identity, không cập nhật SHA vòng lặp trong tài liệu.
