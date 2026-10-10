# Audit native và độ hoàn thiện thị giác — 08/10/2026

**Chưa nghiệm thu toàn frontend.** Theo quyết định người dùng, chỉ thao tác app
Rủ Đi trong Android emulator có cửa sổ. Không dùng web để thay bằng chứng native.
Mỗi màn/layer phải vừa chạy đúng vừa đẹp tương xứng mockup; giữ riêng hai kết quả.
Rule lâu dài ở [AGENTS.md](../../../AGENTS.md#cổng-chất-lượng-thị-giác-ui--quyết-định-người-dùng-08102026).

Đây là ledger native bổ sung cho catalogue 62 cửa/2 layout ở
[ROUTE-COVERAGE.md](./ROUTE-COVERAGE.md). Các ô «bị chặn web cuối» trong catalogue
là checkpoint lịch sử `811c893f`, không mô tả trạng thái quyền hoặc Android hiện tại.
Không đổi nhãn ảnh/capture lịch sử thành source mới.

## Nguồn và giới hạn

Hai emulator riêng `5558`/`5560`, 320×640 portrait, native Rust/JNI thật; tài khoản và
ảnh hoàn toàn tổng hợp. Primary5558 đã cài APK build cây sạch `2f20a301`, SHA256
`963bd5daa0be4348606158dd3c06848e7ef604e06ebbdf035136a718cae0d514`;
peer5560 vẫn APK `b87a16f1`, SHA256
`34e48965242785651cc982199e4b3d823d45d0b4b30a79a98a622080efd63967`.
Không gán APK mới hoặc JS9007 của primary cho peer chưa cập nhật.
Go runtime `54b5dc0e`; JS có fingerprint riêng theo từng lượt. APK/Go không được
gán thành build của SHA JS. Không đụng audit dài đang được bảo vệ.

Evidence nằm ngoài mọi worktree:
`/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/`.
Chỉ receipt, source và tài liệu không chứa credential được bàn giao trong repo.
Ảnh có capability hoặc credential vẫn giữ riêng, không trình bày/commit.

## Các phần đã kiểm và phần còn thiếu

| Nhóm | Chức năng native đã có bằng chứng | Thị giác đối chiếu mockup | Còn phải kiểm/hoàn thiện |
|---|---|---|---|
| A — Welcome/auth | Bốn chương, swipe/reset, Login; font130/200, giảm chuyển động, tối; đăng nhập fixture thật | Welcome `34d57ca9`: 15/16; ba finding nhãn/art/fold resolved; CTA→Login warm `3b71838e` có chuỗi frame được mở | Register/reset/error/keyboard toàn bộ; cold/back/cancel/background/reduced lifecycle; không suy toàn motion |
| B — Gu/onboarding | `6f3f5af2`: save/reopen4 mục/ngân sách thật; Settings entry chọn2/3 khóa lưu, Back/cancel giữ server4; restore UI3/vua-phai, GET200 khớp; tách khỏi capture d3 | Root mở14 ảnh thao tác6f; `d3c4a3aa`: ship chỉ sửa nhãn giữa, resolved; reviewer mở27 ảnh (24after/reference/fingerprint/before130), 14/14 tĩnh ở sáng/tối font1.0/1.3/2.0, state3-selected/vua-phai/enabled; motion N/E | Name/new account/keyboard, disabled/loading/error/offline/retry, TalkBack/motion/lifecycle, native390/430/tablet/iOS; chưa toàn Gu/font/state matrix |
| C — Explore/destination | Native `6f3f5af2`: Cafe2, tìm tên1, query lạ0; save/unsave GET200 khớp, câu hỏi provider thật ra2; đổi Hà Nội0 nơi/selected persisted/restore Đà Lạt; gallery sáng/tối có fingerprint đúng JS | Ship P2-01 viewport đầu: 14/14 tĩnh ở sáng/tối font 1.0; tên/fact đầu và artwork rõ trước nav | Ảnh thiếu/credit, offline/retry/double tap; chữ lớn/toàn state/motion và review các state mới chưa đạt |
| D — Place/chọn kèo | Detail/save/cancel6f; JS41 picker chụp đúng hàngQC rồi add19,689s exit0, GET200 own1→2/firststop/check-ins giữ, pairhai stop nguyên | Root mở ảnh so board01-panel3 và picker, chưa review đủ | Nhãn kèo cụ thể candidate sau41; lỗi/empty/layers/font matrix |
| E — Plan/outing/map | Native d3 tạo kèo/check-in; APK2f20 sửa wrapper0×0, ba coldmap/pan/khớp80,472s exit0; JS41 ground→tapmốc selected16,635s exit0; cleanup chặng thêm nhầm GET200 hai records gốc nguyên | Root mở timeline/map/diagnostic; chưa review đủ đúng source cuối hoặc settled camera | Reorder/pin/cluster/map provider/permission/conflict/partial creation; fullgate/TalkBack |
| F — Create/check-in | Native d3 empty→form/blanksubmit/name/budget→create200.000đ/2người/stop/check-in1, GET200 khớp; JS41 thêm địa điểm giữcheck-in, hủy draft không ghi | Root mở11PNG d3 và sheet41; chưa chấm toàn bộ sheet/check-in | Sheet CTA trên IME đang sửa sau41; từng entry/context/audience/location/cancel/font/theme/error |
| G — Chat/story | MLS hai chiều và media nhận/relaunch hai emulator; poll create/vote/tie/switch/author-close/relaunch; Story thay ảnh/draft/đăng/pause/Back/Keep/DELETE thật; Ngọc bạn đọc/seen/nonauthor | Khung media `2fc1c73f` mở ảnh so board03; Story preview/viewer/question `c9a3f541` 14/16, ship chỉ hai P2 và state đã chụp | Header/history-empty copy; motion/retry/expiry/unauthorized audience/pendingDELETE/TalkBack; poll→tờ hẹn thật; reseal/reverse media |
| H — Groups/friends | Native fa735/c941/d91: tạo nhóm, mời/từ chối/mời lại/đồng ý, đổi vai trò/last-admin, kết bạn pending/duplicate/decline/accept và rời/cancel; API QC thật | Sáu finding của fresh review resolved tại d91; matrix sáng/tối ×100/130/200, expanded và các đích/recovery thật. Chỉ nghiệm thu sáu fixes | Chặn, network/retry/double-tap/lifecycle, motion/TalkBack, expanded IME/chữ lớn, mọi nhánh; input tên dài vẫn cuộn hai dòng; toàn H còn mở |
| I — Hai người | Chưa nhận full native pair workflow | Chưa review native đủ | Hai phiên UI, tờ hẹn/accept/promote, chọn người và các nhánh |
| J — Moments/album/wall | Story không thay bằng chứng khoảnh khắc/album/tường | Chưa review native đủ | Media/crop/draft/retry/audience/view/album/delete, heart/comment đúng quyền |
| K — Ending/diary | b76: sáu profile, thêm/sửa/đổi thứ tự/chuyển/bỏ trang rồi lưu; no-op revision4→4 giữ document. Công khai4→5 rồi riêng tư5→6; other book/published photo200→404, ảnh gốc other luôn404 | Ba finding resolved tại b76; finding đường kẻ resolved tại5cab sau16 ảnh editor/shared caption130/200. Bìa tự giãn, một pager và nhóm soạn trang; review chỉ bốn finding | AI/consent/provider/quota/error/retry/photo-failure, mọi lifecycle/motion, TalkBack/contrast, tablet/iOS; chưa nghiệm thu toàn K |
| L — Profile/people | Profile edit/keyboard/cancel; avatar picker/upload/cancel thật; crop native `2c1189d7` cancel/confirm và server640×640 | Root mở profile/editor/after; chưa rubric đủ own/other/pending/blocked | Avatar error/retry/permission, propagation giữa máy; people/achievements; chữ lớn và states |
| M — Bill | Focused native822 nhập/preview/ghi bill200.000đ, phần75.000/125.000; witness thật; runner toàn bảng vẫn đỏ | Có ảnh đã mở; chưa nhận composition/state đầy đủ | Native whole-flow/canary cùng harness, scan camera/OCR/provider, font/keyboard/errors |
| N — Settlement/finance | Focused native822 batch75.000đ, xác nhận nhận và ledger về0; receiver confirmation không là bank evidence | Chưa nghiệm thu toàn state/layer native | Bảng đầy đủ, role/wait/conflict, share handoff và visual |
| O — Guest ngoài Expo | Cổng riêng, không gán emulator Expo thành guest proof | Không mở browser theo yêu cầu hiện tại | Fresh guest/oracle parity; phương thức visual guest phải giữ đúng phạm vi được phép |
| P — Community | Native deeplink/feed; gallery `6f3f5af2` sáng 35.859s/tối 19.539s/font 2.0 13.933s, đều exit 0; chưa full actions | Ship chỉ P2-01 viewport đầu và P2-02 «Để sau», resolved; baseline sáng/tối font 1.0 đạt 14/14 tĩnh, motion N/E | Font 2.0 chỉ nhãn/control ở viewport đã cuộn; feed/topic/post/edit/comment/report/moderation/filter/cursor/lỗi và toàn lifecycle còn thiếu |
| Q — Keeps/notifications/search | Lượt 52: consent→nháp→lưu thật, own-list của Ngọc không chứa ID keep của Minh và Keeps empty cuộn/bấm; editor `eba8678a` bấm gọi/lưu trên IME; sáng 2.0 41.118s/tối 1.0 53.484s exit 0, sáng 1.0 exit 1 sau save vì selector sai | Ship chỉ P2-NEP-EDITOR-01 CTA trên IME, resolved; 9→12/14 tĩnh, artwork N/A, motion N/E | P3 font 2.0: caret autoscroll đưa nhãn khỏi viewport/crop dòng đầu; chưa kiểm mọi dòng/offset, mọi route/action/error/empty/sheet, AI quality/privacy |
| R — Settings/account/privacy | Theme thật Sáng/Tối; device list read thật; login Ngọc lại thật; avatar crop qua settings | Root mở settings light/dark/device/profile, chưa rubric toàn family | Logout phải thu hồi đúng device còn là finding mở; remote session/revoke/privacy/block/delete-account; không xoá mọi máy để ép xanh |
| S — Root/tab/alias | Hai native runtime có ownership/fingerprint; tab/Back/relaunch thực trong các flow đã nêu | Welcome transfer được review có giới hạn | Deep-link/alias/auth-expiry/foreground/reduced lifecycle; keyboard/nav trong toàn matrix |

Không cộng các route đã mở thành số state/action đã đạt. Provider one-shot thật đã nối sau52b6ba1d; không thay nhánh chưa kiểm bằng response giả. iOS, phần cứng thật và review crypto độc lập vẫn là
cổng riêng, không suy từ emulator hoặc focused unit.

## Đối chiếu cụ thể

Rubric gốc tám trục ×2 ở prompt §13; identity, artwork, hierarchy và state phải2,
tổng ít nhất14/16. Điểm phục vụ tìm lỗi, không là chứng nhận khách quan. Mỗi điểm
phải có ảnh/trạng thái đã mở và nhận xét; motion cần clip/chuỗi frame thực.

- Welcome: [review ảnh34d57ca9](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-peer-r1/welcome-independent-verdict-34d57ca9.md);
  [review CTA warm3b71838e](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-peer-r1/welcome-independent-motion-3b71838e.md).
- Story: [review3b71838e](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-3b71838e/story-independent-verdict-3b71838e.md)
  giữ lịch sử **fix**. [Verdict passc9a3f541](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-c9a3f541/story-independent-verdict-c9a3f541.md)
  mở17afterảnh và chốt **ship chỉ hai P2**: hierarchy/header isolation resolved;
  câu hỏi13→14/16, preview/viewer14/16. Chạm header thật/Back/Keep và DELETE native
  đã kiểm; chưa pendingDELETE/TalkBack/motion/retry/expiry/composer matrix.
- Editor Nếp: [verdict eba8678a](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-eba8678a/nep-editor-independent-verdict-eba8678a.md)
  chốt **ship chỉ P2-NEP-EDITOR-01**, CTA gọi/lưu trên IME resolved ở Android
  320×640 sáng 1.0/2.0 và tối 1.0. Root/reviewer mở toàn bộ 17 ảnh packet,
  reviewer mở thêm ảnh lỗi cuối font 1.0. Lượt sáng 1.0 **69.962s, exit 1**
  đã lưu thật rồi timeout vì tìm title «Ghi chép riêng» thay vì «Điều mình
  muốn giữ»; không ghi runner này xanh. Sáng 2.0/tối 1.0 dùng đích
  `community-keeps`, đều exit 0. Điểm **9→12/14 tĩnh**, artwork/crop N/A,
  motion N/E; P3 autoscroll/crop font 2.0 còn mở. Font đặt trước fresh
  attach, chưa telemetry số RN. Diff `Sheet.tsx`/`PostDetail.tsx` từ eba tới
  `6f3f5af2` rỗng, không gán ảnh eba thành capture JS mới. Chưa toàn editor,
  Keeps, AI quality/privacy, error/retry/motion/TalkBack/font 1.3/tối 2.0.
- Gallery: [verdict 6f3f5af2](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-6f3f5af2/gallery-independent-verdict-6f3f5af2.md)
  chốt **ship chỉ P2-01 composition viewport đầu và P2-02 «Để sau»**,
  cả hai resolved; root/reviewer mở 10/10 PNG, fingerprint đúng JS.
  Community/Explore top, consent sau bài đầu và settings scheme snapshots
  sáng/tối font 1.0 **14/14 tĩnh**, interaction/motion N/E; /14 không thay
  ngưỡng /16. Font 2.0 chỉ nhãn/control tại viewport đã cuộn, không toàn màn
  chữ lớn. Consent dùng RudiButton inline, NBSP ở nhãn nhìn và khoảng trắng
  thường ở accessibilityLabel; source giữ handler/phạm vi consent. Không
  suy TalkBack/target/contrast đo native, lifecycle theme hoặc mọi action.
- Gu: [verdict d3c4a3aa](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-d3c4a3aa/gu-independent-verdict-d3c4a3aa.md)
  chốt **ship chỉ sửa nhãn giữa**, finding resolved: `styles.giua` thêm
  `alignSelf: "stretch"`, giữ chữ giữa/font/ngưỡng tiền/handler. Before130
  chỉ có «Trên»; after130 save-action sáng/tối có đủ «Trên 500K»/«Rộng tay».
  Reviewer tự mở27 PNG trong [packet](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-d3c4a3aa/gu-review-packet.json):
  24after, reference, fingerprint đúng JS và before130. Android320×640,
  sáng/tối × font1.0/1.3/2.0; chỉ state3-selected/vua-phai/enabled,
  **14/14 tĩnh**, motion N/E, không quy thành /16. Sáu receipt after exit0:
  sáng1.0 17.988s, sáng1.3 18.840s, tối1.0 18.038s, tối1.3 19.040s,
  sáng2.0-visible 25.798s, tối2.0-visible 25.648s; `not_full_gate: true`.
  Hai lượt font2.0 đầu đỏ do count ngoài fold vẫn giữ; visible là capture
  cuộn bổ sung, không đổi runner đỏ thành xanh hay gán lỗi UI. Font đặt
  trước fresh attach, chưa telemetry số RN. Reviewer thấy mode0664 ban đầu;
  parent sửa quyền rồi reviewer kiểm lại27/27 mode0600, không gán hồi tố.
  Save/cancel/restore6f là bằng chứng chức năng riêng, chưa chạy lại ở d3;
  chưa name/new-account/disabled/loading/error/offline/TalkBack/motion,
  native390/430/tablet/iOS hoặc toàn Gu/frontend.
- Avatar crop `2c1189d7`: editor thật được mở; Back exit0/6,422s giữ nguyên version
  và hash ảnh640×800. Confirm exit0/13,531s đổi version, GET200 ảnh640×640.
  Root mở editor/cancel/after; không suy crop/permission/retry cho mọi media.
- Bạn đọcNgọc `c9a3f541`: accepted friend trong danh sách nativeMinh; viewer không
  có nút xoá, đóng đổi vòng sang đã xem,16,835s exit0; root mở hai ảnh. Avatar cắt
  hiện trên rail và HTTPNgọc GET200 cùng hash640×640; chưa chứng minh người lạ,
  block/revoke/expiry hoặc mọi audience.

## Cổng trước landing

E/F checkpoint9007 có6/6 flow matrix và6/6 receipt bổ sung exit0; root mở
60+24PNG. Reviewer fresh-context vẫn trả **fix**,9/14 ở bảy trục tĩnh,
motionN/E: hierarchy/fade, overview, form IME, giờ, picker và thu gọn.
Diagnostic thu gọn44,672s exit1 giữ nguyên. Hai lượt đỏ khác là giả định
harness sai selected/checked và cluster/card, không gọi lỗi sản phẩm.
Candidate sửa sáu finding mới qua typecheck và55/55 focused; chưa có
native after/verdict pass. Chi tiết và giới hạn ở RECOVERY-20261008.md,
evidence ngoài repo `native-audit-9007c3bb/finish-review/`.

Checkpoint5569633c sau đó có fingerprint đúng,6/6 matrix và6/6 bổ sung exit0;
root mở60+24PNG. Verdict sáu finding: F1/F3/F5/F6 resolved, F2/F4 partial,
disposition fix. Nhãn / người mất chữ và HH:mm bị cắt với IME; đầu trang gấp
bị cắt dù state đã giữ đúng. Candidate tiếp sửa đúng ba điểm và rút lại hai
file hướng dẫn stale, qua86/86 focused+guide và Go guide; chưa có native
after/verdict cho batch tiếp. Chi tiết receipts/giới hạn ở RECOVERY; không
gộp green thao tác thành green thị giác. Clean Node556 vẫn ghi1602/1605,
ba red được giữ nguyên, không nhận là full npm test hoặc strict gate.

Main chưa nhận batch này. Full mobile sạch gần nhất `2fc1c73f`:1620/1620,
0fail/skip/cancel/todo; không gán cho sửa sau đó. Runtime plugin/detector
không tìm thấy, `imp preflight` exit127; không ghi detector clean.
Craft-floor có và đã được đọc tại
`/home/lakiet/.claude/plugins/marketplaces/impeccable/plugin/skills/impeccable/reference/craft-floor.md`;
đính chính các ghi chú cũ nói reference này thiếu. Còn full strict gate32 ở cây sạch SHA cuối,
identity/canary/ít nhất hai mutant độc lập cùng harness, fresh guest parity,
state/native matrix và Impeccable finish phù hợp tập capture cuối. Không mở web
hoặc chạy benchmark/load để thay những phần thiếu.

## Bằng chứng tiếp nối36f1c5ad

E/F đã có after cùng Android320×640 sáng/tối × font1.0/1.3/2.0:
6matrix+6bổ sung đều exit0, root mở60+24PNG; fold owned-retry
24,597s exit0 và bốn ảnh, fingerprint27,782s exit0 và hai ảnh. Giữ
startup fold red2,668s/exit1 riêng, chưa đủ bằng chứng gán lỗi sản phẩm.
[Verdict fresh36](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-36f1c5ad/finish-review/verdict.md)
chốt **ship chỉ scored fixes**: F2/F4/R2 resolved, F1/F3/F5/F6 reconfirmed;
không chấm lại /16, motion/TalkBack/iOS hoặc toàn E/F/app. Documenter
fresh trả No changes, sáu nhóm token mirror khớp. Chi tiết số ở RECOVERY.

Clean exact36: Node subset1605/1605,0fail/skip/cancel/todo,18,441s;
typecheck7,048s, build artifact14,973s, compile/fixup2,635/0,036s đều0,
tree sạch trước/sau. Năm browser-launch files bị loại trước run; static
export tạo prerequisite cho contract, không mở web và không thay visual
native. Không phải full npm test/strict32. Go guide0/wall0,283s và build
core0/2,408s. Owned API QC53559 đã dùng embedded guide36, health200;
không thay domain/schema hoặc stack audit khác.

Toolkit hiện hành đã hoạt động: plugin4.4.0/engine cache0.1.9;
E/F sáu TSX0finding/0,042s, C/D năm TSX0finding/0,030s. Đây là **regex
source TSX**, không đo contrast/geometry native. Các wrapper127 lịch sử
vẫn giữ; ghi chú unavailable phía trên mô tả thời điểm cũ.

Root mở24ảnh profile390/430/900 sáng/tối font1.0; sáu flow exit0 và
size/font restored320×640/1.0. Đây chỉ coverage loaded/transient đã chụp,
chưa full rubric mọi profile. Root đính chính đọc nhầm form390: heading
đã đủ; direct VIEW390 dark cũng đúng đầu trang, không focus input; chưa
có bug giữ cuộn. Dấu đen nhỏ mép Map430 chưa triage, không gọi cả profile
visual PASS.

C/D loaded hiện có70PNG đã mở ở100/130/200 sáng/tối, gồm Explore top,
lead/compare, destination selected/other rows, place/footer/final action,
chooser không submit.6flow chính+4supplement exit0; hai red harness vẫn
giữ và có ảnh/triage riêng. Fresh finish packet đang review, chưa có điểm
đẹp mới. Chưa credit error/offline/motion/TalkBack/wide C/D hoặc mọi action.
Full app A–S và cổng landing ở các bảng phía trên vẫn mở.

Fresh review C/D36 đã trả **fix** sau50primary+3reference: một finding
C/D–01 về cân khối ảnh/chữ, reason/price compact100. Candidate sửa
riêng PlaceLead, qua69/69focused, typecheck và guide freshness; chưa
credit after/verdict pass hoặc điểm đẹp mới. Không sửa theo screenshot
mang tên lead-facts100 nhưng thực tế là compare, hoặc theo hai red harness.

## Bằng chứng tiếp nối3bb47ef8

C/D–01 đã **resolved, ship chỉ finding đã chấm**: reviewer tự mở24after
và18before. Sáu flow sáng/tối × font100/130/200 tại320×640 exit0,
18,240/21,944/18,240/18,541/18,390/18,239s; root mở24PNG. Facts/reason/
price compact đọc đủ bề ngang; tail200 đủ giá/đơn vị sau cuộn. D detail
không recapture ở3bb; không có /16 mới, không nghiệm thu toàn C/D/app.
[Verdict3bb](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-3bb47ef8/cd-finish-review/verdict.md).

Clean exact3bb focused69/69,0fail/skip/cancel/todo,0,239s;
typecheck7,258s, guide1,295s, compile/fixup2,537/0,051s exit0, cây sạch
trước/sau. Hai fingerprint red39,216/94,536s giữ nguyên do sai chuỗi/điểm
vào harness; fresh attach cùng flow đúng exit0/26,699s. Documenter cập
nhật compact112×112 và vùng facts đủ ngang sau khi mở tám PNG, giữ token.
Đây chưa là strict32/full npm test hoặc cổng landing. Các state, motion,
TalkBack, measured native contrast/targets, wide/iOS vẫn chưa nhận thêm.

Đính chính worker: MOBILE_INPROC_WORKER=0 chỉ chặn chatassist, không chặn
diary.Run/community.Run khởi động riêng với prod PostgreSQL. Audit K phải
quan sát job thật, không coi worker0 là bằng chứng job không thể chạy.

## Đợt thiết kế lại chín vùng — 10/10, source797

Yêu cầu độ đẹp áp dụng lại từng màn/feature; đánh giá5/10 của chủ sản phẩm
không bị thay bằng các verdict scoped cũ. Reference ImageGen mới chỉ để
phản biện bố cục, không phải comp được duyệt hoặc ảnh app. Batchd9 nhận
**fix**: title/back, facts/giá và lý do Explore, nhãn ảnh chữ lớn, plus ở
nhánh chưa có nhóm, vé Plan200, exact ngoài thang kéo và documenter.
Batch9a sửa code; native phát hiện footer profile dư112px vì đếm tab hai
lần. Runner9a dừng có kiểm soát/exit130 và font restored, không tính PASS.
Source797 chỉ sửa footer dư đó; tab nằm trong normal flow.

| Evidence exact797 | Kết quả đã có |
|---|---|
| Clean typecheck/guide/build/compile/fixup | 0;7,238/1,385/16,051/2,826/0,036s |
| Node subset214files, loại5browserdrivers | 1608/1608,0fail/skip/cancel/todo;34,652s |
| Go guide / tree guard | 0;0,252/32,924s; clean trước/sau từng bước |
| Pure-helper identity/canary/hai mutants | 3/3 identity; cả ba variant đỏ đúng test dự đoán, copy compiled ngoài repo |
| Primary5558, sáng/tối × font100/130/200 | 24/24flow0;714,665s; sáu drag theo XML thật; font restored1.0 |
| Profile draft/Back/Huỷ/Save | r2 exit0/53,785s; GET giữ name/bio/city/avatar; hai red harness trước vẫn giữ |
| Explore200 facts/giá/reason, tab, exact note | supplement exit0/43,672s; Cộng đồng selected capture đang loading |

Runtime: JS797, APK2f20a301 giữ nguyên (JS-only), API9a với Go diff rỗng
đến797. Ảnh320×640 primary native; root đã mở mẫu100/130/200 sáng/tối và
supplement, không nhận đã xem mọi capture. Giá exact3.250.123đ giữ nguyên;
note ngoài0–2triệu nằm trên slider. Vé200 đọc đủ ngày/tháng và metadata;
nhãn Minh hoạ/heart không chồng, giá/reason đầy đủ sau cuộn. Đây chưa là
full rubric responsive/a11y, native contrast/targets hoặc TalkBack.

Documenter fresh đã đọc build và mở14ảnh, cập nhật DESIGN/sidecar scoped797;
giữ token và lịch sử. Sau bổ sung hai ảnh no-group visible-final, documenter
đã mở16ảnh. Reviewer verdict pass mở28required+9optional, cả bảy finding
resolved, không thấy hồi quy batch trong mẫu; **ship chỉ bảy finding** ở
`fresh-verdict-pass.md`, không phải chín surface/toàn app. Full npm test/strict32,
landingmain, motion toàn state, tablet/iOS và coverage A–S vẫn mở. Không
quy 24/24 hoặc TSX regex0finding thành14/16 hoặc frontend hoàn tất.
Evidence ngoài repo: `native-audit-797a3f52/{clean-checks.json,
after-matrix-results.json,after-matrix-restore.json,mutation-evidence/,
profile-functional-result-r2.json,root-opened-images.json}` dưới runtime
riêng đã ghi ở các mục trước.

K797 preview→reader có một runner29,603s đỏ do selector cũ «Sửa lời và
ảnh»; source dùng «Sửa theo cách mình nhớ». Resume đúng exit0/7,674s.
Root đã mở bốn PNG preview top/page/controls và reader private. Không
đổi red thành xanh, không nhận toàn K hoặc public/AI/second-page đã kiểm.

## K — hoàn thiện vùng soạn/đọc sổ, c9 → b76 → 5cab

Bản c9 bỏ vòng đo chiều cao hồi tiếp và width1; bìa kín tự giãn, ảnh có ratio
ngay từ placeholder. Bản b76 gom pager/trường/ảnh/công cụ48dp vào trang đang
sửa, chỉ một pager khi đọc; sửa ở header, quản lý sổ mở tại chỗ. Tên sổ và
công cụ xuống dòng theo chữ lớn. Reviewer c9 mở31 required và bốn sheet bổ
sung motion ảnh; verdict b76 mở39 required + bốn sheet ảnh: ba
finding resolved, đường kẻ chữ200 vẫn partial. Bản5cab bỏ interior rules ở
effective scale≥1.25, giữ bottom/focus rule. Reviewer mở16 after được map
trước/sau và chốt **ship chỉ finding2**; ba finding1/3/4 giữ resolved tại b76.
Không cộng tổng /16 hoặc suy ra đã nghiệm thu toàn K/A–S.

Evidence riêng theo SHA dưới runtime ngoài repo đã nêu:

- [Review c9](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-c9c08be7/fresh-k-review.md>) — bốn finding, disposition fix.
- [Verdict b76](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-b76f616d/fresh-k-verdict.md>) — ba resolved, một partial.
- [Verdict5cab](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-5cab4701/fresh-k-rule-verdict.md>) — finding2 resolved, không có regression mới trong16 ảnh.

Native b76: sáu profile sáng/tối × font1.0/1.3/2.0,6/6 exit0,261,735s;
functional69,254s exit0, nháp qua chuyển trang còn nguyên, bỏ trang thử rồi
lưu no-op revision4→4/document/private nguyên. Hai flow5cab shared caption
130/200 exit0/121,935s; bốn editor profile exit0/88,025s. Root mở cả16 ảnh
đối chứng cuối. Không relabel ảnh hoặc witness b76 thành5cab.

Chu kỳ quyền b76 thao tác qua native: công khai27,150s và riêng tư23,695s,
đều exit0. Owner/Ngọc đọc book và published photo200 khi public; sau private
Ngọc book404 và previously-public photo404, owner200. Original personal photo
owner200/Ngọc404 ở cả hai trạng thái. Revision4→5→6, hai trang/document gốc
giữ nguyên. Chỉ hai ảnh chữ tổng hợp được phép; fixture kết thúc private.

Motion b76: manual compose0ảnh normal16/reduced9 frame; edit→preview2ảnh
normal56/reduced11 frame. Root và reviewer mở các contact sheet; vùng ảnh
placeholder→decoded giữ geometry trong lần light100 này. Clip variable-rate,
không chứng minh60fps, save/AI/lật trang hoặc mọi tải ảnh. Các setting motion
được phục hồi. FFmpeg báo duplicate DTS ở reduced compose; số frame decode
vẫn khớp ffprobe9/9, không giấu warning.

Clean exact5cab: typecheck/guide/build/compile/fixup/Go-guide/tree exit0;
Node214 file **1608/1608**,0fail/skip/cancel/todo,34,807s. Tree5106 file;
Go-guide0,236s. Năm browser test bị loại rõ trong receipt do chỉ kiểm native,
không gọi đây là full npm/strict32. Identity8/8; canary và hai mutant mỗi
7/8 đúng predicted-red,0skip, cùng SHA5cab, chỉ contract chọn ảnh/nháp bất biến.
Detector5cab chỉ TSX regex một file0/[]; không đo geometry/contrast native.

Các lần harness đỏ vẫn giữ: c9 off-fold/IME và kỳ vọng revision sai;
b76 mutation r0 thiếu topology packages/shared; privacy r0 assert nhãn preview
sau khi đã chuyển editor. Chúng không thành PASS hồi tố; bản r1/r2 có receipt
riêng. Hồ sơ chi tiết ở RECOVERY. Runtime JS5cab/APK2f20/Go9a có SHA độc lập.
AI/error/retry, full lifecycle/accessibility, toàn A–S và landing main còn mở.

## H — sáu fixes nhóm/bạn bè, fa735 → c941 → d91

Giữ UI3/seed c8e88116, mockup03 là critique-reference; không mở world mới
hoặc thêm raster. Người/nhãn vai trò dẫn mắt trước shortcut và guard. Ba form
giữ field khi IME mở, safe-area footer trên phone; expanded ghép action sát
vật giấy. Nhãn bìa tự cao; tỷ lệ cuốn sổ nằm ở vải. Xác nhận rời nhóm có
hậu quả và hai quyết định trong sheet lấy chiều cao nội dung, footer ngoài cuộn.

[Review fa735](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-fa735354/fresh-h-review.md>)
có sáu finding. [Verdict c941](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-c941f09e/fresh-h-verdict.md>)
resolved bốn, còn sheet82% và nhãn expanded368dp partial.
[Verdict d91](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-d91e66d1/fresh-h-final-verdict.md>)
resolved đủ sáu sau khi mở sáu file chứa16 viewport mới: New sáu compact,
bốn expanded ngắn/dài sáng/tối, sáu reinvite/accept/leave/cancel. Root cũng
mở đủ ảnh. Năm artifact không đổi có hash đối chiếu; F1/F2/F4/F6 giữ
provenance c941, không relabel ảnh cũ thành d91. Ship chỉ sáu fixes.

Đích thành công kết bạn từ Members/Friends và recovery duplicate đều tới
Friends/Đã gửi thật. Copy recovery không lộ trạng thái quan hệ. Vai trò đọc
đủ130/200%; guard quản trị ngắn, mở giải thích được. Leave/cancel ở light100
23,544s và dark200 26,650s exit0; witness chín read200 giữ cả hai active.
Actual leave tại c941 17,486s exit0, recipient về inbox không nhóm; d91 mời
lại20,040s và recipient đồng ý16,085s exit0. Không tạo quyền/membership giả.

Clean exact d91 có chín bước xanh: type7,750s, guide1,341s, export14,919s,
compile2,705s, fixup0,034s, Node214 file **1608/1608**/33,771s,
core-build2,140s, Go-guide0,268s, tree32,547s/**5107 PASS**.
Identity role9/9; canary và hai mutant độc lập mỗi7/9, đỏ đúng ca dự đoán,
cùng harness2072d502,0skip/cancel/todo. TSX regex bảy file exit0/[];
không đo contrast/geometry native. Đây subset native, không full npm/gate32strict.

Giữ log đỏ af37/601/c941: selector cũ, transport khi boot thiết bị, privacy
và source assertion, expanded outer143, cancel assert off-fold, deep link
dev-client trước bundle-ready. Không đếm chúng thành xanh hồi tố. API/JS d91
ở lượt cuối, APK2f20 không đổi; expanded768×1024 là cùng AVD resize và đã
trả320×640/font1.0, không phải tablet độc lập. Input dài vẫn cuộn nội bộ
hai dòng gần caret; chưa chứng minh cả95 ký tự cùng hiện. Network/retry,
lifecycle, motion/reduced-motion, TalkBack/iOS, toàn H/A–S và main còn mở.

## I1 — bảy finding đã chấm, 7dcacedf (10/10)

Giữ UI3/seed c8e88116, mockup06 chỉ critique-reference. Review001 đưa bảy
finding; verdict62 mở14 contact/92 viewport và2 original, resolved sáu,
F5 partial vì «Chọn loại sổ» mất chữ/bẻ từ cuối. Bản7d stretch đúng hàng và
nhãn phụ48dp, căn trái/mực phụ. Root và reviewer mở3 contact/24 viewport
mới: hai phía, sáng/tối100/200, kèm giao kèo Một đôi chỉ đọc/cancel.
[fresh-i1-f5-verdict](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-7dcacedf/fresh-i1-f5-verdict.md>)
resolved F5, không thấy regression trong phạm vi này. Ship chỉ bảy sửa I1;
sáu finding giữ provenance62 với diff/hashes, không relabel92 viewport cũ.

Người dẫn mắt trước bìa/cảnh; tên, vai trò và trạng thái ký có thứ bậc riêng.
Compact giữ entry giao kèo ngoài scroll; proposal/error ở footer quyết định,
recipient có CTA ngay khi vào. Để sau trung tính/đủ chữ48dp. Sau mở primary
con dấu và secondary mực phụ; M6 bìa/dấu tự wrap. Grain giấy/vải canonical,
không thêm raster lấp chỗ trống. Corroboration hiện tại ở DESIGN.md và brief
I1, không phải contract lịch sử đã vắng khỏi app/_layout.tsx.

Tại62, cặp tổng hợp67fbeed6 được thao tác native: Back/cancel unopened;
primary đề nghị21,593s; recipient cancel26,649s còn pending; recipient đồng
ý18,839s mở active. Bảy API witness đọc hai phía chứng minh chỉ lap_so,
không bat_doi/doc_chat. Login third có hai harness đỏ: tutorial stylus chặn
field49,228s; lần sau login thật thành công nhưng landing Messages khác kỳ
vọng Explore114,806s. Identity recovery20,891s xanh, không gọi đỏ thành xanh.
Để sau ở4a từng chỉ vẽ «Để» dù accessibility đủ; giữ ảnh đỏ. M6 thật ở62,
không reset cặp active để replay. Bốn shared couple62 chỉ đọc/cancel.

Native7d tám flow exit0: primary sáng100/20029,152/31,555s, tối100/200
27,299/30,754s; recipient sáng100/20019,190/14,783s, tối100/20015,784/
15,284s. Hai API read200 sau cancel vẫn active/lap_so, không pending hay
quyền cao hơn. Clean checkout đúng7d chín bước xanh: type7,293s, guide1,295s,
export14,448s, compile2,678s, fixup0,035s, Node214 file1608/1608/33,918s,
core-build1,762s, Go-guide0,251s, tree32,374s/5108 file.0fail/skip/cancel/todo.
Consent harness0fc8e99f: identity12/12/0,065s, canary10/12/0,063s, mutant
ignore-empty11/12/0,062s và own-proposal10/12/0,058s, đỏ đúng dự đoán,
đã kiểm khác nghĩa trước chạy. Chỉ contract, không thay API/UI proof.

TSX scan r0[] nhưng exit1 vì đường ChuKy sai; không tính pass. r1 đúng năm
target exit0/[], chỉ regex, không đo contrast/geometry native. JS/API7d,
Metro2725931/API2739700; APK vẫn2f20/hash963bd5da…514, không gọi APK7d.
Nguồn Go không đổi, binary9ca8ea8f trùng62; chỉ restart process có ownership,
không DB/oracle. Hai font trả1.0; peer5560 có draft không chạm. Stylus0 trên
primary/third cần phục hồi khi xong toàn audit (third original không được
lưu trước helper đỏ; phục hồi bằng xoá key về mặc định, không bịa observednull).

I2 còn mở: baseline62 bốn flow xanh/36 viewport (34,008/43,070/42,620/
54,235s), root mở4 contact; A/B độc lập đã đánh giá. Nháp IME7d sáng100/
tối200124,376/124,025s chỉ gõ rồi cancel, root mở4 viewport; vùng nhập và
hành động bị che, không gọi exit0 là UX pass. API giữ điều tránh tổng hợp46
ký tự/version1, không lưu marker nháp. Tem Đã chốt bị cắt xác nhận lại bằng
original7d và XML: text46×14, khung66×26, chuỗi trợ năng đầy đủ. Cần sửa
đọc nội dung/IME/nhịp mục lục/Gu/phản hồi loại sổ rồi QC đúng source. Toàn
I/A–S, motion/TalkBack/lifecycle/network, iOS/native crypto/fullnpm/strict32
và landing main vẫn chưa nghiệm thu.


## I2 — mục lục, điều cần tránh và Gu; chốt sửa 814ea5c9 (10/10)

Phần I2 ghi mở ở mục I1 phía trên đã qua build và review trong phạm vi sau.
Giữ UI3/seed c8e88116, mockup06 chỉ làm critique-reference. Mục lục gom ba
nhóm theo nghĩa; nội dung điều tránh dài tự cao 3–8 dòng theo fontScale;
footer lưu/lỗi nằm trên IME. Loại sổ nhận busy/error trong chính sheet đang
thao tác. Hậu quả dương của đóng sổ được nhấn, không thực hiện đóng sổ.
Tem ĐÃ CHỐT dùng minWidth88 opt-in và chữ stretch/center; các Stamp khác
không đổi. Gu dẫn bằng trạng thái của mình và đủ phạm vi quyền chia sẻ.
Không đổi API/domain, tiền, consent/locks, AI provider hoặc E2EE.

Checkpoint a24cb4bf có 8 profile/84 viewport Android: friends sáng100/tối200;
couple sáng100/200, tối100/200; expanded sáng100/tối200. Root mở cả8 contact.
Có thêm26 stage tương tác xanh,343,691s/12 frame: IME/cancel cả hai field
100/200; lỗi mạng không lưu; retry native lưu đúng116/101 ký tự, foodv3/dungv2.
Type failure giữ Cặp đôi; Gu bật/tắt thật, counterpart chỉ thấy gu của người
bật, common vẫn ẩn. Primary/counterpart200, outsider404; trước/sau bỏ chặn
lỗi Gu đều private. Root mở3 contact tương tác, tổng11 contact/96 viewport.
Chuỗi dài100 đọc từ đầu;200 dùng cuộn, không tuyên bố cả chuỗi cùng hiện.
Expanded768×1024 là cùng AVD resize, không phải tablet độc lập.

[Full finish review I2](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-a24cb4bf/fresh-i2-finish-review.md>)
mở11 contact/96 viewport,20 original và mockup06: mộtP2, lỗi Gu200 bị đoạn
phạm vi ngăn khỏi nút phục hồi. Bản814ea5c9 giữ nguyên permission/handlers,
chỉ khi lỗi gom feedback cùng chính các nút lệnh vào footer cố định. Lượt
cuối8 stage xanh/12 frame mới ở sáng100/tối200: private → lỗi TCP-reset →
retry native → shared → unshare. Root mở2 contact và2 original lỗi;
reviewer mở2 contact/12 frame cùng4 original.
[Verdict F1](</home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-814ea5c9/fresh-i2-f1-verdict.md>)
resolved/ship, không thấy regression hẹp. Các sửa I2 khác carry từ a24 theo
diff; không relabel96 viewport a24 thành814. Đây chốt sửa I2, không toàn I/A–S.

Clean exact814 có9 bước xanh: type7,243s; guide1,288s; export14,462s;
compile2,656s; fixup0,036s; Node214 file1608/1608/34,145s; core1,441s;
Go-guide0,242s; tree32,561s/5109 file.0fail/skip/cancel/todo. Guide62 route,
hash e62d163cd645 giữ nguyên. TSX detector cuối đúng1 target[]/exit0,
chỉ regex; scan8 target a24 cũng chỉ regex, không đo native geometry/contrast.
Gu harness311694a7: identity12/12/0,065s; canary10/12/0,064s;
hai mutant10/12/0,061s và11/12/0,062s, đỏ đúng tên ca dự đoán,
khác nghĩa được ghi trước, cùng harness. File compiled production giữ hash
25ea577a…0ff2. Đây pure contract, không thay API/UI proof hoặc full npm/strict32.

Giữ lịch sử đỏ: 8a stamp uppercase còn cắt100; expandedr0 vuốt tay cầm;
bỏ adb reverse không ngắt HTTP nên type write vẫn chạy; REJECT mặc định để
Gu request treo rồi gửi khi bỏ chặn. Fixture được khôi phục rõ bằng native
proposal/API recipient grant và native tắt Gu, không gọi recipient-native
proof. TCP-reset đúng cổng QC của owned emulator đã kiểm trước/sau tháo rule.
814 dark r0 đã assert lỗi+nút và chụp đúng, nhưng default scroll bắt đầu trên
footer nên không cuộn thân; r1 vuốt trong thân xanh, không sửa product cho
harness. Helper mutation r0 nhầm raw/transformed hash, dừng trước mọi test;
đã ghi riêng, không tính là product test đỏ hoặc pass hồi tố.

JS/API814, Metro3582510/API3581698; Go binary9ca8ea8f byte-identical a24.
APK vẫn build2f20/hash963bd5da…514. Chỉ restart process sở hữu; không DB,
oracle, Docker, soak audit hoặc peer5560. Font1/wm320×640 và mọi rule riêng
đã phục hồi. Stylus primary/third còn0 cho toàn audit, phục hồi ở cuối như
mục I1. Motion/reduced motion, TalkBack, native computed contrast/target,
lifecycle, iOS/hardware, crypto và các state chưa chụp vẫn N/E. J và toàn
A–S/gates cuối/main tiếp tục mở; không tuyên bố app đã đạt chuẩn thẩm mỹ.
